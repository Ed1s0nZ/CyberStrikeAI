package knowledge

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"cyberstrike-ai/internal/mcp"
	"cyberstrike-ai/internal/mcp/builtin"

	"go.uber.org/zap"
)

// RegisterKnowledgeTool registers the knowledge retrieval tool with the MCP server
func RegisterKnowledgeTool(
	mcpServer *mcp.Server,
	retriever *Retriever,
	manager *Manager,
	logger *zap.Logger,
) {
	// register first tool: get all available risk type lists
	listRiskTypesTool := mcp.Tool{
		Name:             builtin.ToolListKnowledgeRiskTypes,
		Description:      "Get a list of all available risk types (risk_type) in the knowledge base. Before searching the knowledge base, you can call this tool first to get the available risk types, then use the correct risk type for a precise search, which can greatly reduce retrieval time and improve retrieval accuracy.",
		ShortDescription: "Get a list of all available risk types in the knowledge base",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
			"required":   []string{},
		},
	}

	listRiskTypesHandler := func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		categories, err := manager.GetCategories()
		if err != nil {
			logger.Error("failed to get risk type list", zap.Error(err))
			return &mcp.ToolResult{
				Content: []mcp.Content{
					{
						Type: "text",
						Text: fmt.Sprintf("failed to get risk type list: %v", err),
					},
				},
				IsError: true,
			}, nil
		}

		if len(categories) == 0 {
			return &mcp.ToolResult{
				Content: []mcp.Content{
					{
						Type: "text",
						Text: "No risk types currently in the knowledge base.",
					},
				},
			}, nil
		}

		var resultText strings.Builder
		resultText.WriteString(fmt.Sprintf("The knowledge base contains %d risk type(s):\n\n", len(categories)))
		for i, category := range categories {
			resultText.WriteString(fmt.Sprintf("%d. %s\n", i+1, category))
		}
		resultText.WriteString("\nTip: When calling the " + builtin.ToolSearchKnowledgeBase + " tool, you can use one of the above risk types as the risk_type parameter to narrow the search scope and improve retrieval efficiency.")

		return &mcp.ToolResult{
			Content: []mcp.Content{
				{
					Type: "text",
					Text: resultText.String(),
				},
			},
		}, nil
	}

	mcpServer.RegisterTool(listRiskTypesTool, listRiskTypesHandler)
	logger.Info("risk type list tool registered", zap.String("toolName", listRiskTypesTool.Name))

	// register second tool: search knowledge base (preserves original functionality)
	searchTool := mcp.Tool{
		Name:             builtin.ToolSearchKnowledgeBase,
		Description:      "Search for relevant security knowledge in the knowledge base. When you need to learn about a specific vulnerability type, attack technique, detection method, or other security knowledge, you can use this tool to retrieve it. The tool uses vector retrieval and hybrid search techniques to automatically find the most relevant knowledge snippets based on semantic similarity and keyword matching. Tip: Before searching, you can first call the " + builtin.ToolListKnowledgeRiskTypes + " tool to get available risk types, then use the correct risk_type parameter for a precise search, which can greatly reduce retrieval time.",
		ShortDescription: "Search security knowledge in the knowledge base (supports vector retrieval and hybrid search)",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search query content, describing the security knowledge topic you want to learn about",
				},
				"risk_type": map[string]interface{}{
					"type":        "string",
					"description": "Optional: specify a risk type (e.g. SQL injection, XSS, file upload, etc.). It is recommended to first call the " + builtin.ToolListKnowledgeRiskTypes + " tool to get the available risk type list, then use the correct risk type for a precise search, which can greatly reduce retrieval time. If not specified, all types are searched.",
				},
			},
			"required": []string{"query"},
		},
	}

	searchHandler := func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		query, ok := args["query"].(string)
		if !ok || query == "" {
			return &mcp.ToolResult{
				Content: []mcp.Content{
					{
						Type: "text",
						Text: "error: query parameter cannot be empty",
					},
				},
				IsError: true,
			}, nil
		}

		riskType := ""
		if rt, ok := args["risk_type"].(string); ok && rt != "" {
			riskType = rt
		}

		logger.Info("executing knowledge base retrieval",
			zap.String("query", query),
			zap.String("riskType", riskType),
		)

		// execute retrieval
		searchReq := &SearchRequest{
			Query:    query,
			RiskType: riskType,
			TopK:     5,
		}

		results, err := retriever.Search(ctx, searchReq)
		if err != nil {
			logger.Error("knowledge base retrieval failed", zap.Error(err))
			return &mcp.ToolResult{
				Content: []mcp.Content{
					{
						Type: "text",
						Text: fmt.Sprintf("retrieval failed: %v", err),
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
						Text: fmt.Sprintf("No knowledge related to query '%s' was found. Suggestions:\n1. Try different keywords\n2. Check if the risk type is correct\n3. Confirm that the knowledge base contains relevant content", query),
					},
				},
			}, nil
		}

		// format results
		var resultText strings.Builder

		// sort by hybrid score first to ensure document order is by hybrid score (core of hybrid retrieval)
		sort.Slice(results, func(i, j int) bool {
			return results[i].Score > results[j].Score
		})

		// group results by document for better context display
		// use an ordered slice to maintain document order (by highest hybrid score)
		type itemGroup struct {
			itemID   string
			results  []*RetrievalResult
			maxScore float64 // highest hybrid score for this document
		}
		itemGroups := make([]*itemGroup, 0)
		itemMap := make(map[string]*itemGroup)

		for _, result := range results {
			itemID := result.Item.ID
			group, exists := itemMap[itemID]
			if !exists {
				group = &itemGroup{
					itemID:   itemID,
					results:  make([]*RetrievalResult, 0),
					maxScore: result.Score,
				}
				itemMap[itemID] = group
				itemGroups = append(itemGroups, group)
			}
			group.results = append(group.results, result)
			if result.Score > group.maxScore {
				group.maxScore = result.Score
			}
		}

		// sort document groups by highest hybrid score
		sort.Slice(itemGroups, func(i, j int) bool {
			return itemGroups[i].maxScore > itemGroups[j].maxScore
		})

		// collect retrieved knowledge item IDs (for logging)
		retrievedItemIDs := make([]string, 0, len(itemGroups))

		resultText.WriteString(fmt.Sprintf("Found %d related knowledge item(s) (with context expansion):\n\n", len(results)))

		resultIndex := 1
		for _, group := range itemGroups {
			itemResults := group.results
			// find the one with the highest hybrid score as the main result (use hybrid score, not similarity)
			mainResult := itemResults[0]
			maxScore := mainResult.Score
			for _, result := range itemResults {
				if result.Score > maxScore {
					maxScore = result.Score
					mainResult = result
				}
			}

			// sort by chunk_index to ensure logical reading order (original document order)
			sort.Slice(itemResults, func(i, j int) bool {
				return itemResults[i].Chunk.ChunkIndex < itemResults[j].Chunk.ChunkIndex
			})

			// display main result (highest hybrid score, show both similarity and hybrid score)
			resultText.WriteString(fmt.Sprintf("--- Result %d (similarity: %.2f%%, hybrid score: %.2f%%) ---\n",
				resultIndex, mainResult.Similarity*100, mainResult.Score*100))
			resultText.WriteString(fmt.Sprintf("Source: [%s] %s (ID: %s)\n", mainResult.Item.Category, mainResult.Item.Title, mainResult.Item.ID))

			// display all chunks in logical order (including main result and expanded chunks)
			if len(itemResults) == 1 {
				// only one chunk, display directly
				resultText.WriteString(fmt.Sprintf("Content snippet:\n%s\n", mainResult.Chunk.ChunkText))
			} else {
				// multiple chunks, display in logical order
				resultText.WriteString("Content snippets (in document order):\n")
				for i, result := range itemResults {
					// mark main result
					marker := ""
					if result.Chunk.ID == mainResult.Chunk.ID {
						marker = " [primary match]"
					}
					resultText.WriteString(fmt.Sprintf("  [Snippet %d%s]\n%s\n", i+1, marker, result.Chunk.ChunkText))
				}
			}
			resultText.WriteString("\n")

			if !contains(retrievedItemIDs, group.itemID) {
				retrievedItemIDs = append(retrievedItemIDs, group.itemID)
			}
			resultIndex++
		}

		// append metadata at the end of results (JSON format, used to extract knowledge item IDs)
		// use special markers to avoid interfering with AI reading of results
		if len(retrievedItemIDs) > 0 {
			metadataJSON, _ := json.Marshal(map[string]interface{}{
				"_metadata": map[string]interface{}{
					"retrievedItemIDs": retrievedItemIDs,
				},
			})
			resultText.WriteString(fmt.Sprintf("\n<!-- METADATA: %s -->", string(metadataJSON)))
		}

		// log retrieval (async, non-blocking)
		// note: conversationID and messageID are not available here; should be logged at the Agent layer
		// actual log recording should be done in the Agent's progressCallback

		return &mcp.ToolResult{
			Content: []mcp.Content{
				{
					Type: "text",
					Text: resultText.String(),
				},
			},
		}, nil
	}

	mcpServer.RegisterTool(searchTool, searchHandler)
	logger.Info("knowledge retrieval tool registered", zap.String("toolName", searchTool.Name))
}

