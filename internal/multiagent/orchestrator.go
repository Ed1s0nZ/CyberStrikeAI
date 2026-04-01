// Package multiagent provides a native Go multi-agent orchestrator that replaces
// the CloudWeGo Eino framework. It uses the existing Agent's LLM client and MCP
// tool execution, adding virtual "task" and "write_todos" tools for sub-agent
// delegation and progress tracking. SSE event types match the frontend contract.
package multiagent

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"cyberstrike-ai/internal/agent"
	"cyberstrike-ai/internal/agents"
	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/security"

	"go.uber.org/zap"
)

// RunResult aligns with single-Agent loop result fields for reuse in storage and SSE finalization logic.
type RunResult struct {
	Response        string
	MCPExecutionIDs []string
	LastReActInput  string
	LastReActOutput string
}

const orchestratorDefaultPrompt = `You are the CyberStrikeAI orchestrator. You coordinate specialist security agents and tools for authorized penetration testing.

Your capabilities:
- Directly use any security tool (nmap, nuclei, subfinder, etc.)
- Delegate complex subtasks to specialist sub-agents via the "task" tool
- Track progress with the "write_todos" tool

Workflow:
1. Analyze the user's request
2. Create a todo list (write_todos) to plan your approach
3. Execute tasks directly or delegate to sub-agents
4. Update todos as you complete each step
5. Synthesize results into a final report

When to use the "task" tool:
- The subtask is independent and can run in parallel
- The subtask requires deep focus (e.g., full port scan + service enumeration)
- You want to isolate a specific testing phase

When NOT to use "task":
- Simple tool calls you can make directly
- Tasks that depend on results from other tasks (do them sequentially)

CRITICAL LANGUAGE RULE: You MUST respond ONLY in English. All output — including todo lists, task descriptions, status updates, tool arguments, reports, and every other piece of text — MUST be in English. NEVER use Chinese, Russian, or any other non-English language. This is a hard requirement with zero exceptions.

Be thorough and persistent — real vulnerability hunting requires extensive testing.`

// subAgentDef holds a resolved sub-agent definition used at runtime.
type subAgentDef struct {
	id          string
	name        string
	description string
	instruction string
	roleTools   []string
	maxIter     int
}

