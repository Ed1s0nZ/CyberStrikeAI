# Tool Runtime And MCP

## Purpose
Define the platform's shared tool fabric, including MCP serving, local execution, external MCP federation, monitoring, and durable result handling.

## Requirements
### Requirement: Shared effective tool surface
The platform SHALL expose a coherent effective tool registry across its supported MCP transports.

#### Scenario: Client accesses tools through different transports
- **WHEN** a client connects through HTTP, SSE-assisted MCP, or stdio
- **THEN** the logical tool surface remains consistent for that runtime

### Requirement: Durable execution observability
Tool invocation SHALL produce monitorable execution records and retrievable result references.

#### Scenario: Large tool output is produced
- **WHEN** a tool returns output too large for inline handling
- **THEN** the system stores the output durably and preserves a retrievable execution reference

## Overview
This domain provides the tool execution fabric for the platform. It includes the internal MCP server, built-in and configured tools, external MCP client federation, execution monitoring, terminal command execution, and durable result storage for large outputs.

## Capabilities
### Capability 1: Unified Tool Registry
- The system must register configured tools and required built-in tools into a common effective registry.
- The registry must expose tool schema and invocation behavior through the MCP surface.

### Capability 2: Multi-Transport MCP Serving
- The system must support MCP access over HTTP request/response, SSE-assisted HTTP, and stdio.
- The system must preserve the same logical tool surface across those transports.

### Capability 3: Normalized Tool Execution
- The system must execute local commands, internal handlers, and terminal operations under a normalized result contract.
- The system must persist execution status, statistics, and result references for later inspection.
- The system must retry with PTY semantics when the command path indicates a TTY requirement.

### Capability 4: External MCP Federation
- The system must support connecting to external MCP servers and incorporating their tools into the effective registry.
- The system must namespace external tools to avoid collisions with internal or other external tools.
- The system must retain cached external metadata so transient disconnects do not erase operator visibility.

### Capability 5: Monitoring And Management
- The system must expose execution monitoring and external-MCP management APIs as first-class operational capabilities.

## Interfaces
- MCP transports:
  - `/mcp`
  - `cmd/mcp-stdio`
- Monitoring APIs:
  - `GET /api/monitor`
  - `GET /api/monitor/execution/:id`
  - `POST /api/monitor/executions/names`
  - `DELETE /api/monitor/execution/:id`
  - `DELETE /api/monitor/executions`
  - `GET /api/monitor/stats`
- External MCP APIs:
  - `GET /api/external-mcp`
  - `GET /api/external-mcp/stats`
  - `GET /api/external-mcp/:name`
  - `PUT /api/external-mcp/:name`
  - `DELETE /api/external-mcp/:name`
  - `POST /api/external-mcp/:name/start`
  - `POST /api/external-mcp/:name/stop`
- Terminal APIs:
  - `POST /api/terminal/run`
  - `POST /api/terminal/run/stream`
  - `GET /api/terminal/ws`

## State Machine
- Tool execution lifecycle:
  - `Registered`
  - `Available`
  - `Invoking`
  - `Succeeded` or `Failed`
  - `Retained` in monitor storage
- External MCP client lifecycle:
  - `Disconnected`
  - `Connecting`
  - `Connected`
  - `Error`
  - `Stopped`
- MCP session lifecycle:
  - `Idle`
  - `SSEEstablished`
  - `RequestHandling`
  - `Closed`

## Data Flow
1. Tool definitions are loaded from config and built-in registrars.
2. The MCP server publishes tool schemas, prompt/resource metadata, and invocation handlers.
3. Agents, APIs, or external clients invoke tools through the MCP surface or terminal surface.
4. The executor runs the underlying command or internal handler and streams output when requested.
5. Execution records, statistics, and large-result references are persisted for later inspection.
6. External MCP tools are discovered, namespaced, cached, and added to the effective tool surface.

## Constraints
- Tool names must be unique within the internal registry; external tools must be namespaced by external MCP server identity.
- Built-in tools required by higher-level domains must survive config apply and tool reload.
- Large outputs may be stored out-of-band, but the execution record must remain sufficient to retrieve, page, search, or filter the result later.
- MCP SSE sessions must follow the platform's JSON-RPC-over-SSE contract with a discoverable POST endpoint per session.
- Allowed exit codes may mark a command as successful even when the process exit code is non-zero.

## Failure Handling
- Unknown tool invocations return structured errors rather than crashing the registry.
- If command execution suggests a TTY requirement, the executor retries with PTY before declaring failure.
- If external MCP connection attempts fail, the manager records explicit error state and preserves cached metadata when possible.
- If an SSE session cannot accept more messages, the platform rejects the request explicitly rather than blocking indefinitely.
- Partial external MCP availability degrades only the affected prefixed tools, not the internal tool fabric.
