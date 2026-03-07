package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/openai"

	"github.com/pkoukk/tiktoken-go"
	"go.uber.org/zap"
)

const (
	// DefaultMinRecentMessage is the number of recent messages to retain when compressing history, ensuring recent conversation context is not compressed
	DefaultMinRecentMessage = 5
	// defaultChunkSize is the message chunk size for each compression pass, splitting old messages into multiple chunks for summarization
	defaultChunkSize = 10
	// defaultMaxImages is the maximum number of images to retain during compression; images exceeding this limit are removed to save context space
	defaultMaxImages = 3
	// defaultSummaryTimeout is the timeout duration for generating a message summary
	defaultSummaryTimeout = 10 * time.Minute
	// Max output size of a single chunk summary to prevent summary bloat.
	defaultMaxSummaryChars = 3500
	// Max number of preserved key findings appended to chunk summary.
	defaultMaxPreservedFindings = 12

	summaryPromptTemplate = `You are an assistant responsible for performing context compression for a security agent. Your task is to compress scan data while keeping all critical penetration testing information intact.

Key information that MUST be preserved:
- Discovered vulnerabilities and potential attack paths
- Scan results and tool outputs (may be compressed but core findings must be retained)
- Obtained access credentials, tokens, or authentication details
- System architecture insights and potential weaknesses
- Current assessment progress
- Failed attempts and dead ends (to avoid repeated work)
- All decision records regarding testing strategy
- Knowledge base citations and retrieved exploitation techniques (record which knowledge items were consulted and their key findings)
- Tool selection rationale derived from retrieved knowledge (retain the "why" behind tool choices)

Compression guidelines:
- Retain precise technical details (URLs, paths, parameters, payloads, etc.)
- Compress verbose tool outputs into overviews but retain key findings
- Record version numbers and identified technologies/components
- Retain original error messages that may indicate vulnerabilities
- Consolidate duplicate or similar findings into a single conclusion with a common description
- When summarising knowledge-base lookups, preserve the category name, knowledge item title, and key technique or payload referenced

Remember: another security agent will rely on this summary to continue testing and must be able to take over seamlessly without losing any operational context, including knowledge that was previously retrieved from the knowledge base.

Conversation segments to compress:
%s

Output format requirements (strict):
1) [TASK_STATUS] — objective progress and current state in 2-4 bullets.
2) [KEY_TECHNICAL_FINDINGS] — critical findings only (vulns, credentials, exploitable paths, decisive errors).
3) [NEXT_BEST_ACTIONS] — concrete next actions (max 5 bullets), aligned to current objective.

Rules:
- Keep it compact and actionable; remove duplicate details.
- Preserve exact indicators when available (host/IP/URL/port/CVE/payload/parameter/path).
- Do not include narrative filler.
- If no findings exist, explicitly state "No confirmed findings yet".`
)

// MemoryCompressor is responsible for compressing historical context before calling the LLM to avoid token explosion.
type MemoryCompressor struct {
	maxTotalTokens   int
	minRecentMessage int
	maxImages        int
	chunkSize        int
	summaryModel     string
	timeout          time.Duration

	tokenCounter     TokenCounter
	completionClient CompletionClient
	logger           *zap.Logger
}

var (
	contextSummaryTagRe = regexp.MustCompile(`(?is)</?context_summary[^>]*>`)
)

// MemoryCompressorConfig is used to initialize a MemoryCompressor.
type MemoryCompressorConfig struct {
	MaxTotalTokens   int
	MinRecentMessage int
	MaxImages        int
	ChunkSize        int
	SummaryModel     string
	Timeout          time.Duration
	TokenCounter     TokenCounter
	CompletionClient CompletionClient
	Logger           *zap.Logger

	// When CompletionClient is nil, a default client can be constructed via OpenAIConfig + HTTPClient.
	OpenAIConfig *config.OpenAIConfig
	HTTPClient   *http.Client
}

