// 多代理子 Agent Markdown（agents/*.md）管理
function _agentsT(key, opts) {
    return typeof window.t === 'function' ? window.t(key, opts) : key;
}

let markdownAgentsEditingFilename = null;
let markdownAgentsEditingIsOrchestrator = false;

function bindAgentsMdListDelegation() {
    const listEl = document.getElementById('agents-md-list');
    if (!listEl || listEl.dataset.agentsClickBound === '1') return;
    listEl.dataset.agentsClickBound = '1';
    listEl.addEventListener('click', function (e) {
        var t = e.target;
        if (!t || !t.closest) return;
        var editBtn = t.closest('[data-action="edit-agent-md"]');
        var delBtn = t.closest('[data-action="delete-agent-md"]');
        if (editBtn) {
            var f = editBtn.getAttribute('data-agent-file');
            if (f) {
                try { editMarkdownAgent(decodeURIComponent(f)); } catch (err) { console.warn(err); }
            }
            return;
        }
        if (delBtn) {
            var f2 = delBtn.getAttribute('data-agent-file');
            if (f2) {
                try { deleteMarkdownAgent(decodeURIComponent(f2)); } catch (err2) { console.warn(err2); }
            }
        }
    });
}

async function loadMarkdownAgents() {
    const listEl = document.getElementById('agents-md-list');
    const dirEl = document.getElementById('agents-md-dir');
    if (!listEl) return;
    bindAgentsMdListDelegation();
    listEl.innerHTML = '<div class="loading-spinner">' + _agentsT('agentsPage.loading') + '</div>';
    try {
        const r = await apiFetch('/api/multi-agent/markdown-agents');
        const data = await r.json();
        if (!r.ok) {
            throw new Error(data.error || r.statusText);
        }
        if (dirEl) {
            const d = data.dir || '';
            dirEl.textContent = d ? (_agentsT('agentsPage.dirLabel') + ': ' + d) : '';
        }
        const agents = data.agents || [];
        if (agents.length === 0) {
            listEl.innerHTML = '<div class="empty-state">' + _agentsT('agentsPage.empty') + '</div>';
            return;
        }
        agents.sort(function (x, y) {
            var ox = x.is_orchestrator ? 1 : 0;
            var oy = y.is_orchestrator ? 1 : 0;
            return oy - ox;
        });
        listEl.innerHTML = agents.map(function (a) {
            const rawFn = a.filename || '';
            const fn = escapeHtml(rawFn);
            const id = escapeHtml(a.id || '');
            const name = escapeHtml(a.name || '');
            const desc = escapeHtml(a.description || _agentsT('agentsPage.noDesc'));
            const orch = !!a.is_orchestrator;
            const badgeLabel = orch ? _agentsT('agentsPage.badgeOrchestrator') : _agentsT('agentsPage.badgeSub');
            const badgeClass = orch ? 'agent-role-badge agent-role-badge--orchestrator' : 'agent-role-badge agent-role-badge--sub';
            return (
                '<div class="skill-card">' +
                '<div class="skill-card-header">' +
                '<h3 class="skill-card-title">' + name + '<span class="' + badgeClass + '">' + escapeHtml(badgeLabel) + '</span></h3>' +
                '<div class="skill-card-description"><code>' + fn + '</code> · id: <code>' + id + '</code><br>' + desc + '</div>' +
                '</div>' +
                '<div class="skill-card-actions">' +
                '<button type="button" class="btn-secondary btn-small" data-action="edit-agent-md" data-agent-file="' + encodeURIComponent(rawFn) + '">' + escapeHtml(_agentsT('common.edit')) + '</button>' +
                '<button type="button" class="btn-secondary btn-small btn-danger" data-action="delete-agent-md" data-agent-file="' + encodeURIComponent(rawFn) + '">' + escapeHtml(_agentsT('common.delete')) + '</button>' +
                '</div></div>'
            );
        }).join('');
    } catch (e) {
        console.error(e);
        listEl.innerHTML = '<div class="empty-state">' + escapeHtml(e.message || String(e)) + '</div>';
        showNotification(_agentsT('agentsPage.loadFailed') + ': ' + e.message, 'error');
    }
}

function showAddMarkdownAgentModal() {
    markdownAgentsEditingFilename = null;
    markdownAgentsEditingIsOrchestrator = false;
    const modal = document.getElementById('agent-md-modal');
    const title = document.getElementById('agent-md-modal-title');
    const row = document.getElementById('agent-md-filename-row');
    if (title) title.textContent = _agentsT('agentsPage.createTitle');
    if (row) row.style.display = '';
    document.getElementById('agent-md-filename-current').value = '';
    document.getElementById('agent-md-filename').value = '';
    document.getElementById('agent-md-filename').disabled = false;
    var roleEl = document.getElementById('agent-md-role');
    if (roleEl) roleEl.value = 'sub';
    document.getElementById('agent-md-id').value = '';
    document.getElementById('agent-md-name').value = '';
    document.getElementById('agent-md-description').value = '';
    document.getElementById('agent-md-tools').value = '';
    document.getElementById('agent-md-bind-role').value = '';
    document.getElementById('agent-md-max-iter').value = '0';
    document.getElementById('agent-md-instruction').value = '';
    if (modal) modal.style.display = 'flex';
}

