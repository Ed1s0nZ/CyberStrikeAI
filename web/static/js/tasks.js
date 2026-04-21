// taskmanagement pagefunction
function _t(key, opts) {
    return typeof window.t === 'function' ? window.t(key, opts) : key;
}

// HTMLescape function(ifnot defined)
if (typeof escapeHtml === 'undefined') {
    function escapeHtml(text) {
        if (text == null) return '';
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }
}

// taskmanagementstatus
const tasksState = {
    allTasks: [],
    filteredTasks: [],
    selectedTasks: new Set(),
    autoRefresh: true,
    refreshInterval: null,
    durationUpdateInterval: null,
    completedTasksHistory: [], // Saverecentcomplete's taskhistory
 showHistory: true // isShow historyrecord
};

// fromlocalStorageloadcompletedtaskhistory
function loadCompletedTasksHistory() {
    try {
        const saved = localStorage.getItem('tasks-completed-history');
        if (saved) {
            const history = JSON.parse(saved);
            // onlypreserverecent24smallwhenwithincomplete's task
            const now = Date.now();
            const oneDayAgo = now - 24 * 60 * 60 * 1000;
            tasksState.completedTasksHistory = history.filter(task => {
                const completedTime = task.completedAt || task.startedAt;
                return completedTime && new Date(completedTime).getTime() > oneDayAgo;
            });
            // Savecleanupafter's history
            saveCompletedTasksHistory();
        }
    } catch (error) {
        console.error('Failed to load completed task history:', error);
        tasksState.completedTasksHistory = [];
    }
}

// SavecompletedtaskhistorytolocalStorage
function saveCompletedTasksHistory() {
    try {
        localStorage.setItem('tasks-completed-history', JSON.stringify(tasksState.completedTasksHistory));
    } catch (error) {
        console.error('Failed to save completed task history:', error);
    }
}

// updatecompletedtaskhistory
function updateCompletedTasksHistory(currentTasks) {
 // Savecurrentalltaskasfast(used forundertime)
    const currentTaskIds = new Set(currentTasks.map(t => t.conversationId));
    
 // if it isfirst timeload,onlyneedSavecurrenttaskfast
    if (tasksState.allTasks.length === 0) {
        return;
    }
    
    const previousTaskIds = new Set(tasksState.allTasks.map(t => t.conversationId));
    
 // outcomplete's task(beforeexistbutatnotexist's )
 // onlyneedtaskfromlistin,thenascompleted
    const justCompleted = tasksState.allTasks.filter(task => {
        return previousTaskIds.has(task.conversationId) && !currentTaskIds.has(task.conversationId);
    });
    
 // willcomplete's taskaddtohistoryin
    justCompleted.forEach(task => {
 // checkisalready exists(avoidrepeatadd)
        const exists = tasksState.completedTasksHistory.some(t => t.conversationId === task.conversationId);
        if (!exists) {
            // iftaskstatusnotisfinalstatus,markascompleted
            const finalStatus = ['completed', 'failed', 'timeout', 'cancelled'].includes(task.status) 
                ? task.status 
                : 'completed';
            
            tasksState.completedTasksHistory.push({
                conversationId: task.conversationId,
                message: task.message || 'Unnamed task',
                startedAt: task.startedAt,
                status: finalStatus,
                completedAt: new Date().toISOString()
            });
        }
    });
    
    // limithistoryrecordcount(at mostpreserve50record)
    if (tasksState.completedTasksHistory.length > 50) {
        tasksState.completedTasksHistory = tasksState.completedTasksHistory
            .sort((a, b) => new Date(b.completedAt || b.startedAt) - new Date(a.completedAt || a.startedAt))
            .slice(0, 50);
    }
    
    saveCompletedTasksHistory();
}

// loadtasklist
async function loadTasks() {
    const listContainer = document.getElementById('tasks-list');
    if (!listContainer) return;
    
    listContainer.innerHTML = '<div class="loading-spinner">' + _t('tasks.loadingTasks') + '</div>';

    try {
 // lineloadrunningin's taskandcompleted's taskhistory
        const [activeResponse, completedResponse] = await Promise.allSettled([
            apiFetch('/api/agent-loop/tasks'),
            apiFetch('/api/agent-loop/tasks/completed').catch(() => null) // ifAPInotexist,returnnull
        ]);

        // processrunningin's task
        if (activeResponse.status === 'rejected' || !activeResponse.value || !activeResponse.value.ok) {
            throw new Error(_t('tasks.loadTaskListFailed'));
        }

        const activeResult = await activeResponse.value.json();
        const activeTasks = activeResult.tasks || [];
        
        // loadcompletedtaskhistory(ifAPIavailable)
        let completedTasks = [];
        if (completedResponse.status === 'fulfilled' && completedResponse.value && completedResponse.value.ok) {
            try {
                const completedResult = await completedResponse.value.json();
                completedTasks = completedResult.tasks || [];
            } catch (e) {
                console.warn('Failed to parse completed task history:', e);
            }
        }
        
        // Savealltask
        tasksState.allTasks = activeTasks;
        
        // updatecompletedtaskhistory(frombackendAPIget)
        if (completedTasks.length > 0) {
 // mergebackendhistoryrecordandlocalhistoryrecord(to)
            const backendTaskIds = new Set(completedTasks.map(t => t.conversationId));
            const localHistory = tasksState.completedTasksHistory.filter(t => 
                !backendTaskIds.has(t.conversationId)
            );
            
 // backend's historyrecordprefer,afteraddlocalhas's 
            tasksState.completedTasksHistory = [
                ...completedTasks.map(t => ({
                    conversationId: t.conversationId,
                    message: t.message || 'Unnamed task',
                    startedAt: t.startedAt,
                    status: t.status || 'completed',
                    completedAt: t.completedAt || new Date().toISOString()
                })),
                ...localHistory
            ];
            
            // limithistoryrecordcount
            if (tasksState.completedTasksHistory.length > 50) {
                tasksState.completedTasksHistory = tasksState.completedTasksHistory
                    .sort((a, b) => new Date(b.completedAt || b.startedAt) - new Date(a.completedAt || a.startedAt))
                    .slice(0, 50);
            }
            
            saveCompletedTasksHistory();
        } else {
 // ifbackendAPInotavailable,stillusefrontendupdatehistory
            updateCompletedTasksHistory(activeTasks);
        }
        
        updateTaskStats(activeTasks);
        filterAndSortTasks();
        startDurationUpdates();
    } catch (error) {
        console.error('Failed to load tasks:', error);
        listContainer.innerHTML = `
            <div class="tasks-empty">
                <p>${_t('tasks.loadFailedRetry')}: ${escapeHtml(error.message)}</p>
                <button class="btn-secondary" onclick="loadTasks()">${_t('tasks.retry')}</button>
            </div>
        `;
    }
}

// updatetaskstatistics
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

