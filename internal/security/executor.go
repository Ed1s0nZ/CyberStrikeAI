package security

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/mcp"
	"cyberstrike-ai/internal/storage"

	"go.uber.org/zap"
)

// Executor security tool executor
type Executor struct {
	config         *config.SecurityConfig
	toolIndex      map[string]*config.ToolConfig // tool index for O(1) lookup
	mcpServer      *mcp.Server
	logger         *zap.Logger
	resultStorage  ResultStorage // result storage (for query tools)
	defaultWorkDir string        // stable default working directory for tool execution
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
		config:         cfg,
		toolIndex:      make(map[string]*config.ToolConfig),
		mcpServer:      mcpServer,
		logger:         logger,
		resultStorage:  nil, // set later via SetResultStorage
		defaultWorkDir: detectDefaultToolWorkDir(),
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

	args = e.normalizeToolArgs(toolName, toolConfig, args)

	missingRequired := e.findMissingRequiredParams(toolConfig, args)
	if len(missingRequired) > 0 {
		e.logger.Warn("tool call missing required parameters",
			zap.String("toolName", toolName),
			zap.Strings("missing", missingRequired),
			zap.Any("args", args),
		)
		return &mcp.ToolResult{
			Content: []mcp.Content{
				{
					Type: "text",
					Text: fmt.Sprintf("Error: tool %s is missing required parameters: %s. Received parameters: %v",
						toolName, strings.Join(missingRequired, ", "), args),
				},
			},
			IsError: true,
		}, nil
	}

	// special handling: exec tool directly executes system commands
	if toolName == "exec" {
		e.logger.Info("executing exec tool")
		return e.executeSystemCommand(ctx, args)
	}

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
	if workDir := e.resolveToolWorkDir(args); workDir != "" {
		cmd.Dir = workDir
	}

	e.logger.Info("executing security tool",
		zap.String("tool", toolName),
		zap.Strings("args", cmdArgs),
		zap.String("workdir", cmd.Dir),
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

func (e *Executor) normalizeToolArgs(toolName string, toolConfig *config.ToolConfig, args map[string]interface{}) map[string]interface{} {
	if args == nil {
		args = map[string]interface{}{}
	}

	normalized := make(map[string]interface{}, len(args))
	for k, v := range args {
		normalized[k] = v
	}

	paramIndex := make(map[string]config.ParameterConfig, len(toolConfig.Parameters))
	knownKeys := make([]string, 0, len(toolConfig.Parameters))
	for _, param := range toolConfig.Parameters {
		paramIndex[param.Name] = param
		knownKeys = append(knownKeys, param.Name)
	}

	changed := false
	mergeParsed := func(parsed map[string]interface{}) {
		for key, value := range parsed {
			existing, exists := normalized[key]
			if !exists || isEmptyArgValue(existing) || looksMalformedArgValue(existing) {
				normalized[key] = value
				changed = true
			}
		}
	}

	if raw, ok := normalized["raw"].(string); ok && strings.TrimSpace(raw) != "" {
		if parsed := parseLooseToolArgs(raw, knownKeys, paramIndex); len(parsed) > 0 {
			mergeParsed(parsed)
			delete(normalized, "raw")
			changed = true
		}
	}

	for _, key := range knownKeys {
		rawValue, ok := normalized[key].(string)
		if !ok || !looksMalformedArgValue(rawValue) {
			continue
		}

		repairedCurrent, parsed, repaired := repairMalformedStringArg(key, rawValue, knownKeys, paramIndex)
		if !repaired {
			continue
		}

		if repairedCurrent != nil {
			normalized[key] = repairedCurrent
		} else {
			delete(normalized, key)
		}
		mergeParsed(parsed)
		changed = true
	}

	if applyToolArgumentHeuristics(normalized, paramIndex) {
		changed = true
	}

	for _, param := range toolConfig.Parameters {
		value, ok := normalized[param.Name]
		if !ok {
			continue
		}
		coerced, didCoerce := coerceToolArgValue(value, param)
		if didCoerce {
			normalized[param.Name] = coerced
			changed = true
		}
	}

	if changed {
		e.logger.Info("normalized malformed tool arguments",
			zap.String("toolName", toolName),
			zap.Any("originalArgs", args),
			zap.Any("normalizedArgs", normalized),
		)
	}

	return normalized
}

func (e *Executor) findMissingRequiredParams(toolConfig *config.ToolConfig, args map[string]interface{}) []string {
	if toolConfig == nil {
		return nil
	}

	missing := make([]string, 0)
	for _, param := range toolConfig.Parameters {
		if !param.Required {
			continue
		}
		value, exists := args[param.Name]
		if !exists || isMissingRequiredValue(value) {
			missing = append(missing, param.Name)
		}
	}
	return missing
}

func isMissingRequiredValue(value interface{}) bool {
	if value == nil {
		return true
	}

	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v) == ""
	case []interface{}:
		return len(v) == 0
	case []string:
		return len(v) == 0
	}

	return false
}

