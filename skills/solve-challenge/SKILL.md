---
name: solve-challenge
description: Solves CTF challenges by performing first-pass triage, identifying the dominant category, and routing execution to the right specialized ctf-* skill. Use when the user gives you a challenge bundle, a remote service, a suspicious file, or only a vague challenge description and you must determine where to start. Do not use it when the category is already clear and a specialized skill can be invoked directly; this is the dispatcher and recon entrypoint, not the deepest reference for category-specific techniques.
license: MIT
compatibility: Requires filesystem-based agent (Claude Code or similar) with bash, Python 3, and internet access. Orchestrates other ctf-* skills.
allowed-tools: Bash Read Write Edit Glob Grep Task WebFetch WebSearch Skill
metadata:
  user-invocable: "true"
  argument-hint: "[category] [challenge-file-or-url]"
---

# CTF Challenge Solver

You're a skilled CTF player. Your goal is to solve the challenge and find the flag.

## Environment Setup

Two setup strategies depending on your workflow:

### Pre-install (recommended before competitions)

Use the central installer entrypoint:

```bash
bash scripts/install_ctf_tools.sh all
```

Run a narrower mode when you only want one tool group:

```bash
bash scripts/install_ctf_tools.sh python
bash scripts/install_ctf_tools.sh apt
bash scripts/install_ctf_tools.sh brew
bash scripts/install_ctf_tools.sh gems
bash scripts/install_ctf_tools.sh go
bash scripts/install_ctf_tools.sh manual
```

The full package lists now live in [scripts/install_ctf_tools.sh](../scripts/install_ctf_tools.sh).

### On-demand (during challenges)

Each category skill's `SKILL.md` has a **Prerequisites** section listing only the tools needed for that category. Install as you go.

## Workflow

### Step 0: CTFd Platform Detection

If the CTF platform URL is known, check if it runs CTFd and switch to API-driven navigation:

```bash
# Detect CTFd (look for /api/v1/ and /themes/core/)
curl -s "$CTF_URL/api/v1/" | head -5
curl -s "$CTF_URL" | grep -oE '/themes/core/'
```

If CTFd is detected, **ask the user for their API token** (generated from CTFd Settings > Access Tokens). The token is not provided by default — the user must create one in the CTFd web UI first. Once provided, set the environment variables and proceed via API:

```bash
export CTF_URL="https://ctf.example.com"
export CTF_TOKEN="ctfd_..."  # Ask user for this
```

Invoke `/ctf-misc` and load its `ctfd-navigation.md` for the full API reference and Python client class.

### Step 1: Recon

1. **Explore files** -- List the challenge directory, run `file *` on everything
2. **Triage binaries** -- `strings`, `xxd | head`, `binwalk`, `checksec` on binaries
3. **Fetch links** -- If the challenge mentions URLs, fetch them FIRST for context
4. **Connect** -- Try remote services (`nc`) to understand what they expect
5. **Read hints** -- Challenge descriptions, filenames, and comments often contain clues

### Step 2: Categorize

Determine the primary category, then invoke the matching skill.

**By file type:**
- `.pcap`, `.pcapng`, `.evtx`, `.raw`, `.dd`, `.E01` -> forensics
- `.elf`, `.exe`, `.so`, `.dll`, binary with no extension -> reverse or pwn (check if remote service provided -- if yes, likely pwn)
- `.py`, `.sage`, `.txt` with numbers -> crypto
- `.apk`, `.wasm`, `.pyc` -> reverse
- Web URL or source code with HTML/JS/PHP/templates -> web
- Images, audio, PDFs with no obvious content -> forensics (steganography)

**By challenge description keywords:**
- "buffer overflow", "ROP", "shellcode", "libc", "heap" -> pwn
- "RSA", "AES", "cipher", "encrypt", "prime", "modulus", "lattice", "LWE", "GCM" -> crypto
- "XSS", "SQL", "injection", "cookie", "JWT", "SSRF" -> web
- "disk image", "memory dump", "packet capture", "registry", "power trace", "side-channel", "spectrogram", "audio tracks", "MKV" -> forensics
- "find", "locate", "identify", "who", "where" -> osint
- "obfuscated", "packed", "C2", "malware", "beacon" -> malware
- "jail", "sandbox", "escape", "encoding", "signal", "game", "Nim", "commitment", "Gray code" -> misc

**By service behavior:**
- Port with interactive prompt, crash on long input -> pwn
- HTTP service -> web
- netcat with math/crypto puzzles -> crypto
- netcat with restricted shell or eval -> misc (jail)

