# DNSdumpster API Reference

## Base URLs

- **DNSdumpster API**: `https://api.dnsdumpster.com`
- **HackerTarget API**: `https://api.hackertarget.com` (supplementary endpoints)

## Authentication

### DNSdumpster API
All requests require the API key as an HTTP header:
```
X-API-Key: YOUR_API_KEY
```

Get your API key at: https://dnsdumpster.com (account dashboard after login)

### HackerTarget API
Free endpoints require no authentication. Member endpoints accept:
- Query parameter: `&apikey=YOUR_KEY`
- HTTP header: `X-API-Key: YOUR_KEY` (HTTPS required)

## DNSdumpster Endpoints

### Domain DNS Lookup (Primary Endpoint)
```
GET https://api.dnsdumpster.com/domain/{domain}
Headers: X-API-Key: YOUR_KEY
```

Returns comprehensive DNS data for a domain: A records (hosts + IPs), MX, NS, TXT,
CNAME, SOA records, ASN info, netblocks, banners/technology data.

Parameters:
- `page` (int) - page number for paginated results (Plus tier only)
- `map` (int, 1) - include base64-encoded domain network map (Plus tier only)

Response (JSON):
```json
{
  "dns": [
    {
      "host": "www.example.com",
      "ip": "93.184.216.34",
      "reverse_dns": "93.184.216.34",
      "asn": "AS15133",
      "country": "US",
      "header": "Apache/2.4",
      "as_name": "Edgecast Inc.",
      "netblock": "93.184.216.0/24"
    }
  ],
  "mx": [
    {
      "host": "mail.example.com",
      "ip": "93.184.216.35",
      "priority": "10",
      "reverse_dns": "mail.example.com",
      "asn": "AS15133",
      "as_name": "Edgecast Inc."
    }
  ],
  "ns": [
    {
      "host": "ns1.example.com",
      "ip": "93.184.216.36",
      "reverse_dns": "ns1.example.com",
      "asn": "AS15133",
      "as_name": "Edgecast Inc."
    }
  ],
  "txt": [
    {"value": "v=spf1 include:_spf.google.com ~all"}
  ],
  "cname": [
    {"host": "blog.example.com", "target": "example.github.io"}
  ],
  "soa": [
    {"mname": "ns1.example.com", "rname": "admin.example.com"}
  ]
}
```

### Banner Search by CIDR (Plus Tier)
```
GET https://api.dnsdumpster.com/banners/{CIDR}
Headers: X-API-Key: YOUR_KEY
```

Search for HTTP/HTTPS banners across a network range. Maximum /24 CIDR (254 hosts).

## HackerTarget Endpoints (Supplementary)

### GeoIP Lookup (Free)
```
GET https://api.hackertarget.com/geoip/?q={ip}
```
Returns plain text with IP geolocation: country, region, city, latitude, longitude, ASN.

### Reverse DNS (Free)
```
GET https://api.hackertarget.com/reversedns/?q={ip}
```
Returns hostnames associated with an IP address.

### Reverse IP Lookup (Free)
```
GET https://api.hackertarget.com/reverseiplookup/?q={ip}
```
Returns all domains hosted on an IP address (shared hosting discovery). One domain per line.

### DNS Lookup (Free)
```
GET https://api.hackertarget.com/dnslookup/?q={domain}
```
Returns all DNS records for a domain in plain text format.

### Host Search (Free)
```
GET https://api.hackertarget.com/hostsearch/?q={domain}
```
Returns hosts related to a domain. CSV format: hostname,ip.

### AS Lookup (Free)
```
GET https://api.hackertarget.com/aslookup/?q={ip}
```
Returns ASN information for an IP. CSV format: ip,asn,range,description,country.

### Whois (Free)
```
GET https://api.hackertarget.com/whois/?q={domain_or_ip}
```
Returns WHOIS data in plain text.

## Error Codes

| Code | Meaning |
|------|---------|
| 200 | Success |
| 401 | Invalid API key |
| 403 | Access denied (insufficient tier) |
| 404 | Domain or resource not found |
| 429 | Rate limited - wait and retry |

Error response format (DNSdumpster):
```json
{"error": "Description of what went wrong"}
```

Error response format (HackerTarget):
```
error check your search parameter
```

## Rate Limits

- **DNSdumpster API**: 1 request per 2 seconds
- **HackerTarget Free**: 50 requests/day, max 2 requests/second
- **HackerTarget Member**: Higher quotas based on plan
- All endpoints: HTTP 429 on rate limit exceeded

## Tiers

### DNSdumpster

| Feature | Free | Plus |
|---------|------|------|
| Domain DNS lookup | 50 records | 200 records |
| Pagination | No | Yes (`?page=2`) |
| Domain mapping | No | Yes (`?map=1`) |
| Banner/CIDR search | No | Yes (up to /24) |
| Rate limit | 1 req/2s | 1 req/2s |

### HackerTarget

| Feature | Free | Member |
|---------|------|--------|
| Daily requests | 50 | Higher |
| Response format | Text | Text + JSON |
| Rate limit | 2 req/s | Higher |

## Response Formats

- **DNSdumpster API**: JSON
- **HackerTarget API**: Plain text (CSV-like), some endpoints support JSON for members
