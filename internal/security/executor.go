package security

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"

	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/mcp"
	"cyberstrike-ai/internal/storage"

	"go.uber.org/zap"
)

// Executor 安全工具执行器
type Executor struct {
	config        *config.SecurityConfig
	toolIndex     map[string]*config.ToolConfig // 工具索引，用于 O(1) 查找
	mcpServer     *mcp.Server
	logger        *zap.Logger
	resultStorage ResultStorage // 结果存储（用于查询工具）
}

// ResultStorage 结果存储接口（直接使用 storage 包的类型）
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

// NewExecutor 创建新的执行器
func NewExecutor(cfg *config.SecurityConfig, mcpServer *mcp.Server, logger *zap.Logger) *Executor {
	executor := &Executor{
		config:        cfg,
		toolIndex:     make(map[string]*config.ToolConfig),
		mcpServer:     mcpServer,
		logger:        logger,
		resultStorage: nil, // 稍后通过 SetResultStorage 设置
	}
	// 构建工具索引
	executor.buildToolIndex()
	return executor
}

// SetResultStorage 设置结果存储
func (e *Executor) SetResultStorage(storage ResultStorage) {
	e.resultStorage = storage
}

// buildToolIndex 构建工具索引，将 O(n) 查找优化为 O(1)
func (e *Executor) buildToolIndex() {
	e.toolIndex = make(map[string]*config.ToolConfig)
	for i := range e.config.Tools {
		if e.config.Tools[i].Enabled {
			e.toolIndex[e.config.Tools[i].Name] = &e.config.Tools[i]
		}
	}
	e.logger.Info("工具索引构建完成",
		zap.Int("totalTools", len(e.config.Tools)),
		zap.Int("enabledTools", len(e.toolIndex)),
	)
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

	// 使用索引查找工具配置（O(1) 查找）
	toolConfig, exists := e.toolIndex[toolName]
	if !exists {
		e.logger.Error("工具未找到或未启用",
			zap.String("toolName", toolName),
			zap.Int("totalTools", len(e.config.Tools)),
			zap.Int("enabledTools", len(e.toolIndex)),
		)
		return nil, fmt.Errorf("工具 %s 未找到或未启用", toolName)
	}

	e.logger.Info("找到工具配置",
		zap.String("toolName", toolName),
		zap.String("command", toolConfig.Command),
		zap.Strings("args", toolConfig.Args),
	)

	// 特殊处理：内部工具（command 以 "internal:" 开头）
	if strings.HasPrefix(toolConfig.Command, "internal:") {
		e.logger.Info("执行内部工具",
			zap.String("toolName", toolName),
			zap.String("command", toolConfig.Command),
		)
		return e.executeInternalTool(ctx, toolName, toolConfig.Command, args)
	}

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
		// 检查退出码是否在允许列表中
		exitCode := getExitCode(err)
		if exitCode != nil && toolConfig.AllowedExitCodes != nil {
			for _, allowedCode := range toolConfig.AllowedExitCodes {
				if *exitCode == allowedCode {
					e.logger.Info("工具执行完成（退出码在允许列表中）",
						zap.String("tool", toolName),
						zap.Int("exitCode", *exitCode),
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
			}
		}

		e.logger.Error("工具执行失败",
			zap.String("tool", toolName),
			zap.Error(err),
			zap.Int("exitCode", getExitCodeValue(err)),
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
		zap.Int("enabledTools", len(e.toolIndex)),
	)

	// 重新构建索引（以防配置更新）
	e.buildToolIndex()

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

		// 根据配置决定暴露给 AI/API 的描述：short_description 或 description
		useFullDescription := strings.TrimSpace(strings.ToLower(e.config.ToolDescriptionMode)) == "full"
		shortDesc := toolConfigCopy.ShortDescription
		if shortDesc == "" {
			// 如果没有简短描述，从详细描述中提取第一行或前10000个字符
			desc := toolConfigCopy.Description
			if len(desc) > 10000 {
				if idx := strings.Index(desc, "\n"); idx > 0 && idx < 10000 {
					shortDesc = strings.TrimSpace(desc[:idx])
				} else {
					shortDesc = desc[:10000] + "..."
				}
			} else {
				shortDesc = desc
			}
		}
		if useFullDescription {
			shortDesc = "" // 使用 description 时清空 ShortDescription，下游会回退到 Description
		}

		tool := mcp.Tool{
			Name:             toolConfigCopy.Name,
			Description:      toolConfigCopy.Description,
			ShortDescription: shortDesc,
			InputSchema:      e.buildInputSchema(&toolConfigCopy),
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
		// 检查是否有 scan_type 参数，如果有则替换默认的扫描类型参数
		hasScanType := false
		var scanTypeValue string
		if scanType, ok := args["scan_type"].(string); ok && scanType != "" {
			hasScanType = true
			scanTypeValue = scanType
		}

		// 添加固定参数（如果指定了 scan_type，可能需要过滤掉默认的扫描类型参数）
		if hasScanType && toolName == "nmap" {
			// 对于 nmap，如果指定了 scan_type，跳过默认的 -sT -sV -sC
			// 这些参数会被 scan_type 参数替换
		} else {
			cmdArgs = append(cmdArgs, toolConfig.Args...)
		}

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

		// 对于需要子命令的工具（如 gobuster dir），position 0 必须紧跟在命令名后、所有 flag 之前
		for _, param := range positionalParams {
			if param.Name == "additional_args" || param.Name == "scan_type" || param.Name == "action" {
				continue
			}
			if param.Position != nil && *param.Position == 0 {
				value := e.getParamValue(args, param)
				if value == nil && param.Default != nil {
					value = param.Default
				}
				if value != nil {
					cmdArgs = append(cmdArgs, e.formatParamValue(param, value))
				}
				break
			}
		}

		// 处理标志参数
		for _, param := range flagParams {
			// 跳过特殊参数，它们会在后面单独处理
			// action 参数仅用于工具内部逻辑，不传递给命令
			if param.Name == "additional_args" || param.Name == "scan_type" || param.Name == "action" {
				continue
			}

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
				var boolVal bool
				var ok bool

				// 尝试多种类型转换
				if boolVal, ok = value.(bool); ok {
					// 已经是布尔值
				} else if numVal, ok := value.(float64); ok {
					// JSON 数字类型（float64）
					boolVal = numVal != 0
					ok = true
				} else if numVal, ok := value.(int); ok {
					// int 类型
					boolVal = numVal != 0
					ok = true
				} else if strVal, ok := value.(string); ok {
					// 字符串类型
					boolVal = strVal == "true" || strVal == "1" || strVal == "yes"
					ok = true
				}

				if ok {
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

		// 然后处理位置参数（位置参数通常在标志参数之后）
		// 对位置参数按位置排序
		// 首先找到最大的位置值，确定需要处理多少个位置
		maxPosition := -1
		for _, param := range positionalParams {
			if param.Position != nil && *param.Position > maxPosition {
				maxPosition = *param.Position
			}
		}

		// 按位置顺序处理参数，确保即使某些位置没有参数或使用默认值，也能正确传递
		// position 0 已在前面插入（子命令优先），此处从 1 开始
		for i := 0; i <= maxPosition; i++ {
			if i == 0 {
				continue
			}
			for _, param := range positionalParams {
				// 跳过特殊参数，它们会在后面单独处理
				// action 参数仅用于工具内部逻辑，不传递给命令
				if param.Name == "additional_args" || param.Name == "scan_type" || param.Name == "action" {
					continue
				}

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
						// 对于非必需参数，如果值为 nil，尝试使用默认值
						if param.Default != nil {
							value = param.Default
						} else {
							// 如果没有默认值，跳过这个位置，继续处理下一个位置
							break
						}
					}
					// 只有当值不为 nil 时才添加到命令参数中
					if value != nil {
						cmdArgs = append(cmdArgs, e.formatParamValue(param, value))
					}
					break
				}
			}
			// 如果某个位置没有找到对应的参数，继续处理下一个位置
			// 这样可以确保位置参数的顺序正确
		}

		// 特殊处理：additional_args 参数（需要按空格分割成多个参数）
		if additionalArgs, ok := args["additional_args"].(string); ok && additionalArgs != "" {
			// 按空格分割，但保留引号内的内容
			additionalArgsList := e.parseAdditionalArgs(additionalArgs)
			cmdArgs = append(cmdArgs, additionalArgsList...)
		}

		// 特殊处理：scan_type 参数（需要按空格分割并插入到合适位置）
		if hasScanType {
			scanTypeArgs := e.parseAdditionalArgs(scanTypeValue)
			if len(scanTypeArgs) > 0 {
				// 对于 nmap，scan_type 应该替换默认的扫描类型参数
				// 由于我们已经跳过了默认的 args，现在需要将 scan_type 插入到合适位置
				// 找到 target 参数的位置（通常是最后一个位置参数）
				insertPos := len(cmdArgs)
				for i := len(cmdArgs) - 1; i >= 0; i-- {
					// target 通常是最后一个非标志参数
					if !strings.HasPrefix(cmdArgs[i], "-") {
						insertPos = i
						break
					}
				}
				// 在 target 之前插入 scan_type 参数
				newArgs := make([]string, 0, len(cmdArgs)+len(scanTypeArgs))
				newArgs = append(newArgs, cmdArgs[:insertPos]...)
				newArgs = append(newArgs, scanTypeArgs...)
				newArgs = append(newArgs, cmdArgs[insertPos:]...)
				cmdArgs = newArgs
			}
		}

		return cmdArgs
	}

	// 如果没有定义参数配置，使用固定参数和通用处理
	// 添加固定参数
	cmdArgs = append(cmdArgs, toolConfig.Args...)

	// 通用处理：将参数转换为命令行参数
	for key, value := range args {
		if key == "_tool_name" {
			continue
		}
		// 使用 --key value 格式
		cmdArgs = append(cmdArgs, fmt.Sprintf("--%s", key))
		if strValue, ok := value.(string); ok {
			cmdArgs = append(cmdArgs, strValue)
		} else {
			cmdArgs = append(cmdArgs, fmt.Sprintf("%v", value))
		}
	}

	return cmdArgs
}

// parseAdditionalArgs 解析 additional_args 字符串，按空格分割但保留引号内的内容
func (e *Executor) parseAdditionalArgs(argsStr string) []string {
	if argsStr == "" {
		return []string{}
	}

	result := make([]string, 0)
	var current strings.Builder
	inQuotes := false
	var quoteChar rune
	escapeNext := false

	runes := []rune(argsStr)
	for i := 0; i < len(runes); i++ {
		r := runes[i]

		if escapeNext {
			current.WriteRune(r)
			escapeNext = false
			continue
		}

		if r == '\\' {
			// 检查下一个字符是否是引号
			if i+1 < len(runes) && (runes[i+1] == '"' || runes[i+1] == '\'') {
				// 转义的引号：跳过反斜杠，将引号作为普通字符写入
				i++
				current.WriteRune(runes[i])
			} else {
				// 其他转义字符：写入反斜杠，下一个字符会在下次迭代处理
				escapeNext = true
				current.WriteRune(r)
			}
			continue
		}

		if !inQuotes && (r == '"' || r == '\'') {
			inQuotes = true
			quoteChar = r
			continue
		}

		if inQuotes && r == quoteChar {
			inQuotes = false
			quoteChar = 0
			continue
		}

		if !inQuotes && (r == ' ' || r == '\t' || r == '\n') {
			if current.Len() > 0 {
				result = append(result, current.String())
				current.Reset()
			}
			continue
		}

		current.WriteRune(r)
	}

	// 处理最后一个参数（如果存在）
	if current.Len() > 0 {
		result = append(result, current.String())
	}

	// 如果解析结果为空，使用简单的空格分割作为降级方案
	if len(result) == 0 {
		result = strings.Fields(argsStr)
	}

	return result
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
	case "object":
		// 对象/字典：序列化为 JSON 字符串
		if jsonBytes, err := json.Marshal(value); err == nil {
			return string(jsonBytes)
		}
		// 如果 JSON 序列化失败，回退到默认格式化
		return fmt.Sprintf("%v", value)
	default:
		formattedValue := fmt.Sprintf("%v", value)
		// 特殊处理：对于 ports 参数（通常是 nmap 等工具的端口参数），清理空格
		// nmap 不接受端口列表中有空格，例如 "80,443, 22" 应该变成 "80,443,22"
		if param.Name == "ports" {
			// 移除所有空格，但保留逗号和其他字符
			formattedValue = strings.ReplaceAll(formattedValue, " ", "")
		}
		return formattedValue
	}
}

// isBackgroundCommand 检测命令是否为完全后台命令（末尾有 & 符号，但不在引号内）
// 注意：command1 & command2 这种情况不算完全后台，因为command2会在前台执行
func (e *Executor) isBackgroundCommand(command string) bool {
	// 移除首尾空格
	command = strings.TrimSpace(command)
	if command == "" {
		return false
	}

	// 检查命令中所有不在引号内的 & 符号
	// 找到最后一个 & 符号，检查它是否在命令末尾
	inSingleQuote := false
	inDoubleQuote := false
	escaped := false
	lastAmpersandPos := -1

	for i, r := range command {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
			continue
		}
		if r == '"' && !inSingleQuote {
			inDoubleQuote = !inDoubleQuote
			continue
		}
		if r == '&' && !inSingleQuote && !inDoubleQuote {
			// 检查 & 前后是否有空格或换行（确保是独立的 &，而不是变量名的一部分）
			isStandalone := false

			// 检查前面：空格、制表符、换行符，或者是命令开头
			if i == 0 {
				isStandalone = true
			} else {
				prev := command[i-1]
				if prev == ' ' || prev == '\t' || prev == '\n' || prev == '\r' {
					isStandalone = true
				}
			}

			// 检查后面：空格、制表符、换行符，或者是命令末尾
			if isStandalone {
				if i == len(command)-1 {
					// 在末尾，肯定是独立的 &
					lastAmpersandPos = i
				} else {
					next := command[i+1]
					if next == ' ' || next == '\t' || next == '\n' || next == '\r' {
						// 后面有空格，是独立的 &
						lastAmpersandPos = i
					}
				}
			}
		}
	}

	// 如果没有找到 & 符号，不是后台命令
	if lastAmpersandPos == -1 {
		return false
	}

	// 检查最后一个 & 后面是否还有非空内容
	afterAmpersand := strings.TrimSpace(command[lastAmpersandPos+1:])
	if afterAmpersand == "" {
		// & 在末尾或后面只有空白字符，这是完全后台命令
		// 检查 & 前面是否有内容
		beforeAmpersand := strings.TrimSpace(command[:lastAmpersandPos])
		return beforeAmpersand != ""
	}

	// 如果 & 后面还有非空内容，说明是 command1 & command2 的情况
	// 这种情况下，command2会在前台执行，所以不算完全后台命令
	return false
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

	// 检测是否为后台命令（包含 & 符号，但不在引号内）
	isBackground := e.isBackgroundCommand(command)

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
		zap.Bool("isBackground", isBackground),
	)

	// 如果是后台命令，使用特殊处理来获取实际的后台进程PID
	if isBackground {
		// 移除命令末尾的 & 符号
		commandWithoutAmpersand := strings.TrimSuffix(strings.TrimSpace(command), "&")
		commandWithoutAmpersand = strings.TrimSpace(commandWithoutAmpersand)

		// 构建新命令：command & pid=$!; echo $pid
		// 使用变量保存PID，确保能获取到正确的后台进程PID
		pidCommand := fmt.Sprintf("%s & pid=$!; echo $pid", commandWithoutAmpersand)

		// 创建新命令来获取PID
		var pidCmd *exec.Cmd
		if workDir != "" {
			pidCmd = exec.CommandContext(ctx, shell, "-c", pidCommand)
			pidCmd.Dir = workDir
		} else {
			pidCmd = exec.CommandContext(ctx, shell, "-c", pidCommand)
		}

		// 获取stdout管道
		stdout, err := pidCmd.StdoutPipe()
		if err != nil {
			e.logger.Error("创建stdout管道失败",
				zap.String("command", command),
				zap.Error(err),
			)
			// 如果创建管道失败，使用shell进程的PID作为fallback
			if err := pidCmd.Start(); err != nil {
				return &mcp.ToolResult{
					Content: []mcp.Content{
						{
							Type: "text",
							Text: fmt.Sprintf("后台命令启动失败: %v", err),
						},
					},
					IsError: true,
				}, nil
			}
			pid := pidCmd.Process.Pid
			go pidCmd.Wait() // 在后台等待，避免僵尸进程
			return &mcp.ToolResult{
				Content: []mcp.Content{
					{
						Type: "text",
						Text: fmt.Sprintf("后台命令已启动\n命令: %s\n进程ID: %d (可能不准确，获取PID失败)\n\n注意: 后台进程将继续运行，不会等待其完成。", command, pid),
					},
				},
				IsError: false,
			}, nil
		}

		// 启动命令
		if err := pidCmd.Start(); err != nil {
			stdout.Close()
			e.logger.Error("后台命令启动失败",
				zap.String("command", command),
				zap.Error(err),
			)
			return &mcp.ToolResult{
				Content: []mcp.Content{
					{
						Type: "text",
						Text: fmt.Sprintf("后台命令启动失败: %v", err),
					},
				},
				IsError: true,
			}, nil
		}

		// 读取第一行输出（PID）
		reader := bufio.NewReader(stdout)
		pidLine, err := reader.ReadString('\n')
		stdout.Close()

		var actualPid int
		if err != nil && err != io.EOF {
			e.logger.Warn("读取后台进程PID失败",
				zap.String("command", command),
				zap.Error(err),
			)
			// 如果读取失败，使用shell进程的PID
			actualPid = pidCmd.Process.Pid
		} else {
			// 解析PID
			pidStr := strings.TrimSpace(pidLine)
			if parsedPid, err := strconv.Atoi(pidStr); err == nil {
				actualPid = parsedPid
			} else {
				e.logger.Warn("解析后台进程PID失败",
					zap.String("command", command),
					zap.String("pidLine", pidStr),
					zap.Error(err),
				)
				// 如果解析失败，使用shell进程的PID
				actualPid = pidCmd.Process.Pid
			}
		}

		// 在goroutine中等待shell进程，避免僵尸进程
		go func() {
			if err := pidCmd.Wait(); err != nil {
				e.logger.Debug("后台命令shell进程执行完成",
					zap.String("command", command),
					zap.Error(err),
				)
			}
		}()

		e.logger.Info("后台命令已启动",
			zap.String("command", command),
			zap.Int("actualPid", actualPid),
		)

		return &mcp.ToolResult{
			Content: []mcp.Content{
				{
					Type: "text",
					Text: fmt.Sprintf("后台命令已启动\n命令: %s\n进程ID: %d\n\n注意: 后台进程将继续运行，不会等待其完成。", command, actualPid),
				},
			},
			IsError: false,
		}, nil
	}

	// 非后台命令：等待输出
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

