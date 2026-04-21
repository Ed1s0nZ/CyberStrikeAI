// dashboardpage:fetchrunningintask,vulnerabilitystatistics,batchtask,tooland Skills statisticsrender

async function refreshDashboard() {
    const runningEl = document.getElementById('dashboard-running-tasks');
    const vulnTotalEl = document.getElementById('dashboard-vuln-total');
    const severityIds = ['critical', 'high', 'medium', 'low', 'info'];

    if (runningEl) runningEl.textContent = '...';
    if (vulnTotalEl) vulnTotalEl.textContent = '...';
    severityIds.forEach(s => {
        const el = document.getElementById('dashboard-severity-' + s);
        if (el) el.textContent = '0';
        const barEl = document.getElementById('dashboard-bar-' + s);
        if (barEl) barEl.style.width = '0%';
    });
    setDashboardOverviewPlaceholder('...');
    setEl('dashboard-kpi-tools-calls', '...');
    setEl('dashboard-kpi-success-rate', '...');
    var chartPlaceholder = document.getElementById('dashboard-tools-pie-placeholder');
    if (chartPlaceholder) { chartPlaceholder.style.removeProperty('display'); chartPlaceholder.textContent = (typeof window.t === 'function' ? window.t('common.loading') : 'Loading...'); }
    var barChartEl = document.getElementById('dashboard-tools-bar-chart');
    if (barChartEl) { barChartEl.style.display = 'none'; barChartEl.innerHTML = ''; }

    if (typeof apiFetch === 'undefined') {
        if (runningEl) runningEl.textContent = '-';
        if (vulnTotalEl) vulnTotalEl.textContent = '-';
        setDashboardOverviewPlaceholder('-');
        return;
    }

    try {
        // Load model health status in parallel (fire-and-forget, doesn't block dashboard)
        loadModelStatus();

        const [tasksRes, vulnRes, batchRes, monitorRes, knowledgeRes, skillsRes] = await Promise.all([
            apiFetch('/api/agent-loop/tasks').then(r => r.ok ? r.json() : null).catch(() => null),
            apiFetch('/api/vulnerabilities/stats').then(r => r.ok ? r.json() : null).catch(() => null),
            apiFetch('/api/batch-tasks?limit=500&page=1').then(r => r.ok ? r.json() : null).catch(() => null),
            apiFetch('/api/monitor/stats').then(r => r.ok ? r.json() : null).catch(() => null),
            apiFetch('/api/knowledge/stats').then(r => r.ok ? r.json() : null).catch(() => null),
            apiFetch('/api/skills/stats').then(r => r.ok ? r.json() : null).catch(() => null)
        ]);

        // runningintask:Agent looptask + batchqueue"executing"countunifiedstatistics,avoidtop KPI andrunningoverviewnotconsistent
        let agentRunningCount = null;
        if (tasksRes && Array.isArray(tasksRes.tasks)) {
            agentRunningCount = tasksRes.tasks.length;
        }
        let batchRunningCount = 0;
        if (batchRes && Array.isArray(batchRes.queues)) {
            batchRes.queues.forEach(q => {
                if ((q.status || '').toLowerCase() === 'running') batchRunningCount++;
            });
        }
        if (runningEl) {
            if (agentRunningCount !== null) {
                runningEl.textContent = String(agentRunningCount + batchRunningCount);
            } else if (batchRes && Array.isArray(batchRes.queues)) {
                runningEl.textContent = String(batchRunningCount);
            } else {
                runningEl.textContent = '-';
            }
        }

        if (vulnRes && typeof vulnRes.total === 'number') {
            if (vulnTotalEl) vulnTotalEl.textContent = String(vulnRes.total);
            const bySeverity = vulnRes.by_severity || {};
            const total = vulnRes.total || 0;
            severityIds.forEach(sev => {
                const count = bySeverity[sev] || 0;
                const el = document.getElementById('dashboard-severity-' + sev);
                if (el) el.textContent = String(count);
                const barEl = document.getElementById('dashboard-bar-' + sev);
                if (barEl) barEl.style.width = total > 0 ? (count / total * 100) + '%' : '0%';
            });
        } else {
            if (vulnTotalEl) vulnTotalEl.textContent = '-';
            severityIds.forEach(sev => {
                const barEl = document.getElementById('dashboard-bar-' + sev);
                if (barEl) barEl.style.width = '0%';
            });
        }

 // batchtaskqueue:bystatusstatistics(optimized;running andon batchRunningCount consistent)
        if (batchRes && Array.isArray(batchRes.queues)) {
            const queues = batchRes.queues;
            let pending = 0, running = batchRunningCount, done = 0;
            queues.forEach(q => {
                const s = (q.status || '').toLowerCase();
                if (s === 'pending' || s === 'paused') pending++;
                else if (s === 'running') { /* already counted into batchRunningCount */ }
                else if (s === 'completed' || s === 'cancelled') done++;
            });
            const total = pending + running + done;
            setEl('dashboard-batch-pending', String(pending));
            setEl('dashboard-batch-running', String(running));
            setEl('dashboard-batch-done', String(done));
            setEl('dashboard-batch-total', total > 0 ? (typeof window.t === 'function' ? window.t('dashboard.totalCount', { count: total }) : `total ${total} `) : (typeof window.t === 'function' ? window.t('dashboard.noTasks') : 'No tasks'));
            
            // updateprogressrecord
            if (total > 0) {
                const pendingPct = (pending / total * 100).toFixed(1);
                const runningPct = (running / total * 100).toFixed(1);
                const donePct = (done / total * 100).toFixed(1);
                updateProgressBar('dashboard-batch-progress-pending', pendingPct);
                updateProgressBar('dashboard-batch-progress-running', runningPct);
                updateProgressBar('dashboard-batch-progress-done', donePct);
            } else {
                updateProgressBar('dashboard-batch-progress-pending', '0');
                updateProgressBar('dashboard-batch-progress-running', '0');
                updateProgressBar('dashboard-batch-progress-done', '0');
            }
        } else {
            setEl('dashboard-batch-pending', '-');
            setEl('dashboard-batch-running', '-');
            setEl('dashboard-batch-done', '-');
            setEl('dashboard-batch-total', '-');
            updateProgressBar('dashboard-batch-progress-pending', '0');
            updateProgressBar('dashboard-batch-progress-running', '0');
            updateProgressBar('dashboard-batch-progress-done', '0');
        }

        // tool call:monitor/stats as { toolName: { totalCalls, successCalls, failedCalls, ... } }(optimized)
        if (monitorRes && typeof monitorRes === 'object') {
            const names = Object.keys(monitorRes);
            let totalCalls = 0, totalSuccess = 0, totalFailed = 0;
            names.forEach(k => {
                const v = monitorRes[k];
                const n = v && (v.totalCalls ?? v.TotalCalls);
                if (typeof n === 'number') totalCalls += n;
                const s = v && (v.successCalls ?? v.SuccessCalls);
                if (typeof s === 'number') totalSuccess += s;
                const f = v && (v.failedCalls ?? v.FailedCalls);
                if (typeof f === 'number') totalFailed += f;
            });
            setEl('dashboard-tools-count', String(names.length));
            setEl('dashboard-tools-calls', formatNumber(totalCalls));
            setEl('dashboard-kpi-tools-calls', String(totalCalls));
            var rateStr = totalCalls > 0 ? ((totalSuccess / totalCalls) * 100).toFixed(1) + '%' : '-';
            setEl('dashboard-kpi-success-rate', rateStr);
            setEl('dashboard-tools-success-rate', rateStr !== '-' ? `Success Rate ${rateStr}` : '-');
            renderDashboardToolsBar(monitorRes);
        } else {
            setEl('dashboard-tools-count', '-');
            setEl('dashboard-tools-calls', '-');
            setEl('dashboard-kpi-tools-calls', '-');
            setEl('dashboard-kpi-success-rate', '-');
            setEl('dashboard-tools-success-rate', '-');
            renderDashboardToolsBar(null);
        }

        // knowledge:{ enabled, total_categories, total_items, ... }(optimized)
        const knowledgeItemsEl = document.getElementById('dashboard-knowledge-items');
        const knowledgeCategoriesEl = document.getElementById('dashboard-knowledge-categories');
        const knowledgeStatusEl = document.getElementById('dashboard-knowledge-status');
        if (knowledgeRes && typeof knowledgeRes === 'object') {
            if (knowledgeRes.enabled === false) {
                // feature not enabled:usestatustabdisplay,valuemaintainas "-"
                if (knowledgeStatusEl) knowledgeStatusEl.textContent = (typeof window.t === 'function' ? window.t('dashboard.notEnabled') : 'Not enabled');
                if (knowledgeItemsEl) knowledgeItemsEl.textContent = '-';
                if (knowledgeCategoriesEl) knowledgeCategoriesEl.textContent = '-';
            } else {
                const categories = knowledgeRes.total_categories ?? 0;
                const items = knowledgeRes.total_items ?? 0;
                if (knowledgeItemsEl) knowledgeItemsEl.textContent = formatNumber(items);
                if (knowledgeCategoriesEl) knowledgeCategoriesEl.textContent = formatNumber(categories);
 // based ondatatolightweightstatustext
                if (knowledgeStatusEl) {
                    if (items > 0 || categories > 0) {
                        knowledgeStatusEl.textContent = (typeof window.t === 'function' ? window.t('dashboard.enabled') : 'Enabled');
                    } else {
                        knowledgeStatusEl.textContent = (typeof window.t === 'function' ? window.t('dashboard.toConfigure') : 'To configure');
                    }
                }
            }
        } else {
            if (knowledgeItemsEl) knowledgeItemsEl.textContent = '-';
            if (knowledgeCategoriesEl) knowledgeCategoriesEl.textContent = '-';
            if (knowledgeStatusEl) knowledgeStatusEl.textContent = '-';
        }

        // Skills:{ total_skills, total_calls, ... }(optimized)
        if (skillsRes && typeof skillsRes === 'object') {
            const totalSkills = skillsRes.total_skills ?? 0;
            const totalCalls = skillsRes.total_calls ?? 0;
            setEl('dashboard-skills-count', formatNumber(totalSkills));
            setEl('dashboard-skills-calls', formatNumber(totalCalls));
            
            // settingsstatustab
            const statusEl = document.getElementById('dashboard-skills-status');
            if (statusEl) {
                if (totalCalls === 0) {
                    statusEl.textContent = (typeof window.t === 'function' ? window.t('dashboard.toUse') : 'To use');
                    statusEl.style.background = 'rgba(0, 0, 0, 0.05)';
                    statusEl.style.color = 'var(--text-secondary)';
                } else if (totalCalls < 10) {
                    statusEl.textContent = (typeof window.t === 'function' ? window.t('dashboard.active') : 'Active');
                    statusEl.style.background = 'rgba(16, 185, 129, 0.1)';
                    statusEl.style.color = '#10b981';
                } else {
                    statusEl.textContent = (typeof window.t === 'function' ? window.t('dashboard.highFreq') : 'High freq');
                    statusEl.style.background = 'rgba(59, 130, 246, 0.1)';
                    statusEl.style.color = '#3b82f6';
                }
            }
        } else {
            setEl('dashboard-skills-count', '-');
            setEl('dashboard-skills-calls', '-');
            const statusEl = document.getElementById('dashboard-skills-status');
            if (statusEl) statusEl.textContent = '-';
        }
    } catch (e) {
        console.warn('Dashboard failed to fetch statistics', e);
        if (runningEl) runningEl.textContent = '-';
        if (vulnTotalEl) vulnTotalEl.textContent = '-';
        setDashboardOverviewPlaceholder('-');
        setEl('dashboard-kpi-success-rate', '-');
        setEl('dashboard-kpi-tools-calls', '-');
        renderDashboardToolsBar(null);
        var ph = document.getElementById('dashboard-tools-pie-placeholder');
        if (ph) { ph.style.removeProperty('display'); ph.textContent = (typeof window.t === 'function' ? window.t('dashboard.noCallData') : 'No call data'); }
    }
}

