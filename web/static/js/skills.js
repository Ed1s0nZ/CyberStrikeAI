// Skills管理相关功能
function _t(key, opts) {
    return typeof window.t === 'function' ? window.t(key, opts) : key;
}
let skillsList = [];
let currentEditingSkillName = null;
let isSavingSkill = false; // 防止重复提交
let skillsSearchKeyword = '';
let skillsSearchTimeout = null; // 搜索防抖定时器
let skillsAutoRefreshTimer = null;
let isAutoRefreshingSkills = false;
const SKILLS_AUTO_REFRESH_INTERVAL_MS = 5000;
let skillsPagination = {
    currentPage: 1,
    pageSize: 20, // 每页20条（默认值，实际从localStorage读取）
    total: 0
};
let skillsStats = {
    total: 0,
    totalCalls: 0,
    totalSuccess: 0,
    totalFailed: 0,
    skillsDir: '',
    stats: []
};

function isSkillsManagementPageActive() {
    const page = document.getElementById('page-skills-management');
    return !!(page && page.classList.contains('active'));
}

function shouldSkipSkillsAutoRefresh() {
    if (isSavingSkill || currentEditingSkillName) {
        return true;
    }

    const modal = document.getElementById('skill-modal');
    if (modal && modal.style.display === 'flex') {
        return true;
    }

    const searchInput = document.getElementById('skills-search');
    if (skillsSearchKeyword || (searchInput && searchInput.value.trim())) {
        return true;
    }

    return false;
}

function startSkillsAutoRefresh() {
    if (skillsAutoRefreshTimer) return;

    skillsAutoRefreshTimer = setInterval(async () => {
        if (!isSkillsManagementPageActive() || shouldSkipSkillsAutoRefresh()) {
            return;
        }
        if (isAutoRefreshingSkills) {
            return;
        }

        isAutoRefreshingSkills = true;
        try {
            await loadSkills(skillsPagination.currentPage, skillsPagination.pageSize);
        } finally {
            isAutoRefreshingSkills = false;
        }
    }, SKILLS_AUTO_REFRESH_INTERVAL_MS);
}

// 获取保存的每页显示数量
function getSkillsPageSize() {
    try {
        const saved = localStorage.getItem('skillsPageSize');
        if (saved) {
            const size = parseInt(saved);
            if ([10, 20, 50, 100].includes(size)) {
                return size;
            }
        }
    } catch (e) {
        console.warn('无法从localStorage读取分页设置:', e);
    }
    return 20; // 默认20
}

// 初始化分页设置
function initSkillsPagination() {
    const savedPageSize = getSkillsPageSize();
    skillsPagination.pageSize = savedPageSize;
}

// 加载skills列表（支持分页）
async function loadSkills(page = 1, pageSize = null) {
    try {
        // 如果没有指定pageSize，使用保存的值或默认值
        if (pageSize === null) {
            pageSize = getSkillsPageSize();
        }
        
        // 更新分页状态（确保使用正确的pageSize）
        skillsPagination.currentPage = page;
        skillsPagination.pageSize = pageSize;
        
        // 清空搜索关键词（正常分页加载时）
        skillsSearchKeyword = '';
        const searchInput = document.getElementById('skills-search');
        if (searchInput) {
            searchInput.value = '';
        }
        
        // 构建URL（支持分页）
        const offset = (page - 1) * pageSize;
        const url = `/api/skills?limit=${pageSize}&offset=${offset}`;
        
        const response = await apiFetch(url);
        if (!response.ok) {
            throw new Error(_t('skills.loadListFailed'));
        }
        const data = await response.json();
        skillsList = data.skills || [];
        skillsPagination.total = data.total || 0;
        
        renderSkillsList();
        renderSkillsPagination();
        updateSkillsManagementStats();
    } catch (error) {
        console.error('加载skills列表失败:', error);
        showNotification(_t('skills.loadListFailed') + ': ' + error.message, 'error');
        const skillsListEl = document.getElementById('skills-list');
        if (skillsListEl) {
            skillsListEl.innerHTML = '<div class="empty-state">' + _t('skills.loadFailedShort') + ': ' + escapeHtml(error.message) + '</div>';
        }
    }
}

