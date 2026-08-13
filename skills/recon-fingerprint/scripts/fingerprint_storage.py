#!/usr/bin/env python3
"""
Fingerprint Storage Script
Parse web fingerprinting results and store to database
"""

import sys
import json
from pathlib import Path

# Add results-storage to path
sys.path.insert(0, str(Path(__file__).parent.parent.parent / 'results-storage' / 'scripts'))

from storage_api import StorageAPI


def main():
    import argparse

    parser = argparse.ArgumentParser(
        description="Store web fingerprinting results to database",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  %(prog)s --host-ip 192.168.1.100 --url "https://example.com" \\
        --technology "Apache 2.4.41" --category "web-server"

  %(prog)s --host-ip 192.168.1.100 --url "https://example.com" \\
        --technology "WordPress 5.8" --category "cms" --subsystem "Web App"
        """
    )

    parser.add_argument("--subsystem", help="Subsystem name (optional)")
    parser.add_argument("--host-ip", required=True, help="Target host IP")
    parser.add_argument("--url", required=True, help="Target URL")
    parser.add_argument("--technology", required=True, help="Discovered technology")
    parser.add_argument("--category", help="Technology category (web-server, cms, framework, etc.)")
    parser.add_argument("--version", help="Technology version")
    parser.add_argument("--confidence", help="Confidence level")

    args = parser.parse_args()

    # Store fingerprint info as host metadata
    try:
        api = StorageAPI()

        # Get or create host with OS fingerprint set to technology stack
        tech_str = args.technology
        if args.version:
            tech_str += f" {args.version}"
        if args.category:
            tech_str += f" ({args.category})"

        subsystem_id = None
        if args.subsystem:
            subsystem_id = api.get_or_create_subsystem(args.subsystem)

        host_id = api._get_or_create_host(
            ip_address=args.host_ip,
            os_fingerprint=tech_str,
            subsystem_id=subsystem_id
        )

        print(f"[+] Stored fingerprint for {args.host_ip}")
        print(f"    Technology: {args.technology}")
        print(f"    Version: {args.version}" if args.version else "")
        print(f"    Category: {args.category}" if args.category else "")

        if args.subsystem:
            print(f"    Subsystem: {args.subsystem}")

    except Exception as e:
        print(f"[-] Error storing fingerprint: {e}")
        sys.exit(1)


if __name__ == "__main__":
    main()
