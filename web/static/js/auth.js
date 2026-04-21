const AUTH_STORAGE_KEY = 'cyberstrike-auth';
let authToken = null;
let authTokenExpiry = null;
let authPromise = null;
let authPromiseResolvers = [];
let isAppInitialized = false;

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
        console.warn('Cannot persist auth info:', error);
    }
}

function clearAuthStorage() {
    authToken = null;
    authTokenExpiry = null;
    try {
        localStorage.removeItem(AUTH_STORAGE_KEY);
    } catch (error) {
        console.warn('Cannot clear auth info:', error);
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
        console.error('Failed to read auth info:', error);
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

function handleUnauthorized({ message = null, silent = false } = {}) {
    clearAuthStorage();
    authPromise = null;
    authPromiseResolvers = [];
    let finalMessage = message;
    if (!finalMessage) {
        if (typeof window !== 'undefined' && typeof window.t === 'function') {
            finalMessage = window.t('auth.sessionExpired');
        } else {
            finalMessage = 'Auth expired, please log in again';
        }
    }
    if (!silent) {
        showLoginOverlay(finalMessage);
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
        const msg = (typeof window !== 'undefined' && typeof window.t === 'function')
            ? window.t('auth.unauthorized')
            : 'Unauthorized access';
        throw new Error(msg);
    }
    return response;
}

/**
 * multipart POST with XMLHttpRequest so upload progress is available (fetch cannotreliableonprogress).
 * returnand fetch 's object:ok,status,json(),text()
 */
async function apiUploadWithProgress(url, formData, options = {}) {
    await ensureAuthenticated();
    const onProgress = typeof options.onProgress === 'function' ? options.onProgress : null;
    return new Promise((resolve, reject) => {
        const xhr = new XMLHttpRequest();
        xhr.open('POST', url);
        if (authToken) {
            xhr.setRequestHeader('Authorization', `Bearer ${authToken}`);
        }
        xhr.upload.onprogress = (e) => {
            if (!onProgress || !e.lengthComputable) return;
            const percent = e.total > 0 ? Math.round((e.loaded / e.total) * 100) : 0;
            onProgress({ loaded: e.loaded, total: e.total, percent });
        };
        xhr.onerror = () => {
            reject(new Error('Network error'));
        };
        xhr.onload = () => {
            if (xhr.status === 401) {
                handleUnauthorized();
                const msg = (typeof window !== 'undefined' && typeof window.t === 'function')
                    ? window.t('auth.unauthorized')
                    : 'Unauthorized access';
                reject(new Error(msg));
                return;
            }
            const responseText = xhr.responseText || '';
            resolve({
                ok: xhr.status >= 200 && xhr.status < 300,
                status: xhr.status,
                text: async () => responseText,
                json: async () => {
                    try {
                        return responseText ? JSON.parse(responseText) : {};
                    } catch (err) {
                        throw err;
                    }
                },
            });
        };
        xhr.send(formData);
    });
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
            const msgEmpty = (typeof window !== 'undefined' && typeof window.t === 'function')
                ? window.t('auth.enterPassword')
                : 'Please enter password';
            errorBox.textContent = msgEmpty;
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
                const fallback = (typeof window !== 'undefined' && typeof window.t === 'function')
                    ? window.t('auth.loginFailedCheck')
                    : 'Login failed, please check password';
                errorBox.textContent = result.error || fallback;
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
        // After successful login, check model health and start periodic polling
        startModelHealthPolling();
    } catch (error) {
        console.error('Login failed:', error);
        if (errorBox) {
            const fallback = (typeof window !== 'undefined' && typeof window.t === 'function')
                ? window.t('auth.loginFailedRetry')
                : 'Login failed, please try again later';
            errorBox.textContent = fallback;
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
 // wait i18n loadcompleteafterthensystemreadymessage,avoidclearcacheafterlanguageShow English bubblestillisin
        try {
            if (window.i18nReady && typeof window.i18nReady.then === 'function') {
                await window.i18nReady;
            }
        } catch (e) {
            console.warn('wait i18n readyFailed,ContinueinitializeChat', e);
        }
        initializeChatUI();
        isAppInitialized = true;
    }
    await refreshAppData();
}

// commontoolfunction
function getStatusText(status) {
    if (typeof window.t !== 'function') {
        const fallback = { pending: 'waitin', running: 'executing', completed: 'completed', failed: 'Failed' };
        return fallback[status] || status;
    }
    const keyMap = { pending: 'mcpDetailModal.statusPending', running: 'mcpDetailModal.statusRunning', completed: 'mcpDetailModal.statusCompleted', failed: 'mcpDetailModal.statusFailed' };
    const key = keyMap[status];
    return key ? window.t(key) : status;
}

function formatDuration(ms) {
    const seconds = Math.floor(ms / 1000);
    const minutes = Math.floor(seconds / 60);
    const hours = Math.floor(minutes / 60);
    
    if (hours > 0) {
 return `${hours}smallwhen${minutes % 60}minute`;
    } else if (minutes > 0) {
 return `${minutes}minute${seconds % 60}second`;
    } else {
        return `${seconds}second`;
    }
}

function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

function formatMarkdown(text) {
    const sanitizeConfig = {
        ALLOWED_TAGS: ['p', 'br', 'strong', 'em', 'u', 's', 'code', 'pre', 'blockquote', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'ul', 'ol', 'li', 'a', 'img', 'table', 'thead', 'tbody', 'tr', 'th', 'td', 'hr'],
        ALLOWED_ATTR: ['href', 'title', 'alt', 'src', 'class'],
        ALLOW_DATA_ATTR: false,
    };
    
    if (typeof DOMPurify !== 'undefined') {
        if (typeof marked !== 'undefined' && !/<[a-z][\s\S]*>/i.test(text)) {
            try {
                marked.setOptions({
                    breaks: true,
                    gfm: true,
                });
                let parsedContent = marked.parse(text);
                return DOMPurify.sanitize(parsedContent, sanitizeConfig);
            } catch (e) {
                console.error('Markdown Parse failed:', e);
                return DOMPurify.sanitize(text, sanitizeConfig);
            }
        } else {
            return DOMPurify.sanitize(text, sanitizeConfig);
        }
    } else if (typeof marked !== 'undefined') {
        try {
            marked.setOptions({
                breaks: true,
                gfm: true,
            });
            return marked.parse(text);
        } catch (e) {
            console.error('Markdown Parse failed:', e);
            return escapeHtml(text).replace(/\n/g, '<br>');
        }
    } else {
        return escapeHtml(text).replace(/\n/g, '<br>');
    }
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
                startModelHealthPolling();
                return;
            }
        } catch (error) {
            console.warn('Local session expired, need to re-login');
        }
    }

    clearAuthStorage();
    showLoginOverlay();
}

