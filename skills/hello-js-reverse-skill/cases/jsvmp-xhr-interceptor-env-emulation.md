# JSVMP 字节码虚拟机 + XHR 拦截器 + 多层 SDK 联动 + jsdom 全量环境伪装

> 难度：★★★★★
> 还原方案：E: jsdom 环境模拟（喂入-截出策略）
> 实现语言：Node.js
> 最后验证日期：2026-04-01
> 平台类型：短视频平台

---

## 技术指纹（供 Phase 0.5 自动匹配）

### JS 特征
- [x] `window['_$webrt_1668687510']` — JSVMP 解释器入口函数，接受十六进制字节码字符串
- [x] 单文件 380KB+，包含 `'HNOJ@?RC'` 魔数校验
- [x] `_0x` 前缀变量大量出现（构造函数、工厂函数、闭包变量）
- [x] UMD 模式导出到 `window.byted_acrawler`（浏览器）或 `module.exports`（Node.js）
- [x] 存在 while-switch 风格的 JSVMP 解释器循环（`_0x1218ef` / `_0x5f1fc4` 函数）
- [x] 内部状态对象含 `bogusIndex`、`msNewTokenList`、`moveList`、`clickList`、`activeState`
- [x] 三层 SDK 脚本联动：`sdk-glue.js`（100KB）→ `bdms.js`（147KB）→ `webmssdk.es5.js`（387KB）

### 参数特征
- [x] `a_bogus`：约 180~192 字符，Base64 变体编码（含 `+`、`/`、`=`）
- [x] `X-Bogus`（旧版）：16 字符，字母数字（由 `frontierSign()` 生成，非 JSVMP 拦截器）
- [x] `msToken`：约 140 字符，Base64 编码，来自目标域名的 Cookie
- [x] `verifyFp` / `fp`：格式 `verify_xxxx_xxxx_xxxx_xxxx_xxxxxxxxxxxx`
- [x] `UIFID`：约 256 字符 hex 编码，浏览器指纹标识

### 请求特征
- [x] 页面 inline script 调用 `window._SdkGlueInit(config, resources)` 完成初始化
- [x] 初始化配置含 `bdms.paths` 数组（正则路径列表，如 `'^/aweme/v1/'`）
- [x] JSVMP 通过修改 `XMLHttpRequest.prototype.open` 实现请求拦截
- [x] 拦截器仅对匹配 `enablePathList` 的 URL 追加 `a_bogus`
- [x] 默认仅拦截 `/web/report`，需通过 `_SdkGlueInit` 注入业务路径
- [x] Cookie 中包含动态字段：`ttwid`、`__ac_nonce`、`__ac_signature`、`s_v_web_id`、`odin_tt`

### 反调试特征
- [x] JSVMP 字节码执行，无法直接设置断点
- [x] debugger 定时器陷阱（可通过 `bypass_debugger_trap` 绕过）
- [x] 环境检测（58 项）：`Function.prototype.toString` 原生性检查、`navigator.webdriver`、`navigator.plugins.length`、`document.hasFocus()`、DOM `offsetHeight/Width` 零值检测、`Object.prototype.toString` 标签、`window.chrome` 存在性、`Performance.timing` 对象、Canvas/WebGL 指纹、AudioContext 指纹等
- [x] 非真实浏览器环境生成的签名会被服务端**静默拒绝**（返回 HTTP 200 + 空 body，不报错）

### 混淆类型
- [x] JSVMP（JavaScript Virtual Machine Protection），字节码存储为十六进制字符串，由解释器 `_$webrt_1668687510` 运行时执行

### 指纹检测规则（Agent 执行）

```
快速检测命令（30秒内完成）：
  - search_code(keyword="webmssdk") → 命中 → 直接定位本案例
  - search_code(keyword="byted_acrawler") → 命中 → 直接定位本案例
  - search_code(keyword="_SdkGlueInit") → 命中 → 直接定位本案例
  - search_code(keyword="a_bogus") → 命中 + 参数长度 180-192 → 高置信度
  - list_scripts → 存在 380KB+ 文件 → 辅助确认
  匹配判定：以上前 3 项命中任意 1 项 → 高置信度匹配
```

---

## 加密方案

- **算法**：JSVMP 自定义字节码虚拟机，内部算法不可直接读取。签名结果为 `a_bogus`（~192 字符 Base64 变体）或 `X-Bogus`（16 字符）
- **密钥来源**：动态计算 — 由浏览器环境指纹（Canvas/WebGL/navigator/screen 等数十项数据）+ 请求 URL query string + User-Agent + 时间戳共同生成
- **加密流程**：
  1. `sdk-glue` 通过 `_SdkGlueInit(config, resources)` 初始化，注入拦截路径列表
  2. `webmssdk` JSVMP 构造函数执行字节码，修改 `XMLHttpRequest.prototype.open`
  3. 当 XHR 请求 URL 匹配 `enablePathList` 中的正则时，拦截器触发
  4. JSVMP 读取完整 URL query string、`document.cookie`、`navigator.*` 属性、`screen.*` 属性等数十项环境指纹
  5. JSVMP 内部通过字节码计算生成 `a_bogus` 值（含环境指纹哈希）
  6. 拦截器将 `&a_bogus=xxx` 追加到 URL 末尾，再调用原始 `open`
- **签名公式**：无法提取（JSVMP 字节码保护，算法封装在虚拟机内部）。采用「喂入-截出」策略：在 jsdom 中执行原始 JSVMP 字节码，通过 XHR Hook 截获最终签名值

---

## 方案方向

