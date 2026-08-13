#!/usr/bin/env python3
"""
Match technologies from HTTP responses.

Usage:
    python tech_matcher.py <response_file>
    python tech_matcher.py --url https://target.com

Features:
- Match technology patterns from HTML/JS content
- Identify frameworks and libraries
- Detect CMS and applications
"""

import sys
import argparse
import json
import urllib.request
import re
from typing import List, Dict, Set


# Technology patterns
TECH_PATTERNS = {
    # Frontend Frameworks
    'React': [
        r'react\.js',
        r'react-dom',
        r'_react',
        r'__REACT__',
        r'<div id="root">',
        r'react-router',
    ],
    'Vue.js': [
        r'vue\.js',
        r'vue-router',
        r'v-if=',
        r'v-for=',
        r'v-bind:',
        r'Vue\.config',
    ],
    'Angular': [
        r'ng-app',
        r'angular\.js',
        r'zone\.js',
        r'ngx-',
        r'\\[ngClass\\]',
    ],
    'jQuery': [
        r'jquery.*\.js',
        r'\$\(document\)',
        r'\.ajax\(',
        r'\.ready\(',
    ],
    'Bootstrap': [
        r'bootstrap\.css',
        r'bootstrap\.js',
        r'class="btn-',
        r'class="container"',
    ],
    'Tailwind': [
        r'tailwindcss',
        r'class="[(a-z):]-[',
    ],

    # Backend Frameworks
    'Express': [
        r'X-Powered-By:\s*Express',
        r'express',
    ],
    'Django': [
        r'csrfmiddlewaretoken',
        r'django',
        r'X-Powered-By:\s*Django',
    ],
    'Flask': [
        r'flask',
        r'werkzeug',
    ],
    'Laravel': [
        r'laravel',
        r'X-CSRF-TOKEN',
        r'_token',
    ],
    'Spring Boot': [
        r'spring-boot',
        r'springframework',
        r'X-Application-Context',
    ],
    'Rails': [
        r'ruby.*rails',
        r'rails\.js',
        r'csrf-param',
        r'turbo-',
    ],
    'ASP.NET': [
        r'\.aspx',
        r'X-AspNet-Version',
        r'__VIEWSTATE',
        r'ScriptManager',
    ],

    # CMS
    'WordPress': [
        r'wp-content',
        r'wp-includes',
        r'wp-json',
        r'wordpress',
    ],
    'Drupal': [
        r'Drupal\.settings',
        r'/sites/all/',
        r'drupal',
    ],
    'Joomla': [
        r'/components/com_',
        r'Joomla!',
        r'joomla',
    ],
    'Ghost': [
        r'ghost-url',
        r'casper',
    ],
    'Shopify': [
        r'Shopify\.theme',
        r'shopify',
    ],

    # Analytics & Tracking
    'Google Analytics': [
        r'googletagmanager\.com',
        r'ga\(',
        r'gtag\(',
        r'UA-\\d+-\\d+',
    ],
    'Hotjar': [
        r'hotjar\.com',
        r'hj\(',
    ],
    'Segment': [
        r'segment\.com',
        r'analytics\.load\(',
    ],

    # CDNs
    'Cloudflare': [
        r'cloudflare',
        r'cf-cache-status',
        r'cf-ray',
    ],
    'AWS CloudFront': [
        r'cloudfront',
        r'x-amz-cf-id',
    ],
    'Akamai': [
        r'akamai',
        r'x-akamai',
    ],

    # Web Servers
    'nginx': [
        r'server:\s*nginx',
        r'nginx',
    ],
    'Apache': [
        r'server:\s*apache',
        r'apache',
    ],
    'IIS': [
        r'server:\s*iis',
        r'microsoft-iis',
    ],
}


def match_technologies(content: str) -> Dict[str, List[str]]:
    """Match technologies from content."""
    matches = {}

    content_lower = content.lower()

    for tech, patterns in TECH_PATTERNS.items():
        matched_patterns = []
        for pattern in patterns:
            if re.search(pattern, content, re.IGNORECASE):
                matched_patterns.append(pattern)

        if matched_patterns:
            matches[tech] = matched_patterns

    return matches


def fetch_url(url: str) -> str:
    """Fetch URL content."""
    if not url.startswith(('http://', 'https://')):
        url = 'https://' + url

    req = urllib.request.Request(url, headers={'User-Agent': 'Mozilla/5.0'})
    with urllib.request.urlopen(req, timeout=10) as response:
        return response.read().decode('utf-8', errors='ignore')


def match_from_file(file_path: str) -> Dict:
    """Match technologies from file."""
    with open(file_path, 'r') as f:
        content = f.read()
    return match_technologies(content)


def main():
    parser = argparse.ArgumentParser(
        description="Match technologies from HTTP responses"
    )
    parser.add_argument("--url", help="Target URL to fetch")
    parser.add_argument("--file", help="File with HTML content")
    parser.add_argument("--json", action="store_true", help="JSON output")
    parser.add_argument("--summary", action="store_true", help="Summary only")

    args = parser.parse_args()

    if args.url:
        print(f"[*] Fetching: {args.url}", file=sys.stderr)
        content = fetch_url(args.url)
        matches = match_technologies(content)
    elif args.file:
        matches = match_from_file(args.file)
    else:
        # Read from stdin
        content = sys.stdin.read()
        matches = match_technologies(content)

    if args.summary:
        print(f"Found {len(matches)} technologies:")
        for tech in sorted(matches.keys()):
            print(f"  - {tech}")
    elif args.json:
        print(json.dumps(matches, indent=2))
    else:
        print("=" * 60)
        print("Technology Detection Results")
        print("=" * 60)

        if not matches:
            print("\nNo technologies detected.")
        else:
            for tech, patterns in sorted(matches.items()):
                print(f"\n[*] {tech}")
                print(f"    Patterns matched: {len(patterns)}")
                for p in patterns[:3]:
                    print(f"      - {p}")
                if len(patterns) > 3:
                    print(f"      ... and {len(patterns) - 3} more")


if __name__ == "__main__":
    main()