func isEmptyArgValue(value interface{}) bool {
	if value == nil {
		return true
	}
	if s, ok := value.(string); ok {
		return strings.TrimSpace(s) == ""
	}
	return false
}

func looksMalformedArgValue(value interface{}) bool {
	s, ok := value.(string)
	if !ok {
		return false
	}
	trimmed := strings.TrimSpace(s)
	return strings.Contains(trimmed, "<parameter=") ||
		strings.Contains(trimmed, "</parameter") ||
		strings.Contains(trimmed, `{"`) ||
		strings.Contains(trimmed, `"`)
}

func repairMalformedStringArg(currentKey, raw string, knownKeys []string, paramIndex map[string]config.ParameterConfig) (interface{}, map[string]interface{}, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil, false
	}

	markupIdx := firstStructuredMarkupIndex(trimmed)
	if markupIdx == -1 {
		return nil, nil, false
	}

	currentParam, hasParam := paramIndex[currentKey]
	var cleanedCurrent interface{}
	head := strings.TrimSpace(trimmed[:markupIdx])
	if head != "" {
		if hasParam {
			if value, ok := cleanLooseArgumentValue(head, &currentParam); ok {
				cleanedCurrent = value
			}
		} else {
			cleanedCurrent = head
		}
	}

	parsed := parseLooseToolArgs(trimmed[markupIdx:], knownKeys, paramIndex)
	return cleanedCurrent, parsed, len(parsed) > 0 || cleanedCurrent != nil
}

func firstStructuredMarkupIndex(raw string) int {
	markers := []string{"<parameter=", "</parameter", "{", `"`}
	first := -1
	for _, marker := range markers {
		idx := strings.Index(raw, marker)
		if idx == -1 {
			continue
		}
		if first == -1 || idx < first {
			first = idx
		}
	}
	return first
}

func parseLooseToolArgs(raw string, knownKeys []string, paramIndex map[string]config.ParameterConfig) map[string]interface{} {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || len(knownKeys) == 0 {
		return nil
	}

	var strict map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &strict); err == nil && len(strict) > 0 {
		return strict
	}

	keys := append([]string(nil), knownKeys...)
	sort.Slice(keys, func(i, j int) bool {
		return len(keys[i]) > len(keys[j])
	})

	escapedKeys := make([]string, 0, len(keys))
	for _, key := range keys {
		escapedKeys = append(escapedKeys, regexp.QuoteMeta(key))
	}
	pattern := regexp.MustCompile(`(?is)"?(` + strings.Join(escapedKeys, "|") + `)"?\s*[:=>]\s*`)
	matches := pattern.FindAllStringSubmatchIndex(trimmed, -1)
	if len(matches) == 0 {
		return nil
	}

	parsed := make(map[string]interface{})
	for i, match := range matches {
		if len(match) < 4 {
			continue
		}

		key := trimmed[match[2]:match[3]]
		valueStart := match[1]
		valueEnd := len(trimmed)
		if i+1 < len(matches) {
			valueEnd = matches[i+1][0]
		}

		param, ok := paramIndex[key]
		var paramPtr *config.ParameterConfig
		if ok {
			paramPtr = &param
		}

		if value, ok := cleanLooseArgumentValue(trimmed[valueStart:valueEnd], paramPtr); ok {
			parsed[key] = value
		}
	}

	if len(parsed) == 0 {
		return nil
	}
	return parsed
}

