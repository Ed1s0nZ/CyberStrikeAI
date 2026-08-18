# Deployment Guide

[中文](../zh-CN/deployment.md)

CyberStrikeAI can run as a local testing tool, an internal team service, or a production red-team platform. Treat it as a high-privilege security system: it can execute commands, call MCP tools, manage WebShell connections, and optionally run C2 listeners.

## Prerequisites

- Go for source runs and binary builds.
- Python for some MCP servers and tool scripts.
- SQLite files under `data/`; no external DB is required by default.
- Actual security tools installed in PATH. YAML files under `tools/` only describe commands.
- At least one `ai.channels` entry. Use `provider: openai_compatible` for OpenAI-compatible endpoints, or `provider: claude` for Eino's native Claude component.

Important persistent paths:

```text
config.yaml
data/
tools/
roles/
skills/
agents/
knowledge_base/
chat_uploads/
```

Back these up before upgrades.

## Startup Modes

Local quick start:

```bash
chmod +x run.sh && ./run.sh
```

`run.sh` is the most common startup path for local use, development, small temporary internal deployments, and quick post-upgrade verification.

For long-running service, boot-time startup, managed logs, and crash recovery, prefer a binary managed by systemd.

Source run:

```bash
go run ./cmd/server --config config.yaml
```

Binary build:

```bash
go build -o cyberstrike-ai ./cmd/server
./cyberstrike-ai --config config.yaml
```

The binary still needs `web/templates`, `web/static`, and the runtime resource directories.

## HTTPS and Reverse Proxy

For local testing, self-signed HTTPS is acceptable:

```yaml
server:
  tls_enabled: true
  tls_auto_self_sign: true
```

For production, use real certificates or terminate TLS at a reverse proxy. If the proxy terminates TLS and forwards HTTP to the app, avoid enabling app-side TLS on the same upstream unless `proxy_pass` uses HTTPS.

Nginx must not buffer SSE:

```nginx
proxy_buffering off;
proxy_http_version 1.1;
proxy_set_header Upgrade $http_upgrade;
proxy_set_header Connection "upgrade";
```

## Deployment Decision Table

| Scenario | Recommended setup | Key settings | Avoid |
| --- | --- | --- | --- |
| Personal testing | `./run.sh` + self-signed HTTPS | `tls_auto_self_sign: true` | Public exposure |
| Internal team | Binary + systemd + internal HTTPS | strong password, audit, backup, IP restrictions | Shared weak password |
| Production red-team platform | Reverse proxy + dedicated OS user + log collection | real certs, proxy auth, C2 only when needed | Direct public admin UI |
| Chat/KB only | Disable C2 and unnecessary MCP | `c2.enabled: false` | All tools enabled by default |
| Tool automation | Isolated workspace + HITL | `workspace_root_dir`, `hitl`, `monitor` | Shell tools globally allowlisted |

## Acceptance Checklist

After startup:

1. Open `/` and verify no HTTP/HTTPS redirect loop.
2. Login and validate `/api/auth/validate`.
3. Run model test in settings.
4. Check tool list and schemas.
5. If KB is enabled, check index status.
6. If external MCP is enabled, verify connection and tool visibility.
7. If C2 is enabled, start and stop a test listener only in an authorized network.
8. Check audit logs for login and config activity.

## Runtime File Layers

- Replaceable: binary, `web/`, default docs/resources.
- Preserve: `config.yaml`, `data/`, custom tools/roles/skills/agents, `knowledge_base`, uploads.
- Cleanup candidates: checkpoints, temporary workspaces, stale payloads, old tool execution records.

## Source Anchors

- App wiring and routes: `internal/app/app.go`
- TLS bootstrap: `internal/app/main_server_tls.go`
- HTTP to HTTPS redirect: `internal/app/main_server_http_redirect.go`
- Config structs: `internal/config/config.go`
- Config apply: `internal/handler/config.go`
