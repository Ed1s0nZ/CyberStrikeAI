const progressTaskState = new Map();
/** @type {{ progressId: string, conversationId: string } | null} */
let userInterruptModalPending = null;
let activeTaskInterval = null;
const ACTIVE_TASK_REFRESH_INTERVAL = 10000; // 10秒检查一次
const TASK_FINAL_STATUSES = new Set(['failed', 'timeout', 'cancelled', 'completed']);

/**
 * 主对话 POST 流仍在读取时，禁止再挂 task-events 补流，否则同一事件会画两遍（与 HITL 是否开启无关）。
 * window.__csAgentLiveStream 由 chat.js sendMessage 在读到 body 后设置，在 finally 中清除。
 */
function syncAgentLiveStreamConversationId(cid) {
    if (!cid) return;
    try {
        const live = window.__csAgentLiveStream;
        if (live && live.active) {
            live.conversationId = cid;
        }
    } catch (e) { /* ignore */ }
}

function shouldSkipTaskEventReplayAttach(conversationId) {
    try {
        const live = window.__csAgentLiveStream;
        if (!live || !live.active || !live.progressId) return false;
        if (!document.getElementById(live.progressId)) return false;
        // 新会话：conversation 事件尚未到达前 conversationId 可能仍为 null，一律不补挂
        if (live.conversationId == null) return true;
        return live.conversationId === conversationId;
    } catch (e) {
        return false;
    }
}
/** 监控页展示：内部 mcp::tool → 模型侧 mcp__tool */
function formatMonitorToolName(name) {
    if (!name || typeof name !== 'string') return name || '';
    return name.includes('::') ? name.replace('::', '__') : name;
}

/** 筛选/API：mcp__tool → 内部 mcp::tool（与库存一致） */
function canonicalMonitorToolName(name) {
    if (!name || typeof name !== 'string') return name || '';
    if (name.includes('::')) return name;
    const idx = name.indexOf('__');
    if (idx > 0) return `${name.slice(0, idx)}::${name.slice(idx + 2)}`;
    return name;
}

function monitorToolNamesEqual(a, b) {
    return canonicalMonitorToolName(a) === canonicalMonitorToolName(b);
}

if (typeof window !== 'undefined') {
    window.shouldSkipTaskEventReplayAttach = shouldSkipTaskEventReplayAttach;
}

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
/** Plan-Execute：将 Eino 内部 agent 名本地化为进度条标题用语 */
function translatePlanExecuteAgentName(name) {
    const n = String(name || '').trim().toLowerCase();
    if (n === 'planner') return typeof window.t === 'function' ? window.t('progress.peAgentPlanner') : '规划器';
    if (n === 'executor') return typeof window.t === 'function' ? window.t('progress.peAgentExecutor') : '执行器';
    if (n === 'replanner' || n === 'execute_replan' || n === 'plan_execute_replan') {
        return typeof window.t === 'function' ? window.t('progress.peAgentReplanning') : '重规划';
    }
    return String(name || '').trim();
}

/** 从 Plan-Execute 模型返回的单层 JSON 中取面向用户的字符串（replanner 常用 response）。 */
function pickPeJSONUserText(o) {
    if (!o || typeof o !== 'object') {
        return '';
    }
    const keys = ['response', 'answer', 'message', 'content', 'summary', 'output', 'text', 'result'];
    for (let i = 0; i < keys.length; i++) {
        const v = o[keys[i]];
        if (typeof v === 'string') {
            const s = v.trim();
            if (s) {
                return s;
            }
        }
    }
    return '';
}

/** 少数模型在 JSON 字符串里仍留下字面量 “\\n”；在已解出正文后再转成换行（不误伤 Windows 盘符时极少命中）。 */
function normalizePeInlineEscapes(s) {
    if (!s || s.indexOf('\\n') < 0) {
        return s;
    }
    return s.replace(/\\n/g, '\n').replace(/\\t/g, '\t');
}

/**
 * Plan-Execute 时间线正文：planner/replanner 的 {"steps":[...]} 转为列表；{"response":"..."} 解包为纯文本；
 * executor 同样解包。流式片段非法 JSON 时保持原文。
 */
function formatTimelineStreamBody(raw, meta) {
    if (!raw || !meta || meta.orchestration !== 'plan_execute') {
        return raw;
    }
    const agent = String(meta.einoAgent || '').trim().toLowerCase();
    const t = String(raw).trim();
    if (t.length < 2 || t.charAt(0) !== '{') {
        return raw;
    }
    try {
        const o = JSON.parse(t);
        if (agent === 'executor') {
            const u = pickPeJSONUserText(o);
            return u ? normalizePeInlineEscapes(u) : raw;
        }
        if (agent === 'planner' || agent === 'replanner' || agent === 'execute_replan' || agent === 'plan_execute_replan') {
            if (o && Array.isArray(o.steps) && o.steps.length) {
                return o.steps.map(function (s, i) {
                    return (i + 1) + '. ' + String(s);
                }).join('\n');
            }
            const u = pickPeJSONUserText(o);
            if (u) {
                return normalizePeInlineEscapes(u);
            }
        }
    } catch (e) {
        return raw;
    }
    return raw;
}

/** 时间线条目：Plan-Execute 主通道流式阶段标题（替代一律「规划中」） */
function einoMainStreamPlanningTitle(responseData) {
    const orch = responseData && responseData.orchestration;
    const agent = responseData && responseData.einoAgent != null ? String(responseData.einoAgent).trim() : '';
    const prefix = timelineAgentBracketPrefix(responseData);
    if (orch === 'plan_execute' && agent) {
        const a = agent.toLowerCase();
        let key = 'chat.planExecuteStreamPhase';
        if (a === 'planner') key = 'chat.planExecuteStreamPlanner';
        else if (a === 'executor') key = 'chat.planExecuteStreamExecutor';
        else if (a === 'replanner' || a === 'execute_replan' || a === 'plan_execute_replan') key = 'chat.planExecuteStreamReplanning';
        const label = typeof window.t === 'function' ? window.t(key) : '输出';
        return prefix + '📝 ' + label;
    }
    // eino_single / deep / supervisor：主通道是模型流式输出，不是「规划」；模型偶发复述工具 stdout 时，旧文案易被误认为工具结果标题。
    if (orch != null && String(orch).trim() !== '' && orch !== 'plan_execute') {
        const streamLabel = typeof window.t === 'function' ? window.t('chat.assistantStreamPhase') : '助手输出';
        return prefix + '📝 ' + streamLabel;
    }
    const plan = typeof window.t === 'function' ? window.t('chat.planning') : '规划中';
    return prefix + '📝 ' + plan;
}

/**
 * Eino 未捕获助手正文占位文案；终态 response 不应覆盖已有流式 buffer。
 */
function isEinoEmptyResponsePlaceholder(text) {
    if (text == null) return false;
    const s = String(text);
    return s.indexOf('no assistant text was captured') !== -1
        || s.indexOf('未捕获到助手文本输出') !== -1;
}

function resolveFinalAssistantResponseText(finalMessage, streamState) {
    const buf = streamState && streamState.buffer != null ? String(streamState.buffer).trim() : '';
    if (isEinoEmptyResponsePlaceholder(finalMessage) && buf) {
        return streamState.buffer;
    }
    return finalMessage;
}

/**
 * 主通道 response 结束时：将流式占位条目固化为 planning（与后端 flushResponsePlan 落库类型一致），
 * 避免 integrateProgressToMCPSection 快照前删除占位导致「助手输出」仅刷新后才出现。
 */
function finalizeMainResponseStreamItem(streamState, finalMessage, responseData) {
    if (!streamState || !streamState.itemId) return false;
    const item = document.getElementById(streamState.itemId);
    if (!item || !item.parentNode) return false;

    const resolved = resolveFinalAssistantResponseText(finalMessage, streamState);
    const fullText = (resolved != null && String(resolved).trim() !== '')
        ? String(resolved)
        : (streamState.buffer || '');
    if (!String(fullText).trim()) {
        item.parentNode.removeChild(item);
        return false;
    }

    const meta = Object.assign({}, streamState.streamMeta || {}, responseData || {});

    item.classList.remove('timeline-item-thinking');
    item.classList.add('timeline-item-planning');
    item.dataset.timelineType = 'planning';
    delete item.dataset.responseStreamPlaceholder;
    if (meta.orchestration != null && String(meta.orchestration).trim() !== '') {
        item.dataset.orchestration = String(meta.orchestration).trim();
    }
    if (meta.einoAgent != null && String(meta.einoAgent).trim() !== '') {
        item.dataset.einoAgent = String(meta.einoAgent).trim();
    }

    const titleEl = item.querySelector('.timeline-item-title');
    if (titleEl && typeof einoMainStreamPlanningTitle === 'function') {
        titleEl.textContent = einoMainStreamPlanningTitle(meta);
    }

    let contentEl = item.querySelector('.timeline-item-content');
    if (!contentEl) {
        contentEl = document.createElement('div');
        contentEl.className = 'timeline-item-content';
        item.appendChild(contentEl);
    }
    flushStreamPlainTextUpdate(contentEl);
    const body = typeof formatTimelineStreamBody === 'function'
        ? formatTimelineStreamBody(fullText, meta)
        : fullText;
    setTimelineItemContentStreamPlain(contentEl, body);
    return true;
}

function translateProgressMessage(message, data) {
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
        '正在启动 Eino 多代理...': 'progress.startingEinoMultiAgent',
        // 英文（与 en-US.json 一致，避免后端/缓存已是英文时无法随语言切换）
        'Calling AI model...': 'progress.callingAI',
        'Last iteration: generating summary and next steps...': 'progress.lastIterSummary',
        'Summary complete': 'progress.summaryDone',
        'Generating final reply...': 'progress.generatingFinalReply',
        'Max iterations reached, generating summary...': 'progress.maxIterSummary',
        'Analyzing your request...': 'progress.analyzingRequestShort',
        'Analyzing your request and planning test strategy...': 'progress.analyzingRequestPlanning',
        'Starting Eino DeepAgent...': 'progress.startingEinoDeepAgent',
        'Starting Eino multi-agent...': 'progress.startingEinoMultiAgent'
    };
    if (map[trim]) return window.t(map[trim]);
    const einoAgentRe = /^\[Eino\]\s*(.+)$/;
    const einoM = trim.match(einoAgentRe);
    if (einoM) {
        let disp = einoM[1];
        if (data && data.orchestration === 'plan_execute') {
            disp = translatePlanExecuteAgentName(disp);
        }
        return window.t('progress.einoAgent', { name: disp });
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
    window.translatePlanExecuteAgentName = translatePlanExecuteAgentName;
    window.einoMainStreamPlanningTitle = einoMainStreamPlanningTitle;
    window.finalizeMainResponseStreamItem = finalizeMainResponseStreamItem;
    window.formatTimelineStreamBody = formatTimelineStreamBody;
}

// 存储工具调用ID到DOM元素的映射，用于更新执行状态。
// 键必须带 progressId 作用域，避免不同任务复用相同 toolCallId 时串线。
const toolCallStatusMap = new Map();

function toolCallMapKey(progressId, toolCallId) {
    return String(progressId) + '::' + String(toolCallId);
}

function getToolCallMapping(progressId, toolCallId) {
    if (!toolCallId) return null;
    const scoped = toolCallStatusMap.get(toolCallMapKey(progressId, toolCallId));
    if (scoped) return scoped;
    // 兼容历史遗留：若 map 中还有旧格式 key（仅 toolCallId），兜底读取。
    return toolCallStatusMap.get(String(toolCallId)) || null;
}

function finalizeOutstandingToolCallsForProgress(progressId, finalStatus) {
    if (!progressId) return;
    const pid = String(progressId);
    for (const [mapKey, mapping] of Array.from(toolCallStatusMap.entries())) {
        if (!mapping) continue;
        if (mapping.progressId != null && String(mapping.progressId) !== pid) continue;
        const tcid = mapping.toolCallId || (String(mapKey).includes('::') ? String(mapKey).split('::').slice(1).join('::') : String(mapKey));
        // 已收到终态结果的调用仅清理索引，不能在任务收尾时被改写成失败。
        if (!mapping.terminalStatus) {
            updateToolCallStatus(mapping.progressId || progressId, tcid, finalStatus);
        }
        toolCallStatusMap.delete(mapKey);
    }
}

// 模型流式输出缓存：progressId -> { assistantId, buffer }
const responseStreamStateByProgressId = new Map();
// 主通道当前迭代轮次缓存：progressId -> { iteration, orchestration }
const mainIterationStateByProgressId = new Map();

/** 工作流多 Agent 节点切换时清空流式聚合，避免推理/输出条目覆盖上一节点内容 */
function clearTimelineStreamStates(progressId) {
    responseStreamStateByProgressId.delete(progressId);
    thinkingStreamStateByProgressId.delete(progressId);
    einoAgentReplyStreamStateByProgressId.delete(progressId);
    const prefix = String(progressId) + '::';
    for (const key of Array.from(toolResultStreamStateByKey.keys())) {
        if (String(key).startsWith(prefix)) {
            toolResultStreamStateByKey.delete(key);
        }
    }
}

/** 同一段主通道流式输出（Eino 可能重复 response_start） */
function sameMainResponseStreamMeta(a, b) {
    if (!a || !b) return false;
    const agentA = String(a.einoAgent != null ? a.einoAgent : '').trim();
    const agentB = String(b.einoAgent != null ? b.einoAgent : '').trim();
    if (!agentA || agentA !== agentB) return false;
    const orchA = String(a.orchestration != null ? a.orchestration : '').trim();
    const orchB = String(b.orchestration != null ? b.orchestration : '').trim();
    if (orchA !== orchB) return false;
    const nodeA = String(a.workflowNodeId != null ? a.workflowNodeId : '').trim();
    const nodeB = String(b.workflowNodeId != null ? b.workflowNodeId : '').trim();
    return nodeA === nodeB;
}

function resolveMainIterationTag(progressId, responseData) {
    const d = responseData || {};
    if (d.iteration != null) {
        return String(d.iteration);
    }
    const cached = mainIterationStateByProgressId.get(String(progressId));
    if (!cached || cached.iteration == null) {
        return '';
    }
    const cachedOrch = String(cached.orchestration != null ? cached.orchestration : '').trim();
    const streamOrch = String(d.orchestration != null ? d.orchestration : '').trim();
    if (cachedOrch && streamOrch && cachedOrch !== streamOrch) {
        return '';
    }
    return String(cached.iteration);
}

function buildMainResponseStreamIdentity(progressId, responseData) {
    const d = responseData || {};
    const agent = String(d.einoAgent != null ? d.einoAgent : '').trim();
    const orch = String(d.orchestration != null ? d.orchestration : '').trim();
    const iterTag = resolveMainIterationTag(progressId, d);
    const nodeId = String(d.workflowNodeId != null ? d.workflowNodeId : '').trim();
    return agent + '|' + orch + '|iter=' + iterTag + '|wfNode=' + nodeId;
}

function extractIterationTagFromStreamIdentity(identity) {
    const s = String(identity || '');
    const idx = s.lastIndexOf('|iter=');
    if (idx < 0) {
        return '';
    }
    return s.slice(idx + 6);
}

/** Plan-Execute 多轮 executor/planner 同名代理：仅在同轮次内复用流式条目 */
function areMainResponseStreamIterationsCompatible(prevIterTag, streamIterTag, orchestration) {
    const orch = String(orchestration != null ? orchestration : '').trim();
    if (orch === 'plan_execute') {
        return prevIterTag === streamIterTag && prevIterTag !== '';
    }
    return !prevIterTag || !streamIterTag || prevIterTag === streamIterTag;
}

/** 仅合并 Eino 对同一段 MessageStream 重复发出的 response_start */
function shouldReuseMainResponseStream(progressId, prevStream, responseData, streamOrch) {
    if (!prevStream || !prevStream.itemId) {
        return false;
    }
    if (!sameMainResponseStreamMeta(prevStream.streamMeta, responseData)) {
        return false;
    }
    const streamId = responseData && responseData.streamId != null ? String(responseData.streamId).trim() : '';
    if (streamId && prevStream.streamId === streamId) {
        return true;
    }
    const orch = String(streamOrch != null ? streamOrch : '').trim();
    if (orch === 'plan_execute') {
        return false;
    }
    const prevIterTag = extractIterationTagFromStreamIdentity(prevStream.streamIdentity || '');
    const streamIterTag = extractIterationTagFromStreamIdentity(
        buildMainResponseStreamIdentity(progressId, responseData)
    );
    return areMainResponseStreamIterationsCompatible(prevIterTag, streamIterTag, orch);
}

// AI 思考流式输出：progressId -> Map(streamId -> { itemId, buffer })
const thinkingStreamStateByProgressId = new Map();

// Eino 子代理回复流式：progressId -> Map(streamId -> { itemId, buffer })
const einoAgentReplyStreamStateByProgressId = new Map();

// 工具输出流式增量：progressId::toolCallId -> { itemId, buffer }
const toolResultStreamStateByKey = new Map();
function toolResultStreamKey(progressId, toolCallId) {
    return String(progressId) + '::' + String(toolCallId);
}

const LIVE_TIMELINE_MAX_ITEMS = 150;
const LIVE_TIMELINE_PRUNE_CHUNK = 50;

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

function escapeHtmlLocal(text) {
    if (!text) return '';
    const div = document.createElement('div');
    div.textContent = String(text);
    return div.innerHTML;
}

function formatTimelinePlainTextHtml(text) {
    return '<pre class="timeline-plain-text">' + escapeHtml(text == null ? '' : String(text)) + '</pre>';
}

/** fenced 块占位（BMP 私用区，正文几乎不会出现） */
const _MD_FENCE_PRE = '\n\uE000CSAI_FENCE_';
const _MD_FENCE_SUF = '_\uE000\n';

function _maskFencedCodeBlocksForMdPreprocess(md) {
    const blocks = [];
    const masked = String(md).replace(/```[\s\S]*?```/g, (m) => {
        const i = blocks.length;
        blocks.push(m);
        return _MD_FENCE_PRE + i + _MD_FENCE_SUF;
    });
    return { masked, blocks };
}

function _unmaskFencedCodeBlocksAfterMdPreprocess(s, blocks) {
    let out = s;
    for (let i = 0; i < blocks.length; i++) {
        out = out.split(_MD_FENCE_PRE + i + _MD_FENCE_SUF).join(blocks[i]);
    }
    return out;
}

/**
 * 模型/网关偶发把「思考」混进正文，用伪 XML 包裹（如 &lt;redacted_thinking&gt;…&lt;/redacted_thinking&gt;）。
 * 与 Markdown 列表混排时，结束标签常被吞进 &lt;li&gt;，其后 **、` 等行内语法全部无法解析；成对块整段移除。
 * @param {string} segment
 * @returns {string}
 */
function _stripXmlReasoningWrappersForMarkdown(segment) {
    let t = String(segment);
    const tags = ['redacted_thinking', 'redacted_reasoning'];
    for (let i = 0; i < tags.length; i++) {
        const name = tags[i];
        const re = new RegExp('<\\s*' + name + '\\b[^>]*>[\\s\\S]*?<\\s*/\\s*' + name + '\\s*>', 'gi');
        t = t.replace(re, '\n\n');
    }
    return t.replace(/\n{3,}/g, '\n\n');
}

/**
 * 解除 LLM 常用的块级 HTML 外壳（`<div>`、`<p>`、`<section>`、`<article>`、`<main>`）。
 * 整段包在块级标签里时，CommonMark 不会在块内再解析 Markdown，导致 **、` 原样显示。
 */
function _unwrapHtmlBlockWrappersForMarkdown(segment) {
    let s = segment;
    let prev;
    for (let i = 0; i < 30 && s !== prev; i++) {
        prev = s;
        s = s.replace(/<div(?:\s[^>]*)?>([\s\S]*?)<\/div>/gi, (_, inner) => String(inner).trim() + '\n\n');
        s = s.replace(/<p(?:\s[^>]*)?>([\s\S]*?)<\/p>/gi, (_, inner) => String(inner).trim() + '\n\n');
        s = s.replace(/<section(?:\s[^>]*)?>([\s\S]*?)<\/section>/gi, (_, inner) => String(inner).trim() + '\n\n');
        s = s.replace(/<article(?:\s[^>]*)?>([\s\S]*?)<\/article>/gi, (_, inner) => String(inner).trim() + '\n\n');
        s = s.replace(/<main(?:\s[^>]*)?>([\s\S]*?)<\/main>/gi, (_, inner) => String(inner).trim() + '\n\n');
        s = s.replace(/\n{3,}/g, '\n\n');
    }
    return s;
}

/**
 * 将 HTML 列表 / 粘连的 `<li>` 还原为 Markdown 列表行，并去掉外层 `<ul>`，便于 marked 解析行内 **、` `
 * @param {string} segment
 * @returns {string}
 */
function _flattenOrphanHtmlLiInMarkdown(segment) {
    let s = segment;
    s = s.replace(/<li(?:\s[^>]*)?>([\s\S]*?)<\/li>/gi, (_, inner) => {
        const body = String(inner).trim().replace(/\s*\n\s*/g, ' ');
        return '- ' + body + '\n';
    });
    s = s.replace(/<\/?ul(?:\s[^>]*)?>/gi, '\n');
    s = s.replace(/<\/?ol(?:\s[^>]*)?>/gi, '\n');
    s = s.replace(/([0-9A-Za-z_\u4e00-\u9fff])\s*<li(?:\s[^>]*)?>\s*/g, (_, ch) => ch + '\n- ');
    return s.replace(/\n{3,}/g, '\n\n');
}

/** 行首 Unicode 项目符号 → Markdown 列表 `- `（模型常用 • 而非 `-`） */
function _normalizeUnicodeBulletMarkersToMdDash(segment) {
    return segment
        .replace(/^\s*\u2022\s+/gm, '- ')
        .replace(/^\s*\u00b7\s+/gm, '- ');
}

/**
 * 修正模型常见的强调语法偏差：
 * 1) 把 `\*\*文本\*\*` 还原为 `**文本**`（常见于多层转义输出）
 * 2) 把 `** 文本 **` 收敛为 `**文本**`（避免分隔符内空格导致不生效）
 * 仅处理单行内容，避免跨段落误匹配。
 */
