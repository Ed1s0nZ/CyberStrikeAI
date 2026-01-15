// Skills管理相关功能
let skillsList = [];
let currentEditingSkillName = null;
let isSavingSkill = false; // 防止重复提交
let skillsSearchKeyword = '';
let skillsSearchTimeout = null; // 搜索防抖定时器
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
            throw new Error('获取skills列表失败');
        }
        const data = await response.json();
        skillsList = data.skills || [];
        skillsPagination.total = data.total || 0;
        
        renderSkillsList();
        renderSkillsPagination();
        updateSkillsManagementStats();
    } catch (error) {
        console.error('加载skills列表失败:', error);
        showNotification('加载skills列表失败: ' + error.message, 'error');
        const skillsListEl = document.getElementById('skills-list');
        if (skillsListEl) {
            skillsListEl.innerHTML = '<div class="empty-state">加载失败: ' + error.message + '</div>';
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
            (skillsSearchKeyword ? '没有找到匹配的skills' : '暂无skills，点击"添加Skill"创建第一个skill') + 
            '</div>';
        // 搜索时隐藏分页
        const paginationContainer = document.getElementById('skills-pagination');
        if (paginationContainer) {
            paginationContainer.innerHTML = '';
        }
        return;
    }

    skillsListEl.innerHTML = filteredSkills.map(skill => {
        const fileSize = skill.file_size || 0;
        const fileSizeStr = fileSize < 1024 ? fileSize + ' B' : 
                           fileSize < 1024 * 1024 ? (fileSize / 1024).toFixed(2) + ' KB' :
                           (fileSize / (1024 * 1024)).toFixed(2) + ' MB';
        
        return `
            <div class="skill-item">
                <div class="skill-item-header">
                    <div class="skill-item-info">
                        <h3 class="skill-item-name">${escapeHtml(skill.name || '')}</h3>
                        ${skill.description ? `<p class="skill-item-desc">${escapeHtml(skill.description)}</p>` : ''}
                    </div>
                    <div class="skill-item-actions">
                        <button class="btn-icon" onclick="viewSkill('${escapeHtml(skill.name)}')" title="查看">
                            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"></path>
                                <circle cx="12" cy="12" r="3"></circle>
                            </svg>
                        </button>
                        <button class="btn-icon" onclick="editSkill('${escapeHtml(skill.name)}')" title="编辑">
                            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"></path>
                                <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"></path>
                            </svg>
                        </button>
                        <button class="btn-icon btn-danger" onclick="deleteSkill('${escapeHtml(skill.name)}')" title="删除">
                            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                <polyline points="3 6 5 6 21 6"></polyline>
                                <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path>
                            </svg>
                        </button>
                    </div>
                </div>
                <div class="skill-item-meta">
                    <span class="skill-meta-item">路径: ${escapeHtml(skill.path || '')}</span>
                    <span class="skill-meta-item">大小: ${fileSizeStr}</span>
                    ${skill.mod_time ? `<span class="skill-meta-item">修改时间: ${escapeHtml(skill.mod_time)}</span>` : ''}
                </div>
            </div>
        `;
    }).join('');
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
    const start = (currentPage - 1) * pageSize + 1;
    const end = Math.min(currentPage * pageSize, total);
    
    let paginationHTML = '<div class="pagination">';
    
    // 左侧：显示范围信息
    paginationHTML += `
        <div class="pagination-info">
            <span>显示 ${start}-${end} / 共 ${total} 条</span>
        </div>
    `;
    
    // 中间：每页数量选择器
    paginationHTML += `
        <div class="pagination-page-size">
            <label for="skills-page-size-pagination">每页:</label>
            <select id="skills-page-size-pagination" onchange="changeSkillsPageSize()">
                <option value="10" ${pageSize === 10 ? 'selected' : ''}>10</option>
                <option value="20" ${pageSize === 20 ? 'selected' : ''}>20</option>
                <option value="50" ${pageSize === 50 ? 'selected' : ''}>50</option>
                <option value="100" ${pageSize === 100 ? 'selected' : ''}>100</option>
            </select>
        </div>
    `;
    
    // 右侧：分页按钮（参考MCP样式：首页、上一页、第X/Y页、下一页、末页）
    paginationHTML += `
        <div class="pagination-controls">
            <button class="btn-secondary" onclick="loadSkills(1, ${pageSize})" ${currentPage === 1 ? 'disabled' : ''}>首页</button>
            <button class="btn-secondary" onclick="loadSkills(${currentPage - 1}, ${pageSize})" ${currentPage === 1 ? 'disabled' : ''}>上一页</button>
            <span class="pagination-page">第 ${currentPage} / ${totalPages || 1} 页</span>
            <button class="btn-secondary" onclick="loadSkills(${currentPage + 1}, ${pageSize})" ${currentPage >= totalPages ? 'disabled' : ''}>下一页</button>
            <button class="btn-secondary" onclick="loadSkills(${totalPages || 1}, ${pageSize})" ${currentPage >= totalPages ? 'disabled' : ''}>末页</button>
        </div>
    `;
    
    paginationHTML += '</div>';
    
    paginationContainer.innerHTML = paginationHTML;
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
    
    if (skillsSearchKeyword) {
        // 有搜索关键词时，使用后端搜索API（加载所有匹配结果，不分页）
        try {
            const response = await apiFetch(`/api/skills?search=${encodeURIComponent(skillsSearchKeyword)}&limit=10000&offset=0`);
            if (!response.ok) {
                throw new Error('获取skills列表失败');
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
            showNotification('搜索失败: ' + error.message, 'error');
        }
    } else {
        // 没有搜索关键词时，恢复分页加载
        await loadSkills(1, skillsPagination.pageSize);
    }
}

// 刷新skills
async function refreshSkills() {
    await loadSkills(skillsPagination.currentPage, skillsPagination.pageSize);
    showNotification('已刷新', 'success');
}

// 显示添加skill模态框
function showAddSkillModal() {
    const modal = document.getElementById('skill-modal');
    if (!modal) return;

    document.getElementById('skill-modal-title').textContent = '添加Skill';
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
            throw new Error('获取skill详情失败');
        }
        const data = await response.json();
        const skill = data.skill;

        const modal = document.getElementById('skill-modal');
        if (!modal) return;

        document.getElementById('skill-modal-title').textContent = '编辑Skill';
        document.getElementById('skill-name').value = skill.name;
        document.getElementById('skill-name').disabled = true; // 编辑时不允许修改名称
        document.getElementById('skill-description').value = skill.description || '';
        document.getElementById('skill-content').value = skill.content || '';
        
        currentEditingSkillName = skillName;
        modal.style.display = 'flex';
    } catch (error) {
        console.error('加载skill详情失败:', error);
        showNotification('加载skill详情失败: ' + error.message, 'error');
    }
}

