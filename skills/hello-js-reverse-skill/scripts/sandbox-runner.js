/**
 * VM 沙箱执行器
 * 用于在隔离的 Node.js 沙箱中执行从浏览器提取的 JS 代码
 * 
 * 使用方式:
 *   node sandbox-runner.js <js-file> [options]
 *   node sandbox-runner.js code.js --url=https://target.com --extract-cookie
 */

const vm = require('vm');
const fs = require('fs');
const path = require('path');

class SandboxRunner {
    constructor(options = {}) {
        this.options = {
            url: options.url || 'https://example.com',
            userAgent: options.userAgent || 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
            timeout: options.timeout || 10000,
            extractCookie: options.extractCookie || false,
            verbose: options.verbose || false,
            ...options,
        };
        
        this.cookies = {};
        this.logs = [];
        this.errors = [];
    }

    createSandbox() {
        const self = this;
        const urlObj = new URL(this.options.url);
        
        const sandbox = {
            window: null,
            self: null,
            globalThis: null,
            top: null,
            parent: null,
            frames: null,
            
            document: this._createDocument(),
            navigator: this._createNavigator(),
            location: this._createLocation(urlObj),
            screen: { width: 1920, height: 1080, availWidth: 1920, availHeight: 1055, colorDepth: 24, pixelDepth: 24 },
            history: { length: 1, pushState: () => {}, replaceState: () => {}, back: () => {}, forward: () => {} },
            
            setTimeout: (fn, ms) => setTimeout(fn, Math.min(ms || 0, 5000)),
            setInterval: (fn, ms) => setInterval(fn, Math.max(ms || 100, 100)),
            clearTimeout, clearInterval,
            requestAnimationFrame: (fn) => setTimeout(fn, 16),
            cancelAnimationFrame: clearTimeout,
            
            String, Array, Object, Math, Date, RegExp, JSON, Map, Set, WeakMap, WeakSet,
            Number, Boolean, Symbol, BigInt,
            parseInt, parseFloat, isNaN, isFinite, NaN, Infinity, undefined,
            encodeURIComponent, decodeURIComponent, encodeURI, decodeURI,
            escape, unescape,
            Error, TypeError, RangeError, SyntaxError, ReferenceError, URIError, EvalError,
            ArrayBuffer, Uint8Array, Uint16Array, Uint32Array, Int8Array, Int16Array, Int32Array,
            Float32Array, Float64Array, DataView,
            Promise, Proxy, Reflect,
            TextEncoder: typeof TextEncoder !== 'undefined' ? TextEncoder : class {},
            TextDecoder: typeof TextDecoder !== 'undefined' ? TextDecoder : class {},
            URL: typeof URL !== 'undefined' ? URL : class {},
            URLSearchParams: typeof URLSearchParams !== 'undefined' ? URLSearchParams : class {},
            
            btoa: (str) => Buffer.from(str, 'binary').toString('base64'),
            atob: (b64) => Buffer.from(b64, 'base64').toString('binary'),
            
            console: {
                log: (...args) => { self.logs.push(args.join(' ')); if (self.options.verbose) console.log('[Sandbox]', ...args); },
                warn: (...args) => { self.logs.push('[WARN] ' + args.join(' ')); },
                error: (...args) => { self.errors.push(args.join(' ')); if (self.options.verbose) console.error('[Sandbox Error]', ...args); },
                info: (...args) => { self.logs.push('[INFO] ' + args.join(' ')); },
                debug: () => {},
                trace: () => {},
                clear: () => {},
            },

            XMLHttpRequest: this._createXHR(),
            fetch: async () => new Response('{}'),
            
            Event: class Event { constructor(type) { this.type = type; } },
            CustomEvent: class CustomEvent { constructor(type, opts) { this.type = type; this.detail = opts?.detail; } },
            MutationObserver: class MutationObserver { observe() {} disconnect() {} },
            IntersectionObserver: class IntersectionObserver { observe() {} disconnect() {} },
            ResizeObserver: class ResizeObserver { observe() {} disconnect() {} },
        };
        
        sandbox.window = sandbox;
        sandbox.self = sandbox;
        sandbox.globalThis = sandbox;
        sandbox.top = sandbox;
        sandbox.parent = sandbox;
        sandbox.frames = sandbox;
        
        // Cookie 拦截
        if (this.options.extractCookie) {
            this._setupCookieTrap(sandbox);
        }
        
        return sandbox;
    }

