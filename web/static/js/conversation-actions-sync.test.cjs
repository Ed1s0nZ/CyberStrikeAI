const fs = require('node:fs');
const test = require('node:test');
const assert = require('node:assert/strict');

const chat = fs.readFileSync('web/static/js/chat.js', 'utf8');
const projects = fs.readFileSync('web/static/js/projects.js', 'utf8');
const template = fs.readFileSync('web/templates/index.html', 'utf8');

function functionSource(source, name, nextName) {
    const start = source.indexOf(`function ${name}(`);
    const end = source.indexOf(`function ${nextName}(`, start);
    assert.notEqual(start, -1, `${name} should exist`);
    assert.notEqual(end, -1, `${nextName} should follow ${name}`);
    return source.slice(start, end);
}

test('全局置顶检查接口结果并即时通知项目文件夹', () => {
    const source = functionSource(chat, 'pinConversation', 'showMoveToGroupSubmenu');

    assert.match(source, /assertConversationActionResponse\(updateResponse, '更新置顶状态失败'\)/);
    assert.match(source, /notifyConversationPinnedChanged\(convId, newPinned\)/);
    assert.match(source, /loadConversationsWithGroups\(\)/);
});

test('项目文件夹内置顶对话优先排序并显示图钉', () => {
    const sortSource = functionSource(projects, 'sortProjectFolderConversations', 'updateChatProjectConversationPinnedState');
    const itemSource = functionSource(projects, 'appendChatProjectConversationItem', 'selectChatProjectConversationItem');

    assert.match(sortSource, /Number\(!!b\?\.pinned\) - Number\(!!a\?\.pinned\)/);
    assert.match(itemSource, /if \(conversation\.pinned\)/);
    assert.match(itemSource, /project-conversation-pinned/);
});

test('删除事件立即移除项目缓存并触发权威刷新', () => {
    const removeSource = functionSource(projects, 'removeChatProjectConversation', 'refreshChatProjectFoldersAfterAction');

    assert.match(removeSource, /chatProjectFolderContext\.conversations = chatProjectFolderContext\.conversations\.filter/);
    assert.match(projects, /document\.addEventListener\('conversation-deleted',[\s\S]{0,300}removeChatProjectConversation\(conversationId\)[\s\S]{0,180}refreshChatProjectFoldersAfterAction\(\)/);
    assert.match(chat, /document\.dispatchEvent\(new CustomEvent\('conversation-deleted'/);
});

test('较旧的项目文件夹请求不能覆盖较新的操作结果', () => {
    const source = functionSource(projects, 'loadChatProjectFolderContext', 'getProjectConversationSortTime');

    assert.match(source, /const loadSeq = \+\+chatProjectFolderContextLoadSeq/);
    assert.match(source, /if \(loadSeq !== chatProjectFolderContextLoadSeq\) return false/);
});

test('项目文件夹菜单可以置顶并立即更新排序', () => {
    const toggleSource = functionSource(projects, 'toggleProjectPinnedFromListMenu', 'initProjectListActionMenu');
    const folderSource = functionSource(projects, 'appendChatProjectFolderItem', 'appendChatProjectConversationItem');

    assert.match(template, /onclick="toggleProjectPinnedFromListMenu\(\)"/);
    assert.match(toggleSource, /JSON\.stringify\(\{ pinned: nextPinned \}\)/);
    assert.match(toggleSource, /updateCachedProjectPinnedState\(projectId, nextPinned\)/);
    assert.match(folderSource, /if \(!isUnassigned && project\.pinned\)/);
    assert.match(folderSource, /project-folder-pinned/);
    assert.match(projects, /\[\.\.\.pinnedProjects, unassignedProject, \.\.\.regularProjects\]/);
});

test('对话侧栏不再显示对话分组区域', () => {
    assert.doesNotMatch(template, /class="conversation-groups-section"/);
    assert.doesNotMatch(template, /id="conversation-groups-list"/);
});

test('删除对话分组检查接口结果并先清理本地状态', () => {
    const deleteSource = functionSource(chat, 'deleteConversationGroupById', 'deleteGroup');
    const contextSource = functionSource(chat, 'deleteGroupFromContext', 'closeGroupContextMenu');

    assert.match(deleteSource, /assertConversationActionResponse\(deleteResponse, '删除分组失败'\)/);
    assert.match(deleteSource, /removeConversationGroupFromLocalState\(groupId\)/);
    assert.match(deleteSource, /if \(currentGroupId === groupId\) exitGroupDetail\(\)/);
    assert.match(contextSource, /deleteConversationGroupById\(groupId, \{ closeContextMenu: true \}\)/);
});
