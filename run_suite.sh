#!/bin/bash
# ═══════════════════════════════════════════════════════════════
# CyberStrikeAI - Full Suite Launcher
# Checks dependencies, builds, and runs the main application
# ═══════════════════════════════════════════════════════════════
set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$DIR"

log()  { echo -e "${GREEN}[+]${NC} $1"; }
warn() { echo -e "${YELLOW}[!]${NC} $1"; }
err()  { echo -e "${RED}[x]${NC} $1"; }
info() { echo -e "${CYAN}[*]${NC} $1"; }

echo "═══════════════════════════════════════════════════════════════"
echo "  CyberStrikeAI Suite - Dependency Check, Build & Run"
echo "═══════════════════════════════════════════════════════════════"
echo ""

ERRORS=0

# ─── 1. Go compiler ──────────────────────────────────────────────
info "Checking Go..."
if command -v go &>/dev/null; then
    GO_VER=$(go version | awk '{print $3}')
    log "Go: $GO_VER"
else
    err "Go not found. Install: https://go.dev/dl/"
    if [ "$(id -u)" -eq 0 ] || [ -n "$SUDO_USER" ]; then
        warn "Attempting auto-install..."
        apt-get update -qq && apt-get install -y -qq golang-go 2>/dev/null && log "Go installed" || ERRORS=$((ERRORS+1))
    else
        ERRORS=$((ERRORS+1))
    fi
fi

# ─── 2. Python 3 ─────────────────────────────────────────────────
info "Checking Python 3..."
if command -v python3 &>/dev/null; then
    PY_VER=$(python3 --version 2>&1)
    log "Python: $PY_VER"
else
    err "Python 3 not found"
    ERRORS=$((ERRORS+1))
fi

# ─── 3. Key security tools (non-fatal) ───────────────────────────
info "Checking security tools..."
TOOLS_OK=0
TOOLS_MISSING=0
for tool in nmap subfinder nuclei nikto sqlmap ffuf gobuster hydra feroxbuster; do
    if command -v "$tool" &>/dev/null; then
        TOOLS_OK=$((TOOLS_OK+1))
    else
        TOOLS_MISSING=$((TOOLS_MISSING+1))
    fi
done
if [ $TOOLS_MISSING -gt 0 ]; then
    warn "$TOOLS_OK tools found, $TOOLS_MISSING missing (non-fatal, agent will use what's available)"
else
    log "All $TOOLS_OK core security tools present"
fi

# ─── 3b. Network capabilities (pcap/raw socket without root) ────
info "Setting network capabilities on security tools..."
CAPS_SET=0
for tool in nmap tcpdump tshark masscan rustscan arp-scan; do
    TOOL_PATH=$(command -v "$tool" 2>/dev/null)
    if [ -n "$TOOL_PATH" ]; then
        if getcap "$TOOL_PATH" 2>/dev/null | grep -q "cap_net_raw"; then
            CAPS_SET=$((CAPS_SET+1))
        else
            if [ "$(id -u)" -eq 0 ] || [ -n "$SUDO_USER" ]; then
                setcap cap_net_raw,cap_net_admin+eip "$TOOL_PATH" 2>/dev/null && CAPS_SET=$((CAPS_SET+1))
            else
                sudo setcap cap_net_raw,cap_net_admin+eip "$TOOL_PATH" 2>/dev/null && CAPS_SET=$((CAPS_SET+1))
            fi
        fi
    fi
done
if [ $CAPS_SET -gt 0 ]; then
    log "Network capabilities set on $CAPS_SET tools (pcap/raw sockets without root)"
else
    warn "Could not set network capabilities - run with sudo once, or pcap tools will fail"
fi

# ─── 3c. Tor and proxychains (for proxy routing) ────────────────
info "Checking Tor and proxychains..."
TOR_OK=0
PC_OK=0
if command -v tor &>/dev/null; then
    log "Tor: installed ($(tor --version | head -1))"
    TOR_OK=1
else
    warn "Tor not installed"
    if [ "$(id -u)" -eq 0 ] || [ -n "$SUDO_USER" ]; then
        apt-get install -y -qq tor 2>/dev/null && TOR_OK=1 && log "Tor: installed" || warn "Tor install failed"
    else
        warn "  Install with: sudo apt install tor"
    fi
