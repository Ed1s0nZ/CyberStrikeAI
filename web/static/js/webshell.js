// WebShell 管理（类似冰蝎/蚁剑：虚拟终端、文件管理、命令执行）

const WEBSHELL_SIDEBAR_WIDTH_KEY = 'webshell_sidebar_width';
const WEBSHELL_DEFAULT_SIDEBAR_WIDTH = 360;
/** 右侧主区域（终端/文件管理）最小宽度，避免拖到中间时右边变形 */
const WEBSHELL_MAIN_MIN_WIDTH = 380;
const WEBSHELL_PROMPT = 'shell> ';
let webshellConnections = [];
let currentWebshellId = null;
let webshellTerminalInstance = null;
let webshellTerminalFitAddon = null;
let webshellTerminalResizeObserver = null;
let webshellTerminalResizeContainer = null;
let webshellCurrentConn = null;
let webshellLineBuffer = '';
let webshellRunning = false;
// 按连接保存命令历史，用于上下键
let webshellHistoryByConn = {};
let webshellHistoryIndex = -1;
const WEBSHELL_HISTORY_MAX = 100;
// 清屏防重入：一次点击只执行一次（避免多次绑定或重复触发导致多个 shell>）
let webshellClearInProgress = false;
// AI 助手：按连接 ID 保存对话 ID，便于多轮对话
let webshellAiConvMap = {};
let webshellAiSending = false;
// 流式打字机效果：当前会话的 response 序号，用于中止过期的打字
let webshellStreamingTypingId = 0;

// 从服务端（SQLite）拉取连接列表
function getWebshellConnections() {
    if (typeof apiFetch === 'undefined') {
        return Promise.resolve([]);
    }
    return apiFetch('/api/webshell/connections', { method: 'GET' })
        .then(function (r) { return r.json(); })
        .then(function (list) { return Array.isArray(list) ? list : []; })
        .catch(function (e) {
            console.warn('读取 WebShell 连接列表失败', e);
            return [];
        });
}

// 从服务端刷新连接列表并重绘侧栏
function refreshWebshellConnectionsFromServer() {
    return getWebshellConnections().then(function (list) {
        webshellConnections = list;
        renderWebshellList();
        return list;
    });
}

// 使用 wsT 避免与全局 window.t 冲突导致无限递归
function wsT(key) {
    var globalT = typeof window !== 'undefined' ? window.t : null;
    if (typeof globalT === 'function' && globalT !== wsT) return globalT(key);
    var fallback = {
        'webshell.title': 'WebShell 管理',
        'webshell.addConnection': '添加连接',
        'webshell.cmdParam': '命令参数名',
        'webshell.cmdParamPlaceholder': '不填默认为 cmd，如填 xxx 则请求为 xxx=命令',
        'webshell.connections': '连接列表',
        'webshell.noConnections': '暂无连接，请点击「添加连接」',
        'webshell.selectOrAdd': '请从左侧选择连接，或添加新的 WebShell 连接',
        'webshell.deleteConfirm': '确定要删除该连接吗？',
        'webshell.editConnection': '编辑',
        'webshell.editConnectionTitle': '编辑连接',
        'webshell.tabTerminal': '虚拟终端',
        'webshell.tabFileManager': '文件管理',
        'webshell.tabAiAssistant': 'AI 助手',
        'webshell.aiSystemReadyMessage': '系统已就绪。请输入您的测试需求，系统将自动执行相应的安全测试。',
        'webshell.aiPlaceholder': '例如：列出当前目录下的文件',
        'webshell.aiSend': '发送',
        'webshell.terminalWelcome': 'WebShell 虚拟终端 — 输入命令后按回车执行（Ctrl+L 清屏）',
        'webshell.quickCommands': '快捷命令',
        'webshell.downloadFile': '下载',
        'webshell.filePath': '当前路径',
        'webshell.listDir': '列出目录',
        'webshell.readFile': '读取',
        'webshell.editFile': '编辑',
        'webshell.deleteFile': '删除',
        'webshell.saveFile': '保存',
        'webshell.cancelEdit': '取消',
        'webshell.parentDir': '上级目录',
        'webshell.execError': '执行失败',
        'webshell.testConnectivity': '测试连通性',
        'webshell.testSuccess': '连通性正常，Shell 可访问',
        'webshell.testFailed': '连通性测试失败',
        'webshell.testNoExpectedOutput': 'Shell 返回了响应但未得到预期输出，请检查连接密码与命令参数名',
        'webshell.clearScreen': '清屏',
        'webshell.running': '执行中…',
        'webshell.waitFinish': '请等待当前命令执行完成',
        'webshell.newDir': '新建目录',
        'webshell.rename': '重命名',
        'webshell.upload': '上传',
        'webshell.newFile': '新建文件',
        'webshell.filterPlaceholder': '过滤文件名',
        'webshell.batchDelete': '批量删除',
        'webshell.batchDownload': '批量下载',
        'webshell.refresh': '刷新',
        'webshell.selectAll': '全选',
        'webshell.breadcrumbHome': '根',
        'common.delete': '删除',
        'common.refresh': '刷新'
    };
    return fallback[key] || key;
}

// 全局只绑定一次：清屏 = 销毁终端并重新创建，保证只出现一个 shell>（不依赖 xterm.clear()，避免某些环境下 clear 不生效或重复写入）
function bindWebshellClearOnce() {
    if (window._webshellClearBound) return;
    window._webshellClearBound = true;
    document.body.addEventListener('click', function (e) {
        var btn = e.target && (e.target.id === 'webshell-terminal-clear' ? e.target : e.target.closest ? e.target.closest('#webshell-terminal-clear') : null);
        if (!btn || !webshellCurrentConn) return;
        e.preventDefault();
        e.stopPropagation();
        if (webshellClearInProgress) return;
        webshellClearInProgress = true;
        try {
            destroyWebshellTerminal();
            webshellLineBuffer = '';
            webshellHistoryIndex = -1;
            initWebshellTerminal(webshellCurrentConn);
        } finally {
            setTimeout(function () { webshellClearInProgress = false; }, 100);
        }
    }, true);
}

// 初始化 WebShell 管理页面（从 SQLite 拉取连接列表）
function initWebshellPage() {
    bindWebshellClearOnce();
    destroyWebshellTerminal();
    webshellCurrentConn = null;
    currentWebshellId = null;
    webshellConnections = [];
    renderWebshellList();
    applyWebshellSidebarWidth();
    initWebshellSidebarResize();
    const workspace = document.getElementById('webshell-workspace');
    if (workspace) {
        workspace.innerHTML = '<div class="webshell-workspace-placeholder" data-i18n="webshell.selectOrAdd">' + (wsT('webshell.selectOrAdd')) + '</div>';
    }
    getWebshellConnections().then(function (list) {
        webshellConnections = list;
        renderWebshellList();
    });
}

function getWebshellSidebarWidth() {
    try {
        const w = parseInt(localStorage.getItem(WEBSHELL_SIDEBAR_WIDTH_KEY), 10);
        if (!isNaN(w) && w >= 260 && w <= 800) return w;
    } catch (e) {}
    return WEBSHELL_DEFAULT_SIDEBAR_WIDTH;
}

function setWebshellSidebarWidth(px) {
    localStorage.setItem(WEBSHELL_SIDEBAR_WIDTH_KEY, String(px));
}

function applyWebshellSidebarWidth() {
    const sidebar = document.getElementById('webshell-sidebar');
    if (!sidebar) return;
    const parentW = sidebar.parentElement ? sidebar.parentElement.offsetWidth : 0;
    let w = getWebshellSidebarWidth();
    if (parentW > 0) w = Math.min(w, Math.max(260, parentW - WEBSHELL_MAIN_MIN_WIDTH));
    sidebar.style.width = w + 'px';
}

function initWebshellSidebarResize() {
    const handle = document.getElementById('webshell-resize-handle');
    const sidebar = document.getElementById('webshell-sidebar');
    if (!handle || !sidebar || handle.dataset.resizeBound === '1') return;
    handle.dataset.resizeBound = '1';
    let startX = 0, startW = 0;
    function onMove(e) {
        const dx = e.clientX - startX;
        let w = Math.round(startW + dx);
        const parentW = sidebar.parentElement ? sidebar.parentElement.offsetWidth : 800;
        const min = 260;
        const max = Math.min(800, parentW - WEBSHELL_MAIN_MIN_WIDTH);
        w = Math.max(min, Math.min(max, w));
        sidebar.style.width = w + 'px';
    }
    function onUp() {
        handle.classList.remove('active');
        document.body.style.cursor = '';
        document.body.style.userSelect = '';
        document.removeEventListener('mousemove', onMove);
        document.removeEventListener('mouseup', onUp);
        setWebshellSidebarWidth(parseInt(sidebar.style.width, 10) || WEBSHELL_DEFAULT_SIDEBAR_WIDTH);
    }
    handle.addEventListener('mousedown', function (e) {
        if (e.button !== 0) return;
        e.preventDefault();
        startX = e.clientX;
        startW = sidebar.offsetWidth;
        handle.classList.add('active');
        document.body.style.cursor = 'col-resize';
        document.body.style.userSelect = 'none';
        document.addEventListener('mousemove', onMove);
        document.addEventListener('mouseup', onUp);
    });
}

// 销毁当前终端实例（切换连接或离开页面时）
function destroyWebshellTerminal() {
    if (webshellTerminalResizeObserver && webshellTerminalResizeContainer) {
        try { webshellTerminalResizeObserver.unobserve(webshellTerminalResizeContainer); } catch (e) {}
        webshellTerminalResizeObserver = null;
        webshellTerminalResizeContainer = null;
    }
    if (webshellTerminalInstance) {
        try {
            webshellTerminalInstance.dispose();
        } catch (e) {}
        webshellTerminalInstance = null;
    }
    webshellTerminalFitAddon = null;
    webshellLineBuffer = '';
    webshellRunning = false;
}

// 渲染连接列表
function renderWebshellList() {
    const listEl = document.getElementById('webshell-list');
    if (!listEl) return;

    if (!webshellConnections.length) {
        listEl.innerHTML = '<div class="webshell-empty" data-i18n="webshell.noConnections">' + (wsT('webshell.noConnections')) + '</div>';
        return;
    }

    listEl.innerHTML = webshellConnections.map(conn => {
        const remark = (conn.remark || conn.url || '').replace(/</g, '&lt;').replace(/>/g, '&gt;');
        const url = (conn.url || '').replace(/</g, '&lt;').replace(/>/g, '&gt;');
        const urlTitle = (conn.url || '').replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;');
        const active = currentWebshellId === conn.id ? ' active' : '';
        const safeId = escapeHtml(conn.id);
        return (
            '<div class="webshell-item' + active + '" data-id="' + safeId + '">' +
            '<div class="webshell-item-remark" title="' + urlTitle + '">' + remark + '</div>' +
            '<div class="webshell-item-url" title="' + urlTitle + '">' + url + '</div>' +
            '<div class="webshell-item-actions">' +
            '<button type="button" class="btn-ghost btn-sm webshell-edit-conn-btn" data-id="' + safeId + '" title="' + wsT('webshell.editConnection') + '">' + wsT('webshell.editConnection') + '</button> ' +
            '<button type="button" class="btn-ghost btn-sm webshell-delete-btn" data-id="' + safeId + '" title="' + wsT('common.delete') + '">' + wsT('common.delete') + '</button>' +
            '</div>' +
            '</div>'
        );
    }).join('');

    listEl.querySelectorAll('.webshell-item').forEach(el => {
        el.addEventListener('click', function (e) {
            if (e.target.closest('.webshell-delete-btn') || e.target.closest('.webshell-edit-conn-btn')) return;
            selectWebshell(el.getAttribute('data-id'));
        });
    });
    listEl.querySelectorAll('.webshell-edit-conn-btn').forEach(btn => {
        btn.addEventListener('click', function (e) {
            e.stopPropagation();
            showEditWebshellModal(btn.getAttribute('data-id'));
        });
    });
    listEl.querySelectorAll('.webshell-delete-btn').forEach(btn => {
        btn.addEventListener('click', function (e) {
            e.stopPropagation();
            deleteWebshell(btn.getAttribute('data-id'));
        });
    });
}

