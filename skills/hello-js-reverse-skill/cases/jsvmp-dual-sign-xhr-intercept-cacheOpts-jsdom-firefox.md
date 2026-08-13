# JSVMP 双签名 + XHR/fetch 拦截器 + cacheOpts 初始化 + jsdom Firefox 环境伪装

> 难度：★★★★★
> 还原方案：E: jsdom 环境模拟（喂入-截出策略）
> 实现语言：Node.js
> 最后验证日期：2026-04-20
> 平台类型：海外短视频平台（国际版）

---

## 技术指纹（供 Phase 0.5 自动匹配）

### JS 特征
- [x] `webmssdk.es5.js` — JSVMP 签名引擎（380KB+），UMD 导出到 `window.byted_acrawler`
- [x] 三层 SDK 联动：`sdk-glue.js`（100KB）→ `bdms.js`（147KB）→ `webmssdk.es5.js`（387KB）
- [x] 存在 while-switch 风格的 JSVMP 解释器循环
- [x] 内部状态对象含 `bogusIndex`、`msNewTokenList`、`moveList`、`clickList`、`activeState`
- [x] `_0x` 前缀变量大量出现（构造函数、工厂函数、闭包变量）
- [x] 页面 inline script 调用 `window._SdkGlueInit(config, resources)` 完成初始化
- [x] 初始化配置含 `cacheOpts` 字段（路径注册 + 缓存策略），区别于旧版仅 `bdms.paths`

### 参数特征
- [x] `X-Bogus`：约 180~192 字符，Base64 变体编码 — 由 JSVMP XHR 拦截器生成（新版主签名）
- [x] `X-Gnarly`：约 120~140 字符，自定义编码 — 由 `frontierSign()` 生成的辅助签名
- [x] `msToken`：约 140 字符，Base64 编码，来自目标域名的 Cookie
- [x] `verifyFp` / `fp`：格式 `verify_xxxx_xxxx_xxxx_xxxx_xxxxxxxxxxxx`

### 请求特征
- [x] `_SdkGlueInit(config, resources)` 初始化时 config 含 `cacheOpts: { paths: [...], ttl: N }`
- [x] JSVMP 同时修改 `XMLHttpRequest.prototype.open` 和 `window.fetch`（双通道拦截）
- [x] 拦截器对匹配 `enablePathList` 的 URL 追加 `X-Bogus`，同时在 header 中注入 `X-Gnarly`
- [x] Cookie 中包含动态字段：`ttwid`、`__ac_nonce`、`__ac_signature`、`s_v_web_id`、`odin_tt`
- [x] 存在 `/web/report` 默认拦截路径 + 业务路径通过 `cacheOpts.paths` 注入

### 反调试特征
- [x] JSVMP 字节码执行，无法直接设置断点
- [x] debugger 定时器陷阱
- [x] 环境检测（60+ 项）：在原有 58 项基础上新增 Firefox 特有检测：
  - `navigator.buildID` 存在性（Firefox 独有）
  - `CSS2Properties` vs `CSSStyleDeclaration` 类型检测
  - `Function.prototype.toString` 格式差异（Firefox 返回 `function name() {\n    [native code]\n}` 含换行缩进）
- [x] 非真实浏览器环境生成的签名会被服务端**静默拒绝**（返回 HTTP 200 + 空 body）

### 混淆类型
- [x] JSVMP（JavaScript Virtual Machine Protection），字节码存储为十六进制字符串

### 指纹检测规则（Agent 执行）

```
快速检测命令（30秒内完成）：
  - search_code(keyword="webmssdk") → 命中 → 直接定位本案例
  - search_code(keyword="byted_acrawler") → 命中 → 直接定位本案例
  - search_code(keyword="_SdkGlueInit") → 命中 → 直接定位本案例
  - search_code(keyword="cacheOpts") → 命中 → 区分本案例（含 cacheOpts）vs 旧版（仅 bdms.paths）
  - search_code(keyword="X-Gnarly") → 命中 → 确认双签名变体
  匹配判定：前 3 项命中任意 1 项 + cacheOpts 命中 → 高置信度匹配本案例
```

### 指纹检测规则 — 区分国际版 vs 国内版技术特征

```
区分方法（技术特征层面）：
  国际版特征：
    - SDK 路径含 "i18n" 或 "intl" 标识
    - cacheOpts 配置中 paths 含 "/api/v1/" 前缀（国际版 API 路径风格）
    - X-Gnarly header 存在（国际版双签名）
    - msToken Cookie 域名为国际版域名
  国内版特征：
    - SDK 路径不含 i18n 标识
    - 仅 bdms.paths 配置（无 cacheOpts）
    - 仅 a_bogus 参数（无 X-Gnarly）
    - msToken Cookie 域名为国内版域名
  技术判定：cacheOpts 存在 + X-Gnarly 存在 → 国际版变体
```