fi
if command -v proxychains4 &>/dev/null; then
    log "proxychains4: installed"
    PC_OK=1
elif command -v proxychains &>/dev/null; then
    log "proxychains: installed (proxychains4 preferred)"
    PC_OK=1
else
    warn "proxychains4 not installed"
    if [ "$(id -u)" -eq 0 ] || [ -n "$SUDO_USER" ]; then
        apt-get install -y -qq proxychains4 2>/dev/null && PC_OK=1 && log "proxychains4: installed" || warn "proxychains4 install failed"
    else
        warn "  Install with: sudo apt install proxychains4"
    fi
fi

# ─── 3d. FlareSolverr (WAF/Cloudflare bypass) ───────────────────
info "Checking FlareSolverr..."
if curl -s --connect-timeout 2 http://127.0.0.1:8191/ >/dev/null 2>&1; then
    log "FlareSolverr: running on :8191"
elif command -v docker &>/dev/null; then
    if docker ps --format '{{.Names}}' 2>/dev/null | grep -q flaresolverr; then
        log "FlareSolverr: running (Docker)"
    else
        warn "FlareSolverr not running. Starting via Docker..."
        docker run -d --name flaresolverr --restart unless-stopped \
            -p 127.0.0.1:8191:8191 \
            -e LOG_LEVEL=info \
            ghcr.io/flaresolverr/flaresolverr:latest >/dev/null 2>&1 \
            && log "FlareSolverr: started on :8191" \
            || warn "FlareSolverr Docker start failed. WAF bypass tool will not work."
    fi
else
    warn "FlareSolverr not running and Docker not available"
    warn "  Install Docker and run: docker run -d --name flaresolverr -p 127.0.0.1:8191:8191 ghcr.io/flaresolverr/flaresolverr:latest"
fi

# ─── 4. Config file ──────────────────────────────────────────────
info "Checking config.yaml..."
if [ -f "$DIR/config.yaml" ]; then
    # Check API key is set
    API_KEY=$(grep "api_key:" config.yaml | head -1 | awk '{print $2}')
    if [ -z "$API_KEY" ] || [ "$API_KEY" = "sk-xxxxxx" ] || [ "$API_KEY" = "PLACEHOLDER" ]; then
        err "API key not configured in config.yaml"
        echo "    Edit config.yaml and set openai.api_key"
        ERRORS=$((ERRORS+1))
    else
        PROVIDER=$(grep "provider:" config.yaml | head -1 | awk '{print $2}')
        MODEL=$(grep "model:" config.yaml | head -1 | awk '{print $2}')
        BASE_URL=$(grep "base_url:" config.yaml | head -1 | awk '{print $2}')
        log "Config: provider=$PROVIDER model=$MODEL"

        # DNS pre-check for the API endpoint
        API_HOST=$(echo "$BASE_URL" | sed 's|https\?://||' | sed 's|/.*||' | sed 's|:.*||')
        if [ -n "$API_HOST" ]; then
            if host "$API_HOST" >/dev/null 2>&1 || dig +short "$API_HOST" >/dev/null 2>&1; then
                log "DNS: $API_HOST resolves OK"
            else
                warn "DNS: Cannot resolve $API_HOST"
                warn "API calls will fail. Fix options:"
                echo "    1. Add to /etc/hosts:  echo \"\$(dig +short $API_HOST @8.8.8.8) $API_HOST\" | sudo tee -a /etc/hosts"
                echo "    2. Fix your DNS resolver (VPN may be blocking it)"
                # Try to auto-fix via Google DNS
                RESOLVED_IP=$(dig +short "$API_HOST" @8.8.8.8 2>/dev/null | head -1)
                if [ -n "$RESOLVED_IP" ]; then
                    warn "Auto-fix: resolved $API_HOST → $RESOLVED_IP via Google DNS"
                    if grep -q "$API_HOST" /etc/hosts 2>/dev/null; then
                        log "Already in /etc/hosts"
                    else
                        echo "$RESOLVED_IP $API_HOST" | sudo tee -a /etc/hosts >/dev/null 2>&1 && \
                            log "Added $API_HOST to /etc/hosts" || \
                            warn "Could not write /etc/hosts (need sudo)"
                    fi
                fi
            fi
        fi
    fi
