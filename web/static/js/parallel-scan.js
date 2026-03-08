// parallel-scan.js — Parallel Scan UI for CyberStrikeAI
(function () {
    'use strict';

    let currentScanId = null;
    let scanEventSource = null;
    let attackVectors = [];
    let scanAgents = {};

    // ── Init ──────────────────────────────────────────────
    window.initParallelScanUI = function () {
        fetchAttackVectors();
    };

    async function fetchAttackVectors() {
        try {
            const res = await apiFetch('/api/parallel-scan/vectors');
            if (res.ok) {
                const data = await res.json();
                attackVectors = data.vectors || [];
            }
        } catch (e) {
            console.error('Failed to fetch attack vectors:', e);
        }
    }

    // ── Start Scan ────────────────────────────────────────
    window.startParallelScan = async function () {
        const targetInput = document.getElementById('parallel-scan-target');
        const maxRoundsInput = document.getElementById('parallel-scan-rounds');
        const reconInput = document.getElementById('parallel-scan-recon');
        if (!targetInput) return;

        const target = targetInput.value.trim();
        if (!target) {
            alert('Please enter a target');
            return;
        }

        // Collect selected agents
        const selectedAgents = [];
        document.querySelectorAll('.parallel-agent-checkbox:checked').forEach(cb => {
            selectedAgents.push(cb.value);
        });

        const maxRounds = parseInt(maxRoundsInput?.value) || 20;
        const reconContext = reconInput?.value?.trim() || '';

        const body = {
            target: target,
            agents: selectedAgents.length > 0 ? selectedAgents : undefined,
            maxRounds: maxRounds,
            reconContext: reconContext || undefined,
        };

        try {
            const res = await apiFetch('/api/parallel-scan', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(body),
            });

            if (!res.ok) {
                const err = await res.json();
                alert('Failed to start scan: ' + (err.error || res.statusText));
                return;
            }

            const scan = await res.json();
            currentScanId = scan.id;
            scanAgents = {};
            (scan.agents || []).forEach(a => { scanAgents[a.id] = a; });

            renderParallelScanTabs(scan);
            connectScanStream(scan.id);
            showParallelScanView();
        } catch (e) {
            console.error('Start parallel scan error:', e);
            alert('Error: ' + e.message);
        }
    };

    // ── SSE Stream ────────────────────────────────────────
    function connectScanStream(scanId) {
        if (scanEventSource) {
            scanEventSource.close();
        }

        const token = localStorage.getItem('auth_token') || '';
        const url = `/api/parallel-scan/${scanId}/stream?token=${encodeURIComponent(token)}`;
        scanEventSource = new EventSource(url);

        scanEventSource.onmessage = function (e) {
            try {
                const event = JSON.parse(e.data);
                handleScanEvent(event);
            } catch (err) {
                console.error('Parse SSE event error:', err);
            }
        };

        scanEventSource.onerror = function () {
            console.warn('SSE connection error, will reconnect...');
        };
    }

    function handleScanEvent(event) {
        // Update agent state
        if (event.agentId && scanAgents[event.agentId]) {
            const agent = scanAgents[event.agentId];
            if (event.data) {
                if (event.data.status) agent.status = event.data.status;
                if (event.data.round !== undefined) agent.currentRound = event.data.round;
                if (event.data.maxRounds !== undefined) agent.maxRounds = event.data.maxRounds;
            }
            if (event.type === 'iteration') agent.totalIterations = (agent.totalIterations || 0) + 1;
            if (event.type === 'tool_call') agent.totalToolCalls = (agent.totalToolCalls || 0) + 1;
            if (event.type === 'vulnerability') agent.totalVulns = (agent.totalVulns || 0) + 1;
        }

        // Update summary tab
        updateSummaryTable();

        // Append event to agent tab log
        appendAgentLog(event);

        // Handle scan completion
        if (event.type === 'scan_done') {
            updateScanStatus('completed');
            if (scanEventSource) {
                scanEventSource.close();
                scanEventSource = null;
            }
        }
        if (event.type === 'agent_done') {
            const agent = scanAgents[event.agentId];
            if (agent) {
                agent.status = 'completed';
                if (event.data) {
                    agent.totalIterations = event.data.totalIterations || agent.totalIterations;
                    agent.totalToolCalls = event.data.totalToolCalls || agent.totalToolCalls;
                    agent.totalVulns = event.data.totalVulns || agent.totalVulns;
                }
            }
            updateSummaryTable();
        }
    }

    // ── Tabs UI ───────────────────────────────────────────
    function renderParallelScanTabs(scan) {
        const container = document.getElementById('parallel-scan-container');
        if (!container) return;

        const agents = scan.agents || [];

        let tabsHtml = `<div class="ps-tabs-header">
            <button class="ps-tab-btn active" data-tab="ps-summary">Summary</button>`;
        agents.forEach(a => {
            tabsHtml += `<button class="ps-tab-btn" data-tab="ps-agent-${a.id}">${a.name}</button>`;
        });
        tabsHtml += `<div class="ps-tabs-actions">
            <button class="ps-stop-all-btn" onclick="stopParallelScan()">Stop All</button>
        </div></div>`;

        // Tab content
        let contentHtml = `<div id="ps-summary" class="ps-tab-content active">
            <div class="ps-summary-info">
                <strong>Target:</strong> ${escapeHtml(scan.target)} &nbsp;|&nbsp;
                <strong>Status:</strong> <span id="ps-scan-status">${scan.status}</span> &nbsp;|&nbsp;
                <strong>Max Rounds:</strong> ${scan.maxRounds}
            </div>
            <table class="ps-summary-table">
                <thead><tr>
                    <th>Agent</th><th>Status</th><th>Round</th>
                    <th>Iterations</th><th>Tool Calls</th><th>Vulns</th><th>Actions</th>
                </tr></thead>
                <tbody id="ps-summary-tbody"></tbody>
            </table>
        </div>`;

        agents.forEach(a => {
            contentHtml += `<div id="ps-agent-${a.id}" class="ps-tab-content" style="display:none;">
                <div class="ps-agent-header">
                    <span class="ps-agent-name">${a.name}</span>
                    <span class="ps-agent-status" id="ps-agent-status-${a.id}">${a.status}</span>
                    <button class="ps-agent-stop-btn" onclick="stopParallelAgent('${a.id}')">Stop</button>
                    <button class="ps-agent-restart-btn" onclick="restartParallelAgent('${a.id}')">Restart</button>
                    ${a.conversationId ? `<button class="ps-agent-chat-btn" onclick="openAgentConversation('${a.conversationId}')">View Chat</button>` : ''}
                </div>
                <div class="ps-agent-log" id="ps-agent-log-${a.id}"></div>
            </div>`;
        });

        container.innerHTML = tabsHtml + contentHtml;

        // Wire tab clicks
        container.querySelectorAll('.ps-tab-btn').forEach(btn => {
            btn.addEventListener('click', function () {
                container.querySelectorAll('.ps-tab-btn').forEach(b => b.classList.remove('active'));
                container.querySelectorAll('.ps-tab-content').forEach(c => { c.style.display = 'none'; c.classList.remove('active'); });
                this.classList.add('active');
                const target = document.getElementById(this.dataset.tab);
                if (target) { target.style.display = 'block'; target.classList.add('active'); }
            });
        });

        updateSummaryTable();
    }

    function updateSummaryTable() {
        const tbody = document.getElementById('ps-summary-tbody');
        if (!tbody) return;

        let html = '';
        Object.values(scanAgents).forEach(a => {
            const statusClass = a.status === 'running' ? 'ps-status-running' :
                                a.status === 'completed' ? 'ps-status-completed' :
                                a.status === 'cancelled' ? 'ps-status-cancelled' : 'ps-status-pending';
            html += `<tr>
                <td>${a.name}</td>
                <td><span class="${statusClass}">${a.status}</span></td>
                <td>${a.currentRound || 0}</td>
                <td>${a.totalIterations || 0}</td>
                <td>${a.totalToolCalls || 0}</td>
                <td>${a.totalVulns || 0}</td>
                <td>
                    <button class="ps-btn-sm" onclick="stopParallelAgent('${a.id}')" ${a.status !== 'running' ? 'disabled' : ''}>Stop</button>
                    <button class="ps-btn-sm" onclick="restartParallelAgent('${a.id}')" ${a.status === 'running' ? 'disabled' : ''}>Restart</button>
                </td>
            </tr>`;
        });
        tbody.innerHTML = html;
    }

    function updateScanStatus(status) {
        const el = document.getElementById('ps-scan-status');
        if (el) el.textContent = status;
    }

    function appendAgentLog(event) {
        const logEl = document.getElementById('ps-agent-log-' + event.agentId);
        if (!logEl) return;

        const entry = document.createElement('div');
        entry.className = 'ps-log-entry ps-log-' + event.type;

        const time = new Date().toLocaleTimeString();
        let text = `[${time}] [${event.type}]`;
        if (event.message) text += ' ' + event.message;
        if (event.type === 'tool_call' && event.data) {
            const toolName = event.data.toolName || event.data.name || '';
            if (toolName) text += ' → ' + toolName;
        }
        if (event.type === 'vulnerability' && event.data) {
            const severity = event.data.severity || '';
            const title = event.data.title || event.message || '';
            text += ` [${severity}] ${title}`;
        }

        entry.textContent = text;
        logEl.appendChild(entry);
        logEl.scrollTop = logEl.scrollHeight;

        // Also update agent status badge in tab content
        const statusEl = document.getElementById('ps-agent-status-' + event.agentId);
        if (statusEl && event.data && event.data.status) {
            statusEl.textContent = event.data.status;
        }
    }

    // ── Controls ──────────────────────────────────────────
    window.stopParallelScan = async function () {
        if (!currentScanId) return;
        try {
            await apiFetch(`/api/parallel-scan/${currentScanId}/stop`, { method: 'POST' });
            updateScanStatus('cancelled');
        } catch (e) {
            console.error('Stop scan error:', e);
        }
    };

    window.stopParallelAgent = async function (agentId) {
        if (!currentScanId) return;
        try {
            await apiFetch(`/api/parallel-scan/${currentScanId}/agents/${agentId}/stop`, { method: 'POST' });
        } catch (e) {
            console.error('Stop agent error:', e);
        }
    };

    window.restartParallelAgent = async function (agentId) {
        if (!currentScanId) return;
        try {
            await apiFetch(`/api/parallel-scan/${currentScanId}/agents/${agentId}/restart`, { method: 'POST' });
        } catch (e) {
            console.error('Restart agent error:', e);
        }
    };

    window.openAgentConversation = function (conversationId) {
        if (typeof loadConversation === 'function') {
            loadConversation(conversationId);
        }
    };

    // ── Mode Toggle ───────────────────────────────────────
    window.toggleScanMode = function (mode) {
        const singleView = document.getElementById('chat-container-wrapper');
        const parallelForm = document.getElementById('parallel-scan-form');
        const parallelContainer = document.getElementById('parallel-scan-container');
        const btnSingle = document.getElementById('mode-btn-single');
        const btnParallel = document.getElementById('mode-btn-parallel');

        if (mode === 'parallel') {
            if (singleView) singleView.style.display = 'none';
            if (parallelForm) parallelForm.style.display = 'block';
            if (parallelContainer) parallelContainer.style.display = 'block';
            if (btnSingle) btnSingle.classList.remove('active');
            if (btnParallel) btnParallel.classList.add('active');
            renderParallelScanForm();
        } else {
            if (singleView) singleView.style.display = '';
            if (parallelForm) parallelForm.style.display = 'none';
            if (parallelContainer) parallelContainer.style.display = 'none';
            if (btnSingle) btnSingle.classList.add('active');
            if (btnParallel) btnParallel.classList.remove('active');
        }
    };

    function showParallelScanView() {
        const form = document.getElementById('parallel-scan-form');
        if (form) form.style.display = 'none';
        const container = document.getElementById('parallel-scan-container');
        if (container) container.style.display = 'block';
    }

    function renderParallelScanForm() {
        const form = document.getElementById('parallel-scan-form');
        if (!form) return;

        let agentCheckboxes = '';
        attackVectors.forEach(v => {
            agentCheckboxes += `<label class="ps-checkbox-label">
                <input type="checkbox" class="parallel-agent-checkbox" value="${v.name}" checked>
                <span class="ps-checkbox-name">${v.name}</span>
                <span class="ps-checkbox-desc">${v.description}</span>
            </label>`;
        });

        form.innerHTML = `
            <div class="ps-form-group">
                <label for="parallel-scan-target">Target</label>
                <input type="text" id="parallel-scan-target" placeholder="e.g. example.com" class="ps-input">
            </div>
            <div class="ps-form-group">
                <label>Attack Vectors</label>
                <div class="ps-checkbox-group">${agentCheckboxes}</div>
            </div>
            <div class="ps-form-row">
                <div class="ps-form-group ps-form-half">
                    <label for="parallel-scan-rounds">Max Rounds</label>
                    <input type="number" id="parallel-scan-rounds" value="20" min="1" max="100" class="ps-input">
                </div>
            </div>
            <div class="ps-form-group">
                <label for="parallel-scan-recon">Recon Context (optional)</label>
                <textarea id="parallel-scan-recon" rows="3" class="ps-input" placeholder="Paste any recon data, subdomains, etc."></textarea>
            </div>
            <button class="ps-start-btn" onclick="startParallelScan()">Start Parallel Scan</button>
        `;
    }

    function escapeHtml(str) {
        const div = document.createElement('div');
        div.appendChild(document.createTextNode(str));
        return div.innerHTML;
    }

    // Auto-init when chat page loads
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', initParallelScanUI);
    } else {
        initParallelScanUI();
    }
})();
