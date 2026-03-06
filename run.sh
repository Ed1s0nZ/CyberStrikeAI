#!/bin/bash

set -euo pipefail

# CyberStrikeAI One-Click Deploy Script
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

# Temporary mirror config (only effective in this script)
PIP_INDEX_URL="${PIP_INDEX_URL:-https://pypi.tuna.tsinghua.edu.cn/simple}"
GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"

# Save original environment variables (for restoration)
ORIGINAL_PIP_INDEX_URL="${PIP_INDEX_URL:-}"
ORIGINAL_GOPROXY="${GOPROXY:-}"

# Progress display function
show_progress() {
    local pid=$1
    local message=$2
    local i=0
    local dots=""
    
    # Check if process exists
    if ! kill -0 "$pid" 2>/dev/null; then
        # Process already ended, return immediately
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
        
        # Check again if process still exists
        if ! kill -0 "$pid" 2>/dev/null; then
            break
        fi
    done
    printf "\r"
}

echo ""
echo "=========================================="
echo "  CyberStrikeAI One-Click Deploy Script"
echo "=========================================="
echo ""

# Show temporary mirror config info
echo ""
warning "⚠️  Note: This script will use temporary mirror sources to speed up downloads"
echo ""
info "Python pip temporary mirror:"
echo "  ${PIP_INDEX_URL}"
info "Go Proxy temporary mirror:"
echo "  ${GOPROXY}"
echo ""
note "These settings only apply during script execution, system config is not modified"
echo ""
sleep 1

CONFIG_FILE="$ROOT_DIR/config.yaml"
VENV_DIR="$ROOT_DIR/venv"
REQUIREMENTS_FILE="$ROOT_DIR/requirements.txt"
BINARY_NAME="cyberstrike-ai"
LOG_DIR="$ROOT_DIR/logs"
LOG_FILE="$LOG_DIR/suite.log"
PID_FILE="$LOG_DIR/suite.pid"
HTTP_ADDR="${HTTP_ADDR:-127.0.0.1:18080}"

# Check configuration file
if [ ! -f "$CONFIG_FILE" ]; then
    error "Config file config.yaml does not exist"
    info "Please ensure you run this script from the project root directory"
    exit 1
fi

# Check and install Python environment
check_python() {
    if ! command -v python3 >/dev/null 2>&1; then
        error "python3 not found"
        echo ""
        info "Please install Python 3.10 or higher first:"
        echo "  macOS:   brew install python3"
        echo "  Ubuntu:  sudo apt-get install python3 python3-venv"
        echo "  CentOS:  sudo yum install python3 python3-pip"
        exit 1
    fi
    
    PYTHON_VERSION=$(python3 --version 2>&1 | awk '{print $2}')
    PYTHON_MAJOR=$(echo "$PYTHON_VERSION" | cut -d. -f1)
    PYTHON_MINOR=$(echo "$PYTHON_VERSION" | cut -d. -f2)
    
    if [ "$PYTHON_MAJOR" -lt 3 ] || ([ "$PYTHON_MAJOR" -eq 3 ] && [ "$PYTHON_MINOR" -lt 10 ]); then
        error "Python version too low: $PYTHON_VERSION (requires 3.10+)"
        exit 1
    fi
    
    success "Python environment check passed: $PYTHON_VERSION"
}

