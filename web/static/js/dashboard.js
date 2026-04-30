// 仪表盘页面：拉取运行中任务、漏洞统计、批量任务、工具与 Skills 统计并渲染。
//
// 工程基础设施：
//   - dashboardState 集中保存运行时状态（in-flight controller / 自动轮询 timer / 上次更新时间 /
//     已被本会话忽略的告警条 reasons）；
//   - 每次 refreshDashboard 入口 abort 上一个 controller，把 signal 传给所有 apiFetch，
//     避免快速连点 / 自动轮询触发 race condition；
//   - 自动轮询：startDashboardAutoRefresh() 每 60 秒拉一次；页面切走 / tab 隐藏时自动暂停，
//     再切回时立即补一次刷新（基于 lastUpdatedAt 避免无效请求）；
//   - 过期检测：updateLastUpdatedNow 记录时间戳；checkDashboardStale 每 30 秒检查，
//     超过 5 分钟未刷新则在「上次更新」徽章上加 .is-stale 类（变灰 + 显示 ⚠️）。

var DASHBOARD_POLL_INTERVAL_MS = 60 * 1000;
var DASHBOARD_STALE_THRESHOLD_MS = 5 * 60 * 1000;
var DASHBOARD_STALE_CHECK_INTERVAL_MS = 30 * 1000;

var dashboardState = {
    currentController: null,    // 当前正在进行的 fetch 的 AbortController
    pollTimer: null,            // 自动轮询的 setInterval id
    staleTimer: null,           // 过期检查的 setInterval id
    lastUpdatedAt: 0,           // 上次成功刷新的时间戳（ms）
    dismissedAlertKey: null,    // 当前会话中被用户「×」掉的告警内容指纹（同样的 reasons 不再弹）
    lastResources: null,        // 上一轮关键资源快照，用于判断是否首次有数据 / 智能 CTA
};

