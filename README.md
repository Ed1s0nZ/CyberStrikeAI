<div align="center">
  <img src="web/static/logo.png" alt="CyberStrikeAI Logo" width="200">
</div>

# CyberStrikeAI


[Chinese](README_CN.md) | [English](README.md)

CyberStrikeAI is an **AI-native security testing platform** built in Go. It integrates 100+ security tools, an intelligent orchestration engine, role-based testing with predefined security roles, a skills system with specialized testing skills, and comprehensive lifecycle management capabilities. Through native MCP protocol and AI agents, it enables end-to-end automation from conversational commands to vulnerability discovery, attack-chain analysis, knowledge retrieval, and result visualization—delivering an auditable, traceable, and collaborative testing environment for security teams.


## Interface & Integration Preview

<div align="center">

### System Dashboard Overview

<img src="./images/dashboard.png" alt="System Dashboard" width="100%">

*The dashboard provides a comprehensive overview of system runtime status, security vulnerabilities, tool usage, and knowledge base, helping users quickly understand the platform's core features and current state.*

### Core Features Overview

<table>
<tr>
<td width="33.33%" align="center">
<strong>Web Console</strong><br/>
<img src="./images/web-console.png" alt="Web Console" width="100%">
</td>
<td width="33.33%" align="center">
<strong>Attack Chain Visualization</strong><br/>
<img src="./images/attack-chain.png" alt="Attack Chain" width="100%">
</td>
<td width="33.33%" align="center">
<strong>Task Management</strong><br/>
<img src="./images/task-management.png" alt="Task Management" width="100%">
</td>
</tr>
<tr>
<td width="33.33%" align="center">
<strong>Vulnerability Management</strong><br/>
<img src="./images/vulnerability-management.png" alt="Vulnerability Management" width="100%">
</td>
<td width="33.33%" align="center">
<strong>MCP Management</strong><br/>
<img src="./images/mcp-management.png" alt="MCP management" width="100%">
</td>
<td width="33.33%" align="center">
<strong>MCP stdio Mode</strong><br/>
<img src="./images/mcp-stdio2.png" alt="MCP stdio mode" width="100%">
</td>
</tr>
<tr>
<td width="33.33%" align="center">
<strong>Knowledge Base</strong><br/>
<img src="./images/knowledge-base.png" alt="Knowledge Base" width="100%">
</td>
<td width="33.33%" align="center">
<strong>Skills Management</strong><br/>
<img src="./images/skills.png" alt="Skills Management" width="100%">
</td>
<td width="33.33%" align="center">
<strong>Role Management</strong><br/>
<img src="./images/role-management.png" alt="Role Management" width="100%">
</td>
</tr>
</table>

</div>

## Highlights

