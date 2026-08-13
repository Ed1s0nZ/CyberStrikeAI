package multiagent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/planexecute"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"

	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/database"
)

// Cairn canonical 运行链（SQLite canonical state cutover）。
//
// 与 legacy 路径（cairn_root.go 的 YAML 双写）的区别：
//   - Run 创建时冻结 project scope snapshot（scope_sha256 + 原始 JSON）。
//   - Planner / Replanner 统一通过 CommitCairnReasonDecision 写 Intent（DB 是唯一真相）。
//   - Executor 先 ClaimNextCairnIntent，成功 observation 走 CompleteCairnIntentWithFact，
//     error / timeout / cancel / max iterations / 空结果一律 FailCairnIntentExecution，
//     绝不创建 Fact。
//   - Replanner respond（BreakLoop）触发 CompleteCairnRun。
//   - root finalizer 兜底：run 未终态时按 context/错误分类写终态。
//   - project_facts 与 YAML 导出由 outbox 消费者异步投影，运行链不再直接双写。

const (
	cairnCanonicalRunSessionKey  = "CairnCanonicalRunID"
	cairnClaimedIntentSessionKey = "CairnClaimedIntent"
)

func cairnNowKey() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func cairnInputHash(text string) string {
	h := sha256.Sum256([]byte(text))
	return hex.EncodeToString(h[:])[:16]
}

func isTerminalCairnRunLocal(status string) bool {
	switch status {
	case database.CairnRunCompleted, database.CairnRunFailed, database.CairnRunCancelled, database.CairnRunTimedOut:
		return true
	default:
		return false
	}
}

// ensureCairnCanonicalRun 复用 active run（resume 语义），否则用 input hash 幂等创建新 run。
func ensureCairnCanonicalRun(ctx context.Context, db *database.DB, projectID, conversationID, goalText, inputHash string) (*database.CairnRun, error) {
	active, err := db.GetActiveCairnRunForConversation(ctx, conversationID)
	if err != nil {
		return nil, fmt.Errorf("cairn canonical: 查询 active run: %w", err)
	}
	if active != nil {
		return active, nil
	}
	scopeJSON := "{}"
	if project, err := db.GetProject(projectID); err == nil && strings.TrimSpace(project.ScopeJSON) != "" {
		scopeJSON = strings.TrimSpace(project.ScopeJSON)
	}
	scopeSHA := cairnInputHash(scopeJSON)
	if strings.TrimSpace(inputHash) == "" {
		inputHash = cairnInputHash(goalText)
	}
	run, err := db.CreateCairnRunWithScopeSnapshot(ctx, &database.CairnRun{
		ProjectID:         projectID,
		ConversationID:    conversationID,
		GoalText:          goalText,
		ScopeSnapshotJSON: scopeJSON,
		ScopeSHA256:       scopeSHA,
	}, "cairn_run:"+conversationID+":"+inputHash)
	if err != nil {
		return nil, fmt.Errorf("cairn canonical: 创建 run: %w", err)
	}
	return run, nil
}

// cairnCanonicalStateForPrompt 把 DB snapshot 投影为与 legacy 兼容的图结构（模型看到相同格式）。
func cairnCanonicalStateForPrompt(snapshot *database.CairnStateSnapshot) *CairnState {
	st := &CairnState{Version: 3, Origin: "cairn_canonical"}
	if snapshot.Initialized && snapshot.Run != nil {
		st.Goal = snapshot.Run.GoalText
		st.UpdatedAt = snapshot.Run.UpdatedAt
	}
	for _, f := range snapshot.Facts {
		st.Facts = append(st.Facts, Fact{
			ID:            f.ID,
			Description:   f.Statement,
			CreatedAt:     f.CreatedAt,
			SourceIntent:  f.SourceIntentID,
			SourceEventID: f.SourceEventID,
		})
	}
	for _, it := range snapshot.Intents {
		st.Intents = append(st.Intents, Intent{
			ID:          it.ID,
			Description: it.Description,
			Status:      it.Status,
		})
	}
	for _, h := range snapshot.Hints {
		st.Hints = append(st.Hints, Hint{
			ID:        h.ID,
			Content:   h.Content,
			Creator:   h.CreatorType,
			CreatedAt: h.CreatedAt,
		})
	}
	normalizeCairnState(st)
	return st
}

