# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

CyberStrikeAI is an AI-native security testing platform written in Go. It orchestrates 100+ security tools via an AI agent, exposes them through MCP (Model Context Protocol), and provides a web UI for conversational security testing. The codebase is primarily in Chinese (comments, config labels, commit messages).

Upstream canonical repo: [Ed1s0nZ/CyberStrikeAI](https://github.com/Ed1s0nZ/CyberStrikeAI). This fork may carry extra features (for example parallel scan orchestration, Anthropic-oriented OpenAI client wiring) merged on top of upstream `main`.

### Git remotes (recommended for fork contributors)

Use **`origin`** for your GitHub fork (push/pull daily work) and **`upstream`** for `Ed1s0nZ/CyberStrikeAI` (fetch official changes).

```bash
git remote add upstream https://github.com/Ed1s0nZ/CyberStrikeAI.git   # once
git fetch upstream
git merge upstream/main    # or: git rebase upstream/main
git push origin main
```

## Build & Run

```bash
# Full setup (Python venv + Go build + run server)
./run.sh

# Build only
go build -o cyberstrike-ai cmd/server/main.go

# Run with custom config
./cyberstrike-ai -config config.yaml

# Run tests
go test ./...

# Run a single package's tests
go test ./internal/agent/
go test ./internal/security/
go test ./internal/storage/
go test ./internal/multiagent/
go test ./internal/einomcp/
go test ./internal/agents/
```

**Prerequisites:** Go 1.23+, Python 3.10+ (for security tool wrappers in `requirements.txt`)

**Smoke check:** After `go build`, run the binary with a config whose `server.port` is free; `GET /` should return `200` and the dashboard HTML. If `security.tools_dir` is relative, start the process with the working directory (or config path) set so `tools/` resolves correctly.

## Architecture

**Entry point:** `cmd/server/main.go` → loads config → creates `app.App` → starts Gin HTTP server on port 8080.

### Core packages (`internal/`)

| Package | Purpose |
|---------|---------|
| `app` | Application bootstrap, Gin router setup, wires all components together |
| `agent` | AI agent loop — iterates tool calls from LLM, manages memory compression |
| `openai` | OpenAI-compatible API client (works with GPT, Claude, DeepSeek, etc.) |
| `config` | YAML config loader (`config.yaml`) |
| `database` | SQLite persistence — conversations, groups, attack chains, batch tasks, vulnerabilities |
| `handler` | HTTP/WebSocket handlers (Gin) — one file per domain (agent, auth, conversation, vulnerability, etc.) |
| `security` | Tool executor — runs CLI tools as subprocesses; auth middleware |
| `mcp` | MCP protocol server (HTTP/SSE/stdio transports) + external MCP client federation |
| `knowledge` | Knowledge base — embedding, indexing, hybrid vector+keyword retrieval |
| `skills` | Skills system — loads SKILL.md files, exposes as tools for the AI agent |
| `attackchain` | Attack chain graph builder from conversation history |
| `storage` | Large result storage — pagination, compression, archival |
| `robot` | Chatbot integrations (DingTalk, Lark/Feishu) via long-lived connections |
| `logger` | Zap-based structured logger |
| `multiagent` | Optional Eino DeepAgent multi-agent run (`RunDeepAgent`); sub-agents from `multi_agent` config and/or `agents/*.md` |
| `einomcp` | Bridges MCP/tool execution into the Eino tool graph (conversation holder, streaming tool output) |
| `agents` | Loads Markdown agent definitions (YAML front matter) for multi-agent orchestration |

### Data-driven configuration (YAML directories)

- **`tools/`** — One YAML per security tool (nmap, sqlmap, nuclei, etc.). Defines `name`, `command`, `args`, `parameters`, `description`. The executor runs these as subprocesses.
- **`roles/`** — Predefined security testing personas (CTF, Penetration Testing, Web Scanning, etc.). Each role has a system prompt and a tool whitelist.
- **`skills/`** — Each subdirectory contains a `SKILL.md` with structured testing methodology (SQL injection, XSS, API security, etc.). Loaded by the skills manager and callable by the AI agent.
- **`knowledge_base/`** — Markdown reference docs (SQL injection variants, prompt injection, etc.) indexed for vector search.
- **`agents/`** — Markdown files describing sub-agents for the Eino multi-agent path (orchestrator + specialists); see `docs/MULTI_AGENT_EINO.md` in tree.

### Web frontend

`web/static/` and `web/templates/` — served by Gin. Dashboard, chat console, attack chain visualization, task/vulnerability management, chat file uploads UI, optional multi-agent mode toggle, parallel scan modal (this fork).

### Key data flow

1. User sends message via WebSocket → `handler/agent.go`
2. **Single-agent path:** Agent loop (`agent/agent.go`) sends to LLM with available tools; or **multi-agent path** when enabled: `POST /api/multi-agent/stream` → `internal/multiagent` (Eino) with tools via `einomcp`
3. LLM returns tool calls → `security/executor.go` runs CLI commands (and/or MCP tools through the same definitions)
4. Results fed back to LLM for next iteration (up to `max_iterations` / multi-agent iteration settings)
5. Attack chain built from conversation → `attackchain/builder.go`
6. Everything persisted to SQLite via `database/`
7. **Parallel scan (fork):** HTTP APIs under `/api/parallel-scan` orchestrate multiple agent-style runs; see `internal/agent/parallel_scan_manager.go` and related handlers

## Configuration

All runtime config lives in `config.yaml`. Key sections: `openai` (LLM credentials; may include `provider` such as Anthropic-compatible proxies and optional macOS Keychain key id), `server` (host/port), `agent` (iteration limits), `multi_agent` (Eino multi-agent: `enabled`, `default_mode`, robot/batch flags, `sub_agents`, `agents_dir`), `mcp` (protocol server), `knowledge` (embedding/retrieval), `security` (`tools_dir` — path is relative to the **config file’s directory**, not necessarily the process cwd). Chat attachments are stored under `chat_uploads/` with APIs in `handler/chat_uploads.go`. Config is also editable through the web UI settings panel.

**Do not commit** machine-specific `config.yaml` edits (passwords, API keys, internal proxy URLs) unless you are replacing them with placeholders for documentation.
