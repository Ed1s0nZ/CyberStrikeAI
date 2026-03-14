package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"cyberstrike-ai/internal/config"

	"go.uber.org/zap"
)

// Client 统一封装与OpenAI兼容模型交互的HTTP客户端。
type Client struct {
	httpClient *http.Client
	config     *config.OpenAIConfig
	logger     *zap.Logger
}

// APIError 表示OpenAI接口返回的非200错误。
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("openai api error: status=%d body=%s", e.StatusCode, e.Body)
}

// NewClient 创建一个新的OpenAI客户端。
func NewClient(cfg *config.OpenAIConfig, httpClient *http.Client, logger *zap.Logger) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Client{
		httpClient: httpClient,
		config:     cfg,
		logger:     logger,
	}
}

// UpdateConfig 动态更新OpenAI配置。
func (c *Client) UpdateConfig(cfg *config.OpenAIConfig) {
	c.config = cfg
}

// ChatCompletion 调用 /chat/completions 接口。
// 当 provider 为 "anthropic" 时，自动将 OpenAI 格式转换为 Anthropic Messages API 格式。
func (c *Client) ChatCompletion(ctx context.Context, payload interface{}, out interface{}) error {
	if c == nil {
		return fmt.Errorf("openai client is not initialized")
	}
	if c.config == nil {
		return fmt.Errorf("openai config is nil")
	}

	if strings.EqualFold(c.config.Provider, "anthropic") {
		return c.anthropicChatCompletion(ctx, payload, out)
	}

	if strings.TrimSpace(c.config.APIKey) == "" {
		return fmt.Errorf("openai api key is empty")
	}

	baseURL := strings.TrimSuffix(c.config.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal openai payload: %w", err)
	}

	c.logger.Debug("sending OpenAI chat completion request",
		zap.Int("payloadSizeKB", len(body)/1024))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build openai request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	requestStart := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("call openai api: %w", err)
	}
	defer resp.Body.Close()

	bodyChan := make(chan []byte, 1)
	errChan := make(chan error, 1)
	go func() {
		responseBody, err := io.ReadAll(resp.Body)
		if err != nil {
			errChan <- err
			return
		}
		bodyChan <- responseBody
	}()

	var respBody []byte
	select {
	case respBody = <-bodyChan:
	case err := <-errChan:
		return fmt.Errorf("read openai response: %w", err)
	case <-ctx.Done():
		return fmt.Errorf("read openai response timeout: %w", ctx.Err())
	case <-time.After(25 * time.Minute):
		return fmt.Errorf("read openai response timeout (25m)")
	}

	c.logger.Debug("received OpenAI response",
		zap.Int("status", resp.StatusCode),
		zap.Duration("duration", time.Since(requestStart)),
		zap.Int("responseSizeKB", len(respBody)/1024),
	)

	if resp.StatusCode != http.StatusOK {
		c.logger.Warn("OpenAI chat completion returned non-200",
			zap.Int("status", resp.StatusCode),
			zap.String("body", string(respBody)),
		)
		return &APIError{
			StatusCode: resp.StatusCode,
			Body:       string(respBody),
		}
	}

	if out != nil {
		if err := json.Unmarshal(respBody, out); err != nil {
			c.logger.Error("failed to unmarshal OpenAI response",
				zap.Error(err),
				zap.String("body", string(respBody)),
			)
			return fmt.Errorf("unmarshal openai response: %w", err)
		}
	}

	return nil
}
