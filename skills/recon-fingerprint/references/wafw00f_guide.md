# WAFW00F Guide

Web Application Firewall (WAF) detection tool.

## Installation

```bash
pip install wafw00f
```

Or from source:
```bash
git clone https://github.com/EnableSecurity/wafw00f.git
cd wafw00f
python setup.py install
```

## Basic Usage

```bash
wafw00f https://target.com
```

## Common Options

| Option | Description |
|--------|-------------|
| `-a` | WAF detection via all checks |
| `-i` | Read targets from file |
| `-p` - | Proxy support |
| `-t` | Request type |
| `-r` | Follow redirects |
| `-o` - | Output to file |
| `-v` - | Verbose mode |

## Examples

### Basic Detection
```bash
wafw00f https://target.com
```

### All Checks
```bash
wafw00f -a https://target.com
```

### Multiple Targets
```bash
wafw00f -i targets.txt
```

### With Proxy
```bash
wafw00f -p http://127.0.0.1:8080 https://target.com
```

## Detection Methods

WAFW00F uses multiple detection methods:

1. **Checking headers** - Server, X-*, etc.
2. **Cookies** - Specific WAF cookies
3. **Response codes** - 403 patterns
4. **Challenge responses** - CAPTCHA pages
5. **Generic attacks** - SQLi, XSS triggers

## Supported WAFs

WAFW00F can detect:

| WAF | Vendor |
|-----|--------|
| Cloudflare | Cloudflare, Inc. |
| Akamai | Akamai Technologies |
| Imperva | Imperva Inc. |
| Fastly | Fastly, Inc. |
| AWS WAF | Amazon |
| Azure Front Door | Microsoft |
| Google CDN | Google |
| Sucuri | Sucuri Inc. |
| Incapsula | Imperva |
| ModSecurity | Trustwave |
| Barracuda | Barracuda Networks |
| F5 ASM | F5 Networks |
| Wordfence | Defiant |

## Output Interpretation

```
[*] Checking https://target.com
[+] Generic Detection: Cloudflare
[+] The site is behind Cloudflare
```

## Tips

1. **First tool to run** - Before active scanning
2. **Check subdomains** - Different subdomains may not be protected
3. **Combine with headers** - Manually verify with curl
4. **False positives** - Always verify findings