---

## 加密方案

- **算法**：JSVMP 自定义字节码虚拟机，内部算法不可直接读取
  - `X-Bogus`（~192 字符 Base64 变体）：由 JSVMP XHR/fetch 拦截器生成
  - `X-Gnarly`（~130 字符自定义编码）：由 `frontierSign()` 导出函数生成
- **密钥来源**：动态计算 — 浏览器环境指纹（Canvas/WebGL/navigator/screen 等 60+ 项）+ 请求 URL query string + User-Agent + 时间戳
- **加密流程**：
  1. `sdk-glue` 通过 `_SdkGlueInit(config, resources)` 初始化，注入 `cacheOpts` 路径列表
  2. `webmssdk` JSVMP 构造函数执行字节码，同时修改 `XMLHttpRequest.prototype.open` 和 `window.fetch`
  3. 当请求 URL 匹配 `enablePathList` 时，XHR 拦截器追加 `X-Bogus` 到 URL，fetch 拦截器注入 `X-Gnarly` 到 header
  4. JSVMP 读取完整 URL query string、`document.cookie`、`navigator.*`、`screen.*` 等 60+ 项环境指纹
  5. 双签名并行生成：X-Bogus（URL 参数）+ X-Gnarly（请求头）
- **签名公式**：无法提取（JSVMP 字节码保护）。采用「喂入-截出」策略

---

## 已验证定位路径（Phase 0.5 命中后直接执行）

### Phase 1：网络捕获定位接口

```
步骤 1: start_network_capture(capture_body=True) + 触发业务操作
步骤 2: list_network_requests → 捕获带 X-Bogus 和 X-Gnarly 的请求
步骤 3: search_code(keyword="bdms") → 找到三个关键脚本 CDN 地址：
        - webmssdk.es5.js (387KB) — JSVMP 签名引擎
        - bdms.js (147KB) — ByteDance Monitoring System
        - sdk-glue.js (100KB) — SDK 胶水层
```

### Phase 2：初始化链路还原

```
步骤 4: search_code(keyword="cacheOpts") → 定位到 sdk-glue 中的新版路径配置机制
步骤 5: 页面 inline script 中找到 _SdkGlueInit 调用及完整的 cacheOpts 配置
步骤 6: search_code(keyword="frontierSign") → 找到 webmssdk 的导出结构：init() / frontierSign() / setConfig()
步骤 7: search_code(keyword="X-Gnarly") → 确认 X-Gnarly 由 fetch 拦截器注入 header
步骤 8: search_code(keyword="bogusIndex") → 确认 X-Bogus 由 XHR 拦截器追加到 URL
```

### Phase 3：jsdom 沙箱验证

```
步骤 9:  在 jsdom (runScripts: 'dangerously') 中依次加载三个脚本
步骤 10: 调用 _SdkGlueInit 并传入 cacheOpts 配置
步骤 11: 触发 XHR → 截获 X-Bogus；触发 fetch → 截获 X-Gnarly
步骤 12: 浏览器生成的签名用 Node.js 请求成功 → 确认非 TLS 问题
步骤 13: jsdom 生成的签名被服务端拒绝 → 确认是环境指纹差异
```

### Phase 4：环境指纹对比（核心突破点 — Firefox 伪装）

```
步骤 14: 用 Camoufox 启动反检测浏览器（注意：Camoufox 基于 Firefox）
步骤 15: navigate 到目标页面
步骤 16: 通过 evaluate_js 分批采集浏览器完整环境指纹（5 批次）
步骤 17: 在 jsdom 中运行完全相同的采集代码
步骤 18: 逐项对比发现 62 项差异（含 Firefox 特有的 4 项）
关键发现：
  - Camoufox 是 Firefox 内核，Function.prototype.toString 格式与 Chrome 不同
  - Firefox 返回 "function name() {\n    [native code]\n}" （含换行和 4 空格缩进）
  - Chrome 返回 "function name() { [native code] }" （单行）
  - JSVMP 会检测 toString 格式来判断浏览器类型
  → jsdom 的 markNative 必须输出 Firefox 格式的 native code 字符串
```

### Phase 5：环境补丁与验证

```
步骤 19: 编写 patchEnvironment() 修复全部 62 项差异
         核心改动：markNative 输出 Firefox 格式 native code
步骤 20: 从 jsdom 内部验证所有检测点通过
步骤 21: 连续 5 次请求，全部返回完整数据，确认稳定性
步骤 22: 使用 got-scraping 替代 axios，模拟 Firefox TLS 指纹
```

