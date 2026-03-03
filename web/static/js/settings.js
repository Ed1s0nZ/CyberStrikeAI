// Settings-related functionality
let currentConfig = null;
let allTools = [];
// Global tool status map, used to save user changes across all pages
// key: unique tool identifier (toolKey), value: { enabled: boolean, is_external: boolean, external_mcp: string }
let toolStateMap = new Map();

// Generate a unique identifier for a tool to distinguish tools with the same name but different sources
function getToolKey(tool) {
    // If it's an external tool, use external_mcp::tool.name as the unique identifier
    // If it's a built-in tool, use tool.name as the identifier
    if (tool.is_external && tool.external_mcp) {
        return `${tool.external_mcp}::${tool.name}`;
    }
    return tool.name;
}
// Read per-page count from localStorage, default is 20
const getToolsPageSize = () => {
    const saved = localStorage.getItem('toolsPageSize');
    return saved ? parseInt(saved, 10) : 20;
};

let toolsPagination = {
    page: 1,
    pageSize: getToolsPageSize(),
    total: 0,
    totalPages: 0
};

// Toggle settings category
function switchSettingsSection(section) {
    // Update nav item status
    document.querySelectorAll('.settings-nav-item').forEach(item => {
        item.classList.remove('active');
    });
    const activeNavItem = document.querySelector(`.settings-nav-item[data-section="${section}"]`);
    if (activeNavItem) {
        activeNavItem.classList.add('active');
    }
    
    // Update content area display
    document.querySelectorAll('.settings-section-content').forEach(content => {
        content.classList.remove('active');
    });
    const activeContent = document.getElementById(`settings-section-${section}`);
    if (activeContent) {
        activeContent.classList.add('active');
    }
    if (section === 'terminal' && typeof initTerminal === 'function') {
        setTimeout(initTerminal, 0);
    }
}

// Open settings
async function openSettings() {
    // Switch to settings page
    if (typeof switchPage === 'function') {
        switchPage('settings');
    }
    
    // Clear global status map on each open, reload latest config
    toolStateMap.clear();
    
    // Reload latest config on each open (system settings page does not need tool list)
    await loadConfig(false);
    
    // Clear previous validation error status
    document.querySelectorAll('.form-group input').forEach(input => {
        input.classList.remove('error');
    });
    
    // Default display basic settings
    switchSettingsSection('basic');
}

// Close settings (keep function for backward compat, no close functionality needed now)
function closeSettings() {
    // Close is no longer needed since this is now a page, not a modal
    // If needed, can switch back to conversations page
    if (typeof switchPage === 'function') {
        switchPage('chat');
    }
}

// Click outside modal to close (only MCP details modal)
window.onclick = function(event) {
    const mcpModal = document.getElementById('mcp-detail-modal');
    
    if (event.target === mcpModal) {
        closeMCPDetail();
    }
}

// Load config
async function loadConfig(loadTools = true) {
    try {
        const response = await apiFetch('/api/config');
        if (!response.ok) {
            throw new Error('Failed to get config');
        }
        
        currentConfig = await response.json();
        
        // Fill OpenAI config
        document.getElementById('openai-api-key').value = currentConfig.openai.api_key || '';
        document.getElementById('openai-base-url').value = currentConfig.openai.base_url || '';
        document.getElementById('openai-model').value = currentConfig.openai.model || '';

        // Fill FOFA config
        const fofa = currentConfig.fofa || {};
        const fofaEmailEl = document.getElementById('fofa-email');
        const fofaKeyEl = document.getElementById('fofa-api-key');
        const fofaBaseUrlEl = document.getElementById('fofa-base-url');
        if (fofaEmailEl) fofaEmailEl.value = fofa.email || '';
        if (fofaKeyEl) fofaKeyEl.value = fofa.api_key || '';
        if (fofaBaseUrlEl) fofaBaseUrlEl.value = fofa.base_url || '';
        
        // Fill Agent config
        document.getElementById('agent-max-iterations').value = currentConfig.agent.max_iterations || 30;
        
        // Fill knowledge base config
        const knowledgeEnabledCheckbox = document.getElementById('knowledge-enabled');
        if (knowledgeEnabledCheckbox) {
            knowledgeEnabledCheckbox.checked = currentConfig.knowledge?.enabled !== false;
        }
        
        // Fill knowledge base detailed config
        if (currentConfig.knowledge) {
            const knowledge = currentConfig.knowledge;
            
            // Basic config
            const basePathInput = document.getElementById('knowledge-base-path');
            if (basePathInput) {
                basePathInput.value = knowledge.base_path || 'knowledge_base';
            }
            
            // Embedding model config
            const embeddingProviderSelect = document.getElementById('knowledge-embedding-provider');
            if (embeddingProviderSelect) {
                embeddingProviderSelect.value = knowledge.embedding?.provider || 'openai';
            }
            
            const embeddingModelInput = document.getElementById('knowledge-embedding-model');
            if (embeddingModelInput) {
                embeddingModelInput.value = knowledge.embedding?.model || '';
            }
            
            const embeddingBaseUrlInput = document.getElementById('knowledge-embedding-base-url');
            if (embeddingBaseUrlInput) {
                embeddingBaseUrlInput.value = knowledge.embedding?.base_url || '';
            }
            
            const embeddingApiKeyInput = document.getElementById('knowledge-embedding-api-key');
            if (embeddingApiKeyInput) {
                embeddingApiKeyInput.value = knowledge.embedding?.api_key || '';
            }
            
            // Retrieval config
            const retrievalTopKInput = document.getElementById('knowledge-retrieval-top-k');
            if (retrievalTopKInput) {
                retrievalTopKInput.value = knowledge.retrieval?.top_k || 5;
            }
            
            const retrievalThresholdInput = document.getElementById('knowledge-retrieval-similarity-threshold');
            if (retrievalThresholdInput) {
                retrievalThresholdInput.value = knowledge.retrieval?.similarity_threshold || 0.7;
            }
            
            const retrievalWeightInput = document.getElementById('knowledge-retrieval-hybrid-weight');
            if (retrievalWeightInput) {
                const hybridWeight = knowledge.retrieval?.hybrid_weight;
                // Allow 0.0 value, only use default when undefined/null
                retrievalWeightInput.value = (hybridWeight !== undefined && hybridWeight !== null) ? hybridWeight : 0.7;
            }
        }

        // Fill bot config
        const robots = currentConfig.robots || {};
        const wecom = robots.wecom || {};
        const dingtalk = robots.dingtalk || {};
        const lark = robots.lark || {};
        const telegram = robots.telegram || {};
        const wecomEnabled = document.getElementById('robot-wecom-enabled');
        if (wecomEnabled) wecomEnabled.checked = wecom.enabled === true;
        const wecomToken = document.getElementById('robot-wecom-token');
        if (wecomToken) wecomToken.value = wecom.token || '';
        const wecomAes = document.getElementById('robot-wecom-encoding-aes-key');
        if (wecomAes) wecomAes.value = wecom.encoding_aes_key || '';
        const wecomCorp = document.getElementById('robot-wecom-corp-id');
        if (wecomCorp) wecomCorp.value = wecom.corp_id || '';
        const wecomSecret = document.getElementById('robot-wecom-secret');
        if (wecomSecret) wecomSecret.value = wecom.secret || '';
        const wecomAgentId = document.getElementById('robot-wecom-agent-id');
        if (wecomAgentId) wecomAgentId.value = wecom.agent_id || '0';
        const dingtalkEnabled = document.getElementById('robot-dingtalk-enabled');
        if (dingtalkEnabled) dingtalkEnabled.checked = dingtalk.enabled === true;
        const dingtalkClientId = document.getElementById('robot-dingtalk-client-id');
        if (dingtalkClientId) dingtalkClientId.value = dingtalk.client_id || '';
        const dingtalkClientSecret = document.getElementById('robot-dingtalk-client-secret');
        if (dingtalkClientSecret) dingtalkClientSecret.value = dingtalk.client_secret || '';
        const larkEnabled = document.getElementById('robot-lark-enabled');
        if (larkEnabled) larkEnabled.checked = lark.enabled === true;
        const larkAppId = document.getElementById('robot-lark-app-id');
        if (larkAppId) larkAppId.value = lark.app_id || '';
        const larkAppSecret = document.getElementById('robot-lark-app-secret');
        if (larkAppSecret) larkAppSecret.value = lark.app_secret || '';
        const larkVerify = document.getElementById('robot-lark-verify-token');
        if (larkVerify) larkVerify.value = lark.verify_token || '';
        const telegramEnabled = document.getElementById('robot-telegram-enabled');
        if (telegramEnabled) telegramEnabled.checked = telegram.enabled === true;
        const telegramBotToken = document.getElementById('robot-telegram-bot-token');
        if (telegramBotToken) telegramBotToken.value = telegram.bot_token || '';
        const telegramAllowedIds = document.getElementById('robot-telegram-allowed-user-ids');
        if (telegramAllowedIds) telegramAllowedIds.value = (telegram.allowed_user_ids || []).join(',');

        // Only load tool list when needed (MCP management page needs it, system settings does not)
        if (loadTools) {
            // Set per-page count (will be set when pagination controls render)
            const savedPageSize = getToolsPageSize();
            toolsPagination.pageSize = savedPageSize;
            
            // Load tool list (with pagination)
            toolsSearchKeyword = '';
            await loadToolsList(1, '');
        }
    } catch (error) {
        console.error('Failed to load config:', error);
        alert('Failed to load config: ' + error.message);
    }
}

