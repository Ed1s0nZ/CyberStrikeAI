#!/usr/bin/env python3
"""
Subdomain Storage Script
Parse subdomain enumeration results and store hosts to database
"""

import sys
import socket
from pathlib import Path

# Add results-storage to path
sys.path.insert(0, str(Path(__file__).parent.parent.parent / 'results-storage' / 'scripts'))

from storage_api import StorageAPI


def resolve_ip(hostname):
    """Resolve hostname to IP address"""
    try:
        return socket.gethostbyname(hostname)
    except socket.gaierror:
        return None


def main():
    import argparse

    parser = argparse.ArgumentParser(
        description="Store subdomain enumeration results to database",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Store from file
  %(prog)s --input-file subdomains.txt

  # Store from file with subsystem
  %(prog)s --input-file subdomains.txt --subsystem "External Infrastructure"

  # Store from stdin
  subfinder -d example.com | %(prog)s --subsystem "External"
        """
    )

    parser.add_argument("--subsystem", help="Subsystem name (optional)")
    parser.add_argument("--input-file", help="File containing subdomains (one per line)")

    args = parser.parse_args()

    # Read subdomains
    if args.input_file:
        try:
            with open(args.input_file, 'r') as f:
                subdomains = [line.strip() for line in f if line.strip()]
        except FileNotFoundError:
            print(f"[-] File not found: {args.input_file}")
            sys.exit(1)
    else:
        subdomains = [line.strip() for line in sys.stdin if line.strip()]

    if not subdomains:
        print("[-] No subdomains to store")
        sys.exit(0)

    # Store to database
    api = StorageAPI()
    stored_count = 0
    skipped_count = 0

    # Get subsystem ID if specified
    subsystem_id = None
    if args.subsystem:
        subsystem_id = api.get_or_create_subsystem(args.subsystem)

    for subdomain in subdomains:
        # Skip if already has IP in line (format: subdomain.com,1.2.3.4)
        if ',' in subdomain:
            parts = subdomain.split(',')
            subdomain = parts[0].strip()
            ip = parts[1].strip()
        else:
            ip = resolve_ip(subdomain)

        if ip:
            try:
                # Create host entry
                api._get_or_create_host(
                    ip_address=ip,
                    hostname=subdomain,
                    subsystem_id=subsystem_id
                )
                stored_count += 1
                print(f"[+] Stored {subdomain} -> {ip}")
            except Exception as e:
                print(f"[-] Error storing {subdomain}: {e}")
        else:
            skipped_count += 1
            print(f"[-] Skipped {subdomain} (could not resolve IP)")

    print(f"\n[+] Total: {stored_count} subdomains stored, {skipped_count} skipped")


if __name__ == "__main__":
    main()