// NewMemoryCompressor creates a new MemoryCompressor.
func NewMemoryCompressor(cfg MemoryCompressorConfig) (*MemoryCompressor, error) {
	if cfg.Logger == nil {
		cfg.Logger = zap.NewNop()
	}

	// If MaxTotalTokens is not explicitly configured, the subsequent logic controls it based on the model max context length;
	// it is recommended to configure this uniformly in openai.max_total_tokens in config.yaml.
	if cfg.MinRecentMessage <= 0 {
		cfg.MinRecentMessage = DefaultMinRecentMessage
	}
	if cfg.MaxImages <= 0 {
		cfg.MaxImages = defaultMaxImages
	}
	if cfg.ChunkSize <= 0 {
		cfg.ChunkSize = defaultChunkSize
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultSummaryTimeout
	}
	if cfg.SummaryModel == "" && cfg.OpenAIConfig != nil {
		if cfg.OpenAIConfig.SummaryModel != "" {
			cfg.SummaryModel = cfg.OpenAIConfig.SummaryModel
		} else if cfg.OpenAIConfig.Model != "" {
			cfg.SummaryModel = cfg.OpenAIConfig.Model
		}
	}
	if cfg.SummaryModel == "" {
		return nil, errors.New("summary model is required (either SummaryModel or OpenAIConfig.Model must be set)")
	}
	if cfg.TokenCounter == nil {
		cfg.TokenCounter = NewTikTokenCounter()
	}

	if cfg.CompletionClient == nil {
		if cfg.OpenAIConfig == nil {
			return nil, errors.New("memory compressor requires either CompletionClient or OpenAIConfig")
		}
		if cfg.HTTPClient == nil {
			cfg.HTTPClient = &http.Client{
				Timeout: 5 * time.Minute,
			}
		}
		// Use summary-specific endpoint when configured, falling back to main config.
		summaryCfg := cfg.OpenAIConfig
		if cfg.OpenAIConfig.SummaryBaseURL != "" || cfg.OpenAIConfig.SummaryAPIKey != "" {
			summaryBaseURL, summaryAPIKey := cfg.OpenAIConfig.EffectiveSummaryConfig()
			summaryCfg = &config.OpenAIConfig{
				APIKey:  summaryAPIKey,
				BaseURL: summaryBaseURL,
				Model:   cfg.SummaryModel,
			}
		}
		cfg.CompletionClient = NewOpenAICompletionClient(summaryCfg, cfg.HTTPClient, cfg.Logger)
	}

	return &MemoryCompressor{
		maxTotalTokens:   cfg.MaxTotalTokens,
		minRecentMessage: cfg.MinRecentMessage,
		maxImages:        cfg.MaxImages,
		chunkSize:        cfg.ChunkSize,
		summaryModel:     cfg.SummaryModel,
		timeout:          cfg.Timeout,
		tokenCounter:     cfg.TokenCounter,
		completionClient: cfg.CompletionClient,
		logger:           cfg.Logger,
	}, nil
}

// UpdateConfig updates the OpenAI configuration (used for dynamic model config updates).
func (mc *MemoryCompressor) UpdateConfig(cfg *config.OpenAIConfig) {
	if cfg == nil {
		return
	}

	// Update summaryModel field: prefer dedicated SummaryModel, fall back to main Model
	if cfg.SummaryModel != "" {
		mc.summaryModel = cfg.SummaryModel
	} else if cfg.Model != "" {
		mc.summaryModel = cfg.Model
	}

	// Update config in completionClient (if it is an OpenAICompletionClient).
	// When a summary-specific endpoint is configured, pass it through so the
	// completion client targets the correct server.
	if openAIClient, ok := mc.completionClient.(*OpenAICompletionClient); ok {
		effectiveCfg := cfg
		if cfg.SummaryBaseURL != "" || cfg.SummaryAPIKey != "" {
			summaryBaseURL, summaryAPIKey := cfg.EffectiveSummaryConfig()
			effectiveCfg = &config.OpenAIConfig{
				APIKey:  summaryAPIKey,
				BaseURL: summaryBaseURL,
				Model:   mc.summaryModel,
			}
		}
		openAIClient.UpdateConfig(effectiveCfg)
		mc.logger.Info("MemoryCompressor config updated",
			zap.String("summary_model", mc.summaryModel),
		)
	}
}

