const fs = require('node:fs');
const vm = require('node:vm');
const test = require('node:test');
const assert = require('node:assert/strict');

const chat = fs.readFileSync('web/static/js/chat.js', 'utf8');

function timingSource(source) {
    const start = source.indexOf('function formatAssistantTurnDuration(');
    const end = source.indexOf('window.setAssistantTurnTiming = setAssistantTurnTiming;', start);
    assert.notEqual(start, -1, 'formatAssistantTurnDuration should exist');
    assert.notEqual(end, -1, 'timing exports should exist');
    return source.slice(start, end);
}

function createClassList() {
    return {
        toggle() {},
    };
}

function createMessage() {
    const label = {
        innerHTML: '',
        classList: createClassList(),
        setAttribute() {},
    };
    return {
        dataset: {},
        querySelector(selector) {
            if (selector === '.mcp-call-label.turn-process-summary') return label;
            return null;
        },
        label,
    };
}

function createHarness(nowMs) {
    const RealDate = Date;
    class TestDate extends RealDate {
        static now() {
            return nowMs;
        }
    }
    const context = {
        Date: TestDate,
        Number,
        Math,
        String,
        document: {
            querySelector() { return null; },
            querySelectorAll() { return []; },
            getElementById() { return null; },
        },
        window: {},
        escapeHtml(value) { return String(value); },
        setInterval() { return 1; },
        clearInterval() {},
    };
    vm.runInNewContext(
        `${timingSource(chat)}; this.setAssistantTurnTiming = setAssistantTurnTiming;`,
        context
    );
    return context;
}

test('刷新运行中任务时忽略摘要中的零耗时并按开始时间恢复', () => {
    const startedAt = '2026-08-12T02:00:00.000Z';
    const startedMs = Date.parse(startedAt);
    const context = createHarness(startedMs + 65_000);
    const message = createMessage();

    context.setAssistantTurnTiming(message, {
        startedAt,
        durationMs: 0,
        status: 'running',
    });

    assert.equal(message.dataset.turnDurationMs, undefined);
    assert.match(message.label.innerHTML, /已处理 1 分钟 5 秒/);
});

test('已完成任务仍优先使用持久化耗时', () => {
    const context = createHarness(Date.parse('2026-08-12T02:05:00.000Z'));
    const message = createMessage();

    context.setAssistantTurnTiming(message, {
        startedAt: '2026-08-12T02:00:00.000Z',
        completedAt: '2026-08-12T02:01:05.000Z',
        durationMs: 65_000,
        status: 'completed',
    });

    assert.equal(message.dataset.turnDurationMs, '65000');
    assert.match(message.label.innerHTML, /耗时 1 分钟 5 秒/);
});