async function editMarkdownAgent(filename) {
    if (!filename) return;
    const modal = document.getElementById('agent-md-modal');
    const title = document.getElementById('agent-md-modal-title');
    const row = document.getElementById('agent-md-filename-row');
    markdownAgentsEditingFilename = null;
    markdownAgentsEditingIsOrchestrator = false;
    if (title) title.textContent = _agentsT('agentsPage.editTitle');
    if (row) row.style.display = 'none';
    try {
        const r = await apiFetch('/api/multi-agent/markdown-agents/' + encodeURIComponent(filename));
        const data = await r.json();
        if (!r.ok) throw new Error(data.error || r.statusText);
        markdownAgentsEditingFilename = data.filename || filename;
        markdownAgentsEditingIsOrchestrator = !!data.is_orchestrator;
        document.getElementById('agent-md-filename-current').value = data.filename || filename;
        document.getElementById('agent-md-filename').value = data.filename || filename;
        document.getElementById('agent-md-filename').disabled = true;
        var roleEl2 = document.getElementById('agent-md-role');
        if (roleEl2) roleEl2.value = data.is_orchestrator ? 'orchestrator' : 'sub';
        document.getElementById('agent-md-id').value = data.id || '';
        document.getElementById('agent-md-name').value = data.name || '';
        document.getElementById('agent-md-description').value = data.description || '';
        document.getElementById('agent-md-tools').value = Array.isArray(data.tools) ? data.tools.join(', ') : '';
        document.getElementById('agent-md-bind-role').value = data.bind_role || '';
        document.getElementById('agent-md-max-iter').value = String(data.max_iterations != null ? data.max_iterations : 0);
        document.getElementById('agent-md-instruction').value = data.instruction || '';
        if (modal) modal.style.display = 'flex';
    } catch (e) {
        showNotification(_agentsT('agentsPage.loadOneFailed') + ': ' + e.message, 'error');
    }
}

function closeMarkdownAgentModal() {
    const modal = document.getElementById('agent-md-modal');
    if (modal) modal.style.display = 'none';
    markdownAgentsEditingFilename = null;
    markdownAgentsEditingIsOrchestrator = false;
}

function parseToolsInput(s) {
    if (!s || !String(s).trim()) return [];
    return String(s).split(/[,;|]/).map(function (x) { return x.trim(); }).filter(Boolean);
}

async function saveMarkdownAgent() {
    const name = document.getElementById('agent-md-name').value.trim();
    if (!name) {
        showNotification(_agentsT('agentsPage.nameRequired'), 'error');
        return;
    }
    const roleSel = document.getElementById('agent-md-role');
    const roleVal = roleSel ? roleSel.value : 'sub';
    const fnDraft = (document.getElementById('agent-md-filename') && document.getElementById('agent-md-filename').value.trim().toLowerCase()) || '';
    const isOrchestratorAgent = markdownAgentsEditingIsOrchestrator ||
        roleVal === 'orchestrator' ||
        fnDraft === 'orchestrator.md';
    const instruction = document.getElementById('agent-md-instruction').value.trim();
    if (!isOrchestratorAgent && !instruction) {
        showNotification(_agentsT('agentsPage.instructionRequired'), 'error');
        return;
    }
    const body = {
        id: document.getElementById('agent-md-id').value.trim(),
        name: name,
        description: document.getElementById('agent-md-description').value.trim(),
        tools: parseToolsInput(document.getElementById('agent-md-tools').value),
        instruction: instruction,
        bind_role: document.getElementById('agent-md-bind-role').value.trim(),
        max_iterations: parseInt(document.getElementById('agent-md-max-iter').value, 10) || 0,
        kind: roleVal === 'orchestrator' ? 'orchestrator' : ''
    };
    const isEdit = !!markdownAgentsEditingFilename;
    let url;
    let method;
    if (isEdit) {
        url = '/api/multi-agent/markdown-agents/' + encodeURIComponent(markdownAgentsEditingFilename);
        method = 'PUT';
    } else {
        url = '/api/multi-agent/markdown-agents';
        method = 'POST';
        const fn = document.getElementById('agent-md-filename').value.trim();
        if (fn && !/^[a-zA-Z0-9][a-zA-Z0-9_.-]*\.md$/.test(fn)) {
            showNotification(_agentsT('agentsPage.filenameInvalid'), 'error');
            return;
        }
        body.filename = fn;
    }
    try {
        const r = await apiFetch(url, {
            method: method,
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(body)
        });
        const data = await r.json().catch(function () { return {}; });
        if (!r.ok) throw new Error(data.error || r.statusText);
        showNotification(isEdit ? _agentsT('agentsPage.saveOk') : _agentsT('agentsPage.createOk'), 'success');
        closeMarkdownAgentModal();
        await loadMarkdownAgents();
    } catch (e) {
        showNotification(_agentsT('agentsPage.saveFailed') + ': ' + e.message, 'error');
    }
}

async function deleteMarkdownAgent(filename) {
    if (!filename) return;
    if (!confirm(_agentsT('agentsPage.deleteConfirm', { name: filename }))) return;
    try {
        const r = await apiFetch('/api/multi-agent/markdown-agents/' + encodeURIComponent(filename), { method: 'DELETE' });
        const data = await r.json().catch(function () { return {}; });
        if (!r.ok) throw new Error(data.error || r.statusText);
        showNotification(_agentsT('agentsPage.deleteOk'), 'success');
        await loadMarkdownAgents();
    } catch (e) {
        showNotification(_agentsT('agentsPage.deleteFailed') + ': ' + e.message, 'error');
    }
}
