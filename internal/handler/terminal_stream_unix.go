//go:build !windows

package handler

import (
	"bufio"
	"context"
	"os/exec"
	"strings"

	"github.com/creack/pty"
)

const ptyCols = 256
const ptyRows = 40

// runCommandStreamImpl 在 Unix 下用 PTY 执行，使 ping 等命令按终端宽度排版（isatty 为真）
func runCommandStreamImpl(cmd *exec.Cmd, sendEvent func(streamEvent), ctx context.Context) int {
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: ptyCols, Rows: ptyRows})
	if err != nil {
		sendEvent(streamEvent{T: "exit", C: -1})
		return -1
	}
	defer ptmx.Close()

	normalize := func(s string) string {
		s = strings.ReplaceAll(s, "\r\n", "\n")
		return strings.ReplaceAll(s, "\r", "\n")
	}
	sc := bufio.NewScanner(ptmx)
	for sc.Scan() {
		sendEvent(streamEvent{T: "out", D: normalize(sc.Text())})
	}
	exitCode := 0
	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}
	if ctx.Err() == context.DeadlineExceeded {
		exitCode = -1
	}
	sendEvent(streamEvent{T: "exit", C: exitCode})
	return exitCode
}
