package multiagent

import (
	"context"
	"errors"
	"io"

	"github.com/cloudwego/eino/schema"
)

// recvEinoSchemaMessageStreamWithContext consumes an Eino schema.Message stream
// and stops promptly when ctx is canceled. EOF and nil chunks are treated as a
// normal stream boundary.
func recvEinoSchemaMessageStreamWithContext(
	ctx context.Context,
	stream *schema.StreamReader[*schema.Message],
	buffer int,
	onChunk func(*schema.Message),
) error {
	if stream == nil {
		return nil
	}
	if buffer <= 0 {
		buffer = 1
	}
	type streamMsg struct {
		chunk *schema.Message
		err   error
	}
	recvCh := make(chan streamMsg, buffer)
	go func() {
		defer close(recvCh)
		for {
			ch, rerr := stream.Recv()
			recvCh <- streamMsg{chunk: ch, err: rerr}
			if rerr != nil {
				return
			}
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case sm, ok := <-recvCh:
			if !ok {
				return nil
			}
			if errors.Is(sm.err, io.EOF) {
				return nil
			}
			if sm.err != nil {
				return sm.err
			}
			if sm.chunk == nil || onChunk == nil {
				continue
			}
			onChunk(sm.chunk)
		}
	}
}
