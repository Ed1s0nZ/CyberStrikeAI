# Integration Examples

Examples of integrating pentest-skills with the results-storage system.

## Overview

The results-storage system integrates with all pentest-skills through **storage scripts**. Each skill has an optional storage script that:

1. **Parses tool output** (nmap XML, sqlmap JSON, etc.)
2. **Calls StorageAPI** to persist findings
3. **Supports --subsystem flag** for optional grouping

## Integration Pattern

All storage scripts follow a consistent pattern:

```python
#!/usr/bin/env python3
"""
[Vulnerability Type] Storage Script
Parses [tool] output and stores to database
"""

import sys
import json
from pathlib import Path

# Add results-storage to path
sys.path.insert(0, str(Path(__file__).parent.parent.parent / 'results-storage' / 'scripts'))

from storage_api import StorageAPI

def main():
    import argparse

    parser = argparse.ArgumentParser(description="Store [vulnerability type] to database")
    parser.add_argument("--subsystem", help="Subsystem name (optional)")
    parser.add_argument("--host-ip", required=True, help="Target host IP")
    parser.add_argument("--url", required=True, help="Vulnerable URL")
    # ... other arguments

    args = parser.parse_args()

    # Parse output (from file or stdin)
    data = parse_output(args)

    # Store to database
    api = StorageAPI()
    api.store_[vulnerability_type]_vulnerability(
        host_ip=args.host_ip,
        url=args.url,
        subsystem=args.subsystem,
        **data
    )

if __name__ == "__main__":
    main()
```

---

## Example 1: Port Scan Storage

### Script: port_scan_storage.py

```python
#!/usr/bin/env python3
"""
Port Scan Results Storage Script
Parse nmap XML output and store to database
"""

import sys
import xml.etree.ElementTree as ET
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent.parent.parent / 'results-storage' / 'scripts'))

from storage_api import StorageAPI

def main():
    import argparse

    parser = argparse.ArgumentParser(description="Store port scan results")
    parser.add_argument("--subsystem", help="Subsystem name (optional)")
    parser.add_argument("--scan-tool", default="nmap", help="Scan tool")
    parser.add_argument("--xml-file", help="Nmap XML file path")

    args = parser.parse_args()

    # Parse XML
    api = StorageAPI()
    ports_data = []

    if args.xml_file:
        tree = ET.parse(args.xml_file)
        root = tree.getroot()
    else:
        xml_content = sys.stdin.read()
        root = ET.fromstring(xml_content)

    for host in root.findall(".//host"):
        ip_elem = host.find("./address[@addrtype='ipv4']")
        if ip_elem is None:
            continue

        ip = ip_elem.get("addr")

        for port in host.findall(".//port"):
            port_info = {
                "port": int(port.get("portid")),
                "protocol": port.get("protocol"),
                "state": port.find("./state").get("state"),
            }

            service = port.find("./service")
            if service is not None:
                port_info.update({
                    "service": service.get("name"),
                    "version": service.get("version"),
                    "product": service.get("product"),
                    "extra_info": service.get("extrainfo")
                })

            ports_data.append(port_info)

        # Store to database
        if ports_data:
            api.store_port_scan(
                host_ip=ip,
                ports=ports_data,
                scan_tool=args.scan_tool,
                subsystem=args.subsystem
            )
            print(f"[+] Stored {len(ports_data)} ports for {ip}")

if __name__ == "__main__":
    main()
```

### Usage

```bash
# Store nmap scan (flat hierarchy)
nmap -sV -p- 192.168.1.0/24 -oX scan.xml
python .claude/skills/recon-port-scan/scripts/port_scan_storage.py \
  --xml-file scan.xml

# Store with subsystem
python .claude/skills/recon-port-scan/scripts/port_scan_storage.py \
  --xml-file scan.xml \
  --subsystem "External Network"
```

---

## Example 2: SQL Injection Storage

### Script: sqli_storage.py

