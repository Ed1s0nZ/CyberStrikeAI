---
name: results-storage
description: SQLite-based persistent storage and reporting system for penetration testing results. Use this skill when user needs to store scan results, query vulnerabilities, generate reports, or manage pentest data across sessions.
---

# Pentest Results Storage & Reporting

## Overview

The `results-storage` skill provides a SQLite-based persistent storage system for all penetration testing activities. It enables cross-session data persistence, structured vulnerability tracking, and professional report generation.

### Key Features

- **Persistent Storage**: SQLite database at `./data/results.db`
- **Flexible Organization**: Subsystem (optional) > Host hierarchy
- **Multi-Format Reports**: Markdown and JSON report generation
- **Query Capabilities**: Filter by severity, type, host, subsystem
- **Zero Breaking Changes**: Works alongside existing skills without modifications

### Data Organization

```
Subsystem (optional)
    └── Host
        ├── Port Scan Results
        ├── Vulnerabilities (SQLi, XSS, LFI, etc.)
        └── Web Findings (detailed web vulnerability info)
```

**Without Subsystem**: Data is organized in a flat Host hierarchy (subsystem_id is NULL)

## Quick Start

### Initialize Database

Database is automatically created on first use:

```python
from .claude.skills.results-storage.scripts.storage_api import StorageAPI

api = StoreAPI()
# Database created at ./data/results.db
```

### Store Vulnerability

```python
# Flat hierarchy (no subsystem)
api.store_sqli_vulnerability(
    host_ip="192.168.1.100",
    url="https://example.com/login?id=1",
    parameter="id",
    payload="1' OR '1'='1",
    severity="Critical",
    title="SQL Injection in Login Page"
)

# With subsystem
api.store_xss_vulnerability(
    host_ip="192.168.1.100",
    url="https://example.com/search",
    xss_type="reflected",
    payload="<script>alert(1)</script>",
    subsystem="Web Application",
    severity="High",
    title="XSS in Search Functionality"
)
```

### Generate Report

```python
from .claude.skills.results-storage.scripts.report_generator import ReportGenerator

gen = ReportGenerator()

# Markdown report
gen.generate_markdown_report(output_path="pentest_report.md")

# JSON report
gen.generate_json_report(output_path="pentest_report.json")

# Subsystem-specific report
gen.generate_markdown_report(
    output_path="webapp_report.md",
    subsystem="Web Application"
)
```

## Database Schema

### Tables

**subsystems** (optional)
- id, name, description, subnet_range, created_at

**hosts**
- id, subsystem_id (nullable), ip_address, hostname, mac_address, os_fingerprint, status, first_seen, last_seen

**vulnerabilities**
- id, host_id, vulnerability_type, severity, title, description, affected_component, proof_of_concept, cvss_score, cwe_id, cve_id, status, discovered_at, discovered_by_skill

**port_scan_results**
- id, host_id, port, protocol, state, service, version, product, extra_info, scan_tool, scan_date

**web_findings** (detailed web vulnerability info)
- id, vulnerability_id, url, parameter, http_method, payload, response_evidence, context, request_headers

**scan_metadata**
- id, host_id, skill_used, scan_command, scan_date, raw_output_path, notes

### Indexes

Optimized for common queries:
- `idx_vulnerabilities_severity` on vulnerabilities(severity)
- `idx_vulnerabilities_type` on vulnerabilities(vulnerability_type)
- `idx_vulnerabilities_host` on vulnerabilities(host_id)
- `idx_port_scans_host` on port_scan_results(host_id)
- `idx_web_findings_vuln` on web_findings(vulnerability_id)

## API Reference

### StorageAPI Class

#### Initialization

```python
api = StorageAPI(db_path="./data/results.db")
```

#### Subsystem Management

```python
# Create subsystem
subsystem_id = api.create_subsystem(
    name="Web Application",
    description="Customer A web infrastructure",
    subnet_range="192.168.1.0/24"
)

# Get or create (returns existing if already exists)
subsystem_id = api.get_or_create_subsystem("Web Application")

# List all subsystems
subsystems = api.list_subsystems()
```

#### Vulnerability Storage

