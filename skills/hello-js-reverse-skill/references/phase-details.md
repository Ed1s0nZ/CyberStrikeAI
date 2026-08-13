> ⚠️ v3.3.0 起，本文档的核心动作清单已回迁到 SKILL.md 核心层。
> 本文档保留作为深度背景参考，但**以 SKILL.md 为准**。如发现不一致，以 SKILL.md 为准。

# Phase 0-5 详细操作手册

> **触发条件**：执行某个 Phase 不确定具体怎么做时读
>
> **前置要求**：已完成硬约束 Checklist 五项复述（见 SKILL.md 顶部）
>
> **版本**：v3.1.0（MCP v0.9.0 工具名迁移）

---

## Phase 0：任务理解与调试环境搭建

### 0.1 任务理解

收到用户的目标 URL 和分析需求后：

1. **明确分析目标**：需要还原哪些加密参数、目标数据是什么
2. **接口分析**：
   - 梳理请求的 URL、Method、Headers、Params、Body
   - 识别并标记签名/动态参数（如 `sign`、`token`、`timestamp`、`nonce`）
   - 根据参数特征（长度、字符集、结构）给出算法初步判断
   - 以简洁表格汇总分析结果，与用户确认

### 0.2 浏览器搭建

根据用户是否提供 Cookie，选择对应路径：

#### 路径 A：带 Cookie 启动

```
MCP 操作：
  launch_browser(headless=false)
  → 启动反检测浏览器（有头模式，方便用户观察）

  navigate(url="目标URL")
  → 导航到目标页面

  cookies(action='set', cookies=[...])
  <!-- v3.1.0: migrated from set_cookies -->
  → 写入 Cookie

  reload()
  → 刷新使 Cookie 生效

  evaluate_js(expression="document.cookie")
  → 验证写入成功
```

#### 路径 B：直接启动（无 Cookie）

```
MCP 操作：
  launch_browser(headless=false)
  navigate(url="目标URL")
  → 直接进入后续分析阶段
```

### 0.3 Cookie 写入方法

用户提供的 Cookie 可能有多种格式：

| 格式 | 处理方式 |
|------|---------|
| Cookie 字符串 `"k1=v1; k2=v2"` | 拆分后构造 `[{name:"k1", value:"v1", domain:".example.com", path:"/"}]` |
| JSON 格式（EditThisCookie 导出） | 直接传入 `cookies(action='set', ...)` <!-- v3.1.0: migrated from set_cookies --> |
| Request Headers 中的 Cookie 字段 | 按 `"; "` 拆分后构造数组 |

### 0.4 项目目录创建

以目标网站/功能命名，结构参考 `templates/` 下的模板：

**Node.js 项目：**
```
project_name/
├── config/
│   ├── encrypt.js          # 提取的原始 JS 加密代码
│   ├── keys.json           # 密钥、IV、Salt 等配置
│   └── headers.json        # 请求头模板
├── utils/
│   ├── encrypt.js          # 加密参数生成函数
│   └── request.js          # 请求封装
├── main.js                 # 主脚本
├── package.json
└── README.md
```

**Python 项目：**
```
project_name/
├── config/
│   ├── sign_logic.js       # 提取的签名逻辑代码
│   ├── keys.json           # 密钥配置
│   └── headers.json        # 请求头模板
├── utils/
│   ├── sign.py             # 签名函数封装
│   └── request.py          # 请求封装
├── main.py
├── requirements.txt
└── README.md
```

---

## Phase 0.5：经验库命中验证

> ⚠️ 本步骤在硬约束 Checklist 的 [CHECK-2] + [CHECK-3] 中已经预执行过。
> 这里是 Phase 0 完成后做二次确认和深入。

### 四分支决策

```
┌─ [CHECK-2] 命中案例？
│   └─ YES → 分支 1：命中 cases/
│
├─ [CHECK-2] 命中工作区 site_* 目录？
│   └─ YES → 分支 2：命中 site_*
│
├─ [CHECK-3] 历史 session 存在？
│   └─ YES → 分支 3：有历史 session
│
└─ 全部未命中 → 分支 4：全未命中（30 秒指纹采集）
```

### 分支 1：命中 cases/

```
MCP 操作：
  readFile("cases/<命中案例>.md")
  → 详读已验证定位路径
  → 按该案例的 Phase 1-5 执行（不要自创流程）
  → 加 1-2 条本次站点特有的断言到 session（区别已有变体）
```