// CompressHistory compresses historical messages based on the token limit. reservedTokens is the number of tokens reserved for non-message content such as tools; compression uses (maxTotalTokens - reservedTokens) as the message token ceiling.
func (mc *MemoryCompressor) CompressHistory(ctx context.Context, messages []ChatMessage, reservedTokens int) ([]ChatMessage, bool, error) {
	if len(messages) == 0 {
		return messages, false, nil
	}

	mc.handleImages(messages)

	systemMsgs, regularMsgs := mc.splitMessages(messages)
	if len(regularMsgs) <= mc.minRecentMessage {
		return messages, false, nil
	}

	effectiveMax := mc.maxTotalTokens
	if reservedTokens > 0 && reservedTokens < mc.maxTotalTokens {
		effectiveMax = mc.maxTotalTokens - reservedTokens
	}

	totalTokens := mc.countTotalTokens(systemMsgs, regularMsgs)
	if totalTokens <= int(float64(effectiveMax)*0.9) {
		return messages, false, nil
	}

	recentStart := len(regularMsgs) - mc.minRecentMessage
	recentStart = mc.adjustRecentStartForToolCalls(regularMsgs, recentStart)
	oldMsgs := regularMsgs[:recentStart]
	recentMsgs := regularMsgs[recentStart:]

	mc.logger.Info("memory compression triggered",
		zap.Int("total_tokens", totalTokens),
		zap.Int("max_total_tokens", mc.maxTotalTokens),
		zap.Int("reserved_tokens", reservedTokens),
		zap.Int("effective_max", effectiveMax),
		zap.Int("system_messages", len(systemMsgs)),
		zap.Int("regular_messages", len(regularMsgs)),
		zap.Int("old_messages", len(oldMsgs)),
		zap.Int("recent_messages", len(recentMsgs)))

	var compressed []ChatMessage
	for i := 0; i < len(oldMsgs); i += mc.chunkSize {
		end := i + mc.chunkSize
		if end > len(oldMsgs) {
			end = len(oldMsgs)
		}
		chunk := oldMsgs[i:end]
		if len(chunk) == 0 {
			continue
		}
		summary, err := mc.summarizeChunk(ctx, chunk)
		if err != nil {
			mc.logger.Warn("chunk summary failed, fallback to raw chunk",
				zap.Error(err),
				zap.Int("start", i),
				zap.Int("end", end))
			compressed = append(compressed, chunk...)
			continue
		}
		compressed = append(compressed, summary)
	}

	finalMessages := make([]ChatMessage, 0, len(systemMsgs)+len(compressed)+len(recentMsgs))
	finalMessages = append(finalMessages, systemMsgs...)
	finalMessages = append(finalMessages, compressed...)
	finalMessages = append(finalMessages, recentMsgs...)

	return finalMessages, true, nil
}

func (mc *MemoryCompressor) handleImages(messages []ChatMessage) {
	if mc.maxImages <= 0 {
		return
	}
	count := 0
	for i := len(messages) - 1; i >= 0; i-- {
		content := messages[i].Content
		if !strings.Contains(content, "[IMAGE]") {
			continue
		}
		count++
		if count > mc.maxImages {
			messages[i].Content = "[Previously attached image removed to preserve context]"
		}
	}
}

func (mc *MemoryCompressor) splitMessages(messages []ChatMessage) (systemMsgs, regularMsgs []ChatMessage) {
	for _, msg := range messages {
		if strings.EqualFold(msg.Role, "system") {
			systemMsgs = append(systemMsgs, msg)
		} else {
			regularMsgs = append(regularMsgs, msg)
		}
	}
	return
}

func (mc *MemoryCompressor) countTotalTokens(systemMsgs, regularMsgs []ChatMessage) int {
	total := 0
	for _, msg := range systemMsgs {
		total += mc.countMessageTokens(msg)
	}
	for _, msg := range regularMsgs {
		total += mc.countMessageTokens(msg)
	}
	return total
}