```python
# Generic vulnerability storage
vuln_id = api.store_vulnerability(
    host_ip="192.168.1.100",
    vuln_type="sqli",
    severity="Critical",
    title="SQL Injection in Login",
    subsystem="Web Application",  # Optional
    description="Full description...",
    affected_component="/login",
    proof_of_concept="id=1' OR '1'='1",
    cvss_score=9.8,
    cwe_id="CWE-89"
)

# SQL Injection (convenience method)
vuln_id = api.store_sqli_vulnerability(
    host_ip="192.168.1.100",
    url="https://example.com/login?id=1",
    parameter="id",
    payload="1' OR '1'='1",
    db_type="MySQL",
    subsystem="Web Application",  # Optional
    **details
)

# XSS (convenience method)
vuln_id = api.store_xss_vulnerability(
    host_ip="192.168.1.100",
    url="https://example.com/search?q=test",
    xss_type="reflected",  # reflected/stored/dom
    payload="<script>alert(1)</script>",
    context="html_body",  # html_body/html_attribute/javascript/dom/url
    subsystem="Web Application",  # Optional
    **details
)

# LFI (convenience method)
vuln_id = api.store_lfi_vulnerability(
    host_ip="192.168.1.100",
    url="https://example.com/download?file=../../etc/passwd",
    payload="../../etc/passwd",
    file_read="root:x:0:0:root:/root:/bin/bash\n...",
    subsystem="Web Application",  # Optional
    **details
)
```

#### Port Scan Storage

```python
api.store_port_scan(
    host_ip="192.168.1.100",
    ports=[
        {
            "port": 80,
            "protocol": "tcp",
            "state": "open",
            "service": "http",
            "version": "Apache httpd 2.4.41",
            "product": "Apache"
        },
        {
            "port": 443,
            "protocol": "tcp",
            "state": "open",
            "service": "https"
        }
    ],
    scan_tool="nmap",
    subsystem="External Network"  # Optional
)
```

#### Query Methods

```python
# Get vulnerabilities with filters
vulns = api.get_vulnerabilities(
    subsystem="Web Application",  # Optional
    severity="Critical",          # Optional
    vuln_type="sqli",            # Optional
    host_ip="192.168.1.100"      # Optional
)
# Returns: List[Dict]

# Get host summary
summary = api.get_host_summary("192.168.1.100")
# Returns: Dict with open_ports, vulnerabilities, os_info, etc.

# Get statistics
stats = api.get_subsystem_statistics(subsystem="Web Application")
# Returns: Dict with total_vulnerabilities, severity_breakdown, etc.
```

### ReportGenerator Class

```python
gen = ReportGenerator(db_path="./data/results.db")

# Markdown report
markdown_path = gen.generate_markdown_report(
    output_path="report.md",      # Optional: auto-generates if None
    subsystem="Web Application"    # Optional: includes all if None
)

# JSON report
json_path = gen.generate_json_report(
    output_path="report.json",
    subsystem="Web Application"
)

# Executive summary only
summary = gen.generate_executive_summary(subsystem="Web Application")

# Risk score calculation
risk_info = gen.calculate_risk_score(vulnerabilities)
# Returns: {"score": 8.5, "level": "High"}
```

## Integration with Existing Skills

All existing skills have optional storage scripts. These scripts:

1. **Parse tool output** (nmap XML, sqlmap JSON, etc.)
2. **Call StorageAPI** to persist findings
3. **Support --subsystem flag** for optional grouping

### Example: Port Scan Storage

```bash
# Flat hierarchy
nmap -sV -p- 192.168.1.0/24 -oX scan.xml
python .claude/skills/recon-port-scan/scripts/port_scan_storage.py \
  --xml-file scan.xml

# With subsystem
python .claude/skills/recon-port-scan/scripts/port_scan_storage.py \
  --xml-file scan.xml \
  --subsystem "External Network"
```

### Example: SQLi Storage

```bash
sqlmap -u "https://example.com/login?id=1" --batch --answers=continuing
python .claude/skills/exploit-sqli/scripts/sqli_storage.py \
  --host-ip 192.168.1.100 \
  --url "https://example.com/login?id=1" \
  --parameter id \
  --subsystem "Web Application" \
  --severity Critical
```

## Usage Scenarios

### Scenario 1: Multi-Session Penetration Test

**Situation**: You're conducting a penetration test over several days and need to maintain state across sessions.

```bash
# Day 1: Initial reconnaissance
nmap -sV -p- 192.168.1.0/24 -oX day1_scan.xml
python .claude/skills/recon-port-scan/scripts/port_scan_storage.py \
  --xml-file day1_scan.xml \
  --subsystem "Customer A"

# Day 2: Web application testing
sqlmap -u "https://customer-a.com/login?id=1" --batch
python .claude/skills/exploit-sqli/scripts/sqli_storage.py \
  --host-ip 192.168.1.100 \
  --url "https://customer-a.com/login?id=1" \
  --parameter id \
  --subsystem "Customer A"

# Day 3: Generate report
python -c "
from .claude.skills.results-storage.scripts.report_generator import ReportGenerator
gen = ReportGenerator()
gen.generate_markdown_report('customer_a_report.md', 'Customer A')
"
```