```python
#!/usr/bin/env python3
"""
SQL Injection Storage Script
Parse sqlmap output and store to database
"""

import sys
import json
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent.parent.parent / 'results-storage' / 'scripts'))

from storage_api import StorageAPI

def parse_sqlmap_output(output_file):
    """Parse sqlmap output (simplified example)"""
    # In reality, parse sqlmap's output format
    # This is a simplified example
    return {
        "parameter": "id",
        "payload": "1' OR '1'='1",
        "db_type": "MySQL",
        "cvss_score": 9.8
    }

def main():
    import argparse

    parser = argparse.ArgumentParser(description="Store SQL injection to database")
    parser.add_argument("--subsystem", help="Subsystem name (optional)")
    parser.add_argument("--host-ip", required=True, help="Target host IP")
    parser.add_argument("--url", required=True, help="Vulnerable URL")
    parser.add_argument("--parameter", required=True, help="Vulnerable parameter")
    parser.add_argument("--payload", required=True, help="Payload used")
    parser.add_argument("--severity", default="High", help="Severity level")

    args = parser.parse_args()

    # Store to database
    api = StorageAPI()
    vuln_id = api.store_sqli_vulnerability(
        host_ip=args.host_ip,
        url=args.url,
        parameter=args.parameter,
        payload=args.payload,
        subsystem=args.subsystem,
        severity=args.severity
    )

    print(f"[+] Stored SQL injection vulnerability (ID: {vuln_id})")

if __name__ == "__main__":
    main()
```

### Usage

```bash
# After running sqlmap
sqlmap -u "https://example.com/login?id=1" --batch

# Manually store the finding
python .claude/skills/exploit-sqli/scripts/sqli_storage.py \
  --host-ip 192.168.1.100 \
  --url "https://example.com/login?id=1" \
  --parameter id \
  --payload "1' OR '1'='1" \
  --subsystem "Web Application" \
  --severity Critical
```

---

## Example 3: XSS Storage

### Script: xss_storage.py

```python
#!/usr/bin/env python3
"""
XSS Storage Script
Parse XSS test output and store to database
"""

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent.parent.parent / 'results-storage' / 'scripts'))

from storage_api import StorageAPI

def main():
    import argparse

    parser = argparse.ArgumentParser(description="Store XSS to database")
    parser.add_argument("--subsystem", help="Subsystem name (optional)")
    parser.add_argument("--host-ip", required=True, help="Target host IP")
    parser.add_argument("--url", required=True, help="Vulnerable URL")
    parser.add_argument("--xss-type", required=True,
                       choices=["reflected", "stored", "dom"],
                       help="XSS type")
    parser.add_argument("--payload", required=True, help="Payload used")
    parser.add_argument("--context", default="html_body", help="XSS context")
    parser.add_argument("--severity", default="Medium", help="Severity level")

    args = parser.parse_args()

    # Store to database
    api = StorageAPI()
    vuln_id = api.store_xss_vulnerability(
        host_ip=args.host_ip,
        url=args.url,
        xss_type=args.xss_type,
        payload=args.payload,
        context=args.context,
        subsystem=args.subsystem,
        severity=args.severity
    )

    print(f"[+] Stored XSS vulnerability (ID: {vuln_id})")

if __name__ == "__main__":
    main()
```

### Usage

```bash
# After testing XSS manually
python .claude/skills/exploit-xss/scripts/xss_storage.py \
  --host-ip 192.168.1.100 \
  --url "https://example.com/search?q=test" \
  --xss-type reflected \
  --payload "<script>alert(1)</script>" \
  --context html_body \
  --subsystem "Web Application" \
  --severity High
```

---

## Example 4: LFI Storage

### Script: lfi_storage.py

```python
#!/usr/bin/env python3
"""
LFI Storage Script
Parse LFI test output and store to database
"""

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent.parent.parent / 'results-storage' / 'scripts'))

from storage_api import StorageAPI

def main():
    import argparse

    parser = argparse.ArgumentParser(description="Store LFI to database")
    parser.add_argument("--subsystem", help="Subsystem name (optional)")
    parser.add_argument("--host-ip", required=True, help="Target host IP")
    parser.add_argument("--url", required=True, help="Vulnerable URL")
    parser.add_argument("--payload", required=True, help="Payload used")
    parser.add_argument("--file-read", help="Content of file read")
    parser.add_argument("--severity", default="High", help="Severity level")

    args = parser.parse_args()

    # Store to database
    api = StorageAPI()
    vuln_id = api.store_lfi_vulnerability(
        host_ip=args.host_ip,
        url=args.url,
        payload=args.payload,
        file_read=args.file_read or "",
        subsystem=args.subsystem,
        severity=args.severity
    )

    print(f"[+] Stored LFI vulnerability (ID: {vuln_id})")

if __name__ == "__main__":
    main()
```