func loadCairnCanonicalStateForPrompt(ctx context.Context, db *database.DB, projectID, conversationID string) (*CairnState, error) {
	snapshot, err := db.GetLatestCairnStateSnapshot(ctx, projectID, conversationID)
	if err != nil {
		return nil, fmt.Errorf("cairn canonical load snapshot: %w", err)
	}
	return cairnCanonicalStateForPrompt(snapshot), nil
}

func buildCairnReasonPrompt(oi, graphYAML string, factIDs []string, openIntents []Intent, maxIntents int) string {
	var sb strings.Builder
	if oi != "" {
		sb.WriteString(oi)
		sb.WriteString("\n\n")
	}
	sb.WriteString("## Graph\n```yaml\n")
	sb.WriteString(graphYAML)
	sb.WriteString("```\n\n")

	sb.WriteString("## Valid facts\n```json\n")
	factIDsJSON, _ := json.Marshal(factIDs)
	sb.Write(factIDsJSON)
	sb.WriteString("\n```\n\n")

	sb.WriteString("## Open Intents\n```json\n")
	openJSON, _ := json.Marshal(openIntents)
	sb.Write(openJSON)
	sb.WriteString("\n```\n\n")

	if maxIntents > 0 {
		sb.WriteString(fmt.Sprintf("## Max intents per round\n%d\n", maxIntents))
	}
	return sb.String()
}

// cairnCanonicalReasonPlannerGenInput 生成 canonical Reason Planner 的模型输入。
func cairnCanonicalReasonPlannerGenInput(
	orchInstruction string,
	db *database.DB,
	projectID, conversationID string,
	maxIntents int,
	appCfg *config.Config,
	mwCfg *config.MultiAgentEinoMiddlewareConfig,
	logger *zap.Logger,
	modelName string,
	rewriteHandlers []adk.ChatModelAgentMiddleware,
) planexecute.GenPlannerModelInputFn {
	oi := strings.TrimSpace(orchInstruction)
	return func(ctx context.Context, userInput []adk.Message) ([]adk.Message, error) {
		st, err := loadCairnCanonicalStateForPrompt(ctx, db, projectID, conversationID)
		if err != nil {
			return nil, err
		}
		graphYAML, err := st.ToYAML()
		if err != nil {
			return nil, fmt.Errorf("cairn canonical reason marshal state: %w", err)
		}
		sb := buildCairnReasonPrompt(oi, graphYAML, st.FactIDs(), st.OpenIntents(), maxIntents)
		msgs := []adk.Message{
			{Role: "system", Content: sb},
		}
		msgs = append(msgs, userInput...)
		if rewritten, rerr := applyBeforeModelRewriteHandlers(ctx, msgs, rewriteHandlers); rerr == nil && len(rewritten) > 0 {
			msgs = rewritten
		}
		logPlanExecuteModelInputEstimate(logger, modelName, conversationID, "cairn_canonical_reason_planner", msgs)
		return msgs, nil
	}
}