jsdom 环境伪装，喂入-截出策略截获 a_bogus。环境值必须与 UA 自洽。

## 🚫 禁动清单（实战踩过的"不要碰"）

> 以下 API / 全局对象 / 原型位置，**未经 `hook_function(mode='trace')` 或 `instrumentation` 证明 JSVMP 真的读了**，严禁在 env-patch 里改动。每一条都源自本案例的真实踩坑——改动后签名正常生成但服务端静默拒绝（HTTP 200 + 空 body）。

### ❌ 禁动对象 1：`win.Error` 构造函数

- **理由**：替换 `win.Error` 会让所有 `instanceof Error` 判断走向不确定，JSVMP 内部多处用 Error 做类型分发，替换会连锁打乱内部状态。**没有任何证据证明 JSVMP 读 Error.stack**（本案例实测）
- **正确做法**：完全不动。如果担心 Error.stack 里的 Node.js 路径泄露，先用 `hook_function(path='Error', mode='trace')` 确认 JSVMP 真的调用了再说

### ❌ 禁动对象 2：任何 `constructor.name` 修改

- **理由**：改 `document.constructor.name = 'HTMLDocument'` 不会让 `document.constructor === window.HTMLDocument` 成立，反而让 `Object.prototype.toString.call(document.constructor)` 返回非标准字符串，得不偿失
- **正确做法**：要让 `Object.prototype.toString.call(document) === '[object HTMLDocument]'`，**只走 `Symbol.toStringTag` 路线**（设到 `HTMLDocument.prototype` 和 `Document.prototype` 上）

### ❌ 禁动对象 3：`Object.prototype` 层的原型链修改

- **理由**：`Object.getPrototypeOf(Object.getPrototypeOf(doc))` 可能越过预期到达 `Object.prototype`，改那里会破坏全局 `Object.prototype.toString`，JSVMP 任何对象的 toString 调用都会跑进你的代码
- **正确做法**：`scanPrototypeChain` 必须有 `if (proto === Object.prototype) break;` 边界

### ❌ 禁动对象 4：`Symbol.toStringTag` 设在实例上

- **理由**：`Object.prototype.toString.call(document)` 的算法是**原型链查找** `Symbol.toStringTag`。设在 `doc` 实例上对 `instanceof` / `constructor.name` 等相关检测不生效
- **正确做法**：统一设到 prototype 上（`HTMLDocument.prototype` / `Document.prototype` / `Screen.prototype` / `Navigator.prototype` / `Performance.prototype`）

### ❌ 禁动对象 5：对 `Function.prototype.toString` 做第二次覆盖

- **理由**：整个 env-patch 只能有**一次** `win.Function.prototype.toString = function() {...}` 赋值。"三层防御"指**单次赋值的函数体内部**有三段检测逻辑（WeakSet + jsdom 源码正则 + 源码 fallback），**不是覆盖三次**。第二次覆盖会让命名函数暴露自身源码给 JSVMP
- **正确做法**：写完第一次赋值后**不要**再写第二次"加固"。`grep -c "win.Function.prototype.toString *=" env-patch.js` 应该 = 1

### 行数自检

| 文件 | 合理范围 | 警报阈值 |
|---|---|---|
| env-patch-firefox.js | ~600-650 行 | > 800 行 |
| env-patch-chrome.js | ~650-700 行 | > 850 行 |

超过警报阈值 → 跑上述 5 条禁动清单的 grep 自检，大概率有一条踩了：

```bash
grep -c "win.Function.prototype.toString *=" env-patch.js  # 必须 = 1
grep -c "win.Error *=" env-patch.js                         # 必须 = 0
grep -c "\.constructor.*name" env-patch.js                  # 必须 = 0
```

## 🌐 UA 分支矩阵（必须按目标站点 UA 选分支，不跨分支混补）

> 环境值必须与 `navigator.userAgent` 自洽。Firefox UA 下补 Chrome 独有 API = 立即暴露。

### Firefox UA 分支（UA 含 `Firefox/XXX`）

**必补**（jsdom 缺失但真实 Firefox 有）：

- ✅ `Notification`（构造函数 + `permission` + 静态 `requestPermission`）
- ✅ `Worker` / `SharedWorker` / `RTCPeerConnection`
- ✅ `AudioContext` / `OfflineAudioContext`（至少 stub `createOscillator` / `createDynamicsCompressor` / `startRendering`）
- ✅ `matchMedia`（返回 `{matches:false, media:query}`）
- ✅ `requestIdleCallback` / `cancelIdleCallback`（`setTimeout` 包装）
- ✅ `indexedDB` / `caches` / `speechSynthesis` / `customElements` / `visualViewport`
- ✅ `isSecureContext = true` / `clientInformation = navigator`
- ✅ `WebSocket`（带静态常量 `CONNECTING / OPEN / CLOSING / CLOSED`）
- ✅ `document.fonts` / `document.timeline`
- ✅ `navigator.permissions.query`（stub） / `navigator.clipboard.readText/writeText`（stub）
- ✅ `IntersectionObserver` / `ResizeObserver` / `PerformanceObserver` / `MessageChannel`（Firefox 有，jsdom 没有）
- ✅ `structuredClone`（Firefox 有，jsdom 没有）

**禁补**（Chrome 独有，Firefox UA 下必须 `undefined`）：

- ❌ `navigator.userAgentData`（Chrome 独有 Client Hints）
- ❌ `navigator.connection`（Chrome 独有 Network Information API）
- ❌ `navigator.getBattery`（Firefox 已从 Web 移除）
- ❌ `window.chrome`（名字都带 chrome）
- ❌ `performance.memory`（Chrome 独有）