// countMessageTokens counts tokens for a single message, including ToolCalls JSON when present.
func (mc *MemoryCompressor) countMessageTokens(msg ChatMessage) int {
	total := mc.countTokens(msg.Content)
	if len(msg.ToolCalls) > 0 {
		if data, err := json.Marshal(msg.ToolCalls); err == nil {
			total += mc.countTokens(string(data))
		}
	}
	return total
}

// getModelName returns the name of the model currently in use for summarization.
// summaryModel is always kept up-to-date by UpdateConfig and is guaranteed non-empty
// after construction (NewMemoryCompressor requires it).
func (mc *MemoryCompressor) getModelName() string {
	return mc.summaryModel
}

func (mc *MemoryCompressor) countTokens(text string) int {
	if mc.tokenCounter == nil {
		return len(text) / 4
	}
	modelName := mc.getModelName()
	count, err := mc.tokenCounter.Count(modelName, text)
	if err != nil {
		return len(text) / 4
	}
	return count
}

// CountTextTokens is a publicly exposed text token counter, used to count tokens for non-message content such as tools (e.g., after serializing tools on the agent side).
func (mc *MemoryCompressor) CountTextTokens(text string) int {
	return mc.countTokens(text)
}

// totalTokensFor provides token statistics without mutating the message list.
func (mc *MemoryCompressor) totalTokensFor(messages []ChatMessage) (totalTokens int, systemCount int, regularCount int) {
	if len(messages) == 0 {
		return 0, 0, 0
	}
	systemMsgs, regularMsgs := mc.splitMessages(messages)
	return mc.countTotalTokens(systemMsgs, regularMsgs), len(systemMsgs), len(regularMsgs)
}

func (mc *MemoryCompressor) summarizeChunk(ctx context.Context, chunk []ChatMessage) (ChatMessage, error) {
	if len(chunk) == 0 {
		return ChatMessage{}, errors.New("chunk is empty")
	}
	formatted := make([]string, 0, len(chunk))
	for _, msg := range chunk {
		formatted = append(formatted, fmt.Sprintf("%s: %s", msg.Role, mc.extractMessageText(msg)))
	}
	conversation := strings.Join(formatted, "\n")
	prompt := fmt.Sprintf(summaryPromptTemplate, conversation)
	preservedFindings := mc.extractCriticalFindings(chunk, defaultMaxPreservedFindings)

	// Use dynamically obtained model name, not the saved summaryModel
	modelName := mc.getModelName()
	summary, err := mc.completionClient.Complete(ctx, modelName, prompt, mc.timeout)
	if err != nil {
		return ChatMessage{}, err
	}
	summary = strings.TrimSpace(summary)
	if summary == "" {
		// Concatenate all messages in the chunk rather than discarding all but the first
		var sb strings.Builder
		for _, msg := range chunk {
			sb.WriteString(fmt.Sprintf("%s: %s\n", msg.Role, mc.extractMessageText(msg)))
		}
		return ChatMessage{
			Role:    "assistant",
			Content: fmt.Sprintf("<context_summary message_count='%d'>[Summary unavailable]\n%s</context_summary>", len(chunk), sb.String()),
		}, nil
	}
	summary = mc.compactSummary(summary)
	if len(preservedFindings) > 0 {
		var preserved strings.Builder
		preserved.WriteString("\n[PRESERVED_KEY_FINDINGS]\n")
		for _, finding := range preservedFindings {
			preserved.WriteString("- ")
			preserved.WriteString(finding)
			preserved.WriteString("\n")
		}
		summary += preserved.String()
	}
	if len(summary) > defaultMaxSummaryChars {
		summary = summary[:defaultMaxSummaryChars] + "...[truncated]"
	}

	return ChatMessage{
		Role:    "assistant",
		Content: fmt.Sprintf("<context_summary message_count='%d'>%s</context_summary>", len(chunk), summary),
	}, nil
}

