// Package multiagent 使用 CloudWeGo Eino 的 DeepAgent（adk/prebuilt/deep）编排多代理，MCP 工具经 einomcp 桥接到现有 Agent。
package multiagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"cyberstrike-ai/internal/agent"
	"cyberstrike-ai/internal/agents"
	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/einomcp"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/deep"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// RunResult 与单 Agent 循环结果字段对齐，便于复用存储与 SSE 收尾逻辑。
type RunResult struct {
	Response        string
	MCPExecutionIDs []string
	LastReActInput  string
	LastReActOutput string
}

// toolCallPendingInfo tracks a tool_call emitted to the UI so we can later
// correlate tool_result events (even when the framework omits ToolCallID) and
// avoid leaving the UI stuck in "running" state on recoverable errors.
type toolCallPendingInfo struct {
	ToolCallID string
	ToolName   string
	EinoAgent  string
	EinoRole   string
}

// RunDeepAgent 使用 Eino DeepAgent 执行一轮对话（流式事件通过 progress 回调输出）。
func RunDeepAgent(
	ctx context.Context,
	appCfg *config.Config,
	ma *config.MultiAgentConfig,
	ag *agent.Agent,
	logger *zap.Logger,
	conversationID string,
	userMessage string,
	history []agent.ChatMessage,
	roleTools []string,
	progress func(eventType, message string, data interface{}),
	agentsMarkdownDir string,
) (*RunResult, error) {
	if appCfg == nil || ma == nil || ag == nil {
		return nil, fmt.Errorf("multiagent: 配置或 Agent 为空")
	}

	effectiveSubs := ma.SubAgents
	var orch *agents.OrchestratorMarkdown
	if strings.TrimSpace(agentsMarkdownDir) != "" {
		load, merr := agents.LoadMarkdownAgentsDir(agentsMarkdownDir)
		if merr != nil {
			if logger != nil {
				logger.Warn("加载 agents 目录 Markdown 失败，沿用 config 中的 sub_agents", zap.Error(merr))
			}
		} else {
			effectiveSubs = agents.MergeYAMLAndMarkdown(ma.SubAgents, load.SubAgents)
			orch = load.Orchestrator
		}
	}
	if ma.WithoutGeneralSubAgent && len(effectiveSubs) == 0 {
		return nil, fmt.Errorf("multi_agent.without_general_sub_agent 为 true 时，必须在 multi_agent.sub_agents 或 agents 目录 Markdown 中配置至少一个子代理")
	}

	holder := &einomcp.ConversationHolder{}
	holder.Set(conversationID)

	var mcpIDsMu sync.Mutex
	var mcpIDs []string
	recorder := func(id string) {
		if id == "" {
			return
		}
		mcpIDsMu.Lock()
		mcpIDs = append(mcpIDs, id)
		mcpIDsMu.Unlock()
	}

	// 与单代理流式一致：在 response_start / response_delta 的 data 中带当前 mcpExecutionIds，供主聊天绑定复制与展示。
	snapshotMCPIDs := func() []string {
		mcpIDsMu.Lock()
		defer mcpIDsMu.Unlock()
		out := make([]string, len(mcpIDs))
		copy(out, mcpIDs)
		return out
	}

	mainDefs := ag.ToolsForRole(roleTools)
	toolOutputChunk := func(toolName, toolCallID, chunk string) {
		// When toolCallId is missing, frontend ignores tool_result_delta.
		if progress == nil || toolCallID == "" {
			return
		}
		progress("tool_result_delta", chunk, map[string]interface{}{
			"toolName":   toolName,
			"toolCallId": toolCallID,
			// index/total/iteration are optional for UI; we don't know them in this bridge.
			"index":     0,
			"total":     0,
			"iteration": 0,
			"source":    "eino",
		})
	}

	mainTools, err := einomcp.ToolsFromDefinitions(ag, holder, mainDefs, recorder, toolOutputChunk)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{
		Timeout: 30 * time.Minute,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   300 * time.Second,
				KeepAlive: 300 * time.Second,
			}).DialContext,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   30 * time.Second,
			ResponseHeaderTimeout: 60 * time.Minute,
		},
	}

	baseModelCfg := &einoopenai.ChatModelConfig{
		APIKey:     appCfg.OpenAI.APIKey,
		BaseURL:    strings.TrimSuffix(appCfg.OpenAI.BaseURL, "/"),
		Model:      appCfg.OpenAI.Model,
		HTTPClient: httpClient,
	}

	deepMaxIter := ma.MaxIteration
	if deepMaxIter <= 0 {
		deepMaxIter = appCfg.Agent.MaxIterations
	}
	if deepMaxIter <= 0 {
		deepMaxIter = 40
	}

	subDefaultIter := ma.SubAgentMaxIterations
	if subDefaultIter <= 0 {
		subDefaultIter = 20
	}

	subAgents := make([]adk.Agent, 0, len(effectiveSubs))
	for _, sub := range effectiveSubs {
		id := strings.TrimSpace(sub.ID)
		if id == "" {
			return nil, fmt.Errorf("multi_agent.sub_agents 中存在空的 id")
		}
		name := strings.TrimSpace(sub.Name)
		if name == "" {
			name = id
		}
		desc := strings.TrimSpace(sub.Description)
		if desc == "" {
			desc = fmt.Sprintf("Specialist agent %s for penetration testing workflow.", id)
		}
		instr := strings.TrimSpace(sub.Instruction)
		if instr == "" {
			instr = "你是 CyberStrikeAI 中的专业子代理，在授权渗透测试场景下协助完成用户委托的子任务。优先使用可用工具获取证据，回答简洁专业。"
		}

		roleTools := sub.RoleTools
		bind := strings.TrimSpace(sub.BindRole)
		if bind != "" && appCfg.Roles != nil {
			if r, ok := appCfg.Roles[bind]; ok && r.Enabled {
				if len(roleTools) == 0 && len(r.Tools) > 0 {
					roleTools = r.Tools
				}
				if len(r.Skills) > 0 {
					var b strings.Builder
					b.WriteString(instr)
					b.WriteString("\n\n本角色推荐通过 list_skills / read_skill 按需加载的 Skills：")
					for i, s := range r.Skills {
						if i > 0 {
							b.WriteString("、")
						}
						b.WriteString(s)
					}
					b.WriteString("。")
					instr = b.String()
				}
			}
		}

		subModel, err := einoopenai.NewChatModel(ctx, baseModelCfg)
		if err != nil {
			return nil, fmt.Errorf("子代理 %q ChatModel: %w", id, err)
		}

		subDefs := ag.ToolsForRole(roleTools)
		subTools, err := einomcp.ToolsFromDefinitions(ag, holder, subDefs, recorder, toolOutputChunk)
		if err != nil {
			return nil, fmt.Errorf("子代理 %q 工具: %w", id, err)
		}

		subMax := sub.MaxIterations
		if subMax <= 0 {
			subMax = subDefaultIter
		}

		subSumMw, err := newEinoSummarizationMiddleware(ctx, subModel, appCfg, logger)
		if err != nil {
			return nil, fmt.Errorf("子代理 %q summarization 中间件: %w", id, err)
		}

		sa, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
			Name:        id,
			Description: desc,
			Instruction: instr,
			Model:       subModel,
			ToolsConfig: adk.ToolsConfig{
				ToolsNodeConfig: compose.ToolsNodeConfig{
					Tools:               subTools,
					UnknownToolsHandler: einomcp.UnknownToolReminderHandler(),
					ToolCallMiddlewares: []compose.ToolMiddleware{
						{Invokable: softRecoveryToolCallMiddleware()},
					},
				},
				EmitInternalEvents: true,
			},
			MaxIterations: subMax,
			Handlers:      []adk.ChatModelAgentMiddleware{subSumMw},
		})
		if err != nil {
			return nil, fmt.Errorf("子代理 %q: %w", id, err)
		}
		subAgents = append(subAgents, sa)
	}

	mainModel, err := einoopenai.NewChatModel(ctx, baseModelCfg)
	if err != nil {
		return nil, fmt.Errorf("Deep 主模型: %w", err)
	}

	mainSumMw, err := newEinoSummarizationMiddleware(ctx, mainModel, appCfg, logger)
	if err != nil {
		return nil, fmt.Errorf("Deep 主代理 summarization 中间件: %w", err)
	}

	// 与 deep.Config.Name 一致。子代理的 assistant 正文也会经 EmitInternalEvents 流出，若全部当主回复会重复（编排器总结 + 子代理原文）。
	orchestratorName := "cyberstrike-deep"
	orchDescription := "Coordinates specialist agents and MCP tools for authorized security testing."
	orchInstruction := strings.TrimSpace(ma.OrchestratorInstruction)
	if orch != nil {
		if strings.TrimSpace(orch.EinoName) != "" {
			orchestratorName = strings.TrimSpace(orch.EinoName)
		}
		if d := strings.TrimSpace(orch.Description); d != "" {
			orchDescription = d
		}
		if ins := strings.TrimSpace(orch.Instruction); ins != "" {
			orchInstruction = ins
		}
	}
	da, err := deep.New(ctx, &deep.Config{
		Name:                   orchestratorName,
		Description:            orchDescription,
		ChatModel:              mainModel,
		Instruction:            orchInstruction,
		SubAgents:              subAgents,
		WithoutGeneralSubAgent: ma.WithoutGeneralSubAgent,
		WithoutWriteTodos:      ma.WithoutWriteTodos,
		MaxIteration:           deepMaxIter,
		// 防止 sub-agent 再调用 task（再委派 sub-agent），形成无限委派链。
		Handlers: []adk.ChatModelAgentMiddleware{
			newNoNestedTaskMiddleware(),
			mainSumMw,
		},
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools:               mainTools,
				UnknownToolsHandler: einomcp.UnknownToolReminderHandler(),
				ToolCallMiddlewares: []compose.ToolMiddleware{
					{Invokable: softRecoveryToolCallMiddleware()},
				},
			},
			EmitInternalEvents: true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("deep.New: %w", err)
	}

	baseMsgs := historyToMessages(history)
	baseMsgs = append(baseMsgs, schema.UserMessage(userMessage))

	streamsMainAssistant := func(agent string) bool {
		return agent == "" || agent == orchestratorName
	}
	einoRoleTag := func(agent string) string {
		if streamsMainAssistant(agent) {
			return "orchestrator"
		}
		return "sub"
	}

	var lastRunMsgs []adk.Message
	var lastAssistant string

	// retryHints tracks the corrective hint to append for each retry attempt.
	// Index i corresponds to the hint that will be appended on attempt i+1.
	var retryHints []adk.Message

