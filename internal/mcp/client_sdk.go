// Package mcp provides the external MCP client - implemented based on the official go-sdk to ensure protocol compatibility
package mcp

import (
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

	"cyberstrike-ai/internal/config"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
)

const (
	clientName    = "CyberStrikeAI"
	clientVersion = "1.0.0"
)

// sdkClient is the external MCP client based on the official MCP Go SDK, implementing the ExternalMCPClient interface
type sdkClient struct {
	session *mcp.ClientSession
	client  *mcp.Client
	logger  *zap.Logger
	mu      sync.RWMutex
	status  string // "disconnected", "connecting", "connected", "error"
}

// newSDKClientFromSession constructs from an already-connected session (used internally by createSDKClient)
func newSDKClientFromSession(session *mcp.ClientSession, client *mcp.Client, logger *zap.Logger) *sdkClient {
	return &sdkClient{
		session: session,
		client:  client,
		logger:  logger,
		status:  "connected",
	}
}

// lazySDKClient defers connection: calls the official SDK to establish connection only during Initialize(),
// implements ExternalMCPClient externally
type lazySDKClient struct {
	serverCfg config.ExternalMCPServerConfig
	logger    *zap.Logger
	inner     ExternalMCPClient // *sdkClient after successful connection
	mu        sync.RWMutex
	status    string
}

func newLazySDKClient(serverCfg config.ExternalMCPServerConfig, logger *zap.Logger) *lazySDKClient {
	return &lazySDKClient{
		serverCfg: serverCfg,
		logger:    logger,
		status:    "connecting",
	}
}

func (c *lazySDKClient) setStatus(s string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status = s
}

func (c *lazySDKClient) GetStatus() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.inner != nil {
		return c.inner.GetStatus()
	}
	return c.status
}

func (c *lazySDKClient) IsConnected() bool {
	c.mu.RLock()
	inner := c.inner
	c.mu.RUnlock()
	if inner != nil {
		return inner.IsConnected()
	}
	return false
}

func (c *lazySDKClient) Initialize(ctx context.Context) error {
	c.mu.Lock()
	if c.inner != nil {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	inner, err := createSDKClient(ctx, c.serverCfg, c.logger)
	if err != nil {
		c.setStatus("error")
		return err
	}

	c.mu.Lock()
	c.inner = inner
	c.mu.Unlock()
	c.setStatus("connected")
	return nil
}

func (c *lazySDKClient) ListTools(ctx context.Context) ([]Tool, error) {
	c.mu.RLock()
	inner := c.inner
	c.mu.RUnlock()
	if inner == nil {
		return nil, fmt.Errorf("not connected")
	}
	return inner.ListTools(ctx)
}

func (c *lazySDKClient) CallTool(ctx context.Context, name string, args map[string]interface{}) (*ToolResult, error) {
	c.mu.RLock()
	inner := c.inner
	c.mu.RUnlock()
	if inner == nil {
		return nil, fmt.Errorf("not connected")
	}
	return inner.CallTool(ctx, name, args)
}

func (c *lazySDKClient) Close() error {
	c.mu.Lock()
	inner := c.inner
	c.inner = nil
	c.mu.Unlock()
	c.setStatus("disconnected")
	if inner != nil {
		return inner.Close()
	}
	return nil
}

func (c *sdkClient) setStatus(s string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status = s
}

func (c *sdkClient) GetStatus() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

func (c *sdkClient) IsConnected() bool {
	return c.GetStatus() == "connected"
}

func (c *sdkClient) Initialize(ctx context.Context) error {
	// sdkClient is created only after a successful Connect in createSDKClient, so it is already connected at Initialize time
	// this method only exists to satisfy the ExternalMCPClient interface; actual connection is done in createSDKClient
	return nil
}

func (c *sdkClient) ListTools(ctx context.Context) ([]Tool, error) {
	if c.session == nil {
		return nil, fmt.Errorf("not connected")
	}
	res, err := c.session.ListTools(ctx, nil)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, nil
	}
	return sdkToolsToOur(res.Tools), nil
}

func (c *sdkClient) CallTool(ctx context.Context, name string, args map[string]interface{}) (*ToolResult, error) {
	if c.session == nil {
		return nil, fmt.Errorf("not connected")
	}
	params := &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	}
	res, err := c.session.CallTool(ctx, params)
	if err != nil {
		return nil, err
	}
	return sdkCallToolResultToOurs(res), nil
}

func (c *sdkClient) Close() error {
	c.setStatus("disconnected")
	if c.session != nil {
		err := c.session.Close()
		c.session = nil
		return err
	}
	return nil
}

