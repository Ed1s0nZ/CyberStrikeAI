package multiagent

import "strings"

type einoMainAssistantStreamHandler struct {
	agentName        string
	emitter          *einoMainResponseStreamEmitter
	stdoutSuppressor *einoExecuteStdoutSuppressor
	assistantOutput  *einoAssistantOutputAccumulator
	runMessages      *einoRunMessageAccumulator

	buf       string
	dupTarget string
}

type einoMainAssistantStreamHandlerConfig struct {
	AgentName        string
	Emitter          *einoMainResponseStreamEmitter
	StdoutSuppressor *einoExecuteStdoutSuppressor
	AssistantOutput  *einoAssistantOutputAccumulator
	RunMessages      *einoRunMessageAccumulator
}

func newEinoMainAssistantStreamHandler(cfg einoMainAssistantStreamHandlerConfig) *einoMainAssistantStreamHandler {
	return &einoMainAssistantStreamHandler{
		agentName:        cfg.AgentName,
		emitter:          cfg.Emitter,
		stdoutSuppressor: cfg.StdoutSuppressor,
		assistantOutput:  cfg.AssistantOutput,
		runMessages:      cfg.RunMessages,
	}
}

func (h *einoMainAssistantStreamHandler) EmitDelta(content string) bool {
	if h == nil || content == "" {
		return false
	}
	var delta string
	h.buf, delta = normalizeStreamingDelta(h.buf, content)
	if delta == "" {
		return false
	}
	if h.dupTarget == "" && h.stdoutSuppressor != nil {
		h.dupTarget = h.stdoutSuppressor.Peek()
	}
	if h.dupTarget != "" {
		return false
	}
	return h.emitter.EmitDelta(delta, h.buf)
}

func (h *einoMainAssistantStreamHandler) Finish() string {
	if h == nil {
		return ""
	}
	body := strings.TrimSpace(h.buf)
	if body == "" {
		return ""
	}
	if h.dupTarget != "" {
		if h.stdoutSuppressor != nil {
			h.stdoutSuppressor.Clear()
		}
		if body != h.dupTarget {
			h.emitter.EmitTailFromFull(h.buf)
		}
	} else {
		h.emitter.EmitTailFromFull(h.buf)
	}
	if h.assistantOutput != nil {
		h.assistantOutput.RecordMainAssistant(h.agentName, body)
	}
	if h.runMessages != nil {
		h.runMessages.AppendAssistantText(body)
	}
	return body
}
