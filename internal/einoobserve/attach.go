// Package einoobserve attaches CloudWeGo Eino [callbacks.Handler] to ADK Runner contexts for
// structured logging and optional SSE trace events (eino_trace_*).
package einoobserve

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"cyberstrike-ai/internal/config"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type ctxSpanKey struct{}

type ctxOtelSpanKey struct{}

// Params for attaching per-run callback instrumentation.
type Params struct {
	Logger           *zap.Logger
	Progress         func(eventType, message string, data interface{})
	ConversationID   string
	OrchMode         string
	OrchestratorName string
	RunID            string
}

// AttachAgentRunCallbacks returns ctx wrapped with callbacks.InitCallbacks when enabled.
// Safe to call with nil cfg or disabled cfg (returns ctx unchanged).
func AttachAgentRunCallbacks(ctx context.Context, cfg *config.MultiAgentEinoCallbacksConfig, p Params) context.Context {
	if ctx == nil {
		return ctx
	}
	if cfg == nil || !cfg.Enabled {
		return ctx
	}
	mode := cfg.EinoCallbacksModeEffective()
	if mode == "off" {
		return ctx
	}
	runID := strings.TrimSpace(p.RunID)
	if runID == "" {
		runID = uuid.New().String()
	}
	if p.Progress != nil && cfg.ShouldEmitEinoTraceSSE(mode) {
		p.Progress("eino_trace_run", "Eino callbacks session", map[string]interface{}{
			"runId":            runID,
			"conversationId":   strings.TrimSpace(p.ConversationID),
			"orchestration":    strings.TrimSpace(p.OrchMode),
			"orchestratorName": strings.TrimSpace(p.OrchestratorName),
			"observeMode":      mode,
			"source":           "eino_callbacks",
		})
	}
	h := &runHandler{
		cfg:    *cfg,
		mode:   mode,
		params: p,
		runID:  runID,
	}
	b := callbacks.NewHandlerBuilder().
		OnStartFn(h.onStart).
		OnEndFn(h.onEnd).
		OnErrorFn(h.onError)
	if mode == "full" {
		b = b.OnStartWithStreamInputFn(h.onStartStreamIn).OnEndWithStreamOutputFn(h.onEndStreamOut)
	}
	ri := &callbacks.RunInfo{
		Name:      "CyberStrikeADKRun",
		Type:      strings.TrimSpace(p.OrchMode),
		Component: components.Component("AgentSession"),
	}
	return callbacks.InitCallbacks(ctx, ri, b.Build())
}

type runHandler struct {
	cfg    config.MultiAgentEinoCallbacksConfig
	mode   string
	params Params
	runID  string

	mu        sync.Mutex
	spanStack []string
	seq       atomic.Uint64
}

func safeRunInfo(info *callbacks.RunInfo) callbacks.RunInfo {
	if info == nil {
		return callbacks.RunInfo{
			Name:      "unknown",
			Type:      "unknown",
			Component: components.Component("unknown"),
		}
	}
	return *info
}

func (h *runHandler) genSpanID() string {
	return fmt.Sprintf("%s-%d", h.runID, h.seq.Add(1))
}

func (h *runHandler) popSpan() (id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.spanStack) == 0 {
		return ""
	}
	id = h.spanStack[len(h.spanStack)-1]
	h.spanStack = h.spanStack[:len(h.spanStack)-1]
	return id
}

// popMatching removes the given id from the stack top if it matches; otherwise pops until empty or match (rare ordering mismatch).
func (h *runHandler) popMatching(want string) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	if want == "" {
		if len(h.spanStack) == 0 {
			return ""
		}
		id := h.spanStack[len(h.spanStack)-1]
		h.spanStack = h.spanStack[:len(h.spanStack)-1]
		return id
	}
	for len(h.spanStack) > 0 {
		top := h.spanStack[len(h.spanStack)-1]
		h.spanStack = h.spanStack[:len(h.spanStack)-1]
		if top == want {
			return top
		}
	}
	return want
}

