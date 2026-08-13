# JS 逆向分析 Skill

面向 Web 逆向分析与接口签名还原场景的 Skill，围绕 `camoufox-reverse` MCP 构建单一工作流：先用 Camoufox 反检测浏览器完成网络捕获、源码定位、Hook 调试与反检测验证，再按需落地到 Node.js 或 Python 算法还原与自动化调用。

## 核心能力

- **双语言算法还原**: Node.js (`crypto` / `crypto-js`) 与 Python (`hashlib` / `pycryptodome`) 双路径实现
- **加密算法还原**: MD5/SHA/AES/DES/RSA/HMAC/Base64 等常见加密算法的纯算法复现
- **JS 混淆还原**: OB混淆、控制流平坦化、eval打包、自定义VM/JSVMP 等混淆类型的分析策略
- **JSVMP 行为追踪**: Hook/插桩/日志/源码级插桩四板斧（v2.5.0 新增第四板斧），从行为层面追踪 JS 虚拟机保护的签名生成链路，对RS 5/6、Akamai sensor_data、webmssdk、obfuscator.io 通用有效（详见专项指南）
- **Cookie 归因分析**: `analyze_cookie_sources` 融合 HTTP Set-Cookie 与 JS document.cookie 日志，一次性解答"这个 Cookie 到底是谁写的"（v2.5.0 新增）
- **反爬类型分档决策（v2.6.0）**：签名型（RS/Akamai）/ 行为型（TK/JY）/ 纯混淆 三类反爬的工具路径完全不同，内置顶层决策框架，避免"observer effect 破坏签名"的经典陷阱
- **动态 Cookie 逆向**: eval首包、api预热、浏览器指纹等 Cookie 生成机制的破解
- **WASM 逆向**: WebAssembly 加密模块的加载、环境补全和调用
- **协议层对抗**: TLS 指纹检测、HTTP/2 强制、频率限制等协议层防护的应对
- **反检测浏览器**: Camoufox C++ 引擎级指纹伪装，绕过 Cloudflare/RS/JY等反爬检测
- **单 MCP 工作流**: 统一使用 `camoufox-reverse` 完成源码搜索、Hook 注入、函数追踪、网络分析、请求拦截与反检测验证
- **错误排查体系**: 完整的请求失败排查流程和签名值不一致排查方法论
- **Codex/Claude Code 完整适配**: 内置 Agent 授权指令框架，确保 AI 全力协助完成分析任务

## 快速使用

### 方式一：AI 对话框直接安装（推荐）

在你的 AI 编码工具（Cursor / Claude Code / codex 等）的对话框中输入：

```
帮我安装下这个skill：hello_js_reverse_skill
项目地址：https://github.com/WhiteNightShadow/hello_js_reverse_skill
```

AI 会自动完成下载、配置，并在后续 JS 逆向相关任务中自动调用该 Skill。

### 方式二：手动安装

将本仓库克隆到对应 AI 工具的 skills 目录下：

**Cursor：**

```bash
git clone https://github.com/WhiteNightShadow/hello_js_reverse_skill.git ~/.cursor/skills/hello_js_reverse_skill
```

**Claude Code（Codex CLI）：**

```bash
git clone https://github.com/WhiteNightShadow/hello_js_reverse_skill.git ~/.codex/skills/hello_js_reverse_skill
```

**VSCode（Copilot / Cline 等插件）：**

```bash
git clone https://github.com/WhiteNightShadow/hello_js_reverse_skill.git ~/.vscode/skills/hello_js_reverse_skill
```

> 安装完成后，AI Agent 会自动读取 `SKILL.md` 获取 JS 逆向分析能力。当你在对话中涉及接口签名分析、反爬对抗、动态 Cookie、混淆 JS、WASM 或浏览器环境调试等场景时，Skill 会自动激活。

## 项目结构


