const fs = require('node:fs');
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
    assert.match(projects, /setInterval\(update, 250\)/);
    assert.match(monitor, /function renderDirectHitlSidebarApproval/);
    assert.match(monitor, /hitlSidebarApprovalSyncTimer = window\.setInterval/);
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
