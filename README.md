<div align="center">
  <img src="web/static/logo.png" alt="CyberStrikeAI Logo" width="300">
</div>

# CyberStrikeAI


[中文](README_CN.md) | [English](README.md)

CyberStrikeAI is an **AI-native security testing platform** built in Go. It integrates 100+ security tools, an intelligent orchestration engine, and comprehensive lifecycle management capabilities. Through native MCP protocol and AI agents, it enables end-to-end automation from conversational commands to vulnerability discovery, attack-chain analysis, knowledge retrieval, and result visualization—delivering an auditable, traceable, and collaborative testing environment for security teams.


## Interface & Integration Preview

### Web Console
<img src="./img/效果.png" alt="Web Console" width="560">

### MCP Integration
- **MCP stdio mode**  
  <img src="./img/mcp-stdio2.png" alt="MCP stdio mode" width="560">
- **MCP management**  
  <img src="./img/MCP管理.png" alt="MCP management" width="560">

### Attack Chain Visualization
<img src="./img/攻击链.png" alt="Attack Chain" width="560">

### Vulnerability Management
<img src="./img/漏洞管理.png" alt="Vulnerability Management" width="560">

### Task Management
<img src="./img/任务.png" alt="Task Management" width="560">

### Role Management
<img src="./img/角色管理.png" alt="Role Management" width="560">

## Highlights

- 🤖 AI decision engine with OpenAI-compatible models (GPT, Claude, DeepSeek, etc.)
- 🔌 Native MCP implementation with HTTP/stdio/SSE transports and external MCP federation
- 🧰 100+ prebuilt tool recipes + YAML-based extension system
- 📄 Large-result pagination, compression, and searchable archives
- 🔗 Attack-chain graph, risk scoring, and step-by-step replay
- 🔒 Password-protected web UI, audit logs, and SQLite persistence
- 📚 Knowledge base with vector search and hybrid retrieval for security expertise
- 📁 Conversation grouping with pinning, rename, and batch management
- 🛡️ Vulnerability management with CRUD operations, severity tracking, status workflow, and statistics
- 📋 Batch task management: create task queues, add multiple tasks, and execute them sequentially
- 🎭 Role-based testing: predefined security testing roles (Penetration Testing, CTF, Web App Scanning, etc.) with custom prompts and tool restrictions

## Tool Overview

CyberStrikeAI ships with 100+ curated tools covering the whole kill chain:

- **Network Scanners** – nmap, masscan, rustscan, arp-scan, nbtscan
- **Web & App Scanners** – sqlmap, nikto, dirb, gobuster, feroxbuster, ffuf, httpx
- **Vulnerability Scanners** – nuclei, wpscan, wafw00f, dalfox, xsser
- **Subdomain Enumeration** – subfinder, amass, findomain, dnsenum, fierce
- **Network Space Search Engines** – fofa_search, zoomeye_search
- **API Security** – graphql-scanner, arjun, api-fuzzer, api-schema-analyzer
- **Container Security** – trivy, clair, docker-bench-security, kube-bench, kube-hunter
- **Cloud Security** – prowler, scout-suite, cloudmapper, pacu, terrascan, checkov
- **Binary Analysis** – gdb, radare2, ghidra, objdump, strings, binwalk
- **Exploitation** – metasploit, msfvenom, pwntools, ropper, ropgadget
- **Password Cracking** – hashcat, john, hashpump
- **Forensics** – volatility, volatility3, foremost, steghide, exiftool
- **Post-Exploitation** – linpeas, winpeas, mimikatz, bloodhound, impacket, responder
- **CTF Utilities** – stegsolve, zsteg, hash-identifier, fcrackzip, pdfcrack, cyberchef
- **System Helpers** – exec, create-file, delete-file, list-files, modify-file

## Basic Usage

### Quick Start (One-Command Deployment)

