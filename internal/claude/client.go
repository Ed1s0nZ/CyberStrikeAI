package claude

import "context"

// Client interface for interacting with Claude.
type Client interface {
	SendPrompt(ctx context.Context, prompt string, opts *PromptOptions) (*Result, error)
}

// PromptOptions configures a prompt request.
type PromptOptions struct {
	SystemPrompt string
	SessionID    string
	MaxTurns     int
	AllowedTools []string
}

// Result represents a Claude CLI response.
type Result struct {
	Type                string  `json:"type"`
	Subtype             string  `json:"subtype"`
	CostUSD             float64 `json:"cost_usd"`
	DurationMs          float64 `json:"duration_ms"`
	DurationAPIMs       float64 `json:"duration_api_ms"`
	IsError             bool    `json:"is_error"`
	NumTurns            int     `json:"num_turns"`
	Result              string  `json:"result"`
	SessionID           string  `json:"session_id"`
	TotalCostUSD        float64 `json:"total_cost_usd"`
	Usage               Usage   `json:"usage"`
}

// Usage represents token usage from a Claude response.
type Usage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
}
