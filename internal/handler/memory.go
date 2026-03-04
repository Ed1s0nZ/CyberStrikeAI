package handler

import (
	"net/http"
	"strconv"
	"strings"

	"cyberstrike-ai/internal/agent"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// MemoryHandler provides HTTP handlers for the persistent memory CRUD API.
type MemoryHandler struct {
	memory *agent.PersistentMemory
	logger *zap.Logger
}

// NewMemoryHandler creates a MemoryHandler backed by the given PersistentMemory.
func NewMemoryHandler(memory *agent.PersistentMemory, logger *zap.Logger) *MemoryHandler {
	return &MemoryHandler{memory: memory, logger: logger}
}

// ListMemories handles GET /api/memories
// Query params: category (optional), limit (default 100), search (optional text search)
func (h *MemoryHandler) ListMemories(c *gin.Context) {
	categoryStr := c.Query("category")
	search := c.Query("search")
	limitStr := c.DefaultQuery("limit", "100")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	cat := agent.MemoryCategory(strings.TrimSpace(categoryStr))

	var entries []*agent.MemoryEntry
	if search != "" {
		entries, err = h.memory.Retrieve(search, cat, limit)
	} else {
		entries, err = h.memory.List(cat, limit)
	}
	if err != nil {
		h.logger.Error("failed to list memories", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if entries == nil {
		entries = []*agent.MemoryEntry{}
	}

	c.JSON(http.StatusOK, gin.H{
		"entries": entries,
		"total":   len(entries),
	})
}

// GetMemoryStats handles GET /api/memories/stats
// Returns counts per category and total.
func (h *MemoryHandler) GetMemoryStats(c *gin.Context) {
	categories := []agent.MemoryCategory{
		agent.MemoryCategoryCredential,
		agent.MemoryCategoryTarget,
		agent.MemoryCategoryVulnerability,
		agent.MemoryCategoryFact,
		agent.MemoryCategoryNote,
	}

	stats := make(map[string]int)
	total := 0
	for _, cat := range categories {
		entries, err := h.memory.List(cat, 10000)
		if err != nil {
			continue
		}
		count := len(entries)
		stats[string(cat)] = count
		total += count
	}

	c.JSON(http.StatusOK, gin.H{
		"total":      total,
		"categories": stats,
		"enabled":    true,
	})
}

// CreateMemory handles POST /api/memories
// Body: { "key": "...", "value": "...", "category": "...", "conversation_id": "..." }
func (h *MemoryHandler) CreateMemory(c *gin.Context) {
	var req struct {
		Key            string `json:"key" binding:"required"`
		Value          string `json:"value" binding:"required"`
		Category       string `json:"category"`
		ConversationID string `json:"conversation_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cat := agent.MemoryCategory(strings.TrimSpace(req.Category))
	if cat == "" {
		cat = agent.MemoryCategoryFact
	}

	entry, err := h.memory.Store(req.Key, req.Value, cat, req.ConversationID)
	if err != nil {
		h.logger.Error("failed to create memory", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"entry": entry})
}

// UpdateMemory handles PUT /api/memories/:id
// Body: { "key": "...", "value": "...", "category": "..." }
func (h *MemoryHandler) UpdateMemory(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}

	var req struct {
		Key      string `json:"key" binding:"required"`
		Value    string `json:"value" binding:"required"`
		Category string `json:"category"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cat := agent.MemoryCategory(strings.TrimSpace(req.Category))
	if cat == "" {
		cat = agent.MemoryCategoryFact
	}

	entry, err := h.memory.UpdateByID(id, req.Key, req.Value, cat)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		h.logger.Error("failed to update memory", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"entry": entry})
}

// DeleteMemory handles DELETE /api/memories/:id
func (h *MemoryHandler) DeleteMemory(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}

	if err := h.memory.Delete(id); err != nil {
		h.logger.Error("failed to delete memory", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// DeleteAllMemories handles DELETE /api/memories (delete all or all in a category)
// Query param: category (optional)
func (h *MemoryHandler) DeleteAllMemories(c *gin.Context) {
	categoryStr := c.Query("category")
	cat := agent.MemoryCategory(strings.TrimSpace(categoryStr))

	entries, err := h.memory.List(cat, 10000)
	if err != nil {
		h.logger.Error("failed to list memories for bulk delete", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	deleted := 0
	for _, e := range entries {
		if delErr := h.memory.Delete(e.ID); delErr != nil {
			h.logger.Warn("failed to delete memory entry", zap.String("id", e.ID), zap.Error(delErr))
		} else {
			deleted++
		}
	}

	c.JSON(http.StatusOK, gin.H{"deleted": deleted})
}
