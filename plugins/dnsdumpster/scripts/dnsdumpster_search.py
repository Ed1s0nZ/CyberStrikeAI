#!/usr/bin/env python3
"""
DNSdumpster DNS Reconnaissance Plugin for CyberStrikeAI
=======================================================
Subdomain enumeration, DNS record lookup (A, AAAA, MX, NS, TXT, SOA, CNAME),
host IP resolution, GeoIP data, reverse DNS, technology detection, network mapping.

Auth: X-API-Key header on every request.
Base URL: https://api.dnsdumpster.com

Also supports HackerTarget API endpoints for host lookup, reverse IP, GeoIP:
  Base URL: https://api.hackertarget.com

Tiers:
  Free:  50 DNS records per domain, 50 HackerTarget calls/day
  Plus:  200 records, pagination, domain mapping, banner/CIDR search
"""

import os
import sys
import json
import time

# -- Config ----------------------------------------------------------------
API_BASE = "https://api.dnsdumpster.com"
HT_BASE = "https://api.hackertarget.com"
API_KEY = os.environ.get("DNSDUMPSTER_API_KEY", "").strip()
TIMEOUT = 30
MAX_RETRIES = 3
RETRY_BACKOFF = 2  # seconds, multiplied by attempt number


def _get(url, params=None, timeout=TIMEOUT, use_header_auth=True):
    """HTTP GET with auth, retry on 429, structured error handling."""
    import requests

    if params is None:
        params = {}

    headers = {"User-Agent": "CyberStrikeAI/1.0"}
    if use_header_auth and API_KEY:
        headers["X-API-Key"] = API_KEY

    last_err = None
    for attempt in range(1, MAX_RETRIES + 1):
        try:
            resp = requests.get(url, params=params, headers=headers,
                                timeout=timeout)

            if resp.status_code == 200:
                content_type = resp.headers.get("Content-Type", "")
                if "application/json" in content_type:
                    return resp.json()
                # HackerTarget returns plain text
                return {"_raw": resp.text, "_status_code": 200}

            if resp.status_code == 401:
                return {
                    "error": "Invalid API key (401). Check your DNSDUMPSTER_API_KEY in Settings > Plugins > DNSdumpster.",
                    "status": "error", "http_code": 401
                }

            if resp.status_code == 403:
                return {
                    "error": "Access denied (403). Your API key may lack permissions for this endpoint or tier.",
                    "status": "error", "http_code": 403
                }

            if resp.status_code == 404:
                try:
                    data = resp.json()
                    return {"error": data.get("error", "Not found"), "status": "error", "http_code": 404}
                except Exception:
                    return {"error": "Resource not found (404)", "status": "error", "http_code": 404}

            if resp.status_code == 429:
                wait = RETRY_BACKOFF * attempt
                last_err = f"Rate limited (429). Retrying in {wait}s (attempt {attempt}/{MAX_RETRIES})"
                time.sleep(wait)
                continue

            # Other errors
            try:
                data = resp.json()
                msg = data.get("error", resp.text[:300])
            except Exception:
                msg = resp.text[:300]
            return {"error": msg, "status": "error", "http_code": resp.status_code}

        except Exception as e:
            last_err = f"{type(e).__name__}: {str(e)}"
            if attempt < MAX_RETRIES:
                time.sleep(RETRY_BACKOFF * attempt)
                continue
            break

    return {"error": f"Request failed after {MAX_RETRIES} attempts: {last_err}", "status": "error"}


def mask_key(s, keep=6):
    """Mask API key for safe display."""
    if not s or len(s) <= keep * 2:
        return "*" * max(len(s) if s else 0, 8)
    return s[:keep] + "*" * (len(s) - keep * 2) + s[-4:]


def _parse_ht_text(text, fields=None):
    """Parse HackerTarget plain text response into structured list.

    HackerTarget returns CSV-like text, one record per line.
    If fields is given, split each line by comma and map to field names.
    """
    if not text or text.startswith("error"):
        return {"error": text or "Empty response from HackerTarget", "status": "error"}

    lines = [l.strip() for l in text.strip().split("\n") if l.strip()]
    if fields:
        records = []
        for line in lines:
            parts = [p.strip() for p in line.split(",")]
            record = {}
            for i, field in enumerate(fields):
                record[field] = parts[i] if i < len(parts) else ""
            records.append(record)
        return records
    return lines


