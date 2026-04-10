// 角色管理相关功能
function _t(key, opts) {
    return typeof window.t === 'function' ? window.t(key, opts) : key;
}
let currentRole = localStorage.getItem('currentRole') || '';
let roles = [];
let rolesSearchKeyword = ''; // 角色搜索关键词
let rolesSearchTimeout = null; // 搜索防抖定时器
let allRoleTools = []; // 存储所有工具列表（用于角色工具选择）
let roleToolsPagination = {
    page: 1,
    pageSize: 20,
    total: 0,
    totalPages: 1
};
let roleToolsSearchKeyword = ''; // 工具搜索关键词
let roleToolStateMap = new Map(); // 工具状态映射：toolKey -> { enabled: boolean, ... }
let roleUsesAllTools = false; // 标记角色是否使用所有工具（当没有配置tools时）
let totalEnabledToolsInMCP = 0; // 已启用的工具总数（从MCP管理中获取，从API响应中获取）
let roleConfiguredTools = new Set(); // 角色配置的工具列表（用于确定哪些工具应该被选中）

// Skills相关
let allRoleSkills = []; // 存储所有skills列表
let roleSkillsSearchKeyword = ''; // Skills搜索关键词
let roleSelectedSkills = new Set(); // 选中的skills集合

// 对角色列表进行排序：默认角色排在第一个，其他按名称排序
function sortRoles(rolesArray) {
    const sortedRoles = [...rolesArray];
    // 将"默认"角色分离出来
    const defaultRole = sortedRoles.find(r => r.name === '默认');
    const otherRoles = sortedRoles.filter(r => r.name !== '默认');
    
    // 其他角色按名称排序，保持固定顺序
    otherRoles.sort((a, b) => {
        const nameA = a.name || '';
        const nameB = b.name || '';
        return nameA.localeCompare(nameB, 'zh-CN');
    });
    
    // 将"默认"角色放在第一个，其他角色按排序后的顺序跟在后面
    const result = defaultRole ? [defaultRole, ...otherRoles] : otherRoles;
    return result;
}

// 加载所有角色
async function loadRoles() {
    try {
        const response = await apiFetch('/api/roles');
        if (!response.ok) {
            throw new Error('加载角色失败');
        }
        const data = await response.json();
        roles = data.roles || [];
        updateRoleSelectorDisplay();
        renderRoleSelectionSidebar(); // 渲染侧边栏角色列表
        return roles;
    } catch (error) {
        console.error('加载角色失败:', error);
        // 提示文案使用 i18n；若此时 i18n 尚未初始化，则回退为可读中文，而不是暴露 key（roles.loadFailed）
        var loadFailedLabel = (typeof window !== 'undefined' && typeof window.t === 'function')
            ? window.t('roles.loadFailed')
            : '加载角色失败';
        showNotification(loadFailedLabel + ': ' + error.message, 'error');
        return [];
    }
}

// 处理角色变更
function handleRoleChange(roleName) {
    const oldRole = currentRole;
    currentRole = roleName || '';
    localStorage.setItem('currentRole', currentRole);
    updateRoleSelectorDisplay();
    renderRoleSelectionSidebar(); // 更新侧边栏选中状态
    
    // 当角色切换时，如果工具列表已加载，标记为需要重新加载
    // 这样下次触发@工具建议时会使用新的角色重新加载工具列表
    if (oldRole !== currentRole && typeof window !== 'undefined') {
        // 通过设置一个标记来通知chat.js需要重新加载工具列表
        window._mentionToolsRoleChanged = true;
    }
}

// 更新角色选择器显示
function updateRoleSelectorDisplay() {
    const roleSelectorBtn = document.getElementById('role-selector-btn');
    const roleSelectorIcon = document.getElementById('role-selector-icon');
    const roleSelectorText = document.getElementById('role-selector-text');
    
    if (!roleSelectorBtn || !roleSelectorIcon || !roleSelectorText) return;

    let selectedRole;
    if (currentRole && currentRole !== '默认') {
        selectedRole = roles.find(r => r.name === currentRole);
    } else {
        selectedRole = roles.find(r => r.name === '默认');
    }

    if (selectedRole) {
        // 使用配置中的图标，如果没有则使用默认图标
        let icon = selectedRole.icon || '🔵';
        // 如果 icon 是 Unicode 转义格式（\U0001F3C6），需要转换为 emoji
        if (icon && typeof icon === 'string') {
            const unicodeMatch = icon.match(/^"?\\U([0-9A-F]{8})"?$/i);
            if (unicodeMatch) {
                try {
                    const codePoint = parseInt(unicodeMatch[1], 16);
                    icon = String.fromCodePoint(codePoint);
                } catch (e) {
                    // 如果转换失败，使用默认图标
                    console.warn('转换 icon Unicode 转义失败:', icon, e);
                    icon = '🔵';
                }
            }
        }
        roleSelectorIcon.textContent = icon;
        const isDefaultRole = selectedRole.name === '默认' || !selectedRole.name;
        const displayName = isDefaultRole && typeof window.t === 'function'
            ? window.t('chat.defaultRole') : (selectedRole.name || (typeof window.t === 'function' ? window.t('chat.defaultRole') : '默认'));
        // 非默认角色时避免被 i18n 的 data-i18n 覆盖成“默认”
        roleSelectorText.setAttribute('data-i18n-skip-text', isDefaultRole ? 'false' : 'true');
        roleSelectorText.textContent = displayName;
    } else {
        // 默认角色
        roleSelectorText.setAttribute('data-i18n-skip-text', 'false');
        roleSelectorIcon.textContent = '🔵';
        roleSelectorText.textContent = typeof window.t === 'function' ? window.t('chat.defaultRole') : '默认';
    }
}

// 渲染主内容区域角色选择列表
function renderRoleSelectionSidebar() {
    const roleList = document.getElementById('role-selection-list');
    if (!roleList) return;

    // 清空列表
    roleList.innerHTML = '';

    // 根据角色配置获取图标，如果没有配置则使用默认图标
    function getRoleIcon(role) {
        if (role.icon) {
            // 如果 icon 是 Unicode 转义格式（\U0001F3C6），需要转换为 emoji
            let icon = role.icon;
            // 检查是否是 Unicode 转义格式（可能包含引号）
            const unicodeMatch = icon.match(/^"?\\U([0-9A-F]{8})"?$/i);
            if (unicodeMatch) {
                try {
                    const codePoint = parseInt(unicodeMatch[1], 16);
                    icon = String.fromCodePoint(codePoint);
                } catch (e) {
                    // 如果转换失败，使用原值
                    console.warn('转换 icon Unicode 转义失败:', icon, e);
                }
            }
            return icon;
        }
        // 如果没有配置图标，根据角色名称的首字符生成默认图标
        // 使用一些通用的默认图标
        return '👤';
    }
    
    // 对角色进行排序：默认角色第一个，其他按名称排序
    const sortedRoles = sortRoles(roles);
    
    // 只显示已启用的角色
    const enabledSortedRoles = sortedRoles.filter(r => r.enabled !== false);
    
    enabledSortedRoles.forEach(role => {
        const isDefaultRole = role.name === '默认';
        const isSelected = isDefaultRole ? (currentRole === '' || currentRole === '默认') : (currentRole === role.name);
        const roleItem = document.createElement('div');
        roleItem.className = 'role-selection-item-main' + (isSelected ? ' selected' : '');
        roleItem.onclick = () => {
            selectRole(role.name);
            closeRoleSelectionPanel(); // 选择后自动关闭面板
        };
        const icon = getRoleIcon(role);
        
        // 处理默认角色的描述
        let description = role.description || _t('roles.noDescription');
        if (isDefaultRole && !role.description) {
            description = _t('roles.defaultRoleDescription');
        }
        
        roleItem.innerHTML = `
            <div class="role-selection-item-icon-main">${icon}</div>
            <div class="role-selection-item-content-main">
                <div class="role-selection-item-name-main">${escapeHtml(role.name)}</div>
                <div class="role-selection-item-description-main">${escapeHtml(description)}</div>
            </div>
            ${isSelected ? '<div class="role-selection-checkmark-main">✓</div>' : ''}
        `;
        roleList.appendChild(roleItem);
    });
}

