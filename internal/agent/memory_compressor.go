package agent

import (
	"context"
	"errors"
	"fmt"
	"net/http"
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

Please provide a technically precise and concise summary covering all context relevant to the security assessment.`
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
	if cfg.SummaryModel == "" && cfg.OpenAIConfig != nil && cfg.OpenAIConfig.Model != "" {
		cfg.SummaryModel = cfg.OpenAIConfig.Model
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
		cfg.CompletionClient = NewOpenAICompletionClient(cfg.OpenAIConfig, cfg.HTTPClient, cfg.Logger)
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

	// Update summaryModel field
	if cfg.Model != "" {
		mc.summaryModel = cfg.Model
	}

	// Update config in completionClient (if it is an OpenAICompletionClient)
	if openAIClient, ok := mc.completionClient.(*OpenAICompletionClient); ok {
		openAIClient.UpdateConfig(cfg)
		mc.logger.Info("MemoryCompressor config updated",
			zap.String("model", cfg.Model),
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
		total += mc.countTokens(msg.Content)
	}
	for _, msg := range regularMsgs {
		total += mc.countTokens(msg.Content)
	}
	return total
}

// getModelName returns the name of the model currently in use (prefers the latest config from completionClient).
func (mc *MemoryCompressor) getModelName() string {
	// If completionClient is an OpenAICompletionClient, get the latest model name from it
	if openAIClient, ok := mc.completionClient.(*OpenAICompletionClient); ok {
		if openAIClient.config != nil && openAIClient.config.Model != "" {
			return openAIClient.config.Model
		}
	}
	// Otherwise use the saved summaryModel
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

	// Use dynamically obtained model name, not the saved summaryModel
	modelName := mc.getModelName()
	summary, err := mc.completionClient.Complete(ctx, modelName, prompt, mc.timeout)
	if err != nil {
		return ChatMessage{}, err
	}
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return chunk[0], nil
	}

	return ChatMessage{
		Role:    "assistant",
		Content: fmt.Sprintf("<context_summary message_count='%d'>%s</context_summary>", len(chunk), summary),
	}, nil
}

func (mc *MemoryCompressor) extractMessageText(msg ChatMessage) string {
	return msg.Content
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
