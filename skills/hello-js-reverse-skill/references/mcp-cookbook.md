> ⚠️ v3.3.0 起，本文档的核心动作清单已回迁到 SKILL.md 核心层。
> 本文档保留作为深度背景参考，但**以 SKILL.md 为准**。如发现不一致，以 SKILL.md 为准。

# MCP 工具使用手册

## 概述

本手册详细说明 `camoufox-reverse` MCP 服务器的使用方式，
提供常见逆向场景下的具体操作步骤。

## 工具速查表

### camoufox-reverse MCP（65 个工具，v0.4.0+）

| 类别 | 工具 | 核心用途 |
|------|------|---------|
| **浏览器** | `launch_browser` / `close_browser` | 启动/关闭反检测浏览器 |
| **导航** | `navigate` / `reload` / `go_back` | 页面导航 |
| **页面交互** | `click` / `type_text` / `wait_for` | 元素交互 |
| **截图** | `take_screenshot` / `take_snapshot` | 截图 / 无障碍树 |
| **页面信息** | `get_page_info` / `get_page_html` | 页面状态和 HTML |
| **源码搜索** | `search_code` | 在所有 JS 中搜索（结构化结果，默认 max_results=50） |
| **精确搜索** | `search_code(..., script_url=...)` | 在指定脚本中搜索（前后 3 行上下文，适合大文件） | <!-- v3.1.0: migrated from search_code_in_script -->
| **源码获取** | `scripts(action='get', ...)` | 获取指定脚本的代码 | <!-- v3.1.0: migrated from get_script_source -->
| **源码保存** | `scripts(action='save', ...)` | 将脚本保存到本地文件 | <!-- v3.1.0: migrated from save_script -->
| **脚本列表** | `scripts(action='list')` | 列出页面加载的所有脚本 | <!-- v3.1.0: migrated from list_scripts -->
| **执行 JS** | `evaluate_js` / `evaluate_js_handle` | 在页面执行任意 JS |
| **加载前注入** | `add_init_script` | 页面加载前注入脚本（支持 `persistent` 跨导航重注入） |
| **伪断点** | `set_breakpoint_via_hook` | 通过 Hook 设置伪断点（支持 `persistent` 持久化） |
| **断点数据** | `get_breakpoint_data` | 获取伪断点捕获的数据 |
| **函数追踪** | `hook_function(..., mode='trace', ...)` / `get_trace_data` | 追踪函数调用（支持 `persistent` 跨导航持久化） | <!-- v3.1.0: migrated from trace_function -->
| **自定义 Hook** | `hook_function` | 注入自定义 Hook（before/after/replace，支持 `non_overridable` 防覆盖） |
| **预设 Hook** | `inject_hook_preset` | 一键 Hook（xhr/fetch/crypto/websocket/debugger_bypass，默认持久化） |
| **移除 Hook** | `remove_hooks` | 移除所有 Hook（支持 `keep_persistent` 保留持久化 Hook） |
| **冻结原型** | `hook_function(..., mode='intercept', ...)` | 冻结任意原型方法，防止页面 JS 覆盖 Hook | <!-- v3.1.0: migrated from freeze_prototype -->
| **属性追踪** | `hook_jsvmp_interpreter(mode='proxy', trackProps=True)` / `get_property_access_log` | Proxy 级别属性访问追踪 | <!-- v3.1.0: migrated from trace_property_access -->
| **JSVMP 插桩** | `hook_jsvmp_interpreter` | 一键插桩 JSVMP（Function.prototype.apply + 30+ 敏感属性） |
| **JSVMP 日志** | `get_jsvmp_log` | 获取 JSVMP 日志（API 调用统计 + 属性读取摘要） |
| **JSVMP 字符串** | `dump_jsvmp_strings` | 提取字符串数组，识别 API 名称，检测混淆模式 |
| **环境采集** | `compare_env` | 全面收集浏览器环境（navigator/screen/canvas/WebGL/Audio/timing） |
| **网络捕获** | `network_capture(action='start', ...)` / `network_capture(action='stop')` | 启停网络捕获（支持 `capture_body` 捕获响应体） | <!-- v3.1.0: migrated from start_network_capture / stop_network_capture -->
| **网络请求** | `list_network_requests` | 列出捕获的请求 |
| **请求详情** | `get_network_request` | 获取请求完整信息（含响应体） |
| **请求来源** | `get_request_initiator` | 获取请求的 JS 调用栈（改进 URL 匹配 + 诊断信息） |
| **请求拦截** | `intercept_request` / `stop_intercept` | 拦截/修改/Mock 请求 |
| **Cookie** | `cookies(action='get', ...)` / `cookies(action='set', ...)` / `delete_cookies` | Cookie 管理 | <!-- v3.1.0: migrated from get_cookies / set_cookies -->
| **Storage** | `get_storage` / `set_storage` | localStorage / sessionStorage |
| **状态管理** | `export_state` / `import_state` | 保存/恢复浏览器状态 |
| **控制台** | `get_console_logs` | 读取控制台输出 |
| **指纹** | `get_fingerprint_info` | 检查浏览器指纹 |
| **反检测** | `check_detection` | 测试是否被反爬识别 |
| **反调试** | `bypass_debugger_trap` | 绕过 debugger 陷阱 |
| **分发循环扫描** | `search_code(keyword='switch', script_url=..., context_chars=500)` | **[v0.4.0]** 定位字节码分发函数（while+switch，case 数过滤） | <!-- v3.1.0: migrated from find_dispatch_loops -->
| **源码级插桩** | `instrumentation(action='install', ...)` | **[v0.4.0]** HTTP 层改写 VMP，每个 obj[key]/fn(args) 插入 tap（通用 VMP 利器） | <!-- v3.1.0: migrated from instrument_jsvmp_source -->
| **插桩日志** | `instrumentation(action='log', ...)` | **[v0.4.0]** 拉取源码插桩日志，带 hot_keys/hot_methods/hot_functions 摘要 | <!-- v3.1.0: migrated from get_instrumentation_log -->
| **插桩状态** | `check_environment()` / `instrumentation(action='stop', ...)` | **[v0.4.0]** 查看/停止源码插桩 | <!-- v3.1.0: migrated from get_instrumentation_status / stop_instrumentation -->
| **Cookie 归因** | `analyze_cookie_sources` | **[v0.4.0]** 融合 HTTP Set-Cookie + JS document.cookie + cookie jar |
| **hook 重载** | `instrumentation(action='reload')` | **[v0.4.0]** 重载使 persistent hook 先于页面 JS 执行（+ 清日志） | <!-- v3.1.0: migrated from reload_with_hooks -->
| **运行时探针日志** | `get_runtime_probe_log` | **[v0.4.0]** 获取 runtime_probe 预设的广谱事件日志 |
| **预设 Hook（扩展）** | `inject_hook_preset("cookie")` / `inject_hook_preset("runtime_probe")` | **[v0.4.0]** 新增两个预设 |
| **首屏注入** | `navigate(pre_inject_hooks=[...])` | **[v0.4.0]** 经 about:blank 装 hook 再 goto，解决首屏挑战页 |

