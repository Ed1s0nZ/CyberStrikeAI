# 本地图编排策略包 MVP：API 与数据模型契约 v1

> 本文是 [本地图编排策略包 MVP 设计](2026-07-13-local-workflow-package-mvp-design.md) 的实现前契约。前端与后端以本文的路径、字段、枚举、状态码和错误码为准；未经版本升级不得改变既有字段语义。

## 1. 范围与不变式

- 仅支持一个工作流的本地 `.csapkg.zip` 包。
- 仅处理 `workflow_definitions`；不导入 Role、Skill、MCP 配置、运行记录或任何可执行文件。
- 导入固定为“上传预检”和“确认应用”两步。预检不修改 `workflow_definitions`。
- 现有工作流 CRUD、`/validate`、`/dry-run`、运行 API 和 `workflow_definitions` 表结构保持兼容。
- 已有 `workflow_definitions.version` 始终是目标实例本地修订号：新建导入从 `1` 开始；覆盖导入由现有本地版本递增；包内 `source_revision` 只用于展示和审计。

## 2. 统一约定

### 2.1 认证与权限

所有 API 均位于现有受保护的 `/api` 路由组。

| 接口 | 所需权限 |
|---|---|
| 导出包 | `workflow:read` |
| 创建或读取预检 | `workflow:write` |
| 应用或读取导入结果 | `workflow:write` |

预检会保存短期、已验证的工作流载荷，故不把它降级为只读权限。`created_by` / `actor_user_id` 取当前已认证会话的 `UserID`。

RBAC 路由映射必须显式新增：`GET /workflows/:id/package` 映射 `workflow:read`；`/workflow-package-inspections` 与 `/workflow-package-imports` 的所有 MVP 路由映射 `workflow:write`。它们与既有工作流定义同属全局资产，写操作仅允许现有 RBAC 的 `all` 资源范围。

### 2.2 错误响应

新接口统一使用如下错误响应；不改变旧工作流 API 的 `{"error":"..."}` 兼容格式。

```json
{
  "error": {
    "code": "WFPKG_ID_CONFLICT",
    "message": "目标实例已存在同 ID 工作流，请选择处理策略",
    "details": {
      "workflow_id": "web-src-hunting"
    }
  }
}
```

`details` 仅包含可安全展示的结构化信息，不返回 Zip 路径、服务端文件路径、Token 或内部堆栈。

### 2.3 时间与哈希

- 所有时间字段使用 RFC 3339 UTC 字符串。
- 哈希固定为小写十六进制 `sha256:<64-hex>`。
- `graph_hash` 是对规范化 `graph_json` 的 SHA-256；`content_hash` 是工作流包项的 SHA-256。

## 3. REST API

### 3.1 导出单工作流包

```http
GET /api/workflows/{id}/package
Accept: application/zip
```

语义：从当前 `workflow_definitions` 行生成 `.csapkg.zip`。只读、幂等、无确认、无数据库写入。

成功响应：

```http
200 OK
Content-Type: application/zip
Content-Disposition: attachment; filename="web-src-hunting.csapkg.zip"
ETag: "sha256:3f..."
X-Workflow-Package-SHA256: sha256:3f...
```

失败：`404 WFPKG_WORKFLOW_NOT_FOUND`、`403`、`500 WFPKG_EXPORT_FAILED`。

### 3.2 创建预检

```http
POST /api/workflow-package-inspections
Content-Type: multipart/form-data

file=@web-src-hunting.csapkg.zip;type=application/zip
```

限制：请求体最大 10 MiB；Zip 解压总量最大 20 MiB；只允许 `manifest.json`、`checksums.sha256` 和一个 `workflows/*.json`。同一包不得有重复条目、软链接、路径穿越或未声明文件。

成功响应：`201 Created`。

