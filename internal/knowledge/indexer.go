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

	"cyberstrike-ai/internal/config"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Indexer indexer，responsible for chunking and vectorizing knowledge items
type Indexer struct {
	db             *sql.DB
	embedder       *Embedder
	logger         *zap.Logger
	chunkSize      int // max tokens per chunk (estimated)
	overlap        int // overlap tokens between chunks
	maxChunks      int // max chunks per knowledge item (0 means unlimited)

	// error
	mu            sync.RWMutex
	lastError string // error
	lastErrorTime time.Time // error
	errorCount int // error

	// rebuild indexstatus
	rebuildMu          sync.RWMutex
	isRebuilding       bool      // whether index is being rebuilt
	rebuildTotalItems  int       // total rebuild items
	rebuildCurrent int // current
	rebuildFailed      int       // rebuild failed items
	rebuildStartTime   time.Time // rebuild start time
	rebuildLastItemID  string    // last processed item ID
	rebuildLastChunks  int       // last processed item chunk count
}

// NewIndexer creates a new indexer
func NewIndexer(db *sql.DB, embedder *Embedder, logger *zap.Logger, indexingCfg *config.IndexingConfig) *Indexer {
	chunkSize := 512
	overlap := 50
	maxChunks := 0
	if indexingCfg != nil {
		if indexingCfg.ChunkSize > 0 {
			chunkSize = indexingCfg.ChunkSize
		}
		if indexingCfg.ChunkOverlap >= 0 {
			overlap = indexingCfg.ChunkOverlap
		}
		if indexingCfg.MaxChunksPerItem > 0 {
			maxChunks = indexingCfg.MaxChunksPerItem
		}
	}
	return &Indexer{
		db:        db,
		embedder:  embedder,
		logger:    logger,
		chunkSize: chunkSize,
		overlap:   overlap,
		maxChunks: maxChunks,
	}
}

// ChunkText chunk text（，title）
func (idx *Indexer) ChunkText(text string) []string {
	// Markdown title，title
	sections := idx.splitByMarkdownHeadersWithContent(text)

	// process each block
	result := make([]string, 0)
	for _, section := range sections {
		// title（title，because content already includes it）
		// ：["# A", "## B", "### C"] -> "[# A > ## B]"
		var parentHeaderPath string
		if len(section.HeaderPath) > 1 {
			parentHeaderPath = strings.Join(section.HeaderPath[:len(section.HeaderPath)-1], " > ")
		}

		// title（ "# Prompt Injection"）
		firstLine, remainingContent := extractFirstLine(section.Content)

		// if remaining content is empty or whitespace only，title，skip
		if strings.TrimSpace(remainingContent) == "" {
			continue
		}

		// block too large，
		if idx.estimateTokens(section.Content) <= idx.chunkSize {
			// block size appropriate，addtitle
			if parentHeaderPath != "" {
				result = append(result, fmt.Sprintf("[%s] %s", parentHeaderPath, section.Content))
			} else {
				result = append(result, section.Content)
			}
		} else {
			// block too large，title，title
			// title（title）
			subSections := idx.splitBySubHeaders(section.Content, firstLine, parentHeaderPath)
			if len(subSections) > 1 {
				// title，recursively process each sub-block
				for _, sub := range subSections {
					if idx.estimateTokens(sub) <= idx.chunkSize {
						result = append(result, sub)
					} else {
						// sub-block still too large，split by paragraphs（title）
						paragraphs := idx.splitByParagraphsWithHeader(sub, parentHeaderPath)
						for _, para := range paragraphs {
							if idx.estimateTokens(para) <= idx.chunkSize {
								result = append(result, para)
							} else {
								// paragraph still too large，split by sentences
								sentenceChunks := idx.splitBySentencesWithOverlap(para)
								for _, chunk := range sentenceChunks {
									result = append(result, chunk)
								}
							}
						}
					}
				}
			} else {
				// title，split by paragraphs（title）
				paragraphs := idx.splitByParagraphsWithHeader(section.Content, parentHeaderPath)
				for _, para := range paragraphs {
					if idx.estimateTokens(para) <= idx.chunkSize {
						result = append(result, para)
					} else {
						// paragraph still too large，split by sentences
						sentenceChunks := idx.splitBySentencesWithOverlap(para)
						for _, chunk := range sentenceChunks {
							result = append(result, chunk)
						}
					}
				}
			}
		}
	}

	return result
}

