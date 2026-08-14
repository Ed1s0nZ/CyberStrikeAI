package multiagent

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

func TestToolCallArgumentsSanitizerRepairsOnlyMalformedObjects(t *testing.T) {
	valid := assistantToolCallsMsg("", "valid")
	valid.ToolCalls[0].Function.Arguments = `{"command":"echo ok"}`
	malformed := assistantToolCallsMsg("", "broken", "array")
	malformed.ToolCalls[0].Function.Arguments = `{"command":"unterminated`
	malformed.ToolCalls[1].Function.Arguments = `[]`
	messages := []adk.Message{valid, malformed, schema.ToolMessage("failed", "broken")}

	out, repaired := sanitizeMalformedToolCallArguments(messages)
	if repaired != 2 {
		t.Fatalf("repaired=%d, want 2", repaired)
	}
	if out[0].ToolCalls[0].Function.Arguments != `{"command":"echo ok"}` {
		t.Fatalf("valid arguments changed: %q", out[0].ToolCalls[0].Function.Arguments)
	}
	for _, tc := range out[1].ToolCalls {
		if tc.Function.Arguments != repairedMalformedToolArguments {
			t.Fatalf("malformed arguments not repaired: %q", tc.Function.Arguments)
		}
	}
	if malformed.ToolCalls[0].Function.Arguments == repairedMalformedToolArguments {
		t.Fatal("input message was mutated")
	}
}

func TestToolCallArgumentsSanitizerMiddlewareRewritesState(t *testing.T) {
	msg := assistantToolCallsMsg("", "broken")
	msg.ToolCalls[0].Function.Arguments = ""
	mw := newToolCallArgumentsSanitizerMiddleware(nil, "test").(*toolCallArgumentsSanitizerMiddleware)
	_, state, err := mw.BeforeModelRewriteState(context.Background(), &adk.ChatModelAgentState{
		Messages: []adk.Message{msg},
	}, &adk.ModelContext{})
	if err != nil {
		t.Fatal(err)
	}
	if got := state.Messages[0].ToolCalls[0].Function.Arguments; got != `{}` {
		t.Fatalf("arguments=%q, want {}", got)
	}
}

func TestValidToolArgumentsJSONObject(t *testing.T) {
	cases := map[string]bool{
		`{}`:      true,
		`{"x":1}`: true,
		`null`:    false,
		`[]`:      false,
		`{"x":`:   false,
		``:        false,
	}
	for input, want := range cases {
		if got := validToolArgumentsJSONObject(input); got != want {
			t.Errorf("validToolArgumentsJSONObject(%q)=%v, want %v", input, got, want)
		}
	}
}
