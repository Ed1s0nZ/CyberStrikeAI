package attackchain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/mcp"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Builder 攻击链构建器
type Builder struct {
	db           *database.DB
	logger       *zap.Logger
	openAIClient *http.Client
	openAIConfig *config.OpenAIConfig
}

// Node 攻击链节点（使用database包的类型）
type Node = database.AttackChainNode

// Edge 攻击链边（使用database包的类型）
type Edge = database.AttackChainEdge

// Chain 完整的攻击链
type Chain struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

// NewBuilder 创建新的攻击链构建器
func NewBuilder(db *database.DB, openAIConfig *config.OpenAIConfig, logger *zap.Logger) *Builder {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}

	return &Builder{
		db:           db,
		logger:       logger,
		openAIClient: &http.Client{Timeout: 5 * time.Minute, Transport: transport},
		openAIConfig: openAIConfig,
	}
}

// BuildChainFromConversation 从对话构建攻击链（一次性生成整个图）
func (b *Builder) BuildChainFromConversation(ctx context.Context, conversationID string) (*Chain, error) {
	b.logger.Info("开始构建攻击链（一次性生成）", zap.String("conversationId", conversationID))

	// 1. 获取对话消息和工具执行记录
	messages, err := b.db.GetMessages(conversationID)
	if err != nil {
		return nil, fmt.Errorf("获取对话消息失败: %w", err)
	}

	executions, err := b.getToolExecutionsByConversation(conversationID)
	if err != nil {
		return nil, fmt.Errorf("获取工具执行记录失败: %w", err)
	}

	// 获取过程详情
	processDetailsMap, err := b.db.GetProcessDetailsByConversation(conversationID)
	if err != nil {
		b.logger.Warn("获取过程详情失败", zap.Error(err))
		processDetailsMap = make(map[string][]database.ProcessDetail)
	}

	if len(executions) == 0 && len(messages) == 0 {
		b.logger.Info("对话中没有数据", zap.String("conversationId", conversationID))
		return &Chain{Nodes: []Node{}, Edges: []Edge{}}, nil
	}

	// 2. 准备上下文数据
	contextData, err := b.prepareContextData(messages, executions, processDetailsMap)
	if err != nil {
		return nil, fmt.Errorf("准备上下文数据失败: %w", err)
	}

	// 3. 一次性生成攻击链（带重试和压缩机制）
	chain, err := b.generateChainWithRetry(ctx, contextData, 5)
	if err != nil {
		return nil, fmt.Errorf("生成攻击链失败: %w", err)
	}

	// 4. 保存到数据库
	if err := b.saveChain(conversationID, chain.Nodes, chain.Edges); err != nil {
		b.logger.Warn("保存攻击链失败", zap.Error(err))
		// 不返回错误，继续返回结果
	}

	b.logger.Info("攻击链构建完成",
		zap.String("conversationId", conversationID),
		zap.Int("nodes", len(chain.Nodes)),
		zap.Int("edges", len(chain.Edges)))

	return chain, nil
}

// getToolExecutionsByConversation 获取对话的工具执行记录
func (b *Builder) getToolExecutionsByConversation(conversationID string) ([]*mcp.ToolExecution, error) {
	// 通过conversation_id关联messages，再通过mcp_execution_ids关联tool_executions
	// 简化实现：直接查询所有工具执行记录，然后过滤（实际应该优化查询）
	allExecutions, err := b.db.LoadToolExecutions()
	if err != nil {
		return nil, err
	}

	// 获取对话的消息，提取mcp_execution_ids
	messages, err := b.db.GetMessages(conversationID)
	if err != nil {
		return nil, err
	}

	// 收集所有execution IDs
	executionIDSet := make(map[string]bool)
	for _, msg := range messages {
		if len(msg.MCPExecutionIDs) > 0 {
			for _, id := range msg.MCPExecutionIDs {
				executionIDSet[id] = true
			}
		}
	}

	// 过滤执行记录
	var filteredExecutions []*mcp.ToolExecution
	for _, exec := range allExecutions {
		if executionIDSet[exec.ID] {
			filteredExecutions = append(filteredExecutions, exec)
		}
	}

	// 按时间排序
	sort.Slice(filteredExecutions, func(i, j int) bool {
		return filteredExecutions[i].StartTime.Before(filteredExecutions[j].StartTime)
	})

	return filteredExecutions, nil
}

