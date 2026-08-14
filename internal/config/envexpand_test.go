package config

import (
	"os"
	"testing"
)

func TestExpandEnvVar(t *testing.T) {
	os.Setenv("TEST_MCP_VAR", "hello")
	os.Setenv("TEST_MCP_PATH", "/usr/local/bin")
	defer os.Unsetenv("TEST_MCP_VAR")
	defer os.Unsetenv("TEST_MCP_PATH")

	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{"plain string", "no vars here", "no vars here"},
		{"empty string", "", ""},
		{"simple var", "${TEST_MCP_VAR}", "hello"},
		{"var in middle", "prefix-${TEST_MCP_VAR}-suffix", "prefix-hello-suffix"},
		{"multiple vars", "${TEST_MCP_PATH}/${TEST_MCP_VAR}", "/usr/local/bin/hello"},
		{"missing var empty", "${NONEXISTENT_MCP_VAR_XYZ}", ""},
		{"default value used", "${NONEXISTENT_MCP_VAR_XYZ:-fallback}", "fallback"},
		{"default not used", "${TEST_MCP_VAR:-unused}", "hello"},
		{"default with path", "${NONEXISTENT_MCP_VAR_XYZ:-/tmp/default}", "/tmp/default"},
		{"unclosed brace", "${UNCLOSED", "${UNCLOSED"},
		{"dollar without brace", "$PLAIN", "$PLAIN"},
		{"empty var name", "${}", ""},
		{"default empty var", "${:-default}", "default"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandEnvVar(tt.input)
			if got != tt.expect {
				t.Errorf("expandEnvVar(%q) = %q, want %q", tt.input, got, tt.expect)
			}
		})
	}
}

func TestExpandConfigEnv(t *testing.T) {
	os.Setenv("TEST_MCP_CMD", "python3")
	os.Setenv("TEST_MCP_TOKEN", "secret123")
	defer os.Unsetenv("TEST_MCP_CMD")
	defer os.Unsetenv("TEST_MCP_TOKEN")

	cfg := &ExternalMCPServerConfig{
		Command: "${TEST_MCP_CMD}",
		Args:    []string{"--token", "${TEST_MCP_TOKEN}", "${MISSING:-default_arg}"},
		Env:     map[string]string{"API_KEY": "${TEST_MCP_TOKEN}", "LEVEL": "${MISSING:-INFO}"},
		URL:     "https://${MISSING:-example.com}/mcp",
		Headers: map[string]string{"Authorization": "Bearer ${TEST_MCP_TOKEN}"},
	}

	ExpandConfigEnv(cfg)

	if cfg.Command != "python3" {
		t.Errorf("Command = %q, want %q", cfg.Command, "python3")
	}
	if cfg.Args[1] != "secret123" {
		t.Errorf("Args[1] = %q, want %q", cfg.Args[1], "secret123")
	}
	if cfg.Args[2] != "default_arg" {
		t.Errorf("Args[2] = %q, want %q", cfg.Args[2], "default_arg")
	}
	if cfg.Env["API_KEY"] != "secret123" {
		t.Errorf("Env[API_KEY] = %q, want %q", cfg.Env["API_KEY"], "secret123")
	}
	if cfg.Env["LEVEL"] != "INFO" {
		t.Errorf("Env[LEVEL] = %q, want %q", cfg.Env["LEVEL"], "INFO")
	}
	if cfg.URL != "https://example.com/mcp" {
		t.Errorf("URL = %q, want %q", cfg.URL, "https://example.com/mcp")
	}
	if cfg.Headers["Authorization"] != "Bearer secret123" {
		t.Errorf("Headers[Authorization] = %q, want %q", cfg.Headers["Authorization"], "Bearer secret123")
	}
}
