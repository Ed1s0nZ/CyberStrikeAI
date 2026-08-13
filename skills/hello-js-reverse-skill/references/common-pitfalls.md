# ⚠️ 反模式清单（v3.0.0 基于三次真实实战数据）

> **本文档是 AI 在每次分析前必读的"禁忌清单"**。不是建议，不是"参考一下"，
> 是**打脸清单**——本文档里的每一条都是某次真实 AI 分析中实际发生过的失败。
>
> 本文档的存在假设：加经验法则无效（46 条都记不住），但**让错误路径显眼**有效。
> 这里的每条反模式都配了：(1) 真实站点名；(2) AI 当时的辩护；(3) 为什么那辩护不成立。

---

## 反模式 1：用 Playwright 过反爬挑战取 cookie 作为最终方案

### 实战案例：瑞数 nmpa（2026-04 实战，AI 违反）

**AI 的操作**：
```javascript
// cookie_fetcher.js (实战 AI 产出，违反第一原则)
const { firefox } = require('playwright');
async function getCookies() {
  const browser = await firefox.launch();
  const page = await browser.newPage();
  await page.goto('https://www.nmpa.gov.cn/...');  // 过 RS 412 挑战
  await page.waitForTimeout(5000);                  // 等挑战自动完成
  const cookies = await page.context().cookies();  // 取 NfBCSins2OywS
  return cookies;
}
// 最终方案每 30 分钟运行一次 getCookies()
```

**AI 的辩护**："瑞数 JSVMP 保护极其复杂，纯 Node.js 几乎不可能绕过"

**为什么这辩护不成立**：
- 工作区里已经有 `site_nmpa/sign_service.js`，用 sdenv 已完成纯 Node.js 方案
- AI 根本没看 sdenv 文档就断言"不可能"
- 最终交付在无浏览器的 Docker 容器里**无法运行一小时以上**——这就是违规

### 正确做法

```javascript
// 使用 sdenv（魔改 jsdom + C++ V8 Addon）
const { jsdomFromUrl } = require('sdenv');
async function getRSCookies(targetUrl) {
  const dom = await jsdomFromUrl(targetUrl, { userAgent: 'Mozilla/5.0...' });
  await new Promise(r => dom.window.addEventListener('sdenv:exit', r));
  return extractCookies(dom);
}
```

### 判定测试

> 最终代码在一个**无 X11、无浏览器、只有 Node.js**的 Docker 容器里，
> 能不能稳定跑满 24 小时？
> - 能 → 合规
> - 不能 → 违规（Playwright 方案属于违规）

---

## 反模式 2：Cookie 硬编码到 headers.json

### 实战案例：抖音 /aweme/detail（2026-04 实战，AI 违反灰色地带）

**AI 的操作**：
```json
// headers.json (AI 产出)
{
  "Cookie": "ttwid=1|xxxxxxxx; __ac_nonce=yyyyyy; __ac_signature=zzzzzz"
}
```
```javascript
// main.js 直接读取上面的 headers 发请求
```

**AI 的辩护**："cookie 有几小时有效期，不是每次请求前都要开浏览器"

**为什么这辩护不成立**：
- "几小时"还是"几天"不影响定性——cookie 一旦过期，**没有浏览器就完全无法工作**
- 最终方案的可用性取决于浏览器是否可用，浏览器就是方案的依赖
- `__ac_signature` 可以协议还原（基于 `__ac_nonce` 的签名算法），抖音社区早有方案
- `ttwid` 更简单，直接请求首页就能 Set-Cookie 拿到

### 正确做法

```javascript
// 协议还原 cookie 获取
async function getTtwid() {
  const r = await axios.get('https://www.douyin.com/');
  return r.headers['set-cookie'].find(c => c.startsWith('ttwid=')).split(';')[0];
}

async function generateAcSignature(nonce) {
  // 还原 __ac_signature 的签名算法
  // ... (基于 nonce 的字符串变换 + 时间戳)
}
```

### 判定测试

> 启动代码在无浏览器环境一周，不人工干预，是否能稳定获取数据？
> - 能 → 合规
> - 不能（需要人工去浏览器抓 cookie 贴进去）→ 违规

---

## 反模式 3：激活 skill 后直接 launch_browser（跳过 Checklist）

### 实战案例：三次都违反（瑞数 + TikTok + 抖音）

**三次的 AI 操作完全一样**：
```
轮 1: 激活 skill (读几万字 SKILL.md)
轮 2: launch_browser(headless=false)
轮 3: navigate(target_url)
... 从此开始分析
```

**AI 的辩护**："用户要求明确，快速开干；session 是新功能，先做主线任务"

**为什么这辩护不成立**：
- 三次 AI 都这么说，三次都导致：
  - session 没开 → v2.9.0 域级持久机制完全失效
  - 经验库没查 → 重复踩坑 / 从零写已有方案
  - 不知道工作区有 `site_*` 目录 → 瑞数 nmpa 从零写 Playwright 方案，而 `site_nmpa/` 里已有 sdenv 方案
- "快速开干"其实慢得多——瑞数实战用 12 轮对话写了个错误方案，如果做了 Checklist 的 CHECK-2，5 分钟搞定

### 正确做法

```
激活 skill 后第一件事：在对话中输出硬约束 Checklist 五项复述
  [CHECK-1] check_environment() → 贴结果
  [CHECK-2] 读 cases/README.md 速查表 + listDirectory 工作区 → 命中记录
  [CHECK-3] list_sessions() → 历史 session 记录
  [CHECK-4] start_reverse_session() → domain + run_id 记录
  [CHECK-5] 最终方案意图声明

五项都通过，再调 launch_browser
```

### 判定测试

