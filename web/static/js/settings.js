// 设置相关功能
let currentConfig = null;
let allTools = [];
// 全局工具状态映射，用于保存用户在所有页面的修改
// key: 唯一工具标识符（toolKey），value: { enabled: boolean, is_external: boolean, external_mcp: string }
let toolStateMap = new Map();

// 生成工具的唯一标识符，用于区分同名但来源不同的工具
function getToolKey(tool) {
    // 如果是外部工具，使用 external_mcp::tool.name 作为唯一标识
    // 如果是内部工具，使用 tool.name 作为标识
    if (tool.is_external && tool.external_mcp) {
        return `${tool.external_mcp}::${tool.name}`;
    }
    return tool.name;
}
// 从localStorage读取每页显示数量，默认为20
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

// 切换设置分类
function switchSettingsSection(section) {
    // 更新导航项状态
    document.querySelectorAll('.settings-nav-item').forEach(item => {
        item.classList.remove('active');
    });
    const activeNavItem = document.querySelector(`.settings-nav-item[data-section="${section}"]`);
    if (activeNavItem) {
        activeNavItem.classList.add('active');
    }
    
    // 更新内容区域显示
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

// 打开设置
async function openSettings() {
    // 切换到设置页面
    if (typeof switchPage === 'function') {
        switchPage('settings');
    }
    
    // 每次打开时清空全局状态映射，重新加载最新配置
    toolStateMap.clear();
    
    // 每次打开时重新加载最新配置（系统设置页面不需要加载工具列表）
    await loadConfig(false);
    
    // 清除之前的验证错误状态
    document.querySelectorAll('.form-group input').forEach(input => {
        input.classList.remove('error');
    });
    
    // 默认显示基本设置
    switchSettingsSection('basic');
}

// 关闭设置（保留函数以兼容旧代码，但现在不需要关闭功能）
function closeSettings() {
    // 不再需要关闭功能，因为现在是页面而不是模态框
    // 如果需要，可以切换回对话页面
    if (typeof switchPage === 'function') {
        switchPage('chat');
    }
}

// 点击模态框外部关闭（只保留MCP详情模态框）
window.onclick = function(event) {
    const mcpModal = document.getElementById('mcp-detail-modal');
    
    if (event.target === mcpModal) {
        closeMCPDetail();
    }
}

// 加载配置
async function loadConfig(loadTools = true) {
    try {
        const response = await apiFetch('/api/config');
        if (!response.ok) {
            throw new Error('获取配置失败');
        }
        
        currentConfig = await response.json();
        
        // 填充OpenAI配置
        document.getElementById('openai-api-key').value = currentConfig.openai.api_key || '';
        document.getElementById('openai-base-url').value = currentConfig.openai.base_url || '';
        document.getElementById('openai-model').value = currentConfig.openai.model || '';

        // 填充FOFA配置
        const fofa = currentConfig.fofa || {};
        const fofaEmailEl = document.getElementById('fofa-email');
        const fofaKeyEl = document.getElementById('fofa-api-key');
        const fofaBaseUrlEl = document.getElementById('fofa-base-url');
        if (fofaEmailEl) fofaEmailEl.value = fofa.email || '';
        if (fofaKeyEl) fofaKeyEl.value = fofa.api_key || '';
        if (fofaBaseUrlEl) fofaBaseUrlEl.value = fofa.base_url || '';
        
        // 填充Agent配置
        document.getElementById('agent-max-iterations').value = currentConfig.agent.max_iterations || 30;

        const ma = currentConfig.multi_agent || {};
        const maEn = document.getElementById('multi-agent-enabled');
        if (maEn) maEn.checked = ma.enabled === true;
        const maMode = document.getElementById('multi-agent-default-mode');
        if (maMode) maMode.value = (ma.default_mode === 'multi') ? 'multi' : 'single';
        const maRobot = document.getElementById('multi-agent-robot-use');
        if (maRobot) maRobot.checked = ma.robot_use_multi_agent === true;
        const maBatch = document.getElementById('multi-agent-batch-use');
        if (maBatch) maBatch.checked = ma.batch_use_multi_agent === true;
        
        // 填充知识库配置
        const knowledgeEnabledCheckbox = document.getElementById('knowledge-enabled');
        if (knowledgeEnabledCheckbox) {
            knowledgeEnabledCheckbox.checked = currentConfig.knowledge?.enabled !== false;
        }
        
        // 填充知识库详细配置
        if (currentConfig.knowledge) {
            const knowledge = currentConfig.knowledge;
            
            // 基本配置
            const basePathInput = document.getElementById('knowledge-base-path');
            if (basePathInput) {
                basePathInput.value = knowledge.base_path || 'knowledge_base';
            }
            
            // 嵌入模型配置
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
            
            // 检索配置
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
                // 允许0.0值，只有undefined/null时才使用默认值
                retrievalWeightInput.value = (hybridWeight !== undefined && hybridWeight !== null) ? hybridWeight : 0.7;
            }

            // 索引配置
            const indexing = knowledge.indexing || {};
            const chunkSizeInput = document.getElementById('knowledge-indexing-chunk-size');
            if (chunkSizeInput) {
                chunkSizeInput.value = indexing.chunk_size || 512;
            }

            const chunkOverlapInput = document.getElementById('knowledge-indexing-chunk-overlap');
            if (chunkOverlapInput) {
                chunkOverlapInput.value = indexing.chunk_overlap ?? 50;
            }

            const maxChunksPerItemInput = document.getElementById('knowledge-indexing-max-chunks-per-item');
            if (maxChunksPerItemInput) {
                maxChunksPerItemInput.value = indexing.max_chunks_per_item ?? 0;
            }

            const maxRpmInput = document.getElementById('knowledge-indexing-max-rpm');
            if (maxRpmInput) {
                maxRpmInput.value = indexing.max_rpm ?? 0;
            }

            const rateLimitDelayInput = document.getElementById('knowledge-indexing-rate-limit-delay-ms');
            if (rateLimitDelayInput) {
                rateLimitDelayInput.value = indexing.rate_limit_delay_ms ?? 300;
            }

            const maxRetriesInput = document.getElementById('knowledge-indexing-max-retries');
            if (maxRetriesInput) {
                maxRetriesInput.value = indexing.max_retries ?? 3;
            }

            const retryDelayInput = document.getElementById('knowledge-indexing-retry-delay-ms');
            if (retryDelayInput) {
                retryDelayInput.value = indexing.retry_delay_ms ?? 1000;
            }
        }

        // 填充机器人配置
        const robots = currentConfig.robots || {};
        const wecom = robots.wecom || {};
        const dingtalk = robots.dingtalk || {};
        const lark = robots.lark || {};
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
        
        // 只有在需要时才加载工具列表（MCP管理页面需要，系统设置页面不需要）
        if (loadTools) {
            // 设置每页显示数量（会在分页控件渲染时设置）
            const savedPageSize = getToolsPageSize();
            toolsPagination.pageSize = savedPageSize;
            
            // 加载工具列表（使用分页）
            toolsSearchKeyword = '';
            await loadToolsList(1, '');
        }
    } catch (error) {
        console.error('加载配置失败:', error);
        const baseMsg = (typeof window !== 'undefined' && typeof window.t === 'function')
            ? window.t('settings.apply.loadFailed')
            : '加载配置失败';
        alert(baseMsg + ': ' + error.message);
    }
}

