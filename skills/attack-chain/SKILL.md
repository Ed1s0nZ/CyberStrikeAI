---
name: attack-chain
description: >-
  多阶段攻击链编排（阶段轴路由器）：完整渗透/从外网到域控/HW演练/云+内网混合等多阶段任务，
  按 ATT&CK 阶段路由到 CyberStrikeAI 原生技能、offensive-*/reverse-* 专项与 competition-* 评测技能。
  单阶段任务不要经过本技能，直接去对应专项。Use when a task spans recon, initial access,
  privilege escalation, lateral movement, evasion, or impact across multiple stages.
---

# Attack Chain — 多阶段攻击链编排（阶段轴路由器）

> 定位：本技能是**阶段轴路由器**，与类目轴的 `offensive-skill-router` / `reverse-skill-router` 正交互补。
> 负责回答三件事：现在处于攻击链哪个阶段、该加载哪个专项技能、阶段之间如何交接。
> 本技能**不内嵌 payload**——payload 在专项技能里，链式打法在 `references/attack-playbooks.md`。

---

## 加载契约（读完立即执行）

1. `NOW` **判定运行模式**：真实授权工作 → 走「真实模式」；评测/靶场/计时演练（tsecbench、CTF、HW 计时赛）→ 走「评测模式」。上下文判不出时问用户一次，之后不再问。
2. `NOW` **用 write_todos 建阶段计划**：3~6 条，覆盖 侦察 / 验证 / 扩大战果 / 交付，与编排器纪律对齐。
3. `NOW` 读 `references/lifecycle-checklist.md` 拿阶段门闩与纪律。
4. `按需` 读 `references/attack-playbooks.md`（6 条链式打法 + 决策矩阵）、`references/evasion-cheatsheet.md`（EDR/AV 绕过与 C2 隐蔽速查）。
5. `全程` 黑板节奏：每确认一条认知（端口/指纹/凭据形态/入口）→ `upsert_project_fact`；每验证一条可复现漏洞 → `record_vulnerability`。上下文压缩后细节会丢，落库优先于继续动作。
6. `全程` 重活（爆破/大字典/全端口扫描/批量 PoC 验证）→ `batch_task` 异步队列，不占 exec 通道。

> 旧版的 case-init.ps1 / ops/ 仪式化流程已废弃（本包不存在这些文件）。真实模式下授权确认一次性完成，不做表单化审批；评测模式下按 CTF 角色的控制面锚点纪律执行。

---

## 何时进入本技能

| 多阶段任务（必须经过本技能） | 单阶段任务（直接去专项，别绕路） |
|---|---|
| "做一次完整渗透测试 / 全流程" | 只扫端口/测绘 → `attack-surface-recon` |
| "从外网打到域控" | 只打某个 Web 漏洞 → `offensive-sqli` 等 |
| "拿到 webshell，下一步怎么办" | 只逆向 APK/样本 → `reverse-skill-router` |
| "HW 攻防演练 / 红队评估" | 只查组件 CVE → `component-vuln-intel` |
| "云 + 内网混合目标" | 只挖 0day/N-day → `zero-day-discovery` |
| "钓鱼 + 后渗透组合" | 只做钓鱼 → `initial-access-phishing` |
| 评测中的多阶段渗透靶场 | 单个 Web/二进制题 → 对应 `competition-*` |

---

## 阶段 → 技能路由矩阵（核心）

**加载纪律**：判定当前阶段后**只加载该阶段的技能**；进入新阶段再加载新技能。wall-clock 是最稀缺资源，禁止一次拉满全部。

### S1 侦察与攻击面测绘
- 原生：`attack-surface-recon`（主）、`component-vuln-intel`（组件识别后必做联网情报）、`source-code-hunting`（.git/JS/密钥泄露）
- 专项：`offensive-osint` / `offensive-osint-methodology`、`pentest-tools`
- 评测：`competition-web-runtime`（含靶场侦察工作流）
- Deep 子代理：`recon` / `intel-collection` / `attack-surface-enumeration`

