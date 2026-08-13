# 路径 B：环境伪装 — jsdom/vm 沙箱六步法完整方法论

> **触发条件**：Phase 2 选择路径 B（jsdom/sdenv/vm 沙箱环境伪装）时读此文档
>
> **前置要求**：已完成反爬类型三分法识别 + 路径选择决策树（见 SKILL.md）
>
> **版本**：v3.1.0（MCP v0.9.0 工具名迁移）

---

## 一、核心思想

不分析 JSVMP 内部算法，而是在 jsdom 中完整运行原始 JSVMP 字节码，通过「采集 → 对比 → 补丁」使 JSVMP 认为自己运行在真实浏览器中，直接截获生成的签名值。

### 适用条件

- JSVMP 与浏览器环境深度绑定（环境指纹参与签名哈希）
- JSVMP 劫持了请求链路（XHR/fetch 拦截器），签名在内部完成
- 算法不可直接提取（字节码保护 + 自定义算法）
- jsdom 可以成功加载并执行 JSVMP 脚本

### 典型用时

2-4 小时（对比路径 A 的 4-8 小时）

---

## 二、步骤 0.5：确认签名函数入口（v2.7.0 新增）

> ⚠️ 跳过此步骤可能导致：只 Hook 了 XHR 漏掉 fetch 通道的签名、未传 cacheOpts 导致拦截器不触发、误以为签名函数不存在而走了更复杂的路径。

### MCP 操作

```
search_code(keyword="frontierSign", script_url='<VMP脚本URL>')
<!-- v3.1.0: migrated from search_code_in_script -->
→ 是否有导出签名函数

search_code(keyword="XMLHttpRequest.prototype.open", script_url='<VMP脚本URL>')
<!-- v3.1.0: migrated from search_code_in_script -->
→ 是否劫持 XHR

search_code(keyword="window.fetch", script_url='<VMP脚本URL>')
<!-- v3.1.0: migrated from search_code_in_script -->
→ 是否劫持 fetch

search_code(keyword="cacheOpts", script_url='<VMP脚本URL>')
<!-- v3.1.0: migrated from search_code_in_script -->
→ 是否使用新版初始化
```

### 判定结果与后续路径

| 判定 | 签名入口类型 | 后续策略 |
|------|-------------|----------|
| 仅 XHR 拦截器 | 单通道截获 | Hook XHR.open 即可 |
| XHR + fetch 双通道拦截 | 双通道截获 | 需同时 Hook 两个通道 |
| 导出函数 + 拦截器 | 优先直接调用 | 优先调用导出函数，拦截器作为备选 |
| 含 cacheOpts 初始化 | 需配置初始化 | 必须传入 cacheOpts 配置才能注册业务路径 |

---

## 三、步骤 1：Camoufox 环境指纹采集

### 目标

用 Camoufox 反检测浏览器采集真实浏览器的完整环境指纹，作为 jsdom 补丁的基准。

### MCP 操作

```
launch_browser({headless: false, os_type: "macos", locale: "zh-CN"})
navigate({url: "目标页面", wait_until: "domcontentloaded"})
compare_env()
→ 采集主流环境基准数据（navigator/screen/canvas/WebGL/Audio/timing）
```

### 分批采集细粒度环境值（4-5 批次）

> ⚠️ 单次 evaluate_js 代码太长会报错，必须分批采集

#### 批次 A：navigator 属性（24 项）

```
evaluate_js(`JSON.stringify({
  userAgent: navigator.userAgent,
  platform: navigator.platform,
  language: navigator.language,
  languages: navigator.languages,
  cookieEnabled: navigator.cookieEnabled,
  doNotTrack: navigator.doNotTrack,
  hardwareConcurrency: navigator.hardwareConcurrency,
  maxTouchPoints: navigator.maxTouchPoints,
  vendor: navigator.vendor,
  vendorSub: navigator.vendorSub,
  productSub: navigator.productSub,
  appVersion: navigator.appVersion,
  appName: navigator.appName,
  appCodeName: navigator.appCodeName,
  onLine: navigator.onLine,
  webdriver: navigator.webdriver,
  pdfViewerEnabled: navigator.pdfViewerEnabled,
  deviceMemory: navigator.deviceMemory,
  plugins_length: navigator.plugins.length,
  mimeTypes_length: navigator.mimeTypes.length,
  connection: navigator.connection ? {
    effectiveType: navigator.connection.effectiveType,
    downlink: navigator.connection.downlink,
    rtt: navigator.connection.rtt
  } : null,
  userAgentData: navigator.userAgentData ? {
    brands: navigator.userAgentData.brands,
    mobile: navigator.userAgentData.mobile,
    platform: navigator.userAgentData.platform
  } : null,
  getBattery: typeof navigator.getBattery,
  mediaDevices: typeof navigator.mediaDevices
})`)
```

