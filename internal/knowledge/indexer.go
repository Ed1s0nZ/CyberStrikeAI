package knowledge

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Indexer is responsible for chunking and vectorizing knowledge items
type Indexer struct {
	db        *sql.DB
	embedder  *Embedder
	logger    *zap.Logger
	chunkSize int // maximum estimated token count per chunk
	overlap   int // overlapping token count between chunks

	// error tracking
	mu            sync.RWMutex
	lastError     string    // most recent error message
	lastErrorTime time.Time // time of most recent error
	errorCount    int       // consecutive error count
}

// NewIndexer creates a new indexer
func NewIndexer(db *sql.DB, embedder *Embedder, logger *zap.Logger) *Indexer {
	return &Indexer{
		db:        db,
		embedder:  embedder,
		logger:    logger,
		chunkSize: 512, // default 512 tokens
		overlap:   50,  // default 50 token overlap
	}
}

// ChunkText splits text into chunks (supports overlap)
func (idx *Indexer) ChunkText(text string) []string {
	// split by Markdown headers
	chunks := idx.splitByMarkdownHeaders(text)

	// if a chunk is too large, split further
	result := make([]string, 0)
	for _, chunk := range chunks {
		if idx.estimateTokens(chunk) <= idx.chunkSize {
			result = append(result, chunk)
		} else {
			// split by paragraphs
			subChunks := idx.splitByParagraphs(chunk)
			for _, subChunk := range subChunks {
				if idx.estimateTokens(subChunk) <= idx.chunkSize {
					result = append(result, subChunk)
				} else {
					// split by sentences (with overlap)
					chunksWithOverlap := idx.splitBySentencesWithOverlap(subChunk)
					result = append(result, chunksWithOverlap...)
				}
			}
		}
	}

	return result
}

// splitByMarkdownHeaders splits by Markdown headers
func (idx *Indexer) splitByMarkdownHeaders(text string) []string {
	// match Markdown headers (# ## ### etc.)
	headerRegex := regexp.MustCompile(`(?m)^#{1,6}\s+.+$`)

	// find all header positions
	matches := headerRegex.FindAllStringIndex(text, -1)
	if len(matches) == 0 {
		return []string{text}
	}

	chunks := make([]string, 0)
	lastPos := 0

	for _, match := range matches {
		start := match[0]
		if start > lastPos {
			chunks = append(chunks, strings.TrimSpace(text[lastPos:start]))
		}
		lastPos = start
	}

	// add the last part
	if lastPos < len(text) {
		chunks = append(chunks, strings.TrimSpace(text[lastPos:]))
	}

	// filter empty chunks
	result := make([]string, 0)
	for _, chunk := range chunks {
		if strings.TrimSpace(chunk) != "" {
			result = append(result, chunk)
		}
	}

	if len(result) == 0 {
		return []string{text}
	}

	return result
}

// splitByParagraphs splits by paragraphs
func (idx *Indexer) splitByParagraphs(text string) []string {
	paragraphs := strings.Split(text, "\n\n")
	result := make([]string, 0)
	for _, p := range paragraphs {
		if strings.TrimSpace(p) != "" {
			result = append(result, strings.TrimSpace(p))
		}
	}
	return result
}

// splitBySentences splits by sentences (for internal use, without overlap logic)
func (idx *Indexer) splitBySentences(text string) []string {
	// simple sentence splitting (by period, question mark, exclamation mark)
	sentenceRegex := regexp.MustCompile(`[.!?]+\s+`)
	sentences := sentenceRegex.Split(text, -1)
	result := make([]string, 0)
	for _, s := range sentences {
		if strings.TrimSpace(s) != "" {
			result = append(result, strings.TrimSpace(s))
		}
	}
	return result
}