// 渲染skills列表
function renderSkillsList() {
    const skillsListEl = document.getElementById('skills-list');
    if (!skillsListEl) return;

    // 后端已经完成搜索过滤，直接使用skillsList
    const filteredSkills = skillsList;

    if (filteredSkills.length === 0) {
        skillsListEl.innerHTML = '<div class="empty-state">' + 
            (skillsSearchKeyword ? _t('skills.noMatch') : _t('skills.noSkills')) + 
            '</div>';
        // 搜索时隐藏分页
        const paginationContainer = document.getElementById('skills-pagination');
        if (paginationContainer) {
            paginationContainer.innerHTML = '';
        }
        return;
    }

    skillsListEl.innerHTML = filteredSkills.map(skill => {
        return `
            <div class="skill-card">
                <div class="skill-card-header">
                    <h3 class="skill-card-title">${escapeHtml(skill.name || '')}</h3>
                    <div class="skill-card-description">${escapeHtml(skill.description || _t('skills.noDescription'))}</div>
                </div>
                <div class="skill-card-actions">
                    <button class="btn-secondary btn-small" onclick="viewSkill('${escapeHtml(skill.name)}')">${_t('common.view')}</button>
                    <button class="btn-secondary btn-small" onclick="editSkill('${escapeHtml(skill.name)}')">${_t('common.edit')}</button>
                    <button class="btn-secondary btn-small btn-danger" onclick="deleteSkill('${escapeHtml(skill.name)}')">${_t('common.delete')}</button>
                </div>
            </div>
        `;
    }).join('');
    
    // 确保列表容器可以滚动，分页栏可见
    // 使用 setTimeout 确保 DOM 更新完成后再检查
    setTimeout(() => {
        const paginationContainer = document.getElementById('skills-pagination');
        if (paginationContainer && !skillsSearchKeyword) {
            // 确保分页栏可见
            paginationContainer.style.display = 'block';
            paginationContainer.style.visibility = 'visible';
        }
    }, 0);
}

