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
	"time"
	"unicode/utf8"

	"cyberstrike-ai/internal/agent"
	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/mcp/builtin"
	"cyberstrike-ai/internal/skills"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// safeTruncateString safely truncates a string, avoiding cuts in the middle of UTF-8 characters
func safeTruncateString(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}

	// Convert string to rune slice to correctly count characters
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}

	// Truncate to max length
	truncated := string(runes[:maxLen])

	// Try to truncate at a punctuation or space for a more natural break
	// Search backwards from the truncation point for a suitable break (no more than 20% of length)
	searchRange := maxLen / 5
	if searchRange > maxLen {
		searchRange = maxLen
	}
	breakChars := []rune("，。、 ,.;:!?！？/\\-_")
	bestBreakPos := len(runes[:maxLen])

	for i := bestBreakPos - 1; i >= bestBreakPos-searchRange && i >= 0; i-- {
		for _, breakChar := range breakChars {
			if runes[i] == breakChar {
				bestBreakPos = i + 1 // break after the punctuation
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
	config           *config.Config // config reference for accessing role information
	knowledgeManager interface {    // knowledge base manager interface
		LogRetrieval(conversationID, messageID, query, riskType string, retrievedItems []string) error
	}
	skillsManager *skills.Manager // Skills manager
}

// NewAgentHandler creates a new Agent handler
func NewAgentHandler(agent *agent.Agent, db *database.DB, cfg *config.Config, logger *zap.Logger) *AgentHandler {
	batchTaskManager := NewBatchTaskManager()
	batchTaskManager.SetDB(db)

	// Load all batch task queues from the database
	if err := batchTaskManager.LoadFromDB(); err != nil {
		logger.Warn("Failed to load batch task queues from database", zap.Error(err))
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

// SetKnowledgeManager sets the knowledge base manager (for logging retrieval)
func (h *AgentHandler) SetKnowledgeManager(manager interface {
	LogRetrieval(conversationID, messageID, query, riskType string, retrievedItems []string) error
}) {
	h.knowledgeManager = manager
}

// SetSkillsManager sets the Skills manager
func (h *AgentHandler) SetSkillsManager(manager *skills.Manager) {
	h.skillsManager = manager
}

// ChatAttachment chat attachment (user-uploaded file)
type ChatAttachment struct {
	FileName string `json:"fileName"` // file name
	Content  string `json:"content"`  // text content or base64 (whether to decode depends on MimeType)
	MimeType string `json:"mimeType,omitempty"`
}

// ChatRequest chat request
type ChatRequest struct {
	Message        string           `json:"message" binding:"required"`
	ConversationID string           `json:"conversationId,omitempty"`
	Role           string           `json:"role,omitempty"` // role name
	Attachments    []ChatAttachment `json:"attachments,omitempty"`
}

const (
	maxAttachments     = 10
	chatUploadsDirName = "chat_uploads" // root directory for conversation attachments (relative to current working directory)
)

// saveAttachmentsToDateAndConversationDir saves attachments to chat_uploads/YYYY-MM-DD/{conversationID}/, returns the saved path for each file (in the same order as attachments)
// When conversationID is empty, "_new" is used as the directory name (new conversation has no ID yet)
func saveAttachmentsToDateAndConversationDir(attachments []ChatAttachment, conversationID string, logger *zap.Logger) (savedPaths []string, err error) {
	if len(attachments) == 0 {
		return nil, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current working directory: %w", err)
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
		raw, decErr := attachmentContentToBytes(a)
		if decErr != nil {
			return nil, fmt.Errorf("failed to decode attachment %s: %w", a.FileName, decErr)
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
			logger.Debug("Conversation attachment saved", zap.Int("index", i+1), zap.String("fileName", a.FileName), zap.String("path", absPath))
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

// userMessageContentForStorage returns the user message content to be stored in the database:
// when attachments are present, appends attachment names (and paths) after the main text,
// so they are visible after page refresh and the model can retrieve paths from history when continuing the conversation
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

// appendAttachmentsToMessage only appends the saved paths of attachments to the end of the user message,
// without inlining attachment content, to avoid excessively long context
func appendAttachmentsToMessage(msg string, attachments []ChatAttachment, savedPaths []string) string {
	if len(attachments) == 0 {
		return msg
	}
	var b strings.Builder
	b.WriteString(msg)
	b.WriteString("\n\n[User-uploaded files have been saved to the following paths (please read file contents as needed, rather than relying on inline content)]\n")
	for i, a := range attachments {
		if i < len(savedPaths) && savedPaths[i] != "" {
			b.WriteString(fmt.Sprintf("- %s: %s\n", a.FileName, savedPaths[i]))
		} else {
			b.WriteString(fmt.Sprintf("- %s: (path unknown, may have failed to save)\n", a.FileName))
		}
	}
	return b.String()
}

// ChatResponse chat response
type ChatResponse struct {
	Response        string    `json:"response"`
	MCPExecutionIDs []string  `json:"mcpExecutionIds,omitempty"` // list of MCP call IDs executed in this conversation
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

	h.logger.Info("Received Agent Loop request",
		zap.String("message", req.Message),
		zap.String("conversationId", req.ConversationID),
	)

	// If no conversation ID, create a new conversation
	conversationID := req.ConversationID
	if conversationID == "" {
		title := safeTruncateString(req.Message, 50)
		conv, err := h.db.CreateConversation(title)
		if err != nil {
			h.logger.Error("Failed to create conversation", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		conversationID = conv.ID
	} else {
		// Verify the conversation exists
		_, err := h.db.GetConversation(conversationID)
		if err != nil {
			h.logger.Error("Conversation does not exist", zap.String("conversationId", conversationID), zap.Error(err))
			c.JSON(http.StatusNotFound, gin.H{"error": "Conversation does not exist"})
			return
		}
	}

	// Preferably restore history context from saved ReAct data
	agentHistoryMessages, err := h.loadHistoryFromReActData(conversationID)
	if err != nil {
		h.logger.Warn("Failed to load history messages from ReAct data, using message table", zap.Error(err))
		// Fall back to using the database message table
		historyMessages, err := h.db.GetMessages(conversationID)
		if err != nil {
			h.logger.Warn("Failed to get history messages", zap.Error(err))
			agentHistoryMessages = []agent.ChatMessage{}
		} else {
			// Convert database messages to Agent message format
			agentHistoryMessages = make([]agent.ChatMessage, 0, len(historyMessages))
			for _, msg := range historyMessages {
				agentHistoryMessages = append(agentHistoryMessages, agent.ChatMessage{
					Role:    msg.Role,
					Content: msg.Content,
				})
			}
			h.logger.Info("Loaded history messages from message table", zap.Int("count", len(agentHistoryMessages)))
		}
	} else {
		h.logger.Info("Restored history context from ReAct data", zap.Int("count", len(agentHistoryMessages)))
	}

	// Validate attachment count (non-streaming)
	if len(req.Attachments) > maxAttachments {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Maximum %d attachments allowed", maxAttachments)})
		return
	}

	// Apply role user prompt and tool configuration
	finalMessage := req.Message
	var roleTools []string  // role-configured tool list
	var roleSkills []string // role-configured skills list (used to prompt AI, but not hardcoded)
	if req.Role != "" && req.Role != "Default" {
		if h.config.Roles != nil {
			if role, exists := h.config.Roles[req.Role]; exists && role.Enabled {
				// Apply user prompt
				if role.UserPrompt != "" {
					finalMessage = role.UserPrompt + "\n\n" + req.Message
					h.logger.Info("Applying role user prompt", zap.String("role", req.Role))
				}
				// Get role-configured tool list (prefer tools field, backward compatible with mcps field)
				if len(role.Tools) > 0 {
					roleTools = role.Tools
					h.logger.Info("Using role-configured tool list", zap.String("role", req.Role), zap.Int("toolCount", len(roleTools)))
				}
				// Get role-configured skills list (used to prompt AI in system prompt, but not hardcoded)
				if len(role.Skills) > 0 {
					roleSkills = role.Skills
					h.logger.Info("Role has configured skills, will prompt AI in system prompt", zap.String("role", req.Role), zap.Int("skillCount", len(roleSkills)), zap.Strings("skills", roleSkills))
				}
			}
		}
	}
	var savedPaths []string
	if len(req.Attachments) > 0 {
		savedPaths, err = saveAttachmentsToDateAndConversationDir(req.Attachments, conversationID, h.logger)
		if err != nil {
			h.logger.Error("Failed to save conversation attachments", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save uploaded files: " + err.Error()})
			return
		}
	}
	finalMessage = appendAttachmentsToMessage(finalMessage, req.Attachments, savedPaths)

	// Save user message: when attachments are present, also save attachment names and paths,
	// so they are visible after refresh and the model can retrieve paths from history when continuing conversation
	userContent := userMessageContentForStorage(req.Message, req.Attachments, savedPaths)
	_, err = h.db.AddMessage(conversationID, "user", userContent, nil)
	if err != nil {
		h.logger.Error("Failed to save user message", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save user message: " + err.Error()})
		return
	}

	// Execute Agent Loop, passing history messages and conversation ID (using finalMessage with role prompt and role tool list)
	// Note: skills are not hardcoded, but the system prompt will hint to AI which skills this role recommends
	result, err := h.agent.AgentLoopWithProgress(c.Request.Context(), finalMessage, agentHistoryMessages, conversationID, nil, roleTools, roleSkills)
	if err != nil {
		h.logger.Error("Agent Loop execution failed", zap.Error(err))

		// Even if execution fails, try to save ReAct data (if result contains any)
		if result != nil && (result.LastReActInput != "" || result.LastReActOutput != "") {
			if saveErr := h.db.SaveReActData(conversationID, result.LastReActInput, result.LastReActOutput); saveErr != nil {
				h.logger.Warn("Failed to save ReAct data for failed task", zap.Error(saveErr))
			} else {
				h.logger.Info("Saved ReAct data for failed task", zap.String("conversationId", conversationID))
			}
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Save assistant reply
	_, err = h.db.AddMessage(conversationID, "assistant", result.Response, result.MCPExecutionIDs)
	if err != nil {
		h.logger.Error("Failed to save assistant message", zap.Error(err))
		// Even if saving fails, return the response, but log the error
		// Because AI has already generated a reply, the user should be able to see it
	}

	// Save the last round of ReAct input and output
	if result.LastReActInput != "" || result.LastReActOutput != "" {
		if err := h.db.SaveReActData(conversationID, result.LastReActInput, result.LastReActOutput); err != nil {
			h.logger.Warn("Failed to save ReAct data", zap.Error(err))
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

// ProcessMessageForRobot is called by robots (WeCom/DingTalk/Lark): same execution path as /api/agent-loop/stream
// (includes progressCallback and process details), but without sending SSE, and returns the full reply at the end
func (h *AgentHandler) ProcessMessageForRobot(ctx context.Context, conversationID, message, role string) (response string, convID string, err error) {
	if conversationID == "" {
		title := safeTruncateString(message, 50)
		conv, createErr := h.db.CreateConversation(title)
		if createErr != nil {
			return "", "", fmt.Errorf("failed to create conversation: %w", createErr)
		}
		conversationID = conv.ID
	} else {
		if _, getErr := h.db.GetConversation(conversationID); getErr != nil {
			return "", "", fmt.Errorf("conversation does not exist")
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
	if role != "" && role != "Default" && h.config.Roles != nil {
		if r, exists := h.config.Roles[role]; exists && r.Enabled {
			if r.UserPrompt != "" {
				finalMessage = r.UserPrompt + "\n\n" + message
			}
			roleTools = r.Tools
			roleSkills = r.Skills
		}
	}

	if _, err = h.db.AddMessage(conversationID, "user", message, nil); err != nil {
		return "", "", fmt.Errorf("failed to save user message: %w", err)
	}

	// Consistent with agent-loop/stream: first create assistant message placeholder,
	// use progressCallback to write process details (without sending SSE)
	assistantMsg, err := h.db.AddMessage(conversationID, "assistant", "Processing...", nil)
	if err != nil {
		h.logger.Warn("Robot: failed to create assistant message placeholder", zap.Error(err))
	}
	var assistantMessageID string
	if assistantMsg != nil {
		assistantMessageID = assistantMsg.ID
	}
	progressCallback := h.createProgressCallback(conversationID, assistantMessageID, nil)

	result, err := h.agent.AgentLoopWithProgress(ctx, finalMessage, agentHistoryMessages, conversationID, progressCallback, roleTools, roleSkills)
	if err != nil {
		errMsg := "Execution failed: " + err.Error()
		if assistantMessageID != "" {
			_, _ = h.db.Exec("UPDATE messages SET content = ? WHERE id = ?", errMsg, assistantMessageID)
			_ = h.db.AddProcessDetail(assistantMessageID, conversationID, "error", errMsg, nil)
		}
		return "", conversationID, err
	}

	// Update assistant message content and MCP execution IDs (consistent with stream)
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
			h.logger.Warn("Robot: failed to update assistant message", zap.Error(err))
		}
	} else {
		if _, err = h.db.AddMessage(conversationID, "assistant", result.Response, result.MCPExecutionIDs); err != nil {
			h.logger.Warn("Robot: failed to save assistant message", zap.Error(err))
		}
	}
	if result.LastReActInput != "" || result.LastReActOutput != "" {
		_ = h.db.SaveReActData(conversationID, result.LastReActInput, result.LastReActOutput)
	}
	return result.Response, conversationID, nil
}

// StreamEvent streaming event
type StreamEvent struct {
	Type    string      `json:"type"`    // conversation, progress, tool_call, tool_result, response, error, cancelled, done
	Message string      `json:"message"` // display message
	Data    interface{} `json:"data,omitempty"`
}

// createProgressCallback creates a progress callback function for saving processDetails
// sendEventFunc: optional streaming event send function, if nil then no streaming events are sent
func (h *AgentHandler) createProgressCallback(conversationID, assistantMessageID string, sendEventFunc func(eventType, message string, data interface{})) agent.ProgressCallback {
	// Used to cache parameters from tool_call events, to be used when tool_result arrives
	toolCallCache := make(map[string]map[string]interface{}) // toolCallId -> arguments

	return func(eventType, message string, data interface{}) {
		// If sendEventFunc is provided, send streaming event
		if sendEventFunc != nil {
			sendEventFunc(eventType, message, data)
		}

		// Cache parameters from tool_call events
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

		// Handle knowledge retrieval log recording
		if eventType == "tool_result" && h.knowledgeManager != nil {
			if dataMap, ok := data.(map[string]interface{}); ok {
				toolName, _ := dataMap["toolName"].(string)
				if toolName == builtin.ToolSearchKnowledgeBase {
					// Extract retrieval information
					query := ""
					riskType := ""
					var retrievedItems []string

					// First try to get parameters from tool_call cache
					if toolCallId, ok := dataMap["toolCallId"].(string); ok && toolCallId != "" {
						if cachedArgs, exists := toolCallCache[toolCallId]; exists {
							if q, ok := cachedArgs["query"].(string); ok && q != "" {
								query = q
							}
							if rt, ok := cachedArgs["risk_type"].(string); ok && rt != "" {
								riskType = rt
							}
							// Clean up cache after use
							delete(toolCallCache, toolCallId)
						}
					}

					// If not in cache, try to extract from argumentsObj
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

					// If query is still empty, try to extract from result (from first line of result text)
					if query == "" {
						if result, ok := dataMap["result"].(string); ok && result != "" {
							// Try to extract query from result (if result contains "No knowledge related to query 'xxx' found")
										if strings.Contains(result, "No relevant knowledge found for query '") {
											start := strings.Index(result, "No relevant knowledge found for query '") + len("No relevant knowledge found for query '")
								end := strings.Index(result[start:], "'")
								if end > 0 {
									query = result[start : start+end]
								}
							}
						}
						// If still empty, use default value
						if query == "" {
							query = "unknown query"
						}
					}

					// Extract retrieved knowledge item IDs from tool results
					// Result format: "Found X related entries:\n\n--- Result 1 (similarity: XX.XX%) ---\nSource: [category] title\n...\n<!-- METADATA: {...} -->"
					if result, ok := dataMap["result"].(string); ok && result != "" {
						// Try to extract knowledge item IDs from metadata
						metadataMatch := strings.Index(result, "<!-- METADATA:")
						if metadataMatch > 0 {
							// Extract metadata JSON
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

						// If not extracted from metadata, but result contains "found", at least mark as having results
								if len(retrievedItems) == 0 && strings.Contains(result, "Found") && !strings.Contains(result, "Not found") {
							// Has results but cannot accurately extract IDs, use special marker
							retrievedItems = []string{"_has_results"}
						}
					}

					// Log retrieval (async, non-blocking)
					go func() {
						if err := h.knowledgeManager.LogRetrieval(conversationID, assistantMessageID, query, riskType, retrievedItems); err != nil {
							h.logger.Warn("Failed to log knowledge retrieval", zap.Error(err))
						}
					}()

					// Add knowledge retrieval event to processDetails
					if assistantMessageID != "" {
						retrievalData := map[string]interface{}{
							"query":    query,
							"riskType": riskType,
							"toolName": toolName,
						}
						if err := h.db.AddProcessDetail(assistantMessageID, conversationID, "knowledge_retrieval", fmt.Sprintf("Retrieve knowledge: %s", query), retrievalData); err != nil {
							h.logger.Warn("Failed to save knowledge retrieval details", zap.Error(err))
						}
					}
				}
			}
		}

		// Save process details to database (excluding response and done events, which are handled separately later)
		if assistantMessageID != "" && eventType != "response" && eventType != "done" {
			if err := h.db.AddProcessDetail(assistantMessageID, conversationID, eventType, message, data); err != nil {
				h.logger.Warn("Failed to save process details", zap.Error(err), zap.String("eventType", eventType))
			}
		}
	}
}

// AgentLoopStream handles Agent Loop streaming requests
func (h *AgentHandler) AgentLoopStream(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// For streaming requests, also send SSE-formatted errors
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		event := StreamEvent{
			Type:    "error",
			Message: "Invalid request parameters: " + err.Error(),
		}
		eventJSON, _ := json.Marshal(event)
		fmt.Fprintf(c.Writer, "data: %s\n\n", eventJSON)
		c.Writer.Flush()
		return
	}

	h.logger.Info("Received Agent Loop streaming request",
		zap.String("message", req.Message),
		zap.String("conversationId", req.ConversationID),
	)

	// Set SSE response headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no") // disable nginx buffering

	// Send initial event
	// Used to track if the client has disconnected
	clientDisconnected := false

	sendEvent := func(eventType, message string, data interface{}) {
		// If client has disconnected, stop sending events
		if clientDisconnected {
			return
		}

		// Check if request context has been cancelled (client disconnected)
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

		// Try to write event, if failed mark client as disconnected
		if _, err := fmt.Fprintf(c.Writer, "data: %s\n\n", eventJSON); err != nil {
			clientDisconnected = true
			h.logger.Debug("Client disconnected, stopping SSE event sending", zap.Error(err))
			return
		}

		// Flush response, if failed mark client as disconnected
		if flusher, ok := c.Writer.(http.Flusher); ok {
			flusher.Flush()
		} else {
			c.Writer.Flush()
		}
	}

	// If no conversation ID, create a new conversation
	conversationID := req.ConversationID
	if conversationID == "" {
		title := safeTruncateString(req.Message, 50)
		conv, err := h.db.CreateConversation(title)
		if err != nil {
			h.logger.Error("Failed to create conversation", zap.Error(err))
			sendEvent("error", "Failed to create conversation: "+err.Error(), nil)
			return
		}
		conversationID = conv.ID
		sendEvent("conversation", "Session created", map[string]interface{}{
			"conversationId": conversationID,
		})
	} else {
		// Verify the conversation exists
		_, err := h.db.GetConversation(conversationID)
		if err != nil {
			h.logger.Error("Conversation does not exist", zap.String("conversationId", conversationID), zap.Error(err))
			sendEvent("error", "Conversation does not exist", nil)
			return
		}
	}

	// Preferably restore history context from saved ReAct data
	agentHistoryMessages, err := h.loadHistoryFromReActData(conversationID)
	if err != nil {
		h.logger.Warn("Failed to load history messages from ReAct data, using message table", zap.Error(err))
		// Fall back to using the database message table
		historyMessages, err := h.db.GetMessages(conversationID)
		if err != nil {
			h.logger.Warn("Failed to get history messages", zap.Error(err))
			agentHistoryMessages = []agent.ChatMessage{}
		} else {
			// Convert database messages to Agent message format
			agentHistoryMessages = make([]agent.ChatMessage, 0, len(historyMessages))
			for _, msg := range historyMessages {
				agentHistoryMessages = append(agentHistoryMessages, agent.ChatMessage{
					Role:    msg.Role,
					Content: msg.Content,
				})
			}
			h.logger.Info("Loaded history messages from message table", zap.Int("count", len(agentHistoryMessages)))
		}
	} else {
		h.logger.Info("Restored history context from ReAct data", zap.Int("count", len(agentHistoryMessages)))
	}

	// Validate attachment count
	if len(req.Attachments) > maxAttachments {
		sendEvent("error", fmt.Sprintf("Maximum %d attachments allowed", maxAttachments), nil)
		return
	}

	// Apply role user prompt and tool configuration
	finalMessage := req.Message
	var roleTools []string // role-configured tool list
	if req.Role != "" && req.Role != "Default" {
		if h.config.Roles != nil {
			if role, exists := h.config.Roles[req.Role]; exists && role.Enabled {
				// Apply user prompt
				if role.UserPrompt != "" {
					finalMessage = role.UserPrompt + "\n\n" + req.Message
					h.logger.Info("Applying role user prompt", zap.String("role", req.Role))
				}
				// Get role-configured tool list (prefer tools field, backward compatible with mcps field)
				if len(role.Tools) > 0 {
					roleTools = role.Tools
					h.logger.Info("Using role-configured tool list", zap.String("role", req.Role), zap.Int("toolCount", len(roleTools)))
				} else if len(role.MCPs) > 0 {
					// Backward compatibility: if only mcps field exists, temporarily use empty list (means use all tools)
					// Because mcps is MCP server names, not tool list
					h.logger.Info("Role config uses old mcps field, will use all tools", zap.String("role", req.Role))
				}
				// Note: role-configured skills are no longer hardcoded; AI can call them on demand via list_skills and read_skill tools
				if len(role.Skills) > 0 {
					h.logger.Info("Role has configured skills, AI can call them on demand via tools", zap.String("role", req.Role), zap.Int("skillCount", len(role.Skills)), zap.Strings("skills", role.Skills))
				}
			}
		}
	}
	var savedPaths []string
	if len(req.Attachments) > 0 {
		savedPaths, err = saveAttachmentsToDateAndConversationDir(req.Attachments, conversationID, h.logger)
		if err != nil {
			h.logger.Error("Failed to save conversation attachments", zap.Error(err))
			sendEvent("error", "Failed to save uploaded files: "+err.Error(), nil)
			return
		}
	}
	// Only append attachment saved paths to finalMessage, avoid inlining file content into model context
	finalMessage = appendAttachmentsToMessage(finalMessage, req.Attachments, savedPaths)
	// If roleTools is empty, it means use all tools (default role or role without configured tools)

	// Save user message: when attachments are present, also save attachment names and paths,
	// so they are visible after refresh and the model can retrieve paths from history when continuing conversation
	userContent := userMessageContentForStorage(req.Message, req.Attachments, savedPaths)
	_, err = h.db.AddMessage(conversationID, "user", userContent, nil)
	if err != nil {
		h.logger.Error("Failed to save user message", zap.Error(err))
	}

	// Pre-create assistant message for associating process details
	assistantMsg, err := h.db.AddMessage(conversationID, "assistant", "Processing...", nil)
	if err != nil {
		h.logger.Error("Failed to create assistant message", zap.Error(err))
		// If creation fails, continue execution but don't save process details
		assistantMsg = nil
	}

	// Create progress callback function, also saving to database
	var assistantMessageID string
	if assistantMsg != nil {
		assistantMessageID = assistantMsg.ID
	}

	// Create progress callback function, reusing unified logic
	progressCallback := h.createProgressCallback(conversationID, assistantMessageID, sendEvent)

	// Create an independent context for task execution, not cancelled with the HTTP request
	// This way, even if the client disconnects (e.g., page refresh), the task continues executing
	baseCtx, cancelWithCause := context.WithCancelCause(context.Background())
	taskCtx, timeoutCancel := context.WithTimeout(baseCtx, 600*time.Minute)
	defer timeoutCancel()
	defer cancelWithCause(nil)

	if _, err := h.tasks.StartTask(conversationID, req.Message, cancelWithCause); err != nil {
		var errorMsg string
		if errors.Is(err, ErrTaskAlreadyRunning) {
			errorMsg = "⚠️ A task is already running in this session. Please wait for the current task to complete or click the 'Stop Task' button before trying again."
			sendEvent("error", errorMsg, map[string]interface{}{
				"conversationId": conversationID,
				"errorType":      "task_already_running",
			})
		} else {
			errorMsg = "❌ Failed to start task: " + err.Error()
			sendEvent("error", errorMsg, map[string]interface{}{
				"conversationId": conversationID,
				"errorType":      "task_start_failed",
			})
		}

		// Update assistant message content and save error details to database
		if assistantMessageID != "" {
			if _, updateErr := h.db.Exec(
				"UPDATE messages SET content = ? WHERE id = ?",
				errorMsg,
				assistantMessageID,
			); updateErr != nil {
				h.logger.Warn("Failed to update assistant message after error", zap.Error(updateErr))
			}
			// Save error details to database
			if err := h.db.AddProcessDetail(assistantMessageID, conversationID, "error", errorMsg, map[string]interface{}{
				"errorType": func() string {
					if errors.Is(err, ErrTaskAlreadyRunning) {
						return "task_already_running"
					}
					return "task_start_failed"
				}(),
			}); err != nil {
				h.logger.Warn("Failed to save error details", zap.Error(err))
			}
		}

		sendEvent("done", "", map[string]interface{}{
			"conversationId": conversationID,
		})
		return
	}

	taskStatus := "completed"
	defer h.tasks.FinishTask(conversationID, taskStatus)

	// Execute Agent Loop with independent context, ensuring task is not interrupted by client disconnect (using finalMessage with role prompt and role tool list)
	sendEvent("progress", "Analyzing your request...", nil)
	// Note: skills are not hardcoded, but the system prompt will hint to AI which skills this role recommends
	var roleSkills []string // role-configured skills list (used to prompt AI, but not hardcoded)
	if req.Role != "" && req.Role != "Default" {
		if h.config.Roles != nil {
			if role, exists := h.config.Roles[req.Role]; exists && role.Enabled {
				if len(role.Skills) > 0 {
					roleSkills = role.Skills
				}
			}
		}
	}
	result, err := h.agent.AgentLoopWithProgress(taskCtx, finalMessage, agentHistoryMessages, conversationID, progressCallback, roleTools, roleSkills)
	if err != nil {
		h.logger.Error("Agent Loop execution failed", zap.Error(err))
		cause := context.Cause(baseCtx)

		// Check if it is a user cancellation: the context cause is ErrTaskCancelled
		// If cause is ErrTaskCancelled, regardless of the error type (including context.Canceled), treat as user cancellation
		// This correctly handles cancellation during API calls
		isCancelled := errors.Is(cause, ErrTaskCancelled)

		switch {
		case isCancelled:
			taskStatus = "cancelled"
			cancelMsg := "Task has been cancelled by the user, subsequent operations have been stopped."

			// Update task status before sending event, to ensure frontend sees the status change in time
			h.tasks.UpdateTaskStatus(conversationID, taskStatus)

			if assistantMessageID != "" {
				if _, updateErr := h.db.Exec(
					"UPDATE messages SET content = ? WHERE id = ?",
					cancelMsg,
					assistantMessageID,
				); updateErr != nil {
					h.logger.Warn("Failed to update assistant message after cancellation", zap.Error(updateErr))
				}
				h.db.AddProcessDetail(assistantMessageID, conversationID, "cancelled", cancelMsg, nil)
			}

			// Even if the task is cancelled, try to save ReAct data (if result contains any)
			if result != nil && (result.LastReActInput != "" || result.LastReActOutput != "") {
				if err := h.db.SaveReActData(conversationID, result.LastReActInput, result.LastReActOutput); err != nil {
					h.logger.Warn("Failed to save ReAct data for cancelled task", zap.Error(err))
				} else {
					h.logger.Info("Saved ReAct data for cancelled task", zap.String("conversationId", conversationID))
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
			timeoutMsg := "Task execution timed out and has been automatically terminated."

			// Update task status before sending event, to ensure frontend sees the status change in time
			h.tasks.UpdateTaskStatus(conversationID, taskStatus)

			if assistantMessageID != "" {
				if _, updateErr := h.db.Exec(
					"UPDATE messages SET content = ? WHERE id = ?",
					timeoutMsg,
					assistantMessageID,
				); updateErr != nil {
					h.logger.Warn("Failed to update assistant message after timeout", zap.Error(updateErr))
				}
				h.db.AddProcessDetail(assistantMessageID, conversationID, "timeout", timeoutMsg, nil)
			}

			// Even if the task times out, try to save ReAct data (if result contains any)
			if result != nil && (result.LastReActInput != "" || result.LastReActOutput != "") {
				if err := h.db.SaveReActData(conversationID, result.LastReActInput, result.LastReActOutput); err != nil {
					h.logger.Warn("Failed to save ReAct data for timed-out task", zap.Error(err))
				} else {
					h.logger.Info("Saved ReAct data for timed-out task", zap.String("conversationId", conversationID))
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
			errorMsg := "Execution failed: " + err.Error()

			// Update task status before sending event, to ensure frontend sees the status change in time
			h.tasks.UpdateTaskStatus(conversationID, taskStatus)

			if assistantMessageID != "" {
				if _, updateErr := h.db.Exec(
					"UPDATE messages SET content = ? WHERE id = ?",
					errorMsg,
					assistantMessageID,
				); updateErr != nil {
					h.logger.Warn("Failed to update assistant message after failure", zap.Error(updateErr))
				}
				h.db.AddProcessDetail(assistantMessageID, conversationID, "error", errorMsg, nil)
			}

			// Even if the task fails, try to save ReAct data (if result contains any)
			if result != nil && (result.LastReActInput != "" || result.LastReActOutput != "") {
				if err := h.db.SaveReActData(conversationID, result.LastReActInput, result.LastReActOutput); err != nil {
					h.logger.Warn("Failed to save ReAct data for failed task", zap.Error(err))
				} else {
					h.logger.Info("Saved ReAct data for failed task", zap.String("conversationId", conversationID))
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

	// Update assistant message content
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
			h.logger.Error("Failed to update assistant message", zap.Error(err))
		}
	} else {
		// If creation failed earlier, create now
		_, err = h.db.AddMessage(conversationID, "assistant", result.Response, result.MCPExecutionIDs)
		if err != nil {
			h.logger.Error("Failed to save assistant message", zap.Error(err))
		}
	}

	// Save the last round of ReAct input and output
	if result.LastReActInput != "" || result.LastReActOutput != "" {
		if err := h.db.SaveReActData(conversationID, result.LastReActInput, result.LastReActOutput); err != nil {
			h.logger.Warn("Failed to save ReAct data", zap.Error(err))
		} else {
			h.logger.Info("ReAct data saved", zap.String("conversationId", conversationID))
		}
	}

	// Send final response
	sendEvent("response", result.Response, map[string]interface{}{
		"mcpExecutionIds": result.MCPExecutionIDs,
		"conversationId":  conversationID,
		"messageId":       assistantMessageID, // include message ID so frontend can associate process details
	})
	sendEvent("done", "", map[string]interface{}{
		"conversationId": conversationID,
	})
}

// CancelAgentLoop cancels a running task
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
		h.logger.Error("Failed to cancel task", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "No running task found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":         "cancelling",
		"conversationId": req.ConversationID,
		"message":        "Cancellation request submitted. Task will stop after the current step completes.",
	})
}

// ListAgentTasks lists all running tasks
func (h *AgentHandler) ListAgentTasks(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"tasks": h.tasks.GetActiveTasks(),
	})
}

// ListCompletedTasks lists recently completed task history
func (h *AgentHandler) ListCompletedTasks(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"tasks": h.tasks.GetCompletedTasks(),
	})
}

// BatchTaskRequest batch task request
type BatchTaskRequest struct {
	Title string   `json:"title"`                    // task title (optional)
	Tasks []string `json:"tasks" binding:"required"` // task list, one task per line
	Role  string   `json:"role,omitempty"`           // role name (optional, empty string means default role)
}

// CreateBatchQueue creates a batch task queue
func (h *AgentHandler) CreateBatchQueue(c *gin.Context) {
	var req BatchTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(req.Tasks) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Task list cannot be empty"})
		return
	}

	// Filter empty tasks
	validTasks := make([]string, 0, len(req.Tasks))
	for _, task := range req.Tasks {
		if task != "" {
			validTasks = append(validTasks, task)
		}
	}

	if len(validTasks) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No valid tasks"})
		return
	}

	queue := h.batchTaskManager.CreateBatchQueue(req.Title, req.Role, validTasks)
	c.JSON(http.StatusOK, gin.H{
		"queueId": queue.ID,
		"queue":   queue,
	})
}

// GetBatchQueue gets a batch task queue
func (h *AgentHandler) GetBatchQueue(c *gin.Context) {
	queueID := c.Param("queueId")
	queue, exists := h.batchTaskManager.GetBatchQueue(queueID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Queue does not exist"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"queue": queue})
}

// ListBatchQueuesResponse batch task queue list response
type ListBatchQueuesResponse struct {
	Queues     []*BatchTaskQueue `json:"queues"`
	Total      int               `json:"total"`
	Page       int               `json:"page"`
	PageSize   int               `json:"page_size"`
	TotalPages int               `json:"total_pages"`
}

// ListBatchQueues lists all batch task queues (supports filtering and pagination)
func (h *AgentHandler) ListBatchQueues(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "10")
	offsetStr := c.DefaultQuery("offset", "0")
	pageStr := c.Query("page")
	status := c.Query("status")
	keyword := c.Query("keyword")

	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)
	page := 1

	// If page parameter is provided, use it to calculate offset
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
			offset = (page - 1) * limit
		}
	}

	// Limit pageSize range
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	// Default status is "all"
	if status == "" {
		status = "all"
	}

	// Get queue list and total count
	queues, total, err := h.batchTaskManager.ListQueues(limit, offset, status, keyword)
	if err != nil {
		h.logger.Error("Failed to get batch task queue list", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Calculate total pages
	totalPages := (total + limit - 1) / limit
	if totalPages == 0 {
		totalPages = 1
	}

	// If using offset to calculate page, recalculate
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

// StartBatchQueue starts executing a batch task queue
func (h *AgentHandler) StartBatchQueue(c *gin.Context) {
	queueID := c.Param("queueId")
	queue, exists := h.batchTaskManager.GetBatchQueue(queueID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Queue does not exist"})
		return
	}

	if queue.Status != "pending" && queue.Status != "paused" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Queue status does not allow starting"})
		return
	}

	// Execute batch tasks in the background
	go h.executeBatchQueue(queueID)

	h.batchTaskManager.UpdateQueueStatus(queueID, "running")
	c.JSON(http.StatusOK, gin.H{"message": "Batch tasks have started executing", "queueId": queueID})
}

// PauseBatchQueue pauses a batch task queue
func (h *AgentHandler) PauseBatchQueue(c *gin.Context) {
	queueID := c.Param("queueId")
	success := h.batchTaskManager.PauseQueue(queueID)
	if !success {
		c.JSON(http.StatusNotFound, gin.H{"error": "Queue does not exist or cannot be paused"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Batch tasks paused"})
}

// DeleteBatchQueue deletes a batch task queue
func (h *AgentHandler) DeleteBatchQueue(c *gin.Context) {
	queueID := c.Param("queueId")
	success := h.batchTaskManager.DeleteQueue(queueID)
	if !success {
		c.JSON(http.StatusNotFound, gin.H{"error": "Queue does not exist"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Batch task queue deleted"})
}

// UpdateBatchTask updates a batch task message
func (h *AgentHandler) UpdateBatchTask(c *gin.Context) {
	queueID := c.Param("queueId")
	taskID := c.Param("taskId")

	var req struct {
		Message string `json:"message" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request parameters: " + err.Error()})
		return
	}

	if req.Message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Task message cannot be empty"})
		return
	}

	err := h.batchTaskManager.UpdateTaskMessage(queueID, taskID, req.Message)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Return updated queue information
	queue, exists := h.batchTaskManager.GetBatchQueue(queueID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Queue does not exist"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Task updated", "queue": queue})
}

// AddBatchTask adds a task to a batch task queue
func (h *AgentHandler) AddBatchTask(c *gin.Context) {
	queueID := c.Param("queueId")

	var req struct {
		Message string `json:"message" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request parameters: " + err.Error()})
		return
	}

	if req.Message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Task message cannot be empty"})
		return
	}

	task, err := h.batchTaskManager.AddTaskToQueue(queueID, req.Message)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Return updated queue information
	queue, exists := h.batchTaskManager.GetBatchQueue(queueID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Queue does not exist"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Task added", "task": task, "queue": queue})
}

// DeleteBatchTask deletes a batch task
func (h *AgentHandler) DeleteBatchTask(c *gin.Context) {
	queueID := c.Param("queueId")
	taskID := c.Param("taskId")

	err := h.batchTaskManager.DeleteTask(queueID, taskID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Return updated queue information
	queue, exists := h.batchTaskManager.GetBatchQueue(queueID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Queue does not exist"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Task deleted", "queue": queue})
}

// executeBatchQueue executes a batch task queue
func (h *AgentHandler) executeBatchQueue(queueID string) {
	h.logger.Info("Starting batch task queue execution", zap.String("queueId", queueID))

	for {
		// Check queue status
		queue, exists := h.batchTaskManager.GetBatchQueue(queueID)
		if !exists || queue.Status == "cancelled" || queue.Status == "completed" || queue.Status == "paused" {
			break
		}

		// Get next task
		task, hasNext := h.batchTaskManager.GetNextTask(queueID)
		if !hasNext {
			// All tasks completed
			h.batchTaskManager.UpdateQueueStatus(queueID, "completed")
			h.logger.Info("Batch task queue execution completed", zap.String("queueId", queueID))
			break
		}

		// Update task status to running
		h.batchTaskManager.UpdateTaskStatus(queueID, task.ID, "running", "", "")

		// Create new conversation
		title := safeTruncateString(task.Message, 50)
		conv, err := h.db.CreateConversation(title)
		var conversationID string
		if err != nil {
			h.logger.Error("Failed to create conversation", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(err))
			h.batchTaskManager.UpdateTaskStatus(queueID, task.ID, "failed", "", "Failed to create conversation: "+err.Error())
			h.batchTaskManager.MoveToNextTask(queueID)
			continue
		}
		conversationID = conv.ID

		// Save conversationId to task (even in running state, to enable viewing conversation)
		h.batchTaskManager.UpdateTaskStatusWithConversationID(queueID, task.ID, "running", "", "", conversationID)

		// Apply role user prompt and tool configuration
		finalMessage := task.Message
		var roleTools []string  // role-configured tool list
		var roleSkills []string // role-configured skills list (used to prompt AI, but not hardcoded)
		if queue.Role != "" && queue.Role != "Default" {
			if h.config.Roles != nil {
				if role, exists := h.config.Roles[queue.Role]; exists && role.Enabled {
					// Apply user prompt
					if role.UserPrompt != "" {
						finalMessage = role.UserPrompt + "\n\n" + task.Message
						h.logger.Info("Applying role user prompt", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("role", queue.Role))
					}
					// Get role-configured tool list (prefer tools field, backward compatible with mcps field)
					if len(role.Tools) > 0 {
						roleTools = role.Tools
						h.logger.Info("Using role-configured tool list", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("role", queue.Role), zap.Int("toolCount", len(roleTools)))
					}
					// Get role-configured skills list (used to prompt AI in system prompt, but not hardcoded)
					if len(role.Skills) > 0 {
						roleSkills = role.Skills
						h.logger.Info("Role has configured skills, will prompt AI in system prompt", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("role", queue.Role), zap.Int("skillCount", len(roleSkills)), zap.Strings("skills", roleSkills))
					}
				}
			}
		}

		// Save user message (save original message, without role prompt)
		_, err = h.db.AddMessage(conversationID, "user", task.Message, nil)
		if err != nil {
			h.logger.Error("Failed to save user message", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("conversationId", conversationID), zap.Error(err))
		}

		// Pre-create assistant message for associating process details
		assistantMsg, err := h.db.AddMessage(conversationID, "assistant", "Processing...", nil)
		if err != nil {
			h.logger.Error("Failed to create assistant message", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("conversationId", conversationID), zap.Error(err))
			// If creation fails, continue execution but don't save process details
			assistantMsg = nil
		}

		// Create progress callback function, reusing unified logic (batch tasks don't need streaming events, so nil is passed)
		var assistantMessageID string
		if assistantMsg != nil {
			assistantMessageID = assistantMsg.ID
		}
		progressCallback := h.createProgressCallback(conversationID, assistantMessageID, nil)

		// Execute task (using finalMessage with role prompt and role tool list)
		h.logger.Info("Executing batch task", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("message", task.Message), zap.String("role", queue.Role), zap.String("conversationId", conversationID))

		// Single subtask timeout: adjusted from 30 minutes to 6 hours, for long-running penetration/scan tasks
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Hour)
		// Store cancel function so it can be cancelled when the queue is cancelled
		h.batchTaskManager.SetTaskCancel(queueID, cancel)
		// Use role-configured tool list from queue config (if empty, use all tools)
		// Note: skills are not hardcoded, but the system prompt will hint to AI which skills this role recommends
		result, err := h.agent.AgentLoopWithProgress(ctx, finalMessage, []agent.ChatMessage{}, conversationID, progressCallback, roleTools, roleSkills)
		// Task execution completed, clean up cancel function
		h.batchTaskManager.SetTaskCancel(queueID, nil)
		cancel()

		if err != nil {
			// Check if it is a cancellation error
			// 1. Directly check if it is context.Canceled (including wrapped errors)
			// 2. Check if the error message contains "context canceled" or "cancelled" keywords
			// 3. Check if result.Response contains cancellation-related messages
			errStr := err.Error()
			isCancelled := errors.Is(err, context.Canceled) ||
				strings.Contains(strings.ToLower(errStr), "context canceled") ||
				strings.Contains(strings.ToLower(errStr), "context cancelled") ||
				(result != nil && result.Response != "" && (strings.Contains(result.Response, "Task has been cancelled") || strings.Contains(result.Response, "Task execution interrupted")))

			if isCancelled {
				h.logger.Info("Batch task cancelled", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("conversationId", conversationID))
				cancelMsg := "Task has been cancelled by the user, subsequent operations have been stopped."
				// If result contains a more specific cancellation message, use it
				if result != nil && result.Response != "" && (strings.Contains(result.Response, "Task has been cancelled") || strings.Contains(result.Response, "Task execution interrupted")) {
					cancelMsg = result.Response
				}
				// Update assistant message content
				if assistantMessageID != "" {
					if _, updateErr := h.db.Exec(
						"UPDATE messages SET content = ? WHERE id = ?",
						cancelMsg,
						assistantMessageID,
					); updateErr != nil {
						h.logger.Warn("Failed to update assistant message after cancellation", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(updateErr))
					}
					// Save cancellation details to database
					if err := h.db.AddProcessDetail(assistantMessageID, conversationID, "cancelled", cancelMsg, nil); err != nil {
						h.logger.Warn("Failed to save cancellation details", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(err))
					}
				} else {
					// If no pre-created assistant message, create a new one
					_, errMsg := h.db.AddMessage(conversationID, "assistant", cancelMsg, nil)
					if errMsg != nil {
						h.logger.Warn("Failed to save cancellation message", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(errMsg))
					}
				}
				// Save ReAct data (if exists)
				if result != nil && (result.LastReActInput != "" || result.LastReActOutput != "") {
					if err := h.db.SaveReActData(conversationID, result.LastReActInput, result.LastReActOutput); err != nil {
						h.logger.Warn("Failed to save ReAct data for cancelled task", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(err))
					}
				}
				h.batchTaskManager.UpdateTaskStatusWithConversationID(queueID, task.ID, "cancelled", cancelMsg, "", conversationID)
			} else {
				h.logger.Error("Batch task execution failed", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("conversationId", conversationID), zap.Error(err))
				errorMsg := "Execution failed: " + err.Error()
				// Update assistant message content
				if assistantMessageID != "" {
					if _, updateErr := h.db.Exec(
						"UPDATE messages SET content = ? WHERE id = ?",
						errorMsg,
						assistantMessageID,
					); updateErr != nil {
						h.logger.Warn("Failed to update assistant message after failure", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(updateErr))
					}
					// Save error details to database
					if err := h.db.AddProcessDetail(assistantMessageID, conversationID, "error", errorMsg, nil); err != nil {
						h.logger.Warn("Failed to save error details", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(err))
					}
				}
				h.batchTaskManager.UpdateTaskStatus(queueID, task.ID, "failed", "", err.Error())
			}
		} else {
			h.logger.Info("Batch task executed successfully", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("conversationId", conversationID))

			// Update assistant message content
			if assistantMessageID != "" {
				mcpIDsJSON := ""
				if len(result.MCPExecutionIDs) > 0 {
					jsonData, _ := json.Marshal(result.MCPExecutionIDs)
					mcpIDsJSON = string(jsonData)
				}
				if _, updateErr := h.db.Exec(
					"UPDATE messages SET content = ?, mcp_execution_ids = ? WHERE id = ?",
					result.Response,
					mcpIDsJSON,
					assistantMessageID,
				); updateErr != nil {
					h.logger.Warn("Failed to update assistant message", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(updateErr))
					// If update fails, try to create a new message
					_, err = h.db.AddMessage(conversationID, "assistant", result.Response, result.MCPExecutionIDs)
					if err != nil {
						h.logger.Error("Failed to save assistant message", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("conversationId", conversationID), zap.Error(err))
					}
				}
			} else {
				// If no pre-created assistant message, create a new one
				_, err = h.db.AddMessage(conversationID, "assistant", result.Response, result.MCPExecutionIDs)
				if err != nil {
					h.logger.Error("Failed to save assistant message", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("conversationId", conversationID), zap.Error(err))
				}
			}

			// Save ReAct data
			if result.LastReActInput != "" || result.LastReActOutput != "" {
				if err := h.db.SaveReActData(conversationID, result.LastReActInput, result.LastReActOutput); err != nil {
					h.logger.Warn("Failed to save ReAct data", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(err))
				} else {
					h.logger.Info("ReAct data saved", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("conversationId", conversationID))
				}
			}

			// Save result
			h.batchTaskManager.UpdateTaskStatusWithConversationID(queueID, task.ID, "completed", result.Response, "", conversationID)
		}

		// Move to next task
		h.batchTaskManager.MoveToNextTask(queueID)

		// Check if cancelled or paused
		queue, _ = h.batchTaskManager.GetBatchQueue(queueID)
		if queue.Status == "cancelled" || queue.Status == "paused" {
			break
		}
	}
}

// loadHistoryFromReActData restores history message context from saved ReAct data
// Uses similar concatenation logic as attack chain generation: preferably uses saved last_react_input and last_react_output,
// falls back to message table if not available
func (h *AgentHandler) loadHistoryFromReActData(conversationID string) ([]agent.ChatMessage, error) {
	// Get saved ReAct input and output
	reactInputJSON, reactOutput, err := h.db.GetReActData(conversationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get ReAct data: %w", err)
	}

	// If last_react_input is empty, fall back to using message table (consistent with attack chain generation logic)
	if reactInputJSON == "" {
		return nil, fmt.Errorf("ReAct data is empty, will use message table")
	}

	dataSource := "database_last_react_input"

	// Parse JSON-format messages array
	var messagesArray []map[string]interface{}
	if err := json.Unmarshal([]byte(reactInputJSON), &messagesArray); err != nil {
		return nil, fmt.Errorf("failed to parse ReAct input JSON: %w", err)
	}

	messageCount := len(messagesArray)

	h.logger.Info("Restoring history context from saved ReAct data",
		zap.String("conversationId", conversationID),
		zap.String("dataSource", dataSource),
		zap.Int("reactInputSize", len(reactInputJSON)),
		zap.Int("messageCount", messageCount),
		zap.Int("reactOutputSize", len(reactOutput)),
	)
	// fmt.Println("messagesArray:", messagesArray)//debug

	// Convert to Agent message format
	agentMessages := make([]agent.ChatMessage, 0, len(messagesArray))
	for _, msgMap := range messagesArray {
		msg := agent.ChatMessage{}

		// Parse role
		if role, ok := msgMap["role"].(string); ok {
			msg.Role = role
		} else {
			continue // skip invalid messages
		}

		// Skip system messages (AgentLoop will re-add them)
		if msg.Role == "system" {
			continue
		}

		// Parse content
		if content, ok := msgMap["content"].(string); ok {
			msg.Content = content
		}

		// Parse tool_calls (if present)
		if toolCallsRaw, ok := msgMap["tool_calls"]; ok && toolCallsRaw != nil {
			if toolCallsArray, ok := toolCallsRaw.([]interface{}); ok {
				msg.ToolCalls = make([]agent.ToolCall, 0, len(toolCallsArray))
				for _, tcRaw := range toolCallsArray {
					if tcMap, ok := tcRaw.(map[string]interface{}); ok {
						toolCall := agent.ToolCall{}

						// Parse ID
						if id, ok := tcMap["id"].(string); ok {
							toolCall.ID = id
						}

						// Parse Type
						if toolType, ok := tcMap["type"].(string); ok {
							toolCall.Type = toolType
						}

						// Parse Function
						if funcMap, ok := tcMap["function"].(map[string]interface{}); ok {
							toolCall.Function = agent.FunctionCall{}

							// Parse function name
							if name, ok := funcMap["name"].(string); ok {
								toolCall.Function.Name = name
							}

							// Parse arguments (may be string or object)
							if argsRaw, ok := funcMap["arguments"]; ok {
								if argsStr, ok := argsRaw.(string); ok {
									// If it's a string, parse as JSON
									var argsMap map[string]interface{}
									if err := json.Unmarshal([]byte(argsStr), &argsMap); err == nil {
										toolCall.Function.Arguments = argsMap
									}
								} else if argsMap, ok := argsRaw.(map[string]interface{}); ok {
									// If already an object, use directly
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

		// Parse tool_call_id (tool role messages)
		if toolCallID, ok := msgMap["tool_call_id"].(string); ok {
			msg.ToolCallID = toolCallID
		}

		agentMessages = append(agentMessages, msg)
	}

	// If last_react_output exists, it needs to be added as the last assistant message
	// Because last_react_input is saved before the iteration starts, it does not include the final output of the last round
	if reactOutput != "" {
		// Check if the last message is an assistant message without tool_calls
		// If it has tool_calls, there should still be tool messages and a final assistant reply after it
		if len(agentMessages) > 0 {
			lastMsg := &agentMessages[len(agentMessages)-1]
			if strings.EqualFold(lastMsg.Role, "assistant") && len(lastMsg.ToolCalls) == 0 {
				// Last message is assistant message without tool_calls, update its content with final output
				lastMsg.Content = reactOutput
			} else {
				// Last message is not assistant message, or has tool_calls, add final output as new assistant message
				agentMessages = append(agentMessages, agent.ChatMessage{
					Role:    "assistant",
					Content: reactOutput,
				})
			}
		} else {
			// If no messages, directly add final output
			agentMessages = append(agentMessages, agent.ChatMessage{
				Role:    "assistant",
				Content: reactOutput,
			})
		}
	}

	if len(agentMessages) == 0 {
		return nil, fmt.Errorf("messages parsed from ReAct data are empty")
	}

	// Fix potentially mismatched tool messages, to avoid OpenAI errors
	// This prevents "messages with role 'tool' must be a response to a preceeding message with 'tool_calls'" errors
	if h.agent != nil {
		if fixed := h.agent.RepairOrphanToolMessages(&agentMessages); fixed {
			h.logger.Info("Fixed mismatched tool messages in history restored from ReAct data",
				zap.String("conversationId", conversationID),
			)
		}
	}

	h.logger.Info("History messages restored from ReAct data",
		zap.String("conversationId", conversationID),
		zap.String("dataSource", dataSource),
		zap.Int("originalMessageCount", messageCount),
		zap.Int("finalMessageCount", len(agentMessages)),
		zap.Bool("hasReactOutput", reactOutput != ""),
	)
	fmt.Println("agentMessages:", agentMessages) //debug
	return agentMessages, nil
}
