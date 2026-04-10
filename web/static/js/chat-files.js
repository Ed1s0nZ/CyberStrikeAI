// 对话附件（chat_uploads）文件管理

let chatFilesCache = [];
/** 后端 GET /api/chat-uploads 返回的目录相对路径（含空文件夹），与 files 合并成树 */
let chatFilesFoldersCache = [];
let chatFilesDisplayed = [];
let chatFilesEditRelativePath = '';
let chatFilesRenameRelativePath = '';

const CHAT_FILES_GROUP_STORAGE_KEY = 'csai_chat_files_group_by';
const CHAT_FILES_BROWSE_PATH_KEY = 'csai_chat_files_browse_path';

/** 按文件夹浏览模式下的当前路径（相对 chat_uploads 的段数组），如 ['2024-03-21','uuid'] */
let chatFilesBrowsePath = [];
/** 非空时，下一次上传文件落到此相对路径（chat_uploads 下目录），如 2026-03-21/uuid/sub */
let chatFilesPendingUploadDir = '';
/** 文件管理页面向服务器上传进行中，避免重复选择并禁用顶栏按钮 */
let chatFilesXHRUploadBusy = false;

function chatFilesLoadBrowsePathFromStorage() {
    try {
        const raw = localStorage.getItem(CHAT_FILES_BROWSE_PATH_KEY);
        if (!raw) {
            chatFilesBrowsePath = [];
            return;
        }
        const p = JSON.parse(raw);
        if (Array.isArray(p) && p.every(function (x) {
            return typeof x === 'string';
        })) {
            chatFilesBrowsePath = p;
        }
    } catch (e) {
        chatFilesBrowsePath = [];
    }
}

function chatFilesSetBrowsePath(path) {
    chatFilesBrowsePath = path.slice();
    try {
        localStorage.setItem(CHAT_FILES_BROWSE_PATH_KEY, JSON.stringify(chatFilesBrowsePath));
    } catch (e) {
        /* ignore */
    }
}

function chatFilesResolveTreeNode(root, path) {
    let node = root;
    let i;
    for (i = 0; i < path.length; i++) {
        const seg = path[i];
        if (!node.dirs[seg]) return null;
        node = node.dirs[seg];
    }
    return node;
}

function chatFilesNormalizeBrowsePathForTree(root) {
    let path = chatFilesBrowsePath.slice();
    while (path.length > 0 && !chatFilesResolveTreeNode(root, path)) {
        path.pop();
    }
    if (path.length !== chatFilesBrowsePath.length) {
        chatFilesSetBrowsePath(path);
    }
}

function initChatFilesPage() {
    chatFilesLoadBrowsePathFromStorage();
    try {
        localStorage.removeItem('csai_chat_files_synthetic_dirs');
    } catch (e) {
        /* ignore */
    }
    ensureChatFilesDocClickClose();
    const sel = document.getElementById('chat-files-group-by');
    if (sel) {
        try {
            const v = localStorage.getItem(CHAT_FILES_GROUP_STORAGE_KEY);
            if (v === 'none' || v === 'date' || v === 'conversation' || v === 'folder') {
                sel.value = v;
            }
        } catch (e) {
            /* ignore */
        }
    }
    loadChatFilesPage();
}

function chatFilesCloseAllMenus() {
    document.querySelectorAll('.chat-files-dropdown').forEach((el) => {
        el.hidden = true;
        el.style.position = '';
        el.style.left = '';
        el.style.top = '';
        el.style.right = '';
        el.style.minWidth = '';
        el.style.zIndex = '';
        el.classList.remove('chat-files-dropdown-fixed');
    });
}

/**
 * 「更多」菜单使用 fixed 定位，避免表格外层 overflow 把菜单裁成一条细线。
 */
function chatFilesToggleMoreMenu(ev, idx) {
    if (ev) ev.stopPropagation();
    const menu = document.getElementById('chat-files-menu-' + idx);
    const btn = ev && ev.currentTarget;
    if (!menu) return;
    const opening = menu.hidden;
    chatFilesCloseAllMenus();
    if (!opening) return;

    menu.hidden = false;
    menu.classList.add('chat-files-dropdown-fixed');
    if (!btn || typeof btn.getBoundingClientRect !== 'function') return;

    requestAnimationFrame(() => {
        const r = btn.getBoundingClientRect();
        const vw = window.innerWidth;
        const vh = window.innerHeight;
        const margin = 8;
        const minW = 220;
        menu.style.boxSizing = 'border-box';
        menu.style.position = 'fixed';
        menu.style.zIndex = '5000';
        menu.style.minWidth = minW + 'px';
        menu.style.right = 'auto';

        const w = Math.max(minW, menu.offsetWidth || minW);
        let left = r.right - w;
        if (left < margin) left = margin;
        if (left + w > vw - margin) left = Math.max(margin, vw - margin - w);
        menu.style.left = left + 'px';

        const gap = 6;
        let top = r.bottom + gap;
        const estH = menu.offsetHeight || 120;
        if (top + estH > vh - margin && r.top - gap - estH >= margin) {
            top = r.top - gap - estH;
        }
        menu.style.top = top + 'px';
    });
}

window.chatFilesCloseAllMenus = chatFilesCloseAllMenus;
window.chatFilesToggleMoreMenu = chatFilesToggleMoreMenu;

function ensureChatFilesDocClickClose() {
    if (window.__chatFilesDocClose) return;
    window.__chatFilesDocClose = true;
    document.addEventListener('click', function (ev) {
        if (ev.target.closest && ev.target.closest('.chat-files-dropdown-wrap')) return;
        chatFilesCloseAllMenus();
    });
    document.addEventListener('keydown', function (ev) {
        if (ev.key === 'Escape') chatFilesCloseAllMenus();
    });
    window.addEventListener(
        'scroll',
        function () {
            chatFilesCloseAllMenus();
        },
        true
    );
    window.addEventListener('resize', function () {
        chatFilesCloseAllMenus();
    });
}

async function loadChatFilesPage() {
    const wrap = document.getElementById('chat-files-list-wrap');
    if (!wrap) return;
    wrap.classList.remove('chat-files-table-wrap--grouped');
    wrap.classList.remove('chat-files-table-wrap--tree');
    wrap.innerHTML = '<div class="loading-spinner" data-i18n="common.loading">加载中…</div>';
    if (typeof window.applyTranslations === 'function') {
        window.applyTranslations(wrap);
    }

    const conv = document.getElementById('chat-files-filter-conv');
    const convQ = conv ? conv.value.trim() : '';
    let url = '/api/chat-uploads';
    if (convQ) {
        url += '?conversation=' + encodeURIComponent(convQ);
    }

    try {
        const res = await apiFetch(url);
        if (!res.ok) {
            const t = await res.text();
            throw new Error(t || res.status);
        }
        const data = await res.json();
        chatFilesCache = Array.isArray(data.files) ? data.files : [];
        chatFilesFoldersCache = Array.isArray(data.folders) ? data.folders : [];
        renderChatFilesTable();
    } catch (e) {
        console.error(e);
        wrap.classList.remove('chat-files-table-wrap--grouped');
        wrap.classList.remove('chat-files-table-wrap--tree');
        const msg = (typeof window.t === 'function') ? window.t('chatFilesPage.errorLoad') : '加载失败';
        wrap.innerHTML = '<div class="error-message">' + escapeHtml(msg + ': ' + (e.message || String(e))) + '</div>';
    }
}

