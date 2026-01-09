package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ExternalMCPClient 外部MCP客户端接口
type ExternalMCPClient interface {
	// Initialize 初始化连接
	Initialize(ctx context.Context) error
	// ListTools 列出工具
	ListTools(ctx context.Context) ([]Tool, error)
	// CallTool 调用工具
	CallTool(ctx context.Context, name string, args map[string]interface{}) (*ToolResult, error)
	// Close 关闭连接
	Close() error
	// IsConnected 检查是否已连接
	IsConnected() bool
	// GetStatus 获取状态
	GetStatus() string
}

// HTTPMCPClient HTTP模式的MCP客户端
type HTTPMCPClient struct {
	url     string
	timeout time.Duration
	client  *http.Client
	logger  *zap.Logger
	mu      sync.RWMutex
	status  string // "disconnected", "connecting", "connected", "error"
}

// NewHTTPMCPClient 创建HTTP模式的MCP客户端
func NewHTTPMCPClient(url string, timeout time.Duration, logger *zap.Logger) *HTTPMCPClient {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &HTTPMCPClient{
		url:     url,
		timeout: timeout,
		client: &http.Client{
			Timeout: timeout,
		},
		logger: logger,
		status: "disconnected",
	}
}

func (c *HTTPMCPClient) setStatus(status string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status = status
}

func (c *HTTPMCPClient) GetStatus() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

func (c *HTTPMCPClient) IsConnected() bool {
	return c.GetStatus() == "connected"
}

func (c *HTTPMCPClient) Initialize(ctx context.Context) error {
	c.setStatus("connecting")

	req := Message{
		ID:      MessageID{value: "1"},
		Method:  "initialize",
		Version: "2.0",
	}

	params := InitializeRequest{
		ProtocolVersion: ProtocolVersion,
		Capabilities:    make(map[string]interface{}),
		ClientInfo: ClientInfo{
			Name:    "CyberStrikeAI",
			Version: "1.0.0",
		},
	}

	paramsJSON, _ := json.Marshal(params)
	req.Params = paramsJSON

	_, err := c.sendRequest(ctx, &req)
	if err != nil {
		c.setStatus("error")
		return fmt.Errorf("初始化失败: %w", err)
	}

	// 发送 initialized 通知（MCP 协议要求：收到 initialize 响应后必须发送此通知）
	notifyReq := Message{
		ID:      MessageID{value: nil}, // 通知没有 ID
		Method:  "notifications/initialized",
		Version: "2.0",
	}
	notifyReq.Params = json.RawMessage("{}")

	// 发送通知（不需要等待响应）
	if err := c.sendNotification(&notifyReq); err != nil {
		c.logger.Warn("发送 initialized 通知失败", zap.Error(err))
		// 通知失败不应该导致初始化失败，只记录警告
	}

	c.setStatus("connected")
	return nil
}

func (c *HTTPMCPClient) ListTools(ctx context.Context) ([]Tool, error) {
	req := Message{
		ID:      MessageID{value: uuid.New().String()},
		Method:  "tools/list",
		Version: "2.0",
	}

	req.Params = json.RawMessage("{}")

	resp, err := c.sendRequest(ctx, &req)
	if err != nil {
		return nil, fmt.Errorf("获取工具列表失败: %w", err)
	}

	var listResp ListToolsResponse
	if err := json.Unmarshal(resp.Result, &listResp); err != nil {
		return nil, fmt.Errorf("解析工具列表失败: %w", err)
	}

	return listResp.Tools, nil
}

func (c *HTTPMCPClient) CallTool(ctx context.Context, name string, args map[string]interface{}) (*ToolResult, error) {
	req := Message{
		ID:      MessageID{value: uuid.New().String()},
		Method:  "tools/call",
		Version: "2.0",
	}

	callReq := CallToolRequest{
		Name:      name,
		Arguments: args,
	}

	paramsJSON, _ := json.Marshal(callReq)
	req.Params = paramsJSON

	resp, err := c.sendRequest(ctx, &req)
	if err != nil {
		return nil, fmt.Errorf("调用工具失败: %w", err)
	}

	var callResp CallToolResponse
	if err := json.Unmarshal(resp.Result, &callResp); err != nil {
		return nil, fmt.Errorf("解析工具调用结果失败: %w", err)
	}

	return &ToolResult{
		Content: callResp.Content,
		IsError: callResp.IsError,
	}, nil
}

