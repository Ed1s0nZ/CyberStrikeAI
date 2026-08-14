const fs = require('node:fs');
const vm = require('node:vm');
const test = require('node:test');
const assert = require('node:assert/strict');

const monitor = fs.readFileSync('web/static/js/monitor.js', 'utf8');
const chatScroll = fs.readFileSync('web/static/js/chat-scroll.js', 'utf8');
const projects = fs.readFileSync('web/static/js/projects.js', 'utf8');
const chat = fs.readFileSync('web/static/js/chat.js', 'utf8');
const styles = fs.readFileSync('web/static/css/style.css', 'utf8');
const template = fs.readFileSync('web/templates/index.html', 'utf8');
const handler = fs.readFileSync('internal/handler/hitl.go', 'utf8');
const zh = JSON.parse(fs.readFileSync('web/static/i18n/zh-CN.json', 'utf8'));
const en = JSON.parse(fs.readFileSync('web/static/i18n/en-US.json', 'utf8'));

test('输入区提供独立审批入口并暴露可配置等待时限', () => {
    assert.match(template, /id="chat-hitl-approval-dock"/);
    assert.match(template, /id="hitl-timeout-select"/);
    assert.match(template, /option value="300" selected/);
    assert.match(chat, /DEFAULT_HITL_TIMEOUT_SECONDS = 300/);
    assert.match(chat, /timeoutSeconds: normalizeHitlTimeoutForChat/);
    assert.match(chat, /body\.hitl = \{[\s\S]*?timeoutSeconds: normalizeHitlTimeoutForChat\(hitlCfg\.timeoutSeconds/);
});

test('输入框可直接保存系统模型和系统推理强度且审批模型只出现在审计 Agent 入口', () => {
    assert.match(chat, /function currentSystemModelLabel\(\)/);
    assert.match(chat, /chatDefaultAIChannel \? chatAIChannels\[chatDefaultAIChannel\]/);
    assert.match(chat, /function currentHitlAuditModelLabel\(\)/);
    assert.match(chat, /const label = currentSystemModelLabel\(\)/);
    assert.doesNotMatch(chat, /const label = data\.model \|\| currentChatModelLabel\(\)/);
    assert.match(chat, /const approvalModel = auditAgent \? currentHitlAuditModelLabel\(\) : ''/);
    assert.match(chat, /hitlAuditModel\.model\.trim\(\)/);
    assert.match(template, /id="chat-model-shortcut"[^>]+onclick="openChatSystemModelPicker\(event\)"/);
    assert.match(template, /id="chat-system-model-menu"[^>]+hidden/);
    assert.doesNotMatch(template, /id="chat-reasoning-shortcut"/);
    assert.match(template, /openChatSystemModelView\('model', event\)[\s\S]{0,1200}openChatSystemModelView\('effort', event\)/);
    assert.match(chat, /function renderChatReasoningEffortOptions\(\)/);
    assert.match(chat, /function currentSystemReasoningEffort\(\)[\s\S]{0,500}reasoning\.effort/);
    assert.match(chat, /case 'low': return 'low'[\s\S]{0,300}case 'max': return 'max'/);
    assert.match(chat, /chatTranslate\('chat\.reasoningEffortUnset', '不指定'\)/);
    assert.match(chat, /function selectChatReasoningEffort\(effort\)[\s\S]{0,2400}reasoning: \{ \.\.\.\(state\.channel\.reasoning \|\| \{\}\), effort: chosen \}/);
    assert.match(chat, /function selectChatReasoningEffort\(effort\)[\s\S]{0,4200}body: JSON\.stringify\(\{ ai: state\.ai \}\)[\s\S]{0,900}apiFetch\('\/api\/config\/apply'/);
    assert.match(chat, /function openChatSystemModelPicker\(event\)[\s\S]{0,4200}apiFetch\('\/api\/config\/list-models'/);
    assert.match(chat, /function selectChatSystemModel\(model\)[\s\S]{0,2600}method: 'PUT'[\s\S]{0,900}apiFetch\('\/api\/config\/apply'/);
    assert.match(chat, /body: JSON\.stringify\(\{ ai: state\.ai \}\)/);
    assert.equal(zh.chat.modelSettingsAria, '选择模型与推理强度');
    assert.equal(en.chat.modelSettingsAria, 'Choose model and reasoning effort');
});

test('审批请求按浏览器、命令、文件和通用工具动态描述', () => {
    assert.match(monitor, /function hitlApprovalTemplate/);
    assert.match(monitor, /hitlApprovalTranslate\(key, fallback\)/);
    assert.match(monitor, /replaceAll\('\{\{' \+ name \+ '\}\}'/);
    assert.match(monitor, /function describeHitlApprovalRequest/);
    assert.match(monitor, /requestVisitUrl/);
    assert.match(monitor, /requestCommand/);
    assert.match(monitor, /requestFile/);
    assert.match(monitor, /requestGeneric/);
    assert.match(monitor, /let displayTool = rawToolName/);
    assert.doesNotMatch(monitor, /displayTool = 'Browser'/);
    assert.doesNotMatch(monitor, /displayTool = hitlApprovalTranslate\('hitl\.toolTerminal'/);
    assert.doesNotMatch(monitor, /displayTool = hitlApprovalTranslate\('hitl\.toolFiles'/);
});

test('Agent 审查不进入人工审批弹窗、倒计时和项目计数', () => {
    const logsHandler = fs.readFileSync('internal/handler/hitl_logs.go', 'utf8');
    const hitlPage = fs.readFileSync('web/static/js/hitl.js', 'utf8');
    assert.match(handler, /CreatePendingInterrupt\([\s\S]{0,260}reviewer string/);
    assert.match(handler, /reviewer != "audit_agent"[\s\S]{0,120}m\.pending\[id\] = p/);
    assert.match(logsHandler, /ADD COLUMN reviewer TEXT NOT NULL DEFAULT 'human'/);
    assert.match(logsHandler, /status = 'pending' AND COALESCE\(reviewer,'human'\) = 'human'/);
    assert.match(monitor, /function isAgentReviewedHitl\(data\)/);
    assert.match(monitor, /if \(!data\.resolved && !isAgentReviewedHitl\(data\)\)/);
    assert.match(monitor, /if \(!isAgentReviewedHitl\(data\)\) \{[\s\S]{0,240}bindHitlApprovalCountdown/);
    assert.match(monitor, /if \(isAgentReviewedHitl\(data\)\) return false/);
    assert.match(projects, /filter\(isHumanProjectPendingApproval\)/);
    assert.match(projects, /if \(!isHumanProjectPendingApproval\(details\)\) return/);
    assert.match(hitlPage, /const items = rawItems\.filter/);
});

test('人工批准不要求输入备注，审查编辑仅发送真正修改过的参数', () => {
    assert.match(monitor, /if \(!approveBtn \|\| !rejectBtn \|\| !statusEl\) return/);
    assert.doesNotMatch(monitor, /!commentInput \|\| !statusEl/);
    assert.match(monitor, /JSON\.stringify\(editedArgs\) === JSON\.stringify\(originalArgs\)/);
    assert.match(monitor, /editedArgs = null/);
});

test('长历史对话的回到最新按钮不会把滚动点击穿透到审批操作', () => {
    assert.match(chatScroll, /function isolateReturnLatestPointerEvent\(event\)/);
    assert.match(chatScroll, /returnLatestButton\.addEventListener\('pointerdown', isolateReturnLatestPointerEvent\)/);
    assert.match(chatScroll, /function onReturnLatestClick\(event\)[\s\S]{0,260}event\.preventDefault\(\)[\s\S]{0,180}event\.stopPropagation\(\)/);
    assert.match(monitor, /const bindExplicitHitlAction = function \(button, decision\)/);
    assert.match(monitor, /button\.addEventListener\('pointerdown'[\s\S]{0,900}pointerClick && !explicitlyPressed/);
    assert.match(monitor, /bindExplicitHitlAction\(approveBtn, 'approve'\)/);
    assert.match(monitor, /bindExplicitHitlAction\(rejectBtn, 'reject'\)/);
});

test('轮次导航使用连续大热区并允许鼠标平滑进入 Codex 风格预览卡', () => {
    const styles = fs.readFileSync('web/static/css/style.css', 'utf8');
    assert.match(styles, /\.chat-turn-rail-markers \{[\s\S]{0,260}gap: 0;/);
    assert.match(styles, /\.chat-turn-rail-markers \{[\s\S]{0,420}overflow-x: hidden;/);
    assert.match(styles, /\.chat-turn-rail-markers \{[\s\S]{0,520}touch-action: pan-y;/);
    assert.match(styles, /\.chat-turn-rail-marker \{[\s\S]{0,260}width: 36px;[\s\S]{0,160}height: 11px;/);
    assert.match(styles, /\.chat-turn-rail-marker::before \{[\s\S]{0,420}width: 12px;[\s\S]{0,120}height: 3px;/);
    assert.match(styles, /\.chat-turn-rail-marker:hover::before \{[\s\S]{0,100}width: 22px;/);
    assert.match(styles, /\.chat-turn-rail-preview \{[\s\S]{0,520}pointer-events: auto;/);
    assert.match(chatScroll, /function scheduleHideTurnPreview\(\)/);
    assert.match(chatScroll, /window\.setTimeout\(hideTurnPreview, 160\)/);
    assert.match(chatScroll, /turnPreview\.addEventListener\('mouseenter'/);
    assert.match(chatScroll, /marker\.addEventListener\('mouseleave', scheduleHideTurnPreview\)/);
});

test('倒计时由服务端时间驱动，到期时只锁定界面并等待服务端拒绝', () => {
    assert.match(handler, /payload\["hitlApproval"\]/);
    assert.match(handler, /"expiresAt":\s+approvalExpiresAt/);
    assert.match(handler, /status = "timeout"/);
    assert.match(handler, /decidedBy = "system"/);
    assert.match(monitor, /function bindHitlApprovalCountdown/);
    assert.match(monitor, /setInterval\(update, 250\)/);
    assert.match(monitor, /expiredAutoRejected/);
    assert.doesNotMatch(monitor, /remaining <= 0[\s\S]{0,240}submitHitlDecisionWithPayload/);
});

test('项目对话列表能同时显示等待批准与运行状态', () => {
    assert.match(projects, /pendingApprovalByConversation: new Map/);
    assert.match(projects, /statusKinds\.push\('approval'\)/);
    assert.match(projects, /statusKinds\.push\('running'\)/);
    assert.match(projects, /window\.setProjectConversationApprovalStatus/);
    assert.match(projects, /api\/hitl\/pending\?page=1&pageSize=200/);
    assert.match(projects, /function bindProjectApprovalProgress/);
    assert.match(projects, /project-approval-progress-value/);
    assert.match(projects, /PROJECT_APPROVAL_TICK_INTERVAL_MS = 1000/);
    assert.match(projects, /function registerProjectApprovalTicker/);
    assert.match(monitor, /function renderDirectHitlSidebarApproval/);
    assert.match(monitor, /hitlSidebarApprovalSyncTimer = window\.setInterval/);
});

test('项目文件夹汇总始终为绿色且只有具体对话按剩余时间变色', () => {
    assert.match(projects, /waitingApprovalCount/);
    assert.match(projects, /aggregate: true, count: folderApprovals\.length/);
    assert.match(projects, /project-task-status--approval-summary', 'is-urgency-normal'/);
    assert.match(projects, /status\.dataset\.approvalUrgency = 'normal'/);
    assert.match(projects, /if \(isApprovalSummary\)[\s\S]{0,520}else \{[\s\S]{0,160}bindProjectApprovalUrgency\(status, details, label\)/);
    assert.doesNotMatch(projects, /currentExpiry < earliestExpiry/);
    assert.match(projects, /PROJECT_APPROVAL_URGENCY_CLASSES/);
    assert.match(projects, /remaining <= 60 \* 1000/);
    assert.match(projects, /remaining <= 3 \* 60 \* 1000/);
    assert.doesNotMatch(projects, /remaining <= 5 \* 60 \* 1000/);
    assert.match(projects, /project-task-status--approval-summary/);
    assert.equal(zh.hitl.waitingApprovalCount, '等待批准 {{count}}');
    assert.equal(zh.hitl.approvalUrgencyMoreThanThree, '最早审批将在 3 分钟后到期');
    assert.equal(typeof en.hitl.waitingApprovalCount, 'string');
    const urgencyFunctionSource = projects.match(
        /function projectApprovalUrgencyLevel\(remainingMilliseconds, hasDeadline\) \{[\s\S]*?\n\}/
    );
    assert.ok(urgencyFunctionSource, '应提供可测试的审批紧急程度函数');
    const urgencyLevel = vm.runInNewContext(`(${urgencyFunctionSource[0]})`);
    assert.equal(urgencyLevel(6 * 60 * 1000, true), 'normal');
    assert.equal(urgencyLevel(4 * 60 * 1000, true), 'normal');
    assert.equal(urgencyLevel(3 * 60 * 1000 + 1, true), 'normal');
    assert.equal(urgencyLevel(3 * 60 * 1000, true), 'warning');
    assert.equal(urgencyLevel(2 * 60 * 1000, true), 'warning');
    assert.equal(urgencyLevel(30 * 1000, true), 'critical');
    assert.equal(urgencyLevel(0, false), 'normal');
});

test('切换对话后主按钮只读取当前可见对话的运行状态', () => {
    assert.match(chat, /function getVisibleChatConversationId\(\)/);
    assert.match(chat, /function shouldTreatLiveChatTaskAsCurrent\(/);
    assert.match(chat, /function isLiveChatTaskVisible\(/);
    assert.match(chat, /if \(visibleConversationId\) return visibleConversationId/);
    assert.match(chat, /isConversationTaskRunning\(visibleConversationId\)/);
    assert.doesNotMatch(
        chat,
        /function getCurrentChatTaskConversationId\(\) \{[\s\S]{0,220}if \(live && live\.active && live\.conversationId\) \{[\s\S]{0,100}return String\(live\.conversationId\)/
    );
    const visibilityFunctionSource = chat.match(
        /function shouldTreatLiveChatTaskAsCurrent\(liveConversationId, visibleConversationId, hasVisibleProgress\) \{[\s\S]*?\n\}/
    );
    assert.ok(visibilityFunctionSource, '应提供可测试的当前任务隔离函数');
    const isCurrent = vm.runInNewContext(`(${visibilityFunctionSource[0]})`);
    assert.equal(isCurrent('running-conversation', '', true), false);
    assert.equal(isCurrent('running-conversation', 'new-conversation', true), false);
    assert.equal(isCurrent('running-conversation', 'running-conversation', false), true);
    assert.equal(isCurrent('', '', true), true);
    assert.equal(isCurrent('', '', false), false);
});

test('无项目使用独立虚拟文件夹且顶部新任务继承当前项目', () => {
    assert.match(projects, /CHAT_UNASSIGNED_PROJECT_FOLDER_ID/);
    assert.match(projects, /_isUnassigned: true/);
    assert.match(projects, /\[\.\.\.pinnedProjects, unassignedProject, \.\.\.regularProjects\]/);
    assert.match(projects, /window\.startNewConversation\(\{ projectId: isUnassigned \? '' : project\.id \}\)/);
    assert.match(chat, /Object\.prototype\.hasOwnProperty\.call\(options, 'projectId'\)/);
    assert.match(chat, /typeof resolveChatProjectSelection === 'function'/);
    assert.match(chat, /String\(inheritedProjectId \|\| ''\)\.trim\(\)/);
    assert.match(chat, /typeof setActiveProjectId === 'function'\) setActiveProjectId\(requestedProjectId\)/);
    assert.equal(zh.chat.newUnassignedConversation, '新建无项目对话');
    assert.equal(typeof en.chat.newUnassignedConversation, 'string');
});

test('单个对话的审批徽标随倒计时同步切换紧急颜色', () => {
    assert.match(projects, /bindProjectApprovalProgress\(status, details\);\s*bindProjectApprovalUrgency\(status, details, label\);/);
    assert.match(fs.readFileSync('web/static/css/style.css', 'utf8'), /\.project-task-status--approval\.is-urgency-critical/);
});

test('项目状态刷新复用单一计时器且切换对话不重复请求完整项目上下文', () => {
    assert.match(projects, /const projectApprovalTickerEntries = new Set\(\)/);
    assert.match(projects, /if \(!changed && !approvalChanged\) return/);
    assert.match(projects, /options\.reloadFolders !== false/);
    assert.match(chat, /refreshChatProjectSelector\(\{ reloadFolders: false, renderFolders: false \}\)/);
    assert.match(projects, /function selectChatProjectConversationItem/);
    assert.match(projects, /options\.renderFolders !== false/);
    assert.match(projects, /projectConversationPreviewSuppressedUntil = Date\.now\(\) \+ 700/);
    assert.match(projects, /project-task-status-group--folder/);
    assert.doesNotMatch(fs.readFileSync('web/static/css/style.css', 'utf8'), /project-task-status-group--folder \.project-task-status--running/);
    assert.match(fs.readFileSync('web/static/css/style.css', 'utf8'), /\.active-tasks-bar \{[\s\S]*?padding: 13px 24px 14px;/);
});

test('运行中对话切换会取消旧事件流并仅恢复最新一页过程详情', () => {
    assert.match(chat, /window\.cancelRunningTaskEventStream\(conversationId\)/);
    assert.match(monitor, /function cancelRunningTaskEventStream/);
    assert.match(monitor, /abortController\.abort\(\)/);
    assert.match(monitor, /signal: abortController\.signal/);
    assert.match(monitor, /initialLatest: true/);
    assert.match(monitor, /autoLoadAll: false/);
});

test('多对话并发时释放隐藏主流且旧请求不能覆盖新对话状态', () => {
    assert.match(chat, /function ownsLiveChatStream\(liveStream\)/);
    assert.match(chat, /function clearLiveChatStreamIfOwned\(liveStream\)/);
    assert.match(chat, /function detachLiveChatStreamForNavigation\(nextConversationId, force = false\)/);
    assert.match(chat, /liveStream\.detached = true;[\s\S]{0,240}controller\.abort\(\)/);
    assert.match(chat, /const requestAbortController = new AbortController\(\)/);
    assert.match(chat, /signal: requestAbortController\.signal/);
    assert.match(chat, /if \(!ownsLiveChatStream\(liveStreamState\) \|\| liveStreamState\.detached\)/);
    assert.match(chat, /const clearedOwnedStream = clearLiveChatStreamIfOwned\(liveStreamState\)/);
    assert.match(chat, /detachLiveChatStreamForNavigation\(conversationId\)/);
    assert.match(chat, /detachLiveChatStreamForNavigation\('', true\)/);
    assert.match(chat, /window\.clearChatHitlApprovalDock\(\)/);
    assert.match(monitor, /if \(conversationId && conversationId !== currentId\) return false/);
    assert.match(monitor, /function scrollProcessDetailsToLatest\(assistantMessageId, smooth = true\)/);
    assert.match(monitor, /timeline\.scrollTop = targetTop/);
    assert.match(chat, /let loadConversationAbortController = null/);
    assert.match(chat, /cancelPendingConversationLoad\(\);[\s\S]{0,220}const conversationLoadController = new AbortController\(\)/);
    assert.match(chat, /signal: conversationLoadController\.signal/);
    assert.match(template, /monitor\.js\?v=20260813-9/);
    assert.match(template, /chat-scroll\.js\?v=20260813-6/);
    assert.match(template, /chat\.js\?v=20260813-4/);
    assert.match(template, /style\.css\?v=20260813-6/);
});

test('输入区 Agent 审查文字保留足够行高且不会裁切字形', () => {
    assert.match(styles, /\.chat-hitl-shortcut > span\s*\{[\s\S]*?display: block/);
    assert.match(styles, /\.chat-hitl-shortcut > span\s*\{[\s\S]*?padding-block: 1px/);
    assert.match(styles, /\.chat-hitl-shortcut > span\s*\{[\s\S]*?line-height: 1\.4/);
});

test('任务结束后对话内审批按钮会变灰并禁止继续操作', () => {
    assert.match(monitor, /ready: false/);
    assert.match(monitor, /function setHitlApprovalTaskAvailability/);
    assert.match(monitor, /conversationExecutionTracker\.ready && !conversationExecutionTracker\.isRunning\(id\)/);
    assert.match(monitor, /hitlPendingInterruptTracker\.ready/);
    assert.match(monitor, /!hitlPendingInterruptTracker\.has\(interruptId\)/);
    assert.match(monitor, /button\.disabled = true/);
    assert.match(monitor, /function setHitlApprovalInterruptedVisualState/);
    assert.match(monitor, /stopHitlApprovalCountdown\(panel\)/);
    assert.match(monitor, /removeAttribute\('data-hitl-expires-at'\)/);
    assert.match(monitor, /hitl\.interruptedApprovalCancelled/);
    assert.match(monitor, /reconcileHitlApprovalStateWithActiveTasks\(normalizedTasks\)/);
    assert.match(monitor, /syncHitlApprovalTaskAvailability\(\)/);
    assert.match(fs.readFileSync('web/static/css/style.css', 'utf8'), /hitl-approval-task-closed/);
    assert.equal(zh.hitl.taskClosedApprovalUnavailable, '任务已结束，审批不可用');
    assert.equal(zh.hitl.interruptedApprovalCancelled, '任务已中断，审批已取消');
    assert.equal(typeof en.hitl.taskClosedApprovalUnavailable, 'string');
    assert.equal(typeof en.hitl.interruptedApprovalCancelled, 'string');
});

test('项目树只保留当前进程仍在运行任务的审批状态', () => {
    assert.match(projects, /chatProjectFolderContext\.runningIds\.has\(conversationId\)/);
    assert.match(projects, /pendingApprovalByConversation\.delete\(conversationId\)/);
    assert.match(monitor, /conversationExecutionTracker\.ready && !conversationExecutionTracker\.isRunning\(conversationId\)/);
});

test('审批状态主动轮询并在服务不可用时立即关闭旧审批', () => {
    assert.match(monitor, /ACTIVE_TASK_REFRESH_INTERVAL = 2000/);
    assert.match(monitor, /apiFetch\('\/api\/hitl\/pending\?page=1&pageSize=200'\)/);
    assert.match(monitor, /function reconcilePendingHitlState\(rawItems\)/);
    assert.match(monitor, /renderChatHitlApprovalDock\(currentPending\)/);
    assert.match(monitor, /restoreHitlInlineForConversation\(currentId\)/);
    assert.match(monitor, /case 'conversation':[\s\S]{0,1800}window\.refreshChatProjectFolders\(\)/);
    assert.match(monitor, /renderActiveTasks\(\[\]\);[\s\S]{0,260}hitlPendingInterruptTracker\.update\(\[\]\)/);
    assert.match(projects, /function syncProjectConversationApprovalStatuses\(items\)/);
    assert.match(projects, /window\.syncProjectConversationApprovalStatuses/);
    assert.match(template, /projects\.js\?v=20260812-6/);
});

test('旧会话首次升级到五分钟默认审批时限，仍允许用户之后主动选择不限时', () => {
    assert.match(fs.readFileSync('web/static/js/hitl.js', 'utf8'), /HITL_TIMEOUT_DEFAULT_MIGRATION_PREFIX/);
    assert.match(fs.readFileSync('web/static/js/hitl.js', 'utf8'), /shouldMigrateLegacyHitlTimeout/);
    assert.match(fs.readFileSync('web/static/js/hitl.js', 'utf8'), /timeoutSeconds: 300/);
    assert.match(fs.readFileSync('web/static/js/hitl.js', 'utf8'), /markLegacyHitlTimeoutMigrated/);
});

test('审批体验文案具有完整中英文资源', () => {
    const hitlKeys = [
        'waitingApprovalShort',
        'requestVisitUrl',
        'requestCommand',
        'viewRequestDetails',
        'timeoutAutoReject',
        'expiredRejected',
    ];
    const chatKeys = [
        'hitlTimeoutLabel',
        'hitlTimeoutFiveMinutes',
        'hitlTimeoutUnlimited',
        'hitlTimeoutHint',
    ];
    hitlKeys.forEach((key) => {
        assert.equal(typeof zh.hitl[key], 'string');
        assert.equal(typeof en.hitl[key], 'string');
    });
    chatKeys.forEach((key) => {
        assert.equal(typeof zh.chat[key], 'string');
        assert.equal(typeof en.chat[key], 'string');
    });
});
