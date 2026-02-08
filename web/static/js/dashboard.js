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

    if (typeof apiFetch === 'undefined') {
        if (runningEl) runningEl.textContent = '-';
        if (vulnTotalEl) vulnTotalEl.textContent = '-';
        setDashboardOverviewPlaceholder('-');
        return;
    }

    try {
        const [tasksRes, vulnRes, batchRes, monitorRes, skillsRes] = await Promise.all([
            apiFetch('/api/agent-loop/tasks').then(r => r.ok ? r.json() : null).catch(() => null),
            apiFetch('/api/vulnerabilities/stats').then(r => r.ok ? r.json() : null).catch(() => null),
            apiFetch('/api/batch-tasks?limit=500&page=1').then(r => r.ok ? r.json() : null).catch(() => null),
            apiFetch('/api/monitor/stats').then(r => r.ok ? r.json() : null).catch(() => null),
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

        // 工具调用：monitor/stats 为 { toolName: { TotalCalls, ... } }
        if (monitorRes && typeof monitorRes === 'object') {
            const names = Object.keys(monitorRes);
            let totalCalls = 0;
            names.forEach(k => {
                const v = monitorRes[k];
                const n = v && (v.totalCalls ?? v.TotalCalls);
                if (typeof n === 'number') totalCalls += n;
            });
            setEl('dashboard-tools-count', String(names.length));
            setEl('dashboard-tools-calls', String(totalCalls));
        } else {
            setEl('dashboard-tools-count', '-');
            setEl('dashboard-tools-calls', '-');
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
    }
}

function setEl(id, text) {
    const el = document.getElementById(id);
    if (el) el.textContent = text;
}

function setDashboardOverviewPlaceholder(t) {
    ['dashboard-batch-pending', 'dashboard-batch-running', 'dashboard-batch-done',
     'dashboard-tools-count', 'dashboard-tools-calls', 'dashboard-skills-count', 'dashboard-skills-calls'].forEach(id => setEl(id, t));
}
