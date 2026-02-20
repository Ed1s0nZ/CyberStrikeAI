// 信息收集页面（FOFA）

const FOFA_FORM_STORAGE_KEY = 'info-collect-fofa-form';
const FOFA_HIDDEN_FIELDS_STORAGE_KEY = 'info-collect-fofa-hidden-fields';

const infoCollectState = {
    currentPayload: null, // { fields, results, query, total, page, size }
    hiddenFields: new Set(),
    selectedRowIndexes: new Set(),
    tableBound: false
};

// HTML转义（如果未定义）
if (typeof escapeHtml === 'undefined') {
    function escapeHtml(text) {
        if (text == null) return '';
        const div = document.createElement('div');
        div.textContent = String(text);
        return div.innerHTML;
    }
}

function getFofaFormElements() {
    return {
        query: document.getElementById('fofa-query'),
        size: document.getElementById('fofa-size'),
        page: document.getElementById('fofa-page'),
        fields: document.getElementById('fofa-fields'),
        full: document.getElementById('fofa-full'),
        meta: document.getElementById('fofa-results-meta'),
        selectedMeta: document.getElementById('fofa-selected-meta'),
        thead: document.getElementById('fofa-results-thead'),
        tbody: document.getElementById('fofa-results-tbody'),
        columnsPanel: document.getElementById('fofa-columns-panel'),
        columnsList: document.getElementById('fofa-columns-list')
    };
}

function loadHiddenFieldsFromStorage() {
    try {
        const raw = localStorage.getItem(FOFA_HIDDEN_FIELDS_STORAGE_KEY);
        if (!raw) return [];
        const arr = JSON.parse(raw);
        if (!Array.isArray(arr)) return [];
        return arr.filter(x => typeof x === 'string');
    } catch (e) {
        return [];
    }
}

function saveHiddenFieldsToStorage() {
    try {
        localStorage.setItem(FOFA_HIDDEN_FIELDS_STORAGE_KEY, JSON.stringify(Array.from(infoCollectState.hiddenFields)));
    } catch (e) {
        // ignore
    }
}

function loadFofaFormFromStorage() {
    try {
        const raw = localStorage.getItem(FOFA_FORM_STORAGE_KEY);
        if (!raw) return null;
        const data = JSON.parse(raw);
        if (!data || typeof data !== 'object') return null;
        return data;
    } catch (e) {
        return null;
    }
}

function saveFofaFormToStorage(payload) {
    try {
        localStorage.setItem(FOFA_FORM_STORAGE_KEY, JSON.stringify(payload));
    } catch (e) {
        // ignore
    }
}

function initInfoCollectPage() {
    const els = getFofaFormElements();
    if (!els.query || !els.size || !els.fields || !els.tbody) return;

    // 恢复隐藏字段
    infoCollectState.hiddenFields = new Set(loadHiddenFieldsFromStorage());

    // 恢复上次输入
    const saved = loadFofaFormFromStorage();
    if (saved) {
        if (typeof saved.query === 'string') els.query.value = saved.query;
        if (typeof saved.size === 'number' || typeof saved.size === 'string') els.size.value = saved.size;
        if (typeof saved.page === 'number' || typeof saved.page === 'string') els.page.value = saved.page;
        if (typeof saved.fields === 'string') els.fields.value = saved.fields;
        if (typeof saved.full === 'boolean') els.full.checked = saved.full;
    }

    // 绑定 Enter 快捷查询（在 query 里用 Ctrl/Cmd+Enter）
    els.query.addEventListener('keydown', (e) => {
        if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
            e.preventDefault();
            submitFofaSearch();
        }
    });

    // 单行输入：按内容自动增高（避免默认留空白行）
    const autoGrow = () => {
        try {
            els.query.style.height = '40px';
            const max = 110;
            const h = Math.min(max, els.query.scrollHeight);
            els.query.style.height = `${h}px`;
        } catch (e) {
            // ignore
        }
    };
    els.query.addEventListener('input', autoGrow);
    // 初始化时也执行一次
    setTimeout(autoGrow, 0);

    // 绑定表格事件（事件委托，只绑定一次）
    bindFofaTableEvents();
    updateSelectedMeta();
}

