package multiagent

import (
	"context"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// agenticOrphanToolPrunerMiddleware is the AgenticMessage equivalent of
// orphanToolPrunerMiddleware. It removes user-role messages whose content
// blocks are exclusively FunctionToolResult entries with CallIDs that do not
// match any FunctionToolCall in the history.
//
// This is a defense-in-depth layer after agenticToolPairReconcilerMiddleware;
// the reconciler handles the common case (assistant followed by its results)
// while this pruner catches stray results that appear before their assistant
// or in non-adjacent positions (e.g. after summarization rewriting).
type agenticOrphanToolPrunerMiddleware struct {
	*adk.TypedBaseChatModelAgentMiddleware[*schema.AgenticMessage]
	logger *zap.Logger
	phase  string
}

func newAgenticOrphanToolPrunerMiddleware(logger *zap.Logger, phase string) adk.TypedChatModelAgentMiddleware[*schema.AgenticMessage] {
	return &agenticOrphanToolPrunerMiddleware{
		TypedBaseChatModelAgentMiddleware: &adk.TypedBaseChatModelAgentMiddleware[*schema.AgenticMessage]{},
		logger:                            logger,
		phase:                             phase,
	}
}

func (m *agenticOrphanToolPrunerMiddleware) BeforeModelRewriteState(
	ctx context.Context,
	state *adk.TypedChatModelAgentState[*schema.AgenticMessage],
	mc *adk.TypedModelContext[*schema.AgenticMessage],
) (context.Context, *adk.TypedChatModelAgentState[*schema.AgenticMessage], error) {
	_ = mc
	if m == nil || state == nil || len(state.Messages) == 0 {
		return ctx, state, nil
	}

	// Pass 1: collect all provided CallIDs from assistant FunctionToolCall blocks.
	provided := make(map[string]struct{}, 8)
	for _, msg := range state.Messages {
		if msg == nil || msg.Role != schema.AgenticRoleTypeAssistant {
			continue
		}
		for _, block := range msg.ContentBlocks {
			if block != nil && block.FunctionToolCall != nil && block.FunctionToolCall.CallID != "" {
				provided[block.FunctionToolCall.CallID] = struct{}{}
			}
		}
	}

	// Fast path: check if any orphan exists.
	hasOrphan := false
	for _, msg := range state.Messages {
		if msg == nil || !isPureAgenticToolResult(msg) {
			continue
		}
		for _, id := range agenticToolResultCallIDs(msg) {
			if _, ok := provided[id]; !ok {
				hasOrphan = true
				break
			}
		}
		if hasOrphan {
			break
		}
	}
	if !hasOrphan {
		return ctx, state, nil
	}

	// Pass 2: build pruned list.
	pruned := make([]*schema.AgenticMessage, 0, len(state.Messages))
	var droppedIDs []string
	var droppedNames []string
	for _, msg := range state.Messages {
		if msg == nil {
			continue
		}
		if !isPureAgenticToolResult(msg) {
			pruned = append(pruned, msg)
			continue
		}
		// Check if ALL result call IDs are orphans. If any is matched, keep the
		// message (the reconciler already handled partial mismatches).
		allOrphan := true
		for _, id := range agenticToolResultCallIDs(msg) {
			if _, ok := provided[id]; ok {
				allOrphan = false
				break
			}
		}
		if allOrphan {
			for _, block := range msg.ContentBlocks {
				if block != nil && block.FunctionToolResult != nil {
					droppedIDs = append(droppedIDs, block.FunctionToolResult.CallID)
					droppedNames = append(droppedNames, block.FunctionToolResult.Name)
				}
			}
			continue
		}
		pruned = append(pruned, msg)
	}

	if len(droppedIDs) == 0 {
		return ctx, state, nil
	}
	if m.logger != nil {
		m.logger.Warn("agentic orphan tool messages pruned before model call",
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
