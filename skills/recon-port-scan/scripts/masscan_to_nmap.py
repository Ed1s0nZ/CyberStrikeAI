#!/usr/bin/env python3
"""
Convert masscan JSON output to nmap-compatible format or nmap commands.

Usage:
    python masscan_to_nmap.py <masscan_json_file>
    python masscan_to_nmap.py scan.json -o ports.txt
    python masscan_to_nmap.py scan.json --nmap-cmd -t 192.168.1.100

Output options:
- Plain text: list of open ports (for nmap -p)
- Nmap commands: generates nmap commands to scan discovered ports
"""

import sys
import json
import argparse
from typing import List, Dict, Any


def parse_masscan_json(json_file: str) -> List[Dict[str, Any]]:
    """
    Parse masscan JSON output file.

    Args:
        json_file: Path to masscan JSON file

    Returns:
        List of discovered open ports
    """
    results = []

    try:
        with open(json_file, 'r') as f:
            for line in f:
                line = line.strip()
                if not line:
                    continue
                try:
                    data = json.loads(line)
                    if data.get("status") == "open":
                        results.append({
                            "ip": data.get("ip", ""),
                            "port": int(data.get("port", 0)),
                            "proto": data.get("proto", "tcp"),
                            "timestamp": data.get("timestamp", ""),
                            "service": data.get("service", {})
                        })
                except json.JSONDecodeError:
                    continue
    except FileNotFoundError:
        print(f"Error: File not found: {json_file}", file=sys.stderr)
        sys.exit(1)

    return results


def format_ports(results: List[Dict[str, Any]], target_ip: str = None) -> str:
    """
    Format results as port list.

    Args:
        results: Parsed masscan results
        target_ip: Filter by specific IP

    Returns:
        Comma-separated list of ports
    """
    if target_ip:
        results = [r for r in results if r["ip"] == target_ip]

    ports = sorted(set(r["port"] for r in results))
    return ",".join(map(str, ports))


def format_by_host(results: List[Dict[str, Any]]) -> str:
    """
    Format results grouped by host IP.

    Args:
        results: Parsed masscan results

    Returns:
        Host:port formatted output
    """
    hosts = {}
    for r in results:
        ip = r["ip"]
        if ip not in hosts:
            hosts[ip] = []
        hosts[ip].append(r["port"])

    lines = []
    for ip in sorted(hosts.keys()):
        ports = sorted(hosts[ip])
        lines.append(f"{ip}: {','.join(map(str, ports))}")

    return "\n".join(lines)


def generate_nmap_commands(results: List[Dict[str, Any]], extra_args: str = "") -> str:
    """
    Generate nmap commands for discovered ports.

    Args:
        results: Parsed masscan results
        extra_args: Additional nmap arguments

    Returns:
        List of nmap commands
    """
    hosts = {}
    for r in results:
        ip = r["ip"]
        if ip not in hosts:
            hosts[ip] = []
        hosts[ip].append(r["port"])

    commands = []
    for ip, ports in hosts.items():
        ports_str = ",".join(map(str, sorted(set(ports))))
        cmd = f"nmap -sV -sC -p {ports_str} {extra_args} {ip}"
        commands.append(cmd)

    return commands


def main():
    parser = argparse.ArgumentParser(
        description="Convert masscan JSON to nmap-compatible format"
    )
    parser.add_argument("json_file", help="Masscan JSON output file")
    parser.add_argument("-o", "--output", help="Output file")
    parser.add_argument("-t", "--target", help="Filter by target IP")
    parser.add_argument("--nmap-cmd", action="store_true",
                        help="Generate nmap commands instead of port list")
    parser.add_argument("--by-host", action="store_true",
                        help="Group output by host IP")
    parser.add_argument("--nmap-args", default="",
                        help="Additional arguments for generated nmap commands")

    args = parser.parse_args()

    # Parse masscan results
    results = parse_masscan_json(args.json_file)

    if not results:
        print("No open ports found in masscan output.", file=sys.stderr)
        sys.exit(0)

    # Generate output
    if args.nmap_cmd:
        output_lines = generate_nmap_commands(results, args.nmap_args)
        output = "\n".join(output_lines)
    elif args.by_host:
        output = format_by_host(results)
    else:
        output = format_ports(results, args.target)

    # Write output
    if args.output:
        with open(args.output, 'w') as f:
            f.write(output)
        print(f"Output written to: {args.output}", file=sys.stderr)
    else:
        print(output)


if __name__ == "__main__":
    main()
