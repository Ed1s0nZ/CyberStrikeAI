package handler

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
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
	robotCmdListAlt     = "conversations"
	robotCmdSwitch      = "switch"
	robotCmdContinue    = "continue"
	robotCmdNew         = "new"
	robotCmdClear       = "clear"
	robotCmdCurrent     = "current"
	robotCmdStop        = "stop"
	robotCmdRoles       = "roles"
	robotCmdRolesList   = "role-list"
	robotCmdSwitchRole  = "role"
	robotCmdDelete      = "delete"
	robotCmdVersion     = "version"
)

// RobotHandler handles bot callbacks for WeCom, DingTalk, Lark, etc.
type RobotHandler struct {
	config         *config.Config
	db             *database.DB
	agentHandler   *AgentHandler
	logger         *zap.Logger
	mu             sync.RWMutex
	sessions       map[string]string             // key: "platform_userID", value: conversationID
	sessionRoles   map[string]string             // key: "platform_userID", value: roleName (defaults to "Default")
	cancelMu       sync.Mutex                    // protects runningCancels
	runningCancels map[string]context.CancelFunc // key: "platform_userID", used by the stop command to cancel running tasks
}

// NewRobotHandler creates a new bot handler.
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

// sessionKey generates a session key.
func (h *RobotHandler) sessionKey(platform, userID string) string {
	return platform + "_" + userID
}

// getOrCreateConversation gets or creates the current session; title is used as the new conversation title (first 50 chars of the user message).
func (h *RobotHandler) getOrCreateConversation(platform, userID, title string) (convID string, isNew bool) {
	h.mu.RLock()
	convID = h.sessions[h.sessionKey(platform, userID)]
	h.mu.RUnlock()
	if convID != "" {
		return convID, false
	}
	t := strings.TrimSpace(title)
	if t == "" {
		t = "New Conversation " + time.Now().Format("01-02 15:04")
	} else {
		t = safeTruncateString(t, 50)
	}
	conv, err := h.db.CreateConversation(t)
	if err != nil {
		h.logger.Warn("failed to create bot session", zap.Error(err))
		return "", false
	}
	convID = conv.ID
	h.mu.Lock()
	h.sessions[h.sessionKey(platform, userID)] = convID
	h.mu.Unlock()
	return convID, true
}

// setConversation switches the current session.
func (h *RobotHandler) setConversation(platform, userID, convID string) {
	h.mu.Lock()
	h.sessions[h.sessionKey(platform, userID)] = convID
	h.mu.Unlock()
}

// getRole returns the role currently used by the user; returns "Default" if not set.
func (h *RobotHandler) getRole(platform, userID string) string {
	h.mu.RLock()
	role := h.sessionRoles[h.sessionKey(platform, userID)]
	h.mu.RUnlock()
	if role == "" {
		return "Default"
	}
	return role
}

// setRole sets the role for the current user.
func (h *RobotHandler) setRole(platform, userID, roleName string) {
	h.mu.Lock()
	h.sessionRoles[h.sessionKey(platform, userID)] = roleName
	h.mu.Unlock()
}

// clearConversation clears the current session by switching to a new conversation.
func (h *RobotHandler) clearConversation(platform, userID string) (newConvID string) {
	title := "New Conversation " + time.Now().Format("01-02 15:04")
	conv, err := h.db.CreateConversation(title)
	if err != nil {
		h.logger.Warn("failed to create new conversation", zap.Error(err))
		return ""
	}
	h.setConversation(platform, userID, conv.ID)
	return conv.ID
}

