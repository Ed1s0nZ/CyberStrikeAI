## CyberStrikeAI 浏览器扩展

**当前版本：0.3.5**

Chrome / Edge（Chromium）DevTools 扩展：在开发者工具中捕获 **Network** 流量，发送到 CyberStrikeAI 进行 AI 辅助安全测试。能力与 Burp Suite 插件对齐，并按生产场景做了性能与体验优化。

---

### 快速开始

1. `chrome://extensions/` → 开发者模式 → **加载已解压的扩展程序**
2. 选择目录：`plugins/browser-extension/cyberstrikeai-browser-extension/`
3. 打开目标页面 → **F12** → 顶部 Tab **CyberStrikeAI**
4. 填写 Host / Port / Password → **Validate**（首次会请求访问服务器地址权限）
5. 左侧选中捕获请求 → **Send** → 在 **Output** 查看 AI 结果

点击浏览器工具栏图标可查看 **只读连接状态**；完整配置与操作均在 DevTools 面板内完成。

---

### 界面说明

```
┌─ 连接栏（Validate 成功后可收起）────────────────────────────┐
│ Logo │ https://host:port │ 连接设置 │ ● OK                  │
├─ 操作栏 ────────────────────────────────────────────────────┤
│ Send │ Latest XHR │ Stop │ Copy │ Clear │ ●捕获中/○已暂停    │
│                    XHR/Fetch only │ Debug │ Markdown         │
├──────────────┬──────────────────────────────────────────────┤
│ Test History │ Output │ Request │ Response                   │
│ Captured Req │ Progress + Final Response                    │
└──────────────┴──────────────────────────────────────────────┘
```

| 区域 | 说明 |
|------|------|
| **连接栏** | Host、Port、HTTPS、Password、Validate；成功后收起为 `https://host:port` 摘要 |
| **Test History** | 最多 50 次 Send 记录，可回看 Progress / Final |
| **Captured Requests** | 当前 Tab 捕获列表，最多 200 条，支持搜索 |
| **Output** | 默认 Tab：流式 Progress + Final Response |
| **Request / Response** | 查看选中流量的 HTTP/1.1 格式原文 |

---

### 功能一览

#### 捕获

- **Background 中枢**：`devtools.js` 监听 Network → `service-worker` 队列 → Panel 订阅
- 默认 **XHR/Fetch only**（可关闭以捕获更多类型）
- 静态资源 URL / MIME **预过滤**，命中前不读响应体
- **● 捕获中 / ○ 已暂停**：暂停后零开销，已有列表仍可 Send
- 单条截断：请求体 **64KB**、响应 **4KB**

#### HTTP 展示与 AI Prompt

- **存储**：内存中保留原始 HAR（含 HTTP/2 伪首部 `:method` 等）
- **展示 / Prompt**：归一化为 **HTTP/1.1**（与 Burp 插件一致）

```http
GET /api/foo HTTP/1.1
Host: example.com
Cookie: ...
```

#### 发送到 CyberStrikeAI

- 弹窗选择：**项目 / 角色 / 对话模式**（动态 API）+ 测试指令
- 支持 **Eino Single**、**Deep**、**Plan-Execute**、**Supervisor**
- **Latest XHR**：一键选中最近 API 请求并打开发送弹窗
- **Stop**：中止本地 SSE + 调用服务端 `/api/agent-loop/cancel`

#### 流式输出

- Progress 日志上限 **512KB**（超出截断）
- **Final Response 不截断**（当前进行中的测试）
- 历史 run 切换后 Final 软截断 **100KB**
- **Markdown**：流式阶段纯文本；结束后 `requestIdleCallback` 渲染；超 **100KB** 降级纯文本
- **Copy**：复制当前 Request / Response / Final

#### 安全与权限

- Token 存 **chrome.storage.session**（关浏览器失效）
- **optional_host_permissions**：Validate 时按需授权 CyberStrikeAI 地址
- Markdown iframe 使用 `sandbox`

---

### 按钮与选项

| 控件 | 作用 |
|------|------|
| **Validate** | 登录并校验 Token；进行中再次点击为 Cancel |
| **连接设置 / 收起** | 展开或折叠 Host/Port/Password 表单 |
| **Send** | 对选中捕获发起到 CyberStrikeAI |
| **Latest XHR** | 选中最近 XHR/Fetch 并 Send |
| **Stop** | 停止当前 AI 流（本地 + 服务端） |
| **Clear Output** | 清空当前 run 的 Progress / Final |
| **● 捕获中 / ○ 已暂停** | 启用或暂停 Network 捕获 |
| **XHR/Fetch only** | 只捕获 API 类请求 |
| **Debug events** | 在 Progress 显示更多 SSE 事件 |
| **Markdown** | Final 完成后渲染富文本 |
| **Clear All** | 清空 Test History |
| **Clear** | 清空当前 Tab 捕获列表 |

---

### 数据与内存（不会无限增长）