# Check and install Go environment
check_go() {
    if ! command -v go >/dev/null 2>&1; then
        error "Go not found"
        echo ""
        info "Please install Go 1.21 or higher first:"
        echo "  macOS:   brew install go"
        echo "  Ubuntu:  sudo apt-get install golang-go"
        echo "  CentOS:  sudo yum install golang"
        echo "  Or visit:  https://go.dev/dl/"
        exit 1
    fi
    
    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    GO_MAJOR=$(echo "$GO_VERSION" | cut -d. -f1)
    GO_MINOR=$(echo "$GO_VERSION" | cut -d. -f2)
    
    if [ "$GO_MAJOR" -lt 1 ] || ([ "$GO_MAJOR" -eq 1 ] && [ "$GO_MINOR" -lt 21 ]); then
        error "Go version too low: $GO_VERSION (requires 1.21+)"
        exit 1
    fi
    
    success "Go environment check passed: $(go version)"
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
        local req_hash_file="$VENV_DIR/.requirements.sha256"
        local current_req_hash=""
        local installed_req_hash=""

        if command -v sha256sum >/dev/null 2>&1; then
            current_req_hash="$(sha256sum "$REQUIREMENTS_FILE" | awk '{print $1}')"
        elif command -v shasum >/dev/null 2>&1; then
            current_req_hash="$(shasum -a 256 "$REQUIREMENTS_FILE" | awk '{print $1}')"
        fi

        if [ -f "$req_hash_file" ]; then
            installed_req_hash="$(cat "$req_hash_file" 2>/dev/null || true)"
        fi

        if [ -n "$current_req_hash" ] && [ "$current_req_hash" = "$installed_req_hash" ]; then
            success "Python dependencies unchanged, skipping pip install"
            return
        fi

        echo ""
        note "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        note "⚠️  Using temporary pip mirror (only valid for this script run)"
        note "   Mirror URL: ${PIP_INDEX_URL}"
        note "   For permanent config, set env var PIP_INDEX_URL"
        note "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        echo ""
        
        info "Upgrading pip..."
        pip install --index-url "$PIP_INDEX_URL" --upgrade pip >/dev/null 2>&1 || true
        
        info "Installing Python packages..."
        echo ""
        
        # Try to install dependencies, capture error output and show progress
        PIP_LOG=$(mktemp)
        (
            set +e  # Disable error exit in subshell
            pip install --index-url "$PIP_INDEX_URL" -r "$REQUIREMENTS_FILE" >"$PIP_LOG" 2>&1
            echo $? > "${PIP_LOG}.exit"
        ) &
        PIP_PID=$!
        
        # Wait briefly to ensure process has started
        sleep 0.1
        
        # Show progress (if process is still running)
        if kill -0 "$PIP_PID" 2>/dev/null; then
            show_progress "$PIP_PID" "Installing packages"
        else
            # Process already ended, wait to ensure exit code file is written
            sleep 0.2
        fi
        
        # Wait for process to complete, ignore wait exit code
        wait "$PIP_PID" 2>/dev/null || true
        
        PIP_EXIT_CODE=0
        if [ -f "${PIP_LOG}.exit" ]; then
            PIP_EXIT_CODE=$(cat "${PIP_LOG}.exit" 2>/dev/null || echo "1")
            rm -f "${PIP_LOG}.exit" 2>/dev/null || true
        else
            # If no exit code file, check log for errors
            if [ -f "$PIP_LOG" ] && grep -q -i "error\|failed\|exception" "$PIP_LOG" 2>/dev/null; then
                PIP_EXIT_CODE=1
            fi
        fi
        
        if [ $PIP_EXIT_CODE -eq 0 ]; then
            success "Python dependencies installed"
            if [ -n "$current_req_hash" ]; then
                echo "$current_req_hash" > "$req_hash_file"
            fi
        else
            # Check if angr installation failed (requires Rust)
            if grep -q "angr" "$PIP_LOG" && grep -q "Rust compiler\|can't find Rust" "$PIP_LOG"; then
                warning "angr installation failed (requires Rust compiler)"
                echo ""
                info "angr is an optional dependency, mainly for binary analysis tools"
                info "To use angr, please install Rust first:"
                echo "  macOS:   curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh"
                echo "  Ubuntu:  curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh"
                echo "  Or visit:  https://rustup.rs/"
                echo ""
                info "Other dependencies installed, you can continue (some tools may be unavailable)"
            else
                warning "Some Python dependencies failed to install, but you can try to continue"
                warning "If issues arise, check the error messages and manually install missing dependencies"
                # Show last few lines of error info
                echo ""
                info "Error details (last 10 lines):"
                tail -n 10 "$PIP_LOG" | sed 's/^/  /'
                echo ""
            fi
        fi
        rm -f "$PIP_LOG"
    else
        warning "requirements.txt not found, skipping Python dependencies installation"
    fi
}

