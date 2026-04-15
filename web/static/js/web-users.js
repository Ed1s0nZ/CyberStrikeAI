let webUsers = [];
let webAccessRoles = [];
let securityManagementLoaded = false;
let activeWebUserId = '';
let activeWebUserPasswordResetId = '';
let activeWebAccessRoleId = '';
let pendingSecurityConfirmAction = null;

function securityText(key, fallback) {
    if (typeof window !== 'undefined' && typeof window.t === 'function') {
        return window.t(key);
    }
    return fallback;
}

function getKnownWebPermissions() {
    return [
        {
            id: 'system.super_admin',
            label: securityText('settingsSecurity.permissionSuperAdminLabel', '超级管理员'),
            description: securityText('settingsSecurity.permissionSuperAdminDesc', '拥有所有控制台权限，可绕过其他 Web RBAC 检查。'),
        },
        {
            id: 'system.config.read',
            label: securityText('settingsSecurity.permissionConfigReadLabel', '系统配置读取'),
            description: securityText('settingsSecurity.permissionConfigReadDesc', '查看系统配置、状态信息和安全管理列表。'),
        },
        {
            id: 'system.config.write',
            label: securityText('settingsSecurity.permissionConfigWriteLabel', '系统配置修改'),
            description: securityText('settingsSecurity.permissionConfigWriteDesc', '修改系统配置并应用变更。'),
        },
        {
            id: 'security.users.manage',
            label: securityText('settingsSecurity.permissionUsersManageLabel', 'Web 用户管理'),
            description: securityText('settingsSecurity.permissionUsersManageDesc', '创建、编辑、禁用、重置密码和删除 Web 用户。'),
        },
        {
            id: 'security.roles.manage',
            label: securityText('settingsSecurity.permissionRolesManageLabel', 'Web 访问角色管理'),
            description: securityText('settingsSecurity.permissionRolesManageDesc', '创建、编辑和删除 Web 访问角色。'),
        },
    ];
}

function setSecurityFeedback(targetId, message = '', type = 'info') {
    const node = document.getElementById(targetId);
    if (!node) {
        return;
    }
    if (!message) {
        node.textContent = '';
        node.className = 'settings-feedback';
        node.style.display = 'none';
        return;
    }
    node.textContent = message;
    node.className = `settings-feedback settings-feedback--${type}`;
    node.style.display = 'block';
}

function setFieldError(inputId, hasError) {
    const input = document.getElementById(inputId);
    if (!input) {
        return;
    }
    input.classList.toggle('error', !!hasError);
}

function clearFieldErrors(inputIds) {
    inputIds.forEach(inputId => setFieldError(inputId, false));
}

function openSecurityModal(modalId) {
    const modal = document.getElementById(modalId);
    if (modal) {
        modal.style.display = 'flex';
    }
}

function closeSecurityModal(modalId) {
    const modal = document.getElementById(modalId);
    if (modal) {
        modal.style.display = 'none';
    }
}

function openSecurityConfirmModal({ title, message, confirmLabel, confirmClassName = 'btn-primary', onConfirm }) {
    const titleNode = document.getElementById('security-confirm-title');
    const messageNode = document.getElementById('security-confirm-message');
    const submitButton = document.getElementById('security-confirm-submit');

    pendingSecurityConfirmAction = typeof onConfirm === 'function' ? onConfirm : null;

    if (titleNode) {
        titleNode.textContent = title || securityText('settingsSecurity.confirmDialogTitle', '确认操作');
    }
    if (messageNode) {
        messageNode.textContent = message || '';
    }
    if (submitButton) {
        submitButton.textContent = confirmLabel || securityText('common.confirm', '确认');
        submitButton.className = confirmClassName;
        submitButton.disabled = false;
    }

    openSecurityModal('security-confirm-modal');
}

function closeSecurityConfirmModal() {
    pendingSecurityConfirmAction = null;
    closeSecurityModal('security-confirm-modal');
}

async function confirmSecurityAction() {
    const action = pendingSecurityConfirmAction;
    const submitButton = document.getElementById('security-confirm-submit');
    if (typeof action !== 'function') {
        closeSecurityConfirmModal();
        return;
    }

    pendingSecurityConfirmAction = null;
    if (submitButton) {
        submitButton.disabled = true;
    }

    closeSecurityModal('security-confirm-modal');
    try {
        await action();
    } finally {
        if (submitButton) {
            submitButton.disabled = false;
        }
    }
}

