// settingsrelatedfunction
let currentConfig = null;
let allTools = [];
// globaltool statemapping,used forSaveuseatallpage's modify
// key: uniquetoolidentifier(toolKey),value: { enabled: boolean, is_external: boolean, external_mcp: string }
let toolStateMap = new Map();

// generatetool's uniqueidentifier,used fordistinguishsame namebutSourcedifferent's tool
function getToolKey(tool) {
    // if it is an external tool,use external_mcp::tool.name asuniqueidentifier
 // if it iswithintool,use tool.name asidentifier
    if (tool.is_external && tool.external_mcp) {
        return `${tool.external_mcp}::${tool.name}`;
    }
    return tool.name;
}
// fromlocalStoragereadPer pagecount,defaults to20
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

// switchsettingscategory
function switchSettingsSection(section) {
    // updatenavigationitemstatus
    document.querySelectorAll('.settings-nav-item').forEach(item => {
        item.classList.remove('active');
    });
    const activeNavItem = document.querySelector(`.settings-nav-item[data-section="${section}"]`);
    if (activeNavItem) {
        activeNavItem.classList.add('active');
    }
    
    // updatecontentareaShow 
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
    if (section === 'plugins') {
        loadPlugins();
    }
}

// opensettings
async function openSettings() {
    // switchtosettingspage
    if (typeof switchPage === 'function') {
        switchPage('settings');
    }
    
    // eachtimeopenwhenclearglobalstatusmapping,re-loadlatestconfiguration
    toolStateMap.clear();
    
    // eachtimeopenwhenre-loadlatestconfiguration(systemsettingspagenotneedloadtoollist)
    await loadConfig(false);
    
 // clearbefore's validateerrorstatus
    document.querySelectorAll('.form-group input').forEach(input => {
        input.classList.remove('error');
    });
    
 // defaultShow settings
    switchSettingsSection('basic');
}

// Closesettings(preservefunctiontocompatibleoldcode,butatnotneedClosefunction)
function closeSettings() {
 // no longerneedClosefunction,becauseatispagenotismodal
 // ifneed,cantoswitchconversationpage
    if (typeof switchPage === 'function') {
        switchPage('chat');
    }
}

// click outside modal to close(onlypreserveMCPdetailsmodal)
window.onclick = function(event) {
    const mcpModal = document.getElementById('mcp-detail-modal');
    
    if (event.target === mcpModal) {
        closeMCPDetail();
    }
}

// loadconfiguration
async function loadConfig(loadTools = true) {
    try {
        const response = await apiFetch('/api/config');
        if (!response.ok) {
            throw new Error('Failed to get configuration');
        }
        
        currentConfig = await response.json();
        
        // populateOpenAIconfiguration
        document.getElementById('openai-api-key').value = currentConfig.openai.api_key || '';
        document.getElementById('openai-base-url').value = currentConfig.openai.base_url || '';
        // Set model in select dropdown (add as option if not present)
        const modelVal = currentConfig.openai.model || '';
        const modelSelect = document.getElementById('openai-model');
        const customInput = document.getElementById('openai-model-custom');
        if (modelVal) {
            // Check if the option already exists
            const exists = [...modelSelect.options].some(o => o.value === modelVal);
            if (!exists) {
                // Clear default placeholder and add the saved model
                modelSelect.innerHTML = '';
                const opt = document.createElement('option');
                opt.value = modelVal;
                opt.textContent = modelVal;
                modelSelect.appendChild(opt);
                const customOpt = document.createElement('option');
                customOpt.value = '__custom__';
                customOpt.textContent = '-- Custom model ID --';
                modelSelect.appendChild(customOpt);
            }
            modelSelect.value = modelVal;
            if (customInput) customInput.style.display = 'none';
        }

        // Set provider dropdown
        const providerSelect = document.getElementById('openai-provider');
        if (providerSelect && currentConfig.openai.provider) {
            providerSelect.value = currentConfig.openai.provider;
        }

        // Populate tool model fields
        const toolModelEl = document.getElementById('openai-tool-model');
        if (toolModelEl) toolModelEl.value = currentConfig.openai?.tool_model || '';
        const toolBaseUrlEl = document.getElementById('openai-tool-base-url');
        if (toolBaseUrlEl) toolBaseUrlEl.value = currentConfig.openai?.tool_base_url || '';
        const toolApiKeyEl = document.getElementById('openai-tool-api-key');
        if (toolApiKeyEl) toolApiKeyEl.value = currentConfig.openai?.tool_api_key || '';
        const toolTempEl = document.getElementById('openai-tool-temperature');
        if (toolTempEl) toolTempEl.value = currentConfig.openai?.tool_temperature || 0;
        const toolTopPEl = document.getElementById('openai-tool-top-p');
        if (toolTopPEl) toolTopPEl.value = currentConfig.openai?.tool_top_p || 0;

        // Populate summary model fields
        const summaryModelEl = document.getElementById('openai-summary-model');
        if (summaryModelEl) summaryModelEl.value = currentConfig.openai?.summary_model || '';
        const summaryBaseUrlEl = document.getElementById('openai-summary-base-url');
        if (summaryBaseUrlEl) summaryBaseUrlEl.value = currentConfig.openai?.summary_base_url || '';
        const summaryApiKeyEl = document.getElementById('openai-summary-api-key');
        if (summaryApiKeyEl) summaryApiKeyEl.value = currentConfig.openai?.summary_api_key || '';
        const summaryTempEl = document.getElementById('openai-summary-temperature');
        if (summaryTempEl) summaryTempEl.value = currentConfig.openai?.summary_temperature || 0;
        const summaryTopPEl = document.getElementById('openai-summary-top-p');
        if (summaryTopPEl) summaryTopPEl.value = currentConfig.openai?.summary_top_p || 0;

        // Populate sampling parameters
        const tempEl = document.getElementById('openai-temperature');
        if (tempEl) tempEl.value = currentConfig.openai?.temperature || 0;
        const topPEl = document.getElementById('openai-top-p');
        if (topPEl) topPEl.value = currentConfig.openai?.top_p || 0;
        const topKEl = document.getElementById('openai-top-k');
        if (topKEl) topKEl.value = currentConfig.openai?.top_k || 0;

        // populateFOFAconfiguration
        const fofa = currentConfig.fofa || {};
        const fofaEmailEl = document.getElementById('fofa-email');
        const fofaKeyEl = document.getElementById('fofa-api-key');
        const fofaBaseUrlEl = document.getElementById('fofa-base-url');
        if (fofaEmailEl) fofaEmailEl.value = fofa.email || '';
        if (fofaKeyEl) fofaKeyEl.value = fofa.api_key || '';
        if (fofaBaseUrlEl) fofaBaseUrlEl.value = fofa.base_url || '';
        
        // populateAgentconfiguration
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
        
        // populateknowledge baseconfiguration
        const knowledgeEnabledCheckbox = document.getElementById('knowledge-enabled');
        if (knowledgeEnabledCheckbox) {
            knowledgeEnabledCheckbox.checked = currentConfig.knowledge?.enabled !== false;
        }
        
        // populateknowledge basedetailedconfiguration
        if (currentConfig.knowledge) {
            const knowledge = currentConfig.knowledge;
            
 // configuration
            const basePathInput = document.getElementById('knowledge-base-path');
            if (basePathInput) {
                basePathInput.value = knowledge.base_path || 'knowledge_base';
            }
            
            // embeddingmodelconfiguration
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
            
            // Retrieveconfiguration
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
 // allow0.0,onlyhasundefined/nullwhenonly usedefault
                retrievalWeightInput.value = (hybridWeight !== undefined && hybridWeight !== null) ? hybridWeight : 0.7;
            }

            // indexconfiguration
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

        // populate Telegram bot configuration
        const robots = currentConfig.robots || {};
        const telegram = robots.telegram || {};
        const tgEnabled = document.getElementById('robot-telegram-enabled');
        if (tgEnabled) tgEnabled.checked = telegram.enabled === true;
        const tgToken = document.getElementById('robot-telegram-bot-token');
        if (tgToken) tgToken.value = telegram.bot_token || '';
        const tgAllowed = document.getElementById('robot-telegram-allowed-users');
        if (tgAllowed) tgAllowed.value = (telegram.allowed_user_ids || []).join(',');

        // Proxy settings
        const proxy = currentConfig.agent?.proxy || {};
        const proxyEnabled = document.getElementById('proxy-enabled');
        if (proxyEnabled) proxyEnabled.checked = proxy.enabled || false;
        const proxyType = document.getElementById('proxy-type');
        if (proxyType) proxyType.value = proxy.type || 'tor';
        const proxyHost = document.getElementById('proxy-host');
        if (proxyHost) proxyHost.value = proxy.host || '127.0.0.1';
        const proxyPort = document.getElementById('proxy-port');
        if (proxyPort) proxyPort.value = proxy.port || 9050;
        const proxyUsername = document.getElementById('proxy-username');
        if (proxyUsername) proxyUsername.value = proxy.username || '';
        const proxyPassword = document.getElementById('proxy-password');
        if (proxyPassword) proxyPassword.value = proxy.password || '';
        const proxyNoProxy = document.getElementById('proxy-no-proxy');
        if (proxyNoProxy) proxyNoProxy.value = proxy.no_proxy || 'localhost,127.0.0.1,*.local';
        const proxyProxychains = document.getElementById('proxy-proxychains');
        if (proxyProxychains) proxyProxychains.checked = proxy.proxychains || false;
        const proxyDns = document.getElementById('proxy-dns');
        if (proxyDns) proxyDns.checked = proxy.dns_proxy !== undefined ? proxy.dns_proxy : true;
        const proxyTorAutostart = document.getElementById('proxy-tor-autostart');
        if (proxyTorAutostart) proxyTorAutostart.checked = proxy.tor_auto_start || false;
        const proxyHealthCheck = document.getElementById('proxy-health-check');
        if (proxyHealthCheck) proxyHealthCheck.checked = proxy.health_check !== undefined ? proxy.health_check : true;

        // Auto-set port when proxy type changes
        document.getElementById('proxy-type')?.addEventListener('change', function() {
            const portEl = document.getElementById('proxy-port');
            if (!portEl) return;
            switch(this.value) {
                case 'tor': portEl.value = '9050'; break;
                case 'socks5': case 'socks5h': portEl.value = '1080'; break;
                case 'http': case 'https': portEl.value = '8080'; break;
            }
        });

        // onlyhasatneedwhenonly loadtoollist(MCPmanagement pageneed,systemsettingspagenotneed)
        if (loadTools) {
            // settingsPer pagecount(willatpaginationcontrolsrenderwhensettings)
            const savedPageSize = getToolsPageSize();
            toolsPagination.pageSize = savedPageSize;
            
            // loadtoollist(usepagination)
            toolsSearchKeyword = '';
            await loadToolsList(1, '');
        }

        // Pre-load plugin list for settings plugins section
        loadPlugins();
    } catch (error) {
        console.error('Failed to load configuration:', error);
        const baseMsg = (typeof window !== 'undefined' && typeof window.t === 'function')
            ? window.t('settings.apply.loadFailed')
            : 'Failed to load configuration';
        alert(baseMsg + ': ' + error.message);
    }
}