func (mc *MemoryCompressor) extractMessageText(msg ChatMessage) string {
	content := strings.TrimSpace(msg.Content)
	if content == "" {
		return ""
	}
	// Avoid recursive expansion when previous chunk summaries are summarized again.
	// Keep them compact by stripping wrapper tags and normalizing whitespace.
	content = contextSummaryTagRe.ReplaceAllString(content, "")
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.TrimSpace(content)
	if len(content) > 1600 {
		content = content[:1600] + "...[truncated]"
	}
	return content
}

func (mc *MemoryCompressor) compactSummary(summary string) string {
	s := strings.TrimSpace(summary)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return strings.Join(out, "\n")
}

func (mc *MemoryCompressor) extractCriticalFindings(chunk []ChatMessage, maxItems int) []string {
	if maxItems <= 0 {
		return nil
	}
	type scored struct {
		text  string
		score int
	}
	candidates := make([]scored, 0, 32)
	seen := make(map[string]struct{})

	scoreLine := func(lowerLine string, role string) int {
		score := 0
		if strings.Contains(lowerLine, "cve-") || strings.Contains(lowerLine, "critical") {
			score += 6
		}
		if strings.Contains(lowerLine, "vulnerab") || strings.Contains(lowerLine, "rce") ||
			strings.Contains(lowerLine, "sqli") || strings.Contains(lowerLine, "xss") ||
			strings.Contains(lowerLine, "lfi") || strings.Contains(lowerLine, "auth bypass") {
			score += 5
		}
		if strings.Contains(lowerLine, "password") || strings.Contains(lowerLine, "token") ||
			strings.Contains(lowerLine, "apikey") || strings.Contains(lowerLine, "api_key") ||
			strings.Contains(lowerLine, "secret") || strings.Contains(lowerLine, "credential") {
			score += 5
		}
		if strings.Contains(lowerLine, "http://") || strings.Contains(lowerLine, "https://") ||
			strings.Contains(lowerLine, "/api/") || strings.Contains(lowerLine, "/bitrix/") {
			score += 3
		}
		if strings.Contains(lowerLine, "port ") || strings.Contains(lowerLine, "open ") ||
			strings.Contains(lowerLine, "service") || strings.Contains(lowerLine, "execution id") {
			score += 2
		}
		if strings.Contains(lowerLine, "error") || strings.Contains(lowerLine, "failed") ||
			strings.Contains(lowerLine, "timeout") || strings.Contains(lowerLine, "unauthorized") ||
			strings.Contains(lowerLine, "forbidden") {
			score += 3
		}
		if role == "user" {
			// Keep explicit user objective/progress direction visible.
			score += 1
		}
		return score
	}

	for _, msg := range chunk {
		text := mc.extractMessageText(msg)
		if text == "" {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(msg.Role))
		lines := strings.Split(text, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			lower := strings.ToLower(line)
			score := scoreLine(lower, role)
			if score < 4 {
				continue
			}
			if len(line) > 220 {
				line = line[:220] + "...[truncated]"
			}
			key := strings.ToLower(line)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			candidates = append(candidates, scored{text: line, score: score})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			return len(candidates[i].text) < len(candidates[j].text)
		}
		return candidates[i].score > candidates[j].score
	})

	if len(candidates) > maxItems {
		candidates = candidates[:maxItems]
	}
	result := make([]string, 0, len(candidates))
	for _, c := range candidates {
		result = append(result, c.text)
	}
	return result
}

func (mc *MemoryCompressor) adjustRecentStartForToolCalls(msgs []ChatMessage, recentStart int) int {
	if recentStart <= 0 || recentStart >= len(msgs) {
		return recentStart
	}

	adjusted := recentStart
	for adjusted > 0 && strings.EqualFold(msgs[adjusted].Role, "tool") {
		adjusted--
	}

	if adjusted != recentStart {
		mc.logger.Debug("adjusted recent window to keep tool call context",
			zap.Int("original_recent_start", recentStart),
			zap.Int("adjusted_recent_start", adjusted),
		)
	}

	return adjusted
}

