package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"cyberstrike-ai/internal/database"
)

// HITLDecision is a human decision on a workflow approval node.
type HITLDecision struct {
	Approved bool
	Comment  string
}

var hitlWaiters sync.Map // runID -> chan HITLDecision

func registerHITLWaiter(runID string) chan HITLDecision {
	ch := make(chan HITLDecision, 1)
	hitlWaiters.Store(runID, ch)
	return ch
}

func unregisterHITLWaiter(runID string, ch chan HITLDecision) {
	hitlWaiters.CompareAndDelete(runID, ch)
}

// NotifyHITLDecision wakes a streaming workflow run waiting at a HITL node.
// Returns true when an active waiter was signaled.
func NotifyHITLDecision(runID string, decision HITLDecision) bool {
	v, ok := hitlWaiters.Load(runID)
	if !ok {
		return false
	}
	ch, ok := v.(chan HITLDecision)
	if !ok {
		return false
	}
	select {
	case ch <- decision:
		return true
	default:
		return true
	}
}

func readHITLDecisionFromDB(db *database.DB, runID string) (HITLDecision, bool, error) {
	if db == nil {
		return HITLDecision{}, false, nil
	}
	run, err := db.GetWorkflowRun(runID)
	if err != nil {
		return HITLDecision{}, false, err
	}
	if run == nil || strings.TrimSpace(run.PendingHITLJSON) == "" {
		return HITLDecision{}, false, nil
	}
	var pending map[string]interface{}
	if err := json.Unmarshal([]byte(run.PendingHITLJSON), &pending); err != nil {
		return HITLDecision{}, false, nil
	}
	raw, ok := pending["decision"]
	if !ok {
		return HITLDecision{}, false, nil
	}
	decision := strings.ToLower(strings.TrimSpace(fmt.Sprint(raw)))
	switch decision {
	case "approved", "approve":
		comment := ""
		if v, ok := pending["comment"]; ok {
			comment = strings.TrimSpace(fmt.Sprint(v))
		}
		return HITLDecision{Approved: true, Comment: comment}, true, nil
	case "rejected", "reject":
		comment := ""
		if v, ok := pending["comment"]; ok {
			comment = strings.TrimSpace(fmt.Sprint(v))
		}
		return HITLDecision{Approved: false, Comment: comment}, true, nil
	default:
		return HITLDecision{}, false, nil
	}
}

func waitWorkflowHITLDecision(ctx context.Context, db *database.DB, runID string) (HITLDecision, error) {
	ch := registerHITLWaiter(runID)
	defer unregisterHITLWaiter(runID, ch)
	return waitWorkflowHITLDecisionWithChannel(ctx, db, runID, ch)
}

func waitWorkflowHITLDecisionWithChannel(ctx context.Context, db *database.DB, runID string, ch chan HITLDecision) (HITLDecision, error) {
	if d, ok, err := readHITLDecisionFromDB(db, runID); err != nil {
		return HITLDecision{}, err
	} else if ok {
		return d, nil
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return HITLDecision{}, ctx.Err()
		case d := <-ch:
			return d, nil
		case <-ticker.C:
			if d, ok, err := readHITLDecisionFromDB(db, runID); err != nil {
				return HITLDecision{}, err
			} else if ok {
				return d, nil
			}
		}
	}
}
