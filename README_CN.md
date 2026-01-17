<div align="center">
  <img src="web/static/logo.png" alt="CyberStrikeAI Logo" width="200">
</div>

# CyberStrikeAI

[中文](README_CN.md) | [English](README.md)

CyberStrikeAI 是一款 **AI 原生安全测试平台**，基于 Go 构建，集成了 100+ 安全工具、智能编排引擎、角色化测试与预设安全测试角色、Skills 技能系统与专业测试技能，以及完整的测试生命周期管理能力。通过原生 MCP 协议与 AI 智能体，支持从对话指令到漏洞发现、攻击链分析、知识检索与结果可视化的全流程自动化，为安全团队提供可审计、可追溯、可协作的专业测试环境。


## 界面与集成预览

<div align="center">

### 核心功能概览

<table>
<tr>
<td width="33.33%" align="center">
<strong>Web 控制台</strong><br/>
<img src="./images/web-console.png" alt="Web 控制台" width="100%">
</td>
<td width="33.33%" align="center">
<strong>攻击链可视化</strong><br/>
<img src="./images/attack-chain.png" alt="攻击链" width="100%">
</td>
<td width="33.33%" align="center">
<strong>任务管理</strong><br/>
<img src="./images/task-management.png" alt="任务管理" width="100%">
</td>
</tr>
<tr>
<td width="33.33%" align="center">
<strong>漏洞管理</strong><br/>
<img src="./images/vulnerability-management.png" alt="漏洞管理" width="100%">
</td>
<td width="33.33%" align="center">
<strong>MCP 管理</strong><br/>
<img src="./images/mcp-management.png" alt="MCP 管理" width="100%">
</td>
<td width="33.33%" align="center">
<strong>MCP stdio 模式</strong><br/>
<img src="./images/mcp-stdio2.png" alt="MCP stdio 模式" width="100%">
</td>
</tr>
<tr>
<td width="33.33%" align="center">
<strong>知识库</strong><br/>
<img src="./images/knowledge-base.png" alt="知识库" width="100%">
</td>
<td width="33.33%" align="center">
<strong>Skills 管理</strong><br/>
<img src="./images/skills.png" alt="Skills 管理" width="100%">
</td>
<td width="33.33%" align="center">
<strong>角色管理</strong><br/>
<img src="./images/role-management.png" alt="角色管理" width="100%">
</td>
</tr>
</table>

</div>

## 特性速览

- 🤖 兼容 OpenAI/DeepSeek/Claude 等模型的智能决策引擎
- 🔌 原生 MCP 协议，支持 HTTP / stdio / SSE 传输模式以及外部 MCP 接入
- 🧰 100+ 现成工具模版 + YAML 扩展能力
- 📄 大结果分页、压缩与全文检索
- 🔗 攻击链可视化、风险打分与步骤回放
- 🔒 Web 登录保护、审计日志、SQLite 持久化
- 📚 知识库功能：向量检索与混合搜索，为 AI 提供安全专业知识
- 📁 对话分组管理：支持分组创建、置顶、重命名、删除等操作
- 🛡️ 漏洞管理功能：完整的漏洞 CRUD 操作，支持严重程度分级、状态流转、按对话/严重程度/状态过滤，以及统计看板
- 📋 批量任务管理：创建任务队列，批量添加任务，依次顺序执行，支持任务编辑与状态跟踪
- 🎭 角色化测试：预设安全测试角色（渗透测试、CTF、Web 应用扫描等），支持自定义提示词和工具限制
- 🎯 Skills 技能系统：20+ 预设安全测试技能（SQL 注入、XSS、API 安全等），可附加到角色或由 AI 按需调用

## 工具概览

系统预置 100+ 渗透/攻防工具，覆盖完整攻击链：

