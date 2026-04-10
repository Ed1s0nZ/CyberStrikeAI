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
        console.warn('无法持久化认证信息:', error);
    }
}

function clearAuthStorage() {
    authToken = null;
    authTokenExpiry = null;
    try {
        localStorage.removeItem(AUTH_STORAGE_KEY);
    } catch (error) {
        console.warn('无法清除认证信息:', error);
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
        console.error('读取认证信息失败:', error);
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
            finalMessage = '认证已过期，请重新登录';
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
            : '未授权访问';
        throw new Error(msg);
    }
    return response;
}

/**
 * multipart POST with XMLHttpRequest so upload progress is available (fetch 无法可靠上报进度).
 * 返回与 fetch 类似的对象：ok、status、json()、text()
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
                    : '未授权访问';
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
                : '请输入密码';
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
                    : '登录失败，请检查密码';
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
    } catch (error) {
        console.error('登录失败:', error);
        if (errorBox) {
            const fallback = (typeof window !== 'undefined' && typeof window.t === 'function')
                ? window.t('auth.loginFailedRetry')
                : '登录失败，请稍后重试';
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
        // 等待 i18n 首包加载完成后再插系统就绪消息，避免清除缓存后语言显示 English 气泡仍是中文
        try {
            if (window.i18nReady && typeof window.i18nReady.then === 'function') {
                await window.i18nReady;
            }
        } catch (e) {
            console.warn('等待 i18n 就绪失败，继续初始化聊天', e);
        }
        initializeChatUI();
        isAppInitialized = true;
    }
    await refreshAppData();
}

// 通用工具函数
function getStatusText(status) {
    if (typeof window.t !== 'function') {
        const fallback = { pending: '等待中', running: '执行中', completed: '已完成', failed: '失败' };
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
                console.error('Markdown 解析失败:', e);
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
            console.error('Markdown 解析失败:', e);
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
                return;
            }
        } catch (error) {
            console.warn('本地会话已失效，需重新登录');
        }
    }

    clearAuthStorage();
    showLoginOverlay();
}

// 用户菜单控制
function toggleUserMenu() {
    const dropdown = document.getElementById('user-menu-dropdown');
    if (!dropdown) return;
    
    const isVisible = dropdown.style.display !== 'none';
    dropdown.style.display = isVisible ? 'none' : 'block';
}

// 点击页面其他地方时关闭下拉菜单
document.addEventListener('click', function(event) {
    const dropdown = document.getElementById('user-menu-dropdown');
    const avatarBtn = document.querySelector('.user-avatar-btn');
    
    if (dropdown && avatarBtn && 
        !dropdown.contains(event.target) && 
        !avatarBtn.contains(event.target)) {
        dropdown.style.display = 'none';
    }
});

// 退出登录
async function logout() {
    // 关闭下拉菜单
    const dropdown = document.getElementById('user-menu-dropdown');
    if (dropdown) {
        dropdown.style.display = 'none';
    }
    
    try {
        // 先尝试调用退出API（如果token有效）
        if (authToken) {
            const headers = new Headers();
            headers.set('Authorization', `Bearer ${authToken}`);
            await fetch('/api/auth/logout', {
                method: 'POST',
                headers: headers,
            }).catch(() => {
                // 忽略错误，继续清除本地认证信息
            });
        }
    } catch (error) {
        console.error('退出登录API调用失败:', error);
    } finally {
        // 无论如何都清除本地认证信息
        clearAuthStorage();
        hideLoginOverlay();
        showLoginOverlay(typeof window.t === 'function' ? window.t('auth.loggedOut') : '已退出登录');
    }
}

// 导出函数供HTML使用
window.toggleUserMenu = toggleUserMenu;
window.logout = logout;

document.addEventListener('DOMContentLoaded', initializeApp);
