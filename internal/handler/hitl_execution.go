package handler

import (
	"encoding/json"
	"strings"
	"time"
)

const hitlPayloadExecutionResult = "executionResult"

type hitlExecutionResult struct {
	Success    bool      `json:"success"`
	Result     string    `json:"result,omitempty"`
	ToolName   string    `json:"toolName,omitempty"`
	ToolCallID string    `json:"toolCallId,omitempty"`
	RecordedAt time.Time `json:"recordedAt"`
}

type hitlApprovedExecTrack struct {
	InterruptID    string
	ConversationID string
	ToolName       string
	ToolCallID     string
}

// TrackApprovedHitlExecution 审批通过后登记，待 tool_result 回写执行结果。
func (m *HITLManager) TrackApprovedHitlExecution(interruptID, conversationID, toolName, toolCallID string) {
	if m == nil {
		return
	}
	interruptID = strings.TrimSpace(interruptID)
	conversationID = strings.TrimSpace(conversationID)
	if interruptID == "" || conversationID == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.approvedExec == nil {
		m.approvedExec = make(map[string][]hitlApprovedExecTrack)
	}
	m.approvedExec[conversationID] = append(m.approvedExec[conversationID], hitlApprovedExecTrack{
		InterruptID:    interruptID,
		ConversationID: conversationID,
		ToolName:       strings.TrimSpace(toolName),
		ToolCallID:     strings.TrimSpace(toolCallID),
	})
}

func (m *HITLManager) popApprovedInterruptForTool(conversationID, toolCallID, toolName string) string {
	if m == nil {
		return ""
	}
	conversationID = strings.TrimSpace(conversationID)
	toolCallID = strings.TrimSpace(toolCallID)
	toolName = strings.TrimSpace(toolName)
	m.mu.Lock()
	defer m.mu.Unlock()
	queue := m.approvedExec[conversationID]
	if len(queue) == 0 {
		return ""
	}
	idx := -1
	if toolCallID != "" {
		for i, t := range queue {
			if t.ToolCallID == toolCallID {
				idx = i
				break
			}
		}
	}
	if idx < 0 && toolName != "" {
		for i, t := range queue {
			if strings.EqualFold(t.ToolName, toolName) {
				idx = i
				break
			}
		}
	}
	if idx < 0 {
		return ""
	}
	id := queue[idx].InterruptID
	queue = append(queue[:idx], queue[idx+1:]...)
	if len(queue) == 0 {
		delete(m.approvedExec, conversationID)
	} else {
		m.approvedExec[conversationID] = queue
	}
	return id
}

func mergeHitlPayloadExecutionResult(payloadJSON string, exec hitlExecutionResult) (string, error) {
	root := make(map[string]interface{})
	if strings.TrimSpace(payloadJSON) != "" {
		_ = json.Unmarshal([]byte(payloadJSON), &root)
	}
	if root == nil {
		root = make(map[string]interface{})
	}
	root[hitlPayloadExecutionResult] = exec
	out, err := json.Marshal(root)
	if err != nil {
		return payloadJSON, err
	}
	return string(out), nil
}

func (h *AgentHandler) recordHitlToolExecutionResult(conversationID, toolCallID, toolName string, success bool, result string) {
	if h == nil || h.hitlManager == nil || h.db == nil {
		return
	}
	interruptID := h.hitlManager.popApprovedInterruptForTool(conversationID, toolCallID, toolName)
	if interruptID == "" {
		return
	}
	var payloadJSON string
	err := h.db.QueryRow(`SELECT payload FROM hitl_interrupts WHERE id = ?`, interruptID).Scan(&payloadJSON)
	if err != nil {
		return
	}
	merged, err := mergeHitlPayloadExecutionResult(payloadJSON, hitlExecutionResult{
		Success:    success,
		Result:     strings.TrimSpace(result),
		ToolName:   strings.TrimSpace(toolName),
		ToolCallID: strings.TrimSpace(toolCallID),
		RecordedAt: time.Now(),
	})
	if err != nil {
		return
	}
	_, _ = h.db.Exec(`UPDATE hitl_interrupts SET payload = ? WHERE id = ?`, merged, interruptID)
}
