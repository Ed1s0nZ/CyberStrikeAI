// 
function _t(key, opts) {
    return typeof window.t === 'function' ? window.t(key, opts) : key;
}

// "" HTML( data-i18n )
function getKnowledgeNotEnabledHTML() {
    return `
        <div class="empty-state" style="text-align: center; padding: 40px 20px;">
            <div style="font-size: 48px; margin-bottom: 20px;">📚</div>
            <h3 data-i18n="knowledge.notEnabledTitle" style="margin-bottom: 10px; color: #666;"></h3>
            <p data-i18n="knowledge.notEnabledHint" style="color: #999; margin-bottom: 20px;"></p>
            <button data-i18n="knowledge.goToSettings" onclick="switchToSettings()" style="
                background: #007bff;
                color: white;
                border: none;
                padding: 10px 20px;
                border-radius: 5px;
                cursor: pointer;
                font-size: 14px;
            "></button>
        </div>
    `;
}

// "",
function renderKnowledgeNotEnabledState(container) {
    if (!container) return;
    container.innerHTML = getKnowledgeNotEnabledHTML();
    if (typeof window.applyTranslations === 'function') {
        window.applyTranslations(container);
    }
}

let knowledgeCategories = [];
let knowledgeItems = [];
let currentEditingItemId = null;
let isSavingKnowledgeItem = false; // 
let retrievalLogsData = []; // ,
let knowledgePagination = {
    currentPage: 1,
 pageSize: 10, // ()
    total: 0,
    currentCategory: ''
};
let knowledgeSearchTimeout = null; // 

// 
async function loadKnowledgeCategories() {
    try {
 // 
        const timestamp = Date.now();
        const response = await apiFetch(`/api/knowledge/categories?_t=${timestamp}`, {
            method: 'GET',
            headers: {
                'Cache-Control': 'no-cache, no-store, must-revalidate',
                'Pragma': 'no-cache',
                'Expires': '0'
            }
        });
        if (!response.ok) {
 throw new Error('');
        }
        const data = await response.json();
        
 // 
        if (data.enabled === false) {
 // ,( data-i18n,)
            renderKnowledgeNotEnabledState(document.getElementById('knowledge-items-list'));
            return [];
        }
        
        knowledgeCategories = data.categories || [];
        
 // 
        const filterDropdown = document.getElementById('knowledge-category-filter-dropdown');
        if (filterDropdown) {
 filterDropdown.innerHTML = '<div class="custom-select-option" data-value="" onclick="selectKnowledgeCategory(\'\')"></div>';
            knowledgeCategories.forEach(category => {
                const option = document.createElement('div');
                option.className = 'custom-select-option';
                option.setAttribute('data-value', category);
                option.textContent = category;
                option.onclick = function() {
                    selectKnowledgeCategory(category);
                };
                filterDropdown.appendChild(option);
            });
        }
        
        return knowledgeCategories;
    } catch (error) {
 console.error(':', error);
 // 
 if (!error.message.includes('')) {
 showNotification(': ' + error.message, 'error');
        }
        return [];
    }
}

// (,)
async function loadKnowledgeItems(category = '', page = 1, pageSize = 10) {
    try {
 // 
        knowledgePagination.currentCategory = category;
        knowledgePagination.currentPage = page;
        knowledgePagination.pageSize = pageSize;
        
 // URL(,)
        const timestamp = Date.now();
        const offset = (page - 1) * pageSize;
        let url = `/api/knowledge/items?categoryPage=true&limit=${pageSize}&offset=${offset}&_t=${timestamp}`;
        if (category) {
            url += `&category=${encodeURIComponent(category)}`;
        }
        
        const response = await apiFetch(url, {
            method: 'GET',
            headers: {
                'Cache-Control': 'no-cache, no-store, must-revalidate',
                'Pragma': 'no-cache',
                'Expires': '0'
            }
        });
        
        if (!response.ok) {
 throw new Error('');
        }
        const data = await response.json();
        
 // 
        if (data.enabled === false) {
 // ,(; data-i18n,)
            const container = document.getElementById('knowledge-items-list');
            if (container && !container.querySelector('.empty-state')) {
                renderKnowledgeNotEnabledState(container);
            }
            knowledgeItems = [];
            knowledgePagination.total = 0;
            renderKnowledgePagination();
            return [];
        }
        
 // 
        const categoriesWithItems = data.categories || [];
 knowledgePagination.total = data.total || 0; // 
        
        renderKnowledgeItemsByCategories(categoriesWithItems);
        
 // ,()
        if (category) {
            const paginationContainer = document.getElementById('knowledge-pagination');
            if (paginationContainer) {
                paginationContainer.innerHTML = '';
            }
        } else {
            renderKnowledgePagination();
        }
        return categoriesWithItems;
    } catch (error) {
 console.error(':', error);
 // 
 if (!error.message.includes('')) {
 showNotification(': ' + error.message, 'error');
        }
        return [];
    }
}

// ()
function renderKnowledgeItemsByCategories(categoriesWithItems) {
    const container = document.getElementById('knowledge-items-list');
    if (!container) return;
    
    if (categoriesWithItems.length === 0) {
 container.innerHTML = '<div class="empty-state"></div>';
        return;
    }
    
 // 
    const totalItems = categoriesWithItems.reduce((sum, cat) => sum + (cat.items?.length || 0), 0);
    const categoryCount = categoriesWithItems.length;
    
 // 
    updateKnowledgeStats(categoriesWithItems, categoryCount);
    
 // 
    let html = '<div class="knowledge-categories-container">';
    
    categoriesWithItems.forEach(categoryData => {
 const category = categoryData.category || '';
        const categoryItems = categoryData.items || [];
        const categoryCount = categoryData.itemCount || categoryItems.length;
        
        html += `
            <div class="knowledge-category-section" data-category="${escapeHtml(category)}">
                <div class="knowledge-category-header">
                    <div class="knowledge-category-info">
                        <h3 class="knowledge-category-title">${escapeHtml(category)}</h3>
 <span class="knowledge-category-count">${categoryCount} </span>
                    </div>
                </div>
                <div class="knowledge-items-grid">
                    ${categoryItems.map(item => renderKnowledgeItemCard(item)).join('')}
                </div>
            </div>
        `;
    });
    
    html += '</div>';
    container.innerHTML = html;
}

// (,)
function renderKnowledgeItems(items) {
    const container = document.getElementById('knowledge-items-list');
    if (!container) return;
    
    if (items.length === 0) {
 container.innerHTML = '<div class="empty-state"></div>';
        return;
    }
    
 // 
    const groupedByCategory = {};
    items.forEach(item => {
 const category = item.category || '';
        if (!groupedByCategory[category]) {
            groupedByCategory[category] = [];
        }
        groupedByCategory[category].push(item);
    });
    
 // 
    updateKnowledgeStats(items, Object.keys(groupedByCategory).length);
    
 // 
    const categories = Object.keys(groupedByCategory).sort();
    let html = '<div class="knowledge-categories-container">';
    
    categories.forEach(category => {
        const categoryItems = groupedByCategory[category];
        const categoryCount = categoryItems.length;
        
        html += `
            <div class="knowledge-category-section" data-category="${escapeHtml(category)}">
                <div class="knowledge-category-header">
                    <div class="knowledge-category-info">
                        <h3 class="knowledge-category-title">${escapeHtml(category)}</h3>
 <span class="knowledge-category-count">${categoryCount} </span>
                    </div>
                </div>
                <div class="knowledge-items-grid">
                    ${categoryItems.map(item => renderKnowledgeItemCard(item)).join('')}
                </div>
            </div>
        `;
    });
    
    html += '</div>';
    container.innerHTML = html;
}

// ()
function renderKnowledgePagination() {
    const container = document.getElementById('knowledge-pagination');
    if (!container) return;
    
    const { currentPage, pageSize, total } = knowledgePagination;
 const totalPages = Math.ceil(total / pageSize); // total
    
    if (totalPages <= 1) {
        container.innerHTML = '';
        return;
    }
    
    let html = '<div class="knowledge-pagination" style="display: flex; justify-content: center; align-items: center; gap: 8px; padding: 20px; flex-wrap: wrap;">';
    
 // 
 html += `<button class="pagination-btn" onclick="loadKnowledgePage(${currentPage - 1})" ${currentPage <= 1 ? 'disabled style="opacity: 0.5; cursor: not-allowed;"' : ''}></button>`;
    
 // ()
 html += `<span style="padding: 0 12px;"> ${currentPage} , ${totalPages} ( ${total} )</span>`;
    
 // 
 html += `<button class="pagination-btn" onclick="loadKnowledgePage(${currentPage + 1})" ${currentPage >= totalPages ? 'disabled style="opacity: 0.5; cursor: not-allowed;"' : ''}></button>`;
    
    html += '</div>';
    container.innerHTML = html;
}

// 
function loadKnowledgePage(page) {
    const { currentCategory, pageSize, total } = knowledgePagination;
    const totalPages = Math.ceil(total / pageSize);
    
    if (page < 1 || page > totalPages) {
        return;
    }
    
    loadKnowledgeItems(currentCategory, page, pageSize);
}