## 场景化操作手册

### 场景 1：分析数据接口的加密参数

**目标**：找到接口中加密参数的生成逻辑

```
步骤 1：启动反检测浏览器
  [camoufox-reverse] launch_browser()

步骤 2：导航到目标页面
  [camoufox-reverse] navigate(url="https://target.com")

步骤 3：确认页面加载
  [camoufox-reverse] take_screenshot() → 确认页面正常

步骤 4：开始网络捕获
  [camoufox-reverse] network_capture(action='start') <!-- v3.1.0: migrated from start_network_capture -->

步骤 5：触发数据请求（如翻页）
  [camoufox-reverse] click(selector=".next-page")
  或
  [camoufox-reverse] evaluate_js(expression="document.querySelector('.next').click()")

步骤 6：查看网络请求
  [camoufox-reverse] list_network_requests

步骤 7：获取目标请求详情
  [camoufox-reverse] get_network_request(request_id=N)
  → 记录 URL、Headers、Params、Response

步骤 8：获取请求调用栈（黄金路径）
  [camoufox-reverse] get_request_initiator(request_id=N)
  → 直接定位发起请求的 JS 函数

步骤 9：搜索加密参数
  [camoufox-reverse] search_code(keyword="参数名=")
  [camoufox-reverse] search_code(keyword="参数名")

步骤 10：读取相关源码
  [camoufox-reverse] scripts(action='get', script_url="包含加密逻辑的脚本URL") <!-- v3.1.0: migrated from get_script_source -->

步骤 11：设置伪断点验证
  [camoufox-reverse] set_breakpoint_via_hook(target_function="加密函数路径")
  触发请求 →
  [camoufox-reverse] get_breakpoint_data
  → 查看捕获的参数和返回值
```

### 场景 2：动态 Cookie 逆向

**目标**：找到 Cookie 的生成逻辑

