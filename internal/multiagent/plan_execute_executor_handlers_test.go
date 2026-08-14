package multiagent

import (
	"context"
	"fmt"
	"testing"

	"cyberstrike-ai/internal/config"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

type stubAgenticChatModelAgentMiddleware struct {
	*adk.TypedBaseChatModelAgentMiddleware[*schema.AgenticMessage]
	tag string
}

func stubAgenticMW(tag string) adk.TypedChatModelAgentMiddleware[*schema.AgenticMessage] {
	return &stubAgenticChatModelAgentMiddleware{
		TypedBaseChatModelAgentMiddleware: &adk.TypedBaseChatModelAgentMiddleware[*schema.AgenticMessage]{},
		tag:                               tag,
	}
}

func TestBuildPlanExecuteAgenticExecutorHandlers_IncludesExecPreMiddlewares(t *testing.T) {
	t.Parallel()
	pre := []adk.TypedChatModelAgentMiddleware[*schema.AgenticMessage]{
		stubAgenticMW("patch"),
		stubAgenticMW("reduction"),
	}

	got, err := buildPlanExecuteAgenticExecutorHandlers(context.Background(), &PlanExecuteRootArgs{
		AgenticExecPreMiddlewares:   pre,
		AgenticFilesystemMiddleware: stubAgenticMW("filesystem"),
		AgenticSkillMiddleware:      stubAgenticMW("skill"),
	})
	if err != nil {
		t.Fatalf("buildPlanExecuteAgenticExecutorHandlers: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("expected 4 pre-tail handlers (2 pre + fs + skill), got %d", len(got))
	}
	for i, want := range []string{"patch", "reduction", "filesystem", "skill"} {
		st, ok := got[i].(*stubAgenticChatModelAgentMiddleware)
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

func TestBuildPlanExecuteAgenticExecutorHandlers_NilArgs(t *testing.T) {
	t.Parallel()
	if _, err := buildPlanExecuteAgenticExecutorHandlers(context.Background(), nil); err == nil {
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
	_, extra, _, err := prependEinoMiddlewares(ctx, mw, einoMWMain, stubTools(25), nil, "", "conv-test", "", nil)
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
