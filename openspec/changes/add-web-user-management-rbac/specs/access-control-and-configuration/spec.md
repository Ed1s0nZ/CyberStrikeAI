## ADDED Requirements

### Requirement: Bootstrap administrator access
The platform SHALL provide a migration-safe initial administrator path when upgrading from deployments that only have the legacy shared-password authentication model.

#### Scenario: Bootstrap administrator is created on first RBAC startup
- **WHEN** the platform starts with Web user authentication enabled and no durable Web user accounts exist yet
- **THEN** the platform creates an initial administrator account from the legacy configured auth secret and grants it super-administrator access

### Requirement: Authenticated request identity context
The platform SHALL attach the authenticated Web user identity to protected request context after successful authentication and authorization.

#### Scenario: Authorized request reaches downstream handler
- **WHEN** a protected request is accepted for an authenticated Web user
- **THEN** downstream handlers can access the acting user's stable identity from request context for authorization-aware processing and audit use

## MODIFIED Requirements

### Requirement: Authenticated control plane
Protected operational interfaces SHALL require a valid authenticated Web user session and SHALL enforce RBAC permission checks before the request is authorized to continue into the target domain handler.

#### Scenario: Protected request without valid session
- **WHEN** a caller invokes a protected API without a valid authenticated Web user session
- **THEN** the request is rejected as unauthorized

#### Scenario: Protected request with sufficient permission
- **WHEN** an authenticated Web user invokes a protected API with an unexpired session and sufficient effective permissions for the requested operation
- **THEN** the request is authorized to continue into the target domain handler

#### Scenario: Protected request without sufficient permission
- **WHEN** an authenticated Web user invokes a protected API with a valid session but lacks the effective permissions required for the requested operation
- **THEN** the request is rejected as forbidden

### Requirement: Session lifecycle enforcement
The platform SHALL enforce session expiry and revocation semantics for authenticated Web user sessions.

#### Scenario: Session expires
- **WHEN** a previously issued Web user session token is validated after its expiry time
- **THEN** the token is treated as invalid and is no longer accepted

#### Scenario: User disable revokes sessions
- **WHEN** a Web user account is disabled after sessions have already been issued for that user
- **THEN** those previously issued sessions are revoked and must not remain usable

#### Scenario: Password change invalidates that user's sessions
- **WHEN** a Web user's password is changed or reset successfully
- **THEN** previously issued sessions for that user are revoked and must not remain usable
