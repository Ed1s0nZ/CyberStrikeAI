const fs = require('node:fs');
const test = require('node:test');
const assert = require('node:assert/strict');

const projects = fs.readFileSync('web/static/js/projects.js', 'utf8');
const styles = fs.readFileSync('web/static/css/style.css', 'utf8');
const chat = fs.readFileSync('web/static/js/chat.js', 'utf8');
const html = fs.readFileSync('web/templates/index.html', 'utf8');
const rbac = fs.readFileSync('web/static/js/rbac-guards.js', 'utf8');

function functionSource(source, name, nextName) {
    const start = source.indexOf(`function ${name}(`);
    const end = source.indexOf(`function ${nextName}(`, start);
    assert.notEqual(start, -1, `${name} should exist`);
    assert.notEqual(end, -1, `${nextName} should follow ${name}`);
    return source.slice(start, end);
}

test('无项目文件夹与普通项目共用悬浮和键盘聚焦预览', () => {
    const source = functionSource(projects, 'appendChatProjectFolderItem', 'appendChatProjectConversationItem');

    assert.match(source, /row\.addEventListener\('mouseenter', \(\) => scheduleShowProjectFolderPreview/);
    assert.match(source, /button\.addEventListener\('focus', \(\) => scheduleShowProjectFolderPreview/);
    assert.doesNotMatch(
        source,
        /if \(!isUnassigned\) \{\s*row\.addEventListener\('mouseenter', \(\) => scheduleShowProjectFolderPreview/
    );
});

test('无项目预览隐藏测试范围和编辑入口', () => {
    const source = functionSource(projects, 'showProjectFolderPreview', 'scheduleShowProjectFolderPreview');

    assert.match(source, /preview\.classList\.toggle\('is-unassigned', isUnassigned\)/);
    assert.match(source, /scopeRow\.hidden = isUnassigned \|\| !scope/);
    assert.match(source, /editButton\.hidden = isUnassigned/);
    assert.match(styles, /\.project-folder-preview\.is-unassigned \.project-folder-preview-edit\s*\{\s*display: none !important;/);
    assert.match(styles, /\.project-folder-preview\.is-unassigned \.project-folder-preview-details\s*\{\s*border-bottom: 0;/);
});

test('项目标题提供受权限保护的新建项目入口', () => {
    const source = functionSource(projects, 'showNewProjectModalFromChatSidebar', 'saveProjectModal');

    assert.match(html, /class="add-group-btn project-folders-add-btn"[\s\S]*?onclick="showNewProjectModalFromChatSidebar\(\)"/);
    assert.match(chat, /projectHeader\.querySelector\('\.project-folders-add-btn'\)/);
    assert.match(source, /window\._projectModalFromChat = false/);
    assert.match(source, /window\._projectModalFromChatSidebar = true/);
    assert.match(rbac, /showNewProjectModalFromChatSidebar: 'project:write'/);
});
