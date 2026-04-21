package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	terminalMaxCommandLen = 4096
	terminalMaxOutputLen  = 256 * 1024 // 256KB
	terminalTimeout       = 30 * time.Minute
)

// TerminalHandler 
type TerminalHandler struct {
	logger *zap.Logger
}

// maskTerminalCommand ,record
func maskTerminalCommand(cmd string) string {
	trimmed := strings.TrimSpace(cmd)
	lower := strings.ToLower(trimmed)
	if strings.Contains(lower, "sudo") || strings.Contains(lower, "password") {
		return "[masked sensitive terminal command]"
	}
	if len(trimmed) > 256 {
		return trimmed[:256] + "..."
	}
	return trimmed
}

// NewTerminalHandler terminal handler
func NewTerminalHandler(logger *zap.Logger) *TerminalHandler {
	return &TerminalHandler{logger: logger}
}

// RunCommandRequest 
type RunCommandRequest struct {
	Command string `json:"command"`
	Shell   string `json:"shell,omitempty"`
	Cwd     string `json:"cwd,omitempty"`
}

// RunCommandResponse 
type RunCommandResponse struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
	Error    string `json:"error,omitempty"`
}

// RunCommand ()
func (h *TerminalHandler) RunCommand(c *gin.Context) {
	var req RunCommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body, command "})
		return
	}

	cmdStr := strings.TrimSpace(req.Command)
	if cmdStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "command "})
		return
	}
	if len(cmdStr) > terminalMaxCommandLen {
		c.JSON(http.StatusBadRequest, gin.H{"error": "command too long"})
		return
	}

	shell := req.Shell
	if shell == "" {
		if runtime.GOOS == "windows" {
			shell = "cmd"
		} else {
			shell = "sh"
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), terminalTimeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/c", cmdStr)
	} else {
		cmd = exec.CommandContext(ctx, shell, "-c", cmdStr)
		// TTY COLUMNS/TERM, ping usage 
		cmd.Env = append(os.Environ(), "COLUMNS=256", "LINES=40", "TERM=xterm-256color")
	}

	if req.Cwd != "" {
		absCwd, err := filepath.Abs(req.Cwd)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid working directory"})
			return
		}
		cur, _ := os.Getwd()
		curAbs, _ := filepath.Abs(cur)
		rel, err := filepath.Rel(curAbs, absCwd)
		if err != nil || strings.HasPrefix(rel, "..") || rel == ".." {
			c.JSON(http.StatusBadRequest, gin.H{"error": "current"})
			return
		}
		cmd.Dir = absCwd
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	stdoutBytes := stdout.Bytes()
	stderrBytes := stderr.Bytes()

	// ,(, buffer)
	truncSuffix := []byte("\n...(output truncated)\n")
	if len(stdoutBytes) > terminalMaxOutputLen {
		tmp := make([]byte, terminalMaxOutputLen+len(truncSuffix))
		n := copy(tmp, stdoutBytes[:terminalMaxOutputLen])
		copy(tmp[n:], truncSuffix)
		stdoutBytes = tmp
	}
	if len(stderrBytes) > terminalMaxOutputLen {
		tmp := make([]byte, terminalMaxOutputLen+len(truncSuffix))
		n := copy(tmp, stderrBytes[:terminalMaxOutputLen])
		copy(tmp[n:], truncSuffix)
		stderrBytes = tmp
	}

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
		if ctx.Err() == context.DeadlineExceeded {
			so := strings.ReplaceAll(string(stdoutBytes), "\r\n", "\n")
			so = strings.ReplaceAll(so, "\r", "\n")
			se := strings.ReplaceAll(string(stderrBytes), "\r\n", "\n")
			se = strings.ReplaceAll(se, "\r", "\n")
			resp := RunCommandResponse{
				Stdout:   so,
				Stderr:   se,
				ExitCode: -1,
				Error:    "command execution timed out(" + terminalTimeout.String() + ")",
			}
			c.JSON(http.StatusOK, resp)
			return
		}
		h.logger.Debug("terminal command execution error", zap.String("command", maskTerminalCommand(cmdStr)), zap.Error(err))
	}

	// \n, \r /
	stdoutStr := strings.ReplaceAll(string(stdoutBytes), "\r\n", "\n")
	stdoutStr = strings.ReplaceAll(stdoutStr, "\r", "\n")
	stderrStr := strings.ReplaceAll(string(stderrBytes), "\r\n", "\n")
	stderrStr = strings.ReplaceAll(stderrStr, "\r", "\n")

	resp := RunCommandResponse{
		Stdout:   stdoutStr,
		Stderr:   stderrStr,
		ExitCode: exitCode,
	}
	if err != nil && exitCode != 0 {
		resp.Error = err.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// streamEvent SSE 
type streamEvent struct {
	T string `json:"t"` // "out" | "err" | "exit"
	D string `json:"d,omitempty"`
	C int `json:"c"` // exit code( omitempty, 0 [exit undefined])
}

// RunCommandStream ,(SSE)
func (h *TerminalHandler) RunCommandStream(c *gin.Context) {
	var req RunCommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body, command "})
		return
	}
	cmdStr := strings.TrimSpace(req.Command)
	if cmdStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "command "})
		return
	}
	if len(cmdStr) > terminalMaxCommandLen {
		c.JSON(http.StatusBadRequest, gin.H{"error": "command too long"})
		return
	}
	shell := req.Shell
	if shell == "" {
		if runtime.GOOS == "windows" {
			shell = "cmd"
		} else {
			shell = "sh"
		}
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), terminalTimeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/c", cmdStr)
	} else {
		cmd = exec.CommandContext(ctx, shell, "-c", cmdStr)
		cmd.Env = append(os.Environ(), "COLUMNS=256", "LINES=40", "TERM=xterm-256color")
	}
	if req.Cwd != "" {
		absCwd, err := filepath.Abs(req.Cwd)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid working directory"})
			return
		}
		cur, _ := os.Getwd()
		curAbs, _ := filepath.Abs(cur)
		rel, err := filepath.Rel(curAbs, absCwd)
		if err != nil || strings.HasPrefix(rel, "..") || rel == ".." {
			c.JSON(http.StatusBadRequest, gin.H{"error": "current"})
			return
		}
		cmd.Dir = absCwd
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		cancel()
		return
	}

	sendEvent := func(ev streamEvent) {
		body, _ := json.Marshal(ev)
		c.SSEvent("", string(body))
		flusher.Flush()
	}

	runCommandStreamImpl(cmd, sendEvent, ctx)
}
