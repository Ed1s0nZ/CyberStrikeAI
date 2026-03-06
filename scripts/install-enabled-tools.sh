#!/usr/bin/env bash
set -euo pipefail

# Installs CyberStrikeAI enabled tools on Debian/Ubuntu hosts.
# Run from repo root:
#   sudo bash scripts/install-enabled-tools.sh
#
# Notes:
# - This script is best-effort for tools with non-standard installers.
# - Some tools require manual install (GUI/licensing/platform specific).

if [[ "${EUID:-$(id -u)}" -ne 0 ]]; then
  echo "Run as root: sudo bash $0" >&2
  exit 1
fi

if ! command -v apt-get >/dev/null 2>&1; then
  echo "apt-get not found. This script currently supports Debian/Ubuntu only." >&2
  exit 1
fi

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
TOOLS_DIR="$ROOT_DIR/tools"
VENV_DIR="$ROOT_DIR/venv"
WORK_DIR="/opt/cyberstrike-tools"
TMP_DIR="$(mktemp -d)"

export DEBIAN_FRONTEND=noninteractive
export GOBIN=/usr/local/bin
export GOPATH=/root/go
mkdir -p "$WORK_DIR" "$GOBIN"

cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

log() { printf "[*] %s\n" "$*"; }
ok() { printf "[OK] %s\n" "$*"; }
warn() { printf "[WARN] %s\n" "$*"; }

have() {
  command -v "$1" >/dev/null 2>&1
}

apt_install_if_available() {
  local pkg="$1"
  if apt-cache show "$pkg" >/dev/null 2>&1; then
    apt-get install -y --no-install-recommends "$pkg"
    return 0
  fi
  return 1
}

install_go_tool() {
  local module="$1"
  local name="$2"
  if have "$name"; then
    ok "$name already present"
    return 0
  fi
  if go install "${module}@latest"; then
    ok "installed $name via go install"
  else
    warn "failed go install for $name ($module)"
  fi
}

install_from_github_release() {
  local repo="$1"
  local pattern="$2"
  local out_name="$3"
  local asset_url
  asset_url="$(
    curl -fsSL "https://api.github.com/repos/${repo}/releases/latest" \
      | jq -r --arg p "$pattern" '.assets[] | select(.name | test($p)) | .browser_download_url' \
      | head -n1
  )"

  if [[ -z "$asset_url" || "$asset_url" == "null" ]]; then
    warn "release asset not found for ${repo} pattern=${pattern}"
    return 1
  fi

  local dl="${TMP_DIR}/asset"
  curl -fsSL "$asset_url" -o "$dl"

  case "$asset_url" in
    *.tar.gz|*.tgz)
      tar -xzf "$dl" -C "$TMP_DIR"
      ;;
    *.zip)
      unzip -q "$dl" -d "$TMP_DIR"
      ;;
    *.deb)
      dpkg -i "$dl" || apt-get install -f -y
      ;;
    *)
      :
      ;;
  esac

  if [[ "$asset_url" == *.deb ]]; then
    if have "$out_name"; then
      ok "installed $out_name from deb"
      return 0
    fi
    warn "deb install completed but $out_name not found in PATH"
    return 1
  fi

  local candidate
  candidate="$(find "$TMP_DIR" -type f -name "$out_name" | head -n1 || true)"
  if [[ -z "$candidate" ]]; then
    candidate="$(find "$TMP_DIR" -type f -perm -u+x | head -n1 || true)"
  fi

  if [[ -z "$candidate" ]]; then
    warn "unable to find binary for $out_name in extracted asset from ${repo}"
    return 1
  fi

  install -m 0755 "$candidate" "/usr/local/bin/${out_name}"
  ok "installed $out_name from ${repo}"
}

install_python_cli_symlink() {
  local cmd_name="$1"
  local source="${VENV_DIR}/bin/${cmd_name}"
  if [[ -x "$source" ]]; then
    ln -sf "$source" "/usr/local/bin/${cmd_name}"
    ok "linked $cmd_name -> $source"
  else
    warn "python console script not found: $source"
  fi
}

