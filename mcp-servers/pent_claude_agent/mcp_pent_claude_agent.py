#!/usr/bin/env python3
"""
Pent Claude Agent MCP Server - 渗透测试工程师 MCP 服务

通过 MCP 协议暴露 AI 渗透测试能力：CyberStrikeAI 可指挥 pent_claude_agent 执行渗透测试任务。
pent_claude_agent 内部使用 Claude Agent SDK，可独立配置 MCP、工具等，作为独立的渗透测试工程师运行。

依赖：pip install mcp claude-agent-sdk（或使用项目 venv）
运行：python mcp_pent_claude_agent.py [--config /path/to/config.yaml]
"""

from __future__ import annotations

import argparse
import asyncio
import os
from typing import Any

import yaml
from mcp.server.fastmcp import FastMCP

# 延迟导入，避免未安装时影响 MCP 启动
_claude_sdk_available = False
try:
    from claude_agent_sdk import ClaudeAgentOptions, query

    _claude_sdk_available = True
except ImportError:
    pass

# ---------------------------------------------------------------------------
# 路径与配置
# ---------------------------------------------------------------------------

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
PROJECT_ROOT = os.path.dirname(os.path.dirname(SCRIPT_DIR))
_DEFAULT_CONFIG_PATH = os.path.join(SCRIPT_DIR, "pent_claude_agent_config.yaml")

# Agent 运行状态（简单内存状态，用于 status）
_last_task: str | None = None
_last_result: str | None = None
_task_count: int = 0


def _load_config(config_path: str | None) -> dict[str, Any]:
    """加载 YAML 配置，合并默认值与用户配置。"""
    defaults: dict[str, Any] = {
        "cwd": PROJECT_ROOT,
        "allowed_tools": ["Read", "Write", "Bash", "Grep", "Glob"],
        "env": {
            "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
            "DISABLE_TELEMETRY": "1",
            "DISABLE_ERROR_REPORTING": "1",
            "DISABLE_BUG_COMMAND": "1",
        },
        "mcp_servers": {},
        "system_prompt": (
            "你是一名专业的渗透测试工程师。根据用户给出的任务，进行安全测试、漏洞分析、信息收集等。"
            "请按步骤执行，输出清晰、可复现的结果。仅在授权范围内进行测试。"
        ),
    }
    path = config_path or os.environ.get("PENT_CLAUDE_AGENT_CONFIG", _DEFAULT_CONFIG_PATH)
    if not os.path.isfile(path):
        return defaults
    try:
        with open(path, "r", encoding="utf-8") as f:
            user = yaml.safe_load(f) or {}
        # 深度合并
        def merge(base: dict, override: dict) -> dict:
            out = dict(base)
            for k, v in override.items():
                if k in out and isinstance(out[k], dict) and isinstance(v, dict):
                    out[k] = merge(out[k], v)
                else:
                    out[k] = v
            return out

        return merge(defaults, user)
    except Exception:
        return defaults


def _resolve_path(s: str) -> str:
    """解析路径占位符。"""
    return s.replace("${PROJECT_ROOT}", PROJECT_ROOT).replace("${SCRIPT_DIR}", SCRIPT_DIR)


def _build_agent_options(config: dict[str, Any], cwd_override: str | None = None) -> ClaudeAgentOptions:
    """从配置构建 ClaudeAgentOptions。"""
    raw_cwd = cwd_override or config.get("cwd", PROJECT_ROOT)
    cwd = _resolve_path(str(raw_cwd)) if isinstance(raw_cwd, str) else str(raw_cwd)
    env = dict(os.environ)
    env.update(config.get("env", {}))
    mcp_servers = config.get("mcp_servers") or {}
    # 解析路径占位符
    for name, cfg in list(mcp_servers.items()):
        if isinstance(cfg, dict):
            args = cfg.get("args") or []
            cfg = dict(cfg)
            cfg["args"] = [_resolve_path(str(a)) for a in args]
            mcp_servers[name] = cfg

    return ClaudeAgentOptions(
        cwd=cwd,
        allowed_tools=config.get("allowed_tools", ["Read", "Write", "Bash", "Grep", "Glob"]),
        disallowed_tools=config.get("disallowed_tools", []),
        mcp_servers=mcp_servers,
        env=env,
        system_prompt=config.get("system_prompt"),
        setting_sources=config.get("setting_sources", ["user", "project"]),
    )


