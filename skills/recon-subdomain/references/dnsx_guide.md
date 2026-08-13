# DNSx Guide

Fast DNS toolkit for resolution and record enumeration.

## Installation

```bash
go install -v github.com/projectdiscovery/dnsx/cmd/dnsx@latest
```

## Basic Usage

```bash
dnsx -l subs.txt
```

## Common Options

| Option | Description |
|--------|-------------|
| `-l` | Input file with domains |
| `-o` | Output file |
| `-a` | A record query |
| `-aaaa` | AAAA record query |
| `-cname` | CNAME record query |
| `-mx` | MX record query |
| `-txt` | TXT record query |
| `-ns` | NS record query |
| `-soa` | SOA record query |
| `-silent` | Only print results |
| `-resp` | Include DNS responses |
| `-json` | JSON output |
| `-rcode` | Filter by response code |
| `-retry` | Retry attempts |

## Examples

### Basic Resolution
```bash
dnsx -l subs.txt -o resolved.txt
```

### With Response Details
```bash
dnsx -l subs.txt -resp -o resolved.txt
```

### JSON Output
```bash
dnsx -l subs.txt -json -o resolved.json
```

### Specific Record Types
```bash
# A records only
dnsx -l subs.txt -a -only-a

# CNAME records only
dnsx -l subs.txt -cname -only-cname

# All record types
dnsx -l subs.txt -a -aaaa -cname -mx -txt -ns -soa
```

### Filter by Response Code
```bash
# Only successful resolutions
dnsx -l subs.txt -rcode,noerror -o resolved.txt

# Only failed resolutions
dnsx -l subs.txt -rcode,nxdomain -o failed.txt
```

### Stdin Input
```bash
cat subs.txt | dnsx -silent
```

## JSON Output Format

```json
{
  "host": "example.com",
  "a": ["1.2.3.4"],
  "aaaa": ["2001:db8::1"],
  "cname": ["alias.example.com"],
  "mx": ["mail.example.com"],
  "txt": ["v=spf1 include:_spf.example.com ~all"],
  "ns": ["ns1.example.com"],
  "soa": ["ns1.example.com"],
  "status": "resolved"
}
```

## Wildcard Detection

```bash
# Test for wildcard
echo "test123xyz.example.com" | dnsx -silent
# If resolves, wildcard exists
```

## Tips

1. **Pipe from subfinder** - `subfinder -d example.com | dnsx -silent`
2. **Use -resp** for debugging DNS issues
3. **JSON format** for programmatic parsing
4. **Combine with httpx** - `dnsx -l subs.txt | httpx -silent`
