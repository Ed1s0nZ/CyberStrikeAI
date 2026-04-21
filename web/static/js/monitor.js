const progressTaskState = new Map();
let activeTaskInterval = null;
const ACTIVE_TASK_REFRESH_INTERVAL = 10000; // 10
const TASK_FINAL_STATUSES = new Set(['failed', 'timeout', 'cancelled', 'completed']);

// BCP 47 ()
function getCurrentTimeLocale() {
    if (typeof window.__locale === 'string' && window.__locale.length) {
        return window.__locale.startsWith('zh') ? 'zh-CN' : 'en-US';
    }
    if (typeof i18next !== 'undefined' && i18next.language) {
        return (i18next.language || '').startsWith('zh') ? 'zh-CN' : 'en-US';
    }
    return 'zh-CN';
}

// toLocaleTimeString : 24 , AM/PM
function getTimeFormatOptions() {
    const loc = getCurrentTimeLocale();
    const base = { hour: '2-digit', minute: '2-digit', second: '2-digit' };
    if (loc === 'zh-CN') {
        base.hour12 = false;
    }
    return base;
}

// (,)
function translateProgressMessage(message) {
    if (!message || typeof message !== 'string') return message;
    if (typeof window.t !== 'function') return message;
    const trim = message.trim();
    const map = {
 // 
 'AI...': 'progress.callingAI',
 ':...': 'progress.lastIterSummary',
 '': 'progress.summaryDone',
 '...': 'progress.generatingFinalReply',
 ',...': 'progress.maxIterSummary',
 '...': 'progress.analyzingRequestShort',
 '': 'progress.analyzingRequestPlanning',
 ' Eino DeepAgent...': 'progress.startingEinoDeepAgent',
 // ( en-US.json ,/)
        'Calling AI model...': 'progress.callingAI',
        'Last iteration: generating summary and next steps...': 'progress.lastIterSummary',
        'Summary complete': 'progress.summaryDone',
        'Generating final reply...': 'progress.generatingFinalReply',
        'Max iterations reached, generating summary...': 'progress.maxIterSummary',
        'Analyzing your request...': 'progress.analyzingRequestShort',
        'Analyzing your request and planning test strategy...': 'progress.analyzingRequestPlanning',
        'Starting Eino DeepAgent...': 'progress.startingEinoDeepAgent'
    };
    if (map[trim]) return window.t(map[trim]);
    const einoAgentRe = /^\[Eino\]\s*(.+)$/;
    const einoM = trim.match(einoAgentRe);
    if (einoM) {
        return window.t('progress.einoAgent', { name: einoM[1] });
    }
 const callingToolPrefixCn = ': ';
    const callingToolPrefixEn = 'Calling tool: ';
    if (trim.indexOf(callingToolPrefixCn) === 0) {
        const name = trim.slice(callingToolPrefixCn.length);
        return window.t('progress.callingTool', { name: name });
    }
    if (trim.indexOf(callingToolPrefixEn) === 0) {
        const name = trim.slice(callingToolPrefixEn.length);
        return window.t('progress.callingTool', { name: name });
    }
    return message;
}
if (typeof window !== 'undefined') {
    window.translateProgressMessage = translateProgressMessage;
}

// IDDOM,
const toolCallStatusMap = new Map();

// :progressId -> { assistantId, buffer }
const responseStreamStateByProgressId = new Map();

// AI :progressId -> Map(streamId -> { itemId, buffer })
const thinkingStreamStateByProgressId = new Map();

// Eino :progressId -> Map(streamId -> { itemId, buffer })
const einoAgentReplyStreamStateByProgressId = new Map();

// :progressId::toolCallId -> { itemId, buffer }
const toolResultStreamStateByKey = new Map();
function toolResultStreamKey(progressId, toolCallId) {
    return String(progressId) + '::' + String(toolCallId);
}

/** Eino : [agentId],// */
function timelineAgentBracketPrefix(data) {
    if (!data || data.einoAgent == null) return '';
    const s = String(data.einoAgent).trim();
    return s ? ('[' + s + '] ') : '';
}

/** /:(/) */
function applyEinoTimelineRole(item, data) {
    if (!item || !data) return;
    const role = data.einoRole;
    if (role === 'orchestrator' || role === 'sub') {
        item.dataset.einoRole = role;
        item.classList.add('timeline-eino-role-' + role);
    }
    const scope = data.einoScope;
    if (scope === 'main' || scope === 'sub') {
        item.dataset.einoScope = scope;
        item.classList.add('timeline-eino-scope-' + scope);
    }
}

// markdown (;)
const assistantMarkdownSanitizeConfig = {
    ALLOWED_TAGS: ['p', 'br', 'strong', 'em', 'u', 's', 'code', 'pre', 'blockquote', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'ul', 'ol', 'li', 'a', 'img', 'table', 'thead', 'tbody', 'tr', 'th', 'td', 'hr'],
    ALLOWED_ATTR: ['href', 'title', 'alt', 'src', 'class'],
    ALLOW_DATA_ATTR: false,
};

function escapeHtmlLocal(text) {
    if (!text) return '';
    const div = document.createElement('div');
    div.textContent = String(text);
    return div.innerHTML;
}

function formatAssistantMarkdownContent(text) {
    const raw = text == null ? '' : String(text);
    if (typeof marked !== 'undefined') {
        try {
            marked.setOptions({ breaks: true, gfm: true });
            const parsed = marked.parse(raw);
            if (typeof DOMPurify !== 'undefined') {
                return DOMPurify.sanitize(parsed, assistantMarkdownSanitizeConfig);
            }
            return parsed;
        } catch (e) {
            return escapeHtmlLocal(raw).replace(/\n/g, '<br>');
        }
    }
    return escapeHtmlLocal(raw).replace(/\n/g, '<br>');
}

function updateAssistantBubbleContent(assistantMessageId, content, renderMarkdown) {
    const assistantElement = document.getElementById(assistantMessageId);
    if (!assistantElement) return;
    const bubble = assistantElement.querySelector('.message-bubble');
    if (!bubble) return;

 // :addMessage append message-bubble 
    const copyBtn = bubble.querySelector('.message-copy-btn');
    if (copyBtn) copyBtn.remove();

    const newContent = content == null ? '' : String(content);
    const html = renderMarkdown
        ? formatAssistantMarkdownContent(newContent)
        : escapeHtmlLocal(newContent).replace(/\n/g, '<br>');

    bubble.innerHTML = html;

 // ()
    assistantElement.dataset.originalContent = newContent;

    if (typeof wrapTablesInBubble === 'function') {
        wrapTablesInBubble(bubble);
    }
    if (copyBtn) bubble.appendChild(copyBtn);
}

const conversationExecutionTracker = {
    activeConversations: new Set(),
    update(tasks = []) {
        this.activeConversations.clear();
        tasks.forEach(task => {
            if (
                task &&
                task.conversationId &&
                !TASK_FINAL_STATUSES.has(task.status)
            ) {
                this.activeConversations.add(task.conversationId);
            }
        });
    },
    isRunning(conversationId) {
        return !!conversationId && this.activeConversations.has(conversationId);
    }
};

function isConversationTaskRunning(conversationId) {
    return conversationExecutionTracker.isRunning(conversationId);
}

/** "";, */
const CHAT_SCROLL_PIN_THRESHOLD_PX = 120;

/** wasPinned DOM , scrollHeight */
function scrollChatMessagesToBottomIfPinned(wasPinned) {
    const messagesDiv = document.getElementById('chat-messages');
    if (!messagesDiv || !wasPinned) return;
    messagesDiv.scrollTop = messagesDiv.scrollHeight;
}

function isChatMessagesPinnedToBottom() {
    const messagesDiv = document.getElementById('chat-messages');
    if (!messagesDiv) return true;
    const { scrollTop, scrollHeight, clientHeight } = messagesDiv;
    return scrollHeight - clientHeight - scrollTop <= CHAT_SCROLL_PIN_THRESHOLD_PX;
}

function registerProgressTask(progressId, conversationId = null) {
    const state = progressTaskState.get(progressId) || {};
    state.conversationId = conversationId !== undefined && conversationId !== null
        ? conversationId
        : (state.conversationId ?? currentConversationId);
    state.cancelling = false;
    progressTaskState.set(progressId, state);

    const progressElement = document.getElementById(progressId);
    if (progressElement) {
        progressElement.dataset.conversationId = state.conversationId || '';
    }
}

function updateProgressConversation(progressId, conversationId) {
    if (!conversationId) {
        return;
    }
    registerProgressTask(progressId, conversationId);
}

function markProgressCancelling(progressId) {
    const state = progressTaskState.get(progressId);
    if (state) {
        state.cancelling = true;
    }
}

function finalizeProgressTask(progressId, finalLabel) {
    const stopBtn = document.getElementById(`${progressId}-stop-btn`);
    if (stopBtn) {
        stopBtn.disabled = true;
        if (finalLabel !== undefined && finalLabel !== '') {
            stopBtn.textContent = finalLabel;
        } else {
 stopBtn.textContent = typeof window.t === 'function' ? window.t('tasks.statusCompleted') : '';
        }
    }
    progressTaskState.delete(progressId);
}

async function requestCancel(conversationId) {
    const response = await apiFetch('/api/agent-loop/cancel', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({ conversationId }),
    });
    const result = await response.json().catch(() => ({}));
    if (!response.ok) {
 throw new Error(result.error || (typeof window.t === 'function' ? window.t('tasks.cancelFailed') : ''));
    }
    return result;
}

function addProgressMessage() {
    const messagesDiv = document.getElementById('chat-messages');
    const messageDiv = document.createElement('div');
    messageCounter++;
    const id = 'progress-' + Date.now() + '-' + messageCounter;
    messageDiv.id = id;
    messageDiv.className = 'message system progress-message';
    
    const contentWrapper = document.createElement('div');
    contentWrapper.className = 'message-content';
    
    const bubble = document.createElement('div');
    bubble.className = 'message-bubble progress-container';
 const progressTitleText = typeof window.t === 'function' ? window.t('chat.progressInProgress') : '...';
 const stopTaskText = typeof window.t === 'function' ? window.t('tasks.stopTask') : '';
 const collapseDetailText = typeof window.t === 'function' ? window.t('tasks.collapseDetail') : '';
    bubble.innerHTML = `
        <div class="progress-header">
            <span class="progress-title">🔍 ${progressTitleText}</span>
            <div class="progress-actions">
                <button class="progress-stop" id="${id}-stop-btn" onclick="cancelProgressTask('${id}')">${stopTaskText}</button>
                <button class="progress-toggle" onclick="toggleProgressDetails('${id}')">${collapseDetailText}</button>
            </div>
        </div>
        <div class="progress-timeline expanded" id="${id}-timeline"></div>
        <div class="progress-footer">
            <button type="button" class="progress-toggle progress-toggle-bottom" onclick="toggleProgressDetails('${id}')">${collapseDetailText}</button>
        </div>
    `;
    
    contentWrapper.appendChild(bubble);
    messageDiv.appendChild(contentWrapper);
    messageDiv.dataset.conversationId = currentConversationId || '';
    messagesDiv.appendChild(messageDiv);
    messagesDiv.scrollTop = messagesDiv.scrollHeight;
    
    return id;
}

// 
function toggleProgressDetails(progressId) {
    const timeline = document.getElementById(progressId + '-timeline');
    const toggleBtns = document.querySelectorAll(`#${progressId} .progress-toggle`);
    
    if (!timeline || !toggleBtns.length) return;
    
 const expandT = typeof window.t === 'function' ? window.t('chat.expandDetail') : '';
 const collapseT = typeof window.t === 'function' ? window.t('tasks.collapseDetail') : '';
    if (timeline.classList.contains('expanded')) {
        timeline.classList.remove('expanded');
        toggleBtns.forEach((btn) => { btn.textContent = expandT; });
    } else {
        timeline.classList.add('expanded');
        toggleBtns.forEach((btn) => { btn.textContent = collapseT; });
    }
}

