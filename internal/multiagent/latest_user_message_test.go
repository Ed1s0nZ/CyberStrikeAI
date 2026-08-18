package multiagent

import (
	"os"
	"strings"
	"testing"

	"cyberstrike-ai/internal/config"
)

func TestPrepareLatestUserMessageForModel_CapsAndPersistsOversizedInput(t *testing.T) {
	dir := t.TempDir()
	appCfg := &config.Config{
		Database: config.DatabaseConfig{Path: dir + "/test.db"},
	}
	mw := &config.MultiAgentEinoMiddlewareConfig{
		LatestUserMessageMaxRunes:  10,
		LatestUserMessageHeadRunes: 4,
		LatestUserMessageTailRunes: 4,
	}
	input := "abcdefghijklmnopqrst"

	out := prepareLatestUserMessageForModel(input, appCfg, mw, "conv-1", nil)
	if out == input {
		t.Fatal("expected oversized input to be replaced with preview")
	}
	if !strings.Contains(out, "artifact_path:") {
		t.Fatalf("expected artifact path in preview: %q", out)
	}
	if !strings.Contains(out, "abcd") || !strings.Contains(out, "qrst") {
		t.Fatalf("expected head and tail preview: %q", out)
	}
	if strings.Contains(out, "efghijklmnop") {
		t.Fatalf("middle content should not remain in model preview: %q", out)
	}

	path := extractArtifactPathForTest(out)
	if path == "" {
		t.Fatalf("could not extract artifact path: %q", out)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	if string(body) != input {
		t.Fatalf("artifact body mismatch: %q", body)
	}
}

func TestPrepareLatestUserMessageForModel_ShortInputUnchanged(t *testing.T) {
	mw := &config.MultiAgentEinoMiddlewareConfig{LatestUserMessageMaxRunes: 100}
	input := "short"
	out := prepareLatestUserMessageForModel(input, nil, mw, "conv-1", nil)
	if out != input {
		t.Fatalf("short input should be unchanged: %q", out)
	}
}

func extractArtifactPathForTest(s string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "artifact_path:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "artifact_path:"))
		}
	}
	return ""
}