install_git_script_symlink() {
  local repo_url="$1"
  local repo_dir="$2"
  local src_rel="$3"
  local out_name="$4"
  if [[ ! -d "$repo_dir/.git" ]]; then
    git clone --depth=1 "$repo_url" "$repo_dir"
  else
    git -C "$repo_dir" pull --ff-only || true
  fi
  install -m 0755 "$repo_dir/$src_rel" "/usr/local/bin/$out_name"
  ok "installed $out_name from $repo_url"
}

log "Updating apt indexes"
apt-get update -y

log "Installing base dependencies"
apt-get install -y --no-install-recommends \
  ca-certificates curl wget git unzip xz-utils jq \
  build-essential make cmake gcc g++ perl \
  php-cli php-zip \
  python3 python3-pip python3-venv \
  ruby-full ruby-dev rubygems \
  golang-go \
  default-jre-headless

# docker.io conflicts with containerd.io on some hosts.
if [[ "${SKIP_DOCKER_INSTALL:-0}" == "1" ]]; then
  warn "SKIP_DOCKER_INSTALL=1; skipping docker engine package install"
elif dpkg -s containerd.io >/dev/null 2>&1; then
  warn "containerd.io is installed; skipping docker.io install to avoid conflicts"
else
  apt-get install -y --no-install-recommends docker.io || warn "unable to install docker.io"
fi

log "Installing common apt tools (where available)"
for pkg in \
  nmap sqlmap gobuster feroxbuster ffuf nikto dirb dnsenum hydra john hashcat \
  masscan wfuzz wafw00f exiftool binwalk steghide fcrackzip pdfcrack foremost \
  arp-scan nbtscan smbmap checksec dirsearch radare2 gdb ; do
  if apt_install_if_available "$pkg"; then
    ok "apt installed: $pkg"
  else
    warn "apt package unavailable: $pkg"
  fi
done

log "Installing npm/gem tools"
if ! have spectral; then
  npm install -g @stoplight/spectral-cli
fi
gem install --no-document one_gadget zsteg wpscan || true

log "Setting up project venv and Python tooling"
if [[ -x "$VENV_DIR/bin/python3" ]]; then
  if "$VENV_DIR/bin/python3" -c 'import _posixsubprocess' >/dev/null 2>&1; then
    ok "reusing existing venv: $VENV_DIR"
  else
    warn "existing venv is broken; recreating with system python"
    /usr/bin/python3 -m venv --clear "$VENV_DIR"
  fi
else
  /usr/bin/python3 -m venv "$VENV_DIR"
fi

"$VENV_DIR/bin/pip" install --upgrade pip

"$VENV_DIR/bin/pip" install -r "$ROOT_DIR/requirements.txt" || true
"$VENV_DIR/bin/pip" install checkov kube-hunter volatility3 ropper ROPGadget fierce xsser || true
"$VENV_DIR/bin/pip" install git+https://github.com/Tib3rius/AutoRecon.git || true
"$VENV_DIR/bin/pip" install git+https://github.com/swisskyrepo/GraphQLmap.git || true
"$VENV_DIR/bin/pip" install git+https://github.com/Pennyw0rth/NetExec.git || true
"$VENV_DIR/bin/pip" install ScoutSuite || true

for c in arjun uro nxc autorecon graphqlmap checkov kube-hunter ropper ROPgadget xsser ScoutSuite bloodhound-python fierce; do
  install_python_cli_symlink "$c"
done
if have nxc; then
  ln -sf "$(command -v nxc)" /usr/local/bin/netexec
  ok "linked netexec -> nxc"
fi
if [[ -x "$VENV_DIR/bin/vol" ]]; then
  ln -sf "$VENV_DIR/bin/vol" /usr/local/bin/volatility3
  ln -sf "$VENV_DIR/bin/vol" /usr/local/bin/volatility
  ok "linked volatility and volatility3 -> vol"
fi