function escapeHtml(s) {
    if (!s) return '';
    const div = document.createElement('div');
    div.textContent = s;
    return div.innerHTML;
}

function formatWebshellAiConvDate(updatedAt) {
    if (!updatedAt) return '';
    var d = typeof updatedAt === 'string' ? new Date(updatedAt) : updatedAt;
    if (isNaN(d.getTime())) return '';
    var now = new Date();
    var sameDay = d.getDate() === now.getDate() && d.getMonth() === now.getMonth() && d.getFullYear() === now.getFullYear();
    if (sameDay) return d.getHours() + ':' + String(d.getMinutes()).padStart(2, '0');
    return (d.getMonth() + 1) + '/' + d.getDate();
}

// 根据后端保存的 processDetail 构建一条时间线项的 HTML（与 appendTimelineItem 展示一致）
function buildWebshellTimelineItemFromDetail(detail) {
    var eventType = detail.eventType || '';
    var title = detail.message || '';
    var data = detail.data || {};
    if (eventType === 'iteration') {
        title = (typeof window.t === 'function') ? window.t('chat.iterationRound', { n: data.iteration || 1 }) : ('第 ' + (data.iteration || 1) + ' 轮迭代');
    } else if (eventType === 'thinking') {
        title = '🤔 ' + ((typeof window.t === 'function') ? window.t('chat.aiThinking') : 'AI 思考');
    } else if (eventType === 'tool_calls_detected') {
        title = '🔧 ' + ((typeof window.t === 'function') ? window.t('chat.toolCallsDetected', { count: data.count || 0 }) : ('检测到 ' + (data.count || 0) + ' 个工具调用'));
    } else if (eventType === 'tool_call') {
        var tn = data.toolName || ((typeof window.t === 'function') ? window.t('chat.unknownTool') : '未知工具');
        var idx = data.index || 0;
        var total = data.total || 0;
        title = '🔧 ' + ((typeof window.t === 'function') ? window.t('chat.callTool', { name: tn, index: idx, total: total }) : ('调用: ' + tn + (total ? ' (' + idx + '/' + total + ')' : '')));
    } else if (eventType === 'tool_result') {
        var success = data.success !== false;
        var tname = data.toolName || '工具';
        title = (success ? '✅ ' : '❌ ') + ((typeof window.t === 'function') ? (success ? window.t('chat.toolExecComplete', { name: tname }) : window.t('chat.toolExecFailed', { name: tname })) : (tname + (success ? ' 执行完成' : ' 执行失败')));
    } else if (eventType === 'progress') {
        title = (typeof window.translateProgressMessage === 'function') ? window.translateProgressMessage(detail.message || '') : (detail.message || '');
    }
    var html = '<span class="webshell-ai-timeline-title">' + escapeHtml(title || '') + '</span>';
    if (eventType === 'tool_call' && data && (data.argumentsObj || data.arguments)) {
        try {
            var args = data.argumentsObj || (data.arguments ? JSON.parse(data.arguments) : null);
            if (args && typeof args === 'object') {
                var paramsLabel = (typeof window.t === 'function') ? window.t('timeline.params') : '参数:';
                html += '<div class="webshell-ai-timeline-msg"><div class="tool-arg-section"><strong>' + escapeHtml(paramsLabel) + '</strong><pre class="tool-args">' + escapeHtml(JSON.stringify(args, null, 2)) + '</pre></div></div>';
            }
        } catch (e) {}
    } else if (eventType === 'tool_result' && data) {
        var isError = data.isError || data.success === false;
        var noResultText = (typeof window.t === 'function') ? window.t('timeline.noResult') : '无结果';
        var result = data.result != null ? data.result : (data.error != null ? data.error : noResultText);
        var resultStr = (typeof result === 'string') ? result : JSON.stringify(result);
        var execResultLabel = (typeof window.t === 'function') ? window.t('timeline.executionResult') : '执行结果:';
        var execIdLabel = (typeof window.t === 'function') ? window.t('timeline.executionId') : '执行ID:';
        html += '<div class="webshell-ai-timeline-msg"><div class="tool-result-section ' + (isError ? 'error' : 'success') + '"><strong>' + escapeHtml(execResultLabel) + '</strong><pre class="tool-result">' + escapeHtml(resultStr) + '</pre>' + (data.executionId ? '<div class="tool-execution-id"><span>' + escapeHtml(execIdLabel) + '</span> <code>' + escapeHtml(String(data.executionId)) + '</code></div>' : '') + '</div></div>';
    } else if (detail.message && detail.message !== title) {
        html += '<div class="webshell-ai-timeline-msg">' + escapeHtml(detail.message) + '</div>';
    }
    return html;
}

// 渲染「执行过程及调用工具」折叠块（默认折叠，刷新后加载历史时保留并可展开）
function renderWebshellProcessDetailsBlock(processDetails, defaultCollapsed) {
    if (!processDetails || processDetails.length === 0) return null;
    var expandLabel = (typeof window.t === 'function') ? window.t('chat.expandDetail') : '展开详情';
    var collapseLabel = (typeof window.t === 'function') ? window.t('tasks.collapseDetail') : '收起详情';
    var headerLabel = (typeof window.t === 'function') ? (window.t('chat.penetrationTestDetail') || '执行过程及调用工具') : '执行过程及调用工具';
    var wrapper = document.createElement('div');
    wrapper.className = 'process-details-container webshell-ai-process-block';
    var collapsed = defaultCollapsed !== false;
    wrapper.innerHTML = '<button type="button" class="webshell-ai-process-toggle" aria-expanded="' + (!collapsed) + '">' + escapeHtml(headerLabel) + ' <span class="ws-toggle-icon">' + (collapsed ? '▶' : '▼') + '</span></button><div class="process-details-content"><div class="progress-timeline webshell-ai-timeline has-items' + (collapsed ? '' : ' expanded') + '"></div></div>';
    var timeline = wrapper.querySelector('.progress-timeline');
    processDetails.forEach(function (d) {
        var item = document.createElement('div');
        item.className = 'webshell-ai-timeline-item webshell-ai-timeline-' + (d.eventType || '');
        item.innerHTML = buildWebshellTimelineItemFromDetail(d);
        timeline.appendChild(item);
    });
    var toggleBtn = wrapper.querySelector('.webshell-ai-process-toggle');
    var toggleIcon = wrapper.querySelector('.ws-toggle-icon');
    toggleBtn.addEventListener('click', function () {
        var isExpanded = timeline.classList.contains('expanded');
        timeline.classList.toggle('expanded');
        toggleBtn.setAttribute('aria-expanded', !isExpanded);
        if (toggleIcon) toggleIcon.textContent = isExpanded ? '▶' : '▼';
    });
    return wrapper;
}

function fetchAndRenderWebshellAiConvList(conn, listEl) {
    if (!conn || !conn.id || !listEl || typeof apiFetch !== 'function') return Promise.resolve();
    return apiFetch('/api/webshell/connections/' + encodeURIComponent(conn.id) + '/ai-conversations', { method: 'GET' })
        .then(function (r) { return r.json(); })
        .then(function (list) {
            if (!Array.isArray(list)) list = [];
            listEl.innerHTML = '';
            list.forEach(function (item) {
                var row = document.createElement('div');
                row.className = 'webshell-ai-conv-item';
                row.dataset.convId = item.id;
                var title = (item.title || '').trim() || item.id.slice(0, 8);
                var dateStr = item.updatedAt ? formatWebshellAiConvDate(item.updatedAt) : '';
                row.innerHTML = '<span class="webshell-ai-conv-item-title">' + escapeHtml(title) + '</span><span class="webshell-ai-conv-item-date">' + escapeHtml(dateStr) + '</span>';
                if (webshellAiConvMap[conn.id] === item.id) row.classList.add('active');
                row.addEventListener('click', function () {
                    webshellAiConvListSelect(conn, item.id, document.getElementById('webshell-ai-messages'), listEl);
                });
                var delBtn = document.createElement('button');
                delBtn.type = 'button';
                delBtn.className = 'btn-ghost btn-sm webshell-ai-conv-del';
                delBtn.textContent = '×';
                delBtn.title = wsT('webshell.aiDeleteConversation') || '删除对话';
                delBtn.addEventListener('click', function (e) {
                    e.stopPropagation();
                    if (!confirm(wsT('webshell.aiDeleteConversationConfirm') || '确定删除该对话？')) return;
                    var deletedId = item.id;
                    apiFetch('/api/conversations/' + encodeURIComponent(deletedId), { method: 'DELETE' })
                        .then(function (r) {
                            if (r.ok) {
                                if (webshellAiConvMap[conn.id] === deletedId) {
                                    delete webshellAiConvMap[conn.id];
                                    var msgs = document.getElementById('webshell-ai-messages');
                                    if (msgs) msgs.innerHTML = '';
                                }
                                fetchAndRenderWebshellAiConvList(conn, listEl);
                                try {
                                    document.dispatchEvent(new CustomEvent('conversation-deleted', { detail: { conversationId: deletedId } }));
                                } catch (err) { /* ignore */ }
                            }
                        })
                        .catch(function (e) { console.warn('删除对话失败', e); });
                });
                row.appendChild(delBtn);
                listEl.appendChild(row);
            });
        })
        .catch(function (e) { console.warn('加载对话列表失败', e); });
}

function webshellAiConvListSelect(conn, convId, messagesContainer, listEl) {
    if (!conn || !convId || !messagesContainer) return;
    webshellAiConvMap[conn.id] = convId;
    if (listEl) listEl.querySelectorAll('.webshell-ai-conv-item').forEach(function (el) {
        el.classList.toggle('active', el.dataset.convId === convId);
    });
    if (typeof apiFetch !== 'function') return;
    apiFetch('/api/conversations/' + encodeURIComponent(convId), { method: 'GET' })
        .then(function (r) { return r.json(); })
        .then(function (data) {
            messagesContainer.innerHTML = '';
            var list = data.messages || [];
            list.forEach(function (msg) {
                var role = (msg.role || '').toLowerCase();
                var content = (msg.content || '').trim();
                if (!content && role !== 'assistant') return;
                var div = document.createElement('div');
                div.className = 'webshell-ai-msg ' + (role === 'user' ? 'user' : 'assistant');
                if (role === 'user') {
                    div.textContent = content;
                } else {
                    if (typeof formatMarkdown === 'function') {
                        div.innerHTML = formatMarkdown(content);
                    } else {
                        div.textContent = content;
                    }
                }
                messagesContainer.appendChild(div);
                if (role === 'assistant' && msg.processDetails && msg.processDetails.length > 0) {
                    var block = renderWebshellProcessDetailsBlock(msg.processDetails, true);
                    if (block) messagesContainer.appendChild(block);
                }
            });
            if (list.length === 0) {
                var readyMsg = wsT('webshell.aiSystemReadyMessage') || '系统已就绪。请输入您的测试需求，系统将自动执行相应的安全测试。';
                var readyDiv = document.createElement('div');
                readyDiv.className = 'webshell-ai-msg assistant';
                readyDiv.textContent = readyMsg;
                messagesContainer.appendChild(readyDiv);
            }
            messagesContainer.scrollTop = messagesContainer.scrollHeight;
        })
        .catch(function (e) { console.warn('加载对话失败', e); });
}

