let currentConversationId = null;

// @ 
let mentionTools = [];
let mentionToolsLoaded = false;
let mentionToolsLoadingPromise = null;
let mentionSuggestionsEl = null;
let mentionFilteredTools = [];
let externalMcpNames = []; // MCP
const mentionState = {
    active: false,
    startIndex: -1,
    query: '',
    selectedIndex: 0,
};

// IME
let isComposing = false;

// 
const DRAFT_STORAGE_KEY = 'cyberstrike-chat-draft';
let draftSaveTimer = null;
const DRAFT_SAVE_DELAY = 500; // 500ms

// (,)
const MAX_CHAT_FILES = 10;
const CHAT_FILE_DEFAULT_PROMPT = '.';
/**
 * : POST /api/chat-uploads, serverPath(),.
 * @type {{ id: number, fileName: string, mimeType: string, serverPath: string|null, uploading: boolean, uploadPercent: number, uploadPromise: Promise<void>|null, uploadError: string|null }[]}
 */
let chatAttachments = [];
let chatAttachmentSeq = 0;

// (Eino): multi_agent.enabled, /agent-loop 
const AGENT_MODE_STORAGE_KEY = 'cyberstrike-chat-agent-mode';
let multiAgentAPIEnabled = false;

function getAgentModeLabelForValue(mode) {
    if (typeof window.t === 'function') {
        return mode === 'multi' ? window.t('chat.agentModeMulti') : window.t('chat.agentModeSingle');
    }
 return mode === 'multi' ? '' : '';
}

function getAgentModeIconForValue(mode) {
    return mode === 'multi' ? '🧩' : '🤖';
}

function syncAgentModeFromValue(value) {
    const hid = document.getElementById('agent-mode-select');
    const label = document.getElementById('agent-mode-text');
    const icon = document.getElementById('agent-mode-icon');
    if (hid) hid.value = value;
    if (label) label.textContent = getAgentModeLabelForValue(value);
    if (icon) icon.textContent = getAgentModeIconForValue(value);
    document.querySelectorAll('.agent-mode-option').forEach(function (el) {
        const v = el.getAttribute('data-value');
        el.classList.toggle('selected', v === value);
    });
}

function closeAgentModePanel() {
    const panel = document.getElementById('agent-mode-panel');
    const btn = document.getElementById('agent-mode-btn');
    if (panel) panel.style.display = 'none';
    if (btn) {
        btn.classList.remove('active');
        btn.setAttribute('aria-expanded', 'false');
    }
}

function toggleAgentModePanel() {
    const panel = document.getElementById('agent-mode-panel');
    const btn = document.getElementById('agent-mode-btn');
    if (!panel || !btn) return;
    const isOpen = panel.style.display === 'flex';
    if (isOpen) {
        closeAgentModePanel();
        return;
    }
    if (typeof closeRoleSelectionPanel === 'function') {
        closeRoleSelectionPanel();
    }
    panel.style.display = 'flex';
    btn.classList.add('active');
    btn.setAttribute('aria-expanded', 'true');
}

function selectAgentMode(mode) {
    if (mode !== 'single' && mode !== 'multi') return;
    try {
        localStorage.setItem(AGENT_MODE_STORAGE_KEY, mode);
    } catch (e) { /* ignore */ }
    syncAgentModeFromValue(mode);
    closeAgentModePanel();
}

async function initChatAgentModeFromConfig() {
    try {
        const r = await apiFetch('/api/config');
        if (!r.ok) return;
        const cfg = await r.json();
        multiAgentAPIEnabled = !!(cfg.multi_agent && cfg.multi_agent.enabled);
        if (typeof window !== 'undefined') {
            window.__csaiMultiAgentPublic = cfg.multi_agent || null;
        }
        const wrap = document.getElementById('agent-mode-wrapper');
        const sel = document.getElementById('agent-mode-select');
        if (!wrap || !sel) return;
        if (!multiAgentAPIEnabled) {
            wrap.style.display = 'none';
            return;
        }
        wrap.style.display = '';
        const def = (cfg.multi_agent && cfg.multi_agent.default_mode === 'multi') ? 'multi' : 'single';
        let stored = localStorage.getItem(AGENT_MODE_STORAGE_KEY);
        if (stored !== 'single' && stored !== 'multi') {
            stored = def;
        }
        sel.value = stored;
        syncAgentModeFromValue(stored);
    } catch (e) {
        console.warn('initChatAgentModeFromConfig', e);
    }
}

document.addEventListener('languagechange', function () {
    const hid = document.getElementById('agent-mode-select');
    if (!hid) return;
    const v = hid.value;
    if (v === 'single' || v === 'multi') {
        syncAgentModeFromValue(v);
    }
});

// localStorage()
function saveChatDraftDebounced(content) {
 // 
    if (draftSaveTimer) {
        clearTimeout(draftSaveTimer);
    }
    
 // 
    draftSaveTimer = setTimeout(() => {
        saveChatDraft(content);
    }, DRAFT_SAVE_DELAY);
}

// localStorage
function saveChatDraft(content) {
    try {
        const chatInput = document.getElementById('chat-input');
        const placeholderText = chatInput ? (chatInput.getAttribute('placeholder') || '').trim() : '';
        const trimmed = (content || '').trim();

 // 
        if (trimmed && (!placeholderText || trimmed !== placeholderText)) {
            localStorage.setItem(DRAFT_STORAGE_KEY, content);
        } else {
 // ,
            localStorage.removeItem(DRAFT_STORAGE_KEY);
        }
    } catch (error) {
 // localStorage,
 console.warn(':', error);
    }
}

// localStorage
function restoreChatDraft() {
    try {
        const chatInput = document.getElementById('chat-input');
        if (!chatInput) {
            return;
        }
        const placeholderText = (chatInput.getAttribute('placeholder') || '').trim();
 // value placeholder ,,
        if (placeholderText && chatInput.value.trim() === placeholderText) {
            chatInput.value = '';
        }
 // ,()
        if (chatInput.value && chatInput.value.trim().length > 0) {
            return;
        }
        
        const draft = localStorage.getItem(DRAFT_STORAGE_KEY);
        const trimmedDraft = draft ? draft.trim() : '';

 // ,,
        if (trimmedDraft && (!placeholderText || trimmedDraft !== placeholderText)) {
            chatInput.value = draft;
 // 
            adjustTextareaHeight(chatInput);
        } else if (trimmedDraft && placeholderText && trimmedDraft === placeholderText) {
 // ,
            localStorage.removeItem(DRAFT_STORAGE_KEY);
        }
    } catch (error) {
 console.warn(':', error);
    }
}

// 
function clearChatDraft() {
    try {
 // ,
        localStorage.removeItem(DRAFT_STORAGE_KEY);
    } catch (error) {
 console.warn(':', error);
    }
}

// textarea
function adjustTextareaHeight(textarea) {
    if (!textarea) return;
    
 // auto,,scrollHeight
    textarea.style.height = 'auto';
 // 
    void textarea.offsetHeight;
    
 // (40px,300px)
    const scrollHeight = textarea.scrollHeight;
    const newHeight = Math.min(Math.max(scrollHeight, 40), 300);
    textarea.style.height = newHeight + 'px';
    
 // ,
    if (!textarea.value || textarea.value.trim().length === 0) {
        textarea.style.height = '40px';
    }
}

// 
async function sendMessage() {
    const input = document.getElementById('chat-input');
    let message = input.value.trim();
    const hasAttachments = chatAttachments && chatAttachments.length > 0;

    if (!message && !hasAttachments) {
        return;
    }

    if (hasAttachments) {
        const needWait = chatAttachments.some((a) => a.uploading);
        if (needWait) {
            const waitLabel = (typeof window.t === 'function')
                ? window.t('chat.waitingAttachmentsUpload')
 : '...';
            chatAttachmentProgressSet(true, 0, waitLabel);
        }
        try {
            await Promise.all(chatAttachments.map((a) => (a.uploadPromise ? a.uploadPromise : Promise.resolve())));
        } finally {
            refreshChatAttachmentUploadProgress();
        }
        const bad = chatAttachments.filter((a) => !a.serverPath);
        if (bad.length) {
            const hint = (typeof window.t === 'function')
                ? window.t('chat.attachmentsUploadIncomplete')
 : ',.';
            alert(hint);
            return;
        }
    }

 // ,()
    if (hasAttachments && !message) {
        message = CHAT_FILE_DEFAULT_PROMPT;
    }

 // (,)
    const displayMessage = hasAttachments
        ? message + '\n' + chatAttachments.map(a => '📎 ' + a.fileName).join('\n')
        : message;
    addMessage('user', displayMessage);
    
 // ,
    if (draftSaveTimer) {
        clearTimeout(draftSaveTimer);
        draftSaveTimer = null;
    }
    
 // ,
    clearChatDraft();
 // 
    try {
        localStorage.removeItem(DRAFT_STORAGE_KEY);
    } catch (e) {
 // 
    }
    
 // ()
    input.value = '';
 // (40px)
    input.style.height = '40px';

 // ()
    const body = {
        message: message,
        conversationId: currentConversationId,
        role: typeof getCurrentRole === 'function' ? getCurrentRole() : ''
    };
    if (hasAttachments) {
        body.attachments = chatAttachments.map((a) => ({
            fileName: a.fileName,
            mimeType: a.mimeType || '',
            serverPath: a.serverPath
        }));
    }
 // 
    chatAttachments = [];
    renderChatFileChips();

 // ()
    const progressId = addProgressMessage();
    const progressElement = document.getElementById(progressId);
    registerProgressTask(progressId, currentConversationId);
    loadActiveTasks();
    let assistantMessageId = null;
    let mcpExecutionIds = [];
    
    try {
        const modeSel = document.getElementById('agent-mode-select');
        const useMulti = multiAgentAPIEnabled && modeSel && modeSel.value === 'multi';
        const streamPath = useMulti ? '/api/multi-agent/stream' : '/api/agent-loop/stream';
        const response = await apiFetch(streamPath, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(body),
        });
        
        if (!response.ok) {
 throw new Error(': ' + response.status);
        }
        
        const reader = response.body.getReader();
        const decoder = new TextDecoder();
        let buffer = '';
        
        while (true) {
            const { done, value } = await reader.read();
            if (done) break;
            
            buffer += decoder.decode(value, { stream: true });
            const lines = buffer.split('\n');
 buffer = lines.pop(); // 
            
            for (const line of lines) {
                if (line.startsWith('data: ')) {
                    try {
                        const eventData = JSON.parse(line.slice(6));
                        handleStreamEvent(eventData, progressElement, progressId, 
                                         () => assistantMessageId, (id) => { assistantMessageId = id; },
                                         () => mcpExecutionIds, (ids) => { mcpExecutionIds = ids; });
                    } catch (e) {
 console.error(':', e, line);
                    }
                }
            }
        }
        
 // buffer
        if (buffer.trim()) {
            const lines = buffer.split('\n');
            for (const line of lines) {
                if (line.startsWith('data: ')) {
                    try {
                        const eventData = JSON.parse(line.slice(6));
                        handleStreamEvent(eventData, progressElement, progressId,
                                         () => assistantMessageId, (id) => { assistantMessageId = id; },
                                         () => mcpExecutionIds, (ids) => { mcpExecutionIds = ids; });
                    } catch (e) {
 console.error(':', e, line);
                    }
                }
            }
        }
        
 // ,
        clearChatDraft();
        try {
            localStorage.removeItem(DRAFT_STORAGE_KEY);
        } catch (e) {
 // 
        }
        
    } catch (error) {
        removeMessage(progressId);
        const msg = error && error.message != null ? String(error.message) : String(error);
        const isNetwork = /network|fetch|Failed to fetch|aborted|AbortError|load failed|NetworkError/i.test(msg);
        if (isNetwork && typeof window.t === 'function') {
            addMessage('system', window.t('chat.streamNetworkErrorHint', { detail: msg }));
        } else if (isNetwork) {
 addMessage('system', '(' + msg + ').,.');
        } else {
 addMessage('system', ': ' + msg);
        }
        if (typeof loadActiveTasks === 'function') {
            loadActiveTasks();
        }
 // ,,
    }
}

// ---------- ----------
function renderChatFileChips() {
    const list = document.getElementById('chat-file-list');
    if (!list) return;
    list.innerHTML = '';
    if (!chatAttachments.length) return;
    chatAttachments.forEach((a, i) => {
        const chip = document.createElement('div');
        chip.className = 'chat-file-chip';
        if (a.uploading) chip.classList.add('chat-file-chip--uploading');
        if (a.uploadError) chip.classList.add('chat-file-chip--error');
        chip.setAttribute('role', 'listitem');
        const name = document.createElement('span');
        name.className = 'chat-file-chip-name';
        name.title = a.fileName;
        let label = a.fileName;
        if (a.uploading) {
 label += ' · ' + ((typeof window.t === 'function') ? window.t('chat.attachmentUploading') : '...');
        } else if (a.uploadError) {
 label += ' · ' + ((typeof window.t === 'function') ? window.t('chat.attachmentUploadFailed') : '');
        }
        name.textContent = label;
        const remove = document.createElement('button');
        remove.type = 'button';
        remove.className = 'chat-file-chip-remove';
 remove.title = typeof window.t === 'function' ? window.t('chatGroup.remove') : '';
        remove.innerHTML = '×';
 remove.setAttribute('aria-label', ' ' + a.fileName);
        remove.addEventListener('click', () => removeChatAttachment(i));
        chip.appendChild(name);
        chip.appendChild(remove);
        list.appendChild(chip);
    });
}

function removeChatAttachment(index) {
    chatAttachments.splice(index, 1);
    renderChatFileChips();
    refreshChatAttachmentUploadProgress();
}

// ,();
function appendChatFilePrompt() {
    const input = document.getElementById('chat-input');
    if (!input || !chatAttachments.length) return;
    if (!input.value.trim()) {
        input.value = CHAT_FILE_DEFAULT_PROMPT;
        adjustTextareaHeight(input);
    }
}

function chatAttachmentProgressSet(visible, percent, detailText) {
    const wrap = document.getElementById('chat-attachment-progress');
    const fill = document.getElementById('chat-attachment-progress-fill');
    const label = document.getElementById('chat-attachment-progress-label');
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
    label.textContent = detailText || '';
}

function refreshChatAttachmentUploadProgress() {
    if (!chatAttachments.length) {
        chatAttachmentProgressSet(false);
        return;
    }
    const uploading = chatAttachments.filter((a) => a.uploading);
    if (!uploading.length) {
        chatAttachmentProgressSet(false);
        return;
    }
    let sum = 0;
    chatAttachments.forEach((a) => {
        sum += a.uploading ? (a.uploadPercent || 0) : 100;
    });
    const overall = Math.round(sum / chatAttachments.length);
    const line = (typeof window.t === 'function')
        ? window.t('chat.uploadingAttachmentsDetail', {
            done: chatAttachments.length - uploading.length,
            total: chatAttachments.length,
            percent: overall
        })
 : (' ' + (chatAttachments.length - uploading.length) + '/' + chatAttachments.length + ' · ' + overall + '%');
    chatAttachmentProgressSet(true, overall, line);
}

async function uploadOneChatAttachment(entry, file) {
    const form = new FormData();
    form.append('file', file);
    const conv = currentConversationId;
    if (conv && String(conv).trim()) {
        form.append('conversationId', String(conv).trim());
    }
    const entryId = entry.id;
    try {
        const res = typeof apiUploadWithProgress === 'function'
            ? await apiUploadWithProgress('/api/chat-uploads', form, {
                onProgress: function (p) {
                    const cur = chatAttachments.find((x) => x.id === entryId);
                    if (cur) {
                        cur.uploadPercent = p.percent;
                        refreshChatAttachmentUploadProgress();
                    }
                }
            })
            : await apiFetch('/api/chat-uploads', { method: 'POST', body: form });
        if (!res.ok) {
            throw new Error(await res.text());
        }
        const data = await res.json().catch(() => ({}));
        const abs = data.absolutePath ? String(data.absolutePath).trim() : '';
        if (!abs) {
            throw new Error('no absolutePath in response');
        }
        const cur = chatAttachments.find((x) => x.id === entryId);
        if (cur) {
            cur.serverPath = abs;
            cur.uploading = false;
            cur.uploadPercent = 100;
            cur.uploadError = null;
        }
    } catch (e) {
        const msg = (e && e.message) ? e.message : String(e);
        const cur = chatAttachments.find((x) => x.id === entryId);
        if (cur) {
            cur.uploading = false;
            cur.uploadError = msg;
            cur.serverPath = null;
        }
 alert(((typeof window.t === 'function') ? window.t('chat.attachmentUploadAlert', { name: file.name }) : (':' + file.name)) + '\n' + msg);
    }
    renderChatFileChips();
    refreshChatAttachmentUploadProgress();
}

async function addFilesToChat(files) {
    if (!files || !files.length) return;
    const next = Array.from(files);
    if (chatAttachments.length + next.length > MAX_CHAT_FILES) {
 alert(' ' + MAX_CHAT_FILES + ' , ' + chatAttachments.length + ' .');
        return;
    }
    next.forEach((file) => {
        const id = ++chatAttachmentSeq;
        const entry = {
            id: id,
            fileName: file.name,
            mimeType: file.type || '',
            serverPath: null,
            uploading: true,
            uploadPercent: 0,
            uploadPromise: null,
            uploadError: null
        };
        entry.uploadPromise = uploadOneChatAttachment(entry, file);
        chatAttachments.push(entry);
    });
    renderChatFileChips();
    refreshChatAttachmentUploadProgress();
    appendChatFilePrompt();
}

function setupChatFileUpload() {
    const inputEl = document.getElementById('chat-file-input');
    const container = document.getElementById('chat-input-container') || document.querySelector('.chat-input-container');
    if (!inputEl || !container) return;

    inputEl.addEventListener('change', function () {
        const files = this.files;
        if (files && files.length) {
 addFilesToChat(files).catch(function () { /* addFilesToChat */ });
        }
        this.value = '';
    });

    container.addEventListener('dragover', function (e) {
        e.preventDefault();
        e.stopPropagation();
        this.classList.add('drag-over');
    });
    container.addEventListener('dragleave', function (e) {
        e.preventDefault();
        e.stopPropagation();
        if (!this.contains(e.relatedTarget)) {
            this.classList.remove('drag-over');
        }
    });
    container.addEventListener('drop', function (e) {
        e.preventDefault();
        e.stopPropagation();
        this.classList.remove('drag-over');
        const files = e.dataTransfer && e.dataTransfer.files;
 if (files && files.length) addFilesToChat(files).catch(function () { /* addFilesToChat */ });
    });
}

// chat-input-container id()
function ensureChatInputContainerId() {
    const c = document.querySelector('.chat-input-container');
    if (c && !c.id) c.id = 'chat-input-container';
}

function setupMentionSupport() {
    mentionSuggestionsEl = document.getElementById('mention-suggestions');
    if (mentionSuggestionsEl) {
        mentionSuggestionsEl.style.display = 'none';
        mentionSuggestionsEl.addEventListener('mousedown', (event) => {
 // 
            event.preventDefault();
        });
    }
    ensureMentionToolsLoaded().catch(() => {
 // ,
    });
}

// (,)
function refreshMentionTools() {
    mentionToolsLoaded = false;
    mentionTools = [];
    externalMcpNames = [];
    mentionToolsLoadingPromise = null;
 // @,
    if (mentionState.active) {
        ensureMentionToolsLoaded().catch(() => {
 // 
        });
    }
}

// window,
if (typeof window !== 'undefined') {
    window.refreshMentionTools = refreshMentionTools;
}

function ensureMentionToolsLoaded() {
 // ,
    if (typeof window !== 'undefined' && window._mentionToolsRoleChanged) {
        mentionToolsLoaded = false;
        mentionTools = [];
        delete window._mentionToolsRoleChanged;
    }
    
    if (mentionToolsLoaded) {
        return Promise.resolve(mentionTools);
    }
    if (mentionToolsLoadingPromise) {
        return mentionToolsLoadingPromise;
    }
    mentionToolsLoadingPromise = fetchMentionTools().finally(() => {
        mentionToolsLoadingPromise = null;
    });
    return mentionToolsLoadingPromise;
}

// ,
function getToolKeyForMention(tool) {
 // , external_mcp::tool.name 
 // , tool.name 
    if (tool.is_external && tool.external_mcp) {
        return `${tool.external_mcp}::${tool.name}`;
    }
    return tool.name;
}

async function fetchMentionTools() {
    const pageSize = 100;
    let page = 1;
    let totalPages = 1;
    const seen = new Set();
    const collected = [];

    try {
 // ( roles.js )
        const roleName = typeof getCurrentRole === 'function' ? getCurrentRole() : '';

 // MCP
        try {
            const mcpResponse = await apiFetch('/api/external-mcp');
            if (mcpResponse.ok) {
                const mcpData = await mcpResponse.json();
                externalMcpNames = Object.keys(mcpData.servers || {}).filter(name => {
                    const server = mcpData.servers[name];
 // MCP
                    return server.status === 'connected' && 
                           (server.config.external_mcp_enable || (server.config.enabled && !server.config.disabled));
                });
            }
        } catch (mcpError) {
 console.warn('MCP:', mcpError);
            externalMcpNames = [];
        }

        while (page <= totalPages && page <= 20) {
 // API URL,,role
            let url = `/api/config/tools?page=${page}&page_size=${pageSize}`;
 if (roleName && roleName !== '') {
                url += `&role=${encodeURIComponent(roleName)}`;
            }

            const response = await apiFetch(url);
            if (!response.ok) {
                break;
            }
            const result = await response.json();
            const tools = Array.isArray(result.tools) ? result.tools : [];
            tools.forEach(tool => {
                if (!tool || !tool.name) {
                    return;
                }
 // ,
                const toolKey = getToolKeyForMention(tool);
                if (seen.has(toolKey)) {
                    return;
                }
                seen.add(toolKey);

 // 
 // role_enabled ,()
 // enabled ()
                let roleEnabled = tool.enabled !== false;
                if (tool.role_enabled !== undefined && tool.role_enabled !== null) {
                    roleEnabled = tool.role_enabled;
                }

                collected.push({
                    name: tool.name,
                    description: tool.description || '',
 enabled: tool.enabled !== false, // 
 roleEnabled: roleEnabled, // 
                    isExternal: !!tool.is_external,
                    externalMcp: tool.external_mcp || '',
 toolKey: toolKey, // 
                });
            });
            totalPages = result.total_pages || 1;
            page += 1;
            if (page > totalPages) {
                break;
            }
        }
        mentionTools = collected;
        mentionToolsLoaded = true;
    } catch (error) {
 console.warn(',@:', error);
    }
    return mentionTools;
}

function handleChatInputInput(event) {
    const textarea = event.target;
    updateMentionStateFromInput(textarea);
 // 
 // requestAnimationFrameDOM,
    requestAnimationFrame(() => {
        adjustTextareaHeight(textarea);
    });
 // localStorage()
    saveChatDraftDebounced(textarea.value);
}

function handleChatInputClick(event) {
    updateMentionStateFromInput(event.target);
}

function handleChatInputKeydown(event) {
 // (IME),,
 // event.isComposing isComposing 
    if (event.isComposing || isComposing) {
        return;
    }

    if (mentionState.active && mentionSuggestionsEl && mentionSuggestionsEl.style.display !== 'none') {
        if (event.key === 'ArrowDown') {
            event.preventDefault();
            moveMentionSelection(1);
            return;
        }
        if (event.key === 'ArrowUp') {
            event.preventDefault();
            moveMentionSelection(-1);
            return;
        }
        if (event.key === 'Enter' || event.key === 'Tab') {
            event.preventDefault();
            applyMentionSelection();
            return;
        }
        if (event.key === 'Escape') {
            event.preventDefault();
            deactivateMentionState();
            return;
        }
    }

    if (event.key === 'Enter' && !event.shiftKey) {
        event.preventDefault();
        sendMessage();
    }
}

function updateMentionStateFromInput(textarea) {
    if (!textarea) {
        deactivateMentionState();
        return;
    }
    const caret = textarea.selectionStart || 0;
    const textBefore = textarea.value.slice(0, caret);
    const atIndex = textBefore.lastIndexOf('@');

    if (atIndex === -1) {
        deactivateMentionState();
        return;
    }

 // 
    if (atIndex > 0) {
        const boundaryChar = textBefore[atIndex - 1];
        if (boundaryChar && !/\s/.test(boundaryChar) && !'([{,.,.;:!?'.includes(boundaryChar)) {
            deactivateMentionState();
            return;
        }
    }

    const querySegment = textBefore.slice(atIndex + 1);

    if (querySegment.includes(' ') || querySegment.includes('\n') || querySegment.includes('\t') || querySegment.includes('@')) {
        deactivateMentionState();
        return;
    }

    if (querySegment.length > 60) {
        deactivateMentionState();
        return;
    }

    mentionState.active = true;
    mentionState.startIndex = atIndex;
    mentionState.query = querySegment.toLowerCase();
    mentionState.selectedIndex = 0;

    if (!mentionToolsLoaded) {
        renderMentionSuggestions({ showLoading: true });
    } else {
        updateMentionCandidates();
        renderMentionSuggestions();
    }

    ensureMentionToolsLoaded().then(() => {
        if (mentionState.active) {
            updateMentionCandidates();
            renderMentionSuggestions();
        }
    });
}

