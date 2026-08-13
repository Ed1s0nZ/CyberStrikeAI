# FFUF Guide

Fuzz Faster U Fool (FFUF) - A fast web fuzzer written in Go.

## Installation

```bash
go install github.com/ffuf/ffuf@latest
```

## Basic Usage

```bash
ffuf -w wordlist.txt -u https://target.com/FUZZ
```

The `FUZZ` keyword is replaced with each word from the wordlist.

## Common Options

| Option | Description |
|--------|-------------|
| `-w` | Wordlist file |
| `-u` | Target URL with FUZZ keyword |
| `-t` | Number of threads (default: 40) |
| `-mc` | Match status codes (e.g., 200,301) |
| `-mw` | Match response words |
| `-ml` | Match response lines |
| `-ms` | Match response size |
| `-fc` | Filter status codes |
| `-fw` | Filter response words |
| `-fl` | Filter response lines |
| `-fs` | Filter response size |
| `-recursion` | Enable recursive scanning |
| `-recursion-depth` | Recursion depth (default: infinite) |
| `-X` | HTTP method |
| `-H` | Add custom header |
| `-b` | Add cookie |
| `-d` | POST data |
| `-o` | Output file |
| `-of` | Output format (json, html, csv) |
| `-rate` | Rate limit (requests/second) |
| `-p` | Delay per request |
| `-timeout` | Request timeout |
| `-x` | Proxy URL |
| `-s` | Do not print additional info |
| `-v` | Verbose mode |
| `-acc` | Calibration (autosuggest filter settings) |

## Examples

### Basic Scan

```bash
ffuf -w wordlist.txt -u https://target.com/FUZZ
```

### Filter by Status Code

```bash
# Only show 200, 301, 302
ffuf -w wordlist.txt -u https://target.com/FUZZ -mc 200,301,302

# Exclude 404
ffuf -w wordlist.txt -u https://target.com/FUZZ -fc 404
```

### Filter by Response Size

```bash
# Match specific size
ffuf -w wordlist.txt -u https://target.com/FUZZ -ms 1520

# Filter out common sizes (false positives)
ffuf -w wordlist.txt -u https://target.com/FUZZ -fs 4242
```

### Recursive Scanning

```bash
ffuf -w wordlist.txt -u https://target.com/FUZZ -recursion -recursion-depth 2
```

### File Extension Fuzzing

```bash
# Single extension
ffuf -w wordlist.txt -u https://target.com/FUZZ -X .txt

# Multiple extensions
ffuf -w wordlist.txt -u https://target.com/FUZZ -X .txt,.php,.bak
```

### Virtual Host Discovery

```bash
ffuf -w subdomains.txt -u http://192.168.1.100 -H "Host: FUZZ.example.com"
```

### POST Parameter Fuzzing

```bash
ffuf -w params.txt -u https://target.com/login -X POST \
  -d "username=FUZZ&password=test"
```

### With Authentication

```bash
# Basic auth
ffuf -w wordlist.txt -u https://user:pass@target.com/FUZZ

# Header-based
ffuf -w wordlist.txt -u https://target.com/FUZZ \
  -H "Authorization: Bearer TOKEN"
```

### Output Formats

```bash
# JSON output
ffuf -w wordlist.txt -u https://target.com/FUZZ -o results.json -of json

# HTML report
ffuf -w wordlist.txt -u https://target.com/FUZZ -o results.html -of html
```

### Calibration

Automatically suggest filter settings based on a test request:

```bash
ffuf -w wordlist.txt -u https://target.com/FUZZ -acc
```

## Advanced Features

### Multiple FUZZ Keywords

```bash
ffuf -w users.txt:FUZ1 -w ids.txt:FUZ2 \
  -u https://target.com/api/user/FUZ1/id/FUZ2
```

### Rate Limiting

```bash
# 100 requests per second
ffuf -w wordlist.txt -u https://target.com/FUZZ -rate 100
```

### Proxy Support

```bash
ffuf -w wordlist.txt -u https://target.com/FUZZ -x http://127.0.0.1:8080
```

### Ignore Wordlist Comments

```bash
ffuf -w wordlist.txt -u https://target.com/FUZZ -ic
```

### Delay Between Requests

```bash
ffuf -w wordlist.txt -u https://target.com/FUZZ -p 100ms
```

## Tips

1. **Use calibration first** - `-acc` helps find good filter settings
2. **Filter aggressively** - Remove common response sizes to reduce noise
3. **Check 403s** - Forbidden responses reveal valid paths
4. **Use recursion** - Discover nested directory structures
5. **Rate limiting** - Avoid WAF blocks with `-rate` or `-p`
6. **Match words/lines** - `-mw` and `-ml` for content-based filtering
