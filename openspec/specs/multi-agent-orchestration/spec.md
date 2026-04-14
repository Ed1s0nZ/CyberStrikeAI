# Multi-Agent Orchestration

## Purpose
Define the optional deep-orchestration contract in which an orchestrator coordinates specialist agents while preserving the same conversation, tool, and evidence model as the default runtime.

## Requirements
### Requirement: Feature-gated multi-agent execution
Multi-agent execution SHALL be exposed as an explicit feature-gated alternative to the default runtime.

#### Scenario: Client requests multi-agent mode while disabled
- **WHEN** a caller invokes a multi-agent endpoint while the feature is disabled
- **THEN** the system returns an explicit disabled error instead of silently falling back

### Requirement: Shared conversation evidence
Orchestrated and delegated work SHALL remain inside the same conversation and evidence model as the parent request.

#### Scenario: Sub-agent work completes
- **WHEN** a delegated sub-agent contributes to a parent request
- **THEN** the resulting tool evidence and final synthesis are persisted under the same conversation record

## Overview
This domain defines the optional deep orchestration mode in which a coordinating agent delegates work to specialist sub-agents while sharing the same conversation, tool fabric, and persistence model as the single-agent runtime. It is intended for tasks that benefit from decomposition without breaking the platform's observable behavior.

## Capabilities
### Capability 1: Feature-Gated Deep Orchestration
- The system must provide multi-agent execution as an explicit alternative to the default runtime.
- The system must make enablement state observable so clients know whether multi-agent execution is available.

### Capability 2: Effective Agent Set Composition
- The system must derive the effective sub-agent set from configured sub-agents plus Markdown-defined sub-agents.
- The system must optionally accept a Markdown-defined orchestrator identity and instruction set.

### Capability 3: Policy-Aware Delegation
- The system must allow sub-agents to inherit role-scoped tool policy and skill hints where configured.
- The system must ensure delegated work stays inside the same conversation and tool fabric as the parent request.

### Capability 4: Event Normalization
- The system must normalize deep-orchestration internals into the same externally visible progress model used by the single-agent runtime.
- The system must produce a terminal response state even when internal orchestration uses multiple delegated steps.

### Capability 5: Operator-Managed Markdown Agents
- The system must allow operators to create, inspect, update, list, and delete Markdown-defined agents through the API.

## Interfaces
- `POST /api/multi-agent`
- `POST /api/multi-agent/stream`
- `GET /api/multi-agent/markdown-agents`
- `GET /api/multi-agent/markdown-agents/:filename`
- `POST /api/multi-agent/markdown-agents`
- `PUT /api/multi-agent/markdown-agents/:filename`
- `DELETE /api/multi-agent/markdown-agents/:filename`

## State Machine
- Feature lifecycle:
  - `Disabled`
  - `EnabledIdle`
  - `Preparing`
  - `Orchestrating`
  - `Delegating`
  - `Synthesizing`
  - `Completed` or `Failed` or `Cancelled`
- Markdown agent definition lifecycle:
  - `Draft`
  - `Validated`
  - `Loadable`
  - `Deleted`

## Data Flow
1. A multi-agent request is prepared using the same conversation and message persistence flow as single-agent execution.
2. The runtime loads effective sub-agent definitions from config and Markdown sources.
3. Shared MCP tools are bridged into the deep-agent framework with conversation-aware execution tracking.
4. The orchestrator delegates subtasks to specialist agents and collects their tool-assisted findings.
5. Internal events are normalized into progress, tool, and response events for the caller.
6. The final synthesized response, tool execution IDs, and ReAct traces are persisted back into the same conversation model.

## Constraints
- Multi-agent execution is feature-gated; callers must not assume it is always available.
- When the platform is configured to require explicit sub-agents, at least one effective sub-agent definition must exist.
- Markdown agent definitions must be structurally valid and uniquely identify the orchestrator when one is declared.
- Orchestrator and sub-agents share the same conversation scope and therefore must not diverge into separate persistence silos.
- Event normalization must leave the UI in a terminal state even if internal orchestration emits partial or recoverable errors.

## Failure Handling
- If multi-agent mode is disabled, the feature returns a clear disabled error rather than silently falling back.
- Invalid agent definitions may be rejected or ignored, but they must not corrupt the active configured set silently.
- Tool or sub-agent failures should be isolated when possible so the orchestrator can recover or synthesize a partial answer.
- Cancellation must terminate the orchestrator and all delegated work under the same parent request.
