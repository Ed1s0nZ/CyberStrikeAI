package handler

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"cyberstrike-ai/internal/agent"
	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/mcp/builtin"
	"cyberstrike-ai/internal/multiagent"
	"cyberstrike-ai/internal/skills"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// safeTruncateString 安全截断字符串，避免在 UTF-8 字符中间截断
func safeTruncateString(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}

	// 将字符串转换为 rune 切片以正确计算字符数
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}

	// 截断到最大长度
	truncated := string(runes[:maxLen])

	// 尝试在标点符号或空格处截断，使截断更自然
	// 在截断点往前查找合适的断点（不超过20%的长度）
	searchRange := maxLen / 5
	if searchRange > maxLen {
		searchRange = maxLen
	}
	breakChars := []rune("，。、 ,.;:!?！？/\\-_")
	bestBreakPos := len(runes[:maxLen])

	for i := bestBreakPos - 1; i >= bestBreakPos-searchRange && i >= 0; i-- {
		for _, breakChar := range breakChars {
			if runes[i] == breakChar {
				bestBreakPos = i + 1 // 在标点符号后断开
				goto found
			}
		}
	}

found:
	truncated = string(runes[:bestBreakPos])
	return truncated + "..."
}

// AgentHandler Agent处理器
type AgentHandler struct {
	agent            *agent.Agent
	db               *database.DB
	logger           *zap.Logger
	tasks            *AgentTaskManager
	batchTaskManager *BatchTaskManager
	config           *config.Config // 配置引用，用于获取角色信息
	knowledgeManager interface {    // 知识库管理器接口
		LogRetrieval(conversationID, messageID, query, riskType string, retrievedItems []string) error
	}
	skillsManager     *skills.Manager // Skills管理器
	agentsMarkdownDir string          // 多代理：Markdown 子 Agent 目录（绝对路径，空则不从磁盘合并）
}

// NewAgentHandler 创建新的Agent处理器
func NewAgentHandler(agent *agent.Agent, db *database.DB, cfg *config.Config, logger *zap.Logger) *AgentHandler {
	batchTaskManager := NewBatchTaskManager()
	batchTaskManager.SetDB(db)

	// 从数据库加载所有批量任务队列
	if err := batchTaskManager.LoadFromDB(); err != nil {
		logger.Warn("从数据库加载批量任务队列失败", zap.Error(err))
	}

	return &AgentHandler{
		agent:            agent,
		db:               db,
		logger:           logger,
		tasks:            NewAgentTaskManager(),
		batchTaskManager: batchTaskManager,
		config:           cfg,
	}
}

// SetKnowledgeManager 设置知识库管理器（用于记录检索日志）
func (h *AgentHandler) SetKnowledgeManager(manager interface {
	LogRetrieval(conversationID, messageID, query, riskType string, retrievedItems []string) error
}) {
	h.knowledgeManager = manager
}

// SetSkillsManager 设置Skills管理器
func (h *AgentHandler) SetSkillsManager(manager *skills.Manager) {
	h.skillsManager = manager
}

// SetAgentsMarkdownDir 设置 agents/*.md 子代理目录（绝对路径）；空表示仅使用 config.yaml 中的 sub_agents。
func (h *AgentHandler) SetAgentsMarkdownDir(absDir string) {
	h.agentsMarkdownDir = strings.TrimSpace(absDir)
}

// ChatAttachment 聊天附件（用户上传的文件）
type ChatAttachment struct {
	FileName   string `json:"fileName"`          // 展示用文件名
	Content    string `json:"content,omitempty"` // 文本或 base64；若已预先上传到服务器可留空
	MimeType   string `json:"mimeType,omitempty"`
	ServerPath string `json:"serverPath,omitempty"` // 已保存在 chat_uploads 下的绝对路径（由 POST /api/chat-uploads 返回）
}

// ChatRequest 聊天请求
type ChatRequest struct {
	Message              string           `json:"message" binding:"required"`
	ConversationID       string           `json:"conversationId,omitempty"`
	Role                 string           `json:"role,omitempty"` // 角色名称
	Attachments          []ChatAttachment `json:"attachments,omitempty"`
	WebShellConnectionID string           `json:"webshellConnectionId,omitempty"` // WebShell 管理 - AI 助手：当前选中的连接 ID，仅使用 webshell_* 工具
}

const (
	maxAttachments     = 10
	chatUploadsDirName = "chat_uploads" // 对话附件保存的根目录（相对当前工作目录）
)

// validateChatAttachmentServerPath 校验绝对路径落在工作目录 chat_uploads 下且为普通文件（防路径穿越）
func validateChatAttachmentServerPath(abs string) (string, error) {
	p := strings.TrimSpace(abs)
	if p == "" {
		return "", fmt.Errorf("empty path")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("获取当前工作目录失败: %w", err)
	}
	root := filepath.Join(cwd, chatUploadsDirName)
	rootAbs, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return "", err
	}
	pathAbs, err := filepath.Abs(filepath.Clean(p))
	if err != nil {
		return "", err
	}
	sep := string(filepath.Separator)
	if pathAbs != rootAbs && !strings.HasPrefix(pathAbs, rootAbs+sep) {
		return "", fmt.Errorf("path outside chat_uploads")
	}
	st, err := os.Stat(pathAbs)
	if err != nil {
		return "", err
	}
	if st.IsDir() {
		return "", fmt.Errorf("not a regular file")
	}
	return pathAbs, nil
}

// avoidChatUploadDestCollision 若 path 已存在则生成带时间戳+随机后缀的新文件名（与上传接口命名风格一致）
func avoidChatUploadDestCollision(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	nameNoExt := strings.TrimSuffix(base, ext)
	suffix := fmt.Sprintf("_%s_%s", time.Now().Format("150405"), shortRand(6))
	var unique string
	if ext != "" {
		unique = nameNoExt + suffix + ext
	} else {
		unique = base + suffix
	}
	return filepath.Join(dir, unique)
}

// relocateManualOrNewUploadToConversation 无会话 ID 时前端会上传到 …/日期/_manual；首条消息创建会话后，将文件移入 …/日期/{conversationId}/ 以便按对话隔离。
func relocateManualOrNewUploadToConversation(absPath, conversationID string, logger *zap.Logger) (string, error) {
	conv := strings.TrimSpace(conversationID)
	if conv == "" {
		return absPath, nil
	}
	convSan := strings.ReplaceAll(conv, string(filepath.Separator), "_")
	if convSan == "" || convSan == "_manual" || convSan == "_new" {
		return absPath, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return absPath, err
	}
	rootAbs, err := filepath.Abs(filepath.Join(cwd, chatUploadsDirName))
	if err != nil {
		return absPath, err
	}
	rel, err := filepath.Rel(rootAbs, absPath)
	if err != nil {
		return absPath, nil
	}
	rel = filepath.ToSlash(filepath.Clean(rel))
	var segs []string
	for _, p := range strings.Split(rel, "/") {
		if p != "" && p != "." {
			segs = append(segs, p)
		}
	}
	// 仅处理扁平结构：日期/_manual|_new/文件名
	if len(segs) != 3 {
		return absPath, nil
	}
	datePart, placeFolder, baseName := segs[0], segs[1], segs[2]
	if placeFolder != "_manual" && placeFolder != "_new" {
		return absPath, nil
	}
	targetDir := filepath.Join(rootAbs, datePart, convSan)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", fmt.Errorf("创建会话附件目录失败: %w", err)
	}
	dest := filepath.Join(targetDir, baseName)
	dest = avoidChatUploadDestCollision(dest)
	if err := os.Rename(absPath, dest); err != nil {
		return "", fmt.Errorf("将附件移入会话目录失败: %w", err)
	}
	out, _ := filepath.Abs(dest)
	if logger != nil {
		logger.Info("对话附件已从占位目录移入会话目录",
			zap.String("from", absPath),
			zap.String("to", out),
			zap.String("conversationId", conv))
	}
	return out, nil
}

// saveAttachmentsToDateAndConversationDir 处理附件：若带 serverPath 则仅校验已存在文件；否则将 content 写入 chat_uploads/YYYY-MM-DD/{conversationID}/。
// conversationID 为空时使用 "_new" 作为目录名（新对话尚未有 ID）
func saveAttachmentsToDateAndConversationDir(attachments []ChatAttachment, conversationID string, logger *zap.Logger) (savedPaths []string, err error) {
	if len(attachments) == 0 {
		return nil, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("获取当前工作目录失败: %w", err)
	}
	dateDir := filepath.Join(cwd, chatUploadsDirName, time.Now().Format("2006-01-02"))
	convDirName := strings.TrimSpace(conversationID)
	if convDirName == "" {
		convDirName = "_new"
	} else {
		convDirName = strings.ReplaceAll(convDirName, string(filepath.Separator), "_")
	}
	targetDir := filepath.Join(dateDir, convDirName)
	if err = os.MkdirAll(targetDir, 0755); err != nil {
		return nil, fmt.Errorf("创建上传目录失败: %w", err)
	}
	savedPaths = make([]string, 0, len(attachments))
	for i, a := range attachments {
		if sp := strings.TrimSpace(a.ServerPath); sp != "" {
			valid, verr := validateChatAttachmentServerPath(sp)
			if verr != nil {
				return nil, fmt.Errorf("附件 %s: %w", a.FileName, verr)
			}
			finalPath, rerr := relocateManualOrNewUploadToConversation(valid, conversationID, logger)
			if rerr != nil {
				return nil, fmt.Errorf("附件 %s: %w", a.FileName, rerr)
			}
			savedPaths = append(savedPaths, finalPath)
			if logger != nil {
				logger.Debug("对话附件使用已上传路径", zap.Int("index", i+1), zap.String("fileName", a.FileName), zap.String("path", finalPath))
			}
			continue
		}
		if strings.TrimSpace(a.Content) == "" {
			return nil, fmt.Errorf("附件 %s 缺少内容或未提供 serverPath", a.FileName)
		}
		raw, decErr := attachmentContentToBytes(a)
		if decErr != nil {
			return nil, fmt.Errorf("附件 %s 解码失败: %w", a.FileName, decErr)
		}
		baseName := filepath.Base(a.FileName)
		if baseName == "" || baseName == "." {
			baseName = "file"
		}
		baseName = strings.ReplaceAll(baseName, string(filepath.Separator), "_")
		ext := filepath.Ext(baseName)
		nameNoExt := strings.TrimSuffix(baseName, ext)
		suffix := fmt.Sprintf("_%s_%s", time.Now().Format("150405"), shortRand(6))
		var unique string
		if ext != "" {
			unique = nameNoExt + suffix + ext
		} else {
			unique = baseName + suffix
		}
		fullPath := filepath.Join(targetDir, unique)
		if err = os.WriteFile(fullPath, raw, 0644); err != nil {
			return nil, fmt.Errorf("写入文件 %s 失败: %w", a.FileName, err)
		}
		absPath, _ := filepath.Abs(fullPath)
		savedPaths = append(savedPaths, absPath)
		if logger != nil {
			logger.Debug("对话附件已保存", zap.Int("index", i+1), zap.String("fileName", a.FileName), zap.String("path", absPath))
		}
	}
	return savedPaths, nil
}

