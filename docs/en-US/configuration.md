# Configuration Reference

[中文](../zh-CN/configuration.md)

The main configuration file is `config.yaml`. Many fields are editable through the Web settings page, but not every field has the same hot-apply behavior.

## Core Sections

```yaml
server:
  host: 0.0.0.0
  port: 8080
  tls_enabled: true
  # Optional: other trusted Web integrations; Chromium extensions need no entry.
  # cors_allowed_origins:
  #   - https://trusted-integration.example
auth:
  session_duration_hours: 12
openai:
  provider: openai
  base_url: https://api.openai.com/v1
  api_key: sk-...
  model: gpt-4.1
agent:
  max_iterations: 12000
  tool_timeout_minutes: 60
```

Change the initial `admin` password from the Web UI after first login. Use HTTPS or a trusted reverse proxy in any shared environment.

Valid Chromium `chrome-extension://<32-character-extension-id>` origins are recognized automatically. The extension must still obtain host permission and authenticate with a password and Bearer token. `server.cors_allowed_origins` remains available as an exact allowlist for other trusted Web integrations; wildcards are not accepted, and changing it requires a restart.

## Hot-Apply Boundaries

`POST /api/config/apply` coordinates model config, tool description mode, MCP tool registration, knowledge components, robot restarts, and C2 runtime reconciliation. It does not make every field instantly effective.

| Section | Usually hot-applies | Extra action |
| --- | --- | --- |
| `openai` | new requests use new model settings | running streams keep their current state |
| `agent.max_iterations` | new tasks | existing tasks continue |
| `hitl.tool_whitelist` | new approval checks | pending approvals are not re-decided |
| `knowledge.enabled` | initializes/updates components | scan and index are still required |
| `knowledge.embedding` | updates retriever/indexer config | rebuild index for existing vectors |
| `robots` | restarts long-lived connections | platform callback settings must still match |
| `c2.enabled` | reconciles C2 runtime | verify existing listeners/sessions manually |
| `server.port/tls` | usually needs process restart | listener settings are not ordinary hot state |

## Fallback Relationships

- `vision.api_key/base_url/provider` can inherit from `openai`.
- `hitl.audit_model` can inherit from `openai`.
- `knowledge.embedding.base_url/api_key` can inherit from model settings.
- rerank config can inherit from embedding/openai.
- `database.knowledge_db_path` can be separate or reuse the main DB.

When debugging, inspect both the child config and the fallback parent.

## Recommended Values

| Field | Conservative | Aggressive | Decide by |
| --- | --- | --- | --- |
| `agent.tool_timeout_minutes` | 10-30 | 60+ | long scanners |
| `shell_no_output_timeout_seconds` | 300-600 | 1200+ | quiet tools |
| `knowledge.indexing.batch_size` | 5-10 | 20+ | embedding API limits |
| `knowledge.indexing.rate_limit_delay_ms` | 300-800 | 0-100 | 429 frequency |
| `retrieval.top_k` | 3-5 | 8-12 | context budget |
| `similarity_threshold` | 0.35-0.45 | 0.5+ | recall vs precision |
| `audit.retention_days` | 15-30 | 90+ | compliance and disk |

## Change Template

Before changing config, write down:

```text
Purpose:
Sections:
Expected impact:
Rollback:
Validation endpoints:
```

After changing, validate the specific subsystem rather than trusting the save message.

## Source Anchors

- Config structs: `internal/config/config.go`
- Env expansion: `internal/config/envexpand.go`
- Config API and apply: `internal/handler/config.go`
- Route registration: `internal/app/app.go`
- C2 reconciliation: `internal/app/c2_lifecycle.go`