// sdkToolsToOur converts SDK []*mcp.Tool to our []Tool
func sdkToolsToOur(tools []*mcp.Tool) []Tool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]Tool, 0, len(tools))
	for _, t := range tools {
		if t == nil {
			continue
		}
		schema := make(map[string]interface{})
		if t.InputSchema != nil {
			// SDK InputSchema may be *jsonschema.Schema or map, normalize to map
			if m, ok := t.InputSchema.(map[string]interface{}); ok {
				schema = m
			} else {
				_ = json.Unmarshal(mustJSON(t.InputSchema), &schema)
			}
		}
		desc := t.Description
		shortDesc := desc
		if t.Annotations != nil && t.Annotations.Title != "" {
			shortDesc = t.Annotations.Title
		}
		out = append(out, Tool{
			Name:             t.Name,
			Description:      desc,
			ShortDescription: shortDesc,
			InputSchema:      schema,
		})
	}
	return out
}

// sdkCallToolResultToOurs converts SDK *mcp.CallToolResult to our *ToolResult
func sdkCallToolResultToOurs(res *mcp.CallToolResult) *ToolResult {
	if res == nil {
		return &ToolResult{Content: []Content{}}
	}
	content := sdkContentToOurs(res.Content)
	return &ToolResult{
		Content: content,
		IsError: res.IsError,
	}
}

func sdkContentToOurs(list []mcp.Content) []Content {
	if len(list) == 0 {
		return nil
	}
	out := make([]Content, 0, len(list))
	for _, c := range list {
		switch v := c.(type) {
		case *mcp.TextContent:
			out = append(out, Content{Type: "text", Text: v.Text})
		default:
			out = append(out, Content{Type: "text", Text: fmt.Sprintf("%v", c)})
		}
	}
	return out
}

func mustJSON(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}

// simpleHTTPClient is a simple JSON-RPC over HTTP client: each request is a single POST, response is in the body. Implements ExternalMCPClient.
// Used for self-hosted MCP (e.g. http://127.0.0.1:8081/mcp) or other endpoints that only support simple POST.
type simpleHTTPClient struct {
	url    string
	client *http.Client
	logger *zap.Logger
	mu     sync.RWMutex
	status string
}

func newSimpleHTTPClient(ctx context.Context, url string, timeout time.Duration, headers map[string]string, logger *zap.Logger) (ExternalMCPClient, error) {
	c := &simpleHTTPClient{
		url:    url,
		client: httpClientWithTimeoutAndHeaders(timeout, headers),
		logger: logger,
		status: "connecting",
	}
	if err := c.initialize(ctx); err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.status = "connected"
	c.mu.Unlock()
	return c, nil
}

func (c *simpleHTTPClient) setStatus(s string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status = s
}

func (c *simpleHTTPClient) GetStatus() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

func (c *simpleHTTPClient) IsConnected() bool {
	return c.GetStatus() == "connected"
}

func (c *simpleHTTPClient) Initialize(context.Context) error {
	return nil // already completed in newSimpleHTTPClient
}

func (c *simpleHTTPClient) initialize(ctx context.Context) error {
	params := InitializeRequest{
		ProtocolVersion: ProtocolVersion,
		Capabilities:    make(map[string]interface{}),
		ClientInfo:      ClientInfo{Name: clientName, Version: clientVersion},
	}
	paramsJSON, _ := json.Marshal(params)
	req := &Message{
		ID:      MessageID{value: "1"},
		Method:  "initialize",
		Version: "2.0",
		Params:  paramsJSON,
	}
	resp, err := c.sendRequest(ctx, req)
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}
	if resp.Error != nil {
		return fmt.Errorf("initialize: %s (code %d)", resp.Error.Message, resp.Error.Code)
	}
	// send notifications/initialized (required by protocol)
	notify := &Message{
		ID:      MessageID{value: nil},
		Method:  "notifications/initialized",
		Version: "2.0",
		Params:  json.RawMessage("{}"),
	}
	_ = c.sendNotification(notify)
	return nil
}

func (c *simpleHTTPClient) sendRequest(ctx context.Context, msg *Message) (*Message, error) {
	body, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(b))
	}
	var out Message
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *simpleHTTPClient) sendNotification(msg *Message) error {
	body, _ := json.Marshal(msg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *simpleHTTPClient) ListTools(ctx context.Context) ([]Tool, error) {
	req := &Message{
		ID:      MessageID{value: uuid.New().String()},
		Method:  "tools/list",
		Version: "2.0",
		Params:  json.RawMessage("{}"),
	}
	resp, err := c.sendRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("tools/list: %s (code %d)", resp.Error.Message, resp.Error.Code)
	}
	var listResp ListToolsResponse
	if err := json.Unmarshal(resp.Result, &listResp); err != nil {
		return nil, err
	}
	return listResp.Tools, nil
}

