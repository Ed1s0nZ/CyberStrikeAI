package multiagent

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestHitlRejectToolResult_toolSearchIsJSON(t *testing.T) {
	raw := HitlRejectToolResult("tool_search", "rejected by user: timeout")
	var payload toolSearchHitlRejectPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(payload.SelectedTools) != 0 {
		t.Fatalf("expected empty selectedTools, got %v", payload.SelectedTools)
	}
	if !payload.HitlRejected {
		t.Fatal("expected _hitlRejected true")
	}
	if !strings.Contains(payload.Reason, "timeout") {
		t.Fatalf("reason=%q", payload.Reason)
	}
}

func TestHitlRejectToolResult_otherToolKeepsLegacyText(t *testing.T) {
	raw := HitlRejectToolResult("nmap", "too risky")
	if strings.HasPrefix(raw, "{") {
		t.Fatalf("expected legacy text, got %q", raw)
	}
	if !strings.HasPrefix(raw, "[HITL Reject]") {
		t.Fatalf("expected [HITL Reject] prefix, got %q", raw)
	}
}

func TestMergeHitlExemptMetaTools_includesToolSearch(t *testing.T) {
	merged := MergeHitlExemptMetaTools([]string{"read_file"})
	found := false
	for _, name := range merged {
		if IsToolSearchTool(name) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("tool_search missing from %v", merged)
	}
	foundProjectFactTools := map[string]bool{
		"upsert_project_fact": false,
		"get_project_fact":    false,
	}
	for _, name := range merged {
		normalized := strings.ToLower(strings.TrimSpace(name))
		if _, ok := foundProjectFactTools[normalized]; ok {
			foundProjectFactTools[normalized] = true
		}
	}
	for name, found := range foundProjectFactTools {
		if !found {
			t.Fatalf("%s missing from %v", name, merged)
		}
	}
}
