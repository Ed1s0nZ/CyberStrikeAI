# 环境补全指南

## 概述

将浏览器端 JS 代码移植到 Node.js 执行时，需要模拟浏览器环境。本指南提供各种场景下的
环境补全方案，从最小补全到完整模拟。

## 原则

1. **最小补全优先**：只补目标代码实际访问的 API，不要一次性补全所有浏览器 API
2. **按需扩展**：运行报错时再补对应的属性/方法
3. **真实值优先**：尽量使用真实浏览器采集的值，而非随意伪造

## 1. Node.js vm 沙箱最小环境

### 基础模板

```javascript
const vm = require('vm');

function createMinimalSandbox(options = {}) {
    const sandbox = {
        // 全局对象
        window: null,
        self: null,
        globalThis: null,
        
        // DOM 最小模拟
        document: {
            cookie: options.cookie || '',
            createElement: (tag) => ({
                tagName: tag.toUpperCase(),
                style: {},
                setAttribute: () => {},
                getAttribute: () => null,
                appendChild: () => {},
                innerHTML: '',
                src: '',
            }),
            getElementById: () => null,
            getElementsByTagName: () => [],
            getElementsByClassName: () => [],
            querySelector: () => null,
            querySelectorAll: () => [],
            head: { appendChild: () => {} },
            body: { appendChild: () => {} },
            location: { href: options.url || 'https://example.com', hostname: 'example.com' },
            referrer: options.referrer || '',
            title: '',
            readyState: 'complete',
        },
        
        // Navigator
        navigator: {
            userAgent: options.userAgent || 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
            appCodeName: 'Mozilla',
            appName: 'Netscape',
            appVersion: '5.0',
            platform: 'MacIntel',
            language: 'zh-CN',
            languages: ['zh-CN', 'zh', 'en'],
            cookieEnabled: true,
            onLine: true,
            plugins: { length: 3 },
            mimeTypes: { length: 4 },
            webdriver: false,
            hardwareConcurrency: 8,
            maxTouchPoints: 0,
        },
        
        // Location
        location: {
            href: options.url || 'https://example.com',
            protocol: 'https:',
            host: 'example.com',
            hostname: 'example.com',
            port: '',
            pathname: '/',
            search: '',
            hash: '',
            origin: 'https://example.com',
        },
        
        // Screen
        screen: {
            width: 1920,
            height: 1080,
            availWidth: 1920,
            availHeight: 1055,
            colorDepth: 24,
            pixelDepth: 24,
        },
        
        // 定时器
        setTimeout: setTimeout,
        setInterval: setInterval,
        clearTimeout: clearTimeout,
        clearInterval: clearInterval,
        
        // 内置对象
        String, Array, Object, Math, Date, RegExp, JSON, Map, Set, WeakMap, WeakSet,
        parseInt, parseFloat, isNaN, isFinite, NaN, Infinity, undefined,
        encodeURIComponent, decodeURIComponent,
        encodeURI, decodeURI,
        escape, unescape,
        Error, TypeError, RangeError, SyntaxError, ReferenceError,
        ArrayBuffer, Uint8Array, Int32Array, Float64Array, DataView,
        Promise,
        Proxy, Reflect,
        Symbol,
        
        // Base64
        btoa: (str) => Buffer.from(str, 'binary').toString('base64'),
        atob: (b64) => Buffer.from(b64, 'base64').toString('binary'),
        
        // Console（用于调试）
        console: console,
    };
    
    // 循环引用
    sandbox.window = sandbox;
    sandbox.self = sandbox;
    sandbox.globalThis = sandbox;
    sandbox.top = sandbox;
    sandbox.parent = sandbox;
    sandbox.frames = sandbox;
    
    return sandbox;
}

// 使用示例
function runInSandbox(code, options = {}) {
    const sandbox = createMinimalSandbox(options);
    vm.createContext(sandbox);
    
    try {
        vm.runInContext(code, sandbox, {
            timeout: options.timeout || 5000,
            filename: options.filename || 'sandbox.js',
        });
    } catch (e) {
        console.error('沙箱执行错误:', e.message);
        throw e;
    }
    
    return sandbox;
}
```

### Cookie 拦截模式

```javascript
function createCookieTrapSandbox(options = {}) {
    const sandbox = createMinimalSandbox(options);
    const cookies = {};
    
    Object.defineProperty(sandbox.document, 'cookie', {
        get() {
            return Object.entries(cookies).map(([k, v]) => `${k}=${v}`).join('; ');
        },
        set(val) {
            const parts = val.split(';')[0].split('=');
            const name = parts[0].trim();
            const value = parts.slice(1).join('=').trim();
            cookies[name] = value;
            console.log(`[Sandbox] Cookie Set: ${name}=${value}`);
        }
    });
    
    sandbox._cookies = cookies;
    return sandbox;
}
```

## 2. WASM 环境补全

### 基础 WASM 加载

```javascript
const fs = require('fs');

async function loadWasm(wasmPath, importObject = {}) {
    const wasmBuffer = fs.readFileSync(wasmPath);
    
    const defaultImports = {
        env: {
            memory: new WebAssembly.Memory({ initial: 256 }),
            table: new WebAssembly.Table({ initial: 0, element: 'anyfunc' }),
            abort: () => { throw new Error('WASM abort'); },
            ...importObject.env,
        },
        wasi_snapshot_preview1: {
            fd_write: () => 0,
            fd_close: () => 0,
            fd_seek: () => 0,
            proc_exit: () => {},
            ...importObject.wasi_snapshot_preview1,
        },
        ...importObject,
    };
    
    const result = await WebAssembly.instantiate(wasmBuffer, defaultImports);
    return result.instance;
}
```

