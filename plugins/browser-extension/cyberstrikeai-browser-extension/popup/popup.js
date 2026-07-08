chrome.action.setBadgeText({ text: '' });

async function renderPopup() {
  const verEl = document.getElementById('version');
  if (verEl && chrome.runtime && chrome.runtime.getManifest) {
    verEl.textContent = 'v' + chrome.runtime.getManifest().version;
  }

  const statusEl = document.getElementById('conn-status');
  if (!statusEl || typeof loadConfig !== 'function') return;

  try {
    const cfg = await loadConfig();
    const endpoint = baseUrlFrom(cfg);
    if (cfg.token) {
      statusEl.className = 'conn-status conn-status--ok';
      statusEl.textContent = `已连接 ${endpoint}`;
    } else {
      statusEl.className = 'conn-status conn-status--idle';
      statusEl.textContent = `未验证 · 目标 ${endpoint}`;
    }
  } catch (_) {
    statusEl.className = 'conn-status conn-status--idle';
    statusEl.textContent = '无法读取配置';
  }
}

renderPopup();
