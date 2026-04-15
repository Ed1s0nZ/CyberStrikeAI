package handler

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
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
	robotCmdHelp        = "帮助"
	robotCmdList        = "列表"
	robotCmdListAlt     = "对话列表"
	robotCmdSwitch      = "切换"
	robotCmdContinue    = "继续"
	robotCmdNew         = "新对话"
	robotCmdClear       = "清空"
	robotCmdCurrent     = "当前"
	robotCmdStop        = "停止"
	robotCmdRoles       = "角色"
	robotCmdRolesList   = "角色列表"
	robotCmdSwitchRole  = "切换角色"
	robotCmdDelete      = "删除"
	robotCmdVersion     = "版本"
)

// RobotHandler 企业微信/钉钉/飞书等机器人回调处理
type RobotHandler struct {
	config         *config.Config
	db             *database.DB
	agentHandler   *AgentHandler
	logger         *zap.Logger
	mu             sync.RWMutex
	sessions       map[string]string             // key: "platform_userID", value: conversationID
	sessionRoles   map[string]string             // key: "platform_userID", value: roleName（默认"默认"）
	cancelMu       sync.Mutex                    // 保护 runningCancels
	runningCancels map[string]context.CancelFunc // key: "platform_userID", 用于停止命令中断任务
}

// NewRobotHandler 创建机器人处理器
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

// sessionKey 生成会话 key
func (h *RobotHandler) sessionKey(platform, userID string) string {
	return platform + "_" + userID
}

// getOrCreateConversation 获取或创建当前会话，title 用于新对话的标题（取用户首条消息前50字）
func (h *RobotHandler) getOrCreateConversation(platform, userID, title string) (convID string, isNew bool) {
	h.mu.RLock()
	convID = h.sessions[h.sessionKey(platform, userID)]
	h.mu.RUnlock()
	if convID != "" {
		return convID, false
	}
	t := strings.TrimSpace(title)
	if t == "" {
		t = "新对话 " + time.Now().Format("01-02 15:04")
	} else {
		t = safeTruncateString(t, 50)
	}
	conv, err := h.db.CreateConversation(t)
	if err != nil {
		h.logger.Warn("创建机器人会话失败", zap.Error(err))
		return "", false
	}
	convID = conv.ID
	h.mu.Lock()
	h.sessions[h.sessionKey(platform, userID)] = convID
	h.mu.Unlock()
	return convID, true
}

// setConversation 切换当前会话
func (h *RobotHandler) setConversation(platform, userID, convID string) {
	h.mu.Lock()
	h.sessions[h.sessionKey(platform, userID)] = convID
	h.mu.Unlock()
}

// getRole 获取当前用户使用的角色，未设置时返回"默认"
func (h *RobotHandler) getRole(platform, userID string) string {
	h.mu.RLock()
	role := h.sessionRoles[h.sessionKey(platform, userID)]
	h.mu.RUnlock()
	if role == "" {
		return "默认"
	}
	return role
}

// setRole 设置当前用户使用的角色
func (h *RobotHandler) setRole(platform, userID, roleName string) {
	h.mu.Lock()
	h.sessionRoles[h.sessionKey(platform, userID)] = roleName
	h.mu.Unlock()
}

// clearConversation 清空当前会话（切换到新对话）
func (h *RobotHandler) clearConversation(platform, userID string) (newConvID string) {
	title := "新对话 " + time.Now().Format("01-02 15:04")
	conv, err := h.db.CreateConversation(title)
	if err != nil {
		h.logger.Warn("创建新对话失败", zap.Error(err))
		return ""
	}
	h.setConversation(platform, userID, conv.ID)
	return conv.ID
}

// HandleMessage 处理用户输入，返回回复文本（供各平台 webhook 调用）
func (h *RobotHandler) HandleMessage(platform, userID, text string) (reply string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "请输入内容或发送「帮助」/ help 查看命令。"
	}

	// 先尝试作为命令处理（支持中英文）
	if cmdReply, ok := h.handleRobotCommand(platform, userID, text); ok {
		return cmdReply
	}

	// 普通消息：走 Agent
	convID, _ := h.getOrCreateConversation(platform, userID, text)
	if convID == "" {
		return "无法创建或获取对话，请稍后再试。"
	}
	// 若对话标题为「新对话 xx:xx」格式（由「新对话」命令创建），将标题更新为首条消息内容，与 Web 端体验一致
	if conv, err := h.db.GetConversation(convID); err == nil && strings.HasPrefix(conv.Title, "新对话 ") {
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
		h.logger.Warn("机器人 Agent 执行失败", zap.String("platform", platform), zap.String("userID", userID), zap.Error(err))
		if errors.Is(err, context.Canceled) {
			return "任务已取消。"
		}
		return "处理失败: " + err.Error()
	}
	if newConvID != convID {
		h.setConversation(platform, userID, newConvID)
	}
	return resp
}

