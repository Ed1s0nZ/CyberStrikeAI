> ⚠️ v3.3.0 起，本文档的核心经验法则（24 条）已回迁到 SKILL.md 核心层。
> 本文档保留作为深度背景参考，但**以 SKILL.md 为准**。如发现不一致，以 SKILL.md 为准。

# 经验法则完整版（30 条）

> **说明**：SKILL.md 核心层保留了前 10 条最常用的。本文档是完整的 30 条
>
> **版本**：v3.1.0（MCP v0.9.0 工具名迁移）
>
> **来源**：v3.0.0 从 46 条压缩合并而来，基于三次真实站点（瑞数/TikTok/抖音）实战数据

---

## 一、识别与选择（5 条）

### 规则 1：反爬类型识别是 Phase 0 的 Phase 0

> 合并原 #28

不加任何 hook 先 navigate，观察 redirect_chain / initial_status / 加载的 JS 特征，三分法判定签名型/行为型/纯混淆。

**用错档的工具不是效率差，是根本跑不通。**

```
MCP 操作：
  navigate(url)  // 不传 pre_inject_hooks
  → 读 initial_status / final_status / redirect_chain
  → 按三分法判断表判定类型
```

### 规则 2：JSVMP 双路径决策

> 合并原 #11, #29, #30

识别到 JSVMP 后先判断走路径 A（算法追踪）还是路径 B（环境伪装）。

签名型反爬只有一条路：`instrumentation(action='install', mode="ast")` <!-- v3.1.0: migrated from instrument_jsvmp_source -->，源码级插桩不动环境，是唯一能同时"观察 VMP"和"让挑战通过"的手段。

`hook_jsvmp_interpreter(mode="transparent")` 是 `mode="proxy"` 的签名安全备选，源码级插桩失败再退到这里。

### 规则 3：Cookie 归因优先于 setter hook

> 合并原 #1, #25

分析动态 Cookie 第一步永远是 `analyze_cookie_sources()`，它能区分三种模式：
- 纯 JS 写入
- 纯 HTTP Set-Cookie
- JS 算 token + 服务端带回来

RS/Akamai 最常见的第三种模式下，单纯 hook `document.cookie` setter 什么都抓不到。

### 规则 4：`pre_inject_hooks` 的正确定位

> 合并原 #26, #30

- **行为型反爬**：可用，首屏挑战页（RS 412、Akamai 首包）必须用它在 navigate 时装好 hook
- **签名型反爬**：**不可用**，永远不要对签名型反爬用它

### 规则 5：经验案例优先匹配 cacheOpts 区分变体

> 原 #40

同一 SDK 体系存在多个变体（单签名 vs 双签名、bdms.paths vs cacheOpts），Phase 0.5 指纹匹配时应优先检测 `cacheOpts` 和 `X-Gnarly` 来区分。

---

## 二、工具技巧（10 条）

### 规则 6：`get_request_initiator` 是黄金路径

> 原 #9

看到加密参数 → 获取请求 ID → `get_request_initiator(request_id=N)` → 直达签名函数，省去大量搜索。

### 规则 7：`inject_hook_preset` 一键到位

> 原 #10

不要手写常见 Hook，预设模板覆盖 xhr/fetch/crypto/websocket/debugger_bypass/cookie/runtime_probe。

### 规则 8：源码级插桩优先于运行时 hook（对 VM 自包含场景）

> 合并原 #23, #24

RS 5/6、Akamai sensor_data、webmssdk 这类"算法全部在 opcode dispatch 循环内"的 VMP，`hook_jsvmp_interpreter` 仍然看不到 switch/case 内部。

```
MCP 操作：
  instrumentation(action='install', url_pattern="**/<VMP文件>", mode="ast", tag="vmp1")
  <!-- v3.1.0: migrated from instrument_jsvmp_source -->
  → 是唯一能打开黑箱的工具

  instrumentation(action='log', tag_filter='vmp1', type_filter='tap_get', limit=200)
  <!-- v3.1.0: migrated from get_instrumentation_log -->
  → hot_keys 指纹学习法 30 秒就能告诉你 VMP 读了哪些环境属性
```

### 规则 9：`instrumentation(action='reload')` 取代裸 `reload`

> 原 #27

装完 hook 想让它先于页面 JS 跑，裸 `reload()` 不能保证顺序。

```
MCP 操作：
  instrumentation(action='reload')
  <!-- v3.1.0: migrated from reload_with_hooks -->
  → 一步到位，默认 clear_log=True 拿到干净快照
```