// executeInternalTool 执行内部工具（不执行外部命令）
func (e *Executor) executeInternalTool(ctx context.Context, toolName string, command string, args map[string]interface{}) (*mcp.ToolResult, error) {
	// 提取内部工具类型（去掉 "internal:" 前缀）
	internalToolType := strings.TrimPrefix(command, "internal:")

	e.logger.Info("执行内部工具",
		zap.String("toolName", toolName),
		zap.String("internalToolType", internalToolType),
		zap.Any("args", args),
	)

	// 根据内部工具类型分发处理
	switch internalToolType {
	case "query_execution_result":
		return e.executeQueryExecutionResult(ctx, args)
	default:
		return &mcp.ToolResult{
			Content: []mcp.Content{
				{
					Type: "text",
					Text: fmt.Sprintf("错误: 未知的内部工具类型: %s", internalToolType),
				},
			},
			IsError: true,
		}, nil
	}
}

// executeQueryExecutionResult 执行查询执行结果工具
func (e *Executor) executeQueryExecutionResult(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
	// 获取 execution_id 参数
	executionID, ok := args["execution_id"].(string)
	if !ok || executionID == "" {
		return &mcp.ToolResult{
			Content: []mcp.Content{
				{
					Type: "text",
					Text: "错误: execution_id 参数必需且不能为空",
				},
			},
			IsError: true,
		}, nil
	}

	// 获取可选参数
	page := 1
	if p, ok := args["page"].(float64); ok {
		page = int(p)
	}
	if page < 1 {
		page = 1
	}

	limit := 100
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}
	if limit < 1 {
		limit = 100
	}
	if limit > 500 {
		limit = 500 // 限制最大每页行数
	}

	search := ""
	if s, ok := args["search"].(string); ok {
		search = s
	}

	filter := ""
	if f, ok := args["filter"].(string); ok {
		filter = f
	}

	useRegex := false
	if r, ok := args["use_regex"].(bool); ok {
		useRegex = r
	}

	// 检查结果存储是否可用
	if e.resultStorage == nil {
		return &mcp.ToolResult{
			Content: []mcp.Content{
				{
					Type: "text",
					Text: "错误: 结果存储未初始化",
				},
			},
			IsError: true,
		}, nil
	}

	// 执行查询
	var resultPage *storage.ResultPage
	var err error

	if search != "" {
		// 搜索模式
		matchedLines, err := e.resultStorage.SearchResult(executionID, search, useRegex)
		if err != nil {
			return &mcp.ToolResult{
				Content: []mcp.Content{
					{
						Type: "text",
						Text: fmt.Sprintf("搜索失败: %v", err),
					},
				},
				IsError: true,
			}, nil
		}
		// 对搜索结果进行分页
		resultPage = paginateLines(matchedLines, page, limit)
	} else if filter != "" {
		// 过滤模式
		filteredLines, err := e.resultStorage.FilterResult(executionID, filter, useRegex)
		if err != nil {
			return &mcp.ToolResult{
				Content: []mcp.Content{
					{
						Type: "text",
						Text: fmt.Sprintf("过滤失败: %v", err),
					},
				},
				IsError: true,
			}, nil
		}
		// 对过滤结果进行分页
		resultPage = paginateLines(filteredLines, page, limit)
	} else {
		// 普通分页查询
		resultPage, err = e.resultStorage.GetResultPage(executionID, page, limit)
		if err != nil {
			return &mcp.ToolResult{
				Content: []mcp.Content{
					{
						Type: "text",
						Text: fmt.Sprintf("查询失败: %v", err),
					},
				},
				IsError: true,
			}, nil
		}
	}

	// 获取元信息
	metadata, err := e.resultStorage.GetResultMetadata(executionID)
	if err != nil {
		// 元信息获取失败不影响查询结果
		e.logger.Warn("获取结果元信息失败", zap.Error(err))
	}

	// 格式化返回结果
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("查询结果 (执行ID: %s)\n", executionID))

	if metadata != nil {
		sb.WriteString(fmt.Sprintf("工具: %s | 大小: %d 字节 (%.2f KB) | 总行数: %d\n",
			metadata.ToolName, metadata.TotalSize, float64(metadata.TotalSize)/1024, metadata.TotalLines))
	}

	sb.WriteString(fmt.Sprintf("第 %d/%d 页，每页 %d 行，共 %d 行\n\n",
		resultPage.Page, resultPage.TotalPages, resultPage.Limit, resultPage.TotalLines))

	if len(resultPage.Lines) == 0 {
		sb.WriteString("没有找到匹配的结果。\n")
	} else {
		for i, line := range resultPage.Lines {
			lineNum := (resultPage.Page-1)*resultPage.Limit + i + 1
			sb.WriteString(fmt.Sprintf("%d: %s\n", lineNum, line))
		}
	}

	sb.WriteString("\n")
	if resultPage.Page < resultPage.TotalPages {
		sb.WriteString(fmt.Sprintf("提示: 使用 page=%d 查看下一页", resultPage.Page+1))
		if search != "" {
			sb.WriteString(fmt.Sprintf("，或使用 search=\"%s\" 继续搜索", search))
			if useRegex {
				sb.WriteString(" (正则模式)")
			}
		}
		if filter != "" {
			sb.WriteString(fmt.Sprintf("，或使用 filter=\"%s\" 继续过滤", filter))
			if useRegex {
				sb.WriteString(" (正则模式)")
			}
		}
		sb.WriteString("\n")
	}

	return &mcp.ToolResult{
		Content: []mcp.Content{
			{
				Type: "text",
				Text: sb.String(),
			},
		},
		IsError: false,
	}, nil
}