// 工具搜索关键词
let toolsSearchKeyword = '';

// 加载工具列表（分页）
async function loadToolsList(page = 1, searchKeyword = '') {
    const toolsList = document.getElementById('tools-list');
    
    // 显示加载状态
    if (toolsList) {
        // 清空整个容器，包括可能存在的分页控件
        toolsList.innerHTML = '<div class="tools-list-items"><div class="loading" style="padding: 20px; text-align: center; color: var(--text-muted);">⏳ ' + (typeof window.t === 'function' ? window.t('mcp.loadingTools') : '正在加载工具列表...') + '</div></div>';
    }
    
    try {
        // 在加载新页面之前，先保存当前页的状态到全局映射
        saveCurrentPageToolStates();
        
        const pageSize = toolsPagination.pageSize;
        let url = `/api/config/tools?page=${page}&page_size=${pageSize}`;
        if (searchKeyword) {
            url += `&search=${encodeURIComponent(searchKeyword)}`;
        }
        
        // 使用较短的超时时间（10秒），避免长时间等待
        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), 10000);
        
        const response = await apiFetch(url, {
            signal: controller.signal
        });
        clearTimeout(timeoutId);
        
        if (!response.ok) {
            throw new Error('获取工具列表失败');
        }
        
        const result = await response.json();
        allTools = result.tools || [];
        toolsPagination = {
            page: result.page || page,
            pageSize: result.page_size || pageSize,
            total: result.total || 0,
            totalPages: result.total_pages || 1
        };
        
        // 初始化工具状态映射（如果工具不在映射中，使用服务器返回的状态）
        allTools.forEach(tool => {
            const toolKey = getToolKey(tool);
            if (!toolStateMap.has(toolKey)) {
                toolStateMap.set(toolKey, {
                    enabled: tool.enabled,
                    is_external: tool.is_external || false,
                    external_mcp: tool.external_mcp || '',
                    name: tool.name // 保存原始工具名称
                });
            }
        });
        
        renderToolsList();
        renderToolsPagination();
    } catch (error) {
        console.error('加载工具列表失败:', error);
        if (toolsList) {
            const isTimeout = error.name === 'AbortError' || error.message.includes('timeout');
            const errorMsg = isTimeout 
                ? (typeof window.t === 'function' ? window.t('mcp.loadToolsTimeout') : '加载工具列表超时，可能是外部MCP连接较慢。请点击"刷新"按钮重试，或检查外部MCP连接状态。')
                : (typeof window.t === 'function' ? window.t('mcp.loadToolsFailed') : '加载工具列表失败') + ': ' + escapeHtml(error.message);
            toolsList.innerHTML = `<div class="error" style="padding: 20px; text-align: center;">${errorMsg}</div>`;
        }
    }
}

// 保存当前页的工具状态到全局映射
function saveCurrentPageToolStates() {
    document.querySelectorAll('#tools-list .tool-item').forEach(item => {
        const checkbox = item.querySelector('input[type="checkbox"]');
        const toolKey = item.dataset.toolKey; // 使用唯一标识符
        const toolName = item.dataset.toolName;
        const isExternal = item.dataset.isExternal === 'true';
        const externalMcp = item.dataset.externalMcp || '';
        if (toolKey && checkbox) {
            toolStateMap.set(toolKey, {
                enabled: checkbox.checked,
                is_external: isExternal,
                external_mcp: externalMcp,
                name: toolName // 保存原始工具名称
            });
        }
    });
}

// 搜索工具
function searchTools() {
    const searchInput = document.getElementById('tools-search');
    const keyword = searchInput ? searchInput.value.trim() : '';
    toolsSearchKeyword = keyword;
    // 搜索时重置到第一页
    loadToolsList(1, keyword);
}

// 清除搜索
function clearSearch() {
    const searchInput = document.getElementById('tools-search');
    if (searchInput) {
        searchInput.value = '';
    }
    toolsSearchKeyword = '';
    loadToolsList(1, '');
}

// 处理搜索框回车事件
function handleSearchKeyPress(event) {
    if (event.key === 'Enter') {
        searchTools();
    }
}

// 渲染工具列表
function renderToolsList() {
    const toolsList = document.getElementById('tools-list');
    if (!toolsList) return;
    
    // 移除可能存在的分页控件（会在 renderToolsPagination 中重新添加）
    const oldPagination = toolsList.querySelector('.tools-pagination');
    if (oldPagination) {
        oldPagination.remove();
    }
    
    // 获取或创建列表容器
    let listContainer = toolsList.querySelector('.tools-list-items');
    if (!listContainer) {
        listContainer = document.createElement('div');
        listContainer.className = 'tools-list-items';
        toolsList.appendChild(listContainer);
    }
    
    // 清空列表容器内容（移除加载提示）
    listContainer.innerHTML = '';
    
    if (allTools.length === 0) {
        listContainer.innerHTML = '<div class="empty">' + (typeof window.t === 'function' ? window.t('mcp.noTools') : '暂无工具') + '</div>';
        if (!toolsList.contains(listContainer)) {
            toolsList.appendChild(listContainer);
        }
        // 更新统计
        updateToolsStats();
        return;
    }
    
    allTools.forEach(tool => {
        const toolKey = getToolKey(tool); // 生成唯一标识符
        const toolItem = document.createElement('div');
        toolItem.className = 'tool-item';
        toolItem.dataset.toolKey = toolKey; // 保存唯一标识符
        toolItem.dataset.toolName = tool.name; // 保存原始工具名称
        toolItem.dataset.isExternal = tool.is_external ? 'true' : 'false';
        toolItem.dataset.externalMcp = tool.external_mcp || '';
        
        // 从全局状态映射获取工具状态，如果不存在则使用服务器返回的状态
        const toolState = toolStateMap.get(toolKey) || {
            enabled: tool.enabled,
            is_external: tool.is_external || false,
            external_mcp: tool.external_mcp || ''
        };
        
        // 外部工具标签，显示来源信息
        let externalBadge = '';
        if (toolState.is_external || tool.is_external) {
            const externalMcpName = toolState.external_mcp || tool.external_mcp || '';
            const badgeText = externalMcpName ? (typeof window.t === 'function' ? window.t('mcp.externalFrom', { name: escapeHtml(externalMcpName) }) : `外部 (${escapeHtml(externalMcpName)})`) : (typeof window.t === 'function' ? window.t('mcp.externalBadge') : '外部');
            const badgeTitle = externalMcpName ? (typeof window.t === 'function' ? window.t('mcp.externalToolFrom', { name: escapeHtml(externalMcpName) }) : `外部MCP工具 - 来源：${escapeHtml(externalMcpName)}`) : (typeof window.t === 'function' ? window.t('mcp.externalBadge') : '外部MCP工具');
            externalBadge = `<span class="external-tool-badge" title="${badgeTitle}">${badgeText}</span>`;
        }
        
        // 生成唯一的checkbox id，使用工具唯一标识符
        const checkboxId = `tool-${escapeHtml(toolKey).replace(/::/g, '--')}`;
        
        toolItem.innerHTML = `
            <input type="checkbox" id="${checkboxId}" ${toolState.enabled ? 'checked' : ''} ${toolState.is_external || tool.is_external ? 'data-external="true"' : ''} onchange="handleToolCheckboxChange('${escapeHtml(toolKey)}', this.checked)" />
            <div class="tool-item-info">
                <div class="tool-item-name">
                    ${escapeHtml(tool.name)}
                    ${externalBadge}
                </div>
                <div class="tool-item-desc">${escapeHtml(tool.description || (typeof window.t === 'function' ? window.t('mcp.noDescription') : '无描述'))}</div>
            </div>
        `;
        listContainer.appendChild(toolItem);
    });
    
    if (!toolsList.contains(listContainer)) {
        toolsList.appendChild(listContainer);
    }
    
    // 更新统计
    updateToolsStats();
}

