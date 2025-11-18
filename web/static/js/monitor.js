const progressTaskState = new Map();
let activeTaskInterval = null;
const ACTIVE_TASK_REFRESH_INTERVAL = 10000; // 10秒检查一次

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