func cleanLooseArgumentValue(raw string, param *config.ParameterConfig) (interface{}, bool) {
	original := strings.TrimSpace(raw)
	if original == "" {
		if param != nil && param.Type == "bool" {
			return true, true
		}
		return "", false
	}

	explicitEmptyString := strings.HasPrefix(original, `""`) || strings.HasPrefix(original, `''`)

	cleaned := strings.TrimLeft(original, " \t\r\n>,")
	for {
		next := strings.TrimSpace(strings.TrimPrefix(cleaned, "</parameter"))
		next = strings.TrimLeft(next, "= \t\r\n")
		if next == cleaned {
			break
		}
		cleaned = next
	}
	cleaned = strings.TrimSpace(cleaned)

	for len(cleaned) > 0 {
		last := cleaned[len(cleaned)-1]
		if last == ',' || last == '}' || last == ']' {
			cleaned = strings.TrimSpace(cleaned[:len(cleaned)-1])
			continue
		}
		break
	}

	if explicitEmptyString {
		return "", true
	}

	if unquoted, err := strconv.Unquote(cleaned); err == nil {
		cleaned = unquoted
	}
	cleaned = strings.TrimSpace(cleaned)

	if cleaned == "" {
		if param != nil && param.Type == "bool" {
			return true, true
		}
		return "", false
	}

	if param != nil {
		switch param.Type {
		case "bool":
			switch strings.ToLower(cleaned) {
			case "true", "1", "yes", "on":
				return true, true
			case "false", "0", "no", "off":
				return false, true
			default:
				return true, true
			}
		case "int":
			if i, err := strconv.Atoi(cleaned); err == nil {
				return i, true
			}
		case "float":
			if f, err := strconv.ParseFloat(cleaned, 64); err == nil {
				return f, true
			}
		case "object":
			var obj map[string]interface{}
			if err := json.Unmarshal([]byte(cleaned), &obj); err == nil {
				return obj, true
			}
		case "array":
			var arr []interface{}
			if err := json.Unmarshal([]byte(cleaned), &arr); err == nil {
				return arr, true
			}
		}
	}

	switch strings.ToLower(cleaned) {
	case "true":
		return true, true
	case "false":
		return false, true
	}

	return cleaned, true
}

func coerceToolArgValue(value interface{}, param config.ParameterConfig) (interface{}, bool) {
	switch param.Type {
	case "bool":
		switch v := value.(type) {
		case bool:
			return value, false
		case string:
			switch strings.ToLower(strings.TrimSpace(v)) {
			case "true", "1", "yes", "on":
				return true, true
			case "false", "0", "no", "off":
				return false, true
			}
		case int:
			return v != 0, true
		case float64:
			return v != 0, true
		}
	case "int":
		switch v := value.(type) {
		case int:
			return value, false
		case float64:
			return int(v), true
		case string:
			if i, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
				return i, true
			}
		}
	case "float":
		switch v := value.(type) {
		case float64:
			return value, false
		case int:
			return float64(v), true
		case string:
			if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
				return f, true
			}
		}
	}
	return value, false
}

