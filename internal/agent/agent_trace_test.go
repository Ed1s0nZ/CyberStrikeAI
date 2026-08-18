package agent

import (
	"encoding/json"
	"testing"
)

func TestExtractLastUserTurnTraceJSON(t *testing.T) {
	raw := []map[string]interface{}{
		{"role": "user", "content": "old question"},
		{"role": "assistant", "content": "old answer"},
		{"role": "user", "content": "new target 1.1.1.1"},
		{"role": "assistant", "tool_calls": []interface{}{map[string]interface{}{
			"id": "c1", "type": "function",
			"function": map[string]interface{}{"name": "nmap", "arguments": "{}"},
		}}},
		{"role": "tool", "tool_call_id": "c1", "content": "open ports"},
	}
	b, _ := json.Marshal(raw)
	out := ExtractLastUserTurnTraceJSON(string(b))
	var trimmed []map[string]interface{}
	if err := json.Unmarshal([]byte(out), &trimmed); err != nil {
		t.Fatal(err)
	}
	if len(trimmed) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(trimmed))
	}
	if trimmed[0]["content"] != "new target 1.1.1.1" {
		t.Fatalf("unexpected first message: %v", trimmed[0])
	}
}

func TestExtractLastUserTurnMessagesSkipsSystem(t *testing.T) {
	msgs := []ChatMessage{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "q"},
		{Role: "assistant", Content: "a"},
	}
	out := ExtractLastUserTurnMessages(msgs)
	if len(out) != 2 {
		t.Fatalf("expected 2, got %d", len(out))
	}
	if out[0].Role != "user" {
		t.Fatal("expected user first")
	}
}

func TestMergeAssistantTraceOutput(t *testing.T) {
	msgs := []ChatMessage{
		{Role: "user", Content: "q"},
		{Role: "assistant", Content: "draft"},
	}
	out := MergeAssistantTraceOutput(msgs, "final summary")
	if out[len(out)-1].Content != "final summary" {
		t.Fatalf("expected merged output, got %q", out[len(out)-1].Content)
	}
}

func TestParseTraceMessagesMarksVersionedModelFacingTrace(t *testing.T) {
	raw := `[{"role":"system","content":"s","extra":{"cyberstrike_model_facing_trace_version":1}},{"role":"user","content":"u"},{"role":"tool","content":"exact","tool_call_id":"c1"}]`
	if !IsModelFacingTraceJSON(raw) {
		t.Fatal("versioned trace not detected")
	}
	msgs, err := ParseTraceMessages(raw)
	if err != nil {
		t.Fatal(err)
	}
	for i, msg := range msgs {
		if !msg.ModelFacingTrace {
			t.Fatalf("message %d missing model-facing marker", i)
		}
	}
	if IsModelFacingTraceJSON(`[{"role":"user","content":"legacy"}]`) {
		t.Fatal("legacy trace incorrectly marked model-facing")
	}
}
