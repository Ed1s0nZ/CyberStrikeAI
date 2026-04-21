#!/usr/bin/env bash
# ============================================================================
# CyberStrikeAI - Cuttlefish (AOSP) Virtual Android Device Setup
# ============================================================================
# Installs Cuttlefish host packages, downloads AOSP images, configures
# the virtual device as a Russian-locale phone with full ADB debug access.
# ============================================================================
set -euo pipefail

CVD_HOME="${CVD_HOME:-$HOME/cuttlefish-workspace}"
CVD_BRANCH="${CVD_BRANCH:-aosp-main}"
CVD_TARGET="${CVD_TARGET:-aosp_cf_x86_64_phone-trunk_staging-userdebug}"
CVD_MEMORY="${CVD_MEMORY:-8192}"
CVD_CPUS="${CVD_CPUS:-4}"
CVD_DISK_MB="${CVD_DISK_MB:-16000}"
PORTAL_VERSION="${PORTAL_VERSION:-latest}"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
info()  { echo -e "${GREEN}[+]${NC} $*"; }
warn()  { echo -e "${YELLOW}[!]${NC} $*"; }
error() { echo -e "${RED}[-]${NC} $*"; exit 1; }

# ── Step 0: Pre-flight ───────────────────────────────────────────────────────
info "Pre-flight checks..."
[[ -e /dev/kvm ]] || error "KVM not available. Enable VT-x/AMD-V in BIOS."
[[ $(uname -m) == "x86_64" ]] || error "x86_64 required for Cuttlefish x86 images."

# ── Step 1: Install host dependencies ────────────────────────────────────────
info "Installing host dependencies..."
sudo apt-get update -qq
sudo apt-get install -y -qq \
    git devscripts equivs config-package-dev debhelper-compat golang \
    curl wget unzip bridge-utils libarchive-tools libfdt1 libfdt-dev \
    python3 python3-pip iptables adb qemu-system-x86 qemu-utils \
    libvirt-daemon-system libvirt-clients cloud-image-utils \
    android-sdk-platform-tools 2>/dev/null || true

# ── Step 2: Build and install cuttlefish host packages ───────────────────────
info "Building Cuttlefish host packages..."
CFISH_BUILD_DIR=$(mktemp -d)
pushd "$CFISH_BUILD_DIR" > /dev/null

if [[ ! -d android-cuttlefish ]]; then
    git clone https://github.com/google/android-cuttlefish.git
fi
cd android-cuttlefish

# Build .deb packages
tools/buildutils/build_packages.sh

# Install packages
sudo dpkg -i ./cuttlefish-base_*.deb ./cuttlefish-user_*.deb 2>/dev/null || true
sudo apt-get install -f -y -qq

popd > /dev/null
rm -rf "$CFISH_BUILD_DIR"

# ── Step 3: User groups ─────────────────────────────────────────────────────
info "Configuring user groups..."
for grp in kvm cvdnetwork render; do
    if getent group "$grp" > /dev/null 2>&1; then
        sudo usermod -aG "$grp" "$USER" 2>/dev/null || true
    fi
done

# ── Step 4: Download AOSP images ────────────────────────────────────────────
info "Setting up workspace at $CVD_HOME..."
mkdir -p "$CVD_HOME"
cd "$CVD_HOME"

