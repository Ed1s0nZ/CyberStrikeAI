# CyberStrikeAI — Technical Documentation

> **AI-Native Security Testing Platform**
>
> CyberStrikeAI is an open-source, Go-based platform that combines 100+ security tools with an AI orchestration engine to deliver end-to-end penetration testing, vulnerability management, and attack-chain analysis — all driven by natural-language conversations.

---

## Table of Contents

1. [Overview](#overview)
2. [Architecture](#architecture)
3. [Project Structure](#project-structure)
4. [Core Components](#core-components)
   - [Agent Engine](#agent-engine)
   - [MCP (Model Context Protocol)](#mcp-model-context-protocol)
   - [Security Executor](#security-executor)
   - [Knowledge Base](#knowledge-base)
   - [Persistent Memory](#persistent-memory)
   - [Attack Chain Builder](#attack-chain-builder)
5. [Tool System](#tool-system)
6. [Role System](#role-system)
7. [Skills System](#skills-system)
8. [Web Interface & API](#web-interface--api)
9. [Chatbot Integrations](#chatbot-integrations)
10. [Configuration Reference](#configuration-reference)
11. [Deployment](#deployment)
12. [Extension Guide](#extension-guide)
13. [Security Model](#security-model)

---

## Overview

CyberStrikeAI bridges the gap between human security expertise and automated tooling. A security professional can type natural-language instructions (e.g., *"Scan 192.168.1.0/24 for open ports, then check discovered web services for SQL injection"*), and the platform will:

1. Select and invoke the right tools (nmap, sqlmap, nuclei, etc.)
2. Parse and correlate results across tool outputs
3. Build an attack-chain graph with severity scoring
4. Remember findings in persistent memory for cross-session continuity
5. Retrieve relevant knowledge-base entries to guide exploitation strategy
6. Track vulnerabilities with full lifecycle management

### Key Statistics

| Metric | Value |
|--------|-------|
| Language | Go 1.23+ (~42,000 LOC) |
| Security tools | 116 YAML recipes |
| Predefined roles | 13 (Pentest, CTF, Cloud, etc.) |
| Predefined skills | 23 (SQLi, XSS, SSRF, etc.) |
| Database | SQLite (conversations + knowledge) |
| LLM support | Any OpenAI-compatible API |
| Transports | HTTP, stdio, SSE (MCP) |

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Client Layer                             │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌───────────────┐   │
│  │  Web UI  │  │ Telegram │  │   Lark   │  │ MCP Clients   │   │
│  │ (SPA)    │  │   Bot    │  │   Bot    │  │ (Cursor/IDE)  │   │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └───────┬───────┘   │
│       │              │              │                │           │
└───────┼──────────────┼──────────────┼────────────────┼───────────┘
        │              │              │                │
┌───────▼──────────────▼──────────────▼────────────────▼───────────┐
│                      HTTP / WebSocket / SSE                      │
│                     Gin Router + Auth Middleware                  │
└───────┬──────────────────────────────────────────────────────────┘
        │
┌───────▼──────────────────────────────────────────────────────────┐
│                        Handler Layer                             │
│  Agent │ Conversation │ Auth │ Config │ Monitor │ Vulnerability  │
│  Role  │ Knowledge    │ MCP  │ Docker │ Skills  │ BatchTask      │
└───────┬──────────────────────────────────────────────────────────┘
        │
┌───────▼──────────────────────────────────────────────────────────┐
│                      Core Services                               │
│  ┌─────────────┐  ┌──────────────┐  ┌─────────────────────────┐ │
│  │ Agent Engine │  │ MCP Server   │  │ Security Executor       │ │
│  │  - LLM loop │  │  - HTTP      │  │  - Tool runner          │ │
│  │  - Memory   │  │  - stdio     │  │  - Sandbox/timeout      │ │
│  │  - RAG      │  │  - SSE       │  │  - Result storage       │ │
│  │  - Time     │  │  - External  │  │  - Large-result paging  │ │
│  └──────┬──────┘  └──────┬───────┘  └────────────┬────────────┘ │
│         │                │                        │              │
│  ┌──────▼──────┐  ┌──────▼───────┐  ┌────────────▼────────────┐ │
│  │ Knowledge   │  │ Attack Chain │  │ Skills Manager          │ │
│  │  - Embedder │  │  - Builder   │  │  - SKILL.md loader      │ │
│  │  - BM25     │  │  - Scoring   │  │  - On-demand retrieval  │ │
│  │  - Retriever│  │  - Graph     │  │                         │ │
│  └──────┬──────┘  └──────────────┘  └─────────────────────────┘ │
└─────────┼────────────────────────────────────────────────────────┘
          │
┌─────────▼────────────────────────────────────────────────────────┐
│                       Data Layer                                 │
│  ┌──────────────────┐  ┌──────────────────┐  ┌───────────────┐  │
│  │ SQLite (main)    │  │ SQLite (knowledge)│  │ File Storage  │  │
│  │  - Conversations │  │  - Embeddings     │  │  - tmp/       │  │
│  │  - Memories      │  │  - BM25 index     │  │  - Artifacts  │  │
│  │  - Vulns         │  │  - Categories     │  │  - Logs       │  │
│  │  - Attack chains │  │                   │  │               │  │
│  └──────────────────┘  └───────────────────┘  └───────────────┘  │
└──────────────────────────────────────────────────────────────────┘
```

### Request Flow (Conversation)

1. **User sends a message** via Web UI, Telegram, Lark, or MCP client
2. **Auth middleware** validates the Bearer token / session
3. **Agent handler** receives the message and prepares context:
   - Injects time awareness block
   - Loads persistent memory entries into system prompt
   - Runs memory similarity check against the user query
   - Pre-fetches relevant knowledge-base entries (RAG preflight)
   - Applies role-specific system prompt and tool restrictions
4. **Agent engine** enters the LLM loop:
   - Sends context + available tools to the OpenAI-compatible API
   - Receives tool calls from the model
   - Dispatches tool calls to the Security Executor (parallel or sequential)
   - Feeds results back to the model
   - Repeats until the model produces a final answer or `max_iterations` is reached
5. **Results** are streamed to the client via SSE, stored in SQLite, and optionally used to update the attack chain graph

---

## Project Structure

```
CyberStrikeAI/
├── cmd/
│   ├── server/              # Main HTTP server entrypoint
│   ├── mcp-stdio/           # MCP stdio transport for IDE integration
│   ├── test-config/         # Config validation utility
│   ├── test-external-mcp/   # External MCP client test
│   └── test-sse-mcp-server/ # SSE MCP server test fixture
│
├── internal/
│   ├── agent/               # AI agent engine (LLM loop, memory, RAG, time)
│   │   ├── agent.go         # Core agent with iterative tool-calling loop
│   │   ├── memory_compressor.go  # Context window compression
│   │   ├── persistent_memory.go  # Cross-session key-value memory (SQLite)
│   │   ├── rag_context.go   # Proactive RAG injection before agent turns
│   │   └── time_awareness.go    # Date/time context injector
│   ├── app/                 # Application bootstrap and route wiring
│   ├── attackchain/         # Attack-chain graph builder and scorer
│   ├── config/              # YAML config loader with defaults and validation
│   ├── database/            # SQLite DAL (conversations, vulns, memories, etc.)
│   ├── filemanager/         # Cross-conversation file tracking
│   ├── handler/             # HTTP handlers (one file per domain)
│   ├── knowledge/           # Knowledge base engine
│   │   ├── embedder.go      # Vector embedding via OpenAI-compatible API
│   │   ├── bm25.go          # Corpus-level BM25 Okapi scoring
│   │   ├── retriever.go     # Hybrid (vector + BM25) retrieval
│   │   ├── indexer.go       # Markdown file scanner and chunk indexer
│   │   ├── manager.go       # CRUD operations for knowledge items
│   │   └── tool.go          # MCP tool wrappers (search_knowledge_base, etc.)
│   ├── logger/              # Structured logging (zap)
│   ├── mcp/                 # MCP protocol implementation
│   │   ├── server.go        # HTTP MCP server with tool registration
│   │   ├── client_sdk.go    # Go SDK MCP client wrapper
│   │   ├── external_manager.go  # External MCP federation (HTTP/stdio/SSE)
│   │   └── builtin/         # Built-in tool constants
│   ├── openai/              # OpenAI-compatible API client (chat + embeddings)
│   ├── robot/               # Chatbot connectors (Telegram, Lark)
│   ├── security/            # Tool executor, auth manager, auth middleware
│   ├── skills/              # Skills manager and MCP tool wrappers
│   └── storage/             # Large-result file storage with pagination
│
├── web/
│   ├── templates/           # HTML templates (single-page application)
│   └── static/              # CSS, JS, and static assets
│
├── tools/                   # 116 YAML tool recipes
├── roles/                   # 13 predefined role YAML files
├── skills/                  # 23 predefined skill directories (each with SKILL.md)
├── knowledge_base/          # Markdown knowledge files by category
│   ├── Bitrix/
│   ├── PHP/
│   ├── SQL Injection/
│   ├── SSL-TLS MITM/
│   ├── Prompt Injection/
│   └── Tools/               # Per-tool usage guides
│
├── scripts/                 # Installation and utility scripts
├── docs/                    # Supplementary documentation
├── config.yaml.example      # Annotated configuration template
├── Dockerfile               # Multi-stage Docker build
├── docker-compose.yml        # Docker Compose stack
├── run.sh                   # One-command launcher (Go + Python setup)
└── run_docker.sh            # Docker lifecycle management script
```

---

## Core Components

### Agent Engine

**Location:** `internal/agent/`

The agent engine is the brain of CyberStrikeAI. It implements an iterative tool-calling loop powered by any OpenAI-compatible LLM.

#### Agent Struct (`agent.go`)

```go
type Agent struct {
    openAIClient          *openai.Client       // Primary LLM client
    toolOpenAIClient      *openai.Client       // Optional separate client for tool iterations
    persistentMemory      *PersistentMemory    // Cross-session memory store
    timeAwareness         *TimeAwareness       // Time context injector
    ragInjector           *RAGContextInjector  // Knowledge-base preflight
    mcpServer             *mcp.Server          // Built-in MCP tools
    externalMCPMgr        *mcp.ExternalMCPManager // Federated external MCPs
    resultStorage         ResultStorage        // Large-result file storage
    // ...
}
```

#### Execution Loop

1. Collect available tools from MCP server + external MCP federation + built-in tools
2. Build system prompt with: role prompt + time context + memory entries + RAG context
3. Call LLM with message history and tool definitions
4. If LLM returns tool calls:
   - Execute tools in parallel (configurable) via Security Executor
   - Handle large results (>threshold) by storing to disk and returning a reference
   - Auto-persist tool results as memory entries
   - Feed results back to LLM
5. Repeat until final text response or `max_iterations` reached

#### Memory Compressor (`memory_compressor.go`)

When the conversation context approaches the token limit (`max_total_tokens`), the memory compressor summarizes older messages using a separate LLM call, preserving key findings while freeing context space.

#### Persistent Memory (`persistent_memory.go`)

A SQLite-backed key-value store with five categories:

| Category | Examples |
|----------|----------|
| `credential` | Discovered passwords, API keys, tokens |
| `target` | IPs, domains, ports, service versions |
| `vulnerability` | CVEs, injection points, exploit details |
| `fact` | Environment observations, OS versions |
| `note` | Operational reminders, engagement scope |

Memories survive conversation compression and server restarts. They are automatically injected into the system prompt and can be queried by semantic similarity.

#### RAG Context Injector (`rag_context.go`)

Before each agent turn, the RAG injector:
- Searches persistent memory for entries semantically similar to the current query
- Extracts entities (IPs, domains) from the query for targeted memory lookup
- Queries the knowledge base with penetration-testing context
- Injects retrieved context as a `<memory_similarity_context>` block

#### Time Awareness (`time_awareness.go`)

Prepends a `<time_context>` block to every system prompt with the current date/time, timezone, Unix timestamp, and session age. Exposes a `get_current_time` tool for on-demand queries.

---

### MCP (Model Context Protocol)

**Location:** `internal/mcp/`

CyberStrikeAI natively implements the Model Context Protocol for tool registration and invocation.

#### Transport Modes

| Mode | Use Case | Endpoint |
|------|----------|----------|
| **HTTP** | Web UI, external clients | `http://<host>:<port>/mcp` |
| **stdio** | IDE integration (Cursor, Claude Code) | `cmd/mcp-stdio/main.go` |
| **SSE** | Real-time streaming | Server-Sent Events endpoint |

#### External MCP Federation (`external_manager.go`)

CyberStrikeAI can connect to third-party MCP servers and aggregate their tools alongside built-in ones. Supported transports: HTTP, stdio, and SSE. Configuration is managed via the Web UI or `config.yaml`.

#### Built-in MCP Tools

The platform registers several built-in tools via MCP:

- **Security tools** — 116 tools from `tools/*.yaml` (nmap, sqlmap, nuclei, etc.)
- **Memory tools** — `store_memory`, `retrieve_memory`, `list_memories`, `delete_memory`
- **Knowledge tools** — `search_knowledge_base`, `list_knowledge_risk_types`
- **Skills tools** — `list_skills`, `read_skill`
- **File tools** — `create_file`, `modify_file`, `delete_file`, `list_files`
- **System tools** — `get_current_time`, `query_execution_result`, `record_vulnerability`

---

### Security Executor

**Location:** `internal/security/`

The Security Executor is responsible for safely running security tools as OS processes.

- **Sandboxing** — Each tool invocation runs in a subprocess with configurable timeouts
- **Argument validation** — Parameters are validated against the YAML tool definition before execution
- **Result handling** — Outputs exceeding `large_result_threshold` bytes are stored to disk and exposed via the `query_execution_result` tool with pagination, keyword search, and regex filtering
- **Parallel execution** — Multiple tool calls can execute concurrently (`parallel_tool_execution: true`, with optional `max_parallel_tools` limit)
- **Retry logic** — Configurable `tool_retry_count` for transient failures

---

### Knowledge Base

**Location:** `internal/knowledge/`

A hybrid retrieval system combining vector similarity search with corpus-level BM25 keyword scoring.

#### Components

| Component | File | Purpose |
|-----------|------|---------|
| **Embedder** | `embedder.go` | Generates vector embeddings via OpenAI-compatible API |
| **BM25** | `bm25.go` | Corpus-level BM25 Okapi scoring with real IDF |
| **Retriever** | `retriever.go` | Hybrid search: blends vector and BM25 scores |
| **Indexer** | `indexer.go` | Scans Markdown files, chunks text, builds index |
| **Manager** | `manager.go` | CRUD for knowledge items via Web UI |
| **Tool** | `tool.go` | MCP tool wrappers for agent access |

#### Retrieval Pipeline

1. User query arrives (explicitly or via RAG preflight)
2. Query is embedded using the configured embedding model
3. Vector similarity search returns top-K candidates from SQLite
4. BM25 scoring runs against the full corpus index
5. Scores are blended: `final = hybrid_weight × vector + (1 - hybrid_weight) × bm25`
6. Results above `similarity_threshold` are returned

#### Knowledge Base Structure

```
knowledge_base/
├── Bitrix/           # Bitrix CMS exploitation guides
├── PHP/              # PHP security patterns
├── SQL Injection/    # SQLi techniques and payloads
├── SSL-TLS MITM/     # MITM attack knowledge
├── Prompt Injection/ # LLM prompt injection techniques
└── Tools/            # Per-tool usage guides (nmap.md, nuclei.md, etc.)
```

---

### Persistent Memory

**Location:** `internal/agent/persistent_memory.go`, `internal/database/`

The persistent memory system provides cross-session state that survives both context compression and server restarts.

#### How It Works

1. **Storage**: Memories are stored in the main SQLite database as key-value pairs with a category tag
2. **Injection**: On every agent turn, all memories are formatted and injected into the system prompt, grouped by category
3. **Agent tools**: The AI agent can create, query, and delete memories using four built-in tools
4. **Auto-persistence**: Tool execution results are automatically stored as memory entries
5. **Similarity search**: The RAG injector uses semantic similarity to surface relevant memories before each turn

#### System Prompt Injection Format

```
[CREDENTIALS]
  • admin_password: P@ssw0rd123
[TARGETS]
  • main_target: 192.168.1.100 (Apache 2.4, port 80/443)
[VULNERABILITIES]
  • sqli_endpoint: /login.php?id= is injectable (union-based)
[FACTS]
  • os_version: Ubuntu 22.04 LTS
[NOTES]
  • scope: Testing authorized for 192.168.1.0/24 only
```

---

### Attack Chain Builder

**Location:** `internal/attackchain/`

The attack chain builder parses conversation history to assemble a graph of targets, tools, vulnerabilities, and their relationships.

- **Automatic construction** — The AI analyzes each conversation to extract attack steps
- **Severity scoring** — Each node and edge is scored based on discovered vulnerabilities
- **Visualization** — The Web UI renders the chain as an interactive graph
- **Step replay** — Users can replay individual steps in the attack chain
- **Export** — Chain data can be exported for external reporting

---

## Tool System

**Location:** `tools/`

CyberStrikeAI ships with 116 YAML-defined tool recipes covering the complete security testing kill chain.

### Tool Categories

| Category | Examples | Count |
|----------|----------|-------|
| Network Scanners | nmap, masscan, rustscan, arp-scan | 5 |
| Web Scanners | sqlmap, nikto, dirb, gobuster, ffuf, feroxbuster | 12 |
| Vulnerability Scanners | nuclei, wpscan, wafw00f, dalfox, xsser | 6 |
| Subdomain Enumeration | subfinder, amass, dnsenum, fierce | 5 |
| API Security | graphql-scanner, arjun, api-fuzzer, api-schema-analyzer | 4 |
| Container Security | trivy, clair, docker-bench-security, kube-bench | 5 |
| Cloud Security | prowler, scout-suite, cloudmapper, pacu, checkov | 6 |
| Binary Analysis | gdb, radare2, ghidra, objdump, strings, binwalk | 8 |
| Exploitation | metasploit, msfvenom, ropper, ropgadget | 5 |
| Password Cracking | hashcat, john, hashpump, hydra | 4 |
| Forensics | volatility, volatility3, foremost, exiftool | 5 |
| Post-Exploitation | linpeas, winpeas, mimikatz, impacket, responder | 6 |
| CTF Utilities | stegsolve, zsteg, fcrackzip, pdfcrack, cyberchef | 7 |
| System Helpers | exec, create-file, delete-file, modify-file, cat | 6 |
| Search Engines | fofa_search, zoomeye_search | 2 |

### YAML Tool Definition Format

```yaml
name: "nmap"                          # Unique tool identifier
command: "nmap"                       # System command to execute
args: ["-sT", "-sV", "-sC"]          # Default arguments
enabled: true                        # Enable/disable toggle
short_description: "Network mapping"  # Brief description (token-efficient)
description: |                        # Full description for AI context
  Nmap is a network scanner used for discovering hosts and services...
notes: |                              # Additional AI guidance
  Always use -sV for version detection. Combine with -sC for default scripts.
parameters:
  - name: "target"                    # Parameter name
    type: "string"                    # Type: string, integer, boolean
    description: "IP address or domain to scan"
    required: true                    # Required vs optional
    position: 0                       # Positional argument index
  - name: "ports"
    type: "string"
    flag: "-p"                        # CLI flag prefix
    description: "Port range (e.g., 1-1000, 80,443)"
```

### Adding a Custom Tool

1. Create a new `.yaml` file in `tools/` (e.g., `tools/my-tool.yaml`)
2. Define `name`, `command`, `args`, `description`, and `parameters`
3. Restart the server or reload configuration
4. The tool appears in the Settings panel and is available to the AI agent

---

## Role System

**Location:** `roles/`

Roles customize the AI agent's behavior, system prompt, and available tools for specific security testing scenarios.

### Predefined Roles (13)

| Role | Description |
|------|-------------|
| Default | General-purpose security testing |
| Penetration Testing | Comprehensive pentest methodology |
| CTF | Capture-the-flag problem solving |
| Web Application Scanning | Web vulnerability assessment |
| API Security Testing | API-focused security testing |
| Binary Analysis | Reverse engineering and binary exploitation |
| Cloud Security Audit | Cloud infrastructure assessment |
| Container Security | Docker/Kubernetes security testing |
| Comprehensive Vulnerability Scan | Full-spectrum vulnerability scanning |
| Digital Forensics | Digital forensics and incident response |
| Information Gathering | Reconnaissance and OSINT |
| Post-Exploitation Testing | Post-compromise assessment |
| Web Framework Testing | Framework-specific vulnerability testing |

### Role YAML Format

```yaml
name: Penetration Testing
description: Professional penetration testing expert
user_prompt: |
  You are a professional cybersecurity penetration testing expert.
  Use professional methods and tools for comprehensive security testing...
icon: "\U0001F3AF"
tools:                    # Restrict available tools (empty = all tools)
  - nmap
  - sqlmap
  - nuclei
  - metasploit
skills:                   # Attach skills as hints
  - sql-injection-testing
  - xss-testing
enabled: true
```

---

## Skills System

**Location:** `skills/`

Skills are detailed, structured testing methodologies that the AI agent can load on demand.

### Predefined Skills (23)

SQL injection, XSS, SSRF, CSRF, XXE, command injection, file upload, IDOR, LDAP injection, XPath injection, deserialization, API security, container security, cloud security, network penetration testing, secure code review, incident response, vulnerability assessment, business logic testing, security automation, security awareness training, mobile app security, and Bitrix24 webhook exploitation.

### Skill Structure

```
skills/
└── sql-injection-testing/
    └── SKILL.md          # Detailed testing methodology
```

Each `SKILL.md` contains:
- Testing methodology and approach
- Tool usage examples with specific commands
- Common payloads and bypass techniques
- Best practices and remediation guidance
- YAML front matter for metadata

### How Skills Work

1. When a role is selected, attached skill names are added to the system prompt as hints
2. The AI agent can call `list_skills` to see all available skills
3. The AI agent calls `read_skill` to load the full content of a skill when needed
4. Skill content is NOT automatically injected — it is retrieved on demand to save context space

---

## Web Interface & API

### Web UI Features

- **Chat console** — Natural-language conversation with streaming SSE output
- **Role selector** — Dropdown to switch security testing roles
- **Task monitor** — Inspect running tool jobs, execution logs, and artifacts
- **Attack chain graph** — Interactive visualization of attack paths
- **Vulnerability management** — Create, track, and filter vulnerabilities
- **Batch task manager** — Queue and execute multiple tasks sequentially
- **Knowledge base** — Browse, create, and manage knowledge items
- **Memory panel** — View, search, edit, and delete persistent memories
- **Conversation groups** — Organize, pin, rename, and batch-manage conversations
- **Settings** — Configure API keys, MCP, tools, Docker, and chatbots
- **File manager** — Track and manage files across conversations

### REST API Endpoints

| Domain | Endpoints |
|--------|-----------|
| **Auth** | `POST /api/auth/login`, `POST /api/auth/change-password` |
| **Conversations** | `GET/POST/DELETE /api/conversations`, `GET /api/conversations/:id/messages` |
| **Agent** | `POST /api/agent/chat` (SSE streaming) |
| **Roles** | `GET/POST /api/roles`, `GET/PUT/DELETE /api/roles/:name` |
| **Vulnerabilities** | `GET/POST /api/vulnerabilities`, `GET/PUT/DELETE /api/vulnerabilities/:id`, `GET /api/vulnerabilities/stats` |
| **Batch Tasks** | `GET/POST /api/batch-tasks`, `POST /api/batch-tasks/:id/start`, `POST /api/batch-tasks/:id/cancel` |
| **Knowledge** | `GET/POST /api/knowledge`, `POST /api/knowledge/scan`, `POST /api/knowledge/search` |
| **Memory** | `GET /api/memory`, `DELETE /api/memory/:id` |
| **Monitor** | `GET /api/monitor/tools`, `GET /api/monitor/executions` |
| **Docker** | `GET /api/docker/status`, `GET /api/docker/logs`, `POST /api/docker/action` |
| **Config** | `GET/POST /api/config` |
| **MCP** | `POST /mcp` (MCP protocol endpoint) |

All endpoints (except login) require Bearer token authentication.

---

## Chatbot Integrations

### Telegram Bot

- Long-polling connection (no public IP required)
- Multi-user support with independent sessions per user ID
- User whitelist via `allowed_user_ids`
- Live progress streaming (message editing during tool execution)
- Role switching via `role <name>` command
- Group chat support with @mention detection

### Lark (Feishu) Bot

- Persistent long-lived WebSocket connection
- Message-based interaction with the AI agent
- Configurable via Web UI or `config.yaml`

Configuration is managed in the `robots` section of `config.yaml` or via the System Settings UI.

---

## Configuration Reference

### Key Configuration Sections

| Section | Purpose |
|---------|---------|
| `server` | HTTP host and port (default: `0.0.0.0:8080`) |
| `auth` | Password and session duration |
| `openai` | LLM API credentials, model selection, token limits |
| `openai.tool_*` | Optional separate model/endpoint for tool iterations |
| `openai.summary_*` | Optional separate model for context summarization |
| `agent` | Iteration limits, parallelism, result storage |
| `agent.time_awareness` | Time context injection and timezone |
| `agent.memory` | Persistent memory toggle and entry cap |
| `mcp` | MCP server enable/host/port |
| `external_mcp` | Federated external MCP servers |
| `security` | Tool directory and description mode |
| `database` | SQLite paths for main and knowledge databases |
| `knowledge` | Knowledge base, embedding config, retrieval tuning |
| `fofa` | FOFA search engine API credentials |
| `robots` | Telegram, Lark, WeCom bot configuration |
| `roles_dir` | Path to role YAML files (default: `roles/`) |
| `skills_dir` | Path to skill directories (default: `skills/`) |

### Multi-Model Support

CyberStrikeAI supports routing different tasks to different LLM endpoints:

| Config Key | Purpose |
|------------|---------|
| `openai.model` | Primary model for conversation |
| `openai.tool_model` | Model for tool-calling iterations (e.g., faster/cheaper) |
| `openai.summary_model` | Model for context summarization |

Each can have its own `base_url` and `api_key`, allowing mixed-provider setups (e.g., Claude for reasoning, DeepSeek for tool calls).

---

## Deployment

### Local Development

```bash
# Prerequisites: Go 1.21+, Python 3.10+
git clone https://github.com/cybersecua/CyberStrikeAI.git
cd CyberStrikeAI
chmod +x run.sh && ./run.sh
```

The `run.sh` script handles Go/Python setup, dependency installation, building, and launching.

### Docker

```bash
# Using docker-compose
docker-compose up -d

# Using the lifecycle script
./run_docker.sh deploy
./run_docker.sh status
./run_docker.sh logs
```

The Dockerfile uses a multi-stage build:
1. **Builder stage** — Compiles the Go binary with CGO enabled (for SQLite)
2. **Runtime stage** — Minimal Debian image with the binary, web assets, tools, and scripts

Exposed ports: `8080` (HTTP), `8081` (MCP)

### Docker Lifecycle Management

The `run_docker.sh` script and Docker API provide full lifecycle control:

| Action | CLI | API |
|--------|-----|-----|
| Deploy | `./run_docker.sh deploy` | `POST /api/docker/action {"action":"deploy"}` |
| Update | `./run_docker.sh update` | `POST /api/docker/action {"action":"update"}` |
| Start/Stop | `./run_docker.sh start/stop` | `POST /api/docker/action {"action":"start/stop"}` |
| Logs | `./run_docker.sh logs` | `GET /api/docker/logs?lines=200` |
| Status | `./run_docker.sh status` | `GET /api/docker/status` |
| Remove | `./run_docker.sh remove` | `POST /api/docker/action {"action":"remove"}` |

Proxy modes (SOCKS, HTTP, Tor) and VPN container integration are supported for network-restricted environments.

---

## Extension Guide

### Adding a New Security Tool

1. Create `tools/my-tool.yaml` with the tool definition
2. Ensure the tool binary is installed on the system (or in the Docker image)
3. Restart or reload — the tool is immediately available

### Creating a Custom Role

1. Create `roles/my-role.yaml` with name, description, user_prompt, tools, and skills
2. Restart or reload — the role appears in the Web UI dropdown

### Adding a Knowledge Base Entry

1. Create a Markdown file in `knowledge_base/<Category>/my-topic.md`
2. Use the Web UI to scan and index the knowledge base
3. The AI agent can now retrieve this knowledge via `search_knowledge_base`

### Creating a Custom Skill

1. Create a directory `skills/my-skill/`
2. Add a `SKILL.md` file with the testing methodology
3. Attach the skill name to roles in their YAML configuration

### Connecting an External MCP Server

1. Navigate to Settings → External MCP in the Web UI
2. Add the MCP server configuration (HTTP, stdio, or SSE)
3. Start the connection — external tools are federated into the agent's tool set

---

## Security Model

### Authentication

- **Password-based auth** with Bearer token sessions
- **Auto-generated strong passwords** when none is configured
- **Configurable session duration** (`session_duration_hours`)
- **Password rotation** via `/api/auth/change-password`

### Authorization

- All API endpoints require valid authentication (except `/api/auth/login`)
- Role-based tool restrictions limit which tools are available per engagement
- User whitelist for Telegram bot access (`allowed_user_ids`)

### Tool Execution Safety

- Each tool runs in a subprocess with configurable timeouts
- Parameters are validated against YAML definitions before execution
- Large outputs are stored to disk rather than kept in memory
- Structured logging for audit trails

### Data Protection

- All data stored locally in SQLite (no external data transmission beyond configured APIs)
- Knowledge base and conversation databases can be separated
- Conversation history is fully auditable with replay capability

---

## Dependencies

### Go Dependencies (Key)

| Package | Purpose |
|---------|---------|
| `gin-gonic/gin` | HTTP router and middleware |
| `gorilla/websocket` | WebSocket for terminal streaming |
| `mattn/go-sqlite3` | SQLite database driver (CGO) |
| `modelcontextprotocol/go-sdk` | MCP protocol implementation |
| `pkoukk/tiktoken-go` | Token counting for context management |
| `uber-go/zap` | Structured logging |
| `larksuite/oapi-sdk-go` | Lark (Feishu) bot SDK |
| `creack/pty` | Pseudo-terminal for tool execution |
| `google/uuid` | UUID generation |

### Python Dependencies

A Python virtual environment is created by `run.sh` for tools that require Python (API fuzzer, HTTP framework tester, etc.).

---

## License & Disclaimer

CyberStrikeAI is designed for **authorized security testing only**. Users must have explicit written authorization before testing any target system. The developers are not responsible for misuse. See the [Disclaimer](README.md#%EF%B8%8F-disclaimer) in the main README.

---

*This documentation reflects the CyberStrikeAI codebase as of v1.4.0. For the development roadmap and planned features, see [ROADMAP.md](ROADMAP.md).*