attemptLoop:
	for attempt := 0; attempt < maxToolCallRecoveryAttempts; attempt++ {
		msgs := make([]adk.Message, 0, len(baseMsgs)+len(retryHints))
		msgs = append(msgs, baseMsgs...)
		msgs = append(msgs, retryHints...)

		if attempt > 0 {
			mcpIDsMu.Lock()
			mcpIDs = mcpIDs[:0]
			mcpIDsMu.Unlock()
		}

		// 仅保留主代理最后一次 assistant 输出；每轮重试重置，避免拼接失败轮次的片段。
		lastAssistant = ""
		var reasoningStreamSeq int64
		var einoSubReplyStreamSeq int64
		toolEmitSeen := make(map[string]struct{})
		var einoMainRound int
		var einoLastAgent string
		subAgentToolStep := make(map[string]int)
		// Track tool calls emitted in this attempt so we can:
		// - attach toolCallId to tool_result when framework omits it
		// - flush running tool calls as failed when a recoverable tool execution error happens
		pendingByID := make(map[string]toolCallPendingInfo)
		pendingQueueByAgent := make(map[string][]string)
		markPending := func(tc toolCallPendingInfo) {
			if tc.ToolCallID == "" {
				return
			}
			pendingByID[tc.ToolCallID] = tc
			pendingQueueByAgent[tc.EinoAgent] = append(pendingQueueByAgent[tc.EinoAgent], tc.ToolCallID)
		}
		popNextPendingForAgent := func(agentName string) (toolCallPendingInfo, bool) {
			q := pendingQueueByAgent[agentName]
			for len(q) > 0 {
				id := q[0]
				q = q[1:]
				pendingQueueByAgent[agentName] = q
				if tc, ok := pendingByID[id]; ok {
					delete(pendingByID, id)
					return tc, true
				}
			}
			return toolCallPendingInfo{}, false
		}
		removePendingByID := func(toolCallID string) {
			if toolCallID == "" {
				return
			}
			delete(pendingByID, toolCallID)
			// queue cleanup is lazy in popNextPendingForAgent
		}
		flushAllPendingAsFailed := func(err error) {
			if progress == nil {
				pendingByID = make(map[string]toolCallPendingInfo)
				pendingQueueByAgent = make(map[string][]string)
				return
			}
			msg := ""
			if err != nil {
				msg = err.Error()
			}
			for _, tc := range pendingByID {
				toolName := tc.ToolName
				if strings.TrimSpace(toolName) == "" {
					toolName = "unknown"
				}
				progress("tool_result", fmt.Sprintf("工具结果 (%s)", toolName), map[string]interface{}{
					"toolName":       toolName,
					"success":        false,
					"isError":        true,
					"result":         msg,
					"resultPreview":  msg,
					"toolCallId":     tc.ToolCallID,
					"conversationId": conversationID,
					"einoAgent":      tc.EinoAgent,
					"einoRole":       tc.EinoRole,
					"source":         "eino",
				})
			}
			pendingByID = make(map[string]toolCallPendingInfo)
			pendingQueueByAgent = make(map[string][]string)
		}

		runner := adk.NewRunner(ctx, adk.RunnerConfig{
			Agent:           da,
			EnableStreaming: true,
		})
		iter := runner.Run(ctx, msgs)

		for {
			ev, ok := iter.Next()
			if !ok {
				lastRunMsgs = msgs
				break attemptLoop
			}
			if ev == nil {
				continue
			}
			if ev.Err != nil {
				canRetry := attempt+1 < maxToolCallRecoveryAttempts

				// Recoverable: API-level JSON argument validation error.
				if canRetry && isRecoverableToolCallArgumentsJSONError(ev.Err) {
					if logger != nil {
						logger.Warn("eino: recoverable tool-call JSON error from model/API", zap.Error(ev.Err), zap.Int("attempt", attempt))
					}
					retryHints = append(retryHints, toolCallArgumentsJSONRetryHint())
					if progress != nil {
						progress("eino_recovery", toolCallArgumentsJSONRecoveryTimelineMessage(attempt), map[string]interface{}{
							"conversationId": conversationID,
							"source":         "eino",
							"einoRetry":      attempt,
							"runIndex":       attempt + 1,
							"maxRuns":        maxToolCallRecoveryAttempts,
							"reason":         "invalid_tool_arguments_json",
						})
					}
					continue attemptLoop
				}

				// Recoverable: tool execution error (unknown sub-agent, tool not found, bad JSON in args, etc.).
				if canRetry && isRecoverableToolExecutionError(ev.Err) {
					if logger != nil {
						logger.Warn("eino: recoverable tool execution error, will retry with corrective hint",
							zap.Error(ev.Err), zap.Int("attempt", attempt))
					}
					// Ensure UI/tool timeline doesn't get stuck at "running" for tool calls that
					// will never receive a proper tool_result due to the recoverable error.
					flushAllPendingAsFailed(ev.Err)
					retryHints = append(retryHints, toolExecutionRetryHint())
					if progress != nil {
						progress("eino_recovery", toolExecutionRecoveryTimelineMessage(attempt), map[string]interface{}{
							"conversationId": conversationID,
							"source":         "eino",
							"einoRetry":      attempt,
							"runIndex":       attempt + 1,
							"maxRuns":        maxToolCallRecoveryAttempts,
							"reason":         "tool_execution_error",
						})
					}
					continue attemptLoop
				}

				// Non-recoverable error.
				flushAllPendingAsFailed(ev.Err)
				if progress != nil {
					progress("error", ev.Err.Error(), map[string]interface{}{
						"conversationId": conversationID,
						"source":         "eino",
					})
				}
				return nil, ev.Err
			}
			if ev.AgentName != "" && progress != nil {
				if streamsMainAssistant(ev.AgentName) {
					if einoMainRound == 0 {
						einoMainRound = 1
						progress("iteration", "", map[string]interface{}{
							"iteration":      1,
							"einoScope":      "main",
							"einoRole":       "orchestrator",
							"einoAgent":      orchestratorName,
							"conversationId": conversationID,
							"source":         "eino",
						})
					} else if einoLastAgent != "" && !streamsMainAssistant(einoLastAgent) {
						einoMainRound++
						progress("iteration", "", map[string]interface{}{
							"iteration":      einoMainRound,
							"einoScope":      "main",
							"einoRole":       "orchestrator",
							"einoAgent":      orchestratorName,
							"conversationId": conversationID,
							"source":         "eino",
						})
					}
				}
				einoLastAgent = ev.AgentName
				progress("progress", fmt.Sprintf("[Eino] %s", ev.AgentName), map[string]interface{}{
					"conversationId": conversationID,
					"einoAgent":      ev.AgentName,
					"einoRole":       einoRoleTag(ev.AgentName),
				})
			}
			if ev.Output == nil || ev.Output.MessageOutput == nil {
				continue
			}
			mv := ev.Output.MessageOutput

			if mv.IsStreaming && mv.MessageStream != nil {
				streamHeaderSent := false
				var reasoningStreamID string
				var toolStreamFragments []schema.ToolCall
				var subAssistantBuf strings.Builder
				var subReplyStreamID string
				var mainAssistantBuf strings.Builder
				for {
					chunk, rerr := mv.MessageStream.Recv()
					if rerr != nil {
						if errors.Is(rerr, io.EOF) {
							break
						}
						if logger != nil {
							logger.Warn("eino stream recv", zap.Error(rerr))
						}
						break
					}
					if chunk == nil {
						continue
					}
					if progress != nil && strings.TrimSpace(chunk.ReasoningContent) != "" {
						if reasoningStreamID == "" {
							reasoningStreamID = fmt.Sprintf("eino-reasoning-%s-%d", conversationID, atomic.AddInt64(&reasoningStreamSeq, 1))
							progress("thinking_stream_start", " ", map[string]interface{}{
								"streamId":  reasoningStreamID,
								"source":    "eino",
								"einoAgent": ev.AgentName,
								"einoRole":  einoRoleTag(ev.AgentName),
							})
						}
						progress("thinking_stream_delta", chunk.ReasoningContent, map[string]interface{}{
							"streamId": reasoningStreamID,
						})
					}
					if chunk.Content != "" {
						if progress != nil && streamsMainAssistant(ev.AgentName) {
							if !streamHeaderSent {
								progress("response_start", "", map[string]interface{}{
									"conversationId":     conversationID,
									"mcpExecutionIds":    snapshotMCPIDs(),
									"messageGeneratedBy": "eino:" + ev.AgentName,
									"einoRole":           "orchestrator",
								})
								streamHeaderSent = true
							}
							progress("response_delta", chunk.Content, map[string]interface{}{
								"conversationId":  conversationID,
								"mcpExecutionIds": snapshotMCPIDs(),
								"einoRole":        "orchestrator",
							})
							mainAssistantBuf.WriteString(chunk.Content)
						} else if !streamsMainAssistant(ev.AgentName) {
							if progress != nil {
								if subReplyStreamID == "" {
									subReplyStreamID = fmt.Sprintf("eino-sub-reply-%s-%d", conversationID, atomic.AddInt64(&einoSubReplyStreamSeq, 1))
									progress("eino_agent_reply_stream_start", "", map[string]interface{}{
										"streamId":       subReplyStreamID,
										"einoAgent":      ev.AgentName,
										"einoRole":       "sub",
										"conversationId": conversationID,
										"source":         "eino",
									})
								}
								progress("eino_agent_reply_stream_delta", chunk.Content, map[string]interface{}{
									"streamId":       subReplyStreamID,
									"conversationId": conversationID,
								})
							}
							subAssistantBuf.WriteString(chunk.Content)
						}
					}
					// 收集流式 tool_calls 全部分片；arguments 在最后一帧常为 ""，需按 index/id 合并后才能展示 subagent_type/description。
					if len(chunk.ToolCalls) > 0 {
						toolStreamFragments = append(toolStreamFragments, chunk.ToolCalls...)
					}
				}
				if streamsMainAssistant(ev.AgentName) {
					if s := strings.TrimSpace(mainAssistantBuf.String()); s != "" {
						lastAssistant = s
					}
				}
				if subAssistantBuf.Len() > 0 && progress != nil {
					if s := strings.TrimSpace(subAssistantBuf.String()); s != "" {
						if subReplyStreamID != "" {
							progress("eino_agent_reply_stream_end", s, map[string]interface{}{
								"streamId":       subReplyStreamID,
								"einoAgent":      ev.AgentName,
								"einoRole":       "sub",
								"conversationId": conversationID,
								"source":         "eino",
							})
						} else {
							progress("eino_agent_reply", s, map[string]interface{}{
								"conversationId": conversationID,
								"einoAgent":      ev.AgentName,
								"einoRole":       "sub",
								"source":         "eino",
							})
						}
					}
				}
				var lastToolChunk *schema.Message
				if merged := mergeStreamingToolCallFragments(toolStreamFragments); len(merged) > 0 {
					lastToolChunk = &schema.Message{ToolCalls: merged}
				}
				tryEmitToolCallsOnce(lastToolChunk, ev.AgentName, orchestratorName, conversationID, progress, toolEmitSeen, subAgentToolStep, markPending)
				continue
			}

			msg, gerr := mv.GetMessage()
			if gerr != nil || msg == nil {
				continue
			}
			tryEmitToolCallsOnce(mergeMessageToolCalls(msg), ev.AgentName, orchestratorName, conversationID, progress, toolEmitSeen, subAgentToolStep, markPending)

			if mv.Role == schema.Assistant {
				if progress != nil && strings.TrimSpace(msg.ReasoningContent) != "" {
					progress("thinking", strings.TrimSpace(msg.ReasoningContent), map[string]interface{}{
						"conversationId": conversationID,
						"source":         "eino",
						"einoAgent":      ev.AgentName,
						"einoRole":       einoRoleTag(ev.AgentName),
					})
				}
				body := strings.TrimSpace(msg.Content)
				if body != "" {
					if streamsMainAssistant(ev.AgentName) {
						if progress != nil {
							progress("response_start", "", map[string]interface{}{
								"conversationId":     conversationID,
								"mcpExecutionIds":    snapshotMCPIDs(),
								"messageGeneratedBy": "eino:" + ev.AgentName,
								"einoRole":           "orchestrator",
							})
							progress("response_delta", body, map[string]interface{}{
								"conversationId":  conversationID,
								"mcpExecutionIds": snapshotMCPIDs(),
								"einoRole":        "orchestrator",
							})
						}
						lastAssistant = body
					} else if progress != nil {
						progress("eino_agent_reply", body, map[string]interface{}{
							"conversationId": conversationID,
							"einoAgent":      ev.AgentName,
							"einoRole":       "sub",
							"source":         "eino",
						})
					}
				}
			}

			if mv.Role == schema.Tool && progress != nil {
				toolName := msg.ToolName
				if toolName == "" {
					toolName = mv.ToolName
				}

				// bridge 工具在 res.IsError=true 时会返回带前缀的内容；这里解析为 success/isError，避免前端误判为成功。
				content := msg.Content
				isErr := false
				if strings.HasPrefix(content, einomcp.ToolErrorPrefix) {
					isErr = true
					content = strings.TrimPrefix(content, einomcp.ToolErrorPrefix)
				}

				preview := content
				if len(preview) > 200 {
					preview = preview[:200] + "..."
				}
				data := map[string]interface{}{
					"toolName":       toolName,
					"success":        !isErr,
					"isError":        isErr,
					"result":         content,
					"resultPreview":  preview,
					"conversationId": conversationID,
					"einoAgent":      ev.AgentName,
					"einoRole":       einoRoleTag(ev.AgentName),
					"source":         "eino",
				}
				toolCallID := strings.TrimSpace(msg.ToolCallID)
				// Some framework paths (e.g. UnknownToolsHandler) may omit ToolCallID on tool messages.
				// Infer from the tool_call emission order for this agent to keep UI state consistent.
				if toolCallID == "" {
					// In some internal tool execution paths, ev.AgentName may be empty for tool-role
					// messages. Try several fallbacks to avoid leaving UI tool_call status stuck.
					if inferred, ok := popNextPendingForAgent(ev.AgentName); ok {
						toolCallID = inferred.ToolCallID
					} else if inferred, ok := popNextPendingForAgent(orchestratorName); ok {
						toolCallID = inferred.ToolCallID
					} else if inferred, ok := popNextPendingForAgent(""); ok {
						toolCallID = inferred.ToolCallID
					} else {
						// last resort: pick any pending toolCallID
						for id := range pendingByID {
							toolCallID = id
							delete(pendingByID, id)
							break
						}
					}
				} else {
					removePendingByID(toolCallID)
				}
				if toolCallID != "" {
					data["toolCallId"] = toolCallID
				}
				progress("tool_result", fmt.Sprintf("工具结果 (%s)", toolName), data)
			}
		}
	}

	mcpIDsMu.Lock()
	ids := append([]string(nil), mcpIDs...)
	mcpIDsMu.Unlock()

	histJSON, _ := json.Marshal(lastRunMsgs)
	cleaned := strings.TrimSpace(lastAssistant)
	cleaned = dedupeRepeatedParagraphs(cleaned, 80)
	cleaned = dedupeParagraphsByLineFingerprint(cleaned, 100)
	out := &RunResult{
		Response:        cleaned,
		MCPExecutionIDs: ids,
		LastReActInput:  string(histJSON),
		LastReActOutput: cleaned,
	}
	if out.Response == "" {
		out.Response = "（Eino DeepAgent 已完成，但未捕获到助手文本输出。请查看过程详情或日志。）"
		out.LastReActOutput = out.Response
	}
	return out, nil
}

