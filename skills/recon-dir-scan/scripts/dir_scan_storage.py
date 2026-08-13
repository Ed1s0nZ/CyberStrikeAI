#!/usr/bin/env python3
"""
Directory Scan Storage Script
Parse directory enumeration results and store findings
"""

import sys
from pathlib import Path

# Add results-storage to path
sys.path.insert(0, str(Path(__file__).parent.parent.parent / 'results-storage' / 'scripts'))

from storage_api import StorageAPI


def main():
    import argparse

    parser = argparse.ArgumentParser(
        description="Store directory scan findings to database",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  %(prog)s --host-ip 192.168.1.100 --url "https://example.com" \\
        --path "/admin" --status 200 --size 1234

  %(prog)s --host-ip 192.168.1.100 --url "https://example.com" \\
        --path "/backup.zip" --status 200 --subsystem "Web App"
        """
    )

    parser.add_argument("--subsystem", help="Subsystem name (optional)")
    parser.add_argument("--host-ip", required=True, help="Target host IP")
    parser.add_argument("--url", required=True, help="Base URL")
    parser.add_argument("--path", required=True, help="Discovered path")
    parser.add_argument("--status", type=int, help="HTTP status code")
    parser.add_argument("--size", type=int, help="Response size")
    parser.add_argument("--tool", default="ffuf", help="Tool used (default: ffuf)")

    args = parser.parse_args()

    # Store as info disclosure or file download finding
    try:
        api = StorageAPI()

        # Build full URL
        full_url = args.url.rstrip('/') + '/' + args.path.lstrip('/')

        # Store as info disclosure finding
        vuln_id = api.store_vulnerability(
            host_ip=args.host_ip,
            vuln_type="info_disclosure",
            severity="Info",
            title=f"Exposed directory: {args.path}",
            description=f"Discovered directory or file at {full_url}",
            affected_component=args.path,
            subsystem=args.subsystem,
            discovered_by_skill=args.tool,
            url=full_url
        )

        print(f"[+] Stored directory finding (ID: {vuln_id})")
        print(f"    Host: {args.host_ip}")
        print(f"    Path: {args.path}")
        print(f"    Status: {args.status}" if args.status else "")
        print(f"    Size: {args.size} bytes" if args.size else "")

        if args.subsystem:
            print(f"    Subsystem: {args.subsystem}")

    except Exception as e:
        print(f"[-] Error storing finding: {e}")
        sys.exit(1)


if __name__ == "__main__":
    main()