### Chrome UA 分支（UA 含 `Chrome/XXX`）

**必补**（与 Firefox 大部分重合，额外加上）：

- ✅ `navigator.userAgentData`（完整 NavigatorUAData，含 `getHighEntropyValues()`）
- ✅ `navigator.connection`（`NetworkInformation` stub）
- ✅ `window.chrome.{app, runtime, csi, loadTimes}`（完整 Chrome 对象）

**禁补**（Firefox 独有）：

- ❌ `navigator.buildID`（Firefox 独有）
- ❌ `InstallTrigger`（Firefox 独有标识符）

### 自检命令（Firefox UA 下）

```bash
grep -c "userAgentData\|navigator\.connection\|getBattery\|window\.chrome\|performance\.memory" env-patch.js
# 必须 = 0（Firefox UA 下）
```

## 各 Phase 加速指引

- **Phase 1**: `search_code(keyword="_SdkGlueInit")` 定位配置；`scripts(action='save')` 保存三件套（webmssdk.es5.js / bdms.js / sdk-glue.js）到 `config/`
- **Phase 2**: 加载顺序 webmssdk → bdms → sdk-glue → `_SdkGlueInit`；签名在 XHR send 阶段追加
- **Phase 3**: 用 Camoufox `compare_env` + `evaluate_js` 分批采集真实环境，与 jsdom 逐项 diff，**确认需要补的环境项范围后再写补丁**
- **Phase 4**: jsdom 配置需 `resources:'usable'`；XHR Hook 必须在 SDK 加载前安装；`scanPrototypeChain` 的 `Object.prototype` 边界不能突破
- **Phase 5**: 连续 ≥5 次请求验证，200 + 空 body = 环境指纹不对不是算法错

## 踩坑记录

- **不要对 Object.prototype / Array.prototype 做 markNative**，会破坏 JSVMP 自身运行
- **SDK 脚本必须保存到本地**（`scripts(action='save')`），不要每次从 CDN 下载
- 签名 192 字符但服务端返回空 body = 环境指纹不对，不是算法错
- **环境补丁逐项添加逐项验证**，不要批量添加后排查
- Firefox 和 Chrome 环境都可以工作，关键是所有值与 UA 自洽
- XHR Hook 顺序：我方 Hook → SDK 加载（保存 Hook 后的引用）→ JSVMP 调用时经过我方 Hook

## 原始定位路径（参考，不要跳过 Phase 直接照做）

> 以下是首次分析时的实际步骤记录，供理解方案演进过程。实战时仍需走 Phase 1-5。

### Phase 1：网络捕获定位接口

```
步骤 1: start_network_capture + list_network_requests → 捕获带 a_bogus 的完整请求 URL
步骤 2: search_code(keyword="bdms") → 找到三个关键脚本的 CDN 地址：
        - webmssdk.es5.js (v1.0.0.20, 387KB) — JSVMP 签名引擎
        - bdms.js (v1.0.1.19, 147KB) — ByteDance Monitoring System
        - sdk-glue.js (v1.0.0.64, 100KB) — SDK 胶水层
```

### Phase 2：初始化链路还原

```
步骤 3: search_code(keyword="enablePathList") → 定位到 sdk-glue 中的路径配置机制
步骤 4: 页面 inline script 中找到 _SdkGlueInit 调用及完整的 paths 配置
步骤 5: search_code(keyword="frontierSign") → 找到 webmssdk 的导出结构：init() / frontierSign() / setConfig()
步骤 6: search_code(keyword="bogusIndex") → 确认 a_bogus 由 JSVMP 的 XHR 拦截器生成（不同于 frontierSign 生成的 X-Bogus）
```

### Phase 3：jsdom 沙箱验证

```
步骤 7: 在 jsdom (runScripts: 'dangerously') 中依次加载三个脚本并调用 _SdkGlueInit，
        成功触发 JSVMP 的 XHR 拦截器生成 192 字符的 a_bogus
步骤 8: 浏览器生成的 a_bogus 用 Node.js axios 请求成功 → 确认非 TLS 问题
步骤 9: jsdom 生成的 a_bogus 被服务端拒绝（返回空 body）→ 确认是环境指纹差异
```

### Phase 4：环境指纹对比（核心突破点）

```
步骤 10: 用 camoufox-reverse MCP 启动反检测浏览器，navigate 到目标页面
步骤 11: 通过 evaluate_js 分批采集浏览器完整环境指纹
         （navigator 24项 / screen 10项 / window 40项 / document 16项 /
          performance 12项 / toString 7项 / Function.toString 6项 /
          DOM布局 6项 / Canvas+WebGL+Audio 各项）
步骤 12: 在 jsdom 中运行完全相同的采集代码
步骤 13: 逐项对比发现 58 项差异，按影响分级修复
```

### Phase 5：环境补丁与验证

```
步骤 14: 编写 patchEnvironment() 函数修复全部 58 项差异
步骤 15: 从 jsdom 内部验证所有检测点通过（win.eval() 内执行 Function.prototype.toString 等）
步骤 16: 连续 5 次请求同一视频 ID，全部返回完整数据（35KB+），确认稳定性
```

---

## 还原代码模板

### 核心函数：markNative — 将 jsdom 函数伪装为浏览器原生函数