func historyToMessages(history []agent.ChatMessage) []adk.Message {
	if len(history) == 0 {
		return nil
	}
	// 放宽条数上限：跨轮历史交给 Eino Summarization（阈值对齐 openai.max_total_tokens）在调用模型前压缩，避免在入队前硬截断为 40 条。
	const maxHistoryMessages = 300
	start := 0
	if len(history) > maxHistoryMessages {
		start = len(history) - maxHistoryMessages
	}
	out := make([]adk.Message, 0, len(history[start:]))
	for _, h := range history[start:] {
		switch h.Role {
		case "user":
			if strings.TrimSpace(h.Content) != "" {
				out = append(out, schema.UserMessage(h.Content))
			}
		case "assistant":
			if strings.TrimSpace(h.Content) == "" && len(h.ToolCalls) > 0 {
				continue
			}
			if strings.TrimSpace(h.Content) != "" {
				out = append(out, schema.AssistantMessage(h.Content, nil))
			}
		default:
			continue
		}
	}
	return out
}

// mergeStreamingToolCallFragments 将流式多帧的 ToolCall 按 index 合并 arguments（与 schema.concatToolCalls 行为一致）。
func mergeStreamingToolCallFragments(fragments []schema.ToolCall) []schema.ToolCall {
	if len(fragments) == 0 {
		return nil
	}
	m, err := schema.ConcatMessages([]*schema.Message{{ToolCalls: fragments}})
	if err != nil || m == nil {
		return fragments
	}
	return m.ToolCalls
}

