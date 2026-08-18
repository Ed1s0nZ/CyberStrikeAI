package handler

import (
	"context"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestRunSSEKeepaliveStopsBeforeHandlerReturns(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/events", nil)

	var writeMu sync.Mutex
	stop := runSSEKeepalive(c, &writeMu)
	stop()

	// A second stop must be safe (channel already closed, goroutine already exited).
	stop()
}

func TestRunSSEKeepaliveExitsOnClientDisconnect(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	ctx, cancel := context.WithCancel(context.Background())
	c.Request = httptest.NewRequest("GET", "/events", nil).WithContext(ctx)

	var writeMu sync.Mutex
	stop := runSSEKeepalive(c, &writeMu)
	cancel()

	done := make(chan struct{})
	go func() {
		stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("keepalive stop did not complete after client disconnect")
	}
}

func TestRunSSEKeepaliveNilMutexIsNoop(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/events", nil)

	stop := runSSEKeepalive(c, nil)
	stop()
}