```
步骤 1：注入 Cookie Hook
  [camoufox-reverse] hook_function(
    function_path="Document.prototype.cookie",
    hook_code="console.log('[Cookie Set]', arguments[0]); console.trace('Cookie Stack');",
    position="before"
  )
  或使用预设 Hook：
  [camoufox-reverse] inject_hook_preset(preset="crypto")

步骤 2：刷新页面触发 Cookie 生成
  [camoufox-reverse] reload()

步骤 3：读取 Hook 输出
  [camoufox-reverse] get_console_logs
  → 找到 Cookie 设置记录和调用栈

步骤 4：根据调用栈定位生成函数
  [camoufox-reverse] search_code(keyword="目标函数名")
  [camoufox-reverse] scripts(action='get', script_url="...") <!-- v3.1.0: migrated from get_script_source -->

步骤 5：分析生成逻辑
  [camoufox-reverse] set_breakpoint_via_hook(target_function="cookie生成函数")
  刷新页面 →
  [camoufox-reverse] get_breakpoint_data → 查看入参和返回值
  [camoufox-reverse] hook_function(function_path="加密函数", mode='trace', log_args=true, log_return=true, log_stack=true) <!-- v3.1.0: migrated from trace_function -->
  [camoufox-reverse] get_trace_data → 查看中间值
```

### 场景 3：混淆代码分析

**目标**：还原混淆的 JS 代码

```
步骤 1：列出所有脚本
  [camoufox-reverse] scripts(action='list') <!-- v3.1.0: migrated from list_scripts -->
  → 找到可疑的混淆脚本（通常文件名无意义或体积很大）

步骤 2：保存混淆脚本到本地
  [camoufox-reverse] scripts(action='save', script_url="混淆脚本URL", save_path="./project/obfuscated.js") <!-- v3.1.0: migrated from save_script -->

步骤 3：搜索关键特征
  [camoufox-reverse] search_code(keyword="_0x")
  → 确认混淆类型（OB混淆）
  
  [camoufox-reverse] search_code(keyword="switch|while.*true")
  → 检查是否有控制流平坦化

步骤 4：在浏览器中还原字符串
  [camoufox-reverse] evaluate_js(expression="返回字符串数组的代码")
  → 获取解密后的字符串映射

步骤 5：定位关键函数
  [camoufox-reverse] search_code(keyword="encrypt|sign|md5|aes")
  → 找到加密相关代码的大致位置
  [camoufox-reverse] scripts(action='get') → 获取上下文 <!-- v3.1.0: migrated from get_script_source -->
```

### 场景 4：WASM 逆向

**目标**：分析 WASM 加密函数的输入输出

```
步骤 1：搜索 WASM 加载代码
  [camoufox-reverse] search_code(keyword="WebAssembly|.wasm|instantiate")

步骤 2：找到 WASM 文件
  [camoufox-reverse] list_network_requests
  → 找到 .wasm 文件的请求

步骤 3：分析 WASM 调用
  [camoufox-reverse] search_code(keyword="exports.|instance.exports")
  → 找到导出函数的调用代码

步骤 4：追踪输入输出
  [camoufox-reverse] set_breakpoint_via_hook(target_function="wasm导出函数调用路径")
  触发请求 →
  [camoufox-reverse] get_breakpoint_data → 查看入参和返回值

步骤 5：在浏览器中测试
  [camoufox-reverse] evaluate_js(expression="调用wasm函数并返回结果")
  → 验证 I/O 关系
```

### 场景 5：WebSocket 通信分析

**目标**：分析 WebSocket 消息格式和加密

```
步骤 1：注入 WebSocket Hook
  [camoufox-reverse] inject_hook_preset(preset="websocket")

步骤 2：读取 WS 消息日志
  [camoufox-reverse] get_console_logs
  → 获取 WS 发送/接收消息

步骤 3：搜索 WS 相关代码
  [camoufox-reverse] search_code(keyword="WebSocket|ws.send|onmessage")

步骤 4：手动发送 WS 消息
  [camoufox-reverse] evaluate_js(expression="ws.send(JSON.stringify({...}))")
```

### 场景 6：通用 JSVMP 源码级插桩（RS/Akamai/webmssdk）[v2.5.0 新增]

**目标**：定位 VMP 读取的环境指纹集与调用的加密原语，不依赖 VMP 具体实现

