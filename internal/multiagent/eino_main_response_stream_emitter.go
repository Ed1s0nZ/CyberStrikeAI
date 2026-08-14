package multiagent

import "cyberstrike-ai/internal/openai"

type einoMainResponseStreamEmitter struct {
	progress       func(eventType, message string, data interface{})
	snapshotMCPIDs func() []string
	conversationID string
	orchMode       string
	agentName      string
	streamID       string
	iteration      int
	headerSent     bool
	wireAccum      string
}

func newEinoMainResponseStreamEmitter(
	conversationID, orchMode, agentName, streamID string,
	iteration int,
	progress func(eventType, message string, data interface{}),
	snapshotMCPIDs func() []string,
) *einoMainResponseStreamEmitter {
	if snapshotMCPIDs == nil {
		snapshotMCPIDs = func() []string { return nil }
	}
	return &einoMainResponseStreamEmitter{
		progress:       progress,
		snapshotMCPIDs: snapshotMCPIDs,
		conversationID: conversationID,
		orchMode:       orchMode,
		agentName:      agentName,
		streamID:       streamID,
		iteration:      iteration,
	}
}

func (e *einoMainResponseStreamEmitter) EmitDelta(delta, accumulated string) bool {
	if e == nil || e.progress == nil || delta == "" {
		return false
	}
	e.emitStart()
	e.progress("response_delta", delta, openai.WithSSEAccumulated(e.responseData(), accumulated))
	e.wireAccum, _ = normalizeStreamingDelta(e.wireAccum, delta)
	return true
}

func (e *einoMainResponseStreamEmitter) EmitTailFromFull(full string) bool {
	if e == nil || full == "" {
		return false
	}
	_, tail := normalizeStreamingDelta(e.wireAccum, full)
	if tail == "" {
		return false
	}
	return e.EmitDelta(tail, full)
}

func (e *einoMainResponseStreamEmitter) emitStart() {
	if e.headerSent || e.progress == nil {
		return
	}
	e.progress("response_start", "", map[string]interface{}{
		"conversationId":     e.conversationID,
		"mcpExecutionIds":    e.snapshotMCPIDs(),
		"messageGeneratedBy": "eino:" + e.agentName,
		"einoRole":           "orchestrator",
		"einoAgent":          e.agentName,
		"orchestration":      e.orchMode,
		"iteration":          e.iteration,
		"streamId":           e.streamID,
	})
	e.headerSent = true
}

func (e *einoMainResponseStreamEmitter) responseData() map[string]interface{} {
	return map[string]interface{}{
		"conversationId":  e.conversationID,
		"mcpExecutionIds": e.snapshotMCPIDs(),
		"einoRole":        "orchestrator",
		"einoAgent":       e.agentName,
		"orchestration":   e.orchMode,
		"iteration":       e.iteration,
		"streamId":        e.streamID,
	}
}