func (h *runHandler) onStart(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
	ri := safeRunInfo(info)
	var parentID string
	h.mu.Lock()
	if len(h.spanStack) > 0 {
		parentID = h.spanStack[len(h.spanStack)-1]
	}
	spanID := h.genSpanID()
	h.spanStack = append(h.spanStack, spanID)
	h.mu.Unlock()

	inSum := summarizeCallbackInput(input, h.cfg.EinoCallbacksMaxInputSummaryRunes())
	if h.cfg.OtelTracingActive() {
		tracer := otel.Tracer("cyberstrike/eino")
		spanName := callbackSpanName(info)
		var sp trace.Span
		ctx, sp = tracer.Start(ctx, spanName,
			trace.WithSpanKind(trace.SpanKindInternal),
			trace.WithAttributes(
				attribute.String("eino.component", string(ri.Component)),
				attribute.String("eino.name", ri.Name),
				attribute.String("eino.type", ri.Type),
				attribute.String("cyberstrike.run_id", h.runID),
				attribute.String("cyberstrike.conversation_id", strings.TrimSpace(h.params.ConversationID)),
				attribute.String("cyberstrike.orchestration", strings.TrimSpace(h.params.OrchMode)),
			),
		)
		if inSum != "" {
			sp.SetAttributes(attribute.String("eino.input.summary", truncateForAttr(inSum, 256)))
		}
		ctx = context.WithValue(ctx, ctxOtelSpanKey{}, sp)
	}
	if h.params.Logger != nil {
		fields := []zap.Field{
			zap.String("runId", h.runID),
			zap.String("spanId", spanID),
			zap.String("parentSpanId", parentID),
			zap.String("component", string(ri.Component)),
			zap.String("name", ri.Name),
			zap.String("type", ri.Type),
			zap.String("phase", "start"),
		}
		if sp, ok := ctx.Value(ctxOtelSpanKey{}).(trace.Span); ok && sp != nil {
			if sc := sp.SpanContext(); sc.IsValid() {
				fields = append(fields,
					zap.String("trace_id", sc.TraceID().String()),
					zap.String("otel_span_id", sc.SpanID().String()),
				)
			}
		}
		if h.cfg.ZapVerbose {
			h.params.Logger.Debug("eino_callback", append(fields, zap.String("inputSummary", inSum))...)
		} else {
			h.params.Logger.Info("eino_callback", fields...)
		}
	}
	if h.params.Progress != nil && h.cfg.ShouldEmitEinoTraceSSE(h.mode) {
		h.params.Progress("eino_trace_start", "", map[string]interface{}{
			"runId":          h.runID,
			"spanId":         spanID,
			"parentSpanId":   parentID,
			"conversationId": strings.TrimSpace(h.params.ConversationID),
			"orchestration":  strings.TrimSpace(h.params.OrchMode),
			"component":      string(ri.Component),
			"name":           ri.Name,
			"type":           ri.Type,
			"ts":             time.Now().UTC().Format(time.RFC3339Nano),
			"inputSummary":   inSum,
			"source":         "eino_callbacks",
		})
	}
	ctx = context.WithValue(ctx, ctxSpanKey{}, spanID)
	return ctx
}

func (h *runHandler) onEnd(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
	ri := safeRunInfo(info)
	spanID, _ := ctx.Value(ctxSpanKey{}).(string)
	if spanID == "" {
		spanID = h.popSpan()
	} else {
		spanID = h.popMatching(spanID)
	}
	outSum := summarizeCallbackOutput(output, h.cfg.EinoCallbacksMaxOutputSummaryRunes())
	if sp, ok := ctx.Value(ctxOtelSpanKey{}).(trace.Span); ok && sp != nil {
		if outSum != "" {
			sp.SetAttributes(attribute.String("eino.output.summary", truncateForAttr(outSum, 256)))
		}
		sp.SetStatus(codes.Ok, "")
		sp.End()
	}
	if h.params.Logger != nil {
		fields := []zap.Field{
			zap.String("runId", h.runID),
			zap.String("spanId", spanID),
			zap.String("component", string(ri.Component)),
			zap.String("name", ri.Name),
			zap.String("type", ri.Type),
			zap.String("phase", "end"),
		}
		if h.cfg.ZapVerbose {
			h.params.Logger.Debug("eino_callback", append(fields, zap.String("outputSummary", outSum))...)
		} else {
			h.params.Logger.Info("eino_callback", fields...)
		}
	}
	if h.params.Progress != nil && h.cfg.ShouldEmitEinoTraceSSE(h.mode) {
		h.params.Progress("eino_trace_end", "", map[string]interface{}{
			"runId":          h.runID,
			"spanId":         spanID,
			"conversationId": strings.TrimSpace(h.params.ConversationID),
			"orchestration":  strings.TrimSpace(h.params.OrchMode),
			"component":      string(ri.Component),
			"name":           ri.Name,
			"type":           ri.Type,
			"ts":             time.Now().UTC().Format(time.RFC3339Nano),
			"outputSummary":  outSum,
			"source":         "eino_callbacks",
		})
	}
	return ctx
}

