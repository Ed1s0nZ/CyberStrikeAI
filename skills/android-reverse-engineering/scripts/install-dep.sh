#!/usr/bin/env bash
# install-dep.sh - Install a single dependency for Android reverse engineering
# Usage: install-dep.sh <dependency>
# Dependencies: java, jadx, vineflower, dex2jar, apktool, adb
#
# Exit codes:
#   0 - installed successfully
#   1 - installation failed
#   2 - requires manual action (e.g. sudo needed but not available)
set -euo pipefail

usage() {
  cat <<EOF
Usage: install-dep.sh <dependency>

Install a dependency required for Android reverse engineering.

Available dependencies:
  java         Java JDK 17+
  jadx         jadx decompiler
  vineflower   Vineflower (Fernflower fork) decompiler
  dex2jar      DEX to JAR converter
  apktool      Android resource decoder
  adb          Android Debug Bridge

The script detects your OS and package manager, then:
  - Installs directly if possible (brew, or user-local install)
  - Uses sudo if available and needed
  - Prints manual instructions if neither option works
EOF
  exit 0
}

if [[ $# -lt 1 || "$1" == "-h" || "$1" == "--help" ]]; then
  usage
fi

DEP="$1"

# --- Detect environment ---
OS="unknown"
PKG_MANAGER="none"
HAS_SUDO=false
ARCH=$(uname -m)

case "$(uname -s)" in
  Linux)  OS="linux" ;;
  Darwin) OS="macos" ;;
esac

# Detect package manager
if command -v brew &>/dev/null; then
  PKG_MANAGER="brew"
elif command -v apt-get &>/dev/null; then
  PKG_MANAGER="apt"
elif command -v dnf &>/dev/null; then
  PKG_MANAGER="dnf"
elif command -v pacman &>/dev/null; then
  PKG_MANAGER="pacman"
fi

# Check sudo availability
if command -v sudo &>/dev/null; then
  if sudo -n true 2>/dev/null; then
    HAS_SUDO=true
  else
    # sudo exists but may need password - we'll try it and let it prompt
    HAS_SUDO=true
  fi
fi

info()  { echo "[INFO] $*"; }
ok()    { echo "[OK] $*"; }
fail()  { echo "[FAIL] $*" >&2; }
manual() {
  echo "[MANUAL] $*" >&2
  echo "         Cannot install automatically. Please install manually and retry." >&2
  exit 2
}

# --- Helper: install via system package manager (needs sudo on Linux) ---
pkg_install() {
  local pkg="$1"
  case "$PKG_MANAGER" in
    brew)
      info "Installing $pkg via Homebrew..."
      brew install "$pkg"
      ;;
    apt)
      if [[ "$HAS_SUDO" == true ]]; then
        info "Installing $pkg via apt..."
        sudo apt-get update -qq && sudo apt-get install -y -qq "$pkg"
      else
        manual "Run: sudo apt-get install $pkg"
      fi
      ;;
    dnf)
      if [[ "$HAS_SUDO" == true ]]; then
        info "Installing $pkg via dnf..."
        sudo dnf install -y "$pkg"
      else
        manual "Run: sudo dnf install $pkg"
      fi
      ;;
    pacman)
      if [[ "$HAS_SUDO" == true ]]; then
        info "Installing $pkg via pacman..."
        sudo pacman -S --noconfirm "$pkg"
      else
        manual "Run: sudo pacman -S $pkg"
      fi
      ;;
    *)
      manual "No supported package manager found. Install $pkg manually."
      ;;
  esac
}

# --- Helper: download a file ---
download() {
  local url="$1" dest="$2"
  if command -v curl &>/dev/null; then
    curl -fsSL -o "$dest" "$url"
  elif command -v wget &>/dev/null; then
    wget -q -O "$dest" "$url"
  else
    fail "Neither curl nor wget available."
    return 1
  fi
}

# --- Helper: get latest GitHub release tag ---
gh_latest_tag() {
  local repo="$1"
  local url="https://api.github.com/repos/$repo/releases/latest"
  if command -v curl &>/dev/null; then
    curl -fsSL "$url" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/'
  elif command -v wget &>/dev/null; then
    wget -q -O - "$url" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/'
  fi
}

# --- Helper: add a line to shell profile if not already present ---
add_to_profile() {
  local line="$1"
  local profile=""
  if [[ -f "$HOME/.zshrc" ]]; then
    profile="$HOME/.zshrc"
  elif [[ -f "$HOME/.bashrc" ]]; then
    profile="$HOME/.bashrc"
  elif [[ -f "$HOME/.profile" ]]; then
    profile="$HOME/.profile"
  fi

  if [[ -n "$profile" ]]; then
    if ! grep -qF "$line" "$profile" 2>/dev/null; then
      echo "$line" >> "$profile"
      info "Added to $profile: $line"
      info "Run 'source $profile' or start a new shell to apply."
    fi
  else
    info "Add this to your shell profile: $line"
  fi
}

# =====================================================================
# Dependency installers
# =====================================================================