- 🤖 AI decision engine with OpenAI-compatible models (GPT, Claude, DeepSeek, etc.)
- 🔌 Native MCP implementation with HTTP/stdio/SSE transports and external MCP federation
- 🧰 100+ prebuilt tool recipes + YAML-based extension system
- 📄 Large-result pagination, compression, and searchable archives
- 🔗 Attack-chain graph, risk scoring, and step-by-step replay
- 🔒 Password-protected web UI, audit logs, and SQLite persistence
- 📚 Knowledge base with vector search and **corpus-level BM25 hybrid retrieval** for security expertise
- 🧠 **Persistent memory**: key-value store that survives conversation compression and server restarts — agents remember credentials, targets, and findings across sessions; tool results are automatically stored as memory entries
- 🔍 **Agent introspection**: before every major action the agent automatically retrieves similar memory entries and pre-fetches relevant knowledge-base guidance to avoid duplicate work and improve tool selection
- ⏰ **Time awareness**: current date/time is automatically injected into every agent system prompt; configurable timezone support
- 📁 Conversation grouping with pinning, rename, and batch management
- 🛡️ Vulnerability management with CRUD operations, severity tracking, status workflow, and statistics
- 📋 Batch task management: create task queues, add multiple tasks, and execute them sequentially
- 🎭 Role-based testing: 13 predefined security testing roles (Penetration Testing, CTF, Web App Scanning, etc.) with custom prompts and tool restrictions
- 🎯 Skills system: 22 predefined security testing skills (SQL injection, XSS, API security, etc.) that can be attached to roles or called on-demand by AI agents
- 🐳 **Docker lifecycle management**: deploy, update, start, stop, restart, and monitor the Docker stack directly from the System Settings UI or via REST API
- 📱 **Chatbot**: DingTalk and Lark (Feishu) long-lived connections so you can talk to CyberStrikeAI from mobile (see [Robot / Chatbot guide](docs/robot_en.md) for setup and commands)

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
git clone https://github.com/cybersecua/CyberStrikeAI.git
cd CyberStrikeAI
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
- **Predefined roles** – System includes 13 predefined security testing roles (Penetration Testing, CTF, Web App Scanning, API Security Testing, Binary Analysis, Cloud Security Audit, etc.) in the `roles/` directory.
- **Custom prompts** – Each role can define a `user_prompt` that prepends to user messages, guiding the AI to adopt specialized testing methodologies and focus areas.
- **Tool restrictions** – Roles can specify a `tools` list to limit available tools, ensuring focused testing workflows (e.g., CTF role restricts to CTF-specific utilities).
- **Skills integration** – Roles can attach security testing skills. Skill names are added to system prompts as hints, and AI agents can access skill content on-demand using the `read_skill` tool.
- **Easy role creation** – Create custom roles by adding YAML files to the `roles/` directory. Each role defines `name`, `description`, `user_prompt`, `icon`, `tools`, `skills`, and `enabled` fields.
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
   skills:
     - api-security-testing
     - sql-injection-testing
   enabled: true
   ```
2. Restart the server or reload configuration; the role appears in the role selector dropdown.

### Skills System
- **Predefined skills** – System includes 22 predefined security testing skills (SQL injection, XSS, API security, cloud security, container security, etc.) in the `skills/` directory.
- **Skill hints in prompts** – When a role is selected, skill names attached to that role are added to the system prompt as recommendations. Skill content is not automatically injected; AI agents must use the `read_skill` tool to access skill details when needed.
- **On-demand access** – AI agents can also access skills on-demand using built-in tools (`list_skills`, `read_skill`), allowing dynamic skill retrieval during task execution.
- **Structured format** – Each skill is a directory containing a `SKILL.md` file with detailed testing methods, tool usage, best practices, and examples. Skills support YAML front matter for metadata.
- **Custom skills** – Create custom skills by adding directories to the `skills/` directory. Each skill directory should contain a `SKILL.md` file with the skill content.

**Creating a custom skill:**
1. Create a directory in `skills/` (e.g., `skills/my-skill/`)
2. Create a `SKILL.md` file in that directory with the skill content
3. Attach the skill to a role by adding it to the role's `skills` field in the role YAML file

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
- **Hybrid retrieval** – combines vector similarity search with **corpus-level BM25 Okapi** keyword scoring for better accuracy. The BM25 index is built from all knowledge chunks (with real inverse document frequency) and is automatically rebuilt when new items are indexed.
- **Configurable hybrid weight** – `knowledge.retrieval.hybrid_weight` blends vector similarity and BM25 scores (1.0 = pure vector, 0.0 = pure keyword).
- **Auto-indexing** – scans the `knowledge_base/` directory for Markdown files and automatically indexes them with embeddings.
- **Web management** – create, update, delete knowledge items through the web UI, with category-based organization.
- **Retrieval logs** – tracks all knowledge retrieval operations for audit and debugging.

**Quick Start (Using Pre-built Knowledge Base):**
1. **Download the knowledge database** – Download the pre-built knowledge database file from [GitHub Releases](https://github.com/cybersecua/CyberStrikeAI/releases).
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


### Persistent Memory

The persistent memory system allows agents to remember key facts across conversation compressions and server restarts. Memories are stored in SQLite alongside conversations and are automatically injected into every system prompt.

**Memory categories:**

| Category | Purpose |
|----------|---------|
| `credential` | Discovered passwords, tokens, API keys, and secrets |
| `target` | IP addresses, domains, service ports, and scope items |
| `vulnerability` | Exploit notes, CVE references, and payload details |
| `fact` | General observations and intelligence about the environment |
| `note` | Operational reminders and planning notes |

**Built-in memory tools (available to the AI agent):**

| Tool | Description |
|------|-------------|
| `store_memory` | Persist a key/value fact with a category |
| `retrieve_memory` | Search memories by query text and optional category |
| `list_memories` | List all stored memories, optionally filtered by category |
| `delete_memory` | Remove a memory entry by ID |

**Configuration (`config.yaml`):**
```yaml
agent:
  memory:
    enabled: true      # Enable persistent memory (default: true)
    max_entries: 200   # Hard cap on entries (0 = unlimited)
```

The context block injected into each system prompt looks like:
```
[CREDENTIALS]
  • admin_password: P@ssw0rd123
[TARGETS]
  • main_target: 192.168.1.100 (Apache 2.4, port 80/443)
[VULNERABILITIES]
  • sqli_endpoint: /login.php?id= is injectable (union-based)