// 选择角色
function selectRole(roleName) {
    // 将"默认"映射为空字符串（表示默认角色）
    if (roleName === '默认') {
        roleName = '';
    }
    handleRoleChange(roleName);
    renderRoleSelectionSidebar(); // 重新渲染以更新选中状态
}

// 切换角色选择面板显示/隐藏
function toggleRoleSelectionPanel() {
    const panel = document.getElementById('role-selection-panel');
    const roleSelectorBtn = document.getElementById('role-selector-btn');
    if (!panel) return;
    
    const isHidden = panel.style.display === 'none' || !panel.style.display;
    
    if (isHidden) {
        if (typeof closeAgentModePanel === 'function') {
            closeAgentModePanel();
        }
        panel.style.display = 'flex'; // 使用flex布局
        // 添加打开状态的视觉反馈
        if (roleSelectorBtn) {
            roleSelectorBtn.classList.add('active');
        }
        
        // 确保面板渲染后再检查位置
        setTimeout(() => {
            const wrapper = document.querySelector('.role-selector-wrapper');
            if (wrapper) {
                const rect = wrapper.getBoundingClientRect();
                const panelHeight = panel.offsetHeight || 400;
                const viewportHeight = window.innerHeight;
                
                // 如果面板顶部超出视窗，滚动到合适位置
                if (rect.top - panelHeight < 0) {
                    const scrollY = window.scrollY + rect.top - panelHeight - 20;
                    window.scrollTo({ top: Math.max(0, scrollY), behavior: 'smooth' });
                }
            }
        }, 10);
    } else {
        panel.style.display = 'none';
        // 移除打开状态的视觉反馈
        if (roleSelectorBtn) {
            roleSelectorBtn.classList.remove('active');
        }
    }
}

// 关闭角色选择面板（选择角色后自动调用）
function closeRoleSelectionPanel() {
    const panel = document.getElementById('role-selection-panel');
    const roleSelectorBtn = document.getElementById('role-selector-btn');
    if (panel) {
        panel.style.display = 'none';
    }
    if (roleSelectorBtn) {
        roleSelectorBtn.classList.remove('active');
    }
}

// 转义HTML
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// 刷新角色列表
async function refreshRoles() {
    await loadRoles();
    // 检查当前页面是否为角色管理页面
    const currentPage = typeof window.currentPage === 'function' ? window.currentPage() : (window.currentPage || 'chat');
    if (currentPage === 'roles-management') {
        renderRolesList();
    }
    // 始终更新侧边栏角色选择列表
    renderRoleSelectionSidebar();
    showNotification('已刷新', 'success');
}

// 渲染角色列表
function renderRolesList() {
    const rolesList = document.getElementById('roles-list');
    if (!rolesList) return;

    // 过滤角色（根据搜索关键词）
    let filteredRoles = roles;
    if (rolesSearchKeyword) {
        const keyword = rolesSearchKeyword.toLowerCase();
        filteredRoles = roles.filter(role => 
            role.name.toLowerCase().includes(keyword) ||
            (role.description && role.description.toLowerCase().includes(keyword))
        );
    }

    if (filteredRoles.length === 0) {
        rolesList.innerHTML = '<div class="empty-state">' + 
            (rolesSearchKeyword ? _t('roles.noMatchingRoles') : _t('roles.noRoles')) + 
            '</div>';
        return;
    }

    // 对角色进行排序：默认角色第一个，其他按名称排序
    const sortedRoles = sortRoles(filteredRoles);
    
    rolesList.innerHTML = sortedRoles.map(role => {
        // 获取角色图标，如果是Unicode转义格式则转换为emoji
        let roleIcon = role.icon || '👤';
        if (roleIcon && typeof roleIcon === 'string') {
            // 检查是否是 Unicode 转义格式（可能包含引号）
            const unicodeMatch = roleIcon.match(/^"?\\U([0-9A-F]{8})"?$/i);
            if (unicodeMatch) {
                try {
                    const codePoint = parseInt(unicodeMatch[1], 16);
                    roleIcon = String.fromCodePoint(codePoint);
                } catch (e) {
                    // 如果转换失败，使用默认图标
                    console.warn('转换 icon Unicode 转义失败:', roleIcon, e);
                    roleIcon = '👤';
                }
            }
        }

        // 获取工具列表显示
        let toolsDisplay = '';
        let toolsCount = 0;
        if (role.name === '默认') {
            toolsDisplay = _t('roleModal.usingAllTools');
        } else if (role.tools && role.tools.length > 0) {
            toolsCount = role.tools.length;
            // 显示前5个工具名称
            const toolNames = role.tools.slice(0, 5).map(tool => {
                // 如果是外部工具，格式为 external_mcp::tool_name，只显示工具名
                const toolName = tool.includes('::') ? tool.split('::')[1] : tool;
                return escapeHtml(toolName);
            });
            if (toolsCount <= 5) {
                toolsDisplay = toolNames.join(', ');
            } else {
                toolsDisplay = toolNames.join(', ') + _t('roleModal.andNMore', { count: toolsCount });
            }
        } else if (role.mcps && role.mcps.length > 0) {
            toolsCount = role.mcps.length;
            toolsDisplay = _t('roleModal.andNMore', { count: toolsCount });
        } else {
            toolsDisplay = _t('roleModal.usingAllTools');
        }

        return `
        <div class="role-card">
            <div class="role-card-header">
                <h3 class="role-card-title">
                    <span class="role-card-icon">${roleIcon}</span>
                    ${escapeHtml(role.name)}
                </h3>
                <span class="role-card-badge ${role.enabled !== false ? 'enabled' : 'disabled'}">
                    ${role.enabled !== false ? _t('roles.enabled') : _t('roles.disabled')}
                </span>
            </div>
            <div class="role-card-description">${escapeHtml(role.description || _t('roles.noDescriptionShort'))}</div>
            <div class="role-card-tools">
                <span class="role-card-tools-label">${_t('roleModal.toolsLabel')}</span>
                <span class="role-card-tools-value">${toolsDisplay}</span>
            </div>
            <div class="role-card-actions">
                <button class="btn-secondary btn-small" onclick="editRole('${escapeHtml(role.name)}')">${_t('common.edit')}</button>
                ${role.name !== '默认' ? `<button class="btn-secondary btn-small btn-danger" onclick="deleteRole('${escapeHtml(role.name)}')">${_t('common.delete')}</button>` : ''}
            </div>
        </div>
    `;
    }).join('');
}

// 处理角色搜索输入
function handleRolesSearchInput() {
    clearTimeout(rolesSearchTimeout);
    rolesSearchTimeout = setTimeout(() => {
        searchRoles();
    }, 300);
}

// 搜索角色
function searchRoles() {
    const searchInput = document.getElementById('roles-search');
    if (!searchInput) return;
    
    rolesSearchKeyword = searchInput.value.trim();
    const clearBtn = document.getElementById('roles-search-clear');
    if (clearBtn) {
        clearBtn.style.display = rolesSearchKeyword ? 'block' : 'none';
    }
    
    renderRolesList();
}

