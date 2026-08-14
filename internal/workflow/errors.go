package workflow

import "errors"

// AwaitingHITLError indicates the workflow paused before a HITL node for human approval.
type AwaitingHITLError struct {
	RunID     string
	NodeID    string
	NodeLabel string
	Prompt    string
	Reviewer  string
}

func (e *AwaitingHITLError) Error() string {
	if e == nil {
		return "workflow awaiting human approval"
	}
	return "workflow awaiting human approval at node " + e.NodeID
}

func IsAwaitingHITL(err error) bool {
	var target *AwaitingHITLError
	return errors.As(err, &target)
}
