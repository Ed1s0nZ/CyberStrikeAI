# Storage API Reference

Complete API reference for the pentest-skills StorageAPI class.

## Overview

The `StorageAPI` class provides the primary interface for storing and querying penetration testing data. It supports:

- **Subsystem (optional) management** - Group hosts by subsystem
- **Vulnerability storage** - Generic and convenience methods
- **Port scan storage** - Store nmap/masscan results
- **Data querying** - Filter and retrieve stored data

## Initialization

```python
from .claude.skills.results-storage.scripts.storage_api import StorageAPI

api = StorageAPI(db_path="./data/results.db")
```

### Parameters

- `db_path` (str, optional) - Path to SQLite database file. Default: `"./data/results.db"`

### Notes

- Database is automatically created if it doesn't exist
- Data directory is created with restrictive permissions (700)
- Database file is created with permissions 600 (owner read/write only)

---

## Subsystem Management

### create_subsystem()

Create a new subsystem (optional organizational unit).

```python
subsystem_id = api.create_subsystem(
    name="Web Application",
    description="Customer A web infrastructure",
    subnet_range="192.168.1.0/24"
)
```

**Parameters:**
- `name` (str, required) - Subsystem name (must be unique)
- `description` (str, optional) - Description
- `subnet_range` (str, optional) - Subnet range (e.g., "192.168.1.0/24")

**Returns:**
- `int` - Subsystem ID

**Raises:**
- `sqlite3.IntegrityError` - If subsystem name already exists

**Example:**
```python
try:
    dmz_id = api.create_subsystem(
        name="DMZ",
        description="Demilitarized Zone",
        subnet_range="10.0.0.0/24"
    )
    print(f"Created DMZ subsystem (ID: {dmz_id})")
except sqlite3.IntegrityError:
    print("DMZ subsystem already exists")
```

---

### get_or_create_subsystem()

Get existing subsystem or create if it doesn't exist.

```python
subsystem_id = api.get_or_create_subsystem("Web Application")
```

**Parameters:**
- `name` (str, required) - Subsystem name

**Returns:**
- `int` - Subsystem ID

**Notes:**
- Idempotent operation - safe to call multiple times
- Returns existing ID if subsystem already exists
- Creates new subsystem if it doesn't exist

**Example:**
```python
# Always get or create, no error handling needed
webapp_id = api.get_or_create_subsystem("Web Application")
```

---

### list_subsystems()

List all subsystems with statistics.

```python
subsystems = api.list_subsystems()
```

**Returns:**
- `List[Dict]` - List of subsystem dictionaries

**Dictionary keys:**
- `id` (int) - Subsystem ID
- `name` (str) - Subsystem name
- `description` (str) - Description
- `subnet_range` (str) - Subnet range
- `created_at` (str) - Creation timestamp
- `host_count` (int) - Number of hosts in subsystem
- `vuln_count` (int) - Number of vulnerabilities in subsystem

**Example:**
```python
subsystems = api.list_subsystems()

print("Subsystems:")
for s in subsystems:
    print(f"  {s['name']}: {s['host_count']} hosts, {s['vuln_count']} vulns")
```

---

## Vulnerability Storage

### store_vulnerability()

Store a generic vulnerability.

```python
vuln_id = api.store_vulnerability(
    host_ip="192.168.1.100",
    vuln_type="sqli",
    severity="Critical",
    title="SQL Injection in Login",
    subsystem="Web Application",  # Optional
    **details
)
```

**Required Parameters:**
- `host_ip` (str) - Target host IP address
- `vuln_type` (str) - Vulnerability type (sqli, xss, lfi, etc.)
- `severity` (str) - Severity level (Critical, High, Medium, Low, Info)
- `title` (str) - Vulnerability title
- `subsystem` (str, optional) - Subsystem name

**Optional Keyword Parameters (details):**
- `description` (str) - Detailed description
- `affected_component` (str) - Affected component/path/endpoint
- `proof_of_concept` (str) - PoC code or payload
- `cvss_score` (float) - CVSS score (0.0-10.0)
- `cwe_id` (str) - CWE identifier (e.g., "CWE-89")
- `cve_id` (str) - CVE identifier (e.g., "CVE-2024-1234")
- `discovered_by_skill` (str) - Skill/tool that discovered this
- `hostname` (str) - Hostname
- `os_fingerprint` (str) - OS fingerprint

