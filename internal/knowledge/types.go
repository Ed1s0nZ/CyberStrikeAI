package knowledge

import (
	"encoding/json"
	"time"
)

// KnowledgeItem 知识库项
type KnowledgeItem struct {
	ID        string    `json:"id"`
	Category  string    `json:"category"`  // 风险类型（文件夹名）
	Title     string    `json:"title"`      // 标题（文件名）
	FilePath  string    `json:"filePath"`  // 文件路径
	Content   string    `json:"content"`   // 文件内容
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// KnowledgeChunk 知识块（用于向量化）
type KnowledgeChunk struct {
	ID         string    `json:"id"`
	ItemID     string    `json:"itemId"`
	ChunkIndex int       `json:"chunkIndex"`
	ChunkText  string    `json:"chunkText"`
	Embedding  []float32 `json:"-"` // 向量嵌入，不序列化到JSON
	CreatedAt time.Time `json:"createdAt"`
}

// RetrievalResult 检索结果
type RetrievalResult struct {
	Chunk      *KnowledgeChunk `json:"chunk"`
	Item       *KnowledgeItem  `json:"item"`
	Similarity float64         `json:"similarity"` // 相似度分数
	Score      float64         `json:"score"`      // 综合分数（混合检索）
}

// RetrievalLog 检索日志
type RetrievalLog struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversationId,omitempty"`
	MessageID      string    `json:"messageId,omitempty"`
	Query          string    `json:"query"`
	RiskType       string    `json:"riskType,omitempty"`
	RetrievedItems []string  `json:"retrievedItems"` // 检索到的知识项ID列表
	CreatedAt      time.Time `json:"createdAt"`
}

// MarshalJSON 自定义JSON序列化，确保时间格式正确
func (r *RetrievalLog) MarshalJSON() ([]byte, error) {
	type Alias RetrievalLog
	return json.Marshal(&struct {
		*Alias
		CreatedAt string `json:"createdAt"`
	}{
		Alias:     (*Alias)(r),
		CreatedAt: r.CreatedAt.Format(time.RFC3339),
	})
}

// SearchRequest 搜索请求
type SearchRequest struct {
	Query     string `json:"query"`
	RiskType  string `json:"riskType,omitempty"`  // 可选：指定风险类型
	TopK      int    `json:"topK,omitempty"`      // 返回Top-K结果，默认5
	Threshold float64 `json:"threshold,omitempty"` // 相似度阈值，默认0.7
}

