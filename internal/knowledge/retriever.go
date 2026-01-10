package knowledge

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"go.uber.org/zap"
)

// Retriever 检索器
type Retriever struct {
	db       *sql.DB
	embedder *Embedder
	config   *RetrievalConfig
	logger   *zap.Logger
}

// RetrievalConfig 检索配置
type RetrievalConfig struct {
	TopK                int
	SimilarityThreshold float64
	HybridWeight        float64
}

// NewRetriever 创建新的检索器
func NewRetriever(db *sql.DB, embedder *Embedder, config *RetrievalConfig, logger *zap.Logger) *Retriever {
	return &Retriever{
		db:       db,
		embedder: embedder,
		config:   config,
		logger:   logger,
	}
}

// UpdateConfig 更新检索配置
func (r *Retriever) UpdateConfig(config *RetrievalConfig) {
	if config != nil {
		r.config = config
		r.logger.Info("检索器配置已更新",
			zap.Int("top_k", config.TopK),
			zap.Float64("similarity_threshold", config.SimilarityThreshold),
			zap.Float64("hybrid_weight", config.HybridWeight),
		)
	}
}

// cosineSimilarity 计算余弦相似度
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0.0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i] * b[i])
		normA += float64(a[i] * a[i])
		normB += float64(b[i] * b[i])
	}

	if normA == 0 || normB == 0 {
		return 0.0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// bm25Score 计算BM25分数（改进版，更接近标准BM25）
// 注意：这是单文档版本的BM25，缺少全局IDF，但比之前的简化版本更准确
func (r *Retriever) bm25Score(query, text string) float64 {
	queryTerms := strings.Fields(strings.ToLower(query))
	if len(queryTerms) == 0 {
		return 0.0
	}

	textLower := strings.ToLower(text)
	textTerms := strings.Fields(textLower)
	if len(textTerms) == 0 {
		return 0.0
	}

	// BM25参数
	k1 := 1.5             // 词频饱和度参数
	b := 0.75             // 长度归一化参数
	avgDocLength := 100.0 // 估算的平均文档长度（用于归一化）
	docLength := float64(len(textTerms))

	score := 0.0
	for _, term := range queryTerms {
		// 计算词频（TF）
		termFreq := 0
		for _, textTerm := range textTerms {
			if textTerm == term {
				termFreq++
			}
		}

		if termFreq > 0 {
			// BM25公式的核心部分
			// TF部分：termFreq / (termFreq + k1 * (1 - b + b * (docLength / avgDocLength)))
			tf := float64(termFreq)
			lengthNorm := 1 - b + b*(docLength/avgDocLength)
			tfScore := tf / (tf + k1*lengthNorm)

			// 简化IDF：使用词长度作为权重（短词通常更重要）
			// 实际BM25需要全局文档统计，这里用简化版本
			idfWeight := 1.0
			if len(term) > 2 {
				// 长词稍微降低权重（但实际BM25中，罕见词IDF更高）
				idfWeight = 1.0 + math.Log(1.0+float64(len(term))/10.0)
			}

			score += tfScore * idfWeight
		}
	}

	// 归一化到0-1范围
	if len(queryTerms) > 0 {
		score = score / float64(len(queryTerms))
	}

	return math.Min(score, 1.0)
}

// Search 搜索知识库
func (r *Retriever) Search(ctx context.Context, req *SearchRequest) ([]*RetrievalResult, error) {
	if req.Query == "" {
		return nil, fmt.Errorf("查询不能为空")
	}

	topK := req.TopK
	if topK <= 0 {
		topK = r.config.TopK
	}
	if topK == 0 {
		topK = 5
	}

	threshold := req.Threshold
	if threshold <= 0 {
		threshold = r.config.SimilarityThreshold
	}
	if threshold == 0 {
		threshold = 0.7
	}

	// 向量化查询（如果提供了risk_type，也包含在查询文本中，以便更好地匹配）
	queryText := req.Query
	if req.RiskType != "" {
		// 将risk_type信息包含到查询中，格式与索引时保持一致
		queryText = fmt.Sprintf("[风险类型: %s] %s", req.RiskType, req.Query)
	}
	queryEmbedding, err := r.embedder.EmbedText(ctx, queryText)
	if err != nil {
		return nil, fmt.Errorf("向量化查询失败: %w", err)
	}

	// 查询所有向量（或按风险类型过滤）
	// 使用精确匹配（=）以提高性能和准确性
	// 由于系统提供了内置工具来获取风险类型列表，用户应该使用准确的category名称
		// 同时，向量嵌入中已包含category信息，即使SQL过滤不完全匹配，向量相似度也能帮助匹配
		var rows *sql.Rows
		if req.RiskType != "" {
			// 使用精确匹配（=），性能更好且更准确
			// 使用 COLLATE NOCASE 实现大小写不敏感匹配，提高容错性
			// 注意：如果用户输入的risk_type与category不完全一致，可能匹配不到
			// 建议用户先调用相应的内置工具获取准确的category名称
		rows, err = r.db.Query(`
			SELECT e.id, e.item_id, e.chunk_index, e.chunk_text, e.embedding, i.category, i.title
			FROM knowledge_embeddings e
			JOIN knowledge_base_items i ON e.item_id = i.id
			WHERE i.category = ? COLLATE NOCASE
		`, req.RiskType)
	} else {
		rows, err = r.db.Query(`
			SELECT e.id, e.item_id, e.chunk_index, e.chunk_text, e.embedding, i.category, i.title
			FROM knowledge_embeddings e
			JOIN knowledge_base_items i ON e.item_id = i.id
		`)
	}
	if err != nil {
		return nil, fmt.Errorf("查询向量失败: %w", err)
	}
	defer rows.Close()

	// 计算相似度
	type candidate struct {
		chunk                 *KnowledgeChunk
		item                  *KnowledgeItem
		similarity            float64
		bm25Score             float64
		hasStrongKeywordMatch bool
		hybridScore           float64 // 混合分数，用于最终排序
	}

	candidates := make([]candidate, 0)

	for rows.Next() {
		var chunkID, itemID, chunkText, embeddingJSON, category, title string
		var chunkIndex int

		if err := rows.Scan(&chunkID, &itemID, &chunkIndex, &chunkText, &embeddingJSON, &category, &title); err != nil {
			r.logger.Warn("扫描向量失败", zap.Error(err))
			continue
		}

		// 解析向量
		var embedding []float32
		if err := json.Unmarshal([]byte(embeddingJSON), &embedding); err != nil {
			r.logger.Warn("解析向量失败", zap.Error(err))
			continue
		}

		// 计算余弦相似度
		similarity := cosineSimilarity(queryEmbedding, embedding)

		// 计算BM25分数（考虑chunk文本、category和title）
		// category和title是结构化字段，完全匹配时应该被优先考虑
		chunkBM25 := r.bm25Score(req.Query, chunkText)
		categoryBM25 := r.bm25Score(req.Query, category)
		titleBM25 := r.bm25Score(req.Query, title)

		// 检查category或title是否有显著匹配（这对于结构化字段很重要）
		hasStrongKeywordMatch := categoryBM25 > 0.3 || titleBM25 > 0.3

		// 综合BM25分数（用于后续排序）
		bm25Score := math.Max(math.Max(chunkBM25, categoryBM25), titleBM25)

		// 收集所有候选（先不严格过滤，以便后续智能处理跨语言情况）
		// 只过滤掉相似度极低的结果（< 0.1），避免噪音
		if similarity < 0.1 {
			continue
		}

		chunk := &KnowledgeChunk{
			ID:         chunkID,
			ItemID:     itemID,
			ChunkIndex: chunkIndex,
			ChunkText:  chunkText,
			Embedding:  embedding,
		}

		item := &KnowledgeItem{
			ID:       itemID,
			Category: category,
			Title:    title,
		}

		candidates = append(candidates, candidate{
			chunk:                 chunk,
			item:                  item,
			similarity:            similarity,
			bm25Score:             bm25Score,
			hasStrongKeywordMatch: hasStrongKeywordMatch,
		})
	}

	// 先按相似度排序（使用更高效的排序）
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].similarity > candidates[j].similarity
	})

	// 智能过滤策略：优先保留关键词匹配的结果，对跨语言查询使用更宽松的阈值
	filteredCandidates := make([]candidate, 0)

	// 检查是否有任何关键词匹配（用于判断是否是跨语言查询）
	hasAnyKeywordMatch := false
	for _, cand := range candidates {
		if cand.hasStrongKeywordMatch {
			hasAnyKeywordMatch = true
			break
		}
	}

	// 检查最高相似度，用于判断是否确实有相关内容
	maxSimilarity := 0.0
	if len(candidates) > 0 {
		maxSimilarity = candidates[0].similarity
	}

	// 应用智能过滤
	// 如果用户设置了高阈值（>=0.8），更严格地遵守阈值，减少自动放宽
	strictMode := threshold >= 0.8

	// 根据是否有关键词匹配，采用不同的阈值策略
	// 严格模式下，禁用跨语言放宽策略，严格遵守用户设置的阈值
	effectiveThreshold := threshold
	if !strictMode && !hasAnyKeywordMatch {
		// 非严格模式下，没有关键词匹配，可能是跨语言查询，适度放宽阈值
		// 但即使跨语言，也不能无脑降低阈值，需要保证最低相关性
		// 跨语言阈值设为0.6，确保返回的结果至少有一定相关性
		effectiveThreshold = math.Max(threshold*0.85, 0.6)
		r.logger.Debug("检测到可能的跨语言查询，使用放宽的阈值",
			zap.Float64("originalThreshold", threshold),
			zap.Float64("effectiveThreshold", effectiveThreshold),
		)
	} else if strictMode {
		// 严格模式下，即使没有关键词匹配，也严格遵守阈值
		r.logger.Debug("严格模式：严格遵守用户设置的阈值",
			zap.Float64("threshold", threshold),
			zap.Bool("hasKeywordMatch", hasAnyKeywordMatch),
		)
	}
	for _, cand := range candidates {
		if cand.similarity >= effectiveThreshold {
			// 达到阈值，直接通过
			filteredCandidates = append(filteredCandidates, cand)
		} else if !strictMode && cand.hasStrongKeywordMatch {
			// 非严格模式下，有关键词匹配但相似度略低于阈值，适当放宽
			// 严格模式下，即使有关键词匹配，也严格遵守阈值
			relaxedThreshold := math.Max(effectiveThreshold*0.85, 0.55)
			if cand.similarity >= relaxedThreshold {
				filteredCandidates = append(filteredCandidates, cand)
			}
		}
		// 如果既没有关键词匹配，相似度又低于阈值，则过滤掉
	}

	// 智能兜底策略：只有在最高相似度达到合理水平时，才考虑返回结果
	// 如果最高相似度都很低（<0.55），说明确实没有相关内容，应该返回空
	// 严格模式下（阈值>=0.8），禁用兜底策略，严格遵守用户设置的阈值
	if len(filteredCandidates) == 0 && len(candidates) > 0 && !strictMode {
		// 即使没有通过阈值过滤，如果最高相似度还可以（>=0.55），可以考虑返回Top-K
		// 但这是最后的兜底，只在确实有一定相关性时才使用
		// 严格模式下不使用兜底策略
		minAcceptableSimilarity := 0.55
		if maxSimilarity >= minAcceptableSimilarity {
			r.logger.Debug("过滤后无结果，但最高相似度可接受，返回Top-K结果",
				zap.Int("totalCandidates", len(candidates)),
				zap.Float64("maxSimilarity", maxSimilarity),
				zap.Float64("effectiveThreshold", effectiveThreshold),
			)
			maxResults := topK
			if len(candidates) < maxResults {
				maxResults = len(candidates)
			}
			// 只返回相似度 >= 0.55 的结果
			for _, cand := range candidates {
				if cand.similarity >= minAcceptableSimilarity && len(filteredCandidates) < maxResults {
					filteredCandidates = append(filteredCandidates, cand)
				}
			}
		} else {
			r.logger.Debug("过滤后无结果，且最高相似度过低，返回空结果",
				zap.Int("totalCandidates", len(candidates)),
				zap.Float64("maxSimilarity", maxSimilarity),
				zap.Float64("minAcceptableSimilarity", minAcceptableSimilarity),
			)
		}
	} else if len(filteredCandidates) == 0 && strictMode {
		// 严格模式下，如果过滤后无结果，直接返回空，不使用兜底策略
		r.logger.Debug("严格模式：过滤后无结果，严格遵守阈值，返回空结果",
			zap.Float64("threshold", threshold),
			zap.Float64("maxSimilarity", maxSimilarity),
		)
	} else if len(filteredCandidates) > topK {
		// 如果过滤后结果太多，只取Top-K
		filteredCandidates = filteredCandidates[:topK]
	}

	candidates = filteredCandidates

	// 混合排序（向量相似度 + BM25）
	// 注意：hybridWeight可以是0.0（纯关键词检索），所以不设置默认值
	// 如果配置文件中未设置，应该在配置加载时使用默认值
	hybridWeight := r.config.HybridWeight
	// 如果未设置，使用默认值0.7（偏重向量检索）
	if hybridWeight < 0 || hybridWeight > 1 {
		r.logger.Warn("混合权重超出范围，使用默认值0.7",
			zap.Float64("provided", hybridWeight))
		hybridWeight = 0.7
	}

	// 先计算混合分数并存储在candidate中，用于排序
	for i := range candidates {
		normalizedBM25 := math.Min(candidates[i].bm25Score, 1.0)
		candidates[i].hybridScore = hybridWeight*candidates[i].similarity + (1-hybridWeight)*normalizedBM25

		// 调试日志：记录前几个候选的分数计算（仅在debug级别）
		if i < 3 {
			r.logger.Debug("混合分数计算",
				zap.Int("index", i),
				zap.Float64("similarity", candidates[i].similarity),
				zap.Float64("bm25Score", candidates[i].bm25Score),
				zap.Float64("normalizedBM25", normalizedBM25),
				zap.Float64("hybridWeight", hybridWeight),
				zap.Float64("hybridScore", candidates[i].hybridScore))
		}
	}

	// 根据混合分数重新排序（这才是真正的混合检索）
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].hybridScore > candidates[j].hybridScore
	})

	// 转换为结果
	results := make([]*RetrievalResult, len(candidates))
	for i, cand := range candidates {
		results[i] = &RetrievalResult{
			Chunk:      cand.chunk,
			Item:       cand.item,
			Similarity: cand.similarity,
			Score:      cand.hybridScore,
		}
	}

	// 上下文扩展：为每个匹配的chunk添加同一文档中的相关chunk
	// 这可以防止文本描述和payload被分开切分时，只返回描述而丢失payload的问题
	results = r.expandContext(ctx, results)

	return results, nil
}