log "Installing Go-based tools"
install_go_tool "github.com/projectdiscovery/nuclei/v3/cmd/nuclei" nuclei
install_go_tool "github.com/projectdiscovery/httpx/cmd/httpx" httpx
install_go_tool "github.com/projectdiscovery/subfinder/v2/cmd/subfinder" subfinder
install_go_tool "github.com/projectdiscovery/katana/cmd/katana" katana
install_go_tool "github.com/lc/gau/v2/cmd/gau" gau
install_go_tool "github.com/tomnomnom/waybackurls" waybackurls
install_go_tool "github.com/projectdiscovery/naabu/v2/cmd/naabu" naabu
install_go_tool "github.com/projectdiscovery/notify/cmd/notify" notify
install_go_tool "github.com/owasp-amass/amass/v4/..." amass
install_go_tool "github.com/hahwul/dalfox/v2" dalfox
install_go_tool "github.com/hakluke/hakrawler" hakrawler
install_go_tool "github.com/jaeles-project/jaeles" jaeles
install_go_tool "github.com/aquasecurity/kube-bench" kube-bench

if command -v nuclei >/dev/null 2>&1; then
  mkdir -p "$WORK_DIR/nuclei-templates"
  # Keep an isolated local template store so scans are reproducible and do not
  # depend on a mutable default home directory.
  nuclei -ut -ud "$WORK_DIR/nuclei-templates" >/dev/null 2>&1 || true
  ok "updated official nuclei templates at $WORK_DIR/nuclei-templates"
fi

log "Installing from git repos / scripts"
install_git_script_symlink "https://github.com/maurosoria/dirsearch.git" "$WORK_DIR/dirsearch" "dirsearch.py" "dirsearch"
install_git_script_symlink "https://github.com/elceef/dnstwist.git" "$WORK_DIR/dnstwist" "dnstwist.py" "dnstwist" || true
install_git_script_symlink "https://github.com/cddmp/enum4linux-ng.git" "$WORK_DIR/enum4linux-ng" "enum4linux-ng.py" "enum4linux-ng" || true
install_git_script_symlink "https://github.com/CiscoCXSecurity/enum4linux.git" "$WORK_DIR/enum4linux" "enum4linux.pl" "enum4linux" || true
install_git_script_symlink "https://github.com/wireghoul/dotdotpwn.git" "$WORK_DIR/dotdotpwn" "dotdotpwn.pl" "dotdotpwn" || true
install_git_script_symlink "https://github.com/docker/docker-bench-security.git" "$WORK_DIR/docker-bench-security" "docker-bench-security.sh" "docker-bench-security" || true

if [[ ! -d "$WORK_DIR/bitrix-nuclei-templates/.git" ]]; then
  git clone --depth=1 https://github.com/jhonnybonny/bitrix-nuclei-templates.git "$WORK_DIR/bitrix-nuclei-templates" || true
else
  git -C "$WORK_DIR/bitrix-nuclei-templates" pull --ff-only || true
fi
if [[ -d "$WORK_DIR/bitrix-nuclei-templates" ]]; then
  ok "installed bitrix nuclei templates at $WORK_DIR/bitrix-nuclei-templates"
  mkdir -p "$WORK_DIR/nuclei-templates/custom"
  rm -rf "$WORK_DIR/nuclei-templates/custom/bitrix"
  ln -s "$WORK_DIR/bitrix-nuclei-templates" "$WORK_DIR/nuclei-templates/custom/bitrix" || true
  ok "linked bitrix templates into $WORK_DIR/nuclei-templates/custom/bitrix"
fi

if [[ ! -d "$WORK_DIR/DeBix/.git" ]]; then
  git clone --depth=1 https://github.com/FaLLenSkiLL1/DeBix.git "$WORK_DIR/DeBix" || true
else
  git -C "$WORK_DIR/DeBix" pull --ff-only || true
fi
if [[ -f "$WORK_DIR/DeBix/DeBix.php" ]]; then
  cat >/usr/local/bin/debix <<'EOF'
#!/usr/bin/env bash
exec php /opt/cyberstrike-tools/DeBix/DeBix.php "$@"
EOF
  chmod +x /usr/local/bin/debix
  ok "installed debix wrapper"
fi

if [[ ! -d "$WORK_DIR/bitrix-decrypt/.git" ]]; then
  git clone --depth=1 https://github.com/jhonnybonny/bitrix-decrypt.git "$WORK_DIR/bitrix-decrypt" || true
else
  git -C "$WORK_DIR/bitrix-decrypt" pull --ff-only || true
fi
if [[ -f "$WORK_DIR/bitrix-decrypt/mail_plugin_password-main/index.php" && -f "$WORK_DIR/bitrix-decrypt/ldap_plugin_password-main/index.php" ]]; then
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
    --help|-h)
      cat <<'USAGE'
