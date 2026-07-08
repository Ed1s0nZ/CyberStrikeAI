/* global chrome, summarizeHarEntry, inferResourceType, mightCaptureRequest */

const FILTER_STORAGE_KEY = 'csai_filter_api_only';
const CAPTURE_ENABLED_KEY = 'csai_capture_enabled';

/** In-memory flags — avoids storage read on every network request. */
let filterApiOnly = true;
let captureEnabled = true;

function syncFromStorage(data) {
  filterApiOnly = (data && data[FILTER_STORAGE_KEY]) !== false;
  captureEnabled = (data && data[CAPTURE_ENABLED_KEY]) !== false;
}

chrome.storage.local.get([FILTER_STORAGE_KEY, CAPTURE_ENABLED_KEY], (data) => {
  syncFromStorage(data);
});

chrome.storage.onChanged.addListener((changes, area) => {
  if (area !== 'local') return;
  if (changes[FILTER_STORAGE_KEY]) {
    filterApiOnly = changes[FILTER_STORAGE_KEY].newValue !== false;
  }
  if (changes[CAPTURE_ENABLED_KEY]) {
    captureEnabled = changes[CAPTURE_ENABLED_KEY].newValue !== false;
  }
});

chrome.runtime.onMessage.addListener((msg) => {
  if (!msg || !msg.type) return;
  if (msg.type === 'set-filter-api' && typeof msg.filterApiOnly === 'boolean') {
    filterApiOnly = msg.filterApiOnly;
  }
  if (msg.type === 'set-capture-enabled' && typeof msg.enabled === 'boolean') {
    captureEnabled = msg.enabled;
  }
});

chrome.devtools.network.onRequestFinished.addListener((request) => {
  if (!captureEnabled) return;

  const tabId = chrome.devtools.inspectedWindow.tabId;
  const req = request.request || {};
  const res = request.response || {};
  const url = req.url || '';
  const resourceType =
    (request._resourceType || request.resourceType || inferResourceType(url, res.headers) || 'other')
      .toLowerCase();

  if (!mightCaptureRequest(url, resourceType, filterApiOnly)) {
    return;
  }

  request.getContent((body) => {
    const entry = summarizeHarEntry(request, body, resourceType);
    chrome.runtime.sendMessage({
      type: 'capture-entry',
      tabId,
      entry,
    });
  });
});

chrome.devtools.panels.create(
  'CyberStrikeAI',
  'icons/icon48.png',
  'panel/panel.html',
  () => {}
);
