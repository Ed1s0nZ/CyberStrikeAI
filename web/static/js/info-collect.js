// Infocollectpage(FOFA)
function _t(key, opts) {
    return typeof window.t === 'function' ? window.t(key, opts) : key;
}

const FOFA_FORM_STORAGE_KEY = 'info-collect-fofa-form';
const FOFA_HIDDEN_FIELDS_STORAGE_KEY = 'info-collect-fofa-hidden-fields';

const infoCollectState = {
    currentPayload: null, // { fields, results, query, total, page, size }
    hiddenFields: new Set(),
    selectedRowIndexes: new Set(),
    tableBound: false
};

// AI parse(naturallanguage -> FOFA)status
let fofaParseAbortController = null;
let fofaParseSlowTimer = null;
let fofaParseToastHandle = null;

// HTMLescape(ifnot defined)
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
        nl: document.getElementById('fofa-nl'),
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

    // restorehidefield
    infoCollectState.hiddenFields = new Set(loadHiddenFieldsFromStorage());

    // restoreontimeinput
    const saved = loadFofaFormFromStorage();
    if (saved) {
        if (typeof saved.query === 'string') els.query.value = saved.query;
        if (typeof saved.size === 'number' || typeof saved.size === 'string') els.size.value = saved.size;
        if (typeof saved.page === 'number' || typeof saved.page === 'string') els.page.value = saved.page;
        if (typeof saved.fields === 'string') els.fields.value = saved.fields;
        if (typeof saved.full === 'boolean') els.full.checked = saved.full;
    }

 // bind Enter fastQuery(at query inuse Ctrl/Cmd+Enter)
    els.query.addEventListener('keydown', (e) => {
        if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
            e.preventDefault();
            submitFofaSearch();
        }
    });

    // naturallanguageinput:Ctrl/Cmd+Enter triggerparse
    if (els.nl) {
        els.nl.addEventListener('keydown', (e) => {
            if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
                e.preventDefault();
                parseFofaNaturalLanguage();
            }
        });
    }

 // textarea:bycontentautoadd(avoiddefaultwhitespaceline)
    const autoGrowTextarea = (el) => {
        if (!el) return;
        try {
            el.style.height = '36px';
            const max = 96;
            const h = Math.min(max, el.scrollHeight);
            el.style.height = `${h}px`;
        } catch (e) {
            // ignore
        }
    };
    els.query.addEventListener('input', () => autoGrowTextarea(els.query));
    if (els.nl) els.nl.addEventListener('input', () => autoGrowTextarea(els.nl));
 // initializewhenalsolinetime
    setTimeout(() => {
        autoGrowTextarea(els.query);
        autoGrowTextarea(els.nl);
    }, 0);

 // bindtableevent(event,onlybindtime)
    bindFofaTableEvents();
    updateSelectedMeta();

    // Load dynamic recon panels from plugins (once)
    if (!window._reconPanelsLoaded) {
        loadReconPanels();
    }
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
        alert(_t('infoCollect.enterFofaQuery'));
        return;
    }

    saveFofaFormToStorage({ query, size, page, fields, full });
    setFofaMeta(_t('infoCollect.querying'));
    setFofaLoading(true);

    try {
        const response = await apiFetch('/api/fofa/search', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ query, size, page, fields, full })
        });

        const result = await response.json().catch(() => ({}));
        if (!response.ok) {
            throw new Error(result.error || `Request failed: ${response.status}`);
        }
        renderFofaResults(result);
    } catch (e) {
        console.error('FOFA QueryFailed:', e);
        setFofaMeta(_t('infoCollect.queryFailed'));
        renderFofaResults({ query, fields: [], results: [], total: 0, page: 1, size: 0 });
        alert(_t('infoCollect.queryFailed') + ': ' + (e && e.message ? e.message : String(e)));
    } finally {
        setFofaLoading(false);
    }
}