// Tool search keyword
let toolsSearchKeyword = '';

// Load tool list (paginated)
async function loadToolsList(page = 1, searchKeyword = '') {
    const toolsList = document.getElementById('tools-list');
    
    // Show loading status
    if (toolsList) {
        // Clear entire container, including any existing pagination controls
        toolsList.innerHTML = '<div class="tools-list-items"><div class="loading" style="padding: 20px; text-align: center; color: var(--text-muted);">⏳ Loading tool list...</div></div>';
    }
    
    try {
        // Before loading new page, save current page status to global map
        saveCurrentPageToolStates();
        
        const pageSize = toolsPagination.pageSize;
        let url = `/api/config/tools?page=${page}&page_size=${pageSize}`;
        if (searchKeyword) {
            url += `&search=${encodeURIComponent(searchKeyword)}`;
        }
        
        // Use shorter timeout (10 seconds) to avoid long waits
        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), 10000);
        
        const response = await apiFetch(url, {
            signal: controller.signal
        });
        clearTimeout(timeoutId);
        
        if (!response.ok) {
            throw new Error('Failed to get tool list');
        }
        
        const result = await response.json();
        allTools = result.tools || [];
        toolsPagination = {
            page: result.page || page,
            pageSize: result.page_size || pageSize,
            total: result.total || 0,
            totalPages: result.total_pages || 1
        };
        
        // Initialize tool status map (use server-returned status if tool not in map)
        allTools.forEach(tool => {
            const toolKey = getToolKey(tool);
            if (!toolStateMap.has(toolKey)) {
                toolStateMap.set(toolKey, {
                    enabled: tool.enabled,
                    is_external: tool.is_external || false,
                    external_mcp: tool.external_mcp || '',
                    name: tool.name // Save original tool name
                });
            }
        });
        
        renderToolsList();
        renderToolsPagination();
    } catch (error) {
        console.error('Failed to load tool list:', error);
        if (toolsList) {
            const isTimeout = error.name === 'AbortError' || error.message.includes('timeout');
            const errorMsg = isTimeout 
                ? 'Tool list load timed out; external MCP connection may be slow. Click "Refresh" to retry, or check external MCP connection status.'
                : `Failed to load tool list: ${escapeHtml(error.message)}`;
            toolsList.innerHTML = `<div class="error" style="padding: 20px; text-align: center;">${errorMsg}</div>`;
        }
    }
}

// Save current page tool status to global map
function saveCurrentPageToolStates() {
    document.querySelectorAll('#tools-list .tool-item').forEach(item => {
        const checkbox = item.querySelector('input[type="checkbox"]');
        const toolKey = item.dataset.toolKey; // Use unique identifier
        const toolName = item.dataset.toolName;
        const isExternal = item.dataset.isExternal === 'true';
        const externalMcp = item.dataset.externalMcp || '';
        if (toolKey && checkbox) {
            toolStateMap.set(toolKey, {
                enabled: checkbox.checked,
                is_external: isExternal,
                external_mcp: externalMcp,
                name: toolName // Save original tool name
            });
        }
    });
}

// Search tools
function searchTools() {
    const searchInput = document.getElementById('tools-search');
    const keyword = searchInput ? searchInput.value.trim() : '';
    toolsSearchKeyword = keyword;
    // Reset to first page when searching
    loadToolsList(1, keyword);
}

// Clear search
function clearSearch() {
    const searchInput = document.getElementById('tools-search');
    if (searchInput) {
        searchInput.value = '';
    }
    toolsSearchKeyword = '';
    loadToolsList(1, '');
}

// Handle search box enter key event
function handleSearchKeyPress(event) {
    if (event.key === 'Enter') {
        searchTools();
    }
}

// Render tool list
function renderToolsList() {
    const toolsList = document.getElementById('tools-list');
    if (!toolsList) return;
    
    // Remove any existing pagination controls (will be re-added in renderToolsPagination)
    const oldPagination = toolsList.querySelector('.tools-pagination');
    if (oldPagination) {
        oldPagination.remove();
    }
    
    // Get or create list container
    let listContainer = toolsList.querySelector('.tools-list-items');
    if (!listContainer) {
        listContainer = document.createElement('div');
        listContainer.className = 'tools-list-items';
        toolsList.appendChild(listContainer);
    }
    
    // Clear list container content (remove loading prompt)
    listContainer.innerHTML = '';
    
    if (allTools.length === 0) {
        listContainer.innerHTML = '<div class="empty">No tools</div>';
        if (!toolsList.contains(listContainer)) {
            toolsList.appendChild(listContainer);
        }
        // Update statistics
        updateToolsStats();
        return;
    }
    
    allTools.forEach(tool => {
        const toolKey = getToolKey(tool); // Generate unique identifier
        const toolItem = document.createElement('div');
        toolItem.className = 'tool-item';
        toolItem.dataset.toolKey = toolKey; // Save unique identifier
        toolItem.dataset.toolName = tool.name; // Save original tool name
        toolItem.dataset.isExternal = tool.is_external ? 'true' : 'false';
        toolItem.dataset.externalMcp = tool.external_mcp || '';
        
        // Get tool status from global map, use server-returned status if not in map
        const toolState = toolStateMap.get(toolKey) || {
            enabled: tool.enabled,
            is_external: tool.is_external || false,
            external_mcp: tool.external_mcp || ''
        };
        
        // External tool badge, show source info
        let externalBadge = '';
        if (toolState.is_external || tool.is_external) {
            const externalMcpName = toolState.external_mcp || tool.external_mcp || '';
            const badgeText = externalMcpName ? `External (${escapeHtml(externalMcpName)})` : 'External';
            const badgeTitle = externalMcpName ? `External MCP Tool - Source: ${escapeHtml(externalMcpName)}` : 'External MCP Tool';
            externalBadge = `<span class="external-tool-badge" title="${badgeTitle}">${badgeText}</span>`;
        }
        
        // Generate unique checkbox id using tool unique identifier
        const checkboxId = `tool-${escapeHtml(toolKey).replace(/::/g, '--')}`;
        
        toolItem.innerHTML = `
            <input type="checkbox" id="${checkboxId}" ${toolState.enabled ? 'checked' : ''} ${toolState.is_external || tool.is_external ? 'data-external="true"' : ''} onchange="handleToolCheckboxChange('${escapeHtml(toolKey)}', this.checked)" />
            <div class="tool-item-info">
                <div class="tool-item-name">
                    ${escapeHtml(tool.name)}
                    ${externalBadge}
                </div>
                <div class="tool-item-desc">${escapeHtml(tool.description || 'NoneDescription')}</div>
            </div>
        `;
        listContainer.appendChild(toolItem);
    });
    
    if (!toolsList.contains(listContainer)) {
        toolsList.appendChild(listContainer);
    }
    
    // Update statistics
    updateToolsStats();
}

