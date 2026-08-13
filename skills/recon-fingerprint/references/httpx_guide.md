# HTTPx Guide

Fast and multi-purpose HTTP toolkit.

## Installation

```bash
go install -v github.com/projectdiscovery/httpx/cmd/httpx@latest
```

## Basic Usage

```bash
httpx -u https://target.com
```

## Common Options

| Option | Description |
|--------|-------------|
| `-u` | Target URL |
| `-l` | File with URLs |
| `-status-code` | Show HTTP status |
| `-title` | Extract page title |
| `-tech-detect` | Technology detection |
| `-server` | Show server header |
| `-websocket` | Detect WebSocket |
| `-cdn` | Detect CDN |
| `-ip` | Show IP address |
| `-ports` | Port scanning |
| `-vhost` | VHOST buster |
| `-csp` | CSP scanner |
| `-fuzz` | Fuzzing |
| `-json` | JSON output |
| `-o` | Output file |

## Examples

### Basic Probe
```bash
httpx -u https://target.com
```

### Technology Detection
```bash
httpx -u https://target.com -tech-detect -server
```

### With Status and Title
```bash
httpx -u https://target.com -status-code -title
```

### Multiple URLs
```bash
httpx -l urls.txt -status-code -title -tech-detect
```

### CDN Detection
```bash
httpx -u https://target.com -cdn
```

### WebSocket Detection
```bash
httpx -u https://target.com -websocket
```

### JSON Output
```bash
httpx -u https://target.com -json -o results.json
```

### SSL Info
```bash
httpx -u https://target.com -tls -tls-grab
```

## Pipelines

```bash
# Subfinder to HTTPx
subfinder -d target.com -silent | httpx -tech-detect

# Combine with nuclei
httpx -u https://target.com -tech-detect | nuclei -tags cve
```

## Tips

1. **Use -silent** - For cleaner output in scripts
2. **Combine tools** - Pipe from subfinder
3. **JSON format** - For parsing results
4. **Batch processing** - Use -l for multiple URLs
