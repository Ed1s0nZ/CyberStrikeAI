#!/usr/bin/env python3
"""
Censys Platform API v3 Plugin for CyberStrikeAI
================================================
Uses the new Censys Platform API (api.platform.censys.io/v3/).
Auth: Bearer token (Personal Access Token), NOT the old API ID/Secret.

Generate token: https://accounts.censys.io/settings/personal-access-tokens

Endpoints used:
  POST /v3/global/search/query     - search hosts/certs/web properties (1 credit)
  GET  /v3/global/asset/host/{ip}  - host detail lookup (free)
  GET  /v3/global/asset/certificate/{fp} - cert detail (free)
  POST /v3/global/search/aggregate - field aggregation (1 credit)
  GET  /v3/accounts/users/credits  - check credit balance (free)
  GET  /v3/global/asset/host/{ip}/names - get hostnames for IP (free)
"""

import os
import sys
import json
import requests

# ── Config ──────────────────────────────────────────────────────────
API_BASE = "https://api.platform.censys.io/v3"
PAT = os.environ.get("CENSYS_PAT", "").strip()
TIMEOUT = 30


def headers():
    return {
        "Authorization": f"Bearer {PAT}",
        "Content-Type": "application/json",
        "Accept": "application/json",
    }


def mask(s, keep=8):
    if len(s) <= keep * 2:
        return "*" * len(s)
    return s[:keep] + "*" * (len(s) - keep * 2) + s[-4:]


def handle_error(resp):
    """Parse Censys v3 error response."""
    try:
        data = resp.json()
        msg = data.get("error", {}).get("message", "") or data.get("message", "") or resp.text[:200]
    except Exception:
        msg = resp.text[:200]
    return {"status": "error", "http_code": resp.status_code, "message": msg}


# ── Commands ────────────────────────────────────────────────────────

def check_credits():
    """Check account credit balance and validate token."""
    resp = requests.get(f"{API_BASE}/accounts/users/credits", headers=headers(), timeout=TIMEOUT)
    if resp.status_code == 401:
        return {"status": "error", "message": "Invalid Censys PAT (401 Unauthorized). Generate at https://accounts.censys.io/settings/personal-access-tokens",
                "token_preview": mask(PAT)}
    if resp.status_code != 200:
        return handle_error(resp)
    data = resp.json()
    return {
        "status": "ok",
        "message": "Censys token valid",
        "token_preview": mask(PAT),
        "credits": data,
    }


def search_query(query, resource_type="host", limit=50, cursor=None, fields=None):
    """Search hosts, certificates, or web properties.
    Costs 1 credit per query."""
    body = {
        "q": query,
        "resource_type": resource_type,
        "limit": min(int(limit), 100),
    }
    if cursor:
        body["cursor"] = cursor
    if fields:
        body["fields"] = fields if isinstance(fields, list) else [f.strip() for f in fields.split(",")]

    resp = requests.post(f"{API_BASE}/global/search/query", headers=headers(),
                         json=body, timeout=TIMEOUT)
    if resp.status_code == 401:
        return {"status": "error", "message": "Invalid Censys PAT"}
    if resp.status_code == 422:
        return {"status": "error", "message": f"Invalid query: {resp.json().get('error', {}).get('message', resp.text[:200])}"}
    if resp.status_code != 200:
        return handle_error(resp)

    data = resp.json()
    result = {
        "status": "success",
        "query": query,
        "resource_type": resource_type,
        "total": data.get("total", 0),
        "count": len(data.get("result", [])),
        "results": data.get("result", []),
    }
    if data.get("cursor"):
        result["next_cursor"] = data["cursor"]
    if data.get("query_credits_used"):
        result["credits_used"] = data["query_credits_used"]
    return result


def host_lookup(ip):
    """Get detailed info for a specific IP (free, no credits)."""
    resp = requests.get(f"{API_BASE}/global/asset/host/{ip}", headers=headers(), timeout=TIMEOUT)
    if resp.status_code == 401:
        return {"status": "error", "message": "Invalid Censys PAT"}
    if resp.status_code == 404:
        return {"status": "error", "message": f"Host {ip} not found in Censys"}
    if resp.status_code != 200:
        return handle_error(resp)
    return {"status": "success", "ip": ip, "data": resp.json()}


def host_names(ip):
    """Get DNS names associated with an IP (free)."""
    resp = requests.get(f"{API_BASE}/global/asset/host/{ip}/names", headers=headers(), timeout=TIMEOUT)
    if resp.status_code != 200:
        return handle_error(resp)
    data = resp.json()
    return {
        "status": "success",
        "ip": ip,
        "names": data.get("result", []),
        "total": data.get("total", 0),
    }


def cert_lookup(fingerprint):
    """Get certificate details by SHA-256 fingerprint (free)."""
    resp = requests.get(f"{API_BASE}/global/asset/certificate/{fingerprint}",
                        headers=headers(), timeout=TIMEOUT)
    if resp.status_code == 404:
        return {"status": "error", "message": f"Certificate {fingerprint} not found"}
    if resp.status_code != 200:
        return handle_error(resp)
    return {"status": "success", "fingerprint": fingerprint, "data": resp.json()}


