# JSVMP 源码级插桩专项指南（第四板斧）

> **v2.5.0 新增文档，v3.4.1 更新健康诊断**。当前执行接口以 camoufox-reverse MCP v1.1.1 的 `instrumentation(action=...)` 为准；文中的旧工具名只用于解释能力来源。
>
> 与 `jsvmp-analysis.md` 的第一/二/三板斧互补——前三板斧诊断"VM 看外界 / 外界看 VM"，本文档讲解"VM 看自己"。

---

## 1. 为什么需要源码级插桩

### 1.1 传统 hook 的盲区

**`hook_jsvmp_interpreter`** 即使升级到 v0.4.0 的多路径版本（apply/call/bind + Reflect.*/Proxy 全局对象 + timing/random），仍然只能看到 VMP 路由到**可 hook JS API** 的部分。但RS 5/6、Akamai sensor_data v2/v3、webmssdk（短视频平台）、obfuscator.io 这类 VMP 的典型结构是：

```js
// 典型的自包含 VMP 字节码分发循环
function _vm(bytecode) {
  var stack = [], pc = 0, env = window;
  while (pc < bytecode.length) {
    var op = bytecode[pc++];
    switch (op) {
      case 1: stack.push(env[bytecode[pc++]]); break;    // ← GET 操作：env[key] 直接访问，不经过 apply/Reflect
      case 2: var k = stack.pop(), o = stack.pop(); stack.push(o[k]); break;  // ← 对象属性读取
      case 3: var args = stack.splice(-bytecode[pc++]);
              var fn = stack.pop(); stack.push(fn.apply(null, args)); break;  // ← 这种才被 apply hook 看到
      case 4: stack.push(bytecode[pc++] + stack.pop()); break;                // ← 纯字符串拼接，hook 完全抓不到
      // ... 数十到数百个 case
    }
  }
}
```

- `case 1` 直接访问 `env[key]`，hook `navigator`/`screen` 的 Proxy 能看到（如果 env 是被 Proxy 的全局），但嵌套多层后 Proxy 链会断。
- `case 2` 读任意对象的任意属性——hook 不可能覆盖所有对象。
- `case 4` 纯内存里的字符串拼接，任何 hook 都看不到。

**结果**：VMP 生成签名需要的关键中间值在 dispatch 循环内部全部丢失，传统 hook 返回的日志里只有零散的 API 调用，无法拼出算法。

### 1.2 源码级插桩的原理

**`instrument_jsvmp_source`** 在 HTTP 层拦截目标脚本，在脚本抵达浏览器前把源码改写成等价的"带 tap 的版本"：

```js
// 改写前
stack.push(env[key]);

// 改写后（regex 模式）
stack.push(__mcp_tap_get(env, key, 'vmp1'));

// 改写前
fn.apply(null, args);

// 改写后（AST 模式）
__mcp_tap_call(fn, null, [args[0], args[1], ...], 'vmp1');
```

`__mcp_tap_get` / `__mcp_tap_call` / `__mcp_tap_method` 三个运行时函数会把每次访问记录到 `window.__mcp_vmp_log`，包括：

- **tap_get**: 读取属性——`{type, tag, key, objType, value (preview)}`
- **tap_method**: `obj.method(args)` 调用——`{type, tag, objType, method, argc, arg0, ret}`
- **tap_call**: `fn(args)` 直接调用——`{type, tag, name, argc, arg0, ret}`
- **tap_call_err**: 调用抛错——`{type, tag, name, err}`

改写后的源码**继续正常执行**——VMP 照跑不误，但每一次对宿主环境或对象的交互都被记录。

---

## 2. 两种改写模式：regex vs AST

`instrument_jsvmp_source(mode=...)` 支持两种模式：

### 2.1 `mode="regex"`（轻量兜底，零依赖）

- **原理**：纯正则匹配 `<identifier>[<expr>]`，替换为 `__mcp_tap_get(identifier, expr, tag)`
- **优点**：无外部 CDN 依赖，页面离线也能用；速度极快；大文件安全（10MB+ VMP 也能秒级改写）
- **覆盖率**：~80% 的 member access；**不覆盖函数调用 tap**（只改写 `obj[key]`，不改写 `fn(args)`）
- **局限**：对带模板字符串、正则字面量、嵌套方括号的代码可能误改写；只改 member access，不改 call
- **使用场景**：
  - 极限大文件（AST 模式内存超限）
  - 快速扫一遍看看 hot_keys 分布

### 2.2 `mode="ast"`（默认，MCP 侧 esprima）

- **原理**：在 MCP 进程中用 esprima-python 解析源码为 AST，识别 `MemberExpression` / `CallExpression` 后插入 tap
- **优点**：同时改写 member access 和 call；不会把字符串字面量或正则内容当作代码节点
- **依赖**：不依赖页面 CDN；esprima 仅覆盖其支持的 JavaScript 语法版本
- **使用场景**：
  - 绝大多数场景（默认推荐）
  - 需要 `hot_methods` / `hot_functions` 数据（只有 AST 模式会生成）
