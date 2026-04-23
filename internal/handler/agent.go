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
	"cyberstrike-ai/internal/debug"
	"cyberstrike-ai/internal/mcp/builtin"
	"cyberstrike-ai/internal/multiagent"
	"cyberstrike-ai/internal/skills"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// safeTruncateString safely truncates string, avoids cutting in middle of UTF-8 characters
func safeTruncateString(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}

	// convert string to rune slice for correct character counting
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}

	// truncate to max length
	truncated := string(runes[:maxLen])

	// try to truncate at punctuation or spaces for more natural breaks
	// search backward from truncation point for suitable break (no more than 20% of length)
	searchRange := maxLen / 5
	if searchRange > maxLen {
		searchRange = maxLen
	}
	breakChars := []rune(",., ,.;:!?!?/\\-_")
	bestBreakPos := len(runes[:maxLen])

	for i := bestBreakPos - 1; i >= bestBreakPos-searchRange && i >= 0; i-- {
		for _, breakChar := range breakChars {
			if runes[i] == breakChar {
				bestBreakPos = i + 1 // break after punctuation
				goto found
			}
		}
	}

found:
	truncated = string(runes[:bestBreakPos])
	return truncated + "..."
}

// AgentHandler Agent handler
type AgentHandler struct {
	agent            *agent.Agent
	db               *database.DB
	logger           *zap.Logger
	tasks            *AgentTaskManager
	batchTaskManager *BatchTaskManager
	config           *config.Config // config reference for getting role info
	knowledgeManager interface {    // knowledge base manager interface
		LogRetrieval(conversationID, messageID, query, riskType string, retrievedItems []string) error
	}
	skillsManager     *skills.Manager // Skills manager
	agentsMarkdownDir string          // multi-agent: Markdown sub-agent directory (absolute path, empty means no disk merge)
	debugSink         debug.Sink
}

// NewAgentHandler creates a new Agent handler
func NewAgentHandler(agent *agent.Agent, db *database.DB, cfg *config.Config, logger *zap.Logger, sink debug.Sink) *AgentHandler {
	if sink == nil {
		sink = debug.NewSink(false, nil, logger)
	}

	batchTaskManager := NewBatchTaskManager()
	batchTaskManager.SetDB(db)

	// load all batch task queues from database
	if err := batchTaskManager.LoadFromDB(); err != nil {
		logger.Warn("failed to load batch task queues from database", zap.Error(err))
	}

	return &AgentHandler{
		agent:            agent,
		db:               db,
		logger:           logger,
		tasks:            NewAgentTaskManager(),
		batchTaskManager: batchTaskManager,
		config:           cfg,
		debugSink:        sink,
	}
}

// SetKnowledgeManager sets knowledge base manager (for recording retrieval logs)
func (h *AgentHandler) SetKnowledgeManager(manager interface {
	LogRetrieval(conversationID, messageID, query, riskType string, retrievedItems []string) error
}) {
	h.knowledgeManager = manager
}

// SetSkillsManager sets Skills manager
func (h *AgentHandler) SetSkillsManager(manager *skills.Manager) {
	h.skillsManager = manager
}

// SetAgentsMarkdownDir sets agents/*.md sub-agent directory (absolute path); empty means only use sub_agents from config.yaml.
func (h *AgentHandler) SetAgentsMarkdownDir(absDir string) {
	h.agentsMarkdownDir = strings.TrimSpace(absDir)
}

// ChatAttachment chat attachment (user uploaded file)
type ChatAttachment struct {
	FileName   string `json:"fileName"`          // display file name
	Content    string `json:"content,omitempty"` // text or base64; can be empty if pre-uploaded to server
	MimeType   string `json:"mimeType,omitempty"`
	ServerPath string `json:"serverPath,omitempty"` // absolute path saved under chat_uploads (returned by POST /api/chat-uploads)
}

// ChatRequest chat request
type ChatRequest struct {
	Message              string           `json:"message" binding:"required"`
	ConversationID       string           `json:"conversationId,omitempty"`
	Role                 string           `json:"role,omitempty"` // role name
	Attachments          []ChatAttachment `json:"attachments,omitempty"`
	WebShellConnectionID string           `json:"webshellConnectionId,omitempty"` // WebShell management - AI assistant: selected connection ID, only uses webshell_* tools
}

const (
	maxAttachments     = 10
	chatUploadsDirName = "chat_uploads" // root directory for chat attachments (relative to working directory)
)

// validateChatAttachmentServerPath validates absolute path is under working directory chat_uploads and is a regular file (prevents path traversal)
func validateChatAttachmentServerPath(abs string) (string, error) {
	p := strings.TrimSpace(abs)
	if p == "" {
		return "", fmt.Errorf("empty path")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
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

// avoidChatUploadDestCollision if path exists, generates new filename with timestamp+random suffix
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

// relocateManualOrNewUploadToConversation when no session ID, frontend uploads to ...//_manual;message, ...//{conversationId}/ conversation.
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
	// only handles flat structure: date/_manual|_new/filename
	if len(segs) != 3 {
		return absPath, nil
	}
	datePart, placeFolder, baseName := segs[0], segs[1], segs[2]
	if placeFolder != "_manual" && placeFolder != "_new" {
		return absPath, nil
	}
	targetDir := filepath.Join(rootAbs, datePart, convSan)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create conversation attachment directory: %w", err)
	}
	dest := filepath.Join(targetDir, baseName)
	dest = avoidChatUploadDestCollision(dest)
	if err := os.Rename(absPath, dest); err != nil {
		return "", fmt.Errorf("failed to move attachment to conversation directory: %w", err)
	}
	out, _ := filepath.Abs(dest)
	if logger != nil {
		logger.Info("attachment moved from placeholder to conversation directory",
			zap.String("from", absPath),
			zap.String("to", out),
			zap.String("conversationId", conv))
	}
	return out, nil
}

