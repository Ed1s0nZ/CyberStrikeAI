## Context

The current control plane authentication model is intentionally simple: `AuthManager` validates one shared password from `config.yaml`, issues an in-memory bearer token, and grants access to all protected `/api/**` routes once the token is valid. This model does not represent operator identity, cannot express least-privilege rules, and makes it impossible to distinguish administrative actions performed by different people.

The requested change adds Web user management inside the system configuration module and adopts RBAC for control-plane access. The repository already has a `roles/` directory and role CRUD endpoints, but those roles define AI Agent behavior, prompt/tool boundaries, and skill bindings. They are not human-operator authorization roles and must remain a separate concept in both data model and UI.

The codebase already has two storage patterns we can reuse:
- Durable runtime configuration is persisted to `config.yaml`.
- Operational data and migration-friendly tables are stored in the SQLite database initialized by `internal/database/database.go`.

Because Web users, RBAC roles, and assignments are mutable operator data rather than static runtime config, this change needs a database-backed model plus targeted authorization checks on protected routes.

## Goals / Non-Goals

**Goals:**
- Introduce database-backed Web operator accounts with individual credentials, enable/disable state, and password reset/change flows.
- Introduce RBAC roles and permission grants for protected Web/API operations.
- Keep AI Agent roles fully separate from RBAC roles in naming, storage, API surface, and UI placement.
- Preserve existing session semantics where practical: login, logout, expiry, revocation, and fail-closed access checks.
- Provide a workable migration path from shared-password deployments to an initial administrator account.

**Non-Goals:**
- Replacing AI Agent role files in `roles/` or changing their behavior.
- Introducing external IAM, LDAP, OAuth, SSO, or organization-level directory sync.
- Designing fine-grained field-level permissions for every object in the platform.
- Reworking unrelated pages outside the authentication/login path, protected route middleware, and system configuration module.

## Decisions

### 1. Model Web users and RBAC roles as new database entities

Web users, RBAC roles, and user-role assignments will be stored in SQLite instead of `config.yaml`.

Why:
- The data is operational, frequently edited, and naturally relational.
- The existing database bootstrap already supports additive table creation and lightweight migrations.
- Passwords for human accounts should be stored as hashes, not as editable plaintext config.

Alternatives considered:
- Store Web users in `config.yaml`: rejected because it mixes secrets with runtime config, creates awkward concurrent edits, and does not scale well for assignments/history.
- Reuse `roles/` YAML files: rejected because those files represent AI behavior, not operator authorization.

### 2. Keep authentication and authorization separate but chained

Authentication will identify a Web user and create a session tied to `user_id`; authorization will then evaluate RBAC permissions for the requested route/action.

Why:
- The current middleware already centralizes protected route entry and can evolve into a two-step guard.
- Separating authn/authz avoids coupling session storage to specific permission layouts.
- This preserves a clear failure mode distinction: `401` for invalid session, `403` for authenticated but unauthorized.

Alternatives considered:
- Keep current “authenticated means full access” behavior and only guard the new user-management APIs: rejected because it leaves the broader control plane outside RBAC.
- Encode permissions directly on users with no roles: rejected because the user explicitly requested an RBAC model and role reuse is operationally simpler.

### 3. Define RBAC roles as Web access roles, not AI roles

The implementation will use distinct terminology such as “Web access roles” or “RBAC roles” in APIs and the system settings UI, while AI Agent roles remain under their existing pages/endpoints.

Why:
- The repository already uses “role” for AI Agent execution personas; overloading the same term would create operator and implementation ambiguity.
- The user explicitly called out that existing role management is not the RBAC concept.

Alternatives considered:
- Merge both into a unified role object: rejected because the concerns, lifecycle, and consumers are entirely different.
- Hide the distinction in UI only: rejected because the ambiguity would remain in APIs, handlers, and persistence.

### 4. Seed an initial administrator from the existing shared-password deployment path

On upgrade, if no Web users exist, the system will bootstrap a default administrator account using a reserved username such as `admin` and the existing configured auth password as its initial password, then require all future logins to use account credentials.

Why:
- Existing deployments already rely on a configured password; reusing it avoids a dead-end upgrade that locks operators out.
- This keeps rollback simple because legacy auth data still exists during the cutover window.
- It minimizes operational steps while moving the platform toward per-user identity.

Alternatives considered:
- Require a new manual bootstrap command before startup: rejected for higher operational friction.
- Keep shared-password login indefinitely alongside user login: rejected because it bypasses RBAC and undermines identity/accountability.

### 5. Start with coarse-grained route-family permissions

The permission catalog should map to stable platform domains, for example: `system.config.read`, `system.config.write`, `security.users.manage`, `security.rbac.manage`, `conversation.read`, `conversation.write`, `extensions.roles.manage`, and similar route-family scopes.

Why:
- Route-family permissions are implementable with current handler organization and avoid over-designing per-field authorization.
- They provide meaningful least privilege without forcing every endpoint to invent a custom rule language.

Alternatives considered:
- Endpoint-by-endpoint custom permissions only: rejected because it increases maintenance cost with limited near-term value.
- One global admin flag: rejected because it is not RBAC.

### 6. Protect privileged-admin safety invariants in the service layer

The system will enforce invariants such as “there must always be at least one enabled super administrator” and “built-in bootstrap permissions cannot be removed in a way that orphans administration.”

Why:
- User and role management are self-hosting features; without safeguards, an admin could lock the platform out of further management.
- These checks belong in backend domain logic, not only in UI validation.

Alternatives considered:
- UI-only safeguards: rejected because API callers could bypass them.

## Risks / Trade-offs

- [Migration ambiguity for old deployments] -> Use deterministic bootstrap behavior when no Web users exist, document the initial username, and keep the legacy password only as the seed for the first admin account.
- [Permission catalog too coarse] -> Start with route-family permissions and leave room for additive refinement without changing the identity model.
- [Operator confusion with existing AI roles] -> Use distinct labels, endpoints, and settings sections for “AI Agent roles” versus “Web access roles”.
- [In-memory session store remains process-local] -> Keep current session scope for this phase and document that HA/shared-session support is out of scope.
- [Admin lockout through bad role edits] -> Enforce last-super-admin protection and fail writes that would remove all administrative access.

## Migration Plan

1. Add database tables for Web users, RBAC roles, permissions, and assignments during startup initialization.
2. Seed built-in RBAC roles and permission grants if they do not already exist.
3. If no Web users exist, create the initial `admin` user with the existing configured auth password and bind it to the built-in super-admin role.
4. Update login and protected middleware to authenticate by username/password and attach `user_id`, username, and effective permissions to request context.
5. Introduce user-management and RBAC-management APIs plus the system settings UI entry points.
6. Remove shared-password login as a valid protected access path once the bootstrap account exists.
7. Rollback strategy: code rollback can continue to honor the legacy `auth.password` path; database additions are additive, so rollback does not require destructive schema changes.

## Open Questions

- Should the first release expose fully custom RBAC role CRUD immediately, or ship built-in roles first with custom role editing enabled in the same phase? The design assumes custom role CRUD is included because the request explicitly calls for an RBAC model rather than fixed access levels.
- Should operator audit trails include `user_id` on all future write actions now, or only attach user identity to request context in this phase and let domain-specific audit expansion happen later? This proposal assumes the latter unless a nearby handler already records actor metadata.
