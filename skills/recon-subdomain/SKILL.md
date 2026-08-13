---
name: recon-subdomain
description: Subdomain enumeration and DNS reconnaissance using subfinder, amass, dnsx, and other tools. Use this skill when user needs to discover subdomains, perform DNS enumeration, gather DNS records, or find hidden subdomains of a target domain.
---

# Subdomain Enumeration / DNS Reconnaissance

## Authorization Warning

**IMPORTANT:** Subdomain enumeration without proper authorization may violate terms of service. Always ensure you have:
- Written permission from the target domain owner
- Defined scope of authorized testing
- Legal compliance with local regulations

## Prerequisites

Required tools that must be installed on your system:
- **subfinder** - `go install -v github.com/projectdiscovery/subfinder/v2/cmd/subfinder@latest`
- **dnsx** - `go install -v github.com/projectdiscovery/dnsx/cmd/dnsx@latest`

Optional tools:
- **amass** - `go install -v github.com/owasp-amass/amass/v4/cmd/amass@latest`
- **assetfinder** - `go install github.com/tomnomnom/assetfinder@latest`
- **puredns** - `go install github.com/d3mondev/puredns/v2@latest`

## Quick Start

Most commonly used commands for subdomain enumeration:

### Fast Subdomain Discovery (Subfinder)
```bash
subfinder -d example.com -o subs.txt
```

### Subdomain Discovery + DNS Resolution
```bash
subfinder -d example.com -silent | dnsx -silent -resp > resolved_subs.txt
```

### Comprehensive Subdomain Enumeration (Amass)
```bash
amass enum -passive -d example.com -o amass_subs.txt
```

### Multiple Tools Combined
```bash
subfinder -d example.com -silent | tee subs1.txt && \
assetfinder --subs-only example.com | tee subs2.txt && \
cat subs1.txt subs2.txt | sort -u > all_subs.txt
```

## Common Scenarios

### Scenario 1: Quick Passive Subdomain Discovery

When you need fast subdomain discovery without direct interaction:

```bash
subfinder -d example.com -o subs.txt
```

**Parameters:**
- `-d example.com` - Target domain
- `-o subs.txt` - Output file
- `-silent` - Suppress stderr output (optional)

**Example:**
```bash
subfinder -d target.com -o target_subs.txt
subfinder -d target.com -silent | head -20
```

---

### Scenario 2: Active Subdomain Enumeration

When you need comprehensive active enumeration:

```bash
amass enum -active -d example.com -o amass_active.txt
```

**Parameters:**
- `-active` - Active enumeration (direct DNS queries)
- `-d example.com` - Target domain
- `-o amass_active.txt` - Output file

**Example:**
```bash
amass enum -active -d target.com -o target_amass.txt
```

**Passive mode (no direct queries):**
```bash
amass enum -passive -d example.com -o amass_passive.txt
```

---

### Scenario 3: DNS Resolution of Discovered Subdomains

When you have a list of subdomains and need to verify which resolve:

```bash
dnsx -l subs.txt -o resolved.txt
```

**Parameters:**
- `-l subs.txt` - Input file with subdomains
- `-o resolved.txt` - Output file
- `-resp` - Include DNS responses in output
- `-json` - Output in JSON format

**Example:**
```bash
dnsx -l target_subs.txt -o resolved.txt -resp
```

**With response details:**
```bash
dnsx -l subs.txt -resp -json -o resolved.json
```

---

### Scenario 4: DNS Record Enumeration

When you need to gather specific DNS records:

```bash
# A records
dnsx -l subs.txt -a -only-a

# AAAA records (IPv6)
dnsx -l subs.txt -aaaa -only-aaaa

# CNAME records
dnsx -l subs.txt -cname -only-cname

# TXT records
dnsx -l subs.txt -txt -only-txt

# MX records
dnsx -l subs.txt -mx -only-mx

# All records
dnsx -l subs.txt -a -aaaa -cname -mx -txt -ns -soa
```

---

### Scenario 5: Wildcard Detection

When the target has wildcard DNS records:

```bash
# Detect wildcard subdomains
puredns discard wildcards.txt < subs.txt > valid_subs.txt
```

**Alternative with dnsx:**
```bash
# Test for wildcard
echo "randomtest12345.example.com" | dnsx -silent
# If resolves, wildcard exists

# Remove wildcard responses
dnsx -l subs.txt -silent -rcode,noerror | grep -v "randomtest"
```

---

### Scenario 6: Subdomain Brute Forcing

When you need to discover subdomains via wordlist:

```bash
# Using puredns
puredns bruteforce wordlist.txt example.com | tee brute_subs.txt
```

**Common wordlists:**
```bash
# SecLists
puredns bruteforce /usr/share/seclists/Discovery/DNS/subdomains-top1million-5000.txt example.com
```

---

### Scenario 7: Multi-Tool Enumeration

Combine multiple tools for maximum coverage:

```bash
# Create output file
> all_subs.txt

# Run subfinder
subfinder -d example.com -silent >> all_subs.txt

# Run assetfinder
assetfinder --subs-only example.com >> all_subs.txt

# Run amass (passive)
amass enum -passive -d example.com >> all_subs.txt

# Sort and deduplicate
sort -u all_subs.txt -o all_subs.txt
```

---

### Scenario 8: Certificate Transparency Log Search

Find subdomains from SSL/TLS certificates:

```bash
# Using crt.sh (web)
curl -s "https://crt.sh/?q=%.example.com&output=json" | \
  jq -r '.[].name_value' | sort -u > ct_subs.txt

# Using subfinder (CT integration)
subfinder -d example.com -sources crtsh -o ct_subs.txt
```

