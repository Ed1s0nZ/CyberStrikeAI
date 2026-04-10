const progressTaskState = new Map();
let activeTaskInterval = null;
const ACTIVE_TASK_REFRESH_INTERVAL = 10000; // 10秒检查一次
const TASK_FINAL_STATUSES = new Set(['failed', 'timeout', 'cancelled', 'completed']);

// 当前界面语言对应的 BCP 47 标签（与时间格式化一致）
function getCurrentTimeLocale() {
    if (typeof window.__locale === 'string' && window.__locale.length) {
        return window.__locale.startsWith('zh') ? 'zh-CN' : 'en-US';
    }
    if (typeof i18next !== 'undefined' && i18next.language) {
        return (i18next.language || '').startsWith('zh') ? 'zh-CN' : 'en-US';
    }
    return 'zh-CN';
}

// toLocaleTimeString 选项：中文用 24 小时制，避免仍显示 AM/PM
function getTimeFormatOptions() {
    const loc = getCurrentTimeLocale();
    const base = { hour: '2-digit', minute: '2-digit', second: '2-digit' };
    if (loc === 'zh-CN') {
        base.hour12 = false;
    }
    return base;
}

// 将后端下发的进度文案转为当前语言的翻译（中英双向映射，切换语言后能跟上）
function translateProgressMessage(message) {
    if (!message || typeof message !== 'string') return message;
    if (typeof window.t !== 'function') return message;
    const trim = message.trim();
    const map = {
        // 中文
        '正在调用AI模型...': 'progress.callingAI',
        '最后一次迭代：正在生成总结和下一步计划...': 'progress.lastIterSummary',
        '总结生成完成': 'progress.summaryDone',
        '正在生成最终回复...': 'progress.generatingFinalReply',
        '达到最大迭代次数，正在生成总结...': 'progress.maxIterSummary',
        '正在分析您的请求...': 'progress.analyzingRequestShort',
        '开始分析请求并制定测试策略': 'progress.analyzingRequestPlanning',
        '正在启动 Eino DeepAgent...': 'progress.startingEinoDeepAgent',
        // 英文（与 en-US.json 一致，避免后端/缓存已是英文时无法随语言切换）
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
    const callingToolPrefixCn = '正在调用工具: ';
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

// 存储工具调用ID到DOM元素的映射，用于更新执行状态
const toolCallStatusMap = new Map();

function finalizeOutstandingToolCallsForProgress(progressId, finalStatus) {
    if (!progressId) return;
    const pid = String(progressId);
    for (const [toolCallId, mapping] of Array.from(toolCallStatusMap.entries())) {
        if (!mapping) continue;
        if (mapping.progressId != null && String(mapping.progressId) !== pid) continue;
        updateToolCallStatus(toolCallId, finalStatus);
        toolCallStatusMap.delete(toolCallId);
    }
}

// 模型流式输出缓存：progressId -> { assistantId, buffer }
const responseStreamStateByProgressId = new Map();

// AI 思考流式输出：progressId -> Map(streamId -> { itemId, buffer })
const thinkingStreamStateByProgressId = new Map();

// Eino 子代理回复流式：progressId -> Map(streamId -> { itemId, buffer })
const einoAgentReplyStreamStateByProgressId = new Map();

// 工具输出流式增量：progressId::toolCallId -> { itemId, buffer }
const toolResultStreamStateByKey = new Map();
function toolResultStreamKey(progressId, toolCallId) {
    return String(progressId) + '::' + String(toolCallId);
}

/** Eino 多代理：时间线标题前加 [agentId]，标明哪一代理产生该工具调用/结果/回复 */
function timelineAgentBracketPrefix(data) {
    if (!data || data.einoAgent == null) return '';
    const s = String(data.einoAgent).trim();
    return s ? ('[' + s + '] ') : '';
}

/** 主/子代理视觉区分：左边框与浅底色（与工具黄/绿状态并存时由具体项类型覆盖次要边） */
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

// markdown 渲染（用于最终合并渲染；流式增量阶段用纯转义避免部分语法不稳定）
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

    // 保留复制按钮：addMessage 会把按钮 append 在 message-bubble 里
    const copyBtn = bubble.querySelector('.message-copy-btn');
    if (copyBtn) copyBtn.remove();

    const newContent = content == null ? '' : String(content);
    const html = renderMarkdown
        ? formatAssistantMarkdownContent(newContent)
        : escapeHtmlLocal(newContent).replace(/\n/g, '<br>');

    bubble.innerHTML = html;

    // 更新原始内容（给复制功能用）
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

/** 距底部该像素内视为「跟随底部」；流式输出时仅在此情况下自动滚到底部，避免用户上滑查看历史时被强制拉回 */
const CHAT_SCROLL_PIN_THRESHOLD_PX = 120;

/** wasPinned 须在 DOM 追加内容之前计算，否则 scrollHeight 变大后会误判 */
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
            stopBtn.textContent = typeof window.t === 'function' ? window.t('tasks.statusCompleted') : '已完成';
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
        throw new Error(result.error || (typeof window.t === 'function' ? window.t('tasks.cancelFailed') : '取消失败'));
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
    const progressTitleText = typeof window.t === 'function' ? window.t('chat.progressInProgress') : '渗透测试进行中...';
    const stopTaskText = typeof window.t === 'function' ? window.t('tasks.stopTask') : '停止任务';
    const collapseDetailText = typeof window.t === 'function' ? window.t('tasks.collapseDetail') : '收起详情';
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

// 切换进度详情显示
function toggleProgressDetails(progressId) {
    const timeline = document.getElementById(progressId + '-timeline');
    const toggleBtns = document.querySelectorAll(`#${progressId} .progress-toggle`);
    
    if (!timeline || !toggleBtns.length) return;
    
    const expandT = typeof window.t === 'function' ? window.t('chat.expandDetail') : '展开详情';
    const collapseT = typeof window.t === 'function' ? window.t('tasks.collapseDetail') : '收起详情';
    if (timeline.classList.contains('expanded')) {
        timeline.classList.remove('expanded');
        toggleBtns.forEach((btn) => { btn.textContent = expandT; });
    } else {
        timeline.classList.add('expanded');
        toggleBtns.forEach((btn) => { btn.textContent = collapseT; });
    }
}

// 编排器开始输出最终回复时隐藏整条进度消息（迭代阶段保持展开可见；此处整行收起而非仅折叠时间线）
function hideProgressMessageForFinalReply(progressId) {
    if (!progressId) return;
    const el = document.getElementById(progressId);
    if (el) {
        el.style.display = 'none';
    }
}

// 折叠所有进度详情
function collapseAllProgressDetails(assistantMessageId, progressId) {
    // 折叠集成到MCP区域的详情
    if (assistantMessageId) {
        const detailsId = 'process-details-' + assistantMessageId;
        const detailsContainer = document.getElementById(detailsId);
        if (detailsContainer) {
            const timeline = detailsContainer.querySelector('.progress-timeline');
            if (timeline) {
                // 确保移除expanded类（无论是否包含）
                timeline.classList.remove('expanded');
                document.querySelectorAll(`#${assistantMessageId} .process-detail-btn`).forEach((btn) => {
                    btn.innerHTML = '<span>' + (typeof window.t === 'function' ? window.t('chat.expandDetail') : '展开详情') + '</span>';
                });
            }
        }
    }
    
    // 折叠独立的详情组件（通过convertProgressToDetails创建的）
    // 查找所有以details-开头的详情组件
    const allDetails = document.querySelectorAll('[id^="details-"]');
    allDetails.forEach(detail => {
        const timeline = detail.querySelector('.progress-timeline');
        const toggleBtns = detail.querySelectorAll('.progress-toggle');
        if (timeline) {
            timeline.classList.remove('expanded');
            const expandT = typeof window.t === 'function' ? window.t('chat.expandDetail') : '展开详情';
            toggleBtns.forEach((btn) => { btn.textContent = expandT; });
        }
    });
    
    // 折叠原始的进度消息（如果还存在）
    if (progressId) {
        const progressTimeline = document.getElementById(progressId + '-timeline');
        const progressToggleBtns = document.querySelectorAll(`#${progressId} .progress-toggle`);
        if (progressTimeline) {
            progressTimeline.classList.remove('expanded');
            const expandT = typeof window.t === 'function' ? window.t('chat.expandDetail') : '展开详情';
            progressToggleBtns.forEach((btn) => { btn.textContent = expandT; });
        }
    }
}

// 获取当前助手消息ID（用于done事件）
function getAssistantId() {
    // 从最近的助手消息中获取ID
    const messages = document.querySelectorAll('.message.assistant');
    if (messages.length > 0) {
        return messages[messages.length - 1].id;
    }
    return null;
}

// 将进度详情集成到工具调用区域（流式阶段助手消息不挂 mcp 条，结束时在此创建，避免图二整行 MCP 芯片样式）
function integrateProgressToMCPSection(progressId, assistantMessageId, mcpExecutionIds) {
    const progressElement = document.getElementById(progressId);
    if (!progressElement) return;

    // Ensure any "running" tool_call badges are closed before we snapshot timeline HTML.
    // Otherwise, once the progress element is removed, later 'done' events may not be able
    // to update the original timeline DOM and the copied HTML would stay "执行中".
    finalizeOutstandingToolCallsForProgress(progressId, 'failed');

    const mcpIds = Array.isArray(mcpExecutionIds) ? mcpExecutionIds : [];
    
    // 获取时间线内容
    const timeline = document.getElementById(progressId + '-timeline');
    let timelineHTML = '';
    if (timeline) {
        timelineHTML = timeline.innerHTML;
    }
    
    // 获取助手消息元素
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
    
    // 查找或创建 MCP 区域
    let mcpSection = assistantElement.querySelector('.mcp-call-section');
    if (!mcpSection) {
        mcpSection = document.createElement('div');
        mcpSection.className = 'mcp-call-section';
        const mcpLabel = document.createElement('div');
        mcpLabel.className = 'mcp-call-label';
        mcpLabel.textContent = '📋 ' + (typeof window.t === 'function' ? window.t('chat.penetrationTestDetail') : '渗透测试详情');
        mcpSection.appendChild(mcpLabel);
        const buttonsContainerInit = document.createElement('div');
        buttonsContainerInit.className = 'mcp-call-buttons';
        mcpSection.appendChild(buttonsContainerInit);
        contentWrapper.appendChild(mcpSection);
    }
    
    // 获取时间线内容
    const hasContent = timelineHTML.trim().length > 0;
    
    // 检查时间线中是否有错误项
    const hasError = timeline && timeline.querySelector('.timeline-item-error');
    
    // 确保按钮容器存在
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
            detailBtn.dataset.execId = execId;
            detailBtn.dataset.execIndex = String(index + 1);
            detailBtn.innerHTML = '<span>' + (typeof window.t === 'function' ? window.t('chat.callNumber', { n: index + 1 }) : '调用 #' + (index + 1)) + '</span>';
            detailBtn.onclick = () => showMCPDetail(execId);
            buttonsContainer.appendChild(detailBtn);
        });
        // 使用批量 API 一次性获取所有工具名称（消除 N 次单独请求）
        if (typeof batchUpdateButtonToolNames === 'function') {
            batchUpdateButtonToolNames(buttonsContainer, mcpIds);
        }
    }
    if (!buttonsContainer.querySelector('.process-detail-btn')) {
        const progressDetailBtn = document.createElement('button');
        progressDetailBtn.className = 'mcp-detail-btn process-detail-btn';
        progressDetailBtn.innerHTML = '<span>' + (typeof window.t === 'function' ? window.t('chat.expandDetail') : '展开详情') + '</span>';
        progressDetailBtn.onclick = () => toggleProcessDetails(null, assistantMessageId);
        buttonsContainer.appendChild(progressDetailBtn);
    }
    
    // 创建详情容器，放在MCP按钮区域下方（统一结构）
    const detailsId = 'process-details-' + assistantMessageId;
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
    
    // 设置详情内容（如果有错误，默认折叠；否则默认折叠）
    detailsContainer.innerHTML = `
        <div class="process-details-content">
            ${hasContent ? `<div class="progress-timeline" id="${detailsId}-timeline">${timelineHTML}</div>` : '<div class="progress-timeline-empty">' + (typeof window.t === 'function' ? window.t('chat.noProcessDetail') : '暂无过程详情（可能执行过快或未触发详细事件）') + '</div>'}
        </div>
    `;
    
    // 确保初始状态是折叠的（默认折叠，特别是错误时）
    if (hasContent) {
        const timeline = document.getElementById(detailsId + '-timeline');
        if (timeline) {
            // 如果有错误，确保折叠；否则也默认折叠
            timeline.classList.remove('expanded');
        }
        
        const expandLabel = typeof window.t === 'function' ? window.t('chat.expandDetail') : '展开详情';
        document.querySelectorAll(`#${assistantMessageId} .process-detail-btn`).forEach((btn) => {
            btn.innerHTML = '<span>' + expandLabel + '</span>';
        });
    }
    
    // 移除原来的进度消息
    removeMessage(progressId);
}

