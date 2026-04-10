#!/usr/bin/env bash
set -euo pipefail

# CyberStrikeAI GitHub one-click upgrade script (Release/Tag)
#
# Default preserves:
# - config.yaml
# - data/
# - venv/ (disabled with --no-venv)
#
# Optional preserves (may overwrite upstream updates):
# - roles/
# - skills/
# - tools/
# Enable with --preserve-custom

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT_DIR"

BINARY_NAME="cyberstrike-ai"
CONFIG_FILE="$ROOT_DIR/config.yaml"
DATA_DIR="$ROOT_DIR/data"
VENV_DIR="$ROOT_DIR/venv"
KNOWLEDGE_BASE_DIR="$ROOT_DIR/knowledge_base"

BACKUP_BASE_DIR="$ROOT_DIR/.upgrade-backup"

GITHUB_REPO="Ed1s0nZ/CyberStrikeAI"

TAG=""
PRESERVE_CUSTOM=0
PRESERVE_VENV=1
STOP_SERVICE=1
FORCE_STOP=0
YES=0

usage() {
  cat <<EOF
Usage:
  ./upgrade.sh [--tag vX.Y.Z] [--preserve-custom] [--no-venv] [--no-stop]
                [--force-stop] [--yes]

Options:
  --tag <tag>          Specify GitHub Release tag (e.g. v1.3.28).
                        If omitted, the script uses the latest release.
  --preserve-custom    Preserve roles/skills/tools (may overwrite upstream files).
                        Use with caution.
  --no-venv             Do not preserve venv/ (Python deps will be re-installed).
  --no-stop             Do not try to stop the running service.
  --force-stop         If no process matching current directory is found, also stop
                        any cyberstrike-ai processes (use with caution).
  --yes                 Do not ask for confirmation.

Description:
  The script backs up config.yaml/data/ (and optionally venv/roles/skills/tools) to
  .upgrade-backup/
EOF
}

log() { printf "%s\n" "$*"; }
info() { log "[INFO]  $*"; }
warn() { log "[WARN]  $*"; }
err() { log "[ERROR] $*"; }

have_cmd() { command -v "$1" >/dev/null 2>&1; }