# Build Go project
build_go_project() {
    echo ""
    note "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    note "⚠️  Using temporary Go Proxy (only valid for this script run)"
    note "   Proxy URL: ${GOPROXY}"
    note "   For permanent config, set env var GOPROXY"
    note "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    
    info "Downloading Go dependencies..."
    GO_DOWNLOAD_LOG=$(mktemp)
    (
        set +e  # Disable error exit in subshell
        export GOPROXY="$GOPROXY"
        go mod download >"$GO_DOWNLOAD_LOG" 2>&1
        echo $? > "${GO_DOWNLOAD_LOG}.exit"
    ) &
    GO_DOWNLOAD_PID=$!
    
    # Wait briefly to ensure process has started
    sleep 0.1
    
    # Show progress (if process is still running)
    if kill -0 "$GO_DOWNLOAD_PID" 2>/dev/null; then
        show_progress "$GO_DOWNLOAD_PID" "Downloading Go dependencies"
    else
        # Process already ended, wait to ensure exit code file is written
        sleep 0.2
    fi
    
    # Wait for process to complete, ignore wait exit code
    wait "$GO_DOWNLOAD_PID" 2>/dev/null || true
    
    GO_DOWNLOAD_EXIT_CODE=0
    if [ -f "${GO_DOWNLOAD_LOG}.exit" ]; then
        GO_DOWNLOAD_EXIT_CODE=$(cat "${GO_DOWNLOAD_LOG}.exit" 2>/dev/null || echo "1")
        rm -f "${GO_DOWNLOAD_LOG}.exit" 2>/dev/null || true
    else
        # If no exit code file, check log for errors
        if [ -f "$GO_DOWNLOAD_LOG" ] && grep -q -i "error\|failed" "$GO_DOWNLOAD_LOG" 2>/dev/null; then
            GO_DOWNLOAD_EXIT_CODE=1
        fi
    fi
    rm -f "$GO_DOWNLOAD_LOG" 2>/dev/null || true
    
    if [ $GO_DOWNLOAD_EXIT_CODE -ne 0 ]; then
        error "Go dependencies download failed"
        exit 1
    fi
    success "Go dependencies downloaded"
    
    info "Building project..."
    GO_BUILD_LOG=$(mktemp)
    (
        set +e  # Disable error exit in subshell
        export GOPROXY="$GOPROXY"
        go build -o "$BINARY_NAME" cmd/server/main.go >"$GO_BUILD_LOG" 2>&1
        echo $? > "${GO_BUILD_LOG}.exit"
    ) &
    GO_BUILD_PID=$!
    
    # Wait briefly to ensure process has started
    sleep 0.1
    
    # Show progress (if process is still running)
    if kill -0 "$GO_BUILD_PID" 2>/dev/null; then
        show_progress "$GO_BUILD_PID" "Building project"
    else
        # Process already ended, wait to ensure exit code file is written
        sleep 0.2
    fi
    
    # Wait for process to complete, ignore wait exit code
    wait "$GO_BUILD_PID" 2>/dev/null || true
    
    GO_BUILD_EXIT_CODE=0
    if [ -f "${GO_BUILD_LOG}.exit" ]; then
        GO_BUILD_EXIT_CODE=$(cat "${GO_BUILD_LOG}.exit" 2>/dev/null || echo "1")
        rm -f "${GO_BUILD_LOG}.exit" 2>/dev/null || true
    else
        # If no exit code file, check log for errors
        if [ -f "$GO_BUILD_LOG" ] && grep -q -i "error\|failed" "$GO_BUILD_LOG" 2>/dev/null; then
            GO_BUILD_EXIT_CODE=1
        fi
    fi
    
    if [ $GO_BUILD_EXIT_CODE -eq 0 ]; then
        success "Project build complete: $BINARY_NAME"
        rm -f "$GO_BUILD_LOG"
    else
        error "Project build failed"
        # Show build errors
        echo ""
        info "Build error details:"
        cat "$GO_BUILD_LOG" | sed 's/^/  /'
        echo ""
        rm -f "$GO_BUILD_LOG"
        exit 1
    fi
}

# Check if rebuild is needed
need_rebuild() {
    if [ ! -f "$BINARY_NAME" ]; then
        return 0  # Build needed
    fi
    
    # Check if source code has updates
    if [ "$BINARY_NAME" -ot cmd/server/main.go ] || \
       [ "$BINARY_NAME" -ot go.mod ] || \
       find internal cmd -name "*.go" -newer "$BINARY_NAME" 2>/dev/null | grep -q .; then
        return 0  # Rebuild needed
    fi
    
    return 1  # No build needed
}

