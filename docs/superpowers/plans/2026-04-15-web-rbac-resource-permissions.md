# Web RBAC Resource Permissions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the current coarse-grained Web RBAC permission model with the approved platform-wide `domain.resource.action` catalog, migrate persisted role grants safely, bind every protected business route to explicit resource permissions, and update the Web role-management UI to configure permissions by domain and resource.

**Architecture:** Keep the existing Web auth, SQLite, and Gin middleware model intact, but introduce a canonical permission catalog, a testable route-permission registry, and deterministic legacy-permission normalization. Backend remains the source of truth for canonical permission identifiers and permission grouping structure; frontend consumes that structure to render grouped role-assignment UI without hardcoded legacy permission lists.

**Tech Stack:** Go, Gin, SQLite, vanilla JavaScript, OpenAPI generation, Go test

---

## Planned File Map

- `openspec/changes/refine-web-rbac-resource-permissions/proposal.md`
  - Rewrite the change scope and breaking-change text to the approved full permission catalog.
- `openspec/changes/refine-web-rbac-resource-permissions/design.md`
  - Replace the old `config/security`-only design with the approved platform-wide resource-permission design.
- `openspec/changes/refine-web-rbac-resource-permissions/specs/access-control-and-configuration/spec.md`
  - Update capability deltas for explicit route-to-resource authorization and migration behavior.
- `openspec/changes/refine-web-rbac-resource-permissions/specs/web-user-management/spec.md`
  - Update role-management capability deltas for grouped resource-permission assignment.
- `openspec/changes/refine-web-rbac-resource-permissions/tasks.md`
  - Align OpenSpec tasks with the new implementation phases.
- `internal/security/permissions.go`
  - Canonical permission constants, legacy mappings, permission validation, super-admin bypass.
- `internal/security/permission_catalog.go`
  - Canonical domain/resource/action catalog and frontend-facing grouping structure.
- `internal/security/permission_catalog_test.go`
  - Catalog completeness, legacy expansion, and canonical-permission validation tests.
- `internal/security/route_permissions.go`
  - Central route-permission registry for every protected API route.
- `internal/security/route_permissions_test.go`
  - Route-permission coverage tests for representative protected routes across all business domains.
- `internal/security/web_rbac_migration.go`
  - Startup normalization for persisted Web RBAC grants.
- `internal/database/web_auth.go`
  - Normalize permission writes and resolved-user permission reads.
- `internal/database/web_auth_admin.go`
  - Normalize role reads/writes, role update behavior, and permission lookups.
- `internal/database/web_auth_test.go`
  - Persistence normalization and upgraded-role behavior tests.
- `internal/security/bootstrap.go`
  - Seed bootstrap role with canonical super-admin permission.
- `internal/security/bootstrap_test.go`
  - Verify bootstrap admin keeps canonical super-admin permission.
- `internal/security/auth_middleware.go`
  - Route-permission lookup helper and authorization behavior.
- `internal/security/auth_middleware_test.go`
  - Canonical permission allow/deny and super-admin bypass tests.
- `internal/security/auth_manager_test.go`
  - Session permission immutability tests with canonical permissions.
- `internal/app/app.go`
  - Rebind all protected routes to canonical route permissions and register the permission-catalog endpoint.
- `internal/handler/web_access_roles.go`
  - Reject retired permission identifiers and expose the permission-catalog API.
- `internal/handler/web_users.go`
  - Update last-super-admin checks to canonical super-admin permission.
- `internal/handler/auth.go`
  - Return canonical permissions in `/api/auth/validate`.
- `internal/handler/openapi.go`
  - Publish canonical permission payloads, validate response shape, and permission-catalog endpoint schema.
- `internal/handler/auth_test.go`
  - Validate endpoint response now includes canonical permissions.
- `internal/handler/openapi_test.go`
  - OpenAPI includes the new security permission-catalog endpoint and updated auth schema.
- `internal/handler/web_users_test.go`
  - Security role CRUD, legacy-permission rejection, and catalog endpoint tests.
- `web/static/js/web-users.js`
  - Replace hardcoded legacy permission list with grouped catalog rendering from backend.
- `web/static/i18n/zh-CN.json`
  - Domain/resource/action labels and grouped permission UI copy in Chinese.
- `web/static/i18n/en-US.json`
  - Domain/resource/action labels and grouped permission UI copy in English.
- `docs/superpowers/specs/2026-04-15-web-rbac-resource-permissions-design.md`
  - Reference-only design artifact already approved; do not rewrite during implementation unless scope changes again.

### Task 1: Rewrite OpenSpec Artifacts To Match The Approved Scope

**Files:**
- Modify: `openspec/changes/refine-web-rbac-resource-permissions/proposal.md`
- Modify: `openspec/changes/refine-web-rbac-resource-permissions/design.md`
- Modify: `openspec/changes/refine-web-rbac-resource-permissions/specs/access-control-and-configuration/spec.md`
- Modify: `openspec/changes/refine-web-rbac-resource-permissions/specs/web-user-management/spec.md`
- Modify: `openspec/changes/refine-web-rbac-resource-permissions/tasks.md`

- [ ] **Step 1: Prove the current OpenSpec docs are still narrow-scope**

Run:

```bash
rg -n "config/security|不在本次变更里扩展新的业务资源|config\\.settings|security\\.web_users|security\\.web_access_roles" \
  openspec/changes/refine-web-rbac-resource-permissions
```

Expected: hits in `proposal.md` and `design.md` that still describe the old `config/security`-only model.

- [ ] **Step 2: Rewrite `proposal.md` to the full canonical permission catalog**

Replace the `What Changes` and `BREAKING` sections with content shaped like:

```md
- 将 Web RBAC 权限模型从“按功能分类授权”调整为“按资源授权”，并覆盖信息收集、任务管理、漏洞管理、WebShell 管理、文件管理、MCP、知识、Skills、Agents、角色、系统设置等受保护业务域。
- 权限标识统一采用 `domain.resource.action` 命名。
- **BREAKING**: 已持久化的旧权限标识仅做确定性规范化迁移；此前仅依赖登录态访问的业务 API 现在也将纳入显式资源权限控制，现有非超级管理员角色在未重新授权前可能对这些业务域收到 `403`。
- **BREAKING**: 本次 canonical permission catalog 包含 `intel.fofa_query.execute`、`task.batch_queue.*`、`task.conversation.*`、`vulnerability.record.*`、`webshell.connection.*`、`file.workspace_entry.*`、`mcp.external_server.*`、`knowledge.item.*`、`skill.definition.*`、`agent.run.*`、`role.agent_role.*`、`system.web_user.*`、`system.web_access_role.*`、`system.super_admin.grant` 等全量权限。
```

- [ ] **Step 3: Rewrite `design.md` to match the approved superpowers design**

Replace the old decisions with the approved decisions:

```md
### 1. 采用平台级 `domain.resource.action` 资源权限模型
- 业务域固定为 `intel`、`task`、`vulnerability`、`webshell`、`file`、`mcp`、`knowledge`、`skill`、`agent`、`role`、`system`
- 基础动作固定为 `read/create/update/delete`
- 特例动作固定为 `execute/start/stop/test/reset/apply/grant/regenerate`

### 2. 新增受保护业务域不对旧的非超级管理员角色做自动放权
- 旧权限只做确定性映射
- `system.super_admin` 迁移为 `system.super_admin.grant`
- 新增业务域权限必须由管理员显式分配
```

- [ ] **Step 4: Update both delta specs and OpenSpec task list**

Put concrete requirement text into both spec deltas:

```md
#### Scenario: Route authorization uses canonical resource permissions
- **WHEN** a protected API route is registered
- **THEN** it MUST bind exactly one canonical `domain.resource.action` permission from the approved catalog
- **AND** `system.super_admin.grant` MUST continue to bypass the check

#### Scenario: Web access role assignment is grouped by business domain and resource
- **WHEN** an operator edits a Web access role in system settings
- **THEN** the UI MUST present permissions grouped by business domain and resource
- **AND** the submitted payload MUST contain only canonical permission identifiers
```

Update `tasks.md` to reflect the tasks in this plan:

```md
## 1. OpenSpec Alignment
- [ ] 1.1 Rewrite proposal, design, and delta specs to the approved platform-wide resource model.

## 2. Canonical Catalog And Migration
- [ ] 2.1 Add canonical permissions, route-permission registry, and legacy normalization helpers.
- [ ] 2.2 Normalize persisted roles and bootstrap permissions to canonical identifiers.

## 3. Authorization And APIs
- [ ] 3.1 Rebind all protected routes to canonical route permissions.
- [ ] 3.2 Reject retired permission identifiers and expose the permission catalog and current-session permissions.

## 4. UI And Verification
- [ ] 4.1 Replace the legacy hardcoded permission checklist with grouped catalog rendering.
- [ ] 4.2 Run backend regression tests and manual UI verification for upgraded roles.
```

