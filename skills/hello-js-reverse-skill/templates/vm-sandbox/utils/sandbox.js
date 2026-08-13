/**
 * VM 沙箱模块
 * 用于执行服务端返回的混淆 JS 代码，提取动态生成的 Cookie/Token
 */

const vm = require('vm');

function createSandbox(options = {}) {
    const cookies = {};
    const logs = [];
    
    const sandbox = {
        window: null,
        self: null,
        globalThis: null,
        top: null,
        parent: null,
        
        document: {
            cookie: options.initialCookie || '',
            createElement: (tag) => ({
                tagName: tag.toUpperCase(), style: {}, src: '',
                setAttribute: () => {}, getAttribute: () => null,
                appendChild: () => {}, innerHTML: '',
            }),
            getElementById: () => null,
            getElementsByTagName: (tag) => tag === 'script' ? [1, 2, 3] : [],
            getElementsByClassName: () => [],
            querySelector: () => null,
            querySelectorAll: () => [],
            head: { appendChild: () => {} },
            body: { appendChild: () => {} },
            location: { href: options.url || 'https://example.com' },
            referrer: '',
            readyState: 'complete',
        },
        
        navigator: {
            userAgent: options.userAgent || 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
            appCodeName: 'Mozilla', appName: 'Netscape', appVersion: '5.0',
            platform: 'MacIntel', language: 'zh-CN', languages: ['zh-CN', 'zh', 'en'],
            cookieEnabled: true, onLine: true, webdriver: false,
            plugins: { length: 3 }, mimeTypes: { length: 4 },
            hardwareConcurrency: 8, maxTouchPoints: 0,
        },
        
        location: {
            href: options.url || 'https://example.com',
            protocol: 'https:', host: 'example.com',
            hostname: 'example.com', pathname: '/', search: '', hash: '',
            origin: 'https://example.com',
        },
        
        screen: { width: 1920, height: 1080, availWidth: 1920, availHeight: 1055, colorDepth: 24, pixelDepth: 24 },
        
        setTimeout: (fn, ms) => setTimeout(fn, Math.min(ms || 0, 3000)),
        setInterval: (fn, ms) => { /* 不执行定时器避免死循环 */ return -1; },
        clearTimeout, clearInterval,
        
        String, Array, Object, Math, Date, RegExp, JSON, Map, Set,
        parseInt, parseFloat, isNaN, isFinite, NaN, Infinity, undefined,
        encodeURIComponent, decodeURIComponent, encodeURI, decodeURI,
        escape, unescape,
        Error, TypeError, RangeError, SyntaxError, ReferenceError,
        ArrayBuffer, Uint8Array, Int32Array, Float64Array, DataView,
        Promise, Proxy, Reflect, Symbol, Number, Boolean,
        
        btoa: (str) => Buffer.from(str, 'binary').toString('base64'),
        atob: (b64) => Buffer.from(b64, 'base64').toString('binary'),
        
        console: {
            log: (...args) => logs.push(args.join(' ')),
            warn: () => {}, error: () => {}, info: () => {},
            debug: () => {}, trace: () => {}, clear: () => {},
        },
        
        XMLHttpRequest: class {
            open() {} setRequestHeader() {} send() {
                this.readyState = 4; this.status = 200; this.responseText = '{}';
                if (this.onreadystatechange) this.onreadystatechange();
            }
            addEventListener(e, fn) { this['on' + e] = fn; }
        },
    };
    
    sandbox.window = sandbox;
    sandbox.self = sandbox;
    sandbox.globalThis = sandbox;
    sandbox.top = sandbox;
    sandbox.parent = sandbox;
    sandbox.frames = sandbox;
    
    // Cookie 拦截
    Object.defineProperty(sandbox.document, 'cookie', {
        get() {
            return Object.entries(cookies).map(([k, v]) => `${k}=${v}`).join('; ');
        },
        set(val) {
            const mainPart = val.split(';')[0];
            const eqIdx = mainPart.indexOf('=');
            if (eqIdx > 0) {
                const name = mainPart.substring(0, eqIdx).trim();
                const value = mainPart.substring(eqIdx + 1).trim();
                cookies[name] = value;
            }
        },
        configurable: true,
    });
    
    if (options.initialCookie) {
        options.initialCookie.split(';').forEach(pair => {
            const [k, ...rest] = pair.trim().split('=');
            if (k) cookies[k.trim()] = rest.join('=');
        });
    }
    
    return { sandbox, cookies, logs };
}

/**
 * 执行混淆 JS 代码并提取 Cookie
 * @param {string} code - 混淆的 JS 代码
 * @param {Object} options - 配置选项
 * @returns {Object} { cookies, logs, success }
 */
function executeAndExtractCookie(code, options = {}) {
    const { sandbox, cookies, logs } = createSandbox(options);
    vm.createContext(sandbox);
    
    try {
        vm.runInContext(code, sandbox, {
            timeout: options.timeout || 5000,
            filename: 'dynamic.js',
        });
        
        return {
            success: true,
            cookies: { ...cookies },
            cookieString: Object.entries(cookies).map(([k, v]) => `${k}=${v}`).join('; '),
            logs,
        };
    } catch (error) {
        return {
            success: false,
            error: error.message,
            cookies: { ...cookies },
            logs,
        };
    }
}

module.exports = { createSandbox, executeAndExtractCookie };
