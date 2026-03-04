package agent

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"cyberstrike-ai/internal/knowledge"

	"go.uber.org/zap"
)

// RAGContextInjector proactively retrieves relevant knowledge-base content
// and injects it into the agent's system prompt before the first LLM call.
//
// This gives the agent immediate awareness of relevant attack techniques,
// vulnerability details, and recommended tooling without waiting for the
// agent to reactively call search_knowledge_base.  The injected block is
// placed in the system prompt so the LLM can use it when reasoning about
// which tools to invoke and how to exploit discovered weaknesses.
type RAGContextInjector struct {
	retriever     *knowledge.Retriever
	logger        *zap.Logger
	maxChunks     int           // maximum knowledge chunks to inject per request
	maxCharsTotal int           // total character budget for the injected context block
	fetchTimeout  time.Duration // per-request timeout for the pre-flight knowledge fetch
}

// RAGContextConfig configures the RAGContextInjector.
type RAGContextConfig struct {
	// MaxChunks is the maximum number of retrieved chunks to include in the
	// injected context block.  Default: 8.
	MaxChunks int
	// MaxCharsTotal is the total character budget for the injected block.
	// Content is truncated when this limit is exceeded.  Default: 6000.
	MaxCharsTotal int
	// FetchTimeout is the per-request timeout for the pre-flight knowledge
	// fetch.  Default: 15s.
	FetchTimeout time.Duration
}

// NewRAGContextInjector creates a new RAGContextInjector with the given
// retriever and configuration.
func NewRAGContextInjector(retriever *knowledge.Retriever, logger *zap.Logger, cfg RAGContextConfig) *RAGContextInjector {
	if cfg.MaxChunks <= 0 {
		cfg.MaxChunks = 8
	}
	if cfg.MaxCharsTotal <= 0 {
		cfg.MaxCharsTotal = 6000
	}
	if cfg.FetchTimeout <= 0 {
		cfg.FetchTimeout = 15 * time.Second
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &RAGContextInjector{
		retriever:     retriever,
		logger:        logger,
		maxChunks:     cfg.MaxChunks,
		maxCharsTotal: cfg.MaxCharsTotal,
		fetchTimeout:  cfg.FetchTimeout,
	}
}

// BuildContextBlock fetches knowledge relevant to query and returns a
// formatted system-prompt block ready for injection.  Returns an empty
// string when no relevant knowledge is found or retrieval fails so callers
// can safely skip injection.
func (r *RAGContextInjector) BuildContextBlock(ctx context.Context, query string) string {
	if r == nil || r.retriever == nil || strings.TrimSpace(query) == "" {
		return ""
	}

	fetchCtx, cancel := context.WithTimeout(ctx, r.fetchTimeout)
	defer cancel()

	// Use a slightly relaxed threshold (0.6) so the proactive fetch casts a
	// wider net.  The LLM will judge relevance in context.
	req := &knowledge.SearchRequest{
		Query:     query,
		TopK:      r.maxChunks,
		Threshold: 0.6,
	}

	results, err := r.retriever.Search(fetchCtx, req)
	if err != nil {
		r.logger.Debug("RAG pre-flight search failed", zap.Error(err))
		return ""
	}
	if len(results) == 0 {
		return ""
	}

	// Sort by hybrid score descending to surface the most relevant results first.
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Group chunks by knowledge-base item so each document appears as a
	// cohesive block rather than as disjointed snippets.
	type itemGroup struct {
		itemID   string
		results  []*knowledge.RetrievalResult
		maxScore float64
	}
	groupMap := make(map[string]*itemGroup)
	itemOrder := make([]string, 0)
	for _, res := range results {
		id := res.Item.ID
		g, exists := groupMap[id]
		if !exists {
			g = &itemGroup{itemID: id}
			groupMap[id] = g
			itemOrder = append(itemOrder, id)
		}
		g.results = append(g.results, res)
		if res.Score > g.maxScore {
			g.maxScore = res.Score
		}
	}

	// Sort groups by their best hybrid score.
	sort.Slice(itemOrder, func(i, j int) bool {
		return groupMap[itemOrder[i]].maxScore > groupMap[itemOrder[j]].maxScore
	})

	var sb strings.Builder
	sb.WriteString("<rag_knowledge_context>\n")
	sb.WriteString("The following knowledge has been automatically retrieved from the knowledge base as relevant to your current task. " +
		"Use it to guide tool selection, exploitation strategy, and bypass techniques:\n\n")

	charBudget := r.maxCharsTotal
	itemCount := 0

	for _, itemID := range itemOrder {
		if charBudget <= 0 {
			break
		}
		g := groupMap[itemID]
		if len(g.results) == 0 {
			continue
		}

		// Sort chunks by document position for natural reading order.
		sort.Slice(g.results, func(i, j int) bool {
			return g.results[i].Chunk.ChunkIndex < g.results[j].Chunk.ChunkIndex
		})

		mainResult := g.results[0]
		header := fmt.Sprintf("[%s] %s (relevance: %.0f%%)\n",
			mainResult.Item.Category, mainResult.Item.Title, g.maxScore*100)

		var chunkText strings.Builder
		for _, res := range g.results {
			chunkText.WriteString(res.Chunk.ChunkText)
			chunkText.WriteString("\n")
		}

		entry := header + chunkText.String() + "\n"
		if len(entry) > charBudget {
			entry = entry[:charBudget] + "...\n\n"
			charBudget = 0
		} else {
			charBudget -= len(entry)
		}
		sb.WriteString(entry)
		itemCount++
	}

	if itemCount == 0 {
		return ""
	}

	sb.WriteString("</rag_knowledge_context>\n")

	r.logger.Info("RAG context block injected into system prompt",
		zap.String("query", truncateStr(query, 80)),
		zap.Int("items", itemCount),
		zap.Int("chars", r.maxCharsTotal-charBudget),
	)

	return sb.String()
}

// ToolGuidanceHint returns a concise hint listing the knowledge-base
// categories that match the current query.  This is appended to the system
// prompt as a lightweight alternative to the full context block when the
// agent already has a large context and needs only a directional hint.
func (r *RAGContextInjector) ToolGuidanceHint(ctx context.Context, query string) string {
	if r == nil || r.retriever == nil || strings.TrimSpace(query) == "" {
		return ""
	}

	fetchCtx, cancel := context.WithTimeout(ctx, r.fetchTimeout)
	defer cancel()

	req := &knowledge.SearchRequest{
		Query:     query,
		TopK:      3,
		Threshold: 0.6,
	}

	results, err := r.retriever.Search(fetchCtx, req)
	if err != nil || len(results) == 0 {
		return ""
	}

	seen := make(map[string]bool)
	categories := make([]string, 0, 3)
	for _, res := range results {
		cat := res.Item.Category
		if !seen[cat] {
			seen[cat] = true
			categories = append(categories, cat)
		}
	}
	if len(categories) == 0 {
		return ""
	}

	return fmt.Sprintf("\nKnowledge base hint: Relevant attack categories detected — %s. "+
		"Use search_knowledge_base for detailed exploitation techniques.",
		strings.Join(categories, ", "))
}

// truncateStr truncates s to at most max runes, appending "..." when trimmed.
func truncateStr(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}
