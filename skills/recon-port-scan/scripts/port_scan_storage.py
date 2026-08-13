#!/usr/bin/env python3
"""
Port Scan Results Storage Script
Parse nmap XML output and store to database
"""

import sys
import xml.etree.ElementTree as ET
from pathlib import Path

# Add results-storage to path
sys.path.insert(0, str(Path(__file__).parent.parent.parent / 'results-storage' / 'scripts'))

from storage_api import StorageAPI


def parse_nmap_xml(xml_content):
    """Parse nmap XML output and extract port information."""
    try:
        root = ET.fromstring(xml_content)
    except ET.ParseError as e:
        print(f"[-] Error parsing XML: {e}")
        return []

    results = []

    for host in root.findall(".//host"):
        # Skip hosts that are down
        status = host.find("./status")
        if status is not None and status.get("state") != "up":
            continue

        # Get IP address
        ip_elem = host.find("./address[@addrtype='ipv4']")
        if ip_elem is None:
            continue

        ip = ip_elem.get("addr")

        # Get hostname if available
        hostname = None
        hostname_elem = host.find("./hostnames/hostname")
        if hostname_elem is not None:
            hostname = hostname_elem.get("name")

        # Get OS fingerprint if available
        os_fingerprint = None
        os_elem = host.find("./os/osmatch")
        if os_elem is not None:
            os_fingerprint = os_elem.get("name", "")

        # Extract ports
        ports_data = []
        for port in host.findall(".//port"):
            port_info = {
                "port": int(port.get("portid")),
                "protocol": port.get("protocol"),
                "state": port.find("./state").get("state"),
            }

            # Add service information if available
            service = port.find("./service")
            if service is not None:
                port_info.update({
                    "service": service.get("name"),
                    "version": service.get("version"),
                    "product": service.get("product"),
                    "extra_info": service.get("extrainfo")
                })

            ports_data.append(port_info)

        if ports_data:
            results.append({
                "ip": ip,
                "hostname": hostname,
                "os_fingerprint": os_fingerprint,
                "ports": ports_data
            })

    return results


def main():
    import argparse

    parser = argparse.ArgumentParser(
        description="Store port scan results to database",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Store from XML file
  %(prog)s --xml-file scan.xml

  # Store from XML file with subsystem
  %(prog)s --xml-file scan.xml --subsystem "External Network"

  # Store from stdin
  nmap -sV -p- target.com -oX - | %(prog)s
        """
    )

    parser.add_argument("--subsystem", help="Subsystem name (optional)")
    parser.add_argument("--scan-tool", default="nmap",
                       help="Scan tool (default: nmap)")
    parser.add_argument("--xml-file", help="Nmap XML file path")

    args = parser.parse_args()

    # Read XML (from file or stdin)
    if args.xml_file:
        try:
            with open(args.xml_file, 'r') as f:
                xml_content = f.read()
        except FileNotFoundError:
            print(f"[-] File not found: {args.xml_file}")
            sys.exit(1)
    else:
        xml_content = sys.stdin.read()

    # Parse scan results
    scan_results = parse_nmap_xml(xml_content)

    if not scan_results:
        print("[-] No host results found in XML")
        sys.exit(0)

    # Store to database
    api = StorageAPI()
    total_ports = 0

    for host_result in scan_results:
        try:
            api.store_port_scan(
                host_ip=host_result["ip"],
                ports=host_result["ports"],
                scan_tool=args.scan_tool,
                subsystem=args.subsystem
            )

            total_ports += len(host_result["ports"])

            hostname_str = f" ({host_result['hostname']})" if host_result['hostname'] else ""
            print(f"[+] Stored {len(host_result['ports'])} ports for {host_result['ip']}{hostname_str}")

        except Exception as e:
            print(f"[-] Error storing results for {host_result['ip']}: {e}")

    print(f"[+] Total: {total_ports} ports stored for {len(scan_results)} hosts")


if __name__ == "__main__":
    main()
