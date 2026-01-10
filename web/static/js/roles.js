// 角色管理相关功能
let currentRole = localStorage.getItem('currentRole') || '';
let roles = [];
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
        showNotification('加载角色失败: ' + error.message, 'error');
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
        roleSelectorText.textContent = selectedRole.name || '默认';
    } else {
        // 默认角色
        roleSelectorIcon.textContent = '🔵';
        roleSelectorText.textContent = '默认';
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
        let description = role.description || '暂无描述';
        if (isDefaultRole && !role.description) {
            description = '默认角色，不额外携带用户提示词，使用默认MCP';
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
}

// 渲染角色列表
function renderRolesList() {
    const rolesList = document.getElementById('roles-list');
    if (!rolesList) return;

    if (roles.length === 0) {
        rolesList.innerHTML = '<div class="empty-state">暂无角色</div>';
        return;
    }

    // 更新统计
    const stats = document.getElementById('roles-stats');
    if (stats) {
        const totalRoles = roles.length;
        const enabledRoles = roles.filter(r => r.enabled !== false).length;
        stats.innerHTML = `
            <div class="role-stat-item">
                <span class="role-stat-label">总角色数</span>
                <span class="role-stat-value">${totalRoles}</span>
            </div>
            <div class="role-stat-item">
                <span class="role-stat-label">已启用</span>
                <span class="role-stat-value">${enabledRoles}</span>
            </div>
        `;
    }

    // 对角色进行排序：默认角色第一个，其他按名称排序
    const sortedRoles = sortRoles(roles);
    
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
        return `
        <div class="role-item">
            <div class="role-item-content">
                <div class="role-item-header">
                    <div class="role-item-name">
                        <span class="role-item-icon" style="margin-right: 8px;">${roleIcon}</span>
                        ${escapeHtml(role.name)}
                    </div>
                    <div class="role-item-badge ${role.enabled !== false ? 'enabled' : 'disabled'}">
                        ${role.enabled !== false ? '已启用' : '已禁用'}
                    </div>
                </div>
                <div class="role-item-description">${escapeHtml(role.description || '无描述')}</div>
                <div class="role-item-details">
                    <div class="role-item-detail">
                        <span class="role-item-detail-label">用户提示词:</span>
                        <span class="role-item-detail-value">${role.user_prompt ? (role.user_prompt.length > 100 ? escapeHtml(role.user_prompt.substring(0, 100)) + '...' : escapeHtml(role.user_prompt)) : '无'}</span>
                    </div>
                    <div class="role-item-detail">
                        <span class="role-item-detail-label">关联的工具:</span>
                        <span class="role-item-detail-value">${
                            role.name === '默认' 
                                ? '使用所有工具' 
                                : (role.tools && role.tools.length > 0 
                                    ? `${role.tools.length} 个工具` 
                                    : (role.mcps && role.mcps.length > 0 
                                        ? `${role.mcps.length} 个工具（兼容旧版）` 
                                        : '使用所有工具'))
                        }</span>
                    </div>
                </div>
            </div>
            <div class="role-item-actions">
                <button class="btn-secondary" onclick="editRole('${escapeHtml(role.name)}')">编辑</button>
                ${role.name !== '默认' ? `<button class="btn-secondary btn-danger" onclick="deleteRole('${escapeHtml(role.name)}')">删除</button>` : ''}
            </div>
        </div>
    `;
    }).join('');
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
                // 但如果使用所有工具，且工具在MCP管理中已启用，确保标记为选中
                const state = roleToolStateMap.get(toolKey);
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
            toolsList.innerHTML = `<div class="tools-error">加载工具列表失败: ${escapeHtml(error.message)}</div>`;
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
        listContainer.innerHTML = '<div class="tools-empty">暂无工具</div>';
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
    
    pagination.innerHTML = `
        <div class="pagination-info">
            显示 ${startItem}-${endItem} / 共 ${total} 个工具${roleToolsSearchKeyword ? ` (搜索: "${escapeHtml(roleToolsSearchKeyword)}")` : ''}
        </div>
        <div class="pagination-controls">
            <button class="btn-secondary" onclick="loadRoleTools(1, '${escapeHtml(roleToolsSearchKeyword)}')" ${page === 1 ? 'disabled' : ''}>首页</button>
            <button class="btn-secondary" onclick="loadRoleTools(${page - 1}, '${escapeHtml(roleToolsSearchKeyword)}')" ${page === 1 ? 'disabled' : ''}>上一页</button>
            <span class="pagination-page">第 ${page} / ${totalPages} 页</span>
            <button class="btn-secondary" onclick="loadRoleTools(${page + 1}, '${escapeHtml(roleToolsSearchKeyword)}')" ${page === totalPages ? 'disabled' : ''}>下一页</button>
            <button class="btn-secondary" onclick="loadRoleTools(${totalPages}, '${escapeHtml(roleToolsSearchKeyword)}')" ${page === totalPages ? 'disabled' : ''}>末页</button>
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
        // 当前页分母应该是当前页已启用的工具数，而不是所有工具数
        const currentPageDenominator = currentPageEnabledInMCP > 0 ? currentPageEnabledInMCP : document.querySelectorAll('#role-tools-list input[type="checkbox"]').length;
        statsEl.innerHTML = `
            <span title="当前页选中的工具数">✅ 当前页已选中: <strong>${currentPageEnabled}</strong> / ${currentPageDenominator}</span>
            <span title="所有已启用工具中选中的工具总数（基于MCP管理）">📊 总计已选中: <strong>${totalEnabled}</strong> / ${totalEnabled} <em>(使用所有已启用工具)</em></span>
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
    
    // 当前页分母应该是当前页已启用的工具数，而不是所有工具数
    const currentPageDenominator = currentPageEnabledInMCP > 0 ? currentPageEnabledInMCP : document.querySelectorAll('#role-tools-list input[type="checkbox"]').length;
    
    statsEl.innerHTML = `
        <span title="当前页选中的工具数（只统计已启用的工具）">✅ 当前页已选中: <strong>${currentPageEnabled}</strong> / ${currentPageDenominator}</span>
        <span title="角色已关联的工具总数（基于角色实际配置）">📊 总计已选中: <strong>${totalSelected}</strong> / ${totalEnabledForRole}</span>
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

    document.getElementById('role-modal-title').textContent = '添加角色';
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

    // 加载并渲染工具列表
    await loadRoleTools(1, '');
    
    // 确保工具列表显示
    if (toolsList) {
        toolsList.style.display = 'block';
    }

    modal.style.display = 'flex';
}

// 编辑角色
async function editRole(roleName) {
    const role = roles.find(r => r.name === roleName);
    if (!role) {
        showNotification('角色不存在', 'error');
        return;
    }

    const modal = document.getElementById('role-modal');
    if (!modal) return;

    document.getElementById('role-modal-title').textContent = '编辑角色';
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
                        const shouldEnable = state && state.mcpEnabled !== false;
                        checkbox.checked = shouldEnable;
                        if (state) {
                            state.enabled = shouldEnable;
                        } else {
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
        } else if (selectedTools.length > 0) {
            // 加载完成后，再次设置选中状态（确保当前页的工具也被正确设置）
            setSelectedRoleTools(selectedTools);
        }
    }

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

// 保存角色
async function saveRole() {
    const name = document.getElementById('role-name').value.trim();
    if (!name) {
        showNotification('角色名称不能为空', 'error');
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

    // 检查是否为默认角色
    const isDefaultRole = name === '默认';
    
    // 默认角色不保存tools字段（使用所有工具）
    // 非默认角色：如果使用所有工具（roleUsesAllTools为true），也不保存tools字段
    let tools = [];
    let disabledTools = []; // 存储未在MCP管理中启用的工具
    
    if (!isDefaultRole && !roleUsesAllTools) {
        // 保存当前页的状态
        saveCurrentRolePageToolStates();
        
        // 收集所有选中的工具（包括未在MCP管理中启用的）
        const allSelectedTools = getAllSelectedRoleTools();
        
        // 检查哪些工具未在MCP管理中启用
        disabledTools = getDisabledTools(allSelectedTools);
        
        // 如果有未启用的工具，提示用户
        if (disabledTools.length > 0) {
            const toolNames = disabledTools.map(t => t.name).join('、');
            const message = `以下 ${disabledTools.length} 个工具未在MCP管理中启用，无法在角色中配置：\n\n${toolNames}\n\n请先在"MCP管理"中启用这些工具，然后再在角色中配置。\n\n是否继续保存？（将只保存已启用的工具）`;
            
            if (!confirm(message)) {
                return; // 用户取消保存
            }
        }
        
        // 获取选中的工具列表（只包含在MCP管理中已启用的工具）
        tools = await getSelectedRoleTools();
    }

    const roleData = {
        name: name,
        description: description,
        icon: icon || undefined, // 如果为空字符串，则不发送该字段
        user_prompt: userPrompt,
        tools: tools, // 默认角色为空数组，表示使用所有工具
        enabled: enabled
    };

    const isEdit = document.getElementById('role-name').disabled;
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
                `${isEdit ? '角色已更新' : '角色已创建'}，但已过滤 ${disabledTools.length} 个未在MCP管理中启用的工具。请先在"MCP管理"中启用这些工具，然后再在角色中配置。`,
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
        showNotification('不能删除默认角色', 'error');
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

