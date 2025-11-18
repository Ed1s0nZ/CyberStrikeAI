const AUTH_STORAGE_KEY = 'cyberstrike-auth';
let authToken = null;
let authTokenExpiry = null;
let authPromise = null;
let authPromiseResolvers = [];
let isAppInitialized = false;

// 当前对话ID
let currentConversationId = null;
// 进度ID与任务信息映射
const progressTaskState = new Map();
// 活跃任务刷新定时器
let activeTaskInterval = null;
const ACTIVE_TASK_REFRESH_INTERVAL = 10000; // 10秒检查一次，提供更实时的任务状态反馈

function isTokenValid() {
    return !!authToken && authTokenExpiry instanceof Date && authTokenExpiry.getTime() > Date.now();
}

function saveAuth(token, expiresAt) {
    const expiry = expiresAt instanceof Date ? expiresAt : new Date(expiresAt);
    authToken = token;
    authTokenExpiry = expiry;
    try {
        localStorage.setItem(AUTH_STORAGE_KEY, JSON.stringify({
            token,
            expiresAt: expiry.toISOString(),
        }));
    } catch (error) {
        console.warn('无法持久化认证信息:', error);
    }
}

function clearAuthStorage() {
    authToken = null;
    authTokenExpiry = null;
    try {
        localStorage.removeItem(AUTH_STORAGE_KEY);
    } catch (error) {
        console.warn('无法清除认证信息:', error);
    }
}

function loadAuthFromStorage() {
    try {
        const raw = localStorage.getItem(AUTH_STORAGE_KEY);
        if (!raw) {
            return false;
        }
        const stored = JSON.parse(raw);
        if (!stored.token || !stored.expiresAt) {
            clearAuthStorage();
            return false;
        }
        const expiry = new Date(stored.expiresAt);
        if (Number.isNaN(expiry.getTime())) {
            clearAuthStorage();
            return false;
        }
        authToken = stored.token;
        authTokenExpiry = expiry;
        return isTokenValid();
    } catch (error) {
        console.error('读取认证信息失败:', error);
        clearAuthStorage();
        return false;
    }
}

function resolveAuthPromises(success) {
    authPromiseResolvers.forEach(resolve => resolve(success));
    authPromiseResolvers = [];
    authPromise = null;
}

function showLoginOverlay(message = '') {
    const overlay = document.getElementById('login-overlay');
    const errorBox = document.getElementById('login-error');
    const passwordInput = document.getElementById('login-password');
    if (!overlay) {
        return;
    }
    overlay.style.display = 'flex';
    if (errorBox) {
        if (message) {
            errorBox.textContent = message;
            errorBox.style.display = 'block';
        } else {
            errorBox.textContent = '';
            errorBox.style.display = 'none';
        }
    }
    setTimeout(() => {
        if (passwordInput) {
            passwordInput.focus();
        }
    }, 100);
}

function hideLoginOverlay() {
    const overlay = document.getElementById('login-overlay');
    const errorBox = document.getElementById('login-error');
    const passwordInput = document.getElementById('login-password');
    if (overlay) {
        overlay.style.display = 'none';
    }
    if (errorBox) {
        errorBox.textContent = '';
        errorBox.style.display = 'none';
    }
    if (passwordInput) {
        passwordInput.value = '';
    }
}

function ensureAuthPromise() {
    if (!authPromise) {
        authPromise = new Promise(resolve => {
            authPromiseResolvers.push(resolve);
        });
    }
    return authPromise;
}

async function ensureAuthenticated() {
    if (isTokenValid()) {
        return true;
    }
    showLoginOverlay();
    await ensureAuthPromise();
    return true;
}

function handleUnauthorized({ message = '认证已过期，请重新登录', silent = false } = {}) {
    clearAuthStorage();
    authPromise = null;
    authPromiseResolvers = [];
    if (!silent) {
        showLoginOverlay(message);
    } else {
        showLoginOverlay();
    }
    return false;
}

async function apiFetch(url, options = {}) {
    await ensureAuthenticated();
    const opts = { ...options };
    const headers = new Headers(options && options.headers ? options.headers : undefined);
    if (authToken && !headers.has('Authorization')) {
        headers.set('Authorization', `Bearer ${authToken}`);
    }
    opts.headers = headers;

    const response = await fetch(url, opts);
    if (response.status === 401) {
        handleUnauthorized();
        throw new Error('未授权访问');
    }
    return response;
}

async function submitLogin(event) {
    event.preventDefault();
    const passwordInput = document.getElementById('login-password');
    const errorBox = document.getElementById('login-error');
    const submitBtn = document.querySelector('.login-submit');

    if (!passwordInput) {
        return;
    }

    const password = passwordInput.value.trim();
    if (!password) {
        if (errorBox) {
            errorBox.textContent = '请输入密码';
            errorBox.style.display = 'block';
        }
        return;
    }

    if (submitBtn) {
        submitBtn.disabled = true;
    }

    try {
        const response = await fetch('/api/auth/login', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ password }),
        });
        const result = await response.json().catch(() => ({}));
        if (!response.ok || !result.token) {
            if (errorBox) {
                errorBox.textContent = result.error || '登录失败，请检查密码';
                errorBox.style.display = 'block';
            }
            return;
        }

        saveAuth(result.token, result.expires_at);
        hideLoginOverlay();
        resolveAuthPromises(true);
        if (!isAppInitialized) {
            await bootstrapApp();
        } else {
            await refreshAppData();
        }
    } catch (error) {
        console.error('登录失败:', error);
        if (errorBox) {
            errorBox.textContent = '登录失败，请稍后重试';
            errorBox.style.display = 'block';
        }
    } finally {
        if (submitBtn) {
            submitBtn.disabled = false;
        }
    }
}

async function refreshAppData(showTaskErrors = false) {
    await Promise.allSettled([
        loadConversations(),
        loadActiveTasks(showTaskErrors),
    ]);
}

async function bootstrapApp() {
    if (!isAppInitialized) {
        initializeChatUI();
        isAppInitialized = true;
    }
    await refreshAppData();
}

function initializeChatUI() {
    const chatInputEl = document.getElementById('chat-input');
    if (chatInputEl) {
        chatInputEl.style.height = '44px';
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
}

function setupLoginUI() {
    const loginForm = document.getElementById('login-form');
    if (loginForm) {
        loginForm.addEventListener('submit', submitLogin);
    }
}

async function initializeApp() {
    setupLoginUI();
    const hasStoredAuth = loadAuthFromStorage();
    if (hasStoredAuth && isTokenValid()) {
        try {
            const response = await apiFetch('/api/auth/validate', {
                method: 'GET',
            });
            if (response.ok) {
                hideLoginOverlay();
                resolveAuthPromises(true);
                await bootstrapApp();
                return;
            }
        } catch (error) {
            console.warn('本地会话已失效，需重新登录');
        }
    }

    clearAuthStorage();
    showLoginOverlay();
}

document.addEventListener('DOMContentLoaded', initializeApp);


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

function finalizeProgressTask(progressId, finalLabel = '已完成') {
    const stopBtn = document.getElementById(`${progressId}-stop-btn`);
    if (stopBtn) {
        stopBtn.disabled = true;
        stopBtn.textContent = finalLabel;
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
        throw new Error(result.error || '取消失败');
    }
    return result;
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
    input.value = '';
    
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
        
    } catch (error) {
        removeMessage(progressId);
        addMessage('system', '错误: ' + error.message);
    }
}