async function parseFofaNaturalLanguage() {
    const els = getFofaFormElements();
    const text = (els.nl?.value || '').trim();
    if (!text) {
        alert(_t('infoCollect.enterNaturalLanguage'));
        return;
    }

 // timeclick:Cancelenterlinein's parse(avoid"toas/Failed")
    if (fofaParseAbortController) {
        try { fofaParseAbortController.abort(); } catch (e) { /* ignore */ }
        return;
    }

 // firstcreate controller,avoidfast's repeatclicktriggerConcurrencyrequest
    fofaParseAbortController = new AbortController();
    setFofaParseLoading(true, _t('infoCollect.parsePending'));

 // hint:torequestcomplete/Cancel/Failedonly 
    fofaParseToastHandle = showInlineToast(_t('infoCollect.parsePendingClickCancel'), { duration: 0, id: 'fofa-parse-pending' });

 // ifexceedthroughsmalltimestillreturn,then"stillatenterlinein",fallasFailed's 
    fofaParseSlowTimer = setTimeout(() => {
        const status = document.getElementById('fofa-nl-status');
        if (status) {
            status.textContent = _t('infoCollect.parseSlow');
            status.style.display = 'block';
        }
    }, 1800);

    try {
        const resp = await apiFetch('/api/fofa/parse', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ text }),
            signal: fofaParseAbortController.signal
        });
        const result = await resp.json().catch(() => ({}));
        if (!resp.ok) {
            throw new Error(result.error || `Request failed: ${resp.status}`);
        }
        showFofaParseModal(text, result);
        showInlineToast(_t('infoCollect.parseDone'));
    } catch (e) {
        // AbortController Cancel:nottreated asFailed
        if (e && (e.name === 'AbortError' || String(e).includes('AbortError'))) {
            showInlineToast(_t('infoCollect.parseCancelled'));
            return;
        }
        console.error('FOFA naturallanguageParse failed:', e);
        showInlineToast(_t('infoCollect.parseFailed') + (e && e.message ? e.message : String(e)), { duration: 2800 });
    }
    finally {
        fofaParseAbortController = null;
        if (fofaParseSlowTimer) {
            clearTimeout(fofaParseSlowTimer);
            fofaParseSlowTimer = null;
        }
        if (fofaParseToastHandle && typeof fofaParseToastHandle.remove === 'function') {
            fofaParseToastHandle.remove();
        }
        fofaParseToastHandle = null;
        setFofaParseLoading(false, '');
    }
}

function setFofaParseLoading(loading, statusText) {
    const btn = document.getElementById('fofa-nl-parse-btn');
    const status = document.getElementById('fofa-nl-status');
    if (btn) {
        if (loading) {
            if (!btn.dataset.originalText) btn.dataset.originalText = btn.textContent || _t('infoCollectPage.parseBtn');
            btn.classList.add('btn-loading');
            btn.textContent = _t('infoCollect.cancelParse');
            btn.title = _t('infoCollect.clickToCancelParse');
            btn.dataset.loading = '1';
            btn.setAttribute('aria-busy', 'true');
            btn.disabled = false;
        } else {
            btn.classList.remove('btn-loading');
            btn.textContent = btn.dataset.originalText || _t('infoCollectPage.parseBtn');
            btn.title = _t('infoCollect.parseToFofa');
            btn.disabled = false;
            delete btn.dataset.loading;
            btn.removeAttribute('aria-busy');
        }
    }
    if (status) {
        const text = (statusText || '').trim();
        if (loading && text) {
            status.textContent = text;
            status.style.display = 'block';
        } else {
            status.textContent = '';
            status.style.display = 'none';
        }
    }
}