function updateMentionCandidates() {
    if (!mentionState.active) {
        mentionFilteredTools = [];
        return;
    }
    const normalizedQuery = (mentionState.query || '').trim().toLowerCase();
    let filtered = mentionTools;

    if (normalizedQuery) {
 // MCP
        const exactMatchedMcp = externalMcpNames.find(mcpName => 
            mcpName.toLowerCase() === normalizedQuery
        );

        if (exactMatchedMcp) {
 // MCP,MCP
            filtered = mentionTools.filter(tool => {
                return tool.externalMcp && tool.externalMcp.toLowerCase() === exactMatchedMcp.toLowerCase();
            });
        } else {
 // MCP
            const partialMatchedMcps = externalMcpNames.filter(mcpName => 
                mcpName.toLowerCase().includes(normalizedQuery)
            );
            
 // :,MCP
            filtered = mentionTools.filter(tool => {
                const nameMatch = tool.name.toLowerCase().includes(normalizedQuery);
                const descMatch = tool.description && tool.description.toLowerCase().includes(normalizedQuery);
                const mcpMatch = tool.externalMcp && tool.externalMcp.toLowerCase().includes(normalizedQuery);
                
 // MCP,MCP
                const mcpPartialMatch = partialMatchedMcps.some(mcpName => 
                    tool.externalMcp && tool.externalMcp.toLowerCase() === mcpName.toLowerCase()
                );
                
                return nameMatch || descMatch || mcpMatch || mcpPartialMatch;
            });
        }
    }

    filtered = filtered.slice().sort((a, b) => {
 // ,
        if (a.roleEnabled !== undefined || b.roleEnabled !== undefined) {
            const aRoleEnabled = a.roleEnabled !== undefined ? a.roleEnabled : a.enabled;
            const bRoleEnabled = b.roleEnabled !== undefined ? b.roleEnabled : b.enabled;
            if (aRoleEnabled !== bRoleEnabled) {
 return aRoleEnabled ? -1 : 1; // 
            }
        }

        if (normalizedQuery) {
 // MCP
            const aMcpExact = a.externalMcp && a.externalMcp.toLowerCase() === normalizedQuery;
            const bMcpExact = b.externalMcp && b.externalMcp.toLowerCase() === normalizedQuery;
            if (aMcpExact !== bMcpExact) {
                return aMcpExact ? -1 : 1;
            }
            
            const aStarts = a.name.toLowerCase().startsWith(normalizedQuery);
            const bStarts = b.name.toLowerCase().startsWith(normalizedQuery);
            if (aStarts !== bStarts) {
                return aStarts ? -1 : 1;
            }
        }
 // , roleEnabled; enabled
        const aEnabled = a.roleEnabled !== undefined ? a.roleEnabled : a.enabled;
        const bEnabled = b.roleEnabled !== undefined ? b.roleEnabled : b.enabled;
        if (aEnabled !== bEnabled) {
            return aEnabled ? -1 : 1;
        }
        return a.name.localeCompare(b.name, 'zh-CN');
    });

    mentionFilteredTools = filtered;
    if (mentionFilteredTools.length === 0) {
        mentionState.selectedIndex = 0;
    } else if (mentionState.selectedIndex >= mentionFilteredTools.length) {
        mentionState.selectedIndex = 0;
    }
}

function renderMentionSuggestions({ showLoading = false } = {}) {
    if (!mentionSuggestionsEl || !mentionState.active) {
        hideMentionSuggestions();
        return;
    }

    const currentQuery = mentionState.query || '';
    const existingList = mentionSuggestionsEl.querySelector('.mention-suggestions-list');
    const canPreserveScroll = !showLoading &&
        existingList &&
        mentionSuggestionsEl.dataset.lastMentionQuery === currentQuery;
    const previousScrollTop = canPreserveScroll ? existingList.scrollTop : 0;

    if (showLoading) {
 mentionSuggestionsEl.innerHTML = '<div class="mention-empty">' + (typeof window.t === 'function' ? window.t('chat.loadingTools') : '...') + '</div>';
        mentionSuggestionsEl.style.display = 'block';
        delete mentionSuggestionsEl.dataset.lastMentionQuery;
        return;
    }

    if (!mentionFilteredTools.length) {
 mentionSuggestionsEl.innerHTML = '<div class="mention-empty">' + (typeof window.t === 'function' ? window.t('chat.noMatchTools') : '') + '</div>';
        mentionSuggestionsEl.style.display = 'block';
        mentionSuggestionsEl.dataset.lastMentionQuery = currentQuery;
        return;
    }

    const itemsHtml = mentionFilteredTools.map((tool, index) => {
        const activeClass = index === mentionState.selectedIndex ? 'active' : '';
 // roleEnabled (),; enabled
        const toolEnabled = tool.roleEnabled !== undefined ? tool.roleEnabled : tool.enabled;
        const disabledClass = toolEnabled ? '' : 'disabled';
 const badge = tool.isExternal ? '<span class="mention-item-badge"></span>' : '<span class="mention-item-badge internal"></span>';
        const nameHtml = escapeHtml(tool.name);
 const description = tool.description && tool.description.length > 0 ? escapeHtml(tool.description) : (typeof window.t === 'function' ? window.t('chat.noDescription') : '');
        const descHtml = `<div class="mention-item-desc">${description}</div>`;
 // 
 const statusLabel = toolEnabled ? '' : (tool.roleEnabled !== undefined ? '()' : '');
        const statusClass = toolEnabled ? 'enabled' : 'disabled';
        const originLabel = tool.isExternal
 ? (tool.externalMcp ? `:${escapeHtml(tool.externalMcp)}` : ':MCP')
 : ':';

        return `
            <button type="button" class="mention-item ${activeClass} ${disabledClass}" data-index="${index}">
                <div class="mention-item-name">
                    <span class="mention-item-icon">🔧</span>
                    <span class="mention-item-text">@${nameHtml}</span>
                    ${badge}
                </div>
                ${descHtml}
                <div class="mention-item-meta">
                    <span class="mention-status ${statusClass}">${statusLabel}</span>
                    <span class="mention-origin">${originLabel}</span>
                </div>
            </button>
        `;
    }).join('');

    const listWrapper = document.createElement('div');
    listWrapper.className = 'mention-suggestions-list';
    listWrapper.innerHTML = itemsHtml;

    mentionSuggestionsEl.innerHTML = '';
    mentionSuggestionsEl.appendChild(listWrapper);
    mentionSuggestionsEl.style.display = 'block';
    mentionSuggestionsEl.dataset.lastMentionQuery = currentQuery;

    if (canPreserveScroll) {
        listWrapper.scrollTop = previousScrollTop;
    }

    listWrapper.querySelectorAll('.mention-item').forEach(item => {
        item.addEventListener('mousedown', (event) => {
            event.preventDefault();
            const idx = parseInt(item.dataset.index, 10);
            if (!Number.isNaN(idx)) {
                mentionState.selectedIndex = idx;
            }
            applyMentionSelection();
        });
    });

    scrollMentionSelectionIntoView();
}

function hideMentionSuggestions() {
    if (mentionSuggestionsEl) {
        mentionSuggestionsEl.style.display = 'none';
        mentionSuggestionsEl.innerHTML = '';
        delete mentionSuggestionsEl.dataset.lastMentionQuery;
    }
}

function deactivateMentionState() {
    mentionState.active = false;
    mentionState.startIndex = -1;
    mentionState.query = '';
    mentionState.selectedIndex = 0;
    mentionFilteredTools = [];
    hideMentionSuggestions();
}

function moveMentionSelection(direction) {
    if (!mentionFilteredTools.length) {
        return;
    }
    const max = mentionFilteredTools.length - 1;
    let nextIndex = mentionState.selectedIndex + direction;
    if (nextIndex < 0) {
        nextIndex = max;
    } else if (nextIndex > max) {
        nextIndex = 0;
    }
    mentionState.selectedIndex = nextIndex;
    updateMentionActiveHighlight();
}

function updateMentionActiveHighlight() {
    if (!mentionSuggestionsEl) {
        return;
    }
    const items = mentionSuggestionsEl.querySelectorAll('.mention-item');
    if (!items.length) {
        return;
    }
    items.forEach(item => item.classList.remove('active'));

    let targetIndex = mentionState.selectedIndex;
    if (targetIndex < 0) {
        targetIndex = 0;
    }
    if (targetIndex >= items.length) {
        targetIndex = items.length - 1;
        mentionState.selectedIndex = targetIndex;
    }

    const activeItem = items[targetIndex];
    if (activeItem) {
        activeItem.classList.add('active');
        scrollMentionSelectionIntoView(activeItem);
    }
}

function scrollMentionSelectionIntoView(targetItem = null) {
    if (!mentionSuggestionsEl) {
        return;
    }
    const activeItem = targetItem || mentionSuggestionsEl.querySelector('.mention-item.active');
    if (activeItem && typeof activeItem.scrollIntoView === 'function') {
        activeItem.scrollIntoView({
            block: 'nearest',
            inline: 'nearest',
            behavior: 'auto'
        });
    }
}

function applyMentionSelection() {
    const textarea = document.getElementById('chat-input');
    if (!textarea || mentionState.startIndex === -1 || !mentionFilteredTools.length) {
        deactivateMentionState();
        return;
    }

    const selectedTool = mentionFilteredTools[mentionState.selectedIndex] || mentionFilteredTools[0];
    if (!selectedTool) {
        deactivateMentionState();
        return;
    }

    const caret = textarea.selectionStart || 0;
    const before = textarea.value.slice(0, mentionState.startIndex);
    const after = textarea.value.slice(caret);
    const mentionText = `@${selectedTool.name}`;
    const needsSpace = after.length === 0 || !/^\s/.test(after);
    const insertText = mentionText + (needsSpace ? ' ' : '');

    textarea.value = before + insertText + after;
    const newCaret = before.length + insertText.length;
    textarea.focus();
    textarea.setSelectionRange(newCaret, newCaret);
    
 // 
    adjustTextareaHeight(textarea);
    saveChatDraftDebounced(textarea.value);

    deactivateMentionState();
}

function initializeChatUI() {
    const chatInputEl = document.getElementById('chat-input');
    if (chatInputEl) {
 // 
        adjustTextareaHeight(chatInputEl);
 // (,)
        if (!chatInputEl.value || chatInputEl.value.trim() === '') {
 // (30),,,
            const messagesDiv = document.getElementById('chat-messages');
            let shouldRestoreDraft = true;
            if (messagesDiv && messagesDiv.children.length > 0) {
 // 
                const lastMessage = messagesDiv.lastElementChild;
                if (lastMessage) {
                    const timeDiv = lastMessage.querySelector('.message-time');
                    if (timeDiv && timeDiv.textContent) {
 // ,,
                        const isUserMessage = lastMessage.classList.contains('user');
                        if (isUserMessage) {
 // ,30,
                            const now = new Date();
                            const messageTimeText = timeDiv.textContent;
 // :(:HH:MM),,
 // ,
 // :,,,
                            shouldRestoreDraft = false;
                        }
                    }
                }
            }
            if (shouldRestoreDraft) {
                restoreChatDraft();
            } else {
 // ,localStorage,
                clearChatDraft();
            }
        }
    }

    const messagesDiv = document.getElementById('chat-messages');
    if (messagesDiv && messagesDiv.childElementCount === 0) {
 const readyMsg = typeof window.t === 'function' ? window.t('chat.systemReadyMessage') : '.,.';
        addMessage('assistant', readyMsg, null, null, null, { systemReadyMessage: true });
    }

    addAttackChainButton(currentConversationId);
    loadActiveTasks(true);
    if (activeTaskInterval) {
        clearInterval(activeTaskInterval);
    }
    activeTaskInterval = setInterval(() => loadActiveTasks(), ACTIVE_TASK_REFRESH_INTERVAL);
    setupMentionSupport();
    ensureChatInputContainerId();
    setupChatFileUpload();
}

// ,ID
let messageCounter = 0;

// 
function wrapTablesInBubble(bubble) {
    const tables = bubble.querySelectorAll('table');
    tables.forEach(table => {
 // 
        if (table.parentElement && table.parentElement.classList.contains('table-wrapper')) {
            return;
        }
        
 // 
        const wrapper = document.createElement('div');
        wrapper.className = 'table-wrapper';
        
 // 
        table.parentNode.insertBefore(wrapper, table);
        wrapper.appendChild(table);
    });
}

/**
 * ""( addMessage )
 */
function refreshSystemReadyMessageBubbles() {
    if (typeof window.t !== 'function') return;
    const text = window.t('chat.systemReadyMessage');
    const escapeHtmlLocal = (s) => {
        if (!s) return '';
        const div = document.createElement('div');
        div.textContent = s;
        return div.innerHTML;
    };
    const defaultSanitizeConfig = {
        ALLOWED_TAGS: ['p', 'br', 'strong', 'em', 'u', 's', 'code', 'pre', 'blockquote', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'ul', 'ol', 'li', 'a', 'img', 'table', 'thead', 'tbody', 'tr', 'th', 'td', 'hr'],
        ALLOWED_ATTR: ['href', 'title', 'alt', 'src', 'class'],
        ALLOW_DATA_ATTR: false,
    };
    let formattedContent;
    if (typeof marked !== 'undefined') {
        try {
            marked.setOptions({ breaks: true, gfm: true });
            const parsed = marked.parse(text);
            formattedContent = typeof DOMPurify !== 'undefined'
                ? DOMPurify.sanitize(parsed, defaultSanitizeConfig)
                : parsed;
        } catch (e) {
            formattedContent = escapeHtmlLocal(text).replace(/\n/g, '<br>');
        }
    } else {
        formattedContent = escapeHtmlLocal(text).replace(/\n/g, '<br>');
    }

    document.querySelectorAll('.message.assistant[data-system-ready-message]').forEach(function (messageDiv) {
        const bubble = messageDiv.querySelector('.message-bubble');
        if (!bubble) return;
        const copyBtn = bubble.querySelector('.message-copy-btn');
        if (copyBtn) copyBtn.remove();
        bubble.innerHTML = formattedContent;
        if (typeof wrapTablesInBubble === 'function') wrapTablesInBubble(bubble);
        messageDiv.dataset.originalContent = text;
        const copyBtnNew = document.createElement('button');
        copyBtnNew.className = 'message-copy-btn';
        copyBtnNew.innerHTML = '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg"><rect x="9" y="9" width="13" height="13" rx="2" ry="2" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" fill="none"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" fill="none"/></svg><span>' + window.t('common.copy') + '</span>';
        copyBtnNew.title = window.t('chat.copyMessageTitle');
        copyBtnNew.onclick = function (e) {
            e.stopPropagation();
            copyMessageToClipboard(messageDiv, this);
        };
        bubble.appendChild(copyBtnNew);
    });
}

// (options.systemReadyMessage true ,)
function addMessage(role, content, mcpExecutionIds = null, progressId = null, createdAt = null, options = null) {
    const messagesDiv = document.getElementById('chat-messages');
    const messageDiv = document.createElement('div');
    messageCounter++;
    const id = 'msg-' + Date.now() + '-' + messageCounter + '-' + Math.random().toString(36).substr(2, 9);
    messageDiv.id = id;
    messageDiv.className = 'message ' + role;
    
 // 
    const avatar = document.createElement('div');
    avatar.className = 'message-avatar';
    if (role === 'user') {
        avatar.textContent = 'U';
    } else if (role === 'assistant') {
        avatar.textContent = 'A';
    } else {
        avatar.textContent = 'S';
    }
    messageDiv.appendChild(avatar);
    
 // 
    const contentWrapper = document.createElement('div');
    contentWrapper.className = 'message-content';
    
 // 
    const bubble = document.createElement('div');
    bubble.className = 'message-bubble';
    
 // Markdown HTML 
    let formattedContent;
    const defaultSanitizeConfig = {
        ALLOWED_TAGS: ['p', 'br', 'strong', 'em', 'u', 's', 'code', 'pre', 'blockquote', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'ul', 'ol', 'li', 'a', 'img', 'table', 'thead', 'tbody', 'tr', 'th', 'td', 'hr'],
        ALLOWED_ATTR: ['href', 'title', 'alt', 'src', 'class'],
        ALLOW_DATA_ATTR: false,
    };
    
 // HTML
    const escapeHtml = (text) => {
        if (!text) return '';
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    };
    
 // :,:
 // 1. Markdown,<code><pre>
 // 2. <code><pre>HTML()
 // 3. DOMPurify
 // XSS,
    
    const parseMarkdown = (raw) => {
        if (typeof marked === 'undefined') {
            return null;
        }
        try {
            marked.setOptions({
                breaks: true,
                gfm: true,
            });
            return marked.parse(raw);
        } catch (e) {
 console.error('Markdown :', e);
            return null;
        }
    };
    
 // ()
    let displayContent = content;
    if (role === 'assistant' && typeof displayContent === 'string' && typeof window.t === 'function') {
 if (displayContent.indexOf(': ') === 0) {
 displayContent = window.t('chat.executeFailed') + ': ' + displayContent.slice(': '.length);
        }
 if (displayContent.indexOf('OpenAI:') !== -1) {
 displayContent = displayContent.replace(/OpenAI:/g, window.t('chat.callOpenAIFailed') + ':');
        }
    }

 // ,HTML,Markdown,
    if (role === 'user') {
        formattedContent = escapeHtml(content).replace(/\n/g, '<br>');
    } else if (typeof DOMPurify !== 'undefined') {
 // Markdown(<code>/<pre>,DOMPurify)
        let parsedContent = parseMarkdown(role === 'assistant' ? displayContent : content);
        if (!parsedContent) {
            parsedContent = content;
        }
        
 // DOMPurify,URL(DOMPurify)
        if (DOMPurify.addHook) {
 // 
            try {
                DOMPurify.removeHook('uponSanitizeAttribute');
            } catch (e) {
 // ,
            }
            
 // URL,(DOMPurify,style)
            DOMPurify.addHook('uponSanitizeAttribute', (node, data) => {
                const attrName = data.attrName.toLowerCase();
                
 // URL(src, href)
                if ((attrName === 'src' || attrName === 'href') && data.attrValue) {
                    const value = data.attrValue.trim().toLowerCase();
 // 
                    if (value.startsWith('javascript:') || 
                        value.startsWith('vbscript:') ||
                        value.startsWith('data:text/html') ||
                        value.startsWith('data:text/javascript')) {
                        data.keepAttr = false;
                        return;
                    }
 // imgsrc,URL(404XSS)
                    if (attrName === 'src' && node.tagName && node.tagName.toLowerCase() === 'img') {
                        if (value.length <= 2 || /^[a-z]$/i.test(value)) {
                            data.keepAttr = false;
                            return;
                        }
                    }
                }
            });
        }
        
        formattedContent = DOMPurify.sanitize(parsedContent, defaultSanitizeConfig);
    } else if (typeof marked !== 'undefined') {
        const rawForParse = role === 'assistant' ? displayContent : content;
        const parsedContent = parseMarkdown(rawForParse);
        if (parsedContent) {
            formattedContent = parsedContent;
        } else {
            formattedContent = escapeHtml(rawForParse).replace(/\n/g, '<br>');
        }
    } else {
        const rawForEscape = role === 'assistant' ? displayContent : content;
        formattedContent = escapeHtml(rawForEscape).replace(/\n/g, '<br>');
    }
    
    bubble.innerHTML = formattedContent;
    
 // :(404XSS)
 // DOMPurifyXSS,
    const images = bubble.querySelectorAll('img');
    images.forEach(img => {
        const src = img.getAttribute('src');
        if (src) {
            const trimmedSrc = src.trim();
 // URL(,)
            if (trimmedSrc.length <= 2 || /^[a-z]$/i.test(trimmedSrc)) {
                img.remove();
            }
        } else {
            img.remove();
        }
    });
    
 // 
    wrapTablesInBubble(bubble);
    
    contentWrapper.appendChild(bubble);
    
 // ,
    if (role === 'assistant') {
        messageDiv.dataset.originalContent = content;
    }
    
 // ()- 
    if (role === 'assistant') {
        const copyBtn = document.createElement('button');
        copyBtn.className = 'message-copy-btn';
 copyBtn.innerHTML = '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg"><rect x="9" y="9" width="13" height="13" rx="2" ry="2" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" fill="none"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" fill="none"/></svg><span>' + (typeof window.t === 'function' ? window.t('common.copy') : '') + '</span>';
 copyBtn.title = typeof window.t === 'function' ? window.t('chat.copyMessageTitle') : '';
        copyBtn.onclick = function(e) {
            e.stopPropagation();
            copyMessageToClipboard(messageDiv, this);
        };
        bubble.appendChild(copyBtn);
    }
    
 // 
    const timeDiv = document.createElement('div');
    timeDiv.className = 'message-time';
 // ,;
    let messageTime;
    if (createdAt) {
 // Date
        if (typeof createdAt === 'string') {
            messageTime = new Date(createdAt);
        } else if (createdAt instanceof Date) {
            messageTime = createdAt;
        } else {
            messageTime = new Date(createdAt);
        }
 // ,
        if (isNaN(messageTime.getTime())) {
            messageTime = new Date();
        }
    } else {
        messageTime = new Date();
    }
    const msgTimeLocale = (typeof window.__locale === 'string' && window.__locale.startsWith('zh')) ? 'zh-CN' : 'en-US';
    const msgTimeOpts = { hour: '2-digit', minute: '2-digit' };
    if (msgTimeLocale === 'zh-CN') msgTimeOpts.hour12 = false;
    timeDiv.textContent = messageTime.toLocaleTimeString(msgTimeLocale, msgTimeOpts);
    try {
        timeDiv.dataset.messageTime = messageTime.toISOString();
    } catch (e) { /* ignore */ }
    contentWrapper.appendChild(timeDiv);
    
 // MCP ; progressId (, integrate )
    if (role === 'assistant' && (mcpExecutionIds && Array.isArray(mcpExecutionIds) && mcpExecutionIds.length > 0) && !progressId) {
        const mcpSection = document.createElement('div');
        mcpSection.className = 'mcp-call-section';
        
        const mcpLabel = document.createElement('div');
        mcpLabel.className = 'mcp-call-label';
 mcpLabel.textContent = '📋 ' + (typeof window.t === 'function' ? window.t('chat.penetrationTestDetail') : '');
        mcpSection.appendChild(mcpLabel);
        
        const buttonsContainer = document.createElement('div');
        buttonsContainer.className = 'mcp-call-buttons';
        
        mcpExecutionIds.forEach((execId, index) => {
            const detailBtn = document.createElement('button');
            detailBtn.className = 'mcp-detail-btn';
 detailBtn.innerHTML = '<span>' + (typeof window.t === 'function' ? window.t('chat.callNumber', { n: index + 1 }) : ' #' + (index + 1)) + '</span>';
            detailBtn.onclick = () => showMCPDetail(execId);
            buttonsContainer.appendChild(detailBtn);
            updateButtonWithToolName(detailBtn, execId, index + 1);
        });
        
        mcpSection.appendChild(buttonsContainer);
        contentWrapper.appendChild(mcpSection);
    }
    
    messageDiv.appendChild(contentWrapper);
 // "",
    if (options && options.systemReadyMessage) {
        messageDiv.setAttribute('data-system-ready-message', '1');
    }
    messagesDiv.appendChild(messageDiv);
    messagesDiv.scrollTop = messagesDiv.scrollHeight;
    return id;
}

// (Markdown)
function copyMessageToClipboard(messageDiv, button) {
    try {
 // Markdown
        const originalContent = messageDiv.dataset.originalContent;

 // 
        const doCopy = (text) => {
 // Clipboard API( HTTPS localhost)
            if (navigator.clipboard && navigator.clipboard.writeText) {
                return navigator.clipboard.writeText(text).then(() => {
                    showCopySuccess(button);
                }).catch(err => {
 console.error('Clipboard API :', err);
                    fallbackCopy(text);
                });
            } else {
 // : execCommand ( HTTP )
                return fallbackCopy(text);
            }
        };

 // ( document.execCommand)
        const fallbackCopy = (text) => {
            try {
                const textArea = document.createElement('textarea');
                textArea.value = text;
                textArea.style.position = 'fixed';
                textArea.style.left = '-999999px';
                textArea.style.top = '-999999px';
                textArea.style.opacity = '0';
                document.body.appendChild(textArea);
                textArea.focus();
                textArea.select();

                const successful = document.execCommand('copy');
                document.body.removeChild(textArea);

                if (successful) {
                    showCopySuccess(button);
                } else {
                    throw new Error('execCommand copy failed');
                }
            } catch (execErr) {
 console.error(':', execErr);
 alert(typeof window.t === 'function' ? window.t('chat.copyFailedManual') : ',');
            }
        };

        if (!originalContent) {
 // ,HTML()
            const bubble = messageDiv.querySelector('.message-bubble');
            if (bubble) {
                const tempDiv = document.createElement('div');
                tempDiv.innerHTML = bubble.innerHTML;
                
 // ()
                const copyBtnInTemp = tempDiv.querySelector('.message-copy-btn');
                if (copyBtnInTemp) {
                    copyBtnInTemp.remove();
                }
                
 // 
                let textContent = tempDiv.textContent || tempDiv.innerText || '';
                textContent = textContent.replace(/\n{3,}/g, '\n\n').trim();

                doCopy(textContent);
            }
            return;
        }
        
 // Markdown
        doCopy(originalContent);
    } catch (error) {
 console.error(':', error);
 alert(typeof window.t === 'function' ? window.t('chat.copyFailedManual') : ',');
    }
}

// 
function showCopySuccess(button) {
    if (button) {
        const originalText = button.innerHTML;
 button.innerHTML = '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg"><path d="M20 6L9 17l-5-5" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" fill="none"/></svg><span>' + (typeof window.t === 'function' ? window.t('common.copied') : '') + '</span>';
        button.style.color = '#10b981';
        button.style.background = 'rgba(16, 185, 129, 0.1)';
        button.style.borderColor = 'rgba(16, 185, 129, 0.3)';
        setTimeout(() => {
            button.innerHTML = originalText;
            button.style.color = '';
            button.style.background = '';
            button.style.borderColor = '';
        }, 2000);
    }
}

