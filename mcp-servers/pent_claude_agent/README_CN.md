# Pent Claude Agent MCP

[English](README.md)

AI 驱动的**渗透测试工程师** MCP 服务。CyberStrikeAI 可指挥 pent_claude_agent 执行渗透测试任务、分析漏洞、进行安全诊断。Agent 内部使用 Claude Agent SDK，可独立配置 MCP、工具等，作为独立的渗透测试工程师运行。

## 工具说明

| 工具 | 说明 |
|------|------|
| `pent_claude_run_pentest_task` | 执行渗透测试任务，Agent 独立执行并返回结果。 |
| `pent_claude_analyze_vulnerability` | 分析漏洞信息并给出修复建议。 |
| `pent_agent_execute` | 执行指定任务，Agent 自动选择工具和方法。 |
| `pent_agent_diagnose` | 对目标（URL、IP、域名）进行安全诊断。 |
| `pent_claude_status` | 获取 pent_claude_agent 的当前状态。 |

## 依赖

- Python 3.10+
- `mcp`、`claude-agent-sdk`、`pyyaml`（使用项目 venv 时已包含；单独运行需：`pip install mcp claude-agent-sdk pyyaml`）

## 配置

Agent 默认使用本目录下的 `pent_claude_agent_config.yaml`。可通过以下方式覆盖：

- 启动 MCP 时传入 `--config /path/to/config.yaml`
- 环境变量 `PENT_CLAUDE_AGENT_CONFIG`

配置项（参见 `pent_claude_agent_config.yaml`）：

- `cwd`: Agent 工作目录
- `allowed_tools`: Agent 可用的工具（Read、Write、Bash、Grep、Glob 等）
- `mcp_servers`: Agent 可挂载的 MCP 服务器（如 reverse_shell）
- `env`: 环境变量（API Key 等）
- `system_prompt`: 角色与行为定义

路径占位符：`${PROJECT_ROOT}` = CyberStrikeAI 项目根目录，`${SCRIPT_DIR}` = 本脚本所在目录。

## 在 CyberStrikeAI 中接入

1. **路径**  
   例如项目根为 `/path/to/CyberStrikeAI-main`，则脚本路径为：  
   `/path/to/CyberStrikeAI-main/mcp-servers/pent_claude_agent/mcp_pent_claude_agent.py`

2. **Web 界面** → **设置** → **外部 MCP** → **添加外部 MCP**，填入以下 JSON（将路径替换为你的实际路径）：

```json
{
  "pent-claude-agent": {
    "command": "/path/to/CyberStrikeAI-main/venv/bin/python3",
    "args": [
      "/path/to/CyberStrikeAI-main/mcp-servers/pent_claude_agent/mcp_pent_claude_agent.py",
      "--config",
      "/path/to/CyberStrikeAI-main/mcp-servers/pent_claude_agent/pent_claude_agent_config.yaml"
    ],
    "description": "渗透测试工程师：下发任务后独立执行并返回结果",
    "timeout": 300,
    "external_mcp_enable": true
  }
}
```

   - `command`：建议使用项目 **venv** 中的 Python，或系统 `python3`。
   - `args`：**必须使用绝对路径** 指向 `mcp_pent_claude_agent.py`。如需指定配置可追加 `--config` 及配置路径。
   - `timeout`：建议 300（渗透测试任务可能较长）。
   - 保存后点击该 MCP 的 **启动**，即可在对话中通过 AI 调用上述工具。

3. **使用流程示例**
   - CyberStrikeAI 调用 `pent_claude_run_pentest_task("扫描目标 192.168.1.1 的开放端口")`。
   - pent_claude_agent 内部启动 Claude Agent，可能使用 Bash、nmap 等工具执行。
   - 结果返回给 CyberStrikeAI。

## 本地单独运行（可选）

```bash
# 在项目根目录，使用 venv
./venv/bin/python mcp-servers/pent_claude_agent/mcp_pent_claude_agent.py
```

进程通过 stdio 与 MCP 客户端通信；CyberStrikeAI 以 stdio 方式启动该脚本时行为相同。

## 安全提示

- 仅在有授权、隔离的测试环境中使用。
- 配置中的 API Key 需妥善保管；生产环境建议使用环境变量。