function applyFofaQueryPreset(preset) {
    const els = getFofaFormElements();
    if (!els.query) return;
    els.query.value = (preset || '').trim();
    els.query.focus();
    saveFofaFormToStorage({
        query: els.query.value,
        size: parseInt(els.size?.value, 10) || 100,
        page: parseInt(els.page?.value, 10) || 1,
        fields: els.fields?.value || '',
        full: !!els.full?.checked
    });
}

function applyFofaFieldsPreset(preset) {
    const els = getFofaFormElements();
    if (!els.fields) return;
    els.fields.value = (preset || '').trim();
    els.fields.focus();
    saveFofaFormToStorage({
        query: (els.query?.value || '').trim(),
        size: parseInt(els.size?.value, 10) || 100,
        page: parseInt(els.page?.value, 10) || 1,
        fields: els.fields.value,
        full: !!els.full?.checked
    });
}

function resetFofaForm() {
    const els = getFofaFormElements();
    if (!els.query) return;
    els.query.value = '';
    if (els.size) els.size.value = 100;
    if (els.page) els.page.value = 1;
    if (els.fields) els.fields.value = 'host,ip,port,domain,title,protocol,country,province,city,server';
    if (els.full) els.full.checked = false;
    saveFofaFormToStorage({
        query: els.query.value,
        size: parseInt(els.size?.value, 10) || 100,
        page: parseInt(els.page?.value, 10) || 1,
        fields: els.fields?.value || '',
        full: !!els.full?.checked
    });
    renderFofaResults({ query: '', fields: [], results: [], total: 0, page: 1, size: 0 });
}

async function submitFofaSearch() {
    const els = getFofaFormElements();
    const query = (els.query?.value || '').trim();
    const size = parseInt(els.size?.value, 10) || 100;
    const page = parseInt(els.page?.value, 10) || 1;
    const fields = (els.fields?.value || '').trim();
    const full = !!els.full?.checked;

    if (!query) {
        alert('请输入 FOFA 查询语法');
        return;
    }

    saveFofaFormToStorage({ query, size, page, fields, full });
    setFofaMeta('查询中...');
    setFofaLoading(true);

    try {
        const response = await apiFetch('/api/fofa/search', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ query, size, page, fields, full })
        });

        const result = await response.json().catch(() => ({}));
        if (!response.ok) {
            throw new Error(result.error || `请求失败: ${response.status}`);
        }
        renderFofaResults(result);
    } catch (e) {
        console.error('FOFA 查询失败:', e);
        setFofaMeta('查询失败');
        renderFofaResults({ query, fields: [], results: [], total: 0, page: 1, size: 0 });
        alert('FOFA 查询失败: ' + (e && e.message ? e.message : String(e)));
    } finally {
        setFofaLoading(false);
    }
}

function setFofaMeta(text) {
    const els = getFofaFormElements();
    if (els.meta) {
        els.meta.textContent = text || '-';
    }
}

function updateSelectedMeta() {
    const els = getFofaFormElements();
    if (els.selectedMeta) {
        els.selectedMeta.textContent = `已选择 ${infoCollectState.selectedRowIndexes.size} 条`;
    }
}

function setFofaLoading(loading) {
    const els = getFofaFormElements();
    if (!els.tbody) return;
    if (loading) {
        const fieldsCount = (document.getElementById('fofa-fields')?.value || '').split(',').filter(Boolean).length;
        const colspan = Math.max(1, fieldsCount + 1);
        els.tbody.innerHTML = `<tr><td class="muted" style="padding: 16px;" colspan="${colspan}">加载中...</td></tr>`;
    }
}

