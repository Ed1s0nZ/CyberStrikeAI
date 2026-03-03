package attackchain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"cyberstrike-ai/internal/agent"
	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/openai"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Builder is the attack chain builder.
type Builder struct {
	db           *database.DB
	logger       *zap.Logger
	openAIClient *openai.Client
	openAIConfig *config.OpenAIConfig
	tokenCounter agent.TokenCounter
	maxTokens    int // max tokens limit, default 100000
}

// Node is an attack chain node (uses the database package type).
type Node = database.AttackChainNode

// Edge is an attack chain edge (uses the database package type).
type Edge = database.AttackChainEdge

// Chain is the complete attack chain
type Chain struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

// NewBuilder creates a new attack chain builder.
func NewBuilder(db *database.DB, openAIConfig *config.OpenAIConfig, logger *zap.Logger) *Builder {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}
	httpClient := &http.Client{Timeout: 5 * time.Minute, Transport: transport}

	// Prefer the unified token limit from config file (config.yaml -> openai.max_total_tokens).
	maxTokens := 0
	if openAIConfig != nil && openAIConfig.MaxTotalTokens > 0 {
		maxTokens = openAIConfig.MaxTotalTokens
	} else if openAIConfig != nil {
		// If max_total_tokens is not explicitly configured, set a reasonable default based on the model.
		model := strings.ToLower(openAIConfig.Model)
		if strings.Contains(model, "gpt-4") {
			maxTokens = 128000 // gpt-4 typically supports 128k
		} else if strings.Contains(model, "gpt-3.5") {
			maxTokens = 16000 // gpt-3.5-turbo typically supports 16k
		} else if strings.Contains(model, "deepseek") {
			maxTokens = 131072 // deepseek-chat typically supports 131k
		} else {
			maxTokens = 100000 // fallback default value
		}
	} else {
		// Use fallback value when no OpenAI config is present, to avoid 0.
		maxTokens = 100000
	}

	return &Builder{
		db:           db,
		logger:       logger,
		openAIClient: openai.NewClient(openAIConfig, httpClient, logger),
		openAIConfig: openAIConfig,
		tokenCounter: agent.NewTikTokenCounter(),
		maxTokens:    maxTokens,
	}
}