// 
function renderProcessDetails(messageId, processDetails) {
    const messageElement = document.getElementById(messageId);
    if (!messageElement) {
        return;
    }
    
 // MCP
    let mcpSection = messageElement.querySelector('.mcp-call-section');
    if (!mcpSection) {
        mcpSection = document.createElement('div');
        mcpSection.className = 'mcp-call-section';
        
        const contentWrapper = messageElement.querySelector('.message-content');
        if (contentWrapper) {
            contentWrapper.appendChild(mcpSection);
        } else {
            return;
        }
    }
    
 // ()
    let mcpLabel = mcpSection.querySelector('.mcp-call-label');
    let buttonsContainer = mcpSection.querySelector('.mcp-call-buttons');
    
 // ,()
    if (!mcpLabel && !buttonsContainer) {
        mcpLabel = document.createElement('div');
        mcpLabel.className = 'mcp-call-label';
 mcpLabel.textContent = '📋 ' + (typeof window.t === 'function' ? window.t('chat.penetrationTestDetail') : '');
        mcpSection.appendChild(mcpLabel);
 } else if (mcpLabel && mcpLabel.textContent !== ('📋 ' + (typeof window.t === 'function' ? window.t('chat.penetrationTestDetail') : ''))) {
 // ,
 mcpLabel.textContent = '📋 ' + (typeof window.t === 'function' ? window.t('chat.penetrationTestDetail') : '');
    }
    
 // ,
    if (!buttonsContainer) {
        buttonsContainer = document.createElement('div');
        buttonsContainer.className = 'mcp-call-buttons';
        mcpSection.appendChild(buttonsContainer);
    }
    
 // ()
    let processDetailBtn = buttonsContainer.querySelector('.process-detail-btn');
    if (!processDetailBtn) {
        processDetailBtn = document.createElement('button');
        processDetailBtn.className = 'mcp-detail-btn process-detail-btn';
 processDetailBtn.innerHTML = '<span>' + (typeof window.t === 'function' ? window.t('chat.expandDetail') : '') + '</span>';
        processDetailBtn.onclick = () => toggleProcessDetails(null, messageId);
        buttonsContainer.appendChild(processDetailBtn);
    }
    
 // ()
    const detailsId = 'process-details-' + messageId;
    let detailsContainer = document.getElementById(detailsId);
    
    if (!detailsContainer) {
        detailsContainer = document.createElement('div');
        detailsContainer.id = detailsId;
        detailsContainer.className = 'process-details-container';
 // 
        if (buttonsContainer.nextSibling) {
            mcpSection.insertBefore(detailsContainer, buttonsContainer.nextSibling);
        } else {
            mcpSection.appendChild(detailsContainer);
        }
    }
    
 // (processDetails,)
    const timelineId = detailsId + '-timeline';
    let timeline = document.getElementById(timelineId);
    
    if (!timeline) {
        const contentDiv = document.createElement('div');
        contentDiv.className = 'process-details-content';
        
        timeline = document.createElement('div');
        timeline.id = timelineId;
        timeline.className = 'progress-timeline';
        
        contentDiv.appendChild(timeline);
        detailsContainer.appendChild(contentDiv);
    }
    
 // processDetails === null "()"
    const isLazyNotLoaded = (processDetails === null);
    if (isLazyNotLoaded) {
        detailsContainer.dataset.lazyNotLoaded = '1';
        detailsContainer.dataset.loaded = '0';
        timeline.innerHTML = '<div class="progress-timeline-empty">' +
 (typeof window.t === 'function' ? window.t('chat.expandDetail') : '') +
 '()</div>';
 // 
        timeline.classList.remove('expanded');
        return;
    }
    detailsContainer.dataset.lazyNotLoaded = '0';
    detailsContainer.dataset.loaded = '1';
 // processDetails,
    if (!processDetails || processDetails.length === 0) {
 // 
 timeline.innerHTML = '<div class="progress-timeline-empty">' + (typeof window.t === 'function' ? window.t('chat.noProcessDetail') : '()') + '</div>';
 // 
        timeline.classList.remove('expanded');
        return;
    }
    
 // 
    timeline.innerHTML = '';
    
    
    function processDetailAgentPrefix(d) {
        if (!d || d.einoAgent == null) return '';
        const s = String(d.einoAgent).trim();
        return s ? ('[' + s + '] ') : '';
    }

 // 
    processDetails.forEach(detail => {
        const eventType = detail.eventType || '';
        const title = detail.message || '';
        const data = detail.data || {};
        const agPx = processDetailAgentPrefix(data);
        
 // 
        let itemTitle = title;
        if (eventType === 'iteration') {
 itemTitle = agPx + (typeof window.t === 'function' ? window.t('chat.iterationRound', { n: data.iteration || 1 }) : ' ' + (data.iteration || 1) + ' ');
        } else if (eventType === 'thinking') {
 itemTitle = agPx + '🤔 ' + (typeof window.t === 'function' ? window.t('chat.aiThinking') : 'AI');
        } else if (eventType === 'tool_calls_detected') {
 itemTitle = agPx + '🔧 ' + (typeof window.t === 'function' ? window.t('chat.toolCallsDetected', { count: data.count || 0 }) : ' ' + (data.count || 0) + ' ');
        } else if (eventType === 'tool_call') {
 const toolName = data.toolName || (typeof window.t === 'function' ? window.t('chat.unknownTool') : '');
            const index = data.index || 0;
            const total = data.total || 0;
 itemTitle = agPx + '🔧 ' + (typeof window.t === 'function' ? window.t('chat.callTool', { name: escapeHtml(toolName), index: index, total: total }) : ': ' + escapeHtml(toolName) + ' (' + index + '/' + total + ')');
        } else if (eventType === 'tool_result') {
 const toolName = data.toolName || (typeof window.t === 'function' ? window.t('chat.unknownTool') : '');
            const success = data.success !== false;
            const statusIcon = success ? '✅' : '❌';
 const execText = success ? (typeof window.t === 'function' ? window.t('chat.toolExecComplete', { name: escapeHtml(toolName) }) : ' ' + escapeHtml(toolName) + ' ') : (typeof window.t === 'function' ? window.t('chat.toolExecFailed', { name: escapeHtml(toolName) }) : ' ' + escapeHtml(toolName) + ' ');
            let execLine = statusIcon + ' ' + execText;
            if (toolName === BuiltinTools.SEARCH_KNOWLEDGE_BASE && success) {
 execLine = '📚 ' + execLine + ' - ' + (typeof window.t === 'function' ? window.t('chat.knowledgeRetrievalTag') : '');
            }
            itemTitle = agPx + execLine;
        } else if (eventType === 'eino_agent_reply') {
 itemTitle = agPx + '💬 ' + (typeof window.t === 'function' ? window.t('chat.einoAgentReplyTitle') : '');
        } else if (eventType === 'knowledge_retrieval') {
 itemTitle = '📚 ' + (typeof window.t === 'function' ? window.t('chat.knowledgeRetrieval') : '');
        } else if (eventType === 'error') {
 itemTitle = '❌ ' + (typeof window.t === 'function' ? window.t('chat.error') : '');
        } else if (eventType === 'cancelled') {
 itemTitle = '⛔ ' + (typeof window.t === 'function' ? window.t('chat.taskCancelled') : '');
        } else if (eventType === 'progress') {
            itemTitle = typeof window.translateProgressMessage === 'function' ? window.translateProgressMessage(detail.message || '') : (detail.message || '');
        }
        
        addTimelineItem(timeline, eventType, {
            title: itemTitle,
            message: detail.message || '',
            data: data,
 createdAt: detail.createdAt // 
        });
    });
    
 // ,,
    const hasErrorOrCancelled = processDetails.some(d => 
        d.eventType === 'error' || d.eventType === 'cancelled'
    );
    if (hasErrorOrCancelled) {
 // 
        timeline.classList.remove('expanded');
 // ""
        const processDetailBtn = messageElement.querySelector('.process-detail-btn');
        if (processDetailBtn) {
 processDetailBtn.innerHTML = '<span>' + (typeof window.t === 'function' ? window.t('chat.expandDetail') : '') + '</span>';
        }
    }
}

// 
function removeMessage(id) {
    const messageDiv = document.getElementById(id);
    if (messageDiv) {
        messageDiv.remove();
    }
}

// ( / @)
const chatInput = document.getElementById('chat-input');
if (chatInput) {
    chatInput.addEventListener('keydown', handleChatInputKeydown);
    chatInput.addEventListener('input', handleChatInputInput);
    chatInput.addEventListener('click', handleChatInputClick);
    chatInput.addEventListener('focus', handleChatInputClick);
 // IME,
    chatInput.addEventListener('compositionstart', () => {
        isComposing = true;
    });
    chatInput.addEventListener('compositionend', () => {
        isComposing = false;
    });
    chatInput.addEventListener('blur', () => {
        setTimeout(() => {
            if (!chatInput.matches(':focus')) {
                deactivateMentionState();
            }
        }, 120);
 // ()
        if (chatInput.value) {
            saveChatDraft(chatInput.value);
        }
    });
}

// 
window.addEventListener('beforeunload', () => {
    const chatInput = document.getElementById('chat-input');
    if (chatInput && chatInput.value) {
 // ,
        saveChatDraft(chatInput.value);
    }
});

// 
async function updateButtonWithToolName(button, executionId, index) {
    try {
        const response = await apiFetch(`/api/monitor/execution/${executionId}`);
        if (response.ok) {
            const exec = await response.json();
 const toolName = exec.toolName || (typeof window.t === 'function' ? window.t('chat.unknownTool') : '');
 // ( name::toolName , toolName )
            const displayToolName = toolName.includes('::') ? toolName.split('::')[1] : toolName;
            button.querySelector('span').textContent = `${displayToolName} #${index}`;
        }
    } catch (error) {
 // ,
 console.error(':', error);
    }
}

// MCP
async function showMCPDetail(executionId) {
    try {
        const response = await apiFetch(`/api/monitor/execution/${executionId}`);
        const exec = await response.json();
        
        if (response.ok) {
 // 
            document.getElementById('detail-tool-name').textContent = exec.toolName || (typeof window.t === 'function' ? window.t('mcpDetailModal.unknown') : 'Unknown');
            document.getElementById('detail-execution-id').textContent = exec.id || 'N/A';
            const statusEl = document.getElementById('detail-status');
            const normalizedStatus = (exec.status || 'unknown').toLowerCase();
            statusEl.textContent = getStatusText(exec.status);
            statusEl.className = `status-chip status-${normalizedStatus}`;
            try {
                statusEl.dataset.detailStatus = (exec.status || '') + '';
            } catch (e) { /* ignore */ }
            const detailTimeLocale = (typeof window.__locale === 'string' && window.__locale.startsWith('zh')) ? 'zh-CN' : 'en-US';
            const detailTimeEl = document.getElementById('detail-time');
            if (detailTimeEl) {
                detailTimeEl.textContent = exec.startTime
                    ? new Date(exec.startTime).toLocaleString(detailTimeLocale)
                    : '-';
                try {
                    detailTimeEl.dataset.detailTimeIso = exec.startTime ? new Date(exec.startTime).toISOString() : '';
                } catch (e) { /* ignore */ }
            }
            
 // 
            const requestData = {
                tool: exec.toolName,
                arguments: exec.arguments
            };
            document.getElementById('detail-request').textContent = JSON.stringify(requestData, null, 2);
            
 // + / 
            const responseElement = document.getElementById('detail-response');
            const successSection = document.getElementById('detail-success-section');
            const successElement = document.getElementById('detail-success');
            const errorSection = document.getElementById('detail-error-section');
            const errorElement = document.getElementById('detail-error');

 // 
            responseElement.className = 'code-block';
            responseElement.textContent = '';
            if (successSection && successElement) {
                successSection.style.display = 'none';
                successElement.textContent = '';
            }
            if (errorSection && errorElement) {
                errorSection.style.display = 'none';
                errorElement.textContent = '';
            }

            if (exec.result) {
                const responseData = {
                    content: exec.result.content,
                    isError: exec.result.isError
                };
                responseElement.textContent = JSON.stringify(responseData, null, 2);

                if (exec.result.isError) {
 // : + 
                    responseElement.className = 'code-block error';
                    if (exec.error && errorSection && errorElement) {
                        errorSection.style.display = 'block';
                        errorElement.textContent = exec.error;
                    }
                } else {
 // :,
                    responseElement.className = 'code-block';
                    if (successSection && successElement) {
                        successSection.style.display = 'block';
                        let successText = '';
                        const content = exec.result.content;
                        if (typeof content === 'string') {
                            successText = content;
                        } else if (Array.isArray(content)) {
                            const texts = content
                                .map(item => (item && typeof item === 'object' && typeof item.text === 'string') ? item.text : '')
                                .filter(Boolean);
                            if (texts.length > 0) {
                                successText = texts.join('\n\n');
                            }
                        } else if (content && typeof content === 'object' && typeof content.text === 'string') {
                            successText = content.text;
                        }
                        if (!successText) {
 successText = typeof window.t === 'function' ? window.t('mcpDetailModal.execSuccessNoContent') : ',.';
                        }
                        successElement.textContent = successText;
                    }
                }
            } else {
 responseElement.textContent = typeof window.t === 'function' ? window.t('chat.noResponseData') : '';
            }
            
 // 
            document.getElementById('mcp-detail-modal').style.display = 'block';
        } else {
 alert((typeof window.t === 'function' ? window.t('mcpDetailModal.getDetailFailed') : '') + ': ' + (exec.error || (typeof window.t === 'function' ? window.t('mcpDetailModal.unknown') : '')));
        }
    } catch (error) {
 alert((typeof window.t === 'function' ? window.t('mcpDetailModal.getDetailFailed') : '') + ': ' + error.message);
    }
}

// MCP
function closeMCPDetail() {
    document.getElementById('mcp-detail-modal').style.display = 'none';
}

// 
function copyDetailBlock(elementId, triggerBtn = null) {
    const target = document.getElementById(elementId);
    if (!target) {
        return;
    }
    const text = target.textContent || '';
    if (!text.trim()) {
        return;
    }

    const originalLabel = triggerBtn ? (triggerBtn.dataset.originalLabel || triggerBtn.textContent.trim()) : '';
    if (triggerBtn && !triggerBtn.dataset.originalLabel) {
        triggerBtn.dataset.originalLabel = originalLabel;
    }

    const showCopiedState = () => {
        if (!triggerBtn) {
            return;
        }
 triggerBtn.textContent = '';
        triggerBtn.disabled = true;
        setTimeout(() => {
            triggerBtn.disabled = false;
 triggerBtn.textContent = triggerBtn.dataset.originalLabel || originalLabel || '';
        }, 1200);
    };

    const fallbackCopy = (value) => {
        return new Promise((resolve, reject) => {
            const textarea = document.createElement('textarea');
            textarea.value = value;
            textarea.style.position = 'fixed';
            textarea.style.opacity = '0';
            document.body.appendChild(textarea);
            textarea.focus();
            textarea.select();
            try {
                const successful = document.execCommand('copy');
                document.body.removeChild(textarea);
                if (successful) {
                    resolve();
                } else {
                    reject(new Error('execCommand failed'));
                }
            } catch (err) {
                document.body.removeChild(textarea);
                reject(err);
            }
        });
    };

    const copyPromise = (navigator.clipboard && typeof navigator.clipboard.writeText === 'function')
        ? navigator.clipboard.writeText(text)
        : fallbackCopy(text);

    copyPromise
        .then(() => {
            showCopiedState();
        })
        .catch(() => {
            if (triggerBtn) {
                triggerBtn.disabled = false;
 triggerBtn.textContent = triggerBtn.dataset.originalLabel || originalLabel || '';
            }
 alert(',.');
        });
}


// 
async function startNewConversation() {
 // ,
    if (currentGroupId) {
        const groupDetailPage = document.getElementById('group-detail-page');
        const chatContainer = document.querySelector('.chat-container');
        if (groupDetailPage) groupDetailPage.style.display = 'none';
        if (chatContainer) chatContainer.style.display = 'flex';
        currentGroupId = null;
 // 
        loadConversationsWithGroups();
    }
    
    currentConversationId = null;
 currentConversationGroupId = null; // 
    document.getElementById('chat-messages').innerHTML = '';
 const readyMsgNew = typeof window.t === 'function' ? window.t('chat.systemReadyMessage') : '.,.';
    addMessage('assistant', readyMsgNew, null, null, null, { systemReadyMessage: true });
    addAttackChainButton(null);
    updateActiveConversation();
 // ,
    await loadGroups();
 // ,
    loadConversationsWithGroups();
 // ,
    if (draftSaveTimer) {
        clearTimeout(draftSaveTimer);
        draftSaveTimer = null;
    }
 // ,
    clearChatDraft();
 // 
    const chatInput = document.getElementById('chat-input');
    if (chatInput) {
        chatInput.value = '';
        adjustTextareaHeight(chatInput);
    }
}

// ()
async function loadConversations(searchQuery = '') {
    try {
        let url = '/api/conversations?limit=50';
        if (searchQuery && searchQuery.trim()) {
            url += '&search=' + encodeURIComponent(searchQuery.trim());
        }
        const response = await apiFetch(url);

        const listContainer = document.getElementById('conversations-list');
        if (!listContainer) {
            return;
        }

 // 
        const sidebarContent = listContainer.closest('.sidebar-content');
        const savedScrollTop = sidebarContent ? sidebarContent.scrollTop : 0;

        const emptyStateHtml = '<div style="padding: 20px; text-align: center; color: var(--text-muted); font-size: 0.875rem;" data-i18n="chat.noHistoryConversations"></div>';
        listContainer.innerHTML = '';

 // 200,(,)
        if (!response.ok) {
            listContainer.innerHTML = emptyStateHtml;
            if (typeof window.applyTranslations === 'function') window.applyTranslations(listContainer);
            return;
        }

        const conversations = await response.json();

        if (!Array.isArray(conversations) || conversations.length === 0) {
            listContainer.innerHTML = emptyStateHtml;
            if (typeof window.applyTranslations === 'function') window.applyTranslations(listContainer);
            return;
        }

        const now = new Date();
        const todayStart = new Date(now.getFullYear(), now.getMonth(), now.getDate());
        const weekday = todayStart.getDay() === 0 ? 7 : todayStart.getDay();
        const startOfWeek = new Date(todayStart);
        startOfWeek.setDate(todayStart.getDate() - (weekday - 1));
        const yesterdayStart = new Date(todayStart);
        yesterdayStart.setDate(todayStart.getDate() - 1);

        const groups = {
            today: [],
            yesterday: [],
            thisWeek: [],
            earlier: [],
        };

        conversations.forEach(conv => {
            const dateObj = conv.updatedAt ? new Date(conv.updatedAt) : new Date();
            const validDate = isNaN(dateObj.getTime()) ? new Date() : dateObj;
            const groupKey = getConversationGroup(validDate, todayStart, startOfWeek, yesterdayStart);
            groups[groupKey].push({
                ...conv,
                _time: validDate,
                _timeText: formatConversationTimestamp(validDate, todayStart, yesterdayStart),
            });
        });

        const groupOrder = [
 { key: 'today', label: '' },
 { key: 'yesterday', label: '' },
 { key: 'thisWeek', label: '' },
 { key: 'earlier', label: '' },
        ];

        const fragment = document.createDocumentFragment();
        let rendered = false;

        groupOrder.forEach(({ key, label }) => {
            const items = groups[key];
            if (!items || items.length === 0) {
                return;
            }
            rendered = true;

            const section = document.createElement('div');
            section.className = 'conversation-group';

            const title = document.createElement('div');
            title.className = 'conversation-group-title';
            title.textContent = label;
            section.appendChild(title);

            items.forEach(itemData => {
 // 
                const isPinned = itemData.pinned || false;
                section.appendChild(createConversationListItemWithMenu(itemData, isPinned));
            });

            fragment.appendChild(section);
        });

        if (!rendered) {
            listContainer.innerHTML = emptyStateHtml;
            if (typeof window.applyTranslations === 'function') window.applyTranslations(listContainer);
            return;
        }

        listContainer.appendChild(fragment);
        updateActiveConversation();
        
 // 
        if (sidebarContent) {
 // requestAnimationFrame DOM 
            requestAnimationFrame(() => {
                sidebarContent.scrollTop = savedScrollTop;
            });
        }
    } catch (error) {
 console.error(':', error);
 // ,()
        const listContainer = document.getElementById('conversations-list');
        if (listContainer) {
            const emptyStateHtml = '<div style="padding: 20px; text-align: center; color: var(--text-muted); font-size: 0.875rem;" data-i18n="chat.noHistoryConversations"></div>';
            listContainer.innerHTML = emptyStateHtml;
            if (typeof window.applyTranslations === 'function') window.applyTranslations(listContainer);
        }
    }
}

function createConversationListItem(conversation) {
    const item = document.createElement('div');
    item.className = 'conversation-item';
    item.dataset.conversationId = conversation.id;
    if (conversation.id === currentConversationId) {
        item.classList.add('active');
    }

    const contentWrapper = document.createElement('div');
    contentWrapper.className = 'conversation-content';

    const title = document.createElement('div');
    title.className = 'conversation-title';
 const titleText = conversation.title || '';
    title.textContent = safeTruncateText(titleText, 60);
 title.title = titleText; // 
    contentWrapper.appendChild(title);

    const time = document.createElement('div');
    time.className = 'conversation-time';
    time.textContent = conversation._timeText || formatConversationTimestamp(conversation._time || new Date());
    contentWrapper.appendChild(time);

    item.appendChild(contentWrapper);

    const deleteBtn = document.createElement('button');
    deleteBtn.className = 'conversation-delete-btn';
    deleteBtn.innerHTML = `
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
            <path d="M3 6h18M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2m3 0v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6h14zM10 11v6M14 11v6" 
                  stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
    `;
 deleteBtn.title = '';
    deleteBtn.onclick = (e) => {
        e.stopPropagation();
        deleteConversation(conversation.id);
    };
    item.appendChild(deleteBtn);

    item.onclick = (e) => {
        e.preventDefault();
        e.stopPropagation();
        loadConversation(conversation.id);
    };
    return item;
}

// 
let conversationSearchTimer = null;
function handleConversationSearch(query) {
 // ,
    if (conversationSearchTimer) {
        clearTimeout(conversationSearchTimer);
    }
    
    const searchInput = document.getElementById('conversation-search-input');
    const clearBtn = document.getElementById('conversation-search-clear');
    
    if (clearBtn) {
        if (query && query.trim()) {
            clearBtn.style.display = 'block';
        } else {
            clearBtn.style.display = 'none';
        }
    }
    
    conversationSearchTimer = setTimeout(() => {
        loadConversations(query);
 }, 300); // 300ms
}

// 
function clearConversationSearch() {
    const searchInput = document.getElementById('conversation-search-input');
    const clearBtn = document.getElementById('conversation-search-clear');
    
    if (searchInput) {
        searchInput.value = '';
    }
    if (clearBtn) {
        clearBtn.style.display = 'none';
    }
    
    loadConversations('');
}

