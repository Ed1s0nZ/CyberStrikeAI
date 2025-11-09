# CyberStrikeAI

🚀 **AI自主渗透测试平台** - 基于Golang构建，内置上百个安全工具，支持灵活扩展自定义工具，通过MCP协议实现AI智能决策与自动化执行，让安全测试像对话一样简单。
  ![详情预览](./img/效果1.png)

## ✨ 功能特性

### 核心功能
- 🤖 **AI智能代理** - 集成OpenAI兼容API（支持GPT、Claude、DeepSeek等），AI自主决策和执行安全测试
- 🧠 **智能决策引擎** - AI分析目标并自动选择最佳测试策略和工具组合
- ⚡ **自主执行** - AI代理自动调用安全工具，无需人工干预，最多支持30轮迭代
- 🔄 **自适应调整** - 根据工具执行结果和发现的漏洞，AI自动调整测试策略
- 📝 **智能总结** - 达到最大迭代次数时，AI自动总结测试结果并提供下一步执行计划
- 💬 **对话式交互** - 自然语言对话界面，支持流式输出（SSE），实时查看执行过程
- 📊 **对话历史管理** - 完整的对话历史记录，支持查看、删除和管理

### 工具集成
- 🔌 **MCP协议支持** - 完整实现MCP协议，支持工具注册、调用、监控
- 🛠️ **灵活工具配置** - 支持从目录加载工具配置（YAML），易于扩展和维护
- 📈 **实时监控** - 监控所有工具的执行状态、结果、调用次数和统计信息
- 🔍 **漏洞自动分析** - 自动分析工具输出，提取和分类发现的漏洞

### 技术特性
- 🚀 **流式输出** - 支持Server-Sent Events (SSE)实时流式输出，提升用户体验
- 💾 **数据持久化** - SQLite数据库存储对话历史和过程详情
- 📝 **详细日志** - 结构化日志记录，便于调试和问题排查
- 🔒 **安全执行** - 工具执行隔离，错误处理和超时控制

## 📁 项目结构

```
CyberStrikeAI/
├── cmd/
│   ├── server/
│   │   └── main.go          # 程序入口，启动HTTP服务器
│   └── test-config/
│       └── main.go          # 配置测试工具
├── internal/
│   ├── agent/               # AI代理模块
│   │   └── agent.go         # Agent Loop实现，处理AI对话和工具调用
│   ├── app/                 # 应用初始化
│   │   └── app.go           # 应用主逻辑，路由设置
│   ├── config/              # 配置管理
│   │   └── config.go        # 配置加载和工具配置管理
│   ├── database/            # 数据库模块
│   │   ├── database.go      # 数据库连接和表结构
│   │   └── conversation.go  # 对话和消息数据访问
│   ├── handler/             # HTTP处理器
│   │   ├── agent.go         # Agent Loop API处理
│   │   ├── conversation.go  # 对话历史API处理
│   │   └── monitor.go       # 监控API处理
│   ├── logger/              # 日志系统
│   │   └── logger.go        # 结构化日志封装
│   ├── mcp/                 # MCP协议实现
│   │   ├── server.go        # MCP服务器核心逻辑
│   │   └── types.go         # MCP协议类型定义
│   └── security/            # 安全工具执行器
│       └── executor.go      # 工具执行和参数构建
├── tools/                   # 工具配置文件目录
│   ├── nmap.yaml            # nmap工具配置
│   ├── sqlmap.yaml          # sqlmap工具配置
│   ├── nikto.yaml           # nikto工具配置
│   ├── dirb.yaml            # dirb工具配置
│   ├── exec.yaml            # 系统命令执行工具配置
│   └── README.md            # 工具配置说明
├── web/                     # Web前端
│   ├── static/              # 静态资源
│   │   ├── css/
│   │   │   └── style.css    # 样式文件
│   │   └── js/
│   │       └── app.js       # 前端JavaScript逻辑
│   └── templates/           # HTML模板
│       └── index.html       # 主页面模板
├── data/                    # 数据目录（自动创建）
│   └── conversations.db     # SQLite数据库文件
├── config.yaml              # 主配置文件
├── go.mod                   # Go模块依赖
├── go.sum                   # Go依赖校验和
├── run.sh                   # 启动脚本
└── README.md                # 项目说明文档
```

