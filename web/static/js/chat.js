let currentConversationId = null;

// @ 提及相关状态
let mentionTools = [];
let mentionToolsLoaded = false;
let mentionToolsLoadingPromise = null;
let mentionSuggestionsEl = null;
let mentionFilteredTools = [];
let externalMcpNames = []; // 外部MCP名称列表
const mentionState = {
    active: false,
    startIndex: -1,
    query: '',
    selectedIndex: 0,
};

// 输入框草稿保存相关
const DRAFT_STORAGE_KEY = 'cyberstrike-chat-draft';
let draftSaveTimer = null;
const DRAFT_SAVE_DELAY = 500; // 500ms防抖延迟

// 保存输入框草稿到localStorage（防抖版本）
function saveChatDraftDebounced(content) {
    // 清除之前的定时器
    if (draftSaveTimer) {
        clearTimeout(draftSaveTimer);
    }
    
    // 设置新的定时器
    draftSaveTimer = setTimeout(() => {
        saveChatDraft(content);
    }, DRAFT_SAVE_DELAY);
}

// 保存输入框草稿到localStorage
function saveChatDraft(content) {
    try {
        if (content && content.trim().length > 0) {
            localStorage.setItem(DRAFT_STORAGE_KEY, content);
        } else {
            // 如果内容为空，清除保存的草稿
            localStorage.removeItem(DRAFT_STORAGE_KEY);
        }
    } catch (error) {
        // localStorage可能已满或不可用，静默失败
        console.warn('保存草稿失败:', error);
    }
}

// 从localStorage恢复输入框草稿
function restoreChatDraft() {
    try {
        const chatInput = document.getElementById('chat-input');
        if (!chatInput) {
            return;
        }
        
        // 如果输入框已有内容，不恢复草稿（避免覆盖用户输入）
        if (chatInput.value && chatInput.value.trim().length > 0) {
            return;
        }
        
        const draft = localStorage.getItem(DRAFT_STORAGE_KEY);
        if (draft && draft.trim().length > 0) {
            chatInput.value = draft;
            // 调整输入框高度以适应内容
            adjustTextareaHeight(chatInput);
        }
    } catch (error) {
        console.warn('恢复草稿失败:', error);
    }
}

// 清除保存的草稿
function clearChatDraft() {
    try {
        // 同步清除，确保立即生效
        localStorage.removeItem(DRAFT_STORAGE_KEY);
    } catch (error) {
        console.warn('清除草稿失败:', error);
    }
}

// 调整textarea高度以适应内容
function adjustTextareaHeight(textarea) {
    if (!textarea) return;
    
    // 重置高度以获取准确的scrollHeight
    textarea.style.height = '44px';
    
    // 计算新高度（最小44px，最大不超过300px）
    const scrollHeight = textarea.scrollHeight;
    const newHeight = Math.min(Math.max(scrollHeight, 44), 300);
    textarea.style.height = newHeight + 'px';
}

// 发送消息
async function sendMessage() {
    const input = document.getElementById('chat-input');
    const message = input.value.trim();
    
    if (!message) {
        return;
    }
    
    // 显示用户消息
    addMessage('user', message);
    
    // 清除防抖定时器，防止在清空输入框后重新保存草稿
    if (draftSaveTimer) {
        clearTimeout(draftSaveTimer);
        draftSaveTimer = null;
    }
    
    // 立即清除草稿，防止页面刷新时恢复
    clearChatDraft();
    // 使用同步方式确保草稿被清除
    try {
        localStorage.removeItem(DRAFT_STORAGE_KEY);
    } catch (e) {
        // 忽略错误
    }
    
    // 立即清空输入框并清除草稿（在发送请求之前）
    input.value = '';
    // 强制重置输入框高度为初始高度（44px）
    input.style.height = '44px';
    
    // 创建进度消息容器（使用详细的进度展示）
    const progressId = addProgressMessage();
    const progressElement = document.getElementById(progressId);
    registerProgressTask(progressId, currentConversationId);
    loadActiveTasks();
    let assistantMessageId = null;
    let mcpExecutionIds = [];
    
    try {
        const response = await apiFetch('/api/agent-loop/stream', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ 
                message: message,
                conversationId: currentConversationId 
            }),
        });
        
        if (!response.ok) {
            throw new Error('请求失败: ' + response.status);
        }
        
        const reader = response.body.getReader();
        const decoder = new TextDecoder();
        let buffer = '';
        
        while (true) {
            const { done, value } = await reader.read();
            if (done) break;
            
            buffer += decoder.decode(value, { stream: true });
            const lines = buffer.split('\n');
            buffer = lines.pop(); // 保留最后一个不完整的行
            
            for (const line of lines) {
                if (line.startsWith('data: ')) {
                    try {
                        const eventData = JSON.parse(line.slice(6));
                        handleStreamEvent(eventData, progressElement, progressId, 
                                         () => assistantMessageId, (id) => { assistantMessageId = id; },
                                         () => mcpExecutionIds, (ids) => { mcpExecutionIds = ids; });
                    } catch (e) {
                        console.error('解析事件数据失败:', e, line);
                    }
                }
            }
        }
        
        // 处理剩余的buffer
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
                        console.error('解析事件数据失败:', e, line);
                    }
                }
            }
        }
        
        // 消息发送成功后，再次确保草稿被清除
        clearChatDraft();
        try {
            localStorage.removeItem(DRAFT_STORAGE_KEY);
        } catch (e) {
            // 忽略错误
        }
        
    } catch (error) {
        removeMessage(progressId);
        addMessage('system', '错误: ' + error.message);
        // 发送失败时，不恢复草稿，因为消息已经显示在对话框中了
    }
}

function setupMentionSupport() {
    mentionSuggestionsEl = document.getElementById('mention-suggestions');
    if (mentionSuggestionsEl) {
        mentionSuggestionsEl.style.display = 'none';
        mentionSuggestionsEl.addEventListener('mousedown', (event) => {
            // 防止点击候选项时输入框失焦
            event.preventDefault();
        });
    }
    ensureMentionToolsLoaded().catch(() => {
        // 忽略加载错误，稍后可重试
    });
}

// 刷新工具列表（重置已加载状态，强制重新加载）
function refreshMentionTools() {
    mentionToolsLoaded = false;
    mentionTools = [];
    externalMcpNames = [];
    mentionToolsLoadingPromise = null;
    // 如果当前正在使用@功能，立即触发重新加载
    if (mentionState.active) {
        ensureMentionToolsLoaded().catch(() => {
            // 忽略加载错误
        });
    }
}

// 将刷新函数暴露到window对象，供其他模块调用
if (typeof window !== 'undefined') {
    window.refreshMentionTools = refreshMentionTools;
}

function ensureMentionToolsLoaded() {
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

// 生成工具的唯一标识符，用于区分同名但来源不同的工具
function getToolKeyForMention(tool) {
    // 如果是外部工具，使用 external_mcp::tool.name 作为唯一标识
    // 如果是内部工具，使用 tool.name 作为标识
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
        // 同时获取外部MCP列表
        try {
            const mcpResponse = await apiFetch('/api/external-mcp');
            if (mcpResponse.ok) {
                const mcpData = await mcpResponse.json();
                externalMcpNames = Object.keys(mcpData.servers || {}).filter(name => {
                    const server = mcpData.servers[name];
                    // 只包含已连接且已启用的MCP
                    return server.status === 'connected' && 
                           (server.config.external_mcp_enable || (server.config.enabled && !server.config.disabled));
                });
            }
        } catch (mcpError) {
            console.warn('加载外部MCP列表失败:', mcpError);
            externalMcpNames = [];
        }

        while (page <= totalPages && page <= 20) {
            const response = await apiFetch(`/api/config/tools?page=${page}&page_size=${pageSize}`);
            if (!response.ok) {
                break;
            }
            const result = await response.json();
            const tools = Array.isArray(result.tools) ? result.tools : [];
            tools.forEach(tool => {
                if (!tool || !tool.name) {
                    return;
                }
                // 使用唯一标识符来去重，而不是只使用工具名称
                const toolKey = getToolKeyForMention(tool);
                if (seen.has(toolKey)) {
                    return;
                }
                seen.add(toolKey);
                collected.push({
                    name: tool.name,
                    description: tool.description || '',
                    enabled: tool.enabled !== false,
                    isExternal: !!tool.is_external,
                    externalMcp: tool.external_mcp || '',
                    toolKey: toolKey, // 保存唯一标识符
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
        console.warn('加载工具列表失败，@提及功能可能不可用:', error);
    }
    return mentionTools;
}

function handleChatInputInput(event) {
    const textarea = event.target;
    updateMentionStateFromInput(textarea);
    // 自动调整输入框高度
    adjustTextareaHeight(textarea);
    // 保存输入内容到localStorage（防抖）
    saveChatDraftDebounced(textarea.value);
}

function handleChatInputClick(event) {
    updateMentionStateFromInput(event.target);
}

function handleChatInputKeydown(event) {
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

    // 限制触发字符之前必须是空白或起始位置
    if (atIndex > 0) {
        const boundaryChar = textBefore[atIndex - 1];
        if (boundaryChar && !/\s/.test(boundaryChar) && !'([{，。,.;:!?'.includes(boundaryChar)) {
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
        // 检查是否精确匹配外部MCP名称
        const exactMatchedMcp = externalMcpNames.find(mcpName => 
            mcpName.toLowerCase() === normalizedQuery
        );

        if (exactMatchedMcp) {
            // 如果完全匹配MCP名称，只显示该MCP下的所有工具
            filtered = mentionTools.filter(tool => {
                return tool.externalMcp && tool.externalMcp.toLowerCase() === exactMatchedMcp.toLowerCase();
            });
        } else {
            // 检查是否部分匹配MCP名称
            const partialMatchedMcps = externalMcpNames.filter(mcpName => 
                mcpName.toLowerCase().includes(normalizedQuery)
            );
            
            // 正常匹配：按工具名称和描述过滤，同时也匹配MCP名称
            filtered = mentionTools.filter(tool => {
                const nameMatch = tool.name.toLowerCase().includes(normalizedQuery);
                const descMatch = tool.description && tool.description.toLowerCase().includes(normalizedQuery);
                const mcpMatch = tool.externalMcp && tool.externalMcp.toLowerCase().includes(normalizedQuery);
                
                // 如果部分匹配到MCP名称，也包含该MCP下的所有工具
                const mcpPartialMatch = partialMatchedMcps.some(mcpName => 
                    tool.externalMcp && tool.externalMcp.toLowerCase() === mcpName.toLowerCase()
                );
                
                return nameMatch || descMatch || mcpMatch || mcpPartialMatch;
            });
        }
    }

    filtered = filtered.slice().sort((a, b) => {
        if (normalizedQuery) {
            // 精确匹配MCP名称的工具优先显示
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
        if (a.enabled !== b.enabled) {
            return a.enabled ? -1 : 1;
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
        mentionSuggestionsEl.innerHTML = '<div class="mention-empty">正在加载工具...</div>';
        mentionSuggestionsEl.style.display = 'block';
        delete mentionSuggestionsEl.dataset.lastMentionQuery;
        return;
    }

    if (!mentionFilteredTools.length) {
        mentionSuggestionsEl.innerHTML = '<div class="mention-empty">没有匹配的工具</div>';
        mentionSuggestionsEl.style.display = 'block';
        mentionSuggestionsEl.dataset.lastMentionQuery = currentQuery;
        return;
    }

    const itemsHtml = mentionFilteredTools.map((tool, index) => {
        const activeClass = index === mentionState.selectedIndex ? 'active' : '';
        const disabledClass = tool.enabled ? '' : 'disabled';
        const badge = tool.isExternal ? '<span class="mention-item-badge">外部</span>' : '<span class="mention-item-badge internal">内置</span>';
        const nameHtml = escapeHtml(tool.name);
        const description = tool.description && tool.description.length > 0 ? escapeHtml(tool.description) : '暂无描述';
        const descHtml = `<div class="mention-item-desc">${description}</div>`;
        const statusLabel = tool.enabled ? '可用' : '已禁用';
        const statusClass = tool.enabled ? 'enabled' : 'disabled';
        const originLabel = tool.isExternal
            ? (tool.externalMcp ? `来源：${escapeHtml(tool.externalMcp)}` : '来源：外部MCP')
            : '来源：内置工具';

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
    
    // 调整输入框高度并保存草稿
    adjustTextareaHeight(textarea);
    saveChatDraftDebounced(textarea.value);

    deactivateMentionState();
}

function initializeChatUI() {
    const chatInputEl = document.getElementById('chat-input');
    if (chatInputEl) {
        chatInputEl.style.height = '44px';
        // 恢复保存的草稿（仅在输入框为空时恢复，避免覆盖用户输入）
        if (!chatInputEl.value || chatInputEl.value.trim() === '') {
            // 检查对话中是否有最近的消息（30秒内），如果有，说明可能是刚刚发送的消息，不恢复草稿
            const messagesDiv = document.getElementById('chat-messages');
            let shouldRestoreDraft = true;
            if (messagesDiv && messagesDiv.children.length > 0) {
                // 检查最后一条消息的时间
                const lastMessage = messagesDiv.lastElementChild;
                if (lastMessage) {
                    const timeDiv = lastMessage.querySelector('.message-time');
                    if (timeDiv && timeDiv.textContent) {
                        // 如果最后一条消息是用户消息，且时间很近，不恢复草稿
                        const isUserMessage = lastMessage.classList.contains('user');
                        if (isUserMessage) {
                            // 检查消息时间，如果是最近30秒内的，不恢复草稿
                            const now = new Date();
                            const messageTimeText = timeDiv.textContent;
                            // 简单检查：如果消息时间显示的是当前时间（格式：HH:MM），且是用户消息，不恢复草稿
                            // 更精确的方法是检查消息的创建时间，但需要从消息元素中获取
                            // 这里采用简单策略：如果最后一条是用户消息，且输入框为空，可能是刚发送的，不恢复草稿
                            shouldRestoreDraft = false;
                        }
                    }
                }
            }
            if (shouldRestoreDraft) {
                restoreChatDraft();
            } else {
                // 即使不恢复草稿，也要清除localStorage中的草稿，避免下次误恢复
                clearChatDraft();
            }
        }
    }

    const messagesDiv = document.getElementById('chat-messages');
    if (messagesDiv && messagesDiv.childElementCount === 0) {
        addMessage('assistant', '系统已就绪。请输入您的测试需求，系统将自动执行相应的安全测试。');
    }

    addAttackChainButton(currentConversationId);
    loadActiveTasks(true);
    if (activeTaskInterval) {
        clearInterval(activeTaskInterval);
    }
    activeTaskInterval = setInterval(() => loadActiveTasks(), ACTIVE_TASK_REFRESH_INTERVAL);
    setupMentionSupport();
}

// 消息计数器，确保ID唯一
let messageCounter = 0;

// 添加消息
function addMessage(role, content, mcpExecutionIds = null, progressId = null, createdAt = null) {
    const messagesDiv = document.getElementById('chat-messages');
    const messageDiv = document.createElement('div');
    messageCounter++;
    const id = 'msg-' + Date.now() + '-' + messageCounter + '-' + Math.random().toString(36).substr(2, 9);
    messageDiv.id = id;
    messageDiv.className = 'message ' + role;
    
    // 创建头像
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
    
    // 创建消息内容容器
    const contentWrapper = document.createElement('div');
    contentWrapper.className = 'message-content';
    
    // 创建消息气泡
    const bubble = document.createElement('div');
    bubble.className = 'message-bubble';
    
    // 解析 Markdown 或 HTML 格式
    let formattedContent;
    const defaultSanitizeConfig = {
        ALLOWED_TAGS: ['p', 'br', 'strong', 'em', 'u', 's', 'code', 'pre', 'blockquote', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'ul', 'ol', 'li', 'a', 'img', 'table', 'thead', 'tbody', 'tr', 'th', 'td', 'hr'],
        ALLOWED_ATTR: ['href', 'title', 'alt', 'src', 'class'],
        ALLOW_DATA_ATTR: false,
    };
    
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
            console.error('Markdown 解析失败:', e);
            return null;
        }
    };
    
    // 对于用户消息，直接转义HTML，不进行Markdown解析，以保留所有特殊字符
    if (role === 'user') {
        formattedContent = escapeHtml(content).replace(/\n/g, '<br>');
    } else if (typeof DOMPurify !== 'undefined') {
        let parsedContent = parseMarkdown(content);
        if (!parsedContent) {
            // 如果 Markdown 解析失败或 marked 不可用，则退回原始内容
            parsedContent = content;
        }
        formattedContent = DOMPurify.sanitize(parsedContent, defaultSanitizeConfig);
    } else if (typeof marked !== 'undefined') {
        const parsedContent = parseMarkdown(content);
        if (parsedContent) {
            formattedContent = parsedContent;
        } else {
            formattedContent = escapeHtml(content).replace(/\n/g, '<br>');
        }
    } else {
        formattedContent = escapeHtml(content).replace(/\n/g, '<br>');
    }
    
    bubble.innerHTML = formattedContent;
    contentWrapper.appendChild(bubble);
    
    // 添加时间戳
    const timeDiv = document.createElement('div');
    timeDiv.className = 'message-time';
    // 如果有传入的创建时间，使用它；否则使用当前时间
    let messageTime;
    if (createdAt) {
        // 处理字符串或Date对象
        if (typeof createdAt === 'string') {
            messageTime = new Date(createdAt);
        } else if (createdAt instanceof Date) {
            messageTime = createdAt;
        } else {
            messageTime = new Date(createdAt);
        }
        // 如果解析失败，使用当前时间
        if (isNaN(messageTime.getTime())) {
            messageTime = new Date();
        }
    } else {
        messageTime = new Date();
    }
    timeDiv.textContent = messageTime.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' });
    contentWrapper.appendChild(timeDiv);
    
    // 如果有MCP执行ID或进度ID，添加查看详情区域（统一使用"渗透测试详情"样式）
    if (role === 'assistant' && ((mcpExecutionIds && Array.isArray(mcpExecutionIds) && mcpExecutionIds.length > 0) || progressId)) {
        const mcpSection = document.createElement('div');
        mcpSection.className = 'mcp-call-section';
        
        const mcpLabel = document.createElement('div');
        mcpLabel.className = 'mcp-call-label';
        mcpLabel.textContent = '📋 渗透测试详情';
        mcpSection.appendChild(mcpLabel);
        
        const buttonsContainer = document.createElement('div');
        buttonsContainer.className = 'mcp-call-buttons';
        
        // 如果有MCP执行ID，添加MCP调用详情按钮
        if (mcpExecutionIds && Array.isArray(mcpExecutionIds) && mcpExecutionIds.length > 0) {
            mcpExecutionIds.forEach((execId, index) => {
                const detailBtn = document.createElement('button');
                detailBtn.className = 'mcp-detail-btn';
                detailBtn.innerHTML = `<span>调用 #${index + 1}</span>`;
                detailBtn.onclick = () => showMCPDetail(execId);
                buttonsContainer.appendChild(detailBtn);
            });
        }
        
        // 如果有进度ID，添加展开详情按钮（统一使用"展开详情"文本）
        if (progressId) {
            const progressDetailBtn = document.createElement('button');
            progressDetailBtn.className = 'mcp-detail-btn process-detail-btn';
            progressDetailBtn.innerHTML = '<span>展开详情</span>';
            progressDetailBtn.onclick = () => toggleProcessDetails(progressId, messageDiv.id);
            buttonsContainer.appendChild(progressDetailBtn);
            // 存储进度ID到消息元素
            messageDiv.dataset.progressId = progressId;
        }
        
        mcpSection.appendChild(buttonsContainer);
        contentWrapper.appendChild(mcpSection);
    }
    
    messageDiv.appendChild(contentWrapper);
    messagesDiv.appendChild(messageDiv);
    messagesDiv.scrollTop = messagesDiv.scrollHeight;
    return id;
}

// 渲染过程详情
function renderProcessDetails(messageId, processDetails) {
    const messageElement = document.getElementById(messageId);
    if (!messageElement) {
        return;
    }
    
    // 查找或创建MCP调用区域
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
    
    // 确保有标签和按钮容器（统一结构）
    let mcpLabel = mcpSection.querySelector('.mcp-call-label');
    let buttonsContainer = mcpSection.querySelector('.mcp-call-buttons');
    
    // 如果没有标签，创建一个（当没有工具调用时）
    if (!mcpLabel && !buttonsContainer) {
        mcpLabel = document.createElement('div');
        mcpLabel.className = 'mcp-call-label';
        mcpLabel.textContent = '📋 渗透测试详情';
        mcpSection.appendChild(mcpLabel);
    } else if (mcpLabel && mcpLabel.textContent !== '📋 渗透测试详情') {
        // 如果标签存在但不是统一格式，更新它
        mcpLabel.textContent = '📋 渗透测试详情';
    }
    
    // 如果没有按钮容器，创建一个
    if (!buttonsContainer) {
        buttonsContainer = document.createElement('div');
        buttonsContainer.className = 'mcp-call-buttons';
        mcpSection.appendChild(buttonsContainer);
    }
    
    // 添加过程详情按钮（如果还没有）
    let processDetailBtn = buttonsContainer.querySelector('.process-detail-btn');
    if (!processDetailBtn) {
        processDetailBtn = document.createElement('button');
        processDetailBtn.className = 'mcp-detail-btn process-detail-btn';
        processDetailBtn.innerHTML = '<span>展开详情</span>';
        processDetailBtn.onclick = () => toggleProcessDetails(null, messageId);
        buttonsContainer.appendChild(processDetailBtn);
    }
    
    // 创建过程详情容器（放在按钮容器之后）
    const detailsId = 'process-details-' + messageId;
    let detailsContainer = document.getElementById(detailsId);
    
    if (!detailsContainer) {
        detailsContainer = document.createElement('div');
        detailsContainer.id = detailsId;
        detailsContainer.className = 'process-details-container';
        // 确保容器在按钮容器之后
        if (buttonsContainer.nextSibling) {
            mcpSection.insertBefore(detailsContainer, buttonsContainer.nextSibling);
        } else {
            mcpSection.appendChild(detailsContainer);
        }
    }
    
    // 创建时间线（即使没有processDetails也要创建，以便展开详情按钮能正常工作）
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
    
    // 如果没有processDetails或为空，显示空状态
    if (!processDetails || processDetails.length === 0) {
        // 显示空状态提示
        timeline.innerHTML = '<div class="progress-timeline-empty">暂无过程详情（可能执行过快或未触发详细事件）</div>';
        // 默认折叠
        timeline.classList.remove('expanded');
        return;
    }
    
    // 清空时间线并重新渲染
    timeline.innerHTML = '';
    
    
    // 渲染每个过程详情事件
    processDetails.forEach(detail => {
        const eventType = detail.eventType || '';
        const title = detail.message || '';
        const data = detail.data || {};
        
        // 根据事件类型渲染不同的内容
        let itemTitle = title;
        if (eventType === 'iteration') {
            itemTitle = `第 ${data.iteration || 1} 轮迭代`;
        } else if (eventType === 'thinking') {
            itemTitle = '🤔 AI思考';
        } else if (eventType === 'tool_calls_detected') {
            itemTitle = `🔧 检测到 ${data.count || 0} 个工具调用`;
        } else if (eventType === 'tool_call') {
            const toolName = data.toolName || '未知工具';
            const index = data.index || 0;
            const total = data.total || 0;
            itemTitle = `🔧 调用工具: ${escapeHtml(toolName)} (${index}/${total})`;
        } else if (eventType === 'tool_result') {
            const toolName = data.toolName || '未知工具';
            const success = data.success !== false;
            const statusIcon = success ? '✅' : '❌';
            itemTitle = `${statusIcon} 工具 ${escapeHtml(toolName)} 执行${success ? '完成' : '失败'}`;
            
            // 如果是知识检索工具，添加特殊标记
            if (toolName === 'search_knowledge_base' && success) {
                itemTitle = `📚 ${itemTitle} - 知识检索`;
            }
        } else if (eventType === 'knowledge_retrieval') {
            itemTitle = '📚 知识检索';
        } else if (eventType === 'error') {
            itemTitle = '❌ 错误';
        } else if (eventType === 'cancelled') {
            itemTitle = '⛔ 任务已取消';
        }
        
        addTimelineItem(timeline, eventType, {
            title: itemTitle,
            message: detail.message || '',
            data: data
        });
    });
    
    // 检查是否有错误或取消事件，如果有，确保详情默认折叠
    const hasErrorOrCancelled = processDetails.some(d => 
        d.eventType === 'error' || d.eventType === 'cancelled'
    );
    if (hasErrorOrCancelled) {
        // 确保时间线是折叠的
        timeline.classList.remove('expanded');
        // 更新按钮文本为"展开详情"
        const processDetailBtn = messageElement.querySelector('.process-detail-btn');
        if (processDetailBtn) {
            processDetailBtn.innerHTML = '<span>展开详情</span>';
        }
    }
}

// 移除消息
function removeMessage(id) {
    const messageDiv = document.getElementById(id);
    if (messageDiv) {
        messageDiv.remove();
    }
}

// 输入框事件绑定（回车发送 / @提及）
const chatInput = document.getElementById('chat-input');
if (chatInput) {
    chatInput.addEventListener('keydown', handleChatInputKeydown);
    chatInput.addEventListener('input', handleChatInputInput);
    chatInput.addEventListener('click', handleChatInputClick);
    chatInput.addEventListener('focus', handleChatInputClick);
    chatInput.addEventListener('blur', () => {
        setTimeout(() => {
            if (!chatInput.matches(':focus')) {
                deactivateMentionState();
            }
        }, 120);
        // 失焦时立即保存草稿（不等待防抖）
        if (chatInput.value) {
            saveChatDraft(chatInput.value);
        }
    });
}

// 页面卸载时立即保存草稿
window.addEventListener('beforeunload', () => {
    const chatInput = document.getElementById('chat-input');
    if (chatInput && chatInput.value) {
        // 立即保存，不使用防抖
        saveChatDraft(chatInput.value);
    }
});

// 显示MCP调用详情
async function showMCPDetail(executionId) {
    try {
        const response = await apiFetch(`/api/monitor/execution/${executionId}`);
        const exec = await response.json();
        
        if (response.ok) {
            // 填充模态框内容
            document.getElementById('detail-tool-name').textContent = exec.toolName || 'Unknown';
            document.getElementById('detail-execution-id').textContent = exec.id || 'N/A';
            const statusEl = document.getElementById('detail-status');
            const normalizedStatus = (exec.status || 'unknown').toLowerCase();
            statusEl.textContent = getStatusText(exec.status);
            statusEl.className = `status-chip status-${normalizedStatus}`;
            document.getElementById('detail-time').textContent = exec.startTime
                ? new Date(exec.startTime).toLocaleString('zh-CN')
                : '—';
            
            // 请求参数
            const requestData = {
                tool: exec.toolName,
                arguments: exec.arguments
            };
            document.getElementById('detail-request').textContent = JSON.stringify(requestData, null, 2);
            
            // 响应结果 + 正确信息 / 错误信息
            const responseElement = document.getElementById('detail-response');
            const successSection = document.getElementById('detail-success-section');
            const successElement = document.getElementById('detail-success');
            const errorSection = document.getElementById('detail-error-section');
            const errorElement = document.getElementById('detail-error');

            // 重置状态
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
                    // 错误场景：响应结果标红 + 错误信息区块
                    responseElement.className = 'code-block error';
                    if (exec.error && errorSection && errorElement) {
                        errorSection.style.display = 'block';
                        errorElement.textContent = exec.error;
                    }
                } else {
                    // 成功场景：响应结果保持普通样式，正确信息单独拎出来
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
                            successText = '执行成功，未返回可展示的文本内容。';
                        }
                        successElement.textContent = successText;
                    }
                }
            } else {
                responseElement.textContent = '暂无响应数据';
            }
            
            // 显示模态框
            document.getElementById('mcp-detail-modal').style.display = 'block';
        } else {
            alert('获取详情失败: ' + (exec.error || '未知错误'));
        }
    } catch (error) {
        alert('获取详情失败: ' + error.message);
    }
}

// 关闭MCP详情模态框
function closeMCPDetail() {
    document.getElementById('mcp-detail-modal').style.display = 'none';
}

// 复制详情面板中的内容
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
        triggerBtn.textContent = '已复制';
        triggerBtn.disabled = true;
        setTimeout(() => {
            triggerBtn.disabled = false;
            triggerBtn.textContent = triggerBtn.dataset.originalLabel || originalLabel || '复制';
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
                triggerBtn.textContent = triggerBtn.dataset.originalLabel || originalLabel || '复制';
            }
            alert('复制失败，请手动选择文本复制。');
        });
}


// 开始新对话
async function startNewConversation() {
    // 如果当前在分组详情页面，先退出分组详情
    if (currentGroupId) {
        const groupDetailPage = document.getElementById('group-detail-page');
        const chatContainer = document.querySelector('.chat-container');
        if (groupDetailPage) groupDetailPage.style.display = 'none';
        if (chatContainer) chatContainer.style.display = 'flex';
        currentGroupId = null;
        // 刷新对话列表
        loadConversationsWithGroups();
    }
    
    currentConversationId = null;
    currentConversationGroupId = null; // 新对话不属于任何分组
    document.getElementById('chat-messages').innerHTML = '';
    addMessage('assistant', '系统已就绪。请输入您的测试需求，系统将自动执行相应的安全测试。');
    addAttackChainButton(null);
    updateActiveConversation();
    // 刷新分组列表，清除分组高亮
    await loadGroups();
    // 刷新对话列表，确保显示最新的历史对话
    loadConversationsWithGroups();
    // 清除防抖定时器，防止恢复草稿时触发保存
    if (draftSaveTimer) {
        clearTimeout(draftSaveTimer);
        draftSaveTimer = null;
    }
    // 清除草稿，新对话不应该恢复之前的草稿
    clearChatDraft();
    // 清空输入框
    const chatInput = document.getElementById('chat-input');
    if (chatInput) {
        chatInput.value = '';
        chatInput.style.height = '44px';
    }
}

// 加载对话列表（按时间分组）
async function loadConversations(searchQuery = '') {
    try {
        let url = '/api/conversations?limit=50';
        if (searchQuery && searchQuery.trim()) {
            url += '&search=' + encodeURIComponent(searchQuery.trim());
        }
        const response = await apiFetch(url);
        const conversations = await response.json();

        const listContainer = document.getElementById('conversations-list');
        if (!listContainer) {
            return;
        }

        const emptyStateHtml = '<div style="padding: 20px; text-align: center; color: var(--text-muted); font-size: 0.875rem;">暂无历史对话</div>';
        listContainer.innerHTML = '';

        if (!Array.isArray(conversations) || conversations.length === 0) {
            listContainer.innerHTML = emptyStateHtml;
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
            { key: 'today', label: '今天' },
            { key: 'yesterday', label: '昨天' },
            { key: 'thisWeek', label: '本周' },
            { key: 'earlier', label: '更早' },
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
                // 判断是否置顶
                const isPinned = itemData.pinned || false;
                section.appendChild(createConversationListItemWithMenu(itemData, isPinned));
            });

            fragment.appendChild(section);
        });

        if (!rendered) {
            listContainer.innerHTML = emptyStateHtml;
            return;
        }

        listContainer.appendChild(fragment);
        updateActiveConversation();
    } catch (error) {
        console.error('加载对话列表失败:', error);
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
    const titleText = conversation.title || '未命名对话';
    title.textContent = safeTruncateText(titleText, 60);
    title.title = titleText; // 设置完整标题以便悬停查看
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
    deleteBtn.title = '删除对话';
    deleteBtn.onclick = (e) => {
        e.stopPropagation();
        deleteConversation(conversation.id);
    };
    item.appendChild(deleteBtn);

    item.onclick = () => loadConversation(conversation.id);
    return item;
}

// 处理历史记录搜索
let conversationSearchTimer = null;
function handleConversationSearch(query) {
    // 防抖处理，避免频繁请求
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
    }, 300); // 300ms防抖延迟
}

