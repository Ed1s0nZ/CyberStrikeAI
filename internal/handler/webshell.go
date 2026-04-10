package handler

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"cyberstrike-ai/internal/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// WebShellHandler 代理执行 WebShell 命令（类似冰蝎/蚁剑），避免前端跨域并统一构建请求
type WebShellHandler struct {
	logger *zap.Logger
	client *http.Client
	db     *database.DB
}

// NewWebShellHandler 创建 WebShell 处理器，db 可为 nil（连接配置接口将不可用）
func NewWebShellHandler(logger *zap.Logger, db *database.DB) *WebShellHandler {
	return &WebShellHandler{
		logger: logger,
		client: &http.Client{
			Timeout:   30 * time.Second,
			Transport: &http.Transport{DisableKeepAlives: false},
		},
		db: db,
	}
}

// CreateConnectionRequest 创建连接请求
type CreateConnectionRequest struct {
	URL      string `json:"url" binding:"required"`
	Password string `json:"password"`
	Type     string `json:"type"`
	Method   string `json:"method"`
	CmdParam string `json:"cmd_param"`
	Remark   string `json:"remark"`
}

// UpdateConnectionRequest 更新连接请求
type UpdateConnectionRequest struct {
	URL      string `json:"url" binding:"required"`
	Password string `json:"password"`
	Type     string `json:"type"`
	Method   string `json:"method"`
	CmdParam string `json:"cmd_param"`
	Remark   string `json:"remark"`
}

// ListConnections 列出所有 WebShell 连接（GET /api/webshell/connections）
func (h *WebShellHandler) ListConnections(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database not available"})
		return
	}
	list, err := h.db.ListWebshellConnections()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if list == nil {
		list = []database.WebShellConnection{}
	}
	c.JSON(http.StatusOK, list)
}

// CreateConnection 创建 WebShell 连接（POST /api/webshell/connections）
func (h *WebShellHandler) CreateConnection(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database not available"})
		return
	}
	var req CreateConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.URL = strings.TrimSpace(req.URL)
	if req.URL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url is required"})
		return
	}
	if _, err := url.Parse(req.URL); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid url"})
		return
	}
	method := strings.ToLower(strings.TrimSpace(req.Method))
	if method != "get" && method != "post" {
		method = "post"
	}
	shellType := strings.ToLower(strings.TrimSpace(req.Type))
	if shellType == "" {
		shellType = "php"
	}
	conn := &database.WebShellConnection{
		ID:        "ws_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:12],
		URL:       req.URL,
		Password:  strings.TrimSpace(req.Password),
		Type:      shellType,
		Method:    method,
		CmdParam:  strings.TrimSpace(req.CmdParam),
		Remark:    strings.TrimSpace(req.Remark),
		CreatedAt: time.Now(),
	}
	if err := h.db.CreateWebshellConnection(conn); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, conn)
}

// UpdateConnection 更新 WebShell 连接（PUT /api/webshell/connections/:id）
func (h *WebShellHandler) UpdateConnection(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database not available"})
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}
	var req UpdateConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.URL = strings.TrimSpace(req.URL)
	if req.URL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url is required"})
		return
	}
	if _, err := url.Parse(req.URL); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid url"})
		return
	}
	method := strings.ToLower(strings.TrimSpace(req.Method))
	if method != "get" && method != "post" {
		method = "post"
	}
	shellType := strings.ToLower(strings.TrimSpace(req.Type))
	if shellType == "" {
		shellType = "php"
	}
	conn := &database.WebShellConnection{
		ID:       id,
		URL:      req.URL,
		Password: strings.TrimSpace(req.Password),
		Type:     shellType,
		Method:   method,
		CmdParam: strings.TrimSpace(req.CmdParam),
		Remark:   strings.TrimSpace(req.Remark),
	}
	if err := h.db.UpdateWebshellConnection(conn); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "connection not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	updated, _ := h.db.GetWebshellConnection(id)
	if updated != nil {
		c.JSON(http.StatusOK, updated)
	} else {
		c.JSON(http.StatusOK, conn)
	}
}

