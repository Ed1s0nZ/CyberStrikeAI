#!/usr/bin/env bash
# decompile.sh - Decompile APK/JAR/AAR using jadx, fernflower, or both
set -euo pipefail

usage() {
  cat <<EOF
Usage: decompile.sh [OPTIONS] <file>

Decompile an Android APK, XAPK, JAR, or AAR file.

Arguments:
  <file>            Path to the .apk, .xapk, .jar, or .aar file

Options:
  -o, --output DIR  Output directory (default: <filename>-decompiled)
  --deobf           Enable deobfuscation of names
  --no-res          Skip resource decoding (faster, code-only)
  --engine ENGINE   Decompiler engine: jadx, fernflower, or both (default: jadx)
  -h, --help        Show this help message

Engines:
  jadx        Use jadx (default). Handles APK/JAR/AAR natively, decodes resources.
  fernflower  Use Fernflower/Vineflower. Better on complex Java, lambdas, generics.
              For APK files, requires dex2jar as intermediate step.
  both        Run both decompilers side by side for comparison.
              jadx output  → <output>/jadx/
              fernflower   → <output>/fernflower/

Environment:
  FERNFLOWER_JAR_PATH   Path to fernflower.jar or vineflower.jar

Examples:
  decompile.sh app-release.apk
  decompile.sh app-bundle.xapk
  decompile.sh --engine both --deobf app-release.apk
  decompile.sh --engine fernflower library.jar
EOF
  exit 0
}

# --- Parse arguments ---
OUTPUT_DIR=""
DEOBF=false
NO_RES=false
ENGINE="jadx"
INPUT_FILE=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    -o|--output)   OUTPUT_DIR="$2"; shift 2 ;;
    --deobf)       DEOBF=true; shift ;;
    --no-res)      NO_RES=true; shift ;;
    --engine)      ENGINE="$2"; shift 2 ;;
    -h|--help)     usage ;;
    -*)            echo "Error: Unknown option $1" >&2; usage ;;
    *)             INPUT_FILE="$1"; shift ;;
  esac
done

# --- Validate input ---
if [[ -z "$INPUT_FILE" ]]; then
  echo "Error: No input file specified." >&2
  usage
fi

if [[ ! -f "$INPUT_FILE" ]]; then
  echo "Error: File not found: $INPUT_FILE" >&2
  exit 1
fi

ext="${INPUT_FILE##*.}"
ext_lower=$(echo "$ext" | tr '[:upper:]' '[:lower:]')
case "$ext_lower" in
  apk|xapk|jar|aar) ;;
  *)
    echo "Error: Unsupported file type '.$ext'. Expected .apk, .xapk, .jar, or .aar" >&2
    exit 1
    ;;
esac

case "$ENGINE" in
  jadx|fernflower|both) ;;
  *)
    echo "Error: Unknown engine '$ENGINE'. Use jadx, fernflower, or both." >&2
    exit 1
    ;;
esac

BASENAME=$(basename "$INPUT_FILE" ".$ext_lower")
INPUT_FILE_ABS=$(realpath "$INPUT_FILE")

if [[ -z "$OUTPUT_DIR" ]]; then
  OUTPUT_DIR="${BASENAME}-decompiled"
fi

# --- XAPK handling ---
# XAPK is a ZIP containing one or more APKs, optional OBB files, and a manifest.json.
# We extract it, find all APKs inside, and decompile each one.
XAPK_EXTRACTED_DIR=""
XAPK_APK_FILES=()