// Render tool list pagination controls
function renderToolsPagination() {
    const toolsList = document.getElementById('tools-list');
    if (!toolsList) return;
    
    // Remove old pagination controls
    const oldPagination = toolsList.querySelector('.tools-pagination');
    if (oldPagination) {
        oldPagination.remove();
    }
    
    // If only one page or no data, do not show pagination
    if (toolsPagination.totalPages <= 1) {
        return;
    }
    
    const pagination = document.createElement('div');
    pagination.className = 'tools-pagination';
    
    const { page, totalPages, total } = toolsPagination;
    const startItem = (page - 1) * toolsPagination.pageSize + 1;
    const endItem = Math.min(page * toolsPagination.pageSize, total);
    
    const savedPageSize = getToolsPageSize();
    pagination.innerHTML = `
        <div class="pagination-info">
            Showing ${startItem}-${endItem} of ${total} tools${toolsSearchKeyword ? ` (search: "${escapeHtml(toolsSearchKeyword)}")` : ''}
        </div>
        <div class="pagination-page-size">
            <label for="tools-page-size-pagination">Per page:</label>
            <select id="tools-page-size-pagination" onchange="changeToolsPageSize()">
                <option value="10" ${savedPageSize === 10 ? 'selected' : ''}>10</option>
                <option value="20" ${savedPageSize === 20 ? 'selected' : ''}>20</option>
                <option value="50" ${savedPageSize === 50 ? 'selected' : ''}>50</option>
                <option value="100" ${savedPageSize === 100 ? 'selected' : ''}>100</option>
            </select>
        </div>
        <div class="pagination-controls">
            <button class="btn-secondary" onclick="loadToolsList(1, '${escapeHtml(toolsSearchKeyword)}')" ${page === 1 ? 'disabled' : ''}>First</button>
            <button class="btn-secondary" onclick="loadToolsList(${page - 1}, '${escapeHtml(toolsSearchKeyword)}')" ${page === 1 ? 'disabled' : ''}>Prev</button>
            <span class="pagination-page">Page ${page} / ${totalPages}</span>
            <button class="btn-secondary" onclick="loadToolsList(${page + 1}, '${escapeHtml(toolsSearchKeyword)}')" ${page === totalPages ? 'disabled' : ''}>Next</button>
            <button class="btn-secondary" onclick="loadToolsList(${totalPages}, '${escapeHtml(toolsSearchKeyword)}')" ${page === totalPages ? 'disabled' : ''}>Last</button>
        </div>
    `;
    
    toolsList.appendChild(pagination);
}

// Handle tool checkbox status change
function handleToolCheckboxChange(toolKey, enabled) {
    // Update global status map
    const toolItem = document.querySelector(`.tool-item[data-tool-key="${toolKey}"]`);
    if (toolItem) {
        const toolName = toolItem.dataset.toolName;
        const isExternal = toolItem.dataset.isExternal === 'true';
        const externalMcp = toolItem.dataset.externalMcp || '';
        toolStateMap.set(toolKey, {
            enabled: enabled,
            is_external: isExternal,
            external_mcp: externalMcp,
            name: toolName // Save original tool name
        });
    }
    updateToolsStats();
}

// Select all tools
function selectAllTools() {
    document.querySelectorAll('#tools-list input[type="checkbox"]').forEach(checkbox => {
        checkbox.checked = true;
        // Update global status map
        const toolItem = checkbox.closest('.tool-item');
        if (toolItem) {
            const toolKey = toolItem.dataset.toolKey;
            const toolName = toolItem.dataset.toolName;
            const isExternal = toolItem.dataset.isExternal === 'true';
            const externalMcp = toolItem.dataset.externalMcp || '';
            if (toolKey) {
                toolStateMap.set(toolKey, {
                    enabled: true,
                    is_external: isExternal,
                    external_mcp: externalMcp,
                    name: toolName // Save original tool name
                });
            }
        }
    });
    updateToolsStats();
}

// Deselect all tools
function deselectAllTools() {
    document.querySelectorAll('#tools-list input[type="checkbox"]').forEach(checkbox => {
        checkbox.checked = false;
        // Update global status map
        const toolItem = checkbox.closest('.tool-item');
        if (toolItem) {
            const toolKey = toolItem.dataset.toolKey;
            const toolName = toolItem.dataset.toolName;
            const isExternal = toolItem.dataset.isExternal === 'true';
            const externalMcp = toolItem.dataset.externalMcp || '';
            if (toolKey) {
                toolStateMap.set(toolKey, {
                    enabled: false,
                    is_external: isExternal,
                    external_mcp: externalMcp,
                    name: toolName // Save original tool name
                });
            }
        }
    });
    updateToolsStats();
}

// Change per-page count
async function changeToolsPageSize() {
    // Try to get selector from two locations (top or pagination area)
    const pageSizeSelect = document.getElementById('tools-page-size') || document.getElementById('tools-page-size-pagination');
    if (!pageSizeSelect) return;
    
    const newPageSize = parseInt(pageSizeSelect.value, 10);
    if (isNaN(newPageSize) || newPageSize < 1) {
        return;
    }
    
    // Save to localStorage
    localStorage.setItem('toolsPageSize', newPageSize.toString());
    
    // Update pagination config
    toolsPagination.pageSize = newPageSize;
    
    // Sync update the other selector (if exists)
    const otherSelect = document.getElementById('tools-page-size') || document.getElementById('tools-page-size-pagination');
    if (otherSelect && otherSelect !== pageSizeSelect) {
        otherSelect.value = newPageSize;
    }
    
    // Reload first page
    await loadToolsList(1, toolsSearchKeyword);
}

