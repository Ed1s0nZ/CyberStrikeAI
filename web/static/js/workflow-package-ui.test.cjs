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
