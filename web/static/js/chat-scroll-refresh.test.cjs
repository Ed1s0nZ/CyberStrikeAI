const fs = require('node:fs');
const test = require('node:test');
const assert = require('node:assert/strict');

const scroll = fs.readFileSync('web/static/js/chat-scroll.js', 'utf8');
const monitor = fs.readFileSync('web/static/js/monitor.js', 'utf8');
const chat = fs.readFileSync('web/static/js/chat.js', 'utf8');
const html = fs.readFileSync('web/templates/index.html', 'utf8');

function functionSource(source, name, nextName) {
    const start = source.indexOf(`function ${name}(`);
    const end = source.indexOf(`function ${nextName}(`, start);
    assert.notEqual(start, -1, `${name} should exist`);
    assert.notEqual(end, -1, `${nextName} should follow ${name}`);
    return source.slice(start, end);
}

test('用户真正滑到底部后恢复自动跟随且不会提前强制跳底', () => {
    const resumeSource = functionSource(scroll, 'resumeFollowingIfAtBottom', 'captureScrollPinState');
    const scrollSource = functionSource(scroll, 'onChatMessagesScroll', 'bindChatScrollListeners');

    assert.match(resumeSource, /thresholdPx/);
    assert.match(scrollSource, /scrolledDown/);
    assert.match(scrollSource, /resumeFollowingIfAtBottom\(CHAT_SCROLL_FOLLOW_THRESHOLD_PX\)/);
    assert.doesNotMatch(scrollSource, /resumeFollowingIfAtBottom\(CHAT_SCROLL_NAV_BOTTOM_THRESHOLD_PX\)/);
    assert.doesNotMatch(scrollSource, /scheduleChatScrollToBottomIfFollowing\(true\)/);
    assert.match(scrollSource, /contentShrank/);
    assert.match(scrollSource, /sh < lastScrollHeight - 1/);
    assert.match(scrollSource, /scrollMode === 'detached' \|\| Date\.now\(\) <= upwardScrollIntentUntil/);
});

test('刷新运行中任务补齐最新详情后保持粘底但尊重用户上滑', () => {
    const attachSource = functionSource(monitor, 'attachRunningTaskEventStream', 'parseToolCallArgsFromData');
    const settleSource = functionSource(scroll, 'settleChatToBottomIfFollowing', 'scrollChatMessagesToBottomIfPinned');

    assert.match(attachSource, /window\.captureScrollPinState\(\)/);
    assert.match(attachSource, /settleToBottomIfFollowing\(12\)/);
    assert.match(attachSource, /settleToBottomIfFollowing\(18\)/);
    assert.match(attachSource, /用户期间没有主动上滑/);
    assert.match(attachSource, /keepFollowingFinalRender/);
    assert.match(attachSource, /最终消息和详情重绘都会增高 DOM/);
    assert.match(settleSource, /scrollMode !== 'following'/);
    assert.match(settleSource, /Date\.now\(\) < detachLockUntil/);
    assert.match(settleSource, /settleFrame\(remaining - 1\)/);
    assert.match(settleSource, /scrollChatToBottomInstant\(\)/);
});

test('消息气泡内部流式增高时仅在跟随模式继续粘底', () => {
    const bindSource = functionSource(scroll, 'bindChatScrollListeners', 'initChatScroll');

    assert.match(bindSource, /scrollMode === 'following'/);
    assert.match(bindSource, /scheduleChatScrollToBottomIfFollowing\(true\)/);
    assert.match(bindSource, /\{ childList: true, subtree: true, characterData: true \}/);
    assert.match(bindSource, /e\.deltaY < -1/);
    assert.match(bindSource, /e\.clientX >= rect\.right - 18/);
    assert.match(bindSource, /e\.key === 'ArrowUp'/);
});

test('页面在任务补流脚本之前加载智能滚动控制器', () => {
    const scrollIndex = html.indexOf('/static/js/chat-scroll.js?v=20260812-2');
    const monitorIndex = html.indexOf('/static/js/monitor.js?v=20260812-5');

    assert.notEqual(scrollIndex, -1);
    assert.notEqual(monitorIndex, -1);
    assert.ok(scrollIndex < monitorIndex);
});

test('直接点击项目对话也会写入 hash 以便刷新后恢复并补流', () => {
    const loadSource = functionSource(chat, 'loadConversation', 'attachDeleteTurnButton');
    const syncSource = functionSource(chat, 'syncChatConversationHash', 'getConversationLiteFromCache');
    const streamSource = functionSource(monitor, 'setCurrentConversationIdFromStream', 'shouldSkipTaskEventReplayAttach');

    assert.match(syncSource, /window\.location\.hash\.split\('\?'\)\[0\] !== '#chat'/);
    assert.match(syncSource, /#chat\?conversation=/);
    assert.match(syncSource, /window\.history\.replaceState/);
    assert.match(loadSource, /syncChatConversationHash\(conversationId\)/);
    assert.match(streamSource, /window\.syncChatConversationHash\(cid\)/);
});

test('刷新恢复运行中助手消息时隐藏处理中占位且终态正文会重新显示', () => {
    const loadSource = functionSource(chat, 'loadConversation', 'attachDeleteTurnButton');
    const updateSource = functionSource(monitor, 'updateAssistantBubbleContent', 'isConversationTaskRunning');

    assert.match(loadSource, /hideAssistantPlaceholder: isAssistantPlaceholder/);
    assert.match(chat, /bubble\.hidden = true/);
    assert.match(updateSource, /assistant-placeholder-content/);
    assert.match(updateSource, /bubble\.hidden = false/);
});

test('刷新补流任务完成后强制折叠自动展开的迭代详情', () => {
    const collapseSource = functionSource(monitor, 'collapseAllProgressDetails', 'getAssistantId');
    const attachSource = functionSource(monitor, 'attachRunningTaskEventStream', 'parseToolCallArgsFromData');

    assert.match(collapseSource, /options/);
    assert.match(collapseSource, /forceCollapse/);
    assert.match(collapseSource, /delete detailsContainer\.dataset\.userExpanded/);
    assert.match(attachSource, /collapseAllProgressDetails\(finalAssistant\.id, progressId, \{ force: true \}\)/);
    assert.doesNotMatch(attachSource, /if \(keepExpanded\)/);
});

test('暗色模式用户气泡使用协调的深蓝灰层级', () => {
    const css = fs.readFileSync('web/static/css/style.css', 'utf8');
    assert.match(css, /html\[data-theme="dark"\] \.message\.user \.message-bubble \{[\s\S]*?background: #1b2638;/);
    assert.match(css, /border-color: rgba\(96, 165, 250, 0\.18\)/);
});
