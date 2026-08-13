#!/usr/bin/env python3
"""
Analyze HTTP response patterns from directory scanning.

Usage:
    python status_code_analyzer.py results.json
    python status_code_analyzer.py results.json --cluster-size

Features:
- Status code distribution
- Response size clustering
- Potential false positive detection
- Interesting findings summary
"""

import sys
import json
import argparse
from collections import Counter, defaultdict
from typing import List, Dict, Tuple


def parse_ffuf_json(file_path: str) -> List[Dict]:
    """Parse ffuf JSON output."""
    try:
        with open(file_path, 'r') as f:
            data = json.load(f)
        return data.get('results', [])
    except (json.JSONDecodeError, FileNotFoundError):
        return []


def analyze_status_codes(results: List[Dict]) -> Dict:
    """Analyze status code distribution."""
    status_dist = Counter()
    status_urls = defaultdict(list)

    for r in results:
        status = r.get('status', 0)
        status_dist[status] += 1
        url = r.get('result', {}).get('url', r.get('url', ''))
        status_urls[status].append(url)

    return {
        'distribution': dict(status_dist),
        'urls': dict(status_urls)
    }


def analyze_response_sizes(results: List[Dict]) -> Dict:
    """Analyze response size patterns."""
    sizes = [r.get('length', 0) for r in results]
    size_counter = Counter(sizes)

    # Find clusters (same size = potentially same content)
    clusters = defaultdict(list)
    for r in results:
        size = r.get('length', 0)
        url = r.get('result', {}).get('url', r.get('url', ''))
        clusters[size].append(url)

    # Find potential false positives (many results with same size)
    false_positive_sizes = [size for size, urls in clusters.items() if len(urls) > 10]

    return {
        'unique_sizes': len(size_counter),
        'size_distribution': dict(size_counter.most_common(10)),
        'clusters': {k: len(v) for k, v in clusters.items()},
        'potential_false_positives': false_positive_sizes
    }


def find_interesting_results(results: List[Dict]) -> List[Dict]:
    """Find potentially interesting results."""
    interesting = []

    for r in results:
        status = r.get('status', 0)
        size = r.get('length', 0)
        url = r.get('result', {}).get('url', r.get('url', ''))

        # Interesting status codes
        if status in [200, 301, 302, 403, 401]:
            interesting.append({
                'url': url,
                'status': status,
                'size': size,
                'reason': f'Status {status}'
            })

        # Large responses (potential data)
        if size > 10000:
            interesting.append({
                'url': url,
                'status': status,
                'size': size,
                'reason': f'Large response ({size} bytes)'
            })

    return interesting


def main():
    parser = argparse.ArgumentParser(
        description="Analyze directory scan results"
    )
    parser.add_argument("input", help="ffuf JSON output file")
    parser.add_argument("--cluster-size", action="store_true",
                        help="Show response size clusters")
    parser.add_argument("--show-urls", action="store_true",
                        help="Show URLs for each category")
    parser.add_argument("--top", type=int, default=20,
                        help="Number of top results to show")

    args = parser.parse_args()

    results = parse_ffuf_json(args.input)
    if not results:
        print("No results found", file=sys.stderr)
        sys.exit(1)

    print(f"Analyzing {len(results)} results...\n")

    # Status code analysis
    status_analysis = analyze_status_codes(results)
    print("=" * 60)
    print("STATUS CODE DISTRIBUTION")
    print("=" * 60)
    for status, count in sorted(status_analysis['distribution'].items()):
        print(f"  {status}: {count}")

    # Response size analysis
    size_analysis = analyze_response_sizes(results)
    print(f"\nUnique response sizes: {size_analysis['unique_sizes']}")

    if size_analysis['potential_false_positives']:
        print(f"\nPotential false positive sizes (>10 results):")
        for size in size_analysis['potential_false_positives'][:5]:
            print(f"  {size} bytes")

    if args.cluster_size:
        print("\n" + "=" * 60)
        print("RESPONSE SIZE CLUSTERS")
        print("=" * 60)
        for size, count in sorted(size_analysis['clusters'].items(),
                                   key=lambda x: x[1], reverse=True)[:10]:
            print(f"  {size} bytes: {count} results")

    # Interesting results
    interesting = find_interesting_results(results)[:args.top]
    if interesting:
        print("\n" + "=" * 60)
        print("INTERESTING RESULTS")
        print("=" * 60)
        for item in interesting:
            print(f"  [{item['status']}] {item['url']}")
            print(f"    Reason: {item['reason']}, Size: {item['size']}")

    # Show URLs by status
    if args.show_urls:
        print("\n" + "=" * 60)
        print("URLS BY STATUS CODE")
        print("=" * 60)
        for status in sorted(status_analysis['urls'].keys()):
            urls = status_analysis['urls'][status]
            print(f"\n{status} ({len(urls)} urls):")
            for url in urls[:10]:
                print(f"  {url}")
            if len(urls) > 10:
                print(f"  ... and {len(urls) - 10} more")


if __name__ == "__main__":
    main()
