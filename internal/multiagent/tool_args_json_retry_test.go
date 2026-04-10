package multiagent

import (
	"errors"
	"testing"
)

func TestIsRecoverableToolCallArgumentsJSONError(t *testing.T) {
	yes := errors.New(`failed to receive stream chunk: error, <400> InternalError.Algo.InvalidParameter: The "function.arguments" parameter of the code model must be in JSON format.`)
	if !isRecoverableToolCallArgumentsJSONError(yes) {
		t.Fatal("expected recoverable for function.arguments + JSON")
	}
	no := errors.New("unrelated network failure")
	if isRecoverableToolCallArgumentsJSONError(no) {
		t.Fatal("expected not recoverable")
	}
}
