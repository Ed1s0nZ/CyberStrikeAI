#!/usr/bin/env bash
# ============================================================================
# CyberStrikeAI - Ghidra Headless MCP Server Launcher
# ============================================================================
# Starts ghidra-headless-mcp as a standalone MCP server for binary analysis.
# The server exposes ~212 tools covering the full Ghidra API: decompilation,
# disassembly, xrefs, type recovery, patching, scripting, and more.
#
# Auto-installs all dependencies (Ghidra, pyghidra, ghidra-headless-mcp) if
# they are not present. Just run and it handles the rest.
#
# Usage:
#   ./start-ghidra-mcp.sh                    # stdio mode (for CyberStrikeAI auto-start)
#   ./start-ghidra-mcp.sh --tcp              # TCP mode on port 8765 (manual start)
#   ./start-ghidra-mcp.sh --tcp --port 9000  # TCP mode on custom port
#
# Environment variables:
#   GHIDRA_INSTALL_DIR  - path to Ghidra installation (auto-detected/installed if missing)
#   GHIDRA_MCP_PORT     - TCP port (default 8765, only for --tcp mode)
#   GHIDRA_MCP_HOME     - path to ghidra-headless-mcp repo (default ~/ghidra-headless-mcp)
#   GHIDRA_VERSION      - Ghidra version to install if missing (default 11.3.2)
# ============================================================================
set -euo pipefail

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
info()  { echo -e "${GREEN}[+]${NC} $*" >&2; }
warn()  { echo -e "${YELLOW}[!]${NC} $*" >&2; }
error() { echo -e "${RED}[-]${NC} $*" >&2; exit 1; }

# ── Configuration ─────────────────────────────────────────────────────────────
GHIDRA_MCP_HOME="${GHIDRA_MCP_HOME:-$HOME/ghidra-headless-mcp}"
GHIDRA_MCP_PORT="${GHIDRA_MCP_PORT:-8765}"
GHIDRA_VERSION="${GHIDRA_VERSION:-11.3.2}"
MODE="stdio"

# ── Parse arguments ───────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
    case "$1" in
        --tcp)        MODE="tcp"; shift ;;
        --stdio)      MODE="stdio"; shift ;;
        --port)       GHIDRA_MCP_PORT="$2"; shift 2 ;;
        --ghidra-dir) export GHIDRA_INSTALL_DIR="$2"; shift 2 ;;
        --help|-h)
            echo "Usage: $0 [--tcp] [--stdio] [--port PORT] [--ghidra-dir DIR]"
            echo ""
            echo "Modes:"
            echo "  --stdio    Run as stdio MCP server (default; for CyberStrikeAI auto-start)"
            echo "  --tcp      Run as TCP MCP server (for manual start; connect via port)"
            echo ""
            echo "Options:"
            echo "  --port PORT         TCP port (default: 8765, only for --tcp)"
            echo "  --ghidra-dir DIR    Override GHIDRA_INSTALL_DIR"
            echo ""
            echo "All dependencies are auto-installed if missing (Ghidra, JDK, pyghidra, ghidra-headless-mcp)."
            exit 0
            ;;
        *) error "Unknown argument: $1" ;;
    esac
done

# ── Step 1: Ensure JDK is installed ──────────────────────────────────────────
ensure_jdk() {
    if java -version &>/dev/null; then
        return 0
    fi
    info "JDK not found. Installing OpenJDK 21..."
    if command -v apt-get &>/dev/null; then
        sudo apt-get update -qq && sudo apt-get install -y -qq openjdk-21-jdk 2>/dev/null || \
        sudo apt-get install -y -qq openjdk-17-jdk 2>/dev/null || true
    elif command -v dnf &>/dev/null; then
        sudo dnf install -y java-21-openjdk-devel 2>/dev/null || \
        sudo dnf install -y java-17-openjdk-devel 2>/dev/null || true
    elif command -v pacman &>/dev/null; then
        sudo pacman -S --noconfirm jdk-openjdk 2>/dev/null || true
    fi
    java -version &>/dev/null || error "Failed to install JDK. Install Java 17+ manually."
    info "JDK installed."
}

