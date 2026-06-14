# System Terminal

The system terminal exposes three authenticated API paths:

- `POST /api/terminal/run`: runs a command and returns the completed output.
- `POST /api/terminal/run/stream`: runs a command and streams output.
- `GET /api/terminal/ws`: opens an interactive terminal WebSocket.

Interactive WebSocket terminal support depends on Unix PTY behavior. On Unix-like systems, `/api/terminal/ws` starts an interactive shell through a PTY and forwards resize messages from the browser.

On Windows, `/api/terminal/ws` returns HTTP 501 because the PTY-backed interactive shell is not available there. The non-interactive and streaming command endpoints remain available on Windows.
