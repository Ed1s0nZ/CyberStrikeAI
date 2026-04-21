package knowledge

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/openai"

	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// Embedder embedder
type Embedder struct {
	openAIClient   *openai.Client
	config         *config.KnowledgeConfig
	openAIConfig   *config.OpenAIConfig // for getting API Key
	logger         *zap.Logger
	rateLimiter    *rate.Limiter       // rate limiter
	rateLimitDelay time.Duration       // request interval time
	maxRetries     int                 // max retry count
	retryDelay     time.Duration       // retry delay
	mu             sync.Mutex          // protects rateLimiter
}

// NewEmbedder creates a new embedder
func NewEmbedder(cfg *config.KnowledgeConfig, openAIConfig *config.OpenAIConfig, openAIClient *openai.Client, logger *zap.Logger) *Embedder {
	// rate limiter
	var rateLimiter *rate.Limiter
	var rateLimitDelay time.Duration

	// if MaxRPM configured,calculate rate limit from RPM
	if cfg.Indexing.MaxRPM > 0 {
		rpm := cfg.Indexing.MaxRPM
		rateLimiter = rate.NewLimiter(rate.Every(time.Minute/time.Duration(rpm)), rpm)
		logger.Info("knowledge baserate limit", zap.Int("maxRPM", rpm))
	} else if cfg.Indexing.RateLimitDelayMs > 0 {
		// if MaxRPM not configured but fixed delay configured,use fixed delay mode
		rateLimitDelay = time.Duration(cfg.Indexing.RateLimitDelayMs) * time.Millisecond
		logger.Info("knowledge base", zap.Duration("delay", rateLimitDelay))
	}

	// retry config
	maxRetries := 3
	retryDelay := 1000 * time.Millisecond
	if cfg.Indexing.MaxRetries > 0 {
		maxRetries = cfg.Indexing.MaxRetries
	}
	if cfg.Indexing.RetryDelayMs > 0 {
		retryDelay = time.Duration(cfg.Indexing.RetryDelayMs) * time.Millisecond
	}

	return &Embedder{
		openAIClient:   openAIClient,
		config:         cfg,
		openAIConfig:   openAIConfig,
		logger:         logger,
		rateLimiter:    rateLimiter,
		rateLimitDelay: rateLimitDelay,
		maxRetries:     maxRetries,
		retryDelay:     retryDelay,
	}
}

// EmbeddingRequest OpenAI embedding request
type EmbeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// EmbeddingResponse OpenAI embedding response
type EmbeddingResponse struct {
	Data []EmbeddingData `json:"data"`
	Error *EmbeddingError `json:"error,omitempty"`
}

// EmbeddingData embedding data
type EmbeddingData struct {
	Embedding []float64 `json:"embedding"`
	Index     int       `json:"index"`
}

// EmbeddingError error
type EmbeddingError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

// waitRateLimiter rate limiter
func (e *Embedder) waitRateLimiter() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.rateLimiter != nil {
		// wait for token
		ctx := context.Background()
		if err := e.rateLimiter.Wait(ctx); err != nil {
			e.logger.Warn("rate limiter", zap.Error(err))
		}
	}

	if e.rateLimitDelay > 0 {
		time.Sleep(e.rateLimitDelay)
	}
}

// EmbedText embed text(with retry and rate limiting)
func (e *Embedder) EmbedText(ctx context.Context, text string) ([]float32, error) {
	if e.openAIClient == nil {
		return nil, fmt.Errorf("OpenAI client not initialized")
	}

	var lastErr error
	for attempt := 0; attempt < e.maxRetries; attempt++ {
		// rate limit
		if attempt > 0 {
			// wait longer on retry
			waitTime := e.retryDelay * time.Duration(attempt)
			e.logger.Debug("waiting before retry", zap.Int("attempt", attempt+1), zap.Duration("waitTime", waitTime))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(waitTime):
			}
		} else {
			e.waitRateLimiter()
		}

		result, err := e.doEmbedText(ctx, text)
		if err == nil {
			return result, nil
		}

		lastErr = err

		// error(429 rate limit,5xx error,error)
		if !e.isRetryableError(err) {
			return nil, err
		}

		e.logger.Debug("embedding request,",
			zap.Int("attempt", attempt+1),
			zap.Int("maxRetries", e.maxRetries),
			zap.Error(err))
	}

	return nil, fmt.Errorf("max retry count (%d): %v", e.maxRetries, lastErr)
}

