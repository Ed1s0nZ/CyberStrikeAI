// Memory panel — view, search, create, edit, and delete persistent memory entries

(function () {
    'use strict';

    // ── State ────────────────────────────────────────────────────────────────

    let allEntries = [];          // local cache of fetched entries
    let memorySearchQuery = '';   // current search string
    let memoryFilterCategory = ''; // current category filter
    let memoryEnabled = true;     // whether the feature is available

    const CATEGORIES = ['credential', 'target', 'vulnerability', 'fact', 'note'];

    // ── Init ─────────────────────────────────────────────────────────────────

    function initMemoryPage() {
        loadMemoryStats();
        loadMemoryEntries();
    }

    // ── API helpers ──────────────────────────────────────────────────────────

    async function apiFetch(url, options) {
        const res = await fetch(url, {
            headers: { 'Content-Type': 'application/json' },
            credentials: 'include',
            ...options,
        });
        if (!res.ok) {
            const body = await res.json().catch(() => ({}));
            throw new Error(body.error || `HTTP ${res.status}`);
        }
        return res.json();
    }

    // ── Stats ────────────────────────────────────────────────────────────────

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

    // ── List ─────────────────────────────────────────────────────────────────

    async function loadMemoryEntries() {
        const listEl = document.getElementById('memory-list');
        if (!listEl) return;

        listEl.innerHTML = '<div class="memory-loading">Loading...</div>';

        try {
            let url = '/api/memories?limit=500';
            if (memorySearchQuery) url += `&search=${encodeURIComponent(memorySearchQuery)}`;
            if (memoryFilterCategory) url += `&category=${encodeURIComponent(memoryFilterCategory)}`;

            const data = await apiFetch(url);
            memoryEnabled = data.enabled !== false;

            if (!memoryEnabled) {
                listEl.innerHTML = `<div class="memory-disabled-notice">${data.message || 'Persistent memory is not enabled.'}</div>`;
                return;
            }

            allEntries = data.entries || [];
            renderMemoryList(allEntries);
        } catch (e) {
            listEl.innerHTML = `<div class="memory-error">Failed to load entries: ${escHtml(e.message)}</div>`;
        }
    }

    function renderMemoryList(entries) {
        const listEl = document.getElementById('memory-list');
        if (!listEl) return;

        updateMemoryCount(entries.length);

        if (entries.length === 0) {
            listEl.innerHTML = '<div class="memory-empty">No memory entries found.</div>';
            return;
        }

        listEl.innerHTML = entries.map(e => renderEntryRow(e)).join('');
    }

    function renderEntryRow(e) {
        const catClass = `memory-cat-${escHtml(e.category || 'fact')}`;
        const updatedAt = e.updated_at ? formatDate(e.updated_at) : '';
        return `
        <div class="memory-entry" data-id="${escHtml(e.id)}">
            <div class="memory-entry-header">
                <span class="memory-entry-key" title="${escHtml(e.key)}">${escHtml(e.key)}</span>
                <span class="memory-entry-cat ${catClass}">${escHtml(e.category || 'fact')}</span>
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
            ${e.conversation_id ? `<div class="memory-entry-conv">Conv: ${escHtml(e.conversation_id)}</div>` : ''}
        </div>`;
    }

    function updateMemoryCount(count) {
        const el = document.getElementById('memory-count');
        if (el) el.textContent = `${count} ${count === 1 ? 'entry' : 'entries'}`;
    }

    // ── Search / Filter ──────────────────────────────────────────────────────

    function onMemorySearchInput(value) {
        memorySearchQuery = value.trim();
        const clearBtn = document.getElementById('memory-search-clear');
        if (clearBtn) clearBtn.style.display = memorySearchQuery ? 'flex' : 'none';
        loadMemoryEntries();
    }

    function clearMemorySearch() {
        memorySearchQuery = '';
        const input = document.getElementById('memory-search-input');
        if (input) input.value = '';
        const clearBtn = document.getElementById('memory-search-clear');
        if (clearBtn) clearBtn.style.display = 'none';
        loadMemoryEntries();
    }

    function filterMemoryByCategory(cat) {
        // Toggle filter: clicking the same category again clears it
        memoryFilterCategory = (memoryFilterCategory === cat) ? '' : cat;

        // Update active state on filter buttons
        document.querySelectorAll('.memory-filter-btn').forEach(btn => {
            btn.classList.toggle('active', btn.dataset.cat === memoryFilterCategory);
        });

        loadMemoryEntries();
        loadMemoryStats();
    }

    // ── Create / Edit modal ──────────────────────────────────────────────────

    function openCreateMemoryModal() {
        setModalMode('create');
        clearModalFields();
        showMemoryModal();
    }

    function openEditMemoryModal(id) {
        const entry = allEntries.find(e => e.id === id);
        if (!entry) {
            showMemoryNotification('Entry not found in local cache. Refreshing…', 'warning');
            loadMemoryEntries();
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
        if (saveBtn) { saveBtn.disabled = true; saveBtn.textContent = 'Saving…'; }

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
            await Promise.all([loadMemoryEntries(), loadMemoryStats()]);
        } catch (e) {
            showMemoryNotification(`Save failed: ${e.message}`, 'error');
        } finally {
            if (saveBtn) { saveBtn.disabled = false; saveBtn.textContent = 'Save'; }
        }
    }

    // ── Delete ───────────────────────────────────────────────────────────────

    async function deleteMemoryEntry(id) {
        if (!confirm('Delete this memory entry? This cannot be undone.')) return;
        try {
            await apiFetch(`/api/memories/${id}`, { method: 'DELETE' });
            showMemoryNotification('Entry deleted.', 'success');
            await Promise.all([loadMemoryEntries(), loadMemoryStats()]);
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
            await Promise.all([loadMemoryEntries(), loadMemoryStats()]);
        } catch (e) {
            showMemoryNotification(`Bulk delete failed: ${e.message}`, 'error');
        }
    }

    // ── Notifications ────────────────────────────────────────────────────────

    function showMemoryNotification(message, type) {
        const el = document.getElementById('memory-notification');
        if (!el) return;
        el.textContent = message;
        el.className = `memory-notification memory-notification-${type}`;
        el.style.display = 'block';
        setTimeout(() => { el.style.display = 'none'; }, 3500);
    }

    // ── Helpers ──────────────────────────────────────────────────────────────

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

    // ── Exports ──────────────────────────────────────────────────────────────

    window.initMemoryPage        = initMemoryPage;
    window.onMemorySearchInput   = onMemorySearchInput;
    window.clearMemorySearch     = clearMemorySearch;
    window.filterMemoryByCategory = filterMemoryByCategory;
    window.openCreateMemoryModal = openCreateMemoryModal;
    window.openEditMemoryModal   = openEditMemoryModal;
    window.closeMemoryModal      = closeMemoryModal;
    window.saveMemoryEntry       = saveMemoryEntry;
    window.deleteMemoryEntry     = deleteMemoryEntry;
    window.deleteAllMemories     = deleteAllMemories;
    window.loadMemoryEntries     = loadMemoryEntries;
})();
