package multiagent

import (
	"strings"
	"testing"

	"cyberstrike-ai/internal/agent"

	"github.com/cloudwego/eino/schema"
)

func TestBuildEinoRunResultNeverPersistsRawAccumulationWithoutModelFacingTrace(t *testing.T) {
	raw := []schema.Message{*schema.ToolMessage(strings.Repeat("raw-tool-output", 1000), "call-1")}
	rawMsgs := make([]*schema.Message, len(raw))
	for i := range raw {
		rawMsgs[i] = &raw[i]
	}
	result := buildEinoRunResultFromAccumulated("deep", rawMsgs, nil, "", "", "empty", nil, true)
	if result.LastAgentTraceInput != "" {
		t.Fatalf("pre-model raw accumulation must not be persisted: %d bytes", len(result.LastAgentTraceInput))
	}

	modelFacing := []*schema.Message{schema.UserMessage("bounded-model-view")}
	result = buildEinoRunResultFromAccumulated("deep", rawMsgs, modelFacing, "ok", "", "empty", nil, false)
	if !strings.Contains(result.LastAgentTraceInput, "bounded-model-view") {
		t.Fatalf("model-facing trace missing: %s", result.LastAgentTraceInput)
	}
	if strings.Contains(result.LastAgentTraceInput, "raw-tool-output") {
		t.Fatal("raw accumulation leaked into persisted model-facing trace")
	}
	if !agent.IsModelFacingTraceJSON(result.LastAgentTraceInput) {
		t.Fatal("persisted model-facing trace is missing its version marker")
	}
}
