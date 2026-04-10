package handler

import (
	"net/http"
	"time"

	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/storage"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// OpenAPIHandler OpenAPI处理器
type OpenAPIHandler struct {
	db               *database.DB
	logger           *zap.Logger
	resultStorage    storage.ResultStorage
	conversationHdlr *ConversationHandler
	agentHdlr        *AgentHandler
}

// NewOpenAPIHandler 创建新的OpenAPI处理器
func NewOpenAPIHandler(db *database.DB, logger *zap.Logger, resultStorage storage.ResultStorage, conversationHdlr *ConversationHandler, agentHdlr *AgentHandler) *OpenAPIHandler {
	return &OpenAPIHandler{
		db:               db,
		logger:           logger,
		resultStorage:    resultStorage,
		conversationHdlr: conversationHdlr,
		agentHdlr:        agentHdlr,
	}
}

// GetOpenAPISpec 获取OpenAPI规范
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
			"description": "AI驱动的自动化安全测试平台API文档",
			"version":     "1.0.0",
			"contact": map[string]interface{}{
				"name": "CyberStrikeAI",
			},
		},
		"servers": []map[string]interface{}{
			{
				"url":         scheme + "://" + host,
				"description": "当前服务器",
			},
		},
		"components": map[string]interface{}{
			"securitySchemes": map[string]interface{}{
				"bearerAuth": map[string]interface{}{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "JWT",
					"description":  "使用Bearer Token进行认证。Token通过 /api/auth/login 接口获取。",
				},
			},
			"schemas": map[string]interface{}{
				"CreateConversationRequest": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"title": map[string]interface{}{
							"type":        "string",
							"description": "对话标题",
							"example":     "Web应用安全测试",
						},
					},
				},
				"Conversation": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type":        "string",
							"description": "对话ID",
							"example":     "550e8400-e29b-41d4-a716-446655440000",
						},
						"title": map[string]interface{}{
							"type":        "string",
							"description": "对话标题",
							"example":     "Web应用安全测试",
						},
						"createdAt": map[string]interface{}{
							"type":        "string",
							"format":      "date-time",
							"description": "创建时间",
						},
						"updatedAt": map[string]interface{}{
							"type":        "string",
							"format":      "date-time",
							"description": "更新时间",
						},
					},
				},
				"ConversationDetail": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type":        "string",
							"description": "对话ID",
						},
						"title": map[string]interface{}{
							"type":        "string",
							"description": "对话标题",
						},
						"status": map[string]interface{}{
							"type":        "string",
							"description": "对话状态：active（进行中）、completed（已完成）、failed（失败）",
							"enum":        []string{"active", "completed", "failed"},
						},
						"createdAt": map[string]interface{}{
							"type":        "string",
							"format":      "date-time",
							"description": "创建时间",
						},
						"updatedAt": map[string]interface{}{
							"type":        "string",
							"format":      "date-time",
							"description": "更新时间",
						},
						"messages": map[string]interface{}{
							"type":        "array",
							"description": "消息列表",
							"items": map[string]interface{}{
								"$ref": "#/components/schemas/Message",
							},
						},
						"messageCount": map[string]interface{}{
							"type":        "integer",
							"description": "消息数量",
						},
					},
				},
				"Message": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type":        "string",
							"description": "消息ID",
						},
						"conversationId": map[string]interface{}{
							"type":        "string",
							"description": "对话ID",
						},
						"role": map[string]interface{}{
							"type":        "string",
							"description": "消息角色：user（用户）、assistant（助手）",
							"enum":        []string{"user", "assistant"},
						},
						"content": map[string]interface{}{
							"type":        "string",
							"description": "消息内容",
						},
						"createdAt": map[string]interface{}{
							"type":        "string",
							"format":      "date-time",
							"description": "创建时间",
						},
					},
				},
				"ConversationResults": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"conversationId": map[string]interface{}{
							"type":        "string",
							"description": "对话ID",
						},
						"messages": map[string]interface{}{
							"type":        "array",
							"description": "消息列表",
							"items": map[string]interface{}{
								"$ref": "#/components/schemas/Message",
							},
						},
						"vulnerabilities": map[string]interface{}{
							"type":        "array",
							"description": "发现的漏洞列表",
							"items": map[string]interface{}{
								"$ref": "#/components/schemas/Vulnerability",
							},
						},
						"executionResults": map[string]interface{}{
							"type":        "array",
							"description": "执行结果列表",
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
							"description": "漏洞ID",
						},
						"title": map[string]interface{}{
							"type":        "string",
							"description": "漏洞标题",
						},
						"description": map[string]interface{}{
							"type":        "string",
							"description": "漏洞描述",
						},
						"severity": map[string]interface{}{
							"type":        "string",
							"description": "严重程度",
							"enum":        []string{"critical", "high", "medium", "low", "info"},
						},
						"status": map[string]interface{}{
							"type":        "string",
							"description": "状态",
							"enum":        []string{"open", "closed", "fixed"},
						},
						"target": map[string]interface{}{
							"type":        "string",
							"description": "受影响的目标",
						},
					},
				},
				"ExecutionResult": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type":        "string",
							"description": "执行ID",
						},
						"toolName": map[string]interface{}{
							"type":        "string",
							"description": "工具名称",
						},
						"status": map[string]interface{}{
							"type":        "string",
							"description": "执行状态",
							"enum":        []string{"success", "failed", "running"},
						},
						"result": map[string]interface{}{
							"type":        "string",
							"description": "执行结果",
						},
						"createdAt": map[string]interface{}{
							"type":        "string",
							"format":      "date-time",
							"description": "创建时间",
						},
					},
				},
				"Error": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"error": map[string]interface{}{
							"type":        "string",
							"description": "错误信息",
						},
					},
				},
				"LoginRequest": map[string]interface{}{
					"type":     "object",
					"required": []string{"password"},
					"properties": map[string]interface{}{
						"password": map[string]interface{}{
							"type":        "string",
							"description": "登录密码",
						},
					},
				},
				"LoginResponse": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"token": map[string]interface{}{
							"type":        "string",
							"description": "认证Token",
						},
						"expires_at": map[string]interface{}{
							"type":        "string",
							"format":      "date-time",
							"description": "Token过期时间",
						},
						"session_duration_hr": map[string]interface{}{
							"type":        "integer",
							"description": "会话持续时间（小时）",
						},
					},
				},
				"ChangePasswordRequest": map[string]interface{}{
					"type":     "object",
					"required": []string{"oldPassword", "newPassword"},
					"properties": map[string]interface{}{
						"oldPassword": map[string]interface{}{
							"type":        "string",
							"description": "当前密码",
						},
						"newPassword": map[string]interface{}{
							"type":        "string",
							"description": "新密码（至少8位）",
						},
					},
				},
				"UpdateConversationRequest": map[string]interface{}{
					"type":     "object",
					"required": []string{"title"},
					"properties": map[string]interface{}{
						"title": map[string]interface{}{
							"type":        "string",
							"description": "对话标题",
						},
					},
				},
				"Group": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type":        "string",
							"description": "分组ID",
						},
						"name": map[string]interface{}{
							"type":        "string",
							"description": "分组名称",
						},
						"icon": map[string]interface{}{
							"type":        "string",
							"description": "分组图标",
						},
						"createdAt": map[string]interface{}{
							"type":        "string",
							"format":      "date-time",
							"description": "创建时间",
						},
						"updatedAt": map[string]interface{}{
							"type":        "string",
							"format":      "date-time",
							"description": "更新时间",
						},
					},
				},
				"CreateGroupRequest": map[string]interface{}{
					"type":     "object",
					"required": []string{"name"},
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type":        "string",
							"description": "分组名称",
						},
						"icon": map[string]interface{}{
							"type":        "string",
							"description": "分组图标（可选）",
						},
					},
				},
				"UpdateGroupRequest": map[string]interface{}{
					"type":     "object",
					"required": []string{"name"},
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type":        "string",
							"description": "分组名称",
						},
						"icon": map[string]interface{}{
							"type":        "string",
							"description": "分组图标",
						},
					},
				},
				"AddConversationToGroupRequest": map[string]interface{}{
					"type":     "object",
					"required": []string{"conversationId", "groupId"},
					"properties": map[string]interface{}{
						"conversationId": map[string]interface{}{
							"type":        "string",
							"description": "对话ID",
						},
						"groupId": map[string]interface{}{
							"type":        "string",
							"description": "分组ID",
						},
					},
				},
				"BatchTaskRequest": map[string]interface{}{
					"type":     "object",
					"required": []string{"tasks"},
					"properties": map[string]interface{}{
						"title": map[string]interface{}{
							"type":        "string",
							"description": "任务标题（可选）",
						},
						"tasks": map[string]interface{}{
							"type":        "array",
							"description": "任务列表，每行一个任务",
							"items": map[string]interface{}{
								"type": "string",
							},
						},
						"role": map[string]interface{}{
							"type":        "string",
							"description": "角色名称（可选）",
						},
					},
				},
				"BatchQueue": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type":        "string",
							"description": "队列ID",
						},
						"title": map[string]interface{}{
							"type":        "string",
							"description": "队列标题",
						},
						"status": map[string]interface{}{
							"type":        "string",
							"description": "队列状态",
							"enum":        []string{"pending", "running", "paused", "completed", "failed"},
						},
						"tasks": map[string]interface{}{
							"type":        "array",
							"description": "任务列表",
							"items": map[string]interface{}{
								"type": "object",
							},
						},
						"createdAt": map[string]interface{}{
							"type":        "string",
							"format":      "date-time",
							"description": "创建时间",
						},
					},
				},
				"CancelAgentLoopRequest": map[string]interface{}{
					"type":     "object",
					"required": []string{"conversationId"},
					"properties": map[string]interface{}{
						"conversationId": map[string]interface{}{
							"type":        "string",
							"description": "对话ID",
						},
					},
				},
				"AgentTask": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"conversationId": map[string]interface{}{
							"type":        "string",
							"description": "对话ID",
						},
						"status": map[string]interface{}{
							"type":        "string",
							"description": "任务状态",
							"enum":        []string{"running", "completed", "failed", "cancelled", "timeout"},
						},
						"startedAt": map[string]interface{}{
							"type":        "string",
							"format":      "date-time",
							"description": "开始时间",
						},
					},
				},
				"CreateVulnerabilityRequest": map[string]interface{}{
					"type":     "object",
					"required": []string{"conversation_id", "title", "severity"},
					"properties": map[string]interface{}{
						"conversation_id": map[string]interface{}{
							"type":        "string",
							"description": "对话ID",
						},
						"title": map[string]interface{}{
							"type":        "string",
							"description": "漏洞标题",
						},
						"description": map[string]interface{}{
							"type":        "string",
							"description": "漏洞描述",
						},
						"severity": map[string]interface{}{
							"type":        "string",
							"description": "严重程度",
							"enum":        []string{"critical", "high", "medium", "low", "info"},
						},
						"status": map[string]interface{}{
							"type":        "string",
							"description": "状态",
							"enum":        []string{"open", "closed", "fixed"},
						},
						"type": map[string]interface{}{
							"type":        "string",
							"description": "漏洞类型",
						},
						"target": map[string]interface{}{
							"type":        "string",
							"description": "受影响的目标",
						},
						"proof": map[string]interface{}{
							"type":        "string",
							"description": "漏洞证明",
						},
						"impact": map[string]interface{}{
							"type":        "string",
							"description": "影响",
						},
						"recommendation": map[string]interface{}{
							"type":        "string",
							"description": "修复建议",
						},
					},
				},
				"UpdateVulnerabilityRequest": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"title": map[string]interface{}{
							"type":        "string",
							"description": "漏洞标题",
						},
						"description": map[string]interface{}{
							"type":        "string",
							"description": "漏洞描述",
						},
						"severity": map[string]interface{}{
							"type":        "string",
							"description": "严重程度",
							"enum":        []string{"critical", "high", "medium", "low", "info"},
						},
						"status": map[string]interface{}{
							"type":        "string",
							"description": "状态",
							"enum":        []string{"open", "closed", "fixed"},
						},
						"type": map[string]interface{}{
							"type":        "string",
							"description": "漏洞类型",
						},
						"target": map[string]interface{}{
							"type":        "string",
							"description": "受影响的目标",
						},
						"proof": map[string]interface{}{
							"type":        "string",
							"description": "漏洞证明",
						},
						"impact": map[string]interface{}{
							"type":        "string",
							"description": "影响",
						},
						"recommendation": map[string]interface{}{
							"type":        "string",
							"description": "修复建议",
						},
					},
				},
				"ListVulnerabilitiesResponse": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"vulnerabilities": map[string]interface{}{
							"type":        "array",
							"description": "漏洞列表",
							"items": map[string]interface{}{
								"$ref": "#/components/schemas/Vulnerability",
							},
						},
						"total": map[string]interface{}{
							"type":        "integer",
							"description": "总数",
						},
						"page": map[string]interface{}{
							"type":        "integer",
							"description": "当前页",
						},
						"page_size": map[string]interface{}{
							"type":        "integer",
							"description": "每页数量",
						},
						"total_pages": map[string]interface{}{
							"type":        "integer",
							"description": "总页数",
						},
					},
				},
				"VulnerabilityStats": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"total": map[string]interface{}{
							"type":        "integer",
							"description": "总漏洞数",
						},
						"by_severity": map[string]interface{}{
							"type":        "object",
							"description": "按严重程度统计",
						},
						"by_status": map[string]interface{}{
							"type":        "object",
							"description": "按状态统计",
						},
					},
				},
				"RoleConfig": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type":        "string",
							"description": "角色名称",
						},
						"description": map[string]interface{}{
							"type":        "string",
							"description": "角色描述",
						},
						"enabled": map[string]interface{}{
							"type":        "boolean",
							"description": "是否启用",
						},
						"systemPrompt": map[string]interface{}{
							"type":        "string",
							"description": "系统提示词",
						},
						"userPrompt": map[string]interface{}{
							"type":        "string",
							"description": "用户提示词",
						},
						"tools": map[string]interface{}{
							"type":        "array",
							"description": "工具列表",
							"items": map[string]interface{}{
								"type": "string",
							},
						},
						"skills": map[string]interface{}{
							"type":        "array",
							"description": "Skills列表",
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
							"description": "Skill名称",
						},
						"description": map[string]interface{}{
							"type":        "string",
							"description": "Skill描述",
						},
						"path": map[string]interface{}{
							"type":        "string",
							"description": "Skill路径",
						},
					},
				},
				"CreateSkillRequest": map[string]interface{}{
					"type":     "object",
					"required": []string{"name", "description"},
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type":        "string",
							"description": "Skill名称",
						},
						"description": map[string]interface{}{
							"type":        "string",
							"description": "Skill描述",
						},
					},
				},
				"UpdateSkillRequest": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"description": map[string]interface{}{
							"type":        "string",
							"description": "Skill描述",
						},
					},
				},
				"ToolExecution": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type":        "string",
							"description": "执行ID",
						},
						"toolName": map[string]interface{}{
							"type":        "string",
							"description": "工具名称",
						},
						"status": map[string]interface{}{
							"type":        "string",
							"description": "执行状态",
							"enum":        []string{"success", "failed", "running"},
						},
						"createdAt": map[string]interface{}{
							"type":        "string",
							"format":      "date-time",
							"description": "创建时间",
						},
					},
				},
				"MonitorResponse": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"executions": map[string]interface{}{
							"type":        "array",
							"description": "执行记录列表",
							"items": map[string]interface{}{
								"$ref": "#/components/schemas/ToolExecution",
							},
						},
						"stats": map[string]interface{}{
							"type":        "object",
							"description": "统计信息",
						},
						"timestamp": map[string]interface{}{
							"type":        "string",
							"format":      "date-time",
							"description": "时间戳",
						},
						"total": map[string]interface{}{
							"type":        "integer",
							"description": "总数",
						},
						"page": map[string]interface{}{
							"type":        "integer",
							"description": "当前页",
						},
						"page_size": map[string]interface{}{
							"type":        "integer",
							"description": "每页数量",
						},
						"total_pages": map[string]interface{}{
							"type":        "integer",
							"description": "总页数",
						},
					},
				},
				"ConfigResponse": map[string]interface{}{
					"type":        "object",
					"description": "配置信息",
				},
				"UpdateConfigRequest": map[string]interface{}{
					"type":        "object",
					"description": "更新配置请求",
				},
				"ExternalMCPConfig": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"enabled": map[string]interface{}{
							"type":        "boolean",
							"description": "是否启用",
						},
						"command": map[string]interface{}{
							"type":        "string",
							"description": "命令",
						},
						"args": map[string]interface{}{
							"type":        "array",
							"description": "参数列表",
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
							"description": "状态",
							"enum":        []string{"connected", "disconnected", "error", "disabled"},
						},
						"toolCount": map[string]interface{}{
							"type":        "integer",
							"description": "工具数量",
						},
						"error": map[string]interface{}{
							"type":        "string",
							"description": "错误信息",
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
					"description": "攻击链数据",
				},
				"MCPMessage": map[string]interface{}{
					"type":        "object",
					"description": "MCP消息（符合JSON-RPC 2.0规范）",
					"required":    []string{"jsonrpc"},
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"description": "消息ID，可以是字符串、数字或null。对于请求，必须提供；对于通知，可以省略",
							"oneOf": []map[string]interface{}{
								{"type": "string"},
								{"type": "number"},
								{"type": "null"},
							},
							"example": "550e8400-e29b-41d4-a716-446655440000",
						},
						"method": map[string]interface{}{
							"type":        "string",
							"description": "方法名。支持的方法：\n- `initialize`: 初始化MCP连接\n- `tools/list`: 列出所有可用工具\n- `tools/call`: 调用工具\n- `prompts/list`: 列出所有提示词模板\n- `prompts/get`: 获取提示词模板\n- `resources/list`: 列出所有资源\n- `resources/read`: 读取资源内容\n- `sampling/request`: 采样请求",
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
							"description": "方法参数（JSON对象），根据不同的method有不同的结构",
							"type":        "object",
						},
						"jsonrpc": map[string]interface{}{
							"type":        "string",
							"description": "JSON-RPC版本，固定为\"2.0\"",
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
							"description": "协议版本",
							"example":     "2024-11-05",
						},
						"capabilities": map[string]interface{}{
							"type":        "object",
							"description": "客户端能力",
						},
						"clientInfo": map[string]interface{}{
							"type":     "object",
							"required": []string{"name", "version"},
							"properties": map[string]interface{}{
								"name": map[string]interface{}{
									"type":        "string",
									"description": "客户端名称",
									"example":     "MyClient",
								},
								"version": map[string]interface{}{
									"type":        "string",
									"description": "客户端版本",
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
							"description": "工具名称",
							"example":     "nmap",
						},
						"arguments": map[string]interface{}{
							"type":        "object",
							"description": "工具参数（键值对），具体参数取决于工具定义",
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
							"description": "消息ID（与请求中的id相同）",
							"oneOf": []map[string]interface{}{
								{"type": "string"},
								{"type": "number"},
								{"type": "null"},
							},
						},
						"result": map[string]interface{}{
							"description": "方法执行结果（JSON对象），结构取决于调用的方法",
							"type":        "object",
						},
						"error": map[string]interface{}{
							"type":        "object",
							"description": "错误信息（如果执行失败）",
							"properties": map[string]interface{}{
								"code": map[string]interface{}{
									"type":        "integer",
									"description": "错误代码",
									"example":     -32600,
								},
								"message": map[string]interface{}{
									"type":        "string",
									"description": "错误消息",
									"example":     "Invalid Request",
								},
								"data": map[string]interface{}{
									"description": "错误详情（可选）",
								},
							},
						},
						"jsonrpc": map[string]interface{}{
							"type":        "string",
							"description": "JSON-RPC版本",
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
					"tags":        []string{"认证"},
					"summary":     "用户登录",
					"description": "使用密码登录获取认证Token",
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
							"description": "登录成功",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/LoginResponse",
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "密码错误",
						},
					},
				},
			},
			"/api/auth/logout": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"认证"},
					"summary":     "用户登出",
					"description": "登出当前会话，使Token失效",
					"operationId": "logout",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "登出成功",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"message": map[string]interface{}{
												"type":    "string",
												"example": "已退出登录",
											},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/auth/change-password": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"认证"},
					"summary":     "修改密码",
					"description": "修改登录密码，修改后所有会话将失效",
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
							"description": "密码修改成功",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"message": map[string]interface{}{
												"type":    "string",
												"example": "密码已更新，请使用新密码重新登录",
											},
										},
									},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "请求参数错误",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/auth/validate": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"认证"},
					"summary":     "验证Token",
					"description": "验证当前Token是否有效",
					"operationId": "validateToken",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Token有效",
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
												"description": "过期时间",
											},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "Token无效或已过期",
						},
					},
				},
			},
			"/api/conversations": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"对话管理"},
					"summary":     "创建对话",
					"description": "创建一个新的安全测试对话。\n**重要说明**：\n- ✅ 创建的对话会**立即保存到数据库**\n- ✅ 前端页面会**自动刷新**显示新对话\n- ✅ 与前端创建的对话**完全一致**\n**创建对话的两种方式**：\n**方式1（推荐）：** 直接使用 `/api/agent-loop` 发送消息，**不提供** `conversationId` 参数，系统会自动创建新对话并发送消息。这是最简单的方式，一步完成创建和发送。\n**方式2：** 先调用此端点创建空对话，然后使用返回的 `conversationId` 调用 `/api/agent-loop` 发送消息。适用于需要先创建对话，稍后再发送消息的场景。\n**示例**：\n```json\n{\n  \"title\": \"Web应用安全测试\"\n}\n```",
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
							"description": "对话创建成功",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/Conversation",
									},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "请求参数错误",
						},
						"401": map[string]interface{}{
							"description": "未授权，需要有效的Token",
						},
						"500": map[string]interface{}{
							"description": "服务器内部错误",
						},
					},
				},
				"get": map[string]interface{}{
					"tags":        []string{"对话管理"},
					"summary":     "列出对话",
					"description": "获取对话列表，支持分页和搜索",
					"operationId": "listConversations",
					"parameters": []map[string]interface{}{
						{
							"name":        "limit",
							"in":          "query",
							"required":    false,
							"description": "返回数量限制",
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
							"description": "偏移量",
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
							"description": "搜索关键词",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "获取成功",
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
							"description": "未授权，需要有效的Token",
						},
					},
				},
			},
			"/api/conversations/{id}": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"对话管理"},
					"summary":     "查看对话详情",
					"description": "获取指定对话的详细信息，包括对话信息和消息列表",
					"operationId": "getConversation",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "对话ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "获取成功",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/ConversationDetail",
									},
								},
							},
						},
						"404": map[string]interface{}{
							"description": "对话不存在",
						},
						"401": map[string]interface{}{
							"description": "未授权，需要有效的Token",
						},
					},
				},
				"put": map[string]interface{}{
					"tags":        []string{"对话管理"},
					"summary":     "更新对话",
					"description": "更新对话标题",
					"operationId": "updateConversation",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "对话ID",
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
							"description": "更新成功",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/Conversation",
									},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "请求参数错误",
						},
						"404": map[string]interface{}{
							"description": "对话不存在",
						},
						"401": map[string]interface{}{
							"description": "未授权，需要有效的Token",
						},
					},
				},
				"delete": map[string]interface{}{
					"tags":        []string{"对话管理"},
					"summary":     "删除对话",
					"description": "删除指定的对话及其所有相关数据（消息、漏洞等）。**此操作不可恢复**。",
					"operationId": "deleteConversation",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "对话ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "删除成功",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"message": map[string]interface{}{
												"type":        "string",
												"description": "成功消息",
												"example":     "删除成功",
											},
										},
									},
								},
							},
						},
						"404": map[string]interface{}{
							"description": "对话不存在",
						},
						"401": map[string]interface{}{
							"description": "未授权，需要有效的Token",
						},
						"500": map[string]interface{}{
							"description": "服务器内部错误",
						},
					},
				},
			},
			"/api/conversations/{id}/results": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"对话管理"},
					"summary":     "获取对话结果",
					"description": "获取指定对话的执行结果，包括消息、漏洞信息和执行结果",
					"operationId": "getConversationResults",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "对话ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "获取成功",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/ConversationResults",
									},
								},
							},
						},
						"404": map[string]interface{}{
							"description": "对话不存在或结果不存在",
						},
						"401": map[string]interface{}{
							"description": "未授权，需要有效的Token",
						},
					},
				},
			},
			"/api/agent-loop": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"对话交互"},
					"summary":     "发送消息并获取AI回复（非流式）",
					"description": "向AI发送消息并获取回复（非流式响应）。**这是与AI交互的核心端点**，与前端聊天功能完全一致。\n**重要说明**：\n- ✅ 通过此API创建/发送的消息会**立即保存到数据库**\n- ✅ 前端页面会**自动刷新**显示新创建的对话和消息\n- ✅ 所有操作都有**完整的交互痕迹**，就像在前端操作一样\n- ✅ 支持角色配置，可以指定使用哪个测试角色\n**推荐使用流程**：\n1. **先创建对话**：调用 `POST /api/conversations` 创建新对话，获取 `conversationId`\n2. **再发送消息**：使用返回的 `conversationId` 调用此端点发送消息\n**使用示例**：\n**步骤1 - 创建对话：**\n```json\nPOST /api/conversations\n{\n  \"title\": \"Web应用安全测试\"\n}\n```\n**步骤2 - 发送消息：**\n```json\nPOST /api/agent-loop\n{\n  \"conversationId\": \"返回的对话ID\",\n  \"message\": \"扫描 http://example.com 的SQL注入漏洞\",\n  \"role\": \"渗透测试\"\n}\n```\n**其他方式**：\n如果不提供 `conversationId`，系统会自动创建新对话并发送消息。但**推荐先创建对话**，这样可以更好地管理对话列表。\n**响应**：返回AI的回复、对话ID和MCP执行ID列表。前端会自动刷新显示新消息。",
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
											"description": "要发送的消息（必需）",
											"example":     "扫描 http://example.com 的SQL注入漏洞",
										},
										"conversationId": map[string]interface{}{
											"type":        "string",
											"description": "对话ID（可选）。\n- **不提供**：自动创建新对话并发送消息（推荐）\n- **提供**：消息会添加到指定对话中（对话必须存在）",
											"example":     "550e8400-e29b-41d4-a716-446655440000",
										},
										"role": map[string]interface{}{
											"type":        "string",
											"description": "角色名称（可选），如：默认、渗透测试、Web应用扫描等",
											"example":     "默认",
										},
									},
									"required": []string{"message"},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "消息发送成功，返回AI回复",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"response": map[string]interface{}{
												"type":        "string",
												"description": "AI的回复内容",
											},
											"conversationId": map[string]interface{}{
												"type":        "string",
												"description": "对话ID",
											},
											"mcpExecutionIds": map[string]interface{}{
												"type":        "array",
												"description": "MCP执行ID列表",
												"items": map[string]interface{}{
													"type": "string",
												},
											},
											"time": map[string]interface{}{
												"type":        "string",
												"format":      "date-time",
												"description": "响应时间",
											},
										},
									},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "请求参数错误",
						},
						"401": map[string]interface{}{
							"description": "未授权，需要有效的Token",
						},
						"500": map[string]interface{}{
							"description": "服务器内部错误",
						},
					},
				},
			},
			"/api/agent-loop/stream": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"对话交互"},
					"summary":     "发送消息并获取AI回复（流式）",
					"description": "向AI发送消息并获取流式回复（Server-Sent Events）。**这是与AI交互的核心端点**，与前端聊天功能完全一致。\n**重要说明**：\n- ✅ 通过此API创建/发送的消息会**立即保存到数据库**\n- ✅ 前端页面会**自动刷新**显示新创建的对话和消息\n- ✅ 所有操作都有**完整的交互痕迹**，就像在前端操作一样\n- ✅ 支持角色配置，可以指定使用哪个测试角色\n- ✅ 返回流式响应，适合实时显示AI回复\n**推荐使用流程**：\n1. **先创建对话**：调用 `POST /api/conversations` 创建新对话，获取 `conversationId`\n2. **再发送消息**：使用返回的 `conversationId` 调用此端点发送消息\n**使用示例**：\n**步骤1 - 创建对话：**\n```json\nPOST /api/conversations\n{\n  \"title\": \"Web应用安全测试\"\n}\n```\n**步骤2 - 发送消息（流式）：**\n```json\nPOST /api/agent-loop/stream\n{\n  \"conversationId\": \"返回的对话ID\",\n  \"message\": \"扫描 http://example.com 的SQL注入漏洞\",\n  \"role\": \"渗透测试\"\n}\n```\n**响应格式**：Server-Sent Events (SSE)，事件类型包括：\n- `message`: 用户消息确认\n- `response`: AI回复片段\n- `progress`: 进度更新\n- `done`: 完成\n- `error`: 错误\n- `cancelled`: 已取消",
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
											"description": "要发送的消息（必需）",
											"example":     "扫描 http://example.com 的SQL注入漏洞",
										},
										"conversationId": map[string]interface{}{
											"type":        "string",
											"description": "对话ID（可选）。\n- **不提供**：自动创建新对话并发送消息（推荐）\n- **提供**：消息会添加到指定对话中（对话必须存在）",
											"example":     "550e8400-e29b-41d4-a716-446655440000",
										},
										"role": map[string]interface{}{
											"type":        "string",
											"description": "角色名称（可选），如：默认、渗透测试、Web应用扫描等",
											"example":     "默认",
										},
									},
									"required": []string{"message"},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "流式响应（Server-Sent Events）",
							"content": map[string]interface{}{
								"text/event-stream": map[string]interface{}{
									"schema": map[string]interface{}{
										"type":        "string",
										"description": "SSE流式数据",
									},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "请求参数错误",
						},
						"401": map[string]interface{}{
							"description": "未授权，需要有效的Token",
						},
						"500": map[string]interface{}{
							"description": "服务器内部错误",
						},
					},
				},
			},
			"/api/multi-agent": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"对话交互"},
					"summary":     "发送消息并获取 AI 回复（Eino DeepAgent，非流式）",
					"description": "与 `POST /api/agent-loop` 请求体相同，但由 **CloudWeGo Eino DeepAgent** 执行多代理编排。**前提**：`multi_agent.enabled: true`（可在设置页或 `config.yaml` 开启）；未启用时返回 404 JSON。请求体支持 `webshellConnectionId`（与单代理 WebShell 助手一致）。",
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
											"description": "要发送的消息（必需）",
										},
										"conversationId": map[string]interface{}{
											"type":        "string",
											"description": "对话 ID（可选，不提供则新建）",
										},
										"role": map[string]interface{}{
											"type":        "string",
											"description": "角色名称（可选）",
										},
										"webshellConnectionId": map[string]interface{}{
											"type":        "string",
											"description": "WebShell 连接 ID（可选，与 agent-loop 行为一致）",
										},
									},
									"required": []string{"message"},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "成功，响应格式同 /api/agent-loop",
						},
						"400": map[string]interface{}{"description": "参数错误"},
						"401": map[string]interface{}{"description": "未授权"},
						"404": map[string]interface{}{"description": "多代理未启用或对话不存在"},
						"500": map[string]interface{}{"description": "执行失败"},
					},
				},
			},
			"/api/multi-agent/stream": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"对话交互"},
					"summary":     "发送消息并获取 AI 回复（Eino DeepAgent，SSE）",
					"description": "与 `POST /api/agent-loop/stream` 类似，事件类型兼容；由 Eino DeepAgent 执行。**前提**：`multi_agent.enabled: true`；路由常注册，未启用时仍返回 200 SSE，流内首条为 `type: error` 后接 `done`。支持 `webshellConnectionId`。",
					"operationId": "sendMessageMultiAgentStream",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"message":            map[string]interface{}{"type": "string"},
										"conversationId":     map[string]interface{}{"type": "string"},
										"role":               map[string]interface{}{"type": "string"},
										"webshellConnectionId": map[string]interface{}{"type": "string"},
									},
									"required": []string{"message"},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "text/event-stream（SSE）",
							"content": map[string]interface{}{
								"text/event-stream": map[string]interface{}{
									"schema": map[string]interface{}{
										"type":        "string",
										"description": "SSE 流",
									},
								},
							},
						},
						"401": map[string]interface{}{"description": "未授权"},
					},
				},
			},
			"/api/agent-loop/cancel": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"对话交互"},
					"summary":     "取消任务",
					"description": "取消正在执行的Agent Loop任务",
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
							"description": "取消请求已提交",
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
												"description": "对话ID",
											},
											"message": map[string]interface{}{
												"type":    "string",
												"example": "已提交取消请求，任务将在当前步骤完成后停止。",
											},
										},
									},
								},
							},
						},
						"404": map[string]interface{}{
							"description": "未找到正在执行的任务",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/agent-loop/tasks": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"对话交互"},
					"summary":     "列出运行中的任务",
					"description": "获取所有正在运行的Agent Loop任务",
					"operationId": "listAgentTasks",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "获取成功",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"tasks": map[string]interface{}{
												"type":        "array",
												"description": "任务列表",
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
							"description": "未授权",
						},
					},
				},
			},
			"/api/agent-loop/tasks/completed": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"对话交互"},
					"summary":     "列出已完成的任务",
					"description": "获取最近完成的Agent Loop任务历史",
					"operationId": "listCompletedTasks",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "获取成功",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"tasks": map[string]interface{}{
												"type":        "array",
												"description": "已完成任务列表",
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
							"description": "未授权",
						},
					},
				},
			},
			"/api/batch-tasks": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"批量任务"},
					"summary":     "创建批量任务队列",
					"description": "创建一个批量任务队列，包含多个任务",
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
							"description": "创建成功",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"queueId": map[string]interface{}{
												"type":        "string",
												"description": "队列ID",
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
							"description": "请求参数错误",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
				"get": map[string]interface{}{
					"tags":        []string{"批量任务"},
					"summary":     "列出批量任务队列",
					"description": "获取所有批量任务队列",
					"operationId": "listBatchQueues",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "获取成功",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"queues": map[string]interface{}{
												"type":        "array",
												"description": "队列列表",
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
							"description": "未授权",
						},
					},
				},
			},
			"/api/batch-tasks/{queueId}": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"批量任务"},
					"summary":     "获取批量任务队列",
					"description": "获取指定批量任务队列的详细信息",
					"operationId": "getBatchQueue",
					"parameters": []map[string]interface{}{
						{
							"name":        "queueId",
							"in":          "path",
							"required":    true,
							"description": "队列ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "获取成功",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/BatchQueue",
									},
								},
							},
						},
						"404": map[string]interface{}{
							"description": "队列不存在",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
				"delete": map[string]interface{}{
					"tags":        []string{"批量任务"},
					"summary":     "删除批量任务队列",
					"description": "删除指定的批量任务队列",
					"operationId": "deleteBatchQueue",
					"parameters": []map[string]interface{}{
						{
							"name":        "queueId",
							"in":          "path",
							"required":    true,
							"description": "队列ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "删除成功",
						},
						"404": map[string]interface{}{
							"description": "队列不存在",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/batch-tasks/{queueId}/start": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"批量任务"},
					"summary":     "启动批量任务队列",
					"description": "开始执行批量任务队列中的任务",
					"operationId": "startBatchQueue",
					"parameters": []map[string]interface{}{
						{
							"name":        "queueId",
							"in":          "path",
							"required":    true,
							"description": "队列ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "启动成功",
						},
						"404": map[string]interface{}{
							"description": "队列不存在",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/batch-tasks/{queueId}/pause": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"批量任务"},
					"summary":     "暂停批量任务队列",
					"description": "暂停正在执行的批量任务队列",
					"operationId": "pauseBatchQueue",
					"parameters": []map[string]interface{}{
						{
							"name":        "queueId",
							"in":          "path",
							"required":    true,
							"description": "队列ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "暂停成功",
						},
						"404": map[string]interface{}{
							"description": "队列不存在",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/batch-tasks/{queueId}/tasks": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"批量任务"},
					"summary":     "添加任务到队列",
					"description": "向批量任务队列添加新任务。任务会添加到队列末尾，按照队列顺序依次执行。每个任务会创建一个独立的对话，支持完整的状态跟踪。\n**任务格式**：\n任务内容是一个字符串，描述要执行的安全测试任务。例如：\n- \"扫描 http://example.com 的SQL注入漏洞\"\n- \"对 192.168.1.1 进行端口扫描\"\n- \"检测 https://target.com 的XSS漏洞\"\n**使用示例**：\n```json\n{\n  \"task\": \"扫描 http://example.com 的SQL注入漏洞\"\n}\n```",
					"operationId": "addBatchTask",
					"parameters": []map[string]interface{}{
						{
							"name":        "queueId",
							"in":          "path",
							"required":    true,
							"description": "队列ID",
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
											"description": "任务内容，描述要执行的安全测试任务（必需）",
											"example":     "扫描 http://example.com 的SQL注入漏洞",
										},
									},
								},
								"examples": map[string]interface{}{
									"sqlInjection": map[string]interface{}{
										"summary":     "SQL注入扫描",
										"description": "扫描目标网站的SQL注入漏洞",
										"value": map[string]interface{}{
											"task": "扫描 http://example.com 的SQL注入漏洞",
										},
									},
									"portScan": map[string]interface{}{
										"summary":     "端口扫描",
										"description": "对目标IP进行端口扫描",
										"value": map[string]interface{}{
											"task": "对 192.168.1.1 进行端口扫描",
										},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "添加成功",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"taskId": map[string]interface{}{
												"type":        "string",
												"description": "新添加的任务ID",
											},
											"message": map[string]interface{}{
												"type":        "string",
												"description": "成功消息",
												"example":     "任务已添加到队列",
											},
										},
									},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "请求参数错误（如task为空）",
						},
						"404": map[string]interface{}{
							"description": "队列不存在",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/batch-tasks/{queueId}/tasks/{taskId}": map[string]interface{}{
				"put": map[string]interface{}{
					"tags":        []string{"批量任务"},
					"summary":     "更新批量任务",
					"description": "更新批量任务队列中的指定任务",
					"operationId": "updateBatchTask",
					"parameters": []map[string]interface{}{
						{
							"name":        "queueId",
							"in":          "path",
							"required":    true,
							"description": "队列ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
						{
							"name":        "taskId",
							"in":          "path",
							"required":    true,
							"description": "任务ID",
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
											"description": "任务内容",
										},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "更新成功",
						},
						"404": map[string]interface{}{
							"description": "任务不存在",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
				"delete": map[string]interface{}{
					"tags":        []string{"批量任务"},
					"summary":     "删除批量任务",
					"description": "从批量任务队列中删除指定任务",
					"operationId": "deleteBatchTask",
					"parameters": []map[string]interface{}{
						{
							"name":        "queueId",
							"in":          "path",
							"required":    true,
							"description": "队列ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
						{
							"name":        "taskId",
							"in":          "path",
							"required":    true,
							"description": "任务ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "删除成功",
						},
						"404": map[string]interface{}{
							"description": "任务不存在",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/groups": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"对话分组"},
					"summary":     "创建分组",
					"description": "创建一个新的对话分组",
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
							"description": "创建成功",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/Group",
									},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "请求参数错误或分组名称已存在",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
				"get": map[string]interface{}{
					"tags":        []string{"对话分组"},
					"summary":     "列出分组",
					"description": "获取所有对话分组",
					"operationId": "listGroups",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "获取成功",
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
							"description": "未授权",
						},
					},
				},
			},
			"/api/groups/{id}": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"对话分组"},
					"summary":     "获取分组",
					"description": "获取指定分组的详细信息",
					"operationId": "getGroup",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "分组ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "获取成功",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/Group",
									},
								},
							},
						},
						"404": map[string]interface{}{
							"description": "分组不存在",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
				"put": map[string]interface{}{
					"tags":        []string{"对话分组"},
					"summary":     "更新分组",
					"description": "更新分组信息",
					"operationId": "updateGroup",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "分组ID",
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
							"description": "更新成功",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/Group",
									},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "请求参数错误或分组名称已存在",
						},
						"404": map[string]interface{}{
							"description": "分组不存在",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
				"delete": map[string]interface{}{
					"tags":        []string{"对话分组"},
					"summary":     "删除分组",
					"description": "删除指定分组",
					"operationId": "deleteGroup",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "分组ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "删除成功",
						},
						"404": map[string]interface{}{
							"description": "分组不存在",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/groups/{id}/conversations": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"对话分组"},
					"summary":     "获取分组中的对话",
					"description": "获取指定分组中的所有对话",
					"operationId": "getGroupConversations",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "分组ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "获取成功",
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
							"description": "分组不存在",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/groups/conversations": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"对话分组"},
					"summary":     "添加对话到分组",
					"description": "将对话添加到指定分组",
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
							"description": "添加成功",
						},
						"400": map[string]interface{}{
							"description": "请求参数错误",
						},
						"404": map[string]interface{}{
							"description": "对话或分组不存在",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/groups/{id}/conversations/{conversationId}": map[string]interface{}{
				"delete": map[string]interface{}{
					"tags":        []string{"对话分组"},
					"summary":     "从分组移除对话",
					"description": "从指定分组中移除对话",
					"operationId": "removeConversationFromGroup",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "分组ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
						{
							"name":        "conversationId",
							"in":          "path",
							"required":    true,
							"description": "对话ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "移除成功",
						},
						"404": map[string]interface{}{
							"description": "对话或分组不存在",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/vulnerabilities": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"漏洞管理"},
					"summary":     "列出漏洞",
					"description": "获取漏洞列表，支持分页和筛选",
					"operationId": "listVulnerabilities",
					"parameters": []map[string]interface{}{
						{
							"name":        "limit",
							"in":          "query",
							"required":    false,
							"description": "每页数量",
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
							"description": "偏移量",
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
							"description": "页码（与offset二选一）",
							"schema": map[string]interface{}{
								"type":    "integer",
								"minimum": 1,
							},
						},
						{
							"name":        "id",
							"in":          "query",
							"required":    false,
							"description": "漏洞ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
						{
							"name":        "conversation_id",
							"in":          "query",
							"required":    false,
							"description": "对话ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
						{
							"name":        "severity",
							"in":          "query",
							"required":    false,
							"description": "严重程度",
							"schema": map[string]interface{}{
								"type": "string",
								"enum": []string{"critical", "high", "medium", "low", "info"},
							},
						},
						{
							"name":        "status",
							"in":          "query",
							"required":    false,
							"description": "状态",
							"schema": map[string]interface{}{
								"type": "string",
								"enum": []string{"open", "closed", "fixed"},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "获取成功",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/ListVulnerabilitiesResponse",
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
				"post": map[string]interface{}{
					"tags":        []string{"漏洞管理"},
					"summary":     "创建漏洞",
					"description": "创建一个新的漏洞记录",
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
							"description": "创建成功",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/Vulnerability",
									},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "请求参数错误",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/vulnerabilities/stats": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"漏洞管理"},
					"summary":     "获取漏洞统计",
					"description": "获取漏洞统计信息",
					"operationId": "getVulnerabilityStats",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "获取成功",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/VulnerabilityStats",
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/vulnerabilities/{id}": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"漏洞管理"},
					"summary":     "获取漏洞",
					"description": "获取指定漏洞的详细信息",
					"operationId": "getVulnerability",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "漏洞ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "获取成功",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/Vulnerability",
									},
								},
							},
						},
						"404": map[string]interface{}{
							"description": "漏洞不存在",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
				"put": map[string]interface{}{
					"tags":        []string{"漏洞管理"},
					"summary":     "更新漏洞",
					"description": "更新漏洞信息",
					"operationId": "updateVulnerability",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "漏洞ID",
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
							"description": "更新成功",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/Vulnerability",
									},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "请求参数错误",
						},
						"404": map[string]interface{}{
							"description": "漏洞不存在",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
				"delete": map[string]interface{}{
					"tags":        []string{"漏洞管理"},
					"summary":     "删除漏洞",
					"description": "删除指定漏洞",
					"operationId": "deleteVulnerability",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "漏洞ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "删除成功",
						},
						"404": map[string]interface{}{
							"description": "漏洞不存在",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/roles": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"角色管理"},
					"summary":     "列出角色",
					"description": "获取所有安全测试角色",
					"operationId": "getRoles",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "获取成功",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"roles": map[string]interface{}{
												"type":        "array",
												"description": "角色列表",
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
							"description": "未授权",
						},
					},
				},
				"post": map[string]interface{}{
					"tags":        []string{"角色管理"},
					"summary":     "创建角色",
					"description": "创建一个新的安全测试角色",
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
							"description": "创建成功",
						},
						"400": map[string]interface{}{
							"description": "请求参数错误",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/roles/{name}": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"角色管理"},
					"summary":     "获取角色",
					"description": "获取指定角色的详细信息",
					"operationId": "getRole",
					"parameters": []map[string]interface{}{
						{
							"name":        "name",
							"in":          "path",
							"required":    true,
							"description": "角色名称",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "获取成功",
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
							"description": "角色不存在",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
				"put": map[string]interface{}{
					"tags":        []string{"角色管理"},
					"summary":     "更新角色",
					"description": "更新指定角色的配置",
					"operationId": "updateRole",
					"parameters": []map[string]interface{}{
						{
							"name":        "name",
							"in":          "path",
							"required":    true,
							"description": "角色名称",
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
							"description": "更新成功",
						},
						"400": map[string]interface{}{
							"description": "请求参数错误",
						},
						"404": map[string]interface{}{
							"description": "角色不存在",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
				"delete": map[string]interface{}{
					"tags":        []string{"角色管理"},
					"summary":     "删除角色",
					"description": "删除指定角色",
					"operationId": "deleteRole",
					"parameters": []map[string]interface{}{
						{
							"name":        "name",
							"in":          "path",
							"required":    true,
							"description": "角色名称",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "删除成功",
						},
						"404": map[string]interface{}{
							"description": "角色不存在",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/roles/skills/list": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"角色管理"},
					"summary":     "获取可用Skills列表",
					"description": "获取所有可用的Skills列表，用于角色配置",
					"operationId": "getSkills",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "获取成功",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"skills": map[string]interface{}{
												"type":        "array",
												"description": "Skills列表",
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
							"description": "未授权",
						},
					},
				},
			},
			"/api/skills": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Skills管理"},
					"summary":     "列出Skills",
					"description": "获取所有Skills列表，支持分页和搜索",
					"operationId": "getSkills",
					"parameters": []map[string]interface{}{
						{
							"name":        "limit",
							"in":          "query",
							"required":    false,
							"description": "每页数量",
							"schema": map[string]interface{}{
								"type":    "integer",
								"default": 20,
							},
						},
						{
							"name":        "offset",
							"in":          "query",
							"required":    false,
							"description": "偏移量",
							"schema": map[string]interface{}{
								"type":    "integer",
								"default": 0,
							},
						},
						{
							"name":        "search",
							"in":          "query",
							"required":    false,
							"description": "搜索关键词",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "获取成功",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"skills": map[string]interface{}{
												"type":        "array",
												"description": "Skills列表",
												"items": map[string]interface{}{
													"$ref": "#/components/schemas/Skill",
												},
											},
											"total": map[string]interface{}{
												"type":        "integer",
												"description": "总数",
											},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
				"post": map[string]interface{}{
					"tags":        []string{"Skills管理"},
					"summary":     "创建Skill",
					"description": "创建一个新的Skill",
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
							"description": "创建成功",
						},
						"400": map[string]interface{}{
							"description": "请求参数错误",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/skills/stats": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Skills管理"},
					"summary":     "获取Skill统计",
					"description": "获取Skill调用统计信息",
					"operationId": "getSkillStats",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "获取成功",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type":        "object",
										"description": "统计信息",
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
				"delete": map[string]interface{}{
					"tags":        []string{"Skills管理"},
					"summary":     "清空Skill统计",
					"description": "清空所有Skill的调用统计",
					"operationId": "clearSkillStats",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "清空成功",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/skills/{name}": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Skills管理"},
					"summary":     "获取Skill",
					"description": "获取指定Skill的详细信息",
					"operationId": "getSkill",
					"parameters": []map[string]interface{}{
						{
							"name":        "name",
							"in":          "path",
							"required":    true,
							"description": "Skill名称",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "获取成功",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/Skill",
									},
								},
							},
						},
						"404": map[string]interface{}{
							"description": "Skill不存在",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
				"put": map[string]interface{}{
					"tags":        []string{"Skills管理"},
					"summary":     "更新Skill",
					"description": "更新指定Skill的信息",
					"operationId": "updateSkill",
					"parameters": []map[string]interface{}{
						{
							"name":        "name",
							"in":          "path",
							"required":    true,
							"description": "Skill名称",
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
							"description": "更新成功",
						},
						"400": map[string]interface{}{
							"description": "请求参数错误",
						},
						"404": map[string]interface{}{
							"description": "Skill不存在",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
				"delete": map[string]interface{}{
					"tags":        []string{"Skills管理"},
					"summary":     "删除Skill",
					"description": "删除指定Skill",
					"operationId": "deleteSkill",
					"parameters": []map[string]interface{}{
						{
							"name":        "name",
							"in":          "path",
							"required":    true,
							"description": "Skill名称",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "删除成功",
						},
						"404": map[string]interface{}{
							"description": "Skill不存在",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/skills/{name}/bound-roles": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Skills管理"},
					"summary":     "获取绑定角色",
					"description": "获取使用指定Skill的所有角色",
					"operationId": "getSkillBoundRoles",
					"parameters": []map[string]interface{}{
						{
							"name":        "name",
							"in":          "path",
							"required":    true,
							"description": "Skill名称",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "获取成功",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"roles": map[string]interface{}{
												"type":        "array",
												"description": "角色列表",
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
							"description": "Skill不存在",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/skills/{name}/stats": map[string]interface{}{
				"delete": map[string]interface{}{
					"tags":        []string{"Skills管理"},
					"summary":     "清空Skill统计",
					"description": "清空指定Skill的调用统计",
					"operationId": "clearSkillStatsByName",
					"parameters": []map[string]interface{}{
						{
							"name":        "name",
							"in":          "path",
							"required":    true,
							"description": "Skill名称",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "清空成功",
						},
						"404": map[string]interface{}{
							"description": "Skill不存在",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/monitor": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"监控"},
					"summary":     "获取监控信息",
					"description": "获取工具执行监控信息，支持分页和筛选",
					"operationId": "monitor",
					"parameters": []map[string]interface{}{
						{
							"name":        "page",
							"in":          "query",
							"required":    false,
							"description": "页码",
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
							"description": "每页数量",
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
							"description": "状态筛选",
							"schema": map[string]interface{}{
								"type": "string",
								"enum": []string{"success", "failed", "running"},
							},
						},
						{
							"name":        "tool",
							"in":          "query",
							"required":    false,
							"description": "工具名称筛选（支持部分匹配）",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "获取成功",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/MonitorResponse",
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/monitor/execution/{id}": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"监控"},
					"summary":     "获取执行记录",
					"description": "获取指定执行记录的详细信息",
					"operationId": "getExecution",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "执行ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "获取成功",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/ToolExecution",
									},
								},
							},
						},
						"404": map[string]interface{}{
							"description": "执行记录不存在",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
				"delete": map[string]interface{}{
					"tags":        []string{"监控"},
					"summary":     "删除执行记录",
					"description": "删除指定的执行记录",
					"operationId": "deleteExecution",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "执行ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "删除成功",
						},
						"404": map[string]interface{}{
							"description": "执行记录不存在",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/monitor/executions": map[string]interface{}{
				"delete": map[string]interface{}{
					"tags":        []string{"监控"},
					"summary":     "批量删除执行记录",
					"description": "批量删除执行记录",
					"operationId": "deleteExecutions",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "删除成功",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/monitor/stats": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"监控"},
					"summary":     "获取统计信息",
					"description": "获取工具执行统计信息",
					"operationId": "getStats",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "获取成功",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type":        "object",
										"description": "统计信息",
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/config": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"配置管理"},
					"summary":     "获取配置",
					"description": "获取系统配置信息",
					"operationId": "getConfig",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "获取成功",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/ConfigResponse",
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
				"put": map[string]interface{}{
					"tags":        []string{"配置管理"},
					"summary":     "更新配置",
					"description": "更新系统配置",
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
							"description": "更新成功",
						},
						"400": map[string]interface{}{
							"description": "请求参数错误",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/config/tools": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"配置管理"},
					"summary":     "获取工具配置",
					"description": "获取所有工具的配置信息",
					"operationId": "getTools",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "获取成功",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type":        "array",
										"description": "工具配置列表",
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/config/apply": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"配置管理"},
					"summary":     "应用配置",
					"description": "应用配置更改",
					"operationId": "applyConfig",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "应用成功",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/external-mcp": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"外部MCP管理"},
					"summary":     "列出外部MCP",
					"description": "获取所有外部MCP配置和状态",
					"operationId": "getExternalMCPs",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "获取成功",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"servers": map[string]interface{}{
												"type":        "object",
												"description": "MCP服务器配置",
												"additionalProperties": map[string]interface{}{
													"$ref": "#/components/schemas/ExternalMCPResponse",
												},
											},
											"stats": map[string]interface{}{
												"type":        "object",
												"description": "统计信息",
											},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/external-mcp/stats": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"外部MCP管理"},
					"summary":     "获取外部MCP统计",
					"description": "获取外部MCP统计信息",
					"operationId": "getExternalMCPStats",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "获取成功",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type":        "object",
										"description": "统计信息",
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/external-mcp/{name}": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"外部MCP管理"},
					"summary":     "获取外部MCP",
					"description": "获取指定外部MCP的配置和状态",
					"operationId": "getExternalMCP",
					"parameters": []map[string]interface{}{
						{
							"name":        "name",
							"in":          "path",
							"required":    true,
							"description": "MCP名称",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "获取成功",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/ExternalMCPResponse",
									},
								},
							},
						},
						"404": map[string]interface{}{
							"description": "MCP不存在",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
				"put": map[string]interface{}{
					"tags":        []string{"外部MCP管理"},
					"summary":     "添加或更新外部MCP",
					"description": "添加新的外部MCP配置或更新现有配置。\n**传输方式**：\n支持两种传输方式：\n**1. stdio（标准输入输出）**：\n```json\n{\n  \"config\": {\n    \"enabled\": true,\n    \"command\": \"node\",\n    \"args\": [\"/path/to/mcp-server.js\"],\n    \"env\": {}\n  }\n}\n```\n**2. sse（Server-Sent Events）**：\n```json\n{\n  \"config\": {\n    \"enabled\": true,\n    \"transport\": \"sse\",\n    \"url\": \"http://127.0.0.1:8082/sse\",\n    \"timeout\": 30\n  }\n}\n```\n**配置参数说明**：\n- `enabled`: 是否启用（boolean，必需）\n- `command`: 命令（stdio模式必需，如：\"node\", \"python\"）\n- `args`: 命令参数数组（stdio模式必需）\n- `env`: 环境变量（object，可选）\n- `transport`: 传输方式（\"stdio\" 或 \"sse\"，sse模式必需）\n- `url`: SSE端点URL（sse模式必需）\n- `timeout`: 超时时间（秒，可选，默认30）\n- `description`: 描述（可选）",
					"operationId": "addOrUpdateExternalMCP",
					"parameters": []map[string]interface{}{
						{
							"name":        "name",
							"in":          "path",
							"required":    true,
							"description": "MCP名称（唯一标识符）",
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
										"summary":     "stdio模式配置",
										"description": "使用标准输入输出方式连接外部MCP服务器",
										"value": map[string]interface{}{
											"config": map[string]interface{}{
												"enabled":     true,
												"command":     "node",
												"args":        []string{"/path/to/mcp-server.js"},
												"env":         map[string]interface{}{},
												"timeout":     30,
												"description": "Node.js MCP服务器",
											},
										},
									},
									"sse": map[string]interface{}{
										"summary":     "SSE模式配置",
										"description": "使用Server-Sent Events方式连接外部MCP服务器",
										"value": map[string]interface{}{
											"config": map[string]interface{}{
												"enabled":     true,
												"transport":   "sse",
												"url":         "http://127.0.0.1:8082/sse",
												"timeout":     30,
												"description": "SSE MCP服务器",
											},
										},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "操作成功",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"message": map[string]interface{}{
												"type":    "string",
												"example": "外部MCP配置已保存",
											},
										},
									},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "请求参数错误（如配置格式不正确、缺少必需字段等）",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/Error",
									},
									"example": map[string]interface{}{
										"error": "stdio模式需要提供command和args参数",
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
				"delete": map[string]interface{}{
					"tags":        []string{"外部MCP管理"},
					"summary":     "删除外部MCP",
					"description": "删除指定的外部MCP配置",
					"operationId": "deleteExternalMCP",
					"parameters": []map[string]interface{}{
						{
							"name":        "name",
							"in":          "path",
							"required":    true,
							"description": "MCP名称",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "删除成功",
						},
						"404": map[string]interface{}{
							"description": "MCP不存在",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/external-mcp/{name}/start": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"外部MCP管理"},
					"summary":     "启动外部MCP",
					"description": "启动指定的外部MCP服务器",
					"operationId": "startExternalMCP",
					"parameters": []map[string]interface{}{
						{
							"name":        "name",
							"in":          "path",
							"required":    true,
							"description": "MCP名称",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "启动成功",
						},
						"404": map[string]interface{}{
							"description": "MCP不存在",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/external-mcp/{name}/stop": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"外部MCP管理"},
					"summary":     "停止外部MCP",
					"description": "停止指定的外部MCP服务器",
					"operationId": "stopExternalMCP",
					"parameters": []map[string]interface{}{
						{
							"name":        "name",
							"in":          "path",
							"required":    true,
							"description": "MCP名称",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "停止成功",
						},
						"404": map[string]interface{}{
							"description": "MCP不存在",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/attack-chain/{conversationId}": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"攻击链"},
					"summary":     "获取攻击链",
					"description": "获取指定对话的攻击链可视化数据",
					"operationId": "getAttackChain",
					"parameters": []map[string]interface{}{
						{
							"name":        "conversationId",
							"in":          "path",
							"required":    true,
							"description": "对话ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "获取成功",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/AttackChain",
									},
								},
							},
						},
						"404": map[string]interface{}{
							"description": "对话不存在",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/attack-chain/{conversationId}/regenerate": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"攻击链"},
					"summary":     "重新生成攻击链",
					"description": "重新生成指定对话的攻击链可视化数据",
					"operationId": "regenerateAttackChain",
					"parameters": []map[string]interface{}{
						{
							"name":        "conversationId",
							"in":          "path",
							"required":    true,
							"description": "对话ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "重新生成成功",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/AttackChain",
									},
								},
							},
						},
						"404": map[string]interface{}{
							"description": "对话不存在",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/conversations/{id}/pinned": map[string]interface{}{
				"put": map[string]interface{}{
					"tags":        []string{"对话管理"},
					"summary":     "设置对话置顶",
					"description": "设置或取消对话的置顶状态",
					"operationId": "updateConversationPinned",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "对话ID",
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
											"description": "是否置顶",
										},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "更新成功",
						},
						"404": map[string]interface{}{
							"description": "对话不存在",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/groups/{id}/pinned": map[string]interface{}{
				"put": map[string]interface{}{
					"tags":        []string{"对话分组"},
					"summary":     "设置分组置顶",
					"description": "设置或取消分组的置顶状态",
					"operationId": "updateGroupPinned",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "分组ID",
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
											"description": "是否置顶",
										},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "更新成功",
						},
						"404": map[string]interface{}{
							"description": "分组不存在",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/groups/{id}/conversations/{conversationId}/pinned": map[string]interface{}{
				"put": map[string]interface{}{
					"tags":        []string{"对话分组"},
					"summary":     "设置分组中对话的置顶",
					"description": "设置或取消分组中对话的置顶状态",
					"operationId": "updateConversationPinnedInGroup",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "分组ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
						{
							"name":        "conversationId",
							"in":          "path",
							"required":    true,
							"description": "对话ID",
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
											"description": "是否置顶",
										},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "更新成功",
						},
						"404": map[string]interface{}{
							"description": "对话或分组不存在",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/knowledge/categories": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"知识库"},
					"summary":     "获取分类",
					"description": "获取知识库的所有分类",
					"operationId": "getKnowledgeCategories",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "获取成功",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"categories": map[string]interface{}{
												"type":        "array",
												"description": "分类列表",
												"items": map[string]interface{}{
													"type": "string",
												},
											},
											"enabled": map[string]interface{}{
												"type":        "boolean",
												"description": "知识库是否启用",
											},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/knowledge/items": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"知识库"},
					"summary":     "列出知识项",
					"description": "获取知识库中的所有知识项",
					"operationId": "getKnowledgeItems",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "获取成功",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"items": map[string]interface{}{
												"type":        "array",
												"description": "知识项列表",
											},
											"enabled": map[string]interface{}{
												"type":        "boolean",
												"description": "知识库是否启用",
											},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
				"post": map[string]interface{}{
					"tags":        []string{"知识库"},
					"summary":     "创建知识项",
					"description": "创建新的知识项",
					"operationId": "createKnowledgeItem",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type":        "object",
									"description": "知识项数据",
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "创建成功",
						},
						"400": map[string]interface{}{
							"description": "请求参数错误",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/knowledge/items/{id}": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"知识库"},
					"summary":     "获取知识项",
					"description": "获取指定知识项的详细信息",
					"operationId": "getKnowledgeItem",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "知识项ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "获取成功",
						},
						"404": map[string]interface{}{
							"description": "知识项不存在",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
				"put": map[string]interface{}{
					"tags":        []string{"知识库"},
					"summary":     "更新知识项",
					"description": "更新指定知识项",
					"operationId": "updateKnowledgeItem",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "知识项ID",
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
									"description": "知识项数据",
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "更新成功",
						},
						"404": map[string]interface{}{
							"description": "知识项不存在",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
				"delete": map[string]interface{}{
					"tags":        []string{"知识库"},
					"summary":     "删除知识项",
					"description": "删除指定知识项",
					"operationId": "deleteKnowledgeItem",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "知识项ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "删除成功",
						},
						"404": map[string]interface{}{
							"description": "知识项不存在",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/knowledge/index-status": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"知识库"},
					"summary":     "获取索引状态",
					"description": "获取知识库索引的构建状态",
					"operationId": "getIndexStatus",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "获取成功",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"enabled": map[string]interface{}{
												"type":        "boolean",
												"description": "知识库是否启用",
											},
											"total_items": map[string]interface{}{
												"type":        "integer",
												"description": "总知识项数",
											},
											"indexed_items": map[string]interface{}{
												"type":        "integer",
												"description": "已索引知识项数",
											},
											"progress_percent": map[string]interface{}{
												"type":        "number",
												"description": "索引进度百分比",
											},
											"is_complete": map[string]interface{}{
												"type":        "boolean",
												"description": "索引是否完成",
											},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/knowledge/index": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"知识库"},
					"summary":     "重建索引",
					"description": "重新构建知识库索引",
					"operationId": "rebuildIndex",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "重建索引任务已启动",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/knowledge/scan": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"知识库"},
					"summary":     "扫描知识库",
					"description": "扫描知识库目录，导入新的知识文件",
					"operationId": "scanKnowledgeBase",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "扫描任务已启动",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/knowledge/search": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"知识库"},
					"summary":     "搜索知识库",
					"description": "在知识库中搜索相关内容。使用向量检索和混合搜索技术，能够根据查询内容的语义相似度和关键词匹配，自动找到最相关的知识片段。\n**搜索说明**：\n- 支持语义相似度搜索（向量检索）\n- 支持关键词匹配（BM25）\n- 支持混合搜索（结合向量和关键词）\n- 可以按风险类型过滤（如：SQL注入、XSS、文件上传等）\n- 建议先调用 `/api/knowledge/categories` 获取可用的风险类型列表\n**使用示例**：\n```json\n{\n  \"query\": \"SQL注入漏洞的检测方法\",\n  \"riskType\": \"SQL注入\",\n  \"topK\": 5,\n  \"threshold\": 0.7\n}\n```",
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
											"description": "搜索查询内容，描述你想要了解的安全知识主题（必需）",
											"example":     "SQL注入漏洞的检测方法",
										},
										"riskType": map[string]interface{}{
											"type":        "string",
											"description": "可选：指定风险类型（如：SQL注入、XSS、文件上传等）。建议先调用 `/api/knowledge/categories` 获取可用的风险类型列表，然后使用正确的风险类型进行精确搜索，这样可以大幅减少检索时间。如果不指定则搜索所有类型。",
											"example":     "SQL注入",
										},
										"topK": map[string]interface{}{
											"type":        "integer",
											"description": "可选：返回Top-K结果数量，默认5",
											"default":     5,
											"minimum":     1,
											"maximum":     50,
											"example":     5,
										},
										"threshold": map[string]interface{}{
											"type":        "number",
											"format":      "float",
											"description": "可选：相似度阈值（0-1之间），默认0.7。只有相似度大于等于此值的结果才会返回",
											"default":     0.7,
											"minimum":     0,
											"maximum":     1,
											"example":     0.7,
										},
									},
								},
								"examples": map[string]interface{}{
									"basic": map[string]interface{}{
										"summary":     "基础搜索",
										"description": "最简单的搜索，只提供查询内容",
										"value": map[string]interface{}{
											"query": "SQL注入漏洞的检测方法",
										},
									},
									"withRiskType": map[string]interface{}{
										"summary":     "按风险类型搜索",
										"description": "指定风险类型进行精确搜索",
										"value": map[string]interface{}{
											"query":     "SQL注入漏洞的检测方法",
											"riskType":  "SQL注入",
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
							"description": "搜索成功",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"results": map[string]interface{}{
												"type":        "array",
												"description": "搜索结果列表，每个结果包含：item（知识项信息）、chunks（匹配的知识片段）、score（相似度分数）",
												"items": map[string]interface{}{
													"type": "object",
													"properties": map[string]interface{}{
														"item": map[string]interface{}{
															"type":        "object",
															"description": "知识项信息",
														},
														"chunks": map[string]interface{}{
															"type":        "array",
															"description": "匹配的知识片段列表",
														},
														"score": map[string]interface{}{
															"type":        "number",
															"description": "相似度分数（0-1之间）",
														},
													},
												},
											},
											"enabled": map[string]interface{}{
												"type":        "boolean",
												"description": "知识库是否启用",
											},
										},
									},
									"example": map[string]interface{}{
										"results": []map[string]interface{}{
											{
												"item": map[string]interface{}{
													"id":       "item-1",
													"title":    "SQL注入漏洞检测",
													"category": "SQL注入",
												},
												"chunks": []map[string]interface{}{
													{
														"text": "SQL注入漏洞的检测方法包括...",
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
							"description": "请求参数错误（如query为空）",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/Error",
									},
									"example": map[string]interface{}{
										"error": "查询不能为空",
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
						"500": map[string]interface{}{
							"description": "服务器内部错误（如知识库未启用或检索失败）",
						},
					},
				},
			},
			"/api/knowledge/retrieval-logs": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"知识库"},
					"summary":     "获取检索日志",
					"description": "获取知识库检索日志",
					"operationId": "getRetrievalLogs",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "获取成功",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"logs": map[string]interface{}{
												"type":        "array",
												"description": "检索日志列表",
											},
											"enabled": map[string]interface{}{
												"type":        "boolean",
												"description": "知识库是否启用",
											},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/knowledge/retrieval-logs/{id}": map[string]interface{}{
				"delete": map[string]interface{}{
					"tags":        []string{"知识库"},
					"summary":     "删除检索日志",
					"description": "删除指定的检索日志",
					"operationId": "deleteRetrievalLog",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "日志ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "删除成功",
						},
						"404": map[string]interface{}{
							"description": "日志不存在",
						},
						"401": map[string]interface{}{
							"description": "未授权",
						},
					},
				},
			},
			"/api/mcp": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"MCP"},
					"summary":     "MCP端点",
					"description": "MCP (Model Context Protocol) 端点，用于处理MCP协议请求。\n**协议说明**：\n本端点遵循 JSON-RPC 2.0 规范，支持以下方法：\n**1. initialize** - 初始化MCP连接\n```json\n{\n  \"jsonrpc\": \"2.0\",\n  \"id\": \"init-1\",\n  \"method\": \"initialize\",\n  \"params\": {\n    \"protocolVersion\": \"2024-11-05\",\n    \"capabilities\": {},\n    \"clientInfo\": {\n      \"name\": \"MyClient\",\n      \"version\": \"1.0.0\"\n    }\n  }\n}\n```\n**2. tools/list** - 列出所有可用工具\n```json\n{\n  \"jsonrpc\": \"2.0\",\n  \"id\": \"list-1\",\n  \"method\": \"tools/list\",\n  \"params\": {}\n}\n```\n**3. tools/call** - 调用工具\n```json\n{\n  \"jsonrpc\": \"2.0\",\n  \"id\": \"call-1\",\n  \"method\": \"tools/call\",\n  \"params\": {\n    \"name\": \"nmap\",\n    \"arguments\": {\n      \"target\": \"192.168.1.1\",\n      \"ports\": \"80,443\"\n    }\n  }\n}\n```\n**4. prompts/list** - 列出所有提示词模板\n```json\n{\n  \"jsonrpc\": \"2.0\",\n  \"id\": \"prompts-list-1\",\n  \"method\": \"prompts/list\",\n  \"params\": {}\n}\n```\n**5. prompts/get** - 获取提示词模板\n```json\n{\n  \"jsonrpc\": \"2.0\",\n  \"id\": \"prompt-get-1\",\n  \"method\": \"prompts/get\",\n  \"params\": {\n    \"name\": \"prompt-name\",\n    \"arguments\": {}\n  }\n}\n```\n**6. resources/list** - 列出所有资源\n```json\n{\n  \"jsonrpc\": \"2.0\",\n  \"id\": \"resources-list-1\",\n  \"method\": \"resources/list\",\n  \"params\": {}\n}\n```\n**7. resources/read** - 读取资源内容\n```json\n{\n  \"jsonrpc\": \"2.0\",\n  \"id\": \"resource-read-1\",\n  \"method\": \"resources/read\",\n  \"params\": {\n    \"uri\": \"resource://example\"\n  }\n}\n```\n**错误代码说明**：\n- `-32700`: Parse error - JSON解析错误\n- `-32600`: Invalid Request - 无效请求\n- `-32601`: Method not found - 方法不存在\n- `-32602`: Invalid params - 参数无效\n- `-32603`: Internal error - 内部错误",
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
										"summary":     "列出所有工具",
										"description": "获取系统中所有可用的MCP工具列表",
										"value": map[string]interface{}{
											"jsonrpc": "2.0",
											"id":      "list-tools-1",
											"method":  "tools/list",
											"params":  map[string]interface{}{},
										},
									},
									"callTool": map[string]interface{}{
										"summary":     "调用工具",
										"description": "调用指定的MCP工具",
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
										"summary":     "初始化连接",
										"description": "初始化MCP连接，获取服务器能力",
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
							"description": "MCP响应（JSON-RPC 2.0格式）",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/MCPResponse",
									},
									"examples": map[string]interface{}{
										"success": map[string]interface{}{
											"summary":     "成功响应",
											"description": "工具调用成功的响应示例",
											"value": map[string]interface{}{
												"jsonrpc": "2.0",
												"id":      "call-tool-1",
												"result": map[string]interface{}{
													"content": []map[string]interface{}{
														{
															"type": "text",
															"text": "工具执行结果...",
														},
													},
													"isError": false,
												},
											},
										},
										"error": map[string]interface{}{
											"summary":     "错误响应",
											"description": "工具调用失败的响应示例",
											"value": map[string]interface{}{
												"jsonrpc": "2.0",
												"id":      "call-tool-1",
												"error": map[string]interface{}{
													"code":    -32601,
													"message": "Tool not found",
													"data":    "工具 'unknown-tool' 不存在",
												},
											},
										},
									},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "请求格式错误（JSON解析失败）",
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
							"description": "未授权，需要有效的Token",
						},
						"405": map[string]interface{}{
							"description": "方法不允许（仅支持POST请求）",
						},
					},
				},
			},
		},
	}

	enrichSpecWithI18nKeys(spec)
	c.JSON(http.StatusOK, spec)
}

// GetConversationResults 获取对话结果（OpenAPI端点）
// 注意：创建对话和获取对话详情直接使用标准的 /api/conversations 端点
// 这个端点只是为了提供结果聚合功能
func (h *OpenAPIHandler) GetConversationResults(c *gin.Context) {
	conversationID := c.Param("id")

	// 验证对话是否存在
	conv, err := h.db.GetConversation(conversationID)
	if err != nil {
		h.logger.Error("获取对话失败", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "对话不存在"})
		return
	}

	// 获取消息列表
	messages, err := h.db.GetMessages(conversationID)
	if err != nil {
		h.logger.Error("获取消息失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 获取漏洞列表
	vulnList, err := h.db.ListVulnerabilities(1000, 0, "", conversationID, "", "")
	if err != nil {
		h.logger.Warn("获取漏洞列表失败", zap.Error(err))
		vulnList = []*database.Vulnerability{}
	}
	vulnerabilities := make([]database.Vulnerability, len(vulnList))
	for i, v := range vulnList {
		vulnerabilities[i] = *v
	}

	// 获取执行结果（从MCP执行记录中获取）
	executionResults := []map[string]interface{}{}
	for _, msg := range messages {
		if len(msg.MCPExecutionIDs) > 0 {
			for _, execID := range msg.MCPExecutionIDs {
				// 尝试从结果存储中获取执行结果
				if h.resultStorage != nil {
					result, err := h.resultStorage.GetResult(execID)
					if err == nil && result != "" {
						// 获取元数据以获取工具名称和创建时间
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
