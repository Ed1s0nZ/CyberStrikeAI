## MODIFIED Requirements

### Requirement: Authenticated control plane
Protected operational interfaces SHALL require a valid authenticated Web user session and SHALL authorize permission-gated operations against explicit canonical resource permission grants associated with the requested resource and action.

#### Scenario: Protected request without valid session
- **WHEN** a caller invokes a protected API without a valid authenticated Web user session
- **THEN** the request is rejected as unauthorized

#### Scenario: Protected request with required resource permission
- **WHEN** an authenticated Web user invokes a permission-gated protected API with an unexpired session and the effective canonical resource permission required for the requested resource action
- **THEN** the request is authorized to continue into the target domain handler

#### Scenario: Protected request without required resource permission
- **WHEN** an authenticated Web user invokes a permission-gated protected API with a valid session but lacks the effective canonical resource permission required for the requested resource action
- **THEN** the request is rejected as forbidden

#### Scenario: Super administrator bypasses resource-specific permission checks
- **WHEN** an authenticated Web user invokes a permission-gated protected API with a valid session and the `system.super_admin.grant` permission
- **THEN** the request is authorized even if the session does not list the narrower canonical resource permission required for that operation

#### Scenario: Route authorization uses canonical resource permissions
- **WHEN** a protected API route is registered
- **THEN** it MUST bind exactly one canonical `domain.resource.action` permission from the approved catalog
- **AND** `system.super_admin.grant` MUST continue to bypass the check

#### Scenario: Legacy permission identifiers are normalized before authorization
- **WHEN** previously persisted Web access role grants still contain retired function-category permission identifiers
- **THEN** the system MUST deterministically normalize them to canonical permission identifiers before those grants are used for authorization
