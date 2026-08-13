#!/usr/bin/env python3
"""
Generate statistics on discovered subdomains.

Usage:
    python subdomain_stats.py subs.txt
    python subdomain_stats.py subs.txt --top-n 50

Features:
- Subdomain level analysis (1st, 2nd, 3rd level)
- Common subdomain word frequency
- Root domain grouping
- IP resolution summary
"""

import sys
import argparse
from collections import Counter
from typing import Dict, List, Tuple
from pathlib import Path


def get_subdomain_level(subdomain: str) -> int:
    """Get the level of a subdomain (1 = direct subdomain, 2 = sub-subdomain, etc.)."""
    parts = subdomain.split('.')
    return len(parts) - 2  # -2 for root domain (domain.tld)


def get_base_domain(subdomain: str) -> str:
    """Extract base domain (root + TLD)."""
    parts = subdomain.split('.')
    if len(parts) >= 2:
        return '.'.join(parts[-2:])
    return subdomain


def get_subdomain_word(subdomain: str) -> str:
    """Extract the first-level subdomain word."""
    parts = subdomain.split('.')
    if len(parts) >= 3:
        return parts[0]
    return ''


def analyze_subdomains(subdomains: List[str]) -> Dict:
    """Analyze subdomain list and generate statistics."""
    stats = {
        'total': len(subdomains),
        'unique': len(set(subdomains)),
        'levels': Counter(),
        'base_domains': Counter(),
        'common_words': Counter(),
    }

    unique_subs = set(subdomains)

    for sub in unique_subs:
        # Count levels
        level = get_subdomain_level(sub)
        stats['levels'][level] += 1

        # Count base domains
        base = get_base_domain(sub)
        stats['base_domains'][base] += 1

        # Count common words
        word = get_subdomain_word(sub)
        if word:
            stats['common_words'][word] += 1

    return stats


def main():
    parser = argparse.ArgumentParser(
        description="Generate subdomain statistics"
    )
    parser.add_argument("input", help="Input subdomain file")
    parser.add_argument("--top-n", type=int, default=20,
                        help="Number of top results to show")
    parser.add_argument("--json", action="store_true",
                        help="Output in JSON format")

    args = parser.parse_args()

    # Read input
    with open(args.input, 'r') as f:
        subdomains = [line.strip() for line in f if line.strip()]

    # Analyze
    stats = analyze_subdomains(subdomains)

    if args.json:
        import json
        # Convert Counters to dicts for JSON serialization
        output = {
            'total': stats['total'],
            'unique': stats['unique'],
            'levels': dict(stats['levels']),
            'base_domains': dict(stats['base_domains'].most_common(args.top_n)),
            'common_words': dict(stats['common_words'].most_common(args.top_n)),
        }
        print(json.dumps(output, indent=2))
    else:
        print("=" * 60)
        print("SUBDOMAIN STATISTICS")
        print("=" * 60)
        print(f"\nTotal entries: {stats['total']}")
        print(f"Unique subdomains: {stats['unique']}")

        print("\n--- Subdomain Levels ---")
        for level in sorted(stats['levels'].keys()):
            print(f"  Level {level}: {stats['levels'][level]}")

        print(f"\n--- Top {args.top_n} Base Domains ---")
        for domain, count in stats['base_domains'].most_common(args.top_n):
            print(f"  {domain}: {count}")

        print(f"\n--- Top {args.top_n} Common Subdomain Words ---")
        for word, count in stats['common_words'].most_common(args.top_n):
            print(f"  {word}: {count}")

        print("\n" + "=" * 60)


if __name__ == "__main__":
    main()
