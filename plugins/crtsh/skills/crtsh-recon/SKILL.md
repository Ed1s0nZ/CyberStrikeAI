# crt.sh Certificate Transparency Reconnaissance

## Overview

crt.sh is a free Certificate Transparency (CT) log search engine. Every TLS certificate issued by a trusted CA is logged in public CT logs. This means **every HTTPS hostname an organization has ever used** is discoverable - including internal names, staging environments, and shadow IT.

## Why CT Logs are Gold for Recon

When an organization gets a TLS certificate, the CA submits it to CT logs. This reveals:
- **Subdomains** that don't appear in DNS brute-force dictionaries
- **Internal hostnames** (dev.internal.corp.com) leaked via cert SANs
- **Staging/test environments** (staging-api.target.com)
- **Decommissioned services** that still resolve in DNS
- **Shadow IT** - certs issued by employees on unauthorized services
- **Organizational structure** - naming patterns reveal team structure
- **Technology stack** - cert issuers indicate automation level
- **Timeline** - when services were deployed/decommissioned

## Quick Start

```
# Fast subdomain discovery (most common use)
crtsh-search query="%.target.com" mode="subdomains"

# Full certificate analysis with issuer breakdown
crtsh-search query="%.target.com"

# Deep subdomain discovery
crtsh-search query="%.%.target.com"

# Include expired certs (historical infrastructure)
crtsh-search query="%.target.com" additional_args="--expired"

# Find related TLDs
crtsh-search query="%.target.%"
```

## Reconnaissance Workflow

### Phase 1: Subdomain Enumeration

```
# Start with broad wildcard search
crtsh-search query="%.target.com" mode="subdomains"

# Output: clean list of subdomains - feed into further tools
# Example output: api.target.com, mail.target.com, vpn.target.com, dev.target.com
```

### Phase 2: Certificate Analysis

```
# Full cert details - look for patterns
crtsh-search query="%.target.com"

# What to look for in output:
# - "top_issuers" → Let's Encrypt = automated, DigiCert = enterprise
# - Wildcard certs (*.target.com) → shared infrastructure
# - Internal names in SAN list (*.internal.target.com)
# - Old certs with names no longer in DNS (decommissioned but maybe still running)
```

### Phase 3: Deep Discovery

```
# Sub-subdomains (internal infrastructure)
crtsh-search query="%.%.target.com"
# Finds: db1.prod.target.com, api.staging.target.com

# Related TLDs (org may own .net, .io, .dev variants)
crtsh-search query="%.target.%"
# Finds: target.io, target.dev, target.net

# Historical (expired certs reveal old infrastructure)
crtsh-search query="%.target.com" additional_args="--expired"
```

### Phase 4: Targeted Hunting

```
# API endpoints
crtsh-search query="%.api.target.com" mode="subdomains"

# Mail infrastructure
crtsh-search query="%.mail.target.com" mode="subdomains"

# VPN/remote access
crtsh-search query="%.vpn.target.com" mode="subdomains"

# Development/staging
crtsh-search query="%.dev.target.com" mode="subdomains"
crtsh-search query="%.staging.target.com" mode="subdomains"
crtsh-search query="%.test.target.com" mode="subdomains"
```

### Phase 5: Chain with Other Tools

```
# 1. Get subdomains from crt.sh
crtsh-search query="%.target.com" mode="subdomains"

# 2. Resolve DNS for each subdomain
# Use: dnsenum, subfinder, or exec with dig/host

# 3. Scan live hosts
# Use: nmap on resolved IPs

# 4. Vulnerability scan
# Use: nuclei on live HTTP services
```

## Interpreting Results

### Subdomain Naming Patterns

| Pattern | Meaning |
|---------|---------|
| `api.target.com` | External API |
| `api-v2.target.com` | API versioning (v1 may be deprecated but alive) |
| `staging-api.target.com` | Staging (often less hardened) |
| `dev.target.com` | Development (may have debug endpoints) |
| `internal.target.com` | Internal (interesting if externally resolvable) |
| `vpn.target.com` | VPN endpoint |
| `mail.target.com` | Mail server |
| `mx1.target.com` | Mail exchanger |
| `ns1.target.com` | Nameserver |
| `db.target.com` | Database (should NOT be public) |
| `jenkins.target.com` | CI/CD (high-value target) |
| `grafana.target.com` | Monitoring (info leak potential) |
| `kibana.target.com` | Log viewer (info leak) |
| `admin.target.com` | Admin panel |
| `portal.target.com` | User portal |
| `sso.target.com` | SSO/auth (high value) |

### Certificate Issuers

| Issuer | Indicates |
|--------|-----------|
| Let's Encrypt | Automated cert management (likely modern infra) |
| DigiCert | Enterprise CA (likely large org, may have EV certs) |
| Comodo/Sectigo | Mixed use (check for cheap DV certs on important services) |
| Cloudflare | Behind Cloudflare CDN/proxy |
| Amazon Trust Services | Hosted on AWS (ACM cert) |
| Google Trust Services | Hosted on GCP |
| Self-signed | Internal service (if externally visible = misconfiguration) |

## Tips

- **Wildcard certs** (`*.target.com`) mean all subdomains share one cert - can't enumerate from CT alone, but the cert's SAN field may list specific names
- **Multi-domain certs** (SAN lists) reveal which services share infrastructure
- **Newly issued certs** may indicate infrastructure changes - monitor over time
- **Expired certs** on still-resolvable hosts are prime targets (forgotten, unpatched)
- **Certificate serial numbers** can be used to find the same cert in other CT logs or search engines