// RunDeepAgent executes one round of conversation using the native Go orchestrator.
// Streaming events are emitted via the progress callback using the same event types
// the frontend expects, maintaining full compatibility with the previous Eino-based
// implementation.
func RunDeepAgent(
	ctx context.Context,
	appCfg *config.Config,
	ma *config.MultiAgentConfig,
	ag *agent.Agent,
	logger *zap.Logger,
	conversationID string,
	userMessage string,
	history []agent.ChatMessage,
	roleTools []string,
	progress func(eventType, message string, data interface{}),
	agentsMarkdownDir string,
) (*RunResult, error) {
	if appCfg == nil || ma == nil || ag == nil {
		return nil, fmt.Errorf("multiagent: config or Agent is nil")
	}

	// Resolve sub-agents from config + markdown directory.
	effectiveSubs := ma.SubAgents
	var orch *agents.OrchestratorMarkdown
	if strings.TrimSpace(agentsMarkdownDir) != "" {
		load, merr := agents.LoadMarkdownAgentsDir(agentsMarkdownDir)
		if merr != nil {
			if logger != nil {
				logger.Warn("failed to load agents dir markdown, falling back to config sub_agents", zap.Error(merr))
			}
		} else {
			effectiveSubs = agents.MergeYAMLAndMarkdown(ma.SubAgents, load.SubAgents)
			orch = load.Orchestrator
		}
	}
	if ma.WithoutGeneralSubAgent && len(effectiveSubs) == 0 {
		return nil, fmt.Errorf("multi_agent.without_general_sub_agent is true but no sub-agents configured in multi_agent.sub_agents or agents markdown directory")
	}

	// Build sub-agent definitions.
	subDefaultIter := ma.SubAgentMaxIterations
	if subDefaultIter <= 0 {
		subDefaultIter = 20
	}

	subDefs := make([]subAgentDef, 0, len(effectiveSubs))
	for _, sub := range effectiveSubs {
		id := strings.TrimSpace(sub.ID)
		if id == "" {
			return nil, fmt.Errorf("multi_agent.sub_agents contains entry with empty id")
		}
		name := strings.TrimSpace(sub.Name)
		if name == "" {
			name = id
		}
		desc := strings.TrimSpace(sub.Description)
		if desc == "" {
			desc = fmt.Sprintf("Specialist agent %s for penetration testing workflow.", id)
		}
		instr := strings.TrimSpace(sub.Instruction)
		if instr == "" {
			instr = "You are a specialist sub-agent in CyberStrikeAI, assisting with user-delegated sub-tasks in authorized penetration testing scenarios. Prioritize using available tools to gather evidence, and respond concisely and professionally. Always respond in English only."
		}

		subRoleTools := sub.RoleTools
		bind := strings.TrimSpace(sub.BindRole)
		if bind != "" && appCfg.Roles != nil {
			if r, ok := appCfg.Roles[bind]; ok && r.Enabled {
				if len(subRoleTools) == 0 && len(r.Tools) > 0 {
					subRoleTools = r.Tools
				}
				if len(r.Skills) > 0 {
					var b strings.Builder
					b.WriteString(instr)
					b.WriteString("\n\nRecommended Skills for this role (load on demand via list_skills / read_skill): ")
					for i, s := range r.Skills {
						if i > 0 {
							b.WriteString(", ")
						}
						b.WriteString(s)
					}
					b.WriteString(".")
					instr = b.String()
				}
			}
		}

		subMax := sub.MaxIterations
		if subMax <= 0 {
			subMax = subDefaultIter
		}

		subDefs = append(subDefs, subAgentDef{
			id:          id,
			name:        name,
			description: desc,
			instruction: instr,
			roleTools:   subRoleTools,
			maxIter:     subMax,
		})
	}

	// Determine orchestrator system prompt.
	orchInstruction := strings.TrimSpace(ma.OrchestratorInstruction)
	if orchInstruction == "" {
		orchInstruction = orchestratorDefaultPrompt
	}
	if orch != nil {
		if ins := strings.TrimSpace(orch.Instruction); ins != "" {
			orchInstruction = ins
		}
	}

	// Max orchestrator iterations.
	deepMaxIter := ma.MaxIteration
	if deepMaxIter <= 0 {
		deepMaxIter = appCfg.Agent.MaxIterations
	}
	if deepMaxIter <= 0 {
		deepMaxIter = 40
	}

	o := &orchestratorState{
		ctx:            ctx,
		appCfg:         appCfg,
		ag:             ag,
		logger:         logger,
		conversationID: conversationID,
		progress:       progress,
		subDefs:        subDefs,
		orchInstruction: orchInstruction,
		maxIter:        deepMaxIter,
		roleTools:      roleTools,
	}

	return o.run(userMessage, history)
}

// orchestratorState holds all state for a single orchestrator run.
type orchestratorState struct {
	ctx            context.Context
	appCfg         *config.Config
	ag             *agent.Agent
	logger         *zap.Logger
	conversationID string
	progress       func(eventType, message string, data interface{})
	subDefs        []subAgentDef
	orchInstruction string
	maxIter        int
	roleTools      []string

	// Mutable state, protected by mu.
	mu     sync.Mutex
	mcpIDs []string
	todos  []todoItem
}

type todoItem struct {
	Content string `json:"content"`
	Status  string `json:"status"`
}

func (o *orchestratorState) recordMCPID(id string) {
	if id == "" {
		return
	}
	o.mu.Lock()
	o.mcpIDs = append(o.mcpIDs, id)
	o.mu.Unlock()
}

func (o *orchestratorState) snapshotMCPIDs() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := make([]string, len(o.mcpIDs))
	copy(out, o.mcpIDs)
	return out
}

func (o *orchestratorState) sendProgress(eventType, message string, data interface{}) {
	if o.progress != nil {
		o.progress(eventType, message, data)
	}
}

