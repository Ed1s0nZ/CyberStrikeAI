# DNSdumpster DNS Reconnaissance

## Overview

DNSdumpster is the premier DNS reconnaissance platform for security assessments. It performs active DNS enumeration to discover subdomains, DNS records, and network infrastructure. Use the `dnsdumpster_search` tool for comprehensive DNS intelligence gathering.

## When to Use DNSdumpster vs Other Tools

| Scenario | Best Tool | Why |
|----------|-----------|-----|
| Comprehensive DNS record dump | **DNSdumpster** (domain) | Returns A, MX, NS, TXT, CNAME + ASN + banners |
| Subdomain discovery from CT logs | **crt.sh** | Passive, covers cert issuance history |
| Active subdomain enumeration | **DNSdumpster** (domain) | Active DNS queries find non-CT subdomains |
| Brute-force subdomain enum | **subfinder** / **amass** | Wordlist-based + multi-source aggregation |
| Find domains on same IP | **DNSdumpster** (reverse) | Reverse IP reveals shared hosting |
| IP geolocation + ASN | **DNSdumpster** (host) | Combines GeoIP, rDNS, ASN in one call |
| Service fingerprinting | **Shodan** / **Censys** | Deeper banner data, vulnerability mapping |
| Port scanning | **nmap** | Active port + service enumeration |
| Email infrastructure | **DNSdumpster** (domain, type=MX) | MX records + mail server IPs |
| DNS zone analysis | **DNSdumpster** (domain, type=NS) | NS records + zone delegation |
| SPF/DKIM/DMARC policy | **DNSdumpster** (domain, type=TXT) | TXT records contain email auth policies |
| Technology detection | **DNSdumpster** (domain) | Banner data shows web server/CMS |
| Network mapping | **DNSdumpster** (domain, map=true) | Visual network topology map |

## Quick Start

```
# Validate API key
dnsdumpster_search query="validate"

# Full domain reconnaissance (primary use case)
dnsdumpster_search query="target.com"

# Filter by DNS record type
dnsdumpster_search query="target.com" type="MX"
dnsdumpster_search query="target.com" type="TXT"
dnsdumpster_search query="target.com" type="NS"

# IP address investigation
dnsdumpster_search query="203.0.113.10" command="host"

# Reverse IP - find co-hosted domains
dnsdumpster_search query="203.0.113.10" command="reverse"

# Subdomain enumeration (alternative source)
dnsdumpster_search query="target.com" command="hostsearch"

# Banner search by CIDR (Plus tier)
dnsdumpster_search query="203.0.113.0/24" command="banners"
```

## DNS Enumeration Methodology

### Phase 1: Passive Subdomain Discovery

Start with passive techniques that don't touch the target directly.

```
# 1. Certificate Transparency logs (crt.sh)
#    Finds subdomains from SSL certificate issuance records.
#    Covers historical certs - finds deprecated/removed subdomains.

# 2. DNSdumpster domain lookup
#    Active DNS enumeration - finds subdomains by querying DNS.
dnsdumpster_search query="target.com"

# 3. Shodan domain DNS (if available)
shodan_search query="target.com" command="domain"
```

### Phase 2: DNS Record Analysis

Analyze each record type for security implications.

```
# Get all records
dnsdumpster_search query="target.com"

# Focus on specific types
dnsdumpster_search query="target.com" type="MX"    # Mail infrastructure
dnsdumpster_search query="target.com" type="TXT"   # SPF/DKIM/DMARC
dnsdumpster_search query="target.com" type="NS"    # DNS infrastructure
dnsdumpster_search query="target.com" type="CNAME" # Alias chains
```

### Phase 3: IP Investigation

For each discovered IP, gather detailed intelligence.

```
# GeoIP, reverse DNS, ASN
dnsdumpster_search query="203.0.113.10" command="host"

# Find other domains on same IP (shared hosting)
dnsdumpster_search query="203.0.113.10" command="reverse"

# Cross-reference with Shodan for service banners
shodan_search query="203.0.113.10" command="host"
```

### Phase 4: Network Mapping

```
# Banner search across a network range (Plus tier)
dnsdumpster_search query="203.0.113.0/24" command="banners"

# Get domain network map
dnsdumpster_search query="target.com" --map
```

### Phase 5: Active Scanning

Feed DNSdumpster results into active tools.

```
# After getting IPs and subdomains from DNSdumpster:
# 1. nmap port scan on all discovered IPs
# 2. nuclei vulnerability scan on web services
# 3. Zone transfer attempt on NS records
# 4. Email security check on MX records
# 5. CNAME takeover check on dangling CNAMEs
```

## Interpreting DNS Records for Pentesting

### A Records (Host-to-IP)

| Finding | Significance |
|---------|-------------|
| Multiple A records | Load balancing / CDN - may need to find origin |
| Internal-looking hostnames | `dev.`, `staging.`, `admin.`, `vpn.` - high-value targets |
| Different subnets | Multiple hosting providers or data centers |
| Cloud provider IPs | AWS, Azure, GCP - check for cloud misconfigurations |

### MX Records (Mail)

