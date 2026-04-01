<div align="center">
  <img src="web/static/logo.png" alt="CyberStrikeAI Logo" width="200">
</div>

# CyberStrikeAI

**Autonomous AI-Powered Penetration Testing Platform**

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Anthropic](https://img.shields.io/badge/Claude-Supported-orange)](https://anthropic.com)
[![OpenAI](https://img.shields.io/badge/OpenAI-Compatible-blue)](https://openai.com)

CyberStrikeAI is an **AI-native security testing platform** built in Go. It orchestrates 117+ security tools through an intelligent agent that autonomously plans, executes, and reports penetration tests — from reconnaissance to exploitation.

**Native Anthropic Claude support** — no proxy needed. Also works with any OpenAI-compatible API (local models via vLLM/Ollama, OpenAI, DeepSeek, etc).

Highlights
🤖 AI decision engine with OpenAI-compatible models (GPT, Claude, DeepSeek, etc.)
🔌 Native MCP implementation with HTTP/stdio/SSE transports and external MCP federation
🧰 100+ prebuilt tool recipes + YAML-based extension system
📄 Large-result pagination, compression, and searchable archives
🔗 Attack-chain graph, risk scoring, and step-by-step replay
🔒 Password-protected web UI, audit logs, and SQLite persistence
📚 Knowledge base with vector search and hybrid retrieval for security expertise
📁 Conversation grouping with pinning, rename, and batch management
🛡️ Vulnerability management with CRUD operations, severity tracking, status workflow, and statistics
📋 Batch task management: create task queues, add multiple tasks, and execute them sequentially
🎭 Role-based testing: predefined security testing roles (Penetration Testing, CTF, Web App Scanning, etc.) with custom prompts and tool restrictions
🧩 Multi-agent mode (Eino DeepAgent): optional orchestration where a coordinator delegates work to Markdown-defined sub-agents via the task tool; main agent in agents/orchestrator.md (or kind: orchestrator), sub-agents under agents/*.md; chat mode switch when multi_agent.enabled is true (see Multi-agent doc)
🎯 Skills system: 20+ predefined security testing skills (SQL injection, XSS, API security, etc.) that can be attached to roles or called on-demand by AI agents
📱 Chatbot: DingTalk and Lark (Feishu) long-lived connections so you can talk to CyberStrikeAI from mobile (see Robot / Chatbot guide for setup and commands)
🐚 WebShell management: Add and manage WebShell connections (e.g. IceSword/AntSword compatible), use a virtual terminal for command execution, a built-in file manager for file operations, and an AI assistant tab that orchestrates tests and keeps per-connection conversation history; supports PHP, ASP, ASPX, JSP and custom shell types with configurable request method and command parameter.
Plugins
CyberStrikeAI includes optional integrations under plugins/.

Burp Suite extension: plugins/burp-suite/cyberstrikeai-burp-extension/
Build output: plugins/burp-suite/cyberstrikeai-burp-extension/dist/cyberstrikeai-burp-extension.jar
Docs: plugins/burp-suite/cyberstrikeai-burp-extension/README.md
Tool Overview
CyberStrikeAI ships with 100+ curated tools covering the whole kill chain:

Network Scanners – nmap, masscan, rustscan, arp-scan, nbtscan
Web & App Scanners – sqlmap, nikto, dirb, gobuster, feroxbuster, ffuf, httpx
Vulnerability Scanners – nuclei, wpscan, wafw00f, dalfox, xsser
Subdomain Enumeration – subfinder, amass, findomain, dnsenum, fierce
Network Space Search Engines – fofa_search, zoomeye_search
API Security – graphql-scanner, arjun, api-fuzzer, api-schema-analyzer
Container Security – trivy, clair, docker-bench-security, kube-bench, kube-hunter
Cloud Security – prowler, scout-suite, cloudmapper, pacu, terrascan, checkov
Binary Analysis – gdb, radare2, ghidra, objdump, strings, binwalk
Exploitation – metasploit, msfvenom, pwntools, ropper, ropgadget
Password Cracking – hashcat, john, hashpump
Forensics – volatility, volatility3, foremost, steghide, exiftool
Post-Exploitation – linpeas, winpeas, mimikatz, bloodhound, impacket, responder
CTF Utilities – stegsolve, zsteg, hash-identifier, fcrackzip, pdfcrack, cyberchef
System Helpers – exec, create-file, delete-file, list-files, modify-file

---

## Key Features

- **Autonomous Agent** — AI plans attack strategy, selects tools, chains findings, persists through hundreds of iterations
- **117+ Security Tools** — nmap, nuclei, sqlmap, subfinder, ffuf, masscan, metasploit, and many more
- **Multi-Agent Orchestrator** — decomposes complex tasks into parallel subtasks with specialist sub-agents
- **Native Anthropic Support** — direct Claude API integration (Opus/Sonnet/Haiku) with automatic rate limiting and retry
- **OpenAI Compatible** — works with any OpenAI-format API (vLLM, Ollama, LiteLLM, OpenRouter)
- **Smart API Discovery** — "Test API" button auto-detects available models and rate limits
- **Knowledge Base (RAG)** — local embedding server (vLLM) for semantic search over security knowledge
- **Skills System** — 42 specialized skill modules (SQL injection, XSS, ELRS exploitation, drone security, etc.)
- **Role-Based Testing** — predefined roles: Penetration Testing, Binary Analysis, Information Gathering
- **Real-Time Streaming** — live SSE output with tool execution progress, thinking process, attack chain visualization
- **Telegram Bot** — chat with CyberStrikeAI via Telegram (long-polling, no public IP required)
- **MCP Protocol** — Model Context Protocol server for external tool integration
- **WebShell Manager** — manage and interact with web shells
- **File Manager** — track, upload, and manage files across conversations
- **Burp Suite Plugin** — send requests from Burp directly to CyberStrikeAI for AI-powered analysis
- **Internationalization** — English + Ukrainian (Українська) UI

---

## Quick Start

### Prerequisites

- **Go 1.21+** — [install](https://go.dev/dl/)
- **Python 3.10+** — for some security tools
- **Security tools** — nmap, subfinder, nuclei, etc. (agent uses what's available)
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

Open `http://localhost:8080` and log in with the generated password (shown in terminal).

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

### Local Models (vLLM/Ollama)

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
  embedding:
    provider: openai
    model: multilingual-e5-small
    base_url: http://127.0.0.1:8102/v1
    api_key: none
```

### Telegram Bot

```yaml
robots:
  telegram:
    enabled: true
    bot_token: "YOUR_BOT_TOKEN"
    allowed_user_ids: [123456789]
```

---

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Web UI (:8080)                         │
│    Chat · Dashboard · Monitor · Roles · Skills · Tasks   │
├─────────────────────────────────────────────────────────┤
│                   Go Backend                              │
│  ┌─────────┐  ┌──────────────┐  ┌────────────────────┐  │
│  │ Single   │  │ Multi-Agent  │  │ Knowledge Base     │  │
│  │ Agent    │  │ Orchestrator │  │ (RAG + Embeddings) │  │
│  │ Loop     │  │ (native Go)  │  │                    │  │
│  └────┬─────┘  └──────┬───────┘  └────────┬───────────┘  │
│       │               │                    │              │
│  ┌────┴───────────────┴────────────────────┴───────────┐ │
│  │              Anthropic / OpenAI Adapter               │ │
│  │    (native Claude Messages API + rate limiting)       │ │
│  └──────────────────────┬────────────────────────────────┘ │
│                         │                                  │
│  ┌──────────────────────┴────────────────────────────────┐ │
│  │           MCP Tool Executor (117+ tools)               │ │
│  │  nmap · nuclei · sqlmap · subfinder · ffuf · masscan   │ │
│  │  metasploit · hydra · gobuster · nikto · feroxbuster   │ │
│  └────────────────────────────────────────────────────────┘ │
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

---

## Scripts

| Script | Purpose |
|--------|---------|
| `./run_suite.sh` | Full dependency check, build, and launch |
| `./run_mcp.sh` | Start local vLLM embedding server for knowledge base |
| `./upgrade.sh` | Upgrade to latest release from GitHub |
| `./deploy.sh` | DUMLmap tool deployment (separate project) |

---

## API

CyberStrikeAI exposes a REST + SSE API:

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/agent-loop/stream` | POST | Single-agent chat (SSE streaming) |
| `/api/multi-agent/stream` | POST | Multi-agent orchestrated task (SSE) |
| `/api/config` | GET/POST | Read/update configuration |
| `/api/config/test-api` | POST | Test API endpoint, discover models & rate limits |
| `/api/conversations` | GET | List conversations |
| `/api/vulnerabilities` | GET | List discovered vulnerabilities |
| `/api/knowledge/search` | POST | Search knowledge base |
| `/api/monitor/stats` | GET | Execution statistics |

Full API docs at `http://localhost:8080` → API Documentation tab.

---

## MCP Servers

Included MCP server implementations:

- **`mcp-servers/reverse_shell/`** — TCP reverse shell handler for pentest scenarios
- **`mcp-servers/pent_claude_agent/`** — Claude Agent SDK wrapper with configurable tools

## Plugins

- **`plugins/burp-suite/`** — Burp Suite extension to send HTTP traffic to CyberStrikeAI for AI analysis

---

## Security Considerations

- **Run in a container** for production use — the agent has unrestricted shell access by design
- **API keys** — use environment variables, not plaintext in config.yaml
- **Network binding** — defaults to `0.0.0.0:8080`, bind to `127.0.0.1` if not behind a reverse proxy
- **Authentication** — all API endpoints require session token (auto-generated password on first run)

---

## Fork History

This is the [cybersecua](https://github.com/cybersecua) fork of [Ed1s0nZ/CyberStrikeAI](https://github.com/Ed1s0nZ/CyberStrikeAI).

Major additions in this fork:
- **Native Anthropic Claude support** — direct Messages API, no proxy
- **Native Go multi-agent orchestrator** — replaced ByteDance Eino framework
- **API health check & model discovery** — auto-detect models and rate limits
- **Telegram bot integration** — long-polling, multi-user, streaming progress
- **Proxy middleware** — global SOCKS5/gsocket routing for tool traffic
- **DroidRun / Cuttlefish** — Android VM control for mobile testing
- **42 custom skills** — drone exploitation, SDR, ELRS, bluetooth, IoT, red team ops
- **Full English codebase** — removed all Chinese, added Ukrainian (uk-UA) locale
- **Removed Chinese dependencies** — no ByteDance/CloudWeGo packages

---

## Contributing

Pull requests welcome. Focus areas:
- New security tool integrations (YAML definitions in `tools/`)
- New skills (SKILL.md files in `skills/`)
- UI improvements
- Documentation

---

## License

MIT License. See [LICENSE](LICENSE).

---

## Credits

- **Original project**: [Ed1s0nZ/CyberStrikeAI](https://github.com/Ed1s0nZ/CyberStrikeAI) — Chinese cybersecurity community
- **Fork maintainer**: [cybersecua](https://github.com/cybersecua) — Ukrainian cybersecurity
- **AI assistance**: Claude Code (Anthropic)

---

![Stargazers over time](https://starchart.cc/cybersecua/CyberStrikeAI.svg)
