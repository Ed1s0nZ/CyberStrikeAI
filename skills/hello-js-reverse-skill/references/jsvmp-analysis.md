# JSVMP 虚拟机保护分析指南

## 什么是 JSVMP

JSVMP（JavaScript Virtual Machine Protection）是一种高级 JS 代码保护技术，将原始 JS 源码编译为自定义字节码，运行时由内嵌的虚拟机解释器逐条执行。这意味着：

- 原始代码逻辑不再以 JS AST 形式存在，无法通过传统反混淆手段还原
- 代码以字节码数组 + 解释器循环的形式运行
- 常见于RS、JY、某数等商业级反爬方案

**核心原则：不反编译字节码，用行为追踪法（Hook / 插桩 / 日志分析 / 源码级插桩四板斧）从 I/O 两端夹逼 + 中间层观察定位签名逻辑。**

> **v2.5.0 更新**：第四板斧「源码级插桩」由 camoufox-reverse MCP v0.4.0 的 `instrumentation(action='install', ...)` 提供支持，在 HTTP 层改写 VMP 源码，对每个 `obj[key]` 和 `fn(args)` 插入 tap，捕获 VM 内部 switch/case 调度——是RS 5/6、Akamai sensor_data、webmssdk、obfuscator.io 这类"VM 自包含"场景的通用武器。详见 [`jsvmp-source-instrumentation.md`](./jsvmp-source-instrumentation.md)。 <!-- v3.1.0: migrated from instrument_jsvmp_source -->

---

## 识别 JSVMP

### 文件特征

| 特征 | 描述 |
|------|------|
| 文件大小 | 200KB+ 的单文件，通常 500KB~2MB |
| 变量命名 | 完全无意义：单字母（`a`, `b`, `c`）或 `_0x` 前缀 |
| 超大数组 | 包含数千个数字元素的数组（字节码） |
| 解释器循环 | `while(true) { switch(opcode) { case 0: ... case 1: ... } }` |
| 栈操作 | 频繁出现 `push`、`pop`、`shift` 操作 |
| API 劫持 | 重写 `XMLHttpRequest`、`fetch`、`document.cookie` 等原生 API |

### 代码模式示例

```javascript
// 典型的 JSVMP 解释器结构
var _0x1234 = [3, 15, 7, 22, ...]; // 字节码数组
var _0x5678 = [];                   // 操作栈
var _0xabcd = 0;                    // 指令指针

function _0xefgh() {
  while (true) {
    var _0x9999 = _0x1234[_0xabcd++];
    switch (_0x9999) {
      case 0: _0x5678.push(_0x1234[_0xabcd++]); break;  // PUSH
      case 1: var a = _0x5678.pop(), b = _0x5678.pop(); _0x5678.push(a + b); break; // ADD
      case 2: /* ... */ break;
      // 数十到数百个 case
    }
  }
}
```

### 如何确认是 JSVMP

```
MCP 操作（推荐顺序）：
  - scripts(action='list') → 找到异常大的 JS 文件（100KB+） <!-- v3.1.0: migrated from list_scripts -->
  - search_code(keyword='switch', script_url=<大 JS 的 url>, context_chars=500)   ← v0.4.0 推荐首选 <!-- v3.1.0: migrated from find_dispatch_loops -->
    → 返回 candidates: [{fn_name, case_count, char_range, preview}]
    → case_count > 50 的基本确认是 VMP 解释器
  - dump_jsvmp_strings(script_url=<大 JS 的 url>) → 提取字符串数组
    → 看 suspicious_patterns 是否包含 "JSVMP interpreter loop (while+switch, large source)"
    → 看 api_names 是否包含 navigator/screen/encrypt/md5/hmac 等

兜底手动方式（大文件 search_code 超时时）：
  - search_code(keyword="switch", script_url=N) → 前后 3 行上下文 <!-- v3.1.0: migrated from search_code_in_script -->
  - search_code(keyword="case 0:|case 1:|case 2:", script_url=N) <!-- v3.1.0: migrated from search_code_in_script -->
  - scripts(action='get', script_id=N, start_line=..., end_line=...) → 按 char_range 读取分发体 <!-- v3.1.0: migrated from get_script_source -->
```

---

## 四板斧方法论（v2.5.0 更新）

### 四板斧关系图

