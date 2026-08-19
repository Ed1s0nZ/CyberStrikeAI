package multiagent

import (
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestConcatToolResultChunksUsesEinoConcatForSingleCall(t *testing.T) {
	got, err := concatToolResultChunks([]*schema.Message{
		schema.ToolMessage("hel", "call-1", schema.WithToolName("execute")),
		schema.ToolMessage("lo", "call-1", schema.WithToolName("execute")),
	})
	if err != nil {
		t.Fatalf("concat: %v", err)
	}
	if len(got) != 1 || got[0].ToolCallID != "call-1" || got[0].Content != "hello" || got[0].ToolName != "execute" {
		t.Fatalf("got = %#v, want one ConcatMessages result", got)
	}
}

func TestConcatToolResultChunksSplitsParallelCalls(t *testing.T) {
	got, err := concatToolResultChunks([]*schema.Message{
		schema.ToolMessage("nmap 1/2 ", "call-1", schema.WithToolName("nmap")),
		schema.ToolMessage("nmap 2/2 ", "call-2", schema.WithToolName("nmap")),
		schema.ToolMessage("22/tcp", "call-1", schema.WithToolName("nmap")),
		schema.ToolMessage("80/tcp", "call-2", schema.WithToolName("nmap")),
	})
	if err != nil {
		t.Fatalf("concat: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got = %#v, want two calls", got)
	}
	if got[0].ToolCallID != "call-1" || got[0].Content != "nmap 1/2 22/tcp" {
		t.Fatalf("call-1 = %#v", got[0])
	}
	if got[1].ToolCallID != "call-2" || got[1].Content != "nmap 2/2 80/tcp" {
		t.Fatalf("call-2 = %#v", got[1])
	}
}
