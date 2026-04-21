package knowledge

import (
	"encoding/json"
	"time"
)

// formatTime format time as RFC3339, return empty string for zero time
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

// KnowledgeItem knowledge base
type KnowledgeItem struct {
	ID        string    `json:"id"`
	Category string `json:"category"` // type(folder name)
	Title     string    `json:"title"`    // title(filename)
	FilePath  string    `json:"filePath"` // file path
	Content   string    `json:"content"`  // file content
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// KnowledgeItemSummary knowledge base(list,)
type KnowledgeItemSummary struct {
	ID        string    `json:"id"`
	Category  string    `json:"category"`
	Title     string    `json:"title"`
	FilePath  string    `json:"filePath"`
	Content string `json:"content,omitempty"` // optional: content preview(, 150 )
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// MarshalJSON custom JSON serialization,time format
func (k *KnowledgeItemSummary) MarshalJSON() ([]byte, error) {
	type Alias KnowledgeItemSummary
	aux := &struct {
		*Alias
		CreatedAt string `json:"createdAt"`
		UpdatedAt string `json:"updatedAt"`
	}{
		Alias: (*Alias)(k),
	}
	aux.CreatedAt = formatTime(k.CreatedAt)
	aux.UpdatedAt = formatTime(k.UpdatedAt)
	return json.Marshal(aux)
}

// MarshalJSON custom JSON serialization,time format
func (k *KnowledgeItem) MarshalJSON() ([]byte, error) {
	type Alias KnowledgeItem
	aux := &struct {
		*Alias
		CreatedAt string `json:"createdAt"`
		UpdatedAt string `json:"updatedAt"`
	}{
		Alias: (*Alias)(k),
	}
	aux.CreatedAt = formatTime(k.CreatedAt)
	aux.UpdatedAt = formatTime(k.UpdatedAt)
	return json.Marshal(aux)
}

// KnowledgeChunk knowledge chunk (for vectorization)
type KnowledgeChunk struct {
	ID         string    `json:"id"`
	ItemID     string    `json:"itemId"`
	ChunkIndex int       `json:"chunkIndex"`
	ChunkText  string    `json:"chunkText"`
	Embedding  []float32 `json:"-"` // vector embedding, not serialized to JSON
	CreatedAt  time.Time `json:"createdAt"`
}

// RetrievalResult retrieval result
type RetrievalResult struct {
	Chunk      *KnowledgeChunk `json:"chunk"`
	Item       *KnowledgeItem  `json:"item"`
	Similarity float64         `json:"similarity"` // similarity score
	Score      float64         `json:"score"`      // composite score (hybrid retrieval)
}

// RetrievalLog retrieval log
type RetrievalLog struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversationId,omitempty"`
	MessageID      string    `json:"messageId,omitempty"`
	Query          string    `json:"query"`
	RiskType       string    `json:"riskType,omitempty"`
	RetrievedItems []string `json:"retrievedItems"` // ID list
	CreatedAt      time.Time `json:"createdAt"`
}

// MarshalJSON custom JSON serialization,time format
func (r *RetrievalLog) MarshalJSON() ([]byte, error) {
	type Alias RetrievalLog
	return json.Marshal(&struct {
		*Alias
		CreatedAt string `json:"createdAt"`
	}{
		Alias:     (*Alias)(r),
		CreatedAt: formatTime(r.CreatedAt),
	})
}

// CategoryWithItems category and its knowledge items(paginate by category)
type CategoryWithItems struct {
	Category  string                `json:"category"`           // category name
	ItemCount int                   `json:"itemCount"`          // total knowledge items in this category
	Items []*KnowledgeItemSummary `json:"items"` // list
}

// SearchRequest search request
type SearchRequest struct {
	Query     string  `json:"query"`
	RiskType string `json:"riskType,omitempty"` // :type
	TopK      int     `json:"topK,omitempty"`      // return Top-K results,default 5
	Threshold float64 `json:"threshold,omitempty"` // similarity threshold,default 0.7
}
