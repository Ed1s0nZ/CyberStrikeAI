#!/usr/bin/env python3
"""
Convert `web/static/i18n/zh-CN.json` -> `web/static/i18n/zh-TW.json`.

This script uses OpenCC to convert *all string leaf values* (keys are preserved).
After conversion, it overrides `lang.zhTW` (and sets `lang.zhCN` / `lang.enUS` defaults).
"""

from __future__ import annotations

import argparse
import json
from typing import Any

from opencc import OpenCC


def convert_any(x: Any, cc: OpenCC) -> Any:
    if isinstance(x, str):
        return cc.convert(x)
    if isinstance(x, list):
        return [convert_any(v, cc) for v in x]
    if isinstance(x, dict):
        # Preserve original key order from the source JSON
        return {k: convert_any(v, cc) for k, v in x.items()}
    return x


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--src",
        default="web/static/i18n/zh-CN.json",
        help="Path to the Simplified Chinese JSON file",
    )
    parser.add_argument(
        "--dst",
        default="web/static/i18n/zh-TW.json",
        help="Path to write the Traditional Chinese JSON file",
    )
    parser.add_argument(
        "--mode",
        default="s2twp",
        help="OpenCC conversion mode (e.g. s2twp for Taiwan with phrases)",
    )
    args = parser.parse_args()

    with open(args.src, "r", encoding="utf-8") as f:
        data = json.load(f)

    cc = OpenCC(args.mode)
    out = convert_any(data, cc)

    # Override language labels to ensure dropdown display is correct.
    lang = out.get("lang")
    if isinstance(lang, dict):
        lang["zhTW"] = "繁體中文"
        lang.setdefault("zhCN", "簡體中文")
        lang.setdefault("enUS", "English")

    with open(args.dst, "w", encoding="utf-8") as f:
        json.dump(out, f, ensure_ascii=False, indent=2)
        f.write("\n")

    print(f"Wrote {args.dst}")


if __name__ == "__main__":
    main()
