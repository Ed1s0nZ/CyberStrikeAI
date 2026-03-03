package robot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"cyberstrike-ai/internal/config"

	"go.uber.org/zap"
)

const (
	telegramReconnectInitial = 5 * time.Second  // initial reconnect interval
	telegramReconnectMax     = 60 * time.Second // maximum reconnect interval
	telegramLongPollTimeout  = 30               // long-poll timeout in seconds
	telegramMaxMessageLen    = 4096             // Telegram message character limit
	telegramEditThrottle     = 3 * time.Second  // minimum interval between message edits
	telegramAPIBase          = "https://api.telegram.org"
)

// ——— Telegram API types ———

type tgResponse struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result"`
	Description string          `json:"description"`
}

type tgUpdate struct {
	UpdateID int64      `json:"update_id"`
	Message  *tgMessage `json:"message"`
}

type tgMessage struct {
	MessageID int64      `json:"message_id"`
	From      *tgUser    `json:"from"`
	Chat      tgChat     `json:"chat"`
	Text      string     `json:"text"`
	Entities  []tgEntity `json:"entities"`
}

type tgUser struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
}

type tgChat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"` // "private", "group", "supergroup", "channel"
}

type tgEntity struct {
	Type   string `json:"type"`
	Offset int    `json:"offset"`
	Length int    `json:"length"`
}

type tgSentMessage struct {
	MessageID int64 `json:"message_id"`
	Chat      tgChat `json:"chat"`
}

// tgBot holds Telegram Bot API state for one polling session.
type tgBot struct {
	apiURL      string
	cfg         config.RobotTelegramConfig
	h           MessageHandler
	logger      *zap.Logger
	botUsername string // fetched via getMe
	allowedSet  map[int64]bool
}

// ——— Public entrypoint ———

// StartTelegram starts the Telegram bot using long polling (no public IP required).
// Automatically reconnects on errors; exits when ctx is cancelled.
func StartTelegram(ctx context.Context, cfg config.RobotTelegramConfig, h MessageHandler, logger *zap.Logger) {
	if !cfg.Enabled || cfg.BotToken == "" {
		return
	}
	go runTelegramLoop(ctx, cfg, h, logger)
}

// ——— Connection loop ———

func runTelegramLoop(ctx context.Context, cfg config.RobotTelegramConfig, h MessageHandler, logger *zap.Logger) {
	backoff := telegramReconnectInitial

	// Build allowed-user set once
	allowedSet := make(map[int64]bool, len(cfg.AllowedUserIDs))
	for _, id := range cfg.AllowedUserIDs {
		allowedSet[id] = true
	}

	for {
		bot := &tgBot{
			apiURL:     fmt.Sprintf("%s/bot%s", telegramAPIBase, cfg.BotToken),
			cfg:        cfg,
			h:          h,
			logger:     logger,
			allowedSet: allowedSet,
		}

		logger.Info("Telegram bot connecting...")
		err := bot.runPollLoop(ctx)

		if ctx.Err() != nil {
			logger.Info("Telegram bot stopped per configuration reload")
			return
		}
		if err != nil {
			logger.Warn("Telegram bot polling error, will reconnect", zap.Error(err), zap.Duration("retry_after", backoff))
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
			if backoff < telegramReconnectMax {
				backoff *= 2
				if backoff > telegramReconnectMax {
					backoff = telegramReconnectMax
				}
			}
		}
	}
}

// ——— Polling loop ———

func (b *tgBot) runPollLoop(ctx context.Context) error {
	// Fetch bot info (username) for group mention detection
	if err := b.fetchBotInfo(ctx); err != nil {
		return fmt.Errorf("getMe failed: %w", err)
	}
	b.logger.Info("Telegram bot started", zap.String("username", b.botUsername))

	var offset int64
	for {
		if ctx.Err() != nil {
			return nil
		}
		updates, err := b.getUpdates(ctx, offset)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		for i := range updates {
			u := &updates[i]
			if u.UpdateID >= offset {
				offset = u.UpdateID + 1
			}
			if u.Message != nil {
				go b.handleUpdate(ctx, u.Message)
			}
		}
	}
}

// ——— Bot API helpers ———

func (b *tgBot) fetchBotInfo(ctx context.Context) error {
	var user tgUser
	if err := b.apiGet(ctx, "getMe", nil, &user); err != nil {
		return err
	}
	b.botUsername = user.Username
	return nil
}

func (b *tgBot) getUpdates(ctx context.Context, offset int64) ([]tgUpdate, error) {
	params := map[string]interface{}{
		"offset":          offset,
		"timeout":         telegramLongPollTimeout,
		"allowed_updates": []string{"message"},
	}
	var updates []tgUpdate
	if err := b.apiPost(ctx, "getUpdates", params, &updates); err != nil {
		return nil, err
	}
	return updates, nil
}