async function refreshDashboard() {
    const runningEl = document.getElementById('dashboard-running-tasks');
    const vulnTotalEl = document.getElementById('dashboard-vuln-total');
    const severityIds = ['critical', 'high', 'medium', 'low', 'info'];

    // severityTotalEl 在后续渲染逻辑中也被引用，必须在 loading 分支外声明
    const severityTotalEl = document.getElementById('dashboard-severity-total');

    // 体验优化：自动轮询 / 已经有数据时，不再把界面闪成「…」占位，
    // 直接在后台拉新数据并平滑替换；只有首次加载时才显示 loading 状态。
    var isInitialLoad = !dashboardState.lastUpdatedAt;
    if (isInitialLoad) {
        if (runningEl) runningEl.textContent = '…';
        if (vulnTotalEl) vulnTotalEl.textContent = '…';
        severityIds.forEach(s => {
            const el = document.getElementById('dashboard-severity-' + s);
            if (el) el.textContent = '0';
            const pctEl = document.getElementById('dashboard-severity-' + s + '-pct');
            if (pctEl) pctEl.textContent = '0%';
        });
        if (severityTotalEl) severityTotalEl.textContent = '0';
        renderSeverityDonut({}, 0);
        renderVulnStatusPanel(null, 0);
        setDashboardOverviewPlaceholder('…');
        setEl('dashboard-kpi-tools-calls', '…');
        setEl('dashboard-kpi-success-rate', '…');
        setKpiSubText('dashboard-kpi-tasks-sub-text', '…');
        setKpiSubText('dashboard-kpi-vuln-sub-text', '…');
        setKpiSubText('dashboard-kpi-tools-sub-text', '…');
        setKpiSubText('dashboard-kpi-rate-sub-text', '…');
        hideEl('dashboard-kpi-vuln-critical-badge');
        hideEl('dashboard-alert-banner');
        setRecentVulnsLoading();
        ['tools', 'skills', 'knowledge', 'roles', 'agents', 'webshell'].forEach(function (k) {
            setEl('dashboard-resource-' + k, '…');
        });
        var chartPlaceholder = document.getElementById('dashboard-tools-pie-placeholder');
        if (chartPlaceholder) { chartPlaceholder.style.removeProperty('display'); chartPlaceholder.textContent = (typeof window.t === 'function' ? window.t('common.loading') : '加载中…'); }
        var barChartEl = document.getElementById('dashboard-tools-bar-chart');
        if (barChartEl) { barChartEl.style.display = 'none'; barChartEl.innerHTML = ''; }
    }

    if (typeof apiFetch === 'undefined') {
        if (runningEl) runningEl.textContent = '-';
        if (vulnTotalEl) vulnTotalEl.textContent = '-';
        setDashboardOverviewPlaceholder('-');
        setRecentVulnsError();
        return;
    }

    // 防 race：abort 上一个仍在进行中的请求，再创建新 controller
    if (dashboardState.currentController) {
        try { dashboardState.currentController.abort(); } catch (_) { /* ignore */ }
    }
    var controller = (typeof AbortController !== 'undefined') ? new AbortController() : null;
    dashboardState.currentController = controller;
    var signal = controller ? controller.signal : undefined;

    // 统一封装：apiFetch + abort signal + 失败/取消都返回 null（不抛错），
    // 让上层可以用解构赋值平铺读取所有结果，避免一处失败导致整个 Promise.all reject
    var fetchJson = function (url) {
        return apiFetch(url, { signal: signal })
            .then(function (r) { return r && r.ok ? r.json() : null; })
            .catch(function () { return null; });
    };

    try {
        // /api/vulnerabilities/stats 只给出 by_severity 与 by_status 两个独立维度，
        // 无法得到「严重 × 待处理」的交叉计数。这里额外拉两次（limit=1，仅取 total），
        // 用真实的「待处理严重 / 待处理高危」数量驱动告警条与 KPI 副标，避免修复后仍报警。
        var openVulnQuery = function (sev) {
            return fetchJson('/api/vulnerabilities?severity=' + sev + '&status=open&limit=1');
        };
        const [
            tasksRes, vulnRes, batchRes, monitorRes, knowledgeRes, skillsRes,
            recentVulnsRes, rolesRes, agentsRes,
            openCriticalRes, openHighRes, toolsConfigRes,
            hitlPendingRes, notificationsRes, externalMcpStatsRes,
            webshellRes
        ] = await Promise.all([
            fetchJson('/api/agent-loop/tasks'),
            fetchJson('/api/vulnerabilities/stats'),
            fetchJson('/api/batch-tasks?limit=500&page=1'),
            fetchJson('/api/monitor/stats'),
            fetchJson('/api/knowledge/stats'),
            fetchJson('/api/skills/stats'),
            fetchJson('/api/vulnerabilities?limit=5&page=1'),
            fetchJson('/api/roles'),
            fetchJson('/api/multi-agent/markdown-agents'),
            openVulnQuery('critical'),
            openVulnQuery('high'),
            // 拉取 MCP 工具的「配置总数」用于「能力总览」（区别于 monitor/stats 的「有调用记录」）。
            // 仅取 total 字段，page_size=1 减少传输；total 已涵盖内部 + 外部 MCP + 直接注册的工具。
            fetchJson('/api/config/tools?page=1&page_size=1'),
            // HITL 待审批：用于「需要立即处理」告警条 + 推荐操作
            fetchJson('/api/hitl/pending'),
            // 通知摘要：since=0 拿最新一批，limit 控制大小；用于「最近事件」内联展示
            fetchJson('/api/notifications/summary?since=0&limit=20&lang=' + encodeURIComponent((window.__locale || 'zh-CN'))),
            // External MCP 健康度
            fetchJson('/api/external-mcp/stats'),
            // WebShell 已建立的连接（pentest 落地后的 foothold，对运营场景非常关键）
            fetchJson('/api/webshell/connections')
        ]);

        // 如果在 await 期间 controller 已被 abort，说明又有新刷新启动了，丢弃本次结果
        if (signal && signal.aborted) return;

        // 运行中任务：Agent 循环任务 + 批量队列「执行中」数量统一统计，避免顶部 KPI 与运行概览不一致
        let agentRunningCount = null;
        if (tasksRes && Array.isArray(tasksRes.tasks)) {
            agentRunningCount = tasksRes.tasks.length;
        }
        let batchRunningCount = 0;
        let batchPendingCount = 0;
        if (batchRes && Array.isArray(batchRes.queues)) {
            batchRes.queues.forEach(q => {
                const s = (q.status || '').toLowerCase();
                if (s === 'running') batchRunningCount++;
                else if (s === 'pending' || s === 'paused') batchPendingCount++;
            });
        }
        const totalRunning = (agentRunningCount || 0) + batchRunningCount;
        if (runningEl) {
            if (agentRunningCount !== null) {
                runningEl.textContent = String(totalRunning);
            } else if (batchRes && Array.isArray(batchRes.queues)) {
                runningEl.textContent = String(batchRunningCount);
            } else {
                runningEl.textContent = '-';
            }
        }
        // KPI 副标：N 待执行 / 全部空闲
        if (batchPendingCount > 0) {
            setKpiSubBadge('dashboard-kpi-tasks-sub-text',
                dt('dashboard.pendingCountLabel', { count: batchPendingCount }, batchPendingCount + ' 待执行'),
                'pending');
        } else if (totalRunning === 0) {
            setKpiSubBadge('dashboard-kpi-tasks-sub-text', dt('dashboard.allIdle', null, '系统空闲'), 'idle');
        } else {
            setKpiSubBadge('dashboard-kpi-tasks-sub-text', dt('dashboard.executingNow', null, '正在执行'), 'running');
        }

        // 解析「待处理」口径的真实计数（专门拉的接口）；若该接口失败则退回 by_severity
        const pickOpenCount = function (res, fallback) {
            if (res && typeof res.total === 'number') return res.total;
            return fallback;
        };

        let criticalCount = 0;
        let highCount = 0;
        let openCriticalCount = 0;
        let openHighCount = 0;
        if (vulnRes && typeof vulnRes.total === 'number') {
            if (vulnTotalEl) vulnTotalEl.textContent = String(vulnRes.total);
            const bySeverity = vulnRes.by_severity || {};
            const total = vulnRes.total || 0;
            criticalCount = bySeverity.critical || 0;
            highCount = bySeverity.high || 0;
            // 优先用专门拉的「待处理」计数；若专项接口失败，则退回 by_severity（宁可误报，不可漏报）
            openCriticalCount = pickOpenCount(openCriticalRes, criticalCount);
            openHighCount = pickOpenCount(openHighRes, highCount);
            if (severityTotalEl) severityTotalEl.textContent = String(total);
            severityIds.forEach(sev => {
                const count = bySeverity[sev] || 0;
                const el = document.getElementById('dashboard-severity-' + sev);
                if (el) el.textContent = String(count);
                const pctEl = document.getElementById('dashboard-severity-' + sev + '-pct');
                if (pctEl) {
                    const pct = total > 0 ? Math.round((count / total) * 100) : 0;
                    pctEl.textContent = pct + '%';
                }
            });
            renderSeverityDonut(bySeverity, total);
            renderVulnStatusPanel(vulnRes.by_status || {}, total);

            // 漏洞 KPI 副标：徽章/文案均使用「待处理」口径
            const critBadge = document.getElementById('dashboard-kpi-vuln-critical-badge');
            const critCountEl = document.getElementById('dashboard-kpi-vuln-critical-count');
            if (critCountEl) critCountEl.textContent = String(openCriticalCount);
            if (critBadge) critBadge.hidden = openCriticalCount === 0;
            const subTextEl = document.getElementById('dashboard-kpi-vuln-sub-text');
            if (subTextEl) {
                if (total === 0) {
                    subTextEl.textContent = dt('dashboard.allClear', null, '暂无新增风险');
                } else if (openCriticalCount === 0 && openHighCount === 0) {
                    // 高严重度全部已处置 → 给正反馈
                    subTextEl.textContent = dt('dashboard.allHandled', null, '高严重度已全部处置');
                } else if (openHighCount > 0) {
                    subTextEl.textContent = dt('dashboard.openHighCountLabel', { count: openHighCount }, '待处理高危 ' + openHighCount);
                } else {
                    subTextEl.textContent = dt('dashboard.totalCount', { count: total }, '共 ' + total + ' 个');
                }
            }
        } else {
            if (vulnTotalEl) vulnTotalEl.textContent = '-';
            if (severityTotalEl) severityTotalEl.textContent = '-';
            severityIds.forEach(sev => {
                const pctEl = document.getElementById('dashboard-severity-' + sev + '-pct');
                if (pctEl) pctEl.textContent = '-';
            });
            renderSeverityDonut({}, 0);
            renderVulnStatusPanel(null, 0);
            hideEl('dashboard-kpi-vuln-critical-badge');
            setKpiSubText('dashboard-kpi-vuln-sub-text', '-');
        }

        // 批量任务队列：按状态统计（优化版；running 与上方 batchRunningCount 一致）
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
            setEl('dashboard-batch-total', total > 0 ? (typeof window.t === 'function' ? window.t('dashboard.totalCount', { count: total }) : `共 ${total} 个`) : (typeof window.t === 'function' ? window.t('dashboard.noTasks') : '暂无任务'));
            
            // 更新进度条
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

        // 工具调用：monitor/stats 为 { toolName: { totalCalls, successCalls, failedCalls, ... } }
        let toolsCount = 0, toolsTotalCalls = 0, toolsSuccessRate = -1, toolsFailedCount = 0;
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
            toolsCount = names.length;
            toolsTotalCalls = totalCalls;
            toolsFailedCount = totalFailed;
            setEl('dashboard-kpi-tools-calls', formatNumber(totalCalls));
            setKpiSubText('dashboard-kpi-tools-sub-text',
                dt('dashboard.toolsCountLabel', { count: toolsCount }, toolsCount + ' 个工具'));
            if (totalCalls > 0) {
                toolsSuccessRate = (totalSuccess / totalCalls) * 100;
                const rateStr = toolsSuccessRate.toFixed(1) + '%';
                setEl('dashboard-kpi-success-rate', rateStr);
                setKpiRateBadge('dashboard-kpi-rate-sub-text', toolsSuccessRate, totalFailed);
            } else {
                setEl('dashboard-kpi-success-rate', '-');
                setKpiSubText('dashboard-kpi-rate-sub-text', dt('dashboard.noCallYet', null, '暂无调用'));
            }
            renderDashboardToolsBar(monitorRes);
        } else {
            setEl('dashboard-kpi-tools-calls', '-');
            setEl('dashboard-kpi-success-rate', '-');
            setKpiSubText('dashboard-kpi-tools-sub-text', '-');
            setKpiSubText('dashboard-kpi-rate-sub-text', '-');
            renderDashboardToolsBar(null);
        }

        // 「能力总览 → MCP 工具」用配置总数（包含未被调用过的工具）；专项接口失败时回落到 monitor 的 names.length
        if (toolsConfigRes && typeof toolsConfigRes.total === 'number') {
            setEl('dashboard-resource-tools', formatNumber(toolsConfigRes.total));
        } else if (toolsCount > 0) {
            setEl('dashboard-resource-tools', formatNumber(toolsCount));
        } else {
            setEl('dashboard-resource-tools', '-');
        }

        // 知识：填充能力总览中的「知识」一行
        if (knowledgeRes && typeof knowledgeRes === 'object') {
            if (knowledgeRes.enabled === false) {
                setEl('dashboard-resource-knowledge', dt('dashboard.notEnabled', null, '未启用'));
            } else {
                const items = knowledgeRes.total_items ?? 0;
                setEl('dashboard-resource-knowledge', formatNumber(items));
            }
        } else {
            setEl('dashboard-resource-knowledge', '-');
        }

        // Skills：填充能力总览中的「Skills」一行
        if (skillsRes && typeof skillsRes === 'object') {
            const totalSkills = skillsRes.total_skills ?? 0;
            setEl('dashboard-resource-skills', formatNumber(totalSkills));
        } else {
            setEl('dashboard-resource-skills', '-');
        }

        // 角色 / Agents
        if (rolesRes) {
            // /api/roles 返回 { roles: [...] } 或者数组本身
            const roles = Array.isArray(rolesRes) ? rolesRes : (rolesRes.roles || []);
            setEl('dashboard-resource-roles', formatNumber(Array.isArray(roles) ? roles.length : 0));
        } else {
            setEl('dashboard-resource-roles', '-');
        }
        if (agentsRes) {
            // /api/multi-agent/markdown-agents 返回 { agents: [...] }
            const agents = Array.isArray(agentsRes) ? agentsRes : (agentsRes.agents || []);
            setEl('dashboard-resource-agents', formatNumber(Array.isArray(agents) ? agents.length : 0));
        } else {
            setEl('dashboard-resource-agents', '-');
        }
        // WebShell 已建立的连接：/api/webshell/connections 直接返回数组（不带包裹），
        // 兼容一下 { connections: [...] } 形式以防后续接口变更
        var webshellList = null;
        if (Array.isArray(webshellRes)) webshellList = webshellRes;
        else if (webshellRes && Array.isArray(webshellRes.connections)) webshellList = webshellRes.connections;
        var webshellCount = webshellList ? webshellList.length : null;
        if (webshellCount !== null) {
            setEl('dashboard-resource-webshell', formatNumber(webshellCount));
        } else {
            setEl('dashboard-resource-webshell', '-');
        }

        // 最近漏洞列表
        renderRecentVulns(recentVulnsRes);

        // External MCP 健康度（同时拿到 down 数喂给 alert banner / 推荐操作）
        var externalMcpDown = renderExternalMcpHealth(externalMcpStatsRes);

        // HITL 待审批数量（喂给 alert banner / 推荐操作）
        var hitlPending = getHitlPendingCount(hitlPendingRes);

        // 「最近事件」内联展示（来自通知摘要，过滤掉已经被仪表盘其他位置覆盖的类型）
        renderRecentEvents(notificationsRes);

        // 关键提醒条：把所有可能的告警源（漏洞/HITL/失败率/MCP健康）合并展示
        renderDashboardAlertBanner({
            criticalCount: openCriticalCount,
            hitlPending: hitlPending,
            failedTools: toolsFailedCount,
            successRate: toolsSuccessRate,
            externalMcpDown: externalMcpDown
        });

        // 智能 CTA：有数据时隐藏「开始你的安全之旅」
        var batchTotalCount = (batchRes && Array.isArray(batchRes.queues)) ? batchRes.queues.length : 0;
        var toolsConfiguredCount = (toolsConfigRes && typeof toolsConfigRes.total === 'number')
            ? toolsConfigRes.total : 0;
        updateSmartCTA({
            totalRunning: totalRunning,
            totalVulns: (vulnRes && typeof vulnRes.total === 'number') ? vulnRes.total : 0,
            totalCalls: toolsTotalCalls,
            toolsConfigured: toolsConfiguredCount,
            batchTotal: batchTotalCount
        });

        // 「推荐操作」：基于全量当前状态智能生成
        renderRecommendedActions({
            openCriticalCount: openCriticalCount,
            hitlPending: hitlPending,
            externalMcpDown: externalMcpDown,
            successRate: toolsSuccessRate,
            failedTools: toolsFailedCount,
            toolsConfigured: toolsConfiguredCount,
            totalVulns: (vulnRes && typeof vulnRes.total === 'number') ? vulnRes.total : 0,
            totalRunning: totalRunning
        });

        // 更新「上次更新」时间
        updateLastUpdatedNow();
    } catch (e) {
        // AbortError 是预期内（被新一次刷新主动取消），不视为错误
        if (e && (e.name === 'AbortError' || (signal && signal.aborted))) return;
        console.warn('仪表盘拉取统计失败', e);
        if (runningEl) runningEl.textContent = '-';
        if (vulnTotalEl) vulnTotalEl.textContent = '-';
        setDashboardOverviewPlaceholder('-');
        setEl('dashboard-kpi-success-rate', '-');
        setEl('dashboard-kpi-tools-calls', '-');
        setKpiSubText('dashboard-kpi-tasks-sub-text', '-');
        setKpiSubText('dashboard-kpi-vuln-sub-text', '-');
        setKpiSubText('dashboard-kpi-tools-sub-text', '-');
        setKpiSubText('dashboard-kpi-rate-sub-text', '-');
        ['tools', 'skills', 'knowledge', 'roles', 'agents', 'webshell'].forEach(function (k) {
            setEl('dashboard-resource-' + k, '-');
        });
        setRecentVulnsError();
        renderDashboardToolsBar(null);
        var ph = document.getElementById('dashboard-tools-pie-placeholder');
        if (ph) { ph.style.removeProperty('display'); ph.textContent = (typeof window.t === 'function' ? window.t('dashboard.noCallData') : '暂无调用数据'); }
    } finally {
        if (dashboardState.currentController === controller) {
            dashboardState.currentController = null;
        }
        // 第一次 refreshDashboard（无论成功与否）完成后即开启自动轮询 + 过期检查；
        // 重复调用是幂等的（内部判断 timer 是否已存在）。
        startDashboardAutoRefresh();
    }
}

