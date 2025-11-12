
// 当前对话ID
let currentConversationId = null;
// 进度ID与任务信息映射
const progressTaskState = new Map();
// 活跃任务刷新定时器
let activeTaskInterval = null;
const ACTIVE_TASK_REFRESH_INTERVAL = 20000;

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
    const response = await fetch('/api/agent-loop/cancel', {
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
        const response = await fetch('/api/agent-loop/stream', {
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
    const detailsId = 'process-details-' + assistantMessageId;
    const detailsContainer = document.getElementById(detailsId);
    if (detailsContainer) {
        const timeline = detailsContainer.querySelector('.progress-timeline');
        if (timeline && timeline.classList.contains('expanded')) {
            timeline.classList.remove('expanded');
            const btn = document.querySelector(`#${assistantMessageId} .process-detail-btn`);
            if (btn) {
                btn.innerHTML = '<span>展开详情</span>';
            }
        }
    }
    
    // 折叠独立的详情组件（通过convertProgressToDetails创建的）
    // 查找所有以details-开头的详情组件
    const allDetails = document.querySelectorAll('[id^="details-"]');
    allDetails.forEach(detail => {
        const timeline = detail.querySelector('.progress-timeline');
        const toggleBtn = detail.querySelector('.progress-toggle');
        if (timeline && timeline.classList.contains('expanded')) {
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
        if (progressTimeline && progressTimeline.classList.contains('expanded')) {
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
    
    // 设置详情内容（默认折叠状态）
    detailsContainer.innerHTML = `
        <div class="process-details-content">
            ${hasContent ? `<div class="progress-timeline" id="${detailsId}-timeline">${timelineHTML}</div>` : '<div class="progress-timeline-empty">暂无过程详情</div>'}
        </div>
    `;
    
    // 确保初始状态是折叠的
    if (hasContent) {
        const timeline = document.getElementById(detailsId + '-timeline');
        if (timeline) {
            timeline.classList.remove('expanded');
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
    
    // 总是显示详情组件，即使没有内容也显示
    bubble.innerHTML = `
        <div class="progress-header">
            <span class="progress-title">📋 渗透测试详情</span>
            ${hasContent ? `<button class="progress-toggle" onclick="toggleProgressDetails('${detailsId}')">收起详情</button>` : ''}
        </div>
        ${hasContent ? `<div class="progress-timeline expanded" id="${detailsId}-timeline">${timelineHTML}</div>` : '<div class="progress-timeline-empty">暂无过程详情（可能执行过快或未触发详细事件）</div>'}
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
            addTimelineItem(timeline, 'cancelled', {
                title: '⛔ 任务已取消',
                message: event.message,
                data: event.data
            });
            const cancelTitle = document.querySelector(`#${progressId} .progress-title`);
            if (cancelTitle) {
                cancelTitle.textContent = '⛔ 任务已取消';
            }
            finalizeProgressTask(progressId, '已取消');
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
                updateProgressConversation(progressId, event.data.conversationId);
            }
            if (progressTaskState.has(progressId)) {
                finalizeProgressTask(progressId, '已完成');
            }
            loadActiveTasks();
            // 完成时自动折叠所有详情（延迟一下确保response事件已处理）
            setTimeout(() => {
                const assistantIdFromDone = getAssistantId();
                if (assistantIdFromDone) {
                    collapseAllProgressDetails(assistantIdFromDone, progressId);
                } else {
                    // 如果无法获取助手ID，尝试折叠所有详情
                    collapseAllProgressDetails(null, progressId);
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
            <span class="timeline-item-title">${options.title}</span>
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
                        <pre class="tool-args">${JSON.stringify(args, null, 2)}</pre>
                    </div>
                </div>
            </div>
        `;
    } else if (type === 'tool_result' && options.data) {
        const data = options.data;
        const isError = data.isError || !data.success;
        const result = data.result || data.error || '无结果';
        content += `
            <div class="timeline-item-content">
                <div class="tool-result-section ${isError ? 'error' : 'success'}">
                    <strong>执行结果:</strong>
                    <pre class="tool-result">${escapeHtml(result)}</pre>
                    ${data.executionId ? `<div class="tool-execution-id">执行ID: <code>${data.executionId}</code></div>` : ''}
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
    
    // 解析 Markdown 格式
    let formattedContent;
    if (typeof marked !== 'undefined') {
        // 使用 marked.js 解析 Markdown
        try {
            // 配置 marked 选项
            marked.setOptions({
                breaks: true,  // 支持换行
                gfm: true,     // 支持 GitHub Flavored Markdown
            });
            formattedContent = marked.parse(content);
        } catch (e) {
            console.error('Markdown 解析失败:', e);
            // 降级处理：转义 HTML 并保留换行
            formattedContent = escapeHtml(content).replace(/\n/g, '<br>');
        }
    } else {
        // 如果没有 marked.js，使用简单处理
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
        }
        
        addTimelineItem(timeline, eventType, {
            title: itemTitle,
            message: detail.message || '',
            data: data
        });
    });
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
        const response = await fetch(`/api/monitor/execution/${executionId}`);
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

// 点击模态框外部关闭
window.onclick = function(event) {
    const modal = document.getElementById('mcp-detail-modal');
    if (event.target == modal) {
        closeMCPDetail();
    }
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
    if (typeof marked !== 'undefined') {
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
    updateActiveConversation();
    // 刷新对话列表，确保显示最新的历史对话
    loadConversations();
}

// 加载对话列表
async function loadConversations() {
    try {
        const response = await fetch('/api/conversations?limit=50');
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
        const response = await fetch(`/api/conversations/${conversationId}`);
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
                const messageId = addMessage(msg.role, msg.content, msg.mcpExecutionIds || []);
                // 如果有过程详情，显示它们
                if (msg.processDetails && msg.processDetails.length > 0 && msg.role === 'assistant') {
                    // 延迟一下，确保消息已经渲染
                    setTimeout(() => {
                        renderProcessDetails(messageId, msg.processDetails);
                    }, 100);
                }
            });
        } else {
            addMessage('assistant', '系统已就绪。请输入您的测试需求，系统将自动执行相应的安全测试。');
        }
        
        // 滚动到底部
        messagesDiv.scrollTop = messagesDiv.scrollHeight;
        
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
        const response = await fetch(`/api/conversations/${conversationId}`, {
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
        const response = await fetch('/api/agent-loop/tasks');
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

        item.innerHTML = `
            <div class="active-task-info">
                <span class="active-task-status">${task.status === 'cancelling' ? '取消中' : '执行中'}</span>
                <span class="active-task-message">${escapeHtml(task.message || '未命名任务')}</span>
            </div>
            <div class="active-task-actions">
                ${timeText ? `<span class="active-task-time">${timeText}</span>` : ''}
                <button class="active-task-cancel">停止任务</button>
            </div>
        `;

        const cancelBtn = item.querySelector('.active-task-cancel');
        cancelBtn.onclick = () => cancelActiveTask(task.conversationId, cancelBtn);
        if (task.status === 'cancelling') {
            cancelBtn.disabled = true;
            cancelBtn.textContent = '取消中...';
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

// 页面加载时初始化
document.addEventListener('DOMContentLoaded', function() {
    // 加载对话列表
    loadConversations();
    
    // 初始化 textarea 高度
    const chatInput = document.getElementById('chat-input');
    if (chatInput) {
        chatInput.style.height = '44px';
    }
    
    // 添加欢迎消息
    addMessage('assistant', '系统已就绪。请输入您的测试需求，系统将自动执行相应的安全测试。');
    
    loadActiveTasks(true);
    if (activeTaskInterval) {
        clearInterval(activeTaskInterval);
    }
    activeTaskInterval = setInterval(() => loadActiveTasks(), ACTIVE_TASK_REFRESH_INTERVAL);
});

