# 工具执行治理

[返回中文文档](README.md)

本文说明 CyberStrikeAI 对长时间工具、MCP 阻塞、大输出、取消和恢复上下文的治理策略。目标是让 Agent 保持标准工具语义，同时避免工具卡死、上下文爆炸、数据库膨胀或恢复时重新注入历史大输出。

## 设计目标

- **Agent 不被工具绑死**：工具调用可以很慢，但当前 runner 只等待有限时间。
- **长任务可继续观察**：超时返回 `execution_id`，后续可用 `wait_tool_execution` 多轮等待。
- **用户和 Agent 都能取消**：当前会话结束或用户停止任务时，会取消仍在运行的工具。
- **数据库与 Agent 视图一致**：DB 保存的是 Agent 实际拿到的兜底后结果，不再保存另一份原始大输出。
- **恢复不会撑爆上下文**：续跑使用 model-facing trace；历史异常大 tool trace 恢复时也会再次裁剪。
- **外部 MCP 有隔离保护**：按 server 限并发、按全局限并发，并对连续失败的 server 熔断。

## 执行模型

普通工具调用仍然对 Eino/Agent 表现为一次标准 tool call，但底层执行分为两段：

```text
Agent 调用工具
  -> ExecutionService 创建 execution
  -> worker 执行真实 MCP/工具调用
  -> Agent bounded wait
       -> 完成：返回工具结果
       -> 未完成：返回 execution_id，worker 继续后台运行
```

这解决了 MCP server、`exec`、`sqlmap`、`nmap`、`nuclei` 等长任务阻塞当前 runner 的问题。

## 工具状态语义

| 状态 | 含义 |
|---|---|
| `queued` | execution 已创建，等待 worker 或并发槽位 |
| `running` | worker 正在执行 |
| `background_running` | 前端展示状态，表示本轮 Agent 已停止等待，但后台仍在跑 |
| `completed` | 本次 tool call 本身已完成 |
| `failed` | 工具真实失败 |
| `cancelled` | 用户、Agent 或会话清理主动取消 |
| `hard_timeout` | 超过硬超时，被系统终止 |
| `orphaned` | 重启/异常后发现 DB 中仍是 running，但运行时已无对应 worker |

注意：`wait_tool_execution` 到达 `timeout_seconds` 时，如果目标 execution 仍在运行，**这次 wait 调用本身是完成的观察动作**，不是工具执行失败。返回体会说明目标仍为 `running`，前端不应显示为红色失败。

## 控制工具

| 工具 | 用途 |
|---|---|
| `get_tool_execution` | 读取 execution 当前状态 |
| `wait_tool_execution` | 等待指定 execution 一段时间 |
| `cancel_tool_execution` | 主动取消指定 execution |

`get_tool_execution` 与 `wait_tool_execution` 支持返回运行中输出预览：

- `include_partial_output`：是否返回 partial output，默认 `true`。
- `partial_output_max_bytes`：本次返回的尾部预览上限，默认 `4096`，最大 `65536`。

partial output 是“已产生输出的有界预览”，不等同于最终 `result`。最终 `result` 仍只在工具结束时写入 canonical execution 记录；不支持流式输出的工具不会返回 partial 字段。

典型流程：

```text
1. 调用 exec/sqlmap/nmap 等长任务
2. 超过 tool_wait_timeout_seconds 后拿到 execution_id
3. Agent 可继续推理、改用其他工具，或调用 wait_tool_execution
4. 仍未完成时可继续等待，或调用 cancel_tool_execution
```

`tool_wait_timeout_seconds` 适用于内部 MCP、外部 MCP，以及 Eino filesystem 的流式 `execute`。Eino 的 `ls/read_file/write_file/edit_file/glob/grep` 等非流式 filesystem 工具会写入 execution 监控记录，但不作为后台 worker 做软等待续跑。

## 取消和会话清理

- 用户点击“停止任务”时，会取消当前会话仍在运行的工具。
- 会话正常结束后，会批量取消当前会话仍 `running` 的工具。
- “中断并继续”类流程不会做会话级批量取消，以免误杀后续需要等待的 worker。
- 取消只针对当前 conversation 绑定的 execution，不会误杀其他会话的工具。

## 外部 MCP 隔离

外部 MCP 可能因为远端 server 卡住、断连或返回异常而拖慢 Agent。系统提供三层保护：

| 能力 | 配置 | 说明 |
|---|---|---|
| 单 server 并发限制 | `external_mcp_max_concurrent_per_server` | 同一个外部 MCP server 同时运行的工具数 |
| 全局并发限制 | `external_mcp_max_concurrent_total` | 所有外部 MCP 工具总并发 |
| 熔断 | `external_mcp_circuit_failure_threshold` / `external_mcp_circuit_cooldown_seconds` | 单 server 连续失败后短期快速失败，避免反复打坏 server |

推荐默认：

```yaml
agent:
  external_mcp_max_concurrent_per_server: 2
  external_mcp_max_concurrent_total: 16
  external_mcp_circuit_failure_threshold: 3
  external_mcp_circuit_cooldown_seconds: 60
```

## 输出兜底

