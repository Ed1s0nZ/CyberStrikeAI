# WhatWeb Guide

Next-generation web scanner that identifies technology.

## Installation

```bash
# Debian/Ubuntu
sudo apt install whatweb

# macOS
brew install whatweb

# From source
git clone https://github.com/urbanadventurer/WhatWeb.git
cd WhatWeb
sudo gem install whatweb
```

## Basic Usage

```bash
whatweb https://target.com
```

## Common Options

| Option | Description |
|--------|-------------|
| `-a` | Aggression level (1-4, default 1) |
| `-v` | Verbose mode |
| `--log-verbose` | Extra verbose |
| `--color` | Colored output |
| `--no-errors` | Suppress errors |
| `-r` | Redirect pages |
| `--max-redirects` | Max redirects |
| `-h` | Headers |

## Aggression Levels

| Level | Requests | Description |
|-------|----------|-------------|
| 1 | 1 | Single request, passive analysis |
| 2 | ~5 | Some active requests |
| 3 | ~20 | More aggressive |
| 4 | 100+ | Very aggressive (may trigger WAF) |

## Examples

### Quick Scan
```bash
whatweb https://target.com
```

### More Aggressive
```bash
whatweb -a 3 https://target.com
```

### Verbose Output
```bash
whatweb -v https://target.com
```

### Multiple Targets
```bash
whatweb https://target1.com https://target2.com
```

### From File
```bash
whatweb -i targets.txt
```

### Output to File
```bash
whatweb https://target.com --log-json output.json
```

## Plugin System

List plugins:
```bash
whatweb --list-plugins
```

Use specific plugins:
```bash
whatweb --plugins WordPress,PHP https://target.com
```

Exclude plugins:
```bash
whatweb --exclude-plugins IIS,Apache https://target.com
```

## Output Formats

### JSON
```bash
whatweb --log-json https://target.com
```

### XML
```bash
whatweb --log-xml https://target.com
```

### SQLite
```bash
whatweb --log-sqlite output.db https://target.com
```

## Tips

1. **Start with level 1** - Passive only
2. **Increase gradually** - Use higher levels if needed
3. **Use verbose** - See what's being detected
4. **Combine with other tools** - Use with wafw00f for full picture