// 选择连接：渲染终端 + 文件管理 Tab，并初始化终端
function selectWebshell(id) {
    currentWebshellId = id;
    renderWebshellList();
    const conn = webshellConnections.find(c => c.id === id);
    const workspace = document.getElementById('webshell-workspace');
    if (!workspace) return;
    if (!conn) {
        workspace.innerHTML = '<div class="webshell-workspace-placeholder">' + wsT('webshell.selectOrAdd') + '</div>';
        return;
    }

    destroyWebshellTerminal();
    webshellCurrentConn = conn;

    workspace.innerHTML =
        '<div class="webshell-tabs">' +
        '<button type="button" class="webshell-tab active" data-tab="terminal">' + wsT('webshell.tabTerminal') + '</button>' +
        '<button type="button" class="webshell-tab" data-tab="file">' + wsT('webshell.tabFileManager') + '</button>' +
        '<button type="button" class="webshell-tab" data-tab="ai">' + (wsT('webshell.tabAiAssistant') || 'AI 助手') + '</button>' +
        '</div>' +
        '<div id="webshell-pane-terminal" class="webshell-pane active">' +
        '<div class="webshell-terminal-toolbar">' +
        '<button type="button" class="btn-ghost btn-sm" id="webshell-terminal-clear" title="' + (wsT('webshell.clearScreen') || '清屏') + '">' + (wsT('webshell.clearScreen') || '清屏') + '</button> ' +
        '<span class="webshell-quick-label">' + (wsT('webshell.quickCommands') || '快捷命令') + ':</span> ' +
        '<button type="button" class="btn-ghost btn-sm webshell-quick-cmd" data-cmd="whoami">whoami</button> ' +
        '<button type="button" class="btn-ghost btn-sm webshell-quick-cmd" data-cmd="id">id</button> ' +
        '<button type="button" class="btn-ghost btn-sm webshell-quick-cmd" data-cmd="pwd">pwd</button> ' +
        '<button type="button" class="btn-ghost btn-sm webshell-quick-cmd" data-cmd="ls -la">ls -la</button> ' +
        '<button type="button" class="btn-ghost btn-sm webshell-quick-cmd" data-cmd="uname -a">uname -a</button> ' +
        '<button type="button" class="btn-ghost btn-sm webshell-quick-cmd" data-cmd="ifconfig">ifconfig</button> ' +
        '<button type="button" class="btn-ghost btn-sm webshell-quick-cmd" data-cmd="ip a">ip a</button> ' +
        '<button type="button" class="btn-ghost btn-sm webshell-quick-cmd" data-cmd="env">env</button> ' +
        '<button type="button" class="btn-ghost btn-sm webshell-quick-cmd" data-cmd="hostname">hostname</button> ' +
        '<button type="button" class="btn-ghost btn-sm webshell-quick-cmd" data-cmd="ps aux">ps aux</button> ' +
        '<button type="button" class="btn-ghost btn-sm webshell-quick-cmd" data-cmd="netstat -tulnp">netstat</button>' +
        '</div>' +
        '<div id="webshell-terminal-container" class="webshell-terminal-container"></div>' +
        '</div>' +
        '<div id="webshell-pane-file" class="webshell-pane">' +
        '<div class="webshell-file-toolbar">' +
        '<div class="webshell-file-breadcrumb" id="webshell-file-breadcrumb"></div>' +
        '<label><span>' + wsT('webshell.filePath') + '</span> <input type="text" id="webshell-file-path" class="form-control" value="." /></label>' +
        '<input type="text" id="webshell-file-filter" class="form-control webshell-file-filter" placeholder="' + (wsT('webshell.filterPlaceholder') || '过滤文件名') + '" />' +
        '<button type="button" class="btn-secondary" id="webshell-list-dir">' + wsT('webshell.listDir') + '</button> ' +
        '<button type="button" class="btn-ghost" id="webshell-parent-dir">' + wsT('webshell.parentDir') + '</button> ' +
        '<button type="button" class="btn-ghost" id="webshell-file-refresh" title="' + (wsT('webshell.refresh') || '刷新') + '">' + (wsT('webshell.refresh') || '刷新') + '</button> ' +
        '<button type="button" class="btn-ghost" id="webshell-mkdir-btn">' + (wsT('webshell.newDir') || '新建目录') + '</button> ' +
        '<button type="button" class="btn-ghost" id="webshell-newfile-btn">' + (wsT('webshell.newFile') || '新建文件') + '</button> ' +
        '<button type="button" class="btn-ghost" id="webshell-upload-btn">' + (wsT('webshell.upload') || '上传') + '</button> ' +
        '<button type="button" class="btn-ghost" id="webshell-batch-delete-btn">' + (wsT('webshell.batchDelete') || '批量删除') + '</button> ' +
        '<button type="button" class="btn-ghost" id="webshell-batch-download-btn">' + (wsT('webshell.batchDownload') || '批量下载') + '</button>' +
        '</div>' +
        '<div id="webshell-file-list" class="webshell-file-list"></div>' +
        '</div>' +
        '<div id="webshell-pane-ai" class="webshell-pane webshell-pane-ai-with-sidebar">' +
        '<div class="webshell-ai-sidebar">' +
        '<button type="button" class="btn-primary btn-sm webshell-ai-new-btn" id="webshell-ai-new-conv">' + (wsT('webshell.aiNewConversation') || '新对话') + '</button>' +
        '<div class="webshell-ai-conv-list" id="webshell-ai-conv-list"></div>' +
        '</div>' +
        '<div class="webshell-ai-main">' +
        '<div id="webshell-ai-messages" class="webshell-ai-messages"></div>' +
        '<div class="webshell-ai-input-row">' +
        '<textarea id="webshell-ai-input" class="webshell-ai-input form-control" rows="2" placeholder="' + (wsT('webshell.aiPlaceholder') || '例如：列出当前目录下的文件') + '"></textarea>' +
        '<button type="button" class="btn-primary" id="webshell-ai-send">' + (wsT('webshell.aiSend') || '发送') + '</button>' +
        '</div>' +
        '</div>' +
        '</div>';

    // Tab 切换
    workspace.querySelectorAll('.webshell-tab').forEach(btn => {
        btn.addEventListener('click', function () {
            const tab = btn.getAttribute('data-tab');
            workspace.querySelectorAll('.webshell-tab').forEach(b => b.classList.remove('active'));
            workspace.querySelectorAll('.webshell-pane').forEach(p => p.classList.remove('active'));
            btn.classList.add('active');
            const pane = document.getElementById('webshell-pane-' + tab);
            if (pane) pane.classList.add('active');
            if (tab === 'terminal' && webshellTerminalInstance && webshellTerminalFitAddon) {
                try { webshellTerminalFitAddon.fit(); } catch (e) {}
            }
        });
    });

    // 文件管理：列出目录、上级目录
    const pathInput = document.getElementById('webshell-file-path');
    document.getElementById('webshell-list-dir').addEventListener('click', function () {
        // 点击时用当前连接，编辑保存后立即生效
        webshellFileListDir(webshellCurrentConn, pathInput ? pathInput.value.trim() || '.' : '.');
    });
    document.getElementById('webshell-parent-dir').addEventListener('click', function () {
        const p = (pathInput && pathInput.value.trim()) || '.';
        if (p === '.' || p === '/') {
            pathInput.value = '..';
        } else {
            pathInput.value = p.replace(/\/[^/]+$/, '') || '.';
        }
        webshellFileListDir(webshellCurrentConn, pathInput.value || '.');
    });

    // 清屏由 bindWebshellClearOnce 统一事件委托处理，此处不再绑定，避免重复绑定导致一次点击出现多个 shell>
    // 快捷命令：点击后执行并输出到终端
    workspace.querySelectorAll('.webshell-quick-cmd').forEach(function (btn) {
        btn.addEventListener('click', function () {
            var cmd = btn.getAttribute('data-cmd');
            if (cmd) runQuickCommand(cmd);
        });
    });
    // 文件：刷新、新建目录、新建文件、上传、批量操作
    var filterInput = document.getElementById('webshell-file-filter');
    document.getElementById('webshell-file-refresh').addEventListener('click', function () {
        webshellFileListDir(webshellCurrentConn, pathInput ? pathInput.value.trim() || '.' : '.');
    });
    if (filterInput) filterInput.addEventListener('input', function () {
        webshellFileListApplyFilter();
    });
    document.getElementById('webshell-mkdir-btn').addEventListener('click', function () { webshellFileMkdir(webshellCurrentConn, pathInput); });
    document.getElementById('webshell-newfile-btn').addEventListener('click', function () { webshellFileNewFile(webshellCurrentConn, pathInput); });
    document.getElementById('webshell-upload-btn').addEventListener('click', function () { webshellFileUpload(webshellCurrentConn, pathInput); });
    document.getElementById('webshell-batch-delete-btn').addEventListener('click', function () { webshellBatchDelete(webshellCurrentConn, pathInput); });
    document.getElementById('webshell-batch-download-btn').addEventListener('click', function () { webshellBatchDownload(webshellCurrentConn, pathInput); });

    // AI 助手：侧边栏对话列表 + 主区消息
    var aiInput = document.getElementById('webshell-ai-input');
    var aiSendBtn = document.getElementById('webshell-ai-send');
    var aiMessages = document.getElementById('webshell-ai-messages');
    var aiNewConvBtn = document.getElementById('webshell-ai-new-conv');
    var aiConvListEl = document.getElementById('webshell-ai-conv-list');

    if (aiNewConvBtn) {
        aiNewConvBtn.addEventListener('click', function () {
            delete webshellAiConvMap[conn.id];
            if (aiMessages) {
                aiMessages.innerHTML = '';
                var readyMsg = wsT('webshell.aiSystemReadyMessage') || '系统已就绪。请输入您的测试需求，系统将自动执行相应的安全测试。';
                var div = document.createElement('div');
                div.className = 'webshell-ai-msg assistant';
                div.textContent = readyMsg;
                aiMessages.appendChild(div);
            }
            if (aiConvListEl) aiConvListEl.querySelectorAll('.webshell-ai-conv-item').forEach(function (el) { el.classList.remove('active'); });
        });
    }
    if (aiSendBtn && aiInput && aiMessages) {
        aiSendBtn.addEventListener('click', function () { runWebshellAiSend(conn, aiInput, aiSendBtn, aiMessages); });
        aiInput.addEventListener('keydown', function (e) {
            if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                runWebshellAiSend(conn, aiInput, aiSendBtn, aiMessages);
            }
        });
        fetchAndRenderWebshellAiConvList(conn, aiConvListEl).then(function () {
            loadWebshellAiHistory(conn, aiMessages).then(function () {
                if (webshellAiConvMap[conn.id] && aiConvListEl) {
                    aiConvListEl.querySelectorAll('.webshell-ai-conv-item').forEach(function (el) {
                        el.classList.toggle('active', el.dataset.convId === webshellAiConvMap[conn.id]);
                    });
                }
            });
        });
    }

    initWebshellTerminal(conn);
}