function setEl(id, text) {
    const el = document.getElementById(id);
    if (el) el.textContent = text;
}

function hideEl(id) {
    const el = document.getElementById(id);
    if (el) el.hidden = true;
}

function showEl(id) {
    const el = document.getElementById(id);
    if (el) el.hidden = false;
}

function setDashboardOverviewPlaceholder(text) {
    ['dashboard-batch-pending', 'dashboard-batch-running', 'dashboard-batch-done', 'dashboard-batch-total'].forEach(id => setEl(id, text));
    updateProgressBar('dashboard-batch-progress-pending', '0');
    updateProgressBar('dashboard-batch-progress-running', '0');
    updateProgressBar('dashboard-batch-progress-done', '0');
}

// 翻译辅助；找不到时回退到 fallback 字符串。
// 命名为 dt 而非 t，避免覆盖 i18n.js 暴露的 window.t（同名函数声明在脚本顶层会写入 window）
function dt(key, opts, fallback) {
    if (typeof window.t === 'function') {
        const v = window.t(key, opts);
        if (v && v !== key) return v;
    }
    return fallback != null ? fallback : key;
}

// KPI 卡片副标：纯文本
function setKpiSubText(id, text) {
    const el = document.getElementById(id);
    if (!el) return;
    el.textContent = text;
    el.classList.remove('is-pending', 'is-running', 'is-idle', 'is-warning', 'is-success', 'is-danger');
}

// KPI 卡片副标：带状态色（pending / running / idle / warning / success / danger）
function setKpiSubBadge(id, text, kind) {
    const el = document.getElementById(id);
    if (!el) return;
    el.textContent = text;
    el.classList.remove('is-pending', 'is-running', 'is-idle', 'is-warning', 'is-success', 'is-danger');
    if (kind) el.classList.add('is-' + kind);
}