func (b *tgBot) sendMessage(ctx context.Context, chatID int64, text string) (*tgSentMessage, error) {
	parts := splitTelegramMessage(text, telegramMaxMessageLen)
	if len(parts) == 0 {
		return nil, nil
	}
	var sent *tgSentMessage
	for i, part := range parts {
		params := map[string]interface{}{
			"chat_id":    chatID,
			"text":       part,
			"parse_mode": "Markdown",
		}
		var msg tgSentMessage
		if err := b.apiPost(ctx, "sendMessage", params, &msg); err != nil {
			// Fall back to plain text if Markdown parse fails
			params["parse_mode"] = ""
			if err2 := b.apiPost(ctx, "sendMessage", params, &msg); err2 != nil {
				b.logger.Warn("Telegram sendMessage failed", zap.Error(err2), zap.Int("part", i))
				continue
			}
		}
		if i == 0 {
			sent = &msg
		}
	}
	return sent, nil
}

func (b *tgBot) editMessageText(ctx context.Context, chatID, messageID int64, text string) error {
	if text == "" {
		return nil
	}
	if len(text) > telegramMaxMessageLen {
		text = text[:telegramMaxMessageLen-3] + "..."
	}
	params := map[string]interface{}{
		"chat_id":    chatID,
		"message_id": messageID,
		"text":       text,
		"parse_mode": "Markdown",
	}
	var result json.RawMessage
	if err := b.apiPost(ctx, "editMessageText", params, &result); err != nil {
		// Retry without Markdown if parse error
		params["parse_mode"] = ""
		return b.apiPost(ctx, "editMessageText", params, &result)
	}
	return nil
}

func (b *tgBot) sendChatAction(ctx context.Context, chatID int64, action string) {
	params := map[string]interface{}{
		"chat_id": chatID,
		"action":  action,
	}
	var result json.RawMessage
	_ = b.apiPost(ctx, "sendChatAction", params, &result)
}

// ——— Message handling ———

func (b *tgBot) handleUpdate(ctx context.Context, msg *tgMessage) {
	if msg.From == nil || msg.From.IsBot {
		return
	}

	// Check allowed users
	if len(b.allowedSet) > 0 && !b.allowedSet[msg.From.ID] {
		b.logger.Debug("Telegram message from unauthorized user", zap.Int64("user_id", msg.From.ID))
		_, _ = b.sendMessage(ctx, msg.Chat.ID, "⛔ You are not authorized to use this bot.")
		return
	}

	// In group chats, only process messages that mention the bot
	text := msg.Text
	if msg.Chat.Type != "private" {
		if !b.isMentioned(msg) {
			return
		}
		text = b.stripMention(text)
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return
	}

	b.logger.Info("Telegram message received",
		zap.Int64("user_id", msg.From.ID),
		zap.String("username", msg.From.Username),
		zap.String("chat_type", msg.Chat.Type),
		zap.String("content_preview", safeTruncate(text, 80)))

	userID := fmt.Sprintf("%d", msg.From.ID)

	// Check if it's a quick command (respond inline without placeholder)
	if isQuickCommand(text) {
		reply := b.h.HandleMessage("telegram", userID, text)
		parts := splitTelegramMessage(reply, telegramMaxMessageLen)
		for _, part := range parts {
			params := map[string]interface{}{
				"chat_id":    msg.Chat.ID,
				"text":       part,
				"parse_mode": "Markdown",
			}
			var result json.RawMessage
			if err := b.apiPost(ctx, "sendMessage", params, &result); err != nil {
				params["parse_mode"] = ""
				_ = b.apiPost(ctx, "sendMessage", params, &result)
			}
		}
		return
	}

	// For agent messages: send placeholder → stream progress → edit with final reply
	placeholderMsg, err := b.sendMessage(ctx, msg.Chat.ID, "⏳ Processing your request...")
	if err != nil || placeholderMsg == nil {
		b.logger.Warn("Telegram failed to send placeholder message", zap.Error(err))
		// Fall back: just process and send a new message
		reply := b.h.HandleMessage("telegram", userID, text)
		_, _ = b.sendMessage(ctx, msg.Chat.ID, reply)
		return
	}
	placeholderID := placeholderMsg.MessageID

	// Streaming progress: throttled Telegram message edits
	var (
		editMu      sync.Mutex
		lastEdit    time.Time
		progressBuf strings.Builder
		stepCount   int
	)

	progressFn := func(step string) {
		if step == "" {
			return
		}
		editMu.Lock()
		defer editMu.Unlock()
		stepCount++
		progressBuf.Reset()
		progressBuf.WriteString(fmt.Sprintf("⚙️ *Working...* (step %d)\n\n`%s`", stepCount, sanitizeProgress(step)))

		// Throttle edits: at most once every telegramEditThrottle
		if time.Since(lastEdit) < telegramEditThrottle {
			return
		}
		lastEdit = time.Now()
		editCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_ = b.editMessageText(editCtx, msg.Chat.ID, placeholderID, progressBuf.String())
	}

	// Periodically resend typing action while processing (lasts 5 s per call)
	typingCtx, cancelTyping := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(4 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-typingCtx.Done():
				return
			case <-ticker.C:
				b.sendChatAction(ctx, msg.Chat.ID, "typing")
			}
		}
	}()

	// Process message (blocking) with streaming progress if supported
	var reply string
	if sh, ok := b.h.(StreamingMessageHandler); ok {
		reply = sh.HandleMessageStream("telegram", userID, text, progressFn)
	} else {
		reply = b.h.HandleMessage("telegram", userID, text)
	}

	cancelTyping()

	// Edit placeholder with final response (split into multiple if too long)
	parts := splitTelegramMessage(reply, telegramMaxMessageLen)
	if len(parts) == 0 {
		parts = []string{"(no response)"}
	}

	editCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := b.editMessageText(editCtx, msg.Chat.ID, placeholderID, parts[0]); err != nil {
		// Edit failed: send as new message
		_, _ = b.sendMessage(ctx, msg.Chat.ID, parts[0])
	}
	// Send overflow parts as additional messages
	for _, part := range parts[1:] {
		_, _ = b.sendMessage(ctx, msg.Chat.ID, part)
	}
}