// Filtertask
function filterTasks() {
    filterAndSortTasks();
}

// sorttask
function sortTasks() {
    filterAndSortTasks();
}

// Filterandsorttask
function filterAndSortTasks() {
    const statusFilter = document.getElementById('tasks-status-filter')?.value || 'all';
    const sortBy = document.getElementById('tasks-sort-by')?.value || 'time-desc';
    
    // mergecurrenttaskandhistorytask
    let allTasks = [...tasksState.allTasks];
    
    // ifShow historyrecord,addhistorytask
    if (tasksState.showHistory) {
        const historyTasks = tasksState.completedTasksHistory
            .filter(ht => !tasksState.allTasks.some(t => t.conversationId === ht.conversationId))
            .map(ht => ({ ...ht, isHistory: true }));
        allTasks = [...allTasks, ...historyTasks];
    }
    
    // Filter
    let filtered = allTasks;
    if (statusFilter === 'active') {
        // onlyrunningin's task(notincludehistory)
        filtered = tasksState.allTasks.filter(task => 
            task.status === 'running' || task.status === 'cancelling'
        );
    } else if (statusFilter === 'history') {
        // onlyhistoryrecord
        filtered = allTasks.filter(task => task.isHistory);
    } else if (statusFilter !== 'all') {
        filtered = allTasks.filter(task => task.status === statusFilter);
    }
    
    // sort
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

// switchShow historyrecord
function toggleShowHistory(show) {
    tasksState.showHistory = show;
    localStorage.setItem('tasks-show-history', show ? 'true' : 'false');
    filterAndSortTasks();
}

// calculatelinewhenlong
function calculateDuration(startedAt) {
    if (!startedAt) return _t('tasks.unknown');
    const start = new Date(startedAt);
    const now = new Date();
    const diff = Math.floor((now - start) / 1000);
    
    if (diff < 60) {
        return diff + _t('tasks.durationSeconds');
    } else if (diff < 3600) {
        const minutes = Math.floor(diff / 60);
        const seconds = diff % 60;
        return minutes + _t('tasks.durationMinutes') + ' ' + seconds + _t('tasks.durationSeconds');
    } else {
        const hours = Math.floor(diff / 3600);
        const minutes = Math.floor((diff % 3600) / 60);
        return hours + _t('tasks.durationHours') + ' ' + minutes + _t('tasks.durationMinutes');
    }
}

// startwhenlongupdate
function startDurationUpdates() {
    // clearold's timer
    if (tasksState.durationUpdateInterval) {
        clearInterval(tasksState.durationUpdateInterval);
    }
    
 // eachsecondupdatetimelinewhenlong
    tasksState.durationUpdateInterval = setInterval(() => {
        updateTaskDurations();
    }, 1000);
}

// updatetasklinewhenlongShow 
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

// rendertasklist
function renderTasks(tasks) {
    const listContainer = document.getElementById('tasks-list');
    if (!listContainer) return;

    if (tasks.length === 0) {
        listContainer.innerHTML = `
            <div class="tasks-empty">
                <p>${_t('tasks.noMatchingTasks')}</p>
                ${tasksState.allTasks.length === 0 && tasksState.completedTasksHistory.length > 0 ? 
                    '<p style="margin-top: 8px; color: var(--text-muted); font-size: 0.875rem;">' + _t('tasks.historyHint') + '</p>' : ''}
            </div>
        `;
        return;
    }

    // statusmapping
    const statusMap = {
        'running': { text: _t('tasks.statusRunning'), class: 'task-status-running' },
        'cancelling': { text: _t('tasks.statusCancelling'), class: 'task-status-cancelling' },
        'failed': { text: _t('tasks.statusFailed'), class: 'task-status-failed' },
        'timeout': { text: _t('tasks.statusTimeout'), class: 'task-status-timeout' },
        'cancelled': { text: _t('tasks.statusCancelled'), class: 'task-status-cancelled' },
        'completed': { text: _t('tasks.statusCompleted'), class: 'task-status-completed' }
    };

    // separatecurrenttaskandhistorytask
    const activeTasks = tasks.filter(t => !t.isHistory);
    const historyTasks = tasks.filter(t => t.isHistory);

    let html = '';
    
    // rendercurrenttask
    if (activeTasks.length > 0) {
        html += activeTasks.map(task => renderTaskItem(task, statusMap)).join('');
    }
    
    // renderhistorytask
    if (historyTasks.length > 0) {
        html += `<div class="tasks-history-section">
            <div class="tasks-history-header">
                <span class="tasks-history-title">📜 ` + _t('tasks.recentCompletedTasks') + `</span>
                <button class="btn-secondary btn-small" onclick="clearTasksHistory()">` + _t('tasks.clearHistory') + `</button>
            </div>
            ${historyTasks.map(task => renderTaskItem(task, statusMap, true)).join('')}
        </div>`;
    }
    
    listContainer.innerHTML = html;
}

// rendertaskitem
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
        : _t('tasks.unknownTime');
    
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
                    ${isHistory ? '<span class="task-history-badge" title="' + _t('tasks.historyBadge') + '">📜</span>' : ''}
                    <span class="task-message" title="${escapeHtml(task.message || _t('tasks.unnamedTask'))}">${escapeHtml(task.message || _t('tasks.unnamedTask'))}</span>
                </div>
                <div class="task-actions">
                    ${duration ? `<span class="task-duration" title="${_t('tasks.duration')}">⏱ ${duration}</span>` : ''}
                    <span class="task-time" title="${isHistory && completedText ? _t('tasks.completedAt') : _t('tasks.startedAt')}">
                        ${isHistory && completedText ? completedText : timeText}
                    </span>
                    ${canCancel ? `<button class="btn-secondary btn-small" onclick="cancelTask('${task.conversationId}', this)">` + _t('tasks.cancelTask') + `</button>` : ''}
                    ${task.conversationId ? `<button class="btn-secondary btn-small" onclick="viewConversation('${task.conversationId}')">` + _t('tasks.viewConversation') + `</button>` : ''}
                </div>
            </div>
            ${task.conversationId ? `
                <div class="task-details">
                    <span class="task-id-label">` + _t('tasks.conversationIdLabel') + `:</span>
                    <span class="task-id-value" title="` + _t('tasks.clickToCopy') + `" onclick="copyTaskId('${task.conversationId}')">${escapeHtml(task.conversationId)}</span>
                </div>
            ` : ''}
        </div>
    `;
}

// cleartaskhistory
function clearTasksHistory() {
    if (!confirm(_t('tasks.clearHistoryConfirm'))) {
        return;
    }
    tasksState.completedTasksHistory = [];
    saveCompletedTasksHistory();
    filterAndSortTasks();
}

// switchtaskselect
function toggleTaskSelection(conversationId, selected) {
    if (selected) {
        tasksState.selectedTasks.add(conversationId);
    } else {
        tasksState.selectedTasks.delete(conversationId);
    }
    updateBatchActions();
}

// updatebatchActionsUI
function updateBatchActions() {
    const batchActions = document.getElementById('tasks-batch-actions');
    const selectedCount = document.getElementById('tasks-selected-count');
    
    if (!batchActions || !selectedCount) return;
    
    const count = tasksState.selectedTasks.size;
    if (count > 0) {
        batchActions.style.display = 'flex';
        selectedCount.textContent = typeof window.t === 'function' ? window.t('mcp.selectedCount', { count: count }) : `Selected ${count} item`;
    } else {
        batchActions.style.display = 'none';
    }
}

// cleartaskselect
function clearTaskSelection() {
    tasksState.selectedTasks.clear();
    updateBatchActions();
 // re-rendertoupdatestatus
    filterAndSortTasks();
}

// batchCanceltask
async function batchCancelTasks() {
    const selected = Array.from(tasksState.selectedTasks);
    if (selected.length === 0) return;
    
    if (!confirm(_t('tasks.confirmCancelTasks', { n: selected.length }))) {
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
            console.error('Failed to cancel task:', conversationId, error);
            failCount++;
        }
    }
    
    // clearselect
    clearTaskSelection();
    
    // Refreshtasklist
    await loadTasks();
    
    // Show result
    if (failCount > 0) {
        alert(_t('tasks.batchCancelResultPartial', { success: successCount, fail: failCount }));
    } else {
        alert(_t('tasks.batchCancelResultSuccess', { n: successCount }));
    }
}

// CopytaskID
function copyTaskId(conversationId) {
    navigator.clipboard.writeText(conversationId).then(() => {
        // Show CopySuccesshint
        const tooltip = document.createElement('div');
        tooltip.textContent = _t('tasks.copiedToast');
        tooltip.style.cssText = 'position: fixed; top: 50%; left: 50%; transform: translate(-50%, -50%); background: rgba(0,0,0,0.8); color: white; padding: 8px 16px; border-radius: 4px; z-index: 10000;';
        document.body.appendChild(tooltip);
        setTimeout(() => tooltip.remove(), 1000);
    }).catch(err => {
        console.error('Copy failed:', err);
    });
}

// Canceltask
async function cancelTask(conversationId, button) {
    if (!conversationId) return;
    
    const originalText = button.textContent;
    button.disabled = true;
    button.textContent = _t('tasks.cancelling');

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
            throw new Error(result.error || _t('tasks.cancelTaskFailed'));
        }

        // fromselectinremove
        tasksState.selectedTasks.delete(conversationId);
        updateBatchActions();
        
        // re-loadtasklist
        await loadTasks();
    } catch (error) {
        console.error('Failed to cancel task:', error);
        alert(_t('tasks.cancelTaskFailed') + ': ' + error.message);
        button.disabled = false;
        button.textContent = originalText;
    }
}

// viewconversation
function viewConversation(conversationId) {
    if (!conversationId) return;
    
    // switchtoconversationpage
    if (typeof switchPage === 'function') {
        switchPage('chat');
 // loadselectedthisconversation - useglobalfunction
        setTimeout(() => {
            // trymultiplemethodloadconversation
            if (typeof loadConversation === 'function') {
                loadConversation(conversationId);
            } else if (typeof window.loadConversation === 'function') {
                window.loadConversation(conversationId);
            } else {
                // iffunctionnotexist,tryviaURLnavigate
                window.location.hash = `chat?conversation=${conversationId}`;
                console.log('switchtoconversationpage,conversationID:', conversationId);
            }
        }, 500);
    }
}

// Refreshtasklist
async function refreshTasks() {
    await loadTasks();
}

// switchautoRefresh
function toggleTasksAutoRefresh(enabled) {
    tasksState.autoRefresh = enabled;
    
    // save tolocalStorage
    localStorage.setItem('tasks-auto-refresh', enabled ? 'true' : 'false');
    
    if (enabled) {
        // StartautoRefresh
        if (!tasksState.refreshInterval) {
            tasksState.refreshInterval = setInterval(() => {
                loadBatchQueues();
            }, 5000);
        }
    } else {
        // stopautoRefresh
        if (tasksState.refreshInterval) {
            clearInterval(tasksState.refreshInterval);
            tasksState.refreshInterval = null;
        }
    }
}

// initializetaskmanagement page
function initTasksPage() {
    // restoreautoRefreshsettings
    const autoRefreshCheckbox = document.getElementById('tasks-auto-refresh');
    if (autoRefreshCheckbox) {
        const saved = localStorage.getItem('tasks-auto-refresh');
        const enabled = saved !== null ? saved === 'true' : true;
        autoRefreshCheckbox.checked = enabled;
        toggleTasksAutoRefresh(enabled);
    } else {
        toggleTasksAutoRefresh(true);
    }
    
    // onlyloadbatchtaskqueue
    loadBatchQueues();
}

// cleanuptimer(pageswitchwhencall)
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

// exportfunctionforglobaluse
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

// ==================== batchtaskfunction ====================

// batchtaskstatus
const batchQueuesState = {
    queues: [],
    currentQueueId: null,
    refreshInterval: null,
    // Filterandpaginationstatus
    filterStatus: 'all', // 'all', 'pending', 'running', 'paused', 'completed', 'cancelled'
    searchKeyword: '',
    currentPage: 1,
    pageSize: 10,
    total: 0,
    totalPages: 1
};

// Show newtaskmodal
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
        // resetrole selectionasdefault
        if (roleSelect) {
            roleSelect.value = '';
        }
        updateBatchImportStats('');
        
 // loadpopulaterolelist
        if (roleSelect && typeof loadRoles === 'function') {
            try {
                const loadedRoles = await loadRoles();
 // clearhasoption(defaultoption)
                roleSelect.innerHTML = '<option value="">' + _t('batchImportModal.defaultRole') + '</option>';
                
                // addEnabled's role
                const sortedRoles = loadedRoles.sort((a, b) => {
                    if (a.name === 'default') return -1;
                    if (b.name === 'default') return 1;
                    return (a.name || '').localeCompare(b.name || '', 'zh-CN');
                });
                
                sortedRoles.forEach(role => {
                    if (role.name !== 'default' && role.enabled !== false) {
                        const option = document.createElement('option');
                        option.value = role.name;
                        option.textContent = role.name;
                        roleSelect.appendChild(option);
                    }
                });
            } catch (error) {
                console.error('Failed to load role list:', error);
            }
        }
        
        modal.style.display = 'block';
        input.focus();
    }
}

// Closenewtaskmodal
function closeBatchImportModal() {
    const modal = document.getElementById('batch-import-modal');
    if (modal) {
        modal.style.display = 'none';
    }
}

// updatenewtaskstatistics
function updateBatchImportStats(text) {
    const statsEl = document.getElementById('batch-import-stats');
    if (!statsEl) return;
    
    const lines = text.split('\n').filter(line => line.trim() !== '');
    const count = lines.length;
    
    if (count > 0) {
        statsEl.innerHTML = '<div class="batch-import-stat">' + _t('tasks.taskCount', { count: count }) + '</div>';
        statsEl.style.display = 'block';
    } else {
        statsEl.style.display = 'none';
    }
}

// listenbatchtaskinput
document.addEventListener('DOMContentLoaded', function() {
    const input = document.getElementById('batch-tasks-input');
    if (input) {
        input.addEventListener('input', function() {
            updateBatchImportStats(this.value);
        });
    }
});

// createbatchtaskqueue
async function createBatchQueue() {
    const input = document.getElementById('batch-tasks-input');
    const titleInput = document.getElementById('batch-queue-title');
    const roleSelect = document.getElementById('batch-queue-role');
    if (!input) return;
    
    const text = input.value.trim();
    if (!text) {
        alert(_t('tasks.enterTaskPrompt'));
        return;
    }
    
 // bylineminutetask
    const tasks = text.split('\n').map(line => line.trim()).filter(line => line !== '');
    if (tasks.length === 0) {
        alert(_t('tasks.noValidTask'));
        return;
    }
    
    // gettitle(optional)
    const title = titleInput ? titleInput.value.trim() : '';
    
    // getrole(optional,emptystringindicatesdefault role)
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
            throw new Error(result.error || _t('tasks.createBatchQueueFailed'));
        }
        
        const result = await response.json();
        closeBatchImportModal();
        
        // Show queuedetails
        showBatchQueueDetail(result.queueId);
        
        // Refreshbatchqueuelist
        refreshBatchQueues();
    } catch (error) {
        console.error('Failed to create batch task queue:', error);
        alert(_t('tasks.createBatchQueueFailed') + ': ' + error.message);
    }
}

// getroleicon(function)
function getRoleIconForDisplay(roleName, rolesList) {
    if (!roleName || roleName === '') {
        return '🔵'; // default roleicon
    }
    
    if (Array.isArray(rolesList) && rolesList.length > 0) {
        const role = rolesList.find(r => r.name === roleName);
        if (role && role.icon) {
            let icon = role.icon;
            // check if it is Unicode escape format(may contain quotes)
            const unicodeMatch = icon.match(/^"?\\U([0-9A-F]{8})"?$/i);
            if (unicodeMatch) {
                try {
                    const codePoint = parseInt(unicodeMatch[1], 16);
                    icon = String.fromCodePoint(codePoint);
                } catch (e) {
                    // Conversion failed,use default icon
                    console.warn('convert icon Unicode escape failed:', icon, e);
                    return '👤';
                }
            }
            return icon;
        }
    }
    return '👤'; // defaulticon
}

// loadbatchtaskqueuelist
async function loadBatchQueues(page) {
    const section = document.getElementById('batch-queues-section');
    if (!section) return;
    
    // ifspecifypage,use it;otherwise usecurrentpage
    if (page !== undefined) {
        batchQueuesState.currentPage = page;
    }
    
    // loadrolelist(used forShow correct's roleicon)
    let loadedRoles = [];
    if (typeof loadRoles === 'function') {
        try {
            loadedRoles = await loadRoles();
        } catch (error) {
            console.warn('Failed to load role list,will use default icons:', error);
        }
    }
    batchQueuesState.loadedRoles = loadedRoles; // save tostatusinforrenderuse
    
    // buildQueryparameter
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
            throw new Error(_t('tasks.loadFailedRetry'));
        }
        
        const result = await response.json();
        batchQueuesState.queues = result.queues || [];
        batchQueuesState.total = result.total || 0;
        batchQueuesState.totalPages = result.total_pages || 1;
        renderBatchQueues();
    } catch (error) {
        console.error('Failed to load batch task queue:', error);
        section.style.display = 'block';
        const list = document.getElementById('batch-queues-list');
        if (list) {
            list.innerHTML = '<div class="tasks-empty"><p>' + _t('tasks.loadFailedRetry') + ': ' + escapeHtml(error.message) + '</p><button class="btn-secondary" onclick="refreshBatchQueues()">' + _t('tasks.retry') + '</button></div>';
        }
    }
}

// Filterbatchtaskqueue
function filterBatchQueues() {
    const statusFilter = document.getElementById('batch-queues-status-filter');
    const searchInput = document.getElementById('batch-queues-search');
    
    if (statusFilter) {
        batchQueuesState.filterStatus = statusFilter.value;
    }
    if (searchInput) {
        batchQueuesState.searchKeyword = searchInput.value.trim();
    }
    
 // reset to first pagere-load
    batchQueuesState.currentPage = 1;
    loadBatchQueues(1);
}

// renderbatchtaskqueuelist
function renderBatchQueues() {
    const section = document.getElementById('batch-queues-section');
    const list = document.getElementById('batch-queues-list');
    const pagination = document.getElementById('batch-queues-pagination');
    
    if (!section || !list) return;
    
    section.style.display = 'block';
    
    const queues = batchQueuesState.queues;
    
    if (queues.length === 0) {
        list.innerHTML = '<div class="tasks-empty"><p>' + _t('tasks.noBatchQueues') + '</p></div>';
        if (pagination) pagination.style.display = 'none';
        return;
    }
    
 // ensurepaginationcontrolscan(resetbeforepossiblysettings's display: none)
    if (pagination) {
        pagination.style.display = '';
    }
    
    list.innerHTML = queues.map(queue => {
        const statusMap = {
            'pending': { text: _t('tasks.statusPending'), class: 'batch-queue-status-pending' },
            'running': { text: _t('tasks.statusRunning'), class: 'batch-queue-status-running' },
            'paused': { text: _t('tasks.statusPaused'), class: 'batch-queue-status-paused' },
            'completed': { text: _t('tasks.statusCompleted'), class: 'batch-queue-status-completed' },
            'cancelled': { text: _t('tasks.statusCancelled'), class: 'batch-queue-status-cancelled' }
        };
        
        const status = statusMap[queue.status] || { text: queue.status, class: 'batch-queue-status-unknown' };
        
        // statisticstaskstatus
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
 // allowDeleteline,completedoralreadyCancelstatus's queue
        const canDelete = queue.status === 'pending' || queue.status === 'completed' || queue.status === 'cancelled';
        
        const titleDisplay = queue.title ? `<span class="batch-queue-title" style="font-weight: 600; color: var(--text-primary); margin-right: 8px;">${escapeHtml(queue.title)}</span>` : '';
        
        // Show roleInfo(usecorrect's roleicon)
        const loadedRoles = batchQueuesState.loadedRoles || [];
        const roleIcon = getRoleIconForDisplay(queue.role, loadedRoles);
        const roleName = queue.role && queue.role !== '' ? queue.role : _t('batchQueueDetailModal.defaultRole');
        const roleDisplay = `<span class="batch-queue-role" style="margin-right: 8px;" title="${_t('batchQueueDetailModal.role')}: ${escapeHtml(roleName)}">${roleIcon} ${escapeHtml(roleName)}</span>`;
        
        return `
            <div class="batch-queue-item" data-queue-id="${queue.id}" onclick="showBatchQueueDetail('${queue.id}')">
                <div class="batch-queue-header">
                    <div class="batch-queue-info" style="flex: 1;">
                        ${titleDisplay}
                        ${roleDisplay}
                        <span class="batch-queue-status ${status.class}">${status.text}</span>
                        <span class="batch-queue-id">${_t('tasks.queueIdLabel')}: ${escapeHtml(queue.id)}</span>
                        <span class="batch-queue-time">${_t('tasks.createdTimeLabel')}: ${new Date(queue.createdAt).toLocaleString()}</span>
                    </div>
                    <div class="batch-queue-progress">
                        <div class="batch-queue-progress-bar">
                            <div class="batch-queue-progress-fill" style="width: ${progress}%"></div>
                        </div>
                        <span class="batch-queue-progress-text">${progress}% (${stats.completed + stats.failed + stats.cancelled}/${stats.total})</span>
                    </div>
                    <div class="batch-queue-actions" style="display: flex; align-items: center; gap: 8px; margin-left: 12px;" onclick="event.stopPropagation();">
                        ${canDelete ? `<button class="btn-secondary btn-small btn-danger" onclick="deleteBatchQueueFromList('${queue.id}')" title="${_t('tasks.deleteQueue')}">${_t('common.delete')}</button>` : ''}
                    </div>
                </div>
                <div class="batch-queue-stats">
                    <span>${_t('tasks.totalLabel')}: ${stats.total}</span>
                    <span>${_t('tasks.pendingLabel')}: ${stats.pending}</span>
                    <span>${_t('tasks.runningLabel')}: ${stats.running}</span>
                    <span style="color: var(--success-color);">${_t('tasks.completedLabel')}: ${stats.completed}</span>
                    <span style="color: var(--error-color);">${_t('tasks.failedLabel')}: ${stats.failed}</span>
                    ${stats.cancelled > 0 ? `<span style="color: var(--text-secondary);">${_t('tasks.cancelledLabel')}: ${stats.cancelled}</span>` : ''}
                </div>
            </div>
        `;
    }).join('');
    
    // render pagination controls
    renderBatchQueuesPagination();
}

// renderbatchtaskqueuepaginationcontrols(referenceSkillsmanagement pagestyle)
function renderBatchQueuesPagination() {
    const paginationContainer = document.getElementById('batch-queues-pagination');
    if (!paginationContainer) return;
    
    const { currentPage, pageSize, total, totalPages } = batchQueuesState;
    
 // even ifonlyhaspagealsoShow paginationInfo(referenceSkillsstyle)
    if (total === 0) {
        paginationContainer.innerHTML = '';
        return;
    }
    
    // calculateShow range
    const start = total === 0 ? 0 : (currentPage - 1) * pageSize + 1;
    const end = total === 0 ? 0 : Math.min(currentPage * pageSize, total);
    
    let paginationHTML = '<div class="pagination">';
    
    // left side:Show rangeInfoandeachpagecountselector(referenceSkillsstyle)
    paginationHTML += `
        <div class="pagination-info">
            <span>` + _t('tasks.paginationShow', { start: start, end: end, total: total }) + `</span>
            <label class="pagination-page-size">
                ` + _t('tasks.paginationPerPage') + `
                <select id="batch-queues-page-size-pagination" onchange="changeBatchQueuesPageSize()">
                    <option value="10" ${pageSize === 10 ? 'selected' : ''}>10</option>
                    <option value="20" ${pageSize === 20 ? 'selected' : ''}>20</option>
                    <option value="50" ${pageSize === 50 ? 'selected' : ''}>50</option>
                    <option value="100" ${pageSize === 100 ? 'selected' : ''}>100</option>
                </select>
            </label>
        </div>
    `;
    
    // right side:paginationbutton(referenceSkillsstyle:First,Previous,X/Ypage,Next,Last)
    paginationHTML += `
        <div class="pagination-controls">
            <button class="btn-secondary" onclick="goBatchQueuesPage(1)" ${currentPage === 1 || total === 0 ? 'disabled' : ''}>` + _t('tasks.paginationFirst') + `</button>
            <button class="btn-secondary" onclick="goBatchQueuesPage(${currentPage - 1})" ${currentPage === 1 || total === 0 ? 'disabled' : ''}>` + _t('tasks.paginationPrev') + `</button>
            <span class="pagination-page">` + _t('tasks.paginationPage', { current: currentPage, total: totalPages || 1 }) + `</span>
            <button class="btn-secondary" onclick="goBatchQueuesPage(${currentPage + 1})" ${currentPage >= totalPages || total === 0 ? 'disabled' : ''}>` + _t('tasks.paginationNext') + `</button>
            <button class="btn-secondary" onclick="goBatchQueuesPage(${totalPages || 1})" ${currentPage >= totalPages || total === 0 ? 'disabled' : ''}>` + _t('tasks.paginationLast') + `</button>
        </div>
    `;
    
    paginationHTML += '</div>';
    
    paginationContainer.innerHTML = paginationHTML;
    
    // ensurepaginationcomponentandlistcontentareaalign(excluding scrollbar)
    function alignPaginationWidth() {
        const batchQueuesList = document.getElementById('batch-queues-list');
        if (batchQueuesList && paginationContainer) {
            // getlist's actualcontentwidth(excluding scrollbar)
            const listClientWidth = batchQueuesList.clientWidth; // visibleareawidth(excluding scrollbar)
            const listScrollHeight = batchQueuesList.scrollHeight; // contenttotal height
            const listClientHeight = batchQueuesList.clientHeight; // visibleareaheight
            const hasScrollbar = listScrollHeight > listClientHeight;
            
            // iflisthasverticalscrollrecord,pagination should align with list content area(clientWidth)
            // if no scrollbar,use100%width
            if (hasScrollbar) {
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
    
    const batchQueuesList = document.getElementById('batch-queues-list');
    if (batchQueuesList) {
        resizeObserver.observe(batchQueuesList);
    }
}

// navigatetospecifypage
function goBatchQueuesPage(page) {
    const { totalPages } = batchQueuesState;
    if (page < 1 || page > totalPages) return;
    
    loadBatchQueues(page);
    
    // scrolltolisttop
    const list = document.getElementById('batch-queues-list');
    if (list) {
        list.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }
}

// change items per page
function changeBatchQueuesPageSize() {
    const pageSizeSelect = document.getElementById('batch-queues-page-size-pagination');
    if (!pageSizeSelect) return;
    
    const newPageSize = parseInt(pageSizeSelect.value, 10);
    if (newPageSize && newPageSize > 0) {
        batchQueuesState.pageSize = newPageSize;
        batchQueuesState.currentPage = 1; // reset to first page
        loadBatchQueues(1);
    }
}

// Show batchtaskqueuedetails
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
        // loadrolelist(ifstillnot loaded)
        let loadedRoles = [];
        if (typeof loadRoles === 'function') {
            try {
                loadedRoles = await loadRoles();
            } catch (error) {
                console.warn('Failed to load role list,will use default icons:', error);
            }
        }
        
        const response = await apiFetch(`/api/batch-tasks/${queueId}`);
        if (!response.ok) {
            throw new Error(_t('tasks.getQueueDetailFailed'));
        }
        
        const result = await response.json();
        const queue = result.queue;
        batchQueuesState.currentQueueId = queueId;
        
        if (title) {
 // textContent itselfwilldoescape;herenotneedthen escapeHtml,otherwisewill && Show &amp;...(looks like"deformed/garbled")
            title.textContent = queue.title ? _t('tasks.batchQueueTitle') + ' - ' + String(queue.title) : _t('tasks.batchQueueTitle');
        }
        
        // updatebuttonShow 
        const pauseBtn = document.getElementById('batch-queue-pause-btn');
        if (addTaskBtn) {
            addTaskBtn.style.display = queue.status === 'pending' ? 'inline-block' : 'none';
        }
        if (startBtn) {
 // pendingstatusShow "startline",pausedstatusShow "Continueline"
            startBtn.style.display = (queue.status === 'pending' || queue.status === 'paused') ? 'inline-block' : 'none';
            if (startBtn && queue.status === 'paused') {
                startBtn.textContent = _t('tasks.resumeExecute');
            } else if (startBtn && queue.status === 'pending') {
                startBtn.textContent = _t('batchQueueDetailModal.startExecute');
            }
        }
        if (pauseBtn) {
            // runningstatusShow "Pausequeue"
            pauseBtn.style.display = queue.status === 'running' ? 'inline-block' : 'none';
        }
        if (deleteBtn) {
 // allowDeleteline,completedoralreadyCancelstatus's queue
            deleteBtn.style.display = (queue.status === 'pending' || queue.status === 'completed' || queue.status === 'cancelled' || queue.status === 'paused') ? 'inline-block' : 'none';
        }
        
        // queuestatusmapping
        const queueStatusMap = {
            'pending': { text: _t('tasks.statusPending'), class: 'batch-queue-status-pending' },
            'running': { text: _t('tasks.statusRunning'), class: 'batch-queue-status-running' },
            'paused': { text: _t('tasks.statusPaused'), class: 'batch-queue-status-paused' },
            'completed': { text: _t('tasks.statusCompleted'), class: 'batch-queue-status-completed' },
            'cancelled': { text: _t('tasks.statusCancelled'), class: 'batch-queue-status-cancelled' }
        };
        
        // taskstatusmapping
        const taskStatusMap = {
            'pending': { text: _t('tasks.statusPending'), class: 'batch-task-status-pending' },
            'running': { text: _t('tasks.statusRunning'), class: 'batch-task-status-running' },
            'completed': { text: _t('tasks.statusCompleted'), class: 'batch-task-status-completed' },
            'failed': { text: _t('tasks.failedLabel'), class: 'batch-task-status-failed' },
            'cancelled': { text: _t('tasks.statusCancelled'), class: 'batch-task-status-cancelled' }
        };
        
        // getroleInfo(ifqueuehasrole configuration)
        let roleDisplay = '';
        if (queue.role && queue.role !== '') {
            // ifhasrole configuration,trygetroledetailedInfo
            let roleName = queue.role;
            let roleIcon = '👤';
            // fromalreadyload's rolelistinfindroleicon
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
                            // Conversion failed,use default icon
                        }
                    }
                    roleIcon = icon;
                }
            }
            roleDisplay = `<div class="detail-item">
                <span class="detail-label">` + _t('batchQueueDetailModal.role') + `</span>
                <span class="detail-value">${roleIcon} ${escapeHtml(roleName)}</span>
            </div>`;
        } else {
            // default role
            roleDisplay = `<div class="detail-item">
                <span class="detail-label">` + _t('batchQueueDetailModal.role') + `</span>
                <span class="detail-value">🔵 ` + _t('batchQueueDetailModal.defaultRole') + `</span>
            </div>`;
        }
        
        content.innerHTML = `
            <div class="batch-queue-detail-info">
                ${queue.title ? `<div class="detail-item">
                    <span class="detail-label">` + _t('batchQueueDetailModal.queueTitle') + `</span>
                    <span class="detail-value">${escapeHtml(queue.title)}</span>
                </div>` : ''}
                ${roleDisplay}
                <div class="detail-item">
                    <span class="detail-label">` + _t('batchQueueDetailModal.queueId') + `</span>
                    <span class="detail-value"><code>${escapeHtml(queue.id)}</code></span>
                </div>
                <div class="detail-item">
                    <span class="detail-label">` + _t('batchQueueDetailModal.status') + `</span>
                    <span class="detail-value"><span class="batch-queue-status ${queueStatusMap[queue.status]?.class || ''}">${queueStatusMap[queue.status]?.text || queue.status}</span></span>
                </div>
                <div class="detail-item">
                    <span class="detail-label">` + _t('batchQueueDetailModal.createdAt') + `</span>
                    <span class="detail-value">${new Date(queue.createdAt).toLocaleString()}</span>
                </div>
                ${queue.startedAt ? `<div class="detail-item">
                    <span class="detail-label">` + _t('batchQueueDetailModal.startedAt') + `</span>
                    <span class="detail-value">${new Date(queue.startedAt).toLocaleString()}</span>
                </div>` : ''}
                ${queue.completedAt ? `<div class="detail-item">
                    <span class="detail-label">` + _t('batchQueueDetailModal.completedAt') + `</span>
                    <span class="detail-value">${new Date(queue.completedAt).toLocaleString()}</span>
                </div>` : ''}
                <div class="detail-item">
                    <span class="detail-label">` + _t('batchQueueDetailModal.taskTotal') + `</span>
                    <span class="detail-value">${queue.tasks.length}</span>
                </div>
            </div>
            <div class="batch-queue-tasks-list">
                <h4>` + _t('batchQueueDetailModal.taskList') + `</h4>
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
                                ${canEdit ? `<button class="btn-secondary btn-small batch-task-edit-btn" onclick="editBatchTaskFromElement(this); event.stopPropagation();">` + _t('common.edit') + `</button>` : ''}
                                ${canEdit ? `<button class="btn-secondary btn-small btn-danger batch-task-delete-btn" onclick="deleteBatchTaskFromElement(this); event.stopPropagation();">` + _t('common.delete') + `</button>` : ''}
                                ${task.conversationId ? `<button class="btn-secondary btn-small" onclick="viewBatchTaskConversation('${task.conversationId}'); event.stopPropagation();">` + _t('tasks.viewConversation') + `</button>` : ''}
                            </div>
                            ${task.startedAt ? `<div class="batch-task-time">` + _t('batchQueueDetailModal.startLabel') + `: ${new Date(task.startedAt).toLocaleString()}</div>` : ''}
                            ${task.completedAt ? `<div class="batch-task-time">` + _t('batchQueueDetailModal.completeLabel') + `: ${new Date(task.completedAt).toLocaleString()}</div>` : ''}
                            ${task.error ? `<div class="batch-task-error">` + _t('batchQueueDetailModal.errorLabel') + `: ${escapeHtml(task.error)}</div>` : ''}
                            ${task.result ? `<div class="batch-task-result">` + _t('batchQueueDetailModal.resultLabel') + `: ${escapeHtml(task.result.substring(0, 200))}${task.result.length > 200 ? '...' : ''}</div>` : ''}
                        </div>
                    `;
                }).join('')}
            </div>
        `;
        
        modal.style.display = 'block';
        
        // ifqueuecurrently running,autoRefresh
        if (queue.status === 'running') {
            startBatchQueueRefresh(queueId);
        }
    } catch (error) {
        console.error('Failed to get queue details:', error);
        alert(_t('tasks.getQueueDetailFailed') + ': ' + error.message);
    }
}