```
                   ┌── 第一板斧 Hook 出入口 ──────────┐
                   │  inject_hook_preset(xhr|fetch    │
                   │    |crypto|cookie|websocket      │
                   │    |debugger_bypass|runtime_probe)│
                   │  hook_function(..., mode='intercept', ...)  │ <!-- v3.1.0: migrated from freeze_prototype -->
                   │  analyze_cookie_sources          │
                   └────────┬─────────────────────────┘
                            │
              夹逼 I/O ──→ ┌─┴─┐ ──→ 推断签名公式
                            │VMP│
              中间层 ──→  ┌─┘   └─┐ ──→ 追踪执行链路
                         │
  ┌── 第二板斧 插桩解释器 ────────────┐   ┌── 第四板斧 源码级插桩 ──────────┐
  │  search_code(keyword='switch',   │   │  instrumentation(action='install')│ <!-- v3.1.0: migrated from find_dispatch_loops, instrument_jsvmp_source -->
  │    script_url=..., context_chars) │   │  instrumentation(action='log')   │ <!-- v3.1.0: migrated from get_instrumentation_log -->
  │  hook_function(mode='trace',     │   │  → summary.hot_keys             │ <!-- v3.1.0: migrated from trace_function -->
  │    分发函数/子函数)              │   │  → summary.hot_methods          │
  │  hook_jsvmp_interpreter          │   │  → summary.hot_functions        │
  │    (mode='proxy', trackProps=True)│   └────────┬────────────────────────┘ <!-- v3.1.0: migrated from trace_property_access -->
                                          └────────┬────────────────────────┘
                                                   │
                            ↓                      ↓
                   ┌── 第三板斧 日志分析 ──────────────────┐
                   │  get_trace_data + get_jsvmp_log       │
                   │    + get_runtime_probe_log            │
                   │    + instrumentation(action='log')    │ <!-- v3.1.0: migrated from get_instrumentation_log -->
                   │  反向追踪法（从签名值→明文）            │
                   │  多次请求对比法（找变化因子）          │
                   └───────────────────────────────────────┘
```

**板斧选择矩阵**：

| VMP 特征 | 推荐板斧组合 |
|---------|-------------|
| 签名通过 CryptoJS / atob / MD5 等可 hook API 走 | 第一 + 第三（hot API hook + 日志反推） |
| VMP 通过 `Function.apply/call` 调子函数 | 第二 + 第三（`hook_jsvmp_interpreter` 多路径 + 日志） |
| VMP 自包含，算法全在 switch/case 内（RS 5/6、Akamai、webmssdk、obfuscator.io） | **第四板斧首选**，配合第一板斧做 I/O 边界 |
| VMP 深度绑定浏览器环境（compare_env 差异大 + 签名随环境变） | 路径 B 环境伪装（见 SKILL.md 场景 10），配第四板斧 `hot_keys` 定位指纹集 |

---

### 快速路径：v0.4.0 黄金 8 步流程（推荐先试，70%+ RS/Akamai/webmssdk 场景直接搞定）

```
Step 1 — search_code(keyword='switch', script_url=..., context_chars=500) 确认 VMP + 记下 fn_name / char_range <!-- v3.1.0: migrated from find_dispatch_loops -->
Step 2 — hook_jsvmp_interpreter(script_url=<VMP basename>) 装多路径探针
Step 3 — inject_hook_preset("cookie", persistent=True) + inject_hook_preset("xhr", persistent=True)
Step 4 — instrumentation(action='install', url_pattern="**/<VMP 文件>", mode="ast", tag="vmp1")  ← 核心新步 <!-- v3.1.0: migrated from instrument_jsvmp_source -->
Step 5 — instrumentation(action='reload') 让所有探针先于 VMP 生效 + 清日志 <!-- v3.1.0: migrated from reload_with_hooks -->
Step 6 — 触发业务操作
Step 7 — instrumentation(action='log', tag_filter="vmp1", limit=300) 看 hot_keys / hot_methods / hot_functions <!-- v3.1.0: migrated from get_instrumentation_log -->
Step 8 — analyze_cookie_sources() 归因最终 cookie 来源
```

详细步骤见 [`jsvmp-source-instrumentation.md`](./jsvmp-source-instrumentation.md)「黄金 8 步流程」节。

---

### 三板斧旧流程（保留以应对第四板斧无法使用的场景）

对于典型 JSVMP，也可以继续用以下一键工具：

