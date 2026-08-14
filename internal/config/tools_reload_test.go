package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReloadSecurityToolsFromDir(t *testing.T) {
	root := t.TempDir()
	toolsDir := filepath.Join(root, "tools")
	if err := os.MkdirAll(toolsDir, 0755); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(root, "config.yaml")
	if err := os.WriteFile(configPath, []byte(`security:
  tools_dir: tools
  tools:
    - name: inline-only
      command: inline-cmd
      enabled: true
      description: inline tool
`), 0644); err != nil {
		t.Fatal(err)
	}

	writeTool := func(name, command string) {
		t.Helper()
		content := "name: " + name + "\ncommand: " + command + "\nenabled: true\ndescription: test\n"
		if err := os.WriteFile(filepath.Join(toolsDir, name+".yaml"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	writeTool("alpha", "alpha-cmd")

	cfg := &Config{
		Security: SecurityConfig{
			ToolsDir: "tools",
			Tools: []ToolConfig{
				{Name: "stale", Command: "stale-cmd", Enabled: true, Description: "should be removed"},
			},
		},
	}

	if err := ReloadSecurityToolsFromDir(cfg, configPath); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(cfg.Security.Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(cfg.Security.Tools))
	}

	names := map[string]string{}
	for _, tool := range cfg.Security.Tools {
		names[tool.Name] = tool.Command
	}
	if names["alpha"] != "alpha-cmd" {
		t.Fatalf("alpha tool missing or wrong command: %#v", names)
	}
	if names["inline-only"] != "inline-cmd" {
		t.Fatalf("inline-only tool missing: %#v", names)
	}
	if _, ok := names["stale"]; ok {
		t.Fatal("stale in-memory tool should not survive reload")
	}

	writeTool("beta", "beta-cmd")
	if err := ReloadSecurityToolsFromDir(cfg, configPath); err != nil {
		t.Fatalf("second reload: %v", err)
	}
	if len(cfg.Security.Tools) != 3 {
		t.Fatalf("expected 3 tools after add, got %d", len(cfg.Security.Tools))
	}
	foundBeta := false
	for _, tool := range cfg.Security.Tools {
		if tool.Name == "beta" {
			foundBeta = true
			break
		}
	}
	if !foundBeta {
		t.Fatal("beta tool not found after second reload")
	}
}

func TestMergeToolsFromDir_DirOverridesInline(t *testing.T) {
	root := t.TempDir()
	toolsDir := filepath.Join(root, "tools")
	if err := os.MkdirAll(toolsDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := "name: shared\ncommand: dir-cmd\nenabled: true\ndescription: from dir\n"
	if err := os.WriteFile(filepath.Join(toolsDir, "shared.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	inline := []ToolConfig{
		{Name: "shared", Command: "inline-cmd", Enabled: true, Description: "from inline"},
	}
	merged, err := MergeToolsFromDir(toolsDir, inline)
	if err != nil {
		t.Fatal(err)
	}
	if len(merged) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(merged))
	}
	if merged[0].Command != "dir-cmd" {
		t.Fatalf("dir tool should win, got command %q", merged[0].Command)
	}
}