// saveAttachmentsToDateAndConversationDir processes attachments: if serverPath set, only validates existing file; otherwise writes content to chat_uploads/YYYY-MM-DD/{conversationID}/.
// when conversationID is empty, uses "_new" as directory name (new conversation has no ID yet)
func saveAttachmentsToDateAndConversationDir(attachments []ChatAttachment, conversationID string, logger *zap.Logger) (savedPaths []string, err error) {
	if len(attachments) == 0 {
		return nil, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
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
		return nil, fmt.Errorf("failed to create upload directory: %w", err)
	}
	savedPaths = make([]string, 0, len(attachments))
	for i, a := range attachments {
		if sp := strings.TrimSpace(a.ServerPath); sp != "" {
			valid, verr := validateChatAttachmentServerPath(sp)
			if verr != nil {
				return nil, fmt.Errorf(" %s: %w", a.FileName, verr)
			}
			finalPath, rerr := relocateManualOrNewUploadToConversation(valid, conversationID, logger)
			if rerr != nil {
				return nil, fmt.Errorf(" %s: %w", a.FileName, rerr)
			}
			savedPaths = append(savedPaths, finalPath)
			if logger != nil {
				logger.Debug("conversation attachment using pre-uploaded path", zap.Int("index", i+1), zap.String("fileName", a.FileName), zap.String("path", finalPath))
			}
			continue
		}
		if strings.TrimSpace(a.Content) == "" {
			return nil, fmt.Errorf("attachment %s missing content or serverPath not provided", a.FileName)
		}
		raw, decErr := attachmentContentToBytes(a)
		if decErr != nil {
			return nil, fmt.Errorf("attachment %s decode failed: %w", a.FileName, decErr)
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
			return nil, fmt.Errorf("failed to write file %s: %w", a.FileName, err)
		}
		absPath, _ := filepath.Abs(fullPath)
		savedPaths = append(savedPaths, absPath)
		if logger != nil {
			logger.Debug("conversation attachment saved", zap.Int("index", i+1), zap.String("fileName", a.FileName), zap.String("path", absPath))
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

// userMessageContentForStorage returns user message content for database storage:(),,continueconversation
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

// appendAttachmentsToMessage only appends attachment save paths to end of user message,,
func appendAttachmentsToMessage(msg string, attachments []ChatAttachment, savedPaths []string) string {
	if len(attachments) == 0 {
		return msg
	}
	var b strings.Builder
	b.WriteString(msg)
	b.WriteString("\n\n[User uploaded files saved to the following paths (read file content as needed)]\n")
	for i, a := range attachments {
		if i < len(savedPaths) && savedPaths[i] != "" {
			b.WriteString(fmt.Sprintf("- %s: %s\n", a.FileName, savedPaths[i]))
		} else {
			b.WriteString(fmt.Sprintf("- %s: (path unknown, save may have failed)\n", a.FileName))
		}
	}
	return b.String()
}

// ChatResponse chat response
type ChatResponse struct {
	Response        string    `json:"response"`
	MCPExecutionIDs []string  `json:"mcpExecutionIds,omitempty"` // list of MCP execution IDs
	ConversationID  string    `json:"conversationId"`            // conversation ID
	Time            time.Time `json:"time"`
}

// AgentLoop handles Agent Loop requests
func (h *AgentHandler) AgentLoop(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("received Agent Loop request",
		zap.String("message", req.Message),
		zap.String("conversationId", req.ConversationID),
	)

	// if no conversation ID, create new conversation
	conversationID := req.ConversationID
	if conversationID == "" {
		title := safeTruncateString(req.Message, 50)
		conv, err := h.db.CreateConversation(title)
		if err != nil {
			h.logger.Error("failed to create conversation", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		conversationID = conv.ID
	} else {
		// verify conversation exists
		_, err := h.db.GetConversation(conversationID)
		if err != nil {
			h.logger.Error("conversation not found", zap.String("conversationId", conversationID), zap.Error(err))
			c.JSON(http.StatusNotFound, gin.H{"error": "conversation not found"})
			return
		}
	}

	// try to restore history context from saved ReAct data first
	agentHistoryMessages, err := h.loadHistoryFromReActData(conversationID)
	if err != nil {
		h.logger.Warn("failed to load history from ReAct data, using message table", zap.Error(err))
		// fall back to database message table
		historyMessages, err := h.db.GetMessages(conversationID)
		if err != nil {
			h.logger.Warn("failed to get history messages", zap.Error(err))
			agentHistoryMessages = []agent.ChatMessage{}
		} else {
			// convert database messages to Agent message format
			agentHistoryMessages = make([]agent.ChatMessage, 0, len(historyMessages))
			for _, msg := range historyMessages {
				agentHistoryMessages = append(agentHistoryMessages, agent.ChatMessage{
					Role:    msg.Role,
					Content: msg.Content,
				})
			}
			h.logger.Info("loaded history messages from message table", zap.Int("count", len(agentHistoryMessages)))
		}
	} else {
		h.logger.Info("restored history context from ReAct data", zap.Int("count", len(agentHistoryMessages)))
	}

	// validate attachment count (non-streaming)
	if len(req.Attachments) > maxAttachments {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("maximum %d attachments", maxAttachments)})
		return
	}

	// apply role user prompt and tool configuration
	finalMessage := req.Message
	var roleTools []string  // role-configured tool list
	var roleSkills []string // role-configured skills list(AI,)

	// WebShell AI-assistant mode: scope the agent to a specific webshell connection.
	// The system prompt below tells the model to pass connection_id on every webshell_*
	// tool call, restricts the visible toolset to webshell + knowledge/skills lookups,
	// and clarifies the workflow: invoke tools to act on the target, then record findings
	// back to the knowledge base or a skill pack.
	if req.WebShellConnectionID != "" {
		conn, err := h.db.GetWebshellConnection(strings.TrimSpace(req.WebShellConnectionID))
		if err != nil || conn == nil {
			h.logger.Warn("WebShell AI assistant: connection not found", zap.String("id", req.WebShellConnectionID), zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "WebShell connection not found"})
			return
		}
		remark := conn.Remark
		if remark == "" {
			remark = conn.URL
		}
		finalMessage = fmt.Sprintf(
			"[WebShell session] Active connection ID: %s (remark: %s).\n"+
				"Always pass connection_id=%q as the first argument on every webshell_* tool call.\n"+
				"Available tools: webshell_exec, webshell_file_list, webshell_file_read, webshell_file_write, "+
				"record_vulnerability, list_knowledge_risk_types, search_knowledge_base, list_skills, read_skill.\n"+
				"Workflow: use the webshell_* tools to act on the target; use record_vulnerability / list_knowledge_risk_types / "+
				"search_knowledge_base to store and retrieve findings; use list_skills / read_skill to discover and apply reusable "+
				"procedures from the knowledge base and skill packs.\n\n"+
				"User request: %s",
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
	} else if req.Role != "" && req.Role != "default" {
		if h.config.Roles != nil {
			if role, exists := h.config.Roles[req.Role]; exists && role.Enabled {
				// apply user prompt
				if role.UserPrompt != "" {
					finalMessage = role.UserPrompt + "\n\n" + req.Message
					h.logger.Info("applied role user prompt", zap.String("role", req.Role))
				}
				// get role-configured tool list(tools,mcps)
				if len(role.Tools) > 0 {
					roleTools = role.Tools
					h.logger.Info("using role-configured tool list", zap.String("role", req.Role), zap.Int("toolCount", len(roleTools)))
				}
				// get role-configured skills list(AI,)
				if len(role.Skills) > 0 {
					roleSkills = role.Skills
					h.logger.Info("role has skills configured, will hint AI in system prompt", zap.String("role", req.Role), zap.Int("skillCount", len(roleSkills)), zap.Strings("skills", roleSkills))
				}
			}
		}
	}
	var savedPaths []string
	if len(req.Attachments) > 0 {
		savedPaths, err = saveAttachmentsToDateAndConversationDir(req.Attachments, conversationID, h.logger)
		if err != nil {
			h.logger.Error("failed to save conversation attachments", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save uploaded file: " + err.Error()})
			return
		}
	}
	finalMessage = appendAttachmentsToMessage(finalMessage, req.Attachments, savedPaths)

	// save user message:,,continueconversation
	userContent := userMessageContentForStorage(req.Message, req.Attachments, savedPaths)
	_, err = h.db.AddMessage(conversationID, "user", userContent, nil)
	if err != nil {
		h.logger.Error("failed to save user message", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save user message: " + err.Error()})
		return
	}

	// execute Agent Loop,messageconversationID(rolefinalMessagerolelist)
	// :skills,AIroleskills
	result, err := h.agent.AgentLoopWithProgress(c.Request.Context(), finalMessage, agentHistoryMessages, conversationID, nil, roleTools, roleSkills)
	if err != nil {
		h.logger.Error("Agent Loop execution failed", zap.Error(err))

		// execution failed,ReAct(result)
		if result != nil && (result.LastReActInput != "" || result.LastReActOutput != "") {
			if saveErr := h.db.SaveReActData(conversationID, result.LastReActInput, result.LastReActOutput); saveErr != nil {
				h.logger.Warn("failed to save ReAct data for failed task", zap.Error(saveErr))
			} else {
				h.logger.Info("saved ReAct data for failed task", zap.String("conversationId", conversationID))
			}
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// save assistant reply
	_, err = h.db.AddMessage(conversationID, "assistant", result.Response, result.MCPExecutionIDs)
	if err != nil {
		h.logger.Error("failed to save assistant message", zap.Error(err))
		// ,returns,recorderror
		// AI,
	}

	// ReAct
	if result.LastReActInput != "" || result.LastReActOutput != "" {
		if err := h.db.SaveReActData(conversationID, result.LastReActInput, result.LastReActOutput); err != nil {
			h.logger.Warn("failed to save ReAct data", zap.Error(err))
		} else {
			h.logger.Info("ReAct data saved", zap.String("conversationId", conversationID))
		}
	}

	c.JSON(http.StatusOK, ChatResponse{
		Response:        result.Response,
		MCPExecutionIDs: result.MCPExecutionIDs,
		ConversationID:  conversationID,
		Time:            time.Now(),
	})
}

// ProcessMessageForRobot processes a message from a robot (Telegram). Similar to /api/agent-loop/stream but returns a string reply.
func (h *AgentHandler) ProcessMessageForRobot(ctx context.Context, conversationID, message, role string) (response string, convID string, err error) {
	if conversationID == "" {
		title := safeTruncateString(message, 50)
		conv, createErr := h.db.CreateConversation(title)
		if createErr != nil {
			return "", "", fmt.Errorf("conversation: %w", createErr)
		}
		conversationID = conv.ID
	} else {
		if _, getErr := h.db.GetConversation(conversationID); getErr != nil {
			return "", "", fmt.Errorf("conversation not found")
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
	if role != "" && role != "default" && h.config.Roles != nil {
		if r, exists := h.config.Roles[role]; exists && r.Enabled {
			if r.UserPrompt != "" {
				finalMessage = r.UserPrompt + "\n\n" + message
			}
			roleTools = r.Tools
			roleSkills = r.Skills
		}
	}

	if _, err = h.db.AddMessage(conversationID, "user", message, nil); err != nil {
		return "", "", fmt.Errorf("message: %w", err)
	}

	// agent-loop/stream :message, progressCallback process details( SSE)
	assistantMsg, err := h.db.AddMessage(conversationID, "assistant", "processing...", nil)
	if err != nil {
		h.logger.Warn("robot: failed to create assistant message placeholder", zap.Error(err))
	}
	var assistantMessageID string
	if assistantMsg != nil {
		assistantMessageID = assistantMsg.ID
	}
	progressCallback := h.createProgressCallback(conversationID, assistantMessageID, nil)

	useRobotMulti := h.config != nil && h.config.MultiAgent.Enabled && h.config.MultiAgent.RobotUseMultiAgent
	if useRobotMulti {
		resultMA, errMA := multiagent.RunOrchestrator(
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
			h.debugSink,
		)
		if errMA != nil {
			errMsg := "execution failed: " + errMA.Error()
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
				h.logger.Warn("robot: failed to update assistant message", zap.Error(err))
			}
		} else {
			if _, err = h.db.AddMessage(conversationID, "assistant", resultMA.Response, resultMA.MCPExecutionIDs); err != nil {
				h.logger.Warn("robot: failed to save assistant message", zap.Error(err))
			}
		}
		if resultMA.LastReActInput != "" || resultMA.LastReActOutput != "" {
			_ = h.db.SaveReActData(conversationID, resultMA.LastReActInput, resultMA.LastReActOutput)
		}
		return resultMA.Response, conversationID, nil
	}

	result, err := h.agent.AgentLoopWithProgress(ctx, finalMessage, agentHistoryMessages, conversationID, progressCallback, roleTools, roleSkills)
	if err != nil {
		errMsg := "execution failed: " + err.Error()
		if assistantMessageID != "" {
			_, _ = h.db.Exec("UPDATE messages SET content = ? WHERE id = ?", errMsg, assistantMessageID)
			_ = h.db.AddProcessDetail(assistantMessageID, conversationID, "error", errMsg, nil)
		}
		return "", conversationID, err
	}

	// message MCP ID( stream )
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
			h.logger.Warn("robot: failed to update assistant message", zap.Error(err))
		}
	} else {
		if _, err = h.db.AddMessage(conversationID, "assistant", result.Response, result.MCPExecutionIDs); err != nil {
			h.logger.Warn("robot: failed to save assistant message", zap.Error(err))
		}
	}
	if result.LastReActInput != "" || result.LastReActOutput != "" {
		_ = h.db.SaveReActData(conversationID, result.LastReActInput, result.LastReActOutput)
	}
	return result.Response, conversationID, nil
}

// StreamEvent
type StreamEvent struct {
	Type    string      `json:"type"`    // conversation, progress, tool_call, tool_result, response, error, cancelled, done
	Message string      `json:"message"` // message
	Data    interface{} `json:"data,omitempty"`
}

// createProgressCallback ,processDetails
// sendEventFunc: ,nil
func (h *AgentHandler) createProgressCallback(conversationID, assistantMessageID string, sendEventFunc func(eventType, message string, data interface{})) agent.ProgressCallback {
	// tool_call,tool_result
	toolCallCache := make(map[string]map[string]interface{}) // toolCallId -> arguments

	return func(eventType, message string, data interface{}) {
		// sendEventFunc,
		if sendEventFunc != nil {
			sendEventFunc(eventType, message, data)
		}

		// tool_call
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

		// retrieval logrecord
		if eventType == "tool_result" && h.knowledgeManager != nil {
			if dataMap, ok := data.(map[string]interface{}); ok {
				toolName, _ := dataMap["toolName"].(string)
				if toolName == builtin.ToolSearchKnowledgeBase {
					//
					query := ""
					riskType := ""
					var retrievedItems []string

					// tool_call
					if toolCallId, ok := dataMap["toolCallId"].(string); ok && toolCallId != "" {
						if cachedArgs, exists := toolCallCache[toolCallId]; exists {
							if q, ok := cachedArgs["query"].(string); ok && q != "" {
								query = q
							}
							if rt, ok := cachedArgs["risk_type"].(string); ok && rt != "" {
								riskType = rt
							}
							//
							delete(toolCallCache, toolCallId)
						}
					}

					// ,argumentsObj
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

					// query,result()
					if query == "" {
						if result, ok := dataMap["result"].(string); ok && result != "" {
							// (" 'xxx' ")
							if strings.Contains(result, " '") {
								start := strings.Index(result, " '") + len(" '")
								end := strings.Index(result[start:], "'")
								if end > 0 {
									query = result[start : start+end]
								}
							}
						}
						// ,default value
						if query == "" {
							query = "unknown query"
						}
					}

					// ID
					// format:" X :\n\n--- 1 (: XX.XX%) ---\n: [] title\n...\n<!-- METADATA: {...} -->"
					if result, ok := dataMap["result"].(string); ok && result != "" {
						// ID
						metadataMatch := strings.Index(result, "<!-- METADATA:")
						if metadataMatch > 0 {
							// JSON
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

						// ," X ",
						if len(retrievedItems) == 0 && strings.Contains(result, "") && !strings.Contains(result, "") {
							// ,ID,
							retrievedItems = []string{"_has_results"}
						}
					}

					// recordretrieval log(,)
					go func() {
						if err := h.knowledgeManager.LogRetrieval(conversationID, assistantMessageID, query, riskType, retrievedItems); err != nil {
							h.logger.Warn("failed to record knowledge retrieval log", zap.Error(err))
						}
					}()

					// addprocessDetails
					if assistantMessageID != "" {
						retrievalData := map[string]interface{}{
							"query":    query,
							"riskType": riskType,
							"toolName": toolName,
						}
						if err := h.db.AddProcessDetail(assistantMessageID, conversationID, "knowledge_retrieval", fmt.Sprintf("knowledge retrieval: %s", query), retrievalData); err != nil {
							h.logger.Warn("failed to save knowledge retrieval details", zap.Error(err))
						}
					}
				}
			}
		}

		// Persist the sub-agent's final reply (stream_end carries the full body).
		if assistantMessageID != "" && eventType == "subagent_reply_stream_end" {
			if err := h.db.AddProcessDetail(assistantMessageID, conversationID, "subagent_reply", message, data); err != nil {
				h.logger.Warn("failed to save process details", zap.Error(err), zap.String("eventType", eventType))
			}
			return
		}

		// Persist every other event as a process_detail, except those that are
		// either the final response itself (response/done/response_{start,delta})
		// or streaming fragments that would flood process_details.
		if assistantMessageID != "" &&
			eventType != "response" &&
			eventType != "done" &&
			eventType != "response_start" &&
			eventType != "response_delta" &&
			eventType != "tool_result_delta" &&
			eventType != "thinking_stream_start" &&
			eventType != "thinking_stream_delta" &&
			eventType != "subagent_reply_stream_start" &&
			eventType != "subagent_reply_stream_delta" &&
			eventType != "subagent_reply_stream_end" {
			if err := h.db.AddProcessDetail(assistantMessageID, conversationID, eventType, message, data); err != nil {
				h.logger.Warn("failed to save process details", zap.Error(err), zap.String("eventType", eventType))
			}
		}
	}
}

// AgentLoopStream Agent Loop
func (h *AgentHandler) AgentLoopStream(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// ,SSEformaterror
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		event := StreamEvent{
			Type:    "error",
			Message: "request parameter error: " + err.Error(),
		}
		eventJSON, _ := json.Marshal(event)
		fmt.Fprintf(c.Writer, "data: %s\n\n", eventJSON)
		c.Writer.Flush()
		return
	}

	h.logger.Info("received Agent Loop streaming request",
		zap.String("message", req.Message),
		zap.String("conversationId", req.ConversationID),
	)

	// SSE
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no") // nginx

	//
	//
	clientDisconnected := false
	// shared with sseKeepalive: ResponseWriter, chunked (ERR_INVALID_CHUNKED_ENCODING).
	var sseWriteMu sync.Mutex
	// delta
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
			// ,
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

		// ,
		if clientDisconnected {
			return
		}

		// ()
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
			h.logger.Debug("client disconnected, stopped sending SSE events", zap.Error(err))
			return
		}
		if flusher, ok := c.Writer.(http.Flusher); ok {
			flusher.Flush()
		} else {
			c.Writer.Flush()
		}
		sseWriteMu.Unlock()
	}

	// if no conversation ID, create new conversation(WebShell connection ID )
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
			h.logger.Error("failed to create conversation", zap.Error(err))
			sendEvent("error", "conversation: "+err.Error(), nil)
			return
		}
		conversationID = conv.ID
		sendEvent("conversation", "session created", map[string]interface{}{
			"conversationId": conversationID,
		})
	} else {
		// verify conversation exists
		_, err := h.db.GetConversation(conversationID)
		if err != nil {
			h.logger.Error("conversation not found", zap.String("conversationId", conversationID), zap.Error(err))
			sendEvent("error", "conversation not found", nil)
			return
		}
	}

	// try to restore history context from saved ReAct data first
	agentHistoryMessages, err := h.loadHistoryFromReActData(conversationID)
	if err != nil {
		h.logger.Warn("failed to load history from ReAct data, using message table", zap.Error(err))
		// fall back to database message table
		historyMessages, err := h.db.GetMessages(conversationID)
		if err != nil {
			h.logger.Warn("failed to get history messages", zap.Error(err))
			agentHistoryMessages = []agent.ChatMessage{}
		} else {
			// convert database messages to Agent message format
			agentHistoryMessages = make([]agent.ChatMessage, 0, len(historyMessages))
			for _, msg := range historyMessages {
				agentHistoryMessages = append(agentHistoryMessages, agent.ChatMessage{
					Role:    msg.Role,
					Content: msg.Content,
				})
			}
			h.logger.Info("loaded history messages from message table", zap.Int("count", len(agentHistoryMessages)))
		}
	} else {
		h.logger.Info("restored history context from ReAct data", zap.Int("count", len(agentHistoryMessages)))
	}

	// validate attachment count
	if len(req.Attachments) > maxAttachments {
		sendEvent("error", fmt.Sprintf("maximum %d attachments", maxAttachments), nil)
		return
	}

	// apply role user prompt and tool configuration
	finalMessage := req.Message
	var roleTools []string // role-configured tool list
	var roleSkills []string
	if req.WebShellConnectionID != "" {
		conn, errConn := h.db.GetWebshellConnection(strings.TrimSpace(req.WebShellConnectionID))
		if errConn != nil || conn == nil {
			h.logger.Warn("WebShell AI assistant: connection not found", zap.String("id", req.WebShellConnectionID), zap.Error(errConn))
			sendEvent("error", "WebShell connection not found", nil)
			return
		}
		remark := conn.Remark
		if remark == "" {
			remark = conn.URL
		}
		finalMessage = fmt.Sprintf(
			"[WebShell session] Active connection ID: %s (remark: %s).\n"+
				"Always pass connection_id=%q as the first argument on every webshell_* tool call.\n"+
				"Available tools: webshell_exec, webshell_file_list, webshell_file_read, webshell_file_write, "+
				"record_vulnerability, list_knowledge_risk_types, search_knowledge_base, list_skills, read_skill.\n"+
				"Workflow: use the webshell_* tools to act on the target; use record_vulnerability / list_knowledge_risk_types / "+
				"search_knowledge_base to store and retrieve findings; use list_skills / read_skill to discover and apply reusable "+
				"procedures from the knowledge base and skill packs.\n\n"+
				"User request: %s",
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
	} else if req.Role != "" && req.Role != "default" {
		if h.config.Roles != nil {
			if role, exists := h.config.Roles[req.Role]; exists && role.Enabled {
				// apply user prompt
				if role.UserPrompt != "" {
					finalMessage = role.UserPrompt + "\n\n" + req.Message
					h.logger.Info("applied role user prompt", zap.String("role", req.Role))
				}
				// get role-configured tool list(tools,mcps)
				if len(role.Tools) > 0 {
					roleTools = role.Tools
					h.logger.Info("using role-configured tool list", zap.String("role", req.Role), zap.Int("toolCount", len(roleTools)))
				} else if len(role.MCPs) > 0 {
					// :mcps,list()
					// mcpsMCP,list
					h.logger.Info("rolemcps,", zap.String("role", req.Role))
				}
				// :roleskills,AIlist_skillsread_skill
				if len(role.Skills) > 0 {
					roleSkills = role.Skills
					h.logger.Info("role has skills, AI can call via tools on demand", zap.String("role", req.Role), zap.Int("skillCount", len(role.Skills)), zap.Strings("skills", role.Skills))
				}
			}
		}
	}
	var savedPaths []string
	if len(req.Attachments) > 0 {
		savedPaths, err = saveAttachmentsToDateAndConversationDir(req.Attachments, conversationID, h.logger)
		if err != nil {
			h.logger.Error("failed to save conversation attachments", zap.Error(err))
			sendEvent("error", "failed to save uploaded file: "+err.Error(), nil)
			return
		}
	}
	// finalMessage,file content
	finalMessage = appendAttachmentsToMessage(finalMessage, req.Attachments, savedPaths)
	// roleTools,(defaultrolerole)

	// save user message:,,continueconversation
	userContent := userMessageContentForStorage(req.Message, req.Attachments, savedPaths)
	_, err = h.db.AddMessage(conversationID, "user", userContent, nil)
	if err != nil {
		h.logger.Error("failed to save user message", zap.Error(err))
	}

	// message,process details
	assistantMsg, err := h.db.AddMessage(conversationID, "assistant", "processing...", nil)
	if err != nil {
		h.logger.Error("failed to create assistant message", zap.Error(err))
		// ,continueprocess details
		assistantMsg = nil
	}

	// ,
	var assistantMessageID string
	if assistantMsg != nil {
		assistantMessageID = assistantMsg.ID
	}

	// ,
	progressCallback := h.createProgressCallback(conversationID, assistantMessageID, sendEvent)

	// ,HTTP
	// (),continue
	baseCtx, cancelWithCause := context.WithCancelCause(context.Background())
	taskCtx, timeoutCancel := context.WithTimeout(baseCtx, 600*time.Minute)
	defer timeoutCancel()
	defer cancelWithCause(nil)

	if _, err := h.tasks.StartTask(conversationID, req.Message, cancelWithCause); err != nil {
		var errorMsg string
		if errors.Is(err, ErrTaskAlreadyRunning) {
			errorMsg = "⚠️ This session already has a running task. Say \"stop\" to cancel it."
			sendEvent("error", errorMsg, map[string]interface{}{
				"conversationId": conversationID,
				"errorType":      "task_already_running",
			})
		} else {
			errorMsg = "❌ : " + err.Error()
			sendEvent("error", errorMsg, map[string]interface{}{
				"conversationId": conversationID,
				"errorType":      "task_start_failed",
			})
		}

		// messageerror
		if assistantMessageID != "" {
			if _, updateErr := h.db.Exec(
				"UPDATE messages SET content = ? WHERE id = ?",
				errorMsg,
				assistantMessageID,
			); updateErr != nil {
				h.logger.Warn("failed to update message after error", zap.Error(updateErr))
			}
			// error
			if err := h.db.AddProcessDetail(assistantMessageID, conversationID, "error", errorMsg, map[string]interface{}{
				"errorType": func() string {
					if errors.Is(err, ErrTaskAlreadyRunning) {
						return "task_already_running"
					}
					return "task_start_failed"
				}(),
			}); err != nil {
				h.logger.Warn("failed to save error details", zap.Error(err))
			}
		}

		sendEvent("done", "", map[string]interface{}{
			"conversationId": conversationID,
		})
		return
	}

	taskStatus := "completed"
	defer h.tasks.FinishTask(conversationID, taskStatus)

	// execute Agent Loop,,(rolefinalMessagerolelist)
	sendEvent("progress", "analyzing your request...", nil)
	// :roleSkills req.Role WebShell
	stopKeepalive := make(chan struct{})
	go sseKeepalive(c, stopKeepalive, &sseWriteMu)
	defer close(stopKeepalive)

	result, err := h.agent.AgentLoopWithProgress(taskCtx, finalMessage, agentHistoryMessages, conversationID, progressCallback, roleTools, roleSkills)
	if err != nil {
		h.logger.Error("Agent Loop execution failed", zap.Error(err))
		cause := context.Cause(baseCtx)

		// :contextcauseErrTaskCancelled
		// causeErrTaskCancelled,errortype(context.Canceled),
		// API
		isCancelled := errors.Is(cause, ErrTaskCancelled)

		switch {
		case isCancelled:
			taskStatus = "cancelled"
			cancelMsg := "Task cancelled by user, subsequent operations stopped."

			// status,status
			h.tasks.UpdateTaskStatus(conversationID, taskStatus)

			if assistantMessageID != "" {
				if _, updateErr := h.db.Exec(
					"UPDATE messages SET content = ? WHERE id = ?",
					cancelMsg,
					assistantMessageID,
				); updateErr != nil {
					h.logger.Warn("failed to update message after cancellation", zap.Error(updateErr))
				}
				h.db.AddProcessDetail(assistantMessageID, conversationID, "cancelled", cancelMsg, nil)
			}

			// ,ReAct(result)
			if result != nil && (result.LastReActInput != "" || result.LastReActOutput != "") {
				if err := h.db.SaveReActData(conversationID, result.LastReActInput, result.LastReActOutput); err != nil {
					h.logger.Warn("failed to save ReAct data for cancelled task", zap.Error(err))
				} else {
					h.logger.Info("saved ReAct data for cancelled task", zap.String("conversationId", conversationID))
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
			timeoutMsg := "Task execution timed out, auto-terminated."

			// status,status
			h.tasks.UpdateTaskStatus(conversationID, taskStatus)

			if assistantMessageID != "" {
				if _, updateErr := h.db.Exec(
					"UPDATE messages SET content = ? WHERE id = ?",
					timeoutMsg,
					assistantMessageID,
				); updateErr != nil {
					h.logger.Warn("failed to update message after timeout", zap.Error(updateErr))
				}
				h.db.AddProcessDetail(assistantMessageID, conversationID, "timeout", timeoutMsg, nil)
			}

			// ,ReAct(result)
			if result != nil && (result.LastReActInput != "" || result.LastReActOutput != "") {
				if err := h.db.SaveReActData(conversationID, result.LastReActInput, result.LastReActOutput); err != nil {
					h.logger.Warn("failed to save ReAct for timed-out task", zap.Error(err))
				} else {
					h.logger.Info("saved ReAct data for timed-out task", zap.String("conversationId", conversationID))
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
			errorMsg := "execution failed: " + err.Error()

			// status,status
			h.tasks.UpdateTaskStatus(conversationID, taskStatus)

			if assistantMessageID != "" {
				if _, updateErr := h.db.Exec(
					"UPDATE messages SET content = ? WHERE id = ?",
					errorMsg,
					assistantMessageID,
				); updateErr != nil {
					h.logger.Warn("failed to update message after failure", zap.Error(updateErr))
				}
				h.db.AddProcessDetail(assistantMessageID, conversationID, "error", errorMsg, nil)
			}

			// ,ReAct(result)
			if result != nil && (result.LastReActInput != "" || result.LastReActOutput != "") {
				if err := h.db.SaveReActData(conversationID, result.LastReActInput, result.LastReActOutput); err != nil {
					h.logger.Warn("failed to save ReAct data for failed task", zap.Error(err))
				} else {
					h.logger.Info("saved ReAct data for failed task", zap.String("conversationId", conversationID))
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

	// message
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
			h.logger.Error("failed to update assistant message", zap.Error(err))
		}
	} else {
		// ,
		_, err = h.db.AddMessage(conversationID, "assistant", result.Response, result.MCPExecutionIDs)
		if err != nil {
			h.logger.Error("failed to save assistant message", zap.Error(err))
		}
	}

	// ReAct
	if result.LastReActInput != "" || result.LastReActOutput != "" {
		if err := h.db.SaveReActData(conversationID, result.LastReActInput, result.LastReActOutput); err != nil {
			h.logger.Warn("failed to save ReAct data", zap.Error(err))
		} else {
			h.logger.Info("ReAct data saved", zap.String("conversationId", conversationID))
		}
	}

	//
	sendEvent("response", result.Response, map[string]interface{}{
		"mcpExecutionIds": result.MCPExecutionIDs,
		"conversationId":  conversationID,
		"messageId":       assistantMessageID, // messageID,process details
	})
	sendEvent("done", "", map[string]interface{}{
		"conversationId": conversationID,
	})
}

// CancelAgentLoop
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
		h.logger.Error("failed to cancel task", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "no running task found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":         "cancelling",
		"conversationId": req.ConversationID,
		"message":        ",currentstop.",
	})
}

// ListAgentTasks
func (h *AgentHandler) ListAgentTasks(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"tasks": h.tasks.GetActiveTasks(),
	})
}

// ListCompletedTasks recently completed task history
func (h *AgentHandler) ListCompletedTasks(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"tasks": h.tasks.GetCompletedTasks(),
	})
}

// BatchTaskRequest batch tasks
type BatchTaskRequest struct {
	Title string   `json:"title"`                    // title()
	Tasks []string `json:"tasks" binding:"required"` // list,
	Role  string   `json:"role,omitempty"`           // role(,defaultrole)
}

// CreateBatchQueue batch tasks
func (h *AgentHandler) CreateBatchQueue(c *gin.Context) {
	var req BatchTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(req.Tasks) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "list"})
		return
	}

	//
	validTasks := make([]string, 0, len(req.Tasks))
	for _, task := range req.Tasks {
		if task != "" {
			validTasks = append(validTasks, task)
		}
	}

	if len(validTasks) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": ""})
		return
	}

	queue := h.batchTaskManager.CreateBatchQueue(req.Title, req.Role, validTasks)
	c.JSON(http.StatusOK, gin.H{
		"queueId": queue.ID,
		"queue":   queue,
	})
}

