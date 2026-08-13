#!/usr/bin/env bash
# =============================================================================
# CyberStrikeAI 工作区初始化脚本
# -----------------------------------------------------------------------------
# 用法: scripts/init-workspace.sh [--build-only] [--no-skills] [--start]
#
# 功能:
#   1. 检查 config.yaml（缺失则从 config.example.yaml 复制并提醒改密钥）
#   2. 同步 skills 技能包（存在 cyberstrike-skill-kb-pack 时；source of truth 是 pack）
#   3. 构建二进制 cyberstrike-ai
#   4. (可选 --start) 启动服务：nohup ./cyberstrike-ai --http
#
# 已知约束:
#   - skills/ 目录是 untracked 运行资产，由 pack 仓库 -> install 脚本同步，
#     不要直接手工编辑运行时的 skills/（改 pack 再同步）。
#   - 服务不是 systemd，是 nohup 手动拉起；重启时注意 kill 旧进程。
# =============================================================================
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

BUILD_ONLY=0
NO_SKILLS=0
START_SERVICE=0
for arg in "$@"; do
  case "$arg" in
    --build-only) BUILD_ONLY=1 ;;
    --no-skills) NO_SKILLS=1 ;;
    --start) START_SERVICE=1 ;;
    *) echo "未知参数: $arg" >&2; exit 2 ;;
  esac
done

echo "==> [1/4] 配置文件"
if [[ ! -f config.yaml ]]; then
  if [[ ! -f config.example.yaml ]]; then
    echo "[错误] 既无 config.yaml 也无 config.example.yaml" >&2; exit 1
  fi
  cp config.example.yaml config.yaml
  echo "    已从 config.example.yaml 生成 config.yaml"
  echo "    [重要] 请编辑 config.yaml 填入 API key / 模型地址 / 端口，且该文件不入库"
else
  echo "    config.yaml 已存在，跳过"
fi
if [[ "$BUILD_ONLY" == "1" ]]; then
  echo "==> 配置完成（--build-only）。如需同步技能包/启动服务，去掉该参数重跑。"
fi

# ---------- skills 同步 ----------
PACK_DIR="${PACK_DIR:-$HOME/cyberstrike-skill-kb-pack}"
if [[ "$NO_SKILLS" == "1" ]]; then
  echo "==> [2/4] 跳过 skills 同步 (--no-skills)"
elif [[ -f "$PACK_DIR/scripts/install_into_cyberstrike.py" ]]; then
  echo "==> [2/4] 从 pack 同步技能包: $PACK_DIR"
  if [[ -x "$PACK_DIR/.venv/bin/python" ]]; then
    PY="$PACK_DIR/.venv/bin/python"
  else
    PY=python3
  fi
  "$PY" "$PACK_DIR/scripts/install_into_cyberstrike.py" --mode merge \
    --skills-dir "$ROOT_DIR/skills" || {
      echo "[警告] pack 安装脚本执行失败（可能参数不同），请人工执行：" >&2
      echo "  $PY $PACK_DIR/scripts/install_into_cyberstrike.py --mode merge" >&2
    }
else
  echo "==> [2/4] 未找到 pack 仓库（$PACK_DIR），跳过 skills 同步"
  echo "    技能包 source of truth: git 仓库 cyberstrike-skill-kb-pack"
fi

# ---------- 构建 ----------
echo "==> [3/4] 构建"
if ! command -v go >/dev/null 2>&1; then
  echo "[错误] 未找到 go，请先运行 scripts/setup-deps.sh" >&2; exit 1
fi
go build -o cyberstrike-ai.new ./cmd/server/main.go
mv cyberstrike-ai.new cyberstrike-ai
chmod +x cyberstrike-ai
echo "    构建完成: $(ls -la cyberstrike-ai | awk '{print $5, $9}')"

# ---------- 启动 ----------
if [[ "$START_SERVICE" == "1" ]]; then
  echo "==> [4/4] 启动服务"
  OLD_PID="$(pgrep -f 'cyberstrike-ai --http' | head -1 || true)"
  if [[ -n "$OLD_PID" ]]; then
    echo "    停止旧进程 PID=$OLD_PID"
    kill "$OLD_PID" || true
    sleep 1
  fi
  nohup ./cyberstrike-ai --http >> cyberstrike-ai.log 2>&1 &
  sleep 2
  NEW_PID="$(pgrep -f 'cyberstrike-ai --http' | head -1 || true)"
  echo "    新进程 PID=$NEW_PID，日志: cyberstrike-ai.log"
  echo "    健康检查: curl -s http://127.0.0.1:8080/api/health (若有)"
else
  echo "==> [4/4] 跳过启动（加 --start 启动服务）"
fi

echo
echo "完成。"