### 规则 10：`search_code(keyword, script_url=url)` 定位大文件

> 原 #16

JSVMP 文件通常 200KB+，用 `search_code(keyword, script_url=url)` <!-- v3.1.0: migrated from search_code_in_script --> 在指定脚本中搜索，获取前后上下文。

### 规则 11：`compare_env` 是补环境的起点

> 原 #17

先在 Camoufox 中采集环境基准数据，再用 `evaluate_js` 分批采集细粒度值（compare_env 不覆盖 Function.toString / Symbol.toStringTag / DOM 布局），与 jsdom 逐项 diff。

### 规则 12：双签名场景必须同时 Hook XHR 和 fetch

> 原 #33

某些平台的 JSVMP 同时修改 `XMLHttpRequest.prototype.open` 和 `window.fetch`，只 Hook 一个通道会丢失另一半签名。

```
MCP 操作：
  inject_hook_preset("xhr", persistent=True)
  inject_hook_preset("fetch", persistent=True)
  → 两个通道都要 Hook
```

### 规则 13：MCP 侧 AST 让插桩在挑战页可用

> 原 #32

v0.5.0 把 AST 从页面内 Acorn 改为 MCP 侧 esprima，RS 412 挑战页也能跑。

`instrumentation(action='install', ...)` <!-- v3.1.0: migrated from instrument_jsvmp_source --> 的 `last_mode_used` 长期是 `"regex (fallback)"` 说明目标有太新语法。

### 规则 14：CSP 策略可能阻断 AST 模式插桩

> 原 #37

严格的 CSP 可能阻止改写代码执行。当前统一接口没有 `csp_bypass` 参数；若运行时健康检查失败，应停止该插桩并降级到更保守的观测方式。

### 规则 15：`navigate` 的 `collect_response_chain` 默认开启

> 原 #39

v0.6.0 起默认记录完整响应链，对签名型反爬的 412→200 判断更可靠。

---

## 三、Hook 与环境伪装踩坑（8 条）

### 规则 16：环境伪装优先于算法追踪

> 原 #18

如果 JSVMP 只是一个「签名黑箱」且可以在 jsdom 中加载执行，优先走路径 B（采集→对比→补丁），比追踪字节码执行快 10 倍。

### 规则 17：Function.prototype.toString 是 jsdom 环境伪装的第一杀手

> 原 #19

jsdom 所有 DOM 方法的 toString() 会暴露实际 JS 代码，必须用 WeakSet + 实例级覆写 + 源码模式正则三层防御。

### 规则 18：环境对比要分批采集

> 原 #20

单次 evaluate_js 代码太长会报错，分 4-5 批：
- navigator
- screen + window
- document + performance + toString
- DOM + Canvas + WebGL + Audio

### 规则 19：jsdom 环境补丁必须在 JSVMP 脚本加载前完成

> 原 #21

XHR Hook 的安装顺序决定能否截获最终 URL。

### 规则 20：服务端静默拒绝是环境检测失败的信号

> 原 #22

返回 HTTP 200 + 空 body（不报错），说明签名格式正确但环境指纹不匹配。

### 规则 21：Firefox 格式 native code 与 Chrome 不同

> 原 #35

Camoufox 基于 Firefox 内核，`Function.prototype.toString` 对原生函数返回含换行缩进格式。

```
Chrome: "function name() { [native code] }"
Firefox: "function name() {\n    [native code]\n}"
```

jsdom 的 markNative 必须匹配采集基准浏览器的格式。

### 规则 22：cacheOpts 是新版 SDK 初始化的必传项

> 原 #34

旧版只需 `bdms.paths`，新版必须同时传入 `cacheOpts`。缺少 cacheOpts 会导致业务路径未注册，拦截器不触发。

### 规则 23：环境伪装前先确认签名函数入口

> 原 #38

在开始 6 步法的环境采集之前，先用 `search_code` 确认 JSVMP 的签名入口类型：
- 单通道 XHR
- 双通道 XHR + fetch
- 导出函数
- cacheOpts 初始化

---

## 四、协议优先反模式（7 条）

### 规则 24：先做最小补环境，不要上来就开浏览器

> 原 #5

协议优先的第一步。能用 Node.js `crypto` 解决的，不要用 `vm` 沙箱；能用 `vm` 沙箱的，不要用 jsdom；能用 jsdom 的，不要用浏览器。

