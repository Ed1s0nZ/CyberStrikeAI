package multiagent

import (
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

func TestNormalizeSingleLeadingSystemMessage_MergesMultipleSystems(t *testing.T) {
	in := []adk.Message{
		schema.SystemMessage("sys-1"),
		schema.UserMessage("u1"),
		schema.SystemMessage("sys-2"),
		schema.AssistantMessage("a1", nil),
	}
	out := normalizeSingleLeadingSystemMessage(in, "orch")
	if len(out) != 3 {
		t.Fatalf("unexpected output length: got %d want 3", len(out))
	}
	if out[0].Role != schema.System {
		t.Fatalf("first message role must be system, got %s", out[0].Role)
	}
	if got := out[0].Content; got != "orch\n\nsys-1\n\nsys-2" {
		t.Fatalf("unexpected merged system content: %q", got)
	}
	if out[1].Role != schema.User || out[2].Role != schema.Assistant {
		t.Fatalf("non-system message order changed unexpectedly")
	}
}

func TestNormalizeSingleLeadingSystemMessage_NoSystemKeepsFlow(t *testing.T) {
	in := []adk.Message{
		schema.UserMessage("u1"),
		schema.AssistantMessage("a1", nil),
	}
	out := normalizeSingleLeadingSystemMessage(in, "")
	if len(out) != 2 {
		t.Fatalf("unexpected output length: got %d want 2", len(out))
	}
	if out[0].Role != schema.User || out[1].Role != schema.Assistant {
		t.Fatalf("message order changed unexpectedly")
	}
}

