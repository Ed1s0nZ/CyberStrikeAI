package security

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/mcp"
	"go.uber.org/zap"
)

// Executor 安全工具执行器
type Executor struct {
	config    *config.SecurityConfig
	mcpServer *mcp.Server
	logger    *zap.Logger
}

// NewExecutor 创建新的执行器
func NewExecutor(cfg *config.SecurityConfig, mcpServer *mcp.Server, logger *zap.Logger) *Executor {
	return &Executor{
		config:    cfg,
		mcpServer: mcpServer,
		logger:    logger,
	}
}

// ExecuteTool 执行安全工具
func (e *Executor) ExecuteTool(ctx context.Context, toolName string, args map[string]interface{}) (*mcp.ToolResult, error) {
	e.logger.Info("ExecuteTool被调用",
		zap.String("toolName", toolName),
		zap.Any("args", args),
	)
	
	// 特殊处理：exec工具直接执行系统命令
	if toolName == "exec" {
		e.logger.Info("执行exec工具")
		return e.executeSystemCommand(ctx, args)
	}

	// 查找工具配置
	var toolConfig *config.ToolConfig
	for i := range e.config.Tools {
		if e.config.Tools[i].Name == toolName && e.config.Tools[i].Enabled {
			toolConfig = &e.config.Tools[i]
			break
		}
	}

	if toolConfig == nil {
		e.logger.Error("工具未找到或未启用",
			zap.String("toolName", toolName),
			zap.Int("totalTools", len(e.config.Tools)),
		)
		return nil, fmt.Errorf("工具 %s 未找到或未启用", toolName)
	}
	
	e.logger.Info("找到工具配置",
		zap.String("toolName", toolName),
		zap.String("command", toolConfig.Command),
		zap.Strings("args", toolConfig.Args),
	)

	// 构建命令 - 根据工具类型使用不同的参数格式
	cmdArgs := e.buildCommandArgs(toolName, toolConfig, args)
	
	e.logger.Info("构建命令参数完成",
		zap.String("toolName", toolName),
		zap.Strings("cmdArgs", cmdArgs),
		zap.Int("argsCount", len(cmdArgs)),
	)
	
	// 验证命令参数
	if len(cmdArgs) == 0 {
		e.logger.Warn("命令参数为空",
			zap.String("toolName", toolName),
			zap.Any("inputArgs", args),
		)
		return &mcp.ToolResult{
			Content: []mcp.Content{
				{
					Type: "text",
					Text: fmt.Sprintf("错误: 工具 %s 缺少必需的参数。接收到的参数: %v", toolName, args),
				},
			},
			IsError: true,
		}, nil
	}

	// 执行命令
	cmd := exec.CommandContext(ctx, toolConfig.Command, cmdArgs...)
	
	e.logger.Info("执行安全工具",
		zap.String("tool", toolName),
		zap.Strings("args", cmdArgs),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		e.logger.Error("工具执行失败",
			zap.String("tool", toolName),
			zap.Error(err),
			zap.String("output", string(output)),
		)
		return &mcp.ToolResult{
			Content: []mcp.Content{
				{
					Type: "text",
					Text: fmt.Sprintf("工具执行失败: %v\n输出: %s", err, string(output)),
				},
			},
			IsError: true,
		}, nil
	}

	e.logger.Info("工具执行成功",
		zap.String("tool", toolName),
		zap.String("output", string(output)),
	)

	return &mcp.ToolResult{
		Content: []mcp.Content{
			{
				Type: "text",
				Text: string(output),
			},
		},
		IsError: false,
	}, nil
}