// 加载 WebShell 连接的 AI 助手对话历史（持久化展示），返回 Promise 供 .then 更新工具栏等；含 processDetails 时渲染折叠的「执行过程及调用工具」
function loadWebshellAiHistory(conn, messagesContainer) {
    if (!conn || !conn.id || !messagesContainer) return Promise.resolve();
    if (typeof apiFetch !== 'function') return Promise.resolve();
    return apiFetch('/api/webshell/connections/' + encodeURIComponent(conn.id) + '/ai-history', { method: 'GET' })
        .then(function (r) { return r.json(); })
        .then(function (data) {
            if (data.conversationId) webshellAiConvMap[conn.id] = data.conversationId;
            var list = Array.isArray(data.messages) ? data.messages : [];
            list.forEach(function (msg) {
                var role = (msg.role || '').toLowerCase();
                var content = (msg.content || '').trim();
                if (!content && role !== 'assistant') return;
                var div = document.createElement('div');
                div.className = 'webshell-ai-msg ' + (role === 'user' ? 'user' : 'assistant');
                if (role === 'user') {
                    div.textContent = content;
                } else {
                    if (typeof formatMarkdown === 'function') {
                        div.innerHTML = formatMarkdown(content);
                    } else {
                        div.textContent = content;
                    }
                }
                messagesContainer.appendChild(div);
                if (role === 'assistant' && msg.processDetails && msg.processDetails.length > 0) {
                    var block = renderWebshellProcessDetailsBlock(msg.processDetails, true);
                    if (block) messagesContainer.appendChild(block);
                }
            });
            if (list.length === 0) {
                var readyMsg = wsT('webshell.aiSystemReadyMessage') || '系统已就绪。请输入您的测试需求，系统将自动执行相应的安全测试。';
                var readyDiv = document.createElement('div');
                readyDiv.className = 'webshell-ai-msg assistant';
                readyDiv.textContent = readyMsg;
                messagesContainer.appendChild(readyDiv);
            }
            messagesContainer.scrollTop = messagesContainer.scrollHeight;
        })
        .catch(function (e) {
            console.warn('加载 WebShell AI 历史失败', conn.id, e);
        });
}

function runWebshellAiSend(conn, inputEl, sendBtn, messagesContainer) {
    if (!conn || !conn.id) return;
    var message = (inputEl && inputEl.value || '').trim();
    if (!message) return;
    if (webshellAiSending) return;
    if (typeof apiFetch !== 'function') {
        if (messagesContainer) {
            var errDiv = document.createElement('div');
            errDiv.className = 'webshell-ai-msg assistant';
            errDiv.textContent = '无法发送：未登录或 apiFetch 不可用';
            messagesContainer.appendChild(errDiv);
            messagesContainer.scrollTop = messagesContainer.scrollHeight;
        }
        return;
    }

    webshellAiSending = true;
    if (sendBtn) sendBtn.disabled = true;

    var userDiv = document.createElement('div');
    userDiv.className = 'webshell-ai-msg user';
    userDiv.textContent = message;
    messagesContainer.appendChild(userDiv);

    var timelineContainer = document.createElement('div');
    timelineContainer.className = 'webshell-ai-timeline';
    timelineContainer.setAttribute('aria-live', 'polite');

    var assistantDiv = document.createElement('div');
    assistantDiv.className = 'webshell-ai-msg assistant';
    assistantDiv.textContent = '…';
    messagesContainer.appendChild(timelineContainer);
    messagesContainer.appendChild(assistantDiv);
    messagesContainer.scrollTop = messagesContainer.scrollHeight;

    function appendTimelineItem(type, title, message, data) {
        var item = document.createElement('div');
        item.className = 'webshell-ai-timeline-item webshell-ai-timeline-' + type;

        var html = '<span class="webshell-ai-timeline-title">' + escapeHtml(title || message || '') + '</span>';

        // 工具调用入参
        if (type === 'tool_call' && data) {
            try {
                var args = data.argumentsObj || (data.arguments ? JSON.parse(data.arguments) : null);
                if (args && typeof args === 'object') {
                    var paramsLabel = (typeof window.t === 'function') ? window.t('timeline.params') : '参数:';
                    html += '<div class="webshell-ai-timeline-msg"><div class="tool-arg-section"><strong>' +
                        escapeHtml(paramsLabel) +
                        '</strong><pre class="tool-args">' +
                        escapeHtml(JSON.stringify(args, null, 2)) +
                        '</pre></div></div>';
                }
            } catch (e) {
                // JSON 解析失败时忽略参数详情，避免打断主流程
            }
        } else if (type === 'tool_result' && data) {
            // 工具调用出参
            var isError = data.isError || data.success === false;
            var noResultText = (typeof window.t === 'function') ? window.t('timeline.noResult') : '无结果';
            var result = data.result != null ? data.result : (data.error != null ? data.error : noResultText);
            var resultStr = (typeof result === 'string') ? result : JSON.stringify(result);
            var execResultLabel = (typeof window.t === 'function') ? window.t('timeline.executionResult') : '执行结果:';
            var execIdLabel = (typeof window.t === 'function') ? window.t('timeline.executionId') : '执行ID:';
            html += '<div class="webshell-ai-timeline-msg"><div class="tool-result-section ' +
                (isError ? 'error' : 'success') +
                '"><strong>' + escapeHtml(execResultLabel) + '</strong><pre class="tool-result">' +
                escapeHtml(resultStr) +
                '</pre>' +
                (data.executionId ? '<div class="tool-execution-id"><span>' +
                    escapeHtml(execIdLabel) +
                    '</span> <code>' +
                    escapeHtml(String(data.executionId)) +
                    '</code></div>' : '') +
                '</div></div>';
        } else if (message && message !== title) {
            html += '<div class="webshell-ai-timeline-msg">' + escapeHtml(message) + '</div>';
        }

        item.innerHTML = html;
        timelineContainer.appendChild(item);
        timelineContainer.classList.add('has-items');
        messagesContainer.scrollTop = messagesContainer.scrollHeight;
    }

    if (inputEl) inputEl.value = '';

    var convId = webshellAiConvMap[conn.id] || '';
    var body = {
        message: message,
        webshellConnectionId: conn.id,
        conversationId: convId
    };

    // 流式输出：支持 progress 实时更新、response 打字机效果；若后端发送多段 response 则追加
    var streamingTarget = '';  // 当前要打字显示的目标全文（用于打字机效果）
    var streamingTypingId = 0;  // 防重入，每次新 response 自增

    apiFetch('/api/agent-loop/stream', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body)
    }).then(function (response) {
        if (!response.ok) {
            assistantDiv.textContent = '请求失败: ' + response.status;
            return;
        }
        return response.body.getReader();
    }).then(function (reader) {
        if (!reader) return;
        var decoder = new TextDecoder();
        var buffer = '';
        return reader.read().then(function processChunk(result) {
            if (result.done) return;
            buffer += decoder.decode(result.value, { stream: true });
            var lines = buffer.split('\n');
            buffer = lines.pop() || '';
            for (var i = 0; i < lines.length; i++) {
                var line = lines[i];
                if (line.indexOf('data: ') !== 0) continue;
                try {
                    var eventData = JSON.parse(line.slice(6));
                    if (eventData.type === 'conversation' && eventData.data && eventData.data.conversationId) {
                        // 先把 conversationId 拿出来，避免后续异步回调里 eventData 被后续事件覆盖导致 undefined 报错
                        var convId = eventData.data.conversationId;
                        webshellAiConvMap[conn.id] = convId;
                        var listEl = document.getElementById('webshell-ai-conv-list');
                        if (listEl) fetchAndRenderWebshellAiConvList(conn, listEl).then(function () {
                            listEl.querySelectorAll('.webshell-ai-conv-item').forEach(function (el) {
                                el.classList.toggle('active', el.dataset.convId === convId);
                            });
                        });
                    } else if (eventData.type === 'response') {
                        var text = (eventData.message != null && eventData.message !== '') ? eventData.message : (eventData.data && typeof eventData.data === 'string' ? eventData.data : '');
                        if (text) {
                            streamingTarget += text;
                            webshellStreamingTypingId += 1;
                            streamingTypingId = webshellStreamingTypingId;
                            runWebshellAiStreamingTyping(assistantDiv, streamingTarget, streamingTypingId, messagesContainer);
                        }
                    } else if (eventData.type === 'error' && eventData.message) {
                        streamingTypingId += 1;
                        var errLabel = (typeof window.t === 'function') ? window.t('chat.error') : '错误';
                        appendTimelineItem('error', '❌ ' + errLabel, eventData.message, eventData.data);
                        assistantDiv.textContent = errLabel + ': ' + eventData.message;
                    } else if (eventData.type === 'progress' && eventData.message) {
                        var progressMsg = (typeof window.translateProgressMessage === 'function')
                            ? window.translateProgressMessage(eventData.message)
                            : eventData.message;
                        appendTimelineItem('progress', '🔍 ' + progressMsg, '', eventData.data);
                        if (!streamingTarget) assistantDiv.textContent = '…';
                    } else if (eventData.type === 'iteration') {
                        var iterN = (eventData.data && eventData.data.iteration) || 0;
                        var iterTitle = (typeof window.t === 'function')
                            ? window.t('chat.iterationRound', { n: iterN || 1 })
                            : (iterN ? ('第 ' + iterN + ' 轮迭代') : (eventData.message || '迭代'));
                        var iterMessage = eventData.message || '';
                        if (iterMessage && typeof window.translateProgressMessage === 'function') {
                            iterMessage = window.translateProgressMessage(iterMessage);
                        }
                        appendTimelineItem('iteration', '🔍 ' + iterTitle, iterMessage, eventData.data);
                        if (!streamingTarget) assistantDiv.textContent = '…';
                    } else if (eventData.type === 'thinking' && eventData.message) {
                        var thinkLabel = (typeof window.t === 'function') ? window.t('chat.aiThinking') : 'AI 思考';
                        appendTimelineItem('thinking', '🤔 ' + thinkLabel, eventData.message, eventData.data);
                        if (!streamingTarget) assistantDiv.textContent = '…';
                    } else if (eventData.type === 'tool_calls_detected' && eventData.data) {
                        var count = eventData.data.count || 0;
                        var detectedLabel = (typeof window.t === 'function')
                            ? window.t('chat.toolCallsDetected', { count: count })
                            : ('检测到 ' + count + ' 个工具调用');
                        appendTimelineItem('tool_calls_detected', '🔧 ' + detectedLabel, eventData.message || '', eventData.data);
                        if (!streamingTarget) assistantDiv.textContent = '…';
                    } else if (eventData.type === 'tool_call' && eventData.data) {
                        var d = eventData.data;
                        var tn = d.toolName || '未知工具';
                        var idx = d.index || 0;
                        var total = d.total || 0;
                        var callTitle = (typeof window.t === 'function')
                            ? window.t('chat.callTool', { name: tn, index: idx, total: total })
                            : ('调用: ' + tn + (total ? ' (' + idx + '/' + total + ')' : ''));
                        var title = '🔧 ' + callTitle;
                        appendTimelineItem('tool_call', title, eventData.message || '', eventData.data);
                        if (!streamingTarget) assistantDiv.textContent = '…';
                    } else if (eventData.type === 'tool_result' && eventData.data) {
                        var dr = eventData.data;
                        var success = dr.success !== false;
                        var tname = dr.toolName || '工具';
                        var titleText = (typeof window.t === 'function')
                            ? (success ? window.t('chat.toolExecComplete', { name: tname }) : window.t('chat.toolExecFailed', { name: tname }))
                            : (tname + (success ? ' 执行完成' : ' 执行失败'));
                        var title = (success ? '✅ ' : '❌ ') + titleText;
                        var sub = eventData.message || (dr.result ? String(dr.result).slice(0, 300) : '');
                        appendTimelineItem('tool_result', title, sub, eventData.data);
                        if (!streamingTarget) assistantDiv.textContent = '…';
                    }
                } catch (e) { /* ignore parse error */ }
            }
            messagesContainer.scrollTop = messagesContainer.scrollHeight;
            return reader.read().then(processChunk);
        });
    }).catch(function (err) {
        assistantDiv.textContent = '请求异常: ' + (err && err.message ? err.message : String(err));
    }).then(function () {
        webshellAiSending = false;
        if (sendBtn) sendBtn.disabled = false;
        if (assistantDiv.textContent === '…' && !streamingTarget) {
            // 没有任何 response 内容，保持纯文本提示
            assistantDiv.textContent = '无回复内容';
        } else if (streamingTarget) {
            // 流式结束：先终止当前打字机循环，避免后续 tick 把 HTML 覆盖回纯文本
            webshellStreamingTypingId += 1;
            // 再使用 Markdown 渲染完整内容
            if (typeof formatMarkdown === 'function') {
                assistantDiv.innerHTML = formatMarkdown(streamingTarget);
            } else {
                assistantDiv.textContent = streamingTarget;
            }
        }
        // 生成结果后：将执行过程折叠并保留，供后续查看；统一放在「助手回复下方」（与刷新后加载历史一致，最佳实践）
        if (timelineContainer && timelineContainer.classList.contains('has-items') && !timelineContainer.closest('.webshell-ai-process-block')) {
            var headerLabel = (typeof window.t === 'function') ? (window.t('chat.penetrationTestDetail') || '执行过程及调用工具') : '执行过程及调用工具';
            var wrap = document.createElement('div');
            wrap.className = 'process-details-container webshell-ai-process-block';
            wrap.innerHTML = '<button type="button" class="webshell-ai-process-toggle" aria-expanded="false">' + escapeHtml(headerLabel) + ' <span class="ws-toggle-icon">▶</span></button><div class="process-details-content"></div>';
            var contentDiv = wrap.querySelector('.process-details-content');
            contentDiv.appendChild(timelineContainer);
            timelineContainer.classList.add('progress-timeline');
            messagesContainer.insertBefore(wrap, assistantDiv.nextSibling);
            var toggleBtn = wrap.querySelector('.webshell-ai-process-toggle');
            var toggleIcon = wrap.querySelector('.ws-toggle-icon');
            toggleBtn.addEventListener('click', function () {
                var isExpanded = timelineContainer.classList.contains('expanded');
                timelineContainer.classList.toggle('expanded');
                toggleBtn.setAttribute('aria-expanded', !isExpanded);
                if (toggleIcon) toggleIcon.textContent = isExpanded ? '▶' : '▼';
            });
        }
        messagesContainer.scrollTop = messagesContainer.scrollHeight;
    });
}