function getCheckedValues(selector) {
    return Array.from(document.querySelectorAll(selector))
        .map(element => element.value)
        .filter(Boolean);
}

function renderSelectableOptions(containerId, items, options = {}) {
    const {
        inputName,
        emptyText,
        selectedValues = [],
    } = options;
    const container = document.getElementById(containerId);
    if (!container) {
        return;
    }

    if (!items.length) {
        container.innerHTML = `<div class="security-empty-state">${escapeHtml(emptyText)}</div>`;
        return;
    }

    const selectedSet = new Set(selectedValues);
    container.innerHTML = items.map(item => `
        <label class="security-option-item">
            <input
                type="checkbox"
                name="${escapeHtml(inputName)}"
                value="${escapeHtml(item.value)}"
                ${selectedSet.has(item.value) ? 'checked' : ''}
            />
            <span class="security-option-content">
                <span class="security-option-title">${escapeHtml(item.label)}</span>
                ${item.description ? `<span class="security-option-description">${escapeHtml(item.description)}</span>` : ''}
            </span>
        </label>
    `).join('');
}

function renderWebUserRoleOptions(selectedRoleIds = []) {
    renderSelectableOptions(
        'web-user-role-options',
        webAccessRoles.map(role => ({
            value: role.id,
            label: role.name || role.id,
            description: role.description || securityText('settingsSecurity.noDescription', '无描述'),
        })),
        {
            inputName: 'web-user-role',
            selectedValues: selectedRoleIds,
            emptyText: securityText('settingsSecurity.noAssignableRoles', '暂无可分配的 Web 访问角色，请先创建角色。'),
        },
    );
}

function renderWebAccessRolePermissionOptions(selectedPermissions = []) {
    const knownPermissions = getKnownWebPermissions();
    const knownIds = new Set(knownPermissions.map(item => item.id));
    const extraPermissions = selectedPermissions
        .filter(permission => !knownIds.has(permission))
        .map(permission => ({
            value: permission,
            label: permission,
            description: '',
        }));

    renderSelectableOptions(
        'web-access-role-permission-options',
        [
            ...knownPermissions.map(item => ({
                value: item.id,
                label: item.label,
                description: item.description,
            })),
            ...extraPermissions,
        ],
        {
            inputName: 'web-access-role-permission',
            selectedValues: selectedPermissions,
            emptyText: securityText('settingsSecurity.noPermissions', '无权限'),
        },
    );
}

function switchSecurityPanel(panel) {
    document.querySelectorAll('.settings-security-tab').forEach(element => {
        element.classList.toggle('active', element.dataset.panel === panel);
    });
    document.querySelectorAll('.settings-security-panel').forEach(element => {
        element.classList.toggle('active', element.id === `settings-security-panel-${panel}`);
    });

    if ((panel === 'users' || panel === 'access-roles') && !securityManagementLoaded) {
        refreshSecurityManagement({ silent: true }).catch(error => {
            console.error('加载安全管理数据失败:', error);
        });
    }
}

async function refreshSecurityManagement(options = {}) {
    const { silent = false } = options;
    const loaders = [loadWebUsers({ silent }), loadWebAccessRoles({ silent })];
    const results = await Promise.allSettled(loaders);
    securityManagementLoaded = true;

    const rejected = results.find(result => result.status === 'rejected');
    if (rejected && !silent) {
        alert(rejected.reason?.message || securityText('settingsSecurity.loadFailed', '加载安全管理数据失败'));
    }
}

async function loadWebUsers(options = {}) {
    const { silent = false } = options;
    const container = document.getElementById('web-users-list');
    if (container) {
        container.innerHTML = `<div class="security-empty-state">${escapeHtml(securityText('common.loading', '加载中…'))}</div>`;
    }

    try {
        const response = await apiFetch('/api/security/web-users');
        const result = await response.json().catch(() => ({}));
        if (!response.ok) {
            throw new Error(result.error || securityText('settingsSecurity.loadUsersFailed', '获取 Web 用户失败'));
        }
        webUsers = Array.isArray(result.users) ? result.users : [];
        renderWebUsers();
        if (!silent) {
            setSecurityFeedback('web-users-feedback');
        }
    } catch (error) {
        console.error('加载 Web 用户失败:', error);
        webUsers = [];
        renderWebUsers();
        setSecurityFeedback('web-users-feedback', error.message || securityText('settingsSecurity.loadUsersFailed', '获取 Web 用户失败'), 'error');
        if (!silent) {
            throw error;
        }
    }
}