function setEl(id, text) {
    const el = document.getElementById(id);
    if (el) el.textContent = text;
}

function setDashboardOverviewPlaceholder(t) {
    ['dashboard-batch-pending', 'dashboard-batch-running', 'dashboard-batch-done', 'dashboard-batch-total',
     'dashboard-tools-count', 'dashboard-tools-calls', 'dashboard-tools-success-rate',
     'dashboard-skills-count', 'dashboard-skills-calls', 'dashboard-skills-status',
     'dashboard-knowledge-items', 'dashboard-knowledge-categories', 'dashboard-knowledge-status'].forEach(id => setEl(id, t));
    updateProgressBar('dashboard-batch-progress-pending', '0');
    updateProgressBar('dashboard-batch-progress-running', '0');
    updateProgressBar('dashboard-batch-progress-done', '0');
}

// format,addthousandsseparator
function formatNumber(num) {
    if (typeof num !== 'number' || isNaN(num)) return '-';
    if (num === 0) return '0';
    return num.toLocaleString('zh-CN');
}

// updateprogressrecordwidth
function updateProgressBar(id, percentage) {
    const el = document.getElementById(id);
    if (el) {
        const pct = parseFloat(percentage) || 0;
        el.style.width = Math.max(0, Math.min(100, pct)) + '%';
    }
}