// Update tool statistics
async function updateToolsStats() {
    const statsEl = document.getElementById('tools-stats');
    if (!statsEl) return;
    
    // Save current page status to global map first
    saveCurrentPageToolStates();
    
    // Calculate enabled tool count on current page
    const currentPageEnabled = Array.from(document.querySelectorAll('#tools-list input[type="checkbox"]:checked')).length;
    const currentPageTotal = document.querySelectorAll('#tools-list input[type="checkbox"]').length;
    
    // Calculate enabled count for all tools
    let totalEnabled = 0;
    let totalTools = toolsPagination.total || 0;
    
    try {
        // If search keyword, only count search results
        if (toolsSearchKeyword) {
            totalTools = allTools.length;
            totalEnabled = allTools.filter(tool => {
                // Prefer global status map, then checkbox status, lastly server-returned status
                const toolKey = getToolKey(tool);
                const savedState = toolStateMap.get(toolKey);
                if (savedState !== undefined) {
                    return savedState.enabled;
                }
                const checkboxId = `tool-${toolKey.replace(/::/g, '--')}`;
                const checkbox = document.getElementById(checkboxId);
                return checkbox ? checkbox.checked : tool.enabled;
            }).length;
        } else {
            // Without search, need to get all tools status
            // Use global status map and current page checkbox status first
            const localStateMap = new Map();
            
            // Get status from current page checkboxes (if not in global map)
            allTools.forEach(tool => {
                const toolKey = getToolKey(tool);
                const savedState = toolStateMap.get(toolKey);
                if (savedState !== undefined) {
                    localStateMap.set(toolKey, savedState.enabled);
                } else {
                    const checkboxId = `tool-${toolKey.replace(/::/g, '--')}`;
                    const checkbox = document.getElementById(checkboxId);
                    if (checkbox) {
                        localStateMap.set(toolKey, checkbox.checked);
                    } else {
                        // If checkbox does not exist (not on current page), use original tool status
                        localStateMap.set(toolKey, tool.enabled);
                    }
                }
            });
            
            // If total tool count exceeds current page, need to get all tools status
            if (totalTools > allTools.length) {
                // Traverse all pages to get complete status
                let page = 1;
                let hasMore = true;
                const pageSize = 100; // Use large page size to reduce requests
                
                while (hasMore && page <= 10) { // Limit to max 10 pages to avoid infinite loop
                    const url = `/api/config/tools?page=${page}&page_size=${pageSize}`;
                    const pageResponse = await apiFetch(url);
                    if (!pageResponse.ok) break;
                    
                    const pageResult = await pageResponse.json();
                    pageResult.tools.forEach(tool => {
                        // Prefer global status map, then server-returned status
                        const toolKey = getToolKey(tool);
                        if (!localStateMap.has(toolKey)) {
                            const savedState = toolStateMap.get(toolKey);
                            localStateMap.set(toolKey, savedState ? savedState.enabled : tool.enabled);
                        }
                    });
                    
                    if (page >= pageResult.total_pages) {
                        hasMore = false;
                    } else {
                        page++;
                    }
                }
            }
            
            // Calculate enabled tool count
            totalEnabled = Array.from(localStateMap.values()).filter(enabled => enabled).length;
        }
    } catch (error) {
        console.warn('Failed to get tool stats, using current page data', error);
        // If fetch fails, use current page data
        totalTools = totalTools || currentPageTotal;
        totalEnabled = currentPageEnabled;
    }
    
    statsEl.innerHTML = `
        <span title="Number of enabled tools on current page">✅ Current page enabled: <strong>${currentPageEnabled}</strong> / ${currentPageTotal}</span>
        <span title="Total enabled tools among all tools">📊 Total enabled: <strong>${totalEnabled}</strong> / ${totalTools}</span>
    `;
}

// Filter tools (deprecated, now using server-side search)
// Keep this function in case it is called elsewhere, actual functionality replaced by searchTools()
function filterTools() {
    // No longer use client-side filtering, trigger server-side search instead
    // Can keep as empty function or remove oninput event
}

// Apply settings
async function applySettings() {
    try {
        // Clear previous validation error status
        document.querySelectorAll('.form-group input').forEach(input => {
            input.classList.remove('error');
        });
        
        // Validate required fields
        const apiKey = document.getElementById('openai-api-key').value.trim();
        const baseUrl = document.getElementById('openai-base-url').value.trim();
        const model = document.getElementById('openai-model').value.trim();
        
        let hasError = false;
        
        if (!apiKey) {
            document.getElementById('openai-api-key').classList.add('error');
            hasError = true;
        }
        
        if (!baseUrl) {
            document.getElementById('openai-base-url').classList.add('error');
            hasError = true;
        }
        
        if (!model) {
            document.getElementById('openai-model').classList.add('error');
            hasError = true;
        }
        
        if (hasError) {
            alert('Please fill in all required fields (fields marked with *)');
            return;
        }
        
        // Collect config
        const knowledgeEnabledCheckbox = document.getElementById('knowledge-enabled');
        const knowledgeEnabled = knowledgeEnabledCheckbox ? knowledgeEnabledCheckbox.checked : true;
        
        // Collect knowledge base config
        const knowledgeConfig = {
            enabled: knowledgeEnabled,
            base_path: document.getElementById('knowledge-base-path')?.value.trim() || 'knowledge_base',
            embedding: {
                provider: document.getElementById('knowledge-embedding-provider')?.value || 'openai',
                model: document.getElementById('knowledge-embedding-model')?.value.trim() || '',
                base_url: document.getElementById('knowledge-embedding-base-url')?.value.trim() || '',
                api_key: document.getElementById('knowledge-embedding-api-key')?.value.trim() || ''
            },
            retrieval: {
                top_k: parseInt(document.getElementById('knowledge-retrieval-top-k')?.value) || 5,
                similarity_threshold: (() => {
                    const val = parseFloat(document.getElementById('knowledge-retrieval-similarity-threshold')?.value);
                    return isNaN(val) ? 0.7 : val;
                })(),
                hybrid_weight: (() => {
                    const val = parseFloat(document.getElementById('knowledge-retrieval-hybrid-weight')?.value);
                    return isNaN(val) ? 0.7 : val; // Allow 0.0 value, only use default when NaN
                })()
            }
        };
        
        const wecomAgentIdVal = document.getElementById('robot-wecom-agent-id')?.value.trim();
        const config = {
            openai: {
                api_key: apiKey,
                base_url: baseUrl,
                model: model
            },
            fofa: {
                email: document.getElementById('fofa-email')?.value.trim() || '',
                api_key: document.getElementById('fofa-api-key')?.value.trim() || '',
                base_url: document.getElementById('fofa-base-url')?.value.trim() || ''
            },
            agent: {
                max_iterations: parseInt(document.getElementById('agent-max-iterations').value) || 30
            },
            knowledge: knowledgeConfig,
            robots: {
                wecom: {
                    enabled: document.getElementById('robot-wecom-enabled')?.checked === true,
                    token: document.getElementById('robot-wecom-token')?.value.trim() || '',
                    encoding_aes_key: document.getElementById('robot-wecom-encoding-aes-key')?.value.trim() || '',
                    corp_id: document.getElementById('robot-wecom-corp-id')?.value.trim() || '',
                    secret: document.getElementById('robot-wecom-secret')?.value.trim() || '',
                    agent_id: parseInt(wecomAgentIdVal, 10) || 0
                },
                dingtalk: {
                    enabled: document.getElementById('robot-dingtalk-enabled')?.checked === true,
                    client_id: document.getElementById('robot-dingtalk-client-id')?.value.trim() || '',
                    client_secret: document.getElementById('robot-dingtalk-client-secret')?.value.trim() || ''
                },
                lark: {
                    enabled: document.getElementById('robot-lark-enabled')?.checked === true,
                    app_id: document.getElementById('robot-lark-app-id')?.value.trim() || '',
                    app_secret: document.getElementById('robot-lark-app-secret')?.value.trim() || '',
                    verify_token: document.getElementById('robot-lark-verify-token')?.value.trim() || ''
                },
                telegram: {
                    enabled: document.getElementById('robot-telegram-enabled')?.checked === true,
                    bot_token: document.getElementById('robot-telegram-bot-token')?.value.trim() || '',
                    allowed_user_ids: (() => {
                        const raw = document.getElementById('robot-telegram-allowed-user-ids')?.value.trim() || '';
                        if (!raw) return [];
                        return raw.split(',').map(s => parseInt(s.trim(), 10)).filter(n => !isNaN(n));
                    })()
                }
            },
            tools: []
        };
        
        // Collect tool enabled status
        // Save current page status to global map first
        saveCurrentPageToolStates();
        
        // Get all tool list for complete status (traverse all pages)
        // Note: regardless of search status, get all tools status to ensure complete save
        try {
            const allToolsMap = new Map();
            let page = 1;
            let hasMore = true;
            const pageSize = 100; // Use reasonable page size
            
            // Traverse all pages to get all tools (no search keyword, get all tools)
            while (hasMore) {
                const url = `/api/config/tools?page=${page}&page_size=${pageSize}`;
                
                const pageResponse = await apiFetch(url);
                if (!pageResponse.ok) {
                    throw new Error('Failed to get tool list');
                }
                
                const pageResult = await pageResponse.json();
                
                // Add tools to map
                // Prefer status from global map (user-modified), then server-returned status
                pageResult.tools.forEach(tool => {
                    const toolKey = getToolKey(tool);
                    const savedState = toolStateMap.get(toolKey);
                    allToolsMap.set(toolKey, {
                        name: tool.name,
                        enabled: savedState ? savedState.enabled : tool.enabled,
                        is_external: savedState ? savedState.is_external : (tool.is_external || false),
                        external_mcp: savedState ? savedState.external_mcp : (tool.external_mcp || '')
                    });
                });
                
                // Check if there are more pages
                if (page >= pageResult.total_pages) {
                    hasMore = false;
                } else {
                    page++;
                }
            }
            
            // Add all tools to config
            allToolsMap.forEach((tool, toolKey) => {
                config.tools.push({
                    name: tool.name,
                    enabled: tool.enabled,
                    is_external: tool.is_external,
                    external_mcp: tool.external_mcp
                });
            });
        } catch (error) {
            console.warn('Failed to get all tool list, using global status map only', error);
            // If fetch fails, use global status map
            toolStateMap.forEach((toolData, toolKey) => {
                // toolData.name saves original tool name
                const toolName = toolData.name || toolKey.split('::').pop();
                config.tools.push({
                    name: toolName,
                    enabled: toolData.enabled,
                    is_external: toolData.is_external,
                    external_mcp: toolData.external_mcp
                });
            });
        }
        
        // Update config
        const updateResponse = await apiFetch('/api/config', {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(config)
        });
        
        if (!updateResponse.ok) {
            const error = await updateResponse.json();
            throw new Error(error.error || 'Failed to update config');
        }
        
        // Apply config
        const applyResponse = await apiFetch('/api/config/apply', {
            method: 'POST'
        });
        
        if (!applyResponse.ok) {
            const error = await applyResponse.json();
            throw new Error(error.error || 'Failed to apply config');
        }
        
        alert('Configuration applied successfully!');
        closeSettings();
    } catch (error) {
        console.error('Failed to apply config:', error);
        alert('Failed to apply config: ' + error.message);
    }
}