// tool searchkeyword
let toolsSearchKeyword = '';

// loadtoollist(pagination)
async function loadToolsList(page = 1, searchKeyword = '') {
    const toolsList = document.getElementById('tools-list');
    
    // show loading state
    if (toolsList) {
 // clearcontainer,includepossiblyexist's paginationcontrols
        toolsList.innerHTML = '<div class="tools-list-items"><div class="loading" style="padding: 20px; text-align: center; color: var(--text-muted);">⏳ ' + (typeof window.t === 'function' ? window.t('mcp.loadingTools') : 'Loading tool list...') + '</div></div>';
    }
    
    try {
 // atloadnewpagebefore,save current page state to global mapping first
        saveCurrentPageToolStates();
        
        const pageSize = toolsPagination.pageSize;
        let url = `/api/config/tools?page=${page}&page_size=${pageSize}`;
        if (searchKeyword) {
            url += `&search=${encodeURIComponent(searchKeyword)}`;
        }
        
 // useshort's exceedwhentime(10second),avoidlongtimewait
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
        
 // initializetool statemapping(iftoolnotatmappingin,useservicereturn's status)
        allTools.forEach(tool => {
            const toolKey = getToolKey(tool);
            if (!toolStateMap.has(toolKey)) {
                toolStateMap.set(toolKey, {
                    enabled: tool.enabled,
                    is_external: tool.is_external || false,
                    external_mcp: tool.external_mcp || '',
                    name: tool.name // save original tool name
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
 ? (typeof window.t === 'function' ? window.t('mcp.loadToolsTimeout') : 'Tool list loading timed out, possibly due to externalMCPconnectionslow.pleaseclick"Refresh"buttonRetry,orcheckExternalMCPconnectionstatus.')
                : (typeof window.t === 'function' ? window.t('mcp.loadToolsFailed') : 'Failed to load tool list') + ': ' + escapeHtml(error.message);
            toolsList.innerHTML = `<div class="error" style="padding: 20px; text-align: center;">${errorMsg}</div>`;
        }
    }
}

// Savecurrentpage's tool statetoglobalmapping
function saveCurrentPageToolStates() {
    document.querySelectorAll('#tools-list .tool-item').forEach(item => {
        const checkbox = item.querySelector('input[type="checkbox"]');
        const toolKey = item.dataset.toolKey; // useuniqueidentifier
        const toolName = item.dataset.toolName;
        const isExternal = item.dataset.isExternal === 'true';
        const externalMcp = item.dataset.externalMcp || '';
        if (toolKey && checkbox) {
            toolStateMap.set(toolKey, {
                enabled: checkbox.checked,
                is_external: isExternal,
                external_mcp: externalMcp,
                name: toolName // save original tool name
            });
        }
    });
}

// searchtool
function searchTools() {
    const searchInput = document.getElementById('tools-search');
    const keyword = searchInput ? searchInput.value.trim() : '';
    toolsSearchKeyword = keyword;
    // searchwhenreset to first page
    loadToolsList(1, keyword);
}

// clear search
function clearSearch() {
    const searchInput = document.getElementById('tools-search');
    if (searchInput) {
        searchInput.value = '';
    }
    toolsSearchKeyword = '';
    loadToolsList(1, '');
}

// processsearchenter keyevent
function handleSearchKeyPress(event) {
    if (event.key === 'Enter') {
        searchTools();
    }
}

// rendertoollist
function renderToolsList() {
    const toolsList = document.getElementById('tools-list');
    if (!toolsList) return;
    
    // removepossiblyexist's paginationcontrols(willat renderToolsPagination inre-add)
    const oldPagination = toolsList.querySelector('.tools-pagination');
    if (oldPagination) {
        oldPagination.remove();
    }
    
    // getorcreatelistcontainer
    let listContainer = toolsList.querySelector('.tools-list-items');
    if (!listContainer) {
        listContainer = document.createElement('div');
        listContainer.className = 'tools-list-items';
        toolsList.appendChild(listContainer);
    }
    
    // clearlistcontainercontent(removeloadhint)
    listContainer.innerHTML = '';
    
    if (allTools.length === 0) {
        listContainer.innerHTML = '<div class="empty">' + (typeof window.t === 'function' ? window.t('mcp.noTools') : 'No tools') + '</div>';
        if (!toolsList.contains(listContainer)) {
            toolsList.appendChild(listContainer);
        }
        // updatestatistics
        updateToolsStats();
        return;
    }
    
    allTools.forEach(tool => {
        const toolKey = getToolKey(tool); // generateuniqueidentifier
        const toolItem = document.createElement('div');
        toolItem.className = 'tool-item';
        toolItem.dataset.toolKey = toolKey; // Saveuniqueidentifier
        toolItem.dataset.toolName = tool.name; // save original tool name
        toolItem.dataset.isExternal = tool.is_external ? 'true' : 'false';
        toolItem.dataset.externalMcp = tool.external_mcp || '';
        
 // fromglobalstatusmappinggettool state,ifnotexistthenuseservicereturn's status
        const toolState = toolStateMap.get(toolKey) || {
            enabled: tool.enabled,
            is_external: tool.is_external || false,
            external_mcp: tool.external_mcp || ''
        };
        
        // Externaltooltab,Show SourceInfo
        let externalBadge = '';
        if (toolState.is_external || tool.is_external) {
            const externalMcpName = toolState.external_mcp || tool.external_mcp || '';
            const badgeText = externalMcpName ? (typeof window.t === 'function' ? window.t('mcp.externalFrom', { name: escapeHtml(externalMcpName) }) : `External (${escapeHtml(externalMcpName)})`) : (typeof window.t === 'function' ? window.t('mcp.externalBadge') : 'External');
            const badgeTitle = externalMcpName ? (typeof window.t === 'function' ? window.t('mcp.externalToolFrom', { name: escapeHtml(externalMcpName) }) : `ExternalMCPtool - Source:${escapeHtml(externalMcpName)}`) : (typeof window.t === 'function' ? window.t('mcp.externalBadge') : 'ExternalMCPtool');
            externalBadge = `<span class="external-tool-badge" title="${badgeTitle}">${badgeText}</span>`;
        }
        
        // generateunique's checkbox id,usetooluniqueidentifier
        const checkboxId = `tool-${escapeHtml(toolKey).replace(/::/g, '--')}`;
        
        toolItem.innerHTML = `
            <input type="checkbox" id="${checkboxId}" ${toolState.enabled ? 'checked' : ''} ${toolState.is_external || tool.is_external ? 'data-external="true"' : ''} onchange="handleToolCheckboxChange('${escapeHtml(toolKey)}', this.checked)" />
            <div class="tool-item-info">
                <div class="tool-item-name">
                    ${escapeHtml(tool.name)}
                    ${externalBadge}
                </div>
                <div class="tool-item-desc">${escapeHtml(tool.description || (typeof window.t === 'function' ? window.t('mcp.noDescription') : 'No description'))}</div>
            </div>
        `;
        listContainer.appendChild(toolItem);
    });
    
    if (!toolsList.contains(listContainer)) {
        toolsList.appendChild(listContainer);
    }
    
    // updatestatistics
    updateToolsStats();
}

// rendertoollistpaginationcontrols
function renderToolsPagination() {
    const toolsList = document.getElementById('tools-list');
    if (!toolsList) return;
    
    // remove old pagination controls
    const oldPagination = toolsList.querySelector('.tools-pagination');
    if (oldPagination) {
        oldPagination.remove();
    }
    
 // ifonlyhaspageorhasdata,hide pagination
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
        if (key === 'mcp.paginationInfo' && opts) return `Show  ${opts.start}-${opts.end} / total ${opts.total} tools`;
        if (key === 'mcp.pageInfo' && opts) return ` ${opts.page} / ${opts.total} page`;
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

// processtoolcheckboxstatuschange
function handleToolCheckboxChange(toolKey, enabled) {
    // update global state mapping
    const toolItem = document.querySelector(`.tool-item[data-tool-key="${toolKey}"]`);
    if (toolItem) {
        const toolName = toolItem.dataset.toolName;
        const isExternal = toolItem.dataset.isExternal === 'true';
        const externalMcp = toolItem.dataset.externalMcp || '';
        toolStateMap.set(toolKey, {
            enabled: enabled,
            is_external: isExternal,
            external_mcp: externalMcp,
            name: toolName // save original tool name
        });
    }
    updateToolsStats();
}

// select alltool
function selectAllTools() {
    document.querySelectorAll('#tools-list input[type="checkbox"]').forEach(checkbox => {
        checkbox.checked = true;
        // update global state mapping
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
                    name: toolName // save original tool name
                });
            }
        }
    });
    updateToolsStats();
}

// nottool
function deselectAllTools() {
    document.querySelectorAll('#tools-list input[type="checkbox"]').forEach(checkbox => {
        checkbox.checked = false;
        // update global state mapping
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
                    name: toolName // save original tool name
                });
            }
        }
    });
    updateToolsStats();
}