// Top 30 toollinetimebar chartcolor(30 notrepeat,soft,distinguish)
var DASHBOARD_BAR_COLORS = [
    '#93c5fd', '#a78bfa', '#6ee7b7', '#fde047', '#fda4af',
    '#7dd3fc', '#a5b4fc', '#5eead4', '#fdba74', '#e9d5ff',
    '#67e8f9', '#c4b5fd', '#86efac', '#fcd34d', '#f9a8d4',
    '#bae6fd', '#c7d2fe', '#99f6e4', '#fed7aa', '#ddd6fe',
    '#22d3ee', '#8b5cf6', '#4ade80', '#fbbf24', '#fb7185',
    '#38bdf8', '#818cf8', '#2dd4bf', '#fb923c', '#e0e7ff'
];

function esc(s) {
    if (typeof s !== 'string') return '';
    return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/"/g, '&quot;');
}

function renderDashboardToolsBar(monitorRes) {
    const placeholder = document.getElementById('dashboard-tools-pie-placeholder');
    const barChartEl = document.getElementById('dashboard-tools-bar-chart');
    if (!placeholder || !barChartEl) return;

    if (!monitorRes || typeof monitorRes !== 'object') {
        placeholder.style.removeProperty('display');
        placeholder.textContent = (typeof window.t === 'function' ? window.t('dashboard.noCallData') : 'No call data');
        barChartEl.style.display = 'none';
        barChartEl.innerHTML = '';
        return;
    }

    const entries = Object.keys(monitorRes).map(function (k) {
        const v = monitorRes[k];
        const totalCalls = v && (v.totalCalls ?? v.TotalCalls);
        return { name: k, totalCalls: typeof totalCalls === 'number' ? totalCalls : 0 };
    }).filter(function (e) { return e.totalCalls > 0; })
        .sort(function (a, b) { return b.totalCalls - a.totalCalls; })
        .slice(0, 30);

    if (entries.length === 0) {
        placeholder.style.removeProperty('display');
        placeholder.textContent = (typeof window.t === 'function' ? window.t('dashboard.noCallData') : 'No call data');
        barChartEl.style.display = 'none';
        barChartEl.innerHTML = '';
        return;
    }

    placeholder.style.display = 'none';
    barChartEl.style.display = 'block';

    const maxCalls = Math.max.apply(null, entries.map(function (e) { return e.totalCalls; }));
    var html = '';
    entries.forEach(function (e, i) {
        var pct = maxCalls > 0 ? (e.totalCalls / maxCalls) * 100 : 0;
        var label = e.name.length > 12 ? e.name.slice(0, 10) + '...' : e.name;
        var color = DASHBOARD_BAR_COLORS[i % DASHBOARD_BAR_COLORS.length];
        var fullName = esc(e.name);
        html += '<div class="dashboard-tools-bar-item" data-tooltip="' + fullName + '">';
        html += '<span class="dashboard-tools-bar-label">' + esc(label) + '</span>';
        html += '<div class="dashboard-tools-bar-track"><div class="dashboard-tools-bar-fill" style="width:' + pct + '%;background:' + color + '"></div></div>';
        html += '<span class="dashboard-tools-bar-value">' + e.totalCalls + '</span>';
        html += '</div>';
    });
    barChartEl.innerHTML = html;
    attachDashboardBarTooltips(barChartEl);
}