### S2 初始访问（Web / 钓鱼 / 云入口）
- Web 主线：`web-attack-methods`（全栈打法）+ `offensive-skill-router` Web 类目逐项（`offensive-sqli` / `offensive-xss` / `offensive-ssrf` / `offensive-ssti` / `offensive-rce` / `offensive-file-upload` / `offensive-deserialization` / `offensive-request-smuggling` / `offensive-idor` / `offensive-xxe` / `offensive-waf-bypass` / `offensive-business-logic`）+ `api-security`
- 钓鱼/社工：`initial-access-phishing`、`offensive-initial-access`
- 云入口：`cloud-attack-methods`、`offensive-cloud`
- 评测：`competition-web-runtime` / `competition-graphql-rpc-drift` / `competition-request-normalization-smuggling` / `competition-websocket-runtime` / `competition-ssrf-metadata-pivot` / `competition-jwt-claim-confusion` / `competition-template-render-path`
- Deep 子代理：`penetration` / `vulnerability-triage`

### S3 权限提升
- 原生：`post-exploitation`（主）
- 专项：`offensive-windows-mitigations` / `offensive-windows-boundaries`、`windows-ad`
- 评测：`competition-kernel-container-escape` / `competition-container-runtime` / `competition-linux-credential-pivot`
- Deep 子代理：`privilege-escalation`

### S4 横向移动与内网域
- 原生：`active-directory-attack`（主）、`post-exploitation`
- 专项：`offensive-active-directory`、`windows-ad`
- 评测：`competition-windows-pivot` / `competition-kerberos-delegation` / `competition-ad-certificate-abuse` / `competition-dpapi-credential-chain` / `competition-lsass-ticket-material` / `competition-relay-coercion-chain` / `competition-identity-windows`
- Deep 子代理：`lateral-movement`

### S5 持久化与通道维持
- 原生：`post-exploitation`
- 评测：`competition-browser-persistence` / `competition-malware-config`
- Deep 子代理：`persistence-maintenance`

### S6 对抗规避与 OPSEC（贯穿全程）
- 原生：`redteam-opsec`（主）
- 专项：`edr-bypass-re`（逆向 EDR 后针对性绕过）、`offensive-waf-bypass`、`offensive-advanced-redteam`
- 速查：`references/evasion-cheatsheet.md`（syscall/unhook/ETW/睡眠加密/C2 隐蔽/LOLBins）
- Deep 子代理：`opsec-evasion`

### S7 影响证明与链式兑现
- 原生：`capability-primitive-search`（低危串联凑等式）、`pentest-verification`（证据铁律：搜索≠漏洞）
- 专项：`specialized-attack-playbooks`（GoEdge/CDN→S3 STS/宝塔+UniApp 等专题链）
- Deep 子代理：`impact-exfiltration`（最小化、脱敏的影响证明）

### S8 报告与清理
- `docs-generator`（报告）、`pentest-output-standards`（输出规范/负结果/台账）、`diagram-generator`（攻击路径图）、`offensive-reporting`
- Deep 子代理：`reporting-remediation` / `cleanup-rollback`

### 跨阶段支撑线（随时可加载）
- 0day/N-day：`zero-day-discovery`、`patch-diff-exploit`、`binary-diff`
- 二进制漏洞：`reverse-skill-router` → `pwn-chain` / `binary-mobile-reversing` / `ida-reverse` / `ghidra-reverse` / `radare2`
- 被堵换路（403/429/WAF/超时）：`proxy-tool-bootstrap`
- 云深挖：`cloud-k8s`、`competition-k8s-control-plane` / `competition-agent-cloud` / `competition-cloud-metadata-path`
- AI/LLM 目标：`ai-llm-app-attack`、`llm-security`、`competition-prompt-injection`
- 无线/硬件/近源：`wireless-hardware-attack`、`wifi-wireless`、`offensive-wifi*`、`hardware-security`、`radio-sdr`
- 方法论卡壳：`search_knowledge_base`（HackTricks 全英文知识库）+ `unlimited-attack-scope`（换路原则）

---

## 路径规划决策树

```
1. 目标类型？     Web / 内网 / 云 / 移动 / IoT / AI / 混合
2. 当前有什么？   纯外部视角 / 有凭据 / 有立足点 / 有源码
3. 终点是什么？   域控 / 数据 / 特定系统 / 影响证明 / flag
4. 约束是什么？   时间盒 / 隐蔽性要求 / 禁碰系统 / 锁频阈值
    ↓
映射到 S1~S8 当前阶段 → 按矩阵加载技能 → 执行
    ↓
一条路走不通 → 回到本技能，按 attack-playbooks.md 决策矩阵换路径
同一路径失败签名 2 次 → 停，换方法（不是换 payload）
```