func shortRand(n int) string {
	const letters = "0123456789abcdef"
	b := make([]byte, n)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = letters[int(b[i])%len(letters)]
	}
	return string(b)
}

func attachmentContentToBytes(a ChatAttachment) ([]byte, error) {
	content := a.Content
	if decoded, err := base64.StdEncoding.DecodeString(content); err == nil && len(decoded) > 0 {
		return decoded, nil
	}
	return []byte(content), nil
}

// userMessageContentForStorage 返回要存入数据库的用户消息内容：有附件时在正文后追加附件名（及路径），刷新后仍能显示，继续对话时大模型也能从历史中拿到路径
func userMessageContentForStorage(message string, attachments []ChatAttachment, savedPaths []string) string {
	if len(attachments) == 0 {
		return message
	}
	var b strings.Builder
	b.WriteString(message)
	for i, a := range attachments {
		b.WriteString("\n📎 ")
		b.WriteString(a.FileName)
		if i < len(savedPaths) && savedPaths[i] != "" {
			b.WriteString(": ")
			b.WriteString(savedPaths[i])
		}
	}
	return b.String()
}

// appendAttachmentsToMessage 仅将附件的保存路径追加到用户消息末尾，不再内联附件内容，避免上下文过长
func appendAttachmentsToMessage(msg string, attachments []ChatAttachment, savedPaths []string) string {
	if len(attachments) == 0 {
		return msg
	}
	var b strings.Builder
	b.WriteString(msg)
	b.WriteString("\n\n[用户上传的文件已保存到以下路径（请按需读取文件内容，而不是依赖内联内容）]\n")
	for i, a := range attachments {
		if i < len(savedPaths) && savedPaths[i] != "" {
			b.WriteString(fmt.Sprintf("- %s: %s\n", a.FileName, savedPaths[i]))
		} else {
			b.WriteString(fmt.Sprintf("- %s: （路径未知，可能保存失败）\n", a.FileName))
		}
	}
	return b.String()
}

// ChatResponse 聊天响应
type ChatResponse struct {
	Response        string    `json:"response"`
	MCPExecutionIDs []string  `json:"mcpExecutionIds,omitempty"` // 本次对话中执行的MCP调用ID列表
	ConversationID  string    `json:"conversationId"`            // 对话ID
	Time            time.Time `json:"time"`
}