func (o *orchestratorState) run(userMessage string, history []agent.ChatMessage) (*RunResult, error) {
	// Build tool definitions: real MCP tools + virtual tools.
	mainDefs := o.ag.ToolsForRole(o.roleTools)

	// Build sub-agent list string for the task tool description.
	var subAgentListParts []string
	for _, sd := range o.subDefs {
		subAgentListParts = append(subAgentListParts, fmt.Sprintf("%s (%s)", sd.id, sd.description))
	}
	subAgentList := strings.Join(subAgentListParts, ", ")
	if subAgentList == "" {
		subAgentList = "general (default general-purpose agent)"
	}

	// Build the tools array: real tools + virtual tools.
	allTools := make([]agent.Tool, 0, len(mainDefs)+2)
	allTools = append(allTools, mainDefs...)

	// "task" virtual tool.
	taskTool := agent.Tool{
		Type: "function",
		Function: agent.FunctionDefinition{
			Name:        "task",
			Description: "Launch a specialist sub-agent to handle an independent subtask autonomously. The agent will use available tools and return results. Available agents: " + subAgentList,
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"agent_id": map[string]interface{}{
						"type":        "string",
						"description": "ID of the sub-agent to use. Available: " + subAgentList,
					},
					"task_description": map[string]interface{}{
						"type":        "string",
						"description": "Clear description of what the sub-agent should accomplish",
					},
				},
				"required": []interface{}{"task_description"},
			},
		},
	}
	allTools = append(allTools, taskTool)

	// "write_todos" virtual tool.
	todoTool := agent.Tool{
		Type: "function",
		Function: agent.FunctionDefinition{
			Name:        "write_todos",
			Description: "Create or update a structured todo list for the current session. Use this to plan multi-step tasks and track progress.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"todos": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"content": map[string]interface{}{"type": "string", "description": "Task description"},
								"status":  map[string]interface{}{"type": "string", "enum": []interface{}{"pending", "in_progress", "completed"}, "description": "Task status"},
							},
						},
					},
				},
				"required": []interface{}{"todos"},
			},
		},
	}
	allTools = append(allTools, todoTool)

	// Build message history.
	messages := []agent.ChatMessage{
		{Role: "system", Content: o.orchInstruction},
	}

	const maxHistoryMessages = 300
	hist := history
	if len(hist) > maxHistoryMessages {
		hist = hist[len(hist)-maxHistoryMessages:]
	}
	for _, h := range hist {
		switch h.Role {
		case "user":
			if strings.TrimSpace(h.Content) != "" {
				messages = append(messages, agent.ChatMessage{Role: "user", Content: h.Content})
			}
		case "assistant":
			if strings.TrimSpace(h.Content) != "" {
				messages = append(messages, agent.ChatMessage{Role: "assistant", Content: h.Content})
			}
		}
	}
	// Prepend language enforcement directly in the user message — Haiku ignores system-level
	// language instructions but reliably follows user-level ones.
	messages = append(messages, agent.ChatMessage{Role: "user", Content: "[IMPORTANT: Respond ONLY in English. All output must be English.]\n\n" + userMessage})

	var lastAssistant string
	var reasoningStreamSeq int64

	// Main orchestrator loop.
	o.sendProgress("iteration", "", map[string]interface{}{
		"iteration":      1,
		"einoScope":      "main",
		"einoRole":       "orchestrator",
		"einoAgent":      "cyberstrike-orchestrator",
		"conversationId": o.conversationID,
		"source":         "native",
	})

	for i := 0; i < o.maxIter; i++ {
		select {
		case <-o.ctx.Done():
			return o.buildResult(lastAssistant, messages), o.ctx.Err()
		default:
		}

		if i > 0 {
			o.sendProgress("iteration", "", map[string]interface{}{
				"iteration":      i + 1,
				"einoScope":      "main",
				"einoRole":       "orchestrator",
				"einoAgent":      "cyberstrike-orchestrator",
				"conversationId": o.conversationID,
				"source":         "native",
			})
		}

		o.sendProgress("progress", "calling AI model...", nil)

		// Streaming LLM call.
		thinkingStreamID := fmt.Sprintf("thinking-stream-%s-%d-%d", o.conversationID, i+1, atomic.AddInt64(&reasoningStreamSeq, 1))
		thinkingStarted := false

		response, err := o.ag.CallStreamWithToolCalls(o.ctx, messages, allTools, func(delta string) error {
			if delta == "" {
				return nil
			}
			if !thinkingStarted {
				thinkingStarted = true
				o.sendProgress("thinking_stream_start", " ", map[string]interface{}{
					"streamId":  thinkingStreamID,
					"source":    "native",
					"einoAgent": "cyberstrike-orchestrator",
					"einoRole":  "orchestrator",
				})
			}
			o.sendProgress("thinking_stream_delta", delta, map[string]interface{}{
				"streamId": thinkingStreamID,
			})
			return nil
		})
		if err != nil {
			o.sendProgress("error", err.Error(), map[string]interface{}{
				"conversationId": o.conversationID,
				"source":         "native",
			})
			return o.buildResult(lastAssistant, messages), err
		}
		if response == nil || len(response.Choices) == 0 {
			return o.buildResult(lastAssistant, messages), fmt.Errorf("no response received from LLM")
		}

		choice := response.Choices[0]

		// If tool calls present, process them.
		if len(choice.Message.ToolCalls) > 0 {
			// Emit thinking content if present.
			if choice.Message.Content != "" {
				o.sendProgress("thinking", choice.Message.Content, map[string]interface{}{
					"iteration": i + 1,
					"streamId":  thinkingStreamID,
				})
			}

			// Add assistant message (with tool calls) to history.
			messages = append(messages, agent.ChatMessage{
				Role:      "assistant",
				Content:   choice.Message.Content,
				ToolCalls: choice.Message.ToolCalls,
			})

			o.sendProgress("tool_calls_detected", fmt.Sprintf("Detected %d tool call(s)", len(choice.Message.ToolCalls)), map[string]interface{}{
				"count":          len(choice.Message.ToolCalls),
				"conversationId": o.conversationID,
				"source":         "native",
				"einoAgent":      "cyberstrike-orchestrator",
				"einoRole":       "orchestrator",
			})

			// Execute each tool call.
			for idx, tc := range choice.Message.ToolCalls {
				toolArgsJSON, _ := json.Marshal(tc.Function.Arguments)

				o.sendProgress("tool_call", fmt.Sprintf("Calling tool: %s", tc.Function.Name), map[string]interface{}{
					"toolName":       tc.Function.Name,
					"arguments":      string(toolArgsJSON),
					"argumentsObj":   tc.Function.Arguments,
					"toolCallId":     tc.ID,
					"index":          idx + 1,
					"total":          len(choice.Message.ToolCalls),
					"conversationId": o.conversationID,
					"source":         "native",
					"einoAgent":      "cyberstrike-orchestrator",
					"einoRole":       "orchestrator",
				})

				var toolResult string
				var isErr bool

				switch tc.Function.Name {
				case "task":
					toolResult, isErr = o.handleTaskCall(tc.Function.Arguments, tc.ID)
				case "write_todos":
					toolResult, isErr = o.handleWriteTodos(tc.Function.Arguments)
				default:
					toolResult, isErr = o.handleMCPTool(tc.Function.Name, tc.Function.Arguments, tc.ID, idx, len(choice.Message.ToolCalls), i+1)
				}

				messages = append(messages, agent.ChatMessage{
					Role:       "tool",
					ToolCallID: tc.ID,
					Content:    toolResult,
				})

				preview := toolResult
				if len(preview) > 200 {
					preview = preview[:200] + "..."
				}
				o.sendProgress("tool_result", fmt.Sprintf("Tool result (%s)", tc.Function.Name), map[string]interface{}{
					"toolName":       tc.Function.Name,
					"success":        !isErr,
					"isError":        isErr,
					"result":         toolResult,
					"resultPreview":  preview,
					"conversationId": o.conversationID,
					"toolCallId":     tc.ID,
					"einoAgent":      "cyberstrike-orchestrator",
					"einoRole":       "orchestrator",
					"source":         "native",
				})
			}

			// Check if this was the final iteration — force summary.
			if i == o.maxIter-1 {
				summaryText := o.forceSummary(messages)
				if summaryText != "" {
					lastAssistant = summaryText
				}
			}
			continue
		}

		// No tool calls — this is the final text response.
		body := strings.TrimSpace(choice.Message.Content)
		if body != "" {
			o.sendProgress("response_start", "", map[string]interface{}{
				"conversationId":   o.conversationID,
				"mcpExecutionIds":  o.snapshotMCPIDs(),
				"messageGeneratedBy": "native:orchestrator",
				"einoRole":         "orchestrator",
			})
			o.sendProgress("response_delta", body, map[string]interface{}{
				"conversationId":  o.conversationID,
				"mcpExecutionIds": o.snapshotMCPIDs(),
				"einoRole":        "orchestrator",
			})
			lastAssistant = body
		}

		messages = append(messages, agent.ChatMessage{
			Role:    "assistant",
			Content: choice.Message.Content,
		})

		if choice.FinishReason == "stop" || len(choice.Message.ToolCalls) == 0 {
			break
		}
	}

	return o.buildResult(lastAssistant, messages), nil
}