// extractFirstLine extract first line and remaining content
func extractFirstLine(content string) (firstLine, remaining string) {
	lines := strings.SplitN(content, "\n", 2)
	if len(lines) == 0 {
		return "", ""
	}
	if len(lines) == 1 {
		return lines[0], ""
	}
	return lines[0], lines[1]
}

// splitBySubHeaders title（for handling large blocks）
// headerPrefix title，add
func (idx *Indexer) splitBySubHeaders(content, headerPrefix, parentPath string) []string {
	// match Markdown title（## ）
	subHeaderRegex := regexp.MustCompile(`(?m)^#{2,6}\s+.+$`)
	matches := subHeaderRegex.FindAllStringIndex(content, -1)

	if len(matches) == 0 {
		// title，returns
		return []string{content}
	}

	result := make([]string, 0, len(matches))
	for i, match := range matches {
		start := match[0]
		nextStart := len(content)
		if i+1 < len(matches) {
			nextStart = matches[i+1][0]
		}

		subContent := strings.TrimSpace(content[start:nextStart])

		// add
		if parentPath != "" {
			result = append(result, fmt.Sprintf("[%s] %s", parentPath, subContent))
		} else {
			result = append(result, subContent)
		}
	}

	return result
}

// splitByParagraphsWithHeader split by paragraphs，addtitle（to preserve context）
func (idx *Indexer) splitByParagraphsWithHeader(content, parentPath string) []string {
	// title
	firstLine, _ := extractFirstLine(content)

	paragraphs := strings.Split(content, "\n\n")
	result := make([]string, 0)

	for i, p := range paragraphs {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			continue
		}

		// title（no actual content）
		if strings.TrimSpace(trimmed) == strings.TrimSpace(firstLine) {
			continue
		}

		// title，add
		if i == 0 && strings.Contains(trimmed, firstLine) {
			if parentPath != "" {
				result = append(result, fmt.Sprintf("[%s] %s", parentPath, trimmed))
			} else {
				result = append(result, trimmed)
			}
		} else {
			// addtitle
			if parentPath != "" {
				result = append(result, fmt.Sprintf("[%s] %s\n%s", parentPath, firstLine, trimmed))
			} else {
				result = append(result, fmt.Sprintf("%s\n%s", firstLine, trimmed))
			}
		}
	}

	return result
}

// Section title
type Section struct {
	HeaderPath []string // title（ ["# SQL ", "## "]）
	Content    string   // block content
}

// splitByMarkdownHeadersWithContent Markdown title，returnstitle
// title，for vector retrieval
//
// ， Markdown:
//   # Prompt Injection
//   introduction content
//   ## Summary
//   table of contents
//
// returns：
//   [{HeaderPath: ["# Prompt Injection"], Content: "# Prompt Injection\nintroduction content"},
//    {HeaderPath: ["# Prompt Injection", "## Summary"], Content: "## Summary\ntable of contents"}]
func (idx *Indexer) splitByMarkdownHeadersWithContent(text string) []Section {
	// match Markdown title (# ## ### )
	headerRegex := regexp.MustCompile(`(?m)^#{1,6}\s+.+$`)

	// title
	matches := headerRegex.FindAllStringIndex(text, -1)
	if len(matches) == 0 {
		// title，returns
		return []Section{{HeaderPath: []string{}, Content: text}}
	}

	sections := make([]Section, 0, len(matches))
	currentHeaderPath := []string{}

	for i, match := range matches {
		start := match[0]
		end := match[1]
		nextStart := len(text)

		// title
		if i+1 < len(matches) {
			nextStart = matches[i+1][0]
		}

		// currenttitle
		headerLine := strings.TrimSpace(text[start:end])

		// title（# count）
		level := 0
		for _, ch := range headerLine {
			if ch == '#' {
				level++
			} else {
				break
			}
		}

		// title：currenttitle，addcurrenttitle
		newPath := make([]string, 0, len(currentHeaderPath)+1)
		for _, h := range currentHeaderPath {
			hLevel := 0
			for _, ch := range h {
				if ch == '#' {
					hLevel++
				} else {
					break
				}
			}
			if hLevel < level {
				newPath = append(newPath, h)
			}
		}
		newPath = append(newPath, headerLine)
		currentHeaderPath = newPath

		// currenttitletitle（currenttitle）
		content := strings.TrimSpace(text[start:nextStart])

		// create block，currenttitle（currenttitle）
		sections = append(sections, Section{
			HeaderPath: append([]string(nil), currentHeaderPath...),
			Content:    content,
		})
	}

	// filter empty blocks
	result := make([]Section, 0, len(sections))
	for _, section := range sections {
		if strings.TrimSpace(section.Content) != "" {
			result = append(result, section)
		}
	}

	if len(result) == 0 {
		return []Section{{HeaderPath: []string{}, Content: text}}
	}

	return result
}

