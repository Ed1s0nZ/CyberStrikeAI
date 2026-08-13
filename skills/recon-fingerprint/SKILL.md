---
name: recon-fingerprint
description: Web fingerprinting and WAF detection using wafw00f, whatweb, nuclei, and httpx. Use this skill when user needs to identify web technologies, detect WAF/CDN, analyze server headers, or fingerprint web applications and frameworks.
---

# Web Fingerprinting & WAF Detection

## Authorization Warning

**IMPORTANT:** Web fingerprinting sends requests to target servers. Always ensure you have:
- Written permission from the target application owner
- Defined scope of authorized testing
- Legal compliance with local regulations

## Prerequisites

Required tools that must be installed on your system:
- **httpx** - `go install -v github.com/projectdiscovery/httpx/cmd/httpx@latest`
- **nuclei** - `go install -v github.com/projectdiscovery/nuclei/v2/cmd/nuclei@latest`

Optional tools:
- **wafw00f** - `pip install wafw00f`
- **whatweb** - Package manager installation
- **fingerprintx** - `go install github.com/praetorian-inc/fingerprintx/cmd/fingerprintx@latest`

## Quick Start

Most commonly used commands for web fingerprinting:

### Basic Technology Detection
```bash
whatweb https://target.com
```

### WAF Detection
```bash
wafw00f https://target.com
```

### HTTP Header Analysis
```bash
curl -I https://target.com
```

### Comprehensive Fingerprinting (Nuclei)
```bash
nuclei -u https://target.com -tags tech
```

## Common Scenarios

### Scenario 1: Quick Technology Fingerprinting

When you need to quickly identify the technology stack:

```bash
whatweb https://target.com --aggression 1
```

**Parameters:**
- `--aggression 1` - Quick scan (1-4, default 1)
- `-a 3` - More aggressive (more requests)
- `-v` - Verbose output

**Example:**
```bash
whatweb https://target.com -a 3
```

---

### Scenario 2: WAF Detection

When you need to detect WAF/CDN protection:

```bash
wafw00f https://target.com
```

**Output shows:**
- WAF vendor (Cloudflare, AWS WAF, Imperva, etc.)
- CDN in use
- Firewall rules detected

**Check multiple targets:**
```bash
wafw00f -i targets.txt
```

---

### Scenario 3: Server Header Analysis

When you need to analyze HTTP headers:

```bash
curl -I https://target.com
```

**Detailed headers:**
```bash
curl -v https://target.com 2>&1 | grep -i "< "
```

**Common headers to check:**
```
Server: nginx/1.18.0
X-Powered-By: PHP/7.4
X-AspNet-Version: 4.0.30319
X-Frame-Options: SAMEORIGIN
X-Content-Type-Options: nosniff
Strict-Transport-Security: max-age=31536000
```

---

### Scenario 4: Technology Detection with Nuclei

When you need comprehensive technology fingerprinting:

```bash
nuclei -u https://target.com -tags tech -severity info
```

**Specific technologies:**
```bash
nuclei -u https://target.com -tags "wordpress,joomla,drupal"
nuclei -u https://target.com -tags "spring-boot,struts2"
nuclei -u https://target.com -tags "react,vue,angular"
```

---

### Scenario 5: HTTPx Fingerprinting

When you need fast HTTP probing with tech detection:

```bash
httpx -u https://target.com -tech-detect -status-code -title
```

**Parameters:**
- `-tech-detect` - Enable technology detection
- `-status-code` - Show HTTP status
- `-title` - Extract page title
- `-server` - Show server header
- `-websocket` - Detect WebSocket
- `-cdn` - Detect CDN

**Example:**
```bash
httpx -u https://target.com -tech-detect -server -cdn -ssl
```

---

### Scenario 6: CMS Detection

When you need to identify the CMS:

```bash
whatweb https://target.com --aggression 3 | grep -i cms
```

**Nuclei CMS detection:**
```bash
nuclei -u https://target.com -tags cms
```

**Common CMS indicators:**
- WordPress: `/wp-login.php`, `/wp-admin/`, `wp-json`
- Drupal: `/user/login`, `Drupal.settings`
- Joomla: `/administrator/components`, `Joomla!`
- TYPO3: `/typo3conf`

---

### Scenario 7: JavaScript Framework Detection

When you need to identify frontend frameworks:

```bash
curl -s https://target.com | grep -i "react\|vue\|angular\|jquery"
```

**Check specific framework files:**
```bash
# React
curl -s https://target.com | grep -i "react"

# Vue.js
curl -s https://target.com | grep -i "vue\.js\|vue-"

# Angular
curl -s https://target.com | grep -i "angular\|ng-app"

# jQuery
curl -s https://target.com | grep -i "jquery"
```

---

### Scenario 8: Server Version Detection

When you need to identify server software and version:

```bash
nmap -sV -p 443,80 target.com
```

**HTTP server banner:**
```bash
curl -I https://target.com | grep -i server
```

**Use httpx for server detection:**
```bash
httpx -u https://target.com -server -response-time
```

---

### Scenario 9: CDN Detection

When you need to identify CDN providers:

```bash
httpx -u https://target.com -cdn
```

**Check HTTP headers for CDN:**
```bash
curl -I https://target.com | grep -i "cf-ray\|x-amz\|x-akamai\|x-fastly"
```

