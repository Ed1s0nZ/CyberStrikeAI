package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"go.uber.org/zap"
)

// CLIClient interacts with Claude via the CLI binary.
type CLIClient struct {
	binaryPath string
	workdir    string // working directory for the claude process (empty = inherit)
	logger     *zap.Logger
}

// NewCLIClient creates a CLIClient using "claude" from PATH.
func NewCLIClient(logger *zap.Logger) *CLIClient {
	return &CLIClient{
		binaryPath: "claude",
		logger:     logger,
	}
}

// NewCLIClientWithPath creates a CLIClient with an explicit binary path.
func NewCLIClientWithPath(path string, logger *zap.Logger) *CLIClient {
	return &CLIClient{
		binaryPath: path,
		logger:     logger,
	}
}

// SendPrompt sends a prompt to the Claude CLI and returns the parsed result.
func (c *CLIClient) SendPrompt(ctx context.Context, prompt string, opts *PromptOptions) (*Result, error) {
	args := c.buildArgs(prompt, opts)
	c.logger.Debug("executing claude CLI", zap.String("binary", c.binaryPath), zap.Strings("args", args))

	cmd := exec.CommandContext(ctx, c.binaryPath, args...)
	if c.workdir != "" {
		cmd.Dir = c.workdir
	}
	stdout, err := cmd.Output()
	if err != nil {
		return nil, c.handleError(err, stdout)
	}

	var result Result
	if err := json.Unmarshal(stdout, &result); err != nil {
		return nil, fmt.Errorf("failed to parse claude output: %w (raw: %s)", err, string(stdout))
	}

	c.logger.Debug("claude CLI response",
		zap.String("session_id", result.SessionID),
		zap.Float64("cost_usd", result.CostUSD),
		zap.Int("input_tokens", result.Usage.InputTokens),
		zap.Int("output_tokens", result.Usage.OutputTokens),
	)

	return &result, nil
}

// buildArgs constructs CLI arguments from the prompt and options.
func (c *CLIClient) buildArgs(prompt string, opts *PromptOptions) []string {
	args := []string{"-p", prompt, "--output-format", "json"}

	if opts == nil {
		return args
	}

	if opts.SystemPrompt != "" {
		args = append(args, "--system-prompt", opts.SystemPrompt)
	}
	if opts.SessionID != "" {
		args = append(args, "--resume", opts.SessionID)
	}
	if opts.MaxTurns > 0 {
		args = append(args, "--max-turns", fmt.Sprintf("%d", opts.MaxTurns))
	}
	if len(opts.AllowedTools) > 0 {
		args = append(args, "--allowedTools", strings.Join(opts.AllowedTools, ","))
	}
	if opts.MCPConfig != "" {
		args = append(args, "--mcp-config", opts.MCPConfig)
	}

	return args
}

// handleError attempts to extract a structured error from CLI output.
func (c *CLIClient) handleError(err error, stdout []byte) error {
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		return fmt.Errorf("failed to execute claude CLI: %w", err)
	}

	// Try parsing stderr as JSON error
	stderr := exitErr.Stderr
	if len(stderr) > 0 {
		var errResult Result
		if jsonErr := json.Unmarshal(stderr, &errResult); jsonErr == nil && errResult.IsError {
			return fmt.Errorf("claude CLI error: %s", errResult.Result)
		}
	}

	// Try parsing stdout as JSON error (claude sometimes writes errors to stdout)
	if len(stdout) > 0 {
		var errResult Result
		if jsonErr := json.Unmarshal(stdout, &errResult); jsonErr == nil && errResult.IsError {
			return fmt.Errorf("claude CLI error: %s", errResult.Result)
		}
	}

	if len(stderr) > 0 {
		return fmt.Errorf("claude CLI failed (exit %d): %s", exitErr.ExitCode(), string(stderr))
	}
	return fmt.Errorf("claude CLI failed (exit %d): %w", exitErr.ExitCode(), err)
}
