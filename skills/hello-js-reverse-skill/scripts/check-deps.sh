#!/bin/bash
# 检查 JS 逆向项目所需的依赖环境（Node.js + Python）

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "=========================================="
echo "  JS 逆向工程环境检查（Node.js + Python）"
echo "=========================================="
echo ""

check_command() {
    local cmd=$1
    local name=$2
    local install_hint=$3
    
    if command -v "$cmd" &> /dev/null; then
        local version=$($cmd --version 2>&1 | head -1)
        echo -e "  ${GREEN}✓${NC} $name: $version"
        return 0
    else
        echo -e "  ${RED}✗${NC} $name: 未安装"
        if [ -n "$install_hint" ]; then
            echo -e "    ${YELLOW}→ 安装: $install_hint${NC}"
        fi
        return 1
    fi
}

check_node_module() {
    local module=$1
    local name=$2
    
    if node -e "require('$module')" 2>/dev/null; then
        local version=$(node -e "console.log(require('$module/package.json').version)" 2>/dev/null || echo "unknown")
        echo -e "  ${GREEN}✓${NC} $name: v$version"
        return 0
    else
        echo -e "  ${YELLOW}○${NC} $name: 未安装（可选）"
        return 1
    fi
}

check_python_module() {
    local module=$1
    local import_name=${2:-$1}
    local name=${3:-$1}
    
    if python3 -c "import $import_name" 2>/dev/null; then
        local version=$(python3 -c "import $import_name; print(getattr($import_name, '__version__', 'installed'))" 2>/dev/null || echo "installed")
        echo -e "  ${GREEN}✓${NC} $name: $version"
        return 0
    else
        echo -e "  ${YELLOW}○${NC} $name: 未安装（可选）"
        return 1
    fi
}

errors=0

echo "▸ Node.js 环境"
check_command "node" "Node.js" "https://nodejs.org/ 或 brew install node" || ((errors++))
check_command "npm" "npm" "随 Node.js 一起安装" || ((errors++))
echo ""

echo "▸ Python 环境"
check_command "python3" "Python 3" "https://python.org/ 或 brew install python3"
check_command "pip3" "pip3" "随 Python 3 一起安装"
echo ""

echo "▸ 浏览器"
if [ -f "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" ]; then
    chrome_version=$("/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" --version 2>/dev/null)
    echo -e "  ${GREEN}✓${NC} Google Chrome: $chrome_version"
elif command -v google-chrome &> /dev/null; then
    chrome_version=$(google-chrome --version 2>/dev/null)
    echo -e "  ${GREEN}✓${NC} Google Chrome: $chrome_version"
else
    echo -e "  ${YELLOW}○${NC} Google Chrome: 未检测到（浏览器自动化模式需要）"
fi
echo ""

echo "▸ 全局 npm 包"
check_node_module "crypto-js" "crypto-js"
check_node_module "axios" "axios"
check_node_module "node-forge" "node-forge"
check_node_module "jsdom" "jsdom"
echo ""

echo "▸ Python 包"
check_python_module "requests" "requests" "requests"
check_python_module "pycryptodome" "Crypto" "pycryptodome"
check_python_module "execjs" "execjs" "PyExecJS"
check_python_module "curl_cffi" "curl_cffi" "curl_cffi"
check_python_module "httpx" "httpx" "httpx"
echo ""

echo "▸ 可选工具"
check_command "curl" "curl" "系统自带"
check_command "jq" "jq" "brew install jq"
echo ""

echo "=========================================="
if [ $errors -gt 0 ]; then
    echo -e "  ${RED}检查完成: $errors 个必需依赖缺失${NC}"
    echo ""
    echo "  快速安装缺失依赖："
    echo "    Node.js: npm install -g crypto-js axios node-forge"
    echo "    Python:  pip3 install requests pycryptodome execjs"
    exit 1
else
    echo -e "  ${GREEN}检查完成: 环境就绪${NC}"
    exit 0
fi