var dashboardBarTooltipEl = null;
var dashboardBarTooltipTimer = null;

function attachDashboardBarTooltips(barChartEl) {
    if (!barChartEl) return;
    if (!dashboardBarTooltipEl) {
        dashboardBarTooltipEl = document.createElement('div');
        dashboardBarTooltipEl.className = 'dashboard-tools-bar-tooltip';
        dashboardBarTooltipEl.setAttribute('role', 'tooltip');
        document.body.appendChild(dashboardBarTooltipEl);
    }
    barChartEl.removeEventListener('mouseover', dashboardBarTooltipOnOver);
    barChartEl.removeEventListener('mouseout', dashboardBarTooltipOnOut);
    barChartEl.addEventListener('mouseover', dashboardBarTooltipOnOver);
    barChartEl.addEventListener('mouseout', dashboardBarTooltipOnOut);
}

function dashboardBarTooltipOnOver(ev) {
    var item = ev.target && ev.target.closest && ev.target.closest('.dashboard-tools-bar-item');
    if (!item || !dashboardBarTooltipEl) return;
    var text = item.getAttribute('data-tooltip');
    if (!text) return;
    clearTimeout(dashboardBarTooltipTimer);
    dashboardBarTooltipTimer = setTimeout(function () {
        dashboardBarTooltipEl.textContent = text;
        dashboardBarTooltipEl.style.display = 'block';
        requestAnimationFrame(function () {
            var rect = item.getBoundingClientRect();
            var ttRect = dashboardBarTooltipEl.getBoundingClientRect();
            var x = rect.left + (rect.width / 2) - (ttRect.width / 2);
            var y = rect.top - ttRect.height - 6;
            if (y < 8) y = rect.bottom + 6;
            var pad = 8;
            if (x < pad) x = pad;
            if (x + ttRect.width > window.innerWidth - pad) x = window.innerWidth - ttRect.width - pad;
            dashboardBarTooltipEl.style.left = x + 'px';
            dashboardBarTooltipEl.style.top = y + 'px';
        });
    }, 180);
}

