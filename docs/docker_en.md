# Docker Deployment Guide

CyberStrikeAI ships with a full Docker lifecycle management layer. You can build, deploy, update, and monitor the Docker stack via the `run_docker.sh` CLI script or from the **System Settings → Docker** panel in the web UI.

---

## Quick Start

```bash
git clone https://github.com/cybersecua/CyberStrikeAI.git
cd CyberStrikeAI
chmod +x run_docker.sh && ./run_docker.sh deploy
```

This will:
1. Build the multi-stage Docker image from the project `Dockerfile`
2. Start the container via Docker Compose
3. Expose the web UI on port **18080** (mapped to internal 8080)

Open `http://localhost:18080` and log in with the auto-generated password printed in the logs.

---

## `run_docker.sh` Reference

```
Usage: ./run_docker.sh <action> [options]

Actions:
  install   Install/validate Docker engine + compose plugin
  deploy    Build and run suite container with current code
  update    Git pull (selected ref) + deploy
  start     Start existing container
  stop      Stop container
  restart   Restart container
  status    Show Docker/suite status
  logs      Show container logs (use -f for follow)
  test      Run container runtime test suite
  remove    Remove container, network, and volumes for this stack

Options:
  --proxy-mode <direct|socks|http|tor|vpn>
  --proxy-url <url>             For socks/http modes (e.g. socks5h://host:1080)
  --vpn-container <name>        For vpn mode (network_mode=container:<name>)
  --git-ref <branch-or-tag>     Used by update (default: main)
  -f, --follow                  Follow logs (for logs action)
```

### Common Commands

| Goal | Command |
|------|---------|
| First-time deploy | `./run_docker.sh deploy` |
| Pull latest code and redeploy | `./run_docker.sh update` |
| Pull a specific tag | `./run_docker.sh update --git-ref v1.5.0` |
| Start a stopped container | `./run_docker.sh start` |
| Stop the container | `./run_docker.sh stop` |
| Restart | `./run_docker.sh restart` |
| View live logs | `./run_docker.sh logs -f` |
| Check status | `./run_docker.sh status` |
| Run runtime tests | `./run_docker.sh test` |
| Tear everything down | `./run_docker.sh remove` |

---

## Proxy & VPN Modes

CyberStrikeAI supports routing container traffic through a proxy or VPN for testing engagements that require a specific egress IP.

### Direct (default)

```bash
./run_docker.sh deploy --proxy-mode direct
```

No proxy; container uses the host network stack directly.

### SOCKS5 Proxy

```bash
./run_docker.sh deploy --proxy-mode socks --proxy-url socks5h://127.0.0.1:1080
```

### HTTP Proxy

```bash
./run_docker.sh deploy --proxy-mode http --proxy-url http://proxy.example.com:8080
```

### Tor

```bash
./run_docker.sh deploy --proxy-mode tor
```

Routes traffic through a local Tor daemon (must be running on the host at the default SOCKS port).

### VPN Container

```bash
./run_docker.sh deploy --proxy-mode vpn --vpn-container my-vpn
```

Sets `network_mode: container:<name>` in the Compose override so the CyberStrikeAI container shares the network namespace of a running VPN container.

---

## System Settings → Docker Panel

Once CyberStrikeAI is running, navigate to **Settings → Docker** in the web UI for a graphical management interface.

### Status Grid

The status grid shows at a glance:

| Field | Description |
|-------|-------------|
| Running in Docker | Whether the app itself is running inside a container |
| Docker Installed | Whether the Docker CLI is available on the host |
| Compose Installed | Whether `docker compose` (v2) or `docker-compose` (v1) is present |
| Container Name | Name of the `cyberstrikeai` container |
| Container Status | Output of `docker ps` status column |
| Container Image | Image name and tag in use |
| Compose Version | `docker compose version` output |
| App :18080 / :8080 | HTTP health probe results for both mapped ports |
| run_docker.sh | Path to the management script (or "Missing" if not found) |
| Checked At | Timestamp of the last status poll |

### Live Log Streaming

1. Set the **Lines** input to the number of log lines to retrieve (default: 300).
2. Click **Refresh Logs** to fetch once.
3. Click **Start Stream** to poll every 2.5 seconds - the button changes to **Stop Stream** to pause.

Logs are sourced from `docker logs --tail N cyberstrikeai` when the Docker CLI is available, falling back to `logs/suite.log` otherwise.

### Lifecycle Actions