### Scenario 2: Multi-Subsystem Organization

**Situation**: Testing a large network with distinct subsystems (DMZ, Internal, Cloud).

```bash
# DMZ scanning
nmap -sV -p- 10.0.0.0/24 -oX dmz_scan.xml
python .claude/skills/recon-port-scan/scripts/port_scan_storage.py \
  --xml-file dmz_scan.xml \
  --subsystem "DMZ"

# Internal network
nmap -sV -p- 192.168.1.0/24 -oX internal_scan.xml
python .claude/skills/recon-port-scan/scripts/port_scan_storage.py \
  --xml-file internal_scan.xml \
  --subsystem "Internal"

# Cloud infrastructure
nmap -sV -p- 10.1.0.0/24 -oX cloud_scan.xml
python .claude/skills/recon-port-scan/scripts/port_scan_storage.py \
  --xml-file cloud_scan.xml \
  --subsystem "Cloud"

# Generate subsystem-specific reports
python -c "
from .claude.skills.results-storage.scripts.report_generator import ReportGenerator
gen = ReportGenerator()
gen.generate_markdown_report('dmz_report.md', 'DMZ')
gen.generate_markdown_report('internal_report.md', 'Internal')
gen.generate_markdown_report('cloud_report.md', 'Cloud')
"

# Or generate comprehensive report
python -c "
gen = ReportGenerator()
gen.generate_markdown_report('full_report.md')
"
```

### Scenario 3: Vulnerability Querying

**Situation**: You need to find all Critical vulnerabilities across all hosts.

```python
from .claude.skills.results-storage.scripts.storage_api import StorageAPI

api = StorageAPI()

critical_vulns = api.get_vulnerabilities(severity="Critical")

for vuln in critical_vulns:
    print(f"[{vuln['id']}] {vuln['title']}")
    print(f"  Host: {vuln['host_ip']}")
    print(f"  Type: {vuln['vulnerability_type']}")
    print(f"  CVSS: {vuln['cvss_score']}")
    print(f"  Discovered: {vuln['discovered_at']}")
    print()
```

### Scenario 4: Flat Hierarchy Usage

**Situation**: Simple test without complex organization.

```bash
# No subsystem specified - uses flat hierarchy
nmap -sV -p- 192.168.1.0/24 -oX scan.xml
python .claude/skills/recon-port-scan/scripts/port_scan_storage.py \
  --xml-file scan.xml

# Query flat hierarchy
python -c "
from .claude.skills.results-storage.scripts.storage_api import StorageAPI
api = StorageAPI()
api.store_xss_vulnerability(
    host_ip='192.168.1.100',
    url='https://example.com/search',
    xss_type='reflected',
    payload='<script>alert(1)</script>',
    # No subsystem = flat hierarchy
    severity='High',
    title='XSS in Search'
)
"
```

### Scenario 5: Host-Based Investigation

**Situation**: Deep dive into a specific host's findings.

```python
from .claude.skills.results-storage.scripts.storage_api import StorageAPI

api = StorageAPI()

# Get comprehensive host summary
host_summary = api.get_host_summary("192.168.1.100")

print(f"IP: {host_summary['ip_address']}")
print(f"Hostname: {host_summary.get('hostname', 'Unknown')}")
print(f"OS: {host_summary.get('os_fingerprint', 'Unknown')}")
print(f"Open Ports: {len(host_summary['ports'])}")
print(f"Vulnerabilities: {len(host_summary['vulnerabilities'])}")

print("\n--- Vulnerabilities ---")
for vuln in host_summary['vulnerabilities']:
    print(f"[{vuln['severity']}] {vuln['title']}")

print("\n--- Open Ports ---")
for port in host_summary['ports']:
    print(f"Port {port['port']}/{port['protocol']}: {port.get('service', 'unknown')} ({port['state']})")
```

### Scenario 6: Custom Queries

**Situation**: You need to query for specific vulnerability patterns.

```python
from .claude.skills.results-storage.scripts.storage_api import StorageAPI

api = StorageAPI()

# All web vulnerabilities in a subsystem
web_vulns = api.get_vulnerabilities(
    subsystem="Web Application",
    vuln_type=["xss", "sqli", "lfi", "ssrf", "xxe"]
)

# High-severity SQLi across all subsystems
sqli_high = api.get_vulnerabilities(
    vuln_type="sqli",
    severity=["High", "Critical"]
)

# Vulnerabilities for a specific host
host_vulns = api.get_vulnerabilities(host_ip="192.168.1.100")
```

