#!/usr/bin/env python3
"""
Merge and deduplicate multiple wordlists.

Usage:
    python merge_wordlists.py wordlist1.txt wordlist2.txt -o merged.txt
    python merge_wordlists.py *.txt -o merged.txt --sort

Features:
- Deduplication
- Optional sorting
- Comment filtering
- Empty line removal
"""

import sys
import argparse
from pathlib import Path
from typing import Set


def read_wordlist(file_path: str, include_comments: bool = False) -> Set[str]:
    """Read unique entries from a wordlist file."""
    entries = set()
    try:
        with open(file_path, 'r', encoding='utf-8', errors='ignore') as f:
            for line in f:
                line = line.strip()
                if not line:
                    continue
                # Skip comments if requested
                if not include_comments and line.startswith('#'):
                    continue
                entries.add(line)
    except FileNotFoundError:
        print(f"Warning: File not found: {file_path}", file=sys.stderr)
    return entries


def main():
    parser = argparse.ArgumentParser(
        description="Merge and deduplicate wordlists"
    )
    parser.add_argument("files", nargs='+', help="Wordlist files to merge")
    parser.add_argument("-o", "--output", required=True, help="Output file")
    parser.add_argument("--sort", action="store_true", help="Sort output alphabetically")
    parser.add_argument("--include-comments", action="store_true",
                        help="Include comment lines (starting with #)")
    parser.add_argument("--lowercase", action="store_true",
                        help="Convert all entries to lowercase")

    args = parser.parse_args()

    all_entries = set()

    # Read all files
    for file_path in args.files:
        entries = read_wordlist(file_path, args.include_comments)
        print(f"Read {len(entries)} unique entries from {file_path}", file=sys.stderr)
        all_entries.update(entries)

    # Lowercase conversion
    if args.lowercase:
        all_entries = {e.lower() for e in all_entries}

    total = len(all_entries)
    print(f"\nTotal unique entries: {total}", file=sys.stderr)

    # Sort if requested
    output = sorted(all_entries) if args.sort else list(all_entries)

    # Write output
    with open(args.output, 'w', encoding='utf-8') as f:
        for entry in output:
            f.write(f"{entry}\n")

    print(f"Wrote {len(output)} entries to {args.output}", file=sys.stderr)


if __name__ == "__main__":
    main()