// 工具成功率徽章着色
function setKpiRateBadge(id, rate, failedCount) {
    const el = document.getElementById(id);
    if (!el) return;
    el.classList.remove('is-pending', 'is-running', 'is-idle', 'is-warning', 'is-success', 'is-danger');
    if (rate >= 95) {
        el.textContent = dt('dashboard.healthyStatus', null, '运行平稳');
        el.classList.add('is-success');
    } else if (rate >= 80) {
        el.textContent = dt('dashboard.normalStatus', null, '基本正常') + (failedCount > 0 ? ' · ' + dt('dashboard.failedNCalls', { count: failedCount }, failedCount + ' 失败') : '');
        el.classList.add('is-warning');
    } else {
        el.textContent = dt('dashboard.degradedStatus', null, '需要关注') + (failedCount > 0 ? ' · ' + dt('dashboard.failedNCalls', { count: failedCount }, failedCount + ' 失败') : '');
        el.classList.add('is-danger');
    }
}

// 关键提醒条：根据严重情况渲染或隐藏。
//   - level: danger（红） > warning（橙） > info（蓝），按 reasons 自动取最高级
//   - 用户点 × 后，把当前 reasons 指纹存入 sessionStorage，本会话内再出现完全相同的内容会自动跳过
//   - 当 reasons 集合发生变化（如又新增一类问题），指纹失效，banner 重新弹出，避免「忽略后永远不再提醒」
function renderDashboardAlertBanner(stats) {
    const banner = document.getElementById('dashboard-alert-banner');
    const titleEl = document.getElementById('dashboard-alert-title');
    const descEl = document.getElementById('dashboard-alert-desc');
    const actsEl = document.getElementById('dashboard-alert-actions');
    if (!banner || !titleEl || !descEl || !actsEl) return;

    const reasons = [];
    // 用 reasonKeys 算指纹（不含本地化字符串，切语言后不会让用户重新看到）
    const reasonKeys = [];
    let level = 'info'; // info | warning | danger

    if (stats.criticalCount > 0) {
        reasons.push(dt('dashboard.alertCriticalReason', { count: stats.criticalCount },
            '存在 ' + stats.criticalCount + ' 个待处理的严重漏洞，建议立即处置'));
        reasonKeys.push('crit:' + stats.criticalCount);
        level = 'danger';
    }
    if (stats.hitlPending > 0) {
        // HITL 待审批是阻塞 Agent 流程的，独立成一条；不影响 level（除非已经是 info 升 warning）
        reasons.push(dt('dashboard.alertHitlReason', { count: stats.hitlPending },
            '有 ' + stats.hitlPending + ' 个待审批的人机协同请求，Agent 正在等待你的决策'));
        reasonKeys.push('hitl:' + stats.hitlPending);
        if (level === 'info') level = 'warning';
    }
    if (stats.successRate >= 0 && stats.successRate < 80 && stats.failedTools > 0) {
        reasons.push(dt('dashboard.alertFailedReason', { count: stats.failedTools },
            '工具调用成功率偏低（' + stats.failedTools + ' 次失败），请检查 MCP 监控'));
        reasonKeys.push('rate:' + Math.round(stats.successRate) + ':' + stats.failedTools);
        if (level === 'info') level = 'warning';
    }
    if (stats.externalMcpDown > 0) {
        // External MCP 异常服务器数 > 0：影响工具可用性
        reasons.push(dt('dashboard.alertMcpDownReason', { count: stats.externalMcpDown },
            'External MCP 服务器有 ' + stats.externalMcpDown + ' 个未运行，相关工具不可用'));
        reasonKeys.push('mcp:' + stats.externalMcpDown);
        if (level === 'info') level = 'warning';
    }

    if (reasons.length === 0) {
        banner.hidden = true;
        banner.classList.remove('is-warning', 'is-danger', 'is-info');
        dashboardState.dismissedAlertKey = null;
        return;
    }

    var fingerprint = level + '|' + reasonKeys.join(',');
    dashboardState.dismissedAlertKey = fingerprint;

    // 检查是否被本会话忽略过同样的内容
    var dismissed = null;
    try { dismissed = sessionStorage.getItem('dashboard.dismissedAlert'); } catch (_) {}
    if (dismissed === fingerprint) {
        banner.hidden = true;
        return;
    }

    banner.hidden = false;
    banner.classList.remove('is-warning', 'is-danger', 'is-info');
    banner.classList.add('is-' + level);

    if (level === 'danger') {
        titleEl.textContent = dt('dashboard.alertDangerTitle', null, '需要立即处理');
    } else if (level === 'warning') {
        titleEl.textContent = dt('dashboard.alertWarningTitle', null, '需要关注');
    } else {
        titleEl.textContent = dt('dashboard.alertTitle', null, '提醒');
    }

    descEl.textContent = reasons.join('；');

    actsEl.innerHTML = '';
    if (stats.criticalCount > 0) {
        const btn = document.createElement('button');
        btn.className = 'dashboard-alert-btn';
        btn.textContent = dt('dashboard.viewVulns', null, '查看漏洞');
        btn.onclick = function () { try { switchPage('vulnerabilities'); } catch (e) {} };
        actsEl.appendChild(btn);
    }
    if (stats.hitlPending > 0) {
        const btn = document.createElement('button');
        btn.className = 'dashboard-alert-btn dashboard-alert-btn-secondary';
        btn.textContent = dt('dashboard.viewHitl', null, '前往审批');
        btn.onclick = function () { try { switchPage('hitl'); } catch (e) {} };
        actsEl.appendChild(btn);
    }
    if (stats.successRate >= 0 && stats.successRate < 80) {
        const btn = document.createElement('button');
        btn.className = 'dashboard-alert-btn dashboard-alert-btn-secondary';
        btn.textContent = dt('dashboard.viewMonitor', null, '查看监控');
        btn.onclick = function () { try { switchPage('mcp-monitor'); } catch (e) {} };
        actsEl.appendChild(btn);
    }
    if (stats.externalMcpDown > 0) {
        const btn = document.createElement('button');
        btn.className = 'dashboard-alert-btn dashboard-alert-btn-secondary';
        btn.textContent = dt('dashboard.viewMcpManagement', null, '管理 MCP');
        btn.onclick = function () { try { switchPage('mcp-management'); } catch (e) {} };
        actsEl.appendChild(btn);
    }
}

// External MCP 健康度：从 /api/external-mcp/stats 解析出 running / total / down，
// 决定是否在「能力总览」第 6 行显示，并把 down 数返回给 alert banner 驱动告警。
function renderExternalMcpHealth(stats) {
    var row = document.getElementById('dashboard-resource-external-mcp-row');
    var textEl = document.getElementById('dashboard-resource-external-mcp-text');
    var healthEl = document.getElementById('dashboard-resource-external-mcp-health');
    if (!row || !textEl) return 0;

    if (!stats || typeof stats !== 'object') {
        row.hidden = true;
        return 0;
    }
    // 兼容多种返回字段：{ total, running, stopped/error }；常见命名都尝试一下
    var total = Number(stats.total ?? stats.Total ?? 0) || 0;
    var running = Number(stats.running ?? stats.Running ?? 0) || 0;
    if (total === 0) {
        row.hidden = true;
        return 0;
    }
    var down = Math.max(0, total - running);
    row.hidden = false;
    textEl.textContent = formatNumber(running) + ' / ' + formatNumber(total);
    if (healthEl) {
        healthEl.classList.remove('is-ok', 'is-warning', 'is-danger');
        if (down === 0) {
            healthEl.classList.add('is-ok');
            healthEl.textContent = dt('dashboard.mcpAllRunning', null, '全部运行');
        } else if (down < total) {
            healthEl.classList.add('is-warning');
            healthEl.textContent = dt('dashboard.mcpPartialDown', { count: down },
                down + ' 个未运行');
        } else {
            healthEl.classList.add('is-danger');
            healthEl.textContent = dt('dashboard.mcpAllDown', null, '全部未运行');
        }
        healthEl.hidden = false;
    }
    return down;
}

