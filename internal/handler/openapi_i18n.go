package handler

// apiDocI18n 为 OpenAPI 文档提供 x-i18n-* 扩展键，供前端 apiDocs 国际化使用。
// 前端通过 apiDocs.tags.* / apiDocs.summary.* / apiDocs.response.* 翻译。

var apiDocI18nTagToKey = map[string]string{
	"认证": "auth", "对话管理": "conversationManagement", "对话交互": "conversationInteraction",
	"批量任务": "batchTasks", "对话分组": "conversationGroups", "漏洞管理": "vulnerabilityManagement",
	"角色管理": "roleManagement", "Skills管理": "skillsManagement", "监控": "monitoring",
	"配置管理": "configManagement", "外部MCP管理": "externalMCPManagement", "攻击链": "attackChain",
	"知识库": "knowledgeBase", "MCP": "mcp",
	"FOFA信息收集": "fofaRecon", "终端": "terminal", "WebShell管理": "webshellManagement",
	"对话附件": "chatUploads", "机器人集成": "robotIntegration", "多代理Markdown": "markdownAgents",
	"项目管理": "projectManagement", "资产管理": "assetManagement",
}

var apiDocI18nSummaryToKey = map[string]string{
	"用户登录": "login", "用户登出": "logout", "修改密码": "changePassword", "验证Token": "validateToken",
	"创建对话": "createConversation", "列出对话": "listConversations", "查看对话详情": "getConversationDetail",
	"更新对话": "updateConversation", "删除对话": "deleteConversation", "获取对话结果": "getConversationResult",
	"发送消息并获取AI回复（非流式）": "sendMessageNonStream", "发送消息并获取AI回复（流式）": "sendMessageStream",
	"取消任务": "cancelTask", "列出运行中的任务": "listRunningTasks", "列出已完成的任务": "listCompletedTasks",
	"创建批量任务队列": "createBatchQueue", "列出批量任务队列": "listBatchQueues", "获取批量任务队列": "getBatchQueue",
	"删除批量任务队列": "deleteBatchQueue", "启动批量任务队列": "startBatchQueue", "暂停批量任务队列": "pauseBatchQueue",
	"添加任务到队列": "addTaskToQueue", "SQL注入扫描": "sqlInjectionScan", "端口扫描": "portScan",
	"更新批量任务": "updateBatchTask", "删除批量任务": "deleteBatchTask",
	"创建分组": "createGroup", "列出分组": "listGroups", "获取分组": "getGroup", "更新分组": "updateGroup",
	"删除分组": "deleteGroup", "获取分组中的对话": "getGroupConversations", "添加对话到分组": "addConversationToGroup",
	"从分组移除对话": "removeConversationFromGroup",
	"列出漏洞":    "listVulnerabilities", "创建漏洞": "createVulnerability", "获取漏洞统计": "getVulnerabilityStats",
	"获取漏洞": "getVulnerability", "更新漏洞": "updateVulnerability", "删除漏洞": "deleteVulnerability",
	"列出角色": "listRoles", "创建角色": "createRole", "获取角色": "getRole", "更新角色": "updateRole", "删除角色": "deleteRole",
	"获取可用Skills列表": "getAvailableSkills", "列出Skills": "listSkills", "创建Skill": "createSkill",
	"获取Skill统计": "getSkillStats", "清空Skill统计": "clearSkillStats", "获取Skill": "getSkill",
	"更新Skill": "updateSkill", "删除Skill": "deleteSkill", "获取绑定角色": "getBoundRoles",
	"获取监控信息": "getMonitorInfo", "获取执行记录": "getExecutionRecords", "删除执行记录": "deleteExecutionRecord",
	"批量删除执行记录": "batchDeleteExecutionRecords", "获取统计信息": "getStats",
	"获取配置": "getConfig", "更新配置": "updateConfig", "获取工具配置": "getToolConfig", "应用配置": "applyConfig",
	"列出外部MCP": "listExternalMCP", "获取外部MCP统计": "getExternalMCPStats", "获取外部MCP": "getExternalMCP",
	"添加或更新外部MCP": "addOrUpdateExternalMCP", "stdio模式配置": "stdioModeConfig", "SSE模式配置": "sseModeConfig",
	"删除外部MCP": "deleteExternalMCP", "启动外部MCP": "startExternalMCP", "停止外部MCP": "stopExternalMCP",
	"获取攻击链": "getAttackChain", "重新生成攻击链": "regenerateAttackChain",
	"设置对话置顶": "pinConversation", "设置分组置顶": "pinGroup", "设置分组中对话的置顶": "pinGroupConversation",
	"获取分类": "getCategories", "列出知识项": "listKnowledgeItems", "创建知识项": "createKnowledgeItem",
	"获取知识项": "getKnowledgeItem", "更新知识项": "updateKnowledgeItem", "删除知识项": "deleteKnowledgeItem",
	"获取索引状态": "getIndexStatus", "构建索引": "startKnowledgeIndex", "扫描知识库": "scanKnowledgeBase",
	"搜索知识库": "searchKnowledgeBase", "基础搜索": "basicSearch", "按风险类型搜索": "searchByRiskType",
	"获取检索日志": "getRetrievalLogs", "删除检索日志": "deleteRetrievalLog",
	"MCP端点": "mcpEndpoint", "列出所有工具": "listAllTools", "调用工具": "invokeTool", "初始化连接": "initConnection",
	"成功响应": "successResponse", "错误响应": "errorResponse",
	// 新增缺失端点
	"删除对话轮次": "deleteConversationTurn", "获取消息过程详情": "getMessageProcessDetails",
	"重跑批量任务队列": "rerunBatchQueue", "修改队列元数据": "updateBatchQueueMetadata",
	"修改队列调度配置": "updateBatchQueueSchedule", "开关Cron自动调度": "setBatchQueueScheduleEnabled",
	"获取所有分组映射": "getAllGroupMappings",
	"FOFA搜索":   "fofaSearch", "自然语言解析为FOFA语法": "fofaParse",
	"测试OpenAI API连接": "testOpenAI",
	"执行终端命令":         "terminalRun", "流式执行终端命令": "terminalRunStream", "WebSocket终端": "terminalWS",
	"列出WebShell连接": "listWebshellConnections", "创建WebShell连接": "createWebshellConnection",
	"更新WebShell连接": "updateWebshellConnection", "删除WebShell连接": "deleteWebshellConnection",
	"获取连接状态": "getWebshellConnectionState", "保存连接状态": "saveWebshellConnectionState",
	"获取AI对话历史": "getWebshellAIHistory", "列出AI对话": "listWebshellAIConversations",
	"执行WebShell命令": "webshellExec", "WebShell文件操作": "webshellFileOp",
	"列出附件": "listChatUploads", "导出附件": "exportChatUploads", "上传附件": "uploadChatFile", "删除附件": "deleteChatUpload",
	"下载附件": "downloadChatUpload", "获取附件文本内容": "getChatUploadContent",
	"写入附件文本内容": "putChatUploadContent", "创建附件目录": "mkdirChatUpload", "重命名附件": "renameChatUpload",
	"企业微信回调验证": "wecomCallbackVerify", "企业微信消息回调": "wecomCallbackMessage",
	"钉钉消息回调": "dingtalkCallback", "飞书消息回调": "larkCallback", "测试机器人消息处理": "testRobot",
	"列出Markdown代理": "listMarkdownAgents", "创建Markdown代理": "createMarkdownAgent",
	"获取Markdown代理详情": "getMarkdownAgent", "更新Markdown代理": "updateMarkdownAgent", "删除Markdown代理": "deleteMarkdownAgent",
	"列出技能包文件": "listSkillPackageFiles", "获取技能包文件内容": "getSkillPackageFile", "写入技能包文件": "putSkillPackageFile",
	"批量获取工具名称": "batchGetToolNames",
	"获取知识库统计":  "getKnowledgeStats",
	"列出项目":     "listProjects", "创建项目": "createProject", "获取项目": "getProject",
	"更新项目": "updateProject", "删除项目": "deleteProject",
	"批量导入资产":        "importAssets",
	"列出或按 key 获取事实": "listProjectFacts", "创建/更新事实": "upsertProjectFact",
	"获取项目事实攻击路径图": "getProjectFactGraph", "列出项目全部事实边": "listProjectFactEdges",
	"添加事实边": "createProjectFactEdge", "删除事实边": "deleteProjectFactEdge",
	"将对话攻击链沉淀到项目事实图": "promoteAttackChainToProject",
}