http_get() {
  # $1: url
  if have_cmd curl; then
    # If GITHUB_TOKEN is provided, use it for api.github.com to avoid low rate limits.
    if [[ -n "${GITHUB_TOKEN:-}" && "$1" == https://api.github.com/* ]]; then
      # Do not use `-f` so we can parse GitHub error JSON bodies and show `message`.
      curl -sSL -H "Authorization: Bearer ${GITHUB_TOKEN}" "$1"
    else
      # Do not use `-f` so we can parse GitHub error JSON bodies and show `message`.
      curl -sSL "$1"
    fi
  elif have_cmd wget; then
    wget -qO- "$1"
  else
    err "curl or wget is required to download GitHub releases. Please install one of them."
    exit 1
  fi
}

stop_service() {
  # Try to stop the service that is running from the current project directory.
  # If nothing is found and --force-stop is enabled, stop all cyberstrike-ai processes.
  if [[ "$STOP_SERVICE" -ne 1 ]]; then
    return 0
  fi

  local pids=""
  if have_cmd pgrep; then
    # Prefer matches where the command line contains the current project path.
    pids="$(pgrep -f "${ROOT_DIR}.*${BINARY_NAME}" || true)"
    if [[ -z "$pids" && "$FORCE_STOP" -eq 1 ]]; then
      warn "No ${BINARY_NAME} process found under the current directory. Will try to force-stop all matching ${BINARY_NAME} processes."
      pids="$(pgrep -f "${BINARY_NAME}" || true)"
    fi
  fi

  if [[ -z "$pids" ]]; then
    info "No ${BINARY_NAME} process detected (or no matching process). Skipping stop step."
    return 0
  fi

  warn "Detected running PID(s): ${pids}"
  for pid in $pids; do
    if kill -0 "$pid" 2>/dev/null; then
      info "Sending SIGTERM to PID=${pid}..."
      kill -TERM "$pid" 2>/dev/null || true
    fi
  done

  # Wait for exit
  local deadline=$((SECONDS + 20))
  while [[ $SECONDS -lt $deadline ]]; do
    local alive=0
    for pid in $pids; do
      if kill -0 "$pid" 2>/dev/null; then
        alive=1
        break
      fi
    done
    if [[ "$alive" -eq 0 ]]; then
      info "Service stopped."
      return 0
    fi
    sleep 1
  done

  warn "Timed out waiting for processes to exit. Still running PID(s): ${pids} (may still hold file handles)."
  return 0
}

backup_dir_tgz() {
  # $1: label, $2: path
  local label="$1"
  local path="$2"
  if [[ -e "$path" ]]; then
    info "Backing up ${label} -> ${BACKUP_BASE_DIR}/$(basename "$path").tgz"
    tar -czf "${BACKUP_BASE_DIR}/$(basename "$path").tgz" -C "$ROOT_DIR" "$(basename "$path")"
  fi
}

backup_config() {
  if [[ -f "$CONFIG_FILE" ]]; then
    cp -a "$CONFIG_FILE" "${BACKUP_BASE_DIR}/config.yaml"
  fi
}

ensure_git_style_env() {
  # No hard requirement; just a sanity check.
  if [[ ! -f "$CONFIG_FILE" ]]; then
    err "Could not find ${CONFIG_FILE}. Please verify you are in the correct project directory."
    exit 1
  fi
}

confirm_or_exit() {
  if [[ "$YES" -eq 1 ]]; then
    return 0
  fi

  if [[ ! -t 0 ]]; then
    err "Non-interactive terminal detected. Please add --yes to continue."
    exit 1
  fi

  warn "About to perform upgrade:"
  info " - Preserve config.yaml: yes"
  info " - Preserve data/: yes"
  if [[ "$PRESERVE_VENV" -eq 1 ]]; then
    info " - Preserve venv/: yes"
  else
    info " - Preserve venv/: no (will remove old venv and re-install deps)"
  fi
  if [[ "$PRESERVE_CUSTOM" -eq 1 ]]; then
    info " - Preserve roles/skills/tools: yes (may overwrite upstream updates)"
  else
    info " - Preserve roles/skills/tools: no (will use upstream versions)"
  fi
  info " - Stop service: ${STOP_SERVICE}"
  echo ""
  read -r -p "Continue? (y/N) " ans
  if [[ "${ans:-N}" != "y" && "${ans:-N}" != "Y" ]]; then
    err "Cancelled."
    exit 1
  fi
}

resolve_tag() {
  if [[ -n "$TAG" ]]; then
    info "Using specified tag: $TAG"
    return 0
  fi

  local api_url="https://api.github.com/repos/${GITHUB_REPO}/releases/latest"
  info "Fetching latest Release..."
  local json
  json="$(http_get "$api_url")"
  TAG="$(printf '%s' "$json" | python3 - <<'PY'
import json, sys
data=json.loads(sys.stdin.read() or "{}")
print(data.get("tag_name",""))
PY
)"

  if [[ -z "$TAG" ]]; then
    local msg
    msg="$(printf '%s' "$json" | python3 -c "import sys,json; d=json.loads(sys.stdin.read() or '{}'); print(d.get('message',''))" 2>/dev/null || true)"

    # Fallback: try query releases list (sometimes latest endpoint returns error JSON without tag_name).
    local fallback_url="https://api.github.com/repos/${GITHUB_REPO}/releases?per_page=1"
    info "Fallback to: ${fallback_url}"
    local fallback_json
    fallback_json="$(http_get "$fallback_url" 2>/dev/null || true)"
    local fallback_tag
    fallback_tag="$(printf '%s' "$fallback_json" | python3 -c "import sys,json; d=json.loads(sys.stdin.read() or '[]'); print(d[0].get('tag_name','') if isinstance(d,list) and d else '')" 2>/dev/null || true)"

    if [[ -n "$fallback_tag" ]]; then
      TAG="$fallback_tag"
      info "Latest Release tag (fallback): $TAG"
      return 0
    fi

    local snippet
    snippet="$(printf '%s' "$json" | python3 -c "import sys; s=sys.stdin.read(); print(s[:300].replace('\\n',' '))" 2>/dev/null || true)"

    if [[ -n "$msg" ]]; then
      err "Failed to fetch latest tag: ${msg}"
    else
      err "Failed to fetch latest tag."
    fi
    if [[ -n "$snippet" ]]; then
      err "API response snippet: ${snippet}"
    fi
    err "Please try using --tag to specify the version, or set export GITHUB_TOKEN=\"...\"."
    exit 1
  fi
  info "Latest Release tag: $TAG"
}

update_config_version() {
  # Replace config.yaml's version: ... with the specified tag.
  local new_tag="$1"
  python3 - "$CONFIG_FILE" "$new_tag" <<PY
import re, sys
path=sys.argv[1]
tag=sys.argv[2]
with open(path, "r", encoding="utf-8") as f:
    lines=f.readlines()

out=[]
replaced=False
for line in lines:
    if re.match(r'^\s*version\s*:', line):
        out.append(f'version: "{tag}"\\n')
        replaced=True
    else:
        out.append(line)

if not replaced:
    # If no version field is found, insert at the beginning (near the top).
    out.insert(0, f'version: "{tag}"\\n')

with open(path, "w", encoding="utf-8") as f:
    f.writelines(out)
PY
}

sync_code() {
  local tmp_dir="$1"
  local new_src_dir="$2"

  # rsync sync: overwrite files from the new version and delete removed files.
  # Preserve user data/config (and optional directories).

  if ! have_cmd rsync; then
    err "rsync not found. This script depends on rsync for safe synchronization. Please install it and retry."
    exit 1
  fi

  local -a rsync_excludes
  rsync_excludes+=( "--exclude=.upgrade-backup/" )
  rsync_excludes+=( "--exclude=config.yaml" )
  rsync_excludes+=( "--exclude=data/" )

  if [[ "$PRESERVE_VENV" -eq 1 ]]; then
    rsync_excludes+=( "--exclude=venv/" )
  fi

  # knowledge_base may not be referenced in config, but many users treat it as the knowledge files directory.
  if [[ -d "$KNOWLEDGE_BASE_DIR" ]]; then
    rsync_excludes+=( "--exclude=knowledge_base/" )
  fi

  if [[ "$PRESERVE_CUSTOM" -eq 1 ]]; then
    rsync_excludes+=( "--exclude=roles/" )
    rsync_excludes+=( "--exclude=skills/" )
    rsync_excludes+=( "--exclude=tools/" )
  fi

  # Ensure this upgrade script itself is not deleted.
  rsync_excludes+=( "--exclude=upgrade.sh" )

  # shellcheck disable=SC2068
  info "Syncing code into current directory (preserving data/config; using rsync --delete)..."
  rsync -a --delete \
    ${rsync_excludes[@]} \
    "${new_src_dir}/" "${ROOT_DIR}/"
}

main() {
  ensure_git_style_env

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --tag)
        TAG="${2:-}"
        shift 2
        ;;
      --preserve-custom)
        PRESERVE_CUSTOM=1
        shift 1
        ;;
      --no-venv)
        PRESERVE_VENV=0
        shift 1
        ;;
      --no-stop)
        STOP_SERVICE=0
        shift 1
        ;;
      --force-stop)
        FORCE_STOP=1
        shift 1
        ;;
      --yes)
        YES=1
        shift 1
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        err "Unknown parameter: $1"
        usage
        exit 1
        ;;
    esac
  done

  confirm_or_exit

  stop_service

  resolve_tag

  local ts
  ts="$(date +"%Y%m%d_%H%M%S")"
  BACKUP_BASE_DIR="${BACKUP_BASE_DIR}/${ts}"
  mkdir -p "$BACKUP_BASE_DIR"

  info "Starting backup into: $BACKUP_BASE_DIR"
  backup_config
  backup_dir_tgz "data" "$DATA_DIR"
  if [[ "$PRESERVE_VENV" -eq 1 ]]; then
    backup_dir_tgz "venv" "$VENV_DIR"
  else
    if [[ -d "$VENV_DIR" ]]; then
      warn "With --no-venv: removing old venv/ (run.sh will re-install Python deps after upgrade)."
      rm -rf "$VENV_DIR"
    fi
  fi
  if [[ -d "$KNOWLEDGE_BASE_DIR" ]]; then
    backup_dir_tgz "knowledge_base" "$KNOWLEDGE_BASE_DIR"
  fi
  if [[ "$PRESERVE_CUSTOM" -eq 1 ]]; then
    backup_dir_tgz "roles" "$ROOT_DIR/roles"
    backup_dir_tgz "skills" "$ROOT_DIR/skills"
    backup_dir_tgz "tools" "$ROOT_DIR/tools"
  fi

  local tmp_dir
  tmp_dir="$(mktemp -d)"
  trap 'rm -rf "$tmp_dir" >/dev/null 2>&1 || true' EXIT

  local tarball="${tmp_dir}/source.tar.gz"
  local url="https://github.com/${GITHUB_REPO}/archive/refs/tags/${TAG}.tar.gz"
  info "Downloading source package: ${url}"
  http_get "$url" >"$tarball"

  info "Extracting source package..."
  tar -xzf "$tarball" -C "$tmp_dir"

  # GitHub tarball usually creates a top-level directory.
  local extracted_dir
  extracted_dir="$(ls -d "${tmp_dir}"/*/ 2>/dev/null | head -n 1 || true)"
  if [[ -z "$extracted_dir" || ! -f "${extracted_dir}/run.sh" ]]; then
    err "run.sh not found in the extracted directory. Please check network/download contents."
    exit 1
  fi

  sync_code "$tmp_dir" "$extracted_dir"

  # Update config.yaml version display
  if [[ -f "$CONFIG_FILE" ]]; then
    info "Updating config.yaml version field to: $TAG"
    update_config_version "$TAG"
  fi

  info "Upgrade complete. Starting service..."
  chmod +x ./run.sh
  ./run.sh
}

main "$@"

