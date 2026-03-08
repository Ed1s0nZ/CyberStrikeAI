package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"cyberstrike-ai/internal/database"

	"go.uber.org/zap"
)

// ScanEvent represents a progress event from a parallel scan
type ScanEvent struct {
	ScanID    string      `json:"scanId"`
	AgentName string      `json:"agentName"`
	AgentID   string      `json:"agentId"`
	Type      string      `json:"type"`
	Message   string      `json:"message,omitempty"`
	Data      interface{} `json:"data,omitempty"`
}

// ParallelScanState tracks in-memory state for a running scan
type ParallelScanState struct {
	ID          string
	Target      string
	MaxRounds   int
	Status      string
	Agents      map[string]*ScanAgentState // agentID -> state
	cancel      context.CancelFunc
	Mu          sync.RWMutex
	subscribers map[chan ScanEvent]struct{}
	subMu       sync.RWMutex
}

// ScanAgentState tracks in-memory state for a single agent in a parallel scan
type ScanAgentState struct {
	ID             string
	Name           string
	ConversationID string
	Status         string
	CurrentRound   int
	TotalIters     int
	TotalTools     int
	TotalVulns     int
	Errors         int
	LastActivity   time.Time
	cancel         context.CancelFunc
}

// ParallelScanManager orchestrates parallel scan sessions
type ParallelScanManager struct {
	agent  *Agent
	db     *database.DB
	logger *zap.Logger
	scans  map[string]*ParallelScanState
	mu     sync.RWMutex
}

// NewParallelScanManager creates a new manager
func NewParallelScanManager(agent *Agent, db *database.DB, logger *zap.Logger) *ParallelScanManager {
	return &ParallelScanManager{
		agent:  agent,
		db:     db,
		logger: logger,
		scans:  make(map[string]*ParallelScanState),
	}
}

