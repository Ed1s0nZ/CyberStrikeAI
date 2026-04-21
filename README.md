<div align="center">
  <img src="web/static/logo.png" alt="CyberStrikeAI Logo" width="200">
</div>

# CyberStrikeAI

**Autonomous AI-Powered Penetration Testing Platform**

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Anthropic](https://img.shields.io/badge/Claude-Supported-orange)](https://anthropic.com)
[![OpenAI](https://img.shields.io/badge/OpenAI-Compatible-blue)](https://openai.com)

CyberStrikeAI is an **AI-native security testing platform** built in Go. It integrates 100+ security tools, an intelligent orchestration engine, role-based testing with predefined security roles, a skills system with specialized testing skills, and comprehensive lifecycle management capabilities. Through native MCP protocol and AI agents, it enables end-to-end automation — from conversational commands to vulnerability discovery, attack-chain analysis, knowledge retrieval, and result visualization — delivering an auditable, traceable, and collaborative testing environment for security teams.

**Native Anthropic Claude support** — no proxy needed. Also works with any OpenAI-compatible API (local vLLM / Ollama / LiteLLM, OpenAI, DeepSeek, OpenRouter).

---

## Highlights

- 🤖 **AI decision engine** — Anthropic Claude (Messages API, native) plus any OpenAI-compatible provider (GPT, DeepSeek, vLLM, Ollama, etc.)
- 🔌 **Native MCP implementation** with HTTP / stdio / SSE transports and external MCP federation
- 🧰 **100+ prebuilt tool recipes** plus a YAML-based extension system
- 📄 **Large-result pagination**, compression, and searchable artefact archives
- 🔗 **Attack-chain graph**, risk scoring, and step-by-step replay
- 🔒 Password-protected web UI, audit logs, SQLite persistence
- 📚 **Knowledge base with vector search** and hybrid retrieval for security expertise (local vLLM embeddings supported)
- 📁 **Conversation grouping** with pinning, rename, and batch management
- 🛡️ **Vulnerability management**: CRUD, severity tracking, status workflow, and statistics
- 📋 **Batch task management**: create task queues, queue multiple tasks, execute them sequentially
- 🎭 **Role-based testing**: predefined roles (Penetration Testing, CTF, Web App Scanning, API Security, Cloud Audit, etc.) with custom prompts and per-role tool restrictions
- 🧩 **Native Go multi-agent orchestrator** — no ByteDance/CloudWeGo dependencies. A coordinator delegates work to Markdown-defined sub-agents through a `task` tool; main agent lives in `agents/orchestrator.md`, sub-agents under `agents/*.md`
- 🎯 **Skills system**: 42 specialized skill packs — SQL injection, XSS, API security, drone/ELRS exploitation, SDR, IoT, red-team operations, and more — attachable to roles or invoked on-demand
- 🐚 **WebShell management**: manage IceSword/AntSword-compatible webshells; virtual terminal, file manager, AI-assistant tab with per-connection conversation history
- 📱 **Telegram chatbot**: long-polling, multi-user, streaming progress — talk to CyberStrikeAI from a phone without exposing your server
- 🌐 **Proxy middleware**: global Tor / SOCKS5 / gsocket / proxychains routing for tool traffic
- 🔍 **Full-spectrum recon plugins**: built-in Shodan, Censys, crt.sh, DNSdumpster, FlareSolverr (WAF/CDN bypass)
- 🌍 **Internationalization**: English + Ukrainian (Українська) + Chinese UI (i18next-based, see [docs/frontend-i18n.md](docs/frontend-i18n.md))

---

## Tool Overview

CyberStrikeAI ships with 100+ curated tools covering the whole kill chain:

- **Network scanners** — nmap, masscan, rustscan, arp-scan, nbtscan
- **Web & app scanners** — sqlmap, nikto, dirb, gobuster, feroxbuster, ffuf, httpx
- **Vulnerability scanners** — nuclei, wpscan, wafw00f, dalfox, xsser
- **Subdomain enumeration** — subfinder, amass, findomain, dnsenum, fierce
- **Network-space search engines** — FOFA, ZoomEye, Shodan, Censys
- **API security** — graphql-scanner, arjun, api-fuzzer, api-schema-analyzer
- **Container security** — trivy, clair, kube-bench, kube-hunter, docker-bench-security
- **Cloud security** — prowler, scout-suite, cloudmapper, pacu, terrascan, checkov
- **Binary analysis** — gdb, radare2, ghidra, objdump, strings, binwalk
- **Exploitation** — metasploit, msfvenom, pwntools, ropper, ropgadget
- **Password cracking** — hashcat, john, hashpump
- **Forensics** — volatility, volatility3, foremost, steghide, exiftool
- **Post-exploitation** — linpeas, winpeas, mimikatz, bloodhound, impacket, responder
- **CTF utilities** — stegsolve, zsteg, hash-identifier, fcrackzip, pdfcrack, cyberchef
- **System helpers** — exec, create-file, delete-file, list-files, modify-file

See [tools/README.md](tools/README.md) for the full YAML schema and how to add your own.

---

## Quick Start

### Prerequisites

- **Go 1.21+** — [install](https://go.dev/dl/)
- **Python 3.10+** — for Python-based tools (api-fuzzer, http-framework-test, …)
- **Security tools** — nmap, subfinder, nuclei, etc. (the agent uses what is on `PATH` and falls back gracefully when a tool is missing)
- **API key** — Anthropic (Claude) or any OpenAI-compatible provider

### One-Command Setup

```bash
git clone https://github.com/cybersecua/CyberStrikeAI.git
cd CyberStrikeAI
sudo ./run_suite.sh
```

This will:
1. Check all dependencies (Go, Python, security tools)
2. Set network capabilities on pcap tools (nmap, tcpdump, etc.)
3. Verify DNS connectivity to your API provider
4. Build the binary
5. Start the server

### Manual Setup

```bash
git clone https://github.com/cybersecua/CyberStrikeAI.git
cd CyberStrikeAI

# Edit config
cp config.yaml.example config.yaml
# Set your API key in config.yaml

# Build and run
go build -o CyberStrikeAI ./cmd/server
./CyberStrikeAI
```

Open `http://localhost:8080` and log in with the generated password (shown in the terminal, or set `auth.password` in `config.yaml`).

### Upgrading

```bash
chmod +x upgrade.sh && ./upgrade.sh --yes
```

The upgrade script backs up your `config.yaml` and `data/` into `.upgrade-backup/`, pulls the latest release from GitHub, bumps the config `version` string, and restarts the server. Optional flags: `--tag vX.Y.Z`, `--no-venv`, `--preserve-custom`. Requires `curl`/`wget` and `rsync`. If GitHub rate-limits, set `GITHUB_TOKEN`.

---

## Configuration

### Anthropic Claude (Recommended)

```yaml
openai:
  provider: anthropic
  base_url: https://api.anthropic.com/v1
  api_key: sk-ant-api03-YOUR_KEY
  model: claude-sonnet-4-20250514
  max_total_tokens: 200000
```

### OpenAI

```yaml
openai:
  provider: openai
  base_url: https://api.openai.com/v1
  api_key: sk-YOUR_KEY
  model: gpt-4
```

### Local Models (vLLM / Ollama)

```yaml
openai:
  provider: openai
  base_url: http://127.0.0.1:8000/v1
  api_key: none
  model: your-model-name
  rate_limit_delay_ms: 0  # no rate limit for local models
```

### Knowledge Base (Local Embeddings)

Start the embedding server:

```bash
./run_mcp.sh
```

This launches `multilingual-e5-small` on port 8102 via vLLM. Configure in `config.yaml`:

```yaml
knowledge:
  enabled: true
  base_path: knowledge_base
  embedding:
    provider: openai
    model: multilingual-e5-small
    base_url: http://127.0.0.1:8102/v1
    api_key: none
  retrieval:
    top_k: 5
    similarity_threshold: 0.7
```

You can also download a pre-built `knowledge.db` (see [Releases](https://github.com/cybersecua/CyberStrikeAI/releases)) and drop it into `data/` — no indexing required. Knowledge items are organized by category (directory name) and auto-chunked for vector search; modified files are re-indexed incrementally.

### Telegram Bot

```yaml
robots:
  telegram:
    enabled: true
    bot_token: "YOUR_BOT_TOKEN"
    allowed_user_ids: [123456789]
```

### Proxy / Anonymization

```yaml
proxy:
  enabled: true
  mode: socks5       # socks5 | http | tor | proxychains
  host: 127.0.0.1
  port: 9050         # default for Tor; otherwise your SOCKS5/HTTP proxy port
```

`mode: tor` wires up the `tor` daemon if present; `mode: proxychains` delegates to a `proxychains` wrapper. Inference traffic (API calls to Anthropic/OpenAI) is proxy-exempt by design — only tool traffic is routed.

### MCP Server

```yaml
mcp:
  enabled: true
  host: 0.0.0.0
  port: 8081
  auth_header: X-MCP-Token
  auth_header_value: ""    # leave empty for auto-generation on first start
```

See [MCP quick starts](#mcp-server-1) below.

---

## Core Workflows

- **Conversational testing** — natural-language prompts drive toolchains; SSE streaming returns progress, tool calls, reasoning deltas, and the attack chain as it grows.
- **Single vs multi-agent** — with `multi_agent.enabled: true`, the chat UI flips between **single** (classic ReAct loop, `/api/agent-loop/stream`) and **multi** (`/api/multi-agent/stream`) per request. The multi-agent coordinator delegates sub-tasks to Markdown-defined specialists via the `task` tool.
- **Role-based testing** — pick a role (Penetration Testing, CTF, Web App Scanning, API Security, Cloud Security Audit, Binary Analysis, etc.) and the agent adopts its system prompt and tool whitelist.
- **Tool monitor** — inspect running jobs, execution logs, and large-result artefacts from the UI.
- **Vulnerability management** — create, update, and track vulnerabilities discovered during testing; filter by severity (critical/high/medium/low/info), status (open/confirmed/fixed/false_positive), and conversation. Stats and export endpoints included.
- **Batch task management** — create queues with multiple tasks; each runs as a separate conversation with status tracking (pending/running/completed/failed/cancelled) and full history.
- **Conversation groups** — organize conversations into groups, pin important ones, rename or delete via context menu.
- **WebShell management** — add IceSword/AntSword-compatible connections; use the virtual terminal for commands, the file manager for listing/editing/upload/delete/rename/download, and the AI assistant tab to script tests with per-connection history.
- **History & audit** — every conversation and tool invocation is stored in SQLite with replay.

### Built-in Safeguards

- Required-field validation prevents blank API credentials from hitting the provider.
- Strong password auto-generated when `auth.password` is empty.
- Unified auth middleware on every web/API call (Bearer-token flow).
- Per-tool timeout and structured logging for triage.

---

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Web UI (:8080)                         │
│    Chat · Dashboard · Monitor · Roles · Skills · Tasks   │
├─────────────────────────────────────────────────────────┤
│                   Go Backend                              │
│  ┌─────────┐  ┌──────────────┐  ┌────────────────────┐  │
│  │ Single  │  │ Multi-Agent  │  │ Knowledge Base     │  │
│  │ Agent   │  │ Orchestrator │  │ (RAG + Embeddings) │  │
│  │ Loop    │  │ (native Go)  │  │                    │  │
│  └────┬────┘  └──────┬───────┘  └────────┬───────────┘  │
│       │              │                    │              │
│  ┌────┴──────────────┴────────────────────┴───────────┐ │
│  │         Anthropic / OpenAI Adapter                  │ │
│  │   (native Claude Messages API + rate limiting)      │ │
│  └──────────────────────┬──────────────────────────────┘ │
│                         │                                │
│  ┌──────────────────────┴──────────────────────────────┐ │
│  │          MCP Tool Executor (117+ tools)              │ │
│  │  nmap · nuclei · sqlmap · subfinder · ffuf · masscan │ │
│  │  metasploit · hydra · gobuster · nikto · feroxbuster │ │
│  └──────────────────────────────────────────────────────┘ │
├─────────────────────────────────────────────────────────┤
│  MCP Server (:8081) · Telegram Bot · Burp Plugin         │
└─────────────────────────────────────────────────────────┘
```

### Multi-Agent Orchestrator

The native Go orchestrator (no external framework dependencies):
- Decomposes complex tasks into subtasks
- Delegates to specialist sub-agents with focused toolsets
- Tracks progress via todo lists
- Synthesizes results into comprehensive reports
- Supports parallel sub-agent execution

Sub-agents are defined in Markdown under `agents/` with YAML frontmatter (`id`, `name`, `description`, `tools`, optional `kind: orchestrator`). See [agents/orchestrator.md](agents/orchestrator.md) for the main coordinator and `agents/*.md` for specialists.

---

## Extending the Platform

### Creating a Custom Tool

1. Drop a YAML file under `tools/`, e.g. `tools/mytool.yaml`:
   ```yaml
   name: mytool
   command: /usr/bin/mytool
   enabled: true
   short_description: "One-line summary of the tool"
   description: |
     Longer Markdown description for the AI agent.
   args: ["--default-flag"]
   parameters:
     - name: target
       type: string
       description: "Target host or URL"
       required: true
       position: 0
     - name: ports
       type: string
       flag: "-p"
       description: "Port range"
   ```
2. Restart the server (or hit the reload endpoint); the tool appears in the tools list and is enableable from **Settings → Tools**.

Full schema: [tools/README.md](tools/README.md).

### Creating a Custom Role

```yaml
# roles/custom-role.yaml
name: Custom Role
description: Focused testing scenario
user_prompt: |
  You are a specialized security tester focusing on API security…
icon: "\U0001F4E1"
tools:
  - api-fuzzer
  - arjun
  - graphql-scanner
enabled: true
```

Reload the config and the role appears in the in-chat role selector.

### Creating a Skill

1. `mkdir skills/<skill-id>` and add `SKILL.md` with YAML front matter (`name`, `description`) plus a markdown body.
2. Optionally add `FORMS.md`, `REFERENCE.md`, `scripts/*.sh`, etc. — they are discovered at runtime.
3. Attach the skill to a role or let the agent invoke it on-demand via the skills tool.

See [skills/README.md](skills/README.md) and the existing 42 skill packs for templates.

---

## MCP Server

CyberStrikeAI speaks MCP in three directions: an embedded HTTP MCP server, a stdio-mode binary, and external MCP federation.

### MCP stdio Quick Start (Cursor / Claude Code / VS Code)

```bash
go build -o cyberstrike-ai-mcp ./cmd/mcp-stdio
```

Wire it into Cursor (`Settings → Tools & MCP → Add Custom MCP → Command`) or Claude Code (`~/.claude.json`):

```json
{
  "mcpServers": {
    "cyberstrike-ai": {
      "command": "/absolute/path/to/cyberstrike-ai-mcp",
      "args": ["--config", "/absolute/path/to/config.yaml"]
    }
  }
}
```

The client launches the process and talks MCP over stdin/stdout.

### MCP HTTP Quick Start

1. In `config.yaml` set `mcp.enabled: true`, `mcp.host`, `mcp.port`. For auth (recommended if the port is reachable on the network), set `mcp.auth_header` and either provide `mcp.auth_header_value` or leave it empty to auto-generate on first start.
2. Start the service. If MCP is enabled, the terminal prints a ready-to-paste JSON block containing the URL and headers.
3. Drop the JSON into `~/.cursor/mcp.json` (Cursor) or `~/.claude.json` (Claude Code) under `mcpServers`.

```json
{
  "mcpServers": {
    "cyberstrike-ai": {
      "url": "http://localhost:8081/mcp",
      "headers": { "X-MCP-Token": "<auto-generated-or-your-value>" },
      "type": "http"
    }
  }
}
```

Omitting `auth_header`/`auth_header_value` leaves the endpoint unauthenticated — suitable only for localhost or trusted networks.

### External MCP Federation

CyberStrikeAI can connect to external MCP servers in **HTTP**, **stdio**, or **SSE** mode. Open **Settings → External MCP** and add a server:

```json
{
  "my-http-mcp":  { "transport": "http",  "url": "http://127.0.0.1:8081/mcp",     "timeout": 30 },
  "my-stdio-mcp": { "command":   "python3", "args": ["/path/to/mcp-server.py"],   "timeout": 30 },
  "my-sse-mcp":   { "transport": "sse",   "url": "http://127.0.0.1:8082/sse",    "timeout": 30 }
}
```

Secrets can be referenced from the environment using `${VAR}` or `${VAR:-default}` syntax (matches Claude Desktop / Cursor / VS Code mcpServers). Expansion happens lazily at connection time so the templates stay in `config.yaml` on disk.

Toggle servers per engagement and monitor connection status, tool count, and error messages from the UI.

### Included MCP Servers

- **`mcp-servers/reverse_shell/`** — TCP reverse-shell listener; start/stop and send commands to connected targets. Works with CyberStrikeAI, Cursor, VS Code, Claude Code.
- **`mcp-servers/pent_claude_agent/`** — Claude Agent SDK wrapper that runs a nested AI pentest engineer with its own configurable tools/MCPs.

See [mcp-servers/README.md](mcp-servers/README.md).

---

## Plugins

- **`plugins/burp-suite/`** — Burp Suite extension: right-click any request → *Send to CyberStrikeAI (stream test)*. The extension prompts for an editable instruction before launching an AI-driven pentest and streams results back into Burp. Build output: `plugins/burp-suite/cyberstrikeai-burp-extension/dist/cyberstrikeai-burp-extension.jar`. Docs: [plugins/burp-suite/…/README.md](plugins/burp-suite/cyberstrikeai-burp-extension/README.md).

---

## API

CyberStrikeAI exposes a REST + SSE API. Everything the UI uses is reachable programmatically.

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/auth/login`, `/api/auth/change-password` | POST | Authentication + password rotation |
| `/api/agent-loop/stream` | POST | Single-agent chat (SSE streaming) |
| `/api/multi-agent/stream` | POST | Multi-agent orchestrated task (SSE) |
| `/api/multi-agent/markdown-agents` | GET/POST/PUT/DELETE | Manage Markdown-defined sub-agents |
| `/api/config`, `/api/config/test-api` | GET/POST | Read/update config, test API credentials and model availability |
| `/api/conversations`, `/api/conversations/:id` | GET/POST/PUT/DELETE | Manage conversations (with search, pagination, pinning) |
| `/api/groups` | GET/POST/PUT/DELETE | Conversation groups |
| `/api/roles`, `/api/roles/:name` | GET/POST/PUT/DELETE | CRUD on roles (YAML hot-reload) |
| `/api/vulnerabilities`, `/api/vulnerabilities/:id`, `/api/vulnerabilities/stats` | GET/POST/PUT/DELETE | Vulnerability management + statistics |
| `/api/batch-tasks`, `/api/batch-tasks/:queueId`, `/api/batch-tasks/:queueId/tasks[/:taskId]` | GET/POST/PUT/DELETE | Batch task queues and individual tasks |
| `/api/batch-tasks/:queueId/start`, `/api/batch-tasks/:queueId/cancel` | POST | Queue control |
| `/api/webshell/connections`, `/api/webshell/exec`, `/api/webshell/fileop` | GET/POST/PUT/DELETE | WebShell connections, command execution, file operations |
| `/api/knowledge/search` | POST | Vector search over knowledge base |
| `/api/monitor/stats` | GET | Tool execution statistics |
| `/api/plugins/*` | varies | Recon plugins: Shodan, Censys, FOFA, ZoomEye, crt.sh, DNSdumpster, FlareSolverr |

Full OpenAPI docs live at `http://localhost:8080` → **API Documentation** tab and include i18n-aware descriptions for each endpoint.

---

## Scripts

| Script | Purpose |
|--------|---------|
| `./run_suite.sh` | Full dependency check, build, and launch |
| `./run_mcp.sh` | Start local vLLM embedding server for the knowledge base |
| `./upgrade.sh` | Upgrade to latest release from GitHub |
| `./run.sh` | Minimal launcher (build + run) |

---

## Project Layout

```
CyberStrikeAI/
├── cmd/                 # Server + MCP stdio + test binaries
├── internal/            # Agent, MCP core, handlers, security executor, proxy, plugins
├── web/                 # Static SPA + HTML templates + i18n JSON
├── tools/               # YAML tool recipes
├── roles/               # Role configurations (12+ predefined security testing roles)
├── skills/              # Skill packs (43 packs, SKILL.md + optional files)
├── agents/              # Multi-agent Markdown (orchestrator + sub-agents)
├── plugins/             # Burp Suite extension
├── mcp-servers/         # Standalone MCP servers (reverse_shell, pent_claude_agent)
├── knowledge_base/      # RAG source markdown
├── docs/                # Documentation (i18n, robot/chatbot, plugins, docker, memory)
├── config.yaml          # Runtime configuration
└── README.md
```

---

## Basic Usage Examples

```
Scan open ports on 192.168.1.1
Perform a comprehensive port scan on 192.168.1.1 focusing on 80,443,22
Check if https://example.com/page?id=1 is vulnerable to SQL injection
Scan https://example.com for hidden directories and outdated software
Enumerate subdomains for example.com, then run nuclei against the results
```

## Advanced Playbooks

```
Load the recon-engagement template, run amass/subfinder, then brute-force dirs on every live host.
Use the external Burp-based MCP server for authenticated traffic replay, then pass findings back for graphing.
Compress the 5 MB nuclei report, summarize critical CVEs, and attach the artifact to the conversation.
Build an attack chain for the latest engagement and export the node list with severity >= high.
```

---

## Security Considerations

- **Run in a container** for production use — the agent has unrestricted shell access by design.
- **API keys** — use `${VAR}` / `${VAR:-default}` env references in MCP configs and plain env vars elsewhere; do not commit plaintext secrets to `config.yaml`.
- **Network binding** — defaults to `0.0.0.0:8080`. Bind to `127.0.0.1` if the host is directly exposed.
- **Authentication** — all API endpoints require a session token; password auto-generates on first run if `auth.password` is empty.

---

## Related Documentation

- [tools/README.md](tools/README.md) — tool YAML schema and extension guide
- [skills/README.md](skills/README.md) — skill pack layout
- [docs/frontend-i18n.md](docs/frontend-i18n.md) — frontend internationalization plan
- [docs/robot_en.md](docs/robot_en.md) — Telegram chatbot setup (DingTalk/Lark/WeCom setup preserved from upstream for users on those platforms)
- [docs/plugins.md](docs/plugins.md) — recon plugin system reference
- [docs/docker_en.md](docs/docker_en.md) — Docker deployment
- [docs/memory_en.md](docs/memory_en.md) — agent persistent memory / RAG context

---

## Fork History

This is the [cybersecua](https://github.com/cybersecua) fork of [Ed1s0nZ/CyberStrikeAI](https://github.com/Ed1s0nZ/CyberStrikeAI).

Major additions in this fork:

- **Native Anthropic Claude support** — direct Messages API, no proxy, native streaming, rate-limit-aware retry
- **Native Go multi-agent orchestrator** — replaced ByteDance / CloudWeGo Eino framework
- **Chinese-dependency-free build** — removed ByteDance, CloudWeGo, Lark, DingTalk, WeCom SDKs
- **API health check & model discovery** — the "Test API" button auto-detects available models and rate limits
- **Telegram bot integration** — long-polling, multi-user, streaming progress, no public IP required
- **Proxy middleware** — global Tor / SOCKS5 / gsocket / proxychains routing for tool traffic
- **Recon plugin system** — Shodan, Censys, FOFA, ZoomEye, crt.sh, DNSdumpster, FlareSolverr as first-class plugins with per-plugin UI tabs and i18n
- **DroidRun / Cuttlefish integration** — Android VM control for mobile security testing
- **43 homegrown skills** — drone exploitation, SDR / LoRa ops, ELRS exploitation, Bluetooth, IoT, Windows red team, SIGINT, chisel tunneling, and more
- **Full English codebase + Ukrainian locale** — removed every Chinese comment and string from code; added `uk-UA` i18n catalog
- **Per-tier sampling** — separate temperature / top_p / top_k for main, tool, and summary models
- **Stronger MCP env-var handling** — `${VAR}` / `${VAR:-default}` references resolved lazily at connection time so secrets stay out of `config.yaml` on disk

See the commit history for the full list.

---

## Contributing

Pull requests welcome. Focus areas:
- New security-tool integrations (YAML definitions in `tools/`)
- New skill packs (`SKILL.md` files in `skills/`)
- New recon plugins
- UI / i18n improvements (translations to any locale)
- Documentation

---

## License

MIT License. See [LICENSE](LICENSE).

---

## ⚠️ Disclaimer

**This tool is for educational and authorized testing purposes only.**

CyberStrikeAI is a professional security testing platform designed to assist security researchers, penetration testers, and IT professionals in conducting security assessments and vulnerability research **with explicit authorization**.

By using this tool you agree to:
- Only test systems for which you have explicit written authorization;
- Comply with all applicable laws, regulations, and ethical standards;
- Take full responsibility for any misuse.

The developers are not responsible for misuse. Ensure your usage complies with local laws and the target system owner's authorization.

---

## Credits

- **Original project**: [Ed1s0nZ/CyberStrikeAI](https://github.com/Ed1s0nZ/CyberStrikeAI) — Chinese cybersecurity community
- **Fork maintainer**: [cybersecua](https://github.com/cybersecua) — Ukrainian cybersecurity
- **AI assistance**: Claude Code (Anthropic)

---

![Stargazers over time](https://starchart.cc/cybersecua/CyberStrikeAI.svg)
