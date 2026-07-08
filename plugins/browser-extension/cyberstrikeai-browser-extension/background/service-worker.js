/* global chrome, CSAI_LIMITS, shouldCaptureEntry */

importScripts('../lib/constants.js', '../lib/capture.js');

/** Background hub: per-tab capture queue + panel subscriptions. */

const capturesByTab = new Map();
const portsByTab = new Map();
const filterApiByTab = new Map();
let captureEnabled = true;

function getCaptures(tabId) {
  if (!capturesByTab.has(tabId)) capturesByTab.set(tabId, []);
  return capturesByTab.get(tabId);
}

function trimCaptures(list) {
  if (list.length > CSAI_LIMITS.MAX_CAPTURED) {
    list.length = CSAI_LIMITS.MAX_CAPTURED;
  }
}

function broadcastTab(tabId, message) {
  const ports = portsByTab.get(tabId);
  if (!ports) return;
  for (const port of ports) {
    try {
      port.postMessage(message);
    } catch (_) {
      ports.delete(port);
    }
  }
}

function cleanupOldTabs(activeTabId) {
  if (capturesByTab.size <= CSAI_LIMITS.MAX_TAB_CAPTURES) return;
  for (const tabId of capturesByTab.keys()) {
    if (tabId !== activeTabId) {
      capturesByTab.delete(tabId);
      portsByTab.delete(tabId);
      filterApiByTab.delete(tabId);
    }
    if (capturesByTab.size <= CSAI_LIMITS.MAX_TAB_CAPTURES) break;
  }
}

chrome.runtime.onConnect.addListener((port) => {
  if (port.name !== 'cyberstrike-panel') return;

  port.onDisconnect.addListener(() => {
    for (const [tabId, set] of portsByTab.entries()) {
      set.delete(port);
      if (set.size === 0) portsByTab.delete(tabId);
    }
  });

  port.onMessage.addListener((msg) => {
    if (!msg || msg.type !== 'subscribe') return;
    const tabId = msg.tabId;
    if (tabId == null) return;

    if (!portsByTab.has(tabId)) portsByTab.set(tabId, new Set());
    portsByTab.get(tabId).add(port);

    if (typeof msg.filterApiOnly === 'boolean') {
      filterApiByTab.set(tabId, msg.filterApiOnly);
    }

    port.postMessage({ type: 'list', entries: getCaptures(tabId) });
    cleanupOldTabs(tabId);
  });
});

chrome.runtime.onMessage.addListener((msg, _sender, sendResponse) => {
  if (!msg || !msg.type) return false;

  if (msg.type === 'capture-entry') {
    const tabId = msg.tabId;
    if (tabId == null) return false;
    if (!captureEnabled) {
      sendResponse({ ok: true, skipped: true });
      return false;
    }
    const filterApi = filterApiByTab.has(tabId) ? filterApiByTab.get(tabId) : true;
    const entry = msg.entry;
    if (!shouldCaptureEntry(entry, filterApi)) {
      sendResponse({ ok: true, skipped: true });
      return false;
    }
    const list = getCaptures(tabId);
    list.unshift(entry);
    trimCaptures(list);
    broadcastTab(tabId, { type: 'entry', entry });
    sendResponse({ ok: true });
    return false;
  }

  if (msg.type === 'clear-captures') {
    const tabId = msg.tabId;
    if (tabId != null) {
      capturesByTab.set(tabId, []);
      broadcastTab(tabId, { type: 'list', entries: [] });
    }
    sendResponse({ ok: true });
    return false;
  }

  if (msg.type === 'set-filter-api') {
    const tabId = msg.tabId;
    if (tabId != null && typeof msg.filterApiOnly === 'boolean') {
      filterApiByTab.set(tabId, msg.filterApiOnly);
    }
    sendResponse({ ok: true });
    return false;
  }

  if (msg.type === 'set-capture-enabled') {
    if (typeof msg.enabled === 'boolean') {
      captureEnabled = msg.enabled;
    }
    sendResponse({ ok: true });
    return false;
  }

  if (msg.type === 'get-latest-api') {
    const tabId = msg.tabId;
    const list = getCaptures(tabId);
    const latest = list.find((e) => {
      const rt = (e.resourceType || '').toLowerCase();
      return rt === 'xhr' || rt === 'fetch';
    });
    sendResponse({ entry: latest || null });
    return false;
  }

  return false;
});

chrome.runtime.onInstalled.addListener(() => {
  if (chrome.contextMenus && chrome.contextMenus.removeAll) {
    chrome.contextMenus.removeAll();
  }
});
