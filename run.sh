#!/bin/bash

set -euo pipefail

# CyberStrikeAI 启动脚本
ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT_DIR"

echo "🚀 启动 CyberStrikeAI..."

CONFIG_FILE="$ROOT_DIR/config.yaml"
VENV_DIR="$ROOT_DIR/venv"
REQUIREMENTS_FILE="$ROOT_DIR/requirements.txt"

# 检查配置文件
if [ ! -f "$CONFIG_FILE" ]; then
    echo "❌ 配置文件 config.yaml 不存在"
    exit 1
fi

# 检查 Python 环境
if ! command -v python3 >/dev/null 2>&1; then
    echo "❌ 未找到 python3，请先安装 Python 3.10+"
    exit 1
fi

# 创建并激活虚拟环境
if [ ! -d "$VENV_DIR" ]; then
    echo "🐍 创建 Python 虚拟环境..."
    python3 -m venv "$VENV_DIR"
fi

echo "🐍 激活虚拟环境..."
# shellcheck disable=SC1091
source "$VENV_DIR/bin/activate"

if [ -f "$REQUIREMENTS_FILE" ]; then
    echo "📦 安装/更新 Python 依赖..."
    pip install -r "$REQUIREMENTS_FILE"
else
    echo "⚠️ 未找到 requirements.txt，跳过 Python 依赖安装"
fi

# 检查 Go 环境
if ! command -v go >/dev/null 2>&1; then
    echo "❌ Go 未安装，请先安装 Go 1.21 或更高版本"
    exit 1
fi

# 下载依赖
echo "📦 下载 Go 依赖..."
go mod download

# 构建项目
echo "🔨 构建项目..."
go build -o cyberstrike-ai cmd/server/main.go

# 运行服务器
echo "✅ 启动服务器..."
./cyberstrike-ai
