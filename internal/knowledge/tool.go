package knowledge

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"cyberstrike-ai/internal/mcp"

	"go.uber.org/zap"
)

// RegisterKnowledgeTool 注册知识检索工具到MCP服务器
func RegisterKnowledgeTool(
	mcpServer *mcp.Server,
	retriever *Retriever,
	manager *Manager,
	logger *zap.Logger,
) {
	// manager 和 retriever 在 handler 中直接使用参数
	_ = manager // 保留参数，可能将来用于日志记录等
	tool := mcp.Tool{
		Name:             "search_knowledge_base",
		Description:      "在知识库中搜索相关的安全知识。当你需要了解特定漏洞类型、攻击技术、检测方法等安全知识时，可以使用此工具进行检索。工具使用向量检索和混合搜索技术，能够根据查询内容的语义相似度和关键词匹配，自动找到最相关的知识片段。",
		ShortDescription: "搜索知识库中的安全知识（支持向量检索和混合搜索）",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "搜索查询内容，描述你想要了解的安全知识主题",
				},
				"risk_type": map[string]interface{}{
					"type":        "string",
					"description": "可选：指定风险类型（如：SQL注入、XSS、文件上传等），如果不指定则搜索所有类型",
				},
			},
			"required": []string{"query"},
		},
	}

	handler := func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		query, ok := args["query"].(string)
		if !ok || query == "" {
			return &mcp.ToolResult{
				Content: []mcp.Content{
					{
						Type: "text",
						Text: "错误: 查询参数不能为空",
					},
				},
				IsError: true,
			}, nil
		}

		riskType := ""
		if rt, ok := args["risk_type"].(string); ok && rt != "" {
			riskType = rt
		}

		logger.Info("执行知识库检索",
			zap.String("query", query),
			zap.String("riskType", riskType),
		)

		// 执行检索
		searchReq := &SearchRequest{
			Query:    query,
			RiskType: riskType,
			TopK:     5,
		}

		results, err := retriever.Search(ctx, searchReq)
		if err != nil {
			logger.Error("知识库检索失败", zap.Error(err))
			return &mcp.ToolResult{
				Content: []mcp.Content{
					{
						Type: "text",
						Text: fmt.Sprintf("检索失败: %v", err),
					},
				},
				IsError: true,
			}, nil
		}

		if len(results) == 0 {
			return &mcp.ToolResult{
				Content: []mcp.Content{
					{
						Type: "text",
						Text: fmt.Sprintf("未找到与查询 '%s' 相关的知识。建议：\n1. 尝试使用不同的关键词\n2. 检查风险类型是否正确\n3. 确认知识库中是否包含相关内容", query),
					},
				},
			}, nil
		}

		// 格式化结果
		var resultText strings.Builder
		resultText.WriteString(fmt.Sprintf("找到 %d 条相关知识：\n\n", len(results)))

		// 收集检索到的知识项ID（用于日志）
		retrievedItemIDs := make([]string, 0, len(results))

		for i, result := range results {
			resultText.WriteString(fmt.Sprintf("--- 结果 %d (相似度: %.2f%%) ---\n", i+1, result.Similarity*100))
			resultText.WriteString(fmt.Sprintf("来源: [%s] %s\n", result.Item.Category, result.Item.Title))
			resultText.WriteString(fmt.Sprintf("内容:\n%s\n\n", result.Chunk.ChunkText))

			if !contains(retrievedItemIDs, result.Item.ID) {
				retrievedItemIDs = append(retrievedItemIDs, result.Item.ID)
			}
		}

		// 在结果末尾添加元数据（JSON格式，用于提取知识项ID）
		// 使用特殊标记，避免影响AI阅读结果
		if len(retrievedItemIDs) > 0 {
			metadataJSON, _ := json.Marshal(map[string]interface{}{
				"_metadata": map[string]interface{}{
					"retrievedItemIDs": retrievedItemIDs,
				},
			})
			resultText.WriteString(fmt.Sprintf("\n<!-- METADATA: %s -->", string(metadataJSON)))
		}

		// 记录检索日志（异步，不阻塞）
		// 注意：这里没有conversationID和messageID，需要在Agent层面记录
		// 实际的日志记录应该在Agent的progressCallback中完成

		return &mcp.ToolResult{
			Content: []mcp.Content{
				{
					Type: "text",
					Text: resultText.String(),
				},
			},
		}, nil
	}

	mcpServer.RegisterTool(tool, handler)
	logger.Info("知识检索工具已注册", zap.String("toolName", tool.Name))
}

// contains 检查切片是否包含元素
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// GetRetrievalMetadata 从工具调用中提取检索元数据（用于日志记录）
func GetRetrievalMetadata(args map[string]interface{}) (query string, riskType string) {
	if q, ok := args["query"].(string); ok {
		query = q
	}
	if rt, ok := args["risk_type"].(string); ok {
		riskType = rt
	}
	return
}

// FormatRetrievalResults 格式化检索结果为字符串（用于日志）
func FormatRetrievalResults(results []*RetrievalResult) string {
	if len(results) == 0 {
		return "未找到相关结果"
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("检索到 %d 条结果:\n", len(results)))

	itemIDs := make(map[string]bool)
	for i, result := range results {
		builder.WriteString(fmt.Sprintf("%d. [%s] %s (相似度: %.2f%%)\n",
			i+1, result.Item.Category, result.Item.Title, result.Similarity*100))
		itemIDs[result.Item.ID] = true
	}

	// 返回知识项ID列表（JSON格式）
	ids := make([]string, 0, len(itemIDs))
	for id := range itemIDs {
		ids = append(ids, id)
	}
	idsJSON, _ := json.Marshal(ids)
	builder.WriteString(fmt.Sprintf("\n检索到的知识项ID: %s", string(idsJSON)))

	return builder.String()
}
