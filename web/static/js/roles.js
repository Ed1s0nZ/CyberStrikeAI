// role managementrelatedfunction
function _t(key, opts) {
    return typeof window.t === 'function' ? window.t(key, opts) : key;
}
let currentRole = localStorage.getItem('currentRole') || '';
let roles = [];
let rolesSearchKeyword = ''; // role searchkeyword
let rolesSearchTimeout = null; // search debounce timer
let allRoleTools = []; // storagealltoollist(used forroletool selection)
let roleToolsPagination = {
    page: 1,
    pageSize: 20,
    total: 0,
    totalPages: 1
};
let roleToolsSearchKeyword = ''; // tool searchkeyword
let roleToolStateMap = new Map(); // tool statemapping:toolKey -> { enabled: boolean, ... }
let roleUsesAllTools = false; // markroleisusealltool(hasconfigurationtoolswhen)
let totalEnabledToolsInMCP = 0; // Enabled's tooltotal(fromMCPmanagementinget,fromAPIresponseinget)
let roleConfiguredTools = new Set(); // role configuration's toollist(used forOKthese toolshouldthisbyselected)

// Skillsrelated
let allRoleSkills = []; // storageallskillslist
let roleSkillsSearchKeyword = ''; // Skillssearchkeyword
let roleSelectedSkills = new Set(); // selected's skills

// forrolelistenterlinesort:default roleat,others sorted by name
function sortRoles(rolesArray) {
    const sortedRoles = [...rolesArray];
 // will"default"roleseparateout
    const defaultRole = sortedRoles.find(r => r.name === 'default');
    const otherRoles = sortedRoles.filter(r => r.name !== 'default');
    
    // otherrolebynamesort,maintainfixedorder
    otherRoles.sort((a, b) => {
        const nameA = a.name || '';
        const nameB = b.name || '';
        return nameA.localeCompare(nameB, 'zh-CN');
    });
    
 // will"default"roleat,otherrolebysortafter's orderatafter
    const result = defaultRole ? [defaultRole, ...otherRoles] : otherRoles;
    return result;
}

// loadallrole
async function loadRoles() {
    try {
        const response = await apiFetch('/api/roles');
        if (!response.ok) {
            throw new Error('Failed to load roles');
        }
        const data = await response.json();
        roles = data.roles || [];
        updateRoleSelectorDisplay();
        renderRoleSelectionSidebar(); // rendersidebarrolelist
        return roles;
    } catch (error) {
        console.error('Failed to load roles:', error);
 // hinttextuse i18n;ifthiswhen i18n not yetinitialize,thenfallbackascanin,notisexpose key(roles.loadFailed)
        var loadFailedLabel = (typeof window !== 'undefined' && typeof window.t === 'function')
            ? window.t('roles.loadFailed')
            : 'Failed to load roles';
        showNotification(loadFailedLabel + ': ' + error.message, 'error');
        return [];
    }
}

// processrole
function handleRoleChange(roleName) {
    const oldRole = currentRole;
    currentRole = roleName || '';
    localStorage.setItem('currentRole', currentRole);
    updateRoleSelectorDisplay();
    renderRoleSelectionSidebar(); // updatesidebarselectedstatus
    
 // roleswitchwhen,iftoollistalreadyload,markasneedre-load
 // this undertimetrigger@toolsuggestwhenwillusenew's rolere-loadtoollist
    if (oldRole !== currentRole && typeof window !== 'undefined') {
 // viasettingsmarknotificationchat.jsneedre-loadtoollist
        window._mentionToolsRoleChanged = true;
    }
}

// updaterole selectionShow 
function updateRoleSelectorDisplay() {
    const roleSelectorBtn = document.getElementById('role-selector-btn');
    const roleSelectorIcon = document.getElementById('role-selector-icon');
    const roleSelectorText = document.getElementById('role-selector-text');
    
    if (!roleSelectorBtn || !roleSelectorIcon || !roleSelectorText) return;

    let selectedRole;
    if (currentRole && currentRole !== 'default') {
        selectedRole = roles.find(r => r.name === currentRole);
    } else {
        selectedRole = roles.find(r => r.name === 'default');
    }

    if (selectedRole) {
 // useconfigurationin's icon,ifhasthenuse default icon
        let icon = selectedRole.icon || '🔵';
        // if icon is Unicode escape format(\U0001F3C6),needconvert to emoji
        if (icon && typeof icon === 'string') {
            const unicodeMatch = icon.match(/^"?\\U([0-9A-F]{8})"?$/i);
            if (unicodeMatch) {
                try {
                    const codePoint = parseInt(unicodeMatch[1], 16);
                    icon = String.fromCodePoint(codePoint);
                } catch (e) {
                    // if conversion fails,use default icon
                    console.warn('convert icon Unicode escape failed:', icon, e);
                    icon = '🔵';
                }
            }
        }
        roleSelectorIcon.textContent = icon;
        const isDefaultRole = selectedRole.name === 'default' || !selectedRole.name;
        const displayName = isDefaultRole && typeof window.t === 'function'
            ? window.t('chat.defaultRole') : (selectedRole.name || (typeof window.t === 'function' ? window.t('chat.defaultRole') : 'default'));
 // default rolewhenavoidby i18n 's data-i18n override"default"
        roleSelectorText.setAttribute('data-i18n-skip-text', isDefaultRole ? 'false' : 'true');
        roleSelectorText.textContent = displayName;
    } else {
        // default role
        roleSelectorText.setAttribute('data-i18n-skip-text', 'false');
        roleSelectorIcon.textContent = '🔵';
        roleSelectorText.textContent = typeof window.t === 'function' ? window.t('chat.defaultRole') : 'default';
    }
}