// BuildChainFromConversation builds an attack chain from a conversation (simplified: user input + last ReAct input + model output).
func (b *Builder) BuildChainFromConversation(ctx context.Context, conversationID string) (*Chain, error) {
	b.logger.Info("starting attack chain build (simplified)", zap.String("conversationId", conversationID))

	// 0. First check whether there are any actual tool execution records.
	messages, err := b.db.GetMessages(conversationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get conversation messages: %w", err)
	}

	if len(messages) == 0 {
		b.logger.Info("no data in conversation", zap.String("conversationId", conversationID))
		return &Chain{Nodes: []Node{}, Edges: []Edge{}}, nil
	}

	// Check for actual tool executions (by inspecting assistant message mcp_execution_ids).
	hasToolExecutions := false
	for i := len(messages) - 1; i >= 0; i-- {
		if strings.EqualFold(messages[i].Role, "assistant") {
			if len(messages[i].MCPExecutionIDs) > 0 {
				hasToolExecutions = true
				break
			}
		}
	}

	// Check whether the task was cancelled (by inspecting the last assistant message content or process_details).
	taskCancelled := false
	for i := len(messages) - 1; i >= 0; i-- {
		if strings.EqualFold(messages[i].Role, "assistant") {
			content := strings.ToLower(messages[i].Content)
			if strings.Contains(content, "cancelled") || strings.Contains(content, "canceled") {
				taskCancelled = true
			}
			break
		}
	}

	// If the task was cancelled and there are no actual tool executions, return an empty attack chain.
	if taskCancelled && !hasToolExecutions {
		b.logger.Info("task cancelled with no actual tool executions, returning empty attack chain",
			zap.String("conversationId", conversationID),
			zap.Bool("taskCancelled", taskCancelled),
			zap.Bool("hasToolExecutions", hasToolExecutions))
		return &Chain{Nodes: []Node{}, Edges: []Edge{}}, nil
	}

	// If there are no actual tool executions, also return an empty attack chain (to prevent AI hallucination).
	if !hasToolExecutions {
		b.logger.Info("no actual tool execution records, returning empty attack chain",
			zap.String("conversationId", conversationID))
		return &Chain{Nodes: []Node{}, Edges: []Edge{}}, nil
	}

	// 1. First try to retrieve the saved last-round ReAct input and output from the database.
	reactInputJSON, modelOutput, err := b.db.GetReActData(conversationID)
	if err != nil {
		b.logger.Warn("failed to get saved ReAct data, will build from message history", zap.Error(err))
		// Continue with the original logic.
		reactInputJSON = ""
		modelOutput = ""
	}

	// var userInput string
	var reactInputFinal string
	var dataSource string // record data source

	// If saved ReAct data was successfully retrieved, use it directly.
	if reactInputJSON != "" && modelOutput != "" {
		// Compute hash of ReAct input for tracking.
		hash := sha256.Sum256([]byte(reactInputJSON))
		reactInputHash := hex.EncodeToString(hash[:])[:16] // use first 16 characters as short identifier

		// Count the number of messages.
		var messageCount int
		var tempMessages []interface{}
		if json.Unmarshal([]byte(reactInputJSON), &tempMessages) == nil {
			messageCount = len(tempMessages)
		}

		dataSource = "database_last_react_input"
		b.logger.Info("building attack chain from saved ReAct data",
			zap.String("conversationId", conversationID),
			zap.String("dataSource", dataSource),
			zap.Int("reactInputSize", len(reactInputJSON)),
			zap.Int("messageCount", messageCount),
			zap.String("reactInputHash", reactInputHash),
			zap.Int("modelOutputSize", len(modelOutput)))

		// Extract user input from the saved ReAct input (JSON format).
		// userInput = b.extractUserInputFromReActInput(reactInputJSON)

		// Convert JSON-format messages to a readable format.
		reactInputFinal = b.formatReActInputFromJSON(reactInputJSON)
	} else {
		// 2. If there is no saved ReAct data, build from conversation messages.
		dataSource = "messages_table"
		b.logger.Info("building ReAct data from message history",
			zap.String("conversationId", conversationID),
			zap.String("dataSource", dataSource),
			zap.Int("messageCount", len(messages)))

		// Extract user input (the last user message).
		for i := len(messages) - 1; i >= 0; i-- {
			if strings.EqualFold(messages[i].Role, "user") {
				// userInput = messages[i].Content
				break
			}
		}

		// Extract the last round of ReAct input (history messages + current user input).
		reactInputFinal = b.buildReActInput(messages)

		// Extract the last output of the LLM (the last assistant message).
		for i := len(messages) - 1; i >= 0; i-- {
			if strings.EqualFold(messages[i].Role, "assistant") {
				modelOutput = messages[i].Content
				break
			}
		}
	}

	// 3. Build a simplified prompt to pass to the LLM in one shot.
	prompt := b.buildSimplePrompt(reactInputFinal, modelOutput)
	// fmt.Println(prompt)
	// 6. Call AI to generate the attack chain (one-shot, no extra processing).
	chainJSON, err := b.callAIForChainGeneration(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("AI generation failed: %w", err)
	}

	// 7. Parse JSON and generate node/edge IDs (the frontend requires valid IDs).
	chainData, err := b.parseChainJSON(chainJSON)
	if err != nil {
		// If parsing fails, return an empty chain and let the frontend handle the error.
		b.logger.Warn("failed to parse attack chain JSON", zap.Error(err), zap.String("raw_json", chainJSON))
		return &Chain{
			Nodes: []Node{},
			Edges: []Edge{},
		}, nil
	}

	b.logger.Info("attack chain build complete",
		zap.String("conversationId", conversationID),
		zap.String("dataSource", dataSource),
		zap.Int("nodes", len(chainData.Nodes)),
		zap.Int("edges", len(chainData.Edges)))

	// Save to database for future loading.
	if err := b.saveChain(conversationID, chainData.Nodes, chainData.Edges); err != nil {
		b.logger.Warn("failed to save attack chain to database", zap.Error(err))
		// Even if saving fails, still return data to the frontend.
	}

	// Return directly without any further processing or validation.
	return chainData, nil
}