if [[ ! -f "$CVD_HOME/bin/launch_cvd" ]]; then
    info "Downloading Cuttlefish host package and AOSP images..."
    info "Using Android CI artifacts..."

    # Try fetching with the cvd tool first if available
    if command -v cvd &>/dev/null; then
        cvd fetch --default_build="${CVD_BRANCH}/${CVD_TARGET}" \
            --target_directory="$CVD_HOME" || true
    fi

    # If cvd fetch didn't work, download manually
    if [[ ! -f "$CVD_HOME/bin/launch_cvd" ]]; then
        warn "Automatic fetch unavailable. Downloading latest nightly..."
        # Download from Android CI - using the fetch_artifacts approach
        FETCH_URL="https://androidbuildinternal.googleapis.com/android/internal/build/v3/builds"

        # Alternative: use the prebuilt fetch script
        if [[ ! -f "$CVD_HOME/fetch_cvd" ]]; then
            # Download the fetch_cvd binary
            FETCH_CVD_URL="https://github.com/google/android-cuttlefish/releases/latest/download/fetch_cvd"
            curl -fSL -o "$CVD_HOME/fetch_cvd" "$FETCH_CVD_URL" 2>/dev/null || {
                warn "Could not download fetch_cvd from GitHub releases."
                warn "Trying alternative approach..."
                # Build fetch_cvd from source
                pushd "$CVD_HOME" > /dev/null
                if [[ ! -d android-cuttlefish ]]; then
                    git clone https://github.com/google/android-cuttlefish.git
                fi
                cd android-cuttlefish
                # The fetch_cvd is part of the host tools
                popd > /dev/null
            }
            chmod +x "$CVD_HOME/fetch_cvd" 2>/dev/null || true
        fi

        if [[ -x "$CVD_HOME/fetch_cvd" ]]; then
            cd "$CVD_HOME"
            ./fetch_cvd --default_build="${CVD_BRANCH}/${CVD_TARGET}" \
                --target_directory="$CVD_HOME" || {
                warn "fetch_cvd failed. You may need to download images manually."
                warn "Go to https://ci.android.com/ → ${CVD_BRANCH} → ${CVD_TARGET}"
                warn "Download: aosp_cf_x86_64_phone-img-*.zip and cvd-host_package.tar.gz"
                warn "Extract both into: $CVD_HOME"
            }
        else
            info "Manual download instructions:"
            echo "  1. Visit https://ci.android.com/"
            echo "  2. Select branch: ${CVD_BRANCH}"
            echo "  3. Select target: ${CVD_TARGET}"
            echo "  4. Download: aosp_cf_x86_64_phone-img-*.zip"
            echo "  5. Download: cvd-host_package.tar.gz"
            echo "  6. Extract both into: $CVD_HOME"
            echo "     cd $CVD_HOME"
            echo "     tar xf cvd-host_package.tar.gz"
            echo "     unzip aosp_cf_x86_64_phone-img-*.zip"
        fi
    fi
else
    info "Cuttlefish workspace already exists at $CVD_HOME"
fi

# ── Step 5: Create Russian locale configuration ─────────────────────────────
info "Creating Russian phone configuration..."
mkdir -p "$CVD_HOME/config"

cat > "$CVD_HOME/config/russian_phone.json" << 'RUCFG'
{
    "comment": "CyberStrikeAI - Russian-configured Android device",
    "instances": [
        {
            "vm": {
                "memory_mb": 8192,
                "cpus": 4,
                "setupwizard_mode": "DISABLED"
            },
            "graphics": {
                "displays": [
                    {
                        "width": 1080,
                        "height": 2400,
                        "dpi": 420,
                        "refresh_rate_hz": 60
                    }
                ]
            }
        }
    ]
}
RUCFG

# ADB setup script for Russian locale/identity (runs on first boot)
cat > "$CVD_HOME/config/setup_russian_identity.sh" << 'RUSHID'
#!/usr/bin/env bash
# ============================================================================
# Configure Cuttlefish as Russian-owned phone
# Run AFTER device boots: ./config/setup_russian_identity.sh [SERIAL]
# ============================================================================
set -euo pipefail

ADB="${ADB:-adb}"
SERIAL="${1:-}"
ADB_CMD="$ADB"
[[ -n "$SERIAL" ]] && ADB_CMD="$ADB -s $SERIAL"

wait_for_device() {
    echo "[+] Waiting for device..."
    $ADB_CMD wait-for-device
    # Wait for boot to complete
    for i in $(seq 1 120); do
        BOOT=$($ADB_CMD shell getprop sys.boot_completed 2>/dev/null | tr -d '\r' || true)
        [[ "$BOOT" == "1" ]] && return 0
        sleep 2
    done
    echo "[-] Boot timeout" && return 1
}

wait_for_device

echo "[+] Configuring Russian locale and identity..."