// splitByParagraphs split by paragraphs
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

// splitBySentences split by sentences（，）
func (idx *Indexer) splitBySentences(text string) []string {
	// simple sentence splitting（by period, question mark, exclamation mark，supports Chinese and English）
	// . ! ? = English punctuation
	// \u3002 = 。(Chinese period)
	// \uFF01 = ！(Chinese exclamation)
	// \uFF1F = ？(Chinese question mark)
	sentenceRegex := regexp.MustCompile(`[.!?\x{3002}\x{FF01}\x{FF1F}]+`)
	sentences := sentenceRegex.Split(text, -1)
	result := make([]string, 0)
	for _, s := range sentences {
		if strings.TrimSpace(s) != "" {
			result = append(result, strings.TrimSpace(s))
		}
	}
	return result
}

// splitBySentencesWithOverlap split by sentences
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
			// current，save it
			result = append(result, currentChunk)

			// current
			overlapText := idx.extractLastTokens(currentChunk, idx.overlap)
			if overlapText != "" {
				// if there is overlap content，as start of next block
				currentChunk = overlapText + "\n" + sentence
			} else {
				// if unable to extract enough overlap content，current
				currentChunk = sentence
			}
		} else {
			currentChunk = testChunk
		}
	}

	// add
	if strings.TrimSpace(currentChunk) != "" {
		result = append(result, currentChunk)
	}

	// filter empty blocks
	filtered := make([]string, 0)
	for _, chunk := range result {
		if strings.TrimSpace(chunk) != "" {
			filtered = append(filtered, chunk)
		}
	}

	return filtered
}

// splitBySentencesSimple split by sentences（version，）
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

// extractLastTokens extract specified token count from end of text
func (idx *Indexer) extractLastTokens(text string, tokenCount int) string {
	if tokenCount <= 0 || text == "" {
		return ""
	}

	// estimate character count（1 token ≈ 4 ）
	charCount := tokenCount * 4
	runes := []rune(text)

	if len(runes) <= charCount {
		return text
	}

	// extract specified number of characters from end
	startPos := len(runes) - charCount
	extracted := string(runes[startPos:])

	// try to find first sentence boundary（supports Chinese and English）
	sentenceBoundary := regexp.MustCompile(`[.!?\x{3002}\x{FF01}\x{FF1F}]+`)
	matches := sentenceBoundary.FindStringIndex(extracted)
	if len(matches) > 0 && matches[0] > 0 {
		// truncate at sentence boundary，preserve complete sentences
		extracted = extracted[matches[0]:]
	}

	return strings.TrimSpace(extracted)
}

// estimateTokens estimate token count（：1 token ≈ 4 ）
func (idx *Indexer) estimateTokens(text string) int {
	return len([]rune(text)) / 4
}