// expandContext 扩展检索结果的上下文
// 对于每个匹配的chunk，自动包含同一文档中的相关chunk（特别是包含代码块、payload的chunk）
func (r *Retriever) expandContext(ctx context.Context, results []*RetrievalResult) []*RetrievalResult {
	if len(results) == 0 {
		return results
	}

	// 收集所有匹配到的文档ID
	itemIDs := make(map[string]bool)
	for _, result := range results {
		itemIDs[result.Item.ID] = true
	}

	// 为每个文档加载所有chunk
	itemChunksMap := make(map[string][]*KnowledgeChunk)
	for itemID := range itemIDs {
		chunks, err := r.loadAllChunksForItem(itemID)
		if err != nil {
			r.logger.Warn("加载文档chunk失败", zap.String("itemId", itemID), zap.Error(err))
			continue
		}
		itemChunksMap[itemID] = chunks
	}

	// 按文档分组结果，每个文档只扩展一次
	resultsByItem := make(map[string][]*RetrievalResult)
	for _, result := range results {
		itemID := result.Item.ID
		resultsByItem[itemID] = append(resultsByItem[itemID], result)
	}

	// 扩展每个文档的结果
	expandedResults := make([]*RetrievalResult, 0, len(results))
	processedChunkIDs := make(map[string]bool) // 避免重复添加

	for itemID, itemResults := range resultsByItem {
		// 获取该文档的所有chunk
		allChunks, exists := itemChunksMap[itemID]
		if !exists {
			// 如果无法加载chunk，直接添加原始结果
			for _, result := range itemResults {
				if !processedChunkIDs[result.Chunk.ID] {
					expandedResults = append(expandedResults, result)
					processedChunkIDs[result.Chunk.ID] = true
				}
			}
			continue
		}

		// 添加原始结果
		for _, result := range itemResults {
			if !processedChunkIDs[result.Chunk.ID] {
				expandedResults = append(expandedResults, result)
				processedChunkIDs[result.Chunk.ID] = true
			}
		}

		// 为该文档的匹配chunk收集需要扩展的相邻chunk
		// 策略：只对混合分数最高的前3个匹配chunk进行扩展，避免扩展过多
		// 先按混合分数排序，只扩展前3个（使用混合分数而不是相似度）
		sortedItemResults := make([]*RetrievalResult, len(itemResults))
		copy(sortedItemResults, itemResults)
		sort.Slice(sortedItemResults, func(i, j int) bool {
			return sortedItemResults[i].Score > sortedItemResults[j].Score
		})

		// 只扩展前3个（或所有，如果少于3个）
		maxExpandFrom := 3
		if len(sortedItemResults) < maxExpandFrom {
			maxExpandFrom = len(sortedItemResults)
		}

		// 使用map去重，避免同一个chunk被多次添加
		relatedChunksMap := make(map[string]*KnowledgeChunk)

		for i := 0; i < maxExpandFrom; i++ {
			result := sortedItemResults[i]
			// 查找相关chunk（上下各2个，排除已处理的chunk）
			relatedChunks := r.findRelatedChunks(result.Chunk, allChunks, processedChunkIDs)
			for _, relatedChunk := range relatedChunks {
				// 使用chunk ID作为key去重
				if !processedChunkIDs[relatedChunk.ID] {
					relatedChunksMap[relatedChunk.ID] = relatedChunk
				}
			}
		}

		// 限制每个文档最多扩展的chunk数量（避免扩展过多）
		// 策略：最多扩展8个chunk，无论匹配了多少个chunk
		// 这样可以避免当多个匹配chunk分散在文档不同位置时，扩展出过多chunk
		maxExpandPerItem := 8

		// 将相关chunk转换为切片并按索引排序，优先选择距离匹配chunk最近的
		relatedChunksList := make([]*KnowledgeChunk, 0, len(relatedChunksMap))
		for _, chunk := range relatedChunksMap {
			relatedChunksList = append(relatedChunksList, chunk)
		}

		// 计算每个相关chunk到最近匹配chunk的距离，按距离排序
		sort.Slice(relatedChunksList, func(i, j int) bool {
			// 计算到最近匹配chunk的距离
			minDistI := len(allChunks)
			minDistJ := len(allChunks)
			for _, result := range itemResults {
				distI := abs(relatedChunksList[i].ChunkIndex - result.Chunk.ChunkIndex)
				distJ := abs(relatedChunksList[j].ChunkIndex - result.Chunk.ChunkIndex)
				if distI < minDistI {
					minDistI = distI
				}
				if distJ < minDistJ {
					minDistJ = distJ
				}
			}
			return minDistI < minDistJ
		})

		// 限制数量
		if len(relatedChunksList) > maxExpandPerItem {
			relatedChunksList = relatedChunksList[:maxExpandPerItem]
		}

		// 添加去重后的相关chunk
		// 使用该文档中混合分数最高的结果作为参考
		maxScore := 0.0
		maxSimilarity := 0.0
		for _, result := range itemResults {
			if result.Score > maxScore {
				maxScore = result.Score
			}
			if result.Similarity > maxSimilarity {
				maxSimilarity = result.Similarity
			}
		}

		// 计算扩展chunk的混合分数（使用相同的混合权重）
		hybridWeight := r.config.HybridWeight
		expandedSimilarity := maxSimilarity * 0.8 // 相关chunk的相似度略低
		// 对于扩展的chunk，BM25分数设为0（因为它们是上下文扩展，不是直接匹配）
		expandedBM25 := 0.0
		expandedScore := hybridWeight*expandedSimilarity + (1-hybridWeight)*expandedBM25

		for _, relatedChunk := range relatedChunksList {
			expandedResult := &RetrievalResult{
				Chunk:      relatedChunk,
				Item:       itemResults[0].Item, // 使用第一个结果的Item信息
				Similarity: expandedSimilarity,
				Score:      expandedScore, // 使用正确的混合分数
			}
			expandedResults = append(expandedResults, expandedResult)
			processedChunkIDs[relatedChunk.ID] = true
		}
	}

	return expandedResults
}

