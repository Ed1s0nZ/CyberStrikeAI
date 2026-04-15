## 1. Persistence And Bootstrap

- [ ] 1.1 Add SQLite tables and repository support for Web users, RBAC roles, role-permission grants, and user-role assignments.
- [ ] 1.2 Implement secure password hashing, per-user session state, and request-context identity fields for authenticated Web users.
- [ ] 1.3 Seed built-in Web access roles and bootstrap the initial `admin` account from the legacy shared password when no Web users exist.

## 2. Authentication And Authorization Backend

- [ ] 2.1 Refactor auth handlers and login payloads from shared-password authentication to username/password Web user authentication.
- [ ] 2.2 Add RBAC permission evaluation middleware for protected route families and return `401` versus `403` consistently.
- [ ] 2.3 Implement user-management and RBAC-management APIs with safeguards for last-super-admin protection, password reset, enable/disable, and assignment changes.

## 3. System Configuration UI

- [ ] 3.1 Update the login flow and session validation UI to collect username plus password and surface permission-aware failures.
- [ ] 3.2 Add Web user management screens to the system configuration module for listing, creating, editing, disabling, deleting, and resetting operator accounts.
- [ ] 3.3 Add Web RBAC role management screens and API wiring with terminology that clearly separates Web access roles from existing AI Agent roles.

## 4. Verification And Documentation

- [ ] 4.1 Add backend tests for bootstrap migration, per-user session revocation, RBAC allow/deny behavior, and administrative safety invariants.
- [ ] 4.2 Add or update frontend/manual verification coverage for login, user management, role assignment, and system-settings navigation behavior.
- [ ] 4.3 Update API documentation, settings copy, and operator-facing help text to distinguish Web access roles from AI Agent roles.