function chatFilesNameFilter(files) {
    const el = document.getElementById('chat-files-filter-name');
    const q = el ? el.value.trim().toLowerCase() : '';
    if (!q) return files;
    return files.filter(function (f) {
        const name = (f.name || '').toLowerCase();
        const sub = (f.subPath || '').toLowerCase();
        return name.includes(q) || sub.includes(q);
    });
}

/** 仅前端按文件名筛选，不重新请求 */
function chatFilesFilterNameOnInput() {
    if (!chatFilesCache.length && !chatFilesFoldersCache.length && chatFilesGetGroupByMode() !== 'folder') return;
    renderChatFilesTable();
}

function formatChatFileBytes(n) {
    if (n < 1024) return n + ' B';
    if (n < 1024 * 1024) return (n / 1024).toFixed(1) + ' KB';
    return (n / (1024 * 1024)).toFixed(1) + ' MB';
}

function chatFilesShowToast(message) {
    const el = document.createElement('div');
    el.className = 'chat-files-toast';
    el.setAttribute('role', 'status');
    el.textContent = message;
    document.body.appendChild(el);
    requestAnimationFrame(() => el.classList.add('chat-files-toast-visible'));
    setTimeout(() => {
        el.classList.remove('chat-files-toast-visible');
        setTimeout(() => el.remove(), 300);
    }, 2200);
}

async function chatFilesCopyText(text) {
    try {
        if (navigator.clipboard && typeof navigator.clipboard.writeText === 'function') {
            await navigator.clipboard.writeText(text);
            return true;
        }
    } catch (e) {
        /* fall through */
    }
    try {
        const ta = document.createElement('textarea');
        ta.value = text;
        ta.setAttribute('readonly', '');
        ta.style.position = 'fixed';
        ta.style.left = '-9999px';
        document.body.appendChild(ta);
        ta.select();
        const ok = document.execCommand('copy');
        document.body.removeChild(ta);
        return ok;
    } catch (e2) {
        return false;
    }
}

async function copyChatFilePathIdx(idx) {
    const f = chatFilesDisplayed[idx];
    if (!f) return;
    const text = (f.absolutePath && String(f.absolutePath).trim())
        ? String(f.absolutePath).trim()
        : ('chat_uploads/' + String(f.relativePath || '').replace(/^\/+/, ''));
    const ok = await chatFilesCopyText(text);
    if (ok) {
        const msg = (typeof window.t === 'function') ? window.t('chatFilesPage.pathCopied') : '路径已复制，可粘贴到对话中引用';
        chatFilesShowToast(msg);
    } else {
        const fail = (typeof window.t === 'function') ? window.t('common.copyFailed') : '复制失败';
        alert(fail);
    }
}

/** 常见二进制扩展名：此类文件无法在纯文本编辑器中打开 */
const CHAT_FILES_BINARY_EXT = new Set([
    'png', 'jpg', 'jpeg', 'gif', 'webp', 'bmp', 'ico', 'tif', 'tiff', 'heic', 'heif', 'svgz',
    'pdf', 'zip', 'rar', '7z', 'tar', 'gz', 'bz2', 'xz', 'zst',
    'mp3', 'm4a', 'wav', 'ogg', 'flac', 'aac',
    'mp4', 'avi', 'mkv', 'mov', 'wmv', 'webm', 'm4v',
    'exe', 'dll', 'so', 'dylib', 'bin', 'app', 'dmg', 'pkg',
    'woff', 'woff2', 'ttf', 'otf', 'eot',
    'sqlite', 'db', 'sqlite3',
    'doc', 'docx', 'xls', 'xlsx', 'ppt', 'pptx', 'odt', 'ods',
    'class', 'jar', 'war', 'apk', 'ipa',
    'iso', 'img'
]);

function chatFileIsBinaryByName(fileName) {
    if (!fileName || typeof fileName !== 'string') return false;
    const i = fileName.lastIndexOf('.');
    if (i < 0 || i === fileName.length - 1) return false;
    const ext = fileName.slice(i + 1).toLowerCase();
    return CHAT_FILES_BINARY_EXT.has(ext);
}

function chatFilesEditBlockedHint() {
    return (typeof window.t === 'function')
        ? window.t('chatFilesPage.editBinaryHint')
        : '图片、压缩包等二进制文件无法在此以文本方式编辑，请使用「下载」。';
}

function chatFilesAlertMessage(raw) {
    const s = (raw == null) ? '' : String(raw).trim();
    const lower = s.toLowerCase();
    if (lower.includes('binary file not editable') || lower.includes('binary')) {
        return chatFilesEditBlockedHint();
    }
    if (lower.includes('file too large') || lower.includes('entity too large') || lower.includes('413')) {
        return (typeof window.t === 'function') ? window.t('chatFilesPage.editTooLarge') : '文件过大，无法在此编辑。';
    }
    return s || ((typeof window.t === 'function') ? window.t('chatFilesPage.errorGeneric') : '操作失败');
}

function chatFilesGetGroupByMode() {
    const sel = document.getElementById('chat-files-group-by');
    const v = sel ? sel.value : 'none';
    if (v === 'date' || v === 'conversation' || v === 'folder') return v;
    return 'none';
}

function chatFilesGroupByChange() {
    const sel = document.getElementById('chat-files-group-by');
    if (sel) {
        try {
            localStorage.setItem(CHAT_FILES_GROUP_STORAGE_KEY, sel.value);
        } catch (e) {
            /* ignore */
        }
    }
    renderChatFilesTable();
}

function chatFilesCompareDateKeysDesc(a, b) {
    const as = String(a);
    const bs = String(b);
    if (as === '—' && bs !== '—') return 1;
    if (bs === '—' && as !== '—') return -1;
    return bs.localeCompare(as);
}

/** 目录树节点：dirs[段名] -> 子节点；files: { idx, name }[] */
function chatFilesTreeMakeNode() {
    return { dirs: {}, files: [] };
}