function renderFofaResults(payload) {
    const els = getFofaFormElements();
    if (!els.thead || !els.tbody) return;

    const fields = Array.isArray(payload.fields) ? payload.fields : [];
    const results = Array.isArray(payload.results) ? payload.results : [];

    // 保存当前 payload 到 state
    infoCollectState.currentPayload = {
        query: payload.query || '',
        total: typeof payload.total === 'number' ? payload.total : 0,
        page: typeof payload.page === 'number' ? payload.page : 1,
        size: typeof payload.size === 'number' ? payload.size : 0,
        fields,
        results
    };

    // 清理选择（避免字段/结果变化导致错位）
    infoCollectState.selectedRowIndexes.clear();
    updateSelectedMeta();

    // 修剪隐藏字段：只保留当前 fields 中存在的
    const allowed = new Set(fields);
    infoCollectState.hiddenFields.forEach(f => {
        if (!allowed.has(f)) infoCollectState.hiddenFields.delete(f);
    });
    saveHiddenFieldsToStorage();

    const total = typeof payload.total === 'number' ? payload.total : 0;
    const size = typeof payload.size === 'number' ? payload.size : 0;
    const page = typeof payload.page === 'number' ? payload.page : 1;

    setFofaMeta(`共 ${total} 条 · 本页 ${results.length} 条 · page=${page} · size=${size}`);

    // 可见字段
    const visibleFields = fields.filter(f => !infoCollectState.hiddenFields.has(f));

    // 列面板
    renderFofaColumnsPanel(fields, visibleFields);

    // 表头（左：勾选列；右：操作列固定）
    const headerCells = [
        '<th class="info-collect-col-select"><input type="checkbox" id="fofa-select-all" title="全选/全不选"/></th>',
        ...visibleFields.map(f => `<th>${escapeHtml(String(f))}</th>`),
        '<th class="info-collect-col-actions">操作</th>'
    ].join('');
    els.thead.innerHTML = `<tr>${headerCells}</tr>`;

    // 表体
    if (results.length === 0) {
        const colspan = Math.max(1, visibleFields.length + 2);
        els.tbody.innerHTML = `<tr><td class="muted" style="padding: 16px;" colspan="${colspan}">暂无数据</td></tr>`;
        return;
    }

    const rowsHtml = results.map((row, idx) => {
        const safeRow = row && typeof row === 'object' ? row : {};
        const target = inferTargetFromRow(safeRow, fields);
        const encoded = encodeURIComponent(JSON.stringify(safeRow));
        const encodedTarget = encodeURIComponent(target || '');

        const selectHtml = `<td class="info-collect-col-select"><input class="fofa-row-select" type="checkbox" data-index="${idx}" title="选择该行"/></td>`;

        const cellsHtml = visibleFields.map(f => {
            const val = safeRow[f];
            const text = val == null ? '' : String(val);
            // host 字段：尽量渲染为可点击链接
            if (f === 'host') {
                const href = normalizeHttpLink(text);
                if (href) {
                    const safeHref = escapeHtml(href);
                    return `<td class="info-collect-cell" data-field="${escapeHtml(f)}" data-full="${escapeHtml(text)}" title="${escapeHtml(text)}"><a class="info-collect-link" href="${safeHref}" target="_blank" rel="noopener noreferrer" onclick="event.stopPropagation();">${escapeHtml(text)}</a></td>`;
                }
            }
            return `<td class="info-collect-cell" data-field="${escapeHtml(f)}" data-full="${escapeHtml(text)}" title="${escapeHtml(text)}"><span class="info-collect-cell-text">${escapeHtml(text)}</span></td>`;
        }).join('');

        const actionHtml = `
            <div class="info-collect-actions">
                <button class="btn-icon" onclick="copyFofaTargetEncoded('${encodedTarget}'); event.stopPropagation();" title="复制目标">
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                        <rect x="9" y="9" width="13" height="13" rx="2" stroke="currentColor" stroke-width="2"/>
                        <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
                    </svg>
                </button>
                <button class="btn-icon" onclick="scanFofaRow('${encoded}'); event.stopPropagation();" title="发送到对话进行扫描">
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                        <path d="M10.5 13.5l3-3" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
                        <path d="M8 8H5a4 4 0 1 0 0 8h3" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
                        <path d="M16 8h3a4 4 0 0 1 0 8h-3" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
                    </svg>
                </button>
            </div>
        `;

        return `<tr data-index="${idx}">${selectHtml}${cellsHtml}<td class="info-collect-col-actions">${actionHtml}</td></tr>`;
    }).join('');

    els.tbody.innerHTML = rowsHtml;

    // 更新全选框状态
    syncSelectAllCheckbox();
}