```
步骤 1：启动浏览器 + 开启响应体捕获
  [camoufox-reverse] launch_browser(headless=False)
  [camoufox-reverse] network_capture(action='start', capture_body=True) <!-- v3.1.0: migrated from start_network_capture -->

步骤 2：第一次导航定位 VMP 脚本
  [camoufox-reverse] navigate(url="https://target.com/")
  [camoufox-reverse] list_network_requests(resource_type="script")
  → 找最大的 JS（100KB+，通常是 sdenv-*.js / FuckCookie_*.js / webmssdk.es5.js）

步骤 3：确认是 VMP
  [camoufox-reverse] search_code(keyword='switch', script_url="https://target.com/xxx/sdenv-xxx.js", context_chars=500) <!-- v3.1.0: migrated from find_dispatch_loops -->
    min_case_count=20
  )
  → case_count > 50 基本确认是 VMP

步骤 4：装源码级插桩（核心）
  [camoufox-reverse] instrumentation(action='install', <!-- v3.1.0: migrated from instrument_jsvmp_source -->
    url_pattern="**/sdenv-*.js",
    mode="ast",       # 能联网用 ast（99% 覆盖）；离线用 regex（80%）
    tag="vmp1"
  )

步骤 5：兜底 hook
  [camoufox-reverse] inject_hook_preset(preset="cookie", persistent=True)
  [camoufox-reverse] inject_hook_preset(preset="xhr", persistent=True)
  [camoufox-reverse] hook_jsvmp_interpreter(script_url="sdenv-")
  [camoufox-reverse] bypass_debugger_trap()

步骤 6：重载让插桩先于 VMP 生效
  [camoufox-reverse] instrumentation(action='reload') <!-- v3.1.0: migrated from reload_with_hooks -->

步骤 7：触发业务操作（翻页 / 登录 / 搜索）

步骤 8：读 hot_keys / hot_methods / hot_functions（30 秒读完 VMP 指纹）
  [camoufox-reverse] instrumentation(action='log', <!-- v3.1.0: migrated from get_instrumentation_log -->
    tag_filter="vmp1",
    type_filter="tap_get",    # 所有 obj[key] 读取
    limit=200
  )
  → summary.hot_keys: VMP 读了哪些属性（top 30，按频次）

  [camoufox-reverse] instrumentation(action='log', <!-- v3.1.0: migrated from get_instrumentation_log -->
    tag_filter="vmp1",
    type_filter="tap_method",  # 所有 obj.method(args) 调用
    limit=200
  )
  → summary.hot_methods: VMP 调了哪些方法（Array.prototype.join / CryptoJS.MD5 ...）

  [camoufox-reverse] instrumentation(action='log', <!-- v3.1.0: migrated from get_instrumentation_log -->
    tag_filter="vmp1",
    type_filter="tap_call",   # 所有 fn(args) 直接调用
    limit=200
  )
  → summary.hot_functions: VMP 调了哪些函数

步骤 9：Cookie 归因
  [camoufox-reverse] analyze_cookie_sources()
  → 对每个 cookie 返回 sources（http_set_cookie / js_document_cookie）
    + http_responses[].url + js_writes[].stack

步骤 10：完工清理
  [camoufox-reverse] instrumentation(action='stop', url_pattern="**/sdenv-*.js") <!-- v3.1.0: migrated from stop_instrumentation -->
  [camoufox-reverse] remove_hooks()
```

**判定与后续策略**：

| hot_methods 包含 | hot_keys 环境属性数 | 策略 |
|-----------------|---------------------|------|
| CryptoJS.MD5 / SubtleCrypto.digest / btoa | 少（< 15） | 纯算法还原（Node.js crypto / Python hashlib） |
| 大量自定义 fn 名 | 少（< 15） | 提取 VMP 子片段 + vm 沙箱 |
| CryptoJS / SubtleCrypto | 多（40+） + cookie 来自 HTTP Set-Cookie | jsdom 环境伪装（走路径 B） |

详细方法论见 `references/jsvmp-source-instrumentation.md`。

### 场景 7：Cookie 归因分析 [v2.5.0 新增]

**目标**：搞清楚某个 Cookie 到底是谁写的（JS / HTTP Set-Cookie / 混合）