    _createDocument() {
        const createElement = (tag) => ({
            tagName: tag.toUpperCase(),
            style: new Proxy({}, { get: () => '', set: () => true }),
            classList: { add: () => {}, remove: () => {}, contains: () => false },
            setAttribute: () => {},
            getAttribute: () => null,
            removeAttribute: () => {},
            appendChild: (child) => child,
            removeChild: () => {},
            insertBefore: () => {},
            addEventListener: () => {},
            removeEventListener: () => {},
            dispatchEvent: () => true,
            innerHTML: '', innerText: '', textContent: '',
            src: '', href: '', id: '', className: '',
            parentNode: null, parentElement: null,
            childNodes: [], children: [], firstChild: null, lastChild: null,
            nextSibling: null, previousSibling: null,
            offsetWidth: 100, offsetHeight: 100,
            getBoundingClientRect: () => ({ top: 0, left: 0, bottom: 100, right: 100, width: 100, height: 100 }),
            querySelector: () => null,
            querySelectorAll: () => [],
            getElementsByTagName: () => [],
            getElementsByClassName: () => [],
            cloneNode: () => createElement(tag),
        });

        return {
            cookie: '',
            createElement,
            createElementNS: (ns, tag) => createElement(tag),
            createTextNode: (text) => ({ nodeType: 3, textContent: text }),
            createDocumentFragment: () => ({ appendChild: () => {}, childNodes: [] }),
            getElementById: () => null,
            getElementsByTagName: (tag) => tag === 'script' ? [1, 2, 3] : [],
            getElementsByClassName: () => [],
            getElementsByName: () => [],
            querySelector: () => null,
            querySelectorAll: () => [],
            addEventListener: () => {},
            removeEventListener: () => {},
            head: { appendChild: () => {} },
            body: { appendChild: () => {}, innerHTML: '' },
            documentElement: { style: {} },
            readyState: 'complete',
            title: '',
            referrer: '',
            domain: '',
            URL: this.options.url,
            characterSet: 'UTF-8',
            contentType: 'text/html',
        };
    }

    _createNavigator() {
        return {
            userAgent: this.options.userAgent,
            appCodeName: 'Mozilla',
            appName: 'Netscape',
            appVersion: '5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36',
            platform: 'MacIntel',
            product: 'Gecko',
            productSub: '20030107',
            vendor: 'Google Inc.',
            vendorSub: '',
            language: 'zh-CN',
            languages: ['zh-CN', 'zh', 'en'],
            cookieEnabled: true,
            onLine: true,
            doNotTrack: null,
            hardwareConcurrency: 8,
            maxTouchPoints: 0,
            webdriver: false,
            plugins: { length: 3, item: () => null, namedItem: () => null, refresh: () => {} },
            mimeTypes: { length: 4, item: () => null, namedItem: () => null },
            permissions: { query: async () => ({ state: 'granted' }) },
            mediaDevices: { enumerateDevices: async () => [] },
            geolocation: { getCurrentPosition: () => {} },
            connection: { effectiveType: '4g', downlink: 10 },
            getBattery: async () => ({ charging: true, chargingTime: 0, dischargingTime: Infinity, level: 1 }),
        };
    }

    _createLocation(urlObj) {
        return {
            href: urlObj.href,
            protocol: urlObj.protocol,
            host: urlObj.host,
            hostname: urlObj.hostname,
            port: urlObj.port,
            pathname: urlObj.pathname,
            search: urlObj.search,
            hash: urlObj.hash,
            origin: urlObj.origin,
            assign: () => {},
            replace: () => {},
            reload: () => {},
            toString: () => urlObj.href,
        };
    }

