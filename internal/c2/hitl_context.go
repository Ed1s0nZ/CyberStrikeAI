package c2

import "context"

type hitlRunCtxKey struct{}

// WithHITLRunContext 将 runCtx（通常为整条 Agent / SSE 请求生命周期）挂到传入的 ctx 上。
// MCP 工具 handler 收到的 ctx 可能是带单次工具超时的子 context，在工具 return 时会被 cancel；
// 危险任务 HITL 应通过 HITLUserContext 使用 runCtx 等待人工审批。
func WithHITLRunContext(ctx, runCtx context.Context) context.Context {
	if ctx == nil || runCtx == nil {
		return ctx
	}
	return context.WithValue(ctx, hitlRunCtxKey{}, runCtx)
}

// HITLUserContext 返回用于 C2 危险任务 HITL 等待的 context：
// 若曾用 WithHITLRunContext 注入更长寿命的 runCtx 则返回之，否则返回 ctx。
func HITLUserContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	if v := ctx.Value(hitlRunCtxKey{}); v != nil {
		if run, ok := v.(context.Context); ok && run != nil {
			return run
		}
	}
	return ctx
}
