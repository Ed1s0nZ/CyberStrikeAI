package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"cyberstrike-ai/internal/config"

	"github.com/pkoukk/tiktoken-go"
	"go.uber.org/zap"
)

const (
	DefaultMaxTotalTokens   = 100_000
	DefaultMinRecentMessage = 10
	defaultChunkSize        = 10
	defaultMaxImages        = 3
	defaultSummaryTimeout   = 10 * time.Minute

	summaryPromptTemplate = `你是一名负责为安全代理执行上下文压缩的助手，任务是在保持所有关键渗透信息完整的前提下压缩扫描数据。

必须保留的关键信息：
- 已发现的漏洞与潜在攻击路径
- 扫描结果与工具输出（可压缩，但需保留核心发现）
- 获取到的访问凭证、令牌或认证细节
- 系统架构洞察与潜在薄弱点
- 当前评估进展
- 失败尝试与死路（避免重复劳动）
- 关于测试策略的所有决策记录

压缩指南：
- 保留精确技术细节（URL、路径、参数、Payload 等）
- 将冗长的工具输出压缩成概述，但保留关键发现
- 记录版本号与识别出的技术/组件信息
- 保留可能暗示漏洞的原始报错
- 将重复或相似发现整合成一条带有共性说明的结论

请牢记：另一位安全代理会依赖这份摘要继续测试，他必须在不损失任何作战上下文的情况下无缝接手。

需要压缩的对话片段：
%s

请给出技术精准且简明扼要的摘要，覆盖全部与安全评估相关的上下文。`
)

// MemoryCompressor 负责在调用LLM前压缩历史上下文，以避免Token爆炸。
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

// MemoryCompressorConfig 用于初始化 MemoryCompressor。
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

	// 当 CompletionClient 为空时，可以通过 OpenAIConfig + HTTPClient 构造默认的客户端。
	OpenAIConfig *config.OpenAIConfig
	HTTPClient   *http.Client
}

// NewMemoryCompressor 创建新的 MemoryCompressor。
func NewMemoryCompressor(cfg MemoryCompressorConfig) (*MemoryCompressor, error) {
	if cfg.Logger == nil {
		cfg.Logger = zap.NewNop()
	}

	if cfg.MaxTotalTokens <= 0 {
		cfg.MaxTotalTokens = DefaultMaxTotalTokens
	}
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

// CompressHistory 根据Token限制压缩历史消息。
func (mc *MemoryCompressor) CompressHistory(ctx context.Context, messages []ChatMessage) ([]ChatMessage, bool, error) {
	if len(messages) == 0 {
		return messages, false, nil
	}

	mc.handleImages(messages)

	systemMsgs, regularMsgs := mc.splitMessages(messages)
	if len(regularMsgs) <= mc.minRecentMessage {
		return messages, false, nil
	}

	totalTokens := mc.countTotalTokens(systemMsgs, regularMsgs)
	if totalTokens <= int(float64(mc.maxTotalTokens)*0.9) {
		return messages, false, nil
	}

	recentStart := len(regularMsgs) - mc.minRecentMessage
	recentStart = mc.adjustRecentStartForToolCalls(regularMsgs, recentStart)
	oldMsgs := regularMsgs[:recentStart]
	recentMsgs := regularMsgs[recentStart:]

	mc.logger.Info("memory compression triggered",
		zap.Int("total_tokens", totalTokens),
		zap.Int("max_total_tokens", mc.maxTotalTokens),
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

func (mc *MemoryCompressor) countTokens(text string) int {
	if mc.tokenCounter == nil {
		return len(text) / 4
	}
	count, err := mc.tokenCounter.Count(mc.summaryModel, text)
	if err != nil {
		return len(text) / 4
	}
	return count
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

	summary, err := mc.completionClient.Complete(ctx, mc.summaryModel, prompt, mc.timeout)
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

// TokenCounter 用于计算文本Token数量。
type TokenCounter interface {
	Count(model, text string) (int, error)
}

// TikTokenCounter 基于 tiktoken 的 Token 统计器。
type TikTokenCounter struct {
	mu               sync.RWMutex
	cache            map[string]*tiktoken.Tiktoken
	fallbackEncoding *tiktoken.Tiktoken
}

// NewTikTokenCounter 创建新的 TikTokenCounter。
func NewTikTokenCounter() *TikTokenCounter {
	return &TikTokenCounter{
		cache: make(map[string]*tiktoken.Tiktoken),
	}
}

// Count 实现 TokenCounter 接口。
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

// CompletionClient 对话压缩时使用的补全接口。
type CompletionClient interface {
	Complete(ctx context.Context, model string, prompt string, timeout time.Duration) (string, error)
}

// OpenAICompletionClient 基于 OpenAI Chat Completion。
type OpenAICompletionClient struct {
	config     *config.OpenAIConfig
	httpClient *http.Client
	logger     *zap.Logger
}

// NewOpenAICompletionClient 创建 OpenAICompletionClient。
func NewOpenAICompletionClient(cfg *config.OpenAIConfig, client *http.Client, logger *zap.Logger) *OpenAICompletionClient {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &OpenAICompletionClient{
		config:     cfg,
		httpClient: client,
		logger:     logger,
	}
}

// Complete 调用OpenAI获取摘要。
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

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	requestCtx := ctx
	var cancel context.CancelFunc
	if timeout > 0 {
		requestCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// 处理 BaseURL 路径拼接：移除末尾斜杠，然后添加 /chat/completions
	baseURL := strings.TrimSuffix(c.config.BaseURL, "/")
	url := baseURL + "/chat/completions"

	c.logger.Debug("calling completion API",
		zap.String("url", url),
		zap.String("model", model),
		zap.Int("prompt_length", len(prompt)))

	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// 读取响应体以获取更详细的错误信息
		bodyBytes, readErr := io.ReadAll(resp.Body)
		errorDetail := resp.Status
		if readErr == nil && len(bodyBytes) > 0 {
			// 尝试解析错误响应
			var errorResp struct {
				Error struct {
					Message string `json:"message"`
					Type    string `json:"type"`
					Code    string `json:"code"`
				} `json:"error"`
			}
			if json.Unmarshal(bodyBytes, &errorResp) == nil && errorResp.Error.Message != "" {
				errorDetail = fmt.Sprintf("%s: %s", resp.Status, errorResp.Error.Message)
			} else {
				errorDetail = fmt.Sprintf("%s: %s", resp.Status, string(bodyBytes))
			}
		}
		c.logger.Warn("completion API request failed",
			zap.Int("status_code", resp.StatusCode),
			zap.String("url", url),
			zap.String("model", model),
			zap.String("error", errorDetail))
		return "", fmt.Errorf("openai completion failed, status: %s", errorDetail)
	}

	var completion OpenAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&completion); err != nil {
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
