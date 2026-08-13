#!/usr/bin/env bash
# =============================================================================
# CyberStrikeAI 环境依赖安装脚本（Debian/Ubuntu/Kali 系）
# -----------------------------------------------------------------------------
# 用法: scripts/setup-deps.sh [--skip-apt]
#
# 处理已知坑:
#   - kali/apt 走 IPv6 会挂 -> 所有 apt 操作强制 IPv4
#   - sqlite3 驱动需要 cgo -> 必须有 gcc
#   - Go 依赖下载慢 -> GOPROXY=https://goproxy.cn,direct
#   - 不安装系统级 Go（避免污染），缺失时给出安装指引
# =============================================================================
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

SKIP_APT=0
for arg in "$@"; do
  case "$arg" in
    --skip-apt) SKIP_APT=1 ;;
    *) echo "未知参数: $arg" >&2; exit 2 ;;
  esac
done

APT_FLAGS=(-o Acquire::ForceIPv4=true)
REQUIRED_APT=(git gcc make curl sqlite3 ca-certificates)

echo "==> [1/3] 系统依赖"
if [[ "$SKIP_APT" == "1" ]]; then
  echo "    --skip-apt，跳过 apt"
else
  if [[ "$(id -u)" != "0" ]]; then
    echo "    [警告] 需要 root 安装系统包；当前非 root，尝试 sudo"
    SUDO="sudo"
  else
    SUDO=""
  fi
  $SUDO apt-get "${APT_FLAGS[@]}" update -y
  $SUDO apt-get "${APT_FLAGS[@]}" install -y "${REQUIRED_APT[@]}"
fi

echo "==> [2/3] Go 工具链"
if command -v go >/dev/null 2>&1; then
  GO_VERSION="$(go version)"
  echo "    已安装: $GO_VERSION"
  if ! go version | grep -qE "go1\.(2[3-9]|[3-9][0-9])"; then
    echo "    [警告] 本项目 go.mod 要求 go 1.25+；低版本可能编译失败"
  fi
else
  echo "    [错误] 未找到 go。请安装 Go 1.25+："
  echo "      wget https://go.dev/dl/go1.25.0.linux-amd64.tar.gz"
  echo "      sudo tar -C /usr/local -xzf go1.25.0.linux-amd64.tar.gz"
  echo "      export PATH=\$PATH:/usr/local/go/bin"
  exit 1
fi

echo "==> [3/3] Go 环境变量（写入 ~/.profile / ~/.bashrc 幂等）"
GOPROXY_URL="https://goproxy.cn,direct"
for rc in "$HOME/.profile" "$HOME/.bashrc"; do
  if [[ -f "$rc" ]] && ! grep -q "goproxy.cn" "$rc"; then
    echo "export GOPROXY=\"$GOPROXY_URL\"" >> "$rc"
    echo "    -> $rc 已追加 GOPROXY"
  fi
done
export GOPROXY="$GOPROXY_URL"

echo "==> 依赖就绪。下一步：scripts/init-workspace.sh"