```
hello_js_reverse_skill/
├── SKILL.md                        # 技能定义（AI Agent 读取的核心文件）
├── README.md                       # 项目文档
│
├── references/                     # 参考文档（深度背景，按需读取）
│   ├── workflow-overview.md        # 工作流总览与解法模式决策树
│   ├── phase-details.md            # Phase 0-5 详细操作（v3.3.0 起以 SKILL.md 为准）
│   ├── mcp-cookbook.md             # MCP 工具场景手册（v3.3.0 起以 SKILL.md 为准）
│   ├── mcp-tool-reference.md      # MCP 工具名迁移映射
│   ├── experience-rules-full.md   # 经验法则完整版（v3.3.0 起以 SKILL.md 为准）
│   ├── path-a-four-tools.md       # 路径 A 四板斧详细步骤
│   ├── path-b-env-emulation.md    # 路径 B 环境伪装六步法
│   ├── jsvmp-analysis.md          # JSVMP 字节码分析
│   ├── jsvmp-source-instrumentation.md # 源码级插桩专项
│   ├── jsdom-env-patches.md       # jsdom 环境补丁模板
│   ├── common-pitfalls.md         # 反模式清单
│   ├── crypto-patterns.md         # 加密算法识别与还原代码
│   ├── obfuscation-guide.md       # JS 混淆类型识别与还原策略
│   ├── hook-techniques.md         # Hook 技术大全
│   ├── anti-debug.md              # 反调试对抗手册
│   ├── environment-patch.md       # 环境补全指南
│   ├── protocol-analysis.md       # 协议层分析
│   └── troubleshooting.md         # 错误排查指南
│
├── cases/                          # 经验案例库（唯一经验库）
│   ├── README.md                   # 高频站点速查表
│   ├── _template.md                # 新案例模板
│   └── *.md                        # 各站点/场景案例
│
├── scripts/                        # 工具脚本
│   ├── check-deps.sh               # 环境依赖检查（Node.js + Python）
│   ├── sandbox-runner.js           # VM 沙箱执行器（CLI工具）
│   ├── hook-generator.js           # Hook 代码生成器（10种Hook模板）
│   └── crypto-identifier.js        # 加密算法识别器（密文特征分析）
│
├── templates/                      # 项目模板（按解法模式和语言分类）
│   ├── node-request/               # 模式A (Node.js)：纯算法还原
│   │   ├── main.js
│   │   ├── utils/encrypt.js
│   │   ├── utils/request.js
│   │   └── package.json
│   ├── python-request/             # 模式A (Python)：纯算法还原
│   │   ├── main.py
│   │   ├── utils/sign.py
│   │   ├── utils/request.py
│   │   ├── config/headers.json
│   │   ├── config/keys.json
│   │   └── requirements.txt
│   ├── vm-sandbox/                 # 模式B：VM 沙箱执行
│   │   ├── main.js
│   │   ├── utils/sandbox.js
│   │   ├── utils/request.js
│   │   └── package.json
│   ├── wasm-loader/                # 模式C：WASM 加载还原
│   │   ├── main.js
│   │   ├── utils/wasm-loader.js
│   │   ├── utils/env-patch.js
│   │   └── package.json
│   └── browser-auto/               # 模式D：浏览器自动化
│       ├── main.js
│       └── package.json
│
└── examples/                       # 示例
    └── demo-analysis.md            # 完整分析流程示例
```

## 快速开始

### 1. 环境检查

```bash
bash scripts/check-deps.sh
```

### 2. 作为 Cursor Skill 使用

将本目录放置到 `.cursor/skills/` 下，AI Agent 会自动读取 `SKILL.md` 获取逆向分析能力。

### 3. 工具脚本

**加密算法识别**:
```bash
node scripts/crypto-identifier.js "e10adc3949ba59abbe56e057f20f883e"
```

**生成 Hook 代码**:
```bash
node scripts/hook-generator.js --type=cookie
node scripts/hook-generator.js --type=xhr --target="/api/data"
node scripts/hook-generator.js --type=all
```

**VM 沙箱执行**:
```bash
node scripts/sandbox-runner.js obfuscated.js --extract-cookie --verbose
```

