#!/bin/bash

set -euo pipefail

# CyberStrikeAI one-click deploy and start script
ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT_DIR"

# Color definitions
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Print colored messages
info() { echo -e "${BLUE}ℹ️  $1${NC}"; }
success() { echo -e "${GREEN}✅ $1${NC}"; }
warning() { echo -e "${YELLOW}⚠️  $1${NC}"; }
error() { echo -e "${RED}❌ $1${NC}"; }
note() { echo -e "${CYAN}ℹ️  $1${NC}"; }

# Temporary mirror/proxy settings (only effective in this script)
PIP_INDEX_URL="${PIP_INDEX_URL:-https://pypi.tuna.tsinghua.edu.cn/simple}"
GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"

# Save original env vars (for restoration)
ORIGINAL_PIP_INDEX_URL="${PIP_INDEX_URL:-}"
ORIGINAL_GOPROXY="${GOPROXY:-}"

# Progress display helper
show_progress() {
    local pid=$1
    local message=$2
    local i=0
    local dots=""
    
    # Check if the process exists
    if ! kill -0 "$pid" 2>/dev/null; then
        # Process already finished; return immediately
        return 0
    fi
    
    while kill -0 "$pid" 2>/dev/null; do
        i=$((i + 1))
        case $((i % 4)) in
            0) dots="." ;;
            1) dots=".." ;;
            2) dots="..." ;;
            3) dots="...." ;;
        esac
        printf "\r${BLUE}⏳ %s%s${NC}" "$message" "$dots"
        sleep 0.5
        
        # Re-check whether the process is still running
        if ! kill -0 "$pid" 2>/dev/null; then
            break
        fi
    done
    printf "\r"
}

print_start_banner() {
    echo ""
    echo "=========================================="
    echo "  CyberStrikeAI Deploy & Start Script"
    echo "  (background HTTPS by default; plain HTTP: $0 --http; foreground: $0 --foreground)"
    echo "=========================================="
    echo ""

    # Show temporary mirror/proxy info
    echo ""
    warning "Note: this script uses temporary mirrors to speed up downloads"
    echo ""
    info "Python pip temporary mirror:"
    echo "  ${PIP_INDEX_URL}"
    info "Go temporary proxy:"
    echo "  ${GOPROXY}"
    echo ""
    note "These settings apply only while this script runs and do not change system config"
    echo ""
    sleep 1
}

CONFIG_FILE="$ROOT_DIR/config.yaml"
VENV_DIR="$ROOT_DIR/venv"
REQUIREMENTS_FILE="$ROOT_DIR/requirements.txt"
BINARY_NAME="cyberstrike-ai"
LOG_DIR="$ROOT_DIR/logs"
RUN_DIR="$ROOT_DIR/run"
PID_FILE="$RUN_DIR/$BINARY_NAME.pid"
SERVER_LOG="$LOG_DIR/$BINARY_NAME.log"

check_config_file() {
    if [ ! -f "$CONFIG_FILE" ]; then
        error "Config file config.yaml not found"
        info "Make sure you run this script from the project root"
        exit 1
    fi
}

# Check Python environment
check_python() {
    if ! command -v python3 >/dev/null 2>&1; then
        error "python3 not found"
        echo ""
        info "Install Python 3.10 or later first:"
        echo "  macOS:   brew install python3"
        echo "  Ubuntu:  sudo apt-get install python3 python3-venv"
        echo "  CentOS:  sudo yum install python3 python3-pip"
        exit 1
    fi
    
    PYTHON_VERSION=$(python3 --version 2>&1 | awk '{print $2}')
    PYTHON_MAJOR=$(echo "$PYTHON_VERSION" | cut -d. -f1)
    PYTHON_MINOR=$(echo "$PYTHON_VERSION" | cut -d. -f2)
    
    if [ "$PYTHON_MAJOR" -lt 3 ] || ([ "$PYTHON_MAJOR" -eq 3 ] && [ "$PYTHON_MINOR" -lt 10 ]); then
        error "Python version too old: $PYTHON_VERSION (requires 3.10+)"
        exit 1
    fi
    
    success "Python check passed: $PYTHON_VERSION"
}

