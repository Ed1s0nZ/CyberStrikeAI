//go:build windows

package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RunCommandWS is not available on Windows because the interactive terminal
// implementation depends on Unix PTY support.
func (h *TerminalHandler) RunCommandWS(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "interactive terminal websocket is not supported on Windows",
	})
}
