#!/usr/bin/env python3
"""
crt.sh Certificate Transparency Search for CyberStrikeAI
=========================================================
Searches Certificate Transparency logs via crt.sh for:
- Subdomain discovery (wildcard cert enumeration)
- Certificate history for a domain
- Issuer analysis
- Infrastructure mapping via cert SANs

Two modes:
  1. HTTP JSON API (default) - fast, no deps beyond requests
  2. Direct PostgreSQL (--db mode) - full SQL power, slower but complete

crt.sh is free and requires no API key.
"""

import sys
import json
import argparse
import re
from datetime import datetime

# ── HTTP API mode ───────────────────────────────────────────────────

def search_http(query, dedupe=True, expired=False, limit=500):
    """Search via crt.sh JSON API (https://crt.sh/?q=QUERY&output=json)."""
    import requests

    url = "https://crt.sh/"
    params = {"q": query, "output": "json"}
    if not expired:
        params["exclude"] = "expired"

    # crt.sh returns 502 on heavy wildcard queries - retry with backoff,
    # then fallback to exact domain (strip % wildcards) if all retries fail
    import time as _time
    certs = None
    last_err = None
    original_query = query

    for attempt in range(3):
        try:
            resp = requests.get(url, params=params, timeout=60,
                               headers={"User-Agent": "CyberStrikeAI/1.0"})
            if resp.status_code == 404:
                return {"status": "success", "query": query, "total": 0, "subdomains": [],
                        "certificates": [], "message": "No results found"}
            if resp.status_code in (502, 503):
                if "exclude" in params:
                    del params["exclude"]  # drop filter to reduce server load
                last_err = f"crt.sh returned {resp.status_code}"
                _time.sleep(5 * (attempt + 1))
                continue
            resp.raise_for_status()
            certs = resp.json()
            break
        except Exception as e:
            last_err = str(e)
            _time.sleep(5 * (attempt + 1))
            continue

    # Fallback: if wildcard query failed 3 times, try without wildcards
    if certs is None and "%" in original_query:
        fallback_query = original_query.replace("%.", "").replace("%", "").strip(".")
        if fallback_query:
            params["q"] = fallback_query
            params.pop("exclude", None)
            try:
                resp = requests.get(url, params=params, timeout=60,
                                   headers={"User-Agent": "CyberStrikeAI/1.0"})
                if resp.status_code == 200:
                    certs = resp.json()
                    query = fallback_query  # update for output
            except Exception:
                pass

    if certs is None:
        return {"status": "error", "message": f"crt.sh failed after 3 retries + fallback: {last_err}",
                "query": original_query, "suggestion": "Try without % wildcard, or try again later"}

    if not isinstance(certs, list):
        return {"status": "success", "query": query, "total": 0, "subdomains": [],
                "certificates": [], "message": "No results"}

    # Truncate if too many
    if len(certs) > limit:
        certs = certs[:limit]

    # Extract unique subdomains from common_name and name_value fields
    subdomains = set()
    cert_entries = []

    for c in certs:
        cn = c.get("common_name", "")
        names = c.get("name_value", "")

        # Parse all names (can be newline-separated)
        all_names = set()
        if cn:
            all_names.add(cn.lower().strip())
        if names:
            for n in names.replace("\n", " ").replace(",", " ").split():
                n = n.strip().lower()
                if n and "." in n:
                    # Remove wildcard prefix for subdomain list
                    clean = n.lstrip("*.")
                    if clean:
                        subdomains.add(clean)
                    all_names.add(n)

        cert_entry = {
            "id": c.get("id"),
            "serial": c.get("serial_number", ""),
            "common_name": cn,
            "names": sorted(all_names) if all_names else [],
            "issuer": c.get("issuer_name", ""),
            "not_before": c.get("not_before", ""),
            "not_after": c.get("not_after", ""),
            "entry_timestamp": c.get("entry_timestamp", ""),
        }
        cert_entries.append(cert_entry)

    # Deduplicate certs by common_name + issuer + not_before
    if dedupe:
        seen = set()
        deduped = []
        for c in cert_entries:
            key = (c["common_name"], c["issuer"], c["not_before"])
            if key not in seen:
                seen.add(key)
                deduped.append(c)
        cert_entries = deduped

    # Sort subdomains
    sorted_subs = sorted(subdomains)

    # Analyze issuers
    issuer_counts = {}
    for c in cert_entries:
        issuer = c["issuer"]
        if issuer:
            # Extract org from issuer DN
            org_match = re.search(r'O=([^,]+)', issuer)
            org = org_match.group(1).strip() if org_match else issuer[:60]
            issuer_counts[org] = issuer_counts.get(org, 0) + 1

    top_issuers = sorted(issuer_counts.items(), key=lambda x: -x[1])[:10]

    return {
        "status": "success",
        "query": query,
        "total_certs": len(cert_entries),
        "unique_subdomains": len(sorted_subs),
        "subdomains": sorted_subs,
        "top_issuers": [{"name": name, "count": count} for name, count in top_issuers],
        "certificates": cert_entries[:100],  # Limit cert detail output
        "note": f"Showing {min(100, len(cert_entries))} of {len(cert_entries)} certificates" if len(cert_entries) > 100 else None,
    }


# ── PostgreSQL direct mode ──────────────────────────────────────────