```javascript
const _origFnToString = win.Function.prototype.toString;
const nativeFnSet = new WeakSet();

function markNative(fn) {
  if (typeof fn === 'function') {
    nativeFnSet.add(fn);
    try {
      const name = fn.name || '';
      Object.defineProperty(fn, 'toString', {
        value: function () { return `function ${name}() { [native code] }`; },
        writable: true, configurable: true,
      });
    } catch (e) {}
  }
  return fn;
}

// jsdom 内部函数的通用源码特征（不在 WeakSet 中的漏网之鱼）
const jsdomPatterns = [
  /^\s*\w+\s*\([^)]*\)\s*\{[\s\S]*?const\s+esValue\s*=/,
  /^\s*function\s*\([^)]*\)\s*\{[\s\S]*?this\._globalObject/,
  /^\s*\w+\s*\([^)]*\)\s*\{\s*const\s+\w+\s*=\s*this\s*!==/,
];

win.Function.prototype.toString = function () {
  if (nativeFnSet.has(this)) {
    return `function ${this.name || ''}() { [native code] }`;
  }
  let src;
  try { src = _origFnToString.call(this); } catch (e) {
    return 'function () { [native code] }';
  }
  for (const pat of jsdomPatterns) {
    if (pat.test(src)) return `function ${this.name || ''}() { [native code] }`;
  }
  return src;
};
```

### 核心函数：深度扫描原型链标记所有 jsdom 内置方法

```javascript
function scanPrototypeChain(obj, maxDepth) {
  let proto = obj;
  for (let d = 0; d < (maxDepth || 10) && proto; d++) {
    for (const name of Object.getOwnPropertyNames(proto)) {
      try {
        const desc = Object.getOwnPropertyDescriptor(proto, name);
        if (desc && typeof desc.value === 'function') markNative(desc.value);
        if (desc && typeof desc.get === 'function') markNative(desc.get);
        if (desc && typeof desc.set === 'function') markNative(desc.set);
      } catch (e) {}
    }
    proto = Object.getPrototypeOf(proto);
    if (proto === Object.prototype || proto === null) break;
  }
}

// 扫描 50+ 原型链，覆盖 jsdom 所有内置类
const protoTargets = [
  win.Document?.prototype, win.HTMLDocument?.prototype,
  win.Element?.prototype, win.HTMLElement?.prototype,
  win.Node?.prototype, win.EventTarget?.prototype,
  win.XMLHttpRequest?.prototype, win.HTMLCanvasElement?.prototype,
  // ... 扫描 50+ 个 DOM 类的原型链
];
for (const proto of protoTargets) {
  if (proto) scanPrototypeChain(proto, 3);
}
scanPrototypeChain(doc, 5);   // document 实例自身
scanPrototypeChain(nav, 5);   // navigator 实例自身
```

### 核心函数：navigator.plugins 完整模拟

```javascript
const pluginData = [
  { name: 'PDF Viewer', filename: 'internal-pdf-viewer', description: 'Portable Document Format' },
  { name: 'Chrome PDF Viewer', filename: 'internal-pdf-viewer', description: 'Portable Document Format' },
  { name: 'Chromium PDF Viewer', filename: 'internal-pdf-viewer', description: 'Portable Document Format' },
  { name: 'Microsoft Edge PDF Viewer', filename: 'internal-pdf-viewer', description: 'Portable Document Format' },
  { name: 'WebKit built-in PDF', filename: 'internal-pdf-viewer', description: 'Portable Document Format' },
];
const mimeData = [
  { type: 'application/pdf', description: 'Portable Document Format', suffixes: 'pdf' },
  { type: 'text/pdf', description: 'Portable Document Format', suffixes: 'pdf' },
];

// 构建完整的 PluginArray → Plugin → MimeType 对象树
// 包含 item()/namedItem()/refresh() 方法 + Symbol.toStringTag
```

### 核心函数：a_bogus 生成（喂入-截出策略）

```javascript
function generateABogus(fullUrl, cookieStr) {
  const dom = initSdk();  // jsdom + 环境补丁 + SDK 加载（单例）
  const win = dom.window;

  // 写入 Cookie 供 JSVMP 读取
  if (cookieStr) {
    cookieStr.split(';').forEach(c => {
      try { win.document.cookie = c.trim(); } catch (e) {}
    });
  }

  return new Promise((resolve) => {
    win.__capturedUrls = [];
    // 在 jsdom 内发起 XHR，触发 JSVMP 拦截器自动追加 a_bogus
    win.eval(`(function(){
      var x = new XMLHttpRequest();
      x.open('GET','${fullUrl.replace(/'/g, "\\'")}',true);
      x.send();
    })()`);

    setTimeout(() => {
      let aBogus = null;
      for (const url of win.__capturedUrls) {
        const m = url.match(/[&?]a_bogus=([^&]+)/);
        if (m) { aBogus = decodeURIComponent(m[1]); break; }
      }
      resolve(aBogus);
    }, 500);
  });
}
```

---

## 踩坑记录