// RegisterTools 注册工具到MCP服务器
func (e *Executor) RegisterTools(mcpServer *mcp.Server) {
	e.logger.Info("开始注册工具",
		zap.Int("totalTools", len(e.config.Tools)),
	)
	
	for i, toolConfig := range e.config.Tools {
		if !toolConfig.Enabled {
			e.logger.Debug("跳过未启用的工具",
				zap.String("tool", toolConfig.Name),
			)
			continue
		}

		// 创建工具配置的副本，避免闭包问题
		toolName := toolConfig.Name
		toolConfigCopy := toolConfig
		
		tool := mcp.Tool{
			Name:        toolConfigCopy.Name,
			Description: toolConfigCopy.Description,
			InputSchema: e.buildInputSchema(&toolConfigCopy),
		}

		handler := func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
			e.logger.Info("工具handler被调用",
				zap.String("toolName", toolName),
				zap.Any("args", args),
			)
			return e.ExecuteTool(ctx, toolName, args)
		}

		mcpServer.RegisterTool(tool, handler)
		e.logger.Info("注册安全工具成功",
			zap.String("tool", toolConfigCopy.Name),
			zap.String("command", toolConfigCopy.Command),
			zap.Int("index", i),
		)
	}
	
	e.logger.Info("工具注册完成",
		zap.Int("registeredCount", len(e.config.Tools)),
	)
}