install_java() {
  if command -v java &>/dev/null; then
    local ver
    ver=$(java -version 2>&1 | head -1 | sed -n 's/.*"\([0-9]*\)\..*/\1/p')
    if [[ -n "$ver" ]] && (( ver >= 17 )); then
      ok "Java $ver already installed"
      return 0
    fi
  fi

  info "Installing Java JDK 17+..."
  case "$PKG_MANAGER" in
    brew)    brew install openjdk@17 ;;
    apt)     pkg_install "openjdk-17-jdk" ;;
    dnf)     pkg_install "java-17-openjdk-devel" ;;
    pacman)  pkg_install "jdk17-openjdk" ;;
    *)       manual "Install Java JDK 17+ from https://adoptium.net/" ;;
  esac

  # Verify
  if command -v java &>/dev/null; then
    ok "Java installed: $(java -version 2>&1 | head -1)"
  else
    fail "Java installation may require PATH update."
    if [[ "$PKG_MANAGER" == "brew" ]]; then
      add_to_profile 'export PATH="/opt/homebrew/opt/openjdk@17/bin:$PATH"'
    fi
    exit 1
  fi
}

install_jadx() {
  if command -v jadx &>/dev/null; then
    ok "jadx already installed: $(jadx --version 2>/dev/null || echo 'unknown')"
    return 0
  fi

  # Try brew first (cleanest)
  if [[ "$PKG_MANAGER" == "brew" ]]; then
    info "Installing jadx via Homebrew..."
    brew install jadx
    ok "jadx installed via Homebrew"
    return 0
  fi

  # User-local install from GitHub releases (no sudo needed)
  info "Installing jadx from GitHub releases..."
  local tag
  tag=$(gh_latest_tag "skylot/jadx")
  if [[ -z "$tag" ]]; then
    fail "Could not determine latest jadx version."
    manual "Download from https://github.com/skylot/jadx/releases/latest"
  fi

  local version="${tag#v}"
  local url="https://github.com/skylot/jadx/releases/download/${tag}/jadx-${version}.zip"
  local tmp_zip
  tmp_zip=$(mktemp /tmp/jadx-XXXXXX.zip)

  info "Downloading jadx $version..."
  download "$url" "$tmp_zip"

  local install_dir="$HOME/.local/share/jadx"
  rm -rf "$install_dir"
  mkdir -p "$install_dir"
  unzip -qo "$tmp_zip" -d "$install_dir"
  rm -f "$tmp_zip"
  chmod +x "$install_dir/bin/jadx" "$install_dir/bin/jadx-gui" 2>/dev/null || true

  # Add to PATH
  mkdir -p "$HOME/.local/bin"
  ln -sf "$install_dir/bin/jadx" "$HOME/.local/bin/jadx"
  ln -sf "$install_dir/bin/jadx-gui" "$HOME/.local/bin/jadx-gui"
  export PATH="$HOME/.local/bin:$PATH"
  add_to_profile 'export PATH="$HOME/.local/bin:$PATH"'

  if command -v jadx &>/dev/null; then
    ok "jadx $version installed to $install_dir"
  else
    ok "jadx $version installed to $install_dir"
    info "Run: export PATH=\"\$HOME/.local/bin:\$PATH\" to use it now"
  fi
}

install_vineflower() {
  # Check if already available
  if command -v vineflower &>/dev/null || command -v fernflower &>/dev/null; then
    ok "Vineflower/Fernflower CLI already installed"
    return 0
  fi
  for candidate in \
    "${FERNFLOWER_JAR_PATH:-}" \
    "$HOME/vineflower/vineflower.jar" \
    "$HOME/fernflower/fernflower.jar" \
    "$HOME/fernflower/build/libs/fernflower.jar" \
    "$HOME/vineflower/build/libs/vineflower.jar"; do
    if [[ -n "$candidate" ]] && [[ -f "$candidate" ]]; then
      ok "Vineflower/Fernflower JAR already exists: $candidate"
      return 0
    fi
  done

  # Try brew
  if [[ "$PKG_MANAGER" == "brew" ]]; then
    info "Installing vineflower via Homebrew..."
    if brew install vineflower 2>/dev/null; then
      ok "Vineflower installed via Homebrew"
      return 0
    fi
    info "Homebrew formula not available, falling back to direct download."
  fi

  # Download JAR from GitHub releases (no sudo needed)
  info "Installing Vineflower from GitHub releases..."
  local tag
  tag=$(gh_latest_tag "Vineflower/vineflower")
  if [[ -z "$tag" ]]; then
    fail "Could not determine latest Vineflower version."
    manual "Download from https://github.com/Vineflower/vineflower/releases/latest"
  fi

  local version="${tag#v}"
  local url="https://github.com/Vineflower/vineflower/releases/download/${tag}/vineflower-${version}.jar"
  local install_dir="$HOME/.local/share/vineflower"
  mkdir -p "$install_dir"

  info "Downloading Vineflower $version..."
  download "$url" "$install_dir/vineflower.jar"

  # Create wrapper script
  mkdir -p "$HOME/.local/bin"
  cat > "$HOME/.local/bin/vineflower" <<'WRAPPER'
#!/usr/bin/env bash
exec java -jar "$HOME/.local/share/vineflower/vineflower.jar" "$@"
WRAPPER
  chmod +x "$HOME/.local/bin/vineflower"

  export PATH="$HOME/.local/bin:$PATH"
  export FERNFLOWER_JAR_PATH="$install_dir/vineflower.jar"
  add_to_profile 'export PATH="$HOME/.local/bin:$PATH"'
  add_to_profile "export FERNFLOWER_JAR_PATH=\"$install_dir/vineflower.jar\""

  ok "Vineflower $version installed to $install_dir/vineflower.jar"
  info "FERNFLOWER_JAR_PATH set to $install_dir/vineflower.jar"
}