// 
function renderKnowledgeItemCard(item) {
 // (itemcontent,,)
    let previewText = '';
    if (item.content) {
 // markdown,150
        let preview = item.content;
 // markdown
        preview = preview.replace(/^#+\s+/gm, '');
 // 
        preview = preview.replace(/```[\s\S]*?```/g, '');
 // 
        preview = preview.replace(/`[^`]+`/g, '');
 // 
        preview = preview.replace(/\[([^\]]+)\]\([^\)]+\)/g, '$1');
 // 
        preview = preview.replace(/\n+/g, ' ').replace(/\s+/g, ' ').trim();
        
        previewText = preview.length > 150 ? preview.substring(0, 150) + '...' : preview;
    }
    
 // 
    const filePath = item.filePath || '';
 const relativePath = filePath.split(/[/\\]/).slice(-2).join('/'); // 
    
 // 
    const createdTime = formatTime(item.createdAt);
    const updatedTime = formatTime(item.updatedAt);
    
 // ,
    const displayTime = updatedTime || createdTime;
 const timeLabel = updatedTime ? '' : '';
    
 // (7)
    let isRecent = false;
    if (item.updatedAt && updatedTime) {
        const updateDate = new Date(item.updatedAt);
        if (!isNaN(updateDate.getTime())) {
            isRecent = (Date.now() - updateDate.getTime()) < 7 * 24 * 60 * 60 * 1000;
        }
    }
    
    return `
        <div class="knowledge-item-card" data-id="${item.id}" data-category="${escapeHtml(item.category)}">
            <div class="knowledge-item-card-header">
                <div class="knowledge-item-card-title-row">
                    <h4 class="knowledge-item-card-title" title="${escapeHtml(item.title)}">${escapeHtml(item.title)}</h4>
                    <div class="knowledge-item-card-actions">
 <button class="knowledge-item-action-btn" onclick="editKnowledgeItem('${item.id}')" title="">
                            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                                <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                                <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                            </svg>
                        </button>
 <button class="knowledge-item-action-btn knowledge-item-delete-btn" onclick="deleteKnowledgeItem('${item.id}')" title="">
                            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                                <path d="M3 6h18M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                            </svg>
                        </button>
                    </div>
                </div>
                ${relativePath ? `<div class="knowledge-item-path">📁 ${escapeHtml(relativePath)}</div>` : ''}
            </div>
            ${previewText ? `
            <div class="knowledge-item-card-content">
                <p class="knowledge-item-preview">${escapeHtml(previewText)}</p>
            </div>
            ` : ''}
            <div class="knowledge-item-card-footer">
                <div class="knowledge-item-meta">
                    ${displayTime ? `<span class="knowledge-item-time" title="${timeLabel}">🕒 ${displayTime}</span>` : ''}
 ${isRecent ? '<span class="knowledge-item-badge-new"></span>' : ''}
                </div>
            </div>
        </div>
    `;
}

// ()
function updateKnowledgeStats(data, categoryCount) {
    const statsContainer = document.getElementById('knowledge-stats');
    if (!statsContainer) return;
    
 // 
    let currentPageItemCount = 0;
    if (Array.isArray(data) && data.length > 0) {
 // categoriesWithItemsitems
        if (data[0].category !== undefined && data[0].items !== undefined) {
 // 
            currentPageItemCount = data.reduce((sum, cat) => sum + (cat.items?.length || 0), 0);
        } else {
 // ()
            currentPageItemCount = data.length;
        }
    }
    
 // (,)
    const totalCategories = (knowledgePagination.total != null) ? knowledgePagination.total : categoryCount;
    
    statsContainer.innerHTML = `
        <div class="knowledge-stat-item">
 <span class="knowledge-stat-label"></span>
            <span class="knowledge-stat-value">${totalCategories}</span>
        </div>
        <div class="knowledge-stat-item">
 <span class="knowledge-stat-label"></span>
 <span class="knowledge-stat-value">${categoryCount} </span>
        </div>
        <div class="knowledge-stat-item">
 <span class="knowledge-stat-label"></span>
 <span class="knowledge-stat-value">${currentPageItemCount} </span>
        </div>
    `;
    
 // 
    updateIndexProgress();
}

// 
let indexProgressInterval = null;

async function updateIndexProgress() {
    try {
        const response = await apiFetch('/api/knowledge/index-status', {
            method: 'GET',
            headers: {
                'Cache-Control': 'no-cache, no-store, must-revalidate',
                'Pragma': 'no-cache',
                'Expires': '0'
            }
        });
        
        if (!response.ok) {
 return; // ,
        }
        
        const status = await response.json();
        const progressContainer = document.getElementById('knowledge-index-progress');
        if (!progressContainer) return;
        
 // 
        if (status.enabled === false) {
 // ,
            progressContainer.style.display = 'none';
            if (indexProgressInterval) {
                clearInterval(indexProgressInterval);
                indexProgressInterval = null;
            }
            return;
        }
        
        const totalItems = status.total_items || 0;
        const indexedItems = status.indexed_items || 0;
        const progressPercent = status.progress_percent || 0;
        const isComplete = status.is_complete || false;
        const lastError = status.last_error || '';
        
 // ()
        const isRebuilding = status.is_rebuilding || false;
        
        if (totalItems === 0) {
 // ,
            progressContainer.style.display = 'none';
            if (indexProgressInterval) {
                clearInterval(indexProgressInterval);
                indexProgressInterval = null;
            }
            return;
        }
        
 // 
        progressContainer.style.display = 'block';
        
 // ,
        if (lastError) {
            progressContainer.innerHTML = `
                <div class="knowledge-index-progress-error" style="
                    background: #fee;
                    border: 1px solid #fcc;
                    border-radius: 8px;
                    padding: 16px;
                    margin-bottom: 16px;
                ">
                    <div style="display: flex; align-items: center; margin-bottom: 8px;">
                        <span style="font-size: 20px; margin-right: 8px;">❌</span>
 <span style="font-weight: bold; color: #c00;"></span>
                    </div>
                    <div style="color: #666; font-size: 14px; margin-bottom: 12px; line-height: 1.5;">
                        ${escapeHtml(lastError)}
                    </div>
                    <div style="color: #999; font-size: 12px; margin-bottom: 12px;">
 :,API,..
                    </div>
                    <div style="display: flex; gap: 8px;">
                        <button onclick="rebuildKnowledgeIndex()" style="
                            background: #007bff;
                            color: white;
                            border: none;
                            padding: 6px 12px;
                            border-radius: 4px;
                            cursor: pointer;
                            font-size: 13px;
 "></button>
                        <button onclick="stopIndexProgressPolling()" style="
                            background: #6c757d;
                            color: white;
                            border: none;
                            padding: 6px 12px;
                            border-radius: 4px;
                            cursor: pointer;
                            font-size: 13px;
 "></button>
                    </div>
                </div>
            `;
 // 
            if (indexProgressInterval) {
                clearInterval(indexProgressInterval);
                indexProgressInterval = null;
            }
 // 
 showNotification(': ' + lastError.substring(0, 100), 'error');
            return;
        }
        

 // 
        if (isRebuilding) {
            const rebuildTotal = status.rebuild_total || totalItems;
            const rebuildCurrent = status.rebuild_current || 0;
            const rebuildFailed = status.rebuild_failed || 0;
            const rebuildLastItemID = status.rebuild_last_item_id || '';
            const rebuildLastChunks = status.rebuild_last_chunks || 0;
            const rebuildStartTime = status.rebuild_start_time || '';

 // ()
            let rebuildProgress = progressPercent;
            if (rebuildTotal > 0) {
                rebuildProgress = (rebuildCurrent / rebuildTotal) * 100;
            }

            progressContainer.innerHTML = `
                <div class="knowledge-index-progress">
                    <div class="progress-header">
                        <span class="progress-icon">🔨</span>
 <span class="progress-text">:${rebuildCurrent}/${rebuildTotal} (${rebuildProgress.toFixed(1)}%) - :${rebuildFailed}</span>
                    </div>
                    <div class="progress-bar-container">
                        <div class="progress-bar" style="width: ${rebuildProgress}%"></div>
                    </div>
                    <div class="progress-hint">
 ${rebuildLastItemID ? `:${escapeHtml(rebuildLastItemID.substring(0, 36))}... (${rebuildLastChunks} chunks)` : '...'}
 ${rebuildStartTime ? `<br>:${new Date(rebuildStartTime).toLocaleString()}` : ''}
                    </div>
                </div>
            `;

 // 
            if (!indexProgressInterval) {
                indexProgressInterval = setInterval(updateIndexProgress, 2000);
            }
            return;
        }
        
        if (isComplete) {
            progressContainer.innerHTML = `
                <div class="knowledge-index-progress-complete">
                    <span class="progress-icon">✅</span>
 <span class="progress-text"> (${indexedItems}/${totalItems})</span>
                </div>
            `;
 // 
            if (indexProgressInterval) {
                clearInterval(indexProgressInterval);
                indexProgressInterval = null;
            }
        } else {
            progressContainer.innerHTML = `
                <div class="knowledge-index-progress">
                    <div class="progress-header">
                        <span class="progress-icon">🔨</span>
 <span class="progress-text">: ${indexedItems}/${totalItems} (${progressPercent.toFixed(1)}%)</span>
                    </div>
                    <div class="progress-bar-container">
                        <div class="progress-bar" style="width: ${progressPercent}%"></div>
                    </div>
 <div class="progress-hint">,</div>
                </div>
            `;
            
 // ,
            if (!indexProgressInterval) {
 indexProgressInterval = setInterval(updateIndexProgress, 3000); // 3
            }
        }
    } catch (error) {
 // 
 console.error(':', error);
        const progressContainer = document.getElementById('knowledge-index-progress');
        if (progressContainer) {
            progressContainer.style.display = 'block';
            progressContainer.innerHTML = `
                <div class="knowledge-index-progress-error" style="
                    background: #fee;
                    border: 1px solid #fcc;
                    border-radius: 8px;
                    padding: 16px;
                    margin-bottom: 16px;
                ">
                    <div style="display: flex; align-items: center; margin-bottom: 8px;">
                        <span style="font-size: 20px; margin-right: 8px;">⚠️</span>
 <span style="font-weight: bold; color: #c00;"></span>
                    </div>
                    <div style="color: #666; font-size: 14px;">
 ,.
                    </div>
                </div>
            `;
        }
 // 
        if (indexProgressInterval) {
            clearInterval(indexProgressInterval);
            indexProgressInterval = null;
        }
    }
}

// 
function stopIndexProgressPolling() {
    if (indexProgressInterval) {
        clearInterval(indexProgressInterval);
        indexProgressInterval = null;
    }
    const progressContainer = document.getElementById('knowledge-index-progress');
    if (progressContainer) {
        progressContainer.style.display = 'none';
    }
}

// 
function selectKnowledgeCategory(category) {
    const trigger = document.getElementById('knowledge-category-filter-trigger');
    const wrapper = document.getElementById('knowledge-category-filter-wrapper');
    const dropdown = document.getElementById('knowledge-category-filter-dropdown');
    
    if (trigger && wrapper && dropdown) {
 const displayText = category || '';
        trigger.querySelector('span').textContent = displayText;
        wrapper.classList.remove('open');
        
 // 
        dropdown.querySelectorAll('.custom-select-option').forEach(opt => {
            opt.classList.remove('selected');
            if (opt.getAttribute('data-value') === category) {
                opt.classList.add('selected');
            }
        });
    }
 // (,API)
    loadKnowledgeItems(category, 1, knowledgePagination.pageSize);
}

// 
function filterKnowledgeItems() {
    const wrapper = document.getElementById('knowledge-category-filter-wrapper');
    if (wrapper) {
        const selectedOption = wrapper.querySelector('.custom-select-option.selected');
        const category = selectedOption ? selectedOption.getAttribute('data-value') : '';
 // 
        loadKnowledgeItems(category, 1, knowledgePagination.pageSize);
    }
}

// ()
function handleKnowledgeSearchInput() {
    const searchInput = document.getElementById('knowledge-search');
    const searchTerm = searchInput?.value.trim() || '';
    
 // 
    if (knowledgeSearchTimeout) {
        clearTimeout(knowledgeSearchTimeout);
    }
    
 // ,
    if (!searchTerm) {
        const wrapper = document.getElementById('knowledge-category-filter-wrapper');
        let category = '';
        if (wrapper) {
            const selectedOption = wrapper.querySelector('.custom-select-option.selected');
            category = selectedOption ? selectedOption.getAttribute('data-value') : '';
        }
        loadKnowledgeItems(category, 1, knowledgePagination.pageSize);
        return;
    }
    
 // ,500ms()
    knowledgeSearchTimeout = setTimeout(() => {
        searchKnowledgeItems();
    }, 500);
}

// (,)
async function searchKnowledgeItems() {
    const searchInput = document.getElementById('knowledge-search');
    const searchTerm = searchInput?.value.trim() || '';
    
    if (!searchTerm) {
 // ()
        const wrapper = document.getElementById('knowledge-category-filter-wrapper');
        let category = '';
        if (wrapper) {
            const selectedOption = wrapper.querySelector('.custom-select-option.selected');
            category = selectedOption ? selectedOption.getAttribute('data-value') : '';
        }
        await loadKnowledgeItems(category, 1, knowledgePagination.pageSize);
        return;
    }
    
    try {
 // 
        const wrapper = document.getElementById('knowledge-category-filter-wrapper');
        let category = '';
        if (wrapper) {
            const selectedOption = wrapper.querySelector('.custom-select-option.selected');
            category = selectedOption ? selectedOption.getAttribute('data-value') : '';
        }
        
 // API
        const timestamp = Date.now();
        let url = `/api/knowledge/items?search=${encodeURIComponent(searchTerm)}&_t=${timestamp}`;
        if (category) {
            url += `&category=${encodeURIComponent(category)}`;
        }
        
        const response = await apiFetch(url, {
            method: 'GET',
            headers: {
                'Cache-Control': 'no-cache, no-store, must-revalidate',
                'Pragma': 'no-cache',
                'Expires': '0'
            }
        });
        
        if (!response.ok) {
 throw new Error('');
        }
        
        const data = await response.json();
        
 // 
        if (data.enabled === false) {
            renderKnowledgeNotEnabledState(document.getElementById('knowledge-items-list'));
            return;
        }
        
 // 
        const categoriesWithItems = data.categories || [];
        
 // 
        const container = document.getElementById('knowledge-items-list');
        if (!container) return;
        
        if (categoriesWithItems.length === 0) {
            container.innerHTML = `
                <div class="empty-state" style="text-align: center; padding: 40px 20px;">
                    <div style="font-size: 48px; margin-bottom: 20px;">🔍</div>
 <h3 style="margin-bottom: 10px;"></h3>
 <p style="color: #999;"> "<strong>${escapeHtml(searchTerm)}</strong>" </p>
 <p style="color: #999; margin-top: 10px; font-size: 0.9em;">,</p>
                </div>
            `;
        } else {
 // 
            const totalItems = categoriesWithItems.reduce((sum, cat) => sum + (cat.items?.length || 0), 0);
            const categoryCount = categoriesWithItems.length;
            
 // 
            updateKnowledgeStats(categoriesWithItems, categoryCount);
            
 // 
            renderKnowledgeItemsByCategories(categoriesWithItems);
        }
        
 // ()
        const paginationContainer = document.getElementById('knowledge-pagination');
        if (paginationContainer) {
            paginationContainer.innerHTML = '';
        }
        
    } catch (error) {
 console.error(':', error);
 showNotification(': ' + error.message, 'error');
    }
}

// 
async function refreshKnowledgeBase() {
    try {
 showNotification('...', 'info');
        const response = await apiFetch('/api/knowledge/scan', {
            method: 'POST'
        });
        if (!response.ok) {
 throw new Error('');
        }
        const data = await response.json();
 // 
        if (data.items_to_index && data.items_to_index > 0) {
 showNotification(`, ${data.items_to_index} `, 'success');
        } else {
 showNotification(data.message || ',', 'success');
        }
 // ()
        await loadKnowledgeCategories();
        await loadKnowledgeItems(knowledgePagination.currentCategory, 1, knowledgePagination.pageSize);
        
 // 
        if (indexProgressInterval) {
            clearInterval(indexProgressInterval);
            indexProgressInterval = null;
        }
        
 // ,
        if (data.items_to_index && data.items_to_index > 0) {
            await new Promise(resolve => setTimeout(resolve, 500));
            updateIndexProgress();
 // (2)
            if (!indexProgressInterval) {
                indexProgressInterval = setInterval(updateIndexProgress, 2000);
            }
        } else {
 // ,
            updateIndexProgress();
        }
    } catch (error) {
 console.error(':', error);
 showNotification(': ' + error.message, 'error');
    }
}

// 
async function rebuildKnowledgeIndex() {
    try {
 if (!confirm('?.')) {
            return;
        }
 showNotification('...', 'info');
        
 // 
        if (indexProgressInterval) {
            clearInterval(indexProgressInterval);
            indexProgressInterval = null;
        }
        
 // "",
        const progressContainer = document.getElementById('knowledge-index-progress');
        if (progressContainer) {
            progressContainer.style.display = 'block';
            progressContainer.innerHTML = `
                <div class="knowledge-index-progress">
                    <div class="progress-header">
                        <span class="progress-icon">🔨</span>
 <span class="progress-text">: ...</span>
                    </div>
                    <div class="progress-bar-container">
                        <div class="progress-bar" style="width: 0%"></div>
                    </div>
 <div class="progress-hint">,</div>
                </div>
            `;
        }
        
        const response = await apiFetch('/api/knowledge/index', {
            method: 'POST'
        });
        if (!response.ok) {
 throw new Error('');
        }
 showNotification(',', 'success');
        
 // ,
        await new Promise(resolve => setTimeout(resolve, 500));
        
 // 
        updateIndexProgress();
        
 // (2,3)
        if (!indexProgressInterval) {
            indexProgressInterval = setInterval(updateIndexProgress, 2000);
        }
    } catch (error) {
 console.error(':', error);
 showNotification(': ' + error.message, 'error');
    }
}

// 
function showAddKnowledgeItemModal() {
    currentEditingItemId = null;
 document.getElementById('knowledge-item-modal-title').textContent = '';
    document.getElementById('knowledge-item-category').value = '';
    document.getElementById('knowledge-item-title').value = '';
    document.getElementById('knowledge-item-content').value = '';
    document.getElementById('knowledge-item-modal').style.display = 'block';
}

// 
async function editKnowledgeItem(id) {
    try {
        const response = await apiFetch(`/api/knowledge/items/${id}`);
        if (!response.ok) {
 throw new Error('');
        }
        const item = await response.json();
        
        currentEditingItemId = id;
 document.getElementById('knowledge-item-modal-title').textContent = '';
        document.getElementById('knowledge-item-category').value = item.category;
        document.getElementById('knowledge-item-title').value = item.title;
        document.getElementById('knowledge-item-content').value = item.content;
        document.getElementById('knowledge-item-modal').style.display = 'block';
    } catch (error) {
 console.error(':', error);
 showNotification(': ' + error.message, 'error');
    }
}

// 
async function saveKnowledgeItem() {
 // 
    if (isSavingKnowledgeItem) {
 showNotification(',...', 'warning');
        return;
    }
    
    const category = document.getElementById('knowledge-item-category').value.trim();
    const title = document.getElementById('knowledge-item-title').value.trim();
    const content = document.getElementById('knowledge-item-content').value.trim();
    
    if (!category || !title || !content) {
 showNotification('', 'error');
        return;
    }
    
 // 
    isSavingKnowledgeItem = true;
    
 // 
    const saveButton = document.querySelector('#knowledge-item-modal .modal-footer .btn-primary');
    const cancelButton = document.querySelector('#knowledge-item-modal .modal-footer .btn-secondary');
    const modal = document.getElementById('knowledge-item-modal');
    
 const originalButtonText = saveButton ? saveButton.textContent : '';
    const originalButtonDisabled = saveButton ? saveButton.disabled : false;
    
 // 
    const categoryInput = document.getElementById('knowledge-item-category');
    const titleInput = document.getElementById('knowledge-item-title');
    const contentInput = document.getElementById('knowledge-item-content');
    
    if (categoryInput) categoryInput.disabled = true;
    if (titleInput) titleInput.disabled = true;
    if (contentInput) contentInput.disabled = true;
    if (cancelButton) cancelButton.disabled = true;
    
 // 
    if (saveButton) {
        saveButton.disabled = true;
        saveButton.style.opacity = '0.6';
        saveButton.style.cursor = 'not-allowed';
 saveButton.textContent = '...';
    }
    
    try {
        const url = currentEditingItemId 
            ? `/api/knowledge/items/${currentEditingItemId}`
            : '/api/knowledge/items';
        const method = currentEditingItemId ? 'PUT' : 'POST';
        
        const response = await apiFetch(url, {
            method: method,
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                category,
                title,
                content
            })
        });
        
        if (!response.ok) {
            const errorData = await response.json().catch(() => ({}));
 throw new Error(errorData.error || '');
        }
        
        const item = await response.json();
 const action = currentEditingItemId ? '' : '';
 const newItemCategory = item.category || category; // 
        
 // ,
        const currentCategory = document.getElementById('knowledge-category-filter-wrapper');
        let selectedCategory = '';
        if (currentCategory) {
            const selectedOption = currentCategory.querySelector('.custom-select-option.selected');
            if (selectedOption) {
                selectedCategory = selectedOption.getAttribute('data-value') || '';
            }
        }
        
 // ,
        closeKnowledgeItemModal();
        
 // ()
        const itemsListContainer = document.getElementById('knowledge-items-list');
        const originalContent = itemsListContainer ? itemsListContainer.innerHTML : '';
        
        if (itemsListContainer) {
 itemsListContainer.innerHTML = '<div class="loading-spinner">...</div>';
        }
        
        try {
 // ,
 console.log('...');
            await loadKnowledgeCategories();
 console.log(',...');
            
 // ,
            let categoryToShow = selectedCategory;
            if (!currentEditingItemId && selectedCategory && selectedCategory !== '' && newItemCategory !== selectedCategory) {
 // ,,
                categoryToShow = newItemCategory;
 // (,)
                const trigger = document.getElementById('knowledge-category-filter-trigger');
                const wrapper = document.getElementById('knowledge-category-filter-wrapper');
                const dropdown = document.getElementById('knowledge-category-filter-dropdown');
                if (trigger && wrapper && dropdown) {
 trigger.querySelector('span').textContent = newItemCategory || '';
                    dropdown.querySelectorAll('.custom-select-option').forEach(opt => {
                        opt.classList.remove('selected');
                        if (opt.getAttribute('data-value') === newItemCategory) {
                            opt.classList.add('selected');
                        }
                    });
                }
 showNotification(`✅ ${action}!"${newItemCategory}".`, 'success');
            }
            
 // ()
            await loadKnowledgeItems(categoryToShow, 1, knowledgePagination.pageSize);
 console.log('');
        } catch (err) {
 console.error(':', err);
 // ,
            if (itemsListContainer && originalContent) {
                itemsListContainer.innerHTML = originalContent;
            }
 showNotification('⚠️ ,,', 'warning');
        }
        
    } catch (error) {
 console.error(':', error);
 showNotification('❌ : ' + error.message, 'error');
        
 // ,alert
        if (typeof window.showNotification !== 'function') {
 alert('❌ : ' + error.message);
        }
        
 // (,)
        if (categoryInput) categoryInput.disabled = false;
        if (titleInput) titleInput.disabled = false;
        if (contentInput) contentInput.disabled = false;
        if (cancelButton) cancelButton.disabled = false;
        if (saveButton) {
            saveButton.disabled = false;
            saveButton.style.opacity = '';
            saveButton.style.cursor = '';
            saveButton.textContent = originalButtonText;
        }
    } finally {
 // 
        isSavingKnowledgeItem = false;
    }
}

// 
async function deleteKnowledgeItem(id) {
 if (!confirm('?')) {
        return;
    }
    
 // 
    const itemCard = document.querySelector(`.knowledge-item-card[data-id="${id}"]`);
    const deleteButton = itemCard ? itemCard.querySelector('.knowledge-item-delete-btn') : null;
    const categorySection = itemCard ? itemCard.closest('.knowledge-category-section') : null;
    let originalDisplay = '';
    let originalOpacity = '';
    let originalButtonOpacity = '';
    
 // 
    if (deleteButton) {
        originalButtonOpacity = deleteButton.style.opacity;
        deleteButton.style.opacity = '0.5';
        deleteButton.style.cursor = 'not-allowed';
        deleteButton.disabled = true;
        
 // 
        const svg = deleteButton.querySelector('svg');
        if (svg) {
            svg.style.animation = 'spin 1s linear infinite';
        }
    }
    
 // UI()
    if (itemCard) {
        originalDisplay = itemCard.style.display;
        originalOpacity = itemCard.style.opacity;
        itemCard.style.transition = 'opacity 0.3s ease-out, transform 0.3s ease-out';
        itemCard.style.opacity = '0';
        itemCard.style.transform = 'translateX(-20px)';
        
 // 
        setTimeout(() => {
            if (itemCard.parentElement) {
                itemCard.remove();
                
 // ,
                if (categorySection) {
                    const remainingItems = categorySection.querySelectorAll('.knowledge-item-card');
                    if (remainingItems.length === 0) {
                        categorySection.style.transition = 'opacity 0.3s ease-out';
                        categorySection.style.opacity = '0';
                        setTimeout(() => {
                            if (categorySection.parentElement) {
                                categorySection.remove();
                            }
                        }, 300);
                    } else {
 // 
                        const categoryCount = categorySection.querySelector('.knowledge-category-count');
                        if (categoryCount) {
                            const newCount = remainingItems.length;
 categoryCount.textContent = `${newCount} `;
                        }
                    }
                }
                
 // ,
            }
        }, 300);
    }
    
    try {
        const response = await apiFetch(`/api/knowledge/items/${id}`, {
            method: 'DELETE'
        });
        
        if (!response.ok) {
            const errorData = await response.json().catch(() => ({}));
 throw new Error(errorData.error || '');
        }
        
 // 
 showNotification('✅ !.', 'success');
        
 // ()
        await loadKnowledgeCategories();
        await loadKnowledgeItems(knowledgePagination.currentCategory, knowledgePagination.currentPage, knowledgePagination.pageSize);
        
    } catch (error) {
 console.error(':', error);
        
 // ,
        if (itemCard && originalDisplay !== 'none') {
            itemCard.style.display = originalDisplay || '';
            itemCard.style.opacity = originalOpacity || '1';
            itemCard.style.transform = '';
            itemCard.style.transition = '';
            
 // ,
            if (categorySection && !categorySection.parentElement) {
 // ()
                await loadKnowledgeItems(knowledgePagination.currentCategory, knowledgePagination.currentPage, knowledgePagination.pageSize);
            }
        }
        
 // 
        if (deleteButton) {
            deleteButton.style.opacity = originalButtonOpacity || '';
            deleteButton.style.cursor = '';
            deleteButton.disabled = false;
            const svg = deleteButton.querySelector('svg');
            if (svg) {
                svg.style.animation = '';
            }
        }
        
 showNotification('❌ : ' + error.message, 'error');
    }
}

// ()
function updateKnowledgeStatsAfterDelete() {
    const statsContainer = document.getElementById('knowledge-stats');
    if (!statsContainer) return;
    
    const allItems = document.querySelectorAll('.knowledge-item-card');
    const allCategories = document.querySelectorAll('.knowledge-category-section');
    
    const totalItems = allItems.length;
    const categoryCount = allCategories.length;
    
 // (,)
    const statsItems = statsContainer.querySelectorAll('.knowledge-stat-item');
    if (statsItems.length >= 2) {
        const totalItemsSpan = statsItems[0].querySelector('.knowledge-stat-value');
        const categoryCountSpan = statsItems[1].querySelector('.knowledge-stat-value');
        
        if (totalItemsSpan) {
            totalItemsSpan.textContent = totalItems;
        }
        if (categoryCountSpan) {
            categoryCountSpan.textContent = categoryCount;
        }
    }
}

// 
function closeKnowledgeItemModal() {
    const modal = document.getElementById('knowledge-item-modal');
    if (modal) {
        modal.style.display = 'none';
    }
    
 // 
    currentEditingItemId = null;
    isSavingKnowledgeItem = false;
    
 // 
    const categoryInput = document.getElementById('knowledge-item-category');
    const titleInput = document.getElementById('knowledge-item-title');
    const contentInput = document.getElementById('knowledge-item-content');
    const saveButton = document.querySelector('#knowledge-item-modal .modal-footer .btn-primary');
    const cancelButton = document.querySelector('#knowledge-item-modal .modal-footer .btn-secondary');
    
    if (categoryInput) {
        categoryInput.disabled = false;
        categoryInput.value = '';
    }
    if (titleInput) {
        titleInput.disabled = false;
        titleInput.value = '';
    }
    if (contentInput) {
        contentInput.disabled = false;
        contentInput.value = '';
    }
    if (saveButton) {
        saveButton.disabled = false;
        saveButton.style.opacity = '';
        saveButton.style.cursor = '';
 saveButton.textContent = '';
    }
    if (cancelButton) {
        cancelButton.disabled = false;
    }
}

// 
async function loadRetrievalLogs(conversationId = '', messageId = '') {
    try {
        let url = '/api/knowledge/retrieval-logs?limit=100';
        if (conversationId) {
            url += `&conversationId=${encodeURIComponent(conversationId)}`;
        }
        if (messageId) {
            url += `&messageId=${encodeURIComponent(messageId)}`;
        }
        
        const response = await apiFetch(url);
        if (!response.ok) {
 throw new Error('');
        }
        const data = await response.json();
        renderRetrievalLogs(data.logs || []);
    } catch (error) {
 console.error(':', error);
 // ,"..."
        renderRetrievalLogs([]);
 // ()
        if (conversationId || messageId) {
            showNotification(_t('retrievalLogs.loadError') + ': ' + error.message, 'error');
        }
    }
}

// 
function renderRetrievalLogs(logs) {
    const container = document.getElementById('retrieval-logs-list');
    if (!container) return;
    
 // ()
    updateRetrievalStats(logs);
    
    if (logs.length === 0) {
        container.innerHTML = '<div class="empty-state">' + _t('retrievalLogs.noRecords') + '</div>';
        retrievalLogsData = [];
        return;
    }
    
 // 
    retrievalLogsData = logs;
    
    container.innerHTML = logs.map((log, index) => {
 // retrievedItems:,,
        let itemCount = 0;
        let hasResults = false;
        
        if (log.retrievedItems) {
            if (Array.isArray(log.retrievedItems)) {
 // 
                const realItems = log.retrievedItems.filter(id => id !== '_has_results');
                itemCount = realItems.length;
 // ,ID,""
                if (log.retrievedItems.includes('_has_results')) {
                    hasResults = true;
 // ID,;""()
                    if (itemCount === 0) {
 itemCount = -1; // -1 
                    }
                } else {
                    hasResults = itemCount > 0;
                }
            } else if (typeof log.retrievedItems === 'string') {
 // ,JSON
                try {
                    const parsed = JSON.parse(log.retrievedItems);
                    if (Array.isArray(parsed)) {
                        const realItems = parsed.filter(id => id !== '_has_results');
                        itemCount = realItems.length;
                        if (parsed.includes('_has_results')) {
                            hasResults = true;
                            if (itemCount === 0) {
                                itemCount = -1;
                            }
                        } else {
                            hasResults = itemCount > 0;
                        }
                    }
                } catch (e) {
 // ,
                }
            }
        }
        
        const timeAgo = getTimeAgo(log.createdAt);
        
        return `
            <div class="retrieval-log-card ${hasResults ? 'has-results' : 'no-results'}" data-index="${index}">
                <div class="retrieval-log-card-header">
                    <div class="retrieval-log-icon">
                        ${hasResults ? '🔍' : '⚠️'}
                    </div>
                    <div class="retrieval-log-main-info">
                        <div class="retrieval-log-query">
                            ${escapeHtml(log.query || _t('retrievalLogs.noQuery'))}
                        </div>
                        <div class="retrieval-log-meta">
                            <span class="retrieval-log-time" title="${formatTime(log.createdAt)}">
                                🕒 ${timeAgo}
                            </span>
                            ${log.riskType ? `<span class="retrieval-log-risk-type">📁 ${escapeHtml(log.riskType)}</span>` : ''}
                        </div>
                    </div>
                    <div class="retrieval-log-result-badge ${hasResults ? 'success' : 'empty'}">
                        ${hasResults ? (itemCount > 0 ? itemCount + ' ' + _t('retrievalLogs.itemsUnit') : _t('retrievalLogs.hasResults')) : _t('retrievalLogs.noResults')}
                    </div>
                </div>
                <div class="retrieval-log-card-body">
                    <div class="retrieval-log-details-grid">
                        ${log.conversationId ? `
                            <div class="retrieval-log-detail-item">
                                <span class="detail-label">${_t('retrievalLogs.conversationId')}</span>
                                <code class="detail-value" title="${_t('retrievalLogs.clickToCopy')}" data-copy-title-copied="${_t('common.copied')}" data-copy-title-click="${_t('retrievalLogs.clickToCopy')}" onclick="var t=this; navigator.clipboard.writeText('${escapeHtml(log.conversationId)}').then(function(){ t.title=t.getAttribute('data-copy-title-copied')||'Copied!'; setTimeout(function(){ t.title=t.getAttribute('data-copy-title-click')||'Click to copy'; }, 2000); });" style="cursor: pointer;">${escapeHtml(log.conversationId)}</code>
                            </div>
                        ` : ''}
                        ${log.messageId ? `
                            <div class="retrieval-log-detail-item">
                                <span class="detail-label">${_t('retrievalLogs.messageId')}</span>
                                <code class="detail-value" title="${_t('retrievalLogs.clickToCopy')}" data-copy-title-copied="${_t('common.copied')}" data-copy-title-click="${_t('retrievalLogs.clickToCopy')}" onclick="var el=this; navigator.clipboard.writeText('${escapeHtml(log.messageId)}').then(function(){ el.title=el.getAttribute('data-copy-title-copied')||el.title; setTimeout(function(){ el.title=el.getAttribute('data-copy-title-click')||el.title; }, 2000); });" style="cursor: pointer;">${escapeHtml(log.messageId)}</code>
                            </div>
                        ` : ''}
                        <div class="retrieval-log-detail-item">
                            <span class="detail-label">${_t('retrievalLogs.retrievalResult')}</span>
                            <span class="detail-value ${hasResults ? 'text-success' : 'text-muted'}">
                                ${hasResults ? (itemCount > 0 ? _t('retrievalLogs.foundCount', { count: itemCount }) : _t('retrievalLogs.foundUnknown')) : _t('retrievalLogs.noMatch')}
                            </span>
                        </div>
                    </div>
                    ${hasResults && log.retrievedItems && log.retrievedItems.length > 0 ? `
                        <div class="retrieval-log-items-preview">
                            <div class="retrieval-log-items-label">${_t('retrievalLogs.retrievedItemsLabel')}</div>
                            <div class="retrieval-log-items-list">
                                ${log.retrievedItems.slice(0, 3).map((itemId, idx) => `
                                    <span class="retrieval-log-item-tag">${idx + 1}</span>
                                `).join('')}
                                ${log.retrievedItems.length > 3 ? `<span class="retrieval-log-item-tag more">+${log.retrievedItems.length - 3}</span>` : ''}
                            </div>
                        </div>
                    ` : ''}
                    <div class="retrieval-log-actions">
                        <button class="btn-secondary btn-sm" onclick="showRetrievalLogDetails(${index})" style="margin-top: 12px; display: inline-flex; align-items: center; gap: 4px;">
                            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                                <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                                <circle cx="12" cy="12" r="3" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                            </svg>
                            ${_t('retrievalLogs.viewDetails')}
                        </button>
                        <button class="btn-secondary btn-sm retrieval-log-delete-btn" onclick="deleteRetrievalLog('${escapeHtml(log.id)}', ${index})" style="margin-top: 12px; margin-left: 8px; display: inline-flex; align-items: center; gap: 4px; color: var(--error-color, #dc3545); border-color: var(--error-color, #dc3545);" onmouseover="this.style.backgroundColor='rgba(220, 53, 69, 0.1)'; this.style.color='#dc3545';" onmouseout="this.style.backgroundColor=''; this.style.color='var(--error-color, #dc3545)';" title="${_t('common.delete')}">
                            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                                <path d="M3 6h18M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                            </svg>
                            ${_t('common.delete')}
                        </button>
                    </div>
                </div>
            </div>
        `;
    }).join('');
}

// 
function updateRetrievalStats(logs) {
    const statsContainer = document.getElementById('retrieval-stats');
    if (!statsContainer) return;
    
    const totalLogs = logs.length;
 // :retrievedItems,>0,
    const successfulLogs = logs.filter(log => {
        if (!log.retrievedItems) return false;
        if (Array.isArray(log.retrievedItems)) {
            const realItems = log.retrievedItems.filter(id => id !== '_has_results');
            return realItems.length > 0 || log.retrievedItems.includes('_has_results');
        }
        return false;
    }).length;
 // (ID,)
    const totalItems = logs.reduce((sum, log) => {
        if (!log.retrievedItems) return sum;
        if (Array.isArray(log.retrievedItems)) {
            const realItems = log.retrievedItems.filter(id => id !== '_has_results');
            return sum + realItems.length;
        }
        return sum;
    }, 0);
    const successRate = totalLogs > 0 ? ((successfulLogs / totalLogs) * 100).toFixed(1) : 0;
    
    statsContainer.innerHTML = `
        <div class="retrieval-stat-item">
 <span class="retrieval-stat-label" data-i18n="retrievalLogs.totalRetrievals"></span>
            <span class="retrieval-stat-value">${totalLogs}</span>
        </div>
        <div class="retrieval-stat-item">
 <span class="retrieval-stat-label" data-i18n="retrievalLogs.successRetrievals"></span>
            <span class="retrieval-stat-value text-success">${successfulLogs}</span>
        </div>
        <div class="retrieval-stat-item">
 <span class="retrieval-stat-label" data-i18n="retrievalLogs.successRate"></span>
            <span class="retrieval-stat-value">${successRate}%</span>
        </div>
        <div class="retrieval-stat-item">
 <span class="retrieval-stat-label" data-i18n="retrievalLogs.retrievedItems"></span>
            <span class="retrieval-stat-value">${totalItems}</span>
        </div>
    `;
    if (typeof window.applyTranslations === 'function') {
        window.applyTranslations(statsContainer);
    }
}

// 
function getTimeAgo(timeStr) {
    if (!timeStr) return '';
    
 // ,
    let date;
    if (typeof timeStr === 'string') {
 // (RFC3339/ISO8601)
        date = new Date(timeStr);
        
 // ,
        if (isNaN(date.getTime())) {
 // SQLite: "2006-01-02 15:04:05" 
            const sqliteMatch = timeStr.match(/(\d{4}-\d{2}-\d{2}[\sT]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:[+-]\d{2}:\d{2}|Z)?)/);
            if (sqliteMatch) {
                let timeStr2 = sqliteMatch[1].replace(' ', 'T');
 // ,ZUTC
                if (!timeStr2.includes('Z') && !timeStr2.match(/[+-]\d{2}:\d{2}$/)) {
                    timeStr2 += 'Z';
                }
                date = new Date(timeStr2);
            }
        }
        
 // ,
        if (isNaN(date.getTime())) {
 // "YYYY-MM-DD HH:MM:SS" 
            const match = timeStr.match(/(\d{4})-(\d{2})-(\d{2})[\sT](\d{2}):(\d{2}):(\d{2})/);
            if (match) {
                date = new Date(
                    parseInt(match[1]), 
                    parseInt(match[2]) - 1, 
                    parseInt(match[3]),
                    parseInt(match[4]),
                    parseInt(match[5]),
                    parseInt(match[6])
                );
            }
        }
    } else {
        date = new Date(timeStr);
    }
    
 // 
    if (isNaN(date.getTime())) {
        return formatTime(timeStr);
    }
    
 // (1970,)
    const year = date.getFullYear();
    if (year < 1970 || year > 2100) {
        return formatTime(timeStr);
    }
    
    const now = new Date();
    const diff = now - date;
    
 // (),
 if (diff < 0 || diff > 365 * 24 * 60 * 60 * 1000 * 10) { // 10
        return formatTime(timeStr);
    }
    
    const seconds = Math.floor(diff / 1000);
    const minutes = Math.floor(seconds / 60);
    const hours = Math.floor(minutes / 60);
    const days = Math.floor(hours / 24);
    
 if (days > 0) return `${days}`;
 if (hours > 0) return `${hours}`;
 if (minutes > 0) return `${minutes}`;
 return '';
}

// ID
function truncateId(id) {
    if (!id || id.length <= 16) return id;
    return id.substring(0, 8) + '...' + id.substring(id.length - 8);
}

// 
function filterRetrievalLogs() {
    const conversationId = document.getElementById('retrieval-logs-conversation-id').value.trim();
    const messageId = document.getElementById('retrieval-logs-message-id').value.trim();
    loadRetrievalLogs(conversationId, messageId);
}

// 
function refreshRetrievalLogs() {
    filterRetrievalLogs();
}

// 
async function deleteRetrievalLog(id, index) {
    if (!confirm(_t('retrievalLogs.deleteConfirm'))) {
        return;
    }
    
 // 
    const logCard = document.querySelector(`.retrieval-log-card[data-index="${index}"]`);
    const deleteButton = logCard ? logCard.querySelector('.retrieval-log-delete-btn') : null;
    let originalButtonOpacity = '';
    let originalButtonDisabled = false;
    
 // 
    if (deleteButton) {
        originalButtonOpacity = deleteButton.style.opacity;
        originalButtonDisabled = deleteButton.disabled;
        deleteButton.style.opacity = '0.5';
        deleteButton.style.cursor = 'not-allowed';
        deleteButton.disabled = true;
        
 // 
        const svg = deleteButton.querySelector('svg');
        if (svg) {
            svg.style.animation = 'spin 1s linear infinite';
        }
    }
    
 // UI()
    if (logCard) {
        logCard.style.transition = 'opacity 0.3s ease-out, transform 0.3s ease-out';
        logCard.style.opacity = '0';
        logCard.style.transform = 'translateX(-20px)';
        
 // 
        setTimeout(() => {
            if (logCard.parentElement) {
                logCard.remove();
                
 // (,)
                updateRetrievalStatsAfterDelete();
            }
        }, 300);
    }
    
    try {
        const response = await apiFetch(`/api/knowledge/retrieval-logs/${id}`, {
            method: 'DELETE'
        });
        
        if (!response.ok) {
            const errorData = await response.json().catch(() => ({}));
 throw new Error(errorData.error || '');
        }
        
 // 
 showNotification('✅ !.', 'success');
        
 // 
        if (retrievalLogsData && index >= 0 && index < retrievalLogsData.length) {
            retrievalLogsData.splice(index, 1);
        }
        
 // 
        const conversationId = document.getElementById('retrieval-logs-conversation-id')?.value.trim() || '';
        const messageId = document.getElementById('retrieval-logs-message-id')?.value.trim() || '';
        await loadRetrievalLogs(conversationId, messageId);
        
    } catch (error) {
 console.error(':', error);
        
 // ,
        if (logCard) {
            logCard.style.opacity = '1';
            logCard.style.transform = '';
            logCard.style.transition = '';
        }
        
 // 
        if (deleteButton) {
            deleteButton.style.opacity = originalButtonOpacity || '';
            deleteButton.style.cursor = '';
            deleteButton.disabled = originalButtonDisabled;
            const svg = deleteButton.querySelector('svg');
            if (svg) {
                svg.style.animation = '';
            }
        }
        
        showNotification(_t('retrievalLogs.deleteError') + ': ' + error.message, 'error');
    }
}

// ()
function updateRetrievalStatsAfterDelete() {
    const statsContainer = document.getElementById('retrieval-stats');
    if (!statsContainer) return;
    
    const allLogs = document.querySelectorAll('.retrieval-log-card');
    const totalLogs = allLogs.length;
    
 // 
    const successfulLogs = Array.from(allLogs).filter(card => {
        return card.classList.contains('has-results');
    }).length;
    
 // (,)
    const totalItems = Array.from(allLogs).reduce((sum, card) => {
        const badge = card.querySelector('.retrieval-log-result-badge');
        if (badge && badge.classList.contains('success')) {
            const text = badge.textContent.trim();
            const match = text.match(/(\d+)/);
            if (match) {
                return sum + parseInt(match[1], 10);
            }
 return sum + 1; // ( "Has results" / "")
        }
        return sum;
    }, 0);
    
    const successRate = totalLogs > 0 ? ((successfulLogs / totalLogs) * 100).toFixed(1) : 0;
    
    statsContainer.innerHTML = `
        <div class="retrieval-stat-item">
 <span class="retrieval-stat-label" data-i18n="retrievalLogs.totalRetrievals"></span>
            <span class="retrieval-stat-value">${totalLogs}</span>
        </div>
        <div class="retrieval-stat-item">
 <span class="retrieval-stat-label" data-i18n="retrievalLogs.successRetrievals"></span>
            <span class="retrieval-stat-value text-success">${successfulLogs}</span>
        </div>
        <div class="retrieval-stat-item">
 <span class="retrieval-stat-label" data-i18n="retrievalLogs.successRate"></span>
            <span class="retrieval-stat-value">${successRate}%</span>
        </div>
        <div class="retrieval-stat-item">
 <span class="retrieval-stat-label" data-i18n="retrievalLogs.retrievedItems"></span>
            <span class="retrieval-stat-value">${totalItems}</span>
        </div>
    `;
    if (typeof window.applyTranslations === 'function') {
        window.applyTranslations(statsContainer);
    }
}

// 
async function showRetrievalLogDetails(index) {
    if (!retrievalLogsData || index < 0 || index >= retrievalLogsData.length) {
        showNotification(_t('retrievalLogs.detailError'), 'error');
        return;
    }
    
    const log = retrievalLogsData[index];
    
 // 
    let retrievedItemsDetails = [];
    if (log.retrievedItems && Array.isArray(log.retrievedItems)) {
        const realItemIds = log.retrievedItems.filter(id => id !== '_has_results');
        if (realItemIds.length > 0) {
            try {
 // 
                const itemPromises = realItemIds.map(async (itemId) => {
                    try {
                        const response = await apiFetch(`/api/knowledge/items/${itemId}`);
                        if (response.ok) {
                            return await response.json();
                        }
                        return null;
                    } catch (err) {
 console.error(` ${itemId} :`, err);
                        return null;
                    }
                });
                
                const items = await Promise.all(itemPromises);
                retrievedItemsDetails = items.filter(item => item !== null);
            } catch (err) {
 console.error(':', err);
            }
        }
    }
    
 // 
    showRetrievalLogDetailsModal(log, retrievedItemsDetails);
}

// 
function showRetrievalLogDetailsModal(log, retrievedItems) {
 // 
    let modal = document.getElementById('retrieval-log-details-modal');
    if (!modal) {
        modal = document.createElement('div');
        modal.id = 'retrieval-log-details-modal';
        modal.className = 'modal';
        modal.innerHTML = `
            <div class="modal-content" style="max-width: 900px; max-height: 90vh; overflow-y: auto;">
                <div class="modal-header">
 <h2 data-i18n="retrievalLogs.detailsTitle"></h2>
                    <span class="modal-close" onclick="closeRetrievalLogDetailsModal()">&times;</span>
                </div>
                <div class="modal-body" id="retrieval-log-details-content">
                </div>
                <div class="modal-footer">
 <button class="btn-secondary" onclick="closeRetrievalLogDetailsModal()" data-i18n="common.close"></button>
                </div>
            </div>
        `;
        if (typeof window.applyTranslations === 'function') {
            window.applyTranslations(modal);
        }
        document.body.appendChild(modal);
    }
    
 // 
    const content = document.getElementById('retrieval-log-details-content');
    const timeAgo = getTimeAgo(log.createdAt);
    const fullTime = formatTime(log.createdAt);
    
    let itemsHtml = '';
    if (retrievedItems.length > 0) {
        itemsHtml = retrievedItems.map((item, idx) => {
 // 
            let preview = item.content || '';
            preview = preview.replace(/^#+\s+/gm, '');
            preview = preview.replace(/```[\s\S]*?```/g, '');
            preview = preview.replace(/`[^`]+`/g, '');
            preview = preview.replace(/\[([^\]]+)\]\([^\)]+\)/g, '$1');
            preview = preview.replace(/\n+/g, ' ').replace(/\s+/g, ' ').trim();
            const previewText = preview.length > 200 ? preview.substring(0, 200) + '...' : preview;
            
            return `
                <div class="retrieval-detail-item-card" style="margin-bottom: 16px; padding: 16px; border: 1px solid var(--border-color); border-radius: 8px; background: var(--bg-secondary);">
                    <div style="display: flex; justify-content: space-between; align-items: start; margin-bottom: 8px;">
                        <h4 style="margin: 0; color: var(--text-primary);">${idx + 1}. ${escapeHtml(item.title || _t('retrievalLogs.untitled'))}</h4>
                        <span style="font-size: 0.875rem; color: var(--text-secondary);">${escapeHtml(item.category || _t('retrievalLogs.uncategorized'))}</span>
                    </div>
                    ${item.filePath ? `<div style="font-size: 0.875rem; color: var(--text-muted); margin-bottom: 8px;">📁 ${escapeHtml(item.filePath)}</div>` : ''}
                    <div style="font-size: 0.875rem; color: var(--text-secondary); line-height: 1.6;">
                        ${escapeHtml(previewText || _t('retrievalLogs.noContentPreview'))}
                    </div>
                </div>
            `;
        }).join('');
    } else {
        itemsHtml = '<div style="padding: 16px; text-align: center; color: var(--text-muted);">' + _t('retrievalLogs.noItemDetails') + '</div>';
    }
    
    content.innerHTML = `
        <div style="display: flex; flex-direction: column; gap: 20px;">
            <div class="retrieval-detail-section">
                <h3 style="margin: 0 0 12px 0; font-size: 1.125rem; color: var(--text-primary);">${_t('retrievalLogs.queryInfo')}</h3>
                <div style="padding: 12px; background: var(--bg-secondary); border-radius: 6px; border-left: 3px solid var(--accent-color);">
                    <div style="font-weight: 500; margin-bottom: 8px; color: var(--text-primary);">${_t('retrievalLogs.queryContent')}</div>
                    <div style="color: var(--text-primary); line-height: 1.6; word-break: break-word;">${escapeHtml(log.query || _t('retrievalLogs.noQuery'))}</div>
                </div>
            </div>
            
            <div class="retrieval-detail-section">
                <h3 style="margin: 0 0 12px 0; font-size: 1.125rem; color: var(--text-primary);">${_t('retrievalLogs.retrievalInfo')}</h3>
                <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 12px;">
                    ${log.riskType ? `
                        <div style="padding: 12px; background: var(--bg-secondary); border-radius: 6px;">
                            <div style="font-size: 0.875rem; color: var(--text-secondary); margin-bottom: 4px;">${_t('retrievalLogs.riskType')}</div>
                            <div style="font-weight: 500; color: var(--text-primary);">${escapeHtml(log.riskType)}</div>
                        </div>
                    ` : ''}
                    <div style="padding: 12px; background: var(--bg-secondary); border-radius: 6px;">
                        <div style="font-size: 0.875rem; color: var(--text-secondary); margin-bottom: 4px;">${_t('retrievalLogs.retrievalTime')}</div>
                        <div style="font-weight: 500; color: var(--text-primary);" title="${fullTime}">${timeAgo}</div>
                    </div>
                    <div style="padding: 12px; background: var(--bg-secondary); border-radius: 6px;">
                        <div style="font-size: 0.875rem; color: var(--text-secondary); margin-bottom: 4px;">${_t('retrievalLogs.retrievalResult')}</div>
                        <div style="font-weight: 500; color: var(--text-primary);">${_t('retrievalLogs.itemsCount', { count: retrievedItems.length })}</div>
                    </div>
                </div>
            </div>
            
            ${log.conversationId || log.messageId ? `
                <div class="retrieval-detail-section">
                    <h3 style="margin: 0 0 12px 0; font-size: 1.125rem; color: var(--text-primary);">${_t('retrievalLogs.relatedInfo')}</h3>
                    <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 12px;">
                        ${log.conversationId ? `
                            <div style="padding: 12px; background: var(--bg-secondary); border-radius: 6px;">
                                <div style="font-size: 0.875rem; color: var(--text-secondary); margin-bottom: 4px;">${_t('retrievalLogs.conversationId')}</div>
                                <code style="font-size: 0.8125rem; color: var(--text-primary); word-break: break-all; cursor: pointer;" 
 onclick="navigator.clipboard.writeText('${escapeHtml(log.conversationId)}'); this.title='!'; setTimeout(() => this.title='', 2000);" 
 title="">${escapeHtml(log.conversationId)}</code>
                            </div>
                        ` : ''}
                        ${log.messageId ? `
                            <div style="padding: 12px; background: var(--bg-secondary); border-radius: 6px;">
                                <div style="font-size: 0.875rem; color: var(--text-secondary); margin-bottom: 4px;">${_t('retrievalLogs.messageId')}</div>
                                <code style="font-size: 0.8125rem; color: var(--text-primary); word-break: break-all; cursor: pointer;" 
 onclick="navigator.clipboard.writeText('${escapeHtml(log.messageId)}'); this.title='!'; setTimeout(() => this.title='', 2000);" 
 title="">${escapeHtml(log.messageId)}</code>
                            </div>
                        ` : ''}
                    </div>
                </div>
            ` : ''}
            
            <div class="retrieval-detail-section">
 <h3 style="margin: 0 0 12px 0; font-size: 1.125rem; color: var(--text-primary);"> (${retrievedItems.length})</h3>
                ${itemsHtml}
            </div>
        </div>
    `;
    
    modal.style.display = 'block';
}