// saveChain 保存攻击链到数据库
func (b *Builder) saveChain(conversationID string, nodes []Node, edges []Edge) error {
	// 先删除旧的攻击链数据
	if err := b.db.DeleteAttackChain(conversationID); err != nil {
		b.logger.Warn("删除旧攻击链失败", zap.Error(err))
	}

	// 保存节点
	for _, node := range nodes {
		metadataJSON, _ := json.Marshal(node.Metadata)
		if err := b.db.SaveAttackChainNode(conversationID, node.ID, node.Type, node.Label, node.ToolExecutionID, string(metadataJSON), node.RiskScore); err != nil {
			b.logger.Warn("保存攻击链节点失败", zap.String("nodeId", node.ID), zap.Error(err))
		}
	}

	// 保存边
	for _, edge := range edges {
		if err := b.db.SaveAttackChainEdge(conversationID, edge.ID, edge.Source, edge.Target, edge.Type, edge.Weight); err != nil {
			b.logger.Warn("保存攻击链边失败", zap.String("edgeId", edge.ID), zap.Error(err))
		}
	}

	return nil
}

// LoadChainFromDatabase 从数据库加载攻击链
func (b *Builder) LoadChainFromDatabase(conversationID string) (*Chain, error) {
	nodes, err := b.db.LoadAttackChainNodes(conversationID)
	if err != nil {
		return nil, fmt.Errorf("加载攻击链节点失败: %w", err)
	}

	edges, err := b.db.LoadAttackChainEdges(conversationID)
	if err != nil {
		return nil, fmt.Errorf("加载攻击链边失败: %w", err)
	}

	return &Chain{
		Nodes: nodes,
		Edges: edges,
	}, nil
}

// ContextData 上下文数据（用于一次性生成攻击链）
type ContextData struct {
	Messages        []database.Message                  `json:"messages"`
	Executions      []*mcp.ToolExecution                `json:"executions"`
	ProcessDetails  map[string][]database.ProcessDetail `json:"process_details"`
	SummarizedItems map[string]string                   `json:"summarized_items"` // 已总结的项目（key: 原始ID, value: 总结内容）
}

// prepareContextData 准备上下文数据
func (b *Builder) prepareContextData(messages []database.Message, executions []*mcp.ToolExecution, processDetails map[string][]database.ProcessDetail) (*ContextData, error) {
	return &ContextData{
		Messages:        messages,
		Executions:      executions,
		ProcessDetails:  processDetails,
		SummarizedItems: make(map[string]string),
	}, nil
}

// generateChainWithRetry 生成攻击链（带重试和压缩机制）
func (b *Builder) generateChainWithRetry(ctx context.Context, contextData *ContextData, maxRetries int) (*Chain, error) {
	for attempt := 0; attempt < maxRetries; attempt++ {
		b.logger.Info("尝试生成攻击链",
			zap.Int("attempt", attempt+1),
			zap.Int("maxRetries", maxRetries))

		// 构建提示词
		prompt, err := b.buildChainGenerationPrompt(contextData)
		if err != nil {
			return nil, fmt.Errorf("构建提示词失败: %w", err)
		}

		// 调用AI生成攻击链
		chainJSON, err := b.callAIForChainGeneration(ctx, prompt)
		if err != nil {
			// 检查是否是上下文过长错误
			if strings.Contains(err.Error(), "context length") || strings.Contains(err.Error(), "too long") || strings.Contains(err.Error(), "context length exceeded") {
				b.logger.Warn("上下文过长，尝试压缩",
					zap.Int("attempt", attempt+1),
					zap.Error(err))

				// 压缩最长的子节点
				if err := b.compressLongestItem(ctx, contextData); err != nil {
					return nil, fmt.Errorf("压缩上下文失败: %w", err)
				}

				// 重试
				continue
			}

			return nil, fmt.Errorf("AI生成失败: %w", err)
		}

		// 解析JSON（传入executions用于ID映射）
		chain, err := b.parseChainJSON(chainJSON, contextData.Executions)
		if err != nil {
			return nil, fmt.Errorf("解析攻击链JSON失败: %w", err)
		}

		return chain, nil
	}

	return nil, fmt.Errorf("生成攻击链失败：超过最大重试次数 %d", maxRetries)
}