### wasm-bindgen (Rust) 环境补全

```javascript
class Window {
    constructor() {
        this.document = {
            body: {},
            createElement: () => ({}),
            getElementById: () => null,
        };
    }
}

function patchWasmBindgenEnv() {
    const win = new Window();
    win.window = win;
    win.self = win;
    
    globalThis.Window = Window;
    globalThis.window = win;
    globalThis.self = win;
    globalThis.document = win.document;
    
    // wasm-bindgen 可能检查 instanceof
    // 确保 win instanceof Window === true
}
```

### Emscripten 环境补全

```javascript
const Module = {
    print: console.log,
    printErr: console.error,
    TOTAL_MEMORY: 16777216,
    noInitialRun: true,
    onRuntimeInitialized: function() {
        console.log('WASM Runtime Ready');
    }
};
```

## 3. jQuery 模拟

```javascript
function createjQueryStub() {
    const $ = function(selector) {
        return {
            length: 1,
            val: () => '',
            text: () => '',
            html: () => '',
            attr: () => '',
            css: () => ({}),
            find: () => $(''),
            each: (fn) => { fn(0, {}); },
            click: () => {},
            on: () => {},
            ajax: $.ajax,
        };
    };
    
    $.ajax = function(options) {
        console.log('[jQuery] $.ajax:', options.url, options.data);
        return { done: (fn) => ({ fail: () => ({}) }) };
    };
    
    $.get = $.post = $.ajax;
    $.fn = $.prototype = {};
    $.extend = Object.assign;
    
    return $;
}
```

## 4. XMLHttpRequest 模拟

```javascript
function createXHRStub(interceptor) {
    return class XMLHttpRequest {
        constructor() {
            this.readyState = 0;
            this.status = 0;
            this.responseText = '';
            this.response = '';
        }
        
        open(method, url) {
            this._method = method;
            this._url = url;
            this.readyState = 1;
        }
        
        setRequestHeader(name, value) {
            this._headers = this._headers || {};
            this._headers[name] = value;
        }
        
        send(body) {
            if (interceptor) {
                interceptor({
                    method: this._method,
                    url: this._url,
                    headers: this._headers,
                    body: body,
                });
            }
            this.readyState = 4;
            this.status = 200;
            if (this.onreadystatechange) this.onreadystatechange();
            if (this.onload) this.onload();
        }
        
        addEventListener(event, handler) {
            this['on' + event] = handler;
        }
    };
}
```

## 5. 环境检测绕过清单

| 检测项 | Node.js 默认 | 需要补全的值 |
|--------|-------------|-------------|
| `typeof window` | `undefined` | `object` |
| `typeof document` | `undefined` | `object` |
| `typeof navigator` | `undefined` | `object` |
| `typeof process` | `object` (暴露) | 需要删除 |
| `typeof module` | `object` (暴露) | 需要删除 |
| `typeof global` | `object` (暴露) | 视情况删除 |
| `navigator.webdriver` | N/A | `false` / `undefined` |
| `window.chrome` | N/A | `{ runtime: {} }` |
| `navigator.plugins.length` | N/A | `> 0` |

### 清除 Node.js 特征

```javascript
function hideNodeFeatures(sandbox) {
    delete sandbox.process;
    delete sandbox.module;
    delete sandbox.exports;
    delete sandbox.require;
    delete sandbox.global;
    delete sandbox.__filename;
    delete sandbox.__dirname;
    delete sandbox.Buffer;
}
```

## 6. 完整环境补全模板（jsdom）

当最小补全不够时，使用 jsdom 提供完整 DOM 环境：

```javascript
const { JSDOM } = require('jsdom');

function createFullBrowserEnv(options = {}) {
    const dom = new JSDOM('<!DOCTYPE html><html><body></body></html>', {
        url: options.url || 'https://example.com',
        referrer: options.referrer || '',
        contentType: 'text/html',
        pretendToBeVisual: true,
        runScripts: 'dangerously',
        resources: 'usable',
    });
    
    const { window } = dom;
    
    // 补充 jsdom 缺少的 API
    if (!window.btoa) {
        window.btoa = (str) => Buffer.from(str, 'binary').toString('base64');
    }
    if (!window.atob) {
        window.atob = (b64) => Buffer.from(b64, 'base64').toString('binary');
    }
    
    return { dom, window, document: window.document };
}
```

## MCP 辅助确定环境需求

```
1. [camoufox-reverse] search_code(keyword="window|document|navigator|location|screen")
   → 先在已加载脚本中识别代码访问了哪些浏览器 API

2. [camoufox-reverse] add_init_script(script="记录被访问的全局变量和属性")
   + reload()
   + get_console_logs
   → 确定运行时真正访问了哪些环境字段

3. [camoufox-reverse] set_breakpoint_via_hook(target_function="环境检测函数路径")
   + get_breakpoint_data
   → 在环境检测函数入口捕获入参、返回值和调用栈，确认需要补全的关键分支
```
