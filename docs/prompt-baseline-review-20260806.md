# CyberStrikeAI 最新基线复盘与提示词优化（2026-08-06）

## 结论

仓库当前没有一份可称为“正式在线 baseline”的 score-bearing 结果。最新评测证据是 2026-07-23 的上下文恢复审计；它验证了配置和代码测试，但明确没有调用官方 benchmark 状态，也没有 post-fix live run。更早的运行审计只恢复出 B-02 `2/6`，后续没有可核验的 submit/awarded/score 事件。因此本轮不能声称线上得分已经改善，只能确认提示词结构、Skill 规范和本地回归测试改善。

最新目录扫描是 2026-08-04 的 Dirsearch 结果，它只能证明扫描工具执行过，不能衡量角色提示词或 Skill 质量。

## 最新基线证据

### 1. 最新评测审计：context recovery

来源：`.glasswing/runs/20260723-164537-context-recovery/`

- `report.md`：本地会话最后恢复于 16:35:52，16:36:22 被取消；当时无 CyberStrikeAI/Eino/plantask 进程、workflow/batch 记录或 checkpoint。
- 根因是控制面信息没有作为 pinned 非秘密事实保存；历史被摘要或裁剪后，模型保留了漏洞链，却丢失官方接口、任务状态和 submit/close 规程。
- 本地验证仅包含 YAML 解析以及 `go test ./internal/project ./internal/multiagent ./internal/agents` 的 227 项测试。
- `coverage-matrix.json` 把 `official benchmark status` 标为 `gap`，证据为 `no live call performed`。
- `validator-results.jsonl` 的两个 confirmed 项仍分别缺少 `post-fix live run` 和 `external platform state`。

判定：这是有效的静态/恢复审计，不是线上模型效果基线。

### 2. 最近一次有分数上下文的运行审计

来源：`.glasswing/runs/20260723-154306-prompt-runtime-tuning/report.md`

- 恢复出的既有状态为 B-02 `2/6`。
- 新角色和 orchestrator 已被加载，但出现大规模角色扇出、重复方法、两次 10 秒目标超时、脚本错误和摘要后再次膨胀。
- 没有 verified submit、awarded、完成事件或最终答复。
- 当时建议的 A/B 指标是：10/30 分钟正确 flag、首次 score-bearing 行动耗时、重复失败、工具错误/超时、token 峰值、摘要、子代理数量和 checkpoint。

判定：`2/6` 只能作为恢复记录，不能作为本轮可比较的官方分数。

### 3. 基线缺失项

- 未绑定可重放的 prompt/Skill 快照或 dirty-tree content hash；
- 未固定并记录模型、推理/温度、工具注册表和知识库快照；
- 未保存官方 start/status/submit 的完整事件链；
- 未对同一可重置 fixture 做多次 control/candidate 试验；
- 未统一采集 token、延迟、失败签名、摘要和子代理指标。

新的可执行评测契约见 `evals/prompt-runtime-v1.yaml`。

## 优化依据

本轮按 OpenAI 官方提示工程建议执行：指令置前并把指令与上下文分隔；具体说明目标、长度、格式和风格；示例只保留用于纠正实测缺口的部分；用“应该做什么”的正向动作替代堆叠否定句。当前模型指南还建议让系统提示词更精简、每条规则只出现一次、只暴露相关工具，并在代表性任务上逐组删除与复测。

对 Skill 使用渐进披露：`SKILL.md` 只保留触发、前置、决策流程、停止条件、输出和 reference 路由；大目录、案例和方法细节留在按需加载的 `references/`。

参考：

- https://help.openai.com/zh-hans-cn/articles/6654000-best-practices-for-prompt-engineering-with-the-openai-api
- https://developers.openai.com/api/docs/guides/latest-model#prompting-best-practices
- https://developers.openai.com/codex/skills

## 已实施修改

### 提示词收敛

- `agents/orchestrator.md`：统一 scope、委派、Skill/工具路由、证据、失败预算、黑板、benchmark 恢复和输出契约；删除重复授权段、口号式“高强度”内容、美元赏金阈值和强制展示思考过程。
- `roles/CTF.yaml`：从完整第二套编排规则改为评测差异层，只保留控制面恢复、最窄 Skill 路由、时间盒、flag 确认和 checkpoint。
- `roles/Orchestrator.yaml`：从 354 行调度 DSL 改为 23 行角色适配层；明确 prompt 不创造 async/poll/cancel 能力。

