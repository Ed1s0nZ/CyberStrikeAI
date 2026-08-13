# JS Hook 技术大全

## 概述

Hook 是 JS 逆向中最核心的技术之一。通过劫持/拦截 JS 原生函数或对象属性，可以捕获加密函数的
输入输出、Cookie 的生成过程、网络请求的参数构造等关键信息。

## 注入方式

### 通过 MCP 注入（推荐）

```
[camoufox-reverse] add_init_script(script=HookScript)
→ 在页面脚本加载前注入，确保 Hook 在目标代码之前生效

[camoufox-reverse] evaluate_js(expression=HookScript)
→ 在当前页面上下文执行，适合页面已加载后的动态注入

[camoufox-reverse] inject_hook_preset(preset="xhr|fetch|crypto|websocket|debugger_bypass|cookie|runtime_probe")
→ 一键注入预设 Hook，覆盖常见逆向场景（v2.5.0 新增 cookie / runtime_probe 两个预设）

[camoufox-reverse] hook_function(function_path="目标函数", hook_code="...", position="before|after|replace")
→ 对指定函数注入自定义 Hook
```

## Hook 模板库

### 1. Cookie Setter Hook

**用途**：监控所有 Cookie 写入操作，定位动态 Cookie 生成逻辑

```javascript
(function() {
    let cookieTemp = '';
    Object.defineProperty(document, 'cookie', {
        set: function(val) {
            console.log('[Hook] Cookie Set:', val);
            if (val.indexOf('目标cookie名') !== -1) {
                console.log('[Hook] ★ 目标Cookie捕获:', val);
                console.trace('[Hook] Cookie 调用栈');
            }
            cookieTemp = val;
            return val;
        },
        get: function() {
            return cookieTemp;
        }
    });
})();
```

### 2. XHR Hook

**用途**：拦截所有 XMLHttpRequest 请求，捕获完整请求参数

```javascript
(function() {
    const _open = XMLHttpRequest.prototype.open;
    const _send = XMLHttpRequest.prototype.send;
    const _setHeader = XMLHttpRequest.prototype.setRequestHeader;
    
    XMLHttpRequest.prototype.open = function(method, url) {
        this._hookMethod = method;
        this._hookUrl = url;
        this._hookHeaders = {};
        console.log('[Hook] XHR Open:', method, url);
        return _open.apply(this, arguments);
    };
    
    XMLHttpRequest.prototype.setRequestHeader = function(name, value) {
        this._hookHeaders[name] = value;
        return _setHeader.apply(this, arguments);
    };
    
    XMLHttpRequest.prototype.send = function(body) {
        console.log('[Hook] XHR Send:', {
            method: this._hookMethod,
            url: this._hookUrl,
            headers: this._hookHeaders,
            body: body
        });
        if (this._hookUrl.indexOf('目标接口') !== -1) {
            console.log('[Hook] ★ 目标请求捕获');
            console.trace('[Hook] 请求调用栈');
        }
        return _send.apply(this, arguments);
    };
})();
```

### 3. Fetch Hook

**用途**：拦截 fetch API 请求

```javascript
(function() {
    const _fetch = window.fetch;
    window.fetch = function(url, options) {
        console.log('[Hook] Fetch:', url, options);
        if (typeof url === 'string' && url.indexOf('目标接口') !== -1) {
            console.log('[Hook] ★ 目标Fetch捕获');
            console.trace('[Hook] Fetch 调用栈');
        }
        return _fetch.apply(this, arguments);
    };
})();
```

### 4. $.ajax Hook（jQuery 场景）

**用途**：拦截 jQuery AJAX 请求，捕获加密参数

```javascript
(function() {
    if (typeof $ === 'undefined' || typeof $.ajax === 'undefined') return;
    
    const _ajax = $.ajax;
    $.ajax = function(options) {
        if (typeof options === 'object') {
            console.log('[Hook] $.ajax:', {
                url: options.url,
                method: options.type || options.method,
                data: options.data,
                headers: options.headers
            });
            
            if (options.url && options.url.indexOf('目标接口') !== -1) {
                console.log('[Hook] ★ 目标Ajax捕获');
                if (options.data && options.data.m) {
                    console.log('[Hook] 加密参数 m:', options.data.m);
                }
            }
        }
        return _ajax.apply(this, arguments);
    };
    
    // 同时 Hook ajaxSetup
    if ($.ajaxSetup) {
        const _setup = $.ajaxSetup;
        $.ajaxSetup = function(options) {
            console.log('[Hook] $.ajaxSetup:', options);
            return _setup.apply(this, arguments);
        };
    }
})();
```