// mergeMessageToolCalls 非流式路径上若仍带分片式 tool_calls，合并后再上报 UI。
func mergeMessageToolCalls(msg *schema.Message) *schema.Message {
	if msg == nil || len(msg.ToolCalls) == 0 {
		return msg
	}
	m, err := schema.ConcatMessages([]*schema.Message{msg})
	if err != nil || m == nil {
		return msg
	}
	out := *msg
	out.ToolCalls = m.ToolCalls
	return &out
}

// toolCallStableID 用于流式阶段去重；OpenAI 流式常先给 index 后补 id。
func toolCallStableID(tc schema.ToolCall) string {
	if tc.ID != "" {
		return tc.ID
	}
	if tc.Index != nil {
		return fmt.Sprintf("idx:%d", *tc.Index)
	}
	return ""
}

// toolCallDisplayName 避免前端「未知工具」：DeepAgent 内置 task 等可能延迟写入 function.name。
func toolCallDisplayName(tc schema.ToolCall) string {
	if n := strings.TrimSpace(tc.Function.Name); n != "" {
		return n
	}
	if n := strings.TrimSpace(tc.Type); n != "" && !strings.EqualFold(n, "function") {
		return n
	}
	return "task"
}

// toolCallsSignatureFlush 用于去重键；无 id/index 时用占位 pos，避免流末帧缺 id 时整条工具事件丢失。
func toolCallsSignatureFlush(msg *schema.Message) string {
	if msg == nil || len(msg.ToolCalls) == 0 {
		return ""
	}
	parts := make([]string, 0, len(msg.ToolCalls))
	for i, tc := range msg.ToolCalls {
		id := toolCallStableID(tc)
		if id == "" {
			id = fmt.Sprintf("pos:%d", i)
		}
		parts = append(parts, id+"|"+toolCallDisplayName(tc))
	}
	sort.Strings(parts)
	return strings.Join(parts, ";")
}

