## ADDED Requirements

### Requirement: Web user management in system configuration
The system SHALL expose Web user management inside the system configuration module and allow authorized administrators to list, create, update, enable, disable, reset passwords for, and delete operator accounts.

#### Scenario: Administrator creates a Web user
- **WHEN** an authorized administrator submits a valid username, display name, initial password, and role assignment for a new Web user
- **THEN** the system creates the user as a durable operator account and returns the saved identity without exposing stored password material

#### Scenario: Administrator disables a Web user
- **WHEN** an authorized administrator disables an existing Web user
- **THEN** the system marks the account unavailable for future login and preserves the user record for continued administration

#### Scenario: Administrator resets a Web user password
- **WHEN** an authorized administrator resets the password for an existing Web user
- **THEN** the system stores the replacement credential securely and invalidates existing sessions for that user

### Requirement: RBAC role management for Web access
The system SHALL allow authorized administrators to manage Web access roles and their permission grants independently from AI Agent roles.

#### Scenario: Administrator creates an RBAC role
- **WHEN** an authorized administrator creates a new Web access role with a unique name and one or more permission grants
- **THEN** the system stores the role for Web authorization use without creating or modifying any AI Agent role file

#### Scenario: Duplicate RBAC role name is rejected
- **WHEN** an authorized administrator attempts to create a Web access role whose name already exists
- **THEN** the system rejects the request and leaves the existing role unchanged

#### Scenario: Administrator updates role permissions
- **WHEN** an authorized administrator changes the permission grants of an existing Web access role
- **THEN** the system persists the updated permission set for future authorization decisions

### Requirement: User-role assignment and effective permissions
The system SHALL support assigning one or more Web access roles to each Web user and SHALL evaluate the union of assigned permissions as that user's effective RBAC permission set.

#### Scenario: User receives multiple access roles
- **WHEN** an authorized administrator assigns multiple Web access roles to the same user
- **THEN** the user's effective permissions include the union of the assigned role permissions

#### Scenario: Role assignment removal reduces permissions
- **WHEN** an authorized administrator removes an assigned Web access role from a user
- **THEN** the user's effective permissions no longer include permissions contributed only by the removed role

### Requirement: Privileged administration safeguards
The system SHALL prevent destructive user or role changes that would remove all enabled administrative access to Web user management.

#### Scenario: Last enabled super administrator cannot be disabled
- **WHEN** an authorized administrator attempts to disable the last enabled user who still has super-administrator access
- **THEN** the system rejects the change and reports that at least one enabled super administrator must remain

#### Scenario: Last enabled super administrator cannot be deleted
- **WHEN** an authorized administrator attempts to delete the last enabled user who still has super-administrator access
- **THEN** the system rejects the change and preserves that administrative account

### Requirement: Web RBAC roles remain distinct from AI Agent roles
The system SHALL keep Web access roles, user assignments, and administration workflows distinct from the existing AI Agent role concept used by `roles/` and related role-management features.

#### Scenario: Administrator manages Web access roles
- **WHEN** an authorized administrator opens the Web user management capability in system configuration
- **THEN** the system presents Web users and Web access roles as authorization entities and does not relabel AI Agent roles as RBAC roles

#### Scenario: AI Agent roles remain unaffected by Web RBAC changes
- **WHEN** an authorized administrator creates, updates, or deletes a Web access role
- **THEN** the existing AI Agent role catalog and its prompt/tool configuration remain unchanged
