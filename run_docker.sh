#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT_DIR"

SERVICE_NAME="${SERVICE_NAME:-cyberstrikeai}"
DOCKER_ENV_FILE="$ROOT_DIR/.docker/docker.env"
DOCKER_OVERRIDE_FILE="$ROOT_DIR/.docker/docker-compose.override.yml"
LOG_FILE="$ROOT_DIR/logs/docker-manager.log"

ACTION="${1:-deploy}"
shift || true

PROXY_MODE="direct"
PROXY_URL=""
VPN_CONTAINER=""
GIT_REF="main"
FOLLOW_LOGS=0

log() { printf "[%s] %s\n" "$(date -u +'%Y-%m-%dT%H:%M:%SZ')" "$*" | tee -a "$LOG_FILE"; }
err() { log "ERROR: $*"; }

usage() {
  cat <<'EOF'
Usage: ./run_docker.sh <action> [options]

Actions:
  install   Install/validate Docker engine + compose plugin
  deploy    Build and run suite container with current code
  update    Git pull (selected ref) + deploy
  start     Start existing container
  stop      Stop container
  restart   Restart container
  status    Show Docker/suite status
  logs      Show container logs (use -f for follow)
  test      Run container runtime test suite
  remove    Remove container, network, and volumes for this stack

Options:
  --proxy-mode <direct|socks|http|tor|vpn>
  --proxy-url <url>             For socks/http modes (e.g. socks5h://host:1080)
  --vpn-container <name>        For vpn mode (network_mode=container:<name>)
  --git-ref <branch-or-tag>     Used by update (default: main)
  -f, --follow                  Follow logs (for logs action)
EOF
}

ensure_dirs() {
  mkdir -p "$ROOT_DIR/.docker" "$ROOT_DIR/logs" "$ROOT_DIR/data" "$ROOT_DIR/tmp"
}

compose_cmd() {
  if docker compose version >/dev/null 2>&1; then
    docker compose "$@"
  else
    docker-compose "$@"
  fi
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --proxy-mode) PROXY_MODE="${2:-direct}"; shift 2 ;;
      --proxy-url) PROXY_URL="${2:-}"; shift 2 ;;
      --vpn-container) VPN_CONTAINER="${2:-}"; shift 2 ;;
      --git-ref) GIT_REF="${2:-main}"; shift 2 ;;
      -f|--follow) FOLLOW_LOGS=1; shift ;;
      -h|--help) usage; exit 0 ;;
      *) err "Unknown argument: $1"; usage; exit 2 ;;
    esac
  done
}

ensure_root_or_sudo() {
  if [[ "${EUID:-$(id -u)}" -eq 0 ]]; then
    return 0
  fi
  if command -v sudo >/dev/null 2>&1; then
    return 0
  fi
  err "This action needs root privileges (or sudo)."
  exit 1
}

run_root() {
  if [[ "${EUID:-$(id -u)}" -eq 0 ]]; then
    "$@"
  else
    sudo "$@"
  fi
}

install_docker() {
  ensure_root_or_sudo
  if command -v docker >/dev/null 2>&1; then
    log "Docker already installed."
  else
    log "Installing Docker engine and compose plugin..."
    run_root apt-get update -y
    run_root apt-get install -y ca-certificates curl gnupg lsb-release
    run_root install -m 0755 -d /etc/apt/keyrings
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | run_root gpg --dearmor -o /etc/apt/keyrings/docker.gpg
    run_root chmod a+r /etc/apt/keyrings/docker.gpg
    echo \
      "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
      $(. /etc/os-release && echo "$VERSION_CODENAME") stable" \
      | run_root tee /etc/apt/sources.list.d/docker.list >/dev/null
    run_root apt-get update -y
    run_root apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
  fi
  run_root systemctl enable --now docker || true
  docker version >/dev/null
  docker compose version >/dev/null
  log "Docker installation/validation completed."
}

write_env_file() {
  local all_proxy="" http_proxy="" https_proxy="" no_proxy="localhost,127.0.0.1,::1"
  case "$PROXY_MODE" in
    direct)
      ;;
    socks|http)
      all_proxy="$PROXY_URL"
      http_proxy="$PROXY_URL"
      https_proxy="$PROXY_URL"
      ;;
    tor)
      all_proxy="socks5h://host.docker.internal:9050"
      http_proxy="$all_proxy"
      https_proxy="$all_proxy"
      ;;
    vpn)
      ;;
    *)
      err "Unsupported proxy mode: $PROXY_MODE"
      exit 2
      ;;
  esac

  cat >"$DOCKER_ENV_FILE" <<EOF