# ── Step 2: Ensure Ghidra is installed ────────────────────────────────────────
ensure_ghidra() {
    # Auto-detect existing installation
    if [[ -z "${GHIDRA_INSTALL_DIR:-}" ]]; then
        for candidate in \
            /opt/ghidra \
            /opt/ghidra_* \
            "$HOME/ghidra" \
            "$HOME/ghidra_${GHIDRA_VERSION}_PUBLIC" \
            /usr/share/ghidra \
            /usr/local/share/ghidra; do
            if [[ -d "$candidate" ]] && find "$candidate" -name "analyzeHeadless" -type f 2>/dev/null | head -1 | grep -q .; then
                export GHIDRA_INSTALL_DIR="$candidate"
                break
            fi
        done
    fi

    if [[ -n "${GHIDRA_INSTALL_DIR:-}" && -d "$GHIDRA_INSTALL_DIR" ]]; then
        info "Ghidra found: $GHIDRA_INSTALL_DIR"
        return 0
    fi

    info "Ghidra not found. Installing Ghidra ${GHIDRA_VERSION}..."

    # Determine download URL
    # Ghidra releases: https://github.com/NationalSecurityAgency/ghidra/releases
    local GHIDRA_DATE="20250221"  # Matches 11.3.2
    local GHIDRA_ZIP="ghidra_${GHIDRA_VERSION}_PUBLIC_${GHIDRA_DATE}.zip"
    local GHIDRA_URL="https://github.com/NationalSecurityAgency/ghidra/releases/download/Ghidra_${GHIDRA_VERSION}_build/${GHIDRA_ZIP}"

    local INSTALL_DIR="/opt/ghidra_${GHIDRA_VERSION}_PUBLIC"
    local TMP_ZIP="/tmp/${GHIDRA_ZIP}"

    info "Downloading from GitHub releases..."
    if ! curl -fSL -o "$TMP_ZIP" "$GHIDRA_URL" 2>&1 | tail -3 >&2; then
        # Try alternative URL format
        GHIDRA_URL="https://github.com/NationalSecurityAgency/ghidra/releases/latest/download/${GHIDRA_ZIP}"
        curl -fSL -o "$TMP_ZIP" "$GHIDRA_URL" 2>&1 | tail -3 >&2 || {
            warn "Auto-download failed. Please download Ghidra manually:"
            warn "  https://ghidra-sre.org/"
            warn "  Extract to /opt/ghidra and set GHIDRA_INSTALL_DIR"
            error "Ghidra installation required."
        }
    fi

    info "Extracting to /opt/..."
    sudo unzip -qo "$TMP_ZIP" -d /opt/ 2>/dev/null || unzip -qo "$TMP_ZIP" -d "$HOME/" 2>/dev/null
    rm -f "$TMP_ZIP"

    # Find the extracted directory
    if [[ -d "$INSTALL_DIR" ]]; then
        export GHIDRA_INSTALL_DIR="$INSTALL_DIR"
    else
        # Try to find it
        local FOUND
        FOUND=$(find /opt "$HOME" -maxdepth 1 -name "ghidra_${GHIDRA_VERSION}*" -type d 2>/dev/null | head -1)
        if [[ -n "$FOUND" ]]; then
            export GHIDRA_INSTALL_DIR="$FOUND"
        else
            error "Ghidra extracted but install directory not found. Set GHIDRA_INSTALL_DIR manually."
        fi
    fi

    info "Ghidra installed: $GHIDRA_INSTALL_DIR"
}

