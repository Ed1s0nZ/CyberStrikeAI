package handler

import (
	"encoding/json"
	"testing"
)

func TestMergeHitlPayloadExecutionResult(t *testing.T) {
	merged, err := mergeHitlPayloadExecutionResult(`{"userMessage":"hi","toolName":"nmap"}`, hitlExecutionResult{
		Success: true,
		Result:  "open ports: 80",
	})
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]interface{}
	if err := json.Unmarshal([]byte(merged), &root); err != nil {
		t.Fatal(err)
	}
	if root["userMessage"] != "hi" {
		t.Fatalf("userMessage lost: %v", root["userMessage"])
	}
	exec, ok := root["executionResult"].(map[string]interface{})
	if !ok || exec["success"] != true {
		t.Fatalf("executionResult missing: %v", root["executionResult"])
	}
}

func TestPopApprovedInterruptForTool(t *testing.T) {
	m := NewHITLManager(nil, nil)
	m.TrackApprovedHitlExecution("hitl_a", "conv1", "nmap", "tc1")
	m.TrackApprovedHitlExecution("hitl_b", "conv1", "exec", "")
	if id := m.popApprovedInterruptForTool("conv1", "tc1", "nmap"); id != "hitl_a" {
		t.Fatalf("tc1 match=%q", id)
	}
	if id := m.popApprovedInterruptForTool("conv1", "", "exec"); id != "hitl_b" {
		t.Fatalf("tool name match=%q", id)
	}
}