```json
{
  "inspection": {
    "id": "wpi_01JQ2K6G7K8W2C1E3R4T5Y6U7I",
    "status": "ready",
    "expires_at": "2026-07-13T09:30:00Z",
    "package": {
      "package_format": "cyberstrikeai.workflow-package",
      "format_version": "1.0",
      "package_id": "pkg_01JWEBHUNT",
      "package_hash": "sha256:af..."
    },
    "workflow": {
      "source_id": "web-src-hunting",
      "name": "Web SRC 猎洞",
      "description": "面向 SRC Web 资产的侦察与漏洞候选流程",
      "source_revision": 18,
      "enabled": true,
      "content_hash": "sha256:51...",
      "graph_hash": "sha256:a9...",
      "node_count": 10,
      "edge_count": 11
    },
    "conflict": {
      "state": "id_conflict",
      "local_workflow": {
        "id": "web-src-hunting",
        "version": 12,
        "content_hash": "sha256:42...",
        "graph_hash": "sha256:17..."
      }
    },
    "warnings": []
  }
}
```

`conflict.state` 固定枚举：

| 值 | 含义 |
|---|---|
| `none` | 目标不存在，可 `create`。 |
| `identical` | 目标同 ID 且 `content_hash` 相同。 |
| `id_conflict` | 目标同 ID，但内容不同。 |

无效包不创建 inspection，直接返回 `422`。常用错误码：`WFPKG_FILE_REQUIRED`、`WFPKG_FILE_TOO_LARGE`、`WFPKG_INVALID_ARCHIVE`、`WFPKG_UNSUPPORTED_FORMAT`、`WFPKG_INVALID_MANIFEST`、`WFPKG_CHECKSUM_MISMATCH`、`WFPKG_MULTIPLE_WORKFLOWS`、`WFPKG_WORKFLOW_INVALID`。

### 3.3 读取预检

```http
GET /api/workflow-package-inspections/{inspectionId}
```

用于前端刷新页面后恢复预检状态。仅 inspection 创建者可读取；不存在返回 `404 WFPKG_INSPECTION_NOT_FOUND`，已过期返回 `409 WFPKG_INSPECTION_EXPIRED`。

### 3.4 应用导入

```http
POST /api/workflow-package-imports
Content-Type: application/json
Idempotency-Key: 4b75a1eb-7ed1-4eb1-a074-389dba3d4d7b
```

```json
{
  "inspection_id": "wpi_01JQ2K6G7K8W2C1E3R4T5Y6U7I",
  "resolution": {
    "action": "overwrite",
    "new_workflow_id": ""
  },
  "confirm_overwrite": true
}
```

字段规则：

| 字段 | 规则 |
|---|---|
| `inspection_id` | 必填；必须是当前用户创建、状态为 `ready` 且未过期的 inspection。 |
| `resolution.action` | `create`、`keep_existing`、`overwrite`、`rename` 之一。 |
| `resolution.new_workflow_id` | 仅 `rename` 必填；去除首尾空格后 1–128 字符，不得包含控制字符。其他 action 必须传空字符串。 |
| `confirm_overwrite` | 仅 `overwrite` 时必须为 `true`。 |
| `Idempotency-Key` | 必填 UUID；同一用户、同一 key、同一请求返回原结果；同 key 不同请求返回冲突。 |

`request_hash` 固定为下列字段按键名升序、无空白序列化后的 SHA-256：`inspection_id`、`resolution.action`、`resolution.new_workflow_id`、`confirm_overwrite`。后端不得将 `Idempotency-Key` 自身计入该 hash。

动作与 inspection 状态的合法组合：

| `conflict.state` | 合法 action | 结果 |
|---|---|---|
| `none` | `create` | 新建目标工作流。 |
| `identical` | `keep_existing` | 返回 `skipped_identical`，不修改工作流。 |
| `id_conflict` | `keep_existing` | 返回 `kept_existing`，不修改工作流。 |
| `id_conflict` | `overwrite` | 完整替换 name、description、graph_json、enabled；本地 version 递增。 |
| `id_conflict` | `rename` | 以 `new_workflow_id` 新建副本，version 为 1。 |

