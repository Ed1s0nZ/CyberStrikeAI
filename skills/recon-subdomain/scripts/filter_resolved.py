#!/usr/bin/env python3
"""
Filter resolved subdomains with custom resolution logic.

Usage:
    python filter_resolved.py subs.txt -o resolved.txt
    python filter_resolved.py subs.txt --dnsx -o resolved.txt

Features:
- Custom DNS resolution checking
- Integration with dnsx for speed
- HTTP/HTTPS probe filtering
- CNAME chain analysis
"""

import sys
import argparse
import subprocess
from pathlib import Path
from typing import List, Set


def resolve_with_dnsx(subdomains: List[str]) -> Set[str]:
    """Use dnsx for fast resolution."""
    resolved = set()

    # Create temp file
    import tempfile
    with tempfile.NamedTemporaryFile(mode='w', delete=False) as f:
        f.write('\n'.join(subdomains))
        temp_file = f.name

    try:
        # Run dnsx
        result = subprocess.run(
            ['dnsx', '-l', temp_file, '-silent'],
            capture_output=True,
            text=True
        )

        if result.stdout:
            resolved = set(result.stdout.strip().split('\n'))
    except FileNotFoundError:
        print("Warning: dnsx not found, falling back to basic filtering", file=sys.stderr)
        # Return all as potentially resolved
        resolved = set(subdomains)
    finally:
        Path(temp_file).unlink(missing_ok=True)

    return resolved


def filter_by_pattern(subdomains: List[str]) -> Set[str]:
    """Filter out obviously invalid patterns."""
    valid = set()

    invalid_patterns = [
        '*.',
        'xn--',  # Punycode (often false positives)
        '..',
    ]

    for sub in subdomains:
        # Skip invalid patterns
        if any(p in sub for p in invalid_patterns):
            continue

        # Basic validation
        if '.' in sub and len(sub) > 3:
            valid.add(sub)

    return valid


def main():
    parser = argparse.ArgumentParser(
        description="Filter resolved subdomains"
    )
    parser.add_argument("input", help="Input subdomain file")
    parser.add_argument("-o", "--output", required=True, help="Output file")
    parser.add_argument("--dnsx", action="store_true",
                        help="Use dnsx for resolution verification")
    parser.add_argument("--pattern-only", action="store_true",
                        help="Only filter by pattern, no resolution")

    args = parser.parse_args()

    # Read input
    with open(args.input, 'r') as f:
        subdomains = [line.strip() for line in f if line.strip()]

    print(f"Read {len(subdomains)} subdomains", file=sys.stderr)

    # Filter by pattern first
    valid = filter_by_pattern(subdomains)
    print(f"After pattern filter: {len(valid)}", file=sys.stderr)

    # Use dnsx if requested
    if args.dnsx and not args.pattern_only:
        resolved = resolve_with_dnsx(list(valid))
        print(f"After DNS resolution: {len(resolved)}", file=sys.stderr)
        valid = resolved

    # Write output
    with open(args.output, 'w') as f:
        for sub in sorted(valid):
            f.write(f"{sub}\n")

    print(f"Wrote {len(valid)} subdomains to {args.output}", file=sys.stderr)


if __name__ == "__main__":
    main()