### 分支 2：命中 site_* 目录

```
MCP 操作：
  listDirectory("site_<target>/")
  → 识别 sign_service.js / cookie_fetcher.js / parser.js / keys.json 等文件

  若有 sign_service.js（已有协议方案）：
    → 直接基于 sign_service.js 建新目录复用
    → 不要从零写新方案
    → 不要用 Playwright/Camoufox 过挑战（违反红线 3）

  若只有 cookie_fetcher.js（浏览器兜底方案）：
    → 警告：这是违反红线 3 的历史残留
    → 本次目标应是替换为协议方案
```

### 分支 3：有历史 session

```
MCP 操作：
  attach_domain_readonly("<eTLD+1>")     # 在 CHECK-3 已做
  verify_assertion(assertion_id='*')     # 批量验证所有断言
  <!-- v3.1.0: migrated from reverify_all_assertions_on_domain -->

  根据 triage.interpretation：
    "all assertions still hold"
      → 跳到 Phase 5 直接验证旧协议代码

    "Minor update"
      → 工作范围 = failed_ids，只修这几条

    "Moderate L4 change"
      → Phase 1-4 走一遍，但 passed 的是 invariants

    "Major L4 upheaval"
      → 重做，但 passed 断言仍是不变量
```

### 分支 4：全未命中（30 秒指纹采集）

```
MCP 操作（这些命令在 CHECK-2 时已跑过，这里是深入）：

  # JSVMP / SDK 特征识别
  search_code(keyword="webmssdk")        # webmssdk 家族
  search_code(keyword="byted_acrawler")  # webmssdk 家族
  search_code(keyword="_sdkGlueInit")    # 抖音特征
  search_code(keyword="cacheOpts")       # TikTok 特征
  search_code(keyword="sdenv")           # 瑞数 RS 特征
  search_code(keyword="meta-12")         # RS 6 特征
  search_code(keyword="FSSBBIl1UgzbN7N") # RS cookie 特征

  # 签名参数特征识别
  search_code(keyword="a_bogus")         # 抖音
  search_code(keyword="X-Bogus")         # TikTok 国际版
  search_code(keyword="X-Gnarly")        # TikTok 国际版
  search_code(keyword="acw_sc__v2")      # Aliyun WAF

  # 环境检测 SDK
  search_code(keyword="acmescripts")     # Akamai

  # 反爬类型关键字
  get_network_request(request_id=<first_request>)
  list_network_requests(resource_type="script")
```

采集完后，**必须**在本次分析结束时沉淀新案例：
1. 按 `cases/_template.md` 格式建立 `cases/<新案例名>.md`
2. 更新 `cases/README.md` 的"高频站点速查表"追加一行
3. 更新 `cases/README.md` 的"变体关系图"（如有兄弟案例）

---

## Phase 1：目标侦察

### 1.1 确认调试页面状态

```
MCP 操作：
  take_screenshot()
  → 截取当前页面视觉状态，确认页面正常

  如需导航到特定子页面：
  navigate(url="目标子页面")

  如果涉及登录态，确认 Cookie 已写入且页面内容正确
```

### 1.2 网络请求捕获

```
MCP 操作：
  network_capture(action='start')
  <!-- v3.1.0: migrated from start_network_capture -->
  → 开始捕获网络流量

  evaluate_js / click / type_text
  → 触发翻页/交互，产生请求

  list_network_requests()
  → 获取捕获的请求列表（支持过滤）

  get_network_request(request_id=N)
  → 获取关键接口的详细信息

  get_request_initiator(request_id=N)
  → 获取发起请求的 JS 调用栈（黄金路径！）

  重点关注：
    - Request URL、Method
    - Request Headers（Cookie、自定义签名头）
    - Query Params / Request Body（识别加密参数）
    - Response 数据结构
    - Initiator Stack（直接定位加密函数）
```

### 1.3 加密参数识别

对比多次请求，分析每个参数：

| 参数类型 | 特征 | 处理方式 |
|----------|------|---------|
| 固定值 | 每次请求相同 | 直接硬编码或从页面提取 |
| 动态值 | 有规律变化 | 判断变化因子（时间戳、页码、随机数、自增） |
| 加密值 | 看似随机 | 根据长度、字符集、格式判断算法类型 |

### 1.4 输出侦察报告