// contains checks whether a slice contains an element
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// GetRetrievalMetadata extracts retrieval metadata from tool call arguments (used for logging)
func GetRetrievalMetadata(args map[string]interface{}) (query string, riskType string) {
	if q, ok := args["query"].(string); ok {
		query = q
	}
	if rt, ok := args["risk_type"].(string); ok {
		riskType = rt
	}
	return
}

// FormatRetrievalResults formats retrieval results as a string (used for logging)
func FormatRetrievalResults(results []*RetrievalResult) string {
	if len(results) == 0 {
		return "no relevant results found"
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("retrieved %d result(s):\n", len(results)))

	itemIDs := make(map[string]bool)
	for i, result := range results {
		builder.WriteString(fmt.Sprintf("%d. [%s] %s (similarity: %.2f%%)\n",
			i+1, result.Item.Category, result.Item.Title, result.Similarity*100))
		itemIDs[result.Item.ID] = true
	}

	// return knowledge item ID list (JSON format)
	ids := make([]string, 0, len(itemIDs))
	for id := range itemIDs {
		ids = append(ids, id)
	}
	idsJSON, _ := json.Marshal(ids)
	builder.WriteString(fmt.Sprintf("\nretrieved knowledge item IDs: %s", string(idsJSON)))

	return builder.String()
}
