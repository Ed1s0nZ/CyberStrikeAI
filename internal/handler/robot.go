package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/database"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	robotCmdHelp        = "help"
	robotCmdList        = "list"
	robotCmdListAlt     = "conversationlist"
	robotCmdSwitch      = "switch"
	robotCmdContinue    = "continue"
	robotCmdNew = "conversation"
	robotCmdClear       = "clear"
	robotCmdCurrent     = "current"
	robotCmdStop        = "stop"
	robotCmdRoles       = "role"
	robotCmdRolesList   = "rolelist"
	robotCmdSwitchRole  = "switchrole"
	robotCmdDelete      = "delete"
	robotCmdVersion     = "version"
)

// RobotHandler handles robot message callbacks (Telegram).
type RobotHandler struct {
	config         *config.Config
	db             *database.DB
	agentHandler   *AgentHandler
	logger         *zap.Logger
	mu             sync.RWMutex
	sessions       map[string]string             // key: "platform_userID", value: conversationID
	sessionRoles   map[string]string             // key: "platform_userID", value: roleName (default "default")
	cancelMu       sync.Mutex                    // guards runningCancels
	runningCancels map[string]context.CancelFunc // key: "platform_userID", stop
}

// NewRobotHandler creates the robot handler.
func NewRobotHandler(cfg *config.Config, db *database.DB, agentHandler *AgentHandler, logger *zap.Logger) *RobotHandler {
	return &RobotHandler{
		config:         cfg,
		db:             db,
		agentHandler:   agentHandler,
		logger:         logger,
		sessions:       make(map[string]string),
		sessionRoles:   make(map[string]string),
		runningCancels: make(map[string]context.CancelFunc),
	}
}

// sessionKey builds a session key.
func (h *RobotHandler) sessionKey(platform, userID string) string {
	return platform + "_" + userID
}

// getOrCreateConversation returns the current conversation or creates a new one.
func (h *RobotHandler) getOrCreateConversation(platform, userID, title string) (convID string, isNew bool) {
	h.mu.RLock()
	convID = h.sessions[h.sessionKey(platform, userID)]
	h.mu.RUnlock()
	if convID != "" {
		return convID, false
	}
	t := strings.TrimSpace(title)
	if t == "" {
		t = "conversation " + time.Now().Format("01-02 15:04")
	} else {
		t = safeTruncateString(t, 50)
	}
	conv, err := h.db.CreateConversation(t)
	if err != nil {
		h.logger.Warn("", zap.Error(err))
		return "", false
	}
	convID = conv.ID
	h.mu.Lock()
	h.sessions[h.sessionKey(platform, userID)] = convID
	h.mu.Unlock()
	return convID, true
}

// setConversation switches the current conversation.
func (h *RobotHandler) setConversation(platform, userID, convID string) {
	h.mu.Lock()
	h.sessions[h.sessionKey(platform, userID)] = convID
	h.mu.Unlock()
}

// getRole returns the current role (defaults to "default").
func (h *RobotHandler) getRole(platform, userID string) string {
	h.mu.RLock()
	role := h.sessionRoles[h.sessionKey(platform, userID)]
	h.mu.RUnlock()
	if role == "" {
		return "default"
	}
	return role
}

// setRole sets the current role.
func (h *RobotHandler) setRole(platform, userID, roleName string) {
	h.mu.Lock()
	h.sessionRoles[h.sessionKey(platform, userID)] = roleName
	h.mu.Unlock()
}

// clearConversation clears the current conversation (creates a new one).
func (h *RobotHandler) clearConversation(platform, userID string) (newConvID string) {
	title := "conversation " + time.Now().Format("01-02 15:04")
	conv, err := h.db.CreateConversation(title)
	if err != nil {
		h.logger.Warn("conversation", zap.Error(err))
		return ""
	}
	h.setConversation(platform, userID, conv.ID)
	return conv.ID
}

