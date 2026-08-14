package knowledge

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"cyberstrike-ai/internal/config"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// normalizeChunkStrategy returns "recursive" or "markdown_then_recursive".
func normalizeChunkStrategy(s string) string {
	v := strings.TrimSpace(strings.ToLower(s))
	switch v {
	case "recursive":
		return "recursive"
	case "markdown_then_recursive", "markdown_recursive", "markdown":
		return "markdown_then_recursive"
	case "":
		return "markdown_then_recursive"
	default:
		return "markdown_then_recursive"
	}
}

func buildKnowledgeIndexChain(
	ctx context.Context,
	indexingCfg *config.IndexingConfig,
	db *sql.DB,
	recursive document.Transformer,
	embeddingModel string,
) (compose.Runnable[[]*schema.Document, []string], error) {
	if recursive == nil {
		return nil, fmt.Errorf("recursive transformer is nil")
	}
	if db == nil {
		return nil, fmt.Errorf("db is nil")
	}
	strategy := normalizeChunkStrategy("markdown_then_recursive")
	batch := 64
	maxChunks := 0
	if indexingCfg != nil {
		strategy = normalizeChunkStrategy(indexingCfg.ChunkStrategy)
		if indexingCfg.BatchSize > 0 {
			batch = indexingCfg.BatchSize
		}
		maxChunks = indexingCfg.MaxChunksPerItem
	}

	si := NewSQLiteIndexer(db, batch, embeddingModel)
	ch := compose.NewChain[[]*schema.Document, []string]()
	if strategy != "recursive" {
		md, err := newMarkdownHeaderSplitter(ctx)
		if err != nil {
			return nil, fmt.Errorf("markdown splitter: %w", err)
		}
		ch.AppendDocumentTransformer(md)
	}
	ch.AppendDocumentTransformer(recursive)
	ch.AppendLambda(newChunkEnrichLambda(maxChunks))
	ch.AppendIndexer(si)
	return ch.Compile(ctx)
}

func newChunkEnrichLambda(maxChunks int) *compose.Lambda {
	return compose.InvokableLambda(func(ctx context.Context, docs []*schema.Document) ([]*schema.Document, error) {
		_ = ctx
		out := make([]*schema.Document, 0, len(docs))
		for _, d := range docs {
			if d == nil || strings.TrimSpace(d.Content) == "" {
				continue
			}
			out = append(out, d)
		}
		if maxChunks > 0 && len(out) > maxChunks {
			out = out[:maxChunks]
		}
		for i, d := range out {
			if d.MetaData == nil {
				d.MetaData = make(map[string]any)
			}
			d.MetaData[metaKBChunkIndex] = i
		}
		return out, nil
	})
}
