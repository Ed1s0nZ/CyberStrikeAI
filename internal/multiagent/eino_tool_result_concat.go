package multiagent

import (
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"
)

// concatToolResultChunks 按 Eino 原生语义合并工具结果流：
//   - 同一 CallID（EventSender 一 call 一 event）：schema.ConcatMessages
//   - 并行工具被摊进同一条流（ToolsNode MergeStreamReaders 扁平化后）：
//     按 CallID 分列后再 ConcatMessages，等价于 schema.ConcatMessageArray
func concatToolResultChunks(chunks []*schema.Message) ([]*schema.Message, error) {
	if len(chunks) == 0 {
		return nil, nil
	}
	if toolResultChunksShareCallID(chunks) {
		merged, err := schema.ConcatMessages(chunks)
		if err != nil {
			return nil, err
		}
		return []*schema.Message{merged}, nil
	}
	return concatToolResultChunksByCallID(chunks)
}

func toolResultChunksShareCallID(chunks []*schema.Message) bool {
	id := ""
	for _, chunk := range chunks {
		if chunk == nil {
			continue
		}
		got := strings.TrimSpace(chunk.ToolCallID)
		if got == "" {
			continue
		}
		if id == "" {
			id = got
			continue
		}
		if got != id {
			return false
		}
	}
	return true
}

func concatToolResultChunksByCallID(chunks []*schema.Message) ([]*schema.Message, error) {
	type column struct {
		key    string
		chunks []*schema.Message
	}
	var ordered []column
	index := make(map[string]int)
	lastKey := ""
	anon := 0
	for _, chunk := range chunks {
		if chunk == nil {
			continue
		}
		key := strings.TrimSpace(chunk.ToolCallID)
		if key == "" {
			if lastKey != "" {
				key = lastKey
			} else {
				key = fmt.Sprintf("\x00anon-%d", anon)
				anon++
			}
		}
		if idx, ok := index[key]; ok {
			ordered[idx].chunks = append(ordered[idx].chunks, chunk)
		} else {
			index[key] = len(ordered)
			ordered = append(ordered, column{key: key, chunks: []*schema.Message{chunk}})
		}
		lastKey = key
	}
	out := make([]*schema.Message, 0, len(ordered))
	for _, col := range ordered {
		merged, err := schema.ConcatMessages(col.chunks)
		if err != nil {
			return nil, err
		}
		if strings.HasPrefix(col.key, "\x00anon-") {
			merged.ToolCallID = ""
		}
		out = append(out, merged)
	}
	return out, nil
}
