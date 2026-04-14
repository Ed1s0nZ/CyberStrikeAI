# Extensibility Governance

## Purpose
Define how operator-managed extension artifacts such as roles, skills, and Markdown agents are created, validated, loaded, and composed into effective runtime behavior.

## Requirements
### Requirement: Managed extension artifacts
The platform SHALL support operator-managed lifecycle operations for roles, skills, and Markdown-defined agents.

#### Scenario: Operator updates an extension artifact
- **WHEN** an operator creates or updates a supported extension artifact
- **THEN** the artifact becomes durable runtime intent only after it is persisted in its authoritative storage form

### Requirement: Deterministic effective runtime composition
The platform SHALL compute a deterministic effective runtime view from config-backed and file-backed extension artifacts.

#### Scenario: Runtime loads extension-backed policy
- **WHEN** a request depends on roles, skills, or Markdown agents
- **THEN** the system resolves a deterministic effective artifact set for that request

## Overview
This domain governs operator-managed extension artifacts: roles, skills, and Markdown-defined agents. These artifacts let the platform change prompt policy, tool policy, and orchestration behavior without rewriting application code.

## Capabilities
### Capability 1: Role Governance
- The system must support creation, inspection, update, listing, and deletion of named roles.
- The system must allow roles to define prompt policy, tool policy, skill hints, and enablement state.

### Capability 2: Skill Governance
- The system must support creation, inspection, update, listing, and deletion of filesystem-backed skills.
- The system must record per-skill usage statistics and expose role-to-skill binding relationships.
- The system must make the skill corpus available both to operators and to agents through dedicated MCP tools.

### Capability 3: Markdown Agent Governance
- The system must support operator-managed Markdown-defined sub-agents and optional orchestrator definitions.
- The system must validate Markdown agent structure before treating the file as loadable runtime intent.

### Capability 4: Effective Runtime Composition
- The system must merge config-backed and file-backed extension artifacts into a deterministic effective runtime view for each request.

## Interfaces
- Role APIs:
  - `GET /api/roles`
  - `GET /api/roles/:name`
  - `GET /api/roles/skills/list`
  - `POST /api/roles`
  - `PUT /api/roles/:name`
  - `DELETE /api/roles/:name`
- Skill APIs:
  - `GET /api/skills`
  - `GET /api/skills/stats`
  - `DELETE /api/skills/stats`
  - `GET /api/skills/:name`
  - `GET /api/skills/:name/bound-roles`
  - `POST /api/skills`
  - `PUT /api/skills/:name`
  - `DELETE /api/skills/:name`
  - `DELETE /api/skills/:name/stats`
- Markdown agent APIs:
  - `GET /api/multi-agent/markdown-agents`
  - `GET /api/multi-agent/markdown-agents/:filename`
  - `POST /api/multi-agent/markdown-agents`
  - `PUT /api/multi-agent/markdown-agents/:filename`
  - `DELETE /api/multi-agent/markdown-agents/:filename`

## State Machine
- Role lifecycle:
  - `Draft`
  - `Enabled`
  - `Disabled`
  - `Deleted`
- Skill lifecycle:
  - `Draft`
  - `Loadable`
  - `Updated`
  - `Deleted`
- Markdown agent lifecycle:
  - `Draft`
  - `Validated`
  - `Loadable`
  - `Deleted`

## Data Flow
1. Operators create or modify roles, skills, or Markdown agent files through APIs.
2. The platform persists those artifacts into config-backed or filesystem-backed storage.
3. Runtime caches are invalidated or reloaded when the artifact type supports lazy refresh.
4. Agent execution consumes the effective role policy, available skills, and available Markdown agents.
5. Skill invocation statistics and bound-role metadata remain queryable for governance and debugging.

## Constraints
- Role names must be unique and stable because they are referenced from conversations, queues, and robot sessions.
- Skill artifacts are directory-backed and must resolve to a canonical `SKILL.md` or compatible fallback filename.
- Markdown agent definitions require valid front matter and must not create multiple conflicting orchestrator definitions.
- Role policy constrains future executions only; it must not rewrite historical conversation records.
- Lazy loading is acceptable for skills and Markdown agents, but the effective artifact set must be deterministic for each request.

## Failure Handling
- Invalid or duplicate role/skill/agent definitions are rejected explicitly.
- Malformed skill or Markdown agent files may be skipped or rejected, but they must never crash unrelated runtime domains.
- Missing skills referenced by a role remain visible as governance drift and should surface as load errors when invoked.
- Cache invalidation failure must not silently serve corrupted content; the platform should fall back to a fresh file read whenever possible.