// splitBySentencesWithOverlap splits by sentences and applies an overlap strategy
func (idx *Indexer) splitBySentencesWithOverlap(text string) []string {
	if idx.overlap <= 0 {
		// if no overlap, use simple splitting
		return idx.splitBySentencesSimple(text)
	}

	sentences := idx.splitBySentences(text)
	if len(sentences) == 0 {
		return []string{}
	}

	result := make([]string, 0)
	currentChunk := ""

	for _, sentence := range sentences {
		testChunk := currentChunk
		if testChunk != "" {
			testChunk += "\n"
		}
		testChunk += sentence

		testTokens := idx.estimateTokens(testChunk)

		if testTokens > idx.chunkSize && currentChunk != "" {
			// current chunk has reached the size limit, save it
			result = append(result, currentChunk)

			// extract overlap from the end of the current chunk
			overlapText := idx.extractLastTokens(currentChunk, idx.overlap)
			if overlapText != "" {
				// if there is overlap content, use it as the start of the next chunk
				currentChunk = overlapText + "\n" + sentence
			} else {
				// if not enough overlap content can be extracted, use the current sentence directly
				currentChunk = sentence
			}
		} else {
			currentChunk = testChunk
		}
	}

	// add the last chunk
	if strings.TrimSpace(currentChunk) != "" {
		result = append(result, currentChunk)
	}

	// filter empty chunks
	filtered := make([]string, 0)
	for _, chunk := range result {
		if strings.TrimSpace(chunk) != "" {
			filtered = append(filtered, chunk)
		}
	}

	return filtered
}

// splitBySentencesSimple splits by sentences (simple version, no overlap)
func (idx *Indexer) splitBySentencesSimple(text string) []string {
	sentences := idx.splitBySentences(text)
	result := make([]string, 0)
	currentChunk := ""

	for _, sentence := range sentences {
		testChunk := currentChunk
		if testChunk != "" {
			testChunk += "\n"
		}
		testChunk += sentence

		if idx.estimateTokens(testChunk) > idx.chunkSize && currentChunk != "" {
			result = append(result, currentChunk)
			currentChunk = sentence
		} else {
			currentChunk = testChunk
		}
	}
	if currentChunk != "" {
		result = append(result, currentChunk)
	}

	return result
}

// extractLastTokens extracts the specified number of tokens from the end of the text
func (idx *Indexer) extractLastTokens(text string, tokenCount int) string {
	if tokenCount <= 0 || text == "" {
		return ""
	}

	// estimate character count (1 token ~= 4 characters)
	charCount := tokenCount * 4
	runes := []rune(text)

	if len(runes) <= charCount {
		return text
	}

	// extract the specified number of characters from the end
	// try to cut at a sentence boundary to avoid splitting mid-sentence
	startPos := len(runes) - charCount
	extracted := string(runes[startPos:])

	// try to find the first sentence boundary (period, question mark, or exclamation mark followed by a space)
	sentenceBoundary := regexp.MustCompile(`[.!?]+\s+`)
	matches := sentenceBoundary.FindStringIndex(extracted)
	if len(matches) > 0 && matches[0] > 0 {
		// cut at sentence boundary to preserve complete sentences
		extracted = extracted[matches[0]:]
	}

	return strings.TrimSpace(extracted)
}

// estimateTokens estimates token count (simple estimate: 1 token ~= 4 characters)
func (idx *Indexer) estimateTokens(text string) int {
	return len([]rune(text)) / 4
}