func applyToolArgumentHeuristics(args map[string]interface{}, paramIndex map[string]config.ParameterConfig) bool {
	changed := false

	if _, hasURLParam := paramIndex["url"]; hasURLParam && isMissingRequiredValue(args["url"]) {
		for _, candidateKey := range []string{"data", "target", "download"} {
			candidate, ok := args[candidateKey].(string)
			if ok && looksLikeURL(candidate) {
				args["url"] = strings.TrimSpace(candidate)
				changed = true
				break
			}
		}
	}

	if _, hasTargetParam := paramIndex["target"]; hasTargetParam && isMissingRequiredValue(args["target"]) {
		if urlValue, ok := args["url"].(string); ok && strings.TrimSpace(urlValue) != "" {
			args["target"] = strings.TrimSpace(urlValue)
			changed = true
		}
	}

	if _, hasDomainParam := paramIndex["domain"]; hasDomainParam && isMissingRequiredValue(args["domain"]) {
		for _, candidateKey := range []string{"url", "target"} {
			candidate, ok := args[candidateKey].(string)
			if !ok {
				continue
			}
			if domain := extractDomainCandidate(candidate); domain != "" {
				args["domain"] = domain
				changed = true
				break
			}
		}
	}

	if _, hasMethodParam := paramIndex["method"]; hasMethodParam && isMissingRequiredValue(args["method"]) {
		if action, ok := args["action"].(string); ok {
			method := strings.ToUpper(strings.TrimSpace(action))
			switch method {
			case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
				args["method"] = method
				changed = true
			}
		}
	}

	return changed
}

func looksLikeURL(value string) bool {
	trimmed := strings.TrimSpace(value)
	return strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://")
}

func extractDomainCandidate(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	trimmed = strings.TrimPrefix(trimmed, "http://")
	trimmed = strings.TrimPrefix(trimmed, "https://")
	if idx := strings.IndexAny(trimmed, "/?#"); idx >= 0 {
		trimmed = trimmed[:idx]
	}
	trimmed = strings.TrimSpace(trimmed)
	return strings.Trim(trimmed, "[]")
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
			if toolName == "nmap" {
				scanTypeValue = normalizeNmapScanType(scanType)
			}
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
		if toolName == "feroxbuster" {
			sanitized, changed := sanitizeFeroxbusterArgs(cmdArgs)
			if changed {
				e.logger.Warn("sanitized feroxbuster duplicate thread flags",
					zap.Strings("originalArgs", cmdArgs),
					zap.Strings("sanitizedArgs", sanitized),
				)
			}
			cmdArgs = sanitized
		}
		if toolName == "fierce" {
			sanitized, changed := sanitizeFierceArgs(cmdArgs)
			if changed {
				e.logger.Warn("sanitized fierce orphan list flags",
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
			if strings.EqualFold(strings.TrimSpace(formattedValue), "common") {
				// map friendly "common" alias used by prompts to a valid nmap port list
				formattedValue = "21,22,23,25,53,80,110,143,443,445,993,995,1433,1521,3306,3389,5432,6379,8080,8443"
			}
			// remove all spaces but preserve commas and other characters
			formattedValue = strings.ReplaceAll(formattedValue, " ", "")
		}
		return formattedValue
	}
}

func normalizeNmapScanType(scanType string) string {
	normalized := strings.TrimSpace(strings.ToLower(scanType))
	switch normalized {
	case "", "default":
		return ""
	case "fast":
		return "-T4"
	case "quick":
		return "-T4 -F"
	case "intensive":
		return "-T4 -A"
	default:
		// keep raw custom flags (e.g. "-sV -Pn")
		return scanType
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

// sanitizeFeroxbusterArgs removes duplicate thread flags (-t/--threads) and keeps only the
// last occurrence, as feroxbuster forbids specifying --threads multiple times.
func sanitizeFeroxbusterArgs(args []string) ([]string, bool) {
	type occ struct {
		start int
		end   int
	}

	occurrences := make([]occ, 0)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-t" || arg == "--threads":
			start := i
			end := i
			if i+1 < len(args) {
				end = i + 1
				i = end
			}
			occurrences = append(occurrences, occ{start: start, end: end})
		case strings.HasPrefix(arg, "--threads="):
			occurrences = append(occurrences, occ{start: i, end: i})
		}
	}

	if len(occurrences) <= 1 {
		return args, false
	}

	remove := make(map[int]struct{}, len(args))
	for _, o := range occurrences[:len(occurrences)-1] {
		for i := o.start; i <= o.end; i++ {
			remove[i] = struct{}{}
		}
	}

	sanitized := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		if _, ok := remove[i]; ok {
			continue
		}
		sanitized = append(sanitized, args[i])
	}

	return sanitized, true
}