// 清除角色搜索
function clearRolesSearch() {
    const searchInput = document.getElementById('roles-search');
    if (searchInput) {
        searchInput.value = '';
    }
    rolesSearchKeyword = '';
    const clearBtn = document.getElementById('roles-search-clear');
    if (clearBtn) {
        clearBtn.style.display = 'none';
    }
    renderRolesList();
}

// 生成工具唯一标识符（与settings.js中的getToolKey保持一致）
function getToolKey(tool) {
    // 如果是外部工具，使用 external_mcp::tool.name 作为唯一标识符
    if (tool.is_external && tool.external_mcp) {
        return `${tool.external_mcp}::${tool.name}`;
    }
    // 内置工具直接使用工具名称
    return tool.name;
}

// 保存当前页的工具状态到全局映射
function saveCurrentRolePageToolStates() {
    document.querySelectorAll('#role-tools-list .role-tool-item').forEach(item => {
        const toolKey = item.dataset.toolKey;
        const checkbox = item.querySelector('input[type="checkbox"]');
        if (toolKey && checkbox) {
            const toolName = item.dataset.toolName;
            const isExternal = item.dataset.isExternal === 'true';
            const externalMcp = item.dataset.externalMcp || '';
            const existingState = roleToolStateMap.get(toolKey);
            roleToolStateMap.set(toolKey, {
                enabled: checkbox.checked,
                is_external: isExternal,
                external_mcp: externalMcp,
                name: toolName,
                mcpEnabled: existingState ? existingState.mcpEnabled : true // 保留MCP启用状态
            });
        }
    });
}

// 加载所有工具列表（用于角色工具选择）
async function loadRoleTools(page = 1, searchKeyword = '') {
    try {
        // 在加载新页面之前，先保存当前页的状态到全局映射
        saveCurrentRolePageToolStates();
        
        const pageSize = roleToolsPagination.pageSize;
        let url = `/api/config/tools?page=${page}&page_size=${pageSize}`;
        if (searchKeyword) {
            url += `&search=${encodeURIComponent(searchKeyword)}`;
        }
        
        const response = await apiFetch(url);
        if (!response.ok) {
            throw new Error('获取工具列表失败');
        }
        
        const result = await response.json();
        allRoleTools = result.tools || [];
        roleToolsPagination = {
            page: result.page || page,
            pageSize: result.page_size || pageSize,
            total: result.total || 0,
            totalPages: result.total_pages || 1
        };
        
        // 更新已启用的工具总数（从API响应中获取）
        if (result.total_enabled !== undefined) {
            totalEnabledToolsInMCP = result.total_enabled;
        }
        
        // 初始化工具状态映射（如果工具不在映射中，使用服务器返回的状态）
        // 但要注意：如果工具已经在映射中（比如编辑角色时预先设置的选中工具），则保留映射中的状态
        allRoleTools.forEach(tool => {
            const toolKey = getToolKey(tool);
            if (!roleToolStateMap.has(toolKey)) {
                // 工具不在映射中
                let enabled = false;
                if (roleUsesAllTools) {
                    // 如果使用所有工具，且工具在MCP管理中已启用，则标记为选中
                    enabled = tool.enabled ? true : false;
                } else {
                    // 如果不使用所有工具，只有工具在角色配置的工具列表中才标记为选中
                    enabled = roleConfiguredTools.has(toolKey);
                }
                roleToolStateMap.set(toolKey, {
                    enabled: enabled,
                    is_external: tool.is_external || false,
                    external_mcp: tool.external_mcp || '',
                    name: tool.name,
                    mcpEnabled: tool.enabled // 保存MCP管理中的原始启用状态
                });
            } else {
                // 工具已在映射中（可能是预先设置的选中工具或用户手动选择的），保留映射中的状态
                // 注意：即使使用所有工具，也不要强制覆盖用户已取消的工具选择
                const state = roleToolStateMap.get(toolKey);
                // 如果使用所有工具，且工具在MCP管理中已启用，确保标记为选中
                if (roleUsesAllTools && tool.enabled) {
                    // 使用所有工具时，确保所有已启用的工具都被选中
                    state.enabled = true;
                }
                // 如果不使用所有工具，保留映射中的状态（不要覆盖，因为状态已经在初始化时正确设置了）
                state.is_external = tool.is_external || false;
                state.external_mcp = tool.external_mcp || '';
                state.mcpEnabled = tool.enabled; // 更新MCP管理中的原始启用状态
                if (!state.name || state.name === toolKey.split('::').pop()) {
                    state.name = tool.name; // 更新工具名称
                }
            }
        });
        
        renderRoleToolsList();
        renderRoleToolsPagination();
        updateRoleToolsStats();
    } catch (error) {
        console.error('加载工具列表失败:', error);
        const toolsList = document.getElementById('role-tools-list');
        if (toolsList) {
            toolsList.innerHTML = `<div class="tools-error">${_t('roleModal.loadToolsFailed')}: ${escapeHtml(error.message)}</div>`;
        }
    }
}

// 渲染角色工具选择列表
function renderRoleToolsList() {
    const toolsList = document.getElementById('role-tools-list');
    if (!toolsList) return;
    
    // 清除加载提示和旧内容
    toolsList.innerHTML = '';
    
    const listContainer = document.createElement('div');
    listContainer.className = 'role-tools-list-items';
    listContainer.innerHTML = '';
    
    if (allRoleTools.length === 0) {
        listContainer.innerHTML = '<div class="tools-empty">' + _t('roleModal.noTools') + '</div>';
        toolsList.appendChild(listContainer);
        return;
    }
    
    allRoleTools.forEach(tool => {
        const toolKey = getToolKey(tool);
        const toolItem = document.createElement('div');
        toolItem.className = 'role-tool-item';
        toolItem.dataset.toolKey = toolKey;
        toolItem.dataset.toolName = tool.name;
        toolItem.dataset.isExternal = tool.is_external ? 'true' : 'false';
        toolItem.dataset.externalMcp = tool.external_mcp || '';
        
        // 从状态映射获取工具状态
        const toolState = roleToolStateMap.get(toolKey) || {
            enabled: tool.enabled,
            is_external: tool.is_external || false,
            external_mcp: tool.external_mcp || ''
        };
        
        // 外部工具标签
        let externalBadge = '';
        if (toolState.is_external || tool.is_external) {
            const externalMcpName = toolState.external_mcp || tool.external_mcp || '';
            const badgeText = externalMcpName ? `外部 (${escapeHtml(externalMcpName)})` : '外部';
            const badgeTitle = externalMcpName ? `外部MCP工具 - 来源：${escapeHtml(externalMcpName)}` : '外部MCP工具';
            externalBadge = `<span class="external-tool-badge" title="${badgeTitle}">${badgeText}</span>`;
        }
        
        // 生成唯一的checkbox id
        const checkboxId = `role-tool-${escapeHtml(toolKey).replace(/::/g, '--')}`;
        
        toolItem.innerHTML = `
            <input type="checkbox" id="${checkboxId}" ${toolState.enabled ? 'checked' : ''} 
                   onchange="handleRoleToolCheckboxChange('${escapeHtml(toolKey)}', this.checked)" />
            <div class="role-tool-item-info">
                <div class="role-tool-item-name">
                    ${escapeHtml(tool.name)}
                    ${externalBadge}
                </div>
                <div class="role-tool-item-desc">${escapeHtml(tool.description || '无描述')}</div>
            </div>
        `;
        listContainer.appendChild(toolItem);
    });
    
    toolsList.appendChild(listContainer);
}

