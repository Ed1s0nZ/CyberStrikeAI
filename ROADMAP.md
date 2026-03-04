# CyberStrikeAI Roadmap

This roadmap outlines the planned development trajectory for CyberStrikeAI. Items are grouped by theme and approximate horizon. Priorities may shift based on community feedback—open an issue or PR to influence the direction.

---

## Legend

| Symbol | Meaning |
|--------|---------|
| ✅ | Shipped / Available |
| 🚧 | In Progress |
| 📋 | Planned |
| 💡 | Under Consideration |

---

## Released (v1.x)

### Core Platform
- ✅ AI agent engine with OpenAI-compatible model support (GPT, Claude, DeepSeek, etc.)
- ✅ Native MCP implementation — HTTP, stdio, and SSE transports
- ✅ External MCP federation (HTTP / stdio / SSE modes)
- ✅ 100+ prebuilt tool recipes in YAML
- ✅ YAML-based tool extension system (hot-reload from `tools/` directory)
- ✅ Large-result pagination, compression, and searchable archives
- ✅ SQLite persistence for conversations, vulnerabilities, and audit logs
- ✅ Password-protected Web UI with Bearer-token auth and session management
- ✅ Streaming SSE output for real-time task progress

### Security Testing Features
- ✅ Role-based testing system (12+ predefined roles: Penetration Testing, CTF, Web App Scanning, API Security, Binary Analysis, Cloud Security Audit, etc.)
- ✅ Skills system (20+ predefined skills: SQL injection, XSS, API security, container security, etc.)
- ✅ Attack-chain graph with severity scoring and step-by-step replay
- ✅ Vulnerability management — CRUD, severity/status tracking, statistics
- ✅ Batch task management — create queues, add tasks, sequential execution with full status tracking
- ✅ Knowledge base with vector search and hybrid (vector + BM25 keyword) retrieval
- ✅ **Corpus-level BM25 Okapi** — real inverse document frequency scoring built from all indexed chunks; replaces the previous per-document approximation
- ✅ Auto-indexing of Markdown knowledge files with incremental updates
- ✅ FOFA / ZoomEye search engine integration

### Agent Intelligence
- ✅ **Persistent memory** — cross-session key-value store (SQLite-backed) with categories (credential, target, vulnerability, fact, note); survives conversation compression; exposed as four agent tools (`store_memory`, `retrieve_memory`, `list_memories`, `delete_memory`)
- ✅ **Time awareness** — current date/time, timezone, and session age automatically injected into every system prompt; configurable via `agent.time_awareness`; `get_current_time` tool for on-demand queries

### Integrations & UX
- ✅ DingTalk and Lark (Feishu) chatbot via persistent long-lived connections
- ✅ Telegram bot via long-polling — multi-user, progress streaming, MCP tool control, role and conversation management; configurable via Web UI
- ✅ Web console with terminal, task monitor, conversation groups, and role selector
- ✅ Conversation grouping — pinning, renaming, batch management
- ✅ MCP stdio mode for Cursor / IDE integration
- ✅ OpenAPI documentation endpoint

---

## Near-Term (Next 1–2 Releases)

### Agent & Orchestration
- ✅ **Parallel tool execution** — agent fans out independent tool calls concurrently to reduce total time on multi-step engagements
- ✅ **Agent memory improvements** — persistent cross-session memory store with category tagging; BM25 corpus index for smarter knowledge retrieval
- ✅ **Memory UI panel** — web interface to view, search, edit, and delete persistent memory entries; category filters; stats strip; bulk delete
- 📋 **Memory expiry / TTL** — optional time-to-live on memory entries so stale facts are automatically purged
- 📋 **Structured task templates** — YAML-defined recon/pentest playbooks that the agent can load and execute end-to-end
- 📋 **Tool chaining macros** — define multi-step pipelines (e.g., subfinder → httpx → nuclei) as a single named operation

### UI / UX
- 📋 **Fully translated English UI** — complete localization of all UI text (in progress in this release)
- 📋 **Dark / light theme toggle** — user-configurable color scheme
- 📋 **Improved attack-chain export** — export as PDF, PNG, or JSON for reporting
- 📋 **Vulnerability report generator** — one-click HTML/Markdown pentest report from discovered vulnerabilities
- 📋 **Real-time collaboration** — allow multiple users to observe or join a running session

### Integrations
- 📋 **Slack / Teams bot** — extend the chatbot system to Slack and Microsoft Teams
- 📋 **Webhook notifications** — send task completion, vulnerability discovery, or attack-chain events to external systems (Slack, PagerDuty, etc.)
- 📋 **JIRA / GitHub Issues integration** — automatically create issues from discovered vulnerabilities
- 📋 **Telegram inline keyboard** — add interactive buttons (confirm/cancel actions, quick role switching) to Telegram bot responses
- 📋 **Telegram file transfer** — send large tool output as downloadable files when the result exceeds the message size limit