- **失败回退**：原始源码解析失败时可自动回落 regex；`last_mode_used="ast"` 不代表改写产物已经在浏览器成功执行

> **v0.5.0 变化**：`mode="ast"` 的实现从 v0.4.x 的"页面内加载 acorn CDN"改为
> **MCP 侧 esprima-python 解析**。这意味着：
> - 挑战页（RS 412、Akamai sensor_data 挑战等）上 AST 模式**能用了**（旧实现因 CDN 被挑战拦住会静默失败）
> - 零 JS 依赖，`instrumentation(action="install")` 的 AST 模式现在**应成为默认选择**
> - 新增 `fallback_on_error=True`，esprima 解析失败时自动回落 regex

### 2.3 选择建议

| 情况 | 选 | 原因 |
|------|-----|-----|
| 首次分析某个 VMP | **AST** | `hot_methods` + `hot_functions` 能快速画出 VMP 行为画像 |
| 只需快速观察 bracket member access | regex | 改写轻、无需 AST |
| VMP 文件 > 5 MB | regex | AST 改写内存开销大 |
| 关心 VMP 调了哪些方法 | **AST** | regex 不生成 tap_method / tap_call |
| 关心 VMP 读了哪些属性 | 任一 | 两种模式都生成 tap_get |

---

## AST 模式健康诊断（v2.6.0 新增）

启用 `instrumentation(action="install", mode="ast")` 后，先通过
`instrumentation(action="status")` 的 `active_patterns[i].last_mode_used` 判断改写路径：

| `last_mode_used` | 含义 | 动作 |
|---|---|---|
| `"ast"` | 原始源码解析成功并完成 AST 改写 | 继续做运行时健康检查 |
| `"regex"` | 你显式指定了 `mode="regex"` | 正常，继续 |
| `"regex (fallback)"` | `mode="ast"` 但 esprima 解析失败，自动回落 regex | **警告**：看下文 |

`"regex (fallback)"` 持续出现时：

1. 先 `get_console_logs` 里搜 `[INSTRUMENT] AST parse failed`，看具体报错
2. 如果是 ES2022+ 的 private field / static block / top-level await 等新语法 → 手动 `mode="regex"`（~80% 覆盖）或 `mode="transparent"`（runtime 观察）兜底
3. 如果是 `SyntaxError: Unexpected token` 开头 → 可能 JS 被加了 BOM 或非标准前缀，可以本地用 `curl` 下来手动检查首 100 字节

一般来说，RS sdenv*.js / Akamai acmescripts*.js / webmssdk 都能被 esprima 解析。如果出现持续 fallback，报告给 MCP 仓库 issue 区。

### 改写产物运行时健康检查

`files_rewritten > 0` 和 `last_mode_used="ast"` 只证明 MCP 生成并下发了改写文本，
不能证明浏览器成功解析和执行了它。每次 reload 后继续检查：

```text
instrumentation(action="status")
evaluate_js(expression="(() => ({
  tapInstalled: window.__mcp_tap_installed === true,
  logReady: Array.isArray(window.__mcp_vmp_log)
}))()")
instrumentation(action="log", tag_filter="vmp1", limit=10)
```

判定：

- `files_rewritten > 0` 且 `tapInstalled=false`：优先怀疑改写产物语法损坏或执行前异常。常见触发结构是嵌套 `CallExpression` / `MemberExpression` / `NewExpression`，不是原始源码 parse 失败。
- `tapInstalled=true` 但日志为空：runtime 已执行，继续检查 VMP 是否触发、tag/filter 是否正确，不要把空日志直接归因于语法错误。
- 确认改写产物失败后，先停止原 AST route，再按 `ast → regex → transparent` 梯度重装；不要通过持续增大 `max_rewrites` 反复下发同一份失败产物。

---

## 3. 黄金 8 步流程

> 这是面向RS 5/6、Akamai sensor_data、webmssdk、obfuscator.io 的通用流程。照抄 `Actions` 段即可，执行方需要替换的只有 `url_pattern` 和 `script_url`。

### Step 1 — 启动 + 网络捕获

```
Actions:
  launch_browser(headless=False)   # 有头调试，看浏览器行为
  start_network_capture(capture_body=True)   # 抓响应体，后续 analyze_cookie_sources 需要
```

### Step 2 — 第一次导航定位 VMP 脚本 URL

```
Actions:
  navigate(url="https://target.com/", wait_until="load")
  list_network_requests(resource_type="script")
  → 找 size 最大的 JS（通常 100KB+，名字像 sdenv-*.js / FuckCookie_*.js / webmssdk.es5.js / akam/xxx.js）
  → 记下 url，下面称为 <VMP_URL>
```

