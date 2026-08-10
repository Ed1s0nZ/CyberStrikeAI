package database

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// LLMUsage 单次模型调用 usage 记录（来源：eino 模型回调的 ResponseMeta.Usage，权威计费值）。
// 用于在任务/对话维度统计 Token 消耗，弥补 messages 表无 usage 列、前端无消耗可视化的缺口。
type LLMUsage struct {
	ID               string
	ConversationID   string
	Phase            string
	Model            string
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	CachedTokens     int
	ReasoningTokens  int
	CreatedAt        time.Time
}

// LLMUsageSummary 对话级 Token 消耗汇总（前端会话设置卡片展示用）。
type LLMUsageSummary struct {
	InputTokens     int       `json:"inputTokens"`
	OutputTokens    int       `json:"outputTokens"`
	TotalTokens     int       `json:"totalTokens"`
	Calls           int       `json:"calls"`
	CachedTokens    int       `json:"cachedTokens"`
	ReasoningTokens int       `json:"reasoningTokens"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// RecordLLMUsage 落库一次模型调用 usage；conversation_id 为空时跳过（后台任务无会话归属）。
func (db *DB) RecordLLMUsage(rec LLMUsage) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	if rec.ConversationID == "" {
		return nil
	}
	if rec.ID == "" {
		rec.ID = uuid.New().String()
	}
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = time.Now()
	}
	_, err := db.Exec(`
		INSERT INTO llm_usage (
			id, conversation_id, phase, model,
			prompt_tokens, completion_tokens, total_tokens, cached_tokens, reasoning_tokens,
			created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.ID, rec.ConversationID, rec.Phase, rec.Model,
		rec.PromptTokens, rec.CompletionTokens, rec.TotalTokens, rec.CachedTokens, rec.ReasoningTokens,
		rec.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("记录 LLM usage 失败: %w", err)
	}
	return nil
}

// GetConversationLLMUsage 汇总指定对话的 Token 消耗；无记录时返回零值汇总。
func (db *DB) GetConversationLLMUsage(conversationID string) (*LLMUsageSummary, error) {
	if db == nil {
		return nil, fmt.Errorf("db is nil")
	}
	var (
		summary   LLMUsageSummary
		updatedAt string
	)
	err := db.QueryRow(`
		SELECT
			COUNT(*),
			COALESCE(SUM(prompt_tokens), 0),
			COALESCE(SUM(completion_tokens), 0),
			COALESCE(SUM(total_tokens), 0),
			COALESCE(SUM(cached_tokens), 0),
			COALESCE(SUM(reasoning_tokens), 0),
			COALESCE(MAX(created_at), '')
		FROM llm_usage
		WHERE conversation_id = ?`,
		conversationID,
	).Scan(
		&summary.Calls,
		&summary.InputTokens,
		&summary.OutputTokens,
		&summary.TotalTokens,
		&summary.CachedTokens,
		&summary.ReasoningTokens,
		&updatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("查询对话 LLM usage 失败: %w", err)
	}
	if updatedAt != "" {
		if t, parseErr := time.Parse("2006-01-02 15:04:05", updatedAt); parseErr == nil {
			summary.UpdatedAt = t
		}
	}
	return &summary, nil
}

// DeleteLLMUsageByConversation 显式清理对话的 usage 记录（防外键级联未生效时残留）。
func (db *DB) DeleteLLMUsageByConversation(conversationID string) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	_, err := db.Exec(`DELETE FROM llm_usage WHERE conversation_id = ?`, conversationID)
	if err != nil {
		return fmt.Errorf("清理对话 LLM usage 失败: %w", err)
	}
	return nil
}
