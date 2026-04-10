# MCP 服务

[English](README.md)

本目录存放 **独立 MCP（Model Context Protocol）服务**，采用标准 MCP 协议（stdio 或部分服务支持 HTTP/SSE），因此 **任意支持 MCP 的客户端** 均可使用——不限于 CyberStrikeAI，**Cursor**、**VS Code**（配合 MCP 扩展）、**Claude Code** 等均可接入。

**我们会持续在此新增好用的 MCP 服务**，覆盖安全测试、自动化与集成等场景，敬请关注。

## 已提供服务

| 服务 | 说明 |
|------|------|
| [reverse_shell](reverse_shell/) | 反向 Shell：开启/停止监听、与已连接目标交互执行命令，完整交互流程。 |

## 使用方式

各 MCP 需在对应客户端里配置后使用。stdio 模式下 `command` 与 `args` 请使用**绝对路径**。

### CyberStrikeAI

1. 打开 Web 界面 → **设置** → **外部 MCP**。
2. 添加新的外部 MCP，按各服务目录下 README 的说明填写 JSON 配置。
3. 保存后点击 **启动**，对话中即可使用对应工具。

### Cursor

在 Cursor 的 MCP 配置中添加（如 **Settings → Tools & MCP → Add Custom MCP**，或编辑 `~/.cursor/mcp.json` / 项目下的 `.cursor/mcp.json`）。stdio 示例：

```json
{
  "mcpServers": {
    "reverse-shell": {
      "command": "/你的绝对路径/venv/bin/python3",
      "args": ["/你的绝对路径/CyberStrikeAI-main/mcp-servers/reverse_shell/mcp_reverse_shell.py"]
    }
  }
}
```

将路径替换为实际路径后，Cursor 会启动该进程并通过 stdio 与 MCP 通信。

### VS Code（MCP 扩展）/ Claude Code / 其他客户端

在对应客户端中配置为通过 **stdio** 启动：**command** 填 Python 可执行文件路径，**args** 填脚本路径（详见各服务 README）。配置位置依客户端而定（如 `.mcp.json`、`~/.claude.json` 或扩展设置），请查阅该客户端的 MCP 说明。

## 依赖说明

- 基于 Python 的服务需 Python 3.10+。
- 建议使用项目自带的 `venv`，例如 `venv/bin/python3` 配合 `mcp-servers/` 下脚本路径。