// doEmbedText embedding request()
func (e *Embedder) doEmbedText(ctx context.Context, text string) ([]float32, error) {
	// use configured embedding model
	model := e.config.Embedding.Model
	if model == "" {
		model = "text-embedding-3-small"
	}

	req := EmbeddingRequest{
		Model: model,
		Input: []string{text},
	}

	// clean baseURL:trim whitespace and trailing slashes
	baseURL := strings.TrimSpace(e.config.Embedding.BaseURL)
	baseURL = strings.TrimSuffix(baseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	// build request
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize request:%w", err)
	}

	requestURL := baseURL + "/embeddings"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request:%w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// use configured API Key,if none, use OpenAI configured one
	apiKey := strings.TrimSpace(e.config.Embedding.APIKey)
	if apiKey == "" && e.openAIConfig != nil {
		apiKey = e.openAIConfig.APIKey
	}
	if apiKey == "" {
		return nil, fmt.Errorf("API Key not configured")
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	// send request
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request:%w", err)
	}
	defer resp.Body.Close()

	// error
	bodyBytes := make([]byte, 0)
	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			bodyBytes = append(bodyBytes, buf[:n]...)
		}
		if err != nil {
			break
		}
	}

	// record()
	requestBodyPreview := string(body)
	if len(requestBodyPreview) > 200 {
		requestBodyPreview = requestBodyPreview[:200] + "..."
	}
	e.logger.Debug("embedding API request",
		zap.String("url", httpReq.URL.String()),
		zap.String("model", model),
		zap.String("requestBody", requestBodyPreview),
		zap.Int("status", resp.StatusCode),
		zap.Int("bodySize", len(bodyBytes)),
		zap.String("contentType", resp.Header.Get("Content-Type")),
	)

	var embeddingResp EmbeddingResponse
	if err := json.Unmarshal(bodyBytes, &embeddingResp); err != nil {
		// error
		bodyPreview := string(bodyBytes)
		if len(bodyPreview) > 500 {
			bodyPreview = bodyPreview[:500] + "..."
		}
		return nil, fmt.Errorf("parse (URL: %s, status:%d, response length:%dbytes): %w\nrequest body:%s\nresponse content preview:%s",
			requestURL, resp.StatusCode, len(bodyBytes), err, requestBodyPreview, bodyPreview)
	}

	if embeddingResp.Error != nil {
		return nil, fmt.Errorf("OpenAI API error (status:%d): type=%s, message=%s",
			resp.StatusCode, embeddingResp.Error.Type, embeddingResp.Error.Message)
	}

	if resp.StatusCode != http.StatusOK {
		bodyPreview := string(bodyBytes)
		if len(bodyPreview) > 500 {
			bodyPreview = bodyPreview[:500] + "..."
		}
		return nil, fmt.Errorf("HTTP request failed (URL: %s, status:%d): response content=%s", requestURL, resp.StatusCode, bodyPreview)
	}

	if len(embeddingResp.Data) == 0 {
		bodyPreview := string(bodyBytes)
		if len(bodyPreview) > 500 {
			bodyPreview = bodyPreview[:500] + "..."
		}
		return nil, fmt.Errorf("embedding data (status:%d, response length:%dbytes)\nresponse content:%s",
			resp.StatusCode, len(bodyBytes), bodyPreview)
	}

	// convert to float32
	embedding := make([]float32, len(embeddingResp.Data[0].Embedding))
	for i, v := range embeddingResp.Data[0].Embedding {
		embedding[i] = float32(v)
	}

	return embedding, nil
}

// isRetryableError error
func (e *Embedder) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// 429 rate limiterror
	if strings.Contains(errStr, "429") || strings.Contains(errStr, "rate limit") {
		return true
	}

	// 5xx error
	if strings.Contains(errStr, "500") || strings.Contains(errStr, "502") ||
		strings.Contains(errStr, "503") || strings.Contains(errStr, "504") {
		return true
	}

	// error
	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "connection") ||
		strings.Contains(errStr, "network") || strings.Contains(errStr, "EOF") {
		return true
	}

	return false
}

// EmbedTexts batch embed text
func (e *Embedder) EmbedTexts(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	embeddings := make([][]float32, len(texts))
	for i, text := range texts {
		embedding, err := e.EmbedText(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("embed text [%d] :%w", i, err)
		}
		embeddings[i] = embedding
	}

	return embeddings, nil
}
