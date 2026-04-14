# Conversation Workspace

## Purpose
Define the durable operator workspace for conversations, attachments, grouping, and execution history so work can be resumed, audited, and organized over time.

## Requirements
### Requirement: Durable conversation history
The system SHALL persist ordered conversation history under a stable conversation identifier.

#### Scenario: Conversation is resumed later
- **WHEN** an operator reopens an existing conversation
- **THEN** the system returns the persisted message history in durable order

#### Scenario: First message creates a conversation
- **WHEN** executable user input is submitted without a conversation identifier
- **THEN** the system creates a new conversation and stores subsequent messages under that identifier

### Requirement: Managed attachment workspace
Conversation attachments SHALL be stored under a controlled server-side root and remain referenceable by durable path.

#### Scenario: Uploaded file is attached to a conversation
- **WHEN** a user uploads a file for conversation use
- **THEN** the file is stored under the managed attachment root and can be referenced by server path

#### Scenario: Path traversal is attempted
- **WHEN** a file operation references a path outside the managed attachment root
- **THEN** the operation is rejected and no out-of-root mutation occurs

### Requirement: Process-detail evidence
Assistant-side execution traces SHALL be storable and retrievable as structured process details.

#### Scenario: Agent emits execution progress
- **WHEN** an assistant response produces intermediate execution events
- **THEN** the system stores those events as process details linked to the assistant message

#### Scenario: Conversation is retrieved in lightweight mode
- **WHEN** a caller requests a lightweight conversation view
- **THEN** durable messages remain available even if detailed process traces are omitted for performance

### Requirement: Workspace organization
The system SHALL support grouping and pinning as organizational metadata independent of message content.

#### Scenario: Conversation is pinned inside a group
- **WHEN** an operator updates pin state for a conversation within a group
- **THEN** the group-local pin metadata changes without rewriting conversation messages

### Requirement: Selective deletion semantics
The system SHALL support deleting a whole conversation or deleting a single turn without corrupting unrelated history.

#### Scenario: Single turn is deleted
- **WHEN** an operator deletes a conversation turn
- **THEN** only the addressed turn is removed and remaining conversation history stays intact

## Overview
The conversation workspace is the durable operator workspace for prompts, replies, file attachments, process timelines, grouping, and message history. It is the primary human-facing record of intent and outcome across chat, WebShell assistant sessions, and robot-initiated executions.

## Capabilities
### Capability 1: Durable Conversation Lifecycle
- The system must allow a conversation to be created explicitly or implicitly from the first executable user request.
- The system must persist ordered user and assistant messages under a stable conversation identifier.
- The system must generate a usable default title when the caller does not provide one.

### Capability 2: Rich Message Evidence
- The system must allow assistant messages to carry MCP execution references and structured process details.
- The system must preserve enough message metadata to reconstruct a human-readable execution timeline after refresh.

### Capability 3: Workspace Retrieval Modes
- The system must support conversation retrieval optimized for either full fidelity or lower-cost historical browsing.
- The system must preserve message order and message authority regardless of retrieval mode.

### Capability 4: Organizational Metadata
- The system must support named groups, global pin state, and group-local pin state.
- The system must treat organizational metadata as orthogonal to message content and execution evidence.

### Capability 5: Attachment Workspace
- The system must store attachments under a controlled server-side root.
- The system must reference attachments by durable server path instead of requiring repeated inline payload expansion.
- The system must support managed file browsing, editing, rename, directory creation, download, and deletion inside that controlled root.

## Interfaces
- Conversation APIs:
  - `POST /api/conversations`
  - `GET /api/conversations`
  - `GET /api/conversations/:id`
  - `PUT /api/conversations/:id`
  - `DELETE /api/conversations/:id`
  - `POST /api/conversations/:id/delete-turn`
  - `GET /api/messages/:id/process-details`
- Group APIs:
  - `POST /api/groups`
  - `GET /api/groups`
  - `GET /api/groups/:id`
  - `PUT /api/groups/:id`
  - `DELETE /api/groups/:id`
  - `POST /api/groups/conversations`
  - `DELETE /api/groups/:id/conversations/:conversationId`
- Attachment APIs:
  - `GET /api/chat-uploads`
  - `POST /api/chat-uploads`
  - `GET /api/chat-uploads/download`
  - `GET /api/chat-uploads/content`
  - `PUT /api/chat-uploads/content`
  - `PUT /api/chat-uploads/rename`
  - `POST /api/chat-uploads/mkdir`
  - `DELETE /api/chat-uploads`

## State Machine
- Conversation lifecycle:
  - `Nonexistent` -> `Active`
  - `Active` -> `Organized`
  - `Organized` -> `Active`
  - `Active` or `Organized` -> `Deleted`
- Message lifecycle:
  - `Created`
  - `Enriched` with process details, MCP execution IDs, or attachment references
  - `Finalized`
  - `Deleted`
- Attachment lifecycle:
  - `Uploaded`
  - `Associated` with a conversation
  - `Renamed` or `Edited`
  - `Deleted`

## Data Flow
1. A conversation is created directly or inferred from the first request.
2. Uploaded files are stored under `chat_uploads/<date>/<conversation-or-placeholder>/...`.
3. User messages persist before agent execution begins.
4. Assistant placeholders and process details accumulate during execution.
5. Final assistant content, tool execution IDs, and derived process records replace placeholders.
6. Group mappings and pin metadata reshape how conversations are listed without changing message history itself.

## Constraints
- Attachment count per request is bounded.
- Conversation attachments are referenced by server path and must remain under the controlled `chat_uploads` root.
- File operations must prevent path traversal and accidental escape outside the attachment root.
- Group membership and pin state are organizational metadata; they must not mutate message content.
- Process details are append-only execution evidence tied to a specific assistant message.
- When a conversation is retrieved in lightweight mode, the message history remains authoritative even if detailed process traces are omitted for performance.

## Failure Handling
- Referencing a nonexistent conversation or message returns a not-found error.
- Invalid attachment paths, invalid names, or path-escape attempts are rejected.
- File collisions are resolved by generating unique names rather than overwriting unrelated files silently.
- If process details cannot be fully loaded, the conversation remains readable and message history is still returned.
- Group operations on missing entities fail explicitly and must not create orphan mappings.
