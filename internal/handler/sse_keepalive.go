package handler

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// sseInterval is how often we write on long SSE streams. Shorter intervals help NATs and
// some proxies that treat connections as idle; 10s is a reasonable balance with traffic.
const sseKeepaliveInterval = 10 * time.Second

// runSSEKeepalive starts periodic SSE heartbeats in a background goroutine.
// The returned stop function must be deferred (or called) before the handler returns so the
// goroutine exits before Gin finalizes the ResponseWriter (avoids "Write called after Handler finished").
//
// writeMu must be the same mutex used by the handler's event writes for this request: concurrent
// writes to http.ResponseWriter break chunked transfer encoding (browser: net::ERR_INVALID_CHUNKED_ENCODING).
func runSSEKeepalive(c *gin.Context, writeMu *sync.Mutex) func() {
	if writeMu == nil {
		return func() {}
	}
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		sseKeepaliveLoop(c, stop, writeMu)
	}()
	var once sync.Once
	return func() {
		once.Do(func() {
			close(stop)
			wg.Wait()
		})
	}
}

// sseKeepaliveLoop sends periodic SSE traffic so proxies (e.g. nginx proxy_read_timeout), NATs,
// and load balancers do not close long-running streams. Some intermediaries ignore comment-only
// lines, so we send both a comment and a minimal data frame (type heartbeat) per tick.
func sseKeepaliveLoop(c *gin.Context, stop <-chan struct{}, writeMu *sync.Mutex) {
	ticker := time.NewTicker(sseKeepaliveInterval)
	defer ticker.Stop()
	ctx := c.Request.Context()
	for {
		select {
		case <-stop:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			writeMu.Lock()
			if sseShuttingDown(stop, ctx) {
				writeMu.Unlock()
				return
			}
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

func sseShuttingDown(stop <-chan struct{}, ctx context.Context) bool {
	select {
	case <-stop:
		return true
	case <-ctx.Done():
		return true
	default:
		return false
	}
}
