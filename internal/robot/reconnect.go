package robot

import (
	"context"
	"time"
)

const (
	reconnectInitial = 5 * time.Second
	reconnectMax     = 60 * time.Second
)

func waitReconnect(ctx context.Context, backoff *time.Duration) bool {
	if ctx.Err() != nil {
		return false
	}
	select {
	case <-ctx.Done():
		return false
	case <-time.After(*backoff):
		if *backoff < reconnectMax {
			*backoff *= 2
			if *backoff > reconnectMax {
				*backoff = reconnectMax
			}
		}
		return true
	}
}

func bumpBackoff(backoff *time.Duration) {
	if *backoff < reconnectMax {
		*backoff *= 2
		if *backoff > reconnectMax {
			*backoff = reconnectMax
		}
	}
}
