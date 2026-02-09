// 仪表盘页面：拉取运行中任务、漏洞统计、批量任务、工具与 Skills 统计并渲染

async function refreshDashboard() {
    const runningEl = document.getElementById('dashboard-running-tasks');
    const vulnTotalEl = document.getElementById('dashboard-vuln-total');
    const severityIds = ['critical', 'high', 'medium', 'low', 'info'];

    if (runningEl) runningEl.textContent = '…';
    if (vulnTotalEl) vulnTotalEl.textContent = '…';
    severityIds.forEach(s => {
        const el = document.getElementById('dashboard-severity-' + s);
        if (el) el.textContent = '0';
        const barEl = document.getElementById('dashboard-bar-' + s);
        if (barEl) barEl.style.width = '0%';
    });
    setDashboardOverviewPlaceholder('…');
    setEl('dashboard-kpi-tools-calls', '…');
    setEl('dashboard-kpi-success-rate', '…');
    var chartPlaceholder = document.getElementById('dashboard-tools-pie-placeholder');
    if (chartPlaceholder) { chartPlaceholder.style.display = 'block'; chartPlaceholder.textContent = '加载中…'; }
    var barChartEl = document.getElementById('dashboard-tools-bar-chart');
    if (barChartEl) { barChartEl.style.display = 'none'; barChartEl.innerHTML = ''; }

    if (typeof apiFetch === 'undefined') {
        if (runningEl) runningEl.textContent = '-';
        if (vulnTotalEl) vulnTotalEl.textContent = '-';
        setDashboardOverviewPlaceholder('-');
        return;
    }

    try {
        const [tasksRes, vulnRes, batchRes, monitorRes, knowledgeRes, skillsRes] = await Promise.all([
            apiFetch('/api/agent-loop/tasks').then(r => r.ok ? r.json() : null).catch(() => null),
            apiFetch('/api/vulnerabilities/stats').then(r => r.ok ? r.json() : null).catch(() => null),
            apiFetch('/api/batch-tasks?limit=500&page=1').then(r => r.ok ? r.json() : null).catch(() => null),
            apiFetch('/api/monitor/stats').then(r => r.ok ? r.json() : null).catch(() => null),
            apiFetch('/api/knowledge/stats').then(r => r.ok ? r.json() : null).catch(() => null),
            apiFetch('/api/skills/stats').then(r => r.ok ? r.json() : null).catch(() => null)
        ]);

        if (tasksRes && Array.isArray(tasksRes.tasks)) {
            if (runningEl) runningEl.textContent = String(tasksRes.tasks.length);
        } else {
            if (runningEl) runningEl.textContent = '-';
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

        // 批量任务队列：按状态统计
        if (batchRes && Array.isArray(batchRes.queues)) {
            const queues = batchRes.queues;
            let pending = 0, running = 0, done = 0;
            queues.forEach(q => {
                const s = (q.status || '').toLowerCase();
                if (s === 'pending' || s === 'paused') pending++;
                else if (s === 'running') running++;
                else if (s === 'completed' || s === 'cancelled') done++;
            });
            setEl('dashboard-batch-pending', String(pending));
            setEl('dashboard-batch-running', String(running));
            setEl('dashboard-batch-done', String(done));
        } else {
            setEl('dashboard-batch-pending', '-');
            setEl('dashboard-batch-running', '-');
            setEl('dashboard-batch-done', '-');
        }

        // 工具调用：monitor/stats 为 { toolName: { totalCalls, successCalls, failedCalls, ... } }
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
            setEl('dashboard-tools-calls', String(totalCalls));
            setEl('dashboard-kpi-tools-calls', String(totalCalls));
            var rateStr = totalCalls > 0 ? ((totalSuccess / totalCalls) * 100).toFixed(1) + '%' : '-';
            setEl('dashboard-kpi-success-rate', rateStr);
            renderDashboardToolsBar(monitorRes);
        } else {
            setEl('dashboard-tools-count', '-');
            setEl('dashboard-tools-calls', '-');
            setEl('dashboard-kpi-tools-calls', '-');
            setEl('dashboard-kpi-success-rate', '-');
            renderDashboardToolsBar(null);
        }

        // 知识：{ enabled, total_categories, total_items, ... }
        const knowledgeValueEl = document.getElementById('dashboard-knowledge-value');
        if (knowledgeRes && typeof knowledgeRes === 'object') {
            if (knowledgeRes.enabled === false) {
                if (knowledgeValueEl) knowledgeValueEl.textContent = '知识功能暂未启用';
            } else {
                const categories = knowledgeRes.total_categories ?? 0;
                const items = knowledgeRes.total_items ?? 0;
                if (knowledgeValueEl) knowledgeValueEl.textContent = `${categories} 个分类，共 ${items} 项`;
            }
        } else {
            if (knowledgeValueEl) knowledgeValueEl.textContent = '-';
        }

        // Skills：{ total_skills, total_calls, ... }
        if (skillsRes && typeof skillsRes === 'object') {
            setEl('dashboard-skills-count', String(skillsRes.total_skills ?? '-'));
            setEl('dashboard-skills-calls', String(skillsRes.total_calls ?? '-'));
        } else {
            setEl('dashboard-skills-count', '-');
            setEl('dashboard-skills-calls', '-');
        }
    } catch (e) {
        console.warn('仪表盘拉取统计失败', e);
        if (runningEl) runningEl.textContent = '-';
        if (vulnTotalEl) vulnTotalEl.textContent = '-';
        setDashboardOverviewPlaceholder('-');
        setEl('dashboard-kpi-success-rate', '-');
        setEl('dashboard-kpi-tools-calls', '-');
        renderDashboardToolsBar(null);
        var ph = document.getElementById('dashboard-tools-pie-placeholder');
        if (ph) { ph.style.display = 'block'; ph.textContent = '暂无调用数据'; }
    }
}

function setEl(id, text) {
    const el = document.getElementById(id);
    if (el) el.textContent = text;
}

function setDashboardOverviewPlaceholder(t) {
    ['dashboard-batch-pending', 'dashboard-batch-running', 'dashboard-batch-done',
     'dashboard-tools-count', 'dashboard-tools-calls', 'dashboard-skills-count', 'dashboard-skills-calls'].forEach(id => setEl(id, t));
    const knowledgeValueEl = document.getElementById('dashboard-knowledge-value');
    if (knowledgeValueEl) knowledgeValueEl.textContent = t;
}

// Top 30 工具执行次数柱状图颜色（30 色不重复，柔和、易区分）
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
        placeholder.style.display = 'block';
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
        placeholder.style.display = 'block';
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
        var label = e.name.length > 12 ? e.name.slice(0, 10) + '…' : e.name;
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
