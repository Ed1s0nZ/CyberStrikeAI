# Pwn Field Notes

Detailed pwn notes that support [`SKILL.md`](SKILL.md). Read this file after confirming the challenge really needs exploitation.

## Table of Contents

- [Heap Exploitation](#heap-exploitation)
- [Additional Exploit Notes](#additional-exploit-notes)
  - [talloc Pool Header Forgery](#talloc-pool-header-forgery)
  - [JIT Compilation Exploits](#jit-compilation-exploits)
  - [Type Confusion in Interpreters](#type-confusion-in-interpreters)
  - [Off-by-One Index / Size Corruption](#off-by-one-index--size-corruption)
  - [Double win() Call](#double-win-call)
  - [Arbitrary Read/Write to Shell via GOT Overwrite](#arbitrary-readwrite-to-shell-via-got-overwrite)
  - [Stack Leak via __environ and memcpy Overflow](#stack-leak-via-__environ-and-memcpy-overflow)
  - [JIT Sandbox Escape via uint16 Jump Truncation](#jit-sandbox-escape-via-uint16-jump-truncation)
  - [DNS Compression Pointer Stack Overflow](#dns-compression-pointer-stack-overflow)
  - [ELF Code Signing Bypass via Program Headers](#elf-code-signing-bypass-via-program-headers)
  - [Game Level Format Signed/Unsigned Coordinate Mismatch](#game-level-format-signedunsigned-coordinate-mismatch)
  - [File Descriptor Inheritance via Missing O_CLOEXEC](#file-descriptor-inheritance-via-missing-o_cloexec)
  - [Sign Extension Integer Underflow in Metadata Parsing](#sign-extension-integer-underflow-in-metadata-parsing)
  - [ROP Chain Construction with Read-Only Primitive](#rop-chain-construction-with-read-only-primitive)
  - [Esoteric Language GOT Overwrite](#esoteric-language-got-overwrite)
  - [Protocol Stack Bleeding](#protocol-stack-bleeding)
  - [Timing Attack Flag Recovery](#timing-attack-flag-recovery)
  - [DNS Record Buffer Overflow](#dns-record-buffer-overflow)
  - [ASAN Shadow Memory Exploitation](#asan-shadow-memory-exploitation)
  - [Format String .fini_array Loop for Multi-Stage Exploitation](#format-string-fini_array-loop-for-multi-stage-exploitation)
  - [Format String with RWX .fini_array Hijack](#format-string-with-rwx-fini_array-hijack)
  - [Custom Canary Preservation](#custom-canary-preservation)
  - [MD5 Preimage Gadget Construction](#md5-preimage-gadget-construction)
  - [Python Sandbox Escape](#python-sandbox-escape)
  - [VM GC-Triggered UAF (Slab Reuse)](#vm-gc-triggered-uaf-slab-reuse)
  - [GC Null-Reference Cascading Corruption](#gc-null-reference-cascading-corruption)
  - [OOB Read via Stride/Rate Leak](#oob-read-via-striderate-leak)
  - [SROP with UTF-8 Constraints](#srop-with-utf-8-constraints)
  - [VM Exploitation (Custom Bytecode)](#vm-exploitation-custom-bytecode)
  - [FUSE/CUSE Character Device Exploitation](#fusecuse-character-device-exploitation)
  - [Busybox/Restricted Shell Escalation](#busyboxrestricted-shell-escalation)
  - [process_vm_readv Sandbox Bypass](#process_vm_readv-sandbox-bypass)
  - [Named Pipe (mkfifo) File Size Bypass](#named-pipe-mkfifo-file-size-bypass)
  - [Shell Tricks](#shell-tricks)
  - [Double Stack Pivot to BSS via leave;ret](#double-stack-pivot-to-bss-via-leaveret)
  - [RETF Architecture Switch for Seccomp Bypass](#retf-architecture-switch-for-seccomp-bypass)
  - [Leakless Libc via Multi-fgets stdout FILE Overwrite](#leakless-libc-via-multi-fgets-stdout-file-overwrite)
  - [Signed/Unsigned Char Underflow to Heap Overflow](#signedunsigned-char-underflow-to-heap-overflow)
  - [TLS Destructor Hijack via `__call_tls_dtors`](#tls-destructor-hijack-via-__call_tls_dtors)
  - [Signed Int Overflow to Negative OOB Heap Write](#signed-int-overflow-to-negative-oob-heap-write)
  - [Custom Shadow Stack Bypass via Pointer Overflow](#custom-shadow-stack-bypass-via-pointer-overflow)
  - [Windows SEH Overwrite + VirtualAlloc ROP](#windows-seh-overwrite--virtualalloc-rop)
  - [SeDebugPrivilege to SYSTEM](#sedebugprivilege-to-system)
  - [mmap/munmap Size Mismatch UAF](#mmapmunmap-size-mismatch-uaf)
  - [strcspn Indirect Null Byte Injection](#strcspn-indirect-null-byte-injection)
  - [Windows CFG Bypass Using system() as Valid Call Target](#windows-cfg-bypass-using-system-as-valid-call-target)
  - [4-Byte Shellcode with Timing Side-Channel](#4-byte-shellcode-with-timing-side-channel)
  - [CRC Oracle as Arbitrary Read Primitive](#crc-oracle-as-arbitrary-read-primitive)
  - [UTF-8 Case Conversion Buffer Overflow](#utf-8-case-conversion-buffer-overflow)
- [Useful Commands](#useful-commands)

## Heap Exploitation

- tcache poisoning (glibc 2.26+), fastbin dup / double free
- House of Force (old glibc), unsorted bin attack
- **House of Apple 2** (glibc 2.34+): FSOP (File Stream Oriented Programming) via `_IO_wfile_jumps` when `__free_hook`/`__malloc_hook` removed. Fake FILE with `_flags = " sh"`, vtable chain → `system(fp)`. For SUID binaries: use `setcontext()` variant to stack pivot → `setuid(0)` → `system()` (dash drops privs when uid != euid). See [heap-techniques.md](heap-techniques.md#setcontext-variant-for-suid-binaries-midnight-flag-2026).
- **Classic unlink**: Corrupt adjacent chunk metadata, trigger backward consolidation for write-what-where primitive. Pre-2.26 glibc only. See [heap-techniques.md](heap-techniques.md#classic-heap-unlink-attack-crypto-cat).
- **House of Force:** Corrupt top chunk size to `0xffffffffffffffff`, next `malloc(target - top - 2*SIZE_SZ)` returns arbitrary address. Pre-2.29 glibc only. See [heap-techniques.md](heap-techniques.md#house-of-force-csaw-ctf-2016).
- **House of Einherjar**: Off-by-one null clears PREV_INUSE, backward consolidation with self-pointing unlink.
- **Safe-linking** (glibc 2.32+): tcache fd mangled as `ptr ^ (chunk_addr >> 12)`.
- Check glibc version: `strings libc.so.6 | grep GLIBC`
- Freed chunks contain libc pointers (fd/bk) -> leak via error messages or missing null-termination
- Heap feng shui: control alloc order/sizes, create holes, place targets adjacent to overflow source
- **Unsafe unlink + top chunk consolidation**: After unlink writes self-pointer to BSS, craft fake BSS chunk spanning to top chunk. `free()` consolidates, relocating heap base to BSS. Subsequent mallocs return BSS memory. See [heap-techniques.md](heap-techniques.md#unsafe-unlink-to-bss--top-chunk-consolidation-seccon-2016).

**House of Orange:** Corrupt top chunk size → large malloc forces sysmalloc → old top freed without calling `free()`. Chain with FSOP. See [heap-techniques.md](heap-techniques.md#house-of-orange).

**House of Spirit:** Forge fake chunk in target area, `free()` it, reallocate to get write access. Requires valid size + next chunk size. See [heap-techniques.md](heap-techniques.md#house-of-spirit).

**House of Lore:** Corrupt smallbin `bk` → link fake chunk → second malloc returns attacker-controlled address. See [heap-techniques.md](heap-techniques.md#house-of-lore).

**ret2dlresolve:** Forge Elf64_Sym/Rela to resolve arbitrary libc function without leak. `Ret2dlresolvePayload(elf, symbol="system", args=["/bin/sh"])`. Requires Partial RELRO. See [advanced.md](advanced.md#ret2dlresolve).

**tcache stashing unlink (glibc 2.29+):** Corrupt smallbin chunk's `bk` during tcache stashing → arbitrary address linked into tcache → write primitive. See [heap-techniques.md](heap-techniques.md#tcache-stashing-unlink-attack).

**UAF vtable pointer encoding shell argument:** After UAF, heap spray places `system()` at offset +3. Object address containing `0x6873` ("sh") in low bytes doubles as the command string argument when `system(this)` is called through the hijacked vtable. See [heap-techniques-2.md](heap-techniques-2.md#uaf-vtable-pointer-encoding-shell-argument-bctf-2017).

**Fastbin stdout vtable two-stage hijack (PIE + Full RELRO):** Use 0x7f byte in libc's stdout region as fake fastbin chunk size. Two-stage: first vtable redirect to `gets()` (rdi=stdout), then `gets()` overwrites vtable again to `system()` with command string. See [heap-techniques.md](heap-fsop.md#fastbin-stdout-vtable-two-stage-hijack-for-pie--full-relro-asis-ctf-2017).

See [heap-techniques.md](heap-techniques.md) for House of Apple 2 FSOP chain (+ setcontext SUID variant), House of Orange/Spirit/Lore/Force, tcache stashing unlink, custom allocator exploitation (nginx pools, talloc), classic unlink, musl libc heap. See [advanced.md](advanced.md) for ret2dlresolve, heap overlap via base conversion, tree data structure stack underallocation.

**GF(2) Gaussian elimination for tcache poisoning:** When a deterministic XOR cipher corrupts heap metadata as a side effect, model the corruption as linear algebra over GF(2). Find a subset of cipher seeds whose combined XOR transforms tcache `fd` from current value to target address. See [advanced-exploits-4.md](advanced-exploits-4.md#gf2-gaussian-elimination-for-multi-pass-tcache-poisoning-midnight-flag-2026).

## Additional Exploit Notes

### talloc Pool Header Forgery
**Pattern:** talloc (hierarchical allocator in Samba/CUPS) pool header forgery. Forge fake pool header with controlled `end`/`object_count` fields to redirect next `talloc()` to arbitrary address. Leak GOT for libc, write `__free_hook` with `system()`. See [heap-techniques.md](heap-techniques.md#talloc-pool-header-forgery-for-arbitrary-readwrite-boston-key-party-2016).

### JIT Compilation Exploits
**Pattern:** Off-by-one in instruction encoding -> misaligned machine code. Embed shellcode as operand bytes of subtraction operations, chain with 2-byte `jmp` instructions. See [advanced.md](advanced.md).

**BF JIT unbalanced bracket:** Unbalanced `]` pops tape address (RWX) from stack → write shellcode to tape with `+`/`-`, trigger `]` to jump to it. See [advanced.md](advanced.md).

### Type Confusion in Interpreters
**Pattern:** Interpreter sets wrong type tag → struct fields reinterpreted. Unused padding bytes in one variant become active pointers/data in another. Flag bytes as type value trigger UNKNOWN_DATA dump. See [advanced.md](advanced.md).

### Off-by-One Index / Size Corruption
**Pattern:** Array index 0 maps to `entries[-1]`, overlapping struct metadata (size field). Corrupted size → OOB read leaks canary/libc, then OOB write places ROP chain. See [advanced.md](advanced.md).

### Double win() Call
**Pattern:** `win()` checks `if (attempts++ > 0)` — needs two calls. Stack two return addresses: `p64(win) + p64(win)`. See [advanced.md](advanced.md).

### Arbitrary Read/Write to Shell via GOT Overwrite
**Pattern:** Binary provides explicit read/write primitives. Leak libc via GOT read, overwrite `strtoll@GOT` with `system`, next call becomes `system(user_input)`. Choose GOT targets where the function takes a user-controlled string as first arg. See [advanced-exploits-3.md](advanced-exploits-3.md#arbitrary-readwrite-to-shell-via-got-overwrite-bsidessf-2026).

### Stack Leak via __environ and memcpy Overflow
**Pattern:** Binary with read-only primitive and `memcpy(stack_buf, user_addr, user_len)`. Leak libc via GOT, leak stack via `__environ`, plant ROP addresses in input buffer, overflow memcpy to copy them over return address, send EOF to trigger return. See [advanced-exploits-3.md](advanced-exploits-3.md#stack-leak-via-__environ-and-memcpy-overflow-bsidessf-2026).

### JIT Sandbox Escape via uint16 Jump Truncation
**Pattern:** JIT compiler truncates conditional jump offset to uint16, causing misalignment when code exceeds 64KB. Embed 2-byte shellcode fragments in `add` immediates, thread with `jmp $+3` to chain execution. See [advanced-exploits-3.md](advanced-exploits-3.md#jit-sandbox-escape-via-conditional-jump-uint16-truncation-bsidessf-2026).

### DNS Compression Pointer Stack Overflow
**Pattern:** Custom DNS server doesn't track decompressed name length. Compression pointer chains revisit data, overflowing stack buffer. Split ROP chain across multiple DNS question entries. See [advanced-exploits-3.md](advanced-exploits-3.md#dns-compression-pointer-stack-overflow-with-multi-question-rop-bsidessf-2026).

### ELF Code Signing Bypass via Program Headers
**Pattern:** Signing scheme hashes section headers/content but not program headers. Append shellcode, modify LOAD segment's `p_offset` to point to appended data — signature still valid, loader executes attacker code. See [advanced-exploits-3.md](advanced-exploits-3.md#elf-code-signing-bypass-via-program-header-manipulation-bsidessf-2026).

### Game Level Format Signed/Unsigned Coordinate Mismatch
**Pattern:** Level editor parses signed integer coordinates but bounds-checks via unsigned comparison — negative coordinates pass the check and write block IDs (arbitrary bytes) before the level array, enabling stack return address overwrite. Leak stack address via hidden developer mode, encode shellcode as block IDs. See [advanced-exploits-3.md](advanced-exploits-3.md#game-level-format-signedunsigned-coordinate-mismatch-bsidessf-2026).

### File Descriptor Inheritance via Missing O_CLOEXEC
**Pattern:** Service reads secret into `memfd_create()` FD without `MFD_CLOEXEC`, then calls `system()` for user commands — child inherits the FD. Bypass `strstr()` keyword filters with shell quote splitting (`p'r'oc` instead of `proc`) to read `/proc/self/fd/N`. See [advanced-exploits-3.md](advanced-exploits-3.md#file-descriptor-inheritance-via-missing-o_cloexec-bsidessf-2026).

### Sign Extension Integer Underflow in Metadata Parsing
**Pattern:** Metadata parser's `to_int32` converts unsigned values >= 0x80000000 to negative signed integers. Used as array index/offset, this causes OOB memory access. Iterate byte-by-byte to leak flag from memory. See [advanced-exploits-3.md](advanced-exploits-3.md#sign-extension-integer-underflow-in-metadata-parsing-bsidessf-2026).

### ROP Chain Construction with Read-Only Primitive
**Pattern:** Binary with only `read()` primitive — no write, no win function. Leak libc via GOT, then "import" arbitrary byte values onto the stack by reading from libc offsets whose content matches desired ROP gadget addresses. Read primitive doubles as write primitive. See [advanced-exploits-3.md](advanced-exploits-3.md#rop-chain-construction-with-read-only-primitive-bsidessf-2026).

### Esoteric Language GOT Overwrite
**Pattern:** Brainfuck/Pikalang interpreter with unbounded tape = arbitrary read/write relative to buffer base. Move pointer to GOT, overwrite byte-by-byte with `system()`. See [advanced.md](advanced.md).

### Protocol Stack Bleeding
Custom network protocols echoing data based on length field leak stack memory when length exceeds actual data (Heartbleed-style). See [overflow-basics.md](overflow-basics.md#protocol-length-field-stack-bleeding-ekoparty-ctf-2016).

### Timing Attack Flag Recovery
Validation time varies per correct character; measure elapsed time per candidate byte to recover flag character-by-character. See [advanced-exploits.md](advanced-exploits.md#timing-attack-for-character-by-character-flag-recovery-rc3-ctf-2016).

### DNS Record Buffer Overflow
**Pattern:** Many AAAA records overflow stack buffer in DNS response parser. Set up DNS server with excessive records, overwrite return address. See [advanced.md](advanced.md).

### ASAN Shadow Memory Exploitation
**Pattern:** Binary with AddressSanitizer has format string + OOB write. ASAN may use "fake stack" (50% chance). Leak PIE, detect real vs fake stack, calculate OOB write offset to overwrite return address. See [advanced.md](advanced.md).

### Format String .fini_array Loop for Multi-Stage Exploitation
**Pattern:** No GOT function called after `printf()`. Overwrite `.fini_array[0]` with `main()` for re-execution loop. Stage 1: leak libc/stack. Stage 2: `printf@GOT` to `system()`, `__stack_chk_fail@GOT` to `main()`. Stage 3: corrupt canary to trigger `__stack_chk_fail` re-entry, now `printf(input)` is `system(input)`. See [format-string.md](format-string.md#format-string-fini_array-loop-for-multi-stage-exploitation-codegate-2016).

### Format String with RWX .fini_array Hijack
**Pattern (Encodinator):** Base85-encoded input in RWX memory passed to `printf()`. Write shellcode to RWX region, overwrite `.fini_array[0]` via format string `%hn` writes. Use convergence loop for base85 argument numbering. See [advanced.md](advanced.md).

### Custom Canary Preservation
**Pattern:** Buffer overflow must preserve known canary value. Write exact canary bytes at correct offset: `b'A' * 64 + b'BIRD' + b'X'`. See [advanced.md](advanced.md).

### MD5 Preimage Gadget Construction
**Pattern (Hashchain):** Brute-force MD5 preimages with `eb 0c` prefix (jmp +12) to skip middle bytes; bytes 14-15 become 2-byte i386 instructions. Build syscall chains from gadgets like `31c0` (xor eax), `cd80` (int 0x80). See [advanced.md](advanced.md) for C code and v2 technique.

### Python Sandbox Escape
AST bypass via f-strings, audit hook bypass with `b'flag.txt'` (bytes vs str), MRO-based `__builtins__` recovery. See [sandbox-escape.md](sandbox-escape.md).

### VM GC-Triggered UAF (Slab Reuse)
**Pattern:** Custom VM with NEWBUF/SLICE/GC opcodes. Slicing creates shared slab reference; dropping+GC'ing slice frees slab while parent still holds it. Allocate function object to reuse slab, leak code pointer via UAF read, overwrite with win() address. See [advanced.md](advanced.md).

### GC Null-Reference Cascading Corruption
**Pattern:** Mark-compact GC follows null references to heap address 0, creating fake object. During compaction, memmove cascades corruption through adjacent object headers → OOB access → libc leak → FSOP. See [advanced.md](advanced.md).

### OOB Read via Stride/Rate Leak
**Pattern:** String processing function with user-controlled stride skips past null terminator, leaking stack canary and return address one byte at a time. Then overflow with leaked values. See [overflow-basics.md](overflow-basics.md).

### SROP with UTF-8 Constraints
**Pattern:** When payload must be valid UTF-8 (Rust binaries, JSON parsers), use SROP — only 3 gadgets needed. Multi-byte UTF-8 sequences spanning register field boundaries "fix" high bytes. See [rop-advanced.md](rop-advanced.md).

### VM Exploitation (Custom Bytecode)
**Pattern:** Custom VM with OOB read/write in syscalls. Leak PIE via XOR-encoded function pointer, overflow to rewrite pointer with `win() ^ KEY`. See [sandbox-escape.md](sandbox-escape.md).

### FUSE/CUSE Character Device Exploitation
Look for `cuse_lowlevel_main()` / `fuse_main()`, backdoor write handlers with command parsing. Exploit to `chmod /etc/passwd` then modify for root access. See [sandbox-escape.md](sandbox-escape.md).

### Busybox/Restricted Shell Escalation
Find writable paths via character devices, target `/etc/passwd` or `/etc/sudoers`, modify permissions then content. See [sandbox-escape.md](sandbox-escape.md).

### process_vm_readv Sandbox Bypass
**Pattern:** Sandbox validates file paths via `process_vm_readv()` + `realpath()`. Map memory with `PROT_READ` only at fixed address via `mmap(MAP_FIXED)` -- sandbox's `process_vm_readv` fails silently, bypassing path validation entirely. See [sandbox-escape.md](sandbox-escape.md#process_vm_readv-failure-as-sandbox-escape-0ctf-2016).

### Named Pipe (mkfifo) File Size Bypass
**Pattern:** Binary checks `stat()` file size before reading. Named pipes report `st_size = 0` but deliver arbitrary data via `read()`. `mkfifo /tmp/pipe && cat payload > /tmp/pipe &` then pass pipe to binary. Combine with `ln -s /flag arena.c` for string reuse in ROP. See [sandbox-escape.md](sandbox-escape.md#named-pipe-mkfifo-for-file-size-check-bypass-nuit-du-hack-2016).

### Shell Tricks
`exec<&3;sh>&3` for fd redirection, `$0` instead of `sh`, `ls -la /proc/self/fd` to find correct fd. See [sandbox-escape.md](sandbox-escape.md).

### Double Stack Pivot to BSS via leave;ret
**Pattern:** Small overflow (only RBP + RIP). Overwrite RBP → BSS address, RIP → `leave; ret` gadget. `leave` sets RSP = RBP (BSS). Second stage at BSS calls `fgets(BSS+offset, large_size, stdin)` to load full ROP chain. See [rop-advanced.md](rop-advanced.md#double-stack-pivot-to-bss-via-leaveret-midnightflag-2026).

### RETF Architecture Switch for Seccomp Bypass
**Pattern:** Seccomp blocks 64-bit syscalls (`open`, `execve`). Use `retf` gadget to load CS=0x23 (IA-32e compatibility mode). In 32-bit mode, `int 0x80` uses different syscall numbers (open=5, read=3, write=4) not covered by the filter. Requires `mprotect` to make BSS executable for 32-bit shellcode. See [rop-advanced.md](rop-advanced.md#retf-architecture-switch-for-seccomp-bypass-midnightflag-2026).

### Leakless Libc via Multi-fgets stdout FILE Overwrite
**Pattern:** No libc leak available. Chain multiple `fgets(addr, 7, stdin)` calls via ROP to construct fake stdout FILE struct on BSS. Set `_IO_write_base` to GOT entry, call `fflush(stdout)` → leaks GOT content → libc base. The 7-byte writes avoid null byte corruption since libc pointer MSBs are already `\x00`. See [advanced-exploits-2.md](advanced-exploits-2.md#leakless-libc-via-multi-fgets-stdout-file-overwrite-midnightflag-2026).

### Signed/Unsigned Char Underflow to Heap Overflow
**Pattern:** Size field stored as `signed char`, cast to `unsigned char` for use. `size = -112` → `(unsigned char)(-112) = 144`, overflowing a 127-byte buffer by 17 bytes. Combine with XOR keystream brute-force for byte-precise writes, forge chunk sizes for unsorted bin promotion (libc leak), FSOP stdout for TLS leak, and TLS destructor (`__call_tls_dtors`) overwrite for RCE. See [advanced-exploits-2.md](advanced-exploits-2.md#signedunsigned-char-underflow-to-heap-overflow--tls-destructor-hijack-midnightflag-2026).

### TLS Destructor Hijack via `__call_tls_dtors`
**Pattern:** Alternative to House of Apple 2 on glibc 2.34+. Forge `__tls_dtor_list` entries with pointer-guard-mangled function pointers: `encoded = rol(target ^ pointer_guard, 0x11)`. Requires leaking pointer guard from TLS segment (via FSOP stdout redirection). Each node calls `PTR_DEMANGLE(func)(obj)` on exit. See [advanced-exploits-2.md](advanced-exploits-2.md#tls-destructor-overwrite-for-rce-via-__call_tls_dtors).

### Signed Int Overflow to Negative OOB Heap Write
**Pattern (Canvas of Fear):** Index formula `y * width + x` in signed 32-bit int overflows to negative value, passing bounds check and writing backward into heap metadata. Use to corrupt adjacent chunk sizes/pointers, leak libc via unsorted bin, redirect a data pointer to `environ` for stack leak, then write ROP chain to main's return address. When binary is behind a web API, chain XSS → Fetch API → heap exploit, and inject `\n` in API parameters for command stacking via `sendline()`. See [advanced-exploits-2.md](advanced-exploits-2.md#signed-int-overflow-to-negative-oob-heap-write--xss-to-binary-pwn-bridge-midnight-2026) for full exploit chain, XSS bridge pattern, and RGB pixel write primitive.

### Custom Shadow Stack Bypass via Pointer Overflow
**Pattern (Revenant):** Userland shadow stack in `.bss` with unbounded pointer. Recurse to advance `shadow_stack_ptr` past the array into user-controlled memory (e.g., `username` buffer), write `win()` there, then overflow the hardware stack return address to match. Both checks pass. See [advanced-exploits-2.md](advanced-exploits-2.md#custom-shadow-stack-bypass-via-pointer-overflow-midnight-2026) for full exploit and `.bss` layout analysis.

### Windows SEH Overwrite + VirtualAlloc ROP
Format string leak defeats ASLR. SEH (Structured Exception Handler) overwrite with stack pivot to ROP chain. `pushad` builds VirtualAlloc call frame for DEP (Data Execution Prevention) bypass. Detached process launcher for shell stability on thread-based servers. See [advanced-exploits-4.md](advanced-exploits-4.md#windows-seh-overwrite--pushad-virtualalloc-rop-rainbowtwo-htb).

### SeDebugPrivilege to SYSTEM
`SeDebugPrivilege` + Meterpreter `migrate -N winlogon.exe` -> SYSTEM. See [advanced-exploits-4.md](advanced-exploits-4.md#sedebugprivilege-to-system-rainbowtwo-htb).

### mmap/munmap Size Mismatch UAF
Over-unmap via mmap(small)/munmap(large) destroys adjacent mappings. Thread stack fills gap, old buffer pointer becomes write-into-stack. Race-free UAF variant. See [advanced-exploits-4.md](advanced-exploits-4.md#mmapmunmap-size-mismatch-uaf-for-thread-stack-overlap-0ctf-2017).

### strcspn Indirect Null Byte Injection
`strcspn(buf, "\r\n")` + null write truncates strings at injected newlines. Bypasses CGI null-byte filtering for path traversal. See [advanced-exploits-4.md](advanced-exploits-4.md#strcspn-as-indirect-null-byte-injection-bsidessf-2017).

### Windows CFG Bypass Using system() as Valid Call Target
**Pattern:** Windows CFG validates indirect call targets but `system()` from msvcrt passes validation since it is a legitimate API entry point. Overwrite function pointer with `system()`, use comma instead of space in arguments to bypass input filters. See [advanced-exploits-4.md](advanced-exploits-4.md#windows-cfg-bypass-using-system-as-valid-call-target-insomnihack-2017).

### 4-Byte Shellcode with Timing Side-Channel
**Pattern:** Binary executes only 4 bytes of user shellcode in a 4096-iteration loop. Callee-saved registers (r12-r15) persist across iterations, enabling incremental state building. The 4096x loop amplifies timing differences for reliable side-channel measurement. See [advanced-exploits-3.md](advanced-exploits-3.md#4-byte-shellcode-with-timing-side-channel-via-persistent-registers-google-ctf-2017).

### CRC Oracle as Arbitrary Read Primitive
**Pattern:** CRC is bijective on single bytes. Overflow a pointer to control the CRC input address, precompute all 256 single-byte CRCs, and reverse-lookup each byte of arbitrary memory. Chain reads to leak GOT, libc, stack, and canary. See [advanced-exploits-3.md](advanced-exploits-3.md#crc-oracle-as-arbitrary-read-primitive-asis-ctf-2017).

### UTF-8 Case Conversion Buffer Overflow
**Pattern:** Unicode case conversion can expand character byte length (e.g., 2-byte UTF-8 becomes 4 bytes when uppercased). If buffer is sized for input length, the longer output overflows. Affects GLib `g_utf8_strup()`, ICU, and similar functions. See [advanced-exploits-3.md](advanced-exploits-3.md#utf-8-case-conversion-buffer-overflow-hitb-ctf-2017).

## Useful Commands

`checksec`, `one_gadget`, `ropper`, `ROPgadget`, `seccomp-tools dump`, `strings libc | grep GLIBC`. See [rop-advanced.md](rop-advanced.md) for full command list and pwntools template.
