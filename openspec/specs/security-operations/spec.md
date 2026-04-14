# Security Operations

## Purpose
Define the platform's security-facing workflows for reconnaissance, WebShell operations, vulnerability tracking, and attack-chain synthesis from conversation evidence.

## Requirements
### Requirement: Server-mediated security operations
Security operations that touch external targets or shells SHALL execute through server-controlled workflows rather than uncontrolled browser-side paths.

#### Scenario: Operator triggers a WebShell action
- **WHEN** a caller invokes a WebShell command or file operation
- **THEN** the request is proxied through the server under the stored connection definition

### Requirement: Conversation-linked findings
Security findings and derived graphs SHALL remain linked to the originating conversation context.

#### Scenario: Vulnerability or attack chain is created
- **WHEN** the system records a vulnerability or generates an attack chain
- **THEN** the artifact remains addressable through its conversation linkage and durable evidence model

### Requirement: FOFA prerequisite validation
FOFA parsing and FOFA search SHALL validate their own upstream prerequisites.

#### Scenario: Natural-language FOFA parse is requested without model config
- **WHEN** a caller requests FOFA natural-language parsing and the required model configuration is unavailable
- **THEN** the request is rejected with an explicit prerequisite error

#### Scenario: FOFA search is requested without FOFA credentials
- **WHEN** a caller requests FOFA search and required FOFA credentials are unavailable
- **THEN** the request is rejected with an explicit credential/configuration error

### Requirement: Managed WebShell connections
The system SHALL persist reusable WebShell connection metadata and per-connection state independently from transient command output.

#### Scenario: Operator updates WebShell connection state
- **WHEN** a caller writes UI state for a known WebShell connection
- **THEN** the state blob is persisted and later readable by connection identifier

### Requirement: Vulnerability lifecycle management
The system SHALL support durable vulnerability creation, update, listing, and deletion with stable severity and status fields.

#### Scenario: Vulnerability status changes
- **WHEN** an operator updates an existing vulnerability
- **THEN** the updated severity/status/evidence fields are persisted without changing its identity

### Requirement: Single-flight attack-chain generation
Attack-chain generation for a conversation SHALL avoid conflicting concurrent builds.

#### Scenario: Duplicate generation is requested for the same conversation
- **WHEN** an attack chain for a conversation is already being generated
- **THEN** a concurrent regenerate/get request is rejected with an explicit conflict outcome

## Overview
This domain captures the security-facing workflows that the platform orchestrates: reconnaissance query generation, FOFA search, WebShell connection management and proxy execution, vulnerability tracking, and attack-chain synthesis from conversation evidence.

## Capabilities
### Capability 1: Reconnaissance Querying
- The system must translate natural-language reconnaissance intent into FOFA-compatible query syntax.
- The system must execute FOFA search against configured credentials and endpoint settings.

### Capability 2: Server-Side WebShell Proxying
- The system must store reusable WebShell connection metadata.
- The system must proxy WebShell command and file operations through the server rather than exposing direct browser-to-shell control.
- The system must persist a per-connection UI state blob independently of command history.

### Capability 3: AI-Assisted WebShell Sessions
- The system must allow AI-assisted WebShell work to be bound to a selected connection.
- The system must reuse the main conversation model for WebShell assistant evidence and history.

### Capability 4: Vulnerability Tracking
- The system must persist vulnerabilities with conversation linkage, severity, status, evidence, impact, and remediation metadata.
- The system must expose list, filter, read, update, delete, and summary statistics operations for vulnerability records.

### Capability 5: Attack-Chain Synthesis
- The system must generate and persist attack-chain graphs from conversation evidence.
- The system must support explicit regeneration when operators want derived graphs rebuilt from source evidence.

## Interfaces
- FOFA APIs:
  - `POST /api/fofa/parse`
  - `POST /api/fofa/search`
- WebShell APIs:
  - `GET /api/webshell/connections`
  - `POST /api/webshell/connections`
  - `PUT /api/webshell/connections/:id`
  - `DELETE /api/webshell/connections/:id`
  - `GET /api/webshell/connections/:id/state`
  - `PUT /api/webshell/connections/:id/state`
  - `GET /api/webshell/connections/:id/ai-history`
  - `GET /api/webshell/connections/:id/ai-conversations`
  - `POST /api/webshell/exec`
  - `POST /api/webshell/file`
- Vulnerability APIs:
  - `GET /api/vulnerabilities`
  - `GET /api/vulnerabilities/stats`
  - `GET /api/vulnerabilities/:id`
  - `POST /api/vulnerabilities`
  - `PUT /api/vulnerabilities/:id`
  - `DELETE /api/vulnerabilities/:id`
- Attack chain APIs:
  - `GET /api/attack-chain/:conversationId`
  - `POST /api/attack-chain/:conversationId/regenerate`

## State Machine
- Vulnerability lifecycle:
  - `Open`
  - `Confirmed`
  - `Fixed`
  - `FalsePositive`
- WebShell connection lifecycle:
  - `Configured`
  - `ContextBound`
  - `Updated`
  - `Deleted`
- Attack chain lifecycle:
  - `Missing`
  - `Generating`
  - `Ready`
  - `Regenerating`

## Data Flow
1. Reconnaissance intent is converted into a FOFA query or sent directly to FOFA search.
2. WebShell connection metadata is created once and reused for command, file, and assistant operations.
3. Server-side proxy logic issues the actual WebShell HTTP requests and returns normalized results to callers.
4. Conversation-linked findings can be promoted into vulnerability records.
5. Attack-chain generation reads conversation evidence, tool traces, and vulnerability context to produce nodes and edges.
6. Derived attack-chain graphs are persisted so later reads can reuse them until regeneration is requested.

## Constraints
- FOFA search requires valid FOFA credentials; natural-language parse also requires a configured LLM backend.
- WebShell operations must stay server-side and must never expose unrestricted filesystem or command access directly to the browser.
- WebShell connection state is isolated per connection ID and must not bleed into other shells.
- Vulnerability severity and status use a controlled vocabulary so dashboards and filters stay stable.
- Attack-chain generation is conversation-scoped and single-flight; one conversation may not generate multiple concurrent chains.
- Conversation ID is the primary linkage between chat evidence, vulnerabilities, and attack-chain artifacts.

## Failure Handling
- Invalid FOFA input, missing credentials, or invalid model configuration return actionable validation errors.
- Invalid WebShell URLs, missing connections, or remote execution failures do not delete stored connection metadata automatically.
- Concurrent attack-chain generation attempts for the same conversation return a conflict rather than racing.
- If an attack chain is stale or incorrect, explicit regeneration replaces the derived artifact without deleting the underlying conversation evidence.
- Vulnerability CRUD failures must be explicit so operators never assume a finding was recorded when persistence failed.