Usage: bitrix-decrypt --mode <mail|ldap> --pass <encrypted_value> [--salt <salt>]
USAGE
      exit 0
      ;;
    *)
      shift
      ;;
  esac
done

if [[ -z "$pass_value" ]]; then
  echo "error: --pass is required" >&2
  exit 2
fi

case "$mode" in
  mail)
    script="/opt/cyberstrike-tools/bitrix-decrypt/mail_plugin_password-main/index.php"
    # mail mode default salt is empty string
    ;;
  ldap)
    script="/opt/cyberstrike-tools/bitrix-decrypt/ldap_plugin_password-main/index.php"
    if [[ -z "$salt_value" ]]; then
      salt_value="ldap"
    fi
    ;;
  *)
    echo "error: --mode must be 'mail' or 'ldap'" >&2
    exit 2
    ;;
esac

exec php "$script" "pass=${pass_value}&salt=${salt_value}"
EOF
  chmod +x /usr/local/bin/bitrix-decrypt
  ok "installed bitrix-decrypt wrapper"
fi

if [[ ! -f "$WORK_DIR/jwt_tool/jwt_tool.py" ]]; then
  git clone --depth=1 https://github.com/ticarpi/jwt_tool.git "$WORK_DIR/jwt_tool" || true
fi
if [[ -f "$WORK_DIR/jwt_tool/jwt_tool.py" ]]; then
  cat >/usr/local/bin/jwt_tool <<'EOF'
#!/usr/bin/env bash
exec python3 /opt/cyberstrike-tools/jwt_tool/jwt_tool.py "$@"
EOF
  chmod +x /usr/local/bin/jwt_tool
  ok "installed jwt_tool wrapper"
fi

if [[ ! -d "$WORK_DIR/cloudmapper/.git" ]]; then
  git clone --depth=1 https://github.com/duo-labs/cloudmapper.git "$WORK_DIR/cloudmapper" || true
fi
if [[ -f "$WORK_DIR/cloudmapper/cloudmapper.py" ]]; then
  cat >/usr/local/bin/cloudmapper <<'EOF'
#!/usr/bin/env bash
exec python3 /opt/cyberstrike-tools/cloudmapper/cloudmapper.py "$@"
EOF
  chmod +x /usr/local/bin/cloudmapper
  ok "installed cloudmapper wrapper"
fi

if [[ ! -d "$WORK_DIR/ParamSpider/.git" ]]; then
  git clone --depth=1 https://github.com/devanshbatham/ParamSpider.git "$WORK_DIR/ParamSpider" || true
fi
if [[ -f "$WORK_DIR/ParamSpider/paramspider.py" ]]; then
  cat >/usr/local/bin/paramspider <<'EOF'
#!/usr/bin/env bash
exec python3 /opt/cyberstrike-tools/ParamSpider/paramspider.py "$@"
EOF
  chmod +x /usr/local/bin/paramspider
  ok "installed paramspider wrapper"
fi

if [[ -f "$ROOT_DIR/scripts/flaresolverr-client.py" ]]; then
  install -m 0755 "$ROOT_DIR/scripts/flaresolverr-client.py" /usr/local/bin/flaresolverr-client
  ok "installed flaresolverr-client wrapper"
fi

if have docker; then
  if ! docker image inspect ghcr.io/flaresolverr/flaresolverr:latest >/dev/null 2>&1; then
    docker pull ghcr.io/flaresolverr/flaresolverr:latest || warn "failed to pull flaresolverr image"
  fi

  if docker ps -a --format '{{.Names}}' | grep -qx 'flaresolverr'; then
    docker start flaresolverr >/dev/null 2>&1 || warn "failed to start existing flaresolverr container"
  else
    docker run -d \
      --name flaresolverr \
      --restart unless-stopped \
      -p 8191:8191 \
      -e LOG_LEVEL=info \
      ghcr.io/flaresolverr/flaresolverr:latest >/dev/null 2>&1 || warn "failed to start flaresolverr container"
  fi

  if docker ps --format '{{.Names}}' | grep -qx 'flaresolverr'; then
    ok "flaresolverr service is running on http://127.0.0.1:8191/v1"
  else
    warn "flaresolverr container is not running; check docker logs flaresolverr"
  fi
