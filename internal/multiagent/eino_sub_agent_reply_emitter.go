package multiagent

import (
	"strings"

	"cyberstrike-ai/internal/openai"
)

type einoSubAgentReplyEmitter struct {
	progress       func(eventType, message string, data interface{})
	conversationID string
	agentName      string
	nextStreamID   func() string

	streamID string
	buf      string
}

func newEinoSubAgentReplyEmitter(
	conversationID, agentName string,
	progress func(eventType, message string, data interface{}),
	nextStreamID func() string,
) *einoSubAgentReplyEmitter {
	return &einoSubAgentReplyEmitter{
		progress:       progress,
		conversationID: conversationID,
		agentName:      agentName,
		nextStreamID:   nextStreamID,
	}
}

func (e *einoSubAgentReplyEmitter) EmitDelta(content string) bool {
	if e == nil || content == "" {
		return false
	}
	var delta string
	e.buf, delta = normalizeStreamingDelta(e.buf, content)
	if delta == "" || e.progress == nil {
		return false
	}
	if e.streamID == "" {
		if e.nextStreamID != nil {
			e.streamID = e.nextStreamID()
		}
		if e.streamID == "" {
			e.streamID = "eino-sub-reply"
		}
		e.progress("eino_agent_reply_stream_start", "", map[string]interface{}{
			"streamId":       e.streamID,
			"einoAgent":      e.agentName,
			"einoRole":       "sub",
			"conversationId": e.conversationID,
			"source":         "eino",
		})
	}
	e.progress("eino_agent_reply_stream_delta", delta, openai.WithSSEAccumulated(map[string]interface{}{
		"streamId":       e.streamID,
		"conversationId": e.conversationID,
	}, e.buf))
	return true
}

func (e *einoSubAgentReplyEmitter) Finish() string {
	if e == nil {
		return ""
	}
	body := strings.TrimSpace(e.buf)
	if body == "" || e.progress == nil {
		return body
	}
	if e.streamID != "" {
		e.progress("eino_agent_reply_stream_end", body, map[string]interface{}{
			"streamId":       e.streamID,
			"einoAgent":      e.agentName,
			"einoRole":       "sub",
			"conversationId": e.conversationID,
			"source":         "eino",
		})
	} else {
		e.EmitComplete(body)
	}
	return body
}

func (e *einoSubAgentReplyEmitter) EmitComplete(body string) bool {
	if e == nil || e.progress == nil {
		return false
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return false
	}
	e.progress("eino_agent_reply", body, map[string]interface{}{
		"conversationId": e.conversationID,
		"einoAgent":      e.agentName,
		"einoRole":       "sub",
		"source":         "eino",
	})
	return true
}