---

### Scenario 9: DNS Zone Transfer

Attempt zone transfer (rarely successful but worth trying):

```bash
# Try zone transfer
dig axfr @ns1.example.com example.com

# With specific nameserver
host -t axfr example.com ns1.example.com
```

---

### Scenario 10: Subdomain Takeover Detection

Check for dangling DNS records:

```bash
# Using subjack
subjack -w subs.txt -t 100 -timeout 10 -o takeovers.txt

# Using nuclei (requires templates)
nuclei -l subs.txt -t /path/to/takeover-templates/
```

---

## Tool Selection Guide

| Scenario | Recommended Tool | Command |
|----------|------------------|---------|
| Quick passive discovery | subfinder | `subfinder -d <domain> -o subs.txt` |
| Comprehensive enumeration | amass | `amass enum -d <domain> -o subs.txt` |
| DNS resolution verification | dnsx | `dnsx -l subs.txt -o resolved.txt` |
| Certificate search | subfinder (crtsh) | `subfinder -d <domain> -sources crtsh` |
| Brute force | puredns | `puredns bruteforce wordlist.txt <domain>` |
| Wildcard handling | puredns | `puredns discard wildcards.txt < subs.txt` |
| Simple alternative | assetfinder | `assetfinder --subs-only <domain>` |

**Tool Comparison:**

| Tool | Speed | Coverage | Passive | Active | Use Case |
|------|-------|----------|---------|--------|----------|
| **subfinder** | Fast | Good | Yes | Limited | Quick discovery |
| **amass** | Slow | Excellent | Yes | Yes | Comprehensive |
| **assetfinder** | Very Fast | Basic | Yes | No | Quick checks |
| **puredns** | Fast | N/A | No | Yes | Brute force |

---

## Common Wordlists

Subdomain brute forcing wordlists:

| Wordlist | Size | Source |
|----------|------|--------|
| subdomains-top1million-5k | 5,000 | SecLists |
| subdomains-top1million-20k | 20,000 | SecLists |
| subdomains-top1million-500k | 500,000 | SecLists |
| DNS-Jaded-Top.txt | ~10,000 | Assetnote wordlists |

**Example usage:**
```bash
puredns bruteforce /path/to/subdomains-top1million-5000.txt example.com
```

---

## Workflow Example

Complete subdomain enumeration workflow:

```bash
# 1. Passive enumeration
subfinder -d target.com -silent > passive.txt
amass enum -passive -d target.com >> passive.txt
assetfinder --subs-only target.com >> passive.txt
sort -u passive.txt -o passive.txt

# 2. Resolve subdomains
dnsx -l passive.txt -silent -o resolved.txt

# 3. Check for alive HTTP services
cat resolved.txt | httpx -silent -status-code -title > alive.txt

# 4. Brute force (optional)
puredns bruteforce wordlist.txt target.com > brute.txt
dnsx -l brute.txt -silent >> resolved.txt

# 5. Final sorted list
sort -u resolved.txt -o final_subs.txt
```

---

## Output Formats

### Subfinder JSON Output

```bash
subfinder -d example.com -json -o subs.json
```

**JSON structure:**
```json
{
  "host": "sub.example.com",
  "source": "crtsh"
}
```

### Dnsx JSON Output

```bash
dnsx -l subs.txt -json -o resolved.json
```

**JSON structure:**
```json
{
  "host": "sub.example.com",
  "a": ["1.2.3.4"],
  "aaaa": [],
  "cname": [],
  "status": "resolved"
}
```

---

## Tips and Best Practices

1. **Start passive** - Use passive methods first to avoid detection
2. **Combine tools** - No single tool finds everything
3. **Check wildcards** - Wildcard DNS can skew results
4. **Verify resolution** - Not all discovered subdomains resolve
5. **Rate limiting** - Be careful with active queries to avoid blocking
6. **Save results** - Keep intermediate results for analysis

---

### Scenario: Persistent Storage of Subdomain Enumeration

When you need to persist subdomain discovery results to the database:

```bash
# Store from file (flat hierarchy)
subfinder -d example.com | python .claude/skills/recon-subdomain/scripts/subdomain_storage.py

# Store from file with subsystem
subfinder -d example.com | python .claude/skills/recon-subdomain/scripts/subdomain_storage.py \
  --subsystem "External Infrastructure"

# Store from file (alternative)
python .claude/skills/recon-subdomain/scripts/subdomain_storage.py \
  --input-file subdomains.txt \
  --subsystem "Customer A"
```

**Parameters:**
- `--subsystem` - Subsystem name (optional, omit for flat hierarchy)
- `--input-file` - File containing subdomains (one per line, optional, reads from stdin if not provided)

**Database location:** `./data/results.db`

**Related skills:** `results-storage` - Query data, generate reports

---

## Resources

### Scripts

- `scripts/merge_subdomains.py` - Merge and deduplicate multiple subdomain lists
- `scripts/filter_resolved.py` - Filter resolved subdomains with custom resolution logic
- `scripts/subdomain_stats.py` - Generate statistics on discovered subdomains

### References

- `references/subfinder_guide.md` - Comprehensive subfinder reference
- `references/amass_guide.md` - Detailed amass usage documentation
- `references/dnsx_guide.md` - DNS resolution tool reference
- `references/dns_techniques.md` - Advanced DNS enumeration techniques

### Assets

- `assets/subdomains-top1m-5k.txt` - Top 5,000 common subdomain words
- `assets/resolvers.txt` - Trusted DNS resolver list
- `assets/wildcard-test.txt` - Subdomain wildcard testing patterns
