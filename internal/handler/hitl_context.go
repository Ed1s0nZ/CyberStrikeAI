package handler

import (
	"strings"
)

const (
	hitlPayloadUserMessage    = "userMessage"
	hitlPayloadThinking       = "thinking"
	hitlPayloadReasoningChain = "reasoningChain"
	hitlPayloadPlanning       = "planning"
)

type hitlCognitionFields struct {
	UserMessage    string
	Thinking       string
	ReasoningChain string
	Planning       string
}

func (h *AgentHandler) enrichHitlApprovalPayload(conversationID, assistantMessageID string, payload map[string]interface{}) {
	if h == nil || payload == nil {
		return
	}
	cog := h.collectHitlCognition(conversationID, assistantMessageID)
	if s := strings.TrimSpace(cog.UserMessage); s != "" {
		payload[hitlPayloadUserMessage] = s
	}
	if s := strings.TrimSpace(cog.Thinking); s != "" {
		payload[hitlPayloadThinking] = s
	}
	if s := strings.TrimSpace(cog.ReasoningChain); s != "" {
		payload[hitlPayloadReasoningChain] = s
	}
	if s := strings.TrimSpace(cog.Planning); s != "" {
		payload[hitlPayloadPlanning] = s
	}
}

func (h *AgentHandler) collectHitlCognition(conversationID, assistantMessageID string) hitlCognitionFields {
	var out hitlCognitionFields
	if h.tasks != nil {
		out = h.tasks.GetHitlCognition(conversationID)
	}
	if strings.TrimSpace(out.UserMessage) == "" && h.db != nil {
		if msg, err := h.db.GetTurnUserMessage(conversationID, assistantMessageID); err == nil {
			out.UserMessage = msg
		}
	}
	if h.db != nil && assistantMessageID != "" {
		dbCog, err := h.db.GetAssistantCognitionTexts(assistantMessageID)
		if err == nil {
			if strings.TrimSpace(out.Thinking) == "" {
				out.Thinking = dbCog.Thinking
			}
			if strings.TrimSpace(out.ReasoningChain) == "" {
				out.ReasoningChain = dbCog.ReasoningChain
			}
			if strings.TrimSpace(out.Planning) == "" {
				out.Planning = dbCog.Planning
			}
		}
	}
	return out
}

func snapshotHitlCognitionFromStreams(thinkingStreams map[string]*thinkingBuf, respPlan *responsePlanAgg) (thinking, reasoningChain, planning string) {
	if len(thinkingStreams) > 0 {
		var thinkingParts, reasoningParts []string
		for _, tb := range thinkingStreams {
			if tb == nil {
				continue
			}
			content := strings.TrimSpace(tb.b.String())
			if content == "" {
				continue
			}
			if tb.persistAs == "reasoning_chain" {
				reasoningParts = append(reasoningParts, content)
			} else {
				thinkingParts = append(thinkingParts, content)
			}
		}
		thinking = strings.Join(thinkingParts, "\n\n")
		reasoningChain = strings.Join(reasoningParts, "\n\n")
	}
	if respPlan != nil {
		planning = strings.TrimSpace(respPlan.b.String())
	}
	return thinking, reasoningChain, planning
}

func (h *AgentHandler) syncHitlCognitionFromProgress(conversationID, assistantMessageID string, thinkingStreams map[string]*thinkingBuf, respPlan *responsePlanAgg) {
	if h == nil || h.tasks == nil {
		return
	}
	thinking, reasoning, planning := snapshotHitlCognitionFromStreams(thinkingStreams, respPlan)
	if thinking == "" && reasoning == "" && planning == "" {
		return
	}
	h.tasks.UpdateHitlCognitionSnapshot(conversationID, assistantMessageID, thinking, reasoning, planning)
}
