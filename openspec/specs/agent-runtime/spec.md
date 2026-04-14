# Agent Runtime

## Purpose
Define the default single-agent execution contract for interactive security conversations, including context recovery, tool use, progress visibility, and cancellation.

## Requirements
### Requirement: Single-agent execution path
The runtime SHALL execute prompts through the default single-agent flow when multi-agent orchestration is not selected.

#### Scenario: User sends a standard prompt
- **WHEN** a valid chat request is submitted without selecting multi-agent mode
- **THEN** the request executes through the single-agent runtime and returns a durable conversation outcome

### Requirement: Observable long-running execution
The runtime SHALL expose progress and terminal state for long-running agent work.

#### Scenario: Execution is cancelled
- **WHEN** an in-flight agent task is explicitly cancelled
- **THEN** the runtime records a terminal cancelled outcome instead of an ambiguous generic failure

## Overview
The agent runtime executes single-agent security conversations backed by an LLM, role-aware tool access, persistent context, and tool-mediated evidence capture. It is the default execution path for interactive chat, API-driven prompting, WebShell assistant mode, and robot-originated work when multi-agent orchestration is not selected.

## Capabilities
### Capability 1: Interactive Single-Agent Execution
- The runtime must support both non-streaming and streaming execution for a user prompt.
- The runtime must create a conversation automatically when the request does not reference an existing one.

### Capability 2: Context Recovery
- The runtime must reconstruct prior context from saved ReAct traces when those traces exist.
- The runtime must fall back to durable message history when ReAct traces are absent or unusable.

### Capability 3: Role-Scoped Execution Policy
- The runtime must allow a role to inject prompt guidance, tool restrictions, and skill hints for the current execution.
- The runtime must preserve the user's raw stored message separately from role-expanded execution context.

### Capability 4: WebShell Assistant Binding
- The runtime must support a WebShell assistant mode that binds execution to a selected connection.
- The runtime must restrict tool visibility in WebShell assistant mode to the approved WebShell-safe tool subset.

### Capability 5: Observable Long-Running Work
- The runtime must emit progress as process details during execution.
- The runtime must expose active task inspection and explicit cancellation for long-running sessions.
- The runtime must preserve tool execution references and large-result references in the final durable record.

## Interfaces
- `POST /api/agent-loop`
- `POST /api/agent-loop/stream`
- `POST /api/agent-loop/cancel`
- `GET /api/agent-loop/tasks`
- `GET /api/agent-loop/tasks/completed`
- OpenAPI-aligned equivalents for conversation results and external API consumers.

## State Machine
- Request lifecycle:
  - `Received`
  - `Validated`
  - `Prepared`
  - `Running`
  - `WaitingOnTool` or `StreamingResponse`
  - `Completed` or `Failed` or `Cancelled`
- Conversation task guard:
  - `Idle` -> `Running`
  - `Running` -> `Idle`
  - `Running` -> `Cancelled`

## Data Flow
1. The runtime validates request shape, conversation identity, attachment count, and optional WebShell context.
2. Historical context is reconstructed from saved ReAct data or message history.
3. Role policy and attachment references are injected into the execution prompt.
4. The user message is persisted before model execution.
5. The runtime invokes the LLM, emits progress/tool events, and records process details.
6. Tool execution IDs and final assistant content are persisted into the conversation.
7. The last ReAct input/output pair is saved for future context recovery and downstream analysis.

## Constraints
- Only one active task may run per conversation at a time.
- Streaming and non-streaming executions must share the same durable conversation semantics.
- Role policies constrain tool visibility for the current execution but do not retroactively rewrite history.
- WebShell assistant executions must be tool-restricted and bound to the selected connection ID.
- Uploads are passed into the prompt as server-side references, not repeated inline file bodies.
- Cancellation must propagate through model and tool execution paths for long-running jobs.

## Failure Handling
- Invalid conversation IDs, invalid WebShell references, and invalid attachments are rejected before execution begins.
- If the user message cannot be persisted, execution does not proceed.
- If assistant persistence fails after generation, the runtime still returns the generated response to the caller when possible and surfaces the persistence error through logs or secondary signals.
- Duplicate concurrent execution attempts on the same conversation are rejected explicitly.
- Cancellation produces a terminal cancelled state rather than masquerading as a generic error.