| # | 坑 | 现象 | 解决方法 |
|---|---|------|---------|
| 1 | Function.prototype.toString 暴露 jsdom | JSVMP 检测到非原生函数，生成无效签名 | 三层防御：WeakSet 精确标记 + jsdom 源码模式正则 + 实例级 Object.defineProperty |
| 2 | navigator.plugins 结构不完整 | 不是简单的 `length=5`，JSVMP 遍历 `item()`/`namedItem()` 并检查 `Symbol.toStringTag` | 构建完整 PluginArray→Plugin→MimeType 对象树 |
| 3 | DOM 布局属性全为 0 | jsdom 无渲染引擎，`offsetHeight`/`Width`/`getBoundingClientRect` 返回 0 | HTMLElement.prototype getter 覆写，解析 style 或返回默认非零值 |
| 4 | Object.prototype.toString 标签错误 | jsdom 的 document 原型链是 `Document` 而非 `HTMLDocument`，screen 是普通对象 | `Symbol.toStringTag` 修复 + 原型链 `constructor.name` 修改 |
| 5 | XHR Hook 顺序导致截获失败 | webmssdk 保存的是 Hook 之前的原始 `open`，我方 Hook 截获不到最终 URL | 必须在 webmssdk 加载前完成 XHR Hook 安装 |
| 6 | 服务端静默拒绝 | 环境不完整时返回 HTTP 200 + 空 body，不返回错误码 | 通过对比 Camoufox 真实环境逐项排查 58 项差异 |
| 7 | ttwid Cookie 无法通过协议获取 | `POST /ttwid/check/` 返回 `status_code: 1002, "check not pass"`；首页 `GET /` 的 `set-cookie` 只返回 `__ac_nonce`，不返回 ttwid | ttwid 是浏览器端 JS 生成后写入的，纯协议无法直接获取。**解决方案**：从 Camoufox 调试浏览器的 Cookie 中导出完整 cookie_str 硬编码到脚本中（含 ttwid、odin_tt 等 httpOnly Cookie）。ttwid 有效期约 1 年，不需要频繁更新 |
| 8 | `JSON.stringify` / `JSON.parse` 的 toString 格式暴露 Node.js 环境 | JSVMP 生成 192 字符 a_bogus 但服务端返回 HTTP 200 + 空 body（2 字节）。签名格式正确但环境指纹哈希不匹配 | V8 原生函数的 `toString()` 返回单行格式 `function stringify() { [native code] }`，而 Firefox 格式是多行 `function stringify() {\n    [native code]\n}`。**必须显式 `markNative(JSON.stringify)` 和 `markNative(JSON.parse)`**，因为它们不在任何 DOM 原型链上，`scanPrototypeChain` 扫描不到 |
| 9 | `doc.createElement` 等 DOM 实例方法未被 markNative | 同上：签名格式正确但服务端静默拒绝 | jsdom 的部分 DOM 方法直接挂在实例上而非原型链上，`scanPrototypeChain(doc, 5)` 扫描不到。**必须在 env-patch 末尾显式标记**：`markNative(doc.createElement)`、`markNative(doc.getElementById)`、`markNative(doc.querySelector)`、`markNative(doc.querySelectorAll)` 等 |
| 10 | jsdomPatterns 正则不完整导致部分 jsdom 函数 toString 泄露 | 同上 | jsdom 内部函数有两个额外特征模式需要加入检测正则：`/tryImplForWrapper/` 和 `/ceReactionsPreSteps/`。漏掉这两个会导致部分 DOM 操作函数的 `toString()` 暴露完整 jsdom 源码 |
| 11 | `resources: 'usable'` 是 JSVMP 拦截器激活的必要条件 | 不加 `resources: 'usable'` 时 SDK 加载成功、`byted_acrawler.frontierSign()` 能返回 X-Bogus，但 JSVMP 的 XHR 拦截器不触发（a_bogus 不生成） | jsdom 的 `resources: 'usable'` 选项会启用完整的 XHR 实现（含网络层），JSVMP 拦截器依赖这个完整实现才能正确 Hook `XMLHttpRequest.prototype.open`。不加此选项时 XHR 是简化版，拦截器无法正常工作。副作用是会尝试加载外部脚本导致 `SyntaxError: Unexpected token '<'`，但不影响 SDK 功能 |
| 12 | `navigator.permissions` 和 `navigator.clipboard` 缺失 | 环境指纹哈希不匹配 | Firefox 浏览器有这两个 API，jsdom 没有。需要补充存根：`nav.permissions = { query: markNative(function query() { return Promise.resolve({ state: 'prompt' }); }) }` 和 `nav.clipboard = { readText/writeText 存根 }` |

### 踩坑记录（2026-04-19 抖音 a_bogus 二次复用补充）

| # | 坑 | 现象 | 解决方法 |
|---|---|---|---|
| 13 | `Function.prototype.toString` 被覆盖两次（"三层防御"被误解为覆盖三段代码）| 签名 192 字符但服务端空 body | 整个 patch 有且仅有一次 `win.Function.prototype.toString = function() {...}` 赋值。三层防御指函数体内部的三段检测逻辑（WeakSet / jsdom 源码正则 / 源码 fallback），不是覆盖三次 |
| 14 | `Symbol.toStringTag` 设在 `doc` 实例上而非 `HTMLDocument.prototype` | 签名 192 字符但服务端空 body | 设到 `win.HTMLDocument.prototype` 和 `win.Document.prototype`。不设实例、不 walk 到 `Object.prototype` |
| 15 | 替换 `win.Error` 构造函数（为"清洗 stack 中 Node.js 路径"）| 签名 192 字符但服务端空 body | 完全不动 Error。没有证据证明 JSVMP 读 stack，先证再动手 |
| 16 | 修改 `document.constructor.name = 'HTMLDocument'` | 签名 192 字符但服务端空 body | 不动任何 `constructor.name`。用 `Symbol.toStringTag` 路线 |
| 17 | Firefox UA 下补 Chrome 独有 API（`userAgentData / connection / getBattery / window.chrome / performance.memory`）| 签名 192 字符但服务端空 body | UA 与 API 矩阵必须自洽，见"UA 分支矩阵"段 |
| 18 | env-patch 代码量爆炸（612 行 → 1030 行）| 签名生成正常但服务端静默拒绝 | 严守行数阈值，每加代码走"先证明需要 → 加 → 验证 → 保留；否则回退"循环，禁止"先加上保险" |