### Step 3 — 确认是 VMP（而不是普通混淆）

```
Actions:
  find_dispatch_loops(script_url="<VMP_URL>", min_case_count=20)
  → 返回：
    {
      "candidates": [
        {"fn_name": "_0xabc", "case_count": 87, "char_range": [45321, 182400], "preview": "..."}
      ],
      "total": 1,
      "source_length": 382112
    }
  → case_count > 50 基本确认是 VMP
  → 记下 fn_name 和 char_range（后续定位方便）
```

### Step 4 — 装源码级插桩（核心）

```
Actions:
  instrument_jsvmp_source(
    url_pattern="**/sdenv-*.js",       # glob 模式，匹配所有 CDN hash 变种
    mode="ast",                         # 联网优先 ast
    tag="vmp1",                         # 一次插桩多 VMP 时区分
    rewrite_member_access=True,
    rewrite_calls=True,
    max_rewrites=5000
  )
  → 返回：{"status": "instrumenting", "pattern": "...", "mode": "ast", "tag": "vmp1"}
```

### Step 5 — 装兜底 hook（与源码插桩互补）

```
Actions:
  inject_hook_preset(preset="cookie", persistent=True)       # 原型链级 cookie hook
  inject_hook_preset(preset="xhr", persistent=True)          # 请求出口
  inject_hook_preset(preset="fetch", persistent=True)        # 请求出口
  inject_hook_preset(preset="crypto", persistent=True)       # btoa/atob/JSON.stringify
  hook_jsvmp_interpreter(script_url="<VMP basename>",        # 多路径运行时探针
                         track_calls=True,
                         track_reflect=True,
                         track_props=True)
  bypass_debugger_trap()                                      # 避免 VMP 的 debugger 陷阱
```

### Step 6 — reload_with_hooks 让探针先于 VMP 生效

```
Actions:
  reload_with_hooks(clear_log=True, wait_until="networkidle")
  → 注意 final_status：
    · 200   → 正常
    · 412   → RS挑战页未过，需要等 challenge 完成
    · 403   → Akamai 拦截，检查指纹或 UA

特殊场景：若目标就是首屏挑战页（RS 412），用：
  navigate(
    url="https://target.com/",
    pre_inject_hooks=["xhr", "fetch", "cookie", "jsvmp_probe"],
    via_blank=True,
    wait_until="networkidle"
  )
```

### Step 7 — 触发业务操作

```
Actions:
  click(selector=".some-btn")
  # 或
  type_text(selector="#search", text="test")
  click(selector=".submit")
  # 或
  evaluate_js(expression="document.querySelector('.nextPage').click()")

  → VMP 会在这一步生成签名并发起请求
```

### Step 8 — 读日志，分析 hot_keys / hot_methods / hot_functions

```
Actions (hot_keys — VMP 读取了哪些属性):
  get_instrumentation_log(
    tag_filter="vmp1",
    type_filter="tap_get",
    limit=200
  )
  → summary.hot_keys:
    {
      "userAgent": 120,
      "plugins": 98,
      "webdriver": 77,
      "platform": 65,
      "language": 54,
      "cookie": 43,
      ...
    }
  → 这就是 VMP 参与签名哈希的完整环境指纹集

Actions (hot_methods — VMP 调用了哪些方法):
  get_instrumentation_log(
    tag_filter="vmp1",
    type_filter="tap_method",
    limit=200
  )
  → summary.hot_methods:
    {
      "Object.defineProperty": 45,
      "Array.prototype.join": 38,
      "String.prototype.charCodeAt": 30,
      "CryptoJS.MD5": 12,
      ...
    }
  → 这告诉你算法是否用标准加密

Actions (hot_functions — VMP 调用了哪些函数):
  get_instrumentation_log(
    tag_filter="vmp1",
    type_filter="tap_call",
    limit=200
  )
  → summary.hot_functions:
    {
      "_0xabc": 300,
      "_0xdef": 150,
      "encodeURIComponent": 8,
      "parseInt": 6,
      ...
    }
```

---

## 4. 进阶技巧

### 4.1 多 VMP 场景（一次分析两个脚本）

```
instrument_jsvmp_source(url_pattern="**/webmssdk.es5.js", mode="ast", tag="webmssdk")
instrument_jsvmp_source(url_pattern="**/a_bogus.js",       mode="ast", tag="bogus")

# 分别看
get_instrumentation_log(tag_filter="webmssdk", type_filter="tap_get")
get_instrumentation_log(tag_filter="bogus",    type_filter="tap_get")
```

### 4.2 key_filter 锁定某个具体属性