// 渲染工具列表分页控件
function renderRoleToolsPagination() {
    const toolsList = document.getElementById('role-tools-list');
    if (!toolsList) return;
    
    // 移除旧的分页控件
    const oldPagination = toolsList.querySelector('.role-tools-pagination');
    if (oldPagination) {
        oldPagination.remove();
    }
    
    // 如果只有一页或没有数据，不显示分页
    if (roleToolsPagination.totalPages <= 1) {
        return;
    }
    
    const pagination = document.createElement('div');
    pagination.className = 'role-tools-pagination';
    
    const { page, totalPages, total } = roleToolsPagination;
    const startItem = (page - 1) * roleToolsPagination.pageSize + 1;
    const endItem = Math.min(page * roleToolsPagination.pageSize, total);
    
    const paginationShowText = _t('roleModal.paginationShow', { start: startItem, end: endItem, total: total }) +
        (roleToolsSearchKeyword ? _t('roleModal.paginationSearch', { keyword: roleToolsSearchKeyword }) : '');
    pagination.innerHTML = `
        <div class="pagination-info">${paginationShowText}</div>
        <div class="pagination-controls">
            <button class="btn-secondary" onclick="loadRoleTools(1, '${escapeHtml(roleToolsSearchKeyword)}')" ${page === 1 ? 'disabled' : ''}>${_t('roleModal.firstPage')}</button>
            <button class="btn-secondary" onclick="loadRoleTools(${page - 1}, '${escapeHtml(roleToolsSearchKeyword)}')" ${page === 1 ? 'disabled' : ''}>${_t('roleModal.prevPage')}</button>
            <span class="pagination-page">${_t('roleModal.pageOf', { page: page, total: totalPages })}</span>
            <button class="btn-secondary" onclick="loadRoleTools(${page + 1}, '${escapeHtml(roleToolsSearchKeyword)}')" ${page === totalPages ? 'disabled' : ''}>${_t('roleModal.nextPage')}</button>
            <button class="btn-secondary" onclick="loadRoleTools(${totalPages}, '${escapeHtml(roleToolsSearchKeyword)}')" ${page === totalPages ? 'disabled' : ''}>${_t('roleModal.lastPage')}</button>
        </div>
    `;
    
    toolsList.appendChild(pagination);
}

// 处理工具checkbox状态变化
function handleRoleToolCheckboxChange(toolKey, enabled) {
    const toolItem = document.querySelector(`.role-tool-item[data-tool-key="${toolKey}"]`);
    if (toolItem) {
        const toolName = toolItem.dataset.toolName;
        const isExternal = toolItem.dataset.isExternal === 'true';
        const externalMcp = toolItem.dataset.externalMcp || '';
        const existingState = roleToolStateMap.get(toolKey);
        roleToolStateMap.set(toolKey, {
            enabled: enabled,
            is_external: isExternal,
            external_mcp: externalMcp,
            name: toolName,
            mcpEnabled: existingState ? existingState.mcpEnabled : true // 保留MCP启用状态
        });
    }
    updateRoleToolsStats();
}

// 全选工具
function selectAllRoleTools() {
    document.querySelectorAll('#role-tools-list input[type="checkbox"]').forEach(checkbox => {
        const toolItem = checkbox.closest('.role-tool-item');
        if (toolItem) {
            const toolKey = toolItem.dataset.toolKey;
            const toolName = toolItem.dataset.toolName;
            const isExternal = toolItem.dataset.isExternal === 'true';
            const externalMcp = toolItem.dataset.externalMcp || '';
            if (toolKey) {
                const existingState = roleToolStateMap.get(toolKey);
                // 只选中在MCP管理中已启用的工具
                const shouldEnable = existingState && existingState.mcpEnabled !== false;
                checkbox.checked = shouldEnable;
                roleToolStateMap.set(toolKey, {
                    enabled: shouldEnable,
                    is_external: isExternal,
                    external_mcp: externalMcp,
                    name: toolName,
                    mcpEnabled: existingState ? existingState.mcpEnabled : true
                });
            }
        }
    });
    updateRoleToolsStats();
}

// 全不选工具
function deselectAllRoleTools() {
    document.querySelectorAll('#role-tools-list input[type="checkbox"]').forEach(checkbox => {
        checkbox.checked = false;
        const toolItem = checkbox.closest('.role-tool-item');
        if (toolItem) {
            const toolKey = toolItem.dataset.toolKey;
            const toolName = toolItem.dataset.toolName;
            const isExternal = toolItem.dataset.isExternal === 'true';
            const externalMcp = toolItem.dataset.externalMcp || '';
            if (toolKey) {
                const existingState = roleToolStateMap.get(toolKey);
                roleToolStateMap.set(toolKey, {
                    enabled: false,
                    is_external: isExternal,
                    external_mcp: externalMcp,
                    name: toolName,
                    mcpEnabled: existingState ? existingState.mcpEnabled : true // 保留MCP启用状态
                });
            }
        }
    });
    updateRoleToolsStats();
}

// 搜索工具
function searchRoleTools(keyword) {
    roleToolsSearchKeyword = keyword;
    const clearBtn = document.getElementById('role-tools-search-clear');
    if (clearBtn) {
        clearBtn.style.display = keyword ? 'block' : 'none';
    }
    loadRoleTools(1, keyword);
}

// 清除搜索
function clearRoleToolsSearch() {
    document.getElementById('role-tools-search').value = '';
    searchRoleTools('');
}

// 更新工具统计信息
function updateRoleToolsStats() {
    const statsEl = document.getElementById('role-tools-stats');
    if (!statsEl) return;
    
    // 统计当前页已选中的工具数
    const currentPageEnabled = Array.from(document.querySelectorAll('#role-tools-list input[type="checkbox"]:checked')).length;
    
    // 统计当前页已启用的工具数（在MCP管理中已启用的工具）
    // 优先从状态映射中获取，如果没有则从工具数据中获取
    let currentPageEnabledInMCP = 0;
    allRoleTools.forEach(tool => {
        const toolKey = getToolKey(tool);
        const state = roleToolStateMap.get(toolKey);
        // 如果工具在MCP管理中已启用（从状态映射或工具数据中获取），计入当前页已启用工具数
        const mcpEnabled = state ? (state.mcpEnabled !== false) : (tool.enabled !== false);
        if (mcpEnabled) {
            currentPageEnabledInMCP++;
        }
    });
    
    // 如果使用所有工具，使用从API获取的已启用工具总数
    if (roleUsesAllTools) {
        // 使用从API响应中获取的已启用工具总数
        const totalEnabled = totalEnabledToolsInMCP || 0;
        // 当前页分母应该是当前页的总工具数（每页20个），而不是当前页已启用的工具数
        const currentPageTotal = document.querySelectorAll('#role-tools-list input[type="checkbox"]').length;
        // 总工具数（所有工具，包括已启用和未启用的）
        const totalTools = roleToolsPagination.total || 0;
        statsEl.innerHTML = `
            <span title="${_t('roleModal.currentPageSelectedTitle')}">✅ ${_t('roleModal.currentPageSelected', { current: currentPageEnabled, total: currentPageTotal })}</span>
            <span title="${_t('roleModal.totalSelectedTitle')}">📊 ${_t('roleModal.totalSelected', { current: totalEnabled, total: totalTools })} <em>${_t('roleModal.usingAllEnabledTools')}</em></span>
        `;
        return;
    }
    
    // 统计角色实际选中的工具数（只统计在MCP管理中已启用的工具）
    let totalSelected = 0;
    roleToolStateMap.forEach(state => {
        // 只统计在MCP管理中已启用且被角色选中的工具
        if (state.enabled && state.mcpEnabled !== false) {
            totalSelected++;
        }
    });
    
    // 如果当前页有未保存的状态，需要合并计算
    document.querySelectorAll('#role-tools-list input[type="checkbox"]').forEach(checkbox => {
        const toolItem = checkbox.closest('.role-tool-item');
        if (toolItem) {
            const toolKey = toolItem.dataset.toolKey;
            const savedState = roleToolStateMap.get(toolKey);
            if (savedState && savedState.enabled !== checkbox.checked && savedState.mcpEnabled !== false) {
                // 状态不一致，使用checkbox状态（但只统计MCP管理中已启用的工具）
                if (checkbox.checked && !savedState.enabled) {
                    totalSelected++;
                } else if (!checkbox.checked && savedState.enabled) {
                    totalSelected--;
                }
            }
        }
    });
    
    // 角色可选择的所有已启用工具总数（应该基于MCP管理中的总数，而不是状态映射）
    // 因为角色可以选择任意已启用的工具，所以总数应该是所有已启用工具的总数
    let totalEnabledForRole = totalEnabledToolsInMCP || 0;
    
    // 如果API返回的总数为0或未设置，尝试从状态映射中统计（作为备选方案）
    if (totalEnabledForRole === 0) {
        roleToolStateMap.forEach(state => {
            // 只统计在MCP管理中已启用的工具
            if (state.mcpEnabled !== false) { // mcpEnabled 为 true 或 undefined（未设置时默认为启用）
                totalEnabledForRole++;
            }
        });
    }
    
    // 当前页分母应该是当前页的总工具数（每页20个），而不是当前页已启用的工具数
    const currentPageTotal = document.querySelectorAll('#role-tools-list input[type="checkbox"]').length;
    // 总工具数（所有工具，包括已启用和未启用的）
    const totalTools = roleToolsPagination.total || 0;
    
    statsEl.innerHTML = `
        <span title="${_t('roleModal.currentPageSelectedTitle')}">✅ ${_t('roleModal.currentPageSelected', { current: currentPageEnabled, total: currentPageTotal })}</span>
        <span title="${_t('roleModal.totalSelectedTitle')}">📊 ${_t('roleModal.totalSelected', { current: totalSelected, total: totalTools })}</span>
    `;
}

