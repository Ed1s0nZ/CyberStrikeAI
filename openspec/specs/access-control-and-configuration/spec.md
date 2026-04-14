# Access Control And Configuration

## Purpose
Define who may operate the platform and how runtime behavior changes become durable, validated, and safely applied.

## Requirements
### Requirement: Authenticated control plane
Protected operational interfaces SHALL require valid authentication.

#### Scenario: Protected request without valid session
- **WHEN** a caller invokes a protected API without a valid bearer session
- **THEN** the request is rejected as unauthorized

#### Scenario: Protected request with valid session
- **WHEN** a caller invokes a protected API with a valid unexpired bearer session
- **THEN** the request is authorized to continue into the target domain handler

### Requirement: Durable configuration changes
Accepted configuration edits SHALL be persisted before they become long-lived platform intent.

#### Scenario: Operator updates runtime settings
- **WHEN** a configuration update is accepted
- **THEN** the canonical config file is updated before the change is treated as durable intent

#### Scenario: Config save fails
- **WHEN** a configuration change cannot be written to the canonical config file
- **THEN** the platform reports the save failure and does not claim the new settings are durably accepted

### Requirement: Session lifecycle enforcement
The platform SHALL enforce session expiry and revocation semantics for operator sessions.

#### Scenario: Session expires
- **WHEN** a previously issued session token is validated after its expiry time
- **THEN** the token is treated as invalid and is no longer accepted

#### Scenario: Password changes invalidate sessions
- **WHEN** the platform password is updated successfully
- **THEN** previously issued sessions are revoked and must not remain usable

### Requirement: Safe runtime apply
Runtime configuration apply SHALL update supported subsystems explicitly and surface any apply failure.

#### Scenario: Apply succeeds
- **WHEN** an operator applies a valid saved configuration
- **THEN** supported runtime subsystems reload their effective settings without requiring full process restart

#### Scenario: Apply fails during optional subsystem reload
- **WHEN** runtime apply cannot reinitialize or refresh a supported subsystem
- **THEN** the platform reports the apply failure and leaves the degradation visible rather than silently masking it

### Requirement: Conditional MCP header authentication
MCP HTTP requests SHALL honor configured header-based authentication when MCP auth headers are enabled.

#### Scenario: MCP request without required header
- **WHEN** MCP header authentication is configured and the request omits the required header or value
- **THEN** the request is rejected as unauthorized

#### Scenario: MCP request with required header
- **WHEN** MCP header authentication is configured and the request supplies the required header and value
- **THEN** the request is forwarded to the MCP handler

## Overview
This domain governs who may operate the platform and how runtime behavior is changed safely. It covers password-based authentication, session lifecycle, protected API access, configuration persistence, hot-apply semantics, and feature-level runtime reconfiguration.

## Capabilities
### Capability 1: Password-Based Operator Access
- The system must authenticate operators with the configured shared password.
- The system must issue expiring bearer sessions on successful authentication.
- The system must reject invalid credentials without creating session state.

### Capability 2: Session Revocation And Expiry
- The system must revoke the current session on logout.
- The system must invalidate expired sessions during validation.
- The system must revoke all sessions when the platform password changes.

### Capability 3: Protected Configuration And Control Plane
- The system must require authentication for protected operational APIs.
- The system must fail closed when a protected request is missing or carries an invalid session token.
- The system must separately enforce MCP header authentication when MCP auth headers are configured.

### Capability 4: Durable Configuration Management
- The system must persist accepted configuration changes to the canonical YAML file before treating them as long-lived operator intent.
- The system must support hot-apply for domains that can be reloaded safely at runtime.
- The system must expose explicit success or failure for both config save and config apply operations.

### Capability 5: Upstream Connectivity Validation
- The system must provide a dedicated way to test model endpoint connectivity without implicitly promoting the test settings into active runtime state.

## Interfaces
- Authentication APIs:
  - `POST /api/auth/login`
  - `POST /api/auth/logout`
  - `POST /api/auth/change-password`
  - `GET /api/auth/validate`
- Configuration APIs:
  - `GET /api/config`
  - `GET /api/config/tools`
  - `PUT /api/config`
  - `POST /api/config/apply`
  - `POST /api/config/test-openai`
- Protected API boundary: all operational `/api/**` routes except explicit public robot callbacks and documentation endpoints.
- MCP auth boundary: `/mcp` requires a configured header/value pair when MCP auth is enabled.

## State Machine
- Session lifecycle:
  - `Unauthenticated` -> `Authenticated` -> `Expired`
  - `Authenticated` -> `Revoked`
- Configuration lifecycle:
  - `Persisted` -> `Edited`
  - `Edited` -> `Saved`
  - `Saved` -> `Applying`
  - `Applying` -> `Applied`
  - `Applying` -> `ApplyFailed`
  - `ApplyFailed` -> `Applying` or `Saved`

## Data Flow
1. An operator authenticates with the current platform password.
2. The platform issues a time-bounded bearer token and attaches it to subsequent protected requests.
3. Configuration edits are submitted through the config API and persisted to the YAML source of truth.
4. Apply requests rebuild or refresh affected runtime services such as tools, agent settings, knowledge retrieval, multi-agent flags, and robot connections.
5. On success, new runtime settings become authoritative for future requests. Existing historical records remain unchanged.

## Constraints
- Authentication is password-based, not user-account-based.
- Sessions are in-memory and therefore process-local; they are not a cross-node SSO mechanism.
- Password changes must invalidate all existing sessions.
- New passwords must be non-empty and meet the platform's minimum length requirement.
- Configuration changes must never widen access unintentionally; auth failures remain fail-closed during and after apply.
- Runtime apply must preserve mandatory built-in capabilities such as vulnerability recording, WebShell tools, skills tools, and queue tools.
- MCP header authentication is conditional: absent configuration means no extra MCP header gate; present configuration means strict enforcement.

## Failure Handling
- Invalid credentials return authentication failure and never create a session.
- Expired or revoked tokens are rejected as unauthorized.
- Configuration save failure must be surfaced immediately so operators know the change was not durably recorded.
- Apply failure may leave the platform partially degraded, but the platform must surface that degradation rather than silently accepting stale behavior.
- Model connectivity testing returns structured success/failure feedback and does not itself mutate active runtime config.
