#!/usr/bin/env bash
set -euo pipefail

# Installs CyberStrikeAI tools/dependencies inside Docker image.
# This script is intentionally best-effort for community tools.

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
TOOLS_HOME="${TOOLS_HOME:-/opt/cyberstrike-tools}"
VENV_DIR="${VENV_DIR:-/opt/cyberstrike-venv}"

export DEBIAN_FRONTEND=noninteractive
export SKIP_DOCKER_INSTALL=1

echo "[*] Installing base OS dependencies..."
apt-get update -y
apt-get install -y --no-install-recommends \
  ca-certificates curl wget git unzip xz-utils jq \
  build-essential make cmake gcc g++ perl \
  php-cli php-zip \
  python3 python3-pip python3-venv python3-dev \
  ruby-full ruby-dev \
  golang-go \
  default-jre-headless

apt_install_if_available() {
  local pkg="$1"
  if apt-cache show "$pkg" >/dev/null 2>&1; then
    apt-get install -y --no-install-recommends "$pkg"
    return 0
  fi
  return 1
}

install_github_binary() {
  local repo="$1" pattern="$2" bin_name="$3"
  local asset_url tmpd asset candidate
  asset_url="$(curl -fsSL "https://api.github.com/repos/${repo}/releases/latest" \
    | jq -r --arg p "$pattern" '.assets[] | select(.name | test($p)) | .browser_download_url' \
    | head -n1)"
  if [[ -z "$asset_url" || "$asset_url" == "null" ]]; then
    echo "[WARN] cannot find release asset: ${repo} pattern=${pattern}"
    return 1
  fi
  tmpd="$(mktemp -d)"
  asset="$tmpd/asset"
  if ! curl -fsSL "$asset_url" -o "$asset"; then
    rm -rf "$tmpd"
    return 1
  fi
  case "$asset_url" in
    *.zip) unzip -q "$asset" -d "$tmpd" ;;
    *.tar.gz|*.tgz) tar -xzf "$asset" -C "$tmpd" ;;
    *) ;;
  esac
  candidate="$(find "$tmpd" -type f -name "$bin_name" | head -n1 || true)"
  if [[ -z "$candidate" ]]; then
    candidate="$(find "$tmpd" -type f -perm -u+x | head -n1 || true)"
  fi
  if [[ -z "$candidate" ]]; then
    echo "[WARN] cannot locate binary ${bin_name} from ${repo}"
    rm -rf "$tmpd"
    return 1
  fi
  install -m 0755 "$candidate" "/usr/local/bin/${bin_name}"
  rm -rf "$tmpd"
  echo "[OK] installed ${bin_name} from ${repo}"
}

optional_pkgs=(
  nmap sqlmap gobuster feroxbuster ffuf nikto dirb dnsenum hydra john hashcat
  masscan wfuzz wafw00f exiftool binwalk steghide fcrackzip pdfcrack foremost
  arp-scan nbtscan smbmap checksec dirsearch radare2 gdb
)
for pkg in "${optional_pkgs[@]}"; do
  if apt_install_if_available "$pkg"; then
    echo "[OK] apt installed: $pkg"
  else
    echo "[WARN] apt package unavailable in this distro: $pkg"
  fi
done

mkdir -p "$TOOLS_HOME" /usr/share/wordlists/dirb /usr/share/wordlists/api /usr/share/wordlists

if [[ -f /usr/share/dirb/wordlists/common.txt ]]; then
  ln -sf /usr/share/dirb/wordlists/common.txt /usr/share/wordlists/dirb/common.txt
fi

if [[ ! -s /usr/share/wordlists/api/api-endpoints.txt ]]; then
  curl -fsSL "https://raw.githubusercontent.com/danielmiessler/SecLists/master/Discovery/Web-Content/api/api-endpoints.txt" \
    -o /usr/share/wordlists/api/api-endpoints.txt || true
fi
if [[ ! -s /usr/share/wordlists/rockyou.txt ]]; then
  curl -fL --retry 3 --retry-delay 2 \
    "https://github.com/brannondorsey/naive-hashcat/releases/download/data/rockyou.txt" \
    -o /usr/share/wordlists/rockyou.txt || true
fi

python3 -m venv "$VENV_DIR"
"$VENV_DIR/bin/pip" install --upgrade pip
"$VENV_DIR/bin/pip" install -r "$ROOT_DIR/requirements.txt" || true
"$VENV_DIR/bin/pip" install \
  checkov kube-hunter volatility3 ropper ROPGadget fierce xsser ScoutSuite \
  requests_ntlm six python-dateutil || true

for c in arjun uro checkov fierce xsser; do
  if [[ -x "$VENV_DIR/bin/$c" ]]; then
    ln -sf "$VENV_DIR/bin/$c" "/usr/local/bin/$c"
  fi
done
if [[ -x "$VENV_DIR/bin/dirsearch" ]]; then
  ln -sf "$VENV_DIR/bin/dirsearch" /usr/local/bin/dirsearch