// 渲染工具列表分页控件
function renderToolsPagination() {
    const toolsList = document.getElementById('tools-list');
    if (!toolsList) return;
    
    // 移除旧的分页控件
    const oldPagination = toolsList.querySelector('.tools-pagination');
    if (oldPagination) {
        oldPagination.remove();
    }
    
    // 如果只有一页或没有数据，不显示分页
    if (toolsPagination.totalPages <= 1) {
        return;
    }
    
    const pagination = document.createElement('div');
    pagination.className = 'tools-pagination';
    
    const { page, totalPages, total } = toolsPagination;
    const startItem = (page - 1) * toolsPagination.pageSize + 1;
    const endItem = Math.min(page * toolsPagination.pageSize, total);
    
    const savedPageSize = getToolsPageSize();
    const t = typeof window.t === 'function' ? window.t : (k) => k;
    const paginationT = (key, opts) => {
        if (typeof window.t === 'function') return window.t(key, opts);
        if (key === 'mcp.paginationInfo' && opts) return `显示 ${opts.start}-${opts.end} / 共 ${opts.total} 个工具`;
        if (key === 'mcp.pageInfo' && opts) return `第 ${opts.page} / ${opts.total} 页`;
        return key;
    };
    pagination.innerHTML = `
        <div class="pagination-info">
            ${paginationT('mcp.paginationInfo', { start: startItem, end: endItem, total: total })}${toolsSearchKeyword ? ` (${t('common.search')}: "${escapeHtml(toolsSearchKeyword)}")` : ''}
        </div>
        <div class="pagination-page-size">
            <label for="tools-page-size-pagination">${t('mcp.perPage')}</label>
            <select id="tools-page-size-pagination" onchange="changeToolsPageSize()">
                <option value="10" ${savedPageSize === 10 ? 'selected' : ''}>10</option>
                <option value="20" ${savedPageSize === 20 ? 'selected' : ''}>20</option>
                <option value="50" ${savedPageSize === 50 ? 'selected' : ''}>50</option>
                <option value="100" ${savedPageSize === 100 ? 'selected' : ''}>100</option>
            </select>
        </div>
        <div class="pagination-controls">
            <button class="btn-secondary" onclick="loadToolsList(1, '${escapeHtml(toolsSearchKeyword)}')" ${page === 1 ? 'disabled' : ''}>${t('mcp.firstPage')}</button>
            <button class="btn-secondary" onclick="loadToolsList(${page - 1}, '${escapeHtml(toolsSearchKeyword)}')" ${page === 1 ? 'disabled' : ''}>${t('mcp.prevPage')}</button>
            <span class="pagination-page">${paginationT('mcp.pageInfo', { page: page, total: totalPages })}</span>
            <button class="btn-secondary" onclick="loadToolsList(${page + 1}, '${escapeHtml(toolsSearchKeyword)}')" ${page === totalPages ? 'disabled' : ''}>${t('mcp.nextPage')}</button>
            <button class="btn-secondary" onclick="loadToolsList(${totalPages}, '${escapeHtml(toolsSearchKeyword)}')" ${page === totalPages ? 'disabled' : ''}>${t('mcp.lastPage')}</button>
        </div>
    `;
    
    toolsList.appendChild(pagination);
}

// 处理工具checkbox状态变化
function handleToolCheckboxChange(toolKey, enabled) {
    // 更新全局状态映射
    const toolItem = document.querySelector(`.tool-item[data-tool-key="${toolKey}"]`);
    if (toolItem) {
        const toolName = toolItem.dataset.toolName;
        const isExternal = toolItem.dataset.isExternal === 'true';
        const externalMcp = toolItem.dataset.externalMcp || '';
        toolStateMap.set(toolKey, {
            enabled: enabled,
            is_external: isExternal,
            external_mcp: externalMcp,
            name: toolName // 保存原始工具名称
        });
    }
    updateToolsStats();
}

// 全选工具
function selectAllTools() {
    document.querySelectorAll('#tools-list input[type="checkbox"]').forEach(checkbox => {
        checkbox.checked = true;
        // 更新全局状态映射
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
                    name: toolName // 保存原始工具名称
                });
            }
        }
    });
    updateToolsStats();
}

// 全不选工具
function deselectAllTools() {
    document.querySelectorAll('#tools-list input[type="checkbox"]').forEach(checkbox => {
        checkbox.checked = false;
        // 更新全局状态映射
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
                    name: toolName // 保存原始工具名称
                });
            }
        }
    });
    updateToolsStats();
}

// 改变每页显示数量
async function changeToolsPageSize() {
    // 尝试从两个位置获取选择器（顶部或分页区域）
    const pageSizeSelect = document.getElementById('tools-page-size') || document.getElementById('tools-page-size-pagination');
    if (!pageSizeSelect) return;
    
    const newPageSize = parseInt(pageSizeSelect.value, 10);
    if (isNaN(newPageSize) || newPageSize < 1) {
        return;
    }
    
    // 保存到localStorage
    localStorage.setItem('toolsPageSize', newPageSize.toString());
    
    // 更新分页配置
    toolsPagination.pageSize = newPageSize;
    
    // 同步更新另一个选择器（如果存在）
    const otherSelect = document.getElementById('tools-page-size') || document.getElementById('tools-page-size-pagination');
    if (otherSelect && otherSelect !== pageSizeSelect) {
        otherSelect.value = newPageSize;
    }
    
    // 重新加载第一页
    await loadToolsList(1, toolsSearchKeyword);
}

