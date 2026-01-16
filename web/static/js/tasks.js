// 任务管理页面功能

// HTML转义函数（如果未定义）
if (typeof escapeHtml === 'undefined') {
    function escapeHtml(text) {
        if (text == null) return '';
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }
}

// 任务管理状态
const tasksState = {
    allTasks: [],
    filteredTasks: [],
    selectedTasks: new Set(),
    autoRefresh: true,
    refreshInterval: null,
    durationUpdateInterval: null,
    completedTasksHistory: [], // 保存最近完成的任务历史
    showHistory: true // 是否显示历史记录
};

// 从localStorage加载已完成任务历史
function loadCompletedTasksHistory() {
    try {
        const saved = localStorage.getItem('tasks-completed-history');
        if (saved) {
            const history = JSON.parse(saved);
            // 只保留最近24小时内完成的任务
            const now = Date.now();
            const oneDayAgo = now - 24 * 60 * 60 * 1000;
            tasksState.completedTasksHistory = history.filter(task => {
                const completedTime = task.completedAt || task.startedAt;
                return completedTime && new Date(completedTime).getTime() > oneDayAgo;
            });
            // 保存清理后的历史
            saveCompletedTasksHistory();
        }
    } catch (error) {
        console.error('加载已完成任务历史失败:', error);
        tasksState.completedTasksHistory = [];
    }
}

// 保存已完成任务历史到localStorage
function saveCompletedTasksHistory() {
    try {
        localStorage.setItem('tasks-completed-history', JSON.stringify(tasksState.completedTasksHistory));
    } catch (error) {
        console.error('保存已完成任务历史失败:', error);
    }
}

// 更新已完成任务历史
function updateCompletedTasksHistory(currentTasks) {
    // 保存当前所有任务作为快照（用于下次比较）
    const currentTaskIds = new Set(currentTasks.map(t => t.conversationId));
    
    // 如果是首次加载，只需要保存当前任务快照
    if (tasksState.allTasks.length === 0) {
        return;
    }
    
    const previousTaskIds = new Set(tasksState.allTasks.map(t => t.conversationId));
    
    // 找出刚完成的任务（之前存在但现在不存在的）
    // 只要任务从列表中消失了，就认为它已完成
    const justCompleted = tasksState.allTasks.filter(task => {
        return previousTaskIds.has(task.conversationId) && !currentTaskIds.has(task.conversationId);
    });
    
    // 将刚完成的任务添加到历史中
    justCompleted.forEach(task => {
        // 检查是否已存在（避免重复添加）
        const exists = tasksState.completedTasksHistory.some(t => t.conversationId === task.conversationId);
        if (!exists) {
            // 如果任务状态不是最终状态，标记为completed
            const finalStatus = ['completed', 'failed', 'timeout', 'cancelled'].includes(task.status) 
                ? task.status 
                : 'completed';
            
            tasksState.completedTasksHistory.push({
                conversationId: task.conversationId,
                message: task.message || '未命名任务',
                startedAt: task.startedAt,
                status: finalStatus,
                completedAt: new Date().toISOString()
            });
        }
    });
    
    // 限制历史记录数量（最多保留50条）
    if (tasksState.completedTasksHistory.length > 50) {
        tasksState.completedTasksHistory = tasksState.completedTasksHistory
            .sort((a, b) => new Date(b.completedAt || b.startedAt) - new Date(a.completedAt || a.startedAt))
            .slice(0, 50);
    }
    
    saveCompletedTasksHistory();
}

// 加载任务列表
async function loadTasks() {
    const listContainer = document.getElementById('tasks-list');
    if (!listContainer) return;
    
    listContainer.innerHTML = '<div class="loading-spinner">加载中...</div>';

    try {
        // 并行加载运行中的任务和已完成的任务历史
        const [activeResponse, completedResponse] = await Promise.allSettled([
            apiFetch('/api/agent-loop/tasks'),
            apiFetch('/api/agent-loop/tasks/completed').catch(() => null) // 如果API不存在，返回null
        ]);

        // 处理运行中的任务
        if (activeResponse.status === 'rejected' || !activeResponse.value || !activeResponse.value.ok) {
            throw new Error('获取任务列表失败');
        }

        const activeResult = await activeResponse.value.json();
        const activeTasks = activeResult.tasks || [];
        
        // 加载已完成任务历史（如果API可用）
        let completedTasks = [];
        if (completedResponse.status === 'fulfilled' && completedResponse.value && completedResponse.value.ok) {
            try {
                const completedResult = await completedResponse.value.json();
                completedTasks = completedResult.tasks || [];
            } catch (e) {
                console.warn('解析已完成任务历史失败:', e);
            }
        }
        
        // 保存所有任务
        tasksState.allTasks = activeTasks;
        
        // 更新已完成任务历史（从后端API获取）
        if (completedTasks.length > 0) {
            // 合并后端历史记录和本地历史记录（去重）
            const backendTaskIds = new Set(completedTasks.map(t => t.conversationId));
            const localHistory = tasksState.completedTasksHistory.filter(t => 
                !backendTaskIds.has(t.conversationId)
            );
            
            // 后端的历史记录优先，然后添加本地独有的
            tasksState.completedTasksHistory = [
                ...completedTasks.map(t => ({
                    conversationId: t.conversationId,
                    message: t.message || '未命名任务',
                    startedAt: t.startedAt,
                    status: t.status || 'completed',
                    completedAt: t.completedAt || new Date().toISOString()
                })),
                ...localHistory
            ];
            
            // 限制历史记录数量
            if (tasksState.completedTasksHistory.length > 50) {
                tasksState.completedTasksHistory = tasksState.completedTasksHistory
                    .sort((a, b) => new Date(b.completedAt || b.startedAt) - new Date(a.completedAt || a.startedAt))
                    .slice(0, 50);
            }
            
            saveCompletedTasksHistory();
        } else {
            // 如果后端API不可用，仍然使用前端逻辑更新历史
            updateCompletedTasksHistory(activeTasks);
        }
        
        updateTaskStats(activeTasks);
        filterAndSortTasks();
        startDurationUpdates();
    } catch (error) {
        console.error('加载任务失败:', error);
        listContainer.innerHTML = `
            <div class="tasks-empty">
                <p>加载失败: ${escapeHtml(error.message)}</p>
                <button class="btn-secondary" onclick="loadTasks()">重试</button>
            </div>
        `;
    }
}

// 更新任务统计
function updateTaskStats(tasks) {
    const stats = {
        running: 0,
        cancelling: 0,
        completed: 0,
        failed: 0,
        timeout: 0,
        cancelled: 0,
        total: tasks.length
    };

    tasks.forEach(task => {
        if (task.status === 'running') {
            stats.running++;
        } else if (task.status === 'cancelling') {
            stats.cancelling++;
        } else if (task.status === 'completed') {
            stats.completed++;
        } else if (task.status === 'failed') {
            stats.failed++;
        } else if (task.status === 'timeout') {
            stats.timeout++;
        } else if (task.status === 'cancelled') {
            stats.cancelled++;
        }
    });

    const statRunning = document.getElementById('stat-running');
    const statCancelling = document.getElementById('stat-cancelling');
    const statCompleted = document.getElementById('stat-completed');
    const statTotal = document.getElementById('stat-total');

    if (statRunning) statRunning.textContent = stats.running;
    if (statCancelling) statCancelling.textContent = stats.cancelling;
    if (statCompleted) statCompleted.textContent = stats.completed;
    if (statTotal) statTotal.textContent = stats.total;
}

// 筛选任务
function filterTasks() {
    filterAndSortTasks();
}

// 排序任务
function sortTasks() {
    filterAndSortTasks();
}