### 5. eval / Function Hook

**用途**：捕获动态生成和执行的代码

```javascript
(function() {
    // eval Hook
    const _eval = window.eval;
    window.eval = function(code) {
        console.log('[Hook] eval 调用, 代码长度:', (typeof code === 'string') ? code.length : 'N/A');
        if (typeof code === 'string' && code.length < 5000) {
            console.log('[Hook] eval 代码:', code.substring(0, 500));
        }
        return _eval.apply(this, arguments);
    };
    
    // Function 构造器 Hook
    const _Function = Function;
    const handler = {
        construct(target, args) {
            const body = args[args.length - 1];
            console.log('[Hook] new Function, body 长度:', body ? body.length : 0);
            if (body && body.indexOf('目标关键词') !== -1) {
                console.log('[Hook] ★ 目标 Function 捕获:', body.substring(0, 500));
            }
            return new target(...args);
        },
        apply(target, thisArg, args) {
            const body = args[args.length - 1];
            console.log('[Hook] Function(), body 长度:', body ? body.length : 0);
            return target.apply(thisArg, args);
        }
    };
    window.Function = new Proxy(_Function, handler);
})();
```

### 6. JSON.parse Hook

**用途**：捕获 JSON 解析操作，常用于响应数据解密场景

```javascript
(function() {
    const _parse = JSON.parse;
    JSON.parse = function(text) {
        const result = _parse.apply(this, arguments);
        console.log('[Hook] JSON.parse:', typeof text === 'string' ? text.substring(0, 200) : text);
        return result;
    };
})();
```

### 7. JSON.stringify Hook

**用途**：捕获 JSON 序列化操作，常在签名前执行

```javascript
(function() {
    const _stringify = JSON.stringify;
    JSON.stringify = function(obj) {
        const result = _stringify.apply(this, arguments);
        console.log('[Hook] JSON.stringify:', result ? result.substring(0, 200) : result);
        return result;
    };
})();
```

### 8. atob / btoa Hook

**用途**：捕获 Base64 编解码操作

```javascript
(function() {
    const _atob = window.atob;
    const _btoa = window.btoa;
    
    window.atob = function(str) {
        const result = _atob(str);
        console.log('[Hook] atob:', str.substring(0, 100), '→', result.substring(0, 100));
        return result;
    };
    
    window.btoa = function(str) {
        const result = _btoa(str);
        console.log('[Hook] btoa:', str.substring(0, 100), '→', result.substring(0, 100));
        return result;
    };
})();
```

### 9. setTimeout / setInterval Hook

**用途**：监控定时器调用，识别反调试和周期性操作

```javascript
(function() {
    const _setTimeout = window.setTimeout;
    const _setInterval = window.setInterval;
    
    window.setTimeout = function(fn, delay) {
        const fnStr = typeof fn === 'function' ? fn.toString().substring(0, 200) : String(fn).substring(0, 200);
        // 过滤 debugger 反调试
        if (fnStr.indexOf('debugger') !== -1) {
            console.log('[Hook] setTimeout 拦截 debugger，已跳过');
            return -1;
        }
        return _setTimeout.apply(this, arguments);
    };
    
    window.setInterval = function(fn, delay) {
        const fnStr = typeof fn === 'function' ? fn.toString().substring(0, 200) : String(fn).substring(0, 200);
        if (fnStr.indexOf('debugger') !== -1) {
            console.log('[Hook] setInterval 拦截 debugger，已跳过');
            return -1;
        }
        return _setInterval.apply(this, arguments);
    };
})();
```

### 10. WebSocket Hook

**用途**：拦截 WebSocket 消息

```javascript
(function() {
    const _WebSocket = window.WebSocket;
    window.WebSocket = function(url, protocols) {
        console.log('[Hook] WebSocket 连接:', url);
        const ws = new _WebSocket(url, protocols);
        
        const _send = ws.send.bind(ws);
        ws.send = function(data) {
            console.log('[Hook] WS 发送:', data);
            return _send(data);
        };
        
        ws.addEventListener('message', function(event) {
            console.log('[Hook] WS 接收:', event.data);
        });
        
        return ws;
    };
    window.WebSocket.prototype = _WebSocket.prototype;
})();
```

### 11. window 属性访问 Hook（蜜罐检测）

**用途**：监控对 window 属性的访问，识别蜜罐检测

```javascript
(function() {
    const handler = {
        get(target, prop, receiver) {
            if (['__selenium', '__webdriver_script_fn', '__driver_evaluate',
                 '$cdc_asdjflasutopfhvcZLmcfl_', 'callPhantom', '_phantom'].includes(prop)) {
                console.log('[Hook] ★ 蜜罐属性访问:', prop);
                return undefined;
            }
            return Reflect.get(target, prop, receiver);
        }
    };
    // 注意：直接代理 window 风险较大，仅在需要时使用
})();
```