// paginateLines 对行列表进行分页
func paginateLines(lines []string, page int, limit int) *storage.ResultPage {
	totalLines := len(lines)
	totalPages := (totalLines + limit - 1) / limit
	if page < 1 {
		page = 1
	}
	if page > totalPages && totalPages > 0 {
		page = totalPages
	}

	start := (page - 1) * limit
	end := start + limit
	if end > totalLines {
		end = totalLines
	}

	var pageLines []string
	if start < totalLines {
		pageLines = lines[start:end]
	} else {
		pageLines = []string{}
	}

	return &storage.ResultPage{
		Lines:      pageLines,
		Page:       page,
		Limit:      limit,
		TotalLines: totalLines,
		TotalPages: totalPages,
	}
}

// buildInputSchema 构建输入模式
func (e *Executor) buildInputSchema(toolConfig *config.ToolConfig) map[string]interface{} {
	schema := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
		"required":   []string{},
	}

	// 如果配置中定义了参数，优先使用配置中的参数定义
	if len(toolConfig.Parameters) > 0 {
		properties := make(map[string]interface{})
		required := []string{}

		for _, param := range toolConfig.Parameters {
			// 跳过 name 为空的参数（避免 YAML 中 name: null 或空导致非法 schema）
			if strings.TrimSpace(param.Name) == "" {
				e.logger.Debug("跳过无名称的参数",
					zap.String("tool", toolConfig.Name),
					zap.String("type", param.Type),
				)
				continue
			}
			// 转换类型为OpenAI/JSON Schema标准类型（空类型默认为 string）
			openAIType := e.convertToOpenAIType(param.Type)

			prop := map[string]interface{}{
				"type":        openAIType,
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

	// 如果没有定义参数配置，返回空schema
	// 这种情况下工具可能只使用固定参数（args字段）
	// 或者需要通过YAML配置文件定义参数
	e.logger.Warn("工具未定义参数配置，返回空schema",
		zap.String("tool", toolConfig.Name),
	)
	return schema
}

// convertToOpenAIType 将配置中的类型转换为OpenAI/JSON Schema标准类型
func (e *Executor) convertToOpenAIType(configType string) string {
	// 空或 null 类型统一视为 string，避免非法 schema 导致工具调用失败
	if strings.TrimSpace(configType) == "" {
		return "string"
	}
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
		// 默认返回原类型，但记录警告
		e.logger.Warn("未知的参数类型，使用原类型",
			zap.String("type", configType),
		)
		return configType
	}
}

// getExitCode 从错误中提取退出码，如果不是ExitError则返回nil
func getExitCode(err error) *int {
	if err == nil {
		return nil
	}
	if exitError, ok := err.(*exec.ExitError); ok {
		if exitError.ProcessState != nil {
			exitCode := exitError.ExitCode()
			return &exitCode
		}
	}
	return nil
}

// getExitCodeValue 从错误中提取退出码值，如果不是ExitError则返回-1
func getExitCodeValue(err error) int {
	if code := getExitCode(err); code != nil {
		return *code
	}
	return -1
}