// 筛选和排序任务
function filterAndSortTasks() {
    const statusFilter = document.getElementById('tasks-status-filter')?.value || 'all';
    const sortBy = document.getElementById('tasks-sort-by')?.value || 'time-desc';
    
    // 合并当前任务和历史任务
    let allTasks = [...tasksState.allTasks];
    
    // 如果显示历史记录，添加历史任务
    if (tasksState.showHistory) {
        const historyTasks = tasksState.completedTasksHistory
            .filter(ht => !tasksState.allTasks.some(t => t.conversationId === ht.conversationId))
            .map(ht => ({ ...ht, isHistory: true }));
        allTasks = [...allTasks, ...historyTasks];
    }
    
    // 筛选
    let filtered = allTasks;
    if (statusFilter === 'active') {
        // 仅运行中的任务（不包括历史）
        filtered = tasksState.allTasks.filter(task => 
            task.status === 'running' || task.status === 'cancelling'
        );
    } else if (statusFilter === 'history') {
        // 仅历史记录
        filtered = allTasks.filter(task => task.isHistory);
    } else if (statusFilter !== 'all') {
        filtered = allTasks.filter(task => task.status === statusFilter);
    }
    
    // 排序
    filtered.sort((a, b) => {
        const aTime = new Date(a.completedAt || a.startedAt);
        const bTime = new Date(b.completedAt || b.startedAt);
        
        switch (sortBy) {
            case 'time-asc':
                return aTime - bTime;
            case 'time-desc':
                return bTime - aTime;
            case 'status':
                return (a.status || '').localeCompare(b.status || '');
            case 'message':
                return (a.message || '').localeCompare(b.message || '');
            default:
                return 0;
        }
    });
    
    tasksState.filteredTasks = filtered;
    renderTasks(filtered);
    updateBatchActions();
}

// 切换显示历史记录
function toggleShowHistory(show) {
    tasksState.showHistory = show;
    localStorage.setItem('tasks-show-history', show ? 'true' : 'false');
    filterAndSortTasks();
}

// 计算执行时长
function calculateDuration(startedAt) {
    if (!startedAt) return '未知';
    const start = new Date(startedAt);
    const now = new Date();
    const diff = Math.floor((now - start) / 1000); // 秒
    
    if (diff < 60) {
        return `${diff}秒`;
    } else if (diff < 3600) {
        const minutes = Math.floor(diff / 60);
        const seconds = diff % 60;
        return `${minutes}分${seconds}秒`;
    } else {
        const hours = Math.floor(diff / 3600);
        const minutes = Math.floor((diff % 3600) / 60);
        return `${hours}小时${minutes}分`;
    }
}

// 开始时长更新
function startDurationUpdates() {
    // 清除旧的定时器
    if (tasksState.durationUpdateInterval) {
        clearInterval(tasksState.durationUpdateInterval);
    }
    
    // 每秒更新一次执行时长
    tasksState.durationUpdateInterval = setInterval(() => {
        updateTaskDurations();
    }, 1000);
}

// 更新任务执行时长显示
function updateTaskDurations() {
    const taskItems = document.querySelectorAll('.task-item[data-task-id]');
    taskItems.forEach(item => {
        const startedAt = item.dataset.startedAt;
        const status = item.dataset.status;
        const durationEl = item.querySelector('.task-duration');
        
        if (durationEl && startedAt && (status === 'running' || status === 'cancelling')) {
            durationEl.textContent = calculateDuration(startedAt);
        }
    });
}

// 渲染任务列表
function renderTasks(tasks) {
    const listContainer = document.getElementById('tasks-list');
    if (!listContainer) return;

    if (tasks.length === 0) {
        listContainer.innerHTML = `
            <div class="tasks-empty">
                <p>当前没有符合条件的任务</p>
                ${tasksState.allTasks.length === 0 && tasksState.completedTasksHistory.length > 0 ? 
                    '<p style="margin-top: 8px; color: var(--text-muted); font-size: 0.875rem;">提示：有已完成的任务历史，请勾选"显示历史记录"查看</p>' : ''}
            </div>
        `;
        return;
    }

    // 状态映射
    const statusMap = {
        'running': { text: '执行中', class: 'task-status-running' },
        'cancelling': { text: '取消中', class: 'task-status-cancelling' },
        'failed': { text: '执行失败', class: 'task-status-failed' },
        'timeout': { text: '执行超时', class: 'task-status-timeout' },
        'cancelled': { text: '已取消', class: 'task-status-cancelled' },
        'completed': { text: '已完成', class: 'task-status-completed' }
    };

    // 分离当前任务和历史任务
    const activeTasks = tasks.filter(t => !t.isHistory);
    const historyTasks = tasks.filter(t => t.isHistory);

    let html = '';
    
    // 渲染当前任务
    if (activeTasks.length > 0) {
        html += activeTasks.map(task => renderTaskItem(task, statusMap)).join('');
    }
    
    // 渲染历史任务
    if (historyTasks.length > 0) {
        html += `<div class="tasks-history-section">
            <div class="tasks-history-header">
                <span class="tasks-history-title">📜 最近完成的任务（最近24小时）</span>
                <button class="btn-secondary btn-small" onclick="clearTasksHistory()">清空历史</button>
            </div>
            ${historyTasks.map(task => renderTaskItem(task, statusMap, true)).join('')}
        </div>`;
    }
    
    listContainer.innerHTML = html;
}

// 渲染单个任务项
function renderTaskItem(task, statusMap, isHistory = false) {
    const startedTime = task.startedAt ? new Date(task.startedAt) : null;
    const completedTime = task.completedAt ? new Date(task.completedAt) : null;
    
    const timeText = startedTime && !isNaN(startedTime.getTime())
        ? startedTime.toLocaleString('zh-CN', { 
            year: 'numeric',
            month: '2-digit',
            day: '2-digit',
            hour: '2-digit',
            minute: '2-digit',
            second: '2-digit'
        })
        : '未知时间';
    
    const completedText = completedTime && !isNaN(completedTime.getTime())
        ? completedTime.toLocaleString('zh-CN', { 
            year: 'numeric',
            month: '2-digit',
            day: '2-digit',
            hour: '2-digit',
            minute: '2-digit',
            second: '2-digit'
        })
        : '';

    const status = statusMap[task.status] || { text: task.status, class: 'task-status-unknown' };
    const isFinalStatus = ['failed', 'timeout', 'cancelled', 'completed'].includes(task.status);
    const canCancel = !isFinalStatus && task.status !== 'cancelling' && !isHistory;
    const isSelected = tasksState.selectedTasks.has(task.conversationId);
    const duration = (task.status === 'running' || task.status === 'cancelling') 
        ? calculateDuration(task.startedAt) 
        : '';

    return `
        <div class="task-item ${isHistory ? 'task-item-history' : ''}" data-task-id="${task.conversationId}" data-started-at="${task.startedAt}" data-status="${task.status}">
            <div class="task-header">
                <div class="task-info">
                    ${canCancel ? `
                        <label class="task-checkbox">
                            <input type="checkbox" ${isSelected ? 'checked' : ''} 
                                   onchange="toggleTaskSelection('${task.conversationId}', this.checked)">
                        </label>
                    ` : '<div class="task-checkbox-placeholder"></div>'}
                    <span class="task-status ${status.class}">${status.text}</span>
                    ${isHistory ? '<span class="task-history-badge" title="历史记录">📜</span>' : ''}
                    <span class="task-message" title="${escapeHtml(task.message || '未命名任务')}">${escapeHtml(task.message || '未命名任务')}</span>
                </div>
                <div class="task-actions">
                    ${duration ? `<span class="task-duration" title="执行时长">⏱ ${duration}</span>` : ''}
                    <span class="task-time" title="${isHistory && completedText ? '完成时间' : '开始时间'}">
                        ${isHistory && completedText ? completedText : timeText}
                    </span>
                    ${canCancel ? `<button class="btn-secondary btn-small" onclick="cancelTask('${task.conversationId}', this)">取消任务</button>` : ''}
                    ${task.conversationId ? `<button class="btn-secondary btn-small" onclick="viewConversation('${task.conversationId}')">查看对话</button>` : ''}
                </div>
            </div>
            ${task.conversationId ? `
                <div class="task-details">
                    <span class="task-id-label">对话ID:</span>
                    <span class="task-id-value" title="点击复制" onclick="copyTaskId('${task.conversationId}')">${escapeHtml(task.conversationId)}</span>
                </div>
            ` : ''}
        </div>
    `;
}