// 获取选中的工具列表（返回toolKey数组）
async function getSelectedRoleTools() {
    // 先保存当前页的状态
    saveCurrentRolePageToolStates();
    
    // 如果没有搜索关键词，需要加载所有页面的工具来确保状态映射完整
    // 但为了性能，我们可以只从状态映射中获取已选中的工具
    // 问题是：如果用户只在某些页面选择了工具，其他页面的工具状态可能不在映射中
    
    // 如果总工具数大于已加载的工具数，我们需要确保所有未加载页面的工具也被考虑
    // 但对于角色工具选择，我们只需要获取用户明确选择过的工具
    // 所以直接从状态映射获取已选中的工具即可
    
    // 从状态映射获取所有选中的工具（只返回在MCP管理中已启用的工具）
    const selectedTools = [];
    roleToolStateMap.forEach((state, toolKey) => {
        // 只返回在MCP管理中已启用且被角色选中的工具
        if (state.enabled && state.mcpEnabled !== false) {
            selectedTools.push(toolKey);
        }
    });
    
    // 如果用户可能在其他页面选择了工具，我们需要确保当前页的状态也被保存
    // 但状态映射应该已经包含了所有访问过的页面的状态
    
    return selectedTools;
}

// 设置选中的工具（用于编辑角色时）
function setSelectedRoleTools(selectedToolKeys) {
    const selectedSet = new Set(selectedToolKeys || []);
    
    // 更新状态映射
    roleToolStateMap.forEach((state, toolKey) => {
        state.enabled = selectedSet.has(toolKey);
    });
    
    // 更新当前页的checkbox状态
    document.querySelectorAll('#role-tools-list .role-tool-item').forEach(item => {
        const toolKey = item.dataset.toolKey;
        const checkbox = item.querySelector('input[type="checkbox"]');
        if (toolKey && checkbox) {
            checkbox.checked = selectedSet.has(toolKey);
        }
    });
    
    updateRoleToolsStats();
}

// 显示添加角色模态框
async function showAddRoleModal() {
    const modal = document.getElementById('role-modal');
    if (!modal) return;

    document.getElementById('role-modal-title').textContent = _t('roleModal.addRole');
    document.getElementById('role-name').value = '';
    document.getElementById('role-name').disabled = false;
    document.getElementById('role-description').value = '';
    document.getElementById('role-icon').value = '';
    document.getElementById('role-user-prompt').value = '';
    document.getElementById('role-enabled').checked = true;

    // 添加角色时：显示工具选择界面，隐藏默认角色提示
    const toolsSection = document.getElementById('role-tools-section');
    const defaultHint = document.getElementById('role-tools-default-hint');
    const toolsControls = document.querySelector('.role-tools-controls');
    const toolsList = document.getElementById('role-tools-list');
    const formHint = toolsSection ? toolsSection.querySelector('.form-hint') : null;
    
    if (defaultHint) {
        defaultHint.style.display = 'none';
    }
    if (toolsControls) {
        toolsControls.style.display = 'block';
    }
    if (toolsList) {
        toolsList.style.display = 'block';
    }
    if (formHint) {
        formHint.style.display = 'block';
    }

    // 重置工具状态
    roleToolStateMap.clear();
    roleConfiguredTools.clear(); // 清空角色配置的工具列表
    roleUsesAllTools = false; // 添加角色时默认不使用所有工具
    roleToolsSearchKeyword = '';
    const searchInput = document.getElementById('role-tools-search');
    if (searchInput) {
        searchInput.value = '';
    }
    const clearBtn = document.getElementById('role-tools-search-clear');
    if (clearBtn) {
        clearBtn.style.display = 'none';
    }
    
    // 清空工具列表 DOM，避免 loadRoleTools 中的 saveCurrentRolePageToolStates 读取旧状态
    if (toolsList) {
        toolsList.innerHTML = '';
    }

    // 重置skills状态
    roleSelectedSkills.clear();
    roleSkillsSearchKeyword = '';
    const skillsSearchInput = document.getElementById('role-skills-search');
    if (skillsSearchInput) {
        skillsSearchInput.value = '';
    }
    const skillsClearBtn = document.getElementById('role-skills-search-clear');
    if (skillsClearBtn) {
        skillsClearBtn.style.display = 'none';
    }

    // 加载并渲染工具列表
    await loadRoleTools(1, '');
    
    // 确保工具列表显示
    if (toolsList) {
        toolsList.style.display = 'block';
    }
    
    // 确保统计信息正确更新（显示0/108）
    updateRoleToolsStats();

    // 加载并渲染skills列表
    await loadRoleSkills();

    modal.style.display = 'flex';
}