// 渲染分页组件（参考MCP管理页面样式）
function renderSkillsPagination() {
    const paginationContainer = document.getElementById('skills-pagination');
    if (!paginationContainer) return;
    
    const total = skillsPagination.total;
    const pageSize = skillsPagination.pageSize;
    const currentPage = skillsPagination.currentPage;
    const totalPages = Math.ceil(total / pageSize);
    
    // 即使只有一页也显示分页信息（参考MCP样式）
    if (total === 0) {
        paginationContainer.innerHTML = '';
        return;
    }
    
    // 计算显示范围
    const start = total === 0 ? 0 : (currentPage - 1) * pageSize + 1;
    const end = total === 0 ? 0 : Math.min(currentPage * pageSize, total);
    
    let paginationHTML = '<div class="pagination">';
    
    const paginationShowText = _t('skillsPage.paginationShow', { start, end, total });
    const perPageLabelText = _t('skillsPage.perPageLabel');
    const firstPageText = _t('skillsPage.firstPage');
    const prevPageText = _t('skillsPage.prevPage');
    const pageOfText = _t('skillsPage.pageOf', { current: currentPage, total: totalPages || 1 });
    const nextPageText = _t('skillsPage.nextPage');
    const lastPageText = _t('skillsPage.lastPage');
    // 左侧：显示范围信息和每页数量选择器（参考MCP样式）
    paginationHTML += `
        <div class="pagination-info">
            <span>${escapeHtml(paginationShowText)}</span>
            <label class="pagination-page-size">
                ${escapeHtml(perPageLabelText)}
                <select id="skills-page-size-pagination" onchange="changeSkillsPageSize()">
                    <option value="10" ${pageSize === 10 ? 'selected' : ''}>10</option>
                    <option value="20" ${pageSize === 20 ? 'selected' : ''}>20</option>
                    <option value="50" ${pageSize === 50 ? 'selected' : ''}>50</option>
                    <option value="100" ${pageSize === 100 ? 'selected' : ''}>100</option>
                </select>
            </label>
        </div>
    `;
    
    // 右侧：分页按钮（参考MCP样式：首页、上一页、第X/Y页、下一页、末页）
    paginationHTML += `
        <div class="pagination-controls">
            <button class="btn-secondary" onclick="loadSkills(1, ${pageSize})" ${currentPage === 1 || total === 0 ? 'disabled' : ''}>${escapeHtml(firstPageText)}</button>
            <button class="btn-secondary" onclick="loadSkills(${currentPage - 1}, ${pageSize})" ${currentPage === 1 || total === 0 ? 'disabled' : ''}>${escapeHtml(prevPageText)}</button>
            <span class="pagination-page">${escapeHtml(pageOfText)}</span>
            <button class="btn-secondary" onclick="loadSkills(${currentPage + 1}, ${pageSize})" ${currentPage >= totalPages || total === 0 ? 'disabled' : ''}>${escapeHtml(nextPageText)}</button>
            <button class="btn-secondary" onclick="loadSkills(${totalPages || 1}, ${pageSize})" ${currentPage >= totalPages || total === 0 ? 'disabled' : ''}>${escapeHtml(lastPageText)}</button>
        </div>
    `;
    
    paginationHTML += '</div>';
    
    paginationContainer.innerHTML = paginationHTML;
    
    // 确保分页组件与列表内容区域对齐（不包括滚动条）
    function alignPaginationWidth() {
        const skillsList = document.getElementById('skills-list');
        if (skillsList && paginationContainer) {
            // 确保分页容器始终可见
            paginationContainer.style.display = '';
            paginationContainer.style.visibility = 'visible';
            paginationContainer.style.opacity = '1';
            
            // 获取列表的实际内容宽度（不包括滚动条）
            const listClientWidth = skillsList.clientWidth; // 可视区域宽度（不包括滚动条）
            const listScrollHeight = skillsList.scrollHeight; // 内容总高度
            const listClientHeight = skillsList.clientHeight; // 可视区域高度
            const hasScrollbar = listScrollHeight > listClientHeight;
            
            // 如果列表有垂直滚动条，分页组件应该与列表内容区域对齐（clientWidth）
            // 如果没有滚动条，使用100%宽度
            if (hasScrollbar && listClientWidth > 0) {
                // 分页组件应该与列表内容区域对齐，不包括滚动条
                paginationContainer.style.width = `${listClientWidth}px`;
            } else {
                // 如果没有滚动条，使用100%宽度
                paginationContainer.style.width = '100%';
            }
        }
    }
    
    // 立即执行一次
    alignPaginationWidth();
    
    // 监听窗口大小变化和列表内容变化
    const resizeObserver = new ResizeObserver(() => {
        alignPaginationWidth();
    });
    
    const skillsList = document.getElementById('skills-list');
    if (skillsList) {
        resizeObserver.observe(skillsList);
    }
    
    // 确保分页容器始终可见（防止被隐藏）
    paginationContainer.style.display = 'block';
    paginationContainer.style.visibility = 'visible';
}

// 改变每页显示数量
async function changeSkillsPageSize() {
    const pageSizeSelect = document.getElementById('skills-page-size-pagination');
    if (!pageSizeSelect) return;
    
    const newPageSize = parseInt(pageSizeSelect.value);
    if (isNaN(newPageSize) || newPageSize <= 0) return;
    
    // 保存到localStorage
    try {
        localStorage.setItem('skillsPageSize', newPageSize.toString());
    } catch (e) {
        console.warn('无法保存分页设置到localStorage:', e);
    }
    
    // 更新分页状态
    skillsPagination.pageSize = newPageSize;
    
    // 重新计算当前页（确保不超出范围）
    const totalPages = Math.ceil(skillsPagination.total / newPageSize);
    const currentPage = Math.min(skillsPagination.currentPage, totalPages || 1);
    skillsPagination.currentPage = currentPage;
    
    // 重新加载数据
    await loadSkills(currentPage, newPageSize);
}

// 更新skills管理统计信息
function updateSkillsManagementStats() {
    const statsEl = document.getElementById('skills-management-stats');
    if (!statsEl) return;

    const totalEl = statsEl.querySelector('.skill-stat-value');
    if (totalEl) {
        totalEl.textContent = skillsPagination.total;
    }
}

// 搜索skills
function handleSkillsSearchInput() {
    clearTimeout(skillsSearchTimeout);
    skillsSearchTimeout = setTimeout(() => {
        searchSkills();
    }, 300);
}