## 快速开始

### 前置要求

- Go 1.21 或更高版本
- OpenAI API Key（或其他兼容OpenAI协议的API，如DeepSeek、Claude等）
- 安全工具（可选）：根据您的需求安装相应的安全工具，系统支持上百个工具

### 安装步骤

1. **克隆项目**
```bash
git clone <repository-url>
cd CyberStrikeAI-main
```

2. **安装依赖**
```bash
go mod download
```

3. **配置**
编辑 `config.yaml` 文件，设置您的API配置：

```yaml
# OpenAI兼容API配置（支持OpenAI、DeepSeek、Claude等）
openai:
  api_key: "sk-your-api-key-here"  # 替换为您的API Key
  base_url: "https://api.openai.com/v1"  # 或使用其他兼容API的地址
  model: "gpt-4"  # 或 "deepseek-chat", "gpt-3.5-turbo" 等

# 服务器配置
server:
  host: "0.0.0.0"
  port: 8080

# 数据库配置
database:
  path: "data/conversations.db"

# 安全工具配置
security:
  tools_dir: "tools"  # 工具配置文件目录
```

**支持的API服务商：**
- OpenAI: `https://api.openai.com/v1`
- DeepSeek: `https://api.deepseek.com/v1`
- 其他兼容OpenAI协议的API服务

4. **安装安全工具（可选）**

根据您的需求安装相应的安全工具。系统支持上百个工具，您可以根据实际需要选择性安装：

```bash
# macOS (使用Homebrew)
brew install nmap sqlmap nuclei httpx gobuster feroxbuster subfinder amass

# Ubuntu/Debian
sudo apt-get install nmap sqlmap nuclei httpx gobuster feroxbuster

# 或使用Docker运行工具
# 或使用各工具的官方安装方法
```

**注意**：不是所有工具都需要安装，AI会根据您的测试需求自动选择可用的工具。如果某个工具未安装，AI会尝试使用替代工具。

5. **启动服务器**

#### 方式一：使用启动脚本（推荐）
```bash
chmod +x run.sh
./run.sh
```

#### 方式二：直接运行
```bash
go run cmd/server/main.go
```

#### 方式三：编译后运行
```bash
go build -o cyberstrike-ai cmd/server/main.go
./cyberstrike-ai
```

#### 方式四：指定配置文件
```bash
go run cmd/server/main.go -config /path/to/config.yaml
```

6. **访问应用**
打开浏览器访问：http://localhost:8080

您将看到：
- **对话测试** - 与AI对话进行渗透测试
- **工具监控** - 查看工具执行状态和结果
- **对话历史** - 管理历史对话记录

## ⚙️ 配置说明

### 完整配置示例

```yaml
# 服务器配置
server:
  host: "0.0.0.0"  # 监听地址
  port: 8080        # HTTP服务端口

# 日志配置
log:
  level: "info"     # 日志级别: debug, info, warn, error
  output: "stdout"  # 输出位置: stdout, stderr, 或文件路径

# MCP协议配置
mcp:
  enabled: true     # 是否启用MCP服务器
  host: "0.0.0.0"   # MCP服务器监听地址
  port: 8081        # MCP服务器端口

# AI模型配置（支持OpenAI兼容API）
openai:
  api_key: "sk-xxx"  # API密钥
  base_url: "https://api.deepseek.com/v1"  # API基础URL
  model: "deepseek-chat"  # 模型名称

# 数据库配置
database:
  path: "data/conversations.db"  # SQLite数据库路径

# 安全工具配置
security:
  # 推荐方式：从目录加载工具配置
  tools_dir: "tools"  # 工具配置文件目录（相对于配置文件所在目录）
  
  # 向后兼容：也可以在主配置文件中直接定义工具
  # tools:
  #   - name: "nmap"
  #     command: "nmap"
  #     args: ["-sT", "-sV", "-sC"]
  #     description: "网络扫描工具"
  #     enabled: true
```

### 工具配置方式

**方式一：使用工具目录（推荐）**

在 `tools/` 目录下为每个工具创建独立的YAML配置文件，例如 `tools/nmap.yaml`：

