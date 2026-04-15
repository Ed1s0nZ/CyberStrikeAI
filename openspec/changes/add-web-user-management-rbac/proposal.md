## Why

The platform currently protects the control plane with a single shared password and in-memory sessions, which makes operator identity, least-privilege access, and action accountability impossible. We need a Web user management model in the system configuration area so administrators can manage individual operator accounts and assign RBAC permissions without conflating them with the existing AI Agent role concept.

## What Changes

- Add Web user management to the system configuration module, including list, create, edit, enable/disable, password reset, and delete flows for operator accounts.
- Introduce RBAC for Web control-plane access, with platform permissions bound to Web users through RBAC roles.
- Keep AI Agent roles under `roles/` unchanged and explicitly separate from RBAC roles used for human operator authorization.
- Replace shared-password login for protected Web APIs with account-based authentication while preserving session validation, logout, and revocation semantics.
- Record the acting Web user identity on authenticated control-plane requests so authorization and audit behavior can distinguish who performed an operation.

## Capabilities

### New Capabilities
- `web-user-management`: Manage operator accounts, RBAC roles, role assignments, user lifecycle, and operator-facing administration workflows in the system configuration module.

### Modified Capabilities
- `access-control-and-configuration`: Change control-plane authentication from a single shared password model to authenticated Web user sessions with RBAC authorization and per-user identity.

## Impact

- Affected backend areas: authentication/session management, authorization middleware, configuration/security handlers, and persistence for Web users and RBAC metadata.
- Affected frontend areas: system settings navigation, security/settings pages, login flow, and user/role management interfaces.
- Affected APIs: auth endpoints, protected `/api/**` authorization checks, and new user/RBAC management endpoints.
- Operational impact: introduces bootstrap/seed admin handling, account lifecycle rules, and migration from existing shared-password deployments.