async function searchSkills() {
    const searchInput = document.getElementById('skills-search');
    if (!searchInput) return;
    
    skillsSearchKeyword = searchInput.value.trim();
    const clearBtn = document.getElementById('skills-search-clear');
    if (clearBtn) {
        clearBtn.style.display = skillsSearchKeyword ? 'block' : 'none';
    }
    
    if (skillsSearchKeyword) {
        // 有搜索关键词时，使用后端搜索API（加载所有匹配结果，不分页）
        try {
            const response = await apiFetch(`/api/skills?search=${encodeURIComponent(skillsSearchKeyword)}&limit=10000&offset=0`);
            if (!response.ok) {
                throw new Error(_t('skills.loadListFailed'));
            }
            const data = await response.json();
            skillsList = data.skills || [];
            skillsPagination.total = data.total || 0;
            renderSkillsList();
            // 搜索时隐藏分页
            const paginationContainer = document.getElementById('skills-pagination');
            if (paginationContainer) {
                paginationContainer.innerHTML = '';
            }
            // 更新统计信息（显示搜索结果数量）
            updateSkillsManagementStats();
        } catch (error) {
            console.error('搜索skills失败:', error);
            showNotification(_t('skills.searchFailed') + ': ' + error.message, 'error');
        }
    } else {
        // 没有搜索关键词时，恢复分页加载
        await loadSkills(1, skillsPagination.pageSize);
    }
}

// 清除skills搜索
function clearSkillsSearch() {
    const searchInput = document.getElementById('skills-search');
    if (searchInput) {
        searchInput.value = '';
    }
    skillsSearchKeyword = '';
    const clearBtn = document.getElementById('skills-search-clear');
    if (clearBtn) {
        clearBtn.style.display = 'none';
    }
    // 恢复分页加载
    loadSkills(1, skillsPagination.pageSize);
}

// 刷新skills
async function refreshSkills() {
    await loadSkills(skillsPagination.currentPage, skillsPagination.pageSize);
    showNotification(_t('skills.refreshed'), 'success');
}

// 显示添加skill模态框
function showAddSkillModal() {
    const modal = document.getElementById('skill-modal');
    if (!modal) return;

    document.getElementById('skill-modal-title').textContent = _t('skills.addSkill');
    document.getElementById('skill-name').value = '';
    document.getElementById('skill-name').disabled = false;
    document.getElementById('skill-description').value = '';
    document.getElementById('skill-content').value = '';
    
    modal.style.display = 'flex';
}

// 编辑skill
async function editSkill(skillName) {
    try {
        const response = await apiFetch(`/api/skills/${encodeURIComponent(skillName)}`);
        if (!response.ok) {
            throw new Error(_t('skills.loadDetailFailed'));
        }
        const data = await response.json();
        const skill = data.skill;

        const modal = document.getElementById('skill-modal');
        if (!modal) return;

        document.getElementById('skill-modal-title').textContent = _t('skills.editSkill');
        document.getElementById('skill-name').value = skill.name;
        document.getElementById('skill-name').disabled = true; // 编辑时不允许修改名称
        document.getElementById('skill-description').value = skill.description || '';
        document.getElementById('skill-content').value = skill.content || '';
        
        currentEditingSkillName = skillName;
        modal.style.display = 'flex';
    } catch (error) {
        console.error('加载skill详情失败:', error);
        showNotification(_t('skills.loadDetailFailed') + ': ' + error.message, 'error');
    }
}