// cairnCanonicalReasonReplannerGenInput 生成 canonical Reason Replanner 的模型输入。
// Executor 结果已由 bridge 原子落库，这里只做快照投影，不再做任何写操作。
func cairnCanonicalReasonReplannerGenInput(
	orchInstruction string,
	db *database.DB,
	projectID, conversationID string,
	maxIntents int,
	appCfg *config.Config,
	mwCfg *config.MultiAgentEinoMiddlewareConfig,
	logger *zap.Logger,
	modelName string,
	rewriteHandlers []adk.ChatModelAgentMiddleware,
) planexecute.GenModelInputFn {
	oi := strings.TrimSpace(orchInstruction)
	return func(ctx context.Context, in *planexecute.ExecutionContext) ([]adk.Message, error) {
		st, err := loadCairnCanonicalStateForPrompt(ctx, db, projectID, conversationID)
		if err != nil {
			return nil, err
		}
		graphYAML, err := st.ToYAML()
		if err != nil {
			return nil, fmt.Errorf("cairn canonical replanner marshal state: %w", err)
		}
		planContent, err := in.Plan.MarshalJSON()
		if err != nil {
			return nil, err
		}
		var sb strings.Builder
		sb.WriteString(buildCairnReasonPrompt(oi, graphYAML, st.FactIDs(), st.OpenIntents(), maxIntents))
		sb.WriteString("## Current Plan (intents)\n```json\n")
		sb.Write(planContent)
		sb.WriteString("\n```\n\n")
		sb.WriteString("## Executed Steps\n```\n")
		for _, step := range in.ExecutedSteps {
			sb.WriteString("Step: " + step.Step + "\n")
			sb.WriteString("Result: " + step.Result + "\n\n")
		}
		sb.WriteString("```\n\n")
		msgs := []adk.Message{
			{Role: "system", Content: sb.String()},
		}
		if rewritten, rerr := applyBeforeModelRewriteHandlers(ctx, msgs, rewriteHandlers); rerr == nil && len(rewritten) > 0 {
			msgs = rewritten
		}
		logPlanExecuteModelInputEstimate(logger, modelName, conversationID, "cairn_canonical_reason_replanner", msgs)
		return msgs, nil
	}
}

// cairnCanonicalExploreExecutorGenInput 生成 canonical Explore Executor 的模型输入。
func cairnCanonicalExploreExecutorGenInput(
	execInstruction string,
	db *database.DB,
	projectID, conversationID string,
	appCfg *config.Config,
	mwCfg *config.MultiAgentEinoMiddlewareConfig,
	logger *zap.Logger,
	modelName string,
) planexecute.GenModelInputFn {
	ei := strings.TrimSpace(execInstruction)
	return func(ctx context.Context, in *planexecute.ExecutionContext) ([]adk.Message, error) {
		st, err := loadCairnCanonicalStateForPrompt(ctx, db, projectID, conversationID)
		if err != nil {
			return nil, err
		}
		graphYAML, err := st.ToYAML()
		if err != nil {
			return nil, fmt.Errorf("cairn canonical explore marshal state: %w", err)
		}
		currentStep := in.Plan.FirstStep()
		if raw, ok := adk.GetSessionValue(ctx, cairnClaimedIntentSessionKey); ok {
			if intent, ok := raw.(*database.CairnIntent); ok && intent != nil && strings.TrimSpace(intent.Description) != "" {
				currentStep = intent.Description
			}
		}
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
		logPlanExecuteModelInputEstimate(logger, modelName, conversationID, "cairn_canonical_explore_executor", msgs)
		return msgs, nil
	}
}

// commitCairnPlanToDB 是 Planner/Replanner 唯一写入口：把 session plan 的 intents 提交 DB 并回填 canonical ID。
func commitCairnPlanToDB(ctx context.Context, db *database.DB, runID string) error {
	raw, ok := adk.GetSessionValue(ctx, planexecute.PlanSessionKey)
	if !ok {
		return nil
	}
	cp, ok := raw.(*cairnPlan)
	if !ok || cp == nil || cp.CommittedToDB {
		return nil
	}
	if len(cp.Intents) == 0 {
		return nil
	}
	descriptions := make([]string, len(cp.Intents))
	for i, it := range cp.Intents {
		descriptions[i] = it.Description
	}
	run, err := db.GetCairnRunByID(ctx, runID)
	if err != nil {
		return err
	}
	if isTerminalCairnRunLocal(run.Status) {
		return nil
	}
	intents, _, err := db.CommitCairnReasonDecision(ctx, runID, run.Revision, descriptions, "cairn_reason:"+runID+":"+cairnNowKey())
	if err != nil {
		return err
	}
	for i := range intents {
		if i < len(cp.Intents) {
			cp.Intents[i].CanonicalID = intents[i].ID
		}
	}
	cp.CommittedToDB = true
	return nil
}