// buildReActInput builds the last-round ReAct input (history messages + current user input).
func (b *Builder) buildReActInput(messages []database.Message) string {
	var builder strings.Builder
	for _, msg := range messages {
		builder.WriteString(fmt.Sprintf("[%s]: %s\n\n", msg.Role, msg.Content))
	}
	return builder.String()
}

// extractUserInputFromReActInput extracts the last user message from the saved ReAct input (JSON array of messages).
// func (b *Builder) extractUserInputFromReActInput(reactInputJSON string) string {
// 	// reactInputJSON is a JSON-formatted ChatMessage array that needs to be parsed
// 	var messages []map[string]interface{}
// 	if err := json.Unmarshal([]byte(reactInputJSON), &messages); err != nil {
// 		b.logger.Warn("failed to parse ReAct input JSON", zap.Error(err))
// 		return ""
// 	}

// 	// Search from the back for the last user message.
// 	for i := len(messages) - 1; i >= 0; i-- {
// 		if role, ok := messages[i]["role"].(string); ok && strings.EqualFold(role, "user") {
// 			if content, ok := messages[i]["content"].(string); ok {
// 				return content
// 			}
// 		}
// 	}

// 	return ""
// }

// formatReActInputFromJSON converts a JSON-format messages array to a human-readable string.
func (b *Builder) formatReActInputFromJSON(reactInputJSON string) string {
	var messages []map[string]interface{}
	if err := json.Unmarshal([]byte(reactInputJSON), &messages); err != nil {
		b.logger.Warn("failed to parse ReAct input JSON", zap.Error(err))
		return reactInputJSON // If parsing fails, return the raw JSON.
	}

	var builder strings.Builder
	for _, msg := range messages {
		role, _ := msg["role"].(string)
		content, _ := msg["content"].(string)

		// Handle assistant messages: extract tool_calls info.
		if role == "assistant" {
			if toolCalls, ok := msg["tool_calls"].([]interface{}); ok && len(toolCalls) > 0 {
				// If there is text content, display it first.
				if content != "" {
					builder.WriteString(fmt.Sprintf("[%s]: %s\n", role, content))
				}
				// Display each tool call in detail.
				builder.WriteString(fmt.Sprintf("[%s] tool calls (%d):\n", role, len(toolCalls)))
				for i, toolCall := range toolCalls {
					if tc, ok := toolCall.(map[string]interface{}); ok {
						toolCallID, _ := tc["id"].(string)
						if funcData, ok := tc["function"].(map[string]interface{}); ok {
							toolName, _ := funcData["name"].(string)
							arguments, _ := funcData["arguments"].(string)
							builder.WriteString(fmt.Sprintf("  [tool call %d]\n", i+1))
							builder.WriteString(fmt.Sprintf("    ID: %s\n", toolCallID))
							builder.WriteString(fmt.Sprintf("    tool name: %s\n", toolName))
							builder.WriteString(fmt.Sprintf("    arguments: %s\n", arguments))
						}
					}
				}
				builder.WriteString("\n")
				continue
			}
		}

		// Handle tool messages: display tool_call_id and full content.
		if role == "tool" {
			toolCallID, _ := msg["tool_call_id"].(string)
			if toolCallID != "" {
				builder.WriteString(fmt.Sprintf("[%s] (tool_call_id: %s):\n%s\n\n", role, toolCallID, content))
			} else {
				builder.WriteString(fmt.Sprintf("[%s]: %s\n\n", role, content))
			}
			continue
		}

		// Other message types (system, user, etc.) are displayed normally.
		builder.WriteString(fmt.Sprintf("[%s]: %s\n\n", role, content))
	}

	return builder.String()
}

