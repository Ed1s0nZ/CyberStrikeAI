const fs = require('node:fs');
const test = require('node:test');
const assert = require('node:assert/strict');

const scroll = fs.readFileSync('web/static/js/chat-scroll.js', 'utf8');
const monitor = fs.readFileSync('web/static/js/monitor.js', 'utf8');
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

    assert.match(attachSource, /window\.captureScrollPinState\(\)/);
    assert.match(attachSource, /window\.CyberStrikeChatScroll\.forceScrollToBottom\(false\)/);
    assert.match(attachSource, /用户期间没有主动上滑/);
    assert.match(attachSource, /keepFollowingFinalRender/);
    assert.match(attachSource, /最终消息和详情重绘都会增高 DOM/);
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
    const scrollIndex = html.indexOf('/static/js/chat-scroll.js?v=20260812-1');
    const monitorIndex = html.indexOf('/static/js/monitor.js?v=20260812-2');

    assert.notEqual(scrollIndex, -1);
    assert.notEqual(monitorIndex, -1);
    assert.ok(scrollIndex < monitorIndex);
});