// (;)
function hideProgressMessageForFinalReply(progressId) {
    if (!progressId) return;
    const el = document.getElementById(progressId);
    if (el) {
        el.style.display = 'none';
    }
}

// 
function collapseAllProgressDetails(assistantMessageId, progressId) {
 // MCP
    if (assistantMessageId) {
        const detailsId = 'process-details-' + assistantMessageId;
        const detailsContainer = document.getElementById(detailsId);
        if (detailsContainer) {
            const timeline = detailsContainer.querySelector('.progress-timeline');
            if (timeline) {
 // expanded()
                timeline.classList.remove('expanded');
                document.querySelectorAll(`#${assistantMessageId} .process-detail-btn`).forEach((btn) => {
 btn.innerHTML = '<span>' + (typeof window.t === 'function' ? window.t('chat.expandDetail') : '') + '</span>';
                });
            }
        }
    }
    
 // (convertProgressToDetails)
 // details-
    const allDetails = document.querySelectorAll('[id^="details-"]');
    allDetails.forEach(detail => {
        const timeline = detail.querySelector('.progress-timeline');
        const toggleBtns = detail.querySelectorAll('.progress-toggle');
        if (timeline) {
            timeline.classList.remove('expanded');
 const expandT = typeof window.t === 'function' ? window.t('chat.expandDetail') : '';
            toggleBtns.forEach((btn) => { btn.textContent = expandT; });
        }
    });
    
 // ()
    if (progressId) {
        const progressTimeline = document.getElementById(progressId + '-timeline');
        const progressToggleBtns = document.querySelectorAll(`#${progressId} .progress-toggle`);
        if (progressTimeline) {
            progressTimeline.classList.remove('expanded');
 const expandT = typeof window.t === 'function' ? window.t('chat.expandDetail') : '';
            progressToggleBtns.forEach((btn) => { btn.textContent = expandT; });
        }
    }
}

// ID(done)
function getAssistantId() {
 // ID
    const messages = document.querySelectorAll('.message.assistant');
    if (messages.length > 0) {
        return messages[messages.length - 1].id;
    }
    return null;
}

// ( mcp ,, MCP )
function integrateProgressToMCPSection(progressId, assistantMessageId, mcpExecutionIds) {
    const progressElement = document.getElementById(progressId);
    if (!progressElement) return;

    const mcpIds = Array.isArray(mcpExecutionIds) ? mcpExecutionIds : [];
    
 // 
    const timeline = document.getElementById(progressId + '-timeline');
    let timelineHTML = '';
    if (timeline) {
        timelineHTML = timeline.innerHTML;
    }
    
 // 
    const assistantElement = document.getElementById(assistantMessageId);
    if (!assistantElement) {
        removeMessage(progressId);
        return;
    }

    const contentWrapper = assistantElement.querySelector('.message-content');
    if (!contentWrapper) {
        removeMessage(progressId);
        return;
    }
    
 // MCP 
    let mcpSection = assistantElement.querySelector('.mcp-call-section');
    if (!mcpSection) {
        mcpSection = document.createElement('div');
        mcpSection.className = 'mcp-call-section';
        const mcpLabel = document.createElement('div');
        mcpLabel.className = 'mcp-call-label';
 mcpLabel.textContent = '📋 ' + (typeof window.t === 'function' ? window.t('chat.penetrationTestDetail') : '');
        mcpSection.appendChild(mcpLabel);
        const buttonsContainerInit = document.createElement('div');
        buttonsContainerInit.className = 'mcp-call-buttons';
        mcpSection.appendChild(buttonsContainerInit);
        contentWrapper.appendChild(mcpSection);
    }
    
 // 
    const hasContent = timelineHTML.trim().length > 0;
    
 // 
    const hasError = timeline && timeline.querySelector('.timeline-item-error');
    
 // 
    let buttonsContainer = mcpSection.querySelector('.mcp-call-buttons');
    if (!buttonsContainer) {
        buttonsContainer = document.createElement('div');
        buttonsContainer.className = 'mcp-call-buttons';
        mcpSection.appendChild(buttonsContainer);
    }

    const hasExecBtns = buttonsContainer.querySelector('.mcp-detail-btn:not(.process-detail-btn)');
    if (mcpIds.length > 0 && !hasExecBtns) {
        mcpIds.forEach((execId, index) => {
            const detailBtn = document.createElement('button');
            detailBtn.className = 'mcp-detail-btn';
 detailBtn.innerHTML = '<span>' + (typeof window.t === 'function' ? window.t('chat.callNumber', { n: index + 1 }) : ' #' + (index + 1)) + '</span>';
            detailBtn.onclick = () => showMCPDetail(execId);
            buttonsContainer.appendChild(detailBtn);
            if (typeof updateButtonWithToolName === 'function') {
                updateButtonWithToolName(detailBtn, execId, index + 1);
            }
        });
    }
    if (!buttonsContainer.querySelector('.process-detail-btn')) {
        const progressDetailBtn = document.createElement('button');
        progressDetailBtn.className = 'mcp-detail-btn process-detail-btn';
 progressDetailBtn.innerHTML = '<span>' + (typeof window.t === 'function' ? window.t('chat.expandDetail') : '') + '</span>';
        progressDetailBtn.onclick = () => toggleProcessDetails(null, assistantMessageId);
        buttonsContainer.appendChild(progressDetailBtn);
    }
    
 // ,MCP()
    const detailsId = 'process-details-' + assistantMessageId;
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
    
 // (,;)
    detailsContainer.innerHTML = `
        <div class="process-details-content">
 ${hasContent ? `<div class="progress-timeline" id="${detailsId}-timeline">${timelineHTML}</div>` : '<div class="progress-timeline-empty">' + (typeof window.t === 'function' ? window.t('chat.noProcessDetail') : '()') + '</div>'}
        </div>
    `;
    
 // (,)
    if (hasContent) {
        const timeline = document.getElementById(detailsId + '-timeline');
        if (timeline) {
 // ,;
            timeline.classList.remove('expanded');
        }
        
 const expandLabel = typeof window.t === 'function' ? window.t('chat.expandDetail') : '';
        document.querySelectorAll(`#${assistantMessageId} .process-detail-btn`).forEach((btn) => {
            btn.innerHTML = '<span>' + expandLabel + '</span>';
        });
    }
    
 // 
    removeMessage(progressId);
}

// 
function toggleProcessDetails(progressId, assistantMessageId) {
    const detailsId = 'process-details-' + assistantMessageId;
    const detailsContainer = document.getElementById(detailsId);
    if (!detailsContainer) return;

 // :
    const maybeLazy = detailsContainer.dataset && detailsContainer.dataset.lazyNotLoaded === '1' && detailsContainer.dataset.loaded !== '1';
    if (maybeLazy) {
        const messageEl = document.getElementById(assistantMessageId);
        const backendMessageId = messageEl && messageEl.dataset ? messageEl.dataset.backendMessageId : '';
        if (backendMessageId && typeof apiFetch === 'function' && typeof renderProcessDetails === 'function') {
            if (detailsContainer.dataset.loading === '1') {
 // ,
            } else {
                detailsContainer.dataset.loading = '1';
 // ,
                const timeline = detailsContainer.querySelector('.progress-timeline');
                if (timeline) {
 timeline.innerHTML = '<div class="progress-timeline-empty">' + ((typeof window.t === 'function') ? window.t('common.loading') : '...') + '</div>';
                }
                apiFetch(`/api/messages/${encodeURIComponent(String(backendMessageId))}/process-details`)
                    .then(async (res) => {
                        const j = await res.json().catch(() => ({}));
                        if (!res.ok) throw new Error((j && j.error) ? j.error : res.status);
                        const details = (j && Array.isArray(j.processDetails)) ? j.processDetails : [];
 // (renderProcessDetails lazy loaded)
                        renderProcessDetails(assistantMessageId, details);
                    })
                    .catch((e) => {
 console.error(':', e);
                        const tl = detailsContainer.querySelector('.progress-timeline');
                        if (tl) {
 tl.innerHTML = '<div class="progress-timeline-empty">' + ((typeof window.t === 'function') ? window.t('chat.noProcessDetail') : '()') + '</div>';
                        }
 // lazy ,
                        detailsContainer.dataset.lazyNotLoaded = '1';
                        detailsContainer.dataset.loaded = '0';
                    })
                    .finally(() => {
                        detailsContainer.dataset.loading = '0';
                    });
            }
        }
    }
    
    const content = detailsContainer.querySelector('.process-details-content');
    const timeline = detailsContainer.querySelector('.progress-timeline');
    const detailBtns = document.querySelectorAll(`#${assistantMessageId} .process-detail-btn`);
    
 const expandT = typeof window.t === 'function' ? window.t('chat.expandDetail') : '';
 const collapseT = typeof window.t === 'function' ? window.t('tasks.collapseDetail') : '';
    const setDetailBtnLabels = (label) => {
        detailBtns.forEach((btn) => { btn.innerHTML = '<span>' + label + '</span>'; });
    };
    if (content && timeline) {
        if (timeline.classList.contains('expanded')) {
            timeline.classList.remove('expanded');
            setDetailBtnLabels(expandT);
        } else {
            timeline.classList.add('expanded');
            setDetailBtnLabels(collapseT);
        }
    } else if (timeline) {
        if (timeline.classList.contains('expanded')) {
            timeline.classList.remove('expanded');
            setDetailBtnLabels(expandT);
        } else {
            timeline.classList.add('expanded');
            setDetailBtnLabels(collapseT);
        }
    }
    
 // ,
    if (timeline && timeline.classList.contains('expanded')) {
        setTimeout(() => {
 // scrollIntoView 
            detailsContainer.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
        }, 100);
    }
}

// 
async function cancelProgressTask(progressId) {
    const state = progressTaskState.get(progressId);
    const stopBtn = document.getElementById(`${progressId}-stop-btn`);

    if (!state || !state.conversationId) {
        if (stopBtn) {
            stopBtn.disabled = true;
            setTimeout(() => {
                stopBtn.disabled = false;
            }, 1500);
        }
 alert(typeof window.t === 'function' ? window.t('tasks.taskInfoNotSynced') : ',.');
        return;
    }

    if (state.cancelling) {
        return;
    }

    markProgressCancelling(progressId);
    if (stopBtn) {
        stopBtn.disabled = true;
 stopBtn.textContent = typeof window.t === 'function' ? window.t('tasks.cancelling') : '...';
    }

    try {
        await requestCancel(state.conversationId);
        loadActiveTasks();
    } catch (error) {
 console.error(':', error);
 alert((typeof window.t === 'function' ? window.t('tasks.cancelTaskFailed') : '') + ': ' + error.message);
        if (stopBtn) {
            stopBtn.disabled = false;
 stopBtn.textContent = typeof window.t === 'function' ? window.t('tasks.stopTask') : '';
        }
        const currentState = progressTaskState.get(progressId);
        if (currentState) {
            currentState.cancelling = false;
        }
    }
}