// Save tool config (standalone function, for MCP management page)
async function saveToolsConfig() {
    try {
        // Save current page status to global map first
        saveCurrentPageToolStates();
        
        // Get current config (only tool part)
        const response = await apiFetch('/api/config');
        if (!response.ok) {
            throw new Error('Failed to get config');
        }
        
        const currentConfig = await response.json();
        
        // Build config object containing only tool config
        const config = {
            openai: currentConfig.openai || {},
            agent: currentConfig.agent || {},
            tools: []
        };
        
        // Collect tool enabled status (same logic as in applySettings)
        try {
            const allToolsMap = new Map();
            let page = 1;
            let hasMore = true;
            const pageSize = 100;
            
            // Traverse all pages to get all tools
            while (hasMore) {
                const url = `/api/config/tools?page=${page}&page_size=${pageSize}`;
                
                const pageResponse = await apiFetch(url);
                if (!pageResponse.ok) {
                    throw new Error('Failed to get tool list');
                }
                
                const pageResult = await pageResponse.json();
                
                // Add tools to map
                pageResult.tools.forEach(tool => {
                    const toolKey = getToolKey(tool);
                    const savedState = toolStateMap.get(toolKey);
                    allToolsMap.set(toolKey, {
                        name: tool.name,
                        enabled: savedState ? savedState.enabled : tool.enabled,
                        is_external: savedState ? savedState.is_external : (tool.is_external || false),
                        external_mcp: savedState ? savedState.external_mcp : (tool.external_mcp || '')
                    });
                });
                
                // Check if there are more pages
                if (page >= pageResult.total_pages) {
                    hasMore = false;
                } else {
                    page++;
                }
            }
            
            // Add all tools to config
            allToolsMap.forEach((tool, toolKey) => {
                config.tools.push({
                    name: tool.name,
                    enabled: tool.enabled,
                    is_external: tool.is_external,
                    external_mcp: tool.external_mcp
                });
            });
        } catch (error) {
            console.warn('Failed to get all tool list, using global status map only', error);
            // If fetch fails, use global status map
            toolStateMap.forEach((toolData, toolKey) => {
                // toolData.name saves original tool name
                const toolName = toolData.name || toolKey.split('::').pop();
                config.tools.push({
                    name: toolName,
                    enabled: toolData.enabled,
                    is_external: toolData.is_external,
                    external_mcp: toolData.external_mcp
                });
            });
        }
        
        // Update config
        const updateResponse = await apiFetch('/api/config', {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(config)
        });
        
        if (!updateResponse.ok) {
            const error = await updateResponse.json();
            throw new Error(error.error || 'Failed to update config');
        }
        
        // Apply config
        const applyResponse = await apiFetch('/api/config/apply', {
            method: 'POST'
        });
        
        if (!applyResponse.ok) {
            const error = await applyResponse.json();
            throw new Error(error.error || 'Failed to apply config');
        }
        
        alert('Tool configuration saved successfully!');
        
        // Reload tool list to reflect latest status
        if (typeof loadToolsList === 'function') {
            await loadToolsList(toolsPagination.page, toolsSearchKeyword);
        }
    } catch (error) {
        console.error('Failed to save tool config:', error);
        alert('Failed to save tool config: ' + error.message);
    }
}

function resetPasswordForm() {
    const currentInput = document.getElementById('auth-current-password');
    const newInput = document.getElementById('auth-new-password');
    const confirmInput = document.getElementById('auth-confirm-password');

    [currentInput, newInput, confirmInput].forEach(input => {
        if (input) {
            input.value = '';
            input.classList.remove('error');
        }
    });
}

