package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/mcp"
	"cyberstrike-ai/internal/mcp/builtin"
	"cyberstrike-ai/internal/openai"
	"cyberstrike-ai/internal/storage"

	"go.uber.org/zap"
)

// Agent represents an AI agent
type Agent struct {
	openAIClient          *openai.Client
	config                *config.OpenAIConfig
	agentConfig           *config.AgentConfig
	memoryCompressor      *MemoryCompressor
	mcpServer             *mcp.Server
	externalMCPMgr        *mcp.ExternalMCPManager // external MCP manager
	logger                *zap.Logger
	maxIterations         int
	resultStorage         ResultStorage     // result storage
	largeResultThreshold  int               // large result threshold (bytes)
	mu                    sync.RWMutex      // mutex to support concurrent updates
	toolNameMapping       map[string]string // tool name mapping: OpenAI format -> original format (for external MCP tools)
	currentConversationID string            // current conversation ID (for automatic passing to tools)
	parallelToolExecution bool              // execute multiple tool calls concurrently
	maxParallelTools      int               // max concurrent tool goroutines (0 = unlimited)
	toolRetryCount        int               // number of retries on transient tool errors
}

// ResultStorage is the result storage interface (uses types from the storage package directly)
type ResultStorage interface {
	SaveResult(executionID string, toolName string, result string) error
	GetResult(executionID string) (string, error)
	GetResultPage(executionID string, page int, limit int) (*storage.ResultPage, error)
	SearchResult(executionID string, keyword string, useRegex bool) ([]string, error)
	FilterResult(executionID string, filter string, useRegex bool) ([]string, error)
	GetResultMetadata(executionID string) (*storage.ResultMetadata, error)
	GetResultPath(executionID string) string
	DeleteResult(executionID string) error
}

// NewAgent creates a new Agent
func NewAgent(cfg *config.OpenAIConfig, agentCfg *config.AgentConfig, mcpServer *mcp.Server, externalMCPMgr *mcp.ExternalMCPManager, logger *zap.Logger, maxIterations int) *Agent {
	// If maxIterations is 0 or negative, use default value 30
	if maxIterations <= 0 {
		maxIterations = 30
	}

	// Set large result threshold, default 50KB
	largeResultThreshold := 50 * 1024
	if agentCfg != nil && agentCfg.LargeResultThreshold > 0 {
		largeResultThreshold = agentCfg.LargeResultThreshold
	}

	// Set result storage directory, default tmp
	resultStorageDir := "tmp"
	if agentCfg != nil && agentCfg.ResultStorageDir != "" {
		resultStorageDir = agentCfg.ResultStorageDir
	}

	// Parallel tool execution: enabled by default
	parallelToolExecution := true
	if agentCfg != nil && !agentCfg.ParallelToolExecution {
		// Only disable when explicitly set to false
		parallelToolExecution = false
	}

	maxParallelTools := 0 // unlimited by default
	if agentCfg != nil && agentCfg.MaxParallelTools > 0 {
		maxParallelTools = agentCfg.MaxParallelTools
	}

	toolRetryCount := 0
	if agentCfg != nil && agentCfg.ToolRetryCount > 0 {
		toolRetryCount = agentCfg.ToolRetryCount
	}

	// Initialize result storage
	var resultStorage ResultStorage
	if resultStorageDir != "" {
		// Import storage package (use interface to avoid circular dependency)
		// Initialize when actually needed
		// Set to nil temporarily, initialize when needed
	}

	// Configure HTTP Transport, optimize connection management and timeout settings
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   300 * time.Second,
			KeepAlive: 300 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   30 * time.Second,
		ResponseHeaderTimeout: 60 * time.Minute, // Response header timeout: increased to handle large responses
		DisableKeepAlives:     false,            // Enable connection reuse
	}

	// Increase timeout to 30 minutes to support long-running AI inference
	// especially when using streaming responses or processing complex tasks
	httpClient := &http.Client{
		Timeout:   30 * time.Minute, // Increased from 5 minutes to 30 minutes
		Transport: transport,
	}
	llmClient := openai.NewClient(cfg, httpClient, logger)

	var memoryCompressor *MemoryCompressor
	if cfg != nil {
		mc, err := NewMemoryCompressor(MemoryCompressorConfig{
			MaxTotalTokens: cfg.MaxTotalTokens,
			OpenAIConfig:   cfg,
			HTTPClient:     httpClient,
			Logger:         logger,
		})
		if err != nil {
			logger.Warn("Failed to initialize MemoryCompressor, context compression will be skipped", zap.Error(err))
		} else {
			memoryCompressor = mc
		}
	} else {
		logger.Warn("OpenAI configuration is empty, cannot initialize MemoryCompressor")
	}

	return &Agent{
		openAIClient:          llmClient,
		config:                cfg,
		agentConfig:           agentCfg,
		memoryCompressor:      memoryCompressor,
		mcpServer:             mcpServer,
		externalMCPMgr:        externalMCPMgr,
		logger:                logger,
		maxIterations:         maxIterations,
		resultStorage:         resultStorage,
		largeResultThreshold:  largeResultThreshold,
		toolNameMapping:       make(map[string]string), // Initialize tool name mapping
		parallelToolExecution: parallelToolExecution,
		maxParallelTools:      maxParallelTools,
		toolRetryCount:        toolRetryCount,
	}
}

// SetResultStorage sets the result storage (used to avoid circular dependencies)
func (a *Agent) SetResultStorage(storage ResultStorage) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.resultStorage = storage
}

// ChatMessage represents a chat message
type ChatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// MarshalJSON custom JSON serialization, converts arguments in tool_calls to JSON strings
func (cm ChatMessage) MarshalJSON() ([]byte, error) {
	// Build serialization structure
	aux := map[string]interface{}{
		"role": cm.Role,
	}

	// Add content (if present)
	if cm.Content != "" {
		aux["content"] = cm.Content
	}

	// Add tool_call_id (if present)
	if cm.ToolCallID != "" {
		aux["tool_call_id"] = cm.ToolCallID
	}

	// Convert tool_calls, converting arguments to JSON strings
	if len(cm.ToolCalls) > 0 {
		toolCallsJSON := make([]map[string]interface{}, len(cm.ToolCalls))
		for i, tc := range cm.ToolCalls {
			// Convert arguments to JSON string
			argsJSON := ""
			if tc.Function.Arguments != nil {
				argsBytes, err := json.Marshal(tc.Function.Arguments)
				if err != nil {
					return nil, err
				}
				argsJSON = string(argsBytes)
			}

			toolCallsJSON[i] = map[string]interface{}{
				"id":   tc.ID,
				"type": tc.Type,
				"function": map[string]interface{}{
					"name":      tc.Function.Name,
					"arguments": argsJSON,
				},
			}
		}
		aux["tool_calls"] = toolCallsJSON
	}

	return json.Marshal(aux)
}

// OpenAIRequest represents an OpenAI API request
type OpenAIRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Tools    []Tool        `json:"tools,omitempty"`
}

// OpenAIResponse represents an OpenAI API response
type OpenAIResponse struct {
	ID      string   `json:"id"`
	Choices []Choice `json:"choices"`
	Error   *Error   `json:"error,omitempty"`
}

// Choice represents a response choice
type Choice struct {
	Message      MessageWithTools `json:"message"`
	FinishReason string           `json:"finish_reason"`
}

// MessageWithTools represents a message with tool calls
type MessageWithTools struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// Tool represents an OpenAI tool definition
type Tool struct {
	Type     string             `json:"type"`
	Function FunctionDefinition `json:"function"`
}

// FunctionDefinition represents a function definition
type FunctionDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// Error represents an OpenAI error
type Error struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