// 
function convertProgressToDetails(progressId, assistantMessageId) {
    const progressElement = document.getElementById(progressId);
    if (!progressElement) return;
    
 // 
    const timeline = document.getElementById(progressId + '-timeline');
 // ,()
    let timelineHTML = '';
    if (timeline) {
        timelineHTML = timeline.innerHTML;
    }
    
 // 
    const assistantElement = document.getElementById(assistantMessageId);
    if (!assistantElement) {
        removeMessage(progressId);
        return;
    }
    
 // 
    const detailsId = 'details-' + Date.now() + '-' + messageCounter++;
    const detailsDiv = document.createElement('div');
    detailsDiv.id = detailsId;
    detailsDiv.className = 'message system progress-details';
    
    const contentWrapper = document.createElement('div');
    contentWrapper.className = 'message-content';
    
    const bubble = document.createElement('div');
    bubble.className = 'message-bubble progress-container completed';
    
 // HTML
    const hasContent = timelineHTML.trim().length > 0;
    
 // 
    const hasError = timeline && timeline.querySelector('.timeline-item-error');
    
 // ,;
    const shouldExpand = !hasError;
    const expandedClass = shouldExpand ? 'expanded' : '';
 const collapseDetailText = typeof window.t === 'function' ? window.t('tasks.collapseDetail') : '';
 const expandDetailText = typeof window.t === 'function' ? window.t('chat.expandDetail') : '';
    const toggleText = shouldExpand ? collapseDetailText : expandDetailText;
 const penetrationDetailText = typeof window.t === 'function' ? window.t('chat.penetrationTestDetail') : '';
 const noProcessDetailText = typeof window.t === 'function' ? window.t('chat.noProcessDetail') : '()';
    bubble.innerHTML = `
        <div class="progress-header">
            <span class="progress-title">📋 ${penetrationDetailText}</span>
            ${hasContent ? `<button class="progress-toggle" onclick="toggleProgressDetails('${detailsId}')">${toggleText}</button>` : ''}
        </div>
        ${hasContent ? `<div class="progress-timeline ${expandedClass}" id="${detailsId}-timeline">${timelineHTML}</div><div class="progress-footer"><button type="button" class="progress-toggle progress-toggle-bottom" onclick="toggleProgressDetails('${detailsId}')">${toggleText}</button></div>` : '<div class="progress-timeline-empty">' + noProcessDetailText + '</div>'}
    `;
    
    contentWrapper.appendChild(bubble);
    detailsDiv.appendChild(contentWrapper);
    
 // 
    const messagesDiv = document.getElementById('chat-messages');
    const insertWasPinned = isChatMessagesPinnedToBottom();
 // assistantElement div,
    if (assistantElement.nextSibling) {
        messagesDiv.insertBefore(detailsDiv, assistantElement.nextSibling);
    } else {
 // ,
        messagesDiv.appendChild(detailsDiv);
    }
    
 // 
    removeMessage(progressId);
    
    scrollChatMessagesToBottomIfPinned(insertWasPinned);
}