// change items per page
async function changeToolsPageSize() {
 // tryfrompositiongetselector(toporpaginationarea)
    const pageSizeSelect = document.getElementById('tools-page-size') || document.getElementById('tools-page-size-pagination');
    if (!pageSizeSelect) return;
    
    const newPageSize = parseInt(pageSizeSelect.value, 10);
    if (isNaN(newPageSize) || newPageSize < 1) {
        return;
    }
    
    // save tolocalStorage
    localStorage.setItem('toolsPageSize', newPageSize.toString());
    
    // updatepaginationconfiguration
    toolsPagination.pageSize = newPageSize;
    
 // syncupdateselector(if exists)
    const otherSelect = document.getElementById('tools-page-size') || document.getElementById('tools-page-size-pagination');
    if (otherSelect && otherSelect !== pageSizeSelect) {
        otherSelect.value = newPageSize;
    }
    
 // re-loadpage
    await loadToolsList(1, toolsSearchKeyword);
}

// updatetoolstatisticsInfo
async function updateToolsStats() {
    const statsEl = document.getElementById('tools-stats');
    if (!statsEl) return;
    
    // save current page state to global mapping first
    saveCurrentPageToolStates();
    
 // calculatecurrentpage's enabletool
    const currentPageEnabled = Array.from(document.querySelectorAll('#tools-list input[type="checkbox"]:checked')).length;
    const currentPageTotal = document.querySelectorAll('#tools-list input[type="checkbox"]').length;
    
 // calculatealltool's enable
    let totalEnabled = 0;
    let totalTools = toolsPagination.total || 0;
    
    try {
        // ifhassearchkeyword,onlystatisticssearchresult
        if (toolsSearchKeyword) {
            totalTools = allTools.length;
            totalEnabled = allTools.filter(tool => {
 // preferuseglobalstatusmapping,otherwise usecheckboxstatus,afteruseservicereturn's status
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
 // hassearchwhen,needgetalltool's status
            // firstuseglobalstatusmappingandcurrentpage's checkboxstatus
            const localStateMap = new Map();
            
 // fromcurrentpage's checkboxgetstatus(ifglobalmappinginhas)
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
                        // ifcheckboxnotexist(notatcurrentpage),usetooloriginalstatus
                        localStateMap.set(toolKey, tool.enabled);
                    }
                }
            });
            
 // iftoolgreater thancurrentpage,needgetalltool's status
            if (totalTools > allTools.length) {
                // iterateallpagegetcompletestatus
                let page = 1;
                let hasMore = true;
 const pageSize = 100; // usebig's pageSizetoreducefewrequesttime
                
 while (hasMore && page <= 10) { // limitat most10page,avoidnoloop
                    const url = `/api/config/tools?page=${page}&page_size=${pageSize}`;
                    const pageResponse = await apiFetch(url);
                    if (!pageResponse.ok) break;
                    
                    const pageResult = await pageResponse.json();
                    pageResult.tools.forEach(tool => {
 // preferuseglobalstatusmapping,otherwise useservicereturn's status
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
            
 // calculateenable's tool
            totalEnabled = Array.from(localStateMap.values()).filter(enabled => enabled).length;
        }
    } catch (error) {
        console.warn('Failed to get tool statistics,using current page data', error);
        // if retrieval fails,usecurrentpage's data
        totalTools = totalTools || currentPageTotal;
        totalEnabled = currentPageEnabled;
    }
    
    const tStats = typeof window.t === 'function' ? window.t : (k) => k;
    statsEl.innerHTML = `
        <span title="${tStats('mcp.currentPageEnabled')}">✅ ${tStats('mcp.currentPageEnabled')}: <strong>${currentPageEnabled}</strong> / ${currentPageTotal}</span>
        <span title="${tStats('mcp.totalEnabled')}">📊 ${tStats('mcp.totalEnabled')}: <strong>${totalEnabled}</strong> / ${totalTools}</span>
    `;
}

// filtertool(already,atuseservicesearch)
// preservethisfunctiontoelsewherecall,butactualfunctionalreadysearchTools()
function filterTools() {
 // no longeruseClientfilter,astriggerservicesearch
    // cantopreserveasemptyfunctionorremoveoninputevent
}

// shouldusesettings
async function applySettings() {
    try {
 // clearbefore's validateerrorstatus
        document.querySelectorAll('.form-group input').forEach(input => {
            input.classList.remove('error');
        });
        
        // validateRequiredfield
        const apiKey = document.getElementById('openai-api-key').value.trim();
        const baseUrl = document.getElementById('openai-base-url').value.trim();
        const model = getSelectedModel();
        
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
                : 'Please fill in all required fields (marked with * )';
            alert(msg);
            return;
        }
        
        // collectconfiguration
        const knowledgeEnabledCheckbox = document.getElementById('knowledge-enabled');
        const knowledgeEnabled = knowledgeEnabledCheckbox ? knowledgeEnabledCheckbox.checked : true;
        
        // collectknowledge baseconfiguration
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
 return isNaN(val) ? 0.7 : val; // allow0.0,onlyhasNaNwhenonly usedefault
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
        
        // Parse Telegram allowed user IDs from comma-separated string
        const tgAllowedRaw = document.getElementById('robot-telegram-allowed-users')?.value.trim() || '';
        const tgAllowedIds = tgAllowedRaw ? tgAllowedRaw.split(',').map(s => parseInt(s.trim(), 10)).filter(n => !isNaN(n) && n > 0) : [];
        const provider = document.getElementById('openai-provider').value || 'openai';
        const config = {
            openai: {
                provider: provider,
                api_key: apiKey,
                base_url: baseUrl,
                model: model,
                tool_model: document.getElementById('openai-tool-model')?.value.trim() || '',
                tool_base_url: document.getElementById('openai-tool-base-url')?.value.trim() || '',
                tool_api_key: document.getElementById('openai-tool-api-key')?.value.trim() || '',
                summary_model: document.getElementById('openai-summary-model')?.value.trim() || '',
                summary_base_url: document.getElementById('openai-summary-base-url')?.value.trim() || '',
                summary_api_key: document.getElementById('openai-summary-api-key')?.value.trim() || '',
                tool_temperature: parseFloat(document.getElementById('openai-tool-temperature')?.value) || 0,
                tool_top_p: parseFloat(document.getElementById('openai-tool-top-p')?.value) || 0,
                summary_temperature: parseFloat(document.getElementById('openai-summary-temperature')?.value) || 0,
                summary_top_p: parseFloat(document.getElementById('openai-summary-top-p')?.value) || 0,
                temperature: parseFloat(document.getElementById('openai-temperature')?.value) || 0,
                top_p: parseFloat(document.getElementById('openai-top-p')?.value) || 0,
                top_k: parseInt(document.getElementById('openai-top-k')?.value) || 0
            },
            fofa: {
                email: document.getElementById('fofa-email')?.value.trim() || '',
                api_key: document.getElementById('fofa-api-key')?.value.trim() || '',
                base_url: document.getElementById('fofa-base-url')?.value.trim() || ''
            },
            agent: {
                max_iterations: parseInt(document.getElementById('agent-max-iterations').value) || 30,
                proxy: {
                    enabled: document.getElementById('proxy-enabled')?.checked || false,
                    type: document.getElementById('proxy-type')?.value || 'tor',
                    host: document.getElementById('proxy-host')?.value?.trim() || '127.0.0.1',
                    port: parseInt(document.getElementById('proxy-port')?.value) || 9050,
                    username: document.getElementById('proxy-username')?.value?.trim() || '',
                    password: document.getElementById('proxy-password')?.value?.trim() || '',
                    no_proxy: document.getElementById('proxy-no-proxy')?.value?.trim() || 'localhost,127.0.0.1',
                    proxychains: document.getElementById('proxy-proxychains')?.checked || false,
                    dns_proxy: document.getElementById('proxy-dns')?.checked || false,
                    tor_auto_start: document.getElementById('proxy-tor-autostart')?.checked || false,
                    health_check: document.getElementById('proxy-health-check')?.checked || false
                }
            },
            multi_agent: {
                enabled: document.getElementById('multi-agent-enabled')?.checked === true,
                default_mode: document.getElementById('multi-agent-default-mode')?.value === 'multi' ? 'multi' : 'single',
                robot_use_multi_agent: document.getElementById('multi-agent-robot-use')?.checked === true,
                batch_use_multi_agent: document.getElementById('multi-agent-batch-use')?.checked === true
            },
            knowledge: knowledgeConfig,
            robots: {
                telegram: {
                    enabled: document.getElementById('robot-telegram-enabled')?.checked === true,
                    bot_token: document.getElementById('robot-telegram-bot-token')?.value.trim() || '',
                    allowed_user_ids: tgAllowedIds
                }
            },
            tools: []
        };
        
        // collecttoolenabled state
        // save current page state to global mapping first
        saveCurrentPageToolStates();
        
        // getalltoollisttogetcompletestatus(iterateallpage)
 // note:noisatsearchstatusunder,all needgetalltool's status,toensurecompleteSave
        try {
            const allToolsMap = new Map();
            let page = 1;
            let hasMore = true;
            const pageSize = 100; // usereasonable's pageSize
            
            // iterate all pages to get all tools(notusesearchkeyword,getAlltool)
            while (hasMore) {
                const url = `/api/config/tools?page=${page}&page_size=${pageSize}`;
                
                const pageResponse = await apiFetch(url);
                if (!pageResponse.ok) {
                    throw new Error('Failed to get tool list');
                }
                
                const pageResult = await pageResponse.json();
                
                // willtooladdtomappingin
 // preferuseglobalstatusmappingin's status(usemodifythrough's ),otherwise useservicereturn's status
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
                
                // check if there are more pages
                if (page >= pageResult.total_pages) {
                    hasMore = false;
                } else {
                    page++;
                }
            }
            
            // willalltooladdtoconfigurationin
            allToolsMap.forEach((tool, toolKey) => {
                config.tools.push({
                    name: tool.name,
                    enabled: tool.enabled,
                    is_external: tool.is_external,
                    external_mcp: tool.external_mcp
                });
            });
        } catch (error) {
            console.warn('Failed to get all tool list,using global state mapping only', error);
            // if retrieval fails,useglobalstatusmapping
            toolStateMap.forEach((toolData, toolKey) => {
                // toolData.name Saveoriginaltool name
                const toolName = toolData.name || toolKey.split('::').pop();
                config.tools.push({
                    name: toolName,
                    enabled: toolData.enabled,
                    is_external: toolData.is_external,
                    external_mcp: toolData.external_mcp
                });
            });
        }
        
        // updateconfiguration
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
                : 'Failed to apply configuration';
            throw new Error(error.error || fallback);
        }
        
        // shoulduseconfiguration
        const applyResponse = await apiFetch('/api/config/apply', {
            method: 'POST'
        });
        
        if (!applyResponse.ok) {
            const error = await applyResponse.json();
            const fallback = (typeof window !== 'undefined' && typeof window.t === 'function')
                ? window.t('settings.apply.applyFailed')
                : 'Failed to apply configuration';
            throw new Error(error.error || fallback);
        }
        
        const successMsg = (typeof window !== 'undefined' && typeof window.t === 'function')
            ? window.t('settings.apply.applySuccess')
            : 'Configuration applied successfully!';
        alert(successMsg);
        closeSettings();
    } catch (error) {
        console.error('Failed to apply configuration:', error);
        const baseMsg = (typeof window !== 'undefined' && typeof window.t === 'function')
            ? window.t('settings.apply.applyFailed')
            : 'Failed to apply configuration';
        alert(baseMsg + ': ' + error.message);
    }
}

