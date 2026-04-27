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
	robotCmdHelp       = "help"
	robotCmdList       = "list"
	robotCmdListAlt    = "conversationlist"
	robotCmdSwitch     = "switch"
	robotCmdContinue   = "continue"
	robotCmdNew        = "conversation"
	robotCmdClear      = "clear"
	robotCmdCurrent    = "current"
	robotCmdStop       = "stop"
	robotCmdRoles      = "role"
	robotCmdRolesList  = "rolelist"
	robotCmdSwitchRole = "switchrole"
	robotCmdDelete     = "delete"
	robotCmdVersion    = "version"
	robotCmdMode       = "mode"
)

// RobotHandler handles robot message callbacks (Telegram).
type RobotHandler struct {
	config         *config.Config
	db             *database.DB
	agentHandler   *AgentHandler
	logger         *zap.Logger
	mu             sync.RWMutex                  // guards sessionRoles
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
		sessionRoles:   make(map[string]string),
		runningCancels: make(map[string]context.CancelFunc),
	}
}

// sessionKey builds a session key.
func (h *RobotHandler) sessionKey(platform, userID string) string {
	return platform + "_" + userID
}

// getOrCreateConversation returns the current conversation (DB-backed),
// or creates a fresh one tagged with the given platform.
func (h *RobotHandler) getOrCreateConversation(platform, userID, title string) (convID string, isNew bool) {
	sess, err := h.db.GetBotSession(platform, userID)
	if err != nil {
		h.logger.Warn("GetBotSession failed; falling back to fresh conversation",
			zap.String("platform", platform), zap.String("user_id", userID), zap.Error(err))
	}
	if sess != nil && sess.ConversationID != "" {
		return sess.ConversationID, false
	}
	t := strings.TrimSpace(title)
	if t == "" {
		t = "conversation " + time.Now().Format("01-02 15:04")
	} else {
		t = safeTruncateString(t, 50)
	}
	conv, err := h.db.CreateConversationWithPlatform(t, platform)
	if err != nil {
		h.logger.Error("CreateConversationWithPlatform failed", zap.Error(err))
		return "", true
	}
	mode := ""
	if sess != nil {
		mode = sess.CurrentMode // preserve mode override if session existed but conversation was deleted
	}
	if upErr := h.db.UpsertBotSession(platform, userID, conv.ID, mode); upErr != nil {
		h.logger.Warn("UpsertBotSession failed", zap.Error(upErr))
	}
	return conv.ID, true
}

