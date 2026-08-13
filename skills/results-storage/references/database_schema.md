# Database Schema Reference

Complete database architecture for pentest-skills results-storage system.

## Overview

The pentest-skills storage system uses SQLite with a two-tier hierarchy:
- **Subsystem** (optional) - Logical grouping of hosts (e.g., "DMZ", "Internal Network")
- **Host** - Individual target systems

When a subsystem is not specified, data is organized in a flat Host hierarchy (subsystem_id is NULL).

## Schema Version

Current version: **1.0.0**

Location: `./data/results.db`

## Tables

### subsystems

Optional organizational units for grouping related hosts.

```sql
CREATE TABLE subsystems (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    subnet_range TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
)
```

**Columns:**
- `id` - Primary key
- `name` - Unique subsystem name (required)
- `description` - Optional description
- `subnet_range` - Optional subnet range (e.g., "192.168.1.0/24")
- `created_at` - Creation timestamp

**Constraints:**
- `name` must be unique

**Indexes:** None (small table)

---

### hosts

Core table storing discovered/created hosts.

```sql
CREATE TABLE hosts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    subsystem_id INTEGER,  -- NULL for flat hierarchy
    ip_address TEXT NOT NULL,
    hostname TEXT,
    mac_address TEXT,
    os_fingerprint TEXT,
    status TEXT DEFAULT 'unknown',  -- up/down/unknown
    first_seen TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_seen TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(subsystem_id, ip_address),
    FOREIGN KEY (subsystem_id) REFERENCES subsystems(id) ON DELETE CASCADE
)
```

**Columns:**
- `id` - Primary key
- `subsystem_id` - Foreign key to subsystems (NULL = flat hierarchy)
- `ip_address` - IP address (required)
- `hostname` - Optional hostname
- `mac_address` - Optional MAC address
- `os_fingerprint` - OS fingerprinting result
- `status` - Host status (up/down/unknown)
- `first_seen` - First discovery timestamp
- `last_seen` - Last seen timestamp

**Constraints:**
- Combination of (subsystem_id, ip_address) must be unique
- Foreign key to subsystems with CASCADE delete

**Indexes:**
- `idx_host_ips` on (ip_address)
- `idx_host_subsystem` on (subsystem_id)

**Notes:**
- A host with subsystem_id = NULL is in flat hierarchy
- A host can appear in multiple subsystems with different IDs

---

### vulnerabilities

Stores all vulnerability findings.

```sql
CREATE TABLE vulnerabilities (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    host_id INTEGER NOT NULL,
    vulnerability_type TEXT NOT NULL,  -- sqli, xss, lfi, etc.
    severity TEXT NOT NULL,  -- Critical, High, Medium, Low, Info
    title TEXT NOT NULL,
    description TEXT,
    affected_component TEXT,
    proof_of_concept TEXT,
    cvss_score REAL,
    cwe_id TEXT,
    cve_id TEXT,
    status TEXT DEFAULT 'open',  -- open, confirmed, false_positive, mitigated
    discovered_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    discovered_by_skill TEXT,
    FOREIGN KEY (host_id) REFERENCES hosts(id) ON DELETE CASCADE
)
```

**Columns:**
- `id` - Primary key
- `host_id` - Foreign key to hosts (required)
- `vulnerability_type` - Vulnerability type (required)
- `severity` - Severity level (required)
- `title` - Vulnerability title (required)
- `description` - Detailed description
- `affected_component` - Affected component/path/endpoint
- `proof_of_concept` - PoC code or payload
- `cvss_score` - CVSS score (0.0-10.0)
- `cwe_id` - CWE identifier (e.g., "CWE-89")
- `cve_id` - CVE identifier (e.g., "CVE-2024-1234")
- `status` - Vulnerability status
- `discovered_at` - Discovery timestamp
- `discovered_by_skill` - Skill/tool that discovered this

**Valid vulnerability_type values:**
- `sqli` - SQL Injection
- `xss` - Cross-Site Scripting
- `lfi` - Local File Inclusion
- `rfi` - Remote File Inclusion
- `ssrf` - Server-Side Request Forgery
- `xxe` - XML External Entity
- `file_download` - Insecure File Download
- `command_injection` - Command Injection
- `csrf` - Cross-Site Request Forgery
- `open_redirect` - Open Redirect
- `path_traversal` - Path Traversal
- `insecure_deserialization` - Insecure Deserialization
- `weak_auth` - Weak Authentication
- `info_disclosure` - Information Disclosure
- `other` - Other