```
MCP 操作：
  - hook_jsvmp_interpreter → 自动 Hook Function.prototype.apply + 追踪 30+ 敏感属性读取
  - dump_jsvmp_strings → 提取字符串数组，识别 API 名称，检测混淆模式
  - 触发目标操作（翻页、提交等）
  - get_jsvmp_log → 获取结构化分析结果：
    · API 调用统计（哪些原生函数被 VM 调用）
    · 属性读取摘要（哪些环境属性被访问）
    · 调用时序（帮助定位签名生成节点）
  - compare_env → 收集完整浏览器环境（navigator/screen/canvas/WebGL/Audio/timing）
    · 用于后续 Node.js/Python 补环境时对照

快速路径无法解决时，再进入手动三板斧流程 ↓
```

### 第一板斧：Hook 出入口（确定 I/O 边界）

**目标**：不看 VM 内部实现，只关心"什么进去了"和"什么出来了"。

#### 1.1 Hook 出口 — 签名值去了哪里

```
MCP 操作：
  - inject_hook_preset(preset="xhr", persistent=True)   → 持久化拦截 XHR（跨导航不丢失）
  - inject_hook_preset(preset="fetch", persistent=True) → 持久化拦截 Fetch
  - hook_function(
      function_path="Document.prototype.cookie",
      hook_code="console.log('[COOKIE_SET]', arguments[0], new Error().stack)",
      position="before", non_overridable=True
    ) → 拦截 Cookie 写入（防覆盖）
  - hook_function(className="XMLHttpRequest", methodName="send", mode='intercept') <!-- v3.1.0: migrated from freeze_prototype -->
    → 冻结 XHR.send 防止 VM 覆盖 Hook
  - reload() → 刷新页面触发 Hook
  - get_console_logs → 查看捕获的签名值和调用栈
```

**关键信息提取**：
- 签名参数名（如 `sign`、`m`、`_signature`）
- 签名值的格式（长度、字符集、编码方式）
- 签名值出现在请求的哪个位置（URL params / Body / Header / Cookie）

#### 1.2 Hook 入口 — 明文从哪里来

```
MCP 操作：
  - inject_hook_preset(preset="crypto", persistent=True)
    → 持久化 Hook btoa/atob/JSON.stringify/JSON.parse

  - hook_function(function_path="String.fromCharCode",
      hook_code="if(arguments.length > 1) console.log('[fromCharCode]', Array.from(arguments).map(Number), String.fromCharCode(...arguments))",
      position="before")
    → JSVMP 高频使用 fromCharCode 构造字符串

  - add_init_script(script=`
      // Hook 常见加密库入口
      const _origMD5 = window.CryptoJS?.MD5;
      if (_origMD5) {
        window.CryptoJS.MD5 = function() {
          console.log('[CryptoJS.MD5] input:', arguments[0]?.toString());
          const result = _origMD5.apply(this, arguments);
          console.log('[CryptoJS.MD5] output:', result.toString());
          return result;
        };
      }
    `)
```

#### 1.3 I/O 关联分析

拿到出口和入口的数据后，进行关联：

```
已知信息：
  出口：sign=a1b2c3d4e5f6... (32位hex → 疑似MD5)
  入口：CryptoJS.MD5 输入 = "page=1&ts=1680000000&key=secret123"

关联结论：
  sign = MD5("page=" + page + "&ts=" + timestamp + "&key=" + secret_key)
```

---

### 第二板斧：插桩解释器（追踪执行链路）

当出入口 Hook 无法直接关联（例如 VM 使用自实现的 MD5 而非 CryptoJS）时，需要插桩解释器。

#### 2.1 定位解释器核心函数

```
MCP 操作：
  - search_code(keyword="while.*true") → 找到解释器主循环
  - search_code(keyword="case 0:|case 1:|case 2:", script_url=N) <!-- v3.1.0: migrated from search_code_in_script -->
    → 在指定大文件中搜索 opcode 分发表（前后 3 行上下文，更精确）
  - scripts(action='get') → 读取解释器函数体 <!-- v3.1.0: migrated from get_script_source -->

判断标准：
  - 函数体超过 500 行
  - 包含 20+ 个 case 分支
  - 有数组索引递增操作（指令指针移动）
```

#### 2.2 分层插桩策略

**原则：由粗到细，逐步缩小范围。**

**第一轮 — 分发器级别（粗粒度）**

