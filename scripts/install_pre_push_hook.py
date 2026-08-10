#!/usr/bin/env python3
"""Idempotent installer for the pre-push sensitive-info gate hook.

Usage: python scripts/install_pre_push_hook.py
Writes .git/hooks/pre-push (managed copy; existing file is overwritten).
"""

import os
import stat
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
HOOK_PATH = os.path.join(ROOT, ".git", "hooks", "pre-push")

HOOK = """#!/bin/sh
# Pre-push sensitive-information gate for CyberStrikeAI.
# Managed by scripts/install_pre_push_hook.py; do not edit by hand.
# Escape hatch (manual review required): SKIP_PRE_PUSH_CHECK=1 git push ...

if [ -n "$SKIP_PRE_PUSH_CHECK" ]; then
    echo "[pre-push] SKIP_PRE_PUSH_CHECK set, skipping sensitive-info gate"
    exit 0
fi

PY=""
for c in python python3 py; do
    if command -v "$c" >/dev/null 2>&1; then PY="$c"; break; fi
done

if [ -z "$PY" ]; then
    echo "[pre-push] ERROR: python not found; refusing to push without sensitive-info gate" >&2
    exit 1
fi

SCRIPT="$(dirname "$0")/../../scripts/pre_push_security_check.py"
exec "$PY" "$SCRIPT" "$@"
"""


def main():
    with open(HOOK_PATH, "w", encoding="utf-8", newline="\n") as f:
        f.write(HOOK)
    try:
        mode = os.stat(HOOK_PATH).st_mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH
        os.chmod(HOOK_PATH, mode)
    except OSError:
        pass  # Windows: exec bit is not enforced
    print(f"[OK] pre-push hook installed: {HOOK_PATH}")


if __name__ == "__main__":
    main()
