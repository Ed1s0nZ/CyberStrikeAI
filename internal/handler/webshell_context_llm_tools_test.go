package handler

import (
	"strings"
	"testing"

	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/mcp/builtin"
)

func TestWebshellAssistantContextListsLLMCallableManagementAndFileTools(t *testing.T) {
	conn := &database.WebShellConnection{ID: "ws_ctx", URL: "http://127.0.0.1/shell.php", Type: "php", OS: "linux", Encoding: "auto"}
	ctx := BuildWebshellAssistantContext(conn, "", "test")
	for _, name := range []string{
		builtin.ToolWebshellExec,
		builtin.ToolWebshellFileList,
		builtin.ToolWebshellFileRead,
		builtin.ToolWebshellFileWrite,
		builtin.ToolWebshellFileOp,
		builtin.ToolManageWebshellList,
		builtin.ToolManageWebshellAdd,
		builtin.ToolManageWebshellUpdate,
		builtin.ToolManageWebshellDelete,
		builtin.ToolManageWebshellTest,
	} {
		if !strings.Contains(ctx, name) {
			t.Fatalf("context missing tool %s:\n%s", name, ctx)
		}
	}
}
