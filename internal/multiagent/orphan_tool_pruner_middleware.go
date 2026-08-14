package multiagent

import (
	"context"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// orphanToolPrunerMiddleware 在每次 ChatModel 调用前剪掉没有对应 assistant(tool_calls) 的孤儿 tool 消息。
//
// 背景：
//   - eino 的 summarization 中间件在触发摘要后，默认把所有非 system 消息替换为 1 条 summary 消息；
//     本项目通过自定义 Finalize（summarizeFinalizeWithRecentAssistantToolTrail）在 summary 后回填
//     最近的 assistant/tool 轨迹。若 Finalize 的保留策略按"条数"截断而未按 round 对齐，可能保留
//     了 tool 结果却把对应的 assistant(tool_calls) 落在了 summary 前面，形成孤儿 tool 消息。
//   - 同样，reduction / tool_search / 自定义断点恢复等任一改写历史的逻辑，都可能破坏
//     tool_call ↔ tool_result 配对。
//
// 一旦孤儿 tool 消息进入 ChatModel，OpenAI 兼容 API（含 DashScope / 各类中转）会返回
// 400 "No tool call found for function call output with call_id ..."，并被 Eino 包装成
// [NodeRunError] 抛出，终止整轮编排。
//
// 设计取舍：
//   - 官方 patchtoolcalls 中间件只补反向（assistant(tc) 缺 tool_result），不处理孤儿 tool。
//     本中间件与之互补，专职兜底正向孤儿。
//   - 仅剔除消息，不向历史里注入虚构 assistant(tc)：虚构 tool_calls 反而会误导模型后续推理。
//     摘要已覆盖被裁剪段的语义，丢一条原始 tool 结果对对话连贯性影响最小。
//   - 位置建议：挂在 summarization / reduction / skill / plantask / system 合并 / 续聊 dedup 之后，
//     tool_search）之后，靠近 ChatModel 调用的那一端。
type orphanToolPrunerMiddleware struct {
	adk.BaseChatModelAgentMiddleware
	logger *zap.Logger
	phase  string
}

// newOrphanToolPrunerMiddleware 构造中间件。phase 仅用于日志区分 deep / supervisor /
// plan_execute_executor / sub_agent，不影响运行时行为。
func newOrphanToolPrunerMiddleware(logger *zap.Logger, phase string) adk.ChatModelAgentMiddleware {
	return &orphanToolPrunerMiddleware{
		logger: logger,
		phase:  phase,
	}
}

// BeforeModelRewriteState 扫描消息列表，收集 assistant.tool_calls 提供的 call_id 集合，
// 再剔除掉 ToolCallID 不在该集合中的 role=tool 消息。
//
// 复杂度：O(N)。当未发现孤儿时不产生任何分配，state 原样返回以便上游快路径。
func (m *orphanToolPrunerMiddleware) BeforeModelRewriteState(
	ctx context.Context,
	state *adk.ChatModelAgentState,
	mc *adk.ModelContext,
) (context.Context, *adk.ChatModelAgentState, error) {
	_ = mc
	if m == nil || state == nil || len(state.Messages) == 0 {
		return ctx, state, nil
	}

	// 第一遍：收集所有已提供的 tool_call_id；同时快路径判定是否真的存在孤儿。
	provided := make(map[string]struct{}, 8)
	for _, msg := range state.Messages {
		if msg == nil {
			continue
		}
		if msg.Role == schema.Assistant {
			for _, tc := range msg.ToolCalls {
				if tc.ID != "" {
					provided[tc.ID] = struct{}{}
				}
			}
		}
	}

	hasOrphan := false
	for _, msg := range state.Messages {
		if msg == nil {
			continue
		}
		if msg.Role == schema.Tool && msg.ToolCallID != "" {
			if _, ok := provided[msg.ToolCallID]; !ok {
				hasOrphan = true
				break
			}
		}
	}
	if !hasOrphan {
		return ctx, state, nil
	}

	// 第二遍：生成剪除孤儿后的新消息列表。
	pruned := make([]adk.Message, 0, len(state.Messages))
	droppedIDs := make([]string, 0, 2)
	droppedNames := make([]string, 0, 2)
	for _, msg := range state.Messages {
		if msg == nil {
			continue
		}
		if msg.Role == schema.Tool && msg.ToolCallID != "" {
			if _, ok := provided[msg.ToolCallID]; !ok {
				droppedIDs = append(droppedIDs, msg.ToolCallID)
				droppedNames = append(droppedNames, msg.ToolName)
				continue
			}
		}
		pruned = append(pruned, msg)
	}

	if m.logger != nil {
		m.logger.Warn("eino orphan tool messages pruned before model call",
			zap.String("phase", m.phase),
			zap.Int("dropped_count", len(droppedIDs)),
			zap.Strings("dropped_tool_call_ids", droppedIDs),
			zap.Strings("dropped_tool_names", droppedNames),
			zap.Int("messages_before", len(state.Messages)),
			zap.Int("messages_after", len(pruned)),
		)
	}

	ns := *state
	ns.Messages = pruned
	return ctx, &ns, nil
}
