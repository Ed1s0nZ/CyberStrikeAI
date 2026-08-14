package mcp

import "cyberstrike-ai/internal/tooloutput"

const DefaultToolResultMaxBytes = 12000

// ToolResultSpillConfig controls where oversized tool results are written on disk
// before the in-memory/DB/agent-facing payload is truncated.
type ToolResultSpillConfig struct {
	RootDir        string
	ProjectID      string
	ConversationID string
	ExecutionID    string
}

// NormalizeToolResultForStorage returns the canonical result used by both the
// agent-facing response and monitor persistence. When maxBytes is exceeded the
// full text is spilled under the reduction cache tree and replaced with a
// <persisted-output> notice that includes the file path.
func NormalizeToolResultForStorage(result *ToolResult, maxBytes int) *ToolResult {
	return NormalizeToolResultForStorageWithSpill(result, maxBytes, ToolResultSpillConfig{})
}

// NormalizeToolResultForStorageWithSpill is NormalizeToolResultForStorage with
// an explicit spill location (conversation/execution scoped).
func NormalizeToolResultForStorageWithSpill(result *ToolResult, maxBytes int, spill ToolResultSpillConfig) *ToolResult {
	if result == nil {
		return nil
	}
	out := cloneToolResult(result)
	if maxBytes <= 0 {
		return out
	}

	total := 0
	for _, c := range out.Content {
		if c.Type == "text" {
			total += len(c.Text)
		}
	}
	if total <= maxBytes {
		return out
	}

	full := ToolResultPlainText(out)
	bound := tooloutput.BoundWithSpill(full, maxBytes, tooloutput.SpillOpts{
		RootDir:        spill.RootDir,
		ProjectID:      spill.ProjectID,
		ConversationID: spill.ConversationID,
		ExecutionID:    spill.ExecutionID,
	})
	out.Content = []Content{{Type: "text", Text: bound}}
	return out
}

func cloneToolResult(in *ToolResult) *ToolResult {
	if in == nil {
		return nil
	}
	out := *in
	if in.Content != nil {
		out.Content = append([]Content(nil), in.Content...)
	}
	return &out
}