// toolCallsRichSignature 用于去重：同一次流式已上报后，紧随其后的非流式消息常带相同 tool_calls。
func toolCallsRichSignature(msg *schema.Message) string {
	base := toolCallsSignatureFlush(msg)
	if base == "" {
		return ""
	}
	parts := make([]string, 0, len(msg.ToolCalls))
	for _, tc := range msg.ToolCalls {
		id := toolCallStableID(tc)
		arg := tc.Function.Arguments
		if len(arg) > 240 {
			arg = arg[:240]
		}
		parts = append(parts, id+":"+arg)
	}
	sort.Strings(parts)
	return base + "|" + strings.Join(parts, ";")
}

func tryEmitToolCallsOnce(
	msg *schema.Message,
	agentName, orchestratorName, conversationID string,
	progress func(string, string, interface{}),
	seen map[string]struct{},
	subAgentToolStep map[string]int,
	markPending func(toolCallPendingInfo),
) {
	if msg == nil || len(msg.ToolCalls) == 0 || progress == nil || seen == nil {
		return
	}
	if toolCallsSignatureFlush(msg) == "" {
		return
	}
	sig := agentName + "\x1e" + toolCallsRichSignature(msg)
	if _, ok := seen[sig]; ok {
		return
	}
	seen[sig] = struct{}{}
	emitToolCallsFromMessage(msg, agentName, orchestratorName, conversationID, progress, subAgentToolStep, markPending)
}