async function loadWebAccessRoles(options = {}) {
    const { silent = false } = options;
    const container = document.getElementById('web-access-roles-list');
    if (container) {
        container.innerHTML = `<div class="security-empty-state">${escapeHtml(securityText('common.loading', '加载中…'))}</div>`;
    }

    try {
        const response = await apiFetch('/api/security/web-access-roles');
        const result = await response.json().catch(() => ({}));
        if (!response.ok) {
            throw new Error(result.error || securityText('settingsSecurity.loadRolesFailed', '获取 Web 访问角色失败'));
        }
        webAccessRoles = Array.isArray(result.roles) ? result.roles : [];
        renderWebAccessRoles();
        if (!silent) {
            setSecurityFeedback('web-access-roles-feedback');
        }
    } catch (error) {
        console.error('加载 Web 访问角色失败:', error);
        webAccessRoles = [];
        renderWebAccessRoles();
        setSecurityFeedback('web-access-roles-feedback', error.message || securityText('settingsSecurity.loadRolesFailed', '获取 Web 访问角色失败'), 'error');
        if (!silent) {
            throw error;
        }
    }
}

function renderWebUsers() {
    const container = document.getElementById('web-users-list');
    if (!container) {
        return;
    }
    if (!webUsers.length) {
        container.innerHTML = `<div class="security-empty-state">${escapeHtml(securityText('settingsSecurity.noUsers', '暂无 Web 用户'))}</div>`;
        return;
    }

    container.innerHTML = webUsers.map(user => {
        const roles = Array.isArray(user.roleNames) && user.roleNames.length ? user.roleNames.join(', ') : securityText('settingsSecurity.noRolesAssigned', '未分配角色');
        const permissions = Array.isArray(user.permissions) && user.permissions.length ? user.permissions.join(', ') : securityText('settingsSecurity.noPermissions', '无权限');
        const lastLogin = user.lastLoginAt ? formatSecurityTime(user.lastLoginAt) : securityText('settingsSecurity.neverLoggedIn', '从未登录');
        const statusKey = user.enabled ? 'settingsSecurity.enabled' : 'settingsSecurity.disabled';
        const statusText = user.enabled ? securityText(statusKey, '已启用') : securityText(statusKey, '已禁用');
        return `
            <div class="security-card">
                <div class="security-card-header">
                    <div>
                        <div class="security-card-title">${escapeHtml(user.displayName || user.username || '')}</div>
                        <div class="security-card-meta">${escapeHtml(user.username || '')}</div>
                    </div>
                    <span class="security-status-pill ${user.enabled ? 'is-enabled' : 'is-disabled'}">${escapeHtml(statusText)}</span>
                </div>
                <div class="security-card-detail"><strong>${escapeHtml(securityText('settingsSecurity.rolesLabel', '角色'))}</strong><span>${escapeHtml(roles)}</span></div>
                <div class="security-card-detail"><strong>${escapeHtml(securityText('settingsSecurity.permissionsLabel', '权限'))}</strong><span>${escapeHtml(permissions)}</span></div>
                <div class="security-card-detail"><strong>${escapeHtml(securityText('settingsSecurity.lastLoginLabel', '最近登录'))}</strong><span>${escapeHtml(lastLogin)}</span></div>
                ${user.mustChangePassword ? `<div class="security-card-note">${escapeHtml(securityText('settingsSecurity.mustChangePasswordHint', '该用户下次登录后需要修改密码。'))}</div>` : ''}
                <div class="security-card-actions">
                    <button class="btn-secondary" type="button" onclick="openWebUserModal('${escapeJsString(user.id)}')">${escapeHtml(securityText('common.edit', '编辑'))}</button>
                    <button class="btn-secondary" type="button" onclick="toggleWebUserEnabled('${escapeJsString(user.id)}', ${user.enabled ? 'false' : 'true'})">${escapeHtml(user.enabled ? securityText('settingsSecurity.disableUserBtn', '禁用') : securityText('settingsSecurity.enableUserBtn', '启用'))}</button>
                    <button class="btn-secondary" type="button" onclick="resetWebUserPassword('${escapeJsString(user.id)}')">${escapeHtml(securityText('settingsSecurity.resetPasswordBtn', '重置密码'))}</button>
                    <button class="btn-danger" type="button" onclick="deleteWebUser('${escapeJsString(user.id)}')">${escapeHtml(securityText('common.delete', '删除'))}</button>
                </div>
            </div>
        `;
    }).join('');
}

