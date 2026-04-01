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

// Retriever retriever
type Retriever struct {
	db       *sql.DB
	embedder *Embedder
	config   *RetrievalConfig
	logger   *zap.Logger
}

// RetrievalConfig 
type RetrievalConfig struct {
	TopK                int
	SimilarityThreshold float64
	HybridWeight        float64
}

// NewRetriever creates a new retriever
func NewRetriever(db *sql.DB, embedder *Embedder, config *RetrievalConfig, logger *zap.Logger) *Retriever {
	return &Retriever{
		db:       db,
		embedder: embedder,
		config:   config,
		logger:   logger,
	}
}

// UpdateConfig update retrieval config
func (r *Retriever) UpdateConfig(config *RetrievalConfig) {
	if config != nil {
		r.config = config
		r.logger.Info("retrieverconfig updated",
			zap.Int("top_k", config.TopK),
			zap.Float64("similarity_threshold", config.SimilarityThreshold),
			zap.Float64("hybrid_weight", config.HybridWeight),
		)
	}
}

// cosineSimilarity cosine similarity
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

// bm25Score BM25 score（version）
// ：lacking global document statistics，use simplified IDF calculation
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

	// BM25 parameters (standard values)
	k1 := 1.2             // term frequency saturation parameter（standard range 1.2-2.0）
	b := 0.75             // length normalization parameter（standard value）
	avgDocLength := 150.0 // estimated average document length（based on typical knowledge chunk size）
	docLength := float64(len(textTerms))

	// calculate term frequency mapping
	textTermFreq := make(map[string]int, len(textTerms))
	for _, term := range textTerms {
		textTermFreq[term]++
	}

	score := 0.0
	matchedQueryTerms := 0

	for _, term := range queryTerms {
		termFreq, exists := textTermFreq[term]
		if !exists || termFreq == 0 {
			continue
		}
		matchedQueryTerms++

		// BM25 TF formula
		tf := float64(termFreq)
		lengthNorm := 1 - b + b*(docLength/avgDocLength)
		tfScore := tf / (tf + k1*lengthNorm)

		// improved IDF calculation：estimate using word length and frequency
		// short words（2-3 ）usually more important，long words IDF 
		idfWeight := 1.0
		termLen := len(term)
		if termLen <= 2 {
			// short words（ go, js）give higher weight
			idfWeight = 1.2 + math.Log(1.0+float64(termFreq)/20.0)
		} else if termLen <= 4 {
			// short words（4 ）
			idfWeight = 1.0 + math.Log(1.0+float64(termFreq)/15.0)
		} else {
			// long wordsslightly lower weight
			idfWeight = 0.9 + math.Log(1.0+float64(termFreq)/10.0)
		}

		score += tfScore * idfWeight
	}

	// normalization：match
	if len(queryTerms) > 0 {
		// use match ratio as additional factor
		matchRatio := float64(matchedQueryTerms) / float64(len(queryTerms))
		score = (score / float64(len(queryTerms))) * (1 + matchRatio) / 2
	}

	return math.Min(score, 1.0)
}