function chatFilesTreeInsertFile(root, f, idx) {
    const rp = String(f.relativePath || '').replace(/\\/g, '/').replace(/^\/+/, '');
    if (!rp) return;
    const parts = rp.split('/').filter(function (p) {
        return p.length > 0;
    });
    if (parts.length < 2) return;
    let node = root;
    for (let i = 0; i < parts.length - 1; i++) {
        const seg = parts[i];
        if (!node.dirs[seg]) node.dirs[seg] = chatFilesTreeMakeNode();
        node = node.dirs[seg];
    }
    node.files.push({ idx: idx, name: parts[parts.length - 1] });
}

function chatFilesBuildTree(files) {
    const root = chatFilesTreeMakeNode();
    files.forEach(function (f, idx) {
        chatFilesTreeInsertFile(root, f, idx);
    });
    return root;
}

/** 将后端返回的目录相对路径（如 a/b/c）并入树，便于展示空文件夹 */
function chatFilesTreeInsertFolderPath(root, relSlash) {
    const rp = String(relSlash || '').replace(/\\/g, '/').replace(/^\/+/, '');
    if (!rp) return;
    const parts = rp.split('/').filter(function (p) {
        return p.length > 0;
    });
    if (!parts.length) return;
    let node = root;
    let i;
    for (i = 0; i < parts.length; i++) {
        const seg = parts[i];
        if (!node.dirs[seg]) node.dirs[seg] = chatFilesTreeMakeNode();
        node = node.dirs[seg];
    }
}

function chatFilesMergeFoldersIntoTree(root, folderPaths) {
    if (!Array.isArray(folderPaths)) return;
    let i;
    for (i = 0; i < folderPaths.length; i++) {
        chatFilesTreeInsertFolderPath(root, folderPaths[i]);
    }
}

function chatFilesTreeRootMerged() {
    const root = chatFilesBuildTree(chatFilesDisplayed);
    chatFilesMergeFoldersIntoTree(root, chatFilesFoldersCache);
    return root;
}

function chatFilesTreeNodeMaxMod(node) {
    let m = 0;
    let i;
    for (i = 0; i < node.files.length; i++) {
        const f = chatFilesDisplayed[node.files[i].idx];
        m = Math.max(m, (f && f.modifiedUnix) || 0);
    }
    const keys = Object.keys(node.dirs);
    for (i = 0; i < keys.length; i++) {
        m = Math.max(m, chatFilesTreeNodeMaxMod(node.dirs[keys[i]]));
    }
    return m;
}

function chatFilesTreeSortDirKeys(node, keys) {
    return keys.slice().sort(function (a, b) {
        const ma = chatFilesTreeNodeMaxMod(node.dirs[a]);
        const mb = chatFilesTreeNodeMaxMod(node.dirs[b]);
        if (mb !== ma) return mb - ma;
        return String(a).localeCompare(String(b));
    });
}

function chatFilesBuildGroups(files, mode) {
    const map = new Map();
    files.forEach(function (f, idx) {
        const key = mode === 'date' ? (f.date || '—') : (f.conversationId || '—');
        if (!map.has(key)) {
            map.set(key, { key: key, items: [] });
        }
        map.get(key).items.push({ idx: idx, f: f });
    });
    const groups = Array.from(map.values());
    groups.forEach(function (g) {
        g.items.sort(function (a, b) {
            return (b.f.modifiedUnix || 0) - (a.f.modifiedUnix || 0);
        });
    });
    if (mode === 'date') {
        groups.sort(function (a, b) {
            return chatFilesCompareDateKeysDesc(a.key, b.key);
        });
    } else {
        groups.sort(function (a, b) {
            const ma = Math.max.apply(
                null,
                a.items.map(function (x) {
                    return x.f.modifiedUnix || 0;
                })
            );
            const mb = Math.max.apply(
                null,
                b.items.map(function (x) {
                    return x.f.modifiedUnix || 0;
                })
            );
            return mb - ma;
        });
    }
    return groups;
}

/** 分组标题：会话 ID 过长时缩短展示，完整值放在 title */
function chatFilesGroupHeadingConversation(key) {
    const c = key == null ? '' : String(key);
    if (c === '' || c === '—') {
        return { text: '—', title: '' };
    }
    if (typeof window.t === 'function') {
        if (c === '_manual') {
            return { text: window.t('chatFilesPage.convManual'), title: '_manual' };
        }
        if (c === '_new') {
            return { text: window.t('chatFilesPage.convNew'), title: '_new' };
        }
    }
    if (c.length > 36) {
        return { text: c.slice(0, 8) + '…' + c.slice(-6), title: c };
    }
    return { text: c, title: c };
}

