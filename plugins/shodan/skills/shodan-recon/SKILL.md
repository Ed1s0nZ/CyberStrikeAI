# Shodan OSINT & Network Reconnaissance

## Overview

Shodan is the world's first search engine for Internet-connected devices. Unlike Google, which indexes web content, Shodan indexes service banners, TLS certificates, and metadata from every publicly roachable host on the Internet. Use the `shodan_search` tool for passive host discovery, service fingerprinting, vulnerability detection, and infrastructure intelligence.

## When to Use Shodan vs Other Tools

| Scenario | Best Tool | Why |
|----------|-----------|-----|
| Find all services on a known IP | **Shodan** (host) | Complete banner + vuln data, free |
| Discover subdomains from CT logs | **crt.sh** | Free, passive, no API key needed |
| Search hosts by service/banner | **Shodan** (search) | Best banner search, huge index |
| Internet-wide host enumeration | **Censys** or Shodan | Both good, Censys has better CQL |
| Find hosts by ASN/org | **Shodan** or Censys | Both support org/ASN filters |
| Vulnerability search by CVE | **Shodan** (search) | `vuln:CVE-xxxx` filter (paid) |
| Exploit database search | **Shodan** (exploits) | Aggregates multiple exploit DBs |
| Certificate transparency | **crt.sh** | Most complete CT log aggregator |
| Certificate details by IP | **Censys** or Shodan | Both index TLS certs |
| DNS resolution | **Shodan** (dns) | Free, fast, bulk resolve |
| Attack surface mapping | **Censys** + Shodan | Use both for completeness |
| C2 infrastructure hunting | **Shodan** (`ssl.jarm`) or Censys (`labels:"c2"`) | JARM fingerprinting |

## Quick Start

```
# First: validate API key and check credits
shodan_search query="validate"

# Look up a specific IP (free, no credits)
shodan_search query="8.8.8.8" command="host"

# Count matching hosts (free)
shodan_search query="apache port:443 country:UA" command="count"

# Full search (costs 1 credit for filtered queries)
shodan_search query="org:\"Target Corp\" port:443" command="search"

# Resolve hostnames to IPs (free)
shodan_search query="google.com,github.com" command="dns"

# Reverse DNS (free)
shodan_search query="8.8.8.8,1.1.1.1" command="reverse"

# Search exploits
shodan_search query="CVE-2021-44228" command="exploits"
```

## Reconnaissance Workflow

### Phase 1: Passive Discovery (Free Tier)

Start with free operations that consume no credits.

```
# 1. Validate key and check plan
shodan_search query="validate"

# 2. Resolve target domains to IPs
shodan_search query="target.com,www.target.com,mail.target.com,vpn.target.com" command="dns"

# 3. Look up each discovered IP (free, detailed)
shodan_search query="203.0.113.10" command="host"

# 4. Reverse DNS on discovered IPs
shodan_search query="203.0.113.10,203.0.113.11,203.0.113.12" command="reverse"

# 5. Count hosts in the target's network range (free)
shodan_search query="net:203.0.113.0/24" command="count"
shodan_search query="net:203.0.113.0/24" command="count" facets="port,org"
```

### Phase 2: Organization Mapping (1 Credit)

```
# Find all hosts by organization name
shodan_search query="org:\"Target Corp\"" command="search"

# Find by ASN
shodan_search query="asn:AS12345" command="search"

# Get domain DNS records and subdomains (1 credit)
shodan_search query="target.com" command="domain"
```

### Phase 3: Service Enumeration (Credits)

```
# Web servers in target range
shodan_search query="net:203.0.113.0/24 port:80,443" command="search"

# SSH servers
shodan_search query="net:203.0.113.0/24 port:22" command="search"

# Mail servers
shodan_search query="net:203.0.113.0/24 port:25,587,993" command="search"

# Exposed databases
shodan_search query="net:203.0.113.0/24 port:3306,5432,27017,6379" command="search"

# Admin panels and dashboards
shodan_search query="net:203.0.113.0/24 http.title:\"Dashboard\"" command="search"
```

### Phase 4: Vulnerability Assessment (Paid)

```
# Find hosts with known CVEs
shodan_search query="org:\"Target Corp\" has_vuln:true" command="search"

# Search for specific CVE
shodan_search query="org:\"Target Corp\" vuln:CVE-2021-44228" command="search"

# Outdated software
shodan_search query="org:\"Target Corp\" product:\"OpenSSH\" version:\"7.\"" command="search"

# Expired TLS certificates
shodan_search query="org:\"Target Corp\" ssl.cert.expired:true" command="search"

# Self-signed certificates
shodan_search query="net:203.0.113.0/24 ssl.cert.issuer.cn:\"*\" -ssl.cert.issuer.o:\"Let's Encrypt\" -ssl.cert.issuer.o:\"DigiCert\"" command="search"
```

### Phase 5: Exploit Research

```
# Find exploits for discovered CVEs
shodan_search query="CVE-2021-44228" command="exploits"

# Find exploits by platform
shodan_search query="platform:linux type:remote" command="exploits"

# Find exploits for a specific product
shodan_search query="Apache Log4j" command="exploits"
```