// handleTaskCall processes the virtual "task" tool: spawns a sub-agent loop.
func (o *orchestratorState) handleTaskCall(args map[string]interface{}, toolCallID string) (string, bool) {
	agentID, _ := args["agent_id"].(string)
	taskDesc, _ := args["task_description"].(string)
	if strings.TrimSpace(taskDesc) == "" {
		return "Error: task_description is required", true
	}

	// Find matching sub-agent definition.
	var matched *subAgentDef
	agentID = strings.TrimSpace(agentID)
	for i := range o.subDefs {
		if strings.EqualFold(o.subDefs[i].id, agentID) || strings.EqualFold(o.subDefs[i].name, agentID) {
			matched = &o.subDefs[i]
			break
		}
	}
	// If no match, use first available or run with default tools.
	if matched == nil && len(o.subDefs) > 0 {
		matched = &o.subDefs[0]
	}

	subInstruction := "You are a specialist sub-agent in CyberStrikeAI. Complete the assigned task using available tools. Be thorough and report results concisely. Always respond in English only — never use Chinese or any other language."
	subRoleTools := o.roleTools
	subMaxIter := 20
	subAgentName := "general"

	if matched != nil {
		subInstruction = matched.instruction
		subRoleTools = matched.roleTools
		subMaxIter = matched.maxIter
		subAgentName = matched.id
	}

	o.sendProgress("progress", fmt.Sprintf("[sub-agent: %s] starting task...", subAgentName), map[string]interface{}{
		"conversationId": o.conversationID,
		"einoAgent":      subAgentName,
		"einoRole":       "sub",
	})

	result, err := o.runSubAgent(subAgentName, subInstruction, taskDesc, subRoleTools, subMaxIter)
	if err != nil {
		errMsg := fmt.Sprintf("Sub-agent %s failed: %s", subAgentName, err.Error())
		o.sendProgress("eino_agent_reply", errMsg, map[string]interface{}{
			"conversationId": o.conversationID,
			"einoAgent":      subAgentName,
			"einoRole":       "sub",
			"source":         "native",
		})
		return errMsg, true
	}

	o.sendProgress("eino_agent_reply", result, map[string]interface{}{
		"conversationId": o.conversationID,
		"einoAgent":      subAgentName,
		"einoRole":       "sub",
		"source":         "native",
	})

	return result, false
}