# -- Commands ---------------------------------------------------------------

def cmd_validate():
    """Validate API key by making a test domain lookup.

    DNSdumpster doesn't have a dedicated account/info endpoint,
    so we validate by making a lightweight request.
    """
    if not API_KEY:
        return {
            "status": "error",
            "message": "DNSDUMPSTER_API_KEY not configured.",
            "note": "Get your API key at https://dnsdumpster.com (account dashboard)",
        }

    # Test with a known domain
    data = _get(f"{API_BASE}/domain/example.com")
    if isinstance(data, dict) and data.get("status") == "error":
        return {
            "status": "error",
            "command": "validate",
            "key_preview": mask_key(API_KEY),
            "message": data.get("error", "API key validation failed"),
        }

    return {
        "status": "success",
        "command": "validate",
        "key_preview": mask_key(API_KEY),
        "message": "API key is valid",
        "api_base": API_BASE,
        "note": "Free tier: 50 records/domain. Plus tier: 200 records, pagination, maps, banner search.",
        "rate_limit": "1 request per 2 seconds",
    }


def cmd_domain(domain, page=None, include_map=False, record_type=None):
    """Full DNS dump for a domain: subdomains, A/MX/NS/TXT/CNAME records, ASN, banners.

    This is the primary DNSdumpster endpoint.
    """
    params = {}
    if page and page > 1:
        params["page"] = page
    if include_map:
        params["map"] = 1

    data = _get(f"{API_BASE}/domain/{domain}", params)

    if isinstance(data, dict) and data.get("status") == "error":
        return data
    if isinstance(data, dict) and "_raw" in data:
        # Unexpected text response
        return {
            "status": "error",
            "command": "domain",
            "message": "Unexpected response format",
            "raw": data["_raw"][:500],
        }

    # Structure the response
    result = {
        "status": "success",
        "command": "domain",
        "domain": domain,
    }

    # DNSdumpster API returns a dict with various record types
    if isinstance(data, dict):
        # DNS records
        dns_records = data.get("dns", data.get("a", []))
        mx_records = data.get("mx", [])
        ns_records = data.get("ns", [])
        txt_records = data.get("txt", [])
        cname_records = data.get("cname", [])
        soa_records = data.get("soa", [])

        # Process A/host records
        hosts = []
        if isinstance(dns_records, list):
            for rec in dns_records:
                if isinstance(rec, dict):
                    entry = {
                        "host": rec.get("host", rec.get("domain", "")),
                        "ip": rec.get("ip", rec.get("address", "")),
                        "reverse_dns": rec.get("reverse_dns", rec.get("rdns", "")),
                        "asn": rec.get("asn", ""),
                        "country": rec.get("country", ""),
                        "header": rec.get("header", rec.get("banner", "")),
                        "provider": rec.get("as_name", rec.get("provider", "")),
                        "netblock": rec.get("netblock", ""),
                    }
                    hosts.append(entry)
                elif isinstance(rec, str):
                    hosts.append({"host": rec})

        result["a_records"] = hosts
        result["total_hosts"] = len(hosts)

        # MX records
        mx_list = []
        if isinstance(mx_records, list):
            for rec in mx_records:
                if isinstance(rec, dict):
                    mx_list.append({
                        "host": rec.get("host", rec.get("domain", "")),
                        "ip": rec.get("ip", rec.get("address", "")),
                        "priority": rec.get("priority", ""),
                        "reverse_dns": rec.get("reverse_dns", rec.get("rdns", "")),
                        "asn": rec.get("asn", ""),
                        "provider": rec.get("as_name", rec.get("provider", "")),
                    })
                elif isinstance(rec, str):
                    mx_list.append({"host": rec})
        result["mx_records"] = mx_list

        # NS records
        ns_list = []
        if isinstance(ns_records, list):
            for rec in ns_records:
                if isinstance(rec, dict):
                    ns_list.append({
                        "host": rec.get("host", rec.get("domain", "")),
                        "ip": rec.get("ip", rec.get("address", "")),
                        "reverse_dns": rec.get("reverse_dns", rec.get("rdns", "")),
                        "asn": rec.get("asn", ""),
                        "provider": rec.get("as_name", rec.get("provider", "")),
                    })
                elif isinstance(rec, str):
                    ns_list.append({"host": rec})
        result["ns_records"] = ns_list

        # TXT records
        txt_list = []
        if isinstance(txt_records, list):
            for rec in txt_records:
                if isinstance(rec, dict):
                    txt_list.append(rec.get("value", rec.get("txt", str(rec))))
                elif isinstance(rec, str):
                    txt_list.append(rec)
        result["txt_records"] = txt_list

        # CNAME records
        cname_list = []
        if isinstance(cname_records, list):
            for rec in cname_records:
                if isinstance(rec, dict):
                    cname_list.append({
                        "host": rec.get("host", rec.get("domain", "")),
                        "target": rec.get("target", rec.get("cname", "")),
                    })
                elif isinstance(rec, str):
                    cname_list.append({"host": rec})
        result["cname_records"] = cname_list

        # SOA records
        if isinstance(soa_records, list) and soa_records:
            result["soa_records"] = soa_records
        elif isinstance(soa_records, dict) and soa_records:
            result["soa_records"] = [soa_records]

        # Subdomains (extract unique hostnames from all record types)
        subdomains = set()
        for h in hosts:
            hostname = h.get("host", "")
            if hostname and hostname != domain and hostname.endswith(f".{domain}"):
                subdomains.add(hostname)
        result["subdomains"] = sorted(subdomains)
        result["total_subdomains"] = len(subdomains)

        # Map data (base64)
        if data.get("image_data") or data.get("map"):
            result["map_data"] = "base64-encoded map available (use include_map=true)"

        # Banner / technology data
        if data.get("banners"):
            result["banners"] = data["banners"]

        # Pass through any extra fields from the API
        for key in ("networks", "netblocks", "asn_info"):
            if key in data:
                result[key] = data[key]

    else:
        # If data came back as list or something else, include it raw
        result["raw_data"] = data

    # Filter by record type if specified
    if record_type:
        rt = record_type.upper()
        filtered = {"status": "success", "command": "domain", "domain": domain, "filter": rt}
        type_map = {
            "A": "a_records",
            "MX": "mx_records",
            "NS": "ns_records",
            "TXT": "txt_records",
            "CNAME": "cname_records",
            "SOA": "soa_records",
        }
        key = type_map.get(rt)
        if key and key in result:
            filtered[key] = result[key]
            filtered["total"] = len(result[key]) if isinstance(result[key], list) else 1
        else:
            filtered["message"] = f"No {rt} records found or unsupported type"
            filtered["available_types"] = list(type_map.keys())
        return filtered

    return result