func (c *HTTPMCPClient) sendRequest(ctx context.Context, msg *Message) (*Message, error) {
	body, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP错误 %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var mcpResp Message
	if err := json.NewDecoder(resp.Body).Decode(&mcpResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if mcpResp.Error != nil {
		return nil, fmt.Errorf("MCP错误: %s (code: %d)", mcpResp.Error.Message, mcpResp.Error.Code)
	}

	return &mcpResp, nil
}

func (c *HTTPMCPClient) sendNotification(msg *Message) error {
	// 通知没有 ID，不需要等待响应
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("序列化通知失败: %w", err)
	}

	// 使用较短的超时发送通知
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// 发送通知，不等待响应（通知不需要响应）
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("发送通知失败: %w", err)
	}
	resp.Body.Close()

	return nil
}

func (c *HTTPMCPClient) Close() error {
	c.setStatus("disconnected")
	return nil
}

// StdioMCPClient stdio模式的MCP客户端
type StdioMCPClient struct {
	command     string
	args        []string
	env         map[string]string
	timeout     time.Duration
	cmd         *exec.Cmd
	stdin       io.WriteCloser
	stdout      io.ReadCloser
	decoder     *json.Decoder
	encoder     *json.Encoder
	logger      *zap.Logger
	mu          sync.RWMutex
	status      string
	requestID   int64
	responses   map[string]chan *Message
	responsesMu sync.Mutex
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewStdioMCPClient 创建stdio模式的MCP客户端
func NewStdioMCPClient(command string, args []string, env map[string]string, timeout time.Duration, logger *zap.Logger) *StdioMCPClient {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &StdioMCPClient{
		command:   command,
		args:      args,
		env:       env,
		timeout:   timeout,
		logger:    logger,
		status:    "disconnected",
		responses: make(map[string]chan *Message),
		ctx:       ctx,
		cancel:    cancel,
	}
}

func (c *StdioMCPClient) setStatus(status string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status = status
}

func (c *StdioMCPClient) GetStatus() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

func (c *StdioMCPClient) IsConnected() bool {
	return c.GetStatus() == "connected"
}

func (c *StdioMCPClient) Initialize(ctx context.Context) error {
	c.setStatus("connecting")

	if err := c.startProcess(); err != nil {
		c.setStatus("error")
		return fmt.Errorf("启动进程失败: %w", err)
	}

	// 启动响应读取goroutine
	go c.readResponses()

	// 发送初始化请求
	req := Message{
		ID:      MessageID{value: "1"},
		Method:  "initialize",
		Version: "2.0",
	}

	params := InitializeRequest{
		ProtocolVersion: ProtocolVersion,
		Capabilities:    make(map[string]interface{}),
		ClientInfo: ClientInfo{
			Name:    "CyberStrikeAI",
			Version: "1.0.0",
		},
	}

	paramsJSON, _ := json.Marshal(params)
	req.Params = paramsJSON

	_, err := c.sendRequest(ctx, &req)
	if err != nil {
		c.setStatus("error")
		c.Close()
		return fmt.Errorf("初始化失败: %w", err)
	}

	// 发送 initialized 通知（MCP 协议要求：收到 initialize 响应后必须发送此通知）
	notifyReq := Message{
		ID:      MessageID{value: nil}, // 通知没有 ID
		Method:  "notifications/initialized",
		Version: "2.0",
	}
	notifyReq.Params = json.RawMessage("{}")

	// 发送通知（不需要等待响应）
	if err := c.sendNotification(&notifyReq); err != nil {
		c.logger.Warn("发送 initialized 通知失败", zap.Error(err))
		// 通知失败不应该导致初始化失败，只记录警告
	}

	c.setStatus("connected")
	return nil
}

func (c *StdioMCPClient) startProcess() error {
	cmd := exec.CommandContext(c.ctx, c.command, c.args...)

	// 设置环境变量
	if c.env != nil && len(c.env) > 0 {
		// 获取当前环境变量
		cmd.Env = os.Environ()
		// 添加或覆盖配置的环境变量
		for key, value := range c.env {
			// 检查是否已存在该环境变量
			found := false
			for i, envVar := range cmd.Env {
				if strings.HasPrefix(envVar, key+"=") {
					cmd.Env[i] = key + "=" + value
					found = true
					break
				}
			}
			if !found {
				cmd.Env = append(cmd.Env, key+"="+value)
			}
		}
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return err
	}

	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		return err
	}

	c.cmd = cmd
	c.stdin = stdin
	c.stdout = stdout
	c.decoder = json.NewDecoder(stdout)
	c.encoder = json.NewEncoder(stdin)

	return nil
}

