package agent

// TestToolNameMapping_NoRaceOnConcurrentRefresh — Finding 6 from the
// single-agent audit.
//
// getAvailableTools used a split-lock pattern: it wiped a.toolNameMapping
// under one Lock/Unlock, then repopulated it with per-entry Lock/Unlock
// calls inside the loop.  A concurrent goroutine calling getAvailableTools
// (or executeToolViaMCP) could observe the map in the transient empty state
// between the wipe and the first entry being written back.
//
// Fix: build the new mapping locally with no lock held, then swap
// atomically under a single Lock/Unlock at the end.  Readers see either
// the old map or the new map — never an empty intermediate.

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/mcp"

	"go.uber.org/zap"
)

// stubMCPClient satisfies mcp.ExternalMCPClient for unit tests.
// ListTools returns a fixed set of synthetic tools; all other methods are no-ops.
type stubMCPClient struct {
	tools []mcp.Tool
}

func (s *stubMCPClient) Initialize(_ context.Context) error { return nil }
func (s *stubMCPClient) ListTools(_ context.Context) ([]mcp.Tool, error) {
	return s.tools, nil
}
func (s *stubMCPClient) CallTool(_ context.Context, _ string, _ map[string]interface{}) (*mcp.ToolResult, error) {
	return &mcp.ToolResult{}, nil
}
func (s *stubMCPClient) Close() error  { return nil }
func (s *stubMCPClient) IsConnected() bool { return true }
func (s *stubMCPClient) GetStatus() string { return "connected" }

// newTestAgentWithExternalMCP creates an Agent backed by an ExternalMCPManager
// pre-loaded with a stub client that advertises the given synthetic tools.
// The caller is responsible for calling mgr.StopAll() after the test.
func newTestAgentWithExternalMCP(t *testing.T, syntheticTools []mcp.Tool) (*Agent, *mcp.ExternalMCPManager) {
	t.Helper()
	logger := zap.NewNop()

	// Build a real ExternalMCPManager and inject the stub client.
	mgr := mcp.NewExternalMCPManager(logger)
	stub := &stubMCPClient{tools: syntheticTools}
	cfg := config.ExternalMCPServerConfig{
		ExternalMCPEnable: true,
		Transport:         "stdio",
		Command:           "stub",
	}
	mgr.AddClientForTest("testmcp", stub, cfg)

	mcpSrv := mcp.NewServer(logger)
	openAICfg := &config.OpenAIConfig{
		APIKey:  "test-key",
		BaseURL: "https://api.test.com/v1",
		Model:   "test-model",
	}
	agentCfg := &config.AgentConfig{MaxIterations: 10}
	a := NewAgent(openAICfg, agentCfg, mcpSrv, mgr, logger, 10, nil)
	return a, mgr
}

func TestToolNameMapping_NoRaceOnConcurrentRefresh(t *testing.T) {
	// Build 3 synthetic external tools.  Their names must contain "::" in the
	// form mcpName::toolName so getAvailableTools parses and maps them.
	syntheticTools := []mcp.Tool{
		{Name: "scan_host", Description: "scan a host"},
		{Name: "enumerate_ports", Description: "enumerate ports"},
		{Name: "fetch_url", Description: "fetch a URL"},
	}
	// getAvailableTools prepends the MCP name, but the stub's ListTools returns
	// bare names; GetAllTools in ExternalMCPManager prepends "testmcp::" before
	// returning them to getAvailableTools.  So the tools arriving in the loop will
	// be named "testmcp::scan_host" etc. — parsed correctly by the idx check.

	a, mgr := newTestAgentWithExternalMCP(t, syntheticTools)
	defer mgr.StopAll()

	// Warm-up: call once so the mapping is already populated.  After this the
	// map must never be empty again (it will be refreshed but never wiped to
	// empty without immediately being repopulated under the fixed implementation).
	_ = a.getAvailableTools(nil)

	// Verify warm-up actually populated the mapping; if the stub isn't wired
	// correctly the test would pass vacuously.
	a.mu.RLock()
	warmupSize := len(a.toolNameMapping)
	a.mu.RUnlock()
	if warmupSize == 0 {
		t.Fatal("warm-up getAvailableTools call produced an empty toolNameMapping — stub client is not wired; test would be vacuous")
	}

	var wg sync.WaitGroup
	var emptyReads atomic.Uint64

	const readers = 50
	const readsPerReader = 200
	const refreshers = 5
	const refreshesEach = 50

	// Reader goroutines: continuously snapshot len(toolNameMapping).
	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < readsPerReader; i++ {
				a.mu.RLock()
				size := len(a.toolNameMapping)
				a.mu.RUnlock()
				if size == 0 {
					emptyReads.Add(1)
				}
			}
		}()
	}

	// Refresher goroutines: hammer getAvailableTools as fast as possible,
	// simulating two conversations refreshing the tool list concurrently.
	for r := 0; r < refreshers; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < refreshesEach; i++ {
				_ = a.getAvailableTools(nil)
			}
		}()
	}

	wg.Wait()

	if emptyReads.Load() > 0 {
		t.Fatalf("toolNameMapping was empty during %d concurrent reads out of %d total — split-lock race confirmed (Finding 6)",
			emptyReads.Load(), readers*readsPerReader)
	}
}
