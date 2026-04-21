# Censys OSINT & Attack Surface Mapping

## Overview

Censys maintains an internet-wide scan database updated daily, covering every publicly routable IPv4 and IPv6 host. Use the `censys-search` tool for passive reconnaissance, attack surface mapping, and infrastructure intelligence.

## When to Use Censys

- **Attack surface discovery** - find all internet-facing hosts for a target org
- **Infrastructure mapping** - identify hosting providers, ASNs, IP ranges
- **Certificate transparency** - find all TLS certs issued for a domain (including subdomains)
- **Service fingerprinting** - identify specific software versions across IP ranges
- **Exposed service detection** - find databases, admin panels, APIs without auth
- **C2 hunting** - identify known C2 infrastructure
- **Pre-nmap intelligence** - know what ports/services to expect before active scanning

## Quick Start

```
# First: validate your API key and check remaining quota
censys-search query="validate"

# Search for hosts with a specific domain's TLS cert
censys-search query='services.tls.certificates.leaf_data.subject.common_name: "*.target.com"'

# Find all hosts in an organization's ASN
censys-search query='autonomous_system.name: "TARGET CORP"' per_page=100

# Get full details for a specific IP
censys-search query="1.2.3.4" resource="detail"
```

## Reconnaissance Workflow

### Phase 1: Domain/Org Discovery

```
# Find all TLS certs for the target domain (includes subdomains)
censys-search query='services.tls.certificates.leaf_data.subject.common_name: "*.target.com"'

# Find by organization name in cert subject
censys-search query='services.tls.certificates.leaf_data.subject.organization: "Target Corp"'

# Search by ASN name
censys-search query='autonomous_system.name: "TARGET"'
```

### Phase 2: Service Enumeration

```
# Find all web servers in the target's IP range
censys-search query='ip: 203.0.113.0/24 AND services.service_name: HTTP'

# Find exposed admin panels
censys-search query='ip: 203.0.113.0/24 AND services.http.response.html_title: "Dashboard"'

# Find all SSH hosts (version fingerprint)
censys-search query='ip: 203.0.113.0/24 AND services.software.product: "OpenSSH"'

# Find mail servers
censys-search query='ip: 203.0.113.0/24 AND (services.port: 25 OR services.port: 587 OR services.port: 465)'
```

### Phase 3: Vulnerability Surface

```
# Outdated OpenSSH (known vulns in 7.x)
censys-search query='autonomous_system.name: "TARGET" AND services.software.product: "OpenSSH" AND services.software.version: "7.*"'

# Exposed databases
censys-search query='ip: 203.0.113.0/24 AND (services.port: 3306 OR services.port: 5432 OR services.port: 27017 OR services.port: 6379)'

# Self-signed certificates (often internal/dev systems)
censys-search query='ip: 203.0.113.0/24 AND labels: "self-signed"'

# Exposed Elasticsearch/Kibana
censys-search query='autonomous_system.name: "TARGET" AND services.http.response.html_title: "Kibana"'
```

### Phase 4: Certificate Intelligence

```
# All certs for the domain (resource=certificates)
censys-search query='parsed.names: "*.target.com"' resource="certificates"

# Find certs about to expire
censys-search query='parsed.names: "target.com" AND parsed.validity_period.not_after: [now TO 2024-12-31]' resource="certificates"

# Find wildcard certs
censys-search query='parsed.subject.common_name: "*.target.com"' resource="certificates"
```

### Phase 5: Feed into Active Scanning

Take Censys results and feed IPs + ports into nmap/nuclei for active verification:

```
# After getting IPs from Censys, run targeted nmap
nmap -sV -p <ports_from_censys> <ip_from_censys>

# Run nuclei against discovered web services
nuclei -u https://<host_from_censys> -t cves/
```

## CQL Query Reference

| Operator | Example | Description |
|----------|---------|-------------|
| `:` | `services.port: 22` | Field match |
| `AND` | `port: 22 AND country: "US"` | Both conditions |
| `OR` | `port: 80 OR port: 443` | Either condition |
| `NOT` | `NOT labels: "cdn"` | Exclude |
| `*` | `"Apache/2.4*"` | Wildcard |
| CIDR | `ip: 10.0.0.0/8` | IP range |
| Range | `[2024-01-01 TO *]` | Date range |

## Key Fields

| Field | Description |
|-------|-------------|
| `services.port` | Open port number |
| `services.service_name` | Service type (HTTP, SSH, FTP, etc.) |
| `services.software.product` | Software name (OpenSSH, Apache, nginx) |
| `services.software.version` | Software version |
| `services.http.response.html_title` | Web page title |
| `services.http.response.status_code` | HTTP status code |
| `services.http.response.headers.server` | Server header |
| `services.tls.certificates.leaf_data.subject.common_name` | TLS cert CN |
| `services.banner` | Service banner text |
| `services.jarm.fingerprint` | JARM TLS fingerprint |
| `autonomous_system.asn` | ASN number |
| `autonomous_system.name` | ASN organization name |
| `location.country_code` | 2-letter country code |
| `location.city` | City name |
| `operating_system.product` | Detected OS |
| `labels` | Censys labels (c2, iot, self-signed, etc.) |
| `dns.reverse_dns.names` | Reverse DNS names |

## Rate Limits

- **Free tier**: 250 queries/month
- **Researcher**: 2,500 queries/month
- **Pro**: 50,000 queries/month

Check your balance: `censys-search query="validate"`
