const fs = require('node:fs');
const vm = require('node:vm');
const test = require('node:test');
const assert = require('node:assert/strict');

const monitor = fs.readFileSync('web/static/js/monitor.js', 'utf8');
const projects = fs.readFileSync('web/static/js/projects.js', 'utf8');
const chat = fs.readFileSync('web/static/js/chat.js', 'utf8');
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

test('审批请求按浏览器、命令、文件和通用工具动态描述', () => {
    assert.match(monitor, /function hitlApprovalTemplate/);
    assert.match(monitor, /hitlApprovalTranslate\(key, fallback\)/);
    assert.match(monitor, /replaceAll\('\{\{' \+ name \+ '\}\}'/);
    assert.match(monitor, /function describeHitlApprovalRequest/);
    assert.match(monitor, /requestVisitUrl/);
    assert.match(monitor, /requestCommand/);
    assert.match(monitor, /requestFile/);
    assert.match(monitor, /requestGeneric/);
});

test('人工批准不要求输入备注，审查编辑仅发送真正修改过的参数', () => {
    assert.match(monitor, /if \(!approveBtn \|\| !rejectBtn \|\| !statusEl\) return/);
    assert.doesNotMatch(monitor, /!commentInput \|\| !statusEl/);
    assert.match(monitor, /JSON\.stringify\(editedArgs\) === JSON\.stringify\(originalArgs\)/);
    assert.match(monitor, /editedArgs = null/);
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

test('项目文件夹汇总待审批数量并按最早到期时间使用绿黄红三档变色', () => {
    assert.match(projects, /waitingApprovalCount/);
    assert.match(projects, /aggregate: true, count: folderApprovals\.length/);
    assert.match(projects, /currentExpiry < earliestExpiry/);
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

test('无项目对话使用独立虚拟文件夹且新任务默认解除项目绑定', () => {
    assert.match(projects, /CHAT_UNASSIGNED_PROJECT_FOLDER_ID/);
    assert.match(projects, /_isUnassigned: true/);
    assert.match(projects, /\[unassignedProject, \.\.\.filtered\]/);
    assert.match(projects, /window\.startNewConversation\(\{ projectId: isUnassigned \? '' : project\.id \}\)/);
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
    assert.match(projects, /if \(!changed\) return/);
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

test('任务结束后对话内审批按钮会变灰并禁止继续操作', () => {
    assert.match(monitor, /ready: false/);
    assert.match(monitor, /function setHitlApprovalTaskAvailability/);
    assert.match(monitor, /conversationExecutionTracker\.ready && !conversationExecutionTracker\.isRunning\(id\)/);
    assert.match(monitor, /button\.disabled = true/);
    assert.match(monitor, /syncHitlApprovalTaskAvailability\(\)/);
    assert.match(fs.readFileSync('web/static/css/style.css', 'utf8'), /hitl-approval-task-closed/);
    assert.equal(zh.hitl.taskClosedApprovalUnavailable, '任务已结束，审批不可用');
    assert.equal(typeof en.hitl.taskClosedApprovalUnavailable, 'string');
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
