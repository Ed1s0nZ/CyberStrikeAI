# JS 逆向工作流总览

## 解法模式决策树

```
目标网站数据接口
    │
    ├─ 能否抓到明确的数据接口？
    │   ├─ 否 → WebSocket？→ 场景7
    │   │       页面渲染数据？→ 场景8（字体反爬/CSS干扰）
    │   └─ 是 ↓
    │
    ├─ 请求中有加密参数？
    │   ├─ 否 → 直接请求即可，检查是否有 Cookie/Header 校验
    │   └─ 是 ↓
    │
    ├─ 加密逻辑在哪里？
    │   ├─ 明文 JS（可直接定位）→ 模式A：纯 Node.js 还原
    │   ├─ 混淆 JS（OB/CFF/eval）→ 反混淆后 → 模式A 或 模式B
    │   ├─ 服务端返回 JS 动态执行 → 模式B：VM 沙箱执行
    │   ├─ WASM 二进制 → 模式C：WASM 加载
    │   ├─ JSVMP 虚拟机保护 → 先走四板斧（Hook/插桩/日志/源码级插桩） → 失败则 → 模式D/E
    │   │    · RS 5/6、Akamai、webmssdk、obfuscator.io → 第四板斧源码级插桩最有效
    │   │    · 深度绑定环境指纹 → 模式E（jsdom 环境伪装）
    │   └─ 无法脱离浏览器环境 → 模式D：浏览器自动化
    │
    ├─ 纯请求方案是否可行？
    │   ├─ 请求成功 → 完成
    │   └─ 请求失败（token failed / 403）↓
    │
    └─ TLS 指纹 / HTTP/2 检测？
        ├─ 是 → 切换到模式D 或使用 HTTP/2
        └─ 否 → 检查 Cookie、时间戳、参数拼接错误
```

## 模式详解

### 模式 A：纯 Node.js 算法还原

**适用条件**：
- 加密算法可用标准库实现（MD5/SHA/AES/DES/RSA/HMAC）
- 无浏览器环境检测
- 无 TLS 指纹校验

**技术栈**：
- `crypto`（Node.js 内置）— MD5/SHA/AES/DES/HMAC
- `crypto-js` — CryptoJS 兼容实现
- `node-rsa` / `node-forge` — RSA 加解密
- `axios` / `node-fetch` — HTTP 请求
- `http2`（Node.js 内置）— HTTP/2 请求

**模板**：`templates/node-request/`

### 模式 B：Node vm 沙箱执行

**适用条件**：
- 服务端返回混淆 JS，需要 eval 执行生成 Cookie/Token
- 加密逻辑依赖大量上下文，手动还原成本过高

**技术栈**：
- `vm`（Node.js 内置）— 沙箱执行
- `vm2` — 更安全的沙箱（可选）
- `jsdom` — 模拟 DOM 环境（可选）

**注意事项**：
- 必须构造最小沙箱环境（document、navigator、location 等）
- Cookie setter 需要拦截
- setTimeout/setInterval 需要提供但可能需要限制

**模板**：`templates/vm-sandbox/`

### 模式 C：WASM 加载还原

**适用条件**：
- 加密函数通过 WebAssembly 实现
- WASM 有明确的导出函数接口

**技术栈**：
- `fs`（Node.js 内置）— 读取 .wasm 文件
- `WebAssembly`（Node.js 内置）— 加载和实例化

**注意事项**：
- 先验证 I/O，不要先反编译 WASM
- 检查 `WebAssembly.Module.imports()` 了解环境依赖
- wasm-bindgen 生成的 WASM 需要补 `Window` 类和 DOM
- `unreachable` panic 通常是环境缺失

**模板**：`templates/wasm-loader/`

### 模式 D：浏览器自动化

**适用条件**：
- TLS 指纹检测，纯请求无法通过
- 复杂环境依赖（window 蜜罐、Canvas 指纹等）
- 加密逻辑过于复杂，还原成本远大于自动化