install_dex2jar() {
  if command -v d2j-dex2jar &>/dev/null || command -v d2j-dex2jar.sh &>/dev/null; then
    ok "dex2jar already installed"
    return 0
  fi

  # Try brew
  if [[ "$PKG_MANAGER" == "brew" ]]; then
    info "Installing dex2jar via Homebrew..."
    if brew install dex2jar 2>/dev/null; then
      ok "dex2jar installed via Homebrew"
      return 0
    fi
    info "Homebrew formula not available, falling back to direct download."
  fi

  # Download from GitHub (no sudo needed)
  info "Installing dex2jar from GitHub releases..."
  local tag
  tag=$(gh_latest_tag "pxb1988/dex2jar")
  if [[ -z "$tag" ]]; then
    # Fallback: pxb1988 hasn't released in a while, try known version
    tag="v2.4"
  fi

  local version="${tag#v}"
  local url="https://github.com/pxb1988/dex2jar/releases/download/${tag}/dex-tools-${version}.zip"
  local tmp_zip
  tmp_zip=$(mktemp /tmp/dex2jar-XXXXXX.zip)

  info "Downloading dex2jar $version..."
  if ! download "$url" "$tmp_zip"; then
    # Try alternate naming
    url="https://github.com/pxb1988/dex2jar/releases/download/${tag}/dex-tools-v${version}.zip"
    download "$url" "$tmp_zip" || {
      fail "Download failed."
      manual "Download from https://github.com/pxb1988/dex2jar/releases/latest"
    }
  fi

  local install_dir="$HOME/.local/share/dex2jar"
  rm -rf "$install_dir"
  mkdir -p "$install_dir"
  unzip -qo "$tmp_zip" -d "$install_dir"
  rm -f "$tmp_zip"

  # The zip may contain a top-level directory - find the actual bin location
  local bin_dir=""
  if [[ -f "$install_dir/d2j-dex2jar.sh" ]]; then
    bin_dir="$install_dir"
  else
    bin_dir=$(find "$install_dir" -name "d2j-dex2jar.sh" -exec dirname {} \; | head -1)
  fi

  if [[ -z "$bin_dir" ]]; then
    fail "Could not find d2j-dex2jar.sh in extracted archive."
    manual "Download and extract manually from https://github.com/pxb1988/dex2jar/releases"
  fi

  chmod +x "$bin_dir"/*.sh 2>/dev/null || true

  mkdir -p "$HOME/.local/bin"
  for script in "$bin_dir"/d2j-*.sh; do
    local name
    name=$(basename "$script" .sh)
    ln -sf "$script" "$HOME/.local/bin/$name"
  done

  export PATH="$HOME/.local/bin:$PATH"
  add_to_profile 'export PATH="$HOME/.local/bin:$PATH"'

  ok "dex2jar $version installed to $install_dir"
}

install_apktool() {
  if command -v apktool &>/dev/null; then
    ok "apktool already installed"
    return 0
  fi

  case "$PKG_MANAGER" in
    brew)    info "Installing apktool via Homebrew..."; brew install apktool ;;
    apt)     pkg_install "apktool" ;;
    *)       manual "Install apktool from https://apktool.org/docs/install" ;;
  esac

  if command -v apktool &>/dev/null; then
    ok "apktool installed"
  else
    fail "apktool installation may have failed."
    exit 1
  fi
}

install_adb() {
  if command -v adb &>/dev/null; then
    ok "adb already installed"
    return 0
  fi

  case "$PKG_MANAGER" in
    brew)    info "Installing adb via Homebrew..."; brew install android-platform-tools ;;
    apt)     pkg_install "adb" ;;
    dnf)     pkg_install "android-tools" ;;
    pacman)  pkg_install "android-tools" ;;
    *)       manual "Install Android SDK Platform Tools from https://developer.android.com/tools/releases/platform-tools" ;;
  esac

  if command -v adb &>/dev/null; then
    ok "adb installed"
  else
    fail "adb installation may have failed."
    exit 1
  fi
}

# =====================================================================
# Dispatch
# =====================================================================

case "$DEP" in
  java)        install_java ;;
  jadx)        install_jadx ;;
  vineflower|fernflower)  install_vineflower ;;
  dex2jar)     install_dex2jar ;;
  apktool)     install_apktool ;;
  adb)         install_adb ;;
  *)
    echo "Error: Unknown dependency '$DEP'" >&2
    echo "Available: java, jadx, vineflower, dex2jar, apktool, adb" >&2
    exit 1
    ;;
esac