### Step 3: Invoke the Category Skill

Once you identify the category, **invoke the matching skill** to get specialized techniques:

| Category | Invoke | When to Use |
|----------|--------|-------------|
| Web | `/ctf-web` | XSS, SQLi, SSTI, SSRF, JWT, file uploads, prototype pollution |
| Pwn | `/ctf-pwn` | Buffer overflow, format string, heap, ROP, sandbox escape |
| Crypto | `/ctf-crypto` | RSA, AES, ECC, PRNG, ZKP, classical ciphers |
| Reverse | `/ctf-reverse` | Binary analysis, game clients, VMs, obfuscated code |
| Forensics | `/ctf-forensics` | Disk images, memory dumps, event logs, stego, network captures |
| OSINT | `/ctf-osint` | Social media, geolocation, DNS, public records |
| Malware | `/ctf-malware` | Obfuscated scripts, C2 traffic, PE/.NET analysis |
| Misc | `/ctf-misc` | Jails, encodings, RF/SDR, esoteric languages, constraint solving |

You can also invoke `/ctf-<category>` to load the full skill instructions with detailed techniques.

### Step 4: Pivot When Stuck

If your first approach doesn't work:

1. **Re-examine assumptions** -- Is this really the category you think? A "web" challenge might need crypto for JWT forgery. A "forensics" PCAP might contain a pwn exploit to replay.
2. **Try a different category skill** -- Many challenges span multiple categories. Invoke a second skill for the cross-cutting technique.
3. **Look for what you missed** -- Hidden files, alternate ports, response headers, comments in source, metadata in images.
4. **Simplify** -- If an exploit is too complex, check if there's a simpler path (default creds, known CVE, logic bug).
5. **Check edge cases** -- Off-by-one, race conditions, integer overflow, encoding mismatches.

**Common multi-category patterns:**
- Forensics + Crypto: encrypted data in PCAP/disk image, need crypto to decrypt
- Web + Reverse: WASM or obfuscated JS in web challenge
- Web + Crypto: JWT forgery, custom MAC/signature schemes
- Reverse + Pwn: reverse the binary first, then exploit the vulnerability
- Forensics + OSINT: recover data from dump, then trace it via public sources
- Misc + Crypto: jail escape requires building crypto primitives under constraints
- OSINT + Stego: social media posts with unicode homoglyph steganography (Cyrillic lookalikes encode bits)
- Web + Forensics: paywall bypass (curl reveals content hidden by CSS overlays)
- Misc + Crypto + Game Theory: multi-phase interactive challenges with AES decryption → HMAC commitment → combinatorial game solving (GF(256) Nim)
- Crypto + Geometry + Lattice: multi-layer challenges progressing from spatial reconstruction → subspace recovery → LWE solving → AES-GCM decryption
- Forensics + Signal Processing: power traces / side-channel analysis requiring statistical analysis of measurement data
- Forensics + Network + Encoding: timing-based encoding in PCAP (inter-packet intervals encode binary data)

### Step 5: Generate Write-up

After solving the challenge, invoke `/ctf-writeup` to generate a standardized submission-style writeup — concise, reproducible, and ready for competition organizers or teammates to validate.

## Flag Formats

Flags vary by CTF. Common formats:
- `flag{...}`, `FLAG{...}`, `CTF{...}`, `TEAM{...}`
- Custom prefixes: check the challenge description or CTF rules for the format (e.g., `ENO{...}`, `HTB{...}`, `picoCTF{...}`)
- Sometimes just a plaintext string with no wrapper

**Validation rule (important):**
- If you find multiple flag-like strings, treat them as candidates and validate before finalizing.
- Prefer the token tied to the intended artifact/workflow (not random metadata noise or obvious decoys).
- Do a corpus-wide uniqueness check and include the source file/path when reporting.

```bash
# Search for common flag patterns in files
grep -rniE '(flag|ctf|eno|htb|pico)\{' .
# Search in binary/memory output
strings output.bin | grep -iE '\{.*\}'
```

## Quick Reference

```bash
# Recon
file *                                    # Identify file types
strings binary | grep -i flag             # Quick string search
xxd binary | head -20                     # Hex dump header
binwalk -e firmware.bin                   # Extract embedded files
checksec --file=binary                    # Check binary protections

# Connect
nc host port                              # Connect to challenge
echo -e "answer1\nanswer2" | nc host port # Scripted input
curl -v http://host:port/                 # HTTP recon

# Python exploit template
python3 -c "
from pwn import *
r = remote('host', port)
r.interactive()
"
```

## Challenge

$ARGUMENTS