async function changePassword() {
    const currentInput = document.getElementById('auth-current-password');
    const newInput = document.getElementById('auth-new-password');
    const confirmInput = document.getElementById('auth-confirm-password');
    const submitBtn = document.querySelector('.change-password-submit');

    [currentInput, newInput, confirmInput].forEach(input => input && input.classList.remove('error'));

    const currentPassword = currentInput?.value.trim() || '';
    const newPassword = newInput?.value.trim() || '';
    const confirmPassword = confirmInput?.value.trim() || '';

    let hasError = false;

    if (!currentPassword) {
        currentInput?.classList.add('error');
        hasError = true;
    }

    if (!newPassword || newPassword.length < 8) {
        newInput?.classList.add('error');
        hasError = true;
    }

    if (newPassword !== confirmPassword) {
        confirmInput?.classList.add('error');
        hasError = true;
    }

    if (hasError) {
        alert('Please fill in the current password and new password correctly. The new password must be at least 8 characters and match the confirmation.');
        return;
    }

    if (submitBtn) {
        submitBtn.disabled = true;
    }

    try {
        const response = await apiFetch('/api/auth/change-password', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                oldPassword: currentPassword,
                newPassword: newPassword
            })
        });

        const result = await response.json().catch(() => ({}));
        if (!response.ok) {
            throw new Error(result.error || 'Failed to change password');
        }

        alert('Password updated. Please log in again with your new password.');
        resetPasswordForm();
        handleUnauthorized({ message: 'Password updated. Please log in again with your new password.', silent: false });
        closeSettings();
    } catch (error) {
        console.error('Failed to change password:', error);
        alert('Failed to change password: ' + error.message);
    } finally {
        if (submitBtn) {
            submitBtn.disabled = false;
        }
    }
}

// ==================== External MCP Management ====================

let currentEditingMCPName = null;

// Fetch external MCP list data (for polling, returns { servers, stats })
async function fetchExternalMCPs() {
    const response = await apiFetch('/api/external-mcp');
    if (!response.ok) throw new Error('Failed to get external MCP list');
    return response.json();
}

// Load external MCP list and render
async function loadExternalMCPs() {
    try {
        const data = await fetchExternalMCPs();
        renderExternalMCPList(data.servers || {});
        renderExternalMCPStats(data.stats || {});
    } catch (error) {
        console.error('Failed to load external MCP list:', error);
        const list = document.getElementById('external-mcp-list');
        if (list) {
            list.innerHTML = `<div class="error">Failed to load: ${escapeHtml(error.message)}</div>`;
        }
    }
}

// Poll list until specified MCP tool count is updated (poll every second, stop when obtained, no fixed delay)
// When name is null, only poll maxAttempts times without checking tool_count
async function pollExternalMCPToolCount(name, maxAttempts = 10) {
    const pollIntervalMs = 1000;
    for (let attempt = 0; attempt < maxAttempts; attempt++) {
        await new Promise(r => setTimeout(r, pollIntervalMs));
        try {
            const data = await fetchExternalMCPs();
            renderExternalMCPList(data.servers || {});
            renderExternalMCPStats(data.stats || {});
            if (name != null) {
                const server = data.servers && data.servers[name];
                if (server && server.tool_count > 0) break;
            }
        } catch (e) {
            console.warn('Failed to poll tool count:', e);
        }
    }
    if (typeof window !== 'undefined' && typeof window.refreshMentionTools === 'function') {
        window.refreshMentionTools();
    }
}

// Render external MCP list
function renderExternalMCPList(servers) {
    const list = document.getElementById('external-mcp-list');
    if (!list) return;
    
    if (Object.keys(servers).length === 0) {
        list.innerHTML = '<div class="empty">📋 No external MCP configurations<br><span style="font-size: 0.875rem; margin-top: 8px; display: block;">Click the "Add External MCP" button to start configuring</span></div>';
        return;
    }
    
    let html = '<div class="external-mcp-items">';
    for (const [name, server] of Object.entries(servers)) {
        const status = server.status || 'disconnected';
        const statusClass = status === 'connected' ? 'status-connected' : 
                           status === 'connecting' ? 'status-connecting' :
                           status === 'error' ? 'status-error' :
                           status === 'disabled' ? 'status-disabled' : 'status-disconnected';
        const statusText = status === 'connected' ? 'Connected' : 
                          status === 'connecting' ? 'Connecting...' :
                          status === 'error' ? 'Connection failed' :
                          status === 'disabled' ? 'Disabled' : 'Not connected';
        const transport = server.config.transport || (server.config.command ? 'stdio' : 'http');
        const transportIcon = transport === 'stdio' ? '⚙️' : '🌐';
        
        html += `
            <div class="external-mcp-item">
                <div class="external-mcp-item-header">
                    <div class="external-mcp-item-info">
                        <h4>${transportIcon} ${escapeHtml(name)}${server.tool_count !== undefined && server.tool_count > 0 ? `<span class="tool-count-badge" title="Tool count">🔧 ${server.tool_count}</span>` : ''}</h4>
                        <span class="external-mcp-status ${statusClass}">${statusText}</span>
                    </div>
                    <div class="external-mcp-item-actions">
                        ${status === 'connected' || status === 'disconnected' || status === 'error' ? 
                            `<button class="btn-small" id="btn-toggle-${escapeHtml(name)}" onclick="toggleExternalMCP('${escapeHtml(name)}', '${status}')" title="${status === 'connected' ? 'Stop connection' : 'Start connection'}">
                                ${status === 'connected' ? '⏸ Stop' : '▶ Start'}
                            </button>` : 
                            status === 'connecting' ? 
                            `<button class="btn-small" id="btn-toggle-${escapeHtml(name)}" disabled style="opacity: 0.6; cursor: not-allowed;">
                                ⏳ Connecting...
                            </button>` : ''}
                        <button class="btn-small" onclick="editExternalMCP('${escapeHtml(name)}')" title="Edit config" ${status === 'connecting' ? 'disabled' : ''}>✏️ Edit</button>
                        <button class="btn-small btn-danger" onclick="deleteExternalMCP('${escapeHtml(name)}')" title="Delete config" ${status === 'connecting' ? 'disabled' : ''}>🗑 Delete</button>
                    </div>
                </div>
                ${status === 'error' && server.error ? `
                <div class="external-mcp-error" style="margin: 12px 0; padding: 12px; background: #fee; border-left: 3px solid #f44; border-radius: 4px; color: #c33; font-size: 0.875rem;">
                    <strong>❌ Connection error:</strong> ${escapeHtml(server.error)}
                </div>` : ''}
                <div class="external-mcp-item-details">
                    <div>
                        <strong>Transport Mode</strong>
                        <span>${transportIcon} ${escapeHtml(transport.toUpperCase())}</span>
                    </div>
                    ${server.tool_count !== undefined && server.tool_count > 0 ? `
                    <div>
                        <strong>Tool Count</strong>
                        <span style="font-weight: 600; color: var(--accent-color);">🔧 ${server.tool_count} tool(s)</span>
                    </div>` : server.tool_count === 0 && status === 'connected' ? `
                    <div>
                        <strong>Tool Count</strong>
                        <span style="color: var(--text-muted);">No tools</span>
                    </div>` : ''}
                    ${server.config.description ? `
                    <div>
                        <strong>Description</strong>
                        <span>${escapeHtml(server.config.description)}</span>
                    </div>` : ''}
                    ${server.config.timeout ? `
                    <div>
                        <strong>Timeout</strong>
                        <span>${server.config.timeout} seconds</span>
                    </div>` : ''}
                    ${transport === 'stdio' && server.config.command ? `
                    <div>
                        <strong>Command</strong>
                        <span style="font-family: monospace; font-size: 0.8125rem;">${escapeHtml(server.config.command)}</span>
                    </div>` : ''}
                    ${transport === 'http' && server.config.url ? `
                    <div>
                        <strong>URL</strong>
                        <span style="font-family: monospace; font-size: 0.8125rem; word-break: break-all;">${escapeHtml(server.config.url)}</span>
                    </div>` : ''}
                </div>
            </div>
        `;
    }
    html += '</div>';
    list.innerHTML = html;
}