// 
function handleStreamEvent(event, progressElement, progressId, 
                          getAssistantId, setAssistantId, getMcpIds, setMcpIds) {
    const streamScrollWasPinned = isChatMessagesPinnedToBottom();
    const timeline = document.getElementById(progressId + '-timeline');
    if (!timeline) return;

 // (error/cancelled),
    const upsertTerminalAssistantMessage = (message, preferredMessageId = null) => {
        const preferredIds = [];
        if (preferredMessageId) preferredIds.push(preferredMessageId);
        const existingAssistantId = typeof getAssistantId === 'function' ? getAssistantId() : null;
        if (existingAssistantId && !preferredIds.includes(existingAssistantId)) {
            preferredIds.push(existingAssistantId);
        }

        for (const id of preferredIds) {
            const element = document.getElementById(id);
            if (element) {
                updateAssistantBubbleContent(id, message, true);
                setAssistantId(id);
                return { assistantId: id, assistantElement: element };
            }
        }

        const assistantId = addMessage('assistant', message, null, progressId);
        setAssistantId(assistantId);
        return { assistantId: assistantId, assistantElement: document.getElementById(assistantId) };
    };
    
    switch (event.type) {
        case 'heartbeat':
 // SSE , UI
            break;
        case 'conversation':
            if (event.data && event.data.conversationId) {
 // ,ID
                const taskState = progressTaskState.get(progressId);
                const originalConversationId = taskState?.conversationId;
                
 // 
                updateProgressConversation(progressId, event.data.conversationId);
                
 // (currentConversationId null),
 // conversation , currentConversationId
                if (currentConversationId === null && originalConversationId !== null) {
 // , conversation 
 // ,
                    break;
                }
                
 // ID
                currentConversationId = event.data.conversationId;
                updateActiveConversation();
                addAttackChainButton(currentConversationId);
                loadActiveTasks();
 // ,,updated_at
 // 
 // loadConversationsWithGroups,
                setTimeout(() => {
                    if (typeof loadConversationsWithGroups === 'function') {
                        loadConversationsWithGroups();
                    } else if (typeof loadConversations === 'function') {
                        loadConversations();
                    }
                }, 200);
            }
            break;
        case 'iteration': {
            const d = event.data || {};
            const n = d.iteration != null ? d.iteration : 1;
            let iterTitle;
            if (d.einoScope === 'main') {
                iterTitle = typeof window.t === 'function'
                    ? window.t('chat.einoOrchestratorRound', { n: n })
 : (' · ' + n + ' ');
            } else if (d.einoScope === 'sub') {
                const ag = d.einoAgent != null ? String(d.einoAgent).trim() : '';
                iterTitle = typeof window.t === 'function'
                    ? window.t('chat.einoSubAgentStep', { n: n, agent: ag })
 : (' · ' + ag + ' · ' + n + ' ');
            } else {
                iterTitle = typeof window.t === 'function'
                    ? window.t('chat.iterationRound', { n: n })
 : (' ' + n + ' ');
            }
            addTimelineItem(timeline, 'iteration', {
                title: iterTitle,
                message: event.message,
                data: event.data,
                iterationN: n
            });
            break;
        }
            
        case 'thinking_stream_start': {
            const d = event.data || {};
            const streamId = d.streamId || null;
            if (!streamId) break;

            let state = thinkingStreamStateByProgressId.get(progressId);
            if (!state) {
                state = new Map();
                thinkingStreamStateByProgressId.set(progressId, state);
            }
 // , buffer
 const thinkBase = typeof window.t === 'function' ? window.t('chat.aiThinking') : 'AI';
            const title = timelineAgentBracketPrefix(d) + '🤔 ' + thinkBase;
            const itemId = addTimelineItem(timeline, 'thinking', {
                title: title,
                message: ' ',
                data: d
            });
            state.set(streamId, { itemId, buffer: '' });
            break;
        }

        case 'thinking_stream_delta': {
            const d = event.data || {};
            const streamId = d.streamId || null;
            if (!streamId) break;

            const state = thinkingStreamStateByProgressId.get(progressId);
            if (!state || !state.has(streamId)) break;
            const s = state.get(streamId);

            const delta = event.message || '';
            s.buffer += delta;

            const item = document.getElementById(s.itemId);
            if (item) {
                const contentEl = item.querySelector('.timeline-item-content');
                if (contentEl) {
                    if (typeof formatMarkdown === 'function') {
                        contentEl.innerHTML = formatMarkdown(s.buffer);
                    } else {
                        contentEl.textContent = s.buffer;
                    }
                }
            }
            break;
        }

        case 'thinking':
 // thinking thinking_stream_* ( streamId), timeline item
            if (event.data && event.data.streamId) {
                const streamId = event.data.streamId;
                const state = thinkingStreamStateByProgressId.get(progressId);
                if (state && state.has(streamId)) {
                    const s = state.get(streamId);
                    s.buffer = event.message || '';
                    const item = document.getElementById(s.itemId);
                    if (item) {
                        const contentEl = item.querySelector('.timeline-item-content');
                        if (contentEl) {
 // contentEl.innerHTML Markdown 
                            if (typeof formatMarkdown === 'function') {
                                contentEl.innerHTML = formatMarkdown(s.buffer);
                            } else {
                                contentEl.textContent = s.buffer;
                            }
                        }
                    }
                    break;
                }
            }

            addTimelineItem(timeline, 'thinking', {
 title: timelineAgentBracketPrefix(event.data) + '🤔 ' + (typeof window.t === 'function' ? window.t('chat.aiThinking') : 'AI'),
                message: event.message,
                data: event.data
            });
            break;
            
        case 'tool_calls_detected':
            addTimelineItem(timeline, 'tool_calls_detected', {
 title: timelineAgentBracketPrefix(event.data) + '🔧 ' + (typeof window.t === 'function' ? window.t('chat.toolCallsDetected', { count: event.data?.count || 0 }) : ' ' + (event.data?.count || 0) + ' '),
                message: event.message,
                data: event.data
            });
            break;

        case 'warning':
            addTimelineItem(timeline, 'warning', {
                title: '⚠️',
                message: event.message,
                data: event.data
            });
            break;
            
        case 'tool_call':
            const toolInfo = event.data || {};
 const toolName = toolInfo.toolName || (typeof window.t === 'function' ? window.t('chat.unknownTool') : '');
            const index = toolInfo.index || 0;
            const total = toolInfo.total || 0;
            const toolCallId = toolInfo.toolCallId || null;
 const toolCallTitle = typeof window.t === 'function' ? window.t('chat.callTool', { name: escapeHtml(toolName), index: index, total: total }) : ': ' + escapeHtml(toolName) + ' (' + index + '/' + total + ')';
            const toolCallItemId = addTimelineItem(timeline, 'tool_call', {
                title: timelineAgentBracketPrefix(toolInfo) + '🔧 ' + toolCallTitle,
                message: event.message,
                data: toolInfo,
                expanded: false
            });
            
 // toolCallId,
            if (toolCallId && toolCallItemId) {
                toolCallStatusMap.set(toolCallId, {
                    itemId: toolCallItemId,
                    timeline: timeline
                });
                
 // 
                updateToolCallStatus(toolCallId, 'running');
            }
            break;

        case 'tool_result_delta': {
            const deltaInfo = event.data || {};
            const toolCallId = deltaInfo.toolCallId || null;
            if (!toolCallId) break;

            const key = toolResultStreamKey(progressId, toolCallId);
            let state = toolResultStreamStateByKey.get(key);
 const toolNameDelta = deltaInfo.toolName || (typeof window.t === 'function' ? window.t('chat.unknownTool') : '');
            const deltaText = event.message || '';
            if (!deltaText) break;

            if (!state) {
 // : tool_result , pre 
 const runningLabel = typeof window.t === 'function' ? window.t('timeline.running') : '...';
                const title = timelineAgentBracketPrefix(deltaInfo) + '⏳ ' + (typeof window.t === 'function'
                    ? window.t('timeline.running')
                    : runningLabel) + ' ' + (typeof window.t === 'function' ? window.t('chat.callTool', { name: escapeHtmlLocal(toolNameDelta), index: deltaInfo.index || 0, total: deltaInfo.total || 0 }) : toolNameDelta);

                const itemId = addTimelineItem(timeline, 'tool_result', {
                    title: title,
                    message: '',
                    data: {
                        toolName: toolNameDelta,
                        success: true,
                        isError: false,
                        result: deltaText,
                        toolCallId: toolCallId,
                        index: deltaInfo.index,
                        total: deltaInfo.total,
                        iteration: deltaInfo.iteration,
                        einoAgent: deltaInfo.einoAgent,
                        source: deltaInfo.source
                    },
                    expanded: false
                });

                state = { itemId, buffer: '' };
                toolResultStreamStateByKey.set(key, state);
            }

            state.buffer += deltaText;
            const item = document.getElementById(state.itemId);
            if (item) {
                const pre = item.querySelector('pre.tool-result');
                if (pre) {
                    pre.textContent = state.buffer;
                }
            }
            break;
        }
            
        case 'tool_result':
            const resultInfo = event.data || {};
 const resultToolName = resultInfo.toolName || (typeof window.t === 'function' ? window.t('chat.unknownTool') : '');
            const success = resultInfo.success !== false;
            const statusIcon = success ? '✅' : '❌';
            const resultToolCallId = resultInfo.toolCallId || null;
 const resultExecText = success ? (typeof window.t === 'function' ? window.t('chat.toolExecComplete', { name: escapeHtml(resultToolName) }) : ' ' + escapeHtml(resultToolName) + ' ') : (typeof window.t === 'function' ? window.t('chat.toolExecFailed', { name: escapeHtml(resultToolName) }) : ' ' + escapeHtml(resultToolName) + ' ');

 // tool ,,
            if (resultToolCallId) {
                const key = toolResultStreamKey(progressId, resultToolCallId);
                const state = toolResultStreamStateByKey.get(key);
                if (state && state.itemId) {
                    const item = document.getElementById(state.itemId);
                    if (item) {
                        const pre = item.querySelector('pre.tool-result');
                        const resultVal = resultInfo.result || resultInfo.error || '';
                        if (pre) pre.textContent = typeof resultVal === 'string' ? resultVal : JSON.stringify(resultVal);

                        const section = item.querySelector('.tool-result-section');
                        if (section) {
                            section.className = 'tool-result-section ' + (success ? 'success' : 'error');
                        }

                        const titleEl = item.querySelector('.timeline-item-title');
                        if (titleEl) {
                            if (resultInfo.einoAgent != null && String(resultInfo.einoAgent).trim() !== '') {
                                item.dataset.einoAgent = String(resultInfo.einoAgent).trim();
                            }
                            titleEl.textContent = timelineAgentBracketPrefix(resultInfo) + statusIcon + ' ' + resultExecText;
                        }
                    }
                    toolResultStreamStateByKey.delete(key);

 // tool_call 
                    if (resultToolCallId && toolCallStatusMap.has(resultToolCallId)) {
                        updateToolCallStatus(resultToolCallId, success ? 'completed' : 'failed');
                        toolCallStatusMap.delete(resultToolCallId);
                    }
                    break;
                }
            }

            if (resultToolCallId && toolCallStatusMap.has(resultToolCallId)) {
                updateToolCallStatus(resultToolCallId, success ? 'completed' : 'failed');
                toolCallStatusMap.delete(resultToolCallId);
            }
            addTimelineItem(timeline, 'tool_result', {
                title: timelineAgentBracketPrefix(resultInfo) + statusIcon + ' ' + resultExecText,
                message: event.message,
                data: resultInfo,
                expanded: false
            });
            break;

        case 'eino_agent_reply_stream_start': {
            const d = event.data || {};
            const streamId = d.streamId || null;
            if (!streamId) break;
            let stateMap = einoAgentReplyStreamStateByProgressId.get(progressId);
            if (!stateMap) {
                stateMap = new Map();
                einoAgentReplyStreamStateByProgressId.set(progressId, stateMap);
            }
 const streamingLabel = typeof window.t === 'function' ? window.t('timeline.running') : '...';
 const replyTitleBase = typeof window.t === 'function' ? window.t('chat.einoAgentReplyTitle') : '';
            const itemId = addTimelineItem(timeline, 'eino_agent_reply', {
                title: timelineAgentBracketPrefix(d) + '💬 ' + replyTitleBase + ' · ' + streamingLabel,
                message: ' ',
                data: d,
                expanded: false
            });
            stateMap.set(streamId, { itemId, buffer: '' });
            break;
        }

        case 'eino_agent_reply_stream_delta': {
            const d = event.data || {};
            const streamId = d.streamId || null;
            if (!streamId) break;
            const delta = event.message || '';
            if (!delta) break;
            const stateMap = einoAgentReplyStreamStateByProgressId.get(progressId);
            if (!stateMap || !stateMap.has(streamId)) break;
            const s = stateMap.get(streamId);
            s.buffer += delta;
            const item = document.getElementById(s.itemId);
            if (item) {
                let contentEl = item.querySelector('.timeline-item-content');
                if (!contentEl) {
                    const header = item.querySelector('.timeline-item-header');
                    if (header) {
                        contentEl = document.createElement('div');
                        contentEl.className = 'timeline-item-content';
                        item.appendChild(contentEl);
                    }
                }
                if (contentEl) {
                    if (typeof formatMarkdown === 'function') {
                        contentEl.innerHTML = formatMarkdown(s.buffer);
                    } else {
                        contentEl.textContent = s.buffer;
                    }
                }
            }
            break;
        }

        case 'eino_agent_reply_stream_end': {
            const d = event.data || {};
            const streamId = d.streamId || null;
            const stateMap = einoAgentReplyStreamStateByProgressId.get(progressId);
            if (streamId && stateMap && stateMap.has(streamId)) {
                const s = stateMap.get(streamId);
                const full = (event.message != null && event.message !== '') ? String(event.message) : s.buffer;
                s.buffer = full;
                const item = document.getElementById(s.itemId);
                if (item) {
                    const titleEl = item.querySelector('.timeline-item-title');
                    if (titleEl) {
 const replyTitleBase = typeof window.t === 'function' ? window.t('chat.einoAgentReplyTitle') : '';
                        titleEl.textContent = timelineAgentBracketPrefix(d) + '💬 ' + replyTitleBase;
                    }
                    let contentEl = item.querySelector('.timeline-item-content');
                    if (!contentEl) {
                        contentEl = document.createElement('div');
                        contentEl.className = 'timeline-item-content';
                        item.appendChild(contentEl);
                    }
                    if (typeof formatMarkdown === 'function') {
                        contentEl.innerHTML = formatMarkdown(full);
                    } else {
                        contentEl.textContent = full;
                    }
                    if (d.einoAgent != null && String(d.einoAgent).trim() !== '') {
                        item.dataset.einoAgent = String(d.einoAgent).trim();
                    }
                }
                stateMap.delete(streamId);
            }
            break;
        }

        case 'eino_agent_reply': {
            const replyData = event.data || {};
 const replyTitleBase = typeof window.t === 'function' ? window.t('chat.einoAgentReplyTitle') : '';
            addTimelineItem(timeline, 'eino_agent_reply', {
                title: timelineAgentBracketPrefix(replyData) + '💬 ' + replyTitleBase,
                message: event.message || '',
                data: replyData,
                expanded: false
            });
            break;
        }
            
        case 'progress':
            const progressTitle = document.querySelector(`#${progressId} .progress-title`);
            if (progressTitle) {
 // , translateProgressMessage 
                const progressEl = document.getElementById(progressId);
                if (progressEl) {
                    progressEl.dataset.progressRawMessage = event.message || '';
                }
                const progressMsg = translateProgressMessage(event.message);
                progressTitle.textContent = '🔍 ' + progressMsg;
            }
            break;
        
        case 'cancelled':
 const taskCancelledText = typeof window.t === 'function' ? window.t('chat.taskCancelled') : '';
            addTimelineItem(timeline, 'cancelled', {
                title: '⛔ ' + taskCancelledText,
                message: event.message,
                data: event.data
            });
            const cancelTitle = document.querySelector(`#${progressId} .progress-title`);
            if (cancelTitle) {
                cancelTitle.textContent = '⛔ ' + taskCancelledText;
            }
            const cancelProgressContainer = document.querySelector(`#${progressId} .progress-container`);
            if (cancelProgressContainer) {
                cancelProgressContainer.classList.add('completed');
            }
            if (progressTaskState.has(progressId)) {
 finalizeProgressTask(progressId, typeof window.t === 'function' ? window.t('tasks.statusCancelled') : '');
            }
            
 // (),
            {
                const preferredMessageId = event.data && event.data.messageId ? event.data.messageId : null;
                const { assistantId, assistantElement } = upsertTerminalAssistantMessage(event.message, preferredMessageId);
                if (assistantElement) {
                    const detailsId = 'process-details-' + assistantId;
                    if (!document.getElementById(detailsId)) {
                        integrateProgressToMCPSection(progressId, assistantId, typeof getMcpIds === 'function' ? (getMcpIds() || []) : []);
                    }
                    setTimeout(() => {
                        collapseAllProgressDetails(assistantId, progressId);
                    }, 100);
                }
            }
            
 // 
            loadActiveTasks();
            break;
            
        case 'response_start': {
            const responseTaskState = progressTaskState.get(progressId);
            const responseOriginalConversationId = responseTaskState?.conversationId;

            const responseData = event.data || {};
            const mcpIds = responseData.mcpExecutionIds || [];
            setMcpIds(mcpIds);

            if (responseData.conversationId) {
 // (currentConversationId null),,
                if (currentConversationId === null && responseOriginalConversationId !== null) {
                    updateProgressConversation(progressId, responseData.conversationId);
                    break;
                }
                currentConversationId = responseData.conversationId;
                updateActiveConversation();
                addAttackChainButton(currentConversationId);
                updateProgressConversation(progressId, responseData.conversationId);
                loadActiveTasks();
            }

 // ,,
 // 
            const agentPrefix = timelineAgentBracketPrefix(responseData);
 const title = agentPrefix + '📝 ' + (typeof window.t === 'function' ? window.t('chat.planning') : '');
            const itemId = addTimelineItem(timeline, 'thinking', {
                title: title,
                message: ' ',
                data: responseData
            });
            responseStreamStateByProgressId.set(progressId, { itemId: itemId, buffer: '' });
            break;
        }

        case 'response_delta': {
            const responseData = event.data || {};
            const responseTaskState = progressTaskState.get(progressId);
            const responseOriginalConversationId = responseTaskState?.conversationId;

            if (responseData.conversationId) {
                if (currentConversationId === null && responseOriginalConversationId !== null) {
                    updateProgressConversation(progressId, responseData.conversationId);
                    break;
                }
            }

 // ,
 // 
            let state = responseStreamStateByProgressId.get(progressId);
            if (!state) {
                state = { itemId: null, buffer: '' };
                responseStreamStateByProgressId.set(progressId, state);
            }

            const deltaContent = event.message || '';
            state.buffer += deltaContent;

 // 
            if (state.itemId) {
                const item = document.getElementById(state.itemId);
                if (item) {
                    const contentEl = item.querySelector('.timeline-item-content');
                    if (contentEl) {
                        if (typeof formatMarkdown === 'function') {
                            contentEl.innerHTML = formatMarkdown(state.buffer);
                        } else {
                            contentEl.textContent = state.buffer;
                        }
                    }
                }
            }
            break;
        }

        case 'response':
 // ,ID
            const responseTaskState = progressTaskState.get(progressId);
            const responseOriginalConversationId = responseTaskState?.conversationId;

 // mcp ids
            const responseData = event.data || {};
            const mcpIds = responseData.mcpExecutionIds || [];
            setMcpIds(mcpIds);

 // ID
            if (responseData.conversationId) {
                if (currentConversationId === null && responseOriginalConversationId !== null) {
                    updateProgressConversation(progressId, responseData.conversationId);
                    break;
                }

                currentConversationId = responseData.conversationId;
                updateActiveConversation();
                addAttackChainButton(currentConversationId);
                updateProgressConversation(progressId, responseData.conversationId);
                loadActiveTasks();
            }

 // response_start/response_delta ,
            const streamState = responseStreamStateByProgressId.get(progressId);
            const existingAssistantId = streamState?.assistantId || getAssistantId();
            let assistantIdFinal = existingAssistantId;

            if (!assistantIdFinal) {
                assistantIdFinal = addMessage('assistant', event.message, mcpIds, progressId);
                setAssistantId(assistantIdFinal);
            } else {
                setAssistantId(assistantIdFinal);
                updateAssistantBubbleContent(assistantIdFinal, event.message, true);
            }

 // (,)
            hideProgressMessageForFinalReply(progressId);

 // ( response ,)
            integrateProgressToMCPSection(progressId, assistantIdFinal, mcpIds);
            responseStreamStateByProgressId.delete(progressId);

            setTimeout(() => {
                collapseAllProgressDetails(assistantIdFinal, progressId);
            }, 3000);

            setTimeout(() => {
                loadConversations();
            }, 200);
            break;
            
        case 'error':
 // 
            addTimelineItem(timeline, 'error', {
 title: '❌ ' + (typeof window.t === 'function' ? window.t('chat.error') : ''),
                message: event.message,
                data: event.data
            });
            
 // 
            const errorTitle = document.querySelector(`#${progressId} .progress-title`);
            if (errorTitle) {
 errorTitle.textContent = '❌ ' + (typeof window.t === 'function' ? window.t('chat.executionFailed') : '');
            }
            
 // (completed)
            const progressContainer = document.querySelector(`#${progressId} .progress-container`);
            if (progressContainer) {
                progressContainer.classList.add('completed');
            }
            
 // ()
            if (progressTaskState.has(progressId)) {
 finalizeProgressTask(progressId, typeof window.t === 'function' ? window.t('tasks.statusFailed') : '');
            }
            
 // (),
            {
                const preferredMessageId = event.data && event.data.messageId ? event.data.messageId : null;
                const { assistantId, assistantElement } = upsertTerminalAssistantMessage(event.message, preferredMessageId);
                if (assistantElement) {
                    const detailsId = 'process-details-' + assistantId;
                    if (!document.getElementById(detailsId)) {
                        integrateProgressToMCPSection(progressId, assistantId, typeof getMcpIds === 'function' ? (getMcpIds() || []) : []);
                    }
                    setTimeout(() => {
                        collapseAllProgressDetails(assistantId, progressId);
                    }, 100);
                }
            }
            
 // ()
            loadActiveTasks();
            break;
            
        case 'done':
 // 
            responseStreamStateByProgressId.delete(progressId);
            thinkingStreamStateByProgressId.delete(progressId);
            einoAgentReplyStreamStateByProgressId.delete(progressId);
 // 
            const prefix = String(progressId) + '::';
            for (const key of Array.from(toolResultStreamStateByKey.keys())) {
                if (String(key).startsWith(prefix)) {
                    toolResultStreamStateByKey.delete(key);
                }
            }
 // ,()
            const doneTitle = document.querySelector(`#${progressId} .progress-title`);
            if (doneTitle) {
 doneTitle.textContent = '✅ ' + (typeof window.t === 'function' ? window.t('chat.penetrationTestComplete') : '');
            }
 // ID
            if (event.data && event.data.conversationId) {
                currentConversationId = event.data.conversationId;
                updateActiveConversation();
                addAttackChainButton(currentConversationId);
                updateProgressConversation(progressId, event.data.conversationId);
            }
            if (progressTaskState.has(progressId)) {
 finalizeProgressTask(progressId, typeof window.t === 'function' ? window.t('tasks.statusCompleted') : '');
            }
            
 // 
            const hasError = timeline && timeline.querySelector('.timeline-item-error');
            
 // ()
            loadActiveTasks();
            
 // ()
            setTimeout(() => {
                loadActiveTasks();
            }, 200);
            
 // (response)
            setTimeout(() => {
                const assistantIdFromDone = getAssistantId();
                if (assistantIdFromDone) {
                    collapseAllProgressDetails(assistantIdFromDone, progressId);
                } else {
 // ID,
                    collapseAllProgressDetails(null, progressId);
                }
                
 // ,()
                if (hasError) {
 // (DOM)
                    setTimeout(() => {
                        collapseAllProgressDetails(assistantIdFromDone || null, progressId);
                    }, 200);
                }
            }, 500);
            break;
    }
    
 // ()
    scrollChatMessagesToBottomIfPinned(streamScrollWasPinned);
}