// HITL 待审批数量：返回 pending 项数；同时可在能力总览或 KPI 副标里使用
function getHitlPendingCount(res) {
    if (!res) return 0;
    if (Array.isArray(res.items)) return res.items.length;
    if (typeof res.total === 'number') return res.total;
    if (Array.isArray(res)) return res.length;
    return 0;
}

// 「最近事件」内联展示：取通知摘要里最重要的前 N 条
// 设计原则：
//   - 不重复 alert banner / KPI 已经表达过的信息（漏洞、HITL 等会被过滤掉避免冗余）
//   - 只显示 p0/p1 优先级，p2 作为兜底（当 p0/p1 不够时）
//   - 整个 section 在没有可显示内容时整个隐藏，避免空模块占地方
function renderRecentEvents(notifRes) {
    var section = document.getElementById('dashboard-section-events');
    var listEl = document.getElementById('dashboard-events-list');
    if (!section || !listEl) return;

    var items = (notifRes && Array.isArray(notifRes.items)) ? notifRes.items : [];
    // 过滤：只看有意义的事件，去掉 actionable 已处理的、以及类型已经在仪表盘其他位置覆盖的
    var coveredTypes = { 'vulnerability_created': true, 'hitl_pending': true };
    var filtered = items.filter(function (it) {
        if (!it || !it.type) return false;
        if (coveredTypes[it.type]) return false;
        return true;
    });

    // 按 level 排序：p0 > p1 > p2，再按时间倒序
    var levelOrder = { p0: 0, p1: 1, p2: 2 };
    filtered.sort(function (a, b) {
        var la = levelOrder[a.level] != null ? levelOrder[a.level] : 9;
        var lb = levelOrder[b.level] != null ? levelOrder[b.level] : 9;
        if (la !== lb) return la - lb;
        var ta = a.createdAt || a.created_at || 0;
        var tb = b.createdAt || b.created_at || 0;
        return new Date(tb).getTime() - new Date(ta).getTime();
    });

    var top = filtered.slice(0, 3);
    if (top.length === 0) {
        section.hidden = true;
        listEl.innerHTML = '';
        return;
    }
    section.hidden = false;

    listEl.innerHTML = top.map(function (it) {
        var level = it.level || 'p2';
        var title = esc(it.title || it.message || dt('dashboard.eventUntitled', null, '事件'));
        var msg = esc(it.message || it.summary || '');
        var when = esc(timeAgoStr(it.createdAt || it.created_at));
        return (
            '<div class="dashboard-event-item lvl-' + esc(level) + '">' +
            '<span class="dashboard-event-dot" aria-hidden="true"></span>' +
            '<div class="dashboard-event-body">' +
            '<div class="dashboard-event-title">' + title + '</div>' +
            (msg && msg !== title ? '<div class="dashboard-event-msg">' + msg + '</div>' : '') +
            '</div>' +
            '<span class="dashboard-event-time">' + when + '</span>' +
            '</div>'
        );
    }).join('');
}

// 推荐操作：基于当前数据状态智能生成「下一步该做什么」。
// 设计原则：每条都必须可点击直达对应页面，按优先级（紧急 > 维护 > 配置）排序，
// 同一时间只显示最重要的 3-5 条；没有可推荐时整个 section 隐藏。
function renderRecommendedActions(state) {
    var section = document.getElementById('dashboard-section-recommend');
    var listEl = document.getElementById('dashboard-recommend-list');
    if (!section || !listEl) return;

    var actions = [];

    // 紧急类：未处理严重漏洞
    if (state.openCriticalCount > 0) {
        actions.push({
            level: 'urgent',
            icon: '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><circle cx="12" cy="17" r="1" fill="currentColor" stroke="none"/></svg>',
            title: dt('dashboard.recoFixCritical', { count: state.openCriticalCount },
                '修复 ' + state.openCriticalCount + ' 个待处理严重漏洞'),
            desc: dt('dashboard.recoFixCriticalDesc', null, '严重等级的漏洞应优先处置'),
            page: 'vulnerabilities'
        });
    }
    // 紧急类：HITL 待审批
    if (state.hitlPending > 0) {
        actions.push({
            level: 'urgent',
            icon: '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M9 11l3 3L22 4"/><path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11"/></svg>',
            title: dt('dashboard.recoApproveHitl', { count: state.hitlPending },
                '审批 ' + state.hitlPending + ' 个 HITL 请求'),
            desc: dt('dashboard.recoApproveHitlDesc', null, 'Agent 正在等待你的决策才能继续'),
            page: 'hitl'
        });
    }
    // 维护类：External MCP 异常
    if (state.externalMcpDown > 0) {
        actions.push({
            level: 'warning',
            icon: '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="2" y1="12" x2="22" y2="12"/><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/></svg>',
            title: dt('dashboard.recoRestartMcp', { count: state.externalMcpDown },
                '检查 ' + state.externalMcpDown + ' 个未运行的 External MCP'),
            desc: dt('dashboard.recoRestartMcpDesc', null, '相关工具在 MCP 服务恢复前不可用'),
            page: 'mcp-management'
        });
    }
    // 维护类：高失败率
    if (state.successRate >= 0 && state.successRate < 80 && state.failedTools > 0) {
        actions.push({
            level: 'warning',
            icon: '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>',
            title: dt('dashboard.recoCheckMonitor', { count: state.failedTools },
                '排查 ' + state.failedTools + ' 次工具调用失败'),
            desc: dt('dashboard.recoCheckMonitorDesc', null, '在 MCP 监控中查看失败的请求详情'),
            page: 'mcp-monitor'
        });
    }
    // 配置类：第一次运行场景
    if (state.toolsConfigured === 0) {
        actions.push({
            level: 'setup',
            icon: '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z"/></svg>',
            title: dt('dashboard.recoSetupMcp', null, '配置首个 MCP 工具'),
            desc: dt('dashboard.recoSetupMcpDesc', null, '安装 MCP 服务后 Agent 才能调用具体能力'),
            page: 'mcp-management'
        });
    }
    if (state.totalVulns === 0 && state.totalRunning === 0 && state.toolsConfigured > 0) {
        actions.push({
            level: 'setup',
            icon: '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg>',
            title: dt('dashboard.recoStartScan', null, '开始第一次扫描'),
            desc: dt('dashboard.recoStartScanDesc', null, '在对话中描述目标，让 AI 协助执行'),
            page: 'chat'
        });
    }

    if (actions.length === 0) {
        section.hidden = true;
        listEl.innerHTML = '';
        return;
    }
    section.hidden = false;
    listEl.innerHTML = actions.slice(0, 5).map(function (a) {
        return (
            '<a class="dashboard-recommend-item lvl-' + a.level + '" data-page="' + esc(a.page) + '" role="button" tabindex="0">' +
            '<span class="dashboard-recommend-icon" aria-hidden="true">' + a.icon + '</span>' +
            '<div class="dashboard-recommend-body">' +
            '<div class="dashboard-recommend-title">' + esc(a.title) + '</div>' +
            '<div class="dashboard-recommend-desc">' + esc(a.desc) + '</div>' +
            '</div>' +
            '<span class="dashboard-recommend-arrow" aria-hidden="true">→</span>' +
            '</a>'
        );
    }).join('');

    // 委托点击/键盘到推荐项 → switchPage
    Array.from(listEl.querySelectorAll('.dashboard-recommend-item')).forEach(function (el) {
        var page = el.getAttribute('data-page');
        el.onclick = function () { try { switchPage(page); } catch (_) {} };
        el.onkeydown = function (e) {
            if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); el.click(); }
        };
    });
}