// 创建进度消息容器
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
    bubble.innerHTML = `
        <div class="progress-header">
            <span class="progress-title">🔍 渗透测试进行中...</span>
            <div class="progress-actions">
                <button class="progress-stop" id="${id}-stop-btn" onclick="cancelProgressTask('${id}')">停止任务</button>
                <button class="progress-toggle" onclick="toggleProgressDetails('${id}')">收起详情</button>
            </div>
        </div>
        <div class="progress-timeline expanded" id="${id}-timeline"></div>
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
    const toggleBtn = document.querySelector(`#${progressId} .progress-toggle`);
    
    if (!timeline || !toggleBtn) return;
    
    if (timeline.classList.contains('expanded')) {
        timeline.classList.remove('expanded');
        toggleBtn.textContent = '展开详情';
    } else {
        timeline.classList.add('expanded');
        toggleBtn.textContent = '收起详情';
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
                const btn = document.querySelector(`#${assistantMessageId} .process-detail-btn`);
                if (btn) {
                    btn.innerHTML = '<span>展开详情</span>';
                }
            }
        }
    }
    
    // 折叠独立的详情组件（通过convertProgressToDetails创建的）
    // 查找所有以details-开头的详情组件
    const allDetails = document.querySelectorAll('[id^="details-"]');
    allDetails.forEach(detail => {
        const timeline = detail.querySelector('.progress-timeline');
        const toggleBtn = detail.querySelector('.progress-toggle');
        if (timeline) {
            timeline.classList.remove('expanded');
            if (toggleBtn) {
                toggleBtn.textContent = '展开详情';
            }
        }
    });
    
    // 折叠原始的进度消息（如果还存在）
    if (progressId) {
        const progressTimeline = document.getElementById(progressId + '-timeline');
        const progressToggleBtn = document.querySelector(`#${progressId} .progress-toggle`);
        if (progressTimeline) {
            progressTimeline.classList.remove('expanded');
            if (progressToggleBtn) {
                progressToggleBtn.textContent = '展开详情';
            }
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

// 将进度详情集成到工具调用区域
function integrateProgressToMCPSection(progressId, assistantMessageId) {
    const progressElement = document.getElementById(progressId);
    if (!progressElement) return;
    
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
    
    // 查找MCP调用区域
    const mcpSection = assistantElement.querySelector('.mcp-call-section');
    if (!mcpSection) {
        // 如果没有MCP区域，创建详情组件放在消息下方
        convertProgressToDetails(progressId, assistantMessageId);
        return;
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
            ${hasContent ? `<div class="progress-timeline" id="${detailsId}-timeline">${timelineHTML}</div>` : '<div class="progress-timeline-empty">暂无过程详情</div>'}
        </div>
    `;
    
    // 确保初始状态是折叠的（默认折叠，特别是错误时）
    if (hasContent) {
        const timeline = document.getElementById(detailsId + '-timeline');
        if (timeline) {
            // 如果有错误，确保折叠；否则也默认折叠
            timeline.classList.remove('expanded');
        }
        
        // 更新按钮文本为"展开详情"（因为默认折叠）
        const processDetailBtn = buttonsContainer.querySelector('.process-detail-btn');
        if (processDetailBtn) {
            processDetailBtn.innerHTML = '<span>展开详情</span>';
        }
    }
    
    // 移除原来的进度消息
    removeMessage(progressId);
}

// 切换过程详情显示
function toggleProcessDetails(progressId, assistantMessageId) {
    const detailsId = 'process-details-' + assistantMessageId;
    const detailsContainer = document.getElementById(detailsId);
    if (!detailsContainer) return;
    
    const content = detailsContainer.querySelector('.process-details-content');
    const timeline = detailsContainer.querySelector('.progress-timeline');
    const btn = document.querySelector(`#${assistantMessageId} .process-detail-btn`);
    
    if (content && timeline) {
        if (timeline.classList.contains('expanded')) {
            timeline.classList.remove('expanded');
            if (btn) btn.innerHTML = '<span>展开详情</span>';
        } else {
            timeline.classList.add('expanded');
            if (btn) btn.innerHTML = '<span>收起详情</span>';
        }
    } else if (timeline) {
        // 如果只有timeline，直接切换
        if (timeline.classList.contains('expanded')) {
            timeline.classList.remove('expanded');
            if (btn) btn.innerHTML = '<span>展开详情</span>';
        } else {
            timeline.classList.add('expanded');
            if (btn) btn.innerHTML = '<span>收起详情</span>';
        }
    }
    
    // 滚动到底部以便查看展开的内容
    if (timeline && timeline.classList.contains('expanded')) {
        setTimeout(() => {
            const messagesDiv = document.getElementById('chat-messages');
            messagesDiv.scrollTop = messagesDiv.scrollHeight;
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
        alert('任务信息尚未同步，请稍后再试。');
        return;
    }

    if (state.cancelling) {
        return;
    }

    markProgressCancelling(progressId);
    if (stopBtn) {
        stopBtn.disabled = true;
        stopBtn.textContent = '取消中...';
    }

    try {
        await requestCancel(state.conversationId);
        loadActiveTasks();
    } catch (error) {
        console.error('取消任务失败:', error);
        alert('取消任务失败: ' + error.message);
        if (stopBtn) {
            stopBtn.disabled = false;
            stopBtn.textContent = '停止任务';
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
    const toggleText = shouldExpand ? '收起详情' : '展开详情';
    
    // 总是显示详情组件，即使没有内容也显示
    bubble.innerHTML = `
        <div class="progress-header">
            <span class="progress-title">📋 渗透测试详情</span>
            ${hasContent ? `<button class="progress-toggle" onclick="toggleProgressDetails('${detailsId}')">${toggleText}</button>` : ''}
        </div>
        ${hasContent ? `<div class="progress-timeline ${expandedClass}" id="${detailsId}-timeline">${timelineHTML}</div>` : '<div class="progress-timeline-empty">暂无过程详情（可能执行过快或未触发详细事件）</div>'}
    `;
    
    contentWrapper.appendChild(bubble);
    detailsDiv.appendChild(contentWrapper);
    
    // 将详情组件插入到助手消息之后
    const messagesDiv = document.getElementById('chat-messages');
    // assistantElement 是消息div，需要插入到它的下一个兄弟节点之前
    if (assistantElement.nextSibling) {
        messagesDiv.insertBefore(detailsDiv, assistantElement.nextSibling);
    } else {
        // 如果没有下一个兄弟节点，直接追加
        messagesDiv.appendChild(detailsDiv);
    }
    
    // 移除原来的进度消息
    removeMessage(progressId);
    
    // 滚动到底部
    messagesDiv.scrollTop = messagesDiv.scrollHeight;
}

// 处理流式事件
function handleStreamEvent(event, progressElement, progressId, 
                          getAssistantId, setAssistantId, getMcpIds, setMcpIds) {
    const timeline = document.getElementById(progressId + '-timeline');
    if (!timeline) return;
    
    switch (event.type) {
        case 'conversation':
            if (event.data && event.data.conversationId) {
                updateProgressConversation(progressId, event.data.conversationId);
                currentConversationId = event.data.conversationId;
                updateActiveConversation();
                addAttackChainButton(currentConversationId);
                loadActiveTasks();
                // 立即刷新对话列表，让新对话显示在历史记录中
                loadConversations();
            }
            break;
        case 'iteration':
            // 添加迭代标记
            addTimelineItem(timeline, 'iteration', {
                title: `第 ${event.data?.iteration || 1} 轮迭代`,
                message: event.message,
                data: event.data
            });
            break;
            
        case 'thinking':
            // 显示AI思考内容
            addTimelineItem(timeline, 'thinking', {
                title: '🤔 AI思考',
                message: event.message,
                data: event.data
            });
            break;
            
        case 'tool_calls_detected':
            // 工具调用检测
            addTimelineItem(timeline, 'tool_calls_detected', {
                title: `🔧 检测到 ${event.data?.count || 0} 个工具调用`,
                message: event.message,
                data: event.data
            });
            break;
            
        case 'tool_call':
            // 显示工具调用信息
            const toolInfo = event.data || {};
            const toolName = toolInfo.toolName || '未知工具';
            const index = toolInfo.index || 0;
            const total = toolInfo.total || 0;
            addTimelineItem(timeline, 'tool_call', {
                title: `🔧 调用工具: ${escapeHtml(toolName)} (${index}/${total})`,
                message: event.message,
                data: toolInfo,
                expanded: false
            });
            break;
            
        case 'tool_result':
            // 显示工具执行结果
            const resultInfo = event.data || {};
            const resultToolName = resultInfo.toolName || '未知工具';
            const success = resultInfo.success !== false;
            const statusIcon = success ? '✅' : '❌';
            addTimelineItem(timeline, 'tool_result', {
                title: `${statusIcon} 工具 ${escapeHtml(resultToolName)} 执行${success ? '完成' : '失败'}`,
                message: event.message,
                data: resultInfo,
                expanded: false
            });
            break;
            
        case 'progress':
            // 更新进度状态
            const progressTitle = document.querySelector(`#${progressId} .progress-title`);
            if (progressTitle) {
                progressTitle.textContent = '🔍 ' + event.message;
            }
            break;
        
        case 'cancelled':
            // 显示错误
            addTimelineItem(timeline, 'cancelled', {
                title: '⛔ 任务已取消',
                message: event.message,
                data: event.data
            });
            
            // 更新进度标题为取消状态
            const cancelTitle = document.querySelector(`#${progressId} .progress-title`);
            if (cancelTitle) {
                cancelTitle.textContent = '⛔ 任务已取消';
            }
            
            // 更新进度容器为已完成状态（添加completed类）
            const cancelProgressContainer = document.querySelector(`#${progressId} .progress-container`);
            if (cancelProgressContainer) {
                cancelProgressContainer.classList.add('completed');
            }
            
            // 完成进度任务（标记为已取消）
            if (progressTaskState.has(progressId)) {
                finalizeProgressTask(progressId, '已取消');
            }
            
            // 如果取消事件包含messageId，说明有助手消息，需要显示取消内容
            if (event.data && event.data.messageId) {
                // 检查助手消息是否已存在
                let assistantId = event.data.messageId;
                let assistantElement = document.getElementById(assistantId);
                
                // 如果助手消息不存在，创建它
                if (!assistantElement) {
                    assistantId = addMessage('assistant', event.message, null, progressId);
                    setAssistantId(assistantId);
                    assistantElement = document.getElementById(assistantId);
                } else {
                    // 如果已存在，更新内容
                    const bubble = assistantElement.querySelector('.message-bubble');
                    if (bubble) {
                        bubble.innerHTML = escapeHtml(event.message).replace(/\n/g, '<br>');
                    }
                }
                
                // 将进度详情集成到工具调用区域（如果还没有）
                if (assistantElement) {
                    const detailsId = 'process-details-' + assistantId;
                    if (!document.getElementById(detailsId)) {
                        integrateProgressToMCPSection(progressId, assistantId);
                    }
                    // 立即折叠详情（取消时应该默认折叠）
                    setTimeout(() => {
                        collapseAllProgressDetails(assistantId, progressId);
                    }, 100);
                }
            } else {
                // 如果没有messageId，创建助手消息并集成详情
                const assistantId = addMessage('assistant', event.message, null, progressId);
                setAssistantId(assistantId);
                
                // 将进度详情集成到工具调用区域
                setTimeout(() => {
                    integrateProgressToMCPSection(progressId, assistantId);
                    // 确保详情默认折叠
                    collapseAllProgressDetails(assistantId, progressId);
                }, 100);
            }
            
            // 立即刷新任务状态
            loadActiveTasks();
            break;
            
        case 'response':
            // 先添加助手回复
            const responseData = event.data || {};
            const mcpIds = responseData.mcpExecutionIds || [];
            setMcpIds(mcpIds);
            
            // 更新对话ID
            if (responseData.conversationId) {
                currentConversationId = responseData.conversationId;
                updateActiveConversation();
                addAttackChainButton(currentConversationId);
                updateProgressConversation(progressId, responseData.conversationId);
                loadActiveTasks();
            }
            
            // 添加助手回复，并传入进度ID以便集成详情
            const assistantId = addMessage('assistant', event.message, mcpIds, progressId);
            setAssistantId(assistantId);
            
            // 将进度详情集成到工具调用区域
            integrateProgressToMCPSection(progressId, assistantId);
            
            // 延迟自动折叠详情（3秒后）
            setTimeout(() => {
                collapseAllProgressDetails(assistantId, progressId);
            }, 3000);
            
            // 刷新对话列表
            loadConversations();
            break;
            
        case 'error':
            // 显示错误
            addTimelineItem(timeline, 'error', {
                title: '❌ 错误',
                message: event.message,
                data: event.data
            });
            
            // 更新进度标题为错误状态
            const errorTitle = document.querySelector(`#${progressId} .progress-title`);
            if (errorTitle) {
                errorTitle.textContent = '❌ 执行失败';
            }
            
            // 更新进度容器为已完成状态（添加completed类）
            const progressContainer = document.querySelector(`#${progressId} .progress-container`);
            if (progressContainer) {
                progressContainer.classList.add('completed');
            }
            
            // 完成进度任务（标记为失败）
            if (progressTaskState.has(progressId)) {
                finalizeProgressTask(progressId, '已失败');
            }
            
            // 如果错误事件包含messageId，说明有助手消息，需要显示错误内容
            if (event.data && event.data.messageId) {
                // 检查助手消息是否已存在
                let assistantId = event.data.messageId;
                let assistantElement = document.getElementById(assistantId);
                
                // 如果助手消息不存在，创建它
                if (!assistantElement) {
                    assistantId = addMessage('assistant', event.message, null, progressId);
                    setAssistantId(assistantId);
                    assistantElement = document.getElementById(assistantId);
                } else {
                    // 如果已存在，更新内容
                    const bubble = assistantElement.querySelector('.message-bubble');
                    if (bubble) {
                        bubble.innerHTML = escapeHtml(event.message).replace(/\n/g, '<br>');
                    }
                }
                
                // 将进度详情集成到工具调用区域（如果还没有）
                if (assistantElement) {
                    const detailsId = 'process-details-' + assistantId;
                    if (!document.getElementById(detailsId)) {
                        integrateProgressToMCPSection(progressId, assistantId);
                    }
                    // 立即折叠详情（错误时应该默认折叠）
                    setTimeout(() => {
                        collapseAllProgressDetails(assistantId, progressId);
                    }, 100);
                }
            } else {
                // 如果没有messageId（比如任务已运行时的错误），创建助手消息并集成详情
                const assistantId = addMessage('assistant', event.message, null, progressId);
                setAssistantId(assistantId);
                
                // 将进度详情集成到工具调用区域
                setTimeout(() => {
                    integrateProgressToMCPSection(progressId, assistantId);
                    // 确保详情默认折叠
                    collapseAllProgressDetails(assistantId, progressId);
                }, 100);
            }
            
            // 立即刷新任务状态（执行失败时任务状态会更新）
            loadActiveTasks();
            break;
            
        case 'done':
            // 完成，更新进度标题（如果进度消息还存在）
            const doneTitle = document.querySelector(`#${progressId} .progress-title`);
            if (doneTitle) {
                doneTitle.textContent = '✅ 渗透测试完成';
            }
            // 更新对话ID
            if (event.data && event.data.conversationId) {
                currentConversationId = event.data.conversationId;
                updateActiveConversation();
                addAttackChainButton(currentConversationId);
                updateProgressConversation(progressId, event.data.conversationId);
            }
            if (progressTaskState.has(progressId)) {
                finalizeProgressTask(progressId, '已完成');
            }
            
            // 检查时间线中是否有错误项
            const hasError = timeline && timeline.querySelector('.timeline-item-error');
            
            // 立即刷新任务状态（确保任务状态同步）
            loadActiveTasks();
            
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
    
    // 自动滚动到底部
    const messagesDiv = document.getElementById('chat-messages');
    messagesDiv.scrollTop = messagesDiv.scrollHeight;
}

// 添加时间线项目
function addTimelineItem(timeline, type, options) {
    const item = document.createElement('div');
    item.className = `timeline-item timeline-item-${type}`;
    
    const time = new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' });
    
    let content = `
        <div class="timeline-item-header">
            <span class="timeline-item-time">${time}</span>
            <span class="timeline-item-title">${escapeHtml(options.title || '')}</span>
        </div>
    `;
    
    // 根据类型添加详细内容
    if (type === 'thinking' && options.message) {
        content += `<div class="timeline-item-content">${formatMarkdown(options.message)}</div>`;
    } else if (type === 'tool_call' && options.data) {
        const data = options.data;
        const args = data.argumentsObj || (data.arguments ? JSON.parse(data.arguments) : {});
        content += `
            <div class="timeline-item-content">
                <div class="tool-details">
                    <div class="tool-arg-section">
                        <strong>参数:</strong>
                        <pre class="tool-args">${escapeHtml(JSON.stringify(args, null, 2))}</pre>
                    </div>
                </div>
            </div>
        `;
    } else if (type === 'tool_result' && options.data) {
        const data = options.data;
        const isError = data.isError || !data.success;
        const result = data.result || data.error || '无结果';
        // 确保 result 是字符串
        const resultStr = typeof result === 'string' ? result : JSON.stringify(result);
        content += `
            <div class="timeline-item-content">
                <div class="tool-result-section ${isError ? 'error' : 'success'}">
                    <strong>执行结果:</strong>
                    <pre class="tool-result">${escapeHtml(resultStr)}</pre>
                    ${data.executionId ? `<div class="tool-execution-id">执行ID: <code>${escapeHtml(data.executionId)}</code></div>` : ''}
                </div>
            </div>
        `;
    } else if (type === 'cancelled') {
        content += `
            <div class="timeline-item-content">
                ${escapeHtml(options.message || '任务已取消')}
            </div>
        `;
    }
    
    item.innerHTML = content;
    timeline.appendChild(item);
    
    // 自动展开详情
    const expanded = timeline.classList.contains('expanded');
    if (!expanded && (type === 'tool_call' || type === 'tool_result')) {
        // 对于工具调用和结果，默认显示摘要
    }
}

// 消息计数器，确保ID唯一
let messageCounter = 0;

// 添加消息
function addMessage(role, content, mcpExecutionIds = null, progressId = null) {
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
    
    // 先使用 DOMPurify 清理（如果可用），这样可以处理已经是 HTML 的内容
    if (typeof DOMPurify !== 'undefined') {
        // 配置 DOMPurify 允许的标签和属性
        const sanitizeConfig = {
            // 允许基本的 Markdown 格式化标签
            ALLOWED_TAGS: ['p', 'br', 'strong', 'em', 'u', 's', 'code', 'pre', 'blockquote', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'ul', 'ol', 'li', 'a', 'img', 'table', 'thead', 'tbody', 'tr', 'th', 'td', 'hr'],
            ALLOWED_ATTR: ['href', 'title', 'alt', 'src', 'class'],
            ALLOW_DATA_ATTR: false,
        };
        
        // 如果内容看起来已经是 HTML（包含 HTML 标签），直接清理
        // 否则先用 marked.js 解析 Markdown，再清理
        if (typeof marked !== 'undefined' && !/<[a-z][\s\S]*>/i.test(content)) {
            // 内容不包含 HTML 标签，可能是 Markdown，使用 marked.js 解析
            try {
                marked.setOptions({
                    breaks: true,
                    gfm: true,
                });
                let parsedContent = marked.parse(content);
                formattedContent = DOMPurify.sanitize(parsedContent, sanitizeConfig);
            } catch (e) {
                console.error('Markdown 解析失败:', e);
                // 降级处理：直接清理原始内容
                formattedContent = DOMPurify.sanitize(content, sanitizeConfig);
            }
        } else {
            // 内容包含 HTML 标签或 marked.js 不可用，直接清理
            formattedContent = DOMPurify.sanitize(content, sanitizeConfig);
        }
    } else if (typeof marked !== 'undefined') {
        // 没有 DOMPurify，但有 marked.js
        try {
            marked.setOptions({
                breaks: true,
                gfm: true,
            });
            formattedContent = marked.parse(content);
        } catch (e) {
            console.error('Markdown 解析失败:', e);
            formattedContent = escapeHtml(content).replace(/\n/g, '<br>');
        }
    } else {
        // 都没有，简单转义
        formattedContent = escapeHtml(content).replace(/\n/g, '<br>');
    }
    
    bubble.innerHTML = formattedContent;
    contentWrapper.appendChild(bubble);
    
    // 添加时间戳
    const timeDiv = document.createElement('div');
    timeDiv.className = 'message-time';
    timeDiv.textContent = new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' });
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
    if (!processDetails || processDetails.length === 0) {
        return;
    }
    
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
    
    // 创建时间线
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

// 回车发送消息，Shift+Enter 换行
const chatInput = document.getElementById('chat-input');
chatInput.addEventListener('keydown', function(e) {
    if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        sendMessage();
    }
    // Shift+Enter 允许默认行为（换行）
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
            document.getElementById('detail-status').textContent = getStatusText(exec.status);
            document.getElementById('detail-time').textContent = new Date(exec.startTime).toLocaleString('zh-CN');
            
            // 请求参数
            const requestData = {
                tool: exec.toolName,
                arguments: exec.arguments
            };
            document.getElementById('detail-request').textContent = JSON.stringify(requestData, null, 2);
            
            // 响应结果
            if (exec.result) {
                const responseData = {
                    content: exec.result.content,
                    isError: exec.result.isError
                };
                document.getElementById('detail-response').textContent = JSON.stringify(responseData, null, 2);
                document.getElementById('detail-response').className = exec.result.isError ? 'code-block error' : 'code-block';
            } else {
                document.getElementById('detail-response').textContent = '暂无响应数据';
            }
            
            // 错误信息
            if (exec.error) {
                document.getElementById('detail-error-section').style.display = 'block';
                document.getElementById('detail-error').textContent = exec.error;
            } else {
                document.getElementById('detail-error-section').style.display = 'none';
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


// 工具函数
function getStatusText(status) {
    const statusMap = {
        'pending': '等待中',
        'running': '执行中',
        'completed': '已完成',
        'failed': '失败'
    };
    return statusMap[status] || status;
}

function formatDuration(ms) {
    const seconds = Math.floor(ms / 1000);
    const minutes = Math.floor(seconds / 60);
    const hours = Math.floor(minutes / 60);
    
    if (hours > 0) {
        return `${hours}小时${minutes % 60}分钟`;
    } else if (minutes > 0) {
        return `${minutes}分钟${seconds % 60}秒`;
    } else {
        return `${seconds}秒`;
    }
}

function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

function formatMarkdown(text) {
    // 配置 DOMPurify 允许的标签和属性
    const sanitizeConfig = {
        ALLOWED_TAGS: ['p', 'br', 'strong', 'em', 'u', 's', 'code', 'pre', 'blockquote', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'ul', 'ol', 'li', 'a', 'img', 'table', 'thead', 'tbody', 'tr', 'th', 'td', 'hr'],
        ALLOWED_ATTR: ['href', 'title', 'alt', 'src', 'class'],
        ALLOW_DATA_ATTR: false,
    };
    
    if (typeof DOMPurify !== 'undefined') {
        // 如果内容看起来已经是 HTML（包含 HTML 标签），直接清理
        // 否则先用 marked.js 解析 Markdown，再清理
        if (typeof marked !== 'undefined' && !/<[a-z][\s\S]*>/i.test(text)) {
            // 内容不包含 HTML 标签，可能是 Markdown，使用 marked.js 解析
            try {
                marked.setOptions({
                    breaks: true,
                    gfm: true,
                });
                let parsedContent = marked.parse(text);
                return DOMPurify.sanitize(parsedContent, sanitizeConfig);
            } catch (e) {
                console.error('Markdown 解析失败:', e);
                return DOMPurify.sanitize(text, sanitizeConfig);
            }
        } else {
            // 内容包含 HTML 标签或 marked.js 不可用，直接清理
            return DOMPurify.sanitize(text, sanitizeConfig);
        }
    } else if (typeof marked !== 'undefined') {
        // 没有 DOMPurify，但有 marked.js
        try {
            marked.setOptions({
                breaks: true,
                gfm: true,
            });
            return marked.parse(text);
        } catch (e) {
            console.error('Markdown 解析失败:', e);
            return escapeHtml(text).replace(/\n/g, '<br>');
        }
    } else {
        return escapeHtml(text).replace(/\n/g, '<br>');
    }
}

// 开始新对话
function startNewConversation() {
    currentConversationId = null;
    document.getElementById('chat-messages').innerHTML = '';
    addMessage('assistant', '系统已就绪。请输入您的测试需求，系统将自动执行相应的安全测试。');
    addAttackChainButton(null);
    updateActiveConversation();
    // 刷新对话列表，确保显示最新的历史对话
    loadConversations();
}

// 加载对话列表
async function loadConversations() {
    try {
        const response = await apiFetch('/api/conversations?limit=50');
        const conversations = await response.json();
        
        const listContainer = document.getElementById('conversations-list');
        listContainer.innerHTML = '';
        
        if (conversations.length === 0) {
            listContainer.innerHTML = '<div style="padding: 20px; text-align: center; color: var(--text-muted); font-size: 0.875rem;">暂无历史对话</div>';
            return;
        }
        
        conversations.forEach(conv => {
            const item = document.createElement('div');
            item.className = 'conversation-item';
            item.dataset.conversationId = conv.id;
            if (conv.id === currentConversationId) {
                item.classList.add('active');
            }
            
            // 创建内容容器
            const contentWrapper = document.createElement('div');
            contentWrapper.className = 'conversation-content';
            
            const title = document.createElement('div');
            title.className = 'conversation-title';
            title.textContent = conv.title || '未命名对话';
            contentWrapper.appendChild(title);
            
            const time = document.createElement('div');
            time.className = 'conversation-time';
            // 解析时间，支持多种格式
            let dateObj;
            if (conv.updatedAt) {
                dateObj = new Date(conv.updatedAt);
                // 检查日期是否有效
                if (isNaN(dateObj.getTime())) {
                    // 如果解析失败，尝试其他格式
                    console.warn('时间解析失败:', conv.updatedAt);
                    dateObj = new Date();
                }
            } else {
                dateObj = new Date();
            }
            
            // 格式化时间显示
            const now = new Date();
            const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
            const yesterday = new Date(today);
            yesterday.setDate(yesterday.getDate() - 1);
            const messageDate = new Date(dateObj.getFullYear(), dateObj.getMonth(), dateObj.getDate());
            
            let timeText;
            if (messageDate.getTime() === today.getTime()) {
                // 今天：只显示时间
                timeText = dateObj.toLocaleTimeString('zh-CN', {
                    hour: '2-digit',
                    minute: '2-digit'
                });
            } else if (messageDate.getTime() === yesterday.getTime()) {
                // 昨天
                timeText = '昨天 ' + dateObj.toLocaleTimeString('zh-CN', {
                    hour: '2-digit',
                    minute: '2-digit'
                });
            } else if (now.getFullYear() === dateObj.getFullYear()) {
                // 今年：显示月日和时间
                timeText = dateObj.toLocaleString('zh-CN', {
                    month: 'short',
                    day: 'numeric',
                    hour: '2-digit',
                    minute: '2-digit'
                });
            } else {
                // 去年或更早：显示完整日期和时间
                timeText = dateObj.toLocaleString('zh-CN', {
                    year: 'numeric',
                    month: 'short',
                    day: 'numeric',
                    hour: '2-digit',
                    minute: '2-digit'
                });
            }
            
            time.textContent = timeText;
            contentWrapper.appendChild(time);
            
            item.appendChild(contentWrapper);
            
            // 创建删除按钮
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
                e.stopPropagation(); // 阻止触发对话加载
                deleteConversation(conv.id);
            };
            item.appendChild(deleteBtn);
            
            item.onclick = () => loadConversation(conv.id);
            listContainer.appendChild(item);
        });
    } catch (error) {
        console.error('加载对话列表失败:', error);
    }
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
        
        // 更新当前对话ID
        currentConversationId = conversationId;
        updateActiveConversation();
        
        // 清空消息区域
        const messagesDiv = document.getElementById('chat-messages');
        messagesDiv.innerHTML = '';
        
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
                
                const messageId = addMessage(msg.role, displayContent, msg.mcpExecutionIds || []);
                // 如果有过程详情，显示它们
                if (msg.processDetails && msg.processDetails.length > 0 && msg.role === 'assistant') {
                    // 延迟一下，确保消息已经渲染
                    setTimeout(() => {
                        renderProcessDetails(messageId, msg.processDetails);
                        // 检查是否有错误或取消事件，如果有，确保详情默认折叠
                        const hasErrorOrCancelled = msg.processDetails.some(d => 
                            d.eventType === 'error' || d.eventType === 'cancelled'
                        );
                        if (hasErrorOrCancelled) {
                            collapseAllProgressDetails(messageId, null);
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
async function deleteConversation(conversationId) {
    // 确认删除
    if (!confirm('确定要删除这个对话吗？此操作不可恢复。')) {
        return;
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

// 加载活跃任务列表
async function loadActiveTasks(showErrors = false) {
    const bar = document.getElementById('active-tasks-bar');
    try {
        const response = await apiFetch('/api/agent-loop/tasks');
        const result = await response.json().catch(() => ({}));

        if (!response.ok) {
            throw new Error(result.error || '获取活跃任务失败');
        }

        renderActiveTasks(result.tasks || []);
    } catch (error) {
        console.error('获取活跃任务失败:', error);
        if (showErrors && bar) {
            bar.style.display = 'block';
            bar.innerHTML = `<div class="active-task-error">无法获取任务状态：${escapeHtml(error.message)}</div>`;
        }
    }
}

function renderActiveTasks(tasks) {
    const bar = document.getElementById('active-tasks-bar');
    if (!bar) return;

    if (!tasks || tasks.length === 0) {
        bar.style.display = 'none';
        bar.innerHTML = '';
        return;
    }

    bar.style.display = 'flex';
    bar.innerHTML = '';

    tasks.forEach(task => {
        const item = document.createElement('div');
        item.className = 'active-task-item';

        const startedTime = task.startedAt ? new Date(task.startedAt) : null;
        const timeText = startedTime && !isNaN(startedTime.getTime())
            ? startedTime.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
            : '';

        // 根据任务状态显示不同的文本
        const statusMap = {
            'running': '执行中',
            'cancelling': '取消中',
            'failed': '执行失败',
            'timeout': '执行超时',
            'cancelled': '已取消',
            'completed': '已完成'
        };
        const statusText = statusMap[task.status] || '执行中';
        const isFinalStatus = ['failed', 'timeout', 'cancelled', 'completed'].includes(task.status);

        item.innerHTML = `
            <div class="active-task-info">
                <span class="active-task-status">${statusText}</span>
                <span class="active-task-message">${escapeHtml(task.message || '未命名任务')}</span>
            </div>
            <div class="active-task-actions">
                ${timeText ? `<span class="active-task-time">${timeText}</span>` : ''}
                ${!isFinalStatus ? '<button class="active-task-cancel">停止任务</button>' : ''}
            </div>
        `;

        // 只有非最终状态的任务才显示停止按钮
        if (!isFinalStatus) {
            const cancelBtn = item.querySelector('.active-task-cancel');
            if (cancelBtn) {
                cancelBtn.onclick = () => cancelActiveTask(task.conversationId, cancelBtn);
                if (task.status === 'cancelling') {
                    cancelBtn.disabled = true;
                    cancelBtn.textContent = '取消中...';
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
    button.textContent = '取消中...';

    try {
        await requestCancel(conversationId);
        loadActiveTasks();
    } catch (error) {
        console.error('取消任务失败:', error);
        alert('取消任务失败: ' + error.message);
        button.disabled = false;
        button.textContent = originalText;
    }
}

// 设置相关功能
let currentConfig = null;
let allTools = [];
// 全局工具状态映射，用于保存用户在所有页面的修改
// key: tool.name, value: { enabled: boolean, is_external: boolean, external_mcp: string }
let toolStateMap = new Map();
// 从localStorage读取每页显示数量，默认为20
const getToolsPageSize = () => {
    const saved = localStorage.getItem('toolsPageSize');
    return saved ? parseInt(saved, 10) : 20;
};

let toolsPagination = {
    page: 1,
    pageSize: getToolsPageSize(),
    total: 0,
    totalPages: 0
};

// 打开设置
async function openSettings() {
    const modal = document.getElementById('settings-modal');
    modal.style.display = 'block';
    
    // 每次打开时清空全局状态映射，重新加载最新配置
    toolStateMap.clear();
    
    // 每次打开时重新加载最新配置
    await loadConfig();
    
    // 清除之前的验证错误状态
    document.querySelectorAll('.form-group input').forEach(input => {
        input.classList.remove('error');
    });
}

// 关闭设置
function closeSettings() {
    const modal = document.getElementById('settings-modal');
    modal.style.display = 'none';
}

// 点击模态框外部关闭
window.onclick = function(event) {
    const settingsModal = document.getElementById('settings-modal');
    const mcpModal = document.getElementById('mcp-detail-modal');
    const monitorModal = document.getElementById('monitor-modal');
    
    if (event.target === settingsModal) {
        closeSettings();
    }
    if (event.target === mcpModal) {
        closeMCPDetail();
    }
    if (event.target === monitorModal) {
        closeMonitorPanel();
    }
}

// 加载配置
async function loadConfig() {
    try {
        const response = await apiFetch('/api/config');
        if (!response.ok) {
            throw new Error('获取配置失败');
        }
        
        currentConfig = await response.json();
        
        // 填充OpenAI配置
        document.getElementById('openai-api-key').value = currentConfig.openai.api_key || '';
        document.getElementById('openai-base-url').value = currentConfig.openai.base_url || '';
        document.getElementById('openai-model').value = currentConfig.openai.model || '';
        
        // 填充Agent配置
        document.getElementById('agent-max-iterations').value = currentConfig.agent.max_iterations || 30;
        
        // 设置每页显示数量（会在分页控件渲染时设置）
        const savedPageSize = getToolsPageSize();
        toolsPagination.pageSize = savedPageSize;
        
        // 加载工具列表（使用分页）
        toolsSearchKeyword = '';
        await loadToolsList(1, '');
    } catch (error) {
        console.error('加载配置失败:', error);
        alert('加载配置失败: ' + error.message);
    }
}

// 工具搜索关键词
let toolsSearchKeyword = '';

// 加载工具列表（分页）
async function loadToolsList(page = 1, searchKeyword = '') {
    try {
        // 在加载新页面之前，先保存当前页的状态到全局映射
        saveCurrentPageToolStates();
        
        const pageSize = toolsPagination.pageSize;
        let url = `/api/config/tools?page=${page}&page_size=${pageSize}`;
        if (searchKeyword) {
            url += `&search=${encodeURIComponent(searchKeyword)}`;
        }
        
        const response = await apiFetch(url);
        if (!response.ok) {
            throw new Error('获取工具列表失败');
        }
        
        const result = await response.json();
        allTools = result.tools || [];
        toolsPagination = {
            page: result.page || page,
            pageSize: result.page_size || pageSize,
            total: result.total || 0,
            totalPages: result.total_pages || 1
        };
        
        // 初始化工具状态映射（如果工具不在映射中，使用服务器返回的状态）
        allTools.forEach(tool => {
            if (!toolStateMap.has(tool.name)) {
                toolStateMap.set(tool.name, {
                    enabled: tool.enabled,
                    is_external: tool.is_external || false,
                    external_mcp: tool.external_mcp || ''
                });
            }
        });
        
        renderToolsList();
        renderToolsPagination();
    } catch (error) {
        console.error('加载工具列表失败:', error);
        const toolsList = document.getElementById('tools-list');
        if (toolsList) {
            toolsList.innerHTML = `<div class="error">加载工具列表失败: ${escapeHtml(error.message)}</div>`;
        }
    }
}

// 保存当前页的工具状态到全局映射
function saveCurrentPageToolStates() {
    document.querySelectorAll('#tools-list .tool-item').forEach(item => {
        const checkbox = item.querySelector('input[type="checkbox"]');
        const toolName = item.dataset.toolName;
        const isExternal = item.dataset.isExternal === 'true';
        const externalMcp = item.dataset.externalMcp || '';
        if (toolName && checkbox) {
            toolStateMap.set(toolName, {
                enabled: checkbox.checked,
                is_external: isExternal,
                external_mcp: externalMcp
            });
        }
    });
}

// 搜索工具
function searchTools() {
    const searchInput = document.getElementById('tools-search');
    const keyword = searchInput ? searchInput.value.trim() : '';
    toolsSearchKeyword = keyword;
    // 搜索时重置到第一页
    loadToolsList(1, keyword);
}

// 清除搜索
function clearSearch() {
    const searchInput = document.getElementById('tools-search');
    if (searchInput) {
        searchInput.value = '';
    }
    toolsSearchKeyword = '';
    loadToolsList(1, '');
}

// 处理搜索框回车事件
function handleSearchKeyPress(event) {
    if (event.key === 'Enter') {
        searchTools();
    }
}

// 渲染工具列表
function renderToolsList() {
    const toolsList = document.getElementById('tools-list');
    if (!toolsList) return;
    
    // 只渲染列表部分，分页控件单独渲染
    const listContainer = toolsList.querySelector('.tools-list-items') || document.createElement('div');
    listContainer.className = 'tools-list-items';
    listContainer.innerHTML = '';
    
    if (allTools.length === 0) {
        listContainer.innerHTML = '<div class="empty">暂无工具</div>';
        if (!toolsList.contains(listContainer)) {
            toolsList.appendChild(listContainer);
        }
        // 更新统计
        updateToolsStats();
        return;
    }
    
    allTools.forEach(tool => {
        const toolItem = document.createElement('div');
        toolItem.className = 'tool-item';
        toolItem.dataset.toolName = tool.name; // 保存原始工具名称
        toolItem.dataset.isExternal = tool.is_external ? 'true' : 'false';
        toolItem.dataset.externalMcp = tool.external_mcp || '';
        
        // 从全局状态映射获取工具状态，如果不存在则使用服务器返回的状态
        const toolState = toolStateMap.get(tool.name) || {
            enabled: tool.enabled,
            is_external: tool.is_external || false,
            external_mcp: tool.external_mcp || ''
        };
        
        // 外部工具标签
        const externalBadge = toolState.is_external ? '<span class="external-tool-badge" title="外部MCP工具">外部</span>' : '';
        
        toolItem.innerHTML = `
            <input type="checkbox" id="tool-${tool.name}" ${toolState.enabled ? 'checked' : ''} ${toolState.is_external ? 'data-external="true"' : ''} onchange="handleToolCheckboxChange('${tool.name}', this.checked)" />
            <div class="tool-item-info">
                <div class="tool-item-name">
                    ${escapeHtml(tool.name)}
                    ${externalBadge}
                </div>
                <div class="tool-item-desc">${escapeHtml(tool.description || '无描述')}</div>
            </div>
        `;
        listContainer.appendChild(toolItem);
    });
    
    if (!toolsList.contains(listContainer)) {
        toolsList.appendChild(listContainer);
    }
    
    // 更新统计
    updateToolsStats();
}

// 渲染工具列表分页控件
function renderToolsPagination() {
    const toolsList = document.getElementById('tools-list');
    if (!toolsList) return;
    
    // 移除旧的分页控件
    const oldPagination = toolsList.querySelector('.tools-pagination');
    if (oldPagination) {
        oldPagination.remove();
    }
    
    // 如果只有一页或没有数据，不显示分页
    if (toolsPagination.totalPages <= 1) {
        return;
    }
    
    const pagination = document.createElement('div');
    pagination.className = 'tools-pagination';
    
    const { page, totalPages, total } = toolsPagination;
    const startItem = (page - 1) * toolsPagination.pageSize + 1;
    const endItem = Math.min(page * toolsPagination.pageSize, total);
    
    const savedPageSize = getToolsPageSize();
    pagination.innerHTML = `
        <div class="pagination-info">
            显示 ${startItem}-${endItem} / 共 ${total} 个工具${toolsSearchKeyword ? ` (搜索: "${escapeHtml(toolsSearchKeyword)}")` : ''}
        </div>
        <div class="pagination-page-size">
            <label for="tools-page-size-pagination">每页:</label>
            <select id="tools-page-size-pagination" onchange="changeToolsPageSize()">
                <option value="10" ${savedPageSize === 10 ? 'selected' : ''}>10</option>
                <option value="20" ${savedPageSize === 20 ? 'selected' : ''}>20</option>
                <option value="50" ${savedPageSize === 50 ? 'selected' : ''}>50</option>
                <option value="100" ${savedPageSize === 100 ? 'selected' : ''}>100</option>
            </select>
        </div>
        <div class="pagination-controls">
            <button class="btn-secondary" onclick="loadToolsList(1, '${escapeHtml(toolsSearchKeyword)}')" ${page === 1 ? 'disabled' : ''}>首页</button>
            <button class="btn-secondary" onclick="loadToolsList(${page - 1}, '${escapeHtml(toolsSearchKeyword)}')" ${page === 1 ? 'disabled' : ''}>上一页</button>
            <span class="pagination-page">第 ${page} / ${totalPages} 页</span>
            <button class="btn-secondary" onclick="loadToolsList(${page + 1}, '${escapeHtml(toolsSearchKeyword)}')" ${page === totalPages ? 'disabled' : ''}>下一页</button>
            <button class="btn-secondary" onclick="loadToolsList(${totalPages}, '${escapeHtml(toolsSearchKeyword)}')" ${page === totalPages ? 'disabled' : ''}>末页</button>
        </div>
    `;
    
    toolsList.appendChild(pagination);
}

// 处理工具checkbox状态变化
function handleToolCheckboxChange(toolName, enabled) {
    // 更新全局状态映射
    const toolItem = document.querySelector(`.tool-item[data-tool-name="${toolName}"]`);
    if (toolItem) {
        const isExternal = toolItem.dataset.isExternal === 'true';
        const externalMcp = toolItem.dataset.externalMcp || '';
        toolStateMap.set(toolName, {
            enabled: enabled,
            is_external: isExternal,
            external_mcp: externalMcp
        });
    }
    updateToolsStats();
}

// 全选工具
function selectAllTools() {
    document.querySelectorAll('#tools-list input[type="checkbox"]').forEach(checkbox => {
        checkbox.checked = true;
        // 更新全局状态映射
        const toolItem = checkbox.closest('.tool-item');
        if (toolItem) {
            const toolName = toolItem.dataset.toolName;
            const isExternal = toolItem.dataset.isExternal === 'true';
            const externalMcp = toolItem.dataset.externalMcp || '';
            if (toolName) {
                toolStateMap.set(toolName, {
                    enabled: true,
                    is_external: isExternal,
                    external_mcp: externalMcp
                });
            }
        }
    });
    updateToolsStats();
}

// 全不选工具
function deselectAllTools() {
    document.querySelectorAll('#tools-list input[type="checkbox"]').forEach(checkbox => {
        checkbox.checked = false;
        // 更新全局状态映射
        const toolItem = checkbox.closest('.tool-item');
        if (toolItem) {
            const toolName = toolItem.dataset.toolName;
            const isExternal = toolItem.dataset.isExternal === 'true';
            const externalMcp = toolItem.dataset.externalMcp || '';
            if (toolName) {
                toolStateMap.set(toolName, {
                    enabled: false,
                    is_external: isExternal,
                    external_mcp: externalMcp
                });
            }
        }
    });
    updateToolsStats();
}

// 改变每页显示数量
async function changeToolsPageSize() {
    // 尝试从两个位置获取选择器（顶部或分页区域）
    const pageSizeSelect = document.getElementById('tools-page-size') || document.getElementById('tools-page-size-pagination');
    if (!pageSizeSelect) return;
    
    const newPageSize = parseInt(pageSizeSelect.value, 10);
    if (isNaN(newPageSize) || newPageSize < 1) {
        return;
    }
    
    // 保存到localStorage
    localStorage.setItem('toolsPageSize', newPageSize.toString());
    
    // 更新分页配置
    toolsPagination.pageSize = newPageSize;
    
    // 同步更新另一个选择器（如果存在）
    const otherSelect = document.getElementById('tools-page-size') || document.getElementById('tools-page-size-pagination');
    if (otherSelect && otherSelect !== pageSizeSelect) {
        otherSelect.value = newPageSize;
    }
    
    // 重新加载第一页
    await loadToolsList(1, toolsSearchKeyword);
}

// 更新工具统计信息
async function updateToolsStats() {
    const statsEl = document.getElementById('tools-stats');
    if (!statsEl) return;
    
    // 先保存当前页的状态到全局映射
    saveCurrentPageToolStates();
    
    // 计算当前页的启用工具数
    const currentPageEnabled = Array.from(document.querySelectorAll('#tools-list input[type="checkbox"]:checked')).length;
    const currentPageTotal = document.querySelectorAll('#tools-list input[type="checkbox"]').length;
    
    // 计算所有工具的启用数
    let totalEnabled = 0;
    let totalTools = toolsPagination.total || 0;
    
    try {
        // 如果有搜索关键词，只统计搜索结果
        if (toolsSearchKeyword) {
            totalTools = allTools.length;
            totalEnabled = allTools.filter(tool => {
                // 优先使用全局状态映射，否则使用checkbox状态，最后使用服务器返回的状态
                const savedState = toolStateMap.get(tool.name);
                if (savedState !== undefined) {
                    return savedState.enabled;
                }
                const checkbox = document.getElementById(`tool-${tool.name}`);
                return checkbox ? checkbox.checked : tool.enabled;
            }).length;
        } else {
            // 没有搜索时，需要获取所有工具的状态
            // 先使用全局状态映射和当前页的checkbox状态
            const localStateMap = new Map();
            
            // 从当前页的checkbox获取状态（如果全局映射中没有）
            allTools.forEach(tool => {
                const savedState = toolStateMap.get(tool.name);
                if (savedState !== undefined) {
                    localStateMap.set(tool.name, savedState.enabled);
                } else {
                    const checkbox = document.getElementById(`tool-${tool.name}`);
                    if (checkbox) {
                        localStateMap.set(tool.name, checkbox.checked);
                    } else {
                        // 如果checkbox不存在（不在当前页），使用工具原始状态
                        localStateMap.set(tool.name, tool.enabled);
                    }
                }
            });
            
            // 如果总工具数大于当前页，需要获取所有工具的状态
            if (totalTools > allTools.length) {
                // 遍历所有页面获取完整状态
                let page = 1;
                let hasMore = true;
                const pageSize = 100; // 使用较大的页面大小以减少请求次数
                
                while (hasMore && page <= 10) { // 限制最多10页，避免无限循环
                    const url = `/api/config/tools?page=${page}&page_size=${pageSize}`;
                    const pageResponse = await apiFetch(url);
                    if (!pageResponse.ok) break;
                    
                    const pageResult = await pageResponse.json();
                    pageResult.tools.forEach(tool => {
                        // 优先使用全局状态映射，否则使用服务器返回的状态
                        if (!localStateMap.has(tool.name)) {
                            const savedState = toolStateMap.get(tool.name);
                            localStateMap.set(tool.name, savedState ? savedState.enabled : tool.enabled);
                        }
                    });
                    
                    if (page >= pageResult.total_pages) {
                        hasMore = false;
                    } else {
                        page++;
                    }
                }
            }
            
            // 计算启用的工具数
            totalEnabled = Array.from(localStateMap.values()).filter(enabled => enabled).length;
        }
    } catch (error) {
        console.warn('获取工具统计失败，使用当前页数据', error);
        // 如果获取失败，使用当前页的数据
        totalTools = totalTools || currentPageTotal;
        totalEnabled = currentPageEnabled;
    }
    
    statsEl.innerHTML = `
        <span title="当前页启用的工具数">✅ 当前页已启用: <strong>${currentPageEnabled}</strong> / ${currentPageTotal}</span>
        <span title="所有工具中启用的工具总数">📊 总计已启用: <strong>${totalEnabled}</strong> / ${totalTools}</span>
    `;
}

// 过滤工具（已废弃，现在使用服务端搜索）
// 保留此函数以防其他地方调用，但实际功能已由searchTools()替代
function filterTools() {
    // 不再使用客户端过滤，改为触发服务端搜索
    // 可以保留为空函数或移除oninput事件
}

// 应用设置
async function applySettings() {
    try {
        // 清除之前的验证错误状态
        document.querySelectorAll('.form-group input').forEach(input => {
            input.classList.remove('error');
        });
        
        // 验证必填字段
        const apiKey = document.getElementById('openai-api-key').value.trim();
        const baseUrl = document.getElementById('openai-base-url').value.trim();
        const model = document.getElementById('openai-model').value.trim();
        
        let hasError = false;
        
        if (!apiKey) {
            document.getElementById('openai-api-key').classList.add('error');
            hasError = true;
        }
        
        if (!baseUrl) {
            document.getElementById('openai-base-url').classList.add('error');
            hasError = true;
        }
        
        if (!model) {
            document.getElementById('openai-model').classList.add('error');
            hasError = true;
        }
        
        if (hasError) {
            alert('请填写所有必填字段（标记为 * 的字段）');
            return;
        }
        
        // 收集配置
        const config = {
            openai: {
                api_key: apiKey,
                base_url: baseUrl,
                model: model
            },
            agent: {
                max_iterations: parseInt(document.getElementById('agent-max-iterations').value) || 30
            },
            tools: []
        };
        
        // 收集工具启用状态
        // 先保存当前页的状态到全局映射
        saveCurrentPageToolStates();
        
        // 获取所有工具列表以获取完整状态（遍历所有页面）
        // 注意：无论是否在搜索状态下，都要获取所有工具的状态，以确保完整保存
        try {
            const allToolsMap = new Map();
            let page = 1;
            let hasMore = true;
            const pageSize = 100; // 使用合理的页面大小
            
            // 遍历所有页面获取所有工具（不使用搜索关键词，获取全部工具）
            while (hasMore) {
                const url = `/api/config/tools?page=${page}&page_size=${pageSize}`;
                
                const pageResponse = await apiFetch(url);
                if (!pageResponse.ok) {
                    throw new Error('获取工具列表失败');
                }
                
                const pageResult = await pageResponse.json();
                
                // 将工具添加到映射中
                // 优先使用全局状态映射中的状态（用户修改过的），否则使用服务器返回的状态
                pageResult.tools.forEach(tool => {
                    const savedState = toolStateMap.get(tool.name);
                    allToolsMap.set(tool.name, {
                        name: tool.name,
                        enabled: savedState ? savedState.enabled : tool.enabled,
                        is_external: savedState ? savedState.is_external : (tool.is_external || false),
                        external_mcp: savedState ? savedState.external_mcp : (tool.external_mcp || '')
                    });
                });
                
                // 检查是否还有更多页面
                if (page >= pageResult.total_pages) {
                    hasMore = false;
                } else {
                    page++;
                }
            }
            
            // 将所有工具添加到配置中
            allToolsMap.forEach(tool => {
                config.tools.push({
                    name: tool.name,
                    enabled: tool.enabled,
                    is_external: tool.is_external,
                    external_mcp: tool.external_mcp
                });
            });
        } catch (error) {
            console.warn('获取所有工具列表失败，仅使用全局状态映射', error);
            // 如果获取失败，使用全局状态映射
            toolStateMap.forEach((toolData, toolName) => {
                config.tools.push({
                    name: toolName,
                    enabled: toolData.enabled,
                    is_external: toolData.is_external,
                    external_mcp: toolData.external_mcp
                });
            });
        }
        
        // 更新配置
        const updateResponse = await apiFetch('/api/config', {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(config)
        });
        
        if (!updateResponse.ok) {
            const error = await updateResponse.json();
            throw new Error(error.error || '更新配置失败');
        }
        
        // 应用配置
        const applyResponse = await apiFetch('/api/config/apply', {
            method: 'POST'
        });
        
        if (!applyResponse.ok) {
            const error = await applyResponse.json();
            throw new Error(error.error || '应用配置失败');
        }
        
        alert('配置已成功应用！');
        closeSettings();
    } catch (error) {
        console.error('应用配置失败:', error);
        alert('应用配置失败: ' + error.message);
    }
}

function resetPasswordForm() {
    const currentInput = document.getElementById('auth-current-password');
    const newInput = document.getElementById('auth-new-password');
    const confirmInput = document.getElementById('auth-confirm-password');

    [currentInput, newInput, confirmInput].forEach(input => {
        if (input) {
            input.value = '';
            input.classList.remove('error');
        }
    });
}

async function changePassword() {
    const currentInput = document.getElementById('auth-current-password');
    const newInput = document.getElementById('auth-new-password');
    const confirmInput = document.getElementById('auth-confirm-password');
    const submitBtn = document.querySelector('.change-password-submit');

    [currentInput, newInput, confirmInput].forEach(input => input && input.classList.remove('error'));

    const currentPassword = currentInput?.value.trim() || '';
    const newPassword = newInput?.value.trim() || '';
    const confirmPassword = confirmInput?.value.trim() || '';

    let hasError = false;

    if (!currentPassword) {
        currentInput?.classList.add('error');
        hasError = true;
    }

    if (!newPassword || newPassword.length < 8) {
        newInput?.classList.add('error');
        hasError = true;
    }

    if (newPassword !== confirmPassword) {
        confirmInput?.classList.add('error');
        hasError = true;
    }

    if (hasError) {
        alert('请正确填写当前密码和新密码，新密码至少 8 位且需要两次输入一致。');
        return;
    }

    if (submitBtn) {
        submitBtn.disabled = true;
    }

    try {
        const response = await apiFetch('/api/auth/change-password', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                oldPassword: currentPassword,
                newPassword: newPassword
            })
        });

        const result = await response.json().catch(() => ({}));
        if (!response.ok) {
            throw new Error(result.error || '修改密码失败');
        }

        alert('密码已更新，请使用新密码重新登录。');
        resetPasswordForm();
        handleUnauthorized({ message: '密码已更新，请使用新密码重新登录。', silent: false });
        closeSettings();
    } catch (error) {
        console.error('修改密码失败:', error);
        alert('修改密码失败: ' + error.message);
    } finally {
        if (submitBtn) {
            submitBtn.disabled = false;
        }
    }
}


// 监控面板状态
const monitorState = {
    executions: [],
    stats: {},
    lastFetchedAt: null,
    pagination: {
        page: 1,
        pageSize: 20,
        total: 0,
        totalPages: 0
    }
};

function openMonitorPanel() {
    const modal = document.getElementById('monitor-modal');
    if (!modal) {
        return;
    }
    modal.style.display = 'block';

    // 重置显示状态
    const statsContainer = document.getElementById('monitor-stats');
    const execContainer = document.getElementById('monitor-executions');
    if (statsContainer) {
        statsContainer.innerHTML = '<div class="monitor-empty">加载中...</div>';
    }
    if (execContainer) {
        execContainer.innerHTML = '<div class="monitor-empty">加载中...</div>';
    }

    const statusFilter = document.getElementById('monitor-status-filter');
    if (statusFilter) {
        statusFilter.value = 'all';
    }

    // 重置分页状态
    monitorState.pagination = {
        page: 1,
        pageSize: 20,
        total: 0,
        totalPages: 0
    };

    refreshMonitorPanel(1);
}

function closeMonitorPanel() {
    const modal = document.getElementById('monitor-modal');
    if (modal) {
        modal.style.display = 'none';
    }
}

async function refreshMonitorPanel(page = null) {
    const statsContainer = document.getElementById('monitor-stats');
    const execContainer = document.getElementById('monitor-executions');

    try {
        // 如果指定了页码，使用指定页码，否则使用当前页码
        const currentPage = page !== null ? page : monitorState.pagination.page;
        const pageSize = monitorState.pagination.pageSize;
        
        const response = await apiFetch(`/api/monitor?page=${currentPage}&page_size=${pageSize}`, { method: 'GET' });
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
        renderMonitorExecutions(monitorState.executions);
        renderMonitorPagination();
    } catch (error) {
        console.error('刷新监控面板失败:', error);
        if (statsContainer) {
            statsContainer.innerHTML = `<div class="monitor-error">无法加载统计信息：${escapeHtml(error.message)}</div>`;
        }
        if (execContainer) {
            execContainer.innerHTML = `<div class="monitor-error">无法加载执行记录：${escapeHtml(error.message)}</div>`;
        }
    }
}

function applyMonitorFilters() {
    const statusFilter = document.getElementById('monitor-status-filter');
    const status = statusFilter ? statusFilter.value : 'all';
    renderMonitorExecutions(monitorState.executions, status);
}

function renderMonitorStats(statsMap = {}, lastFetchedAt = null) {
    const container = document.getElementById('monitor-stats');
    if (!container) {
        return;
    }

    const entries = Object.values(statsMap);
    if (entries.length === 0) {
        container.innerHTML = '<div class="monitor-empty">暂无统计数据</div>';
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
    const lastUpdatedText = lastFetchedAt ? lastFetchedAt.toLocaleString('zh-CN') : 'N/A';
    const lastCallText = totals.lastCallTime ? totals.lastCallTime.toLocaleString('zh-CN') : '暂无调用';

    let html = `
        <div class="monitor-stat-card">
            <h4>总调用次数</h4>
            <div class="monitor-stat-value">${totals.total}</div>
            <div class="monitor-stat-meta">成功 ${totals.success} / 失败 ${totals.failed}</div>
        </div>
        <div class="monitor-stat-card">
            <h4>成功率</h4>
            <div class="monitor-stat-value">${successRate}%</div>
            <div class="monitor-stat-meta">统计自全部工具调用</div>
        </div>
        <div class="monitor-stat-card">
            <h4>最近一次调用</h4>
            <div class="monitor-stat-value" style="font-size:1rem;">${lastCallText}</div>
            <div class="monitor-stat-meta">最后刷新时间：${lastUpdatedText}</div>
        </div>
    `;

    // 显示最多前4个工具的统计（过滤掉 totalCalls 为 0 的工具）
    const topTools = entries
        .filter(tool => (tool.totalCalls || 0) > 0)
        .slice()
        .sort((a, b) => (b.totalCalls || 0) - (a.totalCalls || 0))
        .slice(0, 4);

    topTools.forEach(tool => {
        const toolSuccessRate = tool.totalCalls > 0 ? ((tool.successCalls || 0) / tool.totalCalls * 100).toFixed(1) : '0.0';
        html += `
            <div class="monitor-stat-card">
                <h4>${escapeHtml(tool.toolName || '未知工具')}</h4>
                <div class="monitor-stat-value">${tool.totalCalls || 0}</div>
                <div class="monitor-stat-meta">
                    成功 ${tool.successCalls || 0} / 失败 ${tool.failedCalls || 0} · 成功率 ${toolSuccessRate}%
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
        container.innerHTML = '<div class="monitor-empty">暂无执行记录</div>';
        return;
    }

    const normalizedStatus = statusFilter === 'all' ? null : statusFilter;
    const filtered = normalizedStatus
        ? executions.filter(exec => (exec.status || '').toLowerCase() === normalizedStatus)
        : executions;

    if (filtered.length === 0) {
        container.innerHTML = '<div class="monitor-empty">当前筛选条件下暂无记录</div>';
        return;
    }

    const rows = filtered
        .map(exec => {
            const status = (exec.status || 'unknown').toLowerCase();
            const statusClass = `monitor-status-chip ${status}`;
            const statusLabel = getStatusText(status);
            const startTime = exec.startTime ? new Date(exec.startTime).toLocaleString('zh-CN') : '未知';
            const duration = formatExecutionDuration(exec.startTime, exec.endTime);
            const toolName = escapeHtml(exec.toolName || '未知工具');
            const executionId = escapeHtml(exec.id || '');
            return `
                <tr>
                    <td>${toolName}</td>
                    <td><span class="${statusClass}">${statusLabel}</span></td>
                    <td>${startTime}</td>
                    <td>${duration}</td>
                    <td>
                        <div class="monitor-execution-actions">
                            <button class="btn-secondary" onclick="showMCPDetail('${executionId}')">查看详情</button>
                            <button class="btn-secondary btn-delete" onclick="deleteExecution('${executionId}')" title="删除此执行记录">删除</button>
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
    tableContainer.innerHTML = `
        <table class="monitor-table">
            <thead>
                <tr>
                    <th>工具</th>
                    <th>状态</th>
                    <th>开始时间</th>
                    <th>耗时</th>
                    <th>操作</th>
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
    
    pagination.innerHTML = `
        <div class="pagination-info">
            显示 ${startItem}-${endItem} / 共 ${total} 条记录
        </div>
        <div class="pagination-controls">
            <button class="btn-secondary" onclick="refreshMonitorPanel(1)" ${page === 1 || total === 0 ? 'disabled' : ''}>首页</button>
            <button class="btn-secondary" onclick="refreshMonitorPanel(${page - 1})" ${page === 1 || total === 0 ? 'disabled' : ''}>上一页</button>
            <span class="pagination-page">第 ${page} / ${totalPages || 1} 页</span>
            <button class="btn-secondary" onclick="refreshMonitorPanel(${page + 1})" ${page >= totalPages || total === 0 ? 'disabled' : ''}>下一页</button>
            <button class="btn-secondary" onclick="refreshMonitorPanel(${totalPages || 1})" ${page >= totalPages || total === 0 ? 'disabled' : ''}>末页</button>
        </div>
    `;
    
    container.appendChild(pagination);
}

// 删除执行记录
async function deleteExecution(executionId) {
    if (!executionId) {
        return;
    }
    
    // 确认删除
    if (!confirm('确定要删除此执行记录吗？此操作不可恢复。')) {
        return;
    }
    
    try {
        const response = await apiFetch(`/api/monitor/execution/${executionId}`, {
            method: 'DELETE'
        });
        
        if (!response.ok) {
            const error = await response.json().catch(() => ({}));
            throw new Error(error.error || '删除执行记录失败');
        }
        
        // 删除成功后刷新当前页面
        const currentPage = monitorState.pagination.page;
        await refreshMonitorPanel(currentPage);
        
        alert('执行记录已删除');
    } catch (error) {
        console.error('删除执行记录失败:', error);
        alert('删除执行记录失败: ' + error.message);
    }
}

function formatExecutionDuration(start, end) {
    if (!start) {
        return '未知';
    }
    const startTime = new Date(start);
    const endTime = end ? new Date(end) : new Date();
    if (Number.isNaN(startTime.getTime()) || Number.isNaN(endTime.getTime())) {
        return '未知';
    }
    const diffMs = Math.max(0, endTime - startTime);
    const seconds = Math.floor(diffMs / 1000);
    if (seconds < 60) {
        return `${seconds} 秒`;
    }
    const minutes = Math.floor(seconds / 60);
    if (minutes < 60) {
        const remain = seconds % 60;
        return remain > 0 ? `${minutes} 分 ${remain} 秒` : `${minutes} 分`;
    }
    const hours = Math.floor(minutes / 60);
    const remainMinutes = minutes % 60;
    return remainMinutes > 0 ? `${hours} 小时 ${remainMinutes} 分` : `${hours} 小时`;
}

// ==================== 外部MCP管理 ====================

let currentEditingMCPName = null;

// 加载外部MCP列表
async function loadExternalMCPs() {
    try {
        const response = await apiFetch('/api/external-mcp');
        if (!response.ok) {
            throw new Error('获取外部MCP列表失败');
        }
        
        const data = await response.json();
        renderExternalMCPList(data.servers || {});
        renderExternalMCPStats(data.stats || {});
    } catch (error) {
        console.error('加载外部MCP列表失败:', error);
        const list = document.getElementById('external-mcp-list');
        if (list) {
            list.innerHTML = `<div class="error">加载失败: ${escapeHtml(error.message)}</div>`;
        }
    }
}

// 渲染外部MCP列表
function renderExternalMCPList(servers) {
    const list = document.getElementById('external-mcp-list');
    if (!list) return;
    
    if (Object.keys(servers).length === 0) {
        list.innerHTML = '<div class="empty">📋 暂无外部MCP配置<br><span style="font-size: 0.875rem; margin-top: 8px; display: block;">点击"添加外部MCP"按钮开始配置</span></div>';
        return;
    }
    
    let html = '<div class="external-mcp-items">';
    for (const [name, server] of Object.entries(servers)) {
        const status = server.status || 'disconnected';
        const statusClass = status === 'connected' ? 'status-connected' : 
                           status === 'connecting' ? 'status-connecting' :
                           status === 'error' ? 'status-error' :
                           status === 'disabled' ? 'status-disabled' : 'status-disconnected';
        const statusText = status === 'connected' ? '已连接' : 
                          status === 'connecting' ? '连接中...' :
                          status === 'error' ? '连接失败' :
                          status === 'disabled' ? '已禁用' : '未连接';
        const transport = server.config.transport || (server.config.command ? 'stdio' : 'http');
        const transportIcon = transport === 'stdio' ? '⚙️' : '🌐';
        
        html += `
            <div class="external-mcp-item">
                <div class="external-mcp-item-header">
                    <div class="external-mcp-item-info">
                        <h4>${transportIcon} ${escapeHtml(name)}${server.tool_count !== undefined && server.tool_count > 0 ? `<span class="tool-count-badge" title="工具数量">🔧 ${server.tool_count}</span>` : ''}</h4>
                        <span class="external-mcp-status ${statusClass}">${statusText}</span>
                    </div>
                    <div class="external-mcp-item-actions">
                        ${status === 'connected' || status === 'disconnected' || status === 'error' ? 
                            `<button class="btn-small" id="btn-toggle-${escapeHtml(name)}" onclick="toggleExternalMCP('${escapeHtml(name)}', '${status}')" title="${status === 'connected' ? '停止连接' : '启动连接'}">
                                ${status === 'connected' ? '⏸ 停止' : '▶ 启动'}
                            </button>` : 
                            status === 'connecting' ? 
                            `<button class="btn-small" id="btn-toggle-${escapeHtml(name)}" disabled style="opacity: 0.6; cursor: not-allowed;">
                                ⏳ 连接中...
                            </button>` : ''}
                        <button class="btn-small" onclick="editExternalMCP('${escapeHtml(name)}')" title="编辑配置" ${status === 'connecting' ? 'disabled' : ''}>✏️ 编辑</button>
                        <button class="btn-small btn-danger" onclick="deleteExternalMCP('${escapeHtml(name)}')" title="删除配置" ${status === 'connecting' ? 'disabled' : ''}>🗑 删除</button>
                    </div>
                </div>
                ${status === 'error' && server.error ? `
                <div class="external-mcp-error" style="margin: 12px 0; padding: 12px; background: #fee; border-left: 3px solid #f44; border-radius: 4px; color: #c33; font-size: 0.875rem;">
                    <strong>❌ 连接错误：</strong>${escapeHtml(server.error)}
                </div>` : ''}
                <div class="external-mcp-item-details">
                    <div>
                        <strong>传输模式</strong>
                        <span>${transportIcon} ${escapeHtml(transport.toUpperCase())}</span>
                    </div>
                    ${server.tool_count !== undefined && server.tool_count > 0 ? `
                    <div>
                        <strong>工具数量</strong>
                        <span style="font-weight: 600; color: var(--accent-color);">🔧 ${server.tool_count} 个工具</span>
                    </div>` : server.tool_count === 0 && status === 'connected' ? `
                    <div>
                        <strong>工具数量</strong>
                        <span style="color: var(--text-muted);">暂无工具</span>
                    </div>` : ''}
                    ${server.config.description ? `
                    <div>
                        <strong>描述</strong>
                        <span>${escapeHtml(server.config.description)}</span>
                    </div>` : ''}
                    ${server.config.timeout ? `
                    <div>
                        <strong>超时时间</strong>
                        <span>${server.config.timeout} 秒</span>
                    </div>` : ''}
                    ${transport === 'stdio' && server.config.command ? `
                    <div>
                        <strong>命令</strong>
                        <span style="font-family: monospace; font-size: 0.8125rem;">${escapeHtml(server.config.command)}</span>
                    </div>` : ''}
                    ${transport === 'http' && server.config.url ? `
                    <div>
                        <strong>URL</strong>
                        <span style="font-family: monospace; font-size: 0.8125rem; word-break: break-all;">${escapeHtml(server.config.url)}</span>
                    </div>` : ''}
                </div>
            </div>
        `;
    }
    html += '</div>';
    list.innerHTML = html;
}

// 渲染外部MCP统计信息
function renderExternalMCPStats(stats) {
    const statsEl = document.getElementById('external-mcp-stats');
    if (!statsEl) return;
    
    const total = stats.total || 0;
    const enabled = stats.enabled || 0;
    const disabled = stats.disabled || 0;
    const connected = stats.connected || 0;
    
    statsEl.innerHTML = `
        <span title="总配置数">📊 总数: <strong>${total}</strong></span>
        <span title="已启用的配置数">✅ 已启用: <strong>${enabled}</strong></span>
        <span title="已停用的配置数">⏸ 已停用: <strong>${disabled}</strong></span>
        <span title="当前已连接的配置数">🔗 已连接: <strong>${connected}</strong></span>
    `;
}

// 显示添加外部MCP模态框
function showAddExternalMCPModal() {
    currentEditingMCPName = null;
    document.getElementById('external-mcp-modal-title').textContent = '添加外部MCP';
    document.getElementById('external-mcp-json').value = '';
    document.getElementById('external-mcp-json-error').style.display = 'none';
    document.getElementById('external-mcp-json-error').textContent = '';
    document.getElementById('external-mcp-json').classList.remove('error');
    document.getElementById('external-mcp-modal').style.display = 'block';
}

// 关闭外部MCP模态框
function closeExternalMCPModal() {
    document.getElementById('external-mcp-modal').style.display = 'none';
    currentEditingMCPName = null;
}

// 编辑外部MCP
async function editExternalMCP(name) {
    try {
        const response = await apiFetch(`/api/external-mcp/${encodeURIComponent(name)}`);
        if (!response.ok) {
            throw new Error('获取外部MCP配置失败');
        }
        
        const server = await response.json();
        currentEditingMCPName = name;
        
        document.getElementById('external-mcp-modal-title').textContent = '编辑外部MCP';
        
        // 将配置转换为对象格式（key为名称）
        const config = { ...server.config };
        // 移除tool_count、external_mcp_enable等前端字段，但保留enabled/disabled用于向后兼容
        delete config.tool_count;
        delete config.external_mcp_enable;
        
        // 包装成对象格式：{ "name": { config } }
        const configObj = {};
        configObj[name] = config;
        
        // 格式化JSON
        const jsonStr = JSON.stringify(configObj, null, 2);
        document.getElementById('external-mcp-json').value = jsonStr;
        document.getElementById('external-mcp-json-error').style.display = 'none';
        document.getElementById('external-mcp-json-error').textContent = '';
        document.getElementById('external-mcp-json').classList.remove('error');
        
        document.getElementById('external-mcp-modal').style.display = 'block';
    } catch (error) {
        console.error('编辑外部MCP失败:', error);
        alert('编辑失败: ' + error.message);
    }
}

// 格式化JSON
function formatExternalMCPJSON() {
    const jsonTextarea = document.getElementById('external-mcp-json');
    const errorDiv = document.getElementById('external-mcp-json-error');
    
    try {
        const jsonStr = jsonTextarea.value.trim();
        if (!jsonStr) {
            errorDiv.textContent = 'JSON不能为空';
            errorDiv.style.display = 'block';
            jsonTextarea.classList.add('error');
            return;
        }
        
        const parsed = JSON.parse(jsonStr);
        const formatted = JSON.stringify(parsed, null, 2);
        jsonTextarea.value = formatted;
        errorDiv.style.display = 'none';
        jsonTextarea.classList.remove('error');
    } catch (error) {
        errorDiv.textContent = 'JSON格式错误: ' + error.message;
        errorDiv.style.display = 'block';
        jsonTextarea.classList.add('error');
    }
}

// 加载示例
function loadExternalMCPExample() {
    const example = {
        "hexstrike-ai": {
            command: "python3",
            args: [
                "/path/to/script.py",
                "--server",
                "http://example.com"
            ],
            description: "示例描述",
            timeout: 300
        }
    };
    
    document.getElementById('external-mcp-json').value = JSON.stringify(example, null, 2);
    document.getElementById('external-mcp-json-error').style.display = 'none';
    document.getElementById('external-mcp-json').classList.remove('error');
}

// 保存外部MCP
async function saveExternalMCP() {
    const jsonTextarea = document.getElementById('external-mcp-json');
    const jsonStr = jsonTextarea.value.trim();
    const errorDiv = document.getElementById('external-mcp-json-error');
    
    if (!jsonStr) {
        errorDiv.textContent = 'JSON配置不能为空';
        errorDiv.style.display = 'block';
        jsonTextarea.classList.add('error');
        jsonTextarea.focus();
        return;
    }
    
    let configObj;
    try {
        configObj = JSON.parse(jsonStr);
    } catch (error) {
        errorDiv.textContent = 'JSON格式错误: ' + error.message;
        errorDiv.style.display = 'block';
        jsonTextarea.classList.add('error');
        jsonTextarea.focus();
        return;
    }
    
    // 验证必须是对象格式
    if (typeof configObj !== 'object' || Array.isArray(configObj) || configObj === null) {
        errorDiv.textContent = '配置错误: 必须是JSON对象格式，key为配置名称，value为配置内容';
        errorDiv.style.display = 'block';
        jsonTextarea.classList.add('error');
        return;
    }
    
    // 获取所有配置名称
    const names = Object.keys(configObj);
    if (names.length === 0) {
        errorDiv.textContent = '配置错误: 至少需要一个配置项';
        errorDiv.style.display = 'block';
        jsonTextarea.classList.add('error');
        return;
    }
    
    // 验证每个配置
    for (const name of names) {
        if (!name || name.trim() === '') {
            errorDiv.textContent = '配置错误: 配置名称不能为空';
            errorDiv.style.display = 'block';
            jsonTextarea.classList.add('error');
            return;
        }
        
        const config = configObj[name];
        if (typeof config !== 'object' || Array.isArray(config) || config === null) {
            errorDiv.textContent = `配置错误: "${name}" 的配置必须是对象`;
            errorDiv.style.display = 'block';
            jsonTextarea.classList.add('error');
            return;
        }
        
        // 移除 external_mcp_enable 字段（由按钮控制，但保留 enabled/disabled 用于向后兼容）
        delete config.external_mcp_enable;
        
        // 验证配置内容
        const transport = config.transport || (config.command ? 'stdio' : config.url ? 'http' : '');
        if (!transport) {
            errorDiv.textContent = `配置错误: "${name}" 需要指定command（stdio模式）或url（http模式）`;
            errorDiv.style.display = 'block';
            jsonTextarea.classList.add('error');
            return;
        }
        
        if (transport === 'stdio' && !config.command) {
            errorDiv.textContent = `配置错误: "${name}" stdio模式需要command字段`;
            errorDiv.style.display = 'block';
            jsonTextarea.classList.add('error');
            return;
        }
        
        if (transport === 'http' && !config.url) {
            errorDiv.textContent = `配置错误: "${name}" http模式需要url字段`;
            errorDiv.style.display = 'block';
            jsonTextarea.classList.add('error');
            return;
        }
    }
    
    // 清除错误提示
    errorDiv.style.display = 'none';
    jsonTextarea.classList.remove('error');
    
    try {
        // 如果是编辑模式，只更新当前编辑的配置
        if (currentEditingMCPName) {
            if (!configObj[currentEditingMCPName]) {
                errorDiv.textContent = `配置错误: 编辑模式下，JSON必须包含配置名称 "${currentEditingMCPName}"`;
                errorDiv.style.display = 'block';
                jsonTextarea.classList.add('error');
                return;
            }
            
            const response = await apiFetch(`/api/external-mcp/${encodeURIComponent(currentEditingMCPName)}`, {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ config: configObj[currentEditingMCPName] }),
            });
            
            if (!response.ok) {
                const error = await response.json();
                throw new Error(error.error || '保存失败');
            }
        } else {
            // 添加模式：保存所有配置
            for (const name of names) {
                const config = configObj[name];
                const response = await apiFetch(`/api/external-mcp/${encodeURIComponent(name)}`, {
                    method: 'PUT',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify({ config }),
                });
                
                if (!response.ok) {
                    const error = await response.json();
                    throw new Error(`保存 "${name}" 失败: ${error.error || '未知错误'}`);
                }
            }
        }
        
        closeExternalMCPModal();
        await loadExternalMCPs();
        alert('保存成功');
    } catch (error) {
        console.error('保存外部MCP失败:', error);
        errorDiv.textContent = '保存失败: ' + error.message;
        errorDiv.style.display = 'block';
        jsonTextarea.classList.add('error');
    }
}

// 删除外部MCP
async function deleteExternalMCP(name) {
    if (!confirm(`确定要删除外部MCP "${name}" 吗？`)) {
        return;
    }
    
    try {
        const response = await apiFetch(`/api/external-mcp/${encodeURIComponent(name)}`, {
            method: 'DELETE',
        });
        
        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || '删除失败');
        }
        
        await loadExternalMCPs();
        alert('删除成功');
    } catch (error) {
        console.error('删除外部MCP失败:', error);
        alert('删除失败: ' + error.message);
    }
}

// 切换外部MCP启停
async function toggleExternalMCP(name, currentStatus) {
    const action = currentStatus === 'connected' ? 'stop' : 'start';
    const buttonId = `btn-toggle-${name}`;
    const button = document.getElementById(buttonId);
    
    // 如果是启动操作，显示加载状态
    if (action === 'start' && button) {
        button.disabled = true;
        button.style.opacity = '0.6';
        button.style.cursor = 'not-allowed';
        button.innerHTML = '⏳ 连接中...';
    }
    
    try {
        const response = await apiFetch(`/api/external-mcp/${encodeURIComponent(name)}/${action}`, {
            method: 'POST',
        });
        
        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || '操作失败');
        }
        
        const result = await response.json();
        
        // 如果是启动操作，先立即检查一次状态
        if (action === 'start') {
            // 立即检查一次状态（可能已经连接）
            try {
                const statusResponse = await apiFetch(`/api/external-mcp/${encodeURIComponent(name)}`);
                if (statusResponse.ok) {
                    const statusData = await statusResponse.json();
                    const status = statusData.status || 'disconnected';
                    
                    if (status === 'connected') {
                        // 已经连接，立即刷新
                        await loadExternalMCPs();
                        return;
                    }
                }
            } catch (error) {
                console.error('检查状态失败:', error);
            }
            
            // 如果还未连接，开始轮询
            await pollExternalMCPStatus(name, 30); // 最多轮询30次（约30秒）
        } else {
            // 停止操作，直接刷新
            await loadExternalMCPs();
        }
    } catch (error) {
        console.error('切换外部MCP状态失败:', error);
        alert('操作失败: ' + error.message);
        
        // 恢复按钮状态
        if (button) {
            button.disabled = false;
            button.style.opacity = '1';
            button.style.cursor = 'pointer';
            button.innerHTML = '▶ 启动';
        }
        
        // 刷新状态
        await loadExternalMCPs();
    }
}

// 轮询外部MCP状态
async function pollExternalMCPStatus(name, maxAttempts = 30) {
    let attempts = 0;
    const pollInterval = 1000; // 1秒轮询一次
    
    while (attempts < maxAttempts) {
        await new Promise(resolve => setTimeout(resolve, pollInterval));
        
        try {
            const response = await apiFetch(`/api/external-mcp/${encodeURIComponent(name)}`);
            if (response.ok) {
                const data = await response.json();
                const status = data.status || 'disconnected';
                
                // 更新按钮状态
                const buttonId = `btn-toggle-${name}`;
                const button = document.getElementById(buttonId);
                
                if (status === 'connected') {
                    // 连接成功，刷新列表
                    await loadExternalMCPs();
                    return;
                } else if (status === 'error' || status === 'disconnected') {
                    // 连接失败，刷新列表并显示错误
                    await loadExternalMCPs();
                    if (status === 'error') {
                        alert('连接失败，请检查配置和网络连接');
                    }
                    return;
                } else if (status === 'connecting') {
                    // 仍在连接中，继续轮询
                    attempts++;
                    continue;
                }
            }
        } catch (error) {
            console.error('轮询状态失败:', error);
        }
        
        attempts++;
    }
    
    // 超时，刷新列表
    await loadExternalMCPs();
    alert('连接超时，请检查配置和网络连接');
}

// 在打开设置时加载外部MCP列表
const originalOpenSettings = openSettings;
openSettings = async function() {
    await originalOpenSettings();
    await loadExternalMCPs();
};

// ==================== 攻击链可视化功能 ====================

let attackChainCytoscape = null;
let currentAttackChainConversationId = null;
let isAttackChainLoading = false; // 防止重复加载

// 添加攻击链按钮
function addAttackChainButton(conversationId) {
    const attackChainBtn = document.getElementById('attack-chain-btn');
    if (!attackChainBtn) {
        return;
    }

    if (conversationId) {
        attackChainBtn.disabled = false;
        attackChainBtn.title = '查看当前对话的攻击链';
        attackChainBtn.onclick = () => showAttackChain(conversationId);
    } else {
        attackChainBtn.disabled = true;
        attackChainBtn.title = '请选择一个对话以查看攻击链';
        attackChainBtn.onclick = null;
    }
}

// 显示攻击链模态框
async function showAttackChain(conversationId) {
    // 防止重复点击
    if (isAttackChainLoading) {
        console.log('攻击链正在加载中，请稍候...');
        return;
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
    if (isAttackChainLoading) {
        return; // 防止重复调用
    }
    
    isAttackChainLoading = true;
    
    try {
        const response = await apiFetch(`/api/attack-chain/${conversationId}`);
        
        if (!response.ok) {
            // 处理 409 Conflict（正在生成中）
            if (response.status === 409) {
                const error = await response.json();
                const container = document.getElementById('attack-chain-container');
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
                // 5秒后自动刷新（允许刷新，但保持加载状态防止重复点击）
                setTimeout(() => {
                    refreshAttackChain();
                }, 5000);
                // 在 409 情况下，保持 isAttackChainLoading = true，防止重复点击
                // 但允许 refreshAttackChain 调用 loadAttackChain 来检查状态
                // 注意：不重置 isAttackChainLoading，保持加载状态
                // 恢复按钮状态（虽然保持加载状态，但允许用户手动刷新）
                const regenerateBtn = document.querySelector('button[onclick="regenerateAttackChain()"]');
                if (regenerateBtn) {
                    regenerateBtn.disabled = false;
                    regenerateBtn.style.opacity = '1';
                    regenerateBtn.style.cursor = 'pointer';
                }
                return; // 提前返回，不执行 finally 块中的 isAttackChainLoading = false
            }
            
            const error = await response.json();
            throw new Error(error.error || '加载攻击链失败');
        }
        
        const chainData = await response.json();
        
        // 渲染攻击链
        renderAttackChain(chainData);
        
        // 更新统计信息
        updateAttackChainStats(chainData);
        
        // 成功加载后，重置加载状态
        isAttackChainLoading = false;
        
    } catch (error) {
        console.error('加载攻击链失败:', error);
        const container = document.getElementById('attack-chain-container');
        if (container) {
            container.innerHTML = `<div class="error-message">加载失败: ${error.message}</div>`;
        }
        // 错误时也重置加载状态
        isAttackChainLoading = false;
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
    const isComplexGraph = nodeCount > 20 || edgeCount > 30;
    
    // 准备Cytoscape数据
    const elements = [];
    
    // 添加节点，并预计算文字颜色和边框颜色
    chainData.nodes.forEach(node => {
        const riskScore = node.risk_score || 0;
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
        
        elements.push({
            data: {
                id: node.id,
                label: node.label,
                type: node.type,
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
    
    // 添加边
    chainData.edges.forEach(edge => {
        elements.push({
            data: {
                id: edge.id,
                source: edge.source,
                target: edge.target,
                type: edge.type || 'leads_to',
                weight: edge.weight || 1
            }
        });
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
                    // 统一节点大小，减少布局混乱（根据复杂度调整）
                    'width': nodeCount > 20 ? 60 : 'mapData(riskScore, 0, 100, 45, 75)',
                    'height': nodeCount > 20 ? 60 : 'mapData(riskScore, 0, 100, 45, 75)',
                    'shape': function(ele) {
                        const type = ele.data('type');
                        if (type === 'vulnerability') return 'diamond';
                        if (type === 'action') return 'round-rectangle';
                        if (type === 'target') return 'star';
                        return 'ellipse';
                    },
                    'background-color': function(ele) {
                        const riskScore = ele.data('riskScore') || 0;
                        if (riskScore >= 80) return '#ff4444';  // 红色
                        if (riskScore >= 60) return '#ff8800';  // 橙色
                        if (riskScore >= 40) return '#ffbb00';  // 黄色
                        return '#88cc00';  // 绿色
                    },
                    // 使用预计算的颜色数据
                    'color': 'data(textColor)',
                    'font-size': nodeCount > 20 ? '11px' : '12px',  // 复杂图使用更小字体
                    'font-weight': 'bold',
                    'text-valign': 'center',
                    'text-halign': 'center',
                    'text-wrap': 'wrap',
                    'text-max-width': nodeCount > 20 ? '80px' : '100px',  // 复杂图限制文本宽度
                    'border-width': 2,
                    'border-color': 'data(borderColor)',
                    'overlay-padding': '4px',
                    'text-outline-width': 'data(textOutlineWidth)',
                    'text-outline-color': 'data(textOutlineColor)'
                }
            },
            {
                selector: 'edge',
                style: {
                    'width': 'mapData(weight, 1, 5, 1.5, 3)',
                    'line-color': function(ele) {
                        const type = ele.data('type');
                        if (type === 'discovers') return '#3498db';  // 浅蓝：action发现vulnerability
                        if (type === 'targets') return '#0066ff';  // 蓝色：target指向action
                        if (type === 'enables') return '#e74c3c';  // 深红：vulnerability间的因果关系
                        if (type === 'leads_to') return '#666';  // 灰色：action之间的逻辑顺序
                        return '#999';
                    },
                    'target-arrow-color': function(ele) {
                        const type = ele.data('type');
                        if (type === 'discovers') return '#3498db';
                        if (type === 'targets') return '#0066ff';
                        if (type === 'enables') return '#e74c3c';
                        if (type === 'leads_to') return '#666';
                        return '#999';
                    },
                    'target-arrow-shape': 'triangle',
                    'target-arrow-size': 8,
                    // 对于复杂图，使用straight样式减少交叉；简单图使用bezier更美观
                    'curve-style': isComplexGraph ? 'straight' : 'bezier',
                    'control-point-step-size': isComplexGraph ? 40 : 60,  // bezier控制点间距
                    'control-point-distance': isComplexGraph ? 30 : 50,   // bezier控制点距离
                    'opacity': isComplexGraph ? 0.5 : 0.7,  // 复杂图降低不透明度，减少视觉混乱
                    'line-style': 'solid'
                }
            },
            {
                selector: 'node:selected',
                style: {
                    'border-width': 4,
                    'border-color': '#0066ff'
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
        spacingFactor: isComplexGraph ? 2.5 : 2.0,
        padding: 30
    };
    
    if (typeof cytoscape !== 'undefined' && typeof cytoscapeDagre !== 'undefined') {
        try {
            cytoscape.use(cytoscapeDagre);
            layoutName = 'dagre';
            // 根据图的复杂度调整布局参数
            layoutOptions = {
                name: 'dagre',
                rankDir: 'TB',  // 从上到下
                spacingFactor: isComplexGraph ? 2.5 : 2.0,  // 增加整体间距
                nodeSep: isComplexGraph ? 80 : 60,  // 增加节点间距
                edgeSep: isComplexGraph ? 40 : 30,  // 增加边间距
                rankSep: isComplexGraph ? 120 : 100,  // 增加层级间距
                nodeDimensionsIncludeLabels: true,  // 考虑标签大小
                animate: false,
                padding: 40  // 增加边距
            };
        } catch (e) {
            console.warn('dagre布局注册失败，使用默认布局:', e);
        }
    } else {
        console.warn('dagre布局插件未加载，使用默认布局');
    }
    
    // 应用布局
    attackChainCytoscape.layout(layoutOptions).run();
    
    // 布局完成后，调整视图以适应所有节点
    attackChainCytoscape.fit(undefined, 50);  // 50px padding
    
    // 添加点击事件
    attackChainCytoscape.on('tap', 'node', function(evt) {
        const node = evt.target;
        showNodeDetails(node.data());
    });
    
    // 添加悬停效果
    attackChainCytoscape.on('mouseover', 'node', function(evt) {
        const node = evt.target;
        node.style('opacity', 0.8);
    });
    
    attackChainCytoscape.on('mouseout', 'node', function(evt) {
        const node = evt.target;
        node.style('opacity', 1);
    });
}

// 显示节点详情
function showNodeDetails(nodeData) {
    const detailsPanel = document.getElementById('attack-chain-details');
    const detailsContent = document.getElementById('attack-chain-details-content');
    
    if (!detailsPanel || !detailsContent) {
        return;
    }
    
    detailsPanel.style.display = 'block';
    
    let html = `
        <div class="node-detail-item">
            <strong>节点ID:</strong> <code>${nodeData.id}</code>
        </div>
        <div class="node-detail-item">
            <strong>类型:</strong> ${getNodeTypeLabel(nodeData.type)}
        </div>
        <div class="node-detail-item">
            <strong>标签:</strong> ${escapeHtml(nodeData.label)}
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
    
    if (nodeData.metadata && Object.keys(nodeData.metadata).length > 0) {
        html += `
            <div class="node-detail-item">
                <strong>完整元数据:</strong>
                <pre class="metadata-pre">${JSON.stringify(nodeData.metadata, null, 2)}</pre>
            </div>
        `;
    }
    
    detailsContent.innerHTML = html;
}

// 转义HTML
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
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

// 关闭攻击链模态框
function closeAttackChainModal() {
    const modal = document.getElementById('attack-chain-modal');
    if (modal) {
        modal.style.display = 'none';
    }
    
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
        const wasLoading = isAttackChainLoading;
        isAttackChainLoading = false; // 临时重置，允许刷新
        loadAttackChain(currentAttackChainConversationId).finally(() => {
            // 如果之前正在加载（409 情况），恢复加载状态
            // 否则保持 false（正常完成）
            if (wasLoading) {
                // 检查是否仍然需要保持加载状态（如果还是 409，会在 loadAttackChain 中处理）
                // 这里我们假设如果成功加载，则重置状态
                // 如果还是 409，loadAttackChain 会保持 isAttackChainLoading = true
            }
        });
    }
}

// 重新生成攻击链
async function regenerateAttackChain() {
    if (!currentAttackChainConversationId) {
        return;
    }
    
    // 防止重复点击
    if (isAttackChainLoading) {
        console.log('攻击链正在生成中，请稍候...');
        return;
    }
    
    isAttackChainLoading = true;
    
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
        const response = await apiFetch(`/api/attack-chain/${currentAttackChainConversationId}/regenerate`, {
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
                setTimeout(() => {
                    if (isAttackChainLoading) {
                        refreshAttackChain();
                    }
                }, 5000);
                return;
            }
            
            const error = await response.json();
            throw new Error(error.error || '重新生成攻击链失败');
        }
        
        const chainData = await response.json();
        
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
        isAttackChainLoading = false;
        
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
                        const sourcePos = edge.source().position();
                        const targetPos = edge.target().position();
                        minX = Math.min(minX, sourcePos.x, targetPos.x);
                        minY = Math.min(minY, sourcePos.y, targetPos.y);
                        maxX = Math.max(maxX, sourcePos.x, targetPos.x);
                        maxY = Math.max(maxY, sourcePos.y, targetPos.y);
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
                        const sourcePos = edge.source().position();
                        const targetPos = edge.target().position();
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
                        const label = (nodeData.label || nodeData.id || '').toString();
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