| 数据 | 上限 | 位置 | 清理时机 |
|------|------|------|----------|
| 捕获请求 | 200 条 / Tab | Background + Panel 内存 | 超出丢弃最旧；可手动 Clear |
| Tab 捕获槽 | 20 个 Tab | Background 内存 | 超出丢弃非当前 Tab |
| 测试历史 | 50 条 | Panel 内存 | 超出丢弃最旧；Clear All |
| Progress | 512KB / run | Panel 内存 | 超出截断 |
| Final（进行中） | 无硬上限 | Panel 内存 | — |
| Final（历史） | 100KB 软截断 | Panel 内存 | 切换到其他 run 时 |
| 配置 + Token | 极小 | chrome.storage | 手动改配置 |

- 关闭 **DevTools** → Panel 内存清空  
- 关闭 **浏览器** → Session Token 失效  
- Service Worker 被回收 → Background 捕获队列清空  

---

### 性能说明

| 场景 | 影响 |
|------|------|
| 未开 DevTools | **无影响**（不监听 Network） |
| DevTools 开 + 捕获暂停 | **几乎无影响** |
| DevTools 开 + 捕获中 + XHR only | 仅匹配请求有轻微开销 |
| 高流量 SPA | 建议保持 **XHR/Fetch only**，不需要时点 **已暂停** |

已做优化：过滤器内存缓存、静态资源不读 body、列表增量插入、搜索防抖、rAF 节流流式 UI。

---

### 常见问题

**扩展更新后报错 `chrome.runtime.connect` undefined？**  
扩展重载后旧 DevTools 面板上下文失效。请：**关闭 DevTools → 重新加载扩展 → 再开 F12**。

**Request 里为什么曾经有 `:authority`、`:method`？**  
HTTP/2 伪首部。展示与 AI Prompt 已归一化为 HTTP/1.1；原始 HAR 仍保存在内存 entry 中。

**Console 里 localhost CORS 报错是插件造成的吗？**  
不是。那是页面自身请求本机服务被浏览器拦截，与扩展无关。

**Test History 很多会挡住 Captured Requests 吗？**  
不会。历史区最高占侧边栏 **42%**，超出部分区域内滚动；捕获区占剩余空间。

**会拖慢网页吗？**  
日常浏览（不开 DevTools）无影响。调试时可用 **已暂停** 完全停止捕获。

---

### Popup 与 DevTools 分工

| 位置 | 用途 |
|------|------|
| **DevTools 面板** | 连接、Validate、捕获、Send、Output（主工作区） |
| **扩展 Popup** | 只读连接状态 + 版本号 + 打开 DevTools 引导 |

不在 Popup 中重复完整配置表单，避免与主流程脱节。

---

### 打包发布

```bash
bash plugins/browser-extension/cyberstrikeai-browser-extension/package.sh
# → dist/cyberstrikeai-browser-extension.zip
```

图标从项目根 `images/logo.png` 生成：

```bash
LOGO="images/logo.png"
ICONS="plugins/browser-extension/cyberstrikeai-browser-extension/icons"
for size in 16 48 128; do
  sips -z $size $size "$LOGO" --out "$ICONS/icon${size}.png"
done
```

---

### 限制

- Chrome **不提供** Network 面板右键菜单 API → 使用 **Latest XHR** + 自建列表
- Firefox 需 `about:debugging` 临时加载；`storage.session` 不可用时 Token 回退 `local`
- 无法一键从 Popup 跳转到 DevTools 指定面板（Chrome API 限制）

---

### 目录结构

```text
manifest.json                 # MV3 清单
background/service-worker.js  # 捕获队列、Panel Port、全局开关
devtools.js                   # Network 监听（最早过滤）
devtools.html
panel/
  panel.html / panel.js / panel.css   # 主 UI
popup/
  popup.html / popup.js / popup.css   # 只读状态
lib/
  api.js              # 登录、SSE、项目/角色 API
  storage.js          # 配置与 session token
  capture.js          # HAR 摘要、静态过滤
  http-normalize.js   # HTTP/2 → HTTP/1.1 展示/Prompt
  formatter.js        # toPrompt 组装
  markdown.js         # Final Markdown 渲染
  catalog-cache.js    # 项目/角色 5 分钟缓存
  constants.js        # 上限常量
icons/                # 16 / 48 / 128
package.sh
```

---

### 与 Burp 插件对比

| 能力 | Burp 插件 | 浏览器扩展 |
|------|-----------|------------|
| 流量来源 | Proxy 历史 | DevTools Network |
| 连接配置 | Tab 内 | Tab 内（可折叠） |
| HTTP 格式 | HTTP/1.1 | 展示/Prompt 归一化为 HTTP/1.1 |
| 项目/角色/模式 | Send 弹窗 | Send 弹窗 |
| SSE 输出 | Progress + Final | Progress + Final |
| 捕获开关 | — | ● 捕获中 / ○ 已暂停 |