function formatConversationTimestamp(dateObj, todayStart, yesterdayStart) {
    if (!(dateObj instanceof Date) || isNaN(dateObj.getTime())) {
        return '';
    }
 // todayStart,
    const now = new Date();
    const referenceToday = todayStart || new Date(now.getFullYear(), now.getMonth(), now.getDate());
    const referenceYesterday = yesterdayStart || new Date(referenceToday.getTime() - 24 * 60 * 60 * 1000);
    const messageDate = new Date(dateObj.getFullYear(), dateObj.getMonth(), dateObj.getDate());
    const fmtLocale = (typeof window.__locale === 'string' && window.__locale.startsWith('zh')) ? 'zh-CN' : 'en-US';
 const yesterdayLabel = typeof window.t === 'function' ? window.t('chat.yesterday') : '';

    const timeOnlyOpts = { hour: '2-digit', minute: '2-digit' };
    const dateTimeOpts = { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' };
    const fullDateOpts = { year: 'numeric', month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' };
    if (fmtLocale === 'zh-CN') {
        timeOnlyOpts.hour12 = false;
        dateTimeOpts.hour12 = false;
        fullDateOpts.hour12 = false;
    }
    if (messageDate.getTime() === referenceToday.getTime()) {
        return dateObj.toLocaleTimeString(fmtLocale, timeOnlyOpts);
    }
    if (messageDate.getTime() === referenceYesterday.getTime()) {
        return yesterdayLabel + ' ' + dateObj.toLocaleTimeString(fmtLocale, timeOnlyOpts);
    }
    if (dateObj.getFullYear() === referenceToday.getFullYear()) {
        return dateObj.toLocaleString(fmtLocale, dateTimeOpts);
    }
    return dateObj.toLocaleString(fmtLocale, fullDateOpts);
}

function getConversationGroup(dateObj, todayStart, startOfWeek, yesterdayStart) {
    if (!(dateObj instanceof Date) || isNaN(dateObj.getTime())) {
        return 'earlier';
    }
    const today = new Date(todayStart.getFullYear(), todayStart.getMonth(), todayStart.getDate());
    const yesterday = new Date(yesterdayStart.getFullYear(), yesterdayStart.getMonth(), yesterdayStart.getDate());
    const messageDay = new Date(dateObj.getFullYear(), dateObj.getMonth(), dateObj.getDate());

    if (messageDay.getTime() === today.getTime() || messageDay > today) {
        return 'today';
    }
    if (messageDay.getTime() === yesterday.getTime()) {
        return 'yesterday';
    }
    if (messageDay >= startOfWeek && messageDay < today) {
        return 'thisWeek';
    }
    return 'earlier';
}

// 
async function loadConversation(conversationId) {
    try {
 // : processDetails,;
        const response = await apiFetch(`/api/conversations/${conversationId}?include_process_details=0`);
        const conversation = await response.json();
        
        if (!response.ok) {
 alert(': ' + (conversation.error || ''));
            return;
        }
        
 // ,
 // ,,
        if (currentGroupId) {
            const sidebar = document.querySelector('.conversation-sidebar');
            const groupDetailPage = document.getElementById('group-detail-page');
            const chatContainer = document.querySelector('.chat-container');
            
 // 
            if (sidebar) sidebar.style.display = 'flex';
 // ,
            if (groupDetailPage) groupDetailPage.style.display = 'none';
            if (chatContainer) chatContainer.style.display = 'flex';
            
 // ,
 // ,
            const previousGroupId = currentGroupId;
            currentGroupId = null;
            
 // ,()
            loadConversationsWithGroups();
        }
        
 // ID()
 // 
        if (Object.keys(conversationGroupMappingCache).length === 0) {
            await loadConversationGroupMapping();
        }
        currentConversationGroupId = conversationGroupMappingCache[conversationId] || null;
        
 // ,,
 // ,UI
        await loadGroups();
        
 // ID
        currentConversationId = conversationId;
        updateActiveConversation();
        
 // ,
        const attackChainModal = document.getElementById('attack-chain-modal');
        if (attackChainModal && attackChainModal.style.display === 'block') {
            if (currentAttackChainConversationId !== conversationId) {
                closeAttackChainModal();
            }
        }
        
 // 
        const messagesDiv = document.getElementById('chat-messages');
        messagesDiv.innerHTML = '';
        
 // ,,()
        let hasRecentUserMessage = false;
        if (conversation.messages && conversation.messages.length > 0) {
            const lastMessage = conversation.messages[conversation.messages.length - 1];
            if (lastMessage && lastMessage.role === 'user') {
 // ,30,
                const messageTime = new Date(lastMessage.createdAt);
                const now = new Date();
                const timeDiff = now.getTime() - messageTime.getTime();
 if (timeDiff < 30000) { // 30
                    hasRecentUserMessage = true;
                }
            }
        }
        if (hasRecentUserMessage) {
 // ,
            clearChatDraft();
            const chatInput = document.getElementById('chat-input');
            if (chatInput) {
                chatInput.value = '';
                adjustTextareaHeight(chatInput);
            }
        }
        
 // 
        if (conversation.messages && conversation.messages.length > 0) {
            conversation.messages.forEach(msg => {
 // "...",,processDetails
                let displayContent = msg.content;
 if (msg.role === 'assistant' && msg.content === '...' && msg.processDetails && msg.processDetails.length > 0) {
 // errorcancelled
                    for (let i = msg.processDetails.length - 1; i >= 0; i--) {
                        const detail = msg.processDetails[i];
                        if (detail.eventType === 'error' || detail.eventType === 'cancelled') {
                            displayContent = detail.message || msg.content;
                            break;
                        }
                    }
                }
                
 // 
                const messageId = addMessage(msg.role, displayContent, msg.mcpExecutionIds || [], null, msg.createdAt);
 // messageId,
                const messageEl = document.getElementById(messageId);
                if (messageEl && msg && msg.id) {
                    messageEl.dataset.backendMessageId = String(msg.id);
                }
 // ,(processDetails)
                if (msg.role === 'assistant') {
 // ,
                    setTimeout(() => {
 // processDetails , null ","
                        const hasField = msg && Object.prototype.hasOwnProperty.call(msg, 'processDetails');
                        renderProcessDetails(messageId, hasField ? (msg.processDetails || []) : null);
 // ,,,
                        if (msg.processDetails && msg.processDetails.length > 0) {
                            const hasErrorOrCancelled = msg.processDetails.some(d => 
                                d.eventType === 'error' || d.eventType === 'cancelled'
                            );
                            if (hasErrorOrCancelled) {
                                collapseAllProgressDetails(messageId, null);
                            }
                        }
                    }, 100);
                }
            });
        } else {
 const readyMsgEmpty = typeof window.t === 'function' ? window.t('chat.systemReadyMessage') : '.,.';
            addMessage('assistant', readyMsgEmpty, null, null, null, { systemReadyMessage: true });
        }
        
 // 
        messagesDiv.scrollTop = messagesDiv.scrollHeight;
        
 // 
        addAttackChainButton(conversationId);
        
 // 
        loadConversations();
    } catch (error) {
 console.error(':', error);
 alert(': ' + error.message);
    }
}

// 
async function deleteConversation(conversationId, skipConfirm = false) {
 // ()
    if (!skipConfirm) {
 if (!confirm('?.')) {
            return;
        }
    }
    
    try {
        const response = await apiFetch(`/api/conversations/${conversationId}`, {
            method: 'DELETE'
        });
        
        if (!response.ok) {
            const error = await response.json();
 throw new Error(error.error || '');
        }
        
 // ,
        if (conversationId === currentConversationId) {
            currentConversationId = null;
            document.getElementById('chat-messages').innerHTML = '';
 const readyMsgLoad = typeof window.t === 'function' ? window.t('chat.systemReadyMessage') : '.,.';
            addMessage('assistant', readyMsgLoad, null, null, null, { systemReadyMessage: true });
            addAttackChainButton(null);
        }
        
 // - ,
        delete conversationGroupMappingCache[conversationId];
 // 
        delete pendingGroupMappings[conversationId];
        
 // ,
        if (currentGroupId) {
            await loadGroupConversations(currentGroupId);
        }
        
 // ()
        if (typeof loadConversationsWithGroups === 'function') {
            loadConversationsWithGroups();
        } else if (typeof loadConversations === 'function') {
            loadConversations();
        }
 // ( WebShell AI ),
        try {
            document.dispatchEvent(new CustomEvent('conversation-deleted', { detail: { conversationId } }));
        } catch (e) { /* ignore */ }
    } catch (error) {
 console.error(':', error);
 alert(': ' + error.message);
    }
}

// 
function updateActiveConversation() {
    document.querySelectorAll('.conversation-item').forEach(item => {
        item.classList.remove('active');
        if (currentConversationId && item.dataset.conversationId === currentConversationId) {
            item.classList.add('active');
        }
    });
}

// ==================== ====================

let attackChainCytoscape = null;
let currentAttackChainConversationId = null;
// ID,
const attackChainLoadingMap = new Map(); // Map<conversationId, boolean>

// 
function isAttackChainLoading(conversationId) {
    return attackChainLoadingMap.get(conversationId) === true;
}

// 
function setAttackChainLoading(conversationId, loading) {
    if (loading) {
        attackChainLoadingMap.set(conversationId, true);
    } else {
        attackChainLoadingMap.delete(conversationId);
    }
}

// (,,)
function addAttackChainButton(conversationId) {
 // ,
 // ,
    const conversationHeader = document.getElementById('conversation-header');
    if (conversationHeader) {
        conversationHeader.style.display = 'none';
    }
}

function updateAttackChainAvailability() {
    addAttackChainButton(currentConversationId);
}

// 
async function showAttackChain(conversationId) {
 // ID,,
 // ,()
    if (isAttackChainLoading(conversationId) && currentAttackChainConversationId === conversationId) {
 // ,
        const modal = document.getElementById('attack-chain-modal');
        if (modal && modal.style.display === 'block') {
 console.log(',');
            return;
        }
    }
    
    currentAttackChainConversationId = conversationId;
    const modal = document.getElementById('attack-chain-modal');
    if (!modal) {
 console.error('');
        return;
    }
    
    modal.style.display = 'block';
 // ()
    updateAttackChainStats({ nodes: [], edges: [] });

 // 
    const container = document.getElementById('attack-chain-container');
    if (container) {
 container.innerHTML = '<div class="loading-spinner">' + (typeof window.t === 'function' ? window.t('chat.loading') : '...') + '</div>';
    }
    
 // 
    const detailsPanel = document.getElementById('attack-chain-details');
    if (detailsPanel) {
        detailsPanel.style.display = 'none';
    }
    
 // 
    const regenerateBtn = document.querySelector('button[onclick="regenerateAttackChain()"]');
    if (regenerateBtn) {
        regenerateBtn.disabled = true;
        regenerateBtn.style.opacity = '0.5';
        regenerateBtn.style.cursor = 'not-allowed';
    }
    
 // 
    await loadAttackChain(conversationId);
}

// 
async function loadAttackChain(conversationId) {
    if (isAttackChainLoading(conversationId)) {
 return; // 
    }
    
    setAttackChainLoading(conversationId, true);
    
    try {
        const response = await apiFetch(`/api/attack-chain/${conversationId}`);
        
        if (!response.ok) {
 // 409 Conflict()
            if (response.status === 409) {
                const error = await response.json();
                const container = document.getElementById('attack-chain-container');
                if (container) {
                    container.innerHTML = `
                        <div style="text-align: center; padding: 28px 24px; color: var(--text-secondary);">
                            <div style="display: inline-flex; align-items: center; gap: 8px; font-size: 0.95rem; color: var(--text-primary);">
                                <span role="presentation" aria-hidden="true">⏳</span>
 <span>,</span>
                            </div>
                            <button class="btn-secondary" onclick="refreshAttackChain()" style="margin-top: 12px; font-size: 0.78rem; padding: 4px 12px;">
 
                            </button>
                        </div>
                    `;
                }
 // 5(,)
 // conversationId,
                setTimeout(() => {
 // ID
                    if (currentAttackChainConversationId === conversationId) {
                        refreshAttackChain();
                    }
                }, 5000);
 // 409 ,,
 // refreshAttackChain loadAttackChain 
 // :,
 // (,)
                const regenerateBtn = document.querySelector('button[onclick="regenerateAttackChain()"]');
                if (regenerateBtn) {
                    regenerateBtn.disabled = false;
                    regenerateBtn.style.opacity = '1';
                    regenerateBtn.style.cursor = 'pointer';
                }
 return; // , finally setAttackChainLoading(conversationId, false)
            }
            
            const error = await response.json();
 throw new Error(error.error || '');
        }
        
        const chainData = await response.json();
        
 // ID,
        if (currentAttackChainConversationId !== conversationId) {
 console.log(',,', {
                returned: conversationId,
                current: currentAttackChainConversationId
            });
            setAttackChainLoading(conversationId, false);
            return;
        }
        
 // 
        renderAttackChain(chainData);
        
 // 
        updateAttackChainStats(chainData);
        
 // ,
        setAttackChainLoading(conversationId, false);
        
    } catch (error) {
 console.error(':', error);
        const container = document.getElementById('attack-chain-container');
        if (container) {
 container.innerHTML = '<div class="error-message">' + (typeof window.t === 'function' ? window.t('chat.loadFailed', { message: error.message }) : ': ' + error.message) + '</div>';
        }
 // 
        setAttackChainLoading(conversationId, false);
    } finally {
 // 
        const regenerateBtn = document.querySelector('button[onclick="regenerateAttackChain()"]');
        if (regenerateBtn) {
            regenerateBtn.disabled = false;
            regenerateBtn.style.opacity = '1';
            regenerateBtn.style.cursor = 'pointer';
        }
    }
}

// 
function renderAttackChain(chainData) {
    const container = document.getElementById('attack-chain-container');
    if (!container) {
        return;
    }
    
 // 
    container.innerHTML = '';
    
    if (!chainData.nodes || chainData.nodes.length === 0) {
 container.innerHTML = '<div class="empty-message">' + (typeof window.t === 'function' ? window.t('chat.noAttackChainData') : '') + '</div>';
        return;
    }
    
 // ()
    const nodeCount = chainData.nodes.length;
    const edgeCount = chainData.edges.length;
    const isComplexGraph = nodeCount > 15 || edgeCount > 25;
    
 // :
    chainData.nodes.forEach(node => {
        if (node.label) {
 // :,
            const maxLength = isComplexGraph ? 18 : 22;
            if (node.label.length > maxLength) {
                let truncated = node.label.substring(0, maxLength);
 // 
                const lastPunct = Math.max(
                    truncated.lastIndexOf(','),
                    truncated.lastIndexOf('.'),
                    truncated.lastIndexOf(','),
                    truncated.lastIndexOf(' '),
                    truncated.lastIndexOf('/')
                );
 if (lastPunct > maxLength * 0.6) { // 
                    truncated = truncated.substring(0, lastPunct + 1);
                }
                node.label = truncated + '...';
            }
        }
    });
    
 // Cytoscape
    const elements = [];
    
 // ,,
    chainData.nodes.forEach(node => {
        const riskScore = node.risk_score || 0;
        const nodeType = node.type || '';
        
 // ()
        let typeLabel = '';
        let typeBadge = '';
        let typeColor = '';
        if (nodeType === 'target') {
 typeLabel = '';
 typeBadge = '○'; // ,
 typeColor = '#1976d2'; // 
        } else if (nodeType === 'action') {
 typeLabel = '';
 typeBadge = '▷'; // 
 typeColor = '#f57c00'; // 
        } else if (nodeType === 'vulnerability') {
 typeLabel = '';
 typeBadge = '◇'; // ,
 typeColor = '#d32f2f'; // 
        } else {
            typeLabel = nodeType;
            typeBadge = '•';
            typeColor = '#666';
        }
        
 // 
        let textColor, borderColor, textOutlineWidth, textOutlineColor;
        if (riskScore >= 80) {
 // :,
            textColor = '#fff';
            borderColor = '#fff';
            textOutlineWidth = 1;
            textOutlineColor = '#333';
        } else if (riskScore >= 60) {
 // :,
            textColor = '#fff';
            borderColor = '#fff';
            textOutlineWidth = 1;
            textOutlineColor = '#333';
        } else if (riskScore >= 40) {
 // :,
            textColor = '#333';
            borderColor = '#cc9900';
            textOutlineWidth = 2;
            textOutlineColor = '#fff';
        } else {
 // :,
            textColor = '#1a5a1a';
            borderColor = '#5a8a5a';
            textOutlineWidth = 2;
            textOutlineColor = '#fff';
        }
        
 // ,()
        elements.push({
            data: {
                id: node.id,
 label: node.label, // 
 originalLabel: node.label, // 
                type: nodeType,
 typeLabel: typeLabel, // 
 typeBadge: typeBadge, // 
 typeColor: typeColor, // 
                riskScore: riskScore,
                toolExecutionId: node.tool_execution_id || '',
                metadata: node.metadata || {},
                textColor: textColor,
                borderColor: borderColor,
                textOutlineWidth: textOutlineWidth,
                textOutlineColor: textOutlineColor
            }
        });
    });
    
 // ()
    const nodeIds = new Set(chainData.nodes.map(node => node.id));
    
 // ELK
    const validEdges = [];
    chainData.edges.forEach(edge => {
 // 
        if (nodeIds.has(edge.source) && nodeIds.has(edge.target)) {
            validEdges.push(edge);
            elements.push({
                data: {
                    id: edge.id,
                    source: edge.source,
                    target: edge.target,
                    type: edge.type || 'leads_to',
                    weight: edge.weight || 1
                }
            });
        } else {
 console.warn(':', {
                edgeId: edge.id,
                source: edge.source,
                target: edge.target,
                sourceExists: nodeIds.has(edge.source),
                targetExists: nodeIds.has(edge.target)
            });
        }
    });
    
 // Cytoscape
    attackChainCytoscape = cytoscape({
        container: container,
        elements: elements,
        style: [
            {
                selector: 'node',
                style: {
 // :,
                    'label': function(ele) {
                        const typeLabel = ele.data('typeLabel') || '';
                        const label = ele.data('label') || '';
 // : + 
                        return typeLabel + '\n' + label;
                    },
 // ,
                    'width': function(ele) {
                        const type = ele.data('type');
                        if (type === 'target') return isComplexGraph ? 280 : 320;
                        if (type === 'vulnerability') return isComplexGraph ? 260 : 300;
                        return isComplexGraph ? 240 : 280;
                    },
                    'height': function(ele) {
                        const type = ele.data('type');
                        if (type === 'target') return isComplexGraph ? 100 : 120;
                        if (type === 'vulnerability') return isComplexGraph ? 90 : 110;
                        return isComplexGraph ? 80 : 100;
                    },
                    'shape': 'round-rectangle',
 // : + 
                    'background-color': '#FFFFFF',
                    'background-opacity': 1,
 // ()
                    'border-width': function(ele) {
                        const type = ele.data('type');
 return 0; // ,
                    },
                    'border-color': 'transparent',
 // :
 'color': '#2C3E50', // ,
                    'font-size': function(ele) {
                        const type = ele.data('type');
                        if (type === 'target') return isComplexGraph ? '14px' : '16px';
                        if (type === 'vulnerability') return isComplexGraph ? '13px' : '15px';
                        return isComplexGraph ? '13px' : '15px';
                    },
 'font-weight': '600', // 
                    'font-family': '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, "Microsoft YaHei", sans-serif',
                    'text-valign': 'center',
                    'text-halign': 'center',
                    'text-wrap': 'wrap',
                    'text-max-width': function(ele) {
                        const type = ele.data('type');
                        if (type === 'target') return isComplexGraph ? '240px' : '280px';
                        if (type === 'vulnerability') return isComplexGraph ? '220px' : '260px';
                        return isComplexGraph ? '200px' : '240px';
                    },
                    'text-overflow-wrap': 'anywhere',
                    'text-margin-y': 4,
 'padding': '12px 16px', // 
                    'line-height': 1.5,
                    'text-outline-width': 0
                }
            },
            {
 // :
                selector: 'node[type = "target"]',
                style: {
                    'background-color': '#E3F2FD',
                    'color': '#1565C0',
                    'border-width': 3,
                    'border-color': '#2196F3',
                    'border-style': 'solid'
                }
            },
            {
 // :
                selector: 'node[type = "action"]',
                style: {
                    'background-color': function(ele) {
                        const metadata = ele.data('metadata') || {};
                        const findings = metadata.findings || [];
                        const status = metadata.status || '';
                        const hasFindings = Array.isArray(findings) && findings.length > 0;
                        const isFailedInsight = status === 'failed_insight';
                        
                        if (hasFindings && !isFailedInsight) {
 return '#E8F5E9'; // 
                        } else {
 return '#F5F5F5'; // 
                        }
                    },
                    'color': '#424242',
                    'border-width': 2,
                    'border-color': function(ele) {
                        const metadata = ele.data('metadata') || {};
                        const findings = metadata.findings || [];
                        const status = metadata.status || '';
                        const hasFindings = Array.isArray(findings) && findings.length > 0;
                        const isFailedInsight = status === 'failed_insight';
                        
                        if (hasFindings && !isFailedInsight) {
 return '#4CAF50'; // 
                        } else {
 return '#9E9E9E'; // 
                        }
                    },
                    'border-style': 'solid'
                }
            },
            {
 // :
                selector: 'node[type = "vulnerability"]',
                style: {
                    'background-color': function(ele) {
                        const riskScore = ele.data('riskScore') || 0;
                        if (riskScore >= 80) return '#FFEBEE';
                        if (riskScore >= 60) return '#FFF3E0';
                        if (riskScore >= 40) return '#FFFDE7';
                        return '#E8F5E9';
                    },
                    'color': function(ele) {
                        const riskScore = ele.data('riskScore') || 0;
                        if (riskScore >= 80) return '#C62828';
                        if (riskScore >= 60) return '#E65100';
                        if (riskScore >= 40) return '#F57C00';
                        return '#2E7D32';
                    },
                    'border-width': 3,
                    'border-color': function(ele) {
                        const riskScore = ele.data('riskScore') || 0;
                        if (riskScore >= 80) return '#F44336';
                        if (riskScore >= 60) return '#FF9800';
                        if (riskScore >= 40) return '#FFC107';
                        return '#4CAF50';
                    },
                    'border-style': 'solid'
                }
            },
            {
                selector: 'edge',
                style: {
 // :
                    'width': function(ele) {
                        const type = ele.data('type');
 if (type === 'discovers') return 2.5; // 
 if (type === 'enables') return 2.5; // 
 return 2; // 
                    },
                    'line-color': function(ele) {
                        const type = ele.data('type');
 if (type === 'discovers') return '#42A5F5'; // 
 if (type === 'targets') return '#42A5F5'; // 
 if (type === 'enables') return '#EF5350'; // 
 if (type === 'leads_to') return '#90A4AE'; // 
                        return '#B0BEC5';
                    },
                    'target-arrow-color': function(ele) {
                        const type = ele.data('type');
                        if (type === 'discovers') return '#42A5F5';
                        if (type === 'targets') return '#42A5F5';
                        if (type === 'enables') return '#EF5350';
                        if (type === 'leads_to') return '#90A4AE';
                        return '#B0BEC5';
                    },
                    'target-arrow-shape': 'triangle',
 'arrow-scale': 1.2, // 
                    'curve-style': 'straight',
 'opacity': 0.7, // 
                    'line-style': function(ele) {
                        const type = ele.data('type');
                        if (type === 'targets') return 'dashed';
                        return 'solid';
                    },
                    'line-dash-pattern': function(ele) {
                        const type = ele.data('type');
                        if (type === 'targets') return [8, 4];
                        return [];
                    }
                }
            },
            {
                selector: 'node:selected',
                style: {
                    'border-width': 5,
                    'border-color': '#0066ff',
                    'z-index': 999,
                    'opacity': 1,
                    'overlay-opacity': 0.1,
                    'overlay-color': '#0066ff'
                }
            }
        ],
        userPanningEnabled: true,
        userZoomingEnabled: true,
        boxSelectionEnabled: true
    });
    
 // ELK(DAG,)
    let layoutOptions = {
        name: 'breadthfirst',
        directed: true,
        spacingFactor: isComplexGraph ? 3.0 : 2.5,
        padding: 40
    };
    
 // ELK.js
 // elk.bundled.jsELK,new ELK()
    let elkInstance = null;
    if (typeof ELK !== 'undefined') {
        try {
            elkInstance = new ELK();
        } catch (e) {
 console.warn('ELK:', e);
        }
    }
    
    if (elkInstance) {
        try {
            
 // ELK
            const elkGraph = {
                id: 'root',
                layoutOptions: {
                    'elk.algorithm': 'layered',
                    'elk.direction': 'DOWN',
 'elk.spacing.nodeNode': String(isComplexGraph ? 100 : 120), // 
 'elk.spacing.edgeNode': '50', // 
 'elk.spacing.edgeEdge': '25', // 
 'elk.layered.spacing.nodeNodeBetweenLayers': String(isComplexGraph ? 150 : 180), // 
 'elk.layered.nodePlacement.strategy': 'SIMPLE', // ,
 'elk.layered.crossingMinimization.strategy': 'INTERACTIVE', // 
 'elk.layered.thoroughness': '10', // 
                    'elk.layered.spacing.edgeNodeBetweenLayers': '50',
                    'elk.layered.nodePlacement.strategy': 'BRANDES_KOEPF',
                    'elk.layered.crossingMinimization.strategy': 'LAYER_SWEEP',
                    'elk.layered.crossingMinimization.forceNodeModelOrder': 'true',
                    'elk.layered.cycleBreaking.strategy': 'GREEDY',
                    'elk.layered.thoroughness': '7',
 'elk.padding': '[top=60,left=100,bottom=60,right=100]', // ,
 'elk.spacing.componentComponent': String(isComplexGraph ? 100 : 120) // 
                },
                children: chainData.nodes.map(node => {
                    const type = node.type || '';
                    return {
                        id: node.id,
                        width: type === 'target' ? (isComplexGraph ? 280 : 320) : 
                               type === 'vulnerability' ? (isComplexGraph ? 260 : 300) : 
                               (isComplexGraph ? 240 : 280),
                        height: type === 'target' ? (isComplexGraph ? 100 : 120) : 
                                type === 'vulnerability' ? (isComplexGraph ? 90 : 110) : 
                                (isComplexGraph ? 80 : 100)
                    };
                }),
                edges: validEdges.map(edge => ({
                    id: edge.id,
                    sources: [edge.source],
                    targets: [edge.target]
                }))
            };
            
 // ELK
            elkInstance.layout(elkGraph).then(laidOutGraph => {
 // ELKCytoscape
                if (laidOutGraph && laidOutGraph.children) {
                    laidOutGraph.children.forEach(elkNode => {
                        const cyNode = attackChainCytoscape.getElementById(elkNode.id);
                        if (cyNode && elkNode.x !== undefined && elkNode.y !== undefined) {
                            cyNode.position({
                                x: elkNode.x + (elkNode.width || 0) / 2,
                                y: elkNode.y + (elkNode.height || 0) / 2
                            });
                        }
                    });
                    
 // ,
                    setTimeout(() => {
                        centerAttackChain();
                    }, 150);
                } else {
 throw new Error('ELK');
                }
            }).catch(err => {
 console.warn('ELK,:', err);
 // 
                const layout = attackChainCytoscape.layout(layoutOptions);
                layout.one('layoutstop', () => {
                    setTimeout(() => {
                        centerAttackChain();
                    }, 100);
                });
                layout.run();
            });
        } catch (e) {
 console.warn('ELK,:', e);
 // 
            const layout = attackChainCytoscape.layout(layoutOptions);
            layout.one('layoutstop', () => {
                setTimeout(() => {
                    centerAttackChain();
                }, 100);
            });
            layout.run();
        }
    } else {
 console.warn('ELK.js,.elkjs.');
 // 
        const layout = attackChainCytoscape.layout(layoutOptions);
        layout.one('layoutstop', () => {
            setTimeout(() => {
                centerAttackChain();
            }, 100);
        });
        layout.run();
    }
    
 // 
    function centerAttackChain() {
        try {
            if (!attackChainCytoscape) {
                return;
            }
            
            const container = attackChainCytoscape.container();
            if (!container) {
                return;
            }
            
            const containerWidth = container.offsetWidth;
            const containerHeight = container.offsetHeight;
            
            if (containerWidth === 0 || containerHeight === 0) {
 // 0,
                setTimeout(centerAttackChain, 100);
                return;
            }
            
 // ,
 const padding = 80; // 
            attackChainCytoscape.fit(undefined, padding);
            
 // fit
            setTimeout(() => {
                const extent = attackChainCytoscape.extent();
                if (!extent || typeof extent.x1 === 'undefined' || typeof extent.x2 === 'undefined' || 
                    typeof extent.y1 === 'undefined' || typeof extent.y2 === 'undefined') {
                    return;
                }
                
                const graphWidth = extent.x2 - extent.x1;
                const graphHeight = extent.y2 - extent.y1;
                const currentZoom = attackChainCytoscape.zoom();
                
 // ,
                const availableWidth = containerWidth - padding * 2;
                const availableHeight = containerHeight - padding * 2;
                const widthScale = graphWidth > 0 ? availableWidth / (graphWidth * currentZoom) : 1;
                const heightScale = graphHeight > 0 ? availableHeight / (graphHeight * currentZoom) : 1;
                const scale = Math.min(widthScale, heightScale);
                
 // (0.8-1.3)
                if (scale > 1 && scale < 1.3) {
                    attackChainCytoscape.zoom(currentZoom * scale);
                } else if (scale < 0.8) {
                    attackChainCytoscape.zoom(currentZoom * 0.8);
                }
                
 // 
                const graphCenterX = (extent.x1 + extent.x2) / 2;
                const graphCenterY = (extent.y1 + extent.y2) / 2;
                const zoom = attackChainCytoscape.zoom();
                const pan = attackChainCytoscape.pan();
                
                const graphCenterViewX = graphCenterX * zoom + pan.x;
                const graphCenterViewY = graphCenterY * zoom + pan.y;
                
                const desiredViewX = containerWidth / 2;
                const desiredViewY = containerHeight / 2;
                
                const deltaX = desiredViewX - graphCenterViewX;
                const deltaY = desiredViewY - graphCenterViewY;
                
                attackChainCytoscape.pan({
                    x: pan.x + deltaX,
                    y: pan.y + deltaY
                });
            }, 100);
        } catch (error) {
 console.warn(':', error);
        }
    }
    
 // 
    attackChainCytoscape.on('tap', 'node', function(evt) {
        const node = evt.target;
        showNodeDetails(node.data());
    });
    
 // (CSS)
    attackChainCytoscape.on('mouseover', 'node', function(evt) {
        const node = evt.target;
        node.style('border-width', 5);
        node.style('z-index', 998);
        node.style('overlay-opacity', 0.05);
        node.style('overlay-color', '#333333');
    });
    
    attackChainCytoscape.on('mouseout', 'node', function(evt) {
        const node = evt.target;
        const type = node.data('type');
 // 
        const defaultBorderWidth = type === 'target' ? 5 : 4;
        node.style('border-width', defaultBorderWidth);
        node.style('z-index', 'auto');
        node.style('overlay-opacity', 0);
    });
    
 // 
    window.attackChainOriginalData = chainData;
}

// 
function getEdgeNodes(edge) {
    try {
        const source = edge.source();
        const target = edge.target();
        
 // 
        if (!source || !target || source.length === 0 || target.length === 0) {
            return { source: null, target: null, valid: false };
        }
        
        return { source: source, target: target, valid: true };
    } catch (error) {
 console.warn(':', error, edge.id());
        return { source: null, target: null, valid: false };
    }
}

// ()
function filterAttackChainNodes(searchText) {
    if (!attackChainCytoscape || !window.attackChainOriginalData) {
        return;
    }
    
    const searchLower = searchText.toLowerCase().trim();
    if (searchLower === '') {
 // 
        attackChainCytoscape.nodes().style('display', 'element');
        attackChainCytoscape.edges().style('display', 'element');
 // 
        attackChainCytoscape.nodes().style('border-width', 2);
        return;
    }
    
 // 
    attackChainCytoscape.nodes().forEach(node => {
 // ,
        const originalLabel = node.data('originalLabel') || node.data('label') || '';
        const label = originalLabel.toLowerCase();
        const type = (node.data('type') || '').toLowerCase();
        const matches = label.includes(searchLower) || type.includes(searchLower);
        
        if (matches) {
            node.style('display', 'element');
 // 
            node.style('border-width', 4);
            node.style('border-color', '#0066ff');
        } else {
            node.style('display', 'none');
        }
    });
    
 // 
    attackChainCytoscape.edges().forEach(edge => {
        const { source, target, valid } = getEdgeNodes(edge);
        if (!valid) {
            edge.style('display', 'none');
            return;
        }
        
        const sourceVisible = source.style('display') !== 'none';
        const targetVisible = target.style('display') !== 'none';
        if (sourceVisible && targetVisible) {
            edge.style('display', 'element');
        } else {
            edge.style('display', 'none');
        }
    });
    
 // 
    attackChainCytoscape.fit(undefined, 60);
}

// 
function filterAttackChainByType(type) {
    if (!attackChainCytoscape || !window.attackChainOriginalData) {
        return;
    }
    
    if (type === 'all') {
        attackChainCytoscape.nodes().style('display', 'element');
        attackChainCytoscape.edges().style('display', 'element');
        attackChainCytoscape.nodes().style('border-width', 2);
        attackChainCytoscape.fit(undefined, 60);
        return;
    }
    
 // 
    attackChainCytoscape.nodes().forEach(node => {
        const nodeType = node.data('type') || '';
        if (nodeType === type) {
            node.style('display', 'element');
        } else {
            node.style('display', 'none');
        }
    });
    
 // 
    attackChainCytoscape.edges().forEach(edge => {
        const { source, target, valid } = getEdgeNodes(edge);
        if (!valid) {
            edge.style('display', 'none');
            return;
        }
        
        const sourceVisible = source.style('display') !== 'none';
        const targetVisible = target.style('display') !== 'none';
        if (sourceVisible && targetVisible) {
            edge.style('display', 'element');
        } else {
            edge.style('display', 'none');
        }
    });
    
 // 
    attackChainCytoscape.fit(undefined, 60);
}

// 
function filterAttackChainByRisk(riskLevel) {
    if (!attackChainCytoscape || !window.attackChainOriginalData) {
        return;
    }
    
    if (riskLevel === 'all') {
        attackChainCytoscape.nodes().style('display', 'element');
        attackChainCytoscape.edges().style('display', 'element');
        attackChainCytoscape.nodes().style('border-width', 2);
        attackChainCytoscape.fit(undefined, 60);
        return;
    }
    
 // 
    const riskRanges = {
        'high': [80, 100],
        'medium-high': [60, 79],
        'medium': [40, 59],
        'low': [0, 39]
    };
    
    const [minRisk, maxRisk] = riskRanges[riskLevel] || [0, 100];
    
 // 
    attackChainCytoscape.nodes().forEach(node => {
        const riskScore = node.data('riskScore') || 0;
        if (riskScore >= minRisk && riskScore <= maxRisk) {
            node.style('display', 'element');
        } else {
            node.style('display', 'none');
        }
    });
    
 // 
    attackChainCytoscape.edges().forEach(edge => {
        const { source, target, valid } = getEdgeNodes(edge);
        if (!valid) {
            edge.style('display', 'none');
            return;
        }
        
        const sourceVisible = source.style('display') !== 'none';
        const targetVisible = target.style('display') !== 'none';
        if (sourceVisible && targetVisible) {
            edge.style('display', 'element');
        } else {
            edge.style('display', 'none');
        }
    });
    
 // 
    attackChainCytoscape.fit(undefined, 60);
}

// 
function resetAttackChainFilters() {
 // 
    const searchInput = document.getElementById('attack-chain-search');
    if (searchInput) {
        searchInput.value = '';
    }
    
 // 
    const typeFilter = document.getElementById('attack-chain-type-filter');
    if (typeFilter) {
        typeFilter.value = 'all';
    }
    
 // 
    const riskFilter = document.getElementById('attack-chain-risk-filter');
    if (riskFilter) {
        riskFilter.value = 'all';
    }
    
 // 
    if (attackChainCytoscape) {
        attackChainCytoscape.nodes().forEach(node => {
            node.style('display', 'element');
 node.style('border-width', 2); // 
        });
        attackChainCytoscape.edges().style('display', 'element');
        attackChainCytoscape.fit(undefined, 60);
    }
}

// 
function showNodeDetails(nodeData) {
    const detailsPanel = document.getElementById('attack-chain-details');
    const detailsContent = document.getElementById('attack-chain-details-content');
    
    if (!detailsPanel || !detailsContent) {
        return;
    }
    
 // requestAnimationFrame 
    requestAnimationFrame(() => {
        detailsPanel.style.display = 'flex';
 // ,
        requestAnimationFrame(() => {
            detailsPanel.style.opacity = '1';
        });
    });
    
    let html = `
        <div class="node-detail-item">
 <strong>ID:</strong> <code>${nodeData.id}</code>
        </div>
        <div class="node-detail-item">
 <strong>:</strong> ${getNodeTypeLabel(nodeData.type)}
        </div>
        <div class="node-detail-item">
 <strong>:</strong> ${escapeHtml(nodeData.originalLabel || nodeData.label)}
        </div>
        <div class="node-detail-item">
 <strong>:</strong> ${nodeData.riskScore}/100
        </div>
    `;
    
 // action( + AI)
    if (nodeData.type === 'action' && nodeData.metadata) {
        if (nodeData.metadata.tool_name) {
            html += `
                <div class="node-detail-item">
 <strong>:</strong> <code>${escapeHtml(nodeData.metadata.tool_name)}</code>
                </div>
            `;
        }
        if (nodeData.metadata.tool_intent) {
            html += `
                <div class="node-detail-item">
 <strong>:</strong> <span style="color: #0066ff; font-weight: bold;">${escapeHtml(nodeData.metadata.tool_intent)}</span>
                </div>
            `;
        }
        if (nodeData.metadata.status === 'failed_insight') {
            html += `
                <div class="node-detail-item">
 <strong>:</strong> <span style="color: #ff9800; font-weight: bold;"></span>
                </div>
            `;
        }
        if (nodeData.metadata.ai_analysis) {
            html += `
                <div class="node-detail-item">
 <strong>AI:</strong> <div style="margin-top: 5px; padding: 8px; background: #f5f5f5; border-radius: 4px;">${escapeHtml(nodeData.metadata.ai_analysis)}</div>
                </div>
            `;
        }
        if (nodeData.metadata.findings && Array.isArray(nodeData.metadata.findings) && nodeData.metadata.findings.length > 0) {
            html += `
                <div class="node-detail-item">
 <strong>:</strong>
                    <ul style="margin: 5px 0; padding-left: 20px;">
                        ${nodeData.metadata.findings.map(f => `<li>${escapeHtml(f)}</li>`).join('')}
                    </ul>
                </div>
            `;
        }
    }
    
 // ()
    if (nodeData.type === 'target' && nodeData.metadata && nodeData.metadata.target) {
        html += `
            <div class="node-detail-item">
 <strong>:</strong> <code>${escapeHtml(nodeData.metadata.target)}</code>
            </div>
        `;
    }
    
 // ()
    if (nodeData.type === 'vulnerability' && nodeData.metadata) {
        if (nodeData.metadata.vulnerability_type) {
            html += `
                <div class="node-detail-item">
 <strong>:</strong> ${escapeHtml(nodeData.metadata.vulnerability_type)}
                </div>
            `;
        }
        if (nodeData.metadata.description) {
            html += `
                <div class="node-detail-item">
 <strong>:</strong> ${escapeHtml(nodeData.metadata.description)}
                </div>
            `;
        }
        if (nodeData.metadata.severity) {
            html += `
                <div class="node-detail-item">
 <strong>:</strong> <span style="color: ${getSeverityColor(nodeData.metadata.severity)}; font-weight: bold;">${escapeHtml(nodeData.metadata.severity)}</span>
                </div>
            `;
        }
        if (nodeData.metadata.location) {
            html += `
                <div class="node-detail-item">
 <strong>:</strong> <code>${escapeHtml(nodeData.metadata.location)}</code>
                </div>
            `;
        }
    }
    
    if (nodeData.toolExecutionId) {
        html += `
            <div class="node-detail-item">
 <strong>ID:</strong> <code>${nodeData.toolExecutionId}</code>
            </div>
        `;
    }
    
 // ,
    if (detailsContent) {
        detailsContent.scrollTop = 0;
    }
    
 // requestAnimationFrame DOM 
    requestAnimationFrame(() => {
 // 
        detailsContent.innerHTML = html;
        
 // , DOM 
        requestAnimationFrame(() => {
 // 
            if (detailsContent) {
                detailsContent.scrollTop = 0;
            }
            
 // ,
            const sidebar = document.querySelector('.attack-chain-sidebar-content');
            if (sidebar) {
 // 
                const detailsPanel = document.getElementById('attack-chain-details');
                if (detailsPanel && detailsPanel.offsetParent !== null) {
 // getBoundingClientRect ,
                    const detailsRect = detailsPanel.getBoundingClientRect();
                    const sidebarRect = sidebar.getBoundingClientRect();
                    const scrollTop = sidebar.scrollTop;
                    const relativeTop = detailsRect.top - sidebarRect.top + scrollTop;
 sidebar.scrollTop = relativeTop - 20; // 
                }
            }
        });
    });
}

// 
function getSeverityColor(severity) {
    const colors = {
        'critical': '#ff0000',
        'high': '#ff4444',
        'medium': '#ff8800',
        'low': '#ffbb00'
    };
    return colors[severity.toLowerCase()] || '#666';
}

// 
function getNodeTypeLabel(type) {
    const labels = {
 'action': '',
 'vulnerability': '',
 'target': ''
    };
    return labels[type] || type;
}

// ( i18n, attackChainModal.nodesEdges )
function updateAttackChainStats(chainData) {
    const statsElement = document.getElementById('attack-chain-stats');
    if (statsElement) {
        const nodeCount = chainData.nodes ? chainData.nodes.length : 0;
        const edgeCount = chainData.edges ? chainData.edges.length : 0;
        if (typeof window.t === 'function') {
            statsElement.textContent = window.t('attackChainModal.nodesEdges', {
                nodes: nodeCount,
                edges: edgeCount
            });
        } else {
            statsElement.textContent = `Nodes: ${nodeCount} | Edges: ${edgeCount}`;
        }
    }
}

// ( textContent applyTranslations )
document.addEventListener('languagechange', function () {
    if (window.attackChainOriginalData && typeof updateAttackChainStats === 'function') {
        updateAttackChainStats(window.attackChainOriginalData);
    } else {
        const statsEl = document.getElementById('attack-chain-stats');
        if (statsEl && typeof window.t === 'function') {
            statsEl.textContent = window.t('attackChainModal.nodesEdges', { nodes: 0, edges: 0 });
        }
    }
});

// 
function closeNodeDetails() {
    const detailsPanel = document.getElementById('attack-chain-details');
    if (detailsPanel) {
 // 
        detailsPanel.style.opacity = '0';
        detailsPanel.style.maxHeight = detailsPanel.scrollHeight + 'px';
        
        setTimeout(() => {
            detailsPanel.style.display = 'none';
            detailsPanel.style.maxHeight = '';
            detailsPanel.style.opacity = '';
        }, 300);
    }
    
 // 
    if (attackChainCytoscape) {
        attackChainCytoscape.elements().unselect();
    }
}

// 
function closeAttackChainModal() {
    const modal = document.getElementById('attack-chain-modal');
    if (modal) {
        modal.style.display = 'none';
    }
    
 // 
    closeNodeDetails();
    
 // Cytoscape
    if (attackChainCytoscape) {
        attackChainCytoscape.destroy();
        attackChainCytoscape = null;
    }
    
    currentAttackChainConversationId = null;
}

// ()
// :,
function refreshAttackChain() {
    if (currentAttackChainConversationId) {
 // ,()
        const wasLoading = isAttackChainLoading(currentAttackChainConversationId);
 setAttackChainLoading(currentAttackChainConversationId, false); // ,
        loadAttackChain(currentAttackChainConversationId).finally(() => {
 // (409 ),
 // false()
            if (wasLoading) {
 // ( 409, loadAttackChain )
 // ,
 // 409,loadAttackChain 
            }
        });
    }
}

// 
async function regenerateAttackChain() {
    if (!currentAttackChainConversationId) {
        return;
    }
    
 // ()
    if (isAttackChainLoading(currentAttackChainConversationId)) {
 console.log(',...');
        return;
    }
    
 // ID,
    const savedConversationId = currentAttackChainConversationId;
    setAttackChainLoading(savedConversationId, true);
    
    const container = document.getElementById('attack-chain-container');
    if (container) {
 container.innerHTML = '<div class="loading-spinner">...</div>';
    }
    
 // 
    const regenerateBtn = document.querySelector('button[onclick="regenerateAttackChain()"]');
    if (regenerateBtn) {
        regenerateBtn.disabled = true;
        regenerateBtn.style.opacity = '0.5';
        regenerateBtn.style.cursor = 'not-allowed';
    }
    
    try {
 // 
        const response = await apiFetch(`/api/attack-chain/${savedConversationId}/regenerate`, {
            method: 'POST'
        });
        
        if (!response.ok) {
 // 409 Conflict()
            if (response.status === 409) {
                const error = await response.json();
                if (container) {
                    container.innerHTML = `
                        <div class="loading-spinner" style="text-align: center; padding: 40px;">
 <div style="margin-bottom: 16px;">⏳ ...</div>
                            <div style="color: var(--text-secondary); font-size: 0.875rem;">
 ,
                            </div>
                            <button class="btn-secondary" onclick="refreshAttackChain()" style="margin-top: 16px;">
 
                            </button>
                        </div>
                    `;
                }
 // 5
 // savedConversationId 
                setTimeout(() => {
 // ID,
                    if (currentAttackChainConversationId === savedConversationId && 
                        isAttackChainLoading(savedConversationId)) {
                        refreshAttackChain();
                    }
                }, 5000);
                return;
            }
            
            const error = await response.json();
 throw new Error(error.error || '');
        }
        
        const chainData = await response.json();
        
 // ID,
        if (currentAttackChainConversationId !== savedConversationId) {
 console.log(',,', {
                returned: savedConversationId,
                current: currentAttackChainConversationId
            });
            setAttackChainLoading(savedConversationId, false);
            return;
        }
        
 // 
        renderAttackChain(chainData);
        
 // 
        updateAttackChainStats(chainData);
        
    } catch (error) {
 console.error(':', error);
        if (container) {
 container.innerHTML = `<div class="error-message">: ${error.message}</div>`;
        }
    } finally {
        setAttackChainLoading(savedConversationId, false);
        
 // 
        if (regenerateBtn) {
            regenerateBtn.disabled = false;
            regenerateBtn.style.opacity = '1';
            regenerateBtn.style.cursor = 'pointer';
        }
    }
}

// 
function exportAttackChain(format) {
    if (!attackChainCytoscape) {
 alert('');
        return;
    }
    
 // ()
    setTimeout(() => {
        try {
            if (format === 'png') {
                try {
                    const pngPromise = attackChainCytoscape.png({
                        output: 'blob',
                        bg: 'white',
                        full: true,
                        scale: 1
                    });
                    
 // Promise
                    if (pngPromise && typeof pngPromise.then === 'function') {
                        pngPromise.then(blob => {
                            if (!blob) {
 throw new Error('PNG');
                            }
                            const url = URL.createObjectURL(blob);
                            const a = document.createElement('a');
                            a.href = url;
                            a.download = `attack-chain-${currentAttackChainConversationId || 'export'}-${Date.now()}.png`;
                            document.body.appendChild(a);
                            a.click();
                            document.body.removeChild(a);
                            setTimeout(() => URL.revokeObjectURL(url), 100);
                        }).catch(err => {
 console.error('PNG:', err);
 alert('PNG: ' + (err.message || ''));
                        });
                    } else {
 // Promise,
                        const url = URL.createObjectURL(pngPromise);
                        const a = document.createElement('a');
                        a.href = url;
                        a.download = `attack-chain-${currentAttackChainConversationId || 'export'}-${Date.now()}.png`;
                        document.body.appendChild(a);
                        a.click();
                        document.body.removeChild(a);
                        setTimeout(() => URL.revokeObjectURL(url), 100);
                    }
                } catch (err) {
 console.error('PNG:', err);
 alert('PNG: ' + (err.message || ''));
                }
            } else if (format === 'svg') {
                try {
 // Cytoscape.js 3.x .svg() 
 // : Cytoscape SVG
                    const container = attackChainCytoscape.container();
                    if (!container) {
 throw new Error('');
                    }
                    
 // 
                    const nodes = attackChainCytoscape.nodes();
                    const edges = attackChainCytoscape.edges();
                    
                    if (nodes.length === 0) {
 throw new Error('');
                    }
                    
 // ()
                    let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
                    nodes.forEach(node => {
                        const pos = node.position();
                        const nodeWidth = node.width();
                        const nodeHeight = node.height();
                        const size = Math.max(nodeWidth, nodeHeight) / 2;
                        
                        minX = Math.min(minX, pos.x - size);
                        minY = Math.min(minY, pos.y - size);
                        maxX = Math.max(maxX, pos.x + size);
                        maxY = Math.max(maxY, pos.y + size);
                    });
                    
 // 
                    edges.forEach(edge => {
                        const { source, target, valid } = getEdgeNodes(edge);
                        if (valid) {
                            const sourcePos = source.position();
                            const targetPos = target.position();
                            minX = Math.min(minX, sourcePos.x, targetPos.x);
                            minY = Math.min(minY, sourcePos.y, targetPos.y);
                            maxX = Math.max(maxX, sourcePos.x, targetPos.x);
                            maxY = Math.max(maxY, sourcePos.y, targetPos.y);
                        }
                    });
                    
 // 
                    const padding = 50;
                    minX -= padding;
                    minY -= padding;
                    maxX += padding;
                    maxY += padding;
                    
                    const width = maxX - minX;
                    const height = maxY - minY;
                    
 // SVG 
                    const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
                    svg.setAttribute('width', width.toString());
                    svg.setAttribute('height', height.toString());
                    svg.setAttribute('xmlns', 'http://www.w3.org/2000/svg');
                    svg.setAttribute('viewBox', `${minX} ${minY} ${width} ${height}`);
                    
 // 
                    const bgRect = document.createElementNS('http://www.w3.org/2000/svg', 'rect');
                    bgRect.setAttribute('x', minX.toString());
                    bgRect.setAttribute('y', minY.toString());
                    bgRect.setAttribute('width', width.toString());
                    bgRect.setAttribute('height', height.toString());
                    bgRect.setAttribute('fill', 'white');
                    svg.appendChild(bgRect);
                    
 // defs 
                    const defs = document.createElementNS('http://www.w3.org/2000/svg', 'defs');
                    
 // ()
                    const edgeTypes = ['discovers', 'targets', 'enables', 'leads_to'];
                    edgeTypes.forEach((type, index) => {
                        let color = '#999';
                        if (type === 'discovers') color = '#3498db';
                        else if (type === 'targets') color = '#0066ff';
                        else if (type === 'enables') color = '#e74c3c';
                        else if (type === 'leads_to') color = '#666';
                        
                        const marker = document.createElementNS('http://www.w3.org/2000/svg', 'marker');
                        marker.setAttribute('id', `arrowhead-${type}`);
                        marker.setAttribute('markerWidth', '10');
                        marker.setAttribute('markerHeight', '10');
                        marker.setAttribute('refX', '9');
                        marker.setAttribute('refY', '3');
                        marker.setAttribute('orient', 'auto');
                        const polygon = document.createElementNS('http://www.w3.org/2000/svg', 'polygon');
                        polygon.setAttribute('points', '0 0, 10 3, 0 6');
                        polygon.setAttribute('fill', color);
                        marker.appendChild(polygon);
                        defs.appendChild(marker);
                    });
                    svg.appendChild(defs);
                    
 // (,)
                    edges.forEach(edge => {
                        const { source, target, valid } = getEdgeNodes(edge);
                        if (!valid) {
 return; // 
                        }
                        
                        const sourcePos = source.position();
                        const targetPos = target.position();
                        const edgeData = edge.data();
                        const edgeType = edgeData.type || 'leads_to';
                        
 // 
                        let lineColor = '#999';
                        if (edgeType === 'discovers') lineColor = '#3498db';
                        else if (edgeType === 'targets') lineColor = '#0066ff';
                        else if (edgeType === 'enables') lineColor = '#e74c3c';
                        else if (edgeType === 'leads_to') lineColor = '#666';
                        
 // ()
                        const path = document.createElementNS('http://www.w3.org/2000/svg', 'path');
 // ()
                        const midX = (sourcePos.x + targetPos.x) / 2;
                        const midY = (sourcePos.y + targetPos.y) / 2;
                        const dx = targetPos.x - sourcePos.x;
                        const dy = targetPos.y - sourcePos.y;
                        const offset = Math.min(30, Math.sqrt(dx * dx + dy * dy) * 0.3);
                        
 // 
                        const controlX = midX + (dy > 0 ? -offset : offset);
                        const controlY = midY + (dx > 0 ? offset : -offset);
                        path.setAttribute('d', `M ${sourcePos.x} ${sourcePos.y} Q ${controlX} ${controlY} ${targetPos.x} ${targetPos.y}`);
                        path.setAttribute('stroke', lineColor);
                        path.setAttribute('stroke-width', '2');
                        path.setAttribute('fill', 'none');
                        path.setAttribute('marker-end', `url(#arrowhead-${edgeType})`);
                        svg.appendChild(path);
                    });
                    
 // 
                    nodes.forEach(node => {
                        const pos = node.position();
                        const nodeData = node.data();
                        const riskScore = nodeData.riskScore || 0;
                        const nodeWidth = node.width();
                        const nodeHeight = node.height();
                        const size = Math.max(nodeWidth, nodeHeight) / 2;
                        
 // 
                        let bgColor = '#88cc00';
                        let textColor = '#1a5a1a';
                        let borderColor = '#5a8a5a';
                        if (riskScore >= 80) {
                            bgColor = '#ff4444';
                            textColor = '#fff';
                            borderColor = '#fff';
                        } else if (riskScore >= 60) {
                            bgColor = '#ff8800';
                            textColor = '#fff';
                            borderColor = '#fff';
                        } else if (riskScore >= 40) {
                            bgColor = '#ffbb00';
                            textColor = '#333';
                            borderColor = '#cc9900';
                        }
                        
 // 
                        const nodeType = nodeData.type;
                        let shapeElement;
                        if (nodeType === 'vulnerability') {
 // 
                            shapeElement = document.createElementNS('http://www.w3.org/2000/svg', 'polygon');
                            const points = [
                                `${pos.x},${pos.y - size}`,
                                `${pos.x + size},${pos.y}`,
                                `${pos.x},${pos.y + size}`,
                                `${pos.x - size},${pos.y}`
                            ].join(' ');
                            shapeElement.setAttribute('points', points);
                        } else if (nodeType === 'target') {
 // ()
                            shapeElement = document.createElementNS('http://www.w3.org/2000/svg', 'polygon');
                            const points = [];
                            for (let i = 0; i < 5; i++) {
                                const angle = (i * 4 * Math.PI / 5) - Math.PI / 2;
                                const x = pos.x + size * Math.cos(angle);
                                const y = pos.y + size * Math.sin(angle);
                                points.push(`${x},${y}`);
                            }
                            shapeElement.setAttribute('points', points.join(' '));
                        } else {
 // 
                            shapeElement = document.createElementNS('http://www.w3.org/2000/svg', 'rect');
                            shapeElement.setAttribute('x', (pos.x - size).toString());
                            shapeElement.setAttribute('y', (pos.y - size).toString());
                            shapeElement.setAttribute('width', (size * 2).toString());
                            shapeElement.setAttribute('height', (size * 2).toString());
                            shapeElement.setAttribute('rx', '5');
                            shapeElement.setAttribute('ry', '5');
                        }
                        
                        shapeElement.setAttribute('fill', bgColor);
                        shapeElement.setAttribute('stroke', borderColor);
                        shapeElement.setAttribute('stroke-width', '2');
                        svg.appendChild(shapeElement);
                        
 // ()
 // ,
                        const label = (nodeData.originalLabel || nodeData.label || nodeData.id || '').toString();
                        const maxLength = 15;
                        
 // ,
                        const textGroup = document.createElementNS('http://www.w3.org/2000/svg', 'g');
                        textGroup.setAttribute('text-anchor', 'middle');
                        textGroup.setAttribute('dominant-baseline', 'middle');
                        
 // ()
                        let lines = [];
                        if (label.length > maxLength) {
                            const words = label.split(' ');
                            let currentLine = '';
                            words.forEach(word => {
                                if ((currentLine + word).length <= maxLength) {
                                    currentLine += (currentLine ? ' ' : '') + word;
                                } else {
                                    if (currentLine) lines.push(currentLine);
                                    currentLine = word;
                                }
                            });
                            if (currentLine) lines.push(currentLine);
 lines = lines.slice(0, 2); // 
                        } else {
                            lines = [label];
                        }
                        
 // ()
                        let textOutlineColor = '#fff';
                        let textOutlineWidth = 2;
                        if (riskScore >= 80 || riskScore >= 60) {
 // /:,,
                            textOutlineColor = '#333';
                            textOutlineWidth = 1;
                        } else if (riskScore >= 40) {
 // :,
                            textOutlineColor = '#fff';
                            textOutlineWidth = 2;
                        } else {
 // :,
                            textOutlineColor = '#fff';
                            textOutlineWidth = 2;
                        }
                        
 // 
                        lines.forEach((line, i) => {
                            const textY = pos.y + (i - (lines.length - 1) / 2) * 16;
                            
 // (,text-outline)
                            const strokeText = document.createElementNS('http://www.w3.org/2000/svg', 'text');
                            strokeText.setAttribute('x', pos.x.toString());
                            strokeText.setAttribute('y', textY.toString());
                            strokeText.setAttribute('fill', 'none');
                            strokeText.setAttribute('stroke', textOutlineColor);
                            strokeText.setAttribute('stroke-width', textOutlineWidth.toString());
                            strokeText.setAttribute('stroke-linejoin', 'round');
                            strokeText.setAttribute('stroke-linecap', 'round');
                            strokeText.setAttribute('font-size', '14px');
                            strokeText.setAttribute('font-weight', 'bold');
                            strokeText.setAttribute('font-family', 'Arial, sans-serif');
                            strokeText.setAttribute('text-anchor', 'middle');
                            strokeText.setAttribute('dominant-baseline', 'middle');
                            strokeText.textContent = line;
                            textGroup.appendChild(strokeText);
                            
 // ()
                            const fillText = document.createElementNS('http://www.w3.org/2000/svg', 'text');
                            fillText.setAttribute('x', pos.x.toString());
                            fillText.setAttribute('y', textY.toString());
                            fillText.setAttribute('fill', textColor);
                            fillText.setAttribute('font-size', '14px');
                            fillText.setAttribute('font-weight', 'bold');
                            fillText.setAttribute('font-family', 'Arial, sans-serif');
                            fillText.setAttribute('text-anchor', 'middle');
                            fillText.setAttribute('dominant-baseline', 'middle');
                            fillText.textContent = line;
                            textGroup.appendChild(fillText);
                        });
                        
                        svg.appendChild(textGroup);
                    });
                    
 // SVG 
                    const serializer = new XMLSerializer();
                    let svgString = serializer.serializeToString(svg);
                    
 // XML 
                    if (!svgString.startsWith('<?xml')) {
                        svgString = '<?xml version="1.0" encoding="UTF-8"?>\n' + svgString;
                    }
                    
                    const blob = new Blob([svgString], { type: 'image/svg+xml;charset=utf-8' });
                    const url = URL.createObjectURL(blob);
                    const a = document.createElement('a');
                    a.href = url;
                    a.download = `attack-chain-${currentAttackChainConversationId || 'export'}-${Date.now()}.svg`;
                    document.body.appendChild(a);
                    a.click();
                    document.body.removeChild(a);
                    setTimeout(() => URL.revokeObjectURL(url), 100);
                } catch (err) {
 console.error('SVG:', err);
 alert('SVG: ' + (err.message || ''));
                }
            } else {
 alert(': ' + format);
            }
        } catch (error) {
 console.error(':', error);
 alert(': ' + (error.message || ''));
        }
 }, 100); // 
}

// ============================================
// 
// ============================================

// (API)
let currentGroupId = null; // 
let currentConversationGroupId = null; // ID()
let contextMenuConversationId = null;
let contextMenuGroupId = null;
let groupsCache = [];
let conversationGroupMappingCache = {};
let pendingGroupMappings = {}; // (API)
let conversationsListLoadSeq = 0; // ,

// 
async function loadGroups() {
    try {
        const response = await apiFetch('/api/groups');
        if (!response.ok) {
            groupsCache = [];
            return;
        }
        const data = await response.json();
 // groupsCache
        if (Array.isArray(data)) {
            groupsCache = data;
        } else {
 // ,(,)
            groupsCache = [];
        }

        const groupsList = document.getElementById('conversation-groups-list');
        if (!groupsList) return;

        groupsList.innerHTML = '';

        if (!Array.isArray(groupsCache) || groupsCache.length === 0) {
            return;
        }

 // :(,)
        const sortedGroups = [...groupsCache];

            sortedGroups.forEach(group => {
            const groupItem = document.createElement('div');
            groupItem.className = 'group-item';
 // :
 // 1. ,(currentGroupId)
 // 2. ,(currentConversationGroupId)
            const shouldHighlight = currentGroupId 
                ? (currentGroupId === group.id)
                : (currentConversationGroupId === group.id);
            if (shouldHighlight) {
                groupItem.classList.add('active');
            }
            const isPinned = group.pinned || false;
            if (isPinned) {
                groupItem.classList.add('pinned');
            }
            groupItem.dataset.groupId = group.id;

            const content = document.createElement('div');
            content.className = 'group-item-content';

            const icon = document.createElement('span');
            icon.className = 'group-item-icon';
            icon.textContent = group.icon || '📁';

            const name = document.createElement('span');
            name.className = 'group-item-name';
            name.textContent = group.name;

            content.appendChild(icon);
            content.appendChild(name);

 // ,
            if (isPinned) {
                const pinIcon = document.createElement('span');
                pinIcon.className = 'group-item-pinned';
                pinIcon.innerHTML = '📌';
 pinIcon.title = '';
                name.appendChild(pinIcon);
            }
            groupItem.appendChild(content);

            const menuBtn = document.createElement('button');
            menuBtn.className = 'group-item-menu';
            menuBtn.innerHTML = '⋯';
            menuBtn.onclick = (e) => {
                e.stopPropagation();
                showGroupContextMenu(e, group.id);
            };
            groupItem.appendChild(menuBtn);

            groupItem.onclick = () => {
                enterGroupDetail(group.id);
            };

            groupsList.appendChild(groupItem);
        });
    } catch (error) {
 console.error(':', error);
    }
}

// ()
async function loadConversationsWithGroups(searchQuery = '') {
    const loadSeq = ++conversationsListLoadSeq;
    try {
 // ,
 // 
        await loadGroups();
        if (loadSeq !== conversationsListLoadSeq) return;
        await loadConversationGroupMapping();
        if (loadSeq !== conversationsListLoadSeq) return;

 // ,limit
        const limit = (searchQuery && searchQuery.trim()) ? 1000 : 100;
        let url = `/api/conversations?limit=${limit}`;
        if (searchQuery && searchQuery.trim()) {
            url += '&search=' + encodeURIComponent(searchQuery.trim());
        }
        const response = await apiFetch(url);
        if (loadSeq !== conversationsListLoadSeq) return;

        const listContainer = document.getElementById('conversations-list');
        if (!listContainer) {
            return;
        }

 // 
        const sidebarContent = listContainer.closest('.sidebar-content');
        const savedScrollTop = sidebarContent ? sidebarContent.scrollTop : 0;

        const emptyStateHtml = '<div style="padding: 20px; text-align: center; color: var(--text-muted); font-size: 0.875rem;" data-i18n="chat.noHistoryConversations"></div>';
        listContainer.innerHTML = '';

 // 200,(,)
        if (!response.ok) {
            listContainer.innerHTML = emptyStateHtml;
            if (typeof window.applyTranslations === 'function') window.applyTranslations(listContainer);
            return;
        }

        const conversations = await response.json();
        if (loadSeq !== conversationsListLoadSeq) return;

 // :ID,ID
        const uniqueConversations = [];
        const seenConversationIds = new Set();
        (Array.isArray(conversations) ? conversations : []).forEach(conv => {
            if (!conv || !conv.id || seenConversationIds.has(conv.id)) {
                return;
            }
            seenConversationIds.add(conv.id);
            uniqueConversations.push(conv);
        });

        if (uniqueConversations.length === 0) {
            listContainer.innerHTML = emptyStateHtml;
            if (typeof window.applyTranslations === 'function') window.applyTranslations(listContainer);
            return;
        }
        
 // 
        const pinnedConvs = [];
        const normalConvs = [];
        const hasSearchQuery = searchQuery && searchQuery.trim();

        uniqueConversations.forEach(conv => {
 // ,(,)
            if (hasSearchQuery) {
 // ,
                if (conv.pinned) {
                    pinnedConvs.push(conv);
                } else {
                    normalConvs.push(conv);
                }
                return;
            }

 // ,
 // ""
 // ,""
            if (conversationGroupMappingCache[conv.id]) {
 // ,""
                return;
            }

            if (conv.pinned) {
                pinnedConvs.push(conv);
            } else {
                normalConvs.push(conv);
            }
        });

 // 
        const sortByTime = (a, b) => {
            const timeA = a.updatedAt ? new Date(a.updatedAt) : new Date(0);
            const timeB = b.updatedAt ? new Date(b.updatedAt) : new Date(0);
            return timeB - timeA;
        };

        pinnedConvs.sort(sortByTime);
        normalConvs.sort(sortByTime);

        const fragment = document.createDocumentFragment();

 // 
        if (pinnedConvs.length > 0) {
            pinnedConvs.forEach(conv => {
                fragment.appendChild(createConversationListItemWithMenu(conv, true));
            });
        }

 // 
        normalConvs.forEach(conv => {
            fragment.appendChild(createConversationListItemWithMenu(conv, false));
        });

        if (fragment.children.length === 0) {
            listContainer.innerHTML = emptyStateHtml;
            if (typeof window.applyTranslations === 'function') window.applyTranslations(listContainer);
            return;
        }

        if (loadSeq !== conversationsListLoadSeq) return;
        listContainer.appendChild(fragment);
        updateActiveConversation();
        
 // 
        if (sidebarContent) {
 // requestAnimationFrame DOM 
            requestAnimationFrame(() => {
                if (loadSeq === conversationsListLoadSeq) {
                    sidebarContent.scrollTop = savedScrollTop;
                }
            });
        }
    } catch (error) {
        if (loadSeq !== conversationsListLoadSeq) return;
 console.error(':', error);
 // ,()
        const listContainer = document.getElementById('conversations-list');
        if (listContainer) {
            const emptyStateHtml = '<div style="padding: 20px; text-align: center; color: var(--text-muted); font-size: 0.875rem;" data-i18n="chat.noHistoryConversations"></div>';
            listContainer.innerHTML = emptyStateHtml;
            if (typeof window.applyTranslations === 'function') window.applyTranslations(listContainer);
        }
    }
}

// 
function createConversationListItemWithMenu(conversation, isPinned) {
    const item = document.createElement('div');
    item.className = 'conversation-item';
    item.dataset.conversationId = conversation.id;
    if (conversation.id === currentConversationId) {
        item.classList.add('active');
    }

    const contentWrapper = document.createElement('div');
    contentWrapper.className = 'conversation-content';

    const titleWrapper = document.createElement('div');
    titleWrapper.style.display = 'flex';
    titleWrapper.style.alignItems = 'center';
    titleWrapper.style.gap = '4px';

    const title = document.createElement('div');
    title.className = 'conversation-title';
 const titleText = conversation.title || '';
    title.textContent = safeTruncateText(titleText, 60);
 title.title = titleText; // 
    titleWrapper.appendChild(title);

    if (isPinned) {
        const pinIcon = document.createElement('span');
        pinIcon.className = 'conversation-item-pinned';
        pinIcon.innerHTML = '📌';
 pinIcon.title = '';
        titleWrapper.appendChild(pinIcon);
    }

    contentWrapper.appendChild(titleWrapper);

    const time = document.createElement('div');
    time.className = 'conversation-time';
    const dateObj = conversation.updatedAt ? new Date(conversation.updatedAt) : new Date();
    time.textContent = formatConversationTimestamp(dateObj);
    contentWrapper.appendChild(time);

 // ,
    const groupId = conversationGroupMappingCache[conversation.id];
    if (groupId) {
        const group = groupsCache.find(g => g.id === groupId);
        if (group) {
            const groupTag = document.createElement('div');
            groupTag.className = 'conversation-group-tag';
            groupTag.innerHTML = `<span class="group-tag-icon">${group.icon || '📁'}</span><span class="group-tag-name">${group.name}</span>`;
 groupTag.title = `: ${group.name}`;
            contentWrapper.appendChild(groupTag);
        }
    }

    item.appendChild(contentWrapper);

    const menuBtn = document.createElement('button');
    menuBtn.className = 'conversation-item-menu';
    menuBtn.innerHTML = '⋯';
    menuBtn.onclick = (e) => {
        e.stopPropagation();
        contextMenuConversationId = conversation.id;
        showConversationContextMenu(e);
    };
    item.appendChild(menuBtn);

    item.onclick = (e) => {
        e.preventDefault();
        e.stopPropagation();
        if (currentGroupId) {
            exitGroupDetail();
        }
        loadConversation(conversation.id);
    };

    return item;
}

// 
async function showConversationContextMenu(event) {
    const menu = document.getElementById('conversation-context-menu');
    if (!menu) return;

 // ,
    const submenu = document.getElementById('move-to-group-submenu');
    if (submenu) {
        submenu.style.display = 'none';
        submenuVisible = false;
    }
    const downloadSubmenu = document.getElementById('download-markdown-submenu');
    if (downloadSubmenu) {
        downloadSubmenu.style.display = 'none';
    }
 // 
    clearSubmenuHideTimeout();
    clearSubmenuShowTimeout();
    clearDownloadMarkdownSubmenuHideTimeout();
    submenuLoading = false;

    const convId = contextMenuConversationId;
    
 // 
    const attackChainMenuItem = document.getElementById('attack-chain-menu-item');
    if (attackChainMenuItem) {
        if (convId) {
            const isRunning = typeof isConversationTaskRunning === 'function'
                ? isConversationTaskRunning(convId)
                : false;
            if (isRunning) {
                attackChainMenuItem.style.opacity = '0.5';
                attackChainMenuItem.style.cursor = 'not-allowed';
                attackChainMenuItem.onclick = null;
 attackChainMenuItem.title = ',';
            } else {
                attackChainMenuItem.style.opacity = '1';
                attackChainMenuItem.style.cursor = 'pointer';
                attackChainMenuItem.onclick = showAttackChainFromContext;
 attackChainMenuItem.title = (typeof window.t === 'function' ? window.t('chat.viewAttackChainCurrentConv') : '');
            }
        } else {
            attackChainMenuItem.style.opacity = '0.5';
            attackChainMenuItem.style.cursor = 'not-allowed';
            attackChainMenuItem.onclick = null;
 attackChainMenuItem.title = (typeof window.t === 'function' ? window.t('chat.viewAttackChainSelectConv') : '');
        }
    }
    
 // ()
    if (convId) {
        try {
            let isPinned = false;
 // 
            const conversationGroupId = conversationGroupMappingCache[convId];
            const isInCurrentGroup = currentGroupId && conversationGroupId === currentGroupId;
            
            if (isInCurrentGroup) {
 // ,
                const response = await apiFetch(`/api/groups/${currentGroupId}/conversations`);
                if (response.ok) {
                    const groupConvs = await response.json();
                    const conv = groupConvs.find(c => c.id === convId);
                    if (conv) {
                        isPinned = conv.groupPinned || false;
                    }
                }
            } else {
 // ,,
                const response = await apiFetch(`/api/conversations/${convId}`);
                if (response.ok) {
                    const conv = await response.json();
                    isPinned = conv.pinned || false;
                }
            }
            
 // 
            const pinMenuText = document.getElementById('pin-conversation-menu-text');
            if (pinMenuText && typeof window.t === 'function') {
                pinMenuText.textContent = isPinned ? window.t('contextMenu.unpinConversation') : window.t('contextMenu.pinConversation');
            } else if (pinMenuText) {
 pinMenuText.textContent = isPinned ? '' : '';
            }
        } catch (error) {
 console.error(':', error);
            const pinMenuText = document.getElementById('pin-conversation-menu-text');
            if (pinMenuText && typeof window.t === 'function') {
                pinMenuText.textContent = window.t('contextMenu.pinConversation');
            } else if (pinMenuText) {
 pinMenuText.textContent = '';
            }
        }
    } else {
        const pinMenuText = document.getElementById('pin-conversation-menu-text');
        if (pinMenuText && typeof window.t === 'function') {
            pinMenuText.textContent = window.t('contextMenu.pinConversation');
        } else if (pinMenuText) {
 pinMenuText.textContent = '';
        }
    }

 // 
    menu.style.display = 'block';
    menu.style.visibility = 'visible';
    menu.style.opacity = '1';
    
 // 
    void menu.offsetHeight;
    
 // ,
    const menuRect = menu.getBoundingClientRect();
    const viewportWidth = window.innerWidth;
    const viewportHeight = window.innerHeight;
    
 // (,submenu)
 const submenuWidth = submenu ? 180 : 0; // + 
    
    let left = event.clientX;
    let top = event.clientY;
    
 // ,
 // 
    if (left + menuRect.width + submenuWidth > viewportWidth) {
        left = event.clientX - menuRect.width;
 // ,
        if (left < 0) {
            left = Math.max(8, event.clientX - menuRect.width - submenuWidth);
        }
    }
    
 // ,
    if (top + menuRect.height > viewportHeight) {
        top = Math.max(8, event.clientY - menuRect.height);
    }
    
 // 
    if (left < 0) {
        left = 8;
    }
    
 // 
    if (top < 0) {
        top = 8;
    }
    
    menu.style.left = left + 'px';
    menu.style.top = top + 'px';
    
 // ,
    if (left < event.clientX) {
        if (submenu) {
            submenu.style.left = 'auto';
            submenu.style.right = '100%';
            submenu.style.marginLeft = '0';
            submenu.style.marginRight = '4px';
        }
        if (downloadSubmenu) {
            downloadSubmenu.style.left = 'auto';
            downloadSubmenu.style.right = '100%';
            downloadSubmenu.style.marginLeft = '0';
            downloadSubmenu.style.marginRight = '4px';
        }
    } else {
        if (submenu) {
            submenu.style.left = '100%';
            submenu.style.right = 'auto';
            submenu.style.marginLeft = '4px';
            submenu.style.marginRight = '0';
        }
        if (downloadSubmenu) {
            downloadSubmenu.style.left = '100%';
            downloadSubmenu.style.right = 'auto';
            downloadSubmenu.style.marginLeft = '4px';
            downloadSubmenu.style.marginRight = '0';
        }
    }

 // 
    const closeMenu = (e) => {
 // 
        const moveToGroupSubmenuEl = document.getElementById('move-to-group-submenu');
        const downloadMarkdownSubmenuEl = document.getElementById('download-markdown-submenu');
        const clickedInMenu = menu.contains(e.target);
        const clickedInSubmenu = moveToGroupSubmenuEl && moveToGroupSubmenuEl.contains(e.target);
        const clickedInDownloadSubmenu = downloadMarkdownSubmenuEl && downloadMarkdownSubmenuEl.contains(e.target);
        
        if (!clickedInMenu && !clickedInSubmenu && !clickedInDownloadSubmenu) {
 // closeContextMenu 
            closeContextMenu();
            document.removeEventListener('click', closeMenu);
        }
    };
    setTimeout(() => {
        document.addEventListener('click', closeMenu);
    }, 0);
}

// 
async function showGroupContextMenu(event, groupId) {
    const menu = document.getElementById('group-context-menu');
    if (!menu) return;

    contextMenuGroupId = groupId;

 // ()
    try {
 // 
        let group = groupsCache.find(g => g.id === groupId);
        let isPinned = false;
        
        if (group) {
            isPinned = group.pinned || false;
        } else {
 // ,API
            const response = await apiFetch(`/api/groups/${groupId}`);
            if (response.ok) {
                group = await response.json();
                isPinned = group.pinned || false;
            }
        }
        
 // 
        const pinMenuText = document.getElementById('pin-group-menu-text');
        if (pinMenuText && typeof window.t === 'function') {
            pinMenuText.textContent = isPinned ? window.t('contextMenu.unpinGroup') : window.t('contextMenu.pinGroup');
        } else if (pinMenuText) {
 pinMenuText.textContent = isPinned ? '' : '';
        }
    } catch (error) {
 console.error(':', error);
        const pinMenuText = document.getElementById('pin-group-menu-text');
        if (pinMenuText && typeof window.t === 'function') {
            pinMenuText.textContent = window.t('contextMenu.pinGroup');
        } else if (pinMenuText) {
 pinMenuText.textContent = '';
        }
    }

 // 
    menu.style.display = 'block';
    menu.style.visibility = 'visible';
    menu.style.opacity = '1';
    
 // 
    void menu.offsetHeight;
    
 // ,
    const menuRect = menu.getBoundingClientRect();
    const viewportWidth = window.innerWidth;
    const viewportHeight = window.innerHeight;
    
    let left = event.clientX;
    let top = event.clientY;
    
 // ,
    if (left + menuRect.width > viewportWidth) {
        left = event.clientX - menuRect.width;
    }
    
 // ,
    if (top + menuRect.height > viewportHeight) {
        top = event.clientY - menuRect.height;
    }
    
 // 
    if (left < 0) {
        left = 8;
    }
    
 // 
    if (top < 0) {
        top = 8;
    }
    
    menu.style.left = left + 'px';
    menu.style.top = top + 'px';

 // 
    const closeMenu = (e) => {
        if (!menu.contains(e.target)) {
            menu.style.display = 'none';
            document.removeEventListener('click', closeMenu);
        }
    };
    setTimeout(() => {
        document.addEventListener('click', closeMenu);
    }, 0);
}

// 
async function renameConversation() {
    const convId = contextMenuConversationId;
    if (!convId) return;

 const newTitle = prompt(':', '');
    if (newTitle === null || !newTitle.trim()) {
        closeContextMenu();
        return;
    }

    try {
        const response = await apiFetch(`/api/conversations/${convId}`, {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ title: newTitle.trim() }),
        });

        if (!response.ok) {
            const error = await response.json();
 throw new Error(error.error || '');
        }

 // 
        const item = document.querySelector(`[data-conversation-id="${convId}"]`);
        if (item) {
            const titleEl = item.querySelector('.conversation-title');
            if (titleEl) {
                titleEl.textContent = newTitle.trim();
            }
        }

 // ,
        const groupItem = document.querySelector(`.group-conversation-item[data-conversation-id="${convId}"]`);
        if (groupItem) {
            const groupTitleEl = groupItem.querySelector('.group-conversation-title');
            if (groupTitleEl) {
                groupTitleEl.textContent = newTitle.trim();
            }
        }

 // 
        loadConversationsWithGroups();
    } catch (error) {
 console.error(':', error);
 const failedLabel = typeof window.t === 'function' ? window.t('chat.renameFailed') : '';
 const unknownErr = typeof window.t === 'function' ? window.t('createGroupModal.unknownError') : '';
        alert(failedLabel + ': ' + (error.message || unknownErr));
    }

    closeContextMenu();
}