function inferTargetFromRow(row, fields) {
    // 优先 host（FOFA 常见返回 http(s)://...）
    const host = row.host != null ? String(row.host).trim() : '';
    if (host) return host;

    const domain = row.domain != null ? String(row.domain).trim() : '';
    const ip = row.ip != null ? String(row.ip).trim() : '';
    const port = row.port != null ? String(row.port).trim() : '';
    const protocol = row.protocol != null ? String(row.protocol).trim().toLowerCase() : '';

    const base = domain || ip;
    if (!base) return '';

    if (port) {
        // 仅做一个轻量推断：443 -> https, 80 -> http，其余不强行加 scheme
        const p = parseInt(port, 10);
        if (!isNaN(p) && (p === 80 || p === 443)) {
            const scheme = p === 443 ? 'https' : 'http';
            return `${scheme}://${base}:${p}`;
        }
        if (protocol === 'https' || protocol === 'http') {
            return `${protocol}://${base}:${port}`;
        }
        return `${base}:${port}`;
    }

    return base;
}

function normalizeHttpLink(raw) {
    const v = (raw || '').trim();
    if (!v) return '';
    if (v.startsWith('http://') || v.startsWith('https://')) return v;
    // 某些 host 可能是 domain 或 ip:port；这里不强行拼装，避免误导
    return '';
}

function copyFofaTarget(target) {
    const text = (target || '').trim();
    if (!text) {
        alert('没有可复制的目标');
        return;
    }
    navigator.clipboard.writeText(text).then(() => {
        // 简单提示
        showInlineToast('已复制目标');
    }).catch(() => {
        alert('复制失败，请手动复制：' + text);
    });
}

function copyFofaTargetEncoded(encodedTarget) {
    try {
        copyFofaTarget(decodeURIComponent(encodedTarget || ''));
    } catch (e) {
        copyFofaTarget(encodedTarget || '');
    }
}

function showInlineToast(text) {
    const toast = document.createElement('div');
    toast.textContent = text;
    toast.style.cssText = 'position: fixed; top: 24px; right: 24px; background: rgba(0,0,0,0.85); color: #fff; padding: 10px 12px; border-radius: 8px; z-index: 10000; font-size: 13px;';
    document.body.appendChild(toast);
    setTimeout(() => toast.remove(), 1200);
}

function scanFofaRow(encodedRowJson) {
    let row = {};
    try {
        row = JSON.parse(decodeURIComponent(encodedRowJson));
    } catch (e) {
        console.warn('解析行数据失败', e);
    }

    const fields = (document.getElementById('fofa-fields')?.value || '').split(',').map(s => s.trim()).filter(Boolean);
    const target = inferTargetFromRow(row, fields);
    if (!target) {
        alert('无法从该行推断扫描目标（建议在 fields 中包含 host/ip/port/domain）');
        return;
    }

    // 切换到对话页并发送消息
    if (typeof switchPage === 'function') {
        switchPage('chat');
    } else {
        window.location.hash = 'chat';
    }

    // 尽量切到“信息收集”角色（如果存在）
    try {
        if (typeof selectRole === 'function') {
            selectRole('信息收集');
        } else if (typeof handleRoleChange === 'function') {
            handleRoleChange('信息收集');
        }
    } catch (e) {
        // ignore
    }

    const message = buildScanMessage(target, row);

    setTimeout(() => {
        const input = document.getElementById('chat-input');
        if (input) {
            input.value = message;
            // 触发自动高度调整（chat.js 里如果监听 input）
            input.dispatchEvent(new Event('input', { bubbles: true }));
        }
        if (typeof sendMessage === 'function') {
            sendMessage();
        } else {
            alert('未找到 sendMessage()，请刷新页面后重试');
        }
    }, 200);
}

function buildScanMessage(target, row) {
    const title = row && row.title != null ? String(row.title).trim() : '';
    const server = row && row.server != null ? String(row.server).trim() : '';
    const hintParts = [];
    if (title) hintParts.push(`title="${title}"`);
    if (server) hintParts.push(`server="${server}"`);
    const hint = hintParts.length ? `（FOFA提示：${hintParts.join(', ')}）` : '';

    return `对以下目标做信息收集与基础扫描：\n${target}\n\n要求：\n1) 识别服务/框架与关键指纹\n2) 枚举开放端口与常见管理入口\n3) 用 httpx/指纹/目录探测等方式快速确认可访问面\n4) 输出可复现的命令与结论\n\n${hint}`.trim();
}