**Web Vulnerability Optional Parameters:**
- `url` (str) - Vulnerable URL
- `parameter` (str) - Parameter name
- `http_method` (str) - HTTP method (GET/POST/COOKIE/HEADER)
- `payload` (str) - Payload used
- `response_evidence` (str) - Response evidence
- `context` (str) - Vulnerability context (for XSS)
- `request_headers` (dict) - Request headers

**Returns:**
- `int` - Vulnerability ID

**Valid vuln_type values:**
- `sqli`, `xss`, `lfi`, `rfi`, `ssrf`, `xxe`, `file_download`,
- `command_injection`, `csrf`, `open_redirect`, `path_traversal`,
- `insecure_deserialization`, `weak_auth`, `info_disclosure`, `other`

**Valid severity values:**
- `Critical`, `High`, `Medium`, `Low`, `Info`

**Raises:**
- `ValueError` - If severity is invalid

**Example:**
```python
vuln_id = api.store_vulnerability(
    host_ip="192.168.1.100",
    vuln_type="sqli",
    severity="Critical",
    title="SQL Injection in Login Page",
    description="Authentication bypass via SQL injection in id parameter",
    affected_component="/login",
    proof_of_concept="id=1' OR '1'='1",
    cvss_score=9.8,
    cwe_id="CWE-89",
    url="https://example.com/login?id=1",
    parameter="id",
    subsystem="Web Application"
)
```

---

### store_sqli_vulnerability()

Convenience method to store SQL injection vulnerability.

```python
vuln_id = api.store_sqli_vulnerability(
    host_ip="192.168.1.100",
    url="https://example.com/login?id=1",
    parameter="id",
    payload="1' OR '1'='1",
    subsystem="Web Application",  # Optional
    **details
)
```

**Required Parameters:**
- `host_ip` (str) - Target host IP
- `url` (str) - Vulnerable URL
- `parameter` (str) - Vulnerable parameter name
- `payload` (str) - Payload used
- `subsystem` (str, optional) - Subsystem name

**Optional Keyword Parameters (details):**
- `severity` (str) - Severity level (default: "High")
- `title` (str) - Title (default: auto-generated)
- `description` (str) - Description
- `cvss_score` (float) - CVSS score
- `db_type` (str) - Database type (MySQL, PostgreSQL, etc.)
- Other parameters from `store_vulnerability()`

**Returns:**
- `int` - Vulnerability ID

**Defaults:**
- `vulnerability_type` = "sqli"
- `cwe_id` = "CWE-89"
- `discovered_by_skill` = "exploit-sqli"
- `http_method` = "GET"

**Example:**
```python
api.store_sqli_vulnerability(
    host_ip="192.168.1.100",
    url="https://example.com/login?id=1",
    parameter="id",
    payload="1' OR '1'='1",
    subsystem="Web Application",
    severity="Critical",
    cvss_score=9.8,
    db_type="MySQL"
)
```

---

### store_xss_vulnerability()

Convenience method to store XSS vulnerability.

```python
vuln_id = api.store_xss_vulnerability(
    host_ip="192.168.1.100",
    url="https://example.com/search?q=test",
    xss_type="reflected",
    payload="<script>alert(1)</script>",
    subsystem="Web Application",  # Optional
    **details
)
```

**Required Parameters:**
- `host_ip` (str) - Target host IP
- `url` (str) - Vulnerable URL
- `xss_type` (str) - XSS type (reflected/stored/dom)
- `payload` (str) - Payload used
- `subsystem` (str, optional) - Subsystem name

**Optional Keyword Parameters (details):**
- `severity` (str) - Severity level (default: "Medium")
- `title` (str) - Title (default: auto-generated)
- `description` (str) - Description
- `context` (str) - XSS context (default: "html_body")
- Other parameters from `store_vulnerability()`

**Returns:**
- `int` - Vulnerability ID

