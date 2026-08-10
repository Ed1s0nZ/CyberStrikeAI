#!/usr/bin/env python3
"""Push-time sensitive-information gate for CyberStrikeAI.

Blocks any push whose commits introduce:
  1. forbidden file types / paths (databases, evidence, reports, captures, keys...)
  2. customer asset data: real edu.cn / customer domains, internal IPs
  3. credentials / API tokens / secrets in non-placeholder form
  4. asset/vulnerability details registered in the local asset DB
     (hostnames and IPs extracted from data/conversations.db, read-only)

Project-specific markers (real interface paths, response codes, ...) belong in
local file data/push_guard_extra.txt (one regex per line, gitignored) so they
never enter the repo and the gate cannot match its own definition lines.

Usage:
  manual:   python scripts/pre_push_security_check.py [target-ref]
            (default target: fork/main, then origin/main, then repo root)
  hook:     installed as .git/hooks/pre-push; reads remote refs from stdin

Escape hatch (use only after manual review):
  SKIP_PRE_PUSH_CHECK=1 git push ...
"""

import os
import re
import sqlite3
import subprocess
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
ALL_ZERO = re.compile(r"^0+$")

# ---------------------------------------------------------------- path deny
DENY_PATH_RE = re.compile(
    r"(?:^|/)(?:evidence|reports|data|bootstrap_pkg)(?:/|$)"
    r"|\.(?:db|sqlite|sqlite3|pem|key|crt|p12|pcap|pcapng|zip|rar|7z|tar|gz|"
    r"docx|xlsx|pdf|png|jpe?g|gif|mp4|mp3|exe|dll|pdb)$",
    re.IGNORECASE,
)
# root-level pentest target captures (matches .gitignore rules)
ROOT_CAPTURE_RE = re.compile(r"^[^/]+\.(?:html|js|py|jpg|jpeg|vbs)$", re.IGNORECASE)
TEXT_EXT = re.compile(
    r"\.(?:go|md|yaml|yml|json|ps1|sh|py|js|ts|jsx|tsx|html|css|txt|conf|ini|"
    r"toml|xml|sql|vbs|bat|cfg|example|env|mod|sum|lock)$",
    re.IGNORECASE,
)

# ------------------------------------------------------------ content rules
IP_RE = re.compile(
    r"(?<![\d.])(?:10\.\d{1,3}\.\d{1,3}\.\d{1,3}"
    r"|172\.(?:1[6-9]|2\d|3[01])\.\d{1,3}\.\d{1,3}"
    r"|192\.168\.\d{1,3}\.\d{1,3})(?![\d.])"
)
EDU_CN_RE = re.compile(r"(?i)\b[a-z0-9](?:[a-z0-9-]*[a-z0-9])?\.edu\.cn\b")
KNOWN_CUSTOMER_RE = re.compile(
    r"(?i)\b[a-z0-9-]*\.(?:bjtu|tjutcm|tjnu|nankai|biem|bisu|tjufe|cupl|cqu|tuc|bitc)"
    r"\.(?:com|cn|net|org)\b"
)
# incident-specific markers live in data/push_guard_extra.txt (gitignored,
# one regex per line) to avoid both leaking project specifics into the repo
# and the gate matching its own definition lines.
SECRET_RE = re.compile(
    r"\bsk-[A-Za-z0-9]{16,}\b"
    r"|\bAKIA[0-9A-Z]{16}\b"
    r"|\b(?:ghp|gho|ghu|ghs|github_pat)_[A-Za-z0-9_]{20,}\b"
    r"|\bxox[baprs]-[A-Za-z0-9-]{10,}\b"
    r"|\bAIza[0-9A-Za-z_-]{20,}\b",
)
CRED_RE = re.compile(
    r"""(?i)\b(?:password|passwd|pwd|secret|api[_-]?key|apikey|client[_-]?secret|"""
    r"""access[_-]?token|auth[_-]?token|private[_-]?key)\b\s*[:=]\s*["']?([^\s"',;]{6,})"""
)
ALLOWED_IPS = {
    "127.0.0.1", "0.0.0.0", "::1",
    "10.0.0.1", "10.0.0.2", "10.0.1.1", "10.1.1.1",
    "192.168.1.1", "192.168.0.1", "192.168.1.100",
    "172.16.0.1", "172.17.0.1", "172.18.0.1",
}
PLACEHOLDER_VALUES = {
    "xxxxx", "xxxxxx", "xxx", "password", "changeme", "placeholder",
    "your-password", "your-api-key", "api-key-here", "your_token", "dummy",
    "example", "test", "test123", "123456",
}
ALLOWED_DOMAIN_SUFFIX = (
    ".example.com", ".example.net", ".example.org", ".example.edu",
    ".example.edu.cn", ".test.local", "localhost",
)

