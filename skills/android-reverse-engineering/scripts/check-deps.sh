#!/usr/bin/env bash
# check-deps.sh - Verify dependencies and report what's missing
# Output includes machine-readable INSTALL:<dep> lines for each missing dependency.
# The install-dep.sh script can install each one.
set -euo pipefail

REQUIRED_JAVA_MAJOR=17
errors=0
missing_required=()
missing_optional=()

echo "=== Android Reverse Engineering: Dependency Check ==="
echo

# --- Java ---
java_ok=false
if command -v java &>/dev/null; then
  java_version_output=$(java -version 2>&1 | head -1)
  java_version=$(echo "$java_version_output" | sed -n 's/.*"\([0-9]*\)\..*/\1/p')
  if [[ -z "$java_version" ]]; then
    java_version=$(echo "$java_version_output" | grep -oP '\d+' | head -1)
  fi
  if [[ "$java_version" == "1" ]]; then
    java_version=$(echo "$java_version_output" | sed -n 's/.*"1\.\([0-9]*\)\..*/\1/p')
  fi

  if [[ -n "$java_version" ]] && (( java_version >= REQUIRED_JAVA_MAJOR )); then
    echo "[OK] Java $java_version detected"
    java_ok=true
  else
    echo "[WARN] Java detected but version $java_version is below $REQUIRED_JAVA_MAJOR"
    errors=$((errors + 1))
    missing_required+=("java")
  fi
else
  echo "[MISSING] Java is not installed or not in PATH"
  errors=$((errors + 1))
  missing_required+=("java")
fi

# --- jadx ---
if command -v jadx &>/dev/null; then
  jadx_version=$(jadx --version 2>/dev/null || echo "unknown")
  echo "[OK] jadx $jadx_version detected"
else
  echo "[MISSING] jadx is not installed or not in PATH"
  errors=$((errors + 1))
  missing_required+=("jadx")
fi

# --- Fernflower / Vineflower ---
ff_found=false
if command -v vineflower &>/dev/null; then
  echo "[OK] vineflower CLI detected"
  ff_found=true
elif command -v fernflower &>/dev/null; then
  echo "[OK] fernflower CLI detected"
  ff_found=true
else
  for candidate in \
    "${FERNFLOWER_JAR_PATH:-}" \
    "$HOME/.local/share/vineflower/vineflower.jar" \
    "$HOME/fernflower/build/libs/fernflower.jar" \
    "$HOME/vineflower/build/libs/vineflower.jar" \
    "$HOME/fernflower/fernflower.jar" \
    "$HOME/vineflower/vineflower.jar"; do
    if [[ -n "$candidate" ]] && [[ -f "$candidate" ]]; then
      echo "[OK] Fernflower/Vineflower JAR found: $candidate"
      ff_found=true
      break
    fi
  done
fi
if [[ "$ff_found" == false ]]; then
  echo "[MISSING] Fernflower/Vineflower not found (optional - better output on complex Java code)"
  missing_optional+=("vineflower")
fi

# --- dex2jar ---
if command -v d2j-dex2jar &>/dev/null || command -v d2j-dex2jar.sh &>/dev/null; then
  echo "[OK] dex2jar detected"
else
  echo "[MISSING] dex2jar not found (optional - needed to use Fernflower on APK/DEX files)"
  missing_optional+=("dex2jar")
fi

# --- Optional: apktool ---
if command -v apktool &>/dev/null; then
  echo "[OK] apktool detected (optional)"
else
  echo "[MISSING] apktool not found (optional - useful for resource decoding)"
  missing_optional+=("apktool")
fi

# --- Optional: adb ---
if command -v adb &>/dev/null; then
  echo "[OK] adb detected (optional)"
else
  echo "[MISSING] adb not found (optional - useful for pulling APKs from devices)"
  missing_optional+=("adb")
fi

# --- Machine-readable summary ---
echo
if [[ ${#missing_required[@]} -gt 0 ]]; then
  for dep in "${missing_required[@]}"; do
    echo "INSTALL_REQUIRED:$dep"
  done
fi
if [[ ${#missing_optional[@]} -gt 0 ]]; then
  for dep in "${missing_optional[@]}"; do
    echo "INSTALL_OPTIONAL:$dep"
  done
fi

echo
if (( errors > 0 )); then
  echo "*** ${#missing_required[@]} required dependency/ies missing. ***"
  echo "Run install-dep.sh <name> to install, or see references/setup-guide.md."
  exit 1
else
  if [[ ${#missing_optional[@]} -gt 0 ]]; then
    echo "Required dependencies OK. ${#missing_optional[@]} optional dependency/ies missing."
    echo "Run install-dep.sh <name> to install optional tools."
  else
    echo "All dependencies are installed. Ready to decompile."
  fi
  exit 0
fi
