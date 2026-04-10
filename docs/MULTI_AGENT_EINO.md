# Eino 多代理改造说明（DeepAgent）

本文档记录 **单 Agent（原有 ReAct）** 与 **多 Agent（CloudWeGo Eino `adk/prebuilt/deep`）** 并存的改造范围、进度与后续事项。

## 总体结论

- **改造已可用于生产试验**：流式对话、MCP 工具桥接、配置开关、前端模式切换均已落地。
- **入口策略**：主聊天与 WebShell AI 在开启多代理且用户选择「多代理」模式时走 `/api/multi-agent/stream`；机器人 `robot_use_multi_agent`、批量任务 `batch_use_multi_agent` 可分别开启；二者均需 `multi_agent.enabled`。

## 已完成项

| 项 | 说明 |
|----|------|
| 依赖与代理 | `go.mod` 直接依赖 `github.com/cloudwego/eino`、`eino-ext/.../openai`；`go.mod` 注释与 `scripts/bootstrap-go.sh` 指导 **GOPROXY**（如 `https://goproxy.cn,direct`）。 |
| 配置 | `config.yaml` → `multi_agent`：`enabled`、`default_mode`、`robot_use_multi_agent`、`max_iteration`、`sub_agents`（含可选 `bind_role`）等；结构体见 `internal/config/config.go`。 |
| Markdown 子代理 / 主代理 | **常规用法**：在 `agents_dir`（默认 `agents/`）下放 `*.md`（front matter + 正文）。**子代理**供 Deep `task` 调度；**主代理**为 `orchestrator.md` 或 `kind: orchestrator` 的单个文件，定义协调者 `description` / 系统提示（正文空则回退 `orchestrator_instruction` / Eino 默认）。可选：`multi_agent.sub_agents` 与目录合并（同 id 时 Markdown 覆盖）。管理：**Agents → Agent管理**；API：`/api/multi-agent/markdown-agents*`。 |
| MCP 桥 | `internal/einomcp`：`ToolsFromDefinitions` + 会话 ID 持有者，执行走 `Agent.ExecuteMCPToolForConversation`。 |
| 编排 | `internal/multiagent/runner.go`：`deep.New` + 子 `ChatModelAgent` + `adk.NewRunner`（`EnableStreaming: true`），事件映射为现有 SSE `tool_call` / `response_delta` 等。 |
| HTTP | `POST /api/multi-agent`（非流式）、`POST /api/multi-agent/stream`（SSE）；路由**常注册**，是否可用由运行时 `multi_agent.enabled` 决定（流式未启用时 SSE 内 `error` + `done`）。 |
| 会话准备 | `internal/handler/multi_agent_prepare.go`：`prepareMultiAgentSession`（含 **WebShell** `CreateConversationWithWebshell`、工具白名单与单代理一致）。 |
| 单 Agent | `internal/agent` 增加 `ToolsForRole`、`ExecuteMCPToolForConversation`；原 `/api/agent-loop` 未删改语义。 |
| 前端 | 主聊天：`multi_agent.enabled` 时显示「模式」下拉；WebShell AI 与主聊天共用 `localStorage` 键 `cyberstrike-chat-agent-mode`。设置页可写 `multi_agent` 标量到 YAML。 |
| 流式兼容 | 与 `/api/agent-loop/stream` 共用 `handleStreamEvent`：`conversation`、`progress`、`response_start` / `response_delta`、`thinking` / `thinking_stream_*`（模型 `ReasoningContent`）、`tool_*`、`response`、`done` 等；`tool_result` 带 `toolCallId` 与 `tool_call` 联动；`data.mcpExecutionIds` 与进度 i18n 已对齐。 |
| 批量任务 | `batch_use_multi_agent: true` 时 `executeBatchQueue` 中每子任务调用 `RunDeepAgent`（`roleTools` 沿用队列角色；Eino 路径不注入 `roleSkills` 系统提示，与 Web 多代理会话一致）。 |
| 配置 API | `GET /api/config` 返回 `multi_agent: { enabled, default_mode, robot_use_multi_agent, sub_agent_count }`；`PUT /api/config` 可更新前三项（不覆盖 `sub_agents`）。 |
| OpenAPI | 多代理路径说明已更新（流式未启用为 SSE 错误事件）。 |
| 机器人 | `ProcessMessageForRobot` 在 `enabled && robot_use_multi_agent` 时调用 `multiagent.RunDeepAgent`。 |

## 进行中 / 待办（ backlog ）

| 优先级 | 项 | 说明 |
|--------|----|------|
| P3 | **观测与计费** | Eino 事件可进一步打结构化日志 / trace id，便于排障。 |
| P3 | **测试** | 增加 `internal/multiagent` 与 einomcp 的集成测试（mock model 或录屏回放）。 |

## 关键文件索引

- `internal/multiagent/runner.go` — DeepAgent 组装与事件循环  
- `internal/handler/multi_agent.go` — SSE 与（同步）HTTP  
- `internal/handler/multi_agent_prepare.go` — 会话准备（含 WebShell）  
- `internal/einomcp/` — MCP → Eino Tool  
- `config.yaml` — `multi_agent` 示例块  
- `web/static/js/chat.js` — 模式选择与 stream URL  
- `web/static/js/webshell.js` — WebShell AI 流式 URL 与主聊天模式对齐  
- `web/static/js/settings.js` — 多代理标量保存  

## 版本记录

| 日期 | 说明 |
|------|------|
| 2026-03-22 | 首版：Eino DeepAgent + stream + 前端开关 + GOPROXY 脚本。 |
| 2026-03-22 | 补充：进度文档、`prepareMultiAgentSession` 抽取、WebShell 后端对齐、`POST /api/multi-agent`、OpenAPI `/api/multi-agent*` 条目。 |
| 2026-03-22 | 路由常注册、流式未启用 SSE 错误、`robot_use_multi_agent`、设置页持久化、WebShell/机器人多代理、`bind_role` 子代理 Skills/tools。 |
| 2026-03-22 | `tool_result.toolCallId`、`ReasoningContent`→思考流、`batch_use_multi_agent` 与批量队列 Eino 执行。 |
| 2026-03-22 | 流式工具事件：按稳定签名去重，避免每 chunk 刷屏与「未知工具」；最终回复去重相同段落；内置调度显示为 `task`。 |
| 2026-03-22 | `agents/*.md` 子代理定义、`agents_dir`、合并进 `RunDeepAgent`、前端 Agents 菜单与 CRUD API。 |
| 2026-03-22 | `orchestrator.md` / `kind: orchestrator` 主代理、列表主/子标记、与 `orchestrator_instruction` 优先级。 |