if [[ "$ext_lower" == "xapk" ]]; then
  XAPK_EXTRACTED_DIR=$(mktemp -d "${TMPDIR:-/tmp}/xapk-extract-XXXXXX")
  echo "=== Extracting XAPK archive ==="
  unzip -qo "$INPUT_FILE_ABS" -d "$XAPK_EXTRACTED_DIR"

  # Show manifest.json if present
  if [[ -f "$XAPK_EXTRACTED_DIR/manifest.json" ]]; then
    echo "XAPK manifest found:"
    cat "$XAPK_EXTRACTED_DIR/manifest.json"
    echo
  fi

  # Find all APK files inside
  while IFS= read -r -d '' apk_file; do
    XAPK_APK_FILES+=("$apk_file")
  done < <(find "$XAPK_EXTRACTED_DIR" -name "*.apk" -print0 | sort -z)

  if [[ ${#XAPK_APK_FILES[@]} -eq 0 ]]; then
    echo "Error: No APK files found inside XAPK archive." >&2
    rm -rf "$XAPK_EXTRACTED_DIR"
    exit 1
  fi

  echo "Found ${#XAPK_APK_FILES[@]} APK(s) inside XAPK:"
  for f in "${XAPK_APK_FILES[@]}"; do
    echo "  - $(basename "$f")"
  done
  echo
fi

# --- Locate fernflower JAR ---
find_fernflower_jar() {
  if [[ -n "${FERNFLOWER_JAR_PATH:-}" ]] && [[ -f "$FERNFLOWER_JAR_PATH" ]]; then
    echo "$FERNFLOWER_JAR_PATH"
    return
  fi
  # Check common locations
  for candidate in \
    "$HOME/fernflower/build/libs/fernflower.jar" \
    "$HOME/vineflower/build/libs/vineflower.jar" \
    "$HOME/fernflower/fernflower.jar" \
    "$HOME/vineflower/vineflower.jar"; do
    if [[ -f "$candidate" ]]; then
      echo "$candidate"
      return
    fi
  done
  return 1
}

# --- Locate dex2jar ---
find_dex2jar() {
  if command -v d2j-dex2jar &>/dev/null; then
    echo "d2j-dex2jar"
  elif command -v d2j-dex2jar.sh &>/dev/null; then
    echo "d2j-dex2jar.sh"
  else
    return 1
  fi
}

# --- jadx decompilation ---
run_jadx() {
  local out_dir="$1"

  if ! command -v jadx &>/dev/null; then
    echo "Error: jadx is not installed or not in PATH." >&2
    return 1
  fi

  local args=()
  args+=("-d" "$out_dir")
  [[ "$DEOBF" == true ]] && args+=("--deobf")
  [[ "$NO_RES" == true ]] && args+=("--no-res")
  args+=("--show-bad-code")
  args+=("$INPUT_FILE_ABS")

  echo "Running: jadx ${args[*]}"
  jadx "${args[@]}"

  echo "jadx output: $out_dir/sources/"
  if [[ -d "$out_dir/sources" ]]; then
    local count
    count=$(find "$out_dir/sources" -name "*.java" | wc -l)
    echo "Java files decompiled by jadx: $count"
  fi
}

# --- Fernflower decompilation ---
run_fernflower() {
  local out_dir="$1"
  local jar_to_decompile=""

  local ff_jar
  if ! ff_jar=$(find_fernflower_jar); then
    echo "Error: Fernflower/Vineflower JAR not found." >&2
    echo "Set FERNFLOWER_JAR_PATH or see references/setup-guide.md" >&2
    return 1
  fi

  mkdir -p "$out_dir"

  # For APK/AAR, we need dex2jar first to convert DEX→JAR
  if [[ "$ext_lower" == "apk" || "$ext_lower" == "aar" ]]; then
    local d2j
    if ! d2j=$(find_dex2jar); then
      echo "Error: dex2jar is required to use Fernflower on .$ext_lower files." >&2
      echo "Install dex2jar - see references/setup-guide.md" >&2
      return 1
    fi

    echo "Converting $ext_lower to JAR with dex2jar..."
    local converted_jar="$out_dir/${BASENAME}-dex2jar.jar"
    "$d2j" -f -o "$converted_jar" "$INPUT_FILE_ABS" 2>&1 || true
    if [[ ! -f "$converted_jar" ]]; then
      echo "Error: dex2jar conversion failed." >&2
      return 1
    fi
    jar_to_decompile="$converted_jar"
  else
    jar_to_decompile="$INPUT_FILE_ABS"
  fi

  # Build fernflower args
  local ff_args=()
  ff_args+=("-dgs=1")   # decompile generic signatures
  ff_args+=("-mpm=60")  # 60s max per method to avoid hangs
  if [[ "$DEOBF" == true ]]; then
    ff_args+=("-ren=1")  # rename obfuscated identifiers
  fi
  ff_args+=("$jar_to_decompile")
  ff_args+=("$out_dir")

  echo "Running: java -jar $ff_jar ${ff_args[*]}"
  java -jar "$ff_jar" "${ff_args[@]}"

  # Fernflower outputs a JAR containing .java files - extract it
  local result_jar="$out_dir/$(basename "$jar_to_decompile")"
  if [[ -f "$result_jar" ]]; then
    local sources_dir="$out_dir/sources"
    mkdir -p "$sources_dir"
    unzip -qo "$result_jar" -d "$sources_dir"
    rm -f "$result_jar"
    echo "Fernflower output: $sources_dir/"
    local count
    count=$(find "$sources_dir" -name "*.java" | wc -l)
    echo "Java files decompiled by Fernflower: $count"
  fi

  # Clean up intermediate dex2jar output
  if [[ -n "${converted_jar:-}" ]] && [[ -f "${converted_jar:-}" ]]; then
    rm -f "$converted_jar"
  fi
}

# --- Summary helper ---
print_structure() {
  local src_dir="$1"
  local label="$2"
  if [[ -d "$src_dir" ]]; then
    echo
    echo "Top-level packages ($label):"
    find "$src_dir" -mindepth 1 -maxdepth 3 -type d | head -20 | sed "s|$src_dir/||" | grep -v '^$' | sort
  fi
}

# --- Decompile a single file with the selected engine ---
decompile_single() {
  local file_abs="$1"
  local out_dir="$2"
  local label="$3"

  # Temporarily override INPUT_FILE_ABS for run_jadx/run_fernflower
  local saved_input="$INPUT_FILE_ABS"
  local saved_ext="$ext_lower"
  INPUT_FILE_ABS="$file_abs"
  ext_lower="${file_abs##*.}"
  ext_lower=$(echo "$ext_lower" | tr '[:upper:]' '[:lower:]')

  if [[ -n "$label" ]]; then
    echo "=== Decompiling $label (engine: $ENGINE) ==="
  fi

  case "$ENGINE" in
    jadx)
      run_jadx "$out_dir"
      print_structure "$out_dir/sources" "jadx"
      ;;
    fernflower)
      run_fernflower "$out_dir"
      print_structure "$out_dir/sources" "fernflower"
      ;;
    both)
      echo "--- Pass 1: jadx ---"
      run_jadx "$out_dir/jadx"
      echo
      echo "--- Pass 2: Fernflower ---"
      run_fernflower "$out_dir/fernflower"

      print_structure "$out_dir/jadx/sources" "jadx"
      print_structure "$out_dir/fernflower/sources" "fernflower"

      echo
      echo "=== Comparison ==="
      local jadx_count=0 ff_count=0
      if [[ -d "$out_dir/jadx/sources" ]]; then
        jadx_count=$(find "$out_dir/jadx/sources" -name "*.java" | wc -l)
      fi
      if [[ -d "$out_dir/fernflower/sources" ]]; then
        ff_count=$(find "$out_dir/fernflower/sources" -name "*.java" | wc -l)
      fi
      echo "jadx:        $jadx_count Java files"
      echo "Fernflower:  $ff_count Java files"

      if [[ -d "$out_dir/jadx/sources" ]]; then
        local jadx_errors
        jadx_errors=$(grep -rl 'JADX WARNING\|JADX WARN\|JADX ERROR\|Code decompiled incorrectly' "$out_dir/jadx/sources" 2>/dev/null | wc -l || echo 0)
        echo "jadx files with warnings/errors: $jadx_errors"
      fi
      echo
      echo "Tip: compare specific classes between jadx/ and fernflower/ to pick the better output."
      ;;
  esac

  INPUT_FILE_ABS="$saved_input"
  ext_lower="$saved_ext"
}