### Usage

```bash
# After successful LFI exploitation
python .claude/skills/exploit-lfi/scripts/lfi_storage.py \
  --host-ip 192.168.1.100 \
  --url "https://example.com/download?file=../../etc/passwd" \
  --payload "../../etc/passwd" \
  --file-read "root:x:0:0:root:/root:/bin/bash\n..." \
  --subsystem "Web Application" \
  --severity Critical
```

---

## Example 5: Subdomain Storage

### Script: subdomain_storage.py

```python
#!/usr/bin/env python3
"""
Subdomain Storage Script
Parse subdomain enumeration results and store hosts
"""

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent.parent.parent / 'results-storage' / 'scripts'))

from storage_api import StorageAPI
import socket

def resolve_ip(hostname):
    """Resolve hostname to IP"""
    try:
        return socket.gethostbyname(hostname)
    except socket.gaierror:
        return None

def main():
    import argparse

    parser = argparse.ArgumentParser(description="Store subdomains to database")
    parser.add_argument("--subsystem", help="Subsystem name (optional)")
    parser.add_argument("--input-file", help="File containing subdomains (one per line)")

    args = parser.parse_args()

    # Read subdomains
    if args.input_file:
        with open(args.input_file) as f:
            subdomains = [line.strip() for line in f if line.strip()]
    else:
        subdomains = [line.strip() for line in sys.stdin if line.strip()]

    # Store to database
    api = StorageAPI()
    stored_count = 0

    for subdomain in subdomains:
        ip = resolve_ip(subdomain)
        if ip:
            # Create host entry
            api._get_or_create_host(
                ip_address=ip,
                hostname=subdomain,
                subsystem_id=api.get_or_create_subsystem(args.subsystem) if args.subsystem else None
            )
            stored_count += 1
            print(f"[+] Stored {subdomain} -> {ip}")

    print(f"[+] Stored {stored_count} subdomains")

if __name__ == "__main__":
    main()
```

### Usage

```bash
# After running subfinder
subfinder -d example.com > subdomains.txt

# Store subdomains (flat hierarchy)
python .claude/skills/recon-subdomain/scripts/subdomain_storage.py \
  --input-file subdomains.txt

# Store with subsystem
python .claude/skills/recon-subdomain/scripts/subdomain_storage.py \
  --input-file subdomains.txt \
  --subsystem "External Infrastructure"
```

---

## Complete Workflow Example

### Multi-Day Penetration Test

```bash
# === Day 1: Initial Reconnaissance ===

# Port scan and store
nmap -sV -p- 192.168.1.0/24 -oX day1_scan.xml
python .claude/skills/recon-port-scan/scripts/port_scan_storage.py \
  --xml-file day1_scan.xml \
  --subsystem "Customer A"

# Subdomain enumeration and store
subfinder -d customer-a.com > subdomains.txt
python .claude/skills/recon-subdomain/scripts/subdomain_storage.py \
  --input-file subdomains.txt \
  --subsystem "Customer A"

# === Day 2: Web Application Testing ===

# SQL injection discovery
sqlmap -u "https://customer-a.com/login?id=1" --batch
python .claude/skills/exploit-sqli/scripts/sqli_storage.py \
  --host-ip 192.168.1.100 \
  --url "https://customer-a.com/login?id=1" \
  --parameter id \
  --payload "1' OR '1'='1" \
  --subsystem "Customer A" \
  --severity Critical

# XSS testing
python .claude/skills/exploit-xss/scripts/xss_storage.py \
  --host-ip 192.168.1.100 \
  --url "https://customer-a.com/search?q=test" \
  --xss-type reflected \
  --payload "<script>alert(1)</script>" \
  --subsystem "Customer A" \
  --severity High

# === Day 3: Report Generation ===

# Generate comprehensive report
python -c "
from .claude.skills.results-storage.scripts.report_generator import ReportGenerator
gen = ReportGenerator()
gen.generate_markdown_report('customer_a_report.md', 'Customer A')
gen.generate_json_report('customer_a_report.json', 'Customer A')
"

# Or generate all-subsystems report
python -c "
from .claude.skills.results-storage.scripts.report_generator import ReportGenerator
gen = ReportGenerator()
gen.generate_markdown_report('full_report.md')
"
```