The **Actions** panel lets you run any `run_docker.sh` action with optional parameters:

| Action | Description |
|--------|-------------|
| `deploy` | Build and start the Docker stack |
| `update` | Pull latest code (`--git-ref`) and redeploy |
| `start` | Start a stopped container |
| `stop` | Stop the running container |
| `restart` | Restart the container |
| `status` | Refresh status information |
| `logs` | Fetch recent log lines |
| `test` | Run the container runtime test suite |
| `remove` | **Destructive** - remove containers, network, and volumes (prompts for confirmation) |

Configure optional parameters before running an action:

- **Proxy Mode** - `direct`, `socks`, `http`, `tor`, or `vpn`
- **Proxy URL** - URL for `socks` or `http` modes
- **VPN Container** - container name for `vpn` mode
- **Git Ref** - branch or tag for `update` (default: `main`)

Action output is streamed into the output panel and the status grid refreshes automatically on completion.

---

## REST API

All Docker management operations are also available via authenticated REST API.

### Get Status

```http
GET /api/docker/status
Authorization: Bearer <token>
```

Response:
```json
{
  "in_docker": false,
  "docker_installed": true,
  "compose_installed": true,
  "compose_version": "Docker Compose version v2.24.0",
  "container_name": "cyberstrikeai",
  "container_status": "Up 2 hours",
  "container_image": "cyberstrikeai:latest",
  "script_exists": true,
  "script_path": "/opt/CyberStrikeAI/run_docker.sh",
  "http": {
    "app_18080": { "ok": true, "status_code": 200 },
    "app_8080":  { "ok": false, "error": "connection refused" }
  },
  "checked_at": "2026-03-06T17:00:00Z"
}
```

### Get Logs

```http
GET /api/docker/logs?lines=200
Authorization: Bearer <token>
```

Response:
```json
{
  "source": "docker",
  "lines": 200,
  "log": "..."
}
```

The `source` field is `"docker"` when container logs are available, `"file"` when falling back to `logs/suite.log`, or `"none"` on error.

### Run Action

```http
POST /api/docker/action
Authorization: Bearer <token>
Content-Type: application/json

{
  "action": "restart",
  "proxy_mode": "direct",
  "proxy_url": "",
  "vpn_container": "",
  "git_ref": "main"
}
```

Response:
```json
{
  "action": "restart",
  "success": true,
  "exitCode": 0,
  "output": "...",
  "error": ""
}
```

Allowed `action` values: `install`, `deploy`, `update`, `start`, `stop`, `restart`, `status`, `logs`, `test`, `remove`.

---

## Docker Compose Configuration

The default `docker-compose.yml` maps:
- Port **18080** (host) → **8080** (container) for the web UI
- Port **18081** (host) → **8081** (container) for the MCP server

Data is persisted via Docker volumes:
- `./data` → `/app/data` (databases)
- `./tmp` → `/app/tmp` (large result storage)
- `./logs` → `/app/logs` (log files)
- `./config.docker.yaml` → `/app/config.yaml` (runtime config)

### Custom Overrides

Place custom Compose configuration in `.docker/docker-compose.override.yml`. This file is automatically merged if it exists. Use it for port remapping, resource limits, or additional volume mounts without modifying the tracked `docker-compose.yml`.

---

## Updating

```bash
# Pull latest main branch and redeploy
./run_docker.sh update

# Deploy a specific release tag
./run_docker.sh update --git-ref v1.5.0
```

The `update` action runs `git pull`, rebuilds the image, and restarts the container in one step.

---

## Troubleshooting

### Container exits immediately

Check the logs:
```bash
./run_docker.sh logs
```

The most common cause is a missing or invalid `config.yaml` (or `config.docker.yaml`). Ensure `openai.api_key` and `openai.base_url` are set.

### Web UI unreachable on port 18080

1. Verify the container is running: `./run_docker.sh status`
2. Check that port 18080 is not occupied by another process: `ss -tlnp | grep 18080`
3. If running behind a firewall, ensure the port is open.

### Permission denied running `run_docker.sh`

```bash
chmod +x run_docker.sh && ./run_docker.sh deploy
```

### Docker not found inside container

The `docker` CLI is not installed inside the container by default. The Docker management API (`/api/docker/*`) detects this and falls back to reading `logs/suite.log` for the logs endpoint. Lifecycle actions that require `docker` CLI (start, stop, etc.) must be run from the host.
