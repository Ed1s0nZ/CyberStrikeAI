const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');

test('tool_result 合并回工具调用卡时会回填空参数', () => {
    const source = fs.readFileSync('web/static/js/monitor.js', 'utf8');
    assert.match(source, /function toolCallArgsEmpty\(args\)/);
    assert.match(source, /const resultArgs = parseToolCallArgsFromData\(data\);/);
    assert.match(source, /toolCallArgsEmpty\(state\.args\) && !toolCallArgsEmpty\(resultArgs\)/);
    assert.match(source, /state\.args = resultArgs;/);
});

test('历史 process_details 合并时也会从 tool_result 补齐 tool_call 参数', () => {
    const source = fs.readFileSync('web/static/js/monitor.js', 'utf8');
    assert.match(source, /function absorbResult\(targetDetail, resultDetail\)/);
    assert.match(source, /targetDetail\.data\.argumentsObj = resultArgs;/);
    assert.match(source, /targetDetail\.data\.arguments = JSON\.stringify\(resultArgs\);/);
});

test('同一 toolCallId 的多次调用按 FIFO 合并结果，避免后一次覆盖导致结果记录缺失', () => {
    const source = fs.readFileSync('web/static/js/monitor.js', 'utf8');
    assert.match(source, /list\.push\(copy\)/);
    assert.match(source, /const candidate = list\.shift\(\)/);
    assert.match(source, /callName === resultName/);
});