// 查看skill
async function viewSkill(skillName) {
    try {
        const response = await apiFetch(`/api/skills/${encodeURIComponent(skillName)}`);
        if (!response.ok) {
            throw new Error(_t('skills.loadDetailFailed'));
        }
        const data = await response.json();
        const skill = data.skill;

        // 创建查看模态框
        const modal = document.createElement('div');
        modal.className = 'modal';
        modal.id = 'skill-view-modal';
        const viewTitle = _t('skills.viewSkillTitle', { name: skill.name });
        const descLabel = _t('skills.descriptionLabel');
        const pathLabel = _t('skills.pathLabel');
        const modTimeLabel = _t('skills.modTimeLabel');
        const contentLabel = _t('skills.contentLabel');
        const closeBtn = _t('common.close');
        const editBtn = _t('common.edit');
        modal.innerHTML = `
            <div class="modal-content" style="max-width: 900px; max-height: 90vh;">
                <div class="modal-header">
                    <h2>${escapeHtml(viewTitle)}</h2>
                    <span class="modal-close" onclick="closeSkillViewModal()">&times;</span>
                </div>
                <div class="modal-body" style="overflow-y: auto; max-height: calc(90vh - 120px);">
                    ${skill.description ? `<div style="margin-bottom: 16px;"><strong>${escapeHtml(descLabel)}</strong> ${escapeHtml(skill.description)}</div>` : ''}
                    <div style="margin-bottom: 8px;"><strong>${escapeHtml(pathLabel)}</strong> ${escapeHtml(skill.path || '')}</div>
                    <div style="margin-bottom: 16px;"><strong>${escapeHtml(modTimeLabel)}</strong> ${escapeHtml(skill.mod_time || '')}</div>
                    <div style="margin-bottom: 8px;"><strong>${escapeHtml(contentLabel)}</strong></div>
                    <pre style="background: #f5f5f5; padding: 16px; border-radius: 4px; overflow-x: auto; white-space: pre-wrap; word-wrap: break-word;">${escapeHtml(skill.content || '')}</pre>
                </div>
                <div class="modal-footer">
                    <button class="btn-secondary" onclick="closeSkillViewModal()">${escapeHtml(closeBtn)}</button>
                    <button class="btn-primary" onclick="editSkill('${escapeHtml(skill.name)}'); closeSkillViewModal();">${escapeHtml(editBtn)}</button>
                </div>
            </div>
        `;
        document.body.appendChild(modal);
        modal.style.display = 'flex';
    } catch (error) {
        console.error('查看skill失败:', error);
        showNotification(_t('skills.viewFailed') + ': ' + error.message, 'error');
    }
}

// 关闭查看模态框
function closeSkillViewModal() {
    const modal = document.getElementById('skill-view-modal');
    if (modal) {
        modal.remove();
    }
}

// 关闭skill模态框
function closeSkillModal() {
    const modal = document.getElementById('skill-modal');
    if (modal) {
        modal.style.display = 'none';
        currentEditingSkillName = null;
    }
}

// 保存skill
async function saveSkill() {
    if (isSavingSkill) return;

    const name = document.getElementById('skill-name').value.trim();
    const description = document.getElementById('skill-description').value.trim();
    const content = document.getElementById('skill-content').value.trim();

    if (!name) {
        showNotification(_t('skills.nameRequired'), 'error');
        return;
    }

    if (!content) {
        showNotification(_t('skills.contentRequired'), 'error');
        return;
    }

    // 验证skill名称
    if (!/^[a-zA-Z0-9_-]+$/.test(name)) {
        showNotification(_t('skills.nameInvalid'), 'error');
        return;
    }

    isSavingSkill = true;
    const saveBtn = document.querySelector('#skill-modal .btn-primary');
    if (saveBtn) {
        saveBtn.disabled = true;
        saveBtn.textContent = _t('skills.saving');
    }

    try {
        const isEdit = !!currentEditingSkillName;
        const url = isEdit ? `/api/skills/${encodeURIComponent(currentEditingSkillName)}` : '/api/skills';
        const method = isEdit ? 'PUT' : 'POST';

        const response = await apiFetch(url, {
            method: method,
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                name: name,
                description: description,
                content: content
            })
        });

        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || _t('skills.saveFailed'));
        }

        showNotification(isEdit ? _t('skills.saveSuccess') : _t('skills.createdSuccess'), 'success');
        closeSkillModal();
        await loadSkills(skillsPagination.currentPage, skillsPagination.pageSize);
    } catch (error) {
        console.error('保存skill失败:', error);
        showNotification(_t('skills.saveFailed') + ': ' + error.message, 'error');
    } finally {
        isSavingSkill = false;
        if (saveBtn) {
            saveBtn.disabled = false;
            saveBtn.textContent = _t('common.save');
        }
    }
}