// 清空任务历史
function clearTasksHistory() {
    if (!confirm('确定要清空所有任务历史记录吗？')) {
        return;
    }
    tasksState.completedTasksHistory = [];
    saveCompletedTasksHistory();
    filterAndSortTasks();
}

// 切换任务选择
function toggleTaskSelection(conversationId, selected) {
    if (selected) {
        tasksState.selectedTasks.add(conversationId);
    } else {
        tasksState.selectedTasks.delete(conversationId);
    }
    updateBatchActions();
}

// 更新批量操作UI
function updateBatchActions() {
    const batchActions = document.getElementById('tasks-batch-actions');
    const selectedCount = document.getElementById('tasks-selected-count');
    
    if (!batchActions || !selectedCount) return;
    
    const count = tasksState.selectedTasks.size;
    if (count > 0) {
        batchActions.style.display = 'flex';
        selectedCount.textContent = `已选择 ${count} 项`;
    } else {
        batchActions.style.display = 'none';
    }
}

// 清除任务选择
function clearTaskSelection() {
    tasksState.selectedTasks.clear();
    updateBatchActions();
    // 重新渲染以更新复选框状态
    filterAndSortTasks();
}

// 批量取消任务
async function batchCancelTasks() {
    const selected = Array.from(tasksState.selectedTasks);
    if (selected.length === 0) return;
    
    if (!confirm(`确定要取消 ${selected.length} 个任务吗？`)) {
        return;
    }
    
    let successCount = 0;
    let failCount = 0;
    
    for (const conversationId of selected) {
        try {
            const response = await apiFetch('/api/agent-loop/cancel', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ conversationId }),
            });
            
            if (response.ok) {
                successCount++;
            } else {
                failCount++;
            }
        } catch (error) {
            console.error('取消任务失败:', conversationId, error);
            failCount++;
        }
    }
    
    // 清除选择
    clearTaskSelection();
    
    // 刷新任务列表
    await loadTasks();
    
    // 显示结果
    if (failCount > 0) {
        alert(`批量取消完成：成功 ${successCount} 个，失败 ${failCount} 个`);
    } else {
        alert(`成功取消 ${successCount} 个任务`);
    }
}

// 复制任务ID
function copyTaskId(conversationId) {
    navigator.clipboard.writeText(conversationId).then(() => {
        // 显示复制成功提示
        const tooltip = document.createElement('div');
        tooltip.textContent = '已复制!';
        tooltip.style.cssText = 'position: fixed; top: 50%; left: 50%; transform: translate(-50%, -50%); background: rgba(0,0,0,0.8); color: white; padding: 8px 16px; border-radius: 4px; z-index: 10000;';
        document.body.appendChild(tooltip);
        setTimeout(() => tooltip.remove(), 1000);
    }).catch(err => {
        console.error('复制失败:', err);
    });
}

// 取消任务
async function cancelTask(conversationId, button) {
    if (!conversationId) return;
    
    const originalText = button.textContent;
    button.disabled = true;
    button.textContent = '取消中...';

    try {
        const response = await apiFetch('/api/agent-loop/cancel', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ conversationId }),
        });

        if (!response.ok) {
            const result = await response.json().catch(() => ({}));
            throw new Error(result.error || '取消任务失败');
        }

        // 从选择中移除
        tasksState.selectedTasks.delete(conversationId);
        updateBatchActions();
        
        // 重新加载任务列表
        await loadTasks();
    } catch (error) {
        console.error('取消任务失败:', error);
        alert('取消任务失败: ' + error.message);
        button.disabled = false;
        button.textContent = originalText;
    }
}

// 查看对话
function viewConversation(conversationId) {
    if (!conversationId) return;
    
    // 切换到对话页面
    if (typeof switchPage === 'function') {
        switchPage('chat');
        // 加载并选中该对话 - 使用全局函数
        setTimeout(() => {
            // 尝试多种方式加载对话
            if (typeof loadConversation === 'function') {
                loadConversation(conversationId);
            } else if (typeof window.loadConversation === 'function') {
                window.loadConversation(conversationId);
            } else {
                // 如果函数不存在，尝试通过URL跳转
                window.location.hash = `chat?conversation=${conversationId}`;
                console.log('切换到对话页面，对话ID:', conversationId);
            }
        }, 500);
    }
}

// 刷新任务列表
async function refreshTasks() {
    await loadTasks();
}

// 切换自动刷新
function toggleTasksAutoRefresh(enabled) {
    tasksState.autoRefresh = enabled;
    
    // 保存到localStorage
    localStorage.setItem('tasks-auto-refresh', enabled ? 'true' : 'false');
    
    if (enabled) {
        // 启动自动刷新
        if (!tasksState.refreshInterval) {
            tasksState.refreshInterval = setInterval(() => {
                loadBatchQueues();
            }, 5000);
        }
    } else {
        // 停止自动刷新
        if (tasksState.refreshInterval) {
            clearInterval(tasksState.refreshInterval);
            tasksState.refreshInterval = null;
        }
    }
}

// 初始化任务管理页面
function initTasksPage() {
    // 恢复自动刷新设置
    const autoRefreshCheckbox = document.getElementById('tasks-auto-refresh');
    if (autoRefreshCheckbox) {
        const saved = localStorage.getItem('tasks-auto-refresh');
        const enabled = saved !== null ? saved === 'true' : true;
        autoRefreshCheckbox.checked = enabled;
        toggleTasksAutoRefresh(enabled);
    } else {
        toggleTasksAutoRefresh(true);
    }
    
    // 只加载批量任务队列
    loadBatchQueues();
}

// 清理定时器（页面切换时调用）
function cleanupTasksPage() {
    if (tasksState.refreshInterval) {
        clearInterval(tasksState.refreshInterval);
        tasksState.refreshInterval = null;
    }
    if (tasksState.durationUpdateInterval) {
        clearInterval(tasksState.durationUpdateInterval);
        tasksState.durationUpdateInterval = null;
    }
    tasksState.selectedTasks.clear();
    stopBatchQueueRefresh();
}

// 导出函数供全局使用
window.loadTasks = loadTasks;
window.cancelTask = cancelTask;
window.viewConversation = viewConversation;
window.refreshTasks = refreshTasks;
window.initTasksPage = initTasksPage;
window.cleanupTasksPage = cleanupTasksPage;
window.filterTasks = filterTasks;
window.sortTasks = sortTasks;
window.toggleTaskSelection = toggleTaskSelection;
window.clearTaskSelection = clearTaskSelection;
window.batchCancelTasks = batchCancelTasks;
window.copyTaskId = copyTaskId;
window.toggleTasksAutoRefresh = toggleTasksAutoRefresh;
window.toggleShowHistory = toggleShowHistory;
window.clearTasksHistory = clearTasksHistory;

// ==================== 批量任务功能 ====================

// 批量任务状态
const batchQueuesState = {
    queues: [],
    currentQueueId: null,
    refreshInterval: null,
    // 筛选和分页状态
    filterStatus: 'all', // 'all', 'pending', 'running', 'paused', 'completed', 'cancelled'
    searchKeyword: '',
    currentPage: 1,
    pageSize: 10,
    total: 0,
    totalPages: 1
};

