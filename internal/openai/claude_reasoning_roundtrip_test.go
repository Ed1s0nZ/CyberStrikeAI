package openai

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDisplayReasoningContent(t *testing.T) {
	raw := "hello" + claudeReasoningRoundTripSep + `[{"type":"thinking","thinking":"x","signature":"sig"}]`
	if d := DisplayReasoningContent(raw); d != "hello" {
		t.Fatalf("got %q", d)
	}
	if DisplayReasoningContent("plain") != "plain" {
		t.Fatal()
	}
}

func TestAppendParseClaudeReasoningRoundTrip(t *testing.T) {
	blocks := []claudeContentBlock{
		{Type: "thinking", Thinking: "a", Signature: "s1"},
		{Type: "thinking", Thinking: "b", Signature: "s2"},
	}
	s := appendClaudeReasoningRoundTrip("sum", blocks)
	if !strings.Contains(s, claudeReasoningRoundTripSep) {
		t.Fatal("missing sep")
	}
	display, back := parseClaudeReasoningAssistantBlocks(s)
	if display != "sum" || len(back) != 2 {
		t.Fatalf("display=%q len=%d", display, len(back))
	}
	if back[0].Signature != "s1" || back[1].Thinking != "b" {
		t.Fatalf("%+v", back)
	}
}

func TestConvertOpenAIToClaude_AssistantReasoningReplay(t *testing.T) {
	rc := appendClaudeReasoningRoundTrip("vis", []claudeContentBlock{
		{Type: "thinking", Thinking: "t1", Signature: "sig1"},
	})
	payload := map[string]interface{}{
		"model": "claude-3-5-sonnet-latest",
		"messages": []interface{}{
			map[string]interface{}{
				"role":              "assistant",
				"content":           "out",
				"reasoning_content": rc,
			},
		},
	}
	req, err := convertOpenAIToClaude(payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(req.Messages) != 1 {
		t.Fatalf("messages=%d", len(req.Messages))
	}
	blocks := req.Messages[0].Content.Blocks
	if len(blocks) < 2 {
		t.Fatalf("blocks=%d", len(blocks))
	}
	if blocks[0].Type != "thinking" || blocks[0].Signature != "sig1" {
		t.Fatalf("first block %+v", blocks[0])
	}
	foundText := false
	for _, b := range blocks {
		if b.Type == "text" && b.Text == "out" {
			foundText = true
		}
	}
	if !foundText {
		t.Fatalf("blocks=%+v", blocks)
	}
}

func TestConvertOpenAIToClaude_OutputConfigEffort(t *testing.T) {
	payload := map[string]interface{}{
		"model": "claude-opus-4-8",
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "hi"},
		},
		"thinking": map[string]interface{}{
			"type":    "adaptive",
			"display": "summarized",
		},
		"output_config": map[string]interface{}{
			"effort": "high",
		},
	}
	req, err := convertOpenAIToClaude(payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(req.Thinking) == 0 {
		t.Fatal("expected thinking")
	}
	if len(req.OutputConfig) == 0 {
		t.Fatal("expected output_config")
	}
	var oc map[string]interface{}
	if err := json.Unmarshal(req.OutputConfig, &oc); err != nil {
		t.Fatal(err)
	}
	if oc["effort"] != "high" {
		t.Fatalf("effort=%v", oc["effort"])
	}
}

func TestConvertOpenAIToClaudeAcceptsCompleteMultiToolBatch(t *testing.T) {
	payload := map[string]interface{}{
		"model": "claude-test",
		"messages": []interface{}{
			map[string]interface{}{
				"role": "assistant",
				"tool_calls": []interface{}{
					map[string]interface{}{"id": "c1", "function": map[string]interface{}{"name": "one", "arguments": "{}"}},
					map[string]interface{}{"id": "c2", "function": map[string]interface{}{"name": "two", "arguments": "{}"}},
				},
			},
			map[string]interface{}{"role": "tool", "tool_call_id": "c1", "content": "r1"},
			map[string]interface{}{"role": "tool", "tool_call_id": "c2", "content": "r2"},
			map[string]interface{}{"role": "assistant", "content": "done"},
		},
	}
	req, err := convertOpenAIToClaude(payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(req.Messages) != 3 || len(req.Messages[1].Content.Blocks) != 2 {
		t.Fatalf("unexpected converted messages: %+v", req.Messages)
	}
}

func TestConvertOpenAIToClaudeRejectsMissingToolResult(t *testing.T) {
	payload := map[string]interface{}{
		"model": "claude-test",
		"messages": []interface{}{
			map[string]interface{}{
				"role": "assistant",
				"tool_calls": []interface{}{
					map[string]interface{}{"id": "c1", "function": map[string]interface{}{"name": "one", "arguments": "{}"}},
					map[string]interface{}{"id": "c2", "function": map[string]interface{}{"name": "two", "arguments": "{}"}},
				},
			},
			map[string]interface{}{"role": "tool", "tool_call_id": "c1", "content": "r1"},
		},
	}
	_, err := convertOpenAIToClaude(payload)
	if err == nil || !strings.Contains(err.Error(), "missing tool_result") {
		t.Fatalf("expected missing tool_result error, got %v", err)
	}
}

func TestConvertOpenAIToClaudeRejectsOrphanToolResult(t *testing.T) {
	payload := map[string]interface{}{
		"model": "claude-test",
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "start"},
			map[string]interface{}{"role": "tool", "tool_call_id": "orphan", "content": "result"},
		},
	}
	_, err := convertOpenAIToClaude(payload)
	if err == nil || !strings.Contains(err.Error(), "orphan tool_result") {
		t.Fatalf("expected orphan tool_result error, got %v", err)
	}
}

func TestClaudeToOpenAIResponseJSON_Thinking(t *testing.T) {
	claudeBody := []byte(`{
		"id":"msg_1","type":"message","role":"assistant","model":"x","stop_reason":"end_turn",
		"content":[
			{"type":"thinking","thinking":"step","signature":"sigx"},
			{"type":"text","text":"hi"}
		]
	}`)
	oai, err := claudeToOpenAIResponseJSON(claudeBody)
	if err != nil {
		t.Fatal(err)
	}
	var wrap map[string]interface{}
	if err := json.Unmarshal(oai, &wrap); err != nil {
		t.Fatal(err)
	}
	choices := wrap["choices"].([]interface{})
	ch0 := choices[0].(map[string]interface{})
	msg := ch0["message"].(map[string]interface{})
	rc, _ := msg["reasoning_content"].(string)
	if !strings.Contains(rc, "step") || !strings.Contains(rc, claudeReasoningRoundTripSep) {
		t.Fatalf("reasoning_content=%q", rc)
	}
	if msg["content"] != "hi" {
		t.Fatal()
	}
}

func TestConvertOpenAIToClaudeMapsMaxCompletionTokens(t *testing.T) {
	req, err := convertOpenAIToClaude(map[string]interface{}{
		"model":                 "claude-test",
		"max_completion_tokens": float64(16384),
		"max_tokens":            float64(1024),
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "hello"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if req.MaxTokens != 16384 {
		t.Fatalf("max tokens=%d, want 16384", req.MaxTokens)
	}
}
