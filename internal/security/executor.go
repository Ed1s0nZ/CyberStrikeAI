package security

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/mcp"
	"cyberstrike-ai/internal/storage"

	"go.uber.org/zap"
)

// Executor security tool executor
type Executor struct {
	config        *config.SecurityConfig
	toolIndex     map[string]*config.ToolConfig // tool index for O(1) lookup
	mcpServer     *mcp.Server
	logger        *zap.Logger
	resultStorage ResultStorage // result storage (for query tools)
}

// ResultStorage result storage interface (directly using storage package types)
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

// NewExecutor creates a new executor
func NewExecutor(cfg *config.SecurityConfig, mcpServer *mcp.Server, logger *zap.Logger) *Executor {
	executor := &Executor{
		config:        cfg,
		toolIndex:     make(map[string]*config.ToolConfig),
		mcpServer:     mcpServer,
		logger:        logger,
		resultStorage: nil, // set later via SetResultStorage
	}
	// build tool index
	executor.buildToolIndex()
	return executor
}

// SetResultStorage sets the result storage
func (e *Executor) SetResultStorage(storage ResultStorage) {
	e.resultStorage = storage
}

// buildToolIndex builds the tool index, optimizing O(n) lookup to O(1)
func (e *Executor) buildToolIndex() {
	e.toolIndex = make(map[string]*config.ToolConfig)
	for i := range e.config.Tools {
		if e.config.Tools[i].Enabled {
			e.toolIndex[e.config.Tools[i].Name] = &e.config.Tools[i]
		}
	}
	e.logger.Info("tool index build complete",
		zap.Int("totalTools", len(e.config.Tools)),
		zap.Int("enabledTools", len(e.toolIndex)),
	)
}