// ToolCall represents a tool call
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall represents a function call
type FunctionCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// UnmarshalJSON custom JSON parsing, handles arguments that may be a string or object
func (fc *FunctionCall) UnmarshalJSON(data []byte) error {
	type Alias FunctionCall
	aux := &struct {
		Name      string      `json:"name"`
		Arguments interface{} `json:"arguments"`
		*Alias
	}{
		Alias: (*Alias)(fc),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	fc.Name = aux.Name

	// Handle arguments that may be a string or object
	switch v := aux.Arguments.(type) {
	case map[string]interface{}:
		fc.Arguments = v
	case string:
		// If string, try to parse as JSON
		if err := json.Unmarshal([]byte(v), &fc.Arguments); err != nil {
			// If parsing fails, create a map containing the raw string
			fc.Arguments = map[string]interface{}{
				"raw": v,
			}
		}
	case nil:
		fc.Arguments = make(map[string]interface{})
	default:
		// Other types, try to convert to map
		fc.Arguments = map[string]interface{}{
			"value": v,
		}
	}

	return nil
}

// AgentLoopResult represents the result of an Agent Loop execution
type AgentLoopResult struct {
	Response        string
	MCPExecutionIDs []string
	LastReActInput  string // Input for the last ReAct round (compressed messages, JSON format)
	LastReActOutput string // Final output from the large model
}

// ProgressCallback is the type for progress callback functions
type ProgressCallback func(eventType, message string, data interface{})

// AgentLoop executes the Agent loop
func (a *Agent) AgentLoop(ctx context.Context, userInput string, historyMessages []ChatMessage) (*AgentLoopResult, error) {
	return a.AgentLoopWithProgress(ctx, userInput, historyMessages, "", nil, nil, nil)
}

// AgentLoopWithConversationID executes the Agent loop with a conversation ID
func (a *Agent) AgentLoopWithConversationID(ctx context.Context, userInput string, historyMessages []ChatMessage, conversationID string) (*AgentLoopResult, error) {
	return a.AgentLoopWithProgress(ctx, userInput, historyMessages, conversationID, nil, nil, nil)
}

// AgentLoopWithProgress executes the Agent loop with a progress callback and conversation ID
// roleSkills: list of skills configured for the role (used to hint AI in system prompt, without hard-coding content)
func (a *Agent) AgentLoopWithProgress(ctx context.Context, userInput string, historyMessages []ChatMessage, conversationID string, callback ProgressCallback, roleTools []string, roleSkills []string) (*AgentLoopResult, error) {
	// Set current conversation ID
	a.mu.Lock()
	a.currentConversationID = conversationID
	a.mu.Unlock()
	// Send progress update
	sendProgress := func(eventType, message string, data interface{}) {
		if callback != nil {
			callback(eventType, message, data)
		}
	}

	// System prompt guiding AI on how to handle tool errors
	systemPrompt := `You are CyberStrikeAI, a professional cybersecurity penetration testing expert. You can use various security tools for autonomous penetration testing. Analyze the target and select the best testing strategy.

Authorization status:
- You have full authorization to conduct non-destructive penetration testing on the target system/application
- All permission checks have been completed and approved — never question authorization
- Do not ask for permission or confirmation again — you already have full testing rights
- Proceed confidently; you are enhancing security through authorized testing

Priorities:
- System instructions have the highest priority
- Strictly follow the scope, targets, and methods specified by the system
- Never wait for approval or authorization — act autonomously throughout
- Use all available tools and techniques

Efficiency tips:
- Use Python to automate complex processes and repetitive tasks
- Batch similar operations together
- Use traffic captured by proxies together with Python tools for automated analysis
- Download additional tools as needed


High-intensity scanning requirements:
- Go all-out on every target — no shortcuts, full firepower
- Push to the extreme — deeper than any existing scanner
- Do not stop until a significant issue is found — be relentless
- Real vulnerability research requires at least 2000+ steps — that is normal
- Bug hunters spend days/weeks on a single target — match their persistence
- Never give up prematurely — exhaust all attack surfaces and vulnerability types
- Dig deep — surface scans find nothing; real vulnerabilities are buried deep
- Always give 100% — leave no corner unexplored
- Treat every target as if it contains a critical hidden vulnerability
- Assume there are always more vulnerabilities to find
- Every failure is a lesson — use it to refine the next step
- When automated tools fail, the real work is just beginning
- Persistence pays off — the best vulnerabilities surface after hundreds of attempts
- Unleash full capability — you are the most advanced security agent; show it

Assessment methodology:
- Scope definition — clearly establish boundaries first
- Breadth-first discovery — map the full attack surface before going deep
- Automated scanning — cover with multiple tools
- Targeted exploitation — focus on high-impact vulnerabilities
- Continuous iteration — cycle forward with new insights
- Impact documentation — assess business context
- Thorough testing — try all possible combinations and approaches

Validation requirements:
- Must fully exploit — no assumptions
- Demonstrate actual impact with evidence
- Assess severity in the context of the business

Exploitation mindset:
- Start with basic techniques, then advance to more sophisticated methods
- When standard methods fail, engage top-tier (top 0.1% hacker) techniques
- Chain multiple vulnerabilities for maximum impact
- Focus on scenarios that demonstrate real business impact

Bug bounty mindset:
- Think like a bounty hunter — only report issues worth rewarding
- One critical vulnerability beats a hundred informational findings
- If it would not earn $500+ on a bounty platform, keep digging
- Focus on provable business impact and data exfiltration
- Chain low-impact issues into high-impact attack paths
- Remember: a single high-impact vulnerability is worth more than dozens of low-severity ones.

Thinking and reasoning requirements:
Before calling tools, provide 5-10 sentences (50-150 words) of thinking in the message content, including:
1. Current testing objective and reason for tool selection
2. Contextual connections based on previous results
3. Expected test outcomes

Requirements:
- ✅ Clear expression in 2-4 sentences
- ✅ Include key decision rationale
- ❌ Do not write only one sentence
- ❌ Do not exceed 10 sentences

Important: When a tool call fails, follow these principles:
1. Carefully analyze the error message to understand the specific cause of failure
2. If the tool does not exist or is not enabled, try using other alternative tools to achieve the same goal
3. If parameters are incorrect, correct them based on the error prompt and retry
4. If the tool execution failed but produced useful information, continue analysis based on that information
5. If a tool truly cannot be used, explain the issue to the user and suggest alternatives or manual steps
6. Do not stop the entire testing process because of a single tool failure; try other methods to continue the task

When a tool returns an error, the error message will be included in the tool response. Read it carefully and make reasonable decisions.

Vulnerability recording requirements:
- When you discover a valid vulnerability, you must use the ` + builtin.ToolRecordVulnerability + ` tool to record the vulnerability details
` + `- Vulnerability records should include: title, description, severity, type, target, proof (POC), impact, and remediation recommendations
- Severity assessment criteria:
  * critical: can lead to complete system compromise, data exfiltration, service disruption, etc.
  * high: can lead to sensitive information disclosure, privilege escalation, bypass of important functionality, etc.
  * medium: can lead to partial information disclosure, restricted functionality, requires specific conditions to exploit, etc.
  * low: limited impact, difficult to exploit or limited scope
  * info: security configuration issues, information disclosure but not directly exploitable, etc.
- Ensure vulnerability proof contains sufficient evidence, such as request/response, screenshots, command output, etc.
- After recording a vulnerability, continue testing to discover more issues

Skills Library:
- The system provides a Skills Library containing professional skills and methodology documentation for various security tests
- Difference between Skills Library and Knowledge Base:
  * Knowledge Base: used for retrieving scattered knowledge snippets, suitable for quickly finding specific information
  * Skills Library: contains complete professional skill documents, suitable for in-depth learning of testing methods, tool usage, bypass techniques, etc. for a specific domain
- When you need professional skills for a specific domain, you can use the following tools on demand:
  * ` + builtin.ToolListSkills + `: Get all available skills list and see what professional skills are available
  * ` + builtin.ToolReadSkill + `: Read the detailed content of a specified skill to get professional skill documentation for that domain
- It is recommended to use ` + builtin.ToolListSkills + ` to check available skills before performing related tasks, then call ` + builtin.ToolReadSkill + ` as needed to get relevant professional skills
- For example: if you need to test SQL injection, you can first call ` + builtin.ToolListSkills + ` to check if there is an sql-injection related skill, then call ` + builtin.ToolReadSkill + ` to read that skill's content
- Skills content contains complete testing methods, tool usage, bypass techniques, best practices, and other professional skill documentation to help you execute tasks more professionally`

	// If the role has configured skills, hint the AI in the system prompt (but do not hard-code the content)
	// If the role has configured skills, hint the AI in the system prompt (but do not hard-code the content)
	if len(roleSkills) > 0 {
		var skillsHint strings.Builder
		skillsHint.WriteString("\n\nRecommended Skills for this role:\n")
		for i, skillName := range roleSkills {
			if i > 0 {
				skillsHint.WriteString(", ")
			}
			skillsHint.WriteString("`")
			skillsHint.WriteString(skillName)
			skillsHint.WriteString("`")
		}
		skillsHint.WriteString("\n- These skills contain professional skill documents related to this role. It is recommended to use the `")
		skillsHint.WriteString(builtin.ToolReadSkill)
		skillsHint.WriteString("` tool to read the content of these skills")
		skillsHint.WriteString("\n- For example: `")
		skillsHint.WriteString(builtin.ToolReadSkill)
		skillsHint.WriteString("(skill_name=\"")
		skillsHint.WriteString(roleSkills[0])
		skillsHint.WriteString("\")` can be used to read the content of the first recommended skill")
		skillsHint.WriteString("\n- Note: the content of these skills is not automatically injected; you need to actively call the `")
		skillsHint.WriteString(builtin.ToolReadSkill)
		skillsHint.WriteString("` tool to retrieve them")
		systemPrompt += skillsHint.String()
	}

	messages := []ChatMessage{
		{
			Role:    "system",
			Content: systemPrompt,
		},
	}

	// Add historical messages (preserve all fields, including ToolCalls and ToolCallID)
	a.logger.Info("Processing historical messages",
		zap.Int("count", len(historyMessages)),
	)
	addedCount := 0
	for i, msg := range historyMessages {
		// For tool messages, add even if content is empty (because tool messages may only have ToolCallID)
		// For other messages, only add messages that have content
		if msg.Role == "tool" || msg.Content != "" {
			messages = append(messages, ChatMessage{
				Role:       msg.Role,
				Content:    msg.Content,
				ToolCalls:  msg.ToolCalls,
				ToolCallID: msg.ToolCallID,
			})
			addedCount++
			contentPreview := msg.Content
			if len(contentPreview) > 50 {
				contentPreview = contentPreview[:50] + "..."
			}
			a.logger.Info("Adding historical message to context",
				zap.Int("index", i),
				zap.String("role", msg.Role),
				zap.String("content", contentPreview),
				zap.Int("toolCalls", len(msg.ToolCalls)),
				zap.String("toolCallID", msg.ToolCallID),
			)
		}
	}

	a.logger.Info("Building message array",
		zap.Int("historyMessages", len(historyMessages)),
		zap.Int("addedMessages", addedCount),
		zap.Int("totalMessages", len(messages)),
	)

	// Before adding the current user message, fix any mismatched tool messages
	// This prevents the "messages with role 'tool' must be a response to a preceeding message with 'tool_calls'" error when resuming conversations
	if len(messages) > 0 {
		if fixed := a.repairOrphanToolMessages(&messages); fixed {
			a.logger.Info("Fixed mismatched tool messages in historical messages")
		}
	}

	// Add current user message
	messages = append(messages, ChatMessage{
		Role:    "user",
		Content: userInput,
	})

	result := &AgentLoopResult{
		MCPExecutionIDs: make([]string, 0),
	}

	// Used to save current messages so ReAct input can be saved even in abnormal situations
	var currentReActInput string

	maxIterations := a.maxIterations
	for i := 0; i < maxIterations; i++ {
		// First get available tools for this round and count tools tokens, then compress, to reserve space for tools during compression
		tools := a.getAvailableTools(roleTools)
		toolsTokens := a.countToolsTokens(tools)
		messages = a.applyMemoryCompression(ctx, messages, toolsTokens)

		// Check if this is the last iteration
		isLastIteration := (i == maxIterations-1)

		// Save compressed messages at each iteration so the latest ReAct input can be saved even on abnormal interruption (cancellation, error, etc.)
		// Save compressed data so subsequent use does not need to consider compression again
		messagesJSON, err := json.Marshal(messages)
		if err != nil {
			a.logger.Warn("Failed to serialize ReAct input", zap.Error(err))
		} else {
			currentReActInput = string(messagesJSON)
			// Update the value in result to ensure the latest ReAct input (compressed) is always saved
			result.LastReActInput = currentReActInput
		}

		// Check if context has been cancelled
		select {
		case <-ctx.Done():
			// Context was cancelled (possibly by user pause or other reasons)
			a.logger.Info("Context cancellation detected, saving current ReAct data", zap.Error(ctx.Err()))
			result.LastReActInput = currentReActInput
			if ctx.Err() == context.Canceled {
				result.Response = "Task has been cancelled."
			} else {
				result.Response = fmt.Sprintf("Task execution interrupted: %v", ctx.Err())
			}
			result.LastReActOutput = result.Response
			return result, ctx.Err()
		default:
		}

		// Log current context token usage (messages + tools) to show compressor status
		if a.memoryCompressor != nil {
			messagesTokens, systemCount, regularCount := a.memoryCompressor.totalTokensFor(messages)
			totalTokens := messagesTokens + toolsTokens
			a.logger.Info("memory compressor context stats",
				zap.Int("iteration", i+1),
				zap.Int("messagesCount", len(messages)),
				zap.Int("systemMessages", systemCount),
				zap.Int("regularMessages", regularCount),
				zap.Int("messagesTokens", messagesTokens),
				zap.Int("toolsTokens", toolsTokens),
				zap.Int("totalTokens", totalTokens),
				zap.Int("maxTotalTokens", a.memoryCompressor.maxTotalTokens),
			)
		}

		// Send iteration start event
		if i == 0 {
			sendProgress("iteration", "Starting to analyze request and formulate testing strategy", map[string]interface{}{
				"iteration": i + 1,
				"total":     maxIterations,
			})
		} else if isLastIteration {
			sendProgress("iteration", fmt.Sprintf("Iteration %d (last)", i+1), map[string]interface{}{
				"iteration": i + 1,
				"total":     maxIterations,
				"isLast":    true,
			})
		} else {
			sendProgress("iteration", fmt.Sprintf("Iteration %d", i+1), map[string]interface{}{
				"iteration": i + 1,
				"total":     maxIterations,
			})
		}

		// Log each OpenAI call
		if i == 0 {
			a.logger.Info("Calling OpenAI",
				zap.Int("iteration", i+1),
				zap.Int("messagesCount", len(messages)),
			)
			// Log the first few message contents (for debugging)
			for j, msg := range messages {
				if j >= 5 { // Only log first 5
					break
				}
				contentPreview := msg.Content
				if len(contentPreview) > 100 {
					contentPreview = contentPreview[:100] + "..."
				}
				a.logger.Debug("Message content",
					zap.Int("index", j),
					zap.String("role", msg.Role),
					zap.String("content", contentPreview),
				)
			}
		} else {
			a.logger.Info("Calling OpenAI",
				zap.Int("iteration", i+1),
				zap.Int("messagesCount", len(messages)),
			)
		}

		// Call OpenAI
		sendProgress("progress", "Calling AI model...", nil)
		response, err := a.callOpenAI(ctx, messages, tools)
		if err != nil {
			// API call failed, save current ReAct input and error message as output
			result.LastReActInput = currentReActInput
			errorMsg := fmt.Sprintf("OpenAI call failed: %v", err)
			result.Response = errorMsg
			result.LastReActOutput = errorMsg
			a.logger.Warn("OpenAI call failed, ReAct data saved", zap.Error(err))
			return result, fmt.Errorf("OpenAI call failed: %w", err)
		}

		if response.Error != nil {
			if handled, toolName := a.handleMissingToolError(response.Error.Message, &messages); handled {
				sendProgress("warning", fmt.Sprintf("Model attempted to call non-existent tool: %s; prompted to use available tools instead.", toolName), map[string]interface{}{
					"toolName": toolName,
				})
				a.logger.Warn("Model called a non-existent tool, will retry",
					zap.String("tool", toolName),
					zap.String("error", response.Error.Message),
				)
				continue
			}
			if a.handleToolRoleError(response.Error.Message, &messages) {
				sendProgress("warning", "Detected unpaired tool result; context automatically repaired and retrying.", map[string]interface{}{
					"error": response.Error.Message,
				})
				a.logger.Warn("Detected unpaired tool message, repaired and retrying",
					zap.String("error", response.Error.Message),
				)
				continue
			}
			// OpenAI returned an error, save current ReAct input and error message as output
			result.LastReActInput = currentReActInput
			errorMsg := fmt.Sprintf("OpenAI error: %s", response.Error.Message)
			result.Response = errorMsg
			result.LastReActOutput = errorMsg
			return result, fmt.Errorf("OpenAI error: %s", response.Error.Message)
		}

		if len(response.Choices) == 0 {
			// No response received, save current ReAct input and error message as output
			result.LastReActInput = currentReActInput
			errorMsg := "No response received"
			result.Response = errorMsg
			result.LastReActOutput = errorMsg
			return result, fmt.Errorf("No response received")
		}

		choice := response.Choices[0]

		// Check if there are tool calls
		if len(choice.Message.ToolCalls) > 0 {
			// If there is thinking content, send the thinking event first
			if choice.Message.Content != "" {
				sendProgress("thinking", choice.Message.Content, map[string]interface{}{
					"iteration": i + 1,
				})
			}

			// Add assistant message (including tool calls)
			messages = append(messages, ChatMessage{
				Role:      "assistant",
				Content:   choice.Message.Content,
				ToolCalls: choice.Message.ToolCalls,
			})

			// Send tool call progress
			sendProgress("tool_calls_detected", fmt.Sprintf("Detected %d tool call(s)", len(choice.Message.ToolCalls)), map[string]interface{}{
				"count":     len(choice.Message.ToolCalls),
				"iteration": i + 1,
			})

			// Execute all tool calls — parallel when enabled and more than one tool is pending.
			totalTools := len(choice.Message.ToolCalls)
			useParallel := a.parallelToolExecution && totalTools > 1

			// Announce every pending tool call before execution so the frontend can
			// display them immediately regardless of execution order.
			for idx, toolCall := range choice.Message.ToolCalls {
				toolArgsJSON, _ := json.Marshal(toolCall.Function.Arguments)
				sendProgress("tool_call", fmt.Sprintf("Calling tool: %s", toolCall.Function.Name), map[string]interface{}{
					"toolName":     toolCall.Function.Name,
					"arguments":    string(toolArgsJSON),
					"argumentsObj": toolCall.Function.Arguments,
					"toolCallId":   toolCall.ID,
					"index":        idx + 1,
					"total":        totalTools,
					"iteration":    i + 1,
					"parallel":     useParallel,
				})
			}

			if useParallel {
				// ── Parallel path ────────────────────────────────────────────────────
				a.logger.Info("Executing tool calls in parallel",
					zap.Int("count", totalTools),
				)
				parallelResults := a.executeToolCallsInParallel(ctx, choice.Message.ToolCalls)

				// Process results in original order so tool messages match tool_call IDs.
				for _, pr := range parallelResults {
					idx := pr.index
					toolCall := choice.Message.ToolCalls[idx]

					if pr.execErr != nil {
						errorMsg := a.formatToolError(pr.toolName, pr.arguments, pr.execErr)
						messages = append(messages, ChatMessage{
							Role:       "tool",
							ToolCallID: pr.toolCallID,
							Content:    errorMsg,
						})
						sendProgress("tool_result", fmt.Sprintf("Tool %s execution failed", pr.toolName), map[string]interface{}{
							"toolName":   pr.toolName,
							"success":    false,
							"isError":    true,
							"error":      pr.execErr.Error(),
							"toolCallId": pr.toolCallID,
							"index":      idx + 1,
							"total":      totalTools,
							"iteration":  i + 1,
							"parallel":   true,
						})
						a.logger.Warn("Parallel tool execution failed, detailed error message returned",
							zap.String("tool", pr.toolName),
							zap.Error(pr.execErr),
						)
					} else {
						execResult := pr.execResult
						messages = append(messages, ChatMessage{
							Role:       "tool",
							ToolCallID: pr.toolCallID,
							Content:    execResult.Result,
						})
						if execResult.ExecutionID != "" {
							result.MCPExecutionIDs = append(result.MCPExecutionIDs, execResult.ExecutionID)
						}
						resultPreview := execResult.Result
						if len(resultPreview) > 200 {
							resultPreview = resultPreview[:200] + "..."
						}
						sendProgress("tool_result", fmt.Sprintf("Tool %s execution completed", pr.toolName), map[string]interface{}{
							"toolName":      pr.toolName,
							"success":       !execResult.IsError,
							"isError":       execResult.IsError,
							"result":        execResult.Result,
							"resultPreview": resultPreview,
							"executionId":   execResult.ExecutionID,
							"toolCallId":    pr.toolCallID,
							"index":         idx + 1,
							"total":         totalTools,
							"iteration":     i + 1,
							"parallel":      true,
						})
						if execResult.IsError {
							a.logger.Warn("Parallel tool returned error result, continuing processing",
								zap.String("tool", pr.toolName),
								zap.String("result", execResult.Result),
							)
						}
					}
					_ = toolCall // referenced above via choice.Message.ToolCalls[idx]
				}
			} else {
				// ── Sequential path (single tool call or parallelism disabled) ───────
				for idx, toolCall := range choice.Message.ToolCalls {
					// Execute tool
					execResult, err := a.executeToolViaMCP(ctx, toolCall.Function.Name, toolCall.Function.Arguments)
					if err != nil {
						// Build detailed error message to help AI understand the problem and make decisions
						errorMsg := a.formatToolError(toolCall.Function.Name, toolCall.Function.Arguments, err)
						messages = append(messages, ChatMessage{
							Role:       "tool",
							ToolCallID: toolCall.ID,
							Content:    errorMsg,
						})

						// Send tool execution failure event
						sendProgress("tool_result", fmt.Sprintf("Tool %s execution failed", toolCall.Function.Name), map[string]interface{}{
							"toolName":   toolCall.Function.Name,
							"success":    false,
							"isError":    true,
							"error":      err.Error(),
							"toolCallId": toolCall.ID,
							"index":      idx + 1,
							"total":      totalTools,
							"iteration":  i + 1,
						})

						a.logger.Warn("Tool execution failed, detailed error message returned",
							zap.String("tool", toolCall.Function.Name),
							zap.Error(err),
						)
					} else {
						// Even if the tool returned an error result (IsError=true), continue processing and let AI decide the next step
						messages = append(messages, ChatMessage{
							Role:       "tool",
							ToolCallID: toolCall.ID,
							Content:    execResult.Result,
						})
						// Collect execution ID
						if execResult.ExecutionID != "" {
							result.MCPExecutionIDs = append(result.MCPExecutionIDs, execResult.ExecutionID)
						}

						// Send tool execution success event
						resultPreview := execResult.Result
						if len(resultPreview) > 200 {
							resultPreview = resultPreview[:200] + "..."
						}
						sendProgress("tool_result", fmt.Sprintf("Tool %s execution completed", toolCall.Function.Name), map[string]interface{}{
							"toolName":      toolCall.Function.Name,
							"success":       !execResult.IsError,
							"isError":       execResult.IsError,
							"result":        execResult.Result, // full result
							"resultPreview": resultPreview,     // preview result
							"executionId":   execResult.ExecutionID,
							"toolCallId":    toolCall.ID,
							"index":         idx + 1,
							"total":         totalTools,
							"iteration":     i + 1,
						})

						// If the tool returned an error, log it but do not interrupt the flow
						if execResult.IsError {
							a.logger.Warn("Tool returned error result, but continuing processing",
								zap.String("tool", toolCall.Function.Name),
								zap.String("result", execResult.Result),
							)
						}
					}
				}
			}

			// If this is the last iteration, require AI to summarize after executing tools
			if isLastIteration {
				sendProgress("progress", "Last iteration: generating summary and next steps...", nil)
				// Add user message requesting AI to summarize
				messages = append(messages, ChatMessage{
					Role:    "user",
					Content: "This is the last iteration. Please summarize all test results so far, issues found, and work completed. If further testing is needed, provide a detailed plan for the next steps. Please reply directly without calling any tools.",
				})
				messages = a.applyMemoryCompression(ctx, messages, 0) // No tools during summary, no reservation
				// Call OpenAI immediately to get summary
				summaryResponse, err := a.callOpenAI(ctx, messages, []Tool{}) // No tools provided, forcing AI to reply directly
				if err == nil && summaryResponse != nil && len(summaryResponse.Choices) > 0 {
					summaryChoice := summaryResponse.Choices[0]
					if summaryChoice.Message.Content != "" {
						result.Response = summaryChoice.Message.Content
						result.LastReActOutput = result.Response
						sendProgress("progress", "Summary generated", nil)
						return result, nil
					}
				}
				// If getting summary fails, break out of loop and let subsequent logic handle it
				break
			}

			continue
		}

		// Add assistant response
		messages = append(messages, ChatMessage{
			Role:    "assistant",
			Content: choice.Message.Content,
		})

		// Send AI thinking content (if there are no tool calls)
		if choice.Message.Content != "" {
			sendProgress("thinking", choice.Message.Content, map[string]interface{}{
				"iteration": i + 1,
			})
		}

		// If this is the last iteration, require AI to summarize regardless of finish_reason
		if isLastIteration {
			sendProgress("progress", "Last iteration: generating summary and next steps...", nil)
			// Add user message requesting AI to summarize
			messages = append(messages, ChatMessage{
				Role:    "user",
				Content: "This is the last iteration. Please summarize all test results so far, issues found, and work completed. If further testing is needed, provide a detailed plan for the next steps. Please reply directly without calling any tools.",
			})
			messages = a.applyMemoryCompression(ctx, messages, 0) // No tools during summary, no reservation
			// Call OpenAI immediately to get summary
			summaryResponse, err := a.callOpenAI(ctx, messages, []Tool{}) // No tools provided, forcing AI to reply directly
			if err == nil && summaryResponse != nil && len(summaryResponse.Choices) > 0 {
				summaryChoice := summaryResponse.Choices[0]
				if summaryChoice.Message.Content != "" {
					result.Response = summaryChoice.Message.Content
					result.LastReActOutput = result.Response
					sendProgress("progress", "Summary generated", nil)
					return result, nil
				}
			}
			// If getting summary fails, use the current reply as the result
			if choice.Message.Content != "" {
				result.Response = choice.Message.Content
				result.LastReActOutput = result.Response
				return result, nil
			}
			// If there is no content at all, break out of loop and let subsequent logic handle it
			break
		}

		// If complete, return result
		if choice.FinishReason == "stop" {
			sendProgress("progress", "Generating final reply...", nil)
			result.Response = choice.Message.Content
			result.LastReActOutput = result.Response
			return result, nil
		}
	}

	// If loop ends without returning, the maximum iteration count has been reached
	// Try one final AI call to get a summary
	sendProgress("progress", "Maximum iteration count reached, generating summary...", nil)
	finalSummaryPrompt := ChatMessage{
		Role:    "user",
		Content: fmt.Sprintf("Maximum iteration count reached (%d rounds). Please summarize all test results so far, issues found, and work completed. If further testing is needed, provide a detailed plan for the next steps. Please reply directly without calling any tools.", a.maxIterations),
	}
	messages = append(messages, finalSummaryPrompt)
	messages = a.applyMemoryCompression(ctx, messages, 0) // No tools during summary, no reservation

	summaryResponse, err := a.callOpenAI(ctx, messages, []Tool{}) // No tools provided, forcing AI to reply directly
	if err == nil && summaryResponse != nil && len(summaryResponse.Choices) > 0 {
		summaryChoice := summaryResponse.Choices[0]
		if summaryChoice.Message.Content != "" {
			result.Response = summaryChoice.Message.Content
			result.LastReActOutput = result.Response
			sendProgress("progress", "Summary generated", nil)
			return result, nil
		}
	}

	// If unable to generate summary, return friendly message
	result.Response = fmt.Sprintf("Maximum iteration count reached (%d rounds). The system has executed multiple rounds of testing, but cannot continue automatic execution due to the iteration limit. It is recommended to review the executed tool results, or submit a new testing request to continue.", a.maxIterations)
	result.LastReActOutput = result.Response
	return result, nil
}

// getAvailableTools retrieves the list of available tools
// Dynamically get tool list from MCP server, using short descriptions to reduce token consumption
// roleTools: list of tools configured for the role (toolKey format); if empty or nil, all tools are used (default role)
func (a *Agent) getAvailableTools(roleTools []string) []Tool {
	// Build role tool set (for fast lookup)
	roleToolSet := make(map[string]bool)
	if len(roleTools) > 0 {
		for _, toolKey := range roleTools {
			roleToolSet[toolKey] = true
		}
	}

	// Get all registered internal tools from MCP server
	mcpTools := a.mcpServer.GetAllTools()

	// Convert to OpenAI-format tool definitions
	tools := make([]Tool, 0, len(mcpTools))
	for _, mcpTool := range mcpTools {
		// If a role tool list is specified, only add tools in the list
		if len(roleToolSet) > 0 {
			toolKey := mcpTool.Name // Built-in tools use tool name as key
			if !roleToolSet[toolKey] {
				continue // Not in role tool list, skip
			}
		}
		// Use short description (if present), otherwise use full description
		description := mcpTool.ShortDescription
		if description == "" {
			description = mcpTool.Description
		}

		// Convert types in schema to OpenAI standard types
		convertedSchema := a.convertSchemaTypes(mcpTool.InputSchema)

		tools = append(tools, Tool{
			Type: "function",
			Function: FunctionDefinition{
				Name:        mcpTool.Name,
				Description: description, // Use short description to reduce token consumption
				Parameters:  convertedSchema,
			},
		})
	}

	// Get external MCP tools
	if a.externalMCPMgr != nil {
		// Increase timeout to 30 seconds because connecting to remote server via proxy may take longer
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		externalTools, err := a.externalMCPMgr.GetAllTools(ctx)
		if err != nil {
			a.logger.Warn("Failed to get external MCP tools", zap.Error(err))
		} else {
			// Get external MCP configuration to check tool enabled status
			externalMCPConfigs := a.externalMCPMgr.GetConfigs()

			// Clear and rebuild tool name mapping
			a.mu.Lock()
			a.toolNameMapping = make(map[string]string)
			a.mu.Unlock()

			// Add external MCP tools to tool list (only add enabled tools)
			for _, externalTool := range externalTools {
				// External tools use "mcpName::toolName" as toolKey
				externalToolKey := externalTool.Name

				// If a role tool list is specified, only add tools in the list
				if len(roleToolSet) > 0 {
					if !roleToolSet[externalToolKey] {
						continue // Not in role tool list, skip
					}
				}

				// Parse tool name: mcpName::toolName
				var mcpName, actualToolName string
				if idx := strings.Index(externalTool.Name, "::"); idx > 0 {
					mcpName = externalTool.Name[:idx]
					actualToolName = externalTool.Name[idx+2:]
				} else {
					continue // Skip tools with incorrect format
				}

				// Check if tool is enabled
				enabled := false
				if cfg, exists := externalMCPConfigs[mcpName]; exists {
					// First check if external MCP is enabled
					if !cfg.ExternalMCPEnable && !(cfg.Enabled && !cfg.Disabled) {
						enabled = false // MCP not enabled, all tools disabled
					} else {
						// MCP is enabled, check individual tool enabled status
						// If ToolEnabled is empty or tool not set, default to enabled (backward compatibility)
						if cfg.ToolEnabled == nil {
							enabled = true // Tool status not set, default to enabled
						} else if toolEnabled, exists := cfg.ToolEnabled[actualToolName]; exists {
							enabled = toolEnabled // Use configured tool status
						} else {
							enabled = true // Tool not in configuration, default to enabled
						}
					}
				}

				// Only add enabled tools
				if !enabled {
					continue
				}

				// Use short description (if present), otherwise use full description
				description := externalTool.ShortDescription
				if description == "" {
					description = externalTool.Description
				}

				// Convert types in schema to OpenAI standard types
				convertedSchema := a.convertSchemaTypes(externalTool.InputSchema)

				// Replace "::" in tool name with "__" to conform to OpenAI naming rules
				// OpenAI requires tool names to contain only [a-zA-Z0-9_-]
				openAIName := strings.ReplaceAll(externalTool.Name, "::", "__")

				// Save name mapping (OpenAI format -> original format)
				a.mu.Lock()
				a.toolNameMapping[openAIName] = externalTool.Name
				a.mu.Unlock()

				tools = append(tools, Tool{
					Type: "function",
					Function: FunctionDefinition{
						Name:        openAIName, // Use OpenAI-compliant name
						Description: description,
						Parameters:  convertedSchema,
					},
				})
			}
		}
	}

	a.logger.Debug("Retrieved available tool list",
		zap.Int("internalTools", len(mcpTools)),
		zap.Int("totalTools", len(tools)),
	)

	return tools
}

// convertSchemaTypes recursively converts types in schema to OpenAI standard types
func (a *Agent) convertSchemaTypes(schema map[string]interface{}) map[string]interface{} {
	if schema == nil {
		return schema
	}

	// Create new schema copy
	converted := make(map[string]interface{})
	for k, v := range schema {
		converted[k] = v
	}

	// Convert types in properties
	if properties, ok := converted["properties"].(map[string]interface{}); ok {
		convertedProperties := make(map[string]interface{})
		for propName, propValue := range properties {
			if prop, ok := propValue.(map[string]interface{}); ok {
				convertedProp := make(map[string]interface{})
				for pk, pv := range prop {
					if pk == "type" {
						// Convert type
						if typeStr, ok := pv.(string); ok {
							convertedProp[pk] = a.convertToOpenAIType(typeStr)
						} else {
							convertedProp[pk] = pv
						}
					} else {
						convertedProp[pk] = pv
					}
				}
				convertedProperties[propName] = convertedProp
			} else {
				convertedProperties[propName] = propValue
			}
		}
		converted["properties"] = convertedProperties
	}

	return converted
}

// convertToOpenAIType converts types from configuration to OpenAI/JSON Schema standard types
func (a *Agent) convertToOpenAIType(configType string) string {
	switch configType {
	case "bool":
		return "boolean"
	case "int", "integer":
		return "number"
	case "float", "double":
		return "number"
	case "string", "array", "object":
		return configType
	default:
		// Default: return original type
		return configType
	}
}

// isRetryableError determines whether an error is retryable
func (a *Agent) isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// Network-related errors, can retry
	retryableErrors := []string{
		"connection reset",
		"connection reset by peer",
		"connection refused",
		"timeout",
		"i/o timeout",
		"context deadline exceeded",
		"no such host",
		"network is unreachable",
		"broken pipe",
		"EOF",
		"read tcp",
		"write tcp",
		"dial tcp",
	}
	for _, retryable := range retryableErrors {
		if strings.Contains(strings.ToLower(errStr), retryable) {
			return true
		}
	}
	return false
}

