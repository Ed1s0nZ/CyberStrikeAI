# crt.sh Reference

## What is Certificate Transparency?

Certificate Transparency (CT) is a framework (RFC 6962) that requires CAs to log every TLS certificate they issue to public, auditable logs. This makes it impossible to issue a certificate secretly.

## crt.sh

crt.sh (operated by Sectigo) is the most comprehensive CT log search engine. It aggregates data from all major CT logs and provides both a web interface and programmatic access.

## Access Methods

### HTTP JSON API
```
GET https://crt.sh/?q=QUERY&output=json
GET https://crt.sh/?q=QUERY&output=json&exclude=expired
```

No authentication required. Returns JSON array of certificate entries.

### Direct PostgreSQL
```
Host: crt.sh
Database: certwatch
User: guest
Port: 5432 (default)
```

Allows full SQL queries against the certificate database.

### Key SQL Tables
- `certificate` - raw certificate data
- `certificate_identity` - extracted names (dNSName, rfc822Name)
- `ca_certificate` - CA certificates
- `ct_log_entry` - CT log entries with timestamps

### Useful SQL Functions
- `x509_commonName(certificate)` - extract CN
- `x509_issuerName(certificate)` - extract issuer DN
- `x509_notBefore(certificate)` - validity start
- `x509_notAfter(certificate)` - validity end
- `x509_subjectName(certificate)` - full subject DN

## Query Wildcards

crt.sh uses SQL LIKE wildcards:
- `%` - match any characters (SQL wildcard)
- `_` - match single character
- `%.example.com` - all subdomains of example.com
- `%.%.example.com` - sub-subdomains
- `example.%` - all TLDs

## Response Fields (JSON API)

| Field | Description |
|-------|-------------|
| `id` | crt.sh certificate ID |
| `issuer_ca_id` | Issuer CA ID |
| `issuer_name` | Full issuer DN |
| `common_name` | Certificate CN |
| `name_value` | SAN dNSName values (newline-separated) |
| `serial_number` | Certificate serial |
| `not_before` | Validity start |
| `not_after` | Validity end |
| `entry_timestamp` | When the cert appeared in CT logs |
| `result_count` | Number of matching entries |

## Interesting Queries for Security Research

```sql
-- Find all certs for a domain issued in the last 30 days
SELECT * FROM certificate_identity ci
JOIN certificate c ON c.ID = ci.CERTIFICATE_ID
WHERE ci.NAME_VALUE LIKE '%.target.com'
  AND c.ENTRY_TIMESTAMP > NOW() - INTERVAL '30 days';

-- Find certs with specific issuer
SELECT * FROM certificate_identity ci
JOIN certificate c ON c.ID = ci.CERTIFICATE_ID
WHERE ci.NAME_VALUE LIKE '%.target.com'
  AND x509_issuerName(c.CERTIFICATE) LIKE '%Let''s Encrypt%';

-- Count certs per subdomain
SELECT ci.NAME_VALUE, COUNT(*) as cert_count
FROM certificate_identity ci
WHERE ci.NAME_VALUE LIKE '%.target.com'
GROUP BY ci.NAME_VALUE
ORDER BY cert_count DESC
LIMIT 50;
```