// IndexItem indexes a knowledge item (chunks and vectorizes it)
func (idx *Indexer) IndexItem(ctx context.Context, itemID string) error {
	// get knowledge item (including category and title, used for vectorization)
	var content, category, title string
	err := idx.db.QueryRow("SELECT content, category, title FROM knowledge_base_items WHERE id = ?", itemID).Scan(&content, &category, &title)
	if err != nil {
		return fmt.Errorf("failed to get knowledge item: %w", err)
	}

	// delete old vectors (already cleared uniformly in RebuildIndex; kept here for compatibility when IndexItem is called individually)
	_, err = idx.db.Exec("DELETE FROM knowledge_embeddings WHERE item_id = ?", itemID)
	if err != nil {
		return fmt.Errorf("failed to delete old vectors: %w", err)
	}

	// chunk
	chunks := idx.ChunkText(content)
	idx.logger.Info("knowledge item chunking complete", zap.String("itemId", itemID), zap.Int("chunks", len(chunks)))

	// track errors for this knowledge item
	itemErrorCount := 0
	var firstError error
	firstErrorChunkIndex := -1

	// vectorize each chunk (include category and title info so that risk type can be matched during vector retrieval)
	for i, chunk := range chunks {
		// include category and title info in the text to be vectorized
		// format: "[Risk Type: {category}] [Title: {title}]\n{chunk content}"
		// this way the vector embedding includes risk type info; even if SQL filtering fails, vector similarity can still help match
		textForEmbedding := fmt.Sprintf("[Risk Type: %s] [Title: %s]\n%s", category, title, chunk)

		embedding, err := idx.embedder.EmbedText(ctx, textForEmbedding)
		if err != nil {
			itemErrorCount++
			if firstError == nil {
				firstError = err
				firstErrorChunkIndex = i
				// only log detailed information on the first chunk failure
				chunkPreview := chunk
				if len(chunkPreview) > 200 {
					chunkPreview = chunkPreview[:200] + "..."
				}
				idx.logger.Warn("vectorization failed",
					zap.String("itemId", itemID),
					zap.Int("chunkIndex", i),
					zap.Int("totalChunks", len(chunks)),
					zap.String("chunkPreview", chunkPreview),
					zap.Error(err),
				)

				// update global error tracking
				errorMsg := fmt.Sprintf("vectorization failed (knowledge item: %s): %v", itemID, err)
				idx.mu.Lock()
				idx.lastError = errorMsg
				idx.lastErrorTime = time.Now()
				idx.mu.Unlock()
			}

			// if 2 consecutive chunks fail, stop processing this knowledge item immediately (lower threshold, stop faster)
			// this avoids wasting API calls and detects configuration issues more quickly
			if itemErrorCount >= 2 {
				idx.logger.Error("knowledge item consecutive vectorization failed, stopping processing",
					zap.String("itemId", itemID),
					zap.Int("totalChunks", len(chunks)),
					zap.Int("failedChunks", itemErrorCount),
					zap.Int("firstErrorChunkIndex", firstErrorChunkIndex),
					zap.Error(firstError),
				)
				return fmt.Errorf("knowledge item consecutive vectorization failed (%d chunks failed): %v", itemErrorCount, firstError)
			}
			continue
		}

		// save vector
		chunkID := uuid.New().String()
		embeddingJSON, _ := json.Marshal(embedding)

		_, err = idx.db.Exec(
			"INSERT INTO knowledge_embeddings (id, item_id, chunk_index, chunk_text, embedding, created_at) VALUES (?, ?, ?, ?, ?, datetime('now'))",
			chunkID, itemID, i, chunk, string(embeddingJSON),
		)
		if err != nil {
			idx.logger.Warn("failed to save vector", zap.String("itemId", itemID), zap.Int("chunkIndex", i), zap.Error(err))
			continue
		}
	}

	idx.logger.Info("knowledge item indexing complete", zap.String("itemId", itemID), zap.Int("chunks", len(chunks)))
	return nil
}

// HasIndex checks whether an index exists
func (idx *Indexer) HasIndex() (bool, error) {
	var count int
	err := idx.db.QueryRow("SELECT COUNT(*) FROM knowledge_embeddings").Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check index: %w", err)
	}
	return count > 0, nil
}