// GetBatchQueue batch tasks
func (h *AgentHandler) GetBatchQueue(c *gin.Context) {
	queueID := c.Param("queueId")
	queue, exists := h.batchTaskManager.GetBatchQueue(queueID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "queue not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"queue": queue})
}

// ListBatchQueuesResponse batch taskslist
type ListBatchQueuesResponse struct {
	Queues     []*BatchTaskQueue `json:"queues"`
	Total      int               `json:"total"`
	Page       int               `json:"page"`
	PageSize   int               `json:"page_size"`
	TotalPages int               `json:"total_pages"`
}

// ListBatchQueues batch tasks()
func (h *AgentHandler) ListBatchQueues(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "10")
	offsetStr := c.DefaultQuery("offset", "0")
	pageStr := c.Query("page")
	status := c.Query("status")
	keyword := c.Query("keyword")

	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)
	page := 1

	// page,pageoffset
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
			offset = (page - 1) * limit
		}
	}

	// pageSize
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	// defaultstatus"all"
	if status == "" {
		status = "all"
	}

	// list
	queues, total, err := h.batchTaskManager.ListQueues(limit, offset, status, keyword)
	if err != nil {
		h.logger.Error("batch taskstable failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	//
	totalPages := (total + limit - 1) / limit
	if totalPages == 0 {
		totalPages = 1
	}

	// offsetpage,
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

// StartBatchQueue batch tasks
func (h *AgentHandler) StartBatchQueue(c *gin.Context) {
	queueID := c.Param("queueId")
	queue, exists := h.batchTaskManager.GetBatchQueue(queueID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "queue not found"})
		return
	}

	if queue.Status != "pending" && queue.Status != "paused" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status"})
		return
	}

	// batch tasks
	go h.executeBatchQueue(queueID)

	h.batchTaskManager.UpdateQueueStatus(queueID, "running")
	c.JSON(http.StatusOK, gin.H{"message": "batch tasks", "queueId": queueID})
}