// rendercontentarearole selectionlist
function renderRoleSelectionSidebar() {
    const roleList = document.getElementById('role-selection-list');
    if (!roleList) return;

    // clearlist
    roleList.innerHTML = '';

 // based onrole configurationgeticon,ifhasconfigurationthenuse default icon
    function getRoleIcon(role) {
        if (role.icon) {
            // if icon is Unicode escape format(\U0001F3C6),needconvert to emoji
            let icon = role.icon;
            // check if it is Unicode escape format(may contain quotes)
            const unicodeMatch = icon.match(/^"?\\U([0-9A-F]{8})"?$/i);
            if (unicodeMatch) {
                try {
                    const codePoint = parseInt(unicodeMatch[1], 16);
                    icon = String.fromCodePoint(codePoint);
                } catch (e) {
 // if conversion fails,use
                    console.warn('convert icon Unicode escape failed:', icon, e);
                }
            }
            return icon;
        }
 // ifhasconfigurationicon,based onrolename's charactergeneratedefaulticon
 // usethese common's defaulticon
        return '👤';
    }
    
 // forroleenterlinesort:default role,others sorted by name
    const sortedRoles = sortRoles(roles);
    
    // onlyShow Enabled's role
    const enabledSortedRoles = sortedRoles.filter(r => r.enabled !== false);
    
    enabledSortedRoles.forEach(role => {
        const isDefaultRole = role.name === 'default';
        const isSelected = isDefaultRole ? (currentRole === '' || currentRole === 'default') : (currentRole === role.name);
        const roleItem = document.createElement('div');
        roleItem.className = 'role-selection-item-main' + (isSelected ? ' selected' : '');
        roleItem.onclick = () => {
            selectRole(role.name);
            closeRoleSelectionPanel(); // selectafterautoClosepanel
        };
        const icon = getRoleIcon(role);
        
        // processdefault role's Description
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

// selectrole
function selectRole(roleName) {
    // will"default"mappingasemptystring(indicatesdefault role)
    if (roleName === 'default') {
        roleName = '';
    }
    handleRoleChange(roleName);
    renderRoleSelectionSidebar(); // re-rendertoupdateselectedstatus
}

// switchrole selectionpanelShow /hide
function toggleRoleSelectionPanel() {
    const panel = document.getElementById('role-selection-panel');
    const roleSelectorBtn = document.getElementById('role-selector-btn');
    if (!panel) return;
    
    const isHidden = panel.style.display === 'none' || !panel.style.display;
    
    if (isHidden) {
        if (typeof closeAgentModePanel === 'function') {
            closeAgentModePanel();
        }
        panel.style.display = 'flex'; // useflexlayout
 // addopenstatus's 
        if (roleSelectorBtn) {
            roleSelectorBtn.classList.add('active');
        }
        
        // ensurepanelrenderafterthencheckposition
        setTimeout(() => {
            const wrapper = document.querySelector('.role-selector-wrapper');
            if (wrapper) {
                const rect = wrapper.getBoundingClientRect();
                const panelHeight = panel.offsetHeight || 400;
                const viewportHeight = window.innerHeight;
                
 // ifpaneltopexceedoutviewport,scrolltoposition
                if (rect.top - panelHeight < 0) {
                    const scrollY = window.scrollY + rect.top - panelHeight - 20;
                    window.scrollTo({ top: Math.max(0, scrollY), behavior: 'smooth' });
                }
            }
        }, 10);
    } else {
        panel.style.display = 'none';
 // removeopenstatus's 
        if (roleSelectorBtn) {
            roleSelectorBtn.classList.remove('active');
        }
    }
}

// Closerole selectionpanel(selectroleafterautocall)
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

// escapeHTML
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Refreshrolelist
async function refreshRoles() {
    await loadRoles();
 // checkcurrentpageisasrolemanagement page
    const currentPage = typeof window.currentPage === 'function' ? window.currentPage() : (window.currentPage || 'chat');
    if (currentPage === 'roles-management') {
        renderRolesList();
    }
    // alwaysupdatesidebarrole selectionlist
    renderRoleSelectionSidebar();
    showNotification('Refreshed', 'success');
}

// renderrolelist
function renderRolesList() {
    const rolesList = document.getElementById('roles-list');
    if (!rolesList) return;

    // filterrole(based onsearchkeyword)
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

 // forroleenterlinesort:default role,others sorted by name
    const sortedRoles = sortRoles(filteredRoles);
    
    rolesList.innerHTML = sortedRoles.map(role => {
        // getroleicon,if it isUnicodeescape formatthenconvert toemoji
        let roleIcon = role.icon || '👤';
        if (roleIcon && typeof roleIcon === 'string') {
            // check if it is Unicode escape format(may contain quotes)
            const unicodeMatch = roleIcon.match(/^"?\\U([0-9A-F]{8})"?$/i);
            if (unicodeMatch) {
                try {
                    const codePoint = parseInt(unicodeMatch[1], 16);
                    roleIcon = String.fromCodePoint(codePoint);
                } catch (e) {
                    // if conversion fails,use default icon
                    console.warn('convert icon Unicode escape failed:', roleIcon, e);
                    roleIcon = '👤';
                }
            }
        }

        // gettoollistShow 
        let toolsDisplay = '';
        let toolsCount = 0;
        if (role.name === 'default') {
            toolsDisplay = _t('roleModal.usingAllTools');
        } else if (role.tools && role.tools.length > 0) {
            toolsCount = role.tools.length;
            // Show before5tool name
            const toolNames = role.tools.slice(0, 5).map(tool => {
 // if it is an external tool,formatas external_mcp::tool_name,onlyShow tool
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
                ${role.name !== 'default' ? `<button class="btn-secondary btn-small btn-danger" onclick="deleteRole('${escapeHtml(role.name)}')">${_t('common.delete')}</button>` : ''}
            </div>
        </div>
    `;
    }).join('');
}

// processrole searchinput
function handleRolesSearchInput() {
    clearTimeout(rolesSearchTimeout);
    rolesSearchTimeout = setTimeout(() => {
        searchRoles();
    }, 300);
}

// searchrole
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

// clearrole search
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

// generatetooluniqueidentifier(andsettings.jsin's getToolKeymaintainconsistent)
function getToolKey(tool) {
    // if it is an external tool,use external_mcp::tool.name asuniqueidentifier
    if (tool.is_external && tool.external_mcp) {
        return `${tool.external_mcp}::${tool.name}`;
    }
 // withintooluse directlytool name
    return tool.name;
}

// Savecurrentpage's tool statetoglobalmapping
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
                mcpEnabled: existingState ? existingState.mcpEnabled : true // preserveMCPenabled state
            });
        }
    });
}

// loadalltoollist(used forroletool selection)
async function loadRoleTools(page = 1, searchKeyword = '') {
    try {
 // atloadnewpagebefore,save current page state to global mapping first
        saveCurrentRolePageToolStates();
        
        const pageSize = roleToolsPagination.pageSize;
        let url = `/api/config/tools?page=${page}&page_size=${pageSize}`;
        if (searchKeyword) {
            url += `&search=${encodeURIComponent(searchKeyword)}`;
        }
        
        const response = await apiFetch(url);
        if (!response.ok) {
            throw new Error('Failed to get tool list');
        }
        
        const result = await response.json();
        allRoleTools = result.tools || [];
        roleToolsPagination = {
            page: result.page || page,
            pageSize: result.page_size || pageSize,
            total: result.total || 0,
            totalPages: result.total_pages || 1
        };
        
        // updateEnabled's tooltotal(fromAPIresponseinget)
        if (result.total_enabled !== undefined) {
            totalEnabledToolsInMCP = result.total_enabled;
        }
        
 // initializetool statemapping(iftoolnotatmappingin,useservicereturn's status)
 // butneednote:iftoolalreadyatmappingin(likeEditrolewhenfirstsettings's selectedtool),thenpreservemappingin's status
        allRoleTools.forEach(tool => {
            const toolKey = getToolKey(tool);
            if (!roleToolStateMap.has(toolKey)) {
                // toolnotatmappingin
                let enabled = false;
                if (roleUsesAllTools) {
                    // if using all tools,and tool is inMCPenabled in management,mark as selected
                    enabled = tool.enabled ? true : false;
                } else {
                    // if not using all tools,onlyhastoolatrole configuration's toollistinonly markasselected
                    enabled = roleConfiguredTools.has(toolKey);
                }
                roleToolStateMap.set(toolKey, {
                    enabled: enabled,
                    is_external: tool.is_external || false,
                    external_mcp: tool.external_mcp || '',
                    name: tool.name,
                    mcpEnabled: tool.enabled // SaveMCPoriginal enabled state in management
                });
            } else {
 // toolalreadyatmappingin(possiblyisfirstsettings's selectedtoolorusemanualselect's ),preservemappingin's status
 // note:even ifusealltool,alsonotneedforceoverrideusealreadyCancel's tool selection
                const state = roleToolStateMap.get(toolKey);
                // if using all tools,and tool is inMCPenabled in management,ensuremarkasselected
                if (roleUsesAllTools && tool.enabled) {
                    // usealltoolwhen,ensureallEnabled's toolall byselected
                    state.enabled = true;
                }
                // if not using all tools,preservemappingin's status(notneedoverride,becausestatusalreadyatinitializewhencorrectsettings)
                state.is_external = tool.is_external || false;
                state.external_mcp = tool.external_mcp || '';
                state.mcpEnabled = tool.enabled; // updateMCPoriginal enabled state in management
                if (!state.name || state.name === toolKey.split('::').pop()) {
                    state.name = tool.name; // updatetool name
                }
            }
        });
        
        renderRoleToolsList();
        renderRoleToolsPagination();
        updateRoleToolsStats();
    } catch (error) {
        console.error('Failed to load tool list:', error);
        const toolsList = document.getElementById('role-tools-list');
        if (toolsList) {
            toolsList.innerHTML = `<div class="tools-error">${_t('roleModal.loadToolsFailed')}: ${escapeHtml(error.message)}</div>`;
        }
    }
}

// renderroletool selectionlist
function renderRoleToolsList() {
    const toolsList = document.getElementById('role-tools-list');
    if (!toolsList) return;
    
    // clearloadhintandoldcontent
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
        
        // fromstatusmappinggettool state
        const toolState = roleToolStateMap.get(toolKey) || {
            enabled: tool.enabled,
            is_external: tool.is_external || false,
            external_mcp: tool.external_mcp || ''
        };
        
        // Externaltooltab
        let externalBadge = '';
        if (toolState.is_external || tool.is_external) {
            const externalMcpName = toolState.external_mcp || tool.external_mcp || '';
            const badgeText = externalMcpName ? `External (${escapeHtml(externalMcpName)})` : 'External';
            const badgeTitle = externalMcpName ? `ExternalMCPtool - Source:${escapeHtml(externalMcpName)}` : 'ExternalMCPtool';
            externalBadge = `<span class="external-tool-badge" title="${badgeTitle}">${badgeText}</span>`;
        }
        
        // generateunique's checkbox id
        const checkboxId = `role-tool-${escapeHtml(toolKey).replace(/::/g, '--')}`;
        
        toolItem.innerHTML = `
            <input type="checkbox" id="${checkboxId}" ${toolState.enabled ? 'checked' : ''} 
                   onchange="handleRoleToolCheckboxChange('${escapeHtml(toolKey)}', this.checked)" />
            <div class="role-tool-item-info">
                <div class="role-tool-item-name">
                    ${escapeHtml(tool.name)}
                    ${externalBadge}
                </div>
                <div class="role-tool-item-desc">${escapeHtml(tool.description || 'No description')}</div>
            </div>
        `;
        listContainer.appendChild(toolItem);
    });
    
    toolsList.appendChild(listContainer);
}

// rendertoollistpaginationcontrols
function renderRoleToolsPagination() {
    const toolsList = document.getElementById('role-tools-list');
    if (!toolsList) return;
    
    // remove old pagination controls
    const oldPagination = toolsList.querySelector('.role-tools-pagination');
    if (oldPagination) {
        oldPagination.remove();
    }
    
 // ifonlyhaspageorhasdata,hide pagination
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

// processtoolcheckboxstatuschange
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
            mcpEnabled: existingState ? existingState.mcpEnabled : true // preserveMCPenabled state
        });
    }
    updateRoleToolsStats();
}

// select alltool
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
                // onlyselectedatMCPtools enabled in management
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

// nottool
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
                    mcpEnabled: existingState ? existingState.mcpEnabled : true // preserveMCPenabled state
                });
            }
        }
    });
    updateRoleToolsStats();
}

// searchtool
function searchRoleTools(keyword) {
    roleToolsSearchKeyword = keyword;
    const clearBtn = document.getElementById('role-tools-search-clear');
    if (clearBtn) {
        clearBtn.style.display = keyword ? 'block' : 'none';
    }
    loadRoleTools(1, keyword);
}

// clear search
function clearRoleToolsSearch() {
    document.getElementById('role-tools-search').value = '';
    searchRoleTools('');
}

// updatetoolstatisticsInfo
function updateRoleToolsStats() {
    const statsEl = document.getElementById('role-tools-stats');
    if (!statsEl) return;
    
 // statisticscurrentpagealreadyselected's tool
    const currentPageEnabled = Array.from(document.querySelectorAll('#role-tools-list input[type="checkbox"]:checked')).length;
    
 // statisticscurrentpageEnabled's tool(atMCPtools enabled in management)
 // preferfromstatusmappinginget,ifhasthenfromtooldatainget
    let currentPageEnabledInMCP = 0;
    allRoleTools.forEach(tool => {
        const toolKey = getToolKey(tool);
        const state = roleToolStateMap.get(toolKey);
 // if tool is inMCPenabled in management(fromstatusmappingortooldatainget),incurrentpageEnabledtool
        const mcpEnabled = state ? (state.mcpEnabled !== false) : (tool.enabled !== false);
        if (mcpEnabled) {
            currentPageEnabledInMCP++;
        }
    });
    
    // if using all tools,usefromAPIget's Enabledtooltotal
    if (roleUsesAllTools) {
        // usefromAPIresponseinget's Enabledtooltotal
        const totalEnabled = totalEnabledToolsInMCP || 0;
 // currentpagedenominatorshouldthisiscurrentpage's tool(eachpage20),notiscurrentpageEnabled's tool
        const currentPageTotal = document.querySelectorAll('#role-tools-list input[type="checkbox"]').length;
 // tool(alltool,includeEnabledandNot enabled's )
        const totalTools = roleToolsPagination.total || 0;
        statsEl.innerHTML = `
            <span title="${_t('roleModal.currentPageSelectedTitle')}">✅ ${_t('roleModal.currentPageSelected', { current: currentPageEnabled, total: currentPageTotal })}</span>
            <span title="${_t('roleModal.totalSelectedTitle')}">📊 ${_t('roleModal.totalSelected', { current: totalEnabled, total: totalTools })} <em>${_t('roleModal.usingAllEnabledTools')}</em></span>
        `;
        return;
    }
    
 // statisticsroleactualselected's tool(only count inMCPtools enabled in management)
    let totalSelected = 0;
    roleToolStateMap.forEach(state => {
        // only count inMCPenabled in managementandbyroleselected's tool
        if (state.enabled && state.mcpEnabled !== false) {
            totalSelected++;
        }
    });
    
    // ifcurrentpagehasnot yetSave's status,needmergecalculate
    document.querySelectorAll('#role-tools-list input[type="checkbox"]').forEach(checkbox => {
        const toolItem = checkbox.closest('.role-tool-item');
        if (toolItem) {
            const toolKey = toolItem.dataset.toolKey;
            const savedState = roleToolStateMap.get(toolKey);
            if (savedState && savedState.enabled !== checkbox.checked && savedState.mcpEnabled !== false) {
                // statusnotconsistent,usecheckboxstatus(butonlystatisticsMCPtools enabled in management)
                if (checkbox.checked && !savedState.enabled) {
                    totalSelected++;
                } else if (!checkbox.checked && savedState.enabled) {
                    totalSelected--;
                }
            }
        }
    });
    
 // rolecanselect's allEnabledtooltotal(shouldthisatMCPmanagementin's total,notisstatusmapping)
 // becauserolecantoselectEnabled's tool,sototalshouldthisisallEnabledtool's total
    let totalEnabledForRole = totalEnabledToolsInMCP || 0;
    
 // ifAPIreturn's totalas0ornot yetsettings,tryfromstatusmappinginstatistics(as)
    if (totalEnabledForRole === 0) {
        roleToolStateMap.forEach(state => {
            // only count inMCPtools enabled in management
            if (state.mcpEnabled !== false) { // mcpEnabled as true or undefined(not yetsettingswhendefaults toenable)
                totalEnabledForRole++;
            }
        });
    }
    
 // currentpagedenominatorshouldthisiscurrentpage's tool(eachpage20),notiscurrentpageEnabled's tool
    const currentPageTotal = document.querySelectorAll('#role-tools-list input[type="checkbox"]').length;
 // tool(alltool,includeEnabledandNot enabled's )
    const totalTools = roleToolsPagination.total || 0;
    
    statsEl.innerHTML = `
        <span title="${_t('roleModal.currentPageSelectedTitle')}">✅ ${_t('roleModal.currentPageSelected', { current: currentPageEnabled, total: currentPageTotal })}</span>
        <span title="${_t('roleModal.totalSelectedTitle')}">📊 ${_t('roleModal.totalSelected', { current: totalSelected, total: totalTools })}</span>
    `;
}

// getselected's toollist(returntoolKeyarray)
async function getSelectedRoleTools() {
    // firstSavecurrentpage's status
    saveCurrentRolePageToolStates();
    
 // ifhassearchkeyword,needloadallpage's toolensurestatusmappingcomplete
 // butin order tocan,s cantoonlyfromstatusmappingingetalreadyselected's tool
 // is:ifuseonlyatsomethese pageselecttool,otherpage's tool statepossiblynotatmappingin
    
 // iftoolgreater thanalreadyload's tool,s needensureallnot loadedpage's toolalsoby
 // butforroletool selection,s onlyneedgetuseselectthrough's tool
    // sodirectlyfromstatusmappinggetalreadyselected's toolsuffice
    
    // fromstatusmappinggetallselected's tool(onlyreturnatMCPtools enabled in management)
    const selectedTools = [];
    roleToolStateMap.forEach((state, toolKey) => {
        // onlyreturnatMCPenabled in managementandbyroleselected's tool
        if (state.enabled && state.mcpEnabled !== false) {
            selectedTools.push(toolKey);
        }
    });
    
 // ifusepossiblyatotherpageselecttool,s needensurecurrentpage's statusalsobySave
 // butstatusmappingshouldthisalreadycontainallthrough's page's status
    
    return selectedTools;
}

// settingsselected's tool(used forEditrolewhen)
function setSelectedRoleTools(selectedToolKeys) {
    const selectedSet = new Set(selectedToolKeys || []);
    
    // updatestatusmapping
    roleToolStateMap.forEach((state, toolKey) => {
        state.enabled = selectedSet.has(toolKey);
    });
    
    // updatecurrentpage's checkboxstatus
    document.querySelectorAll('#role-tools-list .role-tool-item').forEach(item => {
        const toolKey = item.dataset.toolKey;
        const checkbox = item.querySelector('input[type="checkbox"]');
        if (toolKey && checkbox) {
            checkbox.checked = selectedSet.has(toolKey);
        }
    });
    
    updateRoleToolsStats();
}

// Show addrolemodal
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

    // addrolewhen:Show tool selectioninterface,hidedefault rolehint
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

    // resettool state
    roleToolStateMap.clear();
    roleConfiguredTools.clear(); // clearrole configuration's toollist
    roleUsesAllTools = false; // addrolewhendefaultnotusealltool
    roleToolsSearchKeyword = '';
    const searchInput = document.getElementById('role-tools-search');
    if (searchInput) {
        searchInput.value = '';
    }
    const clearBtn = document.getElementById('role-tools-search-clear');
    if (clearBtn) {
        clearBtn.style.display = 'none';
    }
    
    // cleartoollist DOM,avoid loadRoleTools in's  saveCurrentRolePageToolStates readoldstatus
    if (toolsList) {
        toolsList.innerHTML = '';
    }

    // resetskillsstatus
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

 // loadrendertoollist
    await loadRoleTools(1, '');
    
    // ensuretoollistShow 
    if (toolsList) {
        toolsList.style.display = 'block';
    }
    
    // ensurestatisticsInfocorrectupdate(Show 0/108)
    updateRoleToolsStats();

 // loadrenderskillslist
    await loadRoleSkills();

    modal.style.display = 'flex';
}

// Editrole
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
    document.getElementById('role-name').disabled = true; // Editwhennotallowmodifyname
    document.getElementById('role-description').value = role.description || '';
    // processiconfield:if it isUnicodeescape format,convert toemoji;otherwiseuse directly
    let iconValue = role.icon || '';
    if (iconValue && iconValue.startsWith('\\U')) {
        // convertUnicodeescape format(like \U0001F3C6)asemoji
        try {
            const codePoint = parseInt(iconValue.substring(2), 16);
            iconValue = String.fromCodePoint(codePoint);
        } catch (e) {
 // if conversion fails,use
        }
    }
    document.getElementById('role-icon').value = iconValue;
    document.getElementById('role-user-prompt').value = role.user_prompt || '';
    document.getElementById('role-enabled').checked = role.enabled !== false;

 // checkisasdefault role
    const isDefaultRole = roleName === 'default';
    const toolsSection = document.getElementById('role-tools-section');
    const defaultHint = document.getElementById('role-tools-default-hint');
    const toolsControls = document.querySelector('.role-tools-controls');
    const toolsList = document.getElementById('role-tools-list');
    const formHint = toolsSection ? toolsSection.querySelector('.form-hint') : null;
    
    if (isDefaultRole) {
        // default role:hidetool selectioninterface,Show hintInfo
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
 // default role:Show tool selectioninterface,hidehintInfo
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

        // resettool state
        roleToolStateMap.clear();
        roleConfiguredTools.clear(); // clearrole configuration's toollist
        roleToolsSearchKeyword = '';
        const searchInput = document.getElementById('role-tools-search');
        if (searchInput) {
            searchInput.value = '';
        }
        const clearBtn = document.getElementById('role-tools-search-clear');
        if (clearBtn) {
            clearBtn.style.display = 'none';
        }

 // preferusetoolsfield,ifhasthenusemcpsfield(backward compatible)
        const selectedTools = role.tools || (role.mcps && role.mcps.length > 0 ? role.mcps : []);
        
 // determineisusealltool:ifhasconfigurationtools(ortoolsasemptyarray),indicatesusealltool
        roleUsesAllTools = !role.tools || role.tools.length === 0;
        
        // Saverole configuration's toollist
        if (selectedTools.length > 0) {
            selectedTools.forEach(toolKey => {
                roleConfiguredTools.add(toolKey);
            });
        }
        
        // ifhasselected's tool,firstinitializestatusmapping
        if (selectedTools.length > 0) {
            roleUsesAllTools = false; // hasconfigurationtool,notusealltool
            // willselected's tooladdtostatusmapping(markasselected)
            selectedTools.forEach(toolKey => {
 // ifmappinginstillhasthis tools,firstcreate adefaultstatus(enabledastrue)
                if (!roleToolStateMap.has(toolKey)) {
                    roleToolStateMap.set(toolKey, {
                        enabled: true,
                        is_external: false,
                        external_mcp: '',
                        name: toolKey.split('::').pop() || toolKey // fromtoolKeyinextracttool name
                    });
                } else {
                    // ifalready exists,updateasselectedstatus
                    const state = roleToolStateMap.get(toolKey);
                    state.enabled = true;
                }
            });
        }

 // loadtoollist(page)
        await loadRoleTools(1, '');
        
        // if using all tools,markcurrentpageallEnabled's toolasselected
        if (roleUsesAllTools) {
            // markcurrentpageallatMCPtools enabled in managementasselected
            document.querySelectorAll('#role-tools-list input[type="checkbox"]').forEach(checkbox => {
                const toolItem = checkbox.closest('.role-tool-item');
                if (toolItem) {
                    const toolKey = toolItem.dataset.toolKey;
                    const toolName = toolItem.dataset.toolName;
                    const isExternal = toolItem.dataset.isExternal === 'true';
                    const externalMcp = toolItem.dataset.externalMcp || '';
                    if (toolKey) {
                        const state = roleToolStateMap.get(toolKey);
                        // onlyselectedatMCPtools enabled in management
 // ifstatusexist,usestatusin's mcpEnabled;otherwiseEnabled(because loadRoleTools shouldthisalreadyinitializealltool)
                        const shouldEnable = state ? (state.mcpEnabled !== false) : true;
                        checkbox.checked = shouldEnable;
                        if (state) {
                            state.enabled = shouldEnable;
                        } else {
 // ifstatusnotexist,createnewstatus(this typecasenotshouldthis,because loadRoleTools shouldthisalreadyinitialize)
                            roleToolStateMap.set(toolKey, {
                                enabled: shouldEnable,
                                is_external: isExternal,
                                external_mcp: externalMcp,
                                name: toolName,
 mcpEnabled: true // Enabled,actualwillatloadRoleToolsinupdate
                            });
                        }
                    }
                }
            });
            // update statistics,ensureShow correct's selectedcount
            updateRoleToolsStats();
        } else if (selectedTools.length > 0) {
            // loadcompleteafter,thentimesettingsselectedstatus(ensurecurrentpage's toolalsobycorrectsettings)
            setSelectedRoleTools(selectedTools);
        }
    }

 // loadsettingsskills
    await loadRoleSkills();
    // settingsrole configuration's skills
    const selectedSkills = role.skills || [];
    roleSelectedSkills.clear();
    selectedSkills.forEach(skill => {
        roleSelectedSkills.add(skill);
    });
    renderRoleSkills();

    modal.style.display = 'flex';
}

// Closerolemodal
function closeRoleModal() {
    const modal = document.getElementById('role-modal');
    if (modal) {
        modal.style.display = 'none';
    }
}

// getallselected's tool(includenot yetatMCPtools enabled in management)
function getAllSelectedRoleTools() {
    // firstSavecurrentpage's status
    saveCurrentRolePageToolStates();
    
 // fromstatusmappinggetallselected's tool(notisatMCPenabled in management)
    const selectedTools = [];
    roleToolStateMap.forEach((state, toolKey) => {
        if (state.enabled) {
            selectedTools.push({
                key: toolKey,
                name: state.name || toolKey.split('::').pop() || toolKey,
                mcpEnabled: state.mcpEnabled !== false // mcpEnabled as false whenisNot enabled,othercasetreated asEnabled
            });
        }
    });
    
    return selectedTools;
}

// checkgetnot yetatMCPtools enabled in management
function getDisabledTools(selectedTools) {
    return selectedTools.filter(tool => {
        const state = roleToolStateMap.get(tool.key);
 // if mcpEnabled as false,thenasisNot enabled
        return state && state.mcpEnabled === false;
    });
}

// loadalltooltostatusmappingin(used forfromuseAlltoolswitchtominutetoolwhen)
async function loadAllToolsToStateMap() {
    try {
 const pageSize = 100; // usebig's pageSizetoreducefewrequesttime
        let page = 1;
        let hasMore = true;
        
        // iterate all pages to get all tools
        while (hasMore) {
            const url = `/api/config/tools?page=${page}&page_size=${pageSize}`;
            const response = await apiFetch(url);
            if (!response.ok) {
                throw new Error('Failed to get tool list');
            }
            
            const result = await response.json();
            
            // willalltooladdtostatusmappingin
            result.tools.forEach(tool => {
                const toolKey = getToolKey(tool);
                if (!roleToolStateMap.has(toolKey)) {
                    // toolnotatmappingin,based oncurrentmodeinitialize
                    let enabled = false;
                    if (roleUsesAllTools) {
                        // if using all tools,and tool is inMCPenabled in management,mark as selected
                        enabled = tool.enabled ? true : false;
                    } else {
                        // if not using all tools,onlyhastoolatrole configuration's toollistinonly markasselected
                        enabled = roleConfiguredTools.has(toolKey);
                    }
                    roleToolStateMap.set(toolKey, {
                        enabled: enabled,
                        is_external: tool.is_external || false,
                        external_mcp: tool.external_mcp || '',
                        name: tool.name,
                        mcpEnabled: tool.enabled // SaveMCPoriginal enabled state in management
                    });
                } else {
                    // toolalreadyatmappingin,updateotherpropertybutpreserveenabledstatus
                    const state = roleToolStateMap.get(toolKey);
                    state.is_external = tool.is_external || false;
                    state.external_mcp = tool.external_mcp || '';
                    state.mcpEnabled = tool.enabled; // updateMCPoriginal enabled state in management
                    if (!state.name || state.name === toolKey.split('::').pop()) {
                        state.name = tool.name; // updatetool name
                    }
                }
            });
            
            // check if there are more pages
            if (page >= result.total_pages) {
                hasMore = false;
            } else {
                page++;
            }
        }
    } catch (error) {
        console.error('Failed to load all tools to state mapping:', error);
        throw error;
    }
}

// Saverole
async function saveRole() {
    const name = document.getElementById('role-name').value.trim();
    if (!name) {
        showNotification(_t('roleModal.roleNameRequired'), 'error');
        return;
    }

    const description = document.getElementById('role-description').value.trim();
    let icon = document.getElementById('role-icon').value.trim();
    // willemojiconvert toUnicodeescape formattomatchYAMLformat(like \U0001F3C6)
    if (icon) {
 // getcharacter's Unicodecode(processemojipossiblyismanycharacter's case)
        const codePoint = icon.codePointAt(0);
        if (codePoint && codePoint > 0x7F) {
 // convert to8enterformat(\U0001F3C6)
            icon = '\\U' + codePoint.toString(16).toUpperCase().padStart(8, '0');
        }
    }
    const userPrompt = document.getElementById('role-user-prompt').value.trim();
    const enabled = document.getElementById('role-enabled').checked;

    const isEdit = document.getElementById('role-name').disabled;
    
 // checkisasdefault role
    const isDefaultRole = name === 'default';
    
 // check if it isfirst timeaddrole(excludedefault roleafter,hasusecreate's role)
    const isFirstUserRole = !isEdit && !isDefaultRole && roles.filter(r => r.name !== 'default').length === 0;
    
    // default rolenotSavetoolsfield(usealltool)
 // default role:if using all tools(roleUsesAllToolsastrue),alsonotSavetoolsfield
    let tools = [];
    let disabledTools = []; // storagenot yetatMCPtools enabled in management
    
    if (!isDefaultRole) {
        // Savecurrentpage's status
        saveCurrentRolePageToolStates();
        
        // collectallselected's tool(includenot yetatMCPenabled in management's )
        let allSelectedTools = getAllSelectedRoleTools();
        
 // if it isfirst timeaddroleandhasselecttool,defaultuseAlltool
        if (isFirstUserRole && allSelectedTools.length === 0) {
            roleUsesAllTools = true;
            showNotification(_t('roleModal.firstRoleNoToolsHint'), 'info');
        } else if (roleUsesAllTools) {
 // ifcurrentusealltool,needcheckuseisCancelthese tool
 // checkstatusmappinginishasnot yetselected's Enabledtool
            let hasUnselectedTools = false;
            roleToolStateMap.forEach((state) => {
 // if tool is inMCPenabled in managementbutnot yetselected,DescriptionuseCancelthistool
                if (state.mcpEnabled !== false && !state.enabled) {
                    hasUnselectedTools = true;
                }
            });
            
 // ifuseCancelthese Enabled's tool,switchtominutetoolmode
            if (hasUnselectedTools) {
 // atswitchbefore,needloadalltooltostatusmappingin
 // this s cantocorrectSavealltool's status(useCancel's that these )
                await loadAllToolsToStateMap();
                
 // willallEnabled's toolmarkasselected(usealreadyCancel's that these )
 // usealreadyCancel's toolatstatusmappinginenabledasfalse,maintainnot
                roleToolStateMap.forEach((state, toolKey) => {
 // if tool is inMCPenabled in management,andstatusmappinginhasmarkasnot yetselected(i.e.enablednotisfalse)
                    // mark as selected
                    if (state.mcpEnabled !== false && state.enabled !== false) {
                        state.enabled = true;
                    }
                });
                
                roleUsesAllTools = false;
            } else {
 // even ifusealltool,alsoneedloadalltooltostatusmappingin,so thatcheckishasNot enabled's toolbyselected
 // this cantouseismanualselectthese Not enabled's tool
                await loadAllToolsToStateMap();
                
 // checkishasNot enabled's toolbymanualselected(enabledastruebutmcpEnabledasfalse)
                let hasDisabledToolsSelected = false;
                roleToolStateMap.forEach((state) => {
                    if (state.enabled && state.mcpEnabled === false) {
                        hasDisabledToolsSelected = true;
                    }
                });
                
 // ifhasNot enabled's toolbyselected,willallEnabled's toolmarkasselected(this isusealltool's defaultlineas)
                if (!hasDisabledToolsSelected) {
                    roleToolStateMap.forEach((state) => {
                        if (state.mcpEnabled !== false) {
                            state.enabled = true;
                        }
                    });
                }
                
 // update allSelectedTools,becauseatstatusmappingincontainalltool
                allSelectedTools = getAllSelectedRoleTools();
            }
        }
        
 // checkthese toolnot yetatMCPenabled in management(noisusealltoolall needcheck)
        disabledTools = getDisabledTools(allSelectedTools);
        
 // ifhasNot enabled's tool,hintuse
        if (disabledTools.length > 0) {
            const toolNames = disabledTools.map(t => t.name).join(',');
 const message = `below ${disabledTools.length} toolsnot yetatMCPenabled in management,cannotatroleinconfiguration:\n\n${toolNames}\n\npleasefirstat"MCPmanagement"inenablethis these tool,afterthenatroleinconfiguration.\n\nisContinueSave?(willonlySaveEnabled's tool)`;
            
            if (!confirm(message)) {
 return; // usecancel save
            }
        }
        
        // if using all tools,notneedgettoollist
        if (!roleUsesAllTools) {
            // getselected's toollist(onlycontainatMCPtools enabled in management)
            tools = await getSelectedRoleTools();
        }
    }

    // getselected's skills
    const skills = Array.from(roleSelectedSkills);

    const roleData = {
        name: name,
        description: description,
        icon: icon || undefined, // ifasemptystring,thennotsendthisfield
        user_prompt: userPrompt,
        tools: tools, // default roleasemptyarray,indicatesusealltool
        skills: skills, // Skillslist
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
            throw new Error(error.error || 'Failed to save role');
        }

 // ifhasNot enabled's toolbyfilter,hintuse
        if (disabledTools.length > 0) {
            let toolNames = disabledTools.map(t => t.name).join(',');
 // iftool namelistlong,truncateShow 
            if (toolNames.length > 100) {
                toolNames = toolNames.substring(0, 100) + '...';
            }
            showNotification(
 `${isEdit ? 'Role updated' : 'Role created'},butalreadyfilter ${disabledTools.length} not yetatMCPtools enabled in management:${toolNames}.pleasefirstat"MCPmanagement"inenablethis these tool,afterthenatroleinconfiguration.`,
                'warning'
            );
        } else {
            showNotification(isEdit ? 'Role updated' : 'Role created', 'success');
        }
        
        closeRoleModal();
        await refreshRoles();
    } catch (error) {
        console.error('Failed to save role:', error);
        showNotification('Failed to save role: ' + error.message, 'error');
    }
}

// Deleterole
async function deleteRole(roleName) {
    if (roleName === 'default') {
        showNotification(_t('roleModal.cannotDeleteDefaultRole'), 'error');
        return;
    }

 if (!confirm(`OKneedDeleterole"${roleName}"?thisActionsnot .`)) {
        return;
    }

    try {
        const response = await apiFetch(`/api/roles/${encodeURIComponent(roleName)}`, {
            method: 'DELETE'
        });

        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || 'Failed to delete role');
        }

        showNotification('Role deleted', 'success');
        
        // ifDelete's iscurrentselected's role,switchtodefault role
        if (currentRole === roleName) {
            handleRoleChange('');
        }

        await refreshRoles();
    } catch (error) {
        console.error('Failed to delete role:', error);
        showNotification('Failed to delete role: ' + error.message, 'error');
    }
}

// atpageswitchwheninitializerolelist
if (typeof switchPage === 'function') {
    const originalSwitchPage = switchPage;
    switchPage = function(page) {
        originalSwitchPage(page);
        if (page === 'roles-management') {
            loadRoles().then(() => renderRolesList());
        }
    };
}

// click outside modal to close
document.addEventListener('click', (e) => {
    const roleSelectModal = document.getElementById('role-select-modal');
    if (roleSelectModal && e.target === roleSelectModal) {
        closeRoleSelectModal();
    }

    const roleModal = document.getElementById('role-modal');
    if (roleModal && e.target === roleModal) {
        closeRoleModal();
    }

    // clickrole selectionpanelExternalClosepanel(butnotincluderole selectionbuttonandpanelitself)
    const roleSelectionPanel = document.getElementById('role-selection-panel');
    const roleSelectorWrapper = document.querySelector('.role-selector-wrapper');
    if (roleSelectionPanel && roleSelectionPanel.style.display !== 'none' && roleSelectionPanel.style.display) {
 // checkclickisatpanelorwrapon
        if (!roleSelectorWrapper?.contains(e.target)) {
            closeRoleSelectionPanel();
        }
    }
});

// pageloadwheninitialize
document.addEventListener('DOMContentLoaded', () => {
    loadRoles();
    updateRoleSelectorDisplay();
});

// languageswitchafterRefreshrole selectionShow (default/customrole)
document.addEventListener('languagechange', () => {
    updateRoleSelectorDisplay();
});

// getcurrentselected's role(forchat.jsuse)
function getCurrentRole() {
    return currentRole || '';
}

// exposefunctiontoglobaluse
if (typeof window !== 'undefined') {
    window.getCurrentRole = getCurrentRole;
    window.toggleRoleSelectionPanel = toggleRoleSelectionPanel;
    window.closeRoleSelectionPanel = closeRoleSelectionPanel;
    window.currentSelectedRole = getCurrentRole();
    
    // listenrolechange,updateglobalvariable
    const originalHandleRoleChange = handleRoleChange;
    handleRoleChange = function(roleName) {
        originalHandleRoleChange(roleName);
        if (typeof window !== 'undefined') {
            window.currentSelectedRole = getCurrentRole();
        }
    };
}

// ==================== Skillsrelatedfunction ====================

// loadskillslist
async function loadRoleSkills() {
    try {
        const response = await apiFetch('/api/roles/skills/list');
        if (!response.ok) {
            throw new Error('loadskillslist failed');
        }
        const data = await response.json();
        allRoleSkills = data.skills || [];
        renderRoleSkills();
    } catch (error) {
        console.error('loadskillslist failed:', error);
        allRoleSkills = [];
        const skillsList = document.getElementById('role-skills-list');
        if (skillsList) {
            skillsList.innerHTML = '<div class="skills-error">' + _t('roleModal.loadSkillsFailed') + ': ' + error.message + '</div>';
        }
    }
}

// renderskillslist
function renderRoleSkills() {
    const skillsList = document.getElementById('role-skills-list');
    if (!skillsList) return;

    // filterskills
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

    // renderskillslist
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

// switchskillselectedstatus
function toggleRoleSkill(skill, checked) {
    if (checked) {
        roleSelectedSkills.add(skill);
    } else {
        roleSelectedSkills.delete(skill);
    }
    updateRoleSkillsStats();
}

// select allskills
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

// notskills
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

// searchskills
function searchRoleSkills(keyword) {
    roleSkillsSearchKeyword = keyword;
    const clearBtn = document.getElementById('role-skills-search-clear');
    if (clearBtn) {
        clearBtn.style.display = keyword ? 'block' : 'none';
    }
    renderRoleSkills();
}

// clearskillssearch
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

// updateskillsstatisticsInfo
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

// HTMLescape function
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}