function renderWebAccessRoles() {
    const container = document.getElementById('web-access-roles-list');
    if (!container) {
        return;
    }
    if (!webAccessRoles.length) {
        container.innerHTML = `<div class="security-empty-state">${escapeHtml(securityText('settingsSecurity.noRoles', '暂无 Web 访问角色'))}</div>`;
        return;
    }

    container.innerHTML = webAccessRoles.map(role => {
        const permissions = Array.isArray(role.permissions) && role.permissions.length ? role.permissions.join(', ') : securityText('settingsSecurity.noPermissions', '无权限');
        const systemHint = role.isSystem ? `<div class="security-card-note">${escapeHtml(securityText('settingsSecurity.systemRoleHint', '系统内置角色仅用于 bootstrap 管理，不允许编辑或删除。'))}</div>` : '';
        return `
            <div class="security-card">
                <div class="security-card-header">
                    <div>
                        <div class="security-card-title">${escapeHtml(role.name || '')}</div>
                        <div class="security-card-meta">${escapeHtml(role.description || securityText('settingsSecurity.noDescription', '无描述'))}</div>
                    </div>
                    <span class="security-status-pill ${role.isSystem ? 'is-system' : 'is-custom'}">${escapeHtml(role.isSystem ? securityText('settingsSecurity.systemRole', '系统') : securityText('settingsSecurity.customRole', '自定义'))}</span>
                </div>
                <div class="security-card-detail"><strong>${escapeHtml(securityText('settingsSecurity.permissionsLabel', '权限'))}</strong><span>${escapeHtml(permissions)}</span></div>
                ${systemHint}
                <div class="security-card-actions">
                    <button class="btn-secondary" type="button" onclick="openWebAccessRoleModal('${escapeJsString(role.id)}')" ${role.isSystem ? 'disabled' : ''}>${escapeHtml(securityText('common.edit', '编辑'))}</button>
                    <button class="btn-danger" type="button" onclick="deleteWebAccessRole('${escapeJsString(role.id)}')" ${role.isSystem ? 'disabled' : ''}>${escapeHtml(securityText('common.delete', '删除'))}</button>
                </div>
            </div>
        `;
    }).join('');
}

function resetWebUserModalForm() {
    activeWebUserId = '';
    setSecurityFeedback('web-user-modal-feedback');
    clearFieldErrors(['web-user-username', 'web-user-display-name', 'web-user-password']);
    const title = document.getElementById('web-user-modal-title');
    const username = document.getElementById('web-user-username');
    const displayName = document.getElementById('web-user-display-name');
    const password = document.getElementById('web-user-password');
    const passwordGroup = document.getElementById('web-user-password-group');
    const enabled = document.getElementById('web-user-enabled');
    const enabledHint = document.getElementById('web-user-enabled-hint');

    if (title) {
        title.textContent = securityText('settingsSecurity.createUserTitle', '新建 Web 用户');
    }
    if (username) {
        username.value = '';
    }
    if (displayName) {
        displayName.value = '';
    }
    if (password) {
        password.value = '';
    }
    if (passwordGroup) {
        passwordGroup.style.display = 'block';
    }
    if (enabled) {
        enabled.checked = true;
        enabled.disabled = true;
    }
    if (enabledHint) {
        enabledHint.textContent = securityText('settingsSecurity.userEnabledCreateHint', '新建用户默认启用，如需禁用请创建后再编辑。');
    }

    renderWebUserRoleOptions([]);
}