# Check Go environment
check_go() {
    if ! command -v go >/dev/null 2>&1; then
        error "Go not found"
        echo ""
        info "Install Go 1.21 or later first:"
        echo "  macOS:   brew install go"
        echo "  Ubuntu:  sudo apt-get install golang-go"
        echo "  CentOS:  sudo yum install golang"
        echo "  Or visit: https://go.dev/dl/"
        exit 1
    fi
    
    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    GO_MAJOR=$(echo "$GO_VERSION" | cut -d. -f1)
    GO_MINOR=$(echo "$GO_VERSION" | cut -d. -f2)
    
    if [ "$GO_MAJOR" -lt 1 ] || ([ "$GO_MAJOR" -eq 1 ] && [ "$GO_MINOR" -lt 21 ]); then
        error "Go version too old: $GO_VERSION (requires 1.21+)"
        exit 1
    fi
    
    success "Go check passed: $(go version)"
}

# Set up Python virtual environment
setup_python_env() {
    if [ ! -d "$VENV_DIR" ]; then
        info "Creating Python virtual environment..."
        python3 -m venv "$VENV_DIR"
        success "Virtual environment created"
    else
        info "Python virtual environment already exists"
    fi
    
    info "Activating virtual environment..."
    # shellcheck disable=SC1091
    source "$VENV_DIR/bin/activate"
    
    if [ -f "$REQUIREMENTS_FILE" ]; then
        echo ""
        note "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        note "Using temporary pip mirror (this script run only)"
        note "   Mirror URL: ${PIP_INDEX_URL}"
        note "   For a permanent setting, set the PIP_INDEX_URL env var"
        note "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        echo ""
        
        info "Upgrading pip..."
        pip install --index-url "$PIP_INDEX_URL" --upgrade pip >/dev/null 2>&1 || true
        
        info "Installing Python dependencies..."
        echo ""
        
        # Install deps in background; capture errors and show progress
        PIP_LOG=$(mktemp)
        (
            set +e  # disable errexit in subshell
            pip install --index-url "$PIP_INDEX_URL" -r "$REQUIREMENTS_FILE" >"$PIP_LOG" 2>&1
            echo $? > "${PIP_LOG}.exit"
        ) &
        PIP_PID=$!
        
        # Brief pause so the process can start
        sleep 0.1
        
        # Show progress while still running
        if kill -0 "$PIP_PID" 2>/dev/null; then
            show_progress "$PIP_PID" "Installing dependencies"
        else
            # Process already finished; wait for exit code file
            sleep 0.2
        fi
        
        # Wait for completion; ignore wait exit code
        wait "$PIP_PID" 2>/dev/null || true
        
        PIP_EXIT_CODE=0
        if [ -f "${PIP_LOG}.exit" ]; then
            PIP_EXIT_CODE=$(cat "${PIP_LOG}.exit" 2>/dev/null || echo "1")
            rm -f "${PIP_LOG}.exit" 2>/dev/null || true
        else
            # No exit code file; check log for errors
            if [ -f "$PIP_LOG" ] && grep -q -i "error\|failed\|exception" "$PIP_LOG" 2>/dev/null; then
                PIP_EXIT_CODE=1
            fi
        fi
        
        if [ $PIP_EXIT_CODE -eq 0 ]; then
            success "Python dependencies installed"
        else
            # Check for angr install failure (needs Rust)
            if grep -q "angr" "$PIP_LOG" && grep -q "Rust compiler\|can't find Rust" "$PIP_LOG"; then
                warning "angr install failed (Rust compiler required)"
                echo ""
                info "angr is optional and mainly used for binary analysis tools"
                info "To use angr, install Rust first:"
                echo "  macOS:   curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh"
                echo "  Ubuntu:  curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh"
                echo "  Or visit: https://rustup.rs/"
                echo ""
                info "Other dependencies are installed; you can continue (some tools may be unavailable)"
            else
                warning "Some Python dependencies failed to install, but continuing"
                warning "If you hit issues, check the errors and install missing packages manually"
                # Show last lines of error output
                echo ""
                info "Error details (last 10 lines):"
                tail -n 10 "$PIP_LOG" | sed 's/^/  /'
                echo ""
            fi
        fi
        rm -f "$PIP_LOG"
    else
        warning "requirements.txt not found; skipping Python dependency install"
    fi
}

