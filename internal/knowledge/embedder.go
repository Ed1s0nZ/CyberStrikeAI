package knowledge

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/openai"

	"go.uber.org/zap"
)

// Embedder 文本嵌入器
type Embedder struct {
	openAIClient *openai.Client
	config       *config.KnowledgeConfig
	openAIConfig *config.OpenAIConfig // 用于获取API Key
	logger       *zap.Logger
}

// NewEmbedder 创建新的嵌入器
func NewEmbedder(cfg *config.KnowledgeConfig, openAIConfig *config.OpenAIConfig, openAIClient *openai.Client, logger *zap.Logger) *Embedder {
	return &Embedder{
		openAIClient: openAIClient,
		config:       cfg,
		openAIConfig: openAIConfig,
		logger:       logger,
	}
}

// EmbeddingRequest OpenAI嵌入请求
type EmbeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// EmbeddingResponse OpenAI嵌入响应
type EmbeddingResponse struct {
	Data []EmbeddingData `json:"data"`
	Error *EmbeddingError `json:"error,omitempty"`
}

// EmbeddingData 嵌入数据
type EmbeddingData struct {
	Embedding []float64 `json:"embedding"`
	Index     int       `json:"index"`
}

// EmbeddingError 嵌入错误
type EmbeddingError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

// EmbedText 对文本进行嵌入
func (e *Embedder) EmbedText(ctx context.Context, text string) ([]float32, error) {
	if e.openAIClient == nil {
		return nil, fmt.Errorf("OpenAI客户端未初始化")
	}

	// 使用配置的嵌入模型
	model := e.config.Embedding.Model
	if model == "" {
		model = "text-embedding-3-small"
	}

	req := EmbeddingRequest{
		Model: model,
		Input: []string{text},
	}

	// 清理baseURL：去除前后空格和尾部斜杠
	baseURL := strings.TrimSpace(e.config.Embedding.BaseURL)
	baseURL = strings.TrimSuffix(baseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	// 构建请求
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	requestURL := baseURL + "/embeddings"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	
	// 使用配置的API Key，如果没有则使用OpenAI配置的
	apiKey := strings.TrimSpace(e.config.Embedding.APIKey)
	if apiKey == "" && e.openAIConfig != nil {
		apiKey = e.openAIConfig.APIKey
	}
	if apiKey == "" {
		return nil, fmt.Errorf("API Key未配置")
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	// 发送请求
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应体以便在错误时输出详细信息
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

	// 记录请求和响应信息（用于调试）
	requestBodyPreview := string(body)
	if len(requestBodyPreview) > 200 {
		requestBodyPreview = requestBodyPreview[:200] + "..."
	}
	e.logger.Debug("嵌入API请求",
		zap.String("url", httpReq.URL.String()),
		zap.String("model", model),
		zap.String("requestBody", requestBodyPreview),
		zap.Int("status", resp.StatusCode),
		zap.Int("bodySize", len(bodyBytes)),
		zap.String("contentType", resp.Header.Get("Content-Type")),
	)

	var embeddingResp EmbeddingResponse
	if err := json.Unmarshal(bodyBytes, &embeddingResp); err != nil {
		// 输出详细的错误信息
		bodyPreview := string(bodyBytes)
		if len(bodyPreview) > 500 {
			bodyPreview = bodyPreview[:500] + "..."
		}
		return nil, fmt.Errorf("解析响应失败 (URL: %s, 状态码: %d, 响应长度: %d字节): %w\n请求体: %s\n响应内容预览: %s",
			requestURL, resp.StatusCode, len(bodyBytes), err, requestBodyPreview, bodyPreview)
	}

	if embeddingResp.Error != nil {
		return nil, fmt.Errorf("OpenAI API错误 (状态码: %d): 类型=%s, 消息=%s",
			resp.StatusCode, embeddingResp.Error.Type, embeddingResp.Error.Message)
	}

	if resp.StatusCode != http.StatusOK {
		bodyPreview := string(bodyBytes)
		if len(bodyPreview) > 500 {
			bodyPreview = bodyPreview[:500] + "..."
		}
		return nil, fmt.Errorf("HTTP请求失败 (URL: %s, 状态码: %d): 响应内容=%s", requestURL, resp.StatusCode, bodyPreview)
	}

	if len(embeddingResp.Data) == 0 {
		bodyPreview := string(bodyBytes)
		if len(bodyPreview) > 500 {
			bodyPreview = bodyPreview[:500] + "..."
		}
		return nil, fmt.Errorf("未收到嵌入数据 (状态码: %d, 响应长度: %d字节)\n响应内容: %s",
			resp.StatusCode, len(bodyBytes), bodyPreview)
	}

	// 转换为float32
	embedding := make([]float32, len(embeddingResp.Data[0].Embedding))
	for i, v := range embeddingResp.Data[0].Embedding {
		embedding[i] = float32(v)
	}

	return embedding, nil
}

// EmbedTexts 批量嵌入文本
func (e *Embedder) EmbedTexts(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	// OpenAI API支持批量，但为了简单起见，我们逐个处理
	// 实际可以使用批量API以提高效率
	embeddings := make([][]float32, len(texts))
	for i, text := range texts {
		embedding, err := e.EmbedText(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("嵌入文本[%d]失败: %w", i, err)
		}
		embeddings[i] = embedding
	}

	return embeddings, nil
}