// setConversation switches the current conversation persistently.
// Preserves any existing mode override.
func (h *RobotHandler) setConversation(platform, userID, convID string) {
	sess, _ := h.db.GetBotSession(platform, userID)
	mode := ""
	if sess != nil {
		mode = sess.CurrentMode
	}
	if err := h.db.UpsertBotSession(platform, userID, convID, mode); err != nil {
		h.logger.Warn("setConversation upsert failed", zap.Error(err))
	}
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

// clearConversation clears the current conversation. Next message
// triggers getOrCreateConversation to make a new one.
func (h *RobotHandler) clearConversation(platform, userID string) {
	if err := h.db.ClearBotSession(platform, userID); err != nil {
		h.logger.Warn("ClearBotSession failed", zap.Error(err))
	}
}

// HandleMessage is the synchronous wrapper for non-streaming clients
// (test endpoint, future non-Telegram bots).
func (h *RobotHandler) HandleMessage(platform, userID, text string) string {
	return h.handleInternal(platform, userID, text, nil)
}

// HandleMessageStream is the StreamingMessageHandler implementation.
// telegram.go invokes this when the streaming-handler interface is
// satisfied; the synchronous HandleMessage falls through to this
// with a no-op (nil) progressFn.
func (h *RobotHandler) HandleMessageStream(platform, userID, text string, progressFn func(step string)) string {
	return h.handleInternal(platform, userID, text, progressFn)
}

// handleInternal is the shared body used by both the streaming and
// non-streaming entry points. It dispatches commands, resolves session,
// and drives ProcessMessageForRobot. progressFn (nil-safe) is forwarded
// for the streaming path.
func (h *RobotHandler) handleInternal(platform, userID, text string, progressFn func(step string)) string {
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
	// resolve per-chat mode override before creating the timeout context
	var forceMode string
	if sess, _ := h.db.GetBotSession(platform, userID); sess != nil {
		forceMode = sess.CurrentMode
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
	// Per-conversation lock: prevent concurrent agent runs on the same
	// chat. Adapter wraps cancel() as CancelCauseFunc; the cause is
	// dropped because the bot reply doesn't surface it. Mirrors
	// multi_agent.go:121-141 / web-UI semantics.
	cancelWithCause := func(cause error) { cancel() }
	if _, err := h.agentHandler.tasks.StartTask(convID, text, cancelWithCause); err != nil {
		if errors.Is(err, ErrTaskAlreadyRunning) {
			return "Another task is running for this chat. Say `stop` to cancel."
		}
		return "Failed to start task: " + err.Error()
	}
	taskStatus := "completed"
	defer func() { h.agentHandler.tasks.FinishTask(convID, taskStatus) }()
	role := h.getRole(platform, userID)
	resp, newConvID, err := h.agentHandler.ProcessMessageForRobot(ctx, convID, text, role, forceMode, progressFn)
	if err != nil {
		taskStatus = "failed"
		h.logger.Warn("Agent execution failed", zap.String("platform", platform), zap.String("userID", userID), zap.Error(err))
		if errors.Is(err, context.Canceled) {
			taskStatus = "cancelled"
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
		"- `mode <single|multi|default>` -- Set per-chat agent mode\n" +
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
	h.clearConversation(platform, userID)
	return "New conversation started. Send a message to begin."
}

func (h *RobotHandler) cmdClear(platform, userID string) string {
	h.clearConversation(platform, userID)
	return "Conversation cleared. Send a message to begin."
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
	sess, _ := h.db.GetBotSession(platform, userID)
	var convID string
	if sess != nil {
		convID = sess.ConversationID
	}
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
	sess, _ := h.db.GetBotSession(platform, userID)
	var currentConvID string
	if sess != nil {
		currentConvID = sess.ConversationID
	}
	if convID == currentConvID {
		if err := h.db.ClearBotSession(platform, userID); err != nil {
			h.logger.Warn("ClearBotSession on delete failed", zap.Error(err))
		}
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
	case text == robotCmdHelp || text == "help" || text == "?":
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
	case text == robotCmdMode:
		return h.cmdMode(platform, userID, ""), true
	case strings.HasPrefix(text, robotCmdMode+" "):
		arg := strings.TrimSpace(text[len(robotCmdMode)+1:])
		return h.cmdMode(platform, userID, arg), true
	default:
		return "", false
	}
}

// cmdMode handles the `mode <single|multi|default>` slash command.
// Returns the reply string to send to the user. Does not invoke the
// agent loop — pure session-state mutation.
func (h *RobotHandler) cmdMode(platform, userID string, arg string) string {
	switch arg {
	case "":
		sess, _ := h.db.GetBotSession(platform, userID)
		override := ""
		if sess != nil {
			override = sess.CurrentMode
		}
		globalMulti := h.config != nil && h.config.MultiAgent.Enabled && h.config.MultiAgent.RobotUseMultiAgent
		effective := "single"
		if override == "multi" || (override == "" && globalMulti) {
			effective = "multi"
		}
		if override == "" {
			return fmt.Sprintf("Current mode: %s (inheriting global default).\nUse `mode single`, `mode multi`, or `mode default`.", effective)
		}
		return fmt.Sprintf("Current mode: %s (per-chat override).\nUse `mode default` to revert.", effective)

	case "single":
		if err := h.db.SetBotMode(platform, userID, "single"); err != nil {
			return "Failed to set mode: " + err.Error()
		}
		return "This chat is now single-agent."

	case "multi":
		if h.config == nil || !h.config.MultiAgent.Enabled {
			return "Multi-agent is disabled in this deployment. Ask the operator to enable it (config: multi_agent.enabled)."
		}
		if err := h.db.SetBotMode(platform, userID, "multi"); err != nil {
			return "Failed to set mode: " + err.Error()
		}
		return "This chat is now multi-agent."

	case "default":
		if err := h.db.SetBotMode(platform, userID, ""); err != nil {
			return "Failed to revert mode: " + err.Error()
		}
		globalMulti := h.config != nil && h.config.MultiAgent.Enabled && h.config.MultiAgent.RobotUseMultiAgent
		fallbackName := "single"
		if globalMulti {
			fallbackName = "multi"
		}
		return fmt.Sprintf("Reverted to global default (%s).", fallbackName)

	default:
		return fmt.Sprintf("Unknown mode '%s'. Use: mode single | mode multi | mode default", arg)
	}
}

// ------ test endpoint ------

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