function showFofaParseModal(nlText, parsed) {
    const existing = document.getElementById('fofa-parse-modal');
    if (existing) existing.remove();

    const safeNL = escapeHtml((nlText || '').trim());
    const warnings = Array.isArray(parsed?.warnings) ? parsed.warnings.filter(Boolean).map(x => String(x)) : [];
    const explanation = parsed?.explanation != null ? String(parsed.explanation) : '';

    const warningsHtml = warnings.length
        ? `<ul style="margin: 8px 0 0 18px;">${warnings.map(w => `<li>${escapeHtml(w)}</li>`).join('')}</ul>`
        : '<div class="muted" style="margin-top: 8px;">' + _t('infoCollect.none') + '</div>';

    const modal = document.createElement('div');
    modal.id = 'fofa-parse-modal';
    modal.className = 'modal';
    modal.style.display = 'block';
    modal.innerHTML = `
        <div class="modal-content" style="max-width: 900px;">
            <div class="modal-header">
                <h2>${_t('infoCollect.parseResultTitle')}</h2>
                <span class="modal-close" id="fofa-parse-modal-close" title="${_t('common.close')}">&times;</span>
            </div>
            <div style="padding: 18px 28px; overflow: auto;">
                <div class="form-group">
                    <label>${_t('infoCollect.naturalLanguageLabel')}</label>
                    <div class="muted" style="margin-top: 6px; white-space: pre-wrap;">${safeNL || '-'}</div>
                </div>

                <div class="form-group" style="margin-top: 14px;">
                    <label for="fofa-parse-query">${_t('infoCollect.fofaQueryEditable')}</label>
                    <textarea id="fofa-parse-query" class="info-collect-query-input" rows="2" placeholder="${_t('infoCollect.queryPlaceholder')}"></textarea>
                    <small class="form-hint">${_t('infoCollect.confirmBeforeQuery')}</small>
                </div>

                <div class="form-group" style="margin-top: 14px;">
                    <label>${_t('infoCollect.reminder')}</label>
                    <div style="background: #fff8e1; border: 1px solid #ffe8a3; border-radius: 10px; padding: 10px 12px;">
                        ${warningsHtml}
                    </div>
                </div>

                ${explanation ? `
                <div class="form-group" style="margin-top: 14px;">
                    <label>${_t('infoCollect.explanation')}</label>
                    <pre style="margin-top: 8px; white-space: pre-wrap; background: var(--bg-tertiary); border: 1px solid var(--border-color); border-radius: 10px; padding: 10px 12px; font-size: 13px;">${escapeHtml(explanation)}</pre>
                </div>` : ''}
            </div>
            <div class="modal-footer" style="padding: 18px 28px;">
                <button class="btn-secondary" type="button" id="fofa-parse-cancel">${_t('infoCollect.parseModalCancel')}</button>
                <button class="btn-secondary" type="button" id="fofa-parse-apply">${_t('infoCollect.parseModalApply')}</button>
                <button class="btn-primary" type="button" id="fofa-parse-apply-run">${_t('infoCollect.parseModalApplyRun')}</button>
            </div>
        </div>
    `;

    document.body.appendChild(modal);

    const queryTextarea = document.getElementById('fofa-parse-query');
    if (queryTextarea) {
        queryTextarea.value = (parsed?.query || '').trim();
        setTimeout(() => {
            try { queryTextarea.focus(); } catch (e) { /* ignore */ }
        }, 0);
    }

    const close = () => modal.remove();
    modal.addEventListener('click', (e) => {
        if (e.target === modal) close();
    });
    document.getElementById('fofa-parse-modal-close')?.addEventListener('click', close);
    document.getElementById('fofa-parse-cancel')?.addEventListener('click', close);

    const applyToQuery = (run) => {
        const els = getFofaFormElements();
        const q = (queryTextarea?.value || '').trim();
        if (!q) {
            showInlineToast(_t('infoCollect.parseResultEmpty'), { duration: 2600 });
            return;
        }
        if (els.query) {
            els.query.value = q;
            try { els.query.focus(); } catch (e) { /* ignore */ }
        }
 // writeformcache(andhas"directlyQuery"consistent)
        saveFofaFormToStorage({
            query: q,
            size: parseInt(els.size?.value, 10) || 100,
            page: parseInt(els.page?.value, 10) || 1,
            fields: (els.fields?.value || '').trim(),
            full: !!els.full?.checked
        });
        close();
        if (run) submitFofaSearch();
    };

    document.getElementById('fofa-parse-apply')?.addEventListener('click', () => applyToQuery(false));
    document.getElementById('fofa-parse-apply-run')?.addEventListener('click', () => applyToQuery(true));

    // Esc Close
    const onKey = (e) => {
        if (e.key === 'Escape') {
            close();
            document.removeEventListener('keydown', onKey);
        }
    };
    document.addEventListener('keydown', onKey);
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
        els.selectedMeta.textContent = _t('infoCollectPage.selectedRows', { count: infoCollectState.selectedRowIndexes.size });
    }
}

