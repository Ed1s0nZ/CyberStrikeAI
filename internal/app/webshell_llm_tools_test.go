package app

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/handler"
	"cyberstrike-ai/internal/mcp"
	"cyberstrike-ai/internal/mcp/builtin"

	"go.uber.org/zap"
)

func TestWebshellLLMToolsExposeManagementAndFullFileOps(t *testing.T) {
	db, err := database.NewDB(filepath.Join(t.TempDir(), "test.db"), zap.NewNop())
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()

	h := handler.NewWebShellHandler(zap.NewNop(), db)
	srv := mcp.NewServer(zap.NewNop())
	registerWebshellTools(srv, db, h, zap.NewNop())
	registerWebshellManagementTools(srv, db, h, zap.NewNop())

	tools := srv.GetAllTools()
	byName := map[string]mcp.Tool{}
	for _, tool := range tools {
		byName[tool.Name] = tool
	}
	required := []string{
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
	}
	for _, name := range required {
		if _, ok := byName[name]; !ok {
			t.Fatalf("tool %s was not registered; got %#v", name, namesOf(tools))
		}
	}

	fileOp := byName[builtin.ToolWebshellFileOp]
	props, ok := fileOp.InputSchema["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("webshell_file_op properties missing: %#v", fileOp.InputSchema)
	}
	for _, prop := range []string{"connection_id", "action", "path", "content", "target_path", "chunk_index"} {
		if _, ok := props[prop]; !ok {
			t.Fatalf("webshell_file_op missing property %s", prop)
		}
	}
	action, ok := props["action"].(map[string]interface{})
	if !ok {
		t.Fatalf("webshell_file_op action schema invalid: %#v", props["action"])
	}
	enum, ok := action["enum"].([]string)
	if !ok {
		t.Fatalf("webshell_file_op action enum invalid: %#v", action["enum"])
	}
	for _, action := range []string{"list", "read", "delete", "write", "mkdir", "rename", "upload", "upload_chunk"} {
		if !containsString(enum, action) {
			t.Fatalf("webshell_file_op action enum missing %s: %#v", action, enum)
		}
	}

	add := byName[builtin.ToolManageWebshellAdd]
	addProps := mustProps(t, add)
	for _, prop := range []string{"url", "password", "type", "method", "cmd_param", "protocol", "user_agent", "remark"} {
		if _, ok := addProps[prop]; !ok {
			t.Fatalf("manage_webshell_add missing property %s", prop)
		}
	}
}