```
MCP 操作：
  - hook_function( <!-- v3.1.0: migrated from trace_function -->
      function_path="解释器分发函数名",
      mode='trace',
      log_args=true, log_return=true, max_captures=500,
      persistent=True  → 持久化追踪，跨导航不丢失
    )
  - 触发一次包含签名参数的请求
  - get_trace_data → 获取 500 条执行记录（自动合并页面数据 + Python 端持久化数据）

分析要点：
  - 观察每次调用的参数模式
  - 找到签名值首次出现的调用序号
  - 缩小时间窗口
```

**第二轮 — 子函数级别（中粒度）**

```
从第一轮日志中识别被频繁调用的子函数（通常 3-5 个核心函数）：

MCP 操作：
  - hook_function(function_path="子函数A", mode='trace', log_args=true, log_return=true, log_stack=true) <!-- v3.1.0: migrated from trace_function -->
  - hook_function(function_path="子函数B", mode='trace', log_args=true, log_return=true, log_stack=true) <!-- v3.1.0: migrated from trace_function -->
  - 再次触发请求
  - get_trace_data → 分析子函数的输入输出
```

**第三轮 — 字符串操作级别（细粒度）**

```
JSVMP 中字符串操作是签名生成的关键信号：

MCP 操作：
  - hook_function(function_path="String.prototype.charAt",
      hook_code="console.log('[charAt]', this.substring(0,50), arguments[0])",
      position="before")
  - hook_function(function_path="String.prototype.charCodeAt", ...)
  - hook_function(function_path="String.prototype.substring", ...)
  - hook_function(function_path="String.prototype.concat", ...)
  - hook_function(function_path="Array.prototype.join", ...)

分析要点：
  - 追踪字符串如何被逐步拼接
  - 观察 charCodeAt 的输入字符和输出编码
  - 关注 join('') 调用 —— 通常是最终拼接点
```

#### 2.3 关键变量监控

**方法 1：使用 MCP 专用工具（推荐）**

```
MCP 操作：
  - hook_jsvmp_interpreter(mode='proxy', trackProps=True, target_expression="疑似签名容器对象") <!-- v3.1.0: migrated from trace_property_access -->
    → Proxy 级别属性访问追踪，自动记录读写操作
  - get_property_access_log → 获取属性访问记录
  - compare_env → 收集完整浏览器环境信息
    → 用于后续补环境时精确对照 Node.js/Python 差异
```

**方法 2：手动注入监控脚本**

```javascript
// 通过 add_init_script(persistent=True) 注入

(function() {
  // 方法1：监控全局变量写入
  const watched = ['sign', 'm', '_signature', 'token'];
  const origDefineProperty = Object.defineProperty;
  watched.forEach(key => {
    let val;
    try {
      origDefineProperty(window, key, {
        get() { return val; },
        set(v) {
          console.log(`[GLOBAL_SET] window.${key} =`, v);
          console.trace();
          val = v;
        },
        configurable: true
      });
    } catch(e) {}
  });

  // 方法2：监控对象属性赋值
  const _origAssign = Object.assign;
  Object.assign = function() {
    const result = _origAssign.apply(this, arguments);
    for (let i = 1; i < arguments.length; i++) {
      const src = arguments[i];
      if (src && typeof src === 'object') {
        Object.keys(src).forEach(k => {
          if (watched.includes(k.toLowerCase())) {
            console.log(`[ASSIGN] ${k} =`, src[k]);
            console.trace();
          }
        });
      }
    }
    return result;
  };
})();
```

---

### 第三板斧：日志分析（从海量数据中提取签名链路）

JSVMP 的 trace 日志通常有数百甚至数千条，需要系统化的分析方法。

#### 3.1 日志采集

```
MCP 操作：
  - get_trace_data → 获取函数追踪数据（自动合并页面数据 + Python 端持久化数据）
  - get_jsvmp_log → 获取 JSVMP 专项日志（含 API 调用统计 + 属性读取摘要）
  - get_console_logs → 获取 Hook 输出
  - get_breakpoint_data → 获取伪断点捕获数据
  - get_property_access_log → 获取属性访问记录
```

#### 3.2 过滤策略

| 过滤维度 | 方法 | 说明 |
|---------|------|------|
| 关键词过滤 | 搜索签名值片段 | 在日志中搜索已知签名值的前8位 |
| 时间窗口过滤 | 只看请求前 1-2 秒 | 签名通常在请求发送前即时生成 |
| 类型过滤 | 只看字符串参数 | 数值型操作大多是 VM 内部调度 |
| 长度过滤 | 关注长度 > 8 的字符串 | 排除短字符串噪声 |
| 变化过滤 | 对比多次请求日志 | 每次都变的是动态参数，不变的是密钥 |

