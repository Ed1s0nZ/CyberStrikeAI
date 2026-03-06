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

var validMemoryCategories = map[agent.MemoryCategory]struct{}{
	agent.MemoryCategoryCredential:    {},
	agent.MemoryCategoryTarget:        {},
	agent.MemoryCategoryVulnerability: {},
	agent.MemoryCategoryFact:          {},
	agent.MemoryCategoryNote:          {},
	agent.MemoryCategoryToolRun:       {},
	agent.MemoryCategoryDiscovery:     {},
	agent.MemoryCategoryPlan:          {},
}

var validMemoryConfidences = map[agent.MemoryConfidence]struct{}{
	agent.MemoryConfidenceHigh:   {},
	agent.MemoryConfidenceMedium: {},
	agent.MemoryConfidenceLow:    {},
}

// NewMemoryHandler creates a MemoryHandler backed by the given PersistentMemory.
func NewMemoryHandler(memory *agent.PersistentMemory, logger *zap.Logger) *MemoryHandler {
	return &MemoryHandler{memory: memory, logger: logger}
}

// ListMemories handles GET /api/memories
// Query params: category (optional), limit (default 100), search (optional text search),
// entity (optional), include_dismissed (optional bool)
func (h *MemoryHandler) ListMemories(c *gin.Context) {
	categoryStr := c.Query("category")
	search := c.Query("search")
	entity := c.Query("entity")
	includeDismissedStr := c.Query("include_dismissed")
	limitStr := c.DefaultQuery("limit", "100")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	cat := agent.MemoryCategory(strings.TrimSpace(categoryStr))
	includeDismissed := includeDismissedStr == "true" || includeDismissedStr == "1"

	var entries []*agent.MemoryEntry
	if entity != "" {
		// Respect category/include_dismissed filters even when entity is provided.
		entity = strings.TrimSpace(entity)
		if includeDismissed {
			entries, err = h.memory.ListAll(cat, 5000)
		} else {
			entries, err = h.memory.List(cat, 5000)
		}
		if err == nil {
			filtered := make([]*agent.MemoryEntry, 0, len(entries))
			for _, entry := range entries {
				if strings.EqualFold(strings.TrimSpace(entry.Entity), entity) {
					filtered = append(filtered, entry)
					if len(filtered) >= limit {
						break
					}
				}
			}
			entries = filtered
		}
	} else if search != "" {
		if includeDismissed {
			entries, err = h.memory.RetrieveAll(search, cat, limit)
		} else {
			entries, err = h.memory.Retrieve(search, cat, limit)
		}
	} else {
		if includeDismissed {
			entries, err = h.memory.ListAll(cat, limit)
		} else {
			entries, err = h.memory.List(cat, limit)
		}
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
// Returns counts per category, per status, and total.
func (h *MemoryHandler) GetMemoryStats(c *gin.Context) {
	categories := []agent.MemoryCategory{
		agent.MemoryCategoryCredential,
		agent.MemoryCategoryTarget,
		agent.MemoryCategoryVulnerability,
		agent.MemoryCategoryFact,
		agent.MemoryCategoryNote,
		agent.MemoryCategoryToolRun,
		agent.MemoryCategoryDiscovery,
		agent.MemoryCategoryPlan,
	}

	stats := make(map[string]int)
	total := 0
	for _, cat := range categories {
		// Use ListAll to count all entries including dismissed ones.
		entries, err := h.memory.ListAll(cat, 10000)
		if err != nil {
			continue
		}
		count := len(entries)
		stats[string(cat)] = count
	}
	allEntries, err := h.memory.ListAll("", 10000)
	if err == nil {
		total = len(allEntries)
	}

	// Count by status.
	statuses := []agent.MemoryStatus{
		agent.MemoryStatusActive,
		agent.MemoryStatusConfirmed,
		agent.MemoryStatusFalsePositive,
		agent.MemoryStatusDisproven,
	}
	statusStats := make(map[string]int)
	for _, status := range statuses {
		entries, err := h.memory.FindByStatus(status, "", 10000)
		if err != nil {
			continue
		}
		statusStats[string(status)] = len(entries)
	}

	c.JSON(http.StatusOK, gin.H{
		"total":      total,
		"categories": stats,
		"by_status":  statusStats,
		"enabled":    true,
	})
}

// UpdateMemoryStatus handles PATCH /api/memories/:id/status
// Body: { "status": "confirmed|false_positive|disproven|active" }
func (h *MemoryHandler) UpdateMemoryStatus(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}

	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	status := agent.MemoryStatus(strings.TrimSpace(req.Status))
	switch status {
	case agent.MemoryStatusActive, agent.MemoryStatusConfirmed, agent.MemoryStatusFalsePositive, agent.MemoryStatusDisproven:
		// valid
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status: must be active, confirmed, false_positive, or disproven"})
		return
	}

	if err := h.memory.SetStatus(id, status); err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		h.logger.Error("failed to update memory status", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "id": id, "status": string(status)})
}

// CreateMemory handles POST /api/memories
// Body: { "key": "...", "value": "...", "category": "...", "conversation_id": "...", "entity": "...", "confidence": "..." }
func (h *MemoryHandler) CreateMemory(c *gin.Context) {
	var req struct {
		Key            string `json:"key" binding:"required"`
		Value          string `json:"value" binding:"required"`
		Category       string `json:"category"`
		ConversationID string `json:"conversation_id"`
		Entity         string `json:"entity"`
		Confidence     string `json:"confidence"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cat := agent.MemoryCategory(strings.TrimSpace(req.Category))
	if cat == "" {
		cat = agent.MemoryCategoryFact
	}
	if _, ok := validMemoryCategories[cat]; !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid category"})
		return
	}

	confidence := agent.MemoryConfidence(strings.TrimSpace(req.Confidence))
	if confidence == "" {
		confidence = agent.MemoryConfidenceMedium
	}
	if _, ok := validMemoryConfidences[confidence]; !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid confidence: must be low, medium, or high"})
		return
	}

	entry, err := h.memory.StoreFull(req.Key, req.Value, cat, req.ConversationID, req.Entity, confidence, agent.MemoryStatusActive)
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
	if _, ok := validMemoryCategories[cat]; !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid category"})
		return
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