// Render external MCP statistics
function renderExternalMCPStats(stats) {
    const statsEl = document.getElementById('external-mcp-stats');
    if (!statsEl) return;
    
    const total = stats.total || 0;
    const enabled = stats.enabled || 0;
    const disabled = stats.disabled || 0;
    const connected = stats.connected || 0;
    
    statsEl.innerHTML = `
        <span title="Total config count">📊 Total: <strong>${total}</strong></span>
        <span title="Enabled config count">✅ Enabled: <strong>${enabled}</strong></span>
        <span title="Disabled config count">⏸ Disabled: <strong>${disabled}</strong></span>
        <span title="Currently connected config count">🔗 Connected: <strong>${connected}</strong></span>
    `;
}

// Show Add External MCP modal
function showAddExternalMCPModal() {
    currentEditingMCPName = null;
    document.getElementById('external-mcp-modal-title').textContent = 'AddExternalMCP';
    document.getElementById('external-mcp-json').value = '';
    document.getElementById('external-mcp-json-error').style.display = 'none';
    document.getElementById('external-mcp-json-error').textContent = '';
    document.getElementById('external-mcp-json').classList.remove('error');
    document.getElementById('external-mcp-modal').style.display = 'block';
}

// Close External MCP modal
function closeExternalMCPModal() {
    document.getElementById('external-mcp-modal').style.display = 'none';
    currentEditingMCPName = null;
}

// EditExternalMCP
async function editExternalMCP(name) {
    try {
        const response = await apiFetch(`/api/external-mcp/${encodeURIComponent(name)}`);
        if (!response.ok) {
            throw new Error('Failed to get external MCP config');
        }
        
        const server = await response.json();
        currentEditingMCPName = name;
        
        document.getElementById('external-mcp-modal-title').textContent = 'EditExternalMCP';
        
        // Convert config to object format (key is name)
        const config = { ...server.config };
        // Remove frontend fields like tool_count, external_mcp_enable, but keep enabled/disabled for backward compat
        delete config.tool_count;
        delete config.external_mcp_enable;
        
        // Wrap into object format: { "name": { config } }
        const configObj = {};
        configObj[name] = config;
        
        // Format JSON
        const jsonStr = JSON.stringify(configObj, null, 2);
        document.getElementById('external-mcp-json').value = jsonStr;
        document.getElementById('external-mcp-json-error').style.display = 'none';
        document.getElementById('external-mcp-json-error').textContent = '';
        document.getElementById('external-mcp-json').classList.remove('error');
        
        document.getElementById('external-mcp-modal').style.display = 'block';
    } catch (error) {
        console.error('EditExternalMCPFailed:', error);
        alert('EditFailed: ' + error.message);
    }
}

// Format JSON
function formatExternalMCPJSON() {
    const jsonTextarea = document.getElementById('external-mcp-json');
    const errorDiv = document.getElementById('external-mcp-json-error');
    
    try {
        const jsonStr = jsonTextarea.value.trim();
        if (!jsonStr) {
            errorDiv.textContent = 'JSON cannot be empty';
            errorDiv.style.display = 'block';
            jsonTextarea.classList.add('error');
            return;
        }
        
        const parsed = JSON.parse(jsonStr);
        const formatted = JSON.stringify(parsed, null, 2);
        jsonTextarea.value = formatted;
        errorDiv.style.display = 'none';
        jsonTextarea.classList.remove('error');
    } catch (error) {
        errorDiv.textContent = 'JSON format error: ' + error.message;
        errorDiv.style.display = 'block';
        jsonTextarea.classList.add('error');
    }
}

// Load example
function loadExternalMCPExample() {
    const example = {
        "hexstrike-ai": {
            command: "python3",
            args: [
                "/path/to/script.py",
                "--server",
                "http://example.com"
            ],
            description: "Example description",
            timeout: 300
        },
        "cyberstrike-ai-http": {
            transport: "http",
            url: "http://127.0.0.1:8081/mcp"
        },
        "cyberstrike-ai-sse": {
            transport: "sse",
            url: "http://127.0.0.1:8081/mcp/sse"
        }
    };
    
    document.getElementById('external-mcp-json').value = JSON.stringify(example, null, 2);
    document.getElementById('external-mcp-json-error').style.display = 'none';
    document.getElementById('external-mcp-json').classList.remove('error');
}

// SaveExternalMCP
async function saveExternalMCP() {
    const jsonTextarea = document.getElementById('external-mcp-json');
    const jsonStr = jsonTextarea.value.trim();
    const errorDiv = document.getElementById('external-mcp-json-error');
    
    if (!jsonStr) {
        errorDiv.textContent = 'JSON config cannot be empty';
        errorDiv.style.display = 'block';
        jsonTextarea.classList.add('error');
        jsonTextarea.focus();
        return;
    }
    
    let configObj;
    try {
        configObj = JSON.parse(jsonStr);
    } catch (error) {
        errorDiv.textContent = 'JSON format error: ' + error.message;
        errorDiv.style.display = 'block';
        jsonTextarea.classList.add('error');
        jsonTextarea.focus();
        return;
    }
    
    // Validate must be object format
    if (typeof configObj !== 'object' || Array.isArray(configObj) || configObj === null) {
        errorDiv.textContent = 'Config error: must be JSON object format, key is config name, value is config content';
        errorDiv.style.display = 'block';
        jsonTextarea.classList.add('error');
        return;
    }
    
    // Get all config names
    const names = Object.keys(configObj);
    if (names.length === 0) {
        errorDiv.textContent = 'Config error: at least one config item is required';
        errorDiv.style.display = 'block';
        jsonTextarea.classList.add('error');
        return;
    }
    
    // Validate each config
    for (const name of names) {
        if (!name || name.trim() === '') {
            errorDiv.textContent = 'Config error: config name cannot be empty';
            errorDiv.style.display = 'block';
            jsonTextarea.classList.add('error');
            return;
        }
        
        const config = configObj[name];
        if (typeof config !== 'object' || Array.isArray(config) || config === null) {
            errorDiv.textContent = `Configuration error: configuration for "${name}" must be an object`;
            errorDiv.style.display = 'block';
            jsonTextarea.classList.add('error');
            return;
        }
        
        // Remove external_mcp_enable field (controlled by button, but keep enabled/disabled for backward compat)
        delete config.external_mcp_enable;
        
        // Validate config content
        const transport = config.transport || (config.command ? 'stdio' : config.url ? 'http' : '');
        if (!transport) {
            errorDiv.textContent = `Configuration error: "${name}" requires command (stdio mode) or url (http/sse mode)`;
            errorDiv.style.display = 'block';
            jsonTextarea.classList.add('error');
            return;
        }
        
        if (transport === 'stdio' && !config.command) {
            errorDiv.textContent = `Configuration error: "${name}" stdio mode requires command field`;
            errorDiv.style.display = 'block';
            jsonTextarea.classList.add('error');
            return;
        }
        
        if (transport === 'http' && !config.url) {
            errorDiv.textContent = `Configuration error: "${name}" http mode requires url field`;
            errorDiv.style.display = 'block';
            jsonTextarea.classList.add('error');
            return;
        }
        
        if (transport === 'sse' && !config.url) {
            errorDiv.textContent = `Configuration error: "${name}" sse mode requires url field`;
            errorDiv.style.display = 'block';
            jsonTextarea.classList.add('error');
            return;
        }
    }
    
    // Clear error prompt
    errorDiv.style.display = 'none';
    jsonTextarea.classList.remove('error');
    
    try {
        // If in edit mode, only update current editing config
        if (currentEditingMCPName) {
            if (!configObj[currentEditingMCPName]) {
                errorDiv.textContent = `Configuration error: in edit mode, JSON must contain config name "${currentEditingMCPName}"`;
                errorDiv.style.display = 'block';
                jsonTextarea.classList.add('error');
                return;
            }
            
            const response = await apiFetch(`/api/external-mcp/${encodeURIComponent(currentEditingMCPName)}`, {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ config: configObj[currentEditingMCPName] }),
            });
            
            if (!response.ok) {
                const error = await response.json();
                throw new Error(error.error || 'Save failed');
            }
        } else {
            // Add mode: save all configs
            for (const name of names) {
                const config = configObj[name];
                const response = await apiFetch(`/api/external-mcp/${encodeURIComponent(name)}`, {
                    method: 'PUT',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify({ config }),
                });
                
                if (!response.ok) {
                    const error = await response.json();
                    throw new Error(`Save "${name}" Failed: ${error.error || 'Unknown error'}`);
                }
            }
        }
        
        closeExternalMCPModal();
        await loadExternalMCPs();
        if (typeof window !== 'undefined' && typeof window.refreshMentionTools === 'function') {
            window.refreshMentionTools();
        }
        // Poll a few times to get asynchronously updated tool count from backend (no fixed delay, stop when obtained)
        pollExternalMCPToolCount(null, 5);
        alert('Saved successfully');
    } catch (error) {
        console.error('SaveExternalMCPFailed:', error);
        errorDiv.textContent = 'Save failed: ' + error.message;
        errorDiv.style.display = 'block';
        jsonTextarea.classList.add('error');
    }
}