// buildSimplePrompt builds a simplified prompt
func (b *Builder) buildSimplePrompt(reactInput, modelOutput string) string {
	return fmt.Sprintf(`You are a professional security testing analyst and attack chain construction expert. Your task is to build a logically clear and educationally meaningful attack chain graph based on conversation records and tool execution results, fully presenting the thought process and execution path of penetration testing.

## Core Objectives

Build an attack chain that tells a complete attack story so learners can:
1. Understand the complete penetration testing workflow and reasoning (every step from target identification to vulnerability discovery)
2. Learn how to extract clues from failures and adjust strategies
3. Understand the actual effectiveness and limitations of tool usage
4. Understand the causal relationship between vulnerability discovery and exploitation

**Key Principle**: Completeness first. Must include all meaningful tool executions and critical steps; do not omit important information to control node count.

## Construction Process (Think in This Order)

### Step 1: Understand the Context
Carefully analyze the tool call sequence in the ReAct input and model output to identify:
- Test targets (IP, domain, URL, etc.)
- Actually executed tools and their parameters
- Key information returned by tools (successful results, error messages, timeouts, etc.)
- AI's analysis and decision-making process

### Step 2: Extract Key Nodes
Extract meaningful nodes from tool execution records, **ensuring no critical steps are omitted**:
- **target nodes**: Create one target node for each independent test target
- **action nodes**: Create one action node for each meaningful tool execution (including failures that provide clues, successful information gathering, vulnerability verification, etc.)
- **vulnerability nodes**: Create one vulnerability node for each truly confirmed vulnerability
- **Completeness check**: Cross-reference the tool call sequence in ReAct input to ensure every meaningful tool execution is included in the attack chain

### Step 3: Build Logical Relationships (Tree Structure)
**Important: Must build a tree structure, not a simple linear chain.**
Connect nodes based on causal relationships to form a tree graph (since it is single-agent execution, chronological order is not required):
- **Branch structure**: One node can have multiple successor nodes (e.g., after port scanning discovers multiple ports, multiple different tests can be conducted simultaneously)
- **Merge structure**: Multiple nodes can point to the same node (e.g., multiple different tests all discover the same vulnerability)
- Identify which actions were executed based on the results of previous actions
- Identify which vulnerabilities were discovered by which actions
- Identify how failed nodes provide clues for subsequent successes
- **Avoid linear chains**: Do not connect all nodes into a single line; build tree structure based on actual parallel testing and branch exploration

### Step 4: Optimize and Refine
- **Completeness check**: Ensure all meaningful tool executions are included, do not omit key steps
- **Merge rules**: Only merge truly similar or duplicate action nodes (e.g., similar calls to the same tool multiple times)
- **Delete rules**: Only delete completely valueless failure nodes (no output whatsoever, pure system errors, duplicate identical failures)
- **Important reminder**: It is better to keep more nodes than to omit key steps. The attack chain must fully present the penetration testing process
- Ensure the attack chain is logically coherent and can tell a complete story

## Node Types Explained

### target (Target Node)
- **Purpose**: Identifies the test target
- **Creation rule**: Create one target node for each independent target (different IP/domain)
- **Multi-target handling**: Nodes for different targets do not connect to each other; each forms its own independent subgraph
- **metadata.target**: Accurately records the target identifier (IP address, domain, URL, etc.)

### action (Action Node)
- **Purpose**: Records tool execution and AI analysis results
- **Label rules**:
  * 10-20 words, verb-object structure
  * Successful nodes: describe execution results (e.g., "Port scan finds 80/443/8080", "Directory scan finds /admin path")
  * Failed nodes: describe failure reason (e.g., "SQL injection attempt (blocked by WAF)", "Port scan timeout (target unreachable)")
- **ai_analysis requirements**:
  * Successful nodes: summarize key findings from tool execution and explain their significance
  * Failed nodes: must explain failure reason, clues obtained, and how these clues guide subsequent actions
  * No more than 150 words; be specific and informative
- **findings requirements**:
  * Extract key information points from tool results
  * Each finding should be an independent, valuable piece of information
  * Successful nodes: list key findings (e.g., ["port 80 open", "port 443 open", "HTTP service is Apache 2.4"])
  * Failed nodes: list failure clues (e.g., ["WAF blocked", "returned 403", "Cloudflare detected"])
- **status flag**:
  * Successful nodes: not set or set to "success"
  * Failed nodes that provide clues: must be set to "failed_insight"
- **risk_score**: Always 0 (action nodes do not assess risk)

### vulnerability (Vulnerability Node)
- **Purpose**: Records truly confirmed security vulnerabilities
- **Creation rules**:
  * Must be a truly confirmed vulnerability, not every finding is a vulnerability
  * Requires clear vulnerability evidence (e.g., SQL injection returns database error, XSS executes successfully, etc.)
- **risk_score rules**:
  * critical (90-100): Can lead to complete system compromise (RCE, SQL injection causing data breach, etc.)
  * high (80-89): Can lead to sensitive information disclosure or privilege escalation
  * medium (60-79): Has security risks but limited impact
  * low (40-59): Minor security issues
- **metadata requirements**:
  * vulnerability_type: Vulnerability type (SQL injection, XSS, RCE, etc.)
  * description: Detailed description of vulnerability location, mechanism, and impact
  * severity: critical/high/medium/low
  * location: Precise vulnerability location (URL, parameter, file path, etc.)

## Node Filtering and Merge Rules

### Failed Nodes That Must Be Retained
The following failure cases must create nodes because they provide valuable clues:
- Tool returns explicit error messages (permission error, connection refused, authentication failure, etc.)
- Timeout or connection failure (may indicate firewall, network isolation, etc.)
- WAF/firewall blocking (returns 403, 406, etc., indicating protective mechanisms exist)
- Tool not installed or misconfigured (but the call was executed)
- Target unreachable (DNS resolution failure, network unavailable, etc.)

### Failed Nodes That Should Be Deleted
The following cases should not create nodes:
- Tool calls with completely no output
- Pure system errors (unrelated to target, e.g., local environment issues)
- Duplicate identical failures (keep only the first occurrence of multiple identical errors)

### Node Merge Rules
The following cases should merge nodes:
- Multiple similar calls to the same tool (e.g., multiple nmap scans of different port ranges, merge into one "port scan" node)
- Multiple similar probes of the same target (e.g., multiple directory scanning tools, merge into one "directory scan" node)

### Node Count Control
- **Completeness first**: Must include all meaningful tool executions and critical steps; do not delete important nodes to control count
- **Suggested range**: Single target typically 8-15 nodes, but can be increased if actual steps are more (maximum 20 nodes)
- **Prioritize retaining**: Critical successful steps, informative failures, discovered vulnerabilities, important information gathering steps
- **Can merge**: Multiple similar calls to the same tool (e.g., multiple nmap scans of different port ranges, merge into one "port scan" node)
- **Can delete**: Tool calls with no output, pure system errors, duplicate identical failures (keep only first occurrence)
- **Important principle**: Better to have slightly more nodes than to omit key steps. The attack chain must fully present the complete penetration testing process

## Edge Types and Weights

### Edge Types
- **leads_to**: Means "leads to" or "guides to", used for action→action, target→action
  * Example: port scan → directory scan (because port 80 was found, directory scanning is performed)
- **discovers**: Means "discovers", **exclusively used for action→vulnerability**
  * Example: SQL injection test → SQL injection vulnerability
  * **Important**: All action→vulnerability edges must use the discovers type; even if multiple actions point to the same vulnerability, consistently use discovers
- **enables**: Means "enables" or "facilitates", **only used for vulnerability→vulnerability, action→action (when subsequent actions depend on previous results)**
  * Example: information disclosure vulnerability → privilege escalation vulnerability (information obtained through disclosure facilitates privilege escalation)
  * **Important**: enables cannot be used for action→vulnerability; action→vulnerability must use discovers

### Edge Weights
- **Weight 1-2**: Weak association (e.g., initial probe to further probe)
- **Weight 3-4**: Medium association (e.g., port found to service identification)
- **Weight 5-7**: Strong association (e.g., vulnerability found, critical information disclosure)
- **Weight 8-10**: Very strong association (e.g., successful vulnerability exploitation, privilege escalation)

### DAG Structure Requirements (Directed Acyclic Graph)
**Critical: Must ensure the generated graph is a true DAG (directed acyclic graph) with no cycles.**

- **Node numbering rules**: Node ids increment starting from "node_1" (node_1, node_2, node_3...)
- **Edge direction rules**: The source node id of all edges must be strictly less than the target node id (source < target); this is the key to ensuring no cycles
  * Example: node_1 → node_2 ✓ (correct)
  * Example: node_2 → node_1 ✗ (wrong, would form a cycle)
  * Example: node_3 → node_5 ✓ (correct)
- **Cycle verification**: Before outputting JSON, must check all edges to ensure no edge has source >= target
- **No isolated nodes**: Ensure every node has at least one edge connection (except possible root nodes)
- **DAG structure features**:
  * One node can have multiple successor nodes (branches), e.g., node_2 (port scan) can connect to node_3, node_4, node_5, etc. simultaneously
  * Multiple nodes can converge on one node (merge), e.g., node_3, node_4, node_5 all point to node_6 (vulnerability node)
  * Avoid connecting all nodes into a single line; build DAG structure based on actual parallel testing and branch exploration
- **Topological sort verification**: If sorted by node id from small to large, all edges should point from left to right (top to bottom), ensuring no cycles

## Attack Chain Logical Coherence Requirements

The built attack chain should be able to answer the following questions:
1. **Starting point**: Where does the test begin? (target node)
2. **Exploration process**: How is information gradually collected? (action node sequence)
3. **Failures and adjustments**: How to adjust strategy when encountering obstacles? (failed_insight nodes)
4. **Key findings**: What important information was discovered? (action findings)
5. **Vulnerability confirmation**: How is the vulnerability confirmed to exist? (action→vulnerability)
6. **Attack path**: What is the complete attack path? (path from target to vulnerability)

## Last Round ReAct Input

%s

## Model Output

%s

## Output Format

Output strictly in the following JSON format without adding any other text:

**Important: The example shows a tree structure. Note that node_2 (port scan) connects to multiple successor nodes (node_3, node_4) simultaneously, forming a branch structure.**

{
   "nodes": [
     {
       "id": "node_1",
       "type": "target",
       "label": "Test target: example.com",
       "risk_score": 40,
       "metadata": {
         "target": "example.com"
       }
     },
     {
       "id": "node_2",
       "type": "action",
       "label": "Port scan finds 80/443/8080",
       "risk_score": 0,
       "metadata": {
         "tool_name": "nmap",
         "tool_intent": "Port scanning",
         "ai_analysis": "Used nmap to perform port scanning on the target, finding ports 80, 443, and 8080 open. Port 80 runs an HTTP service, port 443 runs HTTPS, and port 8080 may be an admin backend. These open ports provide entry points for subsequent web application testing.",
         "findings": ["port 80 open", "port 443 open", "port 8080 open", "HTTP service is Apache 2.4"]
       }
     },
     {
       "id": "node_3",
       "type": "action",
       "label": "Directory scan finds /admin backend",
       "risk_score": 0,
       "metadata": {
         "tool_name": "dirsearch",
         "tool_intent": "Directory scanning",
         "ai_analysis": "Used dirsearch to perform directory scanning on the target, finding the /admin directory exists and is accessible. This directory may be an admin backend and is an important test target.",
         "findings": ["/admin directory exists", "returns 200 status code", "suspected admin backend"]
       }
     },
     {
       "id": "node_4",
       "type": "action",
       "label": "Identify web service as Apache 2.4",
       "risk_score": 0,
       "metadata": {
         "tool_name": "whatweb",
         "tool_intent": "Web service identification",
         "ai_analysis": "Identified that the target runs Apache 2.4 server, providing important information for subsequent vulnerability testing.",
         "findings": ["Apache 2.4", "PHP version info"]
       }
     },
     {
       "id": "node_5",
       "type": "action",
       "label": "SQL injection attempt (blocked by WAF)",
       "risk_score": 0,
       "metadata": {
         "tool_name": "sqlmap",
         "tool_intent": "SQL injection detection",
         "ai_analysis": "SQL injection testing on /login.php was blocked by WAF, returning 403 error. The error message indicates Cloudflare protection was detected. This shows the target has a WAF deployed, requiring adjusted testing strategy.",
         "findings": ["WAF blocked", "returned 403", "Cloudflare detected", "target has WAF deployed"],
         "status": "failed_insight"
       }
     },
     {
       "id": "node_6",
       "type": "vulnerability",
       "label": "SQL injection vulnerability",
       "risk_score": 85,
       "metadata": {
         "vulnerability_type": "SQL injection",
         "description": "An SQL injection vulnerability was found in the username parameter of /admin/login.php. Injecting a payload can bypass login verification and directly obtain admin privileges. The vulnerability returns database error messages, confirming the injection point exists.",
         "severity": "high",
         "location": "/admin/login.php?username="
       }
     }
   ],
   "edges": [
     {
       "source": "node_1",
       "target": "node_2",
       "type": "leads_to",
       "weight": 3
     },
     {
       "source": "node_2",
       "target": "node_3",
       "type": "leads_to",
       "weight": 4
     },
     {
       "source": "node_2",
       "target": "node_4",
       "type": "leads_to",
       "weight": 3
     },
     {
       "source": "node_3",
       "target": "node_5",
       "type": "leads_to",
       "weight": 4
     },
     {
       "source": "node_5",
       "target": "node_6",
       "type": "discovers",
       "weight": 7
     }
   ]
}

## Important Reminders

1. **No fabrication**: Only use tools actually executed in ReAct input and their actual returned results. If no actual data exists, return empty nodes and edges arrays.
2. **DAG structure required**: Must build a true DAG (directed acyclic graph) with no cycles. The source node id of all edges must be strictly less than the target node id (source < target).
3. **Topological order**: Nodes should be numbered in logical order; target nodes are typically node_1, subsequent action nodes increment in execution order, vulnerability nodes come last.
4. **Completeness first**: Must include all meaningful tool executions and critical steps; do not delete important nodes to control node count. The attack chain must fully present the complete process from target identification to vulnerability discovery.
5. **Logical coherence**: Ensure the attack chain can tell a complete, coherent penetration testing story, including all key steps and decision points.
6. **Educational value**: Prioritize retaining nodes with educational significance to help learners understand penetration testing thinking and the complete workflow.
7. **Accuracy**: All node information must be based on actual data; do not speculate or assume.
8. **Completeness check**: Ensure every node has the necessary metadata fields, every edge has correct source and target, no isolated nodes, no cycles.
9. **Do not over-simplify**: If actual execution steps are many, nodes can be increased appropriately (maximum 20), ensuring no key steps are omitted.
10. **Verify before output**: Before outputting JSON, must verify all edges satisfy the source < target condition to ensure correct DAG structure.

Now begin analyzing and building the attack chain:`, reactInput, modelOutput)
}

