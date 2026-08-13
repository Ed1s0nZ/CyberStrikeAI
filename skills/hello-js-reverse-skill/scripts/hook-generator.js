/**
 * Hook 代码生成器
 * 根据逆向目标快速生成 Hook 注入代码
 * 
 * 使用方式:
 *   node hook-generator.js --type=cookie
 *   node hook-generator.js --type=xhr --target="/api/data"
 *   node hook-generator.js --type=all
 *   node hook-generator.js --type=custom --function="encrypt" --module="CryptoJS"
 */

const HOOK_TEMPLATES = {

    cookie: (options = {}) => {
        const target = options.target || '';
        return `(function() {
    var _cookie = document.cookie || '';
    Object.defineProperty(document, 'cookie', {
        set: function(val) {
            ${target ? `if (val.indexOf('${target}') !== -1) {` : '{'}
                console.log('[Hook:Cookie] Set:', val);
                console.trace('[Hook:Cookie] 调用栈');
            }
            var parts = val.split(';');
            var mainPart = parts[0];
            if (_cookie) _cookie += '; ';
            _cookie += mainPart;
        },
        get: function() { return _cookie; },
        configurable: true
    });
    console.log('[Hook:Cookie] Cookie Hook 已注入');
})();`;
    },

    xhr: (options = {}) => {
        const target = options.target || '';
        return `(function() {
    var _open = XMLHttpRequest.prototype.open;
    var _send = XMLHttpRequest.prototype.send;
    var _setHeader = XMLHttpRequest.prototype.setRequestHeader;
    
    XMLHttpRequest.prototype.open = function(method, url) {
        this.__hookMethod = method;
        this.__hookUrl = url;
        this.__hookHeaders = {};
        return _open.apply(this, arguments);
    };
    
    XMLHttpRequest.prototype.setRequestHeader = function(name, value) {
        this.__hookHeaders[name] = value;
        return _setHeader.apply(this, arguments);
    };
    
    XMLHttpRequest.prototype.send = function(body) {
        ${target ? `if (this.__hookUrl && this.__hookUrl.indexOf('${target}') !== -1) {` : '{'}
            console.log('[Hook:XHR] ★', this.__hookMethod, this.__hookUrl);
            console.log('[Hook:XHR] Headers:', JSON.stringify(this.__hookHeaders));
            console.log('[Hook:XHR] Body:', body);
            console.trace('[Hook:XHR] 调用栈');
        }
        return _send.apply(this, arguments);
    };
    console.log('[Hook:XHR] XHR Hook 已注入');
})();`;
    },

    fetch: (options = {}) => {
        const target = options.target || '';
        return `(function() {
    var _fetch = window.fetch;
    window.fetch = function(input, init) {
        var url = typeof input === 'string' ? input : (input.url || '');
        ${target ? `if (url.indexOf('${target}') !== -1) {` : '{'}
            console.log('[Hook:Fetch] ★', url);
            console.log('[Hook:Fetch] Options:', JSON.stringify(init || {}));
            console.trace('[Hook:Fetch] 调用栈');
        }
        return _fetch.apply(this, arguments);
    };
    console.log('[Hook:Fetch] Fetch Hook 已注入');
})();`;
    },

    eval: () => {
        return `(function() {
    var _eval = window.eval;
    window.eval = function(code) {
        console.log('[Hook:eval] 代码长度:', typeof code === 'string' ? code.length : 'N/A');
        if (typeof code === 'string' && code.length < 10000) {
            console.log('[Hook:eval] 内容:', code.substring(0, 500));
        }
        return _eval.apply(this, arguments);
    };
    
    var _Function = Function;
    window.Function = function() {
        var body = arguments[arguments.length - 1];
        console.log('[Hook:Function] body长度:', body ? body.length : 0);
        if (body && body.length < 5000) {
            console.log('[Hook:Function] 内容:', body.substring(0, 500));
        }
        return _Function.apply(this, arguments);
    };
    window.Function.prototype = _Function.prototype;
    console.log('[Hook:eval] eval/Function Hook 已注入');
})();`;
    },

    json: () => {
        return `(function() {
    var _parse = JSON.parse;
    var _stringify = JSON.stringify;
    
    JSON.parse = function(text) {
        var result = _parse.apply(this, arguments);
        console.log('[Hook:JSON] parse:', typeof text === 'string' ? text.substring(0, 200) : typeof text);
        return result;
    };
    
    JSON.stringify = function(obj) {
        var result = _stringify.apply(this, arguments);
        console.log('[Hook:JSON] stringify:', result ? result.substring(0, 200) : result);
        return result;
    };
    console.log('[Hook:JSON] JSON Hook 已注入');
})();`;
    },

    base64: () => {
        return `(function() {
    var _atob = window.atob;
    var _btoa = window.btoa;
    
    window.atob = function(str) {
        var result = _atob(str);
        console.log('[Hook:Base64] atob:', str.substring(0, 80), '→', result.substring(0, 80));
        return result;
    };
    
    window.btoa = function(str) {
        var result = _btoa(str);
        console.log('[Hook:Base64] btoa:', str.substring(0, 80), '→', result.substring(0, 80));
        return result;
    };
    console.log('[Hook:Base64] Base64 Hook 已注入');
})();`;
    },

    websocket: () => {
        return `(function() {
    var _WS = window.WebSocket;
    window.WebSocket = function(url, protocols) {
        console.log('[Hook:WS] 连接:', url);
        var ws = new _WS(url, protocols);
        var _send = ws.send.bind(ws);
        ws.send = function(data) {
            console.log('[Hook:WS] 发送:', typeof data === 'string' ? data.substring(0, 200) : '[binary]');
            return _send(data);
        };
        ws.addEventListener('message', function(e) {
            console.log('[Hook:WS] 接收:', typeof e.data === 'string' ? e.data.substring(0, 200) : '[binary]');
        });
        return ws;
    };
    window.WebSocket.prototype = _WS.prototype;
    console.log('[Hook:WS] WebSocket Hook 已注入');
})();`;
    },

    debugger_bypass: () => {
        return `(function() {
    var _Function = Function;
    Function = function() {
        var body = arguments[arguments.length - 1];
        if (typeof body === 'string' && body.indexOf('debugger') !== -1) {
            arguments[arguments.length - 1] = body.replace(/debugger\\s*;?/g, '');
        }
        return _Function.apply(this, arguments);
    };
    Function.prototype = _Function.prototype;
    
    var _si = window.setInterval;
    window.setInterval = function(fn, ms) {
        if (typeof fn === 'function' && fn.toString().indexOf('debugger') > -1) return -1;
        if (typeof fn === 'string' && fn.indexOf('debugger') > -1) return -1;
        return _si.apply(this, arguments);
    };
    
    var _st = window.setTimeout;
    window.setTimeout = function(fn, ms) {
        if (typeof fn === 'function' && fn.toString().indexOf('debugger') > -1) return -1;
        if (typeof fn === 'string' && fn.indexOf('debugger') > -1) return -1;
        return _st.apply(this, arguments);
    };
    console.log('[Hook:AntiDebug] debugger 绕过已注入');
})();`;
    },

    stealth: () => {
        return `(function() {
    Object.defineProperty(navigator, 'webdriver', { get: function() { return undefined; }, configurable: true });
    
    window.chrome = window.chrome || {};
    window.chrome.runtime = window.chrome.runtime || {};
    
    Object.defineProperty(navigator, 'plugins', {
        get: function() {
            return [
                { name: 'Chrome PDF Plugin', filename: 'internal-pdf-viewer' },
                { name: 'Chrome PDF Viewer', filename: 'mhjfbmdgcfjbbpaeojofohoefgiehjai' },
                { name: 'Native Client', filename: 'internal-nacl-plugin' }
            ];
        }
    });
    
    Object.defineProperty(navigator, 'languages', {
        get: function() { return ['zh-CN', 'zh', 'en']; }
    });
    
    var _getParameter = WebGLRenderingContext.prototype.getParameter;
    WebGLRenderingContext.prototype.getParameter = function(param) {
        if (param === 37445) return 'Intel Inc.';
        if (param === 37446) return 'Intel Iris OpenGL Engine';
        return _getParameter.apply(this, arguments);
    };
    
    console.log('[Hook:Stealth] 隐身模式已注入');
})();`;
    },

    custom: (options = {}) => {
        const funcName = options.function || 'targetFunction';
        return `(function() {
    // 追踪 ${funcName} 函数
    if (typeof ${funcName} !== 'undefined') {
        var _orig = ${funcName};
        ${funcName} = function() {
            console.log('[Hook:Custom] ${funcName} 调用');
            console.log('[Hook:Custom] 参数:', JSON.stringify(Array.from(arguments)));
            var result = _orig.apply(this, arguments);
            console.log('[Hook:Custom] 返回:', JSON.stringify(result));
            console.trace('[Hook:Custom] 调用栈');
            return result;
        };
        console.log('[Hook:Custom] ${funcName} Hook 已注入');
    } else {
        console.warn('[Hook:Custom] ${funcName} 未定义，等待加载...');
        // 使用 getter 延迟 Hook
        var _origVal;
        Object.defineProperty(window, '${funcName}', {
            get: function() { return _origVal; },
            set: function(val) {
                if (typeof val === 'function') {
                    var _fn = val;
                    _origVal = function() {
                        console.log('[Hook:Custom] ${funcName} 调用');
                        console.log('[Hook:Custom] 参数:', JSON.stringify(Array.from(arguments)));
                        var result = _fn.apply(this, arguments);
                        console.log('[Hook:Custom] 返回:', JSON.stringify(result));
                        return result;
                    };
                } else {
                    _origVal = val;
                }
            },
            configurable: true
        });
    }
})();`;
    },
};

function generateHook(type, options = {}) {
    const generator = HOOK_TEMPLATES[type];
    if (!generator) {
        throw new Error(`未知的 Hook 类型: ${type}。可用类型: ${Object.keys(HOOK_TEMPLATES).join(', ')}`);
    }
    return generator(options);
}

function generateAllHooks(options = {}) {
    const hooks = [
        HOOK_TEMPLATES.debugger_bypass(),
        HOOK_TEMPLATES.stealth(),
        HOOK_TEMPLATES.cookie(options),
        HOOK_TEMPLATES.xhr(options),
        HOOK_TEMPLATES.fetch(options),
        HOOK_TEMPLATES.eval(),
        HOOK_TEMPLATES.base64(),
    ];
    return hooks.join('\n\n');
}

// CLI
if (require.main === module) {
    const args = process.argv.slice(2);
    const options = {};
    
    args.forEach(arg => {
        if (arg.startsWith('--type=')) options.type = arg.substring(7);
        else if (arg.startsWith('--target=')) options.target = arg.substring(9);
        else if (arg.startsWith('--function=')) options.function = arg.substring(11);
        else if (arg.startsWith('--output=')) options.output = arg.substring(9);
    });
    
    if (!options.type) {
        console.log('用法: node hook-generator.js --type=<type> [options]');
        console.log('');
        console.log('类型:');
        console.log('  cookie          Cookie setter 拦截');
        console.log('  xhr             XMLHttpRequest 拦截');
        console.log('  fetch           Fetch API 拦截');
        console.log('  eval            eval/Function 拦截');
        console.log('  json            JSON.parse/stringify 拦截');
        console.log('  base64          atob/btoa 拦截');
        console.log('  websocket       WebSocket 拦截');
        console.log('  debugger_bypass debugger 反调试绕过');
        console.log('  stealth         浏览器指纹隐身');
        console.log('  custom          自定义函数拦截');
        console.log('  all             生成所有 Hook（组合包）');
        console.log('');
        console.log('选项:');
        console.log('  --target=<url>     目标接口路径过滤');
        console.log('  --function=<name>  自定义函数名（type=custom 时）');
        console.log('  --output=<file>    输出到文件');
        process.exit(0);
    }
    
    let code;
    if (options.type === 'all') {
        code = generateAllHooks(options);
    } else {
        code = generateHook(options.type, options);
    }
    
    if (options.output) {
        const fs = require('fs');
        fs.writeFileSync(options.output, code);
        console.log(`Hook 代码已保存到: ${options.output}`);
    } else {
        console.log(code);
    }
}

module.exports = { generateHook, generateAllHooks, HOOK_TEMPLATES };