function _normalizeEmphasisMarkersForMarkdown(segment) {
    const raw = String(segment);
    const maskInlineCode = (input) => {
        const blocks = [];
        const masked = input.replace(/`[^`\n]*`/g, (m) => {
            const token = '__CS_INLINE_CODE_' + blocks.length + '__';
            blocks.push(m);
            return token;
        });
        return { masked, blocks };
    };
    const unmaskInlineCode = (input, blocks) => {
        let out = input;
        for (let i = 0; i < blocks.length; i++) {
            out = out.replace('__CS_INLINE_CODE_' + i + '__', blocks[i]);
        }
        return out;
    };
    const isWordLike = (ch) => /[\u4e00-\u9fffA-Za-z0-9]/.test(ch || '');
    const countUnescapedStrongMarkers = (text) => {
        let count = 0;
        for (let i = 0; i < text.length - 1; i++) {
            if (text.charAt(i) === '*' && text.charAt(i + 1) === '*') {
                if (i > 0 && text.charAt(i - 1) === '\\') {
                    continue;
                }
                count++;
                i++;
            }
        }
        return count;
    };
    const normalizeLine = (line) => {
        let lineWork = line;
        // 奇数个 `**` 往往意味着有一个孤立标记；仅清理「空白夹着的 **」这类高置信噪声。
        while (countUnescapedStrongMarkers(lineWork) % 2 === 1) {
            const next = lineWork.replace(/\s\*\*\s/g, ' ');
            if (next === lineWork) break;
            lineWork = next;
        }
        let out = '';
        let cursor = 0;
        while (cursor < lineWork.length) {
            const open = lineWork.indexOf('**', cursor);
            if (open < 0) {
                out += lineWork.slice(cursor);
                break;
            }
            // 允许 `\*\*text\*\*` 先还原，escaped 星号本身不作为强调标记。
            if (open > 0 && lineWork.charAt(open - 1) === '\\') {
                out += lineWork.slice(cursor, open + 2);
                cursor = open + 2;
                continue;
            }
            let close = open + 2;
            while (true) {
                close = lineWork.indexOf('**', close);
                if (close < 0) break;
                if (close > 0 && lineWork.charAt(close - 1) === '\\') {
                    close += 2;
                    continue;
                }
                break;
            }
            if (close < 0) {
                out += lineWork.slice(cursor);
                break;
            }

            let prefix = lineWork.slice(cursor, open);
            const innerRaw = lineWork.slice(open + 2, close);
            const inner = innerRaw.trim();
            const next = lineWork.charAt(close + 2);
            const prevTail = prefix.charAt(prefix.length - 1);

            // 内部为空时不改写，避免把 `****` 等异常输入改坏。
            if (!inner) {
                out += lineWork.slice(cursor, close + 2);
                cursor = close + 2;
                continue;
            }

            // CJK/字母数字与强调标记紧邻时补边界空格，提升解析稳定性。
            if (isWordLike(prevTail) && !/\s$/.test(prefix)) {
                prefix += ' ';
            }
            out += prefix + '**' + inner + '**';
            if (isWordLike(next)) {
                out += ' ';
            }
            cursor = close + 2;
        }
        return out;
    };

    // 先还原常见 escaped strong，再做成对规范化。
    let s = raw.replace(/\\\*\*([^\n*][^\n]*?[^\n*])\\\*\*/g, '**$1**');
    const masked = maskInlineCode(s);
    s = masked.masked
        .split('\n')
        .map(normalizeLine)
        .join('\n');
    s = unmaskInlineCode(s, masked.blocks);
    return s;
}

/**
 * 解析前归一化助手 Markdown：去掉零宽字符，NFKC 将全角 * ` _ 等转为 ASCII，
 * 避免 marked 无法识别强调/行内代码而原样显示 **、反引号；
 * 并移除 &lt;redacted_thinking&gt; 等伪 XML 思考块、修正块级 HTML（`<div>`/`<p>`/…、`<ul>`/`<li>`）与 Unicode 项目符号 `•`，避免块级 HTML 吞掉 inline 解析。
 * @param {string|null|undefined} text
 * @returns {string}
 */
function normalizeAssistantMarkdownSource(text) {
    if (text == null) return '';
    let s = String(text);
    s = s.replace(/[\u200B-\u200D\u200E\u200F\uFEFF\u2060]/g, '');
    try {
        s = s.normalize('NFKC');
    } catch (e) {
        /* ignore */
    }
    s = _normalizeEmphasisMarkersForMarkdown(s);
    s = _stripXmlReasoningWrappersForMarkdown(s);
    const fb = _maskFencedCodeBlocksForMdPreprocess(s);
    s = _unwrapHtmlBlockWrappersForMarkdown(fb.masked);
    s = _flattenOrphanHtmlLiInMarkdown(s);
    s = _normalizeUnicodeBulletMarkersToMdDash(s);
    s = _unmaskFencedCodeBlocksAfterMdPreprocess(s, fb.blocks);
    return s;
}
if (typeof window !== 'undefined') {
    window.normalizeAssistantMarkdownSource = normalizeAssistantMarkdownSource;
}

/**
 * 与 internal/openai.normalizeStreamingDelta 一致：兼容网关/模型返回「累计全文」或整包重发，
 * 避免前端 buffer += chunk 与后端已归一化的增量叠加导致逐段重复（如「响应中显示了响应中显示了」）。
 * @returns {[string, string]} [nextBuffer, effectiveDelta]
 */
function normalizeStreamingDeltaJs(current, incoming) {
    const cur = current == null ? '' : String(current);
    const inc = incoming == null ? '' : String(incoming);
    if (inc === '') {
        return [cur, ''];
    }
    if (cur === '') {
        return [inc, inc];
    }
    if (inc.startsWith(cur) && inc.length > cur.length) {
        return [inc, inc.slice(cur.length)];
    }
    const runeCount = Array.from(cur).length;
    if (inc === cur && runeCount > 1) {
        return [cur, ''];
    }
    return [cur + inc, inc];
}
if (typeof window !== 'undefined') {
    window.normalizeStreamingDeltaJs = normalizeStreamingDeltaJs;
}

/**
 * SSE data.accumulated：服务端权威流式全文。有则直接用作 buffer，避免双端 normalize 叠字。
 * @param {object|null|undefined} data
 * @returns {string|null} 有快照时返回全文；否则 null（回退 delta 归一化）
 */
function streamBufferFromAccumulated(data) {
    if (!data || data.accumulated == null) {
        return null;
    }
    return String(data.accumulated);
}

/**
 * @returns {string} 合并后的 buffer
 */
function mergeStreamBuffer(current, delta, data) {
    const acc = streamBufferFromAccumulated(data);
    if (acc !== null) {
        return acc;
    }
    return normalizeStreamingDeltaJs(current, delta)[0];
}

if (typeof window !== 'undefined') {
    window.streamBufferFromAccumulated = streamBufferFromAccumulated;
    window.mergeStreamBuffer = mergeStreamBuffer;
    window.processSseDataLinesYielding = processSseDataLinesYielding;
    window.flushStreamPlainTextUpdate = flushStreamPlainTextUpdate;
    window.scheduleStreamPlainTextUpdate = scheduleStreamPlainTextUpdate;
}

/** 流式纯文本 DOM：按帧合并更新，尽量增量 appendData，避免每条 SSE 全量 textContent 阻塞主线程 */
const streamPlainDomState = new WeakMap();
/** 跟踪仍有待刷新的流式节点，便于快照时间线前一次性 flush */
const streamPlainDomPendingElements = new Set();

function applyStreamPlainTextNow(contentEl, text, state) {
    if (!contentEl) return;
    const full = text == null ? '' : String(text);
    const prevLen = state && state.renderedLen ? state.renderedLen : 0;
    contentEl.classList.add('timeline-stream-plain');

    if (full.length > prevLen && contentEl.childNodes.length === 1 &&
        contentEl.firstChild && contentEl.firstChild.nodeType === Node.TEXT_NODE) {
        const existing = contentEl.firstChild.nodeValue || '';
        if (existing.length === prevLen && full.startsWith(existing)) {
            const delta = full.slice(prevLen);
            if (delta) {
                contentEl.firstChild.appendData(delta);
                if (state) {
                    state.renderedLen = full.length;
                    state.pendingText = full;
                }
                return;
            }
        }
    }

    contentEl.textContent = full;
    if (state) {
        state.renderedLen = full.length;
        state.pendingText = full;
    }
}

function flushStreamPlainTextUpdate(contentEl) {
    if (!contentEl) return;
    const state = streamPlainDomState.get(contentEl);
    if (!state) return;
    if (state.rafId) {
        cancelAnimationFrame(state.rafId);
        state.rafId = 0;
    }
    applyStreamPlainTextNow(contentEl, state.pendingText, state);
}

function scheduleStreamPlainTextUpdate(contentEl, text) {
    if (!contentEl) return;
    const full = text == null ? '' : String(text);
    let state = streamPlainDomState.get(contentEl);
    if (!state) {
        state = { pendingText: full, rafId: 0, renderedLen: 0 };
        streamPlainDomState.set(contentEl, state);
    } else {
        state.pendingText = full;
    }
    streamPlainDomPendingElements.add(contentEl);
    if (state.rafId) return;
    state.rafId = requestAnimationFrame(function () {
        state.rafId = 0;
        applyStreamPlainTextNow(contentEl, state.pendingText, state);
    });
}

function resetStreamPlainTextState(contentEl) {
    if (!contentEl) return;
    const state = streamPlainDomState.get(contentEl);
    if (state && state.rafId) {
        cancelAnimationFrame(state.rafId);
    }
    streamPlainDomState.delete(contentEl);
    streamPlainDomPendingElements.delete(contentEl);
}

function flushAllPendingStreamPlainUpdates() {
    streamPlainDomPendingElements.forEach(function (el) {
        if (el && el.isConnected) {
            flushStreamPlainTextUpdate(el);
        }
    });
}

/** 流式 delta：纯文本，避免每条全量 marked + DOMPurify */
function setTimelineItemContentStreamPlain(contentEl, text) {
    if (!contentEl) return;
    resetStreamPlainTextState(contentEl);
    applyStreamPlainTextNow(contentEl, text, null);
}

/**
 * 分批处理 SSE data 行并在批间让出主线程，避免单次 read() 内数百条事件连续阻塞 UI。
 * @param {string[]} lines
 * @param {(event: object) => void} onEvent
 * @param {{ yieldEvery?: number }} [options]
 */
async function processSseDataLinesYielding(lines, onEvent, options) {
    const yieldEvery = (options && options.yieldEvery) || 32;
    for (let i = 0; i < lines.length; i++) {
        const line = lines[i];
        if (line.startsWith('data: ')) {
            try {
                onEvent(JSON.parse(line.slice(6)));
            } catch (e) {
                console.error('解析事件数据失败:', e, line);
            }
        }
        if ((i + 1) % yieldEvery === 0 && i + 1 < lines.length) {
            await new Promise(function (resolve) { requestAnimationFrame(resolve); });
        }
    }
}

/** 流结束或非流式：富文本（已消毒的 HTML 字符串） */
function setTimelineItemContentStreamRich(contentEl, html) {
    if (!contentEl) return;
    resetStreamPlainTextState(contentEl);
    contentEl.classList.remove('timeline-stream-plain');
    contentEl.innerHTML = html;
}

function formatAssistantMarkdownContent(text) {
    if (typeof window.csMarkdownSanitize !== 'undefined') {
        return window.csMarkdownSanitize.formatMarkdownToHtml(text, { profile: 'chat' });
    }
    const raw = text == null ? '' : String(text);
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

    if (typeof window.csMarkdownSanitize !== 'undefined') {
        window.csMarkdownSanitize.stripSuspiciousImages(bubble);
    }
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

/** 顶栏「停止任务」与进度条按钮对齐时，用会话 ID 反查当前页的 progress 块 ID（无则弹窗内仍可按会话取消） */
function findProgressIdByConversationId(conversationId) {
    if (!conversationId) {
        return null;
    }
    let fallback = null;
    for (const [pid, st] of progressTaskState) {
        if (st && st.conversationId === conversationId) {
            fallback = pid;
            if (document.getElementById(pid)) {
                return pid;
            }
        }
    }
    return fallback;
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

/** 与 MCP 监控一致：仅终止当前进行中的工具调用，工具返回后本轮推理继续（可选 reason 合并进工具结果） */
async function requestCancelWithContinue(conversationId, reason, options = {}) {
    const executionId = options && options.executionId ? String(options.executionId).trim() : '';
    const body = {
        conversationId,
        reason: reason || '',
        continueAfter: true,
    };
    if (executionId) {
        body.executionId = executionId;
    }
    const response = await apiFetch('/api/agent-loop/cancel', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify(body),
    });
    const result = await response.json().catch(() => ({}));
    if (!response.ok) {
        throw new Error(result.error || (typeof window.t === 'function' ? window.t('tasks.cancelFailed') : '取消失败'));
    }
    return result;
}

function openUserInterruptModal(progressId, conversationId) {
    userInterruptModalPending = {
        progressId: progressId != null && progressId !== '' ? progressId : null,
        conversationId,
    };
    const ta = document.getElementById('user-interrupt-reason');
    if (ta) {
        ta.value = '';
    }
    openAppModal('user-interrupt-modal');
}

function closeUserInterruptModal() {
    userInterruptModalPending = null;
    window.__monitorInterruptContext = null;
    closeAppModal('user-interrupt-modal');
}

async function submitUserInterruptContinue() {
    if (!userInterruptModalPending) {
        return;
    }
    const reason = (document.getElementById('user-interrupt-reason') && document.getElementById('user-interrupt-reason').value || '').trim();
    const { progressId, conversationId } = userInterruptModalPending;
    const monitorCtx = window.__monitorInterruptContext;
    closeUserInterruptModal();
    const stopBtn = progressId ? document.getElementById(`${progressId}-stop-btn`) : null;
    try {
        if (stopBtn) {
            stopBtn.disabled = true;
            stopBtn.textContent = typeof window.t === 'function' ? window.t('tasks.interruptSubmitting') : '提交中...';
        }
        await requestCancelWithContinue(conversationId, reason, {
            executionId: monitorCtx && monitorCtx.executionId ? monitorCtx.executionId : '',
        });
        if (monitorCtx && monitorCtx.executionId && typeof refreshMonitorPanel === 'function') {
            const page = (typeof monitorState !== 'undefined' && monitorState.pagination && monitorState.pagination.page)
                ? monitorState.pagination.page
                : 1;
            await refreshMonitorPanel(page);
            window.__monitorInterruptContext = null;
        }
        loadActiveTasks();
    } catch (error) {
        console.error('中断并继续失败:', error);
        alert((typeof window.t === 'function' ? window.t('tasks.cancelTaskFailed') : '操作失败') + ': ' + error.message);
    } finally {
        if (stopBtn) {
            stopBtn.disabled = false;
            stopBtn.textContent = typeof window.t === 'function' ? window.t('tasks.stopTask') : '停止任务';
        }
    }
}

async function submitUserInterruptHardCancel() {
    if (!userInterruptModalPending) {
        return;
    }
    const { progressId, conversationId } = userInterruptModalPending;
    closeUserInterruptModal();
    if (progressId) {
        await performHardCancelProgressTask(progressId);
        return;
    }
    if (!conversationId) {
        return;
    }
    try {
        await requestCancel(conversationId);
        loadActiveTasks();
    } catch (error) {
        console.error('取消任务失败:', error);
        alert((typeof window.t === 'function' ? window.t('tasks.cancelTaskFailed') : '取消任务失败') + ': ' + error.message);
    }
}

/** 彻底停止任务（原「停止任务」行为） */
async function performHardCancelProgressTask(progressId) {
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
    bubble.classList.add('is-streaming');
    const progressWasPinned = typeof window.captureScrollPinState === 'function'
        ? window.captureScrollPinState()
        : true;
    if (typeof window.scrollChatMessagesToBottomIfPinned === 'function') {
        window.scrollChatMessagesToBottomIfPinned(progressWasPinned);
    } else if (progressWasPinned) {
        messagesDiv.scrollTop = messagesDiv.scrollHeight;
    }

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

// 编排器开始输出最终回复时隐藏整条进度消息（过程已迁入助手气泡的「展开详情」，避免与进度卡重复）
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
        if (detailsContainer && detailsContainer.dataset.userExpanded !== '1') {
            const timeline = detailsContainer.querySelector('.progress-timeline');
            if (timeline) {
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

    // 只 flush 终态标题/正文；完成后的详情回看统一走后端分页，不再复制实时 DOM 快照。
    flushAllPendingStreamPlainUpdates();

    // 进度 DOM 即将移除；先关闭仍处于 running 的工具摘要状态。
    finalizeOutstandingToolCallsForProgress(progressId, 'failed');

    const mcpIds = Array.isArray(mcpExecutionIds) ? mcpExecutionIds : [];
    
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
    
    // 查找或创建 MCP 区域（工具栏 + 工具列表 + 迭代时间线）
    if (typeof window.ensureMcpCallSectionChrome === 'function') {
        window.ensureMcpCallSectionChrome(assistantElement, assistantMessageId);
    }
    const mcpSection = assistantElement.querySelector('.mcp-call-section');
    if (!mcpSection) {
        removeMessage(progressId);
        return;
    }

    if (mcpIds.length > 0 && typeof window.setMcpCallExecutionIds === 'function') {
        window.setMcpCallExecutionIds(assistantElement, mcpIds);
        const toolList = mcpSection.querySelector('.mcp-tool-list');
        if (toolList) toolList.classList.remove('expanded');
    }
    if (typeof window.syncMcpToolsToggleButton === 'function') {
        window.syncMcpToolsToggleButton(assistantElement);
    }

    const toolbar = mcpSection.querySelector('.mcp-call-toolbar');
    if (toolbar && !toolbar.querySelector('.process-detail-btn')) {
        const progressDetailBtn = document.createElement('button');
        progressDetailBtn.className = 'mcp-detail-btn process-detail-btn';
        progressDetailBtn.innerHTML = '<span>' + (typeof window.t === 'function' ? window.t('chat.expandDetail') : '展开详情') + '</span>';
        progressDetailBtn.onclick = () => toggleProcessDetails(null, assistantMessageId);
        toolbar.appendChild(progressDetailBtn);
    }

    const detailsId = 'process-details-' + assistantMessageId;
    let detailsContainer = document.getElementById(detailsId);
    const toolListEl = mcpSection.querySelector('.mcp-tool-list');
    
    if (!detailsContainer) {
        detailsContainer = document.createElement('div');
        detailsContainer.id = detailsId;
        detailsContainer.className = 'process-details-container';
        if (toolListEl) {
            toolListEl.after(detailsContainer);
        } else {
            mcpSection.appendChild(detailsContainer);
        }
    }

    if (typeof renderProcessDetails === 'function') {
        renderProcessDetails(assistantMessageId, null);
    } else {
        const expandLabel = typeof window.t === 'function' ? window.t('chat.expandDetail') : '展开详情';
        detailsContainer.dataset.lazyNotLoaded = '1';
        detailsContainer.dataset.loaded = '0';
        detailsContainer.innerHTML = `
            <div class="process-details-content">
                <div class="progress-timeline" id="${detailsId}-timeline">
                    <div class="progress-timeline-empty">${typeof window.t === 'function' ? window.t('chat.expandDetailLazyHint') : (expandLabel + '（点击后加载迭代详情）')}</div>
                </div>
            </div>
        `;
        if (typeof window.bindProcessDetailsLazyHint === 'function') {
            const tl = detailsContainer.querySelector('.progress-timeline');
            if (tl) window.bindProcessDetailsLazyHint(tl, assistantMessageId);
        }
    }

    const expandLabel = typeof window.t === 'function' ? window.t('chat.expandDetail') : '展开详情';
    document.querySelectorAll(`#${assistantMessageId} .process-detail-btn`).forEach((btn) => {
        btn.innerHTML = '<span>' + expandLabel + '</span>';
    });
    
    removeMessage(progressId);
}

const PROCESS_DETAILS_PAGE_SIZE = 50;
const processDetailsAutoLoadObservers = new WeakMap();

function processDetailsContinuousLabel(kind) {
    if (kind === 'older') {
        return typeof window.t === 'function' ? window.t('chat.loadingEarlierDetails') : '正在加载更早记录…';
    }
    if (kind === 'newer') {
        return typeof window.t === 'function' ? window.t('chat.loadingLaterDetails') : '正在加载更新记录…';
    }
    if (kind === 'retry') {
        return typeof window.t === 'function' ? window.t('common.retry') : '加载失败，点击重试';
    }
    return '';
}

function updateProcessDetailsLoadMoreButton(assistantMessageId, backendMessageId, hasMore) {
    updateProcessDetailsPaginationButtons(assistantMessageId, backendMessageId, {
        hasPrev: false,
        hasNext: hasMore
    });
}

function disconnectProcessDetailsAutoLoader(detailsContainer) {
    if (!detailsContainer) return;
    const old = processDetailsAutoLoadObservers.get(detailsContainer);
    if (old) {
        old.disconnect();
        processDetailsAutoLoadObservers.delete(detailsContainer);
    }
}

function scrollProcessDetailsToLatest(assistantMessageId, smooth) {
    const detailsContainer = document.getElementById('process-details-' + assistantMessageId);
    if (!detailsContainer) return;
    const timeline = detailsContainer.querySelector('.progress-timeline');
    if (!timeline) return;
    const behavior = smooth === false ? 'auto' : 'smooth';
    if (timeline.scrollHeight > timeline.clientHeight + 2) {
        timeline.scrollTo({ top: timeline.scrollHeight, behavior: behavior });
        return;
    }
    const items = timeline.querySelectorAll('.timeline-item');
    if (!items.length) return;
    const lastItem = items[items.length - 1];
    lastItem.scrollIntoView({ behavior: behavior, block: 'nearest' });
}

function updateProcessDetailsJumpLatestVisibility(detailsContainer) {
    if (!detailsContainer) return;
    const btn = detailsContainer.querySelector('.process-details-jump-latest');
    const timeline = detailsContainer.querySelector('.progress-timeline');
    if (!btn || !timeline) return;
    const hasUnloadedNewer = detailsContainer.dataset.hasNext === '1';
    const awayFromTimelineBottom = timeline.scrollHeight - timeline.clientHeight - timeline.scrollTop > 120;
    btn.classList.toggle('visible', hasUnloadedNewer || awayFromTimelineBottom);
}

async function requestProcessDetailsAutoPage(assistantMessageId, backendMessageId, direction, sentinel) {
    const detailsContainer = document.getElementById('process-details-' + assistantMessageId);
    if (!detailsContainer || !sentinel) return;
    const loadingKey = direction === 'prev' ? 'loadingPrev' : 'loadingMore';
    const hasKey = direction === 'prev' ? 'hasPrev' : 'hasNext';
    if (detailsContainer.dataset[hasKey] !== '1' || detailsContainer.dataset[loadingKey] === '1') return;
    detailsContainer.dataset[loadingKey] = '1';
    sentinel.classList.add('is-loading');
    sentinel.textContent = processDetailsContinuousLabel(direction === 'prev' ? 'older' : 'newer');
    try {
        await loadProcessDetailsPaginated(assistantMessageId, backendMessageId, {
            prepend: direction === 'prev',
            append: direction === 'next',
            autoLoadAll: false
        });
    } catch (e) {
        console.error('自动加载过程详情失败:', e);
        sentinel.classList.remove('is-loading');
        sentinel.classList.add('is-error');
        sentinel.textContent = processDetailsContinuousLabel('retry');
        sentinel.onclick = function () {
            sentinel.onclick = null;
            sentinel.classList.remove('is-error');
            requestProcessDetailsAutoPage(assistantMessageId, backendMessageId, direction, sentinel);
        };
    } finally {
        detailsContainer.dataset[loadingKey] = '0';
    }
}

function updateProcessDetailsPaginationButtons(assistantMessageId, backendMessageId, pageState) {
    const detailsContainer = document.getElementById('process-details-' + assistantMessageId);
    if (!detailsContainer) return;
    const timeline = detailsContainer.querySelector('.progress-timeline');
    if (!timeline) return;
    const state = pageState || {};
    detailsContainer.dataset.hasPrev = state.hasPrev ? '1' : '0';
    detailsContainer.dataset.hasNext = state.hasNext ? '1' : '0';

    detailsContainer.querySelectorAll('.process-details-load-prev-btn, .process-details-load-more-btn').forEach(function (el) {
        el.remove();
    });
    timeline.querySelectorAll('.process-details-auto-sentinel').forEach(function (el) {
        el.remove();
    });
    disconnectProcessDetailsAutoLoader(detailsContainer);

    let topSentinel = null;
    let bottomSentinel = null;
    if (state.hasPrev) {
        topSentinel = document.createElement('button');
        topSentinel.type = 'button';
        topSentinel.className = 'process-details-auto-sentinel process-details-auto-sentinel--top';
        topSentinel.setAttribute('aria-label', processDetailsContinuousLabel('older'));
        topSentinel.textContent = processDetailsContinuousLabel('older');
        timeline.prepend(topSentinel);
    }
    if (state.hasNext) {
        bottomSentinel = document.createElement('button');
        bottomSentinel.type = 'button';
        bottomSentinel.className = 'process-details-auto-sentinel process-details-auto-sentinel--bottom';
        bottomSentinel.setAttribute('aria-label', processDetailsContinuousLabel('newer'));
        bottomSentinel.textContent = processDetailsContinuousLabel('newer');
        timeline.appendChild(bottomSentinel);
    }

    let jumpBtn = detailsContainer.querySelector('.process-details-jump-latest');
    if (!jumpBtn) {
        jumpBtn = document.createElement('button');
        jumpBtn.type = 'button';
        jumpBtn.className = 'process-details-jump-latest';
        jumpBtn.textContent = typeof window.t === 'function' ? window.t('chat.backToLatestProgress') : '↓ 回到最新进度';
        detailsContainer.appendChild(jumpBtn);
    }
    jumpBtn.onclick = async function () {
        if (detailsContainer.dataset.hasNext === '1') {
            await loadProcessDetailsPaginated(assistantMessageId, backendMessageId, {
                autoLoadAll: false,
                initialLatest: true
            });
        }
        requestAnimationFrame(function () {
            scrollProcessDetailsToLatest(assistantMessageId, true);
            updateProcessDetailsJumpLatestVisibility(detailsContainer);
        });
    };

    if (timeline.dataset.continuousScrollBound !== '1') {
        timeline.dataset.continuousScrollBound = '1';
        timeline.addEventListener('scroll', function () {
            updateProcessDetailsJumpLatestVisibility(detailsContainer);
        }, { passive: true });
    }

    if (typeof IntersectionObserver === 'function' && (topSentinel || bottomSentinel)) {
        const root = document.getElementById('chat-messages') || null;
        const observer = new IntersectionObserver(function (entries) {
            entries.forEach(function (entry) {
                if (!entry.isIntersecting) return;
                if (detailsContainer.dataset.autoLoadSuspended === '1') return;
                if (entry.target === topSentinel) {
                    requestProcessDetailsAutoPage(assistantMessageId, backendMessageId, 'prev', topSentinel);
                } else if (entry.target === bottomSentinel) {
                    requestProcessDetailsAutoPage(assistantMessageId, backendMessageId, 'next', bottomSentinel);
                }
            });
        }, { root: root, rootMargin: '180px 0px', threshold: 0.01 });
        if (topSentinel) observer.observe(topSentinel);
        if (bottomSentinel) observer.observe(bottomSentinel);
        processDetailsAutoLoadObservers.set(detailsContainer, observer);
    } else {
        if (topSentinel) {
            topSentinel.textContent = typeof window.t === 'function' ? window.t('common.loadMore') : '加载更早记录';
            topSentinel.onclick = function () {
                requestProcessDetailsAutoPage(assistantMessageId, backendMessageId, 'prev', topSentinel);
            };
        }
        if (bottomSentinel) {
            bottomSentinel.textContent = typeof window.t === 'function' ? window.t('common.loadMore') : '加载更新记录';
            bottomSentinel.onclick = function () {
                requestProcessDetailsAutoPage(assistantMessageId, backendMessageId, 'next', bottomSentinel);
            };
        }
    }
    updateProcessDetailsJumpLatestVisibility(detailsContainer);
}

/**
 * 分页加载过程详情并增量渲染。默认全量加载供恢复流程使用；
 * 用户手动展开时由任务状态选择首个历史页或最新页，滚动到边界后自动加载相邻页。
 */
async function loadProcessDetailsPaginated(assistantMessageId, backendMessageId, options) {
    if (!assistantMessageId || !backendMessageId || typeof apiFetch !== 'function' || typeof renderProcessDetails !== 'function') {
        return;
    }
    const opts = options || {};
    const autoLoadAll = opts.autoLoadAll !== false;
    const detailsContainer = document.getElementById('process-details-' + assistantMessageId);
    const PAGE = PROCESS_DETAILS_PAGE_SIZE;
    const existingNextOffset = detailsContainer && detailsContainer.dataset.nextOffset
        ? parseInt(detailsContainer.dataset.nextOffset, 10) || 0
        : 0;
    const prepend = !!opts.prepend;
    let offset = prepend && detailsContainer && detailsContainer.dataset.prevOffset
        ? parseInt(detailsContainer.dataset.prevOffset, 10) || 0
        : opts.append && detailsContainer && detailsContainer.dataset.nextOffset
        ? parseInt(detailsContainer.dataset.nextOffset, 10) || 0
        : 0;
    const anchorId = opts.anchorId != null ? String(opts.anchorId).trim() : '';
    if (opts.initialLatest && !prepend && !opts.append && !anchorId) {
        if (detailsContainer) {
            // 初页渲染完成前禁止顶部哨兵抢先触发；定位到底部后再开放自动加载。
            detailsContainer.dataset.autoLoadSuspended = '1';
        }
        const summaryRes = await apiFetch(
            '/api/messages/' + encodeURIComponent(String(backendMessageId)) + '/process-details?summary=1'
        );
        const summaryJSON = await summaryRes.json().catch(() => ({}));
        if (!summaryRes.ok) {
            throw new Error((summaryJSON && summaryJSON.error) ? summaryJSON.error : String(summaryRes.status));
        }
        const total = summaryJSON && summaryJSON.summary && Number(summaryJSON.summary.total);
        offset = Number.isFinite(total) ? Math.max(0, total - PAGE) : 0;
    }
    let isFirst = !opts.append;
    while (true) {
        const params = new URLSearchParams();
        params.set('limit', String(PAGE));
        if (anchorId && !opts.append && !prepend) {
            params.set('anchorId', anchorId);
        } else {
            params.set('offset', String(offset));
        }
        const res = await apiFetch(
            '/api/messages/' + encodeURIComponent(String(backendMessageId)) +
            '/process-details?' + params.toString()
        );
        const j = await res.json().catch(() => ({}));
        if (!res.ok) {
            throw new Error((j && j.error) ? j.error : String(res.status));
        }
        const details = (j && Array.isArray(j.processDetails)) ? j.processDetails : [];
        const toolExecutions = (j && Array.isArray(j.toolExecutions)) ? j.toolExecutions : [];
        const hasMore = !!(j && j.hasMore);
        renderProcessDetails(assistantMessageId, details, {
            append: !isFirst || opts.append,
            prepend: prepend,
            markLoaded: autoLoadAll ? !hasMore : true,
            toolExecutions: toolExecutions
        });
        // renderProcessDetails 对大页分帧渲染；等待一帧后再放置顶部/底部哨兵，
        // 避免哨兵被后续批次插到时间线中间。
        await new Promise((resolve) => requestAnimationFrame(resolve));
        const responseOffset = j && typeof j.offset === 'number' ? j.offset : offset;
        const total = j && typeof j.total === 'number' ? j.total : responseOffset + details.length;
        const nextOffset = prepend && existingNextOffset > 0
            ? existingNextOffset
            : responseOffset + details.length;
        const prevOffset = Math.max(0, responseOffset - PAGE);
        offset = nextOffset;
        if (detailsContainer) {
            detailsContainer.dataset.lazyNotLoaded = '0';
            detailsContainer.dataset.loaded = hasMore ? 'partial' : '1';
            detailsContainer.dataset.prevOffset = String(prevOffset);
            detailsContainer.dataset.nextOffset = String(nextOffset);
            detailsContainer.dataset.total = String(total);
        }
        updateProcessDetailsPaginationButtons(assistantMessageId, backendMessageId, {
            hasPrev: !autoLoadAll && responseOffset > 0,
            hasNext: !autoLoadAll && nextOffset < total
        });
        if (!hasMore || details.length === 0 || !autoLoadAll) {
            break;
        }
        isFirst = false;
        await new Promise((resolve) => requestAnimationFrame(resolve));
    }
    if (opts.initialLatest) {
        requestAnimationFrame(function () {
            scrollProcessDetailsToLatest(assistantMessageId, false);
            if (detailsContainer) {
                delete detailsContainer.dataset.autoLoadSuspended;
            }
        });
    } else if (opts.initialStart) {
        requestAnimationFrame(function () {
            const container = document.getElementById('process-details-' + assistantMessageId);
            const timeline = container && container.querySelector('.progress-timeline');
            if (timeline) timeline.scrollTop = 0;
        });
    }
}

window.loadProcessDetailsPaginated = loadProcessDetailsPaginated;

function shouldInitiallyOpenProcessDetailsAtLatest(assistantMessageId, detailsContainer) {
    if (!detailsContainer) return false;
    // task-events 恢复流会明确给当前详情容器打 is-streaming 标记。
    if (detailsContainer.classList.contains('is-streaming')) return true;
    try {
        const replay = window.__csTaskEventStream;
        if (replay && replay.active && String(replay.assistantDomId || '') === String(assistantMessageId || '')) {
            return true;
        }
    } catch (e) { /* ignore */ }
    // 其余情况按终态/历史详情处理，从第一条开始，便于顺序复盘。
    return false;
}

function resolveEventBackendMessageId(eventData) {
    if (!eventData || typeof eventData !== 'object') return '';
    const raw = eventData.messageId != null ? eventData.messageId : eventData.assistantMessageId;
    return raw != null ? String(raw).trim() : '';
}

function triggerLazyProcessDetailsLoad(assistantMessageId, backendMessageId, detailsContainer) {
    if (!assistantMessageId || !backendMessageId || !detailsContainer) return false;
    if (detailsContainer.dataset.loading === '1') return false;
    const collapseT = typeof window.t === 'function' ? window.t('tasks.collapseDetail') : '收起详情';
    detailsContainer.dataset.loading = '1';
    const timeline = detailsContainer.querySelector('.progress-timeline');
    if (timeline) {
        timeline.innerHTML = '<div class="progress-timeline-empty">' + ((typeof window.t === 'function') ? window.t('common.loading') : '加载中…') + '</div>';
    }
    const openAtLatest = shouldInitiallyOpenProcessDetailsAtLatest(assistantMessageId, detailsContainer);
    loadProcessDetailsPaginated(assistantMessageId, backendMessageId, {
        autoLoadAll: false,
        initialLatest: openAtLatest,
        initialStart: !openAtLatest
    })
        .catch((e) => {
            console.error('加载过程详情失败:', e);
            const tl = detailsContainer.querySelector('.progress-timeline');
            if (tl) {
                tl.innerHTML = '<div class="progress-timeline-empty">' + ((typeof window.t === 'function') ? window.t('chat.noProcessDetail') : '暂无过程详情（加载失败）') + '</div>';
                if (typeof window.bindProcessDetailsLazyHint === 'function') {
                    window.bindProcessDetailsLazyHint(tl, assistantMessageId);
                }
            }
            detailsContainer.dataset.lazyNotLoaded = '1';
            detailsContainer.dataset.loaded = '0';
        })
        .finally(() => {
            detailsContainer.dataset.loading = '0';
            if (detailsContainer.dataset.userExpanded === '1') {
                const tl = detailsContainer.querySelector('.progress-timeline');
                if (tl) {
                    tl.classList.add('expanded');
                }
                if (typeof syncProcessDetailButtonLabels === 'function') {
                    syncProcessDetailButtonLabels(assistantMessageId, true);
                } else {
                    document.querySelectorAll('#' + assistantMessageId + ' .process-detail-btn').forEach((btn) => {
                        btn.innerHTML = '<span>' + collapseT + '</span>';
                    });
                }
            }
        });
    return true;
}

function maybeReloadLazyProcessDetails(assistantMessageId) {
    const detailsContainer = document.getElementById('process-details-' + assistantMessageId);
    if (!detailsContainer) return;
    const isLazy = detailsContainer.dataset.lazyNotLoaded === '1' && detailsContainer.dataset.loaded !== '1';
    if (!isLazy) return;
    const timeline = detailsContainer.querySelector('.progress-timeline');
    const wantsExpanded = detailsContainer.dataset.userExpanded === '1' ||
        !!(timeline && timeline.classList.contains('expanded'));
    if (!wantsExpanded) return;
    const messageEl = document.getElementById(assistantMessageId);
    const backendId = messageEl && messageEl.dataset ? String(messageEl.dataset.backendMessageId || '').trim() : '';
    if (!backendId) return;
    triggerLazyProcessDetailsLoad(assistantMessageId, backendId, detailsContainer);
}

function scheduleProcessDetailsLoadWhenReady(assistantMessageId, detailsContainer, attempt) {
    if (!assistantMessageId || !detailsContainer) return;
    const tries = typeof attempt === 'number' ? attempt : 0;
    if (tries > 25) return;
    const messageEl = document.getElementById(assistantMessageId);
    const backendId = messageEl && messageEl.dataset ? String(messageEl.dataset.backendMessageId || '').trim() : '';
    if (backendId) {
        triggerLazyProcessDetailsLoad(assistantMessageId, backendId, detailsContainer);
        return;
    }
    setTimeout(() => scheduleProcessDetailsLoadWhenReady(assistantMessageId, detailsContainer, tries + 1), 200);
}

// 切换过程详情显示
function toggleProcessDetails(progressId, assistantMessageId) {
    const detailsId = 'process-details-' + assistantMessageId;
    const detailsContainer = document.getElementById(detailsId);
    if (!detailsContainer) return;

    const expandT = typeof window.t === 'function' ? window.t('chat.expandDetail') : '展开详情';
    const collapseT = typeof window.t === 'function' ? window.t('tasks.collapseDetail') : '收起详情';

    // 懒加载：首次展开时才从后端拉取该条消息的过程详情
    const maybeLazy = detailsContainer.dataset && detailsContainer.dataset.lazyNotLoaded === '1' && detailsContainer.dataset.loaded !== '1';
    if (maybeLazy) {
        const messageEl = document.getElementById(assistantMessageId);
        const backendMessageId = messageEl && messageEl.dataset ? messageEl.dataset.backendMessageId : '';
        if (backendMessageId) {
            triggerLazyProcessDetailsLoad(assistantMessageId, backendMessageId, detailsContainer);
        } else {
            scheduleProcessDetailsLoadWhenReady(assistantMessageId, detailsContainer, 0);
        }
    }
    
    const content = detailsContainer.querySelector('.process-details-content');
    const timeline = detailsContainer.querySelector('.progress-timeline');
    const detailBtns = document.querySelectorAll(`#${assistantMessageId} .process-detail-btn`);
    
    const setDetailBtnLabels = (label) => {
        detailBtns.forEach((btn) => { btn.innerHTML = '<span>' + label + '</span>'; });
    };
    const setExpanded = (expanded) => {
        if (!timeline) return;
        if (expanded) {
            timeline.classList.add('expanded');
            detailsContainer.dataset.userExpanded = '1';
            setDetailBtnLabels(collapseT);
        } else {
            timeline.classList.remove('expanded');
            delete detailsContainer.dataset.userExpanded;
            setDetailBtnLabels(expandT);
        }
    };
    if (content && timeline) {
        setExpanded(!timeline.classList.contains('expanded'));
    } else if (timeline) {
        setExpanded(!timeline.classList.contains('expanded'));
    }
    
    // 滚动到展开的详情位置（流式且用户上滑阅读时不抢主列表滚动）
    if (timeline && timeline.classList.contains('expanded')) {
        setTimeout(() => {
            if (window.CyberStrikeChatScroll && typeof window.CyberStrikeChatScroll.scrollIntoViewIfFollowing === 'function') {
                window.CyberStrikeChatScroll.scrollIntoViewIfFollowing(detailsContainer, { behavior: 'smooth', block: 'nearest' });
            } else if (typeof window.captureScrollPinState === 'function' ? window.captureScrollPinState() : true) {
                detailsContainer.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
            }
        }, 100);
    }
}

// 停止当前进度：弹出「中断并说明 / 彻底停止」
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

    openUserInterruptModal(progressId, state.conversationId);
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
    maybeReloadLazyProcessDetails(domAssistantId);
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

function taskReplayProgressId(conversationId) {
    return 'task-ev-' + String(conversationId || '').replace(/[^a-zA-Z0-9_-]/g, '_');
}

function clearCsTaskReplay() {
    window.csTaskReplay = null;
}

function beginCsTaskReplay(progressId, assistantDomId, conversationId) {
    window.csTaskReplay = {
        progressId: progressId,
        assistantDomId: assistantDomId,
        conversationId: conversationId,
        timelineHostId: 'process-details-' + assistantDomId + '-timeline'
    };
    registerProgressTask(progressId, conversationId);
}

function resolveStreamTimeline(progressId) {
    let timeline = document.getElementById(progressId + '-timeline');
    const r = window.csTaskReplay;
    if (!timeline && r && r.progressId === progressId && r.timelineHostId) {
        timeline = document.getElementById(r.timelineHostId);
    }
    return timeline;
}

/** 去重合并 MCP execution id（顺序：先 prev 后 next），用于多段 Run / 多次 SSE 同一任务。 */
function mergeMcpExecutionIDLists(prev, next) {
    const seen = new Set();
    const out = [];
    const add = function (arr) {
        if (!Array.isArray(arr)) return;
        for (let i = 0; i < arr.length; i++) {
            const s = arr[i] != null ? String(arr[i]).trim() : '';
            if (!s || seen.has(s)) continue;
            seen.add(s);
            out.push(s);
        }
    };
    add(prev);
    add(next);
    return out;
}

function formatEinoRunRetryMessage(message, data) {
    const d = data && typeof data === 'object' ? data : {};
    const base = String(message || '').trim();
    const errRaw = d.errorSummary != null && String(d.errorSummary).trim() !== ''
        ? String(d.errorSummary).trim()
        : (d.error != null ? String(d.error).trim() : '');
    const lines = [];
    if (base) lines.push(base);
    const attempt = Number(d.attempt || 0);
    const maxAttempts = Number(d.maxAttempts || 0);
    const backoffSec = Number(d.backoffSec || 0);
    const kind = formatEinoRunRetryKind(d.errorKind);
    if (Number.isFinite(attempt) && attempt > 0 && Number.isFinite(maxAttempts) && maxAttempts > 0) {
        const retryPlan = typeof window.t === 'function'
            ? window.t('chat.einoRunRetryPlan', { attempt: attempt, maxAttempts: maxAttempts, backoffSec: Number.isFinite(backoffSec) && backoffSec > 0 ? backoffSec : '-' })
            : ('重试进度：第 ' + attempt + '/' + maxAttempts + ' 次，等待 ' + (Number.isFinite(backoffSec) && backoffSec > 0 ? backoffSec : '-') + ' 秒');
        if (!base || base.indexOf(String(attempt) + '/' + String(maxAttempts)) === -1) {
            lines.push(retryPlan);
        }
    }
    if (kind) {
        const kindLabel = typeof window.t === 'function'
            ? window.t('chat.einoRunRetryReasonKind')
            : '原因类型';
        lines.push(kindLabel + '：' + kind);
    }
    if (!errRaw) {
        return lines.join('\n');
    }
    const detailLabel = typeof window.t === 'function'
        ? window.t('chat.einoRunRetryErrorDetail')
        : '错误详情';
    if (!base || base.indexOf(errRaw) === -1) {
        lines.push(detailLabel + '：' + errRaw);
    }
    return lines.join('\n');
}

function formatEinoRunRetryKind(kind) {
    const key = String(kind || '').trim();
    if (!key) return '';
    const labels = {
        rate_limit: '限流 / 请求过多',
        retryable_http: '可重试 HTTP 错误',
        upstream_server: '上游服务错误',
        http_error: 'HTTP 错误',
        upstream_busy: '上游繁忙',
        network: '网络连接异常',
        stream: '流式读取异常',
        transient: '临时异常'
    };
    if (typeof window.t === 'function') {
        const translated = window.t('chat.einoRunRetryKind_' + key);
        if (translated && translated !== 'chat.einoRunRetryKind_' + key) return translated;
    }
    return labels[key] || key;
}

function formatEinoRunRetryTitle(data) {
    const d = data && typeof data === 'object' ? data : {};
    const base = typeof window.t === 'function'
        ? window.t('chat.einoRunRetryTitle')
        : '🔁 临时错误重试';
    const attempt = Number(d.attempt || 0);
    const maxAttempts = Number(d.maxAttempts || 0);
    if (Number.isFinite(attempt) && attempt > 0 && Number.isFinite(maxAttempts) && maxAttempts > 0) {
        return base + '（' + attempt + '/' + maxAttempts + '）';
    }
    return base;
}

// 处理流式事件
function handleStreamEvent(event, progressElement, progressId, 
                          getAssistantId, setAssistantId, getMcpIds, setMcpIds) {
    const streamScrollWasPinned = typeof window.captureScrollPinState === 'function'
        ? window.captureScrollPinState()
        : (typeof window.isChatMessagesPinnedToBottom === 'function' ? window.isChatMessagesPinnedToBottom() : true);

    // 不依赖进度时间线；在首条 SSE 即可绑定用户消息 ID
    if (event.type === 'message_saved') {
        const d = event.data || {};
        if (d.userMessageId) {
            applyBackendMessageIdToLastUser(d.userMessageId);
        }
        scrollChatMessagesToBottomIfPinned(streamScrollWasPinned);
        return;
    }

    const timeline = resolveStreamTimeline(progressId);
    const canHandleWithoutTimeline = ['conversation', 'response', 'error', 'cancelled', 'done'].includes(String(event.type || ''));
    if (!timeline && !canHandleWithoutTimeline) return;

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
                syncAgentLiveStreamConversationId(event.data.conversationId);
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
            const scope = d.einoScope != null ? String(d.einoScope).trim() : '';
            if (scope !== 'sub') {
                const prevMainIter = mainIterationStateByProgressId.get(String(progressId));
                const prevN = prevMainIter && prevMainIter.iteration != null ? prevMainIter.iteration : null;
                const prevNode = prevMainIter && prevMainIter.workflowNodeId != null
                    ? String(prevMainIter.workflowNodeId).trim()
                    : '';
                const curNode = d.workflowNodeId != null ? String(d.workflowNodeId).trim() : '';
                const prevAgent = prevMainIter && prevMainIter.einoAgent != null
                    ? String(prevMainIter.einoAgent).trim()
                    : '';
                const curAgent = d.einoAgent != null ? String(d.einoAgent).trim() : '';
                const prevOrchestration = prevMainIter && prevMainIter.orchestration != null
                    ? String(prevMainIter.orchestration).trim()
                    : '';
                const curOrchestration = d.orchestration != null ? String(d.orchestration).trim() : '';
                const duplicateMainIteration = prevN != null && String(prevN) === String(n) &&
                    prevNode === curNode && prevAgent === curAgent &&
                    prevOrchestration === curOrchestration;
                mainIterationStateByProgressId.set(String(progressId), {
                    iteration: n,
                    orchestration: d.orchestration != null ? d.orchestration : '',
                    workflowNodeId: curNode,
                    einoAgent: curAgent
                });
                // 主通道进入新轮次或工作流切换到新 Agent 节点后，不复用上一段的流式时间线条目
                if (prevN != null && (n < prevN || prevN !== n || (curNode && prevNode && curNode !== prevNode))) {
                    clearTimelineStreamStates(progressId);
                }
                // 同一主代理可能从“进入模型”和“检测到工具批次”两条路径补发同一轮次。
                // 状态缓存仍更新，但时间线只保留一个幂等条目。
                if (duplicateMainIteration) break;
            }
            let iterTitle;
            if (d.orchestration === 'plan_execute' && d.einoScope === 'main') {
                const phase = translatePlanExecuteAgentName(d.einoAgent != null ? d.einoAgent : '');
                iterTitle = typeof window.t === 'function'
                    ? window.t('chat.einoPlanExecuteRound', { n: n, phase: phase })
                    : ('Plan-Execute · 第 ' + n + ' 轮 · ' + phase);
            } else if (d.einoScope === 'main') {
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

        case 'workflow_start': {
            const d = event.data || {};
            const name = d.workflowName || d.workflowId || '';
            addTimelineItem(timeline, 'workflow_start', {
                title: '🧭 工作流开始' + (name ? (' · ' + name) : ''),
                message: event.message || '',
                data: d
            });
            break;
        }

        case 'workflow_done': {
            const d = event.data || {};
            const name = d.workflowName || d.workflowId || '';
            addTimelineItem(timeline, 'workflow_done', {
                title: '✅ 工作流完成' + (name ? (' · ' + name) : ''),
                message: event.message || '',
                data: d
            });
            break;
        }

        case 'workflow_node_start': {
            const d = event.data || {};
            const label = d.label || d.nodeId || '';
            const nodeType = d.nodeType != null ? String(d.nodeType).toLowerCase() : '';
            if (nodeType === 'agent') {
                clearTimelineStreamStates(progressId);
            }
            addTimelineItem(timeline, 'workflow_node_start', {
                title: '▶ 节点开始' + (label ? (' · ' + label) : ''),
                message: event.message || '',
                data: d
            });
            break;
        }

        case 'workflow_node_result': {
            const d = event.data || {};
            const label = d.label || d.nodeId || '';
            const status = d.status || '';
            const nodeType = d.nodeType != null ? String(d.nodeType).toLowerCase() : '';
            let title;
            if (nodeType === 'condition') {
                const matched = d.matched === true || d.matched === 'true' || (d.output && (d.output.matched === true || d.output.matched === 'true'));
                title = (matched ? '✅' : '🔀') + ' 条件判断' + (label ? (' · ' + label) : '') + ' → ' + (matched ? '是' : '否');
            } else {
                const icon = status === 'failed' ? '❌' : (status === 'skipped' ? '⏭️' : '✅');
                title = icon + ' 节点完成' + (label ? (' · ' + label) : '') + (status ? ('（' + status + '）') : '');
            }
            addTimelineItem(timeline, 'workflow_node_result', {
                title: title,
                message: event.message || '',
                data: d
            });
            break;
        }

        case 'workflow_branch_taken':
        case 'workflow_branch_skipped': {
            const d = event.data || {};
            const branch = d.branchLabel || '';
            const target = d.targetLabel || d.targetId || '';
            const taken = event.type === 'workflow_branch_taken';
            addTimelineItem(timeline, event.type, {
                title: (taken ? '➡️' : '⏭️') + (taken ? ' 执行分支' : ' 跳过分支') + (branch ? (' · ' + branch) : '') + (target ? (' → ' + target) : ''),
                message: event.message || '',
                data: d
            });
            break;
        }

        case 'workflow_tool_start': {
            const d = event.data || {};
            const tool = d.tool || d.toolName || '';
            addTimelineItem(timeline, 'workflow_tool_start', {
                title: '🔧 工具节点' + (tool ? (' · ' + tool) : ''),
                message: event.message || '',
                data: d
            });
            break;
        }

        case 'workflow_agent_output': {
            const d = event.data || {};
            const label = d.label || d.nodeId || '';
            addTimelineItem(timeline, 'workflow_agent_output', {
                title: '🤖 Agent 输出' + (label ? (' · ' + label) : ''),
                message: event.message || '',
                data: d
            });
            break;
        }

        case 'workflow_hitl_checkpoint': {
            addTimelineItem(timeline, 'workflow_hitl_checkpoint', {
                title: '🧑‍⚖️ 人工确认检查点',
                message: event.message || '',
                data: event.data || {}
            });
            break;
        }

        case 'workflow_hitl_waiting': {
            const d = event.data || {};
            const hitlItemId = addTimelineItem(timeline, 'workflow_hitl_waiting', {
                title: '🧑‍⚖️ 工作流等待审批',
                message: event.message || '',
                data: d
            });
            renderInlineWorkflowHitlApproval(hitlItemId, d);
            break;
        }

        case 'workflow_hitl_resumed': {
            addTimelineItem(timeline, 'workflow_hitl_resumed', {
                title: '✅ 审批已通过',
                message: event.message || '人工审批已通过，继续执行',
                data: event.data || {}
            });
            break;
        }

        case 'workflow_hitl_rejected': {
            addTimelineItem(timeline, 'workflow_hitl_rejected', {
                title: '❌ 审批已拒绝',
                message: event.message || '',
                data: event.data || {}
            });
            break;
        }

        case 'workflow_paused': {
            addTimelineItem(timeline, 'workflow_paused', {
                title: '⏸️ 工作流已暂停',
                message: event.message || '',
                data: event.data || {}
            });
            break;
        }

        case 'eino_trace_run':
        case 'eino_trace_start':
        case 'eino_trace_end':
        case 'eino_trace_error': {
            const d = event.data || {};
            const comp = d.component != null ? String(d.component) : '';
            const name = d.name != null ? String(d.name) : '';
            let glyph = '◆';
            if (event.type === 'eino_trace_run') glyph = '●';
            else if (event.type === 'eino_trace_start') glyph = '▶';
            else if (event.type === 'eino_trace_end') glyph = '■';
            else if (event.type === 'eino_trace_error') glyph = '✖';
            const title = '[Eino] ' + glyph + ' ' + (comp || 'component') + (name ? '/' + name : '');
            const parts = [];
            if (d.runId) parts.push('run=' + String(d.runId));
            if (d.spanId) parts.push('span=' + String(d.spanId));
            if (d.parentSpanId) parts.push('parent=' + String(d.parentSpanId));
            if (d.inputSummary) parts.push(String(d.inputSummary));
            if (d.outputSummary) parts.push(String(d.outputSummary));
            if (d.error) parts.push(String(d.error));
            if (event.message && String(event.message).trim()) parts.push(String(event.message));
            const body = parts.join(' · ');
            addTimelineItem(timeline, 'progress', { title, message: body, data: d });
            break;
        }
            
        case 'thinking_stream_start':
        case 'reasoning_chain_stream_start': {
            const d = event.data || {};
            const streamId = d.streamId || null;
            if (!streamId) break;

            const timelineType = event.type === 'reasoning_chain_stream_start' ? 'reasoning_chain' : 'thinking';

            let state = thinkingStreamStateByProgressId.get(progressId);
            if (!state) {
                state = new Map();
                thinkingStreamStateByProgressId.set(progressId, state);
            }
            // 同一 streamId 重复 start：复用已有条目，避免孤儿卡片 + 新条目重复收 delta
            if (state.has(streamId)) {
                const ex = state.get(streamId);
                ex.buffer = '';
                const existingItem = document.getElementById(ex.itemId);
                if (existingItem) {
                    const contentEl = existingItem.querySelector('.timeline-item-content');
                    if (contentEl) {
                        setTimelineItemContentStreamPlain(contentEl, '');
                    }
                }
                break;
            }
            const labelBase = typeof window.t === 'function'
                ? window.t(timelineType === 'reasoning_chain' ? 'chat.reasoningChain' : 'chat.aiThinking')
                : (timelineType === 'reasoning_chain' ? '推理过程' : 'AI思考');
            const emoji = timelineType === 'reasoning_chain' ? '🔗' : '🤔';
            const title = timelineAgentBracketPrefix(d) + emoji + ' ' + labelBase;
            const itemId = addTimelineItem(timeline, timelineType, {
                title: title,
                message: ' ',
                data: d
            });
            state.set(streamId, { itemId, buffer: '' });
            break;
        }

        case 'thinking_stream_delta':
        case 'reasoning_chain_stream_delta': {
            const d = event.data || {};
            const streamId = d.streamId || null;
            if (!streamId) break;

            const state = thinkingStreamStateByProgressId.get(progressId);
            if (!state || !state.has(streamId)) break;
            const s = state.get(streamId);

            const delta = event.message || '';
            s.buffer = mergeStreamBuffer(s.buffer, delta, d);

            const item = document.getElementById(s.itemId);
            if (item) {
                const contentEl = item.querySelector('.timeline-item-content');
                if (contentEl) {
                    scheduleStreamPlainTextUpdate(contentEl, s.buffer);
                }
            }
            break;
        }

        case 'thinking':
        case 'reasoning_chain': {
            const timelineType = event.type === 'reasoning_chain' ? 'reasoning_chain' : 'thinking';
            // 若已由 *_stream_* 聚合（带 streamId），避免重复创建 timeline item
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
                            flushStreamPlainTextUpdate(contentEl);
                            setTimelineItemContentStreamPlain(contentEl, s.buffer);
                        }
                    }
                    break;
                }
            }

            const labelBase = typeof window.t === 'function'
                ? window.t(timelineType === 'reasoning_chain' ? 'chat.reasoningChain' : 'chat.aiThinking')
                : (timelineType === 'reasoning_chain' ? '推理过程' : 'AI思考');
            const emoji = timelineType === 'reasoning_chain' ? '🔗' : '🤔';
            addTimelineItem(timeline, timelineType, {
                title: timelineAgentBracketPrefix(event.data) + emoji + ' ' + labelBase,
                message: event.message,
                data: event.data
            });
            break;
        }
            
        case 'tool_calls_detected':
            // 助手正文段结束、进入工具调用：下一段 response_start 应新建时间线条目
            responseStreamStateByProgressId.delete(progressId);
            break;

        case 'warning':
            addTimelineItem(timeline, 'warning', {
                title: '⚠️',
                message: event.message,
                data: event.data
            });
            break;

        case 'hitl_interrupt':
            const hitlItemId = addTimelineItem(timeline, 'warning', {
                title: '🧑‍⚖️ HITL',
                message: event.message,
                data: event.data
            });
            renderInlineHitlApproval(hitlItemId, event.data || {});
            try {
                window.dispatchEvent(new CustomEvent('hitl-interrupt', { detail: event.data || {} }));
            } catch (e) {}
            break;
        case 'hitl_resumed':
            addTimelineItem(timeline, 'progress', {
                title: '✅ HITL',
                message: event.message,
                data: event.data
            });
            break;
        case 'hitl_rejected':
            addTimelineItem(timeline, 'error', {
                title: '⛔ HITL',
                message: event.message,
                data: event.data
            });
            break;

        case 'user_interrupt_continue': {
            const d = event.data || {};
            const titleBase = typeof window.t === 'function'
                ? window.t('chat.userInterruptContinueTitle')
                : '⏸️ 用户中断并继续';
            addTimelineItem(timeline, 'user_interrupt_continue', {
                title: titleBase,
                message: event.message || '',
                data: d
            });
            finalizeOutstandingToolCallsForProgress(progressId, 'failed');
            break;
        }

        case 'eino_stream_error': {
            const d = event.data || {};
            const agent = d.einoAgent ? String(d.einoAgent) : '';
            const title = typeof window.t === 'function'
                ? window.t('chat.einoStreamErrorTitle', { agent: agent || '-' })
                : (agent ? ('⚠️ Eino 流式中断（' + agent + '）') : '⚠️ Eino 流式中断');
            addTimelineItem(timeline, 'warning', {
                title: title,
                message: event.message || (typeof window.t === 'function'
                    ? window.t('chat.einoStreamErrorMessage')
                    : '流式读取异常，系统将按策略重试或结束。'),
                data: d
            });
            break;
        }

        case 'eino_empty_response_continue': {
            const d = event.data || {};
            const title = typeof window.t === 'function'
                ? window.t('chat.einoEmptyResponseContinueTitle')
                : '🔁 自动续跑（无助手正文）';
            addTimelineItem(timeline, 'warning', {
                title: title,
                message: event.message || (typeof window.t === 'function'
                    ? window.t('chat.einoEmptyResponseContinueMessage')
                    : '会话已结束但未捕获到助手正文，正在基于轨迹自动续跑…'),
                data: d
            });
            break;
        }

        case 'eino_run_retry': {
            const d = event.data || {};
            const msg = formatEinoRunRetryMessage(event.message, d);
            addTimelineItem(timeline, 'warning', {
                title: formatEinoRunRetryTitle(d),
                message: msg,
                data: d
            });
            break;
        }

        case 'iteration_limit_reached': {
            addTimelineItem(timeline, 'warning', {
                title: typeof window.t === 'function' ? window.t('chat.iterationLimitReachedTitle') : '⛔ 达到迭代上限',
                message: event.message || (typeof window.t === 'function'
                    ? window.t('chat.iterationLimitReachedMessage')
                    : '已达到最大迭代次数，任务已停止继续自动迭代。'),
                data: event.data
            });
            finalizeOutstandingToolCallsForProgress(progressId, 'failed');
            break;
        }

        case 'eino_pending_orphaned': {
            const d = event.data || {};
            const count = Number(d.pendingCount || 0);
            const countText = Number.isFinite(count) && count > 0 ? String(count) : '?';
            addTimelineItem(timeline, 'warning', {
                title: typeof window.t === 'function' ? window.t('chat.einoPendingOrphanedTitle') : '🧹 工具调用收尾补偿',
                message: event.message || (typeof window.t === 'function'
                    ? window.t('chat.einoPendingOrphanedMessage', { count: countText })
                    : ('检测到 ' + countText + ' 个未闭合工具调用，已自动标记为失败并收尾。')),
                data: d
            });
            finalizeOutstandingToolCallsForProgress(progressId, 'failed');
            break;
        }

        case 'tool_call':
            const toolInfo = event.data || {};
            const toolName = toolInfo.toolName || (typeof window.t === 'function' ? window.t('chat.unknownTool') : '未知工具');
            const index = toolInfo.index || 0;
            const total = toolInfo.total || 0;
            const toolCallId = toolInfo.toolCallId || null;
            if (toolCallId) {
                const existing = getToolCallMapping(progressId, toolCallId);
                if (existing && existing.itemId) {
                    const existingItem = document.getElementById(existing.itemId);
                    if (existingItem) {
                        // 同一 toolCallId 的重复 tool_call（重试/补发）只更新状态，不重复追加条目。
                        if (!existing.terminalStatus) {
                            updateToolCallStatus(progressId, toolCallId, 'running');
                        }
                        break;
                    }
                }
            }
            const toolCallTitle = formatToolCallTimelineTitle(toolName, index, total);
            const toolCallItemId = addTimelineItem(timeline, 'tool_call', {
                title: timelineAgentBracketPrefix(toolInfo) + '🔧 ' + toolCallTitle,
                message: event.message,
                data: toolInfo,
                processDetailId: toolInfo.processDetailId || '',
                expanded: false
            });
            
            // 如果有toolCallId，存储映射关系以便后续更新状态
            if (toolCallId && toolCallItemId) {
                const mapKey = toolCallMapKey(progressId, toolCallId);
                toolCallStatusMap.set(mapKey, {
                    toolCallId: toolCallId,
                    itemId: toolCallItemId,
                    timeline: timeline,
                    progressId: progressId
                });
                
                // 添加执行中状态指示器
                updateToolCallStatus(progressId, toolCallId, 'running');
            }
            break;

        case 'tool_result_delta':
            // 工具执行过程不流式展示，仅等 tool_result 展示最终结果。
            break;
            
        case 'tool_result':
            const resultInfo = event.data || {};
            const resultToolName = resultInfo.toolName || (typeof window.t === 'function' ? window.t('chat.unknownTool') : '未知工具');
            const success = resultInfo.success !== false;
            const resultDisplayState = getToolResultDisplayState(resultInfo, { rawText: event.message || '' });
            const backgroundRunning = resultDisplayState.kind === 'background_running';
            const statusIcon = backgroundRunning ? '⏳' : (success ? '✅' : '❌');
            const resultToolCallId = resultInfo.toolCallId || null;
            const resultStatusForCall = backgroundRunning ? 'background_running' : (success ? 'completed' : 'failed');
            const resultExecText = backgroundRunning
                ? (getBackgroundRunningToolLabel() + ': ' + escapeHtml(resultToolName))
                : (success ? (typeof window.t === 'function' ? window.t('chat.toolExecComplete', { name: escapeHtml(resultToolName) }) : '工具 ' + escapeHtml(resultToolName) + ' 执行完成') : (typeof window.t === 'function' ? window.t('chat.toolExecFailed', { name: escapeHtml(resultToolName) }) : '工具 ' + escapeHtml(resultToolName) + ' 执行失败'));

            if (resultToolCallId) {
                const key = toolResultStreamKey(progressId, resultToolCallId);
                const streamState = toolResultStreamStateByKey.get(key);
                if (streamState && streamState.itemId) {
                    const streamCallItem = document.getElementById(streamState.itemId);
                    if (streamCallItem) {
                        mergeToolResultIntoCallItem(streamCallItem, resultInfo);
                    }
                    toolResultStreamStateByKey.delete(key);
                    const mapKey = toolCallMapKey(progressId, resultToolCallId);
                    if (toolCallStatusMap.has(mapKey)) {
                        updateToolCallStatus(progressId, resultToolCallId, resultStatusForCall);
                        toolCallStatusMap.get(mapKey).terminalStatus = resultStatusForCall;
                    }
                    break;
                }
                if (attachToolResultToCall(progressId, resultToolCallId, resultInfo)) {
                    const mapKey = toolCallMapKey(progressId, resultToolCallId);
                    if (toolCallStatusMap.has(mapKey)) {
                        updateToolCallStatus(progressId, resultToolCallId, resultStatusForCall);
                        toolCallStatusMap.get(mapKey).terminalStatus = resultStatusForCall;
                    }
                    break;
                }
            }

            if (resultToolCallId && toolCallStatusMap.has(toolCallMapKey(progressId, resultToolCallId))) {
                updateToolCallStatus(progressId, resultToolCallId, resultStatusForCall);
                toolCallStatusMap.get(toolCallMapKey(progressId, resultToolCallId)).terminalStatus = resultStatusForCall;
            }
            addTimelineItem(timeline, 'tool_result', {
                title: timelineAgentBracketPrefix(resultInfo) + statusIcon + ' ' + resultExecText,
                message: event.message,
                data: resultInfo,
                processDetailId: resultInfo.processDetailId || '',
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
            if (stateMap.has(streamId)) {
                const ex = stateMap.get(streamId);
                ex.buffer = '';
                const existingItem = document.getElementById(ex.itemId);
                if (existingItem) {
                    let contentEl = existingItem.querySelector('.timeline-item-content');
                    if (contentEl) {
                        setTimelineItemContentStreamPlain(contentEl, '');
                    }
                }
                break;
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
            if (!delta && streamBufferFromAccumulated(d) === null) break;
            const stateMap = einoAgentReplyStreamStateByProgressId.get(progressId);
            if (!stateMap || !stateMap.has(streamId)) break;
            const s = stateMap.get(streamId);
            s.buffer = mergeStreamBuffer(s.buffer, delta, d);
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
                    scheduleStreamPlainTextUpdate(contentEl, s.buffer);
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
                    flushStreamPlainTextUpdate(contentEl);
                    setTimelineItemContentStreamPlain(contentEl, full);
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
                    try {
                        progressEl.dataset.progressRawData = event.data ? JSON.stringify(event.data) : '';
                    } catch (e) {
                        progressEl.dataset.progressRawData = '';
                    }
                }
                const progressMsg = translateProgressMessage(event.message, event.data);
                progressTitle.textContent = '🔍 ' + progressMsg;
            }
            break;
        
        case 'cancelled':
            const taskCancelledText = typeof window.t === 'function' ? window.t('chat.taskCancelled') : '任务已取消';
            if (timeline) {
                addTimelineItem(timeline, 'cancelled', {
                    title: '⛔ ' + taskCancelledText,
                    message: event.message,
                    data: event.data
                });
            }
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
                const preferredMessageId = resolveEventBackendMessageId(event.data) || null;
                const { assistantId, assistantElement } = upsertTerminalAssistantMessage(event.message, preferredMessageId);
                if (assistantId && preferredMessageId) {
                    applyBackendMessageIdToAssistantDom(assistantId, preferredMessageId);
                }
                if (assistantElement) {
                    const detailsId = 'process-details-' + assistantId;
                    if (!document.getElementById(detailsId)) {
                        integrateProgressToMCPSection(progressId, assistantId, typeof getMcpIds === 'function' ? (getMcpIds() || []) : []);
                    } else if (preferredMessageId) {
                        maybeReloadLazyProcessDetails(assistantId);
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
            const streamIdentity = buildMainResponseStreamIdentity(progressId, responseData);
            const streamIterTag = extractIterationTagFromStreamIdentity(streamIdentity);
            const mcpIds = responseData.mcpExecutionIds || [];
            setMcpIds(mergeMcpExecutionIDLists(typeof getMcpIds === 'function' ? (getMcpIds() || []) : [], mcpIds));

            if (responseData.conversationId) {
                // 如果用户已经开始了新对话（currentConversationId 为 null），且这个事件来自旧对话，则忽略
                if (currentConversationId === null && responseOriginalConversationId !== null) {
                    updateProgressConversation(progressId, responseData.conversationId);
                    break;
                }
                currentConversationId = responseData.conversationId;
                syncAgentLiveStreamConversationId(responseData.conversationId);
                updateActiveConversation();
                addAttackChainButton(currentConversationId);
                updateProgressConversation(progressId, responseData.conversationId);
                loadActiveTasks();
            }

            // 多代理模式下，迭代过程中的输出只显示在时间线中，不创建助手消息气泡
            const prevStream = responseStreamStateByProgressId.get(progressId);
            const streamOrch = responseData.orchestration != null
                ? responseData.orchestration
                : (prevStream && prevStream.streamMeta ? prevStream.streamMeta.orchestration : '');
            if (shouldReuseMainResponseStream(progressId, prevStream, responseData, streamOrch)) {
                // Eino 可能对同一段流重复发 response_start；复用已有条目与 buffer，避免多条「助手输出」
                prevStream.streamMeta = Object.assign({}, prevStream.streamMeta || {}, responseData);
                prevStream.streamIdentity = streamIdentity;
                if (responseData.streamId != null) {
                    prevStream.streamId = String(responseData.streamId).trim();
                }
                responseStreamStateByProgressId.set(progressId, prevStream);
                break;
            }
            const title = einoMainStreamPlanningTitle(responseData);
            const itemId = addTimelineItem(timeline, 'thinking', {
                title: title,
                message: ' ',
                data: Object.assign({}, responseData, { responseStreamPlaceholder: true })
            });
            responseStreamStateByProgressId.set(progressId, {
                progressId: progressId,
                itemId: itemId,
                buffer: '',
                streamMeta: responseData,
                streamIdentity: streamIdentity,
                streamId: responseData.streamId != null ? String(responseData.streamId).trim() : ''
            });
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
            const incomingStreamId = responseData.streamId != null ? String(responseData.streamId).trim() : '';
            if (!state) {
                state = { progressId: progressId, itemId: null, buffer: '', streamMeta: responseData, streamId: incomingStreamId };
                responseStreamStateByProgressId.set(progressId, state);
            } else if (!state.streamMeta && responseData && (responseData.einoAgent || responseData.orchestration)) {
                state.streamMeta = responseData;
            }
            if (incomingStreamId && state.streamId && state.streamId !== incomingStreamId) {
                break;
            }
            if (incomingStreamId && !state.streamId) {
                state.streamId = incomingStreamId;
            }

            const deltaContent = event.message || '';
            if (!deltaContent && streamBufferFromAccumulated(responseData) === null) break;
            state.buffer = mergeStreamBuffer(state.buffer, deltaContent, responseData);

            // 流式阶段仅追加纯文本；formatTimelineStreamBody 在终态 response 时一次性处理
            if (state.itemId) {
                const item = document.getElementById(state.itemId);
                if (item) {
                    const contentEl = item.querySelector('.timeline-item-content');
                    if (contentEl) {
                        scheduleStreamPlainTextUpdate(contentEl, state.buffer);
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
            const mcpIds = mergeMcpExecutionIDLists(typeof getMcpIds === 'function' ? (getMcpIds() || []) : [], responseData.mcpExecutionIds || []);
            setMcpIds(mcpIds);

            // 更新对话ID
            if (responseData.conversationId) {
                if (currentConversationId === null && responseOriginalConversationId !== null) {
                    updateProgressConversation(progressId, responseData.conversationId);
                    break;
                }

                currentConversationId = responseData.conversationId;
                syncAgentLiveStreamConversationId(responseData.conversationId);
                updateActiveConversation();
                addAttackChainButton(currentConversationId);
                updateProgressConversation(progressId, responseData.conversationId);
                loadActiveTasks();
            }

            // 如果之前已经在 response_start/response_delta 阶段创建过占位，则复用该消息更新最终内容
            const streamState = responseStreamStateByProgressId.get(progressId);
            const existingAssistantId = streamState?.assistantId || getAssistantId();
            let assistantIdFinal = existingAssistantId;
            const bubbleText = resolveFinalAssistantResponseText(event.message, streamState);

            if (!assistantIdFinal) {
                assistantIdFinal = addMessage('assistant', bubbleText, mcpIds, progressId);
                setAssistantId(assistantIdFinal);
            } else {
                setAssistantId(assistantIdFinal);
                updateAssistantBubbleContent(assistantIdFinal, bubbleText, true);
            }

            // 将 response_start/response_delta 占位固化为 planning，与后端落库一致后再快照过程详情
            if (streamState && streamState.itemId) {
                finalizeMainResponseStreamItem(streamState, event.message, responseData);
            } else if (timeline && bubbleText && String(bubbleText).trim() && !isEinoEmptyResponsePlaceholder(event.message)) {
                addTimelineItem(timeline, 'planning', {
                    title: typeof einoMainStreamPlanningTitle === 'function'
                        ? einoMainStreamPlanningTitle(responseData)
                        : ('📝 ' + (typeof window.t === 'function' ? window.t('chat.planning') : '规划中')),
                    message: event.message,
                    data: responseData,
                    expanded: false
                });
            }

            // 最终回复时隐藏进度卡片（多代理模式下，迭代过程已完整展示）
            hideProgressMessageForFinalReply(progressId);

            // Before integrating/removing the progress DOM, close any outstanding running tool calls
            // so the copied timeline HTML reflects the final status.
            finalizeOutstandingToolCallsForProgress(progressId, 'failed');

            const respMid = responseData.messageId;
            if (respMid) {
                applyBackendMessageIdToAssistantDom(assistantIdFinal, respMid);
            }

            const replayCtx = window.csTaskReplay;
            const directReplay = replayCtx && replayCtx.progressId === progressId;
            if (!directReplay) {
                // 将详情入口集成到工具调用区域；完整过程从后端分页加载，不依赖实时 DOM 快照。
                integrateProgressToMCPSection(progressId, assistantIdFinal, mcpIds);
            }
            responseStreamStateByProgressId.delete(progressId);

            if (respMid) {
                if (typeof window.syncAssistantReasoningContentFromServer === 'function') {
                    setTimeout(function () {
                        window.syncAssistantReasoningContentFromServer(respMid, assistantIdFinal);
                    }, 400);
                }
            }

            setTimeout(() => {
                collapseAllProgressDetails(assistantIdFinal, directReplay ? null : progressId);
            }, 3000);

            setTimeout(() => {
                loadConversations();
            }, 200);
            break;
            
        case 'error':
            // 显示错误
            if (timeline) {
                addTimelineItem(timeline, 'error', {
                    title: '❌ ' + (typeof window.t === 'function' ? window.t('chat.error') : '错误'),
                    message: event.message,
                    data: event.data
                });
            }
            
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
                const preferredMessageId = resolveEventBackendMessageId(event.data) || null;
                const { assistantId, assistantElement } = upsertTerminalAssistantMessage(event.message, preferredMessageId);
                if (assistantId && preferredMessageId) {
                    applyBackendMessageIdToAssistantDom(assistantId, preferredMessageId);
                }
                if (assistantElement) {
                    const detailsId = 'process-details-' + assistantId;
                    if (!document.getElementById(detailsId)) {
                        integrateProgressToMCPSection(progressId, assistantId, typeof getMcpIds === 'function' ? (getMcpIds() || []) : []);
                    } else if (preferredMessageId) {
                        maybeReloadLazyProcessDetails(assistantId);
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
            mainIterationStateByProgressId.delete(String(progressId));
            break;
            
        case 'done':
            if (event.data && event.data.workflowStatus === 'awaiting_hitl') {
                const waitingTitle = document.querySelector(`#${progressId} .progress-title`);
                if (waitingTitle) {
                    waitingTitle.textContent = '⏸️ ' + (typeof window.t === 'function' ? window.t('chat.workflowAwaitingApproval') : '工作流等待审批');
                }
                if (progressTaskState.has(progressId)) {
                    finalizeProgressTask(progressId, typeof window.t === 'function' ? window.t('chat.workflowAwaitingApproval') : '等待审批');
                }
                break;
            }
            // 清理流式输出状态
            responseStreamStateByProgressId.delete(progressId);
            mainIterationStateByProgressId.delete(String(progressId));
            thinkingStreamStateByProgressId.delete(progressId);
            einoAgentReplyStreamStateByProgressId.delete(progressId);
            // 清理工具流式输出占位
            const prefix = String(progressId) + '::';
            for (const key of Array.from(toolResultStreamStateByKey.keys())) {
                if (String(key).startsWith(prefix)) {
                    toolResultStreamStateByKey.delete(key);
                }
            }
            if (window.csTaskReplay && window.csTaskReplay.progressId === progressId) {
                clearCsTaskReplay();
            }
            // 完成，更新进度标题（如果进度消息还存在）
            const doneTitle = document.querySelector(`#${progressId} .progress-title`);
            if (doneTitle) {
                doneTitle.textContent = '✅ ' + (typeof window.t === 'function' ? window.t('chat.penetrationTestComplete') : '渗透测试完成');
            }
            // 更新对话ID
            if (event.data && event.data.conversationId) {
                currentConversationId = event.data.conversationId;
                syncAgentLiveStreamConversationId(event.data.conversationId);
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

function renderInlineHitlApproval(itemId, data) {
    const item = document.getElementById(itemId);
    if (!item || !data || !data.interruptId) return;
    let contentEl = item.querySelector('.timeline-item-content');
    if (!contentEl) {
        // warning 等类型默认没有内容区域；HITL 内联审批需要可交互容器
        contentEl = document.createElement('div');
        contentEl.className = 'timeline-item-content';
        item.appendChild(contentEl);
    }
    const existingPanel = contentEl.querySelector('.hitl-inline-approval');
    if (existingPanel) {
        existingPanel.remove();
    }

    const payload = data.payload && typeof data.payload === 'object' ? data.payload : {};
    const toolName = data.toolName || payload.toolName || '-';
    let mode = String(data.mode || '').trim().toLowerCase();
    if (mode === 'feedback' || mode === 'followup') {
        mode = 'approval';
    }
    const allowEdit = mode === 'review_edit';
    const argsObj = payload.argumentsObj && typeof payload.argumentsObj === 'object' ? payload.argumentsObj : {};
    const argsJSON = JSON.stringify(argsObj, null, 2);

    const panel = document.createElement('div');
    panel.className = 'hitl-inline-approval';
    panel.innerHTML = `
        <div class="hitl-input-help"><strong>${escapeHtml(toolName)}</strong> 待人工审批。模式：${escapeHtml(mode || '-')}。</div>
        ${allowEdit
            ? `<div class="hitl-input-help">审查编辑参数（JSON，可选）：留空表示沿用原参数。</div>
               <textarea class="hitl-edit-args hitl-inline-edit" placeholder='{"command":"ls -la"}'>${escapeHtml(argsJSON === '{}' ? '' : argsJSON)}</textarea>`
            : '<div class="hitl-input-help">当前模式不支持改参，仅可通过/拒绝。</div>'
        }
        <div class="hitl-input-help">备注（可选）：建议写审批依据。</div>
        <input class="hitl-config-input hitl-inline-comment" type="text" placeholder="例如：允许只读命令">
        <div class="hitl-pending-actions">
            <button class="btn-secondary hitl-inline-reject">拒绝</button>
            <button class="btn-primary hitl-inline-approve">通过</button>
        </div>
        <div class="hitl-input-help hitl-inline-status"></div>
    `;
    contentEl.appendChild(panel);

    const approveBtn = panel.querySelector('.hitl-inline-approve');
    const rejectBtn = panel.querySelector('.hitl-inline-reject');
    const commentInput = panel.querySelector('.hitl-inline-comment');
    const editInput = panel.querySelector('.hitl-inline-edit');
    const statusEl = panel.querySelector('.hitl-inline-status');

    const setBusy = function (busy) {
        approveBtn.disabled = busy;
        rejectBtn.disabled = busy;
    };

    const submit = async function (decision) {
        setBusy(true);
        let editedArgs = null;
        if (allowEdit && editInput) {
            const raw = String(editInput.value || '').trim();
            if (raw) {
                try {
                    editedArgs = JSON.parse(raw);
                } catch (e) {
                    statusEl.textContent = 'JSON 参数格式错误';
                    setBusy(false);
                    return;
                }
            }
        }
        const comment = String(commentInput.value || '').trim();
        try {
            if (typeof window.submitHitlDecisionWithPayload === 'function') {
                const convFollow = data.conversationId || (typeof window.currentConversationId === 'string' ? window.currentConversationId : '');
                const ok = await window.submitHitlDecisionWithPayload(data.interruptId, decision, comment, (decision === 'approve' && allowEdit) ? editedArgs : null, convFollow);
                if (!ok) {
                    statusEl.textContent = '提交失败，请重试';
                    setBusy(false);
                    return;
                }
            } else {
                statusEl.textContent = '审批函数未加载';
                setBusy(false);
                return;
            }
            statusEl.textContent = decision === 'approve' ? '已通过，等待执行继续...' : '已拒绝，反馈已交给模型继续迭代...';
            panel.classList.add('hitl-inline-done');
        } catch (e) {
            statusEl.textContent = '提交失败：' + (e && e.message ? e.message : 'unknown error');
            setBusy(false);
        }
    };

    approveBtn.onclick = function () { submit('approve'); };
    rejectBtn.onclick = function () { submit('reject'); };
}

function renderInlineWorkflowHitlApproval(itemId, data) {
    const item = document.getElementById(itemId);
    if (!item || !data) return;
    const runId = data.workflowRunId || data.workflow_run_id;
    if (!runId) return;
    let contentEl = item.querySelector('.timeline-item-content');
    if (!contentEl) {
        contentEl = document.createElement('div');
        contentEl.className = 'timeline-item-content';
        item.appendChild(contentEl);
    }
    const existingPanel = contentEl.querySelector('.workflow-hitl-inline-approval');
    if (existingPanel) existingPanel.remove();

    const label = data.label || data.nodeId || runId;
    const prompt = data.prompt || '';
    const panel = document.createElement('div');
    panel.className = 'workflow-hitl-inline-approval hitl-inline-approval';
    panel.innerHTML = `
        <div class="hitl-input-help"><strong>${escapeHtml(label)}</strong> 等待人工审批。</div>
        ${prompt ? `<div class="hitl-input-help">${escapeHtml(prompt)}</div>` : ''}
        <div class="hitl-input-help">备注（可选）</div>
        <input class="hitl-config-input workflow-hitl-inline-comment" type="text" placeholder="审批意见">
        <div class="hitl-pending-actions">
            <button class="btn-secondary workflow-hitl-inline-reject">拒绝</button>
            <button class="btn-primary workflow-hitl-inline-approve">通过</button>
        </div>
        <div class="hitl-input-help workflow-hitl-inline-status"></div>
    `;
    contentEl.appendChild(panel);

    const approveBtn = panel.querySelector('.workflow-hitl-inline-approve');
    const rejectBtn = panel.querySelector('.workflow-hitl-inline-reject');
    const commentInput = panel.querySelector('.workflow-hitl-inline-comment');
    const statusEl = panel.querySelector('.workflow-hitl-inline-status');

    const setBusy = function (busy) {
        approveBtn.disabled = busy;
        rejectBtn.disabled = busy;
    };

    const submit = async function (approved) {
        setBusy(true);
        const comment = String(commentInput.value || '').trim();
        try {
            const fetchFn = typeof apiFetch === 'function' ? apiFetch : fetch;
            const response = await fetchFn(`/api/workflows/runs/${encodeURIComponent(runId)}/resume`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ approved: approved, comment: comment })
            });
            const body = response && typeof response.json === 'function' ? await response.json() : null;
            if (!response || !response.ok) {
                statusEl.textContent = (body && body.error) ? body.error : '提交失败，请重试';
                setBusy(false);
                return;
            }
            if (body && body.streamResuming) {
                statusEl.textContent = approved ? '已通过，工作流继续执行中…' : '已拒绝';
                panel.classList.add('hitl-inline-done');
                return;
            }
            statusEl.textContent = approved ? '已通过，工作流继续执行' : '已拒绝';
            panel.classList.add('hitl-inline-done');
        } catch (e) {
            statusEl.textContent = '提交失败：' + (e && e.message ? e.message : 'unknown error');
            setBusy(false);
        }
    };

    approveBtn.onclick = function () { submit(true); };
    rejectBtn.onclick = function () { submit(false); };
}

function parseWorkflowHitlPendingJSON(raw) {
    if (!raw) return {};
    if (typeof raw === 'object') return raw;
    try {
        const o = JSON.parse(String(raw));
        return o && typeof o === 'object' ? o : {};
    } catch (e) {
        return {};
    }
}

function workflowHitlDataFromRun(run) {
    if (!run) return null;
    const runId = run.id || run.workflowRunId || run.workflow_run_id;
    if (!runId) return null;
    const pending = parseWorkflowHitlPendingJSON(run.pending_hitl_json || run.pendingHitlJson || run.pendingHitlJSON);
    const pendingHitl = pending.pendingHitl && typeof pending.pendingHitl === 'object' ? pending.pendingHitl : pending;
    return {
        workflowRunId: String(runId),
        nodeId: pendingHitl.nodeId || run.pending_hitl_node_id || run.pendingHitlNodeId || '',
        label: pendingHitl.label || pendingHitl.nodeId || run.pending_hitl_node_id || run.pendingHitlNodeId || runId,
        prompt: pendingHitl.prompt || '',
        conversationId: run.conversation_id || run.conversationId || ''
    };
}

function findWorkflowHitlTimelineItem(detailsContainer, runId) {
    if (!detailsContainer || !runId) return null;
    const rid = String(runId).trim();
    const byRun = detailsContainer.querySelector('[data-workflow-run-id="' + hitlEscapeAttrSelector(rid) + '"]');
    if (byRun) return byRun;
    const items = detailsContainer.querySelectorAll('.timeline-item-workflow_hitl_waiting');
    for (let i = items.length - 1; i >= 0; i--) {
        const el = items[i];
        if (!el.querySelector('.workflow-hitl-inline-approval.hitl-inline-done')) {
            return el;
        }
    }
    return items.length ? items[items.length - 1] : null;
}

/**
 * 刷新或切换会话后：根据 workflow_runs(awaiting_hitl) 恢复工作流内联审批入口。
 */
async function restoreWorkflowHitlInlineForConversation(conversationId) {
    if (!conversationId || typeof apiFetch !== 'function') return;
    if (typeof window.currentConversationId === 'string' && window.currentConversationId !== conversationId) {
        return;
    }
    try {
        const resp = await apiFetch('/api/workflows/runs/pending?conversationId=' + encodeURIComponent(conversationId));
        if (!resp.ok) return;
        const data = await resp.json().catch(function () { return {}; });
        const runs = Array.isArray(data.runs) ? data.runs : [];
        if (!runs.length) return;

        let msgEl = document.querySelector('#chat-messages [data-backend-message-id]');
        const nodes = document.querySelectorAll('#chat-messages .message.assistant');
        for (let i = nodes.length - 1; i >= 0; i--) {
            if (nodes[i] && nodes[i].dataset && nodes[i].dataset.backendMessageId) {
                msgEl = nodes[i];
                break;
            }
        }
        if (!msgEl || !msgEl.id) return;
        const clientMsgId = msgEl.id;
        const backendMsgId = msgEl.dataset.backendMessageId;
        const detailsContainer = document.getElementById('process-details-' + clientMsgId);
        if (!detailsContainer) return;

        if (detailsContainer.dataset.lazyNotLoaded === '1' && detailsContainer.dataset.loaded !== '1') {
            try {
                detailsContainer.dataset.loading = '1';
                if (typeof loadProcessDetailsPaginated === 'function') {
                    await loadProcessDetailsPaginated(clientMsgId, backendMsgId);
                } else if (typeof apiFetch === 'function' && backendMsgId) {
                    const res = await apiFetch('/api/messages/' + encodeURIComponent(backendMsgId) + '/process-details?full=1');
                    const j = await res.json().catch(function () { return {}; });
                    if (res.ok && typeof renderProcessDetails === 'function') {
                        renderProcessDetails(clientMsgId, (j && Array.isArray(j.processDetails)) ? j.processDetails : []);
                    }
                }
            } catch (e) {
                console.error('加载过程详情失败（工作流 HITL 恢复）:', e);
            } finally {
                detailsContainer.dataset.loading = '0';
            }
        }

        expandProcessDetailsTimeline(clientMsgId);

        for (let i = 0; i < runs.length; i++) {
            const hitlData = workflowHitlDataFromRun(runs[i]);
            if (!hitlData) continue;
            let hitlItemEl = findWorkflowHitlTimelineItem(detailsContainer, hitlData.workflowRunId);
            if (!hitlItemEl) {
                const timeline = detailsContainer.querySelector('.progress-timeline');
                if (timeline && typeof addTimelineItem === 'function') {
                    const itemId = addTimelineItem(timeline, 'workflow_hitl_waiting', {
                        title: '🧑‍⚖️ 工作流等待审批',
                        message: hitlData.label || '',
                        data: hitlData
                    });
                    hitlItemEl = document.getElementById(itemId);
                }
            }
            if (hitlItemEl && hitlItemEl.id) {
                renderInlineWorkflowHitlApproval(hitlItemEl.id, hitlData);
            }
        }
    } catch (e) {
        console.error('restoreWorkflowHitlInlineForConversation failed', e);
    }
}

window.restoreWorkflowHitlInlineForConversation = restoreWorkflowHitlInlineForConversation;
window.submitWorkflowHitlDecision = async function submitWorkflowHitlDecision(runId, approved, comment) {
    const fetchFn = typeof apiFetch === 'function' ? apiFetch : fetch;
    const response = await fetchFn('/api/workflows/runs/' + encodeURIComponent(String(runId)) + '/resume', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ approved: !!approved, comment: comment || '' })
    });
    const body = response && typeof response.json === 'function' ? await response.json() : null;
    if (!response || !response.ok) {
        throw new Error((body && body.error) ? body.error : '提交失败');
    }
    return body;
};

function hitlEscapeAttrSelector(val) {
    const s = String(val);
    if (typeof CSS !== 'undefined' && typeof CSS.escape === 'function') {
        return CSS.escape(s);
    }
    return s.replace(/\\/g, '\\\\').replace(/"/g, '\\"');
}

function expandProcessDetailsTimeline(assistantMessageId) {
    if (!assistantMessageId) return;
    const detailsContainer = document.getElementById('process-details-' + assistantMessageId);
    if (!detailsContainer) return;
    const timeline = detailsContainer.querySelector('.progress-timeline');
    if (!timeline) return;
    timeline.classList.add('expanded');
    detailsContainer.dataset.userExpanded = '1';
    if (typeof syncProcessDetailButtonLabels === 'function') {
        syncProcessDetailButtonLabels(assistantMessageId, true);
    } else {
        const collapseT = typeof window.t === 'function' ? window.t('tasks.collapseDetail') : '收起详情';
        document.querySelectorAll('#' + hitlEscapeAttrSelector(assistantMessageId) + ' .process-detail-btn').forEach(function (btn) {
            btn.innerHTML = '<span>' + collapseT + '</span>';
        });
    }
    setTimeout(function () {
        if (window.CyberStrikeChatScroll && typeof window.CyberStrikeChatScroll.scrollIntoViewIfFollowing === 'function') {
            window.CyberStrikeChatScroll.scrollIntoViewIfFollowing(detailsContainer, { behavior: 'smooth', block: 'nearest' });
        } else if (typeof window.captureScrollPinState === 'function' ? window.captureScrollPinState() : true) {
            detailsContainer.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
        }
    }, 100);
}

function findLastAssistantMessageElInChat() {
    const nodes = document.querySelectorAll('#chat-messages .message.assistant');
    for (let i = nodes.length - 1; i >= 0; i--) {
        const el = nodes[i];
        if (el && el.dataset && el.dataset.backendMessageId) return el;
    }
    return null;
}

/**
 * 刷新或切换会话后：根据待审批记录恢复时间线里的内联审批入口，并展开详情区。
 */
async function restoreHitlInlineForConversation(conversationId) {
    if (!conversationId || typeof apiFetch !== 'function') return;
    if (typeof window.currentConversationId === 'string' && window.currentConversationId !== conversationId) {
        return;
    }
    try {
        const resp = await apiFetch('/api/hitl/pending?conversationId=' + encodeURIComponent(conversationId) + '&status=pending&pageSize=50');
        if (!resp.ok) return;
        const data = await resp.json().catch(function () { return {}; });
        const items = Array.isArray(data.items) ? data.items : [];
        for (let i = 0; i < items.length; i++) {
            const item = items[i];
            let backendMsgId = item.messageId != null ? String(item.messageId).trim() : '';
            let msgEl = null;
            if (backendMsgId) {
                msgEl = document.querySelector('#chat-messages [data-backend-message-id="' + hitlEscapeAttrSelector(backendMsgId) + '"]');
            }
            if (!msgEl) {
                msgEl = findLastAssistantMessageElInChat();
                if (msgEl && msgEl.dataset && msgEl.dataset.backendMessageId) {
                    backendMsgId = String(msgEl.dataset.backendMessageId).trim();
                }
            }
            if (!msgEl || !msgEl.id || !backendMsgId) continue;
            const clientMsgId = msgEl.id;
            const detailsContainer = document.getElementById('process-details-' + clientMsgId);
            if (!detailsContainer) continue;
            if (detailsContainer.dataset.lazyNotLoaded === '1' && detailsContainer.dataset.loaded !== '1') {
                try {
                    detailsContainer.dataset.loading = '1';
                    if (typeof loadProcessDetailsPaginated === 'function') {
                        await loadProcessDetailsPaginated(clientMsgId, backendMsgId);
                    } else {
                        const res = await apiFetch('/api/messages/' + encodeURIComponent(backendMsgId) + '/process-details?full=1');
                        const j = await res.json().catch(function () { return {}; });
                        if (!res.ok) throw new Error((j && j.error) ? j.error : String(res.status));
                        const details = (j && Array.isArray(j.processDetails)) ? j.processDetails : [];
                        if (typeof renderProcessDetails === 'function') {
                            renderProcessDetails(clientMsgId, details);
                        }
                    }
                } catch (e) {
                    console.error('加载过程详情失败（HITL 恢复）:', e);
                } finally {
                    detailsContainer.dataset.loading = '0';
                }
            }
            expandProcessDetailsTimeline(clientMsgId);
            let payloadObj = {};
            try {
                payloadObj = JSON.parse(String(item.payload || '{}'));
            } catch (e) {
                payloadObj = {};
            }
            const hitlData = {
                interruptId: item.id,
                mode: item.mode,
                toolName: item.toolName,
                toolCallId: item.toolCallId,
                payload: payloadObj,
                conversationId: item.conversationId || conversationId
            };
            let hitlItemEl = detailsContainer.querySelector('[data-hitl-interrupt-id="' + hitlEscapeAttrSelector(String(item.id)) + '"]');
            if (!hitlItemEl && item.toolCallId) {
                hitlItemEl = detailsContainer.querySelector('[data-tool-call-id="' + hitlEscapeAttrSelector(String(item.toolCallId)) + '"]');
            }
            if (!hitlItemEl && item.toolName) {
                const want = String(item.toolName).trim().toLowerCase();
                const shortWant = want.indexOf('::') >= 0 ? want.split('::').pop() : want;
                const calls = detailsContainer.querySelectorAll('.timeline-item-tool_call');
                for (let j = calls.length - 1; j >= 0; j--) {
                    const tn = String(calls[j].dataset.toolName || '').trim().toLowerCase();
                    const shortTn = tn.indexOf('::') >= 0 ? tn.split('::').pop() : tn;
                    const match = want && (tn === want || tn.endsWith('::' + shortWant) || shortTn === shortWant);
                    if (match) {
                        hitlItemEl = calls[j];
                        break;
                    }
                }
            }
            if (!hitlItemEl) continue;
            renderInlineHitlApproval(hitlItemEl.id, hitlData);
        }
        if (typeof restoreWorkflowHitlInlineForConversation === 'function') {
            await restoreWorkflowHitlInlineForConversation(conversationId);
        }
    } catch (e) {
        console.error('restoreHitlInlineForConversation failed', e);
    }
}

window.expandProcessDetailsTimeline = expandProcessDetailsTimeline;
window.restoreHitlInlineForConversation = restoreHitlInlineForConversation;

/**
 * 无 SSE 时（例如刷新页面后）：从 DB 拉取最后一条助手消息的过程详情并重绘时间线，便于审批通过后仍能看到执行进展。
 */
async function refreshLastAssistantProcessDetails(conversationId) {
    if (!conversationId || typeof apiFetch !== 'function') return;
    if (typeof window.currentConversationId === 'string' && window.currentConversationId !== conversationId) return;
    const msgEl = findLastAssistantMessageElInChat();
    if (!msgEl || !msgEl.dataset.backendMessageId || !msgEl.id) return;
    const backendId = String(msgEl.dataset.backendMessageId).trim();
    const clientId = msgEl.id;
    const detailsContainer = document.getElementById('process-details-' + clientId);
    let wasExpanded = false;
    if (detailsContainer) {
        const tl = detailsContainer.querySelector('.progress-timeline');
        wasExpanded = !!(tl && tl.classList.contains('expanded'));
    }
    try {
        // 恢复流程必须遍历全部分页。直接请求无参数接口只会返回最早 50 条，
        // 长任务刷新后会表现为“旧轮次 → 当前实时轮次”的中间历史缺失。
        if (typeof loadProcessDetailsPaginated === 'function') {
            await loadProcessDetailsPaginated(clientId, backendId);
        } else {
            const res = await apiFetch(
                '/api/messages/' + encodeURIComponent(backendId) + '/process-details?full=1'
            );
            const j = await res.json().catch(function () { return {}; });
            if (!res.ok) return;
            const details = Array.isArray(j.processDetails) ? j.processDetails : [];
            if (typeof renderProcessDetails === 'function') {
                renderProcessDetails(clientId, details);
            }
        }
        if (wasExpanded) {
            expandProcessDetailsTimeline(clientId);
        }
    } catch (e) {
        console.warn('refreshLastAssistantProcessDetails', e);
    }
}

window.refreshLastAssistantProcessDetails = refreshLastAssistantProcessDetails;

const taskEventReplayAttachState = {
    conversationId: null,
    inFlightPromise: null
};

/**
 * 订阅运行中任务的 SSE 镜像（GET /api/agent-loop/task-events），用于 HITL 通过后主连接已断开时接续 UI。
 */
async function attachRunningTaskEventStream(conversationId) {
    if (!conversationId || typeof apiFetch !== 'function') return false;
    if (
        taskEventReplayAttachState.inFlightPromise &&
        taskEventReplayAttachState.conversationId === conversationId
    ) {
        return taskEventReplayAttachState.inFlightPromise;
    }
    if (shouldSkipTaskEventReplayAttach(conversationId)) {
        return false;
    }

    const attachPromise = (async function () {
        try {
            const check = await apiFetch('/api/agent-loop/tasks');
            if (!check.ok) return false;
            const j = await check.json().catch(function () { return {}; });
            const active = (j.tasks || []).some(function (t) {
                return t && t.conversationId === conversationId && (t.status === 'running' || t.status === 'cancelling');
            });
            if (!active) return false;

            const asEl = findLastAssistantMessageElInChat();
            if (!asEl || !asEl.id) return false;
            const backendId = asEl.dataset && asEl.dataset.backendMessageId;
            if (backendId && typeof renderProcessDetails === 'function') {
                // 运行中会话可能远超默认 50 条；完整补齐数据库历史后再接实时事件。
                if (typeof loadProcessDetailsPaginated === 'function') {
                    await loadProcessDetailsPaginated(asEl.id, String(backendId));
                } else {
                    const res = await apiFetch(
                        '/api/messages/' + encodeURIComponent(String(backendId)) + '/process-details?full=1'
                    );
                    const jd = await res.json().catch(function () { return {}; });
                    if (res.ok && Array.isArray(jd.processDetails)) {
                        renderProcessDetails(asEl.id, jd.processDetails);
                    }
                }
                // 历史重绘会重建时间线节点，需重新挂载 HITL 审批入口。
                if (typeof window.restoreHitlInlineForConversation === 'function') {
                    await window.restoreHitlInlineForConversation(conversationId);
                }
            }
            expandProcessDetailsTimeline(asEl.id);

            const progressId = taskReplayProgressId(conversationId);
            beginCsTaskReplay(progressId, asEl.id, conversationId);

            if (window.CyberStrikeChatScroll && typeof window.CyberStrikeChatScroll.onTaskEventStreamBegin === 'function') {
                window.CyberStrikeChatScroll.onTaskEventStreamBegin(conversationId, asEl.id, progressId);
            }

            const url = '/api/agent-loop/task-events?conversationId=' + encodeURIComponent(conversationId);
            const response = await apiFetch(url, {
                method: 'GET',
                headers: { Accept: 'text/event-stream' }
            });
            if (!response.ok) {
                clearCsTaskReplay();
                if (progressTaskState.has(progressId)) {
                    progressTaskState.delete(progressId);
                }
                if (window.CyberStrikeChatScroll && typeof window.CyberStrikeChatScroll.onTaskEventStreamEnd === 'function') {
                    window.CyberStrikeChatScroll.onTaskEventStreamEnd();
                }
                return false;
            }

            let mcpIds = [];
            const assistantDomId = asEl.id;
            const getAssistantIdFn = function () { return assistantDomId; };
            const setAssistantIdFn = function () {};

            const reader = response.body.getReader();
            const decoder = new TextDecoder();
            let buffer = '';
            let replaySawDone = false;
            const dispatchTaskEvent = function (eventData) {
                if (eventData && eventData.type === 'done') {
                    replaySawDone = true;
                }
                handleStreamEvent(eventData, null, progressId, getAssistantIdFn, setAssistantIdFn, function () { return mcpIds; }, function (ids) { mcpIds = mergeMcpExecutionIDLists(mcpIds, ids || []); });
            };
            while (true) {
                const chunk = await reader.read();
                if (chunk.done) break;
                buffer += decoder.decode(chunk.value, { stream: true });
                const lines = buffer.split('\n');
                buffer = lines.pop() || '';
                await processSseDataLinesYielding(lines, dispatchTaskEvent);
            }
            // Flush decoder internal buffer to avoid dropping trailing partial UTF-8 bytes.
            buffer += decoder.decode();
            if (buffer.trim()) {
                const lines = buffer.split('\n');
                await processSseDataLinesYielding(lines, dispatchTaskEvent);
            }
            if (window.csTaskReplay && window.csTaskReplay.progressId === progressId) {
                clearCsTaskReplay();
            }
            if (replaySawDone && progressTaskState.has(progressId)) {
                finalizeProgressTask(progressId, typeof window.t === 'function' ? window.t('tasks.statusCompleted') : '已完成');
            }
            if (window.CyberStrikeChatScroll && typeof window.CyberStrikeChatScroll.onTaskEventStreamEnd === 'function') {
                window.CyberStrikeChatScroll.onTaskEventStreamEnd();
            }
            if (typeof loadActiveTasks === 'function') loadActiveTasks();
            if (replaySawDone && typeof window.loadConversation === 'function' && window.currentConversationId === conversationId) {
                const replayTimeline = document.getElementById('process-details-' + asEl.id + '-timeline');
                const keepExpanded = !!(replayTimeline && replayTimeline.classList.contains('expanded'));
                await window.loadConversation(conversationId);
                // loadConversation 使用轻量消息接口，会把详情重新置为懒加载状态；
                // 任务终态再从 DB 全量对账一次，补回订阅建立期间可能错过的事件。
                await refreshLastAssistantProcessDetails(conversationId);
                if (keepExpanded) {
                    const finalAssistant = findLastAssistantMessageElInChat();
                    if (finalAssistant && finalAssistant.id) {
                        expandProcessDetailsTimeline(finalAssistant.id);
                    }
                }
            }
            return true;
        } catch (e) {
            console.warn('attachRunningTaskEventStream', e);
            clearCsTaskReplay();
            if (window.CyberStrikeChatScroll && typeof window.CyberStrikeChatScroll.onTaskEventStreamEnd === 'function') {
                window.CyberStrikeChatScroll.onTaskEventStreamEnd();
            }
            return false;
        } finally {
            if (taskEventReplayAttachState.inFlightPromise === attachPromise) {
                taskEventReplayAttachState.inFlightPromise = null;
                taskEventReplayAttachState.conversationId = null;
            }
        }
    })();

    taskEventReplayAttachState.conversationId = conversationId;
    taskEventReplayAttachState.inFlightPromise = attachPromise;
    return attachPromise;
}

window.attachRunningTaskEventStream = attachRunningTaskEventStream;
window.taskReplayProgressId = taskReplayProgressId;
window.expandProcessDetailsTimeline = expandProcessDetailsTimeline;

/** 从工具参数提取短摘要（URL/命令等），便于同名工具批量调用时区分 */
function parseToolCallArgsFromData(data) {
    if (!data) return {};
    let args = data.argumentsObj;
    if (args == null && data.arguments != null && String(data.arguments).trim() !== '') {
        try {
            args = JSON.parse(String(data.arguments));
        } catch (e) {
            args = { _raw: String(data.arguments) };
        }
    }
    if (args == null || typeof args !== 'object') {
        return {};
    }
    return args;
}

function formatToolCallTimelineTitle(toolName, index, total) {
    const name = toolName || (typeof window.t === 'function' ? window.t('chat.unknownTool') : '未知工具');
    const idx = index || 0;
    const tot = total || 0;
    if (typeof window.t === 'function') {
        return window.t('chat.callTool', { name: name, index: idx, total: tot });
    }
    return '调用工具: ' + name + (tot ? ' (' + idx + '/' + tot + ')' : '');
}

function collectToolResultTextParts(value, parts, depth) {
    if (value == null || depth > 4) return;
    if (typeof value === 'string') {
        parts.push(value);
        return;
    }
    if (typeof value !== 'object') return;
    if (Array.isArray(value)) {
        value.forEach(function (v) { collectToolResultTextParts(v, parts, depth + 1); });
        return;
    }
    if (typeof value.text === 'string') parts.push(value.text);
    if (typeof value.result === 'string') parts.push(value.result);
    if (typeof value.error === 'string') parts.push(value.error);
    if (value.content != null) collectToolResultTextParts(value.content, parts, depth + 1);
}

function getToolResultDisplayState(data, opts) {
    opts = opts || {};
    data = data || {};
    const toolName = String(data.toolName || data.name || '').trim().toLowerCase();
    const isObservationTool = toolName === 'wait_tool_execution' || toolName === 'get_tool_execution';
    const explicitStatus = String(data.displayStatus || data.status || '').toLowerCase();
    if (explicitStatus === 'background_running') {
        if (isObservationTool) {
            return { kind: 'success', isError: false, success: true };
        }
        return { kind: 'background_running', isError: false, success: false };
    }
    const parts = [];
    if (opts.rawText != null) parts.push(String(opts.rawText));
    collectToolResultTextParts(data.result, parts, 0);
    collectToolResultTextParts(data.error, parts, 0);
    collectToolResultTextParts(data.content, parts, 0);
    if (data.executionId != null) parts.push('execution_id: ' + String(data.executionId));
    if (data.status != null) parts.push('status: ' + String(data.status));
    const text = parts.join('\n');
    const errorLike = data.isError === true || data.success === false;
    const hasExecutionId = !!data.executionId ||
        /execution[_-]?id\\?["']?\s*[:=]\s*\\?["']?[0-9a-f]{8}-[0-9a-f-]{12,}/i.test(text);
    const hasRunningStatus = /status\\?["']?\s*[:=]\s*\\?["']?(running|queued)\\?["']?/i.test(text) ||
        /\bstatus:\s*(running|queued)\b/i.test(text);
    const hasSoftWaitSignal = /(工具已提交到后台执行|本次等待已到达|wait_timeout|wait timeout|background execution|后台执行|仍未完成|still running)/i.test(text);
    if (errorLike && hasExecutionId && hasRunningStatus && hasSoftWaitSignal) {
        if (isObservationTool) {
            return { kind: 'success', isError: false, success: true };
        }
        return { kind: 'background_running', isError: false, success: false };
    }
    return { kind: errorLike ? 'error' : 'success', isError: errorLike, success: !errorLike };
}

function getBackgroundRunningToolLabel() {
    if (typeof window.t === 'function') {
        const translated = window.t('timeline.backgroundRunning');
        if (translated && translated !== 'timeline.backgroundRunning') return translated;
    }
    return '后台执行中';
}

function buildToolResultSectionHtml(data, opts) {
    opts = opts || {};
    const _t = function (k, o) {
        return typeof window.t === 'function' ? window.t(k, o) : k;
    };
    const execResultLabel = _t('timeline.executionResult');
    const execIdLabel = _t('timeline.executionId');
    const waitingLabel = _t('timeline.running');
    if (opts.pending) {
        return (
            '<div class="tool-result-section pending">' +
            '<strong data-i18n="timeline.executionResult">' + escapeHtml(execResultLabel) + '</strong>' +
            '<pre class="tool-result tool-result-pending">' + escapeHtml(waitingLabel) + '</pre>' +
            '</div>'
        );
    }
    const noResultText = _t('timeline.noResult');
    const result = data.result != null ? data.result : (data.error != null ? data.error : (data.resultPreview != null ? data.resultPreview : noResultText));
    const resultStr = typeof result === 'string' ? result : JSON.stringify(result);
    const rawText = opts.rawText != null ? String(opts.rawText) : resultStr;
    const displayState = getToolResultDisplayState(data, { rawText: rawText });
    const sectionClass = displayState.kind === 'background_running' ? 'pending' : (displayState.isError ? 'error' : 'success');
    return (
        '<div class="tool-result-section ' + sectionClass + '">' +
        '<strong data-i18n="timeline.executionResult">' + escapeHtml(execResultLabel) + '</strong>' +
        '<pre class="tool-result">' + escapeHtml(rawText) + '</pre>' +
        (data.executionId ? '<div class="tool-execution-id"><span data-i18n="timeline.executionId">' +
            escapeHtml(execIdLabel) + '</span> <code>' + escapeHtml(String(data.executionId)) + '</code></div>' : '') +
        '</div>'
    );
}

const toolCallDetailStateByItemId = new Map();

function getToolCallDetailState(item) {
    if (!item || !item.id) return {};
    return Object.assign({}, toolCallDetailStateByItemId.get(item.id) || {});
}

function setToolCallDetailState(item, state) {
    if (!item || !item.id) return;
    toolCallDetailStateByItemId.set(item.id, Object.assign({}, state || {}));
    item.classList.add('tool-detail-collapsible');
    item.classList.add('tool-call-collapsible');
    updateToolDetailToggleLabel(item);
}

window.getToolCallDetailState = getToolCallDetailState;

function toolDetailToggleText(expanded) {
    if (typeof window.t === 'function') {
        return expanded
            ? window.t('chat.collapseToolDetail')
            : window.t('chat.viewToolDetail');
    }
    return expanded ? '收起' : '查看详情';
}

function updateToolDetailToggleLabel(item) {
    if (!item) return;
    const header = item.querySelector('.timeline-item-header');
    if (!header) return;
    const expanded = item.classList.contains('tool-call-detail-expanded');
    header.setAttribute('data-tool-detail-label', toolDetailToggleText(expanded) + (expanded ? ' ▴' : ' ▾'));
}

async function fetchFullProcessDetailData(detailId) {
    const id = detailId != null ? String(detailId).trim() : '';
    if (!id || typeof apiFetch !== 'function') return null;
    const res = await apiFetch('/api/process-details/' + encodeURIComponent(id));
    const j = await res.json().catch(() => ({}));
    if (!res.ok) {
        throw new Error((j && j.error) ? j.error : String(res.status));
    }
    const detail = j && j.processDetail ? j.processDetail : null;
    return detail && detail.data ? detail.data : null;
}

function collapseOtherToolCallDetails(item) {
    const timeline = item && item.closest ? item.closest('.progress-timeline') : null;
    if (!timeline) return;
    timeline.querySelectorAll('.timeline-item.tool-detail-collapsible.tool-call-detail-expanded').forEach(function (other) {
        if (other === item) return;
        other.classList.remove('tool-call-detail-expanded');
        const content = other.querySelector('.timeline-item-content.tool-call-detail-content');
        if (content) content.remove();
        updateToolDetailToggleLabel(other);
    });
}

async function renderToolCallDetailContent(item) {
    if (!item || !item.id) return;
    const state = toolCallDetailStateByItemId.get(item.id) || {};
    collapseOtherToolCallDetails(item);

    let content = item.querySelector('.timeline-item-content.tool-call-detail-content');
    if (content) {
        content.remove();
        item.classList.remove('tool-call-detail-expanded');
        updateToolDetailToggleLabel(item);
        return;
    }

    content = document.createElement('div');
    content.className = 'timeline-item-content tool-call-detail-content';
    if (state.payloadDeferred && !state.payloadLoaded && (state.processDetailId || state.resultDetailId)) {
        content.innerHTML = '<div class="progress-timeline-empty">' +
            escapeHtml(typeof window.t === 'function' ? window.t('common.loading') : '加载中…') +
            '</div>';
        item.appendChild(content);
        item.classList.add('tool-call-detail-expanded');
        updateToolDetailToggleLabel(item);
        try {
            if (state.processDetailId && !state.hideArgs) {
                const fullCall = await fetchFullProcessDetailData(state.processDetailId);
                if (fullCall) {
                    state.args = parseToolCallArgsFromData(fullCall);
                    state.payloadDeferred = false;
                }
            }
            if (state.resultDetailId) {
                const fullResult = await fetchFullProcessDetailData(state.resultDetailId);
                if (fullResult) {
                    state.resultData = fullResult;
                    const noResultText = typeof window.t === 'function' ? window.t('timeline.noResult') : '无结果';
                    const result = fullResult.result != null ? fullResult.result : (fullResult.error != null ? fullResult.error : noResultText);
                    state.rawText = typeof result === 'string' ? result : JSON.stringify(result);
                }
            }
            state.payloadLoaded = true;
            setToolCallDetailState(item, state);
            content.remove();
            item.classList.remove('tool-call-detail-expanded');
            renderToolCallDetailContent(item);
        } catch (e) {
            content.innerHTML = '<div class="progress-timeline-empty">' + escapeHtml(e && e.message ? e.message : '加载失败') + '</div>';
        }
        return;
    }

    const args = state.args != null ? state.args : {};
    let resultBlock = '';
    if (state.resultData) {
        resultBlock = '<div class="tool-details tool-result-slot">' +
            buildToolResultSectionHtml(state.resultData, { rawText: state.rawText }) +
            '</div>';
    } else if (state.pending !== false) {
        resultBlock = '<div class="tool-details tool-result-slot">' +
            buildToolResultSectionHtml({}, { pending: true }) +
            '</div>';
    }

    const paramsLabel = typeof window.t === 'function' ? window.t('timeline.params') : '参数:';
    const argsBlock = state.hideArgs ? '' :
        '<div class="tool-arg-section">' +
        '<strong data-i18n="timeline.params">' + escapeHtml(paramsLabel) + '</strong>' +
        '<pre class="tool-args">' + escapeHtml(JSON.stringify(args, null, 2)) + '</pre>' +
        '</div>';
    content.innerHTML = '<div class="tool-details">' + argsBlock + resultBlock + '</div>';
    item.appendChild(content);
    item.classList.add('tool-call-detail-expanded');
    updateToolDetailToggleLabel(item);
}

if (typeof document !== 'undefined' && !document.__cyberStrikeToolCallDetailToggleBound) {
    document.__cyberStrikeToolCallDetailToggleBound = true;
    document.addEventListener('click', function (event) {
        const target = event.target;
        if (target && target.closest && target.closest('button, a, input, textarea, select, pre, code, .timeline-item-content')) {
            return;
        }
        const item = target && target.closest
            ? target.closest('.timeline-item.tool-detail-collapsible')
            : null;
        if (!item) return;
        renderToolCallDetailContent(item);
    });
}

function ensureToolCallResultSlot(item) {
    if (!item) return null;
    if (item.classList.contains('tool-call-collapsible')) {
        const state = toolCallDetailStateByItemId.get(item.id) || {};
        state.pending = true;
        setToolCallDetailState(item, state);
        if (!item.classList.contains('tool-call-detail-expanded')) return null;
    }
    let section = item.querySelector('.tool-result-section');
    if (section) return section;
    const content = item.querySelector('.timeline-item-content');
    if (!content) return null;
    const wrap = document.createElement('div');
    wrap.className = 'tool-details tool-result-slot';
    wrap.innerHTML = buildToolResultSectionHtml({}, { pending: true });
    content.appendChild(wrap);
    return wrap.querySelector('.tool-result-section');
}

function mergeToolResultIntoCallItem(item, data, options) {
    if (!item || !data) return false;
    options = options || {};
    const noResultText = typeof window.t === 'function' ? window.t('timeline.noResult') : '无结果';
    const result = data.result != null ? data.result : (data.error != null ? data.error : (data.resultPreview != null ? data.resultPreview : noResultText));
    const resultStr = typeof result === 'string' ? result : JSON.stringify(result);
    const text = options.rawText != null ? String(options.rawText) : resultStr;
    const displayState = getToolResultDisplayState(data, { rawText: text });
    const backgroundRunning = displayState.kind === 'background_running';

    if (item.classList.contains('tool-call-collapsible')) {
        const state = toolCallDetailStateByItemId.get(item.id) || {};
        state.resultData = data;
        state.rawText = text;
        state.resultDetailId = data.processDetailId || state.resultDetailId || '';
        state.pending = false;
        state.payloadDeferred = data._payloadDeferred === true;
        state.payloadLoaded = data._payloadDeferred !== true;
        setToolCallDetailState(item, state);
        const expanded = item.classList.contains('tool-call-detail-expanded');
        const content = item.querySelector('.timeline-item-content.tool-call-detail-content');
        if (content) content.remove();
        if (expanded) {
            item.classList.remove('tool-call-detail-expanded');
            renderToolCallDetailContent(item);
        }
        item.dataset.toolResultMerged = '1';
        item.dataset.toolSuccess = (!displayState.isError && !backgroundRunning) ? '1' : '0';
        item.dataset.toolDisplayStatus = backgroundRunning ? 'background_running' : (displayState.isError ? 'failed' : 'completed');
        item.classList.remove('tool-call-running', 'tool-call-completed', 'tool-call-failed');
        item.classList.add(backgroundRunning ? 'tool-call-running' : (displayState.isError ? 'tool-call-failed' : 'tool-call-completed'));
        return true;
    }

    let section = item.querySelector('.tool-result-section');
    if (!section) {
        ensureToolCallResultSlot(item);
        section = item.querySelector('.tool-result-section');
    }
    if (!section) return false;

    section.classList.remove('pending');
    section.className = 'tool-result-section ' + (backgroundRunning ? 'pending' : (displayState.isError ? 'error' : 'success'));
    const pre = section.querySelector('pre.tool-result');
    if (pre) {
        pre.classList.remove('tool-result-pending');
        flushStreamPlainTextUpdate(pre);
        pre.textContent = text;
        resetStreamPlainTextState(pre);
    }

    if (data.executionId) {
        let execIdEl = section.querySelector('.tool-execution-id');
        if (!execIdEl) {
            const execIdLabel = typeof window.t === 'function' ? window.t('timeline.executionId') : '执行ID:';
            execIdEl = document.createElement('div');
            execIdEl.className = 'tool-execution-id';
            execIdEl.innerHTML = '<span data-i18n="timeline.executionId">' + escapeHtml(execIdLabel) +
                '</span> <code></code>';
            section.appendChild(execIdEl);
        }
        const code = execIdEl.querySelector('code');
        if (code) code.textContent = String(data.executionId);
    }

    item.dataset.toolResultMerged = '1';
    item.dataset.toolSuccess = (!displayState.isError && !backgroundRunning) ? '1' : '0';
    item.dataset.toolDisplayStatus = backgroundRunning ? 'background_running' : (displayState.isError ? 'failed' : 'completed');
    item.classList.remove('tool-call-running', 'tool-call-completed', 'tool-call-failed');
    item.classList.add(backgroundRunning ? 'tool-call-running' : (displayState.isError ? 'tool-call-failed' : 'tool-call-completed'));
    return true;
}

function findToolCallItemById(root, toolCallId) {
    if (!root || !toolCallId) return null;
    const id = String(toolCallId).trim();
    if (!id) return null;
    try {
        return root.querySelector('[data-tool-call-id="' + CSS.escape(id) + '"]');
    } catch (e) {
        return root.querySelector('[data-tool-call-id="' + id.replace(/"/g, '\\"') + '"]');
    }
}

function attachToolResultToCall(progressId, toolCallId, data, options) {
    if (!toolCallId || !data) return false;
    const mapping = getToolCallMapping(progressId, toolCallId);
    let item = null;
    if (mapping && mapping.itemId) {
        item = document.getElementById(mapping.itemId);
    }
    if (!item && mapping && mapping.timeline) {
        item = findToolCallItemById(mapping.timeline, toolCallId);
    }
    if (!item && progressId) {
        const progressRoot = document.getElementById(String(progressId));
        if (progressRoot) {
            item = findToolCallItemById(progressRoot, toolCallId);
        }
    }
    if (!item) return false;
    mergeToolResultIntoCallItem(item, data, options);
    return true;
}

function coalesceProcessDetailsToolPairs(details) {
    if (!Array.isArray(details) || details.length === 0) return details;
    const callsById = new Map();
    const fifoCalls = [];
    const out = [];

    function absorbResult(targetDetail, resultDetail) {
        const rd = resultDetail.data || {};
        targetDetail.data = targetDetail.data || {};
        targetDetail.data._mergedResult = Object.assign({}, rd);
        if (resultDetail.id) {
            targetDetail.data._mergedResultDetailId = resultDetail.id;
        }
        if (resultDetail.createdAt) {
            targetDetail.data._mergedResultAt = resultDetail.createdAt;
        }
    }

    for (let i = 0; i < details.length; i++) {
        const detail = details[i];
        const et = detail.eventType || '';
        const data = detail.data || {};
        const id = data.toolCallId != null ? String(data.toolCallId).trim() : '';

        if (et === 'tool_call') {
            const copy = {
                id: detail.id,
                eventType: detail.eventType,
                message: detail.message,
                createdAt: detail.createdAt,
                data: Object.assign({}, data)
            };
            if (id) callsById.set(id, copy);
            fifoCalls.push(copy);
            out.push(copy);
        } else         if (et === 'tool_result') {
            let target = null;
            if (id && callsById.has(id)) {
                target = callsById.get(id);
            } else {
                for (let j = 0; j < fifoCalls.length; j++) {
                    const c = fifoCalls[j];
                    if (c && c.data && !c.data._mergedResult) {
                        target = c;
                        break;
                    }
                }
            }
            if (target) {
                // agentFacing 或较新的 tool_result 覆盖旧合并（历史数据可能含 reduction 前全量正文）
                const prev = target.data._mergedResult;
                if (prev && data.agentFacing !== true && prev.agentFacing === true) {
                    out.push(detail);
                    continue;
                }
                absorbResult(target, detail);
                continue;
            }
            out.push(detail);
        } else {
            out.push(detail);
        }
    }
    return out;
}

window.coalesceProcessDetailsToolPairs = coalesceProcessDetailsToolPairs;
window.attachToolResultToCall = attachToolResultToCall;
window.mergeToolResultIntoCallItem = mergeToolResultIntoCallItem;
window.formatToolCallTimelineTitle = formatToolCallTimelineTitle;
window.parseToolCallArgsFromData = parseToolCallArgsFromData;
window.getToolResultDisplayState = getToolResultDisplayState;
window.getBackgroundRunningToolLabel = getBackgroundRunningToolLabel;
window.buildToolResultSectionHtml = buildToolResultSectionHtml;

// 更新工具调用状态
function updateToolCallStatus(progressId, toolCallId, status) {
    const mapping = getToolCallMapping(progressId, toolCallId);
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
    const backgroundRunningLabel = getBackgroundRunningToolLabel();
    let statusText = '';
    if (status === 'running' || status === 'background_running') {
        item.classList.add('tool-call-running');
        statusText = ' <span class="tool-status-badge tool-status-running">' +
            escapeHtml(status === 'background_running' ? backgroundRunningLabel : runningLabel) +
            '</span>';
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
function buildWorkflowConditionResultHtml(data) {
    const output = (data && data.output) || {};
    const expr = (data && data.expression) || output.condition || '';
    const matched = (data && (data.matched === true || data.matched === 'true'))
        || output.matched === true || output.matched === 'true';
    const branchText = matched ? '是（true）' : '否（false）';
    const branchClass = matched ? 'is-true' : 'is-false';
    return `<div class="timeline-item-content workflow-condition-result">
        <div class="workflow-condition-row">
            <span class="workflow-agent-io-label">表达式</span>
            <code>${escapeHtml(String(expr || '（空）'))}</code>
        </div>
        <div class="workflow-condition-row">
            <span class="workflow-agent-io-label">结果</span>
            <span class="workflow-condition-branch ${branchClass}">${escapeHtml(branchText)}</span>
        </div>
    </div>`;
}

function buildWorkflowBranchDetailHtml(data) {
    const cond = (data && data.edgeCondition) || '';
    if (!cond) return '';
    return `<div class="timeline-item-content workflow-branch-detail">
        <span class="workflow-agent-io-label">连线条件</span>
        <code>${escapeHtml(cond)}</code>
    </div>`;
}

function isLiveProgressTimeline(timeline) {
    return !!(timeline && timeline.id && /^progress-\d+-\d+-timeline$/.test(timeline.id));
}

function formatLiveTimelinePrunedMarker(marker) {
    const count = parseInt(marker.dataset.prunedCount || '0', 10) || 0;
    const minIteration = parseInt(marker.dataset.prunedMainIterationMin || '0', 10) || 0;
    const maxIteration = parseInt(marker.dataset.prunedMainIterationMax || '0', 10) || 0;
    const translate = typeof window.t === 'function' ? window.t : null;
    const baseText = translate
        ? translate('chat.liveTimelinePruned', { count: count })
        : ('已收起前 ' + count + ' 条实时过程详情，任务完成后可按页查看完整记录');
    if (minIteration <= 0 || maxIteration < minIteration) return baseText;
    if (minIteration === maxIteration) {
        return baseText + ' · ' + (translate
            ? translate('chat.liveTimelinePrunedSingleRound', { n: minIteration })
            : ('主代理第 ' + minIteration + ' 轮'));
    }
    return baseText + ' · ' + (translate
        ? translate('chat.liveTimelinePrunedRoundRange', { from: minIteration, to: maxIteration })
        : ('主代理第 ' + minIteration + '–' + maxIteration + ' 轮'));
}

function pruneLiveTimelineIfNeeded(timeline) {
    if (!isLiveProgressTimeline(timeline)) return;
    const items = Array.from(timeline.children).filter(function (el) {
        return el && el.classList && el.classList.contains('timeline-item');
    });
    if (items.length <= LIVE_TIMELINE_MAX_ITEMS) return;

    let marker = timeline.querySelector('.timeline-live-pruned-marker');
    let pruned = marker ? (parseInt(marker.dataset.prunedCount || '0', 10) || 0) : 0;
    let removable = items.filter(function (el) {
        return !el.dataset.hitlInterruptId &&
            !el.dataset.workflowRunId &&
            !el.classList.contains('timeline-item-workflow_hitl_waiting') &&
            !el.classList.contains('timeline-item-hitl_interrupt');
    });
    const overflow = items.length - LIVE_TIMELINE_MAX_ITEMS;
    const removeCount = Math.min(removable.length, Math.max(overflow, LIVE_TIMELINE_PRUNE_CHUNK));
    if (removeCount <= 0) return;

    if (!marker) {
        marker = document.createElement('div');
        marker.className = 'timeline-live-pruned-marker';
        marker.setAttribute('role', 'status');
        marker.setAttribute('aria-live', 'polite');
        timeline.insertBefore(marker, timeline.firstChild);
    }
    let prunedMainIterationMin = parseInt(marker.dataset.prunedMainIterationMin || '0', 10) || 0;
    let prunedMainIterationMax = parseInt(marker.dataset.prunedMainIterationMax || '0', 10) || 0;
    for (let i = 0; i < removeCount; i++) {
        const removed = removable[i];
        if (removed && removed.dataset && removed.dataset.timelineType === 'iteration' &&
            removed.dataset.einoScope !== 'sub') {
            const iteration = parseInt(removed.dataset.iterationN || '0', 10) || 0;
            if (iteration > 0) {
                if (prunedMainIterationMin === 0 || iteration < prunedMainIterationMin) {
                    prunedMainIterationMin = iteration;
                }
                if (iteration > prunedMainIterationMax) {
                    prunedMainIterationMax = iteration;
                }
            }
        }
        removed.remove();
    }
    pruned += removeCount;
    marker.dataset.prunedCount = String(pruned);
    marker.dataset.prunedMainIterationMin = String(prunedMainIterationMin);
    marker.dataset.prunedMainIterationMax = String(prunedMainIterationMax);
    marker.textContent = formatLiveTimelinePrunedMarker(marker);
    // 始终放在已保留详情之前；配合 sticky 样式，查看最新内容时也能感知历史已裁剪。
    if (timeline.firstChild !== marker) {
        timeline.insertBefore(marker, timeline.firstChild);
    }
}

function addTimelineItem(timeline, type, options) {
    const item = document.createElement('div');
    // 生成唯一ID
    const itemId = 'timeline-item-' + Date.now() + '-' + Math.random().toString(36).substr(2, 9);
    item.id = itemId;
    item.className = `timeline-item timeline-item-${type}`;
    if (type === 'eino_run_retry') {
        item.classList.add('timeline-item-warning');
    }
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
    if (type === 'tool_calls_detected' && options.data && options.data.count != null) {
        item.dataset.toolCallsCount = String(options.data.count);
    }
    if (type === 'tool_call' && options.data) {
        const d = options.data;
        if (options.processDetailId) {
            item.dataset.processDetailId = String(options.processDetailId);
        }
        item.dataset.toolName = (d.toolName != null && d.toolName !== '') ? String(d.toolName) : '';
        item.dataset.toolIndex = (d.index != null) ? String(d.index) : '0';
        item.dataset.toolTotal = (d.total != null) ? String(d.total) : '0';
        if (d.toolCallId != null && String(d.toolCallId).trim() !== '') {
            item.dataset.toolCallId = String(d.toolCallId).trim();
        }
        const merged = options.mergedResult || d._mergedResult;
        const mergedDisplayState = merged ? getToolResultDisplayState(merged) : null;
        const mergedBackgroundRunning = mergedDisplayState && mergedDisplayState.kind === 'background_running';
        const terminalStatus = String(options.toolStatus || '').toLowerCase();
        if (merged) {
            item.dataset.toolResultMerged = '1';
            item.dataset.toolSuccess = (!mergedDisplayState.isError && !mergedBackgroundRunning) ? '1' : '0';
            item.dataset.toolDisplayStatus = mergedBackgroundRunning ? 'background_running' : (mergedDisplayState.isError ? 'failed' : 'completed');
            item.classList.add(mergedBackgroundRunning ? 'tool-call-running' : (mergedDisplayState.isError ? 'tool-call-failed' : 'tool-call-completed'));
            if (d._mergedResultDetailId) {
                item.dataset.toolResultDetailId = String(d._mergedResultDetailId);
            }
        } else if (terminalStatus === 'completed' || terminalStatus === 'failed') {
            item.dataset.toolSuccess = terminalStatus === 'completed' ? '1' : '0';
            item.classList.add(terminalStatus === 'completed' ? 'tool-call-completed' : 'tool-call-failed');
        } else if (terminalStatus === 'result_missing') {
            item.classList.add('tool-call-incomplete');
            item.title = typeof window.t === 'function' ? window.t('timeline.resultMissing') : '结果记录缺失';
        }
    }
    if (type === 'hitl_interrupt' && options.data && options.data.interruptId != null && String(options.data.interruptId).trim() !== '') {
        item.dataset.hitlInterruptId = String(options.data.interruptId).trim();
    }
    if (type === 'workflow_hitl_waiting' && options.data) {
        const runId = options.data.workflowRunId || options.data.workflow_run_id;
        if (runId != null && String(runId).trim() !== '') {
            item.dataset.workflowRunId = String(runId).trim();
        }
    }
    if (type === 'tool_result' && options.data) {
        const d = options.data;
        const noResultText = typeof window.t === 'function' ? window.t('timeline.noResult') : '无结果';
        const result = d.result != null ? d.result : (d.error != null ? d.error : (d.resultPreview != null ? d.resultPreview : noResultText));
        const resultStr = typeof result === 'string' ? result : JSON.stringify(result);
        const displayState = getToolResultDisplayState(d, { rawText: resultStr });
        if (options.processDetailId) {
            item.dataset.processDetailId = String(options.processDetailId);
        }
        item.dataset.toolName = (d.toolName != null && d.toolName !== '') ? String(d.toolName) : '';
        item.dataset.toolSuccess = (!displayState.isError && displayState.kind !== 'background_running') ? '1' : '0';
        item.dataset.toolDisplayStatus = displayState.kind === 'background_running' ? 'background_running' : (displayState.isError ? 'failed' : 'completed');
    }
    if (options.data && options.data.einoAgent != null && String(options.data.einoAgent).trim() !== '') {
        item.dataset.einoAgent = String(options.data.einoAgent).trim();
    }
    if (options.data && options.data.orchestration != null && String(options.data.orchestration).trim() !== '') {
        item.dataset.orchestration = String(options.data.orchestration).trim();
    }
    if (options.data && options.data.responseStreamPlaceholder === true) {
        item.dataset.responseStreamPlaceholder = '1';
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
    if ((type === 'thinking' || type === 'reasoning_chain' || type === 'planning') && options.message) {
        const streamBody = typeof formatTimelineStreamBody === 'function'
            ? formatTimelineStreamBody(options.message, options.data)
            : options.message;
        content += `<div class="timeline-item-content timeline-stream-plain">${formatTimelinePlainTextHtml(streamBody)}</div>`;
    } else if (type === 'tool_call' && options.data) {
        const data = options.data;
        const args = parseToolCallArgsFromData(data);
        const merged = options.mergedResult || data._mergedResult;
        const mergedDisplayState = merged ? getToolResultDisplayState(merged) : null;
        const mergedBackgroundRunning = mergedDisplayState && mergedDisplayState.kind === 'background_running';
        const terminalStatus = String(options.toolStatus || '').toLowerCase();
        const hasTerminalStatus = terminalStatus === 'completed' || terminalStatus === 'failed';
        const hasHistoricalStatus = hasTerminalStatus || terminalStatus === 'result_missing';
        if (merged) {
            item.classList.add(mergedBackgroundRunning ? 'tool-call-running' : (mergedDisplayState.isError ? 'tool-call-failed' : 'tool-call-completed'));
        } else if (hasTerminalStatus) {
            item.classList.add(terminalStatus === 'completed' ? 'tool-call-completed' : 'tool-call-failed');
        } else if (terminalStatus === 'result_missing') {
            item.classList.add('tool-call-incomplete');
        } else if (!options.skipPendingResult) {
            item.classList.add('tool-call-running');
        }
        setToolCallDetailState(item, {
            args: args,
            resultData: merged || null,
            pending: !merged && !hasHistoricalStatus && !options.skipPendingResult,
            processDetailId: options.processDetailId || '',
            resultDetailId: data._mergedResultDetailId || '',
            payloadDeferred: data._payloadDeferred === true || (merged && merged._payloadDeferred === true),
            payloadLoaded: !(data._payloadDeferred === true || (merged && merged._payloadDeferred === true))
        });
    } else if ((type === 'eino_agent_reply' || type === 'workflow_agent_output') && options.message) {
        let prefix = '';
        if (type === 'workflow_agent_output' && options.data) {
            const source = options.data.inputSource || '';
            const preview = options.data.inputPreview || '';
            if (source || preview) {
                const previewText = String(preview || '').trim();
                const summaryPreview = previewText.length > 80 ? (previewText.slice(0, 80) + '...') : previewText;
                prefix = `<details class="workflow-agent-input">
                    <summary>
                        <span class="workflow-agent-io-label">输入</span>
                        ${source ? `<code>${escapeHtml(source)}</code>` : ''}
                        ${summaryPreview ? `<span class="workflow-agent-input-summary">${escapeHtml(summaryPreview)}</span>` : ''}
                    </summary>
                    ${previewText ? `<pre>${escapeHtml(previewText)}</pre>` : '<div class="workflow-agent-empty">暂无输入预览</div>'}
                </details>`;
            }
        }
        const body = type === 'workflow_agent_output'
            ? `<div class="workflow-agent-output">
                <div class="workflow-agent-io-label">输出</div>
                <div class="workflow-agent-output-body">${formatTimelinePlainTextHtml(options.message)}</div>
            </div>`
            : formatTimelinePlainTextHtml(options.message);
        content += `<div class="timeline-item-content workflow-agent-io">${prefix}${body}</div>`;
    } else if (type === 'workflow_node_result' && options.data && String(options.data.nodeType || '').toLowerCase() === 'condition') {
        content += buildWorkflowConditionResultHtml(options.data);
    } else if ((type === 'workflow_branch_taken' || type === 'workflow_branch_skipped') && options.data) {
        content += buildWorkflowBranchDetailHtml(options.data);
    } else if (type === 'tool_result' && options.data) {
        const data = options.data;
        const noResultText = typeof window.t === 'function' ? window.t('timeline.noResult') : '无结果';
        const result = data.result || data.error || data.resultPreview || noResultText;
        const resultStr = typeof result === 'string' ? result : JSON.stringify(result);
        const displayState = getToolResultDisplayState(data, { rawText: resultStr });
        setToolCallDetailState(item, {
            resultData: data,
            rawText: resultStr,
            pending: false,
            hideArgs: true,
            processDetailId: options.processDetailId || '',
            resultDetailId: options.processDetailId || '',
            payloadDeferred: data._payloadDeferred === true,
            payloadLoaded: data._payloadDeferred !== true
        });
        item.dataset.toolDisplayStatus = displayState.kind === 'background_running' ? 'background_running' : (displayState.isError ? 'failed' : 'completed');
        item.classList.add(displayState.kind === 'background_running' ? 'tool-call-running' : (displayState.isError ? 'tool-call-failed' : 'tool-call-completed'));
    } else if (type === 'cancelled') {
        const taskCancelledLabel = typeof window.t === 'function' ? window.t('chat.taskCancelled') : '任务已取消';
        content += `
            <div class="timeline-item-content">
                ${escapeHtml(options.message || taskCancelledLabel)}
            </div>
        `;
    } else if ((type === 'warning' || type === 'eino_run_retry') && options.message) {
        const streamBody = typeof formatTimelineStreamBody === 'function'
            ? formatTimelineStreamBody(options.message, options.data)
            : options.message;
        content += `<div class="timeline-item-content timeline-stream-plain">${formatTimelinePlainTextHtml(streamBody)}</div>`;
    } else if (type === 'progress' && options.message) {
        content += `<div class="timeline-item-content timeline-eino-trace"><pre class="tool-result">${escapeHtml(options.message)}</pre></div>`;
    } else if (type === 'user_interrupt_continue' && options.message) {
        const streamBody = typeof formatTimelineStreamBody === 'function'
            ? formatTimelineStreamBody(options.message, options.data)
            : options.message;
        content += `<div class="timeline-item-content timeline-stream-plain">${formatTimelinePlainTextHtml(streamBody)}</div>`;
    }

    item.innerHTML = content;
    if (item.classList.contains('tool-detail-collapsible')) {
        updateToolDetailToggleLabel(item);
    }
    if (options.data) {
        applyEinoTimelineRole(item, options.data);
    }
    timeline.appendChild(item);
    pruneLiveTimelineIfNeeded(timeline);
    
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

function getActiveTaskDisplayName(task) {
    const _t = function (k) { return typeof window.t === 'function' ? window.t(k) : k; };
    const unnamedTaskText = _t('tasks.unnamedTask');
    if (!task) return unnamedTaskText;
    const title = (task.title || '').trim();
    if (title) return title;
    const message = (task.message || '').trim();
    return message || unnamedTaskText;
}

function updateActiveTaskConversationTitle(conversationId, newTitle) {
    const bar = document.getElementById('active-tasks-bar');
    if (!bar || !conversationId) return;
    const title = (newTitle || '').trim();
    if (!title) return;
    bar.querySelectorAll('.active-task-item[data-conversation-id="' + conversationId + '"] .active-task-message')
        .forEach(function (el) {
            el.textContent = title;
        });
}
window.updateActiveTaskConversationTitle = updateActiveTaskConversationTitle;

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

    function openActiveTaskConversation(conversationId) {
        if (!conversationId) return;
        if (typeof switchPage === 'function') {
            switchPage('chat');
        }
        if (typeof window.loadConversation === 'function') {
            setTimeout(function () {
                window.loadConversation(conversationId);
            }, 120);
            return;
        }
        window.location.hash = 'chat?conversation=' + encodeURIComponent(conversationId);
    }

    normalizedTasks.forEach(task => {
        const item = document.createElement('div');
        item.className = 'active-task-item active-task-item-clickable';
        if (task && task.conversationId) {
            item.title = (typeof window.t === 'function' ? window.t('tasks.viewConversation') : '查看会话');
            item.setAttribute('role', 'button');
            item.onclick = () => openActiveTaskConversation(task.conversationId);
        }

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
        const taskDisplayName = getActiveTaskDisplayName(task);
        const stopTaskBtnText = _t('tasks.stopTask');

        if (task && task.conversationId) {
            item.dataset.conversationId = task.conversationId;
        }

        item.innerHTML = `
            <div class="active-task-info">
                <span class="active-task-status">${statusText}</span>
                <span class="active-task-message">${escapeHtml(taskDisplayName)}</span>
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
                cancelBtn.onclick = (evt) => {
                    evt.stopPropagation();
                    cancelActiveTask(task.conversationId);
                };
                if (task.status === 'cancelling') {
                    cancelBtn.disabled = true;
                    cancelBtn.textContent = typeof window.t === 'function' ? window.t('tasks.cancelling') : '取消中...';
                }
            }
        }

        bar.appendChild(item);
    });
}

function cancelActiveTask(conversationId) {
    if (!conversationId) {
        return;
    }
    const progressId = findProgressIdByConversationId(conversationId);
    openUserInterruptModal(progressId, conversationId);
}

let monitorPanelFetchSeq = 0;

// 监控面板状态
const monitorState = {
    executions: [],
    summary: null,
    topTools: [],
    timeline: null,
    timelineRange: null,
    timelineError: null,
    timelineLoading: false,
    lastFetchedAt: null,
    retentionDays: 0,
    selectedExecutions: new Set(),
    renderKeys: {
        stats: '',
        executions: '',
        pagination: '',
        timeline: ''
    },
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

let monitorPollTimer = null;
let monitorPollGeneration = 0;
const MONITOR_POLL_INTERVAL_MS = 3000;

function startMonitorPoll() {
    stopMonitorPoll();
    scheduleMonitorPoll(monitorPollGeneration);
}

function scheduleMonitorPoll(generation) {
    monitorPollTimer = setTimeout(async function pollMonitor() {
        monitorPollTimer = null;
        if (generation !== monitorPollGeneration) return;
        const page = document.getElementById('page-mcp-monitor');
        if (!page || !page.classList.contains('active')) {
            return;
        }

        try {
            if (!document.hidden && typeof refreshMonitorPanel === 'function') {
                // 等待本轮完成后再安排下一轮，避免 WSL 或慢网络下请求重叠。
                await refreshMonitorPanel();
            }
        } catch (error) {
            // refreshMonitorPanel 已负责展示错误；轮询仍应继续。
        } finally {
            const activePage = document.getElementById('page-mcp-monitor');
            if (generation === monitorPollGeneration && activePage && activePage.classList.contains('active')) {
                scheduleMonitorPoll(generation);
            }
        }
    }, MONITOR_POLL_INTERVAL_MS);
}

function stopMonitorPoll() {
    monitorPollGeneration++;
    if (monitorPollTimer) {
        clearTimeout(monitorPollTimer);
        monitorPollTimer = null;
    }
}

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
        const mySeq = ++monitorPanelFetchSeq;
        const currentPage = page !== null ? page : monitorState.pagination.page;
        const pageSize = monitorState.pagination.pageSize;

        const statusFilter = document.getElementById('monitor-status-filter');
        const toolFilter = document.getElementById('monitor-tool-filter');
        const currentStatusFilter = statusFilter ? statusFilter.value : 'all';
        const currentToolFilter = toolFilter ? (toolFilter.value.trim() || 'all') : 'all';

        let url = `/api/monitor?page=${currentPage}&page_size=${pageSize}`;
        if (currentStatusFilter && currentStatusFilter !== 'all') {
            url += `&status=${encodeURIComponent(currentStatusFilter)}`;
        }
        if (currentToolFilter && currentToolFilter !== 'all') {
            url += `&tool=${encodeURIComponent(currentToolFilter)}`;
        }

        const range = getMcpMonitorTimelineRange();
        // 后台轮询时保留当前趋势图，避免每 3 秒重新进入加载态并触发闪烁。
        monitorState.timelineLoading = monitorState.timeline == null && !monitorState.timelineError;
        const timelinePromise = fetchMonitorTimeline(range);

        const monitorResp = await apiFetch(url, { method: 'GET' });
        const result = await monitorResp.json().catch(() => ({}));
        if (!monitorResp.ok) {
            throw new Error(result.error || '获取监控数据失败');
        }
        if (mySeq !== monitorPanelFetchSeq) {
            return;
        }

        applyMonitorPayload(result, currentStatusFilter);

        const { timeline, timelineError } = await timelinePromise;
        if (mySeq !== monitorPanelFetchSeq) {
            return;
        }
        applyMonitorTimelinePayload(timeline, timelineError, range);
        initializeMonitorPageSize();
    } catch (error) {
        console.error('刷新监控面板失败:', error);
        monitorState.timelineLoading = false;
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

const MONITOR_FILTER_SELECT_IDS = ['monitor-status-filter'];
const monitorFilterSelectMap = {};
let monitorFilterSelectDocBound = false;
const MONITOR_FILTER_SELECT_CARET = '<svg class="monitor-filter-select-caret" width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M6 9l6 6 6-6" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>';

function closeAllMonitorFilterSelects() {
    Object.keys(monitorFilterSelectMap).forEach(function (id) {
        const reg = monitorFilterSelectMap[id];
        if (!reg || !reg.wrapper) return;
        reg.wrapper.classList.remove('open');
        if (reg.trigger) reg.trigger.setAttribute('aria-expanded', 'false');
    });
}

function syncMonitorFilterSelect(selectId) {
    const reg = monitorFilterSelectMap[selectId];
    if (!reg) return;
    const select = reg.select;
    const dropdown = reg.dropdown;
    const trigger = reg.trigger;
    const valueSpan = trigger.querySelector('.monitor-filter-select-value');

    dropdown.innerHTML = '';
    Array.prototype.forEach.call(select.options, function (opt) {
        const item = document.createElement('button');
        item.type = 'button';
        item.className = 'monitor-filter-select-option';
        item.setAttribute('role', 'option');
        item.setAttribute('data-value', opt.value);
        if (opt.value === select.value) {
            item.classList.add('is-selected');
            item.setAttribute('aria-selected', 'true');
        } else {
            item.setAttribute('aria-selected', 'false');
        }
        const check = document.createElement('span');
        check.className = 'monitor-filter-select-check';
        check.setAttribute('aria-hidden', 'true');
        check.textContent = '✓';
        const label = document.createElement('span');
        label.className = 'monitor-filter-select-label';
        label.textContent = opt.textContent;
        item.appendChild(check);
        item.appendChild(label);
        dropdown.appendChild(item);
    });

    const selectedOpt = select.options[select.selectedIndex];
    if (valueSpan) {
        valueSpan.textContent = selectedOpt ? selectedOpt.textContent : '';
    }
    trigger.disabled = !!select.disabled;
    reg.wrapper.classList.toggle('is-disabled', !!select.disabled);
}

function syncAllMonitorFilterSelects() {
    MONITOR_FILTER_SELECT_IDS.forEach(syncMonitorFilterSelect);
}

function enhanceMonitorFilterSelect(selectId) {
    const select = document.getElementById(selectId);
    if (!select) return;
    const existing = monitorFilterSelectMap[selectId];
    if (existing && existing.select !== select) {
        delete monitorFilterSelectMap[selectId];
    }
    if (select.dataset.monitorCustomSelect === '1') {
        syncMonitorFilterSelect(selectId);
        return;
    }
    select.dataset.monitorCustomSelect = '1';
    select.classList.add('monitor-filter-native-select');
    select.tabIndex = -1;
    select.setAttribute('aria-hidden', 'true');

    const wrapper = document.createElement('div');
    wrapper.className = 'monitor-filter-select-ui';

    const trigger = document.createElement('button');
    trigger.type = 'button';
    trigger.className = 'monitor-filter-select-trigger';
    trigger.setAttribute('aria-haspopup', 'listbox');
    trigger.setAttribute('aria-expanded', 'false');
    const valueSpan = document.createElement('span');
    valueSpan.className = 'monitor-filter-select-value';
    trigger.appendChild(valueSpan);
    trigger.insertAdjacentHTML('beforeend', MONITOR_FILTER_SELECT_CARET);

    const dropdown = document.createElement('div');
    dropdown.className = 'monitor-filter-select-dropdown';
    dropdown.setAttribute('role', 'listbox');

    const parent = select.parentNode;
    parent.insertBefore(wrapper, select);
    wrapper.appendChild(trigger);
    wrapper.appendChild(dropdown);
    wrapper.appendChild(select);

    monitorFilterSelectMap[selectId] = { wrapper: wrapper, trigger: trigger, dropdown: dropdown, select: select };

    trigger.addEventListener('click', function (e) {
        e.stopPropagation();
        if (select.disabled) return;
        const open = wrapper.classList.contains('open');
        closeAllMonitorFilterSelects();
        if (!open) {
            wrapper.classList.add('open');
            trigger.setAttribute('aria-expanded', 'true');
        }
    });

    dropdown.addEventListener('click', function (e) {
        const opt = e.target.closest('.monitor-filter-select-option');
        if (!opt) return;
        e.stopPropagation();
        const val = opt.getAttribute('data-value');
        if (val === null) return;
        if (select.value !== val) {
            select.value = val;
            select.dispatchEvent(new Event('change', { bubbles: true }));
        }
        wrapper.classList.remove('open');
        trigger.setAttribute('aria-expanded', 'false');
        syncMonitorFilterSelect(selectId);
    });

    select.addEventListener('change', function () {
        syncMonitorFilterSelect(selectId);
    });

    if (!select.dataset.monitorFilterBound) {
        select.dataset.monitorFilterBound = '1';
        select.addEventListener('change', function () {
            applyMonitorFilters();
        });
    }

    syncMonitorFilterSelect(selectId);
}

function initMonitorFilterSelects() {
    if (!monitorFilterSelectDocBound) {
        document.addEventListener('click', function (e) {
            if (e.target.closest('.monitor-filter-select-ui')) return;
            closeAllMonitorFilterSelects();
        });
        document.addEventListener('keydown', function (e) {
            if (e.key === 'Escape') closeAllMonitorFilterSelects();
        });
        monitorFilterSelectDocBound = true;
    }
    MONITOR_FILTER_SELECT_IDS.forEach(enhanceMonitorFilterSelect);
    syncAllMonitorFilterSelects();
}

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
    const toolRaw = toolFilter ? (toolFilter.value.trim() || 'all') : 'all';
    const tool = toolRaw === 'all' ? 'all' : canonicalMonitorToolName(toolRaw);
    if (toolFilter) {
        toolFilter.classList.toggle('is-filter-active', toolRaw !== 'all');
    }
    // 当筛选条件改变时，从后端重新获取数据
    await refreshMonitorPanelWithFilter(status, tool);
}

async function refreshMonitorPanelWithFilter(statusFilter = 'all', toolFilter = 'all') {
    const statsContainer = document.getElementById('monitor-stats');
    const execContainer = document.getElementById('monitor-executions');

    try {
        const mySeq = ++monitorPanelFetchSeq;
        const currentPage = 1;
        const pageSize = monitorState.pagination.pageSize;

        let url = `/api/monitor?page=${currentPage}&page_size=${pageSize}`;
        if (statusFilter && statusFilter !== 'all') {
            url += `&status=${encodeURIComponent(statusFilter)}`;
        }
        if (toolFilter && toolFilter !== 'all') {
            url += `&tool=${encodeURIComponent(toolFilter)}`;
        }

        const range = getMcpMonitorTimelineRange();
        monitorState.timelineLoading = monitorState.timeline == null && !monitorState.timelineError;
        const timelinePromise = fetchMonitorTimeline(range);

        const monitorResp = await apiFetch(url, { method: 'GET' });
        const result = await monitorResp.json().catch(() => ({}));
        if (!monitorResp.ok) {
            throw new Error(result.error || '获取监控数据失败');
        }
        if (mySeq !== monitorPanelFetchSeq) {
            return;
        }

        applyMonitorPayload(result, statusFilter);

        const { timeline, timelineError } = await timelinePromise;
        if (mySeq !== monitorPanelFetchSeq) {
            return;
        }
        applyMonitorTimelinePayload(timeline, timelineError, range);
        initializeMonitorPageSize();
    } catch (error) {
        console.error('刷新监控面板失败:', error);
        monitorState.timelineLoading = false;
        if (statsContainer) {
            statsContainer.innerHTML = `<div class="monitor-error">${escapeHtml(typeof window.t === 'function' ? window.t('mcpMonitor.loadStatsError') : '无法加载统计信息')}：${escapeHtml(error.message)}</div>`;
        }
        if (execContainer) {
            execContainer.innerHTML = `<div class="monitor-error">${escapeHtml(typeof window.t === 'function' ? window.t('mcpMonitor.loadExecutionsError') : '无法加载执行记录')}：${escapeHtml(error.message)}</div>`;
        }
    }
}

function applyMonitorPayload(result, statusFilter) {
    const currentPage = monitorState.pagination.page;
    const pageSize = monitorState.pagination.pageSize;

    monitorState.executions = Array.isArray(result.executions) ? result.executions : [];
    monitorState.summary = result.summary || null;
    monitorState.topTools = Array.isArray(result.topTools) ? result.topTools : [];
    monitorState.lastFetchedAt = new Date();
    monitorState.retentionDays = typeof result.retentionDays === 'number' ? result.retentionDays : 0;

    if (result.total !== undefined) {
        monitorState.pagination = {
            page: result.page || currentPage,
            pageSize: result.pageSize || pageSize,
            total: result.total || 0,
            totalPages: result.totalPages || 1
        };
    }

    const locale = typeof window.__locale === 'string' ? window.__locale : '';
    const toolFilterEl = document.getElementById('monitor-tool-filter');
    const currentToolFilter = toolFilterEl ? toolFilterEl.value.trim() : '';
    const statsKey = monitorRenderKey([
        monitorState.summary,
        monitorState.topTools,
        monitorState.retentionDays,
        currentToolFilter,
        locale
    ]);
    const executionsKey = monitorRenderKey([
        monitorState.executions,
        statusFilter || 'all',
        currentToolFilter,
        locale
    ]);
    const paginationKey = monitorRenderKey(monitorState.pagination);

    if (statsKey !== monitorState.renderKeys.stats) {
        monitorState.renderKeys.stats = statsKey;
        renderMonitorStats(monitorState.summary, monitorState.topTools, monitorState.lastFetchedAt);
    } else if (document.querySelector('#monitor-stats .mcp-exec-stats')) {
        const toolCount = monitorState.summary && typeof monitorState.summary.toolCount === 'number'
            ? monitorState.summary.toolCount
            : monitorState.topTools.length;
        updateMonitorStatsSubtitle(monitorState.lastFetchedAt, toolCount, monitorState.retentionDays);
    }

    const executionsChanged = executionsKey !== monitorState.renderKeys.executions;
    if (executionsChanged) {
        monitorState.renderKeys.executions = executionsKey;
        renderMonitorExecutions(monitorState.executions, statusFilter);
    } else {
        updateMonitorExecutionDurations(monitorState.executions);
    }

    // 空列表渲染会清空执行区，因此这种情况下也需要恢复分页控件。
    if (executionsChanged || paginationKey !== monitorState.renderKeys.pagination) {
        monitorState.renderKeys.pagination = paginationKey;
        renderMonitorPagination();
    }
}

function monitorRenderKey(value) {
    try {
        return JSON.stringify(value);
    } catch (error) {
        return String(Date.now());
    }
}

function applyMonitorTimelinePayload(timeline, timelineError, range) {
    const wasLoading = monitorState.timelineLoading;
    const timelineKey = monitorRenderKey([timeline, timelineError || null, range || '']);
    const timelineChanged = timelineKey !== monitorState.renderKeys.timeline;
    monitorState.timeline = timeline;
    monitorState.timelineError = timelineError;
    monitorState.timelineLoading = false;
    if (wasLoading || timelineChanged) {
        monitorState.renderKeys.timeline = timelineKey;
        updateMonitorTimelineSection();
    }
}

async function fetchMonitorTimeline(range) {
    try {
        const timelineResp = await apiFetch(`/api/monitor/calls-timeline?range=${encodeURIComponent(range)}`, { method: 'GET' });
        const timelineJson = await timelineResp.json().catch(() => ({}));
        if (!timelineResp.ok) {
            return { timeline: null, timelineError: timelineJson.error || 'timeline failed' };
        }
        return { timeline: timelineJson, timelineError: null };
    } catch (err) {
        return { timeline: null, timelineError: err && err.message ? err.message : 'timeline failed' };
    }
}

function updateMonitorTimelineSection() {
    const timelineInner = document.querySelector('#monitor-stats .mcp-stats-combined__timeline-inner');
    if (timelineInner) {
        const combined = timelineInner.closest('.mcp-stats-combined');
        const compactEmpty = combined && !!combined.querySelector('.mcp-stats-combined__main');
        timelineInner.innerHTML = renderMcpStatsTimelineBody(
            monitorState.timeline,
            monitorState.timelineError,
            compactEmpty,
            monitorState.timelineLoading
        );
        bindMcpStatsTimelineEvents();
        syncMcpMonitorTimelineRangeUI();
        return;
    }
    if (monitorState.summary) {
        renderMonitorStats(monitorState.summary, monitorState.topTools, monitorState.lastFetchedAt);
    }
}


const MCP_STATS_TOP_N = 3;
const MCP_TIMELINE_RANGES = ['24h', '7d', '30d'];

function getMcpMonitorTimelineRange() {
    if (monitorState.timelineRange && MCP_TIMELINE_RANGES.includes(monitorState.timelineRange)) {
        return monitorState.timelineRange;
    }
    const saved = localStorage.getItem('mcpMonitorTimelineRange');
    const range = MCP_TIMELINE_RANGES.includes(saved) ? saved : '7d';
    monitorState.timelineRange = range;
    return range;
}

function buildMonitorTotals(summary) {
    const s = summary && typeof summary === 'object' ? summary : {};
    const total = s.totalCalls || 0;
    const success = s.successCalls || 0;
    const failed = s.failedCalls || 0;
    return {
        total,
        success,
        failed,
        neutral: Math.max(0, total - success - failed),
        lastCallTime: s.lastCallTime ? new Date(s.lastCallTime) : null,
    };
}

function formatMcpTimelineLabel(isoOrDate, rangeKey, locale) {
    const d = isoOrDate instanceof Date ? isoOrDate : new Date(isoOrDate);
    if (Number.isNaN(d.getTime())) return '';
    if (rangeKey === '24h') {
        return d.toLocaleTimeString(locale, { hour: '2-digit', minute: '2-digit' });
    }
    if (rangeKey === '30d') {
        return d.toLocaleDateString(locale, { month: 'numeric', day: 'numeric' });
    }
    return d.toLocaleString(locale, { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' });
}

function buildMcpTimelineSvg(points, rangeKey) {
    if (!Array.isArray(points) || points.length === 0) return '';
    const W = 400;
    const H = 140;
    const padL = 32;
    const padR = 8;
    const padT = 12;
    const padB = 24;
    const plotW = W - padL - padR;
    const plotH = H - padT - padB;
    const maxVal = Math.max(1, ...points.map((p) => p.total || 0));
    const hasFailed = points.some((p) => (p.failed || 0) > 0);
    const locale = (typeof window.__locale === 'string' && window.__locale.startsWith('zh')) ? 'zh-CN' : 'en-US';
    const barGap = points.length > 48 ? 1 : 2;
    const barW = Math.max(1.6, Math.min(8, (plotW / Math.max(1, points.length)) - barGap));

    const coords = points.map((p, i) => {
        const x = padL + (points.length <= 1 ? plotW / 2 : (i / (points.length - 1)) * plotW);
        const y = padT + plotH - ((p.total || 0) / maxVal) * plotH;
        return { x, y, p, i };
    });

    const linePath = coords.map((c, i) => `${i === 0 ? 'M' : 'L'} ${c.x.toFixed(2)} ${c.y.toFixed(2)}`).join(' ');
    const baseY = padT + plotH;
    const areaPath = `${linePath} L ${coords[coords.length - 1].x.toFixed(2)} ${baseY} L ${coords[0].x.toFixed(2)} ${baseY} Z`;

    let failPath = '';
    if (hasFailed) {
        failPath = coords.map((c, i) => {
            const fy = padT + plotH - ((c.p.failed || 0) / maxVal) * plotH;
            return `${i === 0 ? 'M' : 'L'} ${c.x.toFixed(2)} ${fy.toFixed(2)}`;
        }).join(' ');
    }

    let peakIdx = 0;
    points.forEach((p, i) => {
        if ((p.total || 0) >= (points[peakIdx].total || 0)) peakIdx = i;
    });

    const yTicks = [0, Math.ceil(maxVal / 2), maxVal];
    const yLines = yTicks.map((v) => {
        const y = padT + plotH - (v / maxVal) * plotH;
        const isBase = v === 0;
        return `<line class="mcp-stats-timeline-grid${isBase ? ' mcp-stats-timeline-grid--base' : ''}" x1="${padL}" y1="${y.toFixed(2)}" x2="${W - padR}" y2="${y.toFixed(2)}" />` +
            `<text class="mcp-stats-timeline-y" x="${padL - 4}" y="${(y + 3.5).toFixed(2)}">${v}</text>`;
    }).join('');

    const tickIdx = points.length <= 2
        ? points.map((_, i) => i)
        : [0, Math.floor((points.length - 1) / 2), points.length - 1];
    const xLabels = tickIdx.map((idx, ti) => {
        const c = coords[idx];
        const label = formatMcpTimelineLabel(c.p.t, rangeKey, locale);
        let anchor = 'middle';
        if (tickIdx.length > 1) {
            if (ti === 0) anchor = 'start';
            else if (ti === tickIdx.length - 1) anchor = 'end';
        }
        return `<text class="mcp-stats-timeline-axis" x="${c.x.toFixed(2)}" y="${H - 5}" text-anchor="${anchor}">${escapeHtml(label)}</text>`;
    }).join('');

    const dots = coords.map((c) => {
        const tipTime = formatMcpTimelineLabel(c.p.t, rangeKey, locale);
        const isPeak = c.i === peakIdx && (c.p.total || 0) > 0;
        const dotClass = 'mcp-stats-timeline-dot' + (isPeak ? ' mcp-stats-timeline-dot--peak' : '');
        return `<circle class="${dotClass}" cx="${c.x.toFixed(2)}" cy="${c.y.toFixed(2)}" r="${isPeak ? 2 : 1.5}"
            data-time="${escapeHtml(tipTime)}"
            data-total="${c.p.total || 0}"
            data-failed="${c.p.failed || 0}" />`;
    }).join('');

    const bars = coords.map((c) => {
        const total = c.p.total || 0;
        const failed = c.p.failed || 0;
        const h = total > 0 ? Math.max(3, (total / maxVal) * plotH) : 1;
        const y = baseY - h;
        const failedH = failed > 0 ? Math.max(2, (failed / maxVal) * plotH) : 0;
        const tipTime = formatMcpTimelineLabel(c.p.t, rangeKey, locale);
        return `<g class="mcp-stats-timeline-bar-group">
            <rect class="mcp-stats-timeline-bar${total > 0 ? ' is-active' : ''}" x="${(c.x - barW / 2).toFixed(2)}" y="${y.toFixed(2)}" width="${barW.toFixed(2)}" height="${h.toFixed(2)}" rx="1.6"
                data-time="${escapeHtml(tipTime)}" data-total="${total}" data-failed="${failed}" />
            ${failedH > 0 ? `<rect class="mcp-stats-timeline-bar-fail" x="${(c.x - barW / 2).toFixed(2)}" y="${(baseY - failedH).toFixed(2)}" width="${barW.toFixed(2)}" height="${failedH.toFixed(2)}" rx="1.6"
                data-time="${escapeHtml(tipTime)}" data-total="${total}" data-failed="${failed}" />` : ''}
        </g>`;
    }).join('');

    const peakC = coords[peakIdx];
    const peakMarker = (peakC.p.total || 0) > 0
        ? `<circle class="mcp-stats-timeline-peak-glow" cx="${peakC.x.toFixed(2)}" cy="${peakC.y.toFixed(2)}" r="5" />`
        : '';

    return `<svg class="mcp-stats-timeline__chart" viewBox="0 0 ${W} ${H}" preserveAspectRatio="none" aria-hidden="true">
        <defs>
            <linearGradient id="mcpTimelineAreaFill" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stop-color="#3b82f6" stop-opacity="0.28"/>
                <stop offset="85%" stop-color="#3b82f6" stop-opacity="0.04"/>
                <stop offset="100%" stop-color="#3b82f6" stop-opacity="0"/>
            </linearGradient>
            <linearGradient id="mcpTimelineLineStroke" x1="0" y1="0" x2="1" y2="0">
                <stop offset="0%" stop-color="#60a5fa"/>
                <stop offset="50%" stop-color="#3b82f6"/>
                <stop offset="100%" stop-color="#2563eb"/>
            </linearGradient>
        </defs>
        ${yLines}
        <path class="mcp-stats-timeline-area" d="${areaPath}" fill="url(#mcpTimelineAreaFill)" />
        ${bars}
        ${peakMarker}
        <path class="mcp-stats-timeline-line" d="${linePath}" stroke="url(#mcpTimelineLineStroke)" />
        ${hasFailed ? `<path class="mcp-stats-timeline-line mcp-stats-timeline-line--fail" d="${failPath}" />` : ''}
        ${dots}
        ${xLabels}
    </svg>`;
}

let mcpTimelineEventsBound = false;
let mcpTimelineTooltipEl = null;

function bindMcpStatsTimelineEvents() {
    const root = document.getElementById('monitor-stats');
    if (!root) return;

    root.querySelectorAll('.mcp-stats-timeline__range').forEach((btn) => {
        btn.onclick = function () {
            const range = btn.getAttribute('data-range');
            if (range) setMcpMonitorTimelineRange(range);
        };
    });

    if (mcpTimelineEventsBound) return;
    if (!mcpTimelineTooltipEl) {
        mcpTimelineTooltipEl = document.createElement('div');
        mcpTimelineTooltipEl.className = 'mcp-stats-timeline-tooltip';
        mcpTimelineTooltipEl.setAttribute('role', 'tooltip');
        document.body.appendChild(mcpTimelineTooltipEl);
    }

    root.addEventListener('mousemove', function (e) {
        const dot = e.target.closest('.mcp-stats-timeline-dot, .mcp-stats-timeline-bar, .mcp-stats-timeline-bar-fail');
        if (!dot || !mcpTimelineTooltipEl) {
            root.querySelectorAll('.mcp-stats-timeline-dot.is-active').forEach((d) => d.classList.remove('is-active'));
            root.querySelectorAll('.mcp-stats-timeline-bar.is-hover, .mcp-stats-timeline-bar-fail.is-hover').forEach((d) => d.classList.remove('is-hover'));
            mcpTimelineTooltipEl.style.display = 'none';
            return;
        }
        root.querySelectorAll('.mcp-stats-timeline-dot.is-active').forEach((d) => d.classList.remove('is-active'));
        root.querySelectorAll('.mcp-stats-timeline-bar.is-hover, .mcp-stats-timeline-bar-fail.is-hover').forEach((d) => d.classList.remove('is-hover'));
        dot.classList.add('is-active');
        dot.classList.add('is-hover');
        const time = dot.getAttribute('data-time') || '';
        const total = dot.getAttribute('data-total') || '0';
        const failed = dot.getAttribute('data-failed') || '0';
        const tip = mcpMonitorT('timelineTooltip', { time, total, failed })
            || `${time}：${total} 次（失败 ${failed}）`;
        mcpTimelineTooltipEl.textContent = tip;
        mcpTimelineTooltipEl.style.display = 'block';
        mcpTimelineTooltipEl.style.left = `${e.clientX}px`;
        mcpTimelineTooltipEl.style.top = `${e.clientY}px`;
    });

    root.addEventListener('mouseleave', function (e) {
        if (!e.target.closest || !e.target.closest('.mcp-stats-combined__timeline, .mcp-stats-timeline')) return;
        if (e.relatedTarget && root.contains(e.relatedTarget)) return;
        root.querySelectorAll('.mcp-stats-timeline-dot.is-active').forEach((d) => d.classList.remove('is-active'));
        root.querySelectorAll('.mcp-stats-timeline-bar.is-hover, .mcp-stats-timeline-bar-fail.is-hover').forEach((d) => d.classList.remove('is-hover'));
        if (mcpTimelineTooltipEl) mcpTimelineTooltipEl.style.display = 'none';
    });

    mcpTimelineEventsBound = true;
}

function getMcpTimelineRangeLabel(rangeKey) {
    const key = rangeKey === '24h' ? 'timelineRange24h' : rangeKey === '30d' ? 'timelineRange30d' : 'timelineRange7d';
    return mcpMonitorT(key) || rangeKey;
}

function syncMcpMonitorTimelineRangeUI(activeRange) {
    const range = activeRange || getMcpMonitorTimelineRange();
    document.querySelectorAll('#monitor-stats .mcp-stats-timeline__range').forEach((btn) => {
        const r = btn.getAttribute('data-range');
        const on = r === range;
        btn.classList.toggle('is-active', on);
        btn.setAttribute('aria-pressed', on ? 'true' : 'false');
    });
    const scopeBadge = document.querySelector('#monitor-stats .mcp-stats-scope-badge--timeline');
    if (scopeBadge) scopeBadge.textContent = getMcpTimelineRangeLabel(range);
}

function renderMcpStatsScopeBadges(showTools, showTimeline) {
    const parts = [];
    if (showTools) {
        const cumulative = mcpMonitorT('scopeCumulative') || '累计';
        parts.push(`<span class="mcp-stats-scope-badge mcp-stats-scope-badge--cumulative">${escapeHtml(cumulative)}</span>`);
    }
    if (showTimeline) {
        const range = getMcpMonitorTimelineRange();
        parts.push(`<span class="mcp-stats-scope-badge mcp-stats-scope-badge--timeline">${escapeHtml(getMcpTimelineRangeLabel(range))}</span>`);
    }
    if (!parts.length) return '';
    return `<div class="mcp-stats-combined__scopes" role="note">${parts.join('')}</div>`;
}

function buildTimelineSparseHint(points, timeline) {
    if (!Array.isArray(points) || points.length < 4 || !timeline || !timeline.summary) return '';
    const summaryTotal = timeline.summary.totalCalls || 0;
    const peak = timeline.summary.peak || 0;
    if (summaryTotal === 0 || peak === 0) return '';

    const nonZero = points.filter((p) => (p.total || 0) > 0).length;
    const nonZeroRatio = nonZero / points.length;
    let peakIdx = 0;
    points.forEach((p, i) => {
        if ((p.total || 0) >= (points[peakIdx].total || 0)) peakIdx = i;
    });
    const peakNearEnd = peakIdx >= Math.floor(points.length * 0.8);
    if (nonZeroRatio > 0.3 && !peakNearEnd) return '';

    const rangeKey = timeline.range || getMcpMonitorTimelineRange();
    const locale = (typeof window.__locale === 'string' && window.__locale.startsWith('zh')) ? 'zh-CN' : 'en-US';
    const peakTime = timeline.summary.peakAt
        ? formatMcpTimelineLabel(timeline.summary.peakAt, rangeKey, locale)
        : formatMcpTimelineLabel(points[peakIdx].t, rangeKey, locale);
    return mcpMonitorT('timelineSparseHint', { peak, peakTime })
        || `该时段多数时间为 0，峰值 ${peak} 次出现在 ${peakTime}`;
}

function renderMcpTimelineActiveMoments(points, rangeKey) {
    if (!Array.isArray(points) || points.length === 0) return '';
    const locale = (typeof window.__locale === 'string' && window.__locale.startsWith('zh')) ? 'zh-CN' : 'en-US';
    const active = points
        .map((p, i) => ({ ...p, i }))
        .filter((p) => (p.total || 0) > 0)
        .sort((a, b) => (b.total || 0) - (a.total || 0) || b.i - a.i)
    const shown = active.slice(0, 4);
    const hiddenCount = Math.max(0, active.length - shown.length);
    if (!active.length) return '';
    const label = mcpMonitorT('timelineActiveMoments') || monitorFallback('活跃时段', 'Active moments');
    const moreLabel = mcpMonitorT('timelineMoreMoments', { n: hiddenCount }) || `+${hiddenCount}`;
    const chips = shown.map((p) => {
        const time = formatMcpTimelineLabel(p.t, rangeKey, locale);
        const failed = p.failed || 0;
        const failedLabel = mcpMonitorT('failedCount', { n: failed }) || `失败 ${failed}`;
        return `<span class="mcp-stats-timeline-moment" title="${escapeHtml(time)}">
            <span class="mcp-stats-timeline-moment__time">${escapeHtml(time)}</span>
            <span class="mcp-stats-timeline-moment__count">${p.total || 0}</span>
            ${failed > 0 ? `<span class="mcp-stats-timeline-moment__fail">${escapeHtml(failedLabel)}</span>` : ''}
        </span>`;
    }).join('');
    const moreChip = hiddenCount > 0
        ? `<span class="mcp-stats-timeline-moment mcp-stats-timeline-moment--more" title="${escapeHtml(mcpMonitorT('timelineMoreMomentsTitle', { n: hiddenCount }) || `还有 ${hiddenCount} 个活跃时段`)}">${escapeHtml(moreLabel)}</span>`
        : '';
    return `<div class="mcp-stats-timeline-moments">
        <span class="mcp-stats-timeline-moments__label">${escapeHtml(label)}</span>
        <div class="mcp-stats-timeline-moments__list">${chips}${moreChip}</div>
    </div>`;
}

async function setMcpMonitorTimelineRange(range) {
    if (!MCP_TIMELINE_RANGES.includes(range)) return;
    localStorage.setItem('mcpMonitorTimelineRange', range);
    monitorState.timelineRange = range;
    monitorState.timelineError = null;
    monitorState.timelineLoading = true;
    syncMcpMonitorTimelineRangeUI(range);
    updateMonitorTimelineSection();
    try {
        const { timeline, timelineError } = await fetchMonitorTimeline(range);
        applyMonitorTimelinePayload(timeline, timelineError, range);
    } catch (err) {
        applyMonitorTimelinePayload(null, err.message || 'error', range);
    }
}
window.setMcpMonitorTimelineRange = setMcpMonitorTimelineRange;

function renderMcpStatsTimelineRangeButtons() {
    const activeRange = getMcpMonitorTimelineRange();
    return MCP_TIMELINE_RANGES.map((r) => {
        const labelKey = r === '24h' ? 'timelineRange24h' : r === '30d' ? 'timelineRange30d' : 'timelineRange7d';
        const label = mcpMonitorT(labelKey) || r;
        return `<button type="button" class="mcp-stats-timeline__range${activeRange === r ? ' is-active' : ''}"
            data-range="${r}" aria-pressed="${activeRange === r ? 'true' : 'false'}">${escapeHtml(label)}</button>`;
    }).join('');
}

const MCP_TIMELINE_EMPTY_ICON = '<svg class="mcp-stats-timeline-empty-state__icon" width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>';

function renderMcpStatsTimelineEmptyState(compact) {
    const noData = mcpMonitorT('timelineNoData') || monitorFallback('该时段暂无调用', 'No calls in this period');
    const emptyHint = mcpMonitorT('timelineEmptyHint')
        || monitorFallback('切换时间范围查看其他时段，或在对话/任务中调用 MCP 工具', 'Switch the time range or invoke MCP tools in chat or tasks');
    const compactClass = compact ? ' mcp-stats-timeline-empty-state--compact' : '';
    return `<div class="mcp-stats-timeline-empty-state${compactClass}">
        ${MCP_TIMELINE_EMPTY_ICON}
        <p class="mcp-stats-timeline-empty-state__title">${escapeHtml(noData)}</p>
        <p class="mcp-stats-timeline-empty-state__hint">${escapeHtml(emptyHint)}</p>
    </div>`;
}

function renderMcpStatsTimelineBody(timeline, timelineError, compactEmpty, loading) {
    if (loading) {
        const loadingText = mcpMonitorT('timelineLoading') || monitorFallback('趋势加载中…', 'Loading trend…');
        return `<div class="monitor-empty monitor-empty--inline">${escapeHtml(loadingText)}</div>`;
    }

    const hint = mcpMonitorT('timelineHint') || monitorFallback('全部工具合计', 'All tools combined');

    if (timelineError) {
        const errText = mcpMonitorT('timelineLoadError') || monitorFallback('无法加载调用趋势', 'Failed to load call trend');
        return `<p class="mcp-stats-timeline-error">${escapeHtml(errText)}：${escapeHtml(timelineError)}</p>`;
    }

    const points = timeline && Array.isArray(timeline.points) ? timeline.points : [];
    const summaryTotal = timeline && timeline.summary ? (timeline.summary.totalCalls || 0) : 0;
    const peak = timeline && timeline.summary ? (timeline.summary.peak || 0) : 0;
    const summaryText = mcpMonitorT('timelineSummary', { total: summaryTotal, peak })
        || `区间内 ${summaryTotal} 次 · 峰值 ${peak}`;

    if (points.length === 0 || summaryTotal === 0) {
        return renderMcpStatsTimelineEmptyState(!!compactEmpty);
    }

    const rangeKey = timeline.range || getMcpMonitorTimelineRange();
    const chartSvg = buildMcpTimelineSvg(points, rangeKey);
    const totalLegend = mcpMonitorT('timelineTotalLegend') || '总调用';
    const failLegend = mcpMonitorT('timelineFailedLegend') || '失败';
    const hasFailed = points.some((p) => (p.failed || 0) > 0);
    const sparseHint = buildTimelineSparseHint(points, timeline);
    const momentsHtml = renderMcpTimelineActiveMoments(points, rangeKey);
    const sparseHtml = sparseHint
        ? `<p class="mcp-stats-timeline__sparse-hint">${escapeHtml(sparseHint)}</p>`
        : '';

    return `
        <p class="mcp-stats-timeline__inline-meta">${escapeHtml(hint)} · ${escapeHtml(summaryText)}</p>
        ${momentsHtml}
        <div class="mcp-stats-timeline__chart-wrap">${chartSvg}</div>
        ${sparseHtml}
        <div class="mcp-stats-timeline__legend">
            <span class="mcp-stats-timeline__legend-item">${escapeHtml(totalLegend)}</span>
            ${hasFailed ? `<span class="mcp-stats-timeline__legend-item mcp-stats-timeline__legend-item--fail">${escapeHtml(failLegend)}</span>` : ''}
        </div>`;
}

function renderMcpStatsCombinedSection(topTools, totals, activeToolFilter, timeline, timelineError, showTimeline) {
    const statsTitle = mcpMonitorT('toolStatsTitle') || monitorFallback('工具统计', 'Tool statistics');
    const timelineTitle = mcpMonitorT('timelineTitle') || monitorFallback('调用趋势', 'Call trend');
    const statsHint = mcpMonitorT('toolStatsHint') || monitorFallback('点击色条或列表行筛选下方执行记录', 'Click a bar segment or row to filter records below');
    const hasTools = topTools.length > 0;

    if (!hasTools && !showTimeline) return '';

    const filterChipLabel = activeToolFilter ? formatMonitorToolName(activeToolFilter) : '';
    const filterChip = activeToolFilter
        ? `<span class="mcp-stats-filter-chip" title="${escapeHtml(mcpMonitorT('filterByToolTitle', { tool: filterChipLabel }) || filterChipLabel)}">
            <span class="mcp-stats-filter-chip__label">${escapeHtml(mcpMonitorT('filterActive', { tool: filterChipLabel }) || `已筛选：${filterChipLabel}`)}</span>
            <button type="button" class="mcp-stats-filter-chip__clear mcp-stats-clear-filter" aria-label="${escapeHtml(mcpMonitorT('clearToolFilter') || '清除工具筛选')}">×</button>
        </span>`
        : '';

    const rangeButtons = showTimeline
        ? `<div class="mcp-stats-timeline__ranges" role="group" aria-label="${escapeHtml(timelineTitle)}">${renderMcpStatsTimelineRangeButtons()}</div>`
        : '';

    const panelTitle = showTimeline && hasTools
        ? `${statsTitle} · ${timelineTitle}`
        : (hasTools ? statsTitle : timelineTitle);

    const scopeBadges = renderMcpStatsScopeBadges(hasTools, showTimeline);
    const metaHint = hasTools ? statsHint : '';

    const timelineCol = showTimeline
        ? `<div class="mcp-stats-combined__timeline">
            <p class="mcp-stats-combined__col-label">${escapeHtml(timelineTitle)}</p>
            <div class="mcp-stats-combined__timeline-inner">${renderMcpStatsTimelineBody(timeline, timelineError, hasTools, monitorState.timelineLoading)}</div>
        </div>`
        : '';

    let bodyMod = 'mcp-stats-combined__body';
    if (hasTools && showTimeline) bodyMod += ' mcp-stats-combined__body--full';
    else if (hasTools) bodyMod += ' mcp-stats-combined__body--tools';
    else bodyMod += ' mcp-stats-combined__body--timeline';

    const mainBlock = hasTools
        ? `<div class="mcp-stats-combined__main">${renderMcpStatsToolsPanel(topTools, totals, activeToolFilter)}</div>`
        : '';

    return `
        <section class="mcp-stats-combined" aria-label="${escapeHtml(panelTitle)}">
            <header class="mcp-stats-combined__head">
                <div class="mcp-stats-combined__head-text">
                    <h4 class="mcp-stats-combined__title">${escapeHtml(panelTitle)}</h4>
                    <div class="mcp-stats-combined__meta-row">
                        ${scopeBadges}
                        ${metaHint ? `<p class="mcp-stats-combined__meta">${escapeHtml(metaHint)}</p>` : ''}
                    </div>
                </div>
                <div class="mcp-stats-combined__actions">
                    ${filterChip}
                    ${rangeButtons}
                </div>
            </header>
            <div class="${bodyMod}">
                ${mainBlock}
                ${timelineCol}
            </div>
        </section>`;
}

function mcpMonitorT(key, params) {
    if (typeof window.t !== 'function') return '';
    const fullKey = 'mcpMonitor.' + key;
    const text = window.t(fullKey, {
        ...(params || {}),
        interpolation: { escapeValue: false },
    });
    if (!text || text === fullKey) return '';
    return text;
}

function monitorFallback(zhText, enText) {
    return (typeof window.__locale === 'string' && window.__locale.startsWith('zh')) ? zhText : enText;
}

function refreshMonitorPanelFromState() {
    if (!document.getElementById('monitor-stats')) return;
    if (!monitorState.lastFetchedAt) return;
    const statusFilter = document.getElementById('monitor-status-filter');
    const currentStatusFilter = statusFilter ? statusFilter.value : 'all';
    renderMonitorStats(monitorState.summary, monitorState.topTools, monitorState.lastFetchedAt);
    renderMonitorExecutions(monitorState.executions || [], currentStatusFilter);
    renderMonitorPagination();
}

const MCP_STATS_TOOL_CHEVRON = '<svg class="mcp-stats-tool-chevron" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="9 18 15 12 9 6"/></svg>';

function getMcpStatsRateTone(rateNum) {
    if (rateNum >= 95) return 'is-success';
    if (rateNum >= 80) return 'is-warning';
    return 'is-danger';
}

function getMcpStatsRingStrokeClass(rateNum) {
    if (rateNum >= 95) return '';
    if (rateNum >= 80) return 'is-warning';
    return 'is-danger';
}

function renderMcpStatsSuccessRing(percent) {
    const p = Math.min(100, Math.max(0, parseFloat(percent) || 0));
    const r = 15.9155;
    const circumference = 2 * Math.PI * r;
    const offset = circumference - (p / 100) * circumference;
    const strokeClass = getMcpStatsRingStrokeClass(p);
    return `<div class="mcp-stats-ring-wrap" aria-hidden="true">
        <svg class="mcp-stats-ring-svg" viewBox="0 0 36 36">
            <circle class="mcp-stats-ring-track" cx="18" cy="18" r="${r}" fill="none" stroke-width="3"/>
            <circle class="mcp-stats-ring-fill ${strokeClass}" cx="18" cy="18" r="${r}" fill="none" stroke-width="3"
                stroke-dasharray="${circumference}" stroke-dashoffset="${offset}"/>
        </svg>
    </div>`;
}

function renderMcpStatsToolVolumeBar(total, success, failed, maxTotal) {
    const volumePct = maxTotal > 0 && total > 0 ? (total / maxTotal) * 100 : 0;
    const successPct = total > 0 ? (success / total) * 100 : 0;
    const failPct = total > 0 ? (failed / total) * 100 : 0;
    const legend = mcpMonitorT('barVolumeLegend') || '条长表示相对调用量';
    const volumeTitle = `${total} / ${maxTotal}`;
    return `<div class="mcp-stats-tool-bar-track" title="${escapeHtml(legend)} · ${escapeHtml(volumeTitle)}">
        <div class="mcp-stats-tool-bar-fill" style="width:${volumePct.toFixed(2)}%">
            <div class="mcp-stats-tool-bar-inner">
                <span class="mcp-stats-tool-bar-seg mcp-stats-tool-bar-seg--success" style="width:${successPct.toFixed(2)}%"></span>
                <span class="mcp-stats-tool-bar-seg mcp-stats-tool-bar-seg--fail" style="width:${failPct.toFixed(2)}%"></span>
            </div>
        </div>
    </div>`;
}

function getMcpToolRateClass(rateNum) {
    if (rateNum >= 95) return 'is-success';
    if (rateNum >= 80) return 'is-warning';
    return 'is-danger';
}

const MCP_STATS_DIST_COLORS = ['#3b82f6', '#22c55e', '#f59e0b', '#8b5cf6', '#14b8a6', '#ec4899'];
const MCP_STATS_CHART_MIN_PCT = 5;

function buildMcpStatsChartSegments(topTools, totals, options = {}) {
    const groupSmall = options.groupSmall !== false;
    const minPct = options.minPct ?? MCP_STATS_CHART_MIN_PCT;
    const othersLabel = mcpMonitorT('distOthers') || '其他工具';
    const topNTotal = topTools.reduce((s, t) => s + (t.totalCalls || 0), 0);
    const otherCalls = Math.max(0, totals.total - topNTotal);

    const segments = [];
    let bundledCalls = otherCalls;

    topTools.forEach((tool, i) => {
        const calls = tool.totalCalls || 0;
        if (calls <= 0 || totals.total <= 0) return;
        const pct = (calls / totals.total) * 100;
        if (groupSmall && pct < minPct) {
            bundledCalls += calls;
            return;
        }
        segments.push({
            color: MCP_STATS_DIST_COLORS[i % MCP_STATS_DIST_COLORS.length],
            name: tool.toolName || '',
            calls,
            pct: pct.toFixed(1),
            pctNum: pct,
            isOthers: false,
            colorIndex: i,
        });
    });

    if (bundledCalls > 0 && totals.total > 0) {
        const pct = (bundledCalls / totals.total) * 100;
        segments.push({
            color: '#cbd5e1',
            name: othersLabel,
            calls: bundledCalls,
            pct: pct.toFixed(1),
            pctNum: pct,
            isOthers: true,
            colorIndex: topTools.length,
        });
    }

    let acc = 0;
    return segments.map((s) => {
        const start = acc;
        acc += s.pctNum;
        return { ...s, start, end: acc };
    });
}

function renderMcpStatsShareCell(sharePct, color) {
    const width = Math.min(100, Math.max(0, parseFloat(sharePct) || 0));
    return `<td class="mcp-stats-col-share">
        <div class="mcp-stats-share-cell">
            <span class="mcp-stats-share-pct">${escapeHtml(sharePct)}%</span>
            <span class="mcp-stats-share-track" aria-hidden="true">
                <span class="mcp-stats-share-fill" style="width:${width.toFixed(1)}%;background:${color}"></span>
            </span>
        </div>
    </td>`;
}

function mcpStatsDescribeDonutSegment(startPct, endPct, outerR, innerR) {
    if (endPct <= startPct) return '';
    const span = endPct - startPct;
    const cx = 50;
    const cy = 50;
    const point = (pct, r) => {
        const rad = ((pct / 100) * 360 - 90) * Math.PI / 180;
        return [cx + r * Math.cos(rad), cy + r * Math.sin(rad)];
    };
    if (span >= 99.995) {
        const [x1, y1] = point(0, outerR);
        const [x2, y2] = point(50, outerR);
        const [x3, y3] = point(50, innerR);
        const [x4, y4] = point(0, innerR);
        const [x5, y5] = point(50, outerR);
        const [x6, y6] = point(100, outerR);
        const [x7, y7] = point(100, innerR);
        const [x8, y8] = point(50, innerR);
        return `M ${x1.toFixed(3)} ${y1.toFixed(3)} A ${outerR} ${outerR} 0 0 1 ${x2.toFixed(3)} ${y2.toFixed(3)} A ${outerR} ${outerR} 0 0 1 ${x6.toFixed(3)} ${y6.toFixed(3)} L ${x7.toFixed(3)} ${y7.toFixed(3)} A ${innerR} ${innerR} 0 0 0 ${x8.toFixed(3)} ${y8.toFixed(3)} A ${innerR} ${innerR} 0 0 0 ${x4.toFixed(3)} ${y4.toFixed(3)} Z`;
    }
    const large = span > 50 ? 1 : 0;
    const [x1, y1] = point(startPct, outerR);
    const [x2, y2] = point(endPct, outerR);
    const [x3, y3] = point(endPct, innerR);
    const [x4, y4] = point(startPct, innerR);
    return `M ${x1.toFixed(3)} ${y1.toFixed(3)} A ${outerR} ${outerR} 0 ${large} 1 ${x2.toFixed(3)} ${y2.toFixed(3)} L ${x3.toFixed(3)} ${y3.toFixed(3)} A ${innerR} ${innerR} 0 ${large} 0 ${x4.toFixed(3)} ${y4.toFixed(3)} Z`;
}

function resetMcpStatsDistCenter(panel) {
    if (!panel) return;
    const label = panel.querySelector('.mcp-stats-dist-donut-label');
    const value = panel.querySelector('.mcp-stats-dist-donut-value');
    const unit = panel.querySelector('.mcp-stats-dist-donut-unit');
    if (!label || !value) return;
    label.textContent = panel.getAttribute('data-center-label') || '';
    label.classList.add('is-default');
    const centerVal = panel.getAttribute('data-center-value') || '';
    const numEl = panel.querySelector('.mcp-stats-dist-donut-value-num');
    if (numEl) numEl.textContent = centerVal;
    else value.textContent = centerVal;
    if (unit) {
        unit.textContent = panel.getAttribute('data-center-suffix') || '%';
        unit.hidden = false;
    }
}

function previewMcpStatsDistCenter(panel, toolName, pct) {
    if (!panel) return;
    const label = panel.querySelector('.mcp-stats-dist-donut-label');
    const value = panel.querySelector('.mcp-stats-dist-donut-value');
    const unit = panel.querySelector('.mcp-stats-dist-donut-unit');
    if (!label || !value) return;
    const shortName = toolName.length > 14 ? `${toolName.slice(0, 13)}…` : toolName;
    label.textContent = shortName;
    label.classList.remove('is-default');
    const numEl = panel.querySelector('.mcp-stats-dist-donut-value-num');
    if (numEl) numEl.textContent = pct;
    else value.textContent = pct;
    if (unit) unit.hidden = false;
}

function setMcpStatsDistHover(toolName) {
    const panel = document.querySelector('.mcp-stats-dist-panel');
    const root = document.getElementById('monitor-stats');
    const esc = toolName && typeof CSS !== 'undefined' && CSS.escape
        ? CSS.escape(toolName)
        : (toolName || '').replace(/"/g, '\\"');

    if (panel) {
        panel.querySelectorAll('.mcp-stats-dist-segment, .mcp-stats-dist-legend-item').forEach((el) => {
            const t = el.getAttribute('data-tool-name') || '';
            const match = toolName && t === toolName;
            el.classList.toggle('is-highlighted', !!match);
            el.classList.toggle('is-dimmed', !!toolName && !match && t);
        });
        if (toolName) {
            const el = panel.querySelector(`[data-tool-name="${esc}"]`);
            if (el) {
                previewMcpStatsDistCenter(panel, toolName, el.getAttribute('data-pct') || '');
            }
        } else {
            resetMcpStatsDistCenter(panel);
        }
    }

    if (root) {
        root.querySelectorAll(
            'tr.mcp-stats-tool-row[data-tool-name], .mcp-stats-tool-item[data-tool-name], .mcp-stats-proportion-seg[data-tool-name]'
        ).forEach((el) => {
            const t = el.getAttribute('data-tool-name') || '';
            const match = toolName && t === toolName;
            el.classList.toggle('is-highlighted', !!match);
            el.classList.toggle('is-dimmed', !!toolName && !match && t);
        });
    }
}

function handleMonitorStatsToolFilter(toolName) {
    if (!toolName) return;
    const toolFilter = document.getElementById('monitor-tool-filter');
    if (toolFilter && toolFilter.value === toolName) {
        clearMonitorToolFilter();
        return;
    }
    filterMonitorByTool(toolName);
}

function renderMcpStatsInsightPanel(topTools, totals, activeToolFilter = '', options = {}) {
    const embedded = !!options.embedded;
    const distTitle = mcpMonitorT('distTitle') || '调用分布';
    const distClickHint = mcpMonitorT('distClickHint') || '点击扇区筛选执行记录';
    const distOthersTitle = mcpMonitorT('distOthersNoFilter') || '其他工具无法单独筛选';
    const top6ShareLabel = mcpMonitorT('distTop6Share', { n: MCP_STATS_TOP_N }) || `Top ${MCP_STATS_TOP_N} 占全部调用`;

    const top6Total = topTools.reduce((s, t) => s + (t.totalCalls || 0), 0);
    const top6SharePct = totals.total > 0 ? ((top6Total / totals.total) * 100).toFixed(1) : '0.0';

    const segments = buildMcpStatsChartSegments(topTools, totals, { groupSmall: embedded });

    const segmentPathsHtml = segments.map((s) => {
        const pathD = mcpStatsDescribeDonutSegment(s.start, s.end, 48, 30);
        if (!pathD) return '';
        const isActive = !s.isOthers && activeToolFilter && activeToolFilter === s.name;
        const segAria = s.isOthers
            ? escapeHtml(s.name)
            : escapeHtml(mcpMonitorT('distSegmentAria', { name: s.name, pct: s.pct, calls: s.calls })
                || `${s.name}，占 ${s.pct}%，${s.calls} 次`);
        return `<path class="mcp-stats-dist-segment${isActive ? ' is-active' : ''}${s.isOthers ? ' is-others' : ''}"
            d="${pathD}"
            fill="${s.color}"
            data-tool-name="${s.isOthers ? '' : escapeHtml(s.name)}"
            data-pct="${s.pct}"
            data-calls="${s.calls}"
            data-is-others="${s.isOthers ? '1' : '0'}"
            tabindex="${s.isOthers ? '-1' : '0'}"
            role="${s.isOthers ? 'presentation' : 'button'}"
            aria-label="${segAria}" />`;
    }).join('');

    const legendHtml = embedded ? '' : segments.map((s) => {
        const isActive = !s.isOthers && activeToolFilter && activeToolFilter === s.name;
        const inner = `
            <span class="mcp-stats-dist-swatch" style="--swatch-color:${s.color}"></span>
            <span class="mcp-stats-dist-legend-name" title="${escapeHtml(s.name)}">${escapeHtml(s.name)}</span>
            <span class="mcp-stats-dist-legend-pct">${s.pct}%</span>`;
        if (s.isOthers) {
            return `<li class="mcp-stats-dist-legend-item is-others" title="${escapeHtml(distOthersTitle)}" data-is-others="1">${inner}</li>`;
        }
        const rowAria = mcpMonitorT('toolRowAriaLabel', { name: s.name, total: s.calls, rate: s.pct })
            || `${s.name}，${s.calls} 次调用，占 ${s.pct}%`;
        return `<li class="mcp-stats-dist-legend-item-wrap">
            <button type="button" class="mcp-stats-dist-legend-item${isActive ? ' is-active' : ''}"
                data-tool-name="${escapeHtml(s.name)}"
                data-pct="${s.pct}"
                data-calls="${s.calls}"
                data-is-others="0"
                aria-label="${escapeHtml(rowAria)}"
                aria-pressed="${isActive ? 'true' : 'false'}">${inner}</button>
        </li>`;
    }).join('');

    const centerLabel = embedded ? (mcpMonitorT('distTitle') || '占比') : `Top ${MCP_STATS_TOP_N}`;
    const distHint = totals.total > 0
        ? (mcpMonitorT('distTotalCalls', { n: totals.total }) || `共 ${totals.total} 次调用`)
        : '';

    const bodyClass = embedded ? 'mcp-stats-dist-body mcp-stats-dist-body--chart-only' : 'mcp-stats-dist-body mcp-stats-dist-body--side';
    const legendBlock = legendHtml
        ? `<ul class="mcp-stats-dist-legend mcp-stats-dist-legend--side">${legendHtml}</ul>`
        : '';

    const headerHtml = embedded
        ? `<div class="mcp-stats-dist-embedded-title">${escapeHtml(distTitle)}</div>`
        : `
            <div class="mcp-stats-tools-header">
                <div class="mcp-stats-tools-heading">
                    <h4 class="mcp-stats-tools-title">${escapeHtml(distTitle)}</h4>
                    <span class="mcp-stats-tools-legend">${escapeHtml(distClickHint)}</span>
                </div>
                <span class="mcp-stats-tools-hint">${escapeHtml(distHint)}</span>
            </div>`;

    return `
        <div class="mcp-stats-dist-panel${embedded ? ' mcp-stats-dist-panel--embedded' : ''}" aria-label="${escapeHtml(distTitle)}"
            data-center-label="${escapeHtml(centerLabel)}"
            data-center-value="${top6SharePct}"
            data-center-suffix="%">
            ${headerHtml}
            <div class="${bodyClass}">
                <div class="mcp-stats-dist-chart-stage">
                    <div class="mcp-stats-dist-chart-wrap">
                        <svg class="mcp-stats-dist-svg" viewBox="0 0 100 100" role="img" aria-label="${escapeHtml(top6ShareLabel)} ${top6SharePct}%">
                            <g class="mcp-stats-dist-segments">${segmentPathsHtml}</g>
                        </svg>
                        <div class="mcp-stats-dist-donut-hole" aria-hidden="true">
                            <span class="mcp-stats-dist-donut-label is-default">${centerLabel}</span>
                            <span class="mcp-stats-dist-donut-value"><span class="mcp-stats-dist-donut-value-num">${top6SharePct}</span><span class="mcp-stats-dist-donut-unit">%</span></span>
                        </div>
                    </div>
                </div>
                ${legendBlock}
            </div>
        </div>
    `;
}


function renderMcpStatsStackedBar(success, failed) {
    const total = success + failed;
    if (total <= 0) {
        return '<div class="mcp-stats-stacked-bar" role="presentation"><div class="mcp-stats-stacked-bar-seg mcp-stats-stacked-bar-seg--success" style="flex:1"></div></div>';
    }
    const successFlex = Math.max(0, (success / total) * 100);
    const failFlex = Math.max(0, (failed / total) * 100);
    return `<div class="mcp-stats-stacked-bar" role="presentation">
        <div class="mcp-stats-stacked-bar-seg mcp-stats-stacked-bar-seg--success" style="flex:${successFlex}"></div>
        <div class="mcp-stats-stacked-bar-seg mcp-stats-stacked-bar-seg--fail" style="flex:${failFlex}"></div>
    </div>`;
}

function updateMonitorStatsSubtitle(lastFetchedAt, toolCount, retentionDays) {
    const subtitle = document.getElementById('monitor-stats-subtitle');
    if (!subtitle) return;
    const locale = (typeof window.__locale === 'string' && window.__locale.startsWith('zh')) ? 'zh-CN' : 'en-US';
    const timeText = lastFetchedAt
        ? (lastFetchedAt.toLocaleString ? lastFetchedAt.toLocaleString(locale) : String(lastFetchedAt))
        : '—';
    let text = mcpMonitorT('statsSubtitle', { time: timeText, count: toolCount })
        || monitorFallback(`最后刷新 ${timeText} · 共 ${toolCount} 个工具`, `Refreshed ${timeText} · ${toolCount} tools`);
    if (typeof retentionDays === 'number' && retentionDays > 0) {
        const hint = mcpMonitorT('retentionHint', { days: retentionDays })
            || monitorFallback(`执行记录保留 ${retentionDays} 天，超期自动清理`, `Execution records are kept for ${retentionDays} days, then purged automatically.`);
        text += ' · ' + hint;
    }
    subtitle.textContent = text;
    subtitle.hidden = false;
}

function filterMonitorByTool(toolName) {
    const toolFilter = document.getElementById('monitor-tool-filter');
    if (!toolFilter || !toolName) return;
    toolFilter.value = formatMonitorToolName(toolName);
    toolFilter.classList.add('is-filter-active');
    applyMonitorFilters();
    const execSection = document.querySelector('.monitor-executions');
    if (execSection && typeof execSection.scrollIntoView === 'function') {
        execSection.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
    }
}

function clearMonitorToolFilter() {
    const toolFilter = document.getElementById('monitor-tool-filter');
    if (!toolFilter) return;
    toolFilter.value = '';
    toolFilter.classList.remove('is-filter-active');
    applyMonitorFilters();
}

let monitorStatsPanelEventsBound = false;

function bindMonitorStatsPanelEvents() {
    if (monitorStatsPanelEventsBound) return;
    const root = document.getElementById('monitor-stats');
    if (!root) return;
    root.addEventListener('click', function (e) {
        const clearBtn = e.target.closest('.mcp-stats-clear-filter');
        if (clearBtn) {
            e.preventDefault();
            clearMonitorToolFilter();
            return;
        }
        const filterEl = e.target.closest(
            '.mcp-stats-dist-segment[data-tool-name], .mcp-stats-dist-legend-item[data-tool-name], ' +
            '.mcp-stats-proportion-seg[data-tool-name], .mcp-stats-tool-item[data-tool-name], tr.mcp-stats-tool-row[data-tool-name]'
        );
        if (filterEl && filterEl.getAttribute('data-is-others') !== '1') {
            const tool = filterEl.getAttribute('data-tool-name');
            if (tool) {
                e.preventDefault();
                handleMonitorStatsToolFilter(tool);
            }
            return;
        }
    });
    root.addEventListener('keydown', function (e) {
        if (e.key !== 'Enter' && e.key !== ' ') return;
        const filterEl = e.target.closest(
            '.mcp-stats-dist-segment[data-tool-name], .mcp-stats-proportion-seg[data-tool-name], ' +
            '.mcp-stats-tool-item[data-tool-name], tr.mcp-stats-tool-row[data-tool-name]'
        );
        if (!filterEl || filterEl.getAttribute('data-is-others') === '1') return;
        const tool = filterEl.getAttribute('data-tool-name');
        if (tool) {
            e.preventDefault();
            handleMonitorStatsToolFilter(tool);
        }
    });
    root.addEventListener('mouseover', function (e) {
        const el = e.target.closest(
            '.mcp-stats-dist-segment[data-tool-name], .mcp-stats-dist-legend-item[data-tool-name], ' +
            '.mcp-stats-proportion-seg[data-tool-name], .mcp-stats-tool-item[data-tool-name], tr.mcp-stats-tool-row[data-tool-name]'
        );
        if (!el || el.getAttribute('data-is-others') === '1') return;
        const tool = el.getAttribute('data-tool-name');
        if (tool) setMcpStatsDistHover(tool);
    });
    root.addEventListener('mouseout', function (e) {
        const el = e.target.closest(
            '.mcp-stats-dist-segment[data-tool-name], .mcp-stats-dist-legend-item[data-tool-name], ' +
            '.mcp-stats-proportion-seg[data-tool-name], .mcp-stats-tool-item[data-tool-name], tr.mcp-stats-tool-row[data-tool-name]'
        );
        if (!el) return;
        const related = e.relatedTarget;
        const next = related && related.closest
            ? related.closest(
                '.mcp-stats-dist-segment[data-tool-name], .mcp-stats-dist-legend-item[data-tool-name], ' +
                '.mcp-stats-proportion-seg[data-tool-name], .mcp-stats-tool-item[data-tool-name], tr.mcp-stats-tool-row[data-tool-name]'
            )
            : null;
        if (next) return;
        setMcpStatsDistHover('');
    });
    monitorStatsPanelEventsBound = true;
}

function renderMcpStatsMetricsBar(totals, successRate, rateTone, rateSubText, lastCallText, hasCalls = true) {
    const totalCallsLabel = mcpMonitorT('totalCalls') || monitorFallback('总调用次数', 'Total calls');
    const successRateLabel = mcpMonitorT('successRate') || monitorFallback('成功率', 'Success rate');
    const lastCallLabel = mcpMonitorT('lastCall') || monitorFallback('最近一次调用', 'Last call');
    const successPill = mcpMonitorT('successCount', { n: totals.success }) || monitorFallback(`成功 ${totals.success}`, `Success ${totals.success}`);
    const failedPill = mcpMonitorT('failedCount', { n: totals.failed }) || monitorFallback(`失败 ${totals.failed}`, `Failed ${totals.failed}`);
    const neutralPill = mcpMonitorT('neutralCount', { n: totals.neutral }) || monitorFallback(`终止 ${totals.neutral}`, `Stopped ${totals.neutral}`);
    const rateValue = hasCalls ? `${successRate}%` : successRate;
    const neutralChip = totals.neutral > 0
        ? `<span class="mcp-stats-kpi__chip is-neutral">${escapeHtml(neutralPill)}</span>`
        : '';

    return `
        <div class="mcp-stats-kpi" role="group" aria-label="${escapeHtml(totalCallsLabel)}">
            <article class="mcp-stats-kpi__item mcp-stats-kpi__item--calls">
                <span class="mcp-stats-kpi__accent" aria-hidden="true"></span>
                <div class="mcp-stats-kpi__content">
                    <span class="mcp-stats-kpi__label">${escapeHtml(totalCallsLabel)}</span>
                    <span class="mcp-stats-kpi__value">${totals.total}</span>
                    <div class="mcp-stats-kpi__meta">
                        <span class="mcp-stats-kpi__chip is-ok">${escapeHtml(successPill)}</span>
                        <span class="mcp-stats-kpi__chip is-fail">${escapeHtml(failedPill)}</span>
                        ${neutralChip}
                    </div>
                </div>
            </article>
            <article class="mcp-stats-kpi__item mcp-stats-kpi__item--rate">
                <span class="mcp-stats-kpi__accent" aria-hidden="true"></span>
                <div class="mcp-stats-kpi__content">
                    <span class="mcp-stats-kpi__label">${escapeHtml(successRateLabel)}</span>
                    <span class="mcp-stats-kpi__value mcp-stats-kpi__value--rate ${rateTone}">${rateValue}</span>
                    <span class="mcp-stats-kpi__status ${rateTone}">${escapeHtml(rateSubText)}</span>
                </div>
            </article>
            <article class="mcp-stats-kpi__item mcp-stats-kpi__item--time">
                <span class="mcp-stats-kpi__accent" aria-hidden="true"></span>
                <div class="mcp-stats-kpi__content">
                    <span class="mcp-stats-kpi__label">${escapeHtml(lastCallLabel)}</span>
                    <time class="mcp-stats-kpi__value mcp-stats-kpi__value--time">${escapeHtml(lastCallText)}</time>
                </div>
            </article>
        </div>`;
}

function renderMcpStatsToolTable(topTools, totals, activeToolFilter = '') {
    const colTool = mcpMonitorT('columnTool') || '工具';
    const colCalls = mcpMonitorT('columnCalls') || '调用';
    const colShare = mcpMonitorT('columnShare') || '占比';
    const colRate = mcpMonitorT('columnSuccessRate') || '成功率';
    const unknownToolLabel = mcpMonitorT('unknownTool') || '未知工具';

    let rowsHtml = '';
    topTools.forEach((tool, index) => {
        const rawName = tool.toolName || unknownToolLabel;
        const name = formatMonitorToolName(rawName);
        const total = tool.totalCalls || 0;
        const success = tool.successCalls || 0;
        const failed = tool.failedCalls || 0;
        const effectiveTotal = success + failed;
        const toolRateNum = effectiveTotal > 0 ? (success / effectiveTotal) * 100 : 0;
        const toolRate = toolRateNum.toFixed(1);
        const sharePct = totals.total > 0 ? ((total / totals.total) * 100).toFixed(1) : '0.0';
        const dotColor = MCP_STATS_DIST_COLORS[index % MCP_STATS_DIST_COLORS.length];
        const isActive = activeToolFilter && monitorToolNamesEqual(activeToolFilter, rawName);
        const rateClass = getMcpToolRateClass(toolRateNum);
        const rankClass = index === 0 ? ' rank-1' : index === 1 ? ' rank-2' : index === 2 ? ' rank-3' : '';
        const rowAria = mcpMonitorT('toolRowAriaLabel', { name, total, rate: toolRate })
            || `${name}，${total} 次调用，成功率 ${toolRate}%`;
        rowsHtml += `
            <tr class="mcp-stats-tool-row${isActive ? ' is-active' : ''}"
                data-tool-name="${escapeHtml(rawName)}"
                tabindex="0"
                role="button"
                aria-label="${escapeHtml(rowAria)}"
                aria-pressed="${isActive ? 'true' : 'false'}">
                <td class="col-rank"><span class="mcp-stats-rank${rankClass}">${index + 1}</span></td>
                <td class="col-tool" title="${escapeHtml(name)}">
                    <span class="mcp-stats-tool-dot" style="background:${dotColor}" aria-hidden="true"></span>
                    <span class="mcp-stats-tool-label">${escapeHtml(name)}</span>
                </td>
                <td class="col-num">${total}</td>
                <td class="col-share">${sharePct}%</td>
                <td class="col-rate">
                    <span class="mcp-stats-rate ${rateClass}">${toolRate}%</span>
                    ${failed > 0 ? `<span class="mcp-stats-fail-note">${escapeHtml(mcpMonitorT('failedCount', { n: failed }) || `失败 ${failed}`)}</span>` : ''}
                </td>
            </tr>`;
    });

    return `
        <table class="mcp-stats-tool-table">
            <thead>
                <tr>
                    <th class="col-rank" scope="col">#</th>
                    <th class="col-tool" scope="col">${escapeHtml(colTool)}</th>
                    <th class="col-num" scope="col">${escapeHtml(colCalls)}</th>
                    <th class="col-share" scope="col">${escapeHtml(colShare)}</th>
                    <th class="col-rate" scope="col">${escapeHtml(colRate)}</th>
                </tr>
            </thead>
            <tbody>${rowsHtml}</tbody>
        </table>`;
}

/** MCP 合并面板左侧：堆叠占比条 + 工具排行列表（无饼图/表格套娃） */
function renderMcpStatsToolsPanel(topTools, totals, activeToolFilter = '') {
    const segments = buildMcpStatsChartSegments(topTools, totals, { groupSmall: false });
    const topNTotal = topTools.reduce((s, t) => s + (t.totalCalls || 0), 0);
    const topNSharePct = totals.total > 0 ? ((topNTotal / totals.total) * 100).toFixed(1) : '0.0';
    const caption = mcpMonitorT('rankingSummary', { n: MCP_STATS_TOP_N, pct: topNSharePct, total: totals.total })
        || `Top ${MCP_STATS_TOP_N} 占 ${topNSharePct}% · 共 ${totals.total} 次`;
    const unknownToolLabel = mcpMonitorT('unknownTool') || '未知工具';
    const distAria = mcpMonitorT('distTitle') || '调用分布';

    const stackedHtml = segments.map((s) => {
        const isActive = !s.isOthers && activeToolFilter && monitorToolNamesEqual(activeToolFilter, s.name);
        const displayName = s.isOthers ? s.name : formatMonitorToolName(s.name);
        const title = `${displayName} · ${s.pct}% · ${s.calls}`;
        if (s.isOthers) {
            return `<span class="mcp-stats-proportion-seg is-others" data-is-others="1" role="presentation"
                style="flex:${s.pctNum} 1 0;background:${s.color}" title="${escapeHtml(title)}"></span>`;
        }
        const segAria = mcpMonitorT('distSegmentAria', { name: displayName, pct: s.pct, calls: s.calls })
            || `${displayName}，占 ${s.pct}%，${s.calls} 次`;
        return `<span class="mcp-stats-proportion-seg${isActive ? ' is-active' : ''}"
            data-tool-name="${escapeHtml(s.name)}" data-pct="${s.pct}" data-calls="${s.calls}" data-is-others="0"
            role="button" tabindex="0" aria-label="${escapeHtml(segAria)}"
            style="flex:${s.pctNum} 1 0;background:${s.color}" title="${escapeHtml(title)}"></span>`;
    }).join('');

    const maxCalls = Math.max(1, ...topTools.map((t) => t.totalCalls || 0));
    const listHtml = topTools.map((tool, index) => {
        const rawName = tool.toolName || unknownToolLabel;
        const name = formatMonitorToolName(rawName);
        const total = tool.totalCalls || 0;
        const success = tool.successCalls || 0;
        const failed = tool.failedCalls || 0;
        const effectiveTotal = success + failed;
        const toolRateNum = effectiveTotal > 0 ? (success / effectiveTotal) * 100 : 0;
        const toolRate = toolRateNum.toFixed(1);
        const sharePct = totals.total > 0 ? ((total / totals.total) * 100).toFixed(1) : '0.0';
        const color = MCP_STATS_DIST_COLORS[index % MCP_STATS_DIST_COLORS.length];
        const barPct = maxCalls > 0 ? ((total / maxCalls) * 100).toFixed(1) : '0';
        const isActive = activeToolFilter && monitorToolNamesEqual(activeToolFilter, rawName);
        const rateClass = getMcpToolRateClass(toolRateNum);
        const rankClass = index === 0 ? ' rank-1' : index === 1 ? ' rank-2' : index === 2 ? ' rank-3' : '';
        const rowAria = mcpMonitorT('toolRowAriaLabel', { name, total, rate: toolRate })
            || `${name}，${total} 次，成功率 ${toolRate}%`;
        const failNote = failed > 0
            ? `<span class="mcp-stats-tool-item__fail">${escapeHtml(mcpMonitorT('failedCount', { n: failed }) || `失败 ${failed}`)}</span>`
            : '';
        const successLabel = mcpMonitorT('successCount', { n: success }) || `成功 ${success}`;
        const failedLabel = mcpMonitorT('failedCount', { n: failed }) || `失败 ${failed}`;
        return `<li class="mcp-stats-tool-item${isActive ? ' is-active' : ''}"
            data-tool-name="${escapeHtml(rawName)}" tabindex="0" role="button"
            aria-label="${escapeHtml(rowAria)}" aria-pressed="${isActive ? 'true' : 'false'}">
            <div class="mcp-stats-tool-item__top">
                <span class="mcp-stats-tool-item__rank mcp-stats-rank${rankClass}">${index + 1}</span>
                <span class="mcp-stats-tool-item__dot" style="background:${color}" aria-hidden="true"></span>
                <span class="mcp-stats-tool-item__name" title="${escapeHtml(name)}">${escapeHtml(name)}</span>
                <span class="mcp-stats-tool-item__share">${sharePct}%</span>
            </div>
            <div class="mcp-stats-tool-item__middle">
                <strong class="mcp-stats-tool-item__calls">${total}</strong>
                <span class="mcp-stats-tool-item__calls-label">${escapeHtml(mcpMonitorT('columnCalls') || '调用')}</span>
                <span class="mcp-stats-tool-item__track" aria-hidden="true">
                    <span class="mcp-stats-tool-item__fill" style="width:${barPct}%;background:${color}"></span>
                </span>
            </div>
            <div class="mcp-stats-tool-item__bottom">
                <span class="mcp-stats-tool-item__pill is-success">${escapeHtml(successLabel)}</span>
                <span class="mcp-stats-tool-item__pill${failed > 0 ? ' is-danger' : ''}">${escapeHtml(failedLabel)}</span>
                <span class="mcp-stats-tool-item__rate ${rateClass}">${toolRate}%${failNote}</span>
            </div>
        </li>`;
    }).join('');

    return `
        <div class="mcp-stats-tools-panel" role="region" aria-label="${escapeHtml(mcpMonitorT('toolStatsTitle') || '工具统计')}">
            <div class="mcp-stats-tools-panel__hero">
                <div class="mcp-stats-proportion-bar" role="img" aria-label="${escapeHtml(distAria)}">${stackedHtml}</div>
                <p class="mcp-stats-tools-panel__caption">
                    <span class="mcp-stats-scope-badge mcp-stats-scope-badge--cumulative mcp-stats-scope-badge--inline">${escapeHtml(mcpMonitorT('scopeCumulative') || '累计')}</span>
                    ${escapeHtml(caption)}
                </p>
            </div>
            <ol class="mcp-stats-tools-panel__list">${listHtml}</ol>
        </div>`;
}

function renderMcpStatsChartAside(topTools, totals, activeToolFilter = '') {
    const distTitle = mcpMonitorT('distTitle') || '调用分布';
    const distClickHint = mcpMonitorT('distClickHint') || '点击扇区筛选';
    const top6ShareLabel = mcpMonitorT('distTop6Share', { n: MCP_STATS_TOP_N }) || `Top ${MCP_STATS_TOP_N} 占全部调用`;
    const topNTotal = topTools.reduce((s, t) => s + (t.totalCalls || 0), 0);
    const top6SharePct = totals.total > 0 ? ((topNTotal / totals.total) * 100).toFixed(1) : '0.0';
    const centerLabel = `Top ${MCP_STATS_TOP_N}`;

    const segments = buildMcpStatsChartSegments(topTools, totals, { groupSmall: true });
    const segmentPathsHtml = segments.map((s) => {
        const pathD = mcpStatsDescribeDonutSegment(s.start, s.end, 48, 30);
        if (!pathD) return '';
        const isActive = !s.isOthers && activeToolFilter && activeToolFilter === s.name;
        const segAria = s.isOthers
            ? escapeHtml(s.name)
            : escapeHtml(mcpMonitorT('distSegmentAria', { name: s.name, pct: s.pct, calls: s.calls })
                || `${s.name}，占 ${s.pct}%，${s.calls} 次`);
        return `<path class="mcp-stats-dist-segment${isActive ? ' is-active' : ''}${s.isOthers ? ' is-others' : ''}"
            d="${pathD}" fill="${s.color}"
            data-tool-name="${s.isOthers ? '' : escapeHtml(s.name)}"
            data-pct="${s.pct}" data-calls="${s.calls}"
            data-is-others="${s.isOthers ? '1' : '0'}"
            tabindex="${s.isOthers ? '-1' : '0'}"
            role="${s.isOthers ? 'presentation' : 'button'}"
            aria-label="${segAria}" />`;
    }).join('');

    return `
        <div class="mcp-stats-dist-panel mcp-stats-dist-panel--compact"
            aria-label="${escapeHtml(distTitle)}"
            data-center-label="${escapeHtml(centerLabel)}"
            data-center-value="${top6SharePct}"
            data-center-suffix="%">
            <p class="mcp-stats-panel__aside-title">${escapeHtml(distTitle)}</p>
            <div class="mcp-stats-panel__chart">
                <svg class="mcp-stats-dist-svg" viewBox="0 0 100 100" role="img" aria-label="${escapeHtml(top6ShareLabel)} ${top6SharePct}%">
                    <g class="mcp-stats-dist-segments">${segmentPathsHtml}</g>
                </svg>
                <div class="mcp-stats-dist-donut-hole" aria-hidden="true">
                    <span class="mcp-stats-dist-donut-label is-default">${centerLabel}</span>
                    <span class="mcp-stats-dist-donut-value">
                        <span class="mcp-stats-dist-donut-value-num">${top6SharePct}</span>
                        <span class="mcp-stats-dist-donut-unit">%</span>
                    </span>
                </div>
            </div>
            <p class="mcp-stats-panel__aside-hint">${escapeHtml(distClickHint)}</p>
        </div>`;
}

function renderMcpStatsDetailSection(topTools, totals, activeToolFilter = '', timeline = null, timelineError = null) {
    const showTimeline = timeline != null || !!timelineError;
    return renderMcpStatsCombinedSection(topTools, totals, activeToolFilter, timeline, timelineError, showTimeline);
}

/** @deprecated 保留供其他页面；MCP 监控主面板请用 renderMcpStatsToolTable */
function renderMcpStatsToolRanking(topTools, totals, activeToolFilter = '', options = {}) {
    if (options.bare || options.embedded) {
        return renderMcpStatsToolTable(topTools, totals, activeToolFilter);
    }
    return renderMcpStatsDetailSection(topTools, totals, activeToolFilter);
}

function renderMonitorStats(summary = null, topTools = [], lastFetchedAt = null) {
    const container = document.getElementById('monitor-stats');
    if (!container) {
        return;
    }

    const tools = Array.isArray(topTools) ? topTools : [];
    const totals = buildMonitorTotals(summary);
    const toolCount = summary && typeof summary.toolCount === 'number' ? summary.toolCount : tools.length;
    const showTimeline = monitorState.timelineLoading || monitorState.timeline != null || !!monitorState.timelineError;
    const hasSummaryData = toolCount > 0 || totals.total > 0;

    if (!hasSummaryData && !showTimeline) {
        const noStats = mcpMonitorT('noStatsData') || monitorFallback('暂无统计数据', 'No statistical data');
        container.innerHTML = '<div class="monitor-empty">' + escapeHtml(noStats) + '</div>';
        const subtitle = document.getElementById('monitor-stats-subtitle');
        if (subtitle) subtitle.hidden = true;
        return;
    }

    const effectiveTotal = totals.success + totals.failed;
    const hasCalls = effectiveTotal > 0;
    const successRateNum = hasCalls ? (totals.success / effectiveTotal) * 100 : 0;
    const successRate = hasCalls ? successRateNum.toFixed(1) : '-';
    const locale = (typeof window.__locale === 'string' && window.__locale.startsWith('zh')) ? 'zh-CN' : 'en-US';
    const noCallsYet = mcpMonitorT('noCallsYet') || monitorFallback('暂无调用', 'No calls yet');
    const noCompletedYet = mcpMonitorT('noCompletedYet') || monitorFallback('暂无有效完成', 'No completed outcomes yet');
    const lastCallText = totals.lastCallTime
        ? (totals.lastCallTime.toLocaleString ? totals.lastCallTime.toLocaleString(locale) : String(totals.lastCallTime))
        : noCallsYet;

    const rateTone = hasCalls ? getMcpStatsRateTone(successRateNum) : 'is-muted';
    let rateSubText = totals.total > 0 ? noCompletedYet : noCallsYet;
    if (hasCalls) {
        rateSubText = mcpMonitorT('rateHealthy') || monitorFallback('运行平稳', 'Running smoothly');
        if (successRateNum < 80) rateSubText = mcpMonitorT('rateCritical') || monitorFallback('失败率偏高', 'High failure rate');
        else if (successRateNum < 95) rateSubText = mcpMonitorT('rateWarning') || monitorFallback('存在失败调用', 'Some failures detected');
    }

    const toolFilterEl = document.getElementById('monitor-tool-filter');
    const activeToolFilter = toolFilterEl ? toolFilterEl.value.trim() : '';

    const hasAnyCalls = totals.total > 0;
    const showCombined = hasAnyCalls && (tools.length > 0 || showTimeline);
    const html = `
        <div class="mcp-exec-stats">
            ${renderMcpStatsMetricsBar(totals, successRate, rateTone, rateSubText, lastCallText, hasCalls)}
            ${showCombined ? renderMcpStatsCombinedSection(
                tools,
                totals,
                activeToolFilter,
                monitorState.timeline,
                monitorState.timelineError,
                showTimeline
            ) : ''}
        </div>
    `;

    container.innerHTML = html;
    bindMonitorStatsPanelEvents();
    bindMcpStatsTimelineEvents();
    if (toolFilterEl && activeToolFilter) {
        toolFilterEl.classList.add('is-filter-active');
    } else if (toolFilterEl) {
        toolFilterEl.classList.remove('is-filter-active');
    }
    updateMonitorStatsSubtitle(lastFetchedAt, toolCount, monitorState.retentionDays);
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
        const noRecordsFilter = typeof window.t === 'function' ? window.t('mcpMonitor.noRecordsWithFilter') : monitorFallback('当前筛选条件下暂无记录', 'No records with current filter');
        const noExecutions = typeof window.t === 'function' ? window.t('mcpMonitor.noExecutions') : monitorFallback('暂无执行记录', 'No execution records');
        if (hasFilter) {
            container.innerHTML = '<div class="monitor-empty">' + escapeHtml(noRecordsFilter) + '</div>';
        } else {
            const emptyHint = typeof window.t === 'function' ? window.t('mcpMonitor.emptyHint') : monitorFallback('在对话或任务中调用 MCP 工具后，执行记录将显示在此处', 'Execution records will appear here after you invoke MCP tools in chat or tasks');
            container.innerHTML = `<div class="monitor-empty">
                <p class="monitor-empty__title">${escapeHtml(noExecutions)}</p>
                <p class="monitor-empty__hint">${escapeHtml(emptyHint)}</p>
            </div>`;
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
    const terminateLabel = typeof window.t === 'function' ? window.t('mcpMonitor.terminateExecution') : '终止';
    const statusKeyMap = {
        pending: 'statusPending',
        queued: 'statusQueued',
        running: 'statusRunning',
        completed: 'statusCompleted',
        failed: 'statusFailed',
        cancelled: 'statusCancelled',
        hard_timeout: 'statusHardTimeout',
        orphaned: 'statusOrphaned'
    };
    const locale = (typeof window.__locale === 'string' && window.__locale.startsWith('zh')) ? 'zh-CN' : undefined;
    const rowEntries = executions
        .map(exec => {
            const status = (exec.status || 'unknown').toLowerCase();
            const statusClass = `monitor-status-chip ${status}`;
            const statusKey = statusKeyMap[status];
            const statusLabel = (typeof window.t === 'function' && statusKey) ? window.t('mcpMonitor.' + statusKey) : getStatusText(status);
            const startTime = exec.startTime ? (new Date(exec.startTime).toLocaleString ? new Date(exec.startTime).toLocaleString(locale || 'en-US') : String(exec.startTime)) : unknownLabel;
            const duration = formatExecutionDuration(exec.startTime, exec.endTime);
            const toolName = escapeHtml(formatMonitorToolName(exec.toolName) || unknownToolLabel);
            const rawExecId = exec.id || '';
            const executionId = escapeHtml(rawExecId);
            const terminateBtn = status === 'running'
                ? `<button type="button" class="btn-secondary btn-monitor-abort" data-require-permission="monitor:write" onclick="cancelMCPToolExecution('${rawExecId.replace(/\\/g, '\\\\').replace(/'/g, "\\'")}')">${escapeHtml(terminateLabel)}</button>`
                : '';
            const jsExecId = rawExecId.replace(/\\/g, '\\\\').replace(/'/g, "\\'");
            const isSelected = monitorState.selectedExecutions.has(rawExecId);
            const rowKey = monitorRenderKey([exec, isSelected, locale || 'en-US']);
            return {
                id: rawExecId,
                key: rowKey,
                html: `
                <tr data-execution-id="${executionId}">
                    <td>
                        <input type="checkbox" class="monitor-execution-checkbox theme-checkbox" value="${executionId}" ${isSelected ? 'checked' : ''} onchange="toggleExecutionSelection('${jsExecId}', this.checked)" />
                    </td>
                    <td>${toolName}</td>
                    <td><span class="${statusClass}">${escapeHtml(statusLabel)}</span></td>
                    <td>${escapeHtml(startTime)}</td>
                    <td class="monitor-execution-duration">${escapeHtml(duration)}</td>
                    <td>
                        <div class="monitor-execution-actions">
                            <button class="btn-secondary" onclick="showMCPDetail('${executionId}')">${escapeHtml(viewDetailLabel)}</button>
                            ${terminateBtn}
                            <button class="btn-secondary btn-delete" data-require-permission="monitor:delete" onclick="deleteExecution('${executionId}')" title="${escapeHtml(deleteExecTitle)}">${escapeHtml(deleteLabel)}</button>
                        </div>
                    </td>
                </tr>
            `
            };
        });

    // 清除"加载中..."等提示信息
    const oldEmpty = container.querySelector('.monitor-empty');
    if (oldEmpty) {
        oldEmpty.remove();
    }
    
    const colTool = typeof window.t === 'function' ? window.t('mcpMonitor.columnTool') : '工具';
    const colStatus = typeof window.t === 'function' ? window.t('mcpMonitor.columnStatus') : '状态';
    const colStartTime = typeof window.t === 'function' ? window.t('mcpMonitor.columnStartTime') : '开始时间';
    const colDuration = typeof window.t === 'function' ? window.t('mcpMonitor.columnDuration') : '耗时';
    const colActions = typeof window.t === 'function' ? window.t('mcpMonitor.columnActions') : '操作';
    const headerKey = monitorRenderKey([colTool, colStatus, colStartTime, colDuration, colActions, locale || 'en-US']);
    const headerHtml = `
        <tr>
            <th style="width: 40px;">
                <input type="checkbox" id="monitor-select-all" class="theme-checkbox" onchange="toggleSelectAll(this)" />
            </th>
            <th>${escapeHtml(colTool)}</th>
            <th>${escapeHtml(colStatus)}</th>
            <th>${escapeHtml(colStartTime)}</th>
            <th>${escapeHtml(colDuration)}</th>
            <th>${escapeHtml(colActions)}</th>
        </tr>`;

    let tableContainer = container.querySelector('.monitor-table-container');
    let tableCreated = false;
    if (!tableContainer) {
        tableContainer = document.createElement('div');
        tableContainer.className = 'monitor-table-container';
        tableContainer.innerHTML = '<table class="monitor-table"><thead></thead><tbody></tbody></table>';
        const existingPagination = container.querySelector('.monitor-pagination');
        if (existingPagination) {
            container.insertBefore(tableContainer, existingPagination);
        } else {
            container.appendChild(tableContainer);
        }
        tableCreated = true;
    }

    const table = tableContainer.querySelector('.monitor-table');
    const thead = table && table.querySelector('thead');
    if (thead && table.__monitorHeaderKey !== headerKey) {
        thead.innerHTML = headerHtml;
        table.__monitorHeaderKey = headerKey;
    }
    const changedRows = table
        ? reconcileMonitorExecutionRows(table.querySelector('tbody'), rowEntries)
        : [];
    
    // 更新批量操作状态
    updateBatchActionsState();
    if (typeof rbacAfterDynamicRender === 'function') {
        if (tableCreated) {
            rbacAfterDynamicRender(tableContainer);
        } else {
            changedRows.forEach(function (row) { rbacAfterDynamicRender(row); });
        }
    }
}

function createMonitorExecutionRow(entry) {
    const template = document.createElement('template');
    template.innerHTML = entry.html.trim();
    const row = template.content.firstElementChild;
    if (row) row.__monitorRenderKey = entry.key;
    return row;
}

// 以 execution ID 为 key 对账，只操作新增、删除、换序或内容变化的行。
function reconcileMonitorExecutionRows(tbody, rowEntries) {
    if (!tbody) return [];

    const existingById = new Map();
    Array.from(tbody.children).forEach(function (row) {
        existingById.set(row.dataset.executionId || '', row);
    });

    const desiredIds = new Set(rowEntries.map(function (entry) { return entry.id; }));
    existingById.forEach(function (row, id) {
        if (!desiredIds.has(id)) row.remove();
    });

    const changedRows = [];
    let cursor = tbody.firstElementChild;
    rowEntries.forEach(function (entry) {
        let row = existingById.get(entry.id);
        if (!row || row.__monitorRenderKey !== entry.key) {
            const nextRow = createMonitorExecutionRow(entry);
            if (!nextRow) return;
            if (row && row.parentNode === tbody) {
                const replacingCursor = row === cursor;
                row.replaceWith(nextRow);
                if (replacingCursor) cursor = nextRow;
            }
            row = nextRow;
            existingById.set(entry.id, row);
            changedRows.push(row);
        }

        if (row !== cursor) {
            tbody.insertBefore(row, cursor);
        }
        cursor = row.nextElementSibling;
    });

    return changedRows;
}

// 轮询结果未变化时只原位刷新运行中记录的耗时，避免重建整张表格。
function updateMonitorExecutionDurations(executions = []) {
    const container = document.getElementById('monitor-executions');
    if (!container || !Array.isArray(executions)) return;

    const executionMap = new Map();
    executions.forEach(function (execution) {
        if (execution && execution.id) executionMap.set(String(execution.id), execution);
    });

    container.querySelectorAll('tr[data-execution-id]').forEach(function (row) {
        const execution = executionMap.get(row.dataset.executionId || '');
        if (!execution || String(execution.status || '').toLowerCase() !== 'running') return;
        const durationCell = row.querySelector('.monitor-execution-duration');
        if (durationCell) {
            durationCell.textContent = formatExecutionDuration(execution.startTime, execution.endTime);
        }
    });
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
    const paginationInfoText = mcpMonitorT('paginationInfo', { start: startItem, end: endItem, total: total })
        || (typeof window.t === 'function' ? window.t('mcpMonitor.paginationInfo', { start: startItem, end: endItem, total: total }) : `Show ${startItem}-${endItem} of ${total} records`);
    const perPageLabel = mcpMonitorT('perPageLabel') || (typeof window.t === 'function' ? window.t('mcpMonitor.perPageLabel') : 'Per page');
    const firstPageLabel = mcpMonitorT('firstPage') || (typeof window.t === 'function' ? window.t('mcp.firstPage') : 'First');
    const prevPageLabel = mcpMonitorT('prevPage') || (typeof window.t === 'function' ? window.t('mcp.prevPage') : 'Previous');
    const pageInfoText = mcpMonitorT('pageInfo', { page: page, total: totalPages || 1 })
        || (typeof window.t === 'function' ? window.t('mcp.pageInfo', { page: page, total: totalPages || 1 }) : `Page ${page} / ${totalPages || 1}`);
    const nextPageLabel = mcpMonitorT('nextPage') || (typeof window.t === 'function' ? window.t('mcp.nextPage') : 'Next');
    const lastPageLabel = mcpMonitorT('lastPage') || (typeof window.t === 'function' ? window.t('mcp.lastPage') : 'Last');
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
        
        monitorState.selectedExecutions.delete(executionId);

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

// 切换单条执行记录选中状态（持久化到 monitorState，避免轮询刷新后丢失）
function toggleExecutionSelection(executionId, selected) {
    if (!executionId) {
        return;
    }
    if (selected) {
        monitorState.selectedExecutions.add(executionId);
    } else {
        monitorState.selectedExecutions.delete(executionId);
    }
    updateBatchActionsState();
}

// 更新批量操作状态
function updateBatchActionsState() {
    const selectedCount = monitorState.selectedExecutions.size;
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
    
    // 更新全选复选框状态（仅反映当前页）
    const selectAllCheckbox = document.getElementById('monitor-select-all');
    if (selectAllCheckbox) {
        const allCheckboxes = document.querySelectorAll('.monitor-execution-checkbox');
        if (allCheckboxes.length === 0) {
            selectAllCheckbox.checked = false;
            selectAllCheckbox.indeterminate = false;
        } else {
            const checkedOnPage = Array.from(allCheckboxes).filter(cb => monitorState.selectedExecutions.has(cb.value)).length;
            selectAllCheckbox.checked = checkedOnPage === allCheckboxes.length;
            selectAllCheckbox.indeterminate = checkedOnPage > 0 && checkedOnPage < allCheckboxes.length;
        }
    }
}

// 切换全选
function toggleSelectAll(checkbox) {
    const checkboxes = document.querySelectorAll('.monitor-execution-checkbox');
    checkboxes.forEach(cb => {
        cb.checked = checkbox.checked;
        if (checkbox.checked) {
            monitorState.selectedExecutions.add(cb.value);
        } else {
            monitorState.selectedExecutions.delete(cb.value);
        }
    });
    updateBatchActionsState();
}

// 全选
function selectAllExecutions() {
    const checkboxes = document.querySelectorAll('.monitor-execution-checkbox');
    checkboxes.forEach(cb => {
        cb.checked = true;
        monitorState.selectedExecutions.add(cb.value);
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
    monitorState.selectedExecutions.clear();
    const selectAllCheckbox = document.getElementById('monitor-select-all');
    if (selectAllCheckbox) {
        selectAllCheckbox.checked = false;
        selectAllCheckbox.indeterminate = false;
    }
    updateBatchActionsState();
}

// 批量删除执行记录
async function batchDeleteExecutions() {
    const ids = Array.from(monitorState.selectedExecutions);
    if (ids.length === 0) {
        const selectFirstMsg = typeof window.t === 'function' ? window.t('mcpMonitor.selectExecFirst') : '请先选择要删除的执行记录';
        alert(selectFirstMsg);
        return;
    }
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

        ids.forEach(function (id) {
            monitorState.selectedExecutions.delete(id);
        });
        
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
            let pdata = null;
            if (msgEl.dataset.progressRawData) {
                try {
                    pdata = JSON.parse(msgEl.dataset.progressRawData);
                } catch (e) {
                    pdata = null;
                }
            }
            titleEl.textContent = '\uD83D\uDD0D ' + translateProgressMessage(raw, pdata);
        }
    });
    // 转换后的详情区顶栏「任务执行详情」：仅刷新不在 .progress-message 内的 progress 标题
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
            if (item.dataset.orchestration === 'plan_execute' && scope === 'main') {
                const phase = typeof translatePlanExecuteAgentName === 'function'
                    ? translatePlanExecuteAgentName(item.dataset.einoAgent) : (item.dataset.einoAgent || '');
                titleSpan.textContent = _t('chat.einoPlanExecuteRound', { n: n, phase: phase });
            } else if (scope === 'main') {
                titleSpan.textContent = _t('chat.einoOrchestratorRound', { n: n });
            } else if (scope === 'sub') {
                const agent = item.dataset.einoAgent || '';
                titleSpan.textContent = _t('chat.einoSubAgentStep', { n: n, agent: agent });
            } else {
                titleSpan.textContent = ap + _t('chat.iterationRound', { n: n });
            }
        } else if (type === 'thinking') {
            if (item.dataset.responseStreamPlaceholder === '1' && typeof einoMainStreamPlanningTitle === 'function') {
                titleSpan.textContent = einoMainStreamPlanningTitle({
                    orchestration: item.dataset.orchestration || '',
                    einoAgent: item.dataset.einoAgent || ''
                });
            } else if (item.dataset.orchestration === 'plan_execute' && item.dataset.einoAgent && typeof einoMainStreamPlanningTitle === 'function') {
                titleSpan.textContent = einoMainStreamPlanningTitle({
                    orchestration: 'plan_execute',
                    einoAgent: item.dataset.einoAgent
                });
            } else {
                titleSpan.textContent = ap + '\uD83E\uDD14 ' + _t('chat.aiThinking');
            }
        } else if (type === 'reasoning_chain') {
            titleSpan.textContent = ap + '\uD83D\uDD17 ' + _t('chat.reasoningChain');
        } else if (type === 'planning') {
            if (item.dataset.orchestration && typeof einoMainStreamPlanningTitle === 'function') {
                titleSpan.textContent = einoMainStreamPlanningTitle({
                    orchestration: item.dataset.orchestration,
                    einoAgent: item.dataset.einoAgent || ''
                });
            } else {
                titleSpan.textContent = ap + '\uD83D\uDCDD ' + _t('chat.planning');
            }
        } else if (type === 'tool_calls_detected' && item.dataset.toolCallsCount != null) {
            const count = parseInt(item.dataset.toolCallsCount, 10) || 0;
            titleSpan.textContent = ap + '\uD83D\uDD27 ' + _t('chat.toolCallsDetected', { count: count });
        } else if (type === 'tool_call' && (item.dataset.toolName !== undefined || item.dataset.toolIndex !== undefined)) {
            const name = (item.dataset.toolName != null && item.dataset.toolName !== '') ? item.dataset.toolName : _t('chat.unknownTool');
            const index = parseInt(item.dataset.toolIndex, 10) || 0;
            const total = parseInt(item.dataset.toolTotal, 10) || 0;
            const callTitle = typeof formatToolCallTimelineTitle === 'function'
                ? formatToolCallTimelineTitle(name, index, total)
                : _t('chat.callTool', { name: name, index: index, total: total });
            titleSpan.textContent = ap + '\uD83D\uDD27 ' + callTitle;
        } else if (type === 'tool_result' && (item.dataset.toolName !== undefined || item.dataset.toolSuccess !== undefined)) {
            const name = (item.dataset.toolName != null && item.dataset.toolName !== '') ? item.dataset.toolName : _t('chat.unknownTool');
            const displayStatus = item.dataset.toolDisplayStatus || '';
            const backgroundRunning = displayStatus === 'background_running';
            const success = item.dataset.toolSuccess === '1';
            const icon = backgroundRunning ? '\u23F3 ' : (success ? '\u2705 ' : '\u274C ');
            titleSpan.textContent = ap + icon + (backgroundRunning ? (getBackgroundRunningToolLabel() + ': ' + name) : (success ? _t('chat.toolExecComplete', { name: name }) : _t('chat.toolExecFailed', { name: name })));
        } else if (type === 'eino_agent_reply') {
            titleSpan.textContent = ap + '\uD83D\uDCAC ' + _t('chat.einoAgentReplyTitle');
        } else if (type === 'cancelled') {
            titleSpan.textContent = '\u26D4 ' + _t('chat.taskCancelled');
        } else if (type === 'user_interrupt_continue') {
            titleSpan.textContent = _t('chat.userInterruptContinueTitle');
        } else if (type === 'progress' && item.dataset.progressMessage !== undefined) {
            titleSpan.textContent = typeof window.translateProgressMessage === 'function' ? window.translateProgressMessage(item.dataset.progressMessage) : item.dataset.progressMessage;
        }
        if (timeSpan && item.dataset.createdAtIso) {
            const d = new Date(item.dataset.createdAtIso);
            if (!isNaN(d.getTime())) {
                timeSpan.textContent = d.toLocaleTimeString(timeLocale, timeOpts);
            }
        }
        if (item.classList.contains('tool-detail-collapsible') && typeof updateToolDetailToggleLabel === 'function') {
            updateToolDetailToggleLabel(item);
        }
    });

    document.querySelectorAll('.timeline-live-pruned-marker').forEach(function (marker) {
        marker.textContent = formatLiveTimelinePrunedMarker(marker);
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

    document.querySelectorAll('#chat-messages .message.assistant').forEach(function (msgEl) {
        if (typeof window.syncMcpToolsToggleButton === 'function') {
            window.syncMcpToolsToggleButton(msgEl);
        }
    });

    const copyLabel = _t('common.copy');
    const copyTitle = _t('chat.copyMessageTitle');
    document.querySelectorAll('#chat-messages .message-copy-btn').forEach(function (btn) {
        if (btn.dataset.copySuccessActive === '1') return;
        const span = btn.querySelector('span');
        if (span) span.textContent = copyLabel;
        btn.title = copyTitle;
        btn.setAttribute('aria-label', copyTitle);
    });
}

document.addEventListener('languagechange', function () {
    updateBatchActionsState();
    loadActiveTasks();
    refreshProgressAndTimelineI18n();
    syncAllMonitorFilterSelects();
    refreshMonitorPanelFromState();
});

document.addEventListener('DOMContentLoaded', function () {
    initMonitorFilterSelects();
    bindMonitorStatsPanelEvents();
    if (window.i18nReady && typeof window.i18nReady.then === 'function') {
        window.i18nReady.then(function () {
            refreshMonitorPanelFromState();
        });
    }
});

window.filterMonitorByTool = filterMonitorByTool;
window.clearMonitorToolFilter = clearMonitorToolFilter;