#### 3.3 反向追踪法（核心技巧）

从已知的签名值反向追踪到原始明文：

```
步骤 1：在日志中搜索签名值
  → 搜索 "a1b2c3d4" （签名值的前几位）
  → 找到：[trace#247] return "a1b2c3d4e5f6..."

步骤 2：查看该次调用的输入
  → [trace#247] args: ["page=1&ts=1680000000&key=secret123"]
  → 这是签名函数的输入明文

步骤 3：继续追踪输入来源
  → 搜索 "page=1&ts=1680000000"
  → [trace#245] return "page=1&ts=1680000000&key=secret123"
  → [trace#245] args: ["page=1", "ts=1680000000", "key=secret123"]
  → 这是参数拼接函数

步骤 4：确认各参数来源
  → "page=1" → 用户输入
  → "ts=1680000000" → Date.now() / 1000
  → "key=secret123" → 硬编码密钥

结论：sign = MD5("page=" + page + "&ts=" + Math.floor(Date.now()/1000) + "&key=secret123")
```

#### 3.4 多次请求对比法

```
请求 1: sign=aaa111, ts=1680000000, page=1
请求 2: sign=bbb222, ts=1680000005, page=2
请求 3: sign=ccc333, ts=1680000010, page=1

对比分析：
  - page 相同但 sign 不同（请求1 vs 请求3）→ ts 参与签名
  - ts 不同且 sign 不同 → ts 是变化因子之一
  - 请求1 和 请求3 page 相同但 sign 不同 → 不仅仅是 page 参与

结论：sign 至少由 page + ts 共同决定
```

#### 3.5 算法指纹识别

根据 I/O 特征反推算法类型：

| 输出特征 | 可能算法 |
|---------|---------|
| 32 位 hex | MD5 |
| 40 位 hex | SHA-1 |
| 64 位 hex | SHA-256 |
| 44 位 Base64（含 `=` 填充） | HMAC-SHA256 + Base64 |
| 24/32/44 位 Base64 | AES 加密（CBC/ECB） |
| 输出长度随输入变化 | 非固定哈希，可能是加密或自定义编码 |
| 输出包含特殊分隔符 | 自定义拼接格式 |

---

## 常见 JSVMP 变体及应对

### 变体 1：VM 劫持 XHR/Fetch

**特征**：VM 重写了 `XMLHttpRequest` 或 `fetch`，请求在 VM 内部完成，外部 Hook 抓不到。

**应对**：
```
- Hook 更底层的接口：
  · hook_function(function_path="XMLHttpRequest.prototype.send", ...)
  · 在 add_init_script 中保存原始引用，在 VM 重写之前

- 或 Hook 网络层：
  · network_capture(action='start') → 不依赖 JS 层 Hook，直接在协议层捕获 <!-- v3.1.0: migrated from start_network_capture -->
  · list_network_requests → get_network_request → 获取完整请求详情
```

### 变体 2：VM 动态生成加密函数

**特征**：加密算法不是硬编码在字节码中，而是 VM 运行时通过 `eval` 或 `new Function` 动态构造。

**应对**：
```
- inject_hook_preset(preset="crypto") → 包含 eval/Function Hook
- hook_function(function_path="Function",
    hook_code="console.log('[new Function]', arguments[arguments.length-1].substring(0,200))",
    position="before")
- 从 Hook 日志中提取动态生成的函数源码
```

### 变体 3：多层 VM 嵌套

**特征**：外层 VM 解密出内层 VM 的字节码，再由内层 VM 执行签名逻辑。

**应对**：
```
- 不要试图理解嵌套关系
- 依然从 I/O 两端入手，Hook 最终的出口和最初的入口
- 增加 hook_function(mode='trace') 的 max_captures 到 1000+ <!-- v3.1.0: migrated from trace_function -->
- 用时间戳过滤法缩小日志范围
```

### 变体 4：VM + WASM 混合

**特征**：VM 负责流程控制，核心加密算法调用 WASM 导出函数。