function renderChatFilesTable() {
    const wrap = document.getElementById('chat-files-list-wrap');
    if (!wrap) return;

    chatFilesDisplayed = chatFilesNameFilter(chatFilesCache);
    const groupMode = chatFilesGetGroupByMode();
    const emptyMsg = (typeof window.t === 'function') ? window.t('chatFilesPage.empty') : '暂无文件';
    // 「按文件夹」模式下即使尚无文件，也要显示 chat_uploads 路径栏与「新建文件夹」，否则无法先建目录
    if (!chatFilesDisplayed.length && groupMode !== 'folder') {
        wrap.classList.remove('chat-files-table-wrap--grouped');
        wrap.classList.remove('chat-files-table-wrap--tree');
        wrap.innerHTML = '<div class="empty-state" data-i18n="chatFilesPage.empty">' + escapeHtml(emptyMsg) + '</div>';
        if (typeof window.applyTranslations === 'function') {
            window.applyTranslations(wrap);
        }
        return;
    }

    const thDate = (typeof window.t === 'function') ? window.t('chatFilesPage.colDate') : '日期';
    const thConv = (typeof window.t === 'function') ? window.t('chatFilesPage.colConversation') : '会话';
    const thSubPath = (typeof window.t === 'function') ? window.t('chatFilesPage.colSubPath') : '子路径';
    const thName = (typeof window.t === 'function') ? window.t('chatFilesPage.colName') : '文件名';
    const thSize = (typeof window.t === 'function') ? window.t('chatFilesPage.colSize') : '大小';
    const thModified = (typeof window.t === 'function') ? window.t('chatFilesPage.colModified') : '修改时间';
    const thActions = (typeof window.t === 'function') ? window.t('chatFilesPage.colActions') : '操作';

    const svgCopy = '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>';
    const svgDownload = '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>';
    const svgMore = '<svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><circle cx="12" cy="5" r="1.5"/><circle cx="12" cy="12" r="1.5"/><circle cx="12" cy="19" r="1.5"/></svg>';
    const svgFolder = '<svg class="chat-files-tree-icon" width="16" height="16" viewBox="0 0 24 24" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/></svg>';
    const svgFile = '<svg class="chat-files-tree-file-icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/></svg>';

    const tCopyTitle = escapeHtml((typeof window.t === 'function') ? window.t('chatFilesPage.copyPathTitle') : '复制服务器上的绝对路径，可粘贴到对话中引用');
    const tDlTitle = escapeHtml((typeof window.t === 'function') ? window.t('chatFilesPage.download') : '下载');
    const tMoreTitle = escapeHtml((typeof window.t === 'function') ? window.t('chatFilesPage.moreActions') : '更多操作');

    function rowHtml(f, idx) {
        const rp = f.relativePath || '';
        const pathForTitle = (f.absolutePath && String(f.absolutePath).trim()) ? String(f.absolutePath).trim() : rp;
        const nameEsc = escapeHtml(f.name || '');
        const conv = f.conversationId || '';
        const convEsc = escapeHtml(conv);
        const dt = f.modifiedUnix ? new Date(f.modifiedUnix * 1000).toLocaleString() : '—';
        const canOpenChat = conv && conv !== '_manual' && conv !== '_new';

        const bin = chatFileIsBinaryByName(f.name);
        const editHint = escapeHtml(chatFilesEditBlockedHint());
        const editUnavailable = (typeof window.t === 'function') ? escapeHtml(window.t('chatFilesPage.editUnavailable')) : '不可编辑';
        const tEdit = (typeof window.t === 'function') ? escapeHtml(window.t('chatFilesPage.edit')) : '编辑';
        const tOpenChat = (typeof window.t === 'function') ? escapeHtml(window.t('chatFilesPage.openChat')) : '打开对话';
        const tRename = (typeof window.t === 'function') ? escapeHtml(window.t('chatFilesPage.rename')) : '重命名';
        const tDelete = (typeof window.t === 'function') ? escapeHtml(window.t('common.delete')) : '删除';

        const menuParts = [];
        if (canOpenChat) {
            menuParts.push(`<button type="button" class="chat-files-dropdown-item" onclick="chatFilesCloseAllMenus(); openChatFilesConversationIdx(${idx});">${tOpenChat}</button>`);
        }
        if (!bin) {
            menuParts.push(`<button type="button" class="chat-files-dropdown-item" onclick="chatFilesCloseAllMenus(); openChatFilesEditIdx(${idx});">${tEdit}</button>`);
        } else {
            menuParts.push(`<div class="chat-files-dropdown-item is-disabled" title="${editHint}">${editUnavailable}</div>`);
        }
        menuParts.push(`<button type="button" class="chat-files-dropdown-item" onclick="chatFilesCloseAllMenus(); openChatFilesRenameIdx(${idx});">${tRename}</button>`);
        menuParts.push(`<button type="button" class="chat-files-dropdown-item is-danger" onclick="chatFilesCloseAllMenus(); deleteChatFileIdx(${idx});">${tDelete}</button>`);
        const menuHtml = menuParts.join('');

        const subRaw = (f.subPath && String(f.subPath).trim()) ? String(f.subPath).trim() : '';
        const rootLabel = (typeof window.t === 'function') ? window.t('chatFilesPage.folderRoot') : '（根目录）';
        let subCellInner;
        if (subRaw) {
            const segs = subRaw.split('/').filter(function (s) {
                return s.length > 0;
            });
            subCellInner = '<span class="chat-files-path-breadcrumb">' + segs.map(function (seg, i) {
                return (i > 0 ? '<span class="chat-files-path-sep">›</span>' : '') +
                    '<span class="chat-files-path-crumb">' + escapeHtml(seg) + '</span>';
            }).join('') + '</span>';
        } else {
            subCellInner = '<span class="chat-files-path-root">' + escapeHtml(rootLabel) + '</span>';
        }

        return `<tr>
            <td>${escapeHtml(f.date || '—')}</td>
            <td class="chat-files-cell-conv"><code title="${convEsc}">${convEsc}</code></td>
            <td class="chat-files-cell-subpath" title="${escapeHtml(subRaw || '')}">${subCellInner}</td>
            <td class="chat-files-cell-name" title="${escapeHtml(pathForTitle)}">${nameEsc}</td>
            <td>${formatChatFileBytes(f.size || 0)}</td>
            <td>${escapeHtml(dt)}</td>
            <td class="chat-files-actions">
                <div class="chat-files-action-bar">
                    <button type="button" class="btn-icon" title="${tCopyTitle}" onclick="copyChatFilePathIdx(${idx})">${svgCopy}</button>
                    <button type="button" class="btn-icon" title="${tDlTitle}" onclick="downloadChatFileIdx(${idx})">${svgDownload}</button>
                    <div class="chat-files-dropdown-wrap">
                        <button type="button" class="btn-icon" title="${tMoreTitle}" aria-haspopup="true" onclick="chatFilesToggleMoreMenu(event, ${idx})">${svgMore}</button>
                        <div class="chat-files-dropdown" id="chat-files-menu-${idx}" hidden>${menuHtml}</div>
                    </div>
                </div>
            </td>
        </tr>`;
    }

    const theadHtml = `<thead><tr>
        <th>${escapeHtml(thDate)}</th>
        <th>${escapeHtml(thConv)}</th>
        <th>${escapeHtml(thSubPath)}</th>
        <th>${escapeHtml(thName)}</th>
        <th>${escapeHtml(thSize)}</th>
        <th>${escapeHtml(thModified)}</th>
        <th>${escapeHtml(thActions)}</th>
    </tr></thead>`;

    const theadCompact = `<thead><tr>
        <th>${escapeHtml(thName)}</th>
        <th>${escapeHtml(thSize)}</th>
        <th>${escapeHtml(thModified)}</th>
        <th>${escapeHtml(thActions)}</th>
    </tr></thead>`;

    let innerHtml;

    if (groupMode === 'folder') {
        const root = chatFilesTreeRootMerged();
        chatFilesNormalizeBrowsePathForTree(root);
        const node = chatFilesResolveTreeNode(root, chatFilesBrowsePath);
        const current = node || root;
        const dirKeys = chatFilesTreeSortDirKeys(current, Object.keys(current.dirs));

        current.files.sort(function (a, b) {
            return (chatFilesDisplayed[b.idx].modifiedUnix || 0) - (chatFilesDisplayed[a.idx].modifiedUnix || 0);
        });

        const tRoot = escapeHtml((typeof window.t === 'function') ? window.t('chatFilesPage.browseRoot') : 'chat_uploads');
        const tUp = escapeHtml((typeof window.t === 'function') ? window.t('chatFilesPage.browseUp') : '上级');
        const tMkdir = escapeHtml((typeof window.t === 'function') ? window.t('chatFilesPage.newFolderButton') : '新建文件夹');
        const tEmpty = escapeHtml((typeof window.t === 'function') ? window.t('chatFilesPage.folderEmpty') : '此文件夹为空');
        const tCopyFolder = escapeHtml((typeof window.t === 'function') ? window.t('chatFilesPage.copyFolderPathTitle') : '复制 chat_uploads 下相对路径');
        const tEnter = escapeHtml((typeof window.t === 'function') ? window.t('chatFilesPage.enterFolderTitle') : '进入');

        let breadcrumbHtml = '<nav class="chat-files-breadcrumb" aria-label="breadcrumb">';
        breadcrumbHtml += '<button type="button" class="chat-files-breadcrumb-link" onclick="chatFilesNavigateBreadcrumb(-1)">' + tRoot + '</button>';
        let bi;
        for (bi = 0; bi < chatFilesBrowsePath.length; bi++) {
            const seg = chatFilesBrowsePath[bi];
            const isLast = bi === chatFilesBrowsePath.length - 1;
            breadcrumbHtml += '<span class="chat-files-breadcrumb-sep">/</span>';
            if (isLast) {
                breadcrumbHtml += '<span class="chat-files-breadcrumb-current">' + escapeHtml(seg) + '</span>';
            } else {
                breadcrumbHtml += '<button type="button" class="chat-files-breadcrumb-link" onclick="chatFilesNavigateBreadcrumb(' + bi + ')">' + escapeHtml(seg) + '</button>';
            }
        }
        breadcrumbHtml += '</nav>';

        const upDisabled = chatFilesBrowsePath.length === 0 ? ' disabled' : '';
        const toolbarHtml = '<div class="chat-files-browse-toolbar">' + breadcrumbHtml +
            '<button type="button" class="btn-secondary chat-files-mkdir-btn" onclick="openChatFilesMkdirModal()">' + tMkdir + '</button>' +
            '<button type="button" class="btn-secondary chat-files-browse-up"' + upDisabled + ' onclick="chatFilesNavigateUp()">' + tUp + '</button></div>';

        const svgTrash = '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>';
        const svgUploadToFolder = '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" y1="3" x2="12" y2="15"/></svg>';
        const tDeleteFolder = escapeHtml((typeof window.t === 'function') ? window.t('chatFilesPage.deleteFolderTitle') : '删除文件夹');
        const tUploadToFolder = escapeHtml((typeof window.t === 'function') ? window.t('chatFilesPage.uploadToFolderTitle') : '上传到此文件夹');

        function rowHtmlBrowseFolder(name) {
            const nameAttr = encodeURIComponent(String(name));
            const relToFolder = chatFilesBrowsePath.concat([name]).join('/');
            const uploadDirAttr = encodeURIComponent(relToFolder);
            return `<tr class="chat-files-tr-folder chat-files-tr-folder--nav" role="button" tabindex="0" data-chat-folder-name="${nameAttr}" onclick="chatFilesOnFolderRowClick(event)" onkeydown="chatFilesOnFolderRowKeydown(event)">
                <td class="chat-files-tree-name-cell chat-files-tree-name-cell--folder" title="${tEnter}">
                    <span class="chat-files-tree-name-inner">${svgFolder}<span class="chat-files-tree-name-text">${escapeHtml(name)}</span></span>
                </td>
                <td class="chat-files-tree-muted">—</td>
                <td class="chat-files-tree-muted">—</td>
                <td class="chat-files-actions" data-chat-files-stop="true" onclick="event.stopPropagation()">
                    <div class="chat-files-action-bar">
                        <button type="button" class="btn-icon" title="${tUploadToFolder}" data-upload-dir="${uploadDirAttr}" onclick="chatFilesUploadToFolderClick(event, this)">${svgUploadToFolder}</button>
                        <button type="button" class="btn-icon" title="${tCopyFolder}" data-chat-folder-name="${nameAttr}" onclick="chatFilesCopyFolderPathFromBtn(event, this)">${svgCopy}</button>
                        <button type="button" class="btn-icon btn-danger" title="${tDeleteFolder}" data-chat-folder-name="${nameAttr}" onclick="chatFilesDeleteFolderFromBtn(event, this)">${svgTrash}</button>
                    </div>
                </td>
            </tr>`;
        }

        function rowHtmlTreeFile(f, idx) {
            const pathForTitle = (f.absolutePath && String(f.absolutePath).trim()) ? String(f.absolutePath).trim() : (f.relativePath || '');
            const nameEsc = escapeHtml(f.name || '');
            const dt = f.modifiedUnix ? new Date(f.modifiedUnix * 1000).toLocaleString() : '—';
            const conv = f.conversationId || '';
            const canOpenChat = conv && conv !== '_manual' && conv !== '_new';

            const bin = chatFileIsBinaryByName(f.name);
            const editHint = escapeHtml(chatFilesEditBlockedHint());
            const editUnavailable = (typeof window.t === 'function') ? escapeHtml(window.t('chatFilesPage.editUnavailable')) : '不可编辑';
            const tEdit = (typeof window.t === 'function') ? escapeHtml(window.t('chatFilesPage.edit')) : '编辑';
            const tOpenChat = (typeof window.t === 'function') ? escapeHtml(window.t('chatFilesPage.openChat')) : '打开对话';
            const tRename = (typeof window.t === 'function') ? escapeHtml(window.t('chatFilesPage.rename')) : '重命名';
            const tDelete = (typeof window.t === 'function') ? escapeHtml(window.t('common.delete')) : '删除';

            const menuParts = [];
            if (canOpenChat) {
                menuParts.push(`<button type="button" class="chat-files-dropdown-item" onclick="chatFilesCloseAllMenus(); openChatFilesConversationIdx(${idx});">${tOpenChat}</button>`);
            }
            if (!bin) {
                menuParts.push(`<button type="button" class="chat-files-dropdown-item" onclick="chatFilesCloseAllMenus(); openChatFilesEditIdx(${idx});">${tEdit}</button>`);
            } else {
                menuParts.push(`<div class="chat-files-dropdown-item is-disabled" title="${editHint}">${editUnavailable}</div>`);
            }
            menuParts.push(`<button type="button" class="chat-files-dropdown-item" onclick="chatFilesCloseAllMenus(); openChatFilesRenameIdx(${idx});">${tRename}</button>`);
            menuParts.push(`<button type="button" class="chat-files-dropdown-item is-danger" onclick="chatFilesCloseAllMenus(); deleteChatFileIdx(${idx});">${tDelete}</button>`);
            const menuHtml = menuParts.join('');

            return `<tr class="chat-files-tr-file">
                <td class="chat-files-tree-name-cell" title="${escapeHtml(pathForTitle)}">
                    <span class="chat-files-tree-name-inner">${svgFile}<span class="chat-files-tree-name-text">${nameEsc}</span></span>
                </td>
                <td>${formatChatFileBytes(f.size || 0)}</td>
                <td>${escapeHtml(dt)}</td>
                <td class="chat-files-actions">
                    <div class="chat-files-action-bar">
                        <button type="button" class="btn-icon" title="${tCopyTitle}" onclick="copyChatFilePathIdx(${idx})">${svgCopy}</button>
                        <button type="button" class="btn-icon" title="${tDlTitle}" onclick="downloadChatFileIdx(${idx})">${svgDownload}</button>
                        <div class="chat-files-dropdown-wrap">
                            <button type="button" class="btn-icon" title="${tMoreTitle}" aria-haspopup="true" onclick="chatFilesToggleMoreMenu(event, ${idx})">${svgMore}</button>
                            <div class="chat-files-dropdown" id="chat-files-menu-${idx}" hidden>${menuHtml}</div>
                        </div>
                    </div>
                </td>
            </tr>`;
        }

        const folderRows = dirKeys.map(rowHtmlBrowseFolder).join('');
        const fileRows = current.files.map(function (item) {
            return rowHtmlTreeFile(chatFilesDisplayed[item.idx], item.idx);
        }).join('');

        let bodyRows = folderRows + fileRows;
        if (!bodyRows) {
            bodyRows = '<tr class="chat-files-tr-empty"><td colspan="4" class="chat-files-folder-empty">' + tEmpty + '</td></tr>';
        }

        innerHtml = '<div class="chat-files-browse-wrap">' + toolbarHtml + '<table class="chat-files-table chat-files-table--tree-flat">' + theadCompact + '<tbody>' + bodyRows + '</tbody></table></div>';
    } else if (groupMode === 'none') {
        const rows = chatFilesDisplayed.map(function (f, idx) {
            return rowHtml(f, idx);
        }).join('');
        innerHtml = `<table class="chat-files-table">${theadHtml}<tbody>${rows}</tbody></table>`;
    } else {
        const groups = chatFilesBuildGroups(chatFilesDisplayed, groupMode);
        const blocks = groups.map(function (g) {
            const rows = g.items.map(function (item) {
                return rowHtml(item.f, item.idx);
            }).join('');
            let summaryMain;
            let summaryTitleAttr = '';
            if (groupMode === 'date') {
                summaryMain = escapeHtml(String(g.key));
            } else {
                const h = chatFilesGroupHeadingConversation(g.key);
                summaryMain = escapeHtml(h.text);
                summaryTitleAttr = h.title ? ' title="' + escapeHtml(h.title) + '"' : '';
            }
            const n = g.items.length;
            const countLabel = (typeof window.t === 'function')
                ? escapeHtml(window.t('chatFilesPage.groupCount', { count: n }))
                : escapeHtml(String(n));
            return `<details class="chat-files-group" open>
                <summary class="chat-files-group-summary"${summaryTitleAttr}>
                    <span class="chat-files-group-title">${summaryMain}</span>
                    <span class="chat-files-group-count">${countLabel}</span>
                </summary>
                <div class="chat-files-group-body">
                    <table class="chat-files-table">${theadHtml}<tbody>${rows}</tbody></table>
                </div>
            </details>`;
        }).join('');
        innerHtml = `<div class="chat-files-grouped">${blocks}</div>`;
    }

    ensureChatFilesDocClickClose();

    wrap.innerHTML = innerHtml;
    wrap.classList.toggle('chat-files-table-wrap--grouped', groupMode !== 'none' && groupMode !== 'folder');
    wrap.classList.toggle('chat-files-table-wrap--tree', groupMode === 'folder');
}