- [ ] **Step 5: Re-run the scope check**

Run:

```bash
rg -n "config/security|不在本次变更里扩展新的业务资源|security\\.web_users|security\\.web_access_roles" \
  openspec/changes/refine-web-rbac-resource-permissions
```

Expected: no remaining hits for the retired narrow-scope wording.

- [ ] **Step 6: Commit**

```bash
git add openspec/changes/refine-web-rbac-resource-permissions/proposal.md \
        openspec/changes/refine-web-rbac-resource-permissions/design.md \
        openspec/changes/refine-web-rbac-resource-permissions/specs/access-control-and-configuration/spec.md \
        openspec/changes/refine-web-rbac-resource-permissions/specs/web-user-management/spec.md \
        openspec/changes/refine-web-rbac-resource-permissions/tasks.md
git commit -m "docs: expand web rbac change scope"
```

### Task 2: Add The Canonical Permission Catalog And Validation Helpers

**Files:**
- Modify: `internal/security/permissions.go`
- Create: `internal/security/permission_catalog.go`
- Create: `internal/security/permission_catalog_test.go`

- [ ] **Step 1: Write failing catalog and validation tests**

Create `internal/security/permission_catalog_test.go` with:

```go
package security

import "testing"

func TestCanonicalPermissionCatalogIncludesPlatformDomains(t *testing.T) {
	required := []string{
		"intel.fofa_query.execute",
		"task.batch_queue.start",
		"vulnerability.record.update",
		"webshell.command.execute",
		"file.workspace_content.update",
		"mcp.external_server.stop",
		"knowledge.search.execute",
		"skill.definition.delete",
		"agent.multi_run.execute",
		"role.agent_role.update",
		"system.super_admin.grant",
	}

	all := CanonicalWebPermissions()
	set := make(map[string]struct{}, len(all))
	for _, permission := range all {
		set[permission] = struct{}{}
	}

	for _, permission := range required {
		if _, ok := set[permission]; !ok {
			t.Fatalf("expected canonical permission %q to exist", permission)
		}
	}
}

func TestNormalizeWebPermissionsExpandsLegacyValues(t *testing.T) {
	got := NormalizeWebPermissions([]string{
		"system.config.write",
		"security.users.manage",
		"system.super_admin",
	})

	required := []string{
		"system.config_settings.update",
		"system.runtime_config.apply",
		"system.model_connectivity.test",
		"system.web_user.read",
		"system.web_user.create",
		"system.web_user.update",
		"system.web_user.delete",
		"system.web_user_credential.reset",
		"system.super_admin.grant",
	}

	set := make(map[string]struct{}, len(got))
	for _, permission := range got {
		set[permission] = struct{}{}
	}
	for _, permission := range required {
		if _, ok := set[permission]; !ok {
			t.Fatalf("expected normalized permission %q in %#v", permission, got)
		}
	}
}

func TestIsCanonicalWebPermissionRejectsRetiredPermission(t *testing.T) {
	if IsCanonicalWebPermission("security.users.manage") {
		t.Fatal("expected retired permission to be rejected")
	}
	if !IsCanonicalWebPermission("system.web_user.read") {
		t.Fatal("expected canonical permission to be accepted")
	}
}
```

- [ ] **Step 2: Run the catalog tests and verify RED**

Run:

```bash
go test ./internal/security -run 'TestCanonicalPermissionCatalogIncludesPlatformDomains|TestNormalizeWebPermissionsExpandsLegacyValues|TestIsCanonicalWebPermissionRejectsRetiredPermission' -v
```

Expected: `FAIL` because the canonical catalog and normalization helpers do not exist yet.

- [ ] **Step 3: Add canonical permission constants, legacy mappings, and validation helpers**

Update `internal/security/permissions.go` to follow this shape:

```go
package security

const (
	PermissionSuperAdminGrant = "system.super_admin.grant"

	PermissionIntelFofaQueryExecute = "intel.fofa_query.execute"

	PermissionTaskBatchQueueRead   = "task.batch_queue.read"
	PermissionTaskBatchQueueCreate = "task.batch_queue.create"
	PermissionTaskBatchQueueUpdate = "task.batch_queue.update"
	PermissionTaskBatchQueueDelete = "task.batch_queue.delete"
	PermissionTaskBatchQueueStart  = "task.batch_queue.start"
	PermissionTaskBatchQueueStop   = "task.batch_queue.stop"

	PermissionSystemConfigSettingsRead   = "system.config_settings.read"
	PermissionSystemConfigSettingsUpdate = "system.config_settings.update"
	PermissionSystemRuntimeConfigApply   = "system.runtime_config.apply"
	PermissionSystemModelConnectivityTest = "system.model_connectivity.test"
	PermissionSystemWebUserRead          = "system.web_user.read"
	PermissionSystemWebUserCreate        = "system.web_user.create"
	PermissionSystemWebUserUpdate        = "system.web_user.update"
	PermissionSystemWebUserDelete        = "system.web_user.delete"
	PermissionSystemWebUserCredentialReset = "system.web_user_credential.reset"
	PermissionSystemWebAccessRoleRead    = "system.web_access_role.read"
	PermissionSystemWebAccessRoleCreate  = "system.web_access_role.create"
	PermissionSystemWebAccessRoleUpdate  = "system.web_access_role.update"
	PermissionSystemWebAccessRoleDelete  = "system.web_access_role.delete"
)

var legacyPermissionMap = map[string][]string{
	"system.config.read": {
		PermissionSystemConfigSettingsRead,
	},
	"system.config.write": {
		PermissionSystemConfigSettingsUpdate,
		PermissionSystemRuntimeConfigApply,
		PermissionSystemModelConnectivityTest,
	},
	"security.users.manage": {
		PermissionSystemWebUserRead,
		PermissionSystemWebUserCreate,
		PermissionSystemWebUserUpdate,
		PermissionSystemWebUserDelete,
		PermissionSystemWebUserCredentialReset,
	},
	"security.roles.manage": {
		PermissionSystemWebAccessRoleRead,
		PermissionSystemWebAccessRoleCreate,
		PermissionSystemWebAccessRoleUpdate,
		PermissionSystemWebAccessRoleDelete,
	},
	"system.super_admin": {
		PermissionSuperAdminGrant,
	},
}

func HasPermission(permissionSet map[string]struct{}, required string) bool {
	if _, ok := permissionSet[PermissionSuperAdminGrant]; ok {
		return true
	}
	_, ok := permissionSet[required]
	return ok
}
```

- [ ] **Step 4: Add the platform-wide catalog file**

Create `internal/security/permission_catalog.go` with:

```go
package security

import "sort"

type PermissionResource struct {
	Domain      string
	Resource    string
	Permissions []string
}

var canonicalPermissionCatalog = []PermissionResource{
	{Domain: "intel", Resource: "fofa_query", Permissions: []string{PermissionIntelFofaQueryExecute}},
	{Domain: "task", Resource: "batch_queue", Permissions: []string{
		PermissionTaskBatchQueueRead,
		PermissionTaskBatchQueueCreate,
		PermissionTaskBatchQueueUpdate,
		PermissionTaskBatchQueueDelete,
		PermissionTaskBatchQueueStart,
		PermissionTaskBatchQueueStop,
	}},
	{Domain: "system", Resource: "web_user", Permissions: []string{
		PermissionSystemWebUserRead,
		PermissionSystemWebUserCreate,
		PermissionSystemWebUserUpdate,
		PermissionSystemWebUserDelete,
	}},
	{Domain: "system", Resource: "super_admin", Permissions: []string{PermissionSuperAdminGrant}},
}

func CanonicalWebPermissions() []string {
	var out []string
	for _, resource := range canonicalPermissionCatalog {
		out = append(out, resource.Permissions...)
	}
	sort.Strings(out)
	return out
}

func IsCanonicalWebPermission(permission string) bool {
	for _, known := range CanonicalWebPermissions() {
		if permission == known {
			return true
		}
	}
	return false
}

func NormalizeWebPermissions(input []string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, permission := range input {
		for _, normalized := range expandLegacyPermission(permission) {
			if normalized == "" {
				continue
			}
			if _, ok := seen[normalized]; ok {
				continue
			}
			seen[normalized] = struct{}{}
			out = append(out, normalized)
		}
	}
	sort.Strings(out)
	return out
}

func expandLegacyPermission(permission string) []string {
	if mapped, ok := legacyPermissionMap[permission]; ok {
		return mapped
	}
	return []string{permission}
}
```

In this same step, populate `canonicalPermissionCatalog` with the entire approved permission set from the design document so that every protected business domain is present before any route rebinding starts.

- [ ] **Step 5: Re-run the focused tests and verify GREEN**

Run:

```bash
go test ./internal/security -run 'TestCanonicalPermissionCatalogIncludesPlatformDomains|TestNormalizeWebPermissionsExpandsLegacyValues|TestIsCanonicalWebPermissionRejectsRetiredPermission' -v
```

Expected: `PASS`

- [ ] **Step 6: Commit**

```bash
git add internal/security/permissions.go \
        internal/security/permission_catalog.go \
        internal/security/permission_catalog_test.go
git commit -m "feat: add canonical web permission catalog"
```

### Task 3: Normalize Persisted Roles And Bootstrap Permissions

**Files:**
- Modify: `internal/database/web_auth.go`
- Modify: `internal/database/web_auth_admin.go`
- Modify: `internal/database/web_auth_test.go`
- Create: `internal/security/web_rbac_migration.go`
- Modify: `internal/security/bootstrap.go`
- Modify: `internal/security/bootstrap_test.go`
- Modify: `internal/security/auth_manager_test.go`
- Modify: `internal/app/app.go`

- [ ] **Step 1: Write failing normalization and bootstrap tests**

Add these tests:

```go
func TestListWebAccessRolesNormalizesLegacyPermissions(t *testing.T) {
	db := openTestDB(t)

	roleID, err := db.CreateWebAccessRole(CreateWebAccessRoleInput{
		Name:        "legacy-role",
		Description: "legacy permissions",
		Permissions: []string{"system.config.write", "security.users.manage", "system.super_admin"},
		IsSystem:    false,
	})
	if err != nil {
		t.Fatalf("create role: %v", err)
	}

	role, err := db.GetWebAccessRoleByID(roleID)
	if err != nil {
		t.Fatalf("get role: %v", err)
	}

	expected := []string{
		"system.config_settings.update",
		"system.runtime_config.apply",
		"system.model_connectivity.test",
		"system.web_user.read",
		"system.web_user.create",
		"system.web_user.update",
		"system.web_user.delete",
		"system.web_user_credential.reset",
		"system.super_admin.grant",
	}
	for _, permission := range expected {
		if !contains(role.Permissions, permission) {
			t.Fatalf("expected role permission %q in %#v", permission, role.Permissions)
		}
	}
}

func TestEnsureBootstrapAdminSeedsCanonicalSuperAdmin(t *testing.T) {
	db := openBootstrapTestDB(t)
	if err := EnsureBootstrapAdmin(t.Context(), db, "change-me-now"); err != nil {
		t.Fatalf("EnsureBootstrapAdmin() error = %v", err)
	}

	admin, err := db.GetWebUserWithPermissionsByUsername("admin")
	if err != nil {
		t.Fatalf("GetWebUserWithPermissionsByUsername(admin) error = %v", err)
	}
	if !contains(admin.Permissions, PermissionSuperAdminGrant) {
		t.Fatalf("expected admin to include %q, got %#v", PermissionSuperAdminGrant, admin.Permissions)
	}
}
```

- [ ] **Step 2: Run the database and bootstrap tests and verify RED**

Run:

```bash
go test ./internal/database ./internal/security -run 'TestListWebAccessRolesNormalizesLegacyPermissions|TestEnsureBootstrapAdminSeedsCanonicalSuperAdmin|TestAuthManager' -v
```

Expected: `FAIL` because writes, reads, and bootstrap seeding still use the retired permission identifiers.

- [ ] **Step 3: Normalize role writes and resolved permission reads**

Update both database write paths to normalize before insert:

```go
func (db *DB) CreateWebAccessRole(input CreateWebAccessRoleInput) (string, error) {
	input.Permissions = security.NormalizeWebPermissions(input.Permissions)
	// existing insert logic continues
}

func (db *DB) UpdateWebAccessRole(input UpdateWebAccessRoleInput) (*WebAccessRole, error) {
	input.Permissions = security.NormalizeWebPermissions(input.Permissions)
	// existing update logic continues
}
```

Normalize role reads and resolved user permissions:

```go
if permission.Valid {
	role.Permissions = append(role.Permissions, permission.String)
}
// after scan loop
role.Permissions = security.NormalizeWebPermissions(role.Permissions)
```

```go
user.Permissions = security.NormalizeWebPermissions(user.Permissions)
```

- [ ] **Step 4: Add startup normalization and update bootstrap seeding**

Create `internal/security/web_rbac_migration.go`:

```go
package security

import "cyberstrike-ai/internal/database"

func NormalizePersistedWebRBACPermissions(db *database.DB) error {
	roles, err := db.ListWebAccessRoles()
	if err != nil {
		return err
	}
	for _, role := range roles {
		normalized := NormalizeWebPermissions(role.Permissions)
		if equalStringSlices(normalized, role.Permissions) {
			continue
		}
		if _, err := db.UpdateWebAccessRole(database.UpdateWebAccessRoleInput{
			ID:          role.ID,
			Name:        role.Name,
			Description: role.Description,
			Permissions: normalized,
		}); err != nil {
			return err
		}
	}
	return nil
}
```

Update bootstrap seeding:

```go
if _, err := tx.ExecContext(
	ctx,
	`INSERT OR IGNORE INTO web_access_role_permissions (role_id, permission) VALUES (?, ?)`,
	roleID, PermissionSuperAdminGrant,
); err != nil {
	return "", err
}
```

Call normalization during app startup immediately after bootstrap:

```go
if err := security.EnsureBootstrapAdmin(context.Background(), db, cfg.Auth.Password); err != nil {
	return nil, fmt.Errorf("初始化引导管理员失败: %w", err)
}
if err := security.NormalizePersistedWebRBACPermissions(db); err != nil {
	return nil, fmt.Errorf("规范化 Web RBAC 权限失败: %w", err)
}
```

- [ ] **Step 5: Re-run the focused tests and verify GREEN**

Run:

```bash
go test ./internal/database ./internal/security -run 'TestListWebAccessRolesNormalizesLegacyPermissions|TestEnsureBootstrapAdminSeedsCanonicalSuperAdmin|TestAuthManager' -v
```

Expected: `PASS`

- [ ] **Step 6: Commit**

```bash
git add internal/database/web_auth.go \
        internal/database/web_auth_admin.go \
        internal/database/web_auth_test.go \
        internal/security/web_rbac_migration.go \
        internal/security/bootstrap.go \
        internal/security/bootstrap_test.go \
        internal/security/auth_manager_test.go \
        internal/app/app.go
git commit -m "feat: normalize persisted web rbac permissions"
```

### Task 4: Add A Testable Route-Permission Registry And Rebind System/Auth APIs

**Files:**
- Create: `internal/security/route_permissions.go`
- Create: `internal/security/route_permissions_test.go`
- Modify: `internal/security/auth_middleware.go`
- Modify: `internal/security/auth_middleware_test.go`
- Modify: `internal/app/app.go`
- Modify: `internal/handler/auth.go`
- Modify: `internal/handler/auth_test.go`
- Modify: `internal/handler/web_access_roles.go`
- Modify: `internal/handler/web_users.go`
- Modify: `internal/handler/web_users_test.go`
- Modify: `internal/handler/openapi.go`
- Modify: `internal/handler/openapi_test.go`

- [ ] **Step 1: Write failing route-registry and API behavior tests**

Create `internal/security/route_permissions_test.go`:

```go
package security

import (
	"net/http"
	"testing"
)

func TestLookupRoutePermissionForSystemRoutes(t *testing.T) {
	cases := []struct {
		method string
		path   string
		want   string
	}{
		{http.MethodGet, "/config", PermissionSystemConfigSettingsRead},
		{http.MethodPost, "/config/apply", PermissionSystemRuntimeConfigApply},
		{http.MethodGet, "/security/web-users", PermissionSystemWebUserRead},
		{http.MethodPost, "/security/web-users/:id/reset-password", PermissionSystemWebUserCredentialReset},
		{http.MethodGet, "/security/web-access-roles", PermissionSystemWebAccessRoleRead},
	}

	for _, tc := range cases {
		got, ok := LookupRoutePermission(tc.method, tc.path)
		if !ok {
			t.Fatalf("expected route %s %s to be registered", tc.method, tc.path)
		}
		if got != tc.want {
			t.Fatalf("LookupRoutePermission(%q, %q) = %q, want %q", tc.method, tc.path, got, tc.want)
		}
	}
}
```

Add an auth handler test:

```go
func TestValidateReturnsCanonicalPermissions(t *testing.T) {
	router, _, token := newAuthTestRouter(t, []string{security.PermissionSystemWebUserRead})

	w := doAuthJSONRequest(t, router, http.MethodGet, "/api/auth/validate", nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var payload struct {
		Permissions []string `json:"permissions"`
	}
	decodeJSONBody(t, w, &payload)

	if !contains(payload.Permissions, security.PermissionSystemWebUserRead) {
		t.Fatalf("expected canonical permissions in validate payload, got %#v", payload.Permissions)
	}
}
```

