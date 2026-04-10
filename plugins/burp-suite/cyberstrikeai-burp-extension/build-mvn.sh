#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DIST_DIR="$ROOT_DIR/dist"

MVN_BIN=""
if command -v mvn >/dev/null 2>&1; then
  MVN_BIN="mvn"
else
  # Auto-provision Maven for developer convenience.
  # This is only used to build the jar once in CI/dev; Burp users don't need to run this.
  MAVEN_VERSION="3.9.6"
  BASE_DIR="${HOME}/.cache/cyberstrikeai-burp-extension"
  MAVEN_DIR="$BASE_DIR/apache-maven-$MAVEN_VERSION"
  MAVEN_TGZ="$BASE_DIR/apache-maven-$MAVEN_VERSION-bin.tar.gz"
  MAVEN_URL="https://archive.apache.org/dist/maven/maven-3/$MAVEN_VERSION/binaries/apache-maven-$MAVEN_VERSION-bin.tar.gz"

  if [[ -x "$MAVEN_DIR/bin/mvn" ]]; then
    MVN_BIN="$MAVEN_DIR/bin/mvn"
  else
    echo "[*] Maven not found. Downloading Maven $MAVEN_VERSION ..."
    mkdir -p "$BASE_DIR"
    if command -v curl >/dev/null 2>&1; then
      curl -fsSL "$MAVEN_URL" -o "$MAVEN_TGZ"
    elif command -v wget >/dev/null 2>&1; then
      wget -q "$MAVEN_URL" -O "$MAVEN_TGZ"
    else
      echo "Missing: curl/wget (needed to download Maven)."
      exit 1
    fi
    tar -xzf "$MAVEN_TGZ" -C "$BASE_DIR"
    MVN_BIN="$MAVEN_DIR/bin/mvn"
  fi
fi

rm -rf "$DIST_DIR"
mkdir -p "$DIST_DIR"

echo "[*] Building with Maven (downloads Burp API from Maven Central)..."
(cd "$ROOT_DIR" && "$MVN_BIN" -q -DskipTests package)

cp "$ROOT_DIR/target/cyberstrikeai-burp-extension-1.0.0.jar" "$DIST_DIR/cyberstrikeai-burp-extension.jar"
echo "[+] Done: $DIST_DIR/cyberstrikeai-burp-extension.jar"