// 
async function pinConversation() {
    const convId = contextMenuConversationId;
    if (!convId) return;

    try {
 // 
 // ,conversationGroupMappingCache 
 // IDID
        const conversationGroupId = conversationGroupMappingCache[convId];
        const isInCurrentGroup = currentGroupId && conversationGroupId === currentGroupId;
        
 // ,,
        if (isInCurrentGroup) {
 // 
            const response = await apiFetch(`/api/groups/${currentGroupId}/conversations`);
            const groupConvs = await response.json();
            const conv = groupConvs.find(c => c.id === convId);
            
 // ,,
            const currentPinned = conv && conv.groupPinned !== undefined ? conv.groupPinned : false;
            const newPinned = !currentPinned;

 // 
            await apiFetch(`/api/groups/${currentGroupId}/conversations/${convId}/pinned`, {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ pinned: newPinned }),
            });

 // 
            loadGroupConversations(currentGroupId);
        } else {
 // ,,
            const response = await apiFetch(`/api/conversations/${convId}`);
            const conv = await response.json();
            const newPinned = !conv.pinned;

 // 
            await apiFetch(`/api/conversations/${convId}/pinned`, {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ pinned: newPinned }),
            });

            loadConversationsWithGroups();
        }
    } catch (error) {
 console.error(':', error);
 alert(': ' + (error.message || ''));
    }

    closeContextMenu();
}

