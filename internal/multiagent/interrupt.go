package multiagent

import "errors"

// ErrInterruptContinue 作为 context.CancelCause 使用：用户选择「中断并继续」且当前无进行中的 MCP 工具时，
// 取消当前推理/流式输出，并在同一会话任务内携带用户补充说明自动续跑下一轮（类似 Hermes 式人机回合）。
var ErrInterruptContinue = errors.New("agent interrupt: continue with user-supplied context")
