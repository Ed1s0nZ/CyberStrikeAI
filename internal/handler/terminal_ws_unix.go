//go:build !windows

package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/creack/pty"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// terminalResize is sent by the frontend when the xterm.js terminal is resized.
type terminalResize struct {
	Type string `json:"type"`
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

// wsUpgrader 仅用于系统设置中的终端 WebSocket，会复用已有的登录保护（JWT 中间件在上层路由组）
var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// 由于已在 Gin 路由层做了认证，这里放宽 Origin，方便在同一域名下通过 HTTPS/WSS 访问
		return true
	},
}

// RunCommandWS 提供真正交互式 Shell：基于 WebSocket + PTY 的长会话
// 前端建立 WebSocket 连接后，所有键盘输入都会透传到 Shell，Shell 的输出也会实时写回前端。
func (h *TerminalHandler) RunCommandWS(c *gin.Context) {
	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// 启动交互式 Shell，这里优先使用 bash，找不到则退回 sh
	shell := "bash"
	if _, err := exec.LookPath(shell); err != nil {
		shell = "sh"
	}
	cmd := exec.Command(shell)
	cmd.Env = append(os.Environ(),
		"COLUMNS=80",
		"LINES=24",
		"TERM=xterm-256color",
	)

	// Use 80x24 as a safe default; the frontend will send the actual size immediately after connecting.
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: 80, Rows: 24})
	if err != nil {
		return
	}
	defer ptmx.Close()

	// Shell -> WebSocket：将 PTY 输出实时发给前端
	doneChan := make(chan struct{})
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				_ = conn.WriteMessage(websocket.BinaryMessage, buf[:n])
			}
			if err != nil {
				break
			}
		}
		close(doneChan)
	}()

	// WebSocket -> Shell：将前端输入写入 PTY（包括 sudo 密码、Ctrl+C 等）
	conn.SetReadLimit(64 * 1024)
	_ = conn.SetReadDeadline(time.Now().Add(terminalTimeout))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(terminalTimeout))
		return nil
	})

	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			_ = cmd.Process.Kill()
			break
		}
		if msgType != websocket.TextMessage && msgType != websocket.BinaryMessage {
			continue
		}
		if len(data) == 0 {
			continue
		}
		// Check if this is a resize message (JSON with type:"resize")
		if msgType == websocket.TextMessage && len(data) > 0 && data[0] == '{' {
			var resize terminalResize
			if json.Unmarshal(data, &resize) == nil && resize.Type == "resize" && resize.Cols > 0 && resize.Rows > 0 {
				_ = pty.Setsize(ptmx, &pty.Winsize{Cols: resize.Cols, Rows: resize.Rows})
				continue
			}
		}
		if _, err := ptmx.Write(data); err != nil {
			_ = cmd.Process.Kill()
			break
		}
	}

	<-doneChan
}