func (c *StdioMCPClient) readResponses() {
	defer func() {
		if r := recover(); r != nil {
			c.logger.Error("读取响应时发生panic", zap.Any("error", r))
		}
	}()

	for {
		var msg Message
		if err := c.decoder.Decode(&msg); err != nil {
			if err == io.EOF {
				c.setStatus("disconnected")
				break
			}
			c.logger.Error("读取响应失败", zap.Error(err))
			break
		}

		// 处理响应
		id := msg.ID.String()
		c.responsesMu.Lock()
		if ch, ok := c.responses[id]; ok {
			select {
			case ch <- &msg:
			default:
			}
			delete(c.responses, id)
		}
		c.responsesMu.Unlock()
	}
}

func (c *StdioMCPClient) sendRequest(ctx context.Context, msg *Message) (*Message, error) {
	if c.encoder == nil {
		return nil, fmt.Errorf("进程未启动")
	}

	id := msg.ID.String()
	if id == "" {
		c.mu.Lock()
		c.requestID++
		id = fmt.Sprintf("%d", c.requestID)
		msg.ID = MessageID{value: id}
		c.mu.Unlock()
	}

	// 创建响应通道
	responseCh := make(chan *Message, 1)
	c.responsesMu.Lock()
	c.responses[id] = responseCh
	c.responsesMu.Unlock()

	// 发送请求
	if err := c.encoder.Encode(msg); err != nil {
		c.responsesMu.Lock()
		delete(c.responses, id)
		c.responsesMu.Unlock()
		return nil, fmt.Errorf("发送请求失败: %w", err)
	}

	// 等待响应
	select {
	case resp := <-responseCh:
		if resp.Error != nil {
			return nil, fmt.Errorf("MCP错误: %s (code: %d)", resp.Error.Message, resp.Error.Code)
		}
		return resp, nil
	case <-ctx.Done():
		c.responsesMu.Lock()
		delete(c.responses, id)
		c.responsesMu.Unlock()
		return nil, ctx.Err()
	case <-time.After(c.timeout):
		c.responsesMu.Lock()
		delete(c.responses, id)
		c.responsesMu.Unlock()
		return nil, fmt.Errorf("请求超时")
	}
}

func (c *StdioMCPClient) ListTools(ctx context.Context) ([]Tool, error) {
	req := Message{
		ID:      MessageID{value: uuid.New().String()},
		Method:  "tools/list",
		Version: "2.0",
	}

	req.Params = json.RawMessage("{}")

	resp, err := c.sendRequest(ctx, &req)
	if err != nil {
		return nil, fmt.Errorf("获取工具列表失败: %w", err)
	}

	var listResp ListToolsResponse
	if err := json.Unmarshal(resp.Result, &listResp); err != nil {
		return nil, fmt.Errorf("解析工具列表失败: %w", err)
	}

	return listResp.Tools, nil
}

func (c *StdioMCPClient) sendNotification(msg *Message) error {
	// 通知没有 ID，不需要等待响应
	if c.encoder == nil {
		return fmt.Errorf("进程未启动")
	}

	// 直接发送通知，不等待响应
	if err := c.encoder.Encode(msg); err != nil {
		return fmt.Errorf("发送通知失败: %w", err)
	}

	return nil
}

func (c *StdioMCPClient) CallTool(ctx context.Context, name string, args map[string]interface{}) (*ToolResult, error) {
	req := Message{
		ID:      MessageID{value: uuid.New().String()},
		Method:  "tools/call",
		Version: "2.0",
	}

	callReq := CallToolRequest{
		Name:      name,
		Arguments: args,
	}

	paramsJSON, _ := json.Marshal(callReq)
	req.Params = paramsJSON

	resp, err := c.sendRequest(ctx, &req)
	if err != nil {
		return nil, fmt.Errorf("调用工具失败: %w", err)
	}

	var callResp CallToolResponse
	if err := json.Unmarshal(resp.Result, &callResp); err != nil {
		return nil, fmt.Errorf("解析工具调用结果失败: %w", err)
	}

	return &ToolResult{
		Content: callResp.Content,
		IsError: callResp.IsError,
	}, nil
}

