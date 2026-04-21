// Skillsmanagementrelatedfunction
function _t(key, opts) {
    return typeof window.t === 'function' ? window.t(key, opts) : key;
}
let skillsList = [];
let currentEditingSkillName = null;
let isSavingSkill = false; // prevent duplicate submission
let skillsSearchKeyword = '';
let skillsSearchTimeout = null; // search debounce timer
let skillsAutoRefreshTimer = null;
let isAutoRefreshingSkills = false;
const SKILLS_AUTO_REFRESH_INTERVAL_MS = 5000;
let skillsPagination = {
    currentPage: 1,
 pageSize: 20, // eachpage20record(default,actualfromlocalStorageread)
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

// getSave's Per pagecount
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
        console.warn('cannotfromlocalStoragereadpaginationsettings:', e);
    }
    return 20; // default20
}

// initializepaginationsettings
function initSkillsPagination() {
    const savedPageSize = getSkillsPageSize();
    skillsPagination.pageSize = savedPageSize;
}

// loadskillslist(supportpagination)
async function loadSkills(page = 1, pageSize = null) {
    try {
 // ifhasspecifypageSize,useSave's ordefault
        if (pageSize === null) {
            pageSize = getSkillsPageSize();
        }
        
        // update pagination state(ensureusecorrect's pageSize)
        skillsPagination.currentPage = page;
        skillsPagination.pageSize = pageSize;
        
        // clearsearchkeyword(Normalpaginationloadwhen)
        skillsSearchKeyword = '';
        const searchInput = document.getElementById('skills-search');
        if (searchInput) {
            searchInput.value = '';
        }
        
        // buildURL(supportpagination)
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
        console.error('loadskillslist failed:', error);
        showNotification(_t('skills.loadListFailed') + ': ' + error.message, 'error');
        const skillsListEl = document.getElementById('skills-list');
        if (skillsListEl) {
            skillsListEl.innerHTML = '<div class="empty-state">' + _t('skills.loadFailedShort') + ': ' + escapeHtml(error.message) + '</div>';
        }
    }
}

