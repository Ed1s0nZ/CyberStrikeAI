package openai

import (
	"sync"
	"time"
)

// rateLimiter serializes API requests and enforces a minimum interval between calls.
// On 429 responses, it backs off exponentially before allowing the next request.
type rateLimiter struct {
	mu          sync.Mutex
	minInterval time.Duration // minimum time between requests
	lastCall    time.Time     // when the last request was sent
	backoffEnd  time.Time     // if set, block until this time (429 backoff)
}

func newRateLimiter(minInterval time.Duration) *rateLimiter {
	return &rateLimiter{minInterval: minInterval}
}

// wait blocks until the rate limiter allows the next request.
// Returns immediately if enough time has passed since the last call.
func (rl *rateLimiter) wait() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// If we're in a 429 backoff period, sleep until it ends
	if now.Before(rl.backoffEnd) {
		sleepDur := rl.backoffEnd.Sub(now)
		rl.mu.Unlock()
		time.Sleep(sleepDur)
		rl.mu.Lock()
		now = time.Now()
	}

	// Enforce minimum interval between calls
	elapsed := now.Sub(rl.lastCall)
	if elapsed < rl.minInterval {
		sleepDur := rl.minInterval - elapsed
		rl.mu.Unlock()
		time.Sleep(sleepDur)
		rl.mu.Lock()
	}

	rl.lastCall = time.Now()
}

// backoff sets a backoff period after receiving a 429.
// duration is how long to wait before trying again.
func (rl *rateLimiter) backoff(duration time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	end := time.Now().Add(duration)
	if end.After(rl.backoffEnd) {
		rl.backoffEnd = end
	}
}