### Phase 6: Feed into Active Scanning

Take Shodan results and feed IPs + ports into active tools:

```
# After getting IPs and ports from Shodan:
# 1. Targeted nmap on discovered services
# 2. Nuclei against web services
# 3. Manual verification of interesting findings
# 4. Cross-reference with Censys and crt.sh results
```

## Query Syntax Reference

### Filters

| Filter | Example | Description |
|--------|---------|-------------|
| `port` | `port:22` | Service port number |
| `org` | `org:"Google"` | Organization name |
| `country` | `country:UA` | 2-letter country code |
| `city` | `city:"Kyiv"` | City name |
| `net` | `net:1.2.3.0/24` | CIDR range |
| `os` | `os:"Windows"` | Operating system |
| `product` | `product:"nginx"` | Product name |
| `version` | `version:"1.0"` | Software version |
| `hostname` | `hostname:"example.com"` | Hostname |
| `asn` | `asn:"AS15169"` | Autonomous System Number |
| `before` | `before:"01/01/2025"` | Results before date |
| `after` | `after:"01/01/2025"` | Results after date |
| `tag` | `tag:"ics"` | Shodan tag |
| `has_vuln` | `has_vuln:true` | Hosts with known vulns (paid) |
| `vuln` | `vuln:CVE-2021-44228` | Specific CVE (paid) |

### HTTP Filters

| Filter | Example | Description |
|--------|---------|-------------|
| `http.title` | `http.title:"Dashboard"` | Page title |
| `http.html` | `http.html:"login"` | HTML body content |
| `http.status` | `http.status:200` | HTTP status code |
| `http.component` | `http.component:"WordPress"` | Web component |
| `http.favicon.hash` | `http.favicon.hash:116323821` | Favicon hash |
| `http.server` | `http.server:"Apache"` | Server header |

### SSL/TLS Filters

| Filter | Example | Description |
|--------|---------|-------------|
| `ssl` | `ssl:true` | Has TLS/SSL |
| `ssl.cert.subject.cn` | `ssl.cert.subject.cn:"example.com"` | Cert Common Name |
| `ssl.cert.issuer.o` | `ssl.cert.issuer.o:"Let's Encrypt"` | Cert issuer org |
| `ssl.cert.expired` | `ssl.cert.expired:true` | Expired certs |
| `ssl.version` | `ssl.version:"tlsv1"` | TLS version |
| `ssl.jarm` | `ssl.jarm:"hash"` | JARM fingerprint |

### Boolean Operators

| Operator | Example | Description |
|----------|---------|-------------|
| `AND` (space) | `apache port:443` | Both conditions (implicit AND) |
| `OR` | `nginx OR apache` | Either condition |
| `-` (NOT) | `port:22 -country:CN` | Exclude condition |
| `""` | `"exact phrase"` | Exact banner match |

## Interpreting Results

### Host Lookup Fields

| Field | Significance |
|-------|-------------|
| `ports` | Open services - attack surface |
| `vulns` | Known CVEs - prioritize exploitation |
| `os` | Operating system - helps select exploits |
| `product/version` | Software ID - check for known vulns |
| `hostnames` | DNS names - may reveal internal naming |
| `org/isp` | Hosting provider - infrastructure intel |
| `ssl.jarm` | TLS fingerprint - C2 detection |
| `http.title` | Page title - quick service identification |

### Common Banner Patterns

| Banner Content | Indicates |
|----------------|-----------|
| `SSH-2.0-OpenSSH_7.` | Outdated SSH (check CVEs) |
| `220 Microsoft ESMTP` | Exchange mail server |
| `MongoDB Server Information` | Exposed MongoDB |
| `-NOAUTH` | Redis without authentication |
| `Anonymous access granted` | FTP anonymous login |
| `X-Jenkins` | Jenkins CI/CD server |

## Free vs Paid Strategy

**Maximize free tier:**
1. Use `host` for known IPs (always free)
2. Use `count` with facets for statistics (always free)
3. Use `dns`/`reverse` for name resolution (always free)
4. Use `count` before `search` to see if results are worth a credit

**When to spend credits:**
1. Full search results needed (specific hosts, banners)
2. Domain DNS enumeration
3. Paginating past page 1

**Credit conservation tips:**
- Use `count` + facets first to understand result landscape
- Use `minify` to reduce response size
- Combine Shodan with free tools (crt.sh for subdomains, Censys free lookups)
- Cache results - don't re-run the same query

## Tips

- **JARM fingerprinting** (`ssl.jarm`) is excellent for C2 detection - known C2 frameworks have published JARM hashes
- **Favicon hashes** (`http.favicon.hash`) uniquely identify web applications even when titles change
- **Historical data** via `host --history` shows how services changed over time
- **Faceted counts** are free and reveal organizational structure without spending credits
- Shodan updates slower than Censys (weekly vs daily) but has deeper banner data
- Use `ssl.cert.subject.cn` to find all hosts using a specific TLS certificate
- Combine with crt.sh: find subdomains via CT logs, then look up each IP in Shodan