async function openWebUserModal(userID = '') {
    try {
        if (!webAccessRoles.length) {
            await loadWebAccessRoles({ silent: true });
        }
    } catch (error) {
        setSecurityFeedback('web-users-feedback', error.message || securityText('settingsSecurity.loadRolesFailed', '获取 Web 访问角色失败'), 'error');
        return;
    }

    resetWebUserModalForm();

    const existing = userID ? webUsers.find(user => user.id === userID) : null;
    const title = document.getElementById('web-user-modal-title');
    const username = document.getElementById('web-user-username');
    const displayName = document.getElementById('web-user-display-name');
    const passwordGroup = document.getElementById('web-user-password-group');
    const enabled = document.getElementById('web-user-enabled');
    const enabledHint = document.getElementById('web-user-enabled-hint');

    if (existing) {
        activeWebUserId = existing.id;
        if (title) {
            title.textContent = securityText('settingsSecurity.editUserTitle', '编辑 Web 用户');
        }
        if (username) {
            username.value = existing.username || '';
        }
        if (displayName) {
            displayName.value = existing.displayName || existing.username || '';
        }
        if (passwordGroup) {
            passwordGroup.style.display = 'none';
        }
        if (enabled) {
            enabled.checked = !!existing.enabled;
            enabled.disabled = false;
        }
        if (enabledHint) {
            enabledHint.textContent = securityText('settingsSecurity.userEnabledEditHint', '关闭后该用户将无法继续登录控制台。');
        }
        renderWebUserRoleOptions(Array.isArray(existing.roleIds) ? existing.roleIds : []);
    }

    openSecurityModal('web-user-modal');
}

function closeWebUserModal() {
    resetWebUserModalForm();
    closeSecurityModal('web-user-modal');
}

function buildWebUserPayload() {
    const username = (document.getElementById('web-user-username')?.value || '').trim();
    const displayName = (document.getElementById('web-user-display-name')?.value || '').trim();
    const password = (document.getElementById('web-user-password')?.value || '').trim();
    const enabled = !!document.getElementById('web-user-enabled')?.checked;
    const roleIds = getCheckedValues('input[name="web-user-role"]:checked');
    const isCreate = !activeWebUserId;

    clearFieldErrors(['web-user-username', 'web-user-display-name', 'web-user-password']);

    if (!username) {
        setFieldError('web-user-username', true);
        throw new Error(securityText('settingsSecurity.usernameRequired', '用户名不能为空'));
    }
    if (!displayName) {
        setFieldError('web-user-display-name', true);
        throw new Error(securityText('settingsSecurity.displayNameRequired', '显示名称不能为空'));
    }
    if (isCreate && !password) {
        setFieldError('web-user-password', true);
        throw new Error(securityText('settingsSecurity.passwordRequired', '密码不能为空'));
    }
    if (isCreate && password.length < 8) {
        setFieldError('web-user-password', true);
        throw new Error(securityText('settingsSecurity.passwordMinLength', '密码长度至少需要 8 位'));
    }
    if (!roleIds.length) {
        throw new Error(securityText('settingsSecurity.roleRequired', '至少分配一个 Web 访问角色'));
    }

    return {
        isCreate,
        payload: {
            username,
            displayName,
            enabled,
            password,
            roleIds,
        },
    };
}

async function saveWebUserModal() {
    try {
        const { isCreate, payload } = buildWebUserPayload();
        setSecurityFeedback('web-user-modal-feedback');

        let response;
        if (isCreate) {
            response = await apiFetch('/api/security/web-users', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    username: payload.username,
                    displayName: payload.displayName,
                    password: payload.password,
                    roleIds: payload.roleIds,
                }),
            });
        } else {
            response = await apiFetch(`/api/security/web-users/${encodeURIComponent(activeWebUserId)}`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    username: payload.username,
                    displayName: payload.displayName,
                    enabled: payload.enabled,
                    roleIds: payload.roleIds,
                }),
            });
        }

        const result = await response.json().catch(() => ({}));
        if (!response.ok) {
            throw new Error(result.error || securityText('settingsSecurity.saveUserFailed', '保存 Web 用户失败'));
        }

        closeWebUserModal();
        await loadWebUsers({ silent: true });
        setSecurityFeedback(
            'web-users-feedback',
            isCreate
                ? securityText('settingsSecurity.userCreated', 'Web 用户已创建')
                : securityText('settingsSecurity.userUpdated', 'Web 用户已更新'),
            'success',
        );
    } catch (error) {
        console.error('保存 Web 用户失败:', error);
        setSecurityFeedback('web-user-modal-feedback', error.message || securityText('settingsSecurity.saveUserFailed', '保存 Web 用户失败'), 'error');
    }
}