- **网络扫描**：nmap、masscan、rustscan、arp-scan、nbtscan
- **Web 应用扫描**：sqlmap、nikto、dirb、gobuster、feroxbuster、ffuf、httpx
- **漏洞扫描**：nuclei、wpscan、wafw00f、dalfox、xsser
- **子域名枚举**：subfinder、amass、findomain、dnsenum、fierce
- **网络空间搜索引擎**：fofa_search、zoomeye_search
- **API 安全**：graphql-scanner、arjun、api-fuzzer、api-schema-analyzer
- **容器安全**：trivy、clair、docker-bench-security、kube-bench、kube-hunter
- **云安全**：prowler、scout-suite、cloudmapper、pacu、terrascan、checkov
- **二进制分析**：gdb、radare2、ghidra、objdump、strings、binwalk
- **漏洞利用**：metasploit、msfvenom、pwntools、ropper、ropgadget
- **密码破解**：hashcat、john、hashpump
- **取证分析**：volatility、volatility3、foremost、steghide、exiftool
- **后渗透**：linpeas、winpeas、mimikatz、bloodhound、impacket、responder
- **CTF 实用工具**：stegsolve、zsteg、hash-identifier、fcrackzip、pdfcrack、cyberchef
- **系统辅助**：exec、create-file、delete-file、list-files、modify-file

## 基础使用

### 快速上手（一条命令部署）