**Valid severity values:**
- `Critical` - 9.0-10.0 CVSS, immediate action required
- `High` - 7.0-8.9 CVSS
- `Medium` - 4.0-6.9 CVSS
- `Low` - 0.1-3.9 CVSS
- `Info` - 0.0 CVSS, informational only

**Constraints:**
- Foreign key to hosts with CASCADE delete

**Indexes:**
- `idx_vulnerabilities_severity` on (severity)
- `idx_vulnerabilities_type` on (vulnerability_type)
- `idx_vulnerabilities_host` on (host_id)
- `idx_vulnerabilities_discovered_at` on (discovered_at)
- `idx_vulnerabilities_status` on (status)

---

### port_scan_results

Stores port scan findings (nmap, masscan, etc.).

```sql
CREATE TABLE port_scan_results (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    host_id INTEGER NOT NULL,
    port INTEGER NOT NULL,
    protocol TEXT NOT NULL,  -- tcp/udp
    state TEXT NOT NULL,  -- open/closed/filtered
    service TEXT,
    version TEXT,
    product TEXT,
    extra_info TEXT,
    scan_tool TEXT,  -- nmap/masscan/rustscan
    scan_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (host_id) REFERENCES hosts(id) ON DELETE CASCADE,
    UNIQUE(host_id, port, protocol)
)
```

**Columns:**
- `id` - Primary key
- `host_id` - Foreign key to hosts (required)
- `port` - Port number (required)
- `protocol` - Protocol (required)
- `state` - Port state (required)
- `service` - Service name (e.g., "http", "ssh")
- `version` - Service version (e.g., "2.4.41")
- `product` - Product name (e.g., "Apache httpd")
- `extra_info` - Additional information
- `scan_tool` - Tool used for scan
- `scan_date` - Scan timestamp

**Valid protocol values:**
- `tcp` - TCP protocol
- `udp` - UDP protocol

**Valid state values:**
- `open` - Port is open
- `closed` - Port is closed
- `filtered` - Port state is filtered
- `unfiltered` - Port is unfiltered
- `open|filtered` - Open or filtered
- `closed|filtered` - Closed or filtered

**Constraints:**
- Foreign key to hosts with CASCADE delete
- Combination of (host_id, port, protocol) must be unique

**Indexes:**
- `idx_port_scans_host` on (host_id)
- `idx_port_scans_state` on (state)

---

### web_findings

Stores detailed web vulnerability information.

```sql
CREATE TABLE web_findings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    vulnerability_id INTEGER NOT NULL,
    url TEXT NOT NULL,
    parameter TEXT,
    http_method TEXT,  -- GET/POST/COOKIE/HEADER
    payload TEXT,
    response_evidence TEXT,
    context TEXT,  -- html_body/html_attribute/javascript/dom/url
    request_headers TEXT,  -- JSON string
    FOREIGN KEY (vulnerability_id) REFERENCES vulnerabilities(id) ON DELETE CASCADE
)
```

**Columns:**
- `id` - Primary key
- `vulnerability_id` - Foreign key to vulnerabilities (required)
- `url` - Vulnerable URL (required)
- `parameter` - Vulnerable parameter name
- `http_method` - HTTP method used
- `payload` - Payload used
- `response_evidence` - Response evidence of vulnerability
- `context` - Vulnerability context (for XSS)
- `request_headers` - Request headers (JSON string)

**Valid http_method values:**
- `GET` - GET request parameter
- `POST` - POST body parameter
- `COOKIE` - Cookie parameter
- `HEADER` - HTTP header
- `JSON` - JSON POST body

**Valid context values (for XSS):**
- `html_body` - HTML body context
- `html_attribute` - HTML attribute context
- `javascript` - JavaScript code context
- `dom` - DOM-based context
- `url` - URL parameter context

**Constraints:**
- Foreign key to vulnerabilities with CASCADE delete

**Indexes:**
- `idx_web_findings_vuln` on (vulnerability_id)

**Notes:**
- One-to-many relationship with vulnerabilities
- Only populated for web-based vulnerabilities (SQLi, XSS, LFI, etc.)

---

### scan_metadata

Stores metadata about scans and operations.

