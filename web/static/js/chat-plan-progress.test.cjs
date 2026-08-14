const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');

const { deriveProgress, applyTaskUpdate } = require('./chat-plan-progress.js');

test('任务进度优先定位进行中步骤并保留完成项', () => {
    const progress = deriveProgress([
        { id: '1', subject: '梳理需求', status: 'completed' },
        { id: '2', subject: '实现组件', activeForm: '正在实现组件', status: 'in_progress' },
        { id: '3', subject: '浏览器验证', status: 'pending' }
    ]);
    assert.equal(progress.activeStep, 2);
    assert.equal(progress.completed, 1);
    assert.equal(progress.total, 3);
    assert.equal(progress.allCompleted, false);
});

test('TaskUpdate 成功后即时勾选，最终步骤显示全部完成', () => {
    const initial = [
        { id: '1', subject: '接口', status: 'completed' },
        { id: '2', subject: '界面', status: 'in_progress' }
    ];
    const updated = applyTaskUpdate(initial, { taskId: '2', status: 'completed' });
    const progress = deriveProgress(updated);
    assert.equal(progress.activeStep, 2);
    assert.equal(progress.completed, 2);
    assert.equal(progress.allCompleted, true);
});

test('删除任务不会出现在悬浮清单中', () => {
    const tasks = applyTaskUpdate([
        { id: '1', subject: '保留', status: 'pending' },
        { id: '2', subject: '删除', status: 'pending' }
    ], { taskId: '2', status: 'deleted' });
    assert.deepEqual(tasks.map((task) => task.id), ['1']);
});

test('任务进度样式跟随系统主题变量而非固定深色', () => {
    const css = fs.readFileSync('web/static/css/chat-plan-progress.css', 'utf8');
    assert.match(css, /--agent-plan-surface:\s*var\(--card-bg\)/);
    assert.match(css, /background:\s*var\(--agent-plan-surface\)/);
    assert.match(css, /color:\s*var\(--agent-plan-text\)/);
    assert.doesNotMatch(css, /background:\s*#(?:292929|2b2b2b|303030)/i);
});

test('回到最新按钮与任务进度同时显示时采用上下避让布局', () => {
    const css = fs.readFileSync('web/static/css/chat-plan-progress.css', 'utf8');
    const template = fs.readFileSync('web/templates/index.html', 'utf8');
    assert.match(css, /\.chat-return-latest:not\(\[hidden\]\)\s*\+\s*\.agent-plan-progress:not\(\[hidden\]\)/);
    assert.match(css, /--agent-plan-trigger-bottom:\s*64px/);
    assert.match(css, /--agent-plan-panel-bottom:\s*120px/);
    assert.match(css, /bottom:\s*var\(--agent-plan-trigger-bottom\)/);
    assert.match(css, /bottom:\s*var\(--agent-plan-panel-bottom\)/);
    assert.match(template, /<button[^>]+id="chat-return-latest"[\s\S]*?<\/button>\s*<div id="agent-plan-progress"/);
});

test('计划详情只在真实鼠标移动或主动操作后展开', () => {
    const css = fs.readFileSync('web/static/css/chat-plan-progress.css', 'utf8');
    const source = fs.readFileSync('web/static/js/chat-plan-progress.js', 'utf8');
    assert.doesNotMatch(css, /\.agent-plan-progress:hover\s+\.agent-plan-progress-panel/);
    assert.match(css, /\.agent-plan-progress\.is-hover-active\s+\.agent-plan-progress-panel/);
    assert.match(source, /host\.addEventListener\('pointermove',\s*armHoverAfterPointerMove\)/);
    assert.match(source, /returnLatestButton\.addEventListener\('pointerdown',\s*disarmPassiveHover\)/);
    assert.match(source, /passiveHoverAnchor = \{ x: event\.clientX, y: event\.clientY \}/);
    assert.match(source, /event\.clientX === passiveHoverAnchor\.x && event\.clientY === passiveHoverAnchor\.y/);
    assert.match(source, /host\.classList\.remove\('is-hover-active'\)/);
});

test('服务端判定任务停止后立即清空旧任务卡片', () => {
    const source = fs.readFileSync('web/static/js/chat-plan-progress.js', 'utf8');
    assert.match(source, /payload && payload\.running === false/);
    assert.match(source, /state\.tasks = \[\][\s\S]{0,160}state\.expanded = false/);
});
