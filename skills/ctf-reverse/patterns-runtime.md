# CTF Reverse - Runtime Patching and Oracle Techniques

Malware unpacking, multi-stage shellcode, timing/signal side channels, and CTF-specific oracle attacks that rely on runtime state rather than static pattern matching.

For static reversing patterns (custom VMs, anti-debug, self-modifying code, LLVM obfuscation, S-box generation, SECCOMP/BPF, memory dumps, x86-64 gotchas, byte-wise transforms), see [patterns.md](patterns.md).

## Table of Contents
- [Malware Anti-Analysis Bypass via Patching](#malware-anti-analysis-bypass-via-patching)
- [Multi-Stage Shellcode Loaders](#multi-stage-shellcode-loaders)
- [Timing Side-Channel Attack](#timing-side-channel-attack)
- [Multi-Thread Anti-Debug with Decoy + Signal Handler Mixed Boolean-Arithmetic (ApoorvCTF 2026)](#multi-thread-anti-debug-with-decoy--signal-handler-mixed-boolean-arithmetic-apoorvctf-2026)
- [INT3 Patch + Coredump Brute-Force Oracle (Pwn2Win 2016)](#int3-patch--coredump-brute-force-oracle-pwn2win-2016)
- [Signal Handler Chain + LD_PRELOAD Oracle (Nuit du Hack 2016)](#signal-handler-chain--ld_preload-oracle-nuit-du-hack-2016)
- [printf Format String VM Decompilation to Z3 (SECCON 2017)](#printf-format-string-vm-decompilation-to-z3-seccon-2017)
- [Quadtree Recursive Image Format Parser (Google CTF Quals 2018)](#quadtree-recursive-image-format-parser-google-ctf-quals-2018)

---

## Malware Anti-Analysis Bypass via Patching

**Pattern (Carrot):** Malware with multiple environment checks before executing payload.

**Common checks to patch:**
| Check | Technique | Patch |
|-------|-----------|-------|
| `ptrace(PTRACE_TRACEME)` | Anti-debug | Change `cmp -1` to `cmp 0` |
| `sleep(150)` | Anti-sandbox timing | Change sleep value to 1 |
| `/proc/cpuinfo` "hypervisor" | Anti-VM | Flip `JNZ` to `JZ` |
| "VMware"/"VirtualBox" strings | Anti-VM | Flip `JNZ` to `JZ` |
| `getpwuid` username check | Environment | Flip comparison |
| `LD_PRELOAD` check | Anti-hook | Skip check |
| Fan count / hardware check | Anti-VM | Flip `JLE` to `JGE` |
| Hostname check | Environment | Flip `JNZ` to `JZ` |

**Ghidra patching workflow:**
1. Find check function, identify the conditional jump
2. Click on instruction → `Ctrl+Shift+G` → modify opcode
3. For `JNZ` (0x75) → `JZ` (0x74), or vice versa
4. For immediate values: change operand bytes directly
5. Export: press `O` → choose "Original File" format
6. `chmod +x` the patched binary

**Server-side validation bypass:**
- If patched binary sends system info to remote server, patch the data too
- Modify string addresses in data-gathering functions
- Change format strings to embed correct values directly

---

## Multi-Stage Shellcode Loaders

**Pattern (I Heard You Liked Loaders):** Nested shellcode with XOR decode loops and anti-debug.

**Debugging workflow:**
1. Break at `call rax` in launcher, step into shellcode
2. Bypass ptrace anti-debug: step to syscall, `set $rax=0`
3. Step through XOR decode loop (or break on `int3` if hidden)
4. Repeat for each stage until final payload

**Flag extraction from `mov` instructions:**
```python
# Final stage loads flag 4 bytes at a time via mov ebx, value
# Extract little-endian 4-byte chunks
values = [0x6174654d, 0x7b465443, ...]  # From disassembly
flag = b''.join(v.to_bytes(4, 'little') for v in values)
```

---

## Timing Side-Channel Attack

**Pattern (Clock Out):** Validation time varies per correct character (longer sleep on match).

**Exploitation:**
```python
import time
from pwn import *

flag = ""
for pos in range(flag_length):
    best_char, best_time = '', 0
    for c in string.printable:
        io = remote(host, port)
        start = time.time()
        io.sendline((flag + c).ljust(total_len, 'X'))
        io.recvall()
        elapsed = time.time() - start
        if elapsed > best_time:
            best_time = elapsed
            best_char = c
        io.close()
    flag += best_char
```

---

## Multi-Thread Anti-Debug with Decoy + Signal Handler Mixed Boolean-Arithmetic (ApoorvCTF 2026)

**Pattern (A Golden Experience Requiem):** Multi-threaded binary with layered anti-analysis: Thread 1 performs decoy operations (fake AES + deliberate crash via `ud2`), Thread 2 does the real flag computation in a SIGSEGV signal handler using Mixed Boolean Arithmetic (MBA), Thread 3 erases memory to prevent post-mortem analysis.

**Thread layout:**
| Thread | Purpose | Trap |
|--------|---------|------|
| Thread 1 | Decoy: AES-looking operations → `ud2` crash | Analysts waste time reversing fake crypto |
| Thread 2 | Real flag: SIGSEGV handler with MBA transforms | Hidden in signal handler, not main code path |
| Thread 3 | Memory eraser: zeros out flag data after computation | Prevents memory dumping |
| Main | rdtsc-based anti-debug timing check | Penalizes debugger-attached execution |

**Solving approach — pure Python emulation of MBA logic:**
```python
# MBA helpers (extracted from assembly)
def mba_add(a, b): return (a + b) & 0xff
def mba_xor(a, b): return (a ^ b) & 0xff

def mba_transform(i):
    """Position-dependent transform from signal handler."""
    val = (i * 7 + 0x3f) & 0xff
    rotated = ((i << 3) | (i >> 5)) & 0xff
    return mba_xor(val, rotated)

# S-box (SHA-256 initial hash values repurposed)
SBOX = [0x6a09e667, 0xbb67ae85, 0x3c6ef372, 0xa54ff53a,
        0x510e527f, 0x9b05688c, 0x1f83d9ab, 0x5be0cd19]

def sbox_lookup(i):
    idx = i & 7
    shift = ((i >> 3) & 3) * 8
    return (SBOX[idx] >> shift) & 0xff

# Two interleaved rodata arrays (even indices → array1, odd → array2)
rodata1 = bytes.fromhex("39407691b717c97879013adf3a2adea11c2b04e0")
rodata2 = bytes.fromhex("bb19b025e37eaa786c4116e7aeea00c9c623940d")

flag = []
for i in range(40):  # flag length
    t = mba_transform(i)
    s = sbox_lookup(i)
    mem = rodata1[i // 2] if i % 2 == 0 else rodata2[i // 2]
    flag.append(chr(t ^ s ^ mem))

print(''.join(flag))
```

**Key insight:** The real flag logic is in the signal handler (SIGSEGV/SIGILL), not the main thread. Thread 1's AES-like code and `ud2` crash are intentional misdirection. The `rdtsc` timing check detects debuggers and corrupts output. Bypass by extracting the MBA logic from assembly and reimplementing in Python — never run the binary under a debugger.

**Detection indicators:**
- Multiple `pthread_create` calls with different handler functions
- `signal(SIGSEGV, handler)` or `sigaction` setup
- `ud2` instruction (deliberate illegal instruction)
- `rdtsc` instructions for timing checks
- SHA-256 constants (0x6a09e667...) used as lookup tables, not for hashing

---

## INT3 Patch + Coredump Brute-Force Oracle (Pwn2Win 2016)

Instead of reversing complex transformation logic, patch a byte to `0xCC` (INT3) after the transform, enable core dumps, brute-force each character by running the binary and extracting the transformed result from the coredump via `strings`.

```bash
# Patch byte at transform output point to 0xCC
printf '\xcc' | dd of=binary bs=1 seek=$((0x400ebb)) conv=notrunc
ulimit -c unlimited
# Brute-force each position:
for c in $(seq 32 126); do
    echo -ne "$(printf '\\x%02x' $c)$known_suffix" | ./binary 2>/dev/null
    strings core | grep -q "$expected" && echo "Found: $c"
done
```

**Key insight:** Use INT3/SIGTRAP as a breakpoint oracle -- the coredump captures computed state at the crash point. Avoids full reverse engineering of the transformation.

---

## Signal Handler Chain + LD_PRELOAD Oracle (Nuit du Hack 2016)

Binary uses Unix signals for flow control: `main()` sends SIGINT to itself 1024 times, each handler checks one password character, then calls `signal()` to install the next handler. Bypass: LD_PRELOAD a custom `signal()` that logs when it's called (indicating correct character), brute-force each position.

```c
// LD_PRELOAD library:
#include <signal.h>
sighandler_t signal(int sig, sighandler_t handler) {
    write(2, "CORRECT\n", 8);  // signal() called = char was correct
    return SIG_DFL;
}
```

**Key insight:** Signal-handler-chain anti-reversing can be defeated by hooking `signal()` via LD_PRELOAD. The call to `signal()` (to install the next handler) acts as a side-channel confirming the current character.

---

### printf Format String VM Decompilation to Z3 (SECCON 2017)

A "virtual machine" implemented entirely via `%hhn` format strings. Format string `%hhn` writes the count of printed characters (mod 256) to a pointed-to byte. A sequence of `%Nc%hhn` instructions implements arbitrary byte-to-memory writes, effectively creating a bytecode VM.

**Step 1: Identify instruction types.**
Count unique format patterns to determine the instruction set:
```bash
# Normalize numbers and count unique patterns
sed -e 's/[[:digit:]]\+/1/g' program.fs | sort | uniq -c | sort -nr
```

**Step 2: Write a decompiler.**
Convert format patterns to C-style pseudocode. Each `%N...%hhn` pair maps to a memory write: extract the write address (from the argument pointer) and value (from the character count).

**Step 3: Recognize the algorithm.**
The pseudocode typically reveals a linear equation system over bytes. Map memory addresses to symbolic variables.

**Step 4: Generate Z3 constraints and solve.**
```python
from z3 import *

flag_len = 32  # adjust based on decompiled output
flag = [BitVec(f'f{i}', 8) for i in range(flag_len)]
s = Solver()

# Constrain to printable ASCII
for f in flag:
    s.add(f >= 0x20, f <= 0x7e)

# Add constraints from decompiled format string operations
# e.g., flag[3] + flag[7] == 0xAB (mod 256)
# These come from the write sequences: each %hhn accumulates
# character counts and writes the result to a target byte
s.add((flag[0] + flag[1]) & 0xFF == 0x9A)  # example constraint
s.add((flag[2] ^ flag[3]) & 0xFF == 0x3F)  # example constraint
# ... (add all constraints from decompilation)

if s.check() == sat:
    m = s.model()
    print(bytes([m[f].as_long() for f in flag]))
```

**Decompilation approach in detail:**
1. Extract the write address and value from each `%N...%hhn` pair
2. Map memory addresses to symbolic variables (flag bytes)
3. Build an equation system from the write sequences
4. Solve with Z3

**Key insight:** Format string `%hhn` writes the count of printed characters (mod 256) to a pointed-to byte. A sequence of `%Nc%hhn` instructions implements arbitrary byte-to-memory writes, effectively creating a bytecode VM. Decompile by: (1) extract the write address and value from each `%N...%hhn` pair, (2) map memory addresses to symbolic variables, (3) build an equation system from the write sequences, (4) solve with Z3.

**References:** SECCON 2017

---

## Quadtree Recursive Image Format Parser (Google CTF Quals 2018)

**Pattern:** Challenge ships a proprietary image format. Reverse engineering shows it is a quadtree: the canvas is split into the largest enclosing power-of-two square, that square is recursively split into four quadrants, and a 1-byte command tells which of the four to subdivide further. Quadrants marked as "leaf" are followed by three bytes of RGB color; the rest recurse.

```python
# Command byte: bits 3..0 = {top-left, top-right, bottom-left, bottom-right}
# Bit set ⇒ subdivide; bit clear ⇒ leaf (next 3 bytes = RGB)

def parse(stream, x, y, size):
    cmd = stream.read(1)[0]
    half = size // 2
    children = [
        (x,        y       ),
        (x + half, y       ),
        (x,        y + half),
        (x + half, y + half),
    ]
    for i, (cx, cy) in enumerate(children):
        if cmd & (1 << (3 - i)):
            parse(stream, cx, cy, half)
        else:
            rgb = stream.read(3)
            fill_rect(cx, cy, half, half, rgb)
```

Walk the recursion until `half == 1` (or until a "leaf" bit is seen) and paint the canvas as the format pushes bytes. The flag image renders correctly once the quadrant bit order is matched.

**Key insight:** Proprietary image/compression formats in CTF challenges are almost always quadtrees, LZ77 variants, or Huffman streams. Look for recursive structures with a short command byte followed by either more commands or fixed-width leaf data. Prototype the parser by printing the recursion depth and offset for each call — mismatched depth is the first signal that the bit order or leaf size is wrong.

**References:** Google CTF Quals 2018 — writeup 10335