# Build Go project
build_go_project() {
    echo ""
    note "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    note "Using temporary Go proxy (this script run only)"
    note "   Proxy URL: ${GOPROXY}"
    note "   For a permanent setting, set the GOPROXY env var"
    note "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    
    info "Downloading Go dependencies..."
    GO_DOWNLOAD_LOG=$(mktemp)
    (
        set +e  # disable errexit in subshell
        export GOPROXY="$GOPROXY"
        go mod download >"$GO_DOWNLOAD_LOG" 2>&1
        echo $? > "${GO_DOWNLOAD_LOG}.exit"
    ) &
    GO_DOWNLOAD_PID=$!
    
    # Brief pause so the process can start
    sleep 0.1
    
    # Show progress while still running
    if kill -0 "$GO_DOWNLOAD_PID" 2>/dev/null; then
        show_progress "$GO_DOWNLOAD_PID" "Downloading Go dependencies"
    else
        # Process already finished; wait for exit code file
        sleep 0.2
    fi
    
    # Wait for completion; ignore wait exit code
    wait "$GO_DOWNLOAD_PID" 2>/dev/null || true
    
    GO_DOWNLOAD_EXIT_CODE=0
    if [ -f "${GO_DOWNLOAD_LOG}.exit" ]; then
        GO_DOWNLOAD_EXIT_CODE=$(cat "${GO_DOWNLOAD_LOG}.exit" 2>/dev/null || echo "1")
        rm -f "${GO_DOWNLOAD_LOG}.exit" 2>/dev/null || true
    else
        # No exit code file; check log for errors
        if [ -f "$GO_DOWNLOAD_LOG" ] && grep -q -i "error\|failed" "$GO_DOWNLOAD_LOG" 2>/dev/null; then
            GO_DOWNLOAD_EXIT_CODE=1
        fi
    fi
    rm -f "$GO_DOWNLOAD_LOG" 2>/dev/null || true
    
    if [ $GO_DOWNLOAD_EXIT_CODE -ne 0 ]; then
        error "Go dependency download failed"
        exit 1
    fi
    success "Go dependencies downloaded"
    
    info "Building project..."
    GO_BUILD_LOG=$(mktemp)
    (
        set +e  # disable errexit in subshell
        export GOPROXY="$GOPROXY"
        go build -o "$BINARY_NAME" cmd/server/main.go >"$GO_BUILD_LOG" 2>&1
        echo $? > "${GO_BUILD_LOG}.exit"
    ) &
    GO_BUILD_PID=$!
    
    # Brief pause so the process can start
    sleep 0.1
    
    # Show progress while still running
    if kill -0 "$GO_BUILD_PID" 2>/dev/null; then
        show_progress "$GO_BUILD_PID" "Building project"
    else
        # Process already finished; wait for exit code file
        sleep 0.2
    fi
    
    # Wait for completion; ignore wait exit code
    wait "$GO_BUILD_PID" 2>/dev/null || true
    
    GO_BUILD_EXIT_CODE=0
    if [ -f "${GO_BUILD_LOG}.exit" ]; then
        GO_BUILD_EXIT_CODE=$(cat "${GO_BUILD_LOG}.exit" 2>/dev/null || echo "1")
        rm -f "${GO_BUILD_LOG}.exit" 2>/dev/null || true
    else
        # No exit code file; check log for errors
        if [ -f "$GO_BUILD_LOG" ] && grep -q -i "error\|failed" "$GO_BUILD_LOG" 2>/dev/null; then
            GO_BUILD_EXIT_CODE=1
        fi
    fi
    
    if [ $GO_BUILD_EXIT_CODE -eq 0 ]; then
        success "Build complete: $BINARY_NAME"
        rm -f "$GO_BUILD_LOG"
    else
        error "Build failed"
        # Show build errors
        echo ""
        info "Build error details:"
        cat "$GO_BUILD_LOG" | sed 's/^/  /'
        echo ""
        rm -f "$GO_BUILD_LOG"
        exit 1
    fi
}

# Check whether a rebuild is needed
need_rebuild() {
    if [ ! -f "$BINARY_NAME" ]; then
        return 0  # needs build
    fi
    
    # Check if source changed since last build
    if [ "$BINARY_NAME" -ot cmd/server/main.go ] || \
       [ "$BINARY_NAME" -ot go.mod ] || \
       find internal cmd -name "*.go" -newer "$BINARY_NAME" 2>/dev/null | grep -q .; then
        return 0  # needs rebuild
    fi
    
    return 1  # no rebuild needed
}

read_pid() {
    if [ -f "$PID_FILE" ]; then
        tr -d '[:space:]' < "$PID_FILE" 2>/dev/null || true
    fi
}

