# flaresolverr

## Overview
- Tool name: `flaresolverr`
- Enabled in config: `true`
- Executable: `flaresolverr-client`
- Summary: Bypass Cloudflare/WAF challenges and extract cookies for reuse with other tools

## Detailed Description
FlareSolverr is a browser-automation API service used in authorized security testing when normal HTTP clients are blocked by anti-bot challenge pages (Cloudflare, Akamai, etc.).

Its primary value in pentesting is **cookie extraction**: once FlareSolverr solves the challenge, the clearance cookies (`cf_clearance`, `__cf_bm`, etc.) and the accepted user-agent can be harvested and reused with other tools that would otherwise be blocked.

In CyberStrikeAI, `flaresolverr` is exposed as an API client wrapper that talks to:
- `http://127.0.0.1:8191/v1` (default)

## Cookie Extraction Workflow (WAF Bypass)

**Step 1 - Fetch cookies:**
```bash
flaresolverr --url https://target.example --cookies-only
```

Output:
```json
{
  "cookie_header": "cf_clearance=abc123; __cf_bm=xyz789",
  "user_agent": "Mozilla/5.0 ...",
  "cookies": [{"name": "cf_clearance", "value": "abc123", "domain": ".target.example", ...}]
}
```

**Step 2 - Reuse with other tools:**
```bash
# curl
curl -H "Cookie: cf_clearance=abc123; __cf_bm=xyz789" \
     -A "Mozilla/5.0 ..." \
     https://target.example/admin

# nuclei
nuclei -u https://target.example \
       -H "Cookie: cf_clearance=abc123; __cf_bm=xyz789" \
       -H "User-Agent: Mozilla/5.0 ..."

# ffuf
ffuf -u https://target.example/FUZZ \
     -H "Cookie: cf_clearance=abc123; __cf_bm=xyz789" \
     -H "User-Agent: Mozilla/5.0 ..." \
     -w wordlist.txt

# httpx
httpx -u https://target.example \
      -H "Cookie: cf_clearance=abc123; __cf_bm=xyz789" \
      -H "User-Agent: Mozilla/5.0 ..."
```

**Important:** Clearance cookies typically expire after 15-30 minutes. Re-fetch if tools start receiving 403/challenge responses again.

## Dependencies
- Docker (recommended runtime)
- Running FlareSolverr service container (`ghcr.io/flaresolverr/flaresolverr`)
- `flaresolverr-client` wrapper installed in `PATH`

## Parameters
### `cmd`
- Type: `string`
- Required: `false`
- Default: `request.get`
- Flag: `--cmd`
- Options: `request.get`, `request.post`, `sessions.create`, `sessions.destroy`

### `url`
- Type: `string`
- Required: `false` (required by `request.get` / `request.post`)
- Flag: `--url`

### `cookies_only`
- Type: `bool`
- Required: `false`
- Default: `false`
- Flag: `--cookies-only`
- Returns only cookies and user-agent instead of the full response. Use this to extract clearance cookies for other tools.

### `session_id`
- Type: `string`
- Required: `false`
- Flag: `--session-id`

### `max_timeout`
- Type: `integer`
- Required: `false`
- Default: `60000`
- Flag: `--max-timeout`

### `post_data`
- Type: `string`
- Required: `false`
- Flag: `--post-data`

### `proxy_url`
- Type: `string`
- Required: `false`
- Flag: `--proxy-url`

### `user_agent`
- Type: `string`
- Required: `false`
- Flag: `--user-agent`

### `headers_json`
- Type: `string`
- Required: `false`
- Flag: `--headers-json`

### `cookies_json`
- Type: `string`
- Required: `false`
- Flag: `--cookies-json`

### `download`
- Type: `bool`
- Required: `false`
- Default: `false`
- Flag: `--download`

### `endpoint`
- Type: `string`
- Required: `false`
- Default: `http://127.0.0.1:8191/v1`
- Flag: `--endpoint`

## Practical Examples
```bash
# Extract cookies only (for feeding to other tools)
flaresolverr --cmd request.get --url https://target.example --cookies-only

# Basic fetch behind challenge (full response)
flaresolverr --cmd request.get --url https://target.example

# Create session, then use it for sticky requests
flaresolverr --cmd sessions.create
flaresolverr --cmd request.get --url https://target.example --session-id <session_id>

# POST request through solved browser context
flaresolverr --cmd request.post \
  --url https://target.example/login \
  --post-data "username=test&password=test"
```

## Model Usage Guidance
- When a target returns 403 or a challenge page, use FlareSolverr with `--cookies-only` first to get clearance cookies
- Pass the `cookie_header` and `user_agent` from the output to all subsequent tool calls (curl, nuclei, ffuf, httpx, nikto, etc.)
- Store extracted cookies in memory (credential category) for reuse across the session
- Re-fetch cookies if tools start getting blocked again (cookies expire after ~15-30 min)
- Use sessions (sessions.create) for multi-step workflows that need sticky browser state

## References
- https://github.com/FlareSolverr/FlareSolverr