// StartScan creates and starts a parallel scan
func (m *ParallelScanManager) StartScan(target string, agentNames []string, maxRounds int, reconContext string) (*ParallelScanState, error) {
	if len(agentNames) == 0 {
		agentNames = AttackVectorNames()
	}
	if maxRounds <= 0 {
		maxRounds = 20
	}

	// Create conversation group
	groupName := fmt.Sprintf("Parallel Scan: %s — %s", target, time.Now().Format("2006-01-02 15:04"))
	group, err := m.db.CreateGroup(groupName, "🔍")
	if err != nil {
		m.logger.Warn("Failed to create scan group", zap.Error(err))
	}

	groupID := ""
	if group != nil {
		groupID = group.ID
	}

	// Create DB records
	scanRow, agentRows, err := m.db.CreateParallelScan(target, maxRounds, reconContext, groupID, agentNames)
	if err != nil {
		return nil, fmt.Errorf("create scan: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	state := &ParallelScanState{
		ID:          scanRow.ID,
		Target:      target,
		MaxRounds:   maxRounds,
		Status:      "running",
		Agents:      make(map[string]*ScanAgentState),
		cancel:      cancel,
		subscribers: make(map[chan ScanEvent]struct{}),
	}

	for _, ar := range agentRows {
		agentCtx, agentCancel := context.WithCancel(ctx)
		as := &ScanAgentState{
			ID:     ar.ID,
			Name:   ar.Name,
			Status: "pending",
			cancel: agentCancel,
		}
		state.Agents[ar.ID] = as

		// Get attack vector config
		av := GetAttackVectorByName(ar.Name)
		if av == nil {
			m.logger.Error("Unknown attack vector", zap.String("name", ar.Name))
			continue
		}

		// Launch goroutine (stagger by 2s)
		go m.runAgent(agentCtx, state, as, av, target, reconContext, groupID)
		time.Sleep(2 * time.Second)
	}

	m.db.UpdateParallelScanStatus(scanRow.ID, "running")

	m.mu.Lock()
	m.scans[scanRow.ID] = state
	m.mu.Unlock()

	return state, nil
}

// runAgent runs a single agent's scan loop in a goroutine
func (m *ParallelScanManager) runAgent(ctx context.Context, scan *ParallelScanState, as *ScanAgentState, av *AttackVectorConfig, target, reconContext, groupID string) {
	as.Status = "running"
	m.broadcastEvent(scan, ScanEvent{
		ScanID: scan.ID, AgentName: as.Name, AgentID: as.ID,
		Type: "agent_status",
		Data: map[string]interface{}{"status": "running"},
	})

	m.db.UpdateParallelScanAgent(as.ID, map[string]interface{}{"status": "running"})

	consecutiveErrors := 0

	for round := 1; round <= scan.MaxRounds; round++ {
		select {
		case <-ctx.Done():
			as.Status = "cancelled"
			m.db.UpdateParallelScanAgent(as.ID, map[string]interface{}{"status": "cancelled"})
			return
		default:
		}

		as.CurrentRound = round
		m.broadcastEvent(scan, ScanEvent{
			ScanID: scan.ID, AgentName: as.Name, AgentID: as.ID,
			Type:    "agent_status",
			Message: fmt.Sprintf("Round %d/%d", round, scan.MaxRounds),
			Data:    map[string]interface{}{"status": "running", "round": round, "maxRounds": scan.MaxRounds},
		})

		// Build prompt
		var message string
		if round == 1 {
			message = av.InitialPrompt(target, reconContext)
		} else {
			message = av.ContinuePrompt(target)
		}

		// Create conversation for first round
		if as.ConversationID == "" {
			title := fmt.Sprintf("[%s] %s", as.Name, target)
			conv, err := m.db.CreateConversation(title)
			if err != nil {
				m.logger.Error("Failed to create conversation", zap.Error(err), zap.String("agent", as.Name))
				as.Errors++
				consecutiveErrors++
				if consecutiveErrors > 3 {
					break
				}
				continue
			}
			as.ConversationID = conv.ID
			m.db.UpdateParallelScanAgent(as.ID, map[string]interface{}{"conversation_id": conv.ID})

			// Add to group
			if groupID != "" {
				m.db.AddConversationToGroup(conv.ID, groupID)
			}
		}

		// Save user message
		m.db.AddMessage(as.ConversationID, "user", message, nil)

		// Create progress callback that broadcasts events
		progressCallback := m.createAgentProgressCallback(scan, as)

		// Load history
		history := m.loadHistory(as.ConversationID)

		// Run agent loop
		result, err := m.agent.AgentLoopWithProgress(ctx, message, history, as.ConversationID, progressCallback, nil, nil)
		if err != nil {
			m.logger.Warn("Agent loop error", zap.Error(err), zap.String("agent", as.Name), zap.Int("round", round))
			as.Errors++
			consecutiveErrors++
			m.db.UpdateParallelScanAgent(as.ID, map[string]interface{}{"errors": as.Errors})

			if consecutiveErrors > 3 {
				m.logger.Error("Too many consecutive errors, stopping agent", zap.String("agent", as.Name))
				break
			}
			time.Sleep(10 * time.Second)
			continue
		}

		consecutiveErrors = 0

		// Save assistant response
		if result != nil {
			m.db.AddMessage(as.ConversationID, "assistant", result.Response, result.MCPExecutionIDs)
			if result.LastReActInput != "" || result.LastReActOutput != "" {
				m.db.SaveReActData(as.ConversationID, result.LastReActInput, result.LastReActOutput)
			}
		}

		// Update DB
		m.db.UpdateParallelScanAgent(as.ID, map[string]interface{}{
			"current_round":    as.CurrentRound,
			"total_iterations": as.TotalIters,
			"total_tool_calls": as.TotalTools,
			"total_vulns":      as.TotalVulns,
			"errors":           as.Errors,
			"last_activity":    time.Now(),
		})

		time.Sleep(3 * time.Second)
	}

	as.Status = "completed"
	m.db.UpdateParallelScanAgent(as.ID, map[string]interface{}{"status": "completed"})

	m.broadcastEvent(scan, ScanEvent{
		ScanID: scan.ID, AgentName: as.Name, AgentID: as.ID,
		Type: "agent_done",
		Data: map[string]interface{}{
			"totalIterations": as.TotalIters,
			"totalToolCalls":  as.TotalTools,
			"totalVulns":      as.TotalVulns,
		},
	})

	// Check if all agents are done
	m.checkScanCompletion(scan)
}

// createAgentProgressCallback creates a callback that counts events and broadcasts them
func (m *ParallelScanManager) createAgentProgressCallback(scan *ParallelScanState, as *ScanAgentState) ProgressCallback {
	return func(eventType, message string, data interface{}) {
		as.LastActivity = time.Now()

		switch eventType {
		case "iteration":
			as.TotalIters++
			m.broadcastEvent(scan, ScanEvent{
				ScanID: scan.ID, AgentName: as.Name, AgentID: as.ID,
				Type: "iteration", Message: message, Data: data,
			})
		case "tool_call":
			as.TotalTools++
			m.broadcastEvent(scan, ScanEvent{
				ScanID: scan.ID, AgentName: as.Name, AgentID: as.ID,
				Type: "tool_call", Message: message, Data: data,
			})
		case "vulnerability":
			as.TotalVulns++
			m.broadcastEvent(scan, ScanEvent{
				ScanID: scan.ID, AgentName: as.Name, AgentID: as.ID,
				Type: "vulnerability", Message: message, Data: data,
			})
		default:
			m.broadcastEvent(scan, ScanEvent{
				ScanID: scan.ID, AgentName: as.Name, AgentID: as.ID,
				Type: eventType, Message: message, Data: data,
			})
		}
	}
}

// loadHistory loads conversation history for an agent
func (m *ParallelScanManager) loadHistory(conversationID string) []ChatMessage {
	messages, err := m.db.GetMessages(conversationID)
	if err != nil {
		return []ChatMessage{}
	}
	history := make([]ChatMessage, 0, len(messages))
	for _, msg := range messages {
		history = append(history, ChatMessage{Role: msg.Role, Content: msg.Content})
	}
	return history
}

// broadcastEvent sends an event to all subscribers
func (m *ParallelScanManager) broadcastEvent(scan *ParallelScanState, event ScanEvent) {
	scan.subMu.RLock()
	defer scan.subMu.RUnlock()
	for ch := range scan.subscribers {
		select {
		case ch <- event:
		default:
			// Drop if subscriber is slow
		}
	}
}

// Subscribe returns a channel that receives scan events
func (m *ParallelScanManager) Subscribe(scanID string) (chan ScanEvent, func(), error) {
	m.mu.RLock()
	scan, exists := m.scans[scanID]
	m.mu.RUnlock()
	if !exists {
		return nil, nil, fmt.Errorf("scan not found: %s", scanID)
	}

	ch := make(chan ScanEvent, 100)
	scan.subMu.Lock()
	scan.subscribers[ch] = struct{}{}
	scan.subMu.Unlock()

	unsubscribe := func() {
		scan.subMu.Lock()
		delete(scan.subscribers, ch)
		scan.subMu.Unlock()
		close(ch)
	}

	return ch, unsubscribe, nil
}

// StopScan stops all agents in a scan
func (m *ParallelScanManager) StopScan(scanID string) error {
	m.mu.RLock()
	scan, exists := m.scans[scanID]
	m.mu.RUnlock()
	if !exists {
		return fmt.Errorf("scan not found: %s", scanID)
	}

	scan.cancel()
	scan.Status = "cancelled"
	m.db.UpdateParallelScanStatus(scanID, "cancelled")
	return nil
}

// StopAgent stops a single agent
func (m *ParallelScanManager) StopAgent(scanID, agentID string) error {
	m.mu.RLock()
	scan, exists := m.scans[scanID]
	m.mu.RUnlock()
	if !exists {
		return fmt.Errorf("scan not found: %s", scanID)
	}

	scan.Mu.RLock()
	as, exists := scan.Agents[agentID]
	scan.Mu.RUnlock()
	if !exists {
		return fmt.Errorf("agent not found: %s", agentID)
	}

	as.cancel()
	return nil
}

// RestartAgent restarts a stopped/failed agent
func (m *ParallelScanManager) RestartAgent(scanID, agentID string) error {
	m.mu.RLock()
	scan, exists := m.scans[scanID]
	m.mu.RUnlock()
	if !exists {
		return fmt.Errorf("scan not found: %s", scanID)
	}

	scan.Mu.RLock()
	as, exists := scan.Agents[agentID]
	scan.Mu.RUnlock()
	if !exists {
		return fmt.Errorf("agent not found: %s", agentID)
	}

	av := GetAttackVectorByName(as.Name)
	if av == nil {
		return fmt.Errorf("unknown attack vector: %s", as.Name)
	}

	// Reset state
	ctx, cancel := context.WithCancel(context.Background())
	as.cancel = cancel
	as.Status = "running"
	as.Errors = 0

	go m.runAgent(ctx, scan, as, av, scan.Target, "", "")
	return nil
}

// GetScanState returns current scan state
func (m *ParallelScanManager) GetScanState(scanID string) (*ParallelScanState, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	scan, exists := m.scans[scanID]
	return scan, exists
}

// checkScanCompletion checks if all agents are done and updates scan status
func (m *ParallelScanManager) checkScanCompletion(scan *ParallelScanState) {
	scan.Mu.RLock()
	allDone := true
	for _, as := range scan.Agents {
		if as.Status == "running" || as.Status == "pending" {
			allDone = false
			break
		}
	}
	scan.Mu.RUnlock()

	if allDone {
		scan.Status = "completed"
		m.db.UpdateParallelScanStatus(scan.ID, "completed")

		totalIters := 0
		totalVulns := 0
		for _, as := range scan.Agents {
			totalIters += as.TotalIters
			totalVulns += as.TotalVulns
		}

		m.broadcastEvent(scan, ScanEvent{
			ScanID: scan.ID,
			Type:   "scan_done",
			Data: map[string]interface{}{
				"totalIterations": totalIters,
				"totalVulns":      totalVulns,
			},
		})
	}
}