// buildChainGenerationPrompt 构建攻击链生成提示词
func (b *Builder) buildChainGenerationPrompt(contextData *ContextData) (string, error) {
	var promptBuilder strings.Builder

	promptBuilder.WriteString(`你是一个专业的安全测试分析师。请根据以下对话和工具执行记录，生成清晰、有教育意义的攻击链图。

## 核心原则

**目标：让不懂渗透测试的同学可以通过这个攻击链路学习到知识，而不是无数个节点看花眼。**

## 任务要求

1. **节点类型（简化，只保留3种）**：
   - **target（目标）**：从用户输入中提取测试目标（IP、域名、URL等）
     - **重要：如果对话中测试了多个不同的目标（如先测试A网页，后测试B网页），必须为每个不同的目标创建独立的target节点**
     - 每个target节点只关联属于它的action节点（通过工具执行参数中的目标来判断）
     - 不同目标的action节点之间**不应该**建立关联关系
   - **action（行动）**：**工具执行 + AI分析结果 = 一个action节点**
     - 将每个工具执行和AI对该工具结果的分析合并为一个action节点
     - 节点标签应该清晰描述"做了什么"和"发现了什么"（例如："使用Nmap扫描端口，发现22、80、443端口开放"）
     - 只包含**有效的、成功的**工具执行（忽略错误、失败、无效的执行）
     - **重要：action节点必须关联到正确的target节点（通过工具执行参数判断目标）**
   - **vulnerability（漏洞）**：从工具执行结果和AI分析中提取的**真实漏洞**（不是所有发现都是漏洞）

2. **过滤规则（重要！）**：
   - **忽略所有错误/失败的节点**：
     - 工具执行错误（Error字段不为空，或Result.IsError为true）
     - 工具执行结果为空或无效
     - AI分析中明确标记为"失败"、"错误"、"无效"的内容
   - **只保留有价值的节点**：
     - 成功执行的工具
     - 有实际发现的工具执行
     - 真实存在的漏洞

3. **建立清晰的关联关系**：
   - target → action：目标指向属于它的所有行动（通过工具执行参数判断目标）
   - action → action：行动之间的逻辑顺序（按时间顺序，但只连接有逻辑关系的）
     - **重要：只连接属于同一目标的action节点，不同目标的action节点之间不应该连接**
   - action → vulnerability：行动发现的漏洞
   - vulnerability → vulnerability：漏洞间的因果关系（如SQL注入 → 信息泄露）
     - **重要：只连接属于同一目标的漏洞，不同目标的漏洞之间不应该连接**

4. **节点属性**：
   - 每个节点需要：id, type, label, risk_score, metadata
   - action节点需要：
     - tool_name: 工具名称
     - tool_intent: 工具调用意图（如"端口扫描"、"漏洞扫描"）
     - ai_analysis: AI对工具结果的分析总结（简洁，不超过100字）
     - findings: 关键发现（列表）
   - vulnerability节点需要：type, description, severity, location

## 对话数据

`)

	// 添加消息
	promptBuilder.WriteString("\n### 对话消息\n\n")
	for i, msg := range contextData.Messages {
		promptBuilder.WriteString(fmt.Sprintf("消息%d [%s]:\n", i+1, msg.Role))

		// 检查是否已总结
		if summary, ok := contextData.SummarizedItems[msg.ID]; ok {
			promptBuilder.WriteString(fmt.Sprintf("[已总结] %s\n\n", summary))
		} else {
			content := msg.Content
			if len(content) > 5000 {
				content = content[:5000] + "..."
			}
			promptBuilder.WriteString(fmt.Sprintf("%s\n\n", content))
		}

		// 添加过程详情
		if details, ok := contextData.ProcessDetails[msg.ID]; ok {
			for _, detail := range details {
				if detail.EventType == "thinking" {
					thinkingText := detail.Message
					if summary, ok := contextData.SummarizedItems[detail.ID]; ok {
						thinkingText = "[已总结] " + summary
					} else if len(thinkingText) > 2000 {
						thinkingText = thinkingText[:2000] + "..."
					}
					promptBuilder.WriteString(fmt.Sprintf("思考过程: %s\n", thinkingText))
				}
			}
		}
		promptBuilder.WriteString("\n")
	}

	// 添加工具执行记录（关联对应的AI回复）
	promptBuilder.WriteString("\n### 工具执行记录（包含对应的AI分析）\n\n")

	// 构建工具执行ID到消息的映射（找到工具执行后AI的回复）
	execToMessageMap := b.buildExecutionToMessageMap(contextData)

	for i, exec := range contextData.Executions {
		// 检查是否是错误/失败的执行
		isError := exec.Error != "" || (exec.Result != nil && exec.Result.IsError)
		if isError {
			promptBuilder.WriteString(fmt.Sprintf("执行%d [%s] (ID: %s) - **已忽略（执行失败/错误）**\n\n", i+1, exec.ToolName, exec.ID))
			continue
		}

		promptBuilder.WriteString(fmt.Sprintf("执行%d [%s] (ID: %s):\n", i+1, exec.ToolName, exec.ID))
		promptBuilder.WriteString(fmt.Sprintf("参数: %s\n", b.formatArguments(exec.Arguments)))

		// 检查是否已总结
		var resultText string
		if exec.Result != nil {
			for _, content := range exec.Result.Content {
				if content.Type == "text" {
					resultText += content.Text + "\n"
				}
			}
		}

		// 检查结果是否为空或无效
		if resultText == "" || strings.TrimSpace(resultText) == "" {
			promptBuilder.WriteString("结果: **已忽略（结果为空）**\n\n")
			continue
		}

		if summary, ok := contextData.SummarizedItems[exec.ID]; ok {
			promptBuilder.WriteString(fmt.Sprintf("工具执行结果: [已总结] %s\n", summary))
		} else {
			if len(resultText) > 5000 {
				resultText = resultText[:5000] + "..."
			}
			promptBuilder.WriteString(fmt.Sprintf("工具执行结果: %s\n", resultText))
		}

		// 添加对应的AI分析（工具执行后AI的回复）
		if aiMessage, ok := execToMessageMap[exec.ID]; ok {
			aiContent := aiMessage.Content
			if len(aiContent) > 2000 {
				aiContent = aiContent[:2000] + "..."
			}
			promptBuilder.WriteString(fmt.Sprintf("AI分析: %s\n", aiContent))
		}

		promptBuilder.WriteString("\n")
	}

	promptBuilder.WriteString(`

## 输出格式

请以JSON格式返回攻击链，格式如下：

{
   "nodes": [
     {
       "id": "node_1",
       "type": "target|action|vulnerability",
       "label": "节点标签（清晰、简洁，action节点要描述"做了什么"和"发现了什么"）",
       "risk_score": 0-100,
       "tool_execution_id": "执行记录的真实ID（action节点必须使用上面执行记录中的ID字段）",
       "metadata": {
         "target": "目标（target节点）",
         "tool_name": "工具名称（action节点）",
         "tool_intent": "工具调用意图（action节点，如"端口扫描"、"漏洞扫描"）",
         "ai_analysis": "AI对工具结果的分析总结（action节点，不超过100字）",
         "findings": ["发现1", "发现2"]（action节点，关键发现列表）,
         "vulnerability_type": "漏洞类型（vulnerability节点）",
         "description": "描述（vulnerability节点）",
         "severity": "critical|high|medium|low（vulnerability节点）",
         "location": "漏洞位置（vulnerability节点）"
       }
     }
   ],
   "edges": [
     {
       "source": "node_1",
       "target": "node_2",
       "type": "leads_to|discovers|enables",
       "weight": 1-5
     }
   ]
}

## 重要要求

1. **节点合并**：
   - 每个工具执行和对应的AI分析必须合并为一个action节点
   - action节点的label要清晰描述"做了什么"和"发现了什么"
   - 例如："使用Nmap扫描192.168.1.1，发现22、80、443端口开放"

2. **过滤无效节点**：
   - **必须忽略**所有错误/失败的执行（已在上面标记为"已忽略"的）
   - **必须忽略**结果为空或无效的执行
   - 只保留有价值的、成功的节点

3. **简化结构**：
   - 只创建target、action、vulnerability三种节点
   - 不要创建discovery、decision等节点
   - 让攻击链清晰、有教育意义

4. **关联关系**：
   - target → action：目标指向属于它的所有行动（通过工具执行参数判断目标）
   - action → action：按时间顺序连接，但只连接有逻辑关系的
     - **重要：只连接属于同一目标的action节点，不同目标的action节点之间不应该连接**
   - action → vulnerability：行动发现的漏洞
   - vulnerability → vulnerability：漏洞间的因果关系
     - **重要：只连接属于同一目标的漏洞，不同目标的漏洞之间不应该连接**

5. **多目标处理（重要！）**：
   - 如果对话中测试了多个不同的目标（如先测试A网页，后测试B网页），必须：
     - 为每个不同的目标创建独立的target节点
     - 每个target节点只关联属于它的action和vulnerability节点
     - 不同目标的节点之间**不应该**建立任何关联关系
     - 这样会形成多个独立的攻击链分支，每个分支对应一个测试目标

6. **节点数量控制**：
   - 如果节点太多（>20个），优先保留最重要的节点
   - 合并相似的action节点（如同一工具的连续调用，如果结果相似）

只返回JSON，不要包含其他解释文字。`)

	return promptBuilder.String(), nil
}

