#!/usr/bin/env python3
"""
Extract and analyze HTTP headers for web fingerprinting.

Usage:
    python extract_headers.py <url>
    python extract_headers.py <url> --json
    python extract_headers.py -i urls.txt

Features:
- Extract all HTTP headers
- Identify server technology
- Detect WAF/CDN from headers
- Analyze security headers
"""

import sys
import argparse
import json
import urllib.request
import urllib.error
from typing import Dict, List
from urllib.parse import urlparse


# Technology signatures
SERVER_SIGNATURES = {
    'nginx': ['nginx'],
    'apache': ['apache', 'httpd'],
    'iis': ['iis', 'microsoft-iis'],
    'cloudflare': ['cloudflare', 'cloudflare-server'],
    'apache-jetty': ['jetty'],
    'undertow': ['undertow'],
    'netty': ['netty'],
    'gunicorn': ['gunicorn'],
    'uwsgi': ['uwsgi'],
    'python': ['wsgiserver', 'python'],
    'node': ['express', 'node.js'],
    'php': ['php'],
}

# WAF/CDN signatures
WAF_SIGNATURES = {
    'Cloudflare': ['cf-ray', 'cf-cache-status', 'cloudflare'],
    'AWS WAF/CloudFront': ['x-amz-cf-id', 'x-amzn-requestid'],
    'Akamai': ['x-akamai-transformed', 'akamai'],
    'Fastly': ['x-served-by', 'fastly-ssl'],
    'Imperva': ['x-iinfo', 'x-cdn'],
    'F5 ASM': ['bigipserver', 'bigip'],
    'ModSecurity': ['mod_security'],
    'Barracuda': ['barra_counter_session'],
    'Sucuri': ['x-sucuri-id'],
    'Incapsula': ['x-incapula-id'],
    'Azure Front Door': ['x-azure-ref'],
    'Google CDN': ['x-google-cache-control'],
}

# Security headers
SECURITY_HEADERS = [
    'strict-transport-security',
    'x-frame-options',
    'x-content-type-options',
    'content-security-policy',
    'x-xss-protection',
    'referrer-policy',
    'permissions-policy',
]


def extract_headers(url: str) -> Dict:
    """Extract HTTP headers from URL."""
    try:
        if not url.startswith(('http://', 'https://')):
            url = 'https://' + url

        req = urllib.request.Request(url, method='HEAD')
        with urllib.request.urlopen(req, timeout=10) as response:
            headers = dict(response.headers)
            headers['status_code'] = response.getcode()
            return headers
    except urllib.error.HTTPError as e:
        return {'error': str(e), 'status_code': e.code}
    except Exception as e:
        return {'error': str(e)}


def identify_server(headers: Dict) -> List[str]:
    """Identify server technology from headers."""
    server_header = headers.get('server', '').lower()
    x_powered_by = headers.get('x-powered-by', '').lower()

    technologies = []

    for tech, signatures in SERVER_SIGNATURES.items():
        for sig in signatures:
            if sig in server_header or sig in x_powered_by:
                technologies.append(tech)
                break

    return technologies


def detect_waf(headers: Dict) -> List[str]:
    """Detect WAF/CDN from headers."""
    detected = []

    header_names = {k.lower(): k for k in headers.keys()}
    header_values = {k.lower(): str(v).lower() for k, v in headers.items()}

    for waf, signatures in WAF_SIGNATURES.items():
        for sig in signatures:
            # Check in header names
            if sig in ' '.join(header_names.keys()):
                detected.append(waf)
                break
            # Check in header values
            if sig in ' '.join(header_values.values()):
                detected.append(waf)
                break

    return detected


def check_security_headers(headers: Dict) -> Dict:
    """Check for presence of security headers."""
    present = []
    missing = []

    header_names_lower = {k.lower(): k for k in headers.keys()}

    for header in SECURITY_HEADERS:
        if header in header_names_lower:
            present.append(header)
        else:
            missing.append(header)

    return {'present': present, 'missing': missing}


def analyze_url(url: str, verbose: bool = False) -> Dict:
    """Perform complete header analysis."""
    print(f"[*] Analyzing: {url}", file=sys.stderr)

    headers = extract_headers(url)

    if 'error' in headers:
        return {'url': url, 'error': headers['error']}

    result = {
        'url': url,
        'status_code': headers.get('status_code'),
        'server': headers.get('server', ''),
        'technologies': identify_server(headers),
        'waf_cdn': detect_waf(headers),
        'security_headers': check_security_headers(headers),
    }

    if verbose:
        result['all_headers'] = dict(headers)

    return result


def main():
    parser = argparse.ArgumentParser(
        description="Extract and analyze HTTP headers"
    )
    parser.add_argument("target", help="Target URL or file with URLs")
    parser.add_argument("-i", "--input-file", action="store_true",
                        help="Treat target as file with URLs")
    parser.add_argument("--json", action="store_true",
                        help="Output in JSON format")
    parser.add_argument("-v", "--verbose", action="store_true",
                        help="Show all headers")
    parser.add_argument("-o", "--output", help="Output file")

    args = parser.parse_args()

    results = []

    if args.input_file:
        with open(args.target, 'r') as f:
            urls = [line.strip() for line in f if line.strip()]
            for url in urls:
                results.append(analyze_url(url, args.verbose))
    else:
        results.append(analyze_url(args.target, args.verbose))

    if args.json:
        output = json.dumps(results, indent=2)
    else:
        output_lines = []
        for r in results:
            if 'error' in r:
                output_lines.append(f"[!] {r['url']}: {r['error']}")
                continue

            output_lines.append(f"[*] {r['url']}")
            output_lines.append(f"    Status: {r.get('status_code', 'N/A')}")
            output_lines.append(f"    Server: {r.get('server', 'N/A')}")

            if r['technologies']:
                output_lines.append(f"    Technologies: {', '.join(r['technologies'])}")

            if r['waf_cdn']:
                output_lines.append(f"    WAF/CDN: {', '.join(r['waf_cdn'])}")

            sec = r['security_headers']
            if sec['missing']:
                output_lines.append(f"    Missing Security Headers: {', '.join(sec['missing'])}")

            output_lines.append("")

        output = '\n'.join(output_lines)

    if args.output:
        with open(args.output, 'w') as f:
            f.write(output)
        print(f"[*] Output saved to {args.output}", file=sys.stderr)
    else:
        print(output)


if __name__ == "__main__":
    main()