---

## 变体说明

| 变体 | 差异点 | 影响 |
|------|--------|------|
| X-Bogus（旧版） | 由 `frontierSign()` 生成的 16 字符签名，非 JSVMP 拦截器 | 较简单，可通过 Hook `frontierSign` 入参直接还原 |
| 不同业务线 | `enablePathList` 配置不同，不同业务接口使用不同路径正则 | 需从 `_SdkGlueInit` 调用中提取完整 paths 配置 |
| SDK 版本迭代 | `webmssdk` 版本号更新，字节码变化 | 环境检测项可能增加，需重新对比 `compare_env` |

---

## 浏览器指纹采集方法

使用 camoufox-reverse MCP 在真实浏览器中采集指纹的完整流程：

```
步骤 1: launch_browser({headless: false, os_type: "macos", locale: "zh-CN"})
步骤 2: navigate({url: "目标页面", wait_until: "domcontentloaded"})
步骤 3: 分批采集环境指纹（通过 evaluate_js 在页面上下文中执行）
```

### 批次 A: navigator 属性

```javascript
JSON.stringify({
  nav_userAgent: navigator.userAgent,
  nav_platform: navigator.platform,
  nav_language: navigator.language,
  nav_languages: navigator.languages,
  nav_cookieEnabled: navigator.cookieEnabled,
  nav_doNotTrack: navigator.doNotTrack,
  nav_onLine: navigator.onLine,
  nav_hardwareConcurrency: navigator.hardwareConcurrency,
  nav_maxTouchPoints: navigator.maxTouchPoints,
  nav_webdriver: navigator.webdriver,
  nav_vendor: navigator.vendor,
  nav_productSub: navigator.productSub,
  nav_appVersion: navigator.appVersion,
  nav_deviceMemory: navigator.deviceMemory,
  nav_plugins_length: navigator.plugins ? navigator.plugins.length : -1,
  nav_mimeTypes_length: navigator.mimeTypes ? navigator.mimeTypes.length : -1,
  nav_pdfViewerEnabled: navigator.pdfViewerEnabled,
  nav_connection_type: typeof navigator.connection,
  nav_getBattery_type: typeof navigator.getBattery,
  nav_userAgentData_type: typeof navigator.userAgentData
})
```

### 批次 B: screen + window 属性

```javascript
JSON.stringify({
  scr_width: screen.width, scr_height: screen.height,
  scr_availWidth: screen.availWidth, scr_availHeight: screen.availHeight,
  scr_colorDepth: screen.colorDepth, scr_pixelDepth: screen.pixelDepth,
  scr_availLeft: screen.availLeft, scr_availTop: screen.availTop,
  scr_orientation_type: screen.orientation ? screen.orientation.type : undefined,
  win_innerWidth: window.innerWidth, win_innerHeight: window.innerHeight,
  win_outerWidth: window.outerWidth, win_outerHeight: window.outerHeight,
  win_devicePixelRatio: window.devicePixelRatio,
  win_chrome: typeof window.chrome,
  win_Notification: typeof window.Notification,
  win_SharedWorker: typeof window.SharedWorker,
  win_WebSocket: typeof window.WebSocket,
  win_RTCPeerConnection: typeof window.RTCPeerConnection,
  win_AudioContext: typeof window.AudioContext,
  win_OfflineAudioContext: typeof window.OfflineAudioContext,
  win_fetch: typeof window.fetch,
  win_Worker: typeof window.Worker,
  win_isSecureContext: window.isSecureContext,
  win_matchMedia: typeof window.matchMedia,
  win_indexedDB: typeof window.indexedDB,
  win_visualViewport: typeof window.visualViewport,
  win_clientInformation: typeof window.clientInformation
})
```

### 批次 C: document + performance + toString + Function.toString

```javascript
JSON.stringify({
  doc_hidden: document.hidden,
  doc_visibilityState: document.visibilityState,
  doc_hasFocus: document.hasFocus(),
  doc_characterSet: document.characterSet,
  doc_compatMode: document.compatMode,
  doc_readyState: document.readyState,
  doc_fullscreenEnabled: document.fullscreenEnabled,
  doc_fonts_type: typeof document.fonts,
  doc_timeline_type: typeof document.timeline,
  perf_timing_type: typeof performance.timing,
  perf_navigation_type: typeof performance.navigation,
  perf_memory_type: typeof performance.memory,
  toString_nav: Object.prototype.toString.call(navigator),
  toString_doc: Object.prototype.toString.call(document),
  toString_win: Object.prototype.toString.call(window),
  toString_scr: Object.prototype.toString.call(screen),
  toString_perf: Object.prototype.toString.call(performance),
  fn_eval: eval.toString().substring(0, 60),
  fn_setTimeout: setTimeout.toString().substring(0, 60),
  fn_createElement: document.createElement.toString().substring(0, 80),
  FnProto_toString: Function.prototype.toString.call(Function.prototype.toString).substring(0, 80),
  nav_proto: Object.getPrototypeOf(navigator).constructor.name,
  doc_proto: Object.getPrototypeOf(document).constructor.name,
  nav_toStringTag: navigator[Symbol.toStringTag],
  doc_toStringTag: document[Symbol.toStringTag]
})
```

### 批次 D: DOM 布局 + Canvas + WebGL + Audio