// loadAllChunksForItem 加载文档的所有chunk
func (r *Retriever) loadAllChunksForItem(itemID string) ([]*KnowledgeChunk, error) {
	rows, err := r.db.Query(`
		SELECT id, item_id, chunk_index, chunk_text, embedding
		FROM knowledge_embeddings
		WHERE item_id = ?
		ORDER BY chunk_index
	`, itemID)
	if err != nil {
		return nil, fmt.Errorf("查询chunk失败: %w", err)
	}
	defer rows.Close()

	var chunks []*KnowledgeChunk
	for rows.Next() {
		var chunkID, itemID, chunkText, embeddingJSON string
		var chunkIndex int

		if err := rows.Scan(&chunkID, &itemID, &chunkIndex, &chunkText, &embeddingJSON); err != nil {
			r.logger.Warn("扫描chunk失败", zap.Error(err))
			continue
		}

		// 解析向量（可选，这里不需要）
		var embedding []float32
		if embeddingJSON != "" {
			json.Unmarshal([]byte(embeddingJSON), &embedding)
		}

		chunk := &KnowledgeChunk{
			ID:         chunkID,
			ItemID:     itemID,
			ChunkIndex: chunkIndex,
			ChunkText:  chunkText,
			Embedding:  embedding,
		}
		chunks = append(chunks, chunk)
	}

	return chunks, nil
}