// 
function closeRetrievalLogDetailsModal() {
    const modal = document.getElementById('retrieval-log-details-modal');
    if (modal) {
        modal.style.display = 'none';
    }
}

// 
window.addEventListener('click', function(event) {
    const modal = document.getElementById('retrieval-log-details-modal');
    if (event.target === modal) {
        closeRetrievalLogDetailsModal();
    }
});

// ,;"" data-i18n, applyTranslations(document) 
document.addEventListener('languagechange', function () {
    var cur = typeof window.currentPage === 'function' ? window.currentPage() : (window.currentPage || '');
    if (cur === 'knowledge-retrieval-logs') {
        if (retrievalLogsData && retrievalLogsData.length >= 0) {
            renderRetrievalLogs(retrievalLogsData);
        }
    } else if (cur === 'knowledge-management') {
 // "": data-i18n,applyTranslations ; DOM
        var listEl = document.getElementById('knowledge-items-list');
        if (listEl && typeof window.applyTranslations === 'function') {
            window.applyTranslations(listEl);
        }
    }
});

// 
if (typeof switchPage === 'function') {
    const originalSwitchPage = switchPage;
    window.switchPage = function(page) {
        originalSwitchPage(page);
        
        if (page === 'knowledge-management') {
            loadKnowledgeCategories();
            loadKnowledgeItems(knowledgePagination.currentCategory, 1, knowledgePagination.pageSize);
 updateIndexProgress(); // 
        } else if (page === 'knowledge-retrieval-logs') {
            loadRetrievalLogs();
 // 
            if (indexProgressInterval) {
                clearInterval(indexProgressInterval);
                indexProgressInterval = null;
            }
        } else {
 // 
            if (indexProgressInterval) {
                clearInterval(indexProgressInterval);
                indexProgressInterval = null;
            }
        }
    };
}