| Finding | Significance |
|---------|-------------|
| Google Workspace MX | `ASPMX.L.GOOGLE.COM` - target uses Google mail |
| Microsoft 365 MX | `*.mail.protection.outlook.com` - target uses O365 |
| Self-hosted MX | Own mail server - check for vulns, open relay |
| No MX records | Domain may not receive email - less phishing concern |

### NS Records (DNS Infrastructure)

| Finding | Significance |
|---------|-------------|
| Cloud DNS (Route53, CloudFlare) | Managed DNS - zone transfer unlikely |
| Self-hosted NS | Check for zone transfer (AXFR), DNS cache poisoning |
| Single NS | No redundancy - DoS vector |
| Mismatched NS | Possible misconfiguration or stale delegation |

### TXT Records (SPF/DKIM/DMARC)

| Record | What to Check |
|--------|--------------|
| SPF (`v=spf1`) | Overly permissive `+all` or `~all` enables spoofing |
| DKIM (`v=DKIM1`) | Key strength, selector enumeration |
| DMARC (`v=DMARC1`) | `p=none` means no enforcement - spoofing possible |
| Verification tokens | Cloud services in use (Google, Microsoft, etc.) |
| No SPF/DMARC | Email spoofing wide open |

### CNAME Records (Aliases)

| Finding | Significance |
|---------|-------------|
| Points to cloud service | Check if service is still active (takeover risk) |
| Points to CDN | Cloudflare, Akamai, Fastly - origin IP hunting |
| Dangling CNAME | Target no longer exists - subdomain takeover |
| Long CNAME chains | Potential misconfiguration, performance issues |

## Technology Identification from DNS

### Common DNS Patterns

| DNS Pattern | Technology |
|-------------|-----------|
| `_amazonses.domain.com` TXT | Amazon SES (email sending) |
| `_dmarc.domain.com` TXT | DMARC policy configured |
| `selector._domainkey.domain.com` TXT | DKIM signing configured |
| `aspmx.l.google.com` MX | Google Workspace |
| `*.mail.protection.outlook.com` MX | Microsoft 365 |
| `ns-*.awsdns-*.` NS | AWS Route 53 |
| `*.ns.cloudflare.com` NS | Cloudflare DNS |
| TXT `google-site-verification=` | Google Search Console |
| TXT `MS=` | Microsoft 365 domain verification |
| TXT `atlassian-domain-verification=` | Atlassian (Jira/Confluence) |
| TXT `docusign=` | DocuSign integration |
| TXT `facebook-domain-verification=` | Facebook Business |

## Subdomain Enumeration Strategy

### Combine Multiple Sources

For maximum coverage, use multiple subdomain enumeration techniques:

1. **Certificate Transparency (crt.sh)** - passive, historical, covers wildcard certs
2. **DNSdumpster** - active DNS enumeration, banner data, network mapping
3. **Shodan/Censys** - passive host data with `ssl.cert.subject.cn` or `hostname` filters
4. **Brute-force** - subfinder/amass with wordlists for common subdomain patterns

### Subdomain Patterns to Watch

| Pattern | Indicates |
|---------|-----------|
| `dev.*`, `staging.*`, `test.*` | Development environments (often less secure) |
| `admin.*`, `panel.*`, `manage.*` | Admin interfaces |
| `api.*`, `api-v2.*` | API endpoints |
| `vpn.*`, `remote.*`, `gateway.*` | Remote access infrastructure |
| `mail.*`, `smtp.*`, `imap.*` | Mail infrastructure |
| `ftp.*`, `sftp.*`, `files.*` | File transfer services |
| `db.*`, `mysql.*`, `mongo.*` | Database servers (should not be public) |
| `jenkins.*`, `ci.*`, `build.*` | CI/CD infrastructure |
| `grafana.*`, `kibana.*`, `monitor.*` | Monitoring dashboards |
| `git.*`, `gitlab.*`, `bitbucket.*` | Source code management |
| `backup.*`, `bak.*` | Backup infrastructure |
| `internal.*`, `intranet.*` | Internal services exposed externally |

## Free vs Plus Tier

**Free tier (sufficient for most assessments):**
- Domain lookup: up to 50 records
- Full DNS record types (A, MX, NS, TXT, CNAME, SOA)
- Subdomain enumeration
- ASN and reverse DNS data

**Plus tier (for large targets):**
- Up to 200 records per domain
- Pagination (`?page=2`)
- Domain network mapping (`?map=1`)
- Banner search by CIDR range (up to /24)

**Strategy:** Start with free tier. If a domain has >50 subdomains, upgrade or supplement with hostsearch and crt.sh.

## Tips

- **Combine DNSdumpster + crt.sh** for maximum subdomain coverage - they use different techniques
- **Reverse IP** is invaluable for shared hosting - reveals other targets on same infrastructure
- **TXT records** leak cloud service usage - look for verification tokens
- **MX records** immediately tell you the email platform (Google, O365, self-hosted)
- **CNAME chains** can reveal CDN origin IPs when combined with historical data
- **Banner data** from domain lookup includes HTTP server headers - quick tech fingerprinting
- **Rate limit**: 1 request per 2 seconds - be patient, space out requests
- **ASN data** groups IPs by organization - reveals the target's network footprint
