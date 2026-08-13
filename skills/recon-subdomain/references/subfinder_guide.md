# Subfinder Guide

Fast passive subdomain enumeration tool.

## Installation

```bash
go install -v github.com/projectdiscovery/subfinder/v2/cmd/subfinder@latest
```

## Basic Usage

```bash
subfinder -d example.com
```

## Common Options

| Option | Description |
|--------|-------------|
| `-d` | Target domain |
| `-o` | Output file |
| `-silent` | Suppress stderr |
| `-v` | Verbose mode |
| `-all` | Use all sources |
| `-sources` | Specify sources |
| `-exclude-sources` | Exclude sources |
| `-json` | JSON output |
| `-config` | Config file location |

## Sources

Default sources include:
- **Search Engines**: Google, Bing, Yahoo, Baidu
- **Certificate Transparency**: crt.sh, censys
- **Data Sources**: Shodan, Censys, BufferOver
- **DNS Aggregators**: DNSDumpster, HackerTarget

**List all sources:**
```bash
subfinder -list-sources
```

## Examples

### Basic Discovery
```bash
subfinder -d target.com -o subs.txt
```

### With Specific Sources
```bash
# Only certificate sources
subfinder -d target.com -sources crtsh,censys -o subs.txt

# Exclude slow sources
subfinder -d target.com -exclude-sources archive -o subs.txt
```

### JSON Output
```bash
subfinder -d target.com -json -o subs.json
```

### Silent Mode
```bash
subfinder -d target.com -silent | tee subs.txt
```

## Configuration

Create `~/.config/subfinder/config.yaml`:

```yaml
# API Keys (optional for more results)
censys:
  - key1
  - key2

shodan:
  - key1

github:
  - token1
```

## Tips

1. **Use silent mode** when piping to other tools
2. **Combine sources** for maximum coverage
3. **JSON output** useful for parsing and automation
4. **Rate limiting** is handled automatically