# Main flow
main() {
    local action="${1:-start}"

    mkdir -p "$LOG_DIR"

    case "$action" in
        stop)
            stop_server
            exit 0
            ;;
        status)
            status_server
            exit 0
            ;;
        logs)
            tail_logs
            exit 0
            ;;
        restart)
            stop_server
            ;;
        start|foreground)
            ;;
        *)
            show_usage
            exit 1
            ;;
    esac

    # Environment check
    info "Checking runtime environment..."
    check_python
    check_go
    echo ""
    
    # Set up Python environment
    info "Setting up Python environment..."
    setup_python_env
    echo ""
    
    # Build Go project
    if need_rebuild; then
        info "Preparing to build project..."
        build_go_project
    else
        success "Executable is up to date, skipping build"
    fi
    echo ""
    
    success "All preparations complete!"
    echo ""

    if [ "$action" = "foreground" ]; then
        info "Starting CyberStrikeAI server in foreground..."
        echo "=========================================="
        echo ""
        exec "./$BINARY_NAME"
    fi

    start_server
}

show_usage() {
    cat <<'EOF'
Usage: ./run.sh [start|stop|restart|status|logs|foreground]
  start       Build if needed and run server in background (default)
  stop        Stop background server
  restart     Stop and start background server
  status      Show server status and listeners
  logs        Tail server logs
  foreground  Build if needed and run server in foreground
EOF
}

is_running() {
    if [ ! -f "$PID_FILE" ]; then
        return 1
    fi
    local pid
    pid="$(cat "$PID_FILE" 2>/dev/null || true)"
    if [ -z "$pid" ]; then
        return 1
    fi
    if ! kill -0 "$pid" 2>/dev/null; then
        return 1
    fi
    local cmd
    cmd="$(ps -p "$pid" -o comm= 2>/dev/null || true)"
    [ "$cmd" = "$BINARY_NAME" ]
}

start_server() {
    if is_running; then
        local pid
        pid="$(cat "$PID_FILE")"
        success "Server is already running (PID $pid)"
        status_server
        return 0
    fi

    info "Starting CyberStrikeAI server in background..."
    nohup setsid "./$BINARY_NAME" >> "$LOG_FILE" 2>&1 < /dev/null &
    local pid=$!
    disown "$pid" 2>/dev/null || true
    echo "$pid" > "$PID_FILE"

    local ok=0
    for _ in $(seq 1 40); do
        if curl -fsS "http://${HTTP_ADDR}/" >/dev/null 2>&1; then
            ok=1
            break
        fi
        sleep 0.25
    done

    if [ "$ok" -eq 1 ]; then
        success "Server started (PID $pid)"
        status_server
        return 0
    fi

    error "Server failed to become healthy on http://${HTTP_ADDR}/"
    tail -n 80 "$LOG_FILE" || true
    return 1
}

stop_server() {
    if ! is_running; then
        warning "Server is not running"
        rm -f "$PID_FILE"
        return 0
    fi

    local pid
    pid="$(cat "$PID_FILE")"
    info "Stopping server (PID $pid)..."
    kill "$pid" 2>/dev/null || true

    for _ in $(seq 1 40); do
        if ! kill -0 "$pid" 2>/dev/null; then
            rm -f "$PID_FILE"
            success "Server stopped"
            return 0
        fi
        sleep 0.25
    done

    warning "Graceful stop timed out; forcing termination"
    kill -9 "$pid" 2>/dev/null || true
    rm -f "$PID_FILE"
}

status_server() {
    if is_running; then
        local pid
        pid="$(cat "$PID_FILE")"
        success "Server is running (PID $pid)"
    else
        warning "Server is not running"
    fi

    echo ""
    info "Listener status:"
    ss -ltnp 2>/dev/null | grep -E ':18080|:18081' || echo "  No listeners on 18080/18081"
    echo ""
    info "Health check:"
    if curl -fsS "http://${HTTP_ADDR}/" >/dev/null 2>&1; then
        echo "  http://${HTTP_ADDR}/ -> OK"
    else
        echo "  http://${HTTP_ADDR}/ -> FAIL"
    fi
}

tail_logs() {
    if [ ! -f "$LOG_FILE" ]; then
        warning "Log file not found: $LOG_FILE"
        exit 0
    fi
    tail -f "$LOG_FILE"
}

# Execute main flow
main "${1:-start}"
