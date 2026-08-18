package multiagent

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

const repairedMalformedToolArguments = `{}`

// toolCallArgumentsSanitizerMiddleware guarantees that every historical
// tool_calls[].function.arguments value sent to an OpenAI-compatible provider is
// a syntactically valid JSON object. Some providers reject the entire request
// with HTTP 400 when a model previously emitted truncated arguments.
//
// The original malformed payload is intentionally not copied into model-facing
// history: it may contain secrets and can itself be large enough to trigger the
// same failure again. The paired tool result already records the execution error
// and gives the model enough information to recover.
type toolCallArgumentsSanitizerMiddleware struct {
	adk.BaseChatModelAgentMiddleware
	logger *zap.Logger
	phase  string
}

func newToolCallArgumentsSanitizerMiddleware(logger *zap.Logger, phase string) adk.ChatModelAgentMiddleware {
	return &toolCallArgumentsSanitizerMiddleware{logger: logger, phase: phase}
}

func (m *toolCallArgumentsSanitizerMiddleware) BeforeModelRewriteState(
	ctx context.Context,
	state *adk.ChatModelAgentState,
	mc *adk.ModelContext,
) (context.Context, *adk.ChatModelAgentState, error) {
	_ = mc
	if m == nil || state == nil || len(state.Messages) == 0 {
		return ctx, state, nil
	}

	out, repaired := sanitizeMalformedToolCallArguments(state.Messages)
	if repaired == 0 {
		return ctx, state, nil
	}
	if m.logger != nil {
		m.logger.Warn("eino malformed tool-call arguments repaired before model call",
			zap.String("phase", m.phase),
			zap.Int("repaired_calls", repaired),
		)
	}
	ns := *state
	ns.Messages = out
	return ctx, &ns, nil
}

func sanitizeMalformedToolCallArguments(messages []adk.Message) ([]adk.Message, int) {
	var out []adk.Message
	repaired := 0
	for i, msg := range messages {
		if msg == nil || msg.Role != schema.Assistant || len(msg.ToolCalls) == 0 {
			continue
		}
		calls := append([]schema.ToolCall(nil), msg.ToolCalls...)
		changed := false
		for j := range calls {
			if validToolArgumentsJSONObject(calls[j].Function.Arguments) {
				continue
			}
			calls[j].Function.Arguments = repairedMalformedToolArguments
			changed = true
			repaired++
		}
		if !changed {
			continue
		}
		if out == nil {
			out = append([]adk.Message(nil), messages...)
		}
		cloned := *msg
		cloned.ToolCalls = calls
		out[i] = &cloned
	}
	if out == nil {
		return messages, 0
	}
	return out, repaired
}

func validToolArgumentsJSONObject(arguments string) bool {
	arguments = strings.TrimSpace(arguments)
	if arguments == "" {
		return false
	}
	var value any
	if err := json.Unmarshal([]byte(arguments), &value); err != nil {
		return false
	}
	_, ok := value.(map[string]any)
	return ok
}