// Search knowledge base
func (r *Retriever) Search(ctx context.Context, req *SearchRequest) ([]*RetrievalResult, error) {
	if req.Query == "" {
		return nil, fmt.Errorf("query cannot be empty")
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

	// vectorize query（risk_type，，match）
	queryText := req.Query
	if req.RiskType != "" {
		// include risk_type in query，format consistent with indexing
		queryText = fmt.Sprintf("[type: %s] %s", req.RiskType, req.Query)
	}
	queryEmbedding, err := r.embedder.EmbedText(ctx, queryText)
	if err != nil {
		return nil, fmt.Errorf("vectorize query: %w", err)
	}

	// query all vectors（type）
	// use exact match（=）improve performance and accuracy
	// typelist，category
		// ，category，SQLmatch，helpmatch
		var rows *sql.Rows
		if req.RiskType != "" {
			// use exact match（=），
			// COLLATE NOCASE case-insensitive match，improve fault tolerance
			// ：risk_typecategory，match
			// category
		rows, err = r.db.Query(`
			SELECT e.id, e.item_id, e.chunk_index, e.chunk_text, e.embedding, i.category, i.title
			FROM knowledge_embeddings e
			JOIN knowledge_base_items i ON e.item_id = i.id
			WHERE TRIM(i.category) = TRIM(?) COLLATE NOCASE
		`, req.RiskType)
	} else {
		rows, err = r.db.Query(`
			SELECT e.id, e.item_id, e.chunk_index, e.chunk_text, e.embedding, i.category, i.title
			FROM knowledge_embeddings e
			JOIN knowledge_base_items i ON e.item_id = i.id
		`)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query vectors: %w", err)
	}
	defer rows.Close()

	// calculate similarity
	type candidate struct {
		chunk                 *KnowledgeChunk
		item                  *KnowledgeItem
		similarity            float64
		bm25Score             float64
		hasStrongKeywordMatch bool
		hybridScore           float64 // hybrid score，for final sorting
	}

	candidates := make([]candidate, 0)

	for rows.Next() {
		var chunkID, itemID, chunkText, embeddingJSON, category, title string
		var chunkIndex int

		if err := rows.Scan(&chunkID, &itemID, &chunkIndex, &chunkText, &embeddingJSON, &category, &title); err != nil {
			r.logger.Warn("scan", zap.Error(err))
			continue
		}

		// parse
		var embedding []float32
		if err := json.Unmarshal([]byte(embeddingJSON), &embedding); err != nil {
			r.logger.Warn("parse", zap.Error(err))
			continue
		}

		// cosine similarity
		similarity := cosineSimilarity(queryEmbedding, embedding)

		// calculate BM25 score（considering chunk text, category, and title）
		// categorytitle，match
		chunkBM25 := r.bm25Score(req.Query, chunkText)
		categoryBM25 := r.bm25Score(req.Query, category)
		titleBM25 := r.bm25Score(req.Query, title)

		// check if category or title has significant match（）
		hasStrongKeywordMatch := categoryBM25 > 0.3 || titleBM25 > 0.3

		// composite BM25 score（）
		bm25Score := math.Max(math.Max(chunkBM25, categoryBM25), titleBM25)

		// collect all candidates（do not strictly filter yet，intelligently handle cross-language cases）
		// only filter out very low similarity results（< 0.1），avoid noise
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

	// sort by similarity first（use more efficient sorting）
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].similarity > candidates[j].similarity
	})

	// intelligent filtering strategy：prioritize keyword-matched results，use more relaxed threshold for cross-language queries
	filteredCandidates := make([]candidate, 0)

	// check if there are any keyword matches（to determine if it is a cross-language query）
	hasAnyKeywordMatch := false
	for _, cand := range candidates {
		if cand.hasStrongKeywordMatch {
			hasAnyKeywordMatch = true
			break
		}
	}

	// check highest similarity，to determine if there is actually relevant content
	maxSimilarity := 0.0
	if len(candidates) > 0 {
		maxSimilarity = candidates[0].similarity
	}

	// apply intelligent filtering
	// if user set high threshold（>=0.8），more strictly follow threshold，reduce automatic relaxation
	strictMode := threshold >= 0.8

	// based on whether there are keyword matches，use different threshold strategies
	// in strict mode，disable cross-language relaxation strategy，strictly follow user-set threshold
	effectiveThreshold := threshold
	if !strictMode && !hasAnyKeywordMatch {
		// in strict mode，no keyword matches，may be cross-language query，moderately relax threshold
		// ，，
		// cross-language threshold set to0.6，ensure results have at least some relevance
		effectiveThreshold = math.Max(threshold*0.85, 0.6)
		r.logger.Debug("detected possible cross-language query，using relaxed threshold",
			zap.Float64("originalThreshold", threshold),
			zap.Float64("effectiveThreshold", effectiveThreshold),
		)
	} else if strictMode {
		// in strict mode，no keyword matches，
		r.logger.Debug("：strictly follow user-set threshold",
			zap.Float64("threshold", threshold),
			zap.Bool("hasKeywordMatch", hasAnyKeywordMatch),
		)
	}
	for _, cand := range candidates {
		if cand.similarity >= effectiveThreshold {
			// reached threshold, pass directly
			filteredCandidates = append(filteredCandidates, cand)
		} else if !strictMode && cand.hasStrongKeywordMatch {
			// in strict mode，has keyword match but similarity slightly below threshold，appropriately relax
			// in strict mode，even with keyword matches, strictly follow threshold
			relaxedThreshold := math.Max(effectiveThreshold*0.85, 0.55)
			if cand.similarity >= relaxedThreshold {
				filteredCandidates = append(filteredCandidates, cand)
			}
		}
		// no keyword matches，，
	}

	// intelligent fallback strategy：only when highest similarity reaches reasonable level，consider returning results
	// if highest similarity is very low（<0.55），indicates there is truly no relevant content，should return empty
	// in strict mode（>=0.8），disable fallback strategy，strictly follow user-set threshold
	if len(filteredCandidates) == 0 && len(candidates) > 0 && !strictMode {
		// ，（>=0.55），returnsTop-K
		// ，
		// in strict modedo not use fallback strategy
		minAcceptableSimilarity := 0.55
		if maxSimilarity >= minAcceptableSimilarity {
			r.logger.Debug("no results after filtering，but highest similarity acceptable，return Top-K results",
				zap.Int("totalCandidates", len(candidates)),
				zap.Float64("maxSimilarity", maxSimilarity),
				zap.Float64("effectiveThreshold", effectiveThreshold),
			)
			maxResults := topK
			if len(candidates) < maxResults {
				maxResults = len(candidates)
			}
			// only return similarity >= 0.55 
			for _, cand := range candidates {
				if cand.similarity >= minAcceptableSimilarity && len(filteredCandidates) < maxResults {
					filteredCandidates = append(filteredCandidates, cand)
				}
			}
		} else {
			r.logger.Debug("no results after filtering，and highest similarity too low，return empty results",
				zap.Int("totalCandidates", len(candidates)),
				zap.Float64("maxSimilarity", maxSimilarity),
				zap.Float64("minAcceptableSimilarity", minAcceptableSimilarity),
			)
		}
	} else if len(filteredCandidates) == 0 && strictMode {
		// in strict mode，no results after filtering，returns，do not use fallback strategy
		r.logger.Debug("：no results after filtering，，return empty results",
			zap.Float64("threshold", threshold),
			zap.Float64("maxSimilarity", maxSimilarity),
		)
	}

	// returns Top-K 
	if len(filteredCandidates) > topK {
		// if too many results after filtering，only take Top-K
		filteredCandidates = filteredCandidates[:topK]
	}

	candidates = filteredCandidates

	// hybrid sorting（vector similarity + BM25）
	// ：hybridWeight0.0（），default value
	// ，loaddefault value
	hybridWeight := r.config.HybridWeight
	// if not set，default value0.7（biased towards vector retrieval）
	if hybridWeight < 0 || hybridWeight > 1 {
		r.logger.Warn("hybrid weight out of range，default value0.7",
			zap.Float64("provided", hybridWeight))
		hybridWeight = 0.7
	}

	// hybrid scoreand store in candidate，
	for i := range candidates {
		normalizedBM25 := math.Min(candidates[i].bm25Score, 1.0)
		candidates[i].hybridScore = hybridWeight*candidates[i].similarity + (1-hybridWeight)*normalizedBM25

		// debug log：record（only at debug level）
		if i < 3 {
			r.logger.Debug("hybrid score",
				zap.Int("index", i),
				zap.Float64("similarity", candidates[i].similarity),
				zap.Float64("bm25Score", candidates[i].bm25Score),
				zap.Float64("normalizedBM25", normalizedBM25),
				zap.Float64("hybridWeight", hybridWeight),
				zap.Float64("hybridScore", candidates[i].hybridScore))
		}
	}

	// hybrid score（this is the actual hybrid retrieval）
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].hybridScore > candidates[j].hybridScore
	})

	// convert to results
	results := make([]*RetrievalResult, len(candidates))
	for i, cand := range candidates {
		results[i] = &RetrievalResult{
			Chunk:      cand.chunk,
			Item:       cand.item,
			Similarity: cand.similarity,
			Score:      cand.hybridScore,
		}
	}

	// context expansion：matchchunkaddchunk
	// prevent text descriptions and payloads from being split apart，returning only descriptions and losing payloads
	results = r.expandContext(ctx, results)

	return results, nil
}