// AgentLoop 处理Agent Loop请求
func (h *AgentHandler) AgentLoop(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("收到Agent Loop请求",
		zap.String("message", req.Message),
		zap.String("conversationId", req.ConversationID),
	)

	// 如果没有对话ID，创建新对话
	conversationID := req.ConversationID
	if conversationID == "" {
		title := safeTruncateString(req.Message, 50)
		conv, err := h.db.CreateConversation(title)
		if err != nil {
			h.logger.Error("创建对话失败", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		conversationID = conv.ID
	} else {
		// 验证对话是否存在
		_, err := h.db.GetConversation(conversationID)
		if err != nil {
			h.logger.Error("对话不存在", zap.String("conversationId", conversationID), zap.Error(err))
			c.JSON(http.StatusNotFound, gin.H{"error": "对话不存在"})
			return
		}
	}

	// 优先尝试从保存的ReAct数据恢复历史上下文
	agentHistoryMessages, err := h.loadHistoryFromReActData(conversationID)
	if err != nil {
		h.logger.Warn("从ReAct数据加载历史消息失败，使用消息表", zap.Error(err))
		// 回退到使用数据库消息表
		historyMessages, err := h.db.GetMessages(conversationID)
		if err != nil {
			h.logger.Warn("获取历史消息失败", zap.Error(err))
			agentHistoryMessages = []agent.ChatMessage{}
		} else {
			// 将数据库消息转换为Agent消息格式
			agentHistoryMessages = make([]agent.ChatMessage, 0, len(historyMessages))
			for _, msg := range historyMessages {
				agentHistoryMessages = append(agentHistoryMessages, agent.ChatMessage{
					Role:    msg.Role,
					Content: msg.Content,
				})
			}
			h.logger.Info("从消息表加载历史消息", zap.Int("count", len(agentHistoryMessages)))
		}
	} else {
		h.logger.Info("从ReAct数据恢复历史上下文", zap.Int("count", len(agentHistoryMessages)))
	}

	// 校验附件数量（非流式）
	if len(req.Attachments) > maxAttachments {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("附件最多 %d 个", maxAttachments)})
		return
	}

	// 应用角色用户提示词和工具配置
	finalMessage := req.Message
	var roleTools []string  // 角色配置的工具列表
	var roleSkills []string // 角色配置的skills列表（用于提示AI，但不硬编码内容）

	// WebShell AI 助手模式：绑定当前连接，仅开放 webshell_* 工具并注入 connection_id
	if req.WebShellConnectionID != "" {
		conn, err := h.db.GetWebshellConnection(strings.TrimSpace(req.WebShellConnectionID))
		if err != nil || conn == nil {
			h.logger.Warn("WebShell AI 助手：未找到连接", zap.String("id", req.WebShellConnectionID), zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "未找到该 WebShell 连接"})
			return
		}
		remark := conn.Remark
		if remark == "" {
			remark = conn.URL
		}
		finalMessage = fmt.Sprintf("[WebShell 助手上下文] 当前连接 ID：%s，备注：%s。可用工具（仅在该连接上操作时使用，connection_id 填 \"%s\"）：webshell_exec、webshell_file_list、webshell_file_read、webshell_file_write、record_vulnerability、list_knowledge_risk_types、search_knowledge_base、list_skills、read_skill。请根据用户输入决定下一步：若仅为问候、闲聊或简单问题，直接简短回复即可，不必调用工具；当用户明确需要执行命令、列目录、读写文件、记录漏洞或检索知识库/查看 Skills 等操作时再调用上述工具。\n\n用户请求：%s",
			conn.ID, remark, conn.ID, req.Message)
		roleTools = []string{
			builtin.ToolWebshellExec,
			builtin.ToolWebshellFileList,
			builtin.ToolWebshellFileRead,
			builtin.ToolWebshellFileWrite,
			builtin.ToolRecordVulnerability,
			builtin.ToolListKnowledgeRiskTypes,
			builtin.ToolSearchKnowledgeBase,
			builtin.ToolListSkills,
			builtin.ToolReadSkill,
		}
		roleSkills = nil
	} else if req.Role != "" && req.Role != "默认" {
		if h.config.Roles != nil {
			if role, exists := h.config.Roles[req.Role]; exists && role.Enabled {
				// 应用用户提示词
				if role.UserPrompt != "" {
					finalMessage = role.UserPrompt + "\n\n" + req.Message
					h.logger.Info("应用角色用户提示词", zap.String("role", req.Role))
				}
				// 获取角色配置的工具列表（优先使用tools字段，向后兼容mcps字段）
				if len(role.Tools) > 0 {
					roleTools = role.Tools
					h.logger.Info("使用角色配置的工具列表", zap.String("role", req.Role), zap.Int("toolCount", len(roleTools)))
				}
				// 获取角色配置的skills列表（用于在系统提示词中提示AI，但不硬编码内容）
				if len(role.Skills) > 0 {
					roleSkills = role.Skills
					h.logger.Info("角色配置了skills，将在系统提示词中提示AI", zap.String("role", req.Role), zap.Int("skillCount", len(roleSkills)), zap.Strings("skills", roleSkills))
				}
			}
		}
	}
	var savedPaths []string
	if len(req.Attachments) > 0 {
		savedPaths, err = saveAttachmentsToDateAndConversationDir(req.Attachments, conversationID, h.logger)
		if err != nil {
			h.logger.Error("保存对话附件失败", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "保存上传文件失败: " + err.Error()})
			return
		}
	}
	finalMessage = appendAttachmentsToMessage(finalMessage, req.Attachments, savedPaths)

	// 保存用户消息：有附件时一并保存附件名与路径，刷新后显示、继续对话时大模型也能从历史中拿到路径
	userContent := userMessageContentForStorage(req.Message, req.Attachments, savedPaths)
	_, err = h.db.AddMessage(conversationID, "user", userContent, nil)
	if err != nil {
		h.logger.Error("保存用户消息失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存用户消息失败: " + err.Error()})
		return
	}

	// 执行Agent Loop，传入历史消息和对话ID（使用包含角色提示词的finalMessage和角色工具列表）
	// 注意：skills不会硬编码注入，但会在系统提示词中提示AI这个角色推荐使用哪些skills
	result, err := h.agent.AgentLoopWithProgress(c.Request.Context(), finalMessage, agentHistoryMessages, conversationID, nil, roleTools, roleSkills)
	if err != nil {
		h.logger.Error("Agent Loop执行失败", zap.Error(err))

		// 即使执行失败，也尝试保存ReAct数据（如果result中有）
		if result != nil && (result.LastReActInput != "" || result.LastReActOutput != "") {
			if saveErr := h.db.SaveReActData(conversationID, result.LastReActInput, result.LastReActOutput); saveErr != nil {
				h.logger.Warn("保存失败任务的ReAct数据失败", zap.Error(saveErr))
			} else {
				h.logger.Info("已保存失败任务的ReAct数据", zap.String("conversationId", conversationID))
			}
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 保存助手回复
	_, err = h.db.AddMessage(conversationID, "assistant", result.Response, result.MCPExecutionIDs)
	if err != nil {
		h.logger.Error("保存助手消息失败", zap.Error(err))
		// 即使保存失败，也返回响应，但记录错误
		// 因为AI已经生成了回复，用户应该能看到
	}

	// 保存最后一轮ReAct的输入和输出
	if result.LastReActInput != "" || result.LastReActOutput != "" {
		if err := h.db.SaveReActData(conversationID, result.LastReActInput, result.LastReActOutput); err != nil {
			h.logger.Warn("保存ReAct数据失败", zap.Error(err))
		} else {
			h.logger.Info("已保存ReAct数据", zap.String("conversationId", conversationID))
		}
	}

	c.JSON(http.StatusOK, ChatResponse{
		Response:        result.Response,
		MCPExecutionIDs: result.MCPExecutionIDs,
		ConversationID:  conversationID,
		Time:            time.Now(),
	})
}

// ProcessMessageForRobot 供机器人（企业微信/钉钉/飞书）调用：与 /api/agent-loop/stream 相同执行路径（含 progressCallback、过程详情），仅不发送 SSE，最后返回完整回复
func (h *AgentHandler) ProcessMessageForRobot(ctx context.Context, conversationID, message, role string) (response string, convID string, err error) {
	if conversationID == "" {
		title := safeTruncateString(message, 50)
		conv, createErr := h.db.CreateConversation(title)
		if createErr != nil {
			return "", "", fmt.Errorf("创建对话失败: %w", createErr)
		}
		conversationID = conv.ID
	} else {
		if _, getErr := h.db.GetConversation(conversationID); getErr != nil {
			return "", "", fmt.Errorf("对话不存在")
		}
	}

	agentHistoryMessages, err := h.loadHistoryFromReActData(conversationID)
	if err != nil {
		historyMessages, getErr := h.db.GetMessages(conversationID)
		if getErr != nil {
			agentHistoryMessages = []agent.ChatMessage{}
		} else {
			agentHistoryMessages = make([]agent.ChatMessage, 0, len(historyMessages))
			for _, msg := range historyMessages {
				agentHistoryMessages = append(agentHistoryMessages, agent.ChatMessage{Role: msg.Role, Content: msg.Content})
			}
		}
	}

	finalMessage := message
	var roleTools, roleSkills []string
	if role != "" && role != "默认" && h.config.Roles != nil {
		if r, exists := h.config.Roles[role]; exists && r.Enabled {
			if r.UserPrompt != "" {
				finalMessage = r.UserPrompt + "\n\n" + message
			}
			roleTools = r.Tools
			roleSkills = r.Skills
		}
	}

	if _, err = h.db.AddMessage(conversationID, "user", message, nil); err != nil {
		return "", "", fmt.Errorf("保存用户消息失败: %w", err)
	}

	// 与 agent-loop/stream 一致：先创建助手消息占位，用 progressCallback 写过程详情（不发送 SSE）
	assistantMsg, err := h.db.AddMessage(conversationID, "assistant", "处理中...", nil)
	if err != nil {
		h.logger.Warn("机器人：创建助手消息占位失败", zap.Error(err))
	}
	var assistantMessageID string
	if assistantMsg != nil {
		assistantMessageID = assistantMsg.ID
	}
	progressCallback := h.createProgressCallback(conversationID, assistantMessageID, nil)

	useRobotMulti := h.config != nil && h.config.MultiAgent.Enabled && h.config.MultiAgent.RobotUseMultiAgent
	if useRobotMulti {
		resultMA, errMA := multiagent.RunDeepAgent(
			ctx,
			h.config,
			&h.config.MultiAgent,
			h.agent,
			h.logger,
			conversationID,
			finalMessage,
			agentHistoryMessages,
			roleTools,
			progressCallback,
			h.agentsMarkdownDir,
		)
		if errMA != nil {
			errMsg := "执行失败: " + errMA.Error()
			if assistantMessageID != "" {
				_, _ = h.db.Exec("UPDATE messages SET content = ? WHERE id = ?", errMsg, assistantMessageID)
				_ = h.db.AddProcessDetail(assistantMessageID, conversationID, "error", errMsg, nil)
			}
			return "", conversationID, errMA
		}
		if assistantMessageID != "" {
			mcpIDsJSON := ""
			if len(resultMA.MCPExecutionIDs) > 0 {
				jsonData, _ := json.Marshal(resultMA.MCPExecutionIDs)
				mcpIDsJSON = string(jsonData)
			}
			_, err = h.db.Exec(
				"UPDATE messages SET content = ?, mcp_execution_ids = ? WHERE id = ?",
				resultMA.Response, mcpIDsJSON, assistantMessageID,
			)
			if err != nil {
				h.logger.Warn("机器人：更新助手消息失败", zap.Error(err))
			}
		} else {
			if _, err = h.db.AddMessage(conversationID, "assistant", resultMA.Response, resultMA.MCPExecutionIDs); err != nil {
				h.logger.Warn("机器人：保存助手消息失败", zap.Error(err))
			}
		}
		if resultMA.LastReActInput != "" || resultMA.LastReActOutput != "" {
			_ = h.db.SaveReActData(conversationID, resultMA.LastReActInput, resultMA.LastReActOutput)
		}
		return resultMA.Response, conversationID, nil
	}

	result, err := h.agent.AgentLoopWithProgress(ctx, finalMessage, agentHistoryMessages, conversationID, progressCallback, roleTools, roleSkills)
	if err != nil {
		errMsg := "执行失败: " + err.Error()
		if assistantMessageID != "" {
			_, _ = h.db.Exec("UPDATE messages SET content = ? WHERE id = ?", errMsg, assistantMessageID)
			_ = h.db.AddProcessDetail(assistantMessageID, conversationID, "error", errMsg, nil)
		}
		return "", conversationID, err
	}

	// 更新助手消息内容与 MCP 执行 ID（与 stream 一致）
	if assistantMessageID != "" {
		mcpIDsJSON := ""
		if len(result.MCPExecutionIDs) > 0 {
			jsonData, _ := json.Marshal(result.MCPExecutionIDs)
			mcpIDsJSON = string(jsonData)
		}
		_, err = h.db.Exec(
			"UPDATE messages SET content = ?, mcp_execution_ids = ? WHERE id = ?",
			result.Response, mcpIDsJSON, assistantMessageID,
		)
		if err != nil {
			h.logger.Warn("机器人：更新助手消息失败", zap.Error(err))
		}
	} else {
		if _, err = h.db.AddMessage(conversationID, "assistant", result.Response, result.MCPExecutionIDs); err != nil {
			h.logger.Warn("机器人：保存助手消息失败", zap.Error(err))
		}
	}
	if result.LastReActInput != "" || result.LastReActOutput != "" {
		_ = h.db.SaveReActData(conversationID, result.LastReActInput, result.LastReActOutput)
	}
	return result.Response, conversationID, nil
}

// StreamEvent 流式事件
type StreamEvent struct {
	Type    string      `json:"type"`    // conversation, progress, tool_call, tool_result, response, error, cancelled, done
	Message string      `json:"message"` // 显示消息
	Data    interface{} `json:"data,omitempty"`
}

// createProgressCallback 创建进度回调函数，用于保存processDetails
// sendEventFunc: 可选的流式事件发送函数，如果为nil则不发送流式事件
func (h *AgentHandler) createProgressCallback(conversationID, assistantMessageID string, sendEventFunc func(eventType, message string, data interface{})) agent.ProgressCallback {
	// 用于保存tool_call事件中的参数，以便在tool_result时使用
	toolCallCache := make(map[string]map[string]interface{}) // toolCallId -> arguments

	// thinking_stream_*：不逐条落库，按 streamId 聚合，在后续关键事件前补一条可持久化的 thinking
	type thinkingBuf struct {
		b    strings.Builder
		meta map[string]interface{}
	}
	thinkingStreams := make(map[string]*thinkingBuf) // streamId -> buf
	flushedThinking := make(map[string]bool)         // streamId -> flushed

	// response_start + response_delta：前端时间线显示为「📝 规划中」（monitor.js），不落逐条 delta；
	// 聚合为一条 planning 写入 process_details，刷新后与线上一致。
	var respPlan struct {
		meta map[string]interface{}
		b    strings.Builder
	}
	flushResponsePlan := func() {
		if assistantMessageID == "" {
			return
		}
		content := strings.TrimSpace(respPlan.b.String())
		if content == "" {
			respPlan.meta = nil
			respPlan.b.Reset()
			return
		}
		data := map[string]interface{}{
			"source": "response_stream",
		}
		for k, v := range respPlan.meta {
			data[k] = v
		}
		if err := h.db.AddProcessDetail(assistantMessageID, conversationID, "planning", content, data); err != nil {
			h.logger.Warn("保存过程详情失败", zap.Error(err), zap.String("eventType", "planning"))
		}
		respPlan.meta = nil
		respPlan.b.Reset()
	}

	flushThinkingStreams := func() {
		if assistantMessageID == "" {
			return
		}
		for sid, tb := range thinkingStreams {
			if sid == "" || flushedThinking[sid] || tb == nil {
				continue
			}
			content := strings.TrimSpace(tb.b.String())
			if content == "" {
				flushedThinking[sid] = true
				continue
			}
			data := map[string]interface{}{
				"streamId": sid,
			}
			for k, v := range tb.meta {
				// 避免覆盖 streamId
				if k == "streamId" {
					continue
				}
				data[k] = v
			}
			if err := h.db.AddProcessDetail(assistantMessageID, conversationID, "thinking", content, data); err != nil {
				h.logger.Warn("保存过程详情失败", zap.Error(err), zap.String("eventType", "thinking"))
			}
			flushedThinking[sid] = true
		}
	}

	return func(eventType, message string, data interface{}) {
		// 如果提供了sendEventFunc，发送流式事件
		if sendEventFunc != nil {
			sendEventFunc(eventType, message, data)
		}

		// 保存tool_call事件中的参数
		if eventType == "tool_call" {
			if dataMap, ok := data.(map[string]interface{}); ok {
				toolName, _ := dataMap["toolName"].(string)
				if toolName == builtin.ToolSearchKnowledgeBase {
					if toolCallId, ok := dataMap["toolCallId"].(string); ok && toolCallId != "" {
						if argumentsObj, ok := dataMap["argumentsObj"].(map[string]interface{}); ok {
							toolCallCache[toolCallId] = argumentsObj
						}
					}
				}
			}
		}

		// 处理知识检索日志记录
		if eventType == "tool_result" && h.knowledgeManager != nil {
			if dataMap, ok := data.(map[string]interface{}); ok {
				toolName, _ := dataMap["toolName"].(string)
				if toolName == builtin.ToolSearchKnowledgeBase {
					// 提取检索信息
					query := ""
					riskType := ""
					var retrievedItems []string

					// 首先尝试从tool_call缓存中获取参数
					if toolCallId, ok := dataMap["toolCallId"].(string); ok && toolCallId != "" {
						if cachedArgs, exists := toolCallCache[toolCallId]; exists {
							if q, ok := cachedArgs["query"].(string); ok && q != "" {
								query = q
							}
							if rt, ok := cachedArgs["risk_type"].(string); ok && rt != "" {
								riskType = rt
							}
							// 使用后清理缓存
							delete(toolCallCache, toolCallId)
						}
					}

					// 如果缓存中没有，尝试从argumentsObj中提取
					if query == "" {
						if arguments, ok := dataMap["argumentsObj"].(map[string]interface{}); ok {
							if q, ok := arguments["query"].(string); ok && q != "" {
								query = q
							}
							if rt, ok := arguments["risk_type"].(string); ok && rt != "" {
								riskType = rt
							}
						}
					}

					// 如果query仍然为空，尝试从result中提取（从结果文本的第一行）
					if query == "" {
						if result, ok := dataMap["result"].(string); ok && result != "" {
							// 尝试从结果中提取查询内容（如果结果包含"未找到与查询 'xxx' 相关的知识"）
							if strings.Contains(result, "未找到与查询 '") {
								start := strings.Index(result, "未找到与查询 '") + len("未找到与查询 '")
								end := strings.Index(result[start:], "'")
								if end > 0 {
									query = result[start : start+end]
								}
							}
						}
						// 如果还是为空，使用默认值
						if query == "" {
							query = "未知查询"
						}
					}

					// 从工具结果中提取检索到的知识项ID
					// 结果格式："找到 X 条相关知识：\n\n--- 结果 1 (相似度: XX.XX%) ---\n来源: [分类] 标题\n...\n<!-- METADATA: {...} -->"
					if result, ok := dataMap["result"].(string); ok && result != "" {
						// 尝试从元数据中提取知识项ID
						metadataMatch := strings.Index(result, "<!-- METADATA:")
						if metadataMatch > 0 {
							// 提取元数据JSON
							metadataStart := metadataMatch + len("<!-- METADATA: ")
							metadataEnd := strings.Index(result[metadataStart:], " -->")
							if metadataEnd > 0 {
								metadataJSON := result[metadataStart : metadataStart+metadataEnd]
								var metadata map[string]interface{}
								if err := json.Unmarshal([]byte(metadataJSON), &metadata); err == nil {
									if meta, ok := metadata["_metadata"].(map[string]interface{}); ok {
										if ids, ok := meta["retrievedItemIDs"].([]interface{}); ok {
											retrievedItems = make([]string, 0, len(ids))
											for _, id := range ids {
												if idStr, ok := id.(string); ok {
													retrievedItems = append(retrievedItems, idStr)
												}
											}
										}
									}
								}
							}
						}

						// 如果没有从元数据中提取到，但结果包含"找到 X 条"，至少标记为有结果
						if len(retrievedItems) == 0 && strings.Contains(result, "找到") && !strings.Contains(result, "未找到") {
							// 有结果，但无法准确提取ID，使用特殊标记
							retrievedItems = []string{"_has_results"}
						}
					}

					// 记录检索日志（异步，不阻塞）
					go func() {
						if err := h.knowledgeManager.LogRetrieval(conversationID, assistantMessageID, query, riskType, retrievedItems); err != nil {
							h.logger.Warn("记录知识检索日志失败", zap.Error(err))
						}
					}()

					// 添加知识检索事件到processDetails
					if assistantMessageID != "" {
						retrievalData := map[string]interface{}{
							"query":    query,
							"riskType": riskType,
							"toolName": toolName,
						}
						if err := h.db.AddProcessDetail(assistantMessageID, conversationID, "knowledge_retrieval", fmt.Sprintf("检索知识: %s", query), retrievalData); err != nil {
							h.logger.Warn("保存知识检索详情失败", zap.Error(err))
						}
					}
				}
			}
		}

		// 子代理回复流式增量不落库；结束时合并为一条 eino_agent_reply
		if assistantMessageID != "" && eventType == "eino_agent_reply_stream_end" {
			flushResponsePlan()
			// 确保思考流在子代理回复前能持久化（刷新后可读）
			flushThinkingStreams()
			if err := h.db.AddProcessDetail(assistantMessageID, conversationID, "eino_agent_reply", message, data); err != nil {
				h.logger.Warn("保存过程详情失败", zap.Error(err), zap.String("eventType", eventType))
			}
			return
		}

		// 多代理主代理「规划中」：response_start / response_delta 仅用于 SSE，聚合落一条 planning
		if eventType == "response_start" {
			flushResponsePlan()
			respPlan.meta = nil
			if dataMap, ok := data.(map[string]interface{}); ok {
				respPlan.meta = make(map[string]interface{}, len(dataMap))
				for k, v := range dataMap {
					respPlan.meta[k] = v
				}
			}
			respPlan.b.Reset()
			return
		}
		if eventType == "response_delta" {
			respPlan.b.WriteString(message)
			if dataMap, ok := data.(map[string]interface{}); ok && respPlan.meta == nil {
				respPlan.meta = make(map[string]interface{}, len(dataMap))
				for k, v := range dataMap {
					respPlan.meta[k] = v
				}
			} else if dataMap, ok := data.(map[string]interface{}); ok {
				for k, v := range dataMap {
					respPlan.meta[k] = v
				}
			}
			return
		}
		if eventType == "response" {
			flushResponsePlan()
			return
		}

		// 聚合 thinking_stream_*（ReasoningContent），不逐条落库
		if eventType == "thinking_stream_start" {
			if dataMap, ok := data.(map[string]interface{}); ok {
				if sid, ok2 := dataMap["streamId"].(string); ok2 && sid != "" {
					tb := thinkingStreams[sid]
					if tb == nil {
						tb = &thinkingBuf{meta: map[string]interface{}{}}
						thinkingStreams[sid] = tb
					}
					// 记录元信息（source/einoAgent/einoRole/iteration 等）
					for k, v := range dataMap {
						tb.meta[k] = v
					}
				}
			}
			return
		}
		if eventType == "thinking_stream_delta" {
			if dataMap, ok := data.(map[string]interface{}); ok {
				if sid, ok2 := dataMap["streamId"].(string); ok2 && sid != "" {
					tb := thinkingStreams[sid]
					if tb == nil {
						tb = &thinkingBuf{meta: map[string]interface{}{}}
						thinkingStreams[sid] = tb
					}
					// delta 片段直接拼接；message 本身就是 reasoning content
					tb.b.WriteString(message)
					// 有时 delta 先到 start 未到，补充元信息
					for k, v := range dataMap {
						tb.meta[k] = v
					}
				}
			}
			return
		}

		// 当 Agent 同时发送 thinking_stream_* 和 thinking（带同一 streamId）时，
		// thinking_stream_* 已经会在 flushThinkingStreams() 聚合落库；
		// 这里跳过同 streamId 的 thinking，避免 processDetails 双份展示。
		if eventType == "thinking" {
			if dataMap, ok := data.(map[string]interface{}); ok {
				if sid, ok2 := dataMap["streamId"].(string); ok2 && sid != "" {
					if tb, exists := thinkingStreams[sid]; exists && tb != nil {
						if strings.TrimSpace(tb.b.String()) != "" {
							return
						}
					}
					if flushedThinking[sid] {
						return
					}
				}
			}
		}

		// 保存过程详情到数据库（排除 response/done；response 正文已在 messages 表）
		// response_start/response_delta 已聚合为 planning，不落逐条。
		if assistantMessageID != "" &&
			eventType != "response" &&
			eventType != "done" &&
			eventType != "response_start" &&
			eventType != "response_delta" &&
			eventType != "tool_result_delta" &&
			eventType != "eino_agent_reply_stream_start" &&
			eventType != "eino_agent_reply_stream_delta" &&
			eventType != "eino_agent_reply_stream_end" {
			// 在关键过程事件落库前，先把「规划中」与 thinking_stream 落库
			flushResponsePlan()
			flushThinkingStreams()
			if err := h.db.AddProcessDetail(assistantMessageID, conversationID, eventType, message, data); err != nil {
				h.logger.Warn("保存过程详情失败", zap.Error(err), zap.String("eventType", eventType))
			}
		}
	}
}

// AgentLoopStream 处理Agent Loop流式请求
func (h *AgentHandler) AgentLoopStream(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 对于流式请求，也发送SSE格式的错误
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		event := StreamEvent{
			Type:    "error",
			Message: "请求参数错误: " + err.Error(),
		}
		eventJSON, _ := json.Marshal(event)
		fmt.Fprintf(c.Writer, "data: %s\n\n", eventJSON)
		c.Writer.Flush()
		return
	}

	h.logger.Info("收到Agent Loop流式请求",
		zap.String("message", req.Message),
		zap.String("conversationId", req.ConversationID),
	)

	// 设置SSE响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no") // 禁用nginx缓冲

	// 发送初始事件
	// 用于跟踪客户端是否已断开连接
	clientDisconnected := false
	// 与 sseKeepalive 共用：禁止并发写 ResponseWriter，否则会破坏 chunked 编码（ERR_INVALID_CHUNKED_ENCODING）。
	var sseWriteMu sync.Mutex
	// 用于快速确认模型是否真的产生了流式 delta
	var responseDeltaCount int
	var responseStartLogged bool

	sendEvent := func(eventType, message string, data interface{}) {
		if eventType == "response_start" {
			responseDeltaCount = 0
			responseStartLogged = true
			h.logger.Info("SSE: response_start",
				zap.Int("conversationIdPresent", func() int {
					if m, ok := data.(map[string]interface{}); ok {
						if v, ok2 := m["conversationId"]; ok2 && v != nil && fmt.Sprint(v) != "" {
							return 1
						}
					}
					return 0
				}()),
				zap.String("messageGeneratedBy", func() string {
					if m, ok := data.(map[string]interface{}); ok {
						if v, ok2 := m["messageGeneratedBy"]; ok2 {
							if s, ok3 := v.(string); ok3 {
								return s
							}
							return fmt.Sprint(v)
						}
					}
					return ""
				}()),
			)
		} else if eventType == "response_delta" {
			responseDeltaCount++
			// 只打前几条，避免刷屏
			if responseStartLogged && responseDeltaCount <= 3 {
				h.logger.Info("SSE: response_delta",
					zap.Int("index", responseDeltaCount),
					zap.Int("deltaLen", len(message)),
					zap.String("deltaPreview", func() string {
						p := strings.ReplaceAll(message, "\n", "\\n")
						if len(p) > 80 {
							return p[:80] + "..."
						}
						return p
					}()),
				)
			}
		}

		// 如果客户端已断开，不再发送事件
		if clientDisconnected {
			return
		}

		// 检查请求上下文是否被取消（客户端断开）
		select {
		case <-c.Request.Context().Done():
			clientDisconnected = true
			return
		default:
		}

		event := StreamEvent{
			Type:    eventType,
			Message: message,
			Data:    data,
		}
		eventJSON, _ := json.Marshal(event)

		sseWriteMu.Lock()
		_, err := fmt.Fprintf(c.Writer, "data: %s\n\n", eventJSON)
		if err != nil {
			sseWriteMu.Unlock()
			clientDisconnected = true
			h.logger.Debug("客户端断开连接，停止发送SSE事件", zap.Error(err))
			return
		}
		if flusher, ok := c.Writer.(http.Flusher); ok {
			flusher.Flush()
		} else {
			c.Writer.Flush()
		}
		sseWriteMu.Unlock()
	}

	// 如果没有对话ID，创建新对话（WebShell 助手模式下关联连接 ID 以便持久化展示）
	conversationID := req.ConversationID
	if conversationID == "" {
		title := safeTruncateString(req.Message, 50)
		var conv *database.Conversation
		var err error
		if req.WebShellConnectionID != "" {
			conv, err = h.db.CreateConversationWithWebshell(strings.TrimSpace(req.WebShellConnectionID), title)
		} else {
			conv, err = h.db.CreateConversation(title)
		}
		if err != nil {
			h.logger.Error("创建对话失败", zap.Error(err))
			sendEvent("error", "创建对话失败: "+err.Error(), nil)
			return
		}
		conversationID = conv.ID
		sendEvent("conversation", "会话已创建", map[string]interface{}{
			"conversationId": conversationID,
		})
	} else {
		// 验证对话是否存在
		_, err := h.db.GetConversation(conversationID)
		if err != nil {
			h.logger.Error("对话不存在", zap.String("conversationId", conversationID), zap.Error(err))
			sendEvent("error", "对话不存在", nil)
			return
		}
	}

	// 优先尝试从保存的ReAct数据恢复历史上下文
	agentHistoryMessages, err := h.loadHistoryFromReActData(conversationID)
	if err != nil {
		h.logger.Warn("从ReAct数据加载历史消息失败，使用消息表", zap.Error(err))
		// 回退到使用数据库消息表
		historyMessages, err := h.db.GetMessages(conversationID)
		if err != nil {
			h.logger.Warn("获取历史消息失败", zap.Error(err))
			agentHistoryMessages = []agent.ChatMessage{}
		} else {
			// 将数据库消息转换为Agent消息格式
			agentHistoryMessages = make([]agent.ChatMessage, 0, len(historyMessages))
			for _, msg := range historyMessages {
				agentHistoryMessages = append(agentHistoryMessages, agent.ChatMessage{
					Role:    msg.Role,
					Content: msg.Content,
				})
			}
			h.logger.Info("从消息表加载历史消息", zap.Int("count", len(agentHistoryMessages)))
		}
	} else {
		h.logger.Info("从ReAct数据恢复历史上下文", zap.Int("count", len(agentHistoryMessages)))
	}

	// 校验附件数量
	if len(req.Attachments) > maxAttachments {
		sendEvent("error", fmt.Sprintf("附件最多 %d 个", maxAttachments), nil)
		return
	}

	// 应用角色用户提示词和工具配置
	finalMessage := req.Message
	var roleTools []string // 角色配置的工具列表
	var roleSkills []string
	if req.WebShellConnectionID != "" {
		conn, errConn := h.db.GetWebshellConnection(strings.TrimSpace(req.WebShellConnectionID))
		if errConn != nil || conn == nil {
			h.logger.Warn("WebShell AI 助手：未找到连接", zap.String("id", req.WebShellConnectionID), zap.Error(errConn))
			sendEvent("error", "未找到该 WebShell 连接", nil)
			return
		}
		remark := conn.Remark
		if remark == "" {
			remark = conn.URL
		}
		finalMessage = fmt.Sprintf("[WebShell 助手上下文] 当前连接 ID：%s，备注：%s。可用工具（仅在该连接上操作时使用，connection_id 填 \"%s\"）：webshell_exec、webshell_file_list、webshell_file_read、webshell_file_write、record_vulnerability、list_knowledge_risk_types、search_knowledge_base、list_skills、read_skill。请根据用户输入决定下一步：若仅为问候、闲聊或简单问题，直接简短回复即可，不必调用工具；当用户明确需要执行命令、列目录、读写文件、记录漏洞或检索知识库/查看 Skills 等操作时再调用上述工具。\n\n用户请求：%s",
			conn.ID, remark, conn.ID, req.Message)
		roleTools = []string{
			builtin.ToolWebshellExec,
			builtin.ToolWebshellFileList,
			builtin.ToolWebshellFileRead,
			builtin.ToolWebshellFileWrite,
			builtin.ToolRecordVulnerability,
			builtin.ToolListKnowledgeRiskTypes,
			builtin.ToolSearchKnowledgeBase,
			builtin.ToolListSkills,
			builtin.ToolReadSkill,
		}
	} else if req.Role != "" && req.Role != "默认" {
		if h.config.Roles != nil {
			if role, exists := h.config.Roles[req.Role]; exists && role.Enabled {
				// 应用用户提示词
				if role.UserPrompt != "" {
					finalMessage = role.UserPrompt + "\n\n" + req.Message
					h.logger.Info("应用角色用户提示词", zap.String("role", req.Role))
				}
				// 获取角色配置的工具列表（优先使用tools字段，向后兼容mcps字段）
				if len(role.Tools) > 0 {
					roleTools = role.Tools
					h.logger.Info("使用角色配置的工具列表", zap.String("role", req.Role), zap.Int("toolCount", len(roleTools)))
				} else if len(role.MCPs) > 0 {
					// 向后兼容：如果只有mcps字段，暂时使用空列表（表示使用所有工具）
					// 因为mcps是MCP服务器名称，不是工具列表
					h.logger.Info("角色配置使用旧的mcps字段，将使用所有工具", zap.String("role", req.Role))
				}
				// 注意：角色配置的skills不再硬编码注入，AI可以通过list_skills和read_skill工具按需调用
				if len(role.Skills) > 0 {
					roleSkills = role.Skills
					h.logger.Info("角色配置了skills，AI可通过工具按需调用", zap.String("role", req.Role), zap.Int("skillCount", len(role.Skills)), zap.Strings("skills", role.Skills))
				}
			}
		}
	}
	var savedPaths []string
	if len(req.Attachments) > 0 {
		savedPaths, err = saveAttachmentsToDateAndConversationDir(req.Attachments, conversationID, h.logger)
		if err != nil {
			h.logger.Error("保存对话附件失败", zap.Error(err))
			sendEvent("error", "保存上传文件失败: "+err.Error(), nil)
			return
		}
	}
	// 仅将附件保存路径追加到 finalMessage，避免将文件内容内联到大模型上下文中
	finalMessage = appendAttachmentsToMessage(finalMessage, req.Attachments, savedPaths)
	// 如果roleTools为空，表示使用所有工具（默认角色或未配置工具的角色）

	// 保存用户消息：有附件时一并保存附件名与路径，刷新后显示、继续对话时大模型也能从历史中拿到路径
	userContent := userMessageContentForStorage(req.Message, req.Attachments, savedPaths)
	userMsgRow, err := h.db.AddMessage(conversationID, "user", userContent, nil)
	if err != nil {
		h.logger.Error("保存用户消息失败", zap.Error(err))
	}

	// 预先创建助手消息，以便关联过程详情
	assistantMsg, err := h.db.AddMessage(conversationID, "assistant", "处理中...", nil)
	if err != nil {
		h.logger.Error("创建助手消息失败", zap.Error(err))
		// 如果创建失败，继续执行但不保存过程详情
		assistantMsg = nil
	}

	// 创建进度回调函数，同时保存到数据库
	var assistantMessageID string
	if assistantMsg != nil {
		assistantMessageID = assistantMsg.ID
	}

	// 尽早下发消息 ID，便于前端在流式结束前挂上「删除本轮」等（无需等整段结束再刷新）
	if userMsgRow != nil {
		sendEvent("message_saved", "", map[string]interface{}{
			"conversationId": conversationID,
			"userMessageId":  userMsgRow.ID,
		})
	}

	// 创建进度回调函数，复用统一逻辑
	progressCallback := h.createProgressCallback(conversationID, assistantMessageID, sendEvent)

	// 创建一个独立的上下文用于任务执行，不随HTTP请求取消
	// 这样即使客户端断开连接（如刷新页面），任务也能继续执行
	baseCtx, cancelWithCause := context.WithCancelCause(context.Background())
	taskCtx, timeoutCancel := context.WithTimeout(baseCtx, 600*time.Minute)
	defer timeoutCancel()
	defer cancelWithCause(nil)

	if _, err := h.tasks.StartTask(conversationID, req.Message, cancelWithCause); err != nil {
		var errorMsg string
		if errors.Is(err, ErrTaskAlreadyRunning) {
			errorMsg = "⚠️ 当前会话已有任务正在执行中，请等待当前任务完成或点击「停止任务」按钮后再尝试。"
			sendEvent("error", errorMsg, map[string]interface{}{
				"conversationId": conversationID,
				"errorType":      "task_already_running",
			})
		} else {
			errorMsg = "❌ 无法启动任务: " + err.Error()
			sendEvent("error", errorMsg, map[string]interface{}{
				"conversationId": conversationID,
				"errorType":      "task_start_failed",
			})
		}

		// 更新助手消息内容并保存错误详情到数据库
		if assistantMessageID != "" {
			if _, updateErr := h.db.Exec(
				"UPDATE messages SET content = ? WHERE id = ?",
				errorMsg,
				assistantMessageID,
			); updateErr != nil {
				h.logger.Warn("更新错误后的助手消息失败", zap.Error(updateErr))
			}
			// 保存错误详情到数据库
			if err := h.db.AddProcessDetail(assistantMessageID, conversationID, "error", errorMsg, map[string]interface{}{
				"errorType": func() string {
					if errors.Is(err, ErrTaskAlreadyRunning) {
						return "task_already_running"
					}
					return "task_start_failed"
				}(),
			}); err != nil {
				h.logger.Warn("保存错误详情失败", zap.Error(err))
			}
		}

		sendEvent("done", "", map[string]interface{}{
			"conversationId": conversationID,
		})
		return
	}

	taskStatus := "completed"
	defer h.tasks.FinishTask(conversationID, taskStatus)

	// 执行Agent Loop，传入独立的上下文，确保任务不会因客户端断开而中断（使用包含角色提示词的finalMessage和角色工具列表）
	sendEvent("progress", "正在分析您的请求...", nil)
	// 注意：roleSkills 已在上方根据 req.Role 或 WebShell 模式设置
	stopKeepalive := make(chan struct{})
	go sseKeepalive(c, stopKeepalive, &sseWriteMu)
	defer close(stopKeepalive)

	result, err := h.agent.AgentLoopWithProgress(taskCtx, finalMessage, agentHistoryMessages, conversationID, progressCallback, roleTools, roleSkills)
	if err != nil {
		h.logger.Error("Agent Loop执行失败", zap.Error(err))
		cause := context.Cause(baseCtx)

		// 检查是否是用户取消：context的cause是ErrTaskCancelled
		// 如果cause是ErrTaskCancelled，无论错误是什么类型（包括context.Canceled），都视为用户取消
		// 这样可以正确处理在API调用过程中被取消的情况
		isCancelled := errors.Is(cause, ErrTaskCancelled)

		switch {
		case isCancelled:
			taskStatus = "cancelled"
			cancelMsg := "任务已被用户取消，后续操作已停止。"

			// 在发送事件前更新任务状态，确保前端能及时看到状态变化
			h.tasks.UpdateTaskStatus(conversationID, taskStatus)

			if assistantMessageID != "" {
				if _, updateErr := h.db.Exec(
					"UPDATE messages SET content = ? WHERE id = ?",
					cancelMsg,
					assistantMessageID,
				); updateErr != nil {
					h.logger.Warn("更新取消后的助手消息失败", zap.Error(updateErr))
				}
				h.db.AddProcessDetail(assistantMessageID, conversationID, "cancelled", cancelMsg, nil)
			}

			// 即使任务被取消，也尝试保存ReAct数据（如果result中有）
			if result != nil && (result.LastReActInput != "" || result.LastReActOutput != "") {
				if err := h.db.SaveReActData(conversationID, result.LastReActInput, result.LastReActOutput); err != nil {
					h.logger.Warn("保存取消任务的ReAct数据失败", zap.Error(err))
				} else {
					h.logger.Info("已保存取消任务的ReAct数据", zap.String("conversationId", conversationID))
				}
			}

			sendEvent("cancelled", cancelMsg, map[string]interface{}{
				"conversationId": conversationID,
				"messageId":      assistantMessageID,
			})
			sendEvent("done", "", map[string]interface{}{
				"conversationId": conversationID,
			})
			return
		case errors.Is(err, context.DeadlineExceeded) || errors.Is(cause, context.DeadlineExceeded):
			taskStatus = "timeout"
			timeoutMsg := "任务执行超时，已自动终止。"

			// 在发送事件前更新任务状态，确保前端能及时看到状态变化
			h.tasks.UpdateTaskStatus(conversationID, taskStatus)

			if assistantMessageID != "" {
				if _, updateErr := h.db.Exec(
					"UPDATE messages SET content = ? WHERE id = ?",
					timeoutMsg,
					assistantMessageID,
				); updateErr != nil {
					h.logger.Warn("更新超时后的助手消息失败", zap.Error(updateErr))
				}
				h.db.AddProcessDetail(assistantMessageID, conversationID, "timeout", timeoutMsg, nil)
			}

			// 即使任务超时，也尝试保存ReAct数据（如果result中有）
			if result != nil && (result.LastReActInput != "" || result.LastReActOutput != "") {
				if err := h.db.SaveReActData(conversationID, result.LastReActInput, result.LastReActOutput); err != nil {
					h.logger.Warn("保存超时任务的ReAct数据失败", zap.Error(err))
				} else {
					h.logger.Info("已保存超时任务的ReAct数据", zap.String("conversationId", conversationID))
				}
			}

			sendEvent("error", timeoutMsg, map[string]interface{}{
				"conversationId": conversationID,
				"messageId":      assistantMessageID,
			})
			sendEvent("done", "", map[string]interface{}{
				"conversationId": conversationID,
			})
			return
		default:
			taskStatus = "failed"
			errorMsg := "执行失败: " + err.Error()

			// 在发送事件前更新任务状态，确保前端能及时看到状态变化
			h.tasks.UpdateTaskStatus(conversationID, taskStatus)

			if assistantMessageID != "" {
				if _, updateErr := h.db.Exec(
					"UPDATE messages SET content = ? WHERE id = ?",
					errorMsg,
					assistantMessageID,
				); updateErr != nil {
					h.logger.Warn("更新失败后的助手消息失败", zap.Error(updateErr))
				}
				h.db.AddProcessDetail(assistantMessageID, conversationID, "error", errorMsg, nil)
			}

			// 即使任务失败，也尝试保存ReAct数据（如果result中有）
			if result != nil && (result.LastReActInput != "" || result.LastReActOutput != "") {
				if err := h.db.SaveReActData(conversationID, result.LastReActInput, result.LastReActOutput); err != nil {
					h.logger.Warn("保存失败任务的ReAct数据失败", zap.Error(err))
				} else {
					h.logger.Info("已保存失败任务的ReAct数据", zap.String("conversationId", conversationID))
				}
			}

			sendEvent("error", errorMsg, map[string]interface{}{
				"conversationId": conversationID,
				"messageId":      assistantMessageID,
			})
			sendEvent("done", "", map[string]interface{}{
				"conversationId": conversationID,
			})
		}
		return
	}

	// 更新助手消息内容
	if assistantMsg != nil {
		_, err = h.db.Exec(
			"UPDATE messages SET content = ?, mcp_execution_ids = ? WHERE id = ?",
			result.Response,
			func() string {
				if len(result.MCPExecutionIDs) > 0 {
					jsonData, _ := json.Marshal(result.MCPExecutionIDs)
					return string(jsonData)
				}
				return ""
			}(),
			assistantMessageID,
		)
		if err != nil {
			h.logger.Error("更新助手消息失败", zap.Error(err))
		}
	} else {
		// 如果之前创建失败，现在创建
		_, err = h.db.AddMessage(conversationID, "assistant", result.Response, result.MCPExecutionIDs)
		if err != nil {
			h.logger.Error("保存助手消息失败", zap.Error(err))
		}
	}

	// 保存最后一轮ReAct的输入和输出
	if result.LastReActInput != "" || result.LastReActOutput != "" {
		if err := h.db.SaveReActData(conversationID, result.LastReActInput, result.LastReActOutput); err != nil {
			h.logger.Warn("保存ReAct数据失败", zap.Error(err))
		} else {
			h.logger.Info("已保存ReAct数据", zap.String("conversationId", conversationID))
		}
	}

	// 发送最终响应
	sendEvent("response", result.Response, map[string]interface{}{
		"mcpExecutionIds": result.MCPExecutionIDs,
		"conversationId":  conversationID,
		"messageId":       assistantMessageID, // 包含消息ID，以便前端关联过程详情
	})
	sendEvent("done", "", map[string]interface{}{
		"conversationId": conversationID,
	})
}

