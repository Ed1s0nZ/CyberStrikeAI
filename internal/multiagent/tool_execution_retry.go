package multiagent

import (
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"
)

// isRecoverableToolExecutionError detects tool-level execution errors that can be
// recovered by retrying with a corrective hint. These errors originate from eino
// framework internals (e.g. task_tool.go, tool_node.go) when the LLM produces
// invalid tool calls such as non-existent sub-agent types, malformed JSON arguments,
// or unregistered tool names.
func isRecoverableToolExecutionError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())

	// Sub-agent type not found (from deep/task_tool.go)
	if strings.Contains(s, "subagent type") && strings.Contains(s, "not found") {
		return true
	}

	// Tool not found in toolsNode indexes (from compose/tool_node.go, when UnknownToolsHandler is nil)
	if strings.Contains(s, "tool") && strings.Contains(s, "not found") {
		return true
	}

	// Invalid tool arguments JSON (from einomcp/mcp_tools.go or eino internals)
	if strings.Contains(s, "invalid tool arguments json") {
		return true
	}

	// Failed to unmarshal task tool input json (from deep/task_tool.go)
	if strings.Contains(s, "failed to unmarshal") && strings.Contains(s, "json") {
		return true
	}

	// Generic tool call stream/invoke failure wrapping the above
	if (strings.Contains(s, "failed to stream tool call") || strings.Contains(s, "failed to invoke tool")) &&
		(strings.Contains(s, "not found") || strings.Contains(s, "json") || strings.Contains(s, "unmarshal")) {
		return true
	}

	return false
}

// toolExecutionRetryHint returns a user message appended to the conversation to prompt
// the LLM to correct its tool call after a tool execution error.
func toolExecutionRetryHint() *schema.Message {
	return schema.UserMessage(`[System] Your previous tool call failed because:
- The tool or sub-agent name you used does not exist, OR
- The tool call arguments were not valid JSON.

Please carefully review the available tools and sub-agents listed in your context, use only exact registered names (case-sensitive), and ensure all arguments are well-formed JSON objects. Then retry your action.

[系统提示] 上一次工具调用失败，可能原因：
- 你使用的工具名或子代理名称不存在；
- 工具调用参数不是合法 JSON。

请仔细检查上下文中列出的可用工具和子代理名称（须完全匹配、区分大小写），确保所有参数均为合法的 JSON 对象，然后重新执行。`)
}

// toolExecutionRecoveryTimelineMessage returns a message for the eino_recovery event
// displayed in the UI timeline when a tool execution error triggers a retry.
func toolExecutionRecoveryTimelineMessage(attempt int) string {
	return fmt.Sprintf(
		"工具调用执行失败（工具/子代理名称不存在或参数 JSON 无效）。已向对话追加纠错提示并要求模型重新生成。"+
			"当前为第 %d/%d 轮完整运行。\n\n"+
			"Tool call execution failed (unknown tool/sub-agent name or invalid JSON arguments). "+
			"A corrective hint was appended. This is full run %d of %d.",
		attempt+1, maxToolCallRecoveryAttempts, attempt+1, maxToolCallRecoveryAttempts,
	)
}
