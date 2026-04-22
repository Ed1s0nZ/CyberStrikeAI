#!/usr/bin/env bash
# ============================================================================
# CyberStrikeAI - Frida Kahlo MCP Server Launcher
# ============================================================================
# Starts kahlo-mcp (https://github.com/FuzzySecurity/kahlo-mcp) as a stdio MCP
# server for Android dynamic instrumentation via Frida. The server exposes
# tools for device/process discovery, attach/spawn lifecycle, Frida script
# injection with per-script isolation, telemetry streaming, and artifact
# storage. See https://knifecoat.com/Posts/Scalable+research+tooling+for+agent+systems
# for the background on why the API is shaped the way it is.
#
# Auto-installs everything required: Node.js, the kahlo-mcp repo, npm deps,
# and a default config.json with adbPath auto-detected. `adb` itself is NOT
# auto-installed on Linux distros where it's a separate package — we'll warn
# if it's missing and tell you where to get it (platform-specific USB/udev
# permissions make silent install a bad idea).
#
# Usage:
#   ./start-frida-kahlo-mcp.sh              # stdio mode (for CyberStrikeAI auto-start)
#   ./start-frida-kahlo-mcp.sh --help       # show options
#
# Environment variables:
#   KAHLO_MCP_HOME     - path to kahlo-mcp repo (default ~/kahlo-mcp)
#   KAHLO_MCP_REPO     - git url to clone from (default upstream FuzzySecurity)
#   KAHLO_ADB_PATH     - full path to adb (default: auto-detect via `which adb`)
#   KAHLO_DATA_DIR     - kahlo-mcp data dir (default: kahlo-mcp/data)
#   KAHLO_LOG_LEVEL    - kahlo-mcp log level (default: info)
#
# Prerequisites you must provide yourself:
#   - adb (Android Debug Bridge). On Debian/Ubuntu: `sudo apt-get install android-tools-adb`
#   - A rooted Android device or emulator with frida-server running on it
#     (version must match the `frida` package pinned in kahlo-mcp/package.json)
# ============================================================================
set -euo pipefail

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
info()  { echo -e "${GREEN}[+]${NC} $*" >&2; }
warn()  { echo -e "${YELLOW}[!]${NC} $*" >&2; }
error() { echo -e "${RED}[-]${NC} $*" >&2; exit 1; }

# ── Configuration ─────────────────────────────────────────────────────────────
KAHLO_MCP_HOME="${KAHLO_MCP_HOME:-$HOME/kahlo-mcp}"
KAHLO_MCP_REPO="${KAHLO_MCP_REPO:-https://github.com/FuzzySecurity/kahlo-mcp.git}"
KAHLO_ADB_PATH="${KAHLO_ADB_PATH:-}"
KAHLO_DATA_DIR="${KAHLO_DATA_DIR:-}"
KAHLO_LOG_LEVEL="${KAHLO_LOG_LEVEL:-info}"

# ── Parse arguments ───────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
    case "$1" in
        --adb-path)     KAHLO_ADB_PATH="$2"; shift 2 ;;
        --kahlo-home)   KAHLO_MCP_HOME="$2"; shift 2 ;;
        --install)      INSTALL_ONLY=1; shift ;;
        --help|-h)
            echo "Usage: $0 [--adb-path PATH] [--kahlo-home DIR] [--install]"
            echo ""
            echo "Launches kahlo-mcp (Frida MCP server) in stdio mode. CyberStrikeAI"
            echo "spawns this as a subprocess when the frida-kahlo entry in external_mcp"
            echo "is enabled."
            echo ""
            echo "Options:"
            echo "  --adb-path PATH   Full path to adb binary (default: auto-detect)"
            echo "  --kahlo-home DIR  Override KAHLO_MCP_HOME (default ~/kahlo-mcp)"
            echo "  --install         Install and exit without starting the server"
            echo ""
            echo "kahlo-mcp only supports stdio transport today (SSE on upstream roadmap)."
            exit 0
            ;;
        *) error "Unknown argument: $1 (try --help)" ;;
    esac
done

# ── Step 1: Ensure Node.js is installed (18+) ─────────────────────────────────
ensure_node() {
    if command -v node &>/dev/null; then
        local NODE_MAJOR
        NODE_MAJOR=$(node -e 'console.log(process.versions.node.split(".")[0])' 2>/dev/null || echo 0)
        if [[ "$NODE_MAJOR" -ge 18 ]]; then
            return 0
        fi
        warn "Node $(node -v) detected, but 18+ is required."
    else
        info "Node.js not found. Installing via package manager..."
    fi

    if command -v apt-get &>/dev/null; then
        # Debian/Ubuntu ship older Node in default repos; pull NodeSource if we can.
        if ! command -v curl &>/dev/null; then
            sudo apt-get update -qq && sudo apt-get install -y -qq curl
        fi
        curl -fsSL https://deb.nodesource.com/setup_20.x 2>/dev/null | sudo -E bash - >/dev/null 2>&1 \
            && sudo apt-get install -y -qq nodejs \
            || sudo apt-get install -y -qq nodejs npm 2>/dev/null || true
    elif command -v dnf &>/dev/null; then
        sudo dnf install -y nodejs npm 2>/dev/null || true
    elif command -v pacman &>/dev/null; then
        sudo pacman -S --noconfirm nodejs npm 2>/dev/null || true
    elif command -v brew &>/dev/null; then
        brew install node 2>/dev/null || true
    fi

    command -v node &>/dev/null || error "Failed to install Node.js. Install manually (>=18) and re-run."

    # Re-check version
    local NODE_MAJOR
    NODE_MAJOR=$(node -e 'console.log(process.versions.node.split(".")[0])')
    if [[ "$NODE_MAJOR" -lt 18 ]]; then
        error "Node $(node -v) installed, but 18+ is required. Upgrade manually."
    fi
    info "Node.js: $(node -v)"
}