// 打字机效果：将 target 逐字/逐段写入 el，保证只生效于当前 id 的调用
function runWebshellAiStreamingTyping(el, target, id, scrollContainer) {
    if (!el || id === undefined) return;
    var chunkSize = 3;
    var delayMs = 24;
    function tick() {
        if (id !== webshellStreamingTypingId) return;
        var cur = el.textContent || '';
        if (cur.length >= target.length) {
            el.textContent = target;
            if (scrollContainer) scrollContainer.scrollTop = scrollContainer.scrollHeight;
            return;
        }
        var next = target.slice(0, cur.length + chunkSize);
        el.textContent = next;
        if (scrollContainer) scrollContainer.scrollTop = scrollContainer.scrollHeight;
        setTimeout(tick, delayMs);
    }
    if (el.textContent.length < target.length) setTimeout(tick, delayMs);
}

function getWebshellHistory(connId) {
    if (!connId) return [];
    if (!webshellHistoryByConn[connId]) webshellHistoryByConn[connId] = [];
    return webshellHistoryByConn[connId];
}
function pushWebshellHistory(connId, cmd) {
    if (!connId || !cmd) return;
    if (!webshellHistoryByConn[connId]) webshellHistoryByConn[connId] = [];
    var h = webshellHistoryByConn[connId];
    if (h[h.length - 1] === cmd) return;
    h.push(cmd);
    if (h.length > WEBSHELL_HISTORY_MAX) h.shift();
}

// 执行快捷命令并将输出写入当前终端
function runQuickCommand(cmd) {
    if (!webshellCurrentConn || !webshellTerminalInstance) return;
    if (webshellRunning) return;
    var term = webshellTerminalInstance;
    term.writeln('');
    pushWebshellHistory(webshellCurrentConn.id, cmd);
    webshellRunning = true;
    execWebshellCommand(webshellCurrentConn, cmd).then(function (out) {
        var s = String(out || '').replace(/\r\n/g, '\n').replace(/\r/g, '\n');
        s.split('\n').forEach(function (line) { term.writeln(line.replace(/\r/g, '')); });
        term.write(WEBSHELL_PROMPT);
    }).catch(function (err) {
        term.writeln('\x1b[31m' + (err && err.message ? err.message : wsT('webshell.execError')) + '\x1b[0m');
        term.write(WEBSHELL_PROMPT);
    }).finally(function () { webshellRunning = false; });
}

// ---------- 虚拟终端（xterm + 按行执行） ----------
function initWebshellTerminal(conn) {
    const container = document.getElementById('webshell-terminal-container');
    if (!container || typeof Terminal === 'undefined') {
        if (container) {
            container.innerHTML = '<p class="terminal-error">' + escapeHtml('未加载 xterm.js，请刷新页面') + '</p>';
        }
        return;
    }

    const term = new Terminal({
        cursorBlink: true,
        cursorStyle: 'underline',
        fontSize: 13,
        fontFamily: 'Menlo, Monaco, "Courier New", monospace',
        lineHeight: 1.2,
        scrollback: 2000,
        theme: {
            background: '#0d1117',
            foreground: '#e6edf3',
            cursor: '#58a6ff',
            cursorAccent: '#0d1117',
            selection: 'rgba(88, 166, 255, 0.3)'
        }
    });

    let fitAddon = null;
    if (typeof FitAddon !== 'undefined') {
        const FitCtor = FitAddon.FitAddon || FitAddon;
        fitAddon = new FitCtor();
        term.loadAddon(fitAddon);
    }

    term.open(container);
    // 先 fit 再写内容，避免未计算尺寸时光标/画布错位挡住文字
    try {
        if (fitAddon) fitAddon.fit();
    } catch (e) {}
    // 不再输出欢迎行，避免占用空间、挡住输入
    term.write(WEBSHELL_PROMPT);

    // 按行写入输出，与系统设置终端 writeOutput 一致，避免 ls 等输出错位
    function writeWebshellOutput(term, text, isError) {
        if (!term || !text) return;
        var s = String(text).replace(/\r\n/g, '\n').replace(/\r/g, '\n');
        var lines = s.split('\n');
        var prefix = isError ? '\x1b[31m' : '';
        var suffix = isError ? '\x1b[0m' : '';
        term.write(prefix);
        for (var i = 0; i < lines.length; i++) {
            term.writeln(lines[i].replace(/\r/g, ''));
        }
        term.write(suffix);
    }

    term.onData(function (data) {
        // Ctrl+L 清屏
        if (data === '\x0c') {
            term.clear();
            webshellLineBuffer = '';
            webshellHistoryIndex = -1;
            term.write(WEBSHELL_PROMPT);
            return;
        }
        // 上/下键：命令历史
        if (data === '\x1b[A' || data === '\x1bOA') {
            var hist = getWebshellHistory(webshellCurrentConn ? webshellCurrentConn.id : '');
            if (hist.length === 0) return;
            webshellHistoryIndex = webshellHistoryIndex < 0 ? hist.length : Math.max(0, webshellHistoryIndex - 1);
            webshellLineBuffer = hist[webshellHistoryIndex] || '';
            term.write('\x1b[2K\r' + WEBSHELL_PROMPT + webshellLineBuffer);
            return;
        }
        if (data === '\x1b[B' || data === '\x1bOB') {
            var hist2 = getWebshellHistory(webshellCurrentConn ? webshellCurrentConn.id : '');
            if (hist2.length === 0) return;
            webshellHistoryIndex = webshellHistoryIndex < 0 ? -1 : Math.min(hist2.length - 1, webshellHistoryIndex + 1);
            if (webshellHistoryIndex < 0) webshellLineBuffer = '';
            else webshellLineBuffer = hist2[webshellHistoryIndex] || '';
            term.write('\x1b[2K\r' + WEBSHELL_PROMPT + webshellLineBuffer);
            return;
        }
        // 回车：发送当前行到后端执行
        if (data === '\r' || data === '\n') {
            term.writeln('');
            var cmd = webshellLineBuffer.trim();
            webshellLineBuffer = '';
            webshellHistoryIndex = -1;
            if (cmd) {
                if (webshellRunning) {
                    writeWebshellOutput(term, wsT('webshell.waitFinish'), true);
                    term.write(WEBSHELL_PROMPT);
                    return;
                }
                pushWebshellHistory(webshellCurrentConn ? webshellCurrentConn.id : '', cmd);
                webshellRunning = true;
                execWebshellCommand(webshellCurrentConn, cmd).then(function (out) {
                    webshellRunning = false;
                    if (out && out.length) writeWebshellOutput(term, out, false);
                    term.write(WEBSHELL_PROMPT);
                }).catch(function (err) {
                    webshellRunning = false;
                    writeWebshellOutput(term, err && err.message ? err.message : wsT('webshell.execError'), true);
                    term.write(WEBSHELL_PROMPT);
                });
            } else {
                term.write(WEBSHELL_PROMPT);
            }
            return;
        }
        // 多行粘贴：按行依次执行
        if (data.indexOf('\n') !== -1 || data.indexOf('\r') !== -1) {
            var full = (webshellLineBuffer + data).replace(/\r\n/g, '\n').replace(/\r/g, '\n');
            var lines = full.split('\n');
            webshellLineBuffer = lines.pop() || '';
            if (lines.length > 0 && !webshellRunning && webshellCurrentConn) {
                var runNext = function (idx) {
                    if (idx >= lines.length) {
                        term.write(WEBSHELL_PROMPT + webshellLineBuffer);
                        return;
                    }
                    var line = lines[idx].trim();
                    if (!line) { runNext(idx + 1); return; }
                    pushWebshellHistory(webshellCurrentConn.id, line);
                    webshellRunning = true;
                    execWebshellCommand(webshellCurrentConn, line).then(function (out) {
                        if (out && out.length) writeWebshellOutput(term, out, false);
                        webshellRunning = false;
                        runNext(idx + 1);
                    }).catch(function (err) {
                        writeWebshellOutput(term, err && err.message ? err.message : wsT('webshell.execError'), true);
                        webshellRunning = false;
                        runNext(idx + 1);
                    });
                };
                runNext(0);
            } else {
                term.write(data);
            }
            return;
        }
        // 退格
        if (data === '\x7f' || data === '\b') {
            if (webshellLineBuffer.length > 0) {
                webshellLineBuffer = webshellLineBuffer.slice(0, -1);
                term.write('\b \b');
            }
            return;
        }
        webshellLineBuffer += data;
        term.write(data);
    });

    webshellTerminalInstance = term;
    webshellTerminalFitAddon = fitAddon;
    // 延迟再次 fit，确保容器尺寸稳定后光标与文字不错位
    setTimeout(function () {
        try { if (fitAddon) fitAddon.fit(); } catch (e) {}
    }, 100);
    // 容器尺寸变化时重新 fit，避免光标/文字被遮挡
    if (fitAddon && typeof ResizeObserver !== 'undefined' && container) {
        webshellTerminalResizeContainer = container;
        webshellTerminalResizeObserver = new ResizeObserver(function () {
            try { fitAddon.fit(); } catch (e) {}
        });
        webshellTerminalResizeObserver.observe(container);
    }
}

// 调用后端执行命令
function execWebshellCommand(conn, command) {
    return new Promise(function (resolve, reject) {
        if (typeof apiFetch === 'undefined') {
            reject(new Error('apiFetch 未定义'));
            return;
        }
        apiFetch('/api/webshell/exec', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                url: conn.url,
                password: conn.password || '',
                type: conn.type || 'php',
                method: (conn.method || 'post').toLowerCase(),
                cmd_param: conn.cmdParam || '',
                command: command
            })
        }).then(function (r) { return r.json(); })
            .then(function (data) {
                if (data && data.output !== undefined) resolve(data.output || '');
                else if (data && data.error) reject(new Error(data.error));
                else resolve('');
            })
            .catch(reject);
    });
}

