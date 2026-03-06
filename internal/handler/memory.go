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
	statusStr := strings.TrimSpace(c.Query("status"))
	confidenceStr := strings.TrimSpace(c.Query("confidence"))
	includeDismissedStr := c.Query("include_dismissed")
	limitStr := c.DefaultQuery("limit", "100")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	cat := agent.MemoryCategory(strings.TrimSpace(categoryStr))
	if categoryStr != "" {
		if _, ok := validMemoryCategories[cat]; !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid category"})
			return
		}
	}

	if statusStr != "" {
		switch agent.MemoryStatus(statusStr) {
		case agent.MemoryStatusActive, agent.MemoryStatusConfirmed, agent.MemoryStatusFalsePositive, agent.MemoryStatusDisproven:
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
			return
		}
	}
	if confidenceStr != "" {
		if _, ok := validMemoryConfidences[agent.MemoryConfidence(confidenceStr)]; !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid confidence"})
			return
		}
	}

	includeDismissed := includeDismissedStr == "true" || includeDismissedStr == "1"

	var entries []*agent.MemoryEntry
	baseLimit := 5000
	if search != "" {
		if includeDismissed {
			entries, err = h.memory.RetrieveAll(search, cat, baseLimit)
		} else {
			entries, err = h.memory.Retrieve(search, cat, baseLimit)
		}
	} else {
		if includeDismissed {
			entries, err = h.memory.ListAll(cat, baseLimit)
		} else {
			entries, err = h.memory.List(cat, baseLimit)
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

	entityLower := strings.ToLower(strings.TrimSpace(entity))
	filtered := make([]*agent.MemoryEntry, 0, len(entries))
	for _, entry := range entries {
		if entityLower != "" && !strings.Contains(strings.ToLower(entry.Entity), entityLower) {
			continue
		}
		if statusStr != "" && string(entry.Status) != statusStr {
			continue
		}
		if confidenceStr != "" && string(entry.Confidence) != confidenceStr {
			continue
		}
		filtered = append(filtered, entry)
	}

	total := len(filtered)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	paged := filtered[offset:end]
	hasMore := end < total

	c.JSON(http.StatusOK, gin.H{
		"entries":   paged,
		"total":     total,
		"offset":    offset,
		"limit":     limit,
		"has_more":  hasMore,
		"returned":  len(paged),
		"filtered":  total,
		"requested": limit,
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

	// Count by status using a single grouped query.
	statusCounts, err := h.memory.CountByStatus("")
	statusStats := make(map[string]int)
	for _, status := range []agent.MemoryStatus{
		agent.MemoryStatusActive,
		agent.MemoryStatusConfirmed,
		agent.MemoryStatusFalsePositive,
		agent.MemoryStatusDisproven,
	} {
		statusStats[string(status)] = statusCounts[status]
	}
	if err != nil {
		h.logger.Warn("failed to count memory status stats", zap.Error(err))
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

	cat := agent.MemoryCategoryFact
	if strings.TrimSpace(req.Category) != "" {
		cat = agent.MemoryCategory(strings.TrimSpace(req.Category))
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

	cat := agent.MemoryCategoryFact
	if strings.TrimSpace(req.Category) != "" {
		cat = agent.MemoryCategory(strings.TrimSpace(req.Category))
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

	// Use ListAll so bulk delete truly deletes all entries, including dismissed ones.
	entries, err := h.memory.ListAll(cat, 10000)
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