// CancelAgentLoop 取消正在执行的任务
func (h *AgentHandler) CancelAgentLoop(c *gin.Context) {
	var req struct {
		ConversationID string `json:"conversationId" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ok, err := h.tasks.CancelTask(req.ConversationID, ErrTaskCancelled)
	if err != nil {
		h.logger.Error("取消任务失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到正在执行的任务"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":         "cancelling",
		"conversationId": req.ConversationID,
		"message":        "已提交取消请求，任务将在当前步骤完成后停止。",
	})
}

// ListAgentTasks 列出所有运行中的任务
func (h *AgentHandler) ListAgentTasks(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"tasks": h.tasks.GetActiveTasks(),
	})
}

// ListCompletedTasks 列出最近完成的任务历史
func (h *AgentHandler) ListCompletedTasks(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"tasks": h.tasks.GetCompletedTasks(),
	})
}

// BatchTaskRequest 批量任务请求
type BatchTaskRequest struct {
	Title string   `json:"title"`                    // 任务标题（可选）
	Tasks []string `json:"tasks" binding:"required"` // 任务列表，每行一个任务
	Role  string   `json:"role,omitempty"`           // 角色名称（可选，空字符串表示默认角色）
}

// CreateBatchQueue 创建批量任务队列
func (h *AgentHandler) CreateBatchQueue(c *gin.Context) {
	var req BatchTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(req.Tasks) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "任务列表不能为空"})
		return
	}

	// 过滤空任务
	validTasks := make([]string, 0, len(req.Tasks))
	for _, task := range req.Tasks {
		if task != "" {
			validTasks = append(validTasks, task)
		}
	}

	if len(validTasks) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "没有有效的任务"})
		return
	}

	queue := h.batchTaskManager.CreateBatchQueue(req.Title, req.Role, validTasks)
	c.JSON(http.StatusOK, gin.H{
		"queueId": queue.ID,
		"queue":   queue,
	})
}

// GetBatchQueue 获取批量任务队列
func (h *AgentHandler) GetBatchQueue(c *gin.Context) {
	queueID := c.Param("queueId")
	queue, exists := h.batchTaskManager.GetBatchQueue(queueID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "队列不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"queue": queue})
}

// ListBatchQueuesResponse 批量任务队列列表响应
type ListBatchQueuesResponse struct {
	Queues     []*BatchTaskQueue `json:"queues"`
	Total      int               `json:"total"`
	Page       int               `json:"page"`
	PageSize   int               `json:"page_size"`
	TotalPages int               `json:"total_pages"`
}

// ListBatchQueues 列出所有批量任务队列（支持筛选和分页）
func (h *AgentHandler) ListBatchQueues(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "10")
	offsetStr := c.DefaultQuery("offset", "0")
	pageStr := c.Query("page")
	status := c.Query("status")
	keyword := c.Query("keyword")

	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)
	page := 1

	// 如果提供了page参数，优先使用page计算offset
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
			offset = (page - 1) * limit
		}
	}

	// 限制pageSize范围
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	// 默认status为"all"
	if status == "" {
		status = "all"
	}

	// 获取队列列表和总数
	queues, total, err := h.batchTaskManager.ListQueues(limit, offset, status, keyword)
	if err != nil {
		h.logger.Error("获取批量任务队列列表失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 计算总页数
	totalPages := (total + limit - 1) / limit
	if totalPages == 0 {
		totalPages = 1
	}

	// 如果使用offset计算page，需要重新计算
	if pageStr == "" {
		page = (offset / limit) + 1
	}

	response := ListBatchQueuesResponse{
		Queues:     queues,
		Total:      total,
		Page:       page,
		PageSize:   limit,
		TotalPages: totalPages,
	}

	c.JSON(http.StatusOK, response)
}

// StartBatchQueue 开始执行批量任务队列
func (h *AgentHandler) StartBatchQueue(c *gin.Context) {
	queueID := c.Param("queueId")
	queue, exists := h.batchTaskManager.GetBatchQueue(queueID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "队列不存在"})
		return
	}

	if queue.Status != "pending" && queue.Status != "paused" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "队列状态不允许启动"})
		return
	}

	// 在后台执行批量任务
	go h.executeBatchQueue(queueID)

	h.batchTaskManager.UpdateQueueStatus(queueID, "running")
	c.JSON(http.StatusOK, gin.H{"message": "批量任务已开始执行", "queueId": queueID})
}

// PauseBatchQueue 暂停批量任务队列
func (h *AgentHandler) PauseBatchQueue(c *gin.Context) {
	queueID := c.Param("queueId")
	success := h.batchTaskManager.PauseQueue(queueID)
	if !success {
		c.JSON(http.StatusNotFound, gin.H{"error": "队列不存在或无法暂停"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "批量任务已暂停"})
}

// DeleteBatchQueue 删除批量任务队列
func (h *AgentHandler) DeleteBatchQueue(c *gin.Context) {
	queueID := c.Param("queueId")
	success := h.batchTaskManager.DeleteQueue(queueID)
	if !success {
		c.JSON(http.StatusNotFound, gin.H{"error": "队列不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "批量任务队列已删除"})
}

// UpdateBatchTask 更新批量任务消息
func (h *AgentHandler) UpdateBatchTask(c *gin.Context) {
	queueID := c.Param("queueId")
	taskID := c.Param("taskId")

	var req struct {
		Message string `json:"message" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数: " + err.Error()})
		return
	}

	if req.Message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "任务消息不能为空"})
		return
	}

	err := h.batchTaskManager.UpdateTaskMessage(queueID, taskID, req.Message)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 返回更新后的队列信息
	queue, exists := h.batchTaskManager.GetBatchQueue(queueID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "队列不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "任务已更新", "queue": queue})
}

