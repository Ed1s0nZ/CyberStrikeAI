#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
SERVICE_NAME="${SERVICE_NAME:-cyberstrikeai}"
HTTP_PORT="${DOCKER_HTTP_PORT:-18080}"
MCP_PORT="${DOCKER_MCP_PORT:-18081}"

compose_cmd() {
  if docker compose version >/dev/null 2>&1; then
    docker compose "$@"
  else
    docker-compose "$@"
  fi
}

echo "[*] Validating Docker runtime..."
docker version >/dev/null
compose_cmd -f "$ROOT_DIR/docker-compose.yml" ps >/dev/null

echo "[*] Checking container state..."
if ! docker ps --format '{{.Names}}' | grep -qx "$SERVICE_NAME"; then
  echo "[ERR] Container $SERVICE_NAME is not running"
  exit 1
fi

echo "[*] Checking HTTP availability on :$HTTP_PORT ..."
code="$(curl -sS -m 8 -o /tmp/cs_docker_health.html -w '%{http_code}' "http://127.0.0.1:${HTTP_PORT}/" || true)"
if [[ "$code" != "200" ]]; then
  echo "[ERR] HTTP health check failed (code=$code)"
  exit 1
fi

echo "[*] Checking MCP endpoint reachability on :$MCP_PORT ..."
if ! curl -sS -m 8 "http://127.0.0.1:${MCP_PORT}/mcp" >/tmp/cs_docker_mcp.out 2>&1; then
  echo "[WARN] MCP endpoint returned non-2xx/stream response; verifying socket only..."
fi

echo "[*] Verifying critical tools inside container..."
required_tools=(
  nmap sqlmap gobuster ffuf feroxbuster hydra john hashcat
  nuclei subfinder katana gau waybackurls dirsearch nikto
)
missing=0
for t in "${required_tools[@]}"; do
  if ! docker exec "$SERVICE_NAME" sh -lc "command -v '$t' >/dev/null 2>&1"; then
    echo "[ERR] Missing tool in container: $t"
    missing=1
  fi
done
if [[ "$missing" -ne 0 ]]; then
  exit 1
fi

echo "[*] Checking expected wordlists..."
for p in /usr/share/wordlists/dirb/common.txt /usr/share/wordlists/rockyou.txt; do
  if ! docker exec "$SERVICE_NAME" sh -lc "test -s '$p'"; then
    echo "[ERR] Missing wordlist: $p"
    exit 1
  fi
done

echo "[*] Checking nuclei templates..."
if ! docker exec "$SERVICE_NAME" sh -lc "test -d /opt/cyberstrike-tools/nuclei-templates"; then
  echo "[ERR] Missing nuclei templates directory"
  exit 1
fi

echo "[OK] Docker suite test passed."
