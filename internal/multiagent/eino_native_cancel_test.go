package multiagent

import (
	"context"
	"testing"
)

func TestEinoNativeCancelOptionsByCause(t *testing.T) {
	fullStopOpts, fullStopWait := einoNativeCancelOptions(context.Canceled)
	if len(fullStopOpts) != 2 {
		t.Fatalf("full stop options: got %d want 2", len(fullStopOpts))
	}
	if fullStopWait != einoNativeCancelImmediateWait {
		t.Fatalf("full stop wait: got %v want %v", fullStopWait, einoNativeCancelImmediateWait)
	}

	interruptOpts, interruptWait := einoNativeCancelOptions(ErrInterruptContinue)
	if len(interruptOpts) != 3 {
		t.Fatalf("interrupt options: got %d want 3", len(interruptOpts))
	}
	if interruptWait != einoNativeCancelSafePointWait {
		t.Fatalf("interrupt wait: got %v want %v", interruptWait, einoNativeCancelSafePointWait)
	}
	if interruptWait <= einoNativeCancelSafePointTTL {
		t.Fatalf("interrupt wait must allow the Eino safe-point timeout to elapse: wait=%v ttl=%v", interruptWait, einoNativeCancelSafePointTTL)
	}
}