后端在应用事务开始前必须重新读取目标工作流并比较 inspection 中记录的冲突快照；若本地内容在预检后变化，返回 `409 WFPKG_CONFLICT_CHANGED`，前端必须重新预检。

首次应用成功返回 `201 Created`：

```json
{
  "import": {
    "id": "wpii_01JQ2M93S2PH0WY8X7B4F8R9QG",
    "inspection_id": "wpi_01JQ2K6G7K8W2C1E3R4T5Y6U7I",
    "status": "succeeded",
    "result": "overwritten",
    "action": "overwrite",
    "source_workflow_id": "web-src-hunting",
    "target_workflow_id": "web-src-hunting",
    "workflow": {
      "id": "web-src-hunting",
      "version": 13,
      "content_hash": "sha256:51...",
      "graph_hash": "sha256:a9..."
    },
    "applied_at": "2026-07-13T09:05:00Z"
  }
}
```

同一幂等键重试返回 `200 OK` 和完全相同的 `import` 对象。成功写入后必须调用 `InvalidateCompiledCache(workflowID)`。

错误码：

| HTTP | code | 触发条件 |
|---:|---|---|
| 400 | `WFPKG_IDEMPOTENCY_KEY_REQUIRED` | 缺少或非 UUID 幂等键。 |
| 404 | `WFPKG_INSPECTION_NOT_FOUND` | inspection 不存在或不属于当前用户。 |
| 409 | `WFPKG_INSPECTION_EXPIRED` | inspection 已过期。 |
| 409 | `WFPKG_INSPECTION_CONSUMED` | inspection 已被其他幂等键成功应用。 |
| 409 | `WFPKG_IDEMPOTENCY_KEY_REUSED` | 同 key 的请求体不同。 |
| 409 | `WFPKG_ID_CONFLICT` | action 与预检冲突状态不匹配。 |
| 409 | `WFPKG_OVERWRITE_CONFIRMATION_REQUIRED` | overwrite 未确认。 |
| 409 | `WFPKG_CONFLICT_CHANGED` | 预检后本地工作流已改变。 |
| 422 | `WFPKG_INVALID_ACTION` | action 不在枚举中，或 action 与字段组合不合法。 |
| 422 | `WFPKG_INVALID_RENAME_ID` | rename ID 为空、含控制字符或已存在。 |
| 500 | `WFPKG_IMPORT_FAILED` | 事务失败；不修改目标工作流。 |

### 3.5 查询导入结果

```http
GET /api/workflow-package-imports/{importId}
```

仅导入创建者可读取。响应为 3.4 中的 `import` 对象。该接口不提供列表；MVP 的历史审计通过现有审计日志页面查看。

## 4. SQLite 数据模型

`workflow_definitions` 不增列、不改语义。新增两张表；DDL 即为迁移目标。