```yaml
name: "nmap"
command: "nmap"
args: ["-sT", "-sV", "-sC"]
enabled: true

short_description: "网络扫描工具，用于发现网络主机、开放端口和服务"

description: |
  网络映射和端口扫描工具，用于发现网络中的主机、服务和开放端口。

parameters:
  - name: "target"
    type: "string"
    description: "目标IP地址或域名"
    required: true
    position: 0
    format: "positional"
  
  - name: "ports"
    type: "string"
    description: "端口范围，例如: 1-1000"
    required: false
    flag: "-p"
    format: "flag"
```

**方式二：在主配置文件中定义**

直接在 `config.yaml` 的 `security.tools` 中定义工具配置。

**注意：** 如果同时配置了 `tools_dir` 和 `tools`，`tools_dir` 中的工具优先。

## 🚀 使用示例

### 对话式渗透测试

在Web界面的"对话测试"标签页中，您可以使用自然语言与AI对话：

#### 1. 网络扫描
```
扫描 192.168.1.1 的开放端口
```
或更详细的指令：
```
对 192.168.1.1 进行全面的端口扫描，重点关注80、443、22、21端口
```

#### 2. SQL注入检测
```
检测 https://example.com/page?id=1 是否存在SQL注入漏洞
```

#### 3. Web漏洞扫描
```
扫描 https://example.com 的Web服务器漏洞，包括常见的安全问题
```

#### 4. 目录扫描
```
扫描 https://example.com 的隐藏目录和文件
```

#### 5. 综合安全测试
```
对 example.com 进行全面的安全评估，包括端口扫描、Web漏洞检测和目录枚举
```

#### 6. 多步骤测试
```
首先扫描 192.168.1.1 的开放端口，然后对发现的Web服务进行漏洞扫描
```

### 后渗透测试

在获得初始访问权限后，可以使用后渗透工具进行权限提升、横向移动和持久化：

#### 1. Linux 权限提升枚举
```
使用 linpeas 对目标 Linux 系统进行权限提升检查
```

#### 2. Windows 权限提升枚举
```
使用 winpeas 对目标 Windows 系统进行权限提升检查
```

#### 3. Active Directory 攻击路径分析
```
使用 bloodhound 分析 Active Directory 的攻击路径
```

#### 4. 凭证提取
```
使用 mimikatz 提取 Windows 系统的凭证信息
```

#### 5. 横向移动
```
使用 impacket 工具集进行网络协议攻击和横向移动
```

#### 6. 后门生成
```
使用 msfvenom 生成反向 shell 载荷
```

### CTF 竞赛支持

系统内置了丰富的 CTF 工具，支持各种 CTF 题型的解题：

#### 1. 隐写分析
```
使用 stegsolve 分析图片隐写
使用 zsteg 检测 LSB 隐写
```

#### 2. 密码破解
```
使用 hashcat 破解哈希值
使用 john 破解密码文件
使用 fcrackzip 破解 ZIP 文件密码
使用 pdfcrack 破解 PDF 文件密码
```

#### 3. 二进制分析
```
使用 gdb 调试二进制文件
使用 radare2 进行逆向分析
使用 strings 提取二进制文件中的字符串
```

#### 4. 哈希识别
```
使用 hash-identifier 识别哈希类型
```

#### 5. 数据转换和分析
```
使用 cyberchef 进行各种数据转换和分析
使用 xxd 查看文件十六进制内容
```

#### 6. 综合 CTF 解题
```
分析这个 CTF 题目：给定一个包含隐写和加密的文件，找出 flag
```

### 监控工具执行

在"工具监控"标签页中，您可以：

- 📊 **执行统计** - 查看所有工具的调用次数、成功/失败统计
- 📝 **执行记录** - 查看详细的工具执行历史，包括参数、结果、耗时
- 🔍 **漏洞列表** - 自动提取和分类发现的漏洞
- ⏱️ **实时状态** - 实时查看正在执行的工具状态

### 对话历史管理

- 📚 **查看历史** - 浏览所有历史对话记录
- 🔍 **搜索对话** - 根据标题搜索对话
- 🗑️ **删除对话** - 清理不需要的对话记录
- 📄 **查看详情** - 查看对话的完整消息和工具执行过程

## 效果

### 对话效果
  ![详情预览](./img/效果1.png)
  ![详情预览](./img/效果2.png)

### MCP调用
  ![详情预览](./img/MCP.png)

### 调用链
  ![详情预览](./img/调用链1.png)
  