```
步骤 1：启动环境 + 开启响应体捕获 + cookie hook
  [camoufox-reverse] launch_browser(headless=False)
  [camoufox-reverse] network_capture(action='start', capture_body=True) <!-- v3.1.0: migrated from start_network_capture -->
  [camoufox-reverse] inject_hook_preset(preset="cookie", persistent=True)   # 原型链级

步骤 2：导航到目标（首屏有挑战用 pre_inject_hooks）
  [camoufox-reverse] navigate(
    url="https://target.com/",
    pre_inject_hooks=["xhr", "fetch", "cookie"]
  )

步骤 3：触发可疑 Cookie 的生成场景（刷新 / 登录 / 点业务按钮）

步骤 4：一把归因
  [camoufox-reverse] analyze_cookie_sources(name_filter="acw_tc|NfBCSins|x-bogus")
  → 返回：
    {
      "cookies": {
        "acw_tc": {
          "sources": ["http_set_cookie"],
          "first_set_ts": ...,
          "http_responses": [{"url": ".../challenge", "ts": ..., "header": "acw_tc=...; path=/"}],
          "js_writes": [],
          "current_value": "..."
        },
        "x-bogus-js": {
          "sources": ["js_document_cookie"],
          "first_set_ts": ...,
          "http_responses": [],
          "js_writes": [{"value": "x-bogus-js=...", "stack": "...encrypt@webmssdk.es5.js:..."}],
          "current_value": "..."
        }
      },
      "total_cookies": 2
    }

步骤 5：按归因结果进一步分析
  a. 纯 http_set_cookie → 看 http_responses[].url 是哪个接口，它的请求体里有什么签名参数
     → 用场景 1 分析那个接口的加密参数
  b. 纯 js_document_cookie → 看 js_writes[].stack 定位写入函数，search_code 打开源码
  c. 两者都有 → 两步都做（JS 算 token POST 给服务端，服务端 Set-Cookie 回来）
```

**关键洞察**：

> 对于RS、Akamai 这类"JS 算 token、服务端 Set-Cookie"的混合模式，单纯 `hook_function(Document.prototype.cookie)` 是抓不到最终 cookie 写入的——因为它是 HTTP header 写的，不经过 JS。`analyze_cookie_sources` 的价值就是 30 秒内消除这个盲区。

## 高级技巧

### 使用 hook_function(mode='trace') 自动记录函数调用 <!-- v3.1.0: migrated from trace_function -->

```
[camoufox-reverse] hook_function( <!-- v3.1.0: migrated from trace_function -->
    function_path="window.encrypt",
    mode='trace',
    log_args=true,
    log_return=true,
    log_stack=true,
    max_captures=50,
    persistent=True          → 跨导航持久化，页面刷新/跳转后自动重注入
)
```
→ 每次 `encrypt` 被调用时自动记录参数、返回值和调用栈，不暂停执行
→ `get_trace_data` 会自动合并页面数据和 Python 端持久化数据

### 使用 inject_hook_preset 一键 Hook

```
默认 persistent=True（持久化，跨导航自动重注入）：

[camoufox-reverse] inject_hook_preset(preset="xhr")              → Hook 所有 XHR 请求
[camoufox-reverse] inject_hook_preset(preset="fetch")             → Hook 所有 fetch 请求
[camoufox-reverse] inject_hook_preset(preset="crypto")            → Hook btoa/atob/JSON.stringify
[camoufox-reverse] inject_hook_preset(preset="websocket")         → Hook WebSocket 消息
[camoufox-reverse] inject_hook_preset(preset="debugger_bypass")   → 绕过反调试
```

### 使用 hook_function 自定义 Hook

```
[camoufox-reverse] hook_function(
    function_path="XMLHttpRequest.prototype.open",
    hook_code="console.log('[XHR]', arguments[0], arguments[1])",
    position="before",
    non_overridable=True     → 防覆盖：Object.defineProperty 冻结 + toString 伪装 native code
)
```
→ 在 XHR.open 调用前输出请求方法和 URL，即使页面 JS 尝试覆盖也不会丢失

### 使用 hook_function(mode='intercept') 防止 Hook 被覆盖 <!-- v3.1.0: migrated from freeze_prototype -->

```
[camoufox-reverse] hook_function(className="XMLHttpRequest", methodName="send", mode='intercept') <!-- v3.1.0: migrated from freeze_prototype -->
```
→ 冻结原型方法（configurable:false + writable:false），防止 JSVMP 等覆盖你的 Hook

### 使用 JSVMP 专项工具

```
[camoufox-reverse] hook_jsvmp_interpreter        → 一键插桩 Function.prototype.apply + 30+ 敏感属性
[camoufox-reverse] dump_jsvmp_strings             → 提取字符串数组，识别 API 名称
  触发操作...
[camoufox-reverse] get_jsvmp_log                  → 获取 API 调用统计 + 属性读取摘要
[camoufox-reverse] compare_env                    → 收集完整浏览器环境（补环境基准线）
```