    _createXHR() {
        return class XMLHttpRequest {
            constructor() {
                this.readyState = 0;
                this.status = 0;
                this.statusText = '';
                this.responseText = '';
                this.response = '';
                this.responseType = '';
                this.withCredentials = false;
            }
            open() { this.readyState = 1; }
            setRequestHeader() {}
            send() {
                this.readyState = 4;
                this.status = 200;
                this.responseText = '{}';
                this.response = '{}';
                if (this.onreadystatechange) this.onreadystatechange();
                if (this.onload) this.onload();
            }
            addEventListener(e, fn) { this['on' + e] = fn; }
            abort() {}
            getResponseHeader() { return null; }
            getAllResponseHeaders() { return ''; }
        };
    }

    _setupCookieTrap(sandbox) {
        const self = this;
        const originalCookie = sandbox.document.cookie || '';
        
        Object.defineProperty(sandbox.document, 'cookie', {
            get() {
                return Object.entries(self.cookies)
                    .map(([k, v]) => `${k}=${v}`)
                    .join('; ');
            },
            set(val) {
                const mainPart = val.split(';')[0];
                const eqIdx = mainPart.indexOf('=');
                if (eqIdx > 0) {
                    const name = mainPart.substring(0, eqIdx).trim();
                    const value = mainPart.substring(eqIdx + 1).trim();
                    self.cookies[name] = value;
                    if (self.options.verbose) {
                        console.log(`[Cookie Trap] ${name}=${value}`);
                    }
                }
            },
            configurable: true,
        });
        
        if (originalCookie) {
            originalCookie.split(';').forEach(pair => {
                const [k, ...rest] = pair.trim().split('=');
                if (k) self.cookies[k.trim()] = rest.join('=');
            });
        }
    }

    run(code, filename = 'sandbox.js') {
        const sandbox = this.createSandbox();
        vm.createContext(sandbox);
        
        try {
            const result = vm.runInContext(code, sandbox, {
                timeout: this.options.timeout,
                filename,
                displayErrors: true,
            });
            
            return {
                success: true,
                result,
                cookies: { ...this.cookies },
                logs: [...this.logs],
                errors: [...this.errors],
                sandbox,
            };
        } catch (e) {
            return {
                success: false,
                error: e.message,
                stack: e.stack,
                cookies: { ...this.cookies },
                logs: [...this.logs],
                errors: [...this.errors],
            };
        }
    }

    runFile(filePath) {
        const code = fs.readFileSync(filePath, 'utf-8');
        return this.run(code, path.basename(filePath));
    }
}

// CLI 入口
if (require.main === module) {
    const args = process.argv.slice(2);
    
    if (args.length === 0) {
        console.log('用法: node sandbox-runner.js <js-file> [options]');
        console.log('');
        console.log('选项:');
        console.log('  --url=<url>           设置 location.href');
        console.log('  --extract-cookie      启用 Cookie 拦截');
        console.log('  --verbose             输出详细日志');
        console.log('  --timeout=<ms>        执行超时（默认 10000）');
        process.exit(0);
    }
    
    const filePath = args[0];
    const options = {};
    
    args.slice(1).forEach(arg => {
        if (arg === '--extract-cookie') options.extractCookie = true;
        else if (arg === '--verbose') options.verbose = true;
        else if (arg.startsWith('--url=')) options.url = arg.substring(6);
        else if (arg.startsWith('--timeout=')) options.timeout = parseInt(arg.substring(10));
    });
    
    const runner = new SandboxRunner(options);
    const result = runner.runFile(filePath);
    
    console.log('\n========== 执行结果 ==========');
    console.log('状态:', result.success ? '成功' : '失败');
    
    if (!result.success) {
        console.log('错误:', result.error);
    }
    
    if (Object.keys(result.cookies).length > 0) {
        console.log('\nCookies:');
        Object.entries(result.cookies).forEach(([k, v]) => {
            console.log(`  ${k} = ${v}`);
        });
    }
    
    if (result.logs.length > 0) {
        console.log('\n日志:');
        result.logs.forEach(log => console.log(`  ${log}`));
    }
    
    if (result.errors.length > 0) {
        console.log('\n错误日志:');
        result.errors.forEach(err => console.log(`  ${err}`));
    }
}

module.exports = { SandboxRunner };
