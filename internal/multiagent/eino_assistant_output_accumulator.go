package multiagent

import "strings"

type einoAssistantOutputAccumulator struct {
	orchMode                string
	lastAssistant           string
	lastPlanExecuteExecutor string
}

func newEinoAssistantOutputAccumulator(orchMode string) *einoAssistantOutputAccumulator {
	return &einoAssistantOutputAccumulator{orchMode: orchMode}
}

func (a *einoAssistantOutputAccumulator) RecordMainAssistant(agentName, content string) bool {
	if a == nil {
		return false
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return false
	}
	a.lastAssistant = content
	if a.orchMode == "plan_execute" && strings.EqualFold(strings.TrimSpace(agentName), "executor") {
		a.lastPlanExecuteExecutor = UnwrapPlanExecuteUserText(content)
	}
	return true
}

func (a *einoAssistantOutputAccumulator) LastAssistant() string {
	if a == nil {
		return ""
	}
	return a.lastAssistant
}

func (a *einoAssistantOutputAccumulator) LastPlanExecuteExecutor() string {
	if a == nil {
		return ""
	}
	return a.lastPlanExecuteExecutor
}