type cairnCanonicalPlannerBridge struct {
	inner          adk.Agent
	db             *database.DB
	runID          string
	conversationID string
	logger         *zap.Logger
}

func (b *cairnCanonicalPlannerBridge) Name(ctx context.Context) string { return b.inner.Name(ctx) }
func (b *cairnCanonicalPlannerBridge) Description(ctx context.Context) string {
	return b.inner.Description(ctx)
}

func (b *cairnCanonicalPlannerBridge) Run(ctx context.Context, input *adk.AgentInput, opts ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
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
		if err := commitCairnPlanToDB(ctx, b.db, b.runID); err != nil && b.logger != nil {
			b.logger.Error("cairn canonical planner commit failed", zap.Error(err), zap.String("run_id", b.runID))
			gen.Send(&adk.AgentEvent{Err: fmt.Errorf("cairn canonical planner commit: %w", err)})
		}
	}()
	return out
}

type cairnCanonicalReplannerBridge struct {
	inner          adk.Agent
	db             *database.DB
	runID          string
	conversationID string
	logger         *zap.Logger
}

func (b *cairnCanonicalReplannerBridge) Name(ctx context.Context) string { return b.inner.Name(ctx) }
func (b *cairnCanonicalReplannerBridge) Description(ctx context.Context) string {
	return b.inner.Description(ctx)
}

func (b *cairnCanonicalReplannerBridge) Run(ctx context.Context, input *adk.AgentInput, opts ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	out, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	innerIter := b.inner.Run(ctx, input, opts...)
	go func() {
		defer gen.Close()
		sawBreak := false
		var lastErr error
		for {
			ev, ok := innerIter.Next()
			if !ok {
				break
			}
			if ev == nil {
				continue
			}
			if ev.Err != nil {
				lastErr = ev.Err
			}
			if ev.Action != nil && ev.Action.BreakLoop != nil {
				sawBreak = true
			}
			gen.Send(ev)
		}
		if lastErr != nil {
			// 错误由 root finalizer 统一分类写终态；这里不再重复写。
			return
		}
		if sawBreak {
			run, err := b.db.GetCairnRunByID(ctx, b.runID)
			if err != nil {
				if b.logger != nil {
					b.logger.Error("cairn canonical replanner respond load run failed", zap.Error(err))
				}
				return
			}
			if isTerminalCairnRunLocal(run.Status) {
				return
			}
			if _, err := b.db.CompleteCairnRun(ctx, b.runID, run.Revision, "goal_reached", "reason respond", "cairn_replan_complete:"+b.runID+":"+cairnNowKey()); err != nil && b.logger != nil {
				b.logger.Error("cairn canonical replanner complete run failed", zap.Error(err), zap.String("run_id", b.runID))
			}
			return
		}
		if err := commitCairnPlanToDB(ctx, b.db, b.runID); err != nil && b.logger != nil {
			b.logger.Error("cairn canonical replanner commit failed", zap.Error(err), zap.String("run_id", b.runID))
			gen.Send(&adk.AgentEvent{Err: fmt.Errorf("cairn canonical replanner commit: %w", err)})
		}
	}()
	return out
}

func classifyCairnExecutorError(ctx context.Context, runErr error) (string, string) {
	msg := strings.TrimSpace(runErr.Error())
	if len(msg) > 500 {
		msg = msg[:500]
	}
	if ctx.Err() == context.Canceled {
		return "context_canceled", msg
	}
	if ctx.Err() == context.DeadlineExceeded {
		return "timeout", msg
	}
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "max iteration") || strings.Contains(lower, "iteration limit") {
		return "max_iterations", msg
	}
	return "execution_error", msg
}