// HandleMessageStream processes user input with streaming progress support.
// progressFn is called during agent execution with short step descriptions (e.g. "calling tool: nmap").
// For instant commands (help, list, etc.) it behaves identically to HandleMessage.
// This method satisfies the robot.StreamingMessageHandler interface.
func (h *RobotHandler) HandleMessageStream(platform, userID, text string, progressFn func(step string)) (reply string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "Please enter a message, or send \"help\" to view available commands."
	}

	// Dispatch commands immediately (no streaming needed)
	switch {
	case text == robotCmdHelp || text == "？" || text == "?":
		return h.cmdHelp()
	case text == robotCmdList || text == robotCmdListAlt || text == "list":
		return h.cmdList()
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
		return h.cmdSwitch(platform, userID, id)
	case text == robotCmdNew || text == "new":
		return h.cmdNew(platform, userID)
	case text == robotCmdClear || text == "clear":
		return h.cmdClear(platform, userID)
	case text == robotCmdCurrent || text == "current":
		return h.cmdCurrent(platform, userID)
	case text == robotCmdStop || text == "stop":
		return h.cmdStop(platform, userID)
	case text == robotCmdRoles || text == robotCmdRolesList || text == "roles":
		return h.cmdRoles()
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
		return h.cmdSwitchRole(platform, userID, roleName)
	case strings.HasPrefix(text, robotCmdDelete+" ") || strings.HasPrefix(text, "delete "):
		var convID string
		if strings.HasPrefix(text, robotCmdDelete+" ") {
			convID = strings.TrimSpace(text[len(robotCmdDelete)+1:])
		} else {
			convID = strings.TrimSpace(text[7:])
		}
		return h.cmdDelete(platform, userID, convID)
	case text == robotCmdVersion || text == "version":
		return h.cmdVersion()
	}

	// Regular message: send to Agent with streaming progress
	convID, _ := h.getOrCreateConversation(platform, userID, text)
	if convID == "" {
		return "Unable to create or retrieve conversation. Please try again later."
	}
	if conv, err := h.db.GetConversation(convID); err == nil && strings.HasPrefix(conv.Title, "New Conversation ") {
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
	resp, newConvID, err := h.agentHandler.ProcessMessageForRobotStream(ctx, convID, text, role, progressFn)
	if err != nil {
		h.logger.Warn("bot agent streaming execution failed", zap.String("platform", platform), zap.String("userID", userID), zap.Error(err))
		if errors.Is(err, context.Canceled) {
			return "Task cancelled."
		}
		return "Processing failed: " + err.Error()
	}
	if newConvID != convID {
		h.setConversation(platform, userID, newConvID)
	}
	return resp
}

// HandleMessage processes user input and returns a reply string (called by each platform webhook).
func (h *RobotHandler) HandleMessage(platform, userID, text string) (reply string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "Please enter a message, or send \"help\" to view available commands."
	}

	// Command dispatch
	switch {
	case text == robotCmdHelp || text == "？" || text == "?":
		return h.cmdHelp()
	case text == robotCmdList || text == robotCmdListAlt || text == "list":
		return h.cmdList()
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
		return h.cmdSwitch(platform, userID, id)
	case text == robotCmdNew || text == "new":
		return h.cmdNew(platform, userID)
	case text == robotCmdClear || text == "clear":
		return h.cmdClear(platform, userID)
	case text == robotCmdCurrent || text == "current":
		return h.cmdCurrent(platform, userID)
	case text == robotCmdStop || text == "stop":
		return h.cmdStop(platform, userID)
	case text == robotCmdRoles || text == robotCmdRolesList || text == "roles":
		return h.cmdRoles()
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
		return h.cmdSwitchRole(platform, userID, roleName)
	case strings.HasPrefix(text, robotCmdDelete+" ") || strings.HasPrefix(text, "delete "):
		var convID string
		if strings.HasPrefix(text, robotCmdDelete+" ") {
			convID = strings.TrimSpace(text[len(robotCmdDelete)+1:])
		} else {
			convID = strings.TrimSpace(text[7:])
		}
		return h.cmdDelete(platform, userID, convID)
	case text == robotCmdVersion || text == "version":
		return h.cmdVersion()
	}

	// Regular message: send to Agent
	convID, _ := h.getOrCreateConversation(platform, userID, text)
	if convID == "" {
		return "Unable to create or retrieve conversation. Please try again later."
	}
	// If the conversation title is in "New Conversation xx:xx" format (created by the "new" command), update the title to the first message content, consistent with the web UI experience.
	if conv, err := h.db.GetConversation(convID); err == nil && strings.HasPrefix(conv.Title, "New Conversation ") {
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
		h.logger.Warn("bot agent execution failed", zap.String("platform", platform), zap.String("userID", userID), zap.Error(err))
		if errors.Is(err, context.Canceled) {
			return "Task cancelled."
		}
		return "Processing failed: " + err.Error()
	}
	if newConvID != convID {
		h.setConversation(platform, userID, newConvID)
	}
	return resp
}