// 智能 CTA：用户已经有任何数据（任务运行 / 漏洞 / 工具调用 / 配置过 MCP）就把
// 「开始你的安全之旅」的 CTA 隐藏，只在真正空白的全新环境保留它当引导
function updateSmartCTA(state) {
    var cta = document.getElementById('dashboard-cta-block');
    if (!cta) return;
    var hasData = (
        (state.totalRunning || 0) > 0 ||
        (state.totalVulns || 0) > 0 ||
        (state.totalCalls || 0) > 0 ||
        (state.toolsConfigured || 0) > 0 ||
        (state.batchTotal || 0) > 0
    );
    cta.hidden = hasData;
}

// 「上次更新」时间显示；同时记录 lastUpdatedAt 给 stale 检查使用，并清掉 stale 状态
function updateLastUpdatedNow() {
    dashboardState.lastUpdatedAt = Date.now();
    const el = document.getElementById('dashboard-last-updated-time');
    if (!el) return;
    const d = new Date();
    const pad = function (n) { return n < 10 ? '0' + n : String(n); };
    el.textContent = pad(d.getHours()) + ':' + pad(d.getMinutes()) + ':' + pad(d.getSeconds());
    const wrap = document.getElementById('dashboard-last-updated');
    if (wrap) {
        wrap.classList.remove('is-stale');
        wrap.classList.remove('is-flash');
        // trigger reflow then add class for the flash animation
        void wrap.offsetWidth;
        wrap.classList.add('is-flash');
    }
    const stale = document.getElementById('dashboard-last-updated-stale');
    if (stale) stale.hidden = true;
}

// 数据过期检查：超过 DASHBOARD_STALE_THRESHOLD_MS 未刷新，给徽章加 .is-stale 类，
// 显示 ⚠️ 图标提示用户「这块数据可能已经过期，请手动刷新或检查网络」
function checkDashboardStale() {
    if (!dashboardState.lastUpdatedAt) return;
    var ageMs = Date.now() - dashboardState.lastUpdatedAt;
    var wrap = document.getElementById('dashboard-last-updated');
    var stale = document.getElementById('dashboard-last-updated-stale');
    if (!wrap) return;
    if (ageMs > DASHBOARD_STALE_THRESHOLD_MS) {
        wrap.classList.add('is-stale');
        if (stale) stale.hidden = false;
    } else {
        wrap.classList.remove('is-stale');
        if (stale) stale.hidden = true;
    }
}

// 自动轮询：仪表盘活跃 + tab 可见时每 60 秒静默刷新一次。
// 切走 / tab 隐藏时 setInterval 仍在跑，但 tick 内会检查并跳过实际刷新；
// 重新可见时基于 lastUpdatedAt 判断是否需要立即补刷一次（>= 间隔的一半就刷）。
function startDashboardAutoRefresh() {
    if (dashboardState.pollTimer) return;
    dashboardState.pollTimer = setInterval(function () {
        try {
            var page = document.getElementById('page-dashboard');
            if (!page || !page.classList.contains('active')) return;
            if (typeof document !== 'undefined' && document.hidden) return;
            refreshDashboard();
        } catch (e) {
            console.warn('auto refresh tick failed', e);
        }
    }, DASHBOARD_POLL_INTERVAL_MS);

    if (!dashboardState.staleTimer) {
        dashboardState.staleTimer = setInterval(checkDashboardStale, DASHBOARD_STALE_CHECK_INTERVAL_MS);
    }
}

function stopDashboardAutoRefresh() {
    if (dashboardState.pollTimer) {
        clearInterval(dashboardState.pollTimer);
        dashboardState.pollTimer = null;
    }
    if (dashboardState.staleTimer) {
        clearInterval(dashboardState.staleTimer);
        dashboardState.staleTimer = null;
    }
}

// 严重度配色及中文标签
var SEVERITY_LABELS_FALLBACK = {
    critical: '严重', high: '高危', medium: '中危', low: '低危', info: '信息'
};

function severityShortLabel(id) {
    const key = 'dashboard.severity' + id.charAt(0).toUpperCase() + id.slice(1);
    return t(key, null, SEVERITY_LABELS_FALLBACK[id] || id);
}

// 友好的相对时间："5 分钟前" / "2 小时前" / "昨天" / "3 天前"
function timeAgoStr(iso) {
    if (!iso) return '';
    const d = new Date(iso);
    if (isNaN(d.getTime())) return '';
    const diffSec = Math.max(0, Math.floor((Date.now() - d.getTime()) / 1000));
    if (diffSec < 60) return dt('common.justNow', null, '刚刚');
    const min = Math.floor(diffSec / 60);
    if (min < 60) return dt('common.minutesAgo', { n: min }, min + ' 分钟前');
    const hr = Math.floor(min / 60);
    if (hr < 24) return dt('common.hoursAgo', { n: hr }, hr + ' 小时前');
    const day = Math.floor(hr / 24);
    if (day < 7) return dt('common.daysAgo', { n: day }, day + ' 天前');
    // 超过一周显示日期
    return d.getFullYear() + '-' + String(d.getMonth() + 1).padStart(2, '0') + '-' + String(d.getDate()).padStart(2, '0');
}

// 最近漏洞列表
function setRecentVulnsLoading() {
    const wrap = document.getElementById('dashboard-recent-vulns');
    const empty = document.getElementById('dashboard-recent-vulns-empty');
    if (!wrap) return;
    Array.from(wrap.querySelectorAll('.dashboard-recent-vuln-item')).forEach(function (n) { n.remove(); });
    if (empty) {
        empty.hidden = false;
        empty.classList.remove('is-rich');
        empty.textContent = dt('common.loading', null, '加载中…');
    }
}

function setRecentVulnsError() {
    const wrap = document.getElementById('dashboard-recent-vulns');
    const empty = document.getElementById('dashboard-recent-vulns-empty');
    if (!wrap) return;
    Array.from(wrap.querySelectorAll('.dashboard-recent-vuln-item')).forEach(function (n) { n.remove(); });
    if (empty) {
        empty.hidden = false;
        empty.classList.remove('is-rich');
        empty.textContent = dt('common.loadFailed', null, '加载失败');
    }
}