Add a role handler test:

```go
func TestCreateWebAccessRoleRejectsLegacyPermission(t *testing.T) {
	router, _, adminToken := newWebUsersTestRouter(t)

	w := doJSONRequest(t, router, http.MethodPost, "/api/security/web-access-roles", map[string]any{
		"name":        "legacy-role",
		"description": "should fail",
		"permissions": []string{"security.users.manage"},
	}, adminToken)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
```

- [ ] **Step 2: Run the focused tests and verify RED**

Run:

```bash
go test ./internal/security ./internal/handler -run 'TestLookupRoutePermissionForSystemRoutes|TestValidateReturnsCanonicalPermissions|TestCreateWebAccessRoleRejectsLegacyPermission' -v
```

Expected: `FAIL` because there is no route registry, validate does not return permissions, and role handlers still accept retired permission identifiers.

- [ ] **Step 3: Add the route registry and route-aware authorization helper**

Create `internal/security/route_permissions.go`:

```go
package security

import (
	"net/http"
	"strings"
)

var routePermissionRegistry = map[string]string{
	http.MethodGet + " /config":                              PermissionSystemConfigSettingsRead,
	http.MethodGet + " /config/tools":                        PermissionSystemConfigSettingsRead,
	http.MethodPut + " /config":                              PermissionSystemConfigSettingsUpdate,
	http.MethodPost + " /config/apply":                       PermissionSystemRuntimeConfigApply,
	http.MethodPost + " /config/test-openai":                 PermissionSystemModelConnectivityTest,
	http.MethodGet + " /security/web-users":                  PermissionSystemWebUserRead,
	http.MethodPost + " /security/web-users":                 PermissionSystemWebUserCreate,
	http.MethodPut + " /security/web-users/:id":              PermissionSystemWebUserUpdate,
	http.MethodDelete + " /security/web-users/:id":           PermissionSystemWebUserDelete,
	http.MethodPost + " /security/web-users/:id/reset-password": PermissionSystemWebUserCredentialReset,
	http.MethodGet + " /security/web-access-roles":           PermissionSystemWebAccessRoleRead,
	http.MethodPost + " /security/web-access-roles":          PermissionSystemWebAccessRoleCreate,
	http.MethodPut + " /security/web-access-roles/:id":       PermissionSystemWebAccessRoleUpdate,
	http.MethodDelete + " /security/web-access-roles/:id":    PermissionSystemWebAccessRoleDelete,
	http.MethodGet + " /security/web-access-roles/permission-catalog": PermissionSystemWebAccessRoleRead,
}

func LookupRoutePermission(method, path string) (string, bool) {
	permission, ok := routePermissionRegistry[strings.ToUpper(strings.TrimSpace(method))+" "+strings.TrimSpace(path)]
	return permission, ok
}
```

Update `internal/security/auth_middleware.go`:

```go
func RequireRoutePermission(method, path string) gin.HandlerFunc {
	permission, ok := LookupRoutePermission(method, path)
	if !ok {
		panic("missing route permission: " + method + " " + path)
	}
	return RequirePermission(permission)
}
```

- [ ] **Step 4: Rebind system/security routes and update auth/role handlers**

Update `internal/app/app.go`:

```go
protected.GET("/config", security.RequireRoutePermission(http.MethodGet, "/config"), configHandler.GetConfig)
protected.GET("/config/tools", security.RequireRoutePermission(http.MethodGet, "/config/tools"), configHandler.GetTools)
protected.PUT("/config", security.RequireRoutePermission(http.MethodPut, "/config"), configHandler.UpdateConfig)
protected.POST("/config/apply", security.RequireRoutePermission(http.MethodPost, "/config/apply"), configHandler.ApplyConfig)
protected.POST("/config/test-openai", security.RequireRoutePermission(http.MethodPost, "/config/test-openai"), configHandler.TestOpenAI)

protected.GET("/security/web-users", security.RequireRoutePermission(http.MethodGet, "/security/web-users"), webUsersHandler.ListWebUsers)
protected.POST("/security/web-users", security.RequireRoutePermission(http.MethodPost, "/security/web-users"), webUsersHandler.CreateWebUser)
protected.PUT("/security/web-users/:id", security.RequireRoutePermission(http.MethodPut, "/security/web-users/:id"), webUsersHandler.UpdateWebUser)
protected.POST("/security/web-users/:id/reset-password", security.RequireRoutePermission(http.MethodPost, "/security/web-users/:id/reset-password"), webUsersHandler.ResetWebUserPassword)
protected.DELETE("/security/web-users/:id", security.RequireRoutePermission(http.MethodDelete, "/security/web-users/:id"), webUsersHandler.DeleteWebUser)
protected.GET("/security/web-access-roles", security.RequireRoutePermission(http.MethodGet, "/security/web-access-roles"), webAccessRolesHandler.ListWebAccessRoles)
protected.GET("/security/web-access-roles/permission-catalog", security.RequireRoutePermission(http.MethodGet, "/security/web-access-roles/permission-catalog"), webAccessRolesHandler.GetPermissionCatalog)
protected.POST("/security/web-access-roles", security.RequireRoutePermission(http.MethodPost, "/security/web-access-roles"), webAccessRolesHandler.CreateWebAccessRole)
protected.PUT("/security/web-access-roles/:id", security.RequireRoutePermission(http.MethodPut, "/security/web-access-roles/:id"), webAccessRolesHandler.UpdateWebAccessRole)
protected.DELETE("/security/web-access-roles/:id", security.RequireRoutePermission(http.MethodDelete, "/security/web-access-roles/:id"), webAccessRolesHandler.DeleteWebAccessRole)
```

Update `internal/handler/auth.go`:

```go
c.JSON(http.StatusOK, gin.H{
	"token":                session.Token,
	"username":             session.Username,
	"expires_at":           session.ExpiresAt.UTC().Format(time.RFC3339),
	"must_change_password": session.MustChangePassword,
	"permissions":          sortedPermissionList(session.Permissions),
})
```

Update `internal/handler/web_access_roles.go`:

```go
normalized := security.NormalizeWebPermissions(req.Permissions)
if len(normalized) != len(req.Permissions) {
	c.JSON(http.StatusBadRequest, gin.H{"error": "仅允许使用新的资源权限标识"})
	return
}
for _, permission := range normalized {
	if !security.IsCanonicalWebPermission(permission) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "存在未知权限标识: " + permission})
		return
	}
}
req.Permissions = normalized
```

Add the catalog endpoint:

```go
func (h *WebAccessRolesHandler) GetPermissionCatalog(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"domains":            security.PermissionCatalogDomains(),
		"specialPermissions": []string{security.PermissionSuperAdminGrant},
	})
}
```

Update last-super-admin checks in `internal/handler/web_users.go`:

```go
nextHasSuperAdmin, err := h.db.RoleIDsGrantPermission(req.RoleIDs, security.PermissionSuperAdminGrant)
superAdminCount, err := h.db.CountEnabledUsersWithPermission(security.PermissionSuperAdminGrant)
```

- [ ] **Step 5: Update OpenAPI for the new payloads and rerun tests**

Add to `internal/handler/openapi.go`:

```go
"permissions": map[string]interface{}{
	"type": "array",
	"items": map[string]interface{}{"type": "string"},
	"description": "当前会话拥有的 canonical Web RBAC 权限",
},
```

Register the new endpoint:

```go
"/api/security/web-access-roles/permission-catalog": map[string]interface{}{
	"get": map[string]interface{}{
		"tags":        []string{"安全设置"},
		"summary":     "获取 Web 权限目录",
		"description": "返回按业务域和资源分组的 canonical Web RBAC 权限目录。",
		"operationId": "getWebPermissionCatalog",
		"responses": map[string]interface{}{
			"200": map[string]interface{}{"description": "获取成功"},
			"401": map[string]interface{}{"description": "未授权"},
			"403": map[string]interface{}{"description": "权限不足"},
		},
	},
},
```

Run:

```bash
go test ./internal/security ./internal/handler -run 'TestLookupRoutePermissionForSystemRoutes|TestRequirePermission|TestValidateReturnsCanonicalPermissions|TestCreateWebAccessRoleRejectsLegacyPermission|TestOpenAPIIncludesSecurityRoutes' -v
```

Expected: `PASS`

- [ ] **Step 6: Commit**

```bash
git add internal/security/route_permissions.go \
        internal/security/route_permissions_test.go \
        internal/security/auth_middleware.go \
        internal/security/auth_middleware_test.go \
        internal/app/app.go \
        internal/handler/auth.go \
        internal/handler/auth_test.go \
        internal/handler/web_access_roles.go \
        internal/handler/web_users.go \
        internal/handler/web_users_test.go \
        internal/handler/openapi.go \
        internal/handler/openapi_test.go
git commit -m "feat: bind system routes to canonical web permissions"
```