func (h *RobotHandler) cmdHelp() string {
	return "**[CyberStrikeAI Bot Commands]**\n\n" +
		"- `help` — Show this help\n" +
		"- `list` / `conversations` — List all conversations\n" +
		"- `switch <ID>` / `continue <ID>` — Switch to a conversation\n" +
		"- `new` — Start a new conversation\n" +
		"- `clear` — Clear the current context\n" +
		"- `current` — Show the current conversation ID and title\n" +
		"- `stop` — Stop the running task\n" +
		"- `roles` — List all available roles\n" +
		"- `role <name>` — Switch to the specified role\n" +
		"- `delete <ID>` — Delete a conversation\n" +
		"- `version` — Show the current version\n\n" +
		"---\n" +
		"Any other input is sent directly to the AI for penetration testing / security analysis."
}

func (h *RobotHandler) cmdList() string {
	convs, err := h.db.ListConversations(50, 0, "")
	if err != nil {
		return "Failed to retrieve conversation list: " + err.Error()
	}
	if len(convs) == 0 {
		return "No conversations yet. Send any message to create one automatically."
	}
	var b strings.Builder
	b.WriteString("[Conversation List]\n")
	for i, c := range convs {
		if i >= 20 {
			b.WriteString("… Showing first 20 only\n")
			break
		}
		b.WriteString(fmt.Sprintf("· %s\n  ID: %s\n", c.Title, c.ID))
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func (h *RobotHandler) cmdSwitch(platform, userID, convID string) string {
	if convID == "" {
		return "Please specify a conversation ID, e.g.: switch xxx-xxx-xxx"
	}
	conv, err := h.db.GetConversation(convID)
	if err != nil {
		return "Conversation not found or invalid ID."
	}
	h.setConversation(platform, userID, conv.ID)
	return fmt.Sprintf("Switched to conversation: \"%s\"\nID: %s", conv.Title, conv.ID)
}

func (h *RobotHandler) cmdNew(platform, userID string) string {
	newID := h.clearConversation(platform, userID)
	if newID == "" {
		return "Failed to create new conversation. Please try again."
	}
	return "New conversation started. You can now send messages."
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
		return "No task is currently running."
	}
	return "Current task stopped."
}

func (h *RobotHandler) cmdCurrent(platform, userID string) string {
	h.mu.RLock()
	convID := h.sessions[h.sessionKey(platform, userID)]
	h.mu.RUnlock()
	if convID == "" {
		return "No active conversation. Send any message to create one."
	}
	conv, err := h.db.GetConversation(convID)
	if err != nil {
		return "Current conversation ID: " + convID + " (failed to retrieve title)"
	}
	role := h.getRole(platform, userID)
	return fmt.Sprintf("Current conversation: \"%s\"\nID: %s\nCurrent role: %s", conv.Title, conv.ID, role)
}

func (h *RobotHandler) cmdRoles() string {
	if h.config.Roles == nil || len(h.config.Roles) == 0 {
		return "No roles available."
	}
	names := make([]string, 0, len(h.config.Roles))
	for name, role := range h.config.Roles {
		if role.Enabled {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return "No roles available."
	}
	sort.Slice(names, func(i, j int) bool {
		if names[i] == "Default" {
			return true
		}
		if names[j] == "Default" {
			return false
		}
		return names[i] < names[j]
	})
	var b strings.Builder
	b.WriteString("[Role List]\n")
	for _, name := range names {
		role := h.config.Roles[name]
		desc := role.Description
		if desc == "" {
			desc = "No description"
		}
		b.WriteString(fmt.Sprintf("· %s — %s\n", name, desc))
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func (h *RobotHandler) cmdSwitchRole(platform, userID, roleName string) string {
	if roleName == "" {
		return "Please specify a role name, e.g.: role Penetration Testing"
	}
	if h.config.Roles == nil {
		return "No roles available."
	}
	role, exists := h.config.Roles[roleName]
	if !exists {
		return fmt.Sprintf("Role \"%s\" does not exist. Send \"roles\" to see available roles.", roleName)
	}
	if !role.Enabled {
		return fmt.Sprintf("Role \"%s\" is disabled.", roleName)
	}
	h.setRole(platform, userID, roleName)
	return fmt.Sprintf("Switched to role: \"%s\"\n%s", roleName, role.Description)
}

func (h *RobotHandler) cmdDelete(platform, userID, convID string) string {
	if convID == "" {
		return "Please specify a conversation ID, e.g.: delete xxx-xxx-xxx"
	}
	sk := h.sessionKey(platform, userID)
	h.mu.RLock()
	currentConvID := h.sessions[sk]
	h.mu.RUnlock()
	if convID == currentConvID {
		// When deleting the current conversation, first clear the session binding
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

// —————— WeCom (Enterprise WeChat) ——————

// wecomXML is the WeCom callback XML structure (simplified for plaintext mode; encrypted mode requires decryption before parsing).
type wecomXML struct {
	ToUserName   string `xml:"ToUserName"`
	FromUserName string `xml:"FromUserName"`
	CreateTime   int64  `xml:"CreateTime"`
	MsgType      string `xml:"MsgType"`
	Content      string `xml:"Content"`
	MsgID        string `xml:"MsgId"`
	AgentID      int64  `xml:"AgentID"`
	Encrypt      string `xml:"Encrypt"` // in encrypted mode the message is here
}

// wecomReplyXML is the passive reply XML structure.
type wecomReplyXML struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName"`
	FromUserName string  `xml:"FromUserName"`
	CreateTime   int64   `xml:"CreateTime"`
	MsgType      string  `xml:"MsgType"`
	Content      string  `xml:"Content"`
}

// HandleWecomGET handles WeCom URL verification (GET).
func (h *RobotHandler) HandleWecomGET(c *gin.Context) {
	if !h.config.Robots.Wecom.Enabled {
		c.String(http.StatusNotFound, "")
		return
	}
	echostr := c.Query("echostr")
	if echostr == "" {
		c.String(http.StatusBadRequest, "missing echostr")
		return
	}
	// In plaintext mode, WeCom may pass echostr directly; return it immediately to pass verification.
	c.String(http.StatusOK, echostr)
}

// wecomDecrypt decrypts a WeCom message (AES-256-CBC, PKCS7; plaintext format: 16-byte random + 4-byte length + message + corpID).
func wecomDecrypt(encodingAESKey, encryptedB64 string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(encodingAESKey + "=")
	if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("encoding_aes_key must decode to 32 bytes")
	}
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedB64)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	iv := key[:16]
	mode := cipher.NewCBCDecrypter(block, iv)
	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("ciphertext length is not a multiple of the block size")
	}
	plain := make([]byte, len(ciphertext))
	mode.CryptBlocks(plain, ciphertext)
	// Remove PKCS7 padding
	n := int(plain[len(plain)-1])
	if n < 1 || n > 32 {
		return nil, fmt.Errorf("invalid PKCS7 padding")
	}
	plain = plain[:len(plain)-n]
	// WeCom format: 16-byte random + 4-byte length (big-endian) + message + corpID
	if len(plain) < 20 {
		return nil, fmt.Errorf("plaintext too short")
	}
	msgLen := binary.BigEndian.Uint32(plain[16:20])
	if int(20+msgLen) > len(plain) {
		return nil, fmt.Errorf("message length out of bounds")
	}
	return plain[20 : 20+msgLen], nil
}

// HandleWecomPOST handles WeCom message callbacks (POST), supporting both plaintext and encrypted modes.
func (h *RobotHandler) HandleWecomPOST(c *gin.Context) {
	if !h.config.Robots.Wecom.Enabled {
		c.String(http.StatusOK, "")
		return
	}
	bodyRaw, _ := io.ReadAll(c.Request.Body)
	var body wecomXML
	if err := xml.Unmarshal(bodyRaw, &body); err != nil {
		h.logger.Debug("WeCom POST: failed to parse XML", zap.Error(err))
		c.String(http.StatusOK, "")
		return
	}
	// Encrypted mode: decrypt first, then parse the inner XML
	if body.Encrypt != "" && h.config.Robots.Wecom.EncodingAESKey != "" {
		decrypted, err := wecomDecrypt(h.config.Robots.Wecom.EncodingAESKey, body.Encrypt)
		if err != nil {
			h.logger.Warn("WeCom message decryption failed", zap.Error(err))
			c.String(http.StatusOK, "")
			return
		}
		if err := xml.Unmarshal(decrypted, &body); err != nil {
			h.logger.Warn("WeCom: failed to parse decrypted XML", zap.Error(err))
			c.String(http.StatusOK, "")
			return
		}
	}
	if body.MsgType != "text" {
		c.XML(http.StatusOK, wecomReplyXML{
			ToUserName:   body.FromUserName,
			FromUserName: body.ToUserName,
			CreateTime:  time.Now().Unix(),
			MsgType:     "text",
			Content:     "Only text messages are supported. Please send a text message.",
		})
		return
	}
	userID := body.FromUserName
	text := strings.TrimSpace(body.Content)
	reply := h.HandleMessage("wecom", userID, text)
	// Encrypted mode requires encrypting the reply (simplified to plaintext here; implement encryption if the enterprise requires it).
	c.XML(http.StatusOK, wecomReplyXML{
		ToUserName:   body.FromUserName,
		FromUserName: body.ToUserName,
		CreateTime:  time.Now().Unix(),
		MsgType:     "text",
		Content:     reply,
	})
}

// —————— Test endpoint (requires login; used to verify bot logic without a DingTalk/Lark client) ——————

// RobotTestRequest simulates a bot message request.
type RobotTestRequest struct {
	Platform string `json:"platform"` // e.g. "dingtalk", "lark", "wecom"
	UserID   string `json:"user_id"`
	Text     string `json:"text"`
}

// HandleRobotTest is for local verification: POST JSON { "platform", "user_id", "text" } → returns { "reply": "..." }.
func (h *RobotHandler) HandleRobotTest(c *gin.Context) {
	var req RobotTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "request body must be JSON with platform, user_id, and text fields"})
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

// —————— DingTalk ——————

// HandleDingtalkPOST handles DingTalk event callbacks (Stream mode, etc.); currently a placeholder that returns 200.
func (h *RobotHandler) HandleDingtalkPOST(c *gin.Context) {
	if !h.config.Robots.Dingtalk.Enabled {
		c.JSON(http.StatusOK, gin.H{})
		return
	}
	// DingTalk Stream/event callback format must be parsed per the official docs and replied to asynchronously; returns 200 here for now.
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// —————— Lark (Feishu) ——————

// HandleLarkPOST handles Lark event callbacks; currently a placeholder that returns 200; verification requires returning the challenge.
func (h *RobotHandler) HandleLarkPOST(c *gin.Context) {
	if !h.config.Robots.Lark.Enabled {
		c.JSON(http.StatusOK, gin.H{})
		return
	}
	var body struct {
		Challenge string `json:"challenge"`
	}
	if err := c.ShouldBindJSON(&body); err == nil && body.Challenge != "" {
		c.JSON(http.StatusOK, gin.H{"challenge": body.Challenge})
		return
	}
	c.JSON(http.StatusOK, gin.H{})
}
