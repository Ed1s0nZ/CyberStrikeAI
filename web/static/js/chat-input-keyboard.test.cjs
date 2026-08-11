const fs = require('node:fs');
const vm = require('node:vm');
const test = require('node:test');
const assert = require('node:assert/strict');

const chat = fs.readFileSync('web/static/js/chat.js', 'utf8');

function functionSource(source, name, nextName) {
    const start = source.indexOf(`function ${name}(`);
    const end = source.indexOf(`function ${nextName}(`, start);
    assert.notEqual(start, -1, `${name} should exist`);
    assert.notEqual(end, -1, `${nextName} should follow ${name}`);
    return source.slice(start, end);
}

function createKeydownHarness() {
    const context = {
        isComposing: false,
        mentionState: { active: false },
        mentionSuggestionsEl: null,
        sendCount: 0,
        sendMessage() {
            context.sendCount += 1;
        },
    };
    vm.runInNewContext(
        `${functionSource(chat, 'handleChatInputKeydown', 'updateMentionStateFromInput')}; this.handleChatInputKeydown = handleChatInputKeydown;`,
        context
    );
    return context;
}

test('聊天输入框按 Enter 发送并阻止原生换行', () => {
    const context = createKeydownHarness();
    let prevented = false;

    context.handleChatInputKeydown({
        key: 'Enter',
        shiftKey: false,
        isComposing: false,
        keyCode: 13,
        preventDefault() {
            prevented = true;
        },
    });

    assert.equal(prevented, true);
    assert.equal(context.sendCount, 1);
});

test('聊天输入框按 Shift+Enter 只换行且不发送', () => {
    const context = createKeydownHarness();
    let prevented = false;

    context.handleChatInputKeydown({
        key: 'Enter',
        shiftKey: true,
        isComposing: false,
        keyCode: 13,
        preventDefault() {
            prevented = true;
        },
    });

    assert.equal(prevented, false);
    assert.equal(context.sendCount, 0);
});

test('输入法确认候选词时按 Enter 不会发送', () => {
    const context = createKeydownHarness();

    context.handleChatInputKeydown({
        key: 'Enter',
        shiftKey: false,
        isComposing: true,
        keyCode: 229,
        preventDefault() {
            throw new Error('IME Enter should not be prevented');
        },
    });

    assert.equal(context.sendCount, 0);
});
