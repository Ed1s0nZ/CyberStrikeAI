package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"cyberstrike-ai/internal/agent"
	"cyberstrike-ai/internal/audit"
	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/database"
	workflowrunner "cyberstrike-ai/internal/workflow"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type WorkflowHandler struct {
	db     *database.DB
	logger *zap.Logger
	audit  *audit.Service
	agent  *agent.Agent
	cfg    *config.Config
}

func NewWorkflowHandler(db *database.DB, logger *zap.Logger) *WorkflowHandler {
	return &WorkflowHandler{db: db, logger: logger}
}

func (h *WorkflowHandler) SetAudit(s *audit.Service) {
	h.audit = s
}

type workflowSaveRequest struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Version     int             `json:"version,omitempty"`
	Enabled     *bool           `json:"enabled,omitempty"`
	Graph       json.RawMessage `json:"graph,omitempty"`
	GraphJSON   json.RawMessage `json:"graph_json,omitempty"`
}

type workflowDryRunRequest struct {
	Graph     json.RawMessage        `json:"graph,omitempty"`
	GraphJSON json.RawMessage        `json:"graph_json,omitempty"`
	Inputs    map[string]interface{} `json:"inputs,omitempty"`
}

type workflowGenerateDraftRequest struct {
	Prompt         string                      `json:"prompt"`
	Options        workflowrunner.DraftOptions `json:"options"`
	AvailableTools []workflowrunner.DraftTool  `json:"available_tools,omitempty"`
}

func (h *WorkflowHandler) List(c *gin.Context) {
	includeDisabled := strings.EqualFold(c.Query("includeDisabled"), "true") || c.Query("include_disabled") == "1"
	items, err := h.db.ListWorkflowDefinitions(includeDisabled)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"workflows": items})
}

func (h *WorkflowHandler) Get(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	wf, err := h.db.GetWorkflowDefinition(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if wf == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "工作流不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"workflow": wf})
}

func (h *WorkflowHandler) Create(c *gin.Context) {
	h.save(c, "")
}

func (h *WorkflowHandler) Validate(c *gin.Context) {
	var req workflowSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "无效的请求参数: " + err.Error()})
		return
	}
	graph := req.Graph
	if len(graph) == 0 {
		graph = req.GraphJSON
	}
	if len(graph) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "graph 不能为空"})
		return
	}
	if !json.Valid(graph) {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "graph 必须是合法 JSON"})
		return
	}
	if err := workflowrunner.ValidateGraphJSON(c.Request.Context(), string(graph)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *WorkflowHandler) DryRun(c *gin.Context) {
	var req workflowDryRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数: " + err.Error()})
		return
	}
	graph := req.Graph
	if len(graph) == 0 {
		graph = req.GraphJSON
	}
	if len(graph) == 0 || !json.Valid(graph) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "graph 必须是合法 JSON"})
		return
	}
	inputs := make(map[string]any, len(req.Inputs))
	for k, v := range req.Inputs {
		inputs[k] = v
	}
	result, err := workflowrunner.DryRunGraphJSON(c.Request.Context(), string(graph), inputs)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": result})
}

func (h *WorkflowHandler) GenerateDraft(c *gin.Context) {
	var req workflowGenerateDraftRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数: " + err.Error()})
		return
	}
	draftReq := workflowrunner.DraftRequest{
		Prompt:         req.Prompt,
		Options:        req.Options,
		AvailableTools: req.AvailableTools,
	}
	var result *workflowrunner.DraftResult
	var llmErr error
	if h.cfg != nil {
		if llmCfg, _, ok := h.cfg.ResolveAIChannel(""); ok && strings.TrimSpace(llmCfg.APIKey) != "" && strings.TrimSpace(llmCfg.Model) != "" {
			result, llmErr = workflowrunner.GenerateDraftFromLLM(c.Request.Context(), draftReq, llmCfg, h.logger)
		} else {
			llmErr = errors.New("AI 通道未配置 api_key 或 model")
		}
	} else {
		llmErr = errors.New("工作流生成器未加载平台 AI 配置")
	}
	if llmErr != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "大模型生成失败: " + llmErr.Error()})
		return
	}
	if h.audit != nil {
		h.audit.RecordOK(c, "workflow", "generate_draft", "自然语言生成工作流草稿", "", "", map[string]interface{}{
			"generator": result.Generator,
			"nodes":     result.Stats["nodes"],
			"edges":     result.Stats["edges"],
			"high_risk": result.Audit.HighRisk,
			"savable":   result.Audit.Savable,
		})
	}
	c.JSON(http.StatusOK, gin.H{"result": result})
}

func (h *WorkflowHandler) Update(c *gin.Context) {
	h.save(c, c.Param("id"))
}

func (h *WorkflowHandler) save(c *gin.Context, pathID string) {
	var req workflowSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数: " + err.Error()})
		return
	}
	id := strings.TrimSpace(req.ID)
	if strings.TrimSpace(pathID) != "" {
		id = strings.TrimSpace(pathID)
	}
	name := strings.TrimSpace(req.Name)
	if id == "" || name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "工作流 id 和 name 不能为空"})
		return
	}
	graph := req.Graph
	if len(graph) == 0 {
		graph = req.GraphJSON
	}
	if len(graph) == 0 {
		graph = []byte(`{"nodes":[],"edges":[],"config":{}}`)
	}
	if !json.Valid(graph) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "graph 必须是合法 JSON"})
		return
	}
	if err := workflowrunner.ValidateGraphJSON(c.Request.Context(), string(graph)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "工作流图无法编译: " + err.Error()})
		return
	}
	var probe interface{}
	if err := json.Unmarshal(graph, &probe); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "graph JSON 解析失败: " + err.Error()})
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	wf := &database.WorkflowDefinition{
		ID:          id,
		Name:        name,
		Description: strings.TrimSpace(req.Description),
		Version:     req.Version,
		GraphJSON:   string(graph),
		Enabled:     enabled,
	}
	if err := h.db.UpsertWorkflowDefinition(wf); err != nil {
		if h.logger != nil {
			h.logger.Warn("保存工作流失败", zap.String("id", id), zap.Error(err))
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	saved, _ := h.db.GetWorkflowDefinition(id)
	workflowrunner.InvalidateCompiledCache(id)
	if h.audit != nil {
		h.audit.RecordOK(c, "workflow", "save", "保存工作流", "workflow", id, map[string]interface{}{"name": name})
	}
	c.JSON(http.StatusOK, gin.H{"message": "工作流已保存", "workflow": saved})
}

func (h *WorkflowHandler) Delete(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "工作流 id 不能为空"})
		return
	}
	if err := h.db.DeleteWorkflowDefinition(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	workflowrunner.InvalidateCompiledCache(id)
	if h.audit != nil {
		h.audit.RecordOK(c, "workflow", "delete", "删除工作流", "workflow", id, nil)
	}
	c.JSON(http.StatusOK, gin.H{"message": "工作流已删除"})
}