### Task 5: Bind Agent, Intel, And Task Routes To Canonical Permissions

**Files:**
- Modify: `internal/security/permissions.go`
- Modify: `internal/security/permission_catalog.go`
- Modify: `internal/security/route_permissions.go`
- Modify: `internal/security/route_permissions_test.go`
- Modify: `internal/app/app.go`

- [ ] **Step 1: Add failing route-registry coverage tests for agent/intel/task domains**

Extend `internal/security/route_permissions_test.go`:

```go
func TestLookupRoutePermissionForTaskAndAgentRoutes(t *testing.T) {
	cases := []struct {
		method string
		path   string
		want   string
	}{
		{http.MethodPost, "/robot/test", PermissionAgentRobotTestExecute},
		{http.MethodPost, "/agent-loop", PermissionAgentRunExecute},
		{http.MethodPost, "/agent-loop/cancel", PermissionAgentRunStop},
		{http.MethodGet, "/agent-loop/tasks", PermissionAgentRunRead},
		{http.MethodPost, "/multi-agent", PermissionAgentMultiRunExecute},
		{http.MethodGet, "/multi-agent/markdown-agents", PermissionAgentMarkdownAgentRead},
		{http.MethodPost, "/fofa/search", PermissionIntelFofaQueryExecute},
		{http.MethodGet, "/batch-tasks", PermissionTaskBatchQueueRead},
		{http.MethodPost, "/batch-tasks", PermissionTaskBatchQueueCreate},
		{http.MethodPost, "/batch-tasks/:queueId/start", PermissionTaskBatchQueueStart},
		{http.MethodPost, "/batch-tasks/:queueId/pause", PermissionTaskBatchQueueStop},
		{http.MethodGet, "/conversations", PermissionTaskConversationRead},
		{http.MethodPost, "/conversations", PermissionTaskConversationCreate},
		{http.MethodDelete, "/conversations/:id", PermissionTaskConversationDelete},
		{http.MethodGet, "/groups", PermissionTaskGroupRead},
		{http.MethodPost, "/groups", PermissionTaskGroupCreate},
		{http.MethodGet, "/monitor", PermissionTaskExecutionRead},
		{http.MethodDelete, "/monitor/execution/:id", PermissionTaskExecutionDelete},
		{http.MethodGet, "/attack-chain/:conversationId", PermissionTaskAttackChainRead},
		{http.MethodPost, "/attack-chain/:conversationId/regenerate", PermissionTaskAttackChainRegenerate},
		{http.MethodGet, "/conversations/:id/results", PermissionTaskConversationResultRead},
	}

	for _, tc := range cases {
		got, ok := LookupRoutePermission(tc.method, tc.path)
		if !ok || got != tc.want {
			t.Fatalf("route %s %s mapped to %q (ok=%v), want %q", tc.method, tc.path, got, ok, tc.want)
		}
	}
}
```

- [ ] **Step 2: Run the route-registry tests and verify RED**

Run:

```bash
go test ./internal/security -run 'TestLookupRoutePermissionForTaskAndAgentRoutes' -v
```

Expected: `FAIL` because the new route entries and constants are not added yet.

- [ ] **Step 3: Add canonical constants and registry entries for the first wave of business domains**

Extend `internal/security/permissions.go` and `internal/security/permission_catalog.go` with:

```go
const (
	PermissionAgentRobotTestExecute   = "agent.robot_test.execute"
	PermissionAgentRunRead            = "agent.run.read"
	PermissionAgentRunExecute         = "agent.run.execute"
	PermissionAgentRunStop            = "agent.run.stop"
	PermissionAgentMultiRunRead       = "agent.multi_run.read"
	PermissionAgentMultiRunExecute    = "agent.multi_run.execute"
	PermissionAgentMultiRunStop       = "agent.multi_run.stop"
	PermissionAgentMarkdownAgentRead  = "agent.markdown_agent.read"
	PermissionAgentMarkdownAgentCreate = "agent.markdown_agent.create"
	PermissionAgentMarkdownAgentUpdate = "agent.markdown_agent.update"
	PermissionAgentMarkdownAgentDelete = "agent.markdown_agent.delete"

	PermissionTaskConversationRead          = "task.conversation.read"
	PermissionTaskConversationCreate        = "task.conversation.create"
	PermissionTaskConversationUpdate        = "task.conversation.update"
	PermissionTaskConversationDelete        = "task.conversation.delete"
	PermissionTaskGroupRead                 = "task.group.read"
	PermissionTaskGroupCreate               = "task.group.create"
	PermissionTaskGroupUpdate               = "task.group.update"
	PermissionTaskGroupDelete               = "task.group.delete"
	PermissionTaskExecutionRead             = "task.execution.read"
	PermissionTaskExecutionDelete           = "task.execution.delete"
	PermissionTaskAttackChainRead           = "task.attack_chain.read"
	PermissionTaskAttackChainRegenerate     = "task.attack_chain.regenerate"
	PermissionTaskConversationResultRead    = "task.conversation_result.read"
)
```

Populate `internal/security/route_permissions.go`:

```go
http.MethodPost + " /robot/test":                            PermissionAgentRobotTestExecute,
http.MethodPost + " /agent-loop":                            PermissionAgentRunExecute,
http.MethodPost + " /agent-loop/stream":                     PermissionAgentRunExecute,
http.MethodPost + " /agent-loop/cancel":                     PermissionAgentRunStop,
http.MethodGet + " /agent-loop/tasks":                       PermissionAgentRunRead,
http.MethodGet + " /agent-loop/tasks/completed":             PermissionAgentRunRead,
http.MethodPost + " /multi-agent":                           PermissionAgentMultiRunExecute,
http.MethodPost + " /multi-agent/stream":                    PermissionAgentMultiRunExecute,
http.MethodGet + " /multi-agent/markdown-agents":            PermissionAgentMarkdownAgentRead,
http.MethodGet + " /multi-agent/markdown-agents/:filename":  PermissionAgentMarkdownAgentRead,
http.MethodPost + " /multi-agent/markdown-agents":           PermissionAgentMarkdownAgentCreate,
http.MethodPut + " /multi-agent/markdown-agents/:filename":  PermissionAgentMarkdownAgentUpdate,
http.MethodDelete + " /multi-agent/markdown-agents/:filename": PermissionAgentMarkdownAgentDelete,
http.MethodPost + " /fofa/search":                           PermissionIntelFofaQueryExecute,
http.MethodPost + " /fofa/parse":                            PermissionIntelFofaQueryExecute,
http.MethodPost + " /batch-tasks":                           PermissionTaskBatchQueueCreate,
http.MethodGet + " /batch-tasks":                            PermissionTaskBatchQueueRead,
http.MethodGet + " /batch-tasks/:queueId":                   PermissionTaskBatchQueueRead,
http.MethodPost + " /batch-tasks/:queueId/start":            PermissionTaskBatchQueueStart,
http.MethodPost + " /batch-tasks/:queueId/pause":            PermissionTaskBatchQueueStop,
http.MethodPut + " /batch-tasks/:queueId/schedule-enabled":  PermissionTaskBatchQueueUpdate,
http.MethodDelete + " /batch-tasks/:queueId":                PermissionTaskBatchQueueDelete,
http.MethodPost + " /batch-tasks/:queueId/tasks":            "task.batch_task.create",
http.MethodPut + " /batch-tasks/:queueId/tasks/:taskId":     "task.batch_task.update",
http.MethodDelete + " /batch-tasks/:queueId/tasks/:taskId":  "task.batch_task.delete",
http.MethodPost + " /conversations":                         PermissionTaskConversationCreate,
http.MethodGet + " /conversations":                          PermissionTaskConversationRead,
http.MethodGet + " /conversations/:id":                      PermissionTaskConversationRead,
http.MethodPut + " /conversations/:id":                      PermissionTaskConversationUpdate,
http.MethodDelete + " /conversations/:id":                   PermissionTaskConversationDelete,
http.MethodPost + " /conversations/:id/delete-turn":         PermissionTaskConversationUpdate,
http.MethodGet + " /groups":                                 PermissionTaskGroupRead,
http.MethodGet + " /groups/:id":                             PermissionTaskGroupRead,
http.MethodPost + " /groups":                                PermissionTaskGroupCreate,
http.MethodPut + " /groups/:id":                             PermissionTaskGroupUpdate,
http.MethodDelete + " /groups/:id":                          PermissionTaskGroupDelete,
http.MethodGet + " /monitor":                                PermissionTaskExecutionRead,
http.MethodGet + " /monitor/execution/:id":                  PermissionTaskExecutionRead,
http.MethodGet + " /monitor/stats":                          PermissionTaskExecutionRead,
http.MethodDelete + " /monitor/execution/:id":               PermissionTaskExecutionDelete,
http.MethodDelete + " /monitor/executions":                  PermissionTaskExecutionDelete,
http.MethodGet + " /attack-chain/:conversationId":           PermissionTaskAttackChainRead,
http.MethodPost + " /attack-chain/:conversationId/regenerate": PermissionTaskAttackChainRegenerate,
http.MethodGet + " /conversations/:id/results":              PermissionTaskConversationResultRead,
```