#### 批次 B：screen + window 属性（25 项）

```
evaluate_js(`JSON.stringify({
  screen_width: screen.width,
  screen_height: screen.height,
  screen_availWidth: screen.availWidth,
  screen_availHeight: screen.availHeight,
  screen_colorDepth: screen.colorDepth,
  screen_pixelDepth: screen.pixelDepth,
  screen_orientation: screen.orientation ? screen.orientation.type : null,
  window_innerWidth: window.innerWidth,
  window_innerHeight: window.innerHeight,
  window_outerWidth: window.outerWidth,
  window_outerHeight: window.outerHeight,
  window_devicePixelRatio: window.devicePixelRatio,
  window_screenX: window.screenX,
  window_screenY: window.screenY,
  chrome_exists: typeof window.chrome,
  chrome_runtime: window.chrome ? typeof window.chrome.runtime : null,
  Notification: typeof window.Notification,
  Worker: typeof window.Worker,
  SharedWorker: typeof window.SharedWorker,
  RTCPeerConnection: typeof window.RTCPeerConnection,
  AudioContext: typeof window.AudioContext,
  OfflineAudioContext: typeof window.OfflineAudioContext,
  fetch_exists: typeof window.fetch,
  matchMedia: typeof window.matchMedia,
  indexedDB: typeof window.indexedDB
})`)
```

#### 批次 C：document + performance + toString + Function.toString（28 项）

```
evaluate_js(`JSON.stringify({
  doc_hasFocus: document.hasFocus(),
  doc_readyState: document.readyState,
  doc_visibilityState: document.visibilityState,
  doc_hidden: document.hidden,
  doc_characterSet: document.characterSet,
  doc_compatMode: document.compatMode,
  doc_contentType: document.contentType,
  doc_designMode: document.designMode,
  doc_referrer: document.referrer,
  doc_title: document.title,
  doc_domain: document.domain,
  doc_cookie_type: typeof document.cookie,
  perf_timing: typeof performance.timing,
  perf_navigation: typeof performance.navigation,
  perf_now: typeof performance.now,
  perf_memory: typeof performance.memory,
  fn_toString_createElement: document.createElement.toString(),
  fn_toString_getElementById: document.getElementById.toString(),
  fn_toString_querySelector: document.querySelector.toString(),
  fn_toString_addEventListener: EventTarget.prototype.addEventListener.toString(),
  fn_toString_fetch: typeof window.fetch === 'function' ? window.fetch.toString() : null,
  fn_toString_setTimeout: window.setTimeout.toString(),
  obj_toString_document: Object.prototype.toString.call(document),
  obj_toString_window: Object.prototype.toString.call(window),
  obj_toString_navigator: Object.prototype.toString.call(navigator),
  obj_toString_screen: Object.prototype.toString.call(screen),
  symbol_toStringTag_doc: document[Symbol.toStringTag],
  symbol_toStringTag_screen: screen[Symbol.toStringTag]
})`)
```

#### 批次 D：DOM 布局 + Canvas + WebGL + Audio（指纹检测类）

```
evaluate_js(`(function(){
  var d = document.createElement('div');
  d.style.cssText = 'width:100px;height:100px;position:absolute;left:-9999px';
  document.body.appendChild(d);
  var layout = {
    offsetHeight: d.offsetHeight,
    offsetWidth: d.offsetWidth,
    clientHeight: d.clientHeight,
    clientWidth: d.clientWidth,
    rect: d.getBoundingClientRect()
  };
  document.body.removeChild(d);

  var canvas = document.createElement('canvas');
  var ctx = canvas.getContext('2d');
  ctx.textBaseline = 'top';
  ctx.font = '14px Arial';
  ctx.fillText('test', 2, 2);
  var canvasData = canvas.toDataURL();

  var gl = document.createElement('canvas').getContext('webgl');
  var webglInfo = gl ? {
    vendor: gl.getParameter(gl.VENDOR),
    renderer: gl.getParameter(gl.RENDERER),
    version: gl.getParameter(gl.VERSION)
  } : null;

  return JSON.stringify({
    layout: layout,
    canvas_hash: canvasData.length,
    webgl: webglInfo,
    audioContext: typeof AudioContext,
    offlineAudioContext: typeof OfflineAudioContext
  });
})()`)
```