// DeleteExternalMCP
async function deleteExternalMCP(name) {
    if (!confirm(`Are you sure you want to delete external MCP "${name}"?`)) {
        return;
    }
    
    try {
        const response = await apiFetch(`/api/external-mcp/${encodeURIComponent(name)}`, {
            method: 'DELETE',
        });
        
        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || 'Delete failed');
        }
        
        await loadExternalMCPs();
        // Refresh chat interface tool list, remove deleted MCP tools
        if (typeof window !== 'undefined' && typeof window.refreshMentionTools === 'function') {
            window.refreshMentionTools();
        }
        alert('Deleted successfully');
    } catch (error) {
        console.error('DeleteExternalMCPFailed:', error);
        alert('Failed to delete: ' + error.message);
    }
}

// Toggle external MCP start/stop
async function toggleExternalMCP(name, currentStatus) {
    const action = currentStatus === 'connected' ? 'stop' : 'start';
    const buttonId = `btn-toggle-${name}`;
    const button = document.getElementById(buttonId);
    
    // If starting, show loading status
    if (action === 'start' && button) {
        button.disabled = true;
        button.style.opacity = '0.6';
        button.style.cursor = 'not-allowed';
        button.innerHTML = '⏳ Connecting...';
    }
    
    try {
        const response = await apiFetch(`/api/external-mcp/${encodeURIComponent(name)}/${action}`, {
            method: 'POST',
        });
        
        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || 'Operation failed');
        }
        
        const result = await response.json();
        
        // If starting, immediately check status once
        if (action === 'start') {
            // Immediately check status once (may already be connected)
            try {
                const statusResponse = await apiFetch(`/api/external-mcp/${encodeURIComponent(name)}`);
                if (statusResponse.ok) {
                    const statusData = await statusResponse.json();
                    const status = statusData.status || 'disconnected';
                    
                    if (status === 'connected') {
                        await loadExternalMCPs();
                        if (typeof window !== 'undefined' && typeof window.refreshMentionTools === 'function') {
                            window.refreshMentionTools();
                        }
                        // Poll until MCP tool count is updated (poll every second, no fixed delay)
                        pollExternalMCPToolCount(name, 10);
                        return;
                    }
                }
            } catch (error) {
                console.error('Failed to check status:', error);
            }
            
            // If not yet connected, start polling
            await pollExternalMCPStatus(name, 30); // Poll at most 30 times (about 30 seconds)
        } else {
            // Stop action, refresh directly
            await loadExternalMCPs();
            // Refresh chat interface tool list
            if (typeof window !== 'undefined' && typeof window.refreshMentionTools === 'function') {
                window.refreshMentionTools();
            }
        }
    } catch (error) {
        console.error('Failed to toggle external MCP status:', error);
        alert('Operation failed: ' + error.message);
        
        // Restore button status
        if (button) {
            button.disabled = false;
            button.style.opacity = '1';
            button.style.cursor = 'pointer';
            button.innerHTML = '▶ Start';
        }
        
        // Refresh status
        await loadExternalMCPs();
        // Refresh chat interface tool list
        if (typeof window !== 'undefined' && typeof window.refreshMentionTools === 'function') {
            window.refreshMentionTools();
        }
    }
}

// Poll external MCP status
async function pollExternalMCPStatus(name, maxAttempts = 30) {
    let attempts = 0;
    const pollInterval = 1000; // Poll every 1 second
    
    while (attempts < maxAttempts) {
        await new Promise(resolve => setTimeout(resolve, pollInterval));
        
        try {
            const response = await apiFetch(`/api/external-mcp/${encodeURIComponent(name)}`);
            if (response.ok) {
                const data = await response.json();
                const status = data.status || 'disconnected';
                
                // Update button status
                const buttonId = `btn-toggle-${name}`;
                const button = document.getElementById(buttonId);
                
                if (status === 'connected') {
                    await loadExternalMCPs();
                    if (typeof window !== 'undefined' && typeof window.refreshMentionTools === 'function') {
                        window.refreshMentionTools();
                    }
                    // Poll until MCP tool count is updated (poll every second, no fixed delay)
                    pollExternalMCPToolCount(name, 10);
                    return;
                } else if (status === 'error' || status === 'disconnected') {
                    // Connection failed, refresh list and show error
                    await loadExternalMCPs();
                    // Refresh chat interface tool list
                    if (typeof window !== 'undefined' && typeof window.refreshMentionTools === 'function') {
                        window.refreshMentionTools();
                    }
                    if (status === 'error') {
                        alert('Connection failed, please check config and network connection');
                    }
                    return;
                } else if (status === 'connecting') {
                    // Still connecting, continue polling
                    attempts++;
                    continue;
                }
            }
        } catch (error) {
            console.error('Failed to poll status:', error);
        }
        
        attempts++;
    }
    
    // Timeout, refresh list
    await loadExternalMCPs();
    // Refresh chat interface tool list
    if (typeof window !== 'undefined' && typeof window.refreshMentionTools === 'function') {
        window.refreshMentionTools();
    }
    alert('Connection timed out, please check config and network connection');
}

// Load external MCP list when settings open
const originalOpenSettings = openSettings;
openSettings = async function() {
    await originalOpenSettings();
    await loadExternalMCPs();
};