function setFofaLoading(loading) {
    const els = getFofaFormElements();
    if (!els.tbody) return;
    if (loading) {
        const fieldsCount = (document.getElementById('fofa-fields')?.value || '').split(',').filter(Boolean).length;
        const colspan = Math.max(1, fieldsCount + 1);
        els.tbody.innerHTML = '<tr><td class="muted" style="padding: 16px;" colspan="' + colspan + '">' + escapeHtml(_t('infoCollect.loading')) + '</td></tr>';
    }
}

function renderFofaResults(payload) {
    const els = getFofaFormElements();
    if (!els.thead || !els.tbody) return;

    const fields = Array.isArray(payload.fields) ? payload.fields : [];
    const results = Array.isArray(payload.results) ? payload.results : [];

    // Savecurrent payload to state
    infoCollectState.currentPayload = {
        query: payload.query || '',
        total: typeof payload.total === 'number' ? payload.total : 0,
        page: typeof payload.page === 'number' ? payload.page : 1,
        size: typeof payload.size === 'number' ? payload.size : 0,
        fields,
        results
    };

 // cleanupselect(avoidfield/resultchange)
    infoCollectState.selectedRowIndexes.clear();
    updateSelectedMeta();

 // hidefield:onlypreservecurrent fields inexist's 
    const allowed = new Set(fields);
    infoCollectState.hiddenFields.forEach(f => {
        if (!allowed.has(f)) infoCollectState.hiddenFields.delete(f);
    });
    saveHiddenFieldsToStorage();

    const total = typeof payload.total === 'number' ? payload.total : 0;
    const size = typeof payload.size === 'number' ? payload.size : 0;
    const page = typeof payload.page === 'number' ? payload.page : 1;

    setFofaMeta(_t('infoCollect.resultsMeta', { total, count: results.length, page, size }));

 // canfield
    const visibleFields = fields.filter(f => !infoCollectState.hiddenFields.has(f));

    // columnpanel
    renderFofaColumnsPanel(fields, visibleFields);

 // table header(:column;:Actionscolumnfixed)
    const headerCells = [
        '<th class="info-collect-col-select"><input type="checkbox" id="fofa-select-all" title="' + escapeHtml(_t('infoCollect.selectAll')) + '"/></th>',
        ...visibleFields.map(f => `<th>${escapeHtml(String(f))}</th>`),
        '<th class="info-collect-col-actions">' + escapeHtml(_t('infoCollect.actions')) + '</th>'
    ].join('');
    els.thead.innerHTML = `<tr>${headerCells}</tr>`;

    // table body
    if (results.length === 0) {
        const colspan = Math.max(1, visibleFields.length + 2);
        els.tbody.innerHTML = '<tr><td class="muted" style="padding: 16px;" colspan="' + colspan + '">' + escapeHtml(_t('common.noData')) + '</td></tr>';
        return;
    }

    const rowsHtml = results.map((row, idx) => {
        const safeRow = row && typeof row === 'object' ? row : {};
        const target = inferTargetFromRow(safeRow, fields);
        const encoded = encodeURIComponent(JSON.stringify(safeRow));
        const encodedTarget = encodeURIComponent(target || '');

        const selectHtml = '<td class="info-collect-col-select"><input class="fofa-row-select" type="checkbox" data-index="' + idx + '" title="' + escapeHtml(_t('infoCollect.selectRow')) + '"/></td>';

        const cellsHtml = visibleFields.map(f => {
            const val = safeRow[f];
            const text = val == null ? '' : String(val);
 // host field:as much asrenderascanclicklink
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
                <button class="btn-icon" onclick="copyFofaTargetEncoded('${encodedTarget}'); event.stopPropagation();" title="${escapeHtml(_t('infoCollect.copyTarget'))}">
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                        <rect x="9" y="9" width="13" height="13" rx="2" stroke="currentColor" stroke-width="2"/>
                        <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
                    </svg>
                </button>
                <button class="btn-icon" onclick="scanFofaRow('${encoded}', event); event.stopPropagation();" title="${escapeHtml(_t('infoCollect.sendToChat'))}">
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

 // updateselect allstatus
    syncSelectAllCheckbox();
}

function inferTargetFromRow(row, fields) {
    // prefer host(FOFA commonreturn http(s)://...)
    const host = row.host != null ? String(row.host).trim() : '';
    if (host) return host;

    const domain = row.domain != null ? String(row.domain).trim() : '';
    const ip = row.ip != null ? String(row.ip).trim() : '';
    const port = row.port != null ? String(row.port).trim() : '';
    const protocol = row.protocol != null ? String(row.protocol).trim().toLowerCase() : '';

    const base = domain || ip;
    if (!base) return '';

    if (port) {
 // onlydolightweight:443 -> https, 80 -> http,itsnotline scheme
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
 // somethese host possiblyis domain or ip:port;herenotline,avoid
    return '';
}

function copyFofaTarget(target) {
    const text = (target || '').trim();
    if (!text) {
        alert(_t('infoCollect.noTargetToCopy'));
        return;
    }
    navigator.clipboard.writeText(text).then(() => {
        // simplehint
        showInlineToast(_t('infoCollect.targetCopied'));
    }).catch(() => {
        alert(_t('infoCollect.manualCopyHint') + text);
    });
}

function copyFofaTargetEncoded(encodedTarget) {
    try {
        copyFofaTarget(decodeURIComponent(encodedTarget || ''));
    } catch (e) {
        copyFofaTarget(encodedTarget || '');
    }
}

// showInlineToast('xxx');alsosupport showInlineToast('xxx', { duration: 0, id: '...' })
function showInlineToast(text, options) {
    const opts = options && typeof options === 'object' ? options : {};
    const duration = typeof opts.duration === 'number' ? opts.duration : 1200;
    const id = typeof opts.id === 'string' && opts.id.trim() ? opts.id.trim() : '';
    const replace = opts.replace !== false;

    if (id && replace) {
        document.getElementById(id)?.remove();
    }

    const toast = document.createElement('div');
    if (id) toast.id = id;
    toast.textContent = String(text == null ? '' : text);
    toast.style.cssText = 'position: fixed; top: 24px; right: 24px; background: rgba(0,0,0,0.85); color: #fff; padding: 10px 12px; border-radius: 8px; z-index: 10000; font-size: 13px; max-width: 420px; line-height: 1.4; box-shadow: 0 6px 18px rgba(0,0,0,0.22);';
    document.body.appendChild(toast);

    let timer = null;
    const remove = () => {
        try { if (timer) clearTimeout(timer); } catch (e) { /* ignore */ }
        timer = null;
        try { toast.remove(); } catch (e) { /* ignore */ }
    };

    if (duration > 0) {
        timer = setTimeout(remove, duration);
    }

    return { el: toast, remove };
}

function truncateForPreview(value, maxLen) {
    const s = value == null ? '' : String(value);
    if (maxLen <= 0 || s.length <= maxLen) return s;
    return s.slice(0, maxLen) + '...(' + _t('infoCollect.truncated') + ')';
}

function formatFofaRowSummary(row, fields) {
    const r = row && typeof row === 'object' ? row : {};
    const order = [];
    const seen = new Set();

    const preferred = Array.isArray(fields) ? fields : [];
    preferred.forEach(k => {
        const key = String(k || '').trim();
        if (!key || seen.has(key)) return;
        seen.add(key);
        order.push(key);
    });

    Object.keys(r).sort().forEach(k => {
        if (seen.has(k)) return;
        seen.add(k);
        order.push(k);
    });

    if (order.length === 0) return '-';

    const lines = order.map((k) => {
        const v = r[k];
        let text = '';
        if (v === null) text = 'null';
        else if (v === undefined) text = '';
        else if (typeof v === 'string') text = v === '' ? '""' : v;
        else if (typeof v === 'number' || typeof v === 'boolean') text = String(v);
        else {
            try { text = JSON.stringify(v); } catch (e) { text = String(v); }
        }
        text = truncateForPreview(text, 800);
        return `- ${k}: ${text}`;
    });

    return lines.join('\n');
}

function scanFofaRow(encodedRowJson, clickEvent) {
    let row = {};
    try {
        row = JSON.parse(decodeURIComponent(encodedRowJson));
    } catch (e) {
        console.warn('Failed to parse row data', e);
    }

    const fields = (document.getElementById('fofa-fields')?.value || '').split(',').map(s => s.trim()).filter(Boolean);
    const target = inferTargetFromRow(row, fields);
    if (!target) {
        alert(_t('infoCollect.cannotInferTarget'));
        return;
    }

 // switchtoconversationpagesendmessage(eachtimeclickall newConversation,avoidtohistoryConversation)
    if (typeof switchPage === 'function') {
        switchPage('chat');
    } else {
        window.location.hash = 'chat';
    }

    const message = buildScanMessage(target, row, { fields });
    const autoSend = !!(clickEvent && (clickEvent.ctrlKey || clickEvent.metaKey));

    setTimeout(async () => {
 // newConversation:mustwaititscomplete,otherwisewillatafterinput boxclear
        try {
            if (typeof startNewConversation === 'function') {
                const maybePromise = startNewConversation();
                if (maybePromise && typeof maybePromise.then === 'function') {
                    await maybePromise;
                }
            }
        } catch (e) {
            // ignore
        }

        const input = document.getElementById('chat-input');
        if (input) {
            input.value = message;
            // triggerautoheightadjust(chat.js iniflisten input)
            input.dispatchEvent(new Event('input', { bubbles: true }));
            input.focus();
        }
        if (autoSend) {
            if (typeof sendMessage === 'function') {
                sendMessage();
            } else {
                alert(_t('infoCollect.noSendMessage'));
            }
        } else {
            showInlineToast(_t('infoCollect.filledToInput'));
        }
    }, 250);
}

function buildScanMessage(target, row, options) {
    const opts = options && typeof options === 'object' ? options : {};
    const fields = Array.isArray(opts.fields) ? opts.fields : [];

    const summary = formatFofaRowSummary(row || {}, fields);
 return `forbelowTargetdoInfocollectandScan:\n${target}\n\nneed:\n1) recognizeservice/andclose\n2) enumopenPortandcommonmanagementin\n3) use httpx//directoryetcmethodfastconfirmaccessible\n4) outputcan's commandand\n\nalreadyInfo( FOFA thislineAllfield):\n${summary}`.trim();
}

function bindFofaTableEvents() {
    if (infoCollectState.tableBound) return;
    infoCollectState.tableBound = true;

    const els = getFofaFormElements();
    if (!els.tbody) return;

 // event:select/expand
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
            // clicklinknotpopup
            if (e.target && e.target.tagName === 'A') return;
            if (full && full.length > 0) {
                showCellDetailModal(field, full);
            }
        }
    });

 // thead 's select all(because thead willrender,useeventto document)
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
    // re-rendertable(use state incache's  payload)
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

// clickpanelExternalClose(avoidtabletop)
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
 // allowhideAll,buttouseminavailable:at leastpreserve host/ip/domain in(if exists)
    const keep = ['host', 'ip', 'domain'].find(x => p.fields.includes(x));
    infoCollectState.hiddenFields = new Set(p.fields.filter(f => f !== keep));
    saveHiddenFieldsToStorage();
    renderFofaResults(p);
}

function exportFofaResults(format) {
    const p = infoCollectState.currentPayload;
    if (!p || !Array.isArray(p.results) || p.results.length === 0) {
        alert(_t('infoCollect.noExportResult'));
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

    if (format === 'xlsx') {
 // use SheetJS generate XLSX(needatpageinin xlsx )
        if (typeof XLSX === 'undefined') {
            alert(_t('infoCollect.xlsxNotLoaded'));
            return;
        }
        const aoa = [visibleFields].concat(p.results.map(row => {
            const r = row && typeof row === 'object' ? row : {};
            return visibleFields.map(f => r[f] != null ? r[f] : '');
        }));
        const ws = XLSX.utils.aoa_to_sheet(aoa);
        const wb = XLSX.utils.book_new();
        XLSX.utils.book_append_sheet(wb, ws, _t('infoCollect.batchScanTitle'));
        XLSX.writeFile(wb, `fofa_results_${ts}.xlsx`);
        return;
    }

 // csv:defaultexportcanfield, UTF-8 BOM tocompatible Excel in
    const header = visibleFields;
    const rows = p.results.map(row => {
        const r = row && typeof row === 'object' ? row : {};
        return header.map(f => csvEscape(r[f]));
    });
    const csv = [header.map(csvEscape).join(','), ...rows.map(cols => cols.join(','))].join('\n');
    const csvWithBom = '\uFEFF' + csv;
    downloadBlob(csvWithBom, `fofa_results_${ts}.csv`, 'text/csv;charset=utf-8');
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
        alert(_t('infoCollect.noResults'));
        return;
    }
    const selected = Array.from(infoCollectState.selectedRowIndexes).sort((a, b) => a - b);
    if (selected.length === 0) {
        alert(_t('infoCollect.selectRowsFirst'));
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
 // batchtask:andrecordconsistent,only"thislineAllfield"'s summary(avoidrepeatandexceedlong)
        tasks.push(buildScanMessage(target, row || {}, {
            fields
        }));
    });

    if (tasks.length === 0) {
        alert(_t('infoCollect.noScanTarget'));
        return;
    }

    const title = (p.query ? _t('infoCollect.batchScanTitle') + ':' + p.query : _t('infoCollect.batchScanTitle')).slice(0, 80);
    try {
 // notforceswitchto"Infocollect"role:usecurrentalreadyrole;ifasdefaultthenemptystringtobackenddefault
        let role = '';
        if (typeof getCurrentRole === 'function') {
            try { role = getCurrentRole() || ''; } catch (e) { /* ignore */ }
        }
        if (role === 'default') role = '';

        const resp = await apiFetch('/api/batch-tasks', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ title, tasks, role })
        });
        const result = await resp.json().catch(() => ({}));
        if (!resp.ok) {
            throw new Error(result.error || _t('infoCollect.createQueueFailed') + ': ' + resp.status);
        }
        const queueId = result.queueId;
        if (!queueId) {
            throw new Error('Created successfully but did not return queueId');
        }

 // totaskmanagementopenqueuedetails
        if (typeof switchPage === 'function') switchPage('tasks');
        setTimeout(() => {
            if (typeof showBatchQueueDetail === 'function') {
                showBatchQueueDetail(queueId);
            }
        }, 250);

        if (skipped.length > 0) {
            showInlineToast(_t('infoCollect.queueCreatedSkipped', { n: skipped.length }));
        } else {
            showInlineToast(_t('infoCollect.batchQueueCreated'));
        }
    } catch (e) {
        console.error('Batch scan failed:', e);
        alert(_t('infoCollect.batchScanFailed') + ': ' + (e && e.message ? e.message : String(e)));
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
                <div class="info-collect-cell-modal-title">${escapeHtml(field || _t('infoCollect.field'))}</div>
                <button class="btn-icon" type="button" id="info-collect-cell-modal-close" title="${_t('common.close')}">
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                        <path d="M18 6L6 18M6 6l12 12" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
                    </svg>
                </button>
            </div>
            <div class="info-collect-cell-modal-body">
                <pre class="info-collect-cell-modal-pre">${escapeHtml(fullText || '')}</pre>
            </div>
            <div class="info-collect-cell-modal-footer">
                <button class="btn-secondary" type="button" id="info-collect-cell-modal-copy">${_t('common.copy')}</button>
                <button class="btn-primary" type="button" id="info-collect-cell-modal-ok">${_t('common.close')}</button>
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
        navigator.clipboard.writeText(fullText || '').then(() => showInlineToast(_t('common.copied'))).catch(() => alert(_t('common.copyFailed')));
    });

    // Esc Close
    const onKey = (e) => {
        if (e.key === 'Escape') {
            close();
            document.removeEventListener('keydown', onKey);
        }
    };
    document.addEventListener('keydown', onKey);
}