# ── Locale & Language ────────────────────────────────────────────────────
$ADB_CMD shell settings put system system_locales ru-RU
$ADB_CMD shell setprop persist.sys.locale ru-RU
$ADB_CMD shell setprop persist.sys.language ru
$ADB_CMD shell setprop persist.sys.country RU

# ── Timezone (Moscow) ────────────────────────────────────────────────────
$ADB_CMD shell settings put global auto_time_zone 0
$ADB_CMD shell setprop persist.sys.timezone Europe/Moscow
$ADB_CMD shell service call alarm 3 s16 Europe/Moscow

# ── Keyboard / Input ─────────────────────────────────────────────────────
$ADB_CMD shell settings put secure default_input_method com.android.inputmethod.latin/.LatinIME
$ADB_CMD shell ime enable com.android.inputmethod.latin/.LatinIME

# ── SIM / Carrier Identity (MCC/MNC for MTS Russia: 250/01) ─────────────
$ADB_CMD shell setprop gsm.sim.operator.numeric 25001
$ADB_CMD shell setprop gsm.sim.operator.alpha "MTS"
$ADB_CMD shell setprop gsm.sim.operator.iso-country ru
$ADB_CMD shell setprop gsm.operator.numeric 25001
$ADB_CMD shell setprop gsm.operator.alpha "MTS"
$ADB_CMD shell setprop gsm.operator.iso-country ru
# Alternative carriers:
# Beeline:    25099
# MegaFon:    25002
# Tele2:      25020
# Yota:       25011
# Rostelecom: 25039

# ── Device Identity Spoofing ────────────────────────────────────────────
$ADB_CMD shell setprop ro.product.manufacturer "Xiaomi"
$ADB_CMD shell setprop ro.product.model "Redmi Note 12 Pro"
$ADB_CMD shell setprop ro.product.brand "Redmi"
$ADB_CMD shell setprop ro.product.device "ruby"
$ADB_CMD shell setprop ro.product.name "ruby"
$ADB_CMD shell setprop ro.build.display.id "V14.0.6.0.TMOMIXM"
# Alternative device identities:
# Samsung Galaxy A54: manufacturer=samsung, model=SM-A546B, brand=samsung
# Huawei P40 Lite:    manufacturer=HUAWEI, model=JNY-LX1, brand=HUAWEI
# HONOR 90:           manufacturer=HONOR, model=REA-NX9, brand=HONOR

# ── WiFi Country Code ───────────────────────────────────────────────────
$ADB_CMD shell setprop wifi.country_code RU

# ── Date/Time Format (24h, dd.MM.yyyy - Russian standard) ───────────────
$ADB_CMD shell settings put system time_12_24 24
$ADB_CMD shell settings put system date_format dd.MM.yyyy

# ── NTP Server (Russian) ────────────────────────────────────────────────
$ADB_CMD shell settings put global ntp_server ntp1.stratum2.ru

# ── DNS (Yandex DNS) ────────────────────────────────────────────────────
$ADB_CMD shell setprop net.dns1 77.88.8.8
$ADB_CMD shell setprop net.dns2 77.88.8.1

# ── Developer Options & Debug ────────────────────────────────────────────
$ADB_CMD shell settings put global development_settings_enabled 1
$ADB_CMD shell settings put global adb_enabled 1
$ADB_CMD shell settings put global stay_on_while_plugged_in 3
$ADB_CMD shell settings put secure mock_location 1
$ADB_CMD shell settings put global package_verifier_enable 0
$ADB_CMD shell settings put global verifier_verify_adb_installs 0

# ── Disable animations (faster for automation) ──────────────────────────
$ADB_CMD shell settings put global window_animation_scale 0.0
$ADB_CMD shell settings put global transition_animation_scale 0.0
$ADB_CMD shell settings put global animator_duration_scale 0.0

# ── Install common Russian apps certificates/trust ──────────────────────
# Allow installation from unknown sources
$ADB_CMD shell settings put secure install_non_market_apps 1

