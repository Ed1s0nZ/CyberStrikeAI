# Gobuster Guide

Directory/file & DNS busting tool written in Go.

## Installation

```bash
go install github.com/OJ/gobuster/v3/cmd/gobusterdir@main
```

Or from GitHub releases:
```bash
wget https://github.com/OJ/gobuster/releases/download/v3.6.0/gobuster_3.6.0_linux_amd64.tar.gz
tar -xzf gobuster_3.6.0_linux_amd64.tar.gz
sudo mv gobuster /usr/local/bin/
```

## Basic Usage

```bash
gobuster dir -u https://target.com -w wordlist.txt
```

## Common Options

| Option | Description |
|--------|-------------|
| `-u` | Target URL |
| `-w` | Wordlist path |
| `-t` | Number of threads (default: 10) |
| `-d` | Show debug output |
| `-k` | Skip SSL certificate verification |
| `-r` | Follow redirects |
| `-e` | Show extended output (status, length) |
| `-x` | File extensions to search |
| `-o` | Output file |
| `-q` | Quiet mode (don't print banner/errors) |
| `-n` | Don't print status codes |
| `-P` | Set username:password for proxy |
| `-p` | Proxy URL |
| `--status-codes` | Comma-separated status codes |
| `--no-error` | Don't display errors |
| `--timeout` | HTTP timeout |
| `--threads` | Number of threads |
| `--deep-scan` | Deep scan (recursion) |
| `--random-agent` | Use random User-Agent |
| `--header` | Add custom header |
| `--cookies` | Add cookies |
| `--expanded` | Expand result format |

## Modes

### Dir Mode (Default)

```bash
gobuster dir -u https://target.com -w wordlist.txt
```

### DNS Mode

```bash
gobuster dns -d example.com -w subdomains.txt
```

### Vhost Mode

```bash
gobuster vhost -u https://target.com -w subdomains.txt
```

## Examples

### Basic Directory Scan

```bash
gobuster dir -u https://target.com -w /usr/share/seclists/Discovery/Web-Content/common.txt
```

### With Status Filter

```bash
gobuster dir -u https://target.com -w wordlist.txt \
  --status-codes 200,301,302,403
```

### File Extensions

```bash
gobuster dir -u https://target.com -w wordlist.txt -x php,txt,html,bak
```

### Recursive Scan

```bash
gobuster dir -u https://target.com -w wordlist.txt -r
```

### With Authentication

```bash
# Basic auth
gobuster dir -u https://user:pass@target.com -w wordlist.txt

# Custom header
gobuster dir -u https://target.com -w wordlist.txt \
  --header "Authorization: Bearer TOKEN"
```

### Proxy Support

```bash
gobuster dir -u https://target.com -w wordlist.txt -p http://127.0.0.1:8080
```

### Cookies

```bash
gobuster dir -u https://target.com -w wordlist.txt \
  --cookies "session=abc123;user=admin"
```

### Save Output

```bash
gobuster dir -u https://target.com -w wordlist.txt -o results.txt
```

## DNS Mode Examples

### Subdomain Enumeration

```bash
gobuster dns -d example.com -w subdomains.txt
```

### With Wildcard Detection

```bash
gobuster dns -d example.com -w subdomains.txt --wildcard
```

### Show CNAMEs

```bash
gobuster dns -d example.com -w subdomains.txt --show-cname
```

## Vhost Mode Examples

### Virtual Host Discovery

```bash
gobuster vhost -u https://target.com -w vhosts.txt
```

## Tips

1. **Use -k for self-signed certs** - Skip SSL verification
2. **Adjust threads** - Default 10, increase for faster scans
3. **Filter status codes** - `--status-codes` to reduce noise
4. **Add extensions** - `-x` for specific file types
5. **Use -q for scripts** - Quiet mode for cleaner output
6. **Follow redirects** - `-r` for deeper scanning
