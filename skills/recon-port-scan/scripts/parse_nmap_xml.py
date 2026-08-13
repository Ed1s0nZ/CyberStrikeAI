#!/usr/bin/env python3
"""
Parse nmap XML output to structured JSON format.

Usage:
    python parse_nmap_xml.py <nmap_xml_file>
    python parse_nmap_xml.py scan.xml -o output.json

Output includes:
- Open ports with protocols
- Service versions
- OS detection results
- Host status (up/down)
- Script execution results
"""

import sys
import json
import xml.etree.ElementTree as ET
from pathlib import Path
from typing import Dict, List, Any


def parse_nmap_xml(xml_file: str) -> Dict[str, Any]:
    """
    Parse nmap XML output file.

    Args:
        xml_file: Path to nmap XML file

    Returns:
        Dictionary containing parsed scan results
    """
    tree = ET.parse(xml_file)
    root = tree.getroot()

    results = {
        "command": root.get("command", ""),
        "args": root.get("args", ""),
        "start": root.get("start", ""),
        "hosts": []
    }

    for host in root.findall(".//host"):
        host_data = {
            "status": "",
            "address": "",
            "addresses": [],
            "hostnames": [],
            "ports": {"tcp": [], "udp": [], "ip": []},
            "os": [],
            "scripts": []
        }

        # Get status
        status = host.find("./status")
        if status is not None:
            host_data["status"] = status.get("state", "unknown")

        # Get addresses
        for addr in host.findall("./address"):
            addr_info = {
                "addr": addr.get("addr", ""),
                "type": addr.get("addrtype", "ipv4")
            }
            if addr_info["type"] == "ipv4":
                host_data["address"] = addr_info["addr"]
            host_data["addresses"].append(addr_info)

        # Get hostnames
        for hostname in host.findall(".//hostname"):
            host_data["hostnames"].append({
                "name": hostname.get("name", ""),
                "type": hostname.get("type", "")
            })

        # Get ports
        for port in host.findall(".//port"):
            protocol = port.get("protocol", "tcp")
            port_id = port.get("portid", "")

            state = port.find("./state")
            service = port.find("./service")

            port_info = {
                "port": int(port_id) if port_id.isdigit() else port_id,
                "protocol": protocol,
                "state": state.get("state", "unknown") if state is not None else "unknown",
                "reason": state.get("reason", "") if state is not None else ""
            }

            if service is not None:
                port_info["service"] = {
                    "name": service.get("name", ""),
                    "product": service.get("product", ""),
                    "version": service.get("version", ""),
                    "extrainfo": service.get("extrainfo", ""),
                    "method": service.get("method", ""),
                    "conf": service.get("conf", "")
                }

            host_data["ports"][protocol].append(port_info)

        # Get OS detection
        for osmatch in host.findall(".//osmatch"):
            os_data = {
                "name": osmatch.get("name", ""),
                "accuracy": osmatch.get("accuracy", ""),
                "line": osmatch.get("line", "")
            }
            host_data["os"].append(os_data)

        # Get script results
        for script in host.findall(".//script"):
            script_data = {
                "id": script.get("id", ""),
                "output": script.get("output", "")
            }
            host_data["scripts"].append(script_data)

        results["hosts"].append(host_data)

    return results


def format_summary(results: Dict[str, Any]) -> str:
    """Generate human-readable summary of scan results."""
    lines = []
    lines.append("=" * 60)
    lines.append("NMAP SCAN SUMMARY")
    lines.append("=" * 60)

    for host in results["hosts"]:
        if host["status"] != "up":
            continue

        lines.append(f"\nHost: {host['address']}")
        lines.append(f"Status: {host['status']}")

        if host["hostnames"]:
            lines.append(f"Hostnames: {', '.join(h['name'] for h in host['hostnames'])}")

        # TCP ports
        tcp_ports = [p for p in host["ports"]["tcp"] if p["state"] == "open"]
        if tcp_ports:
            lines.append(f"\nOpen TCP Ports:")
            for p in sorted(tcp_ports, key=lambda x: x["port"]):
                service = p.get("service", {})
                svc_name = service.get("name", "unknown")
                svc_ver = service.get("version", "")
                svc_prod = service.get("product", "")
                line = f"  {p['port']}/tcp - {svc_name}"
                if svc_prod:
                    line += f" ({svc_prod}"
                    if svc_ver:
                        line += f" {svc_ver}"
                    line += ")"
                lines.append(line)

        # UDP ports
        udp_ports = [p for p in host["ports"]["udp"] if p["state"] == "open"]
        if udp_ports:
            lines.append(f"\nOpen UDP Ports:")
            for p in sorted(udp_ports, key=lambda x: x["port"]):
                service = p.get("service", {})
                svc_name = service.get("name", "unknown")
                lines.append(f"  {p['port']}/udp - {svc_name}")

        # OS detection
        if host["os"]:
            lines.append(f"\nOS Detection:")
            for os_match in host["os"][:3]:
                lines.append(f"  {os_match['name']} (accuracy: {os_match['accuracy']}%)")

    lines.append("\n" + "=" * 60)
    return "\n".join(lines)


def main():
    if len(sys.argv) < 2:
        print("Usage: python parse_nmap_xml.py <nmap_xml_file> [-o output.json] [--summary]")
        print("\nExamples:")
        print("  python parse_nmap_xml.py scan.xml")
        print("  python parse_nmap_xml.py scan.xml -o output.json")
        print("  python parse_nmap_xml.py scan.xml --summary")
        sys.exit(1)

    xml_file = sys.argv[1]
    output_file = None
    show_summary = False

    for i in range(2, len(sys.argv)):
        if sys.argv[i] == "-o":
            output_file = sys.argv[i + 1] if i + 1 < len(sys.argv) else None
        elif sys.argv[i] == "--summary":
            show_summary = True

    # Parse XML
    try:
        results = parse_nmap_xml(xml_file)
    except ET.ParseError as e:
        print(f"Error parsing XML: {e}", file=sys.stderr)
        sys.exit(1)
    except FileNotFoundError:
        print(f"File not found: {xml_file}", file=sys.stderr)
        sys.exit(1)

    # Output
    if show_summary:
        print(format_summary(results))

    if output_file:
        with open(output_file, 'w') as f:
            json.dump(results, f, indent=2)
        print(f"\nResults saved to: {output_file}", file=sys.stderr)
    elif not show_summary:
        print(json.dumps(results, indent=2))


if __name__ == "__main__":
    main()