**应对**：
```
- search_code(keyword="WebAssembly|wasm|instantiate")
- Hook WASM 导出函数：
  evaluate_js(expression=`
    // 等 WASM 实例化后 Hook 导出函数
    const origInstantiate = WebAssembly.instantiate;
    WebAssembly.instantiate = async function() {
      const result = await origInstantiate.apply(this, arguments);
      const exports = result.instance?.exports || result.exports;
      Object.keys(exports).forEach(name => {
        if (typeof exports[name] === 'function') {
          const orig = exports[name];
          exports[name] = function() {
            console.log('[WASM]', name, 'args:', Array.from(arguments));
            const ret = orig.apply(this, arguments);
            console.log('[WASM]', name, 'return:', ret);
            return ret;
          };
        }
      });
      return result;
    };
  `)
- 确认 WASM 函数的 I/O 后，下载 .wasm 文件用 Node.js 直接加载调用
```

---

## 前置判断（v2.6.0 新增）：先识别 JSVMP 所在反爬类型

在选路径 A / B 之前，先判断 JSVMP 的反爬类型：

```
navigate(url) 不加任何 hook 观察 redirect_chain
│
├─ 反复 412 后才到 200 → 签名型 JSVMP（RS/Akamai）
│   └─ 走"签名型专属路径"（见下文）
│
├─ 直接 200 但主请求链带签名参数 → 行为型 JSVMP（短视频平台/JY）
│   └─ 按原路径 A / 路径 B 决策树选择
│
└─ 直接 200 无签名参数 → 非 JSVMP 或 VMP 已被跳过
    └─ 回 Phase 2.2 常规混淆分析
```

### 签名型 JSVMP 专属路径（v2.6.0 新增）

签名型 JSVMP 与原路径 A / B 的决策树**不适用**，因为：
- 路径 A 的前三板斧（Hook/插桩/日志）会破坏签名
- 路径 B 的 jsdom 环境伪装成本高（58+ 项修复）

签名型 JSVMP 推荐走的路：

```
1. instrumentation(action='install', mode="ast", tag="...")  (第四板斧) <!-- v3.1.0: migrated from instrument_jsvmp_source -->
2. instrumentation(action='reload')  让插桩后的 VMP 跑完挑战 <!-- v3.1.0: migrated from reload_with_hooks -->
3. instrumentation(action='log', tag_filter, type_filter="tap_get") <!-- v3.1.0: migrated from get_instrumentation_log -->
4. 基于 hot_keys 补最小环境，在 jsdom 或纯 Python 侧独立跑
   （此时 jsdom 补环境的粒度比路径 B 小得多，只补 hot_keys 显示的属性）
```

源码级插桩失败才回落：
```
A. hook_jsvmp_interpreter(mode="transparent")  仅 prototype getter 替换
   - 对签名型大多数情况安全
   - 极严格反爬仍能识别 getter identity 变化，此时真的只能走路径 B 全量 jsdom 伪装
B. 路径 B 完整 jsdom 环境伪装（六步法）
   - 最重但最稳，是最后的兜底
```

---

## 还原策略决策（v2.5.0 扩充源码级插桩分支）

```
JSVMP 签名还原决策树：

第 0 步：先看 hot_methods（第四板斧 summary）
  ├─ hot_methods 包含 CryptoJS.MD5 / SubtleCrypto.digest / HMAC / btoa
  │   → 算法是标准加密，继续看第 1 步分支
  │
  ├─ hot_methods 全是自定义 fn 名，几乎没有 CryptoJS/Subtle
  │   → VMP 自实现加密，直接跳到第 3 步（沙箱运行完整 VM）

第 1 步：标准算法分支
  ├─ 能从 hot_methods + 调用栈反推出明文拼接顺序？
  │   ├─ YES → 纯算法还原（Node.js crypto / Python hashlib+pycryptodome）
  │   └─ NO（明文来自 VM 内部）→ 进第 2 步

第 2 步：能提取独立的签名函数？
  ├─ YES → 沙箱执行（Node.js vm / Python execjs）
  │   · 提取函数及其依赖
  │   · 构造最小执行环境（根据 hot_keys 对齐环境变量）
  └─ NO → 第 3 步

第 3 步：VMP 劫持了整个请求链路 + hot_keys 显示大量环境读取？
  ├─ YES → 走路径 B（jsdom 环境伪装）
  │   · hot_keys 直接告诉你要对齐哪些环境属性（省去 58 项人工 diff）
  │   · analyze_cookie_sources 确认 cookie 是否来自 HTTP Set-Cookie
  │   · 参考 references/jsdom-env-patches.md
  └─ NO（jsdom 加载失败 / VMP 有 CDN 反射检测）→ 第 4 步

第 4 步：加载完整 VM（仅调用签名入口）
  · 下载完整 JS 文件
  · 在 Node.js vm 中加载
  · 补最小浏览器环境（仍然根据 hot_keys 指导）
  · 调用签名函数获取结果

第 5 步：实在无法脱离浏览器
  → 使用 Camoufox 浏览器自动化（最后手段，不是协议优先方案）
```