is_running() {
    local pid="${1:-}"
    if [ -z "$pid" ]; then
        return 1
    fi
    kill -0 "$pid" 2>/dev/null
}

status_server() {
    local pid
    pid="$(read_pid)"
    if is_running "$pid"; then
        success "CyberStrikeAI is running in background"
        info "PID: $pid"
        info "PID file: $PID_FILE"
        info "Log file: $SERVER_LOG"
        return 0
    fi

    if [ -n "$pid" ]; then
        warning "PID file exists but process is not running: $pid"
        info "PID file: $PID_FILE"
    else
        warning "CyberStrikeAI is not running (no PID file)"
    fi
    return 1
}

stop_server() {
    local pid
    pid="$(read_pid)"
    if ! is_running "$pid"; then
        if [ -n "$pid" ]; then
            warning "Removing stale PID file: $PID_FILE"
            rm -f "$PID_FILE"
        else
            warning "CyberStrikeAI is not running"
        fi
        return 0
    fi

    info "Stopping CyberStrikeAI server (PID: $pid)..."
    kill -TERM "$pid" 2>/dev/null || true
    for _ in {1..20}; do
        if ! is_running "$pid"; then
            rm -f "$PID_FILE"
            success "CyberStrikeAI server stopped"
            return 0
        fi
        sleep 0.5
    done

    warning "Timed out waiting for CyberStrikeAI server to stop"
    warning "Still running PID: $pid"
    return 1
}

build_server_args() {
    SERVER_ARGS=(-config "$CONFIG_FILE")
    if [ "$USE_HTTPS" -eq 1 ]; then
        SERVER_ARGS+=(--https)
    fi
    if [ "${#FORWARD_ARGS[@]}" -gt 0 ]; then
        SERVER_ARGS+=("${FORWARD_ARGS[@]}")
    fi
}

