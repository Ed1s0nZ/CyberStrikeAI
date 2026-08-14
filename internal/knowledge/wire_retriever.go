package knowledge

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/openai"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/flow/retriever/multiquery"
	"go.uber.org/zap"
)

// WireRetrieverPipeline builds Eino MultiQuery + HTTP rerank + post-process pipeline on r.
// Call once after NewRetriever; UpdateConfig re-invokes when wireOpenAI is set.
func WireRetrieverPipeline(ctx context.Context, r *Retriever, openAI *config.OpenAIConfig) error {
	if r == nil {
		return fmt.Errorf("retriever is nil")
	}
	if openAI == nil {
		return fmt.Errorf("openai config is nil")
	}
	if r.config == nil {
		return fmt.Errorf("retrieval config is nil")
	}
	r.wireOpenAI = openAI

	httpClient := openai.NewEinoHTTPClient(openAI, &http.Client{Timeout: 120 * time.Second})
	maxCompletionTokens := openAI.MaxCompletionTokensEffective()
	chatCfg := &einoopenai.ChatModelConfig{
		APIKey:              strings.TrimSpace(openAI.APIKey),
		BaseURL:             strings.TrimSuffix(strings.TrimSpace(openAI.BaseURL), "/"),
		Model:               strings.TrimSpace(openAI.Model),
		HTTPClient:          httpClient,
		MaxCompletionTokens: &maxCompletionTokens,
	}
	if chatCfg.Model == "" {
		chatCfg.Model = "gpt-4o"
	}
	rewriteLLM, err := einoopenai.NewChatModel(ctx, chatCfg)
	if err != nil {
		return fmt.Errorf("multi_query rewrite model: %w", err)
	}

	reranker, err := NewHTTPReranker(&r.config.Rerank, openAI, r.logger)
	if err != nil {
		return fmt.Errorf("reranker: %w", err)
	}
	r.SetDocumentReranker(reranker)

	vec := NewVectorEinoRetriever(r)
	mq, err := multiquery.NewRetriever(ctx, &multiquery.Config{
		RewriteLLM:    rewriteLLM,
		MaxQueriesNum: r.config.MultiQuery.MaxQueriesEffective(),
		OrigRetriever: vec,
	})
	if err != nil {
		return fmt.Errorf("multi_query: %w", err)
	}

	r.pipeline = newKnowledgePipelineRetriever(mq, r)
	if r.logger != nil {
		provider := r.config.Rerank.ProviderEffective(strings.TrimSpace(openAI.BaseURL))
		r.logger.Info("知识库检索流水线已启用",
			zap.String("pipeline", "MultiQuery→Vector→Rerank→PostRetrieve"),
			zap.Int("multi_query_max", r.config.MultiQuery.MaxQueriesEffective()),
			zap.String("rerank_provider", provider),
			zap.String("rerank_model", r.config.Rerank.ModelEffective(provider)),
		)
	}
	return nil
}