// ——— Low-level API ———

func (b *tgBot) apiGet(ctx context.Context, method string, params map[string]interface{}, out interface{}) error {
	url := b.apiURL + "/" + method
	if len(params) > 0 {
		qp := make([]string, 0, len(params))
		for k, v := range params {
			qp = append(qp, fmt.Sprintf("%s=%v", k, v))
		}
		url += "?" + strings.Join(qp, "&")
	}
	return b.doRequest(ctx, http.MethodGet, url, nil, out)
}

func (b *tgBot) apiPost(ctx context.Context, method string, params map[string]interface{}, out interface{}) error {
	url := b.apiURL + "/" + method
	body, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return b.doRequest(ctx, http.MethodPost, url, bytes.NewReader(body), out)
}

func (b *tgBot) doRequest(ctx context.Context, httpMethod, url string, body io.Reader, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, httpMethod, url, body)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var tgResp tgResponse
	if err := json.NewDecoder(resp.Body).Decode(&tgResp); err != nil {
		return fmt.Errorf("failed to decode Telegram response: %w", err)
	}
	if !tgResp.OK {
		return fmt.Errorf("Telegram API error: %s", tgResp.Description)
	}
	if out != nil && tgResp.Result != nil {
		return json.Unmarshal(tgResp.Result, out)
	}
	return nil
}

// ——— Helpers ———

// isMentioned returns true if the bot (@username) is mentioned in a group message.
func (b *tgBot) isMentioned(msg *tgMessage) bool {
	if b.botUsername == "" {
		return false
	}
	for _, entity := range msg.Entities {
		if entity.Type == "mention" {
			runes := []rune(msg.Text)
			if entity.Offset+entity.Length <= len(runes) {
				mention := string(runes[entity.Offset : entity.Offset+entity.Length])
				if strings.EqualFold(mention, "@"+b.botUsername) {
					return true
				}
			}
		}
	}
	return false
}

// stripMention removes @BotUsername from the message text.
func (b *tgBot) stripMention(text string) string {
	if b.botUsername == "" {
		return text
	}
	return strings.TrimSpace(strings.ReplaceAll(text, "@"+b.botUsername, ""))
}

// isQuickCommand returns true for commands that respond immediately without agent work.
func isQuickCommand(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	quickCmds := []string{
		"help", "?", "list", "conversations",
		"new", "clear", "current", "stop",
		"roles", "version",
	}
	for _, cmd := range quickCmds {
		if lower == cmd {
			return true
		}
	}
	// switch/continue/role/delete prefixes
	prefixes := []string{"switch ", "continue ", "role ", "delete "}
	for _, prefix := range prefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

// splitTelegramMessage splits a long message into chunks respecting Telegram's character limit.
// It prefers splitting at newlines.
func splitTelegramMessage(text string, limit int) []string {
	if len([]rune(text)) <= limit {
		return []string{text}
	}
	var parts []string
	runes := []rune(text)
	for len(runes) > 0 {
		if len(runes) <= limit {
			parts = append(parts, string(runes))
			break
		}
		chunk := runes[:limit]
		// Try to break at a newline
		cut := limit
		for i := limit - 1; i > limit/2; i-- {
			if chunk[i] == '\n' {
				cut = i + 1
				break
			}
		}
		parts = append(parts, string(runes[:cut]))
		runes = runes[cut:]
		// Strip leading newlines from next chunk
		for len(runes) > 0 && runes[0] == '\n' {
			runes = runes[1:]
		}
	}
	return parts
}

// sanitizeProgress trims and truncates a progress step for display in Telegram.
func sanitizeProgress(step string) string {
	step = strings.TrimSpace(step)
	if len(step) > 200 {
		step = step[:200] + "..."
	}
	// Escape backticks for inline code block
	step = strings.ReplaceAll(step, "`", "'")
	return step
}

// safeTruncate truncates a string to max runes for logging.
func safeTruncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "..."
}
