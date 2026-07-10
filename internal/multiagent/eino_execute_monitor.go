package multiagent

import (
	"context"
	"fmt"

	"cyberstrike-ai/internal/agent"
	"cyberstrike-ai/internal/einomcp"
)

// newEinoExecuteMonitorCallbacks 在 Eino filesystem execute 开始/结束时写入 MCP 监控库并 recorder(executionId)，
// 与 CallTool 路径一致，使监控页能展示「执行中」状态。
func newEinoExecuteMonitorCallbacks(ctx context.Context, ag *agent.Agent, recorder einomcp.ExecutionRecorder) (
	begin func(toolCallID, command string) string,
	finish func(executionID, toolCallID, command, stdout string, success bool, invokeErr error),
) {
	begin = func(toolCallID, command string) string {
		if ag == nil {
			return ""
		}
		args := map[string]interface{}{"command": command}
		id := ag.BeginLocalToolExecution(ctx, "execute", args)
		if id != "" && recorder != nil {
			recorder(id, toolCallID)
		}
		return id
	}
	finish = func(executionID, toolCallID, command, stdout string, success bool, invokeErr error) {
		if ag == nil {
			return
		}
		var err error
		if !success {
			if invokeErr != nil {
				err = invokeErr
			} else {
				err = fmt.Errorf("execute failed")
			}
		}
		args := map[string]interface{}{"command": command}
		id := ag.FinishLocalToolExecution(ctx, executionID, "execute", args, stdout, err)
		if id != "" && recorder != nil && executionID == "" {
			recorder(id, toolCallID)
		}
	}
	return begin, finish
}