---

## Mid-Term (3–6 Months)

### AI & Automation
- 📋 **Multi-model routing** — automatically select the best model (reasoning model for complex planning, faster model for tool execution) to optimize cost and latency
- 📋 **Autonomous recon-to-report pipeline** — fully automated end-to-end pentest workflow from target scoping to final report generation
- ✅ **RAG-enhanced agent** — deeper integration of the knowledge base into agent decision-making for better tool selection and exploit guidance; proactive context injection based on task semantics
- 📋 **Custom agent personas** — allow organizations to define their own agent behavior, escalation rules, and toolset restrictions
- 📋 **Fine-tuned security model support** — tested integration with security-focused fine-tuned models

### Security & Compliance
- 📋 **Multi-user RBAC** — role-based access control with user accounts, scoped permissions (read-only analyst, full operator, admin)
- 📋 **Audit log export** — export structured audit logs (JSON / SYSLOG) to SIEM systems
- 📋 **Engagement scoping** — define authorized target scope and enforce tool/output restrictions within scope boundaries
- 📋 **Data retention policies** — auto-purge or archive old conversations and results

### Tool Ecosystem
- 📋 **Tool marketplace / registry** — community-contributed tool recipes with one-click import
- 📋 **Tool sandboxing** — optional Docker/container isolation for each tool invocation
- 📋 **Tool health monitoring** — detect missing or misconfigured tools and suggest installation commands
- 📋 **Burp Suite extension** — native Burp extension for bi-directional traffic sharing with CyberStrikeAI

### Knowledge Base
- 📋 **Auto-knowledge ingestion** — automatically import CVE details, exploit-db entries, and security advisories into the knowledge base
- 📋 **Knowledge base sharing** — export and import knowledge bases as portable bundles
- 📋 **Semantic deduplication** — automatically merge near-duplicate knowledge items
- 📋 **BM25 index persistence** — store the BM25 corpus index on disk so it does not need to be rebuilt on every startup

---

## Long-Term / Under Consideration

### Platform
- 💡 **Web-based IDE / notebook** — Jupyter-like interface for scripting custom pentest workflows
- 💡 **Plugin architecture** — first-class SDK for third-party integrations beyond MCP
- 💡 **Distributed agent execution** — run agents across multiple nodes for large-scale assessments
- 💡 **API gateway proxy** — transparent security testing proxy mode for API testing
- 💡 **Mobile app** — native iOS/Android companion app

### AI
- 💡 **Reasoning traces / chain-of-thought display** — show the AI's reasoning steps in the UI for transparency
- 💡 **Human-in-the-loop mode** — require explicit approval before executing high-risk tools
- 💡 **Adaptive learning** — capture operator feedback on agent decisions to improve future recommendations
- 💡 **Vulnerability correlation engine** — automatically correlate findings across multiple engagements to identify patterns

### Community
- 💡 **Role / skill / tool sharing hub** — centralized repository for community-contributed roles, skills, and tools
- 💡 **CTF challenge integration** — direct integration with CTF platforms (HackTheBox, TryHackMe, PicoCTF) for practice mode
- 💡 **Certification exam assistance mode** — guided study mode for OSCP, CEH, and similar certifications

---

## Contributing

We welcome contributions in all areas. To propose a roadmap item or discuss implementation details:

1. **Open an issue** using the [Feature Request template](.github/ISSUE_TEMPLATE/feature_request.md)
2. **Join the discussion** on existing roadmap issues
3. **Submit a PR** — all contributions are reviewed and credited

See [README.md](README.md) for development setup instructions.

---

*Last updated: 2026-03-04 — Memory UI panel shipped; RAG-enhanced agent graduated from planned to released. This roadmap is subject to change. Follow the repository to stay updated.*

---

## Telegram Bot — Detailed Roadmap

The Telegram integration (shipped in v1.3.17) provides a foundation for deeper mobile-first control. The following items extend it further:

| Item | Status | Description |
|------|--------|-------------|
| Long-polling bot with multi-user support | ✅ | Independent sessions per Telegram user ID |
| Live progress streaming | ✅ | Placeholder message edited with tool-call steps during execution |
| Role switching via bot commands | ✅ | `role <name>` command supported in Telegram |
| MCP tool configuration via Web UI | ✅ | Tools added/toggled in settings are immediately available to the bot |
| User whitelist (allowed_user_ids) | ✅ | Restrict bot access to specific Telegram user IDs |
| Group chat support (@ mentions) | ✅ | Bot responds to @mention in groups |
| Inline keyboard for confirmations | 📋 | Buttons for dangerous actions (delete, stop) |
| File upload for large results | 📋 | Send results >4096 chars as a `.txt` file |
| Telegram webhook mode (optional) | 📋 | Alternative to polling for low-latency deployments with public IP |
| `/start` onboarding message | 📋 | Automatic welcome message with quick-start tips on first contact |