// 显示新建任务模态框
async function showBatchImportModal() {
    const modal = document.getElementById('batch-import-modal');
    const input = document.getElementById('batch-tasks-input');
    const titleInput = document.getElementById('batch-queue-title');
    const roleSelect = document.getElementById('batch-queue-role');
    if (modal && input) {
        input.value = '';
        if (titleInput) {
            titleInput.value = '';
        }
        // 重置角色选择为默认
        if (roleSelect) {
            roleSelect.value = '';
        }
        updateBatchImportStats('');
        
        // 加载并填充角色列表
        if (roleSelect && typeof loadRoles === 'function') {
            try {
                const loadedRoles = await loadRoles();
                // 清空现有选项（除了默认选项）
                roleSelect.innerHTML = '<option value="">默认</option>';
                
                // 添加已启用的角色
                const sortedRoles = loadedRoles.sort((a, b) => {
                    if (a.name === '默认') return -1;
                    if (b.name === '默认') return 1;
                    return (a.name || '').localeCompare(b.name || '', 'zh-CN');
                });
                
                sortedRoles.forEach(role => {
                    if (role.name !== '默认' && role.enabled !== false) {
                        const option = document.createElement('option');
                        option.value = role.name;
                        option.textContent = role.name;
                        roleSelect.appendChild(option);
                    }
                });
            } catch (error) {
                console.error('加载角色列表失败:', error);
            }
        }
        
        modal.style.display = 'block';
        input.focus();
    }
}

// 关闭新建任务模态框
function closeBatchImportModal() {
    const modal = document.getElementById('batch-import-modal');
    if (modal) {
        modal.style.display = 'none';
    }
}

// 更新新建任务统计
function updateBatchImportStats(text) {
    const statsEl = document.getElementById('batch-import-stats');
    if (!statsEl) return;
    
    const lines = text.split('\n').filter(line => line.trim() !== '');
    const count = lines.length;
    
    if (count > 0) {
        statsEl.innerHTML = `<div class="batch-import-stat">共 ${count} 个任务</div>`;
        statsEl.style.display = 'block';
    } else {
        statsEl.style.display = 'none';
    }
}

// 监听批量任务输入
document.addEventListener('DOMContentLoaded', function() {
    const input = document.getElementById('batch-tasks-input');
    if (input) {
        input.addEventListener('input', function() {
            updateBatchImportStats(this.value);
        });
    }
});

// 创建批量任务队列
async function createBatchQueue() {
    const input = document.getElementById('batch-tasks-input');
    const titleInput = document.getElementById('batch-queue-title');
    const roleSelect = document.getElementById('batch-queue-role');
    if (!input) return;
    
    const text = input.value.trim();
    if (!text) {
        alert('请输入至少一个任务');
        return;
    }
    
    // 按行分割任务
    const tasks = text.split('\n').map(line => line.trim()).filter(line => line !== '');
    if (tasks.length === 0) {
        alert('没有有效的任务');
        return;
    }
    
    // 获取标题（可选）
    const title = titleInput ? titleInput.value.trim() : '';
    
    // 获取角色（可选，空字符串表示默认角色）
    const role = roleSelect ? roleSelect.value || '' : '';
    
    try {
        const response = await apiFetch('/api/batch-tasks', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ title, tasks, role }),
        });
        
        if (!response.ok) {
            const result = await response.json().catch(() => ({}));
            throw new Error(result.error || '创建批量任务队列失败');
        }
        
        const result = await response.json();
        closeBatchImportModal();
        
        // 显示队列详情
        showBatchQueueDetail(result.queueId);
        
        // 刷新批量队列列表
        refreshBatchQueues();
    } catch (error) {
        console.error('创建批量任务队列失败:', error);
        alert('创建批量任务队列失败: ' + error.message);
    }
}

// 获取角色图标（辅助函数）
function getRoleIconForDisplay(roleName, rolesList) {
    if (!roleName || roleName === '') {
        return '🔵'; // 默认角色图标
    }
    
    if (Array.isArray(rolesList) && rolesList.length > 0) {
        const role = rolesList.find(r => r.name === roleName);
        if (role && role.icon) {
            let icon = role.icon;
            // 检查是否是 Unicode 转义格式（可能包含引号）
            const unicodeMatch = icon.match(/^"?\\U([0-9A-F]{8})"?$/i);
            if (unicodeMatch) {
                try {
                    const codePoint = parseInt(unicodeMatch[1], 16);
                    icon = String.fromCodePoint(codePoint);
                } catch (e) {
                    // 转换失败，使用默认图标
                    console.warn('转换 icon Unicode 转义失败:', icon, e);
                    return '👤';
                }
            }
            return icon;
        }
    }
    return '👤'; // 默认图标
}

// 加载批量任务队列列表
async function loadBatchQueues(page) {
    const section = document.getElementById('batch-queues-section');
    if (!section) return;
    
    // 如果指定了page，使用它；否则使用当前页
    if (page !== undefined) {
        batchQueuesState.currentPage = page;
    }
    
    // 加载角色列表（用于显示正确的角色图标）
    let loadedRoles = [];
    if (typeof loadRoles === 'function') {
        try {
            loadedRoles = await loadRoles();
        } catch (error) {
            console.warn('加载角色列表失败，将使用默认图标:', error);
        }
    }
    batchQueuesState.loadedRoles = loadedRoles; // 保存到状态中供渲染使用
    
    // 构建查询参数
    const params = new URLSearchParams();
    params.append('page', batchQueuesState.currentPage.toString());
    params.append('limit', batchQueuesState.pageSize.toString());
    if (batchQueuesState.filterStatus && batchQueuesState.filterStatus !== 'all') {
        params.append('status', batchQueuesState.filterStatus);
    }
    if (batchQueuesState.searchKeyword) {
        params.append('keyword', batchQueuesState.searchKeyword);
    }
    
    try {
        const response = await apiFetch(`/api/batch-tasks?${params.toString()}`);
        if (!response.ok) {
            throw new Error('获取批量任务队列失败');
        }
        
        const result = await response.json();
        batchQueuesState.queues = result.queues || [];
        batchQueuesState.total = result.total || 0;
        batchQueuesState.totalPages = result.total_pages || 1;
        renderBatchQueues();
    } catch (error) {
        console.error('加载批量任务队列失败:', error);
        section.style.display = 'block';
        const list = document.getElementById('batch-queues-list');
        if (list) {
            list.innerHTML = '<div class="tasks-empty"><p>加载失败: ' + escapeHtml(error.message) + '</p><button class="btn-secondary" onclick="refreshBatchQueues()">重试</button></div>';
        }
    }
}

// 筛选批量任务队列
function filterBatchQueues() {
    const statusFilter = document.getElementById('batch-queues-status-filter');
    const searchInput = document.getElementById('batch-queues-search');
    
    if (statusFilter) {
        batchQueuesState.filterStatus = statusFilter.value;
    }
    if (searchInput) {
        batchQueuesState.searchKeyword = searchInput.value.trim();
    }
    
    // 重置到第一页并重新加载
    batchQueuesState.currentPage = 1;
    loadBatchQueues(1);
}