```sql
CREATE TABLE IF NOT EXISTS workflow_package_inspections (
  id TEXT PRIMARY KEY,
  package_hash TEXT NOT NULL,
  manifest_json TEXT NOT NULL,
  workflow_payload_json TEXT NOT NULL,
  inspection_json TEXT NOT NULL,
  source_workflow_id TEXT NOT NULL,
  source_revision INTEGER NOT NULL,
  source_content_hash TEXT NOT NULL,
  source_graph_hash TEXT NOT NULL,
  local_conflict_state TEXT NOT NULL
    CHECK (local_conflict_state IN ('none', 'identical', 'id_conflict')),
  local_workflow_id TEXT,
  local_content_hash TEXT,
  local_graph_hash TEXT,
  created_by TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'ready'
    CHECK (status IN ('ready', 'consumed', 'expired')),
  created_at DATETIME NOT NULL,
  expires_at DATETIME NOT NULL,
  consumed_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_workflow_package_inspections_creator_expiry
  ON workflow_package_inspections(created_by, expires_at);

CREATE TABLE IF NOT EXISTS workflow_package_imports (
  id TEXT PRIMARY KEY,
  inspection_id TEXT NOT NULL,
  request_hash TEXT NOT NULL,
  idempotency_key TEXT NOT NULL,
  actor_user_id TEXT NOT NULL,
  action TEXT NOT NULL
    CHECK (action IN ('create', 'keep_existing', 'overwrite', 'rename')),
  source_workflow_id TEXT NOT NULL,
  target_workflow_id TEXT NOT NULL,
  resulting_workflow_id TEXT,
  result TEXT NOT NULL
    CHECK (result IN ('created', 'overwritten', 'renamed', 'kept_existing', 'skipped_identical', 'failed')),
  error_code TEXT,
  error_message TEXT,
  created_at DATETIME NOT NULL,
  applied_at DATETIME,
  FOREIGN KEY (inspection_id) REFERENCES workflow_package_inspections(id)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_workflow_package_imports_actor_key
  ON workflow_package_imports(actor_user_id, idempotency_key);

CREATE UNIQUE INDEX IF NOT EXISTS uq_workflow_package_imports_inspection_success
  ON workflow_package_imports(inspection_id)
  WHERE result IN ('created', 'overwritten', 'renamed', 'kept_existing', 'skipped_identical');
```

### 4.1 表职责与生命周期

| 表 | 职责 | 保留规则 |
|---|---|---|
| `workflow_package_inspections` | 保存已验证的 Manifest、单工作流载荷、冲突快照和前端恢复所需摘要；不保存原始 Zip。 | `expires_at` 为创建后 30 分钟；到期改为 `expired`；清理任务可在 24 小时后删除。 |
| `workflow_package_imports` | 导入结果、幂等键和应用记录。 | 保留 90 天；删除不影响既有审计日志。 |

inspection 创建时写入 `workflow_payload_json`，应用时只读取该已验证载荷，不信任浏览器重新提交的工作流内容。`inspection_json` 是 3.2 成功响应中的安全摘要快照。

### 4.2 导入事务

导入应用必须在一个 SQLite 事务中完成以下操作：

1. 校验 inspection 所属用户、状态、有效期与幂等键。
2. 再次读取目标 `workflow_definitions`，验证冲突快照未变化。
3. 按 action 新建、覆盖、重命名或保持现有工作流。
4. 新增 `workflow_package_imports` 成功行，并把 inspection 改为 `consumed`。
5. 提交事务；提交后失效工作流编译缓存并写审计日志。

任一步失败必须回滚工作流、导入行和 inspection 状态。`workflow_package_imports.result='failed'` 仅在能安全独立记录失败时写入，绝不替代事务回滚。

## 5. 审计契约

复用现有 `audit_logs`，不在包表中复制审计全文：

| 事件 | category | action | resource |
|---|---|---|---|
| 导出成功 | `workflow_package` | `export` | `workflow/{id}` |
| 预检成功 | `workflow_package` | `inspect` | `inspection/{id}` |
| 预检失败 | `workflow_package` | `inspect` | 无资源 ID；detail 仅含错误码与包 hash |
| 应用成功 | `workflow_package` | `import` | `workflow/{resulting_workflow_id}` |
| 应用失败 | `workflow_package` | `import` | `inspection/{id}` |

## 6. 前后端并行边界

前端可依据本文直接完成：下载按钮、文件上传、预检结果页、冲突动作选择、`confirm_overwrite` 二次确认、导入结果页和错误码国际化。

后端可依据本文直接完成：路由、Handler、包解析服务、SQLite 迁移、事务、RBAC 映射、审计和单元/集成测试。

前端不得自行解析 Zip、计算最终冲突结论或直接提交工作流 JSON；后端是 Manifest、哈希、图校验、冲突复查和导入结果的唯一权威。