function renderRecentVulns(res) {
    const wrap = document.getElementById('dashboard-recent-vulns');
    const empty = document.getElementById('dashboard-recent-vulns-empty');
    if (!wrap) return;

    Array.from(wrap.querySelectorAll('.dashboard-recent-vuln-item')).forEach(function (n) { n.remove(); });

    const list = res && Array.isArray(res.vulnerabilities) ? res.vulnerabilities : [];
    if (list.length === 0) {
        if (empty) {
            empty.hidden = false;
            // 升级版空状态：图标 + 标题 + 描述 + 行动按钮，比纯文本更易引导用户下一步
            empty.classList.add('is-rich');
            empty.innerHTML = (
                '<span class="dashboard-empty-icon" aria-hidden="true">' +
                '<svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/><path d="M9 12l2 2 4-4"/></svg>' +
                '</span>' +
                '<div class="dashboard-empty-title">' + esc(dt('dashboard.noVulnYet', null, '暂无漏洞')) + '</div>' +
                '<div class="dashboard-empty-desc">' + esc(dt('dashboard.noVulnDesc', null, '系统目前安全，开始一次扫描可以发现潜在问题')) + '</div>' +
                '<button type="button" class="dashboard-empty-action" data-action="scan">' +
                esc(dt('dashboard.startScanBtn', null, '前往对话发起扫描')) + ' →</button>'
            );
            var btn = empty.querySelector('[data-action="scan"]');
            if (btn) btn.onclick = function () { try { switchPage('chat'); } catch (_) {} };
        }
        return;
    }
    if (empty) {
        empty.hidden = true;
        empty.classList.remove('is-rich');
    }

    list.slice(0, 5).forEach(function (v) {
        const sev = (v.severity || 'info').toLowerCase();
        const status = (v.status || 'open').toLowerCase();
        const item = document.createElement('a');
        item.className = 'dashboard-recent-vuln-item';
        item.setAttribute('role', 'button');
        item.tabIndex = 0;
        item.onclick = function () { try { switchPage('vulnerabilities'); } catch (e) {} };
        item.onkeydown = function (e) { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); item.click(); } };

        const severityBadge = '<span class="dashboard-recent-vuln-sev sev-' + sev + '">' + esc(severityShortLabel(sev)) + '</span>';
        const title = '<span class="dashboard-recent-vuln-title" title="' + esc(v.title || '') + '">' + esc(v.title || dt('common.untitled', null, '无标题')) + '</span>';
        const target = v.target ? ('<span class="dashboard-recent-vuln-target" title="' + esc(v.target) + '">' + esc(v.target) + '</span>') : '<span class="dashboard-recent-vuln-target"></span>';
        const statusPill = '<span class="dashboard-recent-vuln-status st-' + esc(statusKey(status)) + '"><span class="dashboard-recent-vuln-status-dot"></span>' + esc(statusShortLabel(status)) + '</span>';
        const time = '<span class="dashboard-recent-vuln-time">' + esc(timeAgoStr(v.created_at)) + '</span>';

        item.innerHTML = severityBadge + title + target + statusPill + time;
        wrap.appendChild(item);
    });
}

// 漏洞状态映射：把 status 字符串规整到 4 类（避免脏数据）
function statusKey(s) {
    s = String(s || '').toLowerCase();
    if (s === 'fixed' || s === 'closed' || s === 'resolved') return 'fixed';
    if (s === 'confirmed') return 'confirmed';
    if (s === 'false_positive' || s === 'false-positive' || s === 'fp') return 'fp';
    return 'open';
}

function statusShortLabel(s) {
    const k = statusKey(s);
    if (k === 'fixed') return dt('dashboard.statusFixed', null, '已修复');
    if (k === 'confirmed') return dt('dashboard.statusConfirmed', null, '已确认');
    if (k === 'fp') return dt('dashboard.statusFalsePositive', null, '误报');
    return dt('dashboard.statusOpen', null, '待处理');
}

// 格式化数字，添加千位分隔符
function formatNumber(num) {
    if (typeof num !== 'number' || isNaN(num)) return '-';
    if (num === 0) return '0';
    return num.toLocaleString('zh-CN');
}