// 编辑角色
async function editRole(roleName) {
    const role = roles.find(r => r.name === roleName);
    if (!role) {
        showNotification(_t('roleModal.roleNotFound'), 'error');
        return;
    }

    const modal = document.getElementById('role-modal');
    if (!modal) return;

    document.getElementById('role-modal-title').textContent = _t('roleModal.editRole');
    document.getElementById('role-name').value = role.name;
    document.getElementById('role-name').disabled = true; // 编辑时不允许修改名称
    document.getElementById('role-description').value = role.description || '';
    // 处理icon字段：如果是Unicode转义格式，转换为emoji；否则直接使用
    let iconValue = role.icon || '';
    if (iconValue && iconValue.startsWith('\\U')) {
        // 转换Unicode转义格式（如 \U0001F3C6）为emoji
        try {
            const codePoint = parseInt(iconValue.substring(2), 16);
            iconValue = String.fromCodePoint(codePoint);
        } catch (e) {
            // 如果转换失败，使用原值
        }
    }
    document.getElementById('role-icon').value = iconValue;
    document.getElementById('role-user-prompt').value = role.user_prompt || '';
    document.getElementById('role-enabled').checked = role.enabled !== false;

    // 检查是否为默认角色
    const isDefaultRole = roleName === '默认';
    const toolsSection = document.getElementById('role-tools-section');
    const defaultHint = document.getElementById('role-tools-default-hint');
    const toolsControls = document.querySelector('.role-tools-controls');
    const toolsList = document.getElementById('role-tools-list');
    const formHint = toolsSection ? toolsSection.querySelector('.form-hint') : null;
    
    if (isDefaultRole) {
        // 默认角色：隐藏工具选择界面，显示提示信息
        if (defaultHint) {
            defaultHint.style.display = 'block';
        }
        if (toolsControls) {
            toolsControls.style.display = 'none';
        }
        if (toolsList) {
            toolsList.style.display = 'none';
        }
        if (formHint) {
            formHint.style.display = 'none';
        }
    } else {
        // 非默认角色：显示工具选择界面，隐藏提示信息
        if (defaultHint) {
            defaultHint.style.display = 'none';
        }
        if (toolsControls) {
            toolsControls.style.display = 'block';
        }
        if (toolsList) {
            toolsList.style.display = 'block';
        }
        if (formHint) {
            formHint.style.display = 'block';
        }

        // 重置工具状态
        roleToolStateMap.clear();
        roleConfiguredTools.clear(); // 清空角色配置的工具列表
        roleToolsSearchKeyword = '';
        const searchInput = document.getElementById('role-tools-search');
        if (searchInput) {
            searchInput.value = '';
        }
        const clearBtn = document.getElementById('role-tools-search-clear');
        if (clearBtn) {
            clearBtn.style.display = 'none';
        }

        // 优先使用tools字段，如果没有则使用mcps字段（向后兼容）
        const selectedTools = role.tools || (role.mcps && role.mcps.length > 0 ? role.mcps : []);
        
        // 判断是否使用所有工具：如果没有配置tools（或tools为空数组），表示使用所有工具
        roleUsesAllTools = !role.tools || role.tools.length === 0;
        
        // 保存角色配置的工具列表
        if (selectedTools.length > 0) {
            selectedTools.forEach(toolKey => {
                roleConfiguredTools.add(toolKey);
            });
        }
        
        // 如果有选中的工具，先初始化状态映射
        if (selectedTools.length > 0) {
            roleUsesAllTools = false; // 有配置工具，不使用所有工具
            // 将选中的工具添加到状态映射（标记为选中）
            selectedTools.forEach(toolKey => {
                // 如果映射中还没有这个工具，先创建一个默认状态（enabled为true）
                if (!roleToolStateMap.has(toolKey)) {
                    roleToolStateMap.set(toolKey, {
                        enabled: true,
                        is_external: false,
                        external_mcp: '',
                        name: toolKey.split('::').pop() || toolKey // 从toolKey中提取工具名称
                    });
                } else {
                    // 如果已存在，更新为选中状态
                    const state = roleToolStateMap.get(toolKey);
                    state.enabled = true;
                }
            });
        }

        // 加载工具列表（第一页）
        await loadRoleTools(1, '');
        
        // 如果使用所有工具，标记当前页所有已启用的工具为选中
        if (roleUsesAllTools) {
            // 标记当前页所有在MCP管理中已启用的工具为选中
            document.querySelectorAll('#role-tools-list input[type="checkbox"]').forEach(checkbox => {
                const toolItem = checkbox.closest('.role-tool-item');
                if (toolItem) {
                    const toolKey = toolItem.dataset.toolKey;
                    const toolName = toolItem.dataset.toolName;
                    const isExternal = toolItem.dataset.isExternal === 'true';
                    const externalMcp = toolItem.dataset.externalMcp || '';
                    if (toolKey) {
                        const state = roleToolStateMap.get(toolKey);
                        // 只选中在MCP管理中已启用的工具
                        // 如果状态存在，使用状态中的 mcpEnabled；否则假设已启用（因为 loadRoleTools 应该已经初始化了所有工具）
                        const shouldEnable = state ? (state.mcpEnabled !== false) : true;
                        checkbox.checked = shouldEnable;
                        if (state) {
                            state.enabled = shouldEnable;
                        } else {
                            // 如果状态不存在，创建新状态（这种情况不应该发生，因为 loadRoleTools 应该已经初始化了）
                            roleToolStateMap.set(toolKey, {
                                enabled: shouldEnable,
                                is_external: isExternal,
                                external_mcp: externalMcp,
                                name: toolName,
                                mcpEnabled: true // 假设已启用，实际值会在loadRoleTools中更新
                            });
                        }
                    }
                }
            });
            // 更新统计信息，确保显示正确的选中数量
            updateRoleToolsStats();
        } else if (selectedTools.length > 0) {
            // 加载完成后，再次设置选中状态（确保当前页的工具也被正确设置）
            setSelectedRoleTools(selectedTools);
        }
    }

    // 加载并设置skills
    await loadRoleSkills();
    // 设置角色配置的skills
    const selectedSkills = role.skills || [];
    roleSelectedSkills.clear();
    selectedSkills.forEach(skill => {
        roleSelectedSkills.add(skill);
    });
    renderRoleSkills();

    modal.style.display = 'flex';
}

// 关闭角色模态框
function closeRoleModal() {
    const modal = document.getElementById('role-modal');
    if (modal) {
        modal.style.display = 'none';
    }
}

// 获取所有选中的工具（包括未在MCP管理中启用的工具）
function getAllSelectedRoleTools() {
    // 先保存当前页的状态
    saveCurrentRolePageToolStates();
    
    // 从状态映射获取所有选中的工具（不管是否在MCP管理中启用）
    const selectedTools = [];
    roleToolStateMap.forEach((state, toolKey) => {
        if (state.enabled) {
            selectedTools.push({
                key: toolKey,
                name: state.name || toolKey.split('::').pop() || toolKey,
                mcpEnabled: state.mcpEnabled !== false // mcpEnabled 为 false 时是未启用，其他情况视为已启用
            });
        }
    });
    
    return selectedTools;
}

// 检查并获取未在MCP管理中启用的工具
function getDisabledTools(selectedTools) {
    return selectedTools.filter(tool => {
        const state = roleToolStateMap.get(tool.key);
        // 如果 mcpEnabled 明确为 false，则认为是未启用
        return state && state.mcpEnabled === false;
    });
}

// 加载所有工具到状态映射中（用于从使用全部工具切换到部分工具时）
async function loadAllToolsToStateMap() {
    try {
        const pageSize = 100; // 使用较大的页面大小以减少请求次数
        let page = 1;
        let hasMore = true;
        
        // 遍历所有页面获取所有工具
        while (hasMore) {
            const url = `/api/config/tools?page=${page}&page_size=${pageSize}`;
            const response = await apiFetch(url);
            if (!response.ok) {
                throw new Error('获取工具列表失败');
            }
            
            const result = await response.json();
            
            // 将所有工具添加到状态映射中
            result.tools.forEach(tool => {
                const toolKey = getToolKey(tool);
                if (!roleToolStateMap.has(toolKey)) {
                    // 工具不在映射中，根据当前模式初始化
                    let enabled = false;
                    if (roleUsesAllTools) {
                        // 如果使用所有工具，且工具在MCP管理中已启用，则标记为选中
                        enabled = tool.enabled ? true : false;
                    } else {
                        // 如果不使用所有工具，只有工具在角色配置的工具列表中才标记为选中
                        enabled = roleConfiguredTools.has(toolKey);
                    }
                    roleToolStateMap.set(toolKey, {
                        enabled: enabled,
                        is_external: tool.is_external || false,
                        external_mcp: tool.external_mcp || '',
                        name: tool.name,
                        mcpEnabled: tool.enabled // 保存MCP管理中的原始启用状态
                    });
                } else {
                    // 工具已在映射中，更新其他属性但保留enabled状态
                    const state = roleToolStateMap.get(toolKey);
                    state.is_external = tool.is_external || false;
                    state.external_mcp = tool.external_mcp || '';
                    state.mcpEnabled = tool.enabled; // 更新MCP管理中的原始启用状态
                    if (!state.name || state.name === toolKey.split('::').pop()) {
                        state.name = tool.name; // 更新工具名称
                    }
                }
            });
            
            // 检查是否还有更多页面
            if (page >= result.total_pages) {
                hasMore = false;
            } else {
                page++;
            }
        }
    } catch (error) {
        console.error('加载所有工具到状态映射失败:', error);
        throw error;
    }
}