// 渲染批量任务队列列表
function renderBatchQueues() {
    const section = document.getElementById('batch-queues-section');
    const list = document.getElementById('batch-queues-list');
    const pagination = document.getElementById('batch-queues-pagination');
    
    if (!section || !list) return;
    
    section.style.display = 'block';
    
    const queues = batchQueuesState.queues;
    
    if (queues.length === 0) {
        list.innerHTML = '<div class="tasks-empty"><p>当前没有批量任务队列</p></div>';
        if (pagination) pagination.style.display = 'none';
        return;
    }
    
    // 确保分页控件可见（重置之前可能设置的 display: none）
    if (pagination) {
        pagination.style.display = '';
    }
    
    list.innerHTML = queues.map(queue => {
        const statusMap = {
            'pending': { text: '待执行', class: 'batch-queue-status-pending' },
            'running': { text: '执行中', class: 'batch-queue-status-running' },
            'paused': { text: '已暂停', class: 'batch-queue-status-paused' },
            'completed': { text: '已完成', class: 'batch-queue-status-completed' },
            'cancelled': { text: '已取消', class: 'batch-queue-status-cancelled' }
        };
        
        const status = statusMap[queue.status] || { text: queue.status, class: 'batch-queue-status-unknown' };
        
        // 统计任务状态
        const stats = {
            total: queue.tasks.length,
            pending: 0,
            running: 0,
            completed: 0,
            failed: 0,
            cancelled: 0
        };
        
        queue.tasks.forEach(task => {
            if (task.status === 'pending') stats.pending++;
            else if (task.status === 'running') stats.running++;
            else if (task.status === 'completed') stats.completed++;
            else if (task.status === 'failed') stats.failed++;
            else if (task.status === 'cancelled') stats.cancelled++;
        });
        
        const progress = stats.total > 0 ? Math.round((stats.completed + stats.failed + stats.cancelled) / stats.total * 100) : 0;
        // 允许删除待执行、已完成或已取消状态的队列
        const canDelete = queue.status === 'pending' || queue.status === 'completed' || queue.status === 'cancelled';
        
        const titleDisplay = queue.title ? `<span class="batch-queue-title" style="font-weight: 600; color: var(--text-primary); margin-right: 8px;">${escapeHtml(queue.title)}</span>` : '';
        
        // 显示角色信息（使用正确的角色图标）
        const loadedRoles = batchQueuesState.loadedRoles || [];
        const roleIcon = getRoleIconForDisplay(queue.role, loadedRoles);
        const roleName = queue.role && queue.role !== '' ? queue.role : '默认';
        const roleDisplay = `<span class="batch-queue-role" style="margin-right: 8px;" title="角色: ${escapeHtml(roleName)}">${roleIcon} ${escapeHtml(roleName)}</span>`;
        
        return `
            <div class="batch-queue-item" data-queue-id="${queue.id}" onclick="showBatchQueueDetail('${queue.id}')">
                <div class="batch-queue-header">
                    <div class="batch-queue-info" style="flex: 1;">
                        ${titleDisplay}
                        ${roleDisplay}
                        <span class="batch-queue-status ${status.class}">${status.text}</span>
                        <span class="batch-queue-id">队列ID: ${escapeHtml(queue.id)}</span>
                        <span class="batch-queue-time">创建时间: ${new Date(queue.createdAt).toLocaleString('zh-CN')}</span>
                    </div>
                    <div class="batch-queue-progress">
                        <div class="batch-queue-progress-bar">
                            <div class="batch-queue-progress-fill" style="width: ${progress}%"></div>
                        </div>
                        <span class="batch-queue-progress-text">${progress}% (${stats.completed + stats.failed + stats.cancelled}/${stats.total})</span>
                    </div>
                    <div class="batch-queue-actions" style="display: flex; align-items: center; gap: 8px; margin-left: 12px;" onclick="event.stopPropagation();">
                        ${canDelete ? `<button class="btn-secondary btn-small btn-danger" onclick="deleteBatchQueueFromList('${queue.id}')" title="删除队列">删除</button>` : ''}
                    </div>
                </div>
                <div class="batch-queue-stats">
                    <span>总计: ${stats.total}</span>
                    <span>待执行: ${stats.pending}</span>
                    <span>执行中: ${stats.running}</span>
                    <span style="color: var(--success-color);">已完成: ${stats.completed}</span>
                    <span style="color: var(--error-color);">失败: ${stats.failed}</span>
                    ${stats.cancelled > 0 ? `<span style="color: var(--text-secondary);">已取消: ${stats.cancelled}</span>` : ''}
                </div>
            </div>
        `;
    }).join('');
    
    // 渲染分页控件
    renderBatchQueuesPagination();
}