// IndexItem index knowledge item (chunk and vectorize)
func (idx *Indexer) IndexItem(ctx context.Context, itemID string) error {
	// get knowledge item（including category and title，）
	var content, category, title string
	err := idx.db.QueryRow("SELECT content, category, title FROM knowledge_base_items WHERE id = ?", itemID).Scan(&content, &category, &title)
	if err != nil {
		return fmt.Errorf("get knowledge item：%w", err)
	}

	// delete（ RebuildIndex clear， IndexItem ）
	_, err = idx.db.Exec("DELETE FROM knowledge_embeddings WHERE item_id = ?", itemID)
	if err != nil {
		return fmt.Errorf("delete：%w", err)
	}

	// chunk
	chunks := idx.ChunkText(content)

	// apply max chunk limit
	if idx.maxChunks > 0 && len(chunks) > idx.maxChunks {
		idx.logger.Info("knowledge item chunks exceed limit, truncated",
			zap.String("itemId", itemID),
			zap.Int("originalChunks", len(chunks)),
			zap.Int("maxChunks", idx.maxChunks))
		chunks = chunks[:idx.maxChunks]
	}

	idx.logger.Info("chunk", zap.String("itemId", itemID), zap.Int("chunks", len(chunks)))

	// error
	itemErrorCount := 0
	var firstError error
	firstErrorChunkIndex := -1

	// vectorize each chunk（including category and title ，matchtype）
	for i, chunk := range chunks {
		// include category and title info in vectorized text
		// format："[type：{category}] [title：{title}]\n{chunk }"
		// type，even if SQL filtering fails，helpmatch
		textForEmbedding := fmt.Sprintf("[type：%s] [title：%s]\n%s", category, title, chunk)

		embedding, err := idx.embedder.EmbedText(ctx, textForEmbedding)
		if err != nil {
			itemErrorCount++
			if firstError == nil {
				firstError = err
				firstErrorChunkIndex = i
				// record
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

				// error
				errorMsg := fmt.Sprintf("vectorization failed (：%s): %v", itemID, err)
				idx.mu.Lock()
				idx.lastError = errorMsg
				idx.lastErrorTime = time.Now()
				idx.mu.Unlock()
			}

			// 5 ，stop
			// continue API ，can also detect config issues faster
			// for large documents (over 10 chunks)，allow failure rate up to 50%
			maxConsecutiveFailures := 5
			if len(chunks) > 10 && itemErrorCount > len(chunks)/2 {
				idx.logger.Error("vectorization failed，stop",
					zap.String("itemId", itemID),
					zap.Int("totalChunks", len(chunks)),
					zap.Int("failedChunks", itemErrorCount),
					zap.Int("firstErrorChunkIndex", firstErrorChunkIndex),
					zap.Error(firstError),
				)
				return fmt.Errorf("vectorization failed (%d/%d): %v", itemErrorCount, len(chunks), firstError)
			}
			if itemErrorCount >= maxConsecutiveFailures {
				idx.logger.Error("vectorization failed，stop",
					zap.String("itemId", itemID),
					zap.Int("totalChunks", len(chunks)),
					zap.Int("failedChunks", itemErrorCount),
					zap.Int("firstErrorChunkIndex", firstErrorChunkIndex),
					zap.Error(firstError),
				)
				return fmt.Errorf("vectorization failed (%d): %v", itemErrorCount, firstError)
			}
			continue
		}

		// save vectors
		chunkID := uuid.New().String()
		embeddingJSON, _ := json.Marshal(embedding)

		_, err = idx.db.Exec(
			"INSERT INTO knowledge_embeddings (id, item_id, chunk_index, chunk_text, embedding, created_at) VALUES (?, ?, ?, ?, ?, datetime('now'))",
			chunkID, itemID, i, chunk, string(embeddingJSON),
		)
		if err != nil {
			idx.logger.Warn("save vectors", zap.String("itemId", itemID), zap.Int("chunkIndex", i), zap.Error(err))
			continue
		}
	}

	idx.logger.Info("knowledge item indexing complete", zap.String("itemId", itemID), zap.Int("chunks", len(chunks)))

	// status
	idx.rebuildMu.Lock()
	idx.rebuildLastItemID = itemID
	idx.rebuildLastChunks = len(chunks)
	idx.rebuildMu.Unlock()

	return nil
}

// HasIndex check if index exists
func (idx *Indexer) HasIndex() (bool, error) {
	var count int
	err := idx.db.QueryRow("SELECT COUNT(*) FROM knowledge_embeddings").Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check index：%w", err)
	}
	return count > 0, nil
}