function bindFofaTableEvents() {
    if (infoCollectState.tableBound) return;
    infoCollectState.tableBound = true;

    const els = getFofaFormElements();
    if (!els.tbody) return;

    // 事件委托：选择/单元格展开
    els.tbody.addEventListener('click', (e) => {
        const checkbox = e.target && e.target.classList && e.target.classList.contains('fofa-row-select') ? e.target : null;
        if (checkbox) {
            const idx = parseInt(checkbox.getAttribute('data-index'), 10);
            if (!isNaN(idx)) {
                if (checkbox.checked) infoCollectState.selectedRowIndexes.add(idx);
                else infoCollectState.selectedRowIndexes.delete(idx);
                updateSelectedMeta();
                syncSelectAllCheckbox();
            }
            return;
        }

        const cell = e.target && e.target.closest ? e.target.closest('.info-collect-cell') : null;
        if (cell) {
            const full = cell.getAttribute('data-full') || '';
            const field = cell.getAttribute('data-field') || '';
            // 点击链接不弹窗
            if (e.target && e.target.tagName === 'A') return;
            if (full && full.length > 0) {
                showCellDetailModal(field, full);
            }
        }
    });

    // thead 的全选（因为 thead 会重渲染，用事件捕获到 document）
    document.addEventListener('change', (e) => {
        const t = e.target;
        if (!t || t.id !== 'fofa-select-all') return;
        const checked = !!t.checked;
        toggleSelectAllRows(checked);
    });
}

function toggleSelectAllRows(checked) {
    const els = getFofaFormElements();
    if (!els.tbody) return;
    const boxes = els.tbody.querySelectorAll('input.fofa-row-select');
    infoCollectState.selectedRowIndexes.clear();
    boxes.forEach(b => {
        b.checked = checked;
        const idx = parseInt(b.getAttribute('data-index'), 10);
        if (checked && !isNaN(idx)) infoCollectState.selectedRowIndexes.add(idx);
    });
    updateSelectedMeta();
    syncSelectAllCheckbox();
}

function syncSelectAllCheckbox() {
    const selectAll = document.getElementById('fofa-select-all');
    const els = getFofaFormElements();
    if (!selectAll || !els.tbody) return;
    const boxes = els.tbody.querySelectorAll('input.fofa-row-select');
    const total = boxes.length;
    const selected = infoCollectState.selectedRowIndexes.size;
    if (total === 0) {
        selectAll.checked = false;
        selectAll.indeterminate = false;
        return;
    }
    if (selected === 0) {
        selectAll.checked = false;
        selectAll.indeterminate = false;
    } else if (selected === total) {
        selectAll.checked = true;
        selectAll.indeterminate = false;
    } else {
        selectAll.checked = false;
        selectAll.indeterminate = true;
    }
}

function renderFofaColumnsPanel(allFields, visibleFields) {
    const els = getFofaFormElements();
    if (!els.columnsList) return;
    const currentVisible = new Set(visibleFields);
    els.columnsList.innerHTML = allFields.map(f => {
        const checked = currentVisible.has(f);
        const safe = escapeHtml(f);
        return `
            <label class="info-collect-col-item" title="${safe}">
                <input type="checkbox" ${checked ? 'checked' : ''} onchange="toggleFofaColumn('${safe}', this.checked)" />
                <span>${safe}</span>
            </label>
        `;
    }).join('');
}

function toggleFofaColumn(field, visible) {
    const f = String(field || '').trim();
    if (!f) return;
    if (visible) infoCollectState.hiddenFields.delete(f);
    else infoCollectState.hiddenFields.add(f);
    saveHiddenFieldsToStorage();
    // 重新渲染表格（用 state 中缓存的 payload）
    if (infoCollectState.currentPayload) {
        renderFofaResults(infoCollectState.currentPayload);
    }
}