// buildCommandArgs 构建命令参数
func (e *Executor) buildCommandArgs(toolName string, toolConfig *config.ToolConfig, args map[string]interface{}) []string {
	cmdArgs := make([]string, 0)

	// 如果配置中定义了参数映射，使用配置中的映射规则
	if len(toolConfig.Parameters) > 0 {
		// 先添加固定参数
		cmdArgs = append(cmdArgs, toolConfig.Args...)

		// 按位置参数排序
		positionalParams := make([]config.ParameterConfig, 0)
		flagParams := make([]config.ParameterConfig, 0)

		for _, param := range toolConfig.Parameters {
			if param.Position != nil {
				positionalParams = append(positionalParams, param)
			} else {
				flagParams = append(flagParams, param)
			}
		}

		// 对位置参数按位置排序
		for i := 0; i < len(positionalParams); i++ {
			for _, param := range positionalParams {
				if param.Position != nil && *param.Position == i {
					value := e.getParamValue(args, param)
					if value == nil {
						if param.Required {
							// 必需参数缺失，返回空数组让上层处理错误
							e.logger.Warn("缺少必需的位置参数",
								zap.String("tool", toolName),
								zap.String("param", param.Name),
								zap.Int("position", *param.Position),
							)
							return []string{}
						}
						break
					}
					cmdArgs = append(cmdArgs, e.formatParamValue(param, value))
					break
				}
			}
		}

		// 处理标志参数
		for _, param := range flagParams {
			value := e.getParamValue(args, param)
			if value == nil {
				if param.Required {
					// 必需参数缺失，返回空数组让上层处理错误
					e.logger.Warn("缺少必需的标志参数",
						zap.String("tool", toolName),
						zap.String("param", param.Name),
					)
					return []string{}
				}
				continue
			}

			// 布尔值特殊处理：如果为 false，跳过；如果为 true，只添加标志
			if param.Type == "bool" {
				if boolVal, ok := value.(bool); ok {
					if !boolVal {
						continue // false 时不添加任何参数
					}
					// true 时只添加标志，不添加值
					if param.Flag != "" {
						cmdArgs = append(cmdArgs, param.Flag)
					}
					continue
				}
			}

			format := param.Format
			if format == "" {
				format = "flag" // 默认格式
			}

			switch format {
			case "flag":
				// --flag value 或 -f value
				if param.Flag != "" {
					cmdArgs = append(cmdArgs, param.Flag)
				}
				formattedValue := e.formatParamValue(param, value)
				if formattedValue != "" {
					cmdArgs = append(cmdArgs, formattedValue)
				}
			case "combined":
				// --flag=value 或 -f=value
				if param.Flag != "" {
					cmdArgs = append(cmdArgs, fmt.Sprintf("%s=%s", param.Flag, e.formatParamValue(param, value)))
				} else {
					cmdArgs = append(cmdArgs, e.formatParamValue(param, value))
				}
			case "template":
				// 使用模板字符串
				if param.Template != "" {
					template := param.Template
					template = strings.ReplaceAll(template, "{flag}", param.Flag)
					template = strings.ReplaceAll(template, "{value}", e.formatParamValue(param, value))
					template = strings.ReplaceAll(template, "{name}", param.Name)
					cmdArgs = append(cmdArgs, strings.Fields(template)...)
				} else {
					// 如果没有模板，使用默认格式
					if param.Flag != "" {
						cmdArgs = append(cmdArgs, param.Flag)
					}
					cmdArgs = append(cmdArgs, e.formatParamValue(param, value))
				}
			case "positional":
				// 位置参数（已在上面处理）
				cmdArgs = append(cmdArgs, e.formatParamValue(param, value))
			default:
				// 默认：直接添加值
				cmdArgs = append(cmdArgs, e.formatParamValue(param, value))
			}
		}

		return cmdArgs
	}

	// 向后兼容：如果没有定义参数，使用旧的硬编码逻辑
	switch toolName {
	case "nmap":
		// nmap -sT -sV -sC target [ports]
		// 使用 -sT (TCP连接扫描) 而不是 -sS (SYN扫描)，因为 -sS 需要root权限
		e.logger.Debug("处理nmap参数",
			zap.Any("args", args),
		)
		
		// 尝试多种方式获取target参数
		var target string
		var ok bool
		
		// 方式1: 直接获取target
		if target, ok = args["target"].(string); !ok || target == "" {
			// 方式2: 尝试从tool字段获取（兼容某些格式）
			if toolVal, exists := args["tool"]; exists {
				if toolMap, ok := toolVal.(map[string]interface{}); ok {
					if t, ok := toolMap["target"].(string); ok {
						target = t
					}
				}
			}
		}
		
		if target == "" {
			e.logger.Warn("nmap缺少target参数",
				zap.Any("args", args),
			)
			return cmdArgs // 返回空数组，让上层处理错误
		}
		
		e.logger.Debug("提取到target",
			zap.String("target", target),
		)
		
		// 处理URL格式的目标（提取域名）
		if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
			// 提取域名部分
			target = strings.TrimPrefix(target, "http://")
			target = strings.TrimPrefix(target, "https://")
			// 移除路径部分
			if idx := strings.Index(target, "/"); idx != -1 {
				target = target[:idx]
			}
		}
		
		// 添加扫描选项：-sT (TCP连接扫描，不需要root权限), -sV (版本检测), -sC (默认脚本)
		cmdArgs = append(cmdArgs, "-sT", "-sV", "-sC")
		
		// 添加端口范围（如果指定）
		if ports, ok := args["ports"].(string); ok && ports != "" {
			cmdArgs = append(cmdArgs, "-p", ports)
		}
		
		// 添加目标
		cmdArgs = append(cmdArgs, target)
		
		e.logger.Debug("nmap命令参数构建完成",
			zap.Strings("cmdArgs", cmdArgs),
		)
	case "sqlmap":
		// sqlmap -u url
		if url, ok := args["url"].(string); ok {
			cmdArgs = append(cmdArgs, "-u", url, "--batch", "--level=3", "--risk=2")
		}
	case "nikto":
		// nikto -h target
		if target, ok := args["target"].(string); ok {
			cmdArgs = append(cmdArgs, "-h", target)
		}
	case "dirb":
		// dirb url
		if url, ok := args["url"].(string); ok {
			cmdArgs = append(cmdArgs, url)
		}
	default:
		// 通用处理
		cmdArgs = append(cmdArgs, toolConfig.Args...)
		for key, value := range args {
			if key == "_tool_name" {
				continue
			}
			cmdArgs = append(cmdArgs, fmt.Sprintf("--%s", key))
			if strValue, ok := value.(string); ok {
				cmdArgs = append(cmdArgs, strValue)
			} else {
				cmdArgs = append(cmdArgs, fmt.Sprintf("%v", value))
			}
		}
	}

	return cmdArgs
}

// getParamValue 获取参数值，支持默认值
func (e *Executor) getParamValue(args map[string]interface{}, param config.ParameterConfig) interface{} {
	// 从参数中获取值
	if value, ok := args[param.Name]; ok && value != nil {
		return value
	}

	// 如果参数是必需的但没有提供，返回 nil（让上层处理错误）
	if param.Required {
		return nil
	}

	// 返回默认值
	return param.Default
}