// 
function updateToolCallStatus(toolCallId, status) {
    const mapping = toolCallStatusMap.get(toolCallId);
    if (!mapping) return;
    
    const item = document.getElementById(mapping.itemId);
    if (!item) return;
    
    const titleElement = item.querySelector('.timeline-item-title');
    if (!titleElement) return;
    
 // 
    item.classList.remove('tool-call-running', 'tool-call-completed', 'tool-call-failed');
    
 const runningLabel = typeof window.t === 'function' ? window.t('timeline.running') : '...';
 const completedLabel = typeof window.t === 'function' ? window.t('timeline.completed') : '';
 const failedLabel = typeof window.t === 'function' ? window.t('timeline.execFailed') : '';
    let statusText = '';
    if (status === 'running') {
        item.classList.add('tool-call-running');
        statusText = ' <span class="tool-status-badge tool-status-running">' + escapeHtml(runningLabel) + '</span>';
    } else if (status === 'completed') {
        item.classList.add('tool-call-completed');
        statusText = ' <span class="tool-status-badge tool-status-completed">✅ ' + escapeHtml(completedLabel) + '</span>';
    } else if (status === 'failed') {
        item.classList.add('tool-call-failed');
        statusText = ' <span class="tool-status-badge tool-status-failed">❌ ' + escapeHtml(failedLabel) + '</span>';
    }
    
 // (,)
    const originalText = titleElement.innerHTML;
 // 
    const cleanText = originalText.replace(/\s*<span class="tool-status-badge[^>]*>.*?<\/span>/g, '');
    titleElement.innerHTML = cleanText + statusText;
}

// 
function addTimelineItem(timeline, type, options) {
    const item = document.createElement('div');
 // ID
    const itemId = 'timeline-item-' + Date.now() + '-' + Math.random().toString(36).substr(2, 9);
    item.id = itemId;
    item.className = `timeline-item timeline-item-${type}`;
 // , languagechange 
    item.dataset.timelineType = type;
    if (type === 'iteration') {
        const n = options.iterationN != null ? options.iterationN : (options.data && options.data.iteration != null ? options.data.iteration : 1);
        item.dataset.iterationN = String(n);
        if (options.data && options.data.einoScope) {
            item.dataset.einoScope = String(options.data.einoScope);
        }
    }
    if (type === 'progress' && options.message) {
        item.dataset.progressMessage = options.message;
    }
    if (type === 'tool_calls_detected' && options.data && options.data.count != null) {
        item.dataset.toolCallsCount = String(options.data.count);
    }
    if (type === 'tool_call' && options.data) {
        const d = options.data;
        item.dataset.toolName = (d.toolName != null && d.toolName !== '') ? String(d.toolName) : '';
        item.dataset.toolIndex = (d.index != null) ? String(d.index) : '0';
        item.dataset.toolTotal = (d.total != null) ? String(d.total) : '0';
    }
    if (type === 'tool_result' && options.data) {
        const d = options.data;
        item.dataset.toolName = (d.toolName != null && d.toolName !== '') ? String(d.toolName) : '';
        item.dataset.toolSuccess = d.success !== false ? '1' : '0';
    }
    if (options.data && options.data.einoAgent != null && String(options.data.einoAgent).trim() !== '') {
        item.dataset.einoAgent = String(options.data.einoAgent).trim();
    }

 // createdAt,()
    let eventTime;
    if (options.createdAt) {
 // Date
        if (typeof options.createdAt === 'string') {
            eventTime = new Date(options.createdAt);
        } else if (options.createdAt instanceof Date) {
            eventTime = options.createdAt;
        } else {
            eventTime = new Date(options.createdAt);
        }
 // ,
        if (isNaN(eventTime.getTime())) {
            eventTime = new Date();
        }
    } else {
        eventTime = new Date();
    }
 // ISO,
    try {
        item.dataset.createdAtIso = eventTime.toISOString();
    } catch (e) { /* ignore */ }

    const timeLocale = getCurrentTimeLocale();
    const timeOpts = getTimeFormatOptions();
    const time = eventTime.toLocaleTimeString(timeLocale, timeOpts);
    
    let content = `
        <div class="timeline-item-header">
            <span class="timeline-item-time">${time}</span>
            <span class="timeline-item-title">${escapeHtml(options.title || '')}</span>
        </div>
    `;
    
 // 
    if (type === 'thinking' && options.message) {
        content += `<div class="timeline-item-content">${formatMarkdown(options.message)}</div>`;
    } else if (type === 'tool_call' && options.data) {
        const data = options.data;
        let args = data.argumentsObj;
        if (args == null && data.arguments != null && String(data.arguments).trim() !== '') {
            try {
                args = JSON.parse(String(data.arguments));
            } catch (e) {
                args = { _raw: String(data.arguments) };
            }
        }
        if (args == null || typeof args !== 'object') {
            args = {};
        }
 const paramsLabel = typeof window.t === 'function' ? window.t('timeline.params') : ':';
        content += `
            <div class="timeline-item-content">
                <div class="tool-details">
                    <div class="tool-arg-section">
                        <strong data-i18n="timeline.params">${escapeHtml(paramsLabel)}</strong>
                        <pre class="tool-args">${escapeHtml(JSON.stringify(args, null, 2))}</pre>
                    </div>
                </div>
            </div>
        `;
    } else if (type === 'eino_agent_reply' && options.message) {
        content += `<div class="timeline-item-content">${formatMarkdown(options.message)}</div>`;
    } else if (type === 'tool_result' && options.data) {
        const data = options.data;
        const isError = data.isError || !data.success;
 const noResultText = typeof window.t === 'function' ? window.t('timeline.noResult') : '';
        const result = data.result || data.error || noResultText;
        const resultStr = typeof result === 'string' ? result : JSON.stringify(result);
 const execResultLabel = typeof window.t === 'function' ? window.t('timeline.executionResult') : ':';
 const execIdLabel = typeof window.t === 'function' ? window.t('timeline.executionId') : 'ID:';
        content += `
            <div class="timeline-item-content">
                <div class="tool-result-section ${isError ? 'error' : 'success'}">
                    <strong data-i18n="timeline.executionResult">${escapeHtml(execResultLabel)}</strong>
                    <pre class="tool-result">${escapeHtml(resultStr)}</pre>
                    ${data.executionId ? `<div class="tool-execution-id"><span data-i18n="timeline.executionId">${escapeHtml(execIdLabel)}</span> <code>${escapeHtml(data.executionId)}</code></div>` : ''}
                </div>
            </div>
        `;
    } else if (type === 'cancelled') {
 const taskCancelledLabel = typeof window.t === 'function' ? window.t('chat.taskCancelled') : '';
        content += `
            <div class="timeline-item-content">
                ${escapeHtml(options.message || taskCancelledLabel)}
            </div>
        `;
    }
    
    item.innerHTML = content;
    if (options.data) {
        applyEinoTimelineRole(item, options.data);
    }
    timeline.appendChild(item);
    
 // 
    const expanded = timeline.classList.contains('expanded');
    if (!expanded && (type === 'tool_call' || type === 'tool_result')) {
 // ,
    }
    
 // item ID
    return itemId;
}

// 
async function loadActiveTasks(showErrors = false) {
    const bar = document.getElementById('active-tasks-bar');
    try {
        const response = await apiFetch('/api/agent-loop/tasks');
        const result = await response.json().catch(() => ({}));

        if (!response.ok) {
 throw new Error(result.error || (typeof window.t === 'function' ? window.t('tasks.loadActiveTasksFailed') : ''));
        }

        renderActiveTasks(result.tasks || []);
    } catch (error) {
 console.error(':', error);
        if (showErrors && bar) {
            bar.style.display = 'block';
 const cannotGetStatus = typeof window.t === 'function' ? window.t('tasks.cannotGetTaskStatus') : ':';
            bar.innerHTML = `<div class="active-task-error">${escapeHtml(cannotGetStatus)}${escapeHtml(error.message)}</div>`;
        }
    }
}

function renderActiveTasks(tasks) {
    const bar = document.getElementById('active-tasks-bar');
    if (!bar) return;

    const normalizedTasks = Array.isArray(tasks) ? tasks : [];
    conversationExecutionTracker.update(normalizedTasks);
    if (typeof updateAttackChainAvailability === 'function') {
        updateAttackChainAvailability();
    }

    if (normalizedTasks.length === 0) {
        bar.style.display = 'none';
        bar.innerHTML = '';
        return;
    }

    bar.style.display = 'flex';
    bar.innerHTML = '';

    normalizedTasks.forEach(task => {
        const item = document.createElement('div');
        item.className = 'active-task-item';

        const startedTime = task.startedAt ? new Date(task.startedAt) : null;
        const taskTimeLocale = getCurrentTimeLocale();
        const timeOpts = getTimeFormatOptions();
        const timeText = startedTime && !isNaN(startedTime.getTime())
            ? startedTime.toLocaleTimeString(taskTimeLocale, timeOpts)
            : '';

        const _t = function (k) { return typeof window.t === 'function' ? window.t(k) : k; };
        const statusMap = {
            'running': _t('tasks.statusRunning'),
            'cancelling': _t('tasks.statusCancelling'),
            'failed': _t('tasks.statusFailed'),
            'timeout': _t('tasks.statusTimeout'),
            'cancelled': _t('tasks.statusCancelled'),
            'completed': _t('tasks.statusCompleted')
        };
        const statusText = statusMap[task.status] || _t('tasks.statusRunning');
        const isFinalStatus = ['failed', 'timeout', 'cancelled', 'completed'].includes(task.status);
        const unnamedTaskText = _t('tasks.unnamedTask');
        const stopTaskBtnText = _t('tasks.stopTask');

        item.innerHTML = `
            <div class="active-task-info">
                <span class="active-task-status">${statusText}</span>
                <span class="active-task-message">${escapeHtml(task.message || unnamedTaskText)}</span>
            </div>
            <div class="active-task-actions">
                ${timeText ? `<span class="active-task-time">${timeText}</span>` : ''}
                ${!isFinalStatus ? '<button class="active-task-cancel">' + stopTaskBtnText + '</button>' : ''}
            </div>
        `;

 // 
        if (!isFinalStatus) {
            const cancelBtn = item.querySelector('.active-task-cancel');
            if (cancelBtn) {
                cancelBtn.onclick = () => cancelActiveTask(task.conversationId, cancelBtn);
                if (task.status === 'cancelling') {
                    cancelBtn.disabled = true;
 cancelBtn.textContent = typeof window.t === 'function' ? window.t('tasks.cancelling') : '...';
                }
            }
        }

        bar.appendChild(item);
    });
}

async function cancelActiveTask(conversationId, button) {
    if (!conversationId) return;
    const originalText = button.textContent;
    button.disabled = true;
 button.textContent = typeof window.t === 'function' ? window.t('tasks.cancelling') : '...';

    try {
        await requestCancel(conversationId);
        loadActiveTasks();
    } catch (error) {
 console.error(':', error);
 alert((typeof window.t === 'function' ? window.t('tasks.cancelTaskFailed') : '') + ': ' + error.message);
        button.disabled = false;
        button.textContent = originalText;
    }
}

// 
const monitorState = {
    executions: [],
    stats: {},
    lastFetchedAt: null,
    pagination: {
        page: 1,
        pageSize: (() => {
 // localStorage , 20
            const saved = localStorage.getItem('monitorPageSize');
            return saved ? parseInt(saved, 10) : 20;
        })(),
        total: 0,
        totalPages: 0
    }
};

function openMonitorPanel() {
 // MCP
    if (typeof switchPage === 'function') {
        switchPage('mcp-monitor');
    }
 // 
    initializeMonitorPageSize();
}

// 
function initializeMonitorPageSize() {
    const pageSizeSelect = document.getElementById('monitor-page-size');
    if (pageSizeSelect) {
        pageSizeSelect.value = monitorState.pagination.pageSize;
    }
}

// 
function changeMonitorPageSize() {
    const pageSizeSelect = document.getElementById('monitor-page-size');
    if (!pageSizeSelect) {
        return;
    }
    
    const newPageSize = parseInt(pageSizeSelect.value, 10);
    if (isNaN(newPageSize) || newPageSize <= 0) {
        return;
    }
    
 // localStorage
    localStorage.setItem('monitorPageSize', newPageSize.toString());
    
 // 
    monitorState.pagination.pageSize = newPageSize;
 monitorState.pagination.page = 1; // 
    
 // 
    refreshMonitorPanel(1);
}

function closeMonitorPanel() {
 // ,
 // ,
    if (typeof switchPage === 'function') {
        switchPage('chat');
    }
}

async function refreshMonitorPanel(page = null) {
    const statsContainer = document.getElementById('monitor-stats');
    const execContainer = document.getElementById('monitor-executions');

    try {
 // ,,
        const currentPage = page !== null ? page : monitorState.pagination.page;
        const pageSize = monitorState.pagination.pageSize;
        
 // 
        const statusFilter = document.getElementById('monitor-status-filter');
        const toolFilter = document.getElementById('monitor-tool-filter');
        const currentStatusFilter = statusFilter ? statusFilter.value : 'all';
        const currentToolFilter = toolFilter ? (toolFilter.value.trim() || 'all') : 'all';
        
 // URL
        let url = `/api/monitor?page=${currentPage}&page_size=${pageSize}`;
        if (currentStatusFilter && currentStatusFilter !== 'all') {
            url += `&status=${encodeURIComponent(currentStatusFilter)}`;
        }
        if (currentToolFilter && currentToolFilter !== 'all') {
            url += `&tool=${encodeURIComponent(currentToolFilter)}`;
        }
        
        const response = await apiFetch(url, { method: 'GET' });
        const result = await response.json().catch(() => ({}));
        if (!response.ok) {
 throw new Error(result.error || '');
        }

        monitorState.executions = Array.isArray(result.executions) ? result.executions : [];
        monitorState.stats = result.stats || {};
        monitorState.lastFetchedAt = new Date();
        
 // 
        if (result.total !== undefined) {
            monitorState.pagination = {
                page: result.page || currentPage,
                pageSize: result.page_size || pageSize,
                total: result.total || 0,
                totalPages: result.total_pages || 1
            };
        }

        renderMonitorStats(monitorState.stats, monitorState.lastFetchedAt);
        renderMonitorExecutions(monitorState.executions, currentStatusFilter);
        renderMonitorPagination();
        
 // 
        initializeMonitorPageSize();
    } catch (error) {
 console.error(':', error);
        if (statsContainer) {
 statsContainer.innerHTML = `<div class="monitor-error">${escapeHtml(typeof window.t === 'function' ? window.t('mcpMonitor.loadStatsError') : '')}:${escapeHtml(error.message)}</div>`;
        }
        if (execContainer) {
 execContainer.innerHTML = `<div class="monitor-error">${escapeHtml(typeof window.t === 'function' ? window.t('mcpMonitor.loadExecutionsError') : '')}:${escapeHtml(error.message)}</div>`;
        }
    }
}

// ()
let toolFilterDebounceTimer = null;
function handleToolFilterInput() {
 // 
    if (toolFilterDebounceTimer) {
        clearTimeout(toolFilterDebounceTimer);
    }
    
 // ,500ms
    toolFilterDebounceTimer = setTimeout(() => {
        applyMonitorFilters();
    }, 500);
}

async function applyMonitorFilters() {
    const statusFilter = document.getElementById('monitor-status-filter');
    const toolFilter = document.getElementById('monitor-tool-filter');
    const status = statusFilter ? statusFilter.value : 'all';
    const tool = toolFilter ? (toolFilter.value.trim() || 'all') : 'all';
 // ,
    await refreshMonitorPanelWithFilter(status, tool);
}

async function refreshMonitorPanelWithFilter(statusFilter = 'all', toolFilter = 'all') {
    const statsContainer = document.getElementById('monitor-stats');
    const execContainer = document.getElementById('monitor-executions');

    try {
 const currentPage = 1; // 
        const pageSize = monitorState.pagination.pageSize;
        
 // URL
        let url = `/api/monitor?page=${currentPage}&page_size=${pageSize}`;
        if (statusFilter && statusFilter !== 'all') {
            url += `&status=${encodeURIComponent(statusFilter)}`;
        }
        if (toolFilter && toolFilter !== 'all') {
            url += `&tool=${encodeURIComponent(toolFilter)}`;
        }
        
        const response = await apiFetch(url, { method: 'GET' });
        const result = await response.json().catch(() => ({}));
        if (!response.ok) {
 throw new Error(result.error || '');
        }

        monitorState.executions = Array.isArray(result.executions) ? result.executions : [];
        monitorState.stats = result.stats || {};
        monitorState.lastFetchedAt = new Date();
        
 // 
        if (result.total !== undefined) {
            monitorState.pagination = {
                page: result.page || currentPage,
                pageSize: result.page_size || pageSize,
                total: result.total || 0,
                totalPages: result.total_pages || 1
            };
        }

        renderMonitorStats(monitorState.stats, monitorState.lastFetchedAt);
        renderMonitorExecutions(monitorState.executions, statusFilter);
        renderMonitorPagination();
        
 // 
        initializeMonitorPageSize();
    } catch (error) {
 console.error(':', error);
        if (statsContainer) {
 statsContainer.innerHTML = `<div class="monitor-error">${escapeHtml(typeof window.t === 'function' ? window.t('mcpMonitor.loadStatsError') : '')}:${escapeHtml(error.message)}</div>`;
        }
        if (execContainer) {
 execContainer.innerHTML = `<div class="monitor-error">${escapeHtml(typeof window.t === 'function' ? window.t('mcpMonitor.loadExecutionsError') : '')}:${escapeHtml(error.message)}</div>`;
        }
    }
}


function renderMonitorStats(statsMap = {}, lastFetchedAt = null) {
    const container = document.getElementById('monitor-stats');
    if (!container) {
        return;
    }

    const entries = Object.values(statsMap);
    if (entries.length === 0) {
 const noStats = typeof window.t === 'function' ? window.t('mcpMonitor.noStatsData') : '';
        container.innerHTML = '<div class="monitor-empty">' + escapeHtml(noStats) + '</div>';
        return;
    }

 // 
    const totals = entries.reduce(
        (acc, item) => {
            acc.total += item.totalCalls || 0;
            acc.success += item.successCalls || 0;
            acc.failed += item.failedCalls || 0;
            const lastCall = item.lastCallTime ? new Date(item.lastCallTime) : null;
            if (lastCall && (!acc.lastCallTime || lastCall > acc.lastCallTime)) {
                acc.lastCallTime = lastCall;
            }
            return acc;
        },
        { total: 0, success: 0, failed: 0, lastCallTime: null }
    );

    const successRate = totals.total > 0 ? ((totals.success / totals.total) * 100).toFixed(1) : '0.0';
    const locale = (typeof window.__locale === 'string' && window.__locale.startsWith('zh')) ? 'zh-CN' : undefined;
    const lastUpdatedText = lastFetchedAt ? (lastFetchedAt.toLocaleString ? lastFetchedAt.toLocaleString(locale || 'en-US') : String(lastFetchedAt)) : 'N/A';
 const noCallsYet = typeof window.t === 'function' ? window.t('mcpMonitor.noCallsYet') : '';
    const lastCallText = totals.lastCallTime ? (totals.lastCallTime.toLocaleString ? totals.lastCallTime.toLocaleString(locale || 'en-US') : String(totals.lastCallTime)) : noCallsYet;
 const totalCallsLabel = typeof window.t === 'function' ? window.t('mcpMonitor.totalCalls') : '';
 const successFailedLabel = typeof window.t === 'function' ? window.t('mcpMonitor.successFailed', { success: totals.success, failed: totals.failed }) : ` ${totals.success} / ${totals.failed}`;
 const successRateLabel = typeof window.t === 'function' ? window.t('mcpMonitor.successRate') : '';
 const statsFromAll = typeof window.t === 'function' ? window.t('mcpMonitor.statsFromAllTools') : '';
 const lastCallLabel = typeof window.t === 'function' ? window.t('mcpMonitor.lastCall') : '';
 const lastRefreshLabel = typeof window.t === 'function' ? window.t('mcpMonitor.lastRefreshTime') : '';

    let html = `
        <div class="monitor-stat-card">
            <h4>${escapeHtml(totalCallsLabel)}</h4>
            <div class="monitor-stat-value">${totals.total}</div>
            <div class="monitor-stat-meta">${escapeHtml(successFailedLabel)}</div>
        </div>
        <div class="monitor-stat-card">
            <h4>${escapeHtml(successRateLabel)}</h4>
            <div class="monitor-stat-value">${successRate}%</div>
            <div class="monitor-stat-meta">${escapeHtml(statsFromAll)}</div>
        </div>
        <div class="monitor-stat-card">
            <h4>${escapeHtml(lastCallLabel)}</h4>
            <div class="monitor-stat-value" style="font-size:1rem;">${escapeHtml(lastCallText)}</div>
            <div class="monitor-stat-meta">${escapeHtml(lastRefreshLabel)}:${escapeHtml(lastUpdatedText)}</div>
        </div>
    `;

 // 4( totalCalls 0 )
    const topTools = entries
        .filter(tool => (tool.totalCalls || 0) > 0)
        .slice()
        .sort((a, b) => (b.totalCalls || 0) - (a.totalCalls || 0))
        .slice(0, 4);

 const unknownToolLabel = typeof window.t === 'function' ? window.t('mcpMonitor.unknownTool') : '';
    topTools.forEach(tool => {
        const toolSuccessRate = tool.totalCalls > 0 ? ((tool.successCalls || 0) / tool.totalCalls * 100).toFixed(1) : '0.0';
 const toolMeta = typeof window.t === 'function' ? window.t('mcpMonitor.successFailedRate', { success: tool.successCalls || 0, failed: tool.failedCalls || 0, rate: toolSuccessRate }) : ` ${tool.successCalls || 0} / ${tool.failedCalls || 0} · ${toolSuccessRate}%`;
        html += `
            <div class="monitor-stat-card">
                <h4>${escapeHtml(tool.toolName || unknownToolLabel)}</h4>
                <div class="monitor-stat-value">${tool.totalCalls || 0}</div>
                <div class="monitor-stat-meta">
                    ${escapeHtml(toolMeta)}
                </div>
            </div>
        `;
    });

    container.innerHTML = `<div class="monitor-stats-grid">${html}</div>`;
}

function renderMonitorExecutions(executions = [], statusFilter = 'all') {
    const container = document.getElementById('monitor-executions');
    if (!container) {
        return;
    }

    if (!Array.isArray(executions) || executions.length === 0) {
 // 
        const toolFilter = document.getElementById('monitor-tool-filter');
        const currentToolFilter = toolFilter ? toolFilter.value : 'all';
        const hasFilter = (statusFilter && statusFilter !== 'all') || (currentToolFilter && currentToolFilter !== 'all');
 const noRecordsFilter = typeof window.t === 'function' ? window.t('mcpMonitor.noRecordsWithFilter') : '';
 const noExecutions = typeof window.t === 'function' ? window.t('mcpMonitor.noExecutions') : '';
        if (hasFilter) {
            container.innerHTML = '<div class="monitor-empty">' + escapeHtml(noRecordsFilter) + '</div>';
        } else {
            container.innerHTML = '<div class="monitor-empty">' + escapeHtml(noExecutions) + '</div>';
        }
 // 
        const batchActions = document.getElementById('monitor-batch-actions');
        if (batchActions) {
            batchActions.style.display = 'none';
        }
        return;
    }

 // ,
 // ,
 const unknownLabel = typeof window.t === 'function' ? window.t('mcpMonitor.unknown') : '';
 const unknownToolLabel = typeof window.t === 'function' ? window.t('mcpMonitor.unknownTool') : '';
 const viewDetailLabel = typeof window.t === 'function' ? window.t('mcpMonitor.viewDetail') : '';
 const deleteLabel = typeof window.t === 'function' ? window.t('mcpMonitor.delete') : '';
 const deleteExecTitle = typeof window.t === 'function' ? window.t('mcpMonitor.deleteExecTitle') : '';
    const statusKeyMap = { pending: 'statusPending', running: 'statusRunning', completed: 'statusCompleted', failed: 'statusFailed' };
    const locale = (typeof window.__locale === 'string' && window.__locale.startsWith('zh')) ? 'zh-CN' : undefined;
    const rows = executions
        .map(exec => {
            const status = (exec.status || 'unknown').toLowerCase();
            const statusClass = `monitor-status-chip ${status}`;
            const statusKey = statusKeyMap[status];
            const statusLabel = (typeof window.t === 'function' && statusKey) ? window.t('mcpMonitor.' + statusKey) : getStatusText(status);
            const startTime = exec.startTime ? (new Date(exec.startTime).toLocaleString ? new Date(exec.startTime).toLocaleString(locale || 'en-US') : String(exec.startTime)) : unknownLabel;
            const duration = formatExecutionDuration(exec.startTime, exec.endTime);
            const toolName = escapeHtml(exec.toolName || unknownToolLabel);
            const executionId = escapeHtml(exec.id || '');
            return `
                <tr>
                    <td>
                        <input type="checkbox" class="monitor-execution-checkbox" value="${executionId}" onchange="updateBatchActionsState()" />
                    </td>
                    <td>${toolName}</td>
                    <td><span class="${statusClass}">${escapeHtml(statusLabel)}</span></td>
                    <td>${escapeHtml(startTime)}</td>
                    <td>${escapeHtml(duration)}</td>
                    <td>
                        <div class="monitor-execution-actions">
                            <button class="btn-secondary" onclick="showMCPDetail('${executionId}')">${escapeHtml(viewDetailLabel)}</button>
                            <button class="btn-secondary btn-delete" onclick="deleteExecution('${executionId}')" title="${escapeHtml(deleteExecTitle)}">${escapeHtml(deleteLabel)}</button>
                        </div>
                    </td>
                </tr>
            `;
        })
        .join('');

 // ()
    const oldTableContainer = container.querySelector('.monitor-table-container');
    if (oldTableContainer) {
        oldTableContainer.remove();
    }
 // "..."
    const oldEmpty = container.querySelector('.monitor-empty');
    if (oldEmpty) {
        oldEmpty.remove();
    }
    
 // 
    const tableContainer = document.createElement('div');
    tableContainer.className = 'monitor-table-container';
 const colTool = typeof window.t === 'function' ? window.t('mcpMonitor.columnTool') : '';
 const colStatus = typeof window.t === 'function' ? window.t('mcpMonitor.columnStatus') : '';
 const colStartTime = typeof window.t === 'function' ? window.t('mcpMonitor.columnStartTime') : '';
 const colDuration = typeof window.t === 'function' ? window.t('mcpMonitor.columnDuration') : '';
 const colActions = typeof window.t === 'function' ? window.t('mcpMonitor.columnActions') : '';
    tableContainer.innerHTML = `
        <table class="monitor-table">
            <thead>
                <tr>
                    <th style="width: 40px;">
                        <input type="checkbox" id="monitor-select-all" onchange="toggleSelectAll(this)" />
                    </th>
                    <th>${escapeHtml(colTool)}</th>
                    <th>${escapeHtml(colStatus)}</th>
                    <th>${escapeHtml(colStartTime)}</th>
                    <th>${escapeHtml(colDuration)}</th>
                    <th>${escapeHtml(colActions)}</th>
                </tr>
            </thead>
            <tbody>${rows}</tbody>
        </table>
    `;
    
 // ()
    const existingPagination = container.querySelector('.monitor-pagination');
    if (existingPagination) {
        container.insertBefore(tableContainer, existingPagination);
    } else {
        container.appendChild(tableContainer);
    }
    
 // 
    updateBatchActionsState();
}

// 
function renderMonitorPagination() {
    const container = document.getElementById('monitor-executions');
    if (!container) return;
    
 // 
    const oldPagination = container.querySelector('.monitor-pagination');
    if (oldPagination) {
        oldPagination.remove();
    }
    
    const { page, totalPages, total, pageSize } = monitorState.pagination;
    
 // 
    const pagination = document.createElement('div');
    pagination.className = 'monitor-pagination';
    
 // 
    const startItem = total === 0 ? 0 : (page - 1) * pageSize + 1;
    const endItem = total === 0 ? 0 : Math.min(page * pageSize, total);
 const paginationInfoText = typeof window.t === 'function' ? window.t('mcpMonitor.paginationInfo', { start: startItem, end: endItem, total: total }) : ` ${startItem}-${endItem} / ${total} `;
 const perPageLabel = typeof window.t === 'function' ? window.t('mcpMonitor.perPageLabel') : '';
 const firstPageLabel = typeof window.t === 'function' ? window.t('mcp.firstPage') : '';
 const prevPageLabel = typeof window.t === 'function' ? window.t('mcp.prevPage') : '';
 const pageInfoText = typeof window.t === 'function' ? window.t('mcp.pageInfo', { page: page, total: totalPages || 1 }) : ` ${page} / ${totalPages || 1} `;
 const nextPageLabel = typeof window.t === 'function' ? window.t('mcp.nextPage') : '';
 const lastPageLabel = typeof window.t === 'function' ? window.t('mcp.lastPage') : '';
    pagination.innerHTML = `
        <div class="pagination-info">
            <span>${escapeHtml(paginationInfoText)}</span>
            <label class="pagination-page-size">
                ${escapeHtml(perPageLabel)}
                <select id="monitor-page-size" onchange="changeMonitorPageSize()">
                    <option value="10" ${pageSize === 10 ? 'selected' : ''}>10</option>
                    <option value="20" ${pageSize === 20 ? 'selected' : ''}>20</option>
                    <option value="50" ${pageSize === 50 ? 'selected' : ''}>50</option>
                    <option value="100" ${pageSize === 100 ? 'selected' : ''}>100</option>
                </select>
            </label>
        </div>
        <div class="pagination-controls">
            <button class="btn-secondary" onclick="refreshMonitorPanel(1)" ${page === 1 || total === 0 ? 'disabled' : ''}>${escapeHtml(firstPageLabel)}</button>
            <button class="btn-secondary" onclick="refreshMonitorPanel(${page - 1})" ${page === 1 || total === 0 ? 'disabled' : ''}>${escapeHtml(prevPageLabel)}</button>
            <span class="pagination-page">${escapeHtml(pageInfoText)}</span>
            <button class="btn-secondary" onclick="refreshMonitorPanel(${page + 1})" ${page >= totalPages || total === 0 ? 'disabled' : ''}>${escapeHtml(nextPageLabel)}</button>
            <button class="btn-secondary" onclick="refreshMonitorPanel(${totalPages || 1})" ${page >= totalPages || total === 0 ? 'disabled' : ''}>${escapeHtml(lastPageLabel)}</button>
        </div>
    `;
    
    container.appendChild(pagination);
    
 // 
    initializeMonitorPageSize();
}

// 
async function deleteExecution(executionId) {
    if (!executionId) {
        return;
    }
    
 const deleteConfirmMsg = typeof window.t === 'function' ? window.t('mcpMonitor.deleteExecConfirmSingle') : '?.';
    if (!confirm(deleteConfirmMsg)) {
        return;
    }
    
    try {
        const response = await apiFetch(`/api/monitor/execution/${executionId}`, {
            method: 'DELETE'
        });
        
        if (!response.ok) {
            const error = await response.json().catch(() => ({}));
 const deleteFailedMsg = typeof window.t === 'function' ? window.t('mcpMonitor.deleteExecFailed') : '';
            throw new Error(error.error || deleteFailedMsg);
        }
        
 // 
        const currentPage = monitorState.pagination.page;
        await refreshMonitorPanel(currentPage);
        
 const execDeletedMsg = typeof window.t === 'function' ? window.t('mcpMonitor.execDeleted') : '';
        alert(execDeletedMsg);
    } catch (error) {
 console.error(':', error);
 const deleteFailedMsg = typeof window.t === 'function' ? window.t('mcpMonitor.deleteExecFailed') : '';
        alert(deleteFailedMsg + ': ' + error.message);
    }
}

// 
function updateBatchActionsState() {
    const checkboxes = document.querySelectorAll('.monitor-execution-checkbox:checked');
    const selectedCount = checkboxes.length;
    const batchActions = document.getElementById('monitor-batch-actions');
    const selectedCountSpan = document.getElementById('monitor-selected-count');
    
    if (selectedCount > 0) {
        if (batchActions) {
            batchActions.style.display = 'flex';
        }
    } else {
        if (batchActions) {
            batchActions.style.display = 'none';
        }
    }
    if (selectedCountSpan) {
 selectedCountSpan.textContent = typeof window.t === 'function' ? window.t('mcp.selectedCount', { count: selectedCount }) : ' ' + selectedCount + ' ';
    }
    
 // 
    const selectAllCheckbox = document.getElementById('monitor-select-all');
    if (selectAllCheckbox) {
        const allCheckboxes = document.querySelectorAll('.monitor-execution-checkbox');
        const allChecked = allCheckboxes.length > 0 && Array.from(allCheckboxes).every(cb => cb.checked);
        selectAllCheckbox.checked = allChecked;
        selectAllCheckbox.indeterminate = selectedCount > 0 && selectedCount < allCheckboxes.length;
    }
}

// 
function toggleSelectAll(checkbox) {
    const checkboxes = document.querySelectorAll('.monitor-execution-checkbox');
    checkboxes.forEach(cb => {
        cb.checked = checkbox.checked;
    });
    updateBatchActionsState();
}

// 
function selectAllExecutions() {
    const checkboxes = document.querySelectorAll('.monitor-execution-checkbox');
    checkboxes.forEach(cb => {
        cb.checked = true;
    });
    const selectAllCheckbox = document.getElementById('monitor-select-all');
    if (selectAllCheckbox) {
        selectAllCheckbox.checked = true;
        selectAllCheckbox.indeterminate = false;
    }
    updateBatchActionsState();
}

// 
function deselectAllExecutions() {
    const checkboxes = document.querySelectorAll('.monitor-execution-checkbox');
    checkboxes.forEach(cb => {
        cb.checked = false;
    });
    const selectAllCheckbox = document.getElementById('monitor-select-all');
    if (selectAllCheckbox) {
        selectAllCheckbox.checked = false;
        selectAllCheckbox.indeterminate = false;
    }
    updateBatchActionsState();
}

// 
async function batchDeleteExecutions() {
    const checkboxes = document.querySelectorAll('.monitor-execution-checkbox:checked');
    if (checkboxes.length === 0) {
 const selectFirstMsg = typeof window.t === 'function' ? window.t('mcpMonitor.selectExecFirst') : '';
        alert(selectFirstMsg);
        return;
    }
    
    const ids = Array.from(checkboxes).map(cb => cb.value);
    const count = ids.length;
 const batchConfirmMsg = typeof window.t === 'function' ? window.t('mcpMonitor.batchDeleteConfirm', { count: count }) : ` ${count} ?.`;
    if (!confirm(batchConfirmMsg)) {
        return;
    }
    
    try {
        const response = await apiFetch('/api/monitor/executions', {
            method: 'DELETE',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ ids: ids })
        });
        
        if (!response.ok) {
            const error = await response.json().catch(() => ({}));
 const batchFailedMsg = typeof window.t === 'function' ? window.t('mcp.batchDeleteFailed') : '';
            throw new Error(error.error || batchFailedMsg);
        }
        
        const result = await response.json().catch(() => ({}));
        const deletedCount = result.deleted || count;
        
 // 
        const currentPage = monitorState.pagination.page;
        await refreshMonitorPanel(currentPage);
        
 const batchSuccessMsg = typeof window.t === 'function' ? window.t('mcpMonitor.batchDeleteSuccess', { count: deletedCount }) : ` ${deletedCount} `;
        alert(batchSuccessMsg);
    } catch (error) {
 console.error(':', error);
 const batchFailedMsg = typeof window.t === 'function' ? window.t('mcp.batchDeleteFailed') : '';
        alert(batchFailedMsg + ': ' + error.message);
    }
}

