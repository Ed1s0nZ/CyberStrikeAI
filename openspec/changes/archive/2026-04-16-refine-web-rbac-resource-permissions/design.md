## Context

当前 Web RBAC 已完成账号、角色、会话和鉴权链路建设，但权限目录仍停留在按功能分类的粗粒度模型，核心权限常量只有 `system.config.read`、`system.config.write`、`security.users.manage`、`security.roles.manage` 和 `system.super_admin`。这会让角色授权停留在“给模块整体放权”的层级，无法表达平台业务域下不同资源与动作的最小权限。

已批准设计要求把 Web RBAC 扩展为平台级 canonical resource permission 模型，覆盖当前受保护业务 API 的真实范围，而不再局限于旧的窄范围控制面子集。同时，Web RBAC 与 AI Agent roles 必须继续保持概念和存储分离。

## Goals / Non-Goals

**Goals:**
- 采用平台级 `domain.resource.action` 资源权限模型覆盖受保护业务域。
- 为每个受保护 Web/API 端点建立显式、可审计的 canonical permission 绑定。
- 让 Web access role 的配置和展示按业务域与资源分组，而不是按旧功能分类分组。
- 对旧权限标识执行确定性规范化迁移，并将角色持久化内容收敛到 canonical permission identifiers。
- 保持 `401` 与 `403` 语义、会话模型和 AI Agent roles 体系不变。

**Non-Goals:**
- 不引入字段级、记录级、租户级或 ABAC 策略模型。
- 不修改 Web 登录、会话和认证模型。
- 不合并 Web RBAC roles 与 AI Agent roles。
- 不按 URL 前缀在运行时推导权限。
- 不对新增受保护业务域向旧的非超级管理员角色自动放权。

## Decisions

### 1. 采用平台级 `domain.resource.action` 资源权限模型

- 业务域固定为 `intel`、`task`、`vulnerability`、`webshell`、`file`、`mcp`、`knowledge`、`skill`、`agent`、`role`、`system`
- 基础动作固定为 `read/create/update/delete`
- 特例动作固定为 `execute/start/stop/test/reset/apply/grant/regenerate`

普通权限统一采用 canonical `domain.resource.action` 命名。权限目录按业务域拆分，在每个业务域内定义具体资源，而不是继续保留 `*.manage` 聚合权限。这样既能表达最小权限，又能避免每条路由都成为一个单独权限概念。

canonical permission catalog 以“resource family + approved action subset”定义，读者必须能够从该表唯一推出最终权限集合：

- `intel`: `intel.fofa_query{execute}`
- `task`: `task.batch_queue{read,create,update,delete}`、`task.batch_task{read,create,update,delete}`、`task.conversation{read,create,update,delete}`、`task.group{read,create,update,delete}`、`task.execution{read,start,stop}`、`task.attack_chain{read,create,update,delete,regenerate}`、`task.conversation_result{read}`
- `vulnerability`: `vulnerability.record{read,create,update,delete}`、`vulnerability.stats{read}`
- `webshell`: `webshell.connection{read,create,update,delete}`、`webshell.session{read,create,update,delete}`、`webshell.command{execute}`、`webshell.file{execute}`
- `file`: `file.workspace_entry{read,create,update,delete}`、`file.workspace_content{read,create,update,delete}`
- `mcp`: `mcp.gateway{execute}`、`mcp.external_server{read,create,update,delete,test}`
- `knowledge`: `knowledge.category{read}`、`knowledge.item{read,create,update,delete}`、`knowledge.index{read,create,update,delete}`、`knowledge.retrieval_log{read,delete}`、`knowledge.search{execute}`、`knowledge.stats{read}`
- `skill`: `skill.definition{read,create,update,delete}`、`skill.binding{read}`、`skill.stats{read}`
- `agent`: `agent.run{read,create,update,delete,execute}`、`agent.multi_run{read,create,update,delete,execute}`、`agent.markdown_agent{read,create,update,delete}`、`agent.robot_test{execute}`
- `role`: `role.agent_role{read,create,update,delete}`
- `system`: `system.config_settings{read,update}`、`system.runtime_config{apply}`、`system.model_connectivity{test}`、`system.web_user{read,create,update,delete}`、`system.web_user_credential{reset}`、`system.web_access_role{read,create,update,delete}`、`system.terminal{execute}`、`system.api_spec{read}`、`system.super_admin{grant}`

### 2. 新增受保护业务域不对旧的非超级管理员角色做自动放权

