package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"cyberstrike-ai/internal/attackchain"
	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/multiagent"
	"cyberstrike-ai/internal/project"
	"cyberstrike-ai/internal/security"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const maxProjectDescriptionRunes = 4000

func clampProjectDescription(s string) string {
	r := []rune(s)
	if len(r) <= maxProjectDescriptionRunes {
		return s
	}
	return string(r[:maxProjectDescriptionRunes])
}

// ProjectHandler 项目管理处理器。
type ProjectHandler struct {
	db                    *database.DB
	logger                *zap.Logger
	stateDir              string
	cairnCanonicalEnabled bool
}

// NewProjectHandler 创建项目管理处理器。
func NewProjectHandler(db *database.DB, logger *zap.Logger) *ProjectHandler {
	return &ProjectHandler{db: db, logger: logger, stateDir: "state"}
}

func (h *ProjectHandler) SetCairnStateDir(dir string) {
	if strings.TrimSpace(dir) != "" {
		h.stateDir = strings.TrimSpace(dir)
	}
}

// SetCairnCanonicalEnabled switches read APIs to SQLite canonical snapshots.
// Runtime writes stay on their separately gated path.
func (h *ProjectHandler) SetCairnCanonicalEnabled(enabled bool) {
	h.cairnCanonicalEnabled = enabled
}

type createProjectRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	ScopeJSON   string `json:"scope_json"`
	Status      string `json:"status"`
}

// updateProjectRequest 部分更新：字段省略表示不修改；传 null 或 "" 可清空字符串字段。
type updateProjectRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	ScopeJSON   *string `json:"scope_json"`
	Status      *string `json:"status"`
	Pinned      *bool   `json:"pinned"`
}

// CreateProject POST /api/projects
func (h *ProjectHandler) CreateProject(c *gin.Context) {
	var req createProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	p := &database.Project{
		Name:        strings.TrimSpace(req.Name),
		Description: clampProjectDescription(req.Description),
		ScopeJSON:   req.ScopeJSON,
		Status:      strings.TrimSpace(req.Status),
	}
	created, err := h.db.CreateProject(p)
	if err != nil {
		h.logger.Error("创建项目失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if session, ok := security.CurrentSession(c); ok {
		_ = h.db.SetResourceOwner("project", created.ID, session.UserID)
		_ = h.db.AssignResourceToUser(session.UserID, "project", created.ID)
	}
	c.JSON(http.StatusOK, created)
}

// GetDashboardSummary GET /api/projects/dashboard-summary
func (h *ProjectHandler) GetDashboardSummary(c *gin.Context) {
	limit, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("fact_limit", "5")))
	if limit <= 0 {
		limit = 5
	}
	if limit > 50 {
		limit = 50
	}
	session, _ := security.CurrentSession(c)
	summary, err := h.db.GetProjectDashboardSummaryForAccess(limit, session.UserID, session.Scope)
	if err != nil {
		h.logger.Error("获取项目仪表盘摘要失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if summary.RecentFacts == nil {
		summary.RecentFacts = []database.ProjectDashboardFact{}
	}
	c.JSON(http.StatusOK, summary)
}

// ListProjects GET /api/projects
func (h *ProjectHandler) ListProjects(c *gin.Context) {
	status := c.Query("status")
	search := c.Query("search")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.Query("offset"))
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	session, _ := security.CurrentSession(c)
	list, err := h.db.ListProjectsForAccess(status, search, limit, offset, session.UserID, session.Scope)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if list == nil {
		list = []*database.Project{}
	}
	total, err := h.db.CountProjectsForAccess(status, search, session.UserID, session.Scope)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"projects": list,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	})
}

