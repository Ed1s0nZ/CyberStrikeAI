package openai

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// rateLimiter serializes API requests and enforces minimum intervals.
// On 429 responses, it backs off based on the retry-after header.
// Thread-safe - all API calls go through this single limiter.
type rateLimiter struct {
	mu          sync.Mutex
	minInterval time.Duration // minimum time between requests (configurable)
	lastCall    time.Time     // when the last request was sent
	backoffEnd  time.Time     // block until this time (429 backoff)
}

func newRateLimiter(minInterval time.Duration) *rateLimiter {
	return &rateLimiter{minInterval: minInterval}
}

// wait blocks until the rate limiter allows the next request.
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
func (rl *rateLimiter) backoff(duration time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	end := time.Now().Add(duration)
	if end.After(rl.backoffEnd) {
		rl.backoffEnd = end
	}
}

// parseRetryAfter extracts the retry delay from HTTP response headers.
// Checks: retry-after (standard), anthropic-ratelimit-*-reset headers.
// Returns 0 if no retry information found.
func parseRetryAfter(resp *http.Response) time.Duration {
	if resp == nil {
		return 0
	}

	// Standard retry-after header (seconds or HTTP date)
	if ra := resp.Header.Get("retry-after"); ra != "" {
		if secs, err := strconv.ParseFloat(ra, 64); err == nil && secs > 0 {
			return time.Duration(secs * float64(time.Second))
		}
		if t, err := http.ParseTime(ra); err == nil {
			d := time.Until(t)
			if d > 0 {
				return d
			}
		}
	}

	// Anthropic-specific: anthropic-ratelimit-tokens-reset (ISO 8601 timestamp)
	for _, hdr := range []string{
		"anthropic-ratelimit-tokens-reset",
		"anthropic-ratelimit-input-tokens-reset",
		"anthropic-ratelimit-requests-reset",
	} {
		if val := resp.Header.Get(hdr); val != "" {
			if t, err := time.Parse(time.RFC3339, val); err == nil {
				d := time.Until(t)
				if d > 0 {
					return d + 1*time.Second // add 1s buffer
				}
			}
		}
	}

	// OpenAI-style: x-ratelimit-reset-tokens (duration like "1s", "30s", "1m")
	for _, hdr := range []string{
		"x-ratelimit-reset-tokens",
		"x-ratelimit-reset-requests",
	} {
		if val := resp.Header.Get(hdr); val != "" {
			if d := parseDurationLoose(val); d > 0 {
				return d + 1*time.Second
			}
		}
	}

	return 0
}

// parseDurationLoose parses durations like "1s", "30s", "1m30s", "500ms"
func parseDurationLoose(s string) time.Duration {
	s = strings.TrimSpace(s)
	if d, err := time.ParseDuration(s); err == nil {
		return d
	}
	// Try as plain seconds
	if secs, err := strconv.ParseFloat(s, 64); err == nil && secs > 0 {
		return time.Duration(secs * float64(time.Second))
	}
	return 0
}
