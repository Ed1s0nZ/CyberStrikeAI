// Memory panel - view, search, create, edit, and delete persistent memory entries

(function () {
    'use strict';

    let allEntries = [];
    let memorySearchQuery = '';
    let memoryFilterCategory = '';
    let memoryFilterStatus = '';
    let memoryFilterConfidence = '';
    let memoryFilterEntity = '';
    let memoryIncludeDismissed = true;
    let memoryEnabled = true;

    let memoryOffset = 0;
    let memoryLimit = 100;
    let memoryHasMore = true;
    let memoryLoading = false;
    let memoryScrollBound = false;
    let memorySearchTimer = null;

    const CATEGORIES = ['credential', 'target', 'vulnerability', 'fact', 'note', 'tool_run', 'discovery', 'plan'];
    const STATUS_LABELS = {
        active: 'Active',
        confirmed: 'Confirmed',
        false_positive: 'False Positive',
        disproven: 'Disproven'
    };

    function initMemoryPage() {
        bindMemoryScroll();
        syncFilterUI();
        loadMemoryStats();
        loadMemoryEntries(true);
    }

    async function apiFetch(url, options = {}) {
        const reqOptions = { ...options, credentials: 'include' };
        const headers = new Headers(options.headers || {});
        if (!headers.has('Content-Type') && !(reqOptions.body instanceof FormData)) {
            headers.set('Content-Type', 'application/json');
        }
        reqOptions.headers = headers;

        let res;
        if (typeof window.apiFetch === 'function') {
            res = await window.apiFetch(url, reqOptions);
        } else {
            try {
                const raw = localStorage.getItem('cyberstrike-auth');
                if (raw) {
                    const parsed = JSON.parse(raw);
                    if (parsed && parsed.token && !headers.has('Authorization')) {
                        headers.set('Authorization', `Bearer ${parsed.token}`);
                    }
                }
            } catch (_) {}
            res = await fetch(url, reqOptions);
        }

        if (!res.ok) {
            const body = await res.json().catch(() => ({}));
            throw new Error(body.error || `HTTP ${res.status}`);
        }
        if (res.status === 204) return {};
        return res.json().catch(() => ({}));
    }

    async function loadMemoryStats() {
        try {
            const data = await apiFetch('/api/memories/stats');
            memoryEnabled = data.enabled !== false;
            renderMemoryStats(data);
        } catch (e) {
            console.warn('Memory stats error:', e);
        }
    }

    function renderMemoryStats(data) {
        const statsEl = document.getElementById('memory-stats');
        if (!statsEl) return;

        if (!data.enabled) {
            statsEl.innerHTML = `<div class="memory-disabled-notice">${data.message || 'Persistent memory is not enabled.'}</div>`;
            return;
        }

        const cats = data.categories || {};
        statsEl.innerHTML = `
            <div class="memory-stat-item">
                <span class="memory-stat-value">${data.total || 0}</span>
                <span class="memory-stat-label">Total</span>
            </div>
            ${CATEGORIES.map(cat => `
            <div class="memory-stat-item memory-stat-${cat}" title="${capitalize(cat)}" onclick="filterMemoryByCategory('${cat}')">
                <span class="memory-stat-value">${cats[cat] || 0}</span>
                <span class="memory-stat-label">${capitalize(cat)}</span>
            </div>`).join('')}
        `;
    }

    function buildMemoryQuery() {
        const params = new URLSearchParams();
        params.set('limit', String(memoryLimit));
        params.set('offset', String(memoryOffset));
        if (memorySearchQuery) params.set('search', memorySearchQuery);
        if (memoryFilterCategory) params.set('category', memoryFilterCategory);
        if (memoryFilterStatus) params.set('status', memoryFilterStatus);
        if (memoryFilterConfidence) params.set('confidence', memoryFilterConfidence);
        if (memoryFilterEntity) params.set('entity', memoryFilterEntity);
        if (memoryIncludeDismissed) params.set('include_dismissed', '1');
        return `/api/memories?${params.toString()}`;
    }

    async function loadMemoryEntries(reset) {
        const listEl = document.getElementById('memory-list');
        if (!listEl) return;
        if (memoryLoading) return;
        if (!reset && !memoryHasMore) return;

        memoryLoading = true;
        if (reset) {
            memoryOffset = 0;
            memoryHasMore = true;
            allEntries = [];
            listEl.innerHTML = '<div class="memory-loading">Loading...</div>';
            setMemoryListFooter('');
        } else {
            setMemoryListFooter('Loading more...');
        }

        try {
            const data = await apiFetch(buildMemoryQuery());
            memoryEnabled = data.enabled !== false;

            if (!memoryEnabled) {
                listEl.innerHTML = `<div class="memory-disabled-notice">${data.message || 'Persistent memory is not enabled.'}</div>`;
                setMemoryListFooter('');
                return;
            }

            const entries = data.entries || [];
            memoryHasMore = data.has_more === true;
            memoryOffset += entries.length;
            allEntries = reset ? entries.slice() : allEntries.concat(entries);

            renderMemoryList(entries, reset);
            updateMemoryCount(Number.isFinite(data.total) ? data.total : allEntries.length);

            if (allEntries.length === 0) {
                setMemoryListFooter('');
            } else if (memoryHasMore) {
                setMemoryListFooter('Scroll down to load more...');
            } else {
                setMemoryListFooter('All entries loaded.');
            }
        } catch (e) {
            if (reset) {
                listEl.innerHTML = `<div class="memory-error">Failed to load entries: ${escHtml(e.message)}</div>`;
            } else {
                setMemoryListFooter(`Load failed: ${escHtml(e.message)}`);
            }
        } finally {
            memoryLoading = false;
        }
    }

    function renderMemoryList(entries, reset) {
        const listEl = document.getElementById('memory-list');
        if (!listEl) return;

        if (reset) {
            if (entries.length === 0) {
                listEl.innerHTML = '<div class="memory-empty">No memory entries found.</div>';
                return;
            }
            listEl.innerHTML = entries.map(renderEntryRow).join('');
            return;
        }

        if (entries.length > 0) {
            listEl.insertAdjacentHTML('beforeend', entries.map(renderEntryRow).join(''));
        }
    }

    function renderEntryRow(e) {
        const catClass = `memory-cat-${escHtml(e.category || 'fact')}`;
        const status = String(e.status || 'active').toLowerCase();
        const statusLabel = STATUS_LABELS[status] || status;
        const updatedAt = e.updated_at ? formatDate(e.updated_at) : '';
        return `
        <div class="memory-entry" data-id="${escHtml(e.id)}">
            <div class="memory-entry-header">
                <span class="memory-entry-key" title="${escHtml(e.key)}">${escHtml(e.key)}</span>
                <span class="memory-entry-cat ${catClass}">${escHtml(e.category || 'fact')}</span>
                <span class="memory-entry-status memory-status-${escHtml(status)}">${escHtml(statusLabel)}</span>
                <span class="memory-entry-time">${updatedAt}</span>
                <div class="memory-entry-actions">
                    <button class="btn-icon" title="Edit" onclick="openEditMemoryModal('${escHtml(e.id)}')">
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                            <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 1 2-2v-7"></path>
                            <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"></path>
                        </svg>
                    </button>
                    <button class="btn-icon btn-icon-danger" title="Delete" onclick="deleteMemoryEntry('${escHtml(e.id)}')">
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                            <polyline points="3 6 5 6 21 6"></polyline>
                            <path d="M19 6l-1 14H6L5 6"></path>
                            <path d="M10 11v6"></path>
                            <path d="M14 11v6"></path>
                            <path d="M9 6V4h6v2"></path>
                        </svg>
                    </button>
                </div>
            </div>
            <div class="memory-entry-value">${escHtml(e.value)}</div>
            <div class="memory-entry-meta">
                ${e.entity ? `<span class="memory-entry-entity">Entity: ${escHtml(e.entity)}</span>` : ''}
                ${e.confidence ? `<span class="memory-entry-confidence">Confidence: ${escHtml(e.confidence)}</span>` : ''}
            </div>
            ${e.conversation_id ? `<div class="memory-entry-conv">Conv: ${escHtml(e.conversation_id)}</div>` : ''}
        </div>`;
    }

    function updateMemoryCount(count) {
        const el = document.getElementById('memory-count');
        if (el) el.textContent = `${count} ${count === 1 ? 'entry' : 'entries'}`;
    }

    function setMemoryListFooter(text) {
        const footer = document.getElementById('memory-list-footer');
        if (footer) footer.textContent = text || '';
    }

    function bindMemoryScroll() {
        if (memoryScrollBound) return;
        const listEl = document.getElementById('memory-list');
        if (!listEl) return;
        listEl.addEventListener('scroll', () => {
            if (memoryLoading || !memoryHasMore) return;
            const remaining = listEl.scrollHeight - listEl.scrollTop - listEl.clientHeight;
            if (remaining < 120) {
                loadMemoryEntries(false);
            }
        });
        memoryScrollBound = true;
    }

    function onMemorySearchInput(value) {
        memorySearchQuery = value.trim();
        const clearBtn = document.getElementById('memory-search-clear');
        if (clearBtn) clearBtn.style.display = memorySearchQuery ? 'flex' : 'none';
        if (memorySearchTimer) clearTimeout(memorySearchTimer);
        memorySearchTimer = setTimeout(() => {
            loadMemoryEntries(true);
        }, 280);
    }

    function clearMemorySearch() {
        memorySearchQuery = '';
        const input = document.getElementById('memory-search-input');
        if (input) input.value = '';
        const clearBtn = document.getElementById('memory-search-clear');
        if (clearBtn) clearBtn.style.display = 'none';
        loadMemoryEntries(true);
    }

    function onMemoryEntityInput(value) {
        memoryFilterEntity = value.trim();
        if (memorySearchTimer) clearTimeout(memorySearchTimer);
        memorySearchTimer = setTimeout(() => {
            loadMemoryEntries(true);
        }, 220);
    }

    function onMemoryStatusFilterChange(value) {
        memoryFilterStatus = (value || '').trim();
        loadMemoryEntries(true);
    }

    function onMemoryConfidenceFilterChange(value) {
        memoryFilterConfidence = (value || '').trim();
        loadMemoryEntries(true);
    }

    function toggleMemoryIncludeDismissed(checked) {
        memoryIncludeDismissed = !!checked;
        loadMemoryEntries(true);
    }

    function filterMemoryByCategory(cat) {
        memoryFilterCategory = (memoryFilterCategory === cat) ? '' : cat;
        document.querySelectorAll('.memory-filter-btn').forEach(btn => {
            btn.classList.toggle('active', btn.dataset.cat === memoryFilterCategory);
        });
        loadMemoryEntries(true);
        loadMemoryStats();
    }

    function syncFilterUI() {
        const statusEl = document.getElementById('memory-status-filter');
        if (statusEl) statusEl.value = memoryFilterStatus;
        const confEl = document.getElementById('memory-confidence-filter');
        if (confEl) confEl.value = memoryFilterConfidence;
        const entityEl = document.getElementById('memory-entity-filter');
        if (entityEl) entityEl.value = memoryFilterEntity;
        const dismissedEl = document.getElementById('memory-include-dismissed');
        if (dismissedEl) dismissedEl.checked = memoryIncludeDismissed;
    }

    function openCreateMemoryModal() {
        setModalMode('create');
        clearModalFields();
        showMemoryModal();
    }

    function openEditMemoryModal(id) {
        const entry = allEntries.find(e => e.id === id);
        if (!entry) {
            showMemoryNotification('Entry not found in local cache. Refreshing...', 'warning');
            loadMemoryEntries(true);
            return;
        }
        setModalMode('edit', id);
        populateModalFields(entry);
        showMemoryModal();
    }

    function setModalMode(mode, id) {
        const modal = document.getElementById('memory-modal');
        if (modal) {
            modal.dataset.mode = mode;
            modal.dataset.editId = id || '';
        }
        const title = document.getElementById('memory-modal-title');
        if (title) title.textContent = mode === 'create' ? 'Add Memory Entry' : 'Edit Memory Entry';
    }

    function clearModalFields() {
        setValue('memory-modal-key', '');
        setValue('memory-modal-value', '');
        setValue('memory-modal-category', 'fact');
        setValue('memory-modal-conv-id', '');
    }

    function populateModalFields(entry) {
        setValue('memory-modal-key', entry.key || '');
        setValue('memory-modal-value', entry.value || '');
        setValue('memory-modal-category', entry.category || 'fact');
        setValue('memory-modal-conv-id', entry.conversation_id || '');
    }

    function showMemoryModal() {
        const modal = document.getElementById('memory-modal');
        if (modal) modal.style.display = 'flex';
        setTimeout(() => {
            const keyInput = document.getElementById('memory-modal-key');
            if (keyInput) keyInput.focus();
        }, 50);
    }

    function closeMemoryModal() {
        const modal = document.getElementById('memory-modal');
        if (modal) modal.style.display = 'none';
    }

    async function saveMemoryEntry() {
        const modal = document.getElementById('memory-modal');
        const mode = modal ? modal.dataset.mode : 'create';
        const editId = modal ? modal.dataset.editId : '';

        const key = (document.getElementById('memory-modal-key') || {}).value || '';
        const value = (document.getElementById('memory-modal-value') || {}).value || '';
        const category = (document.getElementById('memory-modal-category') || {}).value || 'fact';
        const convId = (document.getElementById('memory-modal-conv-id') || {}).value || '';

        if (!key.trim()) {
            showMemoryNotification('Key is required.', 'error');
            return;
        }
        if (!value.trim()) {
            showMemoryNotification('Value is required.', 'error');
            return;
        }

        const saveBtn = document.getElementById('memory-modal-save');
        if (saveBtn) { saveBtn.disabled = true; saveBtn.textContent = 'Saving...'; }

        try {
            if (mode === 'create') {
                await apiFetch('/api/memories', {
                    method: 'POST',
                    body: JSON.stringify({ key: key.trim(), value: value.trim(), category, conversation_id: convId.trim() }),
                });
                showMemoryNotification('Memory entry created.', 'success');
            } else {
                await apiFetch(`/api/memories/${editId}`, {
                    method: 'PUT',
                    body: JSON.stringify({ key: key.trim(), value: value.trim(), category }),
                });
                showMemoryNotification('Memory entry updated.', 'success');
            }
            closeMemoryModal();
            await Promise.all([loadMemoryEntries(true), loadMemoryStats()]);
        } catch (e) {
            showMemoryNotification(`Save failed: ${e.message}`, 'error');
        } finally {
            if (saveBtn) { saveBtn.disabled = false; saveBtn.textContent = 'Save'; }
        }
    }

    async function deleteMemoryEntry(id) {
        if (!confirm('Delete this memory entry? This cannot be undone.')) return;
        try {
            await apiFetch(`/api/memories/${id}`, { method: 'DELETE' });
            showMemoryNotification('Entry deleted.', 'success');
            await Promise.all([loadMemoryEntries(true), loadMemoryStats()]);
        } catch (e) {
            showMemoryNotification(`Delete failed: ${e.message}`, 'error');
        }
    }

    async function deleteAllMemories() {
        const catLabel = memoryFilterCategory ? ` (category: ${memoryFilterCategory})` : '';
        const count = allEntries.length;
        if (!confirm(`Delete all ${count} memory entries${catLabel}? This cannot be undone.`)) return;

        const url = memoryFilterCategory
            ? `/api/memories?category=${encodeURIComponent(memoryFilterCategory)}`
            : '/api/memories';

        try {
            const data = await apiFetch(url, { method: 'DELETE' });
            showMemoryNotification(`Deleted ${data.deleted} entries.`, 'success');
            await Promise.all([loadMemoryEntries(true), loadMemoryStats()]);
        } catch (e) {
            showMemoryNotification(`Bulk delete failed: ${e.message}`, 'error');
        }
    }

    function showMemoryNotification(message, type) {
        const el = document.getElementById('memory-notification');
        if (!el) return;
        el.textContent = message;
        el.className = `memory-notification memory-notification-${type}`;
        el.style.display = 'block';
        setTimeout(() => { el.style.display = 'none'; }, 3500);
    }

    function escHtml(str) {
        if (!str) return '';
        return String(str)
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;')
            .replace(/'/g, '&#39;');
    }

    function capitalize(s) {
        return s ? s.charAt(0).toUpperCase() + s.slice(1) : '';
    }

    function setValue(id, val) {
        const el = document.getElementById(id);
        if (el) el.value = val;
    }

    function formatDate(iso) {
        if (!iso) return '';
        try {
            const d = new Date(iso);
            return d.toLocaleString();
        } catch (_) {
            return iso;
        }
    }

    window.initMemoryPage = initMemoryPage;
    window.onMemorySearchInput = onMemorySearchInput;
    window.clearMemorySearch = clearMemorySearch;
    window.onMemoryEntityInput = onMemoryEntityInput;
    window.onMemoryStatusFilterChange = onMemoryStatusFilterChange;
    window.onMemoryConfidenceFilterChange = onMemoryConfidenceFilterChange;
    window.toggleMemoryIncludeDismissed = toggleMemoryIncludeDismissed;
    window.filterMemoryByCategory = filterMemoryByCategory;
    window.openCreateMemoryModal = openCreateMemoryModal;
    window.openEditMemoryModal = openEditMemoryModal;
    window.closeMemoryModal = closeMemoryModal;
    window.saveMemoryEntry = saveMemoryEntry;
    window.deleteMemoryEntry = deleteMemoryEntry;
    window.deleteAllMemories = deleteAllMemories;
    window.loadMemoryEntries = function () { return loadMemoryEntries(true); };
})();