### 规则 25：TLS 指纹是终极壁垒

> 合并原 #6, #36

算法全对但仍失败时，考虑 TLS 指纹：
- Node.js 用 `got-scraping` 模拟 Firefox/Chrome TLS 指纹
- Python 用 `curl_cffi`

### 规则 26：签名不一致时逐环节对比

> 原 #13

排查链路（逐项对比脚本值 vs 浏览器值）：
1. 原始输入参数
2. 参数排序/拼接字符串
3. 时间戳（精度：秒 vs 毫秒）
4. 随机串（长度、字符集）
5. 密钥/盐值
6. 中间摘要
7. 最终密文（编码方式：hex/base64/自定义）

找到第一个偏差点。

### 规则 27：预热请求不是装饰

> 原 #2

`/api2` 类请求在运行时注入关键变量。跳过预热请求可能导致签名缺少必要的动态密钥。

### 规则 28：Node vm 沙箱 ≠ 浏览器

> 原 #3

有些调试干扰机制只在非浏览器环境触发。vm 沙箱中可能遇到：
- `window` 未定义
- `document` 未定义
- `navigator` 未定义
- 定时器行为不同

### 规则 29：降级梯度必须逐级走

> 合并原 #43, #44

```
instrumentation(action='install', mode="ast")
  → 失败
instrumentation(action='install', mode="regex")
  → 覆盖率不足
hook_jsvmp_interpreter(mode="transparent")
  → 日志太少
hook_jsvmp_interpreter(mode="proxy")    ← 仅行为型可用
  → 破坏签名
路径 B（jsdom 环境伪装）
  → 也失败
向用户说明情况
```

每级至少尝试一次并记录失败原因。想放弃时先回查 cases/ 和 common-pitfalls.md。

### 规则 30：case 文件的价值随实战次数指数级增长

> v3.2.0 改写（原"Session 档案与经验库互补"）

第一次分析某站点时写的 case 可能粗糙；第二次分析（升级或变体）时用 case 发现 80% 还成立、20% 变了，就把变化追加到 case 的"变体章节"。三五次之后，这个 case 会覆盖该站点的主要变体谱系，复用价值极高。

case 文件的"可验证事实清单"段列出 5-15 条最小可验证事实（如"X-Bogus 长度 28""webdriver === false"），下次同站升级时逐条手动核对找出"哪些变了"就是今天的工作范围。

---

## 已合并/迁移的旧经验法则索引

| 原编号 | 原标题 | 去向 |
|--------|--------|------|
| #1 | Cookie setter 最高性价比 | → 合并到新 #3（Cookie 归因优先） |
| #4 | WASM panic 常常是环境缺失 | → 移到 references/troubleshooting.md |
| #7 | 真假请求链是常见干扰 | → 移到 references/troubleshooting.md |
| #8 | HTTP/2 可能是唯一的"密码" | → 移到 references/troubleshooting.md |
| #11 | JSVMP 路径选择 | → 合并到新 #2 |
| #12 | JSVMP 中 String.fromCharCode 是高频信号 | → 移到 references/jsvmp-analysis.md |
| #14 | Python execjs 复用 context | → 移到 references/workflow-overview.md |
| #15 | Hook 必须持久化 + 防覆盖 | → 已在工具说明中覆盖 |
| #23 | 源码级插桩优先 | → 合并到新 #8 |
| #24 | hot_keys 指纹学习法 | → 合并到新 #8 |
| #25 | Cookie 归因 | → 合并到新 #3 |
| #26 | pre_inject_hooks 定位 | → 合并到新 #4 |
| #28 | 反爬类型识别 | → 合并到新 #1 |
| #29 | 签名型只走源码插桩 | → 合并到新 #2 |
| #30 | transparent 备选 | → 合并到新 #2, #4 |
| #41 | 开 session 多 10 秒省几小时 | → 已合并到硬约束 Checklist CHECK-4 |
| #42 | 进新域第一反应 list_sessions | → 已合并到 Checklist CHECK-3 |
| #43 | 降级梯度逐级走 | → 合并到新 #29 |
| #44 | 过期不等于失效 | → 合并到新 #29 |
| #45 | 断言价值指数递增 | → 合并到新 #30 |
| #46 | Session 与经验库互补 | → 合并到新 #30 |

---

> **SKILL.md 核心层**保留了前 10 条最常用的经验法则（规则 1-10 的精简版）
> **完整 30 条**在本文档
> **反模式清单**：references/common-pitfalls.md
