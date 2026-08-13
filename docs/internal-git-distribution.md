# CyberStrikeAI 内部分发与上游同步说明

> 适用对象：内部通过 git 分发/协作的成员。
> 最后更新：2026-08-13（cairn-canonical 分支固化日）

---

## 1. 仓库拓扑与分支模型

```
origin (GitHub, 公开上游)
  └── main            # 上游主干（Ed1s0nZ/CyberStrikeAI，发布式提交）

本地/内部远端
  └── cairn-canonical # 本地增强主干：v1.7.12 + Cairn canonical + 历史本地改动
       ├── 3bb8ec5    v1.7.12（上游基线）
       ├── 414e82b    cairn: SQLite canonical state cutover (phase 1/2)
       └── e488089    chore: accumulate verified local agent/role/tool/web changes
```

- **永远在 `cairn-canonical` 上工作**，不要提交到 `main`、不要留在 detached HEAD。
- 上游合入方向是单向的：`origin/main → cairn-canonical`（merge）。
- `cairn-canonical` 推送到内部远端后，其他人 clone 该分支即可获得全部功能。

## 2. Cairn canonical 改动（本次大头，v0.8.13 运行时）

### 2.1 行为开关

```yaml
# config.yaml
multi_agent:
  cairn_sqlite_canonical_enabled: false   # 默认关闭 = 保持 legacy YAML 行为
  cairn_state_dir: state                  # 仅 legacy/导出用
```

- `false`（默认）：Cairn 走原有 YAML v2 双写路径，**行为与 v1.7.12 完全一致**。
- `true`：运行链切换到 SQLite canonical 状态机（下述），旧 YAML 变只读导出。

### 2.2 canonical 模式的行为变化

| 维度 | legacy | canonical（flag=true） |
|---|---|---|
| 状态存储 | YAML 文件（`state/{project}/{conversation}-state.yaml`） | SQLite（`cairn_*` 15 张表，与主库同文件） |
| Intent 标识 | description 字符串 | 规范 UUID + ordinal |
| 失败处理 | 错误/超时/空结果会写成 Fact 污染黑板 | **绝不创建 Fact**，走 `FailCairnIntentExecution` |
| 事实落黑板 | 运行链直接 upsert | outbox 异步投影（`project_fact_projection`） |
| 终态 | 不完整（取消常丢） | run 状态机：completed/failed/cancelled/timed_out，root finalizer 兜底 |
| Scope | 无 | Run 创建时冻结项目 scope snapshot（sha256） |

### 2.3 验证 canonical

```bash
go test ./internal/database ./internal/multiagent   # 仓储 + outbox 投影测试
# 开 flag 后跑一个绑定项目的 Cairn 会话，观察:
#   sqlite3 data/cyberstrike.db "SELECT id,status FROM cairn_runs ORDER BY created_at DESC LIMIT 3"
#   curl "http://127.0.0.1:8080/api/projects/<pid>/cairn/state?conversation_id=<cid>"
#      -> 响应 storage=sqlite_canonical 表示读侧已切 canonical
```

> ⚠️ 上线纪律：先保持 flag=false 跑稳定 → 单会话开 true 冒烟 → 观察 `cairn_runs`/`cairn_outbox`
> 投影是否闭环 → 再全量开启。回滚 = 关 flag + 重启，无数据迁移负担。

## 3. 同步上游（唯一入口：脚本）

```bash
scripts/sync-upstream.sh            # 备份→fetch→merge→build/test
scripts/sync-upstream.sh --dry-run  # 只看差异不 merge
```

- 用 **merge**，不用 rebase（本地 commit 不被重写，冲突好解）。
- 脚本会自动 tar 备份到 `~/skill-backups/`，冲突时给出文件清单并退出。
- **两个永久红线**：
  1. 绝不要 `git clean -fd` —— `skills/`、`knowledge_base/` 是 untracked 运行资产，clean 会全删。
  2. 绝不要提交 `config.yaml`（含密钥，已在 .gitignore）。

