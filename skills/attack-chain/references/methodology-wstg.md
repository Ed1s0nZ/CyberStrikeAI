# 方法论与 WSTG 分类映射

本文件定义 attack-chain 的三层正交分类，只在需要判定阶段、WSTG 测试域或
理解注册表字段时阅读。路由数据本身在
[attack-v19-routes.json](attack-v19-routes.json)，ATT&CK 固定快照在
[enterprise-attack-v19.json](enterprise-attack-v19.json)。

## 目录

1. [方法论主轴（NIST/PTES 生命周期）](#1-方法论主轴nistptes-生命周期)
2. [ATT&CK v19 覆盖层](#2-attck-v19-覆盖层)
3. [WSTG Web/API 测试域](#3-wstg-webapi-测试域)
4. [非 ATT&CK 域与 overlay](#4-非-attck-域与-overlay)

## 1. 方法论主轴（NIST/PTES 生命周期）

8 个阶段描述"当前 Task 要证明什么因果阶段"，决定退出条件与交接形状。
阶段 id 与注册表 `method_phases` 一一对应：

| id | 阶段 | 当前 Task 回答的问题 |
|---|---|---|
| `target-context-init` | Target & Context Initialization | 目标、约束、成功条件、可用工具是什么 |
| `recon-discovery` | Reconnaissance & Discovery | 入口、服务、版本、认证面是什么 |
| `threat-modeling` | Threat Modeling / Attack Surface | 哪里最可能有问题，先验哪个 Hypothesis |
| `vulnerability-analysis` | Vulnerability Analysis | 哪个 Hypothesis 与目标证据相符 |
| `controlled-exploitation` | Controlled Exploitation / Capability Validation | 目标能力是否真实存在（最小可逆确认） |
| `post-exploitation` | Post-exploitation / Objective Proof | 已确认能力是否满足业务目标 |
| `cleanup-verification` | Cleanup & Verification | 临时 route/会话/变更是否已回收 |
| `reporting` | Reporting & Lessons Learned | 证据、负结果、覆盖缺口是否齐全 |

判定规则：

- 只按**当前 Task** 的待证阶段归类；Root Goal 很大不展开全链。
- 一个 Task 只属于一个主阶段；跨阶段诉求用 `partial` + `suggestedNextGoal` 表达。
- 阶段不决定授权；授权与风险门禁由框架统一绑定，skill 层不重复实现。

## 2. ATT&CK v19 覆盖层

Enterprise ATT&CK v19 固定 15 个 tactic，作为**行为覆盖层**标注 route，
不是线性执行顺序。快照 `enterprise-attack-v19.json` 离线固定，运行时不联网更新；
版本升级必须显式重新生成快照并重跑 `validate-routes.mjs`。

15 个 tactic（快照 `tactics[]` 顺序）：

| ID | Name |
|---|---|
| TA0043 | Reconnaissance |
| TA0042 | Resource Development |
| TA0001 | Initial Access |
| TA0002 | Execution |
| TA0003 | Persistence |
| TA0004 | Privilege Escalation |
| TA0005 | Stealth |
| TA0112 | Defense Impairment |
| TA0006 | Credential Access |
| TA0007 | Discovery |
| TA0008 | Lateral Movement |
| TA0009 | Collection |
| TA0011 | Command and Control |
| TA0010 | Exfiltration |
| TA0040 | Impact |

映射规则：

- 只登记当前技能库**真实支持**的 technique/sub-technique；不确定的映射进入
  coverage gap，不猜测、不造占位覆盖。
- 每个 tactic 在 `attack-v19-coverage.json` 中只能是 `supported`（存在真实 route）
  或 `gap`（显式缺口）；gap 不计为覆盖。
- technique 的 tactic 归属以快照 kill chain 为准；注册表 `tactics` 必须与快照一致，
  validator 会机械校验。
- Persistence、Stealth、Command and Control、Exfiltration、Impact 当前无自动路由，
  在 coverage 中显式记为 gap。

## 3. WSTG Web/API 测试域

Web/API 任务**先**按 OWASP WSTG stable 的 12 个测试域分类，再在域内选择专项。
ATT&CK 视角下这些任务大多落在 T1190；不要用 ATT&CK 阶段区分 SQLi 与 OAuth。

| WSTG ID | 测试域 | 已登记专项（示例） |
|---|---|---|
| WSTG-INFO | Information Gathering | `recon-dir-scan`、`recon-fingerprint`、`offensive-bug-identification` |
| WSTG-CONF | Configuration & Deployment | `offensive-fast-checking`（暴露面/配置类快查） |
| WSTG-IDNT | Identity Management | `identity-federation` |
| WSTG-ATHN | Authentication | `offensive-oauth`、`security-passwords` |
| WSTG-ATHZ | Authorization | `offensive-idor` |
| WSTG-SESS | Session Management | `offensive-jwt` |
| WSTG-INPV | Input Validation | `offensive-sqli`、`offensive-ssrf`、`offensive-ssti`、`offensive-rce`、`offensive-xxe`、`offensive-deserialization`、`exploit-lfi`、`exploit-file-download`、`offensive-parameter-pollution`、`offensive-request-smuggling` |
| WSTG-ERRH | Error Handling | 无真实专项（不在注册表引用） |
| WSTG-CRYP | Cryptography | 无真实专项（不在注册表引用） |
| WSTG-BUSL | Business Logic | `offensive-business-logic`、`offensive-race-condition`、`offensive-file-upload` |
| WSTG-CLNT | Client-side | `offensive-xss`、`offensive-open-redirect` |
| WSTG-APIT | API Testing | `api-security`、`offensive-fast-checking` |

域内消歧：

- 同一测试域内有多个专项时，由 Task 的 `vuln_class`（SQLi/SSRF/JWT…）决定；
  注册表 `required.task.vuln_class` 是机械判据。
- 同域存在两代同类专项时（如 `offensive-sqli` 与 `exploit-sqli`），默认按 `priority`
  取一个；Task 显式给出 `preferred_skill` 时以它为准，其余同域 route 不再参与。
- 测试域本身不清楚时，先加载 router `offensive-skill-router` 做分类，再进入唯一专项。
- 域与条件都并列、无法唯一确定时返回 routing gap，不虚构选择。

## 4. 非 ATT&CK 域与 overlay

以下类别在注册表中显式标记，不强行映射 Enterprise ATT&CK：

- **CTF/benchmark**：`ctf-*`、`competition-*`、`solve-challenge`；多阶段隔离评测由
  `ctf-sandbox-orchestrator` 下选，attack-chain 不逐一直接路由。
- **报告类**：`offensive-reporting`、`pentest-report`、`docs-generator`；
  通过 `tactics`/`techniques` 为空的 reporting 路由进入。
- **逆向/取证/二进制**：见 `overlay:binary-reverse`；`reverse-skill-router` 是其内部 router。
- **工具/知识类**：`pentest-tools`、`security-patterns`、`offensive-vuln-classes` 等
  `overlay:knowledge-toolkit`。
- **Mobile/ICS/无线/硬件**：作为 domain overlay（`overlay:mobile-desktop`、`overlay:ics-ot`、
  `overlay:wireless-rf`、`overlay:hardware-embedded`），单域 bounded Task 直接加载专项。
- **手动选择**：`edr-bypass-re`、`offensive-advanced-redteam`、`offensive-initial-access`
  不自动路由，仅人工显式选择。