// ---------- 文件管理 ----------
function webshellFileListDir(conn, path) {
    const listEl = document.getElementById('webshell-file-list');
    if (!listEl) return;
    listEl.innerHTML = '<div class="webshell-loading">' + wsT('common.refresh') + '...</div>';

    if (typeof apiFetch === 'undefined') {
        listEl.innerHTML = '<div class="webshell-file-error">apiFetch 未定义</div>';
        return;
    }

    apiFetch('/api/webshell/file', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            url: conn.url,
            password: conn.password || '',
            type: conn.type || 'php',
            method: (conn.method || 'post').toLowerCase(),
            cmd_param: conn.cmdParam || '',
            action: 'list',
            path: path
        })
    }).then(function (r) { return r.json(); })
        .then(function (data) {
            if (!data.ok && data.error) {
                listEl.innerHTML = '<div class="webshell-file-error">' + escapeHtml(data.error) + '</div><pre class="webshell-file-raw">' + escapeHtml(data.output || '') + '</pre>';
                return;
            }
            listEl.dataset.currentPath = path;
            listEl.dataset.rawOutput = data.output || '';
            renderFileList(listEl, path, data.output || '', conn);
        })
        .catch(function (err) {
            listEl.innerHTML = '<div class="webshell-file-error">' + escapeHtml(err && err.message ? err.message : wsT('webshell.execError')) + '</div>';
        });
}

function renderFileList(listEl, currentPath, rawOutput, conn, nameFilter) {
    var lines = rawOutput.split(/\n/).filter(function (l) { return l.trim(); });
    var items = [];
    for (var i = 0; i < lines.length; i++) {
        var line = lines[i];
        var m = line.match(/\s*(\S+)\s*$/);
        var name = m ? m[1].trim() : line.trim();
        if (name === '.' || name === '..') continue;
        var isDir = line.startsWith('d') || line.toLowerCase().indexOf('<dir>') !== -1;
        var size = '';
        var mode = '';
        if (line.startsWith('-') || line.startsWith('d')) {
            var parts = line.split(/\s+/);
            if (parts.length >= 5) { mode = parts[0]; size = parts[4]; }
        }
        items.push({ name: name, isDir: isDir, line: line, size: size, mode: mode });
    }
    if (nameFilter && nameFilter.trim()) {
        var f = nameFilter.trim().toLowerCase();
        items = items.filter(function (item) { return item.name.toLowerCase().indexOf(f) !== -1; });
    }
    // 面包屑
    var breadcrumbEl = document.getElementById('webshell-file-breadcrumb');
    if (breadcrumbEl) {
        var parts = (currentPath === '.' || currentPath === '') ? [] : currentPath.replace(/^\//, '').split('/');
        breadcrumbEl.innerHTML = '<a href="#" class="webshell-breadcrumb-item" data-path=".">' + (wsT('webshell.breadcrumbHome') || '根') + '</a>' +
            parts.map(function (p, idx) {
                var path = parts.slice(0, idx + 1).join('/');
                return ' / <a href="#" class="webshell-breadcrumb-item" data-path="' + escapeHtml(path) + '">' + escapeHtml(p) + '</a>';
            }).join('');
    }
    var html = '';
    if (items.length === 0 && rawOutput.trim() && !nameFilter) {
        html = '<pre class="webshell-file-raw">' + escapeHtml(rawOutput) + '</pre>';
    } else {
        html = '<table class="webshell-file-table"><thead><tr><th class="webshell-col-check"><input type="checkbox" id="webshell-file-select-all" title="' + (wsT('webshell.selectAll') || '全选') + '" /></th><th>' + wsT('webshell.filePath') + '</th><th class="webshell-col-size">大小</th><th></th></tr></thead><tbody>';
        if (currentPath !== '.' && currentPath !== '') {
            html += '<tr><td></td><td><a href="#" class="webshell-file-link" data-path="' + escapeHtml(currentPath.replace(/\/[^/]+$/, '') || '.') + '" data-isdir="1">..</a></td><td></td><td></td></tr>';
        }
        items.forEach(function (item) {
            var pathNext = currentPath === '.' ? item.name : currentPath + '/' + item.name;
            html += '<tr><td class="webshell-col-check">';
            if (!item.isDir) html += '<input type="checkbox" class="webshell-file-cb" data-path="' + escapeHtml(pathNext) + '" />';
            html += '</td><td><a href="#" class="webshell-file-link" data-path="' + escapeHtml(pathNext) + '" data-isdir="' + (item.isDir ? '1' : '0') + '">' + escapeHtml(item.name) + (item.isDir ? '/' : '') + '</a></td><td class="webshell-col-size">' + escapeHtml(item.size) + '</td><td>';
            if (item.isDir) {
                html += '<button type="button" class="btn-ghost btn-sm webshell-file-rename" data-path="' + escapeHtml(pathNext) + '" data-name="' + escapeHtml(item.name) + '">' + (wsT('webshell.rename') || '重命名') + '</button>';
            } else {
                html += '<button type="button" class="btn-ghost btn-sm webshell-file-read" data-path="' + escapeHtml(pathNext) + '">' + wsT('webshell.readFile') + '</button> ';
                html += '<button type="button" class="btn-ghost btn-sm webshell-file-download" data-path="' + escapeHtml(pathNext) + '">' + wsT('webshell.downloadFile') + '</button> ';
                html += '<button type="button" class="btn-ghost btn-sm webshell-file-edit" data-path="' + escapeHtml(pathNext) + '">' + wsT('webshell.editFile') + '</button> ';
                html += '<button type="button" class="btn-ghost btn-sm webshell-file-rename" data-path="' + escapeHtml(pathNext) + '" data-name="' + escapeHtml(item.name) + '">' + (wsT('webshell.rename') || '重命名') + '</button> ';
                html += '<button type="button" class="btn-ghost btn-sm webshell-file-del" data-path="' + escapeHtml(pathNext) + '">' + wsT('webshell.deleteFile') + '</button>';
            }
            html += '</td></tr>';
        });
        html += '</tbody></table>';
    }
    listEl.innerHTML = html;

    listEl.querySelectorAll('.webshell-file-link').forEach(function (a) {
        a.addEventListener('click', function (e) {
            e.preventDefault();
            const path = a.getAttribute('data-path');
            const isDir = a.getAttribute('data-isdir') === '1';
            const pathInput = document.getElementById('webshell-file-path');
            if (pathInput) pathInput.value = path;
            if (isDir) webshellFileListDir(webshellCurrentConn, path);
            else webshellFileRead(webshellCurrentConn, path, listEl);
        });
    });
    listEl.querySelectorAll('.webshell-file-read').forEach(function (btn) {
        btn.addEventListener('click', function (e) {
            e.preventDefault();
            webshellFileRead(webshellCurrentConn, btn.getAttribute('data-path'), listEl);
        });
    });
    listEl.querySelectorAll('.webshell-file-download').forEach(function (btn) {
        btn.addEventListener('click', function (e) {
            e.preventDefault();
            webshellFileDownload(webshellCurrentConn, btn.getAttribute('data-path'));
        });
    });
    listEl.querySelectorAll('.webshell-file-edit').forEach(function (btn) {
        btn.addEventListener('click', function (e) {
            e.preventDefault();
            webshellFileEdit(webshellCurrentConn, btn.getAttribute('data-path'), listEl);
        });
    });
    listEl.querySelectorAll('.webshell-file-del').forEach(function (btn) {
        btn.addEventListener('click', function (e) {
            e.preventDefault();
            if (!confirm(wsT('webshell.deleteConfirm'))) return;
            webshellFileDelete(webshellCurrentConn, btn.getAttribute('data-path'), function () {
                webshellFileListDir(webshellCurrentConn, document.getElementById('webshell-file-path').value.trim() || '.');
            });
        });
    });
    listEl.querySelectorAll('.webshell-file-rename').forEach(function (btn) {
        btn.addEventListener('click', function (e) {
            e.preventDefault();
            webshellFileRename(webshellCurrentConn, btn.getAttribute('data-path'), btn.getAttribute('data-name'), listEl);
        });
    });
    var selectAll = document.getElementById('webshell-file-select-all');
    if (selectAll) {
        selectAll.addEventListener('change', function () {
            listEl.querySelectorAll('.webshell-file-cb').forEach(function (cb) { cb.checked = selectAll.checked; });
        });
    }
    if (breadcrumbEl) {
        breadcrumbEl.querySelectorAll('.webshell-breadcrumb-item').forEach(function (a) {
            a.addEventListener('click', function (e) {
                e.preventDefault();
                var p = a.getAttribute('data-path');
                var pathInput = document.getElementById('webshell-file-path');
                if (pathInput) pathInput.value = p;
                webshellFileListDir(webshellCurrentConn, p);
            });
        });
    }
}

function webshellFileListApplyFilter() {
    var listEl = document.getElementById('webshell-file-list');
    var path = listEl && listEl.dataset.currentPath ? listEl.dataset.currentPath : (document.getElementById('webshell-file-path') && document.getElementById('webshell-file-path').value.trim()) || '.';
    var raw = listEl && listEl.dataset.rawOutput ? listEl.dataset.rawOutput : '';
    var filterInput = document.getElementById('webshell-file-filter');
    var filter = filterInput ? filterInput.value : '';
    if (!listEl || !raw) return;
    renderFileList(listEl, path, raw, webshellCurrentConn, filter);
}

function webshellFileMkdir(conn, pathInput) {
    if (!conn || typeof apiFetch === 'undefined') return;
    var base = (pathInput && pathInput.value.trim()) || '.';
    var name = prompt(wsT('webshell.newDir') || '新建目录', 'newdir');
    if (name == null || !name.trim()) return;
    var path = base === '.' ? name.trim() : base + '/' + name.trim();
    apiFetch('/api/webshell/file', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ url: conn.url, password: conn.password || '', type: conn.type || 'php', method: (conn.method || 'post').toLowerCase(), cmd_param: conn.cmdParam || '', action: 'mkdir', path: path }) })
        .then(function (r) { return r.json(); })
        .then(function () { webshellFileListDir(conn, base); })
        .catch(function () { webshellFileListDir(conn, base); });
}

function webshellFileNewFile(conn, pathInput) {
    if (!conn || typeof apiFetch === 'undefined') return;
    var base = (pathInput && pathInput.value.trim()) || '.';
    var name = prompt(wsT('webshell.newFile') || '新建文件', 'newfile.txt');
    if (name == null || !name.trim()) return;
    var path = base === '.' ? name.trim() : base + '/' + name.trim();
    var content = prompt('初始内容（可选）', '');
    if (content === null) return;
    var listEl = document.getElementById('webshell-file-list');
    webshellFileWrite(conn, path, content || '', function () { webshellFileListDir(conn, base); }, listEl);
}

