package openai

import "testing"

func TestDisplayReasoningContentStripsLegacyClaudeSuffix(t *testing.T) {
	raw := "hello" + claudeReasoningRoundTripSep + `[{"type":"thinking"}]`
	if got := DisplayReasoningContent(raw); got != "hello" {
		t.Fatalf("DisplayReasoningContent() = %q, want hello", got)
	}
}
