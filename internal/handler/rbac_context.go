package handler

import "context"

// detachedAgentContext lets a long-running Agent survive an SSE disconnect
// while retaining immutable request values such as the authenticated Principal.
func detachedAgentContext(parent context.Context) context.Context {
	if parent == nil {
		parent = context.Background()
	}
	return context.WithoutCancel(parent)
}
