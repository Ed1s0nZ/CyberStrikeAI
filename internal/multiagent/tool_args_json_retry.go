package multiagent

import (
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"
)

// maxToolCallRecoveryAttempts 含首次运行：首次 + 自动重试次数。
// 例如为 3 表示最多共 3 次完整 DeepAgent 运行（2 次失败后各追加一条纠错提示）。
// 该常量同时用于 JSON 参数错误和工具执行错误（如子代理名称不存在）的恢复重试。
const maxToolCallRecoveryAttempts = 5

// toolCallArgumentsJSONRetryHint 追加在用户消息后，提示模型输出合法 JSON 工具参数（部分云厂商会在流式阶段校验 arguments）。
func toolCallArgumentsJSONRetryHint() *schema.Message {
	return schema.UserMessage(`[系统提示] 上一次输出中，工具调用的 function.arguments 不是合法 JSON，接口已拒绝。请重新生成：每个 tool call 的 arguments 必须是完整、可解析的 JSON 对象字符串（键名用双引号，无多余逗号，括号配对）。不要输出截断或不完整的 JSON。

[System] Your previous tool call used invalid JSON in function.arguments and was rejected by the API. Regenerate with strictly valid JSON objects only (double-quoted keys, matched braces, no trailing commas).`)
}

// toolCallArgumentsJSONRecoveryTimelineMessage 供 eino_recovery 事件落库与前端时间线展示。
func toolCallArgumentsJSONRecoveryTimelineMessage(attempt int) string {
	return fmt.Sprintf(
		"接口拒绝了无效的工具参数 JSON。已向对话追加系统提示并要求模型重新生成合法的 function.arguments。"+
			"当前为第 %d/%d 轮完整运行。\n\n"+
			"The API rejected invalid JSON in tool arguments. A system hint was appended. This is full run %d of %d.",
		attempt+1, maxToolCallRecoveryAttempts, attempt+1, maxToolCallRecoveryAttempts,
	)
}

// isRecoverableToolCallArgumentsJSONError 判断是否为「工具参数非合法 JSON」类流式错误，可通过追加提示后重跑一轮。
func isRecoverableToolCallArgumentsJSONError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	if !strings.Contains(s, "json") {
		return false
	}
	if strings.Contains(s, "function.arguments") || strings.Contains(s, "function arguments") {
		return true
	}
	if strings.Contains(s, "invalidparameter") && strings.Contains(s, "json") {
		return true
	}
	if strings.Contains(s, "must be in json format") {
		return true
	}
	return false
}