async def _run_claude_agent(prompt: str, config_path: str | None = None, cwd: str | None = None) -> str:
    """内部执行 Claude Agent，返回最后一轮文本结果。"""
    global _last_task, _last_result, _task_count
    _last_task = prompt
    _task_count += 1

    if not _claude_sdk_available:
        _last_result = "错误：未安装 claude-agent-sdk，请执行 pip install claude-agent-sdk"
        return _last_result

    config = _load_config(config_path)
    options = _build_agent_options(config, cwd_override=cwd)

    messages: list[Any] = []
    try:
        async for message in query(prompt=prompt, options=options):
            messages.append(message)
    except Exception as e:
        _last_result = f"Agent 执行异常: {e}"
        return _last_result

    if not messages:
        _last_result = "(无输出)"
        return _last_result

    # 多轮迭代时，取最后一个 ResultMessage（最后一波结果）
    result_msgs = [m for m in messages if hasattr(m, "result") and getattr(m, "result", None) is not None]
    last = result_msgs[-1] if result_msgs else messages[-1]
    # 提取文本内容，优先 ResultMessage.result，避免输出 metadata
    if hasattr(last, "result") and last.result is not None:
        text = last.result
    elif hasattr(last, "content") and last.content:
        parts = []
        for block in last.content:
            if hasattr(block, "text") and block.text:
                parts.append(block.text)
        text = "\n".join(parts) if parts else "(无输出)"
    else:
        text = "(无输出)"
    _last_result = text
    return _last_result


# ---------------------------------------------------------------------------
# MCP 服务与工具
# ---------------------------------------------------------------------------

app = FastMCP(
    name="pent-claude-agent",
    instructions="渗透测试工程师 MCP：接收任务后，内部启动 Claude Agent 独立执行渗透测试、漏洞分析等，并返回结果。",
)


@app.tool(
    description="执行渗透测试任务。下发任务描述后，pent_claude_agent 会作为独立的渗透测试工程师，使用 Claude Agent 执行任务并返回结果。支持：端口扫描、漏洞探测、Web 安全测试、信息收集等。",
)
async def pent_claude_run_pentest_task(task: str) -> str:
    """Run a penetration testing task. The agent executes independently and returns results."""
    return await _run_claude_agent(task)


@app.tool(
    description="分析漏洞信息。传入漏洞描述、PoC、影响范围等，由 Agent 进行专业分析并给出修复建议。",
)
async def pent_claude_analyze_vulnerability(vuln_info: str) -> str:
    """Analyze vulnerability information and provide remediation suggestions."""
    prompt = f"请对以下漏洞信息进行专业分析，包括：风险等级、影响范围、利用方式、修复建议。\n\n{vuln_info}"
    return await _run_claude_agent(prompt)


@app.tool(
    description="执行指定任务。通用任务执行入口，Agent 会根据任务内容自动选择合适的工具和方法。",
)
async def pent_agent_execute(task: str) -> str:
    """Execute a task. The agent chooses appropriate tools and methods."""
    return await _run_claude_agent(task)


@app.tool(
    description="对目标进行安全诊断。可传入 URL、IP、域名等，Agent 会进行初步的安全评估和诊断。",
)
async def pent_agent_diagnose(target: str) -> str:
    """Diagnose a target (URL, IP, domain) for security assessment."""
    prompt = f"请对以下目标进行安全诊断和初步评估：{target}\n\n包括：可达性、开放服务、常见漏洞面等。"
    return await _run_claude_agent(prompt)


@app.tool(
    description="获取 pent_claude_agent 的当前状态：最近任务、结果摘要、执行次数等。",
)
def pent_claude_status() -> str:
    """Get the current status of pent_claude_agent."""
    global _last_task, _last_result, _task_count
    lines = [
        f"任务执行次数: {_task_count}",
        f"最近任务: {_last_task or '-'}",
        f"最近结果摘要: {(str(_last_result or '-')[:200] + '...') if _last_result and len(str(_last_result)) > 200 else (_last_result or '-')}",
        f"Claude SDK 可用: {_claude_sdk_available}",
    ]
    return "\n".join(lines)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Pent Claude Agent MCP Server")
    parser.add_argument(
        "--config",
        default=None,
        help="Path to pent_claude_agent config YAML (env: PENT_CLAUDE_AGENT_CONFIG)",
    )
    args, _ = parser.parse_known_args()
    # 将 config 路径存入环境，供工具调用时使用
    if args.config:
        os.environ["PENT_CLAUDE_AGENT_CONFIG"] = args.config
    app.run(transport="stdio")
