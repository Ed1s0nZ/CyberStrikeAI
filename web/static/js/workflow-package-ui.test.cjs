const fs = require('node:fs');
const test = require('node:test');
const assert = require('node:assert/strict');

test('图编排提供导入、导出和覆盖确认容器', () => {
    const html = fs.readFileSync('web/templates/index.html', 'utf8');
    const zh = JSON.parse(fs.readFileSync('web/static/i18n/zh-CN.json', 'utf8'));
    assert.match(html, /onclick="openWorkflowPackageImportModal\(\)"/);
    assert.match(html, /onclick="exportCurrentWorkflowPackage\(\)"/);
    assert.match(html, /id="workflow-package-import-modal"/);
    assert.match(html, /id="workflow-package-overwrite-modal"/);
    assert.equal(zh.workflows.package.importLocal, '导入本地包');
});

test('工作流脚本调用包契约的全部端点与冲突错误码', () => {
    const workflows = fs.readFileSync('web/static/js/workflows.js', 'utf8');
    const client = fs.readFileSync('web/static/js/workflow-package-client.js', 'utf8');
    assert.match(workflows, /\/api\/workflows\/\$\{encodeURIComponent\(id\)\}\/package/);
    assert.match(client, /\/api\/workflow-package-inspections/);
    assert.match(client, /\/api\/workflow-package-imports/);
    assert.match(workflows, /WFPKG_CONFLICT_CHANGED/);
    assert.match(workflows, /WFPKG_INSPECTION_EXPIRED/);
});

test('预检无效包的契约错误码都有中文状态分支', () => {
    const workflows = fs.readFileSync('web/static/js/workflows.js', 'utf8');
    [
        'WFPKG_INVALID_ARCHIVE',
        'WFPKG_UNSUPPORTED_FORMAT',
        'WFPKG_INVALID_MANIFEST',
        'WFPKG_CHECKSUM_MISMATCH',
        'WFPKG_MULTIPLE_WORKFLOWS',
        'WFPKG_WORKFLOW_INVALID'
    ].forEach((code) => assert.match(workflows, new RegExp(code)));
});

test('导入请求标识生成失败会进入中文错误处理', () => {
    const workflows = fs.readFileSync('web/static/js/workflows.js', 'utf8');
    assert.match(workflows, /async function performWorkflowPackageImport\(request\)[\s\S]*?try\s*\{[\s\S]*?client\.createIdempotencyKey\(\)[\s\S]*?catch \(error\) \{\s*displayWorkflowPackageError\(error\)/);
});

test('工作流包动态状态同时提供中文和英文词条', () => {
    const zh = JSON.parse(fs.readFileSync('web/static/i18n/zh-CN.json', 'utf8'));
    const en = JSON.parse(fs.readFileSync('web/static/i18n/en-US.json', 'utf8'));
    const keys = [
        ['errors', 'invalidArchive'],
        ['conflict', 'idConflict'],
        ['summary', 'workflowName'],
        ['resolution', 'keepExisting'],
        ['result', 'overwritten']
    ];
    keys.forEach(([section, key]) => {
        assert.equal(typeof zh.workflows.package[section][key], 'string');
        assert.equal(typeof en.workflows.package[section][key], 'string');
    });
});

test('语言切换会刷新工作流包弹窗及其动态状态', () => {
    const workflows = fs.readFileSync('web/static/js/workflows.js', 'utf8');
    assert.match(workflows, /function refreshWorkflowsI18n\(\)[\s\S]*?workflow-package-import-modal[\s\S]*?workflow-package-overwrite-modal[\s\S]*?renderWorkflowPackageInspection\(\)[\s\S]*?renderWorkflowPackageResolution\(\)/);
});

test('每次打开导入弹窗都会开始新的导入会话', () => {
    const workflows = fs.readFileSync('web/static/js/workflows.js', 'utf8');
    const start = workflows.indexOf('window.openWorkflowPackageImportModal = async function ()');
    const end = workflows.indexOf('window.closeWorkflowPackageImportModal = function ()', start);
    const openHandler = workflows.slice(start, end);
    assert.match(openHandler, /resetWorkflowPackageImport\(\);/);
    assert.doesNotMatch(openHandler, /restoreWorkflowPackageState\(\)/);
});