// GetProjectStats GET /api/projects/:id/stats
func (h *ProjectHandler) GetProjectStats(c *gin.Context) {
	stats, err := project.GetProjectStats(h.db, c.Param("id"))
	if err != nil {
		if strings.Contains(err.Error(), "不存在") {
			c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// ListProjectConversations GET /api/projects/:id/conversations
func (h *ProjectHandler) ListProjectConversations(c *gin.Context) {
	projectID := c.Param("id")
	if _, err := h.db.GetProject(projectID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	offset, _ := strconv.Atoi(c.Query("offset"))
	list, err := h.db.ListConversationsByProjectID(projectID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if list == nil {
		list = []*database.Conversation{}
	}
	total, _ := h.db.CountConversationsByProjectID(projectID)
	c.JSON(http.StatusOK, gin.H{
		"conversations": list,
		"total":         total,
		"limit":         limit,
		"offset":        offset,
	})
}

// GetProject GET /api/projects/:id
func (h *ProjectHandler) GetProject(c *gin.Context) {
	p, err := h.db.GetProject(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}
	c.JSON(http.StatusOK, p)
}

// UpdateProject PUT /api/projects/:id
func (h *ProjectHandler) UpdateProject(c *gin.Context) {
	id := c.Param("id")
	p, err := h.db.GetProject(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}
	var req updateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Name != nil {
		if s := strings.TrimSpace(*req.Name); s != "" {
			p.Name = s
		}
	}
	if req.Description != nil {
		p.Description = clampProjectDescription(*req.Description)
	}
	if req.ScopeJSON != nil {
		p.ScopeJSON = *req.ScopeJSON
	}
	if req.Status != nil {
		if s := strings.TrimSpace(*req.Status); s != "" {
			p.Status = s
		}
	}
	if req.Pinned != nil {
		p.Pinned = *req.Pinned
	}
	if err := h.db.UpdateProject(p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, p)
}

// DeleteProject DELETE /api/projects/:id
func (h *ProjectHandler) DeleteProject(c *gin.Context) {
	if err := h.db.DeleteProject(c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

type factLinkRequest struct {
	From       string `json:"from"`
	Type       string `json:"type"`
	Confidence string `json:"confidence,omitempty"`
}

type upsertFactRequest struct {
	FactKey                string            `json:"fact_key" binding:"required"`
	Category               string            `json:"category"`
	Summary                string            `json:"summary" binding:"required"`
	Body                   string            `json:"body"`
	Confidence             string            `json:"confidence"`
	Pinned                 bool              `json:"pinned"`
	RelatedVulnerabilityID string            `json:"related_vulnerability_id"`
	Links                  []factLinkRequest `json:"links"`
	LinksText              *string           `json:"links_text"`
}

// updateFactRequest 部分更新事实；指针字段省略=不修改，body 传 "" 可清空（仍走 merge 逻辑见 Upsert）。
type updateFactRequest struct {
	FactKey                *string            `json:"fact_key"`
	Category               *string            `json:"category"`
	Summary                *string            `json:"summary"`
	Body                   *string            `json:"body"`
	Confidence             *string            `json:"confidence"`
	Pinned                 *bool              `json:"pinned"`
	RelatedVulnerabilityID *string            `json:"related_vulnerability_id"`
	ClearBody              bool               `json:"clear_body"`
	Links                  *[]factLinkRequest `json:"links"`
	LinksText              *string            `json:"links_text"`
}

func factLinksFromRequest(links []factLinkRequest, linksText *string) (*project.ParsedFactLinks, error) {
	if len(links) > 0 {
		parsed := &project.ParsedFactLinks{}
		for i, l := range links {
			from := strings.TrimSpace(l.From)
			edgeType := strings.TrimSpace(l.Type)
			if from == "" {
				return nil, fmt.Errorf("links[%d] 须含 from", i)
			}
			if edgeType == "" {
				return nil, fmt.Errorf("links[%d] 须含 type", i)
			}
			parsed.Incoming = append(parsed.Incoming, database.ProjectFactEdgeFromInput{
				From: from, Type: edgeType, Confidence: strings.TrimSpace(l.Confidence),
			})
		}
		return parsed, nil
	}
	if linksText != nil {
		in, err := project.ParseFactLinksText(*linksText)
		if err != nil {
			return nil, err
		}
		return &project.ParsedFactLinks{Incoming: in}, nil
	}
	return &project.ParsedFactLinks{Incoming: []database.ProjectFactEdgeFromInput{}}, nil
}

type factWithLinksResponse struct {
	*database.ProjectFact
	OutgoingLinks []*database.ProjectFactEdge `json:"outgoing_links,omitempty"`
	IncomingLinks []*database.ProjectFactEdge `json:"incoming_links,omitempty"`
	LinkCounts    *project.LinkCounts         `json:"link_counts,omitempty"`
}

func (h *ProjectHandler) applyFactLinksAfterUpsert(projectID string, fact *database.ProjectFact, links []factLinkRequest, linksText *string, explicitLinks, parseBody bool) error {
	if explicitLinks {
		parsed, err := factLinksFromRequest(links, linksText)
		if err != nil {
			return err
		}
		return project.PersistFactLinksFromParsed(h.db, projectID, fact.FactKey, fact.SourceConversationID, parsed, true)
	}
	if parseBody {
		inputs := project.ParseLinksFromBody(fact.Body)
		if inputs == nil {
			return nil
		}
		return project.PersistFactIncomingLinks(h.db, projectID, fact.FactKey, inputs, true)
	}
	return nil
}

func (h *ProjectHandler) factResponseWithLinks(projectID string, f *database.ProjectFact, includeLinks bool) interface{} {
	if !includeLinks || f == nil {
		return f
	}
	out, _ := h.db.ListOutgoingProjectFactEdges(projectID, f.FactKey)
	in, _ := h.db.ListIncomingProjectFactEdges(projectID, f.FactKey)
	return &factWithLinksResponse{
		ProjectFact:   f,
		OutgoingLinks: out,
		IncomingLinks: in,
	}
}

// ListFacts GET /api/projects/:id/facts （fact_key 查询参数可获取单条详情）
func (h *ProjectHandler) ListFacts(c *gin.Context) {
	projectID := c.Param("id")
	if key := strings.TrimSpace(c.Query("fact_key")); key != "" {
		f, err := h.db.GetProjectFactByKey(projectID, key)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		includeLinks := c.Query("include_links") == "1" || c.Query("include_links") == "true"
		c.JSON(http.StatusOK, h.factResponseWithLinks(projectID, f, includeLinks))
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	offset, _ := strconv.Atoi(c.Query("offset"))
	filter := database.ProjectFactListFilter{
		Category:               c.Query("category"),
		Confidence:             c.Query("confidence"),
		Search:                 c.Query("search"),
		RelatedVulnerabilityID: c.Query("related_vulnerability_id"),
	}
	if c.Query("exclude_deprecated") == "1" || c.Query("exclude_deprecated") == "true" {
		filter.ExcludeDeprecated = true
	}
	list, err := h.db.ListProjectFacts(projectID, filter, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if list == nil {
		list = []*database.ProjectFact{}
	}
	if sparseOnly := c.Query("sparse_only"); sparseOnly == "1" || sparseOnly == "true" {
		filtered := make([]*database.ProjectFact, 0, len(list))
		for _, f := range list {
			if project.IsSparseFactBody(f.Category, f.FactKey, f.Body) {
				filtered = append(filtered, f)
			}
		}
		list = filtered
	}
	includeLinkCounts := c.Query("include_link_counts") == "1" || c.Query("include_link_counts") == "true"
	if !includeLinkCounts {
		c.JSON(http.StatusOK, list)
		return
	}
	counts, err := project.LoadProjectFactLinkCounts(h.db, projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]factWithLinksResponse, 0, len(list))
	for _, f := range list {
		item := factWithLinksResponse{ProjectFact: f}
		if c, ok := counts[f.FactKey]; ok {
			cc := c
			item.LinkCounts = &cc
		}
		out = append(out, item)
	}
	c.JSON(http.StatusOK, out)
}

// GetFactGraph GET /api/projects/:id/fact-graph?view=path|full
func (h *ProjectHandler) GetFactGraph(c *gin.Context) {
	projectID := c.Param("id")
	if _, err := h.db.GetProject(projectID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}
	view := c.DefaultQuery("view", "path")
	excludeDeprecated := true
	if v := c.Query("exclude_deprecated"); v == "0" || v == "false" {
		excludeDeprecated = false
	}
	graph, err := project.BuildProjectFactGraph(h.db, projectID, view, excludeDeprecated)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if graph.Nodes == nil {
		graph.Nodes = []database.ProjectFactGraphNode{}
	}
	if graph.Edges == nil {
		graph.Edges = []database.ProjectFactGraphEdge{}
	}
	c.JSON(http.StatusOK, graph)
}

// GetCairnState GET /api/projects/:id/cairn/state?conversation_id=...
func (h *ProjectHandler) GetCairnState(c *gin.Context) {
	projectID := strings.TrimSpace(c.Param("id"))
	if _, err := h.db.GetProject(projectID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}
	conversationID := strings.TrimSpace(c.Query("conversation_id"))
	if conversationID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "conversation_id 不能为空"})
		return
	}
	boundProjectID, err := h.db.GetConversationProjectID(conversationID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "对话不存在"})
		return
	}
	if boundProjectID != projectID {
		c.JSON(http.StatusForbidden, gin.H{"error": "对话未绑定当前项目"})
		return
	}
	if h.cairnCanonicalEnabled {
		snapshot, err := h.db.GetLatestCairnStateSnapshot(c.Request.Context(), projectID, conversationID)
		if err != nil {
			h.logger.Error("读取 Cairn canonical state 失败", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"project_id":       projectID,
			"conversation_id":  conversationID,
			"storage":          "sqlite_canonical",
			"schema_version":   snapshot.SchemaVersion,
			"initialized":      snapshot.Initialized,
			"run":              snapshot.Run,
			"facts":            snapshot.Facts,
			"intents":          snapshot.Intents,
			"hints":            snapshot.Hints,
			"state_updated_at": cairnSnapshotUpdatedAt(snapshot),
		})
		return
	}
	statePath := multiagent.StateFilePath(h.stateDir, projectID, conversationID)
	state, err := multiagent.LoadCairnState(statePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"project_id": projectID, "conversation_id": conversationID, "storage": "yaml_legacy", "state": state})
}

func cairnSnapshotUpdatedAt(snapshot *database.CairnStateSnapshot) interface{} {
	if snapshot == nil || snapshot.Run == nil {
		return nil
	}
	return snapshot.Run.UpdatedAt
}

// GetCairnGraph GET /api/projects/:id/cairn/graph?conversation_id=...
func (h *ProjectHandler) GetCairnGraph(c *gin.Context) {
	projectID := strings.TrimSpace(c.Param("id"))
	if _, err := h.db.GetProject(projectID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}
	conversationID := strings.TrimSpace(c.Query("conversation_id"))
	if conversationID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "conversation_id 不能为空"})
		return
	}
	boundProjectID, err := h.db.GetConversationProjectID(conversationID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "对话不存在"})
		return
	}
	if boundProjectID != projectID {
		c.JSON(http.StatusForbidden, gin.H{"error": "对话未绑定当前项目"})
		return
	}
	if h.cairnCanonicalEnabled {
		snapshot, err := h.db.GetLatestCairnStateSnapshot(c.Request.Context(), projectID, conversationID)
		if err != nil {
			h.logger.Error("读取 Cairn canonical graph 失败", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		nodes := make([]gin.H, 0, len(snapshot.Facts)+len(snapshot.Intents)+len(snapshot.Hints))
		for _, fact := range snapshot.Facts {
			nodes = append(nodes, gin.H{"id": fact.ID, "type": "fact", "label": fact.Statement, "status": fact.Confidence, "source_intent_id": fact.SourceIntentID, "observation_type": fact.ObservationType})
		}
		for _, intent := range snapshot.Intents {
			nodes = append(nodes, gin.H{"id": intent.ID, "type": "intent", "label": intent.Description, "status": intent.Status, "ordinal": intent.Ordinal, "attempt_count": intent.AttemptCount})
		}
		for _, hint := range snapshot.Hints {
			nodes = append(nodes, gin.H{"id": hint.ID, "type": "hint", "label": hint.Content, "status": hint.Status, "creator_type": hint.CreatorType, "creator_id": hint.CreatorID})
		}
		edges := make([]gin.H, 0, len(snapshot.Facts)+len(snapshot.Edges))
		for _, fact := range snapshot.Facts {
			edges = append(edges, gin.H{"source": fact.SourceIntentID, "target": fact.ID, "type": "produced_fact", "source_event_id": fact.SourceEventID})
		}
		for _, edge := range snapshot.Edges {
			edges = append(edges, gin.H{"id": edge.ID, "source": edge.SourceNodeID, "target": edge.TargetNodeID, "type": edge.EdgeType, "source_event_id": edge.SourceEventID})
		}
		c.JSON(http.StatusOK, gin.H{
			"project_id":       projectID,
			"conversation_id":  conversationID,
			"storage":          "sqlite_canonical",
			"schema_version":   snapshot.SchemaVersion,
			"initialized":      snapshot.Initialized,
			"run":              snapshot.Run,
			"nodes":            nodes,
			"edges":            edges,
			"state_updated_at": cairnSnapshotUpdatedAt(snapshot),
		})
		return
	}
	statePath := multiagent.StateFilePath(h.stateDir, projectID, conversationID)
	state, err := multiagent.LoadCairnState(statePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	nodes := make([]gin.H, 0, len(state.Facts)+len(state.Intents)+len(state.Hints))
	edges := make([]gin.H, 0)
	for _, f := range state.Facts {
		nodes = append(nodes, gin.H{"id": f.ID, "type": "fact", "label": f.Description, "status": "valid", "source_intent": f.SourceIntent})
		if f.SourceIntent != "" {
			edges = append(edges, gin.H{"source": f.SourceIntent, "target": f.ID, "type": "produced_fact"})
		}
	}
	for _, it := range state.Intents {
		nodes = append(nodes, gin.H{"id": it.ID, "type": "intent", "label": it.Description, "status": it.Status, "from": it.From})
		for _, from := range it.From {
			edges = append(edges, gin.H{"source": from, "target": it.ID, "type": "opens_intent"})
		}
	}
	for _, hint := range state.Hints {
		nodes = append(nodes, gin.H{"id": hint.ID, "type": "hint", "label": hint.Content, "creator": hint.Creator})
	}
	c.JSON(http.StatusOK, gin.H{"project_id": projectID, "conversation_id": conversationID, "storage": "yaml_legacy", "nodes": nodes, "edges": edges, "state_updated_at": state.UpdatedAt})
}

// CreateFact POST /api/projects/:id/facts
func (h *ProjectHandler) CreateFact(c *gin.Context) {
	var req upsertFactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	projectID := c.Param("id")
	f := &database.ProjectFact{
		ProjectID:              projectID,
		FactKey:                req.FactKey,
		Category:               req.Category,
		Summary:                req.Summary,
		Body:                   req.Body,
		Confidence:             req.Confidence,
		Pinned:                 req.Pinned,
		RelatedVulnerabilityID: req.RelatedVulnerabilityID,
	}
	created, err := h.db.UpsertProjectFact(f)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	explicitLinks := req.Links != nil || req.LinksText != nil
	if err := h.applyFactLinksAfterUpsert(projectID, created, req.Links, req.LinksText, explicitLinks, !explicitLinks); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	created, _ = h.db.GetProjectFactByKey(projectID, created.FactKey)
	c.JSON(http.StatusOK, h.factResponseWithLinks(projectID, created, true))
}

// UpdateFact PUT /api/projects/:id/facts/:factId
func (h *ProjectHandler) UpdateFact(c *gin.Context) {
	projectID := c.Param("id")
	existing, err := h.db.GetProjectFact(c.Param("factId"))
	if err != nil || existing.ProjectID != projectID {
		c.JSON(http.StatusNotFound, gin.H{"error": "事实不存在"})
		return
	}
	oldFactKey := existing.FactKey
	var req updateFactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.FactKey != nil {
		if k := strings.TrimSpace(*req.FactKey); k != "" {
			existing.FactKey = k
		}
	}
	if req.Category != nil && strings.TrimSpace(*req.Category) != "" {
		existing.Category = *req.Category
	}
	if req.Summary != nil && strings.TrimSpace(*req.Summary) != "" {
		existing.Summary = *req.Summary
	}
	if req.ClearBody {
		existing.Body = ""
	} else if req.Body != nil {
		existing.Body = *req.Body
	}
	if req.Confidence != nil && strings.TrimSpace(*req.Confidence) != "" {
		existing.Confidence = *req.Confidence
	}
	if req.Pinned != nil {
		existing.Pinned = *req.Pinned
	}
	if req.RelatedVulnerabilityID != nil {
		existing.RelatedVulnerabilityID = *req.RelatedVulnerabilityID
	}
	updated, err := h.db.UpsertProjectFact(existing)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if oldFactKey != updated.FactKey {
		if err := h.db.RenameProjectFactKeyEdges(projectID, oldFactKey, updated.FactKey); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	if req.Links != nil || req.LinksText != nil {
		var links []factLinkRequest
		if req.Links != nil {
			links = *req.Links
		}
		if err := h.applyFactLinksAfterUpsert(projectID, updated, links, req.LinksText, true, false); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	} else if req.ClearBody || req.Body != nil {
		if err := h.applyFactLinksAfterUpsert(projectID, updated, nil, nil, false, true); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}
	updated, _ = h.db.GetProjectFactByKey(projectID, updated.FactKey)
	c.JSON(http.StatusOK, h.factResponseWithLinks(projectID, updated, true))
}

// DeleteFact DELETE /api/projects/:id/facts/:factId
func (h *ProjectHandler) DeleteFact(c *gin.Context) {
	existing, err := h.db.GetProjectFact(c.Param("factId"))
	if err != nil || existing.ProjectID != c.Param("id") {
		c.JSON(http.StatusNotFound, gin.H{"error": "事实不存在"})
		return
	}
	if err := h.db.DeleteProjectFact(existing.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

type deprecateFactRequest struct {
	FactKey string `json:"fact_key" binding:"required"`
}

// DeprecateFact POST /api/projects/:id/facts/deprecate
func (h *ProjectHandler) DeprecateFact(c *gin.Context) {
	var req deprecateFactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.db.DeprecateProjectFact(c.Param("id"), req.FactKey); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

type restoreFactRequest struct {
	FactKey    string `json:"fact_key" binding:"required"`
	Confidence string `json:"confidence"` // 可选：confirmed | tentative，默认 tentative
}

// RestoreFact POST /api/projects/:id/facts/restore
func (h *ProjectHandler) RestoreFact(c *gin.Context) {
	var req restoreFactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.db.RestoreProjectFact(c.Param("id"), req.FactKey, req.Confidence); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

type createFactEdgeRequest struct {
	SourceFactKey string `json:"source_fact_key" binding:"required"`
	TargetFactKey string `json:"target_fact_key" binding:"required"`
	EdgeType      string `json:"edge_type" binding:"required"`
	Confidence    string `json:"confidence"`
}

// ListFactEdges GET /api/projects/:id/fact-edges
func (h *ProjectHandler) ListFactEdges(c *gin.Context) {
	projectID := c.Param("id")
	edges, err := h.db.ListProjectFactEdgesByProject(projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if edges == nil {
		edges = []*database.ProjectFactEdge{}
	}
	c.JSON(http.StatusOK, edges)
}

// CreateFactEdge POST /api/projects/:id/fact-edges
func (h *ProjectHandler) CreateFactEdge(c *gin.Context) {
	projectID := c.Param("id")
	var req createFactEdgeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	edge, err := h.db.AddProjectFactEdge(projectID, database.ProjectFactEdgeInput{
		To:         req.TargetFactKey,
		Type:       req.EdgeType,
		Confidence: req.Confidence,
	}, req.SourceFactKey, "")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if f, err := h.db.GetProjectFactByKey(projectID, req.TargetFactKey); err == nil {
		in, _ := h.db.ListIncomingProjectFactEdges(projectID, req.TargetFactKey)
		f.Body = project.SyncBodyLinksSection(f.Body, in)
		_, _ = h.db.UpsertProjectFact(f)
	}
	c.JSON(http.StatusOK, edge)
}

// DeleteFactEdge DELETE /api/projects/:id/fact-edges/:edgeId
func (h *ProjectHandler) DeleteFactEdge(c *gin.Context) {
	projectID := c.Param("id")
	edgeID := c.Param("edgeId")
	edge, err := h.db.GetProjectFactEdge(edgeID)
	if err != nil || edge.ProjectID != projectID {
		c.JSON(http.StatusNotFound, gin.H{"error": "边不存在"})
		return
	}
	if err := h.db.DeleteProjectFactEdge(edgeID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if f, err := h.db.GetProjectFactByKey(projectID, edge.TargetFactKey); err == nil {
		in, _ := h.db.ListIncomingProjectFactEdges(projectID, edge.TargetFactKey)
		f.Body = project.SyncBodyLinksSection(f.Body, in)
		_, _ = h.db.UpsertProjectFact(f)
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// PromoteAttackChain POST /api/projects/:id/promote-attack-chain/:conversationId
func (h *ProjectHandler) PromoteAttackChain(c *gin.Context) {
	projectID := c.Param("id")
	conversationID := c.Param("conversationId")
	session, ok := security.CurrentSession(c)
	if !ok || !h.db.UserCanAccessResource(session.UserID, session.Scope, "project", projectID) ||
		!h.db.UserCanAccessResource(session.UserID, session.Scope, "conversation", conversationID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问目标项目或来源对话"})
		return
	}
	result, err := attackchain.PromoteToProject(h.db, projectID, conversationID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}
