// File Manager - view, upload, register, edit, and delete managed files

(function () {
    'use strict';

    let fmFiles = [];
    let fmSearch = '';
    let fmTypeFilter = '';
    let fmStatusFilter = '';
    let fmOffset = 0;
    let fmLimit = 50;
    let fmTotal = 0;
    let fmLoading = false;
    let fmSearchTimer = null;
    let fmEditingFileId = null;

    const FILE_TYPES = ['report', 'api_docs', 'project_file', 'target_file', 'reversing', 'exfiltrated', 'other'];
    const FILE_STATUSES = ['pending', 'processing', 'analyzed', 'in_progress', 'completed', 'archived'];

    const TYPE_LABELS = {
        report: 'Report', api_docs: 'API Docs', project_file: 'Project',
        target_file: 'Target', reversing: 'Reversing', exfiltrated: 'Exfiltrated', other: 'Other'
    };
    const STATUS_LABELS = {
        pending: 'Pending', processing: 'Processing', analyzed: 'Analyzed',
        in_progress: 'In Progress', completed: 'Completed', archived: 'Archived'
    };
    const TYPE_COLORS = {
        report: '#3b82f6', api_docs: '#8b5cf6', project_file: '#10b981',
        target_file: '#ef4444', reversing: '#f59e0b', exfiltrated: '#ec4899', other: '#6b7280'
    };
    const STATUS_COLORS = {
        pending: '#9ca3af', processing: '#3b82f6', analyzed: '#10b981',
        in_progress: '#f59e0b', completed: '#22c55e', archived: '#6b7280'
    };

    function initFileManagerPage() {
        loadFileStats();
        loadFileList(true);
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

    async function loadFileStats() {
        try {
            const data = await apiFetch('/api/files/stats');
            renderFileStats(data);
        } catch (e) {
            console.warn('File stats error:', e);
        }
    }

    function renderFileStats(data) {
        const el = document.getElementById('fm-stats');
        if (!el) return;

        const byType = data.by_type || {};
        const byStatus = data.by_status || {};
        const totalSize = data.total_size || 0;

        el.innerHTML = `
            <div class="memory-stat-item">
                <span class="memory-stat-value">${data.total || 0}</span>
                <span class="memory-stat-label">Total</span>
            </div>
            <div class="memory-stat-item">
                <span class="memory-stat-value">${formatSize(totalSize)}</span>
                <span class="memory-stat-label">Size</span>
            </div>
            ${FILE_TYPES.map(t => byType[t] ? `
            <div class="memory-stat-item" style="cursor:pointer;border-left:3px solid ${TYPE_COLORS[t]}" title="${TYPE_LABELS[t]}" onclick="onFmTypeFilter('${t}')">
                <span class="memory-stat-value">${byType[t]}</span>
                <span class="memory-stat-label">${TYPE_LABELS[t]}</span>
            </div>` : '').join('')}
        `;
    }

    async function loadFileList(reset) {
        if (fmLoading) return;
        fmLoading = true;

        if (reset) {
            fmOffset = 0;
            fmFiles = [];
        }

        try {
            const params = new URLSearchParams({ limit: fmLimit, offset: fmOffset });
            if (fmSearch) params.set('search', fmSearch);
            if (fmTypeFilter) params.set('file_type', fmTypeFilter);
            if (fmStatusFilter) params.set('status', fmStatusFilter);

            const data = await apiFetch('/api/files?' + params.toString());
            const files = data.files || [];
            fmTotal = data.total || 0;

            if (reset) {
                fmFiles = files;
            } else {
                fmFiles = fmFiles.concat(files);
            }

            renderFileList();
            renderFileCount();
        } catch (e) {
            console.warn('File list error:', e);
        } finally {
            fmLoading = false;
        }
    }

    function renderFileList() {
        const el = document.getElementById('fm-list');
        if (!el) return;

        if (fmFiles.length === 0) {
            el.innerHTML = '<div class="memory-empty" style="text-align:center;padding:40px;color:#6b7280;">No files found. Upload or register files to get started.</div>';
            return;
        }

        el.innerHTML = fmFiles.map(f => `
            <div class="memory-entry" style="cursor:pointer;border-left:3px solid ${TYPE_COLORS[f.file_type] || '#6b7280'}" onclick="openFmDetail('${f.id}')">
                <div class="memory-entry-header" style="display:flex;align-items:center;gap:8px;margin-bottom:4px;">
                    <strong style="flex:1;">${escHtml(f.file_name)}</strong>
                    <span class="memory-badge" style="background:${TYPE_COLORS[f.file_type] || '#6b7280'};color:#fff;padding:2px 8px;border-radius:4px;font-size:11px;">${TYPE_LABELS[f.file_type] || f.file_type}</span>
                    <span class="memory-badge" style="background:${STATUS_COLORS[f.status] || '#6b7280'};color:#fff;padding:2px 8px;border-radius:4px;font-size:11px;">${STATUS_LABELS[f.status] || f.status}</span>
                    <span style="color:#9ca3af;font-size:12px;">${formatSize(f.file_size)}</span>
                </div>
                ${f.summary ? `<div style="color:#d1d5db;font-size:13px;margin-bottom:4px;">${escHtml(f.summary).substring(0, 200)}${f.summary.length > 200 ? '...' : ''}</div>` : ''}
                ${f.tags ? `<div style="margin-bottom:4px;">${f.tags.split(',').map(t => `<span style="background:#374151;color:#9ca3af;padding:1px 6px;border-radius:3px;font-size:11px;margin-right:4px;">${escHtml(t.trim())}</span>`).join('')}</div>` : ''}
                <div style="display:flex;justify-content:space-between;align-items:center;color:#6b7280;font-size:11px;">
                    <span>${escHtml(f.file_path)}</span>
                    <span>${formatDate(f.updated_at)}</span>
                </div>
            </div>
        `).join('');
    }

    function renderFileCount() {
        const el = document.getElementById('fm-count');
        if (el) el.textContent = `${fmFiles.length} / ${fmTotal} files`;

        const footer = document.getElementById('fm-list-footer');
        if (footer && fmFiles.length < fmTotal) {
            footer.innerHTML = `<button class="btn-secondary" onclick="loadMoreFiles()" style="margin:10px auto;display:block;">Load More</button>`;
        } else if (footer) {
            footer.innerHTML = '';
        }
    }

    function loadMoreFiles() {
        fmOffset += fmLimit;
        loadFileList(false);
    }

    async function openFmDetail(id) {
        try {
            const data = await apiFetch('/api/files/' + id);
            const f = data.file;
            if (!f) return;

            fmEditingFileId = id;
            document.getElementById('fm-detail-title').textContent = f.file_name;

            const body = document.getElementById('fm-detail-body');
            body.innerHTML = `
                <div style="display:grid;grid-template-columns:1fr 1fr;gap:12px;margin-bottom:16px;">
                    <div class="form-group">
                        <label>File Type</label>
                        <select id="fm-edit-type">${FILE_TYPES.map(t => `<option value="${t}" ${t === f.file_type ? 'selected' : ''}>${TYPE_LABELS[t]}</option>`).join('')}</select>
                    </div>
                    <div class="form-group">
                        <label>Status</label>
                        <select id="fm-edit-status">${FILE_STATUSES.map(s => `<option value="${s}" ${s === f.status ? 'selected' : ''}>${STATUS_LABELS[s]}</option>`).join('')}</select>
                    </div>
                </div>
                <div class="form-group">
                    <label>Tags (comma-separated)</label>
                    <input type="text" id="fm-edit-tags" value="${escAttr(f.tags)}" placeholder="tag1, tag2, ..." />
                </div>
                <div class="form-group">
                    <label>Summary</label>
                    <textarea id="fm-edit-summary" rows="3" placeholder="What is this file, where it came from...">${escHtml(f.summary)}</textarea>
                </div>
                <div class="form-group">
                    <label>Handle Plan</label>
                    <textarea id="fm-edit-plan" rows="3" placeholder="How to handle this file...">${escHtml(f.handle_plan)}</textarea>
                </div>
                <div class="form-group">
                    <label>Progress</label>
                    <textarea id="fm-edit-progress" rows="3" placeholder="Current progress...">${escHtml(f.progress)}</textarea>
                </div>
                <div class="form-group">
                    <label>Findings</label>
                    <textarea id="fm-edit-findings" rows="5" placeholder="Discoveries and findings...">${escHtml(f.findings)}</textarea>
                </div>
                <div class="form-group">
                    <label>Logs</label>
                    <pre id="fm-edit-logs" style="background:#1a1a2e;padding:12px;border-radius:6px;max-height:200px;overflow-y:auto;font-size:12px;color:#9ca3af;white-space:pre-wrap;">${escHtml(f.logs) || '(no logs)'}</pre>
                </div>
                <div style="display:flex;gap:12px;margin-top:8px;">
                    <div style="flex:1;color:#6b7280;font-size:12px;">
                        <div>Path: ${escHtml(f.file_path)}</div>
                        <div>Size: ${formatSize(f.file_size)} | Created: ${formatDate(f.created_at)} | Updated: ${formatDate(f.updated_at)}</div>
                        <div>ID: ${f.id}</div>
                    </div>
                    <button class="btn-danger-outline" onclick="deleteFmFile('${f.id}')" style="align-self:flex-end;">Delete</button>
                </div>
            `;

            document.getElementById('fm-detail-modal').style.display = 'flex';
        } catch (e) {
            console.warn('File detail error:', e);
        }
    }

    function closeFmDetailModal() {
        document.getElementById('fm-detail-modal').style.display = 'none';
        fmEditingFileId = null;
    }

    async function saveFmDetailEdits() {
        if (!fmEditingFileId) return;

        const updates = {
            file_type: document.getElementById('fm-edit-type').value,
            status: document.getElementById('fm-edit-status').value,
            tags: document.getElementById('fm-edit-tags').value,
            summary: document.getElementById('fm-edit-summary').value,
            handle_plan: document.getElementById('fm-edit-plan').value,
            progress: document.getElementById('fm-edit-progress').value,
            findings: document.getElementById('fm-edit-findings').value,
        };

        try {
            await apiFetch('/api/files/' + fmEditingFileId, {
                method: 'PUT',
                body: JSON.stringify(updates),
            });
            closeFmDetailModal();
            loadFileList(true);
            loadFileStats();
        } catch (e) {
            alert('Error saving: ' + e.message);
        }
    }

    async function deleteFmFile(id) {
        if (!confirm('Delete this file entry? (File on disk will not be removed unless you check the option)')) return;
        try {
            await apiFetch('/api/files/' + id, { method: 'DELETE' });
            closeFmDetailModal();
            loadFileList(true);
            loadFileStats();
        } catch (e) {
            alert('Error deleting: ' + e.message);
        }
    }

    // Upload modal
    function openFileUploadModal() {
        document.getElementById('fm-upload-modal').style.display = 'flex';
        document.getElementById('fm-upload-file').value = '';
    }

    function closeFmUploadModal() {
        document.getElementById('fm-upload-modal').style.display = 'none';
    }

    let fmSelectedFile = null;
    function onFmFileSelected(input) {
        fmSelectedFile = input.files[0] || null;
    }

    async function doFmUpload() {
        if (!fmSelectedFile) { alert('Select a file first'); return; }

        const reader = new FileReader();
        reader.onload = async function () {
            const base64 = reader.result.split(',')[1] || '';
            const fileType = document.getElementById('fm-upload-type').value;

            try {
                await apiFetch('/api/files/upload', {
                    method: 'POST',
                    body: JSON.stringify({
                        file_name: fmSelectedFile.name,
                        content: base64,
                        is_base64: true,
                        mime_type: fmSelectedFile.type,
                        file_type: fileType,
                    }),
                });
                closeFmUploadModal();
                loadFileList(true);
                loadFileStats();
            } catch (e) {
                alert('Upload error: ' + e.message);
            }
        };
        reader.readAsDataURL(fmSelectedFile);
    }

    // Register modal
    function openFileRegisterModal() {
        document.getElementById('fm-register-modal').style.display = 'flex';
        document.getElementById('fm-reg-name').value = '';
        document.getElementById('fm-reg-path').value = '';
        document.getElementById('fm-reg-size').value = '';
    }

    function closeFmRegisterModal() {
        document.getElementById('fm-register-modal').style.display = 'none';
    }

    async function doFmRegister() {
        const name = document.getElementById('fm-reg-name').value.trim();
        const path = document.getElementById('fm-reg-path').value.trim();
        if (!name || !path) { alert('Name and path are required'); return; }

        const fileType = document.getElementById('fm-reg-type').value;
        const size = parseInt(document.getElementById('fm-reg-size').value) || 0;

        try {
            await apiFetch('/api/files/register', {
                method: 'POST',
                body: JSON.stringify({
                    file_name: name,
                    file_path: path,
                    file_size: size,
                    file_type: fileType,
                }),
            });
            closeFmRegisterModal();
            loadFileList(true);
            loadFileStats();
        } catch (e) {
            alert('Register error: ' + e.message);
        }
    }

    // Search and filter
    function onFmSearchInput(val) {
        clearTimeout(fmSearchTimer);
        fmSearchTimer = setTimeout(() => {
            fmSearch = val;
            loadFileList(true);
        }, 300);

        const clearBtn = document.getElementById('fm-search-clear');
        if (clearBtn) clearBtn.style.display = val ? 'flex' : 'none';
    }

    function clearFmSearch() {
        const input = document.getElementById('fm-search-input');
        if (input) input.value = '';
        fmSearch = '';
        const clearBtn = document.getElementById('fm-search-clear');
        if (clearBtn) clearBtn.style.display = 'none';
        loadFileList(true);
    }

    function onFmTypeFilter(val) {
        fmTypeFilter = val;
        const sel = document.getElementById('fm-type-filter');
        if (sel) sel.value = val;
        loadFileList(true);
    }

    function onFmStatusFilter(val) {
        fmStatusFilter = val;
        loadFileList(true);
    }

    // Utilities
    function formatSize(bytes) {
        if (!bytes || bytes === 0) return '0 B';
        const units = ['B', 'KB', 'MB', 'GB', 'TB'];
        const i = Math.floor(Math.log(bytes) / Math.log(1024));
        return (bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0) + ' ' + units[i];
    }

    function formatDate(dateStr) {
        if (!dateStr) return '';
        try {
            const d = new Date(dateStr);
            return d.toLocaleDateString() + ' ' + d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
        } catch (_) { return dateStr; }
    }

    function escHtml(s) {
        if (!s) return '';
        return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
    }

    function escAttr(s) {
        if (!s) return '';
        return s.replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/'/g, '&#39;');
    }

    // Exports
    window.initFileManagerPage = initFileManagerPage;
    window.loadFileList = loadFileList;
    window.loadFileStats = loadFileStats;
    window.openFileUploadModal = openFileUploadModal;
    window.closeFmUploadModal = closeFmUploadModal;
    window.openFileRegisterModal = openFileRegisterModal;
    window.closeFmRegisterModal = closeFmRegisterModal;
    window.doFmUpload = doFmUpload;
    window.doFmRegister = doFmRegister;
    window.onFmSearchInput = onFmSearchInput;
    window.clearFmSearch = clearFmSearch;
    window.onFmTypeFilter = onFmTypeFilter;
    window.onFmStatusFilter = onFmStatusFilter;
    window.onFmFileSelected = onFmFileSelected;
    window.openFmDetail = openFmDetail;
    window.closeFmDetailModal = closeFmDetailModal;
    window.saveFmDetailEdits = saveFmDetailEdits;
    window.deleteFmFile = deleteFmFile;
    window.loadMoreFiles = loadMoreFiles;
})();
