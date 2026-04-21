# Shodan API Reference

## Base URLs

- **Main API**: `https://api.shodan.io`
- **Exploits API**: `https://exploits.shodan.io/api`

## Authentication

All requests require the API key as a query parameter:
```
GET https://api.shodan.io/shodan/host/1.2.3.4?key=YOUR_API_KEY
```

Get your API key at: https://account.shodan.io

## Endpoints

### API Info (Free)
```
GET /api-info?key=API_KEY
```
Returns plan info, query/scan credits, usage limits.

Response:
```json
{
  "scan_credits": 100,
  "usage_limits": {"scan_credits": 100, "query_credits": 100, "monitored_ips": 16},
  "plan": "dev",
  "https": true,
  "unlocked": true,
  "query_credits": 97,
  "monitored_ips": null,
  "unlocked_left": 97,
  "telnet": true
}
```

### Host Lookup (Free)
```
GET /shodan/host/{ip}?key=API_KEY
GET /shodan/host/{ip}?key=API_KEY&history=true&minify=false
```
Returns all services, banners, TLS certs, vulns for an IP.

Parameters:
- `history` (bool) - include historical banners
- `minify` (bool) - return only basic host info (ports, vulns, hostnames)

### Host Search (1 Credit)
```
GET /shodan/host/search?query=apache+port:443&key=API_KEY
GET /shodan/host/search?query=apache&facets=country:10&page=1&minify=true&key=API_KEY
```
Full search with results. Costs 1 query credit for filtered or paged queries.

Parameters:
- `query` (string, required) - Shodan query
- `facets` (string) - comma-separated facets (e.g., "country:10,org:5")
- `page` (int) - page number (default 1)
- `minify` (bool) - minimal results

### Host Count (Free)
```
GET /shodan/host/count?query=apache+port:443&key=API_KEY
GET /shodan/host/count?query=apache&facets=country:10&key=API_KEY
```
Returns total count and optional facets without consuming credits.

### Search Filters (Free)
```
GET /shodan/host/search/filters?key=API_KEY
```
Returns list of all available search filters.

### Search Facets (Free)
```
GET /shodan/host/search/facets?key=API_KEY
```
Returns list of all available facets.

### Search Tokens (Free)
```
GET /shodan/host/search/tokens?query=apache+port:443&key=API_KEY
```
Breaks query into tokens, identifies filters.

### Ports List (Free)
```
GET /shodan/ports?key=API_KEY
```
Lists all ports Shodan crawlers monitor.

### DNS Resolve (Free)
```
GET /dns/resolve?hostnames=google.com,github.com&key=API_KEY
```
Resolves hostnames to IPs. Returns JSON object mapping hostname to IP.

Response:
```json
{"google.com": "142.250.80.46", "github.com": "140.82.121.4"}
```

### DNS Reverse (Free)
```
GET /dns/reverse?ips=8.8.8.8,1.1.1.1&key=API_KEY
```
Reverse DNS lookup. Returns JSON object mapping IP to hostname list.

Response:
```json
{"8.8.8.8": ["dns.google"], "1.1.1.1": ["one.one.one.one"]}
```

### DNS Domain (1 Credit)
```
GET /dns/domain/{domain}?key=API_KEY
GET /dns/domain/{domain}?history=true&type=A&page=1&key=API_KEY
```
Returns subdomains and DNS entries for a domain.

Parameters:
- `history` (bool) - include historical DNS data
- `type` (string) - filter by record type (A, AAAA, CNAME, MX, NS, SOA, TXT)
- `page` (int) - page number

### On-Demand Scan (Scan Credits)
```
POST /shodan/scan?key=API_KEY
Body: ips=1.2.3.4,5.6.7.8
```
Request Shodan to scan specific IPs. Costs 1 scan credit per IP.

### Scan Status (Free)
```
GET /shodan/scan/{id}?key=API_KEY
```
Check scan status: SUBMITTING, QUEUE, PROCESSING, DONE.

### My IP (Free)
```
GET /tools/myip?key=API_KEY
```
Returns your public IP as a string.

### HTTP Headers (Free)
```
GET /tools/httpheaders?key=API_KEY
```
Returns the HTTP headers your client sends.

### Account Profile (Free)
```
GET /account/profile?key=API_KEY
```
Returns account details.

## Exploits API

### Exploit Search
```
GET https://exploits.shodan.io/api/search?query=CVE-2021-44228&key=API_KEY
GET https://exploits.shodan.io/api/search?query=apache&facets=platform&page=1&key=API_KEY
```

Parameters:
- `query` (string, required) - search query
- `facets` (string) - comma-separated facets (author, platform, port, source, type)
- `page` (int) - page number

Query filters: author, bid, cve, date, description, platform, port, title, type (dos, exploit, local, remote, shellcode, webapps)

### Exploit Count
```
GET https://exploits.shodan.io/api/count?query=CVE-2021-44228&key=API_KEY
```
Same as search but returns count only.

## Error Codes

| Code | Meaning |
|------|---------|
| 200 | Success |
| 401 | Invalid API key |
| 402 | Insufficient credits (payment required) |
| 404 | Resource not found |
| 429 | Rate limited - slow down requests |

Error response format:
```json
{"error": "Description of what went wrong"}
```

## Credit Model

| Tier | Query Credits/mo | Scan Credits/mo | Monitored IPs |
|------|-----------------|-----------------|---------------|
| Free | Limited | 0 | 16 |
| Membership | 100 | 100 | 16 |
| Freelancer | 10,000 | 5,000 | 256 |
| Small Business | 20,000 | 10,000 | 65536 |
| Corporate | Unlimited | Unlimited | Unlimited |

**Free operations** (no credits): host lookup, DNS resolve/reverse, count, api-info, filters, facets, ports, myip
**1 query credit**: search (filtered/paged), domain DNS
**1 scan credit**: on-demand scan per IP

## Rate Limits

- Free/Membership: ~1 request/second
- Higher tiers: increased rate limits
- All tiers: 429 on excess (retry with backoff)