// 更新工具统计信息
async function updateToolsStats() {
    const statsEl = document.getElementById('tools-stats');
    if (!statsEl) return;
    
    // 先保存当前页的状态到全局映射
    saveCurrentPageToolStates();
    
    // 计算当前页的启用工具数
    const currentPageEnabled = Array.from(document.querySelectorAll('#tools-list input[type="checkbox"]:checked')).length;
    const currentPageTotal = document.querySelectorAll('#tools-list input[type="checkbox"]').length;
    
    // 计算所有工具的启用数
    let totalEnabled = 0;
    let totalTools = toolsPagination.total || 0;
    
    try {
        // 如果有搜索关键词，只统计搜索结果
        if (toolsSearchKeyword) {
            totalTools = allTools.length;
            totalEnabled = allTools.filter(tool => {
                // 优先使用全局状态映射，否则使用checkbox状态，最后使用服务器返回的状态
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
            // 没有搜索时，需要获取所有工具的状态
            // 先使用全局状态映射和当前页的checkbox状态
            const localStateMap = new Map();
            
            // 从当前页的checkbox获取状态（如果全局映射中没有）
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
                        // 如果checkbox不存在（不在当前页），使用工具原始状态
                        localStateMap.set(toolKey, tool.enabled);
                    }
                }
            });
            
            // 如果总工具数大于当前页，需要获取所有工具的状态
            if (totalTools > allTools.length) {
                // 遍历所有页面获取完整状态
                let page = 1;
                let hasMore = true;
                const pageSize = 100; // 使用较大的页面大小以减少请求次数
                
                while (hasMore && page <= 10) { // 限制最多10页，避免无限循环
                    const url = `/api/config/tools?page=${page}&page_size=${pageSize}`;
                    const pageResponse = await apiFetch(url);
                    if (!pageResponse.ok) break;
                    
                    const pageResult = await pageResponse.json();
                    pageResult.tools.forEach(tool => {
                        // 优先使用全局状态映射，否则使用服务器返回的状态
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
            
            // 计算启用的工具数
            totalEnabled = Array.from(localStateMap.values()).filter(enabled => enabled).length;
        }
    } catch (error) {
        console.warn('获取工具统计失败，使用当前页数据', error);
        // 如果获取失败，使用当前页的数据
        totalTools = totalTools || currentPageTotal;
        totalEnabled = currentPageEnabled;
    }
    
    const tStats = typeof window.t === 'function' ? window.t : (k) => k;
    statsEl.innerHTML = `
        <span title="${tStats('mcp.currentPageEnabled')}">✅ ${tStats('mcp.currentPageEnabled')}: <strong>${currentPageEnabled}</strong> / ${currentPageTotal}</span>
        <span title="${tStats('mcp.totalEnabled')}">📊 ${tStats('mcp.totalEnabled')}: <strong>${totalEnabled}</strong> / ${totalTools}</span>
    `;
}

// 过滤工具（已废弃，现在使用服务端搜索）
// 保留此函数以防其他地方调用，但实际功能已由searchTools()替代
function filterTools() {
    // 不再使用客户端过滤，改为触发服务端搜索
    // 可以保留为空函数或移除oninput事件
}

// 应用设置
async function applySettings() {
    try {
        // 清除之前的验证错误状态
        document.querySelectorAll('.form-group input').forEach(input => {
            input.classList.remove('error');
        });
        
        // 验证必填字段
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
            const msg = (typeof window !== 'undefined' && typeof window.t === 'function')
                ? window.t('settings.apply.fillRequired')
                : '请填写所有必填字段（标记为 * 的字段）';
            alert(msg);
            return;
        }
        
        // 收集配置
        const knowledgeEnabledCheckbox = document.getElementById('knowledge-enabled');
        const knowledgeEnabled = knowledgeEnabledCheckbox ? knowledgeEnabledCheckbox.checked : true;
        
        // 收集知识库配置
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
                    return isNaN(val) ? 0.7 : val; // 允许0.0值，只有NaN时才使用默认值
                })()
            },
            indexing: {
                chunk_size: parseInt(document.getElementById("knowledge-indexing-chunk-size")?.value) || 512,
                chunk_overlap: parseInt(document.getElementById("knowledge-indexing-chunk-overlap")?.value) ?? 50,
                max_chunks_per_item: parseInt(document.getElementById("knowledge-indexing-max-chunks-per-item")?.value) ?? 0,
                max_rpm: parseInt(document.getElementById("knowledge-indexing-max-rpm")?.value) ?? 0,
                rate_limit_delay_ms: parseInt(document.getElementById("knowledge-indexing-rate-limit-delay-ms")?.value) ?? 300,
                max_retries: parseInt(document.getElementById("knowledge-indexing-max-retries")?.value) ?? 3,
                retry_delay_ms: parseInt(document.getElementById("knowledge-indexing-retry-delay-ms")?.value) ?? 1000
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
            multi_agent: {
                enabled: document.getElementById('multi-agent-enabled')?.checked === true,
                default_mode: document.getElementById('multi-agent-default-mode')?.value === 'multi' ? 'multi' : 'single',
                robot_use_multi_agent: document.getElementById('multi-agent-robot-use')?.checked === true,
                batch_use_multi_agent: document.getElementById('multi-agent-batch-use')?.checked === true
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
                }
            },
            tools: []
        };
        
        // 收集工具启用状态
        // 先保存当前页的状态到全局映射
        saveCurrentPageToolStates();
        
        // 获取所有工具列表以获取完整状态（遍历所有页面）
        // 注意：无论是否在搜索状态下，都要获取所有工具的状态，以确保完整保存
        try {
            const allToolsMap = new Map();
            let page = 1;
            let hasMore = true;
            const pageSize = 100; // 使用合理的页面大小
            
            // 遍历所有页面获取所有工具（不使用搜索关键词，获取全部工具）
            while (hasMore) {
                const url = `/api/config/tools?page=${page}&page_size=${pageSize}`;
                
                const pageResponse = await apiFetch(url);
                if (!pageResponse.ok) {
                    throw new Error('获取工具列表失败');
                }
                
                const pageResult = await pageResponse.json();
                
                // 将工具添加到映射中
                // 优先使用全局状态映射中的状态（用户修改过的），否则使用服务器返回的状态
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
                
                // 检查是否还有更多页面
                if (page >= pageResult.total_pages) {
                    hasMore = false;
                } else {
                    page++;
                }
            }
            
            // 将所有工具添加到配置中
            allToolsMap.forEach((tool, toolKey) => {
                config.tools.push({
                    name: tool.name,
                    enabled: tool.enabled,
                    is_external: tool.is_external,
                    external_mcp: tool.external_mcp
                });
            });
        } catch (error) {
            console.warn('获取所有工具列表失败，仅使用全局状态映射', error);
            // 如果获取失败，使用全局状态映射
            toolStateMap.forEach((toolData, toolKey) => {
                // toolData.name 保存了原始工具名称
                const toolName = toolData.name || toolKey.split('::').pop();
                config.tools.push({
                    name: toolName,
                    enabled: toolData.enabled,
                    is_external: toolData.is_external,
                    external_mcp: toolData.external_mcp
                });
            });
        }
        
        // 更新配置
        const updateResponse = await apiFetch('/api/config', {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(config)
        });
        
        if (!updateResponse.ok) {
            const error = await updateResponse.json();
            const fallback = (typeof window !== 'undefined' && typeof window.t === 'function')
                ? window.t('settings.apply.applyFailed')
                : '应用配置失败';
            throw new Error(error.error || fallback);
        }
        
        // 应用配置
        const applyResponse = await apiFetch('/api/config/apply', {
            method: 'POST'
        });
        
        if (!applyResponse.ok) {
            const error = await applyResponse.json();
            const fallback = (typeof window !== 'undefined' && typeof window.t === 'function')
                ? window.t('settings.apply.applyFailed')
                : '应用配置失败';
            throw new Error(error.error || fallback);
        }
        
        const successMsg = (typeof window !== 'undefined' && typeof window.t === 'function')
            ? window.t('settings.apply.applySuccess')
            : '配置已成功应用！';
        alert(successMsg);
        closeSettings();
    } catch (error) {
        console.error('应用配置失败:', error);
        const baseMsg = (typeof window !== 'undefined' && typeof window.t === 'function')
            ? window.t('settings.apply.applyFailed')
            : '应用配置失败';
        alert(baseMsg + ': ' + error.message);
    }
}