func (c *simpleHTTPClient) CallTool(ctx context.Context, name string, args map[string]interface{}) (*ToolResult, error) {
	params := CallToolRequest{Name: name, Arguments: args}
	paramsJSON, _ := json.Marshal(params)
	req := &Message{
		ID:      MessageID{value: uuid.New().String()},
		Method:  "tools/call",
		Version: "2.0",
		Params:  paramsJSON,
	}
	resp, err := c.sendRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("tools/call: %s (code %d)", resp.Error.Message, resp.Error.Code)
	}
	var callResp CallToolResponse
	if err := json.Unmarshal(resp.Result, &callResp); err != nil {
		return nil, err
	}
	return &ToolResult{Content: callResp.Content, IsError: callResp.IsError}, nil
}

func (c *simpleHTTPClient) Close() error {
	c.setStatus("disconnected")
	return nil
}

// createSDKClient creates and connects an external MCP client (using the official SDK) based on configuration,
// returning a *sdkClient implementing ExternalMCPClient.
// Returns (nil, error) on connection failure. ctx is used for connection timeout and cancellation.
func createSDKClient(ctx context.Context, serverCfg config.ExternalMCPServerConfig, logger *zap.Logger) (ExternalMCPClient, error) {
	// Resolve ${VAR} / ${VAR:-default} env refs on the local copy only - keeping the
	// template strings in the caller's stored config is what prevents saveConfig()
	// from persisting resolved secrets back to config.yaml on subsequent writes.
	config.ExpandConfigEnv(&serverCfg)

	timeout := time.Duration(serverCfg.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	transport := serverCfg.Transport
	if transport == "" {
		if serverCfg.Command != "" {
			transport = "stdio"
		} else if serverCfg.URL != "" {
			transport = "http"
		} else {
			return nil, fmt.Errorf("configuration missing command or url")
		}
	}

	client := mcp.NewClient(&mcp.Implementation{
		Name:    clientName,
		Version: clientVersion,
	}, nil)

	var t mcp.Transport
	switch transport {
	case "stdio":
		if serverCfg.Command == "" {
			return nil, fmt.Errorf("stdio mode requires command to be configured")
		}
		// must use exec.Command instead of CommandContext: ctx is cancelled after doConnect returns,
		// using CommandContext(ctx) would immediately kill the child process, causing ListTools and other
		// subsequent requests to fail and show 0 tools
		cmd := exec.Command(serverCfg.Command, serverCfg.Args...)
		if len(serverCfg.Env) > 0 {
			cmd.Env = append(cmd.Env, envMapToSlice(serverCfg.Env)...)
		}
		t = &mcp.CommandTransport{Command: cmd}
	case "sse":
		if serverCfg.URL == "" {
			return nil, fmt.Errorf("sse mode requires url to be configured")
		}
		httpClient := httpClientWithTimeoutAndHeaders(timeout, serverCfg.Headers)
		t = &mcp.SSEClientTransport{
			Endpoint:   serverCfg.URL,
			HTTPClient: httpClient,
		}
	case "http":
		if serverCfg.URL == "" {
			return nil, fmt.Errorf("http mode requires url to be configured")
		}
		httpClient := httpClientWithTimeoutAndHeaders(timeout, serverCfg.Headers)
		t = &mcp.StreamableClientTransport{
			Endpoint:   serverCfg.URL,
			HTTPClient: httpClient,
		}
	case "simple_http":
		// simple JSON-RPC HTTP: each request is a single POST, response is in the body. For self-hosted MCP or legacy endpoints (e.g. http://127.0.0.1:8081/mcp)
		if serverCfg.URL == "" {
			return nil, fmt.Errorf("simple_http mode requires url to be configured")
		}
		return newSimpleHTTPClient(ctx, serverCfg.URL, timeout, serverCfg.Headers, logger)
	default:
		return nil, fmt.Errorf("unsupported transport mode: %s", transport)
	}

	session, err := client.Connect(ctx, t, nil)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}

	return newSDKClientFromSession(session, client, logger), nil
}

func envMapToSlice(env map[string]string) []string {
	m := make(map[string]string)
	for _, s := range os.Environ() {
		if i := strings.IndexByte(s, '='); i > 0 {
			m[s[:i]] = s[i+1:]
		}
	}
	for k, v := range env {
		m[k] = v
	}
	out := make([]string, 0, len(m))
	for k, v := range m {
		out = append(out, k+"="+v)
	}
	return out
}

func httpClientWithTimeoutAndHeaders(timeout time.Duration, headers map[string]string) *http.Client {
	transport := http.DefaultTransport
	if len(headers) > 0 {
		transport = &headerRoundTripper{
			headers: headers,
			base:    http.DefaultTransport,
		}
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}

type headerRoundTripper struct {
	headers map[string]string
	base    http.RoundTripper
}

func (h *headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, v := range h.headers {
		req.Header.Set(k, v)
	}
	return h.base.RoundTrip(req)
}
