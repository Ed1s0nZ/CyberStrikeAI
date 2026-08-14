package multiagent

import (
	"testing"

	"cyberstrike-ai/internal/config"
)

func TestAgentMaxIterations(t *testing.T) {
	if got := agentMaxIterations(nil); got != defaultAgentMaxIterations {
		t.Fatalf("nil cfg: got %d want %d", got, defaultAgentMaxIterations)
	}
	cfg := &config.Config{Agent: config.AgentConfig{MaxIterations: 12000}}
	if got := agentMaxIterations(cfg); got != 12000 {
		t.Fatalf("got %d want 12000", got)
	}
	cfg.Agent.MaxIterations = 0
	if got := agentMaxIterations(cfg); got != defaultAgentMaxIterations {
		t.Fatalf("zero: got %d want %d", got, defaultAgentMaxIterations)
	}
}

func TestResolveMaxIterations(t *testing.T) {
	cfg := &config.Config{Agent: config.AgentConfig{MaxIterations: 12000}}
	if got := resolveMaxIterations(cfg, 0); got != 12000 {
		t.Fatalf("global: got %d want 12000", got)
	}
	if got := resolveMaxIterations(cfg, 50); got != 50 {
		t.Fatalf("override: got %d want 50", got)
	}
}