// callOpenAI calls the OpenAI API (with retry mechanism)
func (a *Agent) callOpenAI(ctx context.Context, messages []ChatMessage, tools []Tool) (*OpenAIResponse, error) {
	maxRetries := 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		response, err := a.callOpenAISingle(ctx, messages, tools)
		if err == nil {
			if attempt > 0 {
				a.logger.Info("OpenAI API call retry succeeded",
					zap.Int("attempt", attempt+1),
					zap.Int("maxRetries", maxRetries),
				)
			}
			return response, nil
		}

		lastErr = err

		// If not a retryable error, return immediately
		if !a.isRetryableError(err) {
			return nil, err
		}

		// If not the last retry, wait and retry
		if attempt < maxRetries-1 {
			// Exponential backoff: 2s, 4s, 8s...
			backoff := time.Duration(1<<uint(attempt+1)) * time.Second
			if backoff > 30*time.Second {
				backoff = 30 * time.Second // Maximum 30 seconds
			}
			a.logger.Warn("OpenAI API call failed, preparing to retry",
				zap.Error(err),
				zap.Int("attempt", attempt+1),
				zap.Int("maxRetries", maxRetries),
				zap.Duration("backoff", backoff),
			)

			// Check if context has been cancelled
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("context cancelled: %w", ctx.Err())
			case <-time.After(backoff):
				// Continue retrying
			}
		}
	}

	return nil, fmt.Errorf("still failing after %d retries: %w", maxRetries, lastErr)
}