function formatExecutionDuration(start, end) {
 const unknownLabel = typeof window.t === 'function' ? window.t('mcpMonitor.unknown') : '';
    if (!start) {
        return unknownLabel;
    }
    const startTime = new Date(start);
    const endTime = end ? new Date(end) : new Date();
    if (Number.isNaN(startTime.getTime()) || Number.isNaN(endTime.getTime())) {
        return unknownLabel;
    }
    const diffMs = Math.max(0, endTime - startTime);
    const seconds = Math.floor(diffMs / 1000);
    if (seconds < 60) {
 return typeof window.t === 'function' ? window.t('mcpMonitor.durationSeconds', { n: seconds }) : seconds + ' ';
    }
    const minutes = Math.floor(seconds / 60);
    if (minutes < 60) {
        const remain = seconds % 60;
        if (remain > 0) {
 return typeof window.t === 'function' ? window.t('mcpMonitor.durationMinutes', { minutes: minutes, seconds: remain }) : minutes + ' ' + remain + ' ';
        }
 return typeof window.t === 'function' ? window.t('mcpMonitor.durationMinutesOnly', { minutes: minutes }) : minutes + ' ';
    }
    const hours = Math.floor(minutes / 60);
    const remainMinutes = minutes % 60;
    if (remainMinutes > 0) {
 return typeof window.t === 'function' ? window.t('mcpMonitor.durationHours', { hours: hours, minutes: remainMinutes }) : hours + ' ' + remainMinutes + ' ';
    }
 return typeof window.t === 'function' ? window.t('mcpMonitor.durationHoursOnly', { hours: hours }) : hours + ' ';
}