## 📡 API接口

### Agent Loop API

#### 标准请求（同步）

**POST** `/api/agent-loop`

请求体：
```json
{
  "message": "扫描 192.168.1.1",
  "conversationId": "optional-conversation-id"  // 可选，用于继续对话
}
```

响应：
```json
{
  "response": "AI的回复内容",
  "mcpExecutionIds": ["exec-id-1", "exec-id-2"],
  "conversationId": "conversation-id",
  "time": "2024-01-01T00:00:00Z"
}
```

使用示例：
```bash
curl -X POST http://localhost:8080/api/agent-loop \
  -H "Content-Type: application/json" \
  -d '{"message": "扫描 192.168.1.1"}'
```

#### 流式请求（推荐，实时输出）

**POST** `/api/agent-loop/stream`

使用Server-Sent Events (SSE)实时返回执行过程。

请求体：
```json
{
  "message": "扫描 192.168.1.1",
  "conversationId": "optional-conversation-id"
}
```

事件类型：
- `progress` - 进度更新
- `iteration` - 迭代开始
- `thinking` - AI思考内容
- `tool_call` - 工具调用开始
- `tool_result` - 工具执行结果
- `response` - 最终回复
- `error` - 错误信息
- `done` - 完成

使用示例（JavaScript）：
```javascript
const eventSource = new EventSource('/api/agent-loop/stream', {
  method: 'POST',
  body: JSON.stringify({ message: '扫描 192.168.1.1' })
});

eventSource.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log(data.type, data.message, data.data);
};
```

### 对话历史API

#### 创建对话

**POST** `/api/conversations`

请求体：
```json
{
  "title": "对话标题"
}
```

#### 获取对话列表

**GET** `/api/conversations`

查询参数：
- `limit` - 限制返回数量（可选）
- `offset` - 偏移量（可选）

#### 获取单个对话

**GET** `/api/conversations/:id`

返回对话的完整信息，包括所有消息。

#### 删除对话

**DELETE** `/api/conversations/:id`

### 监控API

#### 获取所有监控信息

**GET** `/api/monitor`

返回所有执行记录、统计信息和漏洞列表。

#### 获取特定执行记录

**GET** `/api/monitor/execution/:id`

返回指定ID的工具执行详情。

#### 获取统计信息

**GET** `/api/monitor/stats`

返回所有工具的调用统计：
```json
{
  "nmap": {
    "toolName": "nmap",
    "totalCalls": 10,
    "successCalls": 9,
    "failedCalls": 1,
    "lastCallTime": "2024-01-01T00:00:00Z"
  }
}
```

#### 获取漏洞列表

**GET** `/api/monitor/vulnerabilities`

返回所有发现的漏洞：
```json
{
  "total": 5,
  "severityCount": {
    "critical": 0,
    "high": 2,
    "medium": 2,
    "low": 1
  },
  "vulnerabilities": [...]
}
```

### MCP协议接口

**POST** `/api/mcp`

MCP协议端点，支持JSON-RPC 2.0格式的请求。

## 🔌 MCP协议

本项目完整实现了MCP（Model Context Protocol）协议，支持以下功能：

### 支持的方法

- `initialize` - 初始化连接，协商协议版本和功能
- `tools/list` - 列出所有可用的工具
- `tools/call` - 调用指定工具并执行
- `prompts/list` - 列出可用的提示词模板
- `prompts/get` - 获取提示词模板内容
- `resources/list` - 列出可用资源
- `resources/read` - 读取资源内容
- `sampling/request` - 采样请求（占位实现）

### 工具执行机制

- 工具调用是同步执行的，确保错误能正确返回
- 每个工具调用都会创建执行记录，包含：
  - 执行ID（唯一标识）
  - 工具名称和参数
  - 执行状态（running, completed, failed）
  - 开始和结束时间
  - 执行结果或错误信息
- 系统自动跟踪所有工具的执行统计信息

### MCP协议使用示例

#### 初始化连接

```bash
curl -X POST http://localhost:8080/api/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": "1",
    "method": "initialize",
    "params": {
      "protocolVersion": "2024-11-05",
      "capabilities": {},
      "clientInfo": {
        "name": "test-client",
        "version": "1.0.0"
      }
    }
  }'
```

#### 列出工具