// ---- Model endpoint health check ----

let modelHealthInterval = null;

async function checkModelHealth() {
    try {
        const resp = await apiFetch('/api/health/model');
        const data = await resp.json();
        updateModelStatusIndicator(data);

        if (data.status === 'unconfigured') {
            showModelConfigBanner('API key not configured. The AI agent cannot function without it.', 'warning');
        } else if (data.status === 'error') {
            showModelConfigBanner('Model endpoint error: ' + (data.message || 'unknown'), 'error');
        } else {
            hideModelConfigBanner();
        }
    } catch (e) {
        // Server itself is down or network issue - do not show banner
    }
}

function showModelConfigBanner(message, type) {
    var banner = document.getElementById('model-config-banner');
    if (!banner) {
        banner = document.createElement('div');
        banner.id = 'model-config-banner';
        banner.style.cssText = 'position:fixed;top:0;left:0;right:0;z-index:10000;padding:10px 20px;text-align:center;font-size:13px;display:flex;align-items:center;justify-content:center;gap:12px;';
        document.body.appendChild(banner);
    }

    var isWarning = type === 'warning';
    banner.style.background = isWarning ? '#f59e0b' : '#ef4444';
    banner.style.color = '#fff';
    banner.innerHTML = '<span>' + message + '</span>' +
        '<button onclick="switchPage(\'settings\');hideModelConfigBanner();" style="background:rgba(255,255,255,0.2);border:1px solid rgba(255,255,255,0.4);color:#fff;padding:4px 12px;border-radius:4px;cursor:pointer;font-size:12px;">Go to Settings</button>' +
        '<button onclick="hideModelConfigBanner()" style="background:none;border:none;color:rgba(255,255,255,0.7);cursor:pointer;font-size:16px;">\u00d7</button>';
    banner.style.display = 'flex';
}