func emitToolCallsFromMessage(
	msg *schema.Message,
	agentName, orchestratorName, conversationID string,
	progress func(string, string, interface{}),
	subAgentToolStep map[string]int,
	markPending func(toolCallPendingInfo),
) {
	if msg == nil || len(msg.ToolCalls) == 0 || progress == nil {
		return
	}
	if subAgentToolStep == nil {
		subAgentToolStep = make(map[string]int)
	}
	isSubToolRound := agentName != "" && agentName != orchestratorName
	if isSubToolRound {
		subAgentToolStep[agentName]++
		n := subAgentToolStep[agentName]
		progress("iteration", "", map[string]interface{}{
			"iteration":      n,
			"einoScope":      "sub",
			"einoRole":       "sub",
			"einoAgent":      agentName,
			"conversationId": conversationID,
			"source":         "eino",
		})
	}
	role := "orchestrator"
	if isSubToolRound {
		role = "sub"
	}
	progress("tool_calls_detected", fmt.Sprintf("检测到 %d 个工具调用", len(msg.ToolCalls)), map[string]interface{}{
		"count":          len(msg.ToolCalls),
		"conversationId": conversationID,
		"source":         "eino",
		"einoAgent":      agentName,
		"einoRole":       role,
	})
	for idx, tc := range msg.ToolCalls {
		argStr := strings.TrimSpace(tc.Function.Arguments)
		if argStr == "" && len(tc.Extra) > 0 {
			if b, mErr := json.Marshal(tc.Extra); mErr == nil {
				argStr = string(b)
			}
		}
		var argsObj map[string]interface{}
		if argStr != "" {
			if uErr := json.Unmarshal([]byte(argStr), &argsObj); uErr != nil || argsObj == nil {
				argsObj = map[string]interface{}{"_raw": argStr}
			}
		}
		display := toolCallDisplayName(tc)
		toolCallID := tc.ID
		if toolCallID == "" && tc.Index != nil {
			toolCallID = fmt.Sprintf("eino-stream-%d", *tc.Index)
		}
		// Record pending tool calls for later tool_result correlation / recovery flushing.
		// We intentionally record even for unknown tools to avoid "running" badge getting stuck.
		if markPending != nil && toolCallID != "" {
			markPending(toolCallPendingInfo{
				ToolCallID: toolCallID,
				ToolName:   display,
				EinoAgent:  agentName,
				EinoRole:   role,
			})
		}
		progress("tool_call", fmt.Sprintf("正在调用工具: %s", display), map[string]interface{}{
			"toolName":       display,
			"arguments":      argStr,
			"argumentsObj":   argsObj,
			"toolCallId":     toolCallID,
			"index":          idx + 1,
			"total":          len(msg.ToolCalls),
			"conversationId": conversationID,
			"source":         "eino",
			"einoAgent":      agentName,
			"einoRole":       role,
		})
	}
}

