# OpenSpec Domains

This directory is the domain-level specification map for 能盾智御. Each subdirectory is a bounded capability area and is intended to act as a stable source of truth for future changes.

## Domain Map
- `platform-core`
  - Platform lifecycle, serving model, persistence posture, and global invariants.
- `access-control-and-configuration`
  - Authentication, session lifecycle, configuration persistence, and runtime apply semantics.
- `conversation-workspace`
  - Conversations, messages, process details, attachments, grouping, and workspace organization.
- `agent-runtime`
  - Default single-agent execution, streaming, cancellation, role policy, and WebShell assistant mode.
- `multi-agent-orchestration`
  - Deep-agent orchestration, sub-agent composition, and Markdown-defined agent behavior.
- `batch-task-automation`
  - Queue-based sequential execution, cron scheduling, and per-task durability.
- `tool-runtime-and-mcp`
  - MCP registry, tool execution, terminal runtime, external MCP federation, and monitoring.
- `knowledge-base`
  - Filesystem-backed Markdown corpus, indexing, retrieval, and retrieval audit logs.
- `security-operations`
  - FOFA workflows, WebShell operations, vulnerability management, and attack-chain synthesis.
- `robot-channel-integration`
  - WeCom/DingTalk/Lark ingress, robot command semantics, and conversation mapping.
- `extensibility-governance`
  - Roles, skills, Markdown agents, and extension artifact lifecycle.

## Cross-Domain Rules
- Conversation-linked evidence is authoritative across `conversation-workspace`, `agent-runtime`, `security-operations`, and `batch-task-automation`.
- Tool invocations must remain observable through `tool-runtime-and-mcp` even when initiated by agents, robots, or batch queues.
- Runtime configuration changes are governed by `access-control-and-configuration` and may affect every other domain, but they must not silently rewrite historical records.
- Optional domains may degrade independently; `platform-core` remains responsible for fail-visible behavior.
