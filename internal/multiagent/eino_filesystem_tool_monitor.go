package multiagent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"cyberstrike-ai/internal/agent"
	"cyberstrike-ai/internal/einomcp"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

// einoADKFilesystemToolNames 与 cloudwego/eino/adk/middlewares/filesystem 默认 ToolName* 一致。
// execute 已由 eino_execute_monitor 落库，此处不包含。
var einoADKFilesystemToolNames = map[string]struct{}{
	"ls":         {},
	"read_file":  {},
	"write_file": {},
	"edit_file":  {},
	"glob":       {},
	"grep":       {},
}

func isBuiltinEinoADKFilesystemToolName(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	_, ok := einoADKFilesystemToolNames[n]
	return ok
}

func toolCallArgsFromAccumulated(msgs []adk.Message, toolCallID, expectToolName string) map[string]interface{} {
	tid := strings.TrimSpace(toolCallID)
	expect := strings.TrimSpace(expectToolName)
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		if m == nil || m.Role != schema.Assistant || len(m.ToolCalls) == 0 {
			continue
		}
		for j := len(m.ToolCalls) - 1; j >= 0; j-- {
			tc := m.ToolCalls[j]
			if tid != "" && strings.TrimSpace(tc.ID) != tid {
				continue
			}
			fn := strings.TrimSpace(tc.Function.Name)
			if expect != "" && !strings.EqualFold(fn, expect) {
				continue
			}
			raw := strings.TrimSpace(tc.Function.Arguments)
			if raw == "" {
				return map[string]interface{}{}
			}
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(raw), &args); err != nil {
				return map[string]interface{}{"arguments_raw": raw}
			}
			if args == nil {
				return map[string]interface{}{}
			}
			return args
		}
	}
	return map[string]interface{}{}
}

// beginEinoADKFilesystemToolMonitor 在 Eino ADK filesystem 工具开始调用时写入 running 状态。
func beginEinoADKFilesystemToolMonitor(
	ctx context.Context,
	ag *agent.Agent,
	rec einomcp.ExecutionRecorder,
	binder *MCPExecutionBinder,
	toolCallID, toolName string,
) {
	if ag == nil || rec == nil {
		return
	}
	name := strings.TrimSpace(toolName)
	if name == "" || strings.EqualFold(name, "execute") {
		return
	}
	if !isBuiltinEinoADKFilesystemToolName(name) {
		return
	}
	tid := strings.TrimSpace(toolCallID)
	if tid == "" {
		return
	}
	storedName := "eino_fs::" + strings.ToLower(name)
	id := ag.BeginLocalToolExecution(ctx, storedName, map[string]interface{}{})
	if id == "" {
		return
	}
	rec(id, tid)
	if binder != nil {
		binder.Bind(tid, id)
	}
}

// recordEinoADKFilesystemToolMonitor 将 Eino ADK filesystem 中间件工具结果写入 MCP 监控（与 execute / MCP 桥芯片一致）。
func recordEinoADKFilesystemToolMonitor(
	ctx context.Context,
	ag *agent.Agent,
	rec einomcp.ExecutionRecorder,
	binder *MCPExecutionBinder,
	toolName string,
	toolCallID string,
	msgs []adk.Message,
	resultText string,
	isErr bool,
) {
	if ag == nil || rec == nil {
		return
	}
	name := strings.TrimSpace(toolName)
	if name == "" || strings.EqualFold(name, "execute") {
		return
	}
	if !isBuiltinEinoADKFilesystemToolName(name) {
		return
	}
	args := toolCallArgsFromAccumulated(msgs, toolCallID, name)
	storedName := "eino_fs::" + strings.ToLower(name)
	var invErr error
	if isErr {
		t := strings.TrimSpace(resultText)
		if t == "" {
			invErr = errors.New("tool error")
		} else {
			invErr = errors.New(t)
		}
	}
	execID := ""
	if binder != nil {
		execID = binder.ExecutionID(toolCallID)
	}
	id := ag.FinishLocalToolExecution(ctx, execID, storedName, args, resultText, invErr)
	if id != "" && execID == "" {
		rec(id, toolCallID)
	}
}