**Valid xss_type values:**
- `reflected` - Reflected XSS
- `stored` - Stored/persistent XSS
- `dom` - DOM-based XSS

**Valid context values:**
- `html_body` - HTML body context
- `html_attribute` - HTML attribute context
- `javascript` - JavaScript code context
- `dom` - DOM source context
- `url` - URL parameter context

**Defaults:**
- `vulnerability_type` = "xss"
- `cwe_id` = "CWE-79"
- `discovered_by_skill` = "exploit-xss"

**Example:**
```python
api.store_xss_vulnerability(
    host_ip="192.168.1.100",
    url="https://example.com/search?q=test",
    xss_type="reflected",
    payload="<script>alert(document.cookie)</script>",
    context="html_body",
    subsystem="Web Application",
    severity="High"
)
```

---

### store_lfi_vulnerability()

Convenience method to store LFI vulnerability.

```python
vuln_id = api.store_lfi_vulnerability(
    host_ip="192.168.1.100",
    url="https://example.com/download?file=../../etc/passwd",
    payload="../../etc/passwd",
    file_read="root:x:0:0:root:/root:/bin/bash\n...",  # Optional
    subsystem="Web Application",  # Optional
    **details
)
```

**Required Parameters:**
- `host_ip` (str) - Target host IP
- `url` (str) - Vulnerable URL
- `payload` (str) - Payload used
- `subsystem` (str, optional) - Subsystem name

**Optional Keyword Parameters (details):**
- `file_read` (str) - Content of file read (if successful)
- `severity` (str) - Severity level (default: "High")
- `title` (str) - Title (default: auto-generated)
- Other parameters from `store_vulnerability()`

**Returns:**
- `int` - Vulnerability ID

**Defaults:**
- `vulnerability_type` = "lfi"
- `cwe_id` = "CWE-98"
- `discovered_by_skill` = "exploit-lfi"

**Example:**
```python
api.store_lfi_vulnerability(
    host_ip="192.168.1.100",
    url="https://example.com/download?file=../../etc/passwd",
    payload="../../etc/passwd",
    file_read="root:x:0:0:root:/root:/bin/bash\n...",
    subsystem="Web Application",
    severity="Critical"
)
```

---

## Port Scan Storage

### store_port_scan()

Store port scan results.

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
        # ... more ports
    ],
    scan_tool="nmap",
    subsystem="Web Application"  # Optional
)
```

**Required Parameters:**
- `host_ip` (str) - Target host IP
- `ports` (List[Dict]) - List of port dictionaries
- `subsystem` (str, optional) - Subsystem name

**Port Dictionary Keys:**
- `port` (int, required) - Port number
- `protocol` (str, required) - Protocol (tcp/udp)
- `state` (str, required) - Port state (open/closed/filtered)
- `service` (str, optional) - Service name
- `version` (str, optional) - Service version
- `product` (str, optional) - Product name
- `extra_info` (str, optional) - Additional information

**Other Parameters:**
- `scan_tool` (str, optional) - Tool used (default: "nmap")

**Returns:**
- `None`

**Example:**
```python
ports = [
    {"port": 22, "protocol": "tcp", "state": "open", "service": "ssh"},
    {"port": 80, "protocol": "tcp", "state": "open", "service": "http", "version": "Apache 2.4.41"},
    {"port": 443, "protocol": "tcp", "state": "open", "service": "https"}
]

api.store_port_scan(
    host_ip="192.168.1.100",
    ports=ports,
    scan_tool="nmap",
    subsystem="DMZ"
)
```

---

## Query Methods

### get_vulnerabilities()

Query vulnerabilities with optional filters.

```python
vulns = api.get_vulnerabilities(
    subsystem="Web Application",  # Optional
    severity="Critical",          # Optional
    vuln_type="sqli",            # Optional
    host_ip="192.168.1.100"      # Optional
)
```

**Parameters:**
- `subsystem` (str, optional) - Filter by subsystem name
- `severity` (str, optional) - Filter by severity
- `vuln_type` (str or List[str], optional) - Filter by vulnerability type(s)
- `host_ip` (str, optional) - Filter by host IP

**Returns:**
- `List[Dict]` - List of vulnerability dictionaries

**Dictionary keys:**
- All vulnerability columns (id, title, severity, etc.)
- `host_ip` (str) - Host IP address
- `hostname` (str) - Hostname
- `os_fingerprint` (str) - OS fingerprint
- `subsystem_name` (str) - Subsystem name (if applicable)
- Web finding columns (url, parameter, http_method, etc., if applicable)

**Examples:**
```python
# All Critical vulnerabilities
critical_vulns = api.get_vulnerabilities(severity="Critical")