```bash
curl -X POST http://localhost:8080/api/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": "2",
    "method": "tools/list"
  }'
```

#### 调用工具

```bash
curl -X POST http://localhost:8080/api/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": "3",
    "method": "tools/call",
    "params": {
      "name": "nmap",
      "arguments": {
        "target": "192.168.1.1",
        "ports": "1-1000"
      }
    }
  }'
```

## 🛠️ 安全工具支持

### 工具概览

当前系统集成了 **上百个安全工具**，涵盖以下类别：

- **网络扫描工具** - nmap, masscan, rustscan, arp-scan, nbtscan 等
- **Web应用扫描** - sqlmap, nikto, dirb, gobuster, feroxbuster, ffuf, httpx 等
- **漏洞扫描** - nuclei, wpscan, wafw00f, dalfox, xsser 等
- **子域名枚举** - subfinder, amass, findomain, dnsenum, fierce 等
- **API安全** - graphql-scanner, arjun, api-fuzzer, api-schema-analyzer 等
- **容器安全** - trivy, clair, docker-bench-security, kube-bench, kube-hunter 等
- **云安全** - prowler, scout-suite, cloudmapper, pacu, terrascan, checkov 等
- **二进制分析** - gdb, radare2, ghidra, objdump, strings, binwalk 等
- **漏洞利用** - metasploit, msfvenom, pwntools, ropper, ropgadget 等
- **密码破解** - hashcat, john, hashpump 等
- **取证分析** - volatility, volatility3, foremost, steghide, exiftool 等
- **后渗透工具** - linpeas, winpeas, mimikatz, bloodhound, impacket, responder 等
- **CTF工具** - stegsolve, zsteg, hash-identifier, fcrackzip, pdfcrack, cyberchef 等
- **系统工具** - exec, create-file, delete-file, list-files, modify-file 等

### 主要工具示例

- **nmap** - 网络端口扫描和服务识别
  - 功能：主机发现、端口扫描、服务版本检测、操作系统识别
  - 配置：`tools/nmap.yaml`
  
- **sqlmap** - SQL注入自动化检测和利用工具
  - 功能：自动检测SQL注入漏洞、数据库指纹识别、数据提取
  - 配置：`tools/sqlmap.yaml`
  
- **nuclei** - 快速漏洞扫描器
  - 功能：基于模板的漏洞扫描、大规模扫描支持
  - 配置：`tools/nuclei.yaml`
  
- **httpx** - HTTP探测工具
  - 功能：HTTP/HTTPS探测、状态码检测、标题提取
  - 配置：`tools/httpx.yaml`
  
- **exec** - 系统命令执行工具
  - 功能：执行任意系统命令（需谨慎使用）
  - 配置：`tools/exec.yaml`
  - ⚠️ 警告：此工具可以执行任意命令，请确保安全使用

完整的工具列表请查看 `tools/TOOLS_LIST.md` 文件。

### 添加新工具

#### 方法一：创建工具配置文件（推荐）

1. 在 `tools/` 目录下创建新的YAML文件，例如 `tools/mytool.yaml`：

```yaml
name: "mytool"
command: "mytool"
args: ["--default-arg"]
enabled: true

short_description: "简短描述（用于工具列表）"

description: |
  工具的详细描述，帮助AI理解工具的用途和使用场景。

parameters:
  - name: "target"
    type: "string"
    description: "目标参数描述"
    required: true
    position: 0  # 位置参数
    format: "positional"
  
  - name: "option"
    type: "string"
    description: "选项参数描述"
    required: false
    flag: "--option"
    format: "flag"
```

2. 重启服务器，工具会自动加载。

#### 方法二：在主配置文件中添加

在 `config.yaml` 的 `security.tools` 中添加工具配置。

### 工具参数配置

工具参数支持以下格式：

- **positional** - 位置参数，按顺序添加到命令中
- **flag** - 标志参数，格式为 `--flag value` 或 `-f value`
- **combined** - 组合格式，格式为 `--flag=value`
- **template** - 模板格式，使用自定义模板字符串

参数类型支持：
- `string` - 字符串
- `int` / `integer` - 整数
- `bool` / `boolean` - 布尔值
- `array` - 数组（自动转换为逗号分隔的字符串）

## 🔧 故障排除

### API连接问题