function resetWebUserPassword(userID) {
    const user = webUsers.find(item => item.id === userID);
    if (!user) {
        return;
    }

    activeWebUserPasswordResetId = userID;
    setSecurityFeedback('web-user-password-modal-feedback');
    clearFieldErrors(['web-user-reset-password', 'web-user-reset-password-confirm']);

    const title = document.getElementById('web-user-password-modal-title');
    const password = document.getElementById('web-user-reset-password');
    const confirmPassword = document.getElementById('web-user-reset-password-confirm');

    if (title) {
        title.textContent = `${securityText('settingsSecurity.resetPasswordTitle', '重置 Web 用户密码')} · ${user.displayName || user.username || userID}`;
    }
    if (password) {
        password.value = '';
    }
    if (confirmPassword) {
        confirmPassword.value = '';
    }

    openSecurityModal('web-user-password-modal');
}

function closeWebUserPasswordModal() {
    activeWebUserPasswordResetId = '';
    setSecurityFeedback('web-user-password-modal-feedback');
    clearFieldErrors(['web-user-reset-password', 'web-user-reset-password-confirm']);

    const title = document.getElementById('web-user-password-modal-title');
    const password = document.getElementById('web-user-reset-password');
    const confirmPassword = document.getElementById('web-user-reset-password-confirm');

    if (title) {
        title.textContent = securityText('settingsSecurity.resetPasswordTitle', '重置 Web 用户密码');
    }
    if (password) {
        password.value = '';
    }
    if (confirmPassword) {
        confirmPassword.value = '';
    }

    closeSecurityModal('web-user-password-modal');
}

async function submitWebUserPasswordReset() {
    const password = (document.getElementById('web-user-reset-password')?.value || '').trim();
    const confirmPassword = (document.getElementById('web-user-reset-password-confirm')?.value || '').trim();

    clearFieldErrors(['web-user-reset-password', 'web-user-reset-password-confirm']);

    try {
        if (!activeWebUserPasswordResetId) {
            return;
        }
        if (!password) {
            setFieldError('web-user-reset-password', true);
            throw new Error(securityText('settingsSecurity.passwordRequired', '密码不能为空'));
        }
        if (password.length < 8) {
            setFieldError('web-user-reset-password', true);
            throw new Error(securityText('settingsSecurity.passwordMinLength', '密码长度至少需要 8 位'));
        }
        if (password !== confirmPassword) {
            setFieldError('web-user-reset-password', true);
            setFieldError('web-user-reset-password-confirm', true);
            throw new Error(securityText('settingsSecurity.passwordMismatch', '两次输入的密码不一致'));
        }

        setSecurityFeedback('web-user-password-modal-feedback');
        const response = await apiFetch(`/api/security/web-users/${encodeURIComponent(activeWebUserPasswordResetId)}/reset-password`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ password }),
        });
        const result = await response.json().catch(() => ({}));
        if (!response.ok) {
            throw new Error(result.error || securityText('settingsSecurity.resetPasswordFailed', '重置密码失败'));
        }

        closeWebUserPasswordModal();
        await loadWebUsers({ silent: true });
        setSecurityFeedback('web-users-feedback', securityText('settingsSecurity.passwordReset', '密码已重置，旧会话已失效'), 'success');
    } catch (error) {
        console.error('重置密码失败:', error);
        setSecurityFeedback('web-user-password-modal-feedback', error.message || securityText('settingsSecurity.resetPasswordFailed', '重置密码失败'), 'error');
    }
}

function toggleWebUserEnabled(userID, enabled) {
    const user = webUsers.find(item => item.id === userID);
    if (!user) {
        return;
    }

    openSecurityConfirmModal({
        title: securityText('settingsSecurity.confirmDialogTitle', '确认操作'),
        message: enabled
            ? securityText('settingsSecurity.confirmEnableUser', '确定启用这个 Web 用户吗？')
            : securityText('settingsSecurity.confirmDisableUser', '确定禁用这个 Web 用户吗？'),
        confirmLabel: enabled
            ? securityText('settingsSecurity.enableUserBtn', '启用')
            : securityText('settingsSecurity.disableUserBtn', '禁用'),
        confirmClassName: 'btn-primary',
        onConfirm: () => performToggleWebUserEnabled(userID, enabled),
    });
}