// PauseBatchQueue batch tasks
func (h *AgentHandler) PauseBatchQueue(c *gin.Context) {
	queueID := c.Param("queueId")
	success := h.batchTaskManager.PauseQueue(queueID)
	if !success {
		c.JSON(http.StatusNotFound, gin.H{"error": "queue not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "batch tasks"})
}

// DeleteBatchQueue deletebatch tasks
func (h *AgentHandler) DeleteBatchQueue(c *gin.Context) {
	queueID := c.Param("queueId")
	success := h.batchTaskManager.DeleteQueue(queueID)
	if !success {
		c.JSON(http.StatusNotFound, gin.H{"error": "queue not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "batch tasksdelete"})
}

// UpdateBatchTask batch tasksmessage
func (h *AgentHandler) UpdateBatchTask(c *gin.Context) {
	queueID := c.Param("queueId")
	taskID := c.Param("taskId")

	var req struct {
		Message string `json:"message" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request parameters: " + err.Error()})
		return
	}

	if req.Message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "message"})
		return
	}

	err := h.batchTaskManager.UpdateTaskMessage(queueID, taskID, req.Message)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// returns
	queue, exists := h.batchTaskManager.GetBatchQueue(queueID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "queue not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "", "queue": queue})
}

// AddBatchTask addbatch tasks
func (h *AgentHandler) AddBatchTask(c *gin.Context) {
	queueID := c.Param("queueId")

	var req struct {
		Message string `json:"message" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request parameters: " + err.Error()})
		return
	}

	if req.Message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "message"})
		return
	}

	task, err := h.batchTaskManager.AddTaskToQueue(queueID, req.Message)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// returns
	queue, exists := h.batchTaskManager.GetBatchQueue(queueID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "queue not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "add", "task": task, "queue": queue})
}

// DeleteBatchTask deletebatch tasks
func (h *AgentHandler) DeleteBatchTask(c *gin.Context) {
	queueID := c.Param("queueId")
	taskID := c.Param("taskId")

	err := h.batchTaskManager.DeleteTask(queueID, taskID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// returns
	queue, exists := h.batchTaskManager.GetBatchQueue(queueID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "queue not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "delete", "queue": queue})
}

// executeBatchQueue batch tasks
func (h *AgentHandler) executeBatchQueue(queueID string) {
	h.logger.Info("batch tasks", zap.String("queueId", queueID))

	for {
		// status
		queue, exists := h.batchTaskManager.GetBatchQueue(queueID)
		if !exists || queue.Status == "cancelled" || queue.Status == "completed" || queue.Status == "paused" {
			break
		}

		//
		task, hasNext := h.batchTaskManager.GetNextTask(queueID)
		if !hasNext {
			//
			h.batchTaskManager.UpdateQueueStatus(queueID, "completed")
			h.logger.Info("batch tasks", zap.String("queueId", queueID))
			break
		}

		// status
		h.batchTaskManager.UpdateTaskStatus(queueID, task.ID, "running", "", "")

		// conversation
		title := safeTruncateString(task.Message, 50)
		conv, err := h.db.CreateConversation(title)
		var conversationID string
		if err != nil {
			h.logger.Error("failed to create conversation", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(err))
			h.batchTaskManager.UpdateTaskStatus(queueID, task.ID, "failed", "", "conversation: "+err.Error())
			h.batchTaskManager.MoveToNextTask(queueID)
			continue
		}
		conversationID = conv.ID

		// conversationId(status,conversation)
		h.batchTaskManager.UpdateTaskStatusWithConversationID(queueID, task.ID, "running", "", "", conversationID)

		// apply role user prompt and tool configuration
		finalMessage := task.Message
		var roleTools []string  // role-configured tool list
		var roleSkills []string // role-configured skills list(AI,)
		if queue.Role != "" && queue.Role != "default" {
			if h.config.Roles != nil {
				if role, exists := h.config.Roles[queue.Role]; exists && role.Enabled {
					// apply user prompt
					if role.UserPrompt != "" {
						finalMessage = role.UserPrompt + "\n\n" + task.Message
						h.logger.Info("applied role user prompt", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("role", queue.Role))
					}
					// get role-configured tool list(tools,mcps)
					if len(role.Tools) > 0 {
						roleTools = role.Tools
						h.logger.Info("using role-configured tool list", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("role", queue.Role), zap.Int("toolCount", len(roleTools)))
					}
					// get role-configured skills list(AI,)
					if len(role.Skills) > 0 {
						roleSkills = role.Skills
						h.logger.Info("role has skills configured, will hint AI in system prompt", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("role", queue.Role), zap.Int("skillCount", len(roleSkills)), zap.Strings("skills", roleSkills))
					}
				}
			}
		}

		// save user message(message,role)
		_, err = h.db.AddMessage(conversationID, "user", task.Message, nil)
		if err != nil {
			h.logger.Error("failed to save user message", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("conversationId", conversationID), zap.Error(err))
		}

		// message,process details
		assistantMsg, err := h.db.AddMessage(conversationID, "assistant", "processing...", nil)
		if err != nil {
			h.logger.Error("failed to create assistant message", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("conversationId", conversationID), zap.Error(err))
			// ,continueprocess details
			assistantMsg = nil
		}

		// ,(batch tasks,nil)
		var assistantMessageID string
		if assistantMsg != nil {
			assistantMessageID = assistantMsg.ID
		}
		progressCallback := h.createProgressCallback(conversationID, assistantMessageID, nil)

		// (rolefinalMessagerolelist)
		h.logger.Info("batch tasks", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("message", task.Message), zap.String("role", queue.Role), zap.String("conversationId", conversationID))

		// :306,/scan
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Hour)
		// ,current
		h.batchTaskManager.SetTaskCancel(queueID, cancel)
		// rolelist(,)
		// :skills,AIroleskills
		useBatchMulti := h.config != nil && h.config.MultiAgent.Enabled && h.config.MultiAgent.BatchUseMultiAgent
		var result *agent.AgentLoopResult
		var resultMA *multiagent.RunResult
		var runErr error
		if useBatchMulti {
			resultMA, runErr = multiagent.RunOrchestrator(ctx, h.config, &h.config.MultiAgent, h.agent, h.logger, conversationID, finalMessage, []agent.ChatMessage{}, roleTools, progressCallback, h.agentsMarkdownDir, h.debugSink)
		} else {
			result, runErr = h.agent.AgentLoopWithProgress(ctx, finalMessage, []agent.ChatMessage{}, conversationID, progressCallback, roleTools, roleSkills)
		}
		// ,
		h.batchTaskManager.SetTaskCancel(queueID, nil)
		cancel()

		if runErr != nil {
			// error
			// 1. context.Canceled(error)
			// 2. errormessage"context canceled""cancelled"
			// 3. result.Response message
			errStr := runErr.Error()
			partialResp := ""
			if result != nil {
				partialResp = result.Response
			} else if resultMA != nil {
				partialResp = resultMA.Response
			}
			partialLower := strings.ToLower(partialResp)
			partialCancelled := partialResp != "" && (strings.Contains(partialLower, "task was cancelled") ||
				strings.Contains(partialLower, "task has been cancelled") ||
				strings.Contains(partialLower, "execution was interrupted") ||
				strings.Contains(partialLower, "execution interrupted"))
			isCancelled := errors.Is(runErr, context.Canceled) ||
				strings.Contains(strings.ToLower(errStr), "context canceled") ||
				strings.Contains(strings.ToLower(errStr), "context cancelled") ||
				partialCancelled

			if isCancelled {
				h.logger.Info("batch tasks", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("conversationId", conversationID))
				cancelMsg := "Task cancelled by user, subsequent operations stopped."
				// If the partial response already contains a more specific cancel message from the
				// model, prefer it verbatim over the generic fallback.
				if partialCancelled {
					cancelMsg = partialResp
				}
				// message
				if assistantMessageID != "" {
					if _, updateErr := h.db.Exec(
						"UPDATE messages SET content = ? WHERE id = ?",
						cancelMsg,
						assistantMessageID,
					); updateErr != nil {
						h.logger.Warn("failed to update message after cancellation", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(updateErr))
					}
					//
					if err := h.db.AddProcessDetail(assistantMessageID, conversationID, "cancelled", cancelMsg, nil); err != nil {
						h.logger.Warn("", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(err))
					}
				} else {
					// message,
					_, errMsg := h.db.AddMessage(conversationID, "assistant", cancelMsg, nil)
					if errMsg != nil {
						h.logger.Warn("message", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(errMsg))
					}
				}
				// ReAct()
				if result != nil && (result.LastReActInput != "" || result.LastReActOutput != "") {
					if err := h.db.SaveReActData(conversationID, result.LastReActInput, result.LastReActOutput); err != nil {
						h.logger.Warn("failed to save ReAct data for cancelled task", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(err))
					}
				} else if resultMA != nil && (resultMA.LastReActInput != "" || resultMA.LastReActOutput != "") {
					if err := h.db.SaveReActData(conversationID, resultMA.LastReActInput, resultMA.LastReActOutput); err != nil {
						h.logger.Warn("failed to save ReAct data for cancelled task", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(err))
					}
				}
				h.batchTaskManager.UpdateTaskStatusWithConversationID(queueID, task.ID, "cancelled", cancelMsg, "", conversationID)
			} else {
				h.logger.Error("batch tasksexecution failed", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("conversationId", conversationID), zap.Error(runErr))
				errorMsg := "execution failed: " + runErr.Error()
				// message
				if assistantMessageID != "" {
					if _, updateErr := h.db.Exec(
						"UPDATE messages SET content = ? WHERE id = ?",
						errorMsg,
						assistantMessageID,
					); updateErr != nil {
						h.logger.Warn("failed to update message after failure", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(updateErr))
					}
					// error
					if err := h.db.AddProcessDetail(assistantMessageID, conversationID, "error", errorMsg, nil); err != nil {
						h.logger.Warn("failed to save error details", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(err))
					}
				}
				h.batchTaskManager.UpdateTaskStatus(queueID, task.ID, "failed", "", runErr.Error())
			}
		} else {
			h.logger.Info("batch tasks", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("conversationId", conversationID))

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

			// message
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
					h.logger.Warn("failed to update assistant message", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(updateErr))
					// ,message
					_, err = h.db.AddMessage(conversationID, "assistant", resText, mcpIDs)
					if err != nil {
						h.logger.Error("failed to save assistant message", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("conversationId", conversationID), zap.Error(err))
					}
				}
			} else {
				// message,
				_, err = h.db.AddMessage(conversationID, "assistant", resText, mcpIDs)
				if err != nil {
					h.logger.Error("failed to save assistant message", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("conversationId", conversationID), zap.Error(err))
				}
			}

			// ReAct
			if lastIn != "" || lastOut != "" {
				if err := h.db.SaveReActData(conversationID, lastIn, lastOut); err != nil {
					h.logger.Warn("failed to save ReAct data", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(err))
				} else {
					h.logger.Info("ReAct data saved", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("conversationId", conversationID))
				}
			}

			//
			h.batchTaskManager.UpdateTaskStatusWithConversationID(queueID, task.ID, "completed", resText, "", conversationID)
		}

		//
		h.batchTaskManager.MoveToNextTask(queueID)

		//
		queue, _ = h.batchTaskManager.GetBatchQueue(queueID)
		if queue.Status == "cancelled" || queue.Status == "paused" {
			break
		}
	}
}

// loadHistoryFromReActData ReActmessage
// attack chain:last_react_inputlast_react_output,message
func (h *AgentHandler) loadHistoryFromReActData(conversationID string) ([]agent.ChatMessage, error) {
	// ReAct
	reactInputJSON, reactOutput, err := h.db.GetReActData(conversationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get ReAct data: %w", err)
	}

	// last_react_input,message(attack chain)
	if reactInputJSON == "" {
		return nil, fmt.Errorf("ReAct,message")
	}

	dataSource := "database_last_react_input"

	// parseJSONformatmessages
	var messagesArray []map[string]interface{}
	if err := json.Unmarshal([]byte(reactInputJSON), &messagesArray); err != nil {
		return nil, fmt.Errorf("parseReActJSON: %w", err)
	}

	messageCount := len(messagesArray)

	h.logger.Info("ReAct",
		zap.String("conversationId", conversationID),
		zap.String("dataSource", dataSource),
		zap.Int("reactInputSize", len(reactInputJSON)),
		zap.Int("messageCount", messageCount),
		zap.Int("reactOutputSize", len(reactOutput)),
	)
	// fmt.Println("messagesArray:", messagesArray)//debug

	// Agentmessageformat
	agentMessages := make([]agent.ChatMessage, 0, len(messagesArray))
	for _, msgMap := range messagesArray {
		msg := agent.ChatMessage{}

		// parserole
		if role, ok := msgMap["role"].(string); ok {
			msg.Role = role
		} else {
			continue // skipmessage
		}

		// skipsystemmessage(AgentLoopadd)
		if msg.Role == "system" {
			continue
		}

		// parsecontent
		if content, ok := msgMap["content"].(string); ok {
			msg.Content = content
		}

		// parsetool_calls()
		if toolCallsRaw, ok := msgMap["tool_calls"]; ok && toolCallsRaw != nil {
			if toolCallsArray, ok := toolCallsRaw.([]interface{}); ok {
				msg.ToolCalls = make([]agent.ToolCall, 0, len(toolCallsArray))
				for _, tcRaw := range toolCallsArray {
					if tcMap, ok := tcRaw.(map[string]interface{}); ok {
						toolCall := agent.ToolCall{}

						// parseID
						if id, ok := tcMap["id"].(string); ok {
							toolCall.ID = id
						}

						// parseType
						if toolType, ok := tcMap["type"].(string); ok {
							toolCall.Type = toolType
						}

						// parseFunction
						if funcMap, ok := tcMap["function"].(map[string]interface{}); ok {
							toolCall.Function = agent.FunctionCall{}

							// parse
							if name, ok := funcMap["name"].(string); ok {
								toolCall.Function.Name = name
							}

							// parsearguments()
							if argsRaw, ok := funcMap["arguments"]; ok {
								if argsStr, ok := argsRaw.(string); ok {
									// ,parseJSON
									var argsMap map[string]interface{}
									if err := json.Unmarshal([]byte(argsStr), &argsMap); err == nil {
										toolCall.Function.Arguments = argsMap
									}
								} else if argsMap, ok := argsRaw.(map[string]interface{}); ok {
									// ,
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

		// parsetool_call_id(toolrolemessage)
		if toolCallID, ok := msgMap["tool_call_id"].(string); ok {
			msg.ToolCallID = toolCallID
		}

		agentMessages = append(agentMessages, msg)
	}

	// last_react_output,assistantmessage
	// last_react_input,
	if reactOutput != "" {
		// messageassistantmessagetool_calls
		// tool_calls,toolmessageassistant
		if len(agentMessages) > 0 {
			lastMsg := &agentMessages[len(agentMessages)-1]
			if strings.EqualFold(lastMsg.Role, "assistant") && len(lastMsg.ToolCalls) == 0 {
				// assistantmessagetool_calls,content
				lastMsg.Content = reactOutput
			} else {
				// assistantmessage,tool_calls,addassistantmessage
				agentMessages = append(agentMessages, agent.ChatMessage{
					Role:    "assistant",
					Content: reactOutput,
				})
			}
		} else {
			// message,add
			agentMessages = append(agentMessages, agent.ChatMessage{
				Role:    "assistant",
				Content: reactOutput,
			})
		}
	}

	if len(agentMessages) == 0 {
		return nil, fmt.Errorf("ReActparsemessage")
	}

	// toolmessage,OpenAI
	// "messages with role 'tool' must be a response to a preceeding message with 'tool_calls'"error
	if h.agent != nil {
		if fixed := h.agent.RepairOrphanToolMessages(&agentMessages); fixed {
			h.logger.Info("ReActmessagetoolmessage",
				zap.String("conversationId", conversationID),
			)
		}
	}

	h.logger.Info("ReActmessage",
		zap.String("conversationId", conversationID),
		zap.String("dataSource", dataSource),
		zap.Int("originalMessageCount", messageCount),
		zap.Int("finalMessageCount", len(agentMessages)),
		zap.Bool("hasReactOutput", reactOutput != ""),
	)
	fmt.Println("agentMessages:", agentMessages) //debug
	return agentMessages, nil
}