// dedupeRepeatedParagraphs 去掉完全相同的连续/重复段落，缓解多代理各自复述同一列表。
func dedupeRepeatedParagraphs(s string, minLen int) string {
	if s == "" || minLen <= 0 {
		return s
	}
	paras := strings.Split(s, "\n\n")
	var out []string
	seen := make(map[string]bool)
	for _, p := range paras {
		t := strings.TrimSpace(p)
		if len(t) < minLen {
			out = append(out, p)
			continue
		}
		if seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, p)
	}
	return strings.TrimSpace(strings.Join(out, "\n\n"))
}

// dedupeParagraphsByLineFingerprint 去掉「正文行集合相同」的重复段落（开场白略不同也会合并），缓解多代理各写一遍目录清单。
func dedupeParagraphsByLineFingerprint(s string, minParaLen int) string {
	if s == "" || minParaLen <= 0 {
		return s
	}
	paras := strings.Split(s, "\n\n")
	var out []string
	seen := make(map[string]bool)
	for _, p := range paras {
		t := strings.TrimSpace(p)
		if len(t) < minParaLen {
			out = append(out, p)
			continue
		}
		fp := paragraphLineFingerprint(t)
		// 指纹仅在「≥4 条非空行」时有效；单行/短段落长回复（如自我介绍）fp 为空，必须保留，否则会误删全文并触发「未捕获到助手文本」占位。
		if fp == "" {
			out = append(out, p)
			continue
		}
		if seen[fp] {
			continue
		}
		seen[fp] = true
		out = append(out, p)
	}
	return strings.TrimSpace(strings.Join(out, "\n\n"))
}

func paragraphLineFingerprint(t string) string {
	lines := strings.Split(t, "\n")
	norm := make([]string, 0, len(lines))
	for _, L := range lines {
		s := strings.TrimSpace(L)
		if s == "" {
			continue
		}
		norm = append(norm, s)
	}
	if len(norm) < 4 {
		return ""
	}
	sort.Strings(norm)
	return strings.Join(norm, "\x1e")
}
