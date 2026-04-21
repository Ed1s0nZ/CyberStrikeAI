package agent

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"cyberstrike-ai/internal/knowledge"

	"go.uber.org/zap"
)

// ragCache holds a cached RAG result keyed by the normalised query words.
type ragCache struct {
	queryWords map[string]struct{} // lowercased word set of the original query
	result     string              // the formatted block / hint
	createdAt  time.Time
}

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

	// Cache to avoid re-fetching identical or very similar queries.
	cacheMu   sync.Mutex
	blockCache *ragCache // cached BuildContextBlock result
	hintCache  *ragCache // cached ToolGuidanceHint result
}

const (
	// ragCacheTTL is how long a cached RAG result is considered fresh.
	ragCacheTTL = 5 * time.Minute
	// ragCacheSimilarityThreshold is the minimum Jaccard word-overlap
	// required to consider a new query "the same" as the cached one.
	ragCacheSimilarityThreshold = 0.7
)

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

// queryWords splits a query into a lowercase word set for similarity comparison.
func queryWords(query string) map[string]struct{} {
	words := strings.Fields(strings.ToLower(query))
	set := make(map[string]struct{}, len(words))
	for _, w := range words {
		// Strip common punctuation so "target," matches "target".
		w = strings.Trim(w, ".,;:!?\"'`()[]{}/<>")
		if len(w) >= 2 { // skip single-char noise
			set[w] = struct{}{}
		}
	}
	return set
}

// jaccardSimilarity returns the Jaccard index between two word sets.
func jaccardSimilarity(a, b map[string]struct{}) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0
	}
	if len(a) == 0 || len(b) == 0 {
		return 0.0
	}
	intersection := 0
	for w := range a {
		if _, ok := b[w]; ok {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0.0
	}
	return float64(intersection) / float64(union)
}

// checkCache returns the cached result if the query is similar enough and
// the cache is fresh.  Returns ("", false) on miss.
func (r *RAGContextInjector) checkCache(cache *ragCache, newWords map[string]struct{}) (string, bool) {
	if cache == nil {
		return "", false
	}
	if time.Since(cache.createdAt) > ragCacheTTL {
		return "", false
	}
	if jaccardSimilarity(cache.queryWords, newWords) >= ragCacheSimilarityThreshold {
		return cache.result, true
	}
	return "", false
}

// BuildContextBlock fetches knowledge relevant to query and returns a
// formatted system-prompt block ready for injection.  Returns an empty
// string when no relevant knowledge is found or retrieval fails so callers
// can safely skip injection.
//
// Results are cached: when a subsequent query has high word-overlap with the
// previous one (Jaccard >= 0.7) and the cache is < 5 minutes old, the
// cached block is returned without hitting the knowledge base.
func (r *RAGContextInjector) BuildContextBlock(ctx context.Context, query string) string {
	if r == nil || r.retriever == nil || strings.TrimSpace(query) == "" {
		return ""
	}

	words := queryWords(query)

	// Check cache first.
	r.cacheMu.Lock()
	if cached, hit := r.checkCache(r.blockCache, words); hit {
		r.cacheMu.Unlock()
		r.logger.Debug("RAG context block served from cache",
			zap.String("query", truncateStr(query, 80)),
		)
		return cached
	}
	r.cacheMu.Unlock()

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

	block := sb.String()

	r.logger.Info("RAG context block injected into system prompt",
		zap.String("query", truncateStr(query, 80)),
		zap.Int("items", itemCount),
		zap.Int("chars", r.maxCharsTotal-charBudget),
	)

	// Store in cache.
	r.cacheMu.Lock()
	r.blockCache = &ragCache{
		queryWords: words,
		result:     block,
		createdAt:  time.Now(),
	}
	r.cacheMu.Unlock()

	return block
}

// ToolGuidanceHint returns a concise hint listing the knowledge-base
// categories that match the current query.  This is appended to the system
// prompt as a lightweight alternative to the full context block when the
// agent already has a large context and needs only a directional hint.
//
// Results are cached with the same similarity logic as BuildContextBlock.
func (r *RAGContextInjector) ToolGuidanceHint(ctx context.Context, query string) string {
	if r == nil || r.retriever == nil || strings.TrimSpace(query) == "" {
		return ""
	}

	words := queryWords(query)

	// Check cache first.
	r.cacheMu.Lock()
	if cached, hit := r.checkCache(r.hintCache, words); hit {
		r.cacheMu.Unlock()
		r.logger.Debug("RAG tool guidance hint served from cache",
			zap.String("query", truncateStr(query, 80)),
		)
		return cached
	}
	r.cacheMu.Unlock()

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

	hint := fmt.Sprintf("\nKnowledge base hint: Relevant attack categories detected - %s. "+
		"Use search_knowledge_base for detailed exploitation techniques.",
		strings.Join(categories, ", "))

	// Store in cache.
	r.cacheMu.Lock()
	r.hintCache = &ragCache{
		queryWords: words,
		result:     hint,
		createdAt:  time.Now(),
	}
	r.cacheMu.Unlock()

	return hint
}

// truncateStr truncates s to at most max runes, appending "..." when trimmed.
func truncateStr(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}