// startbatchtaskqueue
async function startBatchQueue() {
    const queueId = batchQueuesState.currentQueueId;
    if (!queueId) return;
    
    try {
        const response = await apiFetch(`/api/batch-tasks/${queueId}/start`, {
            method: 'POST',
        });
        
        if (!response.ok) {
            const result = await response.json().catch(() => ({}));
            throw new Error(result.error || _t('tasks.startBatchQueueFailed'));
        }
        
        // Refreshdetails
        showBatchQueueDetail(queueId);
        refreshBatchQueues();
    } catch (error) {
        console.error('Failed to start batch task:', error);
        alert(_t('tasks.startBatchQueueFailed') + ': ' + error.message);
    }
}

// Pausebatchtaskqueue
async function pauseBatchQueue() {
    const queueId = batchQueuesState.currentQueueId;
    if (!queueId) return;
    
    if (!confirm(_t('tasks.pauseQueueConfirm'))) {
        return;
    }
    
    try {
        const response = await apiFetch(`/api/batch-tasks/${queueId}/pause`, {
            method: 'POST',
        });
        
        if (!response.ok) {
            const result = await response.json().catch(() => ({}));
            throw new Error(result.error || _t('tasks.pauseQueueFailed'));
        }
        
        // Refreshdetails
        showBatchQueueDetail(queueId);
        refreshBatchQueues();
    } catch (error) {
        console.error('Failed to pause batch task:', error);
        alert(_t('tasks.pauseQueueFailed') + ': ' + error.message);
    }
}

