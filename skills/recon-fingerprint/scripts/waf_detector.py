#!/usr/bin/env python3
"""
Detect WAF from HTTP headers and cookies.

Usage:
    python waf_detector.py <url>
    python waf_detector.py -i urls.txt

Features:
- Detect WAF from headers
- Detect WAF from cookies
- Detect WAF from response behavior
- Identify CDN providers
"""

import sys
import argparse
import urllib.request
import urllib.error
from typing import Dict, List, Tuple


# WAF detection patterns
WAF_PATTERNS = {
    # Headers
    'Cloudflare': [
        ('header', 'cf-ray'),
        ('header', 'cf-cache-status'),
        ('header', 'cloudflare'),
        ('header', 'cf-challenge'),
    ],
    'AWS WAF': [
        ('header', 'x-amz-cf-id'),
        ('header', 'x-amzn-requestid'),
        ('header', 'x-amz-cf-pop'),
    ],
    'Akamai': [
        ('header', 'x-akamai-transformed'),
        ('header', 'akamai-origin'),
    ],
    'Fastly': [
        ('header', 'x-served-by'),
        ('header', 'fastly-ssl'),
        ('header', 'fastly-request-id'),
    ],
    'Imperva': [
        ('header', 'x-iinfo'),
        ('header', 'x-cdn'),
        ('header', 'imperva'),
    ],
    'F5 ASM': [
        ('header', 'bigipserver'),
        ('header', 'bigip'),
    ],
    'ModSecurity': [
        ('header', 'mod_security'),
        ('header', 'modsec'),
    ],
    'Barracuda': [
        ('header', 'barra_counter_session'),
        ('header', 'barracuda'),
    ],
    'Sucuri': [
        ('header', 'x-sucuri-id'),
        ('header', 'sucuri'),
    ],
    'Incapsula': [
        ('header', 'x-incapula-id'),
        ('header', 'incapula'),
    ],
    'Azure Front Door': [
        ('header', 'x-azure-ref'),
        ('header', 'x-azurefdid'),
    ],
    'Google CDN': [
        ('header', 'x-google-cache-control'),
    ],
    'QUIC.cloud': [
        ('header', 'x-qic'),
    ],
    'SquidProxy': [
        ('header', 'x-squid'),
        ('header', 'via squid'),
    ],
    'Varnish': [
        ('header', 'x-varnish'),
        ('header', 'via varnish'),
    ],
}

# Cookie patterns
COOKIE_PATTERNS = {
    'Cloudflare': ['__cfduid', 'cf_clearance', '__cf_bm'],
    'AWS ALB': ['AWSELB', 'AWSALB', 'AWSALBCORS'],
    'F5 BIG-IP': ['BIGipServer', 'BIGipServerpool'],
    'Citrix NetScaler': ['ns_af', 'ns_cauth'],
    'Google App Engine': ['SACSID', 'SID'],
}

# Challenge pages
CHALLENGE_PATTERNS = [
    'checking your browser',
    'access denied',
    'request blocked',
    'security measure',
    'cloudflare',
    'incapsula',
    'challenge platform',
    'javascript challenge',
]


def detect_from_headers(headers: Dict[str, str]) -> List[str]:
    """Detect WAF from HTTP headers."""
    detected = []

    headers_lower = {k.lower(): v for k, v in headers.items()}

    for waf, patterns in WAF_PATTERNS.items():
        for pattern_type, pattern in patterns:
            if pattern_type == 'header':
                if pattern in ' '.join(headers_lower.keys()):
                    detected.append(waf)
                    break
                elif pattern in ' '.join(headers_lower.values()):
                    detected.append(waf)
                    break

    return list(set(detected))


def detect_from_cookies(headers: Dict[str, str]) -> List[str]:
    """Detect WAF from cookies."""
    detected = []

    cookie_header = headers.get('set-cookie', '') or headers.get('cookie', '')
    if not cookie_header:
        return []

    cookie_lower = cookie_header.lower()

    for waf, patterns in COOKIE_PATTERNS.items():
        for pattern in patterns:
            if pattern in cookie_lower:
                detected.append(waf)
                break

    return list(set(detected))


def detect_from_content(url: str) -> Tuple[List[str], str]:
    """Detect WAF by analyzing response content."""
    detected = []
    content = ''

    try:
        # First try normal request
        req = urllib.request.Request(url)
        with urllib.request.urlopen(req, timeout=10) as response:
            content = response.read().decode('utf-8', errors='ignore')
            status = response.getcode()

        # Check for challenge pages
        content_lower = content.lower()
        for pattern in CHALLENGE_PATTERNS:
            if pattern in content_lower:
                detected.append('WAF Challenge Page')
                break

    except urllib.error.HTTPError as e:
        status = e.code
        # Common WAF status codes
        if status == 403:
            detected.append('Possible WAF (403 Forbidden)')
        elif status == 429:
            detected.append('Rate Limiting')
        content = e.read().decode('utf-8', errors='ignore')
    except Exception as e:
        return [f'Error: {str(e)}'], ''

    return detected, content


def analyze_url(url: str) -> Dict:
    """Complete WAF analysis."""
    if not url.startswith(('http://', 'https://')):
        url = 'https://' + url

    result = {
        'url': url,
        'from_headers': [],
        'from_cookies': [],
        'from_content': [],
        'all_detected': [],
    }

    # Get headers
    try:
        req = urllib.request.Request(url, method='HEAD')
        with urllib.request.urlopen(req, timeout=10) as response:
            headers = dict(response.headers)
    except Exception as e:
        try:
            # HEAD not allowed, try GET
            with urllib.request.urlopen(url, timeout=10) as response:
                headers = dict(response.headers)
        except Exception:
            return {'url': url, 'error': str(e)}

    result['from_headers'] = detect_from_headers(headers)
    result['from_cookies'] = detect_from_cookies(headers)
    result['from_content'], _ = detect_from_content(url)

    result['all_detected'] = list(set(
        result['from_headers'] +
        result['from_cookies'] +
        result['from_content']
    ))

    return result


def main():
    parser = argparse.ArgumentParser(
        description="Detect WAF from headers and cookies"
    )
    parser.add_argument("target", help="Target URL or file with URLs")
    parser.add_argument("-i", "--input-file", action="store_true",
                        help="Treat target as file with URLs")
    parser.add_argument("--json", action="store_true", help="JSON output")

    args = parser.parse_args()

    results = []

    if args.input_file:
        with open(args.target, 'r') as f:
            urls = [line.strip() for line in f if line.strip()]
            for url in urls:
                results.append(analyze_url(url))
    else:
        results.append(analyze_url(args.target))

    if args.json:
        print(json.dumps(results, indent=2))
    else:
        for r in results:
            if 'error' in r:
                print(f"[!] {r['url']}: {r['error']}")
                continue

            print(f"\n[*] Target: {r['url']}")

            if r['all_detected']:
                print(f"    Detected: {', '.join(r['all_detected'])}")

                if r['from_headers']:
                    print(f"      Headers: {', '.join(r['from_headers'])}")
                if r['from_cookies']:
                    print(f"      Cookies: {', '.join(r['from_cookies'])}")
                if r['from_content']:
                    print(f"      Content: {', '.join(r['from_content'])}")
            else:
                print("    No WAF detected")


if __name__ == "__main__":
    main()
