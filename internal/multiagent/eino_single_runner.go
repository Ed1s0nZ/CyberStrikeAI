package multiagent

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"cyberstrike-ai/internal/agent"
	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/einomcp"
	"cyberstrike-ai/internal/openai"
	"cyberstrike-ai/internal/project"
	"cyberstrike-ai/internal/reasoning"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/compose"
	"go.uber.org/zap"
)

// einoSingleAgentName 与 ChatModelAgent.Name 一致，供流式事件映射主对话区。
const einoSingleAgentName = "cyberstrike-eino-single"

// RunEinoSingleChatModelAgent 使用 Eino adk.NewChatModelAgent + adk.NewRunner.Run（官方 Quick Start 的 Query 同属 Runner API；此处用历史 + 用户消息切片等价于多轮 Query）。
// 与 RunDeepAgent 共享 runEinoADKAgentLoop 的 SSE 映射与 MCP 桥。
func RunEinoSingleChatModelAgent(
	ctx context.Context,
	appCfg *config.Config,
	ma *config.MultiAgentConfig,
	ag *agent.Agent,
	db *database.DB,
	logger *zap.Logger,
	conversationID string,
	projectID string,
	userMessage string,
	history []agent.ChatMessage,
	roleTools []string,
	progress func(eventType, message string, data interface{}),
	reasoningClient *reasoning.ClientIntent,
	systemPromptExtra string,
) (*RunResult, error) {
	if appCfg == nil || ag == nil {
		return nil, fmt.Errorf("eino single: 配置或 Agent 为空")
	}
	if ma == nil {
		return nil, fmt.Errorf("eino single: multi_agent 配置为空")
	}

	einoLoc, einoSkillMW, einoFSTools, skillsRoot, einoErr := prepareEinoSkills(ctx, appCfg.SkillsDir, ma, logger)
	if einoErr != nil {
		return nil, einoErr
	}

	holder := &einomcp.ConversationHolder{}
	holder.Set(conversationID)

	var mcpIDsMu sync.Mutex
	var mcpIDs []string
	mcpExecBinder := NewMCPExecutionBinder()
	recorder := func(id, toolCallID string) {
		if id == "" {
			return
		}
		mcpExecBinder.Bind(toolCallID, id)
		mcpIDsMu.Lock()
		mcpIDs = append(mcpIDs, id)
		mcpIDsMu.Unlock()
	}

	snapshotMCPIDs := func() []string {
		mcpIDsMu.Lock()
		defer mcpIDsMu.Unlock()
		out := make([]string, len(mcpIDs))
		copy(out, mcpIDs)
		return out
	}

	toolInvokeNotify := einomcp.NewToolInvokeNotifyHolder()
	einoExecMonitor := newEinoExecuteMonitorCallback(ag, recorder)
	mainDefs := ag.ToolsForRole(roleTools)
	mainTools, err := einomcp.ToolsFromDefinitions(ag, holder, mainDefs, recorder, nil, toolInvokeNotify, einoSingleAgentName)
	if err != nil {
		return nil, err
	}

	mainToolsForCfg, mainOrchestratorPre, singleToolSearchActive, err := prependEinoMiddlewares(ctx, &ma.EinoMiddleware, einoMWMain, mainTools, einoLoc, skillsRoot, conversationID, projectID, logger)
	if err != nil {
		return nil, fmt.Errorf("eino single eino 中间件: %w", err)
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
	httpClient = openai.NewEinoHTTPClient(&appCfg.OpenAI, httpClient)
	openai.AttachSummarizationDiagTransport(httpClient, logger)

	baseModelCfg := &einoopenai.ChatModelConfig{
		APIKey:     appCfg.OpenAI.APIKey,
		BaseURL:    strings.TrimSuffix(appCfg.OpenAI.BaseURL, "/"),
		Model:      appCfg.OpenAI.Model,
		HTTPClient: httpClient,
	}
	reasoning.ApplyToEinoChatModelConfig(baseModelCfg, &appCfg.OpenAI, reasoningClient)

	mainModel, err := einoopenai.NewChatModel(ctx, baseModelCfg)
	if err != nil {
		return nil, fmt.Errorf("eino single 模型: %w", err)
	}

	mainSumMw, err := newEinoSummarizationMiddleware(ctx, mainModel, appCfg, &ma.EinoMiddleware, conversationID, db, projectID, logger)
	if err != nil {
		return nil, fmt.Errorf("eino single summarization: %w", err)
	}

	modelFacingTrace := newModelFacingTraceHolder()

	handlers := make([]adk.ChatModelAgentMiddleware, 0, 8)
	if len(mainOrchestratorPre) > 0 {
		handlers = append(handlers, mainOrchestratorPre...)
	}
	if einoSkillMW != nil {
		if einoFSTools && einoLoc != nil {
			fsMw, fsErr := subAgentFilesystemMiddleware(ctx, einoLoc, toolInvokeNotify, einoSingleAgentName, einoExecMonitor, agentToolTimeoutMinutes(appCfg), nil)
			if fsErr != nil {
				return nil, fmt.Errorf("eino single filesystem 中间件: %w", fsErr)
			}
			handlers = append(handlers, fsMw)
		}
		handlers = append(handlers, einoSkillMW)
	}
	handlers = append(handlers, mainSumMw)
	if teleMw := newEinoModelInputTelemetryMiddleware(logger, appCfg.OpenAI.Model, conversationID, "eino_single"); teleMw != nil {
		handlers = append(handlers, teleMw)
	}
	if capMw := newModelFacingTraceMiddleware(modelFacingTrace); capMw != nil {
		handlers = append(handlers, capMw)
	}

	maxIter := agentMaxIterations(appCfg)

	mainToolsCfg := adk.ToolsConfig{
		ToolsNodeConfig: compose.ToolsNodeConfig{
			Tools:               mainToolsForCfg,
			UnknownToolsHandler: einomcp.UnknownToolReminderHandler(),
			ToolCallMiddlewares: []compose.ToolMiddleware{
				hitlToolCallMiddleware(),
				softRecoveryToolMiddleware(),
			},
		},
		EmitInternalEvents: true,
	}
	ins := project.AppendSystemPromptBlock(ag.EinoSingleAgentSystemInstruction(), systemPromptExtra)
	ins = project.AppendVisionImageAnalysisIfReady(ins, appCfg.Vision.Ready())
	ins = injectToolNamesOnlyInstruction(ctx, ins, mainTools, singleToolSearchActive)
	if logger != nil {
		names := collectToolNames(ctx, mainTools)
		mountedNames := collectToolNames(ctx, mainToolsForCfg)
		logger.Info("eino tool-name injection",
			zap.String("scope", "eino_single"),
			zap.Int("tool_names", len(names)),
			zap.Int("mounted_tool_names", len(mountedNames)),
			zap.Bool("tool_search_middleware", singleToolSearchActive),
		)
	}

	chatCfg := &adk.ChatModelAgentConfig{
		Name:          einoSingleAgentName,
		Description:   "Eino ADK ChatModelAgent with MCP tools for authorized security testing.",
		Instruction:   ins,
		Model:         mainModel,
		ToolsConfig:   mainToolsCfg,
		MaxIterations: maxIter,
		Handlers:      handlers,
	}
	outKey, modelRetry, _ := deepExtrasFromConfig(ma)
	if outKey != "" {
		chatCfg.OutputKey = outKey
	}
	if modelRetry != nil {
		chatCfg.ModelRetryConfig = modelRetry
	}

	chatAgent, err := adk.NewChatModelAgent(ctx, chatCfg)
	if err != nil {
		return nil, fmt.Errorf("eino single NewChatModelAgent: %w", err)
	}

	baseMsgs := historyToMessages(history, appCfg, &ma.EinoMiddleware)
	baseMsgs = appendUserMessageIfNeeded(baseMsgs, userMessage)

	streamsMainAssistant := func(agent string) bool {
		return agent == "" || agent == einoSingleAgentName
	}
	einoRoleTag := func(agent string) string {
		_ = agent
		return "orchestrator"
	}

	return runEinoADKAgentLoop(ctx, &einoADKRunLoopArgs{
		OrchMode:                "eino_single",
		OrchestratorName:        einoSingleAgentName,
		ConversationID:          conversationID,
		Progress:                progress,
		Logger:                  logger,
		SnapshotMCPIDs:          snapshotMCPIDs,
		StreamsMainAssistant:    streamsMainAssistant,
		EinoRoleTag:             einoRoleTag,
		CheckpointDir:           ma.EinoMiddleware.CheckpointDir,
		RunRetryMaxAttempts:     ma.EinoMiddleware.RunRetryMaxAttempts,
		RunRetryMaxBackoffSec:   ma.EinoMiddleware.RunRetryMaxBackoffSec,
		McpIDsMu:                &mcpIDsMu,
		McpIDs:                  &mcpIDs,
		FilesystemMonitorAgent:  ag,
		FilesystemMonitorRecord: recorder,
		MCPExecutionBinder:      mcpExecBinder,
		ToolInvokeNotify:        toolInvokeNotify,
		DA:                      chatAgent,
		ModelFacingTrace:        modelFacingTrace,
		EinoCallbacks:           &ma.EinoCallbacks,
		EmptyResponseMessage: "(Eino ADK single-agent session completed but no assistant text was captured. Check process details or logs.) " +
			"（Eino ADK 单代理会话已完成，但未捕获到助手文本输出。请查看过程详情或日志。）",
	}, baseMsgs)
}