echo "[+] Russian phone identity configured."
echo "    Locale:   ru-RU"
echo "    Timezone: Europe/Moscow"
echo "    Carrier:  MTS (250/01)"
echo "    Device:   Xiaomi Redmi Note 12 Pro"
echo "    DNS:      Yandex (77.88.8.8)"
echo ""
echo "[*] Some properties require reboot to take full effect."
echo "    Reboot with: $ADB_CMD reboot"
RUSHID
chmod +x "$CVD_HOME/config/setup_russian_identity.sh"

# ── Step 6: Create launch/control scripts ────────────────────────────────────
info "Creating control scripts..."

# --- cvd-launch.sh ---
cat > "$CVD_HOME/cvd-launch.sh" << 'LAUNCH'
#!/usr/bin/env bash
# Launch Cuttlefish with Russian config
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

MEMORY="${CVD_MEMORY:-8192}"
CPUS="${CVD_CPUS:-4}"
DISK="${CVD_DISK_MB:-16000}"
GPU="${CVD_GPU:-guest_swiftshader}"

echo "[+] Launching Cuttlefish (${CPUS} CPUs, ${MEMORY}MB RAM, ${GPU} GPU)..."

WEBRTC_PORT="${CVD_WEBRTC_PORT:-8443}"

LAUNCH_ARGS=(
    -daemon
    --memory_mb="$MEMORY"
    --cpus="$CPUS"
    --blank_data_image_mb="$DISK"
    --gpu_mode="$GPU"
    --start_webrtc=true
    --webrtc_public_ip=localhost
    --setupwizard_mode=DISABLED
    --report_anonymous_usage_stats=n
    --use_overlay=true
    --noresume
)

# Add data_policy on first run or if requested
if [[ "${FRESH:-0}" == "1" ]] || [[ ! -d "$SCRIPT_DIR/cuttlefish_runtime.1" ]]; then
    LAUNCH_ARGS+=(--data_policy=always_create)
fi

./bin/launch_cvd "${LAUNCH_ARGS[@]}"

echo "[+] Waiting for boot..."
./bin/adb wait-for-device
for i in $(seq 1 180); do
    BOOT=$(./bin/adb shell getprop sys.boot_completed 2>/dev/null | tr -d '\r' || true)
    if [[ "$BOOT" == "1" ]]; then
        echo "[+] Device booted successfully!"
        # Apply Russian identity
        ADB="$SCRIPT_DIR/bin/adb" "$SCRIPT_DIR/config/setup_russian_identity.sh"
        echo ""
        echo "[+] Cuttlefish is ready."
        echo "    ADB:    $SCRIPT_DIR/bin/adb shell"
        WEBRTC_PORT="${CVD_WEBRTC_PORT:-8443}"
        WEBRTC_URL="https://localhost:${WEBRTC_PORT}"
        echo "    WebRTC: $WEBRTC_URL"
        echo "    Serial: $(./bin/adb devices | grep -v List | awk '{print $1}' | head -1)"

        # Auto-open WebRTC viewer in browser for visual interaction
        for opener in xdg-open sensible-browser google-chrome firefox chromium-browser; do
            if command -v "$opener" &>/dev/null; then
                "$opener" "$WEBRTC_URL" &>/dev/null &
                echo "[+] Opened WebRTC viewer in browser ($opener)"
                break
            fi
        done
        exit 0
    fi
    sleep 2
done
echo "[-] Boot timeout after 6 minutes."
exit 1
LAUNCH
chmod +x "$CVD_HOME/cvd-launch.sh"

# --- cvd-stop.sh ---
cat > "$CVD_HOME/cvd-stop.sh" << 'STOP'
#!/usr/bin/env bash
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"
echo "[+] Stopping Cuttlefish..."
./bin/stop_cvd 2>/dev/null || pkill -f launch_cvd 2>/dev/null || true
echo "[+] Stopped."
STOP
chmod +x "$CVD_HOME/cvd-stop.sh"

