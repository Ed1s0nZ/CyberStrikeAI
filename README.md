# CyberStrikeAI

基于Golang和Gin框架的AI驱动自主渗透测试平台，使用MCP协议集成安全工具。

## 功能特性

- 🤖 **AI代理连接** - 支持Claude、GPT等兼容MCP的AI代理通过FastMCP协议连接
- 🧠 **智能分析** - 决策引擎分析目标并选择最佳测试策略
- ⚡ **自主执行** - AI代理执行全面的安全评估
- 🔄 **实时适应** - 系统根据结果和发现的漏洞进行调整
- 📊 **高级报告** - 可视化方式输出漏洞卡片和风险分析
- 💬 **对话式交互** - 前端以对话形式调用后端agent-loop
- 📈 **实时监控** - 监控安全工具的执行状态、结果、调用次数等

## 项目结构

```
CyberStrikeAI/
├── cmd/
│   └── server/
│       └── main.go          # 程序入口
├── internal/
│   ├── agent/               # AI代理模块
│   ├── app/                 # 应用初始化
│   ├── config/              # 配置管理
│   ├── handler/             # HTTP处理器
│   ├── logger/              # 日志系统
│   ├── mcp/                 # MCP协议实现
│   └── security/            # 安全工具执行器
├── web/
│   ├── static/              # 静态资源
│   │   ├── css/
│   │   └── js/
│   └── templates/           # HTML模板
├── config.yaml              # 配置文件
├── go.mod                   # Go模块文件
└── README.md                # 说明文档
```

## 快速开始

### 前置要求

- Go 1.21 或更高版本
- OpenAI API Key（或其他兼容OpenAI协议的API）
- 安全工具（可选）：nmap, sqlmap, nikto, dirb

### 安装步骤

1. **克隆项目**
```bash
cd /Users/temp/Desktop/wenjian/tools/CyberStrikeAI
```

2. **安装依赖**
```bash
go mod download
```

3. **配置**
编辑 `config.yaml` 文件，设置您的OpenAI API Key：
```yaml
openai:
  api_key: "sk-your-api-key-here"
  base_url: "https://api.openai.com/v1"
  model: "gpt-4"
```

4. **启动服务器**

#### 方式一：使用启动脚本
```bash
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

5. **访问应用**
打开浏览器访问：http://localhost:8080

## 配置说明

### 服务器配置
```yaml
server:
  host: "0.0.0.0"
  port: 8080
```

### MCP配置
```yaml
mcp:
  enabled: true
  host: "0.0.0.0"
  port: 8081
```

### 安全工具配置
```yaml
security:
  tools:
    - name: "nmap"
      command: "nmap"
      args: ["-sV", "-sC"]
      description: "网络扫描工具"
      enabled: true
```

## 使用示例

### 对话式渗透测试

在"对话测试"标签页中，您可以：

1. **网络扫描**
   ```
   扫描 192.168.1.1 的开放端口
   ```

2. **SQL注入检测**
   ```
   检测 https://example.com 的SQL注入漏洞
   ```

3. **Web漏洞扫描**
   ```
   扫描 https://example.com 的Web服务器漏洞
   ```

4. **目录扫描**
   ```
   扫描 https://example.com 的隐藏目录
   ```

### 监控工具执行

在"工具监控"标签页中，您可以：

- 查看所有工具的执行统计
- 查看详细的执行记录
- 查看发现的漏洞列表
- 实时监控工具状态

## API接口

### Agent Loop API

**POST** `/api/agent-loop`

请求体：
```json
{
  "message": "扫描 192.168.1.1"
}
```

使用示例：
```bash
curl -X POST http://localhost:8080/api/agent-loop \
  -H "Content-Type: application/json" \
  -d '{"message": "扫描 192.168.1.1"}'
```

### 监控API

- **GET** `/api/monitor` - 获取所有监控信息
- **GET** `/api/monitor/execution/:id` - 获取特定执行记录
- **GET** `/api/monitor/stats` - 获取统计信息
- **GET** `/api/monitor/vulnerabilities` - 获取漏洞列表

使用示例：
```bash
# 获取所有监控信息
curl http://localhost:8080/api/monitor

# 获取统计信息
curl http://localhost:8080/api/monitor/stats

# 获取漏洞列表
curl http://localhost:8080/api/monitor/vulnerabilities
```

### MCP接口

**POST** `/api/mcp` - MCP协议端点

## MCP协议

本项目实现了MCP（Model Context Protocol）协议，支持：

- `initialize` - 初始化连接
- `tools/list` - 列出可用工具
- `tools/call` - 调用工具

工具调用是异步执行的，系统会跟踪每个工具的执行状态和结果。

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

## 安全工具支持

当前支持的安全工具：
- **nmap** - 网络扫描
- **sqlmap** - SQL注入检测
- **nikto** - Web服务器扫描
- **dirb** - 目录扫描

可以通过修改 `config.yaml` 添加更多工具。

## 故障排除

### 问题：无法连接到OpenAI API

- 检查API Key是否正确
- 检查网络连接
- 检查base_url配置

### 问题：工具执行失败

- 确保已安装相应的安全工具（nmap, sqlmap等）
- 检查工具是否在PATH中
- 某些工具可能需要root权限

### 问题：前端无法加载

- 检查服务器是否正常运行
- 检查端口8080是否被占用
- 查看浏览器控制台错误信息

## 安全注意事项

⚠️ **重要提示**：

- 仅对您拥有或已获得授权的系统进行测试
- 遵守相关法律法规
- 建议在隔离的测试环境中使用
- 不要在生产环境中使用
- 某些安全工具可能需要root权限

## 开发

### 添加新工具

1. 在 `config.yaml` 中添加工具配置
2. 在 `internal/security/executor.go` 的 `buildCommandArgs` 方法中添加参数构建逻辑
3. 在 `internal/agent/agent.go` 的 `getAvailableTools` 方法中添加工具定义

### 构建

```bash
go build -o cyberstrike-ai cmd/server/main.go
```

## 许可证

本项目仅供学习和研究使用。

## 贡献

欢迎提交Issue和Pull Request！