// Deletebatchtaskqueue(fromdetailsmodal)
async function deleteBatchQueue() {
    const queueId = batchQueuesState.currentQueueId;
    if (!queueId) return;
    
    if (!confirm(_t('tasks.deleteQueueConfirm'))) {
        return;
    }
    
    try {
        const response = await apiFetch(`/api/batch-tasks/${queueId}`, {
            method: 'DELETE',
        });
        
        if (!response.ok) {
            const result = await response.json().catch(() => ({}));
            throw new Error(result.error || _t('tasks.deleteQueueFailed'));
        }
        
        closeBatchQueueDetailModal();
        refreshBatchQueues();
    } catch (error) {
        console.error('Failed to delete batch task queue:', error);
        alert(_t('tasks.deleteQueueFailed') + ': ' + error.message);
    }
}

// fromlistDeletebatchtaskqueue
async function deleteBatchQueueFromList(queueId) {
    if (!queueId) return;
    
    if (!confirm(_t('tasks.deleteQueueConfirm'))) {
        return;
    }
    
    try {
        const response = await apiFetch(`/api/batch-tasks/${queueId}`, {
            method: 'DELETE',
        });
        
        if (!response.ok) {
            const result = await response.json().catch(() => ({}));
            throw new Error(result.error || _t('tasks.deleteQueueFailed'));
        }
        
        // ifcurrentcurrently viewthis queue's details,Closedetailsmodal
        if (batchQueuesState.currentQueueId === queueId) {
            closeBatchQueueDetailModal();
        }
        
        // refresh queue list
        refreshBatchQueues();
    } catch (error) {
        console.error('Failed to delete batch task queue:', error);
        alert(_t('tasks.deleteQueueFailed') + ': ' + error.message);
    }
}