async function performToggleWebUserEnabled(userID, enabled) {
    const user = webUsers.find(item => item.id === userID);
    if (!user) {
        return;
    }

    try {
        const response = await apiFetch(`/api/security/web-users/${encodeURIComponent(userID)}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                username: user.username,
                displayName: user.displayName,
                enabled,
                roleIds: Array.isArray(user.roleIds) ? user.roleIds : [],
            }),
        });
        const result = await response.json().catch(() => ({}));
        if (!response.ok) {
            throw new Error(result.error || securityText('settingsSecurity.toggleUserFailed', '更新 Web 用户状态失败'));
        }
        await loadWebUsers({ silent: true });
        setSecurityFeedback(
            'web-users-feedback',
            enabled
                ? securityText('settingsSecurity.userEnabled', 'Web 用户已启用')
                : securityText('settingsSecurity.userDisabled', 'Web 用户已禁用'),
            'success',
        );
    } catch (error) {
        console.error('更新 Web 用户状态失败:', error);
        setSecurityFeedback('web-users-feedback', error.message || securityText('settingsSecurity.toggleUserFailed', '更新 Web 用户状态失败'), 'error');
    }
}

function deleteWebUser(userID) {
    openSecurityConfirmModal({
        title: securityText('settingsSecurity.confirmDialogTitle', '确认操作'),
        message: securityText('settingsSecurity.confirmDeleteUser', '确定删除这个 Web 用户吗？'),
        confirmLabel: securityText('common.delete', '删除'),
        confirmClassName: 'btn-danger',
        onConfirm: () => performDeleteWebUser(userID),
    });
}

async function performDeleteWebUser(userID) {
    try {
        const response = await apiFetch(`/api/security/web-users/${encodeURIComponent(userID)}`, {
            method: 'DELETE',
        });
        const result = await response.json().catch(() => ({}));
        if (!response.ok) {
            throw new Error(result.error || securityText('settingsSecurity.deleteUserFailed', '删除 Web 用户失败'));
        }
        await loadWebUsers({ silent: true });
        setSecurityFeedback('web-users-feedback', securityText('settingsSecurity.userDeleted', 'Web 用户已删除'), 'success');
    } catch (error) {
        console.error('删除 Web 用户失败:', error);
        setSecurityFeedback('web-users-feedback', error.message || securityText('settingsSecurity.deleteUserFailed', '删除 Web 用户失败'), 'error');
    }
}

function resetWebAccessRoleModalForm() {
    activeWebAccessRoleId = '';
    setSecurityFeedback('web-access-role-modal-feedback');
    clearFieldErrors(['web-access-role-name']);

    const title = document.getElementById('web-access-role-modal-title');
    const name = document.getElementById('web-access-role-name');
    const description = document.getElementById('web-access-role-description');

    if (title) {
        title.textContent = securityText('settingsSecurity.createRoleTitle', '新建 Web 访问角色');
    }
    if (name) {
        name.value = '';
    }
    if (description) {
        description.value = '';
    }

    renderWebAccessRolePermissionOptions([]);
}

function openWebAccessRoleModal(roleID = '') {
    resetWebAccessRoleModalForm();

    const existing = roleID ? webAccessRoles.find(role => role.id === roleID) : null;
    const title = document.getElementById('web-access-role-modal-title');
    const name = document.getElementById('web-access-role-name');
    const description = document.getElementById('web-access-role-description');

    if (existing) {
        activeWebAccessRoleId = existing.id;
        if (title) {
            title.textContent = securityText('settingsSecurity.editRoleTitle', '编辑 Web 访问角色');
        }
        if (name) {
            name.value = existing.name || '';
        }
        if (description) {
            description.value = existing.description || '';
        }
        renderWebAccessRolePermissionOptions(Array.isArray(existing.permissions) ? existing.permissions : []);
    }

    openSecurityModal('web-access-role-modal');
}

function closeWebAccessRoleModal() {
    resetWebAccessRoleModalForm();
    closeSecurityModal('web-access-role-modal');
}

function buildWebAccessRolePayload() {
    const name = (document.getElementById('web-access-role-name')?.value || '').trim();
    const description = (document.getElementById('web-access-role-description')?.value || '').trim();
    const permissions = getCheckedValues('input[name="web-access-role-permission"]:checked');

    clearFieldErrors(['web-access-role-name']);

    if (!name) {
        setFieldError('web-access-role-name', true);
        throw new Error(securityText('settingsSecurity.roleNameRequired', '角色名称不能为空'));
    }
    if (!permissions.length) {
        throw new Error(securityText('settingsSecurity.permissionRequired', '至少提供一个权限'));
    }

    return { name, description, permissions };
}

async function saveWebAccessRoleModal() {
    try {
        const payload = buildWebAccessRolePayload();
        const isEdit = !!activeWebAccessRoleId;
        setSecurityFeedback('web-access-role-modal-feedback');

        let response;
        if (isEdit) {
            response = await apiFetch(`/api/security/web-access-roles/${encodeURIComponent(activeWebAccessRoleId)}`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(payload),
            });
        } else {
            response = await apiFetch('/api/security/web-access-roles', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(payload),
            });
        }

        const result = await response.json().catch(() => ({}));
        if (!response.ok) {
            throw new Error(result.error || securityText('settingsSecurity.saveRoleFailed', '保存 Web 访问角色失败'));
        }

        closeWebAccessRoleModal();
        await refreshSecurityManagement({ silent: true });
        setSecurityFeedback(
            'web-access-roles-feedback',
            isEdit
                ? securityText('settingsSecurity.roleUpdated', 'Web 访问角色已更新')
                : securityText('settingsSecurity.roleCreated', 'Web 访问角色已创建'),
            'success',
        );
    } catch (error) {
        console.error('保存 Web 访问角色失败:', error);
        setSecurityFeedback('web-access-role-modal-feedback', error.message || securityText('settingsSecurity.saveRoleFailed', '保存 Web 访问角色失败'), 'error');
    }
}

function deleteWebAccessRole(roleID) {
    openSecurityConfirmModal({
        title: securityText('settingsSecurity.confirmDialogTitle', '确认操作'),
        message: securityText('settingsSecurity.confirmDeleteRole', '确定删除这个 Web 访问角色吗？'),
        confirmLabel: securityText('common.delete', '删除'),
        confirmClassName: 'btn-danger',
        onConfirm: () => performDeleteWebAccessRole(roleID),
    });
}

async function performDeleteWebAccessRole(roleID) {
    try {
        const response = await apiFetch(`/api/security/web-access-roles/${encodeURIComponent(roleID)}`, {
            method: 'DELETE',
        });
        const result = await response.json().catch(() => ({}));
        if (!response.ok) {
            throw new Error(result.error || securityText('settingsSecurity.deleteRoleFailed', '删除 Web 访问角色失败'));
        }
        await refreshSecurityManagement({ silent: true });
        setSecurityFeedback('web-access-roles-feedback', securityText('settingsSecurity.roleDeleted', 'Web 访问角色已删除'), 'success');
    } catch (error) {
        console.error('删除 Web 访问角色失败:', error);
        setSecurityFeedback('web-access-roles-feedback', error.message || securityText('settingsSecurity.deleteRoleFailed', '删除 Web 访问角色失败'), 'error');
    }
}

function formatSecurityTime(value) {
    try {
        return new Date(value).toLocaleString();
    } catch (error) {
        return String(value || '');
    }
}

function escapeJsString(value) {
    return String(value || '').replace(/\\/g, '\\\\').replace(/'/g, "\\'");
}

window.switchSecurityPanel = switchSecurityPanel;
window.refreshSecurityManagement = refreshSecurityManagement;
window.openWebUserModal = openWebUserModal;
window.closeWebUserModal = closeWebUserModal;
window.saveWebUserModal = saveWebUserModal;
window.toggleWebUserEnabled = toggleWebUserEnabled;
window.resetWebUserPassword = resetWebUserPassword;
window.closeWebUserPasswordModal = closeWebUserPasswordModal;
window.submitWebUserPasswordReset = submitWebUserPasswordReset;
window.deleteWebUser = deleteWebUser;
window.openWebAccessRoleModal = openWebAccessRoleModal;
window.closeWebAccessRoleModal = closeWebAccessRoleModal;
window.saveWebAccessRoleModal = saveWebAccessRoleModal;
window.deleteWebAccessRole = deleteWebAccessRole;
window.closeSecurityConfirmModal = closeSecurityConfirmModal;
window.confirmSecurityAction = confirmSecurityAction;