## Report Customization

### Markdown Report Sections

Generated reports include:
1. **Executive Summary** - High-level overview and risk score
2. **Methodology** - Testing approach and tools used
3. **Detailed Findings** - Vulnerabilities grouped by severity
4. **Host Inventory** - All discovered hosts
5. **Recommendations** - Prioritized remediation steps
6. **Appendices** - Commands, tool outputs, compliance

### JSON Report Schema

```json
{
  "metadata": {
    "report_date": "2025-02-14",
    "test_period": {
      "start": "2025-02-10",
      "end": "2025-02-14"
    },
    "testers": ["pentester"]
  },
  "executive_summary": {
    "risk_score": 8.5,
    "total_vulnerabilities": 47,
    "severity_breakdown": {...}
  },
  "subsystems": [...],
  "hosts": [...],
  "vulnerabilities": [...],
  "recommendations": [...]
}
```

## Data Management

### Database Location

`./data/results.db`

### Backup

```bash
# Simple backup
cp ./data/results.db ./data/results_backup_$(date +%Y%m%d).db

# Encrypted backup (recommended for production)
gpg --cipher-algo AES256 --compress-algo 1 --symmetric ./data/results.db
```

### Export/Import

```bash
# SQLite to SQL
sqlite3 ./data/results.db .dump > results_backup.sql

# SQL to SQLite
sqlite3 ./data/results.db < results_backup.sql
```

### Database Reset

```bash
# CAUTION: Deletes all data
rm ./data/results.db
# Database will be recreated on next use
```

## Performance Considerations

- **Capacity**: Tested with 10,000+ vulnerabilities
- **Query Time**: < 1 second for filtered queries
- **Report Generation**: < 10 seconds for full report with 1,000+ vulnerabilities
- **Indexes**: Automatically created for common query patterns

## Security Considerations

### Data Protection

1. **Database Permissions**: Results.db is created with 600 permissions (owner read/write only)
2. **Sensitive Data**: Database contains PoC code - consider encryption for production environments
3. **Audit Trail**: All findings include timestamps and discovery methods

### Best Practices

- **Regular Backups**: Backup after each testing session
- **Access Control**: Ensure file system permissions are restrictive
- **Encryption**: Encrypt backups before storing in shared locations
- **Retention**: Follow retention policies - delete data when no longer needed

## Troubleshooting

### Database Locked

```bash
# Check for processes using the database
lsof ./data/results.db

# Kill blocking processes if needed
kill -9 <PID>
```

### Import Errors

```bash
# Ensure Python path is correct
export PYTHONPATH="${PYTHONPATH}:$(pwd)"
python -c "from .claude.skills.results-storage.scripts.storage_api import StorageAPI"
```

### Report Generation Errors

```bash
# Install Jinja2 if missing
pip install jinja2

# Verify database exists
ls -lh ./data/results.db
```

## Compliance

This storage and reporting system supports compliance with:

- **OWASP Testing Guide** v4.2
- **PTES** (Penetration Testing Execution Standard)
- **OSSTMM** (Open Source Security Testing Methodology)

All timestamps are recorded in UTC for consistency.

## Related Skills

- **recon-port-scan** - Port scanning with storage integration
- **recon-subdomain** - Subdomain enumeration with storage integration
- **exploit-sqli** - SQL injection testing with storage integration
- **exploit-xss** - XSS testing with storage integration
- **exploit-lfi** - LFI testing with storage integration

## Advanced Usage

### Programmatic Access

```python
import sqlite3

conn = sqlite3.connect('./data/results.db')
cursor = conn.cursor()

# Custom SQL queries
cursor.execute("""
    SELECT h.ip_address, COUNT(v.id) as vuln_count
    FROM hosts h
    LEFT JOIN vulnerabilities v ON h.id = v.host_id
    GROUP BY h.ip_address
    ORDER BY vuln_count DESC
""")

for row in cursor.fetchall():
    print(f"{row[0]}: {row[1]} vulnerabilities")

conn.close()
```

### Batch Operations

```python
from .claude.skills.results-storage.scripts.storage_api import StorageAPI

api = StorageAPI()

# Store multiple vulnerabilities
vulnerabilities = [
    {"host_ip": "192.168.1.100", "vuln_type": "xss", "severity": "High", ...},
    {"host_ip": "192.168.1.101", "vuln_type": "sqli", "severity": "Critical", ...},
]

for vuln_data in vulnerabilities:
    api.store_vulnerability(**vuln_data)
```

---

*This skill provides the foundation for persistent, organized penetration testing data management across all pentest-skills capabilities.*