// 清除搜索
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
    // 如果没有传入 todayStart，使用当前日期作为参考
    const now = new Date();
    const referenceToday = todayStart || new Date(now.getFullYear(), now.getMonth(), now.getDate());
    const referenceYesterday = yesterdayStart || new Date(referenceToday.getTime() - 24 * 60 * 60 * 1000);
    const messageDate = new Date(dateObj.getFullYear(), dateObj.getMonth(), dateObj.getDate());

    if (messageDate.getTime() === referenceToday.getTime()) {
        return dateObj.toLocaleTimeString('zh-CN', {
            hour: '2-digit',
            minute: '2-digit'
        });
    }
    if (messageDate.getTime() === referenceYesterday.getTime()) {
        return '昨天 ' + dateObj.toLocaleTimeString('zh-CN', {
            hour: '2-digit',
            minute: '2-digit'
        });
    }
    if (dateObj.getFullYear() === referenceToday.getFullYear()) {
        return dateObj.toLocaleString('zh-CN', {
            month: 'short',
            day: 'numeric',
            hour: '2-digit',
            minute: '2-digit'
        });
    }
    return dateObj.toLocaleString('zh-CN', {
        year: 'numeric',
        month: 'short',
        day: 'numeric',
        hour: '2-digit',
        minute: '2-digit'
    });
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