function hideModelConfigBanner() {
    var banner = document.getElementById('model-config-banner');
    if (banner) banner.style.display = 'none';
}

function updateModelStatusIndicator(data) {
    var indicator = document.getElementById('model-status-dot');
    if (!indicator) {
        var header = document.querySelector('.header-right') || document.querySelector('.header');
        if (!header) return;
        indicator = document.createElement('span');
        indicator.id = 'model-status-dot';
        indicator.style.cssText = 'width:8px;height:8px;border-radius:50%;display:inline-block;margin-right:8px;cursor:help;';
        header.prepend(indicator);
    }

    if (data.status === 'ok') {
        indicator.style.background = '#22c55e';
        indicator.title = 'Model: ' + (data.model || 'connected') + ' (' + (data.latency_ms || '?') + 'ms)';
    } else if (data.status === 'unconfigured') {
        indicator.style.background = '#f59e0b';
        indicator.title = 'API key not configured';
    } else {
        indicator.style.background = '#ef4444';
        indicator.title = 'Model error: ' + (data.message || 'unknown');
    }
}

function startModelHealthPolling() {
    if (modelHealthInterval) clearInterval(modelHealthInterval);
    checkModelHealth();
    modelHealthInterval = setInterval(checkModelHealth, 60000);
}

window.checkModelHealth = checkModelHealth;
window.hideModelConfigBanner = hideModelConfigBanner;

// usemenucontrol
function toggleUserMenu() {
    const dropdown = document.getElementById('user-menu-dropdown');
    if (!dropdown) return;
    
    const isVisible = dropdown.style.display !== 'none';
    dropdown.style.display = isVisible ? 'none' : 'block';
}

// clickpageelsewherewhenClosedropdown menu
document.addEventListener('click', function(event) {
    const dropdown = document.getElementById('user-menu-dropdown');
    const avatarBtn = document.querySelector('.user-avatar-btn');
    
    if (dropdown && avatarBtn && 
        !dropdown.contains(event.target) && 
        !avatarBtn.contains(event.target)) {
        dropdown.style.display = 'none';
    }
});

// logoutlogin
async function logout() {
    // Closedropdown menu
    const dropdown = document.getElementById('user-menu-dropdown');
    if (dropdown) {
        dropdown.style.display = 'none';
    }
    
    try {
        // firsttrycalllogoutAPI(iftokenvalid)
        if (authToken) {
            const headers = new Headers();
            headers.set('Authorization', `Bearer ${authToken}`);
            await fetch('/api/auth/logout', {
                method: 'POST',
                headers: headers,
            }).catch(() => {
                // ignore error,ContinueclearlocalauthenticationInfo
            });
        }
    } catch (error) {
        console.error('logoutloginAPIcallFailed:', error);
    } finally {
 // nohowall clearlocalauthenticationInfo
        clearAuthStorage();
        hideLoginOverlay();
        showLoginOverlay(typeof window.t === 'function' ? window.t('auth.loggedOut') : 'Logged out');
    }
}

// exportfunctionforHTMLuse
window.toggleUserMenu = toggleUserMenu;
window.logout = logout;

document.addEventListener('DOMContentLoaded', initializeApp);