// Savetoolconfiguration(standalonefunction,used forMCPmanagement page)
async function saveToolsConfig() {
    try {
        // save current page state to global mapping first
        saveCurrentPageToolStates();
        
 // getcurrentconfiguration(onlygettoolminute)
        const response = await apiFetch('/api/config');
        if (!response.ok) {
            throw new Error('Failed to get configuration');
        }
        
        const currentConfig = await response.json();
        
        // buildonlycontaintoolconfiguration's configurationobject
        const config = {
            openai: currentConfig.openai || {},
            agent: currentConfig.agent || {},
            tools: []
        };
        
 // collecttoolenabled state(andapplySettingsin's same)
        try {
            const allToolsMap = new Map();
            let page = 1;
            let hasMore = true;
            const pageSize = 100;
            
            // iterate all pages to get all tools
            while (hasMore) {
                const url = `/api/config/tools?page=${page}&page_size=${pageSize}`;
                
                const pageResponse = await apiFetch(url);
                if (!pageResponse.ok) {
                    throw new Error('Failed to get tool list');
                }
                
                const pageResult = await pageResponse.json();
                
                // willtooladdtomappingin
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
                
                // check if there are more pages
                if (page >= pageResult.total_pages) {
                    hasMore = false;
                } else {
                    page++;
                }
            }
            
            // willalltooladdtoconfigurationin
            allToolsMap.forEach((tool, toolKey) => {
                config.tools.push({
                    name: tool.name,
                    enabled: tool.enabled,
                    is_external: tool.is_external,
                    external_mcp: tool.external_mcp
                });
            });
        } catch (error) {
            console.warn('Failed to get all tool list,using global state mapping only', error);
            // if retrieval fails,useglobalstatusmapping
            toolStateMap.forEach((toolData, toolKey) => {
                // toolData.name Saveoriginaltool name
                const toolName = toolData.name || toolKey.split('::').pop();
                config.tools.push({
                    name: toolName,
                    enabled: toolData.enabled,
                    is_external: toolData.is_external,
                    external_mcp: toolData.external_mcp
                });
            });
        }
        
        // updateconfiguration
        const updateResponse = await apiFetch('/api/config', {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(config)
        });
        
        if (!updateResponse.ok) {
            const error = await updateResponse.json();
            throw new Error(error.error || 'Failed to update configuration');
        }
        
        // shoulduseconfiguration
        const applyResponse = await apiFetch('/api/config/apply', {
            method: 'POST'
        });
        
        if (!applyResponse.ok) {
            const error = await applyResponse.json();
            throw new Error(error.error || 'Failed to apply configuration');
        }
        
        alert(typeof window.t === 'function' ? window.t('mcp.toolsConfigSaved') : 'Tool configuration saved successfully!');
        
 // re-loadtoollisttolateststatus
        if (typeof loadToolsList === 'function') {
            await loadToolsList(toolsPagination.page, toolsSearchKeyword);
        }
    } catch (error) {
        console.error('Failed to save tool configuration:', error);
        alert((typeof window.t === 'function' ? window.t('mcp.saveToolsConfigFailed') : 'Failed to save tool configuration') + ': ' + error.message);
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
        alert(typeof window.t === 'function' ? window.t('settings.security.fillPasswordHint') : 'Fill in current and new password correctly. New password at least 8  chars, both entries must match.');
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

        const pwdMsg = typeof window.t === 'function' ? window.t('settings.security.passwordUpdated') : 'Password updated. Please log in again with new password.';
        alert(pwdMsg);
        resetPasswordForm();
        handleUnauthorized({ message: pwdMsg, silent: false });
        closeSettings();
    } catch (error) {
        console.error('Failed to change password:', error);
        alert((typeof window.t === 'function' ? window.t('settings.security.changePasswordFailed') : 'Failed to change password') + ': ' + error.message);
    } finally {
        if (submitBtn) {
            submitBtn.disabled = false;
        }
    }
}