type cairnCanonicalExecutorBridge struct {
	inner          adk.Agent
	db             *database.DB
	runID          string
	conversationID string
	statePath      string
	logger         *zap.Logger
}

func (b *cairnCanonicalExecutorBridge) Name(ctx context.Context) string { return b.inner.Name(ctx) }
func (b *cairnCanonicalExecutorBridge) Description(ctx context.Context) string {
	return b.inner.Description(ctx)
}

func (b *cairnCanonicalExecutorBridge) Run(ctx context.Context, input *adk.AgentInput, opts ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	out, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	go func() {
		defer gen.Close()
		intent, err := b.claimIntent(ctx)
		if err != nil {
			gen.Send(&adk.AgentEvent{Err: fmt.Errorf("cairn canonical intent claim: %w", err)})
			return
		}
		if intent == nil {
			// 没有待执行 intent：让 Replanner 决定收尾，不产生 Fact。
			result := "[no open intent]"
			adk.AddSessionValue(ctx, planexecute.ExecutedStepSessionKey, result)
			gen.Send(adk.EventFromMessage(schema.AssistantMessage(result, nil), nil, schema.Assistant, ""))
			return
		}
		adk.AddSessionValue(ctx, cairnClaimedIntentSessionKey, intent)

		innerIter := b.inner.Run(ctx, input, opts...)
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

		result := cairnExplorerResultFromSession(ctx)
		if result == "" {
			result = lastContent
		}
		finalText, err := b.finalizeIntent(ctx, intent, result, runErr)
		if err != nil {
			gen.Send(&adk.AgentEvent{Err: fmt.Errorf("cairn canonical intent finalize: %w", err)})
			return
		}
		adk.AddSessionValue(ctx, planexecute.ExecutedStepSessionKey, finalText)
		gen.Send(adk.EventFromMessage(schema.AssistantMessage(finalText, nil), nil, schema.Assistant, ""))
	}()
	return out
}

func (b *cairnCanonicalExecutorBridge) claimIntent(ctx context.Context) (*database.CairnIntent, error) {
	run, err := b.db.GetCairnRunByID(ctx, b.runID)
	if err != nil {
		return nil, err
	}
	if isTerminalCairnRunLocal(run.Status) {
		return nil, fmt.Errorf("cairn run %s is terminal (%s)", b.runID, run.Status)
	}
	intent, _, err := b.db.ClaimNextCairnIntent(ctx, b.runID, run.Revision, "cairn_claim:"+b.runID+":"+cairnNowKey())
	if err != nil {
		return nil, err
	}
	return intent, nil
}

// finalizeIntent 分类终结当前 intent。成功 observation 才写 Fact；其余全部失败路径。
func (b *cairnCanonicalExecutorBridge) finalizeIntent(ctx context.Context, intent *database.CairnIntent, result string, runErr error) (string, error) {
	fresh, err := b.db.GetCairnRunByID(ctx, b.runID)
	if err != nil {
		return "", err
	}
	idem := "cairn_exec:" + b.runID + ":" + intent.ID + ":attempt" + fmt.Sprintf("%d", intent.AttemptCount)
	if runErr != nil {
		code, detail := classifyCairnExecutorError(ctx, runErr)
		if _, err := b.failIntent(ctx, intent, fresh.Revision, code, detail, idem); err != nil {
			return "", err
		}
		return "[intent failed: " + code + "] " + detail, nil
	}
	result = strings.TrimSpace(result)
	if result == "" {
		if _, err := b.failIntent(ctx, intent, fresh.Revision, "no_fact", "explorer returned empty result", idem); err != nil {
			return "", err
		}
		return "[explore completed without a fact]", nil
	}
	fact := &database.CairnFact{
		SourceIntentID:  intent.ID,
		SourceSegmentID: "serial-" + intent.ID[:min(8, len(intent.ID))],
		SourceEventID:   cairnExplorerEventID(intent.ID, result),
		Statement:       result,
		ObservationType: "positive",
		Confidence:      "tentative",
		EvidenceJSON:    `{}`,
		ProvenanceJSON:  fmt.Sprintf(`{"conversation_id":%q,"intent_id":%q}`, b.conversationID, intent.ID),
	}
	if _, _, err := b.db.CompleteCairnIntentWithFact(ctx, b.runID, fresh.Revision, fact, idem); err != nil {
		// CAS 冲突时重读 revision 重试一次（串行闭环中冲突只会来自重放）。
		if strings.Contains(err.Error(), "revision conflict") {
			if retry, rerr := b.db.GetCairnRunByID(ctx, b.runID); rerr == nil {
				_, _, err = b.db.CompleteCairnIntentWithFact(ctx, b.runID, retry.Revision, fact, idem)
			}
		}
		if err != nil {
			return "", err
		}
	}
	return result, nil
}