```javascript
JSON.stringify({
  dom_test: (function(){
    var d = document.createElement('div');
    d.style.cssText = 'width:100px;height:100px;position:absolute;left:-9999px';
    document.body.appendChild(d);
    var r = {offsetHeight: d.offsetHeight, offsetWidth: d.offsetWidth,
             clientHeight: d.clientHeight, clientWidth: d.clientWidth};
    var rect = d.getBoundingClientRect();
    r.rect_width = rect.width; r.rect_height = rect.height;
    document.body.removeChild(d);
    return r;
  })(),
  canvas_test: (function(){
    var c = document.createElement('canvas'); c.width = 200; c.height = 50;
    var ctx = c.getContext('2d');
    ctx.textBaseline = 'top'; ctx.font = '14px Arial';
    ctx.fillStyle = '#f60'; ctx.fillRect(125, 1, 62, 20);
    ctx.fillStyle = '#069'; ctx.fillText('abcdef', 2, 15);
    return {dataURL_len: c.toDataURL().length};
  })(),
  webgl_test: (function(){
    try {
      var c = document.createElement('canvas');
      var gl = c.getContext('webgl');
      var ext = gl.getExtension('WEBGL_debug_renderer_info');
      return {
        vendor: gl.getParameter(gl.VENDOR),
        renderer: gl.getParameter(gl.RENDERER),
        unmasked_vendor: ext ? gl.getParameter(ext.UNMASKED_VENDOR_WEBGL) : 'N/A',
        unmasked_renderer: ext ? gl.getParameter(ext.UNMASKED_RENDERER_WEBGL) : 'N/A'
      };
    } catch(e) { return {error: e.message}; }
  })(),
  audio_test: (function(){
    try { var ctx = new OfflineAudioContext(1,44100,44100);
      return {sampleRate: ctx.sampleRate, available: true};
    } catch(e) { return {error: e.message}; }
  })()
})
```

---

## 环境检测差异全表（58 项）

### 致命级（缺失即被拒）

| 检测点 | 真实浏览器值 | jsdom 原始值 | 修复方式 |
|--------|-------------|-------------|---------|
| `Function.prototype.toString` | `function X() { [native code] }` | 暴露实际实现代码（如 `createElement(localName) { const esValue = ...`） | WeakSet 精确标记 + jsdom 源码模式正则 + 实例级 `Object.defineProperty(fn, 'toString')` |
| `navigator.webdriver` | `false` | `undefined` | `Object.defineProperty` 强制设为 `false` |
| `navigator.plugins.length` | `5` | `0` | 模拟 Chrome 5 个 PDF 插件（含完整 PluginArray/Plugin/MimeType 结构） |
| `document.hasFocus()` | `true` | `false` | 方法覆写，始终返回 `true` |
| DOM `offsetHeight/Width` | 非零值（如 100） | `0` | `HTMLElement.prototype` getter 覆写，解析 style 或返回默认值 |
| `JSON.stringify.toString()` | `function stringify() {\n    [native code]\n}` | `function stringify() { [native code] }`（V8 单行格式） | 显式 `markNative(JSON.stringify)` 覆写 toString 为 Firefox 多行格式 |
| `JSON.parse.toString()` | `function parse() {\n    [native code]\n}` | `function parse() { [native code] }`（V8 单行格式） | 显式 `markNative(JSON.parse)` |
| `navigator.permissions` | `Permissions` 对象 | `undefined` | 补充 `{ query: () => Promise.resolve({state:'prompt'}) }` 存根 |
| `navigator.clipboard` | `Clipboard` 对象 | `undefined` | 补充 `{ readText/writeText }` 存根 |

### 高危级（可能参与指纹哈希）

| 检测点 | 真实浏览器值 | jsdom 原始值 | 修复方式 |
|--------|-------------|-------------|---------|
| `Object.prototype.toString(document)` | `[object HTMLDocument]` | `[object Document]` | `Symbol.toStringTag` 修复 + 原型链 `constructor.name` 修改 |
| `Object.prototype.toString(screen)` | `[object Screen]` | `[object Object]` | 重建 screen 对象并设置 `Symbol.toStringTag` |
| `window.chrome` | `{app:{...}, runtime:{...}, csi, loadTimes}` | `undefined` | 完整 Chrome 对象模拟（含 `runtime.OnInstalledReason` 等枚举） |
| `navigator.userAgentData` | `NavigatorUAData` 对象 | `undefined` | 模拟 Chrome brands/mobile/platform + `getHighEntropyValues()` |
| `navigator.platform` | `MacIntel` | `""` | `Object.defineProperty` 覆盖 |
| `navigator.language` | `zh-CN` | `en-US` | `Object.defineProperty` 覆盖 |
| `navigator.languages` | `["zh-CN", "zh"]` | `["en-US", "en"]` | `Object.defineProperty` 覆盖 |
| `performance.timing` | 完整 `PerformanceTiming` 对象（20+ 字段） | `undefined` | 构造合理的 timing 对象（时间戳递增） |
| `performance.navigation` | `PerformanceNavigation` 对象 | `undefined` | 构造 `{type:0, redirectCount:0}` 对象 |
| `document.readyState` | `complete` | `loading` | getter 覆写返回 `'complete'` |
| `screen.colorDepth/pixelDepth` | `30` | `24` | 重建 screen 对象 |
| `screen.orientation` | `{type: 'landscape-primary', angle: 0}` | `undefined` | 重建 screen 对象含 orientation |

### 中危级（API 存在性检测）