// 测试OpenAI连接
async function testOpenAIConnection() {
    const btn = document.getElementById('test-openai-btn');
    const resultEl = document.getElementById('test-openai-result');

    const baseUrl = document.getElementById('openai-base-url').value.trim();
    const apiKey = document.getElementById('openai-api-key').value.trim();
    const model = document.getElementById('openai-model').value.trim();

    if (!apiKey || !model) {
        resultEl.style.color = 'var(--danger-color, #e53e3e)';
        resultEl.textContent = typeof window.t === 'function' ? window.t('settingsBasic.testFillRequired') : '请先填写 API Key 和模型';
        return;
    }

    btn.style.pointerEvents = 'none';
    btn.style.opacity = '0.5';
    resultEl.style.color = 'var(--text-muted, #888)';
    resultEl.textContent = typeof window.t === 'function' ? window.t('settingsBasic.testing') : '测试中...';

    try {
        const response = await apiFetch('/api/config/test-openai', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                base_url: baseUrl,
                api_key: apiKey,
                model: model
            })
        });

        const result = await response.json();

        if (result.success) {
            resultEl.style.color = 'var(--success-color, #38a169)';
            const latency = result.latency_ms ? ` (${result.latency_ms}ms)` : '';
            const modelInfo = result.model ? ` [${result.model}]` : '';
            resultEl.textContent = (typeof window.t === 'function' ? window.t('settingsBasic.testSuccess') : '连接成功') + modelInfo + latency;
        } else {
            resultEl.style.color = 'var(--danger-color, #e53e3e)';
            resultEl.textContent = (typeof window.t === 'function' ? window.t('settingsBasic.testFailed') : '连接失败') + ': ' + (result.error || '未知错误');
        }
    } catch (error) {
        resultEl.style.color = 'var(--danger-color, #e53e3e)';
        resultEl.textContent = (typeof window.t === 'function' ? window.t('settingsBasic.testError') : '测试出错') + ': ' + error.message;
    } finally {
        btn.style.pointerEvents = '';
        btn.style.opacity = '';
    }
}

// 保存工具配置（独立函数，用于MCP管理页面）
async function saveToolsConfig() {
    try {
        // 先保存当前页的状态到全局映射
        saveCurrentPageToolStates();
        
        // 获取当前配置（只获取工具部分）
        const response = await apiFetch('/api/config');
        if (!response.ok) {
            throw new Error('获取配置失败');
        }
        
        const currentConfig = await response.json();
        
        // 构建只包含工具配置的配置对象
        const config = {
            openai: currentConfig.openai || {},
            agent: currentConfig.agent || {},
            tools: []
        };
        
        // 收集工具启用状态（与applySettings中的逻辑相同）
        try {
            const allToolsMap = new Map();
            let page = 1;
            let hasMore = true;
            const pageSize = 100;
            
            // 遍历所有页面获取所有工具
            while (hasMore) {
                const url = `/api/config/tools?page=${page}&page_size=${pageSize}`;
                
                const pageResponse = await apiFetch(url);
                if (!pageResponse.ok) {
                    throw new Error('获取工具列表失败');
                }
                
                const pageResult = await pageResponse.json();
                
                // 将工具添加到映射中
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
                
                // 检查是否还有更多页面
                if (page >= pageResult.total_pages) {
                    hasMore = false;
                } else {
                    page++;
                }
            }
            
            // 将所有工具添加到配置中
            allToolsMap.forEach((tool, toolKey) => {
                config.tools.push({
                    name: tool.name,
                    enabled: tool.enabled,
                    is_external: tool.is_external,
                    external_mcp: tool.external_mcp
                });
            });
        } catch (error) {
            console.warn('获取所有工具列表失败，仅使用全局状态映射', error);
            // 如果获取失败，使用全局状态映射
            toolStateMap.forEach((toolData, toolKey) => {
                // toolData.name 保存了原始工具名称
                const toolName = toolData.name || toolKey.split('::').pop();
                config.tools.push({
                    name: toolName,
                    enabled: toolData.enabled,
                    is_external: toolData.is_external,
                    external_mcp: toolData.external_mcp
                });
            });
        }
        
        // 更新配置
        const updateResponse = await apiFetch('/api/config', {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(config)
        });
        
        if (!updateResponse.ok) {
            const error = await updateResponse.json();
            throw new Error(error.error || '更新配置失败');
        }
        
        // 应用配置
        const applyResponse = await apiFetch('/api/config/apply', {
            method: 'POST'
        });
        
        if (!applyResponse.ok) {
            const error = await applyResponse.json();
            throw new Error(error.error || '应用配置失败');
        }
        
        alert(typeof window.t === 'function' ? window.t('mcp.toolsConfigSaved') : '工具配置已成功保存！');
        
        // 重新加载工具列表以反映最新状态
        if (typeof loadToolsList === 'function') {
            await loadToolsList(toolsPagination.page, toolsSearchKeyword);
        }
    } catch (error) {
        console.error('保存工具配置失败:', error);
        alert((typeof window.t === 'function' ? window.t('mcp.saveToolsConfigFailed') : '保存工具配置失败') + ': ' + error.message);
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
        alert(typeof window.t === 'function' ? window.t('settings.security.fillPasswordHint') : '请正确填写当前密码和新密码，新密码至少 8 位且需要两次输入一致。');
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
            throw new Error(result.error || '修改密码失败');
        }

        const pwdMsg = typeof window.t === 'function' ? window.t('settings.security.passwordUpdated') : '密码已更新，请使用新密码重新登录。';
        alert(pwdMsg);
        resetPasswordForm();
        handleUnauthorized({ message: pwdMsg, silent: false });
        closeSettings();
    } catch (error) {
        console.error('修改密码失败:', error);
        alert((typeof window.t === 'function' ? window.t('settings.security.changePasswordFailed') : '修改密码失败') + ': ' + error.message);
    } finally {
        if (submitBtn) {
            submitBtn.disabled = false;
        }
    }
}

// ==================== 外部MCP管理 ====================

let currentEditingMCPName = null;

// 拉取外部MCP列表数据（供轮询使用，返回 { servers, stats }）
async function fetchExternalMCPs() {
    const response = await apiFetch('/api/external-mcp');
    if (!response.ok) throw new Error('获取外部MCP列表失败');
    return response.json();
}

// 加载外部MCP列表并渲染
async function loadExternalMCPs() {
    try {
        const data = await fetchExternalMCPs();
        renderExternalMCPList(data.servers || {});
        renderExternalMCPStats(data.stats || {});
    } catch (error) {
        console.error('加载外部MCP列表失败:', error);
        const list = document.getElementById('external-mcp-list');
        if (list) {
            const errT = typeof window.t === 'function' ? window.t : (k) => k;
        list.innerHTML = `<div class="error">${escapeHtml(errT('mcp.loadExternalMCPFailed'))}: ${escapeHtml(error.message)}</div>`;
        }
    }
}

