package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"cyberstrike-ai/internal/agent"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ParallelScanHandler handles parallel scan API endpoints
type ParallelScanHandler struct {
	manager *agent.ParallelScanManager
	logger  *zap.Logger
}

// NewParallelScanHandler creates a new handler
func NewParallelScanHandler(manager *agent.ParallelScanManager, logger *zap.Logger) *ParallelScanHandler {
	return &ParallelScanHandler{manager: manager, logger: logger}
}

// CreateParallelScanRequest is the request body for creating a scan
type CreateParallelScanRequest struct {
	Target       string   `json:"target" binding:"required"`
	Agents       []string `json:"agents,omitempty"`       // subset of attack vector names; empty = all 5
	MaxRounds    int      `json:"maxRounds,omitempty"`     // default 20
	ReconContext string   `json:"reconContext,omitempty"`
}

// CreateScan handles POST /api/parallel-scan
func (h *ParallelScanHandler) CreateScan(c *gin.Context) {
	var req CreateParallelScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	scan, err := h.manager.StartScan(req.Target, req.Agents, req.MaxRounds, req.ReconContext)
	if err != nil {
		h.logger.Error("Failed to create parallel scan", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, h.scanStateToJSON(scan))
}

// ListScans handles GET /api/parallel-scan
func (h *ParallelScanHandler) ListScans(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"scans": []interface{}{}})
}

// GetScan handles GET /api/parallel-scan/:id
func (h *ParallelScanHandler) GetScan(c *gin.Context) {
	scanID := c.Param("id")
	scan, exists := h.manager.GetScanState(scanID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "scan not found"})
		return
	}
	c.JSON(http.StatusOK, h.scanStateToJSON(scan))
}

// StreamScan handles GET /api/parallel-scan/:id/stream (SSE)
func (h *ParallelScanHandler) StreamScan(c *gin.Context) {
	scanID := c.Param("id")

	ch, unsubscribe, err := h.manager.Subscribe(scanID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	defer unsubscribe()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	flusher, _ := c.Writer.(http.Flusher)

	for {
		select {
		case event, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(event)
			fmt.Fprintf(c.Writer, "data: %s\n\n", data)
			if flusher != nil {
				flusher.Flush()
			}
		case <-c.Request.Context().Done():
			return
		}
	}
}

// StopScan handles POST /api/parallel-scan/:id/stop
func (h *ParallelScanHandler) StopScan(c *gin.Context) {
	scanID := c.Param("id")
	if err := h.manager.StopScan(scanID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "cancelled"})
}

// StopAgent handles POST /api/parallel-scan/:id/agents/:agentId/stop
func (h *ParallelScanHandler) StopAgent(c *gin.Context) {
	scanID := c.Param("id")
	agentID := c.Param("agentId")
	if err := h.manager.StopAgent(scanID, agentID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "stopped"})
}

// RestartAgent handles POST /api/parallel-scan/:id/agents/:agentId/restart
func (h *ParallelScanHandler) RestartAgent(c *gin.Context) {
	scanID := c.Param("id")
	agentID := c.Param("agentId")
	if err := h.manager.RestartAgent(scanID, agentID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "restarted"})
}

// DeleteScan handles DELETE /api/parallel-scan/:id
func (h *ParallelScanHandler) DeleteScan(c *gin.Context) {
	scanID := c.Param("id")
	h.manager.StopScan(scanID)
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// GetAttackVectors handles GET /api/parallel-scan/vectors
func (h *ParallelScanHandler) GetAttackVectors(c *gin.Context) {
	vectors := agent.DefaultAttackVectors()
	result := make([]map[string]string, len(vectors))
	for i, v := range vectors {
		result[i] = map[string]string{
			"name":        v.Name,
			"description": v.Description,
		}
	}
	c.JSON(http.StatusOK, gin.H{"vectors": result})
}

func (h *ParallelScanHandler) scanStateToJSON(scan *agent.ParallelScanState) gin.H {
	agents := make([]gin.H, 0)
	scan.Mu.RLock()
	for _, as := range scan.Agents {
		agents = append(agents, gin.H{
			"id":              as.ID,
			"name":            as.Name,
			"conversationId":  as.ConversationID,
			"status":          as.Status,
			"currentRound":    as.CurrentRound,
			"totalIterations": as.TotalIters,
			"totalToolCalls":  as.TotalTools,
			"totalVulns":      as.TotalVulns,
			"errors":          as.Errors,
		})
	}
	scan.Mu.RUnlock()

	return gin.H{
		"id":        scan.ID,
		"target":    scan.Target,
		"status":    scan.Status,
		"maxRounds": scan.MaxRounds,
		"agents":    agents,
	}
}
