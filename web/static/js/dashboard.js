// 仪表盘页面：拉取运行中任务、漏洞统计并渲染

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

    if (typeof apiFetch === 'undefined') {
        if (runningEl) runningEl.textContent = '-';
        if (vulnTotalEl) vulnTotalEl.textContent = '-';
        return;
    }

    try {
        const [tasksRes, vulnRes] = await Promise.all([
            apiFetch('/api/agent-loop/tasks').then(r => r.ok ? r.json() : null).catch(() => null),
            apiFetch('/api/vulnerabilities/stats').then(r => r.ok ? r.json() : null).catch(() => null)
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
    } catch (e) {
        console.warn('仪表盘拉取统计失败', e);
        if (runningEl) runningEl.textContent = '-';
        if (vulnTotalEl) vulnTotalEl.textContent = '-';
    }
}
