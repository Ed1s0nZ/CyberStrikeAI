package multiagent

import (
	"strings"

	"cyberstrike-ai/internal/openai"
)

type einoReasoningStreamEmitter struct {
	progress     func(eventType, message string, data interface{})
	conversation string
	orchMode     string
	agentName    string
	einoRole     string
	nextStreamID func() string

	streamID    string
	rawBuf      string
	displayPrev string
}

func newEinoReasoningStreamEmitter(
	conversationID, orchMode, agentName, einoRole string,
	progress func(eventType, message string, data interface{}),
	nextStreamID func() string,
) *einoReasoningStreamEmitter {
	return &einoReasoningStreamEmitter{
		progress:     progress,
		conversation: conversationID,
		orchMode:     orchMode,
		agentName:    agentName,
		einoRole:     einoRole,
		nextStreamID: nextStreamID,
	}
}

func (e *einoReasoningStreamEmitter) EmitDelta(reasoningContent string) bool {
	if e == nil || strings.TrimSpace(reasoningContent) == "" {
		return false
	}
	var rawDelta string
	e.rawBuf, rawDelta = normalizeStreamingDelta(e.rawBuf, reasoningContent)
	if rawDelta == "" || e.progress == nil {
		return false
	}
	fullDisplay := openai.DisplayReasoningContent(e.rawBuf)
	displayDelta := fullDisplay
	if strings.HasPrefix(fullDisplay, e.displayPrev) {
		displayDelta = fullDisplay[len(e.displayPrev):]
	}
	e.displayPrev = fullDisplay
	if displayDelta == "" {
		return false
	}
	if e.streamID == "" {
		if e.nextStreamID != nil {
			e.streamID = e.nextStreamID()
		}
		if e.streamID == "" {
			e.streamID = "eino-reasoning"
		}
		e.progress("reasoning_chain_stream_start", " ", map[string]interface{}{
			"streamId":      e.streamID,
			"source":        "eino",
			"einoAgent":     e.agentName,
			"einoRole":      e.einoRole,
			"orchestration": e.orchMode,
		})
	}
	e.progress("reasoning_chain_stream_delta", displayDelta, openai.WithSSEAccumulated(map[string]interface{}{
		"streamId": e.streamID,
	}, fullDisplay))
	return true
}

func (e *einoReasoningStreamEmitter) Finish() string {
	if e == nil {
		return ""
	}
	display := openai.DisplayReasoningContent(strings.TrimSpace(e.rawBuf))
	if display == "" || e.streamID == "" || e.progress == nil {
		return display
	}
	e.progress("reasoning_chain_stream_end", display, map[string]interface{}{
		"streamId":       e.streamID,
		"conversationId": e.conversation,
		"source":         "eino",
		"einoAgent":      e.agentName,
		"einoRole":       e.einoRole,
		"orchestration":  e.orchMode,
	})
	return display
}

func (e *einoReasoningStreamEmitter) EmitComplete(reasoningContent string) bool {
	if e == nil || e.progress == nil {
		return false
	}
	display := openai.DisplayReasoningContent(strings.TrimSpace(reasoningContent))
	if display == "" {
		return false
	}
	e.progress("reasoning_chain", display, map[string]interface{}{
		"conversationId": e.conversation,
		"source":         "eino",
		"einoAgent":      e.agentName,
		"einoRole":       e.einoRole,
		"orchestration":  e.orchMode,
	})
	return true
}
