package attackchain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"cyberstrike-ai/internal/agent"
	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/mcp"
	"cyberstrike-ai/internal/openai"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Builder 攻击链构建器
type Builder struct {
	db           *database.DB
	logger       *zap.Logger
	openAIClient *openai.Client
	openAIConfig *config.OpenAIConfig
	tokenCounter agent.TokenCounter
	maxTokens    int // 最大tokens限制，默认100000
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
		Proxy:               http.ProxyFromEnvironment,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}
	httpClient := &http.Client{Timeout: 5 * time.Minute, Transport: transport}

	maxTokens := 100000 // 默认100k tokens，可以根据模型调整
	// 根据模型设置合理的默认值
	if openAIConfig != nil {
		model := strings.ToLower(openAIConfig.Model)
		if strings.Contains(model, "gpt-4") {
			maxTokens = 128000 // gpt-4通常支持128k
		} else if strings.Contains(model, "gpt-3.5") {
			maxTokens = 16000 // gpt-3.5-turbo通常支持16k
		} else if strings.Contains(model, "deepseek") {
			maxTokens = 131072 // deepseek-chat通常支持131k
		}
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
	// 在第一次尝试前，先检查tokens并压缩（如果需要）
	totalTokens, err := b.countPromptTokens(contextData)
	if err == nil && totalTokens > b.maxTokens {
		b.logger.Info("检测到tokens超过限制，提前压缩",
			zap.Int("totalTokens", totalTokens),
			zap.Int("maxTokens", b.maxTokens))
		if err := b.compressContextData(ctx, contextData); err != nil {
			return nil, fmt.Errorf("压缩上下文失败: %w", err)
		}
	}

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

				// 使用分片压缩
				if err := b.compressContextData(ctx, contextData); err != nil {
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
**即便某些工具执行或漏洞挖掘没有成功，只要它们提供了关键线索、错误提示或下一步思路，也要被保留下来。**

## 任务要求

1. **节点类型（简化，只保留3种）**：
   - **target（目标）**：从用户输入中提取测试目标（IP、域名、URL等）
     - **重要：如果对话中测试了多个不同的目标（如先测试A网页，后测试B网页），必须为每个不同的目标创建独立的target节点**
     - 每个target节点只关联属于它的action节点（通过工具执行参数中的目标来判断）
     - 不同目标的action节点之间**不应该**建立关联关系
   - **action（行动）**：**工具执行 + AI分析结果 = 一个action节点**
     - 将每个工具执行和AI对该工具结果的分析合并为一个action节点
     - 节点标签应该清晰描述"做了什么"、"得到了什么结果或线索"（例如："使用Nmap扫描端口，发现22、80、443端口开放" 或 "尝试SQLmap，虽然失败但提示存在WAF拦截"）
     - 默认关注成功的执行；但如果执行失败却提供了有价值的线索（错误信息、资产指纹、下一步建议等），也要保留，记为"带线索的失败"行动
     - **重要：action节点必须关联到正确的target节点（通过工具执行参数判断目标）**
   - **vulnerability（漏洞）**：从工具执行结果和AI分析中提取的**真实漏洞**（不是所有发现都是漏洞）。若验证失败但能明确表明某个漏洞利用方向不可行，可作为行动节点的线索描述，而不是漏洞节点。

2. **过滤规则（重要！）**：
   - **默认忽略**彻底无效的信息：完全没有输出、没有任何线索的失败执行仍需过滤
   - **必须保留**下列失败执行：
     - 错误信息里包含了潜在线索、受限条件、可复现的报错
     - 虽未找到漏洞，但收集到了资产信息、技术栈或后续测试方向
     - 用户特别关注的失败尝试
   - **保留策略**：只要行动节点能给后续测试提供启发，就保留；否则忽略

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
     - ai_analysis: AI对工具结果的分析总结（简洁，不超过100字，失败节点需解释线索价值）
     - findings: 关键发现（列表）
     - status: "success" | "failed_insight"（失败但有价值的线索）
     - hints: ["下一步建议1", "限制条件2"]（失败节点可提供的线索列表）
   - vulnerability节点需要：type, description, severity, location

## 对话数据

`)

	// 添加消息
	promptBuilder.WriteString("\n### 对话消息\n\n")
	for i, msg := range contextData.Messages {
		promptBuilder.WriteString(fmt.Sprintf("消息%d [%s]:\n", i+1, msg.Role))

		isUserMessage := strings.EqualFold(msg.Role, "user")
		// 用户输入必须原样提供给攻击链模型
		if isUserMessage {
			promptBuilder.WriteString(fmt.Sprintf("%s\n\n", msg.Content))
		} else if summary, ok := contextData.SummarizedItems[msg.ID]; ok {
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

		statusText := "成功"
		if isError {
			statusText = "失败（可能包含线索）"
		}

		promptBuilder.WriteString(fmt.Sprintf("执行%d [%s] (ID: %s) - 状态: %s\n", i+1, exec.ToolName, exec.ID, statusText))
		promptBuilder.WriteString(fmt.Sprintf("参数: %s\n", b.formatArguments(exec.Arguments)))

		if isError && exec.Error != "" {
			promptBuilder.WriteString(fmt.Sprintf("错误信息: %s\n", exec.Error))
		}

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
		if strings.TrimSpace(resultText) == "" {
			if isError {
				promptBuilder.WriteString("工具执行结果: [失败但未返回正文]\n")
			} else {
				promptBuilder.WriteString("工具执行结果: **已忽略（结果为空）**\n\n")
				continue
			}
		} else {
			if summary, ok := contextData.SummarizedItems[exec.ID]; ok {
				promptBuilder.WriteString(fmt.Sprintf("工具执行结果: [已总结] %s\n", summary))
			} else {
				if len(resultText) > 5000 {
					resultText = resultText[:5000] + "..."
				}
				promptBuilder.WriteString(fmt.Sprintf("工具执行结果: %s\n", resultText))
			}
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
   - action节点的label要清晰描述"做了什么"、"结果/线索是什么"
   - 例如："使用Nmap扫描192.168.1.1，发现22、80、443端口开放" 或 "执行Sqlmap被WAF拦截，提示403并暴露防护厂商"
   - 若为失败但有线索的行动，请在metadata.status中标记为"failed_insight"，并在findings/hints里写清线索价值

2. **过滤无效节点**：
   - **必须忽略**没有任何输出、没有线索的失败执行
   - **必须保留**失败但提供关键线索的执行，确保metadata里解释清楚
   - 只保留对学习或溯源有帮助的节点

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

// countPromptTokens 计算prompt的总tokens数
func (b *Builder) countPromptTokens(contextData *ContextData) (int, error) {
	prompt, err := b.buildChainGenerationPrompt(contextData)
	if err != nil {
		return 0, fmt.Errorf("构建提示词失败: %w", err)
	}

	if b.tokenCounter == nil || b.openAIConfig == nil {
		// 如果没有token计数器或配置，使用简单的估算（4个字符=1个token）
		return len(prompt) / 4, nil
	}

	model := b.openAIConfig.Model
	if model == "" {
		model = "gpt-4" // 默认模型
	}

	count, err := b.tokenCounter.Count(model, prompt)
	if err != nil {
		// 如果计算失败，使用估算
		return len(prompt) / 4, nil
	}
	return count, nil
}

// compressContextData 使用分片压缩方式压缩上下文数据
func (b *Builder) compressContextData(ctx context.Context, contextData *ContextData) error {
	// 计算当前tokens
	totalTokens, err := b.countPromptTokens(contextData)
	if err != nil {
		return fmt.Errorf("计算tokens失败: %w", err)
	}

	b.logger.Info("开始压缩上下文",
		zap.Int("totalTokens", totalTokens),
		zap.Int("maxTokens", b.maxTokens))

	// 如果tokens在限制内，不需要压缩
	if totalTokens <= b.maxTokens {
		return nil
	}

	// 计算需要分成多少份
	numChunks := (totalTokens + b.maxTokens - 1) / b.maxTokens // 向上取整
	if numChunks < 2 {
		numChunks = 2 // 至少分成2份
	}

	b.logger.Info("将上下文分成多个片段进行压缩",
		zap.Int("totalTokens", totalTokens),
		zap.Int("maxTokens", b.maxTokens),
		zap.Int("numChunks", numChunks))

	// 按时间顺序将数据分成多个片段
	chunks, err := b.splitContextDataByTime(contextData, numChunks)
	if err != nil {
		return fmt.Errorf("分割上下文数据失败: %w", err)
	}

	// 对每个片段进行摘要
	summaries := make([]string, 0, len(chunks))
	for i, chunk := range chunks {
		b.logger.Info("压缩片段",
			zap.Int("chunkIndex", i+1),
			zap.Int("totalChunks", len(chunks)),
			zap.Int("chunkSize", len(chunk.Messages)+len(chunk.Executions)))

		summary, err := b.summarizeContextChunk(ctx, chunk)
		if err != nil {
			// 检查是否是认证错误
			if strings.Contains(err.Error(), "Authentication") || strings.Contains(err.Error(), "api key") || strings.Contains(err.Error(), "invalid") {
				return fmt.Errorf("压缩片段%d失败（API认证错误，请检查OpenAI配置）: %w", i+1, err)
			}
			return fmt.Errorf("压缩片段%d失败: %w", i+1, err)
		}
		summaries = append(summaries, summary)
	}

	// 将摘要合并到contextData中
	// 保留用户消息，清空其他数据，用摘要替换
	var userMessages []database.Message
	for _, msg := range contextData.Messages {
		if strings.EqualFold(msg.Role, "user") {
			userMessages = append(userMessages, msg)
		}
	}

	// 清空非用户消息和执行记录
	contextData.Messages = userMessages
	contextData.Executions = []*mcp.ToolExecution{}
	contextData.ProcessDetails = make(map[string][]database.ProcessDetail)

	// 创建一个综合摘要消息
	combinedSummary := strings.Join(summaries, "\n\n---\n\n")
	summaryMsg := database.Message{
		ID:        uuid.New().String(),
		Role:      "assistant",
		Content:   fmt.Sprintf("[上下文摘要 - 包含%d个片段的压缩内容]\n\n%s", len(summaries), combinedSummary),
		CreatedAt: time.Now(),
	}
	contextData.Messages = append(contextData.Messages, summaryMsg)

	// 检查压缩后的tokens
	compressedTokens, err := b.countPromptTokens(contextData)
	if err != nil {
		return fmt.Errorf("计算压缩后tokens失败: %w", err)
	}

	b.logger.Info("压缩完成",
		zap.Int("originalTokens", totalTokens),
		zap.Int("compressedTokens", compressedTokens),
		zap.Int("reduction", totalTokens-compressedTokens))

	// 如果压缩后仍然超过限制，递归压缩
	if compressedTokens > b.maxTokens {
		b.logger.Info("压缩后仍然超过限制，继续递归压缩",
			zap.Int("compressedTokens", compressedTokens),
			zap.Int("maxTokens", b.maxTokens))
		return b.compressContextData(ctx, contextData)
	}

	return nil
}

// ContextChunk 上下文数据片段
type ContextChunk struct {
	Messages       []database.Message
	Executions     []*mcp.ToolExecution
	ProcessDetails map[string][]database.ProcessDetail
}

// splitContextDataByTime 按时间顺序将上下文数据分成多个片段
func (b *Builder) splitContextDataByTime(contextData *ContextData, numChunks int) ([]*ContextChunk, error) {
	if numChunks <= 0 {
		return nil, fmt.Errorf("片段数量必须大于0")
	}

	// 收集所有带时间戳的项目
	type timeItem struct {
		time          time.Time
		itemType      string // "message", "execution", "thinking"
		message       *database.Message
		execution     *mcp.ToolExecution
		processDetail *database.ProcessDetail
	}

	var items []timeItem

	// 添加消息（跳过已总结的）
	for i := range contextData.Messages {
		msg := &contextData.Messages[i]
		if _, alreadySummarized := contextData.SummarizedItems[msg.ID]; alreadySummarized {
			continue
		}
		items = append(items, timeItem{
			time:     msg.CreatedAt,
			itemType: "message",
			message:  msg,
		})
	}

	// 添加工具执行（跳过已总结的）
	for _, exec := range contextData.Executions {
		if _, alreadySummarized := contextData.SummarizedItems[exec.ID]; alreadySummarized {
			continue
		}
		items = append(items, timeItem{
			time:      exec.StartTime,
			itemType:  "execution",
			execution: exec,
		})
	}

	// 添加思考过程（跳过已总结的）
	for _, details := range contextData.ProcessDetails {
		for i := range details {
			detail := &details[i]
			if detail.EventType == "thinking" {
				if _, alreadySummarized := contextData.SummarizedItems[detail.ID]; alreadySummarized {
					continue
				}
				items = append(items, timeItem{
					time:          detail.CreatedAt,
					itemType:      "thinking",
					processDetail: detail,
				})
			}
		}
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("没有可分割的数据")
	}

	// 按时间排序
	sort.Slice(items, func(i, j int) bool {
		return items[i].time.Before(items[j].time)
	})

	// 计算每个片段的大小
	chunkSize := (len(items) + numChunks - 1) / numChunks // 向上取整

	// 创建片段
	chunks := make([]*ContextChunk, 0, numChunks)
	for i := 0; i < len(items); i += chunkSize {
		end := i + chunkSize
		if end > len(items) {
			end = len(items)
		}

		chunk := &ContextChunk{
			Messages:       []database.Message{},
			Executions:     []*mcp.ToolExecution{},
			ProcessDetails: make(map[string][]database.ProcessDetail),
		}

		for j := i; j < end; j++ {
			item := items[j]
			switch item.itemType {
			case "message":
				chunk.Messages = append(chunk.Messages, *item.message)
			case "execution":
				chunk.Executions = append(chunk.Executions, item.execution)
			case "thinking":
				if item.processDetail != nil {
					msgID := item.processDetail.MessageID
					chunk.ProcessDetails[msgID] = append(chunk.ProcessDetails[msgID], *item.processDetail)
				}
			}
		}

		chunks = append(chunks, chunk)
	}

	return chunks, nil
}

// getModelMaxContextLength 获取模型的最大上下文长度
func (b *Builder) getModelMaxContextLength() int {
	if b.openAIConfig == nil {
		return 131072 // 默认值
	}
	model := strings.ToLower(b.openAIConfig.Model)
	if strings.Contains(model, "gpt-4") {
		return 128000
	} else if strings.Contains(model, "gpt-3.5") {
		return 16000
	} else if strings.Contains(model, "deepseek") {
		return 131072
	}
	return 131072 // 默认值
}

// summarizeContextChunk 总结一个上下文片段
func (b *Builder) summarizeContextChunk(ctx context.Context, chunk *ContextChunk) (string, error) {
	// 先构建内容
	content, err := b.buildChunkContent(chunk)
	if err != nil {
		return "", err
	}

	// 使用AI总结
	promptTemplate := `请详细总结以下安全测试对话片段的关键信息。虽然需要压缩内容，但必须保留所有重要的技术细节和上下文信息，确保后续攻击链生成时能够准确理解整个测试过程。

**必须详细保留的内容：**
1. **所有工具执行记录**：
   - 工具名称、执行参数、执行结果（包括成功和失败）
   - 失败执行的错误信息、状态码、响应头等关键线索
   - 工具输出的关键数据（端口、服务版本、漏洞信息等）
   - 每个工具执行的时间顺序和上下文关系

2. **所有发现的漏洞和潜在安全问题**：
   - 漏洞类型、严重程度、位置、利用方式
   - 验证过程和结果
   - 漏洞之间的关联关系

3. **所有测试目标和资产信息**：
   - IP地址、域名、URL、端口等
   - 发现的服务、技术栈、版本信息
   - 资产之间的关联关系

4. **所有测试步骤和决策过程**：
   - 每个测试步骤的详细描述（做了什么、为什么做、结果如何）
   - AI的分析思路和决策依据
   - 失败尝试的原因和从中获得的线索

5. **所有关键发现和线索**：
   - 成功发现的详细信息
   - 失败但提供线索的尝试（错误信息、限制条件、下一步建议等）
   - 收集到的任何有价值的信息（凭据、令牌、配置信息等）

**总结要求：**
- 用结构化的方式组织信息，按时间顺序或逻辑顺序排列
- 对于每个工具执行，必须包含：工具名、目标、参数、结果/错误、关键发现
- 对于每个漏洞，必须包含：类型、位置、严重程度、验证结果
- 保留所有技术细节，不要过度简化
- 确保后续AI能够根据这个摘要完整重建攻击链

对话片段：
%s

请给出详细且结构化的技术摘要（建议1000-2000字，确保信息完整）：`

	// 检查prompt tokens，如果超过限制，需要进一步压缩内容
	maxContextLength := b.getModelMaxContextLength()
	maxPromptTokens := maxContextLength - 2000 // 留出空间给响应和系统消息

	// 尝试构建完整prompt并检查tokens
	fullPrompt := fmt.Sprintf(promptTemplate, content)
	promptTokens, err := b.countTextTokens(fullPrompt)
	if err != nil {
		// 如果计算失败，使用估算
		promptTokens = len(fullPrompt) / 4
	}

	// 如果prompt太大，需要进一步压缩内容
	if promptTokens > maxPromptTokens {
		b.logger.Warn("片段内容过大，需要进一步压缩",
			zap.Int("promptTokens", promptTokens),
			zap.Int("maxPromptTokens", maxPromptTokens))

		// 递归压缩：将chunk进一步分割
		compressedContent, err := b.compressLargeChunk(ctx, chunk, maxPromptTokens)
		if err != nil {
			return "", fmt.Errorf("压缩大片段失败: %w", err)
		}
		content = compressedContent
	}

	prompt := fmt.Sprintf(promptTemplate, content)

	// 检查配置
	if b.openAIConfig == nil {
		return "", fmt.Errorf("OpenAI配置未初始化")
	}
	if b.openAIConfig.APIKey == "" {
		return "", fmt.Errorf("OpenAI API Key未配置")
	}
	if b.openAIConfig.Model == "" {
		return "", fmt.Errorf("OpenAI Model未配置")
	}

	// 直接调用AI API进行总结
	requestBody := map[string]interface{}{
		"model": b.openAIConfig.Model,
		"messages": []map[string]interface{}{
			{
				"role": "system",
				"content": `你是一个资深的安全测试分析师和渗透测试专家，拥有丰富的实战经验。你的任务是总结安全测试对话片段，这些摘要将用于后续构建完整的攻击链图。

**你的专业背景：**
- 精通各种安全测试工具（Nmap、SQLMap、Burp Suite、Metasploit等）的使用和结果分析
- 熟悉常见漏洞类型（SQL注入、XSS、文件上传、命令执行、目录遍历等）的识别和验证
- 理解攻击链的构建逻辑：从信息收集 → 漏洞发现 → 漏洞利用 → 权限提升 → 横向移动
- 能够识别失败尝试中的有价值线索（错误信息、状态码、WAF指纹、技术栈信息等）

**你的总结原则：**
1. **完整性优先**：虽然需要压缩，但必须保留所有技术细节，确保后续AI能够完整重建攻击链
2. **结构化组织**：按时间顺序或逻辑顺序组织信息，让信息易于理解和追踪
3. **技术精准**：使用准确的技术术语，保留具体的数值、版本号、端口号、URL等关键数据
4. **上下文关联**：保留测试步骤之间的因果关系和逻辑关联
5. **失败价值**：即使是失败的尝试，只要提供了线索（错误信息、限制条件、下一步建议），也要详细记录

**你需要特别关注的信息类型：**
- 工具执行：工具名、目标、参数、完整结果（包括错误和失败）
- 漏洞发现：类型、位置、严重程度、验证方法、利用结果
- 资产信息：IP、域名、端口、服务版本、技术栈
- 测试策略：为什么选择这个工具、为什么测试这个目标、发现了什么线索
- 关键数据：凭据、令牌、配置信息、敏感文件内容

请用专业、详细、结构化的中文进行总结，确保信息完整且易于后续处理。`,
			},
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"temperature": 0.3,
		"max_tokens":  4000, // 增加摘要长度，以容纳更详细的内容
	}

	var apiResponse struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if b.openAIClient == nil {
		return "", fmt.Errorf("OpenAI客户端未初始化")
	}
	if err := b.openAIClient.ChatCompletion(ctx, requestBody, &apiResponse); err != nil {
		return "", fmt.Errorf("请求失败: %w", err)
	}

	if len(apiResponse.Choices) == 0 {
		return "", fmt.Errorf("API未返回有效响应")
	}

	return strings.TrimSpace(apiResponse.Choices[0].Message.Content), nil
}

// compressLongestItem 压缩最长的子节点（保留作为备用方法）
func (b *Builder) compressLongestItem(ctx context.Context, contextData *ContextData) error {
	// 使用新的分片压缩方法
	return b.compressContextData(ctx, contextData)
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
				"role": "system",
				"content": `你是一个资深的安全测试分析师和渗透测试专家，拥有丰富的实战经验。你的任务是总结安全测试过程中的关键信息，这些摘要将用于构建攻击链图。

**你的专业背景：**
- 精通各种安全测试工具的使用和结果分析（Nmap、SQLMap、Burp Suite、Metasploit、Nuclei等）
- 熟悉常见漏洞类型的识别和验证（SQL注入、XSS、文件上传、命令执行、目录遍历、SSRF等）
- 理解攻击链的构建逻辑和测试流程
- 能够识别失败尝试中的有价值线索

**你的总结原则：**
1. **保留技术细节**：保留所有重要的技术信息，包括工具名、参数、结果、错误信息、状态码等
2. **突出关键发现**：重点记录发现的漏洞、安全问题、资产信息、凭据等
3. **记录失败线索**：即使是失败的尝试，如果提供了错误信息、限制条件或下一步建议，也要详细记录
4. **保持准确性**：使用准确的技术术语，保留具体的数值、版本号、端口号等关键数据
5. **结构化表达**：用清晰、有条理的方式组织信息

**根据内容类型，你需要特别关注：**
- **AI回复**：提取安全发现、漏洞信息、测试结果、分析思路、决策依据
- **工具执行**：记录工具名、目标、参数、完整结果（成功或失败）、关键发现、错误信息
- **思考过程**：提取关键决策点、测试策略、分析思路、下一步计划

请用专业、准确、简洁的中文进行总结，确保信息完整且易于理解。`,
			},
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"temperature": 0.3,
		"max_tokens":  1000,
	}

	var apiResponse struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if b.openAIClient == nil {
		return "", fmt.Errorf("OpenAI客户端未初始化")
	}
	if err := b.openAIClient.ChatCompletion(ctx, requestBody, &apiResponse); err != nil {
		return "", fmt.Errorf("请求失败: %w", err)
	}

	if len(apiResponse.Choices) == 0 {
		return "", fmt.Errorf("API未返回有效响应")
	}

	return strings.TrimSpace(apiResponse.Choices[0].Message.Content), nil
}

// buildChunkContent 构建chunk的文本内容
func (b *Builder) buildChunkContent(chunk *ContextChunk) (string, error) {
	var contentBuilder strings.Builder

	// 添加消息
	for _, msg := range chunk.Messages {
		if strings.EqualFold(msg.Role, "user") {
			contentBuilder.WriteString(fmt.Sprintf("用户消息: %s\n\n", msg.Content))
		} else {
			contentBuilder.WriteString(fmt.Sprintf("AI回复: %s\n\n", msg.Content))
		}
	}

	// 添加工具执行
	for _, exec := range chunk.Executions {
		contentBuilder.WriteString(fmt.Sprintf("工具执行 [%s] (ID: %s):\n", exec.ToolName, exec.ID))
		contentBuilder.WriteString(fmt.Sprintf("参数: %s\n", b.formatArguments(exec.Arguments)))

		if exec.Error != "" {
			contentBuilder.WriteString(fmt.Sprintf("错误: %s\n", exec.Error))
		}

		if exec.Result != nil {
			var resultText string
			for _, content := range exec.Result.Content {
				if content.Type == "text" {
					resultText += content.Text + "\n"
				}
			}
			if resultText != "" {
				// 如果结果太长，截断
				if len(resultText) > 10000 {
					resultText = resultText[:10000] + "\n... [内容已截断]"
				}
				contentBuilder.WriteString(fmt.Sprintf("结果: %s\n", resultText))
			}
		}
		contentBuilder.WriteString("\n")
	}

	// 添加思考过程
	for _, details := range chunk.ProcessDetails {
		for _, detail := range details {
			if detail.EventType == "thinking" {
				thinkingText := detail.Message
				// 如果思考过程太长，截断
				if len(thinkingText) > 5000 {
					thinkingText = thinkingText[:5000] + "\n... [内容已截断]"
				}
				contentBuilder.WriteString(fmt.Sprintf("思考过程: %s\n\n", thinkingText))
			}
		}
	}

	content := contentBuilder.String()
	if content == "" {
		return "", fmt.Errorf("片段内容为空")
	}
	return content, nil
}

// compressLargeChunk 压缩过大的chunk（递归分割）
func (b *Builder) compressLargeChunk(ctx context.Context, chunk *ContextChunk, maxTokens int) (string, error) {
	// 将chunk进一步分割成更小的子chunk
	// 简单策略：按消息和执行数量平均分割
	totalItems := len(chunk.Messages) + len(chunk.Executions)
	if totalItems <= 1 {
		// 如果只有一个项目，直接截断内容
		content, _ := b.buildChunkContent(chunk)
		if len(content) > maxTokens*4 { // 粗略估算：1 token ≈ 4字符
			content = content[:maxTokens*4] + "\n... [内容过大，已截断]"
		}
		return content, nil
	}

	// 分成2个子chunk
	mid := totalItems / 2
	subChunk1 := &ContextChunk{
		Messages:       []database.Message{},
		Executions:     []*mcp.ToolExecution{},
		ProcessDetails: make(map[string][]database.ProcessDetail),
	}
	subChunk2 := &ContextChunk{
		Messages:       []database.Message{},
		Executions:     []*mcp.ToolExecution{},
		ProcessDetails: make(map[string][]database.ProcessDetail),
	}

	// 分配消息
	for i, msg := range chunk.Messages {
		if i < mid {
			subChunk1.Messages = append(subChunk1.Messages, msg)
		} else {
			subChunk2.Messages = append(subChunk2.Messages, msg)
		}
	}

	// 分配执行
	execStart := len(chunk.Messages)
	for i, exec := range chunk.Executions {
		if execStart+i < mid {
			subChunk1.Executions = append(subChunk1.Executions, exec)
		} else {
			subChunk2.Executions = append(subChunk2.Executions, exec)
		}
	}

	// 递归压缩子chunk
	summary1, err := b.summarizeContextChunk(ctx, subChunk1)
	if err != nil {
		return "", fmt.Errorf("压缩子chunk1失败: %w", err)
	}

	summary2, err := b.summarizeContextChunk(ctx, subChunk2)
	if err != nil {
		return "", fmt.Errorf("压缩子chunk2失败: %w", err)
	}

	// 合并摘要
	return fmt.Sprintf("片段1摘要：\n%s\n\n---\n\n片段2摘要：\n%s", summary1, summary2), nil
}

// countTextTokens 计算文本的tokens数
func (b *Builder) countTextTokens(text string) (int, error) {
	if b.tokenCounter == nil || b.openAIConfig == nil {
		return len(text) / 4, nil
	}

	model := b.openAIConfig.Model
	if model == "" {
		model = "gpt-4"
	}

	count, err := b.tokenCounter.Count(model, text)
	if err != nil {
		return len(text) / 4, nil
	}
	return count, nil
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

	var apiResponse struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if b.openAIClient == nil {
		return "", fmt.Errorf("OpenAI客户端未初始化")
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
		return "", fmt.Errorf("请求失败: %w", err)
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

	// 检查节点标签是否为空或无效
	if strings.TrimSpace(n.Label) == "" {
		return true
	}

	// 对于vulnerability节点，即使没有tool_execution_id也应该保留（漏洞可能不是直接来自工具执行）
	if n.Type == "vulnerability" {
		// 只要标签有意义就保留
		return false
	}

	// 对于target节点，只要标签有意义就保留
	if n.Type == "target" {
		return false
	}

	// 对于action节点，进行更宽松的检查
	if n.Type == "action" {
		// 如果executions为空（可能是压缩后的场景），只要标签有意义就保留
		if len(executions) == 0 {
			// 压缩场景下，只要标签不是明显无效就保留
			labelLower := strings.ToLower(n.Label)
			// 只过滤明显无效的标签
			invalidKeywords := []string{"空节点", "无效节点", "empty node", "invalid node"}
			for _, keyword := range invalidKeywords {
				if strings.Contains(labelLower, keyword) {
					return true
				}
			}
			return false
		}

		// 如果有tool_execution_id，尝试查找对应的工具执行
		if n.ToolExecutionID != "" {
			var exec *mcp.ToolExecution
			for _, e := range executions {
				if e.ID == n.ToolExecutionID {
					exec = e
					break
				}
			}

			if exec != nil {
				// 找到了对应的工具执行，检查是否有效
				// 检查工具执行是否错误或失败
				if exec.Error != "" || (exec.Result != nil && exec.Result.IsError) {
					// 失败但有线索的应该保留
					if !hasInsightfulFailure(n.Metadata) {
						// 即使没有明确标记为有线索，如果标签描述了具体内容，也保留
						labelLower := strings.ToLower(n.Label)
						// 如果标签包含具体的技术信息（端口、服务、漏洞等），说明有价值
						valuableKeywords := []string{"端口", "服务", "漏洞", "扫描", "发现", "获取", "验证", "port", "service", "vulnerability", "scan", "found", "discover"}
						hasValuableInfo := false
						for _, keyword := range valuableKeywords {
							if strings.Contains(labelLower, keyword) {
								hasValuableInfo = true
								break
							}
						}
						if !hasValuableInfo {
							return true
						}
					}
				}

				// 检查工具执行结果是否为空
				if exec.Result == nil || len(exec.Result.Content) == 0 {
					// 结果为空，但如果有线索或标签有意义，也保留
					if !hasInsightfulFailure(n.Metadata) {
						labelLower := strings.ToLower(n.Label)
						valuableKeywords := []string{"端口", "服务", "漏洞", "扫描", "发现", "获取", "验证", "port", "service", "vulnerability", "scan", "found", "discover"}
						hasValuableInfo := false
						for _, keyword := range valuableKeywords {
							if strings.Contains(labelLower, keyword) {
								hasValuableInfo = true
								break
							}
						}
						if !hasValuableInfo {
							return true
						}
					}
				} else {
					// 检查结果文本是否为空
					var resultText string
					for _, content := range exec.Result.Content {
						if content.Type == "text" {
							resultText += content.Text
						}
					}
					if strings.TrimSpace(resultText) == "" {
						// 结果文本为空，但如果有线索或标签有意义，也保留
						if !hasInsightfulFailure(n.Metadata) {
							labelLower := strings.ToLower(n.Label)
							valuableKeywords := []string{"端口", "服务", "漏洞", "扫描", "发现", "获取", "验证", "port", "service", "vulnerability", "scan", "found", "discover"}
							hasValuableInfo := false
							for _, keyword := range valuableKeywords {
								if strings.Contains(labelLower, keyword) {
									hasValuableInfo = true
									break
								}
							}
							if !hasValuableInfo {
								return true
							}
						}
					}
				}
			} else {
				// 找不到对应的工具执行，但可能是压缩后的场景
				// 只要标签有意义就保留（不要因为找不到execution就过滤掉）
				labelLower := strings.ToLower(n.Label)
				invalidKeywords := []string{"空节点", "无效节点", "empty node", "invalid node"}
				for _, keyword := range invalidKeywords {
					if strings.Contains(labelLower, keyword) {
						return true
					}
				}
				// 标签有意义，保留
				return false
			}
		} else {
			// 没有tool_execution_id，但可能是压缩后的场景或AI生成的节点
			// 只要标签有意义就保留
			labelLower := strings.ToLower(n.Label)
			invalidKeywords := []string{"空节点", "无效节点", "empty node", "invalid node"}
			for _, keyword := range invalidKeywords {
				if strings.Contains(labelLower, keyword) {
					return true
				}
			}
			// 标签有意义，保留
			return false
		}
	}

	// 默认保留（已经通过了所有检查）
	return false
}

func hasInsightfulFailure(metadata map[string]interface{}) bool {
	if metadata == nil {
		return false
	}

	if status, ok := metadata["status"].(string); ok {
		normalized := strings.ToLower(strings.TrimSpace(status))
		if normalized == "failed_insight" || normalized == "failed_clue" || normalized == "failed_with_hint" {
			return true
		}
	}

	if hint, ok := metadata["hint"].(string); ok && strings.TrimSpace(hint) != "" {
		return true
	}

	if hints, ok := metadata["hints"].([]interface{}); ok && len(hints) > 0 {
		return true
	}

	if insight, ok := metadata["insight"].(string); ok && strings.TrimSpace(insight) != "" {
		return true
	}

	return false
}
