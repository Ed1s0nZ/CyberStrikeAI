# Platform Core

## Purpose
Define the platform-level operating contract for CyberStrikeAI so all other specs compose against the same lifecycle, serving model, persistence posture, and degradation semantics.

## Requirements
### Requirement: Unified platform surface
The platform SHALL expose one coherent service surface for human operators and programmatic clients.

#### Scenario: Platform enters serving state
- **WHEN** mandatory platform dependencies initialize successfully
- **THEN** the platform exposes its Web, API, and MCP entrypoints under one runtime

### Requirement: Durable operational evidence
The platform SHALL preserve durable records for user-visible work products and execution evidence.

#### Scenario: Client reconnects after prior work
- **WHEN** a client reloads or reconnects after prior activity was persisted
- **THEN** the platform reconstructs visible history from durable records rather than ephemeral UI state

## Overview
CyberStrikeAI is a single-process security operations platform that exposes one coherent system through Web UI, REST API, SSE streams, MCP endpoints, and robot channels. The platform's primary responsibility is to turn operator intent into durable, auditable security workflows that combine LLM reasoning, tool execution, knowledge retrieval, and evidence persistence.

This domain defines platform-level invariants rather than feature-local behavior. Feature domains may evolve independently, but they must remain composable under the same control plane, persistence model, and operational lifecycle.

## Capabilities
### Capability 1: Unified Serving Surface
- The platform must expose a single coherent service boundary for Web UI, REST APIs, SSE streams, and MCP transports.
- The platform must preserve interface consistency so a capability exposed to operators can also be made observable to programmatic clients when that capability is API-backed.

### Capability 2: Durable Operational Record
- The platform must persist operator-visible records that are required to reconstruct prior work, including conversations, tool executions, vulnerabilities, attack chains, and batch queues.
- The platform must treat persisted artifacts as the recovery source after page reload, reconnect, or process restart where persistence is configured.

### Capability 3: Optional Domain Composition
- The platform must allow optional subsystems to be enabled, disabled, initialized, or degraded independently without redefining the global platform contract.
- The platform must preserve global availability whenever only optional domains fail.

### Capability 4: Auditable Execution
- The platform must retain enough execution evidence to explain how a user-visible outcome was produced.
- The platform must support audit reconstruction through message history, process details, tool execution references, and derived artifacts.

## Interfaces
- Human interface: `/` serves the SPA dashboard and operational pages.
- Programmatic HTTP interface: `/api/**` serves authenticated JSON APIs and SSE endpoints.
- MCP interface: `/mcp` serves JSON-RPC over HTTP and SSE; `cmd/mcp-stdio` serves the same capability over stdio.
- Documentation interface: `/openapi/spec` publishes machine-readable API metadata and `/api-docs` publishes human-readable API documentation.
- Operational entrypoints: `cmd/server` runs the full platform; auxiliary `cmd/test-*` binaries validate config and integration assumptions.

## State Machine
- `Bootstrapping` -> configuration, logging, storage, and core managers are initialized.
- `Ready` -> mandatory dependencies are available and routes are registered.
- `Serving` -> HTTP and optional MCP listeners accept traffic.
- `Reconfiguring` -> runtime configuration is being applied; mandatory services remain available while optional domains may be reloaded.
- `Degraded` -> the platform remains up, but one or more optional domains are unavailable or partially functional.
- `Stopped` -> listeners are no longer serving requests.

Allowed transitions:
- `Bootstrapping` -> `Ready` -> `Serving`
- `Serving` -> `Reconfiguring` -> `Serving`
- `Serving` -> `Degraded` -> `Serving`
- `Serving` or `Degraded` -> `Stopped`

## Data Flow
1. A client enters through Web UI, REST, robot webhook, or MCP transport.
2. The platform authenticates the request when the interface is protected.
3. The request is routed into a domain service such as conversation execution, tool invocation, knowledge search, or queue management.
4. Domain services call LLM backends, security tools, MCP clients, or local persistence as needed.
5. Results are normalized into durable records and returned through the calling channel.
6. Derived views such as dashboards, process timelines, and attack chains are reconstructed from persisted artifacts rather than ephemeral UI state alone.

## Constraints
- The platform is designed as a single-node service with some in-memory control state; it is not a distributed control plane by default.
- Mandatory boot dependencies are configuration, logging, authentication, route registration, and primary persistence.
- Optional domains must fail independently whenever possible; a fault in one optional feature must not silently corrupt unrelated domains.
- Durable records are authoritative for user-facing history. In-memory state may accelerate runtime control but must not be the only source of truth for completed work.
- Security posture is fail-closed for protected interfaces and fail-visible for degraded optional features.

## Failure Handling
- If mandatory initialization fails, the platform must not enter `Serving`.
- If optional subsystem initialization fails, the platform may enter `Degraded`, but it must surface that degradation explicitly through the affected interfaces.
- If a derived artifact cannot be loaded, the system should preserve source records and allow regeneration rather than deleting evidence.
- If a transport-specific response path fails, persisted work should remain queryable from another interface when possible.