function toggleFofaColumnsPanel() {
    const els = getFofaFormElements();
    if (!els.columnsPanel) return;
    const show = els.columnsPanel.style.display === 'none' || !els.columnsPanel.style.display;
    els.columnsPanel.style.display = show ? 'block' : 'none';
}

function closeFofaColumnsPanel() {
    const els = getFofaFormElements();
    if (els.columnsPanel) els.columnsPanel.style.display = 'none';
}

// 点击面板外部关闭（避免一直占着表格顶部）
document.addEventListener('click', (e) => {
    const panel = document.getElementById('fofa-columns-panel');
    const btn = e.target && e.target.closest ? e.target.closest('button') : null;
    const isColumnsBtn = btn && btn.getAttribute && btn.getAttribute('onclick') && String(btn.getAttribute('onclick')).includes('toggleFofaColumnsPanel');
    if (!panel || panel.style.display === 'none') return;
    if (panel.contains(e.target) || isColumnsBtn) return;
    panel.style.display = 'none';
});

function showAllFofaColumns() {
    infoCollectState.hiddenFields.clear();
    saveHiddenFieldsToStorage();
    if (infoCollectState.currentPayload) renderFofaResults(infoCollectState.currentPayload);
}

function hideAllFofaColumns() {
    const p = infoCollectState.currentPayload;
    if (!p || !Array.isArray(p.fields)) return;
    // 允许隐藏全部，但给用户一个最小可用：至少保留 host/ip/domain 中之一（如果存在）
    const keep = ['host', 'ip', 'domain'].find(x => p.fields.includes(x));
    infoCollectState.hiddenFields = new Set(p.fields.filter(f => f !== keep));
    saveHiddenFieldsToStorage();
    renderFofaResults(p);
}

function exportFofaResults(format) {
    const p = infoCollectState.currentPayload;
    if (!p || !Array.isArray(p.results) || p.results.length === 0) {
        alert('暂无可导出的结果');
        return;
    }

    const fields = p.fields || [];
    const visibleFields = fields.filter(f => !infoCollectState.hiddenFields.has(f));

    const now = new Date();
    const ts = `${now.getFullYear()}${String(now.getMonth() + 1).padStart(2, '0')}${String(now.getDate()).padStart(2, '0')}_${String(now.getHours()).padStart(2, '0')}${String(now.getMinutes()).padStart(2, '0')}${String(now.getSeconds()).padStart(2, '0')}`;

    if (format === 'json') {
        const payload = {
            query: p.query || '',
            total: p.total || 0,
            page: p.page || 1,
            size: p.size || 0,
            fields: fields,
            results: p.results
        };
        downloadBlob(JSON.stringify(payload, null, 2), `fofa_results_${ts}.json`, 'application/json;charset=utf-8');
        return;
    }

    // csv：默认导出可见字段（更符合“列隐藏”直觉）
    const header = visibleFields;
    const rows = p.results.map(row => {
        const r = row && typeof row === 'object' ? row : {};
        return header.map(f => csvEscape(r[f]));
    });
    const csv = [header.map(csvEscape).join(','), ...rows.map(cols => cols.join(','))].join('\n');
    downloadBlob(csv, `fofa_results_${ts}.csv`, 'text/csv;charset=utf-8');
}

