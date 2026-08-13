package multiagent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/planexecute"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"

	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/database"
)

// cairnPlan 是 Cairn 模式的 Plan 实现：intents 列表（Fact-Intent-Hint 图的探索方向）。
type cairnPlan struct {
	Intents []CairnIntentPayload `json:"intents"`
	// CommittedToDB 标记该 plan 的 intents 已通过 CommitCairnReasonDecision 落库（canonical 路径防重复提交）。
	CommittedToDB bool `json:"-"`
}

func newCairnPlan(context.Context) planexecute.Plan {
	return &cairnPlan{}
}

func (p *cairnPlan) FirstStep() string {
	if p == nil || len(p.Intents) == 0 {
		return ""
	}
	return p.Intents[0].Description
}

func (p *cairnPlan) MarshalJSON() ([]byte, error) {
	type alias cairnPlan
	return json.Marshal((*alias)(p))
}

func (p *cairnPlan) UnmarshalJSON(b []byte) error {
	// planexecute 框架的 plan 工具输出 {"steps": [...]}，解析为 intents
	type stepsPlan struct {
		Steps []string `json:"steps"`
	}
	var sp stepsPlan
	if err := json.Unmarshal(b, &sp); err == nil && len(sp.Steps) > 0 {
		intents := make([]CairnIntentPayload, len(sp.Steps))
		for i, s := range sp.Steps {
			intents[i] = CairnIntentPayload{Description: s}
		}
		p.Intents = intents
		return nil
	}
	// 宽松解析：从 Reason 输出提取 intents（Cairn JSON 协议）
	parsed, err := ParseCairnOutput(string(b))
	if err != nil {
		return err
	}
	kind, data, err := ValidateReasonPayload(parsed, true, 0)
	if err != nil {
		return err
	}
	if kind == "intents" {
		intents, ok := data.([]CairnIntentPayload)
		if !ok {
			return fmt.Errorf("unexpected intents type %T", data)
		}
		p.Intents = intents
		return nil
	}
	return fmt.Errorf("reason output is not intents: %s", kind)
}

const defaultCairnExplorerMaxIterations = 12

// cairnExplorerMaxIterations 将全局 Agent 轮次限制在单个 intent 的预算内。
// Cairn Explorer 必须结束并把事实交给 Replanner，不能继承主 Agent 的超大预算。
func cairnExplorerMaxIterations(configured int) int {
	if configured <= 0 || configured > defaultCairnExplorerMaxIterations {
		return defaultCairnExplorerMaxIterations
	}
	return configured
}

const cairnExplorerResultSessionKey = "CairnExplorerResult"

