---
name: ctf-malware
description: Provides malware analysis and network traffic techniques for CTF challenges. Use when analyzing obfuscated scripts, malicious packages, custom crypto protocols, C2 traffic, PE/.NET binaries, RC4/AES encrypted communications, YARA rules, shellcode analysis, memory forensics for malware (Volatility malfind, process injection detection), anti-analysis techniques (VM/sandbox detection, timing evasion, API hashing, process injection, environment checks), or extracting malware configurations and indicators of compromise.
license: MIT
compatibility: Requires filesystem-based agent (Claude Code or similar) with bash, Python 3, and internet access for tool installation.
allowed-tools: Bash Read Write Edit Glob Grep Task WebFetch WebSearch
metadata:
  user-invocable: "false"
---

# CTF Malware & Network Analysis

Quick reference for malware analysis CTF challenges. Each technique has a one-liner here; see supporting files for full details with code.

## Prerequisites

**Python packages (all platforms):**
```bash
pip install yara-python pefile capstone oletools unicorn pycryptodome \
  volatility3 dissect.cobaltstrike
```

**Linux (apt):**
```bash
apt install strace ltrace tshark binwalk binutils
```

**macOS (Homebrew):**
```bash
brew install wireshark binwalk binutils ghidra
```

**Manual install:**
- dnSpy — [GitHub](https://github.com/dnSpy/dnSpy), .NET decompiler (Windows)

## Additional Resources

- [scripts-and-obfuscation.md](scripts-and-obfuscation.md) - JavaScript deobfuscation, PowerShell analysis, eval/base64 decoding, junk code detection, hex payloads, Debian package analysis, dynamic analysis techniques (strace/ltrace, network monitoring, memory string extraction, automated sandbox execution), YARA rules for malware detection, shellcode analysis (Unicorn Engine, Capstone), memory forensics for malware (Volatility 3 malfind, process injection detection), anti-analysis techniques (VM detection, timing evasion, API hashing, process injection), trojanized plugin analysis with custom alphabet C2 decoding
- [c2-and-protocols.md](c2-and-protocols.md) - C2 traffic patterns, custom crypto protocols, RC4 WebSocket, DNS-based C2, network indicators, PCAP analysis, AES-CBC, encryption ID, Telegram bot recovery, Poison Ivy RAT Camellia decryption
- [pe-and-dotnet.md](pe-and-dotnet.md) - PE analysis (peframe, pe-sieve, pestudio), .NET analysis (dnSpy, AsmResolver), LimeRAT extraction, sandbox evasion, malware config extraction, PyInstaller+PyArmor

---

## When to Pivot

- If the sample is really just a normal crackme, packed challenge binary, or custom VM with no malware behavior, switch to `/ctf-reverse`.
- If the main job is network reconstruction, disk carving, or host artifact recovery, switch to `/ctf-forensics`.
- If the challenge turns into public attribution or infrastructure tracing, switch to `/ctf-osint`.

## Quick Start Commands

```bash
# Static analysis
file suspicious_file
strings -n 8 suspicious_file | head -50
xxd suspicious_file | head -20

# PE analysis
python3 -c "import pefile; pe=pefile.PE('mal.exe'); print(pe.dump_info())" | head
peframe mal.exe

# Dynamic analysis (sandboxed!)
strace -f -s 200 ./suspicious 2>&1 | head -100
ltrace ./suspicious 2>&1 | head -50

# Network indicators
strings suspicious_file | grep -E '[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}'
strings suspicious_file | grep -iE 'http|ftp|ws://'

# YARA scan
yara -r rules.yar suspicious_file
```

## Obfuscated Scripts

- Replace `eval`/`bash` with `echo` to print underlying code; extract base64/hex blobs and analyze with `file`. See [scripts-and-obfuscation.md](scripts-and-obfuscation.md).

## JavaScript & PowerShell Deobfuscation

- JS: Replace `eval` with `console.log`, decode `unescape()`, `atob()`, `String.fromCharCode()`.
- PowerShell: Decode `-enc` base64, replace `IEX` with output. See [scripts-and-obfuscation.md](scripts-and-obfuscation.md).

## Junk Code Detection

- NOP sleds, push/pop pairs, dead writes, unconditional jumps to next instruction. Filter to extract real `call` targets. See [scripts-and-obfuscation.md](scripts-and-obfuscation.md).

## PCAP & Network Analysis

```bash
tshark -r file.pcap -Y "tcp.stream eq X" -T fields -e tcp.payload
```

Look for C2 on unusual ports. Extract IPs/domains with `strings | grep`. See [c2-and-protocols.md](c2-and-protocols.md).

## Custom Crypto Protocols

- Stream ciphers share keystream state for both directions; concatenate ALL payloads chronologically.
- ChaCha20 keystream extraction: send nullbytes (0 XOR anything = anything). See [c2-and-protocols.md](c2-and-protocols.md).

## C2 Traffic Patterns

- Beaconing, DGA, DNS tunneling, HTTP(S) with custom headers, encoded payloads. See [c2-and-protocols.md](c2-and-protocols.md).

## RC4-Encrypted WebSocket C2

- Remap port with `tcprewrite`, add RSA key for TLS decryption, find RC4 key in binary. See [c2-and-protocols.md](c2-and-protocols.md).

## Identifying Encryption Algorithms

- AES: `0x637c777b` S-box; ChaCha20: `expand 32-byte k`; TEA/XTEA: `0x9E3779B9`; RC4: sequential S-box init. See [c2-and-protocols.md](c2-and-protocols.md).

## AES-CBC in Malware

- Key = MD5/SHA256 of hardcoded string; IV = first 16 bytes of ciphertext. See [c2-and-protocols.md](c2-and-protocols.md).

## PE Analysis

```bash
peframe malware.exe      # Quick triage
pe-sieve                 # Runtime analysis
pestudio                 # Static analysis (Windows)
```

See [pe-and-dotnet.md](pe-and-dotnet.md).

## .NET Malware Analysis

- Use dnSpy/ILSpy for decompilation; AsmResolver for programmatic analysis. LimeRAT C2: AES-256-ECB with MD5-derived key. See [pe-and-dotnet.md](pe-and-dotnet.md).

## Malware Configuration Extraction

- Check .data section, PE/.NET resources, registry keys, encrypted config files. See [pe-and-dotnet.md](pe-and-dotnet.md).

## Sandbox Evasion Checks

- VM detection, debugger detection, timing checks, environment checks, analysis tool detection. See [pe-and-dotnet.md](pe-and-dotnet.md).

## Anti-Analysis Techniques

VM detection (CPUID, MAC prefix, registry, disk size), timing evasion (sleep/RDTSC sandbox detection), API hashing (ROR13/DJB2/CRC32 + hashdb lookup), process injection (hollowing, APC, CreateRemoteThread), environment checks. See [scripts-and-obfuscation.md](scripts-and-obfuscation.md#anti-analysis-techniques).

## Trojanized Plugin Analysis

Diff malicious plugin against official release to find injected code in try/except blocks. Custom alphabet rotation (`C[(C.index(ch) - offset) % len(C)]`) decodes C2 domain, XOR decodes endpoint path. See [scripts-and-obfuscation.md](scripts-and-obfuscation.md#trojanized-plugin-analysis-with-custom-alphabet-c2-decoding-inshack-2018).

## PyInstaller + PyArmor Unpacking

- `pyinstxtractor.py` to extract, PyArmor-Unpacker for protected code. See [pe-and-dotnet.md](pe-and-dotnet.md).

## Telegram Bot Evidence Recovery

- Use bot token from malware source to call `getUpdates` and `getFile` APIs. See [c2-and-protocols.md](c2-and-protocols.md).

## Debian Package Analysis

```bash
ar -x package.deb && tar -xf control.tar.xz  # Check postinst scripts
```

See [scripts-and-obfuscation.md](scripts-and-obfuscation.md).

## YARA Rules for Malware Detection

Write YARA rules to match byte patterns, strings, and regex against files or memory dumps. Detect XOR loops (`{31 ?? 80 ?? ?? 4? 75}`), base64 blobs, encoded PowerShell. Use `yarac` to compile for faster scanning. See [scripts-and-obfuscation.md](scripts-and-obfuscation.md#yara-rules-for-malware-detection).

## Shellcode Analysis

Disassemble with `objdump -b binary -m i386:x86-64`, emulate with Unicorn Engine (hook syscalls safely), or use Capstone for programmatic disassembly. Look for XOR decoder stubs. See [scripts-and-obfuscation.md](scripts-and-obfuscation.md#shellcode-analysis).

## Memory Forensics for Malware

`vol3 windows.malfind` detects injected code (PAGE_EXECUTE_READWRITE without mapped file). `windows.pstree` reveals suspicious parent-child relationships. YARA scan memory with `yarascan.YaraScan`. See [scripts-and-obfuscation.md](scripts-and-obfuscation.md#memory-forensics-for-malware).

## Network Indicators Quick Reference

```bash
strings malware | grep -E '[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}'
tshark -r capture.pcap -Y "dns.qry.name" -T fields -e dns.qry.name | sort -u
```