// buildExecutionToMessageMap 构建工具执行ID到AI消息的映射
// 找到每个工具执行后AI的回复消息
func (b *Builder) buildExecutionToMessageMap(contextData *ContextData) map[string]database.Message {
	execToMessageMap := make(map[string]database.Message)

	// 遍历消息，找到包含工具执行ID的消息（通常是assistant消息）
	for _, msg := range contextData.Messages {
		if msg.Role != "assistant" {
			continue
		}

		// 检查消息中是否引用了工具执行ID
		// 通常工具执行后，AI会在回复中引用这些执行ID
		for _, execID := range msg.MCPExecutionIDs {
			// 找到对应的工具执行
			for _, exec := range contextData.Executions {
				if exec.ID == execID {
					// 如果这个执行还没有关联的消息，或者当前消息时间更晚，则更新
					if existingMsg, exists := execToMessageMap[execID]; !exists || msg.CreatedAt.After(existingMsg.CreatedAt) {
						execToMessageMap[execID] = msg
					}
					break
				}
			}
		}
	}

	// 如果通过MCPExecutionIDs找不到，尝试按时间顺序匹配
	// 找到每个工具执行后最近的assistant消息
	for _, exec := range contextData.Executions {
		if _, exists := execToMessageMap[exec.ID]; exists {
			continue
		}

		// 找到执行时间之后最近的assistant消息
		var closestMsg *database.Message
		for i := range contextData.Messages {
			msg := &contextData.Messages[i]
			if msg.Role == "assistant" && msg.CreatedAt.After(exec.StartTime) {
				if closestMsg == nil || msg.CreatedAt.Before(closestMsg.CreatedAt) {
					closestMsg = msg
				}
			}
		}

		if closestMsg != nil {
			execToMessageMap[exec.ID] = *closestMsg
		}
	}

	return execToMessageMap
}