# All SQLi in a subsystem
sqli_vulns = api.get_vulnerabilities(
    subsystem="Web Application",
    vuln_type="sqli"
)

# Multiple types
web_vulns = api.get_vulnerabilities(
    vuln_type=["xss", "sqli", "lfi", "ssrf"]
)

# All vulnerabilities for a host
host_vulns = api.get_vulnerabilities(host_ip="192.168.1.100")
```

---

### get_host_summary()

Get comprehensive host summary.

```python
summary = api.get_host_summary("192.168.1.100", subsystem="Web Application")
```

**Parameters:**
- `host_ip` (str, required) - Host IP address
- `subsystem` (str, optional) - Subsystem name (for disambiguation if host appears in multiple subsystems)

**Returns:**
- `Dict` or `None` - Host summary dictionary, or None if not found

**Dictionary keys:**
- `id` (int) - Host ID
- `ip_address` (str) - IP address
- `hostname` (str) - Hostname
- `os_fingerprint` (str) - OS fingerprint
- `status` (str) - Host status
- `subsystem_name` (str) - Subsystem name (if applicable)
- `first_seen` (str) - First seen timestamp
- `last_seen` (str) - Last seen timestamp
- `ports` (List[Dict]) - List of port dictionaries
- `vulnerabilities` (List[Dict]) - List of vulnerability dictionaries
- `open_ports_count` (int) - Count of open ports
- `vuln_count` (int) - Count of vulnerabilities
- `severity_breakdown` (Dict) - Vulnerability counts by severity

**Example:**
```python
summary = api.get_host_summary("192.168.1.100")

if summary:
    print(f"Host: {summary['ip_address']}")
    print(f"Hostname: {summary.get('hostname', 'Unknown')}")
    print(f"Open Ports: {summary['open_ports_count']}")
    print(f"Vulnerabilities: {summary['vuln_count']}")
    print(f"Severity: {summary['severity_breakdown']}")
```

---

### get_subsystem_statistics()

Get statistics for a subsystem or overall.

```python
stats = api.get_subsystem_statistics(subsystem="Web Application")  # Optional
stats = api.get_subsystem_statistics()  # All data
```

**Parameters:**
- `subsystem` (str, optional) - Subsystem name (None = all data)

**Returns:**
- `Dict` - Statistics dictionary

**Dictionary keys:**
- `subsystem` (str) - Subsystem name (or "all")
- `total_hosts` (int) - Total hosts
- `total_vulnerabilities` (int) - Total vulnerabilities
- `severity_breakdown` (Dict) - Counts by severity
- `type_breakdown` (Dict) - Counts by vulnerability type
- `average_cvss` (float) - Average CVSS score

**Example:**
```python
stats = api.get_subsystem_statistics(subsystem="Web Application")

print(f"Hosts: {stats['total_hosts']}")
print(f"Vulnerabilities: {stats['total_vulnerabilities']}")
print(f"Average CVSS: {stats['average_cvss']}")
print(f"Severity: {stats['severity_breakdown']}")
print(f"Types: {stats['type_breakdown']}")
```

---

## Context Manager

StorageAPI can be used as a context manager:

```python
with StorageAPI() as api:
    # Use api
    api.store_vulnerability(...)
# Connection automatically closed
```

---

## Error Handling

Common exceptions:

- `ValueError` - Invalid severity or vulnerability type
- `sqlite3.IntegrityError` - Constraint violation (duplicate subsystem name, etc.)
- `sqlite3.Error` - General database error

---

For database schema, see `database_schema.md`.

For integration examples, see `integration_examples.md`.
