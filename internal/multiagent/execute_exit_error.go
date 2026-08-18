package multiagent

import "fmt"

// ExecuteExitError 表示 execute 命令非零退出（预期失败，非超时/中断/流异常）。
type ExecuteExitError struct {
	Code int
}

func (e *ExecuteExitError) Error() string {
	if e == nil {
		return "exit status unknown"
	}
	return fmt.Sprintf("exit status %d", e.Code)
}