// formatArguments 格式化工具参数
func (b *Builder) formatArguments(args map[string]interface{}) string {
	if args == nil {
		return "{}"
	}
	jsonData, _ := json.Marshal(args)
	return string(jsonData)
}

// compressLongestItem 压缩最长的子节点
func (b *Builder) compressLongestItem(ctx context.Context, contextData *ContextData) error {
	var longestID string
	var longestType string
	var longestContent string
	maxLength := 0

	// 查找最长的消息
	for _, msg := range contextData.Messages {
		if _, alreadySummarized := contextData.SummarizedItems[msg.ID]; alreadySummarized {
			continue
		}
		length := len(msg.Content)
		if length > maxLength {
			maxLength = length
			longestID = msg.ID
			longestType = "message"
			longestContent = msg.Content
		}
	}

	// 查找最长的工具执行结果
	for _, exec := range contextData.Executions {
		if _, alreadySummarized := contextData.SummarizedItems[exec.ID]; alreadySummarized {
			continue
		}
		if exec.Result != nil {
			var resultText string
			for _, content := range exec.Result.Content {
				if content.Type == "text" {
					resultText += content.Text + "\n"
				}
			}
			length := len(resultText)
			if length > maxLength {
				maxLength = length
				longestID = exec.ID
				longestType = "execution"
				longestContent = resultText
			}
		}
	}

	// 查找最长的思考过程
	for _, details := range contextData.ProcessDetails {
		for _, detail := range details {
			if detail.EventType == "thinking" {
				if _, alreadySummarized := contextData.SummarizedItems[detail.ID]; alreadySummarized {
					continue
				}
				length := len(detail.Message)
				if length > maxLength {
					maxLength = length
					longestID = detail.ID
					longestType = "thinking"
					longestContent = detail.Message
				}
			}
		}
	}

	if longestID == "" {
		return fmt.Errorf("没有找到需要压缩的内容")
	}

	b.logger.Info("压缩最长子节点",
		zap.String("id", longestID),
		zap.String("type", longestType),
		zap.Int("length", maxLength))

	// 使用AI总结
	summary, err := b.summarizeContent(ctx, longestType, longestContent)
	if err != nil {
		return fmt.Errorf("总结内容失败: %w", err)
	}

	// 保存总结
	contextData.SummarizedItems[longestID] = summary

	b.logger.Info("压缩完成",
		zap.String("id", longestID),
		zap.Int("originalLength", maxLength),
		zap.Int("summaryLength", len(summary)))

	return nil
}