function webshellFileUpload(conn, pathInput) {
    if (!conn || typeof apiFetch === 'undefined') return;
    var base = (pathInput && pathInput.value.trim()) || '.';
    var input = document.createElement('input');
    input.type = 'file';
    input.multiple = false;
    input.onchange = function () {
        var file = input.files && input.files[0];
        if (!file) return;
        var reader = new FileReader();
        reader.onload = function () {
            var buf = reader.result;
            var bin = new Uint8Array(buf);
            var CHUNK = 32000;
            var base64Chunks = [];
            for (var i = 0; i < bin.length; i += CHUNK) {
                var slice = bin.subarray(i, Math.min(i + CHUNK, bin.length));
                var b64 = btoa(String.fromCharCode.apply(null, slice));
                base64Chunks.push(b64);
            }
            var path = base === '.' ? file.name : base + '/' + file.name;
            var listEl = document.getElementById('webshell-file-list');
            if (listEl) listEl.innerHTML = '<div class="webshell-loading">' + (wsT('webshell.upload') || '上传') + '...</div>';
            var idx = 0;
            function sendNext() {
                if (idx >= base64Chunks.length) {
                    webshellFileListDir(conn, base);
                    return;
                }
                apiFetch('/api/webshell/file', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ url: conn.url, password: conn.password || '', type: conn.type || 'php', method: (conn.method || 'post').toLowerCase(), cmd_param: conn.cmdParam || '', action: 'upload_chunk', path: path, content: base64Chunks[idx], chunk_index: idx }) })
                    .then(function (r) { return r.json(); })
                    .then(function () { idx++; sendNext(); })
                    .catch(function () { idx++; sendNext(); });
            }
            sendNext();
        };
        reader.readAsArrayBuffer(file);
    };
    input.click();
}

function webshellFileRename(conn, oldPath, oldName, listEl) {
    if (!conn || typeof apiFetch === 'undefined') return;
    var newName = prompt((wsT('webshell.rename') || '重命名') + ': ' + oldName, oldName);
    if (newName == null || newName.trim() === '') return;
    var parts = oldPath.split('/');
    var dir = parts.length > 1 ? parts.slice(0, -1).join('/') + '/' : '';
    var newPath = dir + newName.trim();
    apiFetch('/api/webshell/file', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ url: conn.url, password: conn.password || '', type: conn.type || 'php', method: (conn.method || 'post').toLowerCase(), cmd_param: conn.cmdParam || '', action: 'rename', path: oldPath, target_path: newPath }) })
        .then(function (r) { return r.json(); })
        .then(function () { webshellFileListDir(conn, document.getElementById('webshell-file-path').value.trim() || '.'); })
        .catch(function () { webshellFileListDir(conn, document.getElementById('webshell-file-path').value.trim() || '.'); });
}

function webshellBatchDelete(conn, pathInput) {
    if (!conn) return;
    var listEl = document.getElementById('webshell-file-list');
    var checked = listEl ? listEl.querySelectorAll('.webshell-file-cb:checked') : [];
    var paths = [];
    checked.forEach(function (cb) { paths.push(cb.getAttribute('data-path')); });
    if (paths.length === 0) { alert(wsT('webshell.batchDelete') + '：请先勾选文件'); return; }
    if (!confirm(wsT('webshell.batchDelete') + '：确定删除 ' + paths.length + ' 个文件？')) return;
    var base = (pathInput && pathInput.value.trim()) || '.';
    var i = 0;
    function delNext() {
        if (i >= paths.length) { webshellFileListDir(conn, base); return; }
        webshellFileDelete(conn, paths[i], function () { i++; delNext(); });
    }
    delNext();
}

function webshellBatchDownload(conn, pathInput) {
    if (!conn) return;
    var listEl = document.getElementById('webshell-file-list');
    var checked = listEl ? listEl.querySelectorAll('.webshell-file-cb:checked') : [];
    var paths = [];
    checked.forEach(function (cb) { paths.push(cb.getAttribute('data-path')); });
    if (paths.length === 0) { alert(wsT('webshell.batchDownload') + '：请先勾选文件'); return; }
    paths.forEach(function (path) { webshellFileDownload(conn, path); });
}

// 下载文件到本地（读取内容后触发浏览器下载）
function webshellFileDownload(conn, path) {
    if (typeof apiFetch === 'undefined') return;
    apiFetch('/api/webshell/file', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ url: conn.url, password: conn.password || '', type: conn.type || 'php', method: (conn.method || 'post').toLowerCase(), cmd_param: conn.cmdParam || '', action: 'read', path: path })
    }).then(function (r) { return r.json(); })
        .then(function (data) {
            var content = (data && data.output) != null ? data.output : (data.error || '');
            var name = path.replace(/^.*[/\\]/, '') || 'download.txt';
            var blob = new Blob([content], { type: 'application/octet-stream' });
            var a = document.createElement('a');
            a.href = URL.createObjectURL(blob);
            a.download = name;
            a.click();
            URL.revokeObjectURL(a.href);
        })
        .catch(function (err) { alert(wsT('webshell.execError') + ': ' + (err && err.message ? err.message : '')); });
}

function webshellFileRead(conn, path, listEl) {
    if (typeof apiFetch === 'undefined') return;
    listEl.innerHTML = '<div class="webshell-loading">' + wsT('webshell.readFile') + '...</div>';
    apiFetch('/api/webshell/file', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ url: conn.url, password: conn.password || '', type: conn.type || 'php', method: (conn.method || 'post').toLowerCase(), cmd_param: conn.cmdParam || '', action: 'read', path: path })
    }).then(function (r) { return r.json(); })
        .then(function (data) {
            const out = (data && data.output) ? data.output : (data.error || '');
            listEl.innerHTML = '<div class="webshell-file-content"><pre>' + escapeHtml(out) + '</pre><button type="button" class="btn-ghost" onclick="webshellFileListDir(webshellCurrentConn, document.getElementById(\'webshell-file-path\').value.trim() || \'.\')">' + wsT('webshell.listDir') + '</button></div>';
        })
        .catch(function (err) {
            listEl.innerHTML = '<div class="webshell-file-error">' + escapeHtml(err && err.message ? err.message : '') + '</div>';
        });
}

function webshellFileEdit(conn, path, listEl) {
    if (typeof apiFetch === 'undefined') return;
    listEl.innerHTML = '<div class="webshell-loading">' + wsT('webshell.editFile') + '...</div>';
    apiFetch('/api/webshell/file', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ url: conn.url, password: conn.password || '', type: conn.type || 'php', method: (conn.method || 'post').toLowerCase(), cmd_param: conn.cmdParam || '', action: 'read', path: path })
    }).then(function (r) { return r.json(); })
        .then(function (data) {
            const content = (data && data.output) ? data.output : (data.error || '');
            const pathInput = document.getElementById('webshell-file-path');
            const currentPath = pathInput ? pathInput.value.trim() || '.' : '.';
            listEl.innerHTML =
                '<div class="webshell-file-edit-wrap">' +
                '<div class="webshell-file-edit-path">' + escapeHtml(path) + '</div>' +
                '<textarea id="webshell-edit-textarea" class="webshell-file-edit-textarea" rows="18">' + escapeHtml(content) + '</textarea>' +
                '<div class="webshell-file-edit-actions">' +
                '<button type="button" class="btn-primary btn-sm" id="webshell-edit-save">' + wsT('webshell.saveFile') + '</button> ' +
                '<button type="button" class="btn-ghost btn-sm" id="webshell-edit-cancel">' + wsT('webshell.cancelEdit') + '</button>' +
                '</div></div>';
            document.getElementById('webshell-edit-save').addEventListener('click', function () {
                const textarea = document.getElementById('webshell-edit-textarea');
                const newContent = textarea ? textarea.value : '';
                webshellFileWrite(webshellCurrentConn, path, newContent, function () {
                    webshellFileListDir(webshellCurrentConn, currentPath);
                }, listEl);
            });
            document.getElementById('webshell-edit-cancel').addEventListener('click', function () {
                webshellFileListDir(webshellCurrentConn, currentPath);
            });
        })
        .catch(function (err) {
            listEl.innerHTML = '<div class="webshell-file-error">' + escapeHtml(err && err.message ? err.message : '') + '</div>';
        });
}

function webshellFileWrite(conn, path, content, onDone, listEl) {
    if (typeof apiFetch === 'undefined') return;
    if (listEl) listEl.innerHTML = '<div class="webshell-loading">' + wsT('webshell.saveFile') + '...</div>';
    apiFetch('/api/webshell/file', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ url: conn.url, password: conn.password || '', type: conn.type || 'php', method: (conn.method || 'post').toLowerCase(), cmd_param: conn.cmdParam || '', action: 'write', path: path, content: content })
    }).then(function (r) { return r.json(); })
        .then(function (data) {
            if (data && !data.ok && data.error && listEl) {
                listEl.innerHTML = '<div class="webshell-file-error">' + escapeHtml(data.error) + '</div><pre class="webshell-file-raw">' + escapeHtml(data.output || '') + '</pre>';
                return;
            }
            if (onDone) onDone();
        })
        .catch(function (err) {
            if (listEl) listEl.innerHTML = '<div class="webshell-file-error">' + escapeHtml(err && err.message ? err.message : wsT('webshell.execError')) + '</div>';
        });
}

function webshellFileDelete(conn, path, onDone) {
    if (typeof apiFetch === 'undefined') return;
    apiFetch('/api/webshell/file', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ url: conn.url, password: conn.password || '', type: conn.type || 'php', method: (conn.method || 'post').toLowerCase(), cmd_param: conn.cmdParam || '', action: 'delete', path: path })
    }).then(function (r) { return r.json(); })
        .then(function () { if (onDone) onDone(); })
        .catch(function () { if (onDone) onDone(); });
}

// 删除连接（请求服务端删除后刷新列表）
function deleteWebshell(id) {
    if (!confirm(wsT('webshell.deleteConfirm'))) return;
    if (currentWebshellId === id) destroyWebshellTerminal();
    if (currentWebshellId === id) currentWebshellId = null;
    if (typeof apiFetch === 'undefined') return;
    apiFetch('/api/webshell/connections/' + encodeURIComponent(id), { method: 'DELETE' })
        .then(function () {
            return refreshWebshellConnectionsFromServer();
        })
        .then(function () {
            const workspace = document.getElementById('webshell-workspace');
            if (workspace) {
                workspace.innerHTML = '<div class="webshell-workspace-placeholder">' + wsT('webshell.selectOrAdd') + '</div>';
            }
        })
        .catch(function (e) {
            console.warn('删除 WebShell 连接失败', e);
            refreshWebshellConnectionsFromServer();
        });
}

// 打开添加连接弹窗
function showAddWebshellModal() {
    var editIdEl = document.getElementById('webshell-edit-id');
    if (editIdEl) editIdEl.value = '';
    document.getElementById('webshell-url').value = '';
    document.getElementById('webshell-password').value = '';
    document.getElementById('webshell-type').value = 'php';
    document.getElementById('webshell-method').value = 'post';
    document.getElementById('webshell-cmd-param').value = '';
    document.getElementById('webshell-remark').value = '';
    var titleEl = document.getElementById('webshell-modal-title');
    if (titleEl) titleEl.textContent = wsT('webshell.addConnection');
    var modal = document.getElementById('webshell-modal');
    if (modal) modal.style.display = 'block';
}

// 打开编辑连接弹窗（预填当前连接信息）
function showEditWebshellModal(connId) {
    var conn = webshellConnections.find(function (c) { return c.id === connId; });
    if (!conn) return;
    var editIdEl = document.getElementById('webshell-edit-id');
    if (editIdEl) editIdEl.value = conn.id;
    document.getElementById('webshell-url').value = conn.url || '';
    document.getElementById('webshell-password').value = conn.password || '';
    document.getElementById('webshell-type').value = conn.type || 'php';
    document.getElementById('webshell-method').value = (conn.method || 'post').toLowerCase();
    document.getElementById('webshell-cmd-param').value = conn.cmdParam || '';
    document.getElementById('webshell-remark').value = conn.remark || '';
    var titleEl = document.getElementById('webshell-modal-title');
    if (titleEl) titleEl.textContent = wsT('webshell.editConnectionTitle');
    var modal = document.getElementById('webshell-modal');
    if (modal) modal.style.display = 'block';
}