// 查看skill
async function viewSkill(skillName) {
    try {
        const response = await apiFetch(`/api/skills/${encodeURIComponent(skillName)}`);
        if (!response.ok) {
            throw new Error('获取skill详情失败');
        }
        const data = await response.json();
        const skill = data.skill;

        // 创建查看模态框
        const modal = document.createElement('div');
        modal.className = 'modal';
        modal.id = 'skill-view-modal';
        modal.innerHTML = `
            <div class="modal-content" style="max-width: 900px; max-height: 90vh;">
                <div class="modal-header">
                    <h2>查看Skill: ${escapeHtml(skill.name)}</h2>
                    <span class="modal-close" onclick="closeSkillViewModal()">&times;</span>
                </div>
                <div class="modal-body" style="overflow-y: auto; max-height: calc(90vh - 120px);">
                    ${skill.description ? `<div style="margin-bottom: 16px;"><strong>描述:</strong> ${escapeHtml(skill.description)}</div>` : ''}
                    <div style="margin-bottom: 8px;"><strong>路径:</strong> ${escapeHtml(skill.path || '')}</div>
                    <div style="margin-bottom: 16px;"><strong>修改时间:</strong> ${escapeHtml(skill.mod_time || '')}</div>
                    <div style="margin-bottom: 8px;"><strong>内容:</strong></div>
                    <pre style="background: #f5f5f5; padding: 16px; border-radius: 4px; overflow-x: auto; white-space: pre-wrap; word-wrap: break-word;">${escapeHtml(skill.content || '')}</pre>
                </div>
                <div class="modal-footer">
                    <button class="btn-secondary" onclick="closeSkillViewModal()">关闭</button>
                    <button class="btn-primary" onclick="editSkill('${escapeHtml(skill.name)}'); closeSkillViewModal();">编辑</button>
                </div>
            </div>
        `;
        document.body.appendChild(modal);
        modal.style.display = 'flex';
    } catch (error) {
        console.error('查看skill失败:', error);
        showNotification('查看skill失败: ' + error.message, 'error');
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
        showNotification('skill名称不能为空', 'error');
        return;
    }

    if (!content) {
        showNotification('skill内容不能为空', 'error');
        return;
    }

    // 验证skill名称
    if (!/^[a-zA-Z0-9_-]+$/.test(name)) {
        showNotification('skill名称只能包含字母、数字、连字符和下划线', 'error');
        return;
    }

    isSavingSkill = true;
    const saveBtn = document.querySelector('#skill-modal .btn-primary');
    if (saveBtn) {
        saveBtn.disabled = true;
        saveBtn.textContent = '保存中...';
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
            throw new Error(error.error || '保存skill失败');
        }

        showNotification(isEdit ? 'skill已更新' : 'skill已创建', 'success');
        closeSkillModal();
        await loadSkills(skillsPagination.currentPage, skillsPagination.pageSize);
    } catch (error) {
        console.error('保存skill失败:', error);
        showNotification('保存skill失败: ' + error.message, 'error');
    } finally {
        isSavingSkill = false;
        if (saveBtn) {
            saveBtn.disabled = false;
            saveBtn.textContent = '保存';
        }
    }
}