// 
async function showMoveToGroupSubmenu() {
    const submenu = document.getElementById('move-to-group-submenu');
    if (!submenu) return;

 // ,
    if (submenuVisible && submenu.style.display === 'block') {
        return;
    }

 // ,
    if (submenuLoading) {
        return;
    }

 // 
    clearSubmenuHideTimeout();
    
 // 
    submenuLoading = true;
    submenu.innerHTML = '';

 // - 
    try {
 // ,
        if (!Array.isArray(groupsCache) || groupsCache.length === 0) {
            await loadGroups();
        } else {
 // ,,
 // ,
            try {
                const response = await apiFetch('/api/groups');
                if (response.ok) {
                    const freshGroups = await response.json();
                    if (Array.isArray(freshGroups)) {
                        groupsCache = freshGroups;
                    }
                }
            } catch (err) {
 // ,
 console.warn(',:', err);
            }
        }
        
 // 
        if (!Array.isArray(groupsCache)) {
 console.warn('groupsCache ,');
            groupsCache = [];
 // ,
            if (groupsCache.length === 0) {
                await loadGroups();
            }
        }
    } catch (error) {
 console.error(':', error);
 // ,,
    }

 // ,""
    if (currentGroupId && contextMenuConversationId) {
 // 
        const convInGroup = conversationGroupMappingCache[contextMenuConversationId] === currentGroupId;
        if (convInGroup) {
            const removeItem = document.createElement('div');
            removeItem.className = 'context-submenu-item';
            removeItem.innerHTML = `
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                    <path d="M9 12l6 6M15 12l-6 6" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
 <span></span>
            `;
            removeItem.onclick = () => {
                removeConversationFromGroup(contextMenuConversationId, currentGroupId);
            };
            submenu.appendChild(removeItem);
            
 // 
            const divider = document.createElement('div');
            divider.className = 'context-menu-divider';
            submenu.appendChild(divider);
        }
    }

 // groupsCache 
    if (!Array.isArray(groupsCache)) {
 console.warn('groupsCache ,');
        groupsCache = [];
    }

 // ,()
    if (groupsCache.length > 0) {
 // ID
        const conversationCurrentGroupId = contextMenuConversationId 
            ? conversationGroupMappingCache[contextMenuConversationId] 
            : null;
        
        groupsCache.forEach(group => {
 // 
            if (!group || !group.id || !group.name) {
 console.warn(':', group);
                return;
            }
            
 // ,()
            if (conversationCurrentGroupId && group.id === conversationCurrentGroupId) {
                return;
            }
            
            const item = document.createElement('div');
            item.className = 'context-submenu-item';
            item.innerHTML = `
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
                <span>${group.name}</span>
            `;
            item.onclick = () => {
                moveConversationToGroup(contextMenuConversationId, group.id);
            };
            submenu.appendChild(item);
        });
    } else {
 // ,
 console.warn('showMoveToGroupSubmenu: groupsCache ,');
    }

 // ""
 const addGroupLabel = typeof window.t === 'function' ? window.t('chat.addNewGroup') : '+ ';
    const addItem = document.createElement('div');
    addItem.className = 'context-submenu-item add-group-item';
    addItem.innerHTML = `
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
            <path d="M12 5v14M5 12h14" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
        <span>${addGroupLabel}</span>
    `;
    addItem.onclick = () => {
        showCreateGroupModal(true);
    };
    submenu.appendChild(addItem);

    submenu.style.display = 'block';
    submenuVisible = true;
    submenuLoading = false;
    
 // ,
    setTimeout(() => {
        const submenuRect = submenu.getBoundingClientRect();
        const viewportWidth = window.innerWidth;
        const viewportHeight = window.innerHeight;
        
 // ,
        if (submenuRect.right > viewportWidth) {
            submenu.style.left = 'auto';
            submenu.style.right = '100%';
            submenu.style.marginLeft = '0';
            submenu.style.marginRight = '4px';
        }
        
 // ,
        if (submenuRect.bottom > viewportHeight) {
            const overflow = submenuRect.bottom - viewportHeight;
            const currentTop = parseInt(submenu.style.top) || 0;
            submenu.style.top = (currentTop - overflow - 8) + 'px';
        }
    }, 0);
}