// 切换过程详情显示
function toggleProcessDetails(progressId, assistantMessageId) {
    const detailsId = 'process-details-' + assistantMessageId;
    const detailsContainer = document.getElementById(detailsId);
    if (!detailsContainer) return;

    // 懒加载：首次展开时才从后端拉取该条消息的过程详情
    const maybeLazy = detailsContainer.dataset && detailsContainer.dataset.lazyNotLoaded === '1' && detailsContainer.dataset.loaded !== '1';
    if (maybeLazy) {
        const messageEl = document.getElementById(assistantMessageId);
        const backendMessageId = messageEl && messageEl.dataset ? messageEl.dataset.backendMessageId : '';
        if (backendMessageId && typeof apiFetch === 'function' && typeof renderProcessDetails === 'function') {
            if (detailsContainer.dataset.loading === '1') {
                // 正在加载中，避免重复请求
            } else {
                detailsContainer.dataset.loading = '1';
                // 先展开容器，显示加载态
                const timeline = detailsContainer.querySelector('.progress-timeline');
                if (timeline) {
                    timeline.innerHTML = '<div class="progress-timeline-empty">' + ((typeof window.t === 'function') ? window.t('common.loading') : '加载中…') + '</div>';
                }
                apiFetch(`/api/messages/${encodeURIComponent(String(backendMessageId))}/process-details`)
                    .then(async (res) => {
                        const j = await res.json().catch(() => ({}));
                        if (!res.ok) throw new Error((j && j.error) ? j.error : res.status);
                        const details = (j && Array.isArray(j.processDetails)) ? j.processDetails : [];
                        // 重新渲染详情（renderProcessDetails 会清掉 lazy 标记并写入 loaded）
                        renderProcessDetails(assistantMessageId, details);
                    })
                    .catch((e) => {
                        console.error('加载过程详情失败:', e);
                        const tl = detailsContainer.querySelector('.progress-timeline');
                        if (tl) {
                            tl.innerHTML = '<div class="progress-timeline-empty">' + ((typeof window.t === 'function') ? window.t('chat.noProcessDetail') : '暂无过程详情（加载失败）') + '</div>';
                        }
                        // 失败时保留 lazy 状态，允许用户重试
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
    
    const expandT = typeof window.t === 'function' ? window.t('chat.expandDetail') : '展开详情';
    const collapseT = typeof window.t === 'function' ? window.t('tasks.collapseDetail') : '收起详情';
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
    
    // 滚动到展开的详情位置，而不是滚动到底部
    if (timeline && timeline.classList.contains('expanded')) {
        setTimeout(() => {
            // 使用 scrollIntoView 滚动到详情容器位置
            detailsContainer.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
        }, 100);
    }
}

// 停止当前进度对应的任务
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
        alert(typeof window.t === 'function' ? window.t('tasks.taskInfoNotSynced') : '任务信息尚未同步，请稍后再试。');
        return;
    }

    if (state.cancelling) {
        return;
    }

    markProgressCancelling(progressId);
    if (stopBtn) {
        stopBtn.disabled = true;
        stopBtn.textContent = typeof window.t === 'function' ? window.t('tasks.cancelling') : '取消中...';
    }

    try {
        await requestCancel(state.conversationId);
        loadActiveTasks();
    } catch (error) {
        console.error('取消任务失败:', error);
        alert((typeof window.t === 'function' ? window.t('tasks.cancelTaskFailed') : '取消任务失败') + ': ' + error.message);
        if (stopBtn) {
            stopBtn.disabled = false;
            stopBtn.textContent = typeof window.t === 'function' ? window.t('tasks.stopTask') : '停止任务';
        }
        const currentState = progressTaskState.get(progressId);
        if (currentState) {
            currentState.cancelling = false;
        }
    }
}

// 将进度消息转换为可折叠的详情组件
function convertProgressToDetails(progressId, assistantMessageId) {
    const progressElement = document.getElementById(progressId);
    if (!progressElement) return;
    
    // 获取时间线内容
    const timeline = document.getElementById(progressId + '-timeline');
    // 即使时间线不存在，也创建详情组件（显示空状态）
    let timelineHTML = '';
    if (timeline) {
        timelineHTML = timeline.innerHTML;
    }
    
    // 获取助手消息元素
    const assistantElement = document.getElementById(assistantMessageId);
    if (!assistantElement) {
        removeMessage(progressId);
        return;
    }
    
    // 创建详情组件
    const detailsId = 'details-' + Date.now() + '-' + messageCounter++;
    const detailsDiv = document.createElement('div');
    detailsDiv.id = detailsId;
    detailsDiv.className = 'message system progress-details';
    
    const contentWrapper = document.createElement('div');
    contentWrapper.className = 'message-content';
    
    const bubble = document.createElement('div');
    bubble.className = 'message-bubble progress-container completed';
    
    // 获取时间线HTML内容
    const hasContent = timelineHTML.trim().length > 0;
    
    // 检查时间线中是否有错误项
    const hasError = timeline && timeline.querySelector('.timeline-item-error');
    
    // 如果有错误，默认折叠；否则默认展开
    const shouldExpand = !hasError;
    const expandedClass = shouldExpand ? 'expanded' : '';
    const collapseDetailText = typeof window.t === 'function' ? window.t('tasks.collapseDetail') : '收起详情';
    const expandDetailText = typeof window.t === 'function' ? window.t('chat.expandDetail') : '展开详情';
    const toggleText = shouldExpand ? collapseDetailText : expandDetailText;
    const penetrationDetailText = typeof window.t === 'function' ? window.t('chat.penetrationTestDetail') : '渗透测试详情';
    const noProcessDetailText = typeof window.t === 'function' ? window.t('chat.noProcessDetail') : '暂无过程详情（可能执行过快或未触发详细事件）';
    bubble.innerHTML = `
        <div class="progress-header">
            <span class="progress-title">📋 ${penetrationDetailText}</span>
            ${hasContent ? `<button class="progress-toggle" onclick="toggleProgressDetails('${detailsId}')">${toggleText}</button>` : ''}
        </div>
        ${hasContent ? `<div class="progress-timeline ${expandedClass}" id="${detailsId}-timeline">${timelineHTML}</div><div class="progress-footer"><button type="button" class="progress-toggle progress-toggle-bottom" onclick="toggleProgressDetails('${detailsId}')">${toggleText}</button></div>` : '<div class="progress-timeline-empty">' + noProcessDetailText + '</div>'}
    `;
    
    contentWrapper.appendChild(bubble);
    detailsDiv.appendChild(contentWrapper);
    
    // 将详情组件插入到助手消息之后
    const messagesDiv = document.getElementById('chat-messages');
    const insertWasPinned = isChatMessagesPinnedToBottom();
    // assistantElement 是消息div，需要插入到它的下一个兄弟节点之前
    if (assistantElement.nextSibling) {
        messagesDiv.insertBefore(detailsDiv, assistantElement.nextSibling);
    } else {
        // 如果没有下一个兄弟节点，直接追加
        messagesDiv.appendChild(detailsDiv);
    }
    
    // 移除原来的进度消息
    removeMessage(progressId);
    
    scrollChatMessagesToBottomIfPinned(insertWasPinned);
}

/** 将后端消息 UUID 绑定到助手气泡，供删除本轮 / 过程详情懒加载（domId 为前端 msg-*） */
function applyBackendMessageIdToAssistantDom(domAssistantId, backendMessageId) {
    if (!domAssistantId || !backendMessageId) return;
    const el = document.getElementById(domAssistantId);
    if (!el) return;
    el.dataset.backendMessageId = String(backendMessageId);
    if (typeof attachDeleteTurnButton === 'function') {
        attachDeleteTurnButton(el);
    }
}

/** 将后端用户消息 ID 绑定到最后一条尚未绑定 backendMessageId 的用户气泡 */
function applyBackendMessageIdToLastUser(backendMessageId) {
    if (!backendMessageId) return;
    const users = document.querySelectorAll('#chat-messages .message.user');
    if (!users.length) return;
    const lastUser = users[users.length - 1];
    if (lastUser.dataset.backendMessageId) return;
    lastUser.dataset.backendMessageId = String(backendMessageId);
    if (typeof attachDeleteTurnButton === 'function') {
        attachDeleteTurnButton(lastUser);
    }
}

// 处理流式事件
function handleStreamEvent(event, progressElement, progressId, 
                          getAssistantId, setAssistantId, getMcpIds, setMcpIds) {
    const streamScrollWasPinned = isChatMessagesPinnedToBottom();

    // 不依赖进度时间线；在首条 SSE 即可绑定用户消息 ID
    if (event.type === 'message_saved') {
        const d = event.data || {};
        if (d.userMessageId) {
            applyBackendMessageIdToLastUser(d.userMessageId);
        }
        scrollChatMessagesToBottomIfPinned(streamScrollWasPinned);
        return;
    }

    const timeline = document.getElementById(progressId + '-timeline');
    if (!timeline) return;

    // 终态事件（error/cancelled）优先复用现有助手消息，避免重复追加相同报错
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
            // SSE 长连接保活，无需更新 UI
            break;
        case 'conversation':
            if (event.data && event.data.conversationId) {
                // 在更新之前，先获取任务对应的原始对话ID
                const taskState = progressTaskState.get(progressId);
                const originalConversationId = taskState?.conversationId;
                
                // 更新任务状态
                updateProgressConversation(progressId, event.data.conversationId);
                
                // 如果用户已经开始了新对话（currentConversationId 为 null），
                // 且这个 conversation 事件来自旧对话，就不更新 currentConversationId
                if (currentConversationId === null && originalConversationId !== null) {
                    // 用户已经开始了新对话，忽略旧对话的 conversation 事件
                    // 但仍然更新任务状态，以便正确显示任务信息
                    break;
                }
                
                // 更新当前对话ID
                currentConversationId = event.data.conversationId;
                updateActiveConversation();
                addAttackChainButton(currentConversationId);
                loadActiveTasks();
                // 延迟刷新对话列表，确保用户消息已保存，updated_at已更新
                // 这样新对话才能正确显示在最近对话列表的顶部
                // 使用loadConversationsWithGroups确保分组映射缓存正确加载，无论是否有分组都能立即显示
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
                    : ('主代理 · 第 ' + n + ' 轮');
            } else if (d.einoScope === 'sub') {
                const ag = d.einoAgent != null ? String(d.einoAgent).trim() : '';
                iterTitle = typeof window.t === 'function'
                    ? window.t('chat.einoSubAgentStep', { n: n, agent: ag })
                    : ('子代理 · ' + ag + ' · 第 ' + n + ' 步');
            } else {
                iterTitle = typeof window.t === 'function'
                    ? window.t('chat.iterationRound', { n: n })
                    : ('第 ' + n + ' 轮迭代');
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
            // 若已存在，重置 buffer
            const thinkBase = typeof window.t === 'function' ? window.t('chat.aiThinking') : 'AI思考';
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
            // 如果本 thinking 是由 thinking_stream_* 聚合出来的（带 streamId），避免重复创建 timeline item
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
                            // contentEl.innerHTML 用于兼容 Markdown 展示
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
                title: timelineAgentBracketPrefix(event.data) + '🤔 ' + (typeof window.t === 'function' ? window.t('chat.aiThinking') : 'AI思考'),
                message: event.message,
                data: event.data
            });
            break;
            
        case 'tool_calls_detected':
            addTimelineItem(timeline, 'tool_calls_detected', {
                title: timelineAgentBracketPrefix(event.data) + '🔧 ' + (typeof window.t === 'function' ? window.t('chat.toolCallsDetected', { count: event.data?.count || 0 }) : '检测到 ' + (event.data?.count || 0) + ' 个工具调用'),
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

        case 'eino_recovery': {
            const d = event.data || {};
            const runIdx = d.runIndex != null ? d.runIndex : (d.einoRetry != null ? d.einoRetry + 1 : 1);
            const maxRuns = d.maxRuns != null ? d.maxRuns : 3;
            const title = typeof window.t === 'function'
                ? window.t('chat.einoRecoveryTitle', { n: runIdx, max: maxRuns })
                : ('🔄 工具参数无效 · 第 ' + runIdx + '/' + maxRuns + ' 轮（已追加提示）');
            addTimelineItem(timeline, 'eino_recovery', {
                title: title,
                message: event.message || '',
                data: event.data
            });
            // If the backend triggers a recovery run, any "running" tool_call items in this progress
            // should be closed to avoid being stuck forever.
            finalizeOutstandingToolCallsForProgress(progressId, 'failed');
            break;
        }

        case 'tool_call':
            const toolInfo = event.data || {};
            const toolName = toolInfo.toolName || (typeof window.t === 'function' ? window.t('chat.unknownTool') : '未知工具');
            const index = toolInfo.index || 0;
            const total = toolInfo.total || 0;
            const toolCallId = toolInfo.toolCallId || null;
            const toolCallTitle = typeof window.t === 'function' ? window.t('chat.callTool', { name: escapeHtml(toolName), index: index, total: total }) : '调用工具: ' + escapeHtml(toolName) + ' (' + index + '/' + total + ')';
            const toolCallItemId = addTimelineItem(timeline, 'tool_call', {
                title: timelineAgentBracketPrefix(toolInfo) + '🔧 ' + toolCallTitle,
                message: event.message,
                data: toolInfo,
                expanded: false
            });
            
            // 如果有toolCallId，存储映射关系以便后续更新状态
            if (toolCallId && toolCallItemId) {
                toolCallStatusMap.set(toolCallId, {
                    itemId: toolCallItemId,
                    timeline: timeline,
                    progressId: progressId
                });
                
                // 添加执行中状态指示器
                updateToolCallStatus(toolCallId, 'running');
            }
            break;

        case 'tool_result_delta': {
            const deltaInfo = event.data || {};
            const toolCallId = deltaInfo.toolCallId || null;
            if (!toolCallId) break;

            const key = toolResultStreamKey(progressId, toolCallId);
            let state = toolResultStreamStateByKey.get(key);
            const toolNameDelta = deltaInfo.toolName || (typeof window.t === 'function' ? window.t('chat.unknownTool') : '未知工具');
            const deltaText = event.message || '';
            if (!deltaText) break;

            if (!state) {
                // 首次增量：创建一个 tool_result 占位条目，后续不断更新 pre 内容
                const runningLabel = typeof window.t === 'function' ? window.t('timeline.running') : '执行中...';
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
            const resultToolName = resultInfo.toolName || (typeof window.t === 'function' ? window.t('chat.unknownTool') : '未知工具');
            const success = resultInfo.success !== false;
            const statusIcon = success ? '✅' : '❌';
            const resultToolCallId = resultInfo.toolCallId || null;
            const resultExecText = success ? (typeof window.t === 'function' ? window.t('chat.toolExecComplete', { name: escapeHtml(resultToolName) }) : '工具 ' + escapeHtml(resultToolName) + ' 执行完成') : (typeof window.t === 'function' ? window.t('chat.toolExecFailed', { name: escapeHtml(resultToolName) }) : '工具 ' + escapeHtml(resultToolName) + ' 执行失败');

            // 若此 tool 已经流式推送过增量，则复用占位条目并更新最终结果，避免重复添加一条
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

                    // 同时更新 tool_call 的状态
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
            const streamingLabel = typeof window.t === 'function' ? window.t('timeline.running') : '执行中...';
            const replyTitleBase = typeof window.t === 'function' ? window.t('chat.einoAgentReplyTitle') : '子代理回复';
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
                        const replyTitleBase = typeof window.t === 'function' ? window.t('chat.einoAgentReplyTitle') : '子代理回复';
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
            const replyTitleBase = typeof window.t === 'function' ? window.t('chat.einoAgentReplyTitle') : '子代理回复';
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
                // 保存原文，语言切换时可用 translateProgressMessage 重新套当前语言
                const progressEl = document.getElementById(progressId);
                if (progressEl) {
                    progressEl.dataset.progressRawMessage = event.message || '';
                }
                const progressMsg = translateProgressMessage(event.message);
                progressTitle.textContent = '🔍 ' + progressMsg;
            }
            break;
        
        case 'cancelled':
            const taskCancelledText = typeof window.t === 'function' ? window.t('chat.taskCancelled') : '任务已取消';
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
                finalizeProgressTask(progressId, typeof window.t === 'function' ? window.t('tasks.statusCancelled') : '已取消');
            }
            
            // 复用已有助手消息（若有），避免终态事件重复插入消息
            {
                const preferredMessageId = event.data && event.data.messageId ? event.data.messageId : null;
                const { assistantId, assistantElement } = upsertTerminalAssistantMessage(event.message, preferredMessageId);
                if (assistantId && preferredMessageId) {
                    applyBackendMessageIdToAssistantDom(assistantId, preferredMessageId);
                }
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
            
            // 立即刷新任务状态
            loadActiveTasks();
            // Close any remaining running tool calls for this progress.
            finalizeOutstandingToolCallsForProgress(progressId, 'failed');
            break;
            
        case 'response_start': {
            const responseTaskState = progressTaskState.get(progressId);
            const responseOriginalConversationId = responseTaskState?.conversationId;

            const responseData = event.data || {};
            const mcpIds = responseData.mcpExecutionIds || [];
            setMcpIds(mcpIds);

            if (responseData.conversationId) {
                // 如果用户已经开始了新对话（currentConversationId 为 null），且这个事件来自旧对话，则忽略
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

            // 多代理模式下，迭代过程中的输出只显示在时间线中，不创建助手消息气泡
            // 创建时间线条目用于显示迭代过程中的输出
            const agentPrefix = timelineAgentBracketPrefix(responseData);
            const title = agentPrefix + '📝 ' + (typeof window.t === 'function' ? window.t('chat.planning') : '规划中');
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

            // 多代理模式下，迭代过程中的输出只显示在时间线中
            // 更新时间线条目内容
            let state = responseStreamStateByProgressId.get(progressId);
            if (!state) {
                state = { itemId: null, buffer: '' };
                responseStreamStateByProgressId.set(progressId, state);
            }

            const deltaContent = event.message || '';
            state.buffer += deltaContent;

            // 更新时间线条目内容
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
            // 在更新之前，先获取任务对应的原始对话ID
            const responseTaskState = progressTaskState.get(progressId);
            const responseOriginalConversationId = responseTaskState?.conversationId;

            // 先更新 mcp ids
            const responseData = event.data || {};
            const mcpIds = responseData.mcpExecutionIds || [];
            setMcpIds(mcpIds);

            // 更新对话ID
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

            // 如果之前已经在 response_start/response_delta 阶段创建过占位，则复用该消息更新最终内容
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

            // 移除 response_start/response_delta 阶段创建的「规划中」占位条目。
            // 该条目属于 UI-only 的流式展示，不应被拷贝到最终的过程详情里；
            // 否则会出现“不刷新页面仍显示规划中，刷新后消失”的不一致。
            if (streamState && streamState.itemId) {
                const planningItem = document.getElementById(streamState.itemId);
                if (planningItem && planningItem.parentNode) {
                    planningItem.parentNode.removeChild(planningItem);
                }
            }

            // 最终回复时隐藏进度卡片（多代理模式下，迭代过程已完整展示）
            hideProgressMessageForFinalReply(progressId);

            // Before integrating/removing the progress DOM, close any outstanding running tool calls
            // so the copied timeline HTML reflects the final status.
            finalizeOutstandingToolCallsForProgress(progressId, 'failed');

            // 将进度详情集成到工具调用区域（放在最终 response 之后，保证时间线已完整）
            integrateProgressToMCPSection(progressId, assistantIdFinal, mcpIds);
            responseStreamStateByProgressId.delete(progressId);

            const respMid = responseData.messageId;
            if (respMid) {
                applyBackendMessageIdToAssistantDom(assistantIdFinal, respMid);
            }

            setTimeout(() => {
                collapseAllProgressDetails(assistantIdFinal, progressId);
            }, 3000);

            setTimeout(() => {
                loadConversations();
            }, 200);
            break;
            
        case 'error':
            // 显示错误
            addTimelineItem(timeline, 'error', {
                title: '❌ ' + (typeof window.t === 'function' ? window.t('chat.error') : '错误'),
                message: event.message,
                data: event.data
            });
            
            // 更新进度标题为错误状态
            const errorTitle = document.querySelector(`#${progressId} .progress-title`);
            if (errorTitle) {
                errorTitle.textContent = '❌ ' + (typeof window.t === 'function' ? window.t('chat.executionFailed') : '执行失败');
            }
            
            // 更新进度容器为已完成状态（添加completed类）
            const progressContainer = document.querySelector(`#${progressId} .progress-container`);
            if (progressContainer) {
                progressContainer.classList.add('completed');
            }
            
            // 完成进度任务（标记为失败）
            if (progressTaskState.has(progressId)) {
                finalizeProgressTask(progressId, typeof window.t === 'function' ? window.t('tasks.statusFailed') : '执行失败');
            }
            
            // 复用已有助手消息（若有），避免终态事件重复插入消息
            {
                const preferredMessageId = event.data && event.data.messageId ? event.data.messageId : null;
                const { assistantId, assistantElement } = upsertTerminalAssistantMessage(event.message, preferredMessageId);
                if (assistantId && preferredMessageId) {
                    applyBackendMessageIdToAssistantDom(assistantId, preferredMessageId);
                }
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
            
            // 立即刷新任务状态（执行失败时任务状态会更新）
            loadActiveTasks();
            // Close any remaining running tool calls for this progress.
            finalizeOutstandingToolCallsForProgress(progressId, 'failed');
            break;
            
        case 'done':
            // 清理流式输出状态
            responseStreamStateByProgressId.delete(progressId);
            thinkingStreamStateByProgressId.delete(progressId);
            einoAgentReplyStreamStateByProgressId.delete(progressId);
            // 清理工具流式输出占位
            const prefix = String(progressId) + '::';
            for (const key of Array.from(toolResultStreamStateByKey.keys())) {
                if (String(key).startsWith(prefix)) {
                    toolResultStreamStateByKey.delete(key);
                }
            }
            // 完成，更新进度标题（如果进度消息还存在）
            const doneTitle = document.querySelector(`#${progressId} .progress-title`);
            if (doneTitle) {
                doneTitle.textContent = '✅ ' + (typeof window.t === 'function' ? window.t('chat.penetrationTestComplete') : '渗透测试完成');
            }
            // 更新对话ID
            if (event.data && event.data.conversationId) {
                currentConversationId = event.data.conversationId;
                updateActiveConversation();
                addAttackChainButton(currentConversationId);
                updateProgressConversation(progressId, event.data.conversationId);
            }
            if (progressTaskState.has(progressId)) {
                finalizeProgressTask(progressId, typeof window.t === 'function' ? window.t('tasks.statusCompleted') : '已完成');
            }
            
            // 检查时间线中是否有错误项
            const hasError = timeline && timeline.querySelector('.timeline-item-error');
            
            // 立即刷新任务状态（确保任务状态同步）
            loadActiveTasks();
            // Close any remaining running tool calls for this progress (best-effort).
            finalizeOutstandingToolCallsForProgress(progressId, 'failed');
            
            // 延迟再次刷新任务状态（确保后端已完成状态更新）
            setTimeout(() => {
                loadActiveTasks();
            }, 200);
            
            // 完成时自动折叠所有详情（延迟一下确保response事件已处理）
            setTimeout(() => {
                const assistantIdFromDone = getAssistantId();
                if (assistantIdFromDone) {
                    collapseAllProgressDetails(assistantIdFromDone, progressId);
                } else {
                    // 如果无法获取助手ID，尝试折叠所有详情
                    collapseAllProgressDetails(null, progressId);
                }
                
                // 如果有错误，确保详情是折叠的（错误时应该默认折叠）
                if (hasError) {
                    // 再次确保折叠（延迟一点确保DOM已更新）
                    setTimeout(() => {
                        collapseAllProgressDetails(assistantIdFromDone || null, progressId);
                    }, 200);
                }
            }, 500);
            break;
    }
    
    // 仅在事件处理前用户已在底部附近时跟随滚到底部（避免上滑看历史时被拉回）
    scrollChatMessagesToBottomIfPinned(streamScrollWasPinned);
}

// 更新工具调用状态
function updateToolCallStatus(toolCallId, status) {
    const mapping = toolCallStatusMap.get(toolCallId);
    if (!mapping) return;
    
    const item = document.getElementById(mapping.itemId);
    if (!item) return;
    
    const titleElement = item.querySelector('.timeline-item-title');
    if (!titleElement) return;
    
    // 移除之前的状态类
    item.classList.remove('tool-call-running', 'tool-call-completed', 'tool-call-failed');
    
    const runningLabel = typeof window.t === 'function' ? window.t('timeline.running') : '执行中...';
    const completedLabel = typeof window.t === 'function' ? window.t('timeline.completed') : '已完成';
    const failedLabel = typeof window.t === 'function' ? window.t('timeline.execFailed') : '执行失败';
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
    
    // 更新标题（保留原有文本，追加状态）
    const originalText = titleElement.innerHTML;
    // 移除之前可能存在的状态标记
    const cleanText = originalText.replace(/\s*<span class="tool-status-badge[^>]*>.*?<\/span>/g, '');
    titleElement.innerHTML = cleanText + statusText;
}

// 添加时间线项目
function addTimelineItem(timeline, type, options) {
    const item = document.createElement('div');
    // 生成唯一ID
    const itemId = 'timeline-item-' + Date.now() + '-' + Math.random().toString(36).substr(2, 9);
    item.id = itemId;
    item.className = `timeline-item timeline-item-${type}`;
    // 记录类型与参数，便于 languagechange 时刷新标题文案
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
    if (type === 'eino_recovery' && options.data) {
        const d = options.data;
        if (d.runIndex != null) {
            item.dataset.recoveryRunIndex = String(d.runIndex);
        }
        if (d.maxRuns != null) {
            item.dataset.recoveryMaxRuns = String(d.maxRuns);
        }
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

    // 使用传入的createdAt时间，如果没有则使用当前时间（向后兼容）
    let eventTime;
    if (options.createdAt) {
        // 处理字符串或Date对象
        if (typeof options.createdAt === 'string') {
            eventTime = new Date(options.createdAt);
        } else if (options.createdAt instanceof Date) {
            eventTime = options.createdAt;
        } else {
            eventTime = new Date(options.createdAt);
        }
        // 如果解析失败，使用当前时间
        if (isNaN(eventTime.getTime())) {
            eventTime = new Date();
        }
    } else {
        eventTime = new Date();
    }
    // 保存事件时间 ISO，语言切换时可重算时间格式
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
    
    // 根据类型添加详细内容
    if ((type === 'thinking' || type === 'planning') && options.message) {
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
        const paramsLabel = typeof window.t === 'function' ? window.t('timeline.params') : '参数:';
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
        const noResultText = typeof window.t === 'function' ? window.t('timeline.noResult') : '无结果';
        const result = data.result || data.error || noResultText;
        const resultStr = typeof result === 'string' ? result : JSON.stringify(result);
        const execResultLabel = typeof window.t === 'function' ? window.t('timeline.executionResult') : '执行结果:';
        const execIdLabel = typeof window.t === 'function' ? window.t('timeline.executionId') : '执行ID:';
        content += `
            <div class="timeline-item-content">
                <div class="tool-result-section ${isError ? 'error' : 'success'}">
                    <strong data-i18n="timeline.executionResult">${escapeHtml(execResultLabel)}</strong>
                    <pre class="tool-result">${escapeHtml(resultStr)}</pre>
                    ${data.executionId ? `<div class="tool-execution-id"><span data-i18n="timeline.executionId">${escapeHtml(execIdLabel)}</span> <code>${escapeHtml(data.executionId)}</code></div>` : ''}
                </div>
            </div>
        `;
    } else if (type === 'eino_recovery' && options.message) {
        content += `
            <div class="timeline-item-content timeline-eino-recovery">
                ${escapeHtml(options.message).replace(/\n/g, '<br>')}
            </div>
        `;
    } else if (type === 'cancelled') {
        const taskCancelledLabel = typeof window.t === 'function' ? window.t('chat.taskCancelled') : '任务已取消';
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
    
    // 自动展开详情
    const expanded = timeline.classList.contains('expanded');
    if (!expanded && (type === 'tool_call' || type === 'tool_result')) {
        // 对于工具调用和结果，默认显示摘要
    }
    
    // 返回item ID以便后续更新
    return itemId;
}

// 加载活跃任务列表
async function loadActiveTasks(showErrors = false) {
    const bar = document.getElementById('active-tasks-bar');
    try {
        const response = await apiFetch('/api/agent-loop/tasks');
        const result = await response.json().catch(() => ({}));

        if (!response.ok) {
            throw new Error(result.error || (typeof window.t === 'function' ? window.t('tasks.loadActiveTasksFailed') : '获取活跃任务失败'));
        }

        renderActiveTasks(result.tasks || []);
    } catch (error) {
        console.error('获取活跃任务失败:', error);
        if (showErrors && bar) {
            bar.style.display = 'block';
            const cannotGetStatus = typeof window.t === 'function' ? window.t('tasks.cannotGetTaskStatus') : '无法获取任务状态：';
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

        // 只有非最终状态的任务才显示停止按钮
        if (!isFinalStatus) {
            const cancelBtn = item.querySelector('.active-task-cancel');
            if (cancelBtn) {
                cancelBtn.onclick = () => cancelActiveTask(task.conversationId, cancelBtn);
                if (task.status === 'cancelling') {
                    cancelBtn.disabled = true;
                    cancelBtn.textContent = typeof window.t === 'function' ? window.t('tasks.cancelling') : '取消中...';
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
    button.textContent = typeof window.t === 'function' ? window.t('tasks.cancelling') : '取消中...';

    try {
        await requestCancel(conversationId);
        loadActiveTasks();
    } catch (error) {
        console.error('取消任务失败:', error);
        alert((typeof window.t === 'function' ? window.t('tasks.cancelTaskFailed') : '取消任务失败') + ': ' + error.message);
        button.disabled = false;
        button.textContent = originalText;
    }
}

// 监控面板状态
const monitorState = {
    executions: [],
    stats: {},
    lastFetchedAt: null,
    pagination: {
        page: 1,
        pageSize: (() => {
            // 从 localStorage 读取保存的每页显示数量，默认为 20
            const saved = localStorage.getItem('monitorPageSize');
            return saved ? parseInt(saved, 10) : 20;
        })(),
        total: 0,
        totalPages: 0
    }
};

function openMonitorPanel() {
    // 切换到MCP监控页面
    if (typeof switchPage === 'function') {
        switchPage('mcp-monitor');
    }
    // 初始化每页显示数量选择器
    initializeMonitorPageSize();
}

// 初始化每页显示数量选择器
function initializeMonitorPageSize() {
    const pageSizeSelect = document.getElementById('monitor-page-size');
    if (pageSizeSelect) {
        pageSizeSelect.value = monitorState.pagination.pageSize;
    }
}

// 改变每页显示数量
function changeMonitorPageSize() {
    const pageSizeSelect = document.getElementById('monitor-page-size');
    if (!pageSizeSelect) {
        return;
    }
    
    const newPageSize = parseInt(pageSizeSelect.value, 10);
    if (isNaN(newPageSize) || newPageSize <= 0) {
        return;
    }
    
    // 保存到 localStorage
    localStorage.setItem('monitorPageSize', newPageSize.toString());
    
    // 更新状态
    monitorState.pagination.pageSize = newPageSize;
    monitorState.pagination.page = 1; // 重置到第一页
    
    // 刷新数据
    refreshMonitorPanel(1);
}

function closeMonitorPanel() {
    // 不再需要关闭功能，因为现在是页面而不是模态框
    // 如果需要，可以切换回对话页面
    if (typeof switchPage === 'function') {
        switchPage('chat');
    }
}

async function refreshMonitorPanel(page = null) {
    const statsContainer = document.getElementById('monitor-stats');
    const execContainer = document.getElementById('monitor-executions');

    try {
        // 如果指定了页码，使用指定页码，否则使用当前页码
        const currentPage = page !== null ? page : monitorState.pagination.page;
        const pageSize = monitorState.pagination.pageSize;
        
        // 获取当前的筛选条件
        const statusFilter = document.getElementById('monitor-status-filter');
        const toolFilter = document.getElementById('monitor-tool-filter');
        const currentStatusFilter = statusFilter ? statusFilter.value : 'all';
        const currentToolFilter = toolFilter ? (toolFilter.value.trim() || 'all') : 'all';
        
        // 构建请求 URL
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
            throw new Error(result.error || '获取监控数据失败');
        }

        monitorState.executions = Array.isArray(result.executions) ? result.executions : [];
        monitorState.stats = result.stats || {};
        monitorState.lastFetchedAt = new Date();
        
        // 更新分页信息
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
        
        // 初始化每页显示数量选择器
        initializeMonitorPageSize();
    } catch (error) {
        console.error('刷新监控面板失败:', error);
        if (statsContainer) {
            statsContainer.innerHTML = `<div class="monitor-error">${escapeHtml(typeof window.t === 'function' ? window.t('mcpMonitor.loadStatsError') : '无法加载统计信息')}：${escapeHtml(error.message)}</div>`;
        }
        if (execContainer) {
            execContainer.innerHTML = `<div class="monitor-error">${escapeHtml(typeof window.t === 'function' ? window.t('mcpMonitor.loadExecutionsError') : '无法加载执行记录')}：${escapeHtml(error.message)}</div>`;
        }
    }
}

// 处理工具搜索输入（防抖）
let toolFilterDebounceTimer = null;
function handleToolFilterInput() {
    // 清除之前的定时器
    if (toolFilterDebounceTimer) {
        clearTimeout(toolFilterDebounceTimer);
    }
    
    // 设置新的定时器，500ms后执行筛选
    toolFilterDebounceTimer = setTimeout(() => {
        applyMonitorFilters();
    }, 500);
}

async function applyMonitorFilters() {
    const statusFilter = document.getElementById('monitor-status-filter');
    const toolFilter = document.getElementById('monitor-tool-filter');
    const status = statusFilter ? statusFilter.value : 'all';
    const tool = toolFilter ? (toolFilter.value.trim() || 'all') : 'all';
    // 当筛选条件改变时，从后端重新获取数据
    await refreshMonitorPanelWithFilter(status, tool);
}

async function refreshMonitorPanelWithFilter(statusFilter = 'all', toolFilter = 'all') {
    const statsContainer = document.getElementById('monitor-stats');
    const execContainer = document.getElementById('monitor-executions');

    try {
        const currentPage = 1; // 筛选时重置到第一页
        const pageSize = monitorState.pagination.pageSize;
        
        // 构建请求 URL
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
            throw new Error(result.error || '获取监控数据失败');
        }

        monitorState.executions = Array.isArray(result.executions) ? result.executions : [];
        monitorState.stats = result.stats || {};
        monitorState.lastFetchedAt = new Date();
        
        // 更新分页信息
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
        
        // 初始化每页显示数量选择器
        initializeMonitorPageSize();
    } catch (error) {
        console.error('刷新监控面板失败:', error);
        if (statsContainer) {
            statsContainer.innerHTML = `<div class="monitor-error">${escapeHtml(typeof window.t === 'function' ? window.t('mcpMonitor.loadStatsError') : '无法加载统计信息')}：${escapeHtml(error.message)}</div>`;
        }
        if (execContainer) {
            execContainer.innerHTML = `<div class="monitor-error">${escapeHtml(typeof window.t === 'function' ? window.t('mcpMonitor.loadExecutionsError') : '无法加载执行记录')}：${escapeHtml(error.message)}</div>`;
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
        const noStats = typeof window.t === 'function' ? window.t('mcpMonitor.noStatsData') : '暂无统计数据';
        container.innerHTML = '<div class="monitor-empty">' + escapeHtml(noStats) + '</div>';
        return;
    }

    // 计算总体汇总
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
    const noCallsYet = typeof window.t === 'function' ? window.t('mcpMonitor.noCallsYet') : '暂无调用';
    const lastCallText = totals.lastCallTime ? (totals.lastCallTime.toLocaleString ? totals.lastCallTime.toLocaleString(locale || 'en-US') : String(totals.lastCallTime)) : noCallsYet;
    const totalCallsLabel = typeof window.t === 'function' ? window.t('mcpMonitor.totalCalls') : '总调用次数';
    const successFailedLabel = typeof window.t === 'function' ? window.t('mcpMonitor.successFailed', { success: totals.success, failed: totals.failed }) : `成功 ${totals.success} / 失败 ${totals.failed}`;
    const successRateLabel = typeof window.t === 'function' ? window.t('mcpMonitor.successRate') : '成功率';
    const statsFromAll = typeof window.t === 'function' ? window.t('mcpMonitor.statsFromAllTools') : '统计自全部工具调用';
    const lastCallLabel = typeof window.t === 'function' ? window.t('mcpMonitor.lastCall') : '最近一次调用';
    const lastRefreshLabel = typeof window.t === 'function' ? window.t('mcpMonitor.lastRefreshTime') : '最后刷新时间';

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
            <div class="monitor-stat-meta">${escapeHtml(lastRefreshLabel)}：${escapeHtml(lastUpdatedText)}</div>
        </div>
    `;

    // 显示最多前4个工具的统计（过滤掉 totalCalls 为 0 的工具）
    const topTools = entries
        .filter(tool => (tool.totalCalls || 0) > 0)
        .slice()
        .sort((a, b) => (b.totalCalls || 0) - (a.totalCalls || 0))
        .slice(0, 4);

    const unknownToolLabel = typeof window.t === 'function' ? window.t('mcpMonitor.unknownTool') : '未知工具';
    topTools.forEach(tool => {
        const toolSuccessRate = tool.totalCalls > 0 ? ((tool.successCalls || 0) / tool.totalCalls * 100).toFixed(1) : '0.0';
        const toolMeta = typeof window.t === 'function' ? window.t('mcpMonitor.successFailedRate', { success: tool.successCalls || 0, failed: tool.failedCalls || 0, rate: toolSuccessRate }) : `成功 ${tool.successCalls || 0} / 失败 ${tool.failedCalls || 0} · 成功率 ${toolSuccessRate}%`;
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
        // 根据是否有筛选条件显示不同的提示
        const toolFilter = document.getElementById('monitor-tool-filter');
        const currentToolFilter = toolFilter ? toolFilter.value : 'all';
        const hasFilter = (statusFilter && statusFilter !== 'all') || (currentToolFilter && currentToolFilter !== 'all');
        const noRecordsFilter = typeof window.t === 'function' ? window.t('mcpMonitor.noRecordsWithFilter') : '当前筛选条件下暂无记录';
        const noExecutions = typeof window.t === 'function' ? window.t('mcpMonitor.noExecutions') : '暂无执行记录';
        if (hasFilter) {
            container.innerHTML = '<div class="monitor-empty">' + escapeHtml(noRecordsFilter) + '</div>';
        } else {
            container.innerHTML = '<div class="monitor-empty">' + escapeHtml(noExecutions) + '</div>';
        }
        // 隐藏批量操作栏
        const batchActions = document.getElementById('monitor-batch-actions');
        if (batchActions) {
            batchActions.style.display = 'none';
        }
        return;
    }

    // 由于筛选已经在后端完成，这里直接使用所有传入的执行记录
    // 不再需要前端再次筛选，因为后端已经返回了筛选后的数据
    const unknownLabel = typeof window.t === 'function' ? window.t('mcpMonitor.unknown') : '未知';
    const unknownToolLabel = typeof window.t === 'function' ? window.t('mcpMonitor.unknownTool') : '未知工具';
    const viewDetailLabel = typeof window.t === 'function' ? window.t('mcpMonitor.viewDetail') : '查看详情';
    const deleteLabel = typeof window.t === 'function' ? window.t('mcpMonitor.delete') : '删除';
    const deleteExecTitle = typeof window.t === 'function' ? window.t('mcpMonitor.deleteExecTitle') : '删除此执行记录';
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

    // 先移除旧的表格容器和加载提示（保留分页控件）
    const oldTableContainer = container.querySelector('.monitor-table-container');
    if (oldTableContainer) {
        oldTableContainer.remove();
    }
    // 清除"加载中..."等提示信息
    const oldEmpty = container.querySelector('.monitor-empty');
    if (oldEmpty) {
        oldEmpty.remove();
    }
    
    // 创建表格容器
    const tableContainer = document.createElement('div');
    tableContainer.className = 'monitor-table-container';
    const colTool = typeof window.t === 'function' ? window.t('mcpMonitor.columnTool') : '工具';
    const colStatus = typeof window.t === 'function' ? window.t('mcpMonitor.columnStatus') : '状态';
    const colStartTime = typeof window.t === 'function' ? window.t('mcpMonitor.columnStartTime') : '开始时间';
    const colDuration = typeof window.t === 'function' ? window.t('mcpMonitor.columnDuration') : '耗时';
    const colActions = typeof window.t === 'function' ? window.t('mcpMonitor.columnActions') : '操作';
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
    
    // 在分页控件之前插入表格（如果存在分页控件）
    const existingPagination = container.querySelector('.monitor-pagination');
    if (existingPagination) {
        container.insertBefore(tableContainer, existingPagination);
    } else {
        container.appendChild(tableContainer);
    }
    
    // 更新批量操作状态
    updateBatchActionsState();
}

// 渲染监控面板分页控件
function renderMonitorPagination() {
    const container = document.getElementById('monitor-executions');
    if (!container) return;
    
    // 移除旧的分页控件
    const oldPagination = container.querySelector('.monitor-pagination');
    if (oldPagination) {
        oldPagination.remove();
    }
    
    const { page, totalPages, total, pageSize } = monitorState.pagination;
    
    // 始终显示分页控件
    const pagination = document.createElement('div');
    pagination.className = 'monitor-pagination';
    
    // 处理没有数据的情况
    const startItem = total === 0 ? 0 : (page - 1) * pageSize + 1;
    const endItem = total === 0 ? 0 : Math.min(page * pageSize, total);
    const paginationInfoText = typeof window.t === 'function' ? window.t('mcpMonitor.paginationInfo', { start: startItem, end: endItem, total: total }) : `显示 ${startItem}-${endItem} / 共 ${total} 条记录`;
    const perPageLabel = typeof window.t === 'function' ? window.t('mcpMonitor.perPageLabel') : '每页显示';
    const firstPageLabel = typeof window.t === 'function' ? window.t('mcp.firstPage') : '首页';
    const prevPageLabel = typeof window.t === 'function' ? window.t('mcp.prevPage') : '上一页';
    const pageInfoText = typeof window.t === 'function' ? window.t('mcp.pageInfo', { page: page, total: totalPages || 1 }) : `第 ${page} / ${totalPages || 1} 页`;
    const nextPageLabel = typeof window.t === 'function' ? window.t('mcp.nextPage') : '下一页';
    const lastPageLabel = typeof window.t === 'function' ? window.t('mcp.lastPage') : '末页';
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
    
    // 初始化每页显示数量选择器
    initializeMonitorPageSize();
}

// 删除执行记录
async function deleteExecution(executionId) {
    if (!executionId) {
        return;
    }
    
    const deleteConfirmMsg = typeof window.t === 'function' ? window.t('mcpMonitor.deleteExecConfirmSingle') : '确定要删除此执行记录吗？此操作不可恢复。';
    if (!confirm(deleteConfirmMsg)) {
        return;
    }
    
    try {
        const response = await apiFetch(`/api/monitor/execution/${executionId}`, {
            method: 'DELETE'
        });
        
        if (!response.ok) {
            const error = await response.json().catch(() => ({}));
            const deleteFailedMsg = typeof window.t === 'function' ? window.t('mcpMonitor.deleteExecFailed') : '删除执行记录失败';
            throw new Error(error.error || deleteFailedMsg);
        }
        
        // 删除成功后刷新当前页面
        const currentPage = monitorState.pagination.page;
        await refreshMonitorPanel(currentPage);
        
        const execDeletedMsg = typeof window.t === 'function' ? window.t('mcpMonitor.execDeleted') : '执行记录已删除';
        alert(execDeletedMsg);
    } catch (error) {
        console.error('删除执行记录失败:', error);
        const deleteFailedMsg = typeof window.t === 'function' ? window.t('mcpMonitor.deleteExecFailed') : '删除执行记录失败';
        alert(deleteFailedMsg + ': ' + error.message);
    }
}

// 更新批量操作状态
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
        selectedCountSpan.textContent = typeof window.t === 'function' ? window.t('mcp.selectedCount', { count: selectedCount }) : '已选择 ' + selectedCount + ' 项';
    }
    
    // 更新全选复选框状态
    const selectAllCheckbox = document.getElementById('monitor-select-all');
    if (selectAllCheckbox) {
        const allCheckboxes = document.querySelectorAll('.monitor-execution-checkbox');
        const allChecked = allCheckboxes.length > 0 && Array.from(allCheckboxes).every(cb => cb.checked);
        selectAllCheckbox.checked = allChecked;
        selectAllCheckbox.indeterminate = selectedCount > 0 && selectedCount < allCheckboxes.length;
    }
}

// 切换全选
function toggleSelectAll(checkbox) {
    const checkboxes = document.querySelectorAll('.monitor-execution-checkbox');
    checkboxes.forEach(cb => {
        cb.checked = checkbox.checked;
    });
    updateBatchActionsState();
}

// 全选
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

// 取消全选
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

// 批量删除执行记录
async function batchDeleteExecutions() {
    const checkboxes = document.querySelectorAll('.monitor-execution-checkbox:checked');
    if (checkboxes.length === 0) {
        const selectFirstMsg = typeof window.t === 'function' ? window.t('mcpMonitor.selectExecFirst') : '请先选择要删除的执行记录';
        alert(selectFirstMsg);
        return;
    }
    
    const ids = Array.from(checkboxes).map(cb => cb.value);
    const count = ids.length;
    const batchConfirmMsg = typeof window.t === 'function' ? window.t('mcpMonitor.batchDeleteConfirm', { count: count }) : `确定要删除选中的 ${count} 条执行记录吗？此操作不可恢复。`;
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
            const batchFailedMsg = typeof window.t === 'function' ? window.t('mcp.batchDeleteFailed') : '批量删除执行记录失败';
            throw new Error(error.error || batchFailedMsg);
        }
        
        const result = await response.json().catch(() => ({}));
        const deletedCount = result.deleted || count;
        
        // 删除成功后刷新当前页面
        const currentPage = monitorState.pagination.page;
        await refreshMonitorPanel(currentPage);
        
        const batchSuccessMsg = typeof window.t === 'function' ? window.t('mcpMonitor.batchDeleteSuccess', { count: deletedCount }) : `成功删除 ${deletedCount} 条执行记录`;
        alert(batchSuccessMsg);
    } catch (error) {
        console.error('批量删除执行记录失败:', error);
        const batchFailedMsg = typeof window.t === 'function' ? window.t('mcp.batchDeleteFailed') : '批量删除执行记录失败';
        alert(batchFailedMsg + ': ' + error.message);
    }
}

function formatExecutionDuration(start, end) {
    const unknownLabel = typeof window.t === 'function' ? window.t('mcpMonitor.unknown') : '未知';
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
        return typeof window.t === 'function' ? window.t('mcpMonitor.durationSeconds', { n: seconds }) : seconds + ' 秒';
    }
    const minutes = Math.floor(seconds / 60);
    if (minutes < 60) {
        const remain = seconds % 60;
        if (remain > 0) {
            return typeof window.t === 'function' ? window.t('mcpMonitor.durationMinutes', { minutes: minutes, seconds: remain }) : minutes + ' 分 ' + remain + ' 秒';
        }
        return typeof window.t === 'function' ? window.t('mcpMonitor.durationMinutesOnly', { minutes: minutes }) : minutes + ' 分';
    }
    const hours = Math.floor(minutes / 60);
    const remainMinutes = minutes % 60;
    if (remainMinutes > 0) {
        return typeof window.t === 'function' ? window.t('mcpMonitor.durationHours', { hours: hours, minutes: remainMinutes }) : hours + ' 小时 ' + remainMinutes + ' 分';
    }
    return typeof window.t === 'function' ? window.t('mcpMonitor.durationHoursOnly', { hours: hours }) : hours + ' 小时';
}

/**
 * 语言切换后刷新对话页已渲染的进度条、时间线标题与时间格式（避免仍显示英文或 AM/PM）
 */
function refreshProgressAndTimelineI18n() {
    const _t = function (k, o) {
        return typeof window.t === 'function' ? window.t(k, o) : k;
    };
    const timeLocale = getCurrentTimeLocale();
    const timeOpts = getTimeFormatOptions();

    // 进度块内停止按钮：未禁用时统一为当前语言的「停止任务」（避免仍显示 Stop task）
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
    // 转换后的详情区顶栏「渗透测试详情」：仅刷新不在 .progress-message 内的 progress 标题
    document.querySelectorAll('.progress-container .progress-header .progress-title').forEach(function (titleEl) {
        if (titleEl.closest('.progress-message')) return;
        titleEl.textContent = '\uD83D\uDCCB ' + _t('chat.penetrationTestDetail');
    });

    // 时间线项：按类型重算标题，并重绘时间戳
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
        } else if (type === 'planning') {
            titleSpan.textContent = ap + '\uD83D\uDCDD ' + _t('chat.planning');
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
        } else if (type === 'eino_recovery' && item.dataset.recoveryRunIndex) {
            const n = parseInt(item.dataset.recoveryRunIndex, 10) || 1;
            const mx = parseInt(item.dataset.recoveryMaxRuns, 10) || 3;
            titleSpan.textContent = _t('chat.einoRecoveryTitle', { n: n, max: mx });
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

    // 详情区「展开/收起」按钮
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