// runSubAgent executes a mini agent loop for a sub-agent task.
func (o *orchestratorState) runSubAgent(agentName, instruction, taskDesc string, subRoleTools []string, maxIter int) (string, error) {
	subTools := o.ag.ToolsForRole(subRoleTools)

	messages := []agent.ChatMessage{
		{Role: "system", Content: instruction},
		{Role: "user", Content: fmt.Sprintf("[IMPORTANT: Respond ONLY in English.]\n\nComplete this task:\n\n%s\n\nUse available tools. Be thorough. Report results concisely.", taskDesc)},
	}

	var lastContent string

	for i := 0; i < maxIter; i++ {
		select {
		case <-o.ctx.Done():
			if lastContent != "" {
				return lastContent, nil
			}
			return "", o.ctx.Err()
		default:
		}

		o.sendProgress("iteration", "", map[string]interface{}{
			"iteration":      i + 1,
			"einoScope":      "sub",
			"einoRole":       "sub",
			"einoAgent":      agentName,
			"conversationId": o.conversationID,
			"source":         "native",
		})

		response, err := o.ag.CallStreamWithToolCalls(o.ctx, messages, subTools, func(delta string) error {
			// Sub-agent thinking deltas are silently consumed; we only report the final result.
			return nil
		})
		if err != nil {
			if lastContent != "" {
				return lastContent, nil
			}
			return "", err
		}
		if response == nil || len(response.Choices) == 0 {
			break
		}

		choice := response.Choices[0]

		if len(choice.Message.ToolCalls) > 0 {
			if choice.Message.Content != "" {
				lastContent = choice.Message.Content
			}

			messages = append(messages, agent.ChatMessage{
				Role:      "assistant",
				Content:   choice.Message.Content,
				ToolCalls: choice.Message.ToolCalls,
			})

			o.sendProgress("tool_calls_detected", fmt.Sprintf("Detected %d tool call(s)", len(choice.Message.ToolCalls)), map[string]interface{}{
				"count":          len(choice.Message.ToolCalls),
				"conversationId": o.conversationID,
				"source":         "native",
				"einoAgent":      agentName,
				"einoRole":       "sub",
			})

			for idx, tc := range choice.Message.ToolCalls {
				// Sub-agents cannot call "task" (prevent infinite recursion).
				if tc.Function.Name == "task" {
					messages = append(messages, agent.ChatMessage{
						Role:       "tool",
						ToolCallID: tc.ID,
						Content:    "Nested task delegation is forbidden (already inside a sub-agent delegation chain) to avoid infinite delegation. Please continue the work using the current agent's tools.",
					})
					continue
				}

				toolArgsJSON, _ := json.Marshal(tc.Function.Arguments)
				o.sendProgress("tool_call", fmt.Sprintf("Calling tool: %s", tc.Function.Name), map[string]interface{}{
					"toolName":       tc.Function.Name,
					"arguments":      string(toolArgsJSON),
					"argumentsObj":   tc.Function.Arguments,
					"toolCallId":     tc.ID,
					"index":          idx + 1,
					"total":          len(choice.Message.ToolCalls),
					"conversationId": o.conversationID,
					"source":         "native",
					"einoAgent":      agentName,
					"einoRole":       "sub",
				})

				toolResult, isErr := o.handleMCPTool(tc.Function.Name, tc.Function.Arguments, tc.ID, idx, len(choice.Message.ToolCalls), i+1)

				messages = append(messages, agent.ChatMessage{
					Role:       "tool",
					ToolCallID: tc.ID,
					Content:    toolResult,
				})

				preview := toolResult
				if len(preview) > 200 {
					preview = preview[:200] + "..."
				}
				o.sendProgress("tool_result", fmt.Sprintf("Tool result (%s)", tc.Function.Name), map[string]interface{}{
					"toolName":       tc.Function.Name,
					"success":        !isErr,
					"isError":        isErr,
					"result":         toolResult,
					"resultPreview":  preview,
					"conversationId": o.conversationID,
					"toolCallId":     tc.ID,
					"einoAgent":      agentName,
					"einoRole":       "sub",
					"source":         "native",
				})
			}

			if i == maxIter-1 {
				// Last iteration: ask for summary.
				messages = append(messages, agent.ChatMessage{
					Role:    "user",
					Content: "This is your final iteration. Please summarize your findings and results concisely.",
				})
				summaryText, _ := o.ag.CallStreamText(o.ctx, messages, []agent.Tool{}, func(delta string) error { return nil })
				if strings.TrimSpace(summaryText) != "" {
					return summaryText, nil
				}
			}
			continue
		}

		// No tool calls — sub-agent is done.
		body := strings.TrimSpace(choice.Message.Content)
		if body != "" {
			return body, nil
		}
		if choice.FinishReason == "stop" {
			break
		}
	}

	if lastContent != "" {
		return lastContent, nil
	}
	return "(sub-agent completed without producing a text response)", nil
}