// 
let submenuHideTimeout = null;
// 
let submenuShowTimeout = null;
// 
let submenuLoading = false;
// 
let submenuVisible = false;
// Markdown
let downloadMarkdownSubmenuHideTimeout = null;

// 
function hideMoveToGroupSubmenu() {
    const submenu = document.getElementById('move-to-group-submenu');
    if (submenu) {
        submenu.style.display = 'none';
        submenuVisible = false;
    }
}

// 
function clearSubmenuHideTimeout() {
    if (submenuHideTimeout) {
        clearTimeout(submenuHideTimeout);
        submenuHideTimeout = null;
    }
}

// 
function clearSubmenuShowTimeout() {
    if (submenuShowTimeout) {
        clearTimeout(submenuShowTimeout);
        submenuShowTimeout = null;
    }
}

function clearDownloadMarkdownSubmenuHideTimeout() {
    if (downloadMarkdownSubmenuHideTimeout) {
        clearTimeout(downloadMarkdownSubmenuHideTimeout);
        downloadMarkdownSubmenuHideTimeout = null;
    }
}

function showDownloadMarkdownSubmenu() {
    const submenu = document.getElementById('download-markdown-submenu');
    if (!submenu) return;
    clearDownloadMarkdownSubmenuHideTimeout();
    submenu.style.display = 'block';
}

function hideDownloadMarkdownSubmenu() {
    const submenu = document.getElementById('download-markdown-submenu');
    if (!submenu) return;
    submenu.style.display = 'none';
}

function handleDownloadMarkdownSubmenuEnter() {
    clearDownloadMarkdownSubmenuHideTimeout();
    showDownloadMarkdownSubmenu();
}

function handleDownloadMarkdownSubmenuLeave(event) {
    const submenu = document.getElementById('download-markdown-submenu');
    if (!submenu) return;
    const relatedTarget = event.relatedTarget;
    if (relatedTarget && submenu.contains(relatedTarget)) {
        return;
    }
    clearDownloadMarkdownSubmenuHideTimeout();
    downloadMarkdownSubmenuHideTimeout = setTimeout(() => {
        hideDownloadMarkdownSubmenu();
        downloadMarkdownSubmenuHideTimeout = null;
    }, 200);
}

// ""()
function handleMoveToGroupSubmenuEnter() {
 // 
    clearSubmenuHideTimeout();
    
 // ,
    const submenu = document.getElementById('move-to-group-submenu');
    if (submenu && submenuVisible && submenu.style.display === 'block') {
        return;
    }
    
 // 
    clearSubmenuShowTimeout();
    
 // ,
    submenuShowTimeout = setTimeout(() => {
        showMoveToGroupSubmenu();
        submenuShowTimeout = null;
    }, 100);
}

// ""
function handleMoveToGroupSubmenuLeave(event) {
    const submenu = document.getElementById('move-to-group-submenu');
    if (!submenu) return;
    
 // 
    clearSubmenuShowTimeout();
    
 // 
    const relatedTarget = event.relatedTarget;
    if (relatedTarget && submenu.contains(relatedTarget)) {
 // ,
        return;
    }
    
 // 
    clearSubmenuHideTimeout();
    
 // ,
    submenuHideTimeout = setTimeout(() => {
        hideMoveToGroupSubmenu();
        submenuHideTimeout = null;
    }, 200);
}

// 
async function moveConversationToGroup(convId, groupId) {
    try {
        await apiFetch('/api/groups/conversations', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                conversationId: convId,
                groupId: groupId,
            }),
        });

 // 
        const oldGroupId = conversationGroupMappingCache[convId];
        conversationGroupMappingCache[convId] = groupId;
        
 // ,API
        pendingGroupMappings[convId] = groupId;
        
 // , currentConversationGroupId
        if (currentConversationId === convId) {
            currentConversationGroupId = groupId;
        }
        
 // ,
        if (currentGroupId) {
 // ,,
            if (currentGroupId === oldGroupId || currentGroupId === groupId) {
                await loadGroupConversations(currentGroupId);
            }
        }
        
 // ,
 // ,
 // loadConversationsWithGroups loadConversationGroupMapping,
 // loadConversationGroupMapping pendingGroupMappings 
        await loadConversationsWithGroups();
        
 // :pendingGroupMappings loadConversationGroupMapping 
 // ( loadConversationGroupMapping )
        
 // ,
        await loadGroups();
    } catch (error) {
 console.error(':', error);
 alert(': ' + (error.message || ''));
    }

    closeContextMenu();
}

// 
async function removeConversationFromGroup(convId, groupId) {
    try {
        await apiFetch(`/api/groups/${groupId}/conversations/${convId}`, {
            method: 'DELETE',
        });

 // - ,
        delete conversationGroupMappingCache[convId];
 // 
        delete pendingGroupMappings[convId];
        
 // , currentConversationGroupId
        if (currentConversationId === convId) {
            currentConversationGroupId = null;
        }
        
 // ,
        if (currentGroupId === groupId) {
            await loadGroupConversations(groupId);
        }
        
 // ,
        await loadConversationGroupMapping();
        
 // ,
        await loadGroups();
        
 // ,
 // currentGroupId, null,
        const savedGroupId = currentGroupId;
        currentGroupId = null;
        await loadConversationsWithGroups();
        currentGroupId = savedGroupId;
    } catch (error) {
 console.error(':', error);
 alert(': ' + (error.message || ''));
    }

    closeContextMenu();
}

// 
async function loadConversationGroupMapping() {
    try {
 // ,
        let groups;
        if (Array.isArray(groupsCache) && groupsCache.length > 0) {
            groups = groupsCache;
        } else {
            const response = await apiFetch('/api/groups');
            if (!response.ok) {
 // API,,()
                groups = [];
            } else {
                groups = await response.json();
 // groups,
                if (!Array.isArray(groups)) {
 // null/undefined()
                    if (groups !== null && groups !== undefined) {
 console.warn('loadConversationGroupMapping: groups,', groups);
                    }
                    groups = [];
                }
            }
        }
        
 // 
        const preservedMappings = { ...pendingGroupMappings };
        
        conversationGroupMappingCache = {};

        for (const group of groups) {
            const response = await apiFetch(`/api/groups/${group.id}/conversations`);
            const conversations = await response.json();
 // conversations
            if (Array.isArray(conversations)) {
                conversations.forEach(conv => {
                    conversationGroupMappingCache[conv.id] = group.id;
 // ,()
                    if (preservedMappings[conv.id] === group.id) {
                        delete pendingGroupMappings[conv.id];
                    }
                });
            }
        }
        
 // (API)
        Object.assign(conversationGroupMappingCache, preservedMappings);
    } catch (error) {
 console.error(':', error);
    }
}

// 
function showAttackChainFromContext() {
    const convId = contextMenuConversationId;
    if (!convId) return;
    
    closeContextMenu();
    showAttackChain(convId);
}

function formatConversationDateForMarkdown(value) {
    if (!value) return '';
    const d = new Date(value);
    if (isNaN(d.getTime())) return '';
    const locale = (typeof window.__locale === 'string' && window.__locale.startsWith('zh')) ? 'zh-CN' : 'en-US';
    return d.toLocaleString(locale, {
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
        hour12: false
    });
}

function getConversationRoleLabel(role) {
    switch (role) {
        case 'assistant':
            return 'Assistant';
        case 'user':
            return 'User';
        case 'system':
            return 'System';
        default:
            return role || 'Unknown';
    }
}

function formatConversationAsMarkdown(conversation, options = {}) {
    const includeToolDetails = !!options.includeToolDetails;
    const title = (conversation && conversation.title ? String(conversation.title) : '').trim() || 'Untitled Conversation';
    const createdAt = formatConversationDateForMarkdown(conversation && conversation.createdAt);
    const updatedAt = formatConversationDateForMarkdown(conversation && conversation.updatedAt);
    const messages = Array.isArray(conversation && conversation.messages) ? conversation.messages : [];

    let markdown = `# ${title}\n\n`;
    markdown += `- Conversation ID: \`${conversation && conversation.id ? conversation.id : ''}\`\n`;
    if (createdAt) markdown += `- Created At: ${createdAt}\n`;
    if (updatedAt) markdown += `- Updated At: ${updatedAt}\n`;
    markdown += `- Message Count: ${messages.length}\n\n`;
    markdown += '---\n\n';

    if (messages.length === 0) {
        markdown += '_No messages in this conversation._\n';
        return markdown;
    }

    messages.forEach((msg, index) => {
        const role = getConversationRoleLabel(msg && msg.role);
        const timestamp = formatConversationDateForMarkdown(msg && msg.createdAt);
        const content = msg && typeof msg.content === 'string' ? msg.content : '';

        markdown += `## ${index + 1}. ${role}`;
        if (timestamp) markdown += ` (${timestamp})`;
        markdown += '\n\n';
        markdown += content ? `${content}\n\n` : '_[Empty message]_\n\n';

        if (Array.isArray(msg && msg.processDetails) && msg.processDetails.length > 0) {
            markdown += '### Process Details\n\n';
            msg.processDetails.forEach((detail) => {
                const detailTime = formatConversationDateForMarkdown(detail && detail.timestamp);
                const eventType = detail && detail.eventType ? detail.eventType : 'event';
                const detailMsg = detail && detail.message ? detail.message : '';
                // Avoid "[label]:" pattern because some Markdown parsers treat it as link reference definition.
                markdown += `- \`${eventType}\``;
                if (detailTime) markdown += ` ${detailTime}`;
                if (detailMsg) markdown += `: ${detailMsg}`;
                markdown += '\n';

                if (includeToolDetails && detail && detail.data && (eventType === 'tool_call' || eventType === 'tool_result')) {
                    const pretty = JSON.stringify(detail.data, null, 2);
                    markdown += '\n```json\n';
                    markdown += pretty || '{}';
                    markdown += '\n```\n';
                }
            });
            markdown += '\n';
        }

        if (Array.isArray(msg && msg.mcpExecutionIds) && msg.mcpExecutionIds.length > 0) {
            markdown += `- MCP Execution IDs: ${msg.mcpExecutionIds.join(', ')}\n\n`;
        }

        markdown += '---\n\n';
    });

    return markdown;
}

function buildConversationMarkdownFileName(conversation, options = {}) {
    const includeToolDetails = !!options.includeToolDetails;
    const title = (conversation && conversation.title ? String(conversation.title) : '').trim() || 'conversation';
    const safeTitle = title
        .replace(/[\\/:*?"<>|]/g, '_')
        .replace(/\s+/g, '_')
        .slice(0, 60) || 'conversation';
    const idPart = (conversation && conversation.id ? String(conversation.id) : '').slice(0, 8) || 'export';
    const modePart = includeToolDetails ? 'full' : 'summary';
    return `${safeTitle}_${idPart}_${modePart}.md`;
}

// Markdown
async function downloadConversationMarkdownFromContext(includeToolDetails = false) {
    const convId = contextMenuConversationId;
    if (!convId) return;

    try {
 // :
        const response = await apiFetch(`/api/conversations/${convId}?include_process_details=1`);
        let conversation = null;
        try {
            conversation = await response.json();
        } catch (e) {
            conversation = null;
        }
        if (!response.ok) {
            const errorMsg = conversation && conversation.error ? conversation.error : 'unknown error';
            throw new Error(errorMsg);
        }

        const markdown = formatConversationAsMarkdown(conversation || {}, { includeToolDetails });
        const blob = new Blob([markdown], { type: 'text/markdown;charset=utf-8' });
        const url = URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = url;
        link.download = buildConversationMarkdownFileName(conversation || {}, { includeToolDetails });
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
        URL.revokeObjectURL(url);
    } catch (error) {
 console.error(' Markdown :', error);
 const failedLabel = typeof window.t === 'function' ? window.t('chat.downloadConversationFailed') : '';
        const errMsg = error && error.message ? error.message : 'unknown error';
        alert(failedLabel + ': ' + errMsg);
    }

    closeContextMenu();
}

// 
function deleteConversationFromContext() {
    const convId = contextMenuConversationId;
    if (!convId) return;

 const confirmMsg = typeof window.t === 'function' ? window.t('chat.deleteConversationConfirm') : '?';
    if (confirm(confirmMsg)) {
 deleteConversation(convId, true); // ,
    }
    closeContextMenu();
}

// 
function closeContextMenu() {
    const menu = document.getElementById('conversation-context-menu');
    if (menu) {
        menu.style.display = 'none';
    }
    const submenu = document.getElementById('move-to-group-submenu');
    if (submenu) {
        submenu.style.display = 'none';
        submenuVisible = false;
    }
    const downloadSubmenu = document.getElementById('download-markdown-submenu');
    if (downloadSubmenu) {
        downloadSubmenu.style.display = 'none';
    }
 // 
    clearSubmenuHideTimeout();
    clearSubmenuShowTimeout();
    clearDownloadMarkdownSubmenuHideTimeout();
    submenuLoading = false;
    contextMenuConversationId = null;
}

// 
let allConversationsForBatch = [];

// (), i18n;count 
function updateBatchManageTitle(count) {
    const titleEl = document.getElementById('batch-manage-title');
    if (!titleEl || typeof window.t !== 'function') return;
    const template = window.t('batchManageModal.title', { count: '__C__' });
    const parts = template.split('__C__');
    titleEl.innerHTML = (parts[0] || '') + '<span id="batch-manage-count">' + (count || 0) + '</span>' + (parts[1] || '');
}

async function showBatchManageModal() {
    try {
        const response = await apiFetch('/api/conversations?limit=1000');
        
 // 200,(,)
        if (!response.ok) {
            allConversationsForBatch = [];
        } else {
            const data = await response.json();
            allConversationsForBatch = Array.isArray(data) ? data : [];
        }

        const modal = document.getElementById('batch-manage-modal');
        updateBatchManageTitle(allConversationsForBatch.length);

        renderBatchConversations();
        if (modal) {
            modal.style.display = 'flex';
        }
    } catch (error) {
 console.error(':', error);
 // ,()
        allConversationsForBatch = [];
        const modal = document.getElementById('batch-manage-modal');
        updateBatchManageTitle(0);
        if (modal) {
            renderBatchConversations();
            modal.style.display = 'flex';
        }
    }
}

// ,
function safeTruncateText(text, maxLength = 50) {
    if (!text || typeof text !== 'string') {
        return text || '';
    }
    
 // Array.from ( Unicode )
    const chars = Array.from(text);
    
 // ,
    if (chars.length <= maxLength) {
        return text;
    }
    
 // (,)
    let truncatedChars = chars.slice(0, maxLength);
    
 // ,
 // (20%)
    const searchRange = Math.floor(maxLength * 0.2);
    const breakChars = [',', '.', ',', ' ', ',', '.', ';', ':', '!', '?', '!', '?', '/', '\\', '-', '_'];
    let bestBreakPos = truncatedChars.length;
    
    for (let i = truncatedChars.length - 1; i >= truncatedChars.length - searchRange && i >= 0; i--) {
        if (breakChars.includes(truncatedChars[i])) {
 bestBreakPos = i + 1; // 
            break;
        }
    }
    
 // ,;
    if (bestBreakPos < truncatedChars.length) {
        truncatedChars = truncatedChars.slice(0, bestBreakPos);
    }
    
 // ,
    return truncatedChars.join('') + '...';
}

// 
function renderBatchConversations(filtered = null) {
    const list = document.getElementById('batch-conversations-list');
    if (!list) return;

    const conversations = filtered || allConversationsForBatch;
    list.innerHTML = '';

    conversations.forEach(conv => {
        const row = document.createElement('div');
        row.className = 'batch-conversation-row';
        row.dataset.conversationId = conv.id;

        const checkbox = document.createElement('input');
        checkbox.type = 'checkbox';
        checkbox.className = 'batch-conversation-checkbox';
        checkbox.dataset.conversationId = conv.id;

        const name = document.createElement('div');
        name.className = 'batch-table-col-name';
 const originalTitle = conv.title || (typeof window.t === 'function' ? window.t('batchManageModal.unnamedConversation') : '');
 // ,45()
        const truncatedTitle = safeTruncateText(originalTitle, 45);
        name.textContent = truncatedTitle;
 // title()
        name.title = originalTitle;

        const time = document.createElement('div');
        time.className = 'batch-table-col-time';
        const dateObj = conv.updatedAt ? new Date(conv.updatedAt) : new Date();
        const locale = (typeof i18next !== 'undefined' && i18next.language) ? i18next.language : 'zh-CN';
        time.textContent = dateObj.toLocaleString(locale, {
            year: 'numeric',
            month: '2-digit',
            day: '2-digit',
            hour: '2-digit',
            minute: '2-digit'
        });

        const action = document.createElement('div');
        action.className = 'batch-table-col-action';
        const deleteBtn = document.createElement('button');
        deleteBtn.className = 'batch-delete-btn';
        deleteBtn.innerHTML = '🗑️';
        deleteBtn.onclick = () => deleteConversation(conv.id);
        action.appendChild(deleteBtn);

        row.appendChild(checkbox);
        row.appendChild(name);
        row.appendChild(time);
        row.appendChild(action);

        list.appendChild(row);
    });
}

// 
function filterBatchConversations(query) {
    if (!query || !query.trim()) {
        renderBatchConversations();
        return;
    }

    const filtered = allConversationsForBatch.filter(conv => {
        const title = (conv.title || '').toLowerCase();
        return title.includes(query.toLowerCase());
    });

    renderBatchConversations(filtered);
}

// /
function toggleSelectAllBatch() {
    const selectAll = document.getElementById('batch-select-all');
    const checkboxes = document.querySelectorAll('.batch-conversation-checkbox');
    
    checkboxes.forEach(cb => {
        cb.checked = selectAll.checked;
    });
}

// 
async function deleteSelectedConversations() {
    const checkboxes = document.querySelectorAll('.batch-conversation-checkbox:checked');
    if (checkboxes.length === 0) {
 alert(typeof window.t === 'function' ? window.t('batchManageModal.confirmDeleteNone') : '');
        return;
    }

 const confirmMsg = typeof window.t === 'function' ? window.t('batchManageModal.confirmDeleteN', { count: checkboxes.length }) : ' ' + checkboxes.length + ' ?';
    if (!confirm(confirmMsg)) {
        return;
    }

    const ids = Array.from(checkboxes).map(cb => cb.dataset.conversationId);
    
    try {
        for (const id of ids) {
 await deleteConversation(id, true); // ,
        }
        closeBatchManageModal();
        loadConversationsWithGroups();
    } catch (error) {
 console.error(':', error);
 const failedMsg = typeof window.t === 'function' ? window.t('batchManageModal.deleteFailed') : '';
 const unknownErr = typeof window.t === 'function' ? window.t('createGroupModal.unknownError') : '';
        alert(failedMsg + ': ' + (error.message || unknownErr));
    }
}

// 
function closeBatchManageModal() {
    const modal = document.getElementById('batch-manage-modal');
    if (modal) {
        modal.style.display = 'none';
    }
    const selectAll = document.getElementById('batch-select-all');
    if (selectAll) {
        selectAll.checked = false;
    }
    allConversationsForBatch = [];
}

// (, monitor refreshProgressAndTimelineI18n )
function refreshChatPanelI18n() {
    const locale = (typeof window.__locale === 'string' && window.__locale.startsWith('zh')) ? 'zh-CN' : 'en-US';
    const timeOpts = { hour: '2-digit', minute: '2-digit' };
    if (locale === 'zh-CN') timeOpts.hour12 = false;
    const t = typeof window.t === 'function' ? window.t : function (k) { return k; };

    const messagesEl = document.getElementById('chat-messages');
    if (messagesEl) {
        messagesEl.querySelectorAll('.message-time[data-message-time]').forEach(function (el) {
            try {
                const d = new Date(el.dataset.messageTime);
                if (!isNaN(d.getTime())) {
                    el.textContent = d.toLocaleTimeString(locale, timeOpts);
                }
            } catch (e) { /* ignore */ }
        });
        messagesEl.querySelectorAll('.mcp-call-label').forEach(function (el) {
            el.textContent = '\uD83D\uDCCB ' + t('chat.penetrationTestDetail');
        });
        messagesEl.querySelectorAll('.process-detail-btn').forEach(function (btn) {
            const span = btn.querySelector('span');
            if (!span) return;
            const assistantEl = btn.closest('.message.assistant');
            const messageId = assistantEl && assistantEl.id;
            const detailsId = messageId ? 'process-details-' + messageId : '';
            const timeline = detailsId ? document.getElementById(detailsId) && document.getElementById(detailsId).querySelector('.progress-timeline') : null;
            const expanded = timeline && timeline.classList.contains('expanded');
            span.textContent = expanded ? t('tasks.collapseDetail') : t('chat.expandDetail');
        });
    }

    const mcpModal = document.getElementById('mcp-detail-modal');
    if (mcpModal && mcpModal.style.display === 'block') {
        const detailTimeEl = document.getElementById('detail-time');
        if (detailTimeEl && detailTimeEl.dataset.detailTimeIso) {
            try {
                const d = new Date(detailTimeEl.dataset.detailTimeIso);
                if (!isNaN(d.getTime())) {
                    detailTimeEl.textContent = d.toLocaleString(locale);
                }
            } catch (e) { /* ignore */ }
        }
        const statusEl = document.getElementById('detail-status');
        if (statusEl && statusEl.dataset.detailStatus !== undefined && typeof getStatusText === 'function') {
            statusEl.textContent = getStatusText(statusEl.dataset.detailStatus);
        }
    }
}

// ();;
document.addEventListener('languagechange', function () {
    refreshSystemReadyMessageBubbles();
    refreshChatPanelI18n();
    const modal = document.getElementById('batch-manage-modal');
    if (modal && modal.style.display === 'flex') {
        updateBatchManageTitle(allConversationsForBatch.length);
    }
 // (24h/12h ),
    if (typeof loadConversationsWithGroups === 'function') {
        loadConversationsWithGroups();
    } else if (typeof loadConversations === 'function') {
        loadConversations();
    }
});

// 
function showCreateGroupModal(andMoveConversation = false) {
    const modal = document.getElementById('create-group-modal');
    const input = document.getElementById('create-group-name-input');
    const iconBtn = document.getElementById('create-group-icon-btn');
    const iconPicker = document.getElementById('group-icon-picker');
    const customInput = document.getElementById('custom-icon-input');
    
    if (input) {
        input.value = '';
    }
 // 
    if (iconBtn) {
        iconBtn.textContent = '📁';
    }
 // 
    if (customInput) {
        customInput.value = '';
    }
 // 
    if (iconPicker) {
        iconPicker.style.display = 'none';
    }
    if (modal) {
        modal.style.display = 'flex';
        modal.dataset.moveConversation = andMoveConversation ? 'true' : 'false';
        if (input) {
            setTimeout(() => input.focus(), 100);
        }
    }
}

// 
function closeCreateGroupModal() {
    const modal = document.getElementById('create-group-modal');
    if (modal) {
        modal.style.display = 'none';
    }
    const input = document.getElementById('create-group-name-input');
    if (input) {
        input.value = '';
    }
 // 
    const iconBtn = document.getElementById('create-group-icon-btn');
    if (iconBtn) {
        iconBtn.textContent = '📁';
    }
 // 
    const customInput = document.getElementById('custom-icon-input');
    if (customInput) {
        customInput.value = '';
    }
 // 
    const iconPicker = document.getElementById('group-icon-picker');
    if (iconPicker) {
        iconPicker.style.display = 'none';
    }
}

// 
function selectSuggestion(name) {
    const input = document.getElementById('create-group-name-input');
    if (input) {
        input.value = name;
        input.focus();
    }
}

// i18n key ()
function selectSuggestionByKey(i18nKey) {
    const input = document.getElementById('create-group-name-input');
    if (input && typeof window.t === 'function') {
        input.value = window.t(i18nKey);
        input.focus();
    }
}

// 
function toggleGroupIconPicker() {
    const picker = document.getElementById('group-icon-picker');
    if (picker) {
        const isVisible = picker.style.display !== 'none';
        picker.style.display = isVisible ? 'none' : 'block';
    }
}

// 
function selectGroupIcon(icon) {
    const iconBtn = document.getElementById('create-group-icon-btn');
    if (iconBtn) {
        iconBtn.textContent = icon;
    }
 // 
    const customInput = document.getElementById('custom-icon-input');
    if (customInput) {
        customInput.value = '';
    }
 // 
    const picker = document.getElementById('group-icon-picker');
    if (picker) {
        picker.style.display = 'none';
    }
}

// 
function applyCustomIcon() {
    const customInput = document.getElementById('custom-icon-input');
    if (!customInput) return;
    
    const customIcon = customInput.value.trim();
    if (!customIcon) {
        return;
    }
    
    const iconBtn = document.getElementById('create-group-icon-btn');
    if (iconBtn) {
        iconBtn.textContent = customIcon;
    }
    
 // 
    customInput.value = '';
    const picker = document.getElementById('group-icon-picker');
    if (picker) {
        picker.style.display = 'none';
    }
}

// 
document.addEventListener('DOMContentLoaded', function() {
    const customInput = document.getElementById('custom-icon-input');
    if (customInput) {
        customInput.addEventListener('keydown', function(e) {
            if (e.key === 'Enter') {
                e.preventDefault();
                applyCustomIcon();
            }
        });
    }
    initChatAgentModeFromConfig();
});

// ,
document.addEventListener('click', function(event) {
    const picker = document.getElementById('group-icon-picker');
    const iconBtn = document.getElementById('create-group-icon-btn');
    if (picker && iconBtn) {
 // ,
        if (!picker.contains(event.target) && !iconBtn.contains(event.target)) {
            picker.style.display = 'none';
        }
    }

    const agentWrap = document.getElementById('agent-mode-wrapper');
    const agentPanel = document.getElementById('agent-mode-panel');
    if (agentWrap && agentPanel && agentPanel.style.display === 'flex') {
        if (!agentWrap.contains(event.target)) {
            closeAgentModePanel();
        }
    }
});

// 
async function createGroup(event) {
 // 
    if (event) {
        event.preventDefault();
        event.stopPropagation();
    }

    const input = document.getElementById('create-group-name-input');
    if (!input) {
 console.error('');
        return;
    }

    const name = input.value.trim();
    if (!name) {
 alert(typeof window.t === 'function' ? window.t('createGroupModal.groupNamePlaceholder') : '');
        return;
    }

 // :
    try {
        let groups;
        if (Array.isArray(groupsCache) && groupsCache.length > 0) {
            groups = groupsCache;
        } else {
            const response = await apiFetch('/api/groups');
            groups = await response.json();
        }
        
 // groups
        if (!Array.isArray(groups)) {
            groups = [];
        }
        
        const nameExists = groups.some(g => g.name === name);
        if (nameExists) {
 alert(typeof window.t === 'function' ? window.t('createGroupModal.nameExists') : ',');
            return;
        }
    } catch (error) {
 console.error(':', error);
    }

 // 
    const iconBtn = document.getElementById('create-group-icon-btn');
    const selectedIcon = iconBtn ? iconBtn.textContent.trim() : '📁';

    try {
        const response = await apiFetch('/api/groups', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                name: name,
                icon: selectedIcon,
            }),
        });

        if (!response.ok) {
            const error = await response.json();
 const nameExistsMsg = typeof window.t === 'function' ? window.t('createGroupModal.nameExists') : ',';
 if (error.error && error.error.includes('')) {
                alert(nameExistsMsg);
                return;
            }
 const createFailedMsg = typeof window.t === 'function' ? window.t('createGroupModal.createFailed') : '';
            throw new Error(error.error || createFailedMsg);
        }

        const newGroup = await response.json();
        
 // ""
        const submenu = document.getElementById('move-to-group-submenu');
        const isSubmenuOpen = submenu && submenu.style.display !== 'none';

        await loadGroups();

        const modal = document.getElementById('create-group-modal');
        const shouldMove = modal && modal.dataset.moveConversation === 'true';
        
        closeCreateGroupModal();

        if (shouldMove && contextMenuConversationId) {
            moveConversationToGroup(contextMenuConversationId, newGroup.id);
        }

 // ,,
        if (isSubmenuOpen) {
            await showMoveToGroupSubmenu();
        }
    } catch (error) {
 console.error(':', error);
 const createFailedMsg = typeof window.t === 'function' ? window.t('createGroupModal.createFailed') : '';
 const unknownErr = typeof window.t === 'function' ? window.t('createGroupModal.unknownError') : '';
        alert(createFailedMsg + ': ' + (error.message || unknownErr));
    }
}