func TestWebshellLLMToolCallsCoverManagementAndFileOps(t *testing.T) {
	if os.Getenv("CSA_WEBSHELL_LAB") != "1" {
		t.Skip("requires CSA_WEBSHELL_LAB=1 and local WebShell lab services")
	}

	db, err := database.NewDB(filepath.Join(t.TempDir(), "test.db"), zap.NewNop())
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()

	h := handler.NewWebShellHandler(zap.NewNop(), db)
	srv := mcp.NewServer(zap.NewNop())
	registerWebshellTools(srv, db, h, zap.NewNop())
	registerWebshellManagementTools(srv, db, h, zap.NewNop())

	ctx := context.Background()
	addRes := callTool(t, srv, ctx, builtin.ToolManageWebshellAdd, map[string]interface{}{"url": "http://127.0.0.1:9101/classic.php", "password": "rebeyond", "type": "php", "method": "post", "cmd_param": "cmd", "protocol": "classic", "user_agent": "User-Agent: CyberStrikeAI-LLM-Test/1.0", "remark": "llm tool integration"})
	connID := extractToolField(t, toolText(addRes), "连接ID:")
	defer db.DeleteWebshellConnection(connID)

	updateRes := callTool(t, srv, ctx, builtin.ToolManageWebshellUpdate, map[string]interface{}{"connection_id": connID, "url": "http://127.0.0.1:9101/classic.php", "password": "rebeyond", "type": "php", "method": "post", "cmd_param": "cmd", "protocol": "classic", "user_agent": "User-Agent: CyberStrikeAI-LLM-Test-Updated/1.0", "remark": "llm tool integration updated"})
	if !strings.Contains(toolText(updateRes), "llm tool integration updated") {
		t.Fatalf("manage update output mismatch: %s", toolText(updateRes))
	}

	if res := callTool(t, srv, ctx, builtin.ToolManageWebshellList, map[string]interface{}{}); !strings.Contains(toolText(res), connID) {
		t.Fatalf("manage list missing connection: %s", toolText(res))
	}
	if res := callTool(t, srv, ctx, builtin.ToolManageWebshellTest, map[string]interface{}{"connection_id": connID, "command": "printf CSA_LLM_TOOL"}); !strings.Contains(toolText(res), "CSA_LLM_TOOL") {
		t.Fatalf("manage test output mismatch: %s", toolText(res))
	}
	if res := callTool(t, srv, ctx, builtin.ToolWebshellExec, map[string]interface{}{"connection_id": connID, "command": "printf CSA_EXEC_TOOL"}); !strings.Contains(toolText(res), "CSA_EXEC_TOOL") {
		t.Fatalf("exec output mismatch: %s", toolText(res))
	}

	conn, err := db.GetWebshellConnection(connID)
	if err != nil || conn == nil {
		t.Fatalf("GetWebshellConnection %s: conn=%v err=%v", connID, conn, err)
	}

	base := "llm_tool_" + strings.ReplaceAll(t.Name(), "/", "_")
	defer h.ExecWithConnection(conn, "rm -rf "+base+" "+base+"_renamed")

	callTool(t, srv, ctx, builtin.ToolWebshellFileOp, map[string]interface{}{"connection_id": connID, "action": "mkdir", "path": base})
	callTool(t, srv, ctx, builtin.ToolWebshellFileOp, map[string]interface{}{"connection_id": connID, "action": "rename", "path": base, "target_path": base + "_renamed"})
	callTool(t, srv, ctx, builtin.ToolWebshellFileWrite, map[string]interface{}{"connection_id": connID, "path": base + "_renamed/write.txt", "content": "CSA_WRITE_TOOL"})
	if res := callTool(t, srv, ctx, builtin.ToolWebshellFileRead, map[string]interface{}{"connection_id": connID, "path": base + "_renamed/write.txt"}); !strings.Contains(toolText(res), "CSA_WRITE_TOOL") {
		t.Fatalf("read output mismatch: %s", toolText(res))
	}

	uploadContent := []byte("CSA_UPLOAD_TOOL\x00\x01")
	callTool(t, srv, ctx, builtin.ToolWebshellFileOp, map[string]interface{}{"connection_id": connID, "action": "upload", "path": base + "_renamed/upload.bin", "content": base64.StdEncoding.EncodeToString(uploadContent)})
	if res := callTool(t, srv, ctx, builtin.ToolWebshellFileOp, map[string]interface{}{"connection_id": connID, "action": "read", "path": base + "_renamed/upload.bin"}); !strings.Contains(toolText(res), "CSA_UPLOAD_TOOL") {
		t.Fatalf("upload/read output mismatch: %s", toolText(res))
	}

	large := make([]byte, 128*1024)
	for i := range large {
		large[i] = byte((i*17 + 9) % 251)
	}
	const chunkSize = 32000
	for idx, off := 0, 0; off < len(large); idx, off = idx+1, off+chunkSize {
		end := off + chunkSize
		if end > len(large) {
			end = len(large)
		}
		callTool(t, srv, ctx, builtin.ToolWebshellFileOp, map[string]interface{}{"connection_id": connID, "action": "upload_chunk", "path": base + "_renamed/large.bin", "content": base64.StdEncoding.EncodeToString(large[off:end]), "chunk_index": idx})
	}
	out, ok, errMsg := h.ExecWithConnection(conn, "sha256sum "+base+"_renamed/large.bin")
	if errMsg != "" || !ok {
		t.Fatalf("sha256 command failed ok=%v err=%s out=%s", ok, errMsg, out)
	}
	sum := sha256.Sum256(large)
	if expected := fmt.Sprintf("%x", sum[:]); !strings.Contains(out, expected) {
		t.Fatalf("chunked upload sha256 mismatch: want %s, got %s", expected, out)
	}

	callTool(t, srv, ctx, builtin.ToolWebshellFileOp, map[string]interface{}{"connection_id": connID, "action": "delete", "path": base + "_renamed/write.txt"})
	if res := callTool(t, srv, ctx, builtin.ToolWebshellFileList, map[string]interface{}{"connection_id": connID, "path": base + "_renamed"}); strings.Contains(toolText(res), "write.txt") {
		t.Fatalf("delete did not remove write.txt: %s", toolText(res))
	}

	callTool(t, srv, ctx, builtin.ToolManageWebshellDelete, map[string]interface{}{"connection_id": connID})
	if res := callTool(t, srv, ctx, builtin.ToolManageWebshellList, map[string]interface{}{}); strings.Contains(toolText(res), connID) {
		t.Fatalf("manage delete did not remove connection: %s", toolText(res))
	}
}

func namesOf(tools []mcp.Tool) []string {
	out := make([]string, 0, len(tools))
	for _, tool := range tools {
		out = append(out, tool.Name)
	}
	return out
}
func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
func mustProps(t *testing.T, tool mcp.Tool) map[string]interface{} {
	t.Helper()
	props, ok := tool.InputSchema["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("%s properties missing: %#v", tool.Name, tool.InputSchema)
	}
	return props
}
func callTool(t *testing.T, srv *mcp.Server, ctx context.Context, name string, args map[string]interface{}) *mcp.ToolResult {
	t.Helper()
	res, _, err := srv.CallTool(ctx, name, args)
	if err != nil {
		t.Fatalf("CallTool %s: %v", name, err)
	}
	if res == nil {
		t.Fatalf("CallTool %s returned nil", name)
	}
	if res.IsError {
		t.Fatalf("CallTool %s returned tool error: %s", name, toolText(res))
	}
	return res
}
func toolText(res *mcp.ToolResult) string {
	if res == nil {
		return ""
	}
	var b strings.Builder
	for _, c := range res.Content {
		if c.Type == "text" {
			b.WriteString(c.Text)
		}
	}
	return b.String()
}
func extractToolField(t *testing.T, text, prefix string) string {
	t.Helper()
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	t.Fatalf("tool output missing %s: %s", prefix, text)
	return ""
}