func (b *cairnCanonicalExecutorBridge) failIntent(ctx context.Context, intent *database.CairnIntent, revision int64, code, detail, idem string) (int64, error) {
	_, err := b.db.FailCairnIntentExecution(ctx, b.runID, intent.ID, revision, database.CairnIntentFailed, code, detail, idem)
	if err != nil && strings.Contains(err.Error(), "revision conflict") {
		if retry, rerr := b.db.GetCairnRunByID(ctx, b.runID); rerr == nil {
			_, err = b.db.FailCairnIntentExecution(ctx, b.runID, intent.ID, retry.Revision, database.CairnIntentFailed, code, detail, idem)
		}
	}
	return 0, err
}

// cairnCanonicalRootBridge 是 canonical 路径的最外层兜底：run 未终态时按上下文分类写终态。
type cairnCanonicalRootBridge struct {
	inner  adk.ResumableAgent
	db     *database.DB
	runID  string
	logger *zap.Logger
}

func (b *cairnCanonicalRootBridge) Name(ctx context.Context) string { return b.inner.Name(ctx) }
func (b *cairnCanonicalRootBridge) Description(ctx context.Context) string {
	return b.inner.Description(ctx)
}
func (b *cairnCanonicalRootBridge) Resume(ctx context.Context, info *adk.ResumeInfo, opts ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	return b.inner.Resume(ctx, info, opts...)
}

func (b *cairnCanonicalRootBridge) Run(ctx context.Context, input *adk.AgentInput, opts ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	out, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	innerIter := b.inner.Run(ctx, input, opts...)
	go func() {
		defer gen.Close()
		var lastErr error
		for {
			ev, ok := innerIter.Next()
			if !ok {
				break
			}
			if ev == nil {
				continue
			}
			if ev.Err != nil {
				lastErr = ev.Err
			}
			gen.Send(ev)
		}
		if err := b.finalizeRun(ctx, lastErr); err != nil && b.logger != nil {
			b.logger.Error("cairn canonical root finalize failed", zap.Error(err), zap.String("run_id", b.runID))
		}
	}()
	return out
}

func (b *cairnCanonicalRootBridge) finalizeRun(ctx context.Context, lastErr error) error {
	run, err := b.db.GetCairnRunByID(ctx, b.runID)
	if err != nil {
		return err
	}
	if isTerminalCairnRunLocal(run.Status) {
		return nil
	}
	idem := "cairn_finalize:" + b.runID + ":" + cairnNowKey()
	switch {
	case lastErr != nil:
		code, detail := classifyCairnExecutorError(ctx, lastErr)
		_, err = b.db.FailCairnRun(ctx, b.runID, run.Revision, code, detail, idem)
	case ctx.Err() == context.Canceled:
		_, err = b.db.FailCairnRun(ctx, b.runID, run.Revision, "context_canceled", "run context canceled", idem)
	case ctx.Err() == context.DeadlineExceeded:
		_, err = b.db.FailCairnRun(ctx, b.runID, run.Revision, "timeout", "run deadline exceeded", idem)
	default:
		_, err = b.db.CompleteCairnRun(ctx, b.runID, run.Revision, "normal_end", "run finished", idem)
	}
	return err
}