#### transparent 模式示例（v0.5.0，v2.6.0 吸收）

```python
# 签名型反爬上优先这样用：只替换 prototype getter，不装 Proxy 不改 Function.prototype
await hook_jsvmp_interpreter(
    script_url="sdenv",
    persistent=True,
    mode="transparent",   # 关键
    # track_calls / track_props / track_reflect / proxy_objects 在 transparent 下被忽略
    max_entries=10000,
)

# 效果：window.__mcp_jsvmp_log 仍然有日志，但每条是 {type: "transparent_get", owner, key, value}
# 不会出现 proxy_get / fn_apply / reflect_get 等 proxy 模式专属类型

# 对应的 pre-inject 别名：
await navigate(url, pre_inject_hooks=["jsvmp_probe_transparent"])
```

**transparent 模式的观察口径**：
- ✅ 会记录：navigator.userAgent / screen.width / document.cookie（读） / location.href 等 prototype 级 getter
- ❌ 不会记录：Function.prototype.apply 调用、Reflect.get/set、navigator.plugins[0].name（深层属性）、navigator.hardwareConcurrency（getter 在不同浏览器可能不是 prototype 级）
- **口径比 proxy 小**，所以只用于 proxy 不能用的场合（签名型反爬）

### 使用属性访问追踪

```
[camoufox-reverse] hook_jsvmp_interpreter(mode='proxy', trackProps=True, target_expression="navigator") <!-- v3.1.0: migrated from trace_property_access -->
  触发操作...
[camoufox-reverse] get_property_access_log        → 查看 navigator 的哪些属性被读取
```
→ 精确发现 JSVMP 或签名逻辑读取了哪些环境属性

### 使用 search_code(..., script_url=...) 精确搜索大文件 <!-- v3.1.0: migrated from search_code_in_script -->

```
[camoufox-reverse] search_code(keyword="sign|token|encrypt", script_url=3) <!-- v3.1.0: migrated from search_code_in_script -->
```
→ 在指定脚本中搜索，返回前后 3 行上下文，适合 200KB+ 的 JSVMP 文件

### 使用 add_init_script 进行预分析

```
[camoufox-reverse] add_init_script(script=`
    const knownGlobals = new Set(Object.keys(window));
    setInterval(() => {
        const newGlobals = Object.keys(window).filter(k => !knownGlobals.has(k));
        if (newGlobals.length > 0) {
            console.log('[Monitor] 新全局变量:', newGlobals);
            newGlobals.forEach(k => knownGlobals.add(k));
        }
    }, 1000);
`, persistent=True)         → 跨导航持久化
```

### 使用 evaluate_js 提取运行时数据

```
[camoufox-reverse] evaluate_js(expression=`
    JSON.stringify(document.cookie.split('; ').reduce((obj, pair) => {
        const [k, v] = pair.split('=');
        obj[k] = v;
        return obj;
    }, {}))
`)
```

### 使用 intercept_request 拦截/修改请求

```
[camoufox-reverse] intercept_request(url_pattern="**/api/data*", action="log")     → 仅记录
[camoufox-reverse] intercept_request(url_pattern="**/ads/*", action="block")       → 屏蔽广告
[camoufox-reverse] intercept_request(url_pattern="**/api/*", action="modify", modify_headers={"X-Custom": "test"})
[camoufox-reverse] intercept_request(url_pattern="**/api/*", action="mock", mock_response={"status": 200, "body": "{\"ok\":1}"})
```

### 使用源码级插桩（v2.5.0 新增，通用 VMP 利器）