func (c *StdioMCPClient) Close() error {
	c.cancel()

	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.stdout != nil {
		c.stdout.Close()
	}
	if c.cmd != nil {
		c.cmd.Process.Kill()
		c.cmd.Wait()
	}

	c.setStatus("disconnected")
	return nil
}

// SSEMCPClient SSE模式的MCP客户端
type SSEMCPClient struct {
	url         string
	timeout     time.Duration
	client      *http.Client
	logger      *zap.Logger
	mu          sync.RWMutex
	status      string // "disconnected", "connecting", "connected", "error"
	sseConn     io.ReadCloser
	sseCancel   context.CancelFunc
	requestID   int64
	responses   map[string]chan *Message
	responsesMu sync.Mutex
	ctx         context.Context
}

// NewSSEMCPClient 创建SSE模式的MCP客户端
func NewSSEMCPClient(url string, timeout time.Duration, logger *zap.Logger) *SSEMCPClient {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &SSEMCPClient{
		url:       url,
		timeout:   timeout,
		client:    &http.Client{Timeout: timeout},
		logger:    logger,
		status:    "disconnected",
		responses: make(map[string]chan *Message),
		ctx:       ctx,
		sseCancel: cancel,
	}
}

func (c *SSEMCPClient) setStatus(status string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status = status
}

func (c *SSEMCPClient) GetStatus() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

func (c *SSEMCPClient) IsConnected() bool {
	return c.GetStatus() == "connected"
}

func (c *SSEMCPClient) Initialize(ctx context.Context) error {
	c.setStatus("connecting")

	// 建立SSE连接
	if err := c.connectSSE(); err != nil {
		c.setStatus("error")
		return fmt.Errorf("建立SSE连接失败: %w", err)
	}

	// 启动响应读取goroutine
	go c.readSSEResponses()

	// 发送初始化请求
	req := Message{
		ID:      MessageID{value: "1"},
		Method:  "initialize",
		Version: "2.0",
	}

	params := InitializeRequest{
		ProtocolVersion: ProtocolVersion,
		Capabilities:    make(map[string]interface{}),
		ClientInfo: ClientInfo{
			Name:    "CyberStrikeAI",
			Version: "1.0.0",
		},
	}

	paramsJSON, _ := json.Marshal(params)
	req.Params = paramsJSON

	_, err := c.sendRequest(ctx, &req)
	if err != nil {
		c.setStatus("error")
		c.Close()
		return fmt.Errorf("初始化失败: %w", err)
	}

	// 发送 initialized 通知（MCP 协议要求：收到 initialize 响应后必须发送此通知）
	notifyReq := Message{
		ID:      MessageID{value: nil}, // 通知没有 ID
		Method:  "notifications/initialized",
		Version: "2.0",
	}
	notifyReq.Params = json.RawMessage("{}")

	// 发送通知（不需要等待响应）
	if err := c.sendNotification(&notifyReq); err != nil {
		c.logger.Warn("发送 initialized 通知失败", zap.Error(err))
		// 通知失败不应该导致初始化失败，只记录警告
	}

	c.setStatus("connected")
	return nil
}