### 注意事项

- Camoufox 的 C++ 引擎级指纹伪装提供「干净基准」，比普通 Chrome 更可靠
- `compare_env` 不覆盖 Function.prototype.toString / Symbol.toStringTag / DOM 布局等细粒度检测，这些必须通过 evaluate_js 手动采集
- 采集代码模板参考 cases/ 经验案例中的「浏览器指纹采集方法」段

---

## 四、步骤 2：jsdom 环境采集

### 目标

在本地 Node.js 中创建 jsdom 实例，执行与步骤 1 完全相同的采集代码。

### 实现

```javascript
const { JSDOM } = require('jsdom');
const dom = new JSDOM('<!DOCTYPE html><html><body></body></html>', {
  url: '目标URL',
  pretendToBeVisual: true,
  runScripts: 'dangerously'
});
const win = dom.window;

// 在 win.eval() 中运行与步骤 1 完全相同的采集脚本
// 批次 A
const navResult = win.eval(`JSON.stringify({
  userAgent: navigator.userAgent,
  platform: navigator.platform,
  // ... 与步骤 1 批次 A 完全相同
})`);

// 批次 B、C、D 同理
```

### 关键点

- jsdom 实例的 `url` 参数必须设为目标 URL（影响 document.URL / document.domain）
- `pretendToBeVisual: true` 使 `requestAnimationFrame` 可用
- `runScripts: 'dangerously'` 允许执行脚本（环境伪装必需）

---

## 五、步骤 3：Diff 与严重性分级

### 差异分级标准

#### 致命级 — 缺失即被服务端拒绝

| 检测项 | 浏览器值 | jsdom 默认值 | 影响 |
|--------|----------|-------------|------|
| `Function.prototype.toString` | `function createElement() { [native code] }` | 暴露 jsdom 实际 JS 代码 | 直接暴露非浏览器环境 |
| `navigator.plugins.length` | 5（典型值） | 0 | 插件数为 0 = 非浏览器 |
| `navigator.webdriver` | `false` | `undefined` | webdriver 检测 |
| `document.hasFocus()` | `true` | `false` | 焦点检测 |
| DOM `offsetHeight/Width` | 非零值 | 0 | 布局检测 |

#### 高危级 — 可能参与指纹哈希

| 检测项 | 浏览器值 | jsdom 默认值 |
|--------|----------|-------------|
| `Object.prototype.toString.call(document)` | `[object HTMLDocument]` | `[object Document]` |
| `window.chrome` | `{runtime: {...}, ...}` | `undefined` |
| `navigator.userAgentData` | `{brands: [...], ...}` | `undefined` |
| `performance.timing/navigation` | 完整对象 | 缺失 |
| `Symbol.toStringTag` | 正确标签 | 不正确 |

#### 中危级 — API 存在性检测（30+ 缺失 API）

```
window.Notification / Worker / SharedWorker / RTCPeerConnection
window.AudioContext / OfflineAudioContext / fetch
window.matchMedia / indexedDB / caches / visualViewport
navigator.connection / getBattery / deviceMemory
```

### 参考

完整的差异表和修复代码模板见 references/jsdom-env-patches.md。

---

## 六、步骤 4：patchEnvironment() 实现

### 修复优先级排序

按以下优先级逐项修复，每修一项立即验证：

#### ① markNative 三层防御（最高优先级）

```
核心问题：jsdom 所有 DOM 方法的 toString() 会暴露实际 JS 代码
解决方案：WeakSet + 源码正则 + 实例覆写 + 50+ 原型链扫描

三层防御：
  Layer 1: WeakSet 记录所有需要伪装的函数引用
  Layer 2: 源码模式正则匹配（function X() { [native code] }）
  Layer 3: 实例级覆写 toString（兜底）

⚠️ Firefox vs Chrome 格式差异：
  Chrome: "function createElement() { [native code] }"
  Firefox: "function createElement() {\n    [native code]\n}"
  → Camoufox 基于 Firefox 内核，必须使用 Firefox 格式
```

#### ② navigator 补丁

```
修复项：
  - plugins: 完整 PluginArray/Plugin/MimeType 对象树（length=5）
  - webdriver: false
  - userAgentData: {brands: [...], mobile: false, platform: "macOS"}
  - connection: {effectiveType: "4g", downlink: 10, rtt: 50}
  - 每个属性经 markNative 处理
```

#### ③ window 补丁