```
# 1. 先确认是不是 VMP（case_count > 50 基本是）
[camoufox-reverse] search_code(keyword='switch', script_url="https://target.com/sdenv-xxx.js", context_chars=500) <!-- v3.1.0: migrated from find_dispatch_loops -->

# 2. 装插桩（AST 在 MCP 侧运行；regex 模式兜底）
[camoufox-reverse] instrumentation(action='install', <!-- v3.1.0: migrated from instrument_jsvmp_source -->
    url_pattern="**/sdenv-*.js",    # glob 模式匹配多 hash 版本
    mode="ast",                      # 或 "regex"
    tag="vmp1",                      # 区分多 VMP 的标签
    rewrite_member_access=True,      # 改写每个 obj[key]
    rewrite_calls=True,              # 改写每个 fn(args) / obj.method(args)
    max_rewrites=5000                # 单文件改写上限
)

# 3. 装完后必须重载让插桩生效
[camoufox-reverse] instrumentation(action='reload') <!-- v3.1.0: migrated from reload_with_hooks -->

# 4. 触发操作后读日志
[camoufox-reverse] instrumentation(action='log', tag_filter="vmp1", type_filter="tap_get", limit=200) <!-- v3.1.0: migrated from get_instrumentation_log -->
  → summary.hot_keys 是 VMP 读取的环境属性 top 30（指纹学习金矿）

[camoufox-reverse] instrumentation(action='log', tag_filter="vmp1", type_filter="tap_method", limit=200) <!-- v3.1.0: migrated from get_instrumentation_log -->
  → summary.hot_methods 是 VMP 调用的方法 top 30（ObjectType.methodName 格式）

# 5. 查看插桩状态
[camoufox-reverse] check_environment() <!-- v3.1.0: migrated from get_instrumentation_status -->

# 6. 完工移除
[camoufox-reverse] instrumentation(action='stop', url_pattern="**/sdenv-*.js") <!-- v3.1.0: migrated from stop_instrumentation -->
[camoufox-reverse] instrumentation(action='stop')    # 不传参数 = 全部停止 <!-- v3.1.0: migrated from stop_instrumentation -->
```

**与 hook_jsvmp_interpreter 的分工**：

| 工具 | 能看到 | 看不到 |
|------|--------|--------|
| `hook_jsvmp_interpreter` | VMP 通过 `Function.prototype.apply/call/bind` 或 `Reflect.*` 调子函数，读全局对象属性（Proxy） | VMP 在 switch/case 内部直接 `obj[key]` / `fn(args)` 的调用 |
| `instrumentation(action='install', ...)` | 每一次 member access + 每一次函数调用，无论是否通过可 hook API | 不跨脚本（只改写指定 url_pattern 的那个文件） | <!-- v3.1.0: migrated from instrument_jsvmp_source -->
| **建议**：两者同时开，互相校验 | | |

#### mode="ast" 的 v0.5.0 变化（v2.6.0 吸收）

```python
# v2.5.0 / MCP v0.4.x: mode="ast" 会在页面里注入 acorn CDN script tag
# v2.6.0 / MCP v0.5.0: mode="ast" 在 MCP 进程里用 esprima-python 解析，零页面依赖
await instrumentation(action='install', url_pattern="**/sdenv-*.js", <!-- v3.1.0: migrated from instrument_jsvmp_source -->
    mode="ast",                  # 默认，走 MCP 侧 esprima
    tag="sdenv",
    fallback_on_error=True,      # 新参数：esprima 挂就回落 regex
)

# 查看实际走了哪条路径
status = await instrumentation(action='status')
# status["active_patterns"][0]["last_mode_used"] == "ast" (正常)
#   or "regex (fallback)" (esprima 对该 VMP 的语法支持不全)
```

### 使用 Cookie 归因分析（v2.5.0 新增）

```
# 1. 前置条件：网络抓包 + cookie hook 同时开
[camoufox-reverse] network_capture(action='start', capture_body=True) <!-- v3.1.0: migrated from start_network_capture -->
[camoufox-reverse] inject_hook_preset(preset="cookie", persistent=True)

# 2. 触发场景

# 3. 一键归因
[camoufox-reverse] analyze_cookie_sources(name_filter="ttwid|msToken|acw_tc")

返回字段：
  cookies[name].sources: ["http_set_cookie" | "js_document_cookie"]   # 可能两者都有
  cookies[name].first_set_ts: 首次观察到的毫秒时间戳
  cookies[name].http_responses: [{url, ts, header}]                    # HTTP 写入来源
  cookies[name].js_writes: [{value, stack, ts}]                        # JS 写入来源（含调用栈）
  cookies[name].current_value: 当前 jar 中的值
```

### 使用 runtime_probe 做低开销广谱观察（v2.5.0 新增）

```
# runtime_probe 是 jsvmp_hook 的轻量版：不装 Proxy，只 override 具体热点 API
[camoufox-reverse] inject_hook_preset(preset="runtime_probe", persistent=True)

# 触发操作
[camoufox-reverse] get_runtime_probe_log(type_filter="xhr_send", limit=100)
[camoufox-reverse] get_runtime_probe_log(type_filter="canvas_toDataURL", limit=50)   # canvas 指纹检测
[camoufox-reverse] get_runtime_probe_log(type_filter="nav_read", limit=100)          # navigator 读取
[camoufox-reverse] get_runtime_probe_log(type_filter="addEventListener", limit=100)  # 鼠标/键盘检测

返回 by_type 字段自动聚合各事件类型的总数，便于快速画出"这个页面都在观察什么"。
```