# --- cvd-install-apk.sh ---
cat > "$CVD_HOME/cvd-install-apk.sh" << 'INSTALLAPK'
#!/usr/bin/env bash
# Install APK with optional debug/downgrade flags
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ADB="$SCRIPT_DIR/bin/adb"
APK="${1:?Usage: cvd-install-apk.sh <apk_path> [--debug] [--downgrade] [--replace]}"
shift

FLAGS=()
for arg in "$@"; do
    case "$arg" in
        --debug)     FLAGS+=(-t) ;;
        --downgrade) FLAGS+=(-d) ;;
        --replace)   FLAGS+=(-r) ;;
        *)           FLAGS+=("$arg") ;;
    esac
done

echo "[+] Installing: $APK"
$ADB install "${FLAGS[@]}" "$APK"
echo "[+] Installed."

# Show package info
PKG=$($ADB shell pm list packages -f 2>/dev/null | grep "$(basename "$APK" .apk)" | tail -1 | sed 's/.*=//' || true)
if [[ -n "$PKG" ]]; then
    echo "[+] Package: $PKG"
    $ADB shell dumpsys package "$PKG" | head -20
fi
INSTALLAPK
chmod +x "$CVD_HOME/cvd-install-apk.sh"

# --- cvd-hotswap.sh ---
cat > "$CVD_HOME/cvd-hotswap.sh" << 'HOTSWAP'
#!/usr/bin/env bash
# Hot-swap install: force-stop app, reinstall APK, restart
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ADB="$SCRIPT_DIR/bin/adb"
APK="${1:?Usage: cvd-hotswap.sh <apk_path> [package_name]}"
PACKAGE="${2:-}"

# Try to determine package name from the APK
if [[ -z "$PACKAGE" ]]; then
    PACKAGE=$(aapt dump badging "$APK" 2>/dev/null | grep "package: name=" | sed "s/.*name='\([^']*\)'.*/\1/" || true)
fi

echo "[+] Hot-swap: $APK"

if [[ -n "$PACKAGE" ]]; then
    echo "[+] Stopping: $PACKAGE"
    $ADB shell am force-stop "$PACKAGE" 2>/dev/null || true
fi

echo "[+] Reinstalling..."
$ADB install -r -d -t "$APK"

if [[ -n "$PACKAGE" ]]; then
    echo "[+] Launching: $PACKAGE"
    LAUNCHER=$($ADB shell cmd package resolve-activity --brief "$PACKAGE" 2>/dev/null | tail -1 || true)
    if [[ -n "$LAUNCHER" && "$LAUNCHER" != *"No activity"* ]]; then
        $ADB shell am start -n "$LAUNCHER" 2>/dev/null || true
    fi
fi

echo "[+] Hot-swap complete."
HOTSWAP
chmod +x "$CVD_HOME/cvd-hotswap.sh"

# --- cvd-snapshot.sh ---
cat > "$CVD_HOME/cvd-snapshot.sh" << 'SNAPSHOT'
#!/usr/bin/env bash
# Snapshot management: save/restore device state
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SNAPDIR="$SCRIPT_DIR/snapshots"
mkdir -p "$SNAPDIR"
ACTION="${1:?Usage: cvd-snapshot.sh <save|restore|list> [name]}"
NAME="${2:-default}"