// TokenCounter is used to count the number of text tokens.
type TokenCounter interface {
	Count(model, text string) (int, error)
}

// TikTokenCounter is a token counter based on tiktoken.
type TikTokenCounter struct {
	mu               sync.RWMutex
	cache            map[string]*tiktoken.Tiktoken
	fallbackEncoding *tiktoken.Tiktoken
}

// NewTikTokenCounter creates a new TikTokenCounter.
func NewTikTokenCounter() *TikTokenCounter {
	return &TikTokenCounter{
		cache: make(map[string]*tiktoken.Tiktoken),
	}
}

// Count implements the TokenCounter interface.
func (tc *TikTokenCounter) Count(model, text string) (int, error) {
	enc, err := tc.encodingForModel(model)
	if err != nil {
		return len(text) / 4, err
	}
	tokens := enc.Encode(text, nil, nil)
	return len(tokens), nil
}

func (tc *TikTokenCounter) encodingForModel(model string) (*tiktoken.Tiktoken, error) {
	tc.mu.RLock()
	if enc, ok := tc.cache[model]; ok {
		tc.mu.RUnlock()
		return enc, nil
	}
	tc.mu.RUnlock()

	tc.mu.Lock()
	defer tc.mu.Unlock()

	if enc, ok := tc.cache[model]; ok {
		return enc, nil
	}

	enc, err := tiktoken.EncodingForModel(model)
	if err != nil {
		if tc.fallbackEncoding == nil {
			tc.fallbackEncoding, err = tiktoken.GetEncoding("cl100k_base")
			if err != nil {
				return nil, err
			}
		}
		tc.cache[model] = tc.fallbackEncoding
		return tc.fallbackEncoding, nil
	}

	tc.cache[model] = enc
	return enc, nil
}

// CompletionClient is the completion interface used during conversation compression.
type CompletionClient interface {
	Complete(ctx context.Context, model string, prompt string, timeout time.Duration) (string, error)
}

// OpenAICompletionClient is based on OpenAI Chat Completion.
type OpenAICompletionClient struct {
	config *config.OpenAIConfig
	client *openai.Client
	logger *zap.Logger
}

// NewOpenAICompletionClient creates an OpenAICompletionClient.
func NewOpenAICompletionClient(cfg *config.OpenAIConfig, client *http.Client, logger *zap.Logger) *OpenAICompletionClient {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &OpenAICompletionClient{
		config: cfg,
		client: openai.NewClient(cfg, client, logger),
		logger: logger,
	}
}

// UpdateConfig updates the underlying configuration.
func (c *OpenAICompletionClient) UpdateConfig(cfg *config.OpenAIConfig) {
	c.config = cfg
	if c.client != nil {
		c.client.UpdateConfig(cfg)
	}
}

// Complete calls OpenAI to obtain a summary.
func (c *OpenAICompletionClient) Complete(ctx context.Context, model string, prompt string, timeout time.Duration) (string, error) {
	if c.config == nil {
		return "", errors.New("openai config is required")
	}
	if model == "" {
		return "", errors.New("model name is required")
	}

	reqBody := OpenAIRequest{
		Model: model,
		Messages: []ChatMessage{
			{Role: "user", Content: prompt},
		},
	}

	requestCtx := ctx
	var cancel context.CancelFunc
	if timeout > 0 {
		requestCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	var completion OpenAIResponse
	if c.client == nil {
		return "", errors.New("openai completion client not initialized")
	}
	if err := c.client.ChatCompletion(requestCtx, reqBody, &completion); err != nil {
		if apiErr, ok := err.(*openai.APIError); ok {
			return "", fmt.Errorf("openai completion failed, status: %d, body: %s", apiErr.StatusCode, apiErr.Body)
		}
		return "", err
	}
	if completion.Error != nil {
		return "", errors.New(completion.Error.Message)
	}

	if len(completion.Choices) == 0 || completion.Choices[0].Message.Content == "" {
		return "", errors.New("empty completion response")
	}
	return completion.Choices[0].Message.Content, nil
}