```
修复项：
  - chrome 对象: {runtime: {}, app: {}, ...}
  - 30+ API 存根: Notification / Worker / SharedWorker / RTCPeerConnection /
    AudioContext / OfflineAudioContext / fetch / matchMedia / indexedDB / caches /
    visualViewport 等
  - 每个存根经 markNative 处理
```

#### ④ document + performance 补丁

```
修复项：
  - hasFocus(): 始终返回 true
  - readyState: "complete"
  - visibilityState: "visible"
  - hidden: false
  - performance.timing: 完整 PerformanceTiming 对象
  - performance.navigation: {type: 0, redirectCount: 0}
```

#### ⑤ DOM 布局属性

```
修复项：
  - offsetHeight/offsetWidth: 返回非零值
  - clientHeight/clientWidth: 返回非零值
  - getBoundingClientRect(): 返回合理的 DOMRect
  - scrollHeight/scrollWidth: 返回非零值
```

#### ⑥ Symbol.toStringTag 全面修复

```
修复项：
  - document → "HTMLDocument"（而非 "Document"）
  - screen → "Screen"
  - navigator → "Navigator"
  - window → "Window"
  - 通过 Object.defineProperty 设置 Symbol.toStringTag
```

### 完整代码模板

参考 references/jsdom-env-patches.md 获取每个修复模块的完整实现代码。

---

## 七、步骤 5：内部验证（win.eval）

### 目标

从 jsdom 内部（win.eval）验证所有检测点通过。

### 关键

验证代码必须在 jsdom 的 window 上下文中执行（win.eval），因为 JSVMP 看到的就是这个上下文。

### 验证代码

```javascript
const result = win.eval(`JSON.stringify({
  fnToString: document.createElement.toString(),
  plugins: navigator.plugins.length,
  webdriver: navigator.webdriver,
  hasFocus: document.hasFocus(),
  chromeExists: typeof window.chrome,
  docTag: Object.prototype.toString.call(document),
  offsetTest: (function(){
    var d = document.createElement('div');
    d.style.cssText = 'width:100px;height:100px';
    document.body.appendChild(d);
    var h = d.offsetHeight;
    document.body.removeChild(d);
    return h;
  })()
})`);

// 预期结果：
// fnToString 包含 [native code]（Firefox 格式含换行）
// plugins = 5
// webdriver = false
// hasFocus = true
// chromeExists = 'object'
// docTag = '[object HTMLDocument]'
// offsetTest > 0
```

### 验证清单

| 检测项 | 预期值 | 不通过时的修复模块 |
|--------|--------|-------------------|
| `document.createElement.toString()` | 含 `[native code]` | markNative 三层防御 |
| `navigator.plugins.length` | 5 | navigator 补丁 |
| `navigator.webdriver` | `false` | navigator 补丁 |
| `document.hasFocus()` | `true` | document 补丁 |
| `typeof window.chrome` | `'object'` | window 补丁 |
| `Object.prototype.toString.call(document)` | `'[object HTMLDocument]'` | Symbol.toStringTag 修复 |
| `div.offsetHeight` | > 0 | DOM 布局属性 |

---

## 八、步骤 6：端到端验证

### 目标

在 jsdom 中加载完整 JSVMP 脚本，触发签名生成，用截获的签名值发起真实接口请求。

### 验证流程

```
1. 在 jsdom 中加载完整 JSVMP 脚本并触发签名生成
2. 用截获的签名值发起真实接口请求
3. 确认返回有效数据（非空 body / 非错误码）
4. 连续多次请求验证稳定性（至少 5 次）
```

### 注意事项

- ⚠️ 服务端可能静默拒绝（返回 HTTP 200 + 空 body，不报错）— 这是环境检测失败的信号
- 连续 5 次请求全部返回有效数据才算通过
- 如果间歇性失败，检查是否有时间戳相关的环境值未补齐

---

## 九、环境伪装还原策略表

| 情况 | 策略 | 实现方式 |
|------|------|---------|
| JSVMP 劫持 XHR，在拦截器中追加签名 | 「喂入-截出」（feed-intercept） | jsdom + XHR Hook → 在 jsdom 内发 XHR，拦截器自动追加签名，Hook 截获 |
| JSVMP 导出签名函数到 window | 直接调用导出函数 | jsdom 加载 JSVMP → `win.签名函数(参数)` → 获取签名 |
| JSVMP + 预热初始化（如 SdkGlueInit） | 完整初始化链路 | jsdom 依次加载所有脚本 → 调用初始化函数 → 再触发签名生成 |
| JSVMP + cacheOpts 配置 | 配置驱动初始化 | 传入 cacheOpts 配置 → 注册业务路径 → 拦截器才会触发 |
| 双签名（URL 参数 + header） | 双通道截获 | 同时 Hook XHR.open（URL 签名）+ XHR.setRequestHeader（header 签名） |

