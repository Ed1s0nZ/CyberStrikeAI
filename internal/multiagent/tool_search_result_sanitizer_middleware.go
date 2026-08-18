package multiagent

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// toolSearchResultSanitizerMiddleware prevents malformed historical tool_search
// results (for example an HTML gateway error page) from crashing Eino's dynamic
// tool loader on every retry. Eino expects every tool_search result to be a JSON
// object containing selectedTools.
type toolSearchResultSanitizerMiddleware struct {
	adk.BaseChatModelAgentMiddleware
	logger *zap.Logger
	phase  string
}

func newToolSearchResultSanitizerMiddleware(logger *zap.Logger, phase string) adk.ChatModelAgentMiddleware {
	return &toolSearchResultSanitizerMiddleware{logger: logger, phase: phase}
}

type toolSearchResultEnvelope struct {
	SelectedTools []string `json:"selectedTools"`
}

func validToolSearchResult(content string) bool {
	var result toolSearchResultEnvelope
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return false
	}
	// Reject JSON values such as null. They unmarshal without an error but do not
	// satisfy the object-shaped contract used by the toolsearch middleware.
	return strings.HasPrefix(strings.TrimSpace(content), "{")
}

func (m *toolSearchResultSanitizerMiddleware) BeforeModelRewriteState(
	ctx context.Context,
	state *adk.ChatModelAgentState,
	_ *adk.ModelContext,
) (context.Context, *adk.ChatModelAgentState, error) {
	if m == nil || state == nil || len(state.Messages) == 0 {
		return ctx, state, nil
	}

	var rewritten []adk.Message
	repaired := 0
	for i, msg := range state.Messages {
		if msg == nil || msg.Role != schema.Tool || !IsToolSearchTool(msg.ToolName) || validToolSearchResult(msg.Content) {
			continue
		}
		if rewritten == nil {
			rewritten = append([]adk.Message(nil), state.Messages...)
		}
		clone := *msg
		clone.Content = `{"selectedTools":[],"_recovered":true,"reason":"invalid historical tool_search result"}`
		rewritten[i] = &clone
		repaired++
	}

	if repaired == 0 {
		return ctx, state, nil
	}
	if m.logger != nil {
		m.logger.Warn("invalid historical tool_search results repaired before model call",
			zap.String("phase", m.phase),
			zap.Int("repaired_count", repaired))
	}
	ns := *state
	ns.Messages = rewritten
	return ctx, &ns, nil
}
