package handler

import (
	"net/http"
	"time"

	"cyberstrike-ai/internal/mcp"
	"cyberstrike-ai/internal/security"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// MonitorHandler 监控处理器
type MonitorHandler struct {
	mcpServer *mcp.Server
	executor  *security.Executor
	logger    *zap.Logger
	vulns     []security.Vulnerability
}

// NewMonitorHandler 创建新的监控处理器
func NewMonitorHandler(mcpServer *mcp.Server, executor *security.Executor, logger *zap.Logger) *MonitorHandler {
	return &MonitorHandler{
		mcpServer: mcpServer,
		executor:  executor,
		logger:    logger,
		vulns:     []security.Vulnerability{},
	}
}

// MonitorResponse 监控响应
type MonitorResponse struct {
	Executions    []*mcp.ToolExecution `json:"executions"`
	Stats         map[string]*mcp.ToolStats `json:"stats"`
	Vulnerabilities []security.Vulnerability `json:"vulnerabilities"`
	Report        map[string]interface{} `json:"report"`
	Timestamp     time.Time              `json:"timestamp"`
}

// Monitor 获取监控信息
func (h *MonitorHandler) Monitor(c *gin.Context) {
	// 获取所有执行记录
	executions := h.mcpServer.GetAllExecutions()

	// 分析执行结果，提取漏洞
	for _, exec := range executions {
		if exec.Status == "completed" && exec.Result != nil {
			vulns := h.executor.AnalyzeResults(exec.ToolName, exec.Result)
			h.vulns = append(h.vulns, vulns...)
		}
	}

	// 获取统计信息
	stats := h.mcpServer.GetStats()

	// 生成报告
	report := h.executor.GetVulnerabilityReport(h.vulns)

	c.JSON(http.StatusOK, MonitorResponse{
		Executions:     executions,
		Stats:          stats,
		Vulnerabilities: h.vulns,
		Report:         report,
		Timestamp:      time.Now(),
	})
}

// GetExecution 获取特定执行记录
func (h *MonitorHandler) GetExecution(c *gin.Context) {
	id := c.Param("id")
	
	exec, exists := h.mcpServer.GetExecution(id)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "执行记录未找到"})
		return
	}

	c.JSON(http.StatusOK, exec)
}

// GetStats 获取统计信息
func (h *MonitorHandler) GetStats(c *gin.Context) {
	stats := h.mcpServer.GetStats()
	c.JSON(http.StatusOK, stats)
}

// GetVulnerabilities 获取漏洞列表
func (h *MonitorHandler) GetVulnerabilities(c *gin.Context) {
	report := h.executor.GetVulnerabilityReport(h.vulns)
	c.JSON(http.StatusOK, report)
}