window.chatFilesGroupByChange = chatFilesGroupByChange;

function chatFilesNavigateInto(name) {
    const root = chatFilesTreeRootMerged();
    chatFilesNormalizeBrowsePathForTree(root);
    const next = chatFilesBrowsePath.concat([name]);
    if (!chatFilesResolveTreeNode(root, next)) return;
    chatFilesSetBrowsePath(next);
    renderChatFilesTable();
}

function chatFilesNavigateBreadcrumb(level) {
    const root = chatFilesTreeRootMerged();
    chatFilesNormalizeBrowsePathForTree(root);
    if (level < 0) {
        chatFilesSetBrowsePath([]);
    } else {
        chatFilesSetBrowsePath(chatFilesBrowsePath.slice(0, level + 1));
    }
    renderChatFilesTable();
}

function chatFilesNavigateUp() {
    if (chatFilesBrowsePath.length === 0) return;
    chatFilesSetBrowsePath(chatFilesBrowsePath.slice(0, -1));
    renderChatFilesTable();
}

function chatFilesFolderNameFromRow(el) {
    if (!el || !el.getAttribute) return '';
    try {
        return decodeURIComponent(String(el.getAttribute('data-chat-folder-name') || ''));
    } catch (e) {
        return '';
    }
}

function chatFilesOnFolderRowClick(ev) {
    if (ev.target.closest && ev.target.closest('[data-chat-files-stop]')) return;
    const name = chatFilesFolderNameFromRow(ev.currentTarget);
    if (!name) return;
    chatFilesNavigateInto(name);
}