// renderskillslist
function renderSkillsList() {
    const skillsListEl = document.getElementById('skills-list');
    if (!skillsListEl) return;

    // backendalreadycompletesearchfilter,use directlyskillsList
    const filteredSkills = skillsList;

    if (filteredSkills.length === 0) {
        skillsListEl.innerHTML = '<div class="empty-state">' + 
            (skillsSearchKeyword ? _t('skills.noMatch') : _t('skills.noSkills')) + 
            '</div>';
        // hide pagination during search
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
    
 // ensurelistcontainercantoscroll,paginationcan
    // use setTimeout ensure DOM updatecompleteafterthencheck
    setTimeout(() => {
        const paginationContainer = document.getElementById('skills-pagination');
        if (paginationContainer && !skillsSearchKeyword) {
 // ensurepaginationcan
            paginationContainer.style.display = 'block';
            paginationContainer.style.visibility = 'visible';
        }
    }, 0);
}

// renderpaginationcomponent(referenceMCPmanagement pagestyle)
function renderSkillsPagination() {
    const paginationContainer = document.getElementById('skills-pagination');
    if (!paginationContainer) return;
    
    const total = skillsPagination.total;
    const pageSize = skillsPagination.pageSize;
    const currentPage = skillsPagination.currentPage;
    const totalPages = Math.ceil(total / pageSize);
    
 // even ifonlyhaspagealsoShow paginationInfo(referenceMCPstyle)
    if (total === 0) {
        paginationContainer.innerHTML = '';
        return;
    }
    
    // calculateShow range
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
    // left side:Show rangeInfoandeachpagecountselector(referenceMCPstyle)
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
    
    // right side:paginationbutton(referenceMCPstyle:First,Previous,X/Ypage,Next,Last)
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
    
    // ensurepaginationcomponentandlistcontentareaalign(excluding scrollbar)
    function alignPaginationWidth() {
        const skillsList = document.getElementById('skills-list');
        if (skillsList && paginationContainer) {
 // ensurepaginationcontaineralwayscan
            paginationContainer.style.display = '';
            paginationContainer.style.visibility = 'visible';
            paginationContainer.style.opacity = '1';
            
            // getlist's actualcontentwidth(excluding scrollbar)
            const listClientWidth = skillsList.clientWidth; // visibleareawidth(excluding scrollbar)
            const listScrollHeight = skillsList.scrollHeight; // contenttotal height
            const listClientHeight = skillsList.clientHeight; // visibleareaheight
            const hasScrollbar = listScrollHeight > listClientHeight;
            
            // iflisthasverticalscrollrecord,pagination should align with list content area(clientWidth)
            // if no scrollbar,use100%width
            if (hasScrollbar && listClientWidth > 0) {
                // pagination should align with list content area,excluding scrollbar
                paginationContainer.style.width = `${listClientWidth}px`;
            } else {
                // if no scrollbar,use100%width
                paginationContainer.style.width = '100%';
            }
        }
    }
    
 // immediatelylinetime
    alignPaginationWidth();
    
    // listenwindowSizechangeandlistcontentchange
    const resizeObserver = new ResizeObserver(() => {
        alignPaginationWidth();
    });
    
    const skillsList = document.getElementById('skills-list');
    if (skillsList) {
        resizeObserver.observe(skillsList);
    }
    
 // ensurepaginationcontaineralwayscan(preventbyhide)
    paginationContainer.style.display = 'block';
    paginationContainer.style.visibility = 'visible';
}

// change items per page
async function changeSkillsPageSize() {
    const pageSizeSelect = document.getElementById('skills-page-size-pagination');
    if (!pageSizeSelect) return;
    
    const newPageSize = parseInt(pageSizeSelect.value);
    if (isNaN(newPageSize) || newPageSize <= 0) return;
    
    // save tolocalStorage
    try {
        localStorage.setItem('skillsPageSize', newPageSize.toString());
    } catch (e) {
        console.warn('cannotSavepaginationsettingstolocalStorage:', e);
    }
    
    // update pagination state
    skillsPagination.pageSize = newPageSize;
    
    // re-calculatecurrentpage(ensurenotexceedoutrange)
    const totalPages = Math.ceil(skillsPagination.total / newPageSize);
    const currentPage = Math.min(skillsPagination.currentPage, totalPages || 1);
    skillsPagination.currentPage = currentPage;
    
    // re-loaddata
    await loadSkills(currentPage, newPageSize);
}

// updateskillsmanagementstatisticsInfo
function updateSkillsManagementStats() {
    const statsEl = document.getElementById('skills-management-stats');
    if (!statsEl) return;

    const totalEl = statsEl.querySelector('.skill-stat-value');
    if (totalEl) {
        totalEl.textContent = skillsPagination.total;
    }
}

// searchskills
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
        // hassearchkeywordwhen,usebackendsearchAPI(loadallmatchresult,notpagination)
        try {
            const response = await apiFetch(`/api/skills?search=${encodeURIComponent(skillsSearchKeyword)}&limit=10000&offset=0`);
            if (!response.ok) {
                throw new Error(_t('skills.loadListFailed'));
            }
            const data = await response.json();
            skillsList = data.skills || [];
            skillsPagination.total = data.total || 0;
            renderSkillsList();
            // hide pagination during search
            const paginationContainer = document.getElementById('skills-pagination');
            if (paginationContainer) {
                paginationContainer.innerHTML = '';
            }
            // update statistics(Show searchresultcount)
            updateSkillsManagementStats();
        } catch (error) {
            console.error('searchskillsFailed:', error);
            showNotification(_t('skills.searchFailed') + ': ' + error.message, 'error');
        }
    } else {
 // hassearchkeywordwhen,restorepaginationload
        await loadSkills(1, skillsPagination.pageSize);
    }
}

// clearskillssearch
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
    // restorepaginationload
    loadSkills(1, skillsPagination.pageSize);
}

// Refreshskills
async function refreshSkills() {
    await loadSkills(skillsPagination.currentPage, skillsPagination.pageSize);
    showNotification(_t('skills.refreshed'), 'success');
}

// Show addskillmodal
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

// Editskill
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
        document.getElementById('skill-name').disabled = true; // Editwhennotallowmodifyname
        document.getElementById('skill-description').value = skill.description || '';
        document.getElementById('skill-content').value = skill.content || '';
        
        currentEditingSkillName = skillName;
        modal.style.display = 'flex';
    } catch (error) {
        console.error('loadskilldetailsFailed:', error);
        showNotification(_t('skills.loadDetailFailed') + ': ' + error.message, 'error');
    }
}