STATIC_RULES = [
    (EDU_CN_RE, "客户教育机构域名 (*.edu.cn)"),
    (KNOWN_CUSTOMER_RE, "已知客户域名"),
    (IP_RE, "内网/私网 IP"),
    (SECRET_RE, "疑似 API 密钥/Token"),
    (CRED_RE, "疑似明文凭据"),
]


def load_extra_patterns():
    """Local-only extra regexes (one per line) from data/push_guard_extra.txt."""
    path = os.path.join(ROOT, "data", "push_guard_extra.txt")
    rules = []
    if os.path.isfile(path):
        with open(path, encoding="utf-8") as f:
            for line in f:
                line = line.strip()
                if line and not line.startswith("#"):
                    try:
                        rules.append((re.compile(line), "本地自定义规则"))
                    except re.error as e:
                        print(f"[warn] push_guard_extra.txt 规则无效已跳过: {line} ({e})", file=sys.stderr)
    return rules


def load_db_asset_terms():
    """Read-only extraction of hostnames/IPs from the local asset DB."""
    db = os.path.join(ROOT, "data", "conversations.db")
    terms = set()
    if not os.path.isfile(db):
        return terms
    try:
        con = sqlite3.connect(f"file:{db}?mode=ro", uri=True)
        cur = con.cursor()
        host_cols = []
        for (t,) in cur.execute("SELECT name FROM sqlite_master WHERE type='table'"):
            try:
                cols = [r[1] for r in cur.execute(f"PRAGMA table_info({t})")]
            except sqlite3.Error:
                continue
            for c in cols:
                if c.lower() in ("target", "host", "hostname", "domain", "asset_name", "url", "ip"):
                    host_cols.append((t, c))
        for t, c in host_cols:
            try:
                rows = cur.execute(
                    f"SELECT DISTINCT {c} FROM {t} WHERE {c} IS NOT NULL AND {c} != ''"
                )
            except sqlite3.Error:
                continue
            for (v,) in rows:
                for m in re.finditer(
                    r"(?i)\b(?:[a-z0-9](?:[a-z0-9-]*[a-z0-9])?\.)+[a-z]{2,}\b|(?:\d{1,3}\.){3}\d{1,3}",
                    str(v),
                ):
                    val = m.group(0).lower()
                    if not any(val.endswith(s) for s in ALLOWED_DOMAIN_SUFFIX):
                        terms.add(val)
        con.close()
    except Exception as e:
        print(f"[warn] 本地资产库动态检测不可用: {e}", file=sys.stderr)
    return terms


def build_db_rules(terms):
    return [
        (re.compile(rf"(?i)(?<![a-z0-9.]){re.escape(t)}(?![a-z0-9.])"), "本地资产库登记的域名/IP")
        for t in sorted(terms)
    ]


def git(args):
    return subprocess.run(
        ["git", *args], cwd=ROOT, capture_output=True, text=True, encoding="utf-8", errors="replace"
    )


