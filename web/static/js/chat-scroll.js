/**
 * 主对话区智能粘底滚动：流式输出时自动跟随，用户上滑阅读时不抢焦点。
 * 主 POST 流（sendMessage）与刷新后 task-events 补流共用同一策略。
 */
(function () {
    'use strict';

    /** 距底部在此范围内才继续自动跟随（宜小，避免“差一点也被拽回去”） */
    const CHAT_SCROLL_FOLLOW_THRESHOLD_PX = 48;
    /** FAB 隐藏：用户已手动滚近底部 */
    const CHAT_SCROLL_FAB_HIDE_THRESHOLD_PX = 120;
    /** 用户上滑后的短暂锁，防止 SSE 与 scroll 事件竞态抢滚动 */
    const DETACH_LOCK_MS = 280;

    /** @type {'following' | 'detached'} */
    let scrollMode = 'following';
    let scrollFollowRaf = 0;
    /** 用户脱离跟随后，下方是否有未读的新输出（不按 SSE 次数计） */
    let hasPendingNewBelow = false;
    let listenersBound = false;
    let lastScrollTop = 0;
    let programmaticScroll = false;
    let detachLockUntil = 0;

    function getChatMessagesEl() {
        return document.getElementById('chat-messages');
    }

    /** 主 POST 流 + 刷新后 task-events 补流均视为「流式进行中」 */
    function isStreamActive() {
        try {
            const live = window.__csAgentLiveStream;
            if (live && live.active) return true;
            const replay = window.__csTaskEventStream;
            return !!(replay && replay.active);
        } catch (e) {
            return false;
        }
    }

    function distanceFromBottom(el) {
        if (!el) return 0;
        const { scrollTop, scrollHeight, clientHeight } = el;
        return scrollHeight - clientHeight - scrollTop;
    }

    function isNearBottom(thresholdPx) {
        const el = getChatMessagesEl();
        if (!el) return true;
        return distanceFromBottom(el) <= thresholdPx;
    }

    function isVerticallyScrollable(el) {
        if (!el || el.scrollHeight <= el.clientHeight + 1) return false;
        try {
            const overflowY = window.getComputedStyle(el).overflowY;
            return overflowY === 'auto' || overflowY === 'scroll' || overflowY === 'overlay';
        } catch (e) {
            return true;
        }
    }

    function getScrollableProcessDetailElements() {
        const root = getChatMessagesEl();
        if (!root) return [];
        const targets = [];
        root.querySelectorAll('.progress-timeline.expanded').forEach(function (timeline) {
            if (isVerticallyScrollable(timeline)) targets.push(timeline);
            timeline.querySelectorAll('*').forEach(function (el) {
                if (isVerticallyScrollable(el)) targets.push(el);
            });
        });
        return Array.from(new Set(targets));
    }

    function scrollProcessDetailsTo(position, smooth) {
        const top = position === 'top' ? 0 : null;
        getScrollableProcessDetailElements().forEach(function (el) {
            const targetTop = top === null ? el.scrollHeight : top;
            if (smooth && typeof el.scrollTo === 'function') {
                el.scrollTo({ top: targetTop, behavior: 'smooth' });
            } else {
                el.scrollTop = targetTop;
            }
        });
    }

    function isChatMessagesPinnedToBottom() {
        return isNearBottom(CHAT_SCROLL_FAB_HIDE_THRESHOLD_PX);
    }

    /** 已在底部时恢复 following（解决：手动滚到底但 scrollMode 仍为 detached） */
    function resumeFollowingIfAtBottom() {
        if (Date.now() < detachLockUntil) return false;
        if (!isNearBottom(CHAT_SCROLL_FOLLOW_THRESHOLD_PX)) return false;
        if (scrollMode === 'detached') setScrollFollowing();
        return true;
    }

    function captureScrollPinState() {
        if (Date.now() < detachLockUntil) return false;
        if (resumeFollowingIfAtBottom()) return true;
        return scrollMode === 'following';
    }

    function setScrollFollowing() {
        scrollMode = 'following';
        detachLockUntil = 0;
        hasPendingNewBelow = false;
        updateScrollFabs();
    }

    function markPendingNewBelow() {
        if (scrollMode !== 'detached') return;
        hasPendingNewBelow = true;
        updateScrollFabs();
    }

    function setScrollDetached() {
        scrollMode = 'detached';
        detachLockUntil = Date.now() + DETACH_LOCK_MS;
        cancelAnimationFrame(scrollFollowRaf);
        if (isStreamActive()) {
            hasPendingNewBelow = true;
        }
        updateScrollFabs();
    }

    function scrollChatToBottomInstant() {
        if (scrollMode !== 'following') return;
        const el = getChatMessagesEl();
        if (!el) return;
        programmaticScroll = true;
        scrollProcessDetailsTo('bottom', false);
        el.scrollTop = el.scrollHeight;
        lastScrollTop = el.scrollTop;
        requestAnimationFrame(function () {
            programmaticScroll = false;
        });
    }

    function scrollChatToBottomSmooth() {
        const el = getChatMessagesEl();
        if (!el) return;
        programmaticScroll = true;
        scrollProcessDetailsTo('bottom', true);
        el.scrollTo({ top: el.scrollHeight, behavior: 'smooth' });
        requestAnimationFrame(function () {
            programmaticScroll = false;
            const node = getChatMessagesEl();
            if (node) lastScrollTop = node.scrollTop;
        });
    }

    function scrollChatToTopSmooth() {
        const el = getChatMessagesEl();
        if (!el) return;
        scrollMode = 'detached';
        detachLockUntil = Date.now() + DETACH_LOCK_MS;
        cancelAnimationFrame(scrollFollowRaf);
        programmaticScroll = true;
        scrollProcessDetailsTo('top', true);
        el.scrollTo({ top: 0, behavior: 'smooth' });
        requestAnimationFrame(function () {
            programmaticScroll = false;
            const node = getChatMessagesEl();
            if (node) lastScrollTop = node.scrollTop;
        });
        updateScrollFabs();
    }

    function updateScrollFabs() {
        const bottomFab = document.getElementById('chat-scroll-to-bottom');
        const topFab = document.getElementById('chat-scroll-to-top');

        let label;
        if (hasPendingNewBelow) {
            label = typeof window.t === 'function'
                ? window.t('chat.scrollToBottomHasNew')
                : '↓ 有新内容';
        } else {
            label = typeof window.t === 'function'
                ? window.t('chat.scrollToBottom')
                : '回到底部';
        }
        if (bottomFab) {
            bottomFab.classList.add('visible');
            bottomFab.setAttribute('aria-label', label);
            bottomFab.setAttribute('title', label);
            bottomFab.textContent = label;
        }

        const topLabel = typeof window.t === 'function'
            ? window.t('chat.scrollToTop')
            : '回到顶部';
        if (topFab) {
            topFab.classList.add('visible');
            topFab.setAttribute('aria-label', topLabel);
            topFab.setAttribute('title', topLabel);
            topFab.textContent = topLabel;
        }
    }

    function canAutoScrollNow(wasPinnedBeforeDomUpdate) {
        if (Date.now() < detachLockUntil) return false;
        if (resumeFollowingIfAtBottom()) return true;
        if (scrollMode === 'detached') return false;
        if (wasPinnedBeforeDomUpdate === true) return true;
        return isNearBottom(CHAT_SCROLL_FOLLOW_THRESHOLD_PX);
    }

    function scheduleChatScrollToBottomIfFollowing(wasPinnedBeforeDomUpdate) {
        if (!canAutoScrollNow(wasPinnedBeforeDomUpdate)) {
            markPendingNewBelow();
            return;
        }
        cancelAnimationFrame(scrollFollowRaf);
        scrollFollowRaf = requestAnimationFrame(scrollChatToBottomInstant);
    }

    /** @param {boolean} wasPinned DOM 更新前是否应跟随（由 captureScrollPinState 传入） */
    function scrollChatMessagesToBottomIfPinned(wasPinned) {
        scheduleChatScrollToBottomIfFollowing(wasPinned);
    }

    function forceScrollChatToBottom(smooth) {
        setScrollFollowing();
        cancelAnimationFrame(scrollFollowRaf);
        if (smooth) {
            scrollChatToBottomSmooth();
        } else {
            scrollChatToBottomInstant();
        }
    }

    function onUserSendMessage() {
        setScrollFollowing();
        scrollChatToBottomInstant();
    }

    function clearAllStreamingMarkers() {
        document.querySelectorAll('.progress-container.is-streaming, .process-details-container.is-streaming').forEach(function (el) {
            el.classList.remove('is-streaming');
        });
    }

    function markProgressStreaming(active, progressId) {
        if (!active) {
            clearAllStreamingMarkers();
            return;
        }
        if (!progressId) return;
        const progressEl = document.getElementById(progressId);
        const container = progressEl && progressEl.querySelector('.progress-container');
        if (container) container.classList.add('is-streaming');
    }

    function markProcessDetailsStreaming(active, assistantDomId) {
        if (!active) {
            document.querySelectorAll('.process-details-container.is-streaming').forEach(function (el) {
                el.classList.remove('is-streaming');
            });
            return;
        }
        if (!assistantDomId) return;
        const container = document.getElementById('process-details-' + assistantDomId);
        if (!container) return;
        container.classList.add('is-streaming');
        const timeline = container.querySelector('.progress-timeline');
        if (timeline) timeline.classList.add('expanded');
    }

    function onStreamEnd() {
        clearAllStreamingMarkers();
        try {
            window.__csTaskEventStream = { active: false, conversationId: null, assistantDomId: null, progressId: null };
        } catch (e) { /* ignore */ }
        updateScrollFabs();
    }

    /** 刷新后会话 task-events 补流开始时，与 sendMessage 主流程对齐 */
    function onTaskEventStreamBegin(conversationId, assistantDomId, progressId) {
        try {
            window.__csTaskEventStream = {
                active: true,
                conversationId: conversationId || null,
                assistantDomId: assistantDomId || null,
                progressId: progressId || null
            };
        } catch (e) { /* ignore */ }
        markProcessDetailsStreaming(true, assistantDomId);
        resumeFollowingIfAtBottom();
        updateScrollFabs();
    }

    function onTaskEventStreamEnd() {
        onStreamEnd();
    }

    function applyMessageScrollOption(options) {
        const opt = (options && options.scroll) || 'follow';
        if (opt === 'none') return;
        if (opt === 'force') {
            forceScrollChatToBottom(false);
            return;
        }
        scheduleChatScrollToBottomIfFollowing(captureScrollPinState());
    }

    /** 流式/用户未跟随时禁止 scrollIntoView 抢滚动 */
    function scrollElementIntoViewIfFollowing(el, options) {
        if (!el || !captureScrollPinState()) return;
        el.scrollIntoView(options || { behavior: 'smooth', block: 'nearest' });
    }

    function onChatMessagesScroll() {
        const el = getChatMessagesEl();
        if (!el) return;

        if (programmaticScroll) {
            lastScrollTop = el.scrollTop;
            return;
        }

        const st = el.scrollTop;
        const scrolledUp = st < lastScrollTop - 1;

        if (scrolledUp) {
            setScrollDetached();
        } else if (resumeFollowingIfAtBottom()) {
            /* 拖滚动条/点击轨道跳到底部时也恢复跟随 */
        }

        lastScrollTop = st;
        updateScrollFabs();
    }

    function bindChatScrollListeners() {
        if (listenersBound) return;
        const el = getChatMessagesEl();
        if (!el) return;
        listenersBound = true;
        lastScrollTop = el.scrollTop;

        el.addEventListener('wheel', function (e) {
            if (e.deltaY < -1) setScrollDetached();
        }, { passive: true });

        el.addEventListener('touchmove', function (e) {
            if (e.touches && e.touches.length === 1) {
                el._csTouchLastY = el._csTouchLastY != null ? el._csTouchLastY : e.touches[0].clientY;
                if (e.touches[0].clientY > el._csTouchLastY + 4) {
                    setScrollDetached();
                }
                el._csTouchLastY = e.touches[0].clientY;
            }
        }, { passive: true });
        el.addEventListener('touchstart', function (e) {
            if (e.touches && e.touches.length) {
                el._csTouchLastY = e.touches[0].clientY;
            }
        }, { passive: true });
        el.addEventListener('touchend', function () {
            el._csTouchLastY = null;
        }, { passive: true });

        el.addEventListener('scroll', onChatMessagesScroll, { passive: true });

        const fab = document.getElementById('chat-scroll-to-bottom');
        if (fab) {
            fab.addEventListener('click', function () {
                forceScrollChatToBottom(true);
            });
        }
        const topFab = document.getElementById('chat-scroll-to-top');
        if (topFab) {
            topFab.addEventListener('click', function () {
                scrollChatToTopSmooth();
            });
        }
    }

    function initChatScroll() {
        bindChatScrollListeners();
        const el = getChatMessagesEl();
        if (el) lastScrollTop = el.scrollTop;
        updateScrollFabs();
    }

    window.CyberStrikeChatScroll = {
        init: initChatScroll,
        onUserSendMessage: onUserSendMessage,
        onStreamEnd: onStreamEnd,
        onTaskEventStreamBegin: onTaskEventStreamBegin,
        onTaskEventStreamEnd: onTaskEventStreamEnd,
        captureScrollPinState: captureScrollPinState,
        scheduleScroll: scheduleChatScrollToBottomIfFollowing,
        scrollIfPinned: scrollChatMessagesToBottomIfPinned,
        forceScrollToBottom: forceScrollChatToBottom,
        forceScrollToTop: scrollChatToTopSmooth,
        applyMessageScroll: applyMessageScrollOption,
        scrollIntoViewIfFollowing: scrollElementIntoViewIfFollowing,
        isPinnedToBottom: isChatMessagesPinnedToBottom,
        markProgressStreaming: markProgressStreaming,
        markProcessDetailsStreaming: markProcessDetailsStreaming,
        setScrollFollowing: setScrollFollowing,
        setScrollDetached: setScrollDetached,
    };

    window.isChatMessagesPinnedToBottom = isChatMessagesPinnedToBottom;
    window.captureScrollPinState = captureScrollPinState;
    window.scrollChatMessagesToBottomIfPinned = scrollChatMessagesToBottomIfPinned;

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', initChatScroll);
    } else {
        initChatScroll();
    }
})();