func cairnExplorerResultFromSession(ctx context.Context) string {
	if v, ok := adk.GetSessionValue(ctx, cairnExplorerResultSessionKey); ok {
		if s, ok := v.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func cairnExplorerEventID(step, result string) string {
	return stableCairnID("explore", step, result)
}

type cairnPlannerBridge struct {
	inner     adk.Agent
	statePath string
	logger    *zap.Logger
}

func (b *cairnPlannerBridge) Name(ctx context.Context) string        { return b.inner.Name(ctx) }
func (b *cairnPlannerBridge) Description(ctx context.Context) string { return b.inner.Description(ctx) }

func (b *cairnPlannerBridge) Run(ctx context.Context, input *adk.AgentInput, opts ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	out, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	innerIter := b.inner.Run(ctx, input, opts...)
	go func() {
		defer gen.Close()
		for {
			ev, ok := innerIter.Next()
			if !ok {
				break
			}
			if ev != nil {
				gen.Send(ev)
			}
		}
		if raw, ok := adk.GetSessionValue(ctx, planexecute.PlanSessionKey); ok {
			if plan, ok := raw.(planexecute.Plan); ok && plan != nil {
				if cp, ok := plan.(*cairnPlan); ok {
					if _, err := UpdateCairnState(b.statePath, func(st *CairnState) error {
						for _, intent := range cp.Intents {
							st.AddIntent(intent.From, intent.Description)
						}
						return nil
					}); err != nil && b.logger != nil {
						b.logger.Error("cairn planner state persist failed", zap.Error(err))
					}
				}
			}
		}
	}()
	return out
}

// cairnExecutorBridge 将 Explorer 变成“结果事件”节点。
// Eino planexecute 遇到子 Agent Err 会直接中止 loop；这里把 Explorer 的 Err/Exit
// 归一化为普通 ExecutedStep，让 Replanner 必定接管下一步。
type cairnExecutorBridge struct {
	inner          adk.Agent
	statePath      string
	logger         *zap.Logger
	db             *database.DB
	projectID      string
	conversationID string
}

func (b *cairnExecutorBridge) Name(ctx context.Context) string { return b.inner.Name(ctx) }
func (b *cairnExecutorBridge) Description(ctx context.Context) string {
	return b.inner.Description(ctx)
}

func (b *cairnExecutorBridge) Run(ctx context.Context, input *adk.AgentInput, opts ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	out, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	innerIter := b.inner.Run(ctx, input, opts...)
	go func() {
		defer gen.Close()
		var lastContent string
		var runErr error
		for {
			ev, ok := innerIter.Next()
			if !ok {
				break
			}
			if ev == nil {
				continue
			}
			if ev.Err != nil {
				runErr = ev.Err
				continue
			}
			if ev.Output != nil && ev.Output.MessageOutput != nil && !ev.Output.MessageOutput.IsStreaming {
				if msg, err := ev.Output.MessageOutput.GetMessage(); err == nil && msg != nil && strings.TrimSpace(msg.Content) != "" {
					lastContent = strings.TrimSpace(msg.Content)
				}
			}
			// Explorer 的 exit 只代表“本 intent 结束”，不能终止外层 Cairn loop。
			if ev.Action != nil && ev.Action.Exit {
				continue
			}
			gen.Send(ev)
		}

		step := ""
		if raw, ok := adk.GetSessionValue(ctx, planexecute.PlanSessionKey); ok {
			if plan, ok := raw.(planexecute.Plan); ok && plan != nil {
				step = strings.TrimSpace(plan.FirstStep())
			}
		}
		result := cairnExplorerResultFromSession(ctx)
		if result == "" {
			result = lastContent
		}
		if result == "" && runErr != nil {
			result = "[explore failed] " + strings.TrimSpace(runErr.Error())
		}
		if result == "" {
			result = "[explore completed without a fact]"
		}
		// planexecute.Replanner 读取该 session key；Err 路径不会由 ChatModelAgent 自动写入 OutputKey，必须强制补齐。
		adk.AddSessionValue(ctx, planexecute.ExecutedStepSessionKey, result)
		if err := persistCairnExecutionResult(b.statePath, step, result, runErr); err != nil && b.logger != nil {
			b.logger.Error("cairn explorer result persist failed", zap.Error(err), zap.String("step", step))
		}
		syncCairnFactToProject(b.db, b.projectID, b.conversationID, step, result)
		// planexecute loop 只需要看到一个已完成的 Executor 输出；结果已写入 session/state。
		gen.Send(adk.EventFromMessage(schema.AssistantMessage(result, nil), nil, schema.Assistant, ""))
	}()
	return out
}

func persistCairnExecutionResult(statePath, step, result string, runErr error) error {
	step = strings.TrimSpace(step)
	result = strings.TrimSpace(result)
	if result == "" && runErr != nil {
		result = "[explore failed] " + strings.TrimSpace(runErr.Error())
	}
	if result == "" {
		return nil
	}
	eventID := cairnExplorerEventID(step, result)
	_, err := UpdateCairnState(statePath, func(st *CairnState) error {
		fact, added := st.AddFactIdempotent(eventID, result, step)
		st.MarkIntentDoneByDescription(step)
		if added {
			st.Origin = "cairn"
		}
		_ = fact
		return nil
	})
	return err
}

func syncCairnFactToProject(db *database.DB, projectID, conversationID, step, result string) {
	if db == nil || strings.TrimSpace(projectID) == "" || strings.TrimSpace(result) == "" {
		return
	}
	key := stableCairnID("cairn", conversationID, step, result)
	f := &database.ProjectFact{
		ProjectID:            projectID,
		FactKey:              key,
		Category:             "cairn_fact",
		Summary:              strings.TrimSpace(result),
		Body:                 fmt.Sprintf("source_intent: %s\\nconversation_id: %s", strings.TrimSpace(step), strings.TrimSpace(conversationID)),
		Confidence:           "tentative",
		SourceConversationID: conversationID,
		Pinned:               false,
	}
	if _, err := db.UpsertProjectFact(f); err != nil {
		// 黑板同步失败不能阻断 Cairn 主链；主状态已落在 YAML。
		return
	}
}

// CairnRootArgs 构建 Cairn 模式根 Agent 所需参数。
type CairnRootArgs struct {
	// MainToolCallingModel 使用 ToolCallingChatModel 接口而非具体类型：
	// 上游对 mainModel 包装过 stream-tool-call-index-repair 后是接口类型，具体类型会编译失败。
	MainToolCallingModel model.ToolCallingChatModel
	ExecModel            *einoopenai.ChatModel
	OrchInstruction      string // Reason 提示词（orchestrator-cairn.md）
	ExecInstruction      string // Explore 提示词（executor-cairn.md）
	ToolsCfg             adk.ToolsConfig
	ExecMaxIter          int
	StateDir             string // Fact-Intent-Hint 图存储目录
	MaxIntents           int    // Reason 单轮最大 intent 数（默认 5）
	MaxParallelExplorers int    // 并行 Explorer 上限（默认 8，当前未实现并行，预留）
	LoopMaxIter          int    // Reason→Explore 外层循环上限（默认 10）
	AppCfg               *config.Config
	MwCfg                *config.MultiAgentEinoMiddlewareConfig
	ConversationID       string
	ProjectID            string
	DB                   *database.DB
	Logger               *zap.Logger
	ModelName            string
	// GoalText 是 canonical run 的目标文本（当前用户消息）；为空时 canonical 不可用，回退 legacy。
	GoalText string
	// InputHash 是 canonical run 创建的幂等键输入（同一消息重放复用同一 run）。
	InputHash string
	// 中间件（与 plan_execute 一致）
	ExecPreMiddlewares              []adk.ChatModelAgentMiddleware
	SkillMiddleware                 adk.ChatModelAgentMiddleware
	FilesystemMiddleware            adk.ChatModelAgentMiddleware
	PlannerReplannerRewriteHandlers []adk.ChatModelAgentMiddleware
	ModelFacingTrace                *modelFacingTraceHolder
}

// NewCairnRoot 返回 Cairn 模式编排根节点（复用 planexecute 框架：Reason → Explore → Reason → ...）。
// cairn_sqlite_canonical_enabled=true 且具备项目绑定时走 canonical 路径，否则保持 legacy YAML 路径。
func NewCairnRoot(ctx context.Context, a *CairnRootArgs) (adk.ResumableAgent, error) {
	if a == nil {
		return nil, fmt.Errorf("cairn: args 为空")
	}
	if a.MainToolCallingModel == nil || a.ExecModel == nil {
		return nil, fmt.Errorf("cairn: 模型为空")
	}

	if a.AppCfg != nil && a.AppCfg.MultiAgent.CairnSQLiteCanonicalEnabled &&
		a.DB != nil && strings.TrimSpace(a.ProjectID) != "" &&
		strings.TrimSpace(a.ConversationID) != "" && strings.TrimSpace(a.GoalText) != "" {
		return newCairnCanonicalRoot(ctx, a)
	}

	statePath := StateFilePath(a.StateDir, a.ProjectID, a.ConversationID)

	// Reason Agent（Planner）：读图 → 判断 goal / 开 intent
	reasonCfg := &planexecute.PlannerConfig{
		ToolCallingChatModel: a.MainToolCallingModel,
		NewPlan:              newCairnPlan,
	}
	if fn := cairnReasonPlannerGenInput(a.OrchInstruction, statePath, a.MaxIntents, a.AppCfg, a.MwCfg, a.Logger, a.ModelName, a.ConversationID, a.PlannerReplannerRewriteHandlers); fn != nil {
		reasonCfg.GenInputFn = fn
	}
	reasonPlanner, err := planexecute.NewPlanner(ctx, reasonCfg)
	if err != nil {
		return nil, fmt.Errorf("cairn reason planner: %w", err)
	}
	reasonPlanner = &cairnPlannerBridge{inner: reasonPlanner, statePath: statePath, logger: a.Logger}

	// Reason Replanner：再读图 → 更新 intents
	reasonReplanner, err := planexecute.NewReplanner(ctx, &planexecute.ReplannerConfig{
		ChatModel:  a.MainToolCallingModel,
		GenInputFn: cairnReasonReplannerGenInput(a.OrchInstruction, statePath, a.MaxIntents, a.AppCfg, a.MwCfg, a.Logger, a.ModelName, a.ConversationID, a.PlannerReplannerRewriteHandlers),
		NewPlan:    newCairnPlan,
	})
	if err != nil {
		return nil, fmt.Errorf("cairn reason replanner: %w", err)
	}

	// Explore Agent（Executor）：执行 intent → 提交 fact
	execHandlers, err := buildCairnExploreHandlers(ctx, a)
	if err != nil {
		return nil, err
	}
	executor, err := newCairnExploreExecutor(ctx, &planexecute.ExecutorConfig{
		Model:         a.ExecModel,
		ToolsConfig:   a.ToolsCfg,
		MaxIterations: cairnExplorerMaxIterations(a.ExecMaxIter),
		GenInputFn:    cairnExploreExecutorGenInput(a.ExecInstruction, statePath, a.AppCfg, a.MwCfg, a.Logger, a.ModelName, a.ConversationID),
	}, execHandlers)
	if err != nil {
		return nil, fmt.Errorf("cairn explore executor: %w", err)
	}
	executor = &cairnExecutorBridge{inner: executor, statePath: statePath, logger: a.Logger, db: a.DB, projectID: a.ProjectID, conversationID: a.ConversationID}

	loopMax := a.LoopMaxIter
	if loopMax <= 0 {
		loopMax = 10
	}
	return planexecute.New(ctx, &planexecute.Config{
		Planner:       reasonPlanner,
		Executor:      executor,
		Replanner:     reasonReplanner,
		MaxIterations: loopMax,
	})
}

// cairnReasonPlannerGenInput 生成 Reason Planner 的模型输入。
func cairnReasonPlannerGenInput(
	orchInstruction string,
	statePath string,
	maxIntents int,
	appCfg *config.Config,
	mwCfg *config.MultiAgentEinoMiddlewareConfig,
	logger *zap.Logger,
	modelName string,
	conversationID string,
	rewriteHandlers []adk.ChatModelAgentMiddleware,
) planexecute.GenPlannerModelInputFn {
	oi := strings.TrimSpace(orchInstruction)
	return func(ctx context.Context, userInput []adk.Message) ([]adk.Message, error) {
		// 加载当前状态图
		st, err := LoadCairnState(statePath)
		if err != nil {
			return nil, fmt.Errorf("cairn reason load state: %w", err)
		}
		graphYAML, err := st.ToYAML()
		if err != nil {
			return nil, fmt.Errorf("cairn reason marshal state: %w", err)
		}

		// 构建 Reason 提示词（参考 Cairn reason.md）
		var sb strings.Builder
		if oi != "" {
			sb.WriteString(oi)
			sb.WriteString("\n\n")
		}
		sb.WriteString("## Graph\n```yaml\n")
		sb.WriteString(graphYAML)
		sb.WriteString("```\n\n")

		sb.WriteString("## Valid facts\n```json\n")
		factIDsJSON, _ := json.Marshal(st.FactIDs())
		sb.Write(factIDsJSON)
		sb.WriteString("\n```\n\n")

		openIntents := st.OpenIntents()
		sb.WriteString("## Open Intents\n```json\n")
		openJSON, _ := json.Marshal(openIntents)
		sb.Write(openJSON)
		sb.WriteString("\n```\n\n")

		if maxIntents > 0 {
			sb.WriteString(fmt.Sprintf("## Max intents per round\n%d\n", maxIntents))
		}

		msgs := []adk.Message{
			{Role: "system", Content: sb.String()},
		}
		msgs = append(msgs, userInput...)
		if rewritten, rerr := applyBeforeModelRewriteHandlers(ctx, msgs, rewriteHandlers); rerr == nil && len(rewritten) > 0 {
			msgs = rewritten
		}
		logPlanExecuteModelInputEstimate(logger, modelName, conversationID, "cairn_reason_planner", msgs)
		return msgs, nil
	}
}

// cairnReasonReplannerGenInput 生成 Reason Replanner 的模型输入。
// 关键：解析 Executor 输出（ExecutedSteps）中的文本事实，写入 state 文件，然后生成提示词。
func cairnReasonReplannerGenInput(
	orchInstruction string,
	statePath string,
	maxIntents int,
	appCfg *config.Config,
	mwCfg *config.MultiAgentEinoMiddlewareConfig,
	logger *zap.Logger,
	modelName string,
	conversationID string,
	rewriteHandlers []adk.ChatModelAgentMiddleware,
) planexecute.GenModelInputFn {
	oi := strings.TrimSpace(orchInstruction)
	return func(ctx context.Context, in *planexecute.ExecutionContext) ([]adk.Message, error) {
		// 1. 解析 Executor 输出，写入 state 文件（submit_fact）
		if in.ExecutedSteps != nil {
			for _, step := range in.ExecutedSteps {
				if step.Result == "" {
					continue
				}
				// Explore 输出纯文本事实描述，直接作为 fact description
				factDesc := strings.TrimSpace(step.Result)
				if factDesc == "" {
					continue
				}
				if err := persistCairnExecutionResult(statePath, step.Step, factDesc, nil); err != nil {
					return nil, fmt.Errorf("cairn replanner submit fact: %w", err)
				}
				if logger != nil {
					logger.Info("cairn replanner: fact submitted",
						zap.String("step", step.Step),
						zap.String("fact", factDesc[:min(80, len(factDesc))]),
					)
				}
			}
		}

		// 2. 加载更新后的状态图（已包含新提交的 facts）
		st, err := LoadCairnState(statePath)
		if err != nil {
			return nil, fmt.Errorf("cairn replanner load state: %w", err)
		}
		graphYAML, err := st.ToYAML()
		if err != nil {
			return nil, fmt.Errorf("cairn replanner marshal state: %w", err)
		}

		planContent, err := in.Plan.MarshalJSON()
		if err != nil {
			return nil, err
		}

		var sb strings.Builder
		if oi != "" {
			sb.WriteString(oi)
			sb.WriteString("\n\n")
		}
		sb.WriteString("## Graph\n```yaml\n")
		sb.WriteString(graphYAML)
		sb.WriteString("```\n\n")

		sb.WriteString("## Current Plan (intents)\n```json\n")
		sb.Write(planContent)
		sb.WriteString("\n```\n\n")

		sb.WriteString("## Valid facts\n```json\n")
		factIDsJSON, _ := json.Marshal(st.FactIDs())
		sb.Write(factIDsJSON)
		sb.WriteString("\n```\n\n")

		openIntents := st.OpenIntents()
		sb.WriteString("## Open Intents\n```json\n")
		openJSON, _ := json.Marshal(openIntents)
		sb.Write(openJSON)
		sb.WriteString("\n```\n\n")

		if maxIntents > 0 {
			sb.WriteString(fmt.Sprintf("## Max intents per round\n%d\n", maxIntents))
		}

		msgs := []adk.Message{
			{Role: "system", Content: sb.String()},
		}
		if rewritten, rerr := applyBeforeModelRewriteHandlers(ctx, msgs, rewriteHandlers); rerr == nil && len(rewritten) > 0 {
			msgs = rewritten
		}
		logPlanExecuteModelInputEstimate(logger, modelName, conversationID, "cairn_reason_replanner", msgs)
		return msgs, nil
	}
}

// cairnExploreExecutorGenInput 生成 Explore Executor 的模型输入。
func cairnExploreExecutorGenInput(
	execInstruction string,
	statePath string,
	appCfg *config.Config,
	mwCfg *config.MultiAgentEinoMiddlewareConfig,
	logger *zap.Logger,
	modelName string,
	conversationID string,
) planexecute.GenModelInputFn {
	ei := strings.TrimSpace(execInstruction)
	return func(ctx context.Context, in *planexecute.ExecutionContext) ([]adk.Message, error) {
		// 加载当前状态图
		st, err := LoadCairnState(statePath)
		if err != nil {
			return nil, fmt.Errorf("cairn explore load state: %w", err)
		}
		graphYAML, err := st.ToYAML()
		if err != nil {
			return nil, fmt.Errorf("cairn explore marshal state: %w", err)
		}

		// 当前 step 是 intent description（planexecute 的 step）
		currentStep := in.Plan.FirstStep()

		var sb strings.Builder
		if ei != "" {
			sb.WriteString(ei)
			sb.WriteString("\n\n")
		}
		sb.WriteString("## Graph\n```yaml\n")
		sb.WriteString(graphYAML)
		sb.WriteString("```\n\n")

		sb.WriteString("## Current Intent\n```\n")
		sb.WriteString(currentStep)
		sb.WriteString("\n```\n")

		msgs := []adk.Message{
			{Role: "system", Content: sb.String()},
		}
		msgs = append(msgs, in.UserInput...)
		logPlanExecuteModelInputEstimate(logger, modelName, conversationID, "cairn_explore_executor", msgs)
		return msgs, nil
	}
}

// buildCairnExploreHandlers 组装 Explore Executor 中间件栈。
func buildCairnExploreHandlers(ctx context.Context, a *CairnRootArgs) ([]adk.ChatModelAgentMiddleware, error) {
	var handlers []adk.ChatModelAgentMiddleware
	if len(a.ExecPreMiddlewares) > 0 {
		handlers = append(handlers, a.ExecPreMiddlewares...)
	}
	if a.FilesystemMiddleware != nil {
		handlers = append(handlers, a.FilesystemMiddleware)
	}
	if a.SkillMiddleware != nil {
		handlers = append(handlers, a.SkillMiddleware)
	}
	return handlers, nil
}

// newCairnExploreExecutor 构建 Cairn Explore Executor（复用 plan_execute 的 Executor 模式）。
func newCairnExploreExecutor(ctx context.Context, cfg *planexecute.ExecutorConfig, handlers []adk.ChatModelAgentMiddleware) (adk.Agent, error) {
	if cfg == nil {
		return nil, fmt.Errorf("cairn explore: ExecutorConfig 为空")
	}
	if cfg.Model == nil {
		return nil, fmt.Errorf("cairn explore: Executor Model 为空")
	}
	genInputFn := cfg.GenInputFn
	if genInputFn == nil {
		return nil, fmt.Errorf("cairn explore: GenInputFn 为空")
	}
	// planexecute.GenModelInputFn (ctx, *ExecutionContext) → adk.GenModelInput (ctx, instruction, *AgentInput)
	// 从 session 中读 Plan/UserInput/ExecutedSteps（与 newPlanExecuteExecutor 一致）
	adkGenInput := func(ctx context.Context, instruction string, _ *adk.AgentInput) ([]adk.Message, error) {
		plan, ok := adk.GetSessionValue(ctx, planexecute.PlanSessionKey)
		if !ok {
			return nil, fmt.Errorf("cairn explore executor: session value %q missing", planexecute.PlanSessionKey)
		}
		plan_, ok := plan.(planexecute.Plan)
		if !ok {
			return nil, fmt.Errorf("cairn explore executor: session value %q has invalid type %T", planexecute.PlanSessionKey, plan)
		}
		userInput, ok := adk.GetSessionValue(ctx, planexecute.UserInputSessionKey)
		if !ok {
			return nil, fmt.Errorf("cairn explore executor: session value %q missing", planexecute.UserInputSessionKey)
		}
		userInput_, ok := userInput.([]adk.Message)
		if !ok {
			return nil, fmt.Errorf("cairn explore executor: session value %q has invalid type %T", planexecute.UserInputSessionKey, userInput)
		}
		var executedSteps_ []planexecute.ExecutedStep
		executedStep, ok := adk.GetSessionValue(ctx, planexecute.ExecutedStepsSessionKey)
		if ok {
			executedSteps_, ok = executedStep.([]planexecute.ExecutedStep)
			if !ok {
				return nil, fmt.Errorf("cairn explore executor: session value %q has invalid type %T", planexecute.ExecutedStepsSessionKey, executedStep)
			}
		}
		execCtx := &planexecute.ExecutionContext{
			UserInput:     userInput_,
			Plan:          plan_,
			ExecutedSteps: executedSteps_,
		}
		return genInputFn(ctx, execCtx)
	}
	toolsConfig := cfg.ToolsConfig
	if toolsConfig.ReturnDirectly == nil {
		toolsConfig.ReturnDirectly = make(map[string]bool)
	} else {
		copied := make(map[string]bool, len(toolsConfig.ReturnDirectly)+1)
		for name, enabled := range toolsConfig.ReturnDirectly {
			copied[name] = enabled
		}
		toolsConfig.ReturnDirectly = copied
	}
	// ExitTool 触发 AgentAction.Exit，ReturnDirectly 保证模型提交事实后立即回到 Replanner。
	toolsConfig.ReturnDirectly["exit"] = true
	agentCfg := &adk.ChatModelAgentConfig{
		Name:          "cairn_explorer",
		Description:   "Cairn explore executor",
		Model:         cfg.Model,
		ToolsConfig:   toolsConfig,
		GenModelInput: adkGenInput,
		MaxIterations: cairnExplorerMaxIterations(cfg.MaxIterations),
		OutputKey:     planexecute.ExecutedStepSessionKey,
		Exit:          &adk.ExitTool{},
	}
	if len(handlers) > 0 {
		agentCfg.Handlers = handlers
	}
	return adk.NewChatModelAgent(ctx, agentCfg)
}
