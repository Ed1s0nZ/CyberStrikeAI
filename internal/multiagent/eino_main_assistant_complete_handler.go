package multiagent

import "strings"

type einoMainAssistantCompleteHandler struct {
	agentName        string
	emitter          *einoMainResponseStreamEmitter
	stdoutSuppressor *einoExecuteStdoutSuppressor
	assistantOutput  *einoAssistantOutputAccumulator
}

type einoMainAssistantCompleteHandlerConfig struct {
	AgentName        string
	Emitter          *einoMainResponseStreamEmitter
	StdoutSuppressor *einoExecuteStdoutSuppressor
	AssistantOutput  *einoAssistantOutputAccumulator
}

func newEinoMainAssistantCompleteHandler(cfg einoMainAssistantCompleteHandlerConfig) *einoMainAssistantCompleteHandler {
	return &einoMainAssistantCompleteHandler{
		agentName:        cfg.AgentName,
		emitter:          cfg.Emitter,
		stdoutSuppressor: cfg.StdoutSuppressor,
		assistantOutput:  cfg.AssistantOutput,
	}
}

func (h *einoMainAssistantCompleteHandler) EmitComplete(content string) bool {
	if h == nil {
		return false
	}
	body := strings.TrimSpace(content)
	if body == "" {
		return false
	}
	if h.stdoutSuppressor != nil {
		if dup := h.stdoutSuppressor.Consume(); dup != "" && body == dup {
			if h.assistantOutput != nil {
				h.assistantOutput.RecordMainAssistant(h.agentName, body)
			}
			return false
		}
	}
	emitted := h.emitter.EmitDelta(body, body)
	if h.assistantOutput != nil {
		h.assistantOutput.RecordMainAssistant(h.agentName, body)
	}
	return emitted
}
