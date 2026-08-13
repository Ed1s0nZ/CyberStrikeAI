#!/usr/bin/env python3
"""
Parse and filter ffuf JSON results.

Usage:
    python ffuf_results_parser.py results.json
    python ffuf_results_parser.py results.json --status 200,301
    python ffuf_results_parser.py results.json --min-size 1000

Features:
- Filter by status codes
- Filter by response size
- Extract URLs
- Generate summary statistics
"""

import sys
import json
import argparse
from typing import List, Dict


def parse_ffuf_json(file_path: str) -> List[Dict]:
    """Parse ffuf JSON output file."""
    try:
        with open(file_path, 'r') as f:
            data = json.load(f)
        return data.get('results', [])
    except json.JSONDecodeError:
        print(f"Error: Invalid JSON in {file_path}", file=sys.stderr)
        return []
    except FileNotFoundError:
        print(f"Error: File not found: {file_path}", file=sys.stderr)
        return []


def filter_by_status(results: List[Dict], status_codes: List[int]) -> List[Dict]:
    """Filter results by status code."""
    return [r for r in results if r['status'] in status_codes]


def filter_by_size(results: List[Dict], min_size: int = None, max_size: int = None) -> List[Dict]:
    """Filter results by response size."""
    filtered = results
    if min_size is not None:
        filtered = [r for r in filtered if r['length'] >= min_size]
    if max_size is not None:
        filtered = [r for r in filtered if r['length'] <= max_size]
    return filtered


def extract_urls(results: List[Dict], base_url: str = "") -> List[str]:
    """Extract URLs from results."""
    urls = []
    for r in results:
        url = r.get('result', {}).get('url', r.get('url', ''))
        if url and base_url:
            # Handle FFUF keyword replacement
            if 'FUZZ' in base_url:
                url = base_url.replace('FUZZ', r.get('input', {}).get('FUZZ', ''))
        urls.append(url)
    return urls


def generate_summary(results: List[Dict]) -> Dict:
    """Generate summary statistics."""
    summary = {
        'total': len(results),
        'by_status': {},
        'by_size': {},
        'unique_sizes': set(),
    }

    for r in results:
        status = r.get('status', 0)
        size = r.get('length', 0)

        summary['by_status'][status] = summary['by_status'].get(status, 0) + 1
        summary['unique_sizes'].add(size)

    summary['unique_sizes'] = len(summary['unique_sizes'])
    return summary


def main():
    parser = argparse.ArgumentParser(
        description="Parse and filter ffuf JSON results"
    )
    parser.add_argument("input", help="ffuf JSON output file")
    parser.add_argument("--status", help="Filter by status codes (comma-separated)")
    parser.add_argument("--min-size", type=int, help="Minimum response size")
    parser.add_argument("--max-size", type=int, help="Maximum response size")
    parser.add_argument("--urls", action="store_true", help="Extract URLs only")
    parser.add_argument("--summary", action="store_true", help="Show summary only")
    parser.add_argument("-o", "--output", help="Output file")

    args = parser.parse_args()

    # Parse results
    results = parse_ffuf_json(args.input)
    if not results:
        print("No results found or error parsing file", file=sys.stderr)
        sys.exit(1)

    print(f"Loaded {len(results)} results", file=sys.stderr)

    # Apply filters
    if args.status:
        status_codes = [int(s.strip()) for s in args.status.split(',')]
        results = filter_by_status(results, status_codes)
        print(f"After status filter: {len(results)} results", file=sys.stderr)

    if args.min_size or args.max_size:
        results = filter_by_size(results, args.min_size, args.max_size)
        print(f"After size filter: {len(results)} results", file=sys.stderr)

    # Generate output
    output_lines = []

    if args.summary:
        summary = generate_summary(results)
        output_lines.append("=== FFUF Results Summary ===")
        output_lines.append(f"Total results: {summary['total']}")
        output_lines.append(f"Unique response sizes: {summary['unique_sizes']}")
        output_lines.append("\nBy Status Code:")
        for status, count in sorted(summary['by_status'].items()):
            output_lines.append(f"  {status}: {count}")
    elif args.urls:
        urls = [r.get('result', {}).get('url', r.get('url', '')) for r in results]
        output_lines.extend(urls)
    else:
        # Detailed output
        for r in results:
            url = r.get('result', {}).get('url', r.get('url', ''))
            status = r.get('status', 0)
            size = r.get('length', 0)
            words = r.get('words', 0)
            lines = r.get('lines', 0)
            output_lines.append(f"[{status}] [{size}b] {url}")

    # Write output
    output = '\n'.join(output_lines)

    if args.output:
        with open(args.output, 'w') as f:
            f.write(output)
        print(f"Output written to {args.output}", file=sys.stderr)
    else:
        print(output)


if __name__ == "__main__":
    main()