### 12. Canvas 指纹 Hook

**用途**：拦截 Canvas 指纹采集

```javascript
(function() {
    const _toDataURL = HTMLCanvasElement.prototype.toDataURL;
    HTMLCanvasElement.prototype.toDataURL = function() {
        console.log('[Hook] Canvas toDataURL 调用');
        console.trace('[Hook] Canvas 调用栈');
        return _toDataURL.apply(this, arguments);
    };
    
    const _toBlob = HTMLCanvasElement.prototype.toBlob;
    HTMLCanvasElement.prototype.toBlob = function() {
        console.log('[Hook] Canvas toBlob 调用');
        return _toBlob.apply(this, arguments);
    };
})();
```

### 13. Navigator 属性 Hook

**用途**：伪装浏览器指纹

```javascript
(function() {
    Object.defineProperty(navigator, 'webdriver', {
        get: () => false
    });
    Object.defineProperty(navigator, 'plugins', {
        get: () => [1, 2, 3, 4, 5]
    });
    Object.defineProperty(navigator, 'languages', {
        get: () => ['zh-CN', 'zh', 'en']
    });
})();
```

## 组合 Hook 模板

### 通用逆向分析 Hook（一键注入）

```javascript
(function() {
    console.log('[Hook] ========== 通用逆向Hook已注入 ==========');
    
    // 1. Cookie 监控
    let _cookie = document.cookie;
    Object.defineProperty(document, 'cookie', {
        set(v) { console.log('[Cookie] Set:', v); _cookie += '; ' + v; },
        get() { return _cookie; }
    });
    
    // 2. XHR 监控
    const _xhrOpen = XMLHttpRequest.prototype.open;
    const _xhrSend = XMLHttpRequest.prototype.send;
    XMLHttpRequest.prototype.open = function(m, u) { this._m = m; this._u = u; return _xhrOpen.apply(this, arguments); };
    XMLHttpRequest.prototype.send = function(b) { console.log('[XHR]', this._m, this._u, b); return _xhrSend.apply(this, arguments); };
    
    // 3. Fetch 监控
    const _fetch = window.fetch;
    window.fetch = function() { console.log('[Fetch]', ...arguments); return _fetch.apply(this, arguments); };
    
    // 4. debugger 拦截
    const _si = window.setInterval;
    window.setInterval = function(fn, d) {
        if (typeof fn === 'function' && fn.toString().indexOf('debugger') > -1) return -1;
        return _si.apply(this, arguments);
    };
    
    console.log('[Hook] ========== Hook注入完成 ==========');
})();
```

## MCP 注入最佳实践

1. **使用 `add_init_script`**：确保 Hook 在目标代码之前生效
2. **优先使用 `inject_hook_preset`**：一键注入 xhr/fetch/crypto/websocket/debugger_bypass/cookie/runtime_probe 预设 Hook（v2.5.0 更新）
3. **使用 `hook_function`**：对特定函数注入 before/after/replace Hook
4. **使用 `console.log` 输出**：通过 `get_console_logs` 收集结果
5. **使用 `console.trace`**：在关键点输出调用栈
6. **Camoufox 优势**：Juggler 协议沙箱隔离，Hook 不会被页面 JS 检测到
7. **使用 Proxy 代替直接覆写**：更隐蔽，不改变 `typeof` 结果
8. **首屏挑战页用 `navigate(pre_inject_hooks=[...])`** [v2.5.0]：RS/Akamai 首包挑战在 hook 装好前就跑完了，用这个参数让 hook 先装再 goto
9. **装完 hook 想让它先于页面 JS 跑**：用 `reload_with_hooks()` [v2.5.0] 替代裸 `reload()`，同时会清掉各类 `__mcp_*_log`

---

## cookie 预设：原型链级 document.cookie Hook（v2.5.0 新增）

### 为什么不能直接 `Object.defineProperty(document, 'cookie', ...)`

`document.cookie` 的 getter/setter 定义在**原型链**上（具体是 `Document.prototype` 或 `HTMLDocument.prototype`），不在 `document` 实例上。直接在实例上 defineProperty 会被浏览器忽略或抛错——这是很多自写 Hook 脚本"看起来装上了但完全抓不到 cookie 写入"的根本原因。

### 正确做法（cookie_hook.js 的实现）

