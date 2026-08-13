#!/usr/bin/env python3
"""从 MITRE 官方 STIX 数据生成 attack-chain 的固定 ATT&CK v19 最小快照。

用法：
    python3 build-attack-snapshot.py <enterprise-attack.json> [输出路径]

- 输入为 attack-stix-data 仓库的 enterprise-attack STIX bundle（需先自行下载）。
- 只保留 tactic ID/name/shortname、technique/sub-technique ID/name、tactic 归属、
  platforms、deprecated/revoked，以及来源 URL 与原始文件 SHA256。
- 输出默认写到 ../references/enterprise-attack-v19.json（相对本脚本）。
- 升级 ATT&CK 版本时必须显式重新下载数据、重跑本脚本，并重跑 validate-routes.mjs。

仅使用标准库。
"""

import hashlib
import json
import sys
from datetime import datetime, timezone
from pathlib import Path

SOURCE_URL = (
    "https://raw.githubusercontent.com/mitre-attack/attack-stix-data/"
    "master/enterprise-attack/enterprise-attack.json"
)
SNAPSHOT_ID = "enterprise-attack-19"


def load_bundle(path: Path) -> dict:
    with path.open("r", encoding="utf-8") as handle:
        return json.load(handle)


def sha256_of(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1 << 20), b""):
            digest.update(chunk)
    return digest.hexdigest()


def build_snapshot(bundle: dict, source_sha256: str) -> dict:
    objects = bundle.get("objects", [])

    tactics = []
    for obj in objects:
        if obj.get("type") == "x-mitre-tactic" and not obj.get("revoked", False):
            tactics.append(
                {
                    "id": next(
                        (ref["external_id"] for ref in obj.get("external_references", []) if ref.get("source_name") == "mitre-attack"),
                        None,
                    ),
                    "name": obj.get("name"),
                    "shortname": obj.get("x_mitre_shortname"),
                }
            )
    tactics.sort(key=lambda t: t["id"] or "")

    techniques = {}
    for obj in objects:
        if obj.get("type") != "attack-pattern":
            continue
        attack_id = next(
            (ref["external_id"] for ref in obj.get("external_references", []) if ref.get("source_name") == "mitre-attack"),
            None,
        )
        if not attack_id:
            continue
        is_sub = bool(obj.get("x_mitre_is_subtechnique", False))
        techniques[attack_id] = {
            "name": obj.get("name"),
            "tactics": [phase["phase_name"] for phase in obj.get("kill_chain_phases", []) if phase.get("kill_chain_name") == "mitre-attack"],
            "platforms": obj.get("x_mitre_platforms", []),
            "deprecated": bool(obj.get("x_mitre_deprecated", False)),
            "revoked": bool(obj.get("revoked", False)),
            "is_subtechnique": is_sub,
        }
        if is_sub:
            techniques[attack_id]["parent"] = attack_id.split(".")[0]

    version = None
    modified = None
    for obj in objects:
        if obj.get("type") == "x-mitre-collection":
            version = obj.get("x_mitre_version") or version
            modified = obj.get("modified") or modified

    return {
        "snapshot": SNAPSHOT_ID,
        "source": {
            "url": SOURCE_URL,
            "retrieved_utc": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
            "sha256": source_sha256,
            "attack_version": version,
            "stix_collection_modified": modified,
        },
        "tactics": tactics,
        "techniques": dict(sorted(techniques.items())),
    }


def main() -> int:
    if len(sys.argv) < 2:
        print(__doc__)
        return 2
    src = Path(sys.argv[1])
    if not src.is_file():
        print(f"输入文件不存在：{src}", file=sys.stderr)
        return 2
    out = (
        Path(sys.argv[2])
        if len(sys.argv) > 2
        else Path(__file__).resolve().parent.parent / "references" / "enterprise-attack-v19.json"
    )

    digest = sha256_of(src)
    bundle = load_bundle(src)
    snapshot = build_snapshot(bundle, digest)

    tactic_count = len(snapshot["tactics"])
    tech_count = sum(1 for t in snapshot["techniques"].values() if not t["is_subtechnique"])
    sub_count = sum(1 for t in snapshot["techniques"].values() if t["is_subtechnique"])

    out.parent.mkdir(parents=True, exist_ok=True)
    with out.open("w", encoding="utf-8", newline="\n") as handle:
        json.dump(snapshot, handle, ensure_ascii=False, indent=2)
        handle.write("\n")

    print(f"snapshot: {snapshot['snapshot']} (ATT&CK version={snapshot['source']['attack_version']})")
    print(f"source sha256: {digest}")
    print(f"tactics: {tactic_count}, techniques: {tech_count}, sub-techniques: {sub_count}")
    print(f"written: {out}")
    if tactic_count != 15:
        print("WARNING: tactic 数量不是 15，请确认使用的是 Enterprise ATT&CK v19.x 数据", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
