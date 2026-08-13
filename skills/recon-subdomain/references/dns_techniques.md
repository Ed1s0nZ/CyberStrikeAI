# Advanced DNS Enumeration Techniques

Advanced methods for comprehensive DNS reconnaissance.

## Certificate Transparency Log Search

### crt.sh API

```bash
# Basic search
curl -s "https://crt.sh/?q=%.example.com&output=json" | \
  jq -r '.[].name_value' | sort -u

# With subdomain expansion
curl -s "https://crt.sh/?q=example.com&output=json" | \
  jq -r '.[].name_value' | sed 's/\*\.//g' | sort -u
```

### Google CT Logs

```bash
# Using subfinder with CT sources
subfinder -d example.com -sources crtsh -o ct_subs.txt
```

### Censys Certificate Search

```bash
# Censys API for certificate search
curl -s "https://search.censys.io/api/v2/certificates/search?q=names: example.com&per_page=100" \
  -H "Authorization: YOUR_API_KEY"
```

---

## DNS Zone Transfer

### Attempt Zone Transfer

```bash
# Using dig
dig axfr @ns1.example.com example.com

# Using host
host -t axfr example.com ns1.example.com

# Multiple nameservers
for ns in $(host -t ns example.com | cut -d" " -f4); do
  echo "Trying $ns..."
  dig axfr @$ns example.com
done
```

**Note:** Zone transfers are rarely allowed in modern configurations.

---

## DNS Record Types

### ALL Record Query

```bash
# Query all record types
dig ANY example.com +noall +answer

# Specific types
dig A example.com
dig AAAA example.com
dig CNAME example.com
dig MX example.com
dig NS example.com
dig TXT example.com
dig SOA example.com
dig SRV example.com
dig PTR example.com
```

### TXT Record Enumeration

```bash
# All TXT records
dig txt example.com +short

# Specific TXT records (SPF, DKIM, DMARC)
dig txt _dmarc.example.com +short
dig txt default._domainkey.example.com +short
```

---

## SRV Record Discovery

```bash
# Common SRV records
# Active Directory
dig _ldap._tcp.example.com SRV
dig _kerberos._tcp.example.com SRV

# SIP
dig _sip._tcp.example.com SRV

# MySQL
dig _mysql._tcp.example.com SRV
```

---

## DNSSEC Discovery

```bash
# Check for DNSSEC
dig +dnssec example.com DNSKEY +short

# DS records
dig ds example.com +short

# RRSIG records
dig rrsig example.com +short
```

---

## Wildcard Detection Techniques

### Basic Wildcard Test

```bash
# Test random subdomain
nslookup randomtest12345.example.com

# Using dig
dig randomtest12345.example.com
```

### Automated Wildcard Detection

```bash
# Generate random subdomains
for i in {1..10}; do
  sub="test$i$(openssl rand -hex 4).example.com"
  if host "$sub" >/dev/null 2>&1; then
    echo "Wildcard detected: $sub"
  fi
done
```

### Using dnsx

```bash
# Test for wildcard
echo "test$(date +%s).example.com" | dnsx -silent
```

---

## Subdomain Permutations

### Altdns

Generate permutations from discovered subdomains:

```bash
# Using altdns
altdns -i subs.txt -o output.txt -w words.txt

# Resolve permutations
cat output.txt | dnsx -silent -o resolved.txt
```

### Custom Permutations

```bash
# Common patterns
for sub in $(cat subs.txt); do
  # Add dev/stage/prod
  echo "${sub}-dev"
  echo "${sub}-staging"
  echo "${sub}-prod"
  echo "dev-${sub}"
  echo "stage-${sub}"
  echo "prod-${sub}"
done | dnsx -silent
```

---

## DNSSEC and DANE

### Check DANE Records

```bash
# TLSA records
dig _443._tcp.example.com TLSA +short

# SMTP DANE
dig _25._tcp.mail.example.com TLSA +short
```

---

## Reverse DNS Enumeration

### From IP Range

```bash
# Reverse lookup IP range
for ip in $(seq 1 254); do
  host 192.168.1.$ip | grep -v "not found"
done
```

### Using dnsrecon

```bash
# Reverse lookup of IP range
dnsrecon -r 192.168.1.0/24 -n example.com
```

---

## DNS Cache Snooping

```bash
# Check if nameserver has cached record
dig @ns1.example.com sub.example.com A +norecurse

# With dnsrecon
dnsrecon -n example.com -c cache_snoop
```

---

## Stealthy DNS Enumeration

### Slow Query Rate

```bash
# With delay
while read sub; do
  echo "$sub"
  sleep 1
done < subs.txt | dnsx -silent
```

### Using Different Resolvers

```bash
# Public DNS resolvers
resolvers=(
  "8.8.8.8"
  "1.1.1.1"
  "64.6.64.6"
  "208.67.222.222"
)

for resolver in "${resolvers[@]}"; do
  dig @$resolver example.com
done
```

---

## Security Testing Considerations

1. **Rate Limiting** - Avoid rapid queries to prevent blocking
2. **Source IP Rotation** - Use multiple IPs for large scans
3. **Passive First** - Use passive methods before active enumeration
4. **Wildcards** - Always check for wildcard DNS patterns
5. **Verification** - Resolve all discovered subdomains

---

## Tools Comparison

| Technique | Tools | Detects |
|-----------|-------|---------|
| Certificate Search | crt.sh, subfinder | Historical SSL certificates |
| Zone Transfer | dig, host | Complete zone (if allowed) |
| SRV Discovery | dig | Service-specific records |
| DNSSEC | dig | DNSSEC configuration |
| Reverse DNS | dnsrecon, host | Domains from IP ranges |
| Permutations | altdns | Dev/prod variants |
| Wildcards | dnsx, custom | Wildcard patterns |
