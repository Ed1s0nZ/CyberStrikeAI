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

	content, tid, err := recvSchemaMessageStream(context.Background(), sr)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if content != "hello" {
		t.Fatalf("content=%q want hello", content)
	}
	if tid != "tc-1" {
		t.Fatalf("toolCallID=%q want tc-1", tid)
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

	content, _, err := recvSchemaMessageStream(ctx, sr)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v content=%q", err, content)
	}
}

func TestRecvSchemaMessageStream_RecvError(t *testing.T) {
	sr, sw := schema.Pipe[*schema.Message](4)
	want := errors.New("stream broken")
	_ = sw.Send(nil, want)
	sw.Close()

	_, _, err := recvSchemaMessageStream(context.Background(), sr)
	if !errors.Is(err, want) {
		t.Fatalf("want %v, got %v", want, err)
	}
}

func TestRecvSchemaMessageStream_NilStream(t *testing.T) {
	content, tid, err := recvSchemaMessageStream(context.Background(), nil)
	if err != nil || content != "" || tid != "" {
		t.Fatalf("nil stream: content=%q tid=%q err=%v", content, tid, err)
	}
}

func TestRecvSchemaMessageStream_EOFViaEmptyRead(t *testing.T) {
	sr, sw := schema.Pipe[*schema.Message](4)
	_ = sw.Send(nil, io.EOF)
	sw.Close()

	_, _, err := recvSchemaMessageStream(context.Background(), sr)
	if err != nil {
		t.Fatalf("EOF should not surface as error, got %v", err)
	}
}