// 
async function enterGroupDetail(groupId) {
    currentGroupId = groupId;
 // ,ID,
 // ,
    currentConversationGroupId = null;
    
    try {
        const response = await apiFetch(`/api/groups/${groupId}`);
        const group = await response.json();
        
        if (!group) {
            currentGroupId = null;
            return;
        }

 // ,,
        const sidebar = document.querySelector('.conversation-sidebar');
        const groupDetailPage = document.getElementById('group-detail-page');
        const chatContainer = document.querySelector('.chat-container');
        const titleEl = document.getElementById('group-detail-title');

 // 
        if (sidebar) sidebar.style.display = 'flex';
 // ,
        if (chatContainer) chatContainer.style.display = 'none';
        if (groupDetailPage) groupDetailPage.style.display = 'flex';
        if (titleEl) titleEl.textContent = group.name;

 // ,
        await loadGroups();

 // ()
        loadGroupConversations(groupId, currentGroupSearchQuery);
    } catch (error) {
 console.error(':', error);
        currentGroupId = null;
    }
}

// 
function exitGroupDetail() {
    currentGroupId = null;
 currentGroupSearchQuery = ''; // 
    
 // 
    const searchContainer = document.getElementById('group-search-container');
    const searchInput = document.getElementById('group-search-input');
    if (searchContainer) searchContainer.style.display = 'none';
    if (searchInput) searchInput.value = '';
    
    const sidebar = document.querySelector('.conversation-sidebar');
    const groupDetailPage = document.getElementById('group-detail-page');
    const chatContainer = document.querySelector('.chat-container');

 // 
    if (sidebar) sidebar.style.display = 'flex';
 // ,
    if (groupDetailPage) groupDetailPage.style.display = 'none';
    if (chatContainer) chatContainer.style.display = 'flex';

    loadConversationsWithGroups();
}

// 
async function loadGroupConversations(groupId, searchQuery = '') {
    try {
        if (!groupId) {
            console.error('loadGroupConversations: groupId is null or undefined');
            return;
        }
        
 // 
        if (Object.keys(conversationGroupMappingCache).length === 0) {
            await loadConversationGroupMapping();
        }
        
 // ,
        const list = document.getElementById('group-conversations-list');
        if (!list) {
            console.error('group-conversations-list element not found');
            return;
        }
        
 // 
        if (searchQuery) {
 list.innerHTML = '<div style="padding: 40px; text-align: center; color: var(--text-muted);">' + (typeof window.t === 'function' ? window.t('chat.searching') : '...') + '</div>';
        } else {
 list.innerHTML = '<div style="padding: 40px; text-align: center; color: var(--text-muted);">' + (typeof window.t === 'function' ? window.t('chat.loading') : '...') + '</div>';
        }

 // URL,search
        let url = `/api/groups/${groupId}/conversations`;
        if (searchQuery && searchQuery.trim()) {
            url += '?search=' + encodeURIComponent(searchQuery.trim());
        }
        
        const response = await apiFetch(url);
        if (!response.ok) {
            console.error(`Failed to load conversations for group ${groupId}:`, response.statusText);
 list.innerHTML = '<div style="padding: 40px; text-align: center; color: var(--text-muted);">' + (typeof window.t === 'function' ? window.t('chat.loadFailedRetry') : ',') + '</div>';
            return;
        }
        
        let groupConvs = await response.json();
        
 // null undefined ,
        if (!groupConvs) {
            groupConvs = [];
        }
        
 // 
        if (!Array.isArray(groupConvs)) {
            console.error(`Invalid response for group ${groupId}:`, groupConvs);
 list.innerHTML = '<div style="padding: 40px; text-align: center; color: var(--text-muted);">' + (typeof window.t === 'function' ? window.t('chat.dataFormatError') : '') + '</div>';
            return;
        }
        
 // ()
 // ()
        Object.keys(conversationGroupMappingCache).forEach(convId => {
            if (conversationGroupMappingCache[convId] === groupId) {
 // ,
                if (!groupConvs.find(c => c.id === convId)) {
                    delete conversationGroupMappingCache[convId];
                }
            }
        });
        
 // 
        groupConvs.forEach(conv => {
            conversationGroupMappingCache[conv.id] = groupId;
        });

 // ("")
        list.innerHTML = '';

        if (groupConvs.length === 0) {
 const emptyMsg = typeof window.t === 'function' ? window.t('chat.emptyGroupConversations') : '';
 const noMatchMsg = typeof window.t === 'function' ? window.t('chat.noMatchingConversationsInGroup') : '';
            if (searchQuery && searchQuery.trim()) {
 list.innerHTML = '<div style="padding: 40px; text-align: center; color: var(--text-muted);">' + (noMatchMsg || '') + '</div>';
            } else {
 list.innerHTML = '<div style="padding: 40px; text-align: center; color: var(--text-muted);">' + (emptyMsg || '') + '</div>';
            }
            return;
        }

 // 
        for (const conv of groupConvs) {
            try {
 // ID
                if (!conv.id) {
                    console.warn('Conversation missing id:', conv);
                    continue;
                }
                
                const convResponse = await apiFetch(`/api/conversations/${conv.id}`);
                if (!convResponse.ok) {
                    console.error(`Failed to load conversation ${conv.id}:`, convResponse.statusText);
                    continue;
                }
                
                const fullConv = await convResponse.json();
                
                const item = document.createElement('div');
                item.className = 'group-conversation-item';
                item.dataset.conversationId = conv.id;
 // IDactive
 // ,active
                if (currentGroupId && conv.id === currentConversationId) {
                    item.classList.add('active');
                } else {
                    item.classList.remove('active');
                }

 // 
                const contentWrapper = document.createElement('div');
                contentWrapper.className = 'group-conversation-content-wrapper';

                const titleWrapper = document.createElement('div');
                titleWrapper.style.display = 'flex';
                titleWrapper.style.alignItems = 'center';
                titleWrapper.style.gap = '4px';

                const title = document.createElement('div');
                title.className = 'group-conversation-title';
 const titleText = fullConv.title || conv.title || '';
                title.textContent = safeTruncateText(titleText, 60);
 title.title = titleText; // 
                titleWrapper.appendChild(title);

 // ,
                if (conv.groupPinned) {
                    const pinIcon = document.createElement('span');
                    pinIcon.className = 'conversation-item-pinned';
                    pinIcon.innerHTML = '📌';
 pinIcon.title = '';
                    titleWrapper.appendChild(pinIcon);
                }

                contentWrapper.appendChild(titleWrapper);

                const timeWrapper = document.createElement('div');
                timeWrapper.className = 'group-conversation-time';
                const dateObj = fullConv.updatedAt ? new Date(fullConv.updatedAt) : new Date();
                const convListLocale = (typeof window.__locale === 'string' && window.__locale.startsWith('zh')) ? 'zh-CN' : 'en-US';
                timeWrapper.textContent = dateObj.toLocaleString(convListLocale, {
                    year: 'numeric',
                    month: 'long',
                    day: 'numeric',
                    hour: '2-digit',
                    minute: '2-digit'
                });

                contentWrapper.appendChild(timeWrapper);

 // ,
                if (fullConv.messages && fullConv.messages.length > 0) {
                    const firstMsg = fullConv.messages.find(m => m.role === 'user' && m.content);
                    if (firstMsg && firstMsg.content) {
                        const content = document.createElement('div');
                        content.className = 'group-conversation-content';
                        let preview = firstMsg.content.substring(0, 200);
                        if (firstMsg.content.length > 200) {
                            preview += '...';
                        }
                        content.textContent = preview;
                        contentWrapper.appendChild(content);
                    }
                }

                item.appendChild(contentWrapper);

 // 
                const menuBtn = document.createElement('button');
                menuBtn.className = 'conversation-item-menu';
                menuBtn.innerHTML = '⋯';
                menuBtn.onclick = (e) => {
                    e.stopPropagation();
                    contextMenuConversationId = conv.id;
                    showConversationContextMenu(e);
                };
                item.appendChild(menuBtn);

                item.onclick = (e) => {
                    e.preventDefault();
                    e.stopPropagation();
 // ,
                    const groupDetailPage = document.getElementById('group-detail-page');
                    const chatContainer = document.querySelector('.chat-container');
                    if (groupDetailPage) groupDetailPage.style.display = 'none';
                    if (chatContainer) chatContainer.style.display = 'flex';
                    loadConversation(conv.id);
                };

                list.appendChild(item);
            } catch (err) {
 console.error(` ${conv.id} :`, err);
            }
        }
    } catch (error) {
 console.error(':', error);
    }
}

// 
async function editGroup() {
    if (!currentGroupId) return;

    try {
        const response = await apiFetch(`/api/groups/${currentGroupId}`);
        const group = await response.json();
        if (!group) return;

 const renamePrompt = typeof window.t === 'function' ? window.t('chat.renameGroupPrompt') : ':';
        const newName = prompt(renamePrompt, group.name);
        if (newName === null || !newName.trim()) return;

        const trimmedName = newName.trim();
        
 // :()
        let groups;
        if (Array.isArray(groupsCache) && groupsCache.length > 0) {
            groups = groupsCache;
        } else {
            const response = await apiFetch('/api/groups');
            groups = await response.json();
        }
        
 // groups
        if (!Array.isArray(groups)) {
            groups = [];
        }
        
        const nameExists = groups.some(g => g.name === trimmedName && g.id !== currentGroupId);
        if (nameExists) {
 alert(typeof window.t === 'function' ? window.t('createGroupModal.nameExists') : ',');
            return;
        }

        const updateResponse = await apiFetch(`/api/groups/${currentGroupId}`, {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                name: trimmedName,
                icon: group.icon || '📁',
            }),
        });

        if (!updateResponse.ok) {
            const error = await updateResponse.json();
 if (error.error && error.error.includes('')) {
 alert(',');
                return;
            }
 throw new Error(error.error || '');
        }

        loadGroups();
        
        const titleEl = document.getElementById('group-detail-title');
        if (titleEl) {
            titleEl.textContent = trimmedName;
        }
    } catch (error) {
 console.error(':', error);
 alert(': ' + (error.message || ''));
    }
}

// 
async function deleteGroup() {
    if (!currentGroupId) return;

 const deleteConfirmMsg = typeof window.t === 'function' ? window.t('chat.deleteGroupConfirm') : '?,.';
    if (!confirm(deleteConfirmMsg)) {
        return;
    }

    try {
        await apiFetch(`/api/groups/${currentGroupId}`, {
            method: 'DELETE',
        });

 // 
        groupsCache = groupsCache.filter(g => g.id !== currentGroupId);
        Object.keys(conversationGroupMappingCache).forEach(convId => {
            if (conversationGroupMappingCache[convId] === currentGroupId) {
                delete conversationGroupMappingCache[convId];
            }
        });

 // "",
        const submenu = document.getElementById('move-to-group-submenu');
        if (submenu && submenu.style.display !== 'none') {
 // ,
            await loadGroups();
            await showMoveToGroupSubmenu();
        } else {
            exitGroupDetail();
            await loadGroups();
        }
        
 // ,
        await loadConversationsWithGroups();
    } catch (error) {
 console.error(':', error);
 alert(': ' + (error.message || ''));
    }
}

// 
async function renameGroupFromContext() {
    const groupId = contextMenuGroupId;
    if (!groupId) return;

    try {
        const response = await apiFetch(`/api/groups/${groupId}`);
        const group = await response.json();
        if (!group) return;

 const renamePrompt = typeof window.t === 'function' ? window.t('chat.renameGroupPrompt') : ':';
        const newName = prompt(renamePrompt, group.name);
        if (newName === null || !newName.trim()) {
            closeGroupContextMenu();
            return;
        }

        const trimmedName = newName.trim();
        
 // :()
        let groups;
        if (Array.isArray(groupsCache) && groupsCache.length > 0) {
            groups = groupsCache;
        } else {
            const response = await apiFetch('/api/groups');
            groups = await response.json();
        }
        
 // groups
        if (!Array.isArray(groups)) {
            groups = [];
        }
        
        const nameExists = groups.some(g => g.name === trimmedName && g.id !== groupId);
        if (nameExists) {
 alert(typeof window.t === 'function' ? window.t('createGroupModal.nameExists') : ',');
            return;
        }

        const updateResponse = await apiFetch(`/api/groups/${groupId}`, {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                name: trimmedName,
                icon: group.icon || '📁',
            }),
        });

        if (!updateResponse.ok) {
            const error = await updateResponse.json();
 if (error.error && error.error.includes('')) {
 alert(',');
                return;
            }
 throw new Error(error.error || '');
        }

        loadGroups();
        
 // ,
        if (currentGroupId === groupId) {
            const titleEl = document.getElementById('group-detail-title');
            if (titleEl) {
                titleEl.textContent = trimmedName;
            }
        }
    } catch (error) {
 console.error(':', error);
 const failedLabel = typeof window.t === 'function' ? window.t('chat.renameFailed') : '';
 const unknownErr = typeof window.t === 'function' ? window.t('createGroupModal.unknownError') : '';
        alert(failedLabel + ': ' + (error.message || unknownErr));
    }

    closeGroupContextMenu();
}

// 
async function pinGroupFromContext() {
    const groupId = contextMenuGroupId;
    if (!groupId) return;

    try {
 // 
        const response = await apiFetch(`/api/groups/${groupId}`);
        const group = await response.json();
        if (!group) return;

        const newPinnedState = !group.pinned;

 // API 
        const updateResponse = await apiFetch(`/api/groups/${groupId}/pinned`, {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                pinned: newPinnedState,
            }),
        });

        if (!updateResponse.ok) {
            const error = await updateResponse.json();
 throw new Error(error.error || '');
        }

 // 
        loadGroups();
    } catch (error) {
 console.error(':', error);
 alert(': ' + (error.message || ''));
    }

    closeGroupContextMenu();
}

// 
async function deleteGroupFromContext() {
    const groupId = contextMenuGroupId;
    if (!groupId) return;

 const deleteConfirmMsg = typeof window.t === 'function' ? window.t('chat.deleteGroupConfirm') : '?,.';
    if (!confirm(deleteConfirmMsg)) {
        closeGroupContextMenu();
        return;
    }

    try {
        await apiFetch(`/api/groups/${groupId}`, {
            method: 'DELETE',
        });

 // 
        groupsCache = groupsCache.filter(g => g.id !== groupId);
        Object.keys(conversationGroupMappingCache).forEach(convId => {
            if (conversationGroupMappingCache[convId] === groupId) {
                delete conversationGroupMappingCache[convId];
            }
        });

 // "",
        const submenu = document.getElementById('move-to-group-submenu');
        if (submenu && submenu.style.display !== 'none') {
 // ,
            await loadGroups();
            await showMoveToGroupSubmenu();
        } else {
 // ,
            if (currentGroupId === groupId) {
                exitGroupDetail();
            }
            await loadGroups();
        }
        
 // ,
        await loadConversationsWithGroups();
    } catch (error) {
 console.error(':', error);
 alert(': ' + (error.message || ''));
    }

    closeGroupContextMenu();
}

// 
function closeGroupContextMenu() {
    const menu = document.getElementById('group-context-menu');
    if (menu) {
        menu.style.display = 'none';
    }
    contextMenuGroupId = null;
}


// 
let groupSearchTimer = null;
let currentGroupSearchQuery = '';

// /
function toggleGroupSearch() {
    const searchContainer = document.getElementById('group-search-container');
    const searchInput = document.getElementById('group-search-input');
    
    if (!searchContainer || !searchInput) return;
    
    if (searchContainer.style.display === 'none') {
        searchContainer.style.display = 'block';
        searchInput.focus();
    } else {
        searchContainer.style.display = 'none';
        clearGroupSearch();
    }
}

// 
function handleGroupSearchInput(event) {
 // 
    if (event.key === 'Enter') {
        event.preventDefault();
        performGroupSearch();
        return;
    }
    
 // ESC
    if (event.key === 'Escape') {
        clearGroupSearch();
        toggleGroupSearch();
        return;
    }
    
    const searchInput = document.getElementById('group-search-input');
    const clearBtn = document.getElementById('group-search-clear-btn');
    
    if (!searchInput) return;
    
    const query = searchInput.value.trim();
    
 // /
    if (clearBtn) {
        clearBtn.style.display = query ? 'block' : 'none';
    }
    
 // 
    if (groupSearchTimer) {
        clearTimeout(groupSearchTimer);
    }
    
    groupSearchTimer = setTimeout(() => {
        performGroupSearch();
 }, 300); // 300ms 
}

// 
async function performGroupSearch() {
    const searchInput = document.getElementById('group-search-input');
    if (!searchInput || !currentGroupId) return;
    
    const query = searchInput.value.trim();
    currentGroupSearchQuery = query;
    
 // 
    await loadGroupConversations(currentGroupId, query);
}

// 
function clearGroupSearch() {
    const searchInput = document.getElementById('group-search-input');
    const clearBtn = document.getElementById('group-search-clear-btn');
    
    if (searchInput) {
        searchInput.value = '';
    }
    if (clearBtn) {
        clearBtn.style.display = 'none';
    }
    
    currentGroupSearchQuery = '';
    
 // ()
    if (currentGroupId) {
        loadGroupConversations(currentGroupId, '');
    }
}

// 
document.addEventListener('DOMContentLoaded', async () => {
    await loadGroups();
 // loadConversations
    if (typeof loadConversations === 'function') {
 // ,
        const originalLoad = loadConversations;
        loadConversations = function(...args) {
            loadConversationsWithGroups(...args);
        };
    }
    await loadConversationsWithGroups();
    
 // 
 // OpenAPI,
    let lastFocusTime = Date.now();
 const CONVERSATION_REFRESH_INTERVAL = 30000; // 30,
    
    window.addEventListener('focus', () => {
        const now = Date.now();
 // 30,
        if (now - lastFocusTime > CONVERSATION_REFRESH_INTERVAL) {
            lastFocusTime = now;
            if (typeof loadConversationsWithGroups === 'function') {
                loadConversationsWithGroups();
            }
        }
    });
    
 // ()
    document.addEventListener('visibilitychange', () => {
        if (!document.hidden) {
 // ,
            const now = Date.now();
            if (now - lastFocusTime > CONVERSATION_REFRESH_INTERVAL) {
                lastFocusTime = now;
                if (typeof loadConversationsWithGroups === 'function') {
                    loadConversationsWithGroups();
                }
            }
        }
    });

 // :,( WebShell AI )
    document.addEventListener('conversation-deleted', (e) => {
        const id = e.detail && e.detail.conversationId;
        if (!id) return;
        if (id === currentConversationId) {
            currentConversationId = null;
            const messagesDiv = document.getElementById('chat-messages');
            if (messagesDiv) messagesDiv.innerHTML = '';
 const readyMsg = typeof window.t === 'function' ? window.t('chat.systemReadyMessage') : '.,.';
            addMessage('assistant', readyMsg, null, null, null, { systemReadyMessage: true });
            addAttackChainButton(null);
        }
        if (typeof loadConversationsWithGroups === 'function') {
            loadConversationsWithGroups();
        } else if (typeof loadConversations === 'function') {
            loadConversations();
        }
    });
});