def cmd_banners(cidr):
    """Banner search by CIDR (Plus tier). Max /24 network."""
    data = _get(f"{API_BASE}/banners/{cidr}")

    if isinstance(data, dict) and data.get("status") == "error":
        return data
    if isinstance(data, dict) and "_raw" in data:
        return {
            "status": "success",
            "command": "banners",
            "cidr": cidr,
            "raw": data["_raw"][:5000],
        }

    return {
        "status": "success",
        "command": "banners",
        "cidr": cidr,
        "results": data if isinstance(data, list) else [data],
        "total": len(data) if isinstance(data, list) else 1,
        "note": "Banner search requires Plus tier. Max /24 CIDR.",
    }


def cmd_host(ip):
    """IP address info lookup via HackerTarget GeoIP + reverse DNS.

    Combines multiple HackerTarget endpoints for comprehensive host info.
    """
    results = {"status": "success", "command": "host", "ip": ip}

    # GeoIP lookup
    geo_data = _get(f"{HT_BASE}/geoip/", params={"q": ip}, use_header_auth=False)
    if isinstance(geo_data, dict) and "_raw" in geo_data:
        text = geo_data["_raw"]
        if not text.startswith("error"):
            lines = _parse_ht_text(text)
            if isinstance(lines, list):
                geo_info = {}
                for line in lines:
                    if isinstance(line, str) and ":" in line:
                        k, v = line.split(":", 1)
                        geo_info[k.strip().lower().replace(" ", "_")] = v.strip()
                results["geoip"] = geo_info

    # Reverse DNS
    rdns_data = _get(f"{HT_BASE}/reversedns/", params={"q": ip}, use_header_auth=False)
    if isinstance(rdns_data, dict) and "_raw" in rdns_data:
        text = rdns_data["_raw"]
        if not text.startswith("error"):
            lines = [l.strip() for l in text.strip().split("\n") if l.strip()]
            results["reverse_dns"] = lines

    # AS lookup
    as_data = _get(f"{HT_BASE}/aslookup/", params={"q": ip}, use_header_auth=False)
    if isinstance(as_data, dict) and "_raw" in as_data:
        text = as_data["_raw"]
        if not text.startswith("error"):
            parsed = _parse_ht_text(text, ["ip", "asn", "range", "description", "country"])
            if isinstance(parsed, list) and parsed:
                results["asn_info"] = parsed[0] if len(parsed) == 1 else parsed

    return results