// 
window.addEventListener('beforeunload', function() {
    if (indexProgressInterval) {
        clearInterval(indexProgressInterval);
        indexProgressInterval = null;
    }
});

// 
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

function formatTime(timeStr) {
    if (!timeStr) return '';
    
 // ,
    let date;
    if (typeof timeStr === 'string') {
 // (RFC3339/ISO8601)
        date = new Date(timeStr);
        
 // ,
        if (isNaN(date.getTime())) {
 // SQLite: "2006-01-02 15:04:05" 
            const sqliteMatch = timeStr.match(/(\d{4}-\d{2}-\d{2}[\sT]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:[+-]\d{2}:\d{2}|Z)?)/);
            if (sqliteMatch) {
                let timeStr2 = sqliteMatch[1].replace(' ', 'T');
 // ,ZUTC
                if (!timeStr2.includes('Z') && !timeStr2.match(/[+-]\d{2}:\d{2}$/)) {
                    timeStr2 += 'Z';
                }
                date = new Date(timeStr2);
            }
        }
        
 // ,
        if (isNaN(date.getTime())) {
 // "YYYY-MM-DD HH:MM:SS" 
            const match = timeStr.match(/(\d{4})-(\d{2})-(\d{2})[\sT](\d{2}):(\d{2}):(\d{2})/);
            if (match) {
                date = new Date(
                    parseInt(match[1]), 
                    parseInt(match[2]) - 1, 
                    parseInt(match[3]),
                    parseInt(match[4]),
                    parseInt(match[5]),
                    parseInt(match[6])
                );
            }
        }
    } else {
        date = new Date(timeStr);
    }
    
 // ,
    if (isNaN(date.getTime())) {
 // 
        if (typeof timeStr === 'string' && (timeStr.includes('0001-01-01') || timeStr.startsWith('0001'))) {
            return '';
        }
 console.warn(':', timeStr);
        return '';
    }
    
 // (1970,)
    const year = date.getFullYear();
    if (year < 1970 || year > 2100) {
 // (0001-01-01),,
        if (year === 1) {
            return '';
        }
 console.warn(':', timeStr, ':', date);
        return '';
    }
    
    return date.toLocaleString('zh-CN', {
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
        hour12: false
    });
}