// DeleteConnection 删除 WebShell 连接（DELETE /api/webshell/connections/:id）
func (h *WebShellHandler) DeleteConnection(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database not available"})
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}
	if err := h.db.DeleteWebshellConnection(id); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "connection not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GetConnectionState 获取 WebShell 连接关联的前端持久化状态（GET /api/webshell/connections/:id/state）
func (h *WebShellHandler) GetConnectionState(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database not available"})
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}
	conn, err := h.db.GetWebshellConnection(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if conn == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "connection not found"})
		return
	}
	stateJSON, err := h.db.GetWebshellConnectionState(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	var state interface{}
	if err := json.Unmarshal([]byte(stateJSON), &state); err != nil {
		state = map[string]interface{}{}
	}
	c.JSON(http.StatusOK, gin.H{"state": state})
}

// SaveConnectionState 保存 WebShell 连接关联的前端持久化状态（PUT /api/webshell/connections/:id/state）
func (h *WebShellHandler) SaveConnectionState(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database not available"})
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}
	conn, err := h.db.GetWebshellConnection(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if conn == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "connection not found"})
		return
	}
	var req struct {
		State json.RawMessage `json:"state"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	raw := req.State
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	if len(raw) > 2*1024*1024 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "state payload too large (max 2MB)"})
		return
	}
	var anyJSON interface{}
	if err := json.Unmarshal(raw, &anyJSON); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "state must be valid json"})
		return
	}
	if err := h.db.UpsertWebshellConnectionState(id, string(raw)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GetAIHistory 获取指定 WebShell 连接的 AI 助手对话历史（GET /api/webshell/connections/:id/ai-history）
func (h *WebShellHandler) GetAIHistory(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database not available"})
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}
	conv, err := h.db.GetConversationByWebshellConnectionID(id)
	if err != nil {
		h.logger.Warn("获取 WebShell AI 对话失败", zap.String("connectionId", id), zap.Error(err))
		c.JSON(http.StatusOK, gin.H{"conversationId": nil, "messages": []database.Message{}})
		return
	}
	if conv == nil {
		c.JSON(http.StatusOK, gin.H{"conversationId": nil, "messages": []database.Message{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"conversationId": conv.ID, "messages": conv.Messages})
}

// ListAIConversations 列出该 WebShell 连接下的所有 AI 对话（供侧边栏）
func (h *WebShellHandler) ListAIConversations(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database not available"})
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}
	list, err := h.db.ListConversationsByWebshellConnectionID(id)
	if err != nil {
		h.logger.Warn("列出 WebShell AI 对话失败", zap.String("connectionId", id), zap.Error(err))
		c.JSON(http.StatusOK, []database.WebShellConversationItem{})
		return
	}
	if list == nil {
		list = []database.WebShellConversationItem{}
	}
	c.JSON(http.StatusOK, list)
}

// ExecRequest 执行命令请求（前端传入连接信息 + 命令）
type ExecRequest struct {
	URL      string `json:"url" binding:"required"`
	Password string `json:"password"`
	Type     string `json:"type"`      // php, asp, aspx, jsp, custom
	Method   string `json:"method"`    // GET 或 POST，空则默认 POST
	CmdParam string `json:"cmd_param"` // 命令参数名，如 cmd/xxx，空则默认 cmd
	Command  string `json:"command" binding:"required"`
}

// ExecResponse 执行命令响应
type ExecResponse struct {
	OK       bool   `json:"ok"`
	Output   string `json:"output"`
	Error    string `json:"error,omitempty"`
	HTTPCode int    `json:"http_code,omitempty"`
}

// FileOpRequest 文件操作请求
type FileOpRequest struct {
	URL        string `json:"url" binding:"required"`
	Password   string `json:"password"`
	Type       string `json:"type"`
	Method     string `json:"method"`                    // GET 或 POST，空则默认 POST
	CmdParam   string `json:"cmd_param"`                 // 命令参数名，如 cmd/xxx，空则默认 cmd
	Action     string `json:"action" binding:"required"` // list, read, delete, write, mkdir, rename, upload, upload_chunk
	Path       string `json:"path"`
	TargetPath string `json:"target_path"` // rename 时目标路径
	Content    string `json:"content"`     // write/upload 时使用
	ChunkIndex int    `json:"chunk_index"` // upload_chunk 时，0 表示首块
}

// FileOpResponse 文件操作响应
type FileOpResponse struct {
	OK     bool   `json:"ok"`
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

func (h *WebShellHandler) Exec(c *gin.Context) {
	var req ExecRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.URL = strings.TrimSpace(req.URL)
	req.Command = strings.TrimSpace(req.Command)
	if req.URL == "" || req.Command == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url and command are required"})
		return
	}

	parsed, err := url.Parse(req.URL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid url: only http(s) allowed"})
		return
	}

	useGET := strings.ToUpper(strings.TrimSpace(req.Method)) == "GET"
	cmdParam := strings.TrimSpace(req.CmdParam)
	if cmdParam == "" {
		cmdParam = "cmd"
	}
	var httpReq *http.Request
	if useGET {
		targetURL := h.buildExecURL(req.URL, req.Type, req.Password, cmdParam, req.Command)
		httpReq, err = http.NewRequest(http.MethodGet, targetURL, nil)
	} else {
		body := h.buildExecBody(req.Type, req.Password, cmdParam, req.Command)
		httpReq, err = http.NewRequest(http.MethodPost, req.URL, bytes.NewReader(body))
		httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if err != nil {
		h.logger.Warn("webshell exec NewRequest", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ExecResponse{OK: false, Error: err.Error()})
		return
	}
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (compatible; CyberStrikeAI-WebShell/1.0)")

	resp, err := h.client.Do(httpReq)
	if err != nil {
		h.logger.Warn("webshell exec Do", zap.String("url", req.URL), zap.Error(err))
		c.JSON(http.StatusOK, ExecResponse{OK: false, Error: err.Error()})
		return
	}
	defer resp.Body.Close()

	out, _ := io.ReadAll(resp.Body)
	output := string(out)
	httpCode := resp.StatusCode

	c.JSON(http.StatusOK, ExecResponse{
		OK:       resp.StatusCode == http.StatusOK,
		Output:   output,
		HTTPCode: httpCode,
	})
}

// buildExecBody 按常见 WebShell 约定构建 POST 体（多数使用 pass + cmd，可配置命令参数名）
func (h *WebShellHandler) buildExecBody(shellType, password, cmdParam, command string) []byte {
	form := h.execParams(shellType, password, cmdParam, command)
	return []byte(form.Encode())
}

// buildExecURL 构建 GET 请求的完整 URL（baseURL + ?pass=xxx&cmd=yyy，cmd 可配置）
func (h *WebShellHandler) buildExecURL(baseURL, shellType, password, cmdParam, command string) string {
	form := h.execParams(shellType, password, cmdParam, command)
	if parsed, err := url.Parse(baseURL); err == nil {
		parsed.RawQuery = form.Encode()
		return parsed.String()
	}
	return baseURL + "?" + form.Encode()
}

func (h *WebShellHandler) execParams(shellType, password, cmdParam, command string) url.Values {
	shellType = strings.ToLower(strings.TrimSpace(shellType))
	if shellType == "" {
		shellType = "php"
	}
	if strings.TrimSpace(cmdParam) == "" {
		cmdParam = "cmd"
	}
	form := url.Values{}
	form.Set("pass", password)
	form.Set(cmdParam, command)
	return form
}

func (h *WebShellHandler) FileOp(c *gin.Context) {
	var req FileOpRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.URL = strings.TrimSpace(req.URL)
	req.Action = strings.ToLower(strings.TrimSpace(req.Action))
	if req.URL == "" || req.Action == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url and action are required"})
		return
	}

	parsed, err := url.Parse(req.URL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid url: only http(s) allowed"})
		return
	}

	// 通过执行系统命令实现文件操作（与通用一句话兼容）
	var command string
	shellType := strings.ToLower(strings.TrimSpace(req.Type))
	switch req.Action {
	case "list":
		path := strings.TrimSpace(req.Path)
		if path == "" {
			path = "."
		}
		if shellType == "asp" || shellType == "aspx" {
			command = "dir " + h.escapePath(path)
		} else {
			command = "ls -la " + h.escapePath(path)
		}
	case "read":
		if shellType == "asp" || shellType == "aspx" {
			command = "type " + h.escapePath(strings.TrimSpace(req.Path))
		} else {
			command = "cat " + h.escapePath(strings.TrimSpace(req.Path))
		}
	case "delete":
		if shellType == "asp" || shellType == "aspx" {
			command = "del " + h.escapePath(strings.TrimSpace(req.Path))
		} else {
			command = "rm -f " + h.escapePath(strings.TrimSpace(req.Path))
		}
	case "write":
		path := h.escapePath(strings.TrimSpace(req.Path))
		command = "echo " + h.escapeForEcho(req.Content) + " > " + path
	case "mkdir":
		path := strings.TrimSpace(req.Path)
		if path == "" {
			c.JSON(http.StatusBadRequest, FileOpResponse{OK: false, Error: "path is required for mkdir"})
			return
		}
		if shellType == "asp" || shellType == "aspx" {
			command = "md " + h.escapePath(path)
		} else {
			command = "mkdir -p " + h.escapePath(path)
		}
	case "rename":
		oldPath := strings.TrimSpace(req.Path)
		newPath := strings.TrimSpace(req.TargetPath)
		if oldPath == "" || newPath == "" {
			c.JSON(http.StatusBadRequest, FileOpResponse{OK: false, Error: "path and target_path are required for rename"})
			return
		}
		if shellType == "asp" || shellType == "aspx" {
			command = "move /y " + h.escapePath(oldPath) + " " + h.escapePath(newPath)
		} else {
			command = "mv " + h.escapePath(oldPath) + " " + h.escapePath(newPath)
		}
	case "upload":
		path := strings.TrimSpace(req.Path)
		if path == "" {
			c.JSON(http.StatusBadRequest, FileOpResponse{OK: false, Error: "path is required for upload"})
			return
		}
		if len(req.Content) > 512*1024 {
			c.JSON(http.StatusBadRequest, FileOpResponse{OK: false, Error: "upload content too large (max 512KB base64)"})
			return
		}
		// base64 仅含 A-Za-z0-9+/=，用单引号包裹安全
		command = "echo " + "'" + req.Content + "'" + " | base64 -d > " + h.escapePath(path)
	case "upload_chunk":
		path := strings.TrimSpace(req.Path)
		if path == "" {
			c.JSON(http.StatusBadRequest, FileOpResponse{OK: false, Error: "path is required for upload_chunk"})
			return
		}
		redir := ">>"
		if req.ChunkIndex == 0 {
			redir = ">"
		}
		command = "echo " + "'" + req.Content + "'" + " | base64 -d " + redir + " " + h.escapePath(path)
	default:
		c.JSON(http.StatusBadRequest, FileOpResponse{OK: false, Error: "unsupported action: " + req.Action})
		return
	}

	useGET := strings.ToUpper(strings.TrimSpace(req.Method)) == "GET"
	cmdParam := strings.TrimSpace(req.CmdParam)
	if cmdParam == "" {
		cmdParam = "cmd"
	}
	var httpReq *http.Request
	if useGET {
		targetURL := h.buildExecURL(req.URL, req.Type, req.Password, cmdParam, command)
		httpReq, err = http.NewRequest(http.MethodGet, targetURL, nil)
	} else {
		body := h.buildExecBody(req.Type, req.Password, cmdParam, command)
		httpReq, err = http.NewRequest(http.MethodPost, req.URL, bytes.NewReader(body))
		httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, FileOpResponse{OK: false, Error: err.Error()})
		return
	}
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (compatible; CyberStrikeAI-WebShell/1.0)")

	resp, err := h.client.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusOK, FileOpResponse{OK: false, Error: err.Error()})
		return
	}
	defer resp.Body.Close()

	out, _ := io.ReadAll(resp.Body)
	output := string(out)

	c.JSON(http.StatusOK, FileOpResponse{
		OK:     resp.StatusCode == http.StatusOK,
		Output: output,
	})
}

func (h *WebShellHandler) escapePath(p string) string {
	if p == "" {
		return "."
	}
	// 简单转义空格与敏感字符，避免命令注入
	return "'" + strings.ReplaceAll(p, "'", "'\\''") + "'"
}

func (h *WebShellHandler) escapeForEcho(s string) string {
	// 仅用于 write：base64 写入更安全，这里简单用单引号包裹
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

// ExecWithConnection 在指定 WebShell 连接上执行命令（供 MCP/Agent 等非 HTTP 调用）
func (h *WebShellHandler) ExecWithConnection(conn *database.WebShellConnection, command string) (output string, ok bool, errMsg string) {
	if conn == nil {
		return "", false, "connection is nil"
	}
	command = strings.TrimSpace(command)
	if command == "" {
		return "", false, "command is required"
	}
	useGET := strings.ToUpper(strings.TrimSpace(conn.Method)) == "GET"
	cmdParam := strings.TrimSpace(conn.CmdParam)
	if cmdParam == "" {
		cmdParam = "cmd"
	}
	var httpReq *http.Request
	var err error
	if useGET {
		targetURL := h.buildExecURL(conn.URL, conn.Type, conn.Password, cmdParam, command)
		httpReq, err = http.NewRequest(http.MethodGet, targetURL, nil)
	} else {
		body := h.buildExecBody(conn.Type, conn.Password, cmdParam, command)
		httpReq, err = http.NewRequest(http.MethodPost, conn.URL, bytes.NewReader(body))
		httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if err != nil {
		return "", false, err.Error()
	}
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (compatible; CyberStrikeAI-WebShell/1.0)")
	resp, err := h.client.Do(httpReq)
	if err != nil {
		return "", false, err.Error()
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return string(out), resp.StatusCode == http.StatusOK, ""
}

// FileOpWithConnection 在指定 WebShell 连接上执行文件操作（供 MCP/Agent 调用），支持 list / read / write
func (h *WebShellHandler) FileOpWithConnection(conn *database.WebShellConnection, action, path, content, targetPath string) (output string, ok bool, errMsg string) {
	if conn == nil {
		return "", false, "connection is nil"
	}
	action = strings.ToLower(strings.TrimSpace(action))
	shellType := strings.ToLower(strings.TrimSpace(conn.Type))
	if shellType == "" {
		shellType = "php"
	}
	var command string
	switch action {
	case "list":
		if path == "" {
			path = "."
		}
		if shellType == "asp" || shellType == "aspx" {
			command = "dir " + h.escapePath(strings.TrimSpace(path))
		} else {
			command = "ls -la " + h.escapePath(strings.TrimSpace(path))
		}
	case "read":
		path = strings.TrimSpace(path)
		if path == "" {
			return "", false, "path is required for read"
		}
		if shellType == "asp" || shellType == "aspx" {
			command = "type " + h.escapePath(path)
		} else {
			command = "cat " + h.escapePath(path)
		}
	case "write":
		path = strings.TrimSpace(path)
		if path == "" {
			return "", false, "path is required for write"
		}
		command = "echo " + h.escapeForEcho(content) + " > " + h.escapePath(path)
	default:
		return "", false, "unsupported action: " + action + " (supported: list, read, write)"
	}
	useGET := strings.ToUpper(strings.TrimSpace(conn.Method)) == "GET"
	cmdParam := strings.TrimSpace(conn.CmdParam)
	if cmdParam == "" {
		cmdParam = "cmd"
	}
	var httpReq *http.Request
	var err error
	if useGET {
		targetURL := h.buildExecURL(conn.URL, conn.Type, conn.Password, cmdParam, command)
		httpReq, err = http.NewRequest(http.MethodGet, targetURL, nil)
	} else {
		body := h.buildExecBody(conn.Type, conn.Password, cmdParam, command)
		httpReq, err = http.NewRequest(http.MethodPost, conn.URL, bytes.NewReader(body))
		httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if err != nil {
		return "", false, err.Error()
	}
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (compatible; CyberStrikeAI-WebShell/1.0)")
	resp, err := h.client.Do(httpReq)
	if err != nil {
		return "", false, err.Error()
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return string(out), resp.StatusCode == http.StatusOK, ""
}
