# Robot Channel Integration

## Purpose
Define how enterprise messaging channels map into the platform's conversation and execution model without creating a separate logic path for robot-originated work.

## Requirements
### Requirement: Robot messages reuse platform execution semantics
Robot-originated user input SHALL flow into the same durable conversation and agent execution model used by Web and API clients.

#### Scenario: Robot user sends a non-command message
- **WHEN** a robot message is not interpreted as a workspace command
- **THEN** the system routes it into the standard agent execution path and persists the resulting conversation evidence

### Requirement: Robot users control their active workspace
Robot users SHALL be able to manage their active conversation context through commands.

#### Scenario: Robot user switches active conversation
- **WHEN** a robot user issues a valid switch command
- **THEN** the user's future robot messages bind to the selected conversation

### Requirement: Channel-specific ingress validation
Supported robot channels SHALL honor their callback and handshake rules where applicable.

#### Scenario: Channel requires callback verification
- **WHEN** a supported robot platform sends a verification or signed callback request
- **THEN** the platform validates and answers according to that channel's callback contract

### Requirement: Command-first interpretation
Robot input SHALL be interpreted as a workspace command before it is treated as agent text.

#### Scenario: User sends a stop command
- **WHEN** a robot user sends the stop command while a task is running for that user/session key
- **THEN** the running task is cancelled instead of forwarding the text to the agent as normal conversation input

### Requirement: Per-user role affinity
Robot users SHALL be able to keep and change an active role independently of other users.

#### Scenario: Robot user changes role
- **WHEN** a robot user switches to a valid role
- **THEN** subsequent non-command robot messages execute under that user's selected role until changed again

### Requirement: Local robot validation
The platform SHALL expose a local validation path for robot behavior without requiring live external delivery.

#### Scenario: Operator uses local robot test endpoint
- **WHEN** a caller invokes the local robot test API with platform, user, and text
- **THEN** the platform returns the same logical reply path that a live robot message would produce

## Overview
Robot channel integration exposes the platform through enterprise messaging systems. It maps robot users to platform conversations, supports a command vocabulary for workspace control, and reuses the same agent execution pipeline as the Web experience.

## Capabilities
### Capability 1: Multi-Channel Robot Ingress
- The system must accept robot traffic from the supported enterprise messaging channels.
- The system must honor platform-specific callback validation semantics where required.

### Capability 2: User-To-Conversation Affinity
- The system must maintain per-platform, per-user conversation affinity for follow-up exchanges.
- The system must allow that affinity to be explicitly switched or cleared through robot commands.

### Capability 3: Robot Workspace Commands
- The system must support a command vocabulary for help, listing, switching, creating, clearing, stopping, role selection, deleting, and version display.

### Capability 4: Shared Execution Pipeline
- The system must route non-command text into the same durable agent execution pipeline used by Web/API workflows.
- The system must support robot-triggered multi-agent execution when the robot multi-agent feature flag is enabled.

### Capability 5: Local Validation
- The system must expose a local robot test interface for operator validation without requiring live external delivery.

## Interfaces
- `GET /api/robot/wecom`
- `POST /api/robot/wecom`
- `POST /api/robot/dingtalk`
- `POST /api/robot/lark`
- `POST /api/robot/test`

## State Machine
- Robot user session lifecycle:
  - `Unbound`
  - `BoundConversation`
  - `SwitchedConversation`
  - `ClearedConversation`
- Robot task lifecycle:
  - `Idle`
  - `Running`
  - `Completed` or `Cancelled` or `Failed`

## Data Flow
1. A robot webhook request arrives from a platform-specific callback.
2. The system validates platform-specific handshake or signature requirements when configured.
3. The message is interpreted first as a robot command; if no command matches, it is treated as agent input.
4. The user-to-conversation mapping is created, reused, switched, or cleared based on the command/result.
5. The same conversation persistence and agent execution path as the Web/API runtime produces the reply.
6. The reply is formatted back into the robot platform's response contract.

## Constraints
- Robot session mappings are process-local in-memory state, not a cross-node durable session bus.
- Role selection defaults to the platform's default role when the user has not chosen one.
- Stop commands only affect the current running task for the specific platform/user pair.
- Robot channels must preserve the same conversation semantics as Web/API even if the transport payload differs.
- Platform-specific callbacks may support different richness levels, but every channel must at minimum acknowledge valid requests safely.

## Failure Handling
- Empty messages produce a friendly prompt instead of invoking the agent with an empty request.
- Invalid conversation switches, deletes, or role selections return user-facing error text rather than silent failure.
- Signature validation or challenge failures are rejected explicitly for channels that require them.
- If agent execution fails or is cancelled, the robot receives a terminal reply that reflects the actual outcome.
- Partial platform support is allowed so long as unsupported callback types fail safely and do not corrupt conversation mappings.