// viewskill
async function viewSkill(skillName) {
    try {
        const response = await apiFetch(`/api/skills/${encodeURIComponent(skillName)}`);
        if (!response.ok) {
            throw new Error(_t('skills.loadDetailFailed'));
        }
        const data = await response.json();
        const skill = data.skill;

        // createviewmodal
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
        console.error('viewskillFailed:', error);
        showNotification(_t('skills.viewFailed') + ': ' + error.message, 'error');
    }
}

// Closeviewmodal
function closeSkillViewModal() {
    const modal = document.getElementById('skill-view-modal');
    if (modal) {
        modal.remove();
    }
}

// Closeskillmodal
function closeSkillModal() {
    const modal = document.getElementById('skill-modal');
    if (modal) {
        modal.style.display = 'none';
        currentEditingSkillName = null;
    }
}

// Saveskill
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

    // validateskillname
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
        console.error('SaveskillFailed:', error);
        showNotification(_t('skills.saveFailed') + ': ' + error.message, 'error');
    } finally {
        isSavingSkill = false;
        if (saveBtn) {
            saveBtn.disabled = false;
            saveBtn.textContent = _t('common.save');
        }
    }
}

// Deleteskill
async function deleteSkill(skillName) {
 // firstcheckishasrolebindthisskill
    let boundRoles = [];
    try {
        const checkResponse = await apiFetch(`/api/skills/${encodeURIComponent(skillName)}/bound-roles`);
        if (checkResponse.ok) {
            const checkData = await checkResponse.json();
            boundRoles = checkData.bound_roles || [];
        }
    } catch (error) {
        console.warn('checkskillbindFailed:', error);
 // ifcheckFailed,ContinuelineDelete
    }

    // buildconfirmmessage
    let confirmMessage = _t('skills.deleteConfirm', { name: skillName });
    if (boundRoles.length > 0) {
        const rolesList = boundRoles.join(',');
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
            const rolesList = data.affected_roles.join(',');
            successMessage = _t('skills.deleteSuccessWithRoles', { count: data.affected_roles.length, roles: rolesList });
        }
        showNotification(successMessage, 'success');
        
 // ifcurrentpagehasdata,go back toPrevious
        const currentPage = skillsPagination.currentPage;
        const totalAfterDelete = skillsPagination.total - 1;
        const totalPages = Math.ceil(totalAfterDelete / skillsPagination.pageSize);
        const pageToLoad = currentPage > totalPages && totalPages > 0 ? totalPages : currentPage;
        await loadSkills(pageToLoad, skillsPagination.pageSize);
    } catch (error) {
        console.error('DeleteskillFailed:', error);
        showNotification(_t('skills.deleteFailed') + ': ' + error.message, 'error');
    }
}

// ==================== SkillsstatusMonitorrelatedfunction ====================

// loadskillsMonitordata
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
        console.error('loadskillsMonitordataFailed:', error);
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

// renderskillsMonitorpage
function renderSkillsMonitor() {
 // renderstatistics
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

    // rendercallstatisticstable
    const monitorListEl = document.getElementById('skills-monitor-list');
    if (!monitorListEl) return;

    const stats = skillsStats.stats || [];
    
 // ifhasstatisticsdata,show empty state
    if (stats.length === 0) {
        monitorListEl.innerHTML = '<div class="monitor-empty">' + _t('skills.noCallRecords') + '</div>';
        return;
    }

 // bycalltimesort(fall),ifcalltimesame,bynamesort
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

// RefreshskillsMonitor
async function refreshSkillsMonitor() {
    await loadSkillsMonitor();
    showNotification(_t('skills.refreshed'), 'success');
}

// clearskillsstatisticsdata
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
        // re-loadstatisticsdata
        await loadSkillsMonitor();
    } catch (error) {
        console.error('Failed to clear statistics data:', error);
        showNotification(_t('skills.clearStatsFailed') + ': ' + error.message, 'error');
    }
}

// HTMLescape function
function escapeHtml(text) {
    if (!text) return '';
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// languageswitchwhenre-rendercurrentpage(canlistandpaginationuse _t,needlanguageupdate)
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
