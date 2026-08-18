package multiagent

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
)

func TestRecvSchemaMessageStream_EOF(t *testing.T) {
	sr, sw := schema.Pipe[*schema.Message](4)
	_ = sw.Send(schema.ToolMessage("hello", "tc-1"), nil)
	sw.Close()

	content, tid, toolName, err := recvSchemaMessageStream(context.Background(), sr)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if content != "hello" {
		t.Fatalf("content=%q want hello", content)
	}
	if tid != "tc-1" {
		t.Fatalf("toolCallID=%q want tc-1", tid)
	}
	if toolName != "" {
		t.Fatalf("toolName=%q want empty", toolName)
	}
}

func TestRecvSchemaToolResultMessages_SplitsParallelIDs(t *testing.T) {
	sr, sw := schema.Pipe[*schema.Message](8)
	_ = sw.Send(schema.ToolMessage("one-", "tc-1", schema.WithToolName("nmap")), nil)
	_ = sw.Send(schema.ToolMessage("two-", "tc-2", schema.WithToolName("nmap")), nil)
	_ = sw.Send(schema.ToolMessage("a", "tc-1", schema.WithToolName("nmap")), nil)
	_ = sw.Send(schema.ToolMessage("b", "tc-2", schema.WithToolName("nmap")), nil)
	sw.Close()

	msgs, err := recvSchemaToolResultMessages(context.Background(), sr)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("msgs = %#v, want 2", msgs)
	}
	if msgs[0].ToolCallID != "tc-1" || msgs[0].Content != "one-a" {
		t.Fatalf("msg 0 = %#v", msgs[0])
	}
	if msgs[1].ToolCallID != "tc-2" || msgs[1].Content != "two-b" {
		t.Fatalf("msg 1 = %#v", msgs[1])
	}
}

func TestRecvSchemaMessageStream_CapturesToolName(t *testing.T) {
	sr, sw := schema.Pipe[*schema.Message](4)
	_ = sw.Send(schema.ToolMessage("hello", "tc-1", schema.WithToolName("execute")), nil)
	sw.Close()

	content, tid, toolName, err := recvSchemaMessageStream(context.Background(), sr)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if content != "hello" || tid != "tc-1" || toolName != "execute" {
		t.Fatalf("content=%q tid=%q toolName=%q", content, tid, toolName)
	}
}

func TestRecvSchemaMessageStream_ContextCancel(t *testing.T) {
	sr, sw := schema.Pipe[*schema.Message](4)
	t.Cleanup(func() { sw.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	content, _, _, err := recvSchemaMessageStream(ctx, sr)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v content=%q", err, content)
	}
}

func TestRecvSchemaMessageStream_RecvError(t *testing.T) {
	sr, sw := schema.Pipe[*schema.Message](4)
	want := errors.New("stream broken")
	_ = sw.Send(nil, want)
	sw.Close()

	_, _, _, err := recvSchemaMessageStream(context.Background(), sr)
	if !errors.Is(err, want) {
		t.Fatalf("want %v, got %v", want, err)
	}
}

func TestRecvSchemaMessageStream_NilStream(t *testing.T) {
	content, tid, toolName, err := recvSchemaMessageStream(context.Background(), nil)
	if err != nil || content != "" || tid != "" || toolName != "" {
		t.Fatalf("nil stream: content=%q tid=%q toolName=%q err=%v", content, tid, toolName, err)
	}
}

func TestRecvSchemaMessageStream_EOFViaEmptyRead(t *testing.T) {
	sr, sw := schema.Pipe[*schema.Message](4)
	_ = sw.Send(nil, io.EOF)
	sw.Close()

	_, _, _, err := recvSchemaMessageStream(context.Background(), sr)
	if err != nil {
		t.Fatalf("EOF should not surface as error, got %v", err)
	}
}

func TestRecvEinoSchemaMessageStreamWithContext_SkipsNilChunks(t *testing.T) {
	sr, sw := schema.Pipe[*schema.Message](4)
	_ = sw.Send(nil, nil)
	_ = sw.Send(schema.AssistantMessage("hello", nil), nil)
	sw.Close()

	var got []string
	err := recvEinoSchemaMessageStreamWithContext(context.Background(), sr, 1, func(chunk *schema.Message) {
		got = append(got, chunk.Content)
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(got) != 1 || got[0] != "hello" {
		t.Fatalf("chunks = %#v, want [hello]", got)
	}
}

func TestRecvEinoSchemaMessageStreamWithContext_NilStream(t *testing.T) {
	called := false
	err := recvEinoSchemaMessageStreamWithContext(context.Background(), nil, 0, func(*schema.Message) {
		called = true
	})
	if err != nil {
		t.Fatalf("nil stream should not error, got %v", err)
	}
	if called {
		t.Fatal("nil stream should not call handler")
	}
}
