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
    if (!cfg.token) {
      statusEl.className = 'conn-status conn-status--idle';
      statusEl.textContent = `Not validated · ${endpoint}`;
      return;
    }
    if (isTokenExpiredByTime(cfg.tokenExpiresAt)) {
      statusEl.className = 'conn-status conn-status--idle';
      statusEl.textContent = `Session expired · ${endpoint}`;
      return;
    }

    try {
      await validateTokenSession(endpoint, cfg.token);
      statusEl.className = 'conn-status conn-status--ok';
      statusEl.textContent = `${formatTokenExpiryHint(cfg.tokenExpiresAt)} · ${endpoint}`;
    } catch (err) {
      if (isAuthHttpStatus(err.status)) {
        statusEl.className = 'conn-status conn-status--idle';
        statusEl.textContent = `Server restarted or token invalid · ${endpoint}`;
      } else if (isNetworkFetchError(err)) {
        statusEl.className = 'conn-status conn-status--idle';
        statusEl.textContent = `Cannot reach server · ${endpoint}`;
      } else {
        statusEl.className = 'conn-status conn-status--idle';
        statusEl.textContent = `Validation failed · ${endpoint}`;
      }
    }
  } catch (_) {
    statusEl.className = 'conn-status conn-status--idle';
    statusEl.textContent = 'Cannot read config';
  }
}

renderPopup().catch(() => {});
