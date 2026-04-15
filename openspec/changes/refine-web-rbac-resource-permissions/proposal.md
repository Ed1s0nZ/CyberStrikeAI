## Why

当前 Web RBAC 的权限目录仍是按功能分类和路由族粗粒度授权，例如 `system.config.write`、`security.users.manage`。这种模型无法围绕具体资源表达最小权限，也无法覆盖当前平台内已经受保护、但此前仅依赖登录态或局部约束的业务域。

已批准设计要求将 Web RBAC 扩展为平台级资源权限模型，并保持与 AI Agent roles 分离。OpenSpec 必须反映这个已批准范围，而不是继续停留在旧的窄范围控制面子集。

## What Changes

- 将 Web RBAC 权限模型从“按功能分类授权”调整为“按资源授权”，并覆盖信息收集、任务管理、漏洞管理、WebShell 管理、文件管理、MCP、知识、Skills、Agents、角色、系统设置等受保护业务域。
- 权限标识统一采用 `domain.resource.action` 命名，并使用固定业务域 `intel`、`task`、`vulnerability`、`webshell`、`file`、`mcp`、`knowledge`、`skill`、`agent`、`role`、`system`。
- canonical permission catalog 覆盖平台级权限全集，包括 `intel.fofa_query.execute`、`task.batch_queue.*`、`task.batch_task.*`、`task.conversation.*`、`task.group.*`、`task.execution.*`、`task.attack_chain.*`、`task.conversation_result.read`、`vulnerability.record.*`、`vulnerability.stats.read`、`webshell.connection.*`、`webshell.session.*`、`webshell.command.execute`、`webshell.file.execute`、`file.workspace_entry.*`、`file.workspace_content.*`、`mcp.gateway.execute`、`mcp.external_server.*`、`knowledge.category.read`、`knowledge.item.*`、`knowledge.index.*`、`knowledge.retrieval_log.*`、`knowledge.search.execute`、`knowledge.stats.read`、`skill.definition.*`、`skill.binding.read`、`skill.stats.*`、`agent.run.*`、`agent.multi_run.*`、`agent.markdown_agent.*`、`agent.robot_test.execute`、`role.agent_role.*`、`system.config_settings.*`、`system.runtime_config.apply`、`system.model_connectivity.test`、`system.web_user.*`、`system.web_user_credential.reset`、`system.web_access_role.*`、`system.terminal.execute`、`system.api_spec.read`、`system.super_admin.grant`。
- 将受保护接口的鉴权映射改为每条路由显式绑定一个 canonical `domain.resource.action` 权限，并保持 `401`/`403` 语义不变。
- 更新 Web access role 的创建、编辑、展示和文档，使角色授权按业务域与资源分组展示，只接受 canonical permission identifiers。
- 为旧权限标识定义确定性规范化迁移规则，使升级后的角色持久化内容只保留 canonical permission identifiers。
- **BREAKING**: 已持久化的旧权限标识仅做确定性规范化迁移；此前仅依赖登录态访问的业务 API 现在也将纳入显式资源权限控制，现有非超级管理员角色在未重新授权前可能对这些业务域收到 `403`。
- **BREAKING**: proposal 必须列出完整 canonical permission catalog，而不是旧的窄范围权限子集；依赖旧权限标识、旧角色语义、旧接口示例和旧运维认知的内容都需要同步更新。

## Capabilities

### New Capabilities

### Modified Capabilities
- `access-control-and-configuration`: 将受保护控制面接口的授权模型从 route-family RBAC 调整为平台级 canonical resource permission RBAC，并定义旧权限到新权限的规范化迁移要求。
- `web-user-management`: 将 Web access role 的权限目录、角色配置体验、内置角色语义和角色持久化内容改为平台级 canonical resource permission 模型，同时保持与 AI Agent roles 的概念分离。

## Impact

- Affected backend areas: `internal/security/permissions.go`、鉴权中间件、路由注册、内置角色引导、Web access role 持久化与角色迁移逻辑。
- Affected frontend areas: system settings 中 Web users / Web access roles 的权限展示、角色配置弹窗、权限分组结构和相关 i18n 文案。
- Affected APIs: Web access role 读写 payload 中的 `permissions` 内容语义、受保护接口的权限校验映射、权限目录接口，以及 `/api/auth/validate` 返回的有效权限集合。
- Operational impact: 已存在角色权限需要规范化迁移到 canonical permission catalog；新增受保护业务域权限不会自动授予旧的非超级管理员角色，必须由管理员显式分配。