**Common CDN headers:**
- Cloudflare: `cf-ray`, `cf-cache-status`
- AWS CloudFront: `x-amz-cf-id`
- Akamai: `x-akamai-transformed`
- Fastly: `x-served-by`, `fastly-ssl`
- CloudFront: `via`, `x-amz-cf-pop`

---

### Scenario 10: SSL/TLS Fingerprinting

When you need to analyze SSL configuration:

```bash
nmap --script ssl-cert,ssl-enum-ciphers -p 443 target.com
```

**SSL info with curl:**
```bash
curl -vI https://target.com 2>&1 | grep -i ssl
```

**Using testssl.sh:**
```bash
testssl.sh https://target.com
```

---

## Tool Selection Guide

| Scenario | Recommended Tool | Command |
|----------|------------------|---------|
| Quick tech detect | whatweb | `whatweb https://target.com` |
| WAF detection | wafw00f | `wafw00f https://target.com` |
| Header analysis | curl | `curl -I https://target.com` |
| Comprehensive | nuclei | `nuclei -u https://target.com -tags tech` |
| Fast probing | httpx | `httpx -u https://target.com -tech-detect` |
| CMS detection | nuclei | `nuclei -u https://target.com -tags cms` |
| CDN detection | httpx | `httpx -u https://target.com -cdn` |

**Tool Comparison:**

| Tool | Speed | Coverage | Best For |
|------|-------|----------|----------|
| **whatweb** | Fast | Good | Quick tech stack |
| **wafw00f** | Fast | WAF only | WAF detection |
| **nuclei** | Medium | Excellent | Comprehensive |
| **httpx** | Very Fast | Basic | Fast probing |
| **nmap** | Slow | Deep | SSL/Server details |

---

## Technology Fingerprints

### Web Servers

| Server | Header Pattern | Common Versions |
|--------|---------------|-----------------|
| nginx | `Server: nginx` | 1.18.x, 1.20.x, 1.22.x |
| Apache | `Server: Apache` | 2.4.x, 2.2.x |
| IIS | `Server: Microsoft-IIS` | 7.5, 8.0, 8.5, 10.0 |
| Cloudflare Server | `Server: cloudflare` | - |

### Backend Frameworks

| Framework | Indicators |
|-----------|------------|
| PHP | `X-Powered-By: PHP`, `.php` URLs |
| Python | `Server: WSGIServer`, Python headers |
| Ruby | `X-Powered-By: Phusion Passenger` |
| Node.js | `X-Powered-By: Express` |
| Java | `X-Powered-By: JSP`, `.jspx`, `.do` |
| .NET | `X-AspNet-Version`, `.aspx` |
| Go | `Server: Go-http-server` |

### Frontend Frameworks

| Framework | File/Pattern |
|-----------|---------------|
| React | `react.js`, `react-dom`, `_react`, `__REACT__` |
| Vue.js | `vue.js`, `vue-router`, `v-if`, `v-for` |
| Angular | `ng-app`, `angular.js`, `zone.js` |
| jQuery | `jquery.js`, `$(`, `.ajax()` |

### WAF Signatures

| WAF | Detection Method |
|-----|-----------------|
| Cloudflare | `cf-ray`, `cf-cache-status` headers |
| AWS WAF | `x-amz-cf-id` headers |
| Imperva | `X-Iinfo`, `X-CDN` headers |
| Akamai | `akamai-origin` headers |
| F5 ASM | `BIGipServer` cookies |
| ModSecurity | `Mod_Security` headers |
| Barracuda | `barra_counter_session` cookies |

---

## Tips and Best Practices

1. **Start passive** - Use headers and page content first
2. **Check multiple sources** - Combine tool outputs for accuracy
3. **Verify versions** - Technology detection is not always precise
4. **Note evasion** - Some sites hide their technology stack
5. **WAF first** - Always check for WAF before active scanning
6. **CDN consideration** - CDN may hide actual server info
7. **Save results** - Record fingerprints for correlation

---

## Resources

### Scripts

- `scripts/extract_headers.py` - Extract and analyze HTTP headers
- `scripts/tech_matcher.py` - Match technologies from responses
- `scripts/waf_detector.py` - Detect WAF from headers/cookies

### References

- `references/whatweb_guide.md` - WhatWeb reference guide
- `references/wafw00f_guide.md` - WAF detection guide
- `references/httpx_guide.md` - HTTPx reference
- `references/fingerprinting_techniques.md` - Advanced fingerprinting methods


---

### Scenario: Persistent Storage of Fingerprinting Results

When you need to persist web fingerprinting results to the database:

```bash
# Manual entry after fingerprinting
python .claude/skills/recon-fingerprint/scripts/fingerprint_storage.py \
  --host-ip 192.168.1.100 \
  --url "https://example.com" \
  --technology "Apache 2.4.41" \
  --category "web-server" \
  --version "2.4.41" \
  --subsystem "Web Application"
```

**Parameters:**
- `--host-ip` - Target host IP (required)
- `--url` - Target URL (required)
- `--technology` - Discovered technology (required)
- `--category` - Technology category: web-server, cms, framework, etc. (optional)
- `--version` - Technology version (optional)
- `--confidence` - Confidence level (optional)
- `--subsystem` - Subsystem name (optional)

**Database location:** `./data/results.db`

**Related skills:** `results-storage` - Query data, generate reports

---
### Assets

- `assets/waf-signatures.txt` - Known WAF signatures
- `assets/tech-headers.txt` - Technology header patterns
- `assets/cms-fingerprints.txt` - CMS detection patterns
