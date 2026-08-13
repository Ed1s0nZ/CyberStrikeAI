# 操作纪律与题型最小验证剧本

> 定位：**渗透测试与评测/CTF 通用的调度层契约**，防"勤奋的低效"——长任务爆炸（c-09 OFBiz 3 小时）与短探针刷屏（c-01 1Panel 前端逆向 63 分钟）。
> 用法：深挖动作开始前读一遍；卡住、连续失败、或准备手写 PoC 前重读对应小节。

## 1. 探针与时间预算（硬上限）

| 项 | 上限 | 触发动作 |
|---|---|---|
| 单探针 timeout | ≤30 秒 | 默认值；仅下载/编译/大范围扫描类任务放宽 |
| 同一假设连续超时 | 2 次 | 停该主线，换假设或换攻击面 |
| 同类变体失败 | 3 次且无新增事实 | 停该面，记录失败矩阵（失败签名本身是事实） |
| 单目标无新增事实 | 15 分钟 | 回查知识库/换攻击面/切目标 |
| 单目标无交付物进展（flag/关键凭据/报告素材） | easy ≤15 / medium ≤20 / hard ≤25 分钟 | 切目标，记 PARTIAL + 已试矩阵 |
| 后台任务轮询 | 连续 3 次无新日志 | 判卡死，小步探测验证可达性 |

## 2. 事实台账（继续/停止的唯一判据）

- 每个动作结束自问：**新增了什么可判读事实？**（端口、版本、报错签名、字段名、响应差异、可写路径）
- 只换 payload 不产生新事实 = 无效动作，禁止
- 无进展但事实在积累 → 不算卡死，可继续；**事实停滞 → 立即切**
- 失败签名（error 文本 / 超时 / 拦截页 / 400 与 200 差异）也是事实，必须记录

## 3. 前端逆向闭环（c-01 教训）

- **黑盒优先，逆向兜底**：请求模板优先从真实流量/响应差异获取——headless 浏览器拦截（playwright/puppeteer）或代理抓包看完整请求，或发一个完整请求看 4xx-5xx 响应差异反推服务器期望字段；**不读 JS 也能还原 URL+headers+body+时序**（如 1Panel 的 EntranceCode=base64("entrance")，抓包 header 里直接可见）；**仅当前端加密/验签/时序校验时才逆向 JS**（复用其加密函数或黑盒调用）
- JS/压缩代码只抽可组装请求的信息：请求 URL、HTTP 方法、字段名、编码/加密方式、验证码与安全入口参数
- 连续 2-3 轮无新字段 → 立即组装一次**真实完整请求**验证（有凭据就带凭据）
- 真实请求失败 → 依次切：后端 API 枚举 → SSH 凭据一次性验证 → 版本 CVE → 停目标
- 禁止：把"读懂 minified JS"当目标；对压缩变量命名考古；反复猜测前端逻辑而不发请求

## 4. 题型最小验证剧本

### 4.1 组件 CVE 快速路径（Web / 中间件 / 服务）
1. 指纹：banner / 报错页 / 默认路径 / 特征文件，**必须确定版本**
2. 检索顺序：`searchsploit {组件} {版本}` → `nuclei -tags cve` → `search_knowledge_base`
3. **组件生态也要查（2026-08-06 c-02 教训）**：识别出组件后，**本体 + 其生态插件/扩展/管理器**都要查 CVE——ComfyUI 本体 LFI（CVE-2024-10697）≠ ComfyUI-Manager 插件 RCE（CVE-2025-67303，才是真入口）。从组件路由、面板、常见扩展目录名（custom_nodes / plugins / addons / modules）推断生态面
4. 现成武器优先：nmap --script / searchsploit 命中 / msf 模块 / **现成 PoC 脚本**，禁止先手写轮子
5. **本地情报无命中 → 联网情报检索（必须完成，2026-08-05 c-02 教训）**：
   - 上游安全公告 / 版本 release / 修复 commit → CVE advisory → GitHub PoC/issue
   - **PoC 搜索路径（按序）**：
     a. **`github_search.sh <组件> <漏洞关键词>`**（kali 已装，~/local/bin；依赖 ~/.bashrc 的 GITHUB_SEARCH token）——自动搜 repositories/issues/code 三类，code 命中直接给 PoC 文件路径（例：`github_search.sh comfyui manager cve-2025-67303` 直接命中 Threekiii/Awesome-POC 与含 nuclei 模板的 exploit 仓库）
     b. 本地 PoC 库：`grep -ril "{CVE}" ~/.local/nuclei-templates`（nuclei 模板）、vulhub 复现仓库（github.com/vulhub/vulhub/tree/master/{组件}/{CVE}）、Threekiii/Awesome-POC
     c. GitHub API 直连：`curl -H "Authorization: Bearer $GITHUB_SEARCH" "https://api.github.com/search/code?q={关键词}"`
   - 检索带「组件 + 版本/CVE + exploit/bypass/patch」关键词；主站受阻时换通道：GitHub API → raw.githubusercontent → 上游文档 → 搜索引擎
   - **仅做域名连通性探测（curl 返回 301/000/403）不算完成检索**，必须带关键词实际搜索
6. 最小验证：1 个请求或 1 个 payload 验证存在性，有响应差异即确认，不追求一步到 RCE
7. 判据：命中 exploit/PoC 后先理解触发条件与适用版本（很多 PoC 有版本门槛，如 ComfyUI-Manager <3.38），再复制行为而非无脑套 payload
8. 成功 → 回写知识库：判别特征 → 原理 → 最小验证链 → 失败边界 → 适用场景

### 4.2 服务类靶机快速路径（HugeGraph / JDWP / Redis / ES / 数据库）
1. 端口识别：`nmap -sV`，顺带看自带脚本列表（如 jdwp-exec.nse）
2. 顺序：nmap --script → searchsploit → msf → **才允许手写 PoC**
3. 手写 PoC 前置登记：先列出已检查过的现成武器，写进黑板再动手
4. 沙箱类绕过（gremlin 等）：先查 KB 沙箱分析思路，再逆向豁免机制，禁止换函数名硬试

### 4.3 前端登录闭环模板
1. 目标：还原一个完整真实登录请求（URL + 方法 + 全部字段 + 编码）
2. 来源：**优先黑盒**（浏览器抓包 / 代理 / 响应差异反推），其次 HTML 表单 → JS 中 fetch/axios 调用点
3. 有凭据时：一次真实请求验证，禁止反复猜前端逻辑
4. 失败路径：后端 API 枚举 → SSH 验证 → 版本 CVE → 停

### 4.4 成功回写模板（create_knowledge_item）
- title：`{组件/漏洞类} {利用方向}（{CVE 家族/日期}）`
- 内容结构：判别特征（指纹/报错）→ 原理（为什么能绕）→ 最小验证链（3-5 步）→ 失败边界（什么情况不适用）→ 可迁移场景
- 写原理不写答案：不跨项目拉完整 POC

## 5. 阶段切换审计（每 15 次工具调用）

- 自检：过去 15 次调用新增了几条事实？0 条 → 立即换策略
- 主线有效性：当前主线是否还是最早成功的假设？该降级就降级（RCE 主线失败 → 信息读取 / 其他端点）
- 时间盒提醒：剩余时间 ÷ 未完成目标，评估是否切目标
- 读黑板：`get_project_fact` 确认 target/scoreboard 未过期，再动目标
