const fs = require('node:fs');
const test = require('node:test');
const assert = require('node:assert/strict');

const chat = fs.readFileSync('web/static/js/chat.js', 'utf8');
const hitl = fs.readFileSync('web/static/js/hitl.js', 'utf8');

function functionSource(source, name, nextName) {
    const start = source.indexOf(`function ${name}(`);
    const end = source.indexOf(`function ${nextName}(`, start);
    assert.notEqual(start, -1, `${name} should exist`);
    assert.notEqual(end, -1, `${nextName} should follow ${name}`);
    return source.slice(start, end);
}

test('已有会话缺少本地配置时不会继承其他会话的最近审批设置', () => {
    const source = functionSource(chat, 'getHitlConfigForConversation', 'setHitlReviewerUI');
    const existingConversationBranch = source.slice(source.indexOf('const key = getHitlStorageKeyByConversation(cid)'));

    assert.doesNotMatch(existingConversationBranch, /getHitlLastGlobalConfig/);
    assert.match(existingConversationBranch, /if \(!raw\) \{\s*return fallback;/);
    assert.match(existingConversationBranch, /catch \(e\) \{\s*return fallback;/);
});

test('服务端默认审批人只更新默认值，不覆盖最近会话选择', () => {
    const source = functionSource(hitl, 'applyHitlDefaultReviewerFromServer', 'fetchHitlDefaultReviewer');

    assert.match(source, /window\.csaiHitlDefaultReviewer = v/);
    assert.doesNotMatch(source, /saveHitlLastGlobalConfig/);
});

test('恢复会话审批配置时保留该会话自己的审批人', () => {
    const source = functionSource(hitl, 'syncHitlConfigFromServer', 'syncHitlConfigToServerByCurrentConversation');

    assert.match(source, /const localReviewer = hitlReviewerNormalize\(local && local\.reviewer\)/);
    assert.match(source, /merged = \{[\s\S]*?reviewer: localReviewer/);
    assert.match(source, /saveHitlConversationConfig\(conversationId, \{[\s\S]*?reviewer: localReviewer/);
    assert.doesNotMatch(source, /getHitlLastGlobalConfig/);
});

test('异步同步只能刷新仍处于当前会话的审批界面', () => {
    const source = functionSource(hitl, 'syncHitlConfigFromServer', 'syncHitlConfigToServerByCurrentConversation');

    assert.match(source, /getCurrentConversationIdForHitl\(\) === conversationId[\s\S]*?window\.applyHitlConfigToUI\(normalizedCfg\)/);
});