---

## 真实模式（日常授权工作）

- **Scope 门（一次性）**：目标范围、禁止动作、锁频阈值、业务高峰窗——开工前与用户确认一次，之后全程自主。
- **证据纪律**：遵循 `pentest-verification`——confirmed 必须带可复现证据，tentative 只表线索，负结果也落库。
- **最小影响**：禁止 DoS 类验证；影响证明用最小化、脱敏方式；真实用户数据不访问不下载。
- **高风险动作**（持久化、横向、凭据导出）前评估并记录风险等级。
- 交付：Evidence → Finding → Path 链齐全，`docs-generator` 出报告。

## 评测模式（tsecbench / CTF / 计时演练）

- **时间盒**：wall-clock 最稀缺。每目标软预算，非进展 lane 到预算即杀，记 PARTIAL + coverage。
- **技能优先级**：`competition-*` 优先于通用专项（它是按靶场工作流写的），通用专项做补充。
- **flag 判定**：只有平台 submit 成功响应才算 confirmed；本地发现仅算 observed。
- **六维 → 路由速查**：

| 评测维度 | 主线加载 |
|---|---|
| Web 漏洞挖掘 | `web-attack-methods` + offensive Web 家族 + `competition-web-runtime` |
| 对抗规避 | `redteam-opsec` + `edr-bypass-re` + evasion-cheatsheet + `offensive-waf-bypass` |
| 云攻击 | `cloud-attack-methods` + `cloud-k8s` + `competition-cloud-metadata-path` / `competition-k8s-control-plane` |
| 多阶段渗透 | 本技能 S1→S8 全链 + `post-exploitation` + `active-directory-attack` + `competition-windows-pivot` |
| 漏洞利用 | `capability-primitive-search` + `zero-day-discovery` + `component-vuln-intel` + `specialized-attack-playbooks` |
| 二进制漏洞挖掘 | `reverse-skill-router` + `pwn-chain` + `binary-mobile-reversing` + `patch-diff-exploit` + `offensive-fuzzing` + `competition-reverse-pwn` |

---

## 与 CyberStrikeAI 运行时的咬合

- **角色**：`网络安全专家(自动路由)` / `渗透测试` 角色开工加载 offensive-skill-router 定类目；跨阶段任务再加载本技能定阶段轴。`CTF` 角色激活评测模式时，本技能负责多阶段维度。
- **Deep 模式**：S1~S8 与子代理一一对应（见矩阵）。委派 `task` 时 description 必须自带交接包：完整目标 + 范围 + 已完成事实 + 本轮增量 + 期望交付结构。
- **plan_execute / supervisor**：阶段即 DAG 节点，按依赖关系扇出与汇合。
- **skill 工具渐进加载**：本技能（阶段轴）→ 专项 SKILL.md（打法）→ references（速查），三层按需，禁止一次全拉。
- **黑板**：跨阶段交接靠 `upsert_project_fact`（同 fact_key 覆盖），不靠对话记忆；上下文压缩后从黑板恢复。

---

## Playbook 索引（详见 references/attack-playbooks.md）

| # | 链路 | 一句话路径 |
|---|---|---|
| 1 | 外网 Web → 域控 | 测绘→Web 利用→隧道→凭据→横向→BloodHound→域提权 |
| 2 | 钓鱼 → 内网 | 画像→载荷→上线→提权→凭据→横向 |
| 3 | 近源 → 内网 | WiFi/BadUSB/植入→内网接入点→同 P1 后半 |
| 4 | 云环境 | 云资产→存储桶/SSRF→元数据凭据→IAM 提权→数据 |
| 5 | SRC 快速打点 | 资产→指纹→参数→逐类验证→PoC |
| 6 | AD CS 证书 | certipy→ESC1-8→恶意证书→DCSync |

---

## 任务完成自检（声称完成前 MUST 通过）

- [ ] 每个执行过的阶段都有对应 Fact/Vuln 落库（不是靠记忆）？
- [ ] 每阶段加载的技能与矩阵一致，没有一次性拉满全部技能？
- [ ] 失败路径记录了失败签名与换路依据？
- [ ] 结论证据链（Evidence → Finding → Path）齐全、可复现？
- [ ] 评测模式：flag 以平台 submit 成功为准？真实模式：scope 内、痕迹已清理？