// Closebatchtaskqueuedetailsmodal
function closeBatchQueueDetailModal() {
    const modal = document.getElementById('batch-queue-detail-modal');
    if (modal) {
        modal.style.display = 'none';
    }
    batchQueuesState.currentQueueId = null;
    stopBatchQueueRefresh();
}

// startbatchqueueRefresh
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
    }, 3000); // each3seconds refresh interval
}

// stopbatchqueueRefresh
function stopBatchQueueRefresh() {
    if (batchQueuesState.refreshInterval) {
        clearInterval(batchQueuesState.refreshInterval);
        batchQueuesState.refreshInterval = null;
    }
}

// Refreshbatchtaskqueuelist
async function refreshBatchQueues() {
    await loadBatchQueues(batchQueuesState.currentPage);
}

// viewbatchtask's conversation
function viewBatchTaskConversation(conversationId) {
    if (!conversationId) return;
    
    // Closebatchtaskdetailsmodal
    closeBatchQueueDetailModal();
    
    // use directlyURL hashnavigate,letrouterprocesspageswitchandconversationload
 // this reliable,becauserouterwillensurepageswitchcompleteafterthenloadconversation
    window.location.hash = `chat?conversation=${conversationId}`;
}

// Editbatchtask's status
const editBatchTaskState = {
    queueId: null,
    taskId: null
};