// 保存角色
async function saveRole() {
    const name = document.getElementById('role-name').value.trim();
    if (!name) {
        showNotification(_t('roleModal.roleNameRequired'), 'error');
        return;
    }

    const description = document.getElementById('role-description').value.trim();
    let icon = document.getElementById('role-icon').value.trim();
    // 将emoji转换为Unicode转义格式以匹配YAML格式（如 \U0001F3C6）
    if (icon) {
        // 获取第一个字符的Unicode代码点（处理emoji可能是多个字符的情况）
        const codePoint = icon.codePointAt(0);
        if (codePoint && codePoint > 0x7F) {
            // 转换为8位十六进制格式（\U0001F3C6）
            icon = '\\U' + codePoint.toString(16).toUpperCase().padStart(8, '0');
        }
    }
    const userPrompt = document.getElementById('role-user-prompt').value.trim();
    const enabled = document.getElementById('role-enabled').checked;

    const isEdit = document.getElementById('role-name').disabled;
    
    // 检查是否为默认角色
    const isDefaultRole = name === '默认';
    
    // 检查是否是首次添加角色（排除默认角色后，没有任何用户创建的角色）
    const isFirstUserRole = !isEdit && !isDefaultRole && roles.filter(r => r.name !== '默认').length === 0;
    
    // 默认角色不保存tools字段（使用所有工具）
    // 非默认角色：如果使用所有工具（roleUsesAllTools为true），也不保存tools字段
    let tools = [];
    let disabledTools = []; // 存储未在MCP管理中启用的工具
    
    if (!isDefaultRole) {
        // 保存当前页的状态
        saveCurrentRolePageToolStates();
        
        // 收集所有选中的工具（包括未在MCP管理中启用的）
        let allSelectedTools = getAllSelectedRoleTools();
        
        // 如果是首次添加角色且没有选择工具，默认使用全部工具
        if (isFirstUserRole && allSelectedTools.length === 0) {
            roleUsesAllTools = true;
            showNotification(_t('roleModal.firstRoleNoToolsHint'), 'info');
        } else if (roleUsesAllTools) {
            // 如果当前使用所有工具，需要检查用户是否取消了一些工具
            // 检查状态映射中是否有未选中的已启用工具
            let hasUnselectedTools = false;
            roleToolStateMap.forEach((state) => {
                // 如果工具在MCP管理中已启用但未选中，说明用户取消了该工具
                if (state.mcpEnabled !== false && !state.enabled) {
                    hasUnselectedTools = true;
                }
            });
            
            // 如果用户取消了一些已启用的工具，切换到部分工具模式
            if (hasUnselectedTools) {
                // 在切换之前，需要加载所有工具到状态映射中
                // 这样我们可以正确保存所有工具的状态（除了用户取消的那些）
                await loadAllToolsToStateMap();
                
                // 将所有已启用的工具标记为选中（除了用户已取消的那些）
                // 用户已取消的工具在状态映射中enabled为false，保持不变
                roleToolStateMap.forEach((state, toolKey) => {
                    // 如果工具在MCP管理中已启用，且状态映射中没有明确标记为未选中（即enabled不是false）
                    // 则标记为选中
                    if (state.mcpEnabled !== false && state.enabled !== false) {
                        state.enabled = true;
                    }
                });
                
                roleUsesAllTools = false;
            } else {
                // 即使使用所有工具，也需要加载所有工具到状态映射中，以便检查是否有未启用的工具被选中
                // 这样可以检测用户是否手动选择了一些未启用的工具
                await loadAllToolsToStateMap();
                
                // 检查是否有未启用的工具被手动选中（enabled为true但mcpEnabled为false）
                let hasDisabledToolsSelected = false;
                roleToolStateMap.forEach((state) => {
                    if (state.enabled && state.mcpEnabled === false) {
                        hasDisabledToolsSelected = true;
                    }
                });
                
                // 如果没有未启用的工具被选中，将所有已启用的工具标记为选中（这是使用所有工具的默认行为）
                if (!hasDisabledToolsSelected) {
                    roleToolStateMap.forEach((state) => {
                        if (state.mcpEnabled !== false) {
                            state.enabled = true;
                        }
                    });
                }
                
                // 更新 allSelectedTools，因为现在状态映射中包含了所有工具
                allSelectedTools = getAllSelectedRoleTools();
            }
        }
        
        // 检查哪些工具未在MCP管理中启用（无论是否使用所有工具都要检查）
        disabledTools = getDisabledTools(allSelectedTools);
        
        // 如果有未启用的工具，提示用户
        if (disabledTools.length > 0) {
            const toolNames = disabledTools.map(t => t.name).join('、');
            const message = `以下 ${disabledTools.length} 个工具未在MCP管理中启用，无法在角色中配置：\n\n${toolNames}\n\n请先在"MCP管理"中启用这些工具，然后再在角色中配置。\n\n是否继续保存？（将只保存已启用的工具）`;
            
            if (!confirm(message)) {
                return; // 用户取消保存
            }
        }
        
        // 如果使用所有工具，不需要获取工具列表
        if (!roleUsesAllTools) {
            // 获取选中的工具列表（只包含在MCP管理中已启用的工具）
            tools = await getSelectedRoleTools();
        }
    }

    // 获取选中的skills
    const skills = Array.from(roleSelectedSkills);

    const roleData = {
        name: name,
        description: description,
        icon: icon || undefined, // 如果为空字符串，则不发送该字段
        user_prompt: userPrompt,
        tools: tools, // 默认角色为空数组，表示使用所有工具
        skills: skills, // Skills列表
        enabled: enabled
    };
    const url = isEdit ? `/api/roles/${encodeURIComponent(name)}` : '/api/roles';
    const method = isEdit ? 'PUT' : 'POST';

    try {
        const response = await apiFetch(url, {
            method: method,
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(roleData)
        });

        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || '保存角色失败');
        }

        // 如果有未启用的工具被过滤掉了，提示用户
        if (disabledTools.length > 0) {
            let toolNames = disabledTools.map(t => t.name).join('、');
            // 如果工具名称列表太长，截断显示
            if (toolNames.length > 100) {
                toolNames = toolNames.substring(0, 100) + '...';
            }
            showNotification(
                `${isEdit ? '角色已更新' : '角色已创建'}，但已过滤 ${disabledTools.length} 个未在MCP管理中启用的工具：${toolNames}。请先在"MCP管理"中启用这些工具，然后再在角色中配置。`,
                'warning'
            );
        } else {
            showNotification(isEdit ? '角色已更新' : '角色已创建', 'success');
        }
        
        closeRoleModal();
        await refreshRoles();
    } catch (error) {
        console.error('保存角色失败:', error);
        showNotification('保存角色失败: ' + error.message, 'error');
    }
}