// 关闭弹窗
function closeWebshellModal() {
    var editIdEl = document.getElementById('webshell-edit-id');
    if (editIdEl) editIdEl.value = '';
    var modal = document.getElementById('webshell-modal');
    if (modal) modal.style.display = 'none';
}

// 语言切换时刷新 WebShell 页面内所有由 JS 生成的文案（不重建终端）
function refreshWebshellUIOnLanguageChange() {
    var page = typeof window.currentPage === 'function' ? window.currentPage() : (window.currentPage || '');
    if (page !== 'webshell') return;

    renderWebshellList();
    var workspace = document.getElementById('webshell-workspace');
    if (workspace) {
        if (!currentWebshellId || !webshellCurrentConn) {
            workspace.innerHTML = '<div class="webshell-workspace-placeholder" data-i18n="webshell.selectOrAdd">' + wsT('webshell.selectOrAdd') + '</div>';
        } else {
            // 只更新标签文案，不重建终端
            var tabTerminal = workspace.querySelector('.webshell-tab[data-tab="terminal"]');
            var tabFile = workspace.querySelector('.webshell-tab[data-tab="file"]');
            var tabAi = workspace.querySelector('.webshell-tab[data-tab="ai"]');
            if (tabTerminal) tabTerminal.textContent = wsT('webshell.tabTerminal');
            if (tabFile) tabFile.textContent = wsT('webshell.tabFileManager');
            if (tabAi) tabAi.textContent = wsT('webshell.tabAiAssistant') || 'AI 助手';

            var quickLabel = workspace.querySelector('.webshell-quick-label');
            if (quickLabel) quickLabel.textContent = (wsT('webshell.quickCommands') || '快捷命令') + ':';
            var pathLabel = workspace.querySelector('.webshell-file-toolbar label span');
            var listDirBtn = document.getElementById('webshell-list-dir');
            var parentDirBtn = document.getElementById('webshell-parent-dir');
            if (pathLabel) pathLabel.textContent = wsT('webshell.filePath');
            if (listDirBtn) listDirBtn.textContent = wsT('webshell.listDir');
            if (parentDirBtn) parentDirBtn.textContent = wsT('webshell.parentDir');
            // 文件管理工具栏按钮（红框区域）：切换语言时立即更新
            var refreshBtn = document.getElementById('webshell-file-refresh');
            var mkdirBtn = document.getElementById('webshell-mkdir-btn');
            var newFileBtn = document.getElementById('webshell-newfile-btn');
            var uploadBtn = document.getElementById('webshell-upload-btn');
            var batchDeleteBtn = document.getElementById('webshell-batch-delete-btn');
            var batchDownloadBtn = document.getElementById('webshell-batch-download-btn');
            var filterInput = document.getElementById('webshell-file-filter');
            if (refreshBtn) { refreshBtn.title = wsT('webshell.refresh') || '刷新'; refreshBtn.textContent = wsT('webshell.refresh') || '刷新'; }
            if (mkdirBtn) mkdirBtn.textContent = wsT('webshell.newDir') || '新建目录';
            if (newFileBtn) newFileBtn.textContent = wsT('webshell.newFile') || '新建文件';
            if (uploadBtn) uploadBtn.textContent = wsT('webshell.upload') || '上传';
            if (batchDeleteBtn) batchDeleteBtn.textContent = wsT('webshell.batchDelete') || '批量删除';
            if (batchDownloadBtn) batchDownloadBtn.textContent = wsT('webshell.batchDownload') || '批量下载';
            if (filterInput) filterInput.placeholder = wsT('webshell.filterPlaceholder') || '过滤文件名';

            // AI 助手区域文案：Tab 内按钮、占位符、系统就绪提示
            var aiNewConvBtn = document.getElementById('webshell-ai-new-conv');
            if (aiNewConvBtn) aiNewConvBtn.textContent = wsT('webshell.aiNewConversation') || '新对话';
            var aiInput = document.getElementById('webshell-ai-input');
            if (aiInput) aiInput.placeholder = wsT('webshell.aiPlaceholder') || '例如：列出当前目录下的文件';
            var aiSendBtn = document.getElementById('webshell-ai-send');
            if (aiSendBtn) aiSendBtn.textContent = wsT('webshell.aiSend') || '发送';

            // 如果当前 AI 对话区只有系统就绪提示（没有用户消息），用当前语言重置这条提示
            var aiMessages = document.getElementById('webshell-ai-messages');
            if (aiMessages) {
                var hasUserMsg = !!aiMessages.querySelector('.webshell-ai-msg.user');
                var msgNodes = aiMessages.querySelectorAll('.webshell-ai-msg');
                if (!hasUserMsg && msgNodes.length <= 1) {
                    var readyMsg = wsT('webshell.aiSystemReadyMessage') || '系统已就绪。请输入您的测试需求，系统将自动执行相应的安全测试。';
                    aiMessages.innerHTML = '';
                    var readyDiv = document.createElement('div');
                    readyDiv.className = 'webshell-ai-msg assistant';
                    readyDiv.textContent = readyMsg;
                    aiMessages.appendChild(readyDiv);
                }
            }

            var pathInput = document.getElementById('webshell-file-path');
            var fileListEl = document.getElementById('webshell-file-list');
            if (fileListEl && webshellCurrentConn && pathInput) {
                webshellFileListDir(webshellCurrentConn, pathInput.value.trim() || '.');
            }
        }
    }

    var modal = document.getElementById('webshell-modal');
    if (modal && modal.style.display === 'block') {
        var titleEl = document.getElementById('webshell-modal-title');
        var editIdEl = document.getElementById('webshell-edit-id');
        if (titleEl) {
            titleEl.textContent = (editIdEl && editIdEl.value) ? wsT('webshell.editConnectionTitle') : wsT('webshell.addConnection');
        }
        if (typeof window.applyTranslations === 'function') {
            window.applyTranslations(modal);
        }
    }
}

document.addEventListener('languagechange', function () {
    refreshWebshellUIOnLanguageChange();
});

// 任意入口删除对话后同步：若当前在 WebShell AI 助手且已选连接，则刷新对话列表（与 Chat 侧边栏删除保持一致）
document.addEventListener('conversation-deleted', function (e) {
    var id = e.detail && e.detail.conversationId;
    if (!id || !currentWebshellId || !webshellCurrentConn) return;
    var listEl = document.getElementById('webshell-ai-conv-list');
    if (listEl) fetchAndRenderWebshellAiConvList(webshellCurrentConn, listEl);
    if (webshellAiConvMap[webshellCurrentConn.id] === id) {
        delete webshellAiConvMap[webshellCurrentConn.id];
        var msgs = document.getElementById('webshell-ai-messages');
        if (msgs) msgs.innerHTML = '';
    }
});

// 测试连通性（不保存，仅用当前表单参数请求 Shell 执行 echo 1）
function testWebshellConnection() {
    var url = (document.getElementById('webshell-url') || {}).value;
    if (url && typeof url.trim === 'function') url = url.trim();
    if (!url) {
        alert(wsT('webshell.url') ? (wsT('webshell.url') + ' 必填') : '请填写 Shell 地址');
        return;
    }
    var password = (document.getElementById('webshell-password') || {}).value;
    if (password && typeof password.trim === 'function') password = password.trim(); else password = '';
    var type = (document.getElementById('webshell-type') || {}).value || 'php';
    var method = ((document.getElementById('webshell-method') || {}).value || 'post').toLowerCase();
    var cmdParam = (document.getElementById('webshell-cmd-param') || {}).value;
    if (cmdParam && typeof cmdParam.trim === 'function') cmdParam = cmdParam.trim(); else cmdParam = '';
    var btn = document.getElementById('webshell-test-btn');
    if (btn) { btn.disabled = true; btn.textContent = (typeof wsT === 'function' ? wsT('common.refresh') : '刷新') + '...'; }
    if (typeof apiFetch === 'undefined') {
        if (btn) { btn.disabled = false; btn.textContent = wsT('webshell.testConnectivity'); }
        alert(wsT('webshell.testFailed') || '连通性测试失败');
        return;
    }
    apiFetch('/api/webshell/exec', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            url: url,
            password: password || '',
            type: type,
            method: method === 'get' ? 'get' : 'post',
            cmd_param: cmdParam || '',
            command: 'echo 1'
        })
    })
        .then(function (r) { return r.json(); })
        .then(function (data) {
            if (btn) { btn.disabled = false; btn.textContent = wsT('webshell.testConnectivity'); }
            if (!data) {
                alert(wsT('webshell.testFailed') || '连通性测试失败');
                return;
            }
            // 仅 HTTP 200 不算通过，需校验是否真的执行了 echo 1（响应体 trim 后应为 "1"）
            var output = (data.output != null) ? String(data.output).trim() : '';
            var reallyOk = data.ok && output === '1';
            if (reallyOk) {
                alert(wsT('webshell.testSuccess') || '连通性正常，Shell 可访问');
            } else {
                var msg;
                if (data.ok && output !== '1')
                    msg = wsT('webshell.testNoExpectedOutput') || 'Shell 返回了响应但未得到预期输出，请检查连接密码与命令参数名';
                else
                    msg = (data.error) ? data.error : (wsT('webshell.testFailed') || '连通性测试失败');
                if (data.http_code) msg += ' (HTTP ' + data.http_code + ')';
                alert(msg);
            }
        })
        .catch(function (e) {
            if (btn) { btn.disabled = false; btn.textContent = wsT('webshell.testConnectivity'); }
            alert((wsT('webshell.testFailed') || '连通性测试失败') + ': ' + (e && e.message ? e.message : String(e)));
        });
}

// 保存连接（新建或更新，请求服务端写入 SQLite 后刷新列表）
function saveWebshellConnection() {
    var url = (document.getElementById('webshell-url') || {}).value;
    if (url && typeof url.trim === 'function') url = url.trim();
    if (!url) {
        alert('请填写 Shell 地址');
        return;
    }
    var password = (document.getElementById('webshell-password') || {}).value;
    if (password && typeof password.trim === 'function') password = password.trim(); else password = '';
    var type = (document.getElementById('webshell-type') || {}).value || 'php';
    var method = ((document.getElementById('webshell-method') || {}).value || 'post').toLowerCase();
    var cmdParam = (document.getElementById('webshell-cmd-param') || {}).value;
    if (cmdParam && typeof cmdParam.trim === 'function') cmdParam = cmdParam.trim(); else cmdParam = '';
    var remark = (document.getElementById('webshell-remark') || {}).value;
    if (remark && typeof remark.trim === 'function') remark = remark.trim(); else remark = '';

    var editIdEl = document.getElementById('webshell-edit-id');
    var editId = editIdEl ? editIdEl.value.trim() : '';
    var body = { url: url, password: password, type: type, method: method === 'get' ? 'get' : 'post', cmd_param: cmdParam, remark: remark || url };
    if (typeof apiFetch === 'undefined') return;

    var reqUrl = editId ? ('/api/webshell/connections/' + encodeURIComponent(editId)) : '/api/webshell/connections';
    var reqMethod = editId ? 'PUT' : 'POST';
    apiFetch(reqUrl, {
        method: reqMethod,
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body)
    })
        .then(function (r) { return r.json(); })
        .then(function () {
            closeWebshellModal();
            return refreshWebshellConnectionsFromServer();
        })
        .then(function (list) {
            // 若编辑的是当前选中的连接，同步更新 webshellCurrentConn，使终端/文件管理立即使用新配置
            if (editId && currentWebshellId === editId && Array.isArray(list)) {
                var updated = list.find(function (c) { return c.id === editId; });
                if (updated) webshellCurrentConn = updated;
            }
        })
        .catch(function (e) {
            console.warn('保存 WebShell 连接失败', e);
            alert(e && e.message ? e.message : '保存失败');
        });
}