## 4. 冲突处理指南

上游合入大概率触碰的本地文件（我们的改动都很小，冲突好解）：

| 文件 | 本地改动性质 | 冲突时怎么办 |
|---|---|---|
| `internal/multiagent/runner.go` | Cairn case 传 GoalText/InputHash（~590 行区） | 上游若改 tool call index（~900 行区），git 通常自动合并；真冲突则两边都保留 |
| `internal/config/config.go` | +15 行（cairn 字段） | 上游加配置字段会上下文冲突；两边都留 |
| `internal/database/database.go` | +4 行（initCairnTables 调用） | 两边都留 |
| `internal/database/conversation.go` | 1 行（模式白名单加 cairn） | 两边都留 |
| `internal/handler/project.go` | +163 行（cairn API canonical 分支） | 冲突时保留 canonical 分支 + 上游逻辑 |
| `internal/app/app.go` | +4 行（outbox worker 启动） | 两边都留 |

**处理顺序**：
1. `git merge origin/main` 冲突后，逐个文件手动合并（上游修复保留、本地增量保留）。
2. `git add <文件>` → `git commit`（保留 merge commit）。
3. 验证：
   ```bash
   go build ./cmd/... ./internal/...
   go test ./internal/database ./internal/config ./internal/multiagent ./internal/handler ./internal/app
   git diff --check
   ```
4. 若结构冲突大（如上游也做了 Cairn 类似功能），对照本文档 §2.2 的行为矩阵决定取舍，必要时人工评审。

**新增文件永不冲突**：`internal/database/cairn*.go`、`internal/multiagent/cairn_*.go`、
`agents/orchestrator-cairn.md`、`agents/executor-cairn.md` —— 除非上游也用了同名。

## 5. 新机器初始化（内部分发）

```bash
# 1. 系统依赖（apt + Go 1.25+ + GOPROXY）
scripts/setup-deps.sh

# 2. 工作区初始化（config 生成 + skills 同步 + 构建）
scripts/init-workspace.sh          # 只初始化
scripts/init-workspace.sh --start  # 初始化并启动服务

# 3. 技能包（source of truth）
git clone <内部git>/cyberstrike-skill-kb-pack ~/cyberstrike-skill-kb-pack
scripts/init-workspace.sh          # 会自动从 pack 同步到 skills/
```

注意：
- `skills/` 由 `~/cyberstrike-skill-kb-pack` 的 `install_into_cyberstrike.py --mode merge` 同步，
  **不要直接改运行时 skills/ 再期望入库**；改 pack 再同步（`--mode merge` 不覆盖已存在文件，需手动 cp 验证）。
- 服务是 `nohup ./cyberstrike-ai --http`，不是 systemd；重启见 `init-workspace.sh --start`。

## 6. 目录约定（哪些入库、哪些不入库）

| 路径 | 状态 | 说明 |
|---|---|---|
| `internal/` `agents/` `roles/` `tools/` `web/` `scripts/` `docs/` `cmd/` | ✅ 入库 | 产品源码与资产 |
| `skills/` | ❌ untracked | 由 pack 仓库同步；**git clean 会删** |
| `knowledge_base/` | ❌ untracked | 按场次生成的检索知识，体积大 |
| `state/` `sessions/` `workspace*/` `reports/` `evals/` | ❌ untracked | 运行时产物 |
| `config.yaml` `*.key` `*.pem` `.env*` | ❌ 忽略 | 密钥，禁止入库 |
| `tmp/` | ❌ 忽略 | 探针/临时分析文件（注意 `go build ./...` 会误扫其中的 .go 文件，构建用 `./cmd/... ./internal/...`） |

## 7. 回滚

- **功能回滚**：`cairn_sqlite_canonical_enabled: false` + 重启，回到 legacy 行为，SQLite 表无副作用（additive）。
- **代码回滚**：`git revert <commit>` 或切回基线 tag。备份在 `~/skill-backups/cyberstrike-pre-sync-*.tar.gz`。