// 删除角色
async function deleteRole(roleName) {
    if (roleName === '默认') {
        showNotification(_t('roleModal.cannotDeleteDefaultRole'), 'error');
        return;
    }

    if (!confirm(`确定要删除角色"${roleName}"吗？此操作不可撤销。`)) {
        return;
    }

    try {
        const response = await apiFetch(`/api/roles/${encodeURIComponent(roleName)}`, {
            method: 'DELETE'
        });

        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || '删除角色失败');
        }

        showNotification('角色已删除', 'success');
        
        // 如果删除的是当前选中的角色,切换到默认角色
        if (currentRole === roleName) {
            handleRoleChange('');
        }

        await refreshRoles();
    } catch (error) {
        console.error('删除角色失败:', error);
        showNotification('删除角色失败: ' + error.message, 'error');
    }
}

// 在页面切换时初始化角色列表
if (typeof switchPage === 'function') {
    const originalSwitchPage = switchPage;
    switchPage = function(page) {
        originalSwitchPage(page);
        if (page === 'roles-management') {
            loadRoles().then(() => renderRolesList());
        }
    };
}

// 点击模态框外部关闭
document.addEventListener('click', (e) => {
    const roleSelectModal = document.getElementById('role-select-modal');
    if (roleSelectModal && e.target === roleSelectModal) {
        closeRoleSelectModal();
    }

    const roleModal = document.getElementById('role-modal');
    if (roleModal && e.target === roleModal) {
        closeRoleModal();
    }

    // 点击角色选择面板外部关闭面板（但不包括角色选择按钮和面板本身）
    const roleSelectionPanel = document.getElementById('role-selection-panel');
    const roleSelectorWrapper = document.querySelector('.role-selector-wrapper');
    if (roleSelectionPanel && roleSelectionPanel.style.display !== 'none' && roleSelectionPanel.style.display) {
        // 检查点击是否在面板或包装器上
        if (!roleSelectorWrapper?.contains(e.target)) {
            closeRoleSelectionPanel();
        }
    }
});

// 页面加载时初始化
document.addEventListener('DOMContentLoaded', () => {
    loadRoles();
    updateRoleSelectorDisplay();
});

// 语言切换后刷新角色选择器显示（默认/自定义角色名）
document.addEventListener('languagechange', () => {
    updateRoleSelectorDisplay();
});

// 获取当前选中的角色（供chat.js使用）
function getCurrentRole() {
    return currentRole || '';
}

// 暴露函数到全局作用域
if (typeof window !== 'undefined') {
    window.getCurrentRole = getCurrentRole;
    window.toggleRoleSelectionPanel = toggleRoleSelectionPanel;
    window.closeRoleSelectionPanel = closeRoleSelectionPanel;
    window.currentSelectedRole = getCurrentRole();
    
    // 监听角色变化，更新全局变量
    const originalHandleRoleChange = handleRoleChange;
    handleRoleChange = function(roleName) {
        originalHandleRoleChange(roleName);
        if (typeof window !== 'undefined') {
            window.currentSelectedRole = getCurrentRole();
        }
    };
}

// ==================== Skills相关函数 ====================

// 加载skills列表
async function loadRoleSkills() {
    try {
        const response = await apiFetch('/api/roles/skills/list');
        if (!response.ok) {
            throw new Error('加载skills列表失败');
        }
        const data = await response.json();
        allRoleSkills = data.skills || [];
        renderRoleSkills();
    } catch (error) {
        console.error('加载skills列表失败:', error);
        allRoleSkills = [];
        const skillsList = document.getElementById('role-skills-list');
        if (skillsList) {
            skillsList.innerHTML = '<div class="skills-error">' + _t('roleModal.loadSkillsFailed') + ': ' + error.message + '</div>';
        }
    }
}

// 渲染skills列表
function renderRoleSkills() {
    const skillsList = document.getElementById('role-skills-list');
    if (!skillsList) return;

    // 过滤skills
    let filteredSkills = allRoleSkills;
    if (roleSkillsSearchKeyword) {
        const keyword = roleSkillsSearchKeyword.toLowerCase();
        filteredSkills = allRoleSkills.filter(skill => 
            skill.toLowerCase().includes(keyword)
        );
    }

    if (filteredSkills.length === 0) {
        skillsList.innerHTML = '<div class="skills-empty">' + 
            (roleSkillsSearchKeyword ? _t('roleModal.noMatchingSkills') : _t('roleModal.noSkillsAvailable')) + 
            '</div>';
        updateRoleSkillsStats();
        return;
    }

    // 渲染skills列表
    skillsList.innerHTML = filteredSkills.map(skill => {
        const isSelected = roleSelectedSkills.has(skill);
        return `
            <div class="role-skill-item" data-skill="${skill}">
                <label class="checkbox-label">
                    <input type="checkbox" class="modern-checkbox" 
                           ${isSelected ? 'checked' : ''} 
                           onchange="toggleRoleSkill('${skill}', this.checked)" />
                    <span class="checkbox-custom"></span>
                    <span class="checkbox-text">${escapeHtml(skill)}</span>
                </label>
            </div>
        `;
    }).join('');

    updateRoleSkillsStats();
}

// 切换skill选中状态
function toggleRoleSkill(skill, checked) {
    if (checked) {
        roleSelectedSkills.add(skill);
    } else {
        roleSelectedSkills.delete(skill);
    }
    updateRoleSkillsStats();
}

// 全选skills
function selectAllRoleSkills() {
    let filteredSkills = allRoleSkills;
    if (roleSkillsSearchKeyword) {
        const keyword = roleSkillsSearchKeyword.toLowerCase();
        filteredSkills = allRoleSkills.filter(skill => 
            skill.toLowerCase().includes(keyword)
        );
    }
    filteredSkills.forEach(skill => {
        roleSelectedSkills.add(skill);
    });
    renderRoleSkills();
}

// 全不选skills
function deselectAllRoleSkills() {
    let filteredSkills = allRoleSkills;
    if (roleSkillsSearchKeyword) {
        const keyword = roleSkillsSearchKeyword.toLowerCase();
        filteredSkills = allRoleSkills.filter(skill => 
            skill.toLowerCase().includes(keyword)
        );
    }
    filteredSkills.forEach(skill => {
        roleSelectedSkills.delete(skill);
    });
    renderRoleSkills();
}

// 搜索skills
function searchRoleSkills(keyword) {
    roleSkillsSearchKeyword = keyword;
    const clearBtn = document.getElementById('role-skills-search-clear');
    if (clearBtn) {
        clearBtn.style.display = keyword ? 'block' : 'none';
    }
    renderRoleSkills();
}

// 清除skills搜索
function clearRoleSkillsSearch() {
    const searchInput = document.getElementById('role-skills-search');
    if (searchInput) {
        searchInput.value = '';
    }
    roleSkillsSearchKeyword = '';
    const clearBtn = document.getElementById('role-skills-search-clear');
    if (clearBtn) {
        clearBtn.style.display = 'none';
    }
    renderRoleSkills();
}

// 更新skills统计信息
function updateRoleSkillsStats() {
    const statsEl = document.getElementById('role-skills-stats');
    if (!statsEl) return;

    let filteredSkills = allRoleSkills;
    if (roleSkillsSearchKeyword) {
        const keyword = roleSkillsSearchKeyword.toLowerCase();
        filteredSkills = allRoleSkills.filter(skill => 
            skill.toLowerCase().includes(keyword)
        );
    }

    const selectedCount = Array.from(roleSelectedSkills).filter(skill => 
        filteredSkills.includes(skill)
    ).length;

    statsEl.textContent = _t('roleModal.skillsSelectedCount', { count: selectedCount, total: filteredSkills.length });
}

// HTML转义函数
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}