func (h *RobotHandler) cmdHelp() string {
	return "**【能盾智御 机器人命令】**\n\n" +
		"- `帮助` `help` — 显示本帮助 | Show this help\n" +
		"- `列表` `list` — 列出所有对话标题与 ID | List conversations\n" +
		"- `切换 <ID>` `switch <ID>` — 指定对话继续 | Switch to conversation\n" +
		"- `新对话` `new` — 开启新对话 | Start new conversation\n" +
		"- `清空` `clear` — 清空当前上下文 | Clear context\n" +
		"- `当前` `current` — 显示当前对话 ID 与标题 | Show current conversation\n" +
		"- `停止` `stop` — 中断当前任务 | Stop running task\n" +
		"- `角色` `roles` — 列出所有可用角色 | List roles\n" +
		"- `角色 <名>` `role <name>` — 切换当前角色 | Switch role\n" +
		"- `删除 <ID>` `delete <ID>` — 删除指定对话 | Delete conversation\n" +
		"- `版本` `version` — 显示当前版本号 | Show version\n\n" +
		"---\n" +
		"除以上命令外，直接输入内容将发送给 AI 进行渗透测试/安全分析。\n" +
		"Otherwise, send any text for AI penetration testing / security analysis."
}

func (h *RobotHandler) cmdList() string {
	convs, err := h.db.ListConversations(50, 0, "")
	if err != nil {
		return "获取对话列表失败: " + err.Error()
	}
	if len(convs) == 0 {
		return "暂无对话。发送任意内容将自动创建新对话。"
	}
	var b strings.Builder
	b.WriteString("【对话列表】\n")
	for i, c := range convs {
		if i >= 20 {
			b.WriteString("… 仅显示前 20 条\n")
			break
		}
		b.WriteString(fmt.Sprintf("· %s\n  ID: %s\n", c.Title, c.ID))
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func (h *RobotHandler) cmdSwitch(platform, userID, convID string) string {
	if convID == "" {
		return "请指定对话 ID，例如：切换 xxx-xxx-xxx"
	}
	conv, err := h.db.GetConversation(convID)
	if err != nil {
		return "对话不存在或 ID 错误。"
	}
	h.setConversation(platform, userID, conv.ID)
	return fmt.Sprintf("已切换到对话：「%s」\nID: %s", conv.Title, conv.ID)
}

func (h *RobotHandler) cmdNew(platform, userID string) string {
	newID := h.clearConversation(platform, userID)
	if newID == "" {
		return "创建新对话失败，请重试。"
	}
	return "已开启新对话，可直接发送内容。"
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
		return "当前没有正在执行的任务。"
	}
	return "已停止当前任务。"
}

func (h *RobotHandler) cmdCurrent(platform, userID string) string {
	h.mu.RLock()
	convID := h.sessions[h.sessionKey(platform, userID)]
	h.mu.RUnlock()
	if convID == "" {
		return "当前没有进行中的对话。发送任意内容将创建新对话。"
	}
	conv, err := h.db.GetConversation(convID)
	if err != nil {
		return "当前对话 ID: " + convID + "（获取标题失败）"
	}
	role := h.getRole(platform, userID)
	return fmt.Sprintf("当前对话：「%s」\nID: %s\n当前角色: %s", conv.Title, conv.ID, role)
}

func (h *RobotHandler) cmdRoles() string {
	if h.config.Roles == nil || len(h.config.Roles) == 0 {
		return "暂无可用角色。"
	}
	names := make([]string, 0, len(h.config.Roles))
	for name, role := range h.config.Roles {
		if role.Enabled {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return "暂无可用角色。"
	}
	sort.Slice(names, func(i, j int) bool {
		if names[i] == "默认" {
			return true
		}
		if names[j] == "默认" {
			return false
		}
		return names[i] < names[j]
	})
	var b strings.Builder
	b.WriteString("【角色列表】\n")
	for _, name := range names {
		role := h.config.Roles[name]
		desc := role.Description
		if desc == "" {
			desc = "无描述"
		}
		b.WriteString(fmt.Sprintf("· %s — %s\n", name, desc))
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func (h *RobotHandler) cmdSwitchRole(platform, userID, roleName string) string {
	if roleName == "" {
		return "请指定角色名称，例如：角色 渗透测试"
	}
	if h.config.Roles == nil {
		return "暂无可用角色。"
	}
	role, exists := h.config.Roles[roleName]
	if !exists {
		return fmt.Sprintf("角色「%s」不存在。发送「角色」查看可用角色。", roleName)
	}
	if !role.Enabled {
		return fmt.Sprintf("角色「%s」已禁用。", roleName)
	}
	h.setRole(platform, userID, roleName)
	return fmt.Sprintf("已切换到角色：「%s」\n%s", roleName, role.Description)
}

func (h *RobotHandler) cmdDelete(platform, userID, convID string) string {
	if convID == "" {
		return "请指定对话 ID，例如：删除 xxx-xxx-xxx"
	}
	sk := h.sessionKey(platform, userID)
	h.mu.RLock()
	currentConvID := h.sessions[sk]
	h.mu.RUnlock()
	if convID == currentConvID {
		// 删除当前对话时，先清空会话绑定
		h.mu.Lock()
		delete(h.sessions, sk)
		h.mu.Unlock()
	}
	if err := h.db.DeleteConversation(convID); err != nil {
		return "删除失败: " + err.Error()
	}
	return fmt.Sprintf("已删除对话 ID: %s", convID)
}

func (h *RobotHandler) cmdVersion() string {
	v := h.config.Version
	if v == "" {
		v = "未知"
	}
	return "能盾智御 " + v
}

// handleRobotCommand 处理机器人内置命令；若匹配到命令返回 (回复内容, true)，否则返回 ("", false)
func (h *RobotHandler) handleRobotCommand(platform, userID, text string) (string, bool) {
	switch {
	case text == robotCmdHelp || text == "help" || text == "？" || text == "?":
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

// —————— 企业微信 ——————

// wecomXML 企业微信回调 XML（明文模式下的简化结构；加密模式需先解密再解析）
type wecomXML struct {
	ToUserName   string `xml:"ToUserName"`
	FromUserName string `xml:"FromUserName"`
	CreateTime   int64  `xml:"CreateTime"`
	MsgType      string `xml:"MsgType"`
	Content      string `xml:"Content"`
	MsgID        string `xml:"MsgId"`
	AgentID      int64  `xml:"AgentID"`
	Encrypt      string `xml:"Encrypt"` // 加密模式下消息在此
}

// wecomReplyXML 被动回复 XML（仅用于兼容，当前使用手动构造 XML）
type wecomReplyXML struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName"`
	FromUserName string   `xml:"FromUserName"`
	CreateTime   int64    `xml:"CreateTime"`
	MsgType      string   `xml:"MsgType"`
	Content      string   `xml:"Content"`
}

// HandleWecomGET 企业微信 URL 校验（GET）
func (h *RobotHandler) HandleWecomGET(c *gin.Context) {
	if !h.config.Robots.Wecom.Enabled {
		c.String(http.StatusNotFound, "")
		return
	}
	// Gin 的 Query() 会自动 URL 解码，拿到的就是正确的 base64 字符串
	echostr := c.Query("echostr")
	msgSignature := c.Query("msg_signature")
	timestamp := c.Query("timestamp")
	nonce := c.Query("nonce")

	// 验证签名：将 token、timestamp、nonce、echostr 四个参数排序后拼接计算 SHA1
	signature := h.signWecomRequest(h.config.Robots.Wecom.Token, timestamp, nonce, echostr)
	if signature != msgSignature {
		h.logger.Warn("企业微信 URL 验证签名失败", zap.String("expected", msgSignature), zap.String("got", signature))
		c.String(http.StatusBadRequest, "invalid signature")
		return
	}

	if echostr == "" {
		c.String(http.StatusBadRequest, "missing echostr")
		return
	}

	// 如果配置了 EncodingAESKey，说明是加密模式，需要解密 echostr
	if h.config.Robots.Wecom.EncodingAESKey != "" {
		decrypted, err := wecomDecrypt(h.config.Robots.Wecom.EncodingAESKey, echostr)
		if err != nil {
			h.logger.Warn("企业微信 echostr 解密失败", zap.Error(err))
			c.String(http.StatusBadRequest, "decrypt failed")
			return
		}
		c.String(http.StatusOK, string(decrypted))
		return
	}

	// 明文模式直接返回 echostr
	c.String(http.StatusOK, echostr)
}

// signWecomRequest 生成企业微信请求签名
// 企业微信签名算法：将 token、timestamp、nonce、echostr 四个值排序后拼接成字符串，再计算 SHA1
func (h *RobotHandler) signWecomRequest(token, timestamp, nonce, echostr string) string {
	strs := []string{token, timestamp, nonce, echostr}
	sort.Strings(strs)
	s := strings.Join(strs, "")
	hash := sha1.Sum([]byte(s))
	return fmt.Sprintf("%x", hash)
}

// wecomDecrypt 企业微信消息解密（AES-256-CBC，PKCS7，明文格式：16字节随机+4字节长度+消息+corpID）
func wecomDecrypt(encodingAESKey, encryptedB64 string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(encodingAESKey + "=")
	if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("encoding_aes_key 解码后应为 32 字节")
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
		return nil, fmt.Errorf("密文长度不是块大小的倍数")
	}
	plain := make([]byte, len(ciphertext))
	mode.CryptBlocks(plain, ciphertext)
	// 去除 PKCS7 填充
	n := int(plain[len(plain)-1])
	if n < 1 || n > 32 {
		return nil, fmt.Errorf("无效的 PKCS7 填充")
	}
	plain = plain[:len(plain)-n]
	// 企业微信格式：16 字节随机 + 4 字节长度(大端) + 消息 + corpID
	if len(plain) < 20 {
		return nil, fmt.Errorf("明文过短")
	}
	msgLen := binary.BigEndian.Uint32(plain[16:20])
	if int(20+msgLen) > len(plain) {
		return nil, fmt.Errorf("消息长度越界")
	}
	return plain[20 : 20+msgLen], nil
}

// wecomEncrypt 企业微信消息加密（AES-256-CBC，PKCS7，明文格式：16字节随机+4字节长度+消息+corpID）
func wecomEncrypt(encodingAESKey, message, corpID string) (string, error) {
	key, err := base64.StdEncoding.DecodeString(encodingAESKey + "=")
	if err != nil {
		return "", err
	}
	if len(key) != 32 {
		return "", fmt.Errorf("encoding_aes_key 解码后应为 32 字节")
	}
	// 构造明文：16 字节随机 + 4 字节长度 (大端) + 消息 + corpID
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		// 降级方案：使用时间戳生成随机数
		for i := range random {
			random[i] = byte(time.Now().UnixNano() % 256)
		}
	}
	msgLen := len(message)
	msgBytes := []byte(message)
	corpBytes := []byte(corpID)
	plain := make([]byte, 16+4+msgLen+len(corpBytes))
	copy(plain[:16], random)
	binary.BigEndian.PutUint32(plain[16:20], uint32(msgLen))
	copy(plain[20:20+msgLen], msgBytes)
	copy(plain[20+msgLen:], corpBytes)
	// PKCS7 填充
	padding := aes.BlockSize - len(plain)%aes.BlockSize
	pad := bytes.Repeat([]byte{byte(padding)}, padding)
	plain = append(plain, pad...)
	// AES-256-CBC 加密
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	iv := key[:16]
	ciphertext := make([]byte, len(plain))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, plain)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// HandleWecomPOST 企业微信消息回调（POST），支持明文与加密模式
func (h *RobotHandler) HandleWecomPOST(c *gin.Context) {
	if !h.config.Robots.Wecom.Enabled {
		h.logger.Debug("企业微信机器人未启用，跳过请求")
		c.String(http.StatusOK, "")
		return
	}
	// 从 URL 获取签名参数（加密模式回复时需要用到）
	timestamp := c.Query("timestamp")
	nonce := c.Query("nonce")
	msgSignature := c.Query("msg_signature")

	// 先读取请求体，后续解析/签名验证都会用到
	bodyRaw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.logger.Warn("企业微信 POST 读取请求体失败", zap.Error(err))
		c.String(http.StatusOK, "")
		return
	}
	h.logger.Debug("企业微信 POST 收到请求", zap.String("body", string(bodyRaw)))

	// 验证请求签名防止伪造。企业微信签名算法同 URL 验证，使用 token、timestamp、nonce、 Encrypt 四个字段
	// 若配置了 Token 则必须校验签名，避免未授权请求触发 Agent（防止平台被接管）
	token := h.config.Robots.Wecom.Token
	if token != "" {
		if msgSignature == "" {
			h.logger.Warn("企业微信 POST 缺少签名，已拒绝（需配置 token 并确保回调携带 msg_signature）")
			c.String(http.StatusOK, "")
			return
		}
		var tmp wecomXML
		if err := xml.Unmarshal(bodyRaw, &tmp); err != nil {
			h.logger.Warn("企业微信 POST 签名验证前解析 XML 失败", zap.Error(err))
			c.String(http.StatusOK, "")
			return
		}
		expected := h.signWecomRequest(token, timestamp, nonce, tmp.Encrypt)
		if expected != msgSignature {
			h.logger.Warn("企业微信 POST 签名验证失败", zap.String("expected", expected), zap.String("got", msgSignature))
			c.String(http.StatusOK, "")
			return
		}
	}

	var body wecomXML
	if err := xml.Unmarshal(bodyRaw, &body); err != nil {
		h.logger.Warn("企业微信 POST 解析 XML 失败", zap.Error(err))
		c.String(http.StatusOK, "")
		return
	}
	h.logger.Debug("企业微信 XML 解析成功", zap.String("ToUserName", body.ToUserName), zap.String("FromUserName", body.FromUserName), zap.String("MsgType", body.MsgType), zap.String("Content", body.Content), zap.String("Encrypt", body.Encrypt))

	// 保存企业 ID（用于明文模式回复）
	enterpriseID := body.ToUserName

	// 加密模式：先解密再解析内层 XML
	if body.Encrypt != "" && h.config.Robots.Wecom.EncodingAESKey != "" {
		h.logger.Debug("企业微信进入加密模式解密流程")
		decrypted, err := wecomDecrypt(h.config.Robots.Wecom.EncodingAESKey, body.Encrypt)
		if err != nil {
			h.logger.Warn("企业微信消息解密失败", zap.Error(err))
			c.String(http.StatusOK, "")
			return
		}
		h.logger.Debug("企业微信解密成功", zap.String("decrypted", string(decrypted)))
		if err := xml.Unmarshal(decrypted, &body); err != nil {
			h.logger.Warn("企业微信解密后 XML 解析失败", zap.Error(err))
			c.String(http.StatusOK, "")
			return
		}
		h.logger.Debug("企业微信内层 XML 解析成功", zap.String("FromUserName", body.FromUserName), zap.String("Content", body.Content))
	}

	userID := body.FromUserName
	text := strings.TrimSpace(body.Content)

	// 限制回复内容长度（企业微信限制 2048 字节）
	maxReplyLen := 2000
	limitReply := func(s string) string {
		if len(s) > maxReplyLen {
			return s[:maxReplyLen] + "\n\n（内容过长，已截断）"
		}
		return s
	}

	if body.MsgType != "text" {
		h.logger.Debug("企业微信收到非文本消息", zap.String("MsgType", body.MsgType))
		h.sendWecomReply(c, userID, enterpriseID, limitReply("暂仅支持文本消息，请发送文字。"), timestamp, nonce)
		return
	}

	// 文本消息：先判断是否为内置命令（如 帮助/列表/新对话 等），这类命令处理很快，可以直接走被动回复，避免依赖主动发送 API。
	if cmdReply, ok := h.handleRobotCommand("wecom", userID, text); ok {
		h.logger.Debug("企业微信收到命令消息，走被动回复", zap.String("userID", userID), zap.String("text", text))
		h.sendWecomReply(c, userID, enterpriseID, limitReply(cmdReply), timestamp, nonce)
		return
	}

	h.logger.Debug("企业微信开始处理消息（异步 AI）", zap.String("userID", userID), zap.String("text", text))

	// 企业微信被动回复有 5 秒超时限制，而 AI 调用通常超过该时长。
	// 这里采用推荐做法：立即返回 success（或空串），然后通过主动发送接口推送完整回复。
	c.String(http.StatusOK, "success")

	// 异步处理消息并通过企业微信主动消息接口发送结果
	go func() {
		reply := h.HandleMessage("wecom", userID, text)
		reply = limitReply(reply)
		h.logger.Debug("企业微信消息处理完成", zap.String("userID", userID), zap.String("reply", reply))
		// 调用企业微信 API 主动发送消息
		h.sendWecomMessageViaAPI(userID, enterpriseID, reply)
	}()
}

// sendWecomReply 发送企业微信回复（加密模式自动加密）
// 参数：toUser=用户 ID, fromUser=企业 ID（明文模式）/CorpID（加密模式）, content=回复内容，timestamp/nonce=请求参数
func (h *RobotHandler) sendWecomReply(c *gin.Context, toUser, fromUser, content, timestamp, nonce string) {
	// 加密模式：判断 EncodingAESKey 是否配置
	if h.config.Robots.Wecom.EncodingAESKey != "" {
		// 加密模式使用 CorpID 进行加密
		corpID := h.config.Robots.Wecom.CorpID
		if corpID == "" {
			h.logger.Warn("企业微信加密模式缺少 CorpID 配置")
			c.String(http.StatusOK, "")
			return
		}

		// 构造完整的明文 XML 回复（格式严格按企业微信文档要求）
		plainResp := fmt.Sprintf(`<xml>
<ToUserName><![CDATA[%s]]></ToUserName>
<FromUserName><![CDATA[%s]]></FromUserName>
<CreateTime>%d</CreateTime>
<MsgType><![CDATA[text]]></MsgType>
<Content><![CDATA[%s]]></Content>
</xml>`, toUser, fromUser, time.Now().Unix(), content)

		encrypted, err := wecomEncrypt(h.config.Robots.Wecom.EncodingAESKey, plainResp, corpID)
		if err != nil {
			h.logger.Warn("企业微信回复加密失败", zap.Error(err))
			c.String(http.StatusOK, "")
			return
		}
		// 使用请求中的 timestamp/nonce 生成签名（企业微信要求回复时使用与请求相同的 timestamp 和 nonce）
		msgSignature := h.signWecomRequest(h.config.Robots.Wecom.Token, timestamp, nonce, encrypted)

		h.logger.Debug("企业微信发送加密回复",
			zap.String("Encrypt", encrypted[:50]+"..."),
			zap.String("MsgSignature", msgSignature),
			zap.String("TimeStamp", timestamp),
			zap.String("Nonce", nonce))

		// 加密模式仅返回 4 个核心字段（企业微信官方要求）
		xmlResp := fmt.Sprintf(`<xml><Encrypt><![CDATA[%s]]></Encrypt><MsgSignature><![CDATA[%s]]></MsgSignature><TimeStamp><![CDATA[%s]]></TimeStamp><Nonce><![CDATA[%s]]></Nonce></xml>`, encrypted, msgSignature, timestamp, nonce)
		// also log the final response body so we can cross-check with the
		// network traffic or developer console
		h.logger.Debug("企业微信加密回复包", zap.String("xml", xmlResp))
		// for additional confidence, decrypt the payload ourselves and log it
		if dec, err2 := wecomDecrypt(h.config.Robots.Wecom.EncodingAESKey, encrypted); err2 == nil {
			h.logger.Debug("企业微信加密回复解密检查", zap.String("plain", string(dec)))
		} else {
			h.logger.Warn("企业微信加密回复解密检查失败", zap.Error(err2))
		}

		// 使用 c.Writer.Write 直接写入响应，避免 c.String 的转义问题
		c.Writer.WriteHeader(http.StatusOK)
		// use text/xml as that's what WeCom examples show
		c.Writer.Header().Set("Content-Type", "text/xml; charset=utf-8")
		_, _ = c.Writer.Write([]byte(xmlResp))
		h.logger.Debug("企业微信加密回复已发送")
		return
	}

	// 明文模式
	h.logger.Debug("企业微信发送明文回复", zap.String("ToUserName", toUser), zap.String("FromUserName", fromUser), zap.String("Content", content[:50]+"..."))

	// 手动构造 XML 响应（使用 CDATA 包裹所有字段，并包含 AgentID）
	xmlResp := fmt.Sprintf(`<xml>
<ToUserName><![CDATA[%s]]></ToUserName>
<FromUserName><![CDATA[%s]]></FromUserName>
<CreateTime>%d</CreateTime>
<MsgType><![CDATA[text]]></MsgType>
<Content><![CDATA[%s]]></Content>
</xml>`, toUser, fromUser, time.Now().Unix(), content)

	// log the exact plaintext response for debugging
	h.logger.Debug("企业微信明文回复包", zap.String("xml", xmlResp))

	// use text/xml as recommended by WeCom docs
	c.Header("Content-Type", "text/xml; charset=utf-8")
	c.String(http.StatusOK, xmlResp)
	h.logger.Debug("企业微信明文回复已发送")
}

// —————— 测试接口（需登录，用于验证机器人逻辑，无需钉钉/飞书客户端） ——————

// RobotTestRequest 模拟机器人消息请求
type RobotTestRequest struct {
	Platform string `json:"platform"` // 如 "dingtalk"、"lark"、"wecom"
	UserID   string `json:"user_id"`
	Text     string `json:"text"`
}

// HandleRobotTest 供本地验证：POST JSON { "platform", "user_id", "text" }，返回 { "reply": "..." }
func (h *RobotHandler) HandleRobotTest(c *gin.Context) {
	var req RobotTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体需为 JSON，包含 platform、user_id、text"})
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

// sendWecomMessageViaAPI 通过企业微信 API 主动发送消息（用于异步处理后的结果发送）
func (h *RobotHandler) sendWecomMessageViaAPI(toUser, toParty, content string) {
	if !h.config.Robots.Wecom.Enabled {
		return
	}

	secret := h.config.Robots.Wecom.Secret
	corpID := h.config.Robots.Wecom.CorpID
	agentID := h.config.Robots.Wecom.AgentID

	if secret == "" || corpID == "" {
		h.logger.Warn("企业微信主动 API 缺少 secret 或 corpID 配置")
		return
	}

	// 第 1 步：获取 access_token
	tokenURL := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=%s&corpsecret=%s", corpID, secret)
	resp, err := http.Get(tokenURL)
	if err != nil {
		h.logger.Warn("企业微信获取 token 失败", zap.Error(err))
		return
	}
	defer resp.Body.Close()

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		h.logger.Warn("企业微信 token 响应解析失败", zap.Error(err))
		return
	}
	if tokenResp.ErrCode != 0 {
		h.logger.Warn("企业微信 token 获取错误", zap.String("errmsg", tokenResp.ErrMsg), zap.Int("errcode", tokenResp.ErrCode))
		return
	}

	// 第 2 步：构造发送消息请求
	msgReq := map[string]interface{}{
		"touser":  toUser,
		"msgtype": "text",
		"agentid": agentID,
		"text": map[string]interface{}{
			"content": content,
		},
	}

	msgBody, err := json.Marshal(msgReq)
	if err != nil {
		h.logger.Warn("企业微信消息序列化失败", zap.Error(err))
		return
	}

	// 第 3 步：发送消息
	sendURL := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=%s", tokenResp.AccessToken)
	msgResp, err := http.Post(sendURL, "application/json", bytes.NewReader(msgBody))
	if err != nil {
		h.logger.Warn("企业微信主动发送消息失败", zap.Error(err))
		return
	}
	defer msgResp.Body.Close()

	var sendResp struct {
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
		InvalidUser string `json:"invaliduser"`
		MsgID       string `json:"msgid"`
	}
	if err := json.NewDecoder(msgResp.Body).Decode(&sendResp); err != nil {
		h.logger.Warn("企业微信发送响应解析失败", zap.Error(err))
		return
	}

	if sendResp.ErrCode == 0 {
		h.logger.Debug("企业微信主动发送消息成功", zap.String("msgid", sendResp.MsgID))
	} else {
		h.logger.Warn("企业微信主动发送消息失败", zap.String("errmsg", sendResp.ErrMsg), zap.Int("errcode", sendResp.ErrCode), zap.String("invaliduser", sendResp.InvalidUser))
	}
}

// —————— 钉钉 ——————

// HandleDingtalkPOST 钉钉事件回调（流式接入等）；当前为占位，返回 200
func (h *RobotHandler) HandleDingtalkPOST(c *gin.Context) {
	if !h.config.Robots.Dingtalk.Enabled {
		c.JSON(http.StatusOK, gin.H{})
		return
	}
	// 钉钉流式/事件回调格式需按官方文档解析并异步回复，此处仅返回 200
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// —————— 飞书 ——————

// HandleLarkPOST 飞书事件回调；当前为占位，返回 200；验证时需返回 challenge
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