// 删除skill
async function deleteSkill(skillName) {
    if (!confirm(`确定要删除skill "${skillName}" 吗？此操作不可恢复。`)) {
        return;
    }

    try {
        const response = await apiFetch(`/api/skills/${encodeURIComponent(skillName)}`, {
            method: 'DELETE'
        });

        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || '删除skill失败');
        }

        showNotification('skill已删除', 'success');
        // 如果当前页没有数据了，回到上一页
        const currentPage = skillsPagination.currentPage;
        const totalAfterDelete = skillsPagination.total - 1;
        const totalPages = Math.ceil(totalAfterDelete / skillsPagination.pageSize);
        const pageToLoad = currentPage > totalPages && totalPages > 0 ? totalPages : currentPage;
        await loadSkills(pageToLoad, skillsPagination.pageSize);
    } catch (error) {
        console.error('删除skill失败:', error);
        showNotification('删除skill失败: ' + error.message, 'error');
    }
}

// ==================== Skills状态监控相关函数 ====================

// 加载skills监控数据
async function loadSkillsMonitor() {
    try {
        const response = await apiFetch('/api/skills/stats');
        if (!response.ok) {
            throw new Error('获取skills统计信息失败');
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
        showNotification('加载skills监控数据失败: ' + error.message, 'error');
        const statsEl = document.getElementById('skills-stats');
        if (statsEl) {
            statsEl.innerHTML = '<div class="monitor-error">无法加载统计信息：' + escapeHtml(error.message) + '</div>';
        }
        const monitorListEl = document.getElementById('skills-monitor-list');
        if (monitorListEl) {
            monitorListEl.innerHTML = '<div class="monitor-error">无法加载调用统计：' + escapeHtml(error.message) + '</div>';
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
                <div class="monitor-stat-label">总Skills数</div>
                <div class="monitor-stat-value">${skillsStats.total}</div>
            </div>
            <div class="monitor-stat-card">
                <div class="monitor-stat-label">总调用次数</div>
                <div class="monitor-stat-value">${skillsStats.totalCalls}</div>
            </div>
            <div class="monitor-stat-card">
                <div class="monitor-stat-label">成功调用</div>
                <div class="monitor-stat-value" style="color: #28a745;">${skillsStats.totalSuccess}</div>
            </div>
            <div class="monitor-stat-card">
                <div class="monitor-stat-label">失败调用</div>
                <div class="monitor-stat-value" style="color: #dc3545;">${skillsStats.totalFailed}</div>
            </div>
            <div class="monitor-stat-card">
                <div class="monitor-stat-label">成功率</div>
                <div class="monitor-stat-value">${successRate}%</div>
            </div>
            <div class="monitor-stat-card">
                <div class="monitor-stat-label">Skills目录</div>
                <div class="monitor-stat-value" style="font-size: 0.875rem;">${escapeHtml(skillsStats.skillsDir || '-')}</div>
            </div>
        `;
    }

    // 渲染调用统计表格
    const monitorListEl = document.getElementById('skills-monitor-list');
    if (!monitorListEl) return;

    const stats = skillsStats.stats || [];
    
    // 如果没有统计数据，显示空状态
    if (stats.length === 0) {
        monitorListEl.innerHTML = '<div class="monitor-empty">暂无Skills调用记录</div>';
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
                    <th style="text-align: left;">Skill名称</th>
                    <th style="text-align: center;">总调用</th>
                    <th style="text-align: center;">成功</th>
                    <th style="text-align: center;">失败</th>
                    <th style="text-align: center;">成功率</th>
                    <th style="text-align: left;">最后调用时间</th>
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
                            <td><strong>${escapeHtml(stat.skill_name || '')}</strong></td>
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
    showNotification('已刷新', 'success');
}

// 清空skills统计数据
async function clearSkillsStats() {
    if (!confirm('确定要清空所有Skills统计数据吗？此操作不可恢复。')) {
        return;
    }

    try {
        const response = await apiFetch('/api/skills/stats', {
            method: 'DELETE'
        });

        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || '清空统计数据失败');
        }

        showNotification('已清空所有Skills统计数据', 'success');
        // 重新加载统计数据
        await loadSkillsMonitor();
    } catch (error) {
        console.error('清空统计数据失败:', error);
        showNotification('清空统计数据失败: ' + error.message, 'error');
    }
}

// HTML转义函数
function escapeHtml(text) {
    if (!text) return '';
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}