def search_db(query, include_expired=False, limit=1000):
    """Direct PostgreSQL query to crt.sh database for full power."""
    try:
        import psycopg2
    except ImportError:
        return {"status": "error",
                "message": "psycopg2 not installed. Run: pip install psycopg2-binary"}

    conn_str = "host=crt.sh dbname=certwatch user=guest"

    try:
        conn = psycopg2.connect(conn_str, connect_timeout=15)
        conn.autocommit = True
        cur = conn.cursor()

        # Use the certificate_identity function for proper wildcard matching
        sql = """
            SELECT ci.NAME_VALUE, c.ID, c.SERIAL_NUMBER,
                   x509_commonName(c.CERTIFICATE),
                   x509_issuerName(c.CERTIFICATE),
                   x509_notBefore(c.CERTIFICATE),
                   x509_notAfter(c.CERTIFICATE),
                   c.ENTRY_TIMESTAMP
            FROM certificate_identity ci
            JOIN certificate c ON c.ID = ci.CERTIFICATE_ID
            WHERE ci.NAME_VALUE LIKE %s
                  AND ci.NAME_TYPE = 'dNSName'
        """
        params = [query]

        if not include_expired:
            sql += " AND x509_notAfter(c.CERTIFICATE) > NOW()"

        sql += " ORDER BY c.ENTRY_TIMESTAMP DESC LIMIT %s"
        params.append(limit)

        cur.execute(sql, params)
        rows = cur.fetchall()
        cur.close()
        conn.close()

        subdomains = set()
        cert_entries = []

        for row in rows:
            name_value, cert_id, serial, cn, issuer, not_before, not_after, entry_ts = row
            if name_value:
                clean = name_value.lower().strip().lstrip("*.")
                if clean and "." in clean:
                    subdomains.add(clean)

            cert_entries.append({
                "id": cert_id,
                "serial": serial or "",
                "common_name": cn or "",
                "name_value": name_value or "",
                "issuer": issuer or "",
                "not_before": str(not_before) if not_before else "",
                "not_after": str(not_after) if not_after else "",
                "entry_timestamp": str(entry_ts) if entry_ts else "",
            })

        sorted_subs = sorted(subdomains)
        return {
            "status": "success",
            "query": query,
            "mode": "database",
            "total_certs": len(cert_entries),
            "unique_subdomains": len(sorted_subs),
            "subdomains": sorted_subs,
            "certificates": cert_entries[:100],
        }

    except Exception as e:
        return {"status": "error", "message": f"crt.sh database error: {str(e)}",
                "suggestion": "Try HTTP mode (default) if DB connection fails"}


# ── Subdomain-only mode ─────────────────────────────────────────────

def subdomains_only(query):
    """Fast subdomain extraction - returns just the list, minimal output."""
    result = search_http(query, dedupe=True, expired=False)
    if result["status"] != "success":
        return result
    return {
        "status": "success",
        "query": query,
        "count": len(result.get("subdomains", [])),
        "subdomains": result.get("subdomains", []),
    }


# ── Main ────────────────────────────────────────────────────────────

def main():
    parser = argparse.ArgumentParser(description="crt.sh Certificate Transparency search")
    parser.add_argument("query", nargs="?", help="Domain to search (e.g., example.com or %%.example.com)")
    parser.add_argument("--mode", choices=["http", "db", "subdomains"], default="http",
                       help="Search mode: http (default), db (PostgreSQL direct), subdomains (names only)")
    parser.add_argument("--expired", action="store_true", help="Include expired certificates")
    parser.add_argument("--limit", type=int, default=500, help="Max results (default 500)")
    parser.add_argument("--wildcard", action="store_true",
                       help="Auto-prepend %% wildcard for subdomain discovery")

    # Handle both argparse and positional mode
    if len(sys.argv) > 1 and not sys.argv[1].startswith("-"):
        # Positional mode: query [mode] [limit]
        query = sys.argv[1]
        mode = sys.argv[2] if len(sys.argv) > 2 and not sys.argv[2].startswith("-") else "http"
        limit = 500
        expired = False
        wildcard = False

        # Check for flags mixed with positional
        for arg in sys.argv[2:]:
            if arg == "--expired":
                expired = True
            elif arg == "--wildcard":
                wildcard = True
            elif arg == "db":
                mode = "db"
            elif arg == "subdomains":
                mode = "subdomains"
            elif arg.isdigit():
                limit = int(arg)
    else:
        args = parser.parse_args()
        query = args.query
        mode = args.mode
        expired = args.expired
        limit = args.limit
        wildcard = args.wildcard

    if not query:
        print(json.dumps({
            "status": "error",
            "message": "Missing required parameter: query (domain name)",
            "examples": [
                "example.com - exact domain certificates",
                "%.example.com - all subdomain certificates (wildcard)",
                "%.%.example.com - deep subdomain discovery",
            ],
            "modes": {
                "http": "JSON API (default, fast)",
                "db": "Direct PostgreSQL (complete, slower)",
                "subdomains": "Subdomain list only (minimal output)",
            },
        }, indent=2))
        sys.exit(1)

    # Auto-prepend wildcard for subdomain discovery
    if wildcard and not query.startswith("%"):
        query = f"%.{query}"

    try:
        if mode == "db":
            result = search_db(query, include_expired=expired, limit=limit)
        elif mode == "subdomains":
            result = subdomains_only(query)
        else:
            result = search_http(query, dedupe=True, expired=expired, limit=limit)

        print(json.dumps(result, indent=2, default=str))
        sys.exit(0 if result.get("status") == "success" else 1)

    except Exception as e:
        print(json.dumps({"status": "error", "message": f"{type(e).__name__}: {str(e)}"}))
        sys.exit(1)


if __name__ == "__main__":
    main()