**关键：第四板斧的 `hot_keys` / `hot_methods` 是所有决策的共同输入**——不要在没看 hot_keys 之前就选策略。即使最后选了路径 B，`hot_keys` 也能把"要对齐什么环境"从 58 项人工 diff 降为 20 项精确对齐。

---

## 实战检查清单（v2.5.0 扩充四板斧项）

在 JSVMP 分析过程中，确保以下每项都已完成：

**第一板斧 — Hook 出入口**：
- [ ] `search_code(keyword='switch', script_url=..., context_chars=500)` 确认是 VMP（case_count > 50） <!-- v3.1.0: migrated from find_dispatch_loops -->
- [ ] `inject_hook_preset("xhr", persistent=True)` + `("fetch", persistent=True)` 捕获请求出口
- [ ] `inject_hook_preset("cookie", persistent=True)` **原型链级** cookie hook（替代手写 Document.prototype.cookie 的做法）
- [ ] `analyze_cookie_sources` 归因最终 cookie（HTTP vs JS）
- [ ] `inject_hook_preset("crypto", persistent=True)` 捕获加密原语入口（btoa/atob/CryptoJS/MD5/SHA）
- [ ] Hook String.fromCharCode（JSVMP 高频信号）
- [ ] `hook_function(..., mode='intercept', ...)` 防止 VMP 覆盖 hook <!-- v3.1.0: migrated from freeze_prototype -->

**第二板斧 — 插桩解释器**：
- [ ] `hook_jsvmp_interpreter(script_url=<VMP basename>)` 多路径探针（apply/call/bind + Reflect.*/Proxy）
- [ ] 分层 `hook_function(mode='trace')`（粗 → 中 → 细，用 `max_captures` 限制日志量） <!-- v3.1.0: migrated from trace_function -->
- [ ] `hook_jsvmp_interpreter(mode='proxy', trackProps=True, targets=["navigator.*", "screen.*", ...])` 监控签名容器 <!-- v3.1.0: migrated from trace_property_access -->

**第三板斧 — 日志分析**：
- [ ] `get_jsvmp_log` + `get_trace_data` + `get_runtime_probe_log`（如果装了 runtime_probe）+ `get_console_logs`
- [ ] 反向追踪法，找到签名值首次出现的位置
- [ ] 多次请求对比，确认变化因子和固定因子

**第四板斧 — 源码级插桩 [v2.5.0 新增]**：
- [ ] `instrumentation(action='install', url_pattern=..., mode="ast"|"regex", tag=...)` 装好 <!-- v3.1.0: migrated from instrument_jsvmp_source -->
- [ ] `instrumentation(action='reload')` 让插桩先于 VMP 生效 <!-- v3.1.0: migrated from reload_with_hooks -->
- [ ] 读 `instrumentation(action='log')` 三个 summary：`hot_keys` / `hot_methods` / `hot_functions` <!-- v3.1.0: migrated from get_instrumentation_log -->
- [ ] `hot_keys` 给出 VMP 读取的环境指纹集（top 30）
- [ ] `hot_methods` 给出 VMP 调用的方法（ObjectType.methodName 格式）
- [ ] `hot_functions` 给出 VMP 调用的函数名
- [ ] `instrumentation(action='stop')` 完工清理 <!-- v3.1.0: migrated from stop_instrumentation -->

**环境/时序**：
- [ ] 若目标是首屏挑战页，用 `navigate(pre_inject_hooks=[...])` 或 `via_blank=True`
- [ ] `compare_env` 采集环境基准
- [ ] `bypass_debugger_trap` 绕过反调试

**还原与验证**：
- [ ] 根据 hot_methods 判断算法类型
- [ ] 选择还原策略（纯算法 / 沙箱 / 完整 VM / jsdom 环境伪装 / 浏览器）
- [ ] 实现并验证签名计算结果
- [ ] 端到端验证（连续 5+ 次请求稳定）