// 轮询列表直到指定 MCP 的工具数量已更新（每秒拉一次，拿到即停，无固定延迟）
// name 为 null 时仅按 maxAttempts 次数轮询，不判断 tool_count
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
            console.warn('轮询工具数量失败:', e);
        }
    }
    if (typeof window !== 'undefined' && typeof window.refreshMentionTools === 'function') {
        window.refreshMentionTools();
    }
}

// 渲染外部MCP列表
function renderExternalMCPList(servers) {
    const list = document.getElementById('external-mcp-list');
    if (!list) return;
    
    if (Object.keys(servers).length === 0) {
        const emptyT = typeof window.t === 'function' ? window.t : (k) => k;
        list.innerHTML = '<div class="empty">📋 ' + emptyT('mcp.noExternalMCP') + '<br><span style="font-size: 0.875rem; margin-top: 8px; display: block;">' + emptyT('mcp.clickToAddExternal') + '</span></div>';
        return;
    }
    
    let html = '<div class="external-mcp-items">';
    for (const [name, server] of Object.entries(servers)) {
        const status = server.status || 'disconnected';
        const statusClass = status === 'connected' ? 'status-connected' : 
                           status === 'connecting' ? 'status-connecting' :
                           status === 'error' ? 'status-error' :
                           status === 'disabled' ? 'status-disabled' : 'status-disconnected';
        const statusT = typeof window.t === 'function' ? window.t : (k) => k;
        const statusText = status === 'connected' ? statusT('mcp.connected') : 
                          status === 'connecting' ? statusT('mcp.connecting') :
                          status === 'error' ? statusT('mcp.connectionFailed') :
                          status === 'disabled' ? statusT('mcp.disabled') : statusT('mcp.disconnected');
        const transport = server.config.transport || (server.config.command ? 'stdio' : 'http');
        const transportIcon = transport === 'stdio' ? '⚙️' : '🌐';
        
        html += `
            <div class="external-mcp-item">
                <div class="external-mcp-item-header">
                    <div class="external-mcp-item-info">
                        <h4>${transportIcon} ${escapeHtml(name)}${server.tool_count !== undefined && server.tool_count > 0 ? `<span class="tool-count-badge" title="${escapeHtml(statusT('mcp.toolCount'))}">🔧 ${server.tool_count}</span>` : ''}</h4>
                        <span class="external-mcp-status ${statusClass}">${statusText}</span>
                    </div>
                    <div class="external-mcp-item-actions">
                        ${status === 'connected' || status === 'disconnected' || status === 'error' ? 
                            `<button class="btn-small" id="btn-toggle-${escapeHtml(name)}" onclick="toggleExternalMCP('${escapeHtml(name)}', '${status}')" title="${status === 'connected' ? statusT('mcp.stopConnection') : statusT('mcp.startConnection')}">
                                ${status === 'connected' ? '⏸ ' + statusT('mcp.stop') : '▶ ' + statusT('mcp.start')}
                            </button>` : 
                            status === 'connecting' ? 
                            `<button class="btn-small" id="btn-toggle-${escapeHtml(name)}" disabled style="opacity: 0.6; cursor: not-allowed;">
                                ⏳ ${statusT('mcp.connecting')}
                            </button>` : ''}
                        <button class="btn-small" onclick="editExternalMCP('${escapeHtml(name)}')" title="${statusT('mcp.editConfig')}" ${status === 'connecting' ? 'disabled' : ''}>✏️ ${statusT('common.edit')}</button>
                        <button class="btn-small btn-danger" onclick="deleteExternalMCP('${escapeHtml(name)}')" title="${statusT('mcp.deleteConfig')}" ${status === 'connecting' ? 'disabled' : ''}>🗑 ${statusT('common.delete')}</button>
                    </div>
                </div>
                ${status === 'error' && server.error ? `
                <div class="external-mcp-error" style="margin: 12px 0; padding: 12px; background: #fee; border-left: 3px solid #f44; border-radius: 4px; color: #c33; font-size: 0.875rem;">
                    <strong>❌ ${statusT('mcp.connectionErrorLabel')}</strong>${escapeHtml(server.error)}
                </div>` : ''}
                <div class="external-mcp-item-details">
                    <div>
                        <strong>${statusT('mcp.transportMode')}</strong>
                        <span>${transportIcon} ${escapeHtml(transport.toUpperCase())}</span>
                    </div>
                    ${server.tool_count !== undefined && server.tool_count > 0 ? `
                    <div>
                        <strong>${statusT('mcp.toolCount')}</strong>
                        <span style="font-weight: 600; color: var(--accent-color);">${statusT('mcp.toolsCountValue', { count: server.tool_count })}</span>
                    </div>` : server.tool_count === 0 && status === 'connected' ? `
                    <div>
                        <strong>${statusT('mcp.toolCount')}</strong>
                        <span style="color: var(--text-muted);">${statusT('mcp.noTools')}</span>
                    </div>` : ''}
                    ${server.config.description ? `
                    <div>
                        <strong>${statusT('mcp.description')}</strong>
                        <span>${escapeHtml(server.config.description)}</span>
                    </div>` : ''}
                    ${server.config.timeout ? `
                    <div>
                        <strong>${statusT('mcp.timeout')}</strong>
                        <span>${server.config.timeout} ${statusT('mcp.secondsUnit')}</span>
                    </div>` : ''}
                    ${transport === 'stdio' && server.config.command ? `
                    <div>
                        <strong>${statusT('mcp.command')}</strong>
                        <span style="font-family: monospace; font-size: 0.8125rem;">${escapeHtml(server.config.command)}</span>
                    </div>` : ''}
                    ${transport === 'http' && server.config.url ? `
                    <div>
                        <strong>${statusT('mcp.urlLabel')}</strong>
                        <span style="font-family: monospace; font-size: 0.8125rem; word-break: break-all;">${escapeHtml(server.config.url)}</span>
                    </div>` : ''}
                </div>
            </div>
        `;
    }
    html += '</div>';
    list.innerHTML = html;
}

// 渲染外部MCP统计信息
function renderExternalMCPStats(stats) {
    const statsEl = document.getElementById('external-mcp-stats');
    if (!statsEl) return;
    
    const total = stats.total || 0;
    const enabled = stats.enabled || 0;
    const disabled = stats.disabled || 0;
    const connected = stats.connected || 0;
    
    const statsT = typeof window.t === 'function' ? window.t : (k) => k;
    statsEl.innerHTML = `
        <span title="${statsT('mcp.totalCount')}">📊 ${statsT('mcp.totalCount')}: <strong>${total}</strong></span>
        <span title="${statsT('mcp.enabledCount')}">✅ ${statsT('mcp.enabledCount')}: <strong>${enabled}</strong></span>
        <span title="${statsT('mcp.disabledCount')}">⏸ ${statsT('mcp.disabledCount')}: <strong>${disabled}</strong></span>
        <span title="${statsT('mcp.connectedCount')}">🔗 ${statsT('mcp.connectedCount')}: <strong>${connected}</strong></span>
    `;
}