// exposetoglobal(for index.html onclick call)
window.initInfoCollectPage = initInfoCollectPage;
window.resetFofaForm = resetFofaForm;
window.submitFofaSearch = submitFofaSearch;
window.parseFofaNaturalLanguage = parseFofaNaturalLanguage;
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

// ═══════════════════════════════════════════════
// Dynamic Recon Panel Loading from Plugins
// ═══════════════════════════════════════════════

async function loadReconPanels() {
    try {
        const resp = await fetch('/api/plugins/recon-panels');
        if (!resp.ok) return;
        const data = await resp.json();
        if (!data.panels || data.panels.length === 0) return;

        const tabBar = document.getElementById('recon-tabs');
        const panelContainer = document.getElementById('recon-plugin-panels');
        if (!tabBar || !panelContainer) return;

        for (const entry of data.panels) {
            const panel = entry.panel;
            const pluginName = entry.plugin;

            // Add tab button
            const tab = document.createElement('button');
            tab.className = 'recon-tab';
            tab.setAttribute('data-panel', panel.id);
            tab.onclick = () => switchReconTab(panel.id);
            tab.innerHTML = (panel.icon ? panel.icon + ' ' : '') +
                           (panel.i18n && window.t ? window.t(panel.i18n) : panel.label);
            tabBar.appendChild(tab);

            // Load form HTML
            if (panel.form) {
                try {
                    const formResp = await fetch(`/api/plugins/${pluginName}/web/recon/${panel.form}`);
                    if (formResp.ok) {
                        const html = await formResp.text();
                        const panelDiv = document.createElement('div');
                        panelDiv.id = 'recon-panel-' + panel.id;
                        panelDiv.className = 'recon-panel'; // no .active = hidden by CSS
                        panelDiv.innerHTML = html;
                        panelContainer.appendChild(panelDiv);
                    }
                } catch (e) {
                    console.warn(`Plugin ${pluginName}: failed to load recon form`, e);
                }
            }

            // Load script
            if (panel.script) {
                try {
                    const script = document.createElement('script');
                    script.src = `/api/plugins/${pluginName}/web/recon/${panel.script}`;
                    document.body.appendChild(script);
                } catch (e) {
                    console.warn(`Plugin ${pluginName}: failed to load recon script`, e);
                }
            }
        }
        window._reconPanelsLoaded = true;
    } catch (e) {
        console.warn('Failed to load recon panels:', e);
    }
}

