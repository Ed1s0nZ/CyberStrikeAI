# Stage and Skill Routing

本文件只提供路由规则，不提供 payload 或链式攻击步骤。
`skill:<name>` 表示相邻技能目录 `<skills-root>/<name>/SKILL.md`。
路由的唯一机器可读来源是 [attack-v19-routes.json](attack-v19-routes.json)；
本文件是它的可读说明。

## 目录

1. [解析规则](#1-解析规则)
2. [触发与排除](#2-触发与排除)
3. [P1 目标与上下文](#3-p1-目标与上下文)
4. [P2 侦察与发现](#4-p2-侦察与发现)
5. [P3 威胁建模](#5-p3-威胁建模)
6. [P4 漏洞分析](#6-p4-漏洞分析)
7. [P5 可控利用与能力验证](#7-p5-可控利用与能力验证)
8. [P6 后渗透与目标证明](#8-p6-后渗透与目标证明)
9. [P7 清理与核验](#9-p7-清理与核验)
10. [P8 报告](#10-p8-报告)
11. [CTF 与跨域路由](#11-ctf-与跨域路由)
12. [手动选择的能力](#12-手动选择的能力)
13. [路由示例](#13-路由示例)

## 1. 解析规则

1. 先判断当前 Task 的方法论阶段（P1–P8），再选择 route。
2. Web/API 任务先确定 WSTG 测试域与 `vuln_class`，再进入专项。
3. 一个 epoch 最多加载一个 router 和一个 specialist。
4. 加载前确认相邻 `<name>/SKILL.md` 存在，且 frontmatter `name` 与目录一致。
5. 不支持 `offensive-wifi*`、`competition-*` 等通配名。
6. 候选并列、前置 capability/evidence 不足、工具不可用或无映射时返回 routing gap；
   使用 Runtime 原生工具完成 bounded Task，或在 TaskResult 中记录缺口；
   不得虚构 skill、工具或文件。
7. `competition-*` 只在 constraints 明确声明隔离 CTF/benchmark 后，由
   `ctf-sandbox-orchestrator` 下选。
8. 下游 skill 不得修改框架设定的 TaskEnvelope 基础约束。

安装或更新后运行：

```bash
node .agents/skills/attack-chain/scripts/validate-routes.mjs
```

## 2. 触发与排除

加载 attack-chain：

- Root Goal 跨两个及以上因果阶段；
- 跨信任边界（外到内、云到主机、Web 到 AD）；
- 当前 Task 需要阶段交接。

不加载 attack-chain（直接进入专项或 Runtime 工具）：

- 单端口扫描：`skill:recon-port-scan`
- 单漏洞验证：按 `vuln_class` 选择对应 `offensive-*`/`exploit-*` 专项
- 单逆向任务：`skill:reverse-skill-router` 或具体逆向专项
- 仅报告任务：`skill:offensive-reporting` / `skill:pentest-report` / `skill:docs-generator`

`excludes.single_stage=true` 的 route（横向、权限、云凭据等）不允许作为单阶段任务路由。

## 3. P1 目标与上下文

P1 不加载攻击专项。

- 目标或允许动作不明确：停止目标侧工具并交回 blocker。
- 明确隔离靶场：允许进入 lab/CTF 路由。
- 真实授权环境：使用保守路由，按 TaskEnvelope 约束执行。

## 4. P2 侦察与发现

| 目标 | route | 主 skill |
|---|---|---|
| 公网资产与公开情报 | `route:recon-open-source` | `skill:offensive-osint` |
| OSINT 来源规划 | `route:recon-osint-planning` | `skill:offensive-osint-methodology` |
| 子域名枚举 | `route:recon-subdomain-enum` | `skill:recon-subdomain` |
| 端口与服务基线 | `route:recon-service-scan` | `skill:recon-port-scan` |
| Web 目录/路径面 | `route:recon-web-content` | `skill:recon-dir-scan` |
| 产品、框架与版本指纹 | `route:recon-fingerprint` | `skill:recon-fingerprint` |
| 云服务/资源发现 | `route:cloud-service-discovery` | `skill:offensive-cloud` |
| K8s 资源发现 | `route:k8s-resource-discovery` | `skill:cloud-k8s` |
| AD 域信息（已有域会话） | `route:ad-domain-discovery` | `skill:offensive-active-directory` |

不同资产或不同前置条件已经独立时才能并行。只有一个未知入口时先做一个入口认知 Task，
不要按漏洞类别并行扇出。产品名称或版本线索不会自动确认 CVE。

## 5. P3 威胁建模

| 目标 | route | 主 skill |
|---|---|---|
| Web/API 攻击面与 Hypothesis | `route:tm-web-attack-surface` | `skill:offensive-bug-identification` |

退出 P3 时保留攻击面清单、候选 Hypothesis 与优先级；Hypothesis 必须在目标范围验证。

## 6. P4 漏洞分析

### Web/API（先 WSTG 域，再 vuln_class）

类别不清楚：先 `route:web-classify-router`（`skill:offensive-skill-router`）。
广谱快查：`route:web-triage-fast`（`skill:offensive-fast-checking`）。

| WSTG 域 | vuln_class | 主 skill |
|---|---|---|
| INPV | sqli | `skill:offensive-sqli` |
| INPV | ssrf | `skill:offensive-ssrf` |
| INPV | ssti | `skill:offensive-ssti` |
| INPV | rce | `skill:offensive-rce` |
| INPV | xxe | `skill:offensive-xxe` |
| INPV | deserialization | `skill:offensive-deserialization` |
| INPV | lfi | `skill:exploit-lfi` |
| INPV | file-download | `skill:exploit-file-download` |
| INPV | parameter-pollution | `skill:offensive-parameter-pollution` |
| INPV | request-smuggling | `skill:offensive-request-smuggling` |
| CLNT | xss | `skill:offensive-xss` |
| CLNT | open-redirect | `skill:offensive-open-redirect` |
| BUSL | business-logic | `skill:offensive-business-logic` |
| BUSL | race-condition | `skill:offensive-race-condition` |
| BUSL | file-upload | `skill:offensive-file-upload` |
| ATHZ | idor | `skill:offensive-idor` |
| SESS | jwt | `skill:offensive-jwt` |
| ATHN | oauth | `skill:offensive-oauth` |
| ATHN | credential_testing | `skill:security-passwords` |
| IDNT | identity-federation | `skill:identity-federation` |
| APIT | api | `skill:api-security` |

### 其他 P4 route

| 目标 | route | 主 skill |
|---|---|---|
| WAF 规则有效性（已有拦截观测） | `route:defimp-waf-bypass` | `skill:offensive-waf-bypass` |
| 云元数据凭据可得性（SSRF 后继） | `route:cloud-metadata-credential` | `skill:offensive-cloud` |
| 补丁差分到利用点 | `route:resdev-patch-to-exploit` | `skill:patch-diff-exploit` |
| 协议/格式 fuzz（非 ATT&CK） | `route:vuln-fuzzing` | `skill:offensive-fuzzing` |
| 白盒审计（非 ATT&CK） | `route:vuln-code-audit` | `skill:code-audit` |
| 数据库安全核查（非 ATT&CK） | `route:vuln-database-audit` | `skill:database-security` |
| Windows 缓解机制核对 | `route:privesc-windows-mitigation-check` | `skill:offensive-windows-mitigations` |

先调用 `vulnerability_search`/`web_fetch` 只会形成情报线索；确认必须来自授权目标的
正负对照、稳定动态信号或可验证副作用。

## 7. P5 可控利用与能力验证

只有 Task 明确动作与成功条件时选择：

| 环境 | route | 主 skill |
|---|---|---|
| 命令执行能力确认 | `route:exec-command-validation` | `skill:offensive-rce` |
| Windows 权限边界 | `route:privesc-windows-boundary` | `skill:offensive-windows-boundaries` |
| 容器/K8s 边界 | `route:privesc-container-escape` | `skill:cloud-k8s` |
| 云身份权限边界 | `route:cloud-iam-privesc` | `skill:offensive-cloud` |
| AD Kerberoast | `route:ad-kerberoast` | `skill:offensive-active-directory` |
| 字典/批量凭据验证 | `route:cred-dictionary-attack` | `skill:security-passwords` |

真实环境只确认最小能力，不批量抓取凭据、不建立持久化、不访问无关数据。
需要新的主机权限或独立验收条件时提交 `partial`，由 Planner 决定后继 Task。

## 8. P6 后渗透与目标证明

| 目标 | route | 主 skill |
|---|---|---|
| AD 横向（SMB/Admin Shares） | `route:ad-lateral-smb` | `skill:offensive-active-directory` |
| 替代认证材料（PtH/PtT） | `route:ad-alternate-auth-material` | `skill:offensive-active-directory` |
| 云存储对象证明 | `route:cloud-storage-collection` | `skill:offensive-cloud` |

P6 默认不加载新攻击 skill。复用前驱阶段已验证的最小能力满足 successCriteria：
只证明必要的权限、读取范围或安全边界；使用合成、非敏感或最小记录；
不做全库导出、真实数据外传或额外横向。

route 生命周期使用 Runtime 工具，不依赖第三方 skill：创建前确认 `route_open` 可用、
CIDR 在 scope、使用 credential Ref；复用前先 `route_status`；stale 时只对同一 Ref
`route_reconnect`；收束时 `route_stop` 不再需要的 route 并持久化结果。

## 9. P7 清理与核验

P7 不加载规避或持久化 skill，使用 `route:cleanup-managed-routes`（仅 Runtime 工具）：

- 停止本 Task 创建且不再需要的受管 route。
- 回滚 Task 明确创建的临时对象或配置；不可逆或来源不明时不要擅自删除。
- 记录无法回滚的残留、原因、负责方与建议动作。
- 以 observation/Artifact 证明清理结果；"已尝试"不等于"已清理"。

## 10. P8 报告

| 交付 | route | 主 skill |
|---|---|---|
| 安全评估报告 | `route:reporting-assessment` | `skill:offensive-reporting` |
| 渗透测试报告 | `route:reporting-pentest` | `skill:pentest-report` |
| 通用结构化文档 | `route:reporting-docs` | `skill:docs-generator` |

报告必须区分 confirmed、hypothesis、inconclusive 与 negative finding，
并包含 scope、方法、证据 Ref、复现边界、影响、修复建议、覆盖缺口和清理状态。

## 11. CTF 与跨域路由

多阶段隔离评测：`route:ctf-orchestrated`（router `skill:ctf-sandbox-orchestrator`），
由编排器按题型下选 `competition-*` 下游；attack-chain 不一次加载整个 competition 家族。

通用 CTF 单题型不经过 attack-chain，直接加载：`ctf-web`、`ctf-pwn`、`ctf-reverse`、
`ctf-forensics`、`ctf-osint`、`ctf-crypto`、`ctf-malware`、`ctf-misc`、`ctf-ai-ml`、
`ctf-writeup`、`solve-challenge`。

二进制、移动、AI 与硬件/无线域为 domain overlay（注册表 `domain_overlays`）：

| 领域 | Router/主 skill |
|---|---|
| Reverse 分类 | `skill:reverse-skill-router` |
| Pwn chain | `skill:pwn-chain` |
| Binary diff | `skill:binary-diff` |
| Mobile reverse | `skill:mobile-reverse` / `skill:apk-reverse` |
| LLM security | `skill:llm-security` / `skill:llm-testing` |
| Hardware | `skill:hardware-security` |
| Wireless/RF | `skill:wifi-wireless` 等 `overlay:wireless-rf` |

这些领域如果当前 Task 只有一个 bounded 目标，直接加载专项，不经过 attack-chain。

## 12. 手动选择的能力

以下 skill 存在，但注册表不自动路由（`manual_only_skills`）：

- `skill:edr-bypass-re`
- `skill:offensive-advanced-redteam`
- `skill:offensive-initial-access`

只有人工显式选择时才加载；即使被选择，也不得修改框架设定的 TaskEnvelope 基础约束。

## 13. 路由示例

### 多阶段 Web 到内网评估

P2 只做入口与指纹（`route:recon-fingerprint`）→ P3 攻击面（`route:tm-web-attack-surface`）
→ P4 只验证一个证据相符的 Hypothesis（按 WSTG 域选择专项）→
以 `partial + suggestedNextGoal` 交回 Planner → 新 Task 获得明确动作后进入 P5/P6 →
P6 最小证明 → P7 清理 → P8 报告。每个阶段只加载当下专项，不同时读取整条链。

### 单一 SQLi 验证

直接加载 `skill:offensive-sqli`。不使用 attack-chain。

### 云入口到权限边界

P2 用 `route:recon-fingerprint` 建立云产品/接口证据 →
P4 在一个 Task 中选择 `route:web-inpv-ssrf` 或 `route:cloud-metadata-credential` 验证当前边界 →
需要新的身份、账户或主机边界时提交阶段结果，由 Planner 创建依赖 Task。

### 多阶段 CTF

P1 确认隔离评测约束 → `route:ctf-orchestrated` 由编排器下选 `competition-*` →
flag/证明落库 → P8 writeup（`skill:ctf-writeup`）。