---

## 还原代码模板

### 核心函数：markNative — Firefox 格式 native code 伪装

```javascript
const _origFnToString = win.Function.prototype.toString;
const nativeFnSet = new WeakSet();

function markNative(fn) {
  if (typeof fn === 'function') {
    nativeFnSet.add(fn);
    try {
      const name = fn.name || '';
      // Firefox 格式：含换行和 4 空格缩进
      Object.defineProperty(fn, 'toString', {
        value: function () {
          return `function ${name}() {\n    [native code]\n}`;
        },
        writable: true, configurable: true,
      });
    } catch (e) {}
  }
  return fn;
}

// jsdom 内部函数的通用源码特征
const jsdomPatterns = [
  /^\s*\w+\s*\([^)]*\)\s*\{[\s\S]*?const\s+esValue\s*=/,
  /^\s*function\s*\([^)]*\)\s*\{[\s\S]*?this\._globalObject/,
  /^\s*\w+\s*\([^)]*\)\s*\{\s*const\s+\w+\s*=\s*this\s*!==/,
];

win.Function.prototype.toString = function () {
  if (nativeFnSet.has(this)) {
    // Firefox 格式
    return `function ${this.name || ''}() {\n    [native code]\n}`;
  }
  let src;
  try { src = _origFnToString.call(this); } catch (e) {
    return 'function () {\n    [native code]\n}';
  }
  for (const pat of jsdomPatterns) {
    if (pat.test(src)) return `function ${this.name || ''}() {\n    [native code]\n}`;
  }
  return src;
};
```

### 核心函数：cacheOpts 初始化

```javascript
function initSdkWithCacheOpts(dom) {
  const win = dom.window;
  // 加载三层 SDK
  // sdk-glue.js → bdms.js → webmssdk.es5.js

  // 新版初始化：传入 cacheOpts
  win.eval(`
    window._SdkGlueInit({
      cacheOpts: {
        paths: [
          '^/api/v1/',
          '^/aweme/v1/',
          '^/web/api/'
        ],
        ttl: 300
      }
    }, {
      bdms: { paths: ['^/aweme/v1/', '^/web/'] }
    });
  `);
}
```

### 核心函数：双签名生成（喂入-截出策略）

```javascript
function generateDualSign(fullUrl, cookieStr) {
  const dom = initSdk();  // jsdom + Firefox 环境补丁 + SDK 加载（单例）
  const win = dom.window;

  // 写入 Cookie
  if (cookieStr) {
    cookieStr.split(';').forEach(c => {
      try { win.document.cookie = c.trim(); } catch (e) {}
    });
  }

  return new Promise((resolve) => {
    let xBogus = null;
    let xGnarly = null;

    // Hook XHR 截获 X-Bogus
    win.__capturedUrls = [];
    const origXhrOpen = win.XMLHttpRequest.prototype.open;
    // ... (XHR hook 截获 URL 中的 X-Bogus)

    // Hook fetch 截获 X-Gnarly
    const origFetch = win.fetch;
    win.fetch = function(url, opts) {
      if (opts && opts.headers) {
        xGnarly = opts.headers['X-Gnarly'] || null;
      }
      return origFetch.apply(this, arguments);
    };

    // 触发 XHR
    win.eval(`(function(){
      var x = new XMLHttpRequest();
      x.open('GET','${fullUrl.replace(/'/g, "\\'")}',true);
      x.send();
    })()`);

    setTimeout(() => {
      for (const url of win.__capturedUrls) {
        const m = url.match(/[&?]X-Bogus=([^&]+)/);
        if (m) { xBogus = decodeURIComponent(m[1]); break; }
      }
      resolve({ xBogus, xGnarly });
    }, 500);
  });
}
```

### 核心函数：got-scraping TLS 指纹模拟

```javascript
const { gotScraping } = require('got-scraping');

async function requestWithFirefoxTLS(url, headers) {
  const response = await gotScraping({
    url,
    headers,
    headerGeneratorOptions: {
      browsers: [{ name: 'firefox', minVersion: 115 }],
      operatingSystems: ['macos'],
    },
  });
  return JSON.parse(response.body);
}
```

---

## 踩坑记录