// sanitizeFierceArgs removes fierce list-style flags when they have no value.
// Example invalid pattern: "--subdomains" with no following token.
func sanitizeFierceArgs(args []string) ([]string, bool) {
	requiresValue := map[string]struct{}{
		"--domain":         {},
		"--traverse":       {},
		"--range":          {},
		"--delay":          {},
		"--subdomain-file": {},
		"--dns-file":       {},
	}
	requiresAtLeastOneValue := map[string]struct{}{
		"--subdomains":  {},
		"--dns-servers": {},
		"--search":      {},
	}

	sanitized := make([]string, 0, len(args))
	changed := false

	for i := 0; i < len(args); i++ {
		arg := args[i]

		// Handle "--flag=value" forms; if value is empty, drop the flag.
		if strings.Contains(arg, "=") && strings.HasPrefix(arg, "--") {
			parts := strings.SplitN(arg, "=", 2)
			if len(parts) == 2 {
				key := parts[0]
				val := strings.TrimSpace(parts[1])
				if _, ok := requiresValue[key]; ok && val == "" {
					changed = true
					continue
				}
				if _, ok := requiresAtLeastOneValue[key]; ok && val == "" {
					changed = true
					continue
				}
			}
			sanitized = append(sanitized, arg)
			continue
		}

		if _, ok := requiresValue[arg]; ok {
			// Needs exactly one following value.
			if i+1 >= len(args) {
				changed = true
				continue
			}
			next := strings.TrimSpace(args[i+1])
			if next == "" || strings.HasPrefix(next, "--") {
				changed = true
				continue
			}
			sanitized = append(sanitized, arg, args[i+1])
			i++
			continue
		}

		if _, ok := requiresAtLeastOneValue[arg]; ok {
			// Needs at least one following value token.
			if i+1 >= len(args) {
				changed = true
				continue
			}
			next := strings.TrimSpace(args[i+1])
			if next == "" || strings.HasPrefix(next, "--") {
				changed = true
				continue
			}
			sanitized = append(sanitized, arg, args[i+1])
			i++
			continue
		}

		sanitized = append(sanitized, arg)
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
	workDir := e.resolveToolWorkDir(args)

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

// resolveToolWorkDir resolves the tool working directory.
// Priority: explicit args["workdir"] -> executor default workdir.
func (e *Executor) resolveToolWorkDir(args map[string]interface{}) string {
	base := e.defaultWorkDir
	if wd, ok := args["workdir"].(string); ok && strings.TrimSpace(wd) != "" {
		if filepath.IsAbs(wd) {
			return filepath.Clean(wd)
		}
		if base == "" {
			if cwd, err := os.Getwd(); err == nil {
				base = cwd
			}
		}
		if base != "" {
			return filepath.Clean(filepath.Join(base, wd))
		}
		return filepath.Clean(wd)
	}
	return base
}

// detectDefaultToolWorkDir determines a stable workspace directory for tool execution.
func detectDefaultToolWorkDir() string {
	if env := strings.TrimSpace(os.Getenv("CYBERSTRIKE_WORKDIR")); env != "" {
		if info, err := os.Stat(env); err == nil && info.IsDir() {
			return env
		}
	}

	if cwd, err := os.Getwd(); err == nil {
		if info, statErr := os.Stat(cwd); statErr == nil && info.IsDir() {
			return cwd
		}
	}

	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		if info, statErr := os.Stat(exeDir); statErr == nil && info.IsDir() {
			return exeDir
		}
	}

	return ""
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

	// execute query (prefer file storage; fallback to MCP execution record)
	var resultPage *storage.ResultPage
	var metadata *storage.ResultMetadata
	var storageErr error

	if e.resultStorage != nil {
		if search != "" {
			var matchedLines []string
			matchedLines, storageErr = e.resultStorage.SearchResult(executionID, search, useRegex)
			if storageErr == nil {
				resultPage = paginateLines(matchedLines, page, limit)
			}
		} else if filter != "" {
			var filteredLines []string
			filteredLines, storageErr = e.resultStorage.FilterResult(executionID, filter, useRegex)
			if storageErr == nil {
				resultPage = paginateLines(filteredLines, page, limit)
			}
		} else {
			resultPage, storageErr = e.resultStorage.GetResultPage(executionID, page, limit)
		}
		if storageErr == nil {
			metadata, _ = e.resultStorage.GetResultMetadata(executionID)
		}
	} else {
		storageErr = fmt.Errorf("result storage not initialized")
	}

	if storageErr != nil {
		e.logger.Warn("query_execution_result storage lookup failed, trying MCP execution fallback",
			zap.String("executionID", executionID),
			zap.Error(storageErr),
		)
		if e.mcpServer == nil {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: fmt.Sprintf("query failed: %v", storageErr)}},
				IsError: true,
			}, nil
		}

		execRec, exists := e.mcpServer.GetExecution(executionID)
		if !exists || execRec == nil {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: fmt.Sprintf("query failed: %v", storageErr)}},
				IsError: true,
			}, nil
		}

		var sb strings.Builder
		if execRec.Result != nil {
			for _, content := range execRec.Result.Content {
				sb.WriteString(content.Text)
				sb.WriteString("\n")
			}
		}
		if sb.Len() == 0 && execRec.Error != "" {
			sb.WriteString(execRec.Error)
		}
		raw := sb.String()
		if raw == "" {
			raw = "(no output)"
		}
		lines := strings.Split(raw, "\n")
		if search != "" {
			matchedLines, ferr := filterLines(lines, search, useRegex)
			if ferr != nil {
				return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: fmt.Sprintf("search failed: %v", ferr)}}, IsError: true}, nil
			}
			resultPage = paginateLines(matchedLines, page, limit)
		} else if filter != "" {
			filteredLines, ferr := filterLines(lines, filter, useRegex)
			if ferr != nil {
				return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: fmt.Sprintf("filter failed: %v", ferr)}}, IsError: true}, nil
			}
			resultPage = paginateLines(filteredLines, page, limit)
		} else {
			resultPage = paginateLines(lines, page, limit)
		}
		metadata = &storage.ResultMetadata{
			ExecutionID: executionID,
			ToolName:    execRec.ToolName,
			TotalSize:   len(raw),
			TotalLines:  len(lines),
		}
	}

	// format return result
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("query result (execution ID: %s)\n", executionID))

	if metadata != nil {
		sb.WriteString(fmt.Sprintf("tool: %s | size: %d bytes (%.2f KB) | total lines: %d\n",
			metadata.ToolName, metadata.TotalSize, float64(metadata.TotalSize)/1024, metadata.TotalLines))
	}

	sb.WriteString(fmt.Sprintf("Page %d/%d, %d lines per page, total %d lines\n\n",
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

func filterLines(lines []string, pattern string, useRegex bool) ([]string, error) {
	if !useRegex {
		matched := make([]string, 0)
		for _, line := range lines {
			if strings.Contains(line, pattern) {
				matched = append(matched, line)
			}
		}
		return matched, nil
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regular expression: %w", err)
	}
	matched := make([]string, 0)
	for _, line := range lines {
		if re.MatchString(line) {
			matched = append(matched, line)
		}
	}
	return matched, nil
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