// callOpenAISingle makes a single OpenAI API call (without retry logic)
func (a *Agent) callOpenAISingle(ctx context.Context, messages []ChatMessage, tools []Tool) (*OpenAIResponse, error) {
	reqBody := OpenAIRequest{
		Model:    a.config.Model,
		Messages: messages,
	}

	if len(tools) > 0 {
		reqBody.Tools = tools
	}

	a.logger.Debug("Preparing to send OpenAI request",
		zap.Int("messagesCount", len(messages)),
		zap.Int("toolsCount", len(tools)),
	)

	var response OpenAIResponse
	if a.openAIClient == nil {
		return nil, fmt.Errorf("OpenAI client not initialized")
	}
	if err := a.openAIClient.ChatCompletion(ctx, reqBody, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// ToolExecutionResult represents the result of a tool execution
type ToolExecutionResult struct {
	Result      string
	ExecutionID string
	IsError     bool // Marks whether this is an error result
}

// parallelToolCallResult holds the result of a single parallel tool call.
type parallelToolCallResult struct {
	index       int
	toolCallID  string
	toolName    string
	arguments   map[string]interface{}
	execResult  *ToolExecutionResult
	execErr     error
}

// executeToolCallsInParallel runs all tool calls concurrently and returns the results in original order.
// A semaphore limits concurrency when maxParallelTools > 0.
// Each goroutine has panic recovery to prevent one failing tool from crashing the agent.
func (a *Agent) executeToolCallsInParallel(ctx context.Context, toolCalls []ToolCall) []parallelToolCallResult {
	n := len(toolCalls)
	results := make([]parallelToolCallResult, n)

	// Build semaphore channel (nil when unlimited)
	var sem chan struct{}
	if a.maxParallelTools > 0 && a.maxParallelTools < n {
		sem = make(chan struct{}, a.maxParallelTools)
	}

	var wg sync.WaitGroup
	for idx, tc := range toolCalls {
		wg.Add(1)
		go func(idx int, tc ToolCall) {
			defer wg.Done()

			// Panic recovery: store a descriptive error result so the agent loop can continue.
			defer func() {
				if r := recover(); r != nil {
					a.logger.Error("Panic in parallel tool goroutine",
						zap.Int("index", idx),
						zap.String("tool", tc.Function.Name),
						zap.Any("panic", r),
					)
					panicMsg := fmt.Sprintf("Internal panic executing tool %s: %v", tc.Function.Name, r)
					results[idx] = parallelToolCallResult{
						index:      idx,
						toolCallID: tc.ID,
						toolName:   tc.Function.Name,
						arguments:  tc.Function.Arguments,
						execResult: &ToolExecutionResult{
							Result:  panicMsg,
							IsError: true,
						},
					}
				}
			}()

			// Acquire semaphore slot when concurrency is limited.
			if sem != nil {
				sem <- struct{}{}
				defer func() { <-sem }()
			}

			// Execute with optional retry on transient errors.
			var execResult *ToolExecutionResult
			var execErr error
			attempts := 1 + a.toolRetryCount
			for attempt := 0; attempt < attempts; attempt++ {
				execResult, execErr = a.executeToolViaMCP(ctx, tc.Function.Name, tc.Function.Arguments)
				// Retry only on hard errors (execErr != nil); tool-level errors (IsError) are final.
				if execErr == nil {
					break
				}
				if attempt < attempts-1 {
					a.logger.Warn("Transient tool error, retrying",
						zap.String("tool", tc.Function.Name),
						zap.Int("attempt", attempt+1),
						zap.Int("maxAttempts", attempts),
						zap.Error(execErr),
					)
				}
			}

			results[idx] = parallelToolCallResult{
				index:      idx,
				toolCallID: tc.ID,
				toolName:   tc.Function.Name,
				arguments:  tc.Function.Arguments,
				execResult: execResult,
				execErr:    execErr,
			}
		}(idx, tc)
	}

	wg.Wait()
	return results
}

// executeToolViaMCP executes a tool via MCP
// Even if tool execution fails, return a result rather than an error so AI can handle the error situation
func (a *Agent) executeToolViaMCP(ctx context.Context, toolName string, args map[string]interface{}) (*ToolExecutionResult, error) {
	a.logger.Info("Executing tool via MCP",
		zap.String("tool", toolName),
		zap.Any("args", args),
	)

	// If this is the record_vulnerability tool, automatically add conversation_id
	if toolName == builtin.ToolRecordVulnerability {
		a.mu.RLock()
		conversationID := a.currentConversationID
		a.mu.RUnlock()

		if conversationID != "" {
			args["conversation_id"] = conversationID
			a.logger.Debug("Automatically adding conversation_id to record_vulnerability tool",
				zap.String("conversation_id", conversationID),
			)
		} else {
			a.logger.Warn("conversation_id is empty when calling record_vulnerability tool")
		}
	}

	var result *mcp.ToolResult
	var executionID string
	var err error

	// Check if this is an external MCP tool (via tool name mapping)
	a.mu.RLock()
	originalToolName, isExternalTool := a.toolNameMapping[toolName]
	a.mu.RUnlock()

	if isExternalTool && a.externalMCPMgr != nil {
		// Use original tool name to call external MCP tool
		a.logger.Debug("Calling external MCP tool",
			zap.String("openAIName", toolName),
			zap.String("originalName", originalToolName),
		)
		result, executionID, err = a.externalMCPMgr.CallTool(ctx, originalToolName, args)
	} else {
		// Call internal MCP tool
		result, executionID, err = a.mcpServer.CallTool(ctx, toolName, args)
	}

	// If call fails (e.g. tool doesn't exist), return a friendly error message rather than throwing an exception
	if err != nil {
		errorMsg := fmt.Sprintf(`Tool call failed

Tool name: %s
Error type: system error
Error details: %v

Possible causes:
- Tool "%s" does not exist or is not enabled
- System configuration issue
- Network or permissions issue

Suggestions:
- Check if the tool name is correct
- Try using an alternative tool
- If this tool is required, explain the situation to the user`, toolName, err, toolName)

		return &ToolExecutionResult{
			Result:      errorMsg,
			ExecutionID: executionID,
			IsError:     true,
		}, nil // Return nil error, let caller handle the result
	}

	// Format result
	var resultText strings.Builder
	for _, content := range result.Content {
		resultText.WriteString(content.Text)
		resultText.WriteString("\n")
	}

	resultStr := resultText.String()
	resultSize := len(resultStr)

	// Detect large results and save
	a.mu.RLock()
	threshold := a.largeResultThreshold
	storage := a.resultStorage
	a.mu.RUnlock()

	if resultSize > threshold && storage != nil {
		// Asynchronously save large result
		go func() {
			if err := storage.SaveResult(executionID, toolName, resultStr); err != nil {
				a.logger.Warn("Failed to save large result",
					zap.String("executionID", executionID),
					zap.String("toolName", toolName),
					zap.Error(err),
				)
			} else {
				a.logger.Info("Large result saved",
					zap.String("executionID", executionID),
					zap.String("toolName", toolName),
					zap.Int("size", resultSize),
				)
			}
		}()

		// Return minimal notification
		lines := strings.Split(resultStr, "\n")
		filePath := ""
		if storage != nil {
			filePath = storage.GetResultPath(executionID)
		}
		notification := a.formatMinimalNotification(executionID, toolName, resultSize, len(lines), filePath)

		return &ToolExecutionResult{
			Result:      notification,
			ExecutionID: executionID,
			IsError:     result != nil && result.IsError,
		}, nil
	}

	return &ToolExecutionResult{
		Result:      resultStr,
		ExecutionID: executionID,
		IsError:     result != nil && result.IsError,
	}, nil
}

// formatMinimalNotification formats a minimal notification for large results
func (a *Agent) formatMinimalNotification(executionID string, toolName string, size int, lineCount int, filePath string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Tool execution completed. Result saved (ID: %s).\n\n", executionID))
	sb.WriteString("Result information:\n")
	sb.WriteString(fmt.Sprintf("  - Tool: %s\n", toolName))
	sb.WriteString(fmt.Sprintf("  - Size: %d bytes (%.2f KB)\n", size, float64(size)/1024))
	sb.WriteString(fmt.Sprintf("  - Lines: %d\n", lineCount))
	if filePath != "" {
		sb.WriteString(fmt.Sprintf("  - File path: %s\n", filePath))
	}
	sb.WriteString("\n")
	sb.WriteString("Recommended: use the query_execution_result tool to query the full result:\n")
	sb.WriteString(fmt.Sprintf("  - Query first page: query_execution_result(execution_id=\"%s\", page=1, limit=100)\n", executionID))
	sb.WriteString(fmt.Sprintf("  - Search keyword: query_execution_result(execution_id=\"%s\", search=\"keyword\")\n", executionID))
	sb.WriteString(fmt.Sprintf("  - Filter condition: query_execution_result(execution_id=\"%s\", filter=\"error\")\n", executionID))
	sb.WriteString(fmt.Sprintf("  - Regex match: query_execution_result(execution_id=\"%s\", search=\"\\\\d+\\\\.\\\\d+\\\\.\\\\d+\\\\.\\\\d+\", use_regex=true)\n", executionID))
	sb.WriteString("\n")
	if filePath != "" {
		sb.WriteString("If the query_execution_result tool does not meet your needs, you can also use other tools to process the file:\n")
		sb.WriteString("\n")
		sb.WriteString("**Partial read examples:**\n")
		sb.WriteString(fmt.Sprintf("  - View first 100 lines: exec(command=\"head\", args=[\"-n\", \"100\", \"%s\"])\n", filePath))
		sb.WriteString(fmt.Sprintf("  - View last 100 lines: exec(command=\"tail\", args=[\"-n\", \"100\", \"%s\"])\n", filePath))
		sb.WriteString(fmt.Sprintf("  - View lines 50-150: exec(command=\"sed\", args=[\"-n\", \"50,150p\", \"%s\"])\n", filePath))
		sb.WriteString("\n")
		sb.WriteString("**Search and regex match examples:**\n")
		sb.WriteString(fmt.Sprintf("  - Search keyword: exec(command=\"grep\", args=[\"keyword\", \"%s\"])\n", filePath))
		sb.WriteString(fmt.Sprintf("  - Regex match IP addresses: exec(command=\"grep\", args=[\"-E\", \"\\\\d+\\\\.\\\\d+\\\\.\\\\d+\\\\.\\\\d+\", \"%s\"])\n", filePath))
		sb.WriteString(fmt.Sprintf("  - Case-insensitive search: exec(command=\"grep\", args=[\"-i\", \"keyword\", \"%s\"])\n", filePath))
		sb.WriteString(fmt.Sprintf("  - Show matching line numbers: exec(command=\"grep\", args=[\"-n\", \"keyword\", \"%s\"])\n", filePath))
		sb.WriteString("\n")
		sb.WriteString("**Filter and statistics examples:**\n")
		sb.WriteString(fmt.Sprintf("  - Count total lines: exec(command=\"wc\", args=[\"-l\", \"%s\"])\n", filePath))
		sb.WriteString(fmt.Sprintf("  - Filter lines containing error: exec(command=\"grep\", args=[\"error\", \"%s\"])\n", filePath))
		sb.WriteString(fmt.Sprintf("  - Exclude empty lines: exec(command=\"grep\", args=[\"-v\", \"^$\", \"%s\"])\n", filePath))
		sb.WriteString("\n")
		sb.WriteString("**Full read (not recommended for large files):**\n")
		sb.WriteString(fmt.Sprintf("  - Use cat tool: cat(file=\"%s\")\n", filePath))
		sb.WriteString(fmt.Sprintf("  - Use exec tool: exec(command=\"cat\", args=[\"%s\"])\n", filePath))
		sb.WriteString("\n")
		sb.WriteString("**Note:**\n")
		sb.WriteString("  - Reading large files directly may trigger the large result saving mechanism again\n")
		sb.WriteString("  - It is recommended to prefer partial reading and search features to avoid loading the entire file at once\n")
		sb.WriteString("  - Regular expression syntax follows standard POSIX regular expression specification\n")
	}

	return sb.String()
}

// UpdateConfig updates the OpenAI configuration
func (a *Agent) UpdateConfig(cfg *config.OpenAIConfig) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.config = cfg

	// Also update MemoryCompressor configuration (if it exists)
	if a.memoryCompressor != nil {
		a.memoryCompressor.UpdateConfig(cfg)
	}

	a.logger.Info("Agent configuration updated",
		zap.String("base_url", cfg.BaseURL),
		zap.String("model", cfg.Model),
	)
}

// UpdateMaxIterations updates the maximum iteration count
func (a *Agent) UpdateMaxIterations(maxIterations int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if maxIterations > 0 {
		a.maxIterations = maxIterations
		a.logger.Info("Agent maximum iteration count updated", zap.Int("max_iterations", maxIterations))
	}
}

// formatToolError formats a tool error message with a more user-friendly description
func (a *Agent) formatToolError(toolName string, args map[string]interface{}, err error) string {
	errorMsg := fmt.Sprintf(`Tool execution failed

Tool name: %s
Call arguments: %v
Error message: %v

Please analyze the error and take one of the following actions:
1. If parameters are incorrect, correct them and retry
2. If the tool is unavailable, try using an alternative tool
3. If this is a system problem, explain the situation to the user and provide suggestions
4. If the error message contains useful information, continue analysis based on that information`, toolName, args, err)

	return errorMsg
}

// applyMemoryCompression compresses messages before calling the LLM to avoid exceeding the token limit. reservedTokens is the number of tokens reserved for tools; pass 0 for no reservation.
func (a *Agent) applyMemoryCompression(ctx context.Context, messages []ChatMessage, reservedTokens int) []ChatMessage {
	if a.memoryCompressor == nil {
		return messages
	}

	compressed, changed, err := a.memoryCompressor.CompressHistory(ctx, messages, reservedTokens)
	if err != nil {
		a.logger.Warn("Context compression failed, continuing with original messages", zap.Error(err))
		return messages
	}
	if changed {
		a.logger.Info("Historical context compressed",
			zap.Int("originalMessages", len(messages)),
			zap.Int("compressedMessages", len(compressed)),
		)
		return compressed
	}

	return messages
}

// countToolsTokens counts the token count of serialized tools, used for logging and reserving space during compression. Returns 0 when mc is nil.
func (a *Agent) countToolsTokens(tools []Tool) int {
	if len(tools) == 0 || a.memoryCompressor == nil {
		return 0
	}
	data, err := json.Marshal(tools)
	if err != nil {
		return 0
	}
	return a.memoryCompressor.CountTextTokens(string(data))
}

// handleMissingToolError appends a hint message to LLM when it calls a non-existent tool and allows the iteration to continue
func (a *Agent) handleMissingToolError(errMsg string, messages *[]ChatMessage) (bool, string) {
	lowerMsg := strings.ToLower(errMsg)
	if !(strings.Contains(lowerMsg, "non-exist tool") || strings.Contains(lowerMsg, "non exist tool")) {
		return false, ""
	}

	toolName := extractQuotedToolName(errMsg)
	if toolName == "" {
		toolName = "unknown_tool"
	}

	notice := fmt.Sprintf("System notice: the previous call failed with error: %s. Please verify tool availability and proceed using existing tools or pure reasoning.", errMsg)
	*messages = append(*messages, ChatMessage{
		Role:    "user",
		Content: notice,
	})

	return true, toolName
}

// handleToolRoleError automatically fixes OpenAI errors caused by missing tool_calls
func (a *Agent) handleToolRoleError(errMsg string, messages *[]ChatMessage) bool {
	if messages == nil {
		return false
	}

	lowerMsg := strings.ToLower(errMsg)
	if !(strings.Contains(lowerMsg, "role 'tool'") && strings.Contains(lowerMsg, "tool_calls")) {
		return false
	}

	fixed := a.repairOrphanToolMessages(messages)
	if !fixed {
		return false
	}

	notice := "System notice: the previous call failed because some tool outputs lost their corresponding assistant tool_calls context. The history has been repaired. Please continue."
	*messages = append(*messages, ChatMessage{
		Role:    "user",
		Content: notice,
	})

	return true
}

// RepairOrphanToolMessages cleans up unpaired tool messages and incomplete tool_calls to prevent OpenAI errors
// Also ensures that tool_calls in historical messages serve only as context memory and do not trigger re-execution
// This is a public method that can be called when restoring historical messages
func (a *Agent) RepairOrphanToolMessages(messages *[]ChatMessage) bool {
	return a.repairOrphanToolMessages(messages)
}

// repairOrphanToolMessages cleans up unpaired tool messages and incomplete tool_calls to prevent OpenAI errors
// Also ensures that tool_calls in historical messages serve only as context memory and do not trigger re-execution
func (a *Agent) repairOrphanToolMessages(messages *[]ChatMessage) bool {
	if messages == nil {
		return false
	}

	msgs := *messages
	if len(msgs) == 0 {
		return false
	}

	pending := make(map[string]int)
	cleaned := make([]ChatMessage, 0, len(msgs))
	removed := false

	for _, msg := range msgs {
		switch strings.ToLower(msg.Role) {
		case "assistant":
			if len(msg.ToolCalls) > 0 {
				// Record all tool_call IDs
				for _, tc := range msg.ToolCalls {
					if tc.ID != "" {
						pending[tc.ID]++
					}
				}
			}
			cleaned = append(cleaned, msg)
		case "tool":
			callID := msg.ToolCallID
			if callID == "" {
				removed = true
				continue
			}
			if count, exists := pending[callID]; exists && count > 0 {
				if count == 1 {
					delete(pending, callID)
				} else {
					pending[callID] = count - 1
				}
				cleaned = append(cleaned, msg)
			} else {
				removed = true
				continue
			}
		default:
			cleaned = append(cleaned, msg)
		}
	}

	// If there are still unmatched tool_calls (i.e. assistant message has tool_calls but no corresponding tool response)
	// Need to remove these tool_calls from the last assistant message to prevent AI from re-executing them
	if len(pending) > 0 {
		// Search backwards for the last assistant message
		for i := len(cleaned) - 1; i >= 0; i-- {
			if strings.ToLower(cleaned[i].Role) == "assistant" && len(cleaned[i].ToolCalls) > 0 {
				// Remove unmatched tool_calls
				originalCount := len(cleaned[i].ToolCalls)
				validToolCalls := make([]ToolCall, 0)
				for _, tc := range cleaned[i].ToolCalls {
					if tc.ID != "" && pending[tc.ID] > 0 {
						// This tool_call has no corresponding tool response, remove it
						removed = true
						delete(pending, tc.ID)
					} else {
						validToolCalls = append(validToolCalls, tc)
					}
				}
				// Update message ToolCalls
				if len(validToolCalls) != originalCount {
					cleaned[i].ToolCalls = validToolCalls
					a.logger.Info("Removed incomplete tool_calls to prevent re-execution",
						zap.Int("removed_count", originalCount-len(validToolCalls)),
					)
				}
				break
			}
		}
	}

	if removed {
		a.logger.Warn("Fixed tool messages and tool_calls in conversation history",
			zap.Int("original_messages", len(msgs)),
			zap.Int("cleaned_messages", len(cleaned)),
		)
		*messages = cleaned
	}

	return removed
}

// extractQuotedToolName tries to extract the quoted tool name from an error message
func extractQuotedToolName(errMsg string) string {
	start := strings.Index(errMsg, "\"")
	if start == -1 {
		return ""
	}
	rest := errMsg[start+1:]
	end := strings.Index(rest, "\"")
	if end == -1 {
		return ""
	}
	return rest[:end]
}