// 
function showNotification(message, type = 'info') {
 // (),
    if (typeof window.showNotification === 'function' && window.showNotification !== showNotification) {
        window.showNotification(message, type);
        return;
    }
    
 // toast
    showToastNotification(message, type);
}

// Toast
function showToastNotification(message, type = 'info') {
 // ()
    let container = document.getElementById('toast-notification-container');
    if (!container) {
        container = document.createElement('div');
        container.id = 'toast-notification-container';
        container.style.cssText = `
            position: fixed;
            top: 20px;
            right: 20px;
            z-index: 10000;
            display: flex;
            flex-direction: column;
            gap: 12px;
            pointer-events: none;
        `;
        document.body.appendChild(container);
    }
    
 // 
    const toast = document.createElement('div');
    toast.className = `toast-notification toast-${type}`;
    
 // 
    const typeStyles = {
        success: {
            background: '#28a745',
            color: '#fff',
            icon: '✅'
        },
        error: {
            background: '#dc3545',
            color: '#fff',
            icon: '❌'
        },
        info: {
            background: '#17a2b8',
            color: '#fff',
            icon: 'ℹ️'
        },
        warning: {
            background: '#ffc107',
            color: '#000',
            icon: '⚠️'
        }
    };
    
    const style = typeStyles[type] || typeStyles.info;
    
    toast.style.cssText = `
        background: ${style.background};
        color: ${style.color};
        padding: 14px 20px;
        border-radius: 8px;
        box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
        min-width: 300px;
        max-width: 500px;
        pointer-events: auto;
        animation: slideInRight 0.3s ease-out;
        display: flex;
        align-items: center;
        gap: 12px;
        font-size: 0.9375rem;
        line-height: 1.5;
        word-wrap: break-word;
    `;
    
    toast.innerHTML = `
        <span style="font-size: 1.2em; flex-shrink: 0;">${style.icon}</span>
        <span style="flex: 1;">${escapeHtml(message)}</span>
        <button onclick="this.parentElement.remove()" style="
            background: transparent;
            border: none;
            color: ${style.color};
            cursor: pointer;
            font-size: 1.2em;
            padding: 0;
            margin-left: 8px;
            opacity: 0.7;
            flex-shrink: 0;
            width: 24px;
            height: 24px;
            display: flex;
            align-items: center;
            justify-content: center;
        " onmouseover="this.style.opacity='1'" onmouseout="this.style.opacity='0.7'">×</button>
    `;
    
    container.appendChild(toast);
    
 // (5,7,4)
    const duration = type === 'success' ? 5000 : type === 'error' ? 7000 : 4000;
    setTimeout(() => {
        if (toast.parentElement) {
            toast.style.animation = 'slideOutRight 0.3s ease-out';
            setTimeout(() => {
                if (toast.parentElement) {
                    toast.remove();
                }
            }, 300);
        }
    }, duration);
}

