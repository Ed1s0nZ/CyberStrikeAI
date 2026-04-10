#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIB_DIR="$ROOT_DIR/lib"
DIST_DIR="$ROOT_DIR/dist"
BUILD_DIR="$ROOT_DIR/.build"

API_JAR="$LIB_DIR/burp-extender-api.jar"

if [[ ! -f "$API_JAR" ]]; then
  echo "Missing: $API_JAR"
  echo "Please copy Burp's burp-extender-api.jar into plugins/burp-suite/cyberstrikeai-burp-extension/lib/"
  exit 1
fi

rm -rf "$BUILD_DIR" "$DIST_DIR"
mkdir -p "$BUILD_DIR" "$DIST_DIR"

SRC_FILES=$(find "$ROOT_DIR/src/main/java" -name "*.java")

echo "[*] Compiling..."
javac \
  -encoding UTF-8 \
  --release 11 \
  -cp "$API_JAR" \
  -d "$BUILD_DIR" \
  $SRC_FILES

echo "[*] Packaging..."
JAR_OUT="$DIST_DIR/cyberstrikeai-burp-extension.jar"
jar --create --file "$JAR_OUT" --main-class burp.BurpExtender -C "$BUILD_DIR" .

echo "[+] Done: $JAR_OUT"