// 渲染批量任务队列分页控件（参考Skills管理页面样式）
function renderBatchQueuesPagination() {
    const paginationContainer = document.getElementById('batch-queues-pagination');
    if (!paginationContainer) return;
    
    const { currentPage, pageSize, total, totalPages } = batchQueuesState;
    
    // 即使只有一页也显示分页信息（参考Skills样式）
    if (total === 0) {
        paginationContainer.innerHTML = '';
        return;
    }
    
    // 计算显示范围
    const start = total === 0 ? 0 : (currentPage - 1) * pageSize + 1;
    const end = total === 0 ? 0 : Math.min(currentPage * pageSize, total);
    
    let paginationHTML = '<div class="pagination">';
    
    // 左侧：显示范围信息和每页数量选择器（参考Skills样式）
    paginationHTML += `
        <div class="pagination-info">
            <span>显示 ${start}-${end} / 共 ${total} 条</span>
            <label class="pagination-page-size">
                每页显示
                <select id="batch-queues-page-size-pagination" onchange="changeBatchQueuesPageSize()">
                    <option value="10" ${pageSize === 10 ? 'selected' : ''}>10</option>
                    <option value="20" ${pageSize === 20 ? 'selected' : ''}>20</option>
                    <option value="50" ${pageSize === 50 ? 'selected' : ''}>50</option>
                    <option value="100" ${pageSize === 100 ? 'selected' : ''}>100</option>
                </select>
            </label>
        </div>
    `;
    
    // 右侧：分页按钮（参考Skills样式：首页、上一页、第X/Y页、下一页、末页）
    paginationHTML += `
        <div class="pagination-controls">
            <button class="btn-secondary" onclick="goBatchQueuesPage(1)" ${currentPage === 1 || total === 0 ? 'disabled' : ''}>首页</button>
            <button class="btn-secondary" onclick="goBatchQueuesPage(${currentPage - 1})" ${currentPage === 1 || total === 0 ? 'disabled' : ''}>上一页</button>
            <span class="pagination-page">第 ${currentPage} / ${totalPages || 1} 页</span>
            <button class="btn-secondary" onclick="goBatchQueuesPage(${currentPage + 1})" ${currentPage >= totalPages || total === 0 ? 'disabled' : ''}>下一页</button>
            <button class="btn-secondary" onclick="goBatchQueuesPage(${totalPages || 1})" ${currentPage >= totalPages || total === 0 ? 'disabled' : ''}>末页</button>
        </div>
    `;
    
    paginationHTML += '</div>';
    
    paginationContainer.innerHTML = paginationHTML;
    
    // 确保分页组件与列表内容区域对齐（不包括滚动条）
    function alignPaginationWidth() {
        const batchQueuesList = document.getElementById('batch-queues-list');
        if (batchQueuesList && paginationContainer) {
            // 获取列表的实际内容宽度（不包括滚动条）
            const listClientWidth = batchQueuesList.clientWidth; // 可视区域宽度（不包括滚动条）
            const listScrollHeight = batchQueuesList.scrollHeight; // 内容总高度
            const listClientHeight = batchQueuesList.clientHeight; // 可视区域高度
            const hasScrollbar = listScrollHeight > listClientHeight;
            
            // 如果列表有垂直滚动条，分页组件应该与列表内容区域对齐（clientWidth）
            // 如果没有滚动条，使用100%宽度
            if (hasScrollbar) {
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
    
    const batchQueuesList = document.getElementById('batch-queues-list');
    if (batchQueuesList) {
        resizeObserver.observe(batchQueuesList);
    }
}

// 跳转到指定页面
function goBatchQueuesPage(page) {
    const { totalPages } = batchQueuesState;
    if (page < 1 || page > totalPages) return;
    
    loadBatchQueues(page);
    
    // 滚动到列表顶部
    const list = document.getElementById('batch-queues-list');
    if (list) {
        list.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }
}

// 改变每页显示数量
function changeBatchQueuesPageSize() {
    const pageSizeSelect = document.getElementById('batch-queues-page-size-pagination');
    if (!pageSizeSelect) return;
    
    const newPageSize = parseInt(pageSizeSelect.value, 10);
    if (newPageSize && newPageSize > 0) {
        batchQueuesState.pageSize = newPageSize;
        batchQueuesState.currentPage = 1; // 重置到第一页
        loadBatchQueues(1);
    }
}

// 显示批量任务队列详情
async function showBatchQueueDetail(queueId) {
    const modal = document.getElementById('batch-queue-detail-modal');
    const title = document.getElementById('batch-queue-detail-title');
    const content = document.getElementById('batch-queue-detail-content');
        const startBtn = document.getElementById('batch-queue-start-btn');
        const cancelBtn = document.getElementById('batch-queue-cancel-btn');
        const deleteBtn = document.getElementById('batch-queue-delete-btn');
        const addTaskBtn = document.getElementById('batch-queue-add-task-btn');
        
        if (!modal || !content) return;
        
        try {
        // 加载角色列表（如果还未加载）
        let loadedRoles = [];
        if (typeof loadRoles === 'function') {
            try {
                loadedRoles = await loadRoles();
            } catch (error) {
                console.warn('加载角色列表失败，将使用默认图标:', error);
            }
        }
        
        const response = await apiFetch(`/api/batch-tasks/${queueId}`);
        if (!response.ok) {
            throw new Error('获取队列详情失败');
        }
        
        const result = await response.json();
        const queue = result.queue;
        batchQueuesState.currentQueueId = queueId;
        
        if (title) {
            title.textContent = queue.title ? `批量任务队列 - ${escapeHtml(queue.title)}` : '批量任务队列';
        }
        
        // 更新按钮显示
        const pauseBtn = document.getElementById('batch-queue-pause-btn');
        if (addTaskBtn) {
            addTaskBtn.style.display = queue.status === 'pending' ? 'inline-block' : 'none';
        }
        if (startBtn) {
            // pending状态显示"开始执行"，paused状态显示"继续执行"
            startBtn.style.display = (queue.status === 'pending' || queue.status === 'paused') ? 'inline-block' : 'none';
            if (startBtn && queue.status === 'paused') {
                startBtn.textContent = '继续执行';
            } else if (startBtn && queue.status === 'pending') {
                startBtn.textContent = '开始执行';
            }
        }
        if (pauseBtn) {
            // running状态显示"暂停队列"
            pauseBtn.style.display = queue.status === 'running' ? 'inline-block' : 'none';
        }
        if (deleteBtn) {
            // 允许删除待执行、已完成或已取消状态的队列
            deleteBtn.style.display = (queue.status === 'pending' || queue.status === 'completed' || queue.status === 'cancelled' || queue.status === 'paused') ? 'inline-block' : 'none';
        }
        
        // 队列状态映射
        const queueStatusMap = {
            'pending': { text: '待执行', class: 'batch-queue-status-pending' },
            'running': { text: '执行中', class: 'batch-queue-status-running' },
            'paused': { text: '已暂停', class: 'batch-queue-status-paused' },
            'completed': { text: '已完成', class: 'batch-queue-status-completed' },
            'cancelled': { text: '已取消', class: 'batch-queue-status-cancelled' }
        };
        
        // 任务状态映射
        const taskStatusMap = {
            'pending': { text: '待执行', class: 'batch-task-status-pending' },
            'running': { text: '执行中', class: 'batch-task-status-running' },
            'completed': { text: '已完成', class: 'batch-task-status-completed' },
            'failed': { text: '失败', class: 'batch-task-status-failed' },
            'cancelled': { text: '已取消', class: 'batch-task-status-cancelled' }
        };
        
        // 获取角色信息（如果队列有角色配置）
        let roleDisplay = '';
        if (queue.role && queue.role !== '') {
            // 如果有角色配置，尝试获取角色详细信息
            let roleName = queue.role;
            let roleIcon = '👤';
            // 从已加载的角色列表中查找角色图标
            if (Array.isArray(loadedRoles) && loadedRoles.length > 0) {
                const role = loadedRoles.find(r => r.name === roleName);
                if (role && role.icon) {
                    let icon = role.icon;
                    const unicodeMatch = icon.match(/^"?\\U([0-9A-F]{8})"?$/i);
                    if (unicodeMatch) {
                        try {
                            const codePoint = parseInt(unicodeMatch[1], 16);
                            icon = String.fromCodePoint(codePoint);
                        } catch (e) {
                            // 转换失败，使用默认图标
                        }
                    }
                    roleIcon = icon;
                }
            }
            roleDisplay = `<div class="detail-item">
                <span class="detail-label">角色</span>
                <span class="detail-value">${roleIcon} ${escapeHtml(roleName)}</span>
            </div>`;
        } else {
            // 默认角色
            roleDisplay = `<div class="detail-item">
                <span class="detail-label">角色</span>
                <span class="detail-value">🔵 默认</span>
            </div>`;
        }
        
        content.innerHTML = `
            <div class="batch-queue-detail-info">
                ${queue.title ? `<div class="detail-item">
                    <span class="detail-label">任务标题</span>
                    <span class="detail-value">${escapeHtml(queue.title)}</span>
                </div>` : ''}
                ${roleDisplay}
                <div class="detail-item">
                    <span class="detail-label">队列ID</span>
                    <span class="detail-value"><code>${escapeHtml(queue.id)}</code></span>
                </div>
                <div class="detail-item">
                    <span class="detail-label">状态</span>
                    <span class="detail-value"><span class="batch-queue-status ${queueStatusMap[queue.status]?.class || ''}">${queueStatusMap[queue.status]?.text || queue.status}</span></span>
                </div>
                <div class="detail-item">
                    <span class="detail-label">创建时间</span>
                    <span class="detail-value">${new Date(queue.createdAt).toLocaleString('zh-CN')}</span>
                </div>
                ${queue.startedAt ? `<div class="detail-item">
                    <span class="detail-label">开始时间</span>
                    <span class="detail-value">${new Date(queue.startedAt).toLocaleString('zh-CN')}</span>
                </div>` : ''}
                ${queue.completedAt ? `<div class="detail-item">
                    <span class="detail-label">完成时间</span>
                    <span class="detail-value">${new Date(queue.completedAt).toLocaleString('zh-CN')}</span>
                </div>` : ''}
                <div class="detail-item">
                    <span class="detail-label">任务总数</span>
                    <span class="detail-value">${queue.tasks.length}</span>
                </div>
            </div>
            <div class="batch-queue-tasks-list">
                <h4>任务列表</h4>
                ${queue.tasks.map((task, index) => {
                    const taskStatus = taskStatusMap[task.status] || { text: task.status, class: 'batch-task-status-unknown' };
                    const canEdit = queue.status === 'pending' && task.status === 'pending';
                    const taskMessageEscaped = escapeHtml(task.message).replace(/'/g, "&#39;").replace(/"/g, "&quot;").replace(/\n/g, "\\n");
                    return `
                        <div class="batch-task-item ${task.status === 'running' ? 'batch-task-item-active' : ''}" data-queue-id="${queue.id}" data-task-id="${task.id}" data-task-message="${taskMessageEscaped}">
                            <div class="batch-task-header">
                                <span class="batch-task-index">#${index + 1}</span>
                                <span class="batch-task-status ${taskStatus.class}">${taskStatus.text}</span>
                                <span class="batch-task-message" title="${escapeHtml(task.message)}">${escapeHtml(task.message)}</span>
                                ${canEdit ? `<button class="btn-secondary btn-small batch-task-edit-btn" onclick="editBatchTaskFromElement(this); event.stopPropagation();">编辑</button>` : ''}
                                ${canEdit ? `<button class="btn-secondary btn-small btn-danger batch-task-delete-btn" onclick="deleteBatchTaskFromElement(this); event.stopPropagation();">删除</button>` : ''}
                                ${task.conversationId ? `<button class="btn-secondary btn-small" onclick="viewBatchTaskConversation('${task.conversationId}'); event.stopPropagation();">查看对话</button>` : ''}
                            </div>
                            ${task.startedAt ? `<div class="batch-task-time">开始: ${new Date(task.startedAt).toLocaleString('zh-CN')}</div>` : ''}
                            ${task.completedAt ? `<div class="batch-task-time">完成: ${new Date(task.completedAt).toLocaleString('zh-CN')}</div>` : ''}
                            ${task.error ? `<div class="batch-task-error">错误: ${escapeHtml(task.error)}</div>` : ''}
                            ${task.result ? `<div class="batch-task-result">结果: ${escapeHtml(task.result.substring(0, 200))}${task.result.length > 200 ? '...' : ''}</div>` : ''}
                        </div>
                    `;
                }).join('')}
            </div>
        `;
        
        modal.style.display = 'block';
        
        // 如果队列正在运行，自动刷新
        if (queue.status === 'running') {
            startBatchQueueRefresh(queueId);
        }
    } catch (error) {
        console.error('获取队列详情失败:', error);
        alert('获取队列详情失败: ' + error.message);
    }
}

// 开始批量任务队列
async function startBatchQueue() {
    const queueId = batchQueuesState.currentQueueId;
    if (!queueId) return;
    
    try {
        const response = await apiFetch(`/api/batch-tasks/${queueId}/start`, {
            method: 'POST',
        });
        
        if (!response.ok) {
            const result = await response.json().catch(() => ({}));
            throw new Error(result.error || '启动批量任务失败');
        }
        
        // 刷新详情
        showBatchQueueDetail(queueId);
        refreshBatchQueues();
    } catch (error) {
        console.error('启动批量任务失败:', error);
        alert('启动批量任务失败: ' + error.message);
    }
}

// 暂停批量任务队列
async function pauseBatchQueue() {
    const queueId = batchQueuesState.currentQueueId;
    if (!queueId) return;
    
    if (!confirm('确定要暂停这个批量任务队列吗？当前正在执行的任务将被停止，后续任务将保留待执行状态。')) {
        return;
    }
    
    try {
        const response = await apiFetch(`/api/batch-tasks/${queueId}/pause`, {
            method: 'POST',
        });
        
        if (!response.ok) {
            const result = await response.json().catch(() => ({}));
            throw new Error(result.error || '暂停批量任务失败');
        }
        
        // 刷新详情
        showBatchQueueDetail(queueId);
        refreshBatchQueues();
    } catch (error) {
        console.error('暂停批量任务失败:', error);
        alert('暂停批量任务失败: ' + error.message);
    }
}

// 删除批量任务队列（从详情模态框）
async function deleteBatchQueue() {
    const queueId = batchQueuesState.currentQueueId;
    if (!queueId) return;
    
    if (!confirm('确定要删除这个批量任务队列吗？此操作不可恢复。')) {
        return;
    }
    
    try {
        const response = await apiFetch(`/api/batch-tasks/${queueId}`, {
            method: 'DELETE',
        });
        
        if (!response.ok) {
            const result = await response.json().catch(() => ({}));
            throw new Error(result.error || '删除批量任务队列失败');
        }
        
        closeBatchQueueDetailModal();
        refreshBatchQueues();
    } catch (error) {
        console.error('删除批量任务队列失败:', error);
        alert('删除批量任务队列失败: ' + error.message);
    }
}

// 从列表删除批量任务队列
async function deleteBatchQueueFromList(queueId) {
    if (!queueId) return;
    
    if (!confirm('确定要删除这个批量任务队列吗？此操作不可恢复。')) {
        return;
    }
    
    try {
        const response = await apiFetch(`/api/batch-tasks/${queueId}`, {
            method: 'DELETE',
        });
        
        if (!response.ok) {
            const result = await response.json().catch(() => ({}));
            throw new Error(result.error || '删除批量任务队列失败');
        }
        
        // 如果当前正在查看这个队列的详情，关闭详情模态框
        if (batchQueuesState.currentQueueId === queueId) {
            closeBatchQueueDetailModal();
        }
        
        // 刷新队列列表
        refreshBatchQueues();
    } catch (error) {
        console.error('删除批量任务队列失败:', error);
        alert('删除批量任务队列失败: ' + error.message);
    }
}

// 关闭批量任务队列详情模态框
function closeBatchQueueDetailModal() {
    const modal = document.getElementById('batch-queue-detail-modal');
    if (modal) {
        modal.style.display = 'none';
    }
    batchQueuesState.currentQueueId = null;
    stopBatchQueueRefresh();
}

// 开始批量队列刷新
function startBatchQueueRefresh(queueId) {
    if (batchQueuesState.refreshInterval) {
        clearInterval(batchQueuesState.refreshInterval);
    }
    
    batchQueuesState.refreshInterval = setInterval(() => {
        if (batchQueuesState.currentQueueId === queueId) {
            showBatchQueueDetail(queueId);
            refreshBatchQueues();
        } else {
            stopBatchQueueRefresh();
        }
    }, 3000); // 每3秒刷新一次
}

// 停止批量队列刷新
function stopBatchQueueRefresh() {
    if (batchQueuesState.refreshInterval) {
        clearInterval(batchQueuesState.refreshInterval);
        batchQueuesState.refreshInterval = null;
    }
}

// 刷新批量任务队列列表
async function refreshBatchQueues() {
    await loadBatchQueues(batchQueuesState.currentPage);
}

// 查看批量任务的对话
function viewBatchTaskConversation(conversationId) {
    if (!conversationId) return;
    
    // 关闭批量任务详情模态框
    closeBatchQueueDetailModal();
    
    // 直接使用URL hash跳转，让router处理页面切换和对话加载
    // 这样更可靠，因为router会确保页面切换完成后再加载对话
    window.location.hash = `chat?conversation=${conversationId}`;
}

// 编辑批量任务的状态
const editBatchTaskState = {
    queueId: null,
    taskId: null
};

// 从元素获取任务信息并打开编辑模态框
function editBatchTaskFromElement(button) {
    const taskItem = button.closest('.batch-task-item');
    if (!taskItem) {
        console.error('无法找到任务项元素');
        return;
    }
    
    const queueId = taskItem.getAttribute('data-queue-id');
    const taskId = taskItem.getAttribute('data-task-id');
    const taskMessage = taskItem.getAttribute('data-task-message');
    
    if (!queueId || !taskId) {
        console.error('任务信息不完整');
        return;
    }
    
    // 解码HTML实体
    const decodedMessage = taskMessage
        .replace(/&#39;/g, "'")
        .replace(/&quot;/g, '"')
        .replace(/\\n/g, '\n');
    
    editBatchTask(queueId, taskId, decodedMessage);
}

// 打开编辑批量任务模态框
function editBatchTask(queueId, taskId, currentMessage) {
    editBatchTaskState.queueId = queueId;
    editBatchTaskState.taskId = taskId;
    
    const modal = document.getElementById('edit-batch-task-modal');
    const messageInput = document.getElementById('edit-task-message');
    
    if (!modal || !messageInput) {
        console.error('编辑任务模态框元素不存在');
        return;
    }
    
    messageInput.value = currentMessage;
    modal.style.display = 'block';
    
    // 聚焦到输入框
    setTimeout(() => {
        messageInput.focus();
        messageInput.select();
    }, 100);
    
    // 添加ESC键监听
    const handleKeyDown = (e) => {
        if (e.key === 'Escape') {
            closeEditBatchTaskModal();
            document.removeEventListener('keydown', handleKeyDown);
        }
    };
    document.addEventListener('keydown', handleKeyDown);
    
    // 添加Enter+Ctrl/Cmd保存功能
    const handleKeyPress = (e) => {
        if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
            e.preventDefault();
            saveBatchTask();
            document.removeEventListener('keydown', handleKeyPress);
        }
    };
    messageInput.addEventListener('keydown', handleKeyPress);
}

// 关闭编辑批量任务模态框
function closeEditBatchTaskModal() {
    const modal = document.getElementById('edit-batch-task-modal');
    if (modal) {
        modal.style.display = 'none';
    }
    editBatchTaskState.queueId = null;
    editBatchTaskState.taskId = null;
}

// 保存批量任务
async function saveBatchTask() {
    const queueId = editBatchTaskState.queueId;
    const taskId = editBatchTaskState.taskId;
    const messageInput = document.getElementById('edit-task-message');
    
    if (!queueId || !taskId) {
        alert('任务信息不完整');
        return;
    }
    
    if (!messageInput) {
        alert('无法获取任务消息输入框');
        return;
    }
    
    const message = messageInput.value.trim();
    if (!message) {
        alert('任务消息不能为空');
        return;
    }
    
    try {
        const response = await apiFetch(`/api/batch-tasks/${queueId}/tasks/${taskId}`, {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ message: message }),
        });
        
        if (!response.ok) {
            const result = await response.json().catch(() => ({}));
            throw new Error(result.error || '更新任务失败');
        }
        
        // 关闭编辑模态框
        closeEditBatchTaskModal();
        
        // 刷新队列详情
        if (batchQueuesState.currentQueueId === queueId) {
            showBatchQueueDetail(queueId);
        }
        
        // 刷新队列列表
        refreshBatchQueues();
    } catch (error) {
        console.error('保存任务失败:', error);
        alert('保存任务失败: ' + error.message);
    }
}

// 显示添加批量任务模态框
function showAddBatchTaskModal() {
    const queueId = batchQueuesState.currentQueueId;
    if (!queueId) {
        alert('队列信息不存在');
        return;
    }
    
    const modal = document.getElementById('add-batch-task-modal');
    const messageInput = document.getElementById('add-task-message');
    
    if (!modal || !messageInput) {
        console.error('添加任务模态框元素不存在');
        return;
    }
    
    messageInput.value = '';
    modal.style.display = 'block';
    
    // 聚焦到输入框
    setTimeout(() => {
        messageInput.focus();
    }, 100);
    
    // 添加ESC键监听
    const handleKeyDown = (e) => {
        if (e.key === 'Escape') {
            closeAddBatchTaskModal();
            document.removeEventListener('keydown', handleKeyDown);
        }
    };
    document.addEventListener('keydown', handleKeyDown);
    
    // 添加Enter+Ctrl/Cmd保存功能
    const handleKeyPress = (e) => {
        if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
            e.preventDefault();
            saveAddBatchTask();
            messageInput.removeEventListener('keydown', handleKeyPress);
        }
    };
    messageInput.addEventListener('keydown', handleKeyPress);
}