系统使用 `multi_agent.eino_middleware.reduction_max_length_for_trunc` 作为统一工具结果上限。当前示例配置为 50000 bytes。

```yaml
multi_agent:
  eino_middleware:
    reduction_enable: true
    reduction_max_length_for_trunc: 50000
```

兜底覆盖：

| 渠道 | 行为 |
|---|---|
| Agent 实际拿到的工具结果 | 使用兜底后的 canonical result |
| DB/监控存储 | 保存同一份 canonical result |
| `get_tool_execution` / `wait_tool_execution` | 读取同一份 canonical result |
| Eino `execute` / filesystem 监控记录 | 完成记录前统一兜底 |
| 非流式 `exec` stdout/stderr | 源头 bounded buffer |
| 流式 `exec` stdout/stderr | 推送给前端的累计输出也受上限控制 |
| PTY 执行路径 | 同样受上限控制 |
| 前端详情弹窗 | 额外有 UI 展示截断保护 |

触发上限后，完整输出先写入本地 trunc 文件，Agent 侧只保留计入预算的 `<persisted-output>` 预览（含绝对路径）。因此阈值为 50000 时，上下文文本不会超过该上限。

示例：

```text
<persisted-output>
Output too large (200000). Full output saved to: /path/to/tmp/reduction/conversations/<id>/trunc/<execution_id>
Use read_file with offset/limit to read parts of the file.
Preview (first …):
…

Preview (last …):
…

</persisted-output>
```

当前策略是「全文落盘 + 上下文预览」：超过 `reduction_max_length_for_trunc` 时，完整输出写入本地文件（默认 `tmp/reduction/conversations/<会话ID>/trunc/<execution_id>`），Agent/DB/监控拿到的是带绝对路径的 `<persisted-output>` 预览；可用 `read_file` 按 offset/limit 回读全文。

## DB 与恢复上下文

新执行结果的写入路径如下：

```text
工具完成
  -> NormalizeToolResultForStorage
  -> 写入内存 execution
  -> 写入 DB
  -> 返回给 Agent
```

因此正常情况下，DB 中保存的就是 Agent 拿到的结果。

续跑恢复时，系统使用 `LastAgentTraceInput` 中的 model-facing trace，也就是实际送入 ChatModel 的消息快照，而不是原始事件流累计。恢复入口还会对历史 tool 内容再次应用上限，防止以下情况撑爆上下文：

- 升级前 DB 已经存过原始大输出。
- 手工迁移或导入的数据绕过了当前写入路径。
- 配置从更大阈值改成 50000。
- 未来某条旁路写入漏掉 canonicalize。

## 关键配置建议

长任务场景推荐：

```yaml
agent:
  max_iterations: 800
  tool_timeout_minutes: 60
  tool_wait_timeout_seconds: 30
  external_mcp_max_concurrent_per_server: 2
  external_mcp_max_concurrent_total: 16
  external_mcp_circuit_failure_threshold: 3
  external_mcp_circuit_cooldown_seconds: 60
  shell_no_output_timeout_seconds: 1200

multi_agent:
  eino_middleware:
    reduction_enable: true
    reduction_max_length_for_trunc: 50000
```

参数说明：

| 参数 | 建议 | 说明 |
|---|---:|---|
| `max_iterations` | `300-1000` | 太大等于放弃循环保护 |
| `tool_timeout_minutes` | `60` | 单次工具硬超时，适合 sqlmap 等长任务 |
| `tool_wait_timeout_seconds` | `30-60` | Agent 本轮等待上限，到时返回 `execution_id` |
| `shell_no_output_timeout_seconds` | `600-1200` | 连续无输出时终止，防止静默挂死 |
| `reduction_max_length_for_trunc` | `50000` | 工具结果统一上限 |

不建议把 `tool_wait_timeout_seconds` 设置得很大。长任务应由 worker 后台跑，Agent 通过 `execution_id` 继续观察，而不是一轮等待数分钟。

## 测试建议

可以用以下对话测试长任务语义：

```text
调用 exec 执行 sleep 120；如果超过 10 秒还没完成，不要一直等，告诉我 execution_id，然后调用 wait_tool_execution 等 5 秒；如果仍未完成，再调用 cancel_tool_execution，最后说明状态。
```

可以用以下命令测试大输出兜底：

```text
调用 exec 执行：python3 - <<'PY'
print("A" * 200000)
PY
然后展示工具结果长度和是否包含截断提示。
```

预期：

- 初始长任务会返回 `execution_id`，状态为 `running` 或前端展示 `background_running`。
- `wait_tool_execution` 等待到上限但目标未完成时，本次 wait 调用不应显示为执行失败。
- 大输出结果不会超过 `reduction_max_length_for_trunc`。
- DB、监控详情、Agent 继续推理看到的是同一份兜底结果。

## 当前边界

- 外部 MCP 的远端 server 内部如何采集输出不由 CyberStrikeAI 控制；CyberStrikeAI 会在结果进入本系统后统一兜底、限并发和熔断。
- 超长工具输出会在截断前写入本地 `tmp/reduction/.../trunc/<id>`（或 `reduction_root_dir`），bounded result 中包含可 `read_file` 的绝对路径。