// expandContext retrieval result
// matchchunk，auto-include related chunks from same document（especially chunks containing code blocks and payloads）
func (r *Retriever) expandContext(ctx context.Context, results []*RetrievalResult) []*RetrievalResult {
	if len(results) == 0 {
		return results
	}

	// collect all matched document IDs
	itemIDs := make(map[string]bool)
	for _, result := range results {
		itemIDs[result.Item.ID] = true
	}

	// loadchunk
	itemChunksMap := make(map[string][]*KnowledgeChunk)
	for itemID := range itemIDs {
		chunks, err := r.loadAllChunksForItem(itemID)
		if err != nil {
			r.logger.Warn("loadchunk", zap.String("itemId", itemID), zap.Error(err))
			continue
		}
		itemChunksMap[itemID] = chunks
	}

	// group results by document，expand each document only once
	resultsByItem := make(map[string][]*RetrievalResult)
	for _, result := range results {
		itemID := result.Item.ID
		resultsByItem[itemID] = append(resultsByItem[itemID], result)
	}

	// expand results for each document
	expandedResults := make([]*RetrievalResult, 0, len(results))
	processedChunkIDs := make(map[string]bool) // add

	for itemID, itemResults := range resultsByItem {
		// get all chunks for this document
		allChunks, exists := itemChunksMap[itemID]
		if !exists {
			// loadchunk，add
			for _, result := range itemResults {
				if !processedChunkIDs[result.Chunk.ID] {
					expandedResults = append(expandedResults, result)
					processedChunkIDs[result.Chunk.ID] = true
				}
			}
			continue
		}

		// add
		for _, result := range itemResults {
			if !processedChunkIDs[result.Chunk.ID] {
				expandedResults = append(expandedResults, result)
				processedChunkIDs[result.Chunk.ID] = true
			}
		}

		// collect adjacent chunks to expand for matched chunks
		// ：hybrid score3matchchunk，avoid excessive expansion
		// hybrid score，3（hybrid score）
		sortedItemResults := make([]*RetrievalResult, len(itemResults))
		copy(sortedItemResults, itemResults)
		sort.Slice(sortedItemResults, func(i, j int) bool {
			return sortedItemResults[i].Score > sortedItemResults[j].Score
		})

		// 3（，3）
		maxExpandFrom := 3
		if len(sortedItemResults) < maxExpandFrom {
			maxExpandFrom = len(sortedItemResults)
		}

		// use map for deduplication，chunkadd
		relatedChunksMap := make(map[string]*KnowledgeChunk)

		for i := 0; i < maxExpandFrom; i++ {
			result := sortedItemResults[i]
			// find related chunks（up to 2 above and below，exclude already processed chunks）
			relatedChunks := r.findRelatedChunks(result.Chunk, allChunks, processedChunkIDs)
			for _, relatedChunk := range relatedChunks {
				// use chunk ID as dedup key
				if !processedChunkIDs[relatedChunk.ID] {
					relatedChunksMap[relatedChunk.ID] = relatedChunk
				}
			}
		}

		// limit max expanded chunks per document（avoid excessive expansion）
		// ：expand up to 8 chunks，regardless of how many chunks matched
		// when matched chunks are scattered across document，expand too many chunks
		maxExpandPerItem := 8

		// chunksort by index，prefer closest to matched chunks
		relatedChunksList := make([]*KnowledgeChunk, 0, len(relatedChunksMap))
		for _, chunk := range relatedChunksMap {
			relatedChunksList = append(relatedChunksList, chunk)
		}

		// calculate distance of each related chunk to nearest matched chunk，sort by distance
		sort.Slice(relatedChunksList, func(i, j int) bool {
			// matchchunk
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

		// limit count
		if len(relatedChunksList) > maxExpandPerItem {
			relatedChunksList = relatedChunksList[:maxExpandPerItem]
		}

		// addchunk
		// hybrid score
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

		// chunkhybrid score（）
		hybridWeight := r.config.HybridWeight
		expandedSimilarity := maxSimilarity * 0.8 // related chunk similarity slightly lower
		// for expanded chunks, BM25 score set to 0（context expansion，match）
		expandedBM25 := 0.0
		expandedScore := hybridWeight*expandedSimilarity + (1-hybridWeight)*expandedBM25

		for _, relatedChunk := range relatedChunksList {
			expandedResult := &RetrievalResult{
				Chunk:      relatedChunk,
				Item: itemResults[0].Item, // Item
				Similarity: expandedSimilarity,
				Score: expandedScore, // hybrid score
			}
			expandedResults = append(expandedResults, expandedResult)
			processedChunkIDs[relatedChunk.ID] = true
		}
	}

	return expandedResults
}

// loadAllChunksForItem loadchunk
func (r *Retriever) loadAllChunksForItem(itemID string) ([]*KnowledgeChunk, error) {
	rows, err := r.db.Query(`
		SELECT id, item_id, chunk_index, chunk_text, embedding
		FROM knowledge_embeddings
		WHERE item_id = ?
		ORDER BY chunk_index
	`, itemID)
	if err != nil {
		return nil, fmt.Errorf("failed to query chunks: %w", err)
	}
	defer rows.Close()

	var chunks []*KnowledgeChunk
	for rows.Next() {
		var chunkID, itemID, chunkText, embeddingJSON string
		var chunkIndex int

		if err := rows.Scan(&chunkID, &itemID, &chunkIndex, &chunkText, &embeddingJSON); err != nil {
			r.logger.Warn("scanchunk", zap.Error(err))
			continue
		}

		// parse（，）
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

// findRelatedChunks find other chunks related to given chunk
// ：returnsup to 2 above and belowchunk（4）
// exclude already processed chunks，add
func (r *Retriever) findRelatedChunks(targetChunk *KnowledgeChunk, allChunks []*KnowledgeChunk, processedChunkIDs map[string]bool) []*KnowledgeChunk {
	related := make([]*KnowledgeChunk, 0)

	// up to 2 above and belowchunk
	for _, chunk := range allChunks {
		if chunk.ID == targetChunk.ID {
			continue
		}

		// check if already processed（retrieval result）
		if processedChunkIDs[chunk.ID] {
			continue
		}

		// check if adjacent chunk（index difference no more than 2，and not 0）
		indexDiff := chunk.ChunkIndex - targetChunk.ChunkIndex
		if indexDiff >= -2 && indexDiff <= 2 && indexDiff != 0 {
			related = append(related, chunk)
		}
	}

	// sort by index distance，prefer nearest
	sort.Slice(related, func(i, j int) bool {
		diffI := abs(related[i].ChunkIndex - targetChunk.ChunkIndex)
		diffJ := abs(related[j].ChunkIndex - targetChunk.ChunkIndex)
		return diffI < diffJ
	})

	// limit to max 4 returns（up to 2 above and below）
	if len(related) > 4 {
		related = related[:4]
	}

	return related
}

// abs return absolute value of integer
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