```

---

### Time Awareness

The time awareness feature automatically prepends the current date, time, timezone, and session age to every agent system prompt. This helps the agent:
- Correctly timestamp vulnerability reports and notes
- Reason about time-limited engagements
- Build time-relative plans (schedule scans, set reminders)

**Built-in time tool:**

| Tool | Description |
|------|-------------|
| `get_current_time` | Returns current date/time, timezone, Unix timestamp, and session uptime |

**Configuration (`config.yaml`):**
```yaml
agent:
  time_awareness:
    enabled: true      # Inject time context into prompts (default: true)
    timezone: "UTC"    # IANA timezone (e.g. "America/New_York", "Europe/London", "Asia/Tokyo")
```

The injected block looks like:
```
<time_context>
  Current date and time : 2026-03-04 14:30:00 UTC
  Day of week           : Wednesday
  Unix timestamp        : 1741099800
  Session age           : 2h 15m 30s
</time_context>
```

---

### Docker Lifecycle Management

CyberStrikeAI includes a full Docker management layer accessible from both the web UI and REST API.

**System Settings → Docker panel** lets you:
- View container status, image version, and compose version at a glance
- Stream live container logs with a configurable line count
- Run lifecycle actions (deploy, update, start, stop, restart, remove) with optional proxy or VPN configuration
- Monitor HTTP health probes for the app on ports 8080 and 18080

**CLI via `run_docker.sh`:**
```bash
./run_docker.sh deploy            # Build and start the stack
./run_docker.sh update            # git pull + redeploy
./run_docker.sh start|stop|restart
./run_docker.sh status            # Show container + app status
./run_docker.sh logs              # Print last N container log lines
./run_docker.sh test              # Run container runtime tests
./run_docker.sh remove            # Remove containers, network, and volumes

