package multiagent

import (
	"context"
	"fmt"
	"testing"

	"cyberstrike-ai/internal/config"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
)

type stubChatModelAgentMiddleware struct {
	adk.BaseChatModelAgentMiddleware
	tag string
}

func stubMW(tag string) adk.ChatModelAgentMiddleware {
	return &stubChatModelAgentMiddleware{tag: tag}
}

func TestBuildPlanExecuteExecutorHandlers_IncludesExecPreMiddlewares(t *testing.T) {
	t.Parallel()
	pre := []adk.ChatModelAgentMiddleware{
		stubMW("patch"),
		stubMW("reduction"),
	}

	got, err := buildPlanExecuteExecutorHandlers(context.Background(), &PlanExecuteRootArgs{
		ExecPreMiddlewares:   pre,
		FilesystemMiddleware: stubMW("filesystem"),
		SkillMiddleware:      stubMW("skill"),
	})
	if err != nil {
		t.Fatalf("buildPlanExecuteExecutorHandlers: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("expected 4 pre-tail handlers (2 pre + fs + skill), got %d", len(got))
	}
	for i, want := range []string{"patch", "reduction", "filesystem", "skill"} {
		st, ok := got[i].(*stubChatModelAgentMiddleware)
		if !ok || st.tag != want {
			t.Fatalf("handler[%d]: got %#v want tag %q", i, got[i], want)
		}
	}
}

func stubTools(n int) []tool.BaseTool {
	out := make([]tool.BaseTool, n)
	for i := 0; i < n; i++ {
		out[i] = stubTool{name: fmt.Sprintf("t%d", i)}
	}
	return out
}

func TestBuildPlanExecuteExecutorHandlers_NilArgs(t *testing.T) {
	t.Parallel()
	if _, err := buildPlanExecuteExecutorHandlers(context.Background(), nil); err == nil {
		t.Fatal("expected error for nil args")
	}
}

func TestPrependEinoMiddlewares_Main_IncludesPatch(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mw := configMultiAgentEinoMiddlewareForTest()
	mw.ReductionEnable = false
	mw.ToolSearchEnable = false
	mw.PlantaskEnable = false
	_, extra, _, _, err := prependEinoMiddlewares(ctx, mw, einoMWMain, stubTools(25), nil, "", "conv-test", "", nil)
	if err != nil {
		t.Fatalf("prependEinoMiddlewares: %v", err)
	}
	if len(extra) == 0 {
		t.Fatal("expected patch middleware on einoMWMain when patch_tool_calls enabled")
	}
}

func configMultiAgentEinoMiddlewareForTest() *config.MultiAgentEinoMiddlewareConfig {
	patch := true
	return &config.MultiAgentEinoMiddlewareConfig{
		PatchToolCalls: &patch,
	}
}