else
    err "config.yaml not found"
    if [ -f "$DIR/config.yaml.example" ]; then
        warn "Copying config.yaml.example -> config.yaml"
        cp config.yaml.example config.yaml
        err "Edit config.yaml and set your API key, then re-run"
    fi
    ERRORS=$((ERRORS+1))
fi

# ─── 5. Knowledge base embedding server ──────────────────────────
info "Checking vLLM embedding server (port 8102)..."
if curl -s --connect-timeout 2 http://127.0.0.1:8102/v1/models >/dev/null 2>&1; then
    log "vLLM embedding server: running on :8102"
else
    warn "vLLM embedding server not running on :8102"
    warn "Knowledge base will not work without embeddings"
    warn "Start it with: ./run_mcp.sh"
fi

# ─── 6. MCP server port ──────────────────────────────────────────
MCP_ENABLED=$(grep "^  enabled:" config.yaml | head -1 | awk '{print $2}')
MCP_PORT=$(grep "port: 8081" config.yaml | head -1 | awk '{print $2}')
if [ "$MCP_ENABLED" = "true" ]; then
    info "MCP server: enabled on :${MCP_PORT:-8081}"
else
    warn "MCP server: disabled (enable in config.yaml mcp.enabled)"
fi

# ─── 7. Disk / permissions ───────────────────────────────────────
info "Checking directories..."
for d in data tmp knowledge_base skills tools logs; do
    mkdir -p "$DIR/$d" 2>/dev/null
done
if [ -w "$DIR/data" ]; then
    log "Data directory: writable"
else
    err "Data directory not writable: $DIR/data"
    ERRORS=$((ERRORS+1))
fi

# ─── 8. Go dependencies ──────────────────────────────────────────
info "Checking Go modules..."
if [ -f go.sum ]; then
    log "go.sum present ($(wc -l < go.sum) entries)"
else
    warn "go.sum missing, will download on build"
fi

# ─── Summary ──────────────────────────────────────────────────────
echo ""
if [ $ERRORS -gt 0 ]; then
    err "$ERRORS critical issue(s) found. Fix them and re-run."
    exit 1
fi
log "All checks passed"
echo ""

# ─── 9. Build ─────────────────────────────────────────────────────
info "Building CyberStrikeAI..."
# Use Google Go proxy, not Chinese goproxy.cn
export GOPROXY="https://proxy.golang.org,direct"
if go build -o CyberStrikeAI ./cmd/server 2>&1; then
    BINARY_SIZE=$(du -h CyberStrikeAI | awk '{print $1}')
    log "Build successful: CyberStrikeAI ($BINARY_SIZE)"
else
    err "Build failed"
    exit 1
fi
echo ""

# ─── 10. Kill existing instance ──────────────────────────────────
if pgrep -f "./CyberStrikeAI" >/dev/null 2>&1; then
    warn "Stopping existing CyberStrikeAI..."
    pkill -9 -f "./CyberStrikeAI" 2>/dev/null
    sleep 2
fi

# ─── 11. Launch ───────────────────────────────────────────────────
LOG_FILE="$DIR/logs/cyberstrike_$(date +%Y%m%d_%H%M%S).log"
info "Starting CyberStrikeAI..."
nohup ./CyberStrikeAI >> "$LOG_FILE" 2>&1 &
PID=$!
sleep 3

if kill -0 "$PID" 2>/dev/null; then
    # Get password from config
    PASSWORD=$(grep "password:" config.yaml | head -1 | awk '{print $2}')
    WEB_PORT=$(grep "port: 8080" config.yaml | head -1 | awk '{print $2}')
    WEB_PORT=${WEB_PORT:-8080}

    echo ""
    echo "═══════════════════════════════════════════════════════════════"
    echo -e "  ${GREEN}CyberStrikeAI is running${NC}"
    echo ""
    echo "  PID:       $PID"
    echo "  Web UI:    http://localhost:${WEB_PORT}"
    echo "  Password:  $PASSWORD"
    echo "  Log:       $LOG_FILE"
    echo ""
    echo "  Provider:  $PROVIDER"
    echo "  Model:     $MODEL"
    echo ""
    echo "  Stop:      kill $PID"
    echo "  Logs:      tail -f $LOG_FILE"
    echo "═══════════════════════════════════════════════════════════════"
else
    err "Failed to start. Check log: $LOG_FILE"
    tail -10 "$LOG_FILE"
    exit 1
fi