// AddBatchTask 添加任务到批量任务队列
func (h *AgentHandler) AddBatchTask(c *gin.Context) {
	queueID := c.Param("queueId")

	var req struct {
		Message string `json:"message" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数: " + err.Error()})
		return
	}

	if req.Message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "任务消息不能为空"})
		return
	}

	task, err := h.batchTaskManager.AddTaskToQueue(queueID, req.Message)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 返回更新后的队列信息
	queue, exists := h.batchTaskManager.GetBatchQueue(queueID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "队列不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "任务已添加", "task": task, "queue": queue})
}

// DeleteBatchTask 删除批量任务
func (h *AgentHandler) DeleteBatchTask(c *gin.Context) {
	queueID := c.Param("queueId")
	taskID := c.Param("taskId")

	err := h.batchTaskManager.DeleteTask(queueID, taskID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 返回更新后的队列信息
	queue, exists := h.batchTaskManager.GetBatchQueue(queueID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "队列不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "任务已删除", "queue": queue})
}

// executeBatchQueue 执行批量任务队列
func (h *AgentHandler) executeBatchQueue(queueID string) {
	h.logger.Info("开始执行批量任务队列", zap.String("queueId", queueID))

	for {
		// 检查队列状态
		queue, exists := h.batchTaskManager.GetBatchQueue(queueID)
		if !exists || queue.Status == "cancelled" || queue.Status == "completed" || queue.Status == "paused" {
			break
		}

		// 获取下一个任务
		task, hasNext := h.batchTaskManager.GetNextTask(queueID)
		if !hasNext {
			// 所有任务完成
			h.batchTaskManager.UpdateQueueStatus(queueID, "completed")
			h.logger.Info("批量任务队列执行完成", zap.String("queueId", queueID))
			break
		}

		// 更新任务状态为运行中
		h.batchTaskManager.UpdateTaskStatus(queueID, task.ID, "running", "", "")

		// 创建新对话
		title := safeTruncateString(task.Message, 50)
		conv, err := h.db.CreateConversation(title)
		var conversationID string
		if err != nil {
			h.logger.Error("创建对话失败", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(err))
			h.batchTaskManager.UpdateTaskStatus(queueID, task.ID, "failed", "", "创建对话失败: "+err.Error())
			h.batchTaskManager.MoveToNextTask(queueID)
			continue
		}
		conversationID = conv.ID

		// 保存conversationId到任务中（即使是运行中状态也要保存，以便查看对话）
		h.batchTaskManager.UpdateTaskStatusWithConversationID(queueID, task.ID, "running", "", "", conversationID)

		// 应用角色用户提示词和工具配置
		finalMessage := task.Message
		var roleTools []string  // 角色配置的工具列表
		var roleSkills []string // 角色配置的skills列表（用于提示AI，但不硬编码内容）
		if queue.Role != "" && queue.Role != "默认" {
			if h.config.Roles != nil {
				if role, exists := h.config.Roles[queue.Role]; exists && role.Enabled {
					// 应用用户提示词
					if role.UserPrompt != "" {
						finalMessage = role.UserPrompt + "\n\n" + task.Message
						h.logger.Info("应用角色用户提示词", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("role", queue.Role))
					}
					// 获取角色配置的工具列表（优先使用tools字段，向后兼容mcps字段）
					if len(role.Tools) > 0 {
						roleTools = role.Tools
						h.logger.Info("使用角色配置的工具列表", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("role", queue.Role), zap.Int("toolCount", len(roleTools)))
					}
					// 获取角色配置的skills列表（用于在系统提示词中提示AI，但不硬编码内容）
					if len(role.Skills) > 0 {
						roleSkills = role.Skills
						h.logger.Info("角色配置了skills，将在系统提示词中提示AI", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("role", queue.Role), zap.Int("skillCount", len(roleSkills)), zap.Strings("skills", roleSkills))
					}
				}
			}
		}

		// 保存用户消息（保存原始消息，不包含角色提示词）
		_, err = h.db.AddMessage(conversationID, "user", task.Message, nil)
		if err != nil {
			h.logger.Error("保存用户消息失败", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("conversationId", conversationID), zap.Error(err))
		}

		// 预先创建助手消息，以便关联过程详情
		assistantMsg, err := h.db.AddMessage(conversationID, "assistant", "处理中...", nil)
		if err != nil {
			h.logger.Error("创建助手消息失败", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("conversationId", conversationID), zap.Error(err))
			// 如果创建失败，继续执行但不保存过程详情
			assistantMsg = nil
		}

		// 创建进度回调函数，复用统一逻辑（批量任务不需要流式事件，所以传入nil）
		var assistantMessageID string
		if assistantMsg != nil {
			assistantMessageID = assistantMsg.ID
		}
		progressCallback := h.createProgressCallback(conversationID, assistantMessageID, nil)

		// 执行任务（使用包含角色提示词的finalMessage和角色工具列表）
		h.logger.Info("执行批量任务", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("message", task.Message), zap.String("role", queue.Role), zap.String("conversationId", conversationID))

		// 单个子任务超时时间：从30分钟调整为6小时，适配长时间渗透/扫描任务
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Hour)
		// 存储取消函数，以便在取消队列时能够取消当前任务
		h.batchTaskManager.SetTaskCancel(queueID, cancel)
		// 使用队列配置的角色工具列表（如果为空，表示使用所有工具）
		// 注意：skills不会硬编码注入，但会在系统提示词中提示AI这个角色推荐使用哪些skills
		useBatchMulti := h.config != nil && h.config.MultiAgent.Enabled && h.config.MultiAgent.BatchUseMultiAgent
		var result *agent.AgentLoopResult
		var resultMA *multiagent.RunResult
		var runErr error
		if useBatchMulti {
			resultMA, runErr = multiagent.RunDeepAgent(ctx, h.config, &h.config.MultiAgent, h.agent, h.logger, conversationID, finalMessage, []agent.ChatMessage{}, roleTools, progressCallback, h.agentsMarkdownDir)
		} else {
			result, runErr = h.agent.AgentLoopWithProgress(ctx, finalMessage, []agent.ChatMessage{}, conversationID, progressCallback, roleTools, roleSkills)
		}
		// 任务执行完成，清理取消函数
		h.batchTaskManager.SetTaskCancel(queueID, nil)
		cancel()

		if runErr != nil {
			// 检查是否是取消错误
			// 1. 直接检查是否是 context.Canceled（包括包装后的错误）
			// 2. 检查错误消息中是否包含"context canceled"或"cancelled"关键字
			// 3. 检查 result.Response 中是否包含取消相关的消息
			errStr := runErr.Error()
			partialResp := ""
			if result != nil {
				partialResp = result.Response
			} else if resultMA != nil {
				partialResp = resultMA.Response
			}
			isCancelled := errors.Is(runErr, context.Canceled) ||
				strings.Contains(strings.ToLower(errStr), "context canceled") ||
				strings.Contains(strings.ToLower(errStr), "context cancelled") ||
				(partialResp != "" && (strings.Contains(partialResp, "任务已被取消") || strings.Contains(partialResp, "任务执行中断")))

			if isCancelled {
				h.logger.Info("批量任务被取消", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("conversationId", conversationID))
				cancelMsg := "任务已被用户取消，后续操作已停止。"
				// 如果执行结果中有更具体的取消消息，使用它
				if partialResp != "" && (strings.Contains(partialResp, "任务已被取消") || strings.Contains(partialResp, "任务执行中断")) {
					cancelMsg = partialResp
				}
				// 更新助手消息内容
				if assistantMessageID != "" {
					if _, updateErr := h.db.Exec(
						"UPDATE messages SET content = ? WHERE id = ?",
						cancelMsg,
						assistantMessageID,
					); updateErr != nil {
						h.logger.Warn("更新取消后的助手消息失败", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(updateErr))
					}
					// 保存取消详情到数据库
					if err := h.db.AddProcessDetail(assistantMessageID, conversationID, "cancelled", cancelMsg, nil); err != nil {
						h.logger.Warn("保存取消详情失败", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(err))
					}
				} else {
					// 如果没有预先创建的助手消息，创建一个新的
					_, errMsg := h.db.AddMessage(conversationID, "assistant", cancelMsg, nil)
					if errMsg != nil {
						h.logger.Warn("保存取消消息失败", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(errMsg))
					}
				}
				// 保存ReAct数据（如果存在）
				if result != nil && (result.LastReActInput != "" || result.LastReActOutput != "") {
					if err := h.db.SaveReActData(conversationID, result.LastReActInput, result.LastReActOutput); err != nil {
						h.logger.Warn("保存取消任务的ReAct数据失败", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(err))
					}
				} else if resultMA != nil && (resultMA.LastReActInput != "" || resultMA.LastReActOutput != "") {
					if err := h.db.SaveReActData(conversationID, resultMA.LastReActInput, resultMA.LastReActOutput); err != nil {
						h.logger.Warn("保存取消任务的ReAct数据失败", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(err))
					}
				}
				h.batchTaskManager.UpdateTaskStatusWithConversationID(queueID, task.ID, "cancelled", cancelMsg, "", conversationID)
			} else {
				h.logger.Error("批量任务执行失败", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("conversationId", conversationID), zap.Error(runErr))
				errorMsg := "执行失败: " + runErr.Error()
				// 更新助手消息内容
				if assistantMessageID != "" {
					if _, updateErr := h.db.Exec(
						"UPDATE messages SET content = ? WHERE id = ?",
						errorMsg,
						assistantMessageID,
					); updateErr != nil {
						h.logger.Warn("更新失败后的助手消息失败", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(updateErr))
					}
					// 保存错误详情到数据库
					if err := h.db.AddProcessDetail(assistantMessageID, conversationID, "error", errorMsg, nil); err != nil {
						h.logger.Warn("保存错误详情失败", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(err))
					}
				}
				h.batchTaskManager.UpdateTaskStatus(queueID, task.ID, "failed", "", runErr.Error())
			}
		} else {
			h.logger.Info("批量任务执行成功", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("conversationId", conversationID))

			var resText string
			var mcpIDs []string
			var lastIn, lastOut string
			if useBatchMulti {
				resText = resultMA.Response
				mcpIDs = resultMA.MCPExecutionIDs
				lastIn = resultMA.LastReActInput
				lastOut = resultMA.LastReActOutput
			} else {
				resText = result.Response
				mcpIDs = result.MCPExecutionIDs
				lastIn = result.LastReActInput
				lastOut = result.LastReActOutput
			}

			// 更新助手消息内容
			if assistantMessageID != "" {
				mcpIDsJSON := ""
				if len(mcpIDs) > 0 {
					jsonData, _ := json.Marshal(mcpIDs)
					mcpIDsJSON = string(jsonData)
				}
				if _, updateErr := h.db.Exec(
					"UPDATE messages SET content = ?, mcp_execution_ids = ? WHERE id = ?",
					resText,
					mcpIDsJSON,
					assistantMessageID,
				); updateErr != nil {
					h.logger.Warn("更新助手消息失败", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(updateErr))
					// 如果更新失败，尝试创建新消息
					_, err = h.db.AddMessage(conversationID, "assistant", resText, mcpIDs)
					if err != nil {
						h.logger.Error("保存助手消息失败", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("conversationId", conversationID), zap.Error(err))
					}
				}
			} else {
				// 如果没有预先创建的助手消息，创建一个新的
				_, err = h.db.AddMessage(conversationID, "assistant", resText, mcpIDs)
				if err != nil {
					h.logger.Error("保存助手消息失败", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("conversationId", conversationID), zap.Error(err))
				}
			}

			// 保存ReAct数据
			if lastIn != "" || lastOut != "" {
				if err := h.db.SaveReActData(conversationID, lastIn, lastOut); err != nil {
					h.logger.Warn("保存ReAct数据失败", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(err))
				} else {
					h.logger.Info("已保存ReAct数据", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("conversationId", conversationID))
				}
			}

			// 保存结果
			h.batchTaskManager.UpdateTaskStatusWithConversationID(queueID, task.ID, "completed", resText, "", conversationID)
		}

		// 移动到下一个任务
		h.batchTaskManager.MoveToNextTask(queueID)

		// 检查是否被取消或暂停
		queue, _ = h.batchTaskManager.GetBatchQueue(queueID)
		if queue.Status == "cancelled" || queue.Status == "paused" {
			break
		}
	}
}

// loadHistoryFromReActData 从保存的ReAct数据恢复历史消息上下文
// 采用与攻击链生成类似的拼接逻辑：优先使用保存的last_react_input和last_react_output，若不存在则回退到消息表
func (h *AgentHandler) loadHistoryFromReActData(conversationID string) ([]agent.ChatMessage, error) {
	// 获取保存的ReAct输入和输出
	reactInputJSON, reactOutput, err := h.db.GetReActData(conversationID)
	if err != nil {
		return nil, fmt.Errorf("获取ReAct数据失败: %w", err)
	}

	// 如果last_react_input为空，回退到使用消息表（与攻击链生成逻辑一致）
	if reactInputJSON == "" {
		return nil, fmt.Errorf("ReAct数据为空，将使用消息表")
	}

	dataSource := "database_last_react_input"

	// 解析JSON格式的messages数组
	var messagesArray []map[string]interface{}
	if err := json.Unmarshal([]byte(reactInputJSON), &messagesArray); err != nil {
		return nil, fmt.Errorf("解析ReAct输入JSON失败: %w", err)
	}

	messageCount := len(messagesArray)

	h.logger.Info("使用保存的ReAct数据恢复历史上下文",
		zap.String("conversationId", conversationID),
		zap.String("dataSource", dataSource),
		zap.Int("reactInputSize", len(reactInputJSON)),
		zap.Int("messageCount", messageCount),
		zap.Int("reactOutputSize", len(reactOutput)),
	)
	// fmt.Println("messagesArray:", messagesArray)//debug

	// 转换为Agent消息格式
	agentMessages := make([]agent.ChatMessage, 0, len(messagesArray))
	for _, msgMap := range messagesArray {
		msg := agent.ChatMessage{}

		// 解析role
		if role, ok := msgMap["role"].(string); ok {
			msg.Role = role
		} else {
			continue // 跳过无效消息
		}

		// 跳过system消息（AgentLoop会重新添加）
		if msg.Role == "system" {
			continue
		}

		// 解析content
		if content, ok := msgMap["content"].(string); ok {
			msg.Content = content
		}

		// 解析tool_calls（如果存在）
		if toolCallsRaw, ok := msgMap["tool_calls"]; ok && toolCallsRaw != nil {
			if toolCallsArray, ok := toolCallsRaw.([]interface{}); ok {
				msg.ToolCalls = make([]agent.ToolCall, 0, len(toolCallsArray))
				for _, tcRaw := range toolCallsArray {
					if tcMap, ok := tcRaw.(map[string]interface{}); ok {
						toolCall := agent.ToolCall{}

						// 解析ID
						if id, ok := tcMap["id"].(string); ok {
							toolCall.ID = id
						}

						// 解析Type
						if toolType, ok := tcMap["type"].(string); ok {
							toolCall.Type = toolType
						}

						// 解析Function
						if funcMap, ok := tcMap["function"].(map[string]interface{}); ok {
							toolCall.Function = agent.FunctionCall{}

							// 解析函数名
							if name, ok := funcMap["name"].(string); ok {
								toolCall.Function.Name = name
							}

							// 解析arguments（可能是字符串或对象）
							if argsRaw, ok := funcMap["arguments"]; ok {
								if argsStr, ok := argsRaw.(string); ok {
									// 如果是字符串，解析为JSON
									var argsMap map[string]interface{}
									if err := json.Unmarshal([]byte(argsStr), &argsMap); err == nil {
										toolCall.Function.Arguments = argsMap
									}
								} else if argsMap, ok := argsRaw.(map[string]interface{}); ok {
									// 如果已经是对象，直接使用
									toolCall.Function.Arguments = argsMap
								}
							}
						}

						if toolCall.ID != "" {
							msg.ToolCalls = append(msg.ToolCalls, toolCall)
						}
					}
				}
			}
		}

		// 解析tool_call_id（tool角色消息）
		if toolCallID, ok := msgMap["tool_call_id"].(string); ok {
			msg.ToolCallID = toolCallID
		}

		agentMessages = append(agentMessages, msg)
	}

	// 如果存在last_react_output，需要将其作为最后一条assistant消息
	// 因为last_react_input是在迭代开始前保存的，不包含最后一轮的最终输出
	if reactOutput != "" {
		// 检查最后一条消息是否是assistant消息且没有tool_calls
		// 如果有tool_calls，说明后面应该还有tool消息和最终的assistant回复
		if len(agentMessages) > 0 {
			lastMsg := &agentMessages[len(agentMessages)-1]
			if strings.EqualFold(lastMsg.Role, "assistant") && len(lastMsg.ToolCalls) == 0 {
				// 最后一条是assistant消息且没有tool_calls，用最终输出更新其content
				lastMsg.Content = reactOutput
			} else {
				// 最后一条不是assistant消息，或者有tool_calls，添加最终输出作为新的assistant消息
				agentMessages = append(agentMessages, agent.ChatMessage{
					Role:    "assistant",
					Content: reactOutput,
				})
			}
		} else {
			// 如果没有消息，直接添加最终输出
			agentMessages = append(agentMessages, agent.ChatMessage{
				Role:    "assistant",
				Content: reactOutput,
			})
		}
	}

	if len(agentMessages) == 0 {
		return nil, fmt.Errorf("从ReAct数据解析的消息为空")
	}

	// 修复可能存在的失配tool消息，避免OpenAI报错
	// 这可以防止出现"messages with role 'tool' must be a response to a preceeding message with 'tool_calls'"错误
	if h.agent != nil {
		if fixed := h.agent.RepairOrphanToolMessages(&agentMessages); fixed {
			h.logger.Info("修复了从ReAct数据恢复的历史消息中的失配tool消息",
				zap.String("conversationId", conversationID),
			)
		}
	}

	h.logger.Info("从ReAct数据恢复历史消息完成",
		zap.String("conversationId", conversationID),
		zap.String("dataSource", dataSource),
		zap.Int("originalMessageCount", messageCount),
		zap.Int("finalMessageCount", len(agentMessages)),
		zap.Bool("hasReactOutput", reactOutput != ""),
	)
	fmt.Println("agentMessages:", agentMessages) //debug
	return agentMessages, nil
}