### 4. 使用项目模板

根据分析结果选择合适的模板：

| 场景 | 语言 | 模板 | 何时使用 |
|------|------|------|---------|
| 标准加密算法 | Node.js | `templates/node-request/` | 加密可用 crypto 库还原 |
| 标准加密算法 | Python | `templates/python-request/` | 加密可用 hashlib/pycryptodome 还原 |
| 动态 Cookie | Node.js | `templates/vm-sandbox/` | 服务端返回 JS 生成 Cookie |
| WASM 加密 | Node.js | `templates/wasm-loader/` | 加密在 .wasm 中实现 |
| TLS/环境依赖 | Node.js | `templates/browser-auto/` | 无法脱离浏览器环境 |

**Node.js 项目：**
```bash
cp -r templates/node-request/ my_project/
cd my_project && npm install
# 修改 main.js 中的配置和加密逻辑
node main.js
```

**Python 项目：**
```bash
cp -r templates/python-request/ my_project/
cd my_project && pip install -r requirements.txt
# 修改 main.py 中的配置和 utils/sign.py 中的签名逻辑
python main.py
```

## MCP 工具集成

本 Skill 围绕 [`camoufox-reverse` MCP](https://github.com/WhiteNightShadow/camoufox-reverse-mcp) 服务器组织能力（Camoufox 反检测浏览器，**32 个核心工具，v1.0.0 统一 API**）：
- 源码级插桩 `instrumentation(action='install', mode="ast")` — MCP 侧 esprima 实现，挑战页可用
- 签名安全观察 `hook_jsvmp_interpreter(mode="transparent")` — 仅 prototype getter 替换
- Cookie 归因 `analyze_cookie_sources` — 区分 Set-Cookie 与 document.cookie
- 响应链追踪 `navigate` 返回 `redirect_chain`/`final_status`，秒识反爬类型

| 核心工具 | 用途 |
|---------|------|
| `launch_browser` / `navigate` | 启动反检测浏览器并导航到目标页面 |
| `search_code(keyword, script_url=None)` | 在所有/指定 JS 中搜索关键词 |
| `scripts(action='list'\|'get'\|'save')` | 列出/获取/保存脚本 |
| `hook_function(mode='intercept'\|'trace')` | 自定义 Hook / 函数追踪（支持 non_overridable 防覆盖） |
| `inject_hook_preset` | 一键预设 Hook（xhr/fetch/crypto/websocket/debugger_bypass/cookie/runtime_probe） |
| `hook_jsvmp_interpreter` | 一键插桩 JSVMP 解释器（proxy / transparent 两种模式） |
| `instrumentation(action='install'\|'log'\|'stop'\|'reload')` | 源码级插桩：HTTP 层改写 VMP + 日志 + 重载 |
| `network_capture(action='start'\|'stop')` | 网络捕获（支持 capture_body） |
| `list_network_requests` / `get_network_request` | 列出/获取捕获的请求 |
| `get_request_initiator` | 获取发起请求的 JS 调用栈（黄金路径） |
| `intercept_request` | 拦截/修改/Mock 网络请求 |
| `cookies(action='get'\|'set'\|'delete')` | Cookie 管理 |
| `compare_env` | 全面收集浏览器环境，用于补环境对照 |
| `verify_signer_offline` | 用真实样本离线验证签名代码 |
| `evaluate_js` | 在页面执行任意 JS |
| `export_state` / `import_state` | 保存/恢复浏览器状态 |
| `check_environment` / `reset_browser_state` | 环境自检 / 清理残留状态 |

详见 SKILL.md 的「核心武器」和「工具使用最佳实践」章节。

## 逆向场景覆盖

| 场景 | 技术点 | 参考文档 |
|------|--------|---------|
| 请求参数签名 | MD5/HMAC/自定义哈希 | `crypto-patterns.md` |
| 动态 Cookie | eval首包/api预热/指纹Cookie | `hook-techniques.md` |
| 响应数据加密 | AES/DES/RC4解密 | `crypto-patterns.md` |
| OB混淆 | 字符串还原/CFF/变量重命名 | `obfuscation-guide.md` |
| **JSVMP 虚拟机保护** | **Hook/插桩/日志分析三板斧** | **`jsvmp-analysis.md`** |
| 自定义VM | I/O追踪/Hook解释器 | `obfuscation-guide.md` |
| WASM加密 | 加载/环境补全/导出函数调用 | `environment-patch.md` |
| TLS指纹 | Camoufox 反检测/curl-impersonate/curl_cffi | `protocol-analysis.md` |
| HTTP/2 | Node.js http2模块/Python httpx | `protocol-analysis.md` |
| 字体反爬 | 字体文件解析/CMAP映射 | SKILL.md |
| WebSocket | WS消息分析/协议解析 | `mcp-cookbook.md` |
| 反调试 | debugger绕过/检测对抗 | `anti-debug.md` |
| 环境检测 | webdriver/蜜罐/指纹 | `environment-patch.md` |
| 反检测站点 | Cloudflare/RS/JY绕过 | SKILL.md |
| **请求失败排查** | **Cookie/Header/时间戳/签名对比** | **`troubleshooting.md`** |
| **通用 JSVMP 源码级插桩** | **HTTP 层源码改写 + hot_keys 指纹学习法** | **`jsvmp-source-instrumentation.md`** |
| **Cookie 归因分析** | **HTTP Set-Cookie vs JS document.cookie 融合分析** | **SKILL.md 场景速查** |

## 技术栈

### Node.js
- **crypto** (内置): MD5/SHA/AES/DES/HMAC
- **crypto-js**: CryptoJS 兼容实现
- **node-forge**: RSA/PKI
- **axios**: HTTP 请求
- **vm** (内置): 沙箱执行
- **WebAssembly** (内置): WASM 加载
- **http2** (内置): HTTP/2 请求
- **playwright-core**: 浏览器自动化

### Python
- **requests** / **httpx**: HTTP 请求（httpx 支持 HTTP/2）
- **pycryptodome**: AES/DES/RSA/3DES 加密
- **hashlib** (内置): MD5/SHA 系列
- **hmac** (内置): HMAC 签名
- **base64** (内置): Base64 编解码
- **execjs**: 执行 JS 代码
- **curl_cffi**: 带浏览器 TLS 指纹模拟的 HTTP 客户端

### 调试工具
- **[camoufox-reverse MCP](https://github.com/WhiteNightShadow/camoufox-reverse-mcp) v1.1.1**: 反检测浏览器逆向分析（35 个工具，统一 API，含源码级插桩、Cookie 归因和引擎层属性追踪）

## 版本记录

| 版本 | 日期 | 要点 |
|------|------|------|
| v3.4.1 | 2026-07-29 | 增加 AST 源码插桩执行健康门禁；对齐 camoufox-reverse MCP v1.1.1；保持正常插桩流程不变，仅在异常时进入安全降级 |
| v3.3.1 | 2026-04-21 | 经验法则精简：移除单站点经验（env-patch 行数阈值 / UA 矩阵 / cacheOpts 三件套），合并 evaluate_js 相关为 1 条，总数从 24 压缩到 22 条 |
| v3.3.0 | 2026-04-19 | 核心层回归扩容：Phase 1-5 详细动作 + 10 个场景速查 + 经验法则回迁核心层；案例层强化（禁动清单 + UA 分支矩阵） |
| v3.2.1 | 2026-04-19 | Phase 0.5.1 强化案例复用策略；经验法则第 11 条重写 |
| v3.2.0 | 2026-04-18 | 移除 MCP session 依赖，Checklist 压缩到三项，cases/ 成为唯一经验库 |
| v3.1.0 | 2026-04-18 | SKILL.md 瘦身，references/ 拆分子文档，工具引用对齐 MCP 合并 API |
| v3.0.0 | 2026-04-18 | 硬约束 Checklist + 红线四条 + 经验法则压缩 |