**环境要求：**
- Go 1.21+ ([下载安装](https://go.dev/dl/))
- Python 3.10+ ([下载安装](https://www.python.org/downloads/))

**一条命令部署：**
```bash
git clone https://github.com/Ed1s0nZ/CyberStrikeAI.git
cd CyberStrikeAI-main
chmod +x run.sh && ./run.sh
```

`run.sh` 脚本会自动完成：
- ✅ 检查并验证 Go 和 Python 环境
- ✅ 创建 Python 虚拟环境
- ✅ 安装 Python 依赖包
- ✅ 下载 Go 依赖模块
- ✅ 编译构建项目
- ✅ 启动服务器

**首次配置：**
1. **配置 AI 模型 API**（首次使用前必填）
   - 启动后访问 http://localhost:8080
   - 进入 `设置` → 填写 API 配置信息：
     ```yaml
     openai:
       api_key: "sk-your-key"
       base_url: "https://api.openai.com/v1"  # 或 https://api.deepseek.com/v1
       model: "gpt-4o"  # 或 deepseek-chat, claude-3-opus 等
     ```
   - 或启动前直接编辑 `config.yaml` 文件
2. **登录系统** - 使用控制台显示的自动生成密码（或在 `config.yaml` 中设置 `auth.password`）
3. **安装安全工具（可选）** - 按需安装所需工具：
   ```bash
   # macOS
   brew install nmap sqlmap nuclei httpx gobuster feroxbuster subfinder amass
   # Ubuntu/Debian
   sudo apt-get install nmap sqlmap nuclei httpx gobuster feroxbuster
   ```
   未安装的工具会自动跳过或改用替代方案。

**其他启动方式：**
```bash
# 直接运行（需手动配置环境）
go run cmd/server/main.go

# 手动编译
go build -o cyberstrike-ai cmd/server/main.go
./cyberstrike-ai
```

**说明：** Python 虚拟环境（`venv/`）由 `run.sh` 自动创建和管理。需要 Python 的工具（如 `api-fuzzer`、`http-framework-test` 等）会自动使用该环境。

### 常用流程
- **对话测试**：自然语言触发多步工具编排，SSE 实时输出。
- **角色化测试**：从预设的安全测试角色（渗透测试、CTF、Web 应用扫描、API 安全测试等）中选择，自定义 AI 行为和可用工具。每个角色可应用自定义系统提示词，并可限制可用工具列表，实现聚焦的测试场景。
- **工具监控**：查看任务队列、执行日志、大文件附件。
- **会话历史**：所有对话与工具调用保存在 SQLite，可随时重放。
- **对话分组**：将对话按项目或主题组织到不同分组，支持置顶、重命名、删除等操作，所有数据持久化存储。
- **漏洞管理**：在测试过程中创建、更新和跟踪发现的漏洞。支持按严重程度（严重/高/中/低/信息）、状态（待确认/已确认/已修复/误报）和对话进行过滤，查看统计信息并导出发现。
- **批量任务管理**：创建任务队列，批量添加多个任务，执行前可编辑或删除任务，然后依次顺序执行。每个任务会作为独立对话执行，支持完整的状态跟踪（待执行/执行中/已完成/失败/已取消）和执行历史。
- **可视化配置**：在界面中切换模型、启停工具、设置迭代次数等。

### 默认安全措施
- 设置面板内置必填校验，防止漏配 API Key/Base URL/模型。
- `auth.password` 为空时自动生成 24 位强口令并写回 `config.yaml`。
- 所有 API（除登录外）都需携带 Bearer Token，统一鉴权中间件拦截。
- 每个工具执行都带有超时、日志和错误隔离。

## 进阶使用

### 角色化测试
- **预设角色**：系统内置 12+ 个预设的安全测试角色（渗透测试、CTF、Web 应用扫描、API 安全测试、二进制分析、云安全审计等），位于 `roles/` 目录。
- **自定义提示词**：每个角色可定义 `user_prompt`，会在用户消息前自动添加，引导 AI 采用特定的测试方法和关注重点。
- **工具限制**：角色可指定 `tools` 列表，限制可用工具，实现聚焦的测试流程（如 CTF 角色限制为 CTF 专用工具）。
- **Skills 集成**：角色可附加安全测试技能。技能名称会作为提示添加到系统提示词中，AI 智能体可通过 `read_skill` 工具按需获取技能内容。
- **轻松创建角色**：通过在 `roles/` 目录添加 YAML 文件即可创建自定义角色。每个角色定义 `name`、`description`、`user_prompt`、`icon`、`tools`、`skills`、`enabled` 字段。
- **Web 界面集成**：在聊天界面通过下拉菜单选择角色。角色选择会影响 AI 行为和可用工具建议。

**创建自定义角色示例：**
1. 在 `roles/` 目录创建 YAML 文件（如 `roles/custom-role.yaml`）：
   ```yaml
   name: 自定义角色
   description: 专用测试场景
   user_prompt: 你是一个专注于 API 安全的专业安全测试人员...
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
2. 重启服务或重新加载配置，角色会出现在角色选择下拉菜单中。

### Skills 技能系统
- **预设技能**：系统内置 20+ 个预设的安全测试技能（SQL 注入、XSS、API 安全、云安全、容器安全等），位于 `skills/` 目录。
- **提示词中的技能提示**：当选择某个角色时，该角色附加的技能名称会作为推荐添加到系统提示词中。技能内容不会自动注入，AI 智能体需要时需使用 `read_skill` 工具获取技能详情。
- **按需调用**：AI 智能体也可以通过内置工具（`list_skills`、`read_skill`）按需访问技能，允许在执行任务过程中动态获取相关技能。
- **结构化格式**：每个技能是一个目录，包含一个 `SKILL.md` 文件，详细描述测试方法、工具使用、最佳实践和示例。技能支持 YAML front matter 格式用于元数据。
- **自定义技能**：通过在 `skills/` 目录添加目录即可创建自定义技能。每个技能目录应包含一个 `SKILL.md` 文件。

**创建自定义技能：**
1. 在 `skills/` 目录创建目录（如 `skills/my-skill/`）
2. 在该目录下创建 `SKILL.md` 文件，编写技能内容
3. 在角色的 YAML 文件中，通过添加 `skills` 字段将该技能附加到角色

### 工具编排与扩展
- `tools/*.yaml` 定义命令、参数、提示词与元数据，可热加载。
- `security.tools_dir` 指向目录即可批量启用；仍支持在主配置里内联定义。
- **大结果分页**：超过 200KB 的输出会保存为附件，可通过 `query_execution_result` 工具分页、过滤、正则检索。
- **结果压缩/摘要**：多兆字节日志可先压缩或生成摘要再写入 SQLite，减小档案体积。

**自定义工具的一般步骤**
1. 复制 `tools/` 下现有示例（如 `tools/sample.yaml`）。
2. 修改 `name`、`command`、`args`、`short_description` 等基础信息。
3. 在 `parameters[]` 中声明位置参数或带 flag 的参数，方便智能体自动拼装命令。
4. 视需要补充 `description` 或 `notes`，给 AI 额外上下文或结果解读提示。
5. 重启服务或在界面中重新加载配置，新工具即可在 Settings 面板中启用/禁用。

### 攻击链分析
- 智能体解析每次对话，抽取目标、工具、漏洞与因果关系。
- Web 端可交互式查看链路节点、风险级别及时间轴，支持导出报告。

### MCP 全场景
- **Web 模式**：自带 HTTP MCP 服务供前端调用。
- **MCP stdio 模式**：`go run cmd/mcp-stdio/main.go` 可接入 Cursor/命令行。
- **外部 MCP 联邦**：在设置中注册第三方 MCP（HTTP/stdio/SSE），按需启停并实时查看调用统计与健康度。

#### MCP stdio 快速集成
1. **编译可执行文件**（在项目根目录执行）：
   ```bash
   go build -o cyberstrike-ai-mcp cmd/mcp-stdio/main.go
   ```
2. **在 Cursor 中配置**  
   打开 `Settings → Tools & MCP → Add Custom MCP`，选择 **Command**，指定编译后的程序与配置文件：
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
   将路径替换成你本地的实际地址，Cursor 会自动启动 stdio 版本的 MCP。

#### MCP HTTP 快速集成
1. 确认 `config.yaml` 中 `mcp.enabled: true`，按照需要调整 `mcp.host` / `mcp.port`（本地建议 `127.0.0.1:8081`）。
2. 启动主服务（`./run.sh` 或 `go run cmd/server/main.go`），MCP 端点默认暴露在 `http://<host>:<port>/mcp`。
3. 在 Cursor 内 `Add Custom MCP → HTTP`，将 `Base URL` 设置为 `http://127.0.0.1:8081/mcp`。
4. 也可以在项目根目录创建 `.cursor/mcp.json` 以便团队共享：
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

#### 外部 MCP 联邦（HTTP/stdio/SSE）
CyberStrikeAI 支持通过三种传输模式连接外部 MCP 服务器：
- **HTTP 模式** – 通过 HTTP POST 进行传统的请求/响应通信
- **stdio 模式** – 通过标准输入/输出进行进程间通信
- **SSE 模式** – 通过 Server-Sent Events 实现实时流式通信

添加外部 MCP 服务器：
1. 打开 Web 界面，进入 **设置 → 外部MCP**。
2. 点击 **添加外部MCP**，以 JSON 格式提供配置：

   **HTTP 模式示例：**
   ```json
   {
     "my-http-mcp": {
       "transport": "http",
       "url": "http://127.0.0.1:8081/mcp",
       "description": "HTTP MCP 服务器",
       "timeout": 30
     }
   }
   ```

   **stdio 模式示例：**
   ```json
   {
     "my-stdio-mcp": {
       "command": "python3",
       "args": ["/path/to/mcp-server.py"],
       "description": "stdio MCP 服务器",
       "timeout": 30
     }
   }
   ```

   **SSE 模式示例：**
   ```json
   {
     "my-sse-mcp": {
       "transport": "sse",
       "url": "http://127.0.0.1:8082/sse",
       "description": "SSE MCP 服务器",
       "timeout": 30
     }
   }
   ```

3. 点击 **保存**，然后点击 **启动** 连接服务器。
4. 实时监控连接状态、工具数量和健康度。

**SSE 模式优势：**
- 通过 Server-Sent Events 实现实时双向通信
- 适用于需要持续数据流的场景
- 对于基于推送的通知，延迟更低

可在 `cmd/test-sse-mcp-server/` 目录找到用于验证的测试 SSE MCP 服务器。


### 知识库功能
- **向量检索**：AI 智能体在对话过程中可自动调用 `search_knowledge_base` 工具搜索知识库中的安全知识。
- **混合检索**：结合向量相似度搜索与关键词匹配，提升检索准确性。
- **自动索引**：扫描 `knowledge_base/` 目录下的 Markdown 文件，自动构建向量嵌入索引。
- **Web 管理**：通过 Web 界面创建、更新、删除知识项，支持分类管理。
- **检索日志**：记录所有知识检索操作，便于审计与调试。

**快速开始（使用预构建知识库）：**
1. **下载知识数据库**：从 [GitHub Releases](https://github.com/Ed1s0nZ/CyberStrikeAI/releases) 下载预构建的知识数据库文件。
2. **解压并放置**：将下载的知识数据库文件（`knowledge.db`）解压后放到项目的 `data/` 目录下。
3. **重启服务**：重启 CyberStrikeAI 服务，知识库即可直接使用，无需重新构建索引。

**知识库配置步骤：**
1. **启用功能**：在 `config.yaml` 中设置 `knowledge.enabled: true`：
   ```yaml
   knowledge:
     enabled: true
     base_path: knowledge_base
     embedding:
       provider: openai
       model: text-embedding-v4
       base_url: "https://api.openai.com/v1"  # 或你的嵌入模型 API
       api_key: "sk-xxx"
     retrieval:
       top_k: 5
       similarity_threshold: 0.7
       hybrid_weight: 0.7
   ```
2. **添加知识文件**：将 Markdown 文件放入 `knowledge_base/` 目录，按分类组织（如 `knowledge_base/SQL注入/README.md`）。
3. **扫描索引**：在 Web 界面中点击"扫描知识库"，系统会自动导入文件并构建向量索引。
4. **对话中使用**：AI 智能体在需要安全知识时会自动调用知识检索工具。你也可以显式要求："搜索知识库中关于 SQL 注入的技术"。

**知识库结构说明：**
- 文件按分类组织（目录名作为分类）。
- 每个 Markdown 文件自动切块并生成向量嵌入。
- 支持增量更新，修改后的文件会自动重新索引。


### 自动化与安全
- **REST API**：认证、会话、任务、监控、漏洞管理、角色管理等接口全部开放，可与 CI/CD 集成。
- **角色管理 API**：通过 `/api/roles` 端点管理安全测试角色：`GET /api/roles`（列表）、`GET /api/roles/:name`（获取角色）、`POST /api/roles`（创建角色）、`PUT /api/roles/:name`（更新角色）、`DELETE /api/roles/:name`（删除角色）。角色以 YAML 文件形式存储在 `roles/` 目录，支持热加载。
- **漏洞管理 API**：通过 `/api/vulnerabilities` 端点管理漏洞：`GET /api/vulnerabilities`（列表，支持过滤）、`POST /api/vulnerabilities`（创建）、`GET /api/vulnerabilities/:id`（获取）、`PUT /api/vulnerabilities/:id`（更新）、`DELETE /api/vulnerabilities/:id`（删除）、`GET /api/vulnerabilities/stats`（统计）。
- **批量任务 API**：通过 `/api/batch-tasks` 端点管理批量任务队列：`POST /api/batch-tasks`（创建队列）、`GET /api/batch-tasks`（列表）、`GET /api/batch-tasks/:queueId`（获取队列）、`POST /api/batch-tasks/:queueId/start`（开始执行）、`POST /api/batch-tasks/:queueId/cancel`（取消）、`DELETE /api/batch-tasks/:queueId`（删除队列）、`POST /api/batch-tasks/:queueId/tasks`（添加任务）、`PUT /api/batch-tasks/:queueId/tasks/:taskId`（更新任务）、`DELETE /api/batch-tasks/:queueId/tasks/:taskId`（删除任务）。任务依次顺序执行，每个任务创建独立对话，支持完整状态跟踪。
- **任务控制**：支持暂停/终止长任务、修改参数后重跑、流式获取日志。
- **安全管理**：`/api/auth/change-password` 可即时轮换口令；建议在暴露 MCP 端口时配合网络层 ACL。

## 配置参考

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
  knowledge_db_path: "data/knowledge.db"  # 可选：知识库独立数据库
security:
  tools_dir: "tools"
knowledge:
  enabled: false  # 是否启用知识库功能
  base_path: "knowledge_base"  # 知识库目录路径
  embedding:
    provider: "openai"  # 嵌入模型提供商（目前仅支持 openai）
    model: "text-embedding-v4"  # 嵌入模型名称
    base_url: ""  # 留空则使用 OpenAI 配置的 base_url
    api_key: ""  # 留空则使用 OpenAI 配置的 api_key
  retrieval:
    top_k: 5  # 检索返回的 Top-K 结果数量
    similarity_threshold: 0.7  # 相似度阈值（0-1），低于此值的结果将被过滤
    hybrid_weight: 0.7  # 混合检索权重（0-1），向量检索的权重，1.0 表示纯向量检索，0.0 表示纯关键词检索
roles_dir: "roles"  # 角色配置文件目录（相对于配置文件所在目录）
skills_dir: "skills"  # Skills 目录（相对于配置文件所在目录）
```

### 工具模版示例（`tools/nmap.yaml`）

```yaml
name: "nmap"
command: "nmap"
args: ["-sT", "-sV", "-sC"]
enabled: true
short_description: "网络资产扫描与服务指纹识别"
parameters:
  - name: "target"
    type: "string"
    description: "IP 或域名"
    required: true
    position: 0
  - name: "ports"
    type: "string"
    flag: "-p"
    description: "端口范围，如 1-1000"
```

### 角色配置示例（`roles/渗透测试.yaml`）

```yaml
name: 渗透测试
description: 专业渗透测试专家，全面深入的漏洞检测
user_prompt: 你是一个专业的网络安全渗透测试专家。请使用专业的渗透测试方法和工具，对目标进行全面的安全测试，包括但不限于SQL注入、XSS、CSRF、文件包含、命令执行等常见漏洞。
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

## 项目结构

```
CyberStrikeAI/
├── cmd/                 # Web 服务、MCP stdio 入口及辅助工具
├── internal/            # Agent、MCP 核心、路由与执行器
├── web/                 # 前端静态资源与模板
├── tools/               # YAML 工具目录（含 100+ 示例）
├── roles/               # 角色配置文件目录（含 12+ 预设安全测试角色）
├── skills/              # Skills 目录（含 20+ 预设安全测试技能）
├── images/              # 文档配图
├── config.yaml          # 运行配置
├── run.sh               # 启动脚本
└── README*.md
```

## 基础体验示例

```
扫描 192.168.1.1 的开放端口
对 192.168.1.1 做 80/443/22 重点扫描
检查 https://example.com/page?id=1 是否存在 SQL 注入
枚举 https://example.com 的隐藏目录与组件漏洞
获取 example.com 的子域并批量执行 nuclei
```

## 进阶剧本示例

```
加载侦察剧本：先 amass/subfinder，再对存活主机进行目录爆破。
挂载基于 Burp 的外部 MCP，完成认证流量回放并回传到攻击链。
将 5MB nuclei 报告压缩并生成摘要，附加到对话记录。
构建最新一次测试的攻击链，只导出风险 >= 高的节点列表。
```

## 更新日志

详细版本历史和所有变更请查看 [CHANGELOG.md](CHANGELOG.md)。

### 近期亮点

- **2026-01-15** – 新增 Skills 技能系统，内置 20+ 预设安全测试技能
- **2026-01-11** – 新增角色化测试功能，支持预设安全测试角色
- **2026-01-08** – 新增 SSE 传输模式支持，外部 MCP 联邦支持三种模式
- **2026-01-01** – 新增批量任务管理功能，支持队列式任务执行
- **2025-12-25** – 新增漏洞管理和对话分组功能
- **2025-12-20** – 新增知识库功能，支持向量检索和混合搜索


## 404星链计划 
<img src="./images/404StarLinkLogo.png" width="30%">

CyberStrikeAI 现已加入 [404星链计划](https://github.com/knownsec/404StarLink)

## TCH Top-Ranked Intelligent Pentest Project  
<div align="left">
  <a href="https://zc.tencent.com/competition/competitionHackathon?code=cha004" target="_blank">
    <img src="./images/tch.png" alt="TCH Top-Ranked Intelligent Pentest Project" width="30%">
  </a>
</div>

---

欢迎提交 Issue/PR 贡献新的工具模版或优化建议！
