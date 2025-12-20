package knowledge

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Indexer 索引器，负责将知识项分块并向量化
type Indexer struct {
	db        *sql.DB
	embedder  *Embedder
	logger    *zap.Logger
	chunkSize int // 每个块的最大token数（估算）
	overlap   int // 块之间的重叠token数
}

// NewIndexer 创建新的索引器
func NewIndexer(db *sql.DB, embedder *Embedder, logger *zap.Logger) *Indexer {
	return &Indexer{
		db:        db,
		embedder:  embedder,
		logger:    logger,
		chunkSize: 512, // 默认512 tokens
		overlap:   50,  // 默认50 tokens重叠
	}
}

// ChunkText 将文本分块
func (idx *Indexer) ChunkText(text string) []string {
	// 按Markdown标题分割
	chunks := idx.splitByMarkdownHeaders(text)

	// 如果块太大，进一步分割
	result := make([]string, 0)
	for _, chunk := range chunks {
		if idx.estimateTokens(chunk) <= idx.chunkSize {
			result = append(result, chunk)
		} else {
			// 按段落分割
			subChunks := idx.splitByParagraphs(chunk)
			for _, subChunk := range subChunks {
				if idx.estimateTokens(subChunk) <= idx.chunkSize {
					result = append(result, subChunk)
				} else {
					// 按句子分割
					sentences := idx.splitBySentences(subChunk)
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
				}
			}
		}
	}

	return result
}

// splitByMarkdownHeaders 按Markdown标题分割
func (idx *Indexer) splitByMarkdownHeaders(text string) []string {
	// 匹配Markdown标题 (# ## ### 等)
	headerRegex := regexp.MustCompile(`(?m)^#{1,6}\s+.+$`)

	// 找到所有标题位置
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

	// 添加最后一部分
	if lastPos < len(text) {
		chunks = append(chunks, strings.TrimSpace(text[lastPos:]))
	}

	// 过滤空块
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

// splitByParagraphs 按段落分割
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

// splitBySentences 按句子分割
func (idx *Indexer) splitBySentences(text string) []string {
	// 简单的句子分割（按句号、问号、感叹号）
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

// estimateTokens 估算token数（简单估算：1 token ≈ 4字符）
func (idx *Indexer) estimateTokens(text string) int {
	return len([]rune(text)) / 4
}

// IndexItem 索引知识项（分块并向量化）
func (idx *Indexer) IndexItem(ctx context.Context, itemID string) error {
	// 获取知识项
	var content string
	err := idx.db.QueryRow("SELECT content FROM knowledge_base_items WHERE id = ?", itemID).Scan(&content)
	if err != nil {
		return fmt.Errorf("获取知识项失败: %w", err)
	}

	// 删除旧的向量
	_, err = idx.db.Exec("DELETE FROM knowledge_embeddings WHERE item_id = ?", itemID)
	if err != nil {
		return fmt.Errorf("删除旧向量失败: %w", err)
	}

	// 分块
	chunks := idx.ChunkText(content)
	idx.logger.Info("知识项分块完成", zap.String("itemId", itemID), zap.Int("chunks", len(chunks)))

	// 向量化每个块
	for i, chunk := range chunks {
		chunkPreview := chunk
		if len(chunkPreview) > 200 {
			chunkPreview = chunkPreview[:200] + "..."
		}
		embedding, err := idx.embedder.EmbedText(ctx, chunk)
		if err != nil {
			idx.logger.Warn("向量化失败",
				zap.String("itemId", itemID),
				zap.Int("chunkIndex", i),
				zap.Int("chunkLength", len(chunk)),
				zap.String("chunkPreview", chunkPreview),
				zap.Error(err),
			)
			continue
		}

		// 保存向量
		chunkID := uuid.New().String()
		embeddingJSON, _ := json.Marshal(embedding)

		_, err = idx.db.Exec(
			"INSERT INTO knowledge_embeddings (id, item_id, chunk_index, chunk_text, embedding, created_at) VALUES (?, ?, ?, ?, ?, datetime('now'))",
			chunkID, itemID, i, chunk, string(embeddingJSON),
		)
		if err != nil {
			idx.logger.Warn("保存向量失败", zap.String("itemId", itemID), zap.Int("chunkIndex", i), zap.Error(err))
			continue
		}
	}

	idx.logger.Info("知识项索引完成", zap.String("itemId", itemID), zap.Int("chunks", len(chunks)))
	return nil
}

// HasIndex 检查是否存在索引
func (idx *Indexer) HasIndex() (bool, error) {
	var count int
	err := idx.db.QueryRow("SELECT COUNT(*) FROM knowledge_embeddings").Scan(&count)
	if err != nil {
		return false, fmt.Errorf("检查索引失败: %w", err)
	}
	return count > 0, nil
}

// RebuildIndex 重建所有索引
func (idx *Indexer) RebuildIndex(ctx context.Context) error {
	rows, err := idx.db.Query("SELECT id FROM knowledge_base_items")
	if err != nil {
		return fmt.Errorf("查询知识项失败: %w", err)
	}
	defer rows.Close()

	var itemIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("扫描知识项ID失败: %w", err)
		}
		itemIDs = append(itemIDs, id)
	}

	idx.logger.Info("开始重建索引", zap.Int("totalItems", len(itemIDs)))

	for i, itemID := range itemIDs {
		if err := idx.IndexItem(ctx, itemID); err != nil {
			idx.logger.Warn("索引知识项失败", zap.String("itemId", itemID), zap.Error(err))
			continue
		}
		idx.logger.Debug("索引进度", zap.Int("current", i+1), zap.Int("total", len(itemIDs)))
	}

	idx.logger.Info("索引重建完成", zap.Int("totalItems", len(itemIDs)))
	return nil
}