// fromelementgettaskInfoopenEditmodal
function editBatchTaskFromElement(button) {
    const taskItem = button.closest('.batch-task-item');
    if (!taskItem) {
 console.error('cannottotaskitemelement');
        return;
    }
    
    const queueId = taskItem.getAttribute('data-queue-id');
    const taskId = taskItem.getAttribute('data-task-id');
    const taskMessage = taskItem.getAttribute('data-task-message');
    
    if (!queueId || !taskId) {
        console.error('taskInfonotcomplete');
        return;
    }
    
    // DecodeHTMLentity
    const decodedMessage = taskMessage
        .replace(/&#39;/g, "'")
        .replace(/&quot;/g, '"')
        .replace(/\\n/g, '\n');
    
    editBatchTask(queueId, taskId, decodedMessage);
}

// openEditbatchtaskmodal
function editBatchTask(queueId, taskId, currentMessage) {
    editBatchTaskState.queueId = queueId;
    editBatchTaskState.taskId = taskId;
    
    const modal = document.getElementById('edit-batch-task-modal');
    const messageInput = document.getElementById('edit-task-message');
    
    if (!modal || !messageInput) {
        console.error('Edittaskmodalelementnotexist');
        return;
    }
    
    messageInput.value = currentMessage;
    modal.style.display = 'block';
    
 // toinput box
    setTimeout(() => {
        messageInput.focus();
        messageInput.select();
    }, 100);
    
 // addESClisten
    const handleKeyDown = (e) => {
        if (e.key === 'Escape') {
            closeEditBatchTaskModal();
            document.removeEventListener('keydown', handleKeyDown);
        }
    };
    document.addEventListener('keydown', handleKeyDown);
    
    // addEnter+Ctrl/CmdSavefunction
    const handleKeyPress = (e) => {
        if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
            e.preventDefault();
            saveBatchTask();
            document.removeEventListener('keydown', handleKeyPress);
        }
    };
    messageInput.addEventListener('keydown', handleKeyPress);
}

