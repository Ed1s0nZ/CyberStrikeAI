//go:build windows

package handler

import (
	"bufio"
	"context"
	"os/exec"
	"strings"
	"sync"
)

// runCommandStreamImpl 在 Windows 下用 stdout/stderr 管道执行
func runCommandStreamImpl(cmd *exec.Cmd, sendEvent func(streamEvent), ctx context.Context) int {
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		sendEvent(streamEvent{T: "exit", C: -1})
		return -1
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		sendEvent(streamEvent{T: "exit", C: -1})
		return -1
	}
	if err := cmd.Start(); err != nil {
		sendEvent(streamEvent{T: "exit", C: -1})
		return -1
	}

	normalize := func(s string) string {
		s = strings.ReplaceAll(s, "\r\n", "\n")
		return strings.ReplaceAll(s, "\r", "\n")
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		sc := bufio.NewScanner(stdoutPipe)
		for sc.Scan() {
			sendEvent(streamEvent{T: "out", D: normalize(sc.Text())})
		}
	}()
	go func() {
		defer wg.Done()
		sc := bufio.NewScanner(stderrPipe)
		for sc.Scan() {
			sendEvent(streamEvent{T: "err", D: normalize(sc.Text())})
		}
	}()

	wg.Wait()
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