func (h *runHandler) onError(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
	ri := safeRunInfo(info)
	spanID, _ := ctx.Value(ctxSpanKey{}).(string)
	if spanID == "" {
		spanID = h.popSpan()
	} else {
		spanID = h.popMatching(spanID)
	}
	msg := ""
	if err != nil {
		msg = truncateRunes(err.Error(), h.cfg.EinoCallbacksMaxOutputSummaryRunes())
	}
	if sp, ok := ctx.Value(ctxOtelSpanKey{}).(trace.Span); ok && sp != nil {
		if err != nil {
			sp.RecordError(err)
		}
		sp.SetStatus(codes.Error, msg)
		sp.End()
	}
	if h.params.Logger != nil {
		h.params.Logger.Warn("eino_callback_error",
			zap.String("runId", h.runID),
			zap.String("spanId", spanID),
			zap.String("component", string(ri.Component)),
			zap.String("name", ri.Name),
			zap.String("type", ri.Type),
			zap.Error(err),
		)
	}
	if h.params.Progress != nil && h.cfg.ShouldEmitEinoTraceSSE(h.mode) {
		h.params.Progress("eino_trace_error", msg, map[string]interface{}{
			"runId":          h.runID,
			"spanId":         spanID,
			"conversationId": strings.TrimSpace(h.params.ConversationID),
			"orchestration":  strings.TrimSpace(h.params.OrchMode),
			"component":      string(ri.Component),
			"name":           ri.Name,
			"type":           ri.Type,
			"ts":             time.Now().UTC().Format(time.RFC3339Nano),
			"error":          msg,
			"source":         "eino_callbacks",
		})
	}
	return ctx
}

func (h *runHandler) onStartStreamIn(ctx context.Context, info *callbacks.RunInfo, input *schema.StreamReader[callbacks.CallbackInput]) context.Context {
	ri := safeRunInfo(info)
	if input != nil {
		input.Close()
	}
	if h.params.Logger != nil {
		h.params.Logger.Debug("eino_callback_stream_in",
			zap.String("runId", h.runID),
			zap.String("component", string(ri.Component)),
			zap.String("name", ri.Name),
		)
	}
	return ctx
}

func (h *runHandler) onEndStreamOut(ctx context.Context, info *callbacks.RunInfo, output *schema.StreamReader[callbacks.CallbackOutput]) context.Context {
	ri := safeRunInfo(info)
	if output != nil {
		output.Close()
	}
	if h.params.Logger != nil {
		h.params.Logger.Debug("eino_callback_stream_out",
			zap.String("runId", h.runID),
			zap.String("component", string(ri.Component)),
			zap.String("name", ri.Name),
		)
	}
	return ctx
}

func callbackSpanName(info *callbacks.RunInfo) string {
	if info == nil {
		return "eino.callback"
	}
	comp := strings.TrimSpace(string(info.Component))
	name := strings.TrimSpace(info.Name)
	typ := strings.TrimSpace(info.Type)
	if name != "" && comp != "" {
		return comp + "/" + name
	}
	if typ != "" && comp != "" {
		return comp + "[" + typ + "]"
	}
	if comp != "" {
		return comp
	}
	return "eino.callback"
}

func truncateForAttr(s string, maxRunes int) string {
	return truncateRunes(s, maxRunes)
}

func summarizeCallbackInput(in callbacks.CallbackInput, maxRunes int) string {
	if in == nil {
		return ""
	}
	if ai := adk.ConvAgentCallbackInput(in); ai != nil {
		parts := []string{"agent"}
		if ai.Input != nil {
			parts = append(parts, fmt.Sprintf("messages=%d", len(ai.Input.Messages)))
		}
		if ai.ResumeInfo != nil {
			parts = append(parts, "resume=true")
		}
		return strings.Join(parts, " ")
	}
	if mi := model.ConvCallbackInput(in); mi != nil {
		return fmt.Sprintf("chatModel messages=%d tools=%d", len(mi.Messages), len(mi.Tools))
	}
	if ti := tool.ConvCallbackInput(in); ti != nil {
		raw := ti.ArgumentsInJSON
		return "tool args=" + truncateRunes(raw, maxRunes)
	}
	b, err := json.Marshal(in)
	if err != nil {
		return fmt.Sprintf("%T", in)
	}
	return truncateRunes(string(b), maxRunes)
}

func summarizeCallbackOutput(out callbacks.CallbackOutput, maxRunes int) string {
	if out == nil {
		return ""
	}
	if ao := adk.ConvAgentCallbackOutput(out); ao != nil {
		return "agent_events=stream"
	}
	if mo := model.ConvCallbackOutput(out); mo != nil && mo.Message != nil {
		s := ""
		if mo.Message.Content != "" {
			s = mo.Message.Content
		}
		if mo.TokenUsage != nil {
			return fmt.Sprintf("tokens total=%d completion=%d prompt=%d text=%s",
				mo.TokenUsage.TotalTokens, mo.TokenUsage.CompletionTokens, mo.TokenUsage.PromptTokens,
				truncateRunes(s, minInt(120, maxRunes)))
		}
		return "assistant len=" + itoa(len(s))
	}
	if to := tool.ConvCallbackOutput(out); to != nil {
		if to.Response != "" {
			return truncateRunes(to.Response, maxRunes)
		}
		if to.ToolOutput != nil {
			return "tool_result multimodal"
		}
	}
	b, err := json.Marshal(out)
	if err != nil {
		return fmt.Sprintf("%T", out)
	}
	return truncateRunes(string(b), maxRunes)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}

func truncateRunes(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes]) + "…"
}
