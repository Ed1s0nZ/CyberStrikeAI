## ADDED Requirements

### Requirement: Legacy Web access role permission migration
The system SHALL normalize previously persisted Web access role grants from retired function-category permission identifiers to the current canonical resource permission catalog before those grants are used for authorization or returned for administration.

#### Scenario: Existing role is upgraded from legacy permission identifiers
- **WHEN** the platform starts with Web access roles that still contain retired function-category permission identifiers from the earlier coarse-grained RBAC model
- **THEN** the system replaces those stored grants with the mapped canonical resource permission identifiers and uses only the normalized permission set for future authorization and administration

#### Scenario: Role write with retired permission identifier is rejected
- **WHEN** an authorized administrator creates or updates a Web access role using a retired function-category permission identifier
- **THEN** the system rejects the request and reports that only canonical resource permission identifiers are accepted

#### Scenario: Role write with unapproved canonical-looking permission identifier is rejected
- **WHEN** an authorized administrator creates or updates a Web access role using a permission identifier such as `task.foo.read` that matches the `domain.resource.action` format but is not in the approved canonical permission catalog
- **THEN** the system rejects the request and reports that only approved canonical permission identifiers are accepted

### Requirement: Canonical permission catalog for Web access roles
The system SHALL provide the canonical Web access role permission catalog as the sole backend contract for permission validation and grouped UI rendering.

#### Scenario: Resource permission catalog follows the canonical naming scheme
- **WHEN** the platform publishes or validates Web access role permission identifiers
- **THEN** every non-legacy permission identifier follows the `domain.resource.action` naming scheme and belongs to the approved canonical permission catalog

#### Scenario: Backend publishes grouped canonical catalog for the permission picker
- **WHEN** an authorized administrator requests the available Web access role permissions
- **THEN** the backend response MUST expose the approved canonical permission catalog as the only source for business-domain, resource, and action grouping used by the UI
- **AND** the response structure MUST identify each permission by backend-returned `domain`、`resource` and `action` semantics so the UI groups and renders permissions from that structure instead of inferring groups client-side from raw identifier strings

#### Scenario: Approved permission catalog exposes platform-wide business domains
- **WHEN** an authorized administrator inspects the available Web access role permission grants
- **THEN** the catalog MUST cover only the approved business domains `intel`、`task`、`vulnerability`、`webshell`、`file`、`mcp`、`knowledge`、`skill`、`agent`、`role` and `system`

#### Scenario: Approved resource families expose only their approved action subsets
- **WHEN** an authorized administrator inspects the canonical permission catalog returned by the backend
- **THEN** the catalog MUST include the approved resource families for those domains
- **AND** each resource family MUST expose only its approved action subset, including `intel.fofa_query{execute}`; `task.batch_queue{read,create,update,delete}`、`task.batch_task{read,create,update,delete}`、`task.conversation{read,create,update,delete}`、`task.group{read,create,update,delete}`、`task.execution{read,start,stop}`、`task.attack_chain{read,create,update,delete,regenerate}`、`task.conversation_result{read}`; `vulnerability.record{read,create,update,delete}`、`vulnerability.stats{read}`; `webshell.connection{read,create,update,delete}`、`webshell.session{read,create,update,delete}`、`webshell.command{execute}`、`webshell.file{execute}`; `file.workspace_entry{read,create,update,delete}`、`file.workspace_content{read,create,update,delete}`; `mcp.gateway{execute}`、`mcp.external_server{read,create,update,delete,test}`; `knowledge.category{read}`、`knowledge.item{read,create,update,delete}`、`knowledge.index{read,create,update,delete}`、`knowledge.retrieval_log{read,delete}`、`knowledge.search{execute}`、`knowledge.stats{read}`; `skill.definition{read,create,update,delete}`、`skill.binding{read}`、`skill.stats{read}`; `agent.run{read,create,update,delete,execute}`、`agent.multi_run{read,create,update,delete,execute}`、`agent.markdown_agent{read,create,update,delete}`、`agent.robot_test{execute}`; `role.agent_role{read,create,update,delete}`; `system.config_settings{read,update}`、`system.runtime_config{apply}`、`system.model_connectivity{test}`、`system.web_user{read,create,update,delete}`、`system.web_user_credential{reset}`、`system.web_access_role{read,create,update,delete}`、`system.terminal{execute}`、`system.api_spec{read}` and `system.super_admin{grant}`
- **AND** the response MUST NOT invent additional actions for an approved resource family beyond that approved subset

## MODIFIED Requirements

### Requirement: RBAC role management for Web access
The system SHALL allow authorized administrators to manage Web access roles and their canonical resource permission grants independently from AI Agent roles.

#### Scenario: Administrator creates an RBAC role
- **WHEN** an authorized administrator creates a new Web access role with a unique name and one or more canonical resource permission grants
- **THEN** the system stores the role for Web authorization use without creating or modifying any AI Agent role file

#### Scenario: Duplicate RBAC role name is rejected
- **WHEN** an authorized administrator attempts to create a Web access role whose name already exists
- **THEN** the system rejects the request and leaves the existing role unchanged

#### Scenario: Administrator updates role permissions
- **WHEN** an authorized administrator changes the canonical resource permission grants of an existing Web access role
- **THEN** the system persists the updated permission set for future authorization decisions

#### Scenario: Web access role permission configuration is grouped by business domain and resource
- **WHEN** an operator edits a Web access role in system settings
- **THEN** the UI MUST present permissions grouped by business domain and resource
- **AND** the submitted payload MUST contain only canonical permission identifiers

### Requirement: User-role assignment and effective permissions
The system SHALL support assigning one or more Web access roles to each Web user and SHALL evaluate the union of assigned canonical resource permission grants as that user's effective RBAC permission set.

#### Scenario: User receives multiple access roles
- **WHEN** an authorized administrator assigns multiple Web access roles to the same user
- **THEN** the user's effective permissions include the union of the assigned canonical resource permission grants

#### Scenario: Role assignment removal reduces permissions
- **WHEN** an authorized administrator removes an assigned Web access role from a user
- **THEN** the user's effective permissions no longer include canonical resource permissions contributed only by the removed role

#### Scenario: Role update revokes dependent sessions
- **WHEN** an authorized administrator changes a Web access role that is currently assigned to one or more users
- **THEN** sessions for affected users are revoked so future requests must re-evaluate the updated canonical resource permission set

### Requirement: Session permission introspection for authenticated Web users
The system SHALL expose the current session's canonical effective permissions so authenticated clients can render RBAC-aware UX from the same permission set used for authorization.

#### Scenario: `/api/auth/validate` returns current-session canonical effective permissions
- **WHEN** an authenticated Web user calls `/api/auth/validate`
- **THEN** the response MUST return the canonical effective permission identifiers that are exactly equal to the current session's effective permission set after assigned-role union and legacy-grant normalization have been applied
- **AND** each returned identifier MUST belong to the approved canonical permission catalog
- **AND** the response MUST NOT include additional approved canonical permission identifiers beyond that exact current session effective permission set
- **AND** the returned permission set MUST NOT contain retired permission identifiers or unapproved canonical-looking identifiers
