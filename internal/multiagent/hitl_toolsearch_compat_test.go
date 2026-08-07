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

func TestMergeHitlExemptMetaTools_includesBuiltInExemptions(t *testing.T) {
	merged := MergeHitlExemptMetaTools([]string{"read_file"})
	foundToolSearch := false
	foundWriteFile := false
	for _, name := range merged {
		if IsToolSearchTool(name) {
			foundToolSearch = true
		}
		if strings.EqualFold(strings.TrimSpace(name), "write_file") {
			foundWriteFile = true
		}
	}
	if !foundToolSearch {
		t.Fatalf("tool_search missing from %v", merged)
	}
	if !foundWriteFile {
		t.Fatalf("write_file missing from %v", merged)
	}
	foundProjectFact := false
	for _, name := range merged {
		if strings.EqualFold(strings.TrimSpace(name), "upsert_project_fact") {
			foundProjectFact = true
			break
		}
	}
	if !foundProjectFact {
		t.Fatalf("upsert_project_fact missing from %v", merged)
	}
}