// 加载对话
async function loadConversation(conversationId) {
    try {
        const response = await apiFetch(`/api/conversations/${conversationId}`);
        const conversation = await response.json();
        
        if (!response.ok) {
            alert('加载对话失败: ' + (conversation.error || '未知错误'));
            return;
        }
        
        // 如果当前在分组详情页面，切换到对话界面
        // 退出分组详情模式，显示所有最近对话，提供更好的用户体验
        if (currentGroupId) {
            const sidebar = document.querySelector('.conversation-sidebar');
            const groupDetailPage = document.getElementById('group-detail-page');
            const chatContainer = document.querySelector('.chat-container');
            
            // 确保侧边栏始终可见
            if (sidebar) sidebar.style.display = 'flex';
            // 隐藏分组详情页，显示对话界面
            if (groupDetailPage) groupDetailPage.style.display = 'none';
            if (chatContainer) chatContainer.style.display = 'flex';
            
            // 退出分组详情模式，这样最近对话列表会显示所有对话
            // 用户可以在侧边栏看到所有对话，方便切换
            const previousGroupId = currentGroupId;
            currentGroupId = null;
            
            // 刷新最近对话列表，显示所有对话（包括分组中的）
            loadConversationsWithGroups();
        }
        
        // 获取当前对话所属的分组ID（用于高亮显示）
        // 确保分组映射已加载
        if (Object.keys(conversationGroupMappingCache).length === 0) {
            await loadConversationGroupMapping();
        }
        currentConversationGroupId = conversationGroupMappingCache[conversationId] || null;
        
        // 无论是否在分组详情页面，都刷新分组列表，确保高亮状态正确
        // 这样可以清除之前分组的高亮状态，确保UI状态一致
        await loadGroups();
        
        // 更新当前对话ID
        currentConversationId = conversationId;
        updateActiveConversation();
        
        // 如果攻击链模态框打开且显示的不是当前对话，关闭它
        const attackChainModal = document.getElementById('attack-chain-modal');
        if (attackChainModal && attackChainModal.style.display === 'block') {
            if (currentAttackChainConversationId !== conversationId) {
                closeAttackChainModal();
            }
        }
        
        // 清空消息区域
        const messagesDiv = document.getElementById('chat-messages');
        messagesDiv.innerHTML = '';
        
        // 检查对话中是否有最近的消息，如果有，清除草稿（避免恢复已发送的消息）
        let hasRecentUserMessage = false;
        if (conversation.messages && conversation.messages.length > 0) {
            const lastMessage = conversation.messages[conversation.messages.length - 1];
            if (lastMessage && lastMessage.role === 'user') {
                // 检查消息时间，如果是最近30秒内的，清除草稿
                const messageTime = new Date(lastMessage.createdAt);
                const now = new Date();
                const timeDiff = now.getTime() - messageTime.getTime();
                if (timeDiff < 30000) { // 30秒内
                    hasRecentUserMessage = true;
                }
            }
        }
        if (hasRecentUserMessage) {
            // 如果有最近发送的用户消息，清除草稿
            clearChatDraft();
            const chatInput = document.getElementById('chat-input');
            if (chatInput) {
                chatInput.value = '';
                adjustTextareaHeight(chatInput);
            }
        }
        
        // 加载消息
        if (conversation.messages && conversation.messages.length > 0) {
            conversation.messages.forEach(msg => {
                // 检查消息内容是否为"处理中..."，如果是，检查processDetails中是否有错误或取消事件
                let displayContent = msg.content;
                if (msg.role === 'assistant' && msg.content === '处理中...' && msg.processDetails && msg.processDetails.length > 0) {
                    // 查找最后一个error或cancelled事件
                    for (let i = msg.processDetails.length - 1; i >= 0; i--) {
                        const detail = msg.processDetails[i];
                        if (detail.eventType === 'error' || detail.eventType === 'cancelled') {
                            displayContent = detail.message || msg.content;
                            break;
                        }
                    }
                }
                
                // 传递消息的创建时间
                const messageId = addMessage(msg.role, displayContent, msg.mcpExecutionIds || [], null, msg.createdAt);
                // 对于助手消息，总是渲染过程详情（即使没有processDetails也要显示展开详情按钮）
                if (msg.role === 'assistant') {
                    // 延迟一下，确保消息已经渲染
                    setTimeout(() => {
                        renderProcessDetails(messageId, msg.processDetails || []);
                        // 如果有过程详情，检查是否有错误或取消事件，如果有，确保详情默认折叠
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
            addMessage('assistant', '系统已就绪。请输入您的测试需求，系统将自动执行相应的安全测试。');
        }
        
        // 滚动到底部
        messagesDiv.scrollTop = messagesDiv.scrollHeight;
        
        // 添加攻击链按钮
        addAttackChainButton(conversationId);
        
        // 刷新对话列表
        loadConversations();
    } catch (error) {
        console.error('加载对话失败:', error);
        alert('加载对话失败: ' + error.message);
    }
}

// 删除对话
async function deleteConversation(conversationId, skipConfirm = false) {
    // 确认删除（如果调用者没有跳过确认）
    if (!skipConfirm) {
        if (!confirm('确定要删除这个对话吗？此操作不可恢复。')) {
            return;
        }
    }
    
    try {
        const response = await apiFetch(`/api/conversations/${conversationId}`, {
            method: 'DELETE'
        });
        
        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || '删除失败');
        }
        
        // 如果删除的是当前对话，清空对话界面
        if (conversationId === currentConversationId) {
            currentConversationId = null;
            document.getElementById('chat-messages').innerHTML = '';
            addMessage('assistant', '系统已就绪。请输入您的测试需求，系统将自动执行相应的安全测试。');
            addAttackChainButton(null);
        }
        
        // 刷新对话列表
        loadConversations();
    } catch (error) {
        console.error('删除对话失败:', error);
        alert('删除对话失败: ' + error.message);
    }
}

// 更新活动对话样式
function updateActiveConversation() {
    document.querySelectorAll('.conversation-item').forEach(item => {
        item.classList.remove('active');
        if (currentConversationId && item.dataset.conversationId === currentConversationId) {
            item.classList.add('active');
        }
    });
}

// ==================== 攻击链可视化功能 ====================

let attackChainCytoscape = null;
let currentAttackChainConversationId = null;
// 按对话ID管理加载状态，实现不同对话之间的解耦
const attackChainLoadingMap = new Map(); // Map<conversationId, boolean>

// 检查指定对话是否正在加载
function isAttackChainLoading(conversationId) {
    return attackChainLoadingMap.get(conversationId) === true;
}

// 设置指定对话的加载状态
function setAttackChainLoading(conversationId, loading) {
    if (loading) {
        attackChainLoadingMap.set(conversationId, true);
    } else {
        attackChainLoadingMap.delete(conversationId);
    }
}

// 添加攻击链按钮
function addAttackChainButton(conversationId) {
    const attackChainBtn = document.getElementById('attack-chain-btn');
    const conversationHeader = document.getElementById('conversation-header');
    
    if (!attackChainBtn || !conversationHeader) {
        return;
    }

    if (conversationId) {
        // 显示会话顶部栏
        conversationHeader.style.display = 'block';
        
        const isRunning = typeof isConversationTaskRunning === 'function'
            ? isConversationTaskRunning(conversationId)
            : false;
        if (isRunning) {
            attackChainBtn.disabled = true;
            attackChainBtn.title = '当前对话正在执行，请稍后再生成攻击链';
            attackChainBtn.onclick = null;
        } else {
            attackChainBtn.disabled = false;
            attackChainBtn.title = '查看当前对话的攻击链';
            attackChainBtn.onclick = () => showAttackChain(conversationId);
        }
    } else {
        // 隐藏会话顶部栏
        conversationHeader.style.display = 'none';
        
        attackChainBtn.disabled = true;
        attackChainBtn.title = '请选择一个对话以查看攻击链';
        attackChainBtn.onclick = null;
    }
}

function updateAttackChainAvailability() {
    addAttackChainButton(currentConversationId);
}

// 显示攻击链模态框
async function showAttackChain(conversationId) {
    // 如果当前显示的对话ID不同，或者没有在加载，允许打开
    // 如果正在加载同一个对话，也允许打开（显示加载状态）
    if (isAttackChainLoading(conversationId) && currentAttackChainConversationId === conversationId) {
        // 如果模态框已经打开且显示的是同一个对话，不重复打开
        const modal = document.getElementById('attack-chain-modal');
        if (modal && modal.style.display === 'block') {
            console.log('攻击链正在加载中，模态框已打开');
            return;
        }
    }
    
    currentAttackChainConversationId = conversationId;
    const modal = document.getElementById('attack-chain-modal');
    if (!modal) {
        console.error('攻击链模态框未找到');
        return;
    }
    
    modal.style.display = 'block';
    
    // 清空容器
    const container = document.getElementById('attack-chain-container');
    if (container) {
        container.innerHTML = '<div class="loading-spinner">加载中...</div>';
    }
    
    // 隐藏详情面板
    const detailsPanel = document.getElementById('attack-chain-details');
    if (detailsPanel) {
        detailsPanel.style.display = 'none';
    }
    
    // 禁用重新生成按钮
    const regenerateBtn = document.querySelector('button[onclick="regenerateAttackChain()"]');
    if (regenerateBtn) {
        regenerateBtn.disabled = true;
        regenerateBtn.style.opacity = '0.5';
        regenerateBtn.style.cursor = 'not-allowed';
    }
    
    // 加载攻击链数据
    await loadAttackChain(conversationId);
}

// 加载攻击链数据
async function loadAttackChain(conversationId) {
    if (isAttackChainLoading(conversationId)) {
        return; // 防止重复调用
    }
    
    setAttackChainLoading(conversationId, true);
    
    try {
        const response = await apiFetch(`/api/attack-chain/${conversationId}`);
        
        if (!response.ok) {
            // 处理 409 Conflict（正在生成中）
            if (response.status === 409) {
                const error = await response.json();
                const container = document.getElementById('attack-chain-container');
                if (container) {
                    container.innerHTML = `
                        <div style="text-align: center; padding: 28px 24px; color: var(--text-secondary);">
                            <div style="display: inline-flex; align-items: center; gap: 8px; font-size: 0.95rem; color: var(--text-primary);">
                                <span role="presentation" aria-hidden="true">⏳</span>
                                <span>攻击链生成中，请稍候</span>
                            </div>
                            <button class="btn-secondary" onclick="refreshAttackChain()" style="margin-top: 12px; font-size: 0.78rem; padding: 4px 12px;">
                                刷新
                            </button>
                        </div>
                    `;
                }
                // 5秒后自动刷新（允许刷新，但保持加载状态防止重复点击）
                // 使用闭包保存 conversationId，防止串台
                setTimeout(() => {
                    // 检查当前显示的对话ID是否匹配
                    if (currentAttackChainConversationId === conversationId) {
                        refreshAttackChain();
                    }
                }, 5000);
                // 在 409 情况下，保持加载状态，防止重复点击
                // 但允许 refreshAttackChain 调用 loadAttackChain 来检查状态
                // 注意：不重置加载状态，保持加载状态
                // 恢复按钮状态（虽然保持加载状态，但允许用户手动刷新）
                const regenerateBtn = document.querySelector('button[onclick="regenerateAttackChain()"]');
                if (regenerateBtn) {
                    regenerateBtn.disabled = false;
                    regenerateBtn.style.opacity = '1';
                    regenerateBtn.style.cursor = 'pointer';
                }
                return; // 提前返回，不执行 finally 块中的 setAttackChainLoading(conversationId, false)
            }
            
            const error = await response.json();
            throw new Error(error.error || '加载攻击链失败');
        }
        
        const chainData = await response.json();
        
        // 检查当前显示的对话ID是否匹配，防止串台
        if (currentAttackChainConversationId !== conversationId) {
            console.log('攻击链数据已返回，但当前显示的对话已切换，忽略此次渲染', {
                returned: conversationId,
                current: currentAttackChainConversationId
            });
            setAttackChainLoading(conversationId, false);
            return;
        }
        
        // 渲染攻击链
        renderAttackChain(chainData);
        
        // 更新统计信息
        updateAttackChainStats(chainData);
        
        // 成功加载后，重置加载状态
        setAttackChainLoading(conversationId, false);
        
    } catch (error) {
        console.error('加载攻击链失败:', error);
        const container = document.getElementById('attack-chain-container');
        if (container) {
            container.innerHTML = `<div class="error-message">加载失败: ${error.message}</div>`;
        }
        // 错误时也重置加载状态
        setAttackChainLoading(conversationId, false);
    } finally {
        // 恢复重新生成按钮
        const regenerateBtn = document.querySelector('button[onclick="regenerateAttackChain()"]');
        if (regenerateBtn) {
            regenerateBtn.disabled = false;
            regenerateBtn.style.opacity = '1';
            regenerateBtn.style.cursor = 'pointer';
        }
    }
}

// 渲染攻击链
function renderAttackChain(chainData) {
    const container = document.getElementById('attack-chain-container');
    if (!container) {
        return;
    }
    
    // 清空容器
    container.innerHTML = '';
    
    if (!chainData.nodes || chainData.nodes.length === 0) {
        container.innerHTML = '<div class="empty-message">暂无攻击链数据</div>';
        return;
    }
    
    // 计算图的复杂度（用于动态调整布局和样式）
    const nodeCount = chainData.nodes.length;
    const edgeCount = chainData.edges.length;
    const isComplexGraph = nodeCount > 15 || edgeCount > 25;
    
    // 优化节点标签：智能截断和换行
    chainData.nodes.forEach(node => {
        if (node.label) {
            // 智能截断：优先在标点符号、空格处截断
            const maxLength = isComplexGraph ? 18 : 22;
            if (node.label.length > maxLength) {
                let truncated = node.label.substring(0, maxLength);
                // 尝试在最后一个标点符号或空格处截断
                const lastPunct = Math.max(
                    truncated.lastIndexOf('，'),
                    truncated.lastIndexOf('。'),
                    truncated.lastIndexOf('、'),
                    truncated.lastIndexOf(' '),
                    truncated.lastIndexOf('/')
                );
                if (lastPunct > maxLength * 0.6) { // 如果标点符号位置合理
                    truncated = truncated.substring(0, lastPunct + 1);
                }
                node.label = truncated + '...';
            }
        }
    });
    
    // 准备Cytoscape数据
    const elements = [];
    
    // 添加节点，并预计算文字颜色和边框颜色，同时为类型标签准备数据
    chainData.nodes.forEach(node => {
        const riskScore = node.risk_score || 0;
        const nodeType = node.type || '';
        
        // 根据节点类型设置类型标签文本和标识符（使用更现代的设计）
        let typeLabel = '';
        let typeBadge = '';
        let typeColor = '';
        if (nodeType === 'target') {
            typeLabel = '目标';
            typeBadge = '○';  // 使用空心圆，更现代
            typeColor = '#1976d2';  // 蓝色
        } else if (nodeType === 'action') {
            typeLabel = '行动';
            typeBadge = '▷';  // 使用更简洁的三角形
            typeColor = '#f57c00';  // 橙色
        } else if (nodeType === 'vulnerability') {
            typeLabel = '漏洞';
            typeBadge = '◇';  // 使用空心菱形，更精致
            typeColor = '#d32f2f';  // 红色
        } else {
            typeLabel = nodeType;
            typeBadge = '•';
            typeColor = '#666';
        }
        
        // 根据风险分数计算文字颜色和边框颜色
        let textColor, borderColor, textOutlineWidth, textOutlineColor;
        if (riskScore >= 80) {
            // 红色背景：白色文字，白色边框
            textColor = '#fff';
            borderColor = '#fff';
            textOutlineWidth = 1;
            textOutlineColor = '#333';
        } else if (riskScore >= 60) {
            // 橙色背景：白色文字，白色边框
            textColor = '#fff';
            borderColor = '#fff';
            textOutlineWidth = 1;
            textOutlineColor = '#333';
        } else if (riskScore >= 40) {
            // 黄色背景：深色文字，深色边框
            textColor = '#333';
            borderColor = '#cc9900';
            textOutlineWidth = 2;
            textOutlineColor = '#fff';
        } else {
            // 绿色背景：深绿色文字，深色边框
            textColor = '#1a5a1a';
            borderColor = '#5a8a5a';
            textOutlineWidth = 2;
            textOutlineColor = '#fff';
        }
        
        // 构建带类型标签的显示文本：使用现代极简的设计风格
        // 类型标签显示在顶部，使用简洁的格式，通过间距自然分隔
        const displayLabel = typeBadge + ' ' + typeLabel + '\n\n' + node.label;
        
        elements.push({
            data: {
                id: node.id,
                label: displayLabel,  // 使用包含类型标签的标签
                originalLabel: node.label,  // 保存原始标签用于搜索
                type: nodeType,
                typeLabel: typeLabel,  // 保存类型标签文本
                typeBadge: typeBadge,  // 保存类型标识符
                typeColor: typeColor,  // 保存类型颜色
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
    
    // 添加边（只添加源节点和目标节点都存在的边）
    const nodeIds = new Set(chainData.nodes.map(node => node.id));
    chainData.edges.forEach(edge => {
        // 验证源节点和目标节点是否存在
        if (nodeIds.has(edge.source) && nodeIds.has(edge.target)) {
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
            console.warn('跳过无效的边：源节点或目标节点不存在', {
                edgeId: edge.id,
                source: edge.source,
                target: edge.target,
                sourceExists: nodeIds.has(edge.source),
                targetExists: nodeIds.has(edge.target)
            });
        }
    });
    
    // 初始化Cytoscape
    attackChainCytoscape = cytoscape({
        container: container,
        elements: elements,
        style: [
            {
                selector: 'node',
                style: {
                    'label': 'data(label)',
                    // 增大节点尺寸，使其更加醒目和美观
                    // 根据节点类型调整大小，target节点更大（增加高度以容纳类型标签）
                    'width': function(ele) {
                        const type = ele.data('type');
                        if (type === 'target') return isComplexGraph ? 380 : 420;
                        return isComplexGraph ? 360 : 400;
                    },
                    'height': function(ele) {
                        const type = ele.data('type');
                        if (type === 'target') return isComplexGraph ? 180 : 200;
                        return isComplexGraph ? 170 : 190;
                    },
                    'shape': function(ele) {
                        // 所有节点都使用圆角矩形
                        return 'round-rectangle';
                    },
                    'background-color': function(ele) {
                        const type = ele.data('type');
                        const riskScore = ele.data('riskScore') || 0;
                        
                        // target节点使用更深的蓝色背景，增强对比度
                        if (type === 'target') {
                            return '#e8f4fd';  // 更亮的蓝色背景，提高对比度
                        }
                        
                        // action节点根据执行有效性显示不同颜色
                        if (type === 'action') {
                            const metadata = ele.data('metadata') || {};
                            const findings = metadata.findings || [];
                            const status = metadata.status || '';
                            
                            // 有效执行：有findings且status不是failed_insight
                            const hasFindings = Array.isArray(findings) && findings.length > 0;
                            const isFailedInsight = status === 'failed_insight';
                            
                            if (hasFindings && !isFailedInsight) {
                                return '#f5fbf5';  // 更亮的绿色背景，提高对比度
                            } else {
                                return '#fafafa';  // 浅灰色背景，与白色文字形成对比
                            }
                        }
                        
                        // vulnerability节点根据风险分数显示不同颜色，使用更亮的背景
                        if (type === 'vulnerability') {
                            if (riskScore >= 80) return '#fff0f0';  // 更亮的红色背景
                            if (riskScore >= 60) return '#fff5e6';  // 更亮的橙色背景
                            if (riskScore >= 40) return '#fffef0';  // 更亮的黄色背景
                            return '#f5fbf5';  // 更亮的绿色背景
                        }
                        
                        return '#ffffff';  // 默认白色背景
                    },
                    // 根据节点类型和风险分数设置文字颜色，使用更深的颜色提高对比度
                    // 注意：由于标签包含类型标签和内容，颜色适用于所有文本
                    'color': function(ele) {
                        const type = ele.data('type');
                        const riskScore = ele.data('riskScore') || 0;
                        
                        if (type === 'target') {
                            return '#0d47a1';  // 更深的蓝色文字，提高对比度
                        }
                        
                        // action节点根据执行有效性显示不同文字颜色
                        if (type === 'action') {
                            const metadata = ele.data('metadata') || {};
                            const findings = metadata.findings || [];
                            const status = metadata.status || '';
                            
                            // 有效执行：有findings且status不是failed_insight
                            const hasFindings = Array.isArray(findings) && findings.length > 0;
                            const isFailedInsight = status === 'failed_insight';
                            
                            if (hasFindings && !isFailedInsight) {
                                return '#1b5e20';  // 更深的绿色：有效执行，提高对比度
                            } else {
                                return '#212121';  // 深灰色：无效执行，提高对比度
                            }
                        }
                        
                        // vulnerability节点根据风险分数显示不同文字颜色，使用更深的颜色
                        if (type === 'vulnerability') {
                            if (riskScore >= 80) return '#b71c1c';  // 更深的红色，提高对比度
                            if (riskScore >= 60) return '#bf360c';  // 更深的橙色，提高对比度
                            if (riskScore >= 40) return '#e65100';  // 更深的黄色，提高对比度
                            return '#33691e';  // 更深的绿色，提高对比度
                        }
                        
                        return '#000000';  // 黑色，最高对比度
                    },
                    'font-size': function(ele) {
                        // 进一步增大字体，提高可读性
                        const type = ele.data('type');
                        if (type === 'target') return isComplexGraph ? '20px' : '22px';
                        return isComplexGraph ? '19px' : '21px';
                    },
                    'font-weight': 'bold',  // 加粗字体，提高可读性
                    'font-family': '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif',
                    'text-valign': 'center',
                    'text-halign': 'center',
                    'text-wrap': 'wrap',
                    'text-max-width': function(ele) {
                        const type = ele.data('type');
                        if (type === 'target') return isComplexGraph ? '340px' : '380px';
                        return isComplexGraph ? '320px' : '360px';
                    },
                    'text-overflow-wrap': 'anywhere',
                    'text-margin-y': 5,  // 调整垂直边距以适应多行文本
                    'padding': '18px',  // 进一步增加内边距，使节点内容更有呼吸感
                    'line-height': 1.6,  // 增加行高，提高可读性
                    // 根据节点类型设置边框样式，使用更粗的边框增强视觉效果
                    'border-width': function(ele) {
                        const type = ele.data('type');
                        if (type === 'target') return 6;
                        return 5;
                    },
                    'border-color': function(ele) {
                        const type = ele.data('type');
                        const riskScore = ele.data('riskScore') || 0;
                        
                        if (type === 'target') {
                            return '#1565c0';  // 更深的蓝色边框，提高对比度
                        }
                        
                        // action节点根据执行有效性显示不同边框颜色
                        if (type === 'action') {
                            const metadata = ele.data('metadata') || {};
                            const findings = metadata.findings || [];
                            const status = metadata.status || '';
                            
                            // 有效执行：有findings且status不是failed_insight
                            const hasFindings = Array.isArray(findings) && findings.length > 0;
                            const isFailedInsight = status === 'failed_insight';
                            
                            if (hasFindings && !isFailedInsight) {
                                return '#4caf50';  // 更深的绿色边框：有效执行，提高对比度
                            } else {
                                return '#757575';  // 更深的灰色边框：无效执行，提高对比度
                            }
                        }
                        
                        // vulnerability节点根据风险分数显示不同边框颜色，使用更深的颜色
                        if (type === 'vulnerability') {
                            if (riskScore >= 80) return '#b71c1c';  // 更深的红色边框，提高对比度
                            if (riskScore >= 60) return '#bf360c';  // 更深的橙色边框，提高对比度
                            if (riskScore >= 40) return '#e65100';  // 更深的黄色边框，提高对比度
                            return '#33691e';  // 更深的绿色边框，提高对比度
                        }
                        
                        return '#616161';  // 更深的默认灰色边框，提高对比度
                    },
                    'border-style': 'solid',  // 统一使用实线边框，提高可读性
                    'overlay-padding': '12px',
                    // 移除文字轮廓，使用纯色文字
                    'text-outline-width': 0,
                    'background-opacity': 1
                }
            },
            {
                selector: 'edge',
                style: {
                    'width': 'mapData(weight, 1, 5, 1.5, 3)',
                    'line-color': function(ele) {
                        const type = ele.data('type');
                        // 参考图片风格，使用不同颜色和样式
                        if (type === 'discovers') return '#42a5f5';  // 浅蓝色：action发现vulnerability
                        if (type === 'targets') return '#1976d2';  // 深蓝色：target指向action（虚线）
                        if (type === 'enables') return '#e53935';  // 红色：vulnerability间的因果关系
                        if (type === 'leads_to') return '#616161';  // 灰色：action之间的逻辑顺序
                        return '#9e9e9e';
                    },
                    'target-arrow-color': function(ele) {
                        const type = ele.data('type');
                        if (type === 'discovers') return '#42a5f5';
                        if (type === 'targets') return '#1976d2';
                        if (type === 'enables') return '#e53935';
                        if (type === 'leads_to') return '#616161';
                        return '#9e9e9e';
                    },
                    'target-arrow-shape': 'triangle',
                    // 使用bezier曲线，更美观
                    'curve-style': 'bezier',
                    'control-point-step-size': 60,  // 增加步长，让控制点分布更均匀
                    // 大幅增加控制点距离，避免多条边指向同一节点时箭头重叠
                    // 使用更大的值确保箭头之间有足够的间距
                    'control-point-distance': isComplexGraph ? 180 : 150,
                    'opacity': 0.7,
                    // 根据边类型设置线条样式：targets使用虚线，其他使用实线
                    'line-style': function(ele) {
                        const type = ele.data('type');
                        if (type === 'targets') return 'dashed';  // target相关的边使用虚线
                        return 'solid';
                    },
                    'line-dash-pattern': function(ele) {
                        const type = ele.data('type');
                        if (type === 'targets') return [8, 4];  // 虚线模式
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
    
    // 注册dagre布局（确保依赖已加载）
    let layoutName = 'breadthfirst'; // 默认布局
    let layoutOptions = {
        name: 'breadthfirst',
        directed: true,
        spacingFactor: isComplexGraph ? 3.0 : 2.5,
        padding: 40
    };
    
    if (typeof cytoscape !== 'undefined' && typeof cytoscapeDagre !== 'undefined') {
        try {
            cytoscape.use(cytoscapeDagre);
            layoutName = 'dagre';
            
            // 动态计算布局参数，基于容器尺寸和节点数量
            const containerWidth = container ? container.offsetWidth : 1200;
            const containerHeight = container ? container.offsetHeight : 800;
            
            // 计算平均节点宽度（考虑不同类型节点的平均尺寸）
            const avgNodeWidth = isComplexGraph ? 370 : 410;  // 进一步增大节点尺寸
            const avgNodeHeight = isComplexGraph ? 175 : 195;
            
            // 计算图的层级深度（估算）
            const estimatedDepth = Math.ceil(Math.log2(Math.max(nodeCount, 2))) + 1;
            
            // 动态计算节点水平间距：基于容器宽度和节点数量
            // 目标：使用容器宽度的95%，让图充分展开
            const maxLevelWidth = Math.max(1, Math.ceil(nodeCount / estimatedDepth));
            const targetGraphWidth = containerWidth * 0.95;  // 使用95%的容器宽度，让图更宽
            // 大幅增加最小间距，确保节点不重叠（考虑节点宽度和标签）
            const minNodeSep = avgNodeWidth * 1.5;  // 最小间距为节点宽度的1.5倍，确保节点之间有足够空间
            // 优化间距计算：确保即使节点很多时也有足够的间距
            const availableWidth = targetGraphWidth - avgNodeWidth * maxLevelWidth;
            const calculatedNodeSep = Math.max(
                minNodeSep,
                Math.min(
                    availableWidth / Math.max(1, maxLevelWidth - 1),
                    avgNodeWidth * 3.0  // 最大间距不超过节点宽度的3.0倍，让图更宽
                )
            );
            
            // 动态计算层级间距：基于容器高度和层级数
            // 减小垂直间距，让节点更紧凑，同时节点更大更易读
            const targetGraphHeight = containerHeight * 0.85;
            const calculatedRankSep = Math.max(
                avgNodeHeight * 1.3,  // 减小到节点高度的1.3倍，让节点更紧凑
                Math.min(
                    targetGraphHeight / Math.max(estimatedDepth - 1, 1),
                    avgNodeHeight * 2.0  // 最大不超过节点高度的2.0倍
                )
            );
            
            // 边间距：基于节点间距的合理比例
            // 增加边间距，确保边之间有足够的空间，避免视觉混乱
            const calculatedEdgeSep = Math.max(50, calculatedNodeSep * 0.4);
            
            // 根据图的复杂度调整布局参数，优化可读性和空间利用率
            layoutOptions = {
                name: 'dagre',
                rankDir: 'TB',  // 从上到下
                spacingFactor: 1.2,  // 增加间距因子，让图更宽
                nodeSep: Math.round(calculatedNodeSep),  // 动态计算的节点间距
                edgeSep: Math.round(calculatedEdgeSep),  // 动态计算的边间距
                rankSep: Math.round(calculatedRankSep),  // 动态计算的层级间距
                nodeDimensionsIncludeLabels: true,  // 考虑标签大小
                animate: false,
                padding: Math.max(40, Math.min(60, containerWidth * 0.03)),  // 减少边距，让图更宽
                // 优化边的路由，减少交叉
                edgeRouting: 'polyline',
                // 对齐方式：使用上左对齐，然后手动居中
                align: 'UL'  // 上左对齐（dagre不支持'C'）
            };
        } catch (e) {
            console.warn('dagre布局注册失败，使用默认布局:', e);
        }
    } else {
        console.warn('dagre布局插件未加载，使用默认布局');
    }
    
    // 应用布局，等待布局完成后再平衡和居中
    const layout = attackChainCytoscape.layout(layoutOptions);
    layout.one('layoutstop', () => {
        // 布局完成后，先平衡分支，再修复重叠，最后居中显示
        setTimeout(() => {
            balanceBranches();
            setTimeout(() => {
                fixNodeOverlaps();
                setTimeout(() => {
                    centerAttackChain();
                }, 50);
            }, 50);
        }, 100);
    });
    layout.run();
    
    // 平衡分支分布的函数 - 使分支在根节点左右平均分布
    function balanceBranches() {
        try {
            if (!attackChainCytoscape) {
                return;
            }
            
            // 动态计算节点间距，基于容器尺寸
            const container = attackChainCytoscape.container();
            const containerWidth = container ? container.offsetWidth : 1200;
            const avgNodeWidth = isComplexGraph ? 370 : 410;  // 与布局计算保持一致
            const estimatedDepth = Math.ceil(Math.log2(Math.max(nodeCount, 2))) + 1;
            const maxLevelWidth = Math.max(1, Math.ceil(nodeCount / estimatedDepth));
            const targetGraphWidth = containerWidth * 0.95;  // 与布局计算保持一致，使用95%宽度
            // 与布局计算保持一致，使用更大的间距避免节点重叠
            const minNodeSep = avgNodeWidth * 1.5;  // 与布局计算保持一致
            const availableWidth = targetGraphWidth - avgNodeWidth * maxLevelWidth;
            const spacing = Math.max(
                minNodeSep,
                Math.min(
                    availableWidth / Math.max(1, maxLevelWidth - 1),
                    avgNodeWidth * 3.0  // 与布局计算保持一致
                )
            );
            
            // 找到target节点作为根节点
            const targetNodes = attackChainCytoscape.nodes().filter(node => {
                return node.data('type') === 'target';
            });
            
            if (targetNodes.length === 0) {
                return; // 没有target节点，无法平衡
            }
            
            const rootNode = targetNodes[0];
            const rootPos = rootNode.position();
            const rootX = rootPos.x;
            const rootY = rootPos.y;
            
            // 构建图的邻接表
            const edges = attackChainCytoscape.edges();
            const childrenMap = new Map();
            
            edges.forEach(edge => {
                const { source, target, valid } = getEdgeNodes(edge);
                if (valid) {
                    const sourceId = source.id();
                    const targetId = target.id();
                    
                    if (!childrenMap.has(sourceId)) {
                        childrenMap.set(sourceId, []);
                    }
                    childrenMap.get(sourceId).push(targetId);
                }
            });
            
            // 计算每个节点的子树宽度（递归）
            const subtreeWidth = new Map();
            function calculateSubtreeWidth(nodeId) {
                if (subtreeWidth.has(nodeId)) {
                    return subtreeWidth.get(nodeId);
                }
                
                const children = childrenMap.get(nodeId) || [];
                if (children.length === 0) {
                    subtreeWidth.set(nodeId, 0);
                    return 0;
                }
                
                // 计算所有子树的宽度总和
                let totalWidth = 0;
                children.forEach(childId => {
                    totalWidth += calculateSubtreeWidth(childId);
                });
                
                // 使用动态计算的间距
                const width = Math.max(totalWidth + (children.length - 1) * spacing, spacing);
                
                subtreeWidth.set(nodeId, width);
                return width;
            }
            
            // 计算所有子树宽度
            const nodes = attackChainCytoscape.nodes();
            nodes.forEach(node => {
                calculateSubtreeWidth(node.id());
            });
            
            // 获取根节点的直接子节点
            const rootChildren = childrenMap.get(rootNode.id()) || [];
            
            if (rootChildren.length === 0) {
                return; // 没有子节点
            }
            
            // 将子节点分成左右两组
            const childWidths = rootChildren.map(childId => ({
                id: childId,
                width: subtreeWidth.get(childId) || 100
            })).sort((a, b) => b.width - a.width);
            
            const leftGroup = [];
            const rightGroup = [];
            let leftTotal = 0;
            let rightTotal = 0;
            
            // 贪心分配：将较大的子树交替分配到左右
            childWidths.forEach(child => {
                if (leftTotal <= rightTotal) {
                    leftGroup.push(child);
                    leftTotal += child.width;
                } else {
                    rightGroup.push(child);
                    rightTotal += child.width;
                }
            });
            
            // 计算左右两侧需要的总宽度（使用动态计算的间距）
            const leftTotalWidth = leftGroup.length > 0 ? leftTotal + (leftGroup.length - 1) * spacing : 0;
            const rightTotalWidth = rightGroup.length > 0 ? rightTotal + (rightGroup.length - 1) * spacing : 0;
            // 根据容器宽度动态调整，充分利用水平空间
            // 使用更大的宽度系数，让图充分利用容器空间（使用95%的容器宽度以匹配布局算法）
            const maxSideWidth = Math.max(leftTotalWidth, rightTotalWidth);
            const targetWidth = Math.max(maxSideWidth * 1.2, containerWidth * 0.95);  // 使用95%的容器宽度以匹配布局
            const maxWidth = Math.max(targetWidth, avgNodeWidth * 2);
            
            // 递归调整子树位置
            function adjustSubtree(nodeId, centerX, availableWidth) {
                const node = attackChainCytoscape.getElementById(nodeId);
                if (!node) return;
                
                const currentPos = node.position();
                const children = childrenMap.get(nodeId) || [];
                
                if (children.length === 0) {
                    // 叶子节点
                    node.position({
                        x: centerX,
                        y: currentPos.y
                    });
                    return;
                }
                
                // 计算子节点的宽度
                const childWidths = children.map(childId => ({
                    id: childId,
                    width: subtreeWidth.get(childId) || 100
                }));
                
                const totalChildWidth = childWidths.reduce((sum, c) => sum + c.width, 0);
                const totalSpacing = (children.length - 1) * spacing;
                const neededWidth = totalChildWidth + totalSpacing;
                
                // 如果需要的宽度超过可用宽度，按比例缩放
                const scale = neededWidth > availableWidth ? availableWidth / neededWidth : 1;
                const scaledWidth = neededWidth * scale;
                
                // 分配子节点位置
                let currentOffset = -scaledWidth / 2;
                childWidths.forEach((child, index) => {
                    const childWidth = child.width * scale;
                    const childCenterX = centerX + currentOffset + childWidth / 2;
                    
                    adjustSubtree(child.id, childCenterX, childWidth);
                    currentOffset += childWidth + spacing * scale;
                });
                
                // 调整当前节点到子节点的中心
                const childPositions = children.map(childId => {
                    const childNode = attackChainCytoscape.getElementById(childId);
                    return childNode ? childNode.position().x : centerX;
                });
                const childrenCenterX = childPositions.reduce((sum, x) => sum + x, 0) / childPositions.length;
                
                node.position({
                    x: childrenCenterX,
                    y: currentPos.y
                });
            }
            
            // 调整左侧子树
            let leftOffset = -maxWidth / 2;
            leftGroup.forEach((child, index) => {
                const childWidth = child.width;
                const childCenterX = rootX + leftOffset + childWidth / 2;
                adjustSubtree(child.id, childCenterX, childWidth);
                leftOffset += childWidth + spacing;
            });
            
            // 调整右侧子树
            let rightOffset = maxWidth / 2;
            rightGroup.forEach((child, index) => {
                const childWidth = child.width;
                const childCenterX = rootX + rightOffset - childWidth / 2;
                adjustSubtree(child.id, childCenterX, childWidth);
                rightOffset -= (childWidth + spacing);
            });
            
            // 重新计算根节点的中心位置：基于所有直接子节点的实际位置
            const rootChildrenPositions = rootChildren.map(childId => {
                const childNode = attackChainCytoscape.getElementById(childId);
                return childNode ? childNode.position().x : rootX;
            });
            
            if (rootChildrenPositions.length > 0) {
                // 计算所有子节点的平均 x 位置作为根节点的中心位置
                const childrenCenterX = rootChildrenPositions.reduce((sum, x) => sum + x, 0) / rootChildrenPositions.length;
                rootNode.position({
                    x: childrenCenterX,
                    y: rootY
                });
            } else {
                // 如果没有子节点，保持原位置
                rootNode.position({
                    x: rootX,
                    y: rootY
                });
            }
            
        } catch (error) {
            console.warn('平衡分支时出错:', error);
        }
    }
    
    // 修复节点重叠的函数
    function fixNodeOverlaps() {
        try {
            if (!attackChainCytoscape) {
                return;
            }
            
            const nodes = attackChainCytoscape.nodes();
            const minSpacing = 40; // 节点之间的最小间距（像素），增加以确保不重叠
            const overlapThreshold = 0.05; // 重叠阈值（5%），更敏感地检测重叠
            
            // 按Y坐标分组节点（同一层级的节点）
            const nodesByLevel = new Map();
            nodes.forEach(node => {
                const pos = node.position();
                const y = Math.round(pos.y / 30) * 30; // 将相近的Y坐标归为同一层级（更精细的分组）
                
                if (!nodesByLevel.has(y)) {
                    nodesByLevel.set(y, []);
                }
                nodesByLevel.get(y).push(node);
            });
            
            // 检查并修复同一层级内的重叠
            nodesByLevel.forEach((levelNodes, levelY) => {
                // 按X坐标排序
                levelNodes.sort((a, b) => a.position().x - b.position().x);
                
                // 检查相邻节点是否重叠
                for (let i = 0; i < levelNodes.length - 1; i++) {
                    const node1 = levelNodes[i];
                    const node2 = levelNodes[i + 1];
                    
                    const pos1 = node1.position();
                    const pos2 = node2.position();
                    const width1 = node1.width();
                    const width2 = node2.width();
                    const height1 = node1.height();
                    const height2 = node2.height();
                    
                    // 计算节点边界
                    const left1 = pos1.x - width1 / 2;
                    const right1 = pos1.x + width1 / 2;
                    const top1 = pos1.y - height1 / 2;
                    const bottom1 = pos1.y + height1 / 2;
                    
                    const left2 = pos2.x - width2 / 2;
                    const right2 = pos2.x + width2 / 2;
                    const top2 = pos2.y - height2 / 2;
                    const bottom2 = pos2.y + height2 / 2;
                    
                    // 检查是否重叠
                    const horizontalOverlap = Math.max(0, Math.min(right1, right2) - Math.max(left1, left2));
                    const verticalOverlap = Math.max(0, Math.min(bottom1, bottom2) - Math.max(top1, top2));
                    
                    const overlapArea = horizontalOverlap * verticalOverlap;
                    const node1Area = width1 * height1;
                    const node2Area = width2 * height2;
                    const minArea = Math.min(node1Area, node2Area);
                    
                    // 如果重叠面积超过阈值，调整位置
                    if (overlapArea > minArea * overlapThreshold) {
                        // 计算需要的间距
                        const requiredSpacing = (width1 + width2) / 2 + minSpacing;
                        const currentSpacing = pos2.x - pos1.x;
                        const spacingDiff = requiredSpacing - currentSpacing;
                        
                        if (spacingDiff > 0) {
                            // 向右移动第二个节点及其后续节点
                            const moveDistance = spacingDiff;
                            for (let j = i + 1; j < levelNodes.length; j++) {
                                const node = levelNodes[j];
                                const currentPos = node.position();
                                node.position({
                                    x: currentPos.x + moveDistance,
                                    y: currentPos.y
                                });
                            }
                        }
                    }
                }
            });
            
            // 检查不同层级之间的重叠（垂直方向）- 简化处理
            // 只处理明显的垂直重叠，通过增加层级间距来解决
            const sortedLevels = Array.from(nodesByLevel.keys()).sort((a, b) => a - b);
            for (let i = 0; i < sortedLevels.length - 1; i++) {
                const level1Y = sortedLevels[i];
                const level2Y = sortedLevels[i + 1];
                const level1Nodes = nodesByLevel.get(level1Y);
                const level2Nodes = nodesByLevel.get(level2Y);
                
                // 检查两个层级之间的最小垂直间距
                let minVerticalSpacing = Infinity;
                level1Nodes.forEach(node1 => {
                    const pos1 = node1.position();
                    const height1 = node1.height();
                    const bottom1 = pos1.y + height1 / 2;
                    
                    level2Nodes.forEach(node2 => {
                        const pos2 = node2.position();
                        const height2 = node2.height();
                        const top2 = pos2.y - height2 / 2;
                        
                        const spacing = top2 - bottom1;
                        if (spacing < minVerticalSpacing) {
                            minVerticalSpacing = spacing;
                        }
                    });
                });
                
                // 如果垂直间距太小，向下移动第二个层级的所有节点
                if (minVerticalSpacing < minSpacing) {
                    const moveDistance = minSpacing - minVerticalSpacing;
                    level2Nodes.forEach(node => {
                        const currentPos = node.position();
                        node.position({
                            x: currentPos.x,
                            y: currentPos.y + moveDistance
                        });
                    });
                    
                    // 更新后续层级的Y坐标
                    for (let j = i + 2; j < sortedLevels.length; j++) {
                        const laterLevelY = sortedLevels[j];
                        const laterLevelNodes = nodesByLevel.get(laterLevelY);
                        laterLevelNodes.forEach(node => {
                            const currentPos = node.position();
                            node.position({
                                x: currentPos.x,
                                y: currentPos.y + moveDistance
                            });
                        });
                    }
                }
            }
            
        } catch (error) {
            console.warn('修复节点重叠时出错:', error);
        }
    }
    
    // 居中攻击链的函数
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
                // 如果容器尺寸为0，延迟重试
                setTimeout(centerAttackChain, 100);
                return;
            }
            
            // 先fit以适应所有节点，使用更小的边距以更好地填充空间
            attackChainCytoscape.fit(undefined, 60);
            
            // 等待fit完成，然后根据图的宽度调整缩放，并整体居中
            setTimeout(() => {
                const extent = attackChainCytoscape.extent();
                if (!extent || typeof extent.x1 === 'undefined' || typeof extent.x2 === 'undefined' || 
                    typeof extent.y1 === 'undefined' || typeof extent.y2 === 'undefined') {
                    return;
                }
                
                // 根据图的宽度和容器宽度，调整缩放以更好地利用水平空间
                const graphWidth = extent.x2 - extent.x1;
                const graphHeight = extent.y2 - extent.y1;
                const availableWidth = containerWidth * 0.95; // 使用95%的容器宽度（与布局算法一致）
                const availableHeight = containerHeight * 0.85; // 使用85%的容器高度
                const currentZoom = attackChainCytoscape.zoom();
                
                // 计算基于宽度和高度的缩放比例，选择较小的以适配
                const widthScale = graphWidth > 0 ? availableWidth / (graphWidth * currentZoom) : 1;
                const heightScale = graphHeight > 0 ? availableHeight / (graphHeight * currentZoom) : 1;
                const scale = Math.min(widthScale, heightScale);
                
                if (graphWidth > 0 && scale > 1 && scale < 1.4) {
                    // 如果图在当前缩放下太窄，稍微放大以填充空间，但不要过度放大
                    attackChainCytoscape.zoom(currentZoom * scale);
                }
                
                // 如果图太复杂，稍微缩小视图
                if (isComplexGraph && nodeCount > 20) {
                    attackChainCytoscape.zoom(attackChainCytoscape.zoom() * 0.9);
                }
                
                // 计算图的中心点（在图形坐标系中）
                const graphCenterX = (extent.x1 + extent.x2) / 2;
                const graphCenterY = (extent.y1 + extent.y2) / 2;
                
                // 获取当前的缩放和平移
                const zoom = attackChainCytoscape.zoom();
                const pan = attackChainCytoscape.pan();
                
                // 计算图中心在当前视图中的位置
                const graphCenterViewX = graphCenterX * zoom + pan.x;
                const graphCenterViewY = graphCenterY * zoom + pan.y;
                
                // 目标位置：容器中心
                const desiredViewX = containerWidth / 2;
                const desiredViewY = containerHeight / 2;
                
                // 计算需要平移的距离
                const deltaX = desiredViewX - graphCenterViewX;
                const deltaY = desiredViewY - graphCenterViewY;
                
                // 应用新的平移，使整个图居中（包括所有分支）
                attackChainCytoscape.pan({
                    x: pan.x + deltaX,
                    y: pan.y + deltaY
                });
            }, 150);
        } catch (error) {
            console.warn('居中图表时出错:', error);
        }
    }
    
    // 添加点击事件
    attackChainCytoscape.on('tap', 'node', function(evt) {
        const node = evt.target;
        showNodeDetails(node.data());
    });
    
    // 添加悬停效果（使用事件监听器替代CSS选择器）
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
        // 恢复默认边框宽度
        const defaultBorderWidth = type === 'target' ? 5 : 4;
        node.style('border-width', defaultBorderWidth);
        node.style('z-index', 'auto');
        node.style('overlay-opacity', 0);
    });
    
    // 保存原始数据用于过滤
    window.attackChainOriginalData = chainData;
}

// 安全地获取边的源节点和目标节点
function getEdgeNodes(edge) {
    try {
        const source = edge.source();
        const target = edge.target();
        
        // 检查源节点和目标节点是否存在
        if (!source || !target || source.length === 0 || target.length === 0) {
            return { source: null, target: null, valid: false };
        }
        
        return { source: source, target: target, valid: true };
    } catch (error) {
        console.warn('获取边的节点时出错:', error, edge.id());
        return { source: null, target: null, valid: false };
    }
}

// 过滤攻击链节点（按搜索关键词）
function filterAttackChainNodes(searchText) {
    if (!attackChainCytoscape || !window.attackChainOriginalData) {
        return;
    }
    
    const searchLower = searchText.toLowerCase().trim();
    if (searchLower === '') {
        // 重置所有节点可见性
        attackChainCytoscape.nodes().style('display', 'element');
        attackChainCytoscape.edges().style('display', 'element');
        // 恢复默认边框
        attackChainCytoscape.nodes().style('border-width', 2);
        return;
    }
    
    // 过滤节点
    attackChainCytoscape.nodes().forEach(node => {
        // 使用原始标签进行搜索，不包含类型标签
        const originalLabel = node.data('originalLabel') || node.data('label') || '';
        const label = originalLabel.toLowerCase();
        const type = (node.data('type') || '').toLowerCase();
        const matches = label.includes(searchLower) || type.includes(searchLower);
        
        if (matches) {
            node.style('display', 'element');
            // 高亮匹配的节点
            node.style('border-width', 4);
            node.style('border-color', '#0066ff');
        } else {
            node.style('display', 'none');
        }
    });
    
    // 隐藏没有可见源节点或目标节点的边
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
    
    // 重新调整视图
    attackChainCytoscape.fit(undefined, 60);
}

// 按类型过滤攻击链节点
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
    
    // 过滤节点
    attackChainCytoscape.nodes().forEach(node => {
        const nodeType = node.data('type') || '';
        if (nodeType === type) {
            node.style('display', 'element');
        } else {
            node.style('display', 'none');
        }
    });
    
    // 隐藏没有可见源节点或目标节点的边
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
    
    // 重新调整视图
    attackChainCytoscape.fit(undefined, 60);
}

// 按风险等级过滤攻击链节点
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
    
    // 定义风险范围
    const riskRanges = {
        'high': [80, 100],
        'medium-high': [60, 79],
        'medium': [40, 59],
        'low': [0, 39]
    };
    
    const [minRisk, maxRisk] = riskRanges[riskLevel] || [0, 100];
    
    // 过滤节点
    attackChainCytoscape.nodes().forEach(node => {
        const riskScore = node.data('riskScore') || 0;
        if (riskScore >= minRisk && riskScore <= maxRisk) {
            node.style('display', 'element');
        } else {
            node.style('display', 'none');
        }
    });
    
    // 隐藏没有可见源节点或目标节点的边
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
    
    // 重新调整视图
    attackChainCytoscape.fit(undefined, 60);
}

// 重置攻击链筛选
function resetAttackChainFilters() {
    // 重置搜索框
    const searchInput = document.getElementById('attack-chain-search');
    if (searchInput) {
        searchInput.value = '';
    }
    
    // 重置类型筛选
    const typeFilter = document.getElementById('attack-chain-type-filter');
    if (typeFilter) {
        typeFilter.value = 'all';
    }
    
    // 重置风险筛选
    const riskFilter = document.getElementById('attack-chain-risk-filter');
    if (riskFilter) {
        riskFilter.value = 'all';
    }
    
    // 重置所有节点可见性
    if (attackChainCytoscape) {
        attackChainCytoscape.nodes().forEach(node => {
            node.style('display', 'element');
            node.style('border-width', 2); // 恢复默认边框
        });
        attackChainCytoscape.edges().style('display', 'element');
        attackChainCytoscape.fit(undefined, 60);
    }
}

// 显示节点详情
function showNodeDetails(nodeData) {
    const detailsPanel = document.getElementById('attack-chain-details');
    const detailsContent = document.getElementById('attack-chain-details-content');
    
    if (!detailsPanel || !detailsContent) {
        return;
    }
    
    // 使用 requestAnimationFrame 优化显示动画
    requestAnimationFrame(() => {
        detailsPanel.style.display = 'flex';
        // 在下一帧设置透明度，确保显示动画流畅
        requestAnimationFrame(() => {
            detailsPanel.style.opacity = '1';
        });
    });
    
    let html = `
        <div class="node-detail-item">
            <strong>节点ID:</strong> <code>${nodeData.id}</code>
        </div>
        <div class="node-detail-item">
            <strong>类型:</strong> ${getNodeTypeLabel(nodeData.type)}
        </div>
        <div class="node-detail-item">
            <strong>标签:</strong> ${escapeHtml(nodeData.originalLabel || nodeData.label)}
        </div>
        <div class="node-detail-item">
            <strong>风险评分:</strong> ${nodeData.riskScore}/100
        </div>
    `;
    
    // 显示action节点信息（工具执行 + AI分析）
    if (nodeData.type === 'action' && nodeData.metadata) {
        if (nodeData.metadata.tool_name) {
            html += `
                <div class="node-detail-item">
                    <strong>工具名称:</strong> <code>${escapeHtml(nodeData.metadata.tool_name)}</code>
                </div>
            `;
        }
        if (nodeData.metadata.tool_intent) {
            html += `
                <div class="node-detail-item">
                    <strong>工具意图:</strong> <span style="color: #0066ff; font-weight: bold;">${escapeHtml(nodeData.metadata.tool_intent)}</span>
                </div>
            `;
        }
        if (nodeData.metadata.status === 'failed_insight') {
            html += `
                <div class="node-detail-item">
                    <strong>执行状态:</strong> <span style="color: #ff9800; font-weight: bold;">失败但有线索</span>
                </div>
            `;
        }
        if (nodeData.metadata.ai_analysis) {
            html += `
                <div class="node-detail-item">
                    <strong>AI分析:</strong> <div style="margin-top: 5px; padding: 8px; background: #f5f5f5; border-radius: 4px;">${escapeHtml(nodeData.metadata.ai_analysis)}</div>
                </div>
            `;
        }
        if (nodeData.metadata.findings && Array.isArray(nodeData.metadata.findings) && nodeData.metadata.findings.length > 0) {
            html += `
                <div class="node-detail-item">
                    <strong>关键发现:</strong>
                    <ul style="margin: 5px 0; padding-left: 20px;">
                        ${nodeData.metadata.findings.map(f => `<li>${escapeHtml(f)}</li>`).join('')}
                    </ul>
                </div>
            `;
        }
    }
    
    // 显示目标信息（如果是目标节点）
    if (nodeData.type === 'target' && nodeData.metadata && nodeData.metadata.target) {
        html += `
            <div class="node-detail-item">
                <strong>测试目标:</strong> <code>${escapeHtml(nodeData.metadata.target)}</code>
            </div>
        `;
    }
    
    // 显示漏洞信息（如果是漏洞节点）
    if (nodeData.type === 'vulnerability' && nodeData.metadata) {
        if (nodeData.metadata.vulnerability_type) {
            html += `
                <div class="node-detail-item">
                    <strong>漏洞类型:</strong> ${escapeHtml(nodeData.metadata.vulnerability_type)}
                </div>
            `;
        }
        if (nodeData.metadata.description) {
            html += `
                <div class="node-detail-item">
                    <strong>描述:</strong> ${escapeHtml(nodeData.metadata.description)}
                </div>
            `;
        }
        if (nodeData.metadata.severity) {
            html += `
                <div class="node-detail-item">
                    <strong>严重程度:</strong> <span style="color: ${getSeverityColor(nodeData.metadata.severity)}; font-weight: bold;">${escapeHtml(nodeData.metadata.severity)}</span>
                </div>
            `;
        }
        if (nodeData.metadata.location) {
            html += `
                <div class="node-detail-item">
                    <strong>位置:</strong> <code>${escapeHtml(nodeData.metadata.location)}</code>
                </div>
            `;
        }
    }
    
    if (nodeData.toolExecutionId) {
        html += `
            <div class="node-detail-item">
                <strong>工具执行ID:</strong> <code>${nodeData.toolExecutionId}</code>
            </div>
        `;
    }
    
    // 先重置滚动位置，避免内容更新时的滚动计算
    if (detailsContent) {
        detailsContent.scrollTop = 0;
    }
    
    // 使用 requestAnimationFrame 优化 DOM 更新和滚动
    requestAnimationFrame(() => {
        // 更新内容
        detailsContent.innerHTML = html;
        
        // 在下一帧执行滚动，避免与 DOM 更新冲突
        requestAnimationFrame(() => {
            // 重置详情内容区域的滚动位置
            if (detailsContent) {
                detailsContent.scrollTop = 0;
            }
            
            // 重置侧边栏的滚动位置，确保详情区域可见
            const sidebar = document.querySelector('.attack-chain-sidebar-content');
            if (sidebar) {
                // 找到详情面板的位置
                const detailsPanel = document.getElementById('attack-chain-details');
                if (detailsPanel && detailsPanel.offsetParent !== null) {
                    // 使用 getBoundingClientRect 获取位置，性能更好
                    const detailsRect = detailsPanel.getBoundingClientRect();
                    const sidebarRect = sidebar.getBoundingClientRect();
                    const scrollTop = sidebar.scrollTop;
                    const relativeTop = detailsRect.top - sidebarRect.top + scrollTop;
                    sidebar.scrollTop = relativeTop - 20; // 留一点边距
                }
            }
        });
    });
}

// 获取严重程度颜色
function getSeverityColor(severity) {
    const colors = {
        'critical': '#ff0000',
        'high': '#ff4444',
        'medium': '#ff8800',
        'low': '#ffbb00'
    };
    return colors[severity.toLowerCase()] || '#666';
}

// 获取节点类型标签
function getNodeTypeLabel(type) {
    const labels = {
        'action': '行动',
        'vulnerability': '漏洞',
        'target': '目标'
    };
    return labels[type] || type;
}

// 更新统计信息
function updateAttackChainStats(chainData) {
    const statsElement = document.getElementById('attack-chain-stats');
    if (statsElement) {
        const nodeCount = chainData.nodes ? chainData.nodes.length : 0;
        const edgeCount = chainData.edges ? chainData.edges.length : 0;
        statsElement.textContent = `节点: ${nodeCount} | 边: ${edgeCount}`;
    }
}

// 关闭节点详情
function closeNodeDetails() {
    const detailsPanel = document.getElementById('attack-chain-details');
    if (detailsPanel) {
        // 添加淡出动画
        detailsPanel.style.opacity = '0';
        detailsPanel.style.maxHeight = detailsPanel.scrollHeight + 'px';
        
        setTimeout(() => {
            detailsPanel.style.display = 'none';
            detailsPanel.style.maxHeight = '';
            detailsPanel.style.opacity = '';
        }, 300);
    }
    
    // 取消选中节点
    if (attackChainCytoscape) {
        attackChainCytoscape.elements().unselect();
    }
}

// 关闭攻击链模态框
function closeAttackChainModal() {
    const modal = document.getElementById('attack-chain-modal');
    if (modal) {
        modal.style.display = 'none';
    }
    
    // 关闭节点详情
    closeNodeDetails();
    
    // 清理Cytoscape实例
    if (attackChainCytoscape) {
        attackChainCytoscape.destroy();
        attackChainCytoscape = null;
    }
    
    currentAttackChainConversationId = null;
}

// 刷新攻击链（重新加载）
// 注意：此函数允许在加载过程中调用，用于检查生成状态
function refreshAttackChain() {
    if (currentAttackChainConversationId) {
        // 临时允许刷新，即使正在加载中（用于检查生成状态）
        const wasLoading = isAttackChainLoading(currentAttackChainConversationId);
        setAttackChainLoading(currentAttackChainConversationId, false); // 临时重置，允许刷新
        loadAttackChain(currentAttackChainConversationId).finally(() => {
            // 如果之前正在加载（409 情况），恢复加载状态
            // 否则保持 false（正常完成）
            if (wasLoading) {
                // 检查是否仍然需要保持加载状态（如果还是 409，会在 loadAttackChain 中处理）
                // 这里我们假设如果成功加载，则重置状态
                // 如果还是 409，loadAttackChain 会保持加载状态
            }
        });
    }
}

// 重新生成攻击链
async function regenerateAttackChain() {
    if (!currentAttackChainConversationId) {
        return;
    }
    
    // 防止重复点击（只检查当前对话的加载状态）
    if (isAttackChainLoading(currentAttackChainConversationId)) {
        console.log('攻击链正在生成中，请稍候...');
        return;
    }
    
    // 保存请求时的对话ID，防止串台
    const savedConversationId = currentAttackChainConversationId;
    setAttackChainLoading(savedConversationId, true);
    
    const container = document.getElementById('attack-chain-container');
    if (container) {
        container.innerHTML = '<div class="loading-spinner">重新生成中...</div>';
    }
    
    // 禁用重新生成按钮
    const regenerateBtn = document.querySelector('button[onclick="regenerateAttackChain()"]');
    if (regenerateBtn) {
        regenerateBtn.disabled = true;
        regenerateBtn.style.opacity = '0.5';
        regenerateBtn.style.cursor = 'not-allowed';
    }
    
    try {
        // 调用重新生成接口
        const response = await apiFetch(`/api/attack-chain/${savedConversationId}/regenerate`, {
            method: 'POST'
        });
        
        if (!response.ok) {
            // 处理 409 Conflict（正在生成中）
            if (response.status === 409) {
                const error = await response.json();
                if (container) {
                    container.innerHTML = `
                        <div class="loading-spinner" style="text-align: center; padding: 40px;">
                            <div style="margin-bottom: 16px;">⏳ 攻击链正在生成中...</div>
                            <div style="color: var(--text-secondary); font-size: 0.875rem;">
                                请稍候，生成完成后将自动显示
                            </div>
                            <button class="btn-secondary" onclick="refreshAttackChain()" style="margin-top: 16px;">
                                刷新查看进度
                            </button>
                        </div>
                    `;
                }
                // 5秒后自动刷新
                // savedConversationId 已在函数开始处定义
                setTimeout(() => {
                    // 检查当前显示的对话ID是否匹配，且仍在加载中
                    if (currentAttackChainConversationId === savedConversationId && 
                        isAttackChainLoading(savedConversationId)) {
                        refreshAttackChain();
                    }
                }, 5000);
                return;
            }
            
            const error = await response.json();
            throw new Error(error.error || '重新生成攻击链失败');
        }
        
        const chainData = await response.json();
        
        // 检查当前显示的对话ID是否匹配，防止串台
        if (currentAttackChainConversationId !== savedConversationId) {
            console.log('攻击链数据已返回，但当前显示的对话已切换，忽略此次渲染', {
                returned: savedConversationId,
                current: currentAttackChainConversationId
            });
            setAttackChainLoading(savedConversationId, false);
            return;
        }
        
        // 渲染攻击链
        renderAttackChain(chainData);
        
        // 更新统计信息
        updateAttackChainStats(chainData);
        
    } catch (error) {
        console.error('重新生成攻击链失败:', error);
        if (container) {
            container.innerHTML = `<div class="error-message">重新生成失败: ${error.message}</div>`;
        }
    } finally {
        setAttackChainLoading(savedConversationId, false);
        
        // 恢复重新生成按钮
        if (regenerateBtn) {
            regenerateBtn.disabled = false;
            regenerateBtn.style.opacity = '1';
            regenerateBtn.style.cursor = 'pointer';
        }
    }
}

// 导出攻击链
function exportAttackChain(format) {
    if (!attackChainCytoscape) {
        alert('请先加载攻击链');
        return;
    }
    
    // 确保图形已经渲染完成（使用小延迟）
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
                    
                    // 处理 Promise
                    if (pngPromise && typeof pngPromise.then === 'function') {
                        pngPromise.then(blob => {
                            if (!blob) {
                                throw new Error('PNG导出返回空数据');
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
                            console.error('导出PNG失败:', err);
                            alert('导出PNG失败: ' + (err.message || '未知错误'));
                        });
                    } else {
                        // 如果不是 Promise，直接使用
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
                    console.error('PNG导出错误:', err);
                    alert('导出PNG失败: ' + (err.message || '未知错误'));
                }
            } else if (format === 'svg') {
                try {
                    // Cytoscape.js 3.x 不直接支持 .svg() 方法
                    // 使用替代方案：从 Cytoscape 数据手动构建 SVG
                    const container = attackChainCytoscape.container();
                    if (!container) {
                        throw new Error('无法获取容器元素');
                    }
                    
                    // 获取所有节点和边
                    const nodes = attackChainCytoscape.nodes();
                    const edges = attackChainCytoscape.edges();
                    
                    if (nodes.length === 0) {
                        throw new Error('没有节点可导出');
                    }
                    
                    // 计算所有节点的实际边界（包括节点大小）
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
                    
                    // 也考虑边的范围
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
                    
                    // 添加边距
                    const padding = 50;
                    minX -= padding;
                    minY -= padding;
                    maxX += padding;
                    maxY += padding;
                    
                    const width = maxX - minX;
                    const height = maxY - minY;
                    
                    // 创建 SVG 元素
                    const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
                    svg.setAttribute('width', width.toString());
                    svg.setAttribute('height', height.toString());
                    svg.setAttribute('xmlns', 'http://www.w3.org/2000/svg');
                    svg.setAttribute('viewBox', `${minX} ${minY} ${width} ${height}`);
                    
                    // 添加白色背景矩形
                    const bgRect = document.createElementNS('http://www.w3.org/2000/svg', 'rect');
                    bgRect.setAttribute('x', minX.toString());
                    bgRect.setAttribute('y', minY.toString());
                    bgRect.setAttribute('width', width.toString());
                    bgRect.setAttribute('height', height.toString());
                    bgRect.setAttribute('fill', 'white');
                    svg.appendChild(bgRect);
                    
                    // 创建 defs 用于箭头标记
                    const defs = document.createElementNS('http://www.w3.org/2000/svg', 'defs');
                    
                    // 添加边的箭头标记（为不同类型的边创建不同的箭头）
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
                    
                    // 添加边（先绘制，这样节点会在上面）
                    edges.forEach(edge => {
                        const { source, target, valid } = getEdgeNodes(edge);
                        if (!valid) {
                            return; // 跳过无效的边
                        }
                        
                        const sourcePos = source.position();
                        const targetPos = target.position();
                        const edgeData = edge.data();
                        const edgeType = edgeData.type || 'leads_to';
                        
                        // 获取边的样式
                        let lineColor = '#999';
                        if (edgeType === 'discovers') lineColor = '#3498db';
                        else if (edgeType === 'targets') lineColor = '#0066ff';
                        else if (edgeType === 'enables') lineColor = '#e74c3c';
                        else if (edgeType === 'leads_to') lineColor = '#666';
                        
                        // 创建路径（支持曲线）
                        const path = document.createElementNS('http://www.w3.org/2000/svg', 'path');
                        // 简单的直线路径（可以改进为曲线）
                        const midX = (sourcePos.x + targetPos.x) / 2;
                        const midY = (sourcePos.y + targetPos.y) / 2;
                        const dx = targetPos.x - sourcePos.x;
                        const dy = targetPos.y - sourcePos.y;
                        const offset = Math.min(30, Math.sqrt(dx * dx + dy * dy) * 0.3);
                        
                        // 使用二次贝塞尔曲线
                        const controlX = midX + (dy > 0 ? -offset : offset);
                        const controlY = midY + (dx > 0 ? offset : -offset);
                        path.setAttribute('d', `M ${sourcePos.x} ${sourcePos.y} Q ${controlX} ${controlY} ${targetPos.x} ${targetPos.y}`);
                        path.setAttribute('stroke', lineColor);
                        path.setAttribute('stroke-width', '2');
                        path.setAttribute('fill', 'none');
                        path.setAttribute('marker-end', `url(#arrowhead-${edgeType})`);
                        svg.appendChild(path);
                    });
                    
                    // 添加节点
                    nodes.forEach(node => {
                        const pos = node.position();
                        const nodeData = node.data();
                        const riskScore = nodeData.riskScore || 0;
                        const nodeWidth = node.width();
                        const nodeHeight = node.height();
                        const size = Math.max(nodeWidth, nodeHeight) / 2;
                        
                        // 确定节点颜色
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
                        
                        // 确定节点形状
                        const nodeType = nodeData.type;
                        let shapeElement;
                        if (nodeType === 'vulnerability') {
                            // 菱形
                            shapeElement = document.createElementNS('http://www.w3.org/2000/svg', 'polygon');
                            const points = [
                                `${pos.x},${pos.y - size}`,
                                `${pos.x + size},${pos.y}`,
                                `${pos.x},${pos.y + size}`,
                                `${pos.x - size},${pos.y}`
                            ].join(' ');
                            shapeElement.setAttribute('points', points);
                        } else if (nodeType === 'target') {
                            // 星形（五角星）
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
                            // 圆角矩形
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
                        
                        // 添加文本标签（使用文本描边提高可读性）
                        // 使用原始标签，不包含类型标签前缀
                        const label = (nodeData.originalLabel || nodeData.label || nodeData.id || '').toString();
                        const maxLength = 15;
                        
                        // 创建文本组，包含描边和填充
                        const textGroup = document.createElementNS('http://www.w3.org/2000/svg', 'g');
                        textGroup.setAttribute('text-anchor', 'middle');
                        textGroup.setAttribute('dominant-baseline', 'middle');
                        
                        // 处理长文本（简单换行）
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
                            lines = lines.slice(0, 2); // 最多两行
                        } else {
                            lines = [label];
                        }
                        
                        // 确定文本描边颜色（与原始渲染一致）
                        let textOutlineColor = '#fff';
                        let textOutlineWidth = 2;
                        if (riskScore >= 80 || riskScore >= 60) {
                            // 红色/橙色背景：白色文字，白色描边，深色轮廓
                            textOutlineColor = '#333';
                            textOutlineWidth = 1;
                        } else if (riskScore >= 40) {
                            // 黄色背景：深色文字，白色描边
                            textOutlineColor = '#fff';
                            textOutlineWidth = 2;
                        } else {
                            // 绿色背景：深绿色文字，白色描边
                            textOutlineColor = '#fff';
                            textOutlineWidth = 2;
                        }
                        
                        // 为每行文本创建描边和填充
                        lines.forEach((line, i) => {
                            const textY = pos.y + (i - (lines.length - 1) / 2) * 16;
                            
                            // 描边文本（用于提高对比度，模拟text-outline效果）
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
                            
                            // 填充文本（实际可见的文本）
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
                    
                    // 将 SVG 转换为字符串
                    const serializer = new XMLSerializer();
                    let svgString = serializer.serializeToString(svg);
                    
                    // 确保有 XML 声明
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
                    console.error('SVG导出错误:', err);
                    alert('导出SVG失败: ' + (err.message || '未知错误'));
                }
            } else {
                alert('不支持的导出格式: ' + format);
            }
        } catch (error) {
            console.error('导出失败:', error);
            alert('导出失败: ' + (error.message || '未知错误'));
        }
    }, 100); // 小延迟确保图形已渲染
}

// ============================================
// 对话分组和批量管理功能
// ============================================

// 分组数据管理（使用API）
let currentGroupId = null; // 当前正在查看的分组详情页面
let currentConversationGroupId = null; // 当前对话所属的分组ID（用于高亮显示）
let contextMenuConversationId = null;
let contextMenuGroupId = null;
let groupsCache = [];
let conversationGroupMappingCache = {};
let pendingGroupMappings = {}; // 待保留的分组映射（用于处理后端API延迟的情况）

// 加载分组列表
async function loadGroups() {
    try {
        const response = await apiFetch('/api/groups');
        if (!response.ok) {
            groupsCache = [];
            return;
        }
        const data = await response.json();
        // 确保groupsCache是有效数组
        if (Array.isArray(data)) {
            groupsCache = data;
        } else {
            // 如果返回的不是数组，使用空数组（不打印警告，因为可能后端返回了错误格式但我们要优雅处理）
            groupsCache = [];
        }

        const groupsList = document.getElementById('conversation-groups-list');
        if (!groupsList) return;

        groupsList.innerHTML = '';

        if (!Array.isArray(groupsCache) || groupsCache.length === 0) {
            return;
        }

        // 对分组进行排序：置顶的分组在前（后端已经排序，这里只需要按顺序显示）
        const sortedGroups = [...groupsCache];

            sortedGroups.forEach(group => {
            const groupItem = document.createElement('div');
            groupItem.className = 'group-item';
            // 高亮逻辑：
            // 1. 如果当前在分组详情页面，只高亮当前分组（currentGroupId）
            // 2. 如果不在分组详情页面，高亮当前对话所属的分组（currentConversationGroupId）
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

            // 如果是置顶分组，添加图钉图标
            if (isPinned) {
                const pinIcon = document.createElement('span');
                pinIcon.className = 'group-item-pinned';
                pinIcon.innerHTML = '📌';
                pinIcon.title = '已置顶';
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
        console.error('加载分组列表失败:', error);
    }
}

// 加载对话列表（修改为支持分组和置顶）
async function loadConversationsWithGroups(searchQuery = '') {
    try {
        // 总是重新加载分组列表和分组映射，确保缓存是最新的
        // 这样可以正确处理分组被删除后的情况
        await loadGroups();
        await loadConversationGroupMapping();

        // 如果有搜索关键词，使用更大的limit以获取所有匹配结果
        const limit = (searchQuery && searchQuery.trim()) ? 1000 : 100;
        let url = `/api/conversations?limit=${limit}`;
        if (searchQuery && searchQuery.trim()) {
            url += '&search=' + encodeURIComponent(searchQuery.trim());
        }
        const response = await apiFetch(url);
        const conversations = await response.json();

        const listContainer = document.getElementById('conversations-list');
        if (!listContainer) {
            return;
        }

        const emptyStateHtml = '<div style="padding: 20px; text-align: center; color: var(--text-muted); font-size: 0.875rem;">暂无历史对话</div>';
        listContainer.innerHTML = '';

        if (!Array.isArray(conversations) || conversations.length === 0) {
            listContainer.innerHTML = emptyStateHtml;
            return;
        }
        
        // 分离置顶和普通对话
        const pinnedConvs = [];
        const normalConvs = [];
        const hasSearchQuery = searchQuery && searchQuery.trim();

        conversations.forEach(conv => {
            // 如果有搜索关键词，显示所有匹配的对话（全局搜索，包括分组中的）
            if (hasSearchQuery) {
                // 搜索时显示所有匹配的对话，不管是否在分组中
                if (conv.pinned) {
                    pinnedConvs.push(conv);
                } else {
                    normalConvs.push(conv);
                }
                return;
            }

            // 如果没有搜索关键词，使用原有逻辑
            // "最近对话"列表应该只显示不在任何分组中的对话
            // 无论是否在分组详情页，都不应该在"最近对话"中显示分组中的对话
            if (conversationGroupMappingCache[conv.id]) {
                // 对话在某个分组中，不应该显示在"最近对话"列表中
                return;
            }

            if (conv.pinned) {
                pinnedConvs.push(conv);
            } else {
                normalConvs.push(conv);
            }
        });

        // 按时间排序
        const sortByTime = (a, b) => {
            const timeA = a.updatedAt ? new Date(a.updatedAt) : new Date(0);
            const timeB = b.updatedAt ? new Date(b.updatedAt) : new Date(0);
            return timeB - timeA;
        };

        pinnedConvs.sort(sortByTime);
        normalConvs.sort(sortByTime);

        const fragment = document.createDocumentFragment();

        // 添加置顶对话
        if (pinnedConvs.length > 0) {
            pinnedConvs.forEach(conv => {
                fragment.appendChild(createConversationListItemWithMenu(conv, true));
            });
        }

        // 添加普通对话
        normalConvs.forEach(conv => {
            fragment.appendChild(createConversationListItemWithMenu(conv, false));
        });

        if (fragment.children.length === 0) {
            listContainer.innerHTML = emptyStateHtml;
            return;
        }

        listContainer.appendChild(fragment);
        updateActiveConversation();
    } catch (error) {
        console.error('加载对话列表失败:', error);
    }
}

// 创建带菜单的对话项
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
    const titleText = conversation.title || '未命名对话';
    title.textContent = safeTruncateText(titleText, 60);
    title.title = titleText; // 设置完整标题以便悬停查看
    titleWrapper.appendChild(title);

    if (isPinned) {
        const pinIcon = document.createElement('span');
        pinIcon.className = 'conversation-item-pinned';
        pinIcon.innerHTML = '📌';
        pinIcon.title = '已置顶';
        titleWrapper.appendChild(pinIcon);
    }

    contentWrapper.appendChild(titleWrapper);

    const time = document.createElement('div');
    time.className = 'conversation-time';
    const dateObj = conversation.updatedAt ? new Date(conversation.updatedAt) : new Date();
    time.textContent = formatConversationTimestamp(dateObj);
    contentWrapper.appendChild(time);

    // 如果对话属于某个分组，显示分组标签
    const groupId = conversationGroupMappingCache[conversation.id];
    if (groupId) {
        const group = groupsCache.find(g => g.id === groupId);
        if (group) {
            const groupTag = document.createElement('div');
            groupTag.className = 'conversation-group-tag';
            groupTag.innerHTML = `<span class="group-tag-icon">${group.icon || '📁'}</span><span class="group-tag-name">${group.name}</span>`;
            groupTag.title = `分组: ${group.name}`;
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

    item.onclick = () => {
        if (currentGroupId) {
            exitGroupDetail();
        }
        loadConversation(conversation.id);
    };

    return item;
}

// 显示对话上下文菜单
async function showConversationContextMenu(event) {
    const menu = document.getElementById('conversation-context-menu');
    if (!menu) return;

    // 先隐藏子菜单，确保每次打开菜单时子菜单都是关闭状态
    const submenu = document.getElementById('move-to-group-submenu');
    if (submenu) {
        submenu.style.display = 'none';
        submenuVisible = false;
    }
    // 清除所有定时器
    clearSubmenuHideTimeout();
    clearSubmenuShowTimeout();
    submenuLoading = false;

    const convId = contextMenuConversationId;
    // 先获取对话的置顶状态并更新菜单文本（在显示菜单之前）
    if (convId) {
        try {
            let isPinned = false;
            // 检查对话是否真的在当前分组中
            const conversationGroupId = conversationGroupMappingCache[convId];
            const isInCurrentGroup = currentGroupId && conversationGroupId === currentGroupId;
            
            if (isInCurrentGroup) {
                // 对话在当前分组中，获取分组内置顶状态
                const response = await apiFetch(`/api/groups/${currentGroupId}/conversations`);
                if (response.ok) {
                    const groupConvs = await response.json();
                    const conv = groupConvs.find(c => c.id === convId);
                    if (conv) {
                        isPinned = conv.groupPinned || false;
                    }
                }
            } else {
                // 不在分组详情页面，或者对话不在当前分组中，获取全局置顶状态
                const response = await apiFetch(`/api/conversations/${convId}`);
                if (response.ok) {
                    const conv = await response.json();
                    isPinned = conv.pinned || false;
                }
            }
            
            // 更新菜单文本
            const pinMenuText = document.getElementById('pin-conversation-menu-text');
            if (pinMenuText) {
                pinMenuText.textContent = isPinned ? '取消置顶' : '置顶此对话';
            }
        } catch (error) {
            console.error('获取对话置顶状态失败:', error);
            // 如果获取失败，使用默认文本
            const pinMenuText = document.getElementById('pin-conversation-menu-text');
            if (pinMenuText) {
                pinMenuText.textContent = '置顶此对话';
            }
        }
    } else {
        // 如果没有对话ID，使用默认文本
        const pinMenuText = document.getElementById('pin-conversation-menu-text');
        if (pinMenuText) {
            pinMenuText.textContent = '置顶此对话';
        }
    }

    // 在状态获取完成后再显示菜单
    menu.style.display = 'block';
    menu.style.visibility = 'visible';
    menu.style.opacity = '1';
    
    // 强制重排以获取正确尺寸
    void menu.offsetHeight;
    
    // 计算菜单位置，确保不超出屏幕
    const menuRect = menu.getBoundingClientRect();
    const viewportWidth = window.innerWidth;
    const viewportHeight = window.innerHeight;
    
    // 获取子菜单的宽度（如果存在，重用之前获取的submenu变量）
    const submenuWidth = submenu ? 180 : 0; // 子菜单宽度 + 间距
    
    let left = event.clientX;
    let top = event.clientY;
    
    // 如果菜单会超出右边界，调整到左侧
    // 考虑子菜单的宽度
    if (left + menuRect.width + submenuWidth > viewportWidth) {
        left = event.clientX - menuRect.width;
        // 如果调整后仍然超出，则放在按钮左侧
        if (left < 0) {
            left = Math.max(8, event.clientX - menuRect.width - submenuWidth);
        }
    }
    
    // 如果菜单会超出下边界，调整到上方
    if (top + menuRect.height > viewportHeight) {
        top = Math.max(8, event.clientY - menuRect.height);
    }
    
    // 确保不超出左边界
    if (left < 0) {
        left = 8;
    }
    
    // 确保不超出上边界
    if (top < 0) {
        top = 8;
    }
    
    menu.style.left = left + 'px';
    menu.style.top = top + 'px';
    
    // 如果菜单在右侧，子菜单应该在左侧显示
    if (submenu && left < event.clientX) {
        submenu.style.left = 'auto';
        submenu.style.right = '100%';
        submenu.style.marginLeft = '0';
        submenu.style.marginRight = '4px';
    } else if (submenu) {
        submenu.style.left = '100%';
        submenu.style.right = 'auto';
        submenu.style.marginLeft = '4px';
        submenu.style.marginRight = '0';
    }

    // 点击外部关闭菜单
    const closeMenu = (e) => {
        // 检查点击是否在主菜单或子菜单内
        const moveToGroupSubmenuEl = document.getElementById('move-to-group-submenu');
        const clickedInMenu = menu.contains(e.target);
        const clickedInSubmenu = moveToGroupSubmenuEl && moveToGroupSubmenuEl.contains(e.target);
        
        if (!clickedInMenu && !clickedInSubmenu) {
            // 使用 closeContextMenu 确保同时关闭主菜单和子菜单
            closeContextMenu();
            document.removeEventListener('click', closeMenu);
        }
    };
    setTimeout(() => {
        document.addEventListener('click', closeMenu);
    }, 0);
}

// 显示分组上下文菜单
async function showGroupContextMenu(event, groupId) {
    const menu = document.getElementById('group-context-menu');
    if (!menu) return;

    contextMenuGroupId = groupId;

    // 先获取分组的置顶状态并更新菜单文本（在显示菜单之前）
    try {
        // 先从缓存中查找
        let group = groupsCache.find(g => g.id === groupId);
        let isPinned = false;
        
        if (group) {
            isPinned = group.pinned || false;
        } else {
            // 如果缓存中没有，从API获取
            const response = await apiFetch(`/api/groups/${groupId}`);
            if (response.ok) {
                group = await response.json();
                isPinned = group.pinned || false;
            }
        }
        
        // 更新菜单文本
        const pinMenuText = document.getElementById('pin-group-menu-text');
        if (pinMenuText) {
            pinMenuText.textContent = isPinned ? '取消置顶' : '置顶此分组';
        }
    } catch (error) {
        console.error('获取分组置顶状态失败:', error);
        // 如果获取失败，使用默认文本
        const pinMenuText = document.getElementById('pin-group-menu-text');
        if (pinMenuText) {
            pinMenuText.textContent = '置顶此分组';
        }
    }

    // 在状态获取完成后再显示菜单
    menu.style.display = 'block';
    menu.style.visibility = 'visible';
    menu.style.opacity = '1';
    
    // 强制重排以获取正确尺寸
    void menu.offsetHeight;
    
    // 计算菜单位置，确保不超出屏幕
    const menuRect = menu.getBoundingClientRect();
    const viewportWidth = window.innerWidth;
    const viewportHeight = window.innerHeight;
    
    let left = event.clientX;
    let top = event.clientY;
    
    // 如果菜单会超出右边界，调整到左侧
    if (left + menuRect.width > viewportWidth) {
        left = event.clientX - menuRect.width;
    }
    
    // 如果菜单会超出下边界，调整到上方
    if (top + menuRect.height > viewportHeight) {
        top = event.clientY - menuRect.height;
    }
    
    // 确保不超出左边界
    if (left < 0) {
        left = 8;
    }
    
    // 确保不超出上边界
    if (top < 0) {
        top = 8;
    }
    
    menu.style.left = left + 'px';
    menu.style.top = top + 'px';

    // 点击外部关闭菜单
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

// 重命名对话
async function renameConversation() {
    const convId = contextMenuConversationId;
    if (!convId) return;

    const newTitle = prompt('请输入新标题:', '');
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
            throw new Error(error.error || '更新失败');
        }

        // 更新前端显示
        const item = document.querySelector(`[data-conversation-id="${convId}"]`);
        if (item) {
            const titleEl = item.querySelector('.conversation-title');
            if (titleEl) {
                titleEl.textContent = newTitle.trim();
            }
        }

        // 如果在分组详情页，也需要更新
        const groupItem = document.querySelector(`.group-conversation-item[data-conversation-id="${convId}"]`);
        if (groupItem) {
            const groupTitleEl = groupItem.querySelector('.group-conversation-title');
            if (groupTitleEl) {
                groupTitleEl.textContent = newTitle.trim();
            }
        }

        // 重新加载对话列表
        loadConversationsWithGroups();
    } catch (error) {
        console.error('重命名对话失败:', error);
        alert('重命名失败: ' + (error.message || '未知错误'));
    }

    closeContextMenu();
}

// 置顶对话
async function pinConversation() {
    const convId = contextMenuConversationId;
    if (!convId) return;

    try {
        // 检查对话是否真的在当前分组中
        // 如果对话已经从分组移出，conversationGroupMappingCache 中不会有该对话的映射
        // 或者映射的分组ID不等于当前分组ID
        const conversationGroupId = conversationGroupMappingCache[convId];
        const isInCurrentGroup = currentGroupId && conversationGroupId === currentGroupId;
        
        // 如果当前在分组详情页面，且对话确实在当前分组中，使用分组内置顶
        if (isInCurrentGroup) {
            // 获取当前对话在分组中的置顶状态
            const response = await apiFetch(`/api/groups/${currentGroupId}/conversations`);
            const groupConvs = await response.json();
            const conv = groupConvs.find(c => c.id === convId);
            
            // 如果找不到对话，说明可能有问题，使用默认值
            const currentPinned = conv && conv.groupPinned !== undefined ? conv.groupPinned : false;
            const newPinned = !currentPinned;

            // 更新分组内置顶状态
            await apiFetch(`/api/groups/${currentGroupId}/conversations/${convId}/pinned`, {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ pinned: newPinned }),
            });

            // 重新加载分组对话
            loadGroupConversations(currentGroupId);
        } else {
            // 不在分组详情页面，或者对话不在当前分组中，使用全局置顶
            const response = await apiFetch(`/api/conversations/${convId}`);
            const conv = await response.json();
            const newPinned = !conv.pinned;

            // 更新全局置顶状态
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
        console.error('置顶对话失败:', error);
        alert('置顶失败: ' + (error.message || '未知错误'));
    }

    closeContextMenu();
}

// 显示移动到分组子菜单
async function showMoveToGroupSubmenu() {
    const submenu = document.getElementById('move-to-group-submenu');
    if (!submenu) return;

    // 如果子菜单已经显示，不需要重复渲染
    if (submenuVisible && submenu.style.display === 'block') {
        return;
    }

    // 如果正在加载中，避免重复调用
    if (submenuLoading) {
        return;
    }

    // 清除隐藏定时器
    clearSubmenuHideTimeout();
    
    // 标记为加载中
    submenuLoading = true;
    submenu.innerHTML = '';

    // 确保分组列表已加载 - 强制重新加载以确保数据是最新的
    try {
        // 如果缓存为空，强制加载
        if (!Array.isArray(groupsCache) || groupsCache.length === 0) {
            await loadGroups();
        } else {
            // 即使缓存不为空，也尝试刷新一次，确保数据是最新的
            // 但使用静默方式，不显示错误
            try {
                const response = await apiFetch('/api/groups');
                if (response.ok) {
                    const freshGroups = await response.json();
                    if (Array.isArray(freshGroups)) {
                        groupsCache = freshGroups;
                    }
                }
            } catch (err) {
                // 如果刷新失败，使用缓存的数据
                console.warn('刷新分组列表失败，使用缓存数据:', err);
            }
        }
        
        // 再次验证缓存
        if (!Array.isArray(groupsCache)) {
            console.warn('groupsCache 不是有效数组，重置为空数组');
            groupsCache = [];
            // 如果仍然无效，尝试重新加载
            if (groupsCache.length === 0) {
                await loadGroups();
            }
        }
    } catch (error) {
        console.error('加载分组列表失败:', error);
        // 即使加载失败，也继续显示菜单，使用现有缓存
    }

    // 如果当前在分组详情页面，显示"移出本组"选项
    if (currentGroupId && contextMenuConversationId) {
        // 检查对话是否在当前分组中
        const convInGroup = conversationGroupMappingCache[contextMenuConversationId] === currentGroupId;
        if (convInGroup) {
            const removeItem = document.createElement('div');
            removeItem.className = 'context-submenu-item';
            removeItem.innerHTML = `
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                    <path d="M9 12l6 6M15 12l-6 6" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
                <span>移出本组</span>
            `;
            removeItem.onclick = () => {
                removeConversationFromGroup(contextMenuConversationId, currentGroupId);
            };
            submenu.appendChild(removeItem);
            
            // 添加分隔线
            const divider = document.createElement('div');
            divider.className = 'context-menu-divider';
            submenu.appendChild(divider);
        }
    }

    // 验证 groupsCache 是否为有效数组
    if (!Array.isArray(groupsCache)) {
        console.warn('groupsCache 不是有效数组，重置为空数组');
        groupsCache = [];
    }

    // 如果有分组，显示所有分组（排除对话已所在的分组）
    if (groupsCache.length > 0) {
        // 检查对话当前所在的分组ID
        const conversationCurrentGroupId = contextMenuConversationId 
            ? conversationGroupMappingCache[contextMenuConversationId] 
            : null;
        
        groupsCache.forEach(group => {
            // 验证分组对象是否有效
            if (!group || !group.id || !group.name) {
                console.warn('无效的分组对象:', group);
                return;
            }
            
            // 如果对话已经在当前分组中，不显示该分组（因为已经在里面了）
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
        // 如果仍然没有分组，记录日志以便调试
        console.warn('showMoveToGroupSubmenu: groupsCache 为空，无法显示分组列表');
    }

    // 始终显示"创建分组"选项
    const addItem = document.createElement('div');
    addItem.className = 'context-submenu-item add-group-item';
    addItem.innerHTML = `
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
            <path d="M12 5v14M5 12h14" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
        <span>+ 新增分组</span>
    `;
    addItem.onclick = () => {
        showCreateGroupModal(true);
    };
    submenu.appendChild(addItem);

    submenu.style.display = 'block';
    submenuVisible = true;
    submenuLoading = false;
    
    // 计算子菜单位置，防止溢出
    setTimeout(() => {
        const submenuRect = submenu.getBoundingClientRect();
        const viewportWidth = window.innerWidth;
        const viewportHeight = window.innerHeight;
        
        // 如果子菜单超出右边界，调整到左侧
        if (submenuRect.right > viewportWidth) {
            submenu.style.left = 'auto';
            submenu.style.right = '100%';
            submenu.style.marginLeft = '0';
            submenu.style.marginRight = '4px';
        }
        
        // 如果子菜单超出下边界，调整位置
        if (submenuRect.bottom > viewportHeight) {
            const overflow = submenuRect.bottom - viewportHeight;
            const currentTop = parseInt(submenu.style.top) || 0;
            submenu.style.top = (currentTop - overflow - 8) + 'px';
        }
    }, 0);
}

// 隐藏移动到分组子菜单的定时器
let submenuHideTimeout = null;
// 显示子菜单的防抖定时器
let submenuShowTimeout = null;
// 子菜单是否正在加载中
let submenuLoading = false;
// 子菜单是否已显示
let submenuVisible = false;

// 隐藏移动到分组子菜单
function hideMoveToGroupSubmenu() {
    const submenu = document.getElementById('move-to-group-submenu');
    if (submenu) {
        submenu.style.display = 'none';
        submenuVisible = false;
    }
}

// 清除隐藏子菜单的定时器
function clearSubmenuHideTimeout() {
    if (submenuHideTimeout) {
        clearTimeout(submenuHideTimeout);
        submenuHideTimeout = null;
    }
}

// 清除显示子菜单的定时器
function clearSubmenuShowTimeout() {
    if (submenuShowTimeout) {
        clearTimeout(submenuShowTimeout);
        submenuShowTimeout = null;
    }
}

// 处理鼠标进入"移动到分组"菜单项（带防抖）
function handleMoveToGroupSubmenuEnter() {
    // 清除隐藏定时器
    clearSubmenuHideTimeout();
    
    // 如果子菜单已经显示，不需要重复调用
    const submenu = document.getElementById('move-to-group-submenu');
    if (submenu && submenuVisible && submenu.style.display === 'block') {
        return;
    }
    
    // 清除之前的显示定时器
    clearSubmenuShowTimeout();
    
    // 使用防抖延迟显示，避免频繁触发
    submenuShowTimeout = setTimeout(() => {
        showMoveToGroupSubmenu();
        submenuShowTimeout = null;
    }, 100);
}

// 处理鼠标离开"移动到分组"菜单项
function handleMoveToGroupSubmenuLeave(event) {
    const submenu = document.getElementById('move-to-group-submenu');
    if (!submenu) return;
    
    // 清除显示定时器
    clearSubmenuShowTimeout();
    
    // 检查鼠标是否移动到子菜单
    const relatedTarget = event.relatedTarget;
    if (relatedTarget && submenu.contains(relatedTarget)) {
        // 鼠标移动到子菜单，不清除
        return;
    }
    
    // 清除之前的隐藏定时器
    clearSubmenuHideTimeout();
    
    // 延迟隐藏，给用户时间移动到子菜单
    submenuHideTimeout = setTimeout(() => {
        hideMoveToGroupSubmenu();
        submenuHideTimeout = null;
    }, 200);
}

// 移动对话到分组
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

        // 更新缓存
        const oldGroupId = conversationGroupMappingCache[convId];
        conversationGroupMappingCache[convId] = groupId;
        
        // 将新移动的对话添加到待保留映射中，防止后端API延迟导致映射丢失
        pendingGroupMappings[convId] = groupId;
        
        // 如果移动的是当前对话，更新 currentConversationGroupId
        if (currentConversationId === convId) {
            currentConversationGroupId = groupId;
        }
        
        // 如果当前在分组详情页面，重新加载分组对话
        if (currentGroupId) {
            // 如果从当前分组移出，或者移动到当前分组，都需要重新加载
            if (currentGroupId === oldGroupId || currentGroupId === groupId) {
                await loadGroupConversations(currentGroupId);
            }
        }
        
        // 无论是否在分组详情页面，都需要刷新最近对话列表
        // 因为最近对话列表会根据分组映射缓存来过滤显示，需要立即更新
        // loadConversationsWithGroups 内部会调用 loadConversationGroupMapping，
        // loadConversationGroupMapping 会保留 pendingGroupMappings 中的映射
        await loadConversationsWithGroups();
        
        // 注意：pendingGroupMappings 中的映射会在下次 loadConversationGroupMapping 
        // 成功从后端加载时自动清理（在 loadConversationGroupMapping 中处理）
        
        // 刷新分组列表，更新高亮状态
        await loadGroups();
    } catch (error) {
        console.error('移动对话到分组失败:', error);
        alert('移动失败: ' + (error.message || '未知错误'));
    }

    closeContextMenu();
}

// 从分组中移除对话
async function removeConversationFromGroup(convId, groupId) {
    try {
        await apiFetch(`/api/groups/${groupId}/conversations/${convId}`, {
            method: 'DELETE',
        });

        // 更新缓存 - 立即删除，确保后续加载时能正确识别
        delete conversationGroupMappingCache[convId];
        // 同时从待保留映射中移除
        delete pendingGroupMappings[convId];
        
        // 如果移除的是当前对话，清除 currentConversationGroupId
        if (currentConversationId === convId) {
            currentConversationGroupId = null;
        }
        
        // 如果当前在分组详情页面，重新加载分组对话
        if (currentGroupId === groupId) {
            await loadGroupConversations(groupId);
        }
        
        // 重新加载分组映射，确保缓存是最新的
        await loadConversationGroupMapping();
        
        // 刷新分组列表，更新高亮状态
        await loadGroups();
        
        // 刷新最近对话列表，让移出的对话立即显示
        // 使用临时变量保存 currentGroupId，然后临时设置为 null，确保显示所有不在分组的对话
        const savedGroupId = currentGroupId;
        currentGroupId = null;
        await loadConversationsWithGroups();
        currentGroupId = savedGroupId;
    } catch (error) {
        console.error('从分组中移除对话失败:', error);
        alert('移除失败: ' + (error.message || '未知错误'));
    }

    closeContextMenu();
}

// 加载对话分组映射
async function loadConversationGroupMapping() {
    try {
        // 获取所有分组，然后获取每个分组的对话
        let groups;
        if (Array.isArray(groupsCache) && groupsCache.length > 0) {
            groups = groupsCache;
        } else {
            const response = await apiFetch('/api/groups');
            if (!response.ok) {
                // 如果API请求失败，使用空数组，不打印警告（这是正常错误处理）
                groups = [];
            } else {
                groups = await response.json();
                // 确保groups是有效数组，只在真正异常时才打印警告
                if (!Array.isArray(groups)) {
                    // 只在返回的不是数组且不是null/undefined时才打印警告（可能是后端返回了错误格式）
                    if (groups !== null && groups !== undefined) {
                        console.warn('loadConversationGroupMapping: groups不是有效数组，使用空数组', groups);
                    }
                    groups = [];
                }
            }
        }
        
        // 保存待保留的映射
        const preservedMappings = { ...pendingGroupMappings };
        
        conversationGroupMappingCache = {};

        for (const group of groups) {
            const response = await apiFetch(`/api/groups/${group.id}/conversations`);
            const conversations = await response.json();
            // 确保conversations是有效数组
            if (Array.isArray(conversations)) {
                conversations.forEach(conv => {
                    conversationGroupMappingCache[conv.id] = group.id;
                    // 如果这个对话在待保留映射中，从待保留映射中移除（因为已经从后端加载了）
                    if (preservedMappings[conv.id] === group.id) {
                        delete pendingGroupMappings[conv.id];
                    }
                });
            }
        }
        
        // 恢复待保留的映射（这些是后端API尚未同步的映射）
        Object.assign(conversationGroupMappingCache, preservedMappings);
    } catch (error) {
        console.error('加载对话分组映射失败:', error);
    }
}

// 从上下文菜单删除对话
function deleteConversationFromContext() {
    const convId = contextMenuConversationId;
    if (!convId) return;

    if (confirm('确定要删除此对话吗？')) {
        deleteConversation(convId, true); // 跳过内部确认，因为这里已经确认过了
    }
    closeContextMenu();
}

// 关闭上下文菜单
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
    // 清除所有定时器
    clearSubmenuHideTimeout();
    clearSubmenuShowTimeout();
    submenuLoading = false;
    contextMenuConversationId = null;
}

// 显示批量管理模态框
let allConversationsForBatch = [];

async function showBatchManageModal() {
    try {
        const response = await apiFetch('/api/conversations?limit=1000');
        allConversationsForBatch = await response.json();

        const modal = document.getElementById('batch-manage-modal');
        const countEl = document.getElementById('batch-manage-count');
        if (countEl) {
            countEl.textContent = allConversationsForBatch.length;
        }

        renderBatchConversations();
        if (modal) {
            modal.style.display = 'flex';
        }
    } catch (error) {
        console.error('加载对话列表失败:', error);
        alert('加载对话列表失败');
    }
}

// 安全截断中文字符串，避免在汉字中间截断
function safeTruncateText(text, maxLength = 50) {
    if (!text || typeof text !== 'string') {
        return text || '';
    }
    
    // 使用 Array.from 将字符串转换为字符数组（正确处理 Unicode 代理对）
    const chars = Array.from(text);
    
    // 如果文本长度未超过限制，直接返回
    if (chars.length <= maxLength) {
        return text;
    }
    
    // 截断到最大长度（基于字符数，而不是代码单元）
    let truncatedChars = chars.slice(0, maxLength);
    
    // 尝试在标点符号或空格处截断，使截断更自然
    // 在截断点往前查找合适的断点（不超过20%的长度）
    const searchRange = Math.floor(maxLength * 0.2);
    const breakChars = ['，', '。', '、', ' ', ',', '.', ';', ':', '!', '?', '！', '？', '/', '\\', '-', '_'];
    let bestBreakPos = truncatedChars.length;
    
    for (let i = truncatedChars.length - 1; i >= truncatedChars.length - searchRange && i >= 0; i--) {
        if (breakChars.includes(truncatedChars[i])) {
            bestBreakPos = i + 1; // 在标点符号后断开
            break;
        }
    }
    
    // 如果找到合适的断点，使用它；否则使用原截断位置
    if (bestBreakPos < truncatedChars.length) {
        truncatedChars = truncatedChars.slice(0, bestBreakPos);
    }
    
    // 将字符数组转换回字符串，并添加省略号
    return truncatedChars.join('') + '...';
}

// 渲染批量管理对话列表
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
        const originalTitle = conv.title || '未命名对话';
        // 使用安全截断函数，限制最大长度为45个字符（留出空间显示省略号）
        const truncatedTitle = safeTruncateText(originalTitle, 45);
        name.textContent = truncatedTitle;
        // 设置title属性以显示完整文本（鼠标悬停时）
        name.title = originalTitle;

        const time = document.createElement('div');
        time.className = 'batch-table-col-time';
        const dateObj = conv.updatedAt ? new Date(conv.updatedAt) : new Date();
        time.textContent = dateObj.toLocaleString('zh-CN', {
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

// 筛选批量管理对话
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

// 全选/取消全选
function toggleSelectAllBatch() {
    const selectAll = document.getElementById('batch-select-all');
    const checkboxes = document.querySelectorAll('.batch-conversation-checkbox');
    
    checkboxes.forEach(cb => {
        cb.checked = selectAll.checked;
    });
}

// 删除选中的对话
async function deleteSelectedConversations() {
    const checkboxes = document.querySelectorAll('.batch-conversation-checkbox:checked');
    if (checkboxes.length === 0) {
        alert('请先选择要删除的对话');
        return;
    }

    if (!confirm(`确定要删除选中的 ${checkboxes.length} 条对话吗？`)) {
        return;
    }

    const ids = Array.from(checkboxes).map(cb => cb.dataset.conversationId);
    
    try {
        for (const id of ids) {
            await deleteConversation(id, true); // 跳过内部确认，因为批量删除时已经确认过了
        }
        closeBatchManageModal();
        loadConversationsWithGroups();
    } catch (error) {
        console.error('删除失败:', error);
        alert('删除失败: ' + (error.message || '未知错误'));
    }
}

// 关闭批量管理模态框
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

// 显示创建分组模态框
function showCreateGroupModal(andMoveConversation = false) {
    const modal = document.getElementById('create-group-modal');
    const input = document.getElementById('create-group-name-input');
    if (input) {
        input.value = '';
    }
    if (modal) {
        modal.style.display = 'flex';
        modal.dataset.moveConversation = andMoveConversation ? 'true' : 'false';
        if (input) {
            setTimeout(() => input.focus(), 100);
        }
    }
}

// 关闭创建分组模态框
function closeCreateGroupModal() {
    const modal = document.getElementById('create-group-modal');
    if (modal) {
        modal.style.display = 'none';
    }
    const input = document.getElementById('create-group-name-input');
    if (input) {
        input.value = '';
    }
}

// 创建分组
async function createGroup(event) {
    // 阻止事件冒泡
    if (event) {
        event.preventDefault();
        event.stopPropagation();
    }

    const input = document.getElementById('create-group-name-input');
    if (!input) {
        console.error('找不到输入框');
        return;
    }

    const name = input.value.trim();
    if (!name) {
        alert('请输入分组名称');
        return;
    }

    // 前端校验：检查名称是否已存在
    try {
        let groups;
        if (Array.isArray(groupsCache) && groupsCache.length > 0) {
            groups = groupsCache;
        } else {
            const response = await apiFetch('/api/groups');
            groups = await response.json();
        }
        
        // 确保groups是有效数组
        if (!Array.isArray(groups)) {
            groups = [];
        }
        
        const nameExists = groups.some(g => g.name === name);
        if (nameExists) {
            alert('分组名称已存在，请使用其他名称');
            return;
        }
    } catch (error) {
        console.error('检查分组名称失败:', error);
    }

    try {
        const response = await apiFetch('/api/groups', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                name: name,
                icon: '📁',
            }),
        });

        if (!response.ok) {
            const error = await response.json();
            if (error.error && error.error.includes('已存在')) {
                alert('分组名称已存在，请使用其他名称');
                return;
            }
            throw new Error(error.error || '创建失败');
        }

        const newGroup = await response.json();
        
        // 检查"移动到分组"子菜单是否打开
        const submenu = document.getElementById('move-to-group-submenu');
        const isSubmenuOpen = submenu && submenu.style.display !== 'none';

        await loadGroups();

        const modal = document.getElementById('create-group-modal');
        const shouldMove = modal && modal.dataset.moveConversation === 'true';
        
        closeCreateGroupModal();

        if (shouldMove && contextMenuConversationId) {
            moveConversationToGroup(contextMenuConversationId, newGroup.id);
        }

        // 如果子菜单是打开的，刷新它，让新创建的分组立即显示
        if (isSubmenuOpen) {
            await showMoveToGroupSubmenu();
        }
    } catch (error) {
        console.error('创建分组失败:', error);
        alert('创建失败: ' + (error.message || '未知错误'));
    }
}

// 进入分组详情
async function enterGroupDetail(groupId) {
    currentGroupId = groupId;
    // 进入分组详情页面时，清除当前对话所属的分组ID，避免高亮冲突
    // 因为此时用户是在查看分组详情，而不是在查看分组中的某个对话
    currentConversationGroupId = null;
    
    try {
        const response = await apiFetch(`/api/groups/${groupId}`);
        const group = await response.json();
        
        if (!group) {
            currentGroupId = null;
            return;
        }

        // 显示分组详情页，隐藏对话界面，但保持侧边栏可见
        const sidebar = document.querySelector('.conversation-sidebar');
        const groupDetailPage = document.getElementById('group-detail-page');
        const chatContainer = document.querySelector('.chat-container');
        const titleEl = document.getElementById('group-detail-title');

        // 保持侧边栏可见
        if (sidebar) sidebar.style.display = 'flex';
        // 隐藏对话界面，显示分组详情页
        if (chatContainer) chatContainer.style.display = 'none';
        if (groupDetailPage) groupDetailPage.style.display = 'flex';
        if (titleEl) titleEl.textContent = group.name;

        // 刷新分组列表，确保当前分组高亮显示
        await loadGroups();

        // 加载分组对话（如果有搜索查询则使用搜索查询）
        loadGroupConversations(groupId, currentGroupSearchQuery);
    } catch (error) {
        console.error('加载分组失败:', error);
        currentGroupId = null;
    }
}

// 退出分组详情
function exitGroupDetail() {
    currentGroupId = null;
    currentGroupSearchQuery = ''; // 清除搜索状态
    
    // 隐藏搜索框并清除搜索内容
    const searchContainer = document.getElementById('group-search-container');
    const searchInput = document.getElementById('group-search-input');
    if (searchContainer) searchContainer.style.display = 'none';
    if (searchInput) searchInput.value = '';
    
    const sidebar = document.querySelector('.conversation-sidebar');
    const groupDetailPage = document.getElementById('group-detail-page');
    const chatContainer = document.querySelector('.chat-container');

    // 保持侧边栏可见
    if (sidebar) sidebar.style.display = 'flex';
    // 隐藏分组详情页，显示对话界面
    if (groupDetailPage) groupDetailPage.style.display = 'none';
    if (chatContainer) chatContainer.style.display = 'flex';

    loadConversationsWithGroups();
}

// 加载分组中的对话
async function loadGroupConversations(groupId, searchQuery = '') {
    try {
        if (!groupId) {
            console.error('loadGroupConversations: groupId is null or undefined');
            return;
        }
        
        // 确保分组映射已加载
        if (Object.keys(conversationGroupMappingCache).length === 0) {
            await loadConversationGroupMapping();
        }
        
        // 先清空列表，避免显示旧数据
        const list = document.getElementById('group-conversations-list');
        if (!list) {
            console.error('group-conversations-list element not found');
            return;
        }
        
        // 显示加载状态
        if (searchQuery) {
            list.innerHTML = '<div style="padding: 40px; text-align: center; color: var(--text-muted);">搜索中...</div>';
        } else {
            list.innerHTML = '<div style="padding: 40px; text-align: center; color: var(--text-muted);">加载中...</div>';
        }

        // 构建URL，如果有搜索关键词则添加search参数
        let url = `/api/groups/${groupId}/conversations`;
        if (searchQuery && searchQuery.trim()) {
            url += '?search=' + encodeURIComponent(searchQuery.trim());
        }
        
        const response = await apiFetch(url);
        if (!response.ok) {
            console.error(`Failed to load conversations for group ${groupId}:`, response.statusText);
            list.innerHTML = '<div style="padding: 40px; text-align: center; color: var(--text-muted);">加载失败，请重试</div>';
            return;
        }
        
        let groupConvs = await response.json();
        
        // 处理 null 或 undefined 的情况，将其视为空数组
        if (!groupConvs) {
            groupConvs = [];
        }
        
        // 验证返回的数据类型
        if (!Array.isArray(groupConvs)) {
            console.error(`Invalid response for group ${groupId}:`, groupConvs);
            list.innerHTML = '<div style="padding: 40px; text-align: center; color: var(--text-muted);">数据格式错误</div>';
            return;
        }
        
        // 更新分组映射缓存（只更新当前分组的对话）
        // 先清理该分组之前的映射（如果有对话被移出）
        Object.keys(conversationGroupMappingCache).forEach(convId => {
            if (conversationGroupMappingCache[convId] === groupId) {
                // 如果这个对话不在新的列表中，说明已被移出
                if (!groupConvs.find(c => c.id === convId)) {
                    delete conversationGroupMappingCache[convId];
                }
            }
        });
        
        // 更新当前分组的对话映射
        groupConvs.forEach(conv => {
            conversationGroupMappingCache[conv.id] = groupId;
        });

        // 再次清空列表（清除"加载中"提示）
        list.innerHTML = '';

        if (groupConvs.length === 0) {
            if (searchQuery && searchQuery.trim()) {
                list.innerHTML = '<div style="padding: 40px; text-align: center; color: var(--text-muted);">未找到匹配的对话</div>';
            } else {
                list.innerHTML = '<div style="padding: 40px; text-align: center; color: var(--text-muted);">该分组暂无对话</div>';
            }
            return;
        }

        // 加载每个对话的详细信息以获取消息
        for (const conv of groupConvs) {
            try {
                // 验证对话ID存在
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
                // 只有在分组详情页面且对话ID匹配时才显示active状态
                // 如果不在分组详情页面，不应该显示active状态
                if (currentGroupId && conv.id === currentConversationId) {
                    item.classList.add('active');
                } else {
                    item.classList.remove('active');
                }

                // 创建内容包装器
                const contentWrapper = document.createElement('div');
                contentWrapper.className = 'group-conversation-content-wrapper';

                const titleWrapper = document.createElement('div');
                titleWrapper.style.display = 'flex';
                titleWrapper.style.alignItems = 'center';
                titleWrapper.style.gap = '4px';

                const title = document.createElement('div');
                title.className = 'group-conversation-title';
                const titleText = fullConv.title || conv.title || '未命名对话';
                title.textContent = safeTruncateText(titleText, 60);
                title.title = titleText; // 设置完整标题以便悬停查看
                titleWrapper.appendChild(title);

                // 如果对话在分组中置顶，显示置顶图标
                if (conv.groupPinned) {
                    const pinIcon = document.createElement('span');
                    pinIcon.className = 'conversation-item-pinned';
                    pinIcon.innerHTML = '📌';
                    pinIcon.title = '在分组中已置顶';
                    titleWrapper.appendChild(pinIcon);
                }

                contentWrapper.appendChild(titleWrapper);

                const timeWrapper = document.createElement('div');
                timeWrapper.className = 'group-conversation-time';
                const dateObj = fullConv.updatedAt ? new Date(fullConv.updatedAt) : new Date();
                timeWrapper.textContent = dateObj.toLocaleString('zh-CN', {
                    year: 'numeric',
                    month: 'long',
                    day: 'numeric',
                    hour: '2-digit',
                    minute: '2-digit'
                });

                contentWrapper.appendChild(timeWrapper);

                // 如果有第一条消息，显示内容预览
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

                // 添加三个点菜单按钮
                const menuBtn = document.createElement('button');
                menuBtn.className = 'conversation-item-menu';
                menuBtn.innerHTML = '⋯';
                menuBtn.onclick = (e) => {
                    e.stopPropagation();
                    contextMenuConversationId = conv.id;
                    showConversationContextMenu(e);
                };
                item.appendChild(menuBtn);

                item.onclick = () => {
                    // 切换到对话界面，但保持分组详情状态
                    const groupDetailPage = document.getElementById('group-detail-page');
                    const chatContainer = document.querySelector('.chat-container');
                    if (groupDetailPage) groupDetailPage.style.display = 'none';
                    if (chatContainer) chatContainer.style.display = 'flex';
                    loadConversation(conv.id);
                };

                list.appendChild(item);
            } catch (err) {
                console.error(`加载对话 ${conv.id} 失败:`, err);
            }
        }
    } catch (error) {
        console.error('加载分组对话失败:', error);
    }
}

// 编辑分组
async function editGroup() {
    if (!currentGroupId) return;

    try {
        const response = await apiFetch(`/api/groups/${currentGroupId}`);
        const group = await response.json();
        if (!group) return;

        const newName = prompt('请输入新名称:', group.name);
        if (newName === null || !newName.trim()) return;

        const trimmedName = newName.trim();
        
        // 前端校验：检查名称是否已存在（排除当前分组）
        let groups;
        if (Array.isArray(groupsCache) && groupsCache.length > 0) {
            groups = groupsCache;
        } else {
            const response = await apiFetch('/api/groups');
            groups = await response.json();
        }
        
        // 确保groups是有效数组
        if (!Array.isArray(groups)) {
            groups = [];
        }
        
        const nameExists = groups.some(g => g.name === trimmedName && g.id !== currentGroupId);
        if (nameExists) {
            alert('分组名称已存在，请使用其他名称');
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
            if (error.error && error.error.includes('已存在')) {
                alert('分组名称已存在，请使用其他名称');
                return;
            }
            throw new Error(error.error || '更新失败');
        }

        loadGroups();
        
        const titleEl = document.getElementById('group-detail-title');
        if (titleEl) {
            titleEl.textContent = trimmedName;
        }
    } catch (error) {
        console.error('编辑分组失败:', error);
        alert('编辑失败: ' + (error.message || '未知错误'));
    }
}

// 删除分组
async function deleteGroup() {
    if (!currentGroupId) return;

    if (!confirm('确定要删除此分组吗？分组中的对话不会被删除，但会从分组中移除。')) {
        return;
    }

    try {
        await apiFetch(`/api/groups/${currentGroupId}`, {
            method: 'DELETE',
        });

        // 更新缓存
        groupsCache = groupsCache.filter(g => g.id !== currentGroupId);
        Object.keys(conversationGroupMappingCache).forEach(convId => {
            if (conversationGroupMappingCache[convId] === currentGroupId) {
                delete conversationGroupMappingCache[convId];
            }
        });

        // 如果"移动到分组"子菜单是打开的，刷新它
        const submenu = document.getElementById('move-to-group-submenu');
        if (submenu && submenu.style.display !== 'none') {
            // 子菜单是打开的，重新加载分组列表并刷新子菜单
            await loadGroups();
            await showMoveToGroupSubmenu();
        } else {
            exitGroupDetail();
            await loadGroups();
        }
        
        // 刷新对话列表，确保之前被分组的对话能立即显示
        await loadConversationsWithGroups();
    } catch (error) {
        console.error('删除分组失败:', error);
        alert('删除失败: ' + (error.message || '未知错误'));
    }
}

// 从上下文菜单重命名分组
async function renameGroupFromContext() {
    const groupId = contextMenuGroupId;
    if (!groupId) return;

    try {
        const response = await apiFetch(`/api/groups/${groupId}`);
        const group = await response.json();
        if (!group) return;

        const newName = prompt('请输入新名称:', group.name);
        if (newName === null || !newName.trim()) {
            closeGroupContextMenu();
            return;
        }

        const trimmedName = newName.trim();
        
        // 前端校验：检查名称是否已存在（排除当前分组）
        let groups;
        if (Array.isArray(groupsCache) && groupsCache.length > 0) {
            groups = groupsCache;
        } else {
            const response = await apiFetch('/api/groups');
            groups = await response.json();
        }
        
        // 确保groups是有效数组
        if (!Array.isArray(groups)) {
            groups = [];
        }
        
        const nameExists = groups.some(g => g.name === trimmedName && g.id !== groupId);
        if (nameExists) {
            alert('分组名称已存在，请使用其他名称');
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
            if (error.error && error.error.includes('已存在')) {
                alert('分组名称已存在，请使用其他名称');
                return;
            }
            throw new Error(error.error || '更新失败');
        }

        loadGroups();
        
        // 如果当前在分组详情页，更新标题
        if (currentGroupId === groupId) {
            const titleEl = document.getElementById('group-detail-title');
            if (titleEl) {
                titleEl.textContent = trimmedName;
            }
        }
    } catch (error) {
        console.error('重命名分组失败:', error);
        alert('重命名失败: ' + (error.message || '未知错误'));
    }

    closeGroupContextMenu();
}

// 从上下文菜单置顶分组
async function pinGroupFromContext() {
    const groupId = contextMenuGroupId;
    if (!groupId) return;

    try {
        // 获取当前分组信息
        const response = await apiFetch(`/api/groups/${groupId}`);
        const group = await response.json();
        if (!group) return;

        const newPinnedState = !group.pinned;

        // 调用 API 更新置顶状态
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
            throw new Error(error.error || '更新失败');
        }

        // 重新加载分组列表以更新显示顺序
        loadGroups();
    } catch (error) {
        console.error('置顶分组失败:', error);
        alert('置顶失败: ' + (error.message || '未知错误'));
    }

    closeGroupContextMenu();
}

// 从上下文菜单删除分组
async function deleteGroupFromContext() {
    const groupId = contextMenuGroupId;
    if (!groupId) return;

    if (!confirm('确定要删除此分组吗？分组中的对话不会被删除，但会从分组中移除。')) {
        closeGroupContextMenu();
        return;
    }

    try {
        await apiFetch(`/api/groups/${groupId}`, {
            method: 'DELETE',
        });

        // 更新缓存
        groupsCache = groupsCache.filter(g => g.id !== groupId);
        Object.keys(conversationGroupMappingCache).forEach(convId => {
            if (conversationGroupMappingCache[convId] === groupId) {
                delete conversationGroupMappingCache[convId];
            }
        });

        // 如果"移动到分组"子菜单是打开的，刷新它
        const submenu = document.getElementById('move-to-group-submenu');
        if (submenu && submenu.style.display !== 'none') {
            // 子菜单是打开的，重新加载分组列表并刷新子菜单
            await loadGroups();
            await showMoveToGroupSubmenu();
        } else {
            // 如果当前在分组详情页，退出详情页
            if (currentGroupId === groupId) {
                exitGroupDetail();
            }
            await loadGroups();
        }
        
        // 刷新对话列表，确保之前被分组的对话能立即显示
        await loadConversationsWithGroups();
    } catch (error) {
        console.error('删除分组失败:', error);
        alert('删除失败: ' + (error.message || '未知错误'));
    }

    closeGroupContextMenu();
}

// 关闭分组上下文菜单
function closeGroupContextMenu() {
    const menu = document.getElementById('group-context-menu');
    if (menu) {
        menu.style.display = 'none';
    }
    contextMenuGroupId = null;
}


// 分组搜索相关变量
let groupSearchTimer = null;
let currentGroupSearchQuery = '';

// 切换分组搜索框显示/隐藏
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

// 处理分组搜索输入
function handleGroupSearchInput(event) {
    // 支持回车键搜索
    if (event.key === 'Enter') {
        event.preventDefault();
        performGroupSearch();
        return;
    }
    
    // 支持ESC键关闭搜索
    if (event.key === 'Escape') {
        clearGroupSearch();
        toggleGroupSearch();
        return;
    }
    
    const searchInput = document.getElementById('group-search-input');
    const clearBtn = document.getElementById('group-search-clear-btn');
    
    if (!searchInput) return;
    
    const query = searchInput.value.trim();
    
    // 显示/隐藏清除按钮
    if (clearBtn) {
        clearBtn.style.display = query ? 'block' : 'none';
    }
    
    // 防抖搜索
    if (groupSearchTimer) {
        clearTimeout(groupSearchTimer);
    }
    
    groupSearchTimer = setTimeout(() => {
        performGroupSearch();
    }, 300); // 300ms 防抖
}

// 执行分组搜索
async function performGroupSearch() {
    const searchInput = document.getElementById('group-search-input');
    if (!searchInput || !currentGroupId) return;
    
    const query = searchInput.value.trim();
    currentGroupSearchQuery = query;
    
    // 加载搜索结果
    await loadGroupConversations(currentGroupId, query);
}

// 清除分组搜索
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
    
    // 重新加载分组对话（不搜索）
    if (currentGroupId) {
        loadGroupConversations(currentGroupId, '');
    }
}

// 初始化时加载分组
document.addEventListener('DOMContentLoaded', async () => {
    await loadGroups();
    // 替换原来的loadConversations调用
    if (typeof loadConversations === 'function') {
        // 保留原函数，但使用新函数
        const originalLoad = loadConversations;
        loadConversations = function(...args) {
            loadConversationsWithGroups(...args);
        };
    }
    await loadConversationsWithGroups();
});