# ── Step 2: Check for adb (don't auto-install; see header comment) ────────────
ensure_adb() {
    if [[ -n "$KAHLO_ADB_PATH" ]]; then
        [[ -x "$KAHLO_ADB_PATH" ]] || error "KAHLO_ADB_PATH not executable: $KAHLO_ADB_PATH"
        info "adb: $KAHLO_ADB_PATH (from env)"
        return 0
    fi
    local DETECTED
    DETECTED=$(command -v adb 2>/dev/null || true)
    if [[ -z "$DETECTED" ]]; then
        warn "adb not found on PATH. kahlo-mcp will fail to talk to any device."
        warn "Install it (Debian/Ubuntu: 'sudo apt-get install android-tools-adb', "
        warn "macOS: 'brew install android-platform-tools') and re-run, or pass --adb-path."
        # Use a harmless placeholder so kahlo-mcp can still boot to list its tools.
        KAHLO_ADB_PATH="/usr/bin/adb"
    else
        KAHLO_ADB_PATH="$DETECTED"
        info "adb: $KAHLO_ADB_PATH (auto-detected)"
    fi
}

# ── Step 3: Clone + build kahlo-mcp if missing ────────────────────────────────
ensure_kahlo_mcp() {
    if [[ -d "$KAHLO_MCP_HOME/dist" && -f "$KAHLO_MCP_HOME/dist/index.js" ]]; then
        info "kahlo-mcp: $KAHLO_MCP_HOME (already built)"
        return 0
    fi

    if [[ ! -d "$KAHLO_MCP_HOME/.git" ]]; then
        info "Cloning kahlo-mcp from $KAHLO_MCP_REPO → $KAHLO_MCP_HOME..."
        git clone --depth 1 "$KAHLO_MCP_REPO" "$KAHLO_MCP_HOME" >&2 \
            || error "git clone failed. Check network / the KAHLO_MCP_REPO URL."
    fi

    # The upstream repo has `kahlo-mcp` as a subdir of the clone — normalise both layouts.
    if [[ -d "$KAHLO_MCP_HOME/kahlo-mcp" && -f "$KAHLO_MCP_HOME/kahlo-mcp/package.json" ]]; then
        KAHLO_MCP_HOME="$KAHLO_MCP_HOME/kahlo-mcp"
        info "Using nested kahlo-mcp subdir: $KAHLO_MCP_HOME"
    fi

    [[ -f "$KAHLO_MCP_HOME/package.json" ]] \
        || error "kahlo-mcp package.json not found under $KAHLO_MCP_HOME — unexpected repo layout."

    info "Installing npm dependencies in $KAHLO_MCP_HOME..."
    (cd "$KAHLO_MCP_HOME" && npm install --no-audit --no-fund >&2) \
        || error "npm install failed."

    info "Building kahlo-mcp..."
    (cd "$KAHLO_MCP_HOME" && npm run build >&2) \
        || error "npm run build failed."
}

# ── Step 4: Write config.json if missing ──────────────────────────────────────
ensure_config() {
    local CFG="$KAHLO_MCP_HOME/config.json"
    if [[ -f "$CFG" ]]; then
        info "config.json: $CFG (reusing existing)"
        return 0
    fi
    local DATA_DIR="${KAHLO_DATA_DIR:-$KAHLO_MCP_HOME/data}"
    mkdir -p "$DATA_DIR"
    cat > "$CFG" <<EOF
{
  "transport": "stdio",
  "logLevel": "$KAHLO_LOG_LEVEL",
  "dataDir": "$DATA_DIR",
  "adbPath": "$KAHLO_ADB_PATH"
}
EOF
    info "Wrote default config.json (adbPath=$KAHLO_ADB_PATH, dataDir=$DATA_DIR)."
}

# ── Run dependency checks ─────────────────────────────────────────────────────
info "Checking dependencies..."
ensure_node
ensure_adb
ensure_kahlo_mcp
ensure_config

if [[ "${INSTALL_ONLY:-0}" == "1" ]]; then
    info "Install-only flag set — not starting the server."
    info "Next steps: enable 'frida-kahlo' in config.yaml external_mcp.servers, then start CyberStrikeAI."
    exit 0
fi

info "Launching kahlo-mcp (stdio) from $KAHLO_MCP_HOME..."
cd "$KAHLO_MCP_HOME"
exec node dist/index.js