# Proxy options (passed through to the container environment):
./run_docker.sh deploy --proxy-mode socks --proxy-url socks5h://127.0.0.1:1080
./run_docker.sh deploy --proxy-mode vpn --vpn-container my-vpn
./run_docker.sh update --git-ref v1.5.0
```

**Docker API endpoints:**

| Method | Path | Description |
|--------|------|-------------|
| `GET`  | `/api/docker/status` | Container status, compose version, and HTTP health probes |
| `GET`  | `/api/docker/logs?lines=200` | Last N lines from container logs (or local `logs/suite.log`) |
| `POST` | `/api/docker/action` | Run a lifecycle action (`deploy`, `update`, `start`, `stop`, `restart`, `remove`, `test`) |

**Docker action request body:**
```json
{
  "action": "update",
  "proxy_mode": "direct",
  "proxy_url": "",
  "vpn_container": "",
  "git_ref": "main"
}
```

See [Docker Guide](docs/docker_en.md) for a full walkthrough.

### Agent Introspection

Before every major new action the agent now performs a mandatory introspection pass:

1. **Memory similarity check** – retrieves memory entries that are semantically similar to the current user request, plus entity-based matches (IP addresses, domain names). This surfaces relevant credentials, prior tool results, and known vulnerabilities without requiring the user to repeat context.
2. **Knowledge-base preflight** – runs a focused knowledge-base query combining the user input with penetration-testing terminology to surface relevant exploitation techniques and tool guidance.
3. **Contextual decision** – the agent uses this retrieved context to choose tools and parameters, avoiding duplicate scans and leveraging prior findings automatically.

The introspection context is injected into the system prompt as a `<memory_similarity_context>` block before each agent turn, ensuring the agent never starts a scan it already completed.

### Automation Hooks
- **REST APIs** – everything the UI uses (auth, conversations, tool runs, monitor, vulnerabilities, roles) is available over JSON.
- **Role APIs** – manage security testing roles via `/api/roles` endpoints: `GET /api/roles` (list all roles), `GET /api/roles/:name` (get role), `POST /api/roles` (create role), `PUT /api/roles/:name` (update role), `DELETE /api/roles/:name` (delete role). Roles are stored as YAML files in the `roles/` directory and support hot-reload.
- **Vulnerability APIs** – manage vulnerabilities via `/api/vulnerabilities` endpoints: `GET /api/vulnerabilities` (list with filters), `POST /api/vulnerabilities` (create), `GET /api/vulnerabilities/:id` (get), `PUT /api/vulnerabilities/:id` (update), `DELETE /api/vulnerabilities/:id` (delete), `GET /api/vulnerabilities/stats` (statistics).
- **Batch Task APIs** – manage batch task queues via `/api/batch-tasks` endpoints: `POST /api/batch-tasks` (create queue), `GET /api/batch-tasks` (list queues), `GET /api/batch-tasks/:queueId` (get queue), `POST /api/batch-tasks/:queueId/start` (start execution), `POST /api/batch-tasks/:queueId/cancel` (cancel), `DELETE /api/batch-tasks/:queueId` (delete), `POST /api/batch-tasks/:queueId/tasks` (add task), `PUT /api/batch-tasks/:queueId/tasks/:taskId` (update task), `DELETE /api/batch-tasks/:queueId/tasks/:taskId` (delete task). Tasks execute sequentially, each creating a separate conversation with full status tracking.
- **Docker APIs** – manage the Docker stack via `/api/docker/status`, `/api/docker/logs`, and `/api/docker/action` (see Docker Lifecycle Management section above).
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
  max_total_tokens: 120000  # Token limit shared by memory compression and attack-chain building
database:
  path: "data/conversations.db"
  knowledge_db_path: "data/knowledge.db"  # Optional: separate DB for knowledge base
security:
  tools_dir: "tools"
  tool_description_mode: "full"  # "full" or "short" — controls how much tool info is sent to the LLM
agent:
  max_iterations: 120
  large_result_threshold: 102400  # bytes; results larger than this are stored on disk
  result_storage_dir: "tmp"
  parallel_tool_execution: true
  max_parallel_tools: 0           # 0 = unlimited concurrent tools
  tool_retry_count: 0
  # ── Time Awareness ──────────────────────────────────────────────────────────
  time_awareness:
    enabled: true      # Prepend current date/time to every system prompt (recommended)
    timezone: "UTC"    # IANA timezone name, e.g. "America/New_York", "Europe/London"
  # ── Persistent Memory ───────────────────────────────────────────────────────
  memory:
    enabled: true      # Enable cross-session persistent memory store
    max_entries: 200   # Hard cap on memory entries (0 = unlimited)
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
    hybrid_weight: 0.7  # Weight for vector search (1.0 = pure vector, 0.0 = pure BM25 keyword)
roles_dir: "roles"  # Role configuration directory (relative to config file)
skills_dir: "skills"  # Skills directory (relative to config file)
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

## Related Documentation

| Document | Description |
|----------|-------------|
| [Robot / Chatbot guide](docs/robot_en.md) | Full setup, commands, and troubleshooting for DingTalk & Lark integration. **Follow this to avoid common pitfalls.** |
| [Docker Guide](docs/docker_en.md) | Docker deployment, lifecycle management, proxy/VPN configuration, and System Settings UI reference. |
| [Memory Guide](docs/memory_en.md) | Persistent memory system: categories, agent tools, UI panel, and API reference. |
| [Tool Configuration Guide](tools/README.md) | How to write, configure, and extend YAML tool recipes. |
| [Role Configuration Guide](roles/README.md) | How to create and manage security testing roles. |
| [Skills System Guide](skills/README.md) | How to create and attach skills to roles. |
| [Roadmap](ROADMAP.md) | Planned features and development direction. |

## Project Layout

```
CyberStrikeAI/
├── cmd/                 # Server, MCP stdio, and test entrypoints
├── internal/            # Agent engine, MCP core, handlers, security executor
├── web/                 # Single-page application (templates + static assets)
├── tools/               # YAML tool recipes (100+ provided)
├── roles/               # Role configurations (13 predefined security testing roles)
├── skills/              # Skills library (22 predefined security testing skills)
├── knowledge_base/      # Markdown files for the vector knowledge base
├── scripts/             # Installation and utility scripts
├── docs/                # Additional documentation (chatbot, Docker, memory guides)
├── images/              # Screenshots and diagrams for documentation
├── config.yaml          # Runtime configuration
├── run.sh               # One-command launcher script
├── run_docker.sh        # Docker lifecycle management script
├── docker-compose.yml   # Docker Compose configuration
├── ROADMAP.md           # Development roadmap
└── README.md
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

## 404Starlink 

<img src="./images/404StarLinkLogo.png" width="30%">

CyberStrikeAI has joined [404Starlink](https://github.com/knownsec/404StarLink)

## TCH Top-Ranked Intelligent Pentest Project  
<div align="left">
  <a href="https://zc.tencent.com/competition/competitionHackathon?code=cha004" target="_blank">
    <img src="./images/tch.png" alt="TCH Top-Ranked Intelligent Pentest Project" width="30%">
  </a>
</div>

## Stargazers over time
![Stargazers over time](https://starchart.cc/cybersecua/CyberStrikeAI.svg)


---

## ⚠️ Disclaimer

**This tool is for educational and authorized testing purposes only!**

CyberStrikeAI is a professional security testing platform designed to assist security researchers, penetration testers, and IT professionals in conducting security assessments and vulnerability research **with explicit authorization**.

**By using this tool, you agree to:**
- Use this tool only on systems where you have clear written authorization
- Comply with all applicable laws, regulations, and ethical standards
- Take full responsibility for any unauthorized use or misuse
- Not use this tool for any illegal or malicious purposes

**The developers are not responsible for any misuse!** Please ensure your usage complies with local laws and regulations, and that you have obtained explicit authorization from the target system owner.

---

Need help or want to contribute? Open an issue or PR—community tooling additions are welcome!

See [ROADMAP.md](ROADMAP.md) for planned features and the project direction.