// newCairnCanonicalRoot 构建 canonical 串行闭环（与 legacy 相同的 planexecute 拓扑）。
func newCairnCanonicalRoot(ctx context.Context, a *CairnRootArgs) (adk.ResumableAgent, error) {
	goalText := strings.TrimSpace(a.GoalText)
	if goalText == "" {
		return nil, fmt.Errorf("cairn canonical: goal text 为空")
	}
	run, err := ensureCairnCanonicalRun(ctx, a.DB, a.ProjectID, a.ConversationID, goalText, a.InputHash)
	if err != nil {
		return nil, err
	}

	reasonCfg := &planexecute.PlannerConfig{
		ToolCallingChatModel: a.MainToolCallingModel,
		NewPlan:              newCairnPlan,
	}
	if fn := cairnCanonicalReasonPlannerGenInput(a.OrchInstruction, a.DB, a.ProjectID, a.ConversationID, a.MaxIntents, a.AppCfg, a.MwCfg, a.Logger, a.ModelName, a.PlannerReplannerRewriteHandlers); fn != nil {
		reasonCfg.GenInputFn = fn
	}
	reasonPlanner, err := planexecute.NewPlanner(ctx, reasonCfg)
	if err != nil {
		return nil, fmt.Errorf("cairn canonical reason planner: %w", err)
	}
	reasonPlanner = &cairnCanonicalPlannerBridge{inner: reasonPlanner, db: a.DB, runID: run.ID, conversationID: a.ConversationID, logger: a.Logger}

	reasonReplanner, err := planexecute.NewReplanner(ctx, &planexecute.ReplannerConfig{
		ChatModel:  a.MainToolCallingModel,
		GenInputFn: cairnCanonicalReasonReplannerGenInput(a.OrchInstruction, a.DB, a.ProjectID, a.ConversationID, a.MaxIntents, a.AppCfg, a.MwCfg, a.Logger, a.ModelName, a.PlannerReplannerRewriteHandlers),
		NewPlan:    newCairnPlan,
	})
	if err != nil {
		return nil, fmt.Errorf("cairn canonical reason replanner: %w", err)
	}
	reasonReplanner = &cairnCanonicalReplannerBridge{inner: reasonReplanner, db: a.DB, runID: run.ID, conversationID: a.ConversationID, logger: a.Logger}

	execHandlers, err := buildCairnExploreHandlers(ctx, a)
	if err != nil {
		return nil, err
	}
	executor, err := newCairnExploreExecutor(ctx, &planexecute.ExecutorConfig{
		Model:         a.ExecModel,
		ToolsConfig:   a.ToolsCfg,
		MaxIterations: cairnExplorerMaxIterations(a.ExecMaxIter),
		GenInputFn:    cairnCanonicalExploreExecutorGenInput(a.ExecInstruction, a.DB, a.ProjectID, a.ConversationID, a.AppCfg, a.MwCfg, a.Logger, a.ModelName),
	}, execHandlers)
	if err != nil {
		return nil, fmt.Errorf("cairn canonical explore executor: %w", err)
	}
	executor = &cairnCanonicalExecutorBridge{inner: executor, db: a.DB, runID: run.ID, conversationID: a.ConversationID, logger: a.Logger}

	loopMax := a.LoopMaxIter
	if loopMax <= 0 {
		loopMax = 10
	}
	root, err := planexecute.New(ctx, &planexecute.Config{
		Planner:       reasonPlanner,
		Executor:      executor,
		Replanner:     reasonReplanner,
		MaxIterations: loopMax,
	})
	if err != nil {
		return nil, fmt.Errorf("cairn canonical planexecute: %w", err)
	}
	return &cairnCanonicalRootBridge{inner: root, db: a.DB, runID: run.ID, logger: a.Logger}, nil
}