def cmd_reverse(ip):
    """Reverse IP lookup via HackerTarget - find all domains hosted on an IP."""
    data = _get(f"{HT_BASE}/reverseiplookup/", params={"q": ip}, use_header_auth=False)

    if isinstance(data, dict) and data.get("status") == "error":
        return data

    if isinstance(data, dict) and "_raw" in data:
        text = data["_raw"]
        if text.startswith("error"):
            return {"status": "error", "command": "reverse", "ip": ip, "message": text}

        domains = [l.strip() for l in text.strip().split("\n") if l.strip()]
        return {
            "status": "success",
            "command": "reverse",
            "ip": ip,
            "domains": domains,
            "total": len(domains),
        }

    return {
        "status": "success",
        "command": "reverse",
        "ip": ip,
        "results": data,
    }


def cmd_dns_lookup(domain):
    """DNS lookup via HackerTarget - returns all DNS records for a domain."""
    data = _get(f"{HT_BASE}/dnslookup/", params={"q": domain}, use_header_auth=False)

    if isinstance(data, dict) and data.get("status") == "error":
        return data

    if isinstance(data, dict) and "_raw" in data:
        text = data["_raw"]
        if text.startswith("error"):
            return {"status": "error", "command": "dns", "domain": domain, "message": text}

        records = []
        for line in text.strip().split("\n"):
            line = line.strip()
            if not line:
                continue
            parts = line.split()
            if len(parts) >= 3:
                records.append({
                    "name": parts[0],
                    "type": parts[1] if len(parts) > 1 else "",
                    "value": " ".join(parts[2:]) if len(parts) > 2 else "",
                })
            else:
                records.append({"raw": line})

        return {
            "status": "success",
            "command": "dns",
            "domain": domain,
            "records": records,
            "total": len(records),
        }

    return {
        "status": "success",
        "command": "dns",
        "domain": domain,
        "results": data,
    }


def cmd_hostsearch(domain):
    """Subdomain/host search via HackerTarget - find hosts related to a domain."""
    data = _get(f"{HT_BASE}/hostsearch/", params={"q": domain}, use_header_auth=False)

    if isinstance(data, dict) and data.get("status") == "error":
        return data

    if isinstance(data, dict) and "_raw" in data:
        text = data["_raw"]
        if text.startswith("error"):
            return {"status": "error", "command": "hostsearch", "domain": domain, "message": text}

        parsed = _parse_ht_text(text, ["hostname", "ip"])
        if isinstance(parsed, dict) and parsed.get("status") == "error":
            return parsed

        return {
            "status": "success",
            "command": "hostsearch",
            "domain": domain,
            "hosts": parsed,
            "total": len(parsed),
        }

    return {
        "status": "success",
        "command": "hostsearch",
        "domain": domain,
        "results": data,
    }


# -- Argument Parsing -------------------------------------------------------

def parse_args():
    """Parse arguments: supports JSON object or positional args."""
    if len(sys.argv) > 1:
        # Try JSON mode first (from tool framework)
        try:
            config = json.loads(sys.argv[1])
            if isinstance(config, dict):
                return config
        except (json.JSONDecodeError, TypeError):
            pass

    # Positional mode: query [command] [type]
    config = {}
    args = sys.argv[1:]

    # Parse flags
    positionals = []
    i = 0
    while i < len(args):
        arg = args[i]
        if arg == "--page" and i + 1 < len(args):
            config["page"] = int(args[i + 1])
            i += 1
        elif arg == "--map":
            config["include_map"] = True
        elif arg == "--type" and i + 1 < len(args):
            config["type"] = args[i + 1]
            i += 1
        else:
            positionals.append(arg)
        i += 1

    if len(positionals) > 0:
        config["query"] = positionals[0]
    if len(positionals) > 1:
        config["command"] = positionals[1]
    if len(positionals) > 2:
        config["type"] = positionals[2]

    return config