function chatFilesOnFolderRowKeydown(ev) {
    if (ev.key !== 'Enter' && ev.key !== ' ') return;
    ev.preventDefault();
    const name = chatFilesFolderNameFromRow(ev.currentTarget);
    if (!name) return;
    chatFilesNavigateInto(name);
}

function chatFilesCopyFolderPathFromBtn(ev, btn) {
    if (ev) ev.stopPropagation();
    const name = chatFilesFolderNameFromRow(btn);
    if (!name) return;
    copyChatFolderPathFromBrowse(name);
}

async function deleteChatFolderFromBrowse(folderName) {
    const segs = chatFilesBrowsePath.concat([folderName]);
    const rel = segs.join('/');
    const q = (typeof window.t === 'function') ? window.t('chatFilesPage.confirmDeleteFolder') : '确定删除该文件夹及其中的全部文件？';
    if (!confirm(q)) return;
    try {
        const res = await apiFetch('/api/chat-uploads', {
            method: 'DELETE',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path: rel })
        });
        if (!res.ok) {
            const raw = await res.text();
            if (res.status === 404) {
                let errMsg = raw;
                try {
                    const j = JSON.parse(raw);
                    if (j && j.error) errMsg = j.error;
                } catch (eParse) {
                    /* keep raw */
                }
                if (/not\s*found/i.test(String(errMsg))) {
                    loadChatFilesPage();
                    const cleared = (typeof window.t === 'function')
                        ? window.t('chatFilesPage.folderRemovedStale')
                        : '服务器上不存在该目录，列表已刷新。';
                    if (typeof chatFilesShowToast === 'function') {
                        chatFilesShowToast(cleared);
                    } else {
                        alert(cleared);
                    }
                    return;
                }
            }
            throw new Error(raw || String(res.status));
        }
        loadChatFilesPage();
    } catch (e) {
        alert((e && e.message) ? e.message : String(e));
    }
}

function chatFilesDeleteFolderFromBtn(ev, btn) {
    if (ev) ev.stopPropagation();
    const name = chatFilesFolderNameFromRow(btn);
    if (!name) return;
    deleteChatFolderFromBrowse(name);
}