---

## Programmatic Usage

### Python API

```python
from .claude.skills.results-storage.scripts.storage_api import StorageAPI
from .claude.skills.results-storage.scripts.report_generator import ReportGenerator

# Initialize API
api = StoreAPI()

# Create subsystem
subsystem_id = api.create_subsystem(
    name="Web Application",
    description="Customer A web infrastructure",
    subnet_range="192.168.1.0/24"
)

# Store vulnerabilities
api.store_sqli_vulnerability(
    host_ip="192.168.1.100",
    url="https://example.com/login?id=1",
    parameter="id",
    payload="1' OR '1'='1",
    subsystem="Web Application",
    severity="Critical",
    cvss_score=9.8
)

api.store_xss_vulnerability(
    host_ip="192.168.1.100",
    url="https://example.com/search?q=test",
    xss_type="reflected",
    payload="<script>alert(1)</script>",
    subsystem="Web Application",
    severity="High"
)

# Query vulnerabilities
critical_vulns = api.get_vulnerabilities(
    subsystem="Web Application",
    severity="Critical"
)

for vuln in critical_vulns:
    print(f"[{vuln['id']}] {vuln['title']}")

# Get host summary
host = api.get_host_summary("192.168.1.100")
print(f"Host {host['ip_address']}: {host['vuln_count']} vulnerabilities")

# Get statistics
stats = api.get_subsystem_statistics(subsystem="Web Application")
print(f"Total vulnerabilities: {stats['total_vulnerabilities']}")

# Generate reports
gen = ReportGenerator()
gen.generate_markdown_report(
    output_path="webapp_report.md",
    subsystem="Web Application"
)
```

---

## Tips and Best Practices

### 1. Use Consistent Subsystem Names

```bash
# Good - consistent naming
python ... --subsystem "Web Application"
python ... --subsystem "Web Application"

# Bad - inconsistent
python ... --subsystem "Web App"
python ... --subsystem "webapp"
```

### 2. Store After Every Test Session

```bash
# After each tool execution, immediately store results
nmap -sV target.com -oX scan.xml
python port_scan_storage.py --xml-file scan.xml

# Don't wait until end of engagement
# Data may be lost if system crashes
```

### 3. Backup Database Regularly

```bash
# After each testing day
cp ./data/results.db ./data/results_backup_$(date +%Y%m%d).db

# Or encrypted backup
gpg --cipher-algo AES256 --symmetric ./data/results.db
```

### 4. Verify Storage

```bash
# Quick check after storing
python -c "
from .claude.skills.results-storage.scripts.storage_api import StorageAPI
api = StorageAPI()
stats = api.get_subsystem_statistics('Web Application')
print(f\"Vulnerabilities: {stats['total_vulnerabilities']}\")
"
```

### 5. Generate Subsystem-Specific Reports

```bash
# For specific teams or stakeholders
python -c "
from .claude.skills.results-storage.scripts.report_generator import ReportGenerator
gen = ReportGenerator()
gen.generate_markdown_report('dmz_report.md', 'DMZ')
gen.generate_markdown_report('internal_report.md', 'Internal Network')
"
```

---

## Troubleshooting

### Import Errors

```bash
# Error: Module not found
# Solution: Add results-storage to Python path
export PYTHONPATH="${PYTHONPATH}:$(pwd)"
python .claude/skills/recon-port-scan/scripts/port_scan_storage.py ...
```

### Database Locked

```bash
# Error: database is locked
# Solution: Check for other processes
lsof ./data/results.db
```

### Permission Errors

```bash
# Error: unable to open database file
# Solution: Check data directory permissions
chmod 700 ./data
chmod 600 ./data/results.db
```

---

For database schema, see `database_schema.md`.

For API reference, see `api_reference.md`.