// summarizeContent 总结内容
func (b *Builder) summarizeContent(ctx context.Context, contentType, content string) (string, error) {
	var prompt string
	switch contentType {
	case "message":
		prompt = fmt.Sprintf(`请总结以下AI回复的关键信息，保留所有重要的安全发现、漏洞信息和测试结果。用简洁的中文总结，不超过500字。

AI回复：
%s

总结：`, content)
	case "execution":
		prompt = fmt.Sprintf(`请总结以下工具执行结果的关键信息，保留所有发现的漏洞、重要发现和测试结果。用简洁的中文总结，不超过500字。

工具执行结果：
%s

总结：`, content)
	case "thinking":
		prompt = fmt.Sprintf(`请总结以下AI思考过程的关键决策和思路，保留所有重要的决策点和测试策略。用简洁的中文总结，不超过300字。

思考过程：
%s

总结：`, content)
	default:
		return "", fmt.Errorf("未知的内容类型: %s", contentType)
	}

	requestBody := map[string]interface{}{
		"model": b.openAIConfig.Model,
		"messages": []map[string]interface{}{
			{
				"role":    "system",
				"content": "你是一个专业的安全测试分析师，擅长总结安全测试相关的信息。请用简洁的中文总结关键信息。",
			},
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"temperature": 0.3,
		"max_tokens":  1000,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", b.openAIConfig.BaseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+b.openAIConfig.APIKey)

	resp, err := b.openAIClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API返回错误: %d, %s", resp.StatusCode, string(body))
	}

	var apiResponse struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	if len(apiResponse.Choices) == 0 {
		return "", fmt.Errorf("API未返回有效响应")
	}

	return strings.TrimSpace(apiResponse.Choices[0].Message.Content), nil
}

// callAIForChainGeneration 调用AI生成攻击链
func (b *Builder) callAIForChainGeneration(ctx context.Context, prompt string) (string, error) {
	requestBody := map[string]interface{}{
		"model": b.openAIConfig.Model,
		"messages": []map[string]interface{}{
			{
				"role":    "system",
				"content": "你是一个专业的安全测试分析师，擅长构建攻击链图。请严格按照JSON格式返回攻击链数据。",
			},
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"temperature": 0.3,
		"max_tokens":  8000,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", b.openAIConfig.BaseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+b.openAIConfig.APIKey)

	resp, err := b.openAIClient.Do(req)
	if err != nil {
		// 检查是否是上下文过长错误
		if strings.Contains(err.Error(), "context") || strings.Contains(err.Error(), "length") {
			return "", fmt.Errorf("context length exceeded")
		}
		return "", fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)
		// 检查是否是上下文过长错误
		if strings.Contains(bodyStr, "context") || strings.Contains(bodyStr, "length") || strings.Contains(bodyStr, "too long") {
			return "", fmt.Errorf("context length exceeded")
		}
		return "", fmt.Errorf("API返回错误: %d, %s", resp.StatusCode, bodyStr)
	}

	var apiResponse struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	if len(apiResponse.Choices) == 0 {
		return "", fmt.Errorf("API未返回有效响应")
	}

	content := strings.TrimSpace(apiResponse.Choices[0].Message.Content)
	// 尝试提取JSON（可能包含markdown代码块）
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	return content, nil
}

// ChainJSON 攻击链JSON结构
type ChainJSON struct {
	Nodes []struct {
		ID              string                 `json:"id"`
		Type            string                 `json:"type"`
		Label           string                 `json:"label"`
		RiskScore       int                    `json:"risk_score"`
		ToolExecutionID string                 `json:"tool_execution_id,omitempty"`
		Metadata        map[string]interface{} `json:"metadata"`
	} `json:"nodes"`
	Edges []struct {
		Source string `json:"source"`
		Target string `json:"target"`
		Type   string `json:"type"`
		Weight int    `json:"weight"`
	} `json:"edges"`
}

// parseChainJSON 解析攻击链JSON
func (b *Builder) parseChainJSON(chainJSON string, executions []*mcp.ToolExecution) (*Chain, error) {
	var chainData ChainJSON
	if err := json.Unmarshal([]byte(chainJSON), &chainData); err != nil {
		return nil, fmt.Errorf("解析JSON失败: %w", err)
	}

	// 创建execution ID映射（AI可能返回简单的索引或ID，需要映射到真实的execution ID）
	executionMap := make(map[string]string) // AI返回的ID -> 真实execution ID
	for i, exec := range executions {
		// 支持多种可能的AI返回格式
		executionMap[fmt.Sprintf("exec_%d", i+1)] = exec.ID
		executionMap[fmt.Sprintf("execution_%d", i+1)] = exec.ID
		executionMap[exec.ID] = exec.ID                     // 如果AI直接返回真实ID
		executionMap[fmt.Sprintf("tool_%d", i+1)] = exec.ID // AI可能用tool_1格式
		executionMap[fmt.Sprintf("执行%d", i+1)] = exec.ID    // 中文格式
		executionMap[fmt.Sprintf("执行_%d", i+1)] = exec.ID
	}

	// 创建节点ID映射（AI返回的ID -> 新的UUID）
	nodeIDMap := make(map[string]string)

	// 转换为Chain结构，并过滤无效节点
	nodes := make([]Node, 0, len(chainData.Nodes))
	for _, n := range chainData.Nodes {
		// 过滤无效节点
		if b.shouldFilterNode(n, executions) {
			b.logger.Info("过滤无效节点",
				zap.String("nodeID", n.ID),
				zap.String("nodeType", n.Type),
				zap.String("label", n.Label))
			continue
		}

		// 生成新的UUID节点ID
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

		// 处理tool_execution_id：如果是action或vulnerability节点，需要映射到真实的execution ID
		if n.ToolExecutionID != "" {
			if realExecID, ok := executionMap[n.ToolExecutionID]; ok {
				node.ToolExecutionID = realExecID
			} else {
				// 检查是否是真实的execution ID（UUID格式）
				// 如果是，直接使用；如果不是，尝试从节点ID推断
				if len(n.ToolExecutionID) > 20 { // UUID通常很长
					node.ToolExecutionID = n.ToolExecutionID
				} else {
					// 可能是简单的ID，尝试从节点ID推断
					if realExecID, ok := executionMap[n.ID]; ok {
						node.ToolExecutionID = realExecID
					} else {
						b.logger.Warn("无法映射tool_execution_id",
							zap.String("nodeID", n.ID),
							zap.String("toolExecutionID", n.ToolExecutionID))
						// 对于action节点，如果没有有效的execution ID，清空它（避免外键约束失败）
						if n.Type == "action" {
							node.ToolExecutionID = ""
						}
					}
				}
			}
		} else if n.Type == "action" || n.Type == "vulnerability" {
			// 如果AI没有提供tool_execution_id，尝试从节点ID推断
			// 例如：tool_1 -> 查找exec_1
			if realExecID, ok := executionMap[n.ID]; ok {
				node.ToolExecutionID = realExecID
			} else {
				b.logger.Warn("action/vulnerability节点缺少tool_execution_id",
					zap.String("nodeID", n.ID),
					zap.String("nodeType", n.Type))
			}
		}

		nodes = append(nodes, node)
	}

	// 转换边，更新source和target为新的节点ID
	edges := make([]Edge, 0, len(chainData.Edges))
	for _, e := range chainData.Edges {
		sourceID, ok := nodeIDMap[e.Source]
		if !ok {
			b.logger.Warn("边的源节点ID未找到", zap.String("source", e.Source))
			continue
		}

		targetID, ok := nodeIDMap[e.Target]
		if !ok {
			b.logger.Warn("边的目标节点ID未找到", zap.String("target", e.Target))
			continue
		}

		edge := Edge{
			ID:     fmt.Sprintf("edge_%s", uuid.New().String()),
			Source: sourceID,
			Target: targetID,
			Type:   e.Type,
			Weight: e.Weight,
		}
		edges = append(edges, edge)
	}

	// 过滤掉指向已删除节点的边
	filteredEdges := make([]Edge, 0, len(edges))
	for _, edge := range edges {
		// 检查source和target节点是否都存在
		sourceExists := false
		targetExists := false
		for _, node := range nodes {
			if node.ID == edge.Source {
				sourceExists = true
			}
			if node.ID == edge.Target {
				targetExists = true
			}
		}

		if sourceExists && targetExists {
			filteredEdges = append(filteredEdges, edge)
		} else {
			b.logger.Warn("过滤无效边",
				zap.String("edgeID", edge.ID),
				zap.String("source", edge.Source),
				zap.String("target", edge.Target),
				zap.Bool("sourceExists", sourceExists),
				zap.Bool("targetExists", targetExists))
		}
	}

	return &Chain{
		Nodes: nodes,
		Edges: filteredEdges,
	}, nil
}

// shouldFilterNode 判断是否应该过滤掉这个节点
func (b *Builder) shouldFilterNode(n struct {
	ID              string                 `json:"id"`
	Type            string                 `json:"type"`
	Label           string                 `json:"label"`
	RiskScore       int                    `json:"risk_score"`
	ToolExecutionID string                 `json:"tool_execution_id,omitempty"`
	Metadata        map[string]interface{} `json:"metadata"`
}, executions []*mcp.ToolExecution) bool {
	// 只允许target、action、vulnerability三种节点类型
	if n.Type != "target" && n.Type != "action" && n.Type != "vulnerability" {
		return true
	}

	// 对于action节点，检查对应的工具执行是否有效
	if n.Type == "action" {
		if n.ToolExecutionID == "" {
			// 没有关联工具执行的action节点，可能是无效的
			return true
		}

		// 查找对应的工具执行
		var exec *mcp.ToolExecution
		for _, e := range executions {
			if e.ID == n.ToolExecutionID {
				exec = e
				break
			}
		}

		if exec == nil {
			// 找不到对应的工具执行，可能是无效的
			return true
		}

		// 检查工具执行是否错误或失败
		if exec.Error != "" || (exec.Result != nil && exec.Result.IsError) {
			return true
		}

		// 检查工具执行结果是否为空
		if exec.Result == nil || len(exec.Result.Content) == 0 {
			return true
		}

		// 检查结果文本是否为空
		var resultText string
		for _, content := range exec.Result.Content {
			if content.Type == "text" {
				resultText += content.Text
			}
		}
		if strings.TrimSpace(resultText) == "" {
			return true
		}
	}

	// 检查节点标签是否为空或无效
	if strings.TrimSpace(n.Label) == "" {
		return true
	}

	// 检查标签中是否包含错误/失败的关键词
	labelLower := strings.ToLower(n.Label)
	errorKeywords := []string{"错误", "失败", "无效", "error", "failed", "invalid", "empty", "空"}
	for _, keyword := range errorKeywords {
		if strings.Contains(labelLower, keyword) {
			// 如果标签明确表示错误，但节点类型不是vulnerability，则过滤
			if n.Type != "vulnerability" {
				return true
			}
		}
	}

	return false
}