async function copyChatFolderPathFromBrowse(folderName) {
    const segs = chatFilesBrowsePath.concat([folderName]);
    const rel = segs.join('/');
    const text = rel ? ('chat_uploads/' + rel.replace(/^\/+/, '')) : 'chat_uploads';
    const ok = await chatFilesCopyText(text);
    if (ok) {
        const msg = (typeof window.t === 'function') ? window.t('chatFilesPage.folderPathCopied') : '目录路径已复制';
        chatFilesShowToast(msg);
    } else {
        const fail = (typeof window.t === 'function') ? window.t('common.copyFailed') : '复制失败';
        alert(fail);
    }
}

window.chatFilesNavigateInto = chatFilesNavigateInto;
window.chatFilesNavigateBreadcrumb = chatFilesNavigateBreadcrumb;
window.chatFilesNavigateUp = chatFilesNavigateUp;
window.chatFilesOnFolderRowClick = chatFilesOnFolderRowClick;
window.copyChatFolderPathFromBrowse = copyChatFolderPathFromBrowse;
window.chatFilesOnFolderRowKeydown = chatFilesOnFolderRowKeydown;
window.chatFilesCopyFolderPathFromBtn = chatFilesCopyFolderPathFromBtn;
window.chatFilesDeleteFolderFromBtn = chatFilesDeleteFolderFromBtn;
window.chatFilesOpenUploadPicker = chatFilesOpenUploadPicker;
window.chatFilesUploadToFolderClick = chatFilesUploadToFolderClick;
window.openChatFilesMkdirModal = openChatFilesMkdirModal;
window.closeChatFilesMkdirModal = closeChatFilesMkdirModal;
window.submitChatFilesMkdir = submitChatFilesMkdir;

function openChatFilesConversationIdx(idx) {
    const f = chatFilesDisplayed[idx];
    if (!f || !f.conversationId) return;
    openChatFilesConversation(f.conversationId);
}

function downloadChatFileIdx(idx) {
    const f = chatFilesDisplayed[idx];
    if (!f) return;
    downloadChatFile(f.relativePath, f.name);
}

function openChatFilesEditIdx(idx) {
    const f = chatFilesDisplayed[idx];
    if (!f) return;
    if (chatFileIsBinaryByName(f.name)) {
        alert(chatFilesEditBlockedHint());
        return;
    }
    openChatFilesEdit(f.relativePath);
}

function openChatFilesRenameIdx(idx) {
    const f = chatFilesDisplayed[idx];
    if (!f) return;
    openChatFilesRename(f.relativePath, f.name);
}

function deleteChatFileIdx(idx) {
    const f = chatFilesDisplayed[idx];
    if (!f) return;
    deleteChatFile(f.relativePath);
}

function openChatFilesConversation(conversationId) {
    if (!conversationId) return;
    window.location.hash = 'chat?conversation=' + encodeURIComponent(conversationId);
    if (typeof switchPage === 'function') {
        switchPage('chat');
    }
    setTimeout(() => {
        if (typeof loadConversation === 'function') {
            loadConversation(conversationId);
        }
    }, 400);
}

async function downloadChatFile(relativePath, filename) {
    try {
        const url = '/api/chat-uploads/download?path=' + encodeURIComponent(relativePath);
        const res = await apiFetch(url);
        if (!res.ok) {
            throw new Error(await res.text());
        }
        const blob = await res.blob();
        const a = document.createElement('a');
        a.href = URL.createObjectURL(blob);
        a.download = filename || 'download';
        a.click();
        URL.revokeObjectURL(a.href);
    } catch (e) {
        alert((e && e.message) ? e.message : String(e));
    }
}

async function deleteChatFile(relativePath) {
    const q = (typeof window.t === 'function') ? window.t('chatFilesPage.confirmDelete') : '确定删除该文件？';
    if (!confirm(q)) return;
    try {
        const res = await apiFetch('/api/chat-uploads', {
            method: 'DELETE',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path: relativePath })
        });
        if (!res.ok) {
            throw new Error(await res.text());
        }
        loadChatFilesPage();
    } catch (e) {
        alert((e && e.message) ? e.message : String(e));
    }
}

async function openChatFilesEdit(relativePath) {
    chatFilesEditRelativePath = relativePath;
    const pathEl = document.getElementById('chat-files-edit-path');
    const ta = document.getElementById('chat-files-edit-textarea');
    const modal = document.getElementById('chat-files-edit-modal');
    if (pathEl) pathEl.textContent = relativePath;
    if (ta) ta.value = '';
    if (modal) modal.style.display = 'block';

    try {
        const res = await apiFetch('/api/chat-uploads/content?path=' + encodeURIComponent(relativePath));
        if (!res.ok) {
            let errText = '';
            try {
                const err = await res.json();
                errText = err.error || JSON.stringify(err);
            } catch (e2) {
                errText = await res.text();
            }
            throw new Error(errText || res.status);
        }
        const data = await res.json();
        if (ta) ta.value = data.content != null ? String(data.content) : '';
    } catch (e) {
        if (modal) modal.style.display = 'none';
        alert(chatFilesAlertMessage(e && e.message));
    }
}

function closeChatFilesEditModal() {
    const modal = document.getElementById('chat-files-edit-modal');
    if (modal) modal.style.display = 'none';
    chatFilesEditRelativePath = '';
}

async function saveChatFilesEdit() {
    const ta = document.getElementById('chat-files-edit-textarea');
    if (!ta || !chatFilesEditRelativePath) return;
    try {
        const res = await apiFetch('/api/chat-uploads/content', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path: chatFilesEditRelativePath, content: ta.value })
        });
        if (!res.ok) {
            throw new Error(await res.text());
        }
        closeChatFilesEditModal();
        loadChatFilesPage();
    } catch (e) {
        alert(chatFilesAlertMessage(e && e.message));
    }
}

function openChatFilesRename(relativePath, currentName) {
    chatFilesRenameRelativePath = relativePath;
    const input = document.getElementById('chat-files-rename-input');
    const modal = document.getElementById('chat-files-rename-modal');
    if (input) input.value = currentName || '';
    if (modal) modal.style.display = 'block';
    setTimeout(() => { if (input) input.focus(); }, 100);
}

function closeChatFilesRenameModal() {
    const modal = document.getElementById('chat-files-rename-modal');
    if (modal) modal.style.display = 'none';
    chatFilesRenameRelativePath = '';
}

async function submitChatFilesRename() {
    const input = document.getElementById('chat-files-rename-input');
    const newName = input ? input.value.trim() : '';
    if (!newName || !chatFilesRenameRelativePath) {
        closeChatFilesRenameModal();
        return;
    }
    try {
        const res = await apiFetch('/api/chat-uploads/rename', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path: chatFilesRenameRelativePath, newName: newName })
        });
        if (!res.ok) {
            throw new Error(await res.text());
        }
        closeChatFilesRenameModal();
        loadChatFilesPage();
    } catch (e) {
        alert((e && e.message) ? e.message : String(e));
    }
}