def resolve_ranges():
    """Returns list of (local_sha, base_sha_or_None)."""
    ranges = []
    stdin_data = sys.stdin.read() if not sys.stdin.isatty() else ""
    if stdin_data.strip():
        for line in stdin_data.splitlines():
            parts = line.split()
            if len(parts) != 4:
                continue
            local_sha, remote_sha = parts[1], parts[3]
            if ALL_ZERO.match(remote_sha):
                ranges.append((local_sha, None))
            else:
                ranges.append((local_sha, remote_sha))
        return ranges
    target = None
    if len(sys.argv) > 1:
        target = sys.argv[1]
    else:
        for cand in ("fork/main", "origin/main", "upstream/main"):
            if git(["rev-parse", "--verify", "--quiet", cand]).returncode == 0:
                target = cand
                break
    if target:
        r = git(["rev-parse", "--verify", "--quiet", target])
        if r.returncode == 0:
            return [(git(["rev-parse", "HEAD"]).stdout.strip(), r.stdout.strip())]
        print(f"[error] 目标引用不存在: {target}", file=sys.stderr)
        sys.exit(2)
    return [(git(["rev-parse", "HEAD"]).stdout.strip(), None)]


def changed_files(local_sha, base_sha):
    if base_sha is None:
        r = git(["ls-tree", "-r", "--name-only", local_sha])
        return [(p, "A") for p in r.stdout.splitlines() if p]
    r = git(["diff", "--name-status", "-M", base_sha, local_sha])
    files = []
    for line in r.stdout.splitlines():
        parts = line.split("\t")
        if not parts:
            continue
        status = parts[0][0]
        if status == "D":
            continue
        path = parts[-1]  # rename entries: R100\told\tnew
        files.append((path, status))
    return files


def file_content(local_sha, path):
    r = git(["show", f"{local_sha}:{path}"])
    if r.returncode != 0:
        return None
    return r.stdout


def check_file(path, content, rules):
    findings = []
    if b"\x00" in content.encode("utf-8", "replace"):
        return findings
    for lineno, raw in enumerate(content.splitlines(), 1):
        if "${" in raw:  # ${VAR} placeholder lines exempted
            continue
        for rx, desc in rules:
            for m in rx.finditer(raw):
                val = m.group(0)
                if rx is CRED_RE:
                    if val in PLACEHOLDER_VALUES or m.group(1).lower() in PLACEHOLDER_VALUES:
                        continue
                elif rx is IP_RE and val in ALLOWED_IPS:
                    continue
                if any(val.lower() == s.lstrip(".") or val.lower().endswith(s) for s in ALLOWED_DOMAIN_SUFFIX):
                    continue
                findings.append((path, lineno, desc, val))
    return findings


def main():
    rules = STATIC_RULES + load_extra_patterns() + build_db_rules(load_db_asset_terms())
    findings = []
    blocked_paths = set()
    scanned = 0
    for local_sha, base_sha in resolve_ranges():
        for path, status in changed_files(local_sha, base_sha):
            if DENY_PATH_RE.search(path) or ROOT_CAPTURE_RE.match(path):
                blocked_paths.add(path)
                continue
            if not TEXT_EXT.search(path):
                continue
            content = file_content(local_sha, path)
            if content is None:
                continue
            scanned += 1
            findings.extend(check_file(path, content, rules))

    if blocked_paths:
        print("[BLOCK] 推送被拦截：以下文件属于禁止入仓类型/路径：")
        for p in sorted(blocked_paths):
            print(f"  - {p}")
    if findings:
        print(f"[BLOCK] 推送被拦截：{len(findings)} 处敏感内容（{scanned} 个文本文件已扫描）：")
        seen = set()
        for path, lineno, desc, val in findings:
            key = (path, lineno, desc, val)
            if key in seen:
                continue
            seen.add(key)
            print(f"  - {path}:{lineno} [{desc}] {val[:80]}")
    if blocked_paths or findings:
        print("如需强制推送请人工复核后使用 SKIP_PRE_PUSH_CHECK=1（不推荐）")
        sys.exit(1)
    print(f"[OK] 推送安全检查通过（{scanned} 个文本文件，0 敏感命中）")


if __name__ == "__main__":
    main()