// handleWriteTodos processes the virtual "write_todos" tool.
func (o *orchestratorState) handleWriteTodos(args map[string]interface{}) (string, bool) {
	todosRaw, ok := args["todos"]
	if !ok {
		return "Error: todos field is required", true
	}

	todosJSON, err := json.Marshal(todosRaw)
	if err != nil {
		return "Error: invalid todos format", true
	}

	var todos []todoItem
	if err := json.Unmarshal(todosJSON, &todos); err != nil {
		return "Error: invalid todos format: " + err.Error(), true
	}

	o.mu.Lock()
	o.todos = todos
	o.mu.Unlock()

	// Emit todos as progress events for the frontend.
	todosData := make([]map[string]interface{}, len(todos))
	for i, t := range todos {
		todosData[i] = map[string]interface{}{
			"content": t.Content,
			"status":  t.Status,
		}
	}
	o.sendProgress("todos", "", map[string]interface{}{
		"todos":          todosData,
		"conversationId": o.conversationID,
		"source":         "native",
	})

	return fmt.Sprintf("Todo list updated (%d items)", len(todos)), false
}

// handleMCPTool executes a regular MCP tool via the agent.
func (o *orchestratorState) handleMCPTool(toolName string, args map[string]interface{}, toolCallID string, idx, total, iteration int) (string, bool) {
	if args == nil {
		args = map[string]interface{}{}
	}

	// Set up tool output streaming callback.
	toolCtx := context.WithValue(o.ctx, security.ToolOutputCallbackCtxKey, security.ToolOutputCallback(func(chunk string) {
		if strings.TrimSpace(chunk) == "" {
			return
		}
		o.sendProgress("tool_result_delta", chunk, map[string]interface{}{
			"toolName":   toolName,
			"toolCallId": toolCallID,
			"index":      idx + 1,
			"total":      total,
			"iteration":  iteration,
			"source":     "native",
		})
	}))

	res, err := o.ag.ExecuteMCPToolForConversation(toolCtx, o.conversationID, toolName, args)
	if err != nil {
		return fmt.Sprintf("Tool execution error: %s", err.Error()), true
	}
	if res == nil {
		return "", false
	}
	if res.ExecutionID != "" {
		o.recordMCPID(res.ExecutionID)
	}
	return res.Result, res.IsError
}

