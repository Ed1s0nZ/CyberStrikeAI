package multiagent

import (
	"context"
	"testing"

	"cyberstrike-ai/internal/config"
)

func TestPrepareEinoAgenticSkillsStillCreatesReductionBackendWhenSkillsDisabled(t *testing.T) {
	ma := &config.MultiAgentConfig{
		EinoSkills: config.MultiAgentEinoSkillsConfig{Disable: true},
		EinoMiddleware: config.MultiAgentEinoMiddlewareConfig{
			ReductionEnable: true,
		},
	}
	loc, skillMW, fsTools, skillsRoot, err := prepareEinoAgenticSkills(context.Background(), "", ma, nil)
	if err != nil {
		t.Fatal(err)
	}
	if loc == nil {
		t.Fatal("agentic reduction backend must exist even when Skills are disabled")
	}
	if skillMW != nil || fsTools || skillsRoot != "" {
		t.Fatalf("Agentic Skills unexpectedly enabled: mw=%v fs=%v root=%q", skillMW, fsTools, skillsRoot)
	}
}
