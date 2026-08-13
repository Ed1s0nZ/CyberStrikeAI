#!/usr/bin/env bash
# =============================================================================
# CyberStrikeAI 上游同步脚本
# -----------------------------------------------------------------------------
# 用法:
#   scripts/sync-upstream.sh            # 备份 -> fetch -> merge -> 验证
#   scripts/sync-upstream.sh --dry-run  # 只 fetch + 报告，不 merge
#   scripts/sync-upstream.sh --no-backup
#
# 设计约束（务必遵守，避免踩坑）:
#   1. 使用 merge（不用 rebase）——本地 cairn-canonical 的 commit 不会被重写。
#   2. 绝不执行 `git clean -fd` —— skills/、knowledge_base/ 是 untracked 运行资产，
#      clean 会直接删除。
#   3. merge 前自动 tar 备份（可 --no-backup 跳过）。
#   4. skills/ 与 knowledge_base/ 不打包（体积大且分别由 pack 同步 / 场景生成）。
#   5. 仅在 cairn-canonical 分支上允许 merge；detached HEAD 或 main 会拒绝。
# =============================================================================
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

DRY_RUN=0
DO_BACKUP=1
for arg in "$@"; do
  case "$arg" in
    --dry-run) DRY_RUN=1 ;;
    --no-backup) DO_BACKUP=0 ;;
    *) echo "未知参数: $arg" >&2; exit 2 ;;
  esac
done

# ---------- 0. 环境与分支检查 ----------
if ! command -v git >/dev/null 2>&1; then
  echo "[错误] 未找到 git" >&2; exit 1
fi
CURRENT_BRANCH="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || true)"
if [[ "$CURRENT_BRANCH" == "HEAD" || -z "$CURRENT_BRANCH" ]]; then
  echo "[错误] 当前处于 detached HEAD，请先执行: git checkout cairn-canonical" >&2
  exit 1
fi
if [[ "$CURRENT_BRANCH" != "cairn-canonical" ]]; then
  echo "[警告] 当前分支 '$CURRENT_BRANCH' 不是 cairn-canonical，同步前请确认意图。" >&2
  read -r -p "继续？(y/N) " ans
  [[ "${ans,,}" == "y" ]] || { echo "已取消"; exit 1; }
fi

# ---------- 1. 备份 ----------
if [[ "$DO_BACKUP" == "1" ]]; then
  BACKUP_DIR="${BACKUP_DIR:-$HOME/skill-backups}"
  STAMP="$(date +%Y%m%d-%H%M%S)"
  TARBALL="$BACKUP_DIR/cyberstrike-pre-sync-$STAMP.tar.gz"
  mkdir -p "$BACKUP_DIR"
  echo "[1/4] 备份工作树 -> $TARBALL"
  tar czf "$TARBALL" \
    --exclude=.git --exclude=skills --exclude=knowledge_base \
    --exclude=state --exclude=sessions --exclude=workspace --exclude=workspaces \
    --exclude=reports --exclude=evals --exclude=tmp \
    .
  echo "      备份完成: $(du -h "$TARBALL" | cut -f1)"
else
  echo "[1/4] 跳过备份 (--no-backup)"
fi

# ---------- 2. fetch ----------
echo "[2/4] git fetch origin"
git fetch origin

UPSTREAM_HEAD="$(git rev-parse origin/main)"
LOCAL_HEAD="$(git rev-parse HEAD)"
MERGE_BASE="$(git merge-base HEAD origin/main 2>/dev/null || true)"

if [[ "$UPSTREAM_HEAD" == "$LOCAL_HEAD" ]]; then
  echo "[结果] 本地已与 origin/main 一致，无需合并。"
  exit 0
fi

if [[ -z "$MERGE_BASE" ]]; then
  echo "[错误] 本地分支与 origin/main 无共同祖先（历史被重写？），请人工介入。" >&2
  exit 1
fi

echo "      origin/main 领先本地: $(git rev-list --count "$MERGE_BASE"..origin/main) 个 commit"
echo "      本地领先 origin/main: $(git rev-list --count "$MERGE_BASE"..HEAD) 个 commit"

# ---------- 3. merge ----------
if [[ "$DRY_RUN" == "1" ]]; then
  echo "[3/4] dry-run 模式，跳过 merge。"
  echo "      执行: git merge origin/main"
  exit 0
fi

echo "[3/4] git merge origin/main"
if git merge origin/main -m "merge: upstream origin/main ($(git log -1 --format=%h origin/main))"; then
  echo "      merge 成功"
else
  echo "[错误] merge 产生冲突。" >&2
  echo "------------------------------------------------------------" >&2
  echo "冲突文件:" >&2
  git diff --name-only --diff-filter=U 2>/dev/null | sed 's/^/  /' >&2 || true
  echo "------------------------------------------------------------" >&2
  echo "处理指南见 docs/internal-git-distribution.md「冲突处理」。" >&2
  echo "原则：上游代码保留 + 本地 Cairn 增量保留；解完再执行 git add + git commit。" >&2
  exit 1
fi

# ---------- 4. 验证 ----------
echo "[4/4] 验证（build + test + diff check）"
git diff --check || { echo "[错误] 空白错误"; exit 1; }

if command -v go >/dev/null 2>&1; then
  go build ./cmd/... ./internal/... || { echo "[错误] 编译失败"; exit 1; }
  echo "      go build OK"
  go test ./internal/database ./internal/config ./internal/multiagent ./internal/handler ./internal/app 2>/dev/null \
    && echo "      go test OK" \
    || echo "      [警告] 部分测试失败，请人工确认（见上方输出）"
else
  echo "      [警告] 未找到 go，跳过 build/test"
fi

echo
echo "============================================================"
echo "同步完成。当前 HEAD: $(git log -1 --oneline)"
echo "记住：skills/ 与 knowledge_base/ 永远不提交、不 clean。"
echo "============================================================"
