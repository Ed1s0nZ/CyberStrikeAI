package config

import (
	"os"
	"strings"
)

// expandEnvVar expands ${VAR} and ${VAR:-default} environment variable references
// inside s. Syntax matches the official MCP config format (supported by Claude
// Desktop / Cursor / VS Code mcpServers).
func expandEnvVar(s string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		idx := strings.Index(s[i:], "${")
		if idx < 0 {
			b.WriteString(s[i:])
			break
		}
		b.WriteString(s[i : i+idx])
		i += idx + 2 // skip ${

		end := strings.IndexByte(s[i:], '}')
		if end < 0 {
			// No closing brace - preserve the literal ${ and stop scanning.
			b.WriteString("${")
			continue
		}
		expr := s[i : i+end]
		i += end + 1 // skip }

		varName := expr
		defaultVal := ""
		hasDefault := false
		if colonIdx := strings.Index(expr, ":-"); colonIdx >= 0 {
			varName = expr[:colonIdx]
			defaultVal = expr[colonIdx+2:]
			hasDefault = true
		}

		val := os.Getenv(varName)
		if val == "" && hasDefault {
			val = defaultVal
		}
		b.WriteString(val)
	}
	return b.String()
}

// ExpandConfigEnv expands every ${VAR} / ${VAR:-default} reference in the
// env-var-bearing fields of an ExternalMCPServerConfig (Command, Args, Env
// values, URL, Headers values). This lets MCP server definitions keep secrets
// out of the repo by referencing environment variables instead.
func ExpandConfigEnv(cfg *ExternalMCPServerConfig) {
	cfg.Command = expandEnvVar(cfg.Command)
	for i, arg := range cfg.Args {
		cfg.Args[i] = expandEnvVar(arg)
	}
	for k, v := range cfg.Env {
		cfg.Env[k] = expandEnvVar(v)
	}
	cfg.URL = expandEnvVar(cfg.URL)
	for k, v := range cfg.Headers {
		cfg.Headers[k] = expandEnvVar(v)
	}
}