// ==================== ExternalMCPmanagement ====================

let currentEditingMCPName = null;

// fetchExternalMCPlistdata(forpolluse,return { servers, stats })
async function fetchExternalMCPs() {
    const response = await apiFetch('/api/external-mcp');
    if (!response.ok) throw new Error('getExternalMCPlist failed');
    return response.json();
}

// loadExternalMCPlistrender
async function loadExternalMCPs() {
    try {
        const data = await fetchExternalMCPs();
        renderExternalMCPList(data.servers || {});
        renderExternalMCPStats(data.stats || {});
    } catch (error) {
        console.error('loadExternalMCPlist failed:', error);
        const list = document.getElementById('external-mcp-list');
        if (list) {
            const errT = typeof window.t === 'function' ? window.t : (k) => k;
        list.innerHTML = `<div class="error">${escapeHtml(errT('mcp.loadExternalMCPFailed'))}: ${escapeHtml(error.message)}</div>`;
        }
    }
}

// polllisttospecify MCP 's tool countalreadyupdate(eachsecondtime,toi.e.,no fixed delay)
// name as null whenonlyby maxAttempts timepoll,notdetermine tool_count
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

// renderExternalMCPlist
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

// renderExternalMCPstatisticsInfo
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

// Show Add ExternalMCPmodal
function showAddExternalMCPModal() {
    currentEditingMCPName = null;
    document.getElementById('external-mcp-modal-title').textContent = (typeof window.t === 'function' ? window.t('mcp.addExternalMCP') : 'Add ExternalMCP');
    document.getElementById('external-mcp-json').value = '';
    document.getElementById('external-mcp-json-error').style.display = 'none';
    document.getElementById('external-mcp-json-error').textContent = '';
    document.getElementById('external-mcp-json').classList.remove('error');
    document.getElementById('external-mcp-modal').style.display = 'block';
}