else
  warn "docker not found; flaresolverr runtime not installed"
fi

mkdir -p /usr/local/share/peass
curl -fsSL https://raw.githubusercontent.com/peass-ng/PEASS-ng/master/linPEAS/linpeas.sh -o /usr/local/share/peass/linpeas.sh || true
if [[ -f /usr/local/share/peass/linpeas.sh ]]; then
  chmod +x /usr/local/share/peass/linpeas.sh
  ln -sf /usr/local/share/peass/linpeas.sh /usr/local/bin/linpeas.sh
  ok "installed linpeas.sh"
fi
curl -fsSL https://github.com/peass-ng/PEASS-ng/releases/latest/download/winPEASx64.exe -o /usr/local/share/peass/winPEAS.exe || true
if [[ -f /usr/local/share/peass/winPEAS.exe ]]; then
  ln -sf /usr/local/share/peass/winPEAS.exe /usr/local/bin/winPEAS.exe
  ok "installed winPEAS.exe"
fi

log "Installing release binaries"
install_from_github_release "aquasecurity/trivy" "Linux-64bit\\.tar\\.gz" "trivy" || true
install_from_github_release "tenable/terrascan" "Linux_x86_64\\.tar\\.gz" "terrascan" || true
install_from_github_release "RustScan/RustScan" "amd64\\.deb|x86_64\\.deb" "rustscan" || true
install_from_github_release "io12/pwninit" "linux.*amd64|x86_64.*linux|pwninit" "pwninit" || true
install_from_github_release "sh1yo/x8" "x86_64.*(linux|musl).*(tar\\.gz|zip)" "x8" || true

log "Installing remaining enabled tool commands"
if ! have zap-cli; then
  if apt_install_if_available zaproxy; then
    cat >/usr/local/bin/zap-cli <<'EOF'
#!/usr/bin/env bash
exec zaproxy "$@"
EOF
    chmod +x /usr/local/bin/zap-cli
  elif have docker; then
    cat >/usr/local/bin/zap-cli <<'EOF'
#!/usr/bin/env bash
exec docker run --rm -it ghcr.io/zaproxy/zaproxy:stable zap.sh "$@"
EOF
    chmod +x /usr/local/bin/zap-cli
  fi
fi

if ! have pacu; then
  PACU_VENV="$WORK_DIR/pacu-venv"
  /usr/bin/python3 -m venv "$PACU_VENV" || true
  if [[ -x "$PACU_VENV/bin/pip" ]]; then
    "$PACU_VENV/bin/pip" install --upgrade pip || true
    "$PACU_VENV/bin/pip" install pacu || true
    if [[ -x "$PACU_VENV/bin/pacu" ]]; then
      ln -sf "$PACU_VENV/bin/pacu" /usr/local/bin/pacu
      ok "installed pacu in dedicated venv"
    fi
  fi
fi

if ! have hashpump; then
  if ! apt_install_if_available hashpump; then
    if [[ ! -d "$WORK_DIR/hashpumpy_changed/.git" ]]; then
      git clone --depth=1 https://github.com/2H-K/hashpumpy_changed.git "$WORK_DIR/hashpumpy_changed" || true
    fi
    if [[ -d "$WORK_DIR/hashpumpy_changed" ]]; then
      make -C "$WORK_DIR/hashpumpy_changed" || true
      if [[ -x "$WORK_DIR/hashpumpy_changed/hashpump" ]]; then
        install -m 0755 "$WORK_DIR/hashpumpy_changed/hashpump" /usr/local/bin/hashpump
      elif [[ -x "$WORK_DIR/hashpumpy_changed/HashPump" ]]; then
        install -m 0755 "$WORK_DIR/hashpumpy_changed/HashPump" /usr/local/bin/hashpump
      fi
    fi
  fi
fi