# --- Run ---
echo "=== Decompiling $INPUT_FILE (engine: $ENGINE) ==="
echo "Output directory: $OUTPUT_DIR"
echo

if [[ "$ext_lower" == "xapk" ]]; then
  # Decompile each APK found inside the XAPK
  mkdir -p "$OUTPUT_DIR"

  # Copy XAPK manifest for reference
  if [[ -f "$XAPK_EXTRACTED_DIR/manifest.json" ]]; then
    cp "$XAPK_EXTRACTED_DIR/manifest.json" "$OUTPUT_DIR/xapk-manifest.json"
  fi

  # Copy OBB file list for reference
  obb_files=()
  while IFS= read -r -d '' obb; do
    obb_files+=("$obb")
  done < <(find "$XAPK_EXTRACTED_DIR" -name "*.obb" -print0 2>/dev/null)
  if [[ ${#obb_files[@]} -gt 0 ]]; then
    echo "OBB files found (not decompiled, data-only):"
    for obb in "${obb_files[@]}"; do
      echo "  - $(basename "$obb") ($(du -h "$obb" | cut -f1))"
    done
    echo
  fi

  for apk_file in "${XAPK_APK_FILES[@]}"; do
    apk_name=$(basename "$apk_file" .apk)
    echo
    echo "======================================================"
    decompile_single "$apk_file" "$OUTPUT_DIR/$apk_name" "$apk_name.apk"
  done

  # Cleanup extracted XAPK
  rm -rf "$XAPK_EXTRACTED_DIR"

  echo
  echo "=== XAPK decompilation complete ==="
  echo "Subdirectories in $OUTPUT_DIR/:"
  ls -1 "$OUTPUT_DIR/"
else
  decompile_single "$INPUT_FILE_ABS" "$OUTPUT_DIR" ""
  echo
  echo "=== Decompilation complete ==="
fi