- 旧权限只做确定性映射
- `system.super_admin` 迁移为 `system.super_admin.grant`
- 新增业务域权限必须由管理员显式分配

升级时，只对旧权限执行固定映射和规范化，不基于“历史上能访问某个功能”这一事实，为非超级管理员角色推断新业务域权限。此前仅依赖登录态访问、但现在被纳入显式资源权限控制的业务 API，在未重新授权前应按缺失权限返回 `403`。

固定迁移示例包括：

- `system.config.read` -> `system.config_settings.read`
- `system.config.write` -> `system.config_settings.update`、`system.runtime_config.apply`、`system.model_connectivity.test`
- `security.users.manage` -> `system.web_user.read`、`system.web_user.create`、`system.web_user.update`、`system.web_user.delete`、`system.web_user_credential.reset`
- `security.roles.manage` -> `system.web_access_role.read`、`system.web_access_role.create`、`system.web_access_role.update`、`system.web_access_role.delete`
- `system.super_admin` -> `system.super_admin.grant`

### 3. 受保护路由继续显式绑定权限，但必须绑定 canonical catalog 中的唯一权限

路由注册层继续使用显式 `RequirePermission(...)` 保护端点，不按 URL 前缀推导权限。每条受保护路由必须绑定且仅绑定一个 approved canonical permission identifier。

动作选择规则如下：

- 列表、详情、统计和非变更读取操作绑定 `read`
- 新建操作绑定 `create`
- 更新、保存、重命名等修改操作绑定 `update`
- 删除操作绑定 `delete`
- 运行态切换操作绑定 `start` 或 `stop`
- 命令执行、Agent 执行、MCP 调用、知识检索、WebShell 命令/文件操作绑定 `execute`
- 连通性和自检操作绑定 `test`
- 配置生效操作绑定 `apply`
- 凭据重置操作绑定 `reset`
- 特权兜底与最后一个超级管理员保护使用 `system.super_admin.grant`
- 语义上区别于一般更新的重建类操作绑定 `regenerate`

`system.super_admin.grant` 继续作为全局绕过键，且必须保留最后一个超级管理员保护判断。

### 4. Web access role permission picker 按业务域和资源分组展示，并只提交 canonical permission identifiers

前端仍可沿用 `permissions: []string` 的接口形态，但角色编辑与查看界面必须以后端提供的 canonical permission catalog 作为唯一来源，按 business domain 和 resource 分组展示权限，再在每组下列出该 resource family 的 approved actions。提交 payload 时只允许 approved canonical permission identifiers，不接受 retired function-category identifiers，也不接受格式合法但不在 approved catalog 中的标识。

这样做的原因：

- 展示结构以后端 canonical permission catalog 为唯一来源，便于管理员理解新增业务域权限边界，并避免前后端各自维护动作子集。
- 保留数组形态可减少 API 改动面，同时不牺牲 canonical 化目标。

## Risks / Trade-offs

- [旧角色在新增业务域上访问收紧] -> 这是已批准的 breaking change，需要通过管理员显式分配来恢复访问。
- [路由权限绑定遗漏导致越权或误拒绝] -> 需要补齐 route-permission registry 与回归测试，确保每条受保护路由都绑定 canonical permission。
- [前端展示与后端目录不一致] -> 以后端导出的 canonical permission catalog 及其分组结构作为唯一来源。
- [权限数量增加提升配置复杂度] -> 通过按业务域和资源分组展示控制复杂度，而不是回退到粗粒度 `*.manage`。

## Migration Plan

1. 定义 canonical permission catalog、旧权限到新权限的固定映射表以及合法性校验。
2. 更新内置角色引导逻辑，使新安装实例只写入 canonical permission identifiers。
3. 在启动期或显式迁移步骤中，将数据库内旧权限标识规范化为 canonical permission identifiers 并去重。
4. 更新路由鉴权绑定、角色接口校验、权限目录接口、`/api/auth/validate` 会话权限返回和角色展示文案。
5. 运行回归测试与手动验证，重点覆盖旧角色迁移、显式权限拒绝、超级管理员绕过和角色配置 UI。

回滚策略：

- 代码回滚前必须保留迁移前后的角色权限备份；如果需要回退到旧版本，需要把新的 canonical permission identifiers 反向转换回旧权限目录，否则旧代码无法理解升级后的角色数据。

## Open Questions

- 当前阶段无新增开放问题。平台级业务域与 canonical action vocabulary 已由已批准设计固定，本 change 只负责与之对齐。