// 关闭添加批量任务模态框
function closeAddBatchTaskModal() {
    const modal = document.getElementById('add-batch-task-modal');
    const messageInput = document.getElementById('add-task-message');
    if (modal) {
        modal.style.display = 'none';
    }
    if (messageInput) {
        messageInput.value = '';
    }
}

// 保存添加的批量任务
async function saveAddBatchTask() {
    const queueId = batchQueuesState.currentQueueId;
    const messageInput = document.getElementById('add-task-message');
    
    if (!queueId) {
        alert('队列信息不存在');
        return;
    }
    
    if (!messageInput) {
        alert('无法获取任务消息输入框');
        return;
    }
    
    const message = messageInput.value.trim();
    if (!message) {
        alert('任务消息不能为空');
        return;
    }
    
    try {
        const response = await apiFetch(`/api/batch-tasks/${queueId}/tasks`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ message: message }),
        });
        
        if (!response.ok) {
            const result = await response.json().catch(() => ({}));
            throw new Error(result.error || '添加任务失败');
        }
        
        // 关闭添加任务模态框
        closeAddBatchTaskModal();
        
        // 刷新队列详情
        if (batchQueuesState.currentQueueId === queueId) {
            showBatchQueueDetail(queueId);
        }
        
        // 刷新队列列表
        refreshBatchQueues();
    } catch (error) {
        console.error('添加任务失败:', error);
        alert('添加任务失败: ' + error.message);
    }
}

