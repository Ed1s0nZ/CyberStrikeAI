# Amass Guide

Comprehensive subdomain enumeration and attack surface mapping.

## Installation

```bash
go install -v github.com/owasp-amass/amass/v4/cmd/amass@latest
```

## Basic Usage

```bash
amass enum -d example.com
```

## Common Options

| Option | Description |
|--------|-------------|
| `-active` | Active enumeration |
| `-passive` | Passive enumeration (default) |
| `-d` | Target domain |
| `-o` | Output file |
| `-rf` | Input file with domains |
| `-timeout` | Timeout for enumeration |
| `-max-dns-queries` | Maximum DNS queries |

## Modes

### Passive Mode (Default)

```bash
amass enum -passive -d example.com -o passive.txt
```

Uses data sources without direct DNS queries.

### Active Mode

```bash
amass enum -active -d example.com -o active.txt
```

Performs DNS queries and active reconnaissance.

### Brute Force Mode

```bash
amass enum -brute -d example.com -o brute.txt
```

Uses built-in wordlist for subdomain discovery.

## Examples

### Basic Enumeration
```bash
amass enum -d target.com -o subs.txt
```

### Multiple Domains
```bash
amass enum -d target1.com -d target2.com -o subs.txt
```

### From File
```bash
amass enum -df domains.txt -o subs.txt
```

### With Maximum DNS Queries
```bash
amass enum -active -d example.com -max-dns-queries 20000
```

## Asset Discovery (Additional Feature)

```bash
amass intel -d example.com
```

Discovers related domains, certificates, and other intelligence.

## Tips

1. **Passive first** - Start with passive mode to avoid detection
2. **Active for depth** - Use active mode when passive results insufficient
3. **Long runtime** - Amass is comprehensive but slow
4. **API keys** - Configure APIs for better results

## Configuration

`~/.config/amass/config.ini` for API keys and settings.