var apiDocI18nResponseDescToKey = map[string]string{
	"获取成功": "getSuccess", "未授权": "unauthorized", "未授权，需要有效的Token": "unauthorizedToken",
	"创建成功": "createSuccess", "请求参数错误": "badRequest", "对话不存在": "conversationNotFound",
	"对话不存在或结果不存在": "conversationOrResultNotFound", "请求参数错误（如task为空）": "badRequestTaskEmpty",
	"请求参数错误或分组名称已存在": "badRequestGroupNameExists", "分组不存在": "groupNotFound",
	"请求参数错误（如配置格式不正确、缺少必需字段等）": "badRequestConfig",
	"请求参数错误（如query为空）":         "badRequestQueryEmpty", "方法不允许（仅支持POST请求）": "methodNotAllowed",
	"登录成功": "loginSuccess", "密码错误": "invalidPassword", "登出成功": "logoutSuccess",
	"密码修改成功": "passwordChanged", "Token有效": "tokenValid", "Token无效或已过期": "tokenInvalid",
	"对话创建成功": "conversationCreated", "服务器内部错误": "internalError", "更新成功": "updateSuccess",
	"删除成功": "deleteSuccess", "队列不存在": "queueNotFound", "启动成功": "startSuccess",
	"暂停成功": "pauseSuccess", "添加成功": "addSuccess",
	"任务不存在": "taskNotFound", "对话或分组不存在": "conversationOrGroupNotFound",
	"取消请求已提交": "cancelSubmitted", "未找到正在执行的任务": "noRunningTask",
	"消息发送成功，返回AI回复": "messageSent", "流式响应（Server-Sent Events）": "streamResponse",
	// 新增缺失端点响应
	"参数错误或删除失败": "badRequestOrDeleteFailed",
	"参数错误":      "paramError", "仅已完成或已取消的队列可以重跑": "onlyCompletedOrCancelledCanRerun",
	"参数错误或队列正在运行中": "badRequestOrQueueRunning", "设置成功": "setSuccess",
	"搜索成功": "searchSuccess", "解析成功": "parseSuccess", "测试结果": "testResult",
	"执行完成": "executionDone", "SSE事件流": "sseEventStream", "WebSocket连接已建立": "wsEstablished",
	"文件下载": "fileDownload", "文件不存在": "fileNotFound", "写入成功": "writeSuccess",
	"重命名成功": "renameSuccess", "验证成功，返回解密后的echostr": "wecomVerifySuccess",
	"处理成功": "processSuccess", "代理不存在": "agentNotFound", "保存成功": "saveSuccess",
	"操作结果": "operationResult", "执行结果": "executionResult", "连接不存在": "connectionNotFound",
	"项目列表": "projectList", "项目详情": "projectDetail",
	"事实列表或单条（可含 link_counts / outgoing_links）": "projectFactList",
	"成功": "success", "nodes + edges": "factGraphNodesEdges",
	"边列表": "edgeList", "边已创建": "edgeCreated",
	"沉淀结果（facts/edges/graph）": "promoteAttackChainResult",
	"导入完成":                    "assetImportCompleted", "数量或资产字段校验失败": "assetImportValidationFailed",
	"缺少 asset:write 权限或无权访问指定项目": "assetImportForbidden",
	"导入事务失败": "assetImportTransactionFailed",
}