# ── Step 3: Ensure ghidra-headless-mcp is installed ──────────────────────────
ensure_ghidra_mcp() {
    # Clone if missing
    if [[ ! -d "$GHIDRA_MCP_HOME" ]]; then
        info "Cloning ghidra-headless-mcp..."
        git clone https://github.com/mrphrazer/ghidra-headless-mcp.git "$GHIDRA_MCP_HOME"
    fi

    # Check if Python module is importable
    if python3 -c "import ghidra_headless_mcp" 2>/dev/null; then
        return 0
    fi

    info "Installing ghidra-headless-mcp Python package..."
    cd "$GHIDRA_MCP_HOME"

    # Ensure pip is available
    python3 -m pip --version &>/dev/null || {
        info "Installing pip..."
        if command -v apt-get &>/dev/null; then
            sudo apt-get install -y -qq python3-pip 2>/dev/null || true
        fi
        python3 -m ensurepip --upgrade 2>/dev/null || true
    }

    # Install pyghidra first (Ghidra Python bridge, requires JDK)
    info "Installing pyghidra (Ghidra Python bridge)..."
    pip install pyghidra 2>&1 | tail -5 >&2 || pip install --user pyghidra 2>&1 | tail -5 >&2

    # Install ghidra-headless-mcp
    info "Installing ghidra-headless-mcp..."
    pip install -e . 2>&1 | tail -5 >&2 || pip install --user -e . 2>&1 | tail -5 >&2

    # Verify
    python3 -c "import ghidra_headless_mcp" 2>/dev/null || {
        error "Failed to install ghidra_headless_mcp. Check Python 3.11+ and JDK 17+ are available."
    }

    info "ghidra-headless-mcp installed."
}

# ── Step 4: Ensure Python 3.11+ ──────────────────────────────────────────────
ensure_python() {
    local PY_VER
    PY_VER=$(python3 -c "import sys; print(f'{sys.version_info.major}.{sys.version_info.minor}')" 2>/dev/null || echo "0.0")
    local PY_MAJOR PY_MINOR
    PY_MAJOR=$(echo "$PY_VER" | cut -d. -f1)
    PY_MINOR=$(echo "$PY_VER" | cut -d. -f2)

    if [[ "$PY_MAJOR" -ge 3 && "$PY_MINOR" -ge 11 ]]; then
        return 0
    fi

    warn "Python $PY_VER detected, but 3.11+ is required."
    info "Attempting to install Python 3.11+..."
    if command -v apt-get &>/dev/null; then
        sudo apt-get update -qq
        sudo apt-get install -y -qq python3.12 python3.12-venv 2>/dev/null || \
        sudo apt-get install -y -qq python3.11 python3.11-venv 2>/dev/null || true
    fi

    # Re-check
    PY_VER=$(python3 -c "import sys; print(f'{sys.version_info.major}.{sys.version_info.minor}')" 2>/dev/null || echo "0.0")
    PY_MAJOR=$(echo "$PY_VER" | cut -d. -f1)
    PY_MINOR=$(echo "$PY_VER" | cut -d. -f2)
    if [[ "$PY_MAJOR" -lt 3 || "$PY_MINOR" -lt 11 ]]; then
        error "Python 3.11+ required. Current: $PY_VER. Install Python 3.11+ manually."
    fi
}

# ── Run dependency checks ─────────────────────────────────────────────────────
info "Checking dependencies..."
ensure_python
ensure_jdk
ensure_ghidra
ensure_ghidra_mcp

info "All dependencies ready."
info "Ghidra:             $GHIDRA_INSTALL_DIR"
info "ghidra-headless-mcp: $GHIDRA_MCP_HOME"

# ── Launch server ─────────────────────────────────────────────────────────────
if [[ "$MODE" == "tcp" ]]; then
    info "Starting Ghidra Headless MCP (TCP mode on port $GHIDRA_MCP_PORT)..."
    info "Connect via: tcp://127.0.0.1:$GHIDRA_MCP_PORT"
    info "Press Ctrl+C to stop."
    exec python3 -m ghidra_headless_mcp \
        --transport tcp \
        --host 127.0.0.1 \
        --port "$GHIDRA_MCP_PORT" \
        --ghidra-install-dir "$GHIDRA_INSTALL_DIR"
else
    # Stdio mode - ghidra-headless-mcp reads JSON-RPC from stdin, writes to stdout
    # CyberStrikeAI will spawn this as a subprocess
    exec python3 -m ghidra_headless_mcp \
        --transport stdio \
        --ghidra-install-dir "$GHIDRA_INSTALL_DIR"
fi