```
📋 目标信息
━━━━━━━━━━━━━━━━━━━━━━━━
目标网站：[URL]
分析目标：[需要还原的加密逻辑]
数据接口：[API endpoint]

🔗 接口参数分析
━━━━━━━━━━━━━━━━━━━━━━━━
URL：[完整请求URL]
Method：GET/POST
Headers：
  - Cookie: [关键字段及示例]
  - [自定义头]: [示例值]
加密参数：
  - 参数名: [名称] | 示例值: [值] | 长度: [N] | 字符集: [hex/base64/...] | 初步猜测: [算法]

📊 响应数据样本
━━━━━━━━━━━━━━━━━━━━━━━━
[前2-3条数据]

🧠 技术分析要点
━━━━━━━━━━━━━━━━━━━━━━━━
本目标涉及的签名分析技术点：
  1. [如：OB混淆还原]
  2. [如：动态Cookie生成]
  3. [如：AES-CBC加密]
```

---

## Phase 2：源码分析

### 2.1 关键词搜索定位

```
MCP 操作：
  search_code(keyword="加密参数名")
  → 直接在已加载的 JS 源码中搜索

  search_code(keyword="encrypt|sign|token|md5|sha|aes|des|rsa|hmac|btoa|atob|CryptoJS")
  search_code(keyword="XMLHttpRequest|$.ajax|fetch|beforeSend")
  search_code(keyword="document.cookie")

  根据搜索结果：
  scripts(action='get', url='<脚本URL>')
  <!-- v3.1.0: migrated from get_script_source -->
  → 读取包含加密逻辑的源码片段

  scripts(action='save', url='<脚本URL>', save_path='./config/target.js')
  <!-- v3.1.0: migrated from save_script -->
  → 保存关键脚本到本地分析
```

### 2.2 代码混淆识别

| 混淆类型 | 特征 | 还原策略 |
|---------|------|---------|
| OB 混淆 | `_0x` 前缀变量、十六进制字符串数组 | 字符串解密 + 变量重命名 |
| 控制流平坦化 | `switch-case` 状态机、`while(true)` 循环 | 追踪状态转移还原执行顺序 |
| eval/Function 打包 | `eval(...)` 或 `new Function(...)` 包裹 | Hook eval/Function 拦截源码 |
| JSVMP | 200KB+ 文件、自定义解释器 | 不反编译，走路径 A 或路径 B |

### 2.2+ JSVMP 专项分析

当识别到 JSVMP 时，**严禁尝试反编译字节码**。

#### JSVMP 识别标志

- 超大 JS 文件（200KB+），函数/变量名完全无意义
- 包含自定义解释器循环：`while(true) { switch(opcode) { ... } }`
- 改写或劫持浏览器原生 API（XHR / fetch / Cookie）
- 超大数组（字节码）+ 指针变量 + 栈操作 + 跳转指令

#### 路径选择决策树

```
决策树：
  ├─ JSVMP 是否劫持了请求链路（XHR/fetch 拦截器）？
  │   ├─ YES + 算法与环境指纹深度绑定（如 a_bogus）
  │   │   → 优先选路径 B（环境伪装）
  │   │   → 路径 B 失败时回退路径 A
  │   └─ YES 但签名逻辑相对独立
  │       → 路径 A（算法追踪），提取签名函数
  │
  ├─ JSVMP 仅生成签名参数（不劫持请求）？
  │   ├─ Hook 确认使用标准算法 → 路径 A，纯算法还原
  │   └─ 算法完全自定义 + 环境依赖重 → 路径 B
  │
  └─ 无法判断 → 先快速测试路径 B（30 分钟），不行再走路径 A
```

#### 反爬类型前置判断

```
先判断 JSVMP 所在反爬类型：

┌─ navigate(url) 不加任何 hook，看 redirect_chain
│
├─ redirect_chain 反复 412 然后才到 200
│   → 签名型 JSVMP
│   → 路径 A 只能走第四板斧（源码级插桩）
│   → 前三板斧禁用（会破坏签名）
│
├─ redirect_chain 直接 200 但页面后续 XHR 带签名参数
│   → 行为型 JSVMP
│   → 路径 A 四板斧全开
│
└─ redirect_chain 直接 200 无特殊签名参数
    → 不是签名型也不是 JSVMP，走混淆还原
```

#### 调用链追踪