### 使用 pre_inject_hooks 解决首屏挑战页（v2.5.0 新增）

```
# RS 412 / Akamai 首包检测场景：
# 普通 navigate → launch_browser → navigate → 装 hook
# 但此时 challenge JS 已经跑完了，hook 完全漏掉第一次 VMP 执行

# 正确做法：走 about:blank 先装 hook 再 goto
[camoufox-reverse] navigate(
    url="https://target.com/",
    wait_until="networkidle",
    pre_inject_hooks=["xhr", "fetch", "cookie", "jsvmp_probe"],   # 先装这些
    via_blank=True,                                                # 强制经 about:blank
    collect_response_chain=True                                    # 记录重定向链
)

# 返回：
#   initial_status: 首次响应状态（可能是 412）
#   final_status:   最终状态（challenge 过后的 200）
#   redirect_chain: [{url, status, ts}, ...] 整条重定向链
#   hooks_injected: 实际装上的 hook 名列表
#   warnings:       装 hook 失败等非致命问题
```

## camoufox-reverse MCP 工具分类总览（65 个，v0.4.0+）

| 类别 | 工具 | 说明 |
|------|------|------|
| 浏览器生命周期 | `launch_browser` `close_browser` | 启动/关闭 Camoufox |
| 页面导航 | `navigate` `reload` `go_back` | 导航控制 |
| 页面交互 | `click` `type_text` `wait_for` | UI 自动化 |
| 页面状态 | `get_page_info` `take_screenshot` `take_snapshot` `get_page_html` | 页面信息 |
| 源码分析 | `scripts(action='list')` `scripts(action='get', ...)` `search_code` `search_code(..., script_url=...)` `scripts(action='save', ...)` | JS 逆向核心 | <!-- v3.1.0: migrated from list_scripts, get_script_source, search_code_in_script, save_script -->
| JS 执行 | `evaluate_js` `evaluate_js_handle` `add_init_script` | 代码执行（支持持久化） |
| 调试 | `set_breakpoint_via_hook` `get_breakpoint_data` `get_console_logs` | 伪断点调试（支持持久化） |
| Hook | `hook_function(..., mode='trace', ...)` `get_trace_data` `hook_function` `inject_hook_preset` `remove_hooks` `hook_function(..., mode='intercept', ...)` | 函数 Hook（支持持久化 + 防覆盖） | <!-- v3.1.0: migrated from trace_function, freeze_prototype -->
| 属性追踪 | `hook_jsvmp_interpreter(mode='proxy', trackProps=True)` `get_property_access_log` | Proxy 级别属性访问监控 | <!-- v3.1.0: migrated from trace_property_access -->
| JSVMP 专项 | `hook_jsvmp_interpreter` `get_jsvmp_log` `dump_jsvmp_strings` `compare_env` | JSVMP 虚拟机保护分析 |
| 网络 | `network_capture(action='start', ...)` `network_capture(action='stop')` `list_network_requests` `get_network_request` `get_request_initiator` `intercept_request` `stop_intercept` | 网络分析（支持响应体捕获） | <!-- v3.1.0: migrated from start_network_capture, stop_network_capture -->
| 存储 | `cookies(action='get', ...)` `cookies(action='set', ...)` `delete_cookies` `get_storage` `set_storage` `export_state` `import_state` | 状态管理 | <!-- v3.1.0: migrated from get_cookies, set_cookies -->
| 反检测 | `get_fingerprint_info` `check_detection` `bypass_debugger_trap` | 指纹与反检测 |
| 源码级插桩 | `instrumentation(action='install', ...)` `instrumentation(action='log', ...)` `check_environment()` `instrumentation(action='stop', ...)` `search_code(keyword='switch', script_url=..., context_chars=500)` | **[v0.4.0]** 通用 JSVMP 逆向利器 | <!-- v3.1.0: migrated from instrument_jsvmp_source, get_instrumentation_log, get_instrumentation_status, stop_instrumentation, find_dispatch_loops -->
| Cookie 归因 | `analyze_cookie_sources` | **[v0.4.0]** HTTP Set-Cookie + JS 写入融合分析 |
| 导航增强 | `navigate(pre_inject_hooks)` `instrumentation(action='reload')` `get_runtime_probe_log` | **[v0.4.0]** 首屏挑战/hook 重注入/运行时探针 | <!-- v3.1.0: migrated from reload_with_hooks -->