var currentReconTab = 'fofa';

function switchReconTab(panelId) {
    currentReconTab = panelId;
    // Hide all panels (use class, not inline style - CSS uses .active for display)
    document.querySelectorAll('.recon-panel').forEach(p => {
        p.classList.remove('active');
        p.removeAttribute('style'); // clear any inline display overrides
    });
    // Show selected
    const panel = document.getElementById('recon-panel-' + panelId);
    if (panel) panel.classList.add('active');
    // Update tab active state
    document.querySelectorAll('.recon-tab').forEach(t => t.classList.remove('active'));
    const tab = document.querySelector('.recon-tab[data-panel="' + panelId + '"]');
    if (tab) tab.classList.add('active');
}

function resetCurrentReconForm() {
    if (currentReconTab === 'fofa') {
        resetFofaForm();
    } else {
        // Plugin panels handle their own reset via window['reset_' + panelId]
        const fn = window['reset_' + currentReconTab];
        if (typeof fn === 'function') fn();
    }
}

function submitCurrentReconSearch() {
    if (currentReconTab === 'fofa') {
        submitFofaSearch();
    } else {
        const fn = window['submit_' + currentReconTab];
        if (typeof fn === 'function') fn();
    }
}

// Export
window.switchReconTab = switchReconTab;
window.resetCurrentReconForm = resetCurrentReconForm;
window.submitCurrentReconSearch = submitCurrentReconSearch;
window.loadReconPanels = loadReconPanels;

document.addEventListener('languagechange', function () {
    updateSelectedMeta();
});

if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', function () { updateSelectedMeta(); });
} else {
    updateSelectedMeta();
}

