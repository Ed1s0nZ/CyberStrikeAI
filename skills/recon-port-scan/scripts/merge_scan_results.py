#!/usr/bin/env python3
"""
Merge multiple nmap scan results into a combined report.

Usage:
    python merge_scan_results.py <scan1.xml> <scan2.xml> ... -o merged.json
    python merge_scan_results.py *.xml -o merged.json --summary

Supports:
- Multiple XML files (nmap -oX format)
- Combines results from different scans
- Deduplicates ports per host
- Generates comprehensive summary
"""

import sys
import json
import xml.etree.ElementTree as ET
from pathlib import Path
from typing import Dict, List, Any, Set
from collections import defaultdict


def parse_nmap_xml(xml_file: str) -> Dict[str, Any]:
    """Parse a single nmap XML file."""
    tree = ET.parse(xml_file)
    root = tree.getroot()

    results = {
        "source_file": xml_file,
        "hosts": {}
    }

    for host in root.findall(".//host"):
        # Get primary address
        addr_elem = host.find("./address[@addrtype='ipv4']")
        if addr_elem is None:
            addr_elem = host.find("./address")

        if addr_elem is None:
            continue

        ip = addr_elem.get("addr", "")
        if not ip:
            continue

        # Initialize host data
        if ip not in results["hosts"]:
            results["hosts"][ip] = {
                "address": ip,
                "addresses": [],
                "status": "down",
                "ports": {"tcp": {}, "udp": {}, "ip": {}},
                "os": [],
                "scripts": [],
                "hostnames": []
            }

        host_data = results["hosts"][ip]

        # Status
        status = host.find("./status")
        if status is not None:
            host_data["status"] = status.get("state", host_data["status"])

        # Addresses
        for addr in host.findall("./address"):
            addr_info = {
                "addr": addr.get("addr", ""),
                "type": addr.get("addrtype", "ipv4")
            }
            if addr_info not in host_data["addresses"]:
                host_data["addresses"].append(addr_info)

        # Hostnames
        for hostname in host.findall(".//hostname"):
            hostname_info = {
                "name": hostname.get("name", ""),
                "type": hostname.get("type", "")
            }
            if hostname_info not in host_data["hostnames"]:
                host_data["hostnames"].append(hostname_info)

        # Ports
        for port in host.findall(".//port"):
            protocol = port.get("protocol", "tcp")
            port_id = port.get("portid", "")

            state = port.find("./state")
            service = port.find("./service")

            port_key = f"{port_id}/{protocol}"

            port_info = {
                "port": port_id,
                "protocol": protocol,
                "state": state.get("state", "unknown") if state is not None else "unknown",
                "service": {}
            }

            if service is not None:
                port_info["service"] = {
                    "name": service.get("name", ""),
                    "product": service.get("product", ""),
                    "version": service.get("version", ""),
                    "extrainfo": service.get("extrainfo", "")
                }

            # Merge with existing port info (newer scan overrides if more detailed)
            existing = host_data["ports"][protocol].get(port_key, {})
            if existing:
                # Keep the most detailed service info
                if not port_info["service"].get("name") and existing.get("service", {}).get("name"):
                    port_info["service"] = existing["service"]

            host_data["ports"][protocol][port_key] = port_info

        # OS detection
        for osmatch in host.findall(".//osmatch"):
            os_data = {
                "name": osmatch.get("name", ""),
                "accuracy": osmatch.get("accuracy", "")
            }
            if os_data not in host_data["os"]:
                host_data["os"].append(os_data)

        # Scripts
        for script in host.findall(".//script"):
            script_data = {
                "id": script.get("id", ""),
                "output": script.get("output", "")
            }
            existing_scripts = [s["id"] for s in host_data["scripts"]]
            if script_data["id"] not in existing_scripts:
                host_data["scripts"].append(script_data)

    return results


