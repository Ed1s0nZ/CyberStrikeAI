package agent

import (
	"context"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
)

// TestConversationIDContext_RoundTrip confirms the basic wrap/read pair.
func TestConversationIDContext_RoundTrip(t *testing.T) {
	t.Parallel()

	ctx := withConversationID(context.Background(), "conv-abc")
	if got := conversationIDFromContext(ctx); got != "conv-abc" {
		t.Fatalf("expected conv-abc, got %q", got)
	}
}

// TestConversationIDContext_Nil returns the zero value without panicking
// when passed a nil context. executeToolViaMCP runs on a non-nil ctx in
// production, but the helper's nil-tolerance keeps it safe in tests and
// any future caller that forgets to wire ctx.
func TestConversationIDContext_Nil(t *testing.T) {
	t.Parallel()

	if got := conversationIDFromContext(nil); got != "" {
		t.Fatalf("expected empty string from nil ctx, got %q", got)
	}
}

// TestConversationIDContext_Missing returns empty when ctx has no id.
func TestConversationIDContext_Missing(t *testing.T) {
	t.Parallel()

	if got := conversationIDFromContext(context.Background()); got != "" {
		t.Fatalf("expected empty string for ctx without id, got %q", got)
	}
}

// TestConversationIDContext_EmptyPassthrough documents the semantic choice
// that withConversationID(ctx, "") does not overwrite a prior id. This
// lets a nested AgentLoop call inherit an outer scope instead of silently
// clearing it, which in turn means callers have to positively set "" to
// erase, which no in-tree caller does. If a future caller legitimately
// needs clear-on-empty, change the helper and update this test.
func TestConversationIDContext_EmptyPassthrough(t *testing.T) {
	t.Parallel()

	outer := withConversationID(context.Background(), "outer")
	inner := withConversationID(outer, "")
	if got := conversationIDFromContext(inner); got != "outer" {
		t.Fatalf("expected outer id to survive empty wrap, got %q", got)
	}
}

// TestConversationIDContext_Nested allows overriding the id when the
// caller passes a new non-empty value. This is the pattern
// ExecuteMCPToolForConversation relies on: outer loop wraps "A", orchestrator
// rewraps to "B" for a specific tool dispatch, and the dispatch sees "B".
func TestConversationIDContext_Nested(t *testing.T) {
	t.Parallel()

	outer := withConversationID(context.Background(), "A")
	inner := withConversationID(outer, "B")
	if got := conversationIDFromContext(outer); got != "A" {
		t.Fatalf("outer ctx got clobbered: want A, got %q", got)
	}
	if got := conversationIDFromContext(inner); got != "B" {
		t.Fatalf("inner ctx rewrap failed: want B, got %q", got)
	}
}

// TestConversationIDContext_ConcurrentIsolation is the key behavioural
// claim of the #30 refactor: two (or many) goroutines dispatching tools
// concurrently through the same Agent must each see their own
// conversation id, never another goroutine's. The old
// save-swap-restore on a shared field would fail this — the "prev"
// snapshot of one goroutine was whatever another goroutine had just
// written, so restoration unwound to the wrong value and readers
// inside executeToolViaMCP could observe the wrong id.
func TestConversationIDContext_ConcurrentIsolation(t *testing.T) {
	t.Parallel()

	const workers = 100
	const iterations = 500

	var wg sync.WaitGroup
	var mismatches atomic.Uint64
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		id := "conv-" + string(rune('A'+(w%26))) + "-" + strconv.Itoa(w)
		go func(expectedID string) {
			defer wg.Done()
			ctx := withConversationID(context.Background(), expectedID)
			for i := 0; i < iterations; i++ {
				if got := conversationIDFromContext(ctx); got != expectedID {
					mismatches.Add(1)
					return
				}
			}
		}(id)
	}
	wg.Wait()

	if n := mismatches.Load(); n != 0 {
		t.Fatalf("concurrent isolation broken: %d mismatched reads across %d workers", n, workers)
	}
}