### Skill 渐进披露

- `skills/ctf-sandbox-orchestrator/SKILL.md`：移除全量 child-skill 清单，改为 `When to use / Preconditions / Procedure / Stop conditions / Output`，具体映射按需读取 `references/router-matrix.md`。
- `skills/offensive-skill-router/SKILL.md`：移除重复全量包目录，按需读取 `references/categories.json` 与 operation discipline。
- 修正 `offensive-bug-identification`、`offensive-windows-boundaries` 正文 Metadata 与 frontmatter 名称不一致。

### 体量变化

| 文件 | 修改前 | 修改后 |
| --- | ---: | ---: |
| `agents/orchestrator.md` | 169 行 / 16,717 B | 82 行 / 6,032 B |
| `roles/CTF.yaml` | 127 行 / 10,863 B | 72 行 / 3,788 B |
| `roles/Orchestrator.yaml` | 354 行 / 18,283 B | 23 行 / 1,501 B |
| `ctf-sandbox-orchestrator/SKILL.md` | 142 行 / 7,522 B | 56 行 / 3,820 B |
| `offensive-skill-router/SKILL.md` | 192 行 / 4,233 B | 50 行 / 2,953 B |
| **合计** | **984 行 / 57,618 B** | **283 行 / 18,094 B** |

合计减少 701 行（71.2%）和 39,524 字节（68.6%）。这只是静态上下文体量变化，不能替代线上质量评测。

## 新增质量门

新增 `go run ./cmd/prompt-audit`，覆盖：

- Agent Markdown、角色 YAML 和 Skill manifest 可解析性；
- Skill frontmatter 名称与目录一致性；
- 触发、前置、流程、停止和输出章节；
- Agent/role/Skill 的建议体量阈值；
- `references/`、`scripts/`、`assets/` 引用是否存在；
- Skill 正文 Metadata 名称是否与 manifest 漂移。

本轮静态结果：16 agents、16 roles、180 skills 全部可解析；角色 prompt 无超限。存量债务为 2 个超限 Agent、65 个超大 Skill、57 个断链 reference、23 个正文名称漂移；缺显式 output/preconditions/procedure/stop/trigger 的数量分别为 164/177/73/178/134。章节统计是启发式 warning，不是硬规范失败。

## 验证结果

- `go test ./internal/promptaudit ./internal/skillpackage ./internal/config ./internal/agents ./internal/multiagent`：236 tests passed（5 packages）。
- `go run ./cmd/prompt-audit`：Agent/role/Skill 解析错误均为 0。
- `node skills/attack-chain/scripts/validate-routes.mjs`：21/21 fixtures passed，errors 为 0；15 个 ATT&CK tactics 中 10 个 supported、5 个显式 gap。
- `go test ./internal/...`：758 passed、1 failed。失败项为既有的 `TestEinoStreamingShell_SudoFailsFast`：当前测试环境以 root 运行，`sudo whoami` 成功并输出 `root`，因此测试预期的 sudo error text 不存在；与本轮 prompt/Skill 变更无关。
- 未运行实时靶场、外部扫描或 flag 提交：当前没有可恢复的官方任务状态和可重置 fixture，执行 live A/B 会混入未知外部状态。

## 后续优先级

1. 先修 57 个断链 reference；断链会直接让渐进披露失败。
2. 把 65 个超大 Skill 分批拆到 `references/`，优先治理 `offensive-windows-boundaries`、`offensive-windows-mitigations`、`offensive-vuln-classes`。
3. 为高频/高风险 Skill 补 stop/output/preconditions；启发式 warning 暂不设为 CI hard fail。
4. 获取可重置 B-02 fixture 和官方 status/submit 接口后，按 `evals/prompt-runtime-v1.yaml` 做每个 variant 至少 3 次 A/B；只有官方得分、可靠性和 token/延迟指标同时通过门槛，才能把本轮优化认定为新 baseline。