// 显示添加外部MCP模态框
function showAddExternalMCPModal() {
    currentEditingMCPName = null;
    document.getElementById('external-mcp-modal-title').textContent = (typeof window.t === 'function' ? window.t('mcp.addExternalMCP') : '添加外部MCP');
    document.getElementById('external-mcp-json').value = '';
    document.getElementById('external-mcp-json-error').style.display = 'none';
    document.getElementById('external-mcp-json-error').textContent = '';
    document.getElementById('external-mcp-json').classList.remove('error');
    document.getElementById('external-mcp-modal').style.display = 'block';
}

// 关闭外部MCP模态框
function closeExternalMCPModal() {
    document.getElementById('external-mcp-modal').style.display = 'none';
    currentEditingMCPName = null;
}

// 编辑外部MCP
async function editExternalMCP(name) {
    try {
        const response = await apiFetch(`/api/external-mcp/${encodeURIComponent(name)}`);
        if (!response.ok) {
            throw new Error(typeof window.t === 'function' ? window.t('mcp.getConfigFailed') : '获取外部MCP配置失败');
        }
        
        const server = await response.json();
        currentEditingMCPName = name;
        
        document.getElementById('external-mcp-modal-title').textContent = (typeof window.t === 'function' ? window.t('mcp.editExternalMCP') : '编辑外部MCP');
        
        // 将配置转换为对象格式（key为名称）
        const config = { ...server.config };
        // 移除tool_count、external_mcp_enable等前端字段，但保留enabled/disabled用于向后兼容
        delete config.tool_count;
        delete config.external_mcp_enable;
        
        // 包装成对象格式：{ "name": { config } }
        const configObj = {};
        configObj[name] = config;
        
        // 格式化JSON
        const jsonStr = JSON.stringify(configObj, null, 2);
        document.getElementById('external-mcp-json').value = jsonStr;
        document.getElementById('external-mcp-json-error').style.display = 'none';
        document.getElementById('external-mcp-json-error').textContent = '';
        document.getElementById('external-mcp-json').classList.remove('error');
        
        document.getElementById('external-mcp-modal').style.display = 'block';
    } catch (error) {
        console.error('编辑外部MCP失败:', error);
        alert((typeof window.t === 'function' ? window.t('mcp.operationFailed') : '编辑失败') + ': ' + error.message);
    }
}

// 格式化JSON
function formatExternalMCPJSON() {
    const jsonTextarea = document.getElementById('external-mcp-json');
    const errorDiv = document.getElementById('external-mcp-json-error');
    
    try {
        const jsonStr = jsonTextarea.value.trim();
        if (!jsonStr) {
            errorDiv.textContent = (typeof window.t === 'function' ? window.t('mcp.jsonEmpty') : 'JSON不能为空');
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
        errorDiv.textContent = (typeof window.t === 'function' ? window.t('mcp.jsonError') : 'JSON格式错误') + ': ' + error.message;
        errorDiv.style.display = 'block';
        jsonTextarea.classList.add('error');
    }
}

// 加载示例
function loadExternalMCPExample() {
    const desc = (typeof window.t === 'function' ? window.t('externalMcpModal.exampleDescription') : '示例描述');
    const example = {
        "hexstrike-ai": {
            command: "python3",
            args: [
                "/path/to/script.py",
                "--server",
                "http://example.com"
            ],
            description: desc,
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

// 保存外部MCP
async function saveExternalMCP() {
    const jsonTextarea = document.getElementById('external-mcp-json');
    const jsonStr = jsonTextarea.value.trim();
    const errorDiv = document.getElementById('external-mcp-json-error');
    
    if (!jsonStr) {
        errorDiv.textContent = (typeof window.t === 'function' ? window.t('mcp.jsonEmpty') : 'JSON不能为空');
        errorDiv.style.display = 'block';
        jsonTextarea.classList.add('error');
        jsonTextarea.focus();
        return;
    }
    
    let configObj;
    try {
        configObj = JSON.parse(jsonStr);
    } catch (error) {
        errorDiv.textContent = (typeof window.t === 'function' ? window.t('mcp.jsonError') : 'JSON格式错误') + ': ' + error.message;
        errorDiv.style.display = 'block';
        jsonTextarea.classList.add('error');
        jsonTextarea.focus();
        return;
    }
    
    const t = (typeof window.t === 'function' ? window.t : function (k, opts) { return k; });
    // 验证必须是对象格式
    if (typeof configObj !== 'object' || Array.isArray(configObj) || configObj === null) {
        errorDiv.textContent = t('mcp.configMustBeObject');
        errorDiv.style.display = 'block';
        jsonTextarea.classList.add('error');
        return;
    }
    
    // 获取所有配置名称
    const names = Object.keys(configObj);
    if (names.length === 0) {
        errorDiv.textContent = t('mcp.configNeedOne');
        errorDiv.style.display = 'block';
        jsonTextarea.classList.add('error');
        return;
    }
    
    // 验证每个配置
    for (const name of names) {
        if (!name || name.trim() === '') {
            errorDiv.textContent = t('mcp.configNameEmpty');
            errorDiv.style.display = 'block';
            jsonTextarea.classList.add('error');
            return;
        }
        
        const config = configObj[name];
        if (typeof config !== 'object' || Array.isArray(config) || config === null) {
            errorDiv.textContent = t('mcp.configMustBeObj', { name: name });
            errorDiv.style.display = 'block';
            jsonTextarea.classList.add('error');
            return;
        }
        
        // 移除 external_mcp_enable 字段（由按钮控制，但保留 enabled/disabled 用于向后兼容）
        delete config.external_mcp_enable;
        
        // 验证配置内容
        const transport = config.transport || (config.command ? 'stdio' : config.url ? 'http' : '');
        if (!transport) {
            errorDiv.textContent = t('mcp.configNeedCommand', { name: name });
            errorDiv.style.display = 'block';
            jsonTextarea.classList.add('error');
            return;
        }
        
        if (transport === 'stdio' && !config.command) {
            errorDiv.textContent = t('mcp.configStdioNeedCommand', { name: name });
            errorDiv.style.display = 'block';
            jsonTextarea.classList.add('error');
            return;
        }
        
        if (transport === 'http' && !config.url) {
            errorDiv.textContent = t('mcp.configHttpNeedUrl', { name: name });
            errorDiv.style.display = 'block';
            jsonTextarea.classList.add('error');
            return;
        }
        
        if (transport === 'sse' && !config.url) {
            errorDiv.textContent = t('mcp.configSseNeedUrl', { name: name });
            errorDiv.style.display = 'block';
            jsonTextarea.classList.add('error');
            return;
        }
    }
    
    // 清除错误提示
    errorDiv.style.display = 'none';
    jsonTextarea.classList.remove('error');
    
    try {
        // 如果是编辑模式，只更新当前编辑的配置
        if (currentEditingMCPName) {
            if (!configObj[currentEditingMCPName]) {
                errorDiv.textContent = (typeof window.t === 'function' ? window.t('mcp.configEditMustContainName', { name: currentEditingMCPName }) : '配置错误: 编辑模式下，JSON必须包含配置名称 "' + currentEditingMCPName + '"');
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
                throw new Error(error.error || '保存失败');
            }
        } else {
            // 添加模式：保存所有配置
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
                    throw new Error(`保存 "${name}" 失败: ${error.error || '未知错误'}`);
                }
            }
        }
        
        closeExternalMCPModal();
        await loadExternalMCPs();
        if (typeof window !== 'undefined' && typeof window.refreshMentionTools === 'function') {
            window.refreshMentionTools();
        }
        // 轮询几次以拉取后端异步更新的工具数量（无固定延迟，拿到即停）
        pollExternalMCPToolCount(null, 5);
        alert(typeof window.t === 'function' ? window.t('mcp.saveSuccess') : '保存成功');
    } catch (error) {
        console.error('保存外部MCP失败:', error);
        errorDiv.textContent = (typeof window.t === 'function' ? window.t('mcp.operationFailed') : '保存失败') + ': ' + error.message;
        errorDiv.style.display = 'block';
        jsonTextarea.classList.add('error');
    }
}

// 删除外部MCP
async function deleteExternalMCP(name) {
    if (!confirm((typeof window.t === 'function' ? window.t('mcp.deleteExternalConfirm', { name: name }) : `确定要删除外部MCP "${name}" 吗？`))) {
        return;
    }
    
    try {
        const response = await apiFetch(`/api/external-mcp/${encodeURIComponent(name)}`, {
            method: 'DELETE',
        });
        
        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || '删除失败');
        }
        
        await loadExternalMCPs();
        // 刷新对话界面的工具列表，移除已删除的MCP工具
        if (typeof window !== 'undefined' && typeof window.refreshMentionTools === 'function') {
            window.refreshMentionTools();
        }
        alert(typeof window.t === 'function' ? window.t('mcp.deleteSuccess') : '删除成功');
    } catch (error) {
        console.error('删除外部MCP失败:', error);
        alert((typeof window.t === 'function' ? window.t('mcp.operationFailed') : '删除失败') + ': ' + error.message);
    }
}