```
MCP 操作：
  inject_hook_preset(preset="xhr")    → 一键注入 XHR Hook
  inject_hook_preset(preset="fetch")  → 一键注入 Fetch Hook
  reload()                            → 刷新页面触发 Hook
  get_request_initiator(request_id=N) → 获取 JS 调用栈（黄金路径）
  → 从调用栈中逐层定位：请求发送 → 参数构造 → 加密函数 → 密钥/明文来源
  set_breakpoint_via_hook(target_function="加密函数名")
  get_breakpoint_data()               → 获取捕获的参数、返回值和调用栈
```

#### 提取核心逻辑

```
MCP 操作：
  scripts(action='save', url='<脚本URL>', save_path='./config/target.js')
  <!-- v3.1.0: migrated from save_script -->
  → 保存完整脚本

  手动提取关键函数到 config/encrypt.js
  用中文注释标注每个函数的作用、输入输出
```

### 2.2++ 静态分析关键判断清单

在源码分析阶段，必须确认以下内容：

- [ ] 参数是单独加密还是整条请求链被接管（URL 重写 / 请求劫持）
- [ ] 页码、时间戳、随机数、Cookie、UA、环境变量是否参与运算
- [ ] 是否存在响应解密（接口返回加密字符串而非明文 JSON）
- [ ] 是否存在运行时代码生成（`eval` / `new Function`）
- [ ] 是否有前置请求（预热接口、Token 获取接口）
- [ ] 是否有请求链改写（拦截 XHR/fetch 添加签名头）

---

## Phase 3：动态验证

对静态分析的结论，在调试浏览器页面上进行运行时验证。

### 3.1 Hook 注入验证

```
MCP 操作：
  inject_hook_preset(preset="xhr")     → XHR Hook
  inject_hook_preset(preset="fetch")   → Fetch Hook
  inject_hook_preset(preset="crypto")  → 加密函数 Hook（btoa/atob/JSON.stringify）

  hook_function(
    function_path="自定义目标",
    hook_code="...",
    position="before|after|replace"
  )
  → 自定义 Hook

  reload()
  → 刷新触发 Hook

  get_console_logs()
  → 读取 Hook 输出
```

### 3.2 断点与追踪确认

```
MCP 操作：
  set_breakpoint_via_hook(target_function="加密函数路径")
  → 通过 JS Hook 设置伪断点

  hook_function(
    function_path="加密函数路径",
    mode='trace',
    log_args=True, log_return=True, log_stack=True
  )
  <!-- v3.1.0: migrated from trace_function -->
  → 追踪函数调用（不暂停执行）

  触发目标操作：
  evaluate_js / click / type_text → 模拟交互

  get_breakpoint_data()   → 获取伪断点捕获的参数和返回值
  get_trace_data()        → 获取函数追踪数据

  重点确认：
    - 加密算法的具体模式（AES 的 ECB/CBC、填充方式、密钥长度）
    - 参数拼接顺序和格式
    - 时间戳精度（秒 vs 毫秒）
    - 密钥/IV 的来源（硬编码 vs 服务端返回 vs 动态计算）
```

### 3.3 多次请求对比

```
MCP 操作：
  evaluate_js / click → 触发多次数据请求
  list_network_requests() → 收集捕获的网络请求
  → 对比加密参数变化规律，确认变化因子
```

---

## Phase 4：算法还原

### 4.1 语言选择策略

| 维度 | 选 Node.js | 选 Python |
|------|-----------|-----------|
| 加密逻辑复杂度 | 自定义逻辑可直接用 `vm` 沙箱执行原始 JS | 标准算法可直接用 Python 库还原 |
| 团队技术栈 | 用户/团队偏好 Node.js | 用户/团队偏好 Python |
| JSVMP 场景 | VM 沙箱可直接加载整个 VM | 需 `execjs` 桥接 |
| TLS 指纹需求 | 需额外配置 | `curl_cffi` 一行搞定 |

### 4.2 解法模式选择

| 模式 | 适用场景 | Node.js 模板 | Python 模板 |
|------|---------|-------------|-------------|
| A: 纯算法还原 | 加密逻辑可完整提取，无浏览器环境依赖 | `templates/node-request/` | `templates/python-request/` |
| B: 沙箱执行 JS | 服务端返回混淆 JS 用于生成 Cookie/Token | `templates/vm-sandbox/` | `templates/python-request/`（用 `execjs`） |
| C: WASM 加载还原 | 加密逻辑在 WebAssembly 中实现 | `templates/wasm-loader/` | — |
| D: 浏览器自动化 | TLS 指纹检测、复杂环境依赖、无法脱离浏览器 | `templates/browser-auto/` | — |
| E: jsdom 环境伪装 | JSVMP 深度绑定环境指纹、算法不可提取 | jsdom + references/jsdom-env-patches.md | — |

