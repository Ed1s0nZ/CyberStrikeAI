## CyberStrikeAI Burp Suite 插件（中文说明）

### 功能概述

- 在 Burp 的 `CyberStrikeAI` 标签页中配置 **Host、端口、密码、单/多 Agent**
- 点击 **Validate（验证）**：
  - 调用 `POST /api/auth/login` 用密码换取 Token
  - 调用 `GET /api/auth/validate` 校验 Token
  - 验证通过后 Token 会保存在插件内存中（本次 Burp 会话有效）
- 右键任意 HTTP 请求包 → **Send to CyberStrikeAI (stream test)**：
  - 将该 HTTP 请求（含 headers/body；若存在响应则附带截断片段）发送到 CyberStrikeAI
  - 以 **SSE 流式**接收返回内容，并在标签页中实时展示
  - 单 Agent：`POST /api/agent-loop/stream`
  - 多 Agent：`POST /api/multi-agent/stream`（需要服务端启用 `multi_agent.enabled: true`）
- **测试历史侧边栏（可搜索）**：每次发送都会新增一条记录，方便回看与对比
- **Output 分区**：`Progress`（可折叠）+ `Final Response`（主区域）
- **Markdown 渲染**：最终输出可在 Output 主区域渲染为富文本（可开关）
- **Request / Response 回看**：右侧 Tab 可直接查看该次捕获到的原始请求/响应
- **Stop 取消**：任务创建会话后可调用 `/api/agent-loop/cancel` 停止当前会话任务

### 编译（不依赖 Gradle/Maven，推荐）

> 给普通用户：你们应当直接发 **编译好的 jar**，用户在 Burp 里加载即可，**不需要编译**。

#### 方式 A（推荐，通用）：用 Maven 编译（不需要知道 Burp 在哪）

适合：开发者/CI 打包一次，发布给所有用户使用。

环境要求：

- JDK 11+
- Maven（会从 Maven Central 下载 `burp-extender-api` 依赖）

编译打包：

```bash
cd plugins/burp-suite/cyberstrikeai-burp-extension
./build-mvn.sh
```

产物：

- `dist/cyberstrikeai-burp-extension.jar`

#### 方式 B（离线）：纯 JDK 编译（需要 Burp 的 API jar）

- JDK 11+
- Burp Extender API 的 jar（来自你的 Burp 安装目录）

#### 步骤

1) 在插件目录创建 `lib/`，并把 `burp-extender-api.jar` 复制进去：

```bash
cd plugins/burp-suite/cyberstrikeai-burp-extension
mkdir -p lib
# 复制 Burp 自带的 API jar 到这里，例如：
# cp "/path/to/burp-extender-api.jar" lib/
```

2) 一键编译打包：

```bash
cd plugins/burp-suite/cyberstrikeai-burp-extension
./build.sh
```

产物：

- `dist/cyberstrikeai-burp-extension.jar`

### 在 Burp Suite 中加载

- Burp Suite → **Extensions** → **Installed** → **Add**
- Extension type：**Java**
- 选择 `dist/cyberstrikeai-burp-extension.jar`

### 使用方法

1) 打开 Burp 顶部标签页 `CyberStrikeAI`
2) 填写：
   - **Host**：例如 `127.0.0.1`
   - **Port**：例如 `8080`
   - **Password**：你的 CyberStrikeAI 登录密码（对应服务端 `config.yaml` 的 `auth.password`）
   - **Agent mode**：选择 `Single Agent` 或 `Multi Agent`
3) 点击 **Validate**
   - 成功：状态显示 `OK (token saved)`
   - 失败：状态会显示错误原因（例如密码错误、服务不可达、401/403 等）
4) 在 Burp 的 Proxy/HTTP history/Repeater 等列表中选中一条 HTTP 包
5) 右键 → **Send to CyberStrikeAI (stream test)**
6) 每次发送后会在 `CyberStrikeAI` 标签页左侧显示一个“测试记录”（请求标题 + 单/多 Agent + 状态）；点击对应记录即可在右侧查看该次的流式输出结果

### 常见问题（排错）

- **Validate 失败 / 401**
  - 确认密码是否正确（服务端 `auth.password`）
  - 确认 IP/端口是否能访问（例如浏览器能打开 `http://IP:PORT/`）
  - 若服务器启用了反向代理/HTTPS，需要把插件里 baseUrl 改成对应协议与端口（当前插件默认使用 `http://`）

- **选择 Multi Agent 后提示“多代理未启用”**
  - 服务端需要开启：`config.yaml` 中 `multi_agent.enabled: true`
  - 并重启服务（或按你们项目的动态 apply 配置流程启用）

- **右键发送后无流式输出**
  - 先确认已 Validate（拿到 Token）
  - 确认 Burp 能访问到 CyberStrikeAI（网络/代理/防火墙）
  - 服务端的流式端点为 SSE，插件会解析 `data: {json}` 行；如果中间件缓冲可能影响实时性

