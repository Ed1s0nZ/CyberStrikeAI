package handler

import (
	"fmt"
	"strings"

	"cyberstrike-ai/internal/agent"
	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/mcp/builtin"

	"go.uber.org/zap"
)

// multiAgentPrepared 多代理请求在调用 Eino 前的会话与消息准备结果。
type multiAgentPrepared struct {
	ConversationID     string
	CreatedNew         bool
	History            []agent.ChatMessage
	FinalMessage       string
	RoleTools          []string
	AssistantMessageID string
	UserMessageID      string
}

func (h *AgentHandler) prepareMultiAgentSession(req *ChatRequest) (*multiAgentPrepared, error) {
	if len(req.Attachments) > maxAttachments {
		return nil, fmt.Errorf("附件最多 %d 个", maxAttachments)
	}

	conversationID := strings.TrimSpace(req.ConversationID)
	createdNew := false
	if conversationID == "" {
		title := safeTruncateString(req.Message, 50)
		var conv *database.Conversation
		var err error
		if strings.TrimSpace(req.WebShellConnectionID) != "" {
			conv, err = h.db.CreateConversationWithWebshell(strings.TrimSpace(req.WebShellConnectionID), title)
		} else {
			conv, err = h.db.CreateConversation(title)
		}
		if err != nil {
			return nil, fmt.Errorf("创建对话失败: %w", err)
		}
		conversationID = conv.ID
		createdNew = true
	} else {
		if _, err := h.db.GetConversation(conversationID); err != nil {
			return nil, fmt.Errorf("对话不存在")
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
				agentHistoryMessages = append(agentHistoryMessages, agent.ChatMessage{
					Role:    msg.Role,
					Content: msg.Content,
				})
			}
		}
	}

	finalMessage := req.Message
	var roleTools []string
	if req.WebShellConnectionID != "" {
		conn, errConn := h.db.GetWebshellConnection(strings.TrimSpace(req.WebShellConnectionID))
		if errConn != nil || conn == nil {
			h.logger.Warn("WebShell AI 助手：未找到连接", zap.String("id", req.WebShellConnectionID), zap.Error(errConn))
			return nil, fmt.Errorf("未找到该 WebShell 连接")
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
	} else if req.Role != "" && req.Role != "默认" && h.config != nil && h.config.Roles != nil {
		if role, exists := h.config.Roles[req.Role]; exists && role.Enabled {
			if role.UserPrompt != "" {
				finalMessage = role.UserPrompt + "\n\n" + req.Message
			}
			roleTools = role.Tools
		}
	}

	var savedPaths []string
	if len(req.Attachments) > 0 {
		var aerr error
		savedPaths, aerr = saveAttachmentsToDateAndConversationDir(req.Attachments, conversationID, h.logger)
		if aerr != nil {
			return nil, fmt.Errorf("保存上传文件失败: %w", aerr)
		}
	}
	finalMessage = appendAttachmentsToMessage(finalMessage, req.Attachments, savedPaths)

	userContent := userMessageContentForStorage(req.Message, req.Attachments, savedPaths)
	userMsgRow, uerr := h.db.AddMessage(conversationID, "user", userContent, nil)
	if uerr != nil {
		h.logger.Error("保存用户消息失败", zap.Error(uerr))
		return nil, fmt.Errorf("保存用户消息失败: %w", uerr)
	}
	userMessageID := ""
	if userMsgRow != nil {
		userMessageID = userMsgRow.ID
	}

	assistantMsg, aerr := h.db.AddMessage(conversationID, "assistant", "处理中...", nil)
	var assistantMessageID string
	if aerr != nil {
		h.logger.Warn("创建助手消息占位失败", zap.Error(aerr))
	} else if assistantMsg != nil {
		assistantMessageID = assistantMsg.ID
	}

	return &multiAgentPrepared{
		ConversationID:     conversationID,
		CreatedNew:         createdNew,
		History:            agentHistoryMessages,
		FinalMessage:       finalMessage,
		RoleTools:          roleTools,
		AssistantMessageID: assistantMessageID,
		UserMessageID:      userMessageID,
	}, nil
}