def aggregate(query, resource_type="host", agg_field="autonomous_system.name", num_buckets=25):
    """Aggregate search results by a field (1 credit)."""
    body = {
        "q": query,
        "resource_type": resource_type,
        "agg_field": agg_field,
        "num_buckets": min(int(num_buckets), 100),
    }
    resp = requests.post(f"{API_BASE}/global/search/aggregate", headers=headers(),
                         json=body, timeout=TIMEOUT)
    if resp.status_code != 200:
        return handle_error(resp)
    data = resp.json()
    return {
        "status": "success",
        "query": query,
        "agg_field": agg_field,
        "total": data.get("total", 0),
        "buckets": data.get("result", []),
    }


def webproperty_lookup(hostname, port=443):
    """Look up a web property by hostname:port (free)."""
    resp = requests.get(f"{API_BASE}/global/asset/webproperty/{hostname}:{port}",
                        headers=headers(), timeout=TIMEOUT)
    if resp.status_code == 404:
        return {"status": "error", "message": f"Web property {hostname}:{port} not found"}
    if resp.status_code != 200:
        return handle_error(resp)
    return {"status": "success", "hostname": hostname, "port": port, "data": resp.json()}


# ── Argument Parsing ────────────────────────────────────────────────

def parse_args():
    if len(sys.argv) > 1:
        try:
            config = json.loads(sys.argv[1])
            if isinstance(config, dict):
                return config
        except (json.JSONDecodeError, TypeError):
            pass

    config = {}
    if len(sys.argv) > 1:
        config["query"] = sys.argv[1]
    if len(sys.argv) > 2:
        config["resource_type"] = sys.argv[2]
    if len(sys.argv) > 3:
        try:
            config["limit"] = int(sys.argv[3])
        except ValueError:
            config["command"] = sys.argv[3]
    return config


# ── Main ────────────────────────────────────────────────────────────

def main():
    if not PAT:
        print(json.dumps({
            "status": "error",
            "message": "CENSYS_PAT not configured. Set your Personal Access Token in Settings > Plugins > Censys.",
            "note": "Generate token at https://accounts.censys.io/settings/personal-access-tokens",
            "migration": "Censys migrated to Platform API v3. Old API ID/Secret no longer works. Use a Personal Access Token (PAT) instead."
        }, indent=2))
        sys.exit(1)

    config = parse_args()
    query = config.get("query", "").strip()
    resource_type = config.get("resource_type", "host").strip().lower()
    limit = config.get("limit", 50)
    cursor = config.get("cursor")
    fields = config.get("fields")
    agg_field = config.get("agg_field")
    command = config.get("command", "").strip().lower()

    # Special commands
    if query in ("validate", "credits", "balance", "status") or command in ("validate", "credits"):
        result = check_credits()
        print(json.dumps(result, indent=2))
        sys.exit(0 if result["status"] == "ok" else 1)

    if not query:
        print(json.dumps({
            "status": "error",
            "message": "Missing required parameter: query",
            "special_commands": ["validate - check token validity and credit balance"],
            "search_examples": {
                "host": [
                    'port: 22',
                    'services.http.response.html_title: "Dashboard"',
                    'ip: 1.2.3.0/24',
                    'services.tls.certificate.names: "*.example.com"',
                    'autonomous_system.name: "HETZNER"',
                    'labels: "c2"',
                    'location.country: "Ukraine"',
                    'services.software.product: "OpenSSH" AND services.software.version: "7.*"',
                ],
                "certificate": [
                    'names: "*.example.com"',
                    'issuer.organization: "Let\'s Encrypt"',
                ],
            },
            "resource_types": ["host (default)", "certificate", "webproperty"],
            "free_lookups": [
                "Set resource_type=lookup and query=IP for free host detail",
                "Set resource_type=names and query=IP for DNS names",
                "Set resource_type=cert and query=SHA256 for cert detail",
                "Set resource_type=web and query=hostname for web property",
                "Set resource_type=aggregate and agg_field=field for aggregation",
            ],
        }, indent=2))
        sys.exit(1)

    try:
        # Route to the right function
        if resource_type in ("lookup", "detail", "host-detail"):
            result = host_lookup(query)
        elif resource_type == "names":
            result = host_names(query)
        elif resource_type in ("cert", "cert-detail", "certificate-detail"):
            result = cert_lookup(query)
        elif resource_type in ("web", "webproperty", "webprop"):
            port = config.get("port", 443)
            result = webproperty_lookup(query, port)
        elif resource_type == "aggregate":
            field = agg_field or "autonomous_system.name"
            result = aggregate(query, "host", field, limit)
        elif resource_type in ("certificate", "certificates", "certs"):
            result = search_query(query, "certificate", limit, cursor, fields)
        else:
            result = search_query(query, "host", limit, cursor, fields)

        print(json.dumps(result, indent=2, default=str))
        sys.exit(0 if result.get("status") == "success" or result.get("status") == "ok" else 1)

    except requests.exceptions.ConnectionError:
        print(json.dumps({"status": "error", "message": "Cannot connect to Censys API. Check network/DNS."}))
        sys.exit(1)
    except requests.exceptions.Timeout:
        print(json.dumps({"status": "error", "message": "Censys API timed out. Try a more specific query."}))
        sys.exit(1)
    except Exception as e:
        print(json.dumps({"status": "error", "message": f"{type(e).__name__}: {str(e)}"}))
        sys.exit(1)


if __name__ == "__main__":
    main()