// CloseEditbatchtaskmodal
function closeEditBatchTaskModal() {
    const modal = document.getElementById('edit-batch-task-modal');
    if (modal) {
        modal.style.display = 'none';
    }
    editBatchTaskState.queueId = null;
    editBatchTaskState.taskId = null;
}

// Savebatchtask
async function saveBatchTask() {
    const queueId = editBatchTaskState.queueId;
    const taskId = editBatchTaskState.taskId;
    const messageInput = document.getElementById('edit-task-message');
    
    if (!queueId || !taskId) {
        alert(_t('tasks.taskIncomplete'));
        return;
    }
    
    if (!messageInput) {
        alert(_t('tasks.cannotGetTaskMessageInput'));
        return;
    }
    
    const message = messageInput.value.trim();
    if (!message) {
        alert(_t('tasks.taskMessageRequired'));
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
            throw new Error(result.error || _t('tasks.updateTaskFailed'));
        }
        
        // CloseEditmodal
        closeEditBatchTaskModal();
        
        // Refreshqueuedetails
        if (batchQueuesState.currentQueueId === queueId) {
            showBatchQueueDetail(queueId);
        }
        
        // refresh queue list
        refreshBatchQueues();
    } catch (error) {
        console.error('Failed to save task:', error);
        alert(_t('tasks.saveTaskFailed') + ': ' + error.message);
    }
}

