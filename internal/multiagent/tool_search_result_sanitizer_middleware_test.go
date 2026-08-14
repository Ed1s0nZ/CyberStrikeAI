package multiagent

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

func TestToolSearchResultSanitizerRepairsMalformedHistory(t *testing.T) {
	good := &schema.Message{Role: schema.Tool, ToolName: "tool_search", Content: `{"selectedTools":["grep"]}`}
	bad := &schema.Message{Role: schema.Tool, ToolName: "tool_search", Content: "<html>502 Bad Gateway</html>"}
	other := &schema.Message{Role: schema.Tool, ToolName: "grep", Content: "plain text is valid for other tools"}
	state := &adk.ChatModelAgentState{Messages: []adk.Message{good, bad, other}}

	mw := newToolSearchResultSanitizerMiddleware(nil, "test")
	_, got, err := mw.BeforeModelRewriteState(context.Background(), state, nil)
	if err != nil {
		t.Fatalf("BeforeModelRewriteState: %v", err)
	}
	if got.Messages[0] != good || got.Messages[0].Content != good.Content {
		t.Fatal("valid tool_search result was unexpectedly changed")
	}
	if got.Messages[1] == bad || !validToolSearchResult(got.Messages[1].Content) {
		t.Fatalf("malformed result was not safely replaced: %q", got.Messages[1].Content)
	}
	if got.Messages[2] != other {
		t.Fatal("non-tool_search result was unexpectedly changed")
	}
	if bad.Content != "<html>502 Bad Gateway</html>" {
		t.Fatal("middleware mutated the original message")
	}
}

func TestToolSearchResultSanitizerFastPath(t *testing.T) {
	msg := &schema.Message{Role: schema.Tool, ToolName: "tool_search", Content: `{"selectedTools":[]}`}
	state := &adk.ChatModelAgentState{Messages: []adk.Message{msg}}
	mw := newToolSearchResultSanitizerMiddleware(nil, "test")

	_, got, err := mw.BeforeModelRewriteState(context.Background(), state, nil)
	if err != nil {
		t.Fatalf("BeforeModelRewriteState: %v", err)
	}
	if got != state {
		t.Fatal("valid history should use the allocation-free fast path")
	}
}

func TestValidToolSearchResultRejectsNonObjectJSON(t *testing.T) {
	for _, content := range []string{"null", `[]`, `"text"`, `{"selectedTools":"grep"}`} {
		if validToolSearchResult(content) {
			t.Fatalf("expected invalid tool_search result: %s", content)
		}
	}
}
