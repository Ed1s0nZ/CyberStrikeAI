# Local Workflow Package MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add secure, deterministic single-workflow ZIP export plus two-step, idempotent local package import without changing existing workflow APIs.

**Architecture:** `internal/workflow/package` owns package format, deterministic ZIP construction, archive inspection and import orchestration. `internal/database` owns SQLite schema, lifecycle state and the one transaction that rechecks the inspection snapshot, changes `workflow_definitions`, persists the import result and consumes the inspection. `internal/handler` maps the approved REST contract to these services and app routing/RBAC remains the enforcement boundary.

**Tech Stack:** Go, Gin, SQLite via `github.com/mattn/go-sqlite3`, archive/zip, SHA-256, existing Eino `ValidateGraphJSON`.

## Global Constraints

- Backend only: do not modify `web/templates`, `web/static`, i18n, or any other frontend file.
- Support exactly one `workflows/*.json` item; never execute package contents.
- Request ZIP maximum is 10 MiB and extracted total maximum is 20 MiB.
- Keep `workflow_definitions.version` local: create/rename is 1 and overwrite is the existing local version plus one.
- Use `workflow:read` only for export, `workflow:write` for every inspection/import route, and require existing RBAC `all` scope for package mutations.
- Preserve existing CRUD, validate, dry-run and run API response formats.

---

### Task 1: Package format, canonical hashes and deterministic export

**Files:**
- Create: `internal/workflow/package/manifest.go`
- Create: `internal/workflow/package/exporter.go`
- Test: `internal/workflow/package/exporter_test.go`

**Interfaces:**
- Produces: `Export(database.WorkflowDefinition) ([]byte, ExportMetadata, error)`, `InspectArchive(context.Context, []byte) (*InspectionResult, error)`, and typed package errors exposing `Code`, safe `Message`, and safe `Details`.
- Consumes: `database.WorkflowDefinition` and the existing graph JSON fields only.

- [ ] **Step 1: Write failing package tests.** Cover two identical exports producing byte-identical ZIPs, lower-case `sha256:` hashes, `manifest.json`/`checksums.sha256`/one workflow entry, and source revision equal to the source workflow's local version.
- [ ] **Step 2: Run the package test.** Run `go test ./internal/workflow/package -run 'TestExport'`; expected failure is missing package export symbols.
- [ ] **Step 3: Implement canonical JSON and exporter.** Canonicalize JSON with `Decoder.UseNumber`, hash canonical graph JSON and canonical item JSON, derive a stable package id and fixed ZIP metadata, then write entries in lexical order.
- [ ] **Step 4: Run the package test.** Run `go test ./internal/workflow/package -run 'TestExport'`; expected result is PASS.

### Task 2: Safe package inspection and validation

**Files:**
- Create: `internal/workflow/package/inspector.go`
- Test: `internal/workflow/package/inspector_test.go`

**Interfaces:**
- Consumes: ZIP bytes and `workflow.ValidateGraphJSON(context.Context, string)`.
- Produces: validated manifest/workflow payload, package/content/graph hashes, node/edge counts, and contract error codes without archive paths.

- [ ] **Step 1: Write failing inspector tests.** Use an exported valid package and assert accepted parsing; add independent cases for path traversal, duplicate names, symlink entries, undeclared files, checksum mismatch, two workflow entries, extracted-size overflow, invalid manifest and invalid graph.
- [ ] **Step 2: Run the inspector test.** Run `go test ./internal/workflow/package -run 'TestInspect'`; expected failure is missing inspection implementation.
- [ ] **Step 3: Implement archive checks before parsing.** Reject non-exact paths, duplicate names, links, unexpected entries and declared/actual oversized extraction; validate checksums and Manifest 1.0; require exactly one declared workflow entry; then reuse `ValidateGraphJSON`.
- [ ] **Step 4: Run the inspector test.** Run `go test ./internal/workflow/package -run 'TestInspect'`; expected result is PASS.

### Task 3: SQLite package state, lifecycle, and transactional application

**Files:**
- Modify: `internal/database/database.go`
- Create: `internal/database/workflow_package.go`
- Test: `internal/database/workflow_package_test.go`

**Interfaces:**
- Produces: inspection create/read/expiry methods, `ApplyWorkflowPackageImport` and `PurgeWorkflowPackageLifecycle(time.Time)`.
- Consumes: primitive database request structs carrying inspected payload and immutable conflict snapshot; no browser-supplied workflow JSON.