function openChatFilesMkdirModal() {
    if (chatFilesGetGroupByMode() !== 'folder') return;
    const hint = document.getElementById('chat-files-mkdir-parent-hint');
    const input = document.getElementById('chat-files-mkdir-input');
    const modal = document.getElementById('chat-files-mkdir-modal');
    const p = chatFilesBrowsePath.join('/');
    if (hint) hint.textContent = p ? ('chat_uploads/' + p) : 'chat_uploads';
    if (input) input.value = '';
    if (modal) modal.style.display = 'block';
    if (modal && typeof window.applyTranslations === 'function') {
        window.applyTranslations(modal);
    }
    setTimeout(() => {
        if (input) input.focus();
    }, 100);
}

function closeChatFilesMkdirModal() {
    const modal = document.getElementById('chat-files-mkdir-modal');
    if (modal) modal.style.display = 'none';
    const input = document.getElementById('chat-files-mkdir-input');
    if (input) input.value = '';
}

async function submitChatFilesMkdir() {
    const input = document.getElementById('chat-files-mkdir-input');
    const name = input ? String(input.value).trim() : '';
    if (!name) {
        closeChatFilesMkdirModal();
        return;
    }
    if (name.includes('/') || name.includes('\\') || name === '.' || name === '..') {
        const msg = (typeof window.t === 'function')
            ? window.t('chatFilesPage.mkdirInvalidName')
            : '名称无效';
        alert(msg);
        return;
    }
    const parent = chatFilesBrowsePath.join('/');
    try {
        const res = await apiFetch('/api/chat-uploads/mkdir', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ parent: parent, name: name })
        });
        if (!res.ok) {
            let errText = '';
            try {
                const j = await res.json();
                errText = j.error || JSON.stringify(j);
            } catch (e2) {
                errText = await res.text();
            }
            if (res.status === 409) {
                const msg = (typeof window.t === 'function')
                    ? window.t('chatFilesPage.mkdirExists')
                    : errText;
                alert(msg);
                return;
            }
            throw new Error(errText || String(res.status));
        }
        closeChatFilesMkdirModal();
        loadChatFilesPage();
        const okMsg = (typeof window.t === 'function')
            ? window.t('chatFilesPage.mkdirOk')
            : '文件夹已创建';
        chatFilesShowToast(okMsg);
    } catch (e) {
        alert((e && e.message) ? e.message : String(e));
    }
}

function chatFilesSetUploadProgressUI(visible, percent, fileName) {
    const wrap = document.getElementById('chat-files-upload-progress');
    const fill = document.getElementById('chat-files-upload-progress-fill');
    const label = document.getElementById('chat-files-upload-progress-label');
    if (!wrap || !fill || !label) return;
    if (!visible) {
        wrap.hidden = true;
        fill.style.width = '0%';
        label.textContent = '';
        return;
    }
    wrap.hidden = false;
    const p = Math.min(100, Math.max(0, Math.round(percent)));
    fill.style.width = p + '%';
    const name = fileName || '';
    label.textContent = (typeof window.t === 'function')
        ? window.t('chatFilesPage.uploadingFile', { name: name, percent: p })
        : ('正在上传 ' + name + ' · ' + p + '%');
}

function chatFilesSetUploadBusy(busy) {
    chatFilesXHRUploadBusy = !!busy;
    ['chat-files-header-upload-btn', 'chat-files-refresh-btn'].forEach(function (id) {
        const el = document.getElementById(id);
        if (el) el.disabled = chatFilesXHRUploadBusy;
    });
}

function chatFilesOpenUploadPicker() {
    if (chatFilesXHRUploadBusy) return;
    if (chatFilesGetGroupByMode() === 'folder') {
        chatFilesPendingUploadDir = chatFilesBrowsePath.join('/');
    } else {
        chatFilesPendingUploadDir = '';
    }
    const inp = document.getElementById('chat-files-upload-input');
    if (inp) inp.click();
}

function chatFilesUploadToFolderClick(ev, btn) {
    if (ev) ev.stopPropagation();
    if (chatFilesXHRUploadBusy) return;
    const raw = btn.getAttribute('data-upload-dir');
    if (!raw) return;
    try {
        chatFilesPendingUploadDir = decodeURIComponent(raw);
    } catch (e) {
        chatFilesPendingUploadDir = '';
        return;
    }
    const inp = document.getElementById('chat-files-upload-input');
    if (inp) inp.click();
}

async function onChatFilesUploadPick(ev) {
    const input = ev.target;
    const file = input && input.files && input.files[0];
    if (!file) return;
    const form = new FormData();
    form.append('file', file);
    const pendingDir = chatFilesPendingUploadDir;
    chatFilesPendingUploadDir = '';
    if (pendingDir) {
        form.append('relativeDir', pendingDir);
    } else {
        const conv = document.getElementById('chat-files-filter-conv');
        if (conv && conv.value.trim()) {
            form.append('conversationId', conv.value.trim());
        }
    }
    chatFilesSetUploadBusy(true);
    chatFilesSetUploadProgressUI(true, 0, file.name);
    try {
        const doXhr = typeof apiUploadWithProgress === 'function';
        const res = doXhr
            ? await apiUploadWithProgress('/api/chat-uploads', form, {
                onProgress: function (p) {
                    chatFilesSetUploadProgressUI(true, p.percent, file.name);
                }
            })
            : await apiFetch('/api/chat-uploads', { method: 'POST', body: form });
        if (!res.ok) {
            throw new Error(await res.text());
        }
        const data = await res.json().catch(() => ({}));
        chatFilesSetUploadProgressUI(true, 100, file.name);
        loadChatFilesPage();
        if (data && data.ok) {
            const msg = (typeof window.t === 'function')
                ? window.t('chatFilesPage.uploadOkHint')
                : '上传成功。在列表中点击「复制路径」即可粘贴到对话中引用。';
            chatFilesShowToast(msg);
        }
    } catch (e) {
        alert((e && e.message) ? e.message : String(e));
    } finally {
        chatFilesSetUploadBusy(false);
        chatFilesSetUploadProgressUI(false);
        input.value = '';
    }
}

// 语言切换后重新渲染列表：表头与「更多」菜单由 JS 拼接，无 data-i18n，需用当前语言的 t() 再生成一遍
document.addEventListener('languagechange', function () {
    if (typeof window.currentPage !== 'function') return;
    if (window.currentPage() !== 'chat-files') return;
    if (typeof renderChatFilesTable === 'function') {
        renderChatFilesTable();
    }
});
