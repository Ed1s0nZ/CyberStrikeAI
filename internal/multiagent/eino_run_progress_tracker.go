package multiagent

import (
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"
)

type einoRunProgressTracker struct {
	orchMode         string
	orchestratorName string
	conversationID   string
	progress         func(eventType, message string, data interface{})

	streamsMainAssistant func(agent string) bool
	einoRoleTag          func(agent string) string

	mainRound         int
	lastAgent         string
	toolEmitSeen      map[string]struct{}
	subAgentToolStep  map[string]int
	mainAgentToolStep map[string]int
}

func newEinoRunProgressTracker(
	orchMode, orchestratorName, conversationID string,
	progress func(eventType, message string, data interface{}),
	streamsMainAssistant func(agent string) bool,
	einoRoleTag func(agent string) string,
) *einoRunProgressTracker {
	if streamsMainAssistant == nil {
		streamsMainAssistant = func(agent string) bool {
			return agent == "" || agent == orchestratorName
		}
	}
	if einoRoleTag == nil {
		einoRoleTag = func(agent string) string {
			if streamsMainAssistant(agent) {
				return "orchestrator"
			}
			return "sub"
		}
	}
	return &einoRunProgressTracker{
		orchMode:             orchMode,
		orchestratorName:     orchestratorName,
		conversationID:       conversationID,
		progress:             progress,
		streamsMainAssistant: streamsMainAssistant,
		einoRoleTag:          einoRoleTag,
		toolEmitSeen:         make(map[string]struct{}),
		subAgentToolStep:     make(map[string]int),
		mainAgentToolStep:    make(map[string]int),
	}
}

func (t *einoRunProgressTracker) ObserveAgent(agentName string) {
	if t == nil || strings.TrimSpace(agentName) == "" || t.progress == nil {
		return
	}
	iterEinoAgent := t.orchestratorName
	if t.orchMode == "plan_execute" {
		if a := strings.TrimSpace(agentName); a != "" {
			iterEinoAgent = a
		}
	}
	if t.streamsMainAssistant(agentName) {
		mainIterKey := einoMainIterationKey(iterEinoAgent, t.orchestratorName)
		if t.mainRound == 0 {
			t.mainRound = 1
			t.mainAgentToolStep[mainIterKey] = 1
			t.emitMainIteration(iterEinoAgent, t.mainRound)
		} else if t.lastAgent != "" {
			needBump := false
			if !t.streamsMainAssistant(t.lastAgent) {
				needBump = true
			} else if t.lastAgent != agentName {
				needBump = true
			}
			if needBump {
				t.mainRound++
				t.mainAgentToolStep[mainIterKey] = t.mainRound
				t.emitMainIteration(iterEinoAgent, t.mainRound)
			}
		}
	}
	if t.lastAgent != agentName {
		t.progress("progress", fmt.Sprintf("[Eino] %s", agentName), map[string]interface{}{
			"conversationId": t.conversationID,
			"einoAgent":      agentName,
			"einoRole":       t.einoRoleTag(agentName),
			"orchestration":  t.orchMode,
		})
	}
	t.lastAgent = agentName
}

func (t *einoRunProgressTracker) MainIteration(agentName string) int {
	if t == nil {
		return 0
	}
	key := einoMainIterationKey(agentName, t.orchestratorName)
	if n := t.mainAgentToolStep[key]; n > 0 {
		return n
	}
	return t.mainRound
}

func (t *einoRunProgressTracker) EmitToolCalls(msg *schema.Message, agentName string, markPending func(toolCallPendingInfo)) {
	if t == nil {
		return
	}
	before := t.MainIteration(agentName)
	tryEmitToolCallsOnce(
		msg,
		agentName,
		t.orchestratorName,
		t.conversationID,
		t.orchMode,
		t.progress,
		t.toolEmitSeen,
		t.subAgentToolStep,
		t.mainAgentToolStep,
		markPending,
	)
	if t.streamsMainAssistant(agentName) {
		if after := t.MainIteration(agentName); after > before {
			t.mainRound = after
		}
	}
}

func (t *einoRunProgressTracker) emitMainIteration(agentName string, iteration int) {
	t.progress("iteration", "", map[string]interface{}{
		"iteration":      iteration,
		"einoScope":      "main",
		"einoRole":       "orchestrator",
		"einoAgent":      agentName,
		"orchestration":  t.orchMode,
		"conversationId": t.conversationID,
		"source":         "eino",
	})
}