**技术栈**：
- `playwright-core` — 浏览器自动化
- `camoufox-reverse` MCP — Camoufox 反检测浏览器、Hook、网络捕获、源码搜索

**注意事项**：
- 优先使用 Camoufox 独立浏览器实例，避免污染用户日常浏览环境
- 通过 `inject_hook_preset` / `add_init_script` 注入脚本截取中间数据
- 控制请求频率并结合 `check_detection` 验证反检测效果

**模板**：`templates/browser-auto/`

## 通用调试流程

```
1. 启动 Camoufox 并开启网络捕获
   ↓
2. 触发目标操作（搜索/翻页/登录）
   ↓
3. 找到数据接口，分析参数
   ↓
4. 用 `search_code` 搜索参数名定位 JS 源码
   ↓
5. 用 `get_request_initiator` / `set_breakpoint_via_hook` 追踪调用栈
   ↓
6. 在关键函数设置伪断点或 `trace_function`
   ↓
7. 单步调试确认加密逻辑
   ↓
8. 还原算法并对比中间值
   ↓
9. 编写 Node.js / Python 脚本
   ↓
10. 运行验证
```

---

## JSVMP 分支专属流程（v2.5.0 新增）

识别到 JSVMP（200KB+ 单文件 + while-switch 分发循环 + 字节码数组）时，不要走通用流程的第 4-7 步，改走下面这个分支：

```
1. launch_browser(headless=False) + start_network_capture(capture_body=True)
   ↓
2. 导航前判断是否首屏挑战
   ├─ 是（RS 412 / Akamai 403 等）
   │   → navigate(url=..., pre_inject_hooks=["xhr","fetch","cookie","jsvmp_probe"], via_blank=True)
   └─ 否
       → navigate(url=...) + 之后装 hook + reload_with_hooks()
   ↓
3. find_dispatch_loops(script_url=<VMP_URL>) 确认 VMP（case_count > 50）
   ↓
4. 同时装三路探针：
   ├─ 第一板斧（I/O 边界）：inject_hook_preset(cookie / xhr / fetch / crypto, persistent=True)
   ├─ 第二板斧（解释器）：hook_jsvmp_interpreter(script_url=<VMP basename>)
   └─ 第四板斧（源码级插桩，核心）：instrument_jsvmp_source(
                                     url_pattern="**/<VMP 文件>",
                                     mode="ast", tag="vmp1")
   ↓
5. reload_with_hooks() 让探针先于 VMP 生效 + 清日志
   ↓
6. 触发业务操作，让 VMP 生成一次签名
   ↓
7. 第三板斧读日志，分 3 个维度看：
   ├─ get_instrumentation_log(tag_filter="vmp1") → hot_keys / hot_methods / hot_functions
   ├─ get_jsvmp_log() → API 调用 + 属性读取摘要
   ├─ analyze_cookie_sources() → cookie 归因（HTTP vs JS）
   └─ get_console_logs() → xhr/fetch/cookie 预设的输出
   ↓
8. 根据 hot_methods 选还原策略：
   ├─ 包含标准加密 API（CryptoJS.MD5 / Subtle.digest 等） → 模式 A 纯算法还原
   ├─ 全自定义 + hot_keys 少 → 模式 B 提取函数 + vm/execjs 沙箱
   ├─ 全自定义 + hot_keys 多 + cookie 走 HTTP → 模式 E jsdom 环境伪装
   └─ 以上都不行 → 模式 D 浏览器自动化（最后手段）
   ↓
9. 实现还原代码
   ↓
10. 连续 5+ 次请求验证稳定性
   ↓
11. stop_instrumentation() + remove_hooks() 清理
```

参考文档：
- `references/jsvmp-analysis.md`：四板斧方法论总纲
- `references/jsvmp-source-instrumentation.md`：第四板斧源码级插桩专项（v2.5.0 新）
- `references/jsdom-env-patches.md`：模式 E 环境伪装补丁库