| 检测点 | jsdom 原始值 | 修复方式 |
|--------|-------------|---------|
| `window.Notification` | `undefined` | 构造函数存根 + `permission`/`requestPermission` |
| `window.Worker` / `SharedWorker` | `undefined` | 构造函数存根 |
| `window.RTCPeerConnection` | `undefined` | 构造函数存根 |
| `window.AudioContext` / `OfflineAudioContext` | `undefined` | 完整 Web Audio API 存根（含 `createOscillator`/`createDynamicsCompressor`/`startRendering`） |
| `window.fetch` | `undefined` | 函数存根 + `Request`/`Response`/`Headers`/`AbortController` |
| `window.matchMedia` | `undefined` | 返回 `{matches:false, media:query}` 存根 |
| `window.requestIdleCallback` | `undefined` | `setTimeout` 包装 |
| `window.indexedDB` | `undefined` | `IDBFactory` 存根 |
| `window.caches` | `undefined` | `CacheStorage` 存根 |
| `window.speechSynthesis` | `undefined` | `SpeechSynthesis` 存根 |
| `window.customElements` | `undefined` | `CustomElementRegistry` 存根 |
| `window.visualViewport` | `undefined` | `VisualViewport` 存根 |
| `window.isSecureContext` | `undefined` | 设为 `true` |
| `window.clientInformation` | `undefined` | 指向 `navigator` |
| `navigator.connection` | `undefined` | `NetworkInformation` 存根 |
| `navigator.getBattery` | `undefined` | 返回 `BatteryManager` Promise |
| `navigator.deviceMemory` | `undefined` | 设为 `8` |
| `navigator.maxTouchPoints` | `undefined` | 设为 `0` |
| `navigator.pdfViewerEnabled` | `undefined` | 设为 `true` |
| `navigator.mimeTypes.length` | `0` | 模拟 2 个 MIME 类型 |
| `document.fonts` | `undefined` | `FontFaceSet` 存根 |
| `document.timeline` | `undefined` | `DocumentTimeline` 存根 |
| `document.fullscreenEnabled` | `undefined` | 设为 `true` |

---

## 关键经验总结

### 1. 为什么不反编译 JSVMP 字节码

JSVMP 用自定义指令集执行，反编译成本极高且版本迭代频繁。「喂入-截出」策略（在模拟环境中运行原始字节码，只截获 I/O）是性价比最高的方案。

### 2. jsdom 环境伪装的核心难度排序

| 排名 | 难度 | 检测项 | 为什么难 |
|------|------|--------|---------|
| 1 | ★★★★★ | `Function.prototype.toString` | jsdom 所有 DOM 方法都是 JS 实现，`toString()` 会暴露完整源码。需要三层防御：WeakSet 精确匹配 + jsdom 源码正则模式 + 实例级 toString 覆写 |
| 2 | ★★★★ | `navigator.plugins` 结构 | 不是简单的 `length=5`，JSVMP 可能遍历 `item()`/`namedItem()` 并检查 `Symbol.toStringTag` |
| 3 | ★★★ | DOM 布局属性 | jsdom 无渲染引擎，所有 `offsetHeight`/`Width`/`getBoundingClientRect` 返回 0 |
| 4 | ★★★ | `Object.prototype.toString` 标签 | jsdom 的 document 原型链是 `Document` 而非 `HTMLDocument`，screen 是普通对象 |
| 5 | ★★ | 30+ 缺失 API | 需要逐个补全，且每个都要 `markNative` 处理 toString |

### 3. XHR Hook 顺序至关重要

```
我方 Hook → webmssdk 加载（保存我方 Hook 后的 open 引用）→ JSVMP 执行时调用保存的 open
                                                              ↓
                                               我方 Hook 拦截到带 a_bogus 的 URL
```

如果 webmssdk 先加载再 Hook，JSVMP 保存的是原始 `open`，我方 Hook 无法截获最终 URL。

### 4. 对比方法论

用 Camoufox 反检测浏览器（C++ 引擎级指纹伪装）作为「干净基准」，在目标页面上下文中执行环境采集代码，与 jsdom 输出逐项 diff。分批采集（navigator / screen+window / document+performance+toString / DOM+Canvas+WebGL+Audio）避免单次执行过长。

### 5. env-patch 的「显式 markNative 清单」是必须的最后一步

`scanPrototypeChain` 只能扫描 DOM 类原型链上的方法，以下三类函数必须在 env-patch 末尾**显式逐个 markNative**：

- **V8 全局原生函数**：`JSON.stringify`、`JSON.parse`、`parseInt`、`parseFloat`、`encodeURIComponent`、`decodeURIComponent`、`encodeURI`、`decodeURI`、`btoa`、`atob`
- **jsdom window 上的定时器**：`setTimeout`、`setInterval`、`clearTimeout`、`clearInterval`、`eval`
- **document 实例方法**：`createElement`、`createTextNode`、`createDocumentFragment`、`getElementById`、`getElementsByClassName`、`getElementsByTagName`、`querySelector`、`querySelectorAll`

遗漏任何一个都可能导致 JSVMP 检测到非原生环境，生成的 a_bogus 被服务端静默拒绝（HTTP 200 + 空 body）。

### 6. env-patch 最小化原则（2026-04-19 实测补充）

jsdom 环境伪装的 env-patch **行数越多越不稳**。每一行"保险代码"都可能引入新的泄露点。

**加代码的唯一合法循环**：

1. 用 `hook_function(mode='trace')` 或 `instrumentation(action='install')` 证明 JSVMP 确实读了某个 API / 属性
2. 用 `compare_env` 确认 jsdom 中该 API 的值与真实浏览器不同
3. 加最小化 stub（只 stub 真正被读的属性/方法）
4. 跑真实请求验证成功
5. 保留；否则立即回退

**禁止**："先加上保险"式编码。没有证据的补丁 = 未知泄露点。