- [ ] **Step 1: Write failing DB tests.** Assert migration tables/indexes exist; inspection expiry transitions to `expired`; first successful application consumes inspection; same actor/key/same hash returns stored import; same key/different hash rejects; changed target snapshot rejects; overwrite increments local version; create and rename start at version 1; rollback leaves workflow/import/inspection unchanged on failure.
- [ ] **Step 2: Run the DB test.** Run `go test ./internal/database -run 'TestWorkflowPackage'`; expected failure is missing migration and methods.
- [ ] **Step 3: Add exact DDL and transactional repository method.** Add the two contract tables and indexes to `initTables`; in one `BEGIN` transaction recheck owner/status/expiry/idempotency/snapshot, apply the allowed action, insert import row, mark inspection consumed, and commit. Add 24-hour expired-inspection and 90-day import cleanup.
- [ ] **Step 4: Run the DB test.** Run `go test ./internal/database -run 'TestWorkflowPackage'`; expected result is PASS.

### Task 4: Import orchestration, HTTP handlers, audit and routes

**Files:**
- Create: `internal/workflow/package/importer.go`
- Create: `internal/handler/workflow_package.go`
- Modify: `internal/handler/workflow.go`
- Modify: `internal/app/app.go`
- Modify: `internal/security/rbac_middleware.go`
- Test: `internal/handler/workflow_package_test.go`

**Interfaces:**
- Consumes: authenticated `security.Session`, `Idempotency-Key`, multipart `file`, database package state, and typed package errors.
- Produces: contract response envelopes, `application/zip` export headers, cache invalidation after committed writes, and audit events in category `workflow_package`.

- [ ] **Step 1: Write failing handler/RBAC tests.** Cover 403 mapping for read/write permissions, 10 MiB file limit, export headers/404, creator-only inspection/import reads, 201 first apply/200 idempotent replay, contract error body/status, and the existing validate/dry-run/runs routes still resolving.
- [ ] **Step 2: Run the handler test.** Run `go test ./internal/handler -run 'TestWorkflowPackage'`; expected failure is missing routes/handlers.
- [ ] **Step 3: Implement service and handlers.** Limit upload bytes before multipart parsing; persist only validated payload; perform request-hash and action validation in importer; map package errors to approved statuses; invalidate the compiled cache only after commit; record export/inspect/import success and failure audits.
- [ ] **Step 4: Register and authorize routes.** Register exact paths `GET /workflows/:id/package`, `POST|GET /workflow-package-inspections`, and `POST|GET /workflow-package-imports`; make the route mapper explicit and treat inspection/import POSTs as process-global workflow mutations.
- [ ] **Step 5: Run focused handler tests.** Run `go test ./internal/handler -run 'TestWorkflowPackage'`; expected result is PASS.

### Task 5: Lifecycle wiring and final verification

**Files:**
- Modify: `internal/app/app.go`
- Test: the tests from Tasks 1-4

- [ ] **Step 1: Write the failing lifecycle wiring test or startup-level assertion.** Assert startup invokes package lifecycle cleanup and that the retention loop has no workflow-definition side effect.
- [ ] **Step 2: Implement startup cleanup/loop.** Invoke `PurgeWorkflowPackageLifecycle(time.Now().UTC())` at startup and start an hourly package lifecycle loop after the database is ready.
- [ ] **Step 3: Run format and focused verification.** Run `gofmt -w` only on changed Go files, `go test ./internal/workflow/package`, `go test ./internal/database -run 'TestWorkflowPackage'`, `go test ./internal/handler -run 'TestWorkflowPackage'`, and `git diff --check`.
- [ ] **Step 4: Run compatible regression verification.** Run `go test ./internal/database ./internal/handler ./internal/workflow` in an environment with the required C compiler, then inspect `git diff --check` and `git status --short` before committing.
- [ ] **Step 5: Commit verified files.** Run `git add internal/workflow/package internal/database/database.go internal/database/workflow_package.go internal/handler/workflow.go internal/handler/workflow_package.go internal/security/rbac_middleware.go internal/app/app.go docs/superpowers/plans/2026-07-13-local-workflow-package-mvp.md` followed by `git commit -m "feat: add local workflow package mvp"`.