// enrichSpecWithI18nKeys 在 spec 的每个 operation 上写入 x-i18n-tags、x-i18n-summary，
// 在每个 response 上写入 x-i18n-description，供前端按 key 做国际化。
func enrichSpecWithI18nKeys(spec map[string]interface{}) {
	paths, _ := spec["paths"].(map[string]interface{})
	if paths == nil {
		return
	}
	for _, pathItem := range paths {
		pm, _ := pathItem.(map[string]interface{})
		if pm == nil {
			continue
		}
		for _, method := range []string{"get", "post", "put", "delete", "patch"} {
			opVal, ok := pm[method]
			if !ok {
				continue
			}
			op, _ := opVal.(map[string]interface{})
			if op == nil {
				continue
			}
			// x-i18n-tags: 与 tags 一一对应的 i18n 键数组（spec 中 tags 为 []string）
			switch tags := op["tags"].(type) {
			case []string:
				if len(tags) > 0 {
					keys := make([]string, 0, len(tags))
					for _, s := range tags {
						if k := apiDocI18nTagToKey[s]; k != "" {
							keys = append(keys, k)
						} else {
							keys = append(keys, s)
						}
					}
					op["x-i18n-tags"] = keys
				}
			case []interface{}:
				if len(tags) > 0 {
					keys := make([]interface{}, 0, len(tags))
					for _, t := range tags {
						if s, ok := t.(string); ok {
							if k := apiDocI18nTagToKey[s]; k != "" {
								keys = append(keys, k)
							} else {
								keys = append(keys, s)
							}
						}
					}
					if len(keys) > 0 {
						op["x-i18n-tags"] = keys
					}
				}
			}
			// x-i18n-summary
			if summary, _ := op["summary"].(string); summary != "" {
				if k := apiDocI18nSummaryToKey[summary]; k != "" {
					op["x-i18n-summary"] = k
				}
			}
			// responses -> 每个 status -> x-i18n-description
			if respMap, _ := op["responses"].(map[string]interface{}); respMap != nil {
				for _, rv := range respMap {
					if r, _ := rv.(map[string]interface{}); r != nil {
						if desc, _ := r["description"].(string); desc != "" {
							if k := apiDocI18nResponseDescToKey[desc]; k != "" {
								r["x-i18n-description"] = k
							}
						}
					}
				}
			}
		}
	}
}