function dashboardBarTooltipOnOut(ev) {
    var item = ev.target && ev.target.closest && ev.target.closest('.dashboard-tools-bar-item');
    var related = ev.relatedTarget && ev.relatedTarget.closest && ev.relatedTarget.closest('.dashboard-tools-bar-item');
    if (item && item === related) return;
    clearTimeout(dashboardBarTooltipTimer);
    dashboardBarTooltipTimer = null;
    if (dashboardBarTooltipEl) dashboardBarTooltipEl.style.display = 'none';
}

// ---- Model health status on dashboard ----
async function loadModelStatus() {
    try {
        var resp = await apiFetch('/api/health/model');
        var data = await resp.json();
        var el = document.getElementById('dashboard-model-status-value');
        if (!el) return;
        if (data.status === 'ok') {
            el.innerHTML = '<span style="color:#22c55e;">\u25cf Connected</span> \u2014 ' +
                (data.provider || '') + ' / ' + (data.model || '') +
                ' (' + (data.latency_ms || '?') + 'ms)';
        } else if (data.status === 'unconfigured') {
            el.innerHTML = '<span style="color:#f59e0b;">\u25cf Not Configured</span> \u2014 ' +
                '<a href="#" onclick="switchPage(\'settings\');return false;" style="color:#4f8ef7;">Set API Key</a>';
        } else {
            el.innerHTML = '<span style="color:#ef4444;">\u25cf Error</span> \u2014 ' +
                (data.message || 'Connection failed');
        }
    } catch (e) {
        var el = document.getElementById('dashboard-model-status-value');
        if (el) el.textContent = 'Check failed';
    }
}