**问题：无法连接到OpenAI API**

- ✅ 检查API Key是否正确配置在 `config.yaml` 中
- ✅ 检查网络连接是否正常
- ✅ 验证 `base_url` 配置是否正确
- ✅ 确认API服务商是否支持您使用的模型
- ✅ 检查API配额和余额是否充足

**问题：API返回401或403错误**

- ✅ 验证API Key是否有效
- ✅ 检查API Key是否有权限访问指定模型
- ✅ 确认API服务商的访问限制

### 工具执行问题

**问题：工具执行失败或找不到命令**

- ✅ 确保已安装相应的安全工具：
  ```bash
  # 检查工具是否安装
  which nmap sqlmap nikto dirb
  ```
- ✅ 检查工具是否在系统PATH中
- ✅ 某些工具可能需要root权限（如nmap的SYN扫描）
- ✅ 查看服务器日志获取详细错误信息

**问题：工具执行超时**

- ✅ 某些工具（如nmap全端口扫描）可能需要较长时间
- ✅ 检查网络连接和目标响应
- ✅ 考虑使用更小的扫描范围

**问题：工具参数错误**

- ✅ 检查工具配置文件中的参数定义
- ✅ 查看工具执行日志中的实际命令
- ✅ 参考工具的官方文档验证参数格式

### 服务器问题

**问题：前端无法加载**

- ✅ 检查服务器是否正常运行：
  ```bash
  curl http://localhost:8080
  ```
- ✅ 检查端口8080是否被占用：
  ```bash
  lsof -i :8080
  ```
- ✅ 查看浏览器控制台错误信息
- ✅ 检查防火墙设置

**问题：数据库错误**

- ✅ 确保 `data/` 目录有写权限
- ✅ 检查数据库文件是否损坏
- ✅ 删除数据库文件让系统重新创建（会丢失历史数据）

### 配置问题

**问题：工具未加载**

- ✅ 检查 `tools_dir` 配置是否正确
- ✅ 验证工具配置文件格式是否正确（YAML语法）
- ✅ 查看启动日志中的工具加载信息
- ✅ 确保工具配置中的 `enabled: true`

**问题：MCP服务器无法启动**

- ✅ 检查MCP端口（默认8081）是否被占用
- ✅ 验证MCP配置中的 `enabled: true`
- ✅ 查看日志中的MCP服务器启动信息

### 日志调试

启用详细日志：
```yaml
log:
  level: "debug"  # 改为debug级别
  output: "stdout"
```

查看实时日志：
```bash
# 如果使用run.sh
./run.sh | tee cyberstrike.log

# 或直接运行
go run cmd/server/main.go 2>&1 | tee cyberstrike.log
```

## 安全注意事项

⚠️ **重要提示**：

- 仅对您拥有或已获得授权的系统进行测试
- 遵守相关法律法规
- 建议在隔离的测试环境中使用
- 不要在生产环境中使用
- 某些安全工具可能需要root权限

## 💻 开发指南

### 项目架构

```
┌─────────────┐
│   Web UI    │  ← 用户界面（HTML/CSS/JS）
└──────┬──────┘
       │ HTTP
┌──────▼─────────────────────────────────────┐
│         Gin HTTP Server                    │
│  ┌──────────────────────────────────────┐  │
│  │  Handlers (agent, conversation, etc) │  │
│  └──────┬───────────────────────────────┘  │
│         │                                   │
│  ┌──────▼──────────┐  ┌─────────────────┐ │
│  │  Agent Module   │  │  MCP Server     │ │
│  │  (AI Loop)      │◄─┤  (Tool Manager) │ │
│  └──────┬──────────┘  └────────┬────────┘ │
│         │                      │           │
│  ┌──────▼──────────┐  ┌────────▼────────┐ │
│  │  OpenAI API     │  │ Security Executor│ │
│  └─────────────────┘  └────────┬────────┘ │
│                                │           │
│                       ┌────────▼────────┐  │
│                       │  Security Tools │  │
│                       │ (nmap, sqlmap)  │  │
│                       └─────────────────┘  │
└─────────────────────────────────────────────┘
         │
┌────────▼────────┐
│  SQLite Database│  ← 对话历史和消息存储
└─────────────────┘
```

### 核心模块说明