---

## 十、Firefox vs Chrome 变体差异

> Camoufox 基于 Firefox 内核，与 Chrome 在多个环境值上存在差异。jsdom 补丁必须匹配采集基准浏览器的格式。

### 关键差异

| 检测项 | Chrome 格式 | Firefox (Camoufox) 格式 |
|--------|------------|------------------------|
| `Function.prototype.toString` (native) | `"function name() { [native code] }"` | `"function name() {\n    [native code]\n}"` |
| `navigator.vendor` | `"Google Inc."` | `""` |
| `navigator.vendorSub` | `""` | `""` |
| `navigator.productSub` | `"20030107"` | `"20100101"` |
| `window.chrome` | 完整对象 | `undefined`（需补丁） |
| `navigator.userAgentData` | 完整对象 | `undefined`（Firefox 不支持） |

### 修复策略

- 如果目标站点的 JSVMP 是在 Chrome 环境下开发的，可能期望 Chrome 格式
- 但 Camoufox 采集的基准是 Firefox 格式
- **原则**：以 Camoufox 采集的基准为准，因为 Camoufox 的 C++ 引擎级伪装能通过反检测
- 如果服务端明确校验 Chrome 特征（如 `window.chrome` 必须存在），则需要额外补丁

---

## 十一、常见踩坑与解决

### 坑 1：Canvas 存根不够真实

```
问题：jsdom 的 canvas 返回空白数据，指纹哈希不匹配
解决：使用 node-canvas 或 @napi-rs/canvas 提供真实 Canvas 实现
      或者从 Camoufox 采集 canvas 指纹后硬编码返回值
```

### 坑 2：native code 多行格式

```
问题：markNative 输出了 Chrome 单行格式，但 Camoufox 基准是 Firefox 多行格式
解决：将 markNative 的返回值从
      "function name() { [native code] }"
      改为
      "function name() {\n    [native code]\n}"
```

### 坑 3：Symbol.toStringTag 遗漏

```
问题：Object.prototype.toString.call(document) 返回 [object Document] 而非 [object HTMLDocument]
解决：Object.defineProperty(Document.prototype, Symbol.toStringTag, {
        get: () => 'HTMLDocument', configurable: true
      })
```

### 坑 4：jsdom 环境补丁必须在 JSVMP 脚本加载前完成

```
问题：JSVMP 在加载时就读取环境值，补丁打晚了无效
解决：确保 patchEnvironment() 在任何 JSVMP 脚本 eval 之前执行
      XHR Hook 的安装顺序也决定能否截获最终 URL
```

### 坑 5：服务端静默拒绝

```
问题：返回 HTTP 200 + 空 body（不报错），看起来请求成功但没数据
信号：环境检测失败，签名格式正确但环境指纹不匹配
解决：回到步骤 3 重新 diff，检查是否有遗漏的致命级差异
```

### 坑 6：cacheOpts 未传入

```
问题：拦截器不触发，签名不生成
信号：旧版只需 bdms.paths，新版必须同时传入 cacheOpts
解决：检查 _SdkGlueInit 调用是否包含 cacheOpts 字段
      确认 cacheOpts.paths 覆盖目标 API 路径
```

---

## 十二、JSVMP 核心经验（路径 B 专项）

1. **环境伪装优先于算法追踪** — 路径 B 通常比路径 A 快 10 倍
2. **Function.prototype.toString 是 jsdom 环境伪装的第一杀手** — 三层防御必须到位
3. **环境对比要分批采集** — 单次 evaluate_js 代码太长会报错
4. **jsdom 环境补丁必须在 JSVMP 脚本加载前完成** — 加载顺序决定成败
5. **服务端静默拒绝是环境检测失败的信号** — HTTP 200 + 空 body
6. **Firefox 格式 native code 与 Chrome 不同** — 必须匹配采集基准浏览器的格式
7. **cacheOpts 是新版 SDK 初始化的必传项** — 缺少会导致拦截器不触发
8. **环境伪装前先确认签名函数入口** — 步骤 0.5 不可跳过

---

> **环境补丁代码模板**：references/jsdom-env-patches.md
> **经验案例**：cases/jsvmp-xhr-interceptor-env-emulation.md、cases/jsvmp-dual-sign-xhr-intercept-cacheOpts-jsdom-firefox.md
> **完整工作流**：SKILL.md「路径 B：环境伪装」段
