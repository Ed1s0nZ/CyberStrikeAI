const fs = require('node:fs');
const test = require('node:test');
const assert = require('node:assert/strict');

const chat = fs.readFileSync('web/static/js/chat.js', 'utf8');
const monitor = fs.readFileSync('web/static/js/monitor.js', 'utf8');

function functionSource(source, name, nextName) {
    const start = source.indexOf(`function ${name}(`);
    const end = source.indexOf(`function ${nextName}(`, start);
    assert.notEqual(start, -1, `${name} should exist`);
    assert.notEqual(end, -1, `${nextName} should follow ${name}`);
    return source.slice(start, end);
}

test('用户和助手消息使用同一复制按钮入口', () => {
    const helperSource = functionSource(chat, 'appendMessageCopyButton', 'addMessage');
    const addMessageSource = functionSource(chat, 'addMessage', 'copyMessageToClipboard');

    assert.match(helperSource, /classList\.contains\('assistant'\)[\s\S]*classList\.contains\('user'\)/);
    assert.match(helperSource, /const footer = ensureMessageMetaFooter\(content\)/);
    assert.match(helperSource, /message-bubble \.message-copy-btn/);
    assert.match(helperSource, /copyMessageToClipboard\(messageDiv, this\)/);
    assert.match(addMessageSource, /role === 'assistant' \|\| role === 'user'[\s\S]*messageDiv\.dataset\.originalContent = content/);
    assert.match(addMessageSource, /metaFooter\.appendChild\(timeDiv\)/);
    assert.match(addMessageSource, /role === 'assistant' \|\| role === 'user'[\s\S]*appendMessageCopyButton\(messageDiv\)/);
});

test('刷新消息内容时会保留或补回复制按钮', () => {
    const refreshSource = functionSource(chat, 'refreshSystemReadyMessageBubbles', 'appendMessageCopyButton');
    const updateSource = functionSource(monitor, 'updateAssistantBubbleContent', 'isConversationTaskRunning');

    assert.match(refreshSource, /appendMessageCopyButton\(messageDiv\)/);
    assert.match(updateSource, /window\.appendMessageCopyButton\(assistantElement\)/);
});