### 4.3 编码原则

1. **先通后全**：先成功请求到第 1 页/第 1 条数据，验证加密正确后再扩展
2. **优先纯算法**：标准算法 Node.js 用 `crypto` / `crypto-js`，Python 用 `hashlib` / `pycryptodome` / `hmac`
3. **中间值对比**：打印关键中间值（明文、密文、签名），与浏览器抓包值逐一比对
4. **配置外置**：密钥、Headers 模板等写入独立配置文件
5. **错误处理**：包含重试机制、频率控制、异常告警
6. **逐步验证**：每次只增加一个参数的实现，确保每步可独立验证
7. **代码可运行**：提供的代码必须是可直接复制运行的，不留占位符
8. **分析产物持久化**：长参数值、Cookie、JS 代码片段、请求样本等，第一时间写入 `config/` 目录

### 4.4 配置文件策略

| 产物类型 | 存放位置 | 说明 |
|---------|---------|------|
| Cookie 字符串 | `config/cookies.txt` 或 `config/cookies.json` | 完整 Cookie |
| 长参数样本 | `config/params_sample.json` | 捕获的请求参数样本（含签名值） |
| 提取的 JS 代码 | `config/sign_logic.js` / `config/encrypt.js` | 签名/加密函数及其依赖 |
| Headers 模板 | `config/headers.json` | 完整的请求头 |
| 响应样本 | `config/response_sample.json` | 接口返回的数据样本 |
| 密文样本 | `config/ciphertext_samples.txt` | 多次请求的加密参数值 |

**核心原则**：分析过程中产生的任何长文本都应该立即持久化到 `config/` 文件中。后续代码只需「读取文件」而非「内联长字符串」。

---

## Phase 5：断言驱动交付（v2.9.0 升级）

### 5.1 运行验证

```
MCP 操作：
  1. 运行 main.js / main.py，确认输出正确数据
  2. 与浏览器实际数据交叉验证（≥ 5 次请求，确认签名稳定性）
```

### 5.2 写入/更新域级 Session 档案

```
MCP 操作：
  a. 将本次分析的断言集写入 assertions.json
     至少包含：script_exists / anti_crawl_type / cookie_names

  b. 更新 session.json 的 hot_keys_snapshot 和 cookie_attribution_summary

  c. 更新 fingerprint.json（如有新的指纹采集数据）

  d. 更新 assertions_passed / assertions_failed / assertions_total

  e. 输出："域级 Session 档案已更新，N 条断言已写入，下次分析同域时将自动复用"
```

### 5.3 生成 README.md

记录以下内容：
- 目标信息与接口分析
- 加密逻辑还原过程
- 涉及的签名分析技术点
- 运行方式与依赖说明

### 5.4 经验沉淀

**主动询问用户是否沉淀经验**到 cases/（按 cases/_template.md 格式）。

沉淀内容包括：
- 反爬类型判定过程
- 关键技术点和踩坑记录
- 已验证的定位路径
- 环境补丁清单（如走路径 B）

### 5.5 断言驱动交付清单

| 交付项 | 必须 | 说明 |
|--------|------|------|
| 可运行的 main.js / main.py | ✅ | 纯协议脚本，无浏览器依赖 |
| config/ 目录 | ✅ | 密钥、Headers、JS 代码等配置 |
| README.md | ✅ | 项目说明 + 接口分析记录 |
| Session 档案更新 | ✅ | 断言集 + 指纹快照 + Cookie 归因 |
| cases/ 经验沉淀 | 推荐 | 主动询问用户 |
| ≥ 5 次请求验证 | ✅ | 确认签名稳定性 |

---

> **硬约束 Checklist**：SKILL.md 顶部
> **路径 A 详细操作**：SKILL.md「路径 A：算法追踪（四板斧）」段
> **路径 B 详细操作**：SKILL.md「路径 B：环境伪装（jsdom/vm 沙箱执行原始 JSVMP）」段
> **错误处理准则**：SKILL.md「错误处理准则」段
> **降级梯度表**：SKILL.md「AI 想放弃时的降级梯度表」段
