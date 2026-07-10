package multiagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"unicode/utf8"

	"cyberstrike-ai/internal/agent"
	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/einomcp"
	"cyberstrike-ai/internal/einoobserve"
	"cyberstrike-ai/internal/openai"
	"cyberstrike-ai/internal/security"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// normalizeStreamingDelta 将可能是“累计片段”的 chunk 归一化为“纯增量”。
// 一些模型/桥接层在流式过程中会重复发送已输出前缀，前端若直接 buffer+=chunk 会出现重复文本。
//
// 注意：与 internal/openai.normalizeStreamingDelta 保持一致。
func normalizeStreamingDelta(current, incoming string) (next, delta string) {
	if incoming == "" {
		return current, ""
	}
	if current == "" {
		return incoming, incoming
	}
	if strings.HasPrefix(incoming, current) && len(incoming) > len(current) {
		return incoming, incoming[len(current):]
	}
	if incoming == current && utf8.RuneCountInString(current) > 1 {
		return current, ""
	}
	return current + incoming, incoming
}

func isInterruptContinue(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	return errors.Is(context.Cause(ctx), ErrInterruptContinue)
}

func isEinoIterationLimitError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	if msg == "" {
		return false
	}
	return strings.Contains(msg, "max iteration") ||
		strings.Contains(msg, "maximum iteration") ||
		strings.Contains(msg, "maximum iterations") ||
		strings.Contains(msg, "iteration limit") ||
		strings.Contains(msg, "达到最大迭代")
}

// einoADKRunLoopArgs 将 Eino adk.Runner 事件循环从 RunDeepAgent / RunEinoSingleChatModelAgent 中抽出复用。
type einoADKRunLoopArgs struct {
	OrchMode             string
	OrchestratorName     string
	ConversationID       string
	Progress             func(eventType, message string, data interface{})
	Logger               *zap.Logger
	SnapshotMCPIDs       func() []string
	StreamsMainAssistant func(agent string) bool
	EinoRoleTag          func(agent string) string
	CheckpointDir        string
	// RunRetryMaxAttempts / RunRetryMaxBackoffSec：429、5xx、网络抖动时的指数退避续跑（0=默认 10 次 / 30s 上限）。
	RunRetryMaxAttempts   int
	RunRetryMaxBackoffSec int

	McpIDsMu *sync.Mutex
	McpIDs   *[]string

	// FilesystemMonitorAgent / FilesystemMonitorRecord 非 nil 时，将 Eino ADK filesystem 中间件工具（ls/read_file/write_file/edit_file/glob/grep）
	// 在完成时写入 MCP 监控；execute 仍由 eino_execute_monitor 记录，此处跳过。
	FilesystemMonitorAgent  *agent.Agent
	FilesystemMonitorRecord einomcp.ExecutionRecorder
	MCPExecutionBinder      *MCPExecutionBinder

	// ToolInvokeNotify 与 einomcp.ToolsFromDefinitions 共享：run loop 在迭代前 Set，execute/MCP 桥 Fire 时立即推送 tool_result（ADK 晚到经 toolResultSent 去重）。
	ToolInvokeNotify *einomcp.ToolInvokeNotifyHolder

	DA adk.Agent

	// EmptyResponseMessage 当未捕获到助手正文时的占位（多代理与单代理文案不同）。
	EmptyResponseMessage string

	// ModelFacingTrace 可选：由各 ChatModelAgent Handlers 链末尾中间件写入「即将送入模型」的消息快照；
	// 非空时优先用于 LastAgentTraceInput 序列化，使续跑与 summarization/reduction 后的上下文一致。
	ModelFacingTrace *modelFacingTraceHolder

	// EinoCallbacks 可选：为 ADK Runner 注入 eino [callbacks] 全链路观测（见 internal/einoobserve）。
	EinoCallbacks *config.MultiAgentEinoCallbacksConfig
}