| # | 坑 | 现象 | 解决方法 |
|---|---|------|---------|
| 1 | Firefox vs Chrome 的 native code 格式差异 | Camoufox 基于 Firefox，toString 含换行缩进；jsdom 默认输出 Chrome 格式单行 | markNative 输出 Firefox 格式：`function name() {\n    [native code]\n}` |
| 2 | cacheOpts 未传入导致路径注册失败 | 旧版只需 `bdms.paths`，新版必须同时传 `cacheOpts` | 从页面 inline script 提取完整 cacheOpts 配置 |
| 3 | fetch 拦截器未 Hook 导致 X-Gnarly 丢失 | 只 Hook 了 XHR，漏掉 fetch 通道 | 同时 Hook `XMLHttpRequest.prototype.open` 和 `window.fetch` |
| 4 | TLS 指纹不匹配 | axios 的 TLS 指纹被识别为 Node.js | 使用 got-scraping 模拟 Firefox TLS 指纹 |
| 5 | navigator.buildID 缺失 | Firefox 独有属性，jsdom 不提供 | `Object.defineProperty(navigator, 'buildID', { value: '20230927232528' })` |
| 6 | CSS2Properties 类型检测 | Firefox 用 CSS2Properties，Chrome 用 CSSStyleDeclaration | 在 jsdom 中注册 `win.CSS2Properties = win.CSSStyleDeclaration` 别名 |

---

## 变体说明

| 变体 | 差异点 | 影响 |
|------|--------|------|
| 国内版（仅 a_bogus） | 无 X-Gnarly，无 cacheOpts，仅 bdms.paths 配置 | 参考 `jsvmp-xhr-interceptor-env-emulation.md` 案例 |
| 旧版国际版（无 cacheOpts） | 有 X-Bogus 但初始化走旧版 bdms.paths | 初始化方式不同，签名逻辑相同 |
| SDK 版本迭代 | webmssdk 版本号更新，字节码变化 | 环境检测项可能增加，需重新 compare_env |
| Chrome 环境伪装 | 如果不用 Camoufox 而用 Chrome 采集基准 | markNative 改回 Chrome 单行格式 |

---

## 浏览器指纹采集方法

使用 camoufox-reverse MCP 在 Firefox 内核浏览器中采集指纹：

```
步骤 1: launch_browser({headless: false, os_type: "macos", locale: "zh-CN"})
步骤 2: navigate({url: "目标页面", wait_until: "domcontentloaded"})
步骤 3: 分批采集环境指纹（5 批次）
```

### 批次 A-D：同 jsvmp-xhr-interceptor-env-emulation.md

### 批次 E（新增）：Firefox 特有属性

```javascript
JSON.stringify({
  nav_buildID: navigator.buildID,
  css2props_type: typeof window.CSS2Properties,
  fn_toString_format: document.createElement.toString(),
  // Firefox: "function createElement() {\n    [native code]\n}"
  // Chrome:  "function createElement() { [native code] }"
  mozInnerScreenX: window.mozInnerScreenX,
  mozInnerScreenY: window.mozInnerScreenY,
  InstallTrigger: typeof window.InstallTrigger
})
```

---

## 环境检测差异全表（62 项）

### 继承自基础案例的 58 项

参考 `jsvmp-xhr-interceptor-env-emulation.md` 的完整 58 项差异表。

### 新增 Firefox 特有检测（4 项）

| 检测点 | Firefox 真实值 | jsdom 原始值 | 修复方式 |
|--------|---------------|-------------|---------|
| `Function.prototype.toString` 格式 | `function name() {\n    [native code]\n}` | 暴露实现代码 | markNative 输出 Firefox 格式（含 `\n` 和 4 空格缩进） |
| `navigator.buildID` | `"20230927232528"` | `undefined` | `Object.defineProperty` 设置 |
| `window.CSS2Properties` | `function CSS2Properties()` | `undefined` | 别名指向 `CSSStyleDeclaration` |
| `window.InstallTrigger` | `undefined`（已废弃但 typeof 仍返回 `"undefined"` 而非报错） | 不存在 | 无需修复（行为一致） |

---

## 关键经验总结

### 1. Firefox 格式 native code 是本案例的核心突破点

Camoufox 基于 Firefox 内核，其 `Function.prototype.toString` 对原生函数返回的格式与 Chrome 不同。JSVMP 会检测这个格式来判断浏览器类型。如果 jsdom 的 markNative 输出 Chrome 格式，会导致浏览器类型判断不一致，签名被拒。

### 2. 双签名 = 双通道拦截

X-Bogus 通过 XHR 拦截器追加到 URL，X-Gnarly 通过 fetch 拦截器注入到 header。必须同时 Hook 两个通道才能截获完整签名。

### 3. cacheOpts 是新版初始化的关键

旧版只需 `bdms.paths`，新版必须传入 `cacheOpts` 才能正确注册业务路径。缺少 cacheOpts 会导致拦截器不触发。

### 4. got-scraping 解决 TLS 指纹问题

Node.js 原生 HTTP 客户端的 TLS 指纹会被识别。使用 got-scraping 可以模拟 Firefox 的 TLS Client Hello 指纹，与 Camoufox 采集的环境保持一致。