if ! have rustscan; then
  rustscan_url="$(
    curl -fsSL https://api.github.com/repos/RustScan/RustScan/releases/latest \
      | jq -r '.assets[] | select(.name == "x86_64-linux-rustscan.tar.gz.zip") | .browser_download_url' \
      | head -n1
  )"
  if [[ -n "$rustscan_url" && "$rustscan_url" != "null" ]]; then
    curl -fsSL "$rustscan_url" -o "$TMP_DIR/rustscan.tar.gz.zip" || true
    if [[ -s "$TMP_DIR/rustscan.tar.gz.zip" ]]; then
      unzip -q "$TMP_DIR/rustscan.tar.gz.zip" -d "$TMP_DIR" || true
      tar -xzf "$TMP_DIR/x86_64-linux-rustscan.tar.gz" -C "$TMP_DIR" || true
      if [[ -x "$TMP_DIR/rustscan" ]]; then
        install -m 0755 "$TMP_DIR/rustscan" /usr/local/bin/rustscan
      fi
    fi
  fi
fi

if ! have x8; then
  x8_url="$(
    curl -fsSL https://api.github.com/repos/Sh1Yo/x8/releases/latest \
      | jq -r '.assets[] | select(.name == "x86_64-linux-x8.gz") | .browser_download_url' \
      | head -n1
  )"
  if [[ -n "$x8_url" && "$x8_url" != "null" ]]; then
    curl -fsSL "$x8_url" -o "$TMP_DIR/x8.gz" || true
    if [[ -s "$TMP_DIR/x8.gz" ]]; then
      gunzip -c "$TMP_DIR/x8.gz" > "$TMP_DIR/x8" || true
      if [[ -s "$TMP_DIR/x8" ]]; then
        chmod +x "$TMP_DIR/x8"
        install -m 0755 "$TMP_DIR/x8" /usr/local/bin/x8
      fi
    fi
  fi
fi

if ! have burpsuite; then
  apt_install_if_available burpsuite || true
  if ! have burpsuite && command -v snap >/dev/null 2>&1; then
    snap install burpsuite --classic || true
  fi
  if [[ -x /snap/bin/burpsuite ]]; then
    ln -sf /snap/bin/burpsuite /usr/local/bin/burpsuite
  fi
fi

log "Installing Prowler (official installer, best effort)"
if command -v timeout >/dev/null 2>&1; then
  timeout 180s bash -lc 'curl -fsSL https://raw.githubusercontent.com/prowler-cloud/prowler/master/install.sh | bash -s -- -y' || true
else
  bash -lc 'curl -fsSL https://raw.githubusercontent.com/prowler-cloud/prowler/master/install.sh | bash -s -- -y' || true
fi
if [[ -x /root/.local/bin/prowler ]]; then
  ln -sf /root/.local/bin/prowler /usr/local/bin/prowler
fi

log "Installing Ghidra (best effort)"
if ! have analyzeHeadless; then
  ghidra_asset="$(
    curl -fsSL https://api.github.com/repos/NationalSecurityAgency/ghidra/releases/latest \
      | jq -r '.assets[] | select(.name | test("ghidra_.*_PUBLIC_.*\\.zip")) | .browser_download_url' \
      | head -n1
  )"
  if [[ -n "$ghidra_asset" && "$ghidra_asset" != "null" ]]; then
    mkdir -p /opt/ghidra
    curl -fsSL "$ghidra_asset" -o "$TMP_DIR/ghidra.zip"
    unzip -q "$TMP_DIR/ghidra.zip" -d /opt/ghidra
    ghidra_dir="$(find /opt/ghidra -maxdepth 1 -type d -name 'ghidra_*' | head -n1 || true)"
    if [[ -n "$ghidra_dir" && -x "$ghidra_dir/support/analyzeHeadless" ]]; then
      ln -sf "$ghidra_dir/support/analyzeHeadless" /usr/local/bin/analyzeHeadless
      ok "installed analyzeHeadless"
    fi
  else
    warn "unable to locate ghidra release asset"
  fi
fi

log "Best-effort manual/interactive tools not auto-installed:"
echo "  - burpsuite (GUI/manual download)"
echo "  - cyberchef (web app/UI)"
echo "  - falco (vendor repo/kernel modules)"
echo "  - msfvenom (metasploit framework repo)"
echo "  - scout (depends on ScoutSuite workflow; command may differ by release)"
echo "  - hashpump (source build may vary by distro)"

log "Verification (enabled tools from tools/*.yaml)"
"$ROOT_DIR/scripts/verify-enabled-tools.sh" || true