// formatParamValue 格式化参数值
func (e *Executor) formatParamValue(param config.ParameterConfig, value interface{}) string {
	switch param.Type {
	case "bool":
		// 布尔值应该在上层处理，这里不应该被调用
		if boolVal, ok := value.(bool); ok {
			return fmt.Sprintf("%v", boolVal)
		}
		return "false"
	case "array":
		// 数组：转换为逗号分隔的字符串
		if arr, ok := value.([]interface{}); ok {
			strs := make([]string, 0, len(arr))
			for _, item := range arr {
				strs = append(strs, fmt.Sprintf("%v", item))
			}
			return strings.Join(strs, ",")
		}
		return fmt.Sprintf("%v", value)
	default:
		return fmt.Sprintf("%v", value)
	}
}

// executeSystemCommand 执行系统命令
func (e *Executor) executeSystemCommand(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
	// 获取命令
	command, ok := args["command"].(string)
	if !ok {
		return &mcp.ToolResult{
			Content: []mcp.Content{
				{
					Type: "text",
					Text: "错误: 缺少command参数",
				},
			},
			IsError: true,
		}, nil
	}

	if command == "" {
		return &mcp.ToolResult{
			Content: []mcp.Content{
				{
					Type: "text",
					Text: "错误: command参数不能为空",
				},
			},
			IsError: true,
		}, nil
	}

	// 安全检查：记录执行的命令
	e.logger.Warn("执行系统命令",
		zap.String("command", command),
	)

	// 获取shell类型（可选，默认为sh）
	shell := "sh"
	if s, ok := args["shell"].(string); ok && s != "" {
		shell = s
	}

	// 获取工作目录（可选）
	workDir := ""
	if wd, ok := args["workdir"].(string); ok && wd != "" {
		workDir = wd
	}

	// 构建命令
	var cmd *exec.Cmd
	if workDir != "" {
		cmd = exec.CommandContext(ctx, shell, "-c", command)
		cmd.Dir = workDir
	} else {
		cmd = exec.CommandContext(ctx, shell, "-c", command)
	}

	// 执行命令
	e.logger.Info("执行系统命令",
		zap.String("command", command),
		zap.String("shell", shell),
		zap.String("workdir", workDir),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		e.logger.Error("系统命令执行失败",
			zap.String("command", command),
			zap.Error(err),
			zap.String("output", string(output)),
		)
		return &mcp.ToolResult{
			Content: []mcp.Content{
				{
					Type: "text",
					Text: fmt.Sprintf("命令执行失败: %v\n输出: %s", err, string(output)),
				},
			},
			IsError: true,
		}, nil
	}

	e.logger.Info("系统命令执行成功",
		zap.String("command", command),
		zap.String("output_length", fmt.Sprintf("%d", len(output))),
	)

	return &mcp.ToolResult{
		Content: []mcp.Content{
			{
				Type: "text",
				Text: string(output),
			},
		},
		IsError: false,
	}, nil
}

// buildInputSchema 构建输入模式
func (e *Executor) buildInputSchema(toolConfig *config.ToolConfig) map[string]interface{} {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{},
		"required": []string{},
	}

	// 如果配置中定义了参数，优先使用配置中的参数定义
	if len(toolConfig.Parameters) > 0 {
		properties := make(map[string]interface{})
		required := []string{}

		for _, param := range toolConfig.Parameters {
			prop := map[string]interface{}{
				"type":        param.Type,
				"description": param.Description,
			}

			// 添加默认值
			if param.Default != nil {
				prop["default"] = param.Default
			}

			// 添加枚举选项
			if len(param.Options) > 0 {
				prop["enum"] = param.Options
			}

			properties[param.Name] = prop

			// 添加到必需参数列表
			if param.Required {
				required = append(required, param.Name)
			}
		}

		schema["properties"] = properties
		schema["required"] = required
		return schema
	}

	// 向后兼容：如果没有定义参数，使用旧的硬编码逻辑
	switch toolConfig.Name {
	case "nmap":
		schema["properties"] = map[string]interface{}{
			"target": map[string]interface{}{
				"type":        "string",
				"description": "目标IP地址或域名",
			},
			"ports": map[string]interface{}{
				"type":        "string",
				"description": "端口范围，例如: 1-1000",
			},
		}
		schema["required"] = []string{"target"}
	case "sqlmap":
		schema["properties"] = map[string]interface{}{
			"url": map[string]interface{}{
				"type":        "string",
				"description": "目标URL",
			},
		}
		schema["required"] = []string{"url"}
	case "nikto", "dirb":
		schema["properties"] = map[string]interface{}{
			"target": map[string]interface{}{
				"type":        "string",
				"description": "目标URL",
			},
		}
		schema["required"] = []string{"target"}
	case "exec":
		schema["properties"] = map[string]interface{}{
			"command": map[string]interface{}{
				"type":        "string",
				"description": "要执行的系统命令",
			},
			"shell": map[string]interface{}{
				"type":        "string",
				"description": "使用的shell（可选，默认为sh）",
			},
			"workdir": map[string]interface{}{
				"type":        "string",
				"description": "工作目录（可选）",
			},
		}
		schema["required"] = []string{"command"}
	}

	return schema
}