```
# 只看 VMP 读取 navigator.webdriver 的记录
get_instrumentation_log(tag_filter="vmp1", key_filter="webdriver")

# 只看 VMP 调用 CryptoJS.MD5 的记录（method_name 过滤）
get_instrumentation_log(tag_filter="vmp1", key_filter="MD5")
```

### 4.3 管理插桩 route

```
# 查看所有激活的 route
get_instrumentation_status()

# 停止单个
stop_instrumentation(url_pattern="**/sdenv-*.js")

# 全部停止
stop_instrumentation()
```

### 4.4 与 runtime_probe 互补

runtime_probe 是低开销广谱探针，**不**改写源码，只 override 几个热点 API。源码级插桩改写 VMP 源码；runtime_probe 覆盖浏览器原生 API。两者叠加可以看到：

- VMP 内部 `obj[key]` 调用 → `get_instrumentation_log`
- VMP 最终落到 `XMLHttpRequest.open` / `canvas.toDataURL` / `navigator.userAgent` 的 getter 调用 → `get_runtime_probe_log`

```
inject_hook_preset(preset="runtime_probe", persistent=True)
instrument_jsvmp_source(url_pattern="**/sdenv-*.js", mode="ast", tag="vmp1")
reload_with_hooks()
触发操作
# 两路都读
get_instrumentation_log(tag_filter="vmp1", limit=300)
get_runtime_probe_log(type_filter="xhr_send", limit=100)
get_runtime_probe_log(type_filter="canvas_toDataURL", limit=50)
```

---

## 5. 常见问题与陷阱

### Q1：`files_rewritten=0`，没改写到

**可能原因**：
1. `url_pattern` 没命中——确认 VMP 实际 URL（`list_network_requests` 里看），glob 模式要用 `**/` 前缀才能匹配任意路径
2. VMP 从缓存加载——强制 `reload_with_hooks(clear_log=True)` 或清浏览器缓存
3. pattern 注册太晚——要在 `navigate` / `reload_with_hooks` **之前**调用 `instrument_jsvmp_source`

### Q2：AST 模式报 `parse_error`

**可能原因**：原始脚本包含 esprima-python 不支持的语法。

**对策**：
- 降级为 `mode="regex"`（零依赖，不会 parse）
- 或先 `save_script` 把脚本存本地，人工看看是不是有奇怪的语法，再考虑局部 patch

### Q3：改写后页面崩溃

**可能原因**：
1. `files_rewritten > 0` 但 runtime 标记不存在——AST 改写产物可能因嵌套节点区间冲突而无法解析；停止 AST route 后降级 regex/transparent
2. VMP 自己对源码做完整性校验——用 regex 模式做轻改写，只改 member access
3. Tag 变量名冲突——改个 `tag` 再试
4. 改写后代码过大——把 `max_rewrites` 调小

### Q4：hot_keys 没出现预期的环境指纹

**可能原因**：
1. VMP 还没真正执行——加大 `wait_until="networkidle"`，或 `wait_for` 某个元素后再读日志
2. 改写或执行没成功——先看 `files_rewritten`，再检查 `window.__mcp_tap_installed`
3. tag 搞混了——检查 `tag_filter` 是否写对

### Q5：日志爆炸到上限 20000 条

**对策**：
- `get_instrumentation_log(..., clear=True)` 读完清
- 收紧 `url_pattern`，只插桩必要的一个脚本
- 第一次跑用 `limit=300`，看 top hot_keys 就够，不需要全部 entries

---

## 6. 源码级插桩的还原决策

拿到 `hot_keys` / `hot_methods` / `hot_functions` 三个摘要后，按下面的表格选策略：

| hot_methods 特征 | hot_keys 环境属性数 | cookie 来源（analyze_cookie_sources） | 策略 |
|-----------------|---------------------|--------------------------------------|------|
| 包含 CryptoJS.MD5 / SubtleCrypto.digest / HMAC | 少（< 10） | 全部 js_document_cookie | 纯算法还原（crypto/hashlib） |
| 包含 CryptoJS 但环境读取多 | 中（10-30） | 混合或 js_document_cookie | 提取 JS 签名函数 + vm/execjs 沙箱 |
| 全是自定义 fn 名，无可识别加密 | 多（30+） | 任意 | 路径 B：jsdom 环境伪装（见 `jsdom-env-patches.md`） |
| 全是自定义 fn 名 | 中-多 | 主要 http_set_cookie | 路径 B + 重点关注发 token 的 POST 接口 |

---

## 7. 参考

- `references/jsvmp-analysis.md`：四板斧方法论总纲
- `references/jsdom-env-patches.md`：环境伪装补丁库（路径 B）
- `references/mcp-cookbook.md` 场景 6：源码级插桩场景化操作
- `cases/universal-vmp-source-instrumentation.md`：骨架案例模板
- MCP 上游：https://github.com/WhiteNightShadow/camoufox-reverse-mcp（当前基线 v1.1.1）
