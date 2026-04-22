package multiagent

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
)

// TestIsToolAllowed covers the execution-time role-whitelist gate. The
// assertion that actually matters here is the bypass protection: if
// roleTools names a restricted set, a tool outside that set must be
// rejected even if the caller somehow dispatched it. An empty slice
// means "no role restriction" and must allow anything.
func TestIsToolAllowed(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		roleTools []string
		tool      string
		want      bool
	}{
		{
			name:      "empty role list allows anything",
			roleTools: nil,
			tool:      "nmap",
			want:      true,
		},
		{
			name:      "empty slice allows anything",
			roleTools: []string{},
			tool:      "nuclei",
			want:      true,
		},
		{
			name:      "tool on whitelist is allowed",
			roleTools: []string{"nmap", "nuclei", "subfinder"},
			tool:      "nuclei",
			want:      true,
		},
		{
			name:      "tool not on whitelist is denied",
			roleTools: []string{"nmap", "nuclei"},
			tool:      "subfinder",
			want:      false,
		},
		{
			name:      "case-sensitive match: different case is denied",
			roleTools: []string{"nmap"},
			tool:      "Nmap",
			want:      false,
		},
		{
			name:      "exact-match: whitespace variants are denied",
			roleTools: []string{"nmap"},
			tool:      " nmap",
			want:      false,
		},
		{
			name:      "only one tool on the whitelist, matching",
			roleTools: []string{"record_vulnerability"},
			tool:      "record_vulnerability",
			want:      true,
		},
		{
			name:      "empty tool name against non-empty whitelist is denied",
			roleTools: []string{"nmap"},
			tool:      "",
			want:      false,
		},
	}

	o := &orchestratorState{}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := o.isToolAllowed(tc.tool, tc.roleTools)
			if got != tc.want {
				t.Fatalf("isToolAllowed(%q, %v) = %v, want %v", tc.tool, tc.roleTools, got, tc.want)
			}
		})
	}
}

// TestHandleWriteTodos_Valid checks the happy path: the tool stores the
// submitted todo list, returns a success string, and emits one "todos"
// progress event carrying the normalized list.
func TestHandleWriteTodos_Valid(t *testing.T) {
	t.Parallel()

	var gotEventType string
	var gotData interface{}
	progress := func(eventType, message string, data interface{}) {
		gotEventType = eventType
		gotData = data
	}
	o := &orchestratorState{
		progress:       progress,
		conversationID: "conv-123",
	}

	args := map[string]interface{}{
		"todos": []interface{}{
			map[string]interface{}{"content": "Enumerate open ports", "status": "pending"},
			map[string]interface{}{"content": "Probe HTTP services", "status": "in_progress"},
		},
	}

	result, isErr := o.handleWriteTodos(args)
	if isErr {
		t.Fatalf("expected success, got error result: %q", result)
	}
	if !strings.Contains(result, "(2 items)") {
		t.Fatalf("expected item count in result, got %q", result)
	}

	o.mu.Lock()
	stored := o.todos
	o.mu.Unlock()
	if len(stored) != 2 {
		t.Fatalf("expected 2 todos stored, got %d", len(stored))
	}
	if stored[1].Status != "in_progress" {
		t.Fatalf("expected second todo status=in_progress, got %q", stored[1].Status)
	}

	if gotEventType != "todos" {
		t.Fatalf("expected progress event type %q, got %q", "todos", gotEventType)
	}
	m, ok := gotData.(map[string]interface{})
	if !ok {
		t.Fatalf("expected progress data to be a map, got %T", gotData)
	}
	if m["conversationId"] != "conv-123" {
		t.Fatalf("expected conversationId in progress data, got %v", m["conversationId"])
	}
}

// TestHandleWriteTodos_MissingField covers the error path where the LLM
// called write_todos but omitted the required todos array. The tool must
// return isError=true without mutating state.
func TestHandleWriteTodos_MissingField(t *testing.T) {
	t.Parallel()

	o := &orchestratorState{}
	result, isErr := o.handleWriteTodos(map[string]interface{}{})
	if !isErr {
		t.Fatalf("expected error, got success: %q", result)
	}
	if !strings.Contains(strings.ToLower(result), "required") {
		t.Fatalf("expected error message to mention 'required', got %q", result)
	}
	if len(o.todos) != 0 {
		t.Fatalf("expected todos unchanged on error, got %d items", len(o.todos))
	}
}

// TestHandleWriteTodos_MalformedType covers the case where todos is
// present but not the expected array-of-objects shape. The JSON
// unmarshal should reject it and we must not crash or partially update
// state.
func TestHandleWriteTodos_MalformedType(t *testing.T) {
	t.Parallel()

	o := &orchestratorState{
		todos: []todoItem{{Content: "existing", Status: "pending"}},
	}
	args := map[string]interface{}{"todos": "not-an-array"}

	result, isErr := o.handleWriteTodos(args)
	if !isErr {
		t.Fatalf("expected error, got success: %q", result)
	}
	if len(o.todos) != 1 || o.todos[0].Content != "existing" {
		t.Fatalf("expected existing todos preserved on error, got %+v", o.todos)
	}
}

// TestHandleWriteTodos_EmptyList accepts an explicitly empty list and
// replaces any prior state — the LLM may clear its plan by submitting
// an empty array.
func TestHandleWriteTodos_EmptyList(t *testing.T) {
	t.Parallel()

	o := &orchestratorState{
		todos: []todoItem{{Content: "stale plan", Status: "pending"}},
	}
	args := map[string]interface{}{"todos": []interface{}{}}

	result, isErr := o.handleWriteTodos(args)
	if isErr {
		t.Fatalf("expected success on empty list, got error: %q", result)
	}
	if len(o.todos) != 0 {
		t.Fatalf("expected todos cleared, got %d items", len(o.todos))
	}
}

// TestHandleWriteTodos_JSONRoundtrip confirms that whatever shape the
// LLM submits (as it comes out of the OpenAI tool-args JSON decode) is
// accepted, and that the stored todoItem slice is JSON-encodable back.
// This guards against a refactor accidentally introducing a field type
// that can't round-trip through the SSE data payload.
func TestHandleWriteTodos_JSONRoundtrip(t *testing.T) {
	t.Parallel()

	var captured interface{}
	o := &orchestratorState{
		progress: func(eventType, message string, data interface{}) {
			captured = data
		},
	}

	args := map[string]interface{}{
		"todos": []interface{}{
			map[string]interface{}{"content": "task A", "status": "pending"},
		},
	}
	if _, isErr := o.handleWriteTodos(args); isErr {
		t.Fatalf("unexpected error")
	}

	if _, err := json.Marshal(captured); err != nil {
		t.Fatalf("progress data is not JSON-encodable: %v", err)
	}
}

// TestSnapshotMCPIDs_ConcurrentWrites confirms the mcpIDs mutex works
// under concurrent recordMCPID calls. Relevant now that #30 removed
// the broken save-swap-restore pattern — the orchestrator's own
// concurrency primitives need to stay correct.
func TestSnapshotMCPIDs_ConcurrentWrites(t *testing.T) {
	t.Parallel()

	o := &orchestratorState{}
	var wg sync.WaitGroup
	const workers = 50
	const perWorker = 20

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				o.recordMCPID("exec-id")
			}
		}()
	}
	wg.Wait()

	snap := o.snapshotMCPIDs()
	if len(snap) != workers*perWorker {
		t.Fatalf("expected %d recorded ids, got %d", workers*perWorker, len(snap))
	}
}
