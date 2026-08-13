# Scripts and Obfuscation Analysis

## Table of Contents

- [Obfuscated Scripts (General)](#obfuscated-scripts-general)
- [JavaScript Deobfuscation](#javascript-deobfuscation)
- [PowerShell Analysis](#powershell-analysis)
- [Junk Code Detection](#junk-code-detection)
- [Hex-Encoded Payloads](#hex-encoded-payloads)
- [Debian Package Analysis](#debian-package-analysis)
- [Dynamic Analysis Techniques](#dynamic-analysis-techniques)
- [YARA Rules for Malware Detection](#yara-rules-for-malware-detection)
- [Shellcode Analysis](#shellcode-analysis)
- [Memory Forensics for Malware](#memory-forensics-for-malware)
- [Anti-Analysis Techniques](#anti-analysis-techniques)
  - [VM / Sandbox Detection](#vm--sandbox-detection)
  - [Timing-Based Evasion](#timing-based-evasion)
  - [API Hashing](#api-hashing)
  - [Process Injection Techniques](#process-injection-techniques)
  - [Environment Variable / Hostname Checks](#environment-variable--hostname-checks)

---

## Obfuscated Scripts (General)

- Replace `eval`/`bash` with `echo` to print underlying code
- Extract base64/hex blobs and analyze with `file`
- Common deobfuscation chain: base64 decode -> gzip decode -> reverse -> base64 decode

## JavaScript Deobfuscation

```javascript
// Replace eval with console.log
eval = console.log;
// Then run the obfuscated code

// Common patterns
unescape()           // URL decoding
String.fromCharCode() // Char codes
atob()               // Base64
```

## PowerShell Analysis

```powershell
# Common obfuscation
-enc / -EncodedCommand  # Base64 encoded
IEX / Invoke-Expression # Eval equivalent
[System.Text.Encoding]::Unicode.GetString([System.Convert]::FromBase64String($encoded))
```

## Junk Code Detection

**Pattern:** Obfuscation adds meaningless instructions around real code

**Identification:**
- NOP sleds, push/pop pairs that cancel
- Arithmetic that results in zero/identity
- Dead writes (register written but never read before next write)
- Unconditional jumps to next instruction

**Filtering technique:**
```python
# Identify real calls by looking for patterns
# junk, junk, junk, CALL target, junk, junk
# Extract call targets, ignore surrounding noise

def extract_real_calls(disassembly):
    calls = []
    for instr in disassembly:
        if instr.mnemonic == 'call' and not is_junk_target(instr.operand):
            calls.append(instr)
    return calls
```

## Hex-Encoded Payloads

- Convert hex to bytes, try common transformations: subtract 1, XOR with key

## Debian Package Analysis

```bash
ar -x package.deb           # Unpack debian package
tar -xf control.tar.xz      # Check control files
# Look for postinst scripts that execute payloads
```

---

## Dynamic Analysis Techniques

```bash
# Behavioral monitoring with strace/ltrace
strace -f -e trace=network,file -o trace.log ./malware
ltrace -f -o ltrace.log ./malware

# Network monitoring during execution
# Terminal 1: capture traffic
sudo tcpdump -i any -w malware_traffic.pcap &
# Terminal 2: DNS monitoring
sudo tcpdump -i any port 53 -l | tee dns_queries.log &
# Terminal 3: run sample
timeout 60 ./malware

# File system monitoring (Linux)
inotifywait -m -r /tmp /var/tmp --format '%T %w%f %e' --timefmt '%H:%M:%S' &
./malware

# Process monitoring
watch -n 1 'ps aux | grep -v grep | grep malware'

# Memory string extraction during runtime
# Run malware, then dump strings from its memory
pid=$(pgrep malware)
strings /proc/$pid/maps
cat /proc/$pid/mem 2>/dev/null | strings | grep -i flag
# Or use gdb: gdb -p $pid -batch -ex 'dump memory dump.bin 0x400000 0x500000'
```

```python
# Automated sandbox execution with timeout
import subprocess, os, tempfile

def run_sample(path, timeout=30):
    """Run malware sample with monitoring"""
    with tempfile.NamedTemporaryFile(suffix='.pcap', delete=False) as pcap:
        # Start packet capture
        tcpdump = subprocess.Popen(
            ['sudo', 'tcpdump', '-i', 'any', '-w', pcap.name],
            stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)

        try:
            # Run with strace
            result = subprocess.run(
                ['strace', '-f', '-e', 'trace=network,file', path],
                capture_output=True, text=True, timeout=timeout)
            print("STDOUT:", result.stdout[:500])
            print("STDERR (syscalls):", result.stderr[:2000])
        except subprocess.TimeoutExpired:
            print(f"Sample ran for {timeout}s (killed)")
        finally:
            tcpdump.terminate()
            print(f"PCAP saved: {pcap.name}")
```

**Key insight:** Dynamic analysis reveals runtime behavior that static analysis misses: actual C2 domains resolved, encryption keys in memory, dropped files, and anti-analysis checks that were bypassed. Always run in an isolated environment (VM snapshot, Docker container) and monitor network, filesystem, and process activity simultaneously.

---

### YARA Rules for Malware Detection

```bash
# Basic YARA rule structure
cat > detect_malware.yar << 'EOF'
rule SuspiciousStrings {
    meta:
        description = "Detect common malware indicators"
    strings:
        $s1 = "cmd.exe /c" nocase
        $s2 = "powershell -enc" nocase
        $s3 = {4D 5A 90 00}  // MZ header (hex pattern)
        $s4 = /https?:\/\/[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}/  // IP-based URL
        $xor_loop = {31 ?? 80 ?? ?? 4? 75}  // XOR decode loop pattern
    condition:
        2 of ($s*) or $xor_loop
}
EOF

# Scan files
yara detect_malware.yar suspicious_file.exe
yara -r detect_malware.yar /path/to/directory/  # Recursive scan

# Scan memory dump
yara detect_malware.yar memory.dmp
```

**Common YARA patterns for CTFs:**
```yara
rule Base64_PowerShell {
    strings:
        $enc = "powershell" nocase
        $b64 = /[A-Za-z0-9+\/]{50,}={0,2}/
    condition:
        $enc and $b64
}

rule XOR_Encrypted_PE {
    strings:
        $mz = {4D 5A}
    condition:
        not $mz at 0 and filesize < 1MB
        // PE without MZ header = likely XOR encrypted
}
```

**Key insight:** YARA rules match byte patterns, strings, and regex against files or memory. In CTFs, write rules to detect specific obfuscation patterns (XOR loops, base64 blobs, encoded PowerShell), then apply to memory dumps or malware samples. Use `yarac` to compile rules for faster scanning.

---

### Shellcode Analysis

```bash
# Extract shellcode from binary
objdump -d shellcode.bin -b binary -m i386:x86-64 -M intel

# Emulate shellcode with unicorn engine
python3 << 'PYEOF'
from unicorn import *
from unicorn.x86_const import *

shellcode = open('shellcode.bin', 'rb').read()
mu = Uc(UC_ARCH_X86, UC_MODE_64)
BASE = 0x400000
STACK = 0x7fff0000

mu.mem_map(BASE, 0x1000)
mu.mem_map(STACK - 0x1000, 0x2000)
mu.mem_write(BASE, shellcode)
mu.reg_write(UC_X86_REG_RSP, STACK)

# Hook syscalls to trace behavior
def hook_syscall(mu, user_data):
    rax = mu.reg_read(UC_X86_REG_RAX)
    print(f"syscall: {rax}")

mu.hook_add(UC_HOOK_INSN, hook_syscall, None, 1, 0, UC_X86_INS_SYSCALL)
mu.emu_start(BASE, BASE + len(shellcode))
PYEOF

# Disassemble with capstone
python3 -c "
from capstone import *
md = Cs(CS_ARCH_X86, CS_MODE_64)
code = open('shellcode.bin','rb').read()
for i in md.disasm(code, 0x0):
    print(f'{i.address:#x}: {i.mnemonic} {i.op_str}')
"

# Quick analysis with scdbg (Windows shellcode emulator)
scdbg /f shellcode.bin
```

**Key insight:** Shellcode in CTF malware challenges is often XOR-encoded or staged. Look for decoder stubs (short loops with XOR), then extract and decode the payload. Unicorn Engine emulation is safer than running shellcode — it intercepts syscalls without executing them.

---

### Memory Forensics for Malware

```bash
# Volatility 3 — analyze memory dump for malware indicators
# List processes (look for suspicious names, unusual parents)
vol3 -f memory.dmp windows.pslist
vol3 -f memory.dmp windows.pstree

# Detect hidden/unlinked processes
vol3 -f memory.dmp windows.psscan

# Dump suspicious process memory
vol3 -f memory.dmp windows.memmap --pid PID --dump

# Extract injected code (process hollowing, DLL injection)
vol3 -f memory.dmp windows.malfind

# Network connections from malware
vol3 -f memory.dmp windows.netscan

# Command-line arguments (reveals malware parameters)
vol3 -f memory.dmp windows.cmdline

# DLL list per process (detect injected DLLs)
vol3 -f memory.dmp windows.dlllist --pid PID

# YARA scan on memory dump
vol3 -f memory.dmp yarascan.YaraScan --yara-rules "rule test { strings: $s = \"flag{\" condition: $s }"
```

**Key insight:** `windows.malfind` detects injected code by finding memory regions with `PAGE_EXECUTE_READWRITE` protection and no corresponding mapped file — the hallmark of process injection. Combine with `windows.pstree` to find processes with unexpected parent-child relationships (e.g., `svchost.exe` spawned by `cmd.exe`).

---

## Anti-Analysis Techniques

Malware uses runtime checks to detect analysis environments and alter behavior. Bypass these to reach the actual malicious functionality. For comprehensive anti-debug, anti-VM/sandbox, and anti-DBI bypass strategies, see [ctf-reverse/anti-analysis.md](../ctf-reverse/anti-analysis.md).

### VM / Sandbox Detection

**Pattern:** Malware checks for virtualization artifacts before executing payload. In CTFs, the "real" flag logic is behind these checks.

**Key insight:** Identify the detection method, then patch the check or fake the environment.

```python
# Common VM detection checks and bypasses:

# 1. CPUID check (hypervisor bit 31 of ECX after CPUID leaf 1)
# Bypass: patch JNZ after CPUID to JMP, or run in bare metal
# In GDB: set $ecx = $ecx & ~(1<<31)

# 2. MAC address prefix (VMware: 00:0C:29, 00:50:56; VBox: 08:00:27)
# Bypass: change VM NIC MAC to real hardware prefix

# 3. Registry keys (Windows)
# HKLM\SOFTWARE\VMware, Inc.\VMware Tools
# HKLM\SYSTEM\CurrentControlSet\Services\VBoxGuest
# Bypass: delete keys or patch registry check

# 4. File/process checks
VM_ARTIFACTS = [
    'vmtoolsd.exe', 'vmwaretray.exe', 'VBoxService.exe',
    'qemu-ga.exe', 'sandboxie', 'wireshark.exe',
    '/sys/class/dmi/id/product_name',  # "VMware Virtual Platform"
    'C:\\windows\\system32\\drivers\\vmmouse.sys',
]

# 5. Disk size check (VMs often have small disks)
# if total_disk < 60GB: exit()
# Bypass: expand VM disk or patch comparison

# 6. CPU count / RAM check
# if cpu_count < 2 or ram < 2GB: exit()
# Bypass: allocate more resources to VM
```

### Timing-Based Evasion

**Pattern:** Malware uses `sleep()`, `GetTickCount()`, or RDTSC to detect accelerated execution in sandboxes.

```python
# Detection: large sleep followed by time check
# import time
# start = time.time()
# time.sleep(300)  # 5 minutes
# if time.time() - start < 290: sys.exit()  # Sandbox fast-forwarded sleep

# Bypass approaches:
# 1. Patch sleep to NOP: elf.asm(elf.symbols['sleep'], 'ret')
# 2. Hook GetTickCount/time() to return expected values
# 3. In GDB: set breakpoint after sleep, manually advance
# 4. Binary patching: change sleep(300) to sleep(0)
```

**Key insight:** Look for calls to `sleep`, `time.sleep`, `NtDelayExecution`, `GetTickCount64`, `QueryPerformanceCounter`. If the sample just sits there doing nothing, it's likely in a sleep-based anti-sandbox check.

### API Hashing

**Pattern:** Instead of importing functions by name (visible in strings/imports), malware resolves API addresses at runtime by hashing function names and comparing to hardcoded hash values.

```python
# Common hash algorithms for API resolution:
# ROR13 (rotate-right 13) — most common, used by Metasploit
def ror13_hash(name):
    h = 0
    for c in name:
        h = ((h >> 13) | (h << 19)) & 0xFFFFFFFF
        h = (h + ord(c)) & 0xFFFFFFFF
    return h

# DJB2 hash
def djb2_hash(name):
    h = 5381
    for c in name:
        h = ((h * 33) + ord(c)) & 0xFFFFFFFF
    return h

# CRC32-based
import binascii
def crc32_hash(name):
    return binascii.crc32(name.encode()) & 0xFFFFFFFF

# Reversing: build lookup table from Windows API names
# hashdb.openanalysis.net — online API hash lookup
# ShellcodeHasher — matches hashes against known Windows APIs

# In Ghidra: find the hash comparison constant, look up in hashdb
# Pattern: loop over PEB→Ldr→InMemoryOrderModuleList, hash each export name
```

**Key insight:** When strings output shows almost no readable API names but the binary clearly does complex operations, suspect API hashing. Look for the hash function (small loop with XOR/rotate/add), then use hashdb or build a rainbow table against `kernel32.dll` and `ntdll.dll` exports.

### Process Injection Techniques

**Pattern:** Malware injects code into legitimate processes to evade detection. Understanding the injection method helps extract the actual payload.

```bash
# Classic injection chain:
# 1. OpenProcess(target_pid)
# 2. VirtualAllocEx(remote, ..., PAGE_EXECUTE_READWRITE)
# 3. WriteProcessMemory(remote, shellcode)
# 4. CreateRemoteThread(remote, shellcode_addr)

# Process hollowing:
# 1. CreateProcess(legitimate.exe, CREATE_SUSPENDED)
# 2. NtUnmapViewOfSection(hollow out the image)
# 3. VirtualAllocEx + WriteProcessMemory (write malicious PE)
# 4. SetThreadContext (point EIP/RIP to new entry)
# 5. ResumeThread

# Detection in memory dumps:
vol3 -f memory.dmp windows.malfind  # PAGE_EXECUTE_READWRITE without file backing
vol3 -f memory.dmp windows.hollowfind  # Hollowed processes (VAD vs PEB mismatch)

# APC injection (no new thread):
# QueueUserAPC(shellcode_addr, target_thread, ...)
# Thread executes shellcode on next alertable wait

# For CTF: dump the injected code region and analyze separately
vol3 -f memory.dmp windows.malfind --dump --pid <PID>
```

### Environment Variable / Hostname Checks

**Pattern:** Malware checks for specific environment conditions (hostname, username, domain, locale) to target specific victims or avoid analysis labs.

```python
# Common checks:
# - Hostname matches target: if socket.gethostname() != 'TARGET-PC': exit()
# - Username: if os.getlogin() in ['admin', 'sandbox', 'malware']: exit()
# - Domain membership: if 'WORKGROUP' in os.environ.get('USERDOMAIN', ''): exit()
# - Locale/language: WinAPI GetUserDefaultLangID()
# - Specific file must exist: if not os.path.exists('C:\\Users\\victim\\document.xlsx'): exit()

# Bypass: set environment variables before running
# export COMPUTERNAME=TARGET-PC
# Or patch the comparison in the binary
```

**Key insight:** If a malware sample exits immediately or behaves differently than expected, trace its API calls with `strace`/`ltrace` or step through with a debugger. Look for string comparisons against environment values early in execution.

---

### Trojanized Plugin Analysis with Custom Alphabet C2 Decoding (INShAck 2018)

**Pattern:** Diff a malicious Sublime Text plugin against the official release to find injected base64 payload in try/except block. The payload uses Caesar-cipher-like rotation on a custom alphabet to encode the C2 domain, and XOR with 0x42 for the endpoint path.

```python
# Custom alphabet rotation for C2 domain decoding
C = '0123456789abcdefghijklmnopqrstuvwxyz-.'
encoded_domain = "..."
domain = ''.join(C[(C.index(ch) - 0x0d) % len(C)] for ch in encoded_domain)

# XOR decoding for endpoint path
encoded_path = [44, 45, 54, 43, 36, 59]
path = ''.join(chr(b ^ 0x42) for b in encoded_path)

# Registration + flag retrieval
uuid = register_with_c2(domain)
flag = requests.get(f"http://{domain}/{path}", cookies={"uuid": uuid}).text
```

**Key insight:** Trojanized plugins inject code in exception handlers (try/except blocks visible in diff). Custom alphabets for C2 encoding use modular rotation instead of standard ciphers. Always diff suspicious packages against known-good releases from the official repository to isolate injected code.