// 删除skill
async function deleteSkill(skillName) {
    // 先检查是否有角色绑定了该skill
    let boundRoles = [];
    try {
        const checkResponse = await apiFetch(`/api/skills/${encodeURIComponent(skillName)}/bound-roles`);
        if (checkResponse.ok) {
            const checkData = await checkResponse.json();
            boundRoles = checkData.bound_roles || [];
        }
    } catch (error) {
        console.warn('检查skill绑定失败:', error);
        // 如果检查失败，继续执行删除流程
    }

    // 构建确认消息
    let confirmMessage = _t('skills.deleteConfirm', { name: skillName });
    if (boundRoles.length > 0) {
        const rolesList = boundRoles.join('、');
        confirmMessage = _t('skills.deleteConfirmWithRoles', { name: skillName, count: boundRoles.length, roles: rolesList });
    }

    if (!confirm(confirmMessage)) {
        return;
    }

    try {
        const response = await apiFetch(`/api/skills/${encodeURIComponent(skillName)}`, {
            method: 'DELETE'
        });

        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || _t('skills.deleteFailed'));
        }

        const data = await response.json();
        let successMessage = _t('skills.deleteSuccess');
        if (data.affected_roles && data.affected_roles.length > 0) {
            const rolesList = data.affected_roles.join('、');
            successMessage = _t('skills.deleteSuccessWithRoles', { count: data.affected_roles.length, roles: rolesList });
        }
        showNotification(successMessage, 'success');
        
        // 如果当前页没有数据了，回到上一页
        const currentPage = skillsPagination.currentPage;
        const totalAfterDelete = skillsPagination.total - 1;
        const totalPages = Math.ceil(totalAfterDelete / skillsPagination.pageSize);
        const pageToLoad = currentPage > totalPages && totalPages > 0 ? totalPages : currentPage;
        await loadSkills(pageToLoad, skillsPagination.pageSize);
    } catch (error) {
        console.error('删除skill失败:', error);
        showNotification(_t('skills.deleteFailed') + ': ' + error.message, 'error');
    }
}

// ==================== Skills状态监控相关函数 ====================

// 加载skills监控数据
async function loadSkillsMonitor() {
    try {
        const response = await apiFetch('/api/skills/stats');
        if (!response.ok) {
            throw new Error(_t('skills.loadStatsFailed'));
        }
        const data = await response.json();
        
        skillsStats = {
            total: data.total_skills || 0,
            totalCalls: data.total_calls || 0,
            totalSuccess: data.total_success || 0,
            totalFailed: data.total_failed || 0,
            skillsDir: data.skills_dir || '',
            stats: data.stats || []
        };

        renderSkillsMonitor();
    } catch (error) {
        console.error('加载skills监控数据失败:', error);
        showNotification(_t('skills.loadStatsFailed') + ': ' + error.message, 'error');
        const statsEl = document.getElementById('skills-stats');
        if (statsEl) {
            statsEl.innerHTML = '<div class="monitor-error">' + _t('skills.loadStatsErrorShort') + ': ' + escapeHtml(error.message) + '</div>';
        }
        const monitorListEl = document.getElementById('skills-monitor-list');
        if (monitorListEl) {
            monitorListEl.innerHTML = '<div class="monitor-error">' + _t('skills.loadCallStatsError') + ': ' + escapeHtml(error.message) + '</div>';
        }
    }
}