case "$ACTION" in
    save)
        echo "[+] Saving snapshot: $NAME"
        mkdir -p "$SNAPDIR/$NAME"
        cp -a "$SCRIPT_DIR/cuttlefish_runtime.1/img/"*.img "$SNAPDIR/$NAME/" 2>/dev/null || true
        echo "$(date -Iseconds)" > "$SNAPDIR/$NAME/.timestamp"
        echo "[+] Snapshot saved."
        ;;
    restore)
        [[ -d "$SNAPDIR/$NAME" ]] || { echo "[-] Snapshot not found: $NAME"; exit 1; }
        echo "[+] Restoring snapshot: $NAME"
        "$SCRIPT_DIR/cvd-stop.sh" 2>/dev/null || true
        cp -a "$SNAPDIR/$NAME/"*.img "$SCRIPT_DIR/cuttlefish_runtime.1/img/" 2>/dev/null || true
        echo "[+] Snapshot restored. Launch device to continue."
        ;;
    list)
        echo "Snapshots:"
        for d in "$SNAPDIR"/*/; do
            [[ -d "$d" ]] || continue
            N=$(basename "$d")
            TS=$(cat "$d/.timestamp" 2>/dev/null || echo "unknown")
            SIZE=$(du -sh "$d" 2>/dev/null | cut -f1)
            echo "  - $N  ($TS, $SIZE)"
        done
        ;;
    *) echo "Usage: cvd-snapshot.sh <save|restore|list> [name]" ;;
esac
SNAPSHOT
chmod +x "$CVD_HOME/cvd-snapshot.sh"

# --- cvd-api.sh --- (Guest system API wrapper)
cat > "$CVD_HOME/cvd-api.sh" << 'APICTL'
#!/usr/bin/env bash
# Full guest system API control
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ADB="$SCRIPT_DIR/bin/adb"
CMD="${1:?Usage: cvd-api.sh <command> [args...]}"
shift

case "$CMD" in
    # ── App Management ────────────────────────────────────────────────
    install)       $ADB install -r -t "$@" ;;
    uninstall)     $ADB uninstall "$@" ;;
    list-packages) $ADB shell pm list packages "$@" ;;
    clear-data)    $ADB shell pm clear "$@" ;;
    force-stop)    $ADB shell am force-stop "$@" ;;
    start-app)     $ADB shell monkey -p "$1" -c android.intent.category.LAUNCHER 1 ;;
    start-activity) $ADB shell am start -n "$@" ;;
    broadcast)     $ADB shell am broadcast "$@" ;;

    # ── File Operations ───────────────────────────────────────────────
    push)          $ADB push "$@" ;;
    pull)          $ADB pull "$@" ;;
    shell)         $ADB shell "$@" ;;

    # ── System ────────────────────────────────────────────────────────
    reboot)        $ADB reboot "$@" ;;
    logcat)        $ADB logcat "$@" ;;
    screenshot)    $ADB exec-out screencap -p > "${1:-screenshot.png}" ;;
    screenrecord)  $ADB shell screenrecord "$@" ;;
    props)         $ADB shell getprop "$@" ;;
    setprop)       $ADB shell setprop "$@" ;;
    settings)      $ADB shell settings "$@" ;;
    dumpsys)       $ADB shell dumpsys "$@" ;;

    # ── Network ───────────────────────────────────────────────────────
    port-forward)  $ADB forward "$@" ;;
    reverse)       $ADB reverse "$@" ;;
    tcpdump)       $ADB shell tcpdump "$@" ;;

    # ── Debug ─────────────────────────────────────────────────────────
    bugreport)     $ADB bugreport "$@" ;;
    jdwp)          $ADB jdwp ;;
    attach-debugger)
        PKG="${1:?Package name required}"
        echo "[+] Enabling debug for $PKG..."
        $ADB shell am set-debug-app -w "$PKG"
        echo "[+] Start the app - it will wait for debugger."
        ;;
    strace)
        PID="${1:?PID or package required}"
        # If it's a package name, find the PID
        if [[ "$PID" =~ ^[a-z] ]]; then
            PID=$($ADB shell pidof "$PID" | tr -d '\r')
        fi
        $ADB shell strace -f -p "$PID" "${@:2}"
        ;;

    # ── Frida ─────────────────────────────────────────────────────────
    frida-setup)
        echo "[+] Setting up Frida server on device..."
        ARCH=$($ADB shell getprop ro.product.cpu.abi | tr -d '\r')
        FRIDA_VER="${1:-16.6.6}"
        FRIDA_URL="https://github.com/frida/frida/releases/download/${FRIDA_VER}/frida-server-${FRIDA_VER}-android-${ARCH}.xz"
        echo "[+] Downloading frida-server ${FRIDA_VER} for ${ARCH}..."
        curl -fSL "$FRIDA_URL" | xz -d > /tmp/frida-server
        $ADB push /tmp/frida-server /data/local/tmp/frida-server
        $ADB shell chmod 755 /data/local/tmp/frida-server
        echo "[+] Starting frida-server..."
        $ADB shell "/data/local/tmp/frida-server -D &" || true
        echo "[+] Frida server running."
        ;;

    # ── Proxy ─────────────────────────────────────────────────────────
    set-proxy)
        HOST="${1:?Proxy host required}"
        PORT="${2:?Proxy port required}"
        $ADB shell settings put global http_proxy "${HOST}:${PORT}"
        echo "[+] Proxy set to ${HOST}:${PORT}"
        ;;
    clear-proxy)
        $ADB shell settings put global http_proxy :0
        echo "[+] Proxy cleared."
        ;;

    # ── Cert Install ──────────────────────────────────────────────────
    install-cert)
        CERT="${1:?Certificate file required}"
        HASH=$(openssl x509 -inform PEM -subject_hash_old -in "$CERT" 2>/dev/null | head -1)
        $ADB root 2>/dev/null || true
        $ADB remount 2>/dev/null || true
        $ADB push "$CERT" "/system/etc/security/cacerts/${HASH}.0"
        $ADB shell chmod 644 "/system/etc/security/cacerts/${HASH}.0"
        echo "[+] CA certificate installed as ${HASH}.0"
        ;;

    *)
        echo "Unknown command: $CMD"
        echo "Available: install, uninstall, list-packages, clear-data, force-stop,"
        echo "  start-app, start-activity, broadcast, push, pull, shell, reboot,"
        echo "  logcat, screenshot, screenrecord, props, setprop, settings, dumpsys,"
        echo "  port-forward, reverse, tcpdump, bugreport, jdwp, attach-debugger,"
        echo "  strace, frida-setup, set-proxy, clear-proxy, install-cert"
        exit 1
        ;;
esac
APICTL
chmod +x "$CVD_HOME/cvd-api.sh"

# ── Step 7: Create DroidRun configuration ────────────────────────────────────
info "Creating DroidRun configuration for Cuttlefish..."
mkdir -p "$CVD_HOME/droidrun"

cat > "$CVD_HOME/droidrun/config.yaml" << DROIDCFG
_version: 3

agent:
  name: droidrun
  max_steps: 30
  reasoning: false
  streaming: true
  after_sleep_action: 1.0
  wait_for_stable_ui: 0.5
  use_normalized_coordinates: false

  fast_agent:
    vision: true
    codeact: false
    parallel_tools: true
    safe_execution: false

  manager:
    vision: true

  executor:
    vision: true

  app_cards:
    enabled: true
    mode: local

device:
  serial: null
  use_tcp: false
  platform: android
  auto_setup: true

tools:
  disabled_tools: []
  stealth: false

logging:
  level: INFO
  rich_text: true

tracing:
  enabled: false

telemetry:
  enabled: false
DROIDCFG

# ── Step 8: Symlink ADB for system-wide access ──────────────────────────────
info "Creating convenience symlinks..."
mkdir -p "$HOME/.local/bin"
if [[ -f "$CVD_HOME/bin/adb" ]]; then
    ln -sf "$CVD_HOME/bin/adb" "$HOME/.local/bin/cvd-adb" 2>/dev/null || true
fi

# ── Done ─────────────────────────────────────────────────────────────────────
echo ""
info "============================================"
info "Cuttlefish setup complete!"
info "============================================"
echo ""
echo "Workspace: $CVD_HOME"
echo ""
echo "Quick start:"
echo "  cd $CVD_HOME"
echo "  ./cvd-launch.sh              # Launch device"
echo "  ./cvd-stop.sh                # Stop device"
echo "  ./cvd-install-apk.sh app.apk # Install APK"
echo "  ./cvd-hotswap.sh app.apk     # Hot-swap reinstall"
echo "  ./cvd-api.sh <command>       # Full API control"
echo "  ./cvd-snapshot.sh save test1  # Save state"
echo ""
echo "DroidRun config: $CVD_HOME/droidrun/config.yaml"
echo ""
echo "NOTE: You may need to log out and back in for group changes to take effect."
echo "NOTE: If images were not auto-downloaded, follow the manual instructions above."