// HandleMessage processes a message and returns a reply (used by Telegram bot and test endpoint).
func (h *RobotHandler) HandleMessage(platform, userID, text string) (reply string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "please enter content or send \"help\" to view commands."
	}

	// handle built-in commands
	if cmdReply, ok := h.handleRobotCommand(platform, userID, text); ok {
		return cmdReply
	}

	// agent message: send to Agent
	convID, _ := h.getOrCreateConversation(platform, userID, text)
	if convID == "" {
		return "failed to create conversation."
	}
	// update conversation title if it's the default format
	if conv, err := h.db.GetConversation(convID); err == nil && strings.HasPrefix(conv.Title, "conversation ") {
		newTitle := safeTruncateString(text, 50)
		if newTitle != "" {
			_ = h.db.UpdateConversationTitle(convID, newTitle)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	sk := h.sessionKey(platform, userID)
	h.cancelMu.Lock()
	h.runningCancels[sk] = cancel
	h.cancelMu.Unlock()
	defer func() {
		cancel()
		h.cancelMu.Lock()
		delete(h.runningCancels, sk)
		h.cancelMu.Unlock()
	}()
	role := h.getRole(platform, userID)
	resp, newConvID, err := h.agentHandler.ProcessMessageForRobot(ctx, convID, text, role)
	if err != nil {
		h.logger.Warn("Agent execution failed", zap.String("platform", platform), zap.String("userID", userID), zap.Error(err))
		if errors.Is(err, context.Canceled) {
			return "task cancelled."
		}
		return "processing failed: " + err.Error()
	}
	if newConvID != convID {
		h.setConversation(platform, userID, newConvID)
	}
	return resp
}

func (h *RobotHandler) cmdHelp() string {
	return "**CyberStrikeAI Commands**\n\n" +
		"- `help` -- Show this help\n" +
		"- `list` -- List conversations\n" +
		"- `switch <ID>` -- Switch to conversation\n" +
		"- `new` -- Start new conversation\n" +
		"- `clear` -- Clear context\n" +
		"- `current` -- Show current conversation\n" +
		"- `stop` -- Stop running task\n" +
		"- `roles` -- List roles\n" +
		"- `role <name>` -- Switch role\n" +
		"- `delete <ID>` -- Delete conversation\n" +
		"- `version` -- Show version\n\n" +
		"---\n" +
		"Otherwise, send any text for AI penetration testing / security analysis."
}

func (h *RobotHandler) cmdList() string {
	convs, err := h.db.ListConversations(50, 0, "")
	if err != nil {
		return "failed to list conversations: " + err.Error()
	}
	if len(convs) == 0 {
		return "No conversations. Send a message to start one."
	}
	var b strings.Builder
	b.WriteString("Conversations:\n")
	for i, c := range convs {
		if i >= 20 {
			b.WriteString("... showing first 20\n")
			break
		}
		b.WriteString(fmt.Sprintf("- %s\n  ID: %s\n", c.Title, c.ID))
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func (h *RobotHandler) cmdSwitch(platform, userID, convID string) string {
	if convID == "" {
		return "Please provide a conversation ID: switch xxx-xxx-xxx"
	}
	conv, err := h.db.GetConversation(convID)
	if err != nil {
		return "Conversation ID not found."
	}
	h.setConversation(platform, userID, conv.ID)
	return fmt.Sprintf("Switched to: \"%s\"\nID: %s", conv.Title, conv.ID)
}

func (h *RobotHandler) cmdNew(platform, userID string) string {
	newID := h.clearConversation(platform, userID)
	if newID == "" {
		return "Failed to create conversation."
	}
	return "New conversation started."
}

func (h *RobotHandler) cmdClear(platform, userID string) string {
	return h.cmdNew(platform, userID)
}

func (h *RobotHandler) cmdStop(platform, userID string) string {
	sk := h.sessionKey(platform, userID)
	h.cancelMu.Lock()
	cancel, ok := h.runningCancels[sk]
	if ok {
		delete(h.runningCancels, sk)
		cancel()
	}
	h.cancelMu.Unlock()
	if !ok {
		return "No running task."
	}
	return "Task stopped."
}

func (h *RobotHandler) cmdCurrent(platform, userID string) string {
	h.mu.RLock()
	convID := h.sessions[h.sessionKey(platform, userID)]
	h.mu.RUnlock()
	if convID == "" {
		return "No current conversation. Send a message to start one."
	}
	conv, err := h.db.GetConversation(convID)
	if err != nil {
		return "Current conversation ID: " + convID + " (title unavailable)"
	}
	role := h.getRole(platform, userID)
	return fmt.Sprintf("Current: \"%s\"\nID: %s\nRole: %s", conv.Title, conv.ID, role)
}

func (h *RobotHandler) cmdRoles() string {
	if h.config.Roles == nil || len(h.config.Roles) == 0 {
		return "No roles configured."
	}
	names := make([]string, 0, len(h.config.Roles))
	for name, role := range h.config.Roles {
		if role.Enabled {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return "No enabled roles."
	}
	sort.Slice(names, func(i, j int) bool {
		if names[i] == "default" {
			return true
		}
		if names[j] == "default" {
			return false
		}
		return names[i] < names[j]
	})
	var b strings.Builder
	b.WriteString("Roles:\n")
	for _, name := range names {
		role := h.config.Roles[name]
		desc := role.Description
		if desc == "" {
			desc = "no description"
		}
		b.WriteString(fmt.Sprintf("- %s -- %s\n", name, desc))
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func (h *RobotHandler) cmdSwitchRole(platform, userID, roleName string) string {
	if roleName == "" {
		return "Please specify a role name: role <name>"
	}
	if h.config.Roles == nil {
		return "No roles configured."
	}
	role, exists := h.config.Roles[roleName]
	if !exists {
		return fmt.Sprintf("Role \"%s\" not found. Send \"roles\" to list.", roleName)
	}
	if !role.Enabled {
		return fmt.Sprintf("Role \"%s\" is disabled.", roleName)
	}
	h.setRole(platform, userID, roleName)
	return fmt.Sprintf("Switched to role: \"%s\"\n%s", roleName, role.Description)
}

func (h *RobotHandler) cmdDelete(platform, userID, convID string) string {
	if convID == "" {
		return "Please provide a conversation ID: delete xxx-xxx-xxx"
	}
	sk := h.sessionKey(platform, userID)
	h.mu.RLock()
	currentConvID := h.sessions[sk]
	h.mu.RUnlock()
	if convID == currentConvID {
		h.mu.Lock()
		delete(h.sessions, sk)
		h.mu.Unlock()
	}
	if err := h.db.DeleteConversation(convID); err != nil {
		return "Delete failed: " + err.Error()
	}
	return fmt.Sprintf("Deleted conversation ID: %s", convID)
}

func (h *RobotHandler) cmdVersion() string {
	v := h.config.Version
	if v == "" {
		v = "unknown"
	}
	return "CyberStrikeAI " + v
}

// handleRobotCommand handles robot built-in commands.
func (h *RobotHandler) handleRobotCommand(platform, userID, text string) (string, bool) {
	switch {
	case text == robotCmdHelp || text == "help" || text == "?" || text == "?":
		return h.cmdHelp(), true
	case text == robotCmdList || text == robotCmdListAlt || text == "list":
		return h.cmdList(), true
	case strings.HasPrefix(text, robotCmdSwitch+" ") || strings.HasPrefix(text, robotCmdContinue+" ") || strings.HasPrefix(text, "switch ") || strings.HasPrefix(text, "continue "):
		var id string
		switch {
		case strings.HasPrefix(text, robotCmdSwitch+" "):
			id = strings.TrimSpace(text[len(robotCmdSwitch)+1:])
		case strings.HasPrefix(text, robotCmdContinue+" "):
			id = strings.TrimSpace(text[len(robotCmdContinue)+1:])
		case strings.HasPrefix(text, "switch "):
			id = strings.TrimSpace(text[7:])
		default:
			id = strings.TrimSpace(text[9:])
		}
		return h.cmdSwitch(platform, userID, id), true
	case text == robotCmdNew || text == "new":
		return h.cmdNew(platform, userID), true
	case text == robotCmdClear || text == "clear":
		return h.cmdClear(platform, userID), true
	case text == robotCmdCurrent || text == "current":
		return h.cmdCurrent(platform, userID), true
	case text == robotCmdStop || text == "stop":
		return h.cmdStop(platform, userID), true
	case text == robotCmdRoles || text == robotCmdRolesList || text == "roles":
		return h.cmdRoles(), true
	case strings.HasPrefix(text, robotCmdRoles+" ") || strings.HasPrefix(text, robotCmdSwitchRole+" ") || strings.HasPrefix(text, "role "):
		var roleName string
		switch {
		case strings.HasPrefix(text, robotCmdRoles+" "):
			roleName = strings.TrimSpace(text[len(robotCmdRoles)+1:])
		case strings.HasPrefix(text, robotCmdSwitchRole+" "):
			roleName = strings.TrimSpace(text[len(robotCmdSwitchRole)+1:])
		default:
			roleName = strings.TrimSpace(text[5:])
		}
		return h.cmdSwitchRole(platform, userID, roleName), true
	case strings.HasPrefix(text, robotCmdDelete+" ") || strings.HasPrefix(text, "delete "):
		var convID string
		if strings.HasPrefix(text, robotCmdDelete+" ") {
			convID = strings.TrimSpace(text[len(robotCmdDelete)+1:])
		} else {
			convID = strings.TrimSpace(text[7:])
		}
		return h.cmdDelete(platform, userID, convID), true
	case text == robotCmdVersion || text == "version":
		return h.cmdVersion(), true
	default:
		return "", false
	}
}

// —————— test endpoint ——————

// RobotTestRequest is the test message request.
type RobotTestRequest struct {
	Platform string `json:"platform"` // "telegram", "test"
	UserID   string `json:"user_id"`
	Text     string `json:"text"`
}

// HandleRobotTest handles local verification: POST JSON { "platform", "user_id", "text" }, returns { "reply": "..." }
func (h *RobotHandler) HandleRobotTest(c *gin.Context) {
	var req RobotTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "request body must be JSON with platform, user_id, text"})
		return
	}
	platform := strings.TrimSpace(req.Platform)
	if platform == "" {
		platform = "test"
	}
	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		userID = "test_user"
	}
	reply := h.HandleMessage(platform, userID, req.Text)
	c.JSON(http.StatusOK, gin.H{"reply": reply})
}