// CSS()
if (!document.getElementById('toast-notification-styles')) {
    const style = document.createElement('style');
    style.id = 'toast-notification-styles';
    style.textContent = `
        @keyframes slideInRight {
            from {
                transform: translateX(100%);
                opacity: 0;
            }
            to {
                transform: translateX(0);
                opacity: 1;
            }
        }
        @keyframes slideOutRight {
            from {
                transform: translateX(0);
                opacity: 1;
            }
            to {
                transform: translateX(100%);
                opacity: 0;
            }
        }
    `;
    document.head.appendChild(style);
}

// 
window.addEventListener('click', function(event) {
    const modal = document.getElementById('knowledge-item-modal');
    if (event.target === modal) {
        closeKnowledgeItemModal();
    }
});

// ()
function switchToSettings() {
    if (typeof switchPage === 'function') {
        switchPage('settings');
 // ,
        setTimeout(() => {
            if (typeof switchSettingsSection === 'function') {
 // ()
                const knowledgeSection = document.querySelector('[data-section="knowledge"]');
                if (knowledgeSection) {
                    switchSettingsSection('knowledge');
                } else {
 // ,
                    switchSettingsSection('basic');
 // 
                    setTimeout(() => {
                        const knowledgeEnabledCheckbox = document.getElementById('knowledge-enabled');
                        if (knowledgeEnabledCheckbox) {
                            knowledgeEnabledCheckbox.scrollIntoView({ behavior: 'smooth', block: 'center' });
 // 
                            knowledgeEnabledCheckbox.parentElement.style.transition = 'background-color 0.3s';
                            knowledgeEnabledCheckbox.parentElement.style.backgroundColor = '#e3f2fd';
                            setTimeout(() => {
                                knowledgeEnabledCheckbox.parentElement.style.backgroundColor = '';
                            }, 2000);
                        }
                    }, 300);
                }
            }
        }, 100);
    }
}

// 
document.addEventListener('DOMContentLoaded', function() {
    const wrapper = document.getElementById('knowledge-category-filter-wrapper');
    const trigger = document.getElementById('knowledge-category-filter-trigger');
    
    if (wrapper && trigger) {
 // /
        trigger.addEventListener('click', function(e) {
            e.stopPropagation();
            wrapper.classList.toggle('open');
        });
        
 // 
        document.addEventListener('click', function(e) {
            if (!wrapper.contains(e.target)) {
                wrapper.classList.remove('open');
            }
        });
        
 // 
        const dropdown = document.getElementById('knowledge-category-filter-dropdown');
        if (dropdown) {
 // ""
            const defaultOption = dropdown.querySelector('.custom-select-option[data-value=""]');
            if (defaultOption) {
                defaultOption.classList.add('selected');
            }
            
            dropdown.addEventListener('click', function(e) {
                const option = e.target.closest('.custom-select-option');
                if (option) {
 // 
                    dropdown.querySelectorAll('.custom-select-option').forEach(opt => {
                        opt.classList.remove('selected');
                    });
 // 
                    option.classList.add('selected');
                }
            });
        }
    }
});

