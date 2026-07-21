# Tool Execution Governance

[Back to English documentation](README.md)

This document describes how CyberStrikeAI governs long-running tools, blocking MCP calls, oversized outputs, cancellation, and context restore. The goal is to preserve standard Agent/Eino tool semantics while preventing tool hangs, context blowups, oversized database records, and unsafe resume behavior.

## Goals

- **Keep the agent runner responsive**: tools may run for a long time, but the current runner waits only for a bounded interval.
- **Allow long tasks to continue**: timeout returns an `execution_id`; later turns can call `wait_tool_execution`.
- **Support cancellation**: users and agents can cancel a running execution.
- **Keep DB and agent views identical**: the database stores the same canonical capped result returned to the agent.
- **Protect resume paths**: resume uses model-facing traces and caps historical oversized tool traces.
- **Isolate external MCP failures**: external MCP servers are protected by per-server concurrency limits, global concurrency limits, and circuit breakers.

## Execution Model

Tool calls still appear to Eino/Agent as standard tool invocations, but the blocking work runs in a worker:

```text
Agent calls tool
  -> ExecutionService creates execution
  -> worker runs the real MCP/tool call
  -> Agent bounded wait
       -> completed: return tool result
       -> still running: return execution_id, worker continues in background
```

This prevents MCP servers, `exec`, `sqlmap`, `nmap`, `nuclei`, and similar long-running tools from binding the current runner indefinitely.

## Execution Statuses

| Status | Meaning |
|---|---|
| `queued` | Execution exists and is waiting for a worker or concurrency slot |
| `running` | Worker is executing |
| `background_running` | UI display state: agent stopped waiting, background worker continues |
| `completed` | This tool call completed |
| `failed` | The tool actually failed |
| `cancelled` | User, agent, or session cleanup cancelled the execution |
| `hard_timeout` | The tool exceeded its hard timeout |
| `orphaned` | A persisted running execution no longer has a runtime worker |

Important: when `wait_tool_execution` reaches `timeout_seconds` and the target execution is still running, the wait call itself is a completed observation, not a failed tool execution.

## Control Tools

| Tool | Purpose |
|---|---|
| `get_tool_execution` | Read current execution state |
| `wait_tool_execution` | Wait for a selected execution for a bounded interval |
| `cancel_tool_execution` | Cancel a selected execution |

Typical flow:

```text
1. Call a long-running tool such as exec/sqlmap/nmap
2. After tool_wait_timeout_seconds, receive execution_id
3. Agent can continue reasoning, use other tools, or call wait_tool_execution
4. If still incomplete, continue waiting or call cancel_tool_execution
```

## Cancellation and Session Cleanup

- User stop cancels running tools for the current conversation.
- Normal session end cancels remaining running tools for the current conversation.
- Interrupt-and-continue style flows do not mass-cancel tools.
- Conversation-scoped cancellation avoids killing tools from other conversations.

## External MCP Isolation

External MCP servers can hang, disconnect, or return failures. CyberStrikeAI uses three protections:

| Capability | Config | Description |
|---|---|---|
| Per-server concurrency | `external_mcp_max_concurrent_per_server` | Max simultaneous calls for one external MCP server |
| Global concurrency | `external_mcp_max_concurrent_total` | Max simultaneous external MCP calls across all servers |
| Circuit breaker | `external_mcp_circuit_failure_threshold` / `external_mcp_circuit_cooldown_seconds` | Temporarily fast-fails a server after repeated failures |

Recommended defaults:

```yaml
agent:
  external_mcp_max_concurrent_per_server: 2
  external_mcp_max_concurrent_total: 16
  external_mcp_circuit_failure_threshold: 3
  external_mcp_circuit_cooldown_seconds: 60
```

## Output Governance

CyberStrikeAI uses `multi_agent.eino_middleware.reduction_max_length_for_trunc` as the unified tool result cap. The example configuration uses 50000 bytes.

```yaml
multi_agent:
  eino_middleware:
    reduction_enable: true
    reduction_max_length_for_trunc: 50000
```

Coverage:

| Channel | Behavior |
|---|---|
| Agent-facing tool result | Uses the canonical capped result |
| DB / monitor storage | Stores the same canonical result |
| `get_tool_execution` / `wait_tool_execution` | Reads the same canonical result |
| Eino `execute` / filesystem monitor records | Capped before completion is persisted |
| Non-streaming `exec` stdout/stderr | Source-side bounded buffer |
| Streaming `exec` stdout/stderr | Streamed UI output is also bounded |
| PTY execution path | Uses the same output cap |
| Frontend detail modal | Has an additional UI display cap |

When the cap is reached, CyberStrikeAI keeps a prefix and appends a truncation marker. The marker is counted inside the configured budget, so a 50000-byte cap does not become `50000 + marker`.

Example:

```text
<bounded output prefix>

...[tool output limit reached: kept 50000 bytes; further output suppressed]...
```

The current strategy is “prefix + marker.” For stronger auditability, a future enhancement can use “head 25k + tail 25k + full-output artifact path.”

## Database and Resume Context

New results are written through this path:

```text
tool completes
  -> NormalizeToolResultForStorage
  -> update in-memory execution
  -> persist to DB
  -> return to Agent
```

So, under normal operation, the DB stores exactly the result returned to the agent.

Resume uses `LastAgentTraceInput`, which is the model-facing trace that actually reached ChatModel, not raw event accumulation. The restore path also caps historical tool content to prevent context blowups from:

- pre-upgrade DB records that contain raw large output,
- manual imports or migrations,
- lowering the configured cap from a larger value,
- future bypasses that accidentally skip canonicalization.

## Recommended Configuration

For long-running security tasks:

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

| Parameter | Recommended | Notes |
|---|---:|---|
| `max_iterations` | `300-1000` | Very large values weaken loop protection |
| `tool_timeout_minutes` | `60` | Hard timeout for tools such as sqlmap |
| `tool_wait_timeout_seconds` | `30-60` | Agent wait bound before returning `execution_id` |
| `shell_no_output_timeout_seconds` | `600-1200` | Kills silent hangs |
| `reduction_max_length_for_trunc` | `50000` | Unified tool result cap |

Do not make `tool_wait_timeout_seconds` very large by default. Long tasks should continue in workers and be observed by `execution_id`, rather than blocking one turn for several minutes.

## Testing

Long-task test prompt:

```text
Call exec to run sleep 120. If it is not done after 10 seconds, do not keep waiting; report execution_id, call wait_tool_execution for 5 seconds, then cancel_tool_execution if still incomplete and report final status.
```

Large-output test prompt:

```text
Call exec to run: python3 - <<'PY'
print("A" * 200000)
PY
Then show the tool result length and whether it contains the truncation marker.
```

Expected behavior:

- The initial long task returns an `execution_id` and status `running` or UI `background_running`.
- `wait_tool_execution` timing out while the target is still running is not displayed as a tool failure.
- Large output never exceeds `reduction_max_length_for_trunc`.
- DB, monitor details, and agent continuation use the same capped result.

## Boundaries

- CyberStrikeAI cannot control how a remote external MCP server collects output internally; it caps results after they enter CyberStrikeAI and protects calls with concurrency limits and circuit breakers.
- Full original output is not automatically written to an artifact path today. If complete audit recovery is required, add spill-to-file and include the artifact path in the bounded result.