// RebuildIndex rebuilds all indexes
func (idx *Indexer) RebuildIndex(ctx context.Context) error {
	// reset error tracking
	idx.mu.Lock()
	idx.lastError = ""
	idx.lastErrorTime = time.Time{}
	idx.errorCount = 0
	idx.mu.Unlock()

	rows, err := idx.db.Query("SELECT id FROM knowledge_base_items")
	if err != nil {
		return fmt.Errorf("failed to query knowledge items: %w", err)
	}
	defer rows.Close()

	var itemIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("failed to scan knowledge item ID: %w", err)
		}
		itemIDs = append(itemIDs, id)
	}

	idx.logger.Info("starting index rebuild", zap.Int("totalItems", len(itemIDs)))

	// clear all old vectors before starting rebuild to ensure progress starts from 0
	// this allows GetIndexStatus to accurately reflect rebuild progress
	_, err = idx.db.Exec("DELETE FROM knowledge_embeddings")
	if err != nil {
		idx.logger.Warn("failed to clear old index", zap.Error(err))
		// continue even if clearing fails
	} else {
		idx.logger.Info("old index cleared, starting rebuild")
	}

	failedCount := 0
	consecutiveFailures := 0
	maxConsecutiveFailures := 2 // stop immediately after 2 consecutive failures (lower threshold, stop faster)
	firstFailureItemID := ""
	var firstFailureError error

	for i, itemID := range itemIDs {
		if err := idx.IndexItem(ctx, itemID); err != nil {
			failedCount++
			consecutiveFailures++

			// only log details on the first failure
			if consecutiveFailures == 1 {
				firstFailureItemID = itemID
				firstFailureError = err
				idx.logger.Warn("failed to index knowledge item",
					zap.String("itemId", itemID),
					zap.Int("totalItems", len(itemIDs)),
					zap.Error(err),
				)
			}

			// if too many consecutive failures, likely a configuration issue; stop indexing immediately
			if consecutiveFailures >= maxConsecutiveFailures {
				errorMsg := fmt.Sprintf("%d consecutive knowledge item indexing failures, possibly a configuration issue (e.g. embedding model misconfiguration, invalid API key, insufficient balance). First failure item: %s, error: %v", consecutiveFailures, firstFailureItemID, firstFailureError)
				idx.mu.Lock()
				idx.lastError = errorMsg
				idx.lastErrorTime = time.Now()
				idx.mu.Unlock()

				idx.logger.Error("too many consecutive indexing failures, stopping indexing immediately",
					zap.Int("consecutiveFailures", consecutiveFailures),
					zap.Int("totalItems", len(itemIDs)),
					zap.Int("processedItems", i+1),
					zap.String("firstFailureItemId", firstFailureItemID),
					zap.Error(firstFailureError),
				)
				return fmt.Errorf("too many consecutive indexing failures: %v", firstFailureError)
			}

			// if too many failed knowledge items, log a warning but continue (lower threshold to 30%)
			if failedCount > len(itemIDs)*3/10 && failedCount == len(itemIDs)*3/10+1 {
				errorMsg := fmt.Sprintf("too many failed knowledge items (%d/%d), possibly a configuration issue. First failure item: %s, error: %v", failedCount, len(itemIDs), firstFailureItemID, firstFailureError)
				idx.mu.Lock()
				idx.lastError = errorMsg
				idx.lastErrorTime = time.Now()
				idx.mu.Unlock()

				idx.logger.Error("too many failed knowledge items, possibly a configuration issue",
					zap.Int("failedCount", failedCount),
					zap.Int("totalItems", len(itemIDs)),
					zap.String("firstFailureItemId", firstFailureItemID),
					zap.Error(firstFailureError),
				)
			}
			continue
		}

		// reset consecutive failure count and first failure info on success
		if consecutiveFailures > 0 {
			consecutiveFailures = 0
			firstFailureItemID = ""
			firstFailureError = nil
		}

		// reduce progress log frequency (log every 10 items or every 10%)
		if (i+1)%10 == 0 || (len(itemIDs) > 0 && (i+1)*100/len(itemIDs)%10 == 0 && (i+1)*100/len(itemIDs) > 0) {
			idx.logger.Info("indexing progress", zap.Int("current", i+1), zap.Int("total", len(itemIDs)), zap.Int("failed", failedCount))
		}
	}

	idx.logger.Info("index rebuild complete", zap.Int("totalItems", len(itemIDs)), zap.Int("failedCount", failedCount))
	return nil
}

// GetLastError returns the most recent error message
func (idx *Indexer) GetLastError() (string, time.Time) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.lastError, idx.lastErrorTime
}
