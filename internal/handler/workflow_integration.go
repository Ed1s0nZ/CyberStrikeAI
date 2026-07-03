package handler

import (
	"context"
	"net/http"
	"strings"
	"time"

	"cyberstrike-ai/internal/config"
	workflowrunner "cyberstrike-ai/internal/workflow"

	"github.com/gin-gonic/gin"
)

func (h *AgentHandler) roleForWorkflow(req *ChatRequest) (config.RoleConfig, bool) {
	if h == nil || h.config == nil || h.config.Roles == nil || req == nil {
		return config.RoleConfig{}, false
	}
	roleName := strings.TrimSpace(req.Role)
	if roleName == "" {
		return config.RoleConfig{}, false
	}
	role, ok := h.config.Roles[roleName]
	if !ok || !role.Enabled {
		return config.RoleConfig{}, false
	}
	if role.Name == "" {
		role.Name = roleName
	}
	if !workflowrunner.ShouldAutoRunRoleWorkflow(role) {
		return config.RoleConfig{}, false
	}
	return role, true
}

func (h *AgentHandler) runRoleWorkflowStreamIfBound(
	req *ChatRequest,
	prep *multiAgentPrepared,
	sendEvent func(eventType, message string, data interface{}),
) bool {
	role, ok := h.roleForWorkflow(req)
	if !ok || prep == nil {
		return false
	}
	baseCtx, cancelWithCause := context.WithCancelCause(context.Background())
	defer cancelWithCause(nil)
	progress := h.createProgressCallback(baseCtx, cancelWithCause, prep.ConversationID, prep.AssistantMessageID, sendEvent)
	result, err := workflowrunner.RunRoleBoundWorkflow(baseCtx, workflowrunner.RunArgs{
		DB:                 h.db,
		Logger:             h.logger,
		Role:               role,
		AppCfg:             h.config,
		Agent:              h.agent,
		ConversationID:     prep.ConversationID,
		ProjectID:          h.conversationProjectID(prep.ConversationID),
		UserMessage:        prep.FinalMessage,
		History:            prep.History,
		RoleTools:          prep.RoleTools,
		AgentsMarkdownDir:  h.agentsMarkdownDir,
		SystemPromptExtra:  h.agentSessionContextBlock(prep.ConversationID),
		AssistantMessageID: prep.AssistantMessageID,
		Progress:           progress,
	})
	if err != nil {
		errMsg := "执行角色绑定流程失败: " + err.Error()
		if prep.AssistantMessageID != "" {
			_, _ = h.db.Exec("UPDATE messages SET content = ?, updated_at = ? WHERE id = ?", errMsg, time.Now(), prep.AssistantMessageID)
			_ = h.db.AddProcessDetail(prep.AssistantMessageID, prep.ConversationID, "error", errMsg, nil)
		}
		sendEvent("error", errMsg, map[string]interface{}{"conversationId": prep.ConversationID})
		sendEvent("done", "", map[string]interface{}{"conversationId": prep.ConversationID})
		return true
	}
	if prep.AssistantMessageID != "" {
		_ = h.db.UpdateAssistantMessageFinalize(prep.AssistantMessageID, result.Response, nil, "")
	}
	sendEvent("response", result.Response, map[string]interface{}{
		"conversationId": prep.ConversationID,
		"messageId":      prep.AssistantMessageID,
		"agentMode":      "workflow",
		"workflowRunId":  result.RunID,
	})
	sendEvent("done", "", map[string]interface{}{"conversationId": prep.ConversationID})
	return true
}

func (h *AgentHandler) runRoleWorkflowJSONIfBound(c *gin.Context, req *ChatRequest, prep *multiAgentPrepared) bool {
	role, ok := h.roleForWorkflow(req)
	if !ok || prep == nil {
		return false
	}
	baseCtx, cancelWithCause := context.WithCancelCause(c.Request.Context())
	defer cancelWithCause(nil)
	progress := h.createProgressCallback(baseCtx, cancelWithCause, prep.ConversationID, prep.AssistantMessageID, nil)
	result, err := workflowrunner.RunRoleBoundWorkflow(baseCtx, workflowrunner.RunArgs{
		DB:                 h.db,
		Logger:             h.logger,
		Role:               role,
		AppCfg:             h.config,
		Agent:              h.agent,
		ConversationID:     prep.ConversationID,
		ProjectID:          h.conversationProjectID(prep.ConversationID),
		UserMessage:        prep.FinalMessage,
		History:            prep.History,
		RoleTools:          prep.RoleTools,
		AgentsMarkdownDir:  h.agentsMarkdownDir,
		SystemPromptExtra:  h.agentSessionContextBlock(prep.ConversationID),
		AssistantMessageID: prep.AssistantMessageID,
		Progress:           progress,
	})
	if err != nil {
		errMsg := "执行角色绑定流程失败: " + err.Error()
		if prep.AssistantMessageID != "" {
			_, _ = h.db.Exec("UPDATE messages SET content = ?, updated_at = ? WHERE id = ?", errMsg, time.Now(), prep.AssistantMessageID)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsg, "conversationId": prep.ConversationID})
		return true
	}
	if prep.AssistantMessageID != "" {
		_ = h.db.UpdateAssistantMessageFinalize(prep.AssistantMessageID, result.Response, nil, "")
	}
	c.JSON(http.StatusOK, gin.H{
		"response":           result.Response,
		"conversationId":     prep.ConversationID,
		"assistantMessageId": prep.AssistantMessageID,
		"agentMode":          "workflow",
		"workflowRunId":      result.RunID,
	})
	return true
}