DOCKER_HTTP_PORT=${DOCKER_HTTP_PORT:-18080}
DOCKER_MCP_PORT=${DOCKER_MCP_PORT:-18081}
DOCKER_PROXY_MODE=${PROXY_MODE}
DOCKER_ALL_PROXY=${all_proxy}
DOCKER_HTTP_PROXY=${http_proxy}
DOCKER_HTTPS_PROXY=${https_proxy}
DOCKER_NO_PROXY=${no_proxy}
DOCKER_VPN_CONTAINER=${VPN_CONTAINER}
EOF
  log "Wrote Docker env file: $DOCKER_ENV_FILE"
}

write_override_file() {
  cat >"$DOCKER_OVERRIDE_FILE" <<'EOF'
services:
  cyberstrikeai:
    extra_hosts:
      - "host.docker.internal:host-gateway"
    environment:
      - ALL_PROXY=${DOCKER_ALL_PROXY}
      - HTTP_PROXY=${DOCKER_HTTP_PROXY}
      - HTTPS_PROXY=${DOCKER_HTTPS_PROXY}
      - NO_PROXY=${DOCKER_NO_PROXY}
EOF

  if [[ "$PROXY_MODE" == "vpn" && -n "$VPN_CONTAINER" ]]; then
    cat >>"$DOCKER_OVERRIDE_FILE" <<EOF
    network_mode: "container:${VPN_CONTAINER}"
EOF
  fi
  log "Wrote compose override: $DOCKER_OVERRIDE_FILE"
}

deploy_stack() {
  write_env_file
  write_override_file
  compose_cmd \
    -f "$ROOT_DIR/docker-compose.yml" \
    -f "$DOCKER_OVERRIDE_FILE" \
    --env-file "$DOCKER_ENV_FILE" \
    up -d --build
  log "Deployment completed."
}

update_stack() {
  log "Updating git ref: $GIT_REF"
  git fetch --all --tags
  git checkout "$GIT_REF"
  git pull --ff-only origin "$GIT_REF"
  deploy_stack
}

show_status() {
  log "Docker version: $(docker --version 2>/dev/null || echo missing)"
  log "Compose version: $(docker compose version 2>/dev/null || docker-compose version 2>/dev/null || echo missing)"
  compose_cmd -f "$ROOT_DIR/docker-compose.yml" ps || true
  docker ps --filter "name=^/${SERVICE_NAME}$" --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}' || true
  local code
  code="$(curl -sS -m 5 -o /tmp/cs_docker_health.out -w '%{http_code}' "http://127.0.0.1:${DOCKER_HTTP_PORT:-18080}/" || true)"
  log "HTTP check on :${DOCKER_HTTP_PORT:-18080} => ${code}"
}

tail_logs() {
  if [[ "$FOLLOW_LOGS" -eq 1 ]]; then
    compose_cmd -f "$ROOT_DIR/docker-compose.yml" logs -f --tail=300 "$SERVICE_NAME"
  else
    compose_cmd -f "$ROOT_DIR/docker-compose.yml" logs --tail=300 "$SERVICE_NAME"
  fi
}

run_tests() {
  bash "$ROOT_DIR/scripts/test-docker-suite.sh"
}

remove_stack() {
  compose_cmd -f "$ROOT_DIR/docker-compose.yml" down -v --remove-orphans || true
  log "Removed Docker stack."
}

ensure_dirs
parse_args "$@"

case "$ACTION" in
  install) install_docker ;;
  deploy) deploy_stack ;;
  update) update_stack ;;
  start) compose_cmd -f "$ROOT_DIR/docker-compose.yml" up -d "$SERVICE_NAME" ;;
  stop) compose_cmd -f "$ROOT_DIR/docker-compose.yml" stop "$SERVICE_NAME" ;;
  restart) compose_cmd -f "$ROOT_DIR/docker-compose.yml" restart "$SERVICE_NAME" ;;
  status) show_status ;;
  logs) tail_logs ;;
  test) run_tests ;;
  remove) remove_stack ;;
  *) usage; exit 2 ;;
esac