// 更新进度条宽度
function updateProgressBar(id, percentage) {
    const el = document.getElementById(id);
    if (el) {
        const pct = parseFloat(percentage) || 0;
        el.style.width = Math.max(0, Math.min(100, pct)) + '%';
    }
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

// 漏洞处置状态 + 修复进度面板
// byStatus: { open, confirmed, fixed, false_positive }（任一字段缺失视作 0）
// total: 漏洞总数（来自 stats.total）
function renderVulnStatusPanel(byStatus, total) {
    var get = function (k) {
        if (!byStatus || typeof byStatus !== 'object') return 0;
        return Number(byStatus[k] || 0) || 0;
    };
    var open = get('open');
    var confirmed = get('confirmed');
    var fixed = get('fixed');
    var fp = get('false_positive');

    setEl('dashboard-status-open', formatNumber(open));
    setEl('dashboard-status-confirmed', formatNumber(confirmed));
    setEl('dashboard-status-fixed', formatNumber(fixed));
    setEl('dashboard-status-fp', formatNumber(fp));

    // 修复率：fixed / total（不计入 false_positive 时也可，按 total 维持一致）
    var t = Number(total || 0);
    var rate = t > 0 ? (fixed / t) * 100 : 0;
    var rateStr = t > 0 ? rate.toFixed(rate >= 100 ? 0 : 1) + '%' : '-';
    setEl('dashboard-fix-rate', rateStr);

    var detailEl = document.getElementById('dashboard-fix-detail');
    if (detailEl) {
        detailEl.textContent = '(' + formatNumber(fixed) + ' / ' + formatNumber(t) + ')';
    }

    var fixedPct = t > 0 ? (fixed / t) * 100 : 0;
    var confirmedPct = t > 0 ? (confirmed / t) * 100 : 0;
    var fixedBar = document.getElementById('dashboard-fix-progress-fixed');
    var confirmedBar = document.getElementById('dashboard-fix-progress-confirmed');
    if (fixedBar) fixedBar.style.width = fixedPct.toFixed(2) + '%';
    if (confirmedBar) confirmedBar.style.width = confirmedPct.toFixed(2) + '%';
}

function renderDashboardToolsBar(monitorRes) {
    const placeholder = document.getElementById('dashboard-tools-pie-placeholder');
    const barChartEl = document.getElementById('dashboard-tools-bar-chart');
    if (!placeholder || !barChartEl) return;

    if (!monitorRes || typeof monitorRes !== 'object') {
        placeholder.style.removeProperty('display');
        placeholder.textContent = (typeof window.t === 'function' ? window.t('dashboard.noCallData') : '暂无调用数据');
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
        placeholder.textContent = (typeof window.t === 'function' ? window.t('dashboard.noCallData') : '暂无调用数据');
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

// 漏洞严重程度分布：半环形（donut）渲染
// 几何参数固定，便于配合 viewBox 0 0 560 320 的 SVG 容器
// 段间分隔由 CSS 的白色 stroke 完成，不再使用 gapRad
var SEVERITY_DONUT_CFG = {
    // viewBox 0 0 480 260：整体保持紧凑，但环厚回到「黄金比例」附近，
    // 让弧带本身有视觉分量，又不像最早那版那样占太多空间。
    // 原则：rInner / rOuter ≈ 0.70，ring thickness ≈ rOuter * 0.30。
    cx: 240,
    cy: 215,
    rOuter: 165,
    rInner: 115,    // 环厚 = 50（介于原 90 和上一版 35 之间，自然且有质感）
    labelOffset: 14,
    gapRad: 0
};

var SEVERITY_DEFAULT_LABELS = {
    critical: '严重',
    high: '高危',
    medium: '中危',
    low: '低危',
    info: '信息'
};

function severityLabel(id) {
    var key = 'dashboard.severity' + id.charAt(0).toUpperCase() + id.slice(1);
    if (typeof window.t === 'function') {
        var v = window.t(key);
        if (v && v !== key) return v;
    }
    return SEVERITY_DEFAULT_LABELS[id] || id;
}

function renderSeverityDonut(bySeverity, total) {
    var trackEl = document.getElementById('dashboard-severity-donut-track');
    var segmentsEl = document.getElementById('dashboard-severity-donut-segments');
    var labelsEl = document.getElementById('dashboard-severity-donut-labels');
    if (!trackEl || !segmentsEl || !labelsEl) return;

    var cfg = SEVERITY_DONUT_CFG;

    // 背景轨迹（完整半环）只渲染一次
    if (!trackEl.hasChildNodes()) {
        trackEl.innerHTML = '<path class="donut-track" d="' + halfRingPath(cfg.cx, cfg.cy, cfg.rOuter, cfg.rInner) + '"/>';
    }

    var ids = ['critical', 'high', 'medium', 'low', 'info'];
    var severities = ids.map(function (id) {
        return { id: id, value: (bySeverity && typeof bySeverity[id] === 'number') ? bySeverity[id] : 0 };
    });
    var visible = severities.filter(function (s) { return s.value > 0; });

    if (!total || total <= 0 || visible.length === 0) {
        segmentsEl.innerHTML = '';
        labelsEl.innerHTML = '';
        return;
    }

    // 弧长按 value/total 计算；若严重度求和 < total（存在未分级），右侧会保留背景轨迹的空白
    var sumVisible = visible.reduce(function (s, seg) { return s + seg.value; }, 0);
    var coverage = sumVisible / total; // 半环被实际段覆盖的比例
    var visibleCount = visible.length;
    var totalGapRad = cfg.gapRad * Math.max(0, visibleCount - 1);
    // 半环可用的总弧度 = π * coverage（按比例填充），再扣除段间间隙
    var arcsTotalRad = Math.max(0, Math.PI * coverage - totalGapRad);

    var segmentsHtml = '';
    var labelsHtml = '';
    var cumRad = 0;

    visible.forEach(function (seg, i) {
        var arcFraction = seg.value / sumVisible;
        var segRad = arcsTotalRad * arcFraction;
        var angleStart = Math.PI - cumRad;
        var angleEnd = angleStart - segRad;

        var path = arcSegmentPath(cfg.cx, cfg.cy, cfg.rOuter, cfg.rInner, angleStart, angleEnd);
        segmentsHtml += '<path class="donut-segment seg-' + seg.id + '" d="' + path + '"/>';

        // 仅当占比 >= 5% 时显示外置标签，避免小段标签互相重叠
        var pctOfTotal = (seg.value / total) * 100;
        if (pctOfTotal >= 5) {
            var midAngle = (angleStart + angleEnd) / 2;
            var labelR = cfg.rOuter + cfg.labelOffset;
            var sinMid = Math.sin(midAngle);
            var cosMid = Math.cos(midAngle);
            var lx = cfg.cx + labelR * cosMid;
            // 顶部区域标签整体向上抬一些，避免与外弧贴住；侧边标签则不调整
            var topLift = sinMid > 0.4 ? Math.round((sinMid - 0.3) * 10) : 0;
            var ly = cfg.cy - labelR * sinMid - topLift;

            var anchor = 'middle';
            if (cosMid < -0.15) anchor = 'end';
            else if (cosMid > 0.15) anchor = 'start';

            var pctText = Math.round(pctOfTotal) + '%';
            var name = esc(severityLabel(seg.id));

            // 两行：第一行 "数量 (百分比)"（弧色），第二行 "严重度名称"（同色但稍小）
            labelsHtml += '<text class="donut-label-text label-' + seg.id + '" text-anchor="' + anchor + '" x="' + lx.toFixed(1) + '" y="' + ly.toFixed(1) + '">';
            labelsHtml += '<tspan x="' + lx.toFixed(1) + '" dy="0">' + seg.value + ' <tspan class="donut-label-pct">(' + pctText + ')</tspan></tspan>';
            labelsHtml += '<tspan class="donut-label-name" x="' + lx.toFixed(1) + '" dy="14">' + name + '</tspan>';
            labelsHtml += '</text>';
        }

        cumRad += segRad;
        if (i < visibleCount - 1) cumRad += cfg.gapRad;
    });

    segmentsEl.innerHTML = segmentsHtml;
    labelsEl.innerHTML = labelsHtml;
}

// SVG 半环（背景轨迹）路径
function halfRingPath(cx, cy, rOuter, rInner) {
    var x1Outer = cx - rOuter;
    var y1Outer = cy;
    var x2Outer = cx + rOuter;
    var y2Outer = cy;
    var x1Inner = cx - rInner;
    var y1Inner = cy;
    var x2Inner = cx + rInner;
    var y2Inner = cy;
    return 'M ' + x1Outer + ' ' + y1Outer +
        ' A ' + rOuter + ' ' + rOuter + ' 0 0 1 ' + x2Outer + ' ' + y2Outer +
        ' L ' + x2Inner + ' ' + y2Inner +
        ' A ' + rInner + ' ' + rInner + ' 0 0 0 ' + x1Inner + ' ' + y1Inner + ' Z';
}

// 单段弧形（angleStart > angleEnd，逆时针角度递减，视觉上沿半环顶部顺时针推进）
function arcSegmentPath(cx, cy, rOuter, rInner, angleStart, angleEnd) {
    var x1Outer = cx + rOuter * Math.cos(angleStart);
    var y1Outer = cy - rOuter * Math.sin(angleStart);
    var x2Outer = cx + rOuter * Math.cos(angleEnd);
    var y2Outer = cy - rOuter * Math.sin(angleEnd);
    var x1Inner = cx + rInner * Math.cos(angleStart);
    var y1Inner = cy - rInner * Math.sin(angleStart);
    var x2Inner = cx + rInner * Math.cos(angleEnd);
    var y2Inner = cy - rInner * Math.sin(angleEnd);

    var largeArc = (angleStart - angleEnd) > Math.PI ? 1 : 0;

    return 'M ' + x1Outer.toFixed(2) + ' ' + y1Outer.toFixed(2) +
        ' A ' + rOuter + ' ' + rOuter + ' 0 ' + largeArc + ' 1 ' + x2Outer.toFixed(2) + ' ' + y2Outer.toFixed(2) +
        ' L ' + x2Inner.toFixed(2) + ' ' + y2Inner.toFixed(2) +
        ' A ' + rInner + ' ' + rInner + ' 0 ' + largeArc + ' 0 ' + x1Inner.toFixed(2) + ' ' + y1Inner.toFixed(2) + ' Z';
}

// 语言切换后，仪表盘上由 JS 动态渲染的部分（KPI 副标、告警条、半环图标签、
// 状态卡、最近漏洞列表、能力总览徽章等）不会被 applyTranslations 自动重绘，
// 需要主动重新拉取数据并以新语言重新渲染；与 tasks/vulnerability 等其他页面保持一致。
document.addEventListener('languagechange', function () {
    try {
        var dashboardPage = document.getElementById('page-dashboard');
        if (!dashboardPage || !dashboardPage.classList.contains('active')) {
            return;
        }
        if (typeof refreshDashboard === 'function') {
            refreshDashboard();
        }
    } catch (e) {
        console.warn('languagechange dashboard refresh failed', e);
    }
});

// 页面可见性：从其他 tab 切回时，如果距离上次刷新已经过半个轮询周期，立刻补刷一次；
// 避免后台标签页停留几小时回来时数据还是旧的，又不至于每次切回都打接口。
document.addEventListener('visibilitychange', function () {
    if (document.hidden) return;
    var page = document.getElementById('page-dashboard');
    if (!page || !page.classList.contains('active')) return;
    var ageMs = Date.now() - (dashboardState.lastUpdatedAt || 0);
    if (ageMs >= DASHBOARD_POLL_INTERVAL_MS / 2) {
        try { refreshDashboard(); } catch (_) { /* ignore */ }
    } else {
        // 不需要重新拉数据，但也跑一次 stale 检查更新徽章状态
        checkDashboardStale();
    }
});

// 关闭告警条按钮：把当前 reasons 指纹存入 sessionStorage，本会话不再弹同样的内容
document.addEventListener('click', function (ev) {
    var btn = ev.target && ev.target.closest && ev.target.closest('#dashboard-alert-close');
    if (!btn) return;
    ev.preventDefault();
    var key = dashboardState.dismissedAlertKey || '';
    try { sessionStorage.setItem('dashboard.dismissedAlert', key); } catch (_) {}
    var banner = document.getElementById('dashboard-alert-banner');
    if (banner) banner.hidden = true;
});