- [ ] **Step 4: Rebind the first wave of routes in `internal/app/app.go`**

Replace the inline registrations with route-registry lookups:

```go
protected.POST("/robot/test", security.RequireRoutePermission(http.MethodPost, "/robot/test"), robotHandler.HandleRobotTest)
protected.POST("/agent-loop", security.RequireRoutePermission(http.MethodPost, "/agent-loop"), agentHandler.AgentLoop)
protected.POST("/agent-loop/stream", security.RequireRoutePermission(http.MethodPost, "/agent-loop/stream"), agentHandler.AgentLoopStream)
protected.POST("/agent-loop/cancel", security.RequireRoutePermission(http.MethodPost, "/agent-loop/cancel"), agentHandler.CancelAgentLoop)
protected.GET("/agent-loop/tasks", security.RequireRoutePermission(http.MethodGet, "/agent-loop/tasks"), agentHandler.ListAgentTasks)
protected.GET("/agent-loop/tasks/completed", security.RequireRoutePermission(http.MethodGet, "/agent-loop/tasks/completed"), agentHandler.ListCompletedTasks)
protected.POST("/multi-agent", security.RequireRoutePermission(http.MethodPost, "/multi-agent"), agentHandler.MultiAgentLoop)
protected.POST("/multi-agent/stream", security.RequireRoutePermission(http.MethodPost, "/multi-agent/stream"), agentHandler.MultiAgentLoopStream)
protected.GET("/multi-agent/markdown-agents", security.RequireRoutePermission(http.MethodGet, "/multi-agent/markdown-agents"), markdownAgentsHandler.ListMarkdownAgents)
protected.POST("/fofa/search", security.RequireRoutePermission(http.MethodPost, "/fofa/search"), fofaHandler.Search)
protected.POST("/batch-tasks", security.RequireRoutePermission(http.MethodPost, "/batch-tasks"), agentHandler.CreateBatchQueue)
protected.GET("/conversations", security.RequireRoutePermission(http.MethodGet, "/conversations"), conversationHandler.ListConversations)
protected.GET("/groups", security.RequireRoutePermission(http.MethodGet, "/groups"), groupHandler.ListGroups)
protected.GET("/monitor", security.RequireRoutePermission(http.MethodGet, "/monitor"), monitorHandler.Monitor)
protected.GET("/attack-chain/:conversationId", security.RequireRoutePermission(http.MethodGet, "/attack-chain/:conversationId"), attackChainHandler.GetAttackChain)
```

- [ ] **Step 5: Re-run the route tests and verify GREEN**

Run:

```bash
go test ./internal/security -run 'TestLookupRoutePermissionForTaskAndAgentRoutes' -v
```

Expected: `PASS`

- [ ] **Step 6: Commit**

```bash
git add internal/security/permissions.go \
        internal/security/permission_catalog.go \
        internal/security/route_permissions.go \
        internal/security/route_permissions_test.go \
        internal/app/app.go
git commit -m "feat: protect agent and task routes with canonical permissions"
```

### Task 6: Bind Knowledge, WebShell, File, MCP, Skill, Role, And Vulnerability Routes

**Files:**
- Modify: `internal/security/permissions.go`
- Modify: `internal/security/permission_catalog.go`
- Modify: `internal/security/route_permissions.go`
- Modify: `internal/security/route_permissions_test.go`
- Modify: `internal/app/app.go`

- [ ] **Step 1: Add failing route-registry coverage tests for the remaining business domains**

Extend `internal/security/route_permissions_test.go`:

```go
func TestLookupRoutePermissionForKnowledgeAndOpsRoutes(t *testing.T) {
	cases := []struct {
		method string
		path   string
		want   string
	}{
		{http.MethodGet, "/knowledge/items", "knowledge.item.read"},
		{http.MethodPost, "/knowledge/items", "knowledge.item.create"},
		{http.MethodDelete, "/knowledge/items/:id", "knowledge.item.delete"},
		{http.MethodPost, "/knowledge/index", "knowledge.index.execute"},
		{http.MethodPost, "/knowledge/search", "knowledge.search.execute"},
		{http.MethodGet, "/vulnerabilities", "vulnerability.record.read"},
		{http.MethodPost, "/vulnerabilities", "vulnerability.record.create"},
		{http.MethodDelete, "/vulnerabilities/:id", "vulnerability.record.delete"},
		{http.MethodGet, "/webshell/connections", "webshell.connection.read"},
		{http.MethodPost, "/webshell/connections", "webshell.connection.create"},
		{http.MethodPost, "/webshell/exec", "webshell.command.execute"},
		{http.MethodPost, "/webshell/file", "webshell.file.execute"},
		{http.MethodGet, "/chat-uploads", "file.workspace_entry.read"},
		{http.MethodPut, "/chat-uploads/content", "file.workspace_content.update"},
		{http.MethodGet, "/external-mcp", "mcp.external_server.read"},
		{http.MethodPost, "/external-mcp/:name/start", "mcp.external_server.start"},
		{http.MethodPost, "/mcp", "mcp.gateway.execute"},
		{http.MethodGet, "/roles", "role.agent_role.read"},
		{http.MethodPost, "/roles", "role.agent_role.create"},
		{http.MethodGet, "/skills", "skill.definition.read"},
		{http.MethodDelete, "/skills/:name", "skill.definition.delete"},
		{http.MethodGet, "/openapi/spec", "system.api_spec.read"},
		{http.MethodPost, "/terminal/run", "system.terminal.execute"},
	}

	for _, tc := range cases {
		got, ok := LookupRoutePermission(tc.method, tc.path)
		if !ok || got != tc.want {
			t.Fatalf("route %s %s mapped to %q (ok=%v), want %q", tc.method, tc.path, got, ok, tc.want)
		}
	}
}
```

- [ ] **Step 2: Run the route-registry tests and verify RED**

Run:

```bash
go test ./internal/security -run 'TestLookupRoutePermissionForKnowledgeAndOpsRoutes' -v
```

Expected: `FAIL`

- [ ] **Step 3: Add the remaining constants, catalog entries, and route registry rows**

Extend the canonical constants:

```go
const (
	PermissionVulnerabilityRecordRead   = "vulnerability.record.read"
	PermissionVulnerabilityRecordCreate = "vulnerability.record.create"
	PermissionVulnerabilityRecordUpdate = "vulnerability.record.update"
	PermissionVulnerabilityRecordDelete = "vulnerability.record.delete"
	PermissionVulnerabilityStatsRead    = "vulnerability.stats.read"

	PermissionWebshellConnectionRead   = "webshell.connection.read"
	PermissionWebshellConnectionCreate = "webshell.connection.create"
	PermissionWebshellConnectionUpdate = "webshell.connection.update"
	PermissionWebshellConnectionDelete = "webshell.connection.delete"
	PermissionWebshellConnectionTest   = "webshell.connection.test"
	PermissionWebshellSessionRead      = "webshell.session.read"
	PermissionWebshellSessionUpdate    = "webshell.session.update"
	PermissionWebshellCommandExecute   = "webshell.command.execute"
	PermissionWebshellFileExecute      = "webshell.file.execute"

	PermissionFileWorkspaceEntryRead   = "file.workspace_entry.read"
	PermissionFileWorkspaceEntryCreate = "file.workspace_entry.create"
	PermissionFileWorkspaceEntryUpdate = "file.workspace_entry.update"
	PermissionFileWorkspaceEntryDelete = "file.workspace_entry.delete"
	PermissionFileWorkspaceContentRead = "file.workspace_content.read"
	PermissionFileWorkspaceContentUpdate = "file.workspace_content.update"

	PermissionMCPGatewayExecute       = "mcp.gateway.execute"
	PermissionMCPExternalServerRead   = "mcp.external_server.read"
	PermissionMCPExternalServerCreate = "mcp.external_server.create"
	PermissionMCPExternalServerUpdate = "mcp.external_server.update"
	PermissionMCPExternalServerDelete = "mcp.external_server.delete"
	PermissionMCPExternalServerStart  = "mcp.external_server.start"
	PermissionMCPExternalServerStop   = "mcp.external_server.stop"

	PermissionKnowledgeCategoryRead      = "knowledge.category.read"
	PermissionKnowledgeItemRead          = "knowledge.item.read"
	PermissionKnowledgeItemCreate        = "knowledge.item.create"
	PermissionKnowledgeItemUpdate        = "knowledge.item.update"
	PermissionKnowledgeItemDelete        = "knowledge.item.delete"
	PermissionKnowledgeIndexRead         = "knowledge.index.read"
	PermissionKnowledgeIndexExecute      = "knowledge.index.execute"
	PermissionKnowledgeRetrievalLogRead  = "knowledge.retrieval_log.read"
	PermissionKnowledgeRetrievalLogDelete = "knowledge.retrieval_log.delete"
	PermissionKnowledgeSearchExecute     = "knowledge.search.execute"
	PermissionKnowledgeStatsRead         = "knowledge.stats.read"

	PermissionSkillDefinitionRead   = "skill.definition.read"
	PermissionSkillDefinitionCreate = "skill.definition.create"
	PermissionSkillDefinitionUpdate = "skill.definition.update"
	PermissionSkillDefinitionDelete = "skill.definition.delete"
	PermissionSkillBindingRead      = "skill.binding.read"
	PermissionSkillStatsRead        = "skill.stats.read"
	PermissionSkillStatsDelete      = "skill.stats.delete"

	PermissionRoleAgentRoleRead   = "role.agent_role.read"
	PermissionRoleAgentRoleCreate = "role.agent_role.create"
	PermissionRoleAgentRoleUpdate = "role.agent_role.update"
	PermissionRoleAgentRoleDelete = "role.agent_role.delete"

	PermissionSystemTerminalExecute = "system.terminal.execute"
	PermissionSystemAPISpecRead     = "system.api_spec.read"
)
```

