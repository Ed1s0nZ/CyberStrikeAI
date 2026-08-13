# LuaN1aoAgent v2 Runtime Contract

只在需要核对角色权限、工具名、TaskResult、图语义、Artifact 或 Docker skill 可见性时读本文件。

## 目录

1. [角色与 skill 可见性](#1-角色与-skill-可见性)
2. [Planner 语言不是 Executor 工具](#2-planner-语言不是-executor-工具)
3. [Executor 可用工具](#3-executor-可用工具)
4. [TaskResult 契约](#4-taskresult-契约)
5. [Artifact 与证据](#5-artifact-与证据)
6. [Scope 是软执行约束](#6-scope-是软执行约束)
7. [Sandbox 与 route](#7-sandbox-与-route)
8. [Skill 加载与 Docker 部署](#8-skill-加载与-docker-部署)
9. [恢复与预算](#9-恢复与预算)

## 1. 角色与 skill 可见性

| 角色 | 职责 | 终止工具 | 项目级 skills |
|---|---|---|---|
| Planner | 读压缩图、创建/修补 Task DAG | `planner_submit` | 不加载 |
| Executor | 执行一个 bounded `TaskEnvelope` | `task_result_submit` | 加载 `.agents/skills/` |
| Supervisor | 监督继续、重定向、checkpoint 或停止 | `control_submit` | 不加载 |
| Projector | 把 observation 投影到 reasoning/operation graph | `graph_delta_submit` | 不加载 |

因此本 skill 只能影响 Executor。它不能直接控制 Planner 或 Projector。

## 2. Planner 语言不是 Executor 工具

`create_tasks`、`patch_task`、`replace_dependencies`、`set_task_status` 和
`set_node_status` 是 `planner_submit.commands[]` 的 `kind` 值。
Executor 不得调用或输出这些命令。

Planner 创建的 Task 必须带：

- 稳定 `task:<id>`；
- `goal`、`targetRefs`、真实 `scopeRef`；
- `constraints`、至少一条 `successCriteria`；
- `priority`，可选 `budget.maxTurns`、依赖和并行组。

本 skill 通过 `task_result_submit` 的阶段结果影响后续规划，而不是修改 DAG。

## 3. Executor 可用工具

以运行时注入的 `<available_tools>` 为准，不要假设全部存在。

| 类别 | 真实工具 |
|---|---|
| Workspace | `read`, `grep`, `find`, `ls`, `bash` |
| 公网情报 | `web_fetch`, `web_search`, `vulnerability_search` |
| 浏览器 | `browser_render` |
| 持久材料 | `artifact_read`, `artifact_write` |
| 受管网络 | `route_open`, `route_status`, `route_stop`, `route_reconnect` |
| 终止提交 | `task_result_submit` |

不存在 CyberStrikeAI 的 `write_todos`、`upsert_project_fact`、
`record_vulnerability` 或泛化 `batch_task` 契约。不要调用它们。

## 4. TaskResult 契约

`task_result_submit` 接受：

- `taskId`
- `status`: `completed | partial | failed`
- `summary`
- `evidenceRefs[]`
- `artifactRefs[]`
- 可选 `capabilityRefs[]`
- 可选 `blockerReason`
- 可选 `suggestedNextGoal`
- 可选 `checkpointReason`
- 可选 `retryable`

注意：TypeScript 的内部 `TaskResultStatus` 还定义了 `blocked`，但当前
`task_result_submit` schema 不接受它。Executor 必须使用上述三个可提交状态。
`suggestedNextGoal`、`blockerReason` 与 `checkpointReason` 都是字符串，不是嵌套对象。

合法的阶段交接形状：

```json
{
  "taskId": "task:<id>",
  "status": "partial",
  "summary": "已确认事实、精确负面结论、剩余缺口和完整 Ref",
  "evidenceRefs": ["<runtime evidence ref>"],
  "artifactRefs": ["artifact:<full-runtime-ref>"],
  "capabilityRefs": ["<optional runtime capability ref>"],
  "blockerReason": "<optional string>",
  "suggestedNextGoal": "<one bounded next goal with prerequisite refs, scope and acceptance criteria>",
  "checkpointReason": "<optional string>",
  "retryable": false
}
```

### 状态判定

- `completed`：当前 Task 的全部 successCriteria 已满足。
- `partial`：存在可复用、证据支持的阶段成果，但当前 Task 尚未全部完成或需要新因果阶段。
- `failed`：没有可交付的有效成果，或安全/范围门阻止继续。

`blockerReason` 用于表达缺授权、缺凭据、环境不可用等外部条件；
它不改变 status schema。

## 5. Artifact 与证据

- ExecutionLog 与 Artifact Store 是持久化事实来源。
- 大输出应通过 `artifact_write` 持久化；支持 inline text 或导入 Executor 文件。
- 完整 Artifact Ref 形如 Runtime 返回的 `artifact:*`，不得自行缩写。
- Artifact 是材料指针，不自动等于 Evidence。
- Executor 提交 observation 与 Ref；Projector 决定图语义。

Projector 允许的 reasoning node 类型仅为：

- `Evidence`
- `Hypothesis`
- `Vulnerability`
- `Exploit`

允许的 operation node 类型仅为：

- `Host`, `Port`, `Service`, `WebEndpoint`, `Parameter`
- `Credential`, `AgentSession`, `ShellSession`, `Session`
- `File`, `Process`

不要伪造 `Endpoint`、`CausalChain`、`OperationsGraph`、`proxy_route node`
或任意自定义顶层字段。Route 语义由 Runtime observation 与 Projector 处理。

## 6. Scope 与安全约束

Runtime 框架在系统层已统一绑定与约束授权范围 (`scopeRef`) 与安全门禁，Executor 直接在 TaskEnvelope 约束的框架范围内执行动作。



## 7. Sandbox 与 route

- `auto` 优先使用每 Task Docker Executor；显式 `docker` 不可用时 fail closed。
- Docker Executor 以 UID 1000 运行，无 capabilities、只读 root、受限 `/tmp`，
  并使用持久 `/workspace`。
- 普通 TCP 目标流量经同 Task Gateway；UDP/ICMP 仍为直连并记录 telemetry。
- 受管 route 只接受 `ssh` 或 `chisel`，返回稳定 route/connection Ref。
- `route_stop` 保留定义；`route_reconnect` 恢复同一 Ref。
- route 的 CIDR 只表示潜在可达范围，不等于发现或确认主机。

## 8. Skill 加载与 Docker 部署

- 项目技能目录固定为 `<cwd>/.agents/skills`。
- Runtime 在创建或恢复 Executor session 时构造 skill loader；没有文件 watcher。
- 新 skill 对新 Executor session 可见。为避免长生命周期 server/session 使用旧缓存，
  安装或替换后重启 Web server/container。
- 顶层容器通过 Docker socket 再创建 per-Task 容器时，传给宿主 Docker daemon 的
  bind source 必须是 **宿主真实绝对路径**。只把宿主目录挂到顶层容器的 `/app/...`
  不会让宿主自动拥有 `/app/...`。
- 如果子 Executor 报 bind source 不存在，先修正宿主/容器路径映射；
  不要为了绕过问题切到带 Docker socket 的无隔离 `workspace` 模式。

## 9. 恢复与预算

- 同一 Task 跨 epoch 复用持久 Pi session 与 workspace。
- 不同 Task 相互隔离，跨 Task 材料依靠依赖结果和 Artifact Ref。
- Runtime 持有 turn 和 wall-clock 硬预算；skill 不能扩预算。
- `--resume` 恢复原 Goal 与 Scope，不能同时替换它们。
- 接近预算或收到 stop request 时立即收束并提交阶段结果。