def merge_scans(scan_files: List[str]) -> Dict[str, Any]:
    """Merge multiple scan results."""
    merged = {
        "scan_files": scan_files,
        "hosts": {}
    }

    for scan_file in scan_files:
        try:
            scan_data = parse_nmap_xml(scan_file)
        except ET.ParseError:
            print(f"Warning: Could not parse {scan_file}", file=sys.stderr)
            continue
        except FileNotFoundError:
            print(f"Warning: File not found {scan_file}", file=sys.stderr)
            continue

        # Merge hosts
        for ip, host_data in scan_data["hosts"].items():
            if ip not in merged["hosts"]:
                merged["hosts"][ip] = host_data
            else:
                # Merge ports
                for proto in ["tcp", "udp", "ip"]:
                    merged["hosts"][ip]["ports"][proto].update(
                        host_data["ports"][proto]
                    )

                # Merge OS matches (keep unique)
                for os_match in host_data["os"]:
                    if os_match not in merged["hosts"][ip]["os"]:
                        merged["hosts"][ip]["os"].append(os_match)

                # Merge scripts (keep unique by id)
                existing_script_ids = {s["id"] for s in merged["hosts"][ip]["scripts"]}
                for script in host_data["scripts"]:
                    if script["id"] not in existing_script_ids:
                        merged["hosts"][ip]["scripts"].append(script)

                # Merge hostnames
                for hostname in host_data["hostnames"]:
                    if hostname not in merged["hosts"][ip]["hostnames"]:
                        merged["hosts"][ip]["hostnames"].append(hostname)

    return merged


def format_summary(merged: Dict[str, Any]) -> str:
    """Generate human-readable summary."""
    lines = []
    lines.append("=" * 70)
    lines.append("MERGED SCAN RESULTS SUMMARY")
    lines.append("=" * 70)
    lines.append(f"Source files: {len(merged['scan_files'])}")
    lines.append(f"Unique hosts: {len(merged['hosts'])}")
    lines.append("")

    for ip in sorted(merged["hosts"].keys()):
        host = merged["hosts"][ip]
        if host["status"] != "up":
            continue

        lines.append(f"Host: {ip}")
        lines.append(f"Status: {host['status']}")

        # Count ports
        tcp_open = len([p for p in host["ports"]["tcp"].values() if p["state"] == "open"])
        udp_open = len([p for p in host["ports"]["udp"].values() if p["state"] == "open"])
        lines.append(f"Open ports: {tcp_open} TCP, {udp_open} UDP")

        # Show open TCP ports
        tcp_ports = [(p["port"], p) for p in host["ports"]["tcp"].values() if p["state"] == "open"]
        if tcp_ports:
            lines.append(f"\n  TCP Ports:")
            for port, port_info in sorted(tcp_ports, key=lambda x: int(x[0])):
                svc = port_info.get("service", {})
                svc_name = svc.get("name", "unknown")
                line = f"    {port}/tcp - {svc_name}"
                if svc.get("product"):
                    line += f" ({svc['product']}"
                    if svc.get("version"):
                        line += f" {svc['version']}"
                    line += ")"
                lines.append(line)

        # Show open UDP ports
        udp_ports = [(p["port"], p) for p in host["ports"]["udp"].values() if p["state"] == "open"]
        if udp_ports:
            lines.append(f"\n  UDP Ports:")
            for port, port_info in sorted(udp_ports, key=lambda x: int(x[0])):
                svc = port_info.get("service", {})
                svc_name = svc.get("name", "unknown")
                lines.append(f"    {port}/udp - {svc_name}")

        lines.append("")

    lines.append("=" * 70)
    return "\n".join(lines)


def main():
    if len(sys.argv) < 3:
        print("Usage: python merge_scan_results.py <scan1.xml> <scan2.xml> ... -o output.json")
        print("\nOptions:")
        print("  -o, --output FILE    Output JSON file")
        print("  --summary            Show summary instead of full JSON")
        print("\nExamples:")
        print("  python merge_scan_results.py scan1.xml scan2.xml -o merged.json")
        print("  python merge_scan_results.py *.xml -o merged.json --summary")
        sys.exit(1)

    # Parse arguments
    scan_files = []
    output_file = None
    show_summary = False

    for i in range(1, len(sys.argv)):
        arg = sys.argv[i]
        if arg in ["-o", "--output"]:
            if i + 1 < len(sys.argv):
                output_file = sys.argv[i + 1]
        elif arg == "--summary":
            show_summary = True
        elif arg.endswith(".xml"):
            scan_files.append(arg)

    if not scan_files:
        print("Error: No XML files specified", file=sys.stderr)
        sys.exit(1)

    # Merge scans
    print(f"Merging {len(scan_files)} scan files...", file=sys.stderr)
    merged = merge_scans(scan_files)

    # Output
    if show_summary:
        print(format_summary(merged))

    if output_file:
        with open(output_file, 'w') as f:
            json.dump(merged, f, indent=2)
        print(f"\nResults saved to: {output_file}", file=sys.stderr)
    elif not show_summary:
        print(json.dumps(merged, indent=2))


if __name__ == "__main__":
    main()