function csvEscape(value) {
    if (value == null) return '""';
    const s = String(value).replace(/"/g, '""');
    return `"${s}"`;
}

function downloadBlob(content, filename, mime) {
    const blob = new Blob([content], { type: mime });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
}

async function batchScanSelectedFofaRows() {
    const p = infoCollectState.currentPayload;
    if (!p || !Array.isArray(p.results) || p.results.length === 0) {
        alert('暂无结果');
        return;
    }
    const selected = Array.from(infoCollectState.selectedRowIndexes).sort((a, b) => a - b);
    if (selected.length === 0) {
        alert('请先勾选需要扫描的行');
        return;
    }

    const fields = p.fields || [];
    const tasks = [];
    const skipped = [];
    selected.forEach(idx => {
        const row = p.results[idx];
        const target = inferTargetFromRow(row || {}, fields);
        if (!target) {
            skipped.push(idx + 1);
            return;
        }
        tasks.push(buildScanMessage(target, row || {}));
    });

    if (tasks.length === 0) {
        alert('未能从所选行推断任何可扫描目标（建议 fields 中包含 host/ip/port/domain）');
        return;
    }

    const title = (p.query ? `FOFA 批量扫描：${p.query}` : 'FOFA 批量扫描').slice(0, 80);
    try {
        const resp = await apiFetch('/api/batch-tasks', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ title, tasks, role: '信息收集' })
        });
        const result = await resp.json().catch(() => ({}));
        if (!resp.ok) {
            throw new Error(result.error || `创建批量队列失败: ${resp.status}`);
        }
        const queueId = result.queueId;
        if (!queueId) {
            throw new Error('创建成功但未返回 queueId');
        }

        // 跳到任务管理并打开队列详情
        if (typeof switchPage === 'function') switchPage('tasks');
        setTimeout(() => {
            if (typeof showBatchQueueDetail === 'function') {
                showBatchQueueDetail(queueId);
            }
        }, 250);

        if (skipped.length > 0) {
            showInlineToast(`已创建队列（跳过 ${skipped.length} 条无目标行）`);
        } else {
            showInlineToast('已创建批量扫描队列');
        }
    } catch (e) {
        console.error('批量扫描失败:', e);
        alert('批量扫描失败: ' + (e && e.message ? e.message : String(e)));
    }
}

function showCellDetailModal(field, fullText) {
    const existing = document.getElementById('info-collect-cell-modal');
    if (existing) existing.remove();

    const modal = document.createElement('div');
    modal.id = 'info-collect-cell-modal';
    modal.className = 'info-collect-cell-modal';
    modal.innerHTML = `
        <div class="info-collect-cell-modal-content" role="dialog" aria-modal="true">
            <div class="info-collect-cell-modal-header">
                <div class="info-collect-cell-modal-title">${escapeHtml(field || '字段')}</div>
                <button class="btn-icon" type="button" id="info-collect-cell-modal-close" title="关闭">
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                        <path d="M18 6L6 18M6 6l12 12" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
                    </svg>
                </button>
            </div>
            <div class="info-collect-cell-modal-body">
                <pre class="info-collect-cell-modal-pre">${escapeHtml(fullText || '')}</pre>
            </div>
            <div class="info-collect-cell-modal-footer">
                <button class="btn-secondary" type="button" id="info-collect-cell-modal-copy">复制</button>
                <button class="btn-primary" type="button" id="info-collect-cell-modal-ok">关闭</button>
            </div>
        </div>
    `;

    document.body.appendChild(modal);

    const close = () => modal.remove();
    modal.addEventListener('click', (e) => {
        if (e.target === modal) close();
    });
    document.getElementById('info-collect-cell-modal-close')?.addEventListener('click', close);
    document.getElementById('info-collect-cell-modal-ok')?.addEventListener('click', close);
    document.getElementById('info-collect-cell-modal-copy')?.addEventListener('click', () => {
        navigator.clipboard.writeText(fullText || '').then(() => showInlineToast('已复制')).catch(() => alert('复制失败'));
    });

    // Esc 关闭
    const onKey = (e) => {
        if (e.key === 'Escape') {
            close();
            document.removeEventListener('keydown', onKey);
        }
    };
    document.addEventListener('keydown', onKey);
}

// 暴露到全局（供 index.html onclick 调用）
window.initInfoCollectPage = initInfoCollectPage;
window.resetFofaForm = resetFofaForm;
window.submitFofaSearch = submitFofaSearch;
window.scanFofaRow = scanFofaRow;
window.copyFofaTarget = copyFofaTarget;
window.copyFofaTargetEncoded = copyFofaTargetEncoded;
window.applyFofaQueryPreset = applyFofaQueryPreset;
window.applyFofaFieldsPreset = applyFofaFieldsPreset;
window.toggleFofaColumnsPanel = toggleFofaColumnsPanel;
window.closeFofaColumnsPanel = closeFofaColumnsPanel;
window.showAllFofaColumns = showAllFofaColumns;
window.hideAllFofaColumns = hideAllFofaColumns;
window.toggleFofaColumn = toggleFofaColumn;
window.exportFofaResults = exportFofaResults;
window.batchScanSelectedFofaRows = batchScanSelectedFofaRows;

