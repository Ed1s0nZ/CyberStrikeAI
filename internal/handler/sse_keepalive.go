package handler

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// sseInterval is how often we write on long SSE streams. Shorter intervals help NATs and
// some proxies that treat connections as idle; 10s is a reasonable balance with traffic.
const sseKeepaliveInterval = 10 * time.Second

// sseKeepalive sends periodic SSE traffic so proxies (e.g. nginx proxy_read_timeout), NATs,
// and load balancers do not close long-running streams. Some intermediaries ignore comment-only
// lines, so we send both a comment and a minimal data frame (type heartbeat) per tick.
//
// writeMu must be the same mutex used by sendEvent for this request: concurrent writes to
// http.ResponseWriter break chunked transfer encoding (browser: net::ERR_INVALID_CHUNKED_ENCODING).
func sseKeepalive(c *gin.Context, stop <-chan struct{}, writeMu *sync.Mutex) {
	if writeMu == nil {
		return
	}
	ticker := time.NewTicker(sseKeepaliveInterval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
			select {
			case <-stop:
				return
			case <-c.Request.Context().Done():
				return
			default:
			}
			writeMu.Lock()
			if _, err := fmt.Fprintf(c.Writer, ": keepalive\n\n"); err != nil {
				writeMu.Unlock()
				return
			}
			// data: frame so strict proxies still see downstream bytes (comments alone may not reset timers)
			if _, err := fmt.Fprintf(c.Writer, `data: {"type":"heartbeat"}`+"\n\n"); err != nil {
				writeMu.Unlock()
				return
			}
			if flusher, ok := c.Writer.(http.Flusher); ok {
				flusher.Flush()
			}
			writeMu.Unlock()
		}
	}
}
