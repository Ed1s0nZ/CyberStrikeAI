package handler

import (
	"net/http"
	"time"

	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/storage"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// OpenAPIHandler OpenAPI handler
type OpenAPIHandler struct {
	db               *database.DB
	logger           *zap.Logger
	resultStorage    storage.ResultStorage
	conversationHdlr *ConversationHandler
	agentHdlr        *AgentHandler
}

// NewOpenAPIHandler creates a new OpenAPI handler
func NewOpenAPIHandler(db *database.DB, logger *zap.Logger, resultStorage storage.ResultStorage, conversationHdlr *ConversationHandler, agentHdlr *AgentHandler) *OpenAPIHandler {
	return &OpenAPIHandler{
		db:               db,
		logger:           logger,
		resultStorage:    resultStorage,
		conversationHdlr: conversationHdlr,
		agentHdlr:        agentHdlr,
	}
}

// GetOpenAPISpec returns the OpenAPI specification
func (h *OpenAPIHandler) GetOpenAPISpec(c *gin.Context) {
	host := c.Request.Host
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}

	spec := map[string]interface{}{
		"openapi": "3.0.0",
		"info": map[string]interface{}{
			"title":       "CyberStrikeAI API",
			"description": "AI-powered automated security testing platform API documentation",
			"version":     "1.0.0",
			"contact": map[string]interface{}{
				"name": "CyberStrikeAI",
			},
		},
		"servers": []map[string]interface{}{
			{
				"url":         scheme + "://" + host,
				"description": "Current server",
			},
		},
		"components": map[string]interface{}{
			"securitySchemes": map[string]interface{}{
				"bearerAuth": map[string]interface{}{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "JWT",
					"description":  "Use Bearer Token for authentication. Token is obtained via the /api/auth/login endpoint.",
				},
			},
			"schemas": map[string]interface{}{
				"CreateConversationRequest": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"title": map[string]interface{}{
							"type":        "string",
							"description": "Conversation title",
							"example":     "Web application security testing",
						},
					},
				},
				"Conversation": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type":        "string",
							"description": "Conversation ID",
							"example":     "550e8400-e29b-41d4-a716-446655440000",
						},
						"title": map[string]interface{}{
							"type":        "string",
							"description": "Conversation title",
							"example":     "Web application security testing",
						},
						"createdAt": map[string]interface{}{
							"type":        "string",
							"format":      "date-time",
							"description": "Created at",
						},
						"updatedAt": map[string]interface{}{
							"type":        "string",
							"format":      "date-time",
							"description": "Updated at",
						},
					},
				},
				"ConversationDetail": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type":        "string",
							"description": "Conversation ID",
						},
						"title": map[string]interface{}{
							"type":        "string",
							"description": "Conversation title",
						},
						"status": map[string]interface{}{
							"type":        "string",
							"description": "Conversation status: active (in progress), completed (done), failed",
							"enum":        []string{"active", "completed", "failed"},
						},
						"createdAt": map[string]interface{}{
							"type":        "string",
							"format":      "date-time",
							"description": "Created at",
						},
						"updatedAt": map[string]interface{}{
							"type":        "string",
							"format":      "date-time",
							"description": "Updated at",
						},
						"messages": map[string]interface{}{
							"type":        "array",
							"description": "Message list",
							"items": map[string]interface{}{
								"$ref": "#/components/schemas/Message",
							},
						},
						"messageCount": map[string]interface{}{
							"type":        "integer",
							"description": "Message count",
						},
					},
				},
				"Message": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type":        "string",
							"description": "Message ID",
						},
						"conversationId": map[string]interface{}{
							"type":        "string",
							"description": "Conversation ID",
						},
						"role": map[string]interface{}{
							"type":        "string",
							"description": "Message role: user or assistant",
							"enum":        []string{"user", "assistant"},
						},
						"content": map[string]interface{}{
							"type":        "string",
							"description": "Message content",
						},
						"createdAt": map[string]interface{}{
							"type":        "string",
							"format":      "date-time",
							"description": "Created at",
						},
					},
				},
				"ConversationResults": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"conversationId": map[string]interface{}{
							"type":        "string",
							"description": "Conversation ID",
						},
						"messages": map[string]interface{}{
							"type":        "array",
							"description": "Message list",
							"items": map[string]interface{}{
								"$ref": "#/components/schemas/Message",
							},
						},
						"vulnerabilities": map[string]interface{}{
							"type":        "array",
							"description": "List of discovered vulnerabilities",
							"items": map[string]interface{}{
								"$ref": "#/components/schemas/Vulnerability",
							},
						},
						"executionResults": map[string]interface{}{
							"type":        "array",
							"description": "Execution result list",
							"items": map[string]interface{}{
								"$ref": "#/components/schemas/ExecutionResult",
							},
						},
					},
				},
				"Vulnerability": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type":        "string",
							"description": "Vulnerability ID",
						},
						"title": map[string]interface{}{
							"type":        "string",
							"description": "Vulnerability title",
						},
						"description": map[string]interface{}{
							"type":        "string",
							"description": "Vulnerability description",
						},
						"severity": map[string]interface{}{
							"type":        "string",
							"description": "Severity",
							"enum":        []string{"critical", "high", "medium", "low", "info"},
						},
						"status": map[string]interface{}{
							"type":        "string",
							"description": "Status",
							"enum":        []string{"open", "closed", "fixed"},
						},
						"target": map[string]interface{}{
							"type":        "string",
							"description": "Affected target",
						},
					},
				},
				"ExecutionResult": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type":        "string",
							"description": "Execution ID",
						},
						"toolName": map[string]interface{}{
							"type":        "string",
							"description": "Tool name",
						},
						"status": map[string]interface{}{
							"type":        "string",
							"description": "Execution status",
							"enum":        []string{"success", "failed", "running"},
						},
						"result": map[string]interface{}{
							"type":        "string",
							"description": "Execution result",
						},
						"createdAt": map[string]interface{}{
							"type":        "string",
							"format":      "date-time",
							"description": "Created at",
						},
					},
				},
				"Error": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"error": map[string]interface{}{
							"type":        "string",
							"description": "Error message",
						},
					},
				},
				"LoginRequest": map[string]interface{}{
					"type":     "object",
					"required": []string{"password"},
					"properties": map[string]interface{}{
						"password": map[string]interface{}{
							"type":        "string",
							"description": "Login password",
						},
					},
				},
				"LoginResponse": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"token": map[string]interface{}{
							"type":        "string",
							"description": "Authentication token",
						},
						"expires_at": map[string]interface{}{
							"type":        "string",
							"format":      "date-time",
							"description": "Token expiration time",
						},
						"session_duration_hr": map[string]interface{}{
							"type":        "integer",
							"description": "Session duration (hours)",
						},
					},
				},
				"ChangePasswordRequest": map[string]interface{}{
					"type":     "object",
					"required": []string{"oldPassword", "newPassword"},
					"properties": map[string]interface{}{
						"oldPassword": map[string]interface{}{
							"type":        "string",
							"description": "Current password",
						},
						"newPassword": map[string]interface{}{
							"type":        "string",
							"description": "New password (minimum 8 characters)",
						},
					},
				},
				"UpdateConversationRequest": map[string]interface{}{
					"type":     "object",
					"required": []string{"title"},
					"properties": map[string]interface{}{
						"title": map[string]interface{}{
							"type":        "string",
							"description": "Conversation title",
						},
					},
				},
				"Group": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type":        "string",
							"description": "Group ID",
						},
						"name": map[string]interface{}{
							"type":        "string",
							"description": "Group name",
						},
						"icon": map[string]interface{}{
							"type":        "string",
							"description": "Group icon",
						},
						"createdAt": map[string]interface{}{
							"type":        "string",
							"format":      "date-time",
							"description": "Created at",
						},
						"updatedAt": map[string]interface{}{
							"type":        "string",
							"format":      "date-time",
							"description": "Updated at",
						},
					},
				},
				"CreateGroupRequest": map[string]interface{}{
					"type":     "object",
					"required": []string{"name"},
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type":        "string",
							"description": "Group name",
						},
						"icon": map[string]interface{}{
							"type":        "string",
							"description": "Group icon (optional)",
						},
					},
				},
				"UpdateGroupRequest": map[string]interface{}{
					"type":     "object",
					"required": []string{"name"},
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type":        "string",
							"description": "Group name",
						},
						"icon": map[string]interface{}{
							"type":        "string",
							"description": "Group icon",
						},
					},
				},
				"AddConversationToGroupRequest": map[string]interface{}{
					"type":     "object",
					"required": []string{"conversationId", "groupId"},
					"properties": map[string]interface{}{
						"conversationId": map[string]interface{}{
							"type":        "string",
							"description": "Conversation ID",
						},
						"groupId": map[string]interface{}{
							"type":        "string",
							"description": "Group ID",
						},
					},
				},
				"BatchTaskRequest": map[string]interface{}{
					"type":     "object",
					"required": []string{"tasks"},
					"properties": map[string]interface{}{
						"title": map[string]interface{}{
							"type":        "string",
							"description": "Task title (optional)",
						},
						"tasks": map[string]interface{}{
							"type":        "array",
							"description": "Task list, one task per item",
							"items": map[string]interface{}{
								"type": "string",
							},
						},
						"role": map[string]interface{}{
							"type":        "string",
							"description": "Role name (optional)",
						},
					},
				},
				"BatchQueue": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type":        "string",
							"description": "Queue ID",
						},
						"title": map[string]interface{}{
							"type":        "string",
							"description": "Queue title",
						},
						"status": map[string]interface{}{
							"type":        "string",
							"description": "Queue status",
							"enum":        []string{"pending", "running", "paused", "completed", "failed"},
						},
						"tasks": map[string]interface{}{
							"type":        "array",
							"description": "Task list",
							"items": map[string]interface{}{
								"type": "object",
							},
						},
						"createdAt": map[string]interface{}{
							"type":        "string",
							"format":      "date-time",
							"description": "Created at",
						},
					},
				},
				"CancelAgentLoopRequest": map[string]interface{}{
					"type":     "object",
					"required": []string{"conversationId"},
					"properties": map[string]interface{}{
						"conversationId": map[string]interface{}{
							"type":        "string",
							"description": "Conversation ID",
						},
					},
				},
				"AgentTask": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"conversationId": map[string]interface{}{
							"type":        "string",
							"description": "Conversation ID",
						},
						"status": map[string]interface{}{
							"type":        "string",
							"description": "Task status",
							"enum":        []string{"running", "completed", "failed", "cancelled", "timeout"},
						},
						"startedAt": map[string]interface{}{
							"type":        "string",
							"format":      "date-time",
							"description": "Started at",
						},
					},
				},
				"CreateVulnerabilityRequest": map[string]interface{}{
					"type":     "object",
					"required": []string{"conversation_id", "title", "severity"},
					"properties": map[string]interface{}{
						"conversation_id": map[string]interface{}{
							"type":        "string",
							"description": "Conversation ID",
						},
						"title": map[string]interface{}{
							"type":        "string",
							"description": "Vulnerability title",
						},
						"description": map[string]interface{}{
							"type":        "string",
							"description": "Vulnerability description",
						},
						"severity": map[string]interface{}{
							"type":        "string",
							"description": "Severity",
							"enum":        []string{"critical", "high", "medium", "low", "info"},
						},
						"status": map[string]interface{}{
							"type":        "string",
							"description": "Status",
							"enum":        []string{"open", "closed", "fixed"},
						},
						"type": map[string]interface{}{
							"type":        "string",
							"description": "Vulnerability type",
						},
						"target": map[string]interface{}{
							"type":        "string",
							"description": "Affected target",
						},
						"proof": map[string]interface{}{
							"type":        "string",
							"description": "Vulnerability proof",
						},
						"impact": map[string]interface{}{
							"type":        "string",
							"description": "Impact",
						},
						"recommendation": map[string]interface{}{
							"type":        "string",
							"description": "Remediation recommendation",
						},
					},
				},
				"UpdateVulnerabilityRequest": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"title": map[string]interface{}{
							"type":        "string",
							"description": "Vulnerability title",
						},
						"description": map[string]interface{}{
							"type":        "string",
							"description": "Vulnerability description",
						},
						"severity": map[string]interface{}{
							"type":        "string",
							"description": "Severity",
							"enum":        []string{"critical", "high", "medium", "low", "info"},
						},
						"status": map[string]interface{}{
							"type":        "string",
							"description": "Status",
							"enum":        []string{"open", "closed", "fixed"},
						},
						"type": map[string]interface{}{
							"type":        "string",
							"description": "Vulnerability type",
						},
						"target": map[string]interface{}{
							"type":        "string",
							"description": "Affected target",
						},
						"proof": map[string]interface{}{
							"type":        "string",
							"description": "Vulnerability proof",
						},
						"impact": map[string]interface{}{
							"type":        "string",
							"description": "Impact",
						},
						"recommendation": map[string]interface{}{
							"type":        "string",
							"description": "Remediation recommendation",
						},
					},
				},
				"ListVulnerabilitiesResponse": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"vulnerabilities": map[string]interface{}{
							"type":        "array",
							"description": "Vulnerability list",
							"items": map[string]interface{}{
								"$ref": "#/components/schemas/Vulnerability",
							},
						},
						"total": map[string]interface{}{
							"type":        "integer",
							"description": "Total",
						},
						"page": map[string]interface{}{
							"type":        "integer",
							"description": "Current page",
						},
						"page_size": map[string]interface{}{
							"type":        "integer",
							"description": "Page size",
						},
						"total_pages": map[string]interface{}{
							"type":        "integer",
							"description": "Total pages",
						},
					},
				},
				"VulnerabilityStats": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"total": map[string]interface{}{
							"type":        "integer",
							"description": "Total vulnerabilities",
						},
						"by_severity": map[string]interface{}{
							"type":        "object",
							"description": "Statistics by severity",
						},
						"by_status": map[string]interface{}{
							"type":        "object",
							"description": "Statistics by status",
						},
					},
				},
				"RoleConfig": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type":        "string",
							"description": "Role name",
						},
						"description": map[string]interface{}{
							"type":        "string",
							"description": "Role description",
						},
						"enabled": map[string]interface{}{
							"type":        "boolean",
							"description": "Whether enabled",
						},
						"systemPrompt": map[string]interface{}{
							"type":        "string",
							"description": "System prompt",
						},
						"userPrompt": map[string]interface{}{
							"type":        "string",
							"description": "User prompt",
						},
						"tools": map[string]interface{}{
							"type":        "array",
							"description": "Tool list",
							"items": map[string]interface{}{
								"type": "string",
							},
						},
						"skills": map[string]interface{}{
							"type":        "array",
							"description": "Skills list",
							"items": map[string]interface{}{
								"type": "string",
							},
						},
					},
				},
				"Skill": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type":        "string",
							"description": "Skill name",
						},
						"description": map[string]interface{}{
							"type":        "string",
							"description": "Skill description",
						},
						"path": map[string]interface{}{
							"type":        "string",
							"description": "Skill path",
						},
					},
				},
				"CreateSkillRequest": map[string]interface{}{
					"type":     "object",
					"required": []string{"name", "description"},
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type":        "string",
							"description": "Skill name",
						},
						"description": map[string]interface{}{
							"type":        "string",
							"description": "Skill description",
						},
					},
				},
				"UpdateSkillRequest": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"description": map[string]interface{}{
							"type":        "string",
							"description": "Skill description",
						},
					},
				},
				"ToolExecution": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type":        "string",
							"description": "Execution ID",
						},
						"toolName": map[string]interface{}{
							"type":        "string",
							"description": "Tool name",
						},
						"status": map[string]interface{}{
							"type":        "string",
							"description": "Execution status",
							"enum":        []string{"success", "failed", "running"},
						},
						"createdAt": map[string]interface{}{
							"type":        "string",
							"format":      "date-time",
							"description": "Created at",
						},
					},
				},
				"MonitorResponse": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"executions": map[string]interface{}{
							"type":        "array",
							"description": "Execution record list",
							"items": map[string]interface{}{
								"$ref": "#/components/schemas/ToolExecution",
							},
						},
						"stats": map[string]interface{}{
							"type":        "object",
							"description": "Statistics",
						},
						"timestamp": map[string]interface{}{
							"type":        "string",
							"format":      "date-time",
							"description": "Timestamp",
						},
						"total": map[string]interface{}{
							"type":        "integer",
							"description": "Total",
						},
						"page": map[string]interface{}{
							"type":        "integer",
							"description": "Current page",
						},
						"page_size": map[string]interface{}{
							"type":        "integer",
							"description": "Page size",
						},
						"total_pages": map[string]interface{}{
							"type":        "integer",
							"description": "Total pages",
						},
					},
				},
				"ConfigResponse": map[string]interface{}{
					"type":        "object",
					"description": "Configuration information",
				},
				"UpdateConfigRequest": map[string]interface{}{
					"type":        "object",
					"description": "Update configuration request",
				},
				"ExternalMCPConfig": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"enabled": map[string]interface{}{
							"type":        "boolean",
							"description": "Whether enabled",
						},
						"command": map[string]interface{}{
							"type":        "string",
							"description": "Command",
						},
						"args": map[string]interface{}{
							"type":        "array",
							"description": "Argument list",
							"items": map[string]interface{}{
								"type": "string",
							},
						},
					},
				},
				"ExternalMCPResponse": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"config": map[string]interface{}{
							"$ref": "#/components/schemas/ExternalMCPConfig",
						},
						"status": map[string]interface{}{
							"type":        "string",
							"description": "Status",
							"enum":        []string{"connected", "disconnected", "error", "disabled"},
						},
						"toolCount": map[string]interface{}{
							"type":        "integer",
							"description": "Tool count",
						},
						"error": map[string]interface{}{
							"type":        "string",
							"description": "Error message",
						},
					},
				},
				"AddOrUpdateExternalMCPRequest": map[string]interface{}{
					"type":     "object",
					"required": []string{"config"},
					"properties": map[string]interface{}{
						"config": map[string]interface{}{
							"$ref": "#/components/schemas/ExternalMCPConfig",
						},
					},
				},
				"AttackChain": map[string]interface{}{
					"type":        "object",
					"description": "Attack chain data",
				},
				"MCPMessage": map[string]interface{}{
					"type":        "object",
					"description": "MCP message (conforming to JSON-RPC 2.0 specification)",
					"required":    []string{"jsonrpc"},
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"description": "Message ID, can be a string, number, or null. Required for requests; may be omitted for notifications",
							"oneOf": []map[string]interface{}{
								{"type": "string"},
								{"type": "number"},
								{"type": "null"},
							},
							"example": "550e8400-e29b-41d4-a716-446655440000",
						},
						"method": map[string]interface{}{
							"type":        "string",
							"description": "Method name. Supported methods:\n- `initialize`: Initialize MCP connection\n- `tools/list`: List all available tools\n- `tools/call`: Call a tool\n- `prompts/list`: List all prompt templates\n- `prompts/get`: Get a prompt template\n- `resources/list`: List all resources\n- `resources/read`: Read resource content\n- `sampling/request`: Sampling request",
							"enum": []string{
								"initialize",
								"tools/list",
								"tools/call",
								"prompts/list",
								"prompts/get",
								"resources/list",
								"resources/read",
								"sampling/request",
							},
							"example": "tools/list",
						},
						"params": map[string]interface{}{
							"description": "Method parameters (JSON object), structure varies by method",
							"type":        "object",
						},
						"jsonrpc": map[string]interface{}{
							"type":        "string",
							"description": "JSON-RPC version, fixed to \"2.0\"",
							"enum":        []string{"2.0"},
							"example":     "2.0",
						},
					},
				},
				"MCPInitializeParams": map[string]interface{}{
					"type":     "object",
					"required": []string{"protocolVersion", "capabilities", "clientInfo"},
					"properties": map[string]interface{}{
						"protocolVersion": map[string]interface{}{
							"type":        "string",
							"description": "Protocol version",
							"example":     "2024-11-05",
						},
						"capabilities": map[string]interface{}{
							"type":        "object",
							"description": "Client capabilities",
						},
						"clientInfo": map[string]interface{}{
							"type":     "object",
							"required": []string{"name", "version"},
							"properties": map[string]interface{}{
								"name": map[string]interface{}{
									"type":        "string",
									"description": "Client name",
									"example":     "MyClient",
								},
								"version": map[string]interface{}{
									"type":        "string",
									"description": "Client version",
									"example":     "1.0.0",
								},
							},
						},
					},
				},
				"MCPCallToolParams": map[string]interface{}{
					"type":     "object",
					"required": []string{"name", "arguments"},
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type":        "string",
							"description": "Tool name",
							"example":     "nmap",
						},
						"arguments": map[string]interface{}{
							"type":        "object",
							"description": "Tool arguments (key-value pairs), specific parameters depend on tool definition",
							"example": map[string]interface{}{
								"target": "192.168.1.1",
								"ports":  "80,443",
							},
						},
					},
				},
				"MCPResponse": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"description": "Message ID (same as the id in the request)",
							"oneOf": []map[string]interface{}{
								{"type": "string"},
								{"type": "number"},
								{"type": "null"},
							},
						},
						"result": map[string]interface{}{
							"description": "Method execution result (JSON object), structure depends on the method called",
							"type":        "object",
						},
						"error": map[string]interface{}{
							"type":        "object",
							"description": "Error information (if execution failed)",
							"properties": map[string]interface{}{
								"code": map[string]interface{}{
									"type":        "integer",
									"description": "Error code",
									"example":     -32600,
								},
								"message": map[string]interface{}{
									"type":        "string",
									"description": "Error message",
									"example":     "Invalid Request",
								},
								"data": map[string]interface{}{
									"description": "Error details (optional)",
								},
							},
						},
						"jsonrpc": map[string]interface{}{
							"type":        "string",
							"description": "JSON-RPC version",
							"example":     "2.0",
						},
					},
				},
			},
		},
		"security": []map[string]interface{}{
			{
				"bearerAuth": []string{},
			},
		},
		"paths": map[string]interface{}{
			"/api/auth/login": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"Authentication"},
					"summary":     "User login",
					"description": "Login with password to obtain authentication token",
					"operationId": "login",
					"security":    []map[string]interface{}{},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/LoginRequest",
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Login successful",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/LoginResponse",
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "Incorrect password",
						},
					},
				},
			},
			"/api/auth/logout": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"Authentication"},
					"summary":     "User logout",
					"description": "Logout current session and invalidate token",
					"operationId": "logout",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Logout successful",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"message": map[string]interface{}{
												"type":    "string",
												"example": "Logged out successfully",
											},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/auth/change-password": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"Authentication"},
					"summary":     "Change password",
					"description": "Change login password; all sessions will be invalidated after change",
					"operationId": "changePassword",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/ChangePasswordRequest",
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Password changed successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"message": map[string]interface{}{
												"type":    "string",
												"example": "Password updated, please log in again with the new password",
											},
										},
									},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "Invalid request parameters",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/auth/validate": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Authentication"},
					"summary":     "Validate token",
					"description": "Verify whether the current token is valid",
					"operationId": "validateToken",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Token is valid",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"token": map[string]interface{}{
												"type":        "string",
												"description": "Token",
											},
											"expires_at": map[string]interface{}{
												"type":        "string",
												"format":      "date-time",
												"description": "Expiration time",
											},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "Token is invalid or expired",
						},
					},
				},
			},
			"/api/conversations": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"Conversation Management"},
					"summary":     "Create conversation",
					"description": "Create a new security testing conversation.\n**Important notes**:\n- ✅ The created conversation is **immediately saved to the database**\n- ✅ The frontend page **automatically refreshes** to show the new conversation\n- ✅ **Fully consistent** with conversations created from the frontend\n**Two ways to create a conversation**:\n**Method 1 (recommended):** Send a message directly via `/api/agent-loop` **without** providing the `conversationId` parameter; the system will automatically create a new conversation and send the message. This is the simplest approach - creation and sending in one step.\n**Method 2:** Call this endpoint first to create an empty conversation, then use the returned `conversationId` to call `/api/agent-loop` and send a message. Use this when you need to create the conversation first and send the message later.\n**Example**:\n```json\n{\n  \"title\": \"Web Application Security Testing\"\n}\n```",
					"operationId": "createConversation",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/CreateConversationRequest",
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Conversation created successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/Conversation",
									},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "Invalid request parameters",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized, a valid token is required",
						},
						"500": map[string]interface{}{
							"description": "Internal server error",
						},
					},
				},
				"get": map[string]interface{}{
					"tags":        []string{"Conversation Management"},
					"summary":     "List conversations",
					"description": "Get conversation list with pagination and search support",
					"operationId": "listConversations",
					"parameters": []map[string]interface{}{
						{
							"name":        "limit",
							"in":          "query",
							"required":    false,
							"description": "Result count limit",
							"schema": map[string]interface{}{
								"type":    "integer",
								"default": 50,
								"minimum": 1,
								"maximum": 100,
							},
						},
						{
							"name":        "offset",
							"in":          "query",
							"required":    false,
							"description": "Offset",
							"schema": map[string]interface{}{
								"type":    "integer",
								"default": 0,
								"minimum": 0,
							},
						},
						{
							"name":        "search",
							"in":          "query",
							"required":    false,
							"description": "Search keyword",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Retrieved successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "array",
										"items": map[string]interface{}{
											"$ref": "#/components/schemas/Conversation",
										},
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "Unauthorized, a valid token is required",
						},
					},
				},
			},
			"/api/conversations/{id}": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Conversation Management"},
					"summary":     "Get conversation details",
					"description": "Get detailed information about the specified conversation, including conversation info and message list",
					"operationId": "getConversation",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "Conversation ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Retrieved successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/ConversationDetail",
									},
								},
							},
						},
						"404": map[string]interface{}{
							"description": "Conversation not found",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized, a valid token is required",
						},
					},
				},
				"put": map[string]interface{}{
					"tags":        []string{"Conversation Management"},
					"summary":     "Update conversation",
					"description": "Update conversation title",
					"operationId": "updateConversation",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "Conversation ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/UpdateConversationRequest",
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Updated successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/Conversation",
									},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "Invalid request parameters",
						},
						"404": map[string]interface{}{
							"description": "Conversation not found",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized, a valid token is required",
						},
					},
				},
				"delete": map[string]interface{}{
					"tags":        []string{"Conversation Management"},
					"summary":     "Delete conversation",
					"description": "Delete the specified conversation and all related data (messages, vulnerabilities, etc.). **This operation is irreversible**.",
					"operationId": "deleteConversation",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "Conversation ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Deleted successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"message": map[string]interface{}{
												"type":        "string",
												"description": "Success message",
												"example":     "Deleted successfully",
											},
										},
									},
								},
							},
						},
						"404": map[string]interface{}{
							"description": "Conversation not found",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized, a valid token is required",
						},
						"500": map[string]interface{}{
							"description": "Internal server error",
						},
					},
				},
			},
			"/api/conversations/{id}/results": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Conversation Management"},
					"summary":     "Get conversation results",
					"description": "Get execution results for the specified conversation, including messages, vulnerability info, and execution results",
					"operationId": "getConversationResults",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "Conversation ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Retrieved successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/ConversationResults",
									},
								},
							},
						},
						"404": map[string]interface{}{
							"description": "Conversation not found or result not found",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized, a valid token is required",
						},
					},
				},
			},
			"/api/agent-loop": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"Conversation Interaction"},
					"summary":     "Send message and get AI reply (non-streaming)",
					"description": "Send a message to the AI and receive a reply (non-streaming response). **This is the core endpoint for interacting with the AI**, fully consistent with the frontend chat functionality.\n**Important notes**:\n- ✅ Messages created/sent via this API are **immediately saved to the database**\n- ✅ The frontend page **automatically refreshes** to show the newly created conversations and messages\n- ✅ All operations leave a **complete interaction trail**, just like operating from the frontend\n- ✅ Supports role configuration; you can specify which testing role to use\n**Recommended workflow**:\n1. **Create conversation first**: Call `POST /api/conversations` to create a new conversation and obtain the `conversationId`\n2. **Then send message**: Use the returned `conversationId` to call this endpoint and send the message\n**Usage example**:\n**Step 1 - Create conversation:**\n```json\nPOST /api/conversations\n{\n  \"title\": \"Web Application Security Testing\"\n}\n```\n**Step 2 - Send message:**\n```json\nPOST /api/agent-loop\n{\n  \"conversationId\": \"returned conversation ID\",\n  \"message\": \"Scan http://example.com for SQL injection vulnerabilities\",\n  \"role\": \"Penetration Testing\"\n}\n```\n**Alternative**: If `conversationId` is not provided, the system will automatically create a new conversation and send the message. However, **creating the conversation first is recommended** for better conversation list management.\n**Response**: Returns the AI reply, conversation ID, and MCP execution ID list. The frontend will automatically refresh to display the new message.",
					"operationId": "sendMessage",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"message": map[string]interface{}{
											"type":        "string",
											"description": "Message to send (required)",
											"example":     "Scan http://example.com for SQL injection vulnerabilities",
										},
										"conversationId": map[string]interface{}{
											"type":        "string",
											"description": "Conversation ID (optional).\n- **Not provided**: automatically creates a new conversation and sends the message (recommended)\n- **Provided**: message is added to the specified conversation (conversation must exist)",
											"example":     "550e8400-e29b-41d4-a716-446655440000",
										},
										"role": map[string]interface{}{
											"type":        "string",
											"description": "Role name (optional), e.g.: Default, Penetration Testing, Web Application Scanning, etc.",
											"example":     "Default",
										},
									},
									"required": []string{"message"},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Message sent successfully, returns AI reply",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"response": map[string]interface{}{
												"type":        "string",
												"description": "AI reply content",
											},
											"conversationId": map[string]interface{}{
												"type":        "string",
												"description": "Conversation ID",
											},
											"mcpExecutionIds": map[string]interface{}{
												"type":        "array",
												"description": "MCP execution ID list",
												"items": map[string]interface{}{
													"type": "string",
												},
											},
											"time": map[string]interface{}{
												"type":        "string",
												"format":      "date-time",
												"description": "Response time",
											},
										},
									},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "Invalid request parameters",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized, a valid token is required",
						},
						"500": map[string]interface{}{
							"description": "Internal server error",
						},
					},
				},
			},
			"/api/agent-loop/stream": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"Conversation Interaction"},
					"summary":     "Send message and get AI reply (streaming)",
					"description": "Send a message to the AI and receive a streaming reply (Server-Sent Events). **This is the core endpoint for interacting with the AI**, fully consistent with the frontend chat functionality.\n**Important notes**:\n- ✅ Messages created/sent via this API are **immediately saved to the database**\n- ✅ The frontend page **automatically refreshes** to show the newly created conversations and messages\n- ✅ All operations leave a **complete interaction trail**, just like operating from the frontend\n- ✅ Supports role configuration; you can specify which testing role to use\n- ✅ Returns a streaming response, suitable for displaying AI replies in real time\n**Recommended workflow**:\n1. **Create conversation first**: Call `POST /api/conversations` to create a new conversation and obtain the `conversationId`\n2. **Then send message**: Use the returned `conversationId` to call this endpoint and send the message\n**Usage example**:\n**Step 1 - Create conversation:**\n```json\nPOST /api/conversations\n{\n  \"title\": \"Web Application Security Testing\"\n}\n```\n**Step 2 - Send message (streaming):**\n```json\nPOST /api/agent-loop/stream\n{\n  \"conversationId\": \"returned conversation ID\",\n  \"message\": \"Scan http://example.com for SQL injection vulnerabilities\",\n  \"role\": \"Penetration Testing\"\n}\n```\n**Response format**: Server-Sent Events (SSE), event types include:\n- `message`: User message acknowledgment\n- `response`: AI reply fragment\n- `progress`: Progress update\n- `done`: Completed\n- `error`: Error\n- `cancelled`: Cancelled",
					"operationId": "sendMessageStream",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"message": map[string]interface{}{
											"type":        "string",
											"description": "Message to send (required)",
											"example":     "Scan http://example.com for SQL injection vulnerabilities",
										},
										"conversationId": map[string]interface{}{
											"type":        "string",
											"description": "Conversation ID (optional).\n- **Not provided**: automatically creates a new conversation and sends the message (recommended)\n- **Provided**: message is added to the specified conversation (conversation must exist)",
											"example":     "550e8400-e29b-41d4-a716-446655440000",
										},
										"role": map[string]interface{}{
											"type":        "string",
											"description": "Role name (optional), e.g.: Default, Penetration Testing, Web Application Scanning, etc.",
											"example":     "Default",
										},
									},
									"required": []string{"message"},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Streaming response (Server-Sent Events)",
							"content": map[string]interface{}{
								"text/event-stream": map[string]interface{}{
									"schema": map[string]interface{}{
										"type":        "string",
										"description": "SSE streaming data",
									},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "Invalid request parameters",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized, a valid token is required",
						},
						"500": map[string]interface{}{
							"description": "Internal server error",
						},
					},
				},
			},
			"/api/multi-agent": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"conversation"},
					"summary":     "Send message via multi-agent orchestrator",
					"description": "Route the message through the native multi-agent orchestrator (`internal/multiagent/orchestrator.go`) instead of the single-agent ReAct loop. The orchestrator plans, delegates sub-tasks to Markdown-defined sub-agents under `agents/*.md` via the `task` tool, and synthesises their results. Accepts the same request body shape as `POST /api/agent-loop`. **Requirements:** `multi_agent.enabled: true` in `config.yaml`; the endpoint returns HTTP 404 with a JSON error body when multi-agent mode is disabled. If `webshellConnectionId` is supplied the orchestrator is scoped to that WebShell connection and the sub-agents inherit the connection-scoped tool list, matching `/api/agent-loop` semantics.",
					"operationId": "sendMessageMultiAgent",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"message": map[string]interface{}{
											"type":        "string",
											"description": "The message text to send to the agent.",
										},
										"conversationId": map[string]interface{}{
											"type":        "string",
											"description": "Conversation ID (optional). When omitted, a new conversation is created; when provided, the message is added to the specified conversation (which must exist).",
										},
										"role": map[string]interface{}{
											"type":        "string",
											"description": "Role name (optional). When provided, must match an entry defined under `roles/` — the orchestrator applies the role's system prompt and tool whitelist.",
										},
										"webshellConnectionId": map[string]interface{}{
											"type":        "string",
											"description": "WebShell connection ID (optional). When set, the orchestrator is scoped to the specified WebShell connection — same scoping semantics as `/api/agent-loop`.",
										},
									},
									"required": []string{"message"},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Success. Response body follows the same shape as `/api/agent-loop`.",
						},
						"400": map[string]interface{}{"description": "Invalid request parameters"},
						"401": map[string]interface{}{"description": "Unauthorized"},
						"404": map[string]interface{}{"description": "Multi-agent mode is not enabled, or the referenced conversation does not exist"},
						"500": map[string]interface{}{"description": "Execution failed"},
					},
				},
			},
			"/api/multi-agent/stream": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"conversation"},
					"summary":     "Send message via multi-agent orchestrator (SSE stream)",
					"description": "Streaming variant of `POST /api/agent-loop/stream` routed through the native multi-agent orchestrator. Emits the same set of SSE event types as the single-agent stream (progress, tool calls, sub-agent delegation events, thinking, response, done, error). **Requirements:** `multi_agent.enabled: true`; the endpoint returns HTTP 200 with an SSE body in all cases — transport-level errors are delivered as `type: error` events, followed by `type: done`, rather than as HTTP error codes once the stream has opened. Supports `webshellConnectionId` with the same scoping semantics as the non-stream endpoint.",
					"operationId": "sendMessageMultiAgentStream",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"message":              map[string]interface{}{"type": "string"},
										"conversationId":       map[string]interface{}{"type": "string"},
										"role":                 map[string]interface{}{"type": "string"},
										"webshellConnectionId": map[string]interface{}{"type": "string"},
									},
									"required": []string{"message"},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Server-Sent Events stream. Content-Type is `text/event-stream`.",
							"content": map[string]interface{}{
								"text/event-stream": map[string]interface{}{
									"schema": map[string]interface{}{
										"type":        "string",
										"description": "SSE event payload. Each event has a `type` field — see the Agent Loop stream protocol documentation for the full event-type catalogue.",
									},
								},
							},
						},
						"401": map[string]interface{}{"description": "Unauthorized"},
					},
				},
			},
			"/api/agent-loop/cancel": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"Conversation Interaction"},
					"summary":     "Cancel task",
					"description": "Cancel the currently running Agent Loop task",
					"operationId": "cancelAgentLoop",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/CancelAgentLoopRequest",
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Cancellation request submitted",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"status": map[string]interface{}{
												"type":    "string",
												"example": "cancelling",
											},
											"conversationId": map[string]interface{}{
												"type":        "string",
												"description": "Conversation ID",
											},
											"message": map[string]interface{}{
												"type":    "string",
												"example": "Cancellation request submitted; task will stop after the current step completes.",
											},
										},
									},
								},
							},
						},
						"404": map[string]interface{}{
							"description": "No running task found",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/agent-loop/tasks": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Conversation Interaction"},
					"summary":     "List running tasks",
					"description": "Get all currently running Agent Loop tasks",
					"operationId": "listAgentTasks",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Retrieved successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"tasks": map[string]interface{}{
												"type":        "array",
												"description": "Task list",
												"items": map[string]interface{}{
													"$ref": "#/components/schemas/AgentTask",
												},
											},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/agent-loop/tasks/completed": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Conversation Interaction"},
					"summary":     "List completed tasks",
					"description": "Get recently completed Agent Loop task history",
					"operationId": "listCompletedTasks",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Retrieved successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"tasks": map[string]interface{}{
												"type":        "array",
												"description": "List of completed tasks",
												"items": map[string]interface{}{
													"$ref": "#/components/schemas/AgentTask",
												},
											},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/batch-tasks": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"Batch Tasks"},
					"summary":     "Create batch task queue",
					"description": "Create a batch task queue containing multiple tasks",
					"operationId": "createBatchQueue",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/BatchTaskRequest",
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Created successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"queueId": map[string]interface{}{
												"type":        "string",
												"description": "Queue ID",
											},
											"queue": map[string]interface{}{
												"$ref": "#/components/schemas/BatchQueue",
											},
										},
									},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "Invalid request parameters",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
				"get": map[string]interface{}{
					"tags":        []string{"Batch Tasks"},
					"summary":     "List batch task queues",
					"description": "Get all batch task queues",
					"operationId": "listBatchQueues",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Retrieved successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"queues": map[string]interface{}{
												"type":        "array",
												"description": "Queue list",
												"items": map[string]interface{}{
													"$ref": "#/components/schemas/BatchQueue",
												},
											},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/batch-tasks/{queueId}": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Batch Tasks"},
					"summary":     "Get batch task queue",
					"description": "Get detailed information about a specific batch task queue",
					"operationId": "getBatchQueue",
					"parameters": []map[string]interface{}{
						{
							"name":        "queueId",
							"in":          "path",
							"required":    true,
							"description": "Queue ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Retrieved successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/BatchQueue",
									},
								},
							},
						},
						"404": map[string]interface{}{
							"description": "Queue not found",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
				"delete": map[string]interface{}{
					"tags":        []string{"Batch Tasks"},
					"summary":     "Delete batch task queue",
					"description": "Delete the specified batch task queue",
					"operationId": "deleteBatchQueue",
					"parameters": []map[string]interface{}{
						{
							"name":        "queueId",
							"in":          "path",
							"required":    true,
							"description": "Queue ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Deleted successfully",
						},
						"404": map[string]interface{}{
							"description": "Queue not found",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/batch-tasks/{queueId}/start": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"Batch Tasks"},
					"summary":     "Start batch task queue",
					"description": "Begin executing tasks in the batch task queue",
					"operationId": "startBatchQueue",
					"parameters": []map[string]interface{}{
						{
							"name":        "queueId",
							"in":          "path",
							"required":    true,
							"description": "Queue ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Started successfully",
						},
						"404": map[string]interface{}{
							"description": "Queue not found",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/batch-tasks/{queueId}/pause": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"Batch Tasks"},
					"summary":     "Pause batch task queue",
					"description": "Pause the currently executing batch task queue",
					"operationId": "pauseBatchQueue",
					"parameters": []map[string]interface{}{
						{
							"name":        "queueId",
							"in":          "path",
							"required":    true,
							"description": "Queue ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Paused successfully",
						},
						"404": map[string]interface{}{
							"description": "Queue not found",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/batch-tasks/{queueId}/tasks": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"Batch Tasks"},
					"summary":     "Add task to queue",
					"description": "Add a new task to the batch task queue. Tasks are appended to the end of the queue and executed in order. Each task creates an independent conversation with full status tracking.\n**Task format**:\nTask content is a string describing the security testing task to execute. Examples:\n- \"Scan http://example.com for SQL injection vulnerabilities\"\n- \"Perform a port scan on 192.168.1.1\"\n- \"Detect XSS vulnerabilities on https://target.com\"\n**Usage example**:\n```json\n{\n  \"task\": \"Scan http://example.com for SQL injection vulnerabilities\"\n}\n```",
					"operationId": "addBatchTask",
					"parameters": []map[string]interface{}{
						{
							"name":        "queueId",
							"in":          "path",
							"required":    true,
							"description": "Queue ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type":     "object",
									"required": []string{"task"},
									"properties": map[string]interface{}{
										"task": map[string]interface{}{
											"type":        "string",
											"description": "Task content, describing the security testing task to execute (required)",
											"example":     "Scan http://example.com for SQL injection vulnerabilities",
										},
									},
								},
								"examples": map[string]interface{}{
									"sqlInjection": map[string]interface{}{
										"summary":     "SQL injection scan",
										"description": "Scan target website for SQL injection vulnerabilities",
										"value": map[string]interface{}{
											"task": "Scan http://example.com for SQL injection vulnerabilities",
										},
									},
									"portScan": map[string]interface{}{
										"summary":     "Port scan",
										"description": "Perform a port scan on the target IP",
										"value": map[string]interface{}{
											"task": "Perform a port scan on 192.168.1.1",
										},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Added successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"taskId": map[string]interface{}{
												"type":        "string",
												"description": "ID of the newly added task",
											},
											"message": map[string]interface{}{
												"type":        "string",
												"description": "Success message",
												"example":     "Task added to queue",
											},
										},
									},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "Invalid request parameters (e.g. task is empty)",
						},
						"404": map[string]interface{}{
							"description": "Queue not found",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/batch-tasks/{queueId}/tasks/{taskId}": map[string]interface{}{
				"put": map[string]interface{}{
					"tags":        []string{"Batch Tasks"},
					"summary":     "Update batch task",
					"description": "Update the specified task in the batch task queue",
					"operationId": "updateBatchTask",
					"parameters": []map[string]interface{}{
						{
							"name":        "queueId",
							"in":          "path",
							"required":    true,
							"description": "Queue ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
						{
							"name":        "taskId",
							"in":          "path",
							"required":    true,
							"description": "Task ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"task": map[string]interface{}{
											"type":        "string",
											"description": "Task content",
										},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Updated successfully",
						},
						"404": map[string]interface{}{
							"description": "Task not found",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
				"delete": map[string]interface{}{
					"tags":        []string{"Batch Tasks"},
					"summary":     "Delete batch task",
					"description": "Delete the specified task from the batch task queue",
					"operationId": "deleteBatchTask",
					"parameters": []map[string]interface{}{
						{
							"name":        "queueId",
							"in":          "path",
							"required":    true,
							"description": "Queue ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
						{
							"name":        "taskId",
							"in":          "path",
							"required":    true,
							"description": "Task ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Deleted successfully",
						},
						"404": map[string]interface{}{
							"description": "Task not found",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/groups": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"Conversation Groups"},
					"summary":     "Create group",
					"description": "Create a new conversation group",
					"operationId": "createGroup",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/CreateGroupRequest",
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Created successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/Group",
									},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "Invalid request parameters or group name already exists",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
				"get": map[string]interface{}{
					"tags":        []string{"Conversation Groups"},
					"summary":     "List groups",
					"description": "Get all conversation groups",
					"operationId": "listGroups",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Retrieved successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "array",
										"items": map[string]interface{}{
											"$ref": "#/components/schemas/Group",
										},
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/groups/{id}": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Conversation Groups"},
					"summary":     "Get group",
					"description": "Get detailed information about the specified group",
					"operationId": "getGroup",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "Group ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Retrieved successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/Group",
									},
								},
							},
						},
						"404": map[string]interface{}{
							"description": "Group not found",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
				"put": map[string]interface{}{
					"tags":        []string{"Conversation Groups"},
					"summary":     "Update group",
					"description": "Update group information",
					"operationId": "updateGroup",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "Group ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/UpdateGroupRequest",
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Updated successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/Group",
									},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "Invalid request parameters or group name already exists",
						},
						"404": map[string]interface{}{
							"description": "Group not found",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
				"delete": map[string]interface{}{
					"tags":        []string{"Conversation Groups"},
					"summary":     "Delete group",
					"description": "Delete the specified group",
					"operationId": "deleteGroup",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "Group ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Deleted successfully",
						},
						"404": map[string]interface{}{
							"description": "Group not found",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/groups/{id}/conversations": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Conversation Groups"},
					"summary":     "Get conversations in group",
					"description": "Get all conversations in the specified group",
					"operationId": "getGroupConversations",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "Group ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Retrieved successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "array",
										"items": map[string]interface{}{
											"$ref": "#/components/schemas/Conversation",
										},
									},
								},
							},
						},
						"404": map[string]interface{}{
							"description": "Group not found",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/groups/conversations": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"Conversation Groups"},
					"summary":     "Add conversation to group",
					"description": "Add a conversation to the specified group",
					"operationId": "addConversationToGroup",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/AddConversationToGroupRequest",
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Added successfully",
						},
						"400": map[string]interface{}{
							"description": "Invalid request parameters",
						},
						"404": map[string]interface{}{
							"description": "Conversation or group not found",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/groups/{id}/conversations/{conversationId}": map[string]interface{}{
				"delete": map[string]interface{}{
					"tags":        []string{"Conversation Groups"},
					"summary":     "Remove conversation from group",
					"description": "Remove a conversation from the specified group",
					"operationId": "removeConversationFromGroup",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "Group ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
						{
							"name":        "conversationId",
							"in":          "path",
							"required":    true,
							"description": "Conversation ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Removed successfully",
						},
						"404": map[string]interface{}{
							"description": "Conversation or group not found",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/vulnerabilities": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Vulnerability Management"},
					"summary":     "List vulnerabilities",
					"description": "Get vulnerability list with pagination and filtering support",
					"operationId": "listVulnerabilities",
					"parameters": []map[string]interface{}{
						{
							"name":        "limit",
							"in":          "query",
							"required":    false,
							"description": "Page size",
							"schema": map[string]interface{}{
								"type":    "integer",
								"default": 20,
								"minimum": 1,
								"maximum": 100,
							},
						},
						{
							"name":        "offset",
							"in":          "query",
							"required":    false,
							"description": "Offset",
							"schema": map[string]interface{}{
								"type":    "integer",
								"default": 0,
								"minimum": 0,
							},
						},
						{
							"name":        "page",
							"in":          "query",
							"required":    false,
							"description": "Page number (alternative to offset)",
							"schema": map[string]interface{}{
								"type":    "integer",
								"minimum": 1,
							},
						},
						{
							"name":        "id",
							"in":          "query",
							"required":    false,
							"description": "Vulnerability ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
						{
							"name":        "conversation_id",
							"in":          "query",
							"required":    false,
							"description": "Conversation ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
						{
							"name":        "severity",
							"in":          "query",
							"required":    false,
							"description": "Severity",
							"schema": map[string]interface{}{
								"type": "string",
								"enum": []string{"critical", "high", "medium", "low", "info"},
							},
						},
						{
							"name":        "status",
							"in":          "query",
							"required":    false,
							"description": "Status",
							"schema": map[string]interface{}{
								"type": "string",
								"enum": []string{"open", "closed", "fixed"},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Retrieved successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/ListVulnerabilitiesResponse",
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
				"post": map[string]interface{}{
					"tags":        []string{"Vulnerability Management"},
					"summary":     "Create vulnerability",
					"description": "Create a new vulnerability record",
					"operationId": "createVulnerability",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/CreateVulnerabilityRequest",
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Created successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/Vulnerability",
									},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "Invalid request parameters",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/vulnerabilities/stats": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Vulnerability Management"},
					"summary":     "Get vulnerability statistics",
					"description": "Get vulnerability statistics",
					"operationId": "getVulnerabilityStats",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Retrieved successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/VulnerabilityStats",
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/vulnerabilities/{id}": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Vulnerability Management"},
					"summary":     "Get vulnerability",
					"description": "Get detailed information about the specified vulnerability",
					"operationId": "getVulnerability",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "Vulnerability ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Retrieved successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/Vulnerability",
									},
								},
							},
						},
						"404": map[string]interface{}{
							"description": "Vulnerability not found",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
				"put": map[string]interface{}{
					"tags":        []string{"Vulnerability Management"},
					"summary":     "Update vulnerability",
					"description": "Update vulnerability information",
					"operationId": "updateVulnerability",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "Vulnerability ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/UpdateVulnerabilityRequest",
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Updated successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/Vulnerability",
									},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "Invalid request parameters",
						},
						"404": map[string]interface{}{
							"description": "Vulnerability not found",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
				"delete": map[string]interface{}{
					"tags":        []string{"Vulnerability Management"},
					"summary":     "Delete vulnerability",
					"description": "Delete the specified vulnerability",
					"operationId": "deleteVulnerability",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "Vulnerability ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Deleted successfully",
						},
						"404": map[string]interface{}{
							"description": "Vulnerability not found",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/roles": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Role Management"},
					"summary":     "List roles",
					"description": "Get all security testing roles",
					"operationId": "getRoles",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Retrieved successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"roles": map[string]interface{}{
												"type":        "array",
												"description": "Role list",
												"items": map[string]interface{}{
													"$ref": "#/components/schemas/RoleConfig",
												},
											},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
				"post": map[string]interface{}{
					"tags":        []string{"Role Management"},
					"summary":     "Create role",
					"description": "Create a new security testing role",
					"operationId": "createRole",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/RoleConfig",
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Created successfully",
						},
						"400": map[string]interface{}{
							"description": "Invalid request parameters",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/roles/{name}": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Role Management"},
					"summary":     "Get role",
					"description": "Get detailed information about the specified role",
					"operationId": "getRole",
					"parameters": []map[string]interface{}{
						{
							"name":        "name",
							"in":          "path",
							"required":    true,
							"description": "Role name",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Retrieved successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"role": map[string]interface{}{
												"$ref": "#/components/schemas/RoleConfig",
											},
										},
									},
								},
							},
						},
						"404": map[string]interface{}{
							"description": "Role not found",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
				"put": map[string]interface{}{
					"tags":        []string{"Role Management"},
					"summary":     "Update role",
					"description": "Update configuration for the specified role",
					"operationId": "updateRole",
					"parameters": []map[string]interface{}{
						{
							"name":        "name",
							"in":          "path",
							"required":    true,
							"description": "Role name",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/RoleConfig",
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Updated successfully",
						},
						"400": map[string]interface{}{
							"description": "Invalid request parameters",
						},
						"404": map[string]interface{}{
							"description": "Role not found",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
				"delete": map[string]interface{}{
					"tags":        []string{"Role Management"},
					"summary":     "Delete role",
					"description": "Delete the specified role",
					"operationId": "deleteRole",
					"parameters": []map[string]interface{}{
						{
							"name":        "name",
							"in":          "path",
							"required":    true,
							"description": "Role name",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Deleted successfully",
						},
						"404": map[string]interface{}{
							"description": "Role not found",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/roles/skills/list": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Role Management"},
					"summary":     "Get available skills list",
					"description": "Get all available skills list for role configuration",
					"operationId": "getSkills",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Retrieved successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"skills": map[string]interface{}{
												"type":        "array",
												"description": "Skills list",
												"items": map[string]interface{}{
													"type": "string",
												},
											},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/skills": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Skills Management"},
					"summary":     "List skills",
					"description": "Get all skills list with pagination and search support",
					"operationId": "getSkills",
					"parameters": []map[string]interface{}{
						{
							"name":        "limit",
							"in":          "query",
							"required":    false,
							"description": "Page size",
							"schema": map[string]interface{}{
								"type":    "integer",
								"default": 20,
							},
						},
						{
							"name":        "offset",
							"in":          "query",
							"required":    false,
							"description": "Offset",
							"schema": map[string]interface{}{
								"type":    "integer",
								"default": 0,
							},
						},
						{
							"name":        "search",
							"in":          "query",
							"required":    false,
							"description": "Search keyword",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Retrieved successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"skills": map[string]interface{}{
												"type":        "array",
												"description": "Skills list",
												"items": map[string]interface{}{
													"$ref": "#/components/schemas/Skill",
												},
											},
											"total": map[string]interface{}{
												"type":        "integer",
												"description": "Total",
											},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
				"post": map[string]interface{}{
					"tags":        []string{"Skills Management"},
					"summary":     "Create skill",
					"description": "Create a new skill",
					"operationId": "createSkill",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/CreateSkillRequest",
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Created successfully",
						},
						"400": map[string]interface{}{
							"description": "Invalid request parameters",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/skills/stats": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Skills Management"},
					"summary":     "Get skill statistics",
					"description": "Get skill call statistics",
					"operationId": "getSkillStats",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Retrieved successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type":        "object",
										"description": "Statistics",
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
				"delete": map[string]interface{}{
					"tags":        []string{"Skills Management"},
					"summary":     "Clear skill statistics",
					"description": "Clear call statistics for all skills",
					"operationId": "clearSkillStats",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Cleared successfully",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/skills/{name}": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Skills Management"},
					"summary":     "Get skill",
					"description": "Get detailed information about the specified skill",
					"operationId": "getSkill",
					"parameters": []map[string]interface{}{
						{
							"name":        "name",
							"in":          "path",
							"required":    true,
							"description": "Skill name",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Retrieved successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/Skill",
									},
								},
							},
						},
						"404": map[string]interface{}{
							"description": "Skill not found",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
				"put": map[string]interface{}{
					"tags":        []string{"Skills Management"},
					"summary":     "Update skill",
					"description": "Update information for the specified skill",
					"operationId": "updateSkill",
					"parameters": []map[string]interface{}{
						{
							"name":        "name",
							"in":          "path",
							"required":    true,
							"description": "Skill name",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/UpdateSkillRequest",
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Updated successfully",
						},
						"400": map[string]interface{}{
							"description": "Invalid request parameters",
						},
						"404": map[string]interface{}{
							"description": "Skill not found",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
				"delete": map[string]interface{}{
					"tags":        []string{"Skills Management"},
					"summary":     "Delete skill",
					"description": "Delete the specified skill",
					"operationId": "deleteSkill",
					"parameters": []map[string]interface{}{
						{
							"name":        "name",
							"in":          "path",
							"required":    true,
							"description": "Skill name",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Deleted successfully",
						},
						"404": map[string]interface{}{
							"description": "Skill not found",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/skills/{name}/bound-roles": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Skills Management"},
					"summary":     "Get bound roles",
					"description": "Get all roles that use the specified skill",
					"operationId": "getSkillBoundRoles",
					"parameters": []map[string]interface{}{
						{
							"name":        "name",
							"in":          "path",
							"required":    true,
							"description": "Skill name",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Retrieved successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"roles": map[string]interface{}{
												"type":        "array",
												"description": "Role list",
												"items": map[string]interface{}{
													"type": "string",
												},
											},
										},
									},
								},
							},
						},
						"404": map[string]interface{}{
							"description": "Skill not found",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/skills/{name}/stats": map[string]interface{}{
				"delete": map[string]interface{}{
					"tags":        []string{"Skills Management"},
					"summary":     "Clear skill statistics",
					"description": "Clear call statistics for the specified skill",
					"operationId": "clearSkillStatsByName",
					"parameters": []map[string]interface{}{
						{
							"name":        "name",
							"in":          "path",
							"required":    true,
							"description": "Skill name",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Cleared successfully",
						},
						"404": map[string]interface{}{
							"description": "Skill not found",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/monitor": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Monitoring"},
					"summary":     "Get monitoring information",
					"description": "Get tool execution monitoring information with pagination and filtering support",
					"operationId": "monitor",
					"parameters": []map[string]interface{}{
						{
							"name":        "page",
							"in":          "query",
							"required":    false,
							"description": "Page number",
							"schema": map[string]interface{}{
								"type":    "integer",
								"default": 1,
								"minimum": 1,
							},
						},
						{
							"name":        "page_size",
							"in":          "query",
							"required":    false,
							"description": "Page size",
							"schema": map[string]interface{}{
								"type":    "integer",
								"default": 20,
								"minimum": 1,
								"maximum": 100,
							},
						},
						{
							"name":        "status",
							"in":          "query",
							"required":    false,
							"description": "Status filter",
							"schema": map[string]interface{}{
								"type": "string",
								"enum": []string{"success", "failed", "running"},
							},
						},
						{
							"name":        "tool",
							"in":          "query",
							"required":    false,
							"description": "Tool name filter (supports partial matching)",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Retrieved successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/MonitorResponse",
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/monitor/execution/{id}": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Monitoring"},
					"summary":     "Get execution record",
					"description": "Get detailed information about the specified execution record",
					"operationId": "getExecution",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "Execution ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Retrieved successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/ToolExecution",
									},
								},
							},
						},
						"404": map[string]interface{}{
							"description": "Execution record not found",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
				"delete": map[string]interface{}{
					"tags":        []string{"Monitoring"},
					"summary":     "Delete execution record",
					"description": "Delete the specified execution record",
					"operationId": "deleteExecution",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "Execution ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Deleted successfully",
						},
						"404": map[string]interface{}{
							"description": "Execution record not found",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/monitor/executions": map[string]interface{}{
				"delete": map[string]interface{}{
					"tags":        []string{"Monitoring"},
					"summary":     "Bulk delete execution records",
					"description": "Bulk delete execution records",
					"operationId": "deleteExecutions",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Deleted successfully",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/monitor/stats": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Monitoring"},
					"summary":     "Get statistics",
					"description": "Get tool execution statistics",
					"operationId": "getStats",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Retrieved successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type":        "object",
										"description": "Statistics",
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/config": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Configuration Management"},
					"summary":     "Get configuration",
					"description": "Get system configuration information",
					"operationId": "getConfig",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Retrieved successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/ConfigResponse",
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
				"put": map[string]interface{}{
					"tags":        []string{"Configuration Management"},
					"summary":     "Update configuration",
					"description": "Update system configuration",
					"operationId": "updateConfig",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/UpdateConfigRequest",
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Updated successfully",
						},
						"400": map[string]interface{}{
							"description": "Invalid request parameters",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/config/tools": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Configuration Management"},
					"summary":     "Get tool configuration",
					"description": "Get configuration information for all tools",
					"operationId": "getTools",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Retrieved successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type":        "array",
										"description": "Tool configuration list",
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/config/apply": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"Configuration Management"},
					"summary":     "Apply configuration",
					"description": "Apply configuration changes",
					"operationId": "applyConfig",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Applied successfully",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/external-mcp": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"External MCP Management"},
					"summary":     "List external MCPs",
					"description": "Get all external MCP configurations and status",
					"operationId": "getExternalMCPs",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Retrieved successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"servers": map[string]interface{}{
												"type":        "object",
												"description": "MCP server configuration",
												"additionalProperties": map[string]interface{}{
													"$ref": "#/components/schemas/ExternalMCPResponse",
												},
											},
											"stats": map[string]interface{}{
												"type":        "object",
												"description": "Statistics",
											},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/external-mcp/stats": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"External MCP Management"},
					"summary":     "Get external MCP statistics",
					"description": "Get external MCP statistics",
					"operationId": "getExternalMCPStats",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Retrieved successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type":        "object",
										"description": "Statistics",
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/external-mcp/{name}": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"External MCP Management"},
					"summary":     "Get external MCP",
					"description": "Get configuration and status for the specified external MCP",
					"operationId": "getExternalMCP",
					"parameters": []map[string]interface{}{
						{
							"name":        "name",
							"in":          "path",
							"required":    true,
							"description": "MCP name",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Retrieved successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/ExternalMCPResponse",
									},
								},
							},
						},
						"404": map[string]interface{}{
							"description": "MCP not found",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
				"put": map[string]interface{}{
					"tags":        []string{"External MCP Management"},
					"summary":     "Add or update external MCP",
					"description": "Add a new external MCP configuration or update an existing one.\n**Transport modes**:\nTwo transport modes are supported:\n**1. stdio (standard input/output)**:\n```json\n{\n  \"config\": {\n    \"enabled\": true,\n    \"command\": \"node\",\n    \"args\": [\"/path/to/mcp-server.js\"],\n    \"env\": {}\n  }\n}\n```\n**2. sse (Server-Sent Events)**:\n```json\n{\n  \"config\": {\n    \"enabled\": true,\n    \"transport\": \"sse\",\n    \"url\": \"http://127.0.0.1:8082/sse\",\n    \"timeout\": 30\n  }\n}\n```\n**Configuration parameter descriptions**:\n- `enabled`: whether enabled (boolean, required)\n- `command`: command (required for stdio mode, e.g.: \"node\", \"python\")\n- `args`: command argument array (required for stdio mode)\n- `env`: environment variables (object, optional)\n- `transport`: transport mode (\"stdio\" or \"sse\", required for sse mode)\n- `url`: SSE endpoint URL (required for sse mode)\n- `timeout`: timeout in seconds (optional, default 30)\n- `description`: description (optional)",
					"operationId": "addOrUpdateExternalMCP",
					"parameters": []map[string]interface{}{
						{
							"name":        "name",
							"in":          "path",
							"required":    true,
							"description": "MCP name (unique identifier)",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/AddOrUpdateExternalMCPRequest",
								},
								"examples": map[string]interface{}{
									"stdio": map[string]interface{}{
										"summary":     "stdio mode configuration",
										"description": "Connect to external MCP server using standard input/output",
										"value": map[string]interface{}{
											"config": map[string]interface{}{
												"enabled":     true,
												"command":     "node",
												"args":        []string{"/path/to/mcp-server.js"},
												"env":         map[string]interface{}{},
												"timeout":     30,
												"description": "Node.js MCP server",
											},
										},
									},
									"sse": map[string]interface{}{
										"summary":     "SSE mode configuration",
										"description": "Connect to external MCP server using Server-Sent Events",
										"value": map[string]interface{}{
											"config": map[string]interface{}{
												"enabled":     true,
												"transport":   "sse",
												"url":         "http://127.0.0.1:8082/sse",
												"timeout":     30,
												"description": "SSE MCP server",
											},
										},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Operation successful",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"message": map[string]interface{}{
												"type":    "string",
												"example": "External MCP configuration saved",
											},
										},
									},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "Invalid request parameters (e.g. incorrect configuration format, missing required fields)",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/Error",
									},
									"example": map[string]interface{}{
										"error": "stdio mode requires command and args parameters",
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
				"delete": map[string]interface{}{
					"tags":        []string{"External MCP Management"},
					"summary":     "Delete external MCP",
					"description": "Delete the specified external MCP configuration",
					"operationId": "deleteExternalMCP",
					"parameters": []map[string]interface{}{
						{
							"name":        "name",
							"in":          "path",
							"required":    true,
							"description": "MCP name",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Deleted successfully",
						},
						"404": map[string]interface{}{
							"description": "MCP not found",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/external-mcp/{name}/start": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"External MCP Management"},
					"summary":     "Start external MCP",
					"description": "Start the specified external MCP server",
					"operationId": "startExternalMCP",
					"parameters": []map[string]interface{}{
						{
							"name":        "name",
							"in":          "path",
							"required":    true,
							"description": "MCP name",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Started successfully",
						},
						"404": map[string]interface{}{
							"description": "MCP not found",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/external-mcp/{name}/stop": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"External MCP Management"},
					"summary":     "Stop external MCP",
					"description": "Stop the specified external MCP server",
					"operationId": "stopExternalMCP",
					"parameters": []map[string]interface{}{
						{
							"name":        "name",
							"in":          "path",
							"required":    true,
							"description": "MCP name",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Stopped successfully",
						},
						"404": map[string]interface{}{
							"description": "MCP not found",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/attack-chain/{conversationId}": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Attack Chain"},
					"summary":     "Get attack chain",
					"description": "Get attack chain visualization data for the specified conversation",
					"operationId": "getAttackChain",
					"parameters": []map[string]interface{}{
						{
							"name":        "conversationId",
							"in":          "path",
							"required":    true,
							"description": "Conversation ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Retrieved successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/AttackChain",
									},
								},
							},
						},
						"404": map[string]interface{}{
							"description": "Conversation not found",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/attack-chain/{conversationId}/regenerate": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"Attack Chain"},
					"summary":     "Regenerate attack chain",
					"description": "Regenerate attack chain visualization data for the specified conversation",
					"operationId": "regenerateAttackChain",
					"parameters": []map[string]interface{}{
						{
							"name":        "conversationId",
							"in":          "path",
							"required":    true,
							"description": "Conversation ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Regenerated successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/AttackChain",
									},
								},
							},
						},
						"404": map[string]interface{}{
							"description": "Conversation not found",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/conversations/{id}/pinned": map[string]interface{}{
				"put": map[string]interface{}{
					"tags":        []string{"Conversation Management"},
					"summary":     "Set conversation pin status",
					"description": "Set or unset conversation pin status",
					"operationId": "updateConversationPinned",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "Conversation ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type":     "object",
									"required": []string{"pinned"},
									"properties": map[string]interface{}{
										"pinned": map[string]interface{}{
											"type":        "boolean",
											"description": "Whether pinned",
										},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Updated successfully",
						},
						"404": map[string]interface{}{
							"description": "Conversation not found",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/groups/{id}/pinned": map[string]interface{}{
				"put": map[string]interface{}{
					"tags":        []string{"Conversation Groups"},
					"summary":     "Set group pin status",
					"description": "Set or unset group pin status",
					"operationId": "updateGroupPinned",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "Group ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type":     "object",
									"required": []string{"pinned"},
									"properties": map[string]interface{}{
										"pinned": map[string]interface{}{
											"type":        "boolean",
											"description": "Whether pinned",
										},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Updated successfully",
						},
						"404": map[string]interface{}{
							"description": "Group not found",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/groups/{id}/conversations/{conversationId}/pinned": map[string]interface{}{
				"put": map[string]interface{}{
					"tags":        []string{"Conversation Groups"},
					"summary":     "Set conversation pin in group",
					"description": "Set or unset conversation pin status within a group",
					"operationId": "updateConversationPinnedInGroup",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "Group ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
						{
							"name":        "conversationId",
							"in":          "path",
							"required":    true,
							"description": "Conversation ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type":     "object",
									"required": []string{"pinned"},
									"properties": map[string]interface{}{
										"pinned": map[string]interface{}{
											"type":        "boolean",
											"description": "Whether pinned",
										},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Updated successfully",
						},
						"404": map[string]interface{}{
							"description": "Conversation or group not found",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/knowledge/categories": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Knowledge Base"},
					"summary":     "Get categories",
					"description": "Get all knowledge base categories",
					"operationId": "getKnowledgeCategories",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Retrieved successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"categories": map[string]interface{}{
												"type":        "array",
												"description": "Category list",
												"items": map[string]interface{}{
													"type": "string",
												},
											},
											"enabled": map[string]interface{}{
												"type":        "boolean",
												"description": "Whether knowledge base is enabled",
											},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/knowledge/items": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Knowledge Base"},
					"summary":     "List knowledge items",
					"description": "Get all knowledge items in the knowledge base",
					"operationId": "getKnowledgeItems",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Retrieved successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"items": map[string]interface{}{
												"type":        "array",
												"description": "Knowledge item list",
											},
											"enabled": map[string]interface{}{
												"type":        "boolean",
												"description": "Whether knowledge base is enabled",
											},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
				"post": map[string]interface{}{
					"tags":        []string{"Knowledge Base"},
					"summary":     "Create knowledge item",
					"description": "Create a new knowledge item",
					"operationId": "createKnowledgeItem",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type":        "object",
									"description": "Knowledge item data",
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Created successfully",
						},
						"400": map[string]interface{}{
							"description": "Invalid request parameters",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/knowledge/items/{id}": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Knowledge Base"},
					"summary":     "Get knowledge item",
					"description": "Get detailed information about the specified knowledge item",
					"operationId": "getKnowledgeItem",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "Knowledge item ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Retrieved successfully",
						},
						"404": map[string]interface{}{
							"description": "Knowledge item not found",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
				"put": map[string]interface{}{
					"tags":        []string{"Knowledge Base"},
					"summary":     "Update knowledge item",
					"description": "Update the specified knowledge item",
					"operationId": "updateKnowledgeItem",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "Knowledge item ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type":        "object",
									"description": "Knowledge item data",
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Updated successfully",
						},
						"404": map[string]interface{}{
							"description": "Knowledge item not found",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
				"delete": map[string]interface{}{
					"tags":        []string{"Knowledge Base"},
					"summary":     "Delete knowledge item",
					"description": "Delete the specified knowledge item",
					"operationId": "deleteKnowledgeItem",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "Knowledge item ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Deleted successfully",
						},
						"404": map[string]interface{}{
							"description": "Knowledge item not found",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/knowledge/index-status": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Knowledge Base"},
					"summary":     "Get index status",
					"description": "Get the build status of the knowledge base index",
					"operationId": "getIndexStatus",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Retrieved successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"enabled": map[string]interface{}{
												"type":        "boolean",
												"description": "Whether knowledge base is enabled",
											},
											"total_items": map[string]interface{}{
												"type":        "integer",
												"description": "Total knowledge items",
											},
											"indexed_items": map[string]interface{}{
												"type":        "integer",
												"description": "Number of indexed knowledge items",
											},
											"progress_percent": map[string]interface{}{
												"type":        "number",
												"description": "Index progress percentage",
											},
											"is_complete": map[string]interface{}{
												"type":        "boolean",
												"description": "Whether indexing is complete",
											},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/knowledge/index": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"Knowledge Base"},
					"summary":     "Rebuild index",
					"description": "Rebuild the knowledge base index",
					"operationId": "rebuildIndex",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Index rebuild task started",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/knowledge/scan": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"Knowledge Base"},
					"summary":     "Scan knowledge base",
					"description": "Scan knowledge base directory and import new knowledge files",
					"operationId": "scanKnowledgeBase",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Scan task started",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/knowledge/search": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"Knowledge Base"},
					"summary":     "Search knowledge base",
					"description": "Search for relevant content in the knowledge base. Uses vector retrieval and hybrid search technology to automatically find the most relevant knowledge chunks based on semantic similarity and keyword matching.\n**Search notes**:\n- Supports semantic similarity search (vector retrieval)\n- Supports keyword matching (BM25)\n- Supports hybrid search (combining vector and keyword)\n- Can filter by risk type (e.g.: SQL Injection, XSS, File Upload, etc.)\n- Recommended to call `/api/knowledge/categories` first to get available risk type list\n**Usage example**:\n```json\n{\n  \"query\": \"SQL injection vulnerability detection methods\",\n  \"riskType\": \"SQL Injection\",\n  \"topK\": 5,\n  \"threshold\": 0.7\n}\n```",
					"operationId": "searchKnowledge",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type":     "object",
									"required": []string{"query"},
									"properties": map[string]interface{}{
										"query": map[string]interface{}{
											"type":        "string",
											"description": "Search query content, describing the security knowledge topic you want to learn about (required)",
											"example":     "SQL injection vulnerability detection methods",
										},
										"riskType": map[string]interface{}{
											"type":        "string",
											"description": "Optional: specify risk type (e.g.: SQL Injection, XSS, File Upload, etc.). Recommended to call `/api/knowledge/categories` first to get the available risk type list, then use the correct risk type for precise search to significantly reduce retrieval time. If not specified, all types are searched.",
											"example":     "SQL Injection",
										},
										"topK": map[string]interface{}{
											"type":        "integer",
											"description": "Optional: number of top-K results to return, default 5",
											"default":     5,
											"minimum":     1,
											"maximum":     50,
											"example":     5,
										},
										"threshold": map[string]interface{}{
											"type":        "number",
											"format":      "float",
											"description": "Optional: similarity threshold (between 0 and 1), default 0.7. Only results with similarity >= this value are returned",
											"default":     0.7,
											"minimum":     0,
											"maximum":     1,
											"example":     0.7,
										},
									},
								},
								"examples": map[string]interface{}{
									"basic": map[string]interface{}{
										"summary":     "Basic search",
										"description": "Simplest search, only provide the query content",
										"value": map[string]interface{}{
											"query": "SQL injection vulnerability detection methods",
										},
									},
									"withRiskType": map[string]interface{}{
										"summary":     "Search by risk type",
										"description": "Specify risk type for precise search",
										"value": map[string]interface{}{
											"query":     "SQL injection vulnerability detection methods",
											"riskType":  "SQL Injection",
											"topK":      5,
											"threshold": 0.7,
										},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Search successful",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"results": map[string]interface{}{
												"type":        "array",
												"description": "Search result list; each result contains: item (knowledge item info), chunks (matching knowledge chunks), score (similarity score)",
												"items": map[string]interface{}{
													"type": "object",
													"properties": map[string]interface{}{
														"item": map[string]interface{}{
															"type":        "object",
															"description": "Knowledge item information",
														},
														"chunks": map[string]interface{}{
															"type":        "array",
															"description": "List of matching knowledge chunks",
														},
														"score": map[string]interface{}{
															"type":        "number",
															"description": "Similarity score (between 0 and 1)",
														},
													},
												},
											},
											"enabled": map[string]interface{}{
												"type":        "boolean",
												"description": "Whether knowledge base is enabled",
											},
										},
									},
									"example": map[string]interface{}{
										"results": []map[string]interface{}{
											{
												"item": map[string]interface{}{
													"id":       "item-1",
													"title":    "SQL Injection Vulnerability Detection",
													"category": "SQL Injection",
												},
												"chunks": []map[string]interface{}{
													{
														"text": "SQL injection vulnerability detection methods include...",
													},
												},
												"score": 0.85,
											},
										},
										"enabled": true,
									},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "Invalid request parameters (e.g. query is empty)",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/Error",
									},
									"example": map[string]interface{}{
										"error": "Query cannot be empty",
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
						"500": map[string]interface{}{
							"description": "Internal server error (e.g. knowledge base not enabled or retrieval failed)",
						},
					},
				},
			},
			"/api/knowledge/retrieval-logs": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Knowledge Base"},
					"summary":     "Get retrieval logs",
					"description": "Get knowledge base retrieval logs",
					"operationId": "getRetrievalLogs",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Retrieved successfully",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"logs": map[string]interface{}{
												"type":        "array",
												"description": "Retrieval log list",
											},
											"enabled": map[string]interface{}{
												"type":        "boolean",
												"description": "Whether knowledge base is enabled",
											},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/knowledge/retrieval-logs/{id}": map[string]interface{}{
				"delete": map[string]interface{}{
					"tags":        []string{"Knowledge Base"},
					"summary":     "Delete retrieval log",
					"description": "Delete the specified retrieval log",
					"operationId": "deleteRetrievalLog",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "Log ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Deleted successfully",
						},
						"404": map[string]interface{}{
							"description": "Log not found",
						},
						"401": map[string]interface{}{
							"description": "Unauthorized",
						},
					},
				},
			},
			"/api/mcp": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"MCP"},
					"summary":     "MCP endpoint",
					"description": "MCP (Model Context Protocol) endpoint for processing MCP protocol requests.\n**Protocol description**:\nThis endpoint follows the JSON-RPC 2.0 specification and supports the following methods:\n**1. initialize** - Initialize MCP connection\n```json\n{\n  \"jsonrpc\": \"2.0\",\n  \"id\": \"init-1\",\n  \"method\": \"initialize\",\n  \"params\": {\n    \"protocolVersion\": \"2024-11-05\",\n    \"capabilities\": {},\n    \"clientInfo\": {\n      \"name\": \"MyClient\",\n      \"version\": \"1.0.0\"\n    }\n  }\n}\n```\n**2. tools/list** - List all available tools\n```json\n{\n  \"jsonrpc\": \"2.0\",\n  \"id\": \"list-1\",\n  \"method\": \"tools/list\",\n  \"params\": {}\n}\n```\n**3. tools/call** - Call a tool\n```json\n{\n  \"jsonrpc\": \"2.0\",\n  \"id\": \"call-1\",\n  \"method\": \"tools/call\",\n  \"params\": {\n    \"name\": \"nmap\",\n    \"arguments\": {\n      \"target\": \"192.168.1.1\",\n      \"ports\": \"80,443\"\n    }\n  }\n}\n```\n**4. prompts/list** - List all prompt templates\n```json\n{\n  \"jsonrpc\": \"2.0\",\n  \"id\": \"prompts-list-1\",\n  \"method\": \"prompts/list\",\n  \"params\": {}\n}\n```\n**5. prompts/get** - Get a prompt template\n```json\n{\n  \"jsonrpc\": \"2.0\",\n  \"id\": \"prompt-get-1\",\n  \"method\": \"prompts/get\",\n  \"params\": {\n    \"name\": \"prompt-name\",\n    \"arguments\": {}\n  }\n}\n```\n**6. resources/list** - List all resources\n```json\n{\n  \"jsonrpc\": \"2.0\",\n  \"id\": \"resources-list-1\",\n  \"method\": \"resources/list\",\n  \"params\": {}\n}\n```\n**7. resources/read** - Read resource content\n```json\n{\n  \"jsonrpc\": \"2.0\",\n  \"id\": \"resource-read-1\",\n  \"method\": \"resources/read\",\n  \"params\": {\n    \"uri\": \"resource://example\"\n  }\n}\n```\n**Error code descriptions**:\n- `-32700`: Parse error - JSON parsing error\n- `-32600`: Invalid Request - invalid request\n- `-32601`: Method not found - method does not exist\n- `-32602`: Invalid params - invalid parameters\n- `-32603`: Internal error - internal error",
					"operationId": "mcpEndpoint",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/MCPMessage",
								},
								"examples": map[string]interface{}{
									"listTools": map[string]interface{}{
										"summary":     "List all tools",
										"description": "Get a list of all available MCP tools in the system",
										"value": map[string]interface{}{
											"jsonrpc": "2.0",
											"id":      "list-tools-1",
											"method":  "tools/list",
											"params":  map[string]interface{}{},
										},
									},
									"callTool": map[string]interface{}{
										"summary":     "Call tool",
										"description": "Call the specified MCP tool",
										"value": map[string]interface{}{
											"jsonrpc": "2.0",
											"id":      "call-tool-1",
											"method":  "tools/call",
											"params": map[string]interface{}{
												"name": "nmap",
												"arguments": map[string]interface{}{
													"target": "192.168.1.1",
													"ports":  "80,443",
												},
											},
										},
									},
									"initialize": map[string]interface{}{
										"summary":     "Initialize connection",
										"description": "Initialize MCP connection and get server capabilities",
										"value": map[string]interface{}{
											"jsonrpc": "2.0",
											"id":      "init-1",
											"method":  "initialize",
											"params": map[string]interface{}{
												"protocolVersion": "2024-11-05",
												"capabilities":    map[string]interface{}{},
												"clientInfo": map[string]interface{}{
													"name":    "MyClient",
													"version": "1.0.0",
												},
											},
										},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "MCP response (JSON-RPC 2.0 format)",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/MCPResponse",
									},
									"examples": map[string]interface{}{
										"success": map[string]interface{}{
											"summary":     "Successful response",
											"description": "Example of a successful tool call response",
											"value": map[string]interface{}{
												"jsonrpc": "2.0",
												"id":      "call-tool-1",
												"result": map[string]interface{}{
													"content": []map[string]interface{}{
														{
															"type": "text",
															"text": "Tool execution result...",
														},
													},
													"isError": false,
												},
											},
										},
										"error": map[string]interface{}{
											"summary":     "Error response",
											"description": "Example of a failed tool call response",
											"value": map[string]interface{}{
												"jsonrpc": "2.0",
												"id":      "call-tool-1",
												"error": map[string]interface{}{
													"code":    -32601,
													"message": "Tool not found",
													"data":    "Tool 'unknown-tool' does not exist",
												},
											},
										},
									},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "Invalid request format (JSON parsing failed)",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/MCPResponse",
									},
									"example": map[string]interface{}{
										"id": nil,
										"error": map[string]interface{}{
											"code":    -32700,
											"message": "Parse error",
											"data":    "unexpected end of JSON input",
										},
										"jsonrpc": "2.0",
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "Unauthorized, a valid token is required",
						},
						"405": map[string]interface{}{
							"description": "Method not allowed (only POST requests are supported)",
						},
					},
				},
			},
		},

		// ── Plugin Management ──────────────────────────────────
		"/api/plugins": map[string]interface{}{
			"get": map[string]interface{}{
				"tags": []string{"Plugin Management"}, "summary": "List Plugins",
				"description": "List all discovered plugins with their state, configuration, and capabilities.",
				"responses":   map[string]interface{}{"200": map[string]interface{}{"description": "Plugin list returned"}, "401": map[string]interface{}{"description": "Unauthorized"}},
			},
		},
		"/api/plugins/{name}/enable": map[string]interface{}{
			"post": map[string]interface{}{
				"tags": []string{"Plugin Management"}, "summary": "Enable Plugin",
				"description": "Enable a plugin and hot-load its tools into the MCP server. Tools become immediately available to the AI agent without restart.",
				"parameters":  []map[string]interface{}{{"name": "name", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string"}}},
				"responses":   map[string]interface{}{"200": map[string]interface{}{"description": "Plugin enabled, tools loaded"}, "404": map[string]interface{}{"description": "Plugin not found"}},
			},
		},
		"/api/plugins/{name}/disable": map[string]interface{}{
			"post": map[string]interface{}{
				"tags": []string{"Plugin Management"}, "summary": "Disable Plugin",
				"description": "Disable a plugin and remove its tools from the MCP server.",
				"parameters":  []map[string]interface{}{{"name": "name", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string"}}},
				"responses":   map[string]interface{}{"200": map[string]interface{}{"description": "Plugin disabled"}, "404": map[string]interface{}{"description": "Plugin not found"}},
			},
		},
		"/api/plugins/{name}/config": map[string]interface{}{
			"get": map[string]interface{}{
				"tags": []string{"Plugin Management"}, "summary": "Get Plugin Config",
				"description": "Get plugin configuration variables. Values are masked for security.",
				"parameters":  []map[string]interface{}{{"name": "name", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string"}}},
				"responses":   map[string]interface{}{"200": map[string]interface{}{"description": "Config returned (values masked)"}},
			},
			"post": map[string]interface{}{
				"tags": []string{"Plugin Management"}, "summary": "Set Plugin Config",
				"description": "Set configuration variables for a plugin (e.g., API keys). Values are injected as environment variables when plugin tools execute.",
				"parameters":  []map[string]interface{}{{"name": "name", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string"}}},
				"responses":   map[string]interface{}{"200": map[string]interface{}{"description": "Config saved"}},
			},
		},
		"/api/plugins/{name}/install": map[string]interface{}{
			"post": map[string]interface{}{
				"tags": []string{"Plugin Management"}, "summary": "Install Plugin Dependencies",
				"description": "Install Python dependencies from requirements.txt into an isolated .venv.",
				"parameters":  []map[string]interface{}{{"name": "name", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string"}}},
				"responses":   map[string]interface{}{"200": map[string]interface{}{"description": "Dependencies installed"}, "500": map[string]interface{}{"description": "Install failed"}},
			},
		},
		"/api/plugins/upload": map[string]interface{}{
			"post": map[string]interface{}{
				"tags": []string{"Plugin Management"}, "summary": "Upload Plugin",
				"description": "Upload a plugin ZIP archive. Extracted into plugins/ directory.",
				"responses":   map[string]interface{}{"200": map[string]interface{}{"description": "Plugin uploaded"}, "400": map[string]interface{}{"description": "Invalid archive"}},
			},
		},
		"/api/plugins/{name}": map[string]interface{}{
			"delete": map[string]interface{}{
				"tags": []string{"Plugin Management"}, "summary": "Delete Plugin",
				"description": "Remove a plugin directory and all stored state. Irreversible.",
				"parameters":  []map[string]interface{}{{"name": "name", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string"}}},
				"responses":   map[string]interface{}{"200": map[string]interface{}{"description": "Plugin deleted"}, "404": map[string]interface{}{"description": "Plugin not found"}},
			},
		},
		"/api/config/test-api": map[string]interface{}{
			"post": map[string]interface{}{
				"tags": []string{"Configuration"}, "summary": "Test API Endpoint",
				"description": "Test AI provider API connectivity. Validates key, discovers models, reports rate limits. Supports Anthropic and OpenAI.",
				"responses":   map[string]interface{}{"200": map[string]interface{}{"description": "API test result with models and rate limits"}},
			},
		},
	}

	enrichSpecWithI18nKeys(spec)
	c.JSON(http.StatusOK, spec)
}

// GetConversationResults retrieves conversation results (OpenAPI endpoint)
// Note: Creating conversations and getting conversation details use the standard /api/conversations endpoint directly
// This endpoint is provided only for result aggregation purposes
func (h *OpenAPIHandler) GetConversationResults(c *gin.Context) {
	conversationID := c.Param("id")

	// Verify that the conversation exists
	conv, err := h.db.GetConversation(conversationID)
	if err != nil {
		h.logger.Error("Failed to get conversation", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "Conversation not found"})
		return
	}

	// Get message list
	messages, err := h.db.GetMessages(conversationID)
	if err != nil {
		h.logger.Error("Failed to get messages", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Get vulnerability list
	vulnList, err := h.db.ListVulnerabilities(1000, 0, "", conversationID, "", "")
	if err != nil {
		h.logger.Warn("Failed to get vulnerability list", zap.Error(err))
		vulnList = []*database.Vulnerability{}
	}
	vulnerabilities := make([]database.Vulnerability, len(vulnList))
	for i, v := range vulnList {
		vulnerabilities[i] = *v
	}

	// Get execution results (retrieved from MCP execution records)
	executionResults := []map[string]interface{}{}
	for _, msg := range messages {
		if len(msg.MCPExecutionIDs) > 0 {
			for _, execID := range msg.MCPExecutionIDs {
				// Try to retrieve execution result from result storage
				if h.resultStorage != nil {
					result, err := h.resultStorage.GetResult(execID)
					if err == nil && result != "" {
						// Get metadata to retrieve tool name and creation time
						metadata, err := h.resultStorage.GetResultMetadata(execID)
						toolName := "unknown"
						createdAt := time.Now()
						if err == nil && metadata != nil {
							toolName = metadata.ToolName
							createdAt = metadata.CreatedAt
						}
						executionResults = append(executionResults, map[string]interface{}{
							"id":        execID,
							"toolName":  toolName,
							"status":    "success",
							"result":    result,
							"createdAt": createdAt.Format(time.RFC3339),
						})
					}
				}
			}
		}
	}

	response := map[string]interface{}{
		"conversationId":   conv.ID,
		"messages":         messages,
		"vulnerabilities":  vulnerabilities,
		"executionResults": executionResults,
	}

	c.JSON(http.StatusOK, response)
}