// Show addbatchtaskmodal
function showAddBatchTaskModal() {
    const queueId = batchQueuesState.currentQueueId;
    if (!queueId) {
        alert(_t('tasks.queueInfoMissing'));
        return;
    }
    
    const modal = document.getElementById('add-batch-task-modal');
    const messageInput = document.getElementById('add-task-message');
    
    if (!modal || !messageInput) {
        console.error('addtaskmodalelementnotexist');
        return;
    }
    
    messageInput.value = '';
    modal.style.display = 'block';
    
 // toinput box
    setTimeout(() => {
        messageInput.focus();
    }, 100);
    
 // addESClisten
    const handleKeyDown = (e) => {
        if (e.key === 'Escape') {
            closeAddBatchTaskModal();
            document.removeEventListener('keydown', handleKeyDown);
        }
    };
    document.addEventListener('keydown', handleKeyDown);
    
    // addEnter+Ctrl/CmdSavefunction
    const handleKeyPress = (e) => {
        if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
            e.preventDefault();
            saveAddBatchTask();
            messageInput.removeEventListener('keydown', handleKeyPress);
        }
    };
    messageInput.addEventListener('keydown', handleKeyPress);
}

// Closeaddbatchtaskmodal
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

// Saveadd's batchtask
async function saveAddBatchTask() {
    const queueId = batchQueuesState.currentQueueId;
    const messageInput = document.getElementById('add-task-message');
    
    if (!queueId) {
        alert(_t('tasks.queueInfoMissing'));
        return;
    }
    
    if (!messageInput) {
        alert(_t('tasks.cannotGetTaskMessageInput'));
        return;
    }
    
    const message = messageInput.value.trim();
    if (!message) {
        alert(_t('tasks.taskMessageRequired'));
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
            throw new Error(result.error || _t('tasks.addTaskFailed'));
        }
        
        // Closeaddtaskmodal
        closeAddBatchTaskModal();
        
        // Refreshqueuedetails
        if (batchQueuesState.currentQueueId === queueId) {
            showBatchQueueDetail(queueId);
        }
        
        // refresh queue list
        refreshBatchQueues();
    } catch (error) {
        console.error('Failed to add task:', error);
        alert(_t('tasks.addTaskFailed') + ': ' + error.message);
    }
}

// fromelementgettaskInfoDeletetask
function deleteBatchTaskFromElement(button) {
    const taskItem = button.closest('.batch-task-item');
    if (!taskItem) {
 console.error('cannottotaskitemelement');
        return;
    }
    
    const queueId = taskItem.getAttribute('data-queue-id');
    const taskId = taskItem.getAttribute('data-task-id');
    const taskMessage = taskItem.getAttribute('data-task-message');
    
    if (!queueId || !taskId) {
        console.error('taskInfonotcomplete');
        return;
    }
    
    // DecodeHTMLentitytoShow message
    const decodedMessage = taskMessage
        .replace(/&#39;/g, "'")
        .replace(/&quot;/g, '"')
        .replace(/\\n/g, '\n');
    
 // truncatelongmessageused forconfirmconversation
    const displayMessage = decodedMessage.length > 50 
        ? decodedMessage.substring(0, 50) + '...' 
        : decodedMessage;
    
    if (!confirm(_t('tasks.confirmDeleteTask', { message: displayMessage }))) {
        return;
    }
    
    deleteBatchTask(queueId, taskId);
}

// Deletebatchtask
async function deleteBatchTask(queueId, taskId) {
    if (!queueId || !taskId) {
        alert(_t('tasks.taskIncomplete'));
        return;
    }
    
    try {
        const response = await apiFetch(`/api/batch-tasks/${queueId}/tasks/${taskId}`, {
            method: 'DELETE',
        });
        
        if (!response.ok) {
            const result = await response.json().catch(() => ({}));
            throw new Error(result.error || _t('tasks.deleteTaskFailed'));
        }
        
        // Refreshqueuedetails
        if (batchQueuesState.currentQueueId === queueId) {
            showBatchQueueDetail(queueId);
        }
        
        // refresh queue list
        refreshBatchQueues();
    } catch (error) {
        console.error('Failed to delete task:', error);
        alert(_t('tasks.deleteTaskFailed') + ': ' + error.message);
    }
}

// exportfunction
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