// Vulnerability 漏洞信息
type Vulnerability struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Severity    string    `json:"severity"` // low, medium, high, critical
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Target      string    `json:"target"`
	FoundAt     time.Time `json:"foundAt"`
	Details     string    `json:"details"`
}

// AnalyzeResults 分析工具执行结果，提取漏洞信息
func (e *Executor) AnalyzeResults(toolName string, result *mcp.ToolResult) []Vulnerability {
	vulnerabilities := []Vulnerability{}

	if result.IsError {
		return vulnerabilities
	}

	// 分析输出内容
	for _, content := range result.Content {
		if content.Type == "text" {
			vulns := e.parseToolOutput(toolName, content.Text)
			vulnerabilities = append(vulnerabilities, vulns...)
		}
	}

	return vulnerabilities
}

// parseToolOutput 解析工具输出
func (e *Executor) parseToolOutput(toolName, output string) []Vulnerability {
	vulnerabilities := []Vulnerability{}

	// 简单的漏洞检测逻辑
	outputLower := strings.ToLower(output)

	// SQL注入检测
	if strings.Contains(outputLower, "sql injection") || strings.Contains(outputLower, "sqli") {
		vulnerabilities = append(vulnerabilities, Vulnerability{
			ID:          fmt.Sprintf("sql-%d", time.Now().Unix()),
			Type:        "SQL Injection",
			Severity:    "high",
			Title:       "SQL注入漏洞",
			Description: "检测到潜在的SQL注入漏洞",
			FoundAt:     time.Now(),
			Details:     output,
		})
	}

	// XSS检测
	if strings.Contains(outputLower, "xss") || strings.Contains(outputLower, "cross-site scripting") {
		vulnerabilities = append(vulnerabilities, Vulnerability{
			ID:          fmt.Sprintf("xss-%d", time.Now().Unix()),
			Type:        "XSS",
			Severity:  "medium",
			Title:       "跨站脚本攻击漏洞",
			Description: "检测到潜在的XSS漏洞",
			FoundAt:     time.Now(),
			Details:     output,
		})
	}

	// 开放端口检测
	if toolName == "nmap" {
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			if strings.Contains(line, "open") && strings.Contains(line, "port") {
				vulnerabilities = append(vulnerabilities, Vulnerability{
					ID:          fmt.Sprintf("port-%d", time.Now().Unix()),
					Type:        "Open Port",
					Severity:    "low",
					Title:       "开放端口",
					Description: fmt.Sprintf("发现开放端口: %s", line),
					FoundAt:     time.Now(),
					Details:     line,
				})
			}
		}
	}

	return vulnerabilities
}

// GetVulnerabilityReport 生成漏洞报告
func (e *Executor) GetVulnerabilityReport(vulnerabilities []Vulnerability) map[string]interface{} {
	severityCount := map[string]int{
		"critical": 0,
		"high":     0,
		"medium":   0,
		"low":      0,
	}

	for _, vuln := range vulnerabilities {
		severityCount[vuln.Severity]++
	}

	return map[string]interface{}{
		"total":           len(vulnerabilities),
		"severityCount":   severityCount,
		"vulnerabilities": vulnerabilities,
		"generatedAt":     time.Now(),
	}
}