func (c *SSEMCPClient) connectSSE() error {
	// 建立SSE连接（GET请求，Accept: text/event-stream）
	// SSE连接需要长连接，使用无超时的客户端
	sseClient := &http.Client{
		Timeout: 0, // 无超时，用于长连接
	}

	req, err := http.NewRequestWithContext(c.ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return fmt.Errorf("创建SSE请求失败: %w", err)
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	resp, err := sseClient.Do(req)
	if err != nil {
		return fmt.Errorf("SSE连接失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return fmt.Errorf("SSE连接失败，状态码: %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/event-stream") {
		resp.Body.Close()
		return fmt.Errorf("服务器不支持SSE，Content-Type: %s", contentType)
	}

	c.sseConn = resp.Body
	return nil
}

func (c *SSEMCPClient) readSSEResponses() {
	defer func() {
		if r := recover(); r != nil {
			c.logger.Error("读取SSE响应时发生panic", zap.Any("error", r))
		}
		if c.sseConn != nil {
			c.sseConn.Close()
		}
		c.setStatus("disconnected")
	}()

	if c.sseConn == nil {
		return
	}

	scanner := &sseScanner{reader: bufio.NewReader(c.sseConn)}

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		// 读取SSE事件
		event, err := scanner.readEvent()
		if err != nil {
			if err == io.EOF {
				c.setStatus("disconnected")
				return
			}
			c.logger.Error("读取SSE数据失败", zap.Error(err))
			return
		}

		if event == nil || len(event.Data) == 0 {
			continue
		}

		// 解析JSON消息
		var msg Message
		if err := json.Unmarshal(event.Data, &msg); err != nil {
			c.logger.Warn("解析SSE消息失败", zap.Error(err), zap.String("data", string(event.Data)))
			continue
		}

		// 处理响应
		id := msg.ID.String()
		c.responsesMu.Lock()
		if ch, ok := c.responses[id]; ok {
			select {
			case ch <- &msg:
			default:
			}
			delete(c.responses, id)
		}
		c.responsesMu.Unlock()
	}
}

// sseEvent SSE事件
type sseEvent struct {
	Event string
	Data  []byte
	ID    string
	Retry int
}

// sseScanner SSE扫描器
type sseScanner struct {
	reader *bufio.Reader
}

func (s *sseScanner) readEvent() (*sseEvent, error) {
	event := &sseEvent{}

	for {
		line, err := s.reader.ReadString('\n')
		if err != nil {
			return nil, err
		}

		line = strings.TrimRight(line, "\r\n")

		// 空行表示事件结束
		if len(line) == 0 {
			if len(event.Data) > 0 {
				return event, nil
			}
			continue
		}

		// 解析SSE行
		if strings.HasPrefix(line, "event: ") {
			event.Event = strings.TrimSpace(line[7:])
		} else if strings.HasPrefix(line, "data: ") {
			data := []byte(strings.TrimSpace(line[6:]))
			if len(event.Data) > 0 {
				event.Data = append(event.Data, '\n')
			}
			event.Data = append(event.Data, data...)
		} else if strings.HasPrefix(line, "id: ") {
			event.ID = strings.TrimSpace(line[4:])
		} else if strings.HasPrefix(line, "retry: ") {
			fmt.Sscanf(line[7:], "%d", &event.Retry)
		}
	}
}

func (c *SSEMCPClient) sendRequest(ctx context.Context, msg *Message) (*Message, error) {
	if c.sseConn == nil {
		return nil, fmt.Errorf("SSE连接未建立")
	}

	id := msg.ID.String()
	if id == "" {
		c.mu.Lock()
		c.requestID++
		id = fmt.Sprintf("%d", c.requestID)
		msg.ID = MessageID{value: id}
		c.mu.Unlock()
	}

	// 创建响应通道
	responseCh := make(chan *Message, 1)
	c.responsesMu.Lock()
	c.responses[id] = responseCh
	c.responsesMu.Unlock()

	// 通过HTTP POST发送请求（SSE用于接收响应，请求通过POST发送）
	body, err := json.Marshal(msg)
	if err != nil {
		c.responsesMu.Lock()
		delete(c.responses, id)
		c.responsesMu.Unlock()
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	// 使用POST请求发送消息（通常SSE服务器会提供两个端点：一个用于SSE，一个用于POST）
	// 如果URL是SSE端点，尝试使用相同的URL但改为POST，或者使用URL + "/message"
	postURL := c.url
	if strings.HasSuffix(postURL, "/sse") {
		postURL = strings.TrimSuffix(postURL, "/sse")
		postURL += "/message"
	} else if strings.HasSuffix(postURL, "/events") {
		postURL = strings.TrimSuffix(postURL, "/events")
		postURL += "/message"
	} else if !strings.Contains(postURL, "/message") {
		// 如果URL不包含/message，尝试添加
		postURL = strings.TrimSuffix(postURL, "/")
		postURL += "/message"
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, postURL, bytes.NewReader(body))
	if err != nil {
		c.responsesMu.Lock()
		delete(c.responses, id)
		c.responsesMu.Unlock()
		return nil, fmt.Errorf("创建POST请求失败: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		c.responsesMu.Lock()
		delete(c.responses, id)
		c.responsesMu.Unlock()
		return nil, fmt.Errorf("发送POST请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 如果POST请求直接返回响应（非SSE模式），直接解析
	if resp.StatusCode == http.StatusOK && resp.Header.Get("Content-Type") == "application/json" {
		var mcpResp Message
		if err := json.NewDecoder(resp.Body).Decode(&mcpResp); err != nil {
			c.responsesMu.Lock()
			delete(c.responses, id)
			c.responsesMu.Unlock()
			return nil, fmt.Errorf("解析响应失败: %w", err)
		}

		if mcpResp.Error != nil {
			c.responsesMu.Lock()
			delete(c.responses, id)
			c.responsesMu.Unlock()
			return nil, fmt.Errorf("MCP错误: %s (code: %d)", mcpResp.Error.Message, mcpResp.Error.Code)
		}

		return &mcpResp, nil
	}

	// 否则等待SSE响应
	select {
	case resp := <-responseCh:
		if resp.Error != nil {
			return nil, fmt.Errorf("MCP错误: %s (code: %d)", resp.Error.Message, resp.Error.Code)
		}
		return resp, nil
	case <-ctx.Done():
		c.responsesMu.Lock()
		delete(c.responses, id)
		c.responsesMu.Unlock()
		return nil, ctx.Err()
	case <-time.After(c.timeout):
		c.responsesMu.Lock()
		delete(c.responses, id)
		c.responsesMu.Unlock()
		return nil, fmt.Errorf("请求超时")
	}
}

func (c *SSEMCPClient) ListTools(ctx context.Context) ([]Tool, error) {
	req := Message{
		ID:      MessageID{value: uuid.New().String()},
		Method:  "tools/list",
		Version: "2.0",
	}

	req.Params = json.RawMessage("{}")

	resp, err := c.sendRequest(ctx, &req)
	if err != nil {
		return nil, fmt.Errorf("获取工具列表失败: %w", err)
	}

	var listResp ListToolsResponse
	if err := json.Unmarshal(resp.Result, &listResp); err != nil {
		return nil, fmt.Errorf("解析工具列表失败: %w", err)
	}

	return listResp.Tools, nil
}

func (c *SSEMCPClient) sendNotification(msg *Message) error {
	// 通知没有 ID，不需要等待响应
	if c.sseConn == nil {
		return fmt.Errorf("SSE连接未建立")
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("序列化通知失败: %w", err)
	}

	// 使用 POST 发送通知（与 sendRequest 类似的逻辑）
	postURL := c.url
	if strings.HasSuffix(postURL, "/sse") {
		postURL = strings.TrimSuffix(postURL, "/sse")
		postURL += "/message"
	} else if strings.HasSuffix(postURL, "/events") {
		postURL = strings.TrimSuffix(postURL, "/events")
		postURL += "/message"
	} else if !strings.Contains(postURL, "/message") {
		postURL = strings.TrimSuffix(postURL, "/")
		postURL += "/message"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, postURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("创建POST请求失败: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// 发送通知，不等待响应（通知不需要响应）
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("发送通知失败: %w", err)
	}
	resp.Body.Close()

	return nil
}

func (c *SSEMCPClient) CallTool(ctx context.Context, name string, args map[string]interface{}) (*ToolResult, error) {
	req := Message{
		ID:      MessageID{value: uuid.New().String()},
		Method:  "tools/call",
		Version: "2.0",
	}

	callReq := CallToolRequest{
		Name:      name,
		Arguments: args,
	}

	paramsJSON, _ := json.Marshal(callReq)
	req.Params = paramsJSON

	resp, err := c.sendRequest(ctx, &req)
	if err != nil {
		return nil, fmt.Errorf("调用工具失败: %w", err)
	}

	var callResp CallToolResponse
	if err := json.Unmarshal(resp.Result, &callResp); err != nil {
		return nil, fmt.Errorf("解析工具调用结果失败: %w", err)
	}

	return &ToolResult{
		Content: callResp.Content,
		IsError: callResp.IsError,
	}, nil
}

func (c *SSEMCPClient) Close() error {
	c.sseCancel()

	if c.sseConn != nil {
		c.sseConn.Close()
		c.sseConn = nil
	}

	c.setStatus("disconnected")
	return nil
}