// ExecuteTool executes a security tool
func (e *Executor) ExecuteTool(ctx context.Context, toolName string, args map[string]interface{}) (*mcp.ToolResult, error) {
	e.logger.Info("ExecuteTool called",
		zap.String("toolName", toolName),
		zap.Any("args", args),
	)

	// special handling: exec tool directly executes system commands
	if toolName == "exec" {
		e.logger.Info("executing exec tool")
		return e.executeSystemCommand(ctx, args)
	}

	// use index to look up tool configuration (O(1) lookup)
	toolConfig, exists := e.toolIndex[toolName]
	if !exists {
		e.logger.Error("tool not found or not enabled",
			zap.String("toolName", toolName),
			zap.Int("totalTools", len(e.config.Tools)),
			zap.Int("enabledTools", len(e.toolIndex)),
		)
		return nil, fmt.Errorf("tool %s not found or not enabled", toolName)
	}

	e.logger.Info("tool configuration found",
		zap.String("toolName", toolName),
		zap.String("command", toolConfig.Command),
		zap.Strings("args", toolConfig.Args),
	)

	// special handling: internal tools (command starts with "internal:")
	if strings.HasPrefix(toolConfig.Command, "internal:") {
		e.logger.Info("executing internal tool",
			zap.String("toolName", toolName),
			zap.String("command", toolConfig.Command),
		)
		return e.executeInternalTool(ctx, toolName, toolConfig.Command, args)
	}

	// build command - use different parameter formats depending on tool type
	cmdArgs := e.buildCommandArgs(toolName, toolConfig, args)

	e.logger.Info("command arguments built",
		zap.String("toolName", toolName),
		zap.Strings("cmdArgs", cmdArgs),
		zap.Int("argsCount", len(cmdArgs)),
	)

	// validate command arguments
	if len(cmdArgs) == 0 {
		e.logger.Warn("command arguments are empty",
			zap.String("toolName", toolName),
			zap.Any("inputArgs", args),
		)
		return &mcp.ToolResult{
			Content: []mcp.Content{
				{
					Type: "text",
					Text: fmt.Sprintf("Error: tool %s is missing required parameters. Received parameters: %v", toolName, args),
				},
			},
			IsError: true,
		}, nil
	}

	// execute command
	cmd := exec.CommandContext(ctx, toolConfig.Command, cmdArgs...)

	e.logger.Info("executing security tool",
		zap.String("tool", toolName),
		zap.Strings("args", cmdArgs),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		// check if exit code is in the allowed list
		exitCode := getExitCode(err)
		if exitCode != nil && toolConfig.AllowedExitCodes != nil {
			for _, allowedCode := range toolConfig.AllowedExitCodes {
				if *exitCode == allowedCode {
					e.logger.Info("tool execution complete (exit code in allowed list)",
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

		e.logger.Error("tool execution failed",
			zap.String("tool", toolName),
			zap.Error(err),
			zap.Int("exitCode", getExitCodeValue(err)),
			zap.String("output", string(output)),
		)
		return &mcp.ToolResult{
			Content: []mcp.Content{
				{
					Type: "text",
					Text: fmt.Sprintf("tool execution failed: %v\noutput: %s", err, string(output)),
				},
			},
			IsError: true,
		}, nil
	}

	e.logger.Info("tool execution successful",
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

// RegisterTools registers tools to the MCP server
func (e *Executor) RegisterTools(mcpServer *mcp.Server) {
	e.logger.Info("starting tool registration",
		zap.Int("totalTools", len(e.config.Tools)),
		zap.Int("enabledTools", len(e.toolIndex)),
	)

	// rebuild index (in case config was updated)
	e.buildToolIndex()

	for i, toolConfig := range e.config.Tools {
		if !toolConfig.Enabled {
			e.logger.Debug("skipping disabled tool",
				zap.String("tool", toolConfig.Name),
			)
			continue
		}

		// create a copy of tool config to avoid closure issues
		toolName := toolConfig.Name
		toolConfigCopy := toolConfig

		// decide whether to expose short_description or description to AI/API based on config
		useFullDescription := strings.TrimSpace(strings.ToLower(e.config.ToolDescriptionMode)) == "full"
		shortDesc := toolConfigCopy.ShortDescription
		if shortDesc == "" {
			// if no short description, extract first line or first 10000 characters from full description
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
			shortDesc = "" // when using description, clear ShortDescription, downstream will fall back to Description
		}

		tool := mcp.Tool{
			Name:             toolConfigCopy.Name,
			Description:      toolConfigCopy.Description,
			ShortDescription: shortDesc,
			InputSchema:      e.buildInputSchema(&toolConfigCopy),
		}

		handler := func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
			e.logger.Info("tool handler called",
				zap.String("toolName", toolName),
				zap.Any("args", args),
			)
			return e.ExecuteTool(ctx, toolName, args)
		}

		mcpServer.RegisterTool(tool, handler)
		e.logger.Info("security tool registered successfully",
			zap.String("tool", toolConfigCopy.Name),
			zap.String("command", toolConfigCopy.Command),
			zap.Int("index", i),
		)
	}

	e.logger.Info("tool registration complete",
		zap.Int("registeredCount", len(e.config.Tools)),
	)
}

// buildCommandArgs builds command arguments
func (e *Executor) buildCommandArgs(toolName string, toolConfig *config.ToolConfig, args map[string]interface{}) []string {
	cmdArgs := make([]string, 0)

	// if parameter mappings are defined in config, use the config mapping rules
	if len(toolConfig.Parameters) > 0 {
		// check if there is a scan_type parameter; if so, replace the default scan type parameter
		hasScanType := false
		var scanTypeValue string
		if scanType, ok := args["scan_type"].(string); ok && scanType != "" {
			hasScanType = true
			scanTypeValue = scanType
		}

		// add fixed arguments (may need to filter out default scan type args if scan_type is specified)
		if hasScanType && toolName == "nmap" {
			// for nmap, if scan_type is specified, skip the default -sT -sV -sC
			// these args will be replaced by the scan_type parameter
		} else {
			cmdArgs = append(cmdArgs, toolConfig.Args...)
		}

		// sort by positional parameters
		positionalParams := make([]config.ParameterConfig, 0)
		flagParams := make([]config.ParameterConfig, 0)

		for _, param := range toolConfig.Parameters {
			if param.Position != nil {
				positionalParams = append(positionalParams, param)
			} else {
				flagParams = append(flagParams, param)
			}
		}

		// for tools that need subcommands (e.g. gobuster dir), position 0 must come right after
		// the command name, before all flags
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

		// handle flag parameters
		for _, param := range flagParams {
			// skip special parameters, they will be handled separately below
			// action parameter is only used for internal tool logic, not passed to the command
			if param.Name == "additional_args" || param.Name == "scan_type" || param.Name == "action" {
				continue
			}

			value := e.getParamValue(args, param)
			if value == nil {
				if param.Required {
					// required parameter is missing, return empty array for upper layer to handle error
					e.logger.Warn("missing required flag parameter",
						zap.String("tool", toolName),
						zap.String("param", param.Name),
					)
					return []string{}
				}
				continue
			}

			// special handling for boolean values: skip if false; if true, only add the flag
			if param.Type == "bool" {
				var boolVal bool
				var ok bool

				// try multiple type conversions
				if boolVal, ok = value.(bool); ok {
					// already a boolean
				} else if numVal, ok := value.(float64); ok {
					// JSON number type (float64)
					boolVal = numVal != 0
					ok = true
				} else if numVal, ok := value.(int); ok {
					// int type
					boolVal = numVal != 0
					ok = true
				} else if strVal, ok := value.(string); ok {
					// string type
					boolVal = strVal == "true" || strVal == "1" || strVal == "yes"
					ok = true
				}

				if ok {
					if !boolVal {
						continue // don't add any parameter when false
					}
					// when true, only add the flag, not the value
					if param.Flag != "" {
						cmdArgs = append(cmdArgs, param.Flag)
					}
					continue
				}
			}

			format := param.Format
			if format == "" {
				format = "flag" // default format
			}

			switch format {
			case "flag":
				// --flag value or -f value
				formattedValue := e.formatParamValue(param, value)
				if strings.TrimSpace(formattedValue) == "" {
					if param.Required {
						e.logger.Warn("required flag parameter has empty value",
							zap.String("tool", toolName),
							zap.String("param", param.Name),
						)
						return []string{}
					}
					continue
				}
				if param.Flag != "" {
					cmdArgs = append(cmdArgs, param.Flag)
				}
				cmdArgs = append(cmdArgs, formattedValue)
			case "combined":
				// --flag=value or -f=value
				formattedValue := e.formatParamValue(param, value)
				if strings.TrimSpace(formattedValue) == "" {
					if param.Required {
						e.logger.Warn("required combined parameter has empty value",
							zap.String("tool", toolName),
							zap.String("param", param.Name),
						)
						return []string{}
					}
					continue
				}
				if param.Flag != "" {
					cmdArgs = append(cmdArgs, fmt.Sprintf("%s=%s", param.Flag, formattedValue))
				} else {
					cmdArgs = append(cmdArgs, formattedValue)
				}
			case "template":
				// use template string
				formattedValue := e.formatParamValue(param, value)
				if strings.TrimSpace(formattedValue) == "" {
					if param.Required {
						e.logger.Warn("required template parameter has empty value",
							zap.String("tool", toolName),
							zap.String("param", param.Name),
						)
						return []string{}
					}
					continue
				}
				if param.Template != "" {
					template := param.Template
					template = strings.ReplaceAll(template, "{flag}", param.Flag)
					template = strings.ReplaceAll(template, "{value}", formattedValue)
					template = strings.ReplaceAll(template, "{name}", param.Name)
					cmdArgs = append(cmdArgs, strings.Fields(template)...)
				} else {
					// if no template, use default format
					if param.Flag != "" {
						cmdArgs = append(cmdArgs, param.Flag)
					}
					cmdArgs = append(cmdArgs, formattedValue)
				}
			case "positional":
				// positional parameter (already handled above)
				formattedValue := e.formatParamValue(param, value)
				if strings.TrimSpace(formattedValue) == "" {
					if param.Required {
						e.logger.Warn("required positional parameter has empty value",
							zap.String("tool", toolName),
							zap.String("param", param.Name),
						)
						return []string{}
					}
					continue
				}
				cmdArgs = append(cmdArgs, formattedValue)
			default:
				// default: add value directly
				formattedValue := e.formatParamValue(param, value)
				if strings.TrimSpace(formattedValue) == "" {
					if param.Required {
						e.logger.Warn("required parameter has empty value",
							zap.String("tool", toolName),
							zap.String("param", param.Name),
						)
						return []string{}
					}
					continue
				}
				cmdArgs = append(cmdArgs, formattedValue)
			}
		}

		// then handle positional parameters (positional params usually come after flag params)
		// sort positional parameters by position
		// first find the maximum position value, to determine how many positions to process
		maxPosition := -1
		for _, param := range positionalParams {
			if param.Position != nil && *param.Position > maxPosition {
				maxPosition = *param.Position
			}
		}

		// process parameters in positional order, ensuring correct transmission even if some positions
		// have no parameters or use default values
		// position 0 was already inserted above (subcommand first), start from 1 here
		for i := 0; i <= maxPosition; i++ {
			if i == 0 {
				continue
			}
			for _, param := range positionalParams {
				// skip special parameters, they will be handled separately below
				// action parameter is only used for internal tool logic, not passed to the command
				if param.Name == "additional_args" || param.Name == "scan_type" || param.Name == "action" {
					continue
				}

				if param.Position != nil && *param.Position == i {
					value := e.getParamValue(args, param)
					if value == nil {
						if param.Required {
							// required parameter is missing, return empty array for upper layer to handle error
							e.logger.Warn("missing required positional parameter",
								zap.String("tool", toolName),
								zap.String("param", param.Name),
								zap.Int("position", *param.Position),
							)
							return []string{}
						}
						// for non-required parameters, try to use default value if nil
						if param.Default != nil {
							value = param.Default
						} else {
							// if no default value, skip this position and continue to the next
							break
						}
					}
					// only add to command arguments when value is not nil
					if value != nil {
						formattedValue := e.formatParamValue(param, value)
						if strings.TrimSpace(formattedValue) == "" {
							if param.Required {
								e.logger.Warn("required positional parameter has empty value",
									zap.String("tool", toolName),
									zap.String("param", param.Name),
									zap.Int("position", *param.Position),
								)
								return []string{}
							}
						} else {
							cmdArgs = append(cmdArgs, formattedValue)
						}
					}
					break
				}
			}
			// if no parameter found for a position, continue to the next position
			// this ensures the positional parameter order is correct
		}

		// special handling: additional_args parameter (needs to be split by spaces into multiple arguments)
		if additionalArgs, ok := args["additional_args"].(string); ok && additionalArgs != "" {
			// split by spaces, but preserve content inside quotes
			additionalArgsList := e.parseAdditionalArgs(additionalArgs)
			cmdArgs = append(cmdArgs, additionalArgsList...)
		}

		// special handling: scan_type parameter (needs to be split by spaces and inserted at the right position)
		if hasScanType {
			scanTypeArgs := e.parseAdditionalArgs(scanTypeValue)
			if len(scanTypeArgs) > 0 {
				// prepend scan_type args to avoid splitting flag-value pairs that may exist in
				// additional_args (e.g. "--min-rate 500"), which can happen with heuristic insertion.
				newArgs := make([]string, 0, len(cmdArgs)+len(scanTypeArgs))
				newArgs = append(newArgs, scanTypeArgs...)
				newArgs = append(newArgs, cmdArgs...)
				cmdArgs = newArgs
			}
		}

		if toolName == "nmap" {
			sanitized, changed := sanitizeNmapArgs(cmdArgs, os.Geteuid() == 0)
			if changed {
				e.logger.Warn("sanitized nmap arguments for non-root execution",
					zap.Strings("originalArgs", cmdArgs),
					zap.Strings("sanitizedArgs", sanitized),
				)
			}
			cmdArgs = sanitized
		}

		return cmdArgs
	}

	// if no parameter configuration defined, use fixed args and generic handling
	// add fixed arguments
	cmdArgs = append(cmdArgs, toolConfig.Args...)

	// generic handling: convert parameters to command line arguments
	for key, value := range args {
		if key == "_tool_name" {
			continue
		}
		// use --key value format
		cmdArgs = append(cmdArgs, fmt.Sprintf("--%s", key))
		if strValue, ok := value.(string); ok {
			cmdArgs = append(cmdArgs, strValue)
		} else {
			cmdArgs = append(cmdArgs, fmt.Sprintf("%v", value))
		}
	}

	return cmdArgs
}

// parseAdditionalArgs parses the additional_args string, splitting by spaces but preserving content inside quotes
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
			// check if the next character is a quote
			if i+1 < len(runes) && (runes[i+1] == '"' || runes[i+1] == '\'') {
				// escaped quote: skip backslash, write quote as normal character
				i++
				current.WriteRune(runes[i])
			} else {
				// other escape characters: write backslash, next character will be processed in next iteration
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

	// handle the last argument (if exists)
	if current.Len() > 0 {
		result = append(result, current.String())
	}

	// if parse result is empty, use simple space split as fallback
	if len(result) == 0 {
		result = strings.Fields(argsStr)
	}

	return result
}

// getParamValue gets parameter value, supporting default values
func (e *Executor) getParamValue(args map[string]interface{}, param config.ParameterConfig) interface{} {
	// get value from parameters
	if value, ok := args[param.Name]; ok && value != nil {
		return value
	}

	// if parameter is required but not provided, return nil (let upper layer handle the error)
	if param.Required {
		return nil
	}

	// return default value
	return param.Default
}

// formatParamValue formats parameter value
func (e *Executor) formatParamValue(param config.ParameterConfig, value interface{}) string {
	switch param.Type {
	case "bool":
		// boolean values should be handled by the upper layer; this should not be called
		if boolVal, ok := value.(bool); ok {
			return fmt.Sprintf("%v", boolVal)
		}
		return "false"
	case "array":
		// array: convert to comma-separated string
		if arr, ok := value.([]interface{}); ok {
			strs := make([]string, 0, len(arr))
			for _, item := range arr {
				strs = append(strs, fmt.Sprintf("%v", item))
			}
			return strings.Join(strs, ",")
		}
		return fmt.Sprintf("%v", value)
	case "object":
		// object/dictionary: serialize to JSON string
		if jsonBytes, err := json.Marshal(value); err == nil {
			return string(jsonBytes)
		}
		// if JSON serialization fails, fall back to default formatting
		return fmt.Sprintf("%v", value)
	default:
		formattedValue := fmt.Sprintf("%v", value)
		// special handling: for the ports parameter (usually nmap and similar tools), remove spaces
		// nmap doesn't accept spaces in port list, e.g. "80,443, 22" should become "80,443,22"
		if param.Name == "ports" {
			// remove all spaces but preserve commas and other characters
			formattedValue = strings.ReplaceAll(formattedValue, " ", "")
		}
		return formattedValue
	}
}

// sanitizeNmapArgs removes nmap options that require root when running as non-root.
// This keeps scans running instead of hard-failing with privilege errors.
func sanitizeNmapArgs(args []string, isRoot bool) ([]string, bool) {
	if isRoot {
		return args, false
	}

	sanitized := make([]string, 0, len(args))
	changed := false
	hasSynScan := false
	hasTcpConnect := false

	for _, arg := range args {
		switch arg {
		case "-O", "-A", "--osscan-guess":
			changed = true
			continue
		case "-sS":
			hasSynScan = true
			changed = true
			continue
		case "-sT":
			hasTcpConnect = true
		}
		sanitized = append(sanitized, arg)
	}

	if hasSynScan && !hasTcpConnect {
		sanitized = append([]string{"-sT"}, sanitized...)
	}

	return sanitized, changed
}

// isBackgroundCommand detects whether a command is a fully background command (has & at the end, but not inside quotes)
// Note: command1 & command2 is not considered fully background because command2 runs in the foreground
func (e *Executor) isBackgroundCommand(command string) bool {
	// trim leading/trailing spaces
	command = strings.TrimSpace(command)
	if command == "" {
		return false
	}

	// check all & symbols in the command that are not inside quotes
	// find the last & symbol and check if it is at the end of the command
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
			// check if there is a space or newline before/after & (to ensure it is a standalone &, not part of a variable name)
			isStandalone := false

			// check before: space, tab, newline, or beginning of command
			if i == 0 {
				isStandalone = true
			} else {
				prev := command[i-1]
				if prev == ' ' || prev == '\t' || prev == '\n' || prev == '\r' {
					isStandalone = true
				}
			}

			// check after: space, tab, newline, or end of command
			if isStandalone {
				if i == len(command)-1 {
					// at the end, definitely standalone &
					lastAmpersandPos = i
				} else {
					next := command[i+1]
					if next == ' ' || next == '\t' || next == '\n' || next == '\r' {
						// space after, standalone &
						lastAmpersandPos = i
					}
				}
			}
		}
	}

	// if no & symbol found, not a background command
	if lastAmpersandPos == -1 {
		return false
	}

	// check if there is any non-whitespace content after the last &
	afterAmpersand := strings.TrimSpace(command[lastAmpersandPos+1:])
	if afterAmpersand == "" {
		// & is at the end or only whitespace after it, this is a fully background command
		// check if there is content before &
		beforeAmpersand := strings.TrimSpace(command[:lastAmpersandPos])
		return beforeAmpersand != ""
	}

	// if there is non-whitespace content after &, this is the command1 & command2 pattern
	// in this case, command2 runs in the foreground, so it's not a fully background command
	return false
}

// executeSystemCommand executes a system command
func (e *Executor) executeSystemCommand(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
	// get the command
	command, ok := args["command"].(string)
	if !ok {
		return &mcp.ToolResult{
			Content: []mcp.Content{
				{
					Type: "text",
					Text: "Error: missing command parameter",
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
					Text: "Error: command parameter cannot be empty",
				},
			},
			IsError: true,
		}, nil
	}

	// security check: log the executed command
	e.logger.Warn("executing system command",
		zap.String("command", command),
	)

	// get shell type (optional, defaults to sh)
	shell := "sh"
	if s, ok := args["shell"].(string); ok && s != "" {
		shell = s
	}

	// get working directory (optional)
	workDir := ""
	if wd, ok := args["workdir"].(string); ok && wd != "" {
		workDir = wd
	}

	// detect if this is a background command (contains & symbol, but not inside quotes)
	isBackground := e.isBackgroundCommand(command)

	// build command
	var cmd *exec.Cmd
	if workDir != "" {
		cmd = exec.CommandContext(ctx, shell, "-c", command)
		cmd.Dir = workDir
	} else {
		cmd = exec.CommandContext(ctx, shell, "-c", command)
	}

	// execute command
	e.logger.Info("executing system command",
		zap.String("command", command),
		zap.String("shell", shell),
		zap.String("workdir", workDir),
		zap.Bool("isBackground", isBackground),
	)

	// if it's a background command, use special handling to get the actual background process PID
	if isBackground {
		// remove the & symbol at the end of the command
		commandWithoutAmpersand := strings.TrimSuffix(strings.TrimSpace(command), "&")
		commandWithoutAmpersand = strings.TrimSpace(commandWithoutAmpersand)

		// build new command: command & pid=$!; echo $pid
		// use variable to save PID, ensuring we can get the correct background process PID
		pidCommand := fmt.Sprintf("%s & pid=$!; echo $pid", commandWithoutAmpersand)

		// create new command to get PID
		var pidCmd *exec.Cmd
		if workDir != "" {
			pidCmd = exec.CommandContext(ctx, shell, "-c", pidCommand)
			pidCmd.Dir = workDir
		} else {
			pidCmd = exec.CommandContext(ctx, shell, "-c", pidCommand)
		}

		// get stdout pipe
		stdout, err := pidCmd.StdoutPipe()
		if err != nil {
			e.logger.Error("failed to create stdout pipe",
				zap.String("command", command),
				zap.Error(err),
			)
			// if pipe creation fails, use the shell process PID as fallback
			if err := pidCmd.Start(); err != nil {
				return &mcp.ToolResult{
					Content: []mcp.Content{
						{
							Type: "text",
							Text: fmt.Sprintf("background command failed to start: %v", err),
						},
					},
					IsError: true,
				}, nil
			}
			pid := pidCmd.Process.Pid
			go pidCmd.Wait() // wait in background to avoid zombie processes
			return &mcp.ToolResult{
				Content: []mcp.Content{
					{
						Type: "text",
						Text: fmt.Sprintf("background command started\ncommand: %s\nprocess ID: %d (may be inaccurate, failed to get PID)\n\nNote: background process will continue running and will not be waited for.", command, pid),
					},
				},
				IsError: false,
			}, nil
		}

		// start the command
		if err := pidCmd.Start(); err != nil {
			stdout.Close()
			e.logger.Error("background command failed to start",
				zap.String("command", command),
				zap.Error(err),
			)
			return &mcp.ToolResult{
				Content: []mcp.Content{
					{
						Type: "text",
						Text: fmt.Sprintf("background command failed to start: %v", err),
					},
				},
				IsError: true,
			}, nil
		}

		// read the first line of output (PID)
		reader := bufio.NewReader(stdout)
		pidLine, err := reader.ReadString('\n')
		stdout.Close()

		var actualPid int
		if err != nil && err != io.EOF {
			e.logger.Warn("failed to read background process PID",
				zap.String("command", command),
				zap.Error(err),
			)
			// if reading fails, use the shell process PID
			actualPid = pidCmd.Process.Pid
		} else {
			// parse PID
			pidStr := strings.TrimSpace(pidLine)
			if parsedPid, err := strconv.Atoi(pidStr); err == nil {
				actualPid = parsedPid
			} else {
				e.logger.Warn("failed to parse background process PID",
					zap.String("command", command),
					zap.String("pidLine", pidStr),
					zap.Error(err),
				)
				// if parsing fails, use the shell process PID
				actualPid = pidCmd.Process.Pid
			}
		}

		// wait for the shell process in a goroutine to avoid zombie processes
		go func() {
			if err := pidCmd.Wait(); err != nil {
				e.logger.Debug("background command shell process completed",
					zap.String("command", command),
					zap.Error(err),
				)
			}
		}()

		e.logger.Info("background command started",
			zap.String("command", command),
			zap.Int("actualPid", actualPid),
		)

		return &mcp.ToolResult{
			Content: []mcp.Content{
				{
					Type: "text",
					Text: fmt.Sprintf("background command started\ncommand: %s\nprocess ID: %d\n\nNote: background process will continue running and will not be waited for.", command, actualPid),
				},
			},
			IsError: false,
		}, nil
	}

	// non-background command: wait for output
	output, err := cmd.CombinedOutput()
	if err != nil {
		e.logger.Error("system command execution failed",
			zap.String("command", command),
			zap.Error(err),
			zap.String("output", string(output)),
		)
		return &mcp.ToolResult{
			Content: []mcp.Content{
				{
					Type: "text",
					Text: fmt.Sprintf("command execution failed: %v\noutput: %s", err, string(output)),
				},
			},
			IsError: true,
		}, nil
	}

	e.logger.Info("system command executed successfully",
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

// executeInternalTool executes an internal tool (does not execute external commands)
func (e *Executor) executeInternalTool(ctx context.Context, toolName string, command string, args map[string]interface{}) (*mcp.ToolResult, error) {
	// extract internal tool type (remove "internal:" prefix)
	internalToolType := strings.TrimPrefix(command, "internal:")

	e.logger.Info("executing internal tool",
		zap.String("toolName", toolName),
		zap.String("internalToolType", internalToolType),
		zap.Any("args", args),
	)

	// dispatch based on internal tool type
	switch internalToolType {
	case "query_execution_result":
		return e.executeQueryExecutionResult(ctx, args)
	default:
		return &mcp.ToolResult{
			Content: []mcp.Content{
				{
					Type: "text",
					Text: fmt.Sprintf("Error: unknown internal tool type: %s", internalToolType),
				},
			},
			IsError: true,
		}, nil
	}
}

// executeQueryExecutionResult executes the query execution result tool
func (e *Executor) executeQueryExecutionResult(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
	// get execution_id parameter
	executionID, ok := args["execution_id"].(string)
	if !ok || executionID == "" {
		return &mcp.ToolResult{
			Content: []mcp.Content{
				{
					Type: "text",
					Text: "Error: execution_id parameter is required and cannot be empty",
				},
			},
			IsError: true,
		}, nil
	}

	// get optional parameters
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
		limit = 500 // limit maximum lines per page
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

	// check if result storage is available
	if e.resultStorage == nil {
		return &mcp.ToolResult{
			Content: []mcp.Content{
				{
					Type: "text",
					Text: "Error: result storage is not initialized",
				},
			},
			IsError: true,
		}, nil
	}

	// execute query
	var resultPage *storage.ResultPage
	var err error

	if search != "" {
		// search mode
		matchedLines, err := e.resultStorage.SearchResult(executionID, search, useRegex)
		if err != nil {
			return &mcp.ToolResult{
				Content: []mcp.Content{
					{
						Type: "text",
						Text: fmt.Sprintf("search failed: %v", err),
					},
				},
				IsError: true,
			}, nil
		}
		// paginate search results
		resultPage = paginateLines(matchedLines, page, limit)
	} else if filter != "" {
		// filter mode
		filteredLines, err := e.resultStorage.FilterResult(executionID, filter, useRegex)
		if err != nil {
			return &mcp.ToolResult{
				Content: []mcp.Content{
					{
						Type: "text",
						Text: fmt.Sprintf("filter failed: %v", err),
					},
				},
				IsError: true,
			}, nil
		}
		// paginate filter results
		resultPage = paginateLines(filteredLines, page, limit)
	} else {
		// normal paginated query
		resultPage, err = e.resultStorage.GetResultPage(executionID, page, limit)
		if err != nil {
			return &mcp.ToolResult{
				Content: []mcp.Content{
					{
						Type: "text",
						Text: fmt.Sprintf("query failed: %v", err),
					},
				},
				IsError: true,
			}, nil
		}
	}

	// get metadata
	metadata, err := e.resultStorage.GetResultMetadata(executionID)
	if err != nil {
		// metadata fetch failure does not affect query results
		e.logger.Warn("failed to get result metadata", zap.Error(err))
	}

	// format return result
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("query result (execution ID: %s)\n", executionID))

	if metadata != nil {
		sb.WriteString(fmt.Sprintf("tool: %s | size: %d bytes (%.2f KB) | total lines: %d\n",
			metadata.ToolName, metadata.TotalSize, float64(metadata.TotalSize)/1024, metadata.TotalLines))
	}

	sb.WriteString(fmt.Sprintf("page %d/%d, %d lines per page, total %d lines\n\n",
		resultPage.Page, resultPage.TotalPages, resultPage.Limit, resultPage.TotalLines))

	if len(resultPage.Lines) == 0 {
		sb.WriteString("no matching results found.\n")
	} else {
		for i, line := range resultPage.Lines {
			lineNum := (resultPage.Page-1)*resultPage.Limit + i + 1
			sb.WriteString(fmt.Sprintf("%d: %s\n", lineNum, line))
		}
	}

	sb.WriteString("\n")
	if resultPage.Page < resultPage.TotalPages {
		sb.WriteString(fmt.Sprintf("hint: use page=%d to view next page", resultPage.Page+1))
		if search != "" {
			sb.WriteString(fmt.Sprintf(", or use search=\"%s\" to continue searching", search))
			if useRegex {
				sb.WriteString(" (regex mode)")
			}
		}
		if filter != "" {
			sb.WriteString(fmt.Sprintf(", or use filter=\"%s\" to continue filtering", filter))
			if useRegex {
				sb.WriteString(" (regex mode)")
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

// paginateLines paginates a list of lines
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

// buildInputSchema builds the input schema
func (e *Executor) buildInputSchema(toolConfig *config.ToolConfig) map[string]interface{} {
	schema := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
		"required":   []string{},
	}

	// if parameters are defined in config, use them preferentially
	if len(toolConfig.Parameters) > 0 {
		properties := make(map[string]interface{})
		required := []string{}

		for _, param := range toolConfig.Parameters {
			// skip parameters with empty name (avoid illegal schema from name: null or empty in YAML)
			if strings.TrimSpace(param.Name) == "" {
				e.logger.Debug("skipping parameter with no name",
					zap.String("tool", toolConfig.Name),
					zap.String("type", param.Type),
				)
				continue
			}
			// convert types to OpenAI/JSON Schema standard types (empty type defaults to string)
			openAIType := e.convertToOpenAIType(param.Type)

			prop := map[string]interface{}{
				"type":        openAIType,
				"description": param.Description,
			}

			// add default value
			if param.Default != nil {
				prop["default"] = param.Default
			}

			// add enum options
			if len(param.Options) > 0 {
				prop["enum"] = param.Options
			}

			properties[param.Name] = prop

			// add to required parameters list
			if param.Required {
				required = append(required, param.Name)
			}
		}

		schema["properties"] = properties
		schema["required"] = required
		return schema
	}

	// if no parameter configuration defined, return empty schema
	// in this case the tool may only use fixed arguments (args field)
	// or parameters need to be defined via YAML config file
	e.logger.Warn("tool has no parameter configuration defined, returning empty schema",
		zap.String("tool", toolConfig.Name),
	)
	return schema
}

// convertToOpenAIType converts config types to OpenAI/JSON Schema standard types
func (e *Executor) convertToOpenAIType(configType string) string {
	// empty or null type defaults to string, to avoid illegal schema causing tool call failures
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
		// return original type by default, but log a warning
		e.logger.Warn("unknown parameter type, using original type",
			zap.String("type", configType),
		)
		return configType
	}
}

// getExitCode extracts exit code from error, returns nil if not an ExitError
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

// getExitCodeValue extracts exit code value from error, returns -1 if not an ExitError
func getExitCodeValue(err error) int {
	if code := getExitCode(err); code != nil {
		return *code
	}
	return -1
}