// findRelatedChunks 查找与给定chunk相关的其他chunk
// 策略：只返回上下各2个相邻的chunk（共最多4个）
// 排除已处理的chunk，避免重复添加
func (r *Retriever) findRelatedChunks(targetChunk *KnowledgeChunk, allChunks []*KnowledgeChunk, processedChunkIDs map[string]bool) []*KnowledgeChunk {
	related := make([]*KnowledgeChunk, 0)

	// 查找上下各2个相邻chunk
	for _, chunk := range allChunks {
		if chunk.ID == targetChunk.ID {
			continue
		}

		// 检查是否已经被处理过（可能已经在检索结果中）
		if processedChunkIDs[chunk.ID] {
			continue
		}

		// 检查是否是相邻chunk（索引相差不超过2，且不为0）
		indexDiff := chunk.ChunkIndex - targetChunk.ChunkIndex
		if indexDiff >= -2 && indexDiff <= 2 && indexDiff != 0 {
			related = append(related, chunk)
		}
	}

	// 按索引距离排序，优先选择最近的
	sort.Slice(related, func(i, j int) bool {
		diffI := abs(related[i].ChunkIndex - targetChunk.ChunkIndex)
		diffJ := abs(related[j].ChunkIndex - targetChunk.ChunkIndex)
		return diffI < diffJ
	})

	// 限制最多返回4个（上下各2个）
	if len(related) > 4 {
		related = related[:4]
	}

	return related
}

// abs 返回整数的绝对值
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