```javascript
// 沿原型链找到定义 cookie 描述符的 owner
function findCookieDescriptor() {
    var proto = Object.getPrototypeOf(document);
    while (proto) {
        var d = Object.getOwnPropertyDescriptor(proto, 'cookie');
        if (d) return { descriptor: d, owner: proto };
        proto = Object.getPrototypeOf(proto);
    }
    return null;
}

var found = findCookieDescriptor();
if (!found) return;

var origSet = found.descriptor.set;
var origGet = found.descriptor.get;

// 在正确的 owner 上替换（关键：owner 是 Document.prototype 或 HTMLDocument.prototype）
Object.defineProperty(found.owner, 'cookie', {
    set: function (value) {
        window.__mcp_cookie_log.push({
            op: 'set', value: String(value),
            stack: new Error().stack, ts: Date.now()
        });
        return origSet.call(this, value);
    },
    get: function () {
        var v = origGet.call(this);
        window.__mcp_cookie_log.push({
            op: 'get', value: String(v),
            stack: new Error().stack, ts: Date.now()
        });
        return v;
    },
    configurable: true, enumerable: true
});
```

### 使用方式（三步）

```
# 1. 注入（默认持久化）
[camoufox-reverse] inject_hook_preset(preset="cookie", persistent=True)

# 2. 触发场景

# 3. 拉日志（两种方式）
# a. 读原始日志
[camoufox-reverse] evaluate_js(expression="JSON.stringify(window.__mcp_cookie_log.slice(-20))")

# b. 或直接走 Cookie 归因（推荐）
[camoufox-reverse] analyze_cookie_sources(name_filter="目标cookie名")
```

### 适用场景

- 所有涉及 JS 写入 cookie 的场景（eval 首包、指纹 cookie、JS 计算 token 后 `document.cookie = ...`）
- **不适用**：HTTP Set-Cookie 写入的场景（这种用 `analyze_cookie_sources` 的 http_responses 分支）

---

## runtime_probe 预设：低开销广谱运行时探针（v2.5.0 新增）

### 与 jsvmp_hook 的区别

| 维度 | `jsvmp_hook`（`hook_jsvmp_interpreter`） | `runtime_probe` |
|------|---------------------------------------|----------------|
| 实现方式 | 在 navigator/screen 等全局对象上装 **Proxy** | **不装 Proxy**，只 override 具体热点 API |
| 开销 | 高（每次属性读取都进 Proxy trap） | 低（只在调用热点 API 时记录） |
| 覆盖面 | 全局对象所有属性 + apply/call/bind/Reflect.* | 固定一组 API |
| 安全性 | 个别页面可能被 Proxy 破坏 | 非常安全 |
| 典型用途 | JSVMP 深度分析 | "这个页面都在做什么"快速摸底 |

### runtime_probe 覆盖的 API 清单

| 类别 | 覆盖项 | 日志 type |
|------|-------|----------|
| XHR | `XMLHttpRequest.prototype.open/send` | `xhr_open` / `xhr_send` |
| fetch | `window.fetch` | `fetch` |
| Canvas 指纹 | `HTMLCanvasElement.prototype.toDataURL` | `canvas_toDataURL` |
| Canvas 上下文 | `HTMLCanvasElement.prototype.getContext` | `canvas_getContext` |
| WebGL | `WebGLRenderingContext.prototype.getParameter` | `webgl_getParameter` |
| navigator | userAgent / platform / language / languages / webdriver / hardwareConcurrency / deviceMemory / vendor / appVersion / plugins / mimeTypes 的 getter | `nav_read` |
| 事件 | `EventTarget.prototype.addEventListener`（只记 mouse/key/devicemotion 等 bot 检测类） | `addEventListener` |

### 使用方式

```
# 1. 注入
[camoufox-reverse] inject_hook_preset(preset="runtime_probe", persistent=True)

# 2. 触发场景

# 3. 拉日志（自动按 type 聚合）
[camoufox-reverse] get_runtime_probe_log(limit=300)

# 按类型过滤
[camoufox-reverse] get_runtime_probe_log(type_filter="canvas_toDataURL", limit=50)
[camoufox-reverse] get_runtime_probe_log(type_filter="xhr_send", limit=100)
[camoufox-reverse] get_runtime_probe_log(type_filter="nav_read", limit=100)
```

### 典型诊断模式

- **反爬是否做 canvas 指纹检测？** → 看 `by_type.canvas_toDataURL` 是否 > 0
- **反爬是否读 navigator.webdriver？** → `get_runtime_probe_log(type_filter="nav_read")` 里找 `prop: "webdriver"`
- **反爬是否检测鼠标移动？** → 看 `by_type.addEventListener` 里 mousemove/mousedown 的计数
