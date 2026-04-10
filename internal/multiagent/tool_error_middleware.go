package multiagent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/compose"
)

// softRecoveryToolCallMiddleware returns an InvokableToolMiddleware that catches
// specific recoverable errors from tool execution (JSON parse errors, tool-not-found,
// etc.) and converts them into soft errors: nil error + descriptive error content
// returned to the LLM. This allows the model to self-correct within the same
// iteration rather than crashing the entire graph and requiring a full replay.
//
// Without this middleware, a JSON parse failure in any tool's InvokableRun propagates
// as a hard error through the Eino ToolsNode → [NodeRunError] → ev.Err, which
// either triggers the full-replay retry loop (expensive) or terminates the run
// entirely once retries are exhausted. With it, the LLM simply sees an error message
// in the tool result and can adjust its next tool call accordingly.
func softRecoveryToolCallMiddleware() compose.InvokableToolMiddleware {
	return func(next compose.InvokableToolEndpoint) compose.InvokableToolEndpoint {
		return func(ctx context.Context, input *compose.ToolInput) (*compose.ToolOutput, error) {
			output, err := next(ctx, input)
			if err == nil {
				return output, nil
			}
			if !isSoftRecoverableToolError(err) {
				return output, err
			}
			// Convert the hard error into a soft error: the LLM will see this
			// message as the tool's output and can self-correct.
			msg := buildSoftRecoveryMessage(input.Name, input.Arguments, err)
			return &compose.ToolOutput{Result: msg}, nil
		}
	}
}

// isSoftRecoverableToolError determines whether a tool execution error should be
// silently converted to a tool-result message rather than crashing the graph.
func isSoftRecoverableToolError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())

	// JSON unmarshal/parse failures — the model generated truncated or malformed arguments.
	if isJSONRelatedError(s) {
		return true
	}

	// Sub-agent type not found (from deep/task_tool.go)
	if strings.Contains(s, "subagent type") && strings.Contains(s, "not found") {
		return true
	}

	// Tool not found in ToolsNode indexes
	if strings.Contains(s, "tool") && strings.Contains(s, "not found") {
		return true
	}

	return false
}

// isJSONRelatedError checks whether an error string indicates a JSON parsing problem.
func isJSONRelatedError(lower string) bool {
	if !strings.Contains(lower, "json") {
		return false
	}
	jsonIndicators := []string{
		"unexpected end of json",
		"unmarshal",
		"invalid character",
		"cannot unmarshal",
		"invalid tool arguments",
		"failed to unmarshal",
		"must be in json format",
		"unexpected eof",
	}
	for _, ind := range jsonIndicators {
		if strings.Contains(lower, ind) {
			return true
		}
	}
	return false
}

// buildSoftRecoveryMessage creates a bilingual error message that the LLM can act on.
func buildSoftRecoveryMessage(toolName, arguments string, err error) string {
	// Truncate arguments preview to avoid flooding the context.
	argPreview := arguments
	if len(argPreview) > 300 {
		argPreview = argPreview[:300] + "... (truncated)"
	}

	// Try to determine if it's specifically a JSON parse error for a friendlier message.
	errStr := err.Error()
	var jsonErr *json.SyntaxError
	isJSONErr := strings.Contains(strings.ToLower(errStr), "json") ||
		strings.Contains(strings.ToLower(errStr), "unmarshal")
	_ = jsonErr // suppress unused

	if isJSONErr {
		return fmt.Sprintf(
			"[Tool Error] The arguments for tool '%s' are not valid JSON and could not be parsed.\n"+
				"Error: %s\n"+
				"Arguments received: %s\n\n"+
				"Please fix the JSON (ensure double-quoted keys, matched braces/brackets, no trailing commas, "+
				"no truncation) and call the tool again.\n\n"+
				"[工具错误] 工具 '%s' 的参数不是合法 JSON，无法解析。\n"+
				"错误：%s\n"+
				"收到的参数：%s\n\n"+
				"请修正 JSON（确保双引号键名、括号配对、无尾部逗号、无截断），然后重新调用工具。",
			toolName, errStr, argPreview,
			toolName, errStr, argPreview,
		)
	}

	return fmt.Sprintf(
		"[Tool Error] Tool '%s' execution failed: %s\n"+
			"Arguments: %s\n\n"+
			"Please review the available tools and their expected arguments, then retry.\n\n"+
			"[工具错误] 工具 '%s' 执行失败：%s\n"+
			"参数：%s\n\n"+
			"请检查可用工具及其参数要求，然后重试。",
		toolName, errStr, argPreview,
		toolName, errStr, argPreview,
	)
}