// 渲染skills监控页面
function renderSkillsMonitor() {
    // 渲染总体统计
    const statsEl = document.getElementById('skills-stats');
    if (statsEl) {
        const successRate = skillsStats.totalCalls > 0 
            ? ((skillsStats.totalSuccess / skillsStats.totalCalls) * 100).toFixed(1) 
            : '0.0';
        
        statsEl.innerHTML = `
            <div class="monitor-stat-card">
                <div class="monitor-stat-label">${_t('skills.totalSkillsCount')}</div>
                <div class="monitor-stat-value">${skillsStats.total}</div>
            </div>
            <div class="monitor-stat-card">
                <div class="monitor-stat-label">${_t('skills.totalCallsCount')}</div>
                <div class="monitor-stat-value">${skillsStats.totalCalls}</div>
            </div>
            <div class="monitor-stat-card">
                <div class="monitor-stat-label">${_t('skills.successfulCalls')}</div>
                <div class="monitor-stat-value" style="color: #28a745;">${skillsStats.totalSuccess}</div>
            </div>
            <div class="monitor-stat-card">
                <div class="monitor-stat-label">${_t('skills.failedCalls')}</div>
                <div class="monitor-stat-value" style="color: #dc3545;">${skillsStats.totalFailed}</div>
            </div>
            <div class="monitor-stat-card">
                <div class="monitor-stat-label">${_t('skills.successRate')}</div>
                <div class="monitor-stat-value">${successRate}%</div>
            </div>
        `;
    }

    // 渲染调用统计表格
    const monitorListEl = document.getElementById('skills-monitor-list');
    if (!monitorListEl) return;

    const stats = skillsStats.stats || [];
    
    // 如果没有统计数据，显示空状态
    if (stats.length === 0) {
        monitorListEl.innerHTML = '<div class="monitor-empty">' + _t('skills.noCallRecords') + '</div>';
        return;
    }

    // 按调用次数排序（降序），如果调用次数相同，按名称排序
    const sortedStats = [...stats].sort((a, b) => {
        const callsA = b.total_calls || 0;
        const callsB = a.total_calls || 0;
        if (callsA !== callsB) {
            return callsA - callsB;
        }
        return (a.skill_name || '').localeCompare(b.skill_name || '');
    });

    monitorListEl.innerHTML = `
        <table class="monitor-table">
            <thead>
                <tr>
                    <th style="text-align: left !important;">${_t('skills.skillName')}</th>
                    <th style="text-align: center;">${_t('skills.totalCalls')}</th>
                    <th style="text-align: center;">${_t('skills.success')}</th>
                    <th style="text-align: center;">${_t('skills.failure')}</th>
                    <th style="text-align: center;">${_t('skills.successRate')}</th>
                    <th style="text-align: left;">${_t('skills.lastCallTime')}</th>
                </tr>
            </thead>
            <tbody>
                ${sortedStats.map(stat => {
                    const totalCalls = stat.total_calls || 0;
                    const successCalls = stat.success_calls || 0;
                    const failedCalls = stat.failed_calls || 0;
                    const successRate = totalCalls > 0 ? ((successCalls / totalCalls) * 100).toFixed(1) : '0.0';
                    const lastCallTime = stat.last_call_time && stat.last_call_time !== '-' ? stat.last_call_time : '-';
                    
                    return `
                        <tr>
                            <td style="text-align: left !important;"><strong>${escapeHtml(stat.skill_name || '')}</strong></td>
                            <td style="text-align: center;">${totalCalls}</td>
                            <td style="text-align: center; color: #28a745; font-weight: 500;">${successCalls}</td>
                            <td style="text-align: center; color: #dc3545; font-weight: 500;">${failedCalls}</td>
                            <td style="text-align: center;">${successRate}%</td>
                            <td style="color: var(--text-secondary);">${escapeHtml(lastCallTime)}</td>
                        </tr>
                    `;
                }).join('')}
            </tbody>
        </table>
    `;
}

// 刷新skills监控
async function refreshSkillsMonitor() {
    await loadSkillsMonitor();
    showNotification(_t('skills.refreshed'), 'success');
}

// 清空skills统计数据
async function clearSkillsStats() {
    if (!confirm(_t('skills.clearStatsConfirm'))) {
        return;
    }

    try {
        const response = await apiFetch('/api/skills/stats', {
            method: 'DELETE'
        });

        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || _t('skills.clearStatsFailed'));
        }

        showNotification(_t('skills.statsCleared'), 'success');
        // 重新加载统计数据
        await loadSkillsMonitor();
    } catch (error) {
        console.error('清空统计数据失败:', error);
        showNotification(_t('skills.clearStatsFailed') + ': ' + error.message, 'error');
    }
}

// HTML转义函数
function escapeHtml(text) {
    if (!text) return '';
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// 语言切换时重新渲染当前页（技能列表与分页使用 _t，需随语言更新）
document.addEventListener('languagechange', function () {
    const page = document.getElementById('page-skills-management');
    if (page && page.classList.contains('active')) {
        renderSkillsList();
        if (!skillsSearchKeyword) {
            renderSkillsPagination();
        }
    }
});

document.addEventListener('DOMContentLoaded', function () {
    startSkillsAutoRefresh();
});