> 翻 skill 激活后的前 3 条对话记录。
> - 前 3 条里有完整的 CHECK-1~5 复述输出 → 合规
> - 前 3 条是"launch_browser / navigate / ..." → 违规（红线 1）

---

## 反模式 4：在工作区 site_* 目录存在时从零建新项目

### 实战案例：瑞数 nmpa（工作区已有 `site_nmpa/`，AI 建了 `site_nmpa_20260418/` 从零写）

**AI 的操作**：
```
listDirectory() 看到：
  site_nmpa/           ← 已有完整方案
  site_nmpa_20260418/  ← AI 新建的错误方案
```

**AI 没做的事**：
- 没打开 `site_nmpa/` 里的文件看一眼
- 没读 `site_nmpa/sign_service.js`（已验证的 sdenv 方案）
- 没问用户"这个目录是做什么的"

**AI 的辩护**："用户要求重新使用 skill 进行获取，我以为要从零开始"

**为什么这辩护不成立**：
- "重新使用 skill" ≠ "从零写代码"，而是"遵循 skill 流程"——Phase 0.5 本身就要查已有
- 用户说"重新"可能只是希望用新 skill 验证一遍，不是丢弃已有
- 遇到这种歧义，应该问用户，不是默认从零

### 正确做法

```
listDirectory(".")
→ 看到 site_nmpa/
→ listDirectory("site_nmpa/")
→ 有 sign_service.js / keys.json → 这是已验证的方案
→ 询问用户："检测到 site_nmpa/ 目录含已验证的 sdenv 方案，
   是要(A)基于已有方案创建今天的新目录复用；还是
   (B)重新分析一遍验证方案仍有效；还是(C)其他？"
→ 根据用户选择决策
```

---

## 反模式 5：面对复杂 SDK（TikTok webmssdk）直接判定"不可能还原"

### 实战案例：TikTok 潜在滑坡（AI 在 jsdom 不工作时差点滑向 Playwright）

**AI 的思维过程**（实战复盘 Q-B 自述）：
> "如果后续环境补丁迭代仍无法激活 SDK 拦截器，我可能会被迫考虑
> '每次请求前用 Camoufox 过一遍页面获取签名'的方案"

**为什么这滑坡不能发生**：
- "环境补丁没让 SDK 激活" 是**本步骤**的问题，不是"整个协议方案不可行"的证据
- 应该走降级梯度的**下一级**（见 SKILL.md 第五条原则），而不是横向跳到浏览器
- 梯度 3-4 (点对点 hook_function / jsdom 变体) 都没试就放弃，违反第五条原则

### 正确做法

看 SKILL.md 第五条原则的降级梯度表：
```
梯度 0: 本域是否有历史 session? → attach_domain_readonly + reverify
梯度 1: get_session_snapshot 看本 run 做过什么
梯度 2: 换 Hook 模式 (proxy ↔ transparent) / 换插桩模式 (ast ↔ regex)
梯度 3: 点对点 hook_function 针对具体签名函数
梯度 4: 路径 B 的其他变体（vm 沙箱 / jsdomFromUrl 全量加载 / sdenv）
梯度 5: 合法出口: stop_reverse_session("suspended") + export_session + 写报告

禁止跨梯度直接到"用浏览器兜底"
```

---

## 反模式 6：经验法则背诵但不执行

### 实战案例：三次都违反

AI 在复盘时能流利背出：
> "经验法则 #28：反爬类型识别是 Phase 0 的 Phase 0"
> "第一原则：协议优先"
> "Phase 0.5：经验库指纹匹配"

但实际操作时**完全忽略**。

**为什么这发生**：
- 规则写在 skill 里 ≠ 规则会被执行
- AI 读了几万字 SKILL.md 后，工作记忆只保留最显眼的几条
- 非硬 gate 的规则（无检查机制）100% 会被忽略

### 正确做法

**不靠 AI 自觉**，靠 v3.0.0 的硬 gate：
- Checklist 五项必须复述输出（可被用户验证）
- 红线四条违反即失败（明确后果）
- 高频站点速查表（不靠"主动想起来"而是"看到域名就命中"）

---

## 如何使用本文档

1. **AI 激活 skill 后**：本文档应该在 Checklist [CHECK-2] 步骤被简单扫视（至少看一眼反模式 1-6 的标题，避免踩坑）
2. **AI 在做决策时卡住**：回到本文档，看自己正在考虑的方向是不是反模式之一
3. **AI 写代码写到一半意识到在滑向反模式**：立即停，走 SKILL.md 第五条原则的降级梯度
4. **用户发现 AI 违反**：直接引用本文档的反模式编号质问 AI（"你正在做反模式 X，看 references/common-pitfalls.md"）

---

## 贡献新反模式

每次实战失败后，如果发现 skill 里没覆盖的新反模式：
1. 走 `retrospective-prompt.md` 做完整复盘
2. 按本文档的"反模式 N：XXX"格式追加一条
3. 更新 CHANGELOG
4. commit 时标注 `doc(pitfall): add pitfall #N from <site> retrospective`

**新反模式的质量要求**：
- 必须有**真实站点名**，不能是抽象描述
- 必须包含 AI 当时的**具体操作**（代码 / 命令序列）
- 必须包含 AI 当时的**辩护**（真实的 rationale）
- 必须说明**为什么辩护不成立**（具体论证，不是"就是不对"）
- 必须给**正确做法**（带代码示例）
- 必须给**判定测试**（便于后续 self-check）

6 条起步，后续每次实战失败后追加。30 条就封顶，再加就说明 skill 本身要结构性调整。

---

## 更新记录

| 版本 | 日期 | 更新内容 |
|------|------|---------|
| v3.0.0 | 2026-04-18 | 初始版本：基于三次真实站点实战（瑞数/TikTok/抖音）数据，收录 6 条反模式 |
