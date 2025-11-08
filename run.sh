#!/bin/bash

# CyberStrikeAI 启动脚本

echo "🚀 启动 CyberStrikeAI..."

# 检查配置文件
if [ ! -f "config.yaml" ]; then
    echo "❌ 配置文件 config.yaml 不存在"
    exit 1
fi

# 检查Go环境
if ! command -v go &> /dev/null; then
    echo "❌ Go 未安装，请先安装 Go 1.21 或更高版本"
    exit 1
fi

# 下载依赖
echo "📦 下载依赖..."
go mod download

# 构建项目
echo "🔨 构建项目..."
go build -o cyberstrike-ai cmd/server/main.go

if [ $? -ne 0 ]; then
    echo "❌ 构建失败"
    exit 1
fi

# 运行服务器
echo "✅ 启动服务器..."
./cyberstrike-ai