// 从元素获取任务信息并删除任务
function deleteBatchTaskFromElement(button) {
    const taskItem = button.closest('.batch-task-item');
    if (!taskItem) {
        console.error('无法找到任务项元素');
        return;
    }
    
    const queueId = taskItem.getAttribute('data-queue-id');
    const taskId = taskItem.getAttribute('data-task-id');
    const taskMessage = taskItem.getAttribute('data-task-message');
    
    if (!queueId || !taskId) {
        console.error('任务信息不完整');
        return;
    }
    
    // 解码HTML实体以显示消息
    const decodedMessage = taskMessage
        .replace(/&#39;/g, "'")
        .replace(/&quot;/g, '"')
        .replace(/\\n/g, '\n');
    
    // 截断长消息用于确认对话框
    const displayMessage = decodedMessage.length > 50 
        ? decodedMessage.substring(0, 50) + '...' 
        : decodedMessage;
    
    if (!confirm(`确定要删除这个任务吗？\n\n任务内容: ${displayMessage}\n\n此操作不可恢复。`)) {
        return;
    }
    
    deleteBatchTask(queueId, taskId);
}

// 删除批量任务
async function deleteBatchTask(queueId, taskId) {
    if (!queueId || !taskId) {
        alert('任务信息不完整');
        return;
    }
    
    try {
        const response = await apiFetch(`/api/batch-tasks/${queueId}/tasks/${taskId}`, {
            method: 'DELETE',
        });
        
        if (!response.ok) {
            const result = await response.json().catch(() => ({}));
            throw new Error(result.error || '删除任务失败');
        }
        
        // 刷新队列详情
        if (batchQueuesState.currentQueueId === queueId) {
            showBatchQueueDetail(queueId);
        }
        
        // 刷新队列列表
        refreshBatchQueues();
    } catch (error) {
        console.error('删除任务失败:', error);
        alert('删除任务失败: ' + error.message);
    }
}

// 导出函数
window.showBatchImportModal = showBatchImportModal;
window.closeBatchImportModal = closeBatchImportModal;
window.createBatchQueue = createBatchQueue;
window.showBatchQueueDetail = showBatchQueueDetail;
window.startBatchQueue = startBatchQueue;
window.pauseBatchQueue = pauseBatchQueue;
window.deleteBatchQueue = deleteBatchQueue;
window.closeBatchQueueDetailModal = closeBatchQueueDetailModal;
window.refreshBatchQueues = refreshBatchQueues;
window.viewBatchTaskConversation = viewBatchTaskConversation;
window.editBatchTask = editBatchTask;
window.editBatchTaskFromElement = editBatchTaskFromElement;
window.closeEditBatchTaskModal = closeEditBatchTaskModal;
window.saveBatchTask = saveBatchTask;
window.filterBatchQueues = filterBatchQueues;
window.goBatchQueuesPage = goBatchQueuesPage;
window.changeBatchQueuesPageSize = changeBatchQueuesPageSize;
window.showAddBatchTaskModal = showAddBatchTaskModal;
window.closeAddBatchTaskModal = closeAddBatchTaskModal;
window.saveAddBatchTask = saveAddBatchTask;
window.deleteBatchTaskFromElement = deleteBatchTaskFromElement;
window.deleteBatchQueueFromList = deleteBatchQueueFromList;
