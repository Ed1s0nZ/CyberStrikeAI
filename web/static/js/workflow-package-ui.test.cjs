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