// saveChain saves the attack chain to the database.
func (b *Builder) saveChain(conversationID string, nodes []Node, edges []Edge) error {
	// First delete old attack chain data.
	if err := b.db.DeleteAttackChain(conversationID); err != nil {
		b.logger.Warn("failed to delete old attack chain", zap.Error(err))
	}

	for _, node := range nodes {
		metadataJSON, _ := json.Marshal(node.Metadata)
		if err := b.db.SaveAttackChainNode(conversationID, node.ID, node.Type, node.Label, "", string(metadataJSON), node.RiskScore); err != nil {
			b.logger.Warn("failed to save attack chain node", zap.String("nodeId", node.ID), zap.Error(err))
		}
	}

	// Save edges.
	for _, edge := range edges {
		if err := b.db.SaveAttackChainEdge(conversationID, edge.ID, edge.Source, edge.Target, edge.Type, edge.Weight); err != nil {
			b.logger.Warn("failed to save attack chain edge", zap.String("edgeId", edge.ID), zap.Error(err))
		}
	}

	return nil
}

// LoadChainFromDatabase loads the attack chain from the database.
func (b *Builder) LoadChainFromDatabase(conversationID string) (*Chain, error) {
	nodes, err := b.db.LoadAttackChainNodes(conversationID)
	if err != nil {
		return nil, fmt.Errorf("failed to load attack chain nodes: %w", err)
	}

	edges, err := b.db.LoadAttackChainEdges(conversationID)
	if err != nil {
		return nil, fmt.Errorf("failed to load attack chain edges: %w", err)
	}

	return &Chain{
		Nodes: nodes,
		Edges: edges,
	}, nil
}