read_config_auth_password() {
    awk '
        /^[[:space:]]*auth:[[:space:]]*$/ {
            in_auth = 1
            next
        }
        in_auth && /^[^[:space:]#][^:]*:/ {
            in_auth = 0
        }
        in_auth && /^[[:space:]]*password:[[:space:]]*/ {
            line = $0
            sub(/^[[:space:]]*password:[[:space:]]*/, "", line)
            sub(/[[:space:]]+#.*$/, "", line)
            gsub(/^"/, "", line)
            gsub(/"$/, "", line)
            gsub(/^\047/, "", line)
            gsub(/\047$/, "", line)
            print line
            exit
        }
    ' "$CONFIG_FILE" 2>/dev/null || true
}

print_generated_password_from_log() {
    local offset="${1:-0}"
    local pid="${2:-}"
    local password=""
    local new_log=""
    local configured_password=""

    for _ in {1..20}; do
        if [ -f "$SERVER_LOG" ]; then
            new_log="$(tail -c +"$((offset + 1))" "$SERVER_LOG" 2>/dev/null || true)"
            password="$(printf "%s\n" "$new_log" | awk -F': ' '/^Password: / { value=$2 } END { print value }')"
            if [ -n "$password" ]; then
                echo ""
                success "Web login password generated"
                echo "=========================================="
                echo "  CyberStrikeAI Auto-Generated Web Password"
                echo "  Password: $password"
                echo "=========================================="
                warning "Anyone with this password can fully control CyberStrikeAI. Store it securely and change auth.password soon."
                return 0
            fi
        fi

        configured_password="$(read_config_auth_password)"
        if [ -n "$configured_password" ]; then
            echo ""
            success "Web login password is configured"
            echo "=========================================="
            echo "  CyberStrikeAI Web Login Password"
            echo "  Password: $configured_password"
            echo "=========================================="
            warning "Anyone with this password can fully control CyberStrikeAI. Store it securely."
            return 0
        fi

        if [ -n "$pid" ] && ! is_running "$pid"; then
            break
        fi
        sleep 0.5
    done

    note "No Web login password was found in this startup log or config.yaml."
    return 1
}

start_server_background() {
    mkdir -p "$LOG_DIR" "$RUN_DIR"

    local existing_pid
    existing_pid="$(read_pid)"
    if is_running "$existing_pid"; then
        success "CyberStrikeAI server is already running in background"
        info "PID: $existing_pid"
        info "Log file: $SERVER_LOG"
        info "Stop it with: $0 --stop"
        return 0
    fi
    if [ -n "$existing_pid" ]; then
        warning "Removing stale PID file: $PID_FILE"
        rm -f "$PID_FILE"
    fi

    build_server_args

    if [ "$USE_HTTPS" -eq 1 ]; then
        info "Starting CyberStrikeAI server in background (HTTPS + HTTP/2, self-signed cert)..."
        note "For plain HTTP, use: $0 --http"
    else
        info "Starting CyberStrikeAI server in background (HTTP)..."
        note "For HTTPS with a self-signed cert, use: $0 --https"
    fi
    info "Log file: $SERVER_LOG"
    echo "=========================================="
    echo ""

    local log_offset=0
    if [ -f "$SERVER_LOG" ]; then
        log_offset="$(wc -c < "$SERVER_LOG" 2>/dev/null | tr -d '[:space:]' || echo 0)"
        if [ -z "$log_offset" ]; then
            log_offset=0
        fi
    fi

    local https_env=0
    if [ "$USE_HTTPS" -eq 1 ]; then
        https_env=1
    fi

    CYBERSTRIKE_HTTPS="$https_env" nohup "./$BINARY_NAME" "${SERVER_ARGS[@]}" >>"$SERVER_LOG" 2>&1 </dev/null &
    local server_pid=$!
    echo "$server_pid" > "$PID_FILE"

    sleep 1
    if is_running "$server_pid"; then
        success "CyberStrikeAI server started in background"
        info "PID: $server_pid"
        info "PID file: $PID_FILE"
        info "Log file: $SERVER_LOG"
        info "Stop: $0 --stop"
        info "Status: $0 --status"
        info "Foreground mode: $0 --foreground"
        print_generated_password_from_log "$log_offset" "$server_pid" || true
        return 0
    fi

    rm -f "$PID_FILE"
    error "CyberStrikeAI server failed to start"
    if [ -f "$SERVER_LOG" ]; then
        info "Last log lines:"
        tail -n 20 "$SERVER_LOG" | sed 's/^/  /'
    fi
    return 1
}

start_server_foreground() {
    build_server_args

    if [ "$USE_HTTPS" -eq 1 ]; then
        info "Starting CyberStrikeAI server in foreground (HTTPS + HTTP/2, self-signed cert)..."
        note "For plain HTTP, use: $0 --http"
    else
        info "Starting CyberStrikeAI server in foreground (HTTP)..."
        note "For HTTPS with a self-signed cert, use: $0 --https"
    fi
    echo "=========================================="
    echo ""

    local https_env=0
    if [ "$USE_HTTPS" -eq 1 ]; then
        https_env=1
    fi

    CYBERSTRIKE_HTTPS="$https_env" exec "./$BINARY_NAME" "${SERVER_ARGS[@]}"
}

# Main flow
# Default: background HTTPS; --http uses plain HTTP.
main() {
    USE_HTTPS=1
    RUN_IN_BACKGROUND=1
    ACTION="start"
    FORWARD_ARGS=()
    for arg in "$@"; do
        case "$arg" in
            --http)
                USE_HTTPS=0
                ;;
            --https)
                USE_HTTPS=1
                ;;
            --foreground)
                RUN_IN_BACKGROUND=0
                ;;
            --background|--daemon)
                RUN_IN_BACKGROUND=1
                ;;
            --stop)
                ACTION="stop"
                ;;
            --status)
                ACTION="status"
                ;;
            --restart)
                ACTION="restart"
                ;;
            *)
                FORWARD_ARGS+=("$arg")
                ;;
        esac
    done

    case "$ACTION" in
        stop)
            stop_server
            return $?
            ;;
        status)
            status_server
            return $?
            ;;
        restart)
            stop_server || exit 1
            ;;
    esac

    check_config_file
    print_start_banner

    # Environment checks
    info "Checking runtime environment..."
    check_python
    check_go
    echo ""
    
    # Python setup
    info "Setting up Python environment..."
    setup_python_env
    echo ""
    
    # Go build
    if need_rebuild; then
        info "Preparing to build project..."
        build_go_project
    else
        success "Binary is up to date; skipping build"
    fi
    echo ""
    
    # Start server
    success "All setup complete!"
    echo ""

    # Always pass config.yaml from project root so cwd does not matter; extra args still apply (e.g. -config override; last Go flag wins).
    if [ "$RUN_IN_BACKGROUND" -eq 1 ]; then
        start_server_background
    else
        start_server_foreground
    fi
}

# Run main (supports args, e.g. ./run.sh --http, ./run.sh --foreground, ./run.sh --stop)
main "$@"