```sql
CREATE TABLE scan_metadata (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    host_id INTEGER NOT NULL,
    skill_used TEXT NOT NULL,
    scan_command TEXT,
    scan_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    raw_output_path TEXT,
    notes TEXT,
    FOREIGN KEY (host_id) REFERENCES hosts(id) ON DELETE CASCADE
)
```

**Columns:**
- `id` - Primary key
- `host_id` - Foreign key to hosts (required)
- `skill_used` - Skill/tool name used (required)
- `scan_command` - Command executed
- `scan_date` - Scan timestamp
- `raw_output_path` - Path to raw output file
- `notes` - Additional notes

**Constraints:**
- Foreign key to hosts with CASCADE delete

**Indexes:**
- `idx_scan_metadata_host` on (host_id)
- `idx_scan_metadata_skill` on (skill_used)

**Notes:**
- Optional table for tracking scan operations
- Not used by core queries, for audit trail

---

### metadata

System metadata table.

```sql
CREATE TABLE metadata (
    key TEXT PRIMARY KEY,
    value TEXT,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
)
```

**Columns:**
- `key` - Metadata key
- `value` - Metadata value
- `updated_at` - Last update timestamp

**Standard keys:**
- `schema_version` - Database schema version

---

## Entity Relationship Diagram

```
subsystems (1) ----< (0-1) hosts
                             |
                             | (1)
                             |
                             V
                       vulnerabilities (0-many)
                             |
                             | (1)
                             |
                             V
                       web_findings (0-many)

hosts (1) ----< (0-many) port_scan_results
hosts (1) ----< (0-many) scan_metadata
```

## Cascading Deletes

All child tables use `ON DELETE CASCADE`:
- Deleting a subsystem → deletes all hosts in that subsystem
- Deleting a host → deletes all vulnerabilities, ports, metadata for that host
- Deleting a vulnerability → deletes all web_findings for that vulnerability

## Data Integrity

### Uniqueness Constraints

1. **subsystems.name** - Each subsystem name is unique
2. **hosts(subsystem_id, ip_address)** - Each IP can appear once per subsystem
3. **port_scan_results(host_id, port, protocol)** - No duplicate port entries

### Foreign Key Relationships

1. **hosts.subsystem_id** → **subsystems.id**
2. **vulnerabilities.host_id** → **hosts.id**
3. **port_scan_results.host_id** → **hosts.id**
4. **scan_metadata.host_id** → **hosts.id**
5. **web_findings.vulnerability_id** → **vulnerabilities.id**

## Performance Considerations

### Indexes

All frequently queried columns are indexed:
- Host lookups by IP address
- Vulnerability filtering by severity, type, host
- Port scan filtering by host, state

### Query Patterns

Common query patterns optimized by indexes:
- Find all Critical/High vulnerabilities
- Get all vulnerabilities for a host
- List all hosts in a subsystem
- Count open ports per host
- Vulnerability discovery trends

## Migration Notes

When upgrading schema versions:

1. **Backup database** before migration
2. **Use ALTER TABLE** for additive changes
3. **Preserve data** - never drop columns with data
4. **Update schema_version** in metadata table
5. **Test migrations** on copy of production database

## Example Queries

### Find all Critical vulnerabilities

```sql
SELECT v.*, h.ip_address
FROM vulnerabilities v
JOIN hosts h ON v.host_id = h.id
WHERE v.severity = 'Critical'
ORDER BY v.cvss_score DESC;
```

### Count vulnerabilities per host

```sql
SELECT
    h.ip_address,
    COUNT(*) as total_vulns,
    COUNT(CASE WHEN v.severity = 'Critical' THEN 1 END) as critical_count
FROM hosts h
JOIN vulnerabilities v ON h.id = v.host_id
GROUP BY h.id
ORDER BY total_vulns DESC;
```

### Find exposed services

```sql
SELECT
    psr.service,
    psr.version,
    COUNT(*) as host_count
FROM port_scan_results psr
WHERE psr.state = 'open'
GROUP BY psr.service, psr.version
ORDER BY host_count DESC;
```

### Get subsystem statistics

```sql
SELECT
    s.name,
    COUNT(DISTINCT h.id) as host_count,
    COUNT(v.id) as vuln_count
FROM subsystems s
JOIN hosts h ON s.id = h.subsystem_id
LEFT JOIN vulnerabilities v ON h.id = v.host_id
GROUP BY s.id;
```

---

For API usage, see `api_reference.md`.

For integration examples, see `integration_examples.md`.