fi
if [[ -x "$VENV_DIR/bin/vol" ]]; then
  ln -sf "$VENV_DIR/bin/vol" /usr/local/bin/volatility3
  ln -sf "$VENV_DIR/bin/vol" /usr/local/bin/volatility
fi

export GOBIN=/usr/local/bin
export GOPATH=/root/go
echo "[*] Installing Go-based tools..."
go install github.com/projectdiscovery/nuclei/v3/cmd/nuclei@latest || true
go install github.com/projectdiscovery/httpx/cmd/httpx@latest || true
go install github.com/projectdiscovery/subfinder/v2/cmd/subfinder@latest || true
go install github.com/projectdiscovery/katana/cmd/katana@latest || true
go install github.com/lc/gau/v2/cmd/gau@latest || true
go install github.com/tomnomnom/waybackurls@latest || true
go install github.com/projectdiscovery/naabu/v2/cmd/naabu@latest || true
go install github.com/hahwul/dalfox/v2@latest || true

# fallback to prebuilt releases for tools requiring newer go toolchain
command -v nuclei >/dev/null 2>&1 || install_github_binary "projectdiscovery/nuclei" "linux_amd64.*(zip|tar.gz)$" "nuclei" || true
command -v subfinder >/dev/null 2>&1 || install_github_binary "projectdiscovery/subfinder" "linux_amd64.*(zip|tar.gz)$" "subfinder" || true
command -v katana >/dev/null 2>&1 || install_github_binary "projectdiscovery/katana" "linux_amd64.*(zip|tar.gz)$" "katana" || true
command -v feroxbuster >/dev/null 2>&1 || install_github_binary "epi052/feroxbuster" "x86_64-linux.*(zip|tar.gz)$" "feroxbuster" || true

# nikto is not available in all apt repos; install from official repo if missing.
if ! command -v nikto >/dev/null 2>&1; then
  if [[ ! -d "$TOOLS_HOME/nikto/.git" ]]; then
    git clone --depth=1 https://github.com/sullo/nikto.git "$TOOLS_HOME/nikto" || true
  fi
  if [[ -f "$TOOLS_HOME/nikto/program/nikto.pl" ]]; then
    cat >/usr/local/bin/nikto <<'EOF'
#!/usr/bin/env bash
exec perl /opt/cyberstrike-tools/nikto/program/nikto.pl "$@"
EOF
    chmod +x /usr/local/bin/nikto
  fi
fi

mkdir -p "$TOOLS_HOME/nuclei-templates"
if command -v nuclei >/dev/null 2>&1; then
  nuclei -ut -ud "$TOOLS_HOME/nuclei-templates" >/dev/null 2>&1 || true
fi

if [[ ! -d "$TOOLS_HOME/bitrix-nuclei-templates/.git" ]]; then
  git clone --depth=1 https://github.com/jhonnybonny/bitrix-nuclei-templates.git "$TOOLS_HOME/bitrix-nuclei-templates" || true
fi
mkdir -p "$TOOLS_HOME/nuclei-templates/custom"
ln -sfn "$TOOLS_HOME/bitrix-nuclei-templates" "$TOOLS_HOME/nuclei-templates/custom/bitrix" || true

if [[ ! -d "$TOOLS_HOME/DeBix/.git" ]]; then
  git clone --depth=1 https://github.com/FaLLenSkiLL1/DeBix.git "$TOOLS_HOME/DeBix" || true
fi
if [[ -f "$TOOLS_HOME/DeBix/DeBix.php" ]]; then
  cat >/usr/local/bin/debix <<'EOF'
#!/usr/bin/env bash
exec php /opt/cyberstrike-tools/DeBix/DeBix.php "$@"
EOF
  chmod +x /usr/local/bin/debix
fi

if [[ ! -d "$TOOLS_HOME/bitrix-decrypt/.git" ]]; then
  git clone --depth=1 https://github.com/jhonnybonny/bitrix-decrypt.git "$TOOLS_HOME/bitrix-decrypt" || true
fi
if [[ -f "$TOOLS_HOME/bitrix-decrypt/mail_plugin_password-main/index.php" ]]; then
  cat >/usr/local/bin/bitrix-decrypt <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
mode="mail"
pass_value=""
salt_value=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --mode) mode="${2:-}"; shift 2 ;;
    --pass) pass_value="${2:-}"; shift 2 ;;
    --salt) salt_value="${2:-}"; shift 2 ;;
    *) shift ;;
  esac
done
if [[ -z "$pass_value" ]]; then
  echo "error: --pass is required" >&2
  exit 2
fi
case "$mode" in
  mail) script="/opt/cyberstrike-tools/bitrix-decrypt/mail_plugin_password-main/index.php" ;;
  ldap) script="/opt/cyberstrike-tools/bitrix-decrypt/ldap_plugin_password-main/index.php"; [[ -z "$salt_value" ]] && salt_value="ldap" ;;
  *) echo "error: --mode must be mail or ldap" >&2; exit 2 ;;
esac
exec php "$script" "pass=${pass_value}&salt=${salt_value}"
EOF
  chmod +x /usr/local/bin/bitrix-decrypt
fi

echo "[OK] Container tool installation finished."