- **Agent模块** (`internal/agent/agent.go`)
  - 实现Agent Loop，处理AI对话和工具调用决策
  - 支持多轮对话和工具调用链
  - 处理工具执行错误和重试逻辑

- **MCP Server** (`internal/mcp/server.go`)
  - 实现MCP协议服务器
  - 管理工具注册和调用
  - 跟踪工具执行状态和统计

- **Security Executor** (`internal/security/executor.go`)
  - 执行安全工具命令
  - 构建工具参数
  - 解析工具输出

- **Database** (`internal/database/`)
  - SQLite数据库操作
  - 对话和消息管理
  - 过程详情存储

### 添加新工具

#### 推荐方式：使用工具配置文件

1. 在 `tools/` 目录创建工具配置文件（如 `tools/mytool.yaml`）
2. 定义工具参数和描述
3. 重启服务器，工具自动加载

#### 高级方式：自定义参数构建逻辑

如果工具需要特殊的参数处理，可以在 `internal/security/executor.go` 的 `buildCommandArgs` 方法中添加：

```go
case "mytool":
    // 自定义参数构建逻辑
    if target, ok := args["target"].(string); ok {
        cmdArgs = append(cmdArgs, "--target", target)
    }
```

### 构建和部署

#### 本地构建

```bash
go build -o cyberstrike-ai cmd/server/main.go
```

#### 交叉编译

```bash
# Linux
GOOS=linux GOARCH=amd64 go build -o cyberstrike-ai-linux cmd/server/main.go

# macOS
GOOS=darwin GOARCH=amd64 go build -o cyberstrike-ai-macos cmd/server/main.go

# Windows
GOOS=windows GOARCH=amd64 go build -o cyberstrike-ai.exe cmd/server/main.go
```

#### Docker部署（示例）

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o cyberstrike-ai cmd/server/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates nmap sqlmap nikto dirb
WORKDIR /root/
COPY --from=builder /app/cyberstrike-ai .
COPY --from=builder /app/config.yaml .
COPY --from=builder /app/tools ./tools
COPY --from=builder /app/web ./web
EXPOSE 8080
CMD ["./cyberstrike-ai"]
```

### 代码贡献

欢迎提交Issue和Pull Request！

贡献指南：
1. Fork本项目
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启Pull Request

## 📋 技术栈

- **后端框架**: Gin (Go Web Framework)
- **数据库**: SQLite3
- **日志**: Zap (Uber's structured logging)
- **配置**: YAML
- **协议**: MCP (Model Context Protocol)
- **前端**: HTML/CSS/JavaScript (原生，无框架依赖)

## 🔐 安全注意事项

⚠️ **重要提示**：

- ⚠️ **仅对您拥有或已获得授权的系统进行测试**
- ⚠️ **遵守相关法律法规和道德准则**
- ⚠️ **建议在隔离的测试环境中使用**
- ⚠️ **不要在生产环境中使用**
- ⚠️ **某些安全工具可能需要root权限，请谨慎使用**
- ⚠️ **exec工具可以执行任意系统命令，存在安全风险**
- ⚠️ **妥善保管API密钥，不要提交到代码仓库**

## ⚙️ 高级特性

### AI迭代机制

- **最大迭代次数**：系统支持最多30轮AI迭代，确保复杂测试任务能够完成
- **智能总结**：当达到最大迭代次数时，AI会自动总结所有测试结果、发现的问题和已完成的工作
- **下一步计划**：如果测试未完成，AI会提供详细的下一步执行计划，指导后续测试

### 工具执行优化

- **错误处理**：工具执行失败时，AI会自动分析错误原因并尝试替代方案
- **参数优化**：AI会根据目标特征自动优化工具参数，提高测试效率
- **结果分析**：自动分析工具输出，提取关键信息和漏洞

## 📄 许可证

本项目仅供学习和研究使用。

## 🤝 贡献

欢迎提交Issue和Pull Request！

如果您发现bug或有功能建议，请：
1. 查看现有的Issues，避免重复
2. 创建详细的Issue描述问题或建议
3. 提交Pull Request时请包含清晰的说明

## 📞 支持

如有问题或需要帮助，请：
- 查看本文档的故障排除部分
- 查看项目的Issues
- 提交新的Issue描述您的问题

## 🙏 致谢

感谢所有贡献者和开源社区的支持！
