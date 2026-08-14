package security

import "os/exec"

// ShellSession 在 Start 时记录根 shell 的进程组 ID，取消/超时时可杀整组（即使 cmd.Process 已失效）。
type ShellSession struct {
	Cmd     *exec.Cmd
	rootPID int
}

// StartShellSession 配置独立进程组并启动 shell，缓存 rootPID（Unix 下即 PGID）。
func StartShellSession(cmd *exec.Cmd) (*ShellSession, error) {
	if err := prepareShellCmdSession(cmd); err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	pid := 0
	if cmd.Process != nil {
		pid = cmd.Process.Pid
	}
	return &ShellSession{Cmd: cmd, rootPID: pid}, nil
}

// Wait 等待 shell 退出。
func (s *ShellSession) Wait() error {
	if s == nil || s.Cmd == nil {
		return nil
	}
	return s.Cmd.Wait()
}

// Terminate 终止 shell 及其进程组。
func (s *ShellSession) Terminate() {
	if s == nil {
		return
	}
	terminateProcessGroup(s.rootPID, s.Cmd)
}

// TerminateShellSession 终止由 StartShellSession 启动的会话。
func TerminateShellSession(session *ShellSession) {
	if session != nil {
		session.Terminate()
	}
}
