package multiagent

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
)

// noNestedTaskMiddleware 禁止在已经处于 task(sub-agent) 执行链中再次调用 task，
// 避免子代理再次委派子代理造成的无限委派/递归。
//
// 通过在 ctx 中设置临时标记来实现嵌套检测：外层 task 调用会先标记 ctx，
// 子代理内再调用 task 时会命中该标记并拒绝。
type noNestedTaskMiddleware struct {
	adk.BaseChatModelAgentMiddleware
}

type nestedTaskCtxKey struct{}

func newNoNestedTaskMiddleware() adk.ChatModelAgentMiddleware {
	return &noNestedTaskMiddleware{}
}

func (m *noNestedTaskMiddleware) WrapInvokableToolCall(
	ctx context.Context,
	endpoint adk.InvokableToolCallEndpoint,
	tCtx *adk.ToolContext,
) (adk.InvokableToolCallEndpoint, error) {
	if tCtx == nil || strings.TrimSpace(tCtx.Name) == "" {
		return endpoint, nil
	}
	// Deep 内置 task 工具名固定为 "task"；为兼容可能的大小写/空白，仅做不区分大小写匹配。
	if !strings.EqualFold(strings.TrimSpace(tCtx.Name), "task") {
		return endpoint, nil
	}

	// 已在 task 执行链中：拒绝继续委派，直接报错让上层快速终止。
	if ctx != nil {
		if v, ok := ctx.Value(nestedTaskCtxKey{}).(bool); ok && v {
			return func(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
				// Important: return a tool result text (not an error) to avoid hard-stopping the whole multi-agent run.
				// The nested task is still prevented from spawning another sub-agent, so recursion is avoided.
				_ = argumentsInJSON
				_ = opts
				return "Nested task delegation is forbidden (already inside a sub-agent delegation chain) to avoid infinite delegation. Please continue the work using the current agent's tools.", nil
			}, nil
		}
	}

	// 标记当前 task 调用链，确保子代理内的再次 task 调用能检测到嵌套。
	return func(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
		ctx2 := ctx
		if ctx2 == nil {
			ctx2 = context.Background()
		}
		ctx2 = context.WithValue(ctx2, nestedTaskCtxKey{}, true)
		return endpoint(ctx2, argumentsInJSON, opts...)
	}, nil
}