// CloseExternalMCPmodal
function closeExternalMCPModal() {
    document.getElementById('external-mcp-modal').style.display = 'none';
    currentEditingMCPName = null;
}

// Edit ExternalMCP
async function editExternalMCP(name) {
    try {
        const response = await apiFetch(`/api/external-mcp/${encodeURIComponent(name)}`);
        if (!response.ok) {
            throw new Error(typeof window.t === 'function' ? window.t('mcp.getConfigFailed') : 'getExternalMCPconfigurationFailed');
        }
        
        const server = await response.json();
        currentEditingMCPName = name;
        
        document.getElementById('external-mcp-modal-title').textContent = (typeof window.t === 'function' ? window.t('mcp.editExternalMCP') : 'Edit ExternalMCP');
        
        // willconfigurationconvert toobjectformat(keyasname)
        const config = { ...server.config };
        // removetool_count,external_mcp_enableetcfrontendfield,butpreserveenabled/disabledused forbackward compatible
        delete config.tool_count;
        delete config.external_mcp_enable;
        
 // wrapobjectformat:{ "name": { config } }
        const configObj = {};
        configObj[name] = config;
        
        // formatJSON
        const jsonStr = JSON.stringify(configObj, null, 2);
        document.getElementById('external-mcp-json').value = jsonStr;
        document.getElementById('external-mcp-json-error').style.display = 'none';
        document.getElementById('external-mcp-json-error').textContent = '';
        document.getElementById('external-mcp-json').classList.remove('error');
        
        document.getElementById('external-mcp-modal').style.display = 'block';
    } catch (error) {
        console.error('Edit ExternalMCPFailed:', error);
        alert((typeof window.t === 'function' ? window.t('mcp.operationFailed') : 'Edit failed') + ': ' + error.message);
    }
}

// formatJSON
function formatExternalMCPJSON() {
    const jsonTextarea = document.getElementById('external-mcp-json');
    const errorDiv = document.getElementById('external-mcp-json-error');
    
    try {
        const jsonStr = jsonTextarea.value.trim();
        if (!jsonStr) {
            errorDiv.textContent = (typeof window.t === 'function' ? window.t('mcp.jsonEmpty') : 'JSONcannot be empty');
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
        errorDiv.textContent = (typeof window.t === 'function' ? window.t('mcp.jsonError') : 'JSONformat error') + ': ' + error.message;
        errorDiv.style.display = 'block';
        jsonTextarea.classList.add('error');
    }
}

// loadExample
function loadExternalMCPExample() {
    const desc = (typeof window.t === 'function' ? window.t('externalMcpModal.exampleDescription') : 'Example description');
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

// SaveExternalMCP
async function saveExternalMCP() {
    const jsonTextarea = document.getElementById('external-mcp-json');
    const jsonStr = jsonTextarea.value.trim();
    const errorDiv = document.getElementById('external-mcp-json-error');
    
    if (!jsonStr) {
        errorDiv.textContent = (typeof window.t === 'function' ? window.t('mcp.jsonEmpty') : 'JSONcannot be empty');
        errorDiv.style.display = 'block';
        jsonTextarea.classList.add('error');
        jsonTextarea.focus();
        return;
    }
    
    let configObj;
    try {
        configObj = JSON.parse(jsonStr);
    } catch (error) {
        errorDiv.textContent = (typeof window.t === 'function' ? window.t('mcp.jsonError') : 'JSONformat error') + ': ' + error.message;
        errorDiv.style.display = 'block';
        jsonTextarea.classList.add('error');
        jsonTextarea.focus();
        return;
    }
    
    const t = (typeof window.t === 'function' ? window.t : function (k, opts) { return k; });
    // validatemustisobjectformat
    if (typeof configObj !== 'object' || Array.isArray(configObj) || configObj === null) {
        errorDiv.textContent = t('mcp.configMustBeObject');
        errorDiv.style.display = 'block';
        jsonTextarea.classList.add('error');
        return;
    }
    
    // getallconfigurationname
    const names = Object.keys(configObj);
    if (names.length === 0) {
        errorDiv.textContent = t('mcp.configNeedOne');
        errorDiv.style.display = 'block';
        jsonTextarea.classList.add('error');
        return;
    }
    
    // validateeachconfiguration
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
        
 // remove external_mcp_enable field(buttoncontrol,butpreserve enabled/disabled used forbackward compatible)
        delete config.external_mcp_enable;
        
        // validateconfigurationcontent
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
    
    // clearerrorhint
    errorDiv.style.display = 'none';
    jsonTextarea.classList.remove('error');
    
    try {
        // if it isEditmode,onlyupdatecurrentEdit's configuration
        if (currentEditingMCPName) {
            if (!configObj[currentEditingMCPName]) {
                errorDiv.textContent = (typeof window.t === 'function' ? window.t('mcp.configEditMustContainName', { name: currentEditingMCPName }) : 'Config error: Editmodeunder,JSONmustcontainconfigurationname "' + currentEditingMCPName + '"');
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
            // addmode:Saveallconfiguration
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
                    throw new Error(`Save "${name}" Failed: ${error.error || 'unknown error'}`);
                }
            }
        }
        
        closeExternalMCPModal();
        await loadExternalMCPs();
        if (typeof window !== 'undefined' && typeof window.refreshMentionTools === 'function') {
            window.refreshMentionTools();
        }
 // polltimetofetchbackendasyncupdate's tool count(no fixed delay,toi.e.)
        pollExternalMCPToolCount(null, 5);
        alert(typeof window.t === 'function' ? window.t('mcp.saveSuccess') : 'Save successful');
    } catch (error) {
        console.error('SaveExternalMCPFailed:', error);
        errorDiv.textContent = (typeof window.t === 'function' ? window.t('mcp.operationFailed') : 'Save failed') + ': ' + error.message;
        errorDiv.style.display = 'block';
        jsonTextarea.classList.add('error');
    }
}

// DeleteExternalMCP
async function deleteExternalMCP(name) {
    if (!confirm((typeof window.t === 'function' ? window.t('mcp.deleteExternalConfirm', { name: name }) : `OKneedDeleteExternalMCP "${name}" ?`))) {
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
        // refresh chat UI tool list,removealreadyDelete's MCPtool
        if (typeof window !== 'undefined' && typeof window.refreshMentionTools === 'function') {
            window.refreshMentionTools();
        }
        alert(typeof window.t === 'function' ? window.t('mcp.deleteSuccess') : 'Delete successful');
    } catch (error) {
        console.error('DeleteExternalMCPFailed:', error);
        alert((typeof window.t === 'function' ? window.t('mcp.operationFailed') : 'Delete failed') + ': ' + error.message);
    }
}

// switchExternalMCP
async function toggleExternalMCP(name, currentStatus) {
    const action = currentStatus === 'connected' ? 'stop' : 'start';
    const buttonId = `btn-toggle-${name}`;
    const button = document.getElementById(buttonId);
    
    // if it isStartActions,show loading state
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
        
 // if it isStartActions,firstimmediatelychecktimestatus
        if (action === 'start') {
 // immediatelychecktimestatus(possiblyalreadyconnection)
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
 // polltothis MCP tool countalreadyupdate(eachsecondtime,no fixed delay)
                        pollExternalMCPToolCount(name, 10);
                        return;
                    }
                }
            } catch (error) {
                console.error('Failed to check status:', error);
            }
            
            // ifstillnot yetconnection,startpoll
 await pollExternalMCPStatus(name, 30); // at mostpoll30time(30second)
        } else {
            // stopActions,directlyRefresh
            await loadExternalMCPs();
            // refresh chat UI tool list
            if (typeof window !== 'undefined' && typeof window.refreshMentionTools === 'function') {
                window.refreshMentionTools();
            }
        }
    } catch (error) {
        console.error('switchExternalMCPstatusFailed:', error);
        alert((typeof window.t === 'function' ? window.t('mcp.operationFailed') : 'Operation failed') + ': ' + error.message);
        
        // restorebuttonstatus
        if (button) {
            button.disabled = false;
            button.style.opacity = '1';
            button.style.cursor = 'pointer';
            button.innerHTML = '▶ Start';
        }
        
        // Refreshstatus
        await loadExternalMCPs();
        // refresh chat UI tool list
        if (typeof window !== 'undefined' && typeof window.refreshMentionTools === 'function') {
            window.refreshMentionTools();
        }
    }
}