// RebuildIndex rebuild all indexes
func (idx *Indexer) RebuildIndex(ctx context.Context) error {
	// status
	idx.rebuildMu.Lock()
	idx.isRebuilding = true
	idx.rebuildTotalItems = 0
	idx.rebuildCurrent = 0
	idx.rebuildFailed = 0
	idx.rebuildStartTime = time.Now()
	idx.rebuildLastItemID = ""
	idx.rebuildLastChunks = 0
	idx.rebuildMu.Unlock()

	// error
	idx.mu.Lock()
	idx.lastError = ""
	idx.lastErrorTime = time.Time{}
	idx.errorCount = 0
	idx.mu.Unlock()

	rows, err := idx.db.Query("SELECT id FROM knowledge_base_items")
	if err != nil {
		// status
		idx.rebuildMu.Lock()
		idx.isRebuilding = false
		idx.rebuildMu.Unlock()
		return fmt.Errorf("failed to query knowledge items：%w", err)
	}
	defer rows.Close()

	var itemIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			// status
			idx.rebuildMu.Lock()
			idx.isRebuilding = false
			idx.rebuildMu.Unlock()
			return fmt.Errorf("scan ID ：%w", err)
		}
		itemIDs = append(itemIDs, id)
	}

	idx.rebuildMu.Lock()
	idx.rebuildTotalItems = len(itemIDs)
	idx.rebuildMu.Unlock()

	idx.logger.Info("start rebuilding index", zap.Int("totalItems", len(itemIDs)))

	// ：clear，
	// IndexItem delete，then inserts new vectors
	// so after config update, only changed items are reindexed，preserving other items indexes

	failedCount := 0
	consecutiveFailures := 0
	maxConsecutiveFailures := 5 // 5 stop（error）
	firstFailureItemID := ""
	var firstFailureError error

	for i, itemID := range itemIDs {
		if err := idx.IndexItem(ctx, itemID); err != nil {
			failedCount++
			consecutiveFailures++

			// record
			if consecutiveFailures == 1 {
				firstFailureItemID = itemID
				firstFailureError = err
				idx.logger.Warn("failed to index knowledge item",
					zap.String("itemId", itemID),
					zap.Int("totalItems", len(itemIDs)),
					zap.Error(err),
				)
			}

			// ，，stop
			if consecutiveFailures >= maxConsecutiveFailures {
				errorMsg := fmt.Sprintf(" %d ，possibly config issue（error、API 、）。：%s, error：%v", consecutiveFailures, firstFailureItemID, firstFailureError)
				idx.mu.Lock()
				idx.lastError = errorMsg
				idx.lastErrorTime = time.Now()
				idx.mu.Unlock()

				idx.logger.Error("too many consecutive index failures，stop",
					zap.Int("consecutiveFailures", consecutiveFailures),
					zap.Int("totalItems", len(itemIDs)),
					zap.Int("processedItems", i+1),
					zap.String("firstFailureItemId", firstFailureItemID),
					zap.Error(firstFailureError),
				)
				return fmt.Errorf("too many consecutive index failures：%v", firstFailureError)
			}

			// if too many knowledge items failed，recordcontinue（lower threshold to 30%）
			if failedCount > len(itemIDs)*3/10 && failedCount == len(itemIDs)*3/10+1 {
				errorMsg := fmt.Sprintf("too many knowledge items failed to index (%d/%d)，possibly config issue。：%s, error：%v", failedCount, len(itemIDs), firstFailureItemID, firstFailureError)
				idx.mu.Lock()
				idx.lastError = errorMsg
				idx.lastErrorTime = time.Now()
				idx.mu.Unlock()

				idx.logger.Error("too many knowledge items failed to index，possibly config issue",
					zap.Int("failedCount", failedCount),
					zap.Int("totalItems", len(itemIDs)),
					zap.String("firstFailureItemId", firstFailureItemID),
					zap.Error(firstFailureError),
				)
			}
			continue
		}

		// reset consecutive failure count on success
		if consecutiveFailures > 0 {
			consecutiveFailures = 0
			firstFailureItemID = ""
			firstFailureError = nil
		}

		// update rebuild progress
		idx.rebuildMu.Lock()
		idx.rebuildCurrent = i + 1
		idx.rebuildFailed = failedCount
		idx.rebuildMu.Unlock()

		// reduce progress log frequency（ 10 10% record）
		if (i+1)%10 == 0 || (len(itemIDs) > 0 && (i+1)*100/len(itemIDs)%10 == 0 && (i+1)*100/len(itemIDs) > 0) {
			idx.logger.Info("index progress", zap.Int("current", i+1), zap.Int("total", len(itemIDs)), zap.Int("failed", failedCount))
		}
	}

	// status
	idx.rebuildMu.Lock()
	idx.isRebuilding = false
	idx.rebuildMu.Unlock()

	idx.logger.Info("index rebuild complete", zap.Int("totalItems", len(itemIDs)), zap.Int("failedCount", failedCount))
	return nil
}

// GetLastError error
func (idx *Indexer) GetLastError() (string, time.Time) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.lastError, idx.lastErrorTime
}

// GetRebuildStatus rebuild indexstatus
func (idx *Indexer) GetRebuildStatus() (isRebuilding bool, totalItems int, current int, failed int, lastItemID string, lastChunks int, startTime time.Time) {
	idx.rebuildMu.RLock()
	defer idx.rebuildMu.RUnlock()
	return idx.isRebuilding, idx.rebuildTotalItems, idx.rebuildCurrent, idx.rebuildFailed, idx.rebuildLastItemID, idx.rebuildLastChunks, idx.rebuildStartTime
}