// forceSummary asks the LLM for a final summary when max iterations are reached.
func (o *orchestratorState) forceSummary(messages []agent.ChatMessage) string {
	o.sendProgress("progress", "max iterations reached, generating summary...", nil)

	summaryMessages := make([]agent.ChatMessage, len(messages))
	copy(summaryMessages, messages)
	summaryMessages = append(summaryMessages, agent.ChatMessage{
		Role:    "user",
		Content: "Maximum iteration count reached. Please summarize all test results, discovered issues, and completed work so far. If further testing is needed, provide a detailed plan for next steps. Reply directly without calling tools.",
	})

	o.sendProgress("response_start", "", map[string]interface{}{
		"conversationId":   o.conversationID,
		"mcpExecutionIds":  o.snapshotMCPIDs(),
		"messageGeneratedBy": "max_iter_summary",
	})

	streamText, _ := o.ag.CallStreamText(o.ctx, summaryMessages, []agent.Tool{}, func(delta string) error {
		o.sendProgress("response_delta", delta, map[string]interface{}{
			"conversationId": o.conversationID,
		})
		return nil
	})
	return strings.TrimSpace(streamText)
}

// buildResult constructs the final RunResult.
func (o *orchestratorState) buildResult(lastAssistant string, messages []agent.ChatMessage) *RunResult {
	o.mu.Lock()
	ids := make([]string, len(o.mcpIDs))
	copy(ids, o.mcpIDs)
	o.mu.Unlock()

	histJSON, _ := json.Marshal(messages)
	cleaned := strings.TrimSpace(lastAssistant)
	cleaned = dedupeRepeatedParagraphs(cleaned, 80)
	cleaned = dedupeParagraphsByLineFingerprint(cleaned, 100)

	out := &RunResult{
		Response:        cleaned,
		MCPExecutionIDs: ids,
		LastReActInput:  string(histJSON),
		LastReActOutput: cleaned,
	}
	if out.Response == "" {
		out.Response = "(Orchestrator completed, but no assistant text output was captured. Please check process details or logs.)"
		out.LastReActOutput = out.Response
	}
	return out
}

// dedupeRepeatedParagraphs removes identical consecutive/repeated paragraphs.
func dedupeRepeatedParagraphs(s string, minLen int) string {
	if s == "" || minLen <= 0 {
		return s
	}
	paras := strings.Split(s, "\n\n")
	var out []string
	seen := make(map[string]bool)
	for _, p := range paras {
		t := strings.TrimSpace(p)
		if len(t) < minLen {
			out = append(out, p)
			continue
		}
		if seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, p)
	}
	return strings.TrimSpace(strings.Join(out, "\n\n"))
}

// dedupeParagraphsByLineFingerprint removes paragraphs whose body line sets are identical.
func dedupeParagraphsByLineFingerprint(s string, minParaLen int) string {
	if s == "" || minParaLen <= 0 {
		return s
	}
	paras := strings.Split(s, "\n\n")
	var out []string
	seen := make(map[string]bool)
	for _, p := range paras {
		t := strings.TrimSpace(p)
		if len(t) < minParaLen {
			out = append(out, p)
			continue
		}
		fp := paragraphLineFingerprint(t)
		if fp == "" {
			out = append(out, p)
			continue
		}
		if seen[fp] {
			continue
		}
		seen[fp] = true
		out = append(out, p)
	}
	return strings.TrimSpace(strings.Join(out, "\n\n"))
}

func paragraphLineFingerprint(t string) string {
	lines := strings.Split(t, "\n")
	norm := make([]string, 0, len(lines))
	for _, L := range lines {
		s := strings.TrimSpace(L)
		if s == "" {
			continue
		}
		norm = append(norm, s)
	}
	if len(norm) < 4 {
		return ""
	}
	sort.Strings(norm)
	return strings.Join(norm, "\x1e")
}