// 切换外部MCP启停
async function toggleExternalMCP(name, currentStatus) {
    const action = currentStatus === 'connected' ? 'stop' : 'start';
    const buttonId = `btn-toggle-${name}`;
    const button = document.getElementById(buttonId);
    
    // 如果是启动操作，显示加载状态
    if (action === 'start' && button) {
        button.disabled = true;
        button.style.opacity = '0.6';
        button.style.cursor = 'not-allowed';
        button.innerHTML = '⏳ 连接中...';
    }
    
    try {
        const response = await apiFetch(`/api/external-mcp/${encodeURIComponent(name)}/${action}`, {
            method: 'POST',
        });
        
        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || '操作失败');
        }
        
        const result = await response.json();
        
        // 如果是启动操作，先立即检查一次状态
        if (action === 'start') {
            // 立即检查一次状态（可能已经连接）
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
                        // 轮询直到该 MCP 工具数量已更新（每秒拉一次，无固定延迟）
                        pollExternalMCPToolCount(name, 10);
                        return;
                    }
                }
            } catch (error) {
                console.error('检查状态失败:', error);
            }
            
            // 如果还未连接，开始轮询
            await pollExternalMCPStatus(name, 30); // 最多轮询30次（约30秒）
        } else {
            // 停止操作，直接刷新
            await loadExternalMCPs();
            // 刷新对话界面的工具列表
            if (typeof window !== 'undefined' && typeof window.refreshMentionTools === 'function') {
                window.refreshMentionTools();
            }
        }
    } catch (error) {
        console.error('切换外部MCP状态失败:', error);
        alert((typeof window.t === 'function' ? window.t('mcp.operationFailed') : '操作失败') + ': ' + error.message);
        
        // 恢复按钮状态
        if (button) {
            button.disabled = false;
            button.style.opacity = '1';
            button.style.cursor = 'pointer';
            button.innerHTML = '▶ 启动';
        }
        
        // 刷新状态
        await loadExternalMCPs();
        // 刷新对话界面的工具列表
        if (typeof window !== 'undefined' && typeof window.refreshMentionTools === 'function') {
            window.refreshMentionTools();
        }
    }
}

// 轮询外部MCP状态
async function pollExternalMCPStatus(name, maxAttempts = 30) {
    let attempts = 0;
    const pollInterval = 1000; // 1秒轮询一次
    
    while (attempts < maxAttempts) {
        await new Promise(resolve => setTimeout(resolve, pollInterval));
        
        try {
            const response = await apiFetch(`/api/external-mcp/${encodeURIComponent(name)}`);
            if (response.ok) {
                const data = await response.json();
                const status = data.status || 'disconnected';
                
                // 更新按钮状态
                const buttonId = `btn-toggle-${name}`;
                const button = document.getElementById(buttonId);
                
                if (status === 'connected') {
                    await loadExternalMCPs();
                    if (typeof window !== 'undefined' && typeof window.refreshMentionTools === 'function') {
                        window.refreshMentionTools();
                    }
                    // 轮询直到该 MCP 工具数量已更新（每秒拉一次，无固定延迟）
                    pollExternalMCPToolCount(name, 10);
                    return;
                } else if (status === 'error' || status === 'disconnected') {
                    // 连接失败，刷新列表并显示错误
                    await loadExternalMCPs();
                    // 刷新对话界面的工具列表
                    if (typeof window !== 'undefined' && typeof window.refreshMentionTools === 'function') {
                        window.refreshMentionTools();
                    }
                    if (status === 'error') {
                        alert(typeof window.t === 'function' ? window.t('mcp.connectionFailedCheck') : '连接失败，请检查配置和网络连接');
                    }
                    return;
                } else if (status === 'connecting') {
                    // 仍在连接中，继续轮询
                    attempts++;
                    continue;
                }
            }
        } catch (error) {
            console.error('轮询状态失败:', error);
        }
        
        attempts++;
    }
    
    // 超时，刷新列表
    await loadExternalMCPs();
    // 刷新对话界面的工具列表
    if (typeof window !== 'undefined' && typeof window.refreshMentionTools === 'function') {
        window.refreshMentionTools();
    }
    alert(typeof window.t === 'function' ? window.t('mcp.connectionTimeout') : '连接超时，请检查配置和网络连接');
}

// 在打开设置时加载外部MCP列表
const originalOpenSettings = openSettings;
openSettings = async function() {
    await originalOpenSettings();
    await loadExternalMCPs();
};

// 语言切换后重新渲染 MCP 管理页中由 JS 写入的区块（innerHTML 不会随 data-i18n 自动更新）
document.addEventListener('languagechange', function () {
    try {
        const mcpPage = document.getElementById('page-mcp-management');
        if (mcpPage && mcpPage.classList.contains('active')) {
            if (typeof loadExternalMCPs === 'function') {
                loadExternalMCPs().catch(function () { /* ignore */ });
            }
            if (typeof updateToolsStats === 'function') {
                updateToolsStats().catch(function () { /* ignore */ });
            }
        }
    } catch (e) {
        console.warn('languagechange MCP refresh failed', e);
    }
});