func runEinoADKAgentLoop(ctx context.Context, args *einoADKRunLoopArgs, baseMsgs []adk.Message) (*RunResult, error) {
	if args == nil || args.DA == nil {
		return nil, fmt.Errorf("eino run loop: args 或 Agent 为空")
	}
	if args.McpIDs == nil {
		s := []string{}
		args.McpIDs = &s
	}
	if args.McpIDsMu == nil {
		args.McpIDsMu = &sync.Mutex{}
	}

	orchMode := args.OrchMode
	orchestratorName := args.OrchestratorName
	conversationID := args.ConversationID
	progress := args.Progress
	logger := args.Logger
	snapshotMCPIDs := args.SnapshotMCPIDs
	if snapshotMCPIDs == nil {
		snapshotMCPIDs = func() []string { return nil }
	}
	streamsMainAssistant := args.StreamsMainAssistant
	if streamsMainAssistant == nil {
		streamsMainAssistant = func(agent string) bool {
			return agent == "" || agent == orchestratorName
		}
	}
	einoRoleTag := args.EinoRoleTag
	if einoRoleTag == nil {
		einoRoleTag = func(agent string) string {
			if streamsMainAssistant(agent) {
				return "orchestrator"
			}
			return "sub"
		}
	}
	da := args.DA
	mcpIDsMu := args.McpIDsMu
	mcpIDs := args.McpIDs

	// panic recovery：防止 Eino 框架内部 panic 导致整个 goroutine 崩溃、连接无法正常关闭。
	defer func() {
		if r := recover(); r != nil {
			if logger != nil {
				logger.Error("eino runner panic recovered", zap.Any("recover", r), zap.Stack("stack"))
			}
			if progress != nil {
				progress("error", fmt.Sprintf("Internal error: %v / 内部错误: %v", r, r), map[string]interface{}{
					"conversationId": conversationID,
					"source":         "eino",
				})
			}
		}
	}()

	var lastAssistant string
	var lastPlanExecuteExecutor string
	msgs := append([]adk.Message(nil), baseMsgs...)
	runAccumulatedMsgs := append([]adk.Message(nil), msgs...)
	baseAccumulatedCount := len(runAccumulatedMsgs)

	emptyHint := strings.TrimSpace(args.EmptyResponseMessage)
	if emptyHint == "" {
		emptyHint = "(Eino session completed but no assistant text was captured. Check process details or logs.) " +
			"（Eino 会话已完成，但未捕获到助手文本输出。请查看过程详情或日志。）"
	}

	lastAssistant = ""
	lastPlanExecuteExecutor = ""
	var reasoningStreamSeq int64
	var einoSubReplyStreamSeq int64
	var mainResponseStreamSeq int64
	toolEmitSeen := make(map[string]struct{})
	var einoMainRound int
	var einoLastAgent string
	subAgentToolStep := make(map[string]int)
	// mainAgentToolStep：主代理每次工具调用批次递增，供 UI 显示「第 N 轮」（单代理无子代理切换时原先会一直停在第 1 轮）。
	mainAgentToolStep := make(map[string]int)
	pendingByID := make(map[string]toolCallPendingInfo)
	pendingQueueByAgent := make(map[string][]string)
	var pendingMu sync.Mutex
	markPending := func(tc toolCallPendingInfo) {
		if tc.ToolCallID == "" {
			return
		}
		pendingMu.Lock()
		defer pendingMu.Unlock()
		pendingByID[tc.ToolCallID] = tc
		pendingQueueByAgent[tc.EinoAgent] = append(pendingQueueByAgent[tc.EinoAgent], tc.ToolCallID)
	}
	markPendingWithMonitor := func(tc toolCallPendingInfo) {
		markPending(tc)
		beginEinoADKFilesystemToolMonitor(
			ctx,
			args.FilesystemMonitorAgent,
			args.FilesystemMonitorRecord,
			args.MCPExecutionBinder,
			tc.ToolCallID,
			tc.ToolName,
		)
	}
	popNextPendingForAgent := func(agentName string) (toolCallPendingInfo, bool) {
		pendingMu.Lock()
		defer pendingMu.Unlock()
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
		pendingMu.Lock()
		defer pendingMu.Unlock()
		delete(pendingByID, toolCallID)
	}
	popAnyPending := func() (toolCallPendingInfo, bool) {
		pendingMu.Lock()
		defer pendingMu.Unlock()
		for id, tc := range pendingByID {
			delete(pendingByID, id)
			return tc, true
		}
		return toolCallPendingInfo{}, false
	}
	pendingCount := func() int {
		pendingMu.Lock()
		defer pendingMu.Unlock()
		return len(pendingByID)
	}
	flushAllPendingAsFailed := func(err error) {
		pendingMu.Lock()
		pendingSnapshot := make([]toolCallPendingInfo, 0, len(pendingByID))
		for _, tc := range pendingByID {
			pendingSnapshot = append(pendingSnapshot, tc)
		}
		pendingByID = make(map[string]toolCallPendingInfo)
		pendingQueueByAgent = make(map[string][]string)
		pendingMu.Unlock()

		if progress == nil {
			return
		}
		msg := ""
		if err != nil {
			msg = err.Error()
		}
		for _, tc := range pendingSnapshot {
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
	}

	// 最近一次成功的 Eino filesystem execute 的标准输出（trim）：用于抑制模型紧接着复述同一字符串时的重复「助手输出」时间线。
	var executeStdoutDupMu sync.Mutex
	var pendingExecuteStdoutDup string
	recordPendingExecuteStdoutDup := func(toolName, stdout string, isErr bool) {
		if isErr || !strings.EqualFold(strings.TrimSpace(toolName), "execute") {
			return
		}
		t := strings.TrimSpace(stdout)
		if t == "" {
			return
		}
		executeStdoutDupMu.Lock()
		pendingExecuteStdoutDup = t
		executeStdoutDupMu.Unlock()
	}

	var toolResultSent sync.Map // toolCallID -> struct{}；ADK Tool 事件去重（权威正文来自 reduction 处理后的 agent 上下文）
	tryEmitToolResultProgress := func(toolName, content, toolCallID string, isErr bool, agentName string) {
		// 仅由 ADK schema.Tool 事件调用；MCP/execute 桥在 reduction 前的 ToolInvokeNotify 不得推送 tool_result，
		// 否则全量输出会先占位并触发 toolResultSent 去重，导致 UI/监控展示与 agent 实际收到的截断正文不一致。
		toolName = strings.TrimSpace(toolName)
		if toolName == "" {
			toolName = "unknown"
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
			"agentFacing":    true, // 与 reduction 后送入 ChatModel 的正文一致，供前端展示
			"conversationId": conversationID,
			"einoAgent":      agentName,
			"einoRole":       einoRoleTag(agentName),
			"source":         "eino",
		}
		tid := strings.TrimSpace(toolCallID)
		if tid == "" {
			if inferred, ok := popNextPendingForAgent(agentName); ok {
				tid = inferred.ToolCallID
			} else if inferred, ok := popNextPendingForAgent(orchestratorName); ok {
				tid = inferred.ToolCallID
			} else if inferred, ok := popNextPendingForAgent(""); ok {
				tid = inferred.ToolCallID
			} else if inferred, ok := popAnyPending(); ok {
				tid = inferred.ToolCallID
			}
		}
		if tid != "" {
			removePendingByID(tid)
			if _, loaded := toolResultSent.LoadOrStore(tid, struct{}{}); loaded {
				return
			}
			data["toolCallId"] = tid
			toolCallID = tid
		}
		recordPendingExecuteStdoutDup(toolName, content, isErr)
		recordEinoADKFilesystemToolMonitor(ctx, args.FilesystemMonitorAgent, args.FilesystemMonitorRecord, args.MCPExecutionBinder, toolName, toolCallID, runAccumulatedMsgs, content, isErr)
		if args.FilesystemMonitorAgent != nil && args.MCPExecutionBinder != nil {
			if execID := args.MCPExecutionBinder.ExecutionID(toolCallID); execID != "" {
				args.FilesystemMonitorAgent.UpdateMCPExecutionDisplayResult(execID, content)
			}
		}
		if progress != nil {
			progress("tool_result", fmt.Sprintf("工具结果 (%s)", toolName), data)
		}
	}

	if args.EinoCallbacks != nil {
		ctx = einoobserve.AttachAgentRunCallbacks(ctx, args.EinoCallbacks, einoobserve.Params{
			Logger:           logger,
			Progress:         progress,
			ConversationID:   conversationID,
			OrchMode:         orchMode,
			OrchestratorName: orchestratorName,
		})
	}

	runnerCfg := adk.RunnerConfig{
		Agent: da,
		// 启用 ADK 流式事件：plan_execute 也需要输出 reasoning/response 流，
		// 与 deep/supervisor/eino_single 的前端体验保持一致。
		EnableStreaming: true,
	}
	var cpStore *fileCheckPointStore
	var checkPointID string
	if cp := strings.TrimSpace(args.CheckpointDir); cp != "" {
		cpDir := filepath.Join(cp, sanitizeEinoPathSegment(conversationID))
		st, stErr := newFileCheckPointStore(cpDir)
		if stErr != nil {
			if logger != nil {
				logger.Warn("eino checkpoint store disabled", zap.String("dir", cpDir), zap.Error(stErr))
			}
		} else {
			cpStore = st
			checkPointID = buildEinoCheckpointID(orchMode)
			runnerCfg.CheckPointStore = st
			if logger != nil {
				logger.Info("eino runner: checkpoint store enabled",
					zap.String("dir", cpDir),
					zap.String("checkPointID", checkPointID))
			}
		}
	}
	runner := adk.NewRunner(ctx, runnerCfg)
	startRunnerIter := func(runMsgs []adk.Message) *adk.AsyncIterator[*adk.AgentEvent] {
		if checkPointID != "" {
			return runner.Run(ctx, runMsgs, adk.WithCheckPointID(checkPointID))
		}
		return runner.Run(ctx, runMsgs)
	}
	var iter *adk.AsyncIterator[*adk.AgentEvent]
	if cpStore != nil && checkPointID != "" {
		if _, existed, getErr := cpStore.Get(ctx, checkPointID); getErr != nil {
			if logger != nil {
				logger.Warn("eino checkpoint preflight get failed", zap.String("checkPointID", checkPointID), zap.Error(getErr))
			}
		} else if existed {
			if progress != nil {
				progress("progress", "检测到断点，正在从中断节点恢复执行...", map[string]interface{}{
					"conversationId": conversationID,
					"source":         "eino",
					"orchestration":  orchMode,
					"checkPointID":   checkPointID,
				})
			}
			if logger != nil {
				logger.Info("eino runner: resume from checkpoint", zap.String("checkPointID", checkPointID))
			}
			resumeIter, resumeErr := runner.Resume(ctx, checkPointID)
			if resumeErr == nil {
				iter = resumeIter
			} else {
				if logger != nil {
					logger.Warn("eino runner: resume failed, fallback to fresh run",
						zap.String("checkPointID", checkPointID),
						zap.Error(resumeErr))
				}
				if progress != nil {
					progress("progress", "断点恢复失败，已回退为全新执行。", map[string]interface{}{
						"conversationId": conversationID,
						"source":         "eino",
						"orchestration":  orchMode,
						"checkPointID":   checkPointID,
					})
				}
			}
		}
	}
	if iter == nil {
		iter = startRunnerIter(msgs)
	}
	transientRetrier := newEinoTransientRunRetrier(einoTransientRunRetryPolicyFromArgs(args))
	handleRunErr := func(runErr error) error {
		if runErr == nil {
			return nil
		}
		if errors.Is(runErr, context.DeadlineExceeded) {
			flushAllPendingAsFailed(runErr)
			if progress != nil {
				progress("error", runErr.Error(), map[string]interface{}{
					"conversationId": conversationID,
					"source":         "eino",
					"errorKind":      "timeout",
				})
			}
			return runErr
		}
		// context.Canceled 是唯一应当直接终止编排的错误（用户关闭页面、主动停止等）。
		if errors.Is(runErr, context.Canceled) {
			flushAllPendingAsFailed(runErr)
			if progress != nil {
				progress("error", runErr.Error(), map[string]interface{}{
					"conversationId": conversationID,
					"source":         "eino",
				})
			}
			return runErr
		}
		if isEinoIterationLimitError(runErr) {
			flushAllPendingAsFailed(runErr)
			if progress != nil {
				progress("iteration_limit_reached", runErr.Error(), map[string]interface{}{
					"conversationId": conversationID,
					"source":         "eino",
					"orchestration":  orchMode,
				})
				progress("error", runErr.Error(), map[string]interface{}{
					"conversationId": conversationID,
					"source":         "eino",
					"errorKind":      "iteration_limit",
				})
			}
			return runErr
		}
		flushAllPendingAsFailed(runErr)
		if progress != nil {
			progress("error", runErr.Error(), map[string]interface{}{
				"conversationId": conversationID,
				"source":         "eino",
			})
		}
		return runErr
	}

	maybeRetryTransientRun := func(runErr error) (restarted bool, fatal error) {
		if runErr == nil {
			return false, nil
		}
		if !isEinoTransientRunError(runErr) {
			return false, handleRunErr(runErr)
		}
		restarted, restartMsgs, ctxSource, backoff, retErr := transientRetrier.tryRetry(
			ctx, runErr, args, baseMsgs, runAccumulatedMsgs, baseAccumulatedCount,
		)
		if retErr != nil {
			flushAllPendingAsFailed(runErr)
			if logger != nil {
				logger.Warn("eino transient retry exhausted",
					zap.Error(retErr),
					zap.String("orchestration", orchMode),
					zap.Int("maxAttempts", transientRetrier.maxAttempts()))
			}
			return false, retErr
		}
		if !restarted {
			return false, nil
		}
		attemptNo := transientRetrier.attempt()
		maxAttempts := transientRetrier.maxAttempts()
		if logger != nil {
			logger.Warn("eino transient error, retrying after backoff",
				zap.Error(runErr),
				zap.String("orchestration", orchMode),
				zap.Int("attempt", attemptNo),
				zap.Int("maxAttempts", maxAttempts),
				zap.Duration("backoff", backoff))
		}
		if progress != nil {
			progress("eino_run_retry", fmt.Sprintf("遇到临时错误（限流或网络波动），%d 秒后第 %d/%d 次重试…", int(backoff.Seconds()), attemptNo, maxAttempts), map[string]interface{}{
				"conversationId": conversationID,
				"source":         "eino",
				"orchestration":  orchMode,
				"error":          runErr.Error(),
				"attempt":        attemptNo,
				"maxAttempts":    maxAttempts,
				"backoffSec":     int(backoff.Seconds()),
			})
			progress("eino_run_retry", "已恢复上下文，正在重试…", map[string]interface{}{
				"conversationId": conversationID,
				"source":         "eino",
				"orchestration":  orchMode,
				"attempt":        attemptNo,
				"contextSource":  string(ctxSource),
			})
		}
		msgs = restartMsgs
		iter = startRunnerIter(msgs)
		return true, nil
	}

	// 仅在退避重试后真正收到数据/完成一步时清零，避免重启后首个无错 ADK 事件误把计数打回 0。
	confirmTransientRetryRecovery := func() {
		if transientRetrier.attempt() > 0 {
			transientRetrier.reset()
		}
	}

	takePartial := func(runErr error) (*RunResult, error) {
		if len(runAccumulatedMsgs) <= baseAccumulatedCount {
			return nil, runErr
		}
		ids := snapshotMCPIDs()
		return buildEinoRunResultFromAccumulated(
			orchMode, runAccumulatedMsgs, modelFacingTraceSnapshot(args),
			lastAssistant, lastPlanExecuteExecutor, emptyHint, ids, true,
		), runErr
	}

	for {
		// iter.Next 可能长时间阻塞（工具执行、模型推理）；须与 ctx 联动，否则取消/超时无法及时 flush pending。
		ev, ok, iterCtxErr := nextAgentEventWithContext(ctx, iter)
		if iterCtxErr != nil {
			flushAllPendingAsFailed(iterCtxErr)
			if progress != nil {
				if isInterruptContinue(ctx) {
					progress("progress", "已暂停当前输出，正在合并用户补充并继续…", map[string]interface{}{
						"conversationId": conversationID,
						"source":         "eino",
						"kind":           "interrupt_continue",
					})
				} else {
					progress("error", iterCtxErr.Error(), map[string]interface{}{
						"conversationId": conversationID,
						"source":         "eino",
					})
				}
			}
			return takePartial(iterCtxErr)
		}
		if !ok {
			// iter 结束并不总是“正常完成”：
			// 当取消/超时发生在 iter.Next() 阻塞期间时，可能直接返回 !ok。
			// 此时必须保留 checkpoint，避免后续恢复时被误判为“无断点”而全量重跑。
			if ctxErr := ctx.Err(); ctxErr != nil {
				flushAllPendingAsFailed(ctxErr)
				if progress != nil {
					if isInterruptContinue(ctx) {
						progress("progress", "已暂停当前输出，正在合并用户补充并继续…", map[string]interface{}{
							"conversationId": conversationID,
							"source":         "eino",
							"kind":           "interrupt_continue",
						})
					} else {
						progress("error", ctxErr.Error(), map[string]interface{}{
							"conversationId": conversationID,
							"source":         "eino",
						})
					}
				}
				return takePartial(ctxErr)
			}
			if orphanCount := pendingCount(); orphanCount > 0 {
				flushAllPendingAsFailed(errors.New("pending tool call missing result before run completion"))
				if progress != nil {
					progress("eino_pending_orphaned", "pending tool calls were force-closed at run end", map[string]interface{}{
						"conversationId": conversationID,
						"source":         "eino",
						"orchestration":  orchMode,
						"pendingCount":   orphanCount,
					})
				}
			}
			if cpStore != nil && checkPointID != "" {
				if p, pErr := cpStore.path(checkPointID); pErr == nil {
					if rmErr := os.Remove(p); rmErr != nil && !os.IsNotExist(rmErr) && logger != nil {
						logger.Warn("eino checkpoint cleanup failed", zap.String("path", p), zap.Error(rmErr))
					}
				}
			}
			break
		}
		if ev == nil {
			continue
		}
		if ev.Err != nil {
			restarted, retErr := maybeRetryTransientRun(ev.Err)
			if retErr != nil {
				return takePartial(retErr)
			}
			if restarted {
				continue
			}
		}
		if ev.AgentName != "" && progress != nil {
			iterEinoAgent := orchestratorName
			if orchMode == "plan_execute" {
				if a := strings.TrimSpace(ev.AgentName); a != "" {
					iterEinoAgent = a
				}
			}
			if streamsMainAssistant(ev.AgentName) {
				mainIterKey := einoMainIterationKey(iterEinoAgent, orchestratorName)
				if einoMainRound == 0 {
					einoMainRound = 1
					mainAgentToolStep[mainIterKey] = 1
					progress("iteration", "", map[string]interface{}{
						"iteration":      1,
						"einoScope":      "main",
						"einoRole":       "orchestrator",
						"einoAgent":      iterEinoAgent,
						"orchestration":  orchMode,
						"conversationId": conversationID,
						"source":         "eino",
					})
				} else if einoLastAgent != "" {
					needBump := false
					if !streamsMainAssistant(einoLastAgent) {
						needBump = true // 子代理 → 主代理
					} else if einoLastAgent != ev.AgentName {
						needBump = true // plan_execute：planner ↔ executor 等主代理切换
					}
					if needBump {
						einoMainRound++
						mainAgentToolStep[mainIterKey] = einoMainRound
						progress("iteration", "", map[string]interface{}{
							"iteration":      einoMainRound,
							"einoScope":      "main",
							"einoRole":       "orchestrator",
							"einoAgent":      iterEinoAgent,
							"orchestration":  orchMode,
							"conversationId": conversationID,
							"source":         "eino",
						})
					}
				}
			}
			// 仅在代理切换时更新进度标题；同一代理的每个 ADK 事件不再重复刷 progress。
			if einoLastAgent != ev.AgentName {
				progress("progress", fmt.Sprintf("[Eino] %s", ev.AgentName), map[string]interface{}{
					"conversationId": conversationID,
					"einoAgent":      ev.AgentName,
					"einoRole":       einoRoleTag(ev.AgentName),
					"orchestration":  orchMode,
				})
			}
			einoLastAgent = ev.AgentName
		}
		if ev.Output == nil || ev.Output.MessageOutput == nil {
			continue
		}
		mv := ev.Output.MessageOutput

		if mv.IsStreaming && mv.MessageStream != nil && mv.Role == schema.Tool {
			toolName := strings.TrimSpace(mv.ToolName)
			content, streamToolCallID, toolStreamRecvErr := recvSchemaMessageStream(ctx, mv.MessageStream)
			isErr := einoToolResultIsError(toolName, content)
			content = einoToolResultBody(content)
			if streamToolCallID != "" {
				opts := []schema.ToolMessageOption{schema.WithToolName(toolName)}
				runAccumulatedMsgs = append(runAccumulatedMsgs, schema.ToolMessage(content, streamToolCallID, opts...))
			}
			tryEmitToolResultProgress(toolName, content, streamToolCallID, isErr, ev.AgentName)
			if toolStreamRecvErr != nil && logger != nil {
				logger.Warn("eino tool result stream recv error",
					zap.Error(toolStreamRecvErr),
					zap.String("agent", ev.AgentName),
					zap.String("tool", toolName))
			}
			if toolStreamRecvErr == nil {
				confirmTransientRetryRecovery()
			}
			continue
		}

		if mv.IsStreaming && mv.MessageStream != nil {
			mainStreamID := fmt.Sprintf("eino-main-%s-%d", conversationID, atomic.AddInt64(&mainResponseStreamSeq, 1))
			streamHeaderSent := false
			var reasoningStreamID string
			var toolStreamFragments []schema.ToolCall
			var subAssistantBuf string
			var subReplyStreamID string
			var mainAssistantBuf string
			// 已通过 response_delta 推到前端的正文（与 monitor.js normalizeStreamingDeltaJs 累积一致）
			var mainAssistWireAccum string
			var mainAssistDupTarget string // 非空表示本段主助手流需缓冲至 EOF，与 execute 输出比对去重
			var reasoningBuf string
			var prevReasoningDisplay string // UI 用：剥离 Claude 内部 signature 尾缀后的累计展示
			var streamRecvErr error
			type streamMsg struct {
				chunk *schema.Message
				err   error
			}
			recvCh := make(chan streamMsg, 8)
			go func() {
				defer close(recvCh)
				for {
					ch, rerr := mv.MessageStream.Recv()
					recvCh <- streamMsg{chunk: ch, err: rerr}
					if rerr != nil {
						return
					}
				}
			}()
		streamRecvLoop:
			for {
				select {
				case <-ctx.Done():
					streamRecvErr = ctx.Err()
					break streamRecvLoop
				case sm, ok := <-recvCh:
					if !ok {
						break streamRecvLoop
					}
					chunk, rerr := sm.chunk, sm.err
					if rerr != nil {
						if errors.Is(rerr, io.EOF) {
							break streamRecvLoop
						}
						if logger != nil {
							logger.Warn("eino stream recv error, flushing incomplete stream",
								zap.Error(rerr),
								zap.String("agent", ev.AgentName),
								zap.Int("toolFragments", len(toolStreamFragments)))
						}
						streamRecvErr = rerr
						break streamRecvLoop
					}
					if chunk == nil {
						continue
					}
					if progress != nil && strings.TrimSpace(chunk.ReasoningContent) != "" {
						var reasoningDelta string
						reasoningBuf, reasoningDelta = normalizeStreamingDelta(reasoningBuf, chunk.ReasoningContent)
						if reasoningDelta != "" {
							fullDisplay := openai.DisplayReasoningContent(reasoningBuf)
							var displayDelta string
							if strings.HasPrefix(fullDisplay, prevReasoningDisplay) {
								displayDelta = fullDisplay[len(prevReasoningDisplay):]
							} else {
								displayDelta = fullDisplay
							}
							prevReasoningDisplay = fullDisplay
							if displayDelta != "" {
								if reasoningStreamID == "" {
									reasoningStreamID = fmt.Sprintf("eino-reasoning-%s-%d", conversationID, atomic.AddInt64(&reasoningStreamSeq, 1))
									progress("reasoning_chain_stream_start", " ", map[string]interface{}{
										"streamId":      reasoningStreamID,
										"source":        "eino",
										"einoAgent":     ev.AgentName,
										"einoRole":      einoRoleTag(ev.AgentName),
										"orchestration": orchMode,
									})
								}
								progress("reasoning_chain_stream_delta", displayDelta, openai.WithSSEAccumulated(map[string]interface{}{
									"streamId": reasoningStreamID,
								}, fullDisplay))
							}
						}
					}
					if chunk.Content != "" {
						if progress != nil && streamsMainAssistant(ev.AgentName) {
							var contentDelta string
							mainAssistantBuf, contentDelta = normalizeStreamingDelta(mainAssistantBuf, chunk.Content)
							if contentDelta != "" {
								if mainAssistDupTarget == "" {
									executeStdoutDupMu.Lock()
									if pendingExecuteStdoutDup != "" {
										mainAssistDupTarget = pendingExecuteStdoutDup
									}
									executeStdoutDupMu.Unlock()
								}
								if mainAssistDupTarget != "" {
									// 已展示过 tool_result，缓冲全文；EOF 后与 execute 输出相同则不再发助手流
								} else {
									if !streamHeaderSent {
										progress("response_start", "", map[string]interface{}{
											"conversationId":     conversationID,
											"mcpExecutionIds":    snapshotMCPIDs(),
											"messageGeneratedBy": "eino:" + ev.AgentName,
											"einoRole":           "orchestrator",
											"einoAgent":          ev.AgentName,
											"orchestration":      orchMode,
											"iteration":          einoMainRound,
											"streamId":           mainStreamID,
										})
										streamHeaderSent = true
									}
									progress("response_delta", contentDelta, openai.WithSSEAccumulated(map[string]interface{}{
										"conversationId":  conversationID,
										"mcpExecutionIds": snapshotMCPIDs(),
										"einoRole":        "orchestrator",
										"einoAgent":       ev.AgentName,
										"orchestration":   orchMode,
										"iteration":       einoMainRound,
										"streamId":        mainStreamID,
									}, mainAssistantBuf))
									mainAssistWireAccum, _ = normalizeStreamingDelta(mainAssistWireAccum, contentDelta)
								}
							}
						} else if !streamsMainAssistant(ev.AgentName) {
							var subDelta string
							subAssistantBuf, subDelta = normalizeStreamingDelta(subAssistantBuf, chunk.Content)
							if subDelta != "" {
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
									progress("eino_agent_reply_stream_delta", subDelta, openai.WithSSEAccumulated(map[string]interface{}{
										"streamId":       subReplyStreamID,
										"conversationId": conversationID,
									}, subAssistantBuf))
								}
							}
						}
					}
					if len(chunk.ToolCalls) > 0 {
						toolStreamFragments = append(toolStreamFragments, chunk.ToolCalls...)
					}
				}
			}
			if progress != nil && reasoningStreamID != "" && strings.TrimSpace(reasoningBuf) != "" {
				progress("reasoning_chain_stream_end", openai.DisplayReasoningContent(strings.TrimSpace(reasoningBuf)), map[string]interface{}{
					"streamId":       reasoningStreamID,
					"conversationId": conversationID,
					"source":         "eino",
					"einoAgent":      ev.AgentName,
					"einoRole":       einoRoleTag(ev.AgentName),
					"orchestration":  orchMode,
				})
			}
			if streamsMainAssistant(ev.AgentName) {
				s := strings.TrimSpace(mainAssistantBuf)
				if mainAssistDupTarget != "" {
					executeStdoutDupMu.Lock()
					pendingExecuteStdoutDup = ""
					executeStdoutDupMu.Unlock()
					if s != "" && s == mainAssistDupTarget {
						// 与刚展示的 execute 结果完全一致：不再发助手流式事件，仍写入轨迹与最终回复字段
						lastAssistant = s
						runAccumulatedMsgs = append(runAccumulatedMsgs, schema.AssistantMessage(s, nil))
						if orchMode == "plan_execute" && strings.EqualFold(strings.TrimSpace(ev.AgentName), "executor") {
							lastPlanExecuteExecutor = UnwrapPlanExecuteUserText(s)
						}
					} else if s != "" {
						if progress != nil {
							// 仅用 TrimSpace 与 execute 比对；推到 UI 的必须是 mainAssistantBuf，
							// 否则尾部空白/换行与已流式前缀不一致时，前端 normalize 会走拼接路径造成叠字。
							_, eofTail := normalizeStreamingDelta(mainAssistWireAccum, mainAssistantBuf)
							if eofTail != "" {
								if !streamHeaderSent {
									progress("response_start", "", map[string]interface{}{
										"conversationId":     conversationID,
										"mcpExecutionIds":    snapshotMCPIDs(),
										"messageGeneratedBy": "eino:" + ev.AgentName,
										"einoRole":           "orchestrator",
										"einoAgent":          ev.AgentName,
										"orchestration":      orchMode,
										"iteration":          einoMainRound,
										"streamId":           mainStreamID,
									})
								}
								progress("response_delta", eofTail, openai.WithSSEAccumulated(map[string]interface{}{
									"conversationId":  conversationID,
									"mcpExecutionIds": snapshotMCPIDs(),
									"einoRole":        "orchestrator",
									"einoAgent":       ev.AgentName,
									"orchestration":   orchMode,
									"iteration":       einoMainRound,
									"streamId":        mainStreamID,
								}, mainAssistantBuf))
								mainAssistWireAccum, _ = normalizeStreamingDelta(mainAssistWireAccum, eofTail)
							}
						}
						lastAssistant = s
						runAccumulatedMsgs = append(runAccumulatedMsgs, schema.AssistantMessage(s, nil))
						if orchMode == "plan_execute" && strings.EqualFold(strings.TrimSpace(ev.AgentName), "executor") {
							lastPlanExecuteExecutor = UnwrapPlanExecuteUserText(s)
						}
					}
				} else if s != "" {
					lastAssistant = s
					runAccumulatedMsgs = append(runAccumulatedMsgs, schema.AssistantMessage(s, nil))
					if orchMode == "plan_execute" && strings.EqualFold(strings.TrimSpace(ev.AgentName), "executor") {
						lastPlanExecuteExecutor = UnwrapPlanExecuteUserText(s)
					}
				}
			}
			if strings.TrimSpace(subAssistantBuf) != "" && progress != nil {
				if s := strings.TrimSpace(subAssistantBuf); s != "" {
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
				lastToolChunk = mergeMessageToolCalls(&schema.Message{ToolCalls: merged})
			}
			tryEmitToolCallsOnce(lastToolChunk, ev.AgentName, orchestratorName, conversationID, orchMode, progress, toolEmitSeen, subAgentToolStep, mainAgentToolStep, markPendingWithMonitor)
			// 流式路径此前只把 tool_calls 推给进度 UI，未写入 runAccumulatedMsgs；落库后 loadHistory→RepairOrphan 会删掉全部 tool 结果，表现为「续跑/下轮失忆」。
			if lastToolChunk != nil && len(lastToolChunk.ToolCalls) > 0 {
				runAccumulatedMsgs = append(runAccumulatedMsgs, schema.AssistantMessage("", lastToolChunk.ToolCalls))
			}
			if streamRecvErr != nil {
				if isInterruptContinue(ctx) {
					return takePartial(streamRecvErr)
				}
				if progress != nil {
					progress("eino_stream_error", streamRecvErr.Error(), map[string]interface{}{
						"conversationId": conversationID,
						"source":         "eino",
						"einoAgent":      ev.AgentName,
						"einoRole":       einoRoleTag(ev.AgentName),
					})
				}
				restarted, retErr := maybeRetryTransientRun(streamRecvErr)
				if retErr != nil {
					return takePartial(retErr)
				}
				if restarted {
					continue
				}
			} else {
				confirmTransientRetryRecovery()
			}
			continue
		}

		msg, gerr := mv.GetMessage()
		if gerr != nil || msg == nil {
			continue
		}
		runAccumulatedMsgs = append(runAccumulatedMsgs, msg)
		tryEmitToolCallsOnce(mergeMessageToolCalls(msg), ev.AgentName, orchestratorName, conversationID, orchMode, progress, toolEmitSeen, subAgentToolStep, mainAgentToolStep, markPendingWithMonitor)

		if mv.Role == schema.Assistant {
			if progress != nil && strings.TrimSpace(msg.ReasoningContent) != "" {
				progress("reasoning_chain", openai.DisplayReasoningContent(strings.TrimSpace(msg.ReasoningContent)), map[string]interface{}{
					"conversationId": conversationID,
					"source":         "eino",
					"einoAgent":      ev.AgentName,
					"einoRole":       einoRoleTag(ev.AgentName),
					"orchestration":  orchMode,
				})
			}
			body := strings.TrimSpace(msg.Content)
			if body != "" {
				if streamsMainAssistant(ev.AgentName) {
					executeStdoutDupMu.Lock()
					dup := pendingExecuteStdoutDup
					if dup != "" && body == dup {
						pendingExecuteStdoutDup = ""
						executeStdoutDupMu.Unlock()
						lastAssistant = body
						if orchMode == "plan_execute" && strings.EqualFold(strings.TrimSpace(ev.AgentName), "executor") {
							lastPlanExecuteExecutor = UnwrapPlanExecuteUserText(body)
						}
						// 非流式：与 execute 输出相同则跳过助手通道展示（msg 已在上方写入 runAccumulatedMsgs）
					} else {
						if dup != "" {
							pendingExecuteStdoutDup = ""
						}
						executeStdoutDupMu.Unlock()
						if progress != nil {
							nonStreamID := fmt.Sprintf("eino-main-%s-%d", conversationID, atomic.AddInt64(&mainResponseStreamSeq, 1))
							progress("response_start", "", map[string]interface{}{
								"conversationId":     conversationID,
								"mcpExecutionIds":    snapshotMCPIDs(),
								"messageGeneratedBy": "eino:" + ev.AgentName,
								"einoRole":           "orchestrator",
								"einoAgent":          ev.AgentName,
								"orchestration":      orchMode,
								"iteration":          einoMainRound,
								"streamId":           nonStreamID,
							})
							progress("response_delta", body, openai.WithSSEAccumulated(map[string]interface{}{
								"conversationId":  conversationID,
								"mcpExecutionIds": snapshotMCPIDs(),
								"einoRole":        "orchestrator",
								"einoAgent":       ev.AgentName,
								"orchestration":   orchMode,
								"iteration":       einoMainRound,
								"streamId":        nonStreamID,
							}, body))
						}
						lastAssistant = body
						if orchMode == "plan_execute" && strings.EqualFold(strings.TrimSpace(ev.AgentName), "executor") {
							lastPlanExecuteExecutor = UnwrapPlanExecuteUserText(body)
						}
					}
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

		if (mv.Role == schema.Tool || msg.Role == schema.Tool) && progress != nil {
			toolName := msg.ToolName
			if toolName == "" {
				toolName = mv.ToolName
			}

			content := msg.Content
			isErr := einoToolResultIsError(toolName, content)
			content = einoToolResultBody(content)

			toolCallID := strings.TrimSpace(msg.ToolCallID)
			tryEmitToolResultProgress(toolName, content, toolCallID, isErr, ev.AgentName)
		}
		confirmTransientRetryRecovery()
	}

	mcpIDsMu.Lock()
	ids := append([]string(nil), *mcpIDs...)
	mcpIDsMu.Unlock()

	out := buildEinoRunResultFromAccumulated(
		orchMode, runAccumulatedMsgs, modelFacingTraceSnapshot(args),
		lastAssistant, lastPlanExecuteExecutor, emptyHint, ids, false,
	)
	return out, nil
}

// modelFacingTraceSnapshot returns only the state that actually reached the model boundary.
// Never fall back to event-stream accumulation here: it can contain pre-reduction tool output
// that the model never received (for example when summarization failed before the first call).
func modelFacingTraceSnapshot(args *einoADKRunLoopArgs) []adk.Message {
	if args != nil && args.ModelFacingTrace != nil {
		if snap := args.ModelFacingTrace.Snapshot(); len(snap) > 0 {
			return snap
		}
	}
	return nil
}

func einoPartialRunLastOutputHint() string {
	return "[执行未正常结束（用户停止、超时或异常）。续跑时请基于上文已产生的工具与结果继续，勿重复已完成步骤。]\n" +
		"[Run ended abnormally; continue from the trace above without repeating completed steps.]"
}

// friendlyEinoExecuteInvokeTail 将 Eino execute 超时/中断/流异常转为简短提示。
// 命令非零退出（ExecuteExitError）已有 exec 对齐的正文，不再追加「执行未正常结束」。
func friendlyEinoExecuteInvokeTail(invokeErr error) string {
	if invokeErr == nil {
		return ""
	}
	var exitErr *ExecuteExitError
	if errors.As(invokeErr, &exitErr) {
		return ""
	}
	if errors.Is(invokeErr, context.DeadlineExceeded) {
		return einoExecuteTimeoutUserHint()
	}
	if errors.Is(invokeErr, context.Canceled) {
		return ""
	}
	if strings.Contains(invokeErr.Error(), "shell inactivity timeout") {
		return ""
	}
	return "[执行未正常结束] " + invokeErr.Error()
}

// einoToolResultIsError 统一判断 Eino 工具结果是否应标记为错误（与 MCP exec 的 IsError 对齐）。
func einoToolResultIsError(toolName, content string) bool {
	if strings.HasPrefix(content, einomcp.ToolErrorPrefix) {
		return true
	}
	if strings.TrimSpace(toolName) == "execute" && security.IsCommandFailureResult(content) {
		return true
	}
	return false
}

// einoToolResultBody 去掉工具错误前缀，返回展示/持久化正文。
func einoToolResultBody(content string) string {
	if strings.HasPrefix(content, einomcp.ToolErrorPrefix) {
		return strings.TrimPrefix(content, einomcp.ToolErrorPrefix)
	}
	return content
}

// nextAgentEventWithContext 在 ctx 取消时不再无限阻塞于 iter.Next()（工具执行/模型推理期间常见）。
func nextAgentEventWithContext(ctx context.Context, iter *adk.AsyncIterator[*adk.AgentEvent]) (ev *adk.AgentEvent, ok bool, ctxErr error) {
	if iter == nil {
		return nil, false, nil
	}
	type nextRes struct {
		ev *adk.AgentEvent
		ok bool
	}
	ch := make(chan nextRes, 1)
	go func() {
		e, o := iter.Next()
		ch <- nextRes{e, o}
	}()
	select {
	case <-ctx.Done():
		return nil, false, ctx.Err()
	case res := <-ch:
		return res.ev, res.ok, nil
	}
}

// recvSchemaMessageStream 消费 ADK Tool 流式结果；ctx 取消时立即返回，避免 amass 等无输出时永久阻塞。
func recvSchemaMessageStream(ctx context.Context, stream *schema.StreamReader[*schema.Message]) (content, toolCallID string, recvErr error) {
	if stream == nil {
		return "", "", nil
	}
	type streamMsg struct {
		chunk *schema.Message
		err   error
	}
	recvCh := make(chan streamMsg, 8)
	go func() {
		defer close(recvCh)
		for {
			ch, rerr := stream.Recv()
			recvCh <- streamMsg{chunk: ch, err: rerr}
			if rerr != nil {
				return
			}
		}
	}()
	var buf strings.Builder
	for {
		select {
		case <-ctx.Done():
			return buf.String(), toolCallID, ctx.Err()
		case sm, open := <-recvCh:
			if !open {
				return buf.String(), toolCallID, nil
			}
			rerr := sm.err
			if errors.Is(rerr, io.EOF) {
				return buf.String(), toolCallID, nil
			}
			if rerr != nil {
				return buf.String(), toolCallID, rerr
			}
			chunk := sm.chunk
			if chunk == nil {
				continue
			}
			if chunk.Content != "" {
				buf.WriteString(chunk.Content)
			}
			if tid := strings.TrimSpace(chunk.ToolCallID); tid != "" {
				toolCallID = tid
			}
		}
	}
}

func buildEinoRunResultFromAccumulated(
	orchMode string,
	runAccumulatedMsgs []adk.Message,
	persistMsgs []adk.Message,
	lastAssistant string,
	lastPlanExecuteExecutor string,
	emptyHint string,
	mcpIDs []string,
	partial bool,
) *RunResult {
	traceForJSON := persistMsgs
	traceJSON := ""
	if len(traceForJSON) > 0 {
		traceForJSON = markModelFacingTraceForPersistence(traceForJSON)
		if histJSON, err := json.Marshal(traceForJSON); err == nil {
			traceJSON = string(histJSON)
		}
	}
	cleaned := strings.TrimSpace(lastAssistant)
	if orchMode == "plan_execute" {
		if e := strings.TrimSpace(lastPlanExecuteExecutor); e != "" {
			cleaned = e
		} else {
			cleaned = UnwrapPlanExecuteUserText(cleaned)
		}
	}
	if cleaned == "" {
		if fb := strings.TrimSpace(einoExtractFallbackAssistantFromMsgs(runAccumulatedMsgs)); fb != "" {
			cleaned = fb
		}
	}
	cleaned = dedupeRepeatedParagraphs(cleaned, 80)
	cleaned = dedupeParagraphsByLineFingerprint(cleaned, 100)
	// 防止超长响应导致 JSON 序列化慢或 OOM（多代理拼接大量工具输出时可能触发）。
	const maxResponseRunes = 100000
	if rs := []rune(cleaned); len(rs) > maxResponseRunes {
		cleaned = string(rs[:maxResponseRunes]) + "\n\n... (response truncated / 响应已截断)"
	}
	lastOut := cleaned
	resp := cleaned
	if partial && cleaned == "" {
		lastOut = einoPartialRunLastOutputHint()
		resp = emptyHint
	}
	out := &RunResult{
		Response:             resp,
		MCPExecutionIDs:      mcpIDs,
		LastAgentTraceInput:  traceJSON,
		LastAgentTraceOutput: lastOut,
	}
	if !partial && out.Response == "" {
		out.Response = emptyHint
		out.LastAgentTraceOutput = out.Response
	}
	return out
}

func markModelFacingTraceForPersistence(msgs []adk.Message) []adk.Message {
	out := cloneADKMessagesForTrace(msgs)
	if len(out) == 0 || out[0] == nil {
		return out
	}
	if out[0].Extra == nil {
		out[0].Extra = make(map[string]any, 1)
	}
	out[0].Extra[agent.ModelFacingTraceVersionKey] = 1
	return out
}

// einoExtractFallbackAssistantFromMsgs 在「主通道未产出助手正文」时，从 Eino ADK 轨迹中回填用户可见回复。
// 典型场景：监督者仅调用 exit（final_result 落在 Tool 消息中），或工具结果已写入历史但 lastAssistant 未更新。
//
// 优先级：最后一次 exit 工具输出 → 最后一条含 exit 的助手 tool_calls 参数中的 final_result。
func einoExtractFallbackAssistantFromMsgs(msgs []adk.Message) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		if m == nil || m.Role != schema.Tool {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(m.ToolName), adk.ToolInfoExit.Name) {
			continue
		}
		content := strings.TrimSpace(m.Content)
		if content == "" || strings.HasPrefix(content, einomcp.ToolErrorPrefix) {
			continue
		}
		return content
	}
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		if m == nil || m.Role != schema.Assistant {
			continue
		}
		if s := einoExtractExitFinalFromAssistantToolCalls(m); s != "" {
			return s
		}
	}
	return ""
}

func einoExtractExitFinalFromAssistantToolCalls(msg *schema.Message) string {
	if msg == nil || len(msg.ToolCalls) == 0 {
		return ""
	}
	for i := len(msg.ToolCalls) - 1; i >= 0; i-- {
		tc := msg.ToolCalls[i]
		if !strings.EqualFold(strings.TrimSpace(tc.Function.Name), adk.ToolInfoExit.Name) {
			continue
		}
		if s := einoParseExitFinalResultArguments(tc.Function.Arguments); s != "" {
			return s
		}
	}
	return ""
}

func einoParseExitFinalResultArguments(arguments string) string {
	arguments = strings.TrimSpace(arguments)
	if arguments == "" {
		return ""
	}
	var wrap struct {
		FinalResult json.RawMessage `json:"final_result"`
	}
	if err := json.Unmarshal([]byte(arguments), &wrap); err != nil || len(wrap.FinalResult) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(wrap.FinalResult, &s); err == nil {
		return strings.TrimSpace(s)
	}
	var anyVal interface{}
	if err := json.Unmarshal(wrap.FinalResult, &anyVal); err != nil {
		return ""
	}
	b, err := json.Marshal(anyVal)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func buildEinoCheckpointID(orchMode string) string {
	mode := sanitizeEinoPathSegment(strings.TrimSpace(orchMode))
	if mode == "" {
		mode = "default"
	}
	return "runner-" + mode
}