/**
 * ,( AM/PM)
 */
function refreshProgressAndTimelineI18n() {
    const _t = function (k, o) {
        return typeof window.t === 'function' ? window.t(k, o) : k;
    };
    const timeLocale = getCurrentTimeLocale();
    const timeOpts = getTimeFormatOptions();

 // :""( Stop task)
    document.querySelectorAll('.progress-message .progress-stop').forEach(function (btn) {
        if (!btn.disabled && btn.id && btn.id.indexOf('-stop-btn') !== -1) {
            const cancelling = _t('tasks.cancelling');
            if (btn.textContent !== cancelling) {
                btn.textContent = _t('tasks.stopTask');
            }
        }
    });
    document.querySelectorAll('.progress-toggle').forEach(function (btn) {
        const timeline = btn.closest('.progress-container, .message-bubble') &&
            btn.closest('.progress-container, .message-bubble').querySelector('.progress-timeline');
        const expanded = timeline && timeline.classList.contains('expanded');
        btn.textContent = expanded ? _t('tasks.collapseDetail') : _t('chat.expandDetail');
    });
    document.querySelectorAll('.progress-message').forEach(function (msgEl) {
        const raw = msgEl.dataset.progressRawMessage;
        const titleEl = msgEl.querySelector('.progress-title');
        if (titleEl && raw) {
            titleEl.textContent = '\uD83D\uDD0D ' + translateProgressMessage(raw);
        }
    });
 // "": .progress-message progress 
    document.querySelectorAll('.progress-container .progress-header .progress-title').forEach(function (titleEl) {
        if (titleEl.closest('.progress-message')) return;
        titleEl.textContent = '\uD83D\uDCCB ' + _t('chat.penetrationTestDetail');
    });

 // :,
    document.querySelectorAll('.timeline-item').forEach(function (item) {
        const type = item.dataset.timelineType;
        const titleSpan = item.querySelector('.timeline-item-title');
        const timeSpan = item.querySelector('.timeline-item-time');
        if (!titleSpan) return;
        const ap = (item.dataset.einoAgent && item.dataset.einoAgent !== '') ? ('[' + item.dataset.einoAgent + '] ') : '';
        if (type === 'iteration' && item.dataset.iterationN) {
            const n = parseInt(item.dataset.iterationN, 10) || 1;
            const scope = item.dataset.einoScope;
            if (scope === 'main') {
                titleSpan.textContent = _t('chat.einoOrchestratorRound', { n: n });
            } else if (scope === 'sub') {
                const agent = item.dataset.einoAgent || '';
                titleSpan.textContent = _t('chat.einoSubAgentStep', { n: n, agent: agent });
            } else {
                titleSpan.textContent = ap + _t('chat.iterationRound', { n: n });
            }
        } else if (type === 'thinking') {
            titleSpan.textContent = ap + '\uD83E\uDD14 ' + _t('chat.aiThinking');
        } else if (type === 'tool_calls_detected' && item.dataset.toolCallsCount != null) {
            const count = parseInt(item.dataset.toolCallsCount, 10) || 0;
            titleSpan.textContent = ap + '\uD83D\uDD27 ' + _t('chat.toolCallsDetected', { count: count });
        } else if (type === 'tool_call' && (item.dataset.toolName !== undefined || item.dataset.toolIndex !== undefined)) {
            const name = (item.dataset.toolName != null && item.dataset.toolName !== '') ? item.dataset.toolName : _t('chat.unknownTool');
            const index = parseInt(item.dataset.toolIndex, 10) || 0;
            const total = parseInt(item.dataset.toolTotal, 10) || 0;
            titleSpan.textContent = ap + '\uD83D\uDD27 ' + _t('chat.callTool', { name: name, index: index, total: total });
        } else if (type === 'tool_result' && (item.dataset.toolName !== undefined || item.dataset.toolSuccess !== undefined)) {
            const name = (item.dataset.toolName != null && item.dataset.toolName !== '') ? item.dataset.toolName : _t('chat.unknownTool');
            const success = item.dataset.toolSuccess === '1';
            const icon = success ? '\u2705 ' : '\u274C ';
            titleSpan.textContent = ap + icon + (success ? _t('chat.toolExecComplete', { name: name }) : _t('chat.toolExecFailed', { name: name }));
        } else if (type === 'eino_agent_reply') {
            titleSpan.textContent = ap + '\uD83D\uDCAC ' + _t('chat.einoAgentReplyTitle');
        } else if (type === 'cancelled') {
            titleSpan.textContent = '\u26D4 ' + _t('chat.taskCancelled');
        } else if (type === 'progress' && item.dataset.progressMessage !== undefined) {
            titleSpan.textContent = typeof window.translateProgressMessage === 'function' ? window.translateProgressMessage(item.dataset.progressMessage) : item.dataset.progressMessage;
        }
        if (timeSpan && item.dataset.createdAtIso) {
            const d = new Date(item.dataset.createdAtIso);
            if (!isNaN(d.getTime())) {
                timeSpan.textContent = d.toLocaleTimeString(timeLocale, timeOpts);
            }
        }
    });

 // "/"
    document.querySelectorAll('.process-detail-btn span').forEach(function (span) {
        const btn = span.closest('.process-detail-btn');
        const assistantId = btn && btn.closest('.message.assistant') && btn.closest('.message.assistant').id;
        if (!assistantId) return;
        const detailsId = 'process-details-' + assistantId;
        const timeline = document.getElementById(detailsId) && document.getElementById(detailsId).querySelector('.progress-timeline');
        const expanded = timeline && timeline.classList.contains('expanded');
        span.textContent = expanded ? _t('tasks.collapseDetail') : _t('chat.expandDetail');
    });
}

document.addEventListener('languagechange', function () {
    updateBatchActionsState();
    loadActiveTasks();
    refreshProgressAndTimelineI18n();
});
