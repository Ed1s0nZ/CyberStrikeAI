package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func dockerEntrypointScriptPath(t *testing.T) string {
	t.Helper()

	path := filepath.Clean(filepath.Join("..", "..", "scripts", "docker", "docker-entrypoint.sh"))
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("docker entrypoint script not found: %v", err)
	}
	return path
}

func TestDockerEntrypointInitializesRuntimeConfigFromTemplate(t *testing.T) {
	tmpDir := t.TempDir()
	templatePath := filepath.Join(tmpDir, "config.example.yaml")
	runtimeDir := filepath.Join(tmpDir, "runtime-config")
	runtimeConfigPath := filepath.Join(runtimeDir, "config.yaml")
	templateContent := []byte("auth:\n  password: \n")

	if err := os.WriteFile(templatePath, templateContent, 0644); err != nil {
		t.Fatalf("write template config: %v", err)
	}

	cmd := exec.Command("bash", dockerEntrypointScriptPath(t), "true")
	cmd.Env = append(os.Environ(),
		"CYBERSTRIKE_TEMPLATE_CONFIG="+templatePath,
		"CYBERSTRIKE_RUNTIME_CONFIG_DIR="+runtimeDir,
		"CYBERSTRIKE_RUNTIME_CONFIG_PATH="+runtimeConfigPath,
	)

	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("run docker entrypoint: %v\n%s", err, out)
	}

	got, err := os.ReadFile(runtimeConfigPath)
	if err != nil {
		t.Fatalf("read runtime config: %v", err)
	}
	if string(got) != string(templateContent) {
		t.Fatalf("runtime config mismatch\nwant:\n%s\ngot:\n%s", templateContent, got)
	}
}

func TestDockerEntrypointPreservesExistingRuntimeConfig(t *testing.T) {
	tmpDir := t.TempDir()
	templatePath := filepath.Join(tmpDir, "config.example.yaml")
	runtimeDir := filepath.Join(tmpDir, "runtime-config")
	runtimeConfigPath := filepath.Join(runtimeDir, "config.yaml")
	templateContent := []byte("auth:\n  password: \n")
	existingContent := []byte("auth:\n  password: existing-secret\n")

	if err := os.WriteFile(templatePath, templateContent, 0644); err != nil {
		t.Fatalf("write template config: %v", err)
	}
	if err := os.MkdirAll(runtimeDir, 0755); err != nil {
		t.Fatalf("mkdir runtime dir: %v", err)
	}
	if err := os.WriteFile(runtimeConfigPath, existingContent, 0644); err != nil {
		t.Fatalf("write runtime config: %v", err)
	}

	cmd := exec.Command("bash", dockerEntrypointScriptPath(t), "true")
	cmd.Env = append(os.Environ(),
		"CYBERSTRIKE_TEMPLATE_CONFIG="+templatePath,
		"CYBERSTRIKE_RUNTIME_CONFIG_DIR="+runtimeDir,
		"CYBERSTRIKE_RUNTIME_CONFIG_PATH="+runtimeConfigPath,
	)

	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("run docker entrypoint: %v\n%s", err, out)
	}

	got, err := os.ReadFile(runtimeConfigPath)
	if err != nil {
		t.Fatalf("read runtime config: %v", err)
	}
	if string(got) != string(existingContent) {
		t.Fatalf("runtime config should be preserved\nwant:\n%s\ngot:\n%s", existingContent, got)
	}
}