// pollExternalMCPstatus
async function pollExternalMCPStatus(name, maxAttempts = 30) {
    let attempts = 0;
 const pollInterval = 1000; // 1secondpolltime
    
    while (attempts < maxAttempts) {
        await new Promise(resolve => setTimeout(resolve, pollInterval));
        
        try {
            const response = await apiFetch(`/api/external-mcp/${encodeURIComponent(name)}`);
            if (response.ok) {
                const data = await response.json();
                const status = data.status || 'disconnected';
                
                // updatebuttonstatus
                const buttonId = `btn-toggle-${name}`;
                const button = document.getElementById(buttonId);
                
                if (status === 'connected') {
                    await loadExternalMCPs();
                    if (typeof window !== 'undefined' && typeof window.refreshMentionTools === 'function') {
                        window.refreshMentionTools();
                    }
 // polltothis MCP tool countalreadyupdate(eachsecondtime,no fixed delay)
                    pollExternalMCPToolCount(name, 10);
                    return;
                } else if (status === 'error' || status === 'disconnected') {
 // Connection failed,Refreshlistshow error
                    await loadExternalMCPs();
                    // refresh chat UI tool list
                    if (typeof window !== 'undefined' && typeof window.refreshMentionTools === 'function') {
                        window.refreshMentionTools();
                    }
                    if (status === 'error') {
                        alert(typeof window.t === 'function' ? window.t('mcp.connectionFailedCheck') : 'Connection failed. Check config and network');
                    }
                    return;
                } else if (status === 'connecting') {
                    // stillatConnecting,Continuepoll
                    attempts++;
                    continue;
                }
            }
        } catch (error) {
            console.error('Failed to poll status:', error);
        }
        
        attempts++;
    }
    
    // exceedwhen,Refreshlist
    await loadExternalMCPs();
    // refresh chat UI tool list
    if (typeof window !== 'undefined' && typeof window.refreshMentionTools === 'function') {
        window.refreshMentionTools();
    }
    alert(typeof window.t === 'function' ? window.t('mcp.connectionTimeout') : 'Connection timed out. Check config and network');
}

// atopensettingswhenloadExternalMCPlist
const originalOpenSettings = openSettings;
openSettings = async function() {
    await originalOpenSettings();
    await loadExternalMCPs();
};

// --- API Test / Model Discovery ---

// Test the configured API endpoint, populate model dropdown and rate limits
async function testApiEndpoint() {
    const provider = document.getElementById('openai-provider').value;
    const baseUrl = document.getElementById('openai-base-url').value.trim();
    const apiKey = document.getElementById('openai-api-key').value.trim();
    const resultDiv = document.getElementById('api-test-result');
    const rateLimitDiv = document.getElementById('api-rate-limits');
    const modelSelect = document.getElementById('openai-model');
    const customInput = document.getElementById('openai-model-custom');
    const btn = document.getElementById('test-api-btn');

    if (!apiKey) {
        resultDiv.innerHTML = '<span style="color: var(--error-color);">Enter API key first</span>';
        return;
    }

    btn.disabled = true;
    btn.textContent = 'Testing...';
    resultDiv.innerHTML = '<span style="color: var(--text-muted);">Connecting...</span>';

    try {
        const response = await apiFetch('/api/config/test-api', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({ provider, base_url: baseUrl, api_key: apiKey })
        });
        const data = await response.json();

        if (data.status === 'ok') {
            resultDiv.innerHTML = '<span style="color: var(--success-color);">\u2713 Connected</span>';

            // Populate model dropdown
            const currentModel = modelSelect.value;
            modelSelect.innerHTML = '';
            if (data.models && data.models.length > 0) {
                data.models.forEach(m => {
                    const opt = document.createElement('option');
                    opt.value = m.id;
                    opt.textContent = m.name || m.id;
                    modelSelect.appendChild(opt);
                });
                // Add custom option
                const customOpt = document.createElement('option');
                customOpt.value = '__custom__';
                customOpt.textContent = '-- Custom model ID --';
                modelSelect.appendChild(customOpt);

                // Restore previous selection if it exists in the new list
                if (currentModel && [...modelSelect.options].some(o => o.value === currentModel)) {
                    modelSelect.value = currentModel;
                }
            }

            // Show rate limits
            if (data.rate_limits) {
                const rl = data.rate_limits;
                let info = [];
                if (rl.requests_per_minute) info.push(rl.requests_per_minute + ' req/min');
                if (rl.input_tokens_per_minute) info.push((rl.input_tokens_per_minute / 1000).toFixed(0) + 'k input tokens/min');
                if (rl.output_tokens_per_minute) info.push((rl.output_tokens_per_minute / 1000).toFixed(0) + 'k output tokens/min');
                if (info.length > 0) {
                    rateLimitDiv.style.display = 'block';
                    rateLimitDiv.innerHTML = 'Rate limits: ' + info.join(' \u00b7 ');
                    if (data.recommended_delay_ms > 0) {
                        rateLimitDiv.innerHTML += ' \u00b7 <span style="color: var(--warning-color);">Recommended delay: ' + data.recommended_delay_ms + 'ms</span>';
                    }
                }
            }

            // Auto-set recommended delay in knowledge indexing if present
            if (data.recommended_delay_ms !== undefined) {
                const delayInput = document.getElementById('knowledge-indexing-rate-limit-delay-ms');
                if (delayInput && data.recommended_delay_ms > 0) {
                    delayInput.value = data.recommended_delay_ms;
                }
            }
        } else {
            resultDiv.innerHTML = '<span style="color: var(--error-color);">\u2717 ' + (data.error || 'Unknown error') + '</span>';
            rateLimitDiv.style.display = 'none';
        }
    } catch (err) {
        resultDiv.innerHTML = '<span style="color: var(--error-color);">\u2717 ' + err.message + '</span>';
    } finally {
        btn.disabled = false;
        btn.textContent = 'Test API';
    }
}

// Handle select change: show custom text input when "Custom model ID" is selected
document.addEventListener('change', function(e) {
    if (e.target && e.target.id === 'openai-model') {
        const customInput = document.getElementById('openai-model-custom');
        if (e.target.value === '__custom__') {
            customInput.style.display = '';
            customInput.focus();
        } else {
            customInput.style.display = 'none';
        }
    }
});