**Prerequisites:**
- Go 1.21+ ([Install](https://go.dev/dl/))
- Python 3.10+ ([Install](https://www.python.org/downloads/))

**One-Command Deployment:**
```bash
git clone https://github.com/Ed1s0nZ/CyberStrikeAI.git
cd CyberStrikeAI-main
chmod +x run.sh && ./run.sh
```

The `run.sh` script will automatically:
- ✅ Check and validate Go & Python environments
- ✅ Create Python virtual environment
- ✅ Install Python dependencies
- ✅ Download Go dependencies
- ✅ Build the project
- ✅ Start the server

**First-Time Configuration:**
1. **Configure OpenAI-compatible API** (required before first use)
   - Open http://localhost:8080 after launch
   - Go to `Settings` → Fill in your API credentials:
     ```yaml
     openai:
       api_key: "sk-your-key"
       base_url: "https://api.openai.com/v1"  # or https://api.deepseek.com/v1
       model: "gpt-4o"  # or deepseek-chat, claude-3-opus, etc.
     ```
   - Or edit `config.yaml` directly before launching
2. **Login** - Use the auto-generated password shown in the console (or set `auth.password` in `config.yaml`)
3. **Install security tools (optional)** - Install tools as needed:
   ```bash
   # macOS
   brew install nmap sqlmap nuclei httpx gobuster feroxbuster subfinder amass
   # Ubuntu/Debian
   sudo apt-get install nmap sqlmap nuclei httpx gobuster feroxbuster
   ```
   AI automatically falls back to alternatives when a tool is missing.

**Alternative Launch Methods:**
```bash
# Direct Go run (requires manual setup)
go run cmd/server/main.go

# Manual build
go build -o cyberstrike-ai cmd/server/main.go
./cyberstrike-ai
```

**Note:** The Python virtual environment (`venv/`) is automatically created and managed by `run.sh`. Tools that require Python (like `api-fuzzer`, `http-framework-test`, etc.) will automatically use this environment.

### Core Workflows
- **Conversation testing** – Natural-language prompts trigger toolchains with streaming SSE output.
- **Role-based testing** – Select from predefined security testing roles (Penetration Testing, CTF, Web App Scanning, API Security Testing, etc.) to customize AI behavior and tool availability. Each role applies custom system prompts and can restrict available tools for focused testing scenarios.
- **Tool monitor** – Inspect running jobs, execution logs, and large-result attachments.
- **History & audit** – Every conversation and tool invocation is stored in SQLite with replay.
- **Conversation groups** – Organize conversations into groups, pin important groups, rename or delete groups via context menu.
- **Vulnerability management** – Create, update, and track vulnerabilities discovered during testing. Filter by severity (critical/high/medium/low/info), status (open/confirmed/fixed/false_positive), and conversation. View statistics and export findings.
- **Batch task management** – Create task queues with multiple tasks, add or edit tasks before execution, and run them sequentially. Each task executes as a separate conversation, with status tracking (pending/running/completed/failed/cancelled) and full execution history.
- **Settings** – Tweak provider keys, MCP enablement, tool toggles, and agent iteration limits.

### Built-in Safeguards
- Required-field validation prevents accidental blank API credentials.
- Auto-generated strong passwords when `auth.password` is empty.
- Unified auth middleware for every web/API call (Bearer token flow).
- Timeout and sandbox guards per tool, plus structured logging for triage.

## Advanced Usage

### Role-Based Testing
- **Predefined roles** – System includes 12+ predefined security testing roles (Penetration Testing, CTF, Web App Scanning, API Security Testing, Binary Analysis, Cloud Security Audit, etc.) in the `roles/` directory.
- **Custom prompts** – Each role can define a `user_prompt` that prepends to user messages, guiding the AI to adopt specialized testing methodologies and focus areas.
- **Tool restrictions** – Roles can specify a `tools` list to limit available tools, ensuring focused testing workflows (e.g., CTF role restricts to CTF-specific utilities).
- **Easy role creation** – Create custom roles by adding YAML files to the `roles/` directory. Each role defines `name`, `description`, `user_prompt`, `icon`, `tools`, and `enabled` fields.
- **Web UI integration** – Select roles from a dropdown in the chat interface. Role selection affects both AI behavior and available tool suggestions.

**Creating a custom role (example):**
1. Create a YAML file in `roles/` (e.g., `roles/custom-role.yaml`):
   ```yaml
   name: Custom Role
   description: Specialized testing scenario
   user_prompt: You are a specialized security tester focusing on API security...
   icon: "\U0001F4E1"
   tools:
     - api-fuzzer
     - arjun
     - graphql-scanner
   enabled: true
   ```
2. Restart the server or reload configuration; the role appears in the role selector dropdown.

### Tool Orchestration & Extensions
- **YAML recipes** in `tools/*.yaml` describe commands, arguments, prompts, and metadata.
- **Directory hot-reload** – pointing `security.tools_dir` to a folder is usually enough; inline definitions in `config.yaml` remain supported for quick experiments.
- **Large-result pagination** – outputs beyond 200 KB are stored as artifacts retrievable through the `query_execution_result` tool with paging, filters, and regex search.
- **Result compression** – multi-megabyte logs can be summarized or losslessly compressed before persisting to keep SQLite lean.

**Creating a custom tool (typical flow)**
1. Copy an existing YAML file from `tools/` (for example `tools/sample.yaml`).
2. Update `name`, `command`, `args`, and `short_description`.
3. Describe positional or flag parameters in `parameters[]` so the agent knows how to build CLI arguments.
4. Provide a longer `description`/`notes` block if the agent needs extra context or post-processing tips.
5. Restart the server or reload configuration; the new tool becomes available immediately and can be enabled/disabled from the Settings panel.

### Attack-Chain Intelligence
- AI parses each conversation to assemble targets, tools, vulnerabilities, and relationships.
- The web UI renders the chain as an interactive graph with severity scoring and step replay.
- Export the chain or raw findings to external reporting pipelines.

### MCP Everywhere
- **Web mode** – ships with HTTP MCP server automatically consumed by the UI.
- **MCP stdio mode** – `go run cmd/mcp-stdio/main.go` exposes the agent to Cursor/CLI.
- **External MCP federation** – register third-party MCP servers (HTTP, stdio, or SSE) from the UI, toggle them per engagement, and monitor their health and call volume in real time.

#### MCP stdio quick start
1. **Build the binary** (run from the project root):
   ```bash
   go build -o cyberstrike-ai-mcp cmd/mcp-stdio/main.go
   ```
2. **Wire it up in Cursor**  
   Open `Settings → Tools & MCP → Add Custom MCP`, pick **Command**, then point to the compiled binary and your config:
   ```json
   {
     "mcpServers": {
       "cyberstrike-ai": {
         "command": "/absolute/path/to/cyberstrike-ai-mcp",
         "args": [
           "--config",
           "/absolute/path/to/config.yaml"
         ]
       }
     }
   }
   ```
   Replace the paths with your local locations; Cursor will launch the stdio server automatically.

#### MCP HTTP quick start
1. Ensure `config.yaml` has `mcp.enabled: true` and adjust `mcp.host` / `mcp.port` if you need a non-default binding (localhost:8081 works well for local Cursor usage).
2. Start the main service (`./run.sh` or `go run cmd/server/main.go`); the MCP endpoint lives at `http://<host>:<port>/mcp`.
3. In Cursor, choose **Add Custom MCP → HTTP** and set `Base URL` to `http://127.0.0.1:8081/mcp`.
4. Prefer committing the setup via `.cursor/mcp.json` so teammates can reuse it:
   ```json
   {
     "mcpServers": {
       "cyberstrike-ai-http": {
         "transport": "http",
         "url": "http://127.0.0.1:8081/mcp"
       }
     }
   }
   ```

#### External MCP federation (HTTP/stdio/SSE)
CyberStrikeAI supports connecting to external MCP servers via three transport modes:
- **HTTP mode** – traditional request/response over HTTP POST
- **stdio mode** – process-based communication via standard input/output
- **SSE mode** – Server-Sent Events for real-time streaming communication

To add an external MCP server:
1. Open the Web UI and navigate to **Settings → External MCP**.
2. Click **Add External MCP** and provide the configuration in JSON format:

   **HTTP mode example:**
   ```json
   {
     "my-http-mcp": {
       "transport": "http",
       "url": "http://127.0.0.1:8081/mcp",
       "description": "HTTP MCP server",
       "timeout": 30
     }
   }
   ```

   **stdio mode example:**
   ```json
   {
     "my-stdio-mcp": {
       "command": "python3",
       "args": ["/path/to/mcp-server.py"],
       "description": "stdio MCP server",
       "timeout": 30
     }
   }
   ```

   **SSE mode example:**
   ```json
   {
     "my-sse-mcp": {
       "transport": "sse",
       "url": "http://127.0.0.1:8082/sse",
       "description": "SSE MCP server",
       "timeout": 30
     }
   }
   ```

3. Click **Save** and then **Start** to connect to the server.
4. Monitor the connection status, tool count, and health in real time.

**SSE mode benefits:**
- Real-time bidirectional communication via Server-Sent Events
- Suitable for scenarios requiring continuous data streaming
- Lower latency for push-based notifications

A test SSE MCP server is available at `cmd/test-sse-mcp-server/` for validation purposes.

### Knowledge Base
- **Vector search** – AI agent can automatically search the knowledge base for relevant security knowledge during conversations using the `search_knowledge_base` tool.
- **Hybrid retrieval** – combines vector similarity search with keyword matching for better accuracy.
- **Auto-indexing** – scans the `knowledge_base/` directory for Markdown files and automatically indexes them with embeddings.
- **Web management** – create, update, delete knowledge items through the web UI, with category-based organization.
- **Retrieval logs** – tracks all knowledge retrieval operations for audit and debugging.

**Quick Start (Using Pre-built Knowledge Base):**
1. **Download the knowledge database** – Download the pre-built knowledge database file from [GitHub Releases](https://github.com/Ed1s0nZ/CyberStrikeAI/releases).
2. **Extract and place** – Extract the downloaded knowledge database file (`knowledge.db`) and place it in the project's `data/` directory.
3. **Restart the service** – Restart the CyberStrikeAI service, and the knowledge base will be ready to use immediately without rebuilding the index.

**Setting up the knowledge base:**
1. **Enable in config** – set `knowledge.enabled: true` in `config.yaml`:
   ```yaml
   knowledge:
     enabled: true
     base_path: knowledge_base
     embedding:
       provider: openai
       model: text-embedding-v4
       base_url: "https://api.openai.com/v1"  # or your embedding API
       api_key: "sk-xxx"
     retrieval:
       top_k: 5
       similarity_threshold: 0.7
       hybrid_weight: 0.7
   ```
2. **Add knowledge files** – place Markdown files in `knowledge_base/` directory, organized by category (e.g., `knowledge_base/SQL Injection/README.md`).
3. **Scan and index** – use the web UI to scan the knowledge base directory, which will automatically import files and build vector embeddings.
4. **Use in conversations** – the AI agent will automatically use `search_knowledge_base` when it needs security knowledge. You can also explicitly ask: "Search the knowledge base for SQL injection techniques".

**Knowledge base structure:**
- Files are organized by category (directory name becomes the category).
- Each Markdown file becomes a knowledge item with automatic chunking for vector search.
- The system supports incremental updates – modified files are re-indexed automatically.


### Automation Hooks
- **REST APIs** – everything the UI uses (auth, conversations, tool runs, monitor, vulnerabilities, roles) is available over JSON.
- **Role APIs** – manage security testing roles via `/api/roles` endpoints: `GET /api/roles` (list all roles), `GET /api/roles/:name` (get role), `POST /api/roles` (create role), `PUT /api/roles/:name` (update role), `DELETE /api/roles/:name` (delete role). Roles are stored as YAML files in the `roles/` directory and support hot-reload.
- **Vulnerability APIs** – manage vulnerabilities via `/api/vulnerabilities` endpoints: `GET /api/vulnerabilities` (list with filters), `POST /api/vulnerabilities` (create), `GET /api/vulnerabilities/:id` (get), `PUT /api/vulnerabilities/:id` (update), `DELETE /api/vulnerabilities/:id` (delete), `GET /api/vulnerabilities/stats` (statistics).
- **Batch Task APIs** – manage batch task queues via `/api/batch-tasks` endpoints: `POST /api/batch-tasks` (create queue), `GET /api/batch-tasks` (list queues), `GET /api/batch-tasks/:queueId` (get queue), `POST /api/batch-tasks/:queueId/start` (start execution), `POST /api/batch-tasks/:queueId/cancel` (cancel), `DELETE /api/batch-tasks/:queueId` (delete), `POST /api/batch-tasks/:queueId/tasks` (add task), `PUT /api/batch-tasks/:queueId/tasks/:taskId` (update task), `DELETE /api/batch-tasks/:queueId/tasks/:taskId` (delete task). Tasks execute sequentially, each creating a separate conversation with full status tracking.
- **Task control** – pause/resume/stop long scans, re-run steps with new params, or stream transcripts.
- **Audit & security** – rotate passwords via `/api/auth/change-password`, enforce short-lived sessions, and restrict MCP ports at the network layer when exposing the service.

## Configuration Reference

```yaml
auth:
  password: "change-me"
  session_duration_hours: 12
server:
  host: "0.0.0.0"
  port: 8080
log:
  level: "info"
  output: "stdout"
mcp:
  enabled: true
  host: "0.0.0.0"
  port: 8081
openai:
  api_key: "sk-xxx"
  base_url: "https://api.deepseek.com/v1"
  model: "deepseek-chat"
database:
  path: "data/conversations.db"
  knowledge_db_path: "data/knowledge.db"  # Optional: separate DB for knowledge base
security:
  tools_dir: "tools"
knowledge:
  enabled: false  # Enable knowledge base feature
  base_path: "knowledge_base"  # Path to knowledge base directory
  embedding:
    provider: "openai"  # Embedding provider (currently only "openai")
    model: "text-embedding-v4"  # Embedding model name
    base_url: ""  # Leave empty to use OpenAI base_url
    api_key: ""  # Leave empty to use OpenAI api_key
  retrieval:
    top_k: 5  # Number of top results to return
    similarity_threshold: 0.7  # Minimum similarity score (0-1)
    hybrid_weight: 0.7  # Weight for vector search (1.0 = pure vector, 0.0 = pure keyword)
roles_dir: "roles"  # Role configuration directory (relative to config file)
```

### Tool Definition Example (`tools/nmap.yaml`)

```yaml
name: "nmap"
command: "nmap"
args: ["-sT", "-sV", "-sC"]
enabled: true
short_description: "Network mapping & service fingerprinting"
parameters:
  - name: "target"
    type: "string"
    description: "IP or domain"
    required: true
    position: 0
  - name: "ports"
    type: "string"
    flag: "-p"
    description: "Range, e.g. 1-1000"
```

### Role Definition Example (`roles/penetration-testing.yaml`)

```yaml
name: Penetration Testing
description: Professional penetration testing expert for comprehensive security testing
user_prompt: You are a professional cybersecurity penetration testing expert. Please use professional penetration testing methods and tools to conduct comprehensive security testing on targets, including but not limited to SQL injection, XSS, CSRF, file inclusion, command execution and other common vulnerabilities.
icon: "\U0001F3AF"
tools:
  - nmap
  - sqlmap
  - nuclei
  - burpsuite
  - metasploit
  - httpx
  - record_vulnerability
  - list_knowledge_risk_types
  - search_knowledge_base
enabled: true
```

## Project Layout

```
CyberStrikeAI/
├── cmd/                 # Server, MCP stdio entrypoints, tooling
├── internal/            # Agent, MCP core, handlers, security executor
├── web/                 # Static SPA + templates
├── tools/               # YAML tool recipes (100+ examples provided)
├── roles/               # Role configurations (12+ predefined security testing roles)
├── img/                 # Docs screenshots & diagrams
├── config.yaml          # Runtime configuration
├── run.sh               # Convenience launcher
└── README*.md
```

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
Use external Burp-based MCP server for authenticated traffic replay, then pass findings back for graphing.
Compress the 5 MB nuclei report, summarize critical CVEs, and attach the artifact to the conversation.
Build an attack chain for the latest engagement and export the node list with severity >= high.
```

## Changelog

See [CHANGELOG.md](CHANGELOG.md) for detailed version history and all changes.

### Recent Highlights

- **2026-01-11** – Role-based testing with predefined security testing roles
- **2026-01-08** – SSE transport mode support for external MCP servers
- **2026-01-01** – Batch task management with queue-based execution
- **2025-12-25** – Vulnerability management and conversation grouping features
- **2025-12-20** – Knowledge base with vector search and hybrid retrieval

## Star History

![Star History Chart](https://api.star-history.com/svg?repos=Ed1s0nZ/CyberStrikeAI&type=date&legend=top-left)


## 404Starlink 

<img src="./img/404StarLinkLogo.png" width="30%">

CyberStrikeAI has joined [404Starlink](https://github.com/knownsec/404StarLink)

## TCH Top-Ranked Intelligent Pentest Project  
<div align="left">
  <a href="https://zc.tencent.com/competition/competitionHackathon?code=cha004" target="_blank">
    <img src="./img/tch.png" alt="TCH Top-Ranked Intelligent Pentest Project" width="30%">
  </a>
</div>



---

Need help or want to contribute? Open an issue or PR—community tooling additions are welcome!