Populate the route registry with the remaining entries, including:

```go
http.MethodGet + " /knowledge/categories":              PermissionKnowledgeCategoryRead,
http.MethodGet + " /knowledge/items":                   PermissionKnowledgeItemRead,
http.MethodGet + " /knowledge/items/:id":               PermissionKnowledgeItemRead,
http.MethodPost + " /knowledge/items":                  PermissionKnowledgeItemCreate,
http.MethodPut + " /knowledge/items/:id":               PermissionKnowledgeItemUpdate,
http.MethodDelete + " /knowledge/items/:id":            PermissionKnowledgeItemDelete,
http.MethodGet + " /knowledge/index-status":            PermissionKnowledgeIndexRead,
http.MethodPost + " /knowledge/index":                  PermissionKnowledgeIndexExecute,
http.MethodPost + " /knowledge/scan":                   PermissionKnowledgeIndexExecute,
http.MethodGet + " /knowledge/retrieval-logs":          PermissionKnowledgeRetrievalLogRead,
http.MethodDelete + " /knowledge/retrieval-logs/:id":   PermissionKnowledgeRetrievalLogDelete,
http.MethodPost + " /knowledge/search":                 PermissionKnowledgeSearchExecute,
http.MethodGet + " /knowledge/stats":                   PermissionKnowledgeStatsRead,
http.MethodGet + " /vulnerabilities":                   PermissionVulnerabilityRecordRead,
http.MethodGet + " /vulnerabilities/stats":             PermissionVulnerabilityStatsRead,
http.MethodPost + " /vulnerabilities":                  PermissionVulnerabilityRecordCreate,
http.MethodPut + " /vulnerabilities/:id":               PermissionVulnerabilityRecordUpdate,
http.MethodDelete + " /vulnerabilities/:id":            PermissionVulnerabilityRecordDelete,
http.MethodGet + " /webshell/connections":              PermissionWebshellConnectionRead,
http.MethodPost + " /webshell/connections":             PermissionWebshellConnectionCreate,
http.MethodPut + " /webshell/connections/:id":          PermissionWebshellConnectionUpdate,
http.MethodDelete + " /webshell/connections/:id":       PermissionWebshellConnectionDelete,
http.MethodGet + " /webshell/connections/:id/state":    PermissionWebshellSessionRead,
http.MethodPut + " /webshell/connections/:id/state":    PermissionWebshellSessionUpdate,
http.MethodPost + " /webshell/exec":                    PermissionWebshellCommandExecute,
http.MethodPost + " /webshell/file":                    PermissionWebshellFileExecute,
http.MethodGet + " /chat-uploads":                      PermissionFileWorkspaceEntryRead,
http.MethodPost + " /chat-uploads":                     PermissionFileWorkspaceEntryCreate,
http.MethodDelete + " /chat-uploads":                   PermissionFileWorkspaceEntryDelete,
http.MethodPut + " /chat-uploads/rename":               PermissionFileWorkspaceEntryUpdate,
http.MethodGet + " /chat-uploads/content":              PermissionFileWorkspaceContentRead,
http.MethodPut + " /chat-uploads/content":              PermissionFileWorkspaceContentUpdate,
http.MethodGet + " /external-mcp":                      PermissionMCPExternalServerRead,
http.MethodPut + " /external-mcp/:name":                PermissionMCPExternalServerUpdate,
http.MethodDelete + " /external-mcp/:name":             PermissionMCPExternalServerDelete,
http.MethodPost + " /external-mcp/:name/start":         PermissionMCPExternalServerStart,
http.MethodPost + " /external-mcp/:name/stop":          PermissionMCPExternalServerStop,
http.MethodPost + " /mcp":                              PermissionMCPGatewayExecute,
http.MethodGet + " /roles":                             PermissionRoleAgentRoleRead,
http.MethodGet + " /roles/:name":                       PermissionRoleAgentRoleRead,
http.MethodPost + " /roles":                            PermissionRoleAgentRoleCreate,
http.MethodPut + " /roles/:name":                       PermissionRoleAgentRoleUpdate,
http.MethodDelete + " /roles/:name":                    PermissionRoleAgentRoleDelete,
http.MethodGet + " /skills":                            PermissionSkillDefinitionRead,
http.MethodGet + " /skills/stats":                      PermissionSkillStatsRead,
http.MethodDelete + " /skills/stats":                   PermissionSkillStatsDelete,
http.MethodGet + " /skills/:name":                      PermissionSkillDefinitionRead,
http.MethodGet + " /skills/:name/bound-roles":          PermissionSkillBindingRead,
http.MethodPost + " /skills":                           PermissionSkillDefinitionCreate,
http.MethodPut + " /skills/:name":                      PermissionSkillDefinitionUpdate,
http.MethodDelete + " /skills/:name":                   PermissionSkillDefinitionDelete,
http.MethodDelete + " /skills/:name/stats":             PermissionSkillStatsDelete,
http.MethodPost + " /terminal/run":                     PermissionSystemTerminalExecute,
http.MethodPost + " /terminal/run/stream":              PermissionSystemTerminalExecute,
http.MethodGet + " /terminal/ws":                       PermissionSystemTerminalExecute,
http.MethodGet + " /openapi/spec":                      PermissionSystemAPISpecRead,
```

- [ ] **Step 4: Rebind the second wave of routes in `internal/app/app.go`**

Replace the remaining unprotected business routes with route-registry bindings:

```go
knowledgeRoutes.GET("/items", security.RequireRoutePermission(http.MethodGet, "/knowledge/items"), func(c *gin.Context) { ... })
knowledgeRoutes.POST("/items", security.RequireRoutePermission(http.MethodPost, "/knowledge/items"), func(c *gin.Context) { ... })
protected.GET("/vulnerabilities", security.RequireRoutePermission(http.MethodGet, "/vulnerabilities"), vulnerabilityHandler.ListVulnerabilities)
protected.POST("/vulnerabilities", security.RequireRoutePermission(http.MethodPost, "/vulnerabilities"), vulnerabilityHandler.CreateVulnerability)
protected.GET("/webshell/connections", security.RequireRoutePermission(http.MethodGet, "/webshell/connections"), webshellHandler.ListConnections)
protected.POST("/webshell/exec", security.RequireRoutePermission(http.MethodPost, "/webshell/exec"), webshellHandler.Exec)
protected.GET("/chat-uploads", security.RequireRoutePermission(http.MethodGet, "/chat-uploads"), chatUploadsHandler.List)
protected.PUT("/chat-uploads/content", security.RequireRoutePermission(http.MethodPut, "/chat-uploads/content"), chatUploadsHandler.PutContent)
protected.GET("/external-mcp", security.RequireRoutePermission(http.MethodGet, "/external-mcp"), externalMCPHandler.GetExternalMCPs)
protected.POST("/mcp", security.RequireRoutePermission(http.MethodPost, "/mcp"), func(c *gin.Context) { mcpServer.HandleHTTP(c.Writer, c.Request) })
protected.GET("/roles", security.RequireRoutePermission(http.MethodGet, "/roles"), roleHandler.GetRoles)
protected.GET("/skills", security.RequireRoutePermission(http.MethodGet, "/skills"), skillsHandler.GetSkills)
protected.GET("/openapi/spec", security.RequireRoutePermission(http.MethodGet, "/openapi/spec"), openAPIHandler.GetOpenAPISpec)
```

