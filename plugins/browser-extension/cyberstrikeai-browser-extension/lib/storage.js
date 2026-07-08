/* global chrome */

const STORAGE_KEYS = {
  host: 'csai_host',
  port: 'csai_port',
  https: 'csai_https',
  lastProjectId: 'csai_last_project',
  lastRole: 'csai_last_role',
  lastAgentMode: 'csai_last_agent_mode',
  lastInstruction: 'csai_last_instruction',
  filterApiOnly: 'csai_filter_api_only',
  captureEnabled: 'csai_capture_enabled',
  renderMarkdown: 'csai_render_markdown',
  showDebugEvents: 'csai_show_debug',
};

const SESSION_TOKEN_KEY = 'csai_token';

const DEFAULTS = {
  host: '127.0.0.1',
  port: '8080',
  https: true,
  lastInstruction: CSAI_DEFAULT_INSTRUCTION,
  filterApiOnly: true,
  captureEnabled: true,
  renderMarkdown: true,
  showDebugEvents: false,
};

function localGet(keys) {
  return new Promise((resolve) => chrome.storage.local.get(keys, resolve));
}

function localSet(obj) {
  return new Promise((resolve) => chrome.storage.local.set(obj, resolve));
}

function sessionGet(keys) {
  return new Promise((resolve) => {
    if (!chrome.storage.session) {
      chrome.storage.local.get(keys, resolve);
      return;
    }
    chrome.storage.session.get(keys, resolve);
  });
}

function sessionSet(obj) {
  return new Promise((resolve) => {
    if (!chrome.storage.session) {
      chrome.storage.local.set(obj, resolve);
      return;
    }
    chrome.storage.session.set(obj, resolve);
  });
}

async function loadConfig() {
  const data = await localGet(Object.values(STORAGE_KEYS));
  const sess = await sessionGet([SESSION_TOKEN_KEY]);
  return {
    host: data[STORAGE_KEYS.host] || DEFAULTS.host,
    port: data[STORAGE_KEYS.port] || DEFAULTS.port,
    https: data[STORAGE_KEYS.https] !== false,
    token: sess[SESSION_TOKEN_KEY] || '',
    lastProjectId: data[STORAGE_KEYS.lastProjectId] || '',
    lastRole: data[STORAGE_KEYS.lastRole] || '',
    lastAgentMode: data[STORAGE_KEYS.lastAgentMode] || 'eino_single',
    lastInstruction: data[STORAGE_KEYS.lastInstruction] || DEFAULTS.lastInstruction,
    filterApiOnly: data[STORAGE_KEYS.filterApiOnly] !== false,
    captureEnabled: data[STORAGE_KEYS.captureEnabled] !== false,
    renderMarkdown: data[STORAGE_KEYS.renderMarkdown] !== false,
    showDebugEvents: data[STORAGE_KEYS.showDebugEvents] === true,
  };
}

async function saveConfig(partial) {
  const localMap = {};
  if (partial.host != null) localMap[STORAGE_KEYS.host] = partial.host;
  if (partial.port != null) localMap[STORAGE_KEYS.port] = partial.port;
  if (partial.https != null) localMap[STORAGE_KEYS.https] = partial.https;
  if (partial.lastProjectId != null) localMap[STORAGE_KEYS.lastProjectId] = partial.lastProjectId;
  if (partial.lastRole != null) localMap[STORAGE_KEYS.lastRole] = partial.lastRole;
  if (partial.lastAgentMode != null) localMap[STORAGE_KEYS.lastAgentMode] = partial.lastAgentMode;
  if (partial.lastInstruction != null) localMap[STORAGE_KEYS.lastInstruction] = partial.lastInstruction;
  if (partial.filterApiOnly != null) localMap[STORAGE_KEYS.filterApiOnly] = partial.filterApiOnly;
  if (partial.captureEnabled != null) localMap[STORAGE_KEYS.captureEnabled] = partial.captureEnabled;
  if (partial.renderMarkdown != null) localMap[STORAGE_KEYS.renderMarkdown] = partial.renderMarkdown;
  if (partial.showDebugEvents != null) localMap[STORAGE_KEYS.showDebugEvents] = partial.showDebugEvents;
  if (Object.keys(localMap).length) await localSet(localMap);
  if (partial.token != null) await sessionSet({ [SESSION_TOKEN_KEY]: partial.token });
}

function baseUrlFrom(cfg) {
  const scheme = cfg.https ? 'https' : 'http';
  return `${scheme}://${cfg.host}:${cfg.port}`;
}

/** Request optional host permission for the configured CyberStrikeAI origin. */
async function ensureHostPermission(baseUrl) {
  if (!chrome.permissions || !chrome.permissions.request) return;
  let origin;
  try {
    origin = new URL(baseUrl).origin + '/*';
  } catch (_) {
    throw new Error('Invalid Host/Port');
  }
  const has = await chrome.permissions.contains({ origins: [origin] });
  if (has) return;
  const granted = await chrome.permissions.request({ origins: [origin] });
  if (!granted) {
    throw new Error('需要授权访问 CyberStrikeAI 服务器地址');
  }
}