// Provider change handler - update base URL placeholder and clear test results
function onProviderChange() {
    const provider = document.getElementById('openai-provider').value;
    const baseUrlInput = document.getElementById('openai-base-url');
    const resultDiv = document.getElementById('api-test-result');
    const rateLimitDiv = document.getElementById('api-rate-limits');
    const modelSelect = document.getElementById('openai-model');
    const customInput = document.getElementById('openai-model-custom');

    // Reset test results
    if (resultDiv) resultDiv.innerHTML = '';
    if (rateLimitDiv) { rateLimitDiv.style.display = 'none'; rateLimitDiv.innerHTML = ''; }

    // Reset model dropdown
    if (modelSelect) {
        modelSelect.innerHTML = '<option value="">-- Test API to load models --</option>';
    }
    if (customInput) customInput.style.display = 'none';

    // Suggest default base URL
    if (provider === 'anthropic') {
        if (!baseUrlInput.value || baseUrlInput.value === 'https://api.openai.com/v1') {
            baseUrlInput.value = 'https://api.anthropic.com/v1';
        }
        baseUrlInput.placeholder = 'https://api.anthropic.com/v1';
    } else {
        if (!baseUrlInput.value || baseUrlInput.value === 'https://api.anthropic.com/v1') {
            baseUrlInput.value = 'https://api.openai.com/v1';
        }
        baseUrlInput.placeholder = 'https://api.openai.com/v1';
    }
}

// Helper: get the effective model value from select or custom input
function getSelectedModel() {
    const modelSelect = document.getElementById('openai-model');
    const customInput = document.getElementById('openai-model-custom');
    if (modelSelect.value === '__custom__') {
        return (customInput.value || '').trim();
    }
    return (modelSelect.value || '').trim();
}

// languageswitchafterre-render MCP managementpagein JS write's block(innerHTML notwill data-i18n autoupdate)
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

// ═══════════════════════════════════════════════
// Plugin Management
// ═══════════════════════════════════════════════

async function loadPlugins() {
    const container = document.getElementById('plugins-list');
    if (!container) return;

    try {
        const response = await apiFetch('/api/plugins');
        const data = await response.json();

        if (!data.plugins || data.plugins.length === 0) {
            container.innerHTML = '<div style="text-align: center; padding: 40px; color: var(--text-muted);">No plugins found. Drop plugin folders into the <code>plugins/</code> directory.</div>';
            return;
        }

        container.innerHTML = data.plugins.map(function(p) { return renderPluginCard(p); }).join('');
    } catch (err) {
        container.innerHTML = '<div style="color: var(--error-color); padding: 20px;">Failed to load plugins: ' + err.message + '</div>';
    }
}

function renderPluginCard(plugin) {
    var m = plugin.manifest;
    var badges = [];
    if (m.provides.tools) badges.push(plugin.tool_count + ' tools');
    if (m.provides.skills) badges.push(plugin.skill_count + ' skills');
    if (m.provides.agents) badges.push(plugin.agent_count + ' agents');
    if (m.provides.knowledge) badges.push('knowledge');

    var configHtml = '';
    if (m.config && m.config.length > 0) {
        var configItems = m.config.map(function(c) {
            return '<div style="display: flex; gap: 8px; align-items: center; margin-bottom: 8px;">' +
                '<label style="min-width: 140px; font-size: 12px; color: var(--text-muted);">' + c.name + (c.required ? ' *' : '') + '</label>' +
                '<input type="password" class="plugin-config-input" data-plugin="' + m.name + '" data-key="' + c.name + '" ' +
                'placeholder="' + c.description + '" style="flex: 1; font-size: 12px;" />' +
                '<button class="btn-secondary btn-small" onclick="savePluginConfigVar(\'' + m.name + '\', \'' + c.name + '\', this)" style="font-size: 11px;">Set</button>' +
                '</div>';
        }).join('');
        configHtml = '<div class="plugin-config" id="plugin-config-' + m.name + '" style="margin-top: 12px; ' + (plugin.enabled ? '' : 'display:none;') + '">' +
            configItems + '</div>';
    }

    var installBtn = '';
    if (m.requirements) {
        installBtn = '<button class="btn-secondary btn-small" onclick="installPluginDeps(\'' + m.name + '\', this)" style="font-size: 11px;">' +
            (plugin.installed ? 'Reinstall Deps' : 'Install Deps') + '</button>';
    }

    var badgesHtml = badges.map(function(b) {
        return '<span style="font-size: 10px; padding: 2px 6px; border-radius: 4px; background: var(--bg-tertiary, #333); color: var(--text-muted);">' + b + '</span>';
    }).join('');

    var warningHtml = '';
    if (!plugin.config_set && m.config && m.config.some(function(c) { return c.required; })) {
        warningHtml = '<div style="margin-top: 8px; font-size: 11px; color: var(--warning-color);">Configure required API keys before enabling</div>';
    }

    var errorHtml = '';
    if (plugin.error) {
        errorHtml = '<div style="margin-top: 8px; font-size: 11px; color: var(--error-color);">' + plugin.error + '</div>';
    }

    return '<div class="plugin-card" style="border: 1px solid var(--border-color); border-radius: 8px; padding: 16px; margin-bottom: 12px; background: var(--card-bg, var(--bg-secondary));">' +
        '<div style="display: flex; justify-content: space-between; align-items: flex-start;">' +
            '<div>' +
                '<div style="display: flex; align-items: center; gap: 8px;">' +
                    '<strong style="font-size: 14px;">' + m.name + '</strong>' +
                    '<span style="font-size: 11px; color: var(--text-muted);">v' + m.version + '</span>' +
                    badgesHtml +
                '</div>' +
                '<p style="font-size: 12px; color: var(--text-muted); margin: 4px 0;">' + m.description + '</p>' +
                (m.author ? '<span style="font-size: 11px; color: var(--text-muted);">by ' + m.author + '</span>' : '') +
            '</div>' +
            '<div style="display: flex; gap: 8px; align-items: center;">' +
                installBtn +
                '<label class="toggle-switch" style="margin: 0;">' +
                    '<input type="checkbox" ' + (plugin.enabled ? 'checked' : '') + ' onchange="togglePlugin(\'' + m.name + '\', this.checked)" />' +
                    '<span class="toggle-slider"></span>' +
                '</label>' +
            '</div>' +
        '</div>' +
        warningHtml +
        errorHtml +
        configHtml +
    '</div>';
}

async function togglePlugin(name, enabled) {
    try {
        var action = enabled ? 'enable' : 'disable';
        await apiFetch('/api/plugins/' + name + '/' + action, { method: 'POST' });
        showNotification('Plugin ' + name + ' ' + action + 'd', 'success');
        // Show/hide config section
        var configEl = document.getElementById('plugin-config-' + name);
        if (configEl) configEl.style.display = enabled ? '' : 'none';
    } catch (err) {
        showNotification('Failed to toggle plugin: ' + err.message, 'error');
        loadPlugins(); // reload to sync state
    }
}

async function savePluginConfigVar(pluginName, key, btn) {
    var input = btn.parentElement.querySelector('input');
    var value = input.value.trim();
    if (!value) return;

    try {
        btn.disabled = true;
        btn.textContent = 'Saving...';
        var body = {};
        body[key] = value;
        await apiFetch('/api/plugins/' + pluginName + '/config', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify(body)
        });
        input.value = '';
        input.placeholder = 'Configured';
        showNotification(key + ' saved', 'success');
    } catch (err) {
        showNotification('Failed to save config: ' + err.message, 'error');
    } finally {
        btn.disabled = false;
        btn.textContent = 'Set';
    }
}

async function installPluginDeps(name, btn) {
    try {
        btn.disabled = true;
        btn.textContent = 'Installing...';
        await apiFetch('/api/plugins/' + name + '/install', { method: 'POST' });
        showNotification('Dependencies installed for ' + name, 'success');
        btn.textContent = 'Reinstall Deps';
    } catch (err) {
        showNotification('Install failed: ' + err.message, 'error');
        btn.textContent = 'Install Deps';
    } finally {
        btn.disabled = false;
    }
}