# -- Main -------------------------------------------------------------------

def main():
    if not API_KEY:
        print(json.dumps({
            "status": "error",
            "message": "DNSDUMPSTER_API_KEY not configured. Set your API key in Settings > Plugins > DNSdumpster.",
            "note": "Get your API key at https://dnsdumpster.com (sign in, then check account dashboard)",
            "free_tier": "Free tier: 50 records per domain lookup, basic DNS enumeration",
            "plus_tier": "Plus tier: 200 records, pagination, domain mapping, CIDR banner search",
            "commands": {
                "domain": "Full DNS dump for a domain (subdomains, A/MX/NS/TXT records)",
                "host": "IP address info (GeoIP, reverse DNS, ASN)",
                "reverse": "Reverse IP lookup (find all domains on same IP)",
                "dns": "DNS record lookup for a domain",
                "hostsearch": "Host/subdomain search for a domain",
                "banners": "Banner search by CIDR (Plus tier)",
                "validate": "Check API key validity",
            },
        }, indent=2))
        sys.exit(1)

    config = parse_args()
    query = config.get("query", "").strip()
    command = config.get("command", "").strip().lower()
    record_type = config.get("type", "").strip() or None

    # Auto-detect command from query if not specified
    if not command:
        if query in ("validate", "status", "info", "check"):
            command = "validate"
        else:
            command = "domain"  # default

    # Also support command=validate with empty query
    if command in ("validate", "status", "info", "check"):
        result = cmd_validate()
    elif not query:
        print(json.dumps({
            "status": "error",
            "message": "Missing required parameter: query",
            "commands": {
                "domain": "Full DNS dump for a domain (subdomains, A/MX/NS/TXT records, ASN, banners)",
                "host": "IP address info lookup (GeoIP, reverse DNS, ASN)",
                "reverse": "Reverse IP lookup (find all domains hosted on same IP)",
                "dns": "DNS record lookup for a domain",
                "hostsearch": "Host/subdomain search for a domain",
                "banners": "Banner search by CIDR network (Plus tier, max /24)",
                "validate": "Check API key validity",
            },
            "examples": [
                'dnsdumpster_search query="example.com" command="domain"',
                'dnsdumpster_search query="example.com" command="domain" type="MX"',
                'dnsdumpster_search query="8.8.8.8" command="host"',
                'dnsdumpster_search query="8.8.8.8" command="reverse"',
                'dnsdumpster_search query="example.com" command="dns"',
                'dnsdumpster_search query="example.com" command="hostsearch"',
                'dnsdumpster_search query="192.168.1.0/24" command="banners"',
            ],
        }, indent=2))
        sys.exit(1)
    elif command == "domain":
        result = cmd_domain(
            query,
            page=config.get("page"),
            include_map=config.get("include_map", False),
            record_type=record_type,
        )
    elif command == "host":
        result = cmd_host(query)
    elif command in ("reverse", "reverseip", "reverse-ip"):
        result = cmd_reverse(query)
    elif command in ("dns", "dnslookup", "dns-lookup", "lookup"):
        result = cmd_dns_lookup(query)
    elif command in ("hostsearch", "subdomains", "hosts"):
        result = cmd_hostsearch(query)
    elif command in ("banners", "banner"):
        result = cmd_banners(query)
    else:
        # Unknown command, default to domain lookup
        result = cmd_domain(
            query,
            page=config.get("page"),
            include_map=config.get("include_map", False),
            record_type=record_type,
        )

    try:
        print(json.dumps(result, indent=2, default=str))
        sys.exit(0 if result.get("status") == "success" else 1)
    except Exception as e:
        print(json.dumps({"status": "error", "message": f"{type(e).__name__}: {str(e)}"}))
        sys.exit(1)


if __name__ == "__main__":
    main()