- [ ] **Step 5: Re-run the route-registry tests and verify GREEN**

Run:

```bash
go test ./internal/security -run 'TestLookupRoutePermissionForKnowledgeAndOpsRoutes' -v
```

Expected: `PASS`

- [ ] **Step 6: Commit**

```bash
git add internal/security/permissions.go \
        internal/security/permission_catalog.go \
        internal/security/route_permissions.go \
        internal/security/route_permissions_test.go \
        internal/app/app.go
git commit -m "feat: protect remaining business routes with canonical permissions"
```

### Task 7: Replace The Legacy Web Role UI With Grouped Catalog Rendering

**Files:**
- Modify: `web/static/js/web-users.js`
- Modify: `web/static/i18n/zh-CN.json`
- Modify: `web/static/i18n/en-US.json`

- [ ] **Step 1: Write the grouped rendering changes behind a new catalog loader**

Replace the hardcoded `getKnownWebPermissions()` with backend-loaded catalog state:

```js
let webPermissionCatalog = { domains: [], specialPermissions: [] };

async function loadWebPermissionCatalog() {
    const response = await apiFetch('/api/security/web-access-roles/permission-catalog');
    const result = await response.json().catch(() => ({}));
    if (!response.ok) {
        throw new Error(result.error || securityText('settingsSecurity.loadPermissionCatalogFailed', '获取权限目录失败'));
    }
    webPermissionCatalog = {
        domains: Array.isArray(result.domains) ? result.domains : [],
        specialPermissions: Array.isArray(result.specialPermissions) ? result.specialPermissions : [],
    };
}
```

- [ ] **Step 2: Replace flat checkbox rendering with domain/resource grouping**

Add grouped render helpers:

```js
function renderPermissionGroups(selectedPermissions = []) {
    const selected = new Set(selectedPermissions);
    const groups = (webPermissionCatalog.domains || []).map(domain => `
        <section class="security-permission-domain">
            <header class="security-permission-domain__header">
                <span>${escapeHtml(permissionDomainLabel(domain.id))}</span>
                <span>${countDomainSelections(domain, selected)}</span>
            </header>
            ${(domain.resources || []).map(resource => `
                <div class="security-permission-resource">
                    <div class="security-permission-resource__title">${escapeHtml(permissionResourceLabel(domain.id, resource.id))}</div>
                    <div class="security-permission-resource__actions">
                        ${(resource.permissions || []).map(permission => `
                            <label class="security-option-item">
                                <input type="checkbox"
                                       name="web-access-role-permission"
                                       value="${escapeHtml(permission)}"
                                       ${selected.has(permission) ? 'checked' : ''} />
                                <span>${escapeHtml(permissionActionLabel(permission))}</span>
                            </label>
                        `).join('')}
                    </div>
                </div>
            `).join('')}
        </section>
    `).join('');

    const special = (webPermissionCatalog.specialPermissions || []).map(permission => `
        <label class="security-option-item security-option-item--danger">
            <input type="checkbox"
                   name="web-access-role-permission"
                   value="${escapeHtml(permission)}"
                   ${selected.has(permission) ? 'checked' : ''} />
            <span>${escapeHtml(permissionActionLabel(permission))}</span>
        </label>
    `).join('');

    document.getElementById('web-access-role-permission-options').innerHTML = groups + special;
}
```

- [ ] **Step 3: Update the list cards to display grouped summaries instead of raw comma-joined strings**

Add a summary helper:

```js
function summarizePermissions(permissions = []) {
    if (!permissions.length) {
        return securityText('settingsSecurity.noPermissions', '无权限');
    }
    const grouped = groupPermissionsForSummary(permissions);
    return grouped.map(item => `${item.domain} / ${item.resource}: ${item.actions.join('、')}`).join('；');
}
```

Use it in both card renderers:

```js
const permissions = summarizePermissions(Array.isArray(role.permissions) ? role.permissions : []);
```

- [ ] **Step 4: Add i18n entries for domains, resources, actions, and new errors**

Add to `web/static/i18n/zh-CN.json`:

```json
"loadPermissionCatalogFailed": "获取权限目录失败",
"permissionAction.read": "查看",
"permissionAction.create": "新建",
"permissionAction.update": "修改",
"permissionAction.delete": "删除",
"permissionAction.execute": "执行",
"permissionAction.start": "启动",
"permissionAction.stop": "停止",
"permissionAction.test": "测试",
"permissionAction.reset": "重置",
"permissionAction.apply": "应用",
"permissionAction.grant": "授权",
"permissionAction.regenerate": "重生成",
"permissionDomain.intel": "信息收集",
"permissionDomain.task": "任务管理",
"permissionDomain.vulnerability": "漏洞管理",
"permissionDomain.webshell": "WebShell 管理",
"permissionDomain.file": "文件管理",
"permissionDomain.mcp": "MCP",
"permissionDomain.knowledge": "知识库",
"permissionDomain.skill": "Skills",
"permissionDomain.agent": "Agents",
"permissionDomain.role": "角色",
"permissionDomain.system": "系统设置"
```

Add the English equivalents to `web/static/i18n/en-US.json`.

- [ ] **Step 5: Run backend tests plus manual browser verification**

Run:

```bash
go test ./internal/handler -run 'TestCreateWebAccessRoleRejectsLegacyPermission|TestValidateReturnsCanonicalPermissions|TestOpenAPIIncludesSecurityRoutes' -v
```

Expected: `PASS`

Manual verification:

```text
1. 登录系统设置 -> 安全设置 -> Web 访问角色
2. 打开“新建 Web 访问角色”弹窗
3. 确认权限按业务域和资源分组展示，且不再出现 `system.config.write` 等旧权限
4. 新建一个只含 `system.web_user.read` 的角色，保存成功
5. 角色列表和用户列表里的权限摘要以“域 / 资源: 动作”形式展示
```

- [ ] **Step 6: Commit**

```bash
git add web/static/js/web-users.js \
        web/static/i18n/zh-CN.json \
        web/static/i18n/en-US.json
git commit -m "feat: group web role permissions by domain and resource"
```

### Task 8: Run The Full Verification Sweep

**Files:**
- Modify: `docs/superpowers/plans/2026-04-15-web-rbac-resource-permissions.md`

- [ ] **Step 1: Run the focused backend regression suite**

Run:

```bash
go test ./internal/security ./internal/database ./internal/handler -v
```

Expected: `PASS`

- [ ] **Step 2: Run the broader project test sweep**

Run:

```bash
go test ./... 
```

Expected: `PASS` or a known pre-existing unrelated failure that is documented before merge.

- [ ] **Step 3: Execute the upgrade and authorization smoke checklist**

Validate these cases manually:

```text
1. 使用带旧权限标识的历史角色数据启动系统，确认角色权限被规范化为 canonical identifiers
2. `super-admin` 登录后可以访问 system、agent、knowledge、mcp、webshell 等所有受保护域
3. 一个仅有 `system.web_user.read` 的角色可以查看 Web 用户，但不能创建、删除、重置密码
4. 一个未被授予 `knowledge.*` 的旧角色访问知识库接口时返回 `403`
5. 删除或更新 Web 访问角色后，受影响用户旧会话失效并重新走最新权限计算
6. `/api/auth/validate` 返回 canonical `permissions`
7. `/api/security/web-access-roles/permission-catalog` 返回分组目录
```

- [ ] **Step 4: Mark plan notes with verification results**

Append a short execution note to this plan after implementation:

```md
## Execution Notes
- Full backend suite: PASS
- `go test ./...`: PASS
- Manual upgrade smoke: PASS
- Residual risk: if any unrelated pre-existing failure remains, record the exact package path and failing command output here before completion
```

- [ ] **Step 5: Final commit**

```bash
git add docs/superpowers/plans/2026-04-15-web-rbac-resource-permissions.md
git commit -m "test: verify web rbac resource permissions rollout"
```

## Execution Notes
- Focused backend suite: PASS
  - `go test ./internal/security ./internal/database ./internal/handler -v`
- Full backend suite: PASS
  - `go test ./...`
- Manual upgrade smoke: PASS
  - Isolated temp config and SQLite under `/tmp/csai-task8-xmhbid`
  - `/api/auth/validate` returned canonical `permissions=["system.super_admin.grant"]`
  - `/api/security/web-access-roles/permission-catalog` returned grouped catalog with 11 domains
  - Super-admin successfully accessed protected `system` / `agent` / `knowledge` / `mcp` / `webshell` / `role` / `skill` / `openapi` routes
  - A role limited to `system.web_user.read` could read Web users but received `403` for create and unrelated knowledge APIs
  - Updating the role invalidated the affected user's existing session token
- Residual risk: plan note only; no new code changes were made in this step
