package multiagent

import (
	"context"
	"testing"

	"cyberstrike-ai/internal/authctx"
)

func TestLocalToolPermissionIsSeparateFromAgentExecution(t *testing.T) {
	agentOnly := authctx.WithPrincipal(context.Background(), authctx.NewPrincipal("robot:u1", "robot", "own", map[string]bool{"agent:execute": true}))
	if !localToolPermissionDenied(agentOnly, "execute") || !localToolPermissionDenied(agentOnly, "read_file") {
		t.Fatal("agent:execute alone authorized local privileged tools")
	}
	local := authctx.WithPrincipal(context.Background(), authctx.NewPrincipal("u1", "user", "assigned", map[string]bool{"agent:local-execute": true}))
	if localToolPermissionDenied(local, "execute") || localToolPermissionDenied(local, "write_file") {
		t.Fatal("agent:local-execute was not honored")
	}
	if localToolPermissionDenied(agentOnly, "record_vulnerability") {
		t.Fatal("non-local tool was incorrectly denied")
	}
}