// callAIForChainGeneration calls AI to generate the attack chain.
func (b *Builder) callAIForChainGeneration(ctx context.Context, prompt string) (string, error) {
	requestBody := map[string]interface{}{
		"model": b.openAIConfig.Model,
		"messages": []map[string]interface{}{
			{
				"role":    "system",
				"content": "You are a professional security testing analyst skilled at building attack chain graphs. Return attack chain data strictly in JSON format.",
			},
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"temperature": 0.3,
		"max_tokens":  8000,
	}

	var apiResponse struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if b.openAIClient == nil {
		return "", fmt.Errorf("OpenAI client not initialized")
	}
	if err := b.openAIClient.ChatCompletion(ctx, requestBody, &apiResponse); err != nil {
		var apiErr *openai.APIError
		if errors.As(err, &apiErr) {
			bodyStr := strings.ToLower(apiErr.Body)
			if strings.Contains(bodyStr, "context") || strings.Contains(bodyStr, "length") || strings.Contains(bodyStr, "too long") {
				return "", fmt.Errorf("context length exceeded")
			}
		} else if strings.Contains(strings.ToLower(err.Error()), "context") || strings.Contains(strings.ToLower(err.Error()), "length") {
			return "", fmt.Errorf("context length exceeded")
		}
		return "", fmt.Errorf("request failed: %w", err)
	}

	if len(apiResponse.Choices) == 0 {
		return "", fmt.Errorf("API returned no valid response")
	}

	content := strings.TrimSpace(apiResponse.Choices[0].Message.Content)
	// Try to extract JSON (may be wrapped in a markdown code block).
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	return content, nil
}

// ChainJSON is the attack chain JSON structure.
type ChainJSON struct {
	Nodes []struct {
		ID        string                 `json:"id"`
		Type      string                 `json:"type"`
		Label     string                 `json:"label"`
		RiskScore int                    `json:"risk_score"`
		Metadata  map[string]interface{} `json:"metadata"`
	} `json:"nodes"`
	Edges []struct {
		Source string `json:"source"`
		Target string `json:"target"`
		Type   string `json:"type"`
		Weight int    `json:"weight"`
	} `json:"edges"`
}

// parseChainJSON parses the attack chain JSON.
func (b *Builder) parseChainJSON(chainJSON string) (*Chain, error) {
	var chainData ChainJSON
	if err := json.Unmarshal([]byte(chainJSON), &chainData); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Create a node ID mapping (AI-returned ID -> new UUID).
	nodeIDMap := make(map[string]string)

	// Convert to Chain structure.
	nodes := make([]Node, 0, len(chainData.Nodes))
	for _, n := range chainData.Nodes {
		// Generate a new UUID node ID.
		newNodeID := fmt.Sprintf("node_%s", uuid.New().String())
		nodeIDMap[n.ID] = newNodeID

		node := Node{
			ID:        newNodeID,
			Type:      n.Type,
			Label:     n.Label,
			RiskScore: n.RiskScore,
			Metadata:  n.Metadata,
		}
		if node.Metadata == nil {
			node.Metadata = make(map[string]interface{})
		}
		nodes = append(nodes, node)
	}

	// Convert edges.
	edges := make([]Edge, 0, len(chainData.Edges))
	for _, e := range chainData.Edges {
		sourceID, ok := nodeIDMap[e.Source]
		if !ok {
			continue
		}
		targetID, ok := nodeIDMap[e.Target]
		if !ok {
			continue
		}

		// Generate edge ID (required by the frontend).
		edgeID := fmt.Sprintf("edge_%s", uuid.New().String())

		edges = append(edges, Edge{
			ID:     edgeID,
			Source: sourceID,
			Target: targetID,
			Type:   e.Type,
			Weight: e.Weight,
		})
	}

	return &Chain{
		Nodes: nodes,
		Edges: edges,
	}, nil
}

// All methods below are no longer used and have been removed to simplify the code.
